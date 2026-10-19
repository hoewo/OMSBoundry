# Money Flow Ledger：Obligation 减免

里程碑：M4  
功能编号：MF-M4-004  
日期：2026-09-19  
状态：已实现并通过纵向、并发、迁移与独立运行验证

## 目标

已登录的 Money Flow 本地管理员能够在一条 `open` Obligation 上记录一笔已经生效的部分或全部减免。每笔减免形成独立、不可变的 Remission，减少该义务后续可核销余额，但不覆盖原始金额、不改写 Obligation revision，也不迁移既有 Allocation。

本功能只表达“债权人已经有效免除一部分应收或应付款义务，系统现在记录这一处置事实”。它不是义务依据金额变化、坏账、争议中金额冻结、付款折扣建议、退款、现金冲销、核销反向或义务关闭。管理员记录事实不等于系统证明其法律效力或完成会计处理。

## 核心建模决策

- Remission 是 Obligation 上独立的处置事实，不是 `amount_adjustment`，不生成新的 Obligation revision；
- current effective amount 继续只表示经过依据调整后的义务金额；remitted amount 表示已经生效的减免累计；两者不互相覆盖；
- 每笔 Remission 使用始终为正的精确金额，并引用记录时的 current Obligation revision 和币种；同一 revision 可以形成多笔 Remission；
- Remission 金额不得超过记录前的 outstanding amount；全部减免允许令 outstanding amount 归零，但 Obligation 仍保持 `open`，需要再由独立受控关闭事实显式处理；
- Allocation 仍保存实际资金与义务的核销关系。新增 Allocation 的上限改为 effective amount 减净 Allocation 再减 Remission；既有 Allocation 不修改；
- 后续金额减少 Adjustment 的结果不得低于“净 Allocation + Remission”；金额增加仍可扩大可处理余额；
- Remission、Amount Adjustment、Allocation Apply 与 Allocation Reverse 都以 Obligation 行锁形成确定顺序。Remission 不取得 Cash Transaction 锁，避免形成 Obligation → Cash 的反向锁序；
- 第一版不提供 Remission Reversal。录错不能更新或删除原事实，未来如需要纠错必须另立反向事实能力。

详细依据见 [ADR-0020](../../adr/0020-obligation-remission-is-a-separate-disposition-fact.md)。

## 业务事实

- Remission ID、Command ID、Audit ID 相互独立；
- 用户提交 Obligation ID、Expected Current Revision、正减免金额、必填原因和必填人工来源依据；
- 币种、Actor、记录时间和记录时 effective amount 由服务端与数据库从可信事实读取或生成，客户端不能覆盖；
- 原因和人工来源依据分别去除首尾空白，长度均为 1 至 200 个字符；来源类型固定为 `manual`；
- Remission 冻结 Obligation revision、减免金额、当时 effective amount、币种、原因、人工来源依据、可信记录 Actor 和记录时间；
- Remission、Command 和审计必须在一个 PostgreSQL 事务内提交；它们一经形成均不可更新或删除；
- 同一 Obligation 可以按真实发生次数追加多笔 Remission，不用可写汇总字段保存累计值。

## 当前投影

```text
obligation original amount
  = Obligation 最初确认的 revision 1 effective amount

obligation current effective amount
  = effective amount of Obligation current revision

obligation net allocated amount
  = SUM(Allocation.amount) - SUM(AllocationReversal.amount)

obligation remitted amount
  = SUM(ObligationRemission.amount)

obligation outstanding amount
  = current effective amount
  - net allocated amount
  - remitted amount
```

- 当前列表和详情显示 original、current effective、allocated、remitted 与 outstanding；
- Remission 历史展示记录时 revision、金额、原因、人工来源、可信 Actor 和时间；
- revision 历史与 Adjustment 历史不因 Remission 改写；
- Allocation 历史继续展示其形成时引用的 Obligation revision；
- 第一版没有 Remission Reversal，因此 remitted amount 只随新事实增加；未来增加反向能力时再改为净额投影。

## 用户流程

```text
管理员登录
  → 打开 open Obligation 详情
  → 查看 effective／allocated／remitted／outstanding 和历史
  → 输入正减免金额、原因和人工来源依据
  → 提交 Command ID + Expected Current Revision
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放
  → 锁定 Obligation，重新读取 current revision、effective amount、净 Allocation 和累计 Remission
  → 校验本次金额不超过最新 outstanding
  → 写入 Remission、Command 和审计
  → 返回详情，展示新的 remitted／outstanding 和完整历史
```

## 幂等、并发与数据库保护

