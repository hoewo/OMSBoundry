# OA C3 Billing 来源引用就绪度审计（2026-09-27）

> 后续状态：本报告记录的是修改前基线。其发现的 Project 摘要、Contract／Money 精确 Scope、Settlement endpoint 和稳定摘要缺口，已在同日的[C3 上游精确引用兼容实验](./oa-c3-source-reference-compatibility-experiment-20260927.md)中关闭到 `target-reference-compatible`。Billing 消费侧、IAM 实际交换、C3、Gateway 与 D1 仍未因此通过。

## 判定

- 审计问题：现有 Party、Project、Contract、Money Flow 与 Evidence reference-read 是否已经满足 Billing / Invoice 第一版的精确来源验证契约。
- 判定：Party 与 Evidence 已满足精确资源凭证和最小引用要求；Project 已满足精确 Project Scope，但引用响应缺少 Billing 要冻结的显示摘要；Contract 与 Money Flow 仍只接受 Tenant Scope，且引用响应缺少稳定来源摘要；Contract 没有 Settlement reference endpoint。
- C3 当前结论：`blocked-by-reference-contract-compatibility`。Billing 核心和目标端权限基座可继续独立验证，但类型化 `project`、`contract`、`settlement`、`money_obligation` 来源不能进入正式 IAM 纵向闭环。
- OA 整体结论：继续为 `insufficient-evidence`。

## 1. 要回答的问题

Billing 不能通过读取相邻数据库或持有租户级万能凭证来“完成”C3。每个类型化来源必须满足下面四项条件：

1. IAM 只为单一目标 Audience、单一 Action 和精确资源 Scope 签发短期凭证；
2. 目标应用复验 Tenant、Audience、Action、Scope 和资源 ID；
3. reference-read 返回稳定 ID、Tenant、revision、状态和可冻结的最小摘要；
4. Billing 只保存该版本化响应的快照，不复制相邻领域模型，也不在线跨库联表。

本审计通过静态契约与实现交叉核对判断现有服务是否已经满足这些前置条件，不以“存在一个 GET 路由”替代权限和响应兼容性证明。

## 2. 验证方法

1. 读取 Billing 的 `ReferenceResolver`、来源类型和目标技术契约；
2. 核对每个来源应用实际公开的 reference 路径和 JSON 响应；
3. 核对各目标应用 IAM Provider 接受的 Scope 种类、资源类型和 Action；
4. 搜索 Settlement reference endpoint，不能用 Contract reference 或数据库表替代；
5. 检查 Evidence 与 Party 的精确资源拒绝测试；
6. 对每项给出 `ready`、`partial` 或 `not-ready`，并记录禁止采用的绕过方式。

## 3. 直接证据矩阵

| 来源 | 实际 reference endpoint | 精确资源 IAM | Billing 所需响应 | 判定 |
| --- | --- | --- | --- | --- |
| Party Directory | `/api/v1/references/parties/{party_id}` | 支持 `party:{id}` 与 `party-directory.reference.read` | ID、Tenant、kind、legal name、状态、revision | `ready` |
| Project Workspace | `/api/v1/references/projects/{project_id}` | 支持 `project:{id}` 与 `project-workspace.read` | ID、Tenant、状态、revision；当前没有显示摘要 | `partial` |
| Contract Ledger | `/api/v1/references/contracts/{contract_id}` | IAM Provider 只接受 Tenant Scope | ID、Tenant、状态、revision；当前没有显示摘要 | `not-ready` |
| Contract Settlement | 无版本化 Settlement reference endpoint | 无 | Billing 需要 Settlement ID、Tenant、revision、状态和摘要 | `not-ready` |
| Money Flow Obligation | `/api/v1/references/obligations/{obligation_id}` | IAM Provider 只接受 Tenant Scope | ID、Tenant、状态、revision；当前没有显示摘要 | `not-ready` |
| Evidence Service | `/v1/references/files/{file_id}/versions/{version}` | 支持 `file:{id}` 与 `evidence-service.view`，并有相邻 File 拒绝测试 | File ID、Version、状态、显示名 | `ready` |

