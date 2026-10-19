# OA C7 奖金形成与分配责任边界审计

日期：2026-09-28  
结论：`boundary-supported / implementation-not-started`

## 1. 本轮要回答的问题

**问题 A**：参考 OA 已经可以设置经营／生产奖金池并把奖金分配给项目人员，但实现把奖池、独立 Bonus、宽财务记录和当前人员关系混在一起。若直接复制，无法证明奖金基数、公式、人员资格、历史纠错和最终付款分别由谁拥有。

**可验证结果 B**：确认一个可进入下一实现轮的最小 C7 边界，明确应用所有权、来源端口、Customer Overlay 公式、人员快照、应付协作、权限与 C7-A01～A18；不把边界文档冒充运行通过。

## 2. 直接证据

| 来源 | 观察 | 对目标边界的含义 |
| --- | --- | --- |
| `docs/OA_2026_REQUIREMENTS_CONFIRMED.md` BONUS-001～003 | 经营基数来自甲方结算，生产基数来自公司回款；用户填写比例，客户公式除以 1.065 | 两种来源稳定存在；公式是客户策略，不是 Money 或 Project 通用规则 |
| 同文档 STAT-003／004 | 需要按人员查看跨项目奖金，且属于敏感信息 | C7 必须发布只读投影并服务端授权；跨项目统计属于 C8，不进入可写核心 |
| `frontend/src/pages/UsageGuide.tsx` | 经营候选来自经营负责人／参与人，生产候选来自生产负责人／项目人员；比例合计不超过 100% | 人员资格来自 Project Assignment 与 People，而不是自由选择 User 表 |
| `backend/internal/oa2026/service.go` | 运行时重读 Contract／FinancialRecord 当前值，以 `float64` 计算并 upsert 单条 Pool | 证明当前用户结果，但不保留来源 revision、策略版本或历史，且不满足精确金额约束 |
| `backend/internal/services/financial_service.go` | 批量分配直接创建多条 `FinancialRecord`，支持比例／固定金额 | 证明批量旅程存在；固定金额缺少确认需求，宽表也不能成为目标模型 |
| `backend/internal/models/bonus.go` | 独立 Bonus 表与宽财务记录并存，只有 Project、User、类型和浮点金额 | 当前处于双真相过渡态，不能从表存在推断稳定所有权 |
| `DeleteBonus`／`DeleteFinancialRecord` | 奖金可以更新和删除 | 目标必须使用 revision、取代与撤回，不能静默改写或删除历史 |
| `docs/界面改版阶段需求整理.md` | 奖金是结果分配，不建议与成本合并；经营与生产分配对象不同 | C7 不进入 Cost & Outsourcing；两类可共享应用边界但保持类型化权限／来源 |

## 3. 已比较的责任方案

| 方案 | 短期便利 | 长期问题 | 判定 |
| --- | --- | --- | --- |
| 继续使用 Money／FinancialRecord | 页面和统计复用快 | Money 被迫拥有公式、人员资格和来源原因；宽表继续扩张 | 拒绝 |
| 放入 Project Workspace | Project 页面最集中 | Project 会拥有金额、策略和分配历史，破坏可选聚合边界 | 拒绝 |
| 放入 Production Operations | 生产奖金就近 | 无法承载经营奖金，且污染 Stage／Deliverable 内核 | 拒绝 |
| 放入 People & Groups | 人员选择方便 | 名册会拥有奖金、工资和金额，人员变更会污染历史 | 拒绝 |
| 经营／生产各建一个应用 | 权限表面清晰 | 当前生命周期、模型和纠错相同，形成无证据复制 | 暂不采用 |
| 独立 Bonus Distribution，一个应用两种类型 | 保持一个稳定生命周期，来源和权限仍类型化 | 需要新增应用、端口和装配证据 | 接受 |

## 4. 接受的最小边界

Bonus Distribution 拥有：

- 奖池类型、适用性、计算策略版本、输入快照和精确输出；
- Pool／Allocation current 与不可变 revision；
- 发布、取代、撤回和分配纠错；
- Person／Assignment 发生时快照；
- 面向 Money 的人员应付建议和目标决定引用。

相邻应用继续拥有：

- Contract：经营奖金使用的客户结算；
- Money：生产奖金使用的已确认公司回款，以及所有应付、付款、核销和余额；
- People & Groups：Person 当前档案；
- Project：项目身份和 Assignment；
- IAM：Subject、Tenant、Grant 与短期能力凭证；
- C8 投影：跨项目人员奖金统计、公司统计和工作台摘要。

工资、薪酬核算、考勤、税务、社保、银行代发、管理费和会计凭证不是 C7。

## 5. 计算与 Customer Overlay

公共内核执行版本化策略，不内置某客户公式。第一条策略 `reference-oa-bonus-v1` 固定：

- business 输入：已确认 Contract Settlement；
- production 输入：Money 确认的公司回款 Transaction／Allocation 集合；
- 参数：用户输入比例；
- 公式：`base × ratio ÷ 1.065`；
- 输出：整数最小货币单位；
- 舍入：必须在策略契约中显式定义，不能依赖语言默认浮点舍入。

