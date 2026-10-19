# Money Flow Ledger：当前 Obligation 逾期与账龄投影

里程碑：M4  
功能编号：MF-M4-008  
日期：2026-09-20  
状态：已实现并验证

## 目标

让管理员基于查询时的 current Obligation 事实，查看仍有余额的 open 应收／应付在指定分类日的到期与逾期分布，并能下钻到每一条义务。

本功能是可重建的只读投影，不创建账龄事实、不修改 Obligation，也不声称能够重放过去某日的历史余额。

## 事实来源

每条投影只使用当前已有事实：

- Obligation current revision 的方向、标题、本方、往来方、币种和 `due_on`；
- current effective amount；
- Allocation 与 Allocation Reversal 的当前净分配额；
- Obligation Remission 的当前累计额；
- 根状态。

当前剩余金额继续使用唯一公式：

```text
current outstanding
  = current effective amount
  - net allocated amount
  - remitted amount
```

账龄页面不保存、覆盖或修正上述任何事实。

## 分类日边界

- 未提供 `as_of_date` 时，使用服务当前 UTC 日期；
- 可以选择今天或未来日期，用于把**查询时的当前余额**按该日期重新分类；
- 过去日期拒绝查询，因为历史 Adjustment、Allocation、Reversal、Remission 和 Closure 还没有完整的历史时点重放接口；
- 未来分类不预测期间内会发生的收付款、减免、调额或关闭，页面必须明确这一限制；
- 日期只接受严格的 `YYYY-MM-DD` LocalDate。

`as_of_date` 是账龄分类基准，不是业务事实发生时间，也不是历史报表快照。

## 纳入与排除

仅纳入同时满足以下条件的义务：

- 状态为 `open`；
- current outstanding 大于 0。

以下对象不进入报告：

- draft；
- closed；
- outstanding 为 0 但尚未执行受控关闭的 open 义务；
- Cash Transaction、Internal Transfer 和未分配资金。

## 逾期与分段规则

```text
due_on >= as_of_date  → 未到期
due_on <  as_of_date  → 逾期天数 = as_of_date - due_on
```

固定分段：

| Bucket | 规则 |
| --- | --- |
| `not_due` | 到期日等于或晚于分类日 |
| `days_1_30` | 逾期 1–30 天 |
| `days_31_60` | 逾期 31–60 天 |
| `days_61_90` | 逾期 61–90 天 |
| `days_91_plus` | 逾期 91 天及以上 |

到期日当天不算逾期。所有差值按 LocalDate 的自然日计算，不使用时刻或小时数。

## 汇总与下钻

- 汇总必须按 `direction + currency` 分组；应收、应付不得相减，不同币种不得相加；
- 每组展示义务数、当前剩余总额、逾期笔数、逾期总额和五个 bucket 金额；
- 金额使用精确十进制累加，不经过二进制浮点数；
- 明细展示标题、方向、本方、往来方、到期日、逾期天数、bucket 和 current outstanding；
- 每条明细链接到现有 Obligation 详情，以便查看 revision、Allocation、Reversal、Remission 和 Closure 前置事实；
- 明细采用确定顺序：应收在前、应付在后；同方向按币种、bucket、到期日和 Obligation ID 排序。

## 权限与失败行为

- 第一版沿用当前 Money Flow 的 administrator 访问边界；
- 只读查询不写业务审计；登录与请求安全边界保持不变；
- 日期无效或早于服务当前日期时返回 `422` 并展示原因；
- 读取任何组成事实失败时返回 `500`，不能把失败渲染为空报告。

## 明确不做

- 历史 `as_of` 余额重放；
- 未来现金流预测；
- 账龄策略配置或客户自定义 bucket；
- 坏账、预期信用损失或会计减值；
- 票据账龄、付款计划、催收流程或自动通知；
- Excel 导出；
- Contract Ledger 连接。

## 验收条件

1. 到期日等于分类日的义务落入未到期，前一天分别落入逾期 1 天；
2. 30／31、60／61、90／91 天边界准确；
3. draft、closed 和 open 零余额不出现在报告；
4. Allocation Reversal、Amount Adjustment 与 Remission 改变 current outstanding 后，报告即时反映同一余额公式；
5. 应收／应付和不同币种分别汇总，精确小数累加无浮点误差；
6. 每个汇总金额可由页面明细逐项重建，每条明细可下钻到 Obligation；
7. 过去日期被明确拒绝，未来日期页面明确不是余额预测；
8. Money Flow 在 Contract Ledger 停止时仍能独立生成报告；
9. 单元测试、真实 PostgreSQL 纵向测试、竞态测试和独立构建通过。
