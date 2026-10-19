# Production Operations：多生产类型交付

## 元信息

- 路径：`spec/production-operations/multi-type-production.md`
- 所属模块：`production-operations`
- 状态：🟡 部分实现；C4-A01～A14 当前本地轮次通过，Production 专项撤权／账号停用／Assignment 结束／真实浏览器已补证，双 Tenant 与真实 Ubuntu 待验
- 优先级：P0
- 上层旅程：[OA 完全体](../assembly/full-reference-oa-validation.md) 的 C4
- 现有第一实现：[Design Operations](../design-operations/design-production-delivery.md)
- 关联技术：`tech/production-operations/decision.md`
- 关联决策：BF-032、BF-038、BF-039、BF-056

## 问题与可验证结果

**问题 A**：多生产类型既共享负责人、阶段、成果、revision 和纠错，又有不同阶段与完成条件；按类型复制应用会复制稳定生命周期，把类型塞入 Project 或通用工作流又会扩大核心和丢失业务不变量。

**可验证结果 B**：Production Operations 作为一个独立数据所有者，以代码拥有、版本化 Type Definition 装入 `design-v1` 和 `consultation-v1`；同一 Project 的两种 Work Item 分别推进、完成和纠错，未选择的类型真实缺席，既有设计数据与 J3 行为保持兼容。

## 稳定边界

### Production Operations 拥有

- Production Work Item ID、Project 引用、type key、definition version、状态和 revision；
- 顺序 Stage 实例、唯一 frontier、适用性、完成／重开 revision；
- active `production_owner` Assignment 引用；
- Evidence File ID／Version 的成果业务引用、替换链和显示快照；
- command 幂等、可信 Actor、不可变历史、允许动作和 reference-read。

### 只引用

- Project Workspace 的 Project ID、status、revision 和 `project_type_keys`；
- Project Workspace 的同 Project active `production_owner` Assignment；
- Evidence Service 的 File ID／Version、status、revision 和摘要；
- IAM 验证后的 subject、tenant、audience、action 和 scope。

### 不拥有

- Project 身份、父子结构、客户或整体完成状态；
- Person／Group／Membership、账号或客户岗位；
- 文件字节、OSS 对象键、STS 或签名 URL；
- 批复审计、成本、外委、奖金、合同、结算、开票、义务、实际资金和核销；
- 用户可编辑的通用流程、任意条件、脚本或表单平台。

## Type Definition 契约

每个定义包含稳定 key、不可变 version、有序 stage、每阶段可不适用规则和必需成果槽位。定义随代码／契约发布并独立验收；数据库实例保存 definition key/version，不能用当前配置重新解释旧记录。

未知、未安装或未被 Project 选择的类型失败关闭。类型版本升级不得原地改变旧 Work Item；需要新语义时创建新版本并明确迁移／并存规则。

### `design-v1`（既有）

| 顺序 | Stage | 可不适用 | 必需成果 |
| --- | --- | --- | --- |
| 10 | `scheme` | 是 | `primary-output` |
| 20 | `preliminary-design` | 是 | `primary-output` |
| 30 | `construction-drawing` | 否 | `primary-output` |
| 40 | `construction-support` | 否 | `primary-output` |

### `consultation-v1`（C4 第一增量）

| 顺序 | Stage | 可不适用 | 必需成果 |
| --- | --- | --- | --- |
| 10 | `scheme` | 是 | `primary-output`（适用时） |
| 20 | `report` | 否 | `primary-output` |

## 多类型规则

- Project 可以选择一个或多个类型；每个类型对应独立 Work Item，不合并 revision、frontier、负责人引用或历史。
- 同 Project、同 definition key 同时最多一个未取消 Work Item；不同类型可并存。
- 页面可以把同名栏目做只读／交互组合，但写命令必须带明确 Work Item 和 Stage。
- 一种类型完成不自动完成另一类型，也不自动写 Project 整体状态。
- 未选择的类型不得出现在缺口、导航、权限要求、备份特殊逻辑或启动依赖中。

