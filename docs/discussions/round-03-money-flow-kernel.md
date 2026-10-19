# 通用商务业务与模块基建讨论：第 3 轮——经营收支内核

日期：2026-09-19  
状态：候选设计，不是会计系统规格，不包含代码实现

关联文档：

- [第 1 轮：总体业务与模块拆分](./round-01-contract-income-expense-foundation.md)
- [第 2 轮：合同内核](./round-02-contract-kernel.md)

## 本轮主要结论

1. 收支系统的最小稳定主线是：**义务 → 票据 → 实际资金 → 核销 → 余额与统计**。这五类事实不能合成一个 `FinancialRecord`。
2. 合同金额不是应收，开票不是应收，审批不是到账，付款授权不是付款。每个词都必须对应不同业务对象。
3. 余额必须由“已确认义务及其调整 − 有效核销 − 核销型减免/坏账”计算，不能简单使用“合同额 − 回款”。
4. 一笔资金可核销多个义务，一个义务也可由多笔资金核销；预收、超付和暂时无法匹配的资金保留为未分配余额。
5. 统计只是带 `as_of` 时点的读模型。任何总额必须能下钻到义务、票据、资金和核销明细。
6. 付款申请与审批属于可选 `payment-workflow`，实际收付款属于 money-flow。客户没有审批时，核心收支能力仍能独立运行。

## 一、先统一六个容易混淆的概念

| 概念 | 回答的问题 | 是否直接改变现金 |
| --- | --- | --- |
| 合同金额 | 合同条款约定了多少 | 否 |
| 应收/应付义务 | 某一方现在应该向另一方支付多少 | 否 |
| 发票/票据 | 已开具或收到什么票据 | 否 |
| 付款授权 | 某笔付款是否获准执行 | 否 |
| 实际收款/付款 | 资金事实上是否发生 | 是 |
| 核销 | 这笔资金结清了哪些义务 | 不新增现金，只解释归属 |

“收入、成本、利润”在不同行业可能采用不同确认规则。第一版通用系统只稳定承诺经营义务、票据、现金和核销，不冒充会计收入确认或法定财务报表。

## 二、现实中的候选通用收支流程

### 1. 取得业务依据

义务可以来自：

- 某个合同 revision 或结算事实；
- 订单、验收单、计量单、人员款项清单；
- 奖金、费用、工资等独立业务模块；
- 历史导入；
- 无上游系统时的人工登记。

收支内核只保存稳定的来源快照引用，不负责重新解释来源模块的全部业务。

### 2. 建立义务

每一笔应收或应付至少明确：

- 债权方和债务方；
- 相对于账簿所有者的方向：应收或应付；
- 金额与币种；
- 到期日；
- 来源快照；
- 结算对象或约定收款对象；
- 可选业务分类与上下文；
- 创建、确认和调整审计信息。

分期款优先拆成多个到期义务，而不是在一个对象内隐藏复杂分期数组。这样账龄、逾期和核销都更稳定。

### 3. 可选确认审批

- 有审批的客户：先建立草稿义务，审批通过后确认；
- 无审批的客户：有权用户可直接确认；
- 审批绑定义务版本和负载哈希，金额、对象、到期日或来源变化后旧审批失效；
- 审批拒绝不产生 open 义务；审批通过也不产生实际现金。

### 4. 登记票据

- 我方开票、收到对方发票或其他结算票据分别登记；
- 一张票据可以关联多个义务，一个义务也可以分多张票据；
- 票据金额与义务金额不要求始终一一相等；
- 作废、红冲和更正使用追加事实，不覆盖原票据；
- 税率、抵扣、申报和税务会计仍属于外部财税边界。

### 5. 形成付款意图（可选）

需要付款审批、资金计划或银行指令的客户，可以建立 Payment Intent：

- 计划支付对象、金额和日期；
- 计划核销的应付义务；
- 资金账户或资金来源；
- 审批与执行状态。

Payment Intent 是可选工作流对象，不是实际付款，也不能冲减应付余额。

### 6. 记录实际资金

实际资金可以来自银行接口、流水导入或人工录入。至少记录：

- 收款、付款或内部转账类型；
- 金额、币种和发生时间；
- 付款方、收款方；
- 外部流水号和数据来源；
- 证据引用；
- 导入/人工确认状态。

