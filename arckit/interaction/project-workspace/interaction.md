# Project 工作空间 - 交互规范

## 交互策略

### 核心任务

用户在同一 Project 工作空间完成立项、责任和资源装配，并按独立能力查看 Project 范围的资料。建立普通 Project 时，用户可以从 Party Directory 选择一个 active 主体作为客户，也可以明确选择“不关联客户主体”完成 Base 立项。选择后，Project 只保存 Party ID、来源 revision、类型和法定名称快照；当前 Party 档案继续由 Party Directory 独占。资料连接只保存 Evidence File ID、冻结 revision、关系键和备注；文件元数据、版本、字节和下载授权继续由 Evidence Service 独占。

### 用户场景

- 用户建立尚未明确客户的内部 Project，不依赖 Party Directory 也能完成创建。
- 用户建立客户项目，先从 Party Directory 加载 active 主体，再选择一条精确 revision。
- Party Directory 无权限、无结果或暂不可用时，页面说明客户选择不可用，但不阻塞无主体 Base Project。
- Project 创建成功后，详情显示立项时客户快照；Party 后续改名、停用或故障不改写该快照。
- 首个切片不允许创建后改绑或解绑客户，页面必须明确该限制。
- 用户在“资源连接”中选择 Evidence 资料，输入 File ID、冻结 revision 和业务关系键，并在 Project revision 成功推进后进入“项目资料”读取元数据。
- 用户只具备 `project-workspace.materials.read` 时可以查看文件名、版本、摘要、媒体类型与来源状态，但不会自动获得下载能力。
- 用户点击可用资料的下载入口时，页面先切换到 Evidence Service 的独立授权链；拒绝下载不会撤销或扩大 Project 的资料读取能力。
- Evidence revision 变化、权限撤销或服务不可用时，页面保留 Project 和其余分段，并提供重新读取、刷新权限或返回工作台的恢复路径。

### 主路径

用户从统一工作台进入 Project → 新建项目 → 页面默认选择“不关联客户主体” → 若用户需要客户，页面通过 Gateway 为当前 IAM Session 请求 Party Directory 用户凭证 → 读取 active Party 列表 → 用户选择带 revision 的主体 → 提交 Project 创建命令 → Project 服务使用自己的工作负载身份再次精确读取并验证 Party → 在本地事务中保存最小快照 → 详情显示快照。需要装入项目资料时，用户在“资源连接”选择 Evidence → 提交精确 File ID／revision → Project 保存 Resource Link 并推进自己的 revision → 用户进入“项目资料” → Project 以 metadata-only 工作负载凭证读取各文件元数据 → 页面逐段显示可用、来源已更新、禁止、失效或不可用 → 用户点击可用资料时再由 Gateway 为 Evidence Service 单独授权并进入 Evidence 下载流程。

### 决策点

- 浏览器读取 Party 列表所需的用户凭证与 Project 服务端精确验证所需的工作负载凭证是两条独立授权链。
- 浏览器列表只改善选择体验，不是可信验证；Project 必须重新验证 exact Party ID／revision／tenant／active 状态。
- 客户主体是可选直接依赖。Party 故障只能阻断带客户的新建命令，不能阻断无主体新建或既有 Project 读取。
- 页面不提供改绑／解绑入口；该语义必须等真实业务规则明确后新增命令、revision 和历史事件。
- Project 详情展示保存的快照，不在每次读取时在线拼接 Party 当前名称。
- “资源连接”写动作与“项目资料”读动作是两个独立 Project Capability；没有 `resource-link.manage` 的用户可以只读资料，没有 `materials.read` 的用户看不到资料页签。
- Project workload 只获得 Evidence `metadata.read`；页面中的下载动作必须进入 Evidence Service 自己的 audience、Capability 与精确 File Scope，不把 Project Credential 当作下载凭证。
- 单个 Evidence 分段失败只改变该分段状态；Project 页面、其他资料和其他业务页签保持可用，不把失败或未知伪装为空集合。

