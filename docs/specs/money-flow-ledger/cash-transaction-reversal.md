# Money Flow Ledger：Cash Transaction 完整冲销

里程碑：M4  
功能编号：MF-M4-002  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够纠正一条不应继续作为当前资金事实生效的人工 Cash Transaction：追加一条引用原 Cash Transaction 的不可变 Reversal，令该资金的当前有效金额和未分配金额归零，同时永久保留原登记、历史 Allocation、Allocation Reversal 和审计链。

本功能只处理“原资金事实登记错误或最终确认并未发生”的完整冲销。若资金确实发生，之后又发生退款、拒付扣回或反向付款，后续移动必须登记为另一条实际 Cash Transaction；不能用本功能抹去两次真实资金移动。

## 核心建模决策

- Cash Transaction Reversal 是原资金事实的完整失效记录，不是负数 Cash Transaction、退款、反向付款、银行流水状态、Allocation Reversal 或会计凭证；
- 一条 Cash Transaction 最多有一条 Reversal；Reversal 的方向、金额和币种必须精确复制原事实，用户不能提交或覆盖；
- 原 Cash Transaction 永久保留。没有 Reversal 时派生状态为 `recorded`，存在 Reversal 时派生状态为 `reversed`；状态不是根表可写字段；
- recorded 资金的 current effective amount 等于原金额；reversed 资金的 current effective amount 和 unallocated amount 都等于 0；
- Cash Reversal 只能在该资金的净 allocated amount 为 0 时发生。若仍存在 active Allocation，管理员必须先逐条使用 MF-M4-001 完整反向；
- Cash Reversal 生效后，Allocation Apply 的应用层和数据库触发器都必须拒绝该资金，不能因历史 Allocation 已释放而重新分配；
- Reverse Cash、Apply Allocation 与 Reverse Allocation 都先锁 Cash Transaction。Apply／Reverse Allocation 再按既定顺序锁 Obligation，Cash Reverse 在持有 Cash 锁时核对净 Allocation，因此三类命令形成确定顺序。

详细依据见 [ADR-0018](../../adr/0018-cash-transaction-reversal-invalidates-only-unallocated-facts.md)。

## 业务事实

- Reversal ID、Command ID 和 Audit ID 是三个不同 UUID；
- 冲销原因必填，去除首尾空白后长度为 1 至 200 个字符；原因说明为何原登记不应继续生效，不替代 Evidence 或审批；
- 来源类型固定为 `manual`；
- `reversed_at` 由服务端可信 UTC 时钟生成，客户端不能提交或覆盖；
- Reversal 冻结原 Cash Transaction 的 ID、方向、精确金额和币种；
- 金额严格等于原 Cash Transaction 的正金额，数据库继续使用 `numeric(24,6)`，领域和页面不经过二进制浮点数；
- 新 Reversal 立即生效且不可更新或删除；本轮没有草稿、部分冲销、撤回 Reversal、Reversal 的 Reversal 或物理清理；
- 原外部流水唯一范围继续被原 Cash Transaction 占用。冲销不允许用另一 Command 重新登记同一外部流水并伪装成没有历史。

## 当前投影

```text
cash net allocated amount
  = SUM(Allocation.amount) - SUM(AllocationReversal.amount)

cash current effective amount
  = original CashTransaction.amount       when no CashTransactionReversal
  = 0                                     when reversed

cash unallocated amount
  = current effective amount - net allocated amount
```

- Reversal 写入前要求 net allocated amount 精确为 0，因此 reversed 资金的 unallocated amount 必为 0；
- 列表和详情继续显示原始金额，同时明确显示 current effective amount、`reversed` 状态和 Reversal 事实；
- 当前投影不删除历史 Allocation。已经完整反向的 Allocation 及其 Reversal 仍可从资金详情下钻；
- 本轮不实现 `as_of` 查询，但原登记时间和冲销时间为后续历史投影保留事实基础。

## 用户流程

```text
管理员登录
  → 打开 Cash Transaction 详情
  → 若净 allocated amount > 0，先进入每条 active Allocation 完成反向核销
  → 返回资金详情，确认净 allocated amount = 0
  → 填写完整冲销原因并确认
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放
  → 锁定原 Cash Transaction
  → 校验原事实存在、尚未冲销且净 allocated amount = 0
  → Reversal、命令和审计在同一事务提交
  → 返回资金详情，状态派生为 reversed
  → 当前有效金额和未分配金额为 0，历史事实保持可见
```

## 幂等、并发与数据库保护

