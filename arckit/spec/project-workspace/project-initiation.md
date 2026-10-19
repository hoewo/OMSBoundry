# Project Workspace：项目立项

## 元信息

- 路径：`spec/project-workspace/project-initiation.md`
- 所属模块：`project-workspace`
- 状态：🟢 已实现
- 优先级：P0
- 负责人：BizFoundry
- 上层旅程：[参考 OA Demo](../assembly/reference-oa-demo.md) 的 J1
- 关联模型：`tech/_shared/models/Project.yaml`、`ProjectAssignment.yaml`、`ProjectResourceLink.yaml`、`ProjectHistoryEvent.yaml`
- 关联 API：`tech/_shared/contracts/project-*.yaml`
- 关联技术：`tech/project-workspace/solution.md`
- 实施状态：模型✅ | API✅ | 设计✅ | 技术✅ | 开发✅
- 变更历史：
  - 2026-10-02：BF-084 增加有界立项门槛与原子初始派发；完整仿 OA 前端／公网验收仍待独立完成，不取代早期 J1 通过证据。
  - 2026-09-25：固定 J1 最小业务结果、边界、状态、不变量与验收矩阵。
  - 2026-09-25：独立 Go 服务、PostgreSQL、HTTP、模板／正式 IAM、外部引用校验、Docker 和 J1-A01～J1-A18 证据通过；完整入口与页面装配仍后续验证。
  - 2026-09-26：把 Assignment 管理拆为通用管理和三项稳定角色能力，并在正式 D1 完成 C1 多角色、多 Project、撤权和状态拒绝矩阵。
  - 2026-09-26：实现 Project Package、两级父子关系、既有项目 attach／wrap 转换与安全撤销；保留原 Project ID、Assignment、Resource Link 和历史，合同金额分配仍保持未知且不进入 Base。
  - 2026-09-27：增加 Project／Package 客户工作区、服务端 `allowed_actions` 投影、显式 IAM 权限刷新和撤权后的只读恢复。
  - 2026-09-27：以参考 OA 的“项目立项选择客户”为首个 Party 消费场景；普通 Project 可选精确 Party revision 并冻结最小客户快照，Party 当前档案仍由 Party Directory 独占。

## 问题与结果

**问题 A**：项目资料和后续工作开始前，没有稳定项目身份、责任范围和可访问历史，合同、资金、人员与文件容易再次被复制到同一个大表中。

**可验证结果 B**：有权限的用户建立一个项目，补全最小资料，为 Person 或 Group 建立有期限的项目 Assignment，完成立项后所有变更都有可追溯历史；Project 只保存稳定引用，不接管 People、Contract、Money Flow 或 Evidence 的事实。

## J1 最小范围

### Project 拥有

- 租户内稳定 Project ID；
- 租户内不区分大小写唯一的项目编号；
- 名称、摘要、计划起止日期；
- 一个或多个稳定项目类型 key；类型只表示 Project 选择的业务方向，具体类型定义和阶段规则由 Production Operations 拥有；
- `draft`、`pending_completion`、`active`、`cancelled` 状态；
- Project revision、可信 Actor、命令幂等记录和不可变历史；
- 项目中的 Assignment；
- 指向外部应用资源的类型化 Resource Link。
- `project`／`package` 节点类型、最多两级的父子关系和稳定排序；Package 可暂不编号，普通／子 Project 必须使用正式唯一编号；
- 既有 Project 转入 Package 的不可变转换记录，以及满足安全条件时的显式撤销。
- 可选的客户 Party 精确引用，以及选择发生时的 `kind`／`legal_name` 最小不可变快照。

### Project 只引用

- People & Groups 的 Person ID、Group ID 和来源 revision；
- Party Directory 的 Party ID、来源 revision 和当前有效性；
- Contract、Money Flow、Evidence 与设计运营扩展的 Resource ID；
- IAM Subject 和 Capability Credential 中已经验证的 Actor。

### Project 不拥有