### 状态流

- 新建默认态：主体选择为“无”，Base 立项可直接提交。
- 主体加载成功：下拉显示 active Party 名称、类型和 revision。
- 主体目录空：保留“无”选项，提示可以先去 Party Directory 登记或继续 Base 立项。
- 主体目录失败：保留已填 Project 字段与“无”选项，说明失败不代表已创建。
- 创建成功：关闭对话框，打开新 Project，显示客户快照或“未选择”。
- 服务端验证失败：保留表单；过期／inactive 返回业务错误，服务不可用返回 503，不产生 Project。
- 资料未连接：项目资料页显示空状态并引导到“资源连接”，不自动创建 Link。
- 资料已读取：逐段显示文件名、版本、SHA-256 摘要、媒体类型、冻结／当前 revision 和观察时间。
- 资料来源变化：分段标记“来源已更新”，保留冻结 revision，并要求用户重新判断是否调整 Link。
- Evidence 不可用：分段标记“来源暂不可用”，页面保持 200 和可重试状态。
- Project 资料权限撤销：资料页签和写入口按最新 `allowed_actions` 收敛，当前编辑态退出；刷新权限不恢复已撤销动作。
- Evidence 下载拒绝：留在 Project 页面并展示独立授权失败，不清空 Project 资料或误报 Project 权限丢失。

### 输入输出边界

浏览器提交可选 `client_party={party_id,source_revision}`，不提交名称、类型、登记标识、Tenant 或 Actor。Project 服务通过版本化 reference API 取得可信投影，只持久化 ID、revision、kind 与 legal name。页面不读取 Party 内部表，不复制主体管理表单，也不把客户角色写回 Party。资料 Link 命令只提交 Project expected revision、File ID、可选来源 revision、关系键和幂等 Command ID；Project 页面不接收 Bucket、Object Key、长期密钥或下载 URL。资料列表只消费 Project 的分段投影；下载数据面在 Evidence 页面完成。

## 页面状态

| 状态 | 触发条件 | 关键控件 |
| --- | --- | --- |
| Base 默认态 | 打开新建 Project | Project 字段、“不选择客户主体”、边界说明 |
| 主体目录已加载 | 用户选择加载且授权／读取成功 | active Party 下拉、名称／类型／revision |
| 主体目录空或失败 | 结果为空、403、503 或无法解析 | 保留输入、“无”选项、重载与 Party 入口说明 |
| 创建成功 | 服务端精确验证并提交成功 | Project 身份、revision 1、客户快照或未选择 |
| 验证拒绝 | revision 过期、inactive、跨租户或依赖故障 | 零写入说明、保留输入、重新加载主体 |
| 资料空状态 | Project 没有 active Evidence Link | 空状态、前往资源连接 |
| 资料可用 | metadata-only 组合完成 | 文件摘要、revision、来源状态、独立下载入口 |
| 资料来源变化／不可用 | revision 漂移、403、无效引用或 503 | 逐段状态、冻结值、重新读取和来源恢复 |
| 资料权限变化 | Project `materials.read` 被撤销或 Credential 过期 | 页签收敛、退出资料态、刷新权限／返回工作台 |

## 导航流程

### 进入方式

用户从 Project 列表点击“新建项目”。Party Directory 是否装入由 Suite Manifest 决定；本地直连模式和未装入 Party 的组合只显示可独立完成的 Base 立项。

### 页面内导航

“加载主体目录”只更新新建对话框中的下拉与反馈，不离开 Project 页面。用户需要维护主体时从统一工作台进入 Party Directory；返回 Project 后重新加载，不能沿用旧 revision 静默提交。“资源连接”负责建立 Evidence Link，“项目资料”负责读取组合结果。两者是同一 Project 页面中的不同页签；建立成功后保留当前 Project 并重新载入最新 revision。点击下载入口离开 Project 前先完成 Evidence 应用授权，失败则停留在当前页。

