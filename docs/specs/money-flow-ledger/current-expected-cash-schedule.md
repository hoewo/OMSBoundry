# Money Flow Ledger：当前预计现金排期

日期：2026-09-20  
功能编号：MF-M4-015  
状态：已实现并验证

## 1. 用户结果

已登录的本地 administrator 可以为一条 `open` 且当前未结余额大于 0 的 Obligation 设置、改期或清除预计现金日期，并在一个页面查看预计流入、预计流出和未排期余额。

本功能只记录经营排期判断。`due_on`、Obligation revision、资金事实、核销、减免和关闭均不被修改。

## 2. 稳定不变量

- Expectation 是不可变追加事实，同一 Obligation 的 sequence 从 1 连续递增；
- `set` 必须有有效的 `expected_on`，`clear` 的 `expected_on` 必须为空；
- 每次 `set`／`clear` 都必须有 1–200 字的原因；
- 写入时 Obligation 必须为 `open`，并与表单携带的 Expected Current Revision 一致；
- `set` 时 current outstanding 必须大于 0；
- `clear` 只允许清除一个当前有效的 `set`，不能连续清除或清除从未设置的排期；
- 当前事实由最大 sequence 唯一确定，不通过覆盖或删除旧行实现；
- 记录时间在同一 Obligation 的 Expectation 历史内非递减；
- 预计金额只从查询时的 current outstanding 派生，不写入 Expectation；
- 状态不是 `open` 或 current outstanding 为 0 的 Obligation 不进入当前排期；
- 没有当前 `set` 的 open 正余额进入“未排期”，不能使用 `due_on` 自动替代；
- 汇总键为 `expected_on + ledger_owner_party_id + direction + currency`；未排期汇总键不含日期；
- 应收只形成预计流入，应付只形成预计流出；不同本方主体和币种绝不合并金额。

## 3. 命令与幂等

规范化负载包含：

- Cash Expectation action；
- Obligation ID；
- Expected Current Revision；
- `expected_on`（仅 `set`）；
- 原因；
- `source_kind=manual`；
- 可信 Actor kind 和 ID。

负载使用带格式版本的确定性 JSON 计算 SHA-256。同一 Command ID、同一负载返回原 Expectation；同一 Command ID、不同负载返回 `409`。客户端提交的 actor 字段无效，Actor 只来自登录 Principal。

## 4. 持久化与并发

- Command ID 先取得 transaction advisory lock；
- 首次命令在同一事务中锁定 Obligation 行，检查状态、revision、余额和最新 Expectation；
- sequence 在锁内由最新 sequence 加一得到；
- Expectation、command row 和 audit entry 必须原子提交；
- 数据库触发器重复保护状态、revision、余额、action/date 配对、连续 sequence、清除前置状态和记录时间顺序；
- Expectation 与命令表都禁止 UPDATE 和 DELETE。

Expectation 与 Allocation、Remission、Adjustment、Closure 通过同一 Obligation 行锁串行，因此不会在关闭后写入排期，也不会绕过 current revision 和 current balance 检查。

## 5. 当前投影

页面 `/cash-forecast` 包含：

1. 设置／改期表单；
2. 按预计日、本方、方向、币种汇总的已排期金额；
3. 可下钻到 Obligation 的已排期明细；
4. 按本方、方向、币种汇总的未排期金额；
5. 未排期明细；
6. 按 Obligation 查询的完整 Expectation 历史；
7. 当前有效排期的清除表单。

每次读取在同一只读 `REPEATABLE READ` 快照中取得 current Obligation 余额和最新 Expectation。排序必须确定：预计日、本方、方向、币种、Obligation ID；未排期将日期放在最后。

## 6. 明确不做

- 使用 `due_on` 自动生成预计日期；
- 一条 Obligation 拆成多个预计日期或计划金额；
- 置信度、乐观／基准／悲观场景；
- 银行余额起点、账户维度、流动性缺口或融资建议；
- 自动承诺付款、审批或提醒；
- 跨币种换算和汇总；
- 修改历史 Expectation；
- Contract Ledger 或外部系统自动写入。

## 7. 验收口径

1. administrator 可设置、改期和清除预计日期，页面显示可信 Actor、原因、sequence 和时间；
2. `due_on` 与 Obligation revision 在上述操作前后保持不变；
3. 同命令重放不新增事实，同命令不同负载冲突；
4. draft、closed、revision 过期、零余额 set、无当前排期 clear 均被明确拒绝且不产生部分事实；
5. 同一 Obligation 的并发写入形成确定的连续 sequence 或明确冲突，不丢历史；
6. Allocation／Remission 后预计金额随 current outstanding 变化；余额归零后从当前投影消失；
7. 未设置日期的 open 正余额只进入未排期，不落入任一日期；
8. 汇总能够由明细逐项重建，并严格隔离本方、方向和币种；
9. `000015` migration 可从 `000014` 升级并重放，既有事实和历史余额结果不变；
10. 本方主体台账导出包含完整 Expectation 历史及对应审计；
11. 真实 PostgreSQL HTTP 流程覆盖设置、改期、部分核销后的投影变化、清除和历史；
12. Contract Ledger 停止时 Money Flow 可独立完成全流程，两个系统全量回归通过。
