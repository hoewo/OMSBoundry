# Money Flow Ledger：历史 Obligation 余额 recorded-time `as_of`

里程碑：M4  
功能编号：MF-M4-009  
日期：2026-09-20  
状态：已完成并验证

## 目标

让管理员针对一条已经确认过的 Obligation，按明确的 UTC 记录时刻重放当时已被系统记录的 revision、Allocation／Reversal、Remission 和 Closure，得到可解释的历史余额快照。

本功能回答：

> 截至这个系统记录时刻，本账簿已经记录了多少有效义务、净核销、减免和剩余金额，当时是否已经受控关闭？

它不回答合同、票据、资金的业务生效日，也不构成会计期间、法律时效或银行入账历史。

## 时间语义

- `as_of` 必须是带时区的 RFC3339 时间，进入领域后统一为 UTC；
- 未提供时使用服务当前 UTC 时刻；
- 查询包含 `recorded_at / applied_at / reversed_at / closed_at <= as_of` 的事实；
- 未来时刻拒绝，避免把 current 结果伪装成未来预测；
- 早于 revision 1 确认时刻时，返回“该时刻义务尚未确认”，不把 draft 当作正式余额；
- 多个事实时间完全相同时视为同一不可拆分记录批次：该时刻之前全部排除，该时刻及之后全部纳入；
- revision 同刻时用连续 revision 号选择最大值，其他余额事实以集合求和，随机 UUID 不承担业务顺序。

`as_of` 是 recorded-time，不是 `due_on`、`occurred_on` 或其他业务 LocalDate。

## 历史事实重放

在指定 `as_of`：

```text
historical revision
  = recorded_at <= as_of 的最大连续 revision

historical effective amount
  = historical revision.effective_amount

historical net allocated
  = Σ(applied_at <= as_of 的 Allocation)
  - Σ(reversed_at <= as_of 的对应 Allocation Reversal)

historical remitted
  = Σ(recorded_at <= as_of 的 Remission)

historical outstanding
  = historical effective amount
  - historical net allocated
  - historical remitted

historical status
  = closed_at <= as_of 时为 closed，否则为 open
```

快照同时返回：

- revision、方向、标题、本方与往来方快照；
- effective、net allocated、remitted、outstanding 和币种；
- due date、revision recorded time；
- 截至时点的 Allocation、Reversal、Remission 数量形成的历史 balance sequence；
- 若已关闭，返回 Closure ID、结清构成和 closed time。

查询不得读取当前根状态或 current revision 后再“减回去”。所有值直接从截至 `as_of` 的不可变事实构造。

## 时间单调门禁

历史查询上线前，schema 必须保证同一 Obligation 的记录事实时间不会倒退：

- confirmation revision；
- amount adjustment 与 resulting revision；
- Allocation；
- Allocation Reversal；
- Remission；
- Closure。

每个新事实的记录时间必须大于或等于该 Obligation 已存在的最大记录事实时间。相等表示同一不可拆分批次，允许；小于则数据库拒绝。

migration 必须在安装门禁前验证所有既有历史前缀：

- 每个事件时刻都能找到已确认 revision；
- net allocated 不为负；
- outstanding 不为负；
- 出现 Closure 的前缀 outstanding 必须为 0。

任何既有数据不满足时 migration 失败，不静默重排、不修改时间戳。

## 查询一致性与失败行为

- 单次快照在 PostgreSQL `REPEATABLE READ READ ONLY` 事务中生成；
- Obligation 不存在返回 `404`；
- 时刻格式无效、未来时刻或早于确认返回 `422`；
- 历史前缀出现负核销、负 outstanding、closed 非零余额或其他矛盾时返回 `500`，不能展示看似成功的错误快照；
- 只读查询不写审计、不改变任何 Command 幂等状态。

## 页面入口

- Obligation 详情提供“历史余额”入口；
- 查询页面接受 RFC3339 UTC 时刻并明确 recorded-time 口径；
- 快照展示公式组成，并链接回当前 Obligation 详情；
- 当前详情与历史快照并存，历史查询不替换 current 投影。

## 明确不做

- draft 历史；
- 按业务日期追溯、回溯生效或补录历史有效期；
- 会计期间、账期关账或法定报表；
- 历史账龄列表、跨义务汇总或批量导出；
- Cash Transaction 历史余额、未分配资金历史；
- 票据、退款、坏账、预测或 Contract Ledger 连接器。

## 验收条件

1. 确认前返回未确认；确认时刻得到 revision 1 全额余额；
2. Allocation 前后、Reversal 前后、连续 Adjustment 前后和多笔 Remission 前后均得到对应精确历史金额；
3. Closure 前为 open，Closure 时刻及之后为 closed 且 outstanding 为 0；
4. 同时刻的 resulting revision 与 Adjustment 作为一个批次出现，不暴露中间半状态；
5. current 根状态、后续 revision 和后续余额事实不会污染较早快照；
6. 非法、未来和缺失时刻有明确失败；不存在与尚未确认能够区分；
7. migration 从 `000013` 升到 temporal guard 后不改写既有事实并可重放；倒退时间和无效历史前缀被数据库拒绝；
8. 单元、真实 PostgreSQL、竞态、完整 HTTP、独立运行和升级验证通过；
9. Contract Ledger 停止时 Money Flow 仍可独立查询历史余额。