- 每次冲销意图携带 UUID Command ID；
- 幂等负载由标准化后的 Cash Transaction ID、冲销原因、来源类型和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同负载串行或并发重放返回原 Reversal，不重复失效或写审计；
- 相同 Command ID 改变 Cash Transaction、原因或 Actor 返回显式幂等冲突；
- 不同 Command ID 串行或并发冲销同一 Cash Transaction 时恰好一个成功，另一个得到包含既有 Reversal ID 的显式状态冲突；
- Cash Reverse 在 Command 锁后取得 Cash Transaction 行锁，再读取净 Allocation；Allocation Apply 和 Allocation Reverse 也先取得同一 Cash 行锁；
- 若 Cash Reverse 先提交，后续 Apply 在取得 Cash 锁后看到 Reversal 并拒绝；若 Apply 先提交，Reverse 看到非零净 allocated amount 并拒绝；
- PostgreSQL Reversal 插入触发器锁定 Cash Transaction，校验复制的方向、金额与币种完全等于原事实，检查净 Allocation 为 0，并拒绝第二条 Reversal；
- PostgreSQL Allocation 插入触发器必须拒绝已冲销资金，直接 SQL 不能绕过；
- Reversal、创建命令和审计原子提交；缺失事实、仍有 active Allocation、重复冲销、数据库或审计失败均不留下部分事实；
- Cash Transaction、Cash Reversal、两类创建命令和审计全部不可 UPDATE／DELETE。

## 权限、审计与错误边界

- GET `/cash-transactions/{id}` 继续显示详情；POST `/cash-transactions/{id}/reverse` 只允许 Money Flow 本地 administrator；
- POST 必须通过 CSRF；Actor 只来自登录 Principal，客户端提交的方向、金额、币种、状态、Actor 和时间全部无效；
- `cash_transaction_reversed` 审计保存 Command ID、可信 Actor、Reversal ID、原 Cash Transaction、方向、精确金额、币种和原因；
- Cash Transaction ID 非法或事实不存在返回 404；验证错误返回 422；Command 冲突、仍有净 Allocation 或已经冲销返回 409；数据库失败返回 500；
- 详情读取失败不能把已冲销资金渲染为 recorded，也不能丢失 Reversal；
- 本功能只访问 Money Flow 数据库，不访问 Contract Ledger。

## 明确不做

- 部分 Cash Reversal、负数 Cash Transaction、编辑或删除原资金事实；
- 撤回 Reversal、Reversal 的 Reversal、链式冲销；
- 退款、拒付扣回、反向付款、银行流水状态或银行对账；
- 自动批量反向 Allocation、自动重建正确 Cash Transaction 或自动匹配；
- Obligation adjustment、减免、坏账、关闭或 revision 2；
- 票据、账户余额、`as_of` 查询、统计报表、导入导出；
- 文件 Evidence、审批、会计总账、跨币种或 Contract 连接器。

## 验收标准

1. administrator 可以从净 allocated amount 为 0 的 recorded Cash Transaction 详情追加完整 Reversal；原 Cash Transaction 保留，列表和详情显示 `reversed`、原因、可信 Actor 与时间；
2. Reversal 的方向、金额和币种只能从原 Cash Transaction 复制，客户端伪造字段无效；
3. 冲销后的 current effective amount、allocated amount 和 unallocated amount 都精确为 0，原始金额、发生日、主体快照和来源不变；
4. 仍有 active Allocation 时冲销返回明确冲突且不写任何 Reversal／Command／审计；逐条反向 Allocation 后可以冲销；
5. Cash Reversal 生效后，页面不再把该资金列为 Allocation 候选，构造请求和直接 SQL 也不能新增 Allocation；
6. 一条 Cash Transaction 最多完整冲销一次；不同 Command 串行或并发尝试时恰好一个成功；
7. 相同 Command ID／负载串行或并发重放只形成一条 Reversal、一条 Command 和一条审计；改变负载或 Actor 返回冲突；
8. 无效或缺失 Cash Transaction、空或过长原因、重复冲销、审计失败和数据库失败均不留下半成品；
9. Cash Transaction、Reversal、Command 和审计不可更新或删除；直接 SQL 复制错误方向、金额或币种被数据库拒绝；
10. Cash Reverse 与 Allocation Apply／Reverse 并发时形成确定结果：要么分配先成功而冲销被拒绝，要么冲销先成功而分配被拒绝，不出现 reversed 且 net allocated amount > 0；
11. HTTP 覆盖登录、administrator、CSRF、可信 Actor、成功、重放、冲突、404、422 和数据库失败；
12. migration 从 `000009` 升至 `000010` 并可重放，既有 Cash Transaction 初始均为 recorded，原 Party、Obligation、Allocation、Allocation Reversal、Internal Transfer 和余额保持不变；
13. 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation、Cash Transaction、Allocation、Allocation Reversal、Cash Reversal、投影核对和冲销后分配拒绝；
14. Contract Ledger 停止时上述 Money Flow 流程和 ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器联调；
15. `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
