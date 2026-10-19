# ADR-0019：Obligation 金额调整追加连续 revision

日期：2026-09-19  
状态：接受

## 背景

Money Flow 已把正式 Obligation 建模为不可变 revision 1，并以根记录的 `current_revision` 选择当前投影。Allocation 在形成时引用明确的 Obligation revision，当前余额则由当前义务金额减去 Allocation／Reversal 净额计算。M4 需要支持义务依据变化后的金额增加或减少，但不能覆盖已经确认的金额、破坏既有 Allocation 历史，或把减免、坏账和关闭混成一个万能状态。

如果只在根记录维护一个可写 current amount，revision 历史会失去意义，Allocation 也无法解释当时依据。若只追加独立 delta 而不推进 revision，后续 Allocation 的 Expected Revision 无法表达它是基于哪一个当前金额形成。若允许负数金额同时表达方向，则校验、页面和审计都更容易把“减少义务”误解为负义务。

## 决策

每次金额调整追加一条不可变 `ObligationAdjustment`，并从当前 revision N 连续形成完整快照 revision N+1：

- Adjustment 使用 `increase` 或 `decrease` 类型和始终为正的 adjustment amount；
- 新 revision 复制 source revision 的全部非金额事实，只把 effective amount 设置为精确计算结果；
- 根记录 original amount 保留最初录入值；当前 effective amount 只来自 current revision；
- revision 1 标记为 `confirmation`，后续金额 revision 标记为 `amount_adjustment`，并保存明确 source revision、可信 Actor 和形成时间；
- 减少结果必须大于零且不得低于当前 Allocation 净额；增加结果必须处于 `numeric(24,6)` 范围；
- 既有 Allocation 继续引用形成时 revision，不迁移、不改写；新 Allocation 必须消费最新 revision；
- effective amount 等于净分配额只代表 outstanding 为零，不自动关闭义务；
- 每个 source revision 只能有一个后继 Adjustment，多次调整通过连续 revision 表达；
- Adjustment 与 Allocation Apply／Reverse 共用 Obligation 行锁形成顺序，但 Adjustment 不取得 Cash Transaction 锁；
- repository 与 PostgreSQL 触发器都校验 revision 连续性、精确算术、字段复制和净分配下限。

为使 revision 字段语义覆盖确认和调整，schema 把 revision 中的金额明确命名为 effective amount，把 Actor／时间明确命名为 recorded by／at；旧 revision 1 在 migration 中无损标记为 confirmation。`source_draft_version` 只属于 confirmation revision，Adjustment revision 改用 `source_revision`。

## 结果

优点：

- 初始金额、每次变化原因和任一 Allocation 当时引用的金额版本都可以独立解释；
- current projection 仍只依赖一个连续 revision，不引入可写汇总字段；
- 增加和减少共享同一套幂等、权限、审计和并发约束，同时保持业务方向明确；
- 后续 Contract 连接器可以提出“基于 source revision 的调整建议”，但不能覆盖 Money Flow 已确认事实；
- revision 与时间事实为后续 `as_of` 投影保留稳定基础。

代价与限制：

- 即使只变化一个金额，新 revision 仍复制完整快照；这是为换取稳定读取和历史解释接受的局部重复；
- 当前不支持撤回 Adjustment；录错后只能再追加相反方向的新 Adjustment，并保留两次事实；
- outstanding 为零仍需未来独立的受控关闭能力；
- 减免、坏账和折扣即使也会降低余额，仍需独立语义和报表分类，不能复用本 Adjustment 类型冒充。

## 否决方案

### 原地更新 Obligation 或 revision 金额

否决。它会覆盖已经确认的事实，让旧 Allocation 的 revision 引用失去可解释含义，也无法重建历史余额。

### 只追加 delta，不推进 Obligation revision

否决。后续 Allocation 无法通过 Expected Revision 证明使用了哪个当前金额，并会形成“revision 与当前金额”两套时间轴。

### 使用正负金额，不保存调整类型

否决。负数把方向隐藏在表示形式中，容易与负义务、冲销或会计借贷混淆；显式类型配合正金额更易验证。

### 允许调整后低于净 Allocation，再自动反向超出部分

否决。自动选择和反向历史 Allocation 会把一个金额调整变成不可见的多事实级联。管理员必须先显式纠正 Allocation，或选择不低于净分配额的结果。

### 金额归零即自动关闭

否决。余额为零、减免完毕、坏账处置和操作性关闭是不同事实；本功能不替代未来关闭模型。
