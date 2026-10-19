# M7-001 IAM 与 Capability 契约 v1

日期：2026-09-20  
状态：accepted for M7 implementation

## 1. 最小根因

> 浏览器、Gateway、IAM 和目标应用如果各自解释身份与权限 → 用可签名、可复验、带明确 audience／action／scope 的版本化契约传递同一个授权事实。

## 2. 契约组成

| 契约 | 所有者 | 用途 | 规范文件 |
| --- | --- | --- | --- |
| IAM Context v1 | 目标应用接入层从已验证凭证派生 | 向业务代码提供稳定可信 Actor、Tenant、Actions 与 Scope | [`iam-context.schema.json`](../../../contracts/access/v1/iam-context.schema.json) |
| Capability Credential v1 | IAM | 以短期 Ed25519 JWT 向一个目标 audience 证明 Subject 的有界动作 | [`capability-credential-claims.schema.json`](../../../contracts/access/v1/capability-credential-claims.schema.json) |
| Capability Manifest v1 | 目标应用 | 发布本应用支持的稳定业务动作、风险和资源类型 | [`capability-manifest.schema.json`](../../../contracts/access/v1/capability-manifest.schema.json) |
| Suite Manifest v1 | 装配层 | 声明安装／启用应用、静态路由、上游、健康端点和 audience | [`suite-manifest.schema.json`](../../../contracts/access/v1/suite-manifest.schema.json) |
| Workload Assertion v1 | 调用应用 | 用独立 Ed25519 私钥证明工作负载身份，并把一次性断言绑定到本次交换请求 | [`workload-assertion-claims.schema.json`](../../../contracts/access/v1/workload-assertion-claims.schema.json) |
| Service Credential Request v1 | IAM | 用断言交换单 audience、单 action、精确资源的短期 Capability Credential | [`service-credential-request.schema.json`](../../../contracts/access/v1/service-credential-request.schema.json) |
| Workload Public Key Bundle／Policy v1 | 调用应用／部署层 | 分离调用方私钥与 IAM 公钥，限制工作负载可申请的能力上限 | [`workload-public-key-bundle.schema.json`](../../../contracts/access/v1/workload-public-key-bundle.schema.json)、[`workload-identity-policy.schema.json`](../../../contracts/access/v1/workload-identity-policy.schema.json) |

JSON Schema 固定结构，Go conformance test 固定跨字段语义。未知 `schema_version`、未知字段、重复 ID、未知 capability、Tenant 不一致和重叠路由全部拒绝。Capability Manifest 可以互斥地声明单个 `capability_namespace` 或多个不重复的 `capability_namespaces`；未声明时兼容回退 application ID。Gateway 与 IAM 对任一声明 namespace 执行同样的 Catalog 所有权校验，不能用 audience 前缀替代。

## 3. 安装 Tenant 边界

M7 使用一个部署实例绑定一个 Tenant ID：

- Suite Manifest 保存安装 Tenant ID；
- IAM 只为该安装中的 Subject 签发该 Tenant 的凭证；
- Gateway 验证凭证 Tenant 与 Suite Manifest 一致；
- Contract 与 Money Flow 验证凭证 Tenant 与自己的 `TENANT_ID` 配置一致；
- 当前业务表不增加 tenant 列，也不宣称同一数据库承载多个企业的数据隔离。

未来如果真实客户要求共享实例多租户，必须先给每个业务事实定义 Tenant 所有权、迁移和查询隔离，不能仅靠 Token 字段冒充多租户。

## 4. 会话与凭证流

```text
Browser → Nginx → Gateway → IAM login
IAM 建立 opaque Suite Session（数据库只保存 SHA-256）
Browser 进入某个启用应用
Gateway/IAM 授权入口根据 Suite Session 与 IAM 授予签发该 audience 的短期 Capability Credential
Browser 以 HttpOnly Cookie 携带 Credential
Gateway 删除客户端 Authorization 与 X-OMS-* Header，验证签名与基础声明
Gateway 把原始已验证 JWT 放入内部 Authorization Header
Target App 再次验证签名、issuer、tenant、audience、time、action、scope
Target App 派生 IAM Context，并结合资源状态与领域不变量最终授权
```

实际业务请求从 Gateway 直接到目标应用，不同步调用 Project、People & Groups 或 IAM。IAM 不转发业务请求。

