# C3 项目经营链路只读组合

## 元信息

- 路径：`spec/assembly/c3-commercial-summary.md`
- 上层旅程：`full-reference-oa-validation.md` 的 C3
- 状态：🟡 本地 D1 已验证；真实浏览器最终点击、双 Tenant 和真实 Ubuntu 待验
- 事实所有者：Contract Ledger、Money Flow Ledger、Billing / Invoice
- 组合上下文所有者：Project Workspace

## A → B

项目经营人员需要跨合同、应收／回款和开票应用判断同一项目的经营进展 → 在 Project Workspace 看到来源明确、可下钻、允许局部失败的实时分段，而不把不同应用的数据复制到共享表。

## 用户结果

1. 用户从一个 Project 打开“经营链路”，看到该 Project 已显式连接的合同、确认义务、资金流水和开票登记。
2. 每个分段显示来源应用、业务状态、linked revision、source revision、读取耗时和来源入口。
3. Contract 显示当前有效合同金额；Money 显示义务的确认／核销／汇付／未清金额或资金流水的金额／核销；Billing 显示已开票登记金额。
4. 一个来源不可用、引用失效或 revision 变化时，只影响对应分段；其他分段仍可读。
5. 未知金额明确显示“未知”，不能按零金额展示或参与计算。

## 不变量

- Project 只拥有 Resource Link 和 Project 上下文，不拥有合同、义务、资金、核销或开票事实。
- 组合结果只存在于当前响应，不写数据库、不形成新的可写聚合、不替代来源应用。
- 不跨来源自动加总。合同金额、义务金额、现金金额和开票金额语义不同，第一版只并列展示。
- 只读取 active Resource Link；解除连接后不再出现在组合结果中。
- 每次来源读取都使用 Project 工作负载身份交换目标 audience、单 action、精确资源 Scope 的短期凭证，目标应用最终授权。
- 人类用户必须同时具备 Project 精确 Scope 与独立的 `project-workspace.commercial-summary.read`；普通 `project-workspace.read` 不能隐式取得经营金额。
- 用户跳转来源应用时必须重新取得该应用的人类 Capability；服务间凭证不能交给浏览器。
- Segment 顺序与 Project Resource Link 顺序稳定一致，不因并行返回顺序改变。

## 状态与恢复

| Segment 状态 | 含义 | 用户恢复 |
| --- | --- | --- |
| `available` | 来源当前投影已读取且 revision 与 Link 一致 | 可下钻来源 |
| `revision_changed` | 来源已产生新 revision | 查看来源并决定是否重建 Link；不静默覆盖 linked revision |
| `invalid` | 资源不存在、租户／类型不匹配或投影不符合契约 | 检查 Link 或来源事实 |
| `unavailable` | IAM、网络或来源应用暂不可用 | 保留其他分段并可重新读取 |

## 第一版范围

- Contract：`contract`。
- Money Flow：`obligation`、`transaction`。
- Billing：`invoice_registration`。
- Project 页面只读组合和来源下钻。
- 最多八个来源请求并行；所有请求仍受统一超时上限约束。

## 明确不负责

- 不自动从合同创建义务、从义务创建发票或从发票创建现金。
- 不实现应开未开、回款预测、税务生命周期、红冲／作废或会计分录。
- 不建立财务数据仓库、跨应用共享表、缓存真相或统一 ORM。
- 不把 `registration_invalidated` 冒充现实票据已作废。

## 验收

- [x] Contract、Money 和 Billing 引用端口返回版本化、类型化 C3 投影。
- [x] Project 使用并行读取，保持输入顺序，并对单来源失败局部降级。
- [x] Contract 未知金额不显示为 0。
- [x] 页面显示来源、revision、耗时、四类状态和来源入口。
- [x] Project 工作负载策略仅允许三个目标应用的精确 reference action／资源类型。
- [x] 经营汇总使用独立人类 Capability；只有普通 Project read 时目标端返回 403，页面不展示经营页签。
- [ ] D1 从 Nginx → Gateway → IAM → Project 进入页面并完成真实点击（页面资源与 API 已通过，macOS 自动化通道仍报告锁屏）。
- [x] D1 实际建立合同、义务、资金和开票 Link，验证四类分段数据。
- [x] 分别停止 Contract、Money、Billing 和 IAM，证明局部降级、恢复后无需重启且无写入。
- [x] 完成总经理／财务允许、生产经理拒绝，以及撤权、恢复和账号停用矩阵。
- [x] 记录串行等价分段耗时与并行组合的 p50／p95；本地 D1 为 19/362 ms 与 13/220 ms，并行 p95 低于 800 ms。

直接证据见 [OA C3 项目经营链路 D1 实验报告](../../../docs/reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。本轮共 100 个 HTTP 断言、0 个状态差异；该结论不覆盖双 Tenant、真实 Ubuntu 或 C3 自动业务动作。
