# Money Flow Ledger：Obligation 受控关闭

里程碑：M4  
功能编号：MF-M4-006  
日期：2026-09-19  
完成日期：2026-09-20  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够在一条 settlement outstanding 已精确归零的 `open` Obligation 上记录显式、不可变的操作性关闭事实，并把义务状态推进为 `closed`。

关闭只表示本系统不再接受该义务的常规余额写入。它不自动证明合同已经终止、履约已经完成、法律权利已经消灭、会计已经结账或外部系统已经同步。

## 核心建模决策

- outstanding 归零不会自动关闭；关闭必须由 administrator 提交独立 Command；
- 第一版只接受当前公式 `effective amount - net allocated - remitted amount = 0`；延期的坏账不参与关闭；
- Closure 是独立、不可变事实，不生成 Obligation revision，不修改金额、Allocation、Reversal 或 Remission；
- Closure 冻结关闭时的 current revision、effective amount、net allocated、remitted、currency、balance sequence、关闭构成、原因、可信 Actor 和时间；
- 关闭把 Obligation 根状态从 `open` 推进为 `closed`；历史投影继续由原事实重建；
- `closed` 禁止 Amount Adjustment、Remission、Allocation Apply、Allocation Reversal 和再次关闭；第一版不提供重新打开；
- 关闭与所有余额写入共用 Obligation 行锁。Closure 不取得 Cash Transaction 锁，不改变 Cash → Obligation 的稳定锁序。

详细依据见 [ADR-0022](../../adr/0022-obligation-closure-is-an-explicit-zero-balance-terminal-fact.md)。

## 余额版本

Amount Adjustment 会推进 current revision，但 Allocation、Allocation Reversal 和 Remission 不推进 revision。关闭命令因此同时消费：

```text
expected current revision

expected balance sequence
  = COUNT(Allocation)
  + COUNT(AllocationReversal)
  + COUNT(ObligationRemission)
```

这些事实均不可删除，balance sequence 单调递增。即使余额经过“反向后重新分配”再次回到 0，sequence 也会变化，基于旧页面提交的关闭命令会得到显式冲突。

未来若增加新的余额事实或 Reversal，必须在同一 migration 中扩展 sequence 定义，不能静默遗漏。

## 关闭事实

每条 Closure 保存：

- Closure ID、Obligation ID 与关闭时 current revision；
- 关闭时 balance sequence；
- 关闭时 effective amount、net allocated amount、remitted amount 与 currency；
- 服务端派生的 settlement basis：`allocated`、`remitted` 或 `mixed`；
- 固定 `manual` 来源类型和用户明确填写的关闭原因；
- 可信 `closed_by` Actor 与服务端关闭时间。

同一 Obligation 最多一条 Closure。Closure、Command 和审计必须原子提交且不可更新或删除。

## 关闭构成

- `allocated`：net allocated 等于 effective，remitted 为 0；
- `remitted`：remitted 等于 effective，net allocated 为 0；
- `mixed`：net allocated 与 remitted 均大于 0，合计等于 effective。

构成由 repository 和数据库从最新事实派生，客户端不能提交或覆盖。

## 用户流程

```text
管理员登录
  → 打开 open Obligation 详情
  → 查看 current revision、effective／allocated／remitted／outstanding
  → outstanding 为 0 时显示受控关闭表单
  → 输入关闭原因
  → 提交 Command ID + Expected Current Revision + Expected Balance Sequence
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放
  → 锁定 Obligation，重读最新 revision、sequence 和余额
  → 校验仍为 open、sequence 未变化、outstanding 仍为 0
  → 写入 Closure、Command、closed 状态和审计
  → 返回只读详情并展示关闭事实
```

## 并发与状态边界

- Closure 与 Allocation Apply：关闭前余额已经为 0，Apply 无剩余容量，不能成为一个成功的竞争写入；Closure 先提交时 Apply 还会因 `closed` 拒绝；
- Closure 与 Allocation Reversal：Reversal 先提交则释放余额、Closure 拒绝；Closure 先提交则 Reversal 因 `closed` 拒绝；
- Closure 与 Remission：关闭前余额已经为 0，Remission 无可处置余额，不能成为一个成功的竞争写入；Closure 先提交时 Remission 还会因 `closed` 拒绝；
- Closure 与 Amount Adjustment：Adjustment 先提交会推进 revision，旧 Closure 命令冲突；Closure 先提交则 Adjustment 因 `closed` 拒绝；
- 同 Command ID／相同负载重放返回原 Closure；改变任一业务字段或 Actor 返回幂等冲突；
- 不同关闭命令并发时恰好一个成功，另一个看到 `closed`。