## 5. Token 与 Cookie 决策

### Suite Session

- Cookie 名：`oms_suite_session`；
- 32 字节密码学随机值，数据库只保存 SHA-256；
- `HttpOnly`、`Secure`、`SameSite=Lax`、`Path=/`；
- 默认绝对 TTL 12 小时；注销、账号停用、密码重置或 session generation 变化立即失效；
- Session 只证明登录状态，不能直接作为业务应用授权凭证。

### Capability Credential

- 载体：JWT/JWS Compact Serialization；
- 算法：Ed25519（`alg=EdDSA`），Header 必含 `typ=JWT`、`kid`，禁止 `none` 和算法回退；
- 一个 Token 只有一个 `aud`；
- 默认 TTL 5 分钟，允许时钟偏差最多 30 秒；
- 必含 `iss/sub/tid/aud/name/actions/scope/iat/nbf/exp/auth_time/jti/ver`；
- 每个 audience 使用独立 Cookie 名和 route-prefix Path，Cookie 同样为 `HttpOnly`、`Secure`、`SameSite=Lax`；
- Gateway 只能转发原始签名 Token，不能修改 claims 或自行签发；
- 目标应用只接受自己当前或兼容 Manifest 中存在的 action。

注销和账号停用后，IAM 不再签发新凭证；已经签发的凭证最多继续到 5 分钟 TTL 结束。M7 不建立在线逐请求撤销表。需要即时撤销的高风险能力必须在后续决策中增加 introspection 或更短专用凭证，不能暗称已经即时撤销。

### 服务间工作负载凭证

服务间访问不复用用户 Session，也不把长期 Bearer Token 写入环境变量。调用应用持有独立 Ed25519 私钥，IAM 只持其公开公钥集合和部署层 Workload Policy：

```text
Calling App
  → 生成最长 60 秒、一次性、请求摘要绑定的 Workload Assertion
  → POST IAM /api/v1/service-credentials
IAM
  → 验证工作负载签名、Tenant、时间、请求摘要、jti 重放和 Policy 上限
  → 签发最长 60 秒、单 audience、单 action、精确 Resource Scope 的 Capability Credential
Calling App
  → 调用目标应用；目标应用按普通 Capability Credential 独立复验并最终授权
```

- Assertion 固定 `aud=identity-access`、`purpose=capability-exchange`；`iss` 是稳定 Workload ID，`sub` 是服务身份 UUID；
- `request_sha256` 绑定规范化 `{audience, action, scope}`，已经签名的 Assertion 不能替换目标能力或资源；
- `jti` 在 IAM PostgreSQL 中一次性消费，重放返回 `409`；签名／时间／租户错误返回 `401`，超过 Workload Policy 上限返回 `403`；
- 请求只接受 `resources` Scope，最多 32 个资源；当前 Project→People 引用验证每次只请求一个 Person 或 Group；
- 调用方可以缓存未接近过期的凭证，目标应用返回 `401` 时失效并最多重换一次；IAM／目标应用不可用时，新跨域命令失败关闭，既有本地事实读取不依赖它们在线；
- 工作负载私钥文件权限必须为 `0600`。轮换时新 key 为 `active`，旧 key 在短重叠窗口标为 `retiring`；调用方每次签名、IAM 每次验签都重新读取各自文件，因此不要求重启业务进程；
- IAM 为每次成功交换写入 `service_identity` Actor 的不可变审计，包含 Workload ID、Key ID、Assertion ID、目标 audience/action/scope，但不记录私钥、Assertion 或最终 Token 原文。

人类用户凭证默认最长五分钟；工作负载交换凭证上限为一分钟。两者使用同一目标应用 Capability Credential Claims 与最终授权规则，但身份建立方式、TTL 和审计 Actor 不同。完整决策见 [ADR-0034](../../adr/0034-workload-identities-exchange-request-bound-capabilities.md)。

## 6. Scope v1

Scope 有两种形态：

- `tenant`：只对配置的安装 Tenant 生效，`resources` 必须为空；
- `resources`：每项包含稳定 `type + id`，至少一项，不允许重复。

