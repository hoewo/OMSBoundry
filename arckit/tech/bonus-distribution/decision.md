# Bonus Distribution 技术边界决策

状态：✅ 已采用；独立领域／PostgreSQL、受保护 HTTP／IAM、真实来源、人类工作区、正式 D1、未选缺席、联合空目标恢复、恢复后继续写入及 Money 直接跨进程建议已实现；正式 D1 connector、建议引用恢复、单类型最小组合、双 Tenant 与真实 Ubuntu 尚未实现。

## 决策

C7 新建独立 `bonus-distribution` 应用和数据库，在一个部署边界内承载 `business` 与 `production` 两种 Bonus Pool。两类奖金共享稳定聚合结构和历史规则，但使用类型化来源 adapter、独立 Capability 和不同 Project Assignment Policy。

应用只拥有奖金如何形成、按什么策略计算和如何分配。Contract Ledger 拥有客户结算，Money Flow Ledger 拥有公司回款、应付、实际付款、核销和余额，People & Groups 拥有 Person 当前档案，Project Workspace 拥有 Assignment。跨应用只读取版本化快照或提交可独立确认的建议。

参考 OA 的三套并存结构不得直接迁移：

- `bonuses` 独立表只有用户、类型和浮点金额，缺少来源与 revision；
- `financial_records` 把奖金、成本、收付款和票据混在宽表，并允许更新／删除；
- `oa2026_bonus_pools` 每 Project／类型只有一条可覆盖记录，运行时重读当前来源并使用浮点公式。

这些结构只能证明用户旅程和客户公式存在，不能成为目标数据模型。

## 领域模型边界

第一实现至少包含：

- `BonusPoolCurrent`：当前 Project／类型奖池、状态、revision、当前发布引用和未分配余额；
- `BonusPoolRevision`：输入快照、策略 key／version、精确比例、舍入、计算输出、状态、Actor 和时间的完整不可变 revision；
- `BonusAllocationCurrent`：当前奖池 revision 下的分配状态和目标建议引用；
- `BonusAllocationRevision`：Person／Assignment 快照、比例、金额、理由和纠错关系；
- `BonusSourceSnapshot`：Contract Settlement 或 Money Transaction／Allocation 的类型化 ID、revision、金额、币种、状态和 digest；
- `BonusPolicySnapshot`：策略 key、语义版本、参数和舍入方式；
- `BonusPayableSuggestionCurrent`／`Revision`：面向 Money 的来源建议与目标同步状态；
- 技术 `CommandReceipt`／`AuditEvent`：只服务幂等与审计，不作为业务查询宽表。

经营与生产使用同一表结构时，`bonus_type` 是闭集鉴别器，所有来源约束、策略类型和 Capability 必须由领域服务与数据库约束共同校验。不得使用 nullable 宽记录容纳工资、成本、票据或实际付款。

## 精确金额与计算

金额使用整数最小货币单位和显式 ISO currency。比例使用定点十进制或整数基点，策略声明输入尺度、中间精度和最终舍入；任何运行路径不得使用二进制浮点数形成正式结果。

`reference-oa-bonus-v1` 保存公式 `base × ratio ÷ 1.065` 的语义版本和舍入规则。除数、建议比例和区间均属于 Customer Overlay 策略，不成为公共代码的条件分支。策略执行产生规范化 input digest 和 output digest；相同策略版本与输入必须得到相同输出。

发布时冻结来源与策略，不在读取 current 时重新计算。上游 revision 改变时，实时 reference-read 可以标记 drift，但只有显式重算／发布命令产生新 revision。

## 来源端口

类型化端口为：

- Project：active Project、Tenant、revision、稳定摘要；
- Project Assignment：经营／生产负责人和参与者的 ID、revision、角色、有效时点；
- People：active Person、Tenant、revision、最小显示快照；
- Contract Settlement：经营奖金基数所需的 confirmed Settlement、Project、金额、币种和摘要；
- Money Receipt Set：生产奖金基数所需的已确认 Transaction／Allocation 集合、Project、计入金额、币种和摘要；
- Money Payable Suggestion：人员分配的应付建议受理、决定和 Obligation 引用。

每次外部读取都使用 IAM 交换的单 audience、单 action、精确资源短期凭证。目标应用复验 Tenant、资源、revision、状态和摘要。resolver 具有超时、响应大小上限、严格 JSON、一次 401 重换票和 404／422／503 分类；未知金额保持 null，不转换成零。

Money Receipt Set 必须由 Money Flow 以版本化 reference API 汇总并签出 digest，Bonus Distribution 不逐表读取交易或自行重建核销。Settlement 也必须由 Contract Ledger 提供，不能从 Project 或 Billing 反推。

## 聚合与事务

Pool 的 current、完整 revision、全部 Allocation revision、Command Receipt 和 Audit Event 在一个本地事务内提交。发布命令校验 expected revision、来源 digest、策略版本、分配比例与金额守恒，任一失败不产生部分发布。

发布后的 revision 不可更新或删除。更正产生新 revision，取代关系由 stable ID 明确记录；撤回追加状态 revision。数据库 trigger 拒绝历史篡改和物理删除。