## 数据库保护

- Closure 插入触发器锁定 Obligation，并重新计算 revision、sequence、effective、net allocated、remitted、currency、basis 与零余额；
- `open → closed` 状态更新必须引用已经插入的 Closure，关闭时间与根记录 `updated_at` 一致；
- 延迟约束确保事务提交时 Closure 与 `closed` 状态同时存在，直接 SQL 不能只插 Closure 不更新状态；
- Allocation、Remission 与 Adjustment 已有触发器继续要求 `open`；本功能补齐 Allocation Reversal 的 `open` 检查；
- Closure、Command 与审计使用现有不可变触发器；根状态不能从 `closed` 直接改回其他状态。

## 权限、HTTP 与错误边界

- GET `/obligations/{id}` 显示状态、归零构成和 Closure 历史；
- POST `/obligations/{id}/close` 只允许 Money Flow 本地 administrator；
- POST 必须通过 CSRF，请求体限制为 16 KiB，Actor 只来自登录 Principal；
- Obligation ID 非法或不存在返回 404；输入错误返回 422；非 open、revision／sequence 过期、余额非零或幂等冲突返回 409；数据库失败返回 500；
- 失败时重读最新详情并生成新 Command ID，不能复用已改变负载的旧命令；
- Allocation 详情在关联 Obligation 已 closed 时不再显示反向核销表单；构造 POST 仍由 repository 与数据库拒绝；
- 本功能只访问 Money Flow 数据库，不读取 Contract Ledger。

## 明确不做

- 自动根据余额、日期、到期或后台任务关闭；
- 把关闭解释为法律终止、履约证明、会计结账或跨系统确认；
- 坏账、预计可收回余额、会计减值或税务核销；
- 重新打开、撤销、删除或直接修改 Closure；
- closed 后继续调额、减免、Allocation Apply／Reversal；
- Evidence、审批、多人复核、Billing Document、退款、账龄、`as_of`、导入导出；
- Contract Snapshot Ref 或 Contract 连接器。

## 验收标准

1. administrator 可以关闭余额由全额 Allocation、全额 Remission 或二者组合归零的 open Obligation；状态变为 closed，revision 与全部金额事实不变；
2. outstanding 非零、draft、closed、非法或缺失 Obligation 均不能关闭；余额归零本身不会自动关闭；
3. Closure 冻结 current revision、balance sequence、effective、net allocated、remitted、currency、派生 basis、原因、可信 Actor 与时间，客户端伪造服务端字段无效；
4. Expected Current Revision 或 Expected Balance Sequence 过期时返回冲突，即使最新余额再次归零也不能使用旧命令关闭；
5. 相同 Command ID／负载串行或并发重放只形成一条 Closure、Command 和审计；改变负载或 Actor 冲突，不同命令竞争恰好一个成功；
6. Closure 与 Allocation Apply／Reverse、Remission、Amount Adjustment 并发时形成确定顺序，最终不能出现 closed 且 outstanding 非零；
7. closed 后 Amount Adjustment、Remission、Allocation Apply、Allocation Reversal、再次关闭和直接 SQL 状态回退均被拒绝；历史读取继续可用；
8. Closure、Command、状态和审计原子提交；审计失败不留下任何部分事实；Closure、Command 和审计不可更新或删除；
9. 数据库直接写入不能伪造 revision、sequence、金额、币种、basis、Actor 或时间，也不能只插 Closure 而不推进状态；
10. HTTP 覆盖 administrator、可信 Actor、CSRF、成功、重放、冲突、404、422、500、closed 只读页面和关联 Allocation 不再可反向；
11. migration 从 `000012` 升至 `000013` 并可重放；既有 draft/open Obligation、revision、Adjustment、Remission、Allocation／Reversal 与余额保持不变；
12. 隔离空库真实 HTTP 流程完成确认、调额、Allocation／Remission 组合归零、显式关闭、closed 后全部余额写入拒绝和数据库投影核对；
13. Contract Ledger 停止时上述 Money Flow 流程和 ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
14. `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`、关键并发重复压力，以及两个应用 `GOWORK=off` 独立测试／构建通过。
