# OA C3 上游精确引用兼容实验（2026-09-27）

## 判定

- 问题 A：Billing 的 Project、Contract、Settlement 和 Money Obligation 类型化来源此前不能同时满足“目标应用精确授权 + 稳定最小摘要”。
- 可验证结果 B：四类来源均由事实所有者通过版本化 reference endpoint 返回 ID、Tenant、摘要、状态和 revision；正式 IAM 凭证只覆盖一个精确资源，相邻资源、错误类型、错误动作和跨 Tenant 失败关闭。
- 本轮判定：`target-reference-compatible`。
- C3 判定：仍为 `insufficient-evidence`。Billing 尚未使用工作负载身份向 IAM 交换这些凭证，也未实现 HTTP resolver、来源快照写入、故障恢复、组合页面、Gateway 或 D1。

本实验关闭的是[前序就绪度审计](./oa-c3-billing-source-reference-readiness-audit-20260927.md)发现的目标端兼容缺口，不把静态／模块级兼容性冒充完整跨应用链路。

## 1. 不能破坏的边界

1. Project、Contract、Settlement 和 Obligation 继续由各自应用拥有和读取；Billing 不访问相邻数据库。
2. 引用响应是最小 read model，不导出 ORM、完整领域对象、金额、主体或证据内容。
3. 普通人类 Tenant Scope `view` 不得替代工作负载 reference action。
4. 每份正式 reference credential 只允许一个 Audience、一个 Action 和一个精确资源。
5. Settlement 是 Contract Ledger 的追加不可变事实，不因对外引用而变成 Contract revision 或 Billing 事实。
6. 旧本地 Demo 的显式静态 reference token 暂时保留为模板 Provider 兼容入口；它不是正式 IAM 通过证据。

## 2. 执行步骤

### 步骤一：固定 Project 最小摘要

- `ProjectReference` 追加 `display_summary`，值来自 Project 自有 `Name`；
- `project-workspace.read + project:{id}` 保持既有版本化授权边界；
- HTTP 层继续在查库前执行 `CanProject(project_id)`；
- 新增 exact Project 成功和 adjacent Project `403` 测试。

这是一项加法响应兼容，不改变 Project、Assignment 或 Resource Link 的所有权。

### 步骤二：分离 Contract 页面查看权与工作负载引用权

- Capability Manifest 新增 `contract-ledger.reference.read`；
- Contract IAM Provider 保留 Tenant Scope `contract-ledger.view` 的人类页面路径；
- resource scope 只接受一个 `contract` 或 `settlement`，且 action 必须唯一等于 `contract-ledger.reference.read`；
- Principal 保存按资源类型分组的精确 ID，reference middleware 在调用用例前同时复验 action、type 和 route ID；
- Contract reference 追加由 Contract 自己提供的标题摘要。

### 步骤三：增加独立 Settlement reference port

- 新增 `GET /api/v1/references/settlements/{settlement_id}`；
- Contract Service／Repository 按 Settlement ID 从 Contract 自有数据库读取不可变结算；
- 响应只返回 Settlement ID、Tenant、`settlement` 类型、标题、`recorded` 和 revision 1；
- `source_revision` 与 `settlement_sequence` 继续作为 Contract 内部历史解释，不伪装成 Settlement 自身可变 revision；
- 不返回结算金额、确认主体、合同详情或 Evidence。

### 步骤四：收紧 Money reference

- Capability Manifest 新增 `money-flow-ledger.reference.read`；
- Money IAM Provider 对 resource scope 只接受一个 `obligation` 或 `transaction`，且 action 唯一；
- Obligation 返回自己拥有的标题摘要；Transaction 返回自己拥有的流水说明；
- route ID 与 Credential resource ID／type 不一致时在查库前返回 `403`；
- Tenant Scope `money-flow-ledger.view` 继续供人类页面使用，但不能访问正式 reference endpoint。

### 步骤五：同步版本化契约和能力目录

- Project、Contract、Obligation、Transaction reference 契约升级为 1.1，明确 `credential_scope` 和 `display_summary`；
- 新增 Settlement reference 1.0 契约；
- IAM Capability Catalog 测试固定 Contract／Money reference action 允许的资源类型；
- Access Contract 验收固定两个新 action 的稳定命名空间。

## 3. 最终契约矩阵

