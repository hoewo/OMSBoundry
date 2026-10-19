# ADR-0018：Cash Transaction Reversal 只使净分配为零的原资金事实失效

日期：2026-09-19  
状态：accepted

## 背景

MF-M3 已把人工实际收付款建模为独立、不可变 Cash Transaction，MF-M4-001 又允许用独立 Allocation Reversal 纠正错误核销。下一步需要纠正“这条资金事实本身不应继续有效”的情况。

如果直接更新或删除 Cash Transaction，会破坏外部流水去重、Allocation 引用、主体快照和审计链。如果把所有后续退款、拒付和反向付款都叫作 Reversal，又会抹去真实发生过的第二次资金移动。若允许在仍有 active Allocation 时冲销，还会出现有效资金为 0、义务却仍被核销的矛盾状态。

## 决策

1. 使用独立、不可变 `cash_transaction_reversal` 引用原 Cash Transaction，一条原事实最多一条完整 Reversal；
2. Reversal 完整复制原事实的方向、金额和币种，用户只提供冲销原因；
3. 原 Cash Transaction 不更新、不删除；`recorded`／`reversed` 为派生状态；
4. Reversal 只允许在该资金的 Allocation 净额为 0 时形成。若存在 active Allocation，必须先逐条追加 Allocation Reversal；
5. recorded 资金的 current effective amount 为原金额，reversed 资金为 0；unallocated amount 使用 current effective amount 减 Allocation 净额；
6. Cash Reversal 生效后，应用层和 PostgreSQL Allocation 插入触发器共同禁止任何新 Allocation；
7. Cash Reverse、Allocation Apply 和 Allocation Reverse 都以 Cash Transaction 行锁作为第一把业务锁，使并发结果可串行解释；
8. 相同 Command ID 通过标准化负载哈希幂等；Reversal、Command 与审计在同一事务提交并全部不可变；
9. 真实发生的退款、拒付扣回或反向付款必须另记新的 Cash Transaction，并保留两次资金移动，不能用本 Reversal 替代。

## 结果

- 错误资金登记可以在不破坏历史引用的前提下完整失效；
- Allocation Reversal 成为 Cash Reversal 的显式前置纠错步骤，不发生隐式级联；
- 冲销后的资金不会重新出现在未分配候选中，也不能被构造请求或直接 SQL 再次分配；
- 真实退款等场景仍保留独立事实空间，后续可以单独建模，不被本功能提前决定；
- 页面和未来汇总必须区分 original amount 与 current effective amount。

## 未选择方案

### 更新原 Cash Transaction 状态或金额

拒绝。它会把纠错结果写回根事实，削弱不可变历史并让外部流水、Allocation 和审计难以解释。

### 自动级联反向全部 Allocation

拒绝。批量隐式写入会隐藏每条纠错原因、失败位置和幂等边界，也扩大当前切片；管理员应先显式处理每条 active Allocation。

### 允许冲销后继续保留未分配原金额

拒绝。若原资金事实已失效，它不能继续作为可分配资金；current effective amount 和 unallocated amount 都必须为 0。

### 把退款或反向付款当成 Cash Reversal

拒绝。原收付款和后来反向移动都真实发生时，应保留两条实际资金事实；Reversal 只表达原登记不应继续有效。

### 使用负数 Cash Transaction

拒绝。它无法明确引用被纠正事实，也会把录入纠错和真实反向资金移动混在同一金额符号规则中。
