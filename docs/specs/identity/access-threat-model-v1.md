# M7-001 统一入口与 IAM 威胁模型 v1

日期：2026-09-20  
状态：accepted for M7 implementation

## 1. 保护资产

- 用户密码、邀请／激活凭据和 Suite Session；
- IAM Ed25519 私钥与受信公钥集合；
- Tenant、Subject、Capability 授予和账号状态；
- Contract、Money Flow 的业务事实、幂等命令和不可变审计；
- Suite／Capability Manifest 与路由配置；
- 历史 `local_user` Actor 到 IAM Subject 的解释关系。

## 2. 信任边界

```text
不可信浏览器／公网
  → Nginx（公网 TLS 边界）
  → Go Gateway（路由和基础凭证边界）
  → IAM／Workbench／目标应用（VPC 或容器内网）
  → 各应用独占 PostgreSQL
```

浏览器传入的 Header、Cookie、路径、表单和 Token 都是不可信输入。Gateway 是防护层但不是最终授权者；目标应用必须能够在 Gateway 配置错误或被绕过时拒绝伪造权限。

## 3. 威胁与控制

| 威胁 | 必须控制 | 验收证据 |
| --- | --- | --- |
| 客户端伪造身份 Header | Gateway 先删除 `Authorization`、`Proxy-Authorization`、全部 `X-OMS-*`，再从已验证 Token 构造内部请求 | 带伪造 Header 的请求到达上游时只包含 Gateway 派生值；无有效签名仍为 401 |
| 伪造或篡改 JWT | Ed25519；固定 `alg=EdDSA`；按 `kid` 选择公钥；Gateway 和目标应用都验签 | `none`、HS256、未知 `kid`、改 payload、错签名全部失败 |
| Token 投向错误应用 | 每个 Token 单一 audience；route 与目标应用重复校验 `aud` | Contract Token 访问 Money 返回 401 |
| 跨 Tenant 使用 | Suite Manifest、Gateway、目标应用同时校验 Tenant ID | 错 Tenant Token 在进入业务 handler 前失败 |
| 过期、提前或长寿命 Token | `iat/nbf/exp`、最大 5 分钟 TTL、最多 30 秒偏差 | 过期、未来签发、超过 TTL 上限全部失败 |
| Token／Session 被窃取 | TLS；HttpOnly／Secure／SameSite Cookie；日志脱敏；短期 Token；Session 只存 hash | 日志和错误不含 Cookie／Token；非 HTTPS 生产配置拒绝启动 |
| CSRF | Capability Cookie 之外继续使用应用专属双提交 CSRF；注销和写操作只接受 POST | 缺少、错误、跨应用 CSRF Token 的写请求为 403 |
| 重放业务写入 | Token `jti` 用于追溯，业务命令继续使用 Command ID、Expected Revision 和数据库约束 | 同命令同负载幂等，不同负载冲突；Token 更换不绕过业务幂等 |
| 开放重定向／登录钓鱼 | return path 只允许站内绝对路径，拒绝 scheme、host、`//`、反斜线 | 危险 return 参数回退到 Workbench 根路径 |
| Gateway 绕过 | Gateway／应用只绑定内网；目标应用自己验签和授权，不信普通 Header | 直连目标端口且无有效 Token 仍为 401 |
| Gateway 越权签发 | Gateway 没有私钥，不生成或修改 claims | 运行配置只含受信公钥；签发代码和私钥只存在 IAM |
| IAM 不可用 | 未过期短期凭证可继续；登录、续签、新授权失败关闭 | 停止 IAM 后旧 Token 在 TTL 内按契约工作，过期后拒绝 |
| 账号停用后旧 Token | 停止签发并撤销 Suite Session；显式接受最长 5 分钟残余窗口 | 停用后无法续签；残余窗口和结束时间可审计 |
| 私钥泄露 | 私钥只在 IAM 持久化受限目录；公钥有 `kid`；支持 active／retiring／retired | 轮换期间新旧合法 Token 按窗口验证，retired 后旧 key 失败 |
| Manifest 被篡改 | 文件最小权限；启动严格解析；记录摘要；无效或不兼容时 readiness 失败 | 修改 route、重复 ID、禁用 IAM 或未知 Capability 时拒绝 ready |
| Cookie 冲突或路径越界 | Suite Session 与每个 audience 使用不同 Cookie 名；Capability Cookie 限定 route prefix | Contract Cookie 不随 Money 路径发送，反之亦然 |
| 权限与页面耦合 | Capability 使用业务动作；服务端强制；`allowed_actions` 与命令检查同源 | 隐藏按钮不能赋权，直接构造请求仍被拒绝 |
| 历史 Actor 被冒名合并 | 不按用户名自动合并；旧审计不改写；显式映射 | 同名不同本地 UUID 必须人工确认，冲突时迁移失败 |
| 敏感信息进入日志或导出 | 密码、Hash、Session、JWT、私钥、邀请 Token 禁止记录和导出 | 日志、备份清单和业务导出定向扫描无敏感原文 |

## 4. 密钥轮换

- IAM 持有一个 active 私钥和零个或多个仍在验证窗口内的 retiring 公钥；
- JWT Header `kid` 必须存在且在受信集合中；
- 新 Token 只由 active key 签发；
- retiring key 至少保留“最大 Token TTL + 最大时钟偏差”，之后标记 retired 并从验证集合移除；
- Gateway 和目标应用加载完整公钥集合，配置刷新失败时保留最后一个已验证集合并报警，不接受未知 key；
- M7 不通过网络动态拉取未认证 JWKS；D1 使用由部署包注入和原子替换的受信公钥文件。

## 5. 故障策略

| 故障 | 行为 |
| --- | --- |
| Nginx 停止 | 整个公网入口不可用，内部端口仍不暴露 |
| Gateway 停止 | 应用不可从公网访问；IAM 和业务数据库不损坏 |
| IAM 停止 | 新登录／续签失败；未过期凭证按 TTL 工作 |
| 单个目标应用停止 | 该路由返回 503；其他应用和 IAM 继续工作 |
| Suite Manifest 无效 | Gateway／Workbench 不 ready，不使用部分解析结果 |
| 公钥文件无效 | Gateway／目标应用不 ready，不降级为跳过验签 |
| 数据库不可用 | 对应服务不 ready；不使用缓存身份制造成功 |

## 6. 明确残余风险

- M7 采用最长 5 分钟离线凭证，注销、停用或密码重置不能让已经泄露的 Token 瞬间失效；
- D1 是单节点，宿主机故障会中断全部入口，不属于 HA；
- 当前业务库是单安装 Tenant，不具备共享数据库的行级多租户隔离；
- M7 不接外部 IdP，不覆盖企业 SSO、MFA、设备信任或条件访问；
- Workbench 只是最小壳，不代表完整跨应用交互设计已经完成。

这些风险必须在 UI、运行手册和验收报告中如实说明，不能被“统一登录”字样掩盖。
