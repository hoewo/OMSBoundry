# ADR-0022：Obligation 关闭是显式零余额终态事实

日期：2026-09-19  
状态：接受

## 背景

Money Flow 已能通过 Allocation／Reversal、Amount Adjustment 和 Remission 把 Obligation outstanding 精确归零，但各功能均明确拒绝自动关闭。余额是可重建投影，关闭则表示本系统不再接受常规余额写入；两者不是同一个事实。

只消费 current revision 不足以保护关闭意图。Allocation、Allocation Reversal 和 Remission 都会改变余额，却不推进 Obligation revision。用户看到余额为 0 后，其他操作者可能反向 Allocation，再用新资金重新归零；若仍接受旧页面的关闭命令，系统无法证明操作者关闭的是自己看到的那组事实。

坏账在 ADR-0021 中已经延期，不能作为本功能的隐含关闭依据。

## 决策

- 只有 `open` 且 `effective amount = net allocated + remitted amount` 的 Obligation 可以显式关闭；
- 关闭使用独立、不可变的 `ObligationClosure`，不生成 revision，不修改任何金额事实；
- Closure 冻结 current revision、余额事实 sequence、effective、net allocated、remitted、currency、派生 settlement basis、原因、可信 Actor 与时间；
- balance sequence 定义为 Allocation、Allocation Reversal 与 Obligation Remission 三类不可变事实的数量之和；命令同时消费 Expected Current Revision 与 Expected Balance Sequence；
- 关闭与全部余额写入通过 Obligation 行锁串行化；Closure 不取得 Cash Transaction 锁；
- Closure、命令、`closed` 状态和审计在同一事务提交，数据库延迟约束禁止只形成部分关闭；
- `closed` 是 Money Flow 的操作性终态：禁止调额、减免、Allocation Apply／Reversal 和再次关闭；第一版不重新打开；
- 关闭不证明法律终止、履约完成、会计结账或 Contract Ledger 状态，也不触发跨系统动作。

详细行为见 [Obligation 受控关闭规格](../specs/money-flow-ledger/obligation-controlled-closure.md)。

## 结果

优点：

- 零余额与生命周期决定保持分离，不会因某次核销自动封闭义务；
- revision 与 balance sequence 共同保护用户看到的完整关闭前置事实；
- closed 不会在后续 Allocation Reversal 后悄悄恢复非零余额；
- Allocation、Remission 和 Adjustment 历史完全保留，关闭构成可解释；
- 不引入 Contract Ledger 运行依赖或虚假跨系统结清证明。

代价与限制：

- 所有余额写入路径和数据库触发器都必须认识 closed；
- 关闭错误目前无法重新打开，只能等待未来独立纠错规格；
- 新增余额事实类型时必须同步扩展 balance sequence，否则关闭并发保护不完整；
- 坏账延期期间，不能以坏账为依据关闭仍有 settlement outstanding 的义务。

## 否决方案

### outstanding 归零时自动关闭

否决。Allocation Reversal、调额或录入纠错都可能在归零后发生；自动关闭会把计算结果误当成操作者的生命周期决定。

### 关闭只检查 Expected Current Revision

否决。Allocation、Reversal 和 Remission 不推进 revision，旧页面无法发现余额事实已经替换。

### 关闭后仍允许 Allocation Reversal

否决。它会产生 `closed` 且 outstanding 非零的矛盾状态。错误关闭或错误 Allocation 需要未来受控纠错链，不能原地打破终态不变量。

### 关闭时创建一份新 revision

否决。关闭没有改变义务依据、金额、主体或到期日；伪造 revision 会混淆依据变化与操作生命周期。

### 读取 Contract Ledger 后才能关闭

否决。Money Flow 必须独立运行；未来若客户需要外部证明，只能通过可选门禁或版本化快照，不得直接读库或形成启动依赖。