关键实现证据：

- `backend/services/contract-ledger/internal/identity/iam_provider.go` 明确要求 `scope.kind=tenant` 且资源列表为空；
- `backend/services/money-flow-ledger/internal/identity/iam_provider.go` 使用相同 Tenant Scope 约束；
- `backend/services/contract-ledger/internal/httpserver/reference_handlers.go` 只返回 Contract 的 ID、Tenant、resource type、status 与 revision；
- `backend/services/money-flow-ledger/internal/httpserver/reference_handlers.go` 只返回 Obligation／Transaction 的同类最小字段；
- Contract Ledger 中不存在 `/api/v1/references/settlements/{id}`；
- Project Workspace 与 Evidence Service 已有目标端精确 Scope 校验，Party Directory 另有工作负载凭证和 D1 故障证据。

## 4. 发现的问题

### 4.1 权限粒度不兼容

Billing 如果为 Contract 或 Money Flow 请求精确资源 Credential，当前目标应用会因为 Scope 不是 Tenant 而拒绝。若改发 Tenant Credential，Billing 工作负载将获得读取该 Tenant 全部合同或义务的能力，违反最小权限和“连接器只读取本次业务所需事实”的边界。

### 4.2 响应模型不兼容

Billing 的来源快照要求稳定的 `source_application` 和 `display_summary`。当前 Project、Contract 和 Money reference 响应没有稳定显示摘要。Billing 不能自行跨库补查，也不能把某个当前页面字段或 `resource_id` 拼接值冒充来源业务摘要。

### 4.3 Settlement 端口缺失

第一版来源类型已经声明 `settlement`，但 Contract Ledger 没有对应 reference endpoint。把 Contract revision 当作 Settlement revision 会混淆合同法律事实与结算事实，直接破坏既有数据所有权。

## 5. 禁止的绕过

- 不给 Billing 工作负载签发 Contract／Money Tenant Scope 万能读取凭证；
- 不让 Billing 直接读取 Contract／Money 数据库或复用其 ORM／领域对象；
- 不把 Contract ID、Obligation ID 或前端拼接文本伪装成稳定业务摘要；
- 不用 Contract reference 代替 Settlement reference；
- 不因为手工来源可以运行，就宣称类型化 C3 来源已经通过。

## 6. 收敛后的前置契约

后续实现必须先让目标应用分别提供版本化、精确资源的 reference-read：

- Contract：`contract:{id}` + 稳定只读动作，返回最小 Contract 摘要；
- Settlement：`settlement:{id}` + 独立 endpoint，返回 Settlement 自有 revision、状态和摘要；
- Money Obligation：`obligation:{id}` + 稳定只读动作，返回最小义务摘要；
- Project：保留 `project:{id}`，在兼容版本中增加稳定显示摘要；
- 所有目标端继续自行复验 Tenant、Audience、Action、Scope，并用相邻资源负例证明不会扩大读取范围。

这些是 reference read-model 和授权适配的加法兼容变化，不改变 Contract、Settlement、Money、Project 的事实所有权，也不允许 Billing 成为跨域聚合数据库。

## 7. 本轮范围与后续证据

本轮只确认兼容性事实，没有修改上游应用、没有建立 Billing HTTP server，也没有运行 C3 类型化来源旅程。后续每次仍只关闭一个可验证结果：先补齐上游精确引用契约和负例，再让 Billing HTTP resolver 使用工作负载身份调用，最后进入 Gateway／IAM 签发矩阵、页面、故障、D1 与恢复。

第二阶段的“资质单位 + 施工方”主体关系不得反向修改本轮结论。它将在独立、脱敏的第二客户场景中验证 Party 角色、合同与资金方向是否需要 Overlay 或新的稳定类型，而不是提前写进 OA Base。