来源、策略或比例变化产生新的计算和 revision。读取页面不会重新计算并覆盖已发布结果。

## 6. 人员与分配边界

经营候选读取当时有效的经营负责人／经营参与 Assignment；生产候选读取生产负责人和适用生产角色 Assignment。发布冻结 Person 和 Assignment 精确 revision，后续改名、离组或停用不改写历史。

第一版只接受比例分配：每行大于零、合计不超过 100%，未分配余额显式保留。当前代码里的固定金额、同人多角色重复分配和任意 User 选择没有足够确认需求，保持 `needs_business_evidence`。

## 7. 与 Money 的边界

“已发布奖金”不是“已经付款”。C7 可以逐分配提出 Payable Suggestion；Money 独立接受或拒绝。接受生成且仅生成一个 Money 自有 Obligation，拒绝生成零个。实际付款、核销和冲销继续只在 Money 内发生。

若已形成应付的奖金分配被撤回或取代，C7 只能提出撤回／调整建议，不能跨库删除义务。C6 已验证的建议协作形状可复用技术机制，但 C7 使用自己的来源类型、模型和 Capability，不复用 C6 领域对象。

## 8. 最小组合

- 领域核心：IAM／模板 + Project + People + Bonus Distribution + 类型化来源契约夹具；
- 经营奖金真实组合：上述组合 + Contract；
- 生产奖金真实组合：上述组合 + Money；
- 人员应付组合：上述组合 + Money 建议受理；
- 未选择 C7：入口、进程、Capability、migration、备份和完成规则全部缺席，C1～C6 不受影响。

这说明“独立应用”仍允许显式直接依赖；它不等于复制来源事实或把所有系统强制打包。

## 9. C7 第一实现轮验收矩阵

| 编号 | 必须证明的结果 |
| --- | --- |
| C7-A01 | 经营奖池冻结精确 Settlement revision，来源变化不改写发布历史 |
| C7-A02 | 生产奖池冻结精确 Money 收款集合和 digest |
| C7-A03 | 客户策略以精确金额和显式舍入产生确定结果 |
| C7-A04 | 同 Command／同摘要幂等，不同摘要 409 |
| C7-A05 | 发布后只能追加 revision、取代或撤回，不能更新／删除历史 |
| C7-A06 | 经营与生产来源、Capability 和 Assignment Policy 互相隔离 |
| C7-A07 | 分配候选只来自有效 Person／Assignment 精确引用 |
| C7-A08 | 人员和 Assignment 后续变化不改写已发布分配 |
| C7-A09 | 比例与金额守恒，未分配余额显式 |
| C7-A10 | 金额／人员统计由服务端按能力脱敏 |
| C7-A11 | 任一必需依赖无效或不可用时零部分写入 |
| C7-A12 | 恢复后原意图单次提交，成功重放不重新依赖来源 |
| C7-A13 | Money 接受只创建一个 Obligation，拒绝创建零个 |
| C7-A14 | C7 撤回／取代不级联删除 Money 事实 |
| C7-A15 | 单类型最小组合不隐式启动另一来源应用 |
| C7-A16 | 未选装真实缺席，重新选装不丢数据 |
| C7-A17 | 空目标恢复保持 ID、revision、摘要、分配和目标引用 |
| C7-A18 | 客户公式和岗位只在策略／Overlay，不进入公共内核 |

## 10. Base／Overlay 变化账目

| 层 | 本轮结论 |
| --- | --- |
| 稳定 Base | Pool／Allocation 生命周期、精确金额、revision、快照、权限、幂等和建议边界 |
| 配置 | 来源端点、超时、页面显示名称 |
| 策略 | `reference-oa-bonus-v1` 公式、比例参数、舍入与经营／生产 Assignment Policy |
| 类型化扩展 | `business`／`production` 来源 adapter 与奖金类型 |
| Suite Manifest | 是否启用 C7、启用哪类来源和是否启用 Money 建议 connector |
| 客户专项 | 当前无；不得建立客户核心分支或客户专用表 |

## 11. 未解决事项

- 公式最终舍入方式尚未从业务材料确认，第一实现前必须在策略契约中明确；
- 固定金额分配、同人多角色、奖金审批和应付调整仍缺稳定业务证据；
- C7 是否参与 Project Completion 及 `not_applicable` 的完成含义尚未确认；
- 真实税务／薪酬／银行代发不在本轮，不能用“Compensation”名称暗示已覆盖；
- C8 人员奖金统计只确认读取结果，投影刷新、时效和性能仍待下一边界审计。

## 12. 结论

C7 边界成立：采用独立 Bonus Distribution 应用，并将经营／生产作为一个生命周期下的类型化来源与权限差异。该结论减少了把 Project、People 或 Money 扩成万能平台的风险，也保留了客户公式的 Overlay 空间。

本轮没有代码、数据库、API、页面、D1 或恢复证据，因此只能声明 `boundary-supported / implementation-not-started`。下一轮必须从 fresh state 选择领域模型／契约与核心闭环，不能把本文档视为 C7 已实现。
