# OMS1 P6 员工 API Key 与开放 API 本地验证报告

日期：2026-10-25
状态：本地门禁通过；未部署 D1／公网
决策：BF-241

## 1. A → B

员工若把网页登录态、固定租户参数或长期明文凭证交给 CLI／Skill，就无法在账号停用、权限撤回或工具泄漏时独立止损，也无法安全追踪调用 → IAM 独立拥有绑定 Tenant／Subject／Person 的命名 API Key，Gateway 只把通过当前账号与 Grant 复验的 Key 换成短期有界上下文，Workbench 提供一次展示和独立撤销入口。

## 2. 已交付

- IAM migration `000007_open_api_credentials.sql`：API Key 非秘密元数据、bcrypt verifier、创建／撤销事件、最小调用记录和不可变清理记录；调用记录固定保留 180 天。
- Key 生命周期：名称；30／90／180／365 天且默认 180 天；每员工每租户最多 5 个未到期有效 Key；完整 `bfk_…` secret 只在创建响应返回一次；撤销不可逆，到期自动失效且不占有效名额。
- 当前授权：创建要求 `open-api.key.manage` 与 `open-api.invoice-reimbursement.use`；每次调用重新核对 active Subject、当前 Grant 和 People Person 映射。Key 不保存可越过后续撤权的永久能力。
- IAM HTTP：登录会话＋CSRF 的本人 Key 列表、创建、撤销与本人调用记录；独立 `open-api.tenant-audit.read` 才能查询租户范围。Gateway 公网边界显式阻断 IAM 内部交换路由。
- Gateway：`/open-api/v1/` route／method allowlist；HTTPS 在读取 Key 前强制；外部 Authorization 不向后传递；5 分钟、精确 Key scope 的签名上下文；每 Key 60／分钟、每 Tenant 600／分钟及来源 IP 短期防护；统一错误、`Retry-After`、请求 ID 和脱敏 `/24`／`/64` 网络记录。
- 当前 V1 runtime 仅开放 profile、settings、OpenAPI 规范和中文 `curl` 示例。未知 route 不执行 Key 验证；Webhook 返回并展示 `not_available`，不接受或保存回调地址。
- Workbench：独立“开放平台”入口、本人 Key／使用记录、创建与撤销二次确认、到期提示、一次 secret 复制。secret 不进入存储或全局状态；创建后先展示 secret，再异步刷新列表，避免刷新失败吞掉唯一展示机会。
- 运维：`deployments/d1/ops/d1 purge-open-api-usage OPERATOR_ID` 只对现有 IAM 执行 180 天清理并返回 run ID、cutoff 与删除数，不启动全套服务，也不级联业务事实。

## 3. 关键不变量

- IAM 拥有账号、Grant 与 API Key；People 拥有 Person；Finance／Evidence 拥有报销与文件。P6 没有新增共享业务表或把 Person 档案复制进 IAM。
- Tenant、Subject、Person 和能力全部从已验证 Key 与当前 Owner 引用派生；开放 API 不接受切换身份参数。
- 数据库、列表、日志、审计、OpenAPI 和示例均不得包含完整 secret；调用审计不保存 Authorization、请求／响应正文、发票字段或文件内容。
- IP 不作为员工身份，也不绑定 Key；只保存脱敏网络并用于短期防护。HTTP 在解析认证信息前拒绝。
- Gateway 不能成为任意 Owner proxy；新增业务端点必须显式加入 V1 allowlist，并继续由业务 Owner 最终授权。
- 调用结果审计回写失败只产生清洗后的可观测日志，不反向伪造或撤销已经返回给调用方的业务结果。

## 4. 实际验证

通过的门禁：

1. Identity & Access、Gateway、Workbench 三个 Go module 的低并发 `go test -p 1 -count=1 ./...` 与 `go vet ./...`；
2. `node --check oms_phase1_open_api.js` 与 `oms_phase1_open_api.test.mjs` 六项断言，覆盖 nested IAM 错误、唯一 secret 展示顺序、dismiss-only、默认 180 天与可选 365 天；
3. 一次性 PostgreSQL 18.6 上 `go test -tags=integration ./internal/apikey ./internal/httpserver`，覆盖 migration 7、bcrypt verifier、180 天期限、5 Key 上限、撤销、账号 Grant 回收、Person 停用、本人／租户审计和 180 天清理；
4. Gateway 契约测试覆盖公开文档不含有效 Key、HTTPS 前置、统一 401／429、精确 IAM 交换输入、签名 audience／action／scope、Key／Tenant 限流、来源 IP 限流、审计完成、未知路由不验 Key 和 IAM internal 路由公网 404；
5. Workbench 服务测试覆盖版本化 immutable 资源、开放平台页面与新运行时装配；D1 shell 语法、配置渲染和 capability manifest 继续由仓库门禁校验；
6. 两个临时 PostgreSQL 容器在测试后均停止并删除，验证结束时 `docker ps` 为空，没有遗留本地服务。

## 5. 验收中修复的问题

- 初版创建流程先刷新 Key 列表再展示 secret；若刷新失败，已经生成的 secret 将永远无法再次取得。现改为创建响应成功后立即展示，再异步刷新非关键列表。
- IAM JSON problem 使用嵌套 `error.code`，页面最初只识别顶层 code。现同时解析两种版本化形状并保留服务端可读消息。
- 外层来源 IP 限流原本会返回浏览器 Gateway 的错误结构。现在所有 `/open-api/v1/` 限流路径都返回 V1 统一 envelope 与 `Retry-After`。
- 审计完成回写曾静默吞掉故障。现在记录 request ID、Key prefix 和固定失败原因，同时明确禁止记录 transport error 中可能包含的内部地址。

## 6. 未外推范围

- 没有部署 D1 或 `oatest.antuinfo.cn`，也没有改变线上账号、Grant、Key 或业务数据。
- P6 不提供 Evidence 上传、报销草稿／明细／附件、提交／撤回、本人报销列表／详情或付款确认。它们属于 P7；当前文档也明确提示尚未开放。
- P6 没有开放 API 写命令，因此 BF-221 的业务幂等、BF-222～BF-225 的报销状态与本人查询、BF-230～BF-232 的文件／草稿生命周期和 BF-234 的真实纵向闭环尚未完成。
- 未建立 Webhook、推送、AI／CLI runtime、IP allowlist、通用 API 平台、计费或行为分析能力。
