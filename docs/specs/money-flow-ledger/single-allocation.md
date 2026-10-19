# Money Flow Ledger：单笔资金义务分配与余额不变量

里程碑：M3  
功能编号：MF-M3-006  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够把一笔已经登记的实际资金，部分或全部分配给一条已经确认的 `open` 义务，并从分配列表、详情及两端事实页面核对已分配金额、资金未分配金额和义务剩余金额。本功能只建立一条 Cash Transaction 与一条 Obligation 之间的不可变 Allocation；每个命令只处理一对事实，不提供批量协议、反向核销或跨系统连接。

## 业务事实

- Allocation 表示“这部分实际资金核销了这条义务”的独立关系，不是新的资金、义务调整、票据匹配或会计凭证；
- Allocation 必须引用一条 `recorded` Cash Transaction 和一条 `open` Obligation 的当前 revision；命令必须显式携带 Expected Obligation Revision，过期时不能静默套用到新事实；
- 本轮只接受直接两方收付，Cash Transaction 和 Obligation 的本方账簿主体、payer Party ID 与 payee Party ID 必须分别一致：
  - `receivable` 只能使用 `receipt`，共同的 debtor 是 payer、creditor 是 payee；
  - `payable` 只能使用 `payment`，共同的 debtor 是 payer、creditor 是 payee；
- 只比较不可变事实中的 Party ID，不用 Party 当前名称重新判断；创建后的 Party 资料变化不得改变既有 Allocation；
- Cash Transaction 与 Obligation 必须同币种；本轮没有汇率、折算金额或汇兑损益；
- 分配金额必填且严格大于 0，只接受普通非负十进制文本，整数最多 18 位、小数最多 6 位，持久化为 PostgreSQL `numeric(24,6)`；不得经过二进制浮点数；
- 分配金额不得超过资金当前未分配金额，也不得超过义务当前剩余金额；等于任一侧剩余额时允许；
- 人工分配依据必填，去除首尾空白后长度为 1 至 200 个字符；来源类型固定为 `manual`，它只说明人工匹配依据，不伪装为银行或外部系统已核验；
- `applied_at` 由服务端可信时钟生成，客户端不能指定或覆盖；
- 新 Allocation 立即形成不可变 `active` 事实。本轮没有草稿、编辑、删除、反向核销或状态更新；未来错误修正只能追加引用原 Allocation 的 reversal 事实；
- Allocation ID、Command ID 和 Audit ID 是不同 UUID；同一资金与义务允许以后形成多条独立 Allocation，因此不设置二者组合唯一约束。

## 唯一余额模型

当前尚无义务调整、资金冲销和 Allocation reversal，因此本轮唯一计算口径是：

```text
cash allocated amount
  = 该 Cash Transaction 的全部 Allocation 金额之和

cash unallocated amount
  = Cash Transaction amount - cash allocated amount

obligation allocated amount
  = 该 Obligation 的全部 Allocation 金额之和

obligation outstanding amount
  = 当前 Obligation revision original amount - obligation allocated amount
```

- 三个派生金额都从不可变事实查询计算，不在 Cash Transaction 或 Obligation 根表维护可写汇总字段；
- 本轮不允许负余额，也不把负数截断为 0；会导致任一侧小于 0 的命令必须整体拒绝；
- 义务余额为 0 是“已足额分配”的只读投影，本轮不自动把 Obligation 状态改为 `closed`；义务关闭、调整和重新打开属于后续独立事实；
- 未匹配资金始终保留为 Cash Transaction 的未分配金额，不因列表筛选或分配其他资金而丢失。

## 用户流程

```text
管理员登录
  → 打开资金分配列表
  → 选择一笔 recorded 资金和一条 open 义务
  → 页面显示两端当前金额与剩余额
  → 填写分配金额和人工分配依据
  → 服务端从会话构造可信 Actor
  → PostgreSQL 先解决 Command 重放，再依次锁定资金和义务
  → 校验 Expected Obligation Revision、方向、双方 Party、币种及两侧余额
  → Allocation、创建命令和审计在同一事务提交
  → 进入详情页，并可从资金和义务页面核对新的余额投影
```

## 幂等、并发与数据库保护