银行流水是更强证据，但第一版不强制接银行。人工记录必须显式标记来源，不能伪装成银行已核验事实。

### 7. 分配与核销

- 一笔资金可以拆分核销多个义务；
- 一个义务可以被多笔资金部分核销；
- 核销不能超过该笔资金可分配余额；
- 对义务超额分配需形成预收/预付或独立调整，不允许静默丢弃；
- 无法匹配的实际资金保留为未分配现金，后续再核销；
- 票据匹配和资金核销是两类关系，不应直接用“发票关联付款”替代义务账。

### 8. 调整、冲销和异常处理

- 已确认义务金额变化：追加 Obligation Adjustment；
- 错误现金记录：追加反向 Cash Reversal，再记录正确事实；
- 错误核销：反向 Allocation，不删除原关系；
- 退款：记录反向资金并关联原资金或义务；
- 减免、坏账、争议或豁免：记录有原因、有权限的义务调整；
- 重复导入：通过外部流水号、来源和幂等键拦截。

### 9. 关闭义务

义务在余额归零或通过受控减免/坏账处理后关闭。部分已付、逾期、超期和待开票是投影状态，不需要不断修改主状态字段。

### 10. 生成统计和预测

系统按指定时点，从义务、票据、资金、核销和调整事实生成：

- 已确认应收/应付；
- 已收/已付；
- 未收/未付；
- 已开票/已收票；
- 未分配收款/付款；
- 逾期金额与账龄；
- 预计流入/流出；
- 实际流入/流出。

## 三、最小产品对象

### 1. Source Snapshot Reference：来源快照引用

```text
source_system
source_type
source_id
source_revision
source_event_id?
amount_basis?
```

它可以引用合同，也可以引用订单、结算单、人员清单或外部系统。收支内核不依赖合同模块才能工作。

### 2. Obligation：应收/应付义务

核心字段：

- obligation ID；
- ledger owner；
- direction：receivable/payable；
- creditor、debtor 和可选 settlement party；
- 精确金额与币种；
- due date；
- source snapshot ref；
- 分类和业务上下文引用；
- 当前 revision；
- 最小生命周期状态。

收款对象与合同相对方不一致时，必须记录第三方收款依据，不能只改一个名称字段。

### 3. Obligation Adjustment：义务调整

义务确认后不直接改原金额。增减、减免、坏账、更正和转移都形成追加调整，保留：

- 调整金额与方向；
- 调整原因；
- 来源和证据；
- 生效时间；
- 授权或审批证明；
- 调整后的 obligation revision。

### 4. Billing Document：票据记录

只保存通用票据事实：

- issued/received 方向；
- 票据号码、日期、金额、币种；
- 开具方和接收方；
- 状态与更正关系；
- 文件证据；
- 与义务的多对多 Billing Link。

税务平台状态和行业票种由适配器或扩展模块提供。

### 5. Cash Transaction：实际资金记录

- receipt/payment/internal transfer；
- 金额与币种；
- 发生时间；
- 付款方、收款方；
- 账户引用；
- 外部流水号与来源；
- 证据引用；
- 原资金记录引用（退款或冲销）。

`internal transfer` 不计经营收入或支出，避免公司内部账户互转把统计翻倍。

### 6. Allocation：资金核销关系

Allocation 是独立对象，而不是 Cash Transaction 上的单个 `obligation_id`：

- cash transaction ID；
- obligation ID；
- 分配金额；
- 币种；
- 生效时间；
- 操作人和证据；
- 反向核销引用。

第一版可以要求资金与义务同币种。跨币种核销、汇率和汇兑损益留给扩展模块。

### 7. Payment Intent：可选付款工作流对象

Payment Intent 负责计划、审批和执行协调，可独立启用。它可以引用多个应付义务，并在银行返回结果后关联实际 Cash Transaction。

不需要付款审批的客户可以完全不部署此对象。

## 四、最小状态与派生状态

### 义务主状态

| 状态 | 含义 |
| --- | --- |
| `draft` | 尚未确认，可编辑或取消 |
| `open` | 已确认，存在待处理余额 |
| `closed` | 余额已经结清或经受控处理关闭 |
| `cancelled` | 未形成有效义务或以正式撤销事实取消 |