- 人员姓名、账号、密码、组织树、Group 成员名册或 IAM 角色；
- Party 当前档案、登记标识、联系方式、搜索目录或主体合并判断；客户 Party 在项目中的选择和当时快照属于 Project，当前主体资料属于 Party Directory；
- 合同条款、合同金额、合同 revision、结算事实；
- 应收应付、实际资金、核销、余额或发票；
- 文件字节、对象键、STS、签名 URL 或文件摘要真相；
- 设计阶段、成果要求、批复审计、奖金、成本、外委和客户专用公式；
- 通用任务、审批、通知或任意 JSON 扩展平台。

## 状态与不变量

### Project 状态

| 当前状态 | 允许动作 | 进入条件 |
| --- | --- | --- |
| `draft` | 修改基础资料、增加／结束 Assignment、增加／解除 Resource Link、请求完成立项、取消 | 创建成功 |
| `pending_completion` | 修正基础资料或 Assignment、重新请求完成立项、取消 | 完成立项校验发现可补全项 |
| `active` | 查看、增加／结束 Assignment、增加／解除 Resource Link、取消 | 满足所选版本化门槛；默认双负责人，business-owner-v1 允许生产负责人后补，Package 编号可选；普通项目编号、名称、类型和经营负责人仍必需 |
| `cancelled` | 只读查看和历史查询 | 明确取消命令成功 |

- `active` 和 `cancelled` 不物理删除；取消必须记录原因。
- 基础资料、Assignment 和 Resource Link 的命令都消费 `expected_revision`；冲突返回当前 revision，不做最后写入者覆盖。
- 创建、修改、立项、取消、Assignment 和 Resource Link 命令都要求 `command_id`，相同 ID 与相同内容重放返回原结果；同 ID 不同内容必须拒绝。
- 每个成功写命令与对应 History Event 在同一数据库事务内提交。
- 一个项目同一时刻只能有一个 active `business_owner` 和一个 active `production_owner`；`participant` 可以有多个。
- Assignment 结束后可以重新建立新 Assignment；历史记录不改写旧记录。
- `project_type_keys` 是类型化扩展的稳定 key，不允许把设计阶段字段塞入 Project。
- Resource Link 的解除只结束关联，不删除或修改目标资源。
- Package 只能是顶层，Project 最多属于一个 Package；不允许 Package 再成为子项或形成第三层。
- `attach_existing` 与 `wrap_as_package` 均保留原 Project ID、编号、Assignment、Resource Link、外部 URL 语义和历史，不迁移或复制目标应用事实。
- 自动创建的外层 Package 只有在仍为 `draft`、仍只有原首个子 Project、revision 未被后续命令推进且没有 Assignment／Resource Link 时才允许自动撤销；不满足时必须拒绝并保持原关系不变。
- Package 不拥有合同金额向子项目的分配、资金汇总或生产事实；这些未知规则在取得业务证据前不得进入 Base。
- 客户 Party 是可选的直接依赖：未选择时 Project 仍可独立创建；选择时必须精确验证 ID、tenant、active 状态和 revision，验证失败或 Party 不可用时整笔创建失败且零写入。
- 已创建 Project 的客户快照不随 Party 改名、停用或 revision 推进而改写；首个切片不提供重选／解除，必须等待明确业务语义后以新命令和历史事件增加，不能借基础资料更新静默覆盖。

## 用户场景

### US-1：创建项目草稿 [P0]

**角色**：拥有 `project-workspace.create` 的用户。

**场景**：

- **Given** 用户具有租户级创建能力，且项目编号尚未使用；
- **When** 用户提交项目编号、名称、摘要、计划日期和至少一个项目类型；
- **Then** 系统创建 `draft` Project、revision 1 和 `project_created` 历史事件。

**验收条件**：

- [x] 同一 `command_id` 重试只得到同一 Project；
- [x] 同租户编号大小写冲突返回 `409 PROJECT_NUMBER_CONFLICT`；
- [x] 其他租户可以使用相同编号；
- [x] 缺少身份返回 401，能力不足返回 403。

### US-2：建立项目责任 [P0]

**角色**：拥有通用 `project-workspace.assignment.manage`，或只拥有与目标 Assignment 类型匹配的 `project-workspace.assignment.business-owner.manage`、`project-workspace.assignment.production-owner.manage`、`project-workspace.assignment.participant.manage` 的用户。

