# ADR-0007：合同结算是独立的累计金额事实序列

日期：2026-09-19  
状态：接受，用于 Contract Ledger M2

## 背景

合同原始金额、当前生效文书形成的有效金额和双方后来确认的结算金额是三种不同事实。若用结算覆盖合同金额，会丢失文书历史；若把每次阶段结算都解释成增量后求和，又会错误处理现实中常见的“截至本次累计确认”文书。结算还可能发生在合同终止后，因此不能把终止等同于结算完成。

当前没有证据支持行业计量、税额、质保金或费用明细平台，也没有授权让 Contract Ledger 直接创建 Money Flow 义务。

## 决策

- Settlement 是 Contract Ledger 内部独立、不可变的事实，不是 Contract Instrument，也不形成 contract revision；
- 第一版支持 `progress` 阶段结算与 `final` 最终结算，两者金额都是截至该次确认的累计金额；
- 同一合同以从 1 开始的连续 `settlement_sequence` 排序，当前结算投影读取最新 sequence，不对阶段金额求和；
- `final` 至多一条，成功后拒绝任何新结算；
- 结算允许发生在 `active` 或 `terminated` 合同上，`draft`、`cancelled`、`closed` 不接受新增；
- 每条结算引用建立时的 source contract revision，继承该 revision 币种，并冻结用户明确选择的一个或多个确认主体快照；
- 命令同时携带 Expected Contract Revision 与 Expected Settlement Sequence，并在 Contract 行锁下核对；
- 结算、补充协议和终止文书共用 Contract 行锁形成确定顺序，但结算本身不推进 revision 或状态；
- Settlement、幂等命令、确认主体快照和可信 Actor 审计在一个 PostgreSQL 事务内提交并受不可变触发器保护。

## 金额语义

页面和领域模型始终分开表达：

1. revision 1 的原始合同金额；
2. 当前 revision 的有效金额；
3. 最新 Settlement 的累计结算金额。

三者可以相同、增加、减少或存在待确定值。结算金额允许为零，即使当前有效金额待确定也可按已确认文书记录。所有金额继续使用十进制文本与 PostgreSQL `numeric(24,6)`，不经过二进制浮点数。

## 后果

- 阶段结算不会被后续结算改写；后续事实只提供新的累计口径；
- 确认主体不是默认等于全部签约主体，必须由用户逐项选择；
- Party 当前资料或后续 contract revision 不会漂移既有结算快照；
- 最终结算错误不能直接修改或重新打开，需要未来独立的受控纠错事实；
- 合同终止后仍可最终结算，但最终结算不会自动把合同推进为 `closed`；
- 合同内结算文书编号不区分大小写唯一。

## 系统边界

结算不产生应收、应付、发票、资金、核销或义务建议，不调用 Money Flow Ledger，也不要求它在线。未来若要把结算用于义务建议，必须由可选连接器消费版本化结算契约，并另行定义幂等、失败恢复和端到端联调；该能力不进入当前 Settlement 写入事务。
