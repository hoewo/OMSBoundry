# Money Flow Ledger：人工实际收款／付款登记

里程碑：M3  
功能编号：MF-M3-005  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够相对于明确选择的本方账簿主体，人工登记一笔已经发生的收款或付款，并从列表和详情页核对资金方向、付款方／收款方快照、精确金额、发生日、资金账户标签和来源依据。本功能只建立独立、不可变的实际资金事实，不选择义务、不形成 Allocation，也不声称人工记录已经过银行核验。

## 业务事实

- Cash Transaction 表示已经发生的资金收付，不是应收应付义务、票据、付款申请、审批结果、Allocation 或会计凭证；
- `receipt`／`payment` 必须相对于用户明确选择的本方账簿主体解释，不能只保存没有参照方的方向；
- `receipt`：往来主体是 payer，本方账簿主体是 payee；
- `payment`：本方账簿主体是 payer，往来主体是 payee；
- 本方账簿主体与往来主体必须是 Money Flow 当前存在的两个不同 Party；本轮不建设共享主体中心或资金账户主数据；
- 登记时冻结 payer 和 payee 的 Party ID、Party revision、类型、法定名称／姓名和可选登记标识；之后 Party 当前资料变化不得改写资金事实；
- 描述必填，去除首尾空白后长度为 1 至 200 个字符；描述不是业务唯一键；
- 金额必填且严格大于 0，只接受普通非负十进制文本，整数最多 18 位、小数最多 6 位，持久化为 PostgreSQL `numeric(24,6)`；不得经过二进制浮点数；
- 币种去除空白并转为大写，必须是三个英文字母；本轮只校验稳定格式，不内置可能过期的完整货币字典；
- 资金发生日必填，使用严格 `YYYY-MM-DD` LocalDate；允许补录历史收付款，不擅自拒绝过去日期；
- 资金账户标签必填，去除首尾空白后长度为 1 至 100 个字符，用来说明本方资金归属，例如“基本户尾号 1234”或“现金账户”；它不是共享 Account 实体，不保存密码或访问凭据；
- 来源类型固定为 `manual`；人工来源依据必填，去除首尾空白后长度为 1 至 200 个字符，明确说明人工判断所依据的回单、流水截图或其他材料；
- 外部流水号可选，去除首尾空白后长度最多 100 个字符；非空值在同一“本方账簿主体 + 不区分大小写的资金账户标签 + 来源类型”范围内不区分大小写唯一；
- 新记录立即形成不可变 `recorded` 资金事实；本轮没有现金草稿、修改、删除、退款或冲销入口；
- Cash Transaction ID、Command ID 和 Audit ID 是不同 UUID；描述、金额、日期或 Party 组合不作为资金身份或模糊去重键。

## 用户流程

```text
管理员登录
  → 打开实际收付款列表
  → 选择 receipt 或 payment
  → 选择本方账簿主体和不同的往来主体
  → 填写描述、精确金额、币种、发生日、账户标签和人工来源依据
  → 可选填写外部流水号
  → 服务端从会话构造可信 Actor，并标准化、验证业务意图
  → PostgreSQL 读取并冻结两个 Party 当前 revision 快照
  → Cash Transaction、创建命令和审计在同一事务提交
  → 进入详情页核对 payer、payee 与全部资金事实
```

## 幂等、重复流水与并发

- 每次创建意图携带 UUID Command ID；
- 幂等负载由标准化后的 type、本方 Party ID、往来 Party ID、描述、金额、币种、发生日、账户标签、人工来源依据、外部流水号和可信 Actor 身份计算 SHA-256；
- 账户标签和外部流水号在幂等与重复比较时使用不区分大小写的规范键，页面仍显示首次成功记录的清理后文本；
- 相同 Command ID 与相同标准化负载重放返回原 Cash Transaction 及其原始 Party 快照，不重复读取今天的 Party 资料；
- 相同 Command ID 改变任一业务字段或 Actor 时返回显式幂等冲突；
- 非空外部流水号在其唯一范围内先取得 advisory transaction lock，再检查已存在记录；不同 Command ID 命中同一规范流水时返回显式重复流水冲突，不把它静默当作重放；
- PostgreSQL 部分唯一索引同时保护外部流水重复边界；
- PostgreSQL 事务按 Command ID 获取 advisory transaction lock，并在首次创建时以固定 Party ID 顺序读取双方 Party，避免相同命令并发形成重复结果；
- 外部流水号为空时只按 Command ID 保证幂等，不根据描述、金额、日期或主体相似度猜测重复资金。