**场景**：

- **Given** Project 尚未取消，People & Groups 可以提供有效 Person／Group 引用；
- **When** 用户分别建立经营负责人、生产负责人和参与人 Assignment；
- **Then** Project 保存稳定引用、来源 revision、生效时间和项目角色，不复制人员名册。

**验收条件**：

- [x] 不存在、跨租户、已归档或类型不匹配的引用被拒绝且不产生 Assignment；
- [x] People 暂时不可用时返回 503 且可用同一 `command_id` 重试；
- [x] 第二个 active `business_owner`／`production_owner` 被拒绝；
- [x] 更换负责人通过结束旧 Assignment 后新增记录，旧历史仍可读。
- [x] 角色专用能力只能建立或结束匹配的 Assignment；经营经理不能管理生产负责人，生产经理不能管理经营负责人；拒绝不推进 Project revision。

### US-3：完成立项 [P0]

**角色**：拥有 `project-workspace.initiation.complete` 的用户。

**场景**：

- **Given** Project 是 `draft` 或 `pending_completion`；
- **When** 用户请求完成立项；
- **Then** 服务端检查所选版本化立项门槛（省略保持双负责人，business-owner-v1 允许生产负责人后补），成功时进入 `active`，否则返回逐项缺口并进入或保持 `pending_completion`；Project 不判断 Production 的阶段定义，未注册或未选择的生产类型由 Production 写入口失败关闭。

### 初始派发增量（BF-084）

A→B：原 OA 在派发时只需名称及经营负责人，现有严格 Create 提前索要正式资料 → 新 `InitiationDispatch v1` 在同一事务中保存 Project draft 与经营 Assignment，正式编号和类型在完成时后补，保持稳定 Project ID。

相应创建／层级能力和经营分工能力必须同时具备，Person 引用由 People 精确验证；已成功命令可离线重放。000010 不改历史事实，仅放宽未激活草稿的空编号／类型并保持激活约束。原 Create API 的输入与幂等语义不变；不生成 DRAFT 编号或假类型。不负责甲方／资质／管理费、任意流程和其他应用事实；这些由各自领域的下一切片对应原 OA 验收。

**验收条件**：

- [x] 缺口响应可以直接映射到可补全字段，不用读取日志猜测；
- [x] 完成立项不创建合同、资金义务、文件或设计阶段；
- [x] 并发使用旧 revision 的请求返回 409；
- [x] 完成后相关人员可通过受限查询看到 Project，其他租户不可见。

### US-4：关联外部资源 [P1]

**角色**：拥有 `project-workspace.resource-link.manage` 的用户。

**场景**：

- **Given** Project 未取消，目标应用返回可验证的稳定 Resource ID；
- **When** 用户建立类型化关联；
- **Then** Project 保存目标应用、资源类型、ID、可选 revision 和关系 key，详情读取时由目标应用提供当前事实。

**验收条件**：

- [x] 关联不复制目标金额、状态或文件技术字段；
- [x] 目标应用不可用时不写入未经验证的新 Link；
- [x] 已存在同一 active Link 的重试幂等；
- [x] 目标应用读取失败时 Project 仍可读取自身事实，并明确显示目标能力不可用。

### US-5：取消并追溯 [P0]

**角色**：拥有 `project-workspace.cancel` 的用户。

**场景**：

- **Given** Project 尚未取消；
- **When** 用户提交取消原因和当前 revision；
- **Then** Project 进入 `cancelled`，后续写动作被拒绝，历史、Assignment 和 Link 仍可查询。

**验收条件**：

- [x] 取消不调用或删除任何外部资源；
- [x] 历史按 revision 与事件序号稳定排序；
- [x] 任意旧 Project 版本都能解释是谁、何时、通过什么命令改变了什么。

### US-6：建立项目包并转换既有项目 [P0]

**角色**：拥有租户级 `project-workspace.hierarchy.manage` 的用户。

**场景**：

- **Given** 一个未取消、尚未属于 Package 的既有 Project；
- **When** 用户把它附加到既有 Package，或创建外层 Package 并把原 Project 作为首个子项；
- **Then** 原 Project ID 和所有已有事实保持不变，只新增两级关系、双方 revision、转换记录和历史事件。