同一 Command ID 和请求摘要幂等；不同摘要冲突。成功重放先查本地 receipt，不重新调用外部来源。并发发布只有一个 expected revision 成功。

## 当前实现证据

`backend/services/bonus-distribution` 已实现第一轮领域和数据层：

- `internal/bonus` 定义 `BonusPool`、`BonusAllocation`、类型化 business／production source resolver、Project／Person／Assignment snapshot、Actor 和 Store 端口；
- `internal/overlays/referenceoa` 独立实现 `reference-oa-bonus-v1`，以 `math/big` 计算精确中间值并执行 half-up 舍入，公共核心不识别客户除数；
- PostgreSQL migration 分离 Pool／Allocation current、完整 revision、Command Receipt 与 Audit Event，并用 trigger 拒绝历史更新／删除和 current 物理删除；
- Store 在事务内使用 Tenant＋Command advisory lock、expected revision 与 request digest，实现失败零写入、成功重放、并发单次提交以及发布、取代、撤回的追加历史；
- 服务层在外部复验前先读取成功 receipt，使发布后的原 Command 即使上游离线也能返回原结果，同时新命令仍对来源 drift 失败关闭。

普通单元测试、独立 module 的 `GOWORK=off` test／vet／build，以及 PostgreSQL 18.6 integration race 已通过。真库矩阵覆盖两类来源隔离、精确公式与舍入、分配守恒、Project/action 隔离、来源变化不改写历史、取代／撤回、历史防篡改、依赖失败零写入、恢复与本地重放、不同摘要冲突、八路并发草稿单次提交、并发发布冲突、跨租户隐藏和过量分配拒绝。

第二轮又实现：

- `cmd/server` 与 `/api/v1/bonus-pools`、Pool history、publication、supersession、withdrawal、Allocation history 等版本化入口；
- `bonus-distribution` canonical audience、Ed25519 JWT、严格 issuer／tenant／time／action／resource 验证，以及模板／IAM 配置互斥；
- `bonus-pool`、`bonus-allocation` Capability Manifest 与经营／生产 Project Assignment Policy；
- Project、Assignment、Person、Settlement、Receipt Set 的请求绑定工作负载读取，含超时、严格 JSON、响应上限、一次 401 换票和确定性错误分类；
- Pool current／history、来源总额／条目和 Allocation current／history 的服务端金额脱敏；
- 真实 PostgreSQL 18 加严格 HTTP 来源替身的 protected runtime race：依赖失败零写入、恢复单次提交、成功重放零上游、错 audience／tenant／Project／相邻角色／相邻资源拒绝，以及 401／404／409／422／503。

`bonus-distribution-api.yaml`、`people-person-reference-get.yaml`、`money-receipt-set-reference-get.yaml` 和更新后的 `project-assignment-reference-get.yaml` 固化了本轮端口。People 使用 `people-groups.person.reference.read + person:{id}` 输出最小 Person 投影；Money 使用不可变 Receipt Set 冻结已确认 CNY receipt allocations，并把确认与 `receipt_set:{id}` 精确引用分权。IAM、Project、People、Contract、Money、Bonus 的六进程六数据库组合已经完成两类奖金发布；canonical workload policy 只允许 Bonus 申请四个精确来源权限。

根页面和同源 JavaScript 实现 Project 范围的人类工作台。正式 Gateway 前缀下使用相对请求；本地 Token 只写 `sessionStorage`。collection 与 pool `allowed_actions` 决定草稿、发布、替代、撤回和历史入口；金额 `null` 明确显示未授权。纠错不编辑 published：先创建同类型 draft，再提交旧新 expected revision 的 supersession。自动化六进程验收已覆盖页面资源、无凭证 401、Contract 停机 503／零部分写入、恢复、原子替代、撤回和三条历史链。

第五轮已完成正式 D1：`Dockerfile.bonus-distribution` 构建 Linux/amd64 非 root 镜像，Compose 为 Bonus 配置独立数据库／migration／workload key，Manifest／Workbench／Gateway／IAM Catalog 按开关联动，联合备份与恢复包含 `bonus-distribution.dump`。隔离 D1 的 16 条正式入口断言和 5 条精确工作负载审计无偏差；联合备份 `20260928T143704Z` 从空目标恢复后保持 Pool／Allocation、快照、四份 canonical digest 和历史，并从生产 Pool revision 2 追加撤回到 revision 3。取消选装时路由、菜单、进程、migration、工作负载密钥和 dump 均缺席，重新选装保留数据。

第六轮已完成 Payable Suggestion 直接跨进程协作：C7 使用 `bonus-distribution/bonus_allocation` 来源和独立 Capability，Money intake 支持精确 Allocation 与 Person 快照。People Person 不按名称映射，而由运维命令把 exact Person revision／digest 人工确认到 Money 自有 `individual` Party revision，并保存不可变映射审计。Bonus 在提交 Money 前先保存 `target_unavailable`，目标恢复后原命令可重试；Money 独立接受或拒绝，接受使用确定性子命令只生成一个 `bonus_payable_suggestion` Obligation，拒绝生成零个。源 Allocation 后续撤回不跨库级联。一个临时 PostgreSQL、两个应用进程和独立数据库已通过 11 条 HTTP 断言；正式 D1 connector、建议引用联合恢复和单类型最小组合仍待后续实现。

