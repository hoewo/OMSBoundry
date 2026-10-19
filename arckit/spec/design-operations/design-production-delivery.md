# Design Operations：设计生产与交付

> 兼容定位（2026-09-27）：本文继续作为已实现 `design-v1` 的行为与回归基线。C4 目标以同一事实源兼容演进为 Production Operations；多类型新增、旧入口限制和 `consultation-v1` 由 [多生产类型交付](../production-operations/multi-type-production.md) 约束，不能反向改写本文已经验证的 ID、revision、成果和历史。

## 元信息

- 路径：`spec/design-operations/design-production-delivery.md`
- 所属模块：`design-operations`
- 状态：🟢 已实现
- 优先级：P0
- 负责人：BizFoundry
- 上层旅程：[参考 OA Demo](../assembly/reference-oa-demo.md) 的 J3
- 关联模型：`tech/_shared/models/DesignWorkItem.yaml`、`DesignStage.yaml`、`DesignDeliverable.yaml`、`DesignHistoryEvent.yaml`
- 关联 API：`tech/_shared/contracts/design-*.yaml`
- 关联技术：`tech/design-operations/solution.md`
- 实施状态：模型✅ | API✅ | 设计✅ | 技术✅ | 开发✅
- 变更历史：
  - 2026-09-25：固定第一种 `design-v1` 生产类型的工作项、阶段、适用性、成果引用、纠错和验收边界。
  - 2026-09-25：完成独立运行时、Project／Assignment／Evidence reference-read、J3-A01～J3-A20、备份恢复、Linux release 与非 root Docker 验证。

## 问题与结果

**问题 A**：设计项目虽然已经立项并有生产负责人，但设计阶段、必交成果和完成条件没有独立事实所有者；空白字段也无法区分“尚未完成”和“明确不适用”。

**可验证结果 B**：有权限的用户为一个已立项的 `design` Project 建立设计生产工作项，绑定有效生产负责人，按固定顺序完成或明确跳过允许不适用的阶段，以 Evidence File Version 登记成果，并得到可下钻、可纠错的生产完成结果。

## J3 最小范围

### Design Operations 拥有

- 租户内稳定 Design Work Item ID；
- `design-v1` 类型和类型定义版本；
- `scheme`、`preliminary-design`、`construction-drawing`、`construction-support` 四个阶段实例及顺序；
- 阶段当前状态、明确不适用原因、完成／重开 revision；
- 每个适用阶段的 `primary-output` 必交成果槽位；
- 指向 Evidence File ID／Version 的成果引用及替换历史；
- 指向 Project 和 Project Assignment 的稳定引用；
- Work Item revision、命令幂等记录、可信 Actor 和不可变历史；
- Design Work Item 的 `draft`、`in-progress`、`completed`、`cancelled` 当前投影。

### Design Operations 只引用

- Project Workspace 的 Project ID、Project revision、`design` 类型和状态；
- Project Workspace 的 active `production_owner` Assignment ID 与来源 revision；
- Evidence Service 的 File ID、Version、tenant、状态与来源 revision；
- IAM 已验证的 tenant、subject、audience、action 和 scope。

### Design Operations 不拥有

- Project 名称、编号、日期、参与人名册或 Project Assignment 生命周期；
- 人员、Group、Membership、账号、密码或 IAM 授权；
- 文件字节、对象键、摘要真相、STS 或签名 URL；
- 合同、结算、义务、资金、核销、成本、外委、奖金或薪酬；
- 通用任务、工作流、审批、通知、表单设计器或客户自定义阶段脚本；
- 咨询、勘察、测绘等第二生产类型。

## 类型定义与状态

`design-v1` 是代码和契约拥有的版本化类型定义，不是用户可编辑的万能流程。第一版固定：

| 顺序 | Stage key | 显示含义 | 可设不适用 | 必交成果 |
| --- | --- | --- | --- | --- |
| 10 | `scheme` | 方案阶段 | 是 | `primary-output` |
| 20 | `preliminary-design` | 初步设计阶段 | 是 | `primary-output` |
| 30 | `construction-drawing` | 施工图阶段 | 否 | `primary-output` |
| 40 | `construction-support` | 施工配合／竣工验收阶段 | 否 | `primary-output` |

新增第二生产类型时必须新增独立、版本化类型定义和验收，不通过修改 Contract、Money、People、Evidence 或 Project 核心表实现。

### Work Item 状态

| 当前状态 | 允许动作 | 进入条件 |
| --- | --- | --- |
| `draft` | 设置生产负责人、查看、取消 | 创建成功但尚无有效负责人 |
| `in-progress` | 设置负责人、登记／替换成果、完成／重开阶段、设置／恢复不适用、取消 | 存在有效 `production_owner` 引用且尚有未完成阶段 |
| `completed` | 查看、重开阶段、取消 | 四阶段全部 `completed` 或合法 `not-applicable` |
| `cancelled` | 只读与历史查询 | 明确取消命令成功 |

