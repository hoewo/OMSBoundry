# Money Flow Ledger：人工应收／应付义务草稿

里程碑：M3  
功能编号：MF-M3-003  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够从本系统主体目录选择本方账簿主体和往来主体，人工建立一条应收或应付义务草稿，并从列表和详情页看到创建时冻结的债权方、债务方、金额、到期日和人工来源依据。本功能只证明义务草稿事实能够独立、可信地形成，不把草稿确认、实际资金和核销混入同一轮。

## 业务事实

- Obligation 表示一方应向另一方支付的经营义务，不是合同金额、票据、付款申请、实际资金或核销；
- `receivable`／`payable` 必须相对于用户明确选择的本方账簿主体解释，不能只保存一个没有参照方的方向字段；
- `receivable`：本方账簿主体是 creditor，往来主体是 debtor；
- `payable`：往来主体是 creditor，本方账簿主体是 debtor；
- 本方账簿主体与往来主体必须是 Money Flow 当前存在的两个不同 Party；本轮不建设全局组织设置或共享主体中心；
- 草稿在创建时冻结 creditor 和 debtor 的 Party ID、Party revision、类型、法定名称／姓名和可选登记标识；之后 Party 当前资料变化不得静默改写草稿快照；
- 标题必填，去除首尾空白后长度为 1 至 200 个字符；标题不是业务唯一键；
- 原始金额必填且严格大于 0，只接受普通非负十进制文本，整数最多 18 位、小数最多 6 位，持久化为 PostgreSQL `numeric(24,6)`；不得经过二进制浮点数；
- 币种去除空白并转为大写，必须是三个英文字母；本轮只校验稳定格式，不内置可能过期的完整货币字典；
- 到期日必填，使用严格 `YYYY-MM-DD` LocalDate；允许登记已经到期的历史义务，不擅自拒绝过去日期；
- 来源类型固定为 `manual`，人工来源依据必填，去除首尾空白后长度为 1 至 200 个字符；它是本次人工录入时冻结的来源说明，不伪装成已经核验的 Contract、订单或银行事实；
- 新建状态固定为 `draft`，`draft_version` 固定为 1；草稿不进入应收应付余额、账龄、现金预测或核销计算；
- Obligation ID、Command ID 和 Audit ID 是不同 UUID；名称、来源依据和 Party ID 组合都不作为 Obligation 身份或唯一键。

## 用户流程

```text
管理员登录
  → 打开义务草稿列表
  → 选择 receivable 或 payable
  → 选择本方账簿主体和不同的往来主体
  → 填写标题、精确金额、币种、到期日和人工来源依据
  → 服务端从会话构造 Actor，并标准化、验证业务意图
  → PostgreSQL 读取并冻结两个 Party 当前 revision 快照
  → Obligation draft、创建命令和审计在同一事务提交
  → 进入详情页核对 creditor、debtor 与全部草稿事实
```

## 幂等与并发

- 每次创建意图携带 UUID Command ID；
- 幂等负载由标准化后的 direction、本方 Party ID、往来 Party ID、标题、金额、币种、到期日、人工来源依据和可信 Actor 身份计算 SHA-256；
- 相同 Command ID 与相同标准化负载重放返回原 Obligation 及其原始 Party 快照，不重复读取今天的 Party 资料生成新快照；
- 相同 Command ID 改变任一业务字段或 Actor 时返回显式幂等冲突；
- PostgreSQL 事务按 Command ID 获取 advisory transaction lock，并在首次创建时以固定 Party ID 顺序读取两个 Party，避免相同命令并发形成重复结果；
- 不通过标题、金额或来源依据做相似义务合并，也不猜测两条业务事实是否重复。

## 权限、审计与数据边界

- GET／POST `/obligations` 和 GET `/obligations/{id}` 均要求 Money Flow 自己的有效本地会话；本轮只允许 administrator；
- POST 必须通过 CSRF；请求中的 Actor 字段全部忽略，Actor 只从当前 Principal 构造；
- `obligation_draft_created` 审计保存 Command ID、可信 Actor、direction、双方 Party ID、金额、币种、到期日和 draft version；
- Obligation、不可变创建命令和审计必须原子提交；Party 不存在或审计失败时不能留下 Obligation 或 Command；
- 创建命令和审计禁止 UPDATE／DELETE；Obligation 草稿本轮没有编辑、删除或状态推进入口；
- Money Flow 只读取并写入自己的数据库，不访问 Contract Ledger 数据库、Cookie、用户、Party、HTTP 接口或文件。

## 验收标准

1. administrator 可以分别建立 receivable 和 payable 草稿，列表和详情正确显示方向、金额、到期日、来源及 draft version 1。
2. receivable 的 creditor 必须是选择的本方主体，payable 的 debtor 必须是选择的本方主体；双方不能相同。
3. 创建时冻结双方 Party revision、类型、名称和登记标识；之后修改 Party 当前名称不改变既有草稿详情。
4. 非法 direction、Party ID、同一 Party、空或过长标题、零／负／超精度金额、非法币种、非法日期和空或过长来源依据均明确拒绝且不写数据库。
5. 金额标准化不经过浮点数；`000100.500000` 形成 `100.5`，最小 `0.000001` 可用。
6. 相同 Command ID 和相同标准化负载串行或并发重放只形成一个 Obligation、一个 Command 和一条审计；改变负载或 Actor 返回冲突。
7. Party 不存在或故意令审计失败时，Obligation、Command 和审计全部为零；创建命令和审计不可变。
8. HTTP POST 要求登录、administrator 和有效 CSRF；客户端 Actor 字段不能覆盖真实用户。
9. 列表和详情来自 PostgreSQL；数据库错误不伪装为空列表、404 或创建成功。
10. migration 从 `000003` 升级到 `000004` 并可重复执行，表、索引、约束、外键和不可变触发器存在。
11. 单元、竞态、真实 PostgreSQL、真实 HTTP 和独立启动验证通过；Contract Ledger 停止时仍能完整创建和查看草稿。

## 明确不做

- 草稿编辑、取消、确认或 `open` 状态；
- Obligation revision 1、确认审批或外部决策门禁；
- 义务调整、减免、坏账、争议、关闭或余额计算；
- 发票／票据、实际收付款、资金账户、Allocation 或核销；
- 分期数组；分期义务将在上层按多个独立义务表达；
- 第三方结算对象、代收代付或收款依据；
- 多币种核销、汇率或会计收入确认；
- Contract Snapshot Ref、Contract → Money Flow 连接器、跨库外键、共享 Party ID 或自动义务生成；
- 客户行业分类、项目强制归属、审批字段、万能扩展字段和重复义务智能判断。
