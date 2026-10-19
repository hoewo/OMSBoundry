# Money Flow Ledger：Obligation 金额调整

里程碑：M4  
功能编号：MF-M4-003  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够在一条 `open` Obligation 的当前 revision 上追加一次明确的金额增加或减少。每次调整形成一条独立、不可变的 Adjustment，并连续形成 Obligation revision N+1；原确认金额、旧 revision、既有 Allocation／Reversal 和审计链永久保留。

本功能只表达“义务依据变化后，当前应收或应付金额随之增加或减少”。它不是减免、坏账、争议、退款、资金冲销、核销反向或义务关闭；这些业务事实必须分别建模。

## 核心建模决策

- Adjustment 是 Obligation 内部的版本推进事实；每次成功调整都必须从当前 revision N 连续形成 revision N+1，不能原地修改 revision；
- 增加和减少使用同一种 Adjustment 事实，通过 `increase`／`decrease` 类型区分；调整金额始终是正的精确十进制数，不使用有符号金额表达方向；
- 新 revision 完整复制上一 revision 的方向、标题、本方主体、债权方／债务方快照、币种、到期日和来源依据，只改变 effective amount；
- revision 1 的 effective amount 等于最初确认金额；当前有效金额只从 `current_revision` 对应 revision 读取；Obligation 根记录中的 original amount 继续保留最初录入金额，不成为当前金额的第二真相源；
- 减少后的 effective amount 必须大于 0，并且不得低于该 Obligation 的当前 Allocation 净额；增加后的金额必须仍可由 `numeric(24,6)` 精确保存；
- Adjustment 不自动创建、反向或改写任何 Allocation。既有 Allocation 永久保留其形成时引用的 Obligation revision；后续 Allocation 只能引用新的 current revision；
- effective amount 等于 Allocation 净额时，outstanding amount 为 0，但 Obligation 仍保持 `open`；本功能不把余额归零解释为关闭；
- Adjustment、Allocation Apply 与 Allocation Reverse 都以 Obligation 行锁形成确定顺序。Adjustment 不取得 Cash Transaction 锁，避免与既定 Cash → Obligation 锁序形成反向依赖。

详细依据见 [ADR-0019](../../adr/0019-obligation-amount-adjustment-appends-revision.md)。

## 业务事实

- Adjustment ID、Command ID、Audit ID 相互独立；
- 用户提交 Obligation ID、Expected Current Revision、调整类型、正调整金额和必填原因；
- 原金额、结果金额、币种、Actor 和发生时间由服务端和数据库从可信事实计算或生成，客户端不能覆盖；
- 原因去除首尾空白后长度为 1 至 200 个字符；来源类型固定为 `manual`；
- 每个 source revision 最多形成一个后继 Adjustment，resulting revision 必须严格等于 source revision + 1；同一 Obligation 可以连续形成多次 Adjustment；
- Adjustment 冻结 previous effective amount、adjustment amount、resulting effective amount、币种、可信 Actor 和调整时间；
- Adjustment、新 revision、Command、根记录 current revision 推进和审计必须在一个 PostgreSQL 事务内提交；
- Adjustment、revision、Command 和审计一经形成均不可更新或删除。

## 当前投影

```text
obligation original amount
  = Obligation 草稿最初确认的金额

obligation current effective amount
  = effective amount of Obligation current revision

obligation net allocated amount
  = SUM(Allocation.amount) - SUM(AllocationReversal.amount)

obligation outstanding amount
  = current effective amount - net allocated amount
```

- 当前列表和详情同时显示 original amount 与 current effective amount；尚未调整时两者相等；
- revision 历史按 revision 正序展示类型、来源 revision、effective amount、可信 Actor 和形成时间；
- Adjustment 历史展示增加／减少、前值、调整额、结果值、原因、Actor 和时间；
- Allocation 历史仍展示每条 Allocation 当时引用的 Obligation revision，不因后续调整被重写；
- 本轮不提供 `as_of` 查询，但连续 revision、Adjustment 时间与 Allocation／Reversal 时间为后续历史重建保留完整事实。

## 用户流程

```text
管理员登录
  → 打开 open Obligation 详情
  → 查看 original／current effective／allocated／outstanding
  → 选择增加或减少，填写正调整金额和原因
  → 提交 Command ID + Expected Current Revision
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放
  → 锁定 Obligation 并重新读取 current revision 与 Allocation 净额
  → 校验结果金额大于 0 且不低于净分配额
  → 写入 Adjustment、revision N+1、Command、current revision 和审计
  → 返回详情，展示新的当前金额、余额和完整历史
```

## 幂等、并发与数据库保护

