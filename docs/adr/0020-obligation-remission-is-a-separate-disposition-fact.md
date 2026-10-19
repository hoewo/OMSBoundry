# ADR-0020：Obligation 减免是独立处置事实

日期：2026-09-19  
状态：接受

## 背景

Money Flow 已用 Obligation revision 表达义务依据金额变化，并用 Allocation／Reversal 表达实际资金核销。`MF-M4-003` 明确 current effective amount 是最新义务依据金额，不能让减免、坏账和关闭借用普通金额调整而失去各自语义。

减免虽然也降低 outstanding，但它并不一定说明原依据金额错误或范围变化。它表达债权人已经免除一部分应履行金额：对于应收，通常是本方放弃一部分债权；对于应付，通常是外部债权人免除本方一部分债务。系统中的 administrator 只是记录 Actor，不能被解释为法律上的减免方或审批人。

若把减免写成 `decrease amount_adjustment`，后续无法区分“合同／订单等依据金额减少”和“原义务仍可解释但部分已被免除”，报表也会丢失处置原因。若只在根表维护 remitted total，又会产生不可审计的第二真相源。

## 决策

减免使用独立、不可变的 `ObligationRemission`，不推进 Obligation revision：

- Remission 引用记录时的 current Obligation revision，保存正金额、当时 effective amount、币种、原因、人工来源依据、可信记录 Actor 和时间；
- 多笔 Remission 可以引用同一 revision；累计值只由不可变明细求和，不保存在 Obligation 根记录；
- current effective amount 继续只由 Obligation current revision 决定；remitted amount 独立求和；
- outstanding = current effective amount − Allocation/Reversal 净额 − Remission 累计；
- Remission 不得超过最新 outstanding；结果为零不自动关闭；
- Allocation 的义务上限必须扣除 Remission；金额减少 Adjustment 的结果下限必须同时包含净 Allocation 与 Remission；
- Remission、Amount Adjustment、Allocation Apply／Reverse 都通过 Obligation 行锁形成确定顺序；Remission 不取得 Cash Transaction 锁；
- Remission、Command 和审计原子提交且不可更新或删除；第一版不提供 Remission Reversal；
- `recorded_by` 只表示谁在本系统记录，不能推导谁作出减免、谁审批或减免是否合法。

## 结果

优点：

- 义务依据金额、实际资金核销和减免处置可以分别解释，不需要从一个负数调整猜测业务含义；
- 既有 revision 与 Allocation 历史保持稳定，当前余额仍可从明细完全重建；
- 后续坏账可以使用另一类处置事实和报表口径，不必污染 Remission；
- 未来受控关闭可以明确引用“资金核销归零”或“减免处置归零”的实际历史，而不把余额归零当作自动生命周期事件。

代价与限制：

- 所有读取义务余额和所有写入上限都必须同时纳入 Remission，数据库触发器也要维护同一组合不变量；
- Remission 不推进 revision，因此多个处置事实可以共享同一 Obligation revision；时间顺序由独立事实时间线表达；
- 第一版记录错误暂时没有反向入口，不能通过更新或删除修正；需要时必须追加独立 Remission Reversal；
- 当前不证明法律效力、不执行审批、不形成会计分录，也不提供税务分类。

## 否决方案

### 复用 `decrease amount_adjustment`

否决。它会把依据金额变化和债务免除混为同一类事实，破坏 `MF-M4-003` 已锁定的语义边界，并让坏账、折扣和关闭继续向万能 Adjustment 聚集。

### 为 Remission 追加新的 Obligation revision

否决。revision 当前描述义务依据的完整快照；Remission 不修改标题、主体、到期日、来源或依据金额。强行推进 revision 会制造没有条款变化的新版本，并让多个同 revision 处置无法自然组合。

### 只在 Obligation 根表更新 `remitted_amount`

否决。可写汇总无法解释每次金额、原因、来源和 Actor，也不能可靠幂等或反向重建。

### 把减免和坏账合成一种 `write_off`

否决。减免表示义务被免除；坏账表示应收的可收回性处置，未必消灭原权利。两者可能影响相似余额，但原因、适用方向、权限和报告口径不同。

### outstanding 归零时自动关闭

否决。资金结清、全额减免、未来坏账处置和操作性关闭是不同事实。关闭必须在独立功能中明确前置条件、Actor、原因和历史引用。
