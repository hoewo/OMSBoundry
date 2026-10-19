# 生产交付工作区 - 交互规范

## 交互策略

### 核心任务

用户在一个工作区内选择指定类型的 Production Work Item，核对 Project 与 revision，装配生产负责人和 Evidence 成果，按不可变类型定义推进阶段，并通过历史解释每次变化。

### 用户场景

- 生产操作者从统一工作台进入，查看当前 Credential Scope 内的设计或咨询工作项。
- 有创建能力的操作者以 Project ID、来源 revision 和生产类型建立工作项。
- 有负责人、成果或阶段能力的操作者只看到对应表单和按钮。
- Grant、Project Assignment 或账号状态变化后，用户需要明确知道旧动作已经失效。
- revision 冲突、引用失效或依赖不可用时，用户需要知道本次操作没有写入，并能重新载入或返回上游核对。

### 主路径

用户通过统一工作台进入 → Gateway 使用 IAM Session 签发 Production 短期凭证 → 页面读取列表级 `allowed_actions` → 用户选择工作项 → 页面读取详情级 `allowed_actions`、阶段和成果 → 用户提交一个带 `command_id` 与 `expected_revision` 的动作 → 页面重新读取列表、详情和允许动作 → 用户在历史中核对新 revision。

### 决策点

- 页面只根据 Production API 返回的 `allowed_actions` 呈现写入口，不读取客户岗位名，不解析 HttpOnly Credential。
- Project Assignment 策略入口携带 Project Scope、Assignment ID 与 revision；租户级直接 Grant 入口不伪造 Assignment。
- `not-applicable` 是显式业务动作，必须填写原因；空成果或隐藏阶段不代表不适用。
- 已签 Credential 的最长残余窗口不改变刷新语义；用户显式刷新后必须使用新 Credential 和新 `allowed_actions`。
- 401／403 立即退出提交状态、关闭表单并清除旧动作；重新授权失败时返回统一工作台或登录。
- 409 不自动重放旧 revision；422／503 不清空已读取的业务投影，也不显示为成功。

### 信息揭示

左侧先显示 Scope 内工作项的类型、状态、Project 和 revision。右侧先显示工作项身份、当前阶段、服务端允许动作，再按“阶段与成果、负责人、历史”展开。外部引用只显示稳定 ID、Version 与来源 revision。

### 状态流

- 已就绪：列表和详情读取成功，动作按当前 Credential 与业务状态收敛。
- 建立工作项：有 create 能力时输入类型、Project ID 与来源 revision。
- 推进生产：按详情动作设置负责人、登记成果、调整适用性、完成或重开阶段。
- 提交成功：重新读取新 revision、阶段状态和允许动作。
- 权限变化恢复：撤权或停用后退出提交状态，通过 Gateway 重新授权；失败则返回工作台／登录。
- 冲突与依赖失败恢复：409 读取最新 revision；422／503 保留当前投影并提示核对来源或稍后重试。

### 反馈与恢复

加载与提交反馈固定显示在工作区顶部。提交期间不重复发送。成功反馈包含最新 revision。401／403 清空页面缓存的写动作；权限刷新成功后重新读取列表与当前详情，刷新失败不继续显示旧表单。409 明确“没有写入”并载入最新状态。422／503 保留选中记录和输入上下文，用户重新确认后产生新的 `command_id`。

### 输入输出边界

页面只提交 Production 命令字段，以及 Project、Assignment、Evidence 的稳定 ID、Version 与 revision。页面不编辑 Project、不管理人员、不上传文件字节。输出包含 Work Item、类型版本、Stage、Deliverable 引用、`allowed_actions`、revision 和不可变历史。

### 线框投影要求

`default.html` 分别投影“已就绪”“建立工作项”“推进生产”“提交成功”“权限变化恢复”“冲突与依赖失败恢复”。每个状态都保留 Work Item／Project 身份或明确的尚未建立状态。

## 页面状态

| 状态 | 触发条件 | 关键控件 |
| --- | --- | --- |
| 已就绪 | 列表与详情读取成功 | 工作项列表、类型、状态、revision、allowed_actions、三页签 |
| 建立工作项 | 列表动作包含 create | 类型、Project ID、来源 revision、确认 |
| 推进生产 | 详情动作允许负责人、成果或阶段操作 | Assignment 引用、Evidence 引用、完成／重开／不适用 |
| 提交成功 | 命令成功或幂等复用 | 最新状态、revision、阶段和动作 |
| 权限变化恢复 | 重新授权后动作变化，或收到 401／403 | 只读摘要、重新授权、工作台／登录 |
| 冲突与依赖失败恢复 | 收到 409／422／503 | 未写入说明、最新状态、来源核对、显式重试 |

## 导航流程

### 进入方式

租户级操作者从统一工作台进入。Project Assignment 操作者从带 Project 与 Assignment 事实参数的上下文入口进入。正式组合由 Gateway 设置 HttpOnly Credential；本地模板环境可以在当前标签页设置测试 Token。

### 页面内导航

左侧工作项列表切换当前记录。右侧“阶段与成果、负责人、历史”页签不改变路由，不丢失当前 Work Item 与 revision。

### 离开方式

用户可以返回统一工作台或关闭页面。重新授权失败时页面不保留可提交表单，明确引导返回工作台或登录。

## 交互行为

### 工作项列表

- 列表只显示当前 Credential Scope 内的记录。
- 新建入口只在列表 `allowed_actions` 包含 `production-work-item.create` 时显示。
- 选择记录后读取详情并以详情 `allowed_actions` 重绘所有写入口。

### 阶段与成果

- 只有 `stage.manage` 才显示完成、重开、设为不适用与恢复适用。
- 只有 `deliverable.manage` 且存在 ready Stage 才显示成果登记表单。
- 设为不适用和重开必须填写原因；成果登记只提交 Evidence 稳定引用。

### 负责人和取消

- 只有 `owner.manage` 显示负责人表单；服务端复验同 Project 的 active `production_owner` Assignment。
- 只有 `cancel` 显示取消入口；取消后记录和历史保留，写动作按服务端状态收敛。

### 权限刷新

- 页面把当前入口的租户或 Project／Assignment 参数提交给 Gateway，不自行签发凭证。
- 刷新成功后重新读取列表与详情；撤销的动作不沿用旧缓存。
- 刷新收到 401／403 时退出提交状态，关闭所有对话框并显示恢复入口。

## 错误处理

| 错误类型 | 展示方式 | 恢复操作 |
| --- | --- | --- |
| 401／403 | 顶部权限恢复条，所有写入口隐藏 | 通过 Gateway 重新授权；失败则返回工作台／登录 |
| 409 revision／状态冲突 | 顶部说明本次没有写入 | 载入最新 revision，由用户重新决定 |
| 422 引用或阶段无效 | 当前动作附近说明 | 返回 Project／People／Evidence 核对稳定引用 |
| 503 依赖不可用 | 保留当前详情和输入上下文 | 依赖恢复后由用户发起新命令重试 |
| 无法解析响应 | 页面级错误 | 返回统一工作台或检查入口路由 |

## 加载策略

首次加载保留页面框架。刷新列表不清空已选详情；详情刷新保留 Work Item 身份。提交只锁定当前动作，完成后重新读取列表、详情和 `allowed_actions`。
