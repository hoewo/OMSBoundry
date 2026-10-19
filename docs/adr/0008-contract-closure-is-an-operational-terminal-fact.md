# ADR-0008：合同关闭是操作性终态事实

日期：2026-09-19  
状态：接受，用于 Contract Ledger M2

## 背景

合同取消、法律终止、最终结算和日常管理结束不是同一件事。只用 `active/terminated` 无法表达“本系统中不再接受普通写入”，但把关闭自动等同于履约完成、到期、结清或终止又会制造未经确认的法律与财务事实。

部分合同有正式最终结算文书，另一些合同没有独立 final Settlement。强制所有合同先登记 final 会诱导用户伪造零金额或不存在的文书；完全不检查结算又可能让用户无意中把仍待结算的合同封闭。

## 决策

- Contract Closure 是独立、不可变的操作事实，不是 Contract Instrument，不产生 revision；
- Closure 可以从 active 或 terminated 状态发生，并把 Contract 生命周期投影推进为 closed；
- closed 表示不再接受补充协议、终止、结算和再次关闭，不自动证明法律终止、履约完成或财务清偿；
- 关闭命令必须声明 `final_recorded` 或 `not_required`，repository 根据当前 Settlement 事实验证声明；
- `final_recorded` 引用真实 final Settlement；`not_required` 只在没有 final Settlement 时成立；
- 命令同时消费 Expected Contract Revision 与 Expected Settlement Sequence，并通过 Contract 行锁同补充、终止和结算串行化；
- 关闭日期不得早于当前 Instrument 生效日期或最新 Settlement 确认日期；
- Closure、幂等命令、Contract 状态更新和可信 Actor 审计在一个 PostgreSQL 事务内提交并受不可变保护。

## 为什么不要求 final Settlement

并非每份合同都有独立最终结算文书。要求 final 会把“系统工作流前置条件”伪装成“业务事实”，并导致虚假数据。`not_required` 是操作者对当前合同是否还需要本系统结算写入的明确决定，不创建金额事实，也不删除已有 progress 历史。

## 为什么关闭不产生 revision

revision 表示法律文书生效后的完整合同条款快照。操作性关闭没有改变合同条款，也不应伪造一份文书。Closure 引用关闭时的 current revision 和 settlement sequence，已经足以回答关闭基于哪一版合同与哪一组结算事实。

## 后果

- active 合同可以在无需终止文书时关闭；terminated 合同也可以在收尾后关闭；
- final Settlement 仍不会自动关闭合同，关闭仍需独立、显式命令；
- 关闭后历史查询、关系查看和审计继续可用；
- 已有关系仍可读取，关系本身的历史补录继续遵守关系功能自己的正式合同规则；
- 错误关闭不能直接重新打开，必须等待独立受控纠错规格；
- 自然到期不会由后台任务自动推断，避免时区、日期和法律语义误判。

## 系统边界

关闭完全由 Contract Ledger 拥有，不读取 Money Flow 内部数据，也不以“资金已经核销”为前置条件。未来客户若要求关闭前检查外部义务，只能通过可选决策门禁提供非越权证明，不能让另一个系统成为 Contract Ledger 的启动或核心关闭前提。
