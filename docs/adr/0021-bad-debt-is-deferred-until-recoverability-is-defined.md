# ADR-0021：坏账在可收回性模型明确前延期

日期：2026-09-19  
状态：接受

## 背景

Money Flow 已分别用 Obligation revision 表达依据金额、Allocation／Reversal 表达实际资金核销、Remission 表达已经生效的义务免除。当前 outstanding 的唯一公式是：

```text
current effective amount - net allocated amount - remitted amount
```

讨论材料曾把坏账列为候选异常处置，但没有说明它是预计损失、会计减值、最终核销，还是法律债权终止。参考 OMS 雏形也没有可验证的坏账流程、余额口径、转回或追回款规则。

坏账与 Remission 的关键差异是：坏账通常描述应收的可收回性，未必消灭原权利。若直接从现有 outstanding 扣减，会阻止后续资金继续核销原义务；若不扣减，则无法表达任何余额效果。

## 决策

- 不在当前信息下实现 Bad Debt 写入功能；`MF-M4-005` 结束于边界审计并延期；
- 当前代码和数据库不增加 bad-debt 字段、事实表、状态、命令或审计动作；
- Amount Adjustment、Remission、Cash Reversal、Allocation 和未来 Closure 均不得被复用或重新命名为坏账；
- outstanding 保持现有单一语义，不混入“预计可收回”或“账面净额”；
- 将来只有在真实场景同时定义 settlement outstanding、expected collectible、转回、追回款、调额和关闭关系后，才重新评估独立 Bad Debt 事实；
- 会计减值、备抵、凭证与税务处理不进入 Money Flow Ledger 内核。

详细审计见 [Obligation 坏账处置边界审计](../specs/money-flow-ledger/obligation-bad-debt-audit.md)。

## 结果

优点：

- 不会把坏账伪装成减免并错误阻断未来回款；
- 不会在单一 outstanding 字段中混合应履行余额、预计可收回额和账面净额；
- 保持当前 Allocation、Adjustment 与 Remission 不变量稳定；
- 为真实需求出现后建立完整的处置、转回和追回款闭环保留空间。

代价：

- 当前系统不能登记或统计坏账；
- 受控关闭第一版不能引用尚不存在的坏账事实；
- 真实客户若立即需要坏账，必须先补齐业务证据和模型决策，不能通过自由文本变通。

## 否决方案

### 直接复制 Remission，只把类型改成 bad debt

否决。它会让“预计不能收回”在运行时等同“债权已经免除”，并使后续 Allocation 失去容量。

### 把坏账做成 decrease Amount Adjustment

否决。依据金额没有因此改变，旧 revision 也不应被伪造为金额依据变化。

### 先建表，余额和追回款以后再补

否决。没有投影与后续动作的孤立事实不是可用纵向功能，也会提前固化字段和不可变约束。

### 在 Money Flow 内实现会计减值与凭证

否决。当前系统没有支撑会计计量的事实、期间和科目边界，这会把窄域产品扩成不完整总账。