**验收条件**：

- [x] Package 可以不填正式编号，普通／子 Project 仍必须有租户内唯一编号；
- [x] Package 永远是顶层，拒绝第三层、跨租户、已取消节点、旧 revision 和重复 active 转换；
- [x] `attach_existing` 可显式撤销并恢复原顶层位置；安全的 `wrap_as_package` 撤销后保留外层 Package 为已取消历史记录，不物理删除；
- [x] 自动外层 Package 一旦新增子项、Assignment、Resource Link、状态或资料变更，就拒绝自动撤销且不产生部分写入；
- [x] 转换前后的 Assignment、Resource Link、合同／资金／生产引用 ID 均不移动、不复制、不改写；
- [x] Package 子项读取只返回 Project 自有摘要，不同步聚合外部应用金额或状态。

### US-7：立项时选择客户主体 [P0]

**角色**：拥有 `project-workspace.create` 的用户。

**场景**：

- **Given** Party Directory 中存在同租户、active、指定 revision 的组织或个人；
- **When** 用户创建普通 Project 并提交 `party_id + source_revision`；
- **Then** Project 使用精确资源工作负载凭证验证 Party，只保存 ID、revision、类型和法定名称快照，并把它写入 `project_created` 历史。

**验收条件**：

- [x] Party 当前名称后来变化不改写 Project 的创建快照或历史；
- [x] revision 过期、跨租户、inactive、响应来源不符和服务不可用分别失败关闭，且不创建 Project；
- [x] 相同创建 command 重放先读取本地命令结果，Party 已变化或不可用时仍返回原 Project，不再次产生依赖调用；
- [x] 不选择客户时保持原有独立创建能力；Package 不接受客户 Party；
- [x] Project 不保存登记标识、联系方式或 Party 完整档案，不读取 Party 内部表。

## 权限与可见性

第一版 Capability 目录固定为：

- `project-workspace.create`：租户内创建；
- `project-workspace.read`：读取凭证 scope 内的 Project；
- `project-workspace.commercial-summary.read`：读取凭证 scope 内 Project 的合同、资金与开票只读经营组合；不能由普通 Project 查看权隐式获得；
- `project-workspace.update`：更新基础资料；
- `project-workspace.assignment.manage`：管理全部 Assignment，只供显式站点业务管理范围；
- `project-workspace.assignment.business-owner.manage`：只管理 `business_owner`；
- `project-workspace.assignment.production-owner.manage`：只管理 `production_owner`；
- `project-workspace.assignment.participant.manage`：只管理 `participant`；
- `project-workspace.resource-link.manage`：管理 Resource Link；
- `project-workspace.hierarchy.manage`：租户级建立 Package、转换父子关系和安全撤销；因一个命令同时改变 Project 与 Package，资源级凭证不能执行；
- `project-workspace.initiation.complete`：完成立项；
- `project-workspace.cancel`：取消；
- `project-workspace.history.read`：读取历史。

正式 IAM、本地模板 Provider 和测试 Provider 必须产出同形 v1 上下文。单应用本地运行可以不经过 Nginx 和 Gateway，但不能关闭鉴权；目标服务始终验证 tenant、audience、action、scope、有效期和 Project 状态。

客户页面不读取 IAM 角色名，也不把按钮是否可见当作授权。列表与详情必须使用 Project 服务返回的 `allowed_actions`；正式入口的“刷新权限”经 Gateway 使用当前 IAM Session 重新签发短期 Credential。刷新失败或命令返回 401／403 时页面立即退出编辑态，Gateway 清除旧 Project Capability Cookie；重新获得能力前不得保留可提交表单。

## J1 可执行验收矩阵

实现后每行必须对应自动化测试或可重复脚本；当前“计划”不代表已通过。

