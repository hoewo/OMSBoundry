# ADR-0023：票据在生命周期与义务匹配语义明确前延期

日期：2026-09-20  
状态：接受

## 背景

Money Flow 已用 Obligation 表达应收应付、Cash Transaction 表达实际资金、Allocation 表达资金核销。讨论材料把 Billing Document 与 Billing Link 列为独立子域，但仍未定义票号唯一范围、状态、更正／作废／红冲、匹配金额和错误 Link 撤销。

OMS 雏形只提供我方向甲方开票的展示信号，且税务口径明确待确认。OA 虽然实现我方和对方开票，却把发票与支付、奖金、成本放在同一 `FinancialRecord` 中，并直接一对一关联付款；该结构不能证明 BizFoundry 所需的票据—义务多对多语义。

## 决策

- 不在当前信息下实现 Billing Document 或 Billing Link；`MF-M4-007` 结束于边界审计并延期；
- 当前代码和数据库不增加通用发票表、票据状态、票据汇总或“发票关联付款”字段；
- 票据不得被合并进 Obligation、Cash Transaction、Allocation 或 Evidence；文件只能证明附件存在，不能替代票据业务事实；
- `recorded / voided` 只保留为讨论候选，不在缺少红冲与更正规则时固化为状态机；
- 多对多 Billing Link 必须在真实案例中明确匹配金额、币种、主体兼容、上限、撤销和票据失效后的历史行为后，才可进入实现；
- 税率、税点、抵扣、申报、会计凭证和外部开票平台继续留在 Money Flow 内核之外。

详细审计见 [Billing Document 与 Billing Link 边界审计](../specs/money-flow-ledger/billing-document-link-audit.md)。

## 结果

优点：

- 保持票据、义务、现金和会计事实边界清晰；
- 不把 OA 的客户定制一对一关系误当成通用多对多规则；
- 避免在票号、红冲和更正语义未知时固化不可变 schema；
- 为以后从真实案例交付完整登记、匹配、纠错闭环保留空间。

代价：

- 当前 Money Flow 不能登记或统计已开票／已收票金额；
- 独立闭环暂时停留在义务、现金、核销与关闭，不包含票据；
- 若首个真实客户立即需要票据，必须先补齐案例和验收口径，不能用自由文本或附件绕过。

## 否决方案

### 复制 OA FinancialRecord

否决。它混合不同业务事实、金额使用浮点数、历史可直接编辑，并把发票一对一挂到付款，不满足当前不变量。

### 先做一张只有号码、日期和金额的发票表

否决。号码范围和纠错链会直接改变唯一约束与不可变结构；孤立登记也没有完成 Billing Link 闭环。

### 把 Billing Link 做成 Cash Transaction 上的 invoice_id

否决。票据匹配与资金核销是两类不同关系，合并后无法表达未付款票据、预收款和多对多场景。