以下状态应计算得出，不作为第二真相源：

- partially settled；
- settled；
- overdue；
- unbilled/partially billed/fully billed；
- written off；
- overpaid/prepaid。

### 其他对象状态

- Billing Document：recorded / voided；
- Cash Transaction：recorded / reversed；
- Allocation：active / reversed；
- Payment Intent：由可选工作流模块独立定义。

## 五、核心计算口径

在指定 `as_of` 时点和单一币种下：

```text
confirmed obligation amount
  = original confirmed amount
  + effective adjustments

allocated amount
  = active allocations

outstanding amount
  = confirmed obligation amount
  - allocated amount
  - effective write-off/reduction adjustments

unallocated cash
  = recorded cash amount
  - active allocations from that cash transaction

overdue amount
  = outstanding amount of obligations whose due date < as_of
```

注意：减免/坏账可以实现为特定 Obligation Adjustment，因此实际公式中不应重复扣减。上述拆写只用于说明口径，最终代码必须选定一个唯一演算模型。

统计必须同时保留：

- 原始总额；
- 调整额；
- 核销额；
- 余额；
- 未分配现金。

不能把负余额简单截为 0。负值通常意味着预收、超付、退款未处理或错误核销，需要单独展示和处理。

## 六、统计模块的最小边界

### 通用维度

- 组织/账簿所有者；
- 应收或应付；
- 往来方；
- 来源类型与来源引用；
- 可选合同、项目或其他上下文；
- 分类 code；
- 币种；
- 到期期间和发生期间；
- `as_of` 时点。

### 通用指标

- 义务原始额、调整额和当前确认额；
- 已核销、未核销；
- 已开票/已收票及未匹配票据；
- 实际收款、付款和未分配资金；
- 逾期金额、账龄分段；
- 未来到期金额和预测现金流。

### 非通用指标

利润、收入确认、完工百分比、管理费、奖金、税负和行业专项成本需要明确计算政策，只能作为扩展投影，不能放进第一版通用统计。

### 投影原则

- 投影可以缓存或异步更新，但能够从原始事实重建；
- 每个统计值都能下钻到明细；
- 修复投影不修改业务事实；
- 多币种默认分币种展示，不用隐式汇率相加；
- 历史报表必须带 `as_of`，不能用今天的合同和义务状态重算过去。

## 七、代码层最小结构

### 1. 内部组件

```text
money-obligation-domain
  义务、调整、余额不变量

money-billing-domain
  票据、作废/更正、义务匹配

money-cash-domain
  实际资金、退款、冲销、导入来源

money-allocation-domain
  多对多核销与可分配余额

money-application
  命令编排、幂等、并发版本和可选门禁

money-projection
  余额、账龄、票据、现金流和统计读模型
```

它们可以先作为同一 money-flow 部署单元内的严格包边界。不要一开始拆成多个网络服务，也不要合并为一个万能实体。

### 2. 最小命令契约

| 命令 | 主要结果 |
| --- | --- |
| `CreateObligationDraft` | 建立待确认义务 |
| `ConfirmObligation` | 形成 open 应收或应付 |
| `AdjustObligation` | 追加金额或归属调整 |
| `CancelOrCloseObligation` | 受控取消或关闭 |
| `RecordBillingDocument` | 记录开出/收到票据 |
| `VoidOrCorrectBillingDocument` | 追加作废或更正事实 |
| `RecordCashTransaction` | 记录实际收付款 |
| `ReverseCashTransaction` | 追加反向资金事实 |
| `ApplyAllocation` | 将资金分配到义务 |
| `ReverseAllocation` | 撤销错误核销但保留历史 |

写命令统一携带幂等键、actor、expected revision、业务时点、来源和可选门禁证明。

### 3. 最小查询契约

- `GetObligationSnapshot(id, as_of?)`
- `ListOutstandingObligations(filters, as_of)`
- `GetCashTransaction(id)`
- `ListUnallocatedCash(filters)`
- `GetCounterpartyStatement(filters, as_of)`
- `GetAgingReport(filters, as_of)`
- `GetCashForecast(filters, period)`
- `TraceMoneyFlow(source_ref|obligation_id|cash_id)`

### 4. 最小事件契约

