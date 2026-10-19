# 统一工作台个人工作与经营指标 - 交互规范

## 核心任务

当前登录人从统一工作台看到已选装业务应用实时发布的可执行工作和按来源分区的经营指标，并通过来源深链进入原应用；工作台不推断、不保存、更改或代办来源状态，也不跨来源计算金额。

## 主路径

Gateway 先验证 Suite Session 并向工作台注入当前 Subject → 页面只从 Suite Manifest 发现声明 `work_item_projection` 的已启用来源 → 浏览器向 Gateway 请求该来源的当前人短期 Capability Credential → 通过同源 HttpOnly Cookie 读取来源 `WorkItemProjection v1` → 校验 schema、来源与响应 Subject → 按来源分组、排序和去重 → 点击深链进入来源应用 → 来源应用按 current state、Scope、`allowed_actions` 与 expected revision 再授权真正命令。

经营指标并行走独立路径：页面只发现声明 `metric_projection` 的已启用来源 → 取得同一当前人的来源短期 Credential → 读取 `MetricProjection v1` → 校验 schema、来源、Tenant、Scope、精确十进制／redacted 不变量 → 按来源、Ledger Owner Party 和 Currency 分区展示 → 点击深链回到来源明细。工作台不合计不同来源、口径、主体或币种。

## 决策点

- 页面没有 Subject 输入框，也不把浏览器提交的 Subject 当真；来源使用 IAM Credential 的 Subject。
- 工作台 JavaScript 看不到 Capability Token；Gateway 只通过 HttpOnly、来源路径限定 Cookie 转交短期凭证。
- `work-items.read` 与来源业务写动作分别授权。只有来源 Assignment 当前有效、目标资源在 Credential Scope 内且至少一个可执行动作仍获授权时，来源才发布工作项。
- 第一来源只支持直接分配给 Person 的 active `production_owner`；Group 到 Person 的解析需要版本化 Membership／Policy 证据后另行实现。
- 最多四路并发，每个来源 3.5 秒超时。一个来源失败不取消其他来源，也不把 `unavailable` 显示成空。
- Workbench 不代理 POST／PUT／DELETE，不记录“已处理”，不缓存完成事实。
- `statistics.read` 与 `amount.read` 分权；缺少金额能力时工作台显示“金额已隐藏”，并要求 `value` 与 `source_digest` 同时为 null，绝不显示 0。
- 指标来源按 Suite Manifest 显式选装；来源 403、5xx、超时或契约错误只降级该来源，不能清空或污染其他来源。

## 页面状态

| 状态 | 触发条件 | 展示与恢复 |
| --- | --- | --- |
| 加载中 | 正在为已选来源授权和读取 | 保留来源卡片与“正在连接” |
| 已同步 | 契约、来源和 Subject 校验通过 | 显示原因、状态、revision 和来源深链 |
| 空 | 来源成功返回 `items: []` | 明确“当前没有可执行工作项” |
| 无权读取 | IAM／Gateway／来源返回 403 | 不展示相邻 Subject 数据，提示缺少来源权限 |
| 暂不可用 | 超时、5xx、无效 JSON 或契约不匹配 | 只降级该来源，其他来源继续展示 |
| 未选装 | Manifest 没有声明来源 | 不发出来源请求，说明当前组合没有个人工作来源 |

经营指标另有同构状态：加载中、已同步、空、无权读取、局部不可用和未选装。已同步状态必须分别显示指标名、原样十进制金额或“金额已隐藏”、Currency 与 Ledger Owner Party；局部不可用不得显示为空或零。

## 导航与反馈

- 工作项按来源分组；组内先按 `due_at`／`observed_at`，再按稳定 Work Item ID 排序和去重。
- 点击条目只使用来源返回的相对深链，并限制在来源的 Suite `route_prefix` 下。
- Production 深链携带 `work_item_id`，来源页面载入列表后重新读取该工作项详情与最新 `allowed_actions`。
- Grant 撤销、账号停用或 Credential 过期后，新授权／新读取失败关闭；页面不复用旧动作。
- 指标条目按来源分组，再按 Ledger Owner Party、Currency 和稳定 Metric Key 排序去重；分组之间没有总计栏。

## 输入输出边界

输入只有 Gateway 注入的当前会话上下文、Suite Manifest 声明和来源只读投影。个人工作输出只包含来源、Work Item、Project／目标资源 Scope、原因、状态、动作、revision、观测时间、深链与目标端 `allowed_actions`；经营指标只包含来源、Metric Key、Tenant／Party Scope、单位、Currency、精确值或 redacted、口径、观测时间、摘要和深链。工作台不拥有 Assignment、Production Stage、完成状态、财务公式或金额事实。

## 当前验证边界

生产个人工作来源以及 Money Flow 经营指标来源、Workbench、Gateway Manifest 与 Access Contract 的零容器单元测试已通过。指标消费者当前只证明 Manifest 发现、Tenant／契约校验、来源分区、精确值／脱敏展示和来源局部失败文案；真实 Gateway→IAM→来源浏览器链、Credential 撤销时序、正式 D1、第二来源运行时局部失败和真实 Ubuntu 尚未运行。