### 离开方式

取消对话框不保存任何事实。创建成功后进入 Project 详情；详情显示立项快照，但没有客户改绑或解绑动作。

## 交互行为

### 加载主体目录

- 每次打开新建对话框默认清空旧 Party 选项，并保留“不选择客户主体”。
- 正式 Suite 先调用 `/gateway/authorize/party-directory`，再读取 `/party/api/v1/parties?status=active&limit=100`。
- 选项显示主体名称、类型和 revision；提交值只包含 ID 与 revision。
- 读取失败不清空 Project 基础字段，不禁用 Base 创建按钮。

### 创建 Project

- 未选择主体时不发送 `client_party` 字段。
- 选择主体时发送精确 ID 与 option 上携带的来源 revision。
- 服务端错误由 Project 原样投影；客户端不把 Party 列表当作最终授权。
- 成功后重置表单、刷新 Project 列表并打开新记录。

### 详情快照

- 有客户快照时显示 legal name 与 source revision。
- 无客户快照时显示“未选择（Base 可选）”。
- Party 当前名称变化不触发详情自动替换；这不是缓存过期，而是历史解释所需的 Project 自有快照。

### 建立与读取项目资料

- 目标应用选择 Evidence 时，资源类型固定为 `evidence-file`，默认关系键为 `project-material`。
- 建立 Link 使用 Project 当前 expected revision；409 时重新载入 Project，用户确认后重新提交，不盲目重放旧 revision。
- 资料页只在 `allowed_actions` 包含 `project-workspace.materials.read` 时显示，切入页签后按需加载。
- 返回的每个分段独立展示状态。空集合与来源失败是不同状态；来源失败不删除 Link。
- Project 页面不展示 Bucket、Object Key 或 Access Grant，不以资料读取权替代下载权。

### 进入 Evidence 下载

- 只有 `available` 分段显示“前往 Evidence 获取下载授权”。
- 点击后调用统一入口的 Evidence 授权入口，再打开该 File 的 Evidence 页面。
- Evidence 授权拒绝只在当前页面显示下载失败；Project 的资料列表和其他动作保持原样。
- 本地直连模式没有统一入口换证时不显示下载按钮，只保留元数据验证能力。

## 错误处理

| 错误类型 | 展示方式 | 恢复操作 |
| --- | --- | --- |
| Party 用户授权 401／403 | 主体选择区说明无权读取 | 继续无主体立项或返回工作台获取权限 |
| 主体列表 503 | 主体选择区说明目录暂不可用 | 保留 Project 输入，重载或继续 Base 立项 |
| 精确 revision／状态拒绝 | Project 顶部错误，声明未创建 | 重新加载 Party 列表并由用户重新选择 |
| Project→Party 503 | Project 顶部依赖错误，声明零写入 | Party 恢复后使用同一幂等命令或重新发起 |
| Project 创建 401／403 | 退出创建态并收敛 Project 动作 | 刷新 Project 权限或返回工作台 |
| Project 资料读取 401／403 | 资料页签收敛，页面退出资料态 | 刷新 Project 权限或返回工作台 |
| Evidence metadata 403／无效引用 | 对应分段显示禁止／失效，不删除 Link | 由管理员修复精确 File 权限或由用户调整 Link |
| Evidence metadata 503 | 对应分段显示来源暂不可用 | Evidence 恢复后重新读取 |
| Evidence 下载授权 401／403 | Project 页顶部显示独立下载授权失败 | 保留资料列表，返回工作台申请 Evidence 下载能力 |

## 加载策略

Party 列表、经营链路和项目资料均按页签按需加载，不作为离线可信缓存。Project 详情只读取自己的数据库快照；因此 Party 或 Evidence 停机时既有 Project 页面仍可打开。资料读取保留一次请求内的逐段观察时间与耗时，重新读取会整体替换当前页面投影，不把旧成功结果混入新失败结果。
