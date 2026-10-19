# ADR-0016：内部账户转账是独立的非经营资金事实

日期：2026-09-19  
状态：接受

## 背景

Money Flow 已经用 Cash Transaction 表达本方 Party 与不同往来 Party 之间已经发生的 `receipt`／`payment`，并允许这些资金通过 Allocation 核销经营义务。M3 退出门槛同时要求，公司内部账户之间的资金移动不能被拆成经营付款与经营收款后重复计入实际流出和流入。

当前只有人工账户标签，没有 Account 主数据、账户余额、银行接口或会计复式分录。若为了内部转账提前建立两条独立资金腿，会引入尚无消费者的账户模型、跨记录原子配对和冲销语义；若直接把现有 Cash Transaction 放宽为 payer 与 payee 相同，又会破坏其外部往来方快照、未分配资金和 Allocation 语义。

## 决策

内部账户转账使用独立、不可变的 Internal Transfer aggregate：

- 一次转账只形成一条事实；
- 保存同一个本方 Party 的创建时快照；
- 保存不同的来源账户标签与目标账户标签、精确金额、币种、发生日、人工依据和可选外部参考；
- 不生成 Cash Transaction，不参与 Allocation、义务余额或未分配资金；
- 经营 receipt／payment 投影只读取 Cash Transaction，因此 Internal Transfer 不进入经营流入或流出；
- 将来的账户余额投影从同一 Transfer 派生来源 `-amount` 与目标 `+amount`，不把两条派生腿提升为可独立编辑的业务事实；
- 将来的错误修正追加反向／冲销事实，不修改原 Transfer。

Internal Transfer 拥有独立 Command ID 幂等、外部参考重复保护、可信 Actor 审计和不可变数据库约束。它仍属于 Money Flow Ledger，不形成新系统或公共账户平台。

## 结果

优点：

- 数据结构上消除把一次内部移动误当两笔经营收支的可能；
- 保持 Cash Transaction 的双方 Party、Allocation 和未分配资金不变量不变；
- 在没有账户主数据时仍能完成可追溯的最小闭环；
- 以后增加账户余额时可以从单一事实确定性派生两端变化。

代价与限制：

- 当前不能计算真实账户余额或验证来源账户资金充足；
- 账户标签是人工文本，只在单条 Transfer 内保证来源与目标不区分大小写后不同；
- 不处理跨币种、手续费、银行双流水配对或自动对账；
- M4 实现反向／冲销时需要为 Internal Transfer 定义对应追加事实。

## 否决方案

### 建立一条 payment 和一条 receipt

否决。两条记录各自看起来都是经营资金，容易被分别统计、分配或冲销；即使增加自由文本标签，也不能形成结构性的不重复保证。

### 扩展 Cash Transaction 为 `internal_transfer`

当前否决。现有 Cash Transaction 要求不同 payer／payee Party，并天然拥有可分配余额；放宽后需要大量条件字段与条件约束，容易让 Internal Transfer 被误列为可核销资金。

### 立即建立 Account 与双腿账本

当前否决。没有账户余额、银行适配或复式记账需求证据，提前建立账户主数据和两腿生命周期会超出 M3。出现稳定需求时可把 Internal Transfer 投影到该模型，而不改写历史事实。