- `ObligationConfirmed`
- `ObligationAdjusted`
- `ObligationClosed`
- `BillingDocumentRecorded`
- `BillingDocumentVoided`
- `CashTransactionRecorded`
- `CashTransactionReversed`
- `AllocationApplied`
- `AllocationReversed`

每个事件含事件 ID、聚合 ID、revision、发生时间和必要来源引用，使用 outbox 保证与业务写入一致提交。

## 八、可选审批与付款执行

继续复用第 2 轮的 `DecisionGate`：

- `ConfirmObligation` 可配置是否审批；
- `AdjustObligation` 在超过阈值时可审批；
- Payment Intent 可单独审批付款；
- 坏账、减免和手工核销可以要求更高等级审批；
- 无审批策略时，权限校验后直接执行同一命令。

付款审批通过后只产生授权证明。真正减少应付余额的仍是实际付款和 Allocation。

## 九、管理费、奖金、工资如何接入

这些都不应成为 money-flow 内核固定字段：

- 管理费规则模块计算结果后，生成独立应收/应付义务或显式抵扣事实；
- 奖金模块负责人员、比例和分配逻辑，确认结果后生成应付义务；
- 工资模块负责人事、税社保和工资计算，付款时把应付义务交给 money-flow；
- 报销模块负责申请和审批，批准后生成应付义务；
- 不允许仅在付款金额中暗扣管理费却不留下独立业务事实。

## 十、与合同系统的组合方式

合同系统发布 `ContractActivated`、`ContractChangeActivated` 或 `ContractSettlementRecorded`。客户装配层决定是否：

- 不自动处理，只供人工建义务时选择；
- 按里程碑自动创建义务草稿；
- 在结算后创建最终义务；
- 合同变更时创建义务调整建议。

money-flow 只接受标准 `SourceSnapshotRef`，因此没有合同系统时也能用订单、外部 ID 或人工依据运行。

## 十一、从两个样本提取的经验与反例

### OMS 雏形中的有用候选信息

- 应收、应付与实际资金分开；
- 付款授权与付款结果分开；
- 到期日、逾期和现金预测有现实价值；
- 管理费和人员款项需要完整追溯。

但“所有义务必须审批”“固定甲方—我方—施工队—人员链”和具体工资责任都不是通用事实。

### OA 已实现经验

值得保留：

- 收款与开票独立记录；
- 公司级统计是聚合视图；
- 项目与全局权限分层；
- 合同有效金额可由变更重放。

需要重构：

- 一个 `FinancialRecord` 同时承载奖金、成本、付款、发票、专家费和委托业务；
- 使用 `type` 加大量可空字段表达完全不同的业务；
- 合同额直接等同应收；
- 未收等于合同额减全部回款，负数直接截 0；
- 发票直接一对一关联付款，没有独立义务和多对多匹配；
- 所有记录强制依赖 Project；
- 已发生资金允许直接编辑；
- 金额使用 `float64`；
- 设计费、勘察费、咨询费写死在统计结构中。

## 十二、本轮暂定的“最小可复用经营收支小系统”

第一版只做：

1. 应收/应付义务及追加调整；
2. 票据登记与义务匹配；
3. 实际收付款和冲销；
4. 多对多资金核销；
5. 未分配资金；
6. 余额、逾期、账龄和现金预测投影；
7. 来源、业务上下文和证据引用；
8. 权限、审批、文件、导入和事件端口；
9. 历史时点查询和完整追溯。

先不做复式记账、科目余额表、财务报表、税务申报、工资计算、报销流程、奖金计算、银行付款执行和复杂跨币种核销。

## 下一轮建议

第 4 轮将合同与 money-flow 放在一起，形成最终的“可组合小系统基建蓝图”：

- 共享哪些极小值对象；
- 哪些端口可以复用，哪些必须各域独立；
- 数据库、事件、权限、文件和审批如何组合；
- 客户定制放在配置、策略、扩展模块还是装配层；
- 用三个典型客户组合验证是否真的解耦。

## 本轮仍然开放的问题

- 第一版票据是否作为内置子域，还是可选插件；
- 义务是否允许没有到期日；
- 是否需要第一版就支持第三方代收代付；
- Payment Intent 是否作为 money-flow 标准插件交付；
- 退款、对冲和坏账的第一版最小规则；
- 经营统计与专业财务系统之间的对账责任边界。
