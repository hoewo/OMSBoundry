# ADR-0031：预计现金日期是独立经营事实

日期：2026-09-20  
状态：accepted

## 背景

Money Flow Ledger 已经能够展示义务到期日、当前未结余额和当前经营收支汇总，但尚不能回答“按当前判断，哪一天预计收到或付出这笔未结金额”。

直接把 Obligation `due_on` 当作预计现金日期会混淆条款事实和经营估计；把预计金额保存为可写汇总字段，则会在 Allocation、Reversal、Remission 和 Adjustment 后形成第二套余额真相。

## 决策

新增不可变、按 Obligation 递增 sequence 的 Cash Expectation 事实：

- `set` 保存一个明确的 `expected_on` 和本次判断原因；
- `clear` 取消当前排期，同样必须保存原因；
- 每次操作冻结当时的 Obligation revision、可信 Actor 与系统记录时间；
- 同一 Command ID 和同一规范化意图安全重放，不同意图冲突；
- 当前排期只取每条 Obligation 的最新 Expectation；最新动作为 `clear` 时视为未排期；
- 当前预计金额不写入 Expectation，而是查询时从 current outstanding 派生；
- open 正余额没有当前 `set` 时进入未排期集合；余额归零或状态不再 open 时从当前投影移除，但历史事实不删除；
- 所有汇总按本方账簿主体、应收／应付方向和币种隔离。

## 结果

预计日期可以独立调整且不会修改合同式到期日；部分收付和减免会自然改变预计金额，不需要同步更新计划金额。用户能够看见未排期缺口，而系统不会用到期日偷偷补值。

代价是第一版只能表达“一条义务的全部当前未结余额预计在一个日期发生”，不能表达分期、多概率场景或人为覆盖预计金额。出现真实分期案例后，应追加独立计划行模型，而不是改变既有 Expectation 历史。

本决策不引入银行余额预测、付款承诺、审批、会计凭证或 Contract Ledger 依赖。

