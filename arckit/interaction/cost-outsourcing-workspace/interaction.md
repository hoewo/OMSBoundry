# 成本与外协工作区 - 交互规范

## 交互策略

### 核心任务

用户在 Project 范围内登记已经发生的成本或外协委托，冻结 Project、服务方 Party、可选 Contract 与 Evidence 的精确版本，通过追加 revision 完成纠正、撤回和外协评估，并从精确来源 revision 提出由 Money 独立决定的应付建议。页面不创建 Project／Party／Contract，不上传文件字节，也不直接生成或决定 Money 应付款。

### 主路径

统一工作台或独立入口进入 → Gateway／IAM 签发 `cost-outsourcing` 短期凭证 → 输入 Project ID → 切换“成本录入／外协委托／应付建议” → 读取记录、金额可见性与 `allowed_actions` → 登记或选择记录 → 查看精确快照 → 追加纠正／撤回／评估，或从精确来源提出建议 → Money 独立决定 → 用户显式同步目标决定 → 重读当前 revision 与历史。

### 决策点

- 页面只根据 API 返回的 `allowed_actions` 显示动作，不根据岗位名、按钮位置或前端缓存自行授权。
- 金额为空表示当前凭证没有金额读取能力，不显示为 `0`；金额为 `0` 是明确业务值。
- 成本与外协是同一应用内两个独立聚合；切换页签保留 Project，但不混合列表、revision 或命令。
- 应付建议必须选择精确 Cost Entry／Engagement ID 和 revision；页面不从名称、列表位置或当前金额猜来源。
- `target_unavailable` 表示建议已在 C6 留存但尚未证明 Money 接收；`proposed` 表示 Money 已接收并等待独立决定。
- `accepted` 只显示 Money 返回的 Obligation ID／revision；`rejected` 明确不生成 Obligation。页面不能代替 Money 接受或拒绝。
- 外协服务方必须使用 Party ID／revision；可选合同还必须精确关联当前 Project 与服务方，页面不能只靠名称自动匹配。
- Evidence 只提交 File ID、Version 与槽位；页面不读取或持久化文件字节。
- 纠正、撤回和评估使用当前 revision；409 后重读并让用户重新决定，不自动重放旧意图。
- 422 表示精确引用失效；503 表示上游暂不可验证。两者都必须明确“本次没有写入”。
- 401／403 立即关闭旧编辑能力并要求经 Gateway／IAM 刷新；刷新后重新读取 `allowed_actions`。

### 信息揭示

页面先显示 Project、当前模式、列表、状态、金额和 revision；选中记录后显示服务端允许动作、冻结引用、Evidence 和历史。稳定 ID 与摘要并存：业务人员先读名称和内容，诊断时仍能复制 ID／revision。

### 输入输出边界

输入只包含 C6 命令字段，以及 Project／Party／Contract／Evidence 的稳定 ID 与精确 revision。输出包含 Cost Entry、Outsourcing Engagement、Assessment、Payable Suggestion、允许动作与追加历史；Money 只通过建议目标状态和可选 Obligation 引用出现。Billing／C7 不在本页面触发，Money 也不是 Cost／Engagement 登记组合的启动前提。

## 页面状态

| 状态 | 触发条件 | 关键控件 |
| --- | --- | --- |
| 已就绪 | 当前模式列表读取成功 | Project、模式页签、记录列表、详情、允许动作 |
| 真实空状态 | 读取成功且当前模式无记录 | 边界说明、按权限显示的首个登记动作 |
| 登记／纠正 | `create`／`correct` 与业务状态允许 | 金额、业务字段、精确引用、Evidence、原因 |
| 金额与 Evidence 权限变化 | 刷新后能力增减 | 脱敏金额、收敛后的 Evidence／纠正入口 |
| 提交成功 | 命令成功或同一幂等意图复用 | 新 revision、重读后的动作、历史入口 |
| 建议已送达 | Money 返回 pending | 精确来源、双方 Party 快照、金额、due date、`proposed` |
| 建议目标不可用 | C6 已留存但 Money 未确认接收 | `target_unavailable`、输入保留、显式同步 |
| Money 已决定 | 同步得到 accepted／rejected | 目标 revision、决定理由、可选 Obligation 引用 |
| 冲突／引用／依赖失败 | 409／422／503 | 零写入说明、保留输入、重读／核对／显式重试 |
| 未认证／被拒绝 | 401／403 | 只读上下文、刷新凭证、返回工作台 |

