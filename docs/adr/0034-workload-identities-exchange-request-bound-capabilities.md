# ADR-0034：服务间访问使用工作负载身份交换请求绑定的短期能力凭证

日期：2026-09-26  
状态：Accepted

## 背景

Project Workspace 在建立 Assignment 时必须验证 People & Groups 的 Person／Group 引用。把长期 Bearer Token 写入环境变量，会把人工签发、秘密轮换和进程重建变成生产前提；让 Project 共用用户 Session、IAM 私钥或 People 数据库则会破坏身份责任、数据所有权和独立部署边界。

OA 权限不只有“人访问应用”。IAM／Policy Provider 在读取版本化业务事实，以及应用验证跨域稳定引用时，也需要可审计、可轮换、最小化的服务身份链路。

## 决策

1. 每个需要服务间访问的应用使用独立工作负载身份和 Ed25519 密钥。私钥只挂载到调用应用；IAM 只读取版本化公钥集合。
2. 调用方为每次交换生成最长 60 秒的一次性 JWT Assertion。Assertion 固定 `aud=identity-access`、`purpose=capability-exchange`，并用 SHA-256 绑定规范化的目标 audience、单一 action 和精确 Resource Scope。
3. IAM 验证签名、Tenant、时间、请求摘要和部署层 Workload Policy，并把 `jti` 写入 PostgreSQL 防止重放；越权请求失败关闭。
4. IAM 只签发最长 60 秒、单 audience、单 action、精确资源的现有 Capability Credential。目标应用继续独立复验并作最终授权。
5. 调用方可以在内存中按 audience/action/resource 缓存未接近过期的凭证；收到目标应用 `401` 时失效并最多重换一次。IAM 或目标应用不可用时，新跨域命令失败，已有本地事实仍可读。
6. 密钥轮换采用 active／retiring 公钥重叠窗口。调用进程每次签名重新读取私钥文件，IAM 每次验证重新读取公钥集合，因此轮换不要求重启业务进程。
7. D1 用私有／公开两个独立卷落实密钥放置；D2～D4 仍使用同一协议，只改变安全分发和节点放置，不改变 API、身份、Capability 或数据所有权。

## 原因

- 工作负载能证明“哪个应用在请求”，IAM 仍是最终能力凭证的唯一签发者；
- 请求摘要和一次性 `jti` 防止已签断言被换目标或重放；
- 精确资源与一分钟 TTL 限制泄露、停用和策略变化后的影响面；
- 公私钥分离避免把 IAM 变成调用应用签名秘密的保管者；
- 动态交换消除长期 Bearer Token、人工续签和为换 Token 重建应用进程。

## 后果

- 部署必须为每个工作负载生成、备份和轮换私钥，并把对应公钥安全提供给 IAM；
- IAM 数据库新增一次性 Assertion 记录和 `service_identity` 审计 Actor；
- 工作负载策略必须随部署版本受控，不能由业务请求动态扩大；
- IAM 故障不影响已有业务事实读取，但会阻止需要新跨域引用验证或新策略求值的命令；
- 该机制解决服务身份和凭证生命周期，不替代 C1～C8 用户角色矩阵、Business Fact Policy 或目标应用最终授权。

## 被否决方案

### 长期静态 Bearer Token

需要人工签发、环境变量分发和进程重建，泄露窗口长，也无法可靠区分不同调用工作负载。

### 共享用户账号或 Suite Session

把机器访问伪装成人类登录，审计、停用和最小权限边界不清晰。

### 把 IAM 私钥交给业务应用

任何调用应用都能伪造其他 Subject 的能力凭证，违反只有 IAM／受信 Policy Provider 签发权限结论的约束。

### 每个目标应用自建另一套服务账号密码

会重复身份生命周期并使统一审计、轮换和最小策略分散到每个小应用。