| ID | 场景 | 输入／故障 | 期望证据 | 层级 |
| --- | --- | --- | --- | --- |
| J1-A01 | 创建草稿 | 合法 command、Project 数据 | 201、revision 1、创建事件 | HTTP＋PostgreSQL 集成 |
| J1-A02 | 创建幂等 | 同 ID 同内容两次 | 同一 Project，无重复事件 | HTTP＋PostgreSQL 集成 |
| J1-A03 | 幂等冲突 | 同 ID 不同内容 | 409，不改数据库 | HTTP＋PostgreSQL 集成 |
| J1-A04 | 编号冲突 | 同租户仅大小写不同 | 409，唯一约束仍成立 | PostgreSQL 集成 |
| J1-A05 | 乐观并发 | 两个命令消费同一 revision | 恰好一个成功，另一个 409 | PostgreSQL 集成 |
| J1-A06 | Assignment | 有效 Person／Group 引用 | active Assignment、来源 revision、历史事件 | HTTP 集成 |
| J1-A07 | 非法引用 | 不存在／跨租户／归档引用 | 422，不写 Assignment | HTTP 集成 |
| J1-A08 | People 故障 | 验证引用时服务不可用 | 503、同 command 可重试、无部分写入 | 系统集成 |
| J1-A09 | 负责人唯一 | 第二个同角色 active Assignment | 409，旧负责人不变 | PostgreSQL 集成 |
| J1-A10 | 完成立项 | 资料、类型、双负责人齐全 | `active`、revision 增加、事件可读 | HTTP 集成 |
| J1-A11 | 缺口立项 | 缺生产负责人 | 422 缺口清单、状态 `pending_completion` | HTTP 集成 |
| J1-A12 | 外部 Link | 合法稳定引用 | 只保存类型化引用，无目标事实副本 | HTTP＋Schema 守卫 |
| J1-A13 | 目标故障 | 验证 Link 时目标不可用 | 503、无虚假成功、Project 可读 | 系统集成 |
| J1-A14 | 取消 | 明确原因和当前 revision | `cancelled`、事实保留、写入被拒绝 | HTTP＋PostgreSQL 集成 |
| J1-A15 | 越权 | 缺凭证／错误 action／跨租户 scope | 401／403，零写入 | 契约＋HTTP 集成 |
| J1-A16 | 历史不可变 | 尝试 UPDATE／DELETE 历史 | 数据库拒绝；查询顺序稳定 | PostgreSQL 集成 |
| J1-A17 | 独立运行 | 关闭 Project Workspace | People、Contract、Money、Evidence 自身测试仍通过 | 组合验收 |
| J1-A18 | 本地入口 | 模板 IAM 直接调用 Project | 与正式 IAM 同形授权结果 | 契约测试 |
| J1-A19 | 创建 Package | 可选编号、合法资料 | `package` 顶层节点、revision 1、创建事件 | HTTP＋PostgreSQL 集成 |
| J1-A20 | 附加既有 Project | 当前 Project／Package revision | 原 Project ID 与事实不变、双 revision 和转换事件 | PostgreSQL 集成 |
| J1-A21 | 外层包裹 | 既有顶层 Project＋新 Package 资料 | 新 Package、原 Project 成为首个子项 | PostgreSQL 集成 |
| J1-A22 | 层级拒绝 | Package 作为子项／第三层／跨租户 | 409、零部分写入 | PostgreSQL 集成 |
| J1-A23 | 转换幂等与并发 | 同 command 重放／旧双 revision | 同一结果；竞争者 409 | PostgreSQL 集成 |
| J1-A24 | 安全撤销 | 未继续使用的转换 | 原 Project 恢复顶层；历史保留 | PostgreSQL 集成 |
| J1-A25 | 不安全撤销 | 外层 Package 已有业务事实或变化 | 409、关系与事实均不变 | PostgreSQL 集成 |
| J1-A26 | 权限边界 | 无 hierarchy 能力／资源级凭证 | 403、用例不执行 | HTTP＋Capability 契约 |
| J1-A27 | 项目包客户页面 | 创建 Package、attach／wrap、查看层级和安全撤销 | 页面只调用公开 API；原 Project ID／事实不变；高风险撤销有原因和恢复反馈 | 页面资源契约＋HTTP＋PostgreSQL 集成 |
| J1-A28 | 权限刷新与撤权恢复 | 正式入口撤销 hierarchy 后重新签发／签发失败 | `allowed_actions` 收敛、写控件消失、旧页面零写入；失败时旧 Cookie 被清除 | Nginx＋Gateway＋IAM＋Project 系统验收 |
| J1-A29 | 选择客户主体 | active 精确 Party ID／revision | revision 1 Project 和最小客户快照；历史同值 | HTTP＋PostgreSQL 集成 |
| J1-A30 | Party 变化后读取 | Party 改名／revision 推进 | Project 创建快照和历史不变 | 系统集成 |
| J1-A31 | Party 失败关闭 | 过期、跨租户、inactive、错误来源或 503 | 422／503；Project 零写入 | HTTP＋PostgreSQL 集成 |
| J1-A32 | 创建命令重放 | 首次成功后 Party 改名或离线 | 返回同一 Project，零新 Party 调用／事件 | 系统集成 |
| J1-A33 | 精确 IAM 凭证 | 创建含 Party 的 Project | 请求 `party-directory.reference.read`，scope 仅目标 Party | 契约＋单元测试 |