### Stage 状态

- `pending`：前序条件未满足；不能登记完成。
- `ready`：当前可以登记成果、完成或设置不适用的阶段。
- `completed`：服务器验证有效生产负责人、全部必交成果和 Evidence 引用后完成。
- `not-applicable`：仅允许 `scheme`／`preliminary-design`，必须记录原因和 Actor。

任一时刻最多一个 Stage 为 `ready`。创建后第一个 Stage 为 `ready`，其余为 `pending`；完成或合法跳过当前 Stage 后，下一个 Stage 变为 `ready`。最后一个 Stage 完成后 Work Item 进入 `completed`。

## 关键不变量

- 只允许 active、包含 `design` 类型的 Project 创建 Work Item；同 Project 同时最多一个未取消的 `design-v1` Work Item。
- 设置负责人必须引用该 Project 当前 active 的 `production_owner` Assignment；Assignment 结束不会被本应用改写，后续 Stage 写操作失败并要求重新设置负责人。
- 每个写命令消费 `expected_revision` 和 `command_id`；相同 ID 同内容返回原结果，不同内容返回 409。
- Stage 完成前必须在线验证负责人和每个 active 必交 Evidence Version；Project 或 Evidence 不可用时失败关闭且不形成部分完成。
- 成果替换先验证新 Evidence Version，再追加新 DesignDeliverable 并结束旧引用；旧 File ID／Version 不改写、不删除。
- 完成后的 Stage 不能靠替换成果静默改变；必须先执行显式 `reopen`。
- 重开某 Stage 会把它变为 `ready`，把其后已经 `completed` 的 Stage 变回 `pending`，但保留原成果引用和全部历史；其后的 `not-applicable` 决定保留。
- 恢复早期 `not-applicable` Stage 使用同一回退规则；旧不适用原因仍在历史中。
- Work Item 的 `completed` 是 Design Operations 自己的完成投影，不直接更新 Project 状态，也不宣称经营链已完成。
- 取消只冻结本 Work Item；不取消 Project、不删除 Evidence 文件、不结束 Project Assignment。

## 用户场景

### US-1：建立设计生产工作项 [P0]

**Given** active Project 包含 `design` 类型；**When** 用户提交 Project ID 和已知 revision；**Then** 系统创建 `design-v1` Work Item、四个 Stage、revision 1 和创建事件。

- 同 Project 重复 active Work Item 返回冲突；
- Project 不存在、跨租户、未立项、已取消或不含 `design` 时零写入；
- Project 暂时不可用时返回 503，可用同 command 重试。

### US-2：设置生产负责人 [P0]

**Given** Work Item 未取消；**When** 用户提交 Project Assignment ID；**Then** 系统验证其属于同 Project、同 tenant、角色为 active `production_owner`，保存引用并进入或保持 `in-progress`。

- 不复制 Person／Group 或 Membership；
- 原负责人结束后，新 Stage 写入被拒绝，直到显式设置新的 active Assignment；
- 更换负责人只追加新引用和历史，不修改旧 Assignment。

### US-3：登记与替换阶段成果 [P0]

**Given** 当前 Stage 为 `ready`；**When** 用户把一个可用 Evidence File Version 登记为 `primary-output`；**Then** 系统只保存 File ID、Version、Evidence revision、业务槽位和显示快照。

- Evidence 不存在、跨租户、状态不可用或 revision 不匹配时零写入；
- Evidence 停机时返回 503，同 command 重试不产生重复成果；
- 替换成果追加新记录并结束旧记录；已经完成的 Stage 必须先重开。

### US-4：推进、跳过与恢复 Stage [P0]

**Given** 当前 Stage 为 `ready`；**When** 用户完成 Stage 或为允许的 Stage 明确设置不适用；**Then** 系统推进到下一 Stage，并保留条件证据。

- 缺负责人、缺必交成果、非当前 Stage 或必交引用失效时不能完成；
- `construction-drawing` 和 `construction-support` 不能设不适用；
- 重开已完成 Stage 或恢复不适用会显式回退后续完成投影，不删除历史或成果引用。

### US-5：完成、取消与追溯 [P0]

**Given** 最后一个 Stage 满足完成条件；**When** 用户完成该 Stage；**Then** Work Item 进入 `completed`，reference-read 返回可解释完成状态。

- Project 当前经营状态不由本应用推测或改写；
- 取消后所有写命令拒绝，详情和历史仍可读；
- Project Workspace 停机时已存在 Work Item 和历史仍可读，但新建、设置负责人和 Stage 写入失败关闭；
- Evidence 停机只阻止新的成果登记和 Stage 完成，不让既有 Work Item 消失。

## 权限与可见性