## 权限、审计与数据边界

- GET／POST `/cash-transactions` 和 GET `/cash-transactions/{id}` 均要求 Money Flow 自己的有效 administrator 会话；
- POST 必须通过 CSRF；请求中的 Actor 字段全部忽略，Actor 只从当前 Principal 构造；
- `cash_transaction_recorded` 审计保存 Command ID、可信 Actor、type、双方 Party ID、金额、币种、发生日、账户标签、来源类型和外部流水号；
- Cash Transaction、不可变创建命令和审计必须原子提交；Party 不存在、重复流水或审计失败时不能留下部分事实；
- Cash Transaction、创建命令和审计禁止 UPDATE／DELETE；错误登记以后只能通过独立 Cash Reversal 事实处理，不能原地覆盖；
- Money Flow 只读取并写入自己的数据库，不访问 Contract Ledger 数据库、Cookie、用户、Party、HTTP 接口或文件。

## 验收标准

1. administrator 可以分别登记 receipt 和 payment，列表和详情正确显示方向、金额、发生日、账户标签、来源和 `recorded` 状态。
2. receipt 的 payee 必须是选择的本方主体，payment 的 payer 必须是选择的本方主体；双方不能相同。
3. 登记时冻结双方 Party revision、类型、名称和登记标识；之后修改 Party 当前名称不改变既有资金详情。
4. 非法 type、Party ID、同一 Party、空或过长描述、零／负／超精度金额、非法币种、非法日期、空或过长账户标签、空或过长来源依据和过长外部流水号均明确拒绝且不写数据库。
5. 金额标准化不经过浮点数；`000100.500000` 形成 `100.5`，最小 `0.000001` 可用。
6. 相同 Command ID 和相同标准化负载串行或并发重放只形成一个 Cash Transaction、一个 Command 和一条审计；改变负载或 Actor 返回冲突。
7. 不同 Command ID 使用大小写等价的同范围非空外部流水号时，串行或并发都只有一个成功；不同本方主体或不同账户标签可以使用相同外部流水号。
8. Party 不存在、重复流水或故意令审计失败时，Cash Transaction、Command 和审计全部为零；资金、创建命令和审计不可变。
9. HTTP POST 要求登录、administrator 和有效 CSRF；客户端 Actor 字段不能覆盖真实用户。
10. 列表和详情来自 PostgreSQL；数据库错误不伪装为空列表、404 或创建成功。
11. migration 从 `000005` 升级到 `000006` 并可重复执行，表、索引、约束、外键和不可变触发器存在。
12. 单元、竞态、真实 PostgreSQL、真实 HTTP 和独立启动验证通过；Contract Ledger 停止时仍能完整创建、重放、拒绝重复流水并查看资金记录。

## 明确不做

- Cash Transaction 草稿、编辑、删除、退款、冲销或反向记录；
- `internal_transfer`；内部账户转账需要明确两端账户和统计排除规则后独立实现；
- 银行接口、流水文件导入、自动对账或“银行已核验”状态；
- 资金账户目录、账户余额、支付指令、Payment Intent 或付款审批；
- 选择 Obligation、Allocation、部分核销、未分配余额计算或义务余额变化；
- 票据、发票、会计凭证、收入确认、税务处理、汇率或跨币种核销；
- 文件 Evidence；本轮仅保存人工来源依据文本；
- Contract Snapshot Ref、Contract → Money Flow 连接器、跨库外键或共享 Party ID。
