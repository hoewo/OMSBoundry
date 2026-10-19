# Money Flow Ledger：当前经营收支汇总与逐项下钻

里程碑：M4  
功能编号：MF-M4-011  
日期：2026-09-20  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够查看一个只由当前已验证事实派生的经营收支汇总：按本方账簿主体与币种分别展示当前应收余额、当前应付余额、当前有效实际收款、当前有效实际付款、未分配收款和未分配付款，并从每一项下钻到构成该金额的 Obligation 或 Cash Transaction。

本功能是只读 current 投影，不创建第二套汇总事实，不把义务余额称为会计收入，不把实际收付款称为银行账户余额，也不推测尚未实现的退款净额。

## 分组边界

每一行由以下复合键唯一确定：

```text
ledger owner Party ID + currency
```

- 同名 Party 不合并；Party ID 才是分组身份；
- CNY、USD 等不同币种绝不相加，也不换算；
- 页面使用 Money Flow 当前 Party 名称作为分组标签，点击事实后仍查看各 Obligation／Cash Transaction 已冻结的历史 Party 快照；
- 只有至少一项金额有正贡献的分组进入汇总，全部为零的已关闭义务或已冲销资金不制造空行。

## 六项指标与唯一公式

单条正式 Obligation 当前余额继续使用既有公式：

```text
obligation outstanding
  = current effective amount
  - net allocated amount
  - remitted amount
```

单条 Cash Transaction 当前投影继续使用既有公式：

```text
cash current effective amount
  = original amount when no Cash Reversal exists
  = 0               when Cash Reversal exists

cash unallocated amount
  = current effective amount - net allocated amount
```

汇总指标：

| 指标键 | 页面名称 | 贡献事实 | 汇总金额 |
| --- | --- | --- | --- |
| `receivable_outstanding` | 当前应收余额 | 正 outstanding 的正式 receivable Obligation | `SUM(outstanding)` |
| `payable_outstanding` | 当前应付余额 | 正 outstanding 的正式 payable Obligation | `SUM(outstanding)` |
| `receipt_effective` | 当前有效实际收款 | 未被 Cash Reversal 冲销的 receipt | `SUM(current effective amount)` |
| `payment_effective` | 当前有效实际付款 | 未被 Cash Reversal 冲销的 payment | `SUM(current effective amount)` |
| `receipt_unallocated` | 未分配收款 | unallocated 大于 0 的 receipt | `SUM(unallocated amount)` |
| `payment_unallocated` | 未分配付款 | unallocated 大于 0 的 payment | `SUM(unallocated amount)` |

每项同时显示正贡献事实数量。未分配金额是对应有效实际收付款的组成信息，不能与有效金额再次相加形成“总额”。

## 明确排除的事实

- Obligation 草稿没有正式余额，不进入汇总；
- outstanding 为 0 的 open／closed Obligation 不贡献当前应收或应付；
- 已完整 Cash Reversal 的资金 current effective 与 unallocated 都为 0，不贡献实际收付；
- Allocation 与 Allocation Reversal 只通过既有净额公式影响 Obligation outstanding 和 Cash unallocated，不作为独立金额重复汇总；
- Remission 只通过既有余额公式降低 outstanding，不作为现金；
- Internal Transfer 是非经营性内部资金事实，不进入六项指标；
- 尚未实现的 Refund／Returned Payment、Billing Document、坏账、银行余额、会计凭证和 Contract Ledger 事实全部排除。

## 一致性与精确金额

- 汇总请求在 Money Flow 自己的 PostgreSQL 中使用只读 `REPEATABLE READ` 事务读取；
- 下钻请求也在一个独立只读 `REPEATABLE READ` 事务中读取，并从该次返回的贡献事实精确求和得到下钻总额；
- 汇总与用户稍后点击的下钻是两个请求，期间若形成新事实，允许显示不同的 `generated_at`；不能假装跨请求冻结业务；
- 数据库金额继续使用 `numeric`，Go 层只接受非负、最多六位小数的十进制文本，并用六位定标整数求和；禁止二进制浮点数；
- 任一存量事实产生负余额、超精度或非法金额时，整次投影失败，不能截为零或返回部分汇总。

## 用户流程