## 页面内导航

左侧“使用说明／业务记录”切换产品说明和操作区。“成本录入／外协委托／应付建议”只切换同一 Project 下的聚合或交接视图，不改变事实所有权。选择列表项后右侧显示详情与动作；历史按需加载，不阻塞当前列表。离开页面前未提交表单只存在浏览器内，不在服务端形成草稿。

## 交互行为

### 成本录入

- 新建要求 Project revision、成本类型、日期、币种、整数最小货币单位、说明；车辆成本额外要求正数里程。
- Evidence 能力存在时可以登记一个或多个精确文件版本；没有能力时入口隐藏，服务端仍复验。
- 纠正提交完整的新当前值和原因；撤回只改变状态并保留历史。

### 外协委托

- 新建要求 Project revision、服务方 Party ID／revision、范围、币种和整数金额。
- 可选 Contract 必须携带 application、resource type、resource ID 与 revision，并由服务端复验 Project、Party、币种和金额边界。
- 评估是追加事实，不覆盖外协委托；五分制评估提交当前 Engagement revision 与上一 Assessment revision。

### 应付建议

- 新建要求来源类型、来源 ID／revision、账簿所有者 Party ID／revision、到期日和用途；Cost Entry 还要求单独选择交易对方 Party。
- 页面显示来源 digest、双方 Party 快照、精确金额和目标 revision，便于确认 Money 接收的事实与 C6 当前事实一致。
- `target_unavailable` 允许在 Money 恢复后显式同步；同步失败不改变 C6 revision，恢复后新命令或相同已成功命令保持幂等。
- `proposed` 可以同步 Money 当前决定或在目标仍 pending 时撤回；accepted／rejected 是目标决定，只能读取和查看历史。
- accepted 显示 Obligation ID／revision 并提供目标下钻语义；rejected 显示决定理由且不呈现虚构义务。

### 权限刷新

- 首次进入没有凭证时打开访问说明；本地 Token 只保存在当前标签页。
- 正式环境由 Gateway 从 IAM Session 换取目标 audience 凭证；刷新后重读列表和当前详情。
- 金额能力撤销后金额立即显示“金额未授权”；Evidence／纠正／评估能力撤销后对应入口消失，旧表单不可继续提交。

## 错误处理

| 错误 | 页面含义 | 恢复动作 |
| --- | --- | --- |
| 401 | 凭证无效或过期 | 清除旧动作，经 Gateway／IAM 重新授权 |
| 403 | Capability 或 Project／资源 Scope 不允许 | 保留 Project 上下文，显示只读或返回工作台 |
| 409 | revision、状态或 Command payload 冲突 | 重读最新事实，不自动重放旧命令 |
| 422 | Project／Party／Contract／Evidence 精确引用失效 | 返回来源应用核对 ID 与 revision |
| 503 | 引用依赖或 Money 同步暂不可用 | 引用命令明确零写入；已留存建议保持当前 revision，恢复后显式同步 |

## 加载与反馈

首次加载保留页面框架；模式切换和刷新期间不把故障显示为空列表。提交只锁定当前动作，成功后同时重读列表、选中详情与 `allowed_actions`。成功反馈包含新 revision；错误反馈包含是否写入、失败类别和下一步。

## 线框投影要求

`default.html` 分别投影已就绪、真实空状态、登记／纠正、建议提交与目标不可用、Money 接受／拒绝、权限变化、提交成功和冲突／依赖失败八类状态。所有画布使用桌面设备框和共享灰度线框样式，关键节点以 `data-kit` 标注。
