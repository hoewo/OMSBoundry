# 奖金形成与分配生命周期

## 元信息

- 路径：`spec/bonus-distribution/bonus-lifecycle.md`
- 所属模块：`bonus-distribution`
- 状态：🟡 开发中；独立领域／PostgreSQL、受保护 HTTP／IAM、真实来源、人类工作区、正式 D1、未选缺席、联合空目标恢复、恢复后继续写入及 C7→Money 直接跨进程协作已验证；单类型最小组合、建议引用联合恢复、双 Tenant 与真实 Ubuntu 仍待验证
- 优先级：P0
- 依赖能力：IAM、Project Workspace、People & Groups；经营奖金读取 Contract Settlement，生产奖金读取 Money Flow 已确认回款；Money Flow 仅在提出奖金应付建议时加入
- 上层旅程：[OA 完全体 C7](../assembly/full-reference-oa-validation.md#c7奖金计算与分配)

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 同时用可变奖池、独立 Bonus 和宽 FinancialRecord 表达奖金，计算输入、策略版本、人员资格、分配结果和实际付款互相覆盖，无法稳定说明某人为什么得到这笔奖金以及来源变化后如何纠正。

**可验证结果 B**：用户可以在独立 Bonus Distribution 应用中依据已确认来源建立经营或生产奖池，冻结计算策略与输入快照，按当时的项目责任人员发布可追溯分配；Money Flow 独立确认应付和实际付款，来源变化、撤销、重算、拒绝和依赖故障均不会改写既有历史。

## 产品边界

第一版使用一个独立 Bonus Distribution 应用承载经营奖金与生产奖金。两类奖金共用“来源快照 → 计算草稿 → 发布奖池 → 人员分配 → 撤销或取代”的稳定生命周期，但使用不同来源端口和独立 Capability。出现不同审批、税务、容量或独立交付证据时，可以通过兼容契约拆分；当前不复制两个应用。

应用拥有：

- 奖金类型、适用性、计算策略引用和计算结果；
- 来源事实在计算时的精确 ID、revision、金额、币种和摘要快照；
- 奖池 current、不可变 revision、发布与取代关系；
- 人员分配、人员／Assignment 发生时快照和分配历史；
- 面向 Money 的应付建议及目标决定引用。

应用不拥有 Project、合同／结算、回款、人员当前档案、项目 Assignment、工资、考勤、税务、应付义务、实际付款、核销、公司收入统计或会计凭证。

## 奖金类型与来源

### 经营奖金

经营奖金基数来自 Contract Ledger 已确认的客户结算事实。每次计算冻结精确 Settlement ID／revision、Project、金额、币种、状态和稳定摘要；合同当前值变化不反向修改已发布奖池。

### 生产奖金

生产奖金基数来自 Money Flow Ledger 已确认的公司回款集合。每次计算冻结组成回款的 Transaction／Allocation ID、revision、计入金额、币种、Project 和稳定摘要；未分配资金、待确认义务、票据金额和应收余额不自动成为奖金基数。

来源为空、币种不一致、revision 过期、状态不允许或来源服务不可用时，新的计算或发布失败关闭。已有发布结果继续可读，并明确显示实时来源不可用或已发生 drift。

## 计算策略

计算策略包含稳定 key、语义版本、适用奖金类型、精确比例表示、公式、舍入方式和生效条件。计算结果保存策略 key／version、全部输入快照、输出金额和计算摘要。

参考 OA 的“基数 × 用户填写比例 ÷ 1.065”是 `reference-oa-bonus-v1` Customer Overlay 策略，不进入公共金额内核。比例由有权限的用户输入；公共内核只执行版本化策略和精确十进制定点运算，不使用二进制浮点数。金额使用整数最小货币单位，舍入必须由策略显式声明。

同一 Project 与奖金类型可以有多个历史计算，但同一来源摘要和策略输入的同一 Command 重放只能得到同一个结果。来源变化只产生 drift，不自动重算或覆盖已发布结果。

## 奖池生命周期

第一版奖池状态为：

- `draft`：计算结果已建立，允许在 expected revision 下重新计算或调整分配草稿；
- `published`：输入、策略、奖池金额和分配集合冻结；
- `superseded`：新发布 revision 明确取代旧发布结果，旧结果继续可追溯；
- `withdrawn`：错录或取消被明确撤回，历史和下游引用保留。

发布是显式命令，不由填写发布日期或页面保存隐式触发。已发布奖池不原地编辑或物理删除；纠正先建立新 revision，再由有权限的 Actor 显式发布并取代旧 revision。撤回不级联删除 Money Flow 的义务、付款或核销。

经营或生产奖金可以被显式标记为 `not_applicable`，并允许恢复。留空、金额为零或依赖不可用不等于不适用。

## 人员分配

经营奖金候选来自 Project 当时有效的经营负责人和经营参与 Assignment；生产奖金候选来自当时有效的生产负责人和适用生产角色 Assignment。People & Groups 提供 Person 当前引用，Project 提供 Assignment 事实，Bonus Distribution 在发布时冻结：

- Person ID／revision 与最小显示快照；
- Assignment ID／revision、角色 key 和有效时点；
- 分配比例、精确分配金额、币种和分配理由；
- 所属奖池 ID／revision。

人员后续改名、离组、停用或 Assignment 结束不改写历史分配。新的草稿只能选择计算时仍满足策略资格的人员；来源服务不可用时不得按缓存名称创建新分配。

第一版正式分配只接受比例分配，比例必须大于零且合计不超过 100%。未分完部分保持未分配，不自动平均。固定金额、同一人员按多个角色重复分配和跨 Project 分配缺少稳定业务证据，不进入第一版。

## 应付与实际付款

发布后的每条人员分配可以提出版本化 Payable Suggestion。建议冻结分配 ID／revision／digest、Person 快照、Project、金额、币种、用途、到期日和 Command ID。

Money Flow 独立受理、接受或拒绝：

- 接受后形成 Money 自有 payable Obligation；
- 拒绝时不形成 Obligation；
- 实际付款、核销、余额和冲销只由 Money Flow 管理；
- Bonus Distribution 只保存建议同步状态、Money 决定 revision 和可选 Obligation 引用；
- “已发布”“已建议”“已形成应付”“部分付款”和“已付清”是不同状态，不得互相冒充。

撤回或取代已形成应付的分配时，Bonus Distribution 只能提出撤回／调整建议；Money Flow 根据自身状态独立决定，不能跨库删除或修改义务。

## 权限

稳定动作至少包括：

- `bonus-pool.calculate|read|publish|supersede|withdraw|history.read`；
- `bonus-allocation.manage|read|amount.read`；
- `bonus-payable-suggestion.create|read|withdraw`。

经营与生产奖金使用不同的 Project Assignment Policy。身份管理员、普通经营成员和普通生产成员不因岗位名获得奖金发布权；经营责任不能操作生产奖池，生产责任不能操作经营奖池。目标应用复验 Tenant、Project／Pool Scope、奖金类型、当前状态、expected revision 和来源摘要，并返回动态 `allowed_actions`。奖金金额和人员奖金统计属于敏感读取，必须由服务端收敛。

## 最小组合与独立性

领域核心组合为 IAM／本地模板 + Project + People & Groups + Bonus Distribution，并以契约测试提供来源端口。真实经营奖金加入 Contract；真实生产奖金加入 Money。向 Money 提出应付建议时 Money 是显式目标依赖，不是奖池历史读取的启动前提。

未选择 Bonus Distribution 时，Workbench、Gateway、IAM Catalog、进程、migration、备份和 Project Completion Profile 均不出现 C7 项；C1～C6 继续工作。选择单一奖金类型时，不要求启用另一类型的来源端口或权限策略。

## 故障与恢复

- 计算、发布和分配命令在 Project／People／Contract／Money 任一必需引用无效时零业务写入；
- 依赖暂不可用返回可恢复失败，已有 current／revision／history 仍可读；
- 成功 Command 重放先命中本地 receipt，不重新读取外部来源；
- 同一 Command ID 不同请求摘要冲突；
- current、revision、command 和 audit 在同一本地事务提交；
- 跨应用建议使用各自事务、幂等和显式同步恢复，不使用分布式事务；
- 备份恢复保持 Pool／Allocation ID、revision、输入摘要、人员快照、目标引用和不可变历史。

## C7 第一实现轮验收

- C7-A01：经营奖池冻结精确 Settlement revision，来源改变不改写已发布结果；
- C7-A02：生产奖池冻结精确 Money Transaction／Allocation 集合和摘要；
- C7-A03：`reference-oa-bonus-v1` 按精确金额、比例和显式舍入得到确定结果；
- C7-A04：同输入同 Command 幂等，不同摘要冲突；
- C7-A05：草稿发布后输入、策略和分配冻结，纠正只能追加 revision；
- C7-A06：经营与生产来源、Capability 和 Project 责任互相隔离；
- C7-A07：分配候选只来自当时有效的精确 Person／Assignment；
- C7-A08：人员或 Assignment 后续变化不改写历史分配；
- C7-A09：比例大于零、合计不超过 100%，金额守恒且未分配余额显式；
- C7-A10：无金额读取能力时 current、history 和统计均由服务端脱敏；
- C7-A11：Project／People／Contract／Money 引用无效或不可用时新命令零部分写入；
- C7-A12：依赖恢复后原业务意图只提交一次，成功重放不重新依赖上游；
- C7-A13：应付建议接受只生成一个 Money Obligation，拒绝生成零个；
- C7-A14：撤回或取代不级联删除 Money 事实，调整由 Money 独立决定；
- C7-A15：经营奖金最小组合不隐式启动生产来源，生产奖金最小组合不隐式启动 Contract；
- C7-A16：未选装时入口、进程、migration 和备份项全部缺席，重新选装后数据仍在；
- C7-A17：联合空目标恢复保持 ID、revision、摘要、分配和建议引用；
- C7-A18：客户公式、岗位名和页面术语只存在于版本化策略／Overlay，不进入公共内核。

## 当前实现状态

第一实现收缩已经建立独立 `backend/services/bonus-distribution` Go module。公共领域核心使用同一个 `BonusPool` 聚合承载 `business` 与 `production`，但分别调用 `ResolveBusinessSource` 与 `ResolveProductionSource`，没有宽财务记录或按名称关联。Project、来源、Person、Assignment 与策略全部冻结精确 revision／digest；比例使用整数基点，金额使用整数最小货币单位，未分配余额显式保存。

`reference-oa-bonus-v1` 位于 Customer Overlay evaluator，不进入公共核心条件分支。其 `base × ratio ÷ 1.065` 使用精确有理数中间值和 half-up 舍入；当前只接受比例分配，合计超过 100%、重复 Person／Assignment、跨类型来源和错误 Project 责任均在写入前拒绝。

PostgreSQL 18 已实现 Pool／Allocation current、不可变 revision、Command Receipt 与 Audit Event。草稿创建、发布、取代和撤回在单一本地事务中提交；数据库 trigger 拒绝历史更新／删除和 current 物理删除；Tenant、Project、action、expected revision、来源 drift、失败零写入、成功本地重放和并发单次提交已有真实数据库证据。因此 C7-A01～A09 已在本轮声明范围内通过。

第二实现收缩已增加版本化 HTTP、`bonus-distribution` audience 的 Ed25519 IAM、Capability Manifest、经营／生产 Project Assignment Policy，以及请求绑定的 Project／People／Contract／Money 网络适配器。适配器使用精确资源 Scope、严格 JSON、1 MiB 响应上限、显式超时和一次 401 换票；404／422 归类为无效引用，其他上游失败归类为暂不可用。目标应用复验 Tenant、Project／Pool／Allocation Scope、奖金类型和状态，当前／历史／分配历史均在服务端把无金额能力字段置为 `null`。

真实 PostgreSQL 18 加严格 HTTP 模拟来源已验证：依赖中断返回 503 且零业务写入，恢复后原业务意图只创建一次，成功 Command 在上游再次离线时本地重放；401／404／409／422／503、错 audience／tenant／Project／相邻角色／相邻资源均失败关闭。因此 C7-A10、A11、A12 在当前受保护后端范围内通过。Project／Contract 已有目标 reference-read 形状；People Person 与 Money Receipt Set 的版本化契约、消费适配器和真实来源应用端点均已建立。IAM、Project、People、Contract、Money、Bonus 已以六个独立进程／数据库完成经营和生产奖金发布，运行期只使用正式工作负载换票与 HTTP 引用，不共享业务表或静态来源 Token。

第三实现收缩已增加 C7 人类工作区：Project 范围列表、经营／生产草稿、固定策略与来源核对、按 `allowed_actions` 发布／替代／撤回、Pool／Allocation 历史、金额脱敏，以及 401／403／409／422／503 的显式恢复。浏览器只提交稳定 ID、revision、比例和原因，不接收手填来源金额，也不直接读取相邻数据库。

六进程六数据库自动化又验证：根页面与同源脚本可取、无凭证 API 返回 401；Contract 停机时替代草稿返回可恢复 503 且 Pool 数量不变，恢复后同一业务输入只建立一个草稿；旧 published 与新 draft 在单事务形成 superseded／published 双向关系，生产 published 可撤回，三条历史链均完整。因此正常、纠错和依赖失败的人类交互所依赖后端闭环已通过。

第五实现收缩已把 Bonus Distribution 作为正式 D1 可选应用装入独立镜像、数据库、migration、工作负载身份、Capability Catalog、Suite Manifest、Workbench、Gateway 路由和联合备份。隔离 `omsd1c7` 经 Nginx→Gateway→IAM 完成 16 条入口断言且偏差为 0，并留下 Project、People、Contract、Money 共 5 条精确工作负载审计；经营与生产各发布一个 Pool。

联合备份 `20260928T143704Z` 经空目标恢复保持 Pool／Allocation ID、revision、来源／策略／人员快照、四份 canonical digest 和历史；恢复后生产 Pool 从 `published revision 2` 追加为 `withdrawn revision 3`。取消选装时 Manifest、Workbench、route、runtime、migration、工作负载密钥和 dump 均缺席，重新选装后原数据仍在。因此 C7-A16 已通过，C7-A17 在建议引用之外的 Pool／Allocation 范围通过。

C7→Money Payable Suggestion 已完成直接跨进程收缩。Bonus 以 `bonus-distribution/bonus_allocation` 冻结已发布 Allocation revision／digest、Project、Person 快照、金额和用途；Money 只接受经人工确认、不可变的 People Person → Money individual Party 精确映射，并独立接受或拒绝。Money 初始离线时 Bonus 先追加 `target_unavailable`，恢复后以同一命令重试；接受恰好形成一个 Money 自有 payable／open Obligation，拒绝形成零个，重放不重复。源 Pool／Allocation 后续撤回不级联修改已形成义务，映射篡改和未映射 Person revision 均失败关闭。因此 C7-A13、C7-A14 在该直接 HTTP 组合范围内通过。

本轮使用一个临时 PostgreSQL、Bonus 与 Money 两个进程和独立数据库完成 11 条 HTTP 断言，结束后运行容器为 0。C7-A17 的建议引用联合备份／空目标恢复、正式 D1 connector、经营／生产单类型最小组合、完整双 Tenant 与真实 Ubuntu 仍待验证；结果不得外推为 C8 或 OA 完整体通过。证据见 [C7→Money 应付建议实验](../../../docs/reports/oa-c7-money-payable-suggestion-experiment-20260929.md)与 [C7 正式 D1 装配、未选缺席与联合恢复实验](../../../docs/reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)。

## 不进入第一版

- 工资、薪酬核算、考勤、个税、社保、银行代发和会计凭证；
- 公司收入、管理费、跨项目人员奖金报表与通用统计平台；
- 用户可编辑的任意公式、任意审批流或低代码规则引擎；
- 固定金额分配、跨 Project 奖池、自动审批和自动付款；
- 通过共享表、跨库 ORM、人员姓名或客户岗位关联事实；
- 用“奖金已发布”推断 Project、生产或财务已经完成。

## 复审条件

真实客户证明固定金额或多人多角色分配是稳定规则；经营与生产奖金出现独立交付／部署要求；奖金形成需要强制审批、税务或薪酬生命周期；People／Project 快照不能表达实际资格；Money Payable Suggestion 不能满足真实付款纠错；或第二参考产品出现不同稳定来源语义时，重新审查应用边界和策略契约。
