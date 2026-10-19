# OA C3 Billing 消费侧精确引用实验（2026-09-27）

## 判定

- 问题 A：目标应用虽已提供精确 reference-read，但 Billing 仍可能使用长期聚合 Token、接受错租户／旧 revision／不可用状态，或在依赖失败后产生部分写入。
- 可验证结果 B：Billing 以独立工作负载 Policy 上限逐引用交换单 audience、单 action、单资源 Credential；类型化 resolver 严格复验响应，并证明失败、恢复和成功重放不会重复写入。
- 本轮判定：`billing-consumer-reference-compatible`。
- C3 判定：仍为 `insufficient-evidence`。Billing 完整 HTTP 用户旅程、用户 IAM／Policy 矩阵、Gateway、页面、D1、备份恢复和 C3 只读组合尚未验收。

## 1. 不能破坏的边界

1. Billing 不读取 Party、Project、Contract、Money 或 Evidence 数据库，也不导入其 ORM／领域对象。
2. 每张正式工作负载凭证只允许一个 Audience、一个 Action 和一个精确资源，不允许 Tenant Scope 或万能聚合 Token。
3. 外部响应只有在 Tenant、类型、ID、revision、可用状态和摘要全部匹配时才能冻结成 Billing 自有不可变快照。
4. 外部依赖失败不能写成功 Command、Registration、revision 或业务审计；既有 Billing 快照仍可读。
5. 成功 Command 的幂等重放必须先于外部调用返回原结果。

## 2. 工作负载权限上限

`contracts/access/v1/examples/workload-identity-policy.json` 新增独立 `billing-invoice` 身份，只允许：

| Audience | Action | Resource type |
| --- | --- | --- |
| Party Directory | `party-directory.reference.read` | `party` |
| Project Workspace | `project-workspace.read` | `project` |
| Contract Ledger | `contract-ledger.reference.read` | `contract`, `settlement` |
| Money Flow Ledger | `money-flow-ledger.reference.read` | `obligation` |
| Evidence Service | `evidence-service.view` | `file` |

Access Contract 测试固定该集合及顺序，不允许误加 Transaction、Tenant Scope、写动作或相邻应用能力。Billing resolver 对共享 `workload-identity.TokenSource` 有编译期接口断言，因此正式运行时沿用已验证的 Ed25519 Assertion、请求摘要绑定、最长 60 秒 Credential 和逐资源缓存失效机制。

## 3. 类型化解析与状态规则

resolver 固定调用目标路径，并用请求中的资源类型／ID获取凭证：

- Party：`/api/v1/references/parties/{id}`；
- Project：`/api/v1/references/projects/{id}`；
- Contract／Settlement：各自 reference endpoint；
- Money Obligation：`/api/v1/references/obligations/{id}`；
- Evidence：`/v1/references/files/{file_id}/versions/{version}`。

第一版可冻结状态是显式白名单：Party `active`；Project `active|pending_completion`；Contract `active|terminated|closed`；Settlement `recorded`；Obligation `open|closed`；Evidence `available`。Evidence response revision 必须等于不可变 File Version。未知状态、空摘要、错类型、错 ID、错 Tenant 和 stale revision 全部失败关闭。

这些状态只表示“该事实当前可作为开票登记来源冻结”，不授权 Billing 修改目标状态，也不把目标状态变成 Billing 生命周期。

## 4. 故障与恢复

- 第一次 `401`：只失效该 audience/action/type/id 的缓存并重新交换一次；第二次失败不循环重试；
- `404`／`422`：`REFERENCE_INVALID`，调用方应修正引用；
- Credential 交换失败、401 重试失败、403、网络、5xx、畸形、尾随 JSON 或超过 1 MiB：`REFERENCE_SERVICE_UNAVAILABLE`，可在依赖恢复后重试；
- 服务先查询本地 Command 结果再解析外部引用；依赖失败时创建次数为 0；恢复后原 Command 成功一次；再次重放时即使依赖重新故障，也不调用 resolver、不重复创建。

## 5. 实际运行证据

本轮验证命令和最终结果在 `docs/testing.md` 的“C3 Billing 消费侧精确引用”条目登记。覆盖范围包括 Billing 全模块普通测试、resolver 与领域服务 race、Access Contract、IAM workload、共享 workload identity、vet、Policy JSON 和 diff 检查。

未运行的项目明确不计入通过：PostgreSQL integration tag、真实 IAM 进程交换、完整 Billing HTTP、浏览器、Nginx、Gateway、D1、备份和恢复。

## 6. 下一可执行缺口

下一轮只建立 Billing 最小 HTTP 用户旅程：把现有领域服务、IAM Provider、类型化 resolver 和 PostgreSQL Store 装入受保护 endpoint，完成允许／相邻角色拒绝、Project B、跨 Tenant、状态、金额字段收敛、依赖 503 无部分写入和恢复后幂等重试。该门禁通过前不进入页面、C3 BFF、Gateway 或 D1。
