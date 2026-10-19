# ADR-0033：统一入口使用 IAM Session 与 Ed25519 单 Audience 短期凭证

日期：2026-09-20  
状态：Accepted

## 背景

Contract 与 Money Flow 当前各自维护密码、Session 和角色。如果 Gateway 只把浏览器身份写进普通 Header，目标应用无法判断 Header 是否伪造；如果每个业务请求都同步调用 IAM，IAM 故障会把所有已登录业务请求变成强耦合；如果所有服务共享 HMAC 密钥，任何验证方都能伪造 IAM 凭证。

## 决策

1. IAM 使用 opaque Suite Session 管理浏览器登录状态，数据库只保存 Session SHA-256。
2. 业务授权使用 IAM 签发的 Ed25519 JWT Capability Credential；私钥只属于 IAM，Gateway 和目标应用只持有公钥。
3. 每个 Credential 只有一个 audience，默认 TTL 5 分钟，并绑定 issuer、tenant、subject、actions、scope、签发／生效／过期时间、认证时间和唯一 jti。
4. Gateway 删除所有客户端身份 Header，对 Token 作基础验签后只转发原始签名 Token；目标应用独立复验并作最终授权。
5. 当前业务请求不逐请求同步调用 IAM。IAM 停止时，已签发凭证只在自身剩余 TTL 内有效，新登录、续签和新授权失败关闭。
6. M7 一个安装实例绑定一个 Tenant ID；现有业务表不因 Token 存在而宣称多租户。
7. 浏览器写入继续使用应用专属 CSRF 防护，JWT 不替代业务幂等、Expected Revision 或领域约束。

## 原因

- 非对称签名保持“只有 IAM 能签发”，同时允许 Gateway 和每个应用独立验证；
- 单 audience 降低凭证被错误投向其他应用的风险；
- 短 TTL 在不增加逐请求 IAM 依赖的情况下限制停用和泄露后的残余窗口；
- 原始 Token 被目标应用复验，比信任 Gateway 生成的普通 Header 更符合纵深防御；
- 保留本地模板 Provider，避免统一入口破坏小应用独立交付。

## 后果

- 必须实现密钥生成、持久化、轮换和公钥分发，并验证 `kid`；
- 注销和停用不是即时撤销，最长残余窗口为 Token TTL；高风险能力若需即时撤销必须另立决策；
- 目标应用需要新增认证 Adapter 与 IAM Actor 类型，但旧 `local_user` 审计不迁移；
- Gateway 和目标应用必须共享契约测试，但不能共享业务数据库或内部领域对象；
- D2～D4 仍使用同一凭证协议，只改变端点和副本配置。

## 被否决方案

### 只信任 Gateway 身份 Header

目标应用无法独立验证来源，一旦网络或代理配置错误就可能把客户端伪造 Header 当成可信身份。

### 每个请求在线 introspection

会把 IAM 变成所有业务请求的同步依赖，不符合已确认的独立工作和故障边界。

### 所有服务共享 HMAC 密钥

任何验证方都拥有签名能力，违反只有 IAM／受信 Policy Provider 可以签发权限结论的约束。

### 长期 JWT 或把 Suite Session 直接给业务应用

长期 Token 扩大撤销窗口；Suite Session 只证明登录状态，不携带目标应用可复验的 audience、action 和 scope。
