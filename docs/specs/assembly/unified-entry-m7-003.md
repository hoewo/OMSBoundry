# M7-003 Nginx、Go Gateway 与 Unified Workbench 最小入口

日期：2026-09-20
状态：implemented and verified

## 1. 最小根因

> 多个服务各自暴露地址、各自解释入口身份 → 公网只进入 Nginx，由小型 Go Gateway 按 Suite Manifest 路由和验证凭证，Workbench 只展示当前启用应用。

## 2. 组件职责

| 组件 | 做什么 | 明确不做什么 |
| --- | --- | --- |
| Nginx | 唯一公网监听、TLS 终止位置、连接与请求体边界、转发到 Gateway | 不登录、不验业务 Token、不认识用户或业务表 |
| Go Gateway | 严格读取 Suite Manifest、静态路由、限流、清除外来身份 Header、登录／授权交换、基础离线验签、访问日志 | 不保存账号密码、不签发权限、不拥有角色、项目或业务数据 |
| Unified Workbench | 展示登录表单、当前用户和 Manifest 中启用的业务应用；提交进入应用请求 | 不读取 JWT、不保存身份、不直接调用业务数据库 |

## 3. 路由与授权

- `/auth/*`：Gateway 去掉 `/auth` 前缀并转发 IAM；
- `/workbench/*`：Gateway 在线向 IAM 解析 opaque Suite Session，仅给 Workbench 注入展示所需 Subject／Tenant／Display Name；
- `/gateway/login`、`/gateway/logout`：Gateway 只转发账号输入和 CSRF 给 IAM，不校验、不保存密码；
- `/gateway/authorize/{application}`：Gateway 用 Suite Session 向 IAM 申请单一 Audience Credential，然后设置 `HttpOnly`、Route Prefix 限定的应用 Cookie；
- 业务路由：Gateway 删除客户端 `Authorization`、`Proxy-Authorization` 和全部 `X-OMS-*`，从对应 Cookie 读取 Token，校验 EdDSA／`kid`／issuer／tenant／audience／时间／Tenant Scope／`entry_action`，只把原始签名 Token 放入内部 `Authorization: Bearer`；
- 禁用应用没有路由、没有 Workbench 菜单，也不能通过手写 URL 到达上游。

`entry_action` 由业务应用在 Suite Manifest 显式声明并属于自身 Audience。BF-082 兼容此单值形式，并允许互斥的有界 `entry_actions` ANY 集合；至少一个原生能力匹配即可入站，但不补授其他能力。Gateway 只做“能否进入应用”的粗门禁；目标应用仍需复验完整 Credential，并针对每个读写动作和资源状态作最终授权。部署必须闭合 Gateway／Workbench／Manifest 消费者兼容性，见 [入口能力契约](./gateway-entry-capabilities-v1.md)。

## 4. 安全与故障行为

- 内存限流以来源地址和一分钟窗口实施，D1 的 HTML、登录／换票和业务 API 默认共用每分钟 120 次动态请求预算；不可变的 `/workbench/assets/*` GET／HEAD 不消耗该业务预算，避免正常业务突发把 CSS／JS 改写成 429 JSON。D4 前需替换或证明跨副本限流语义；
- Workbench 同一页面、同一 Audience 和同一 Scope 的连续来源读取允许在 30 秒页面内存租约中复用已经由 IAM 签发并存于 HttpOnly Cookie 的短期能力；Scope 改变必须串行重新换票。租约不保存凭证、不跳过任何目标 API 请求，目标应用仍逐请求复验签名、Scope、资源和业务状态；
- Gateway 每个请求记录 request ID、方法、路径、状态、响应字节和耗时，不记录 Cookie、Session、JWT 或密码；
- IAM 公钥文件严格解析，active／retiring Ed25519 Key 均可验证；原子替换后下一个请求读取新集合；
- IAM 停止时，已有未过期业务 Credential 仍可离线通过，登录、续签、WorkBench 会话展示和新授权失败；
- 单个业务上游停止只返回该路由 503；其余静态路由继续工作；
- Suite Manifest、Key 文件或必需 IAM 路由无效时 Gateway 拒绝启动或不 ready；
- Workbench 只信任内网 Gateway 清洗后注入的展示 Header；D1 不发布 Workbench 内部端口。

## 5. Nginx 配置

[`deployments/d1/nginx/nginx.conf`](../../../deployments/d1/nginx/nginx.conf) 只定义一个 `gateway:8088` 上游。D1 本地验收监听 8080；生产按 [`nginx-tls.conf.example`](../../../deployments/d1/nginx/nginx-tls.conf.example) 挂载证书并监听 443，业务容器不映射公网端口。

## 6. 已验证结果

- Gateway 单元／HTTP 测试覆盖登录 Cookie 透传、外来身份 Header 清理、可信 Workbench 上下文、Audience 专属 Cookie、原始 Bearer Token 转发、路径去前缀、禁用路由、动态请求限流以及静态资产不消耗业务预算；Workbench 测试覆盖同 Scope 租约复用和 Scope 改变重新换票；
- Credential 攻击测试覆盖篡改签名、错误 issuer／tenant／audience、缺入口 action、过期、超长 TTL 与错误 Scope；
- Nginx 1.28 Alpine 容器内 `nginx -t` 通过；
- 实际 Nginx 容器、实际 Gateway 进程、实际 IAM＋PostgreSQL 和实际 Workbench 组成链路，匿名页面建立 CSRF、登录返回 303、Session Cookie 生效、伪造 `X-OMS-Subject-ID` 被替换为真实管理员、Workbench 只显示 Manifest 启用应用、`/auth/health/ready` 经完整入口返回 ready；
- Gateway 与 Workbench 均在 `GOWORK=off` 下独立测试和 `go vet`。

本阶段结束时尚未让 Contract 或 Money Flow 接受中心 Credential；后续 M7-004、M7-005 已分别完成接入，M7-006 已完成当前组合的 D1 交付验收。
