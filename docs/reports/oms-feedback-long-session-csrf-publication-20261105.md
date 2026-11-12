# OMS Feedback 长登录 CSRF 连续性与提交后停留发布记录（2026-11-05）

## 问题与结论

登录后的“产品反馈”最初可用，但约一小时后点击“提交反馈”会显示“产品反馈暂时不可用”，即使 Feedback SDK 的本人反馈与通知请求仍返回 200。根因是 IAM Session 有效期长于 `oms_iam_csrf` Cookie 的一小时有效期；Workbench Feedback 宿主直接读取 Cookie，缺失时在请求 `/gateway/feedback/submission-snapshot` 之前就抛错，随后把本地 CSRF 缺失误呈现为整个产品不可用。

Workbench 现会在 Feedback 配置和提交快照请求前读取当前 CSRF；Cookie 缺失时，通过同源 Identity `GET /api/v1/csrf` 刷新，校验响应 JSON 与新 Cookie 一致，并复用同一时刻的刷新请求。Session、Tenant、Subject、Feedback 配置和快照端口的既有鉴权不变；刷新 CSRF 不会创建登录 Session。

同一版本还发布 BF-303：检测到本账号新增 Feedback 后停止有界轮询，但保持 iframe 可见并把已配置的 V2 SDK 切换到 `status`，让用户停留在“我的反馈”，不再主动关闭反馈层或进入 V3 智能客服。

## 本地验证

- `node --check internal/server/oms_phase1_feedback.js`：通过；
- `node --test internal/server/oms_phase1_feedback.test.mjs`：28 项断言通过，覆盖 Cookie 复用、缺失时刷新、并发去重、配置／快照使用刷新值及失败关闭；
- Workbench `GOWORK=off go test ./...` 与 `GOWORK=off go vet ./...`：通过；
- `git diff --check`：通过；
- 发布前逐项摘要比较的 15 个 Workbench 静态资源中，14 个与线上 v1.71 完全一致；唯一内容变化为 Feedback JS，HTML 只更新该资源的缓存版本。

## 发布制品与恢复点

- Unified Workbench：`v0.1.0-test.oms-v1.74`
- 镜像摘要：`sha256:632c975b434c6fe914e6ebf88cf64aa9fdde7446500b0d864ec526165e952807`
- 平台：`linux/amd64`
- 构建元数据：`deployments/d1/.runtime/build-metadata/v74-workbench.json`
- D1 环境：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.74.https.env`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261105T112239Z`
- 回滚环境：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.72.https.env`

候选环境从 v1.72 复制，只替换 `D1_WORKBENCH_IMAGE`；排除该行后的 SHA-256 同为 `b6624c82432816e25c58f404ac8b69721e13b2bbe7da0288b48a8e3d99a0d2f3`。未部署此前仅含 BF-303 的 v1.73 候选，v1.74 将其连同 CSRF 连续性修复一起取代。

## D1 发布验证

1. 候选 `preflight-upgrade` 成功，IAM 契约解析返回 `contracts-valid`。
2. 正式 `upgrade` 成功并创建上述恢复点；D1 正式升级入口按既有策略重建组合容器并保留命名数据卷，镜像／环境选择除 Workbench 外未改变。
3. 升级后独立执行 D1 `check` 成功；Identity schema 保持 `000012_multi_organization_accounts.sql`，各启用 Owner schema 均为 current。
4. 恢复点 `checksums.sha256` 的 19 个条目全部通过。
5. 19/19 个长期容器均为 healthy；运行中的 Workbench 配置镜像、Image ID 与 Registry 摘要精确等于 v1.74，架构为 Linux AMD64。

## 公网验证

- `https://oatest.antuinfo.cn/` 返回 303 到 `/workbench/?view=oms-v1`。
- 公网 `oms-phase1-feedback.js?v=20261104.2` 与本地候选 SHA-256 同为 `b3bd4a9fe3806ba0bd8df8c7fe40a8374bce0414476237f6c05b1d7b9bcf7fda`，包含 CSRF 刷新端点、刷新复用和 BF-303 状态切换。
- 公网 `GET /auth/api/v1/csrf` 返回 200、`Cache-Control: no-store` 与新的 `oms_iam_csrf` Cookie。
- 隔离请求携带该 Cookie 和匹配 Header 调用提交快照时，在没有 Session 的情况下返回 401，证明 CSRF 可建立且 Session 安全边界没有被绕过。
- 正式 D1 重启使验证浏览器原 Session 失效；刷新后正确回到 OMS 登录页。本轮未代填账号密码，因此发布后的登录态点击验收留待用户重新登录；发布前已在真实登录态确认 SDK 本人反馈／通知 200，而缺失 CSRF 时旧 Workbench 会在快照请求前本地失败。

## 边界

本次没有修改 Feedback SDK、Gateway、Identity schema、API Key、HMAC 身份派生、Grant、账号、组织或业务 Owner，也没有创建反馈或执行线上业务写入。CSRF 刷新只恢复与现有 Session 相匹配的浏览器请求能力，不延长 Session，也不把 CSRF 或 Feedback Key 写入日志、文档或固定证据。
