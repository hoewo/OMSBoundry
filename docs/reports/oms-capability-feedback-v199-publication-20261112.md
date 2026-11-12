# OMS 全站 Capability 与 Feedback Session v1.99 发布记录

日期：2026-11-12  
事项：BF-330、BF-331  
目标：`https://oatest.antuinfo.cn`  
结论：已发布；运行与公网负向／Feedback 正向证据通过。业务 Owner 的有权账号公网正向读取未借系统管理员扩权，须使用独立业务验收账号继续执行。

## 发布内容

- Gateway 的 `POST /gateway/authorize/{application}` 在 `Accept: application/json` 时只返回 `application_id`、`audience`、`expires_at`、`expires_in_seconds`，响应 `Cache-Control: no-store`；Capability JWT 继续只存在于按 Owner Route 隔离的 HttpOnly Cookie 和 Gateway 内部 `Authorization: Bearer` Header。
- Workbench 按 Audience 在内存维护最长 5 分钟、最多提前 60 秒更新的租约；并发更新 single-flight，写请求发送前必须换新票，读请求只可在旧票仍有效时有界降级，Owner 凭证失效最多换票并重放一次。
- Feedback 正式模式从浏览器直出长期 API Key 切换为 Gateway 中介的 15 分钟 `fbs_` Session。长期 Key 只留在 Gateway；浏览器不持有 Refresh Token，也不把 Session 写入 Local Storage、Session Storage、URL 或业务库。
- 公网只读验收脚本同步到当前资源版本，能够检查 JSON 租约字段、`no-store` 和浏览器无凭证持久化，并新增 `0600` 业务验收账号凭证文件入口。系统管理员不再被错误假设为业务操作员。

## 交付物

- 源码提交：`78f334642152823fef1ba2e6a94c2f5fa339d699`
- Gateway：`v0.1.0-test.oms-v1.99@sha256:0b02079ffdd8cc289757b439f5dbf37a543a6809bffe534ecd68383ac115fbfd`
- Unified Workbench：`v0.1.0-test.oms-v1.99@sha256:ff728e62898c09bb0debbdff8825102a0bada00e223186eefafca10c1eeb5ab4`
- Linux AMD64 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.99_linux_amd64.tar.gz`
- 发布包 SHA-256：`2dd804e840b0ebef69c93474e2c7d5b7c89e3bea3b38552857e38d123201bca7`
- 发布前联合备份：`/var/backups/omsbizfoundry/20261112T125812Z`

## 实际验证

本地与发布包：

- Gateway、Workbench `go test ./...` 与 `go vet ./...` 通过。
- Workbench Node 306/306 通过；变更 JavaScript `node --check` 通过。
- `acceptance/p9-full-composition/test-preflight`、`deployments/d1/ops/test-p9-composition`、发布包校验与 `git diff --check` 通过。
- 新增协调器测试覆盖提前更新、并发 single-flight、短暂失败读降级、写前失败关闭、403 清理和无浏览器凭证存储。

远端与公网：

- 候选配置预检返回 `contracts-valid`；独立 `d1 check` 通过。
- 备份含 21 项 checksum，`sha256sum -c` 通过；数据库指纹存在。
- 21/21 Compose 长运行服务 healthy；运行 Gateway／Workbench 镜像与上述摘要一致，Gateway `FEEDBACK_AUTH_MODE=session`。
- `/` 返回 303 到 `/workbench/?view=oms-v1`，`/health/ready` 返回 ready，Feedback V3 入口返回 200。
- Capability、主运行时和 Feedback 三个公网静态资源与本地产物逐字节 SHA-256 一致；静态 Feedback 脚本未发现 API Key、Bearer、访问 Token 或 Secret。
- 匿名 `POST /gateway/feedback/config` 返回 403 `csrf_failed` 且 `Cache-Control: no-store`。
- Chrome 完整刷新后，登录态 `POST /gateway/feedback/config` 返回 200；页面实际加载 `feedback.feitianchengzi.com/sdk-releases/3.0.4/submit?embed=web`，智能客服、我的反馈、问答输入和直接反馈入口可见；提交快照端点持续返回 200。没有代用户发送问答或反馈。
- 当前普通账号申请 Party／Project／Contract／Billing／Money Capability 均返回 403；部署系统管理员申请 Party Capability 同样返回 403。这证明无授权主体失败关闭，也证明没有为验收破坏系统管理员与业务权限隔离。

## 发布过程异常与恢复

标准 `d1 upgrade` 已先成功创建并校验联合备份。Compose 停栈时，一个两周前遗留的一次性 Cost／Outsourcing workload-key 容器短暂占用默认网络，导致网络删除步骤返回非零；业务卷和备份未删除。随后使用同一 v1.99 候选执行 `d1 up`，完整栈重建成功并通过 `d1 check`、21/21 healthy 和公网检查，无需恢复数据。遗留调试容器 `omsv152-inspect` 只从业务网络断开，没有删除。

## 明确未外推的结论

- 当前没有可用的、带五个 Owner 业务 Grant 的专用公网验收账号；不得把系统管理员临时扩权来凑正向结果。`verify-public-oms-v1.py --login-credentials-file <0600-json>` 已准备好，取得受控业务验收账号后应继续执行有权租约、五 Owner 读取和超过 5 分钟的真实续期旅程。
- 本次没有提交业务事实、没有改变 Grant、Owner API、schema、Capability TTL 或 OMS 登录 Session，也没有执行 Feedback 真实提交。
- 登录态 Feedback 正向与无授权 Owner 负向已通过；15 分钟 Feedback 主动续期、后台恢复和退出后拒绝仍需单独长会话验收，不能由打开面板外推。
