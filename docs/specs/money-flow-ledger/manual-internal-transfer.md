# Money Flow Ledger：人工内部账户转账

里程碑：M3  
功能编号：MF-M3-008  
日期：2026-09-19  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够为一个明确的本方账簿主体，人工登记一笔已经发生的同币种内部账户转账，并核对来源账户标签、目标账户标签、精确金额、发生日和来源依据。系统以一条不可变 Internal Transfer 表达完整移动，不把它拆成一条经营付款和一条经营收款，因此不会把公司内部资金移动重复计入经营流入或流出。

## 核心建模决策

- Internal Transfer 是已经发生的内部资金移动，不是应收／应付义务、经营收款、经营付款、Allocation、Payment Intent 或会计凭证；
- 一次转账只形成一条 Internal Transfer，不生成两条 Cash Transaction；
- 现有 Cash Transaction 继续只表示本方与不同外部往来 Party 之间的 `receipt`／`payment`，并且只有它能参与 Allocation；
- Internal Transfer 使用同一个本方 Party 快照，同时保存不同的来源账户标签和目标账户标签，不需要虚构付款方／收款方两个 Party；
- 当前不建设资金账户主数据或账户余额，因此不持久化两条可编辑“资金腿”。未来账户余额投影必须从同一 Internal Transfer 派生来源账户 `-amount` 与目标账户 `+amount`；
- 经营收款／付款投影只读取 Cash Transaction；Internal Transfer 结构上不进入经营流入、经营流出、未分配资金或义务核销；
- Internal Transfer 不能被 Allocation 引用，数据库中也不存在该引用路径。

详细依据见 [ADR-0016](../../adr/0016-internal-transfer-is-a-separate-non-operating-fact.md)。

## 业务事实

- 管理员必须选择一个 Money Flow 当前存在的本方 Party；创建时冻结其 Party ID、revision、类型、法定名称／姓名和可选登记标识；
- 描述必填，去除首尾空白后长度为 1 至 200 个字符；
- 金额必填且严格大于 0，只接受普通十进制文本，整数最多 18 位、小数最多 6 位，保存为 PostgreSQL `numeric(24,6)`，不经过二进制浮点数；
- 币种去除空白并转为大写，必须是三个英文字母；第一版只允许同币种账户间移动，不形成汇兑事实；
- 发生日必填并使用严格 `YYYY-MM-DD` LocalDate；允许补录历史转账；
- 来源账户标签和目标账户标签均必填，清理后长度为 1 至 100 个字符，并且不区分大小写比较后必须不同；
- 账户标签只是本系统内的人工归属标签，不是银行账号、共享 Account 主数据或访问凭据；
- 来源类型固定为 `manual`；人工来源依据必填，清理后长度为 1 至 200 个字符；
- 外部转账参考可选，清理后长度最多 100 个字符。非空时，在“本方 Party + 不区分大小写的来源账户标签 + 目标账户标签 + 来源类型”范围内不区分大小写唯一；
- 新记录立即形成不可变 `recorded` 事实；本轮没有草稿、编辑、删除、反向转账或冲销；
- Internal Transfer ID、Command ID 和 Audit ID 是三个不同 UUID。

## 用户流程

```text
管理员登录
  → 打开内部账户转账列表
  → 选择本方账簿主体
  → 填写描述、精确金额、币种和发生日
  → 填写不同的来源／目标账户标签和人工来源依据
  → 可选填写外部转账参考
  → 服务端从会话构造可信 Actor
  → PostgreSQL 冻结本方 Party 当前快照
  → Internal Transfer、创建命令和审计在同一事务提交
  → 进入详情页核对完整、不可变的非经营资金移动
```

## 幂等、重复参考与并发

