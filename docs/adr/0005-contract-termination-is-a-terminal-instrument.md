# ADR-0005：合同终止是独立的终态 Instrument

日期：2026-09-19  
状态：接受，用于 Contract Ledger M2

## 背景

补充协议已经能够按显式顺序形成 revision 2+，但合同仍只有 `active` 当前状态，无法记录一份已经生效的终止文书。若只修改根表状态，会丢失终止依据、顺序、Actor 和当时有效条款；若把终止塞进普通补充协议自由文本，又无法可靠阻止终止后的写入或区分结算。

## 决策

- 新增类型化 `termination_instrument`，它与原合同、补充协议共享不可变 Instrument envelope，但使用独立命令和业务校验；
- 终止文书连续消费当前 revision，并形成下一份完整 revision；
- 新 revision 保持上一 revision 的有效金额和条款，复制 Party Snapshot；
- 成功写入时 Contract 状态从 `active` 原子变为 `terminated`；
- `terminated` 是法律文书驱动的终态，不能继续追加补充协议或再次终止；
- `cancelled` 继续只表示未激活草稿取消；`closed` 保留给后续结算／归档完成后的操作性关闭；
- 终止原因是类型化命令中的必填字段，落在 Instrument 的说明载荷中；
- 终止命令、Instrument、revision、快照、状态推进和审计必须原子提交且不可变。

## 为什么终止仍产生 revision

即使金额和一般条款没有变化，合同在该文书生效后已经从“有效”变为“已终止”。让终止 Instrument 对应一个明确 revision，可以保持“每份生效文书产生一个版本”的重放规则，并固定终止时点的条款、金额和签约主体快照。

## 为什么不复用补充协议命令

补充协议的结果是合同继续 active，并允许金额 adjustment、restatement 或 unchanged；终止的结果是进入不可继续变更的终态，金额只能保持不变。两者拥有不同前置条件和结果，因此只共享稳定的 Instrument/revision 写入机制，不共享无约束业务命令。

## 为什么终止不处理结算

终止回答“合同何时、依据什么文书不再继续有效”；结算回答“截至某个口径最终应结、已结和未结多少”。把二者合并会覆盖原始／当前有效金额，并把 Contract Ledger 与收支事实耦合。结算必须在后续独立规格中建模，必要时再通过版本化契约与 Money Flow 协作。

## 后果

- Contract 状态约束增加 `terminated`；
- Instrument 类型约束增加 `termination_instrument`，金额语义固定为 `unchanged`；
- 新增独立 termination command 和 `contract_terminated` 审计；
- 查询把 terminated 合同继续按 revision-backed 合同读取；
- 补充协议与终止写入竞争同一 Contract 行锁和 Expected Revision，保证只有一个顺序事实成功；
- 恢复、撤销终止、自然到期和关闭都需要后续独立决策，不能直接修改终止事实。

## 系统边界

本决策只影响 Contract Ledger 内部生命周期，不创建应收应付、退款、结算或核销，不要求 Money Flow Ledger 在线。未来出现真实跨系统业务动作时，仍必须通过版本化快照/API/事件、幂等接收、失败恢复和联调测试实现。
