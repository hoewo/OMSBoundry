# Cost & Outsourcing 技术边界决策

状态：✅ 已采用；领域核心、正式入口、精确引用、人类工作区、正式 D1 装配／恢复、Party 运行时切换及 C6→Money Payable Suggestion 接受／拒绝／恢复已验证；Money 完整 D1 消费链、完整双 Tenant 及真实 Ubuntu 待补证。

## 决策

C6 新建独立 `cost-outsourcing` 应用和数据库，在同一部署边界内实现 `CostEntry` 与 `OutsourcingEngagement` 两个聚合。共享基础设施不等于共享领域表：每个聚合拥有独立 current、revision 和类型化命令；同一应用可以共享只含 Actor、摘要、响应类型和响应快照的技术 Command Receipt／Audit Event 表，但其中不得出现可被业务查询依赖的宽领域字段。

不复用参考 OA 的 `FinancialRecord`，也不把 C6 加入 Production、Project、Contract、Money 或 Billing 数据库。该决策是当前一个参考场景下的最小边界；未来可通过兼容 API 和数据迁移拆分聚合，但不得提前共享 ORM 或跨库写入。

## 不变量

- 独立 PostgreSQL 数据库、账号、migration、备份和恢复边界；
- 金额统一整数最小货币单位和显式币种，里程使用整数米；
- current 与完整 revision 在同一事务提交，历史禁止更新／删除；
- 所有写命令带 Command ID，修改带 expected revision；
- 外部引用冻结类型、ID、revision、状态和稳定显示快照；
- Party、Project、Contract、Evidence 使用单 audience、单 action、单精确资源工作负载凭证；
- 目标应用复验 Tenant、类型、ID、revision、状态和摘要，失败零部分写入；
- 成功重放先于外部调用命中本地结果；
- C6 只创建 Payable Suggestion，Money 才能确认 Obligation 和实际付款；
- Contract、Money、Billing 的后续变化不会静默改写 C6 历史；
- 撤回 C6 事实不能级联删除其他应用事实。

## 目标模型

第一版至少包含：

- `CostEntryCurrent`／`CostEntryRevision`；
- `OutsourcingEngagementCurrent`／`OutsourcingEngagementRevision`；
- `OutsourcingAssessmentRevision`；
- `PayableSuggestionCurrent`／`PayableSuggestionRevision`；
- `ExternalReferenceSnapshot`：Project／Party／Contract／Money／Billing 的最小冻结投影；
- `EvidenceReference`：File ID／Version、摘要、slot 和加入 revision；
- 应用级技术 `CommandReceipt` 与 `AuditEvent`；只服务幂等／审计，不作为跨聚合业务读模型。

数据库约束已经保证 Tenant 复合唯一性、revision 单调、金额范围、评分 1～5、current 与 matching revision 一致、Command ID 请求摘要一致和历史不可变；withdrawn 后非法动作同时由领域服务和事务内 current 检查拒绝。实现证据见 [C6 领域核心实验](../../../docs/reports/oa-c6-core-experiment-20260928.md)。受保护 HTTP、IAM、Scope 与 adapter 的直接证据见 [C6 受保护 HTTP、IAM 与精确引用实验](../../../docs/reports/oa-c6-protected-http-reference-experiment-20260928.md)。

`PayableSuggestionCurrent` 与 Money 的 `PayableSuggestionIntake` 分属两个数据库。C6 current 从 `target_unavailable` 经目标接收成为 `proposed`，再只同步为 `accepted|rejected|withdrawn`；Money intake 从 `pending` 进入 `accepted|rejected|withdrawn`。接受分支以确定性子命令在 Money 内创建并确认唯一 Obligation，拒绝分支不创建 Obligation；C6 只保存目标 revision、决定理由和可选 Obligation 引用。模型与接口见 [`PayableSuggestion.yaml`](../_shared/models/PayableSuggestion.yaml)、[`PayableSuggestionIntake.yaml`](../_shared/models/PayableSuggestionIntake.yaml) 和 [`payable-suggestion-api.yaml`](../_shared/contracts/payable-suggestion-api.yaml)。

正式 D1 已把 C6 装入独立镜像、数据库、migration、Workload Key、Suite Manifest、Workbench、Nginx／Gateway 路由和联合备份恢复。57 个正式入口断言、Evidence／Party 故障零写入、精确 IAM workload audit、未选装缺席、空目标恢复和恢复后继续写入已经通过；直接证据见 [C6 正式 D1 装配、故障与恢复实验](../../../docs/reports/oa-c6-d1-formal-composition-recovery-experiment-20260928.md)。

## 引用端口