- 每次创建意图携带 UUID Command ID；
- 幂等负载由标准化后的 Cash Transaction ID、Obligation ID、Expected Obligation Revision、金额、来源类型、人工分配依据和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同标准化负载重放返回原 Allocation，不重新消费今天的余额，也不重复写审计；
- 相同 Command ID 改变任一业务字段或 Actor 时返回显式幂等冲突；
- 首次执行固定按 Cash Transaction、Obligation 的顺序取得行锁；同一资金或同一义务上的并发命令必须串行检查聚合余额；
- PostgreSQL 插入校验触发器再次保护状态、current revision、方向、双方 Party、币种和两侧不超额，即使绕过应用仓储也不能写入不一致 Allocation；
- Allocation、创建命令和审计必须原子提交；任一校验或审计失败时三者全部回滚；
- Allocation、创建命令和审计禁止 UPDATE／DELETE。

## 权限与系统边界

- GET／POST `/allocations` 和 GET `/allocations/{id}` 均要求 Money Flow 自己的有效 administrator 会话；
- POST 必须通过 CSRF；请求中的 Actor、`applied_at`、币种、双方主体或余额字段全部不可信，Actor 只从当前 Principal 构造，其余事实从数据库读取；
- `allocation_applied` 审计保存 Command ID、可信 Actor、Cash Transaction ID、Obligation ID、Obligation revision、金额、币种和人工分配依据；
- Money Flow 只读取并写入自己的数据库，不访问 Contract Ledger 数据库、Cookie、用户、Party、HTTP 接口或文件；当前没有跨系统业务流动，因此本功能只做独立运行验证，不伪造连接器联调。

## 验收标准

1. administrator 可以对兼容的一笔 receipt／receivable 或 payment／payable 创建部分或全额 Allocation，列表和详情显示 `active`、两端引用、金额、币种、依据、Actor 和时间。
2. Cash Transaction 与 Obligation 的列表和详情显示从 Allocation 求和得到的 allocated、unallocated／outstanding；根事实金额不被修改。
3. draft、非 open 或 Expected Obligation Revision 过期的义务明确拒绝；Cash Transaction 或 Obligation 不存在时不留下部分事实。
4. 方向、本方主体、payer、payee 或币种任一不一致都明确拒绝；第三方代收付与跨币种不能静默通过。
5. 零、负数、非法文本、超过 18 位整数或 6 位小数的金额，以及空或过长人工依据均明确拒绝。
6. 分配金额等于资金未分配额或义务剩余额时成功；超过任一侧时冲突，余额绝不小于 0。
7. 相同 Command ID 和相同标准化负载串行或并发重放只形成一个 Allocation、一个 Command 和一条审计；改变负载或 Actor 返回冲突。
8. 不同 Command ID 并发争用同一资金或同一义务时，成功金额总和不超过任一侧；失败命令不留下 Command 或审计。
9. 故意令审计失败时整体回滚；Allocation、创建命令和审计不可变；直接 SQL 也不能绕过兼容与超额保护。
10. HTTP POST 要求登录、administrator 和有效 CSRF；客户端伪造 Actor、时间、币种和余额字段无效；数据库错误不伪装为成功、空列表或 404。
11. migration 从 `000006` 升级到 `000007` 并可重复执行，既有 Party、Obligation 和 Cash Transaction 保持不变，表、索引、约束、外键、校验函数和不可变触发器存在。
12. 单元、竞态、真实 PostgreSQL、真实 HTTP 和独立启动验证通过；Contract Ledger 停止时仍能完整创建、重放、拒绝超额并查看 Allocation 与两侧余额。

## 明确不做

- 一次请求同时分配多笔资金或多条义务；本轮每个命令只形成一个 Allocation；
- Allocation reversal、编辑、删除、作废或替换；
- Obligation adjustment、减免、坏账、争议、自动关闭或余额人工覆盖；
- Cash reversal、退款、内部账户转账、账户余额或支付指令；
- 第三方代收代付、预收、预付、超额处理或往来主体不一致的分配；
- 跨币种核销、汇率、折算金额或汇兑损益；
- 票据匹配、会计凭证、收入确认、税务处理或文件 Evidence；
- Contract Snapshot Ref、Contract → Money Flow 连接器、跨库外键或共享 Party ID。
