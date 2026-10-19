# ADR-0025：历史余额使用单调 recorded-time 重放

日期：2026-09-20  
状态：接受

## 背景

Obligation revision、Allocation／Reversal、Remission 和 Closure 已经是不可变事实，并分别保存服务端记录时间，理论上能够重放历史余额。但现有 schema 只保护金额和状态不变量，没有阻止同一 Obligation 后写入的事实携带更早时间。

如果只使用 `created_at <= as_of` 或当前根状态倒推，极端并发、时钟回拨或直接 SQL 会生成顺序错误的历史前缀。另一方面，`due_on` 和 Cash `occurred_on` 是业务 LocalDate，不能替代系统何时记录该事实。

## 决策

- 历史余额采用 recorded-time：revision 的 `recorded_at`、Allocation 的 `applied_at`、Reversal 的 `reversed_at`、Remission 的 `recorded_at` 和 Closure 的 `closed_at`；
- `as_of` 使用带时区 RFC3339 instant，统一转换为 UTC，包含时间小于或等于该时刻的事实；未来时刻拒绝；
- 同一 Obligation 的上述事实时间由 PostgreSQL temporal guard 保证非递减；等时事实是不可拆分批次，不使用随机 UUID 创造虚假先后；
- migration 先验证既有全部事件时点的历史前缀，再安装写入门禁；验证失败时停止升级，不重排或覆盖历史；
- 历史 effective amount 取截至时点的最大 revision，净核销取 Allocation 减届时已存在的 Reversal，减免取累计 Remission，状态由届时是否存在 Closure 派生；
- 查询使用只读可重复读事务，不依赖 obligations 当前 `status` 或 `current_revision`；
- 第一版只提供单条 Obligation 快照和下钻，不把它扩成历史账龄报表、会计期间或跨系统报表。

详细行为见 [历史 Obligation 余额 recorded-time as_of](../specs/money-flow-ledger/historical-obligation-balance-as-of.md)。

## 结果

优点：

- 历史结果完全来自不可变事实，可解释且能重建；
- current 状态和后续纠错不会污染过去快照；
- 数据库门禁让应用写入、并发写入和直接 SQL 共享同一时间不变量；
- 明确区分系统记录历史与业务生效日、账龄分类日和会计期间。

代价与限制：

- 时间回拨或在锁前取得的旧时间可能导致写入被拒绝，调用方需要刷新重试；这是拒绝伪历史的代价；
- 等时事实不提供批次内部中间状态；
- 既有异常数据会阻止 migration，需要先人工审计，不能自动修复；
- 当前只支持单 Obligation，不提供跨账簿历史汇总。

## 否决方案

### 使用 obligations.updated_at 和当前余额倒推

否决。根表只保存 current 指针和状态，无法解释某次 Allocation Reversal、Remission 或中间 revision。

### 只按业务 LocalDate 查询

否决。due date 与 cash occurred date 回答业务日期，不说明系统在当时是否已经记录该事实。

### 信任现有时间戳但不增加单调门禁

否决。后写事实可能带更早时间，导致历史前缀失去因果一致性。

### 为同刻事实按 UUID 排序

否决。UUID 是技术标识，不是业务或提交顺序；同刻事实应作为一个原子可见批次处理。

### 直接建设历史账龄和会计期间

否决。应先证明单条余额重放正确；账龄汇总和会计期间还有各自时区、关账与业务有效时间规则。
