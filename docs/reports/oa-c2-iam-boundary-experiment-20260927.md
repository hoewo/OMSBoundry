# OA C2 IAM 边界与故障恢复实验

日期：2026-09-27  
结论：**C2 的跨租户目标校验、正式账号停用、IAM 不可用、Policy 事实源不可用及恢复审计子命题通过；OA 整体仍为 `insufficient-evidence`**

## 1. 要回答的问题

本实验验证的不是“登录页面能否打开”，而是下面这条安全边界在故障与权限变化下是否仍然成立：

> 已签发的短期 Capability Credential 可以由目标应用离线复验，但账号停用后不能再登录、继续使用 Suite Session 或换取新凭证；IAM 或 Policy 依赖不可用时，显式刷新必须失败关闭并清除旧路由凭证，失败的 Policy 计算不能留下“已签发”审计；恢复后只能重新签发一份新凭证。目标应用必须拒绝签名有效但租户不匹配的凭证。

这是一条有意保留的可用性与安全平衡：业务请求不逐次同步依赖 IAM，已经签发的应用凭证最多保留其签名有效期；任何需要重新授权的动作都立即受账号、Grant、Assignment 和依赖状态约束。

## 2. 明确的失效语义

| 场景 | 已确认行为 | 不允许的行为 |
| --- | --- | --- |
| 账号停用 | Suite Session 立即返回 401；新登录和新授权返回 401；显式授权刷新清除应用凭证 Cookie | 继续登录、继续换证或用旧 Session 获得新权限 |
| 已签发应用凭证 | 目标应用可在签名有效期内离线复验，当前最大残余窗口为 300 秒 | 把短期凭证解释成长效 Session，或超过 `exp` 继续使用 |
| IAM 不可用 | 现有短期凭证仅在自身 TTL 内可用；显式授权刷新返回 502 并清除应用凭证 Cookie | Gateway 保留旧 Cookie 冒充刷新成功，或绕开 IAM 新签发 |
| Project Policy 事实源不可用 | IAM 返回 503 `policy_dependency_unavailable`；不写 `iam_policy_credential_issued` 审计；Gateway 清除应用凭证 Cookie | 把基础设施故障冒充 403 业务拒绝，或留下未实际签发的成功审计 |
| 依赖恢复 | 重新授权成功，并且该恢复动作恰好新增一条签发审计 | 自动重放先前失败的授权或重复签发 |
| 租户不匹配 | 目标应用拒绝签名、Audience、Action 均正确但 Tenant 不同的凭证 | 仅凭有效签名跨租户访问 |

Gateway 只负责路由授权上下文的刷新与清理；目标应用仍独立复验凭证并结合本地状态作最终授权。502 表示 Gateway 无法到达 IAM，503 表示 IAM 已接收请求但其 Policy 事实依赖不可用，两者都不能降级成允许。

## 3. 可复验入口

隔离 Compose project 为 `omsbizfoundry-c2`，唯一宿主入口为 `http://127.0.0.1:18101`。账号、IAM、Project 和 Evidence 的故障注入只作用于该隔离 project：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_COMPOSE_OVERRIDE_FILE=/tmp/omsbizfoundry-c2-networks.yaml \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
C2_ENABLE_DEPENDENCY_FAILURE_TEST=true \
C2_ENABLE_IAM_BOUNDARY_TEST=true \
C2_EVIDENCE_DIR=/tmp/oms-c2-iam-boundary-evidence-20260927 \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

脚本的退出清理会恢复本轮停止的 IAM、Project 和 Evidence 服务，并等待健康检查通过。该入口只应运行于隔离验收环境。

## 4. 直接证据

最新镜像重建后的 C2 正式入口验收结果为：

- **110 个 HTTP 断言，状态不匹配 0，最长本地请求 0.910337 秒**；
- IAM 审计计数为 Policy 31、Project 工作负载 11、Evidence 工作负载 12、撤权 99、账号停用 2；
- 账号停用后 Suite Session、重新登录和新授权均为 401，显式刷新不再保留应用凭证 Cookie；
- IAM 停机时显式刷新返回 502，并清除旧应用凭证 Cookie；恢复 IAM 后重新授权成功；
- Project 停机导致 Policy 事实不可读时，显式刷新返回 503，失败前后 `iam_policy_credential_issued` 数量不变；Project 恢复后重新授权成功，签发审计恰好增加一条；
- 在 IAM／Project 恢复过程中不需要重启 Business Engagement，已经确认的本地业务事实始终可读；
- C1 正式入口随后回归 **152 个 HTTP 断言，状态不匹配 0，最长本地请求 0.530457 秒**。

跨租户目标隔离由 Business Engagement 的目标凭证复验测试直接覆盖：测试使用真实签名流程构造 Audience 和 Action 正确、但 Tenant 与请求租户不一致的 Credential，目标应用返回 `ErrUnauthenticated`。这证明目标校验器不会只看签名或 Capability；它不是一条完整的双租户 D1 浏览器旅程。

## 5. 实现收敛

- IAM 将 Policy 事实 HTTP 客户端的网络错误、非成功状态和无效响应归类为显式依赖不可用，并在 API 层稳定映射为 503 `policy_dependency_unavailable`；401／403／404／422 仍保持授权拒绝语义。
- Gateway 在 IAM 请求构造失败、网络失败、非成功响应或无效成功载荷时都清除现有应用 Capability Cookie，避免页面把旧权限上下文误认为刷新结果。
- C2 正式入口验收增加显式开关，默认不破坏普通回归；启用后验证账号停用、IAM 停机、Project Policy 依赖停机、审计不变量与恢复后单次签发。
- 目标应用增加有效签名但错误 Tenant 的回归，继续由目标应用而非 Gateway 承担最终隔离。

## 6. 局限与下一步

本实验关闭了 C2 当前最重要的 IAM 故障边界，但不等于“跨租户完整矩阵已经通过”。当前直接证据是目标应用的签名凭证错租户拒绝，以及单租户隔离 D1 中的账号／IAM／Policy 故障旅程；尚未运行两个真实 Tenant、两套资源和浏览器会话的完整交叉矩阵。

本实验也不证明真实 Ubuntu／公网 HTTPS、C3～C8 权限旅程、字段／金额／更广文件动作、长期撤权传播、高可用 IAM 或 OA 整体架构可行性。OA 总结论继续保持 `insufficient-evidence`。