- Project：active Project、Tenant、revision、稳定摘要；
- Party：active provider、Tenant、kind、revision、legal name；
- Contract／Settlement：可选法律依据、Tenant、revision、状态、币种、金额与摘要；
- Evidence：available File Version、Tenant、version、revision、摘要；
- Money：建议目标确认、Obligation／Transaction 最小投影；
- Billing：Invoice Registration 最小投影。

resolver 必须有超时、响应大小上限、严格 JSON、一次 401 重换票和错误分类。404／错类型／stale／不可用业务状态映射为引用无效；换票失败、403、网络、5xx、畸形或超大响应映射为依赖不可用。未知金额保持 null，不转成 0。

## API 与 Capability

canonical audience 为 `cost-outsourcing`。第一版 API 分开发布：

- `/api/v1/cost-entries`
- `/api/v1/cost-entries/{id}`、`/history`、`/corrections`、`/withdrawal`
- `/api/v1/outsourcing-engagements`
- `/api/v1/outsourcing-engagements/{id}`、`/history`、`/corrections`、`/withdrawal`、`/assessments`
- `/api/v1/payable-suggestions`
- `/api/v1/payable-suggestions/{id}`、`/history`、`/synchronization`、`/withdrawal`
- `/api/v1/references/projects/{project_id}/cost-outsourcing`

Capability Manifest 使用互斥的 `capability_namespaces` 显式声明 `cost-entry`、`outsourcing-engagement` 与 `outsourcing-payable-suggestion`。单 namespace 应用继续使用 `capability_namespace`，既有未声明应用继续回退 application ID。不得把动作伪装成 audience 前缀或为获得多个 namespace 拆成共享数据库的多个应用。

人类请求使用 Project／资源 Scope；服务端返回动态 `allowed_actions`，金额字段无权时为 null，Evidence 动作另行判权。该矩阵已经在目标应用与真实 PostgreSQL 18 上通过 Tenant、Project、精确 Cost／Engagement、相邻资源、金额与 Evidence 能力组合验证。Nginx／Gateway 只做入口边界，IAM 签发能力，C6 最终授权。

## Payable Suggestion 连接器

连接器提交来源种类、来源 ID／revision／digest、双方 Party 快照、金额、币种、到期日、用途、建议 ID 和请求摘要。Money 先返回 `pending` intake，后续由拥有 `payable-suggestion.decide` 的 Money Actor 独立接受或拒绝；接受返回 Obligation ID／revision，拒绝不返回 Obligation。C6 保存目标决定引用但不复制义务状态机。网络超时后使用同一 Command ID 重试，目标端必须幂等；Obligation 已提交而 intake 决定尚未提交的部分失败使用确定性子命令恢复，不能创建第二条义务。

第一轮不自动发送：用户显式选择来源 revision 并提交建议。批量、自动审批、部分接受和反向冲销保持未知，不能用数据库事务跨服务伪造原子性。

## D1 与独立性

Suite Manifest 启用 C6 时加入独立镜像、数据库账号、migration、Workload Key、Capability Catalog、Gateway 路由、Workbench 入口和联合备份项。未选择时全部缺席且不删除数据。

最低登记组合只需要 Project、Party 与 Evidence；Contract、Money、Billing 通过功能开关和页面动作显式加入。任何依赖停机只降级对应新命令或实时投影，既有冻结历史和不依赖该来源的命令继续可用。

Contract 与 Money 各自通过来源自有、追加不可变的 Party 运行时转换账切换 Directory 权威；切换前要求全部已引用本地 Party 存在经目标 revision 复验的映射，回滚只恢复本地兼容读取且不恢复本地写权。C6 不按名称映射，也不替来源应用隐式转换 ID。Payable Suggestion 提交时 Money 使用自身转换账把 Directory Party revision 解析到本地账簿 Party，并把双方 ID／revision 同时冻结在 intake 中。

## 被拒绝方案

- 宽 `FinancialRecord`：混合类型、可变字段、粗权限和级联删除，无法保持领域含义；
- Production 内置：污染 Work Item／Stage／Deliverable 边界；
- Money 内置：把来源原因和外委范围误当成已确认义务或付款；
- 客户自由表单／流程引擎：当前没有稳定审批状态或第二使用点；
- 名称关联：Party 身份和历史必须使用稳定 ID／revision。

## Project Completion

C6 当前不进入 `reference-oa-c5-v1` 或新的完整完成 Profile。只有取得成本／外委适用性、审核完成和 not-applicable 的稳定业务证据后，Project 才可通过新的版本化 Profile 读取最小 C6 投影。GET 必须纯读，显式确认冻结来源 revision；不能以“存在一条成本或外委记录”推断完成。

## 复审条件

与产品规格相同：独立交付反证、稳定审批样本、两个聚合的部署要求分化、义务建议无法满足真实流程，或明确的 Project Completion 规则出现时，登记新决策并兼容演进，不能静默改表。
