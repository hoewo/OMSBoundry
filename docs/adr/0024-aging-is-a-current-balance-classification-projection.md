# ADR-0024：账龄是当前余额的日期分类投影

日期：2026-09-20  
状态：接受

## 背景

Money Flow 已拥有可重建的 current Obligation outstanding，但尚未提供逾期和账龄视图。M4 同时规划了历史 `as_of` 余额；如果直接允许任意过去日期，却仍使用今天的 current revision、Allocation、Reversal 和 Remission，就会生成看似历史、实际错误的报表。

当前每条 Obligation 已有不可变 `due_on` 和精确 current outstanding，因此可以先交付不新增业务事实的当前账龄投影。

## 决策

- 账龄是查询时 current Obligation 余额的只读派生，不保存账龄表或可写汇总；
- 只纳入 `open` 且 current outstanding 大于 0 的义务；draft、closed 和 open 零余额排除；
- `due_on >= as_of_date` 为未到期，过去 1–30、31–60、61–90、91+ 自然日分别进入固定 bucket；
- `as_of_date` 缺省为服务当前 UTC 日期，只允许今天或未来；未来日期只改变到期分类，不预测余额变化；
- 在完整历史事实重放交付前拒绝过去日期，避免错误使用 current 余额冒充历史余额；
- 汇总按 direction 与 currency 分组，不净额合并应收／应付，也不跨币种换算；
- 金额以精确十进制累加，每个汇总必须能下钻到现有 Obligation 详情；
- 本功能不增加 migration、审计动作或跨系统依赖。

详细行为见 [当前 Obligation 逾期与账龄投影](../specs/money-flow-ledger/current-obligation-aging-projection.md)。

## 结果

优点：

- 利用已经验证的余额事实快速形成可用经营视图；
- 不建立第二套余额真相，也不引入异步投影一致性问题；
- 明确阻止错误的历史报表；
- 保持方向、币种和明细来源可解释。

代价与限制：

- 不能查询过去某日真实 outstanding；
- 未来分类只是一种当前余额假设，不是现金预测；
- bucket 暂时固定，不能按客户偏好配置；
- 第一版需要读取当前候选义务后在应用层精确汇总，后续只有出现真实性能证据时才增加缓存或专用读模型。

## 否决方案

### 接受任意历史日期并使用 current 余额

否决。调额、核销、反向、减免和关闭都可能发生在分类日之后，这会把今天的事实错误投射到过去。

### 在 obligations 根表保存 overdue 和 aging_bucket

否决。它们随日期自动变化，会形成需要定时更新且可能漂移的第二真相源。

### 合并应收与应付或不同币种

否决。净额会掩盖各自风险，跨币种相加则隐含不存在的汇率事实。

### 同时实现催收、坏账和现金预测

否决。这些能力需要各自独立的事实、策略和验收，不属于当前只读投影切片。