- `design-work-item.create`
- `design-work-item.read`
- `design-work-item.owner.manage`
- `design-work-item.deliverable.manage`
- `design-work-item.stage.manage`
- `design-work-item.cancel`
- `design-work-item.history.read`
- `design-work-item.reference.read`

本地模板与正式 IAM 必须产生同形 v1 上下文。scope 必须是 tenant 或明确 Work Item／Project 范围；拥有 Project Assignment 不自动获得系统能力，Design Operations 始终作最终授权。

## J3 可执行验收矩阵

| ID | 场景 | 输入／故障 | 期望证据 | 层级 |
| --- | --- | --- | --- | --- |
| J3-A01 | 创建 Work Item | active design Project | 201、四 Stage、revision 1、事件 | HTTP＋PostgreSQL |
| J3-A02 | 非法 Project | 跨租户／非 active／无 design | 422、零写入 | 系统集成 |
| J3-A03 | Project 故障 | reference API 不可用 | 503、可重试、零写入 | 系统集成 |
| J3-A04 | active 唯一 | 同 Project 创建第二个 | 409、原 Work Item 不变 | PostgreSQL |
| J3-A05 | 设置负责人 | active production_owner | 200、只保存 Assignment 引用 | HTTP 集成 |
| J3-A06 | 非法负责人 | 错 Project／角色／已结束 | 422、零写入 | 系统集成 |
| J3-A07 | 负责人失效 | 设置后 Assignment 结束 | Stage 写入失败，换人后恢复 | 系统集成 |
| J3-A08 | 登记成果 | 合法 Evidence Version | active deliverable、来源 revision、事件 | HTTP 集成 |
| J3-A09 | Evidence 故障 | 登记时不可用 | 503、同 command 可重试、零部分写 | 系统集成 |
| J3-A10 | 替换成果 | 新 File Version | 新记录 active，旧记录结束且可追溯 | PostgreSQL＋HTTP |
| J3-A11 | 完成缺口 | 无负责人／缺 primary-output | 422 缺口，不推进 | 领域＋HTTP |
| J3-A12 | 顺序守卫 | 尝试完成 pending Stage | 422，不越级 | 领域测试 |
| J3-A13 | 不适用 | scheme 明确原因 | not-applicable、下一 Stage ready | HTTP 集成 |
| J3-A14 | 非法不适用 | construction-drawing | 422、状态不变 | 领域测试 |
| J3-A15 | 恢复不适用 | 恢复较早 Stage | 该 Stage ready，后续完成投影回退，历史保留 | PostgreSQL＋HTTP |
| J3-A16 | 重开完成 Stage | 明确原因 | Stage／Work Item 回退，无历史删除 | PostgreSQL＋HTTP |
| J3-A17 | 最终完成 | 四 Stage 满足条件 | Work Item completed、reference-read 可用 | 系统集成 |
| J3-A18 | 幂等与并发 | command 重放／旧 revision | 同结果或 409，恰好一次事件 | PostgreSQL |
| J3-A19 | 越权与取消 | 无凭证／错 scope／取消后写入 | 401／403／409，零额外写入 | 契约＋HTTP |
| J3-A20 | 独立与历史 | 停 Project／Evidence、尝试改历史 | 既有读取可用，外部写失败关闭，历史不可变 | 组合验收 |

## 实现与验证证据

- `backend/services/design-operations` 是独立 Go module、PostgreSQL 数据所有者和发布单元，包含 migration、十个 v1 HTTP 路由、模板／正式 IAM Provider、Dockerfile、备份恢复与 Linux release；
- Project Workspace 提供 Project 和 Assignment 最小 reference-read；Evidence Service 提供 File／Version 最小 reference-read，均不暴露目标应用内部表或 OSS Object Key；
- 领域、HTTP 和真实 PostgreSQL 集成测试覆盖 J3-A01～J3-A20，包括阶段顺序、适用性、负责人／成果失效、替换、回退、完成、取消、幂等、并发、scope、故障关闭和历史不可变；
- 2026-09-25 已通过普通 `go test`／`go vet`／`go build`、PostgreSQL 18 integration、race、冷备份恢复、Linux AMD64 release 摘要校验、非 root Docker 构建与 readiness 冒烟；准确命令见 `docs/testing.md`。

## Demo 通过定义

J3-A01～J3-A20 已获得直接自动化和可重复验收证据。该结果只证明一种设计生产类型的业务纵向闭环；第二类型、完整 OA、Project 总体完成、统一入口和商业通用性仍必须分别验证。

## 明确延期

- 咨询、勘察、测绘和多类型并行；
- 自定义阶段编辑器、通用工作流、审批、通知和待办平台；
- 生产批复审计、成本、外委、奖金、薪酬、考勤与客户公式；
- CAD 在线预览／编辑；
- 自动写 Project 完成状态、自动创建 Money Flow 义务或自动修改 Contract；
- Nginx、Gateway、中心 IAM 和 D1 总装。