| 事实所有者 | 路径 | 正式 Action | 精确 Scope | 稳定摘要 | revision 解释 |
| --- | --- | --- | --- | --- | --- |
| Project Workspace | `/api/v1/references/projects/{id}` | `project-workspace.read` | `project:{id}` | Project name | 当前 Project revision |
| Contract Ledger | `/api/v1/references/contracts/{id}` | `contract-ledger.reference.read` | `contract:{id}` | Contract title | 草稿 draft version 或正式 current revision |
| Contract Ledger | `/api/v1/references/settlements/{id}` | `contract-ledger.reference.read` | `settlement:{id}` | Settlement title | 不可变记录固定 revision 1 |
| Money Flow Ledger | `/api/v1/references/obligations/{id}` | `money-flow-ledger.reference.read` | `obligation:{id}` | Obligation title | 草稿 draft version 或正式 current revision |
| Money Flow Ledger | `/api/v1/references/transactions/{id}` | `money-flow-ledger.reference.read` | `transaction:{id}` | Transaction description | 不可变记录固定 revision 1 |

## 4. 权限负例

模块级测试直接覆盖：

- exact resource + correct action：允许；
- 同类型 adjacent resource：`403`，目标用例不执行；
- route 是 Contract、Scope 却是 Settlement（或 Obligation／Transaction 互换）：`403`；
- resource scope 携带普通 `view`：IAM Provider 拒绝；
- Tenant Scope `view` 调用正式 reference endpoint：`403`；
- Credential Tenant 与目标应用配置 Tenant 不同：IAM Provider 拒绝；
- 未知资源类型、错误 Audience、过期签名：沿用既有失败关闭验证。

这组结果证明正式 reference 不是“登录后都能看”，而是目标应用自己执行的对象级最终授权。

## 5. 实际运行证据

本轮实际执行并通过：

```bash
go test ./backend/services/project-workspace/internal/auth \
  ./backend/services/project-workspace/internal/projects \
  ./backend/services/project-workspace/internal/httpserver \
  ./backend/services/contract-ledger/internal/contract \
  ./backend/services/contract-ledger/internal/httpserver \
  ./backend/services/contract-ledger/internal/identity \
  ./backend/services/money-flow-ledger/internal/httpserver \
  ./backend/services/money-flow-ledger/internal/identity \
  ./backend/platform/identity-access/internal/capability \
  ./acceptance/access-contract
```

以及三个目标应用的完整普通回归：

```bash
go test ./backend/services/project-workspace/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/money-flow-ledger/...

go test -race ./backend/services/project-workspace/internal/auth \
  ./backend/services/project-workspace/internal/projects \
  ./backend/services/project-workspace/internal/httpserver \
  ./backend/services/contract-ledger/internal/contract \
  ./backend/services/contract-ledger/internal/httpserver \
  ./backend/services/contract-ledger/internal/identity \
  ./backend/services/money-flow-ledger/internal/httpserver \
  ./backend/services/money-flow-ledger/internal/identity

go vet ./backend/services/project-workspace/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/money-flow-ledger/... \
  ./backend/platform/identity-access/internal/capability \
  ./acceptance/access-contract

(cd acceptance/contract-ledger-demo && GOWORK=off go test ./...)
(cd acceptance/money-flow-demo && GOWORK=off go test ./...)
(cd acceptance/project-workspace-demo && GOWORK=off go test ./...)
```

所有普通测试、目标包 race、vet 和三个既有独立 Demo 驱动回归均通过；两个 Capability Manifest 通过 JSON 解析，`git diff --check` 通过。Demo 回归只证明兼容入口未被本轮精确 IAM 模式破坏，没有重新运行完整 fresh-state／run／replay／cleanup。没有运行 PostgreSQL integration tag、真实 IAM exchange、Gateway、浏览器、D1 或恢复验收，因此这些项目不计为本轮通过。

## 6. 对解耦平衡点的证据

本轮变化属于稳定 Base 的加法兼容层：三个事实所有者各自增加最小 read model 和目标端授权，不复制业务表，也不把 Billing 的需求写进其内部领域模型。消费者得到足够的 ID、摘要、状态和 revision 来冻结来源快照，但无法借此读取金额、主体、合同内部结构或核销事实。

这支持当前“尽量只做加法”的方向，但证据只来自一个 OA C3 候选。若后续第二参考产品需要完全不同的摘要、批量查询或跨资源授权，应先检查它是稳定共性、类型化扩展还是客户专项，不能立即扩宽 reference 或退回共享表。

## 7. 下一可执行缺口

下一轮只处理 Billing 消费侧：

1. 为 Billing 建立独立可轮换工作负载身份及 Workload Policy 上限；
2. 逐来源交换单 action、精确资源、最长 60 秒 Credential；
3. HTTP resolver 校验 Tenant、ID、revision、状态和摘要并生成 Billing 自有快照；
4. 覆盖 stale、inactive、adjacent、wrong audience、目标 503、IAM 502 和恢复后单次写入；
5. 在上述链路通过前，不进入 C3 页面、Gateway、D1 或“C3 已完成”的结论。
