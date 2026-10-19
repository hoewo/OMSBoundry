# Money Flow Ledger：义务确认与 revision 1

里程碑：M3  
功能编号：MF-M3-004  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够在义务详情页确认一条仍处于 `draft` 的人工应收或应付义务。确认必须显式消费当前 `draft_version`，把草稿中已经冻结的全部义务事实复制为不可变 Obligation revision 1，并将义务状态原子推进为 `open`。本轮只建立正式义务事实，不引入草稿编辑、调整、实际资金、余额或核销。

## 业务事实

- `draft` 是尚未确认的录入事实；`open` 表示已经形成正式应收或应付义务，但不代表已有实际收付款或核销；
- 确认只允许从 `draft` 到 `open`，不能重复推进已经 `open`、`closed` 或 `cancelled` 的义务；
- 确认命令必须携带 `Expected Draft Version`，且必须等于当前草稿的 `draft_version`；本轮只有 version 1，但接口和并发边界不得依赖“永远只有 1”；
- revision 1 冻结 direction、title、ledger owner、creditor／debtor Party 快照、原始金额、币种、到期日、来源类型和人工来源依据；
- revision 1 必须直接复制草稿内已经冻结的 Party 快照，不在确认时重新读取 Party 当前资料；Party 当前名称或 revision 的变化不得改写历史草稿或正式义务；
- revision 号从 1 开始，确认成功后义务 `current_revision = 1`；revision 是不可变事实，后续调整如有真实需求只能追加新 revision；
- `open` 义务的当前展示投影来自 `current_revision` 对应的 revision，而不是可变地重解释草稿列；
- 确认 Actor 和确认时间是 revision 1 的事实；客户端不能提供或覆盖 Actor；
- 本轮不计算已核销金额、剩余余额、账龄或现金预测。

## 用户流程

```text
管理员登录
  → 打开 draft 义务详情
  → 核对方向、双方快照、金额、币种、到期日和来源
  → 提交 Command ID + Expected Draft Version
  → 服务端从当前会话构造可信 Actor
  → PostgreSQL 锁定确认命令和目标义务
  → 复制草稿事实形成不可变 revision 1
  → 写入确认命令和审计，并把状态推进为 open
  → 返回详情页展示 open、current revision 1 和确认记录
```

## 幂等与并发

- 每次确认意图携带独立 UUID Command ID；
- 幂等负载由格式版本、Obligation ID、Expected Draft Version 和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同负载重放返回原义务和 revision 1，不重复确认、不重复写审计；
- 相同 Command ID 改变义务、Expected Draft Version 或 Actor 时返回显式幂等冲突；
- 事务先按 Command ID 获取 advisory transaction lock并检查历史命令，再对目标 Obligation 执行 `FOR UPDATE`；
- 两个不同 Command ID 并发确认同一草稿时，只允许一个成功；另一个在取得行锁后看到非 `draft` 状态并返回状态冲突；
- Expected Draft Version 不匹配时返回版本冲突，不形成 revision、命令、状态变更或审计；
- replay 检查先于当前状态检查，因此原成功命令可在义务已经 `open` 后安全重放。

## 权限、审计与数据边界

- POST `/obligations/{id}/confirm` 要求 Money Flow 自己的有效 administrator 会话和有效 CSRF；
- 确认表单中的 Obligation ID 只取路由参数，Actor 只从当前 Principal 构造；
- `obligation_confirmed` 审计记录 Expected Draft Version、resulting revision、状态变化和可信 Actor；
- revision 1、确认命令、`open` 状态、`current_revision = 1` 与审计必须在同一事务提交；任一写入失败必须整体回滚；
- revision、确认命令和审计禁止 UPDATE／DELETE；
- Money Flow 只访问自己的数据库；确认不查询 Contract Ledger，不创建 Contract Snapshot Ref，也不跨库引用合同或身份。

## 验收标准

1. administrator 可以确认 draft 义务，详情和列表显示 `open`、current revision 1 及确认 Actor／时间。
2. revision 1 完整复制草稿方向、标题、双方 Party 快照、金额、币种、到期日和人工来源；确认前修改 Party 当前资料不影响复制结果。
3. 确认缺失义务返回 404；非 draft 返回状态冲突；Expected Draft Version 不匹配返回版本冲突；所有失败均不留下部分事实。
4. 相同 Command ID 与相同负载串行或并发重放只形成一条 revision、一条确认命令和一条审计；改变负载或 Actor 返回幂等冲突。
5. 两个不同 Command ID 并发确认同一草稿时恰好一个成功，另一个返回状态冲突。
6. HTTP POST 要求登录、administrator 和有效 CSRF；客户端 Actor 字段不能覆盖真实用户。
7. 确认成功后表单不再显示；刷新成功页面和重放原命令都不会重复确认。
8. 义务正式投影来自 current revision；revision、确认命令和审计受数据库不可变保护。
9. migration 从 `000004` 升级到 `000005` 并可重复执行，旧有 draft 数据保持可确认。
10. 单元、竞态、真实 PostgreSQL、真实 HTTP 和独立启动验证通过；Contract Ledger 停止时仍能完整确认并查看义务。

## 明确不做

- 草稿编辑或取消；
- revision 2、义务调整、减免、坏账、争议、关闭或余额计算；
- 审批流、多人复核或通用状态机；
- 发票／票据、实际收付款、资金账户、Allocation 或核销；
- Contract Snapshot Ref、Contract → Money Flow 连接器或跨系统自动确认；
- 会计凭证、收入确认、汇率和多币种核销。