- 每次创建意图携带 UUID Command ID；
- 幂等负载由标准化后的本方 Party ID、描述、金额、币种、发生日、两个账户标签、人工来源依据、外部转账参考和可信 Actor 身份计算 SHA-256；
- 账户标签和外部转账参考在幂等与重复比较时使用不区分大小写的规范键，页面保留首次成功事实的清理后文本；
- 相同 Command ID 和相同标准化负载串行或并发重放返回原 Transfer 及原始 Party 快照；
- 相同 Command ID 改变任一字段或 Actor 返回显式幂等冲突；
- 非空外部转账参考先按规范唯一范围获取 advisory transaction lock，再查询已存在事实；不同 Command ID 命中同一范围时返回显式重复参考冲突；
- PostgreSQL 部分唯一索引再次保护并发重复；外部转账参考为空时只按 Command ID 幂等，不使用描述、金额或日期模糊去重。

## 权限、审计与错误边界

- GET／POST `/internal-transfers` 和 GET `/internal-transfers/{id}` 只允许 Money Flow 自己的 administrator；
- POST 必须通过 CSRF；Actor 只来自登录 Principal，客户端提交的 Actor、状态、统计分类和记录时间全部无效；
- `internal_transfer_recorded` 审计保存 Command ID、可信 Actor、本方 Party、精确金额、币种、发生日、两个账户标签、来源和外部参考；
- Transfer、创建命令和审计必须原子提交；Party 缺失、重复参考或审计失败时不得留下部分事实；
- Transfer、创建命令和审计禁止 UPDATE／DELETE；错误登记以后只能通过 M4 的独立反向／冲销事实纠正；
- 列表和详情数据库失败返回 500，不能伪装为空列表或 404；父 ID 非法或事实不存在返回 404；
- 本功能只访问 Money Flow 自己的数据库，不访问 Contract Ledger。

## 明确不做

- 把一次内部转账拆成经营 payment 与 receipt；
- 让 Internal Transfer 参与 Obligation、Allocation 或未分配资金；
- 资金账户目录、银行账号、实时账户余额、透支校验或账户权限；
- 银行接口、自动对账、付款执行或“银行已核验”状态；
- 跨币种转账、汇率、手续费或汇兑损益；
- 修改、删除、反向转账、退款或 Cash Reversal；
- 经营统计、指定时点账户余额或会计复式分录；
- Contract Ledger 连接器或任何跨系统业务流动。

## 验收标准

1. administrator 可以登记和查看一条同币种内部转账，列表和详情显示本方快照、不同账户标签、精确金额、日期、来源和 `recorded` 状态；
2. 非法 UUID、缺失 Party、空或过长字段、相同账户标签、零／负／超精度金额、非法币种／日期和过长外部参考均明确拒绝且不写数据库；
3. 金额标准化不经过浮点数；`000100.500000` 形成 `100.5`，`0.000001` 可用；
4. 相同 Command ID／负载串行或并发重放只形成一个 Transfer、一个 Command 和一条审计；改变负载或 Actor 返回冲突；
5. 不同 Command ID 使用大小写等价的同范围外部转账参考时，串行或并发只有一个成功；不同账户方向或本方 Party 可以复用；
6. Party 当前资料变化不改写既有 Transfer 快照；Party 缺失或审计失败时整体回滚；Transfer、Command 和审计不可变；
7. 建立既有经营 receipt 与 payment 后再登记 Internal Transfer，Cash Transaction 数量、经营 receipt／payment 金额、Allocation 和未分配资金完全不变；Transfer 单独形成一条事实；
8. 数据库 schema 不允许 Allocation 引用 Internal Transfer；页面也不把它列为可分配资金；
9. HTTP 覆盖登录、administrator、CSRF、可信 Actor、创建、重放、冲突、列表、详情、404 和数据库失败；
10. migration 从 `000007` 升级至 `000008` 并可重放，原 Party、Obligation、Cash Transaction 与 Allocation 全部保持不变；
11. 隔离空库真实 HTTP 流程完成 bootstrap、Party、经营收付款、内部转账与数据库统计隔离核对；
12. Contract Ledger 停止时 Money Flow 仍能完成 migration、ready、登录、内部转账登记、重放、冲突、列表和详情；当前没有跨系统业务流动，因此不伪造连接器联调；
13. `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。