## Payable Suggestion 协作

Bonus Distribution 为每条已发布 Allocation 产生自己的 Payable Suggestion，冻结 allocation ID／revision／digest、Person 快照、Project、金额、币种、用途和可选到期日。Money 保存独立 intake 并由 Money Actor 接受或拒绝；接受创建唯一 payable Obligation，拒绝不创建。

双方使用独立数据库、独立事务、稳定幂等键和显式同步恢复，不使用共享表、跨库 ORM、双写或分布式事务。网络超时后用同一 Command ID 查询／重试；Money 已提交 Obligation 但 intake 决定未更新时使用确定性子命令恢复。

C6 的 `payable-suggestion-api` 提供已验证的协作形状，但 C7 必须发布自己的 source application、source type、Capability 和请求模型，不能复用 C6 领域对象或让 C6 代发奖金建议。

C7 的来源固定为 `source_application=bonus-distribution`、`source_type=bonus_allocation`。Counterparty 保存 People Person 的 exact ID／revision／digest；Money 在 intake 时通过不可变人工映射解析到自身 `individual` Party，映射不存在、revision／digest 不一致、目标 Party 非 individual 或当前 Party revision 漂移都失败关闭。Ledger Owner 始终是 Money 自有 Party 快照；因此身份档案和财务主体各自保持数据所有权。

## API 与 Capability

canonical audience 为 `bonus-distribution`。第一实现的资源入口围绕：

  - `/api/v1/bonus-pools` 与 `/{id}`；
  - `/{id}/history`、`/{id}/publication`、`/supersession`、`/{id}/withdrawal`；
  - `/api/v1/bonus-allocations/{id}/history`；
- `/api/v1/bonus-payable-suggestions` 与同步／撤回入口；
- `/api/v1/references/projects/{project_id}/bonus-status`。

当前 Capability Manifest 显式声明 `bonus-pool`、`bonus-allocation` 和 `bonus-payable-suggestion` namespaces。经营／生产差异由 action、Project Assignment Policy 和目标端类型校验共同约束，不用客户岗位名或页面路由作授权；Money 目标能力只允许 submit／read／withdraw 应付建议，不授予创建付款或直接写义务的权限。

服务端返回动态 `allowed_actions`。无 `amount.read` 时 current、history、分配和统计金额统一为 null；不能通过写接口盲写不可见的旧金额。Nginx／Gateway 只负责入口，IAM 签发能力，Bonus Distribution 最终授权。

## D1 与独立性

Suite Manifest 启用 C7 时加入独立镜像、数据库账号、migration、Workload Key、Capability Catalog、Gateway 路由、Workbench 入口和联合备份项。未选择时全部缺席且不删除数据。

经营奖金组合显式选择 Contract；生产奖金组合显式选择 Money 作为输入来源。用于应付建议的 Money 目标功能独立开关，不得让既有奖池／历史读取依赖 Money 在线。真实来源独立进程最小组合以及正式 D1 的 Suite Manifest、入口、未选缺席和联合恢复均已通过。当前正式 D1 同时选择 Contract 与 Money，不能冒充经营／生产单类型最小组合；真实 Ubuntu 也尚未通过。

端点全部由配置注入，不绑定 localhost、IP 或 Deployment Profile。D1 保持单节点最低档，D2～D4 使用同一镜像、API、migration 和数据所有权。

## 被拒绝方案

- Money 内置奖金：会把计算原因、人员资格和分配策略误当成已确认义务；
- Project／Production 内置奖金：会污染项目身份或生产阶段内核；
- People 内置奖金：会把人员名册变成薪酬／金额事实所有者；
- 宽 FinancialRecord：类型分支、浮点金额和删除会破坏历史与独立权限；
- 两个奖金类型复制应用：当前生命周期高度相同，没有独立交付反证；
- 运行时任意公式引擎：当前只有一个已确认客户策略，抽象收益不足；
- 用户名或角色名称关联：必须使用稳定 Person／Assignment ID 与 revision。

## Project Completion 与 C8

C7 当前不直接完成 Project。只有获得奖金适用性、发布条件和 not-applicable 对 Project 完成含义的稳定证据后，新的 Completion Profile 才能读取 C7 最小状态投影。GET 纯读，显式确认冻结来源 revision；不能用奖池金额大于零推断项目完成。

人员奖金跨项目统计、公司收入、薪酬统计和待办属于 C8 只读投影／其他责任域。它们只能消费 C7 版本化投影，不能成为 C7 可写表或反向修改 Pool／Allocation。

## 复审条件

真实客户证明经营与生产具有不同审批／税务／部署生命周期；固定金额分配或多角色重复分配形成稳定规则；来源需要跨多个 Contract／Project；奖金进入正式工资代发；或 C7→Money 建议不能表达现实纠错时，以兼容迁移和新决策复审，不静默扩宽当前表。