IAM 支持两类来源明确的授权：Tenant Grant 可以签发 Tenant Scope，也可以由请求方主动收窄为目标应用 Manifest 声明的 Resource Scope；Resource Grant 只能为显式授予的资源签发 Resource Scope，不能在省略 scope 时提升为 Tenant Scope。单次 Resource Scope 最多包含 32 个不重复资源，Credential 只保留对全部请求资源均成立的 action；结果为空时返回 `403`。

直接 Resource Grant 用于 Project 未安装时的明确资源授权和策略链路的最小验收，不冒充 Business Fact Policy。Project Assignment、Resource Link 或客户岗位产生的动态范围仍必须经受信 IAM 授权模块或 Policy Provider 求值，并输出同形 Credential；浏览器、Gateway、Workbench 和业务事实应用不能自行扩大 scope。

## 7. Gateway 与目标应用校验顺序

Gateway 基础校验：

1. 删除所有客户端 `Authorization`、`Proxy-Authorization` 和 `X-OMS-*` Header；
2. 从当前 route prefix 对应的 HttpOnly Cookie 取 Token；
3. 只接受配置的 issuer、`kid` 和 EdDSA；
4. 校验 `tid`、`aud`、`iat/nbf/exp`、`jti` 及 Tenant／Resource Scope 基本格式；
5. 确认 route 所需 action 出现在 Token 中；
6. 将同一个原始 Token 作为内部 `Authorization: Bearer` 转发并记录不含 Token 的访问日志。

目标应用必须独立重复 3～5，并额外：

- 校验 action 属于自己的 Capability Manifest；
- 校验 Tenant 与应用配置一致；
- 校验资源属于 Scope；
- 校验本地状态、revision、金额、幂等与其他领域不变量；
- 把 `sub/name/tid/jti` 派生为可信 IAM Actor，不读取普通身份 Header。

## 8. CSRF 与浏览器写入

Capability Cookie 不能代替 CSRF 防护。Contract 与 Money Flow 保留应用专属双提交 CSRF Token；所有状态变更继续要求安全 HTTP 方法、CSRF 校验和业务幂等／并发门禁。登录、授权和注销入口只接受站内相对 return path，拒绝绝对 URL、`//` 和反斜线变体。

## 9. Manifest 兼容与启停

- `schema_version=1` 只接受显式支持的字段，未知版本失败；
- Capability ID 一经发布不改含义；废弃时先停止授予，再经过兼容窗口移除；
- Suite Application ID、Audience 和 Route Prefix 在同一清单内唯一；每个业务应用显式声明属于自身 Audience 的 `entry_action`，Gateway 不从页面或命名习惯猜测入口权限；
- `identity-access` 必须 `required=true` 且 `enabled=true`；
- 禁用应用会移除 Gateway 路由和 Workbench 菜单，但不删除数据库、文件、Capability Manifest 或历史授权审计；
- 应用重新启用前验证 release version、schema、健康端点和 Capability Manifest 兼容性。

## 10. 失败语义

| 情况 | 结果 |
| --- | --- |
| 无 Suite Session／Capability Cookie | GET 跳转 IAM 授权入口；非安全重放请求返回 401 |
| Token 格式、签名、issuer、tenant、audience 或时间错误 | 401，不向上游转发 |
| 已认证但缺 action／scope | 403 |
| action 存在但目标资源状态不允许 | 409 或领域既有错误，不伪装为权限不足 |
| IAM 不可用 | 已签发未过期凭证继续按自身有效期；登录、续签和新授权失败关闭 |
| Manifest 无效或不兼容 | Gateway／应用拒绝 ready，不带病接流量 |

## 11. Actor 规则

- 中心模式新事实使用 `actor_kind=iam_subject`；
- `actor_id` 是不可变 IAM Subject UUID，`actor_display_name` 是操作当时快照；
- `tenant_id` 和 Capability Credential `jti` 进入新增认证审计或业务审计 details；
- 旧 `local_user` Actor 保持原值，通过显式关联解释，不回填改写；
- Gateway 日志使用 request ID、tenant、subject、audience 和结果，绝不记录密码、Session、JWT 或 Cookie 原文。

## 12. 契约样例

有效样例位于 [`contracts/access/v1/examples/`](../../../contracts/access/v1/examples/)。负例由 `acceptance/access-contract` 固定，至少覆盖未知字段、缺 issuer、重复 Capability、错误前缀、IAM 被禁用、Tenant Scope 带资源、Resource Scope 为空、过长 TTL、错误 audience 和过期时间。