- 幂等负载包含格式版本、Obligation ID、Expected Current Revision、标准化金额、标准化原因、标准化人工来源依据和可信 Actor；
- 相同 Command ID 与相同负载串行或并发重放返回原 Remission，不重复扣减余额；改变任一业务字段或 Actor 返回显式幂等冲突；
- 不同 Command ID 表示不同减免事实，可以在同一 revision 连续追加；事务必须在 Obligation 锁内以最新 outstanding 逐笔校验，不能因并发累计超额；
- Remission 与 Allocation Apply 并发时，先提交者占用余额，后提交者重新读取最新累计并在超额时得到 typed conflict；
- Remission 与 Amount Adjustment 并发时，若 Remission 先提交，减少 Adjustment 必须以“净 Allocation + Remission”为下限；若 Adjustment 先提交，Remission 使用新的 effective amount；
- Remission 与 Allocation Reverse 并发时两者都可以成功，Reverse 释放的 Allocation 容量体现在最终 outstanding；
- PostgreSQL Remission 触发器再次校验 `open`、current revision、币种、记录时 effective amount、正金额和最新余额；
- PostgreSQL Allocation 触发器把 Remission 纳入 outstanding 上限；Adjustment 触发器把 Remission 纳入结果金额下限；直接 SQL 不能绕过组合不变量。

## 权限、审计与错误边界

- GET `/obligations/{id}` 继续显示详情；POST `/obligations/{id}/remissions` 只允许 Money Flow 本地 administrator；
- POST 必须通过 CSRF，且请求体限制在 16 KiB；Actor 只来自登录 Principal；
- `obligation_remission_recorded` 审计保存 Command ID、Obligation revision、金额、记录时 effective amount、币种、原因、人工来源依据和可信 Actor；
- Obligation ID 非法或不存在返回 404；输入验证错误返回 422；非 open、revision 过期、金额超过 outstanding 或幂等冲突返回 409；数据库失败返回 500；
- 失败时重新读取最新详情并生成新的 Command ID，不能把一个已经改变负载的命令继续复用；
- 本功能只访问 Money Flow 数据库，不访问 Contract Ledger。

## 明确不做

- 修改 original amount、current effective amount、Obligation revision 或非金额条款；
- 把普通依据金额减少、价格更正、范围变化或技术纠错记录成 Remission；
- 坏账、资产减值、争议、折扣建议、审批中减免、未来生效的减免或会计凭证；
- 自动创建、反向、迁移或重新分配 Allocation；
- 自动关闭、结清证明或把 outstanding 为零直接解释为 `closed`；
- Remission Reversal、删除、撤回、负数减免或结果为负；
- Evidence、多人复核、审批、Billing Document、账龄、`as_of`、统计、导入导出；
- Contract Snapshot Ref 或 Contract 连接器。

## 验收标准

1. administrator 可以对 `open` Obligation 追加部分或全部 Remission；不生成 revision，original／effective amount 和全部旧事实不变；
2. 当前列表和详情展示 remitted amount，并按 effective − net allocated − remitted 精确计算 outstanding；
3. 金额、原因和人工来源依据由用户明确提交；effective amount、币种、Actor 和时间由服务端与数据库可信取得，客户端伪造无效；
4. 本次 Remission 可以等于、不能超过最新 outstanding；等于时 outstanding 为 0 但状态仍为 `open`；
5. 相同 Command ID／负载串行或并发重放只形成一条 Remission、Command 和审计；改变负载或 Actor 返回冲突；
6. 不同 Command 可以在同一 revision 连续追加，但并发累计不能令 allocated + remitted 超过 effective amount；
7. 新 Allocation 必须同时遵守资金余额和扣除 Remission 后的义务 outstanding；既有 Allocation 与历史 revision 不被改写；
8. 减少 Amount Adjustment 的结果不得低于净 Allocation + Remission；增加 Adjustment 后余额正确扩大；
9. Remission 与 Allocation Apply／Reverse、Amount Adjustment 并发时形成可解释顺序，最终 effective amount 始终不低于 allocated + remitted；
10. 直接 SQL 不能伪造 revision、币种、effective amount 或绕过组合余额；Remission、Command 和审计不可更新或删除；
11. 缺失义务、非 open、过期 revision、空／过长原因或来源、非法金额、超额、CSRF、非 administrator、审计失败和数据库失败均不留下半成品；
12. HTTP 覆盖登录、administrator、可信 Actor、成功、重放、冲突、404、422 和 500；
13. migration 从 `000011` 升至 `000012` 并可重放；既有 Obligation revision、Adjustment、Allocation／Reversal、Cash Reversal 和余额不变，历史义务的 remitted amount 为 0；
14. 隔离空库真实 HTTP 流程完成确认、金额调整、既有 Allocation、同 revision 连续减免、超额拒绝、减免后继续 Allocation、组合归零、归零后继续减免拒绝和全部历史投影核对；
15. Contract Ledger 停止时上述 Money Flow 流程和 ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
16. `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