- 幂等负载包含格式版本、Obligation ID、Expected Current Revision、调整类型、标准化调整金额、标准化原因和可信 Actor；
- 相同 Command ID 与相同负载串行或并发重放返回原 Adjustment 和 resulting revision，不重复推进；
- 相同 Command ID 改变任一业务字段或 Actor 返回显式幂等冲突；
- 不同 Command ID 并发调整同一 source revision 时，恰好一个形成 revision N+1，另一个得到 revision conflict；
- Adjustment 先锁 Obligation，再读取 source revision 和当前 Allocation 净额；
- Allocation Apply 固定先锁 Cash Transaction、再锁 Obligation。若 Adjustment 先提交，旧 revision 的 Apply 被拒绝；若 Apply 先提交，减少调整必须在新的净额上重新校验；
- Allocation Reverse 也在 Cash Transaction 后锁 Obligation。若 Reverse 先提交，调整看到释放后的净额；若 Adjustment 先提交，Reverse 仍可纠正原 Allocation，余额随净额恢复；
- PostgreSQL Adjustment 触发器再次校验状态、revision 连续性、前值、币种、精确算术和净分配下限；revision 插入触发器校验非金额字段只能从 source revision 复制；
- PostgreSQL Obligation 推进触发器只允许已存在的确认或 Adjustment 驱动合法 current revision 变化；直接 SQL 不能跳 revision、伪造结果金额或把当前金额降到净分配以下。

## 权限、审计与错误边界

- GET `/obligations/{id}` 继续显示详情；POST `/obligations/{id}/adjust-amount` 只允许 Money Flow 本地 administrator；
- POST 必须通过 CSRF，且请求体限制在 16 KiB；Actor 只来自登录 Principal；
- `obligation_amount_adjusted` 审计保存 Command ID、source／resulting revision、类型、前值、调整额、结果值、币种、原因和可信 Actor；
- Obligation ID 非法或不存在返回 404；输入验证错误返回 422；非 open、revision 过期、减少后低于净分配或幂等冲突返回 409；数据库失败返回 500；
- 失败时重新读取最新详情并生成新的 Command ID，不能复用一个已改变负载的命令；
- 本功能只访问 Money Flow 数据库，不访问 Contract Ledger。

## 明确不做

- 修改标题、到期日、币种、方向、双方主体或来源依据；
- 把 Adjustment 当作减免、坏账、争议、折扣、退款、拒付、反向付款或会计凭证；
- 部分关闭、自动关闭、结清证明或余额为零时自动改变状态；
- 自动反向或重建 Allocation、自动匹配、预收预付和超额核销；
- 负数调整额、结果为零、删除或撤回 Adjustment、Adjustment Reversal；
- Billing Document、票据、账龄、`as_of`、预测、统计、导入导出；
- Evidence、审批、多人复核、Contract Snapshot Ref 或 Contract 连接器。

## 验收标准

1. administrator 可以对 `open` Obligation 追加增加或减少 Adjustment，并连续形成 revision N+1；原 revision 与原始金额不变；
2. 当前列表和详情从 current revision 展示 effective amount，并以它减 Allocation 净额得到 outstanding amount；
3. 增加／减少使用正调整金额，结果由服务端和数据库精确计算；客户端伪造前值、结果值、币种、Actor 或时间无效；
4. 减少后的金额必须大于 0 且可以等于、不能低于当前 Allocation 净额；等于净额时 outstanding 为 0 但状态仍为 `open`；
5. 新 revision 只改变 effective amount，方向、标题、主体快照、币种、到期日和来源依据与 source revision 完全一致；
6. 既有 Allocation 保留历史 revision；新 Allocation 必须引用新的 current revision，并使用调整后的 outstanding 上限；
7. 相同 Command ID／负载串行或并发重放只形成一条 Adjustment、一个新 revision、一条 Command 和一条审计；改变负载或 Actor 返回冲突；
8. 不同 Command 并发调整同一 source revision 时恰好一个成功；另一请求返回包含 expected／actual 的 revision conflict；
9. Adjustment 与 Allocation Apply／Reverse 并发时形成可解释的确定顺序，最终 effective amount 不低于 Allocation 净额；
10. 直接 SQL 不能跳过 current revision、伪造复制字段、错误算术或绕过净分配下限；Adjustment、revision、Command 和审计不可更新或删除；
11. 缺失义务、非 open、非法类型、空或过长原因、非法金额、结果为零、余额冲突、审计失败和数据库失败均不留下半成品；
12. HTTP 覆盖登录、administrator、CSRF、可信 Actor、成功、重放、冲突、404、422 和 500；
13. migration 从 `000010` 升至 `000011` 并可重放；既有 revision 1 金额、Actor、时间、Allocation／Reversal、Cash Reversal 和余额不变；
14. 隔离空库真实 HTTP 流程完成确认、Allocation、增加、减少、旧 revision Allocation 拒绝、新 revision Allocation 成功和全部投影核对；
15. Contract Ledger 停止时上述 Money Flow 流程和 ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
16. `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
