# ADR-0017：Allocation Reversal 是一次性不可变完整抵销事实

日期：2026-09-19  
状态：接受

## 背景

Money Flow 的 Allocation 已经是不可变核销事实，并由其正金额求和形成 Cash Transaction 的已分配金额和 Obligation 的已核销金额。M3 明确禁止编辑或删除错误 Allocation，但当时尚无纠错路径。M4 必须在不破坏历史、幂等和并发余额不变量的前提下恢复被错误占用的两侧容量。

若直接把 Allocation 改成 `cancelled`、删除记录或维护可写余额，历史会失真，当前余额也会出现第二真相源。若允许任意部分负数记录，则同一原 Allocation 可以产生复杂的多次抵销、超额反向和顺序语义，增加当前没有证据支持的状态空间。

## 决策

每条 Allocation 最多追加一条独立、不可变、完整等额的 Allocation Reversal：

- Reversal 通过外键引用原 Allocation，并复制其 Cash Transaction、Obligation、Obligation revision、精确金额和币种；
- 复制字段全部由 repository 从原 Allocation 得到，数据库触发器再次校验完全一致；
- 用户只提交 Allocation ID 和人工反向原因；Actor 与时间来自可信服务端；
- 原 Allocation 不变，状态由是否存在 Reversal 派生为 `active` 或 `reversed`；
- 当前余额使用 `SUM(allocation) - SUM(reversal)` 重建；Apply 的应用层和数据库校验使用相同净额；
- Reversal 与 Apply 固定按 Cash Transaction、Obligation 顺序加锁，同一两端的释放和重新分配串行检查；
- 一条 Allocation 的 Reversal 唯一；相同 Command 幂等重放，不同 Command 重复反向明确冲突；
- 若要把错误金额改小，先完整反向，再新增正确 Allocation，两个事实都留在历史中。

本决策不要求 Obligation 仍处于原 revision 或重新证明原 Allocation 的兼容性。Reversal 是对已形成关系的纠错；后续 Obligation Adjustment 必须在自己的设计中说明如何把所有历史 Allocation／Reversal 纳入当前和 `as_of` 投影。

## 结果

优点：

- 原始核销与纠错原因都可审计，不需要篡改或删除历史；
- 每条 Allocation 只有 active／reversed 两种可推导状态，余额公式简单且可由数据库保护；
- 完整反向后重新分配能够表达任意更正金额，同时避免部分抵销链；
- 为以后按时间重建余额保留了两个明确时间点。

代价与限制：

- 更正部分金额需要“反向 + 新分配”两个操作；当前不提供复合命令，两个动作之间存在可见但一致的暂态；
- 当前只计算最新净额，不提供 `as_of` 查询；
- Reversal 自身登记错误暂不支持再次反向，只能在后续纠错模型中扩展；
- 后续 Cash Reversal、Obligation Adjustment 和关闭规则必须显式考虑已经 active 或 reversed 的 Allocation。

## 否决方案

### 更新 Allocation 状态并从余额查询排除

否决。状态更新会改写已经发生的事实，无法独立证明谁在何时为什么纠正，也削弱当前不可变触发器。

### 删除错误 Allocation

否决。删除会丢失审计链、改变历史余额，并让幂等命令引用悬空。

### 写入负数 Allocation

否决。它混合“资金对应义务”的正向事实与“纠正某一条事实”的语义，且无法由外键保证抵销对象、金额上限和一次性约束。

### 第一版允许部分和多次 Reversal

当前否决。完整反向后重新分配已经覆盖金额更正用例；部分和多次抵销会引入累计上限、链顺序、并发争用和 UI 解释成本，尚无真实需求证据。