```text
管理员登录
  → 打开“当前经营汇总”
  → 查看按本方主体、币种隔离的六项 current 指标与贡献数量
  → 点击任一正金额指标
  → 服务端验证指标键、Party ID 和币种
  → 在新的同库一致快照中读取全部正贡献事实
  → 页面显示精确下钻总额和逐条 Obligation／Cash Transaction
  → 点击事实进入现有详情页核对 revision、Allocation、Remission 或 Reversal 历史
```

## 下钻内容

Obligation 指标下钻至少显示：

- Obligation ID、标题和方向；
- 当前往来方历史快照名称；
- 到期日、current revision、current effective amount；
- net allocated、remitted 与本项贡献 outstanding；
- 链接到现有 Obligation 详情。

Cash 指标下钻至少显示：

- Cash Transaction ID、描述和收款／付款方向；
- 当前往来方历史快照名称；
- 发生日、账户标签、original amount；
- current effective、net allocated、unallocated 与本项贡献金额；
- 链接到现有 Cash Transaction 详情。

下钻只列出对所选指标有正贡献的事实。构成金额和页面列出的事实集合必须一一对应。

## 权限和错误边界

- GET `/operating-summary` 和 GET `/operating-summary/details` 都要求 Money Flow 自己的有效 administrator 会话；
- 本功能不写业务事实，不产生审计记录，也不需要 CSRF 写入动作；页面仍提供安全注销表单；
- 非法指标、Party ID 或币种返回 `422`；合法查询但没有任何正贡献事实返回 `404`；
- 数据库读取失败、存量金额非法或投影不变量破坏返回 `500`，不能伪装为空汇总；
- 本功能只访问 Money Flow 数据库，不读取 Contract Ledger API、数据库、Party、Cookie 或领域对象。

## 验收标准

1. 汇总按 ledger owner Party ID 与币种分别形成行，同名 Party、不同 Party ID 不合并，不同币种不合并。
2. receivable／payable 只汇总正式 Obligation 的正 outstanding，使用 current revision、Allocation／Reversal 净额和 Remission；草稿、零余额与 closed 零余额不贡献。
3. receipt／payment effective 只汇总没有 Cash Reversal 的当前有效资金；已冲销资金不贡献。
4. receipt／payment unallocated 使用 current effective 减 Allocation／Reversal 净额；已全部分配的资金不贡献未分配指标。
5. Internal Transfer、Billing、坏账、退款、Contract 和会计事实均不进入任何指标。
6. 每项金额和事实数量正确，`0.1 + 0.2` 精确等于 `0.3`，六位小数不丢失，不使用浮点数。
7. 六个指标的正金额均提供下钻链接；零金额没有伪造的下钻入口。
8. Obligation 下钻总额精确等于页面全部 Obligation 贡献金额之和，并链接到现有详情。
9. Cash 下钻总额精确等于页面全部 Cash Transaction 贡献金额之和，并链接到现有详情。
10. 非法 metric、Party ID、currency 返回 `422`；没有贡献事实返回 `404`；数据库错误返回 `500`。
11. 汇总与下钻分别使用只读 `REPEATABLE READ` 快照，并显示各自 UTC `generated_at`。
12. 匿名请求跳转本地登录，非 administrator 返回 `403`；有效 administrator 可读且不能通过请求覆盖 Actor 或金额。
13. 真实 PostgreSQL 测试覆盖两本方主体、两币种、两方向、部分核销、Allocation Reversal、Remission、Cash Reversal、零余额关闭和 Internal Transfer 排除。
14. 隔离空库真实 HTTP 流程从建立主体、义务、资金和核销到汇总、六类下钻中的代表路径全部通过。
15. Contract Ledger 停止时 Money Flow 仍可独立生成汇总和下钻；当前没有跨系统业务流动，因此不制造连接器联调。

## 明确不做

- 按历史 `as_of` 重放整个汇总；
- 日期区间、月报、趋势图、现金预测或到期计划；
- 收入、利润、成本、税额、会计期间或总账口径；
- 银行账户主数据、账户余额或对账；
- 退款／退回款净额、拒付、手续费和跨币种换算；
- 导出、导入、定时报表或外部 BI；
- Contract → Money Flow 连接器或跨系统联调。