### 实际证据（2026-09-25）

- `internal/httpserver/system_integration_test.go` 穿过真实 HTTP、模板 IAM、PostgreSQL 18 与故障可控的 People／Contract Reference API，覆盖 A01～A03、A06～A08、A10～A14；
- `internal/projects/postgres_repository_integration_test.go` 覆盖 A04、A05、A09、A11、A14、A16，以及并发同 command 只形成一份结果；
- `internal/httpserver/server_test.go` 与 `internal/auth/auth_test.go` 覆盖 A15、A18；模板与正式 IAM 产生同形 tenant、subject、action 和 scope 上下文；
- Project Schema 守卫证明不复制合同金额、资金余额、对象键或人员姓名；目标不可用时详情返回自身事实并把 Link 标为 `unavailable`；
- People、Contract、Money 与 Evidence 原有测试在 Project 加入 workspace 后共同通过，覆盖 A17；
- 独立 `go test`／`go vet`／`go build`、PostgreSQL 18 integration、Docker image 构建及容器 ready＋创建 Project 冒烟均实际通过；命令记录在 `docs/testing.md`。

### 首个 Party 消费边界补充证据（2026-09-27）

`000003_project_client_party.sql`、Project 创建服务和 Party Reference Port 已覆盖 J1-A29～A33：HTTP＋PostgreSQL 18 系统集成从精确 Party 投影创建 Project，随后把上游改名并推进 revision，Project 详情与历史仍保留创建时快照；同一 command 重放没有第二次 Party 调用。过期 revision 和上游 503 均返回明确错误且数据库中没有对应 Project。独立契约测试覆盖跨租户、inactive、错误来源，并证明正式模式只请求目标 Party 的 `party-directory.reference.read` 精确资源凭证。

该证据首先确认 `Party Directory → Project Workspace` 的第一个消费关系；后续 D1 专项已经补齐正式 API 入口、精确 IAM 工作负载授权、依赖停机／恢复和联合空目标还原。客户选择 UI 已实现并通过真实 D1 HTTP／故障验收：浏览器以独立 Party 用户凭证发现候选，服务端仍使用精确工作负载凭证复验，且页面保留无主体 Base 立项。重选／解除语义、Contract／Money 两套本地主体迁移和完整双租户矩阵仍需独立验证。

### C1 责任权限补充证据（2026-09-26）

`acceptance/d1-c1-authorization/verify-running` 使用正式 `Nginx → Gateway → IAM／Policy → Project → People` 链，在两个 Project、多个 Package 和九类 Subject 上执行 134 个 HTTP 断言，状态不匹配为 0。它证明：