## 生命周期与纠错

Work Item 和 Stage 状态沿用已验证的 `draft`、`in-progress`、`completed`、`cancelled` 与 `pending`、`ready`、`completed`、`not-applicable`。推进、替换、重开和恢复适用性规则与 `design-v1` 保持同一稳定语义：

- 只有唯一 ready Stage 可以写入或完成；
- 完成时重新验证 active owner 和所有必需 Evidence Version；
- 成果替换追加新引用，不改写旧 File ID／Version；
- 已完成 Stage 先重开后才能改变成果；
- 恢复较早不适用 Stage 或重开会回退后续完成投影但保留历史；
- 每个命令带 `command_id` 和 `expected_revision`，重复成功返回原结果，同 ID 异载荷拒绝。

## 权限

目标通用 Capability 使用生产领域名，不使用客户岗位名：

- `production-work-item.create`
- `production-work-item.read`
- `production-work-item.owner.manage`
- `production-work-item.deliverable.manage`
- `production-work-item.stage.manage`
- `production-work-item.cancel`
- `production-work-item.history.read`
- `production-work-item.reference.read`

现有 `design-work-item.*` 只允许在兼容入口代理 `design-v1`，不能操作 `consultation-v1`。Customer Overlay 把参考 OA 的生产经理／项目生产负责人映射为稳定 Capability；Project Assignment Policy 产生精确 Project Scope；目标应用结合 type、Work Item、Project、当前状态和 revision 作最终授权。

## 第一实现验收

验收编号 C4-A01～C4-A14 以[边界审计报告](../../../docs/reports/oa-c4-multi-production-boundary-audit-20260927.md)为准。最低通过集必须同时包含：

- 既有 `design-v1` 真库兼容升级；
- `consultation-v1` 创建、负责人、适用性、报告成果、完成、重开、取消、历史；
- 同 Project 双类型互不覆盖；
- Project／Assignment／Evidence 故障零部分写与恢复恰好一次；
- 相邻角色、相邻 Project、错 Tenant／Audience／Scope 和撤权拒绝；
- J3-A01～J3-A20、Level A Design 和当前 Level B 回归；
- 无共享表、跨应用 ORM、通用流程解释器、客户核心分支或双写。

2026-09-27～28 已落地不可变类型注册表、`consultation-v1` 两阶段实例化、旧 Design API 类型隔离、PostgreSQL additive migration，以及 canonical Production API／Capability／Overlay／Assignment Policy。C4-A01～A09 的真库证据覆盖旧事实确定性升级、类型门禁零写入、同 Project 双类型隔离和完整生命周期；canonical 权限覆盖双 audience、旧能力拒绝、精确 Project／Work Item Scope、相邻角色／资源／租户／revision 拒绝。随后 consultation-only 最小组合、Level A／B 聚合回归，以及 People＋Project＋Evidence＋Production 的隔离 D1 正式入口、Workbench、未选应用缺席、依赖故障恢复和联合空目标恢复均通过。新的 86 条入口断言与真实浏览器又直接证明 Grant 撤销后 `allowed_actions` 收敛、恢复授权、账号停用退出提交态、重新登录恢复，以及 Assignment 结束后新策略凭证拒发。双 Tenant 和真实 Ubuntu 仍以报告限制为准。

## 明确延期

- `survey-v1`：试验阶段是否拥有结构化试验事实尚待证据；
- `mapping-v1`：需避免仅因阶段形状相同就冒充业务已确认；
- 设计校审单／评分的类型专属扩展；
- C5 批复审计、C6 成本外委、C7 奖金；
- 跨域项目整体完成写回、通用任务／审批／通知和自定义流程编辑器；
- 双 Tenant 与真实 Ubuntu；D1 本地正式最小组合、权限生命周期和真实浏览器已经通过，真实环境仍单独验收。