- 身份超级管理员只有身份治理权时不能进入 Project；OA 业务管理员依靠显式 Project Capability 建立 Project、装配责任并完成立项；
- 经营经理和生产经理只能管理对应负责人 Assignment，跨角色建立或结束均为 `403` 且 Project revision 不变；
- 项目经营负责人和生产负责人通过带 revision 的 Assignment Fact 只得到 Project A 的 `read/update`，Project B 与错误 Fact／Scope 被拒绝；
- 参与者只读，伪造身份 Header 和直接写请求不能增加权限；总经理、财务只读且不能修改；
- `active` 状态下不允许的资料修改由 Project 以 `409` 拒绝，说明 IAM Capability 不能绕过本地状态机；
- Assignment 结束后旧 revision 不能签发新凭证；已签发凭证只保留契约声明的最长五分钟残余窗口；Grant、Policy 签发和撤销均有 IAM 审计。
- 总经理没有结构治理能力，OA 业务管理员通过显式 `project-workspace.hierarchy.manage` 完成 Package 创建、attach／wrap、幂等重放和安全撤销；旧双 revision、第三层和已继续使用的自动 Package 撤销均失败关闭且不产生部分写入。

客户站点角色只存在于版本化 Overlay；Project 只认识上述稳定 Capability 和三个稳定 Assignment role key。该结果完成 C1 的责任权限与 Project Package Base 子矩阵，不代表 Package 客户页面／金额分摊、跨租户或 C2～C8 已完成。

### C1 项目包与父子转换补充证据（2026-09-26）

Project Workspace migration `000002_project_package_hierarchy.sql`、领域服务和 PostgreSQL 18 集成验收已覆盖 J1-A19～J1-A26 的本地事务与 HTTP 权限部分：两级约束由服务与数据库共同守卫；attach／wrap 保存不可变转换记录；安全撤销恢复原 Project，自动外层 Package 以取消记录留存；已有 Resource Link 在转换与撤销后 ID 不变；外层 Package 增加 Evidence Link 后撤销返回冲突且没有部分写入。普通测试与 `-race -tags=integration` 均通过。

该证据补齐 C1 的 Base 层级事实、恢复机制和 Nginx → Gateway → IAM 正式入口；J1-A27～A28 的页面与撤权恢复证据见 2026-09-27 补充验收。它仍不证明跨租户运行矩阵、项目包合同金额分配或 C2～C8。金额分配保持显式未知项，阶段总判定保持 `insufficient-evidence`。

### C1 项目包页面与权限变化补充证据（2026-09-27）

Project 列表和详情现在由服务端返回凭证 action、scope 与当前状态共同收敛后的 `allowed_actions`，详情同时返回可安全撤销所需的 active Package Conversion。客户页面只调用公开 Project API，提供 Package 创建、attach／wrap、层级查看和带原因的安全撤销；不读取客户角色名，不复制第二套状态机，也不暴露 HttpOnly Capability Cookie。普通测试、`go vet`、`go build`、PostgreSQL 18 `-race -tags=integration` 和页面脚本语法检查均通过。

更新隔离 D1 后，`acceptance/d1-c1-authorization/verify-running` 通过正式 `Nginx → Gateway → IAM／Policy → Project → People` 链执行 152 个 HTTP 断言，状态不匹配为 0，最长本地请求 0.263164 秒。它直接证明：OA 业务管理员和只读角色收到不同动作集合；撤销 `hierarchy.manage` 后重新授权，页面数据中的动作立即收敛且直接写入返回 403；撤销全部 Project 入口能力后重新授权失败，Gateway 清除旧 Project Capability Cookie。该验收检查页面资源、脚本和同一公开 API，但未执行像素级浏览器视觉回归。

J1-A27～A28 的当前工程证据已补齐；客户 Party 人类选择与 D1 恢复证据又补齐 J1-A29～A33 的页面入口，但该结论仍不证明项目客户改绑／解绑、项目包合同金额分配、跨租户、真实公网／公司环境或 C2～C8，OA 完整体阶段判定继续为 `insufficient-evidence`。

## Demo 通过定义

J1 只有在 J1-A01～J1-A33 全部获得实际证据后才算完成。规格、空目录、手工演示或只在浏览器看到页面都不能替代通过。业务内核可以用模板 IAM 独立验收；J1-A28 必须额外穿过 Nginx、Gateway、中心 IAM 与 Workbench，并复用同一 API、migration 和状态机。J1-A29～A33 已同时取得核心、正式 D1 API 链和客户选择页面证据，但不等于已有项目客户改绑／解绑语义或旧台账本地主体已经迁移。
