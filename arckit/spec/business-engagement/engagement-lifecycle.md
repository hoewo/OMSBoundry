# Business Engagement：经营承接

## 元信息

- 路径：`spec/business-engagement/engagement-lifecycle.md`
- 所属模块：`business-engagement`
- 状态：🟡 进行中；责任边界、后端纵向闭环、正式 D1 装配、页面和第一组权限矩阵已实现，空目标与完整故障恢复尚未完成
- 优先级：P0
- 上层旅程：[OA 完全体](../assembly/full-reference-oa-validation.md) 的 C2
- 关联技术：`tech/business-engagement/decision.md`
- 关联决策：BF-003、BF-005～BF-007、BF-009～BF-010、BF-016～BF-017、BF-036～BF-042
- 实施状态：规格✅ | 边界✅ | 模型✅ | API✅ | D1 装配✅ | 页面✅ | 验收🟡

## 问题与可验证结果

**问题 A**：承接方式、阶段适用性、投标材料和承接完成条件如果继续放在 Project、Contract 或宽财务表中，Project 会吸收客户经营流程，Contract 会拥有合同形成前的事实，费用原因与实际付款也会重新混在一起。

**可验证结果 B**：Business Engagement 独立拥有一个项目的经营承接事实和可追溯历史；Project、Evidence、Contract 与 Money Flow 只通过版本化引用和建议衔接，未选择 Contract 或 Money Flow 时仍能完成“判断是否适用、选择承接方式、补齐材料、得出可解释阶段状态”的最小闭环。项目客户关系由 Project 拥有；只有以后出现与项目客户不同且证据充分的独立主体角色时，Business Engagement 才直接引用 Party。

## 证据边界

参考 OA 已确认并实现以下行为：

- 招投标阶段可以整体设为“不适用”，并可恢复；
- 不适用与单份材料因承接方式而不要求是两类事实；
- 承接方式是比选、竞争性谈判、招投标三选一；
- 招投标要求招标、投标和中标材料，比选／竞争性谈判不要求招标材料，但仍要求投标和中标材料；
- 适用材料补齐后，经营进度的承接阶段自动完成；
- 当前页面使用项目级 `view_business_info`／`manage_business_info`，没有证明目标 Capability 应继续合并为一个粗动作。

以下语义尚未得到参考需求和运行实现的直接证据，因此不进入第一版内核：

- 未中标、流标、终止承接、重新发起和多轮承接的准确状态机；
- “中标文件存在”是否等于法律或经营上的最终承接确认；
- 阶段完成后是否自动创建合同，还是只产生人工确认的合同建议；
- 专家费的申请、审批、义务确认和实际付款责任链；
- 客户所在地、投资额或行业规则如何决定承接方式和材料清单。

## 第一版稳定范围

### Business Engagement 拥有

- 租户内稳定 Engagement ID、Project Reference 和 revision；
- `applicable`／`not_applicable` 适用性事实、原因、Actor 和历史；
- `comparison`、`competitive_negotiation`、`tender` 三种稳定承接方式；
- 当前方式对应的材料要求投影，以及 Evidence File ID／Version 的业务含义引用；
- 当前完成条件、缺口、允许动作、可信 Actor、命令幂等记录和不可变历史；
- 对 Contract 或 Money Flow 的版本化建议及其投递／确认状态，但不拥有目标事实。

### Business Engagement 只引用

- Project Workspace 的 Project ID、tenant、状态和 revision；
- Evidence Service 的 File ID／Version；
- IAM Capability Credential 中验证过的 Subject、Tenant、Action 和 Resource Scope；
- Contract、Money Flow 返回的建议确认 ID，不复制其合同、义务或付款内容。

若后续业务证据确认存在与 Project 客户不同的甲方、资质单位或其他独立主体角色，才增加 Party Directory 的 Party ID／revision 引用并由 Business Engagement 冻结该角色的最小快照；当前第一版不预建该字段。

### Business Engagement 不拥有

- Project 名称、编号、父子结构、Assignment 或整体状态；
- Party 当前档案、内部人员名册、账号或客户岗位；
- 合同条款、金额、补充、终止、结算和法律 revision；
- 资金义务、实际收付、核销、余额、发票或会计事实；
- 文件字节、对象键、摘要真相、STS 或签名 URL；
- 客户所在地、投资额到承接方式的自动判断；
- 通用审批、低代码表单、任意材料清单或万能经营流程。

## 第一版生命周期

第一版只承诺由直接证据支撑的最小状态，不用猜测填充完整投标流程：

| 状态 | 含义 | 允许的核心动作 |
| --- | --- | --- |
| `draft` | 已引用 Project，尚未确认适用性或承接方式 | 设置适用性、选择方式、取消 |
| `collecting_evidence` | 阶段适用且方式已选，适用材料尚未齐备 | 登记／替换 Evidence 引用、改变方式、设为不适用、取消 |
| `ready` | 当前方式要求的材料均已登记 | 查看缺口、纠正材料、恢复处理中、提出下游建议 |
| `not_applicable` | 当前项目明确跳过该阶段 | 恢复适用；不得登记阶段材料或自动创建合同 |
| `cancelled` | 明确终止本记录且保留历史 | 只读与历史查询 |

`ready` 只表示参考 OA 已确认的“材料条件完成”，不自动声明中标、签约或收到款项。后续若业务证据证明需要 `won`、`lost`、`aborted`、`reopened` 或多轮记录，应以兼容 revision 增加，而不是把 `ready` 偷换成未经确认的最终结果。

## 材料策略

2026-10-02 BF-095 多附件候选：原 OA 同类多文件的直接证据要求扩展当前集合。既有 v1 路由增加 append／remove_one、复用原权限和 expected revision；旧 put 整类替换、remove 整类移除及旧 hash／回执保持。仅结束 BE 业务引用，不物理删除 Evidence；当前总数最多 100，与既有 Project material-reference v1 消费上限一致。只追加 migration 000002、保留旧行／历史；升级后禁止旧二进制直接回滚。候选实施／验证范围见[切片规格](../../../docs/specs/assembly/oa-bidding-material-collection-slice.md)，此段不宣称已经部署或全链通过。

Base 固定当前已有证据的三种方式和三类稳定材料 key：

| 承接方式 | `tender_document` | `bid_document` | `award_document` |
| --- | --- | --- | --- |
| `tender` | required | required | required |
| `comparison` | not-required-by-method | required | required |
| `competitive_negotiation` | not-required-by-method | required | required |

- 阶段 `not_applicable` 与材料 `not-required-by-method` 必须分别保存和显示；
- 留空不等于不适用；必需材料缺失时状态不得进入 `ready`；
- 材料替换形成新 revision／历史事件，旧 File Version 引用仍可解释；
- 客户增加材料、改名称或增加校验规则时，先放入版本化策略／Overlay；只有第二个稳定场景证明语义一致后才提升为 Base；
- 客户策略不得改变 Evidence、Project、Contract 或 Money Flow 的数据所有权。

## 用户场景

### US-1：建立经营承接记录 [P0]

- **Given** Project 引用存在、同租户且状态允许开始经营承接；
- **When** 有权限的用户创建 Engagement；
- **Then** 系统保存稳定 ID、Project 引用、revision 1 和创建历史，不复制 Project 资料。

### US-2：选择适用性和承接方式 [P0]

- **Given** Engagement 未取消；
- **When** 用户设为不适用，或恢复适用并选择一种承接方式；
- **Then** 系统按当前方式重算材料要求和缺口，记录原因、Actor、expected revision 与历史。

### US-3：登记承接材料并形成可解释完成状态 [P0]

- **Given** 当前方式已确定且 Evidence 引用可验证；
- **When** 用户登记当前方式要求的材料；
- **Then** 所有适用材料齐备时进入 `ready`，否则保持 `collecting_evidence` 并返回明确缺口。

### US-4：纠错与恢复 [P0]

- **Given** 方式、适用性或材料登记错误；
- **When** 用户以当前 revision 显式纠正；
- **Then** 系统追加历史并重算状态，不静默覆盖旧解释，也不修改 Evidence 字节或外部业务事实。

### US-5：向下游提出建议 [P1，语义待业务证据收口]

- **Given** Engagement 为 `ready`，且所选组合启用了目标应用；
- **When** 用户明确请求建立合同草稿或记录经营费用义务；
- **Then** 连接器发送带 Engagement revision、Project／Party 快照和幂等键的建议；目标应用独立确认，失败可重试，Business Engagement 不把“已发送”冒充“目标已建立”。

## 权限与身份

第一版候选 Capability 使用领域动作，不使用“经营经理”“项目经营负责人”等客户岗位名：

- `business-engagement.create`
- `business-engagement.read`
- `business-engagement.applicability.manage`
- `business-engagement.method.select`
- `business-engagement.evidence.manage`
- `business-engagement.suggestion.submit`（P1 预留，当前 Manifest 不发布）
- `business-engagement.cancel`
- `business-engagement.history.read`

当前已实现的七个 Capability 名称与拆分由 `contracts/access/v1/examples/business-engagement-capabilities.json` 固定；下游建议动作只是 P1 候选，在建议接口、幂等与目标确认语义实现前不纳入可签发 Manifest。`reference-oa-c2@1.0.0` 把站点角色映射为显式 tenant Capability，`business-engagement-project-access@1.0.0` 把 Project Assignment 的稳定 role key 映射为精确 Project Scope；目标应用不认识“经营经理”等客户岗位。Nginx 不解释身份，Gateway 清洗外来身份 Header 并执行入口门禁，IAM／Policy 只签发有界凭证，Business Engagement 复验 tenant、audience、action、scope、revision 和当前状态并作最终授权。

第一条 API 闭环已实现双重服务端门禁：先验证目标应用原生 action，再验证 tenant 或精确 Project／Business Engagement scope；即使请求绕过 Gateway 直接到应用，缺凭证、缺 action、跨 Project scope 和伪造身份 Header 仍失败关闭。业务 Service／数据库随后继续校验 expected revision、当前状态、引用 revision 和材料规则，因此 Gateway／IAM 不替代目标应用最终授权。

身份超级管理员默认没有 Business Engagement 业务动作。隔离 D1 已直接验证允许角色、相邻角色拒绝、Project B 拒绝、状态拒绝、Assignment 结束／Grant 撤销后的新凭证拒绝、直接 API 绕过拒绝和拒绝不产生部分写入；跨租户、账号停用和 Policy／依赖故障仍需专项证据。

## 最小组合与故障边界

- 最小闭环：IAM／同形本地 Provider + Project Workspace + Evidence Service + Business Engagement；Party Directory 在需要结构化主体引用时作为显式依赖，第一版是否允许仅名称快照由实现规格另行确认；
- Contract、Money Flow 和 Billing 不应成为“完成承接材料闭环”的启动前提；
- Project 不可用时，新建或需新鲜 Project revision 的写入失败关闭，既有 Engagement 历史仍可读；
- Evidence 不可用时不得确认新 File Version，既有引用和历史仍可解释；
- IAM／Policy 不可用时，高风险新授权失败关闭，未过期凭证只在既定 TTL／scope 内继续；
- 下游 Contract／Money 不可用时，建议保持可重试状态，不回滚已经确认的 Engagement revision。

## 第一版验收门槛

- [x] 独立数据库、migration、应用 release，以及隔离 D1 联合备份、销毁专属卷后的空目标恢复和继续写入；真实 Ubuntu／离机恢复仍由环境验收负责；
- [x] 创建、适用性、三种方式、材料缺口、`ready`、材料替换／移除、不适用／恢复适用、取消和历史的单元／PostgreSQL 18 真库测试；
- [x] Project／Evidence 引用失败关闭，不复制跨应用事实；正式 D1 又证明成功路径使用精确 Project／File 工作负载凭证；
- [x] 相同命令幂等、同 command 不同载荷拒绝、旧 revision 冲突、并发恰好一个成功；
- [x] 核心 Service／API 不导入 Contract／Money，未启用二者时可计算材料缺口与 `ready`；独立 release／部署验收仍待完成；
- [ ] 下游建议的幂等、目标拒绝、超时、重试和确认证据；
- [ ] 多角色、多 Project、跨租户、状态、撤权和伪造身份矩阵；第一组 C2 权限矩阵已通过 89 个正式入口断言，扩展后的 95 个断言又覆盖 409／503 自动恢复；跨租户、账号停用和完整 IAM／Policy 故障仍待验；
- [ ] 客户页面可完成主要任务并在 401／403／409／503 后可解释恢复；当前 Workbench 入口、服务端 `allowed_actions`、实际浏览器撤权／恢复刷新、Assignment 结束后的新入口拒绝、409 stale revision 无写入及 Evidence 503 恢复后新命令单次写入已通过，真实浏览器最终点击复核仍待直接证据；
- [x] Nginx → Gateway → IAM／Policy → Business Engagement 的正式 D1 入口证据；
- [x] C1 权限矩阵在多目标 Assignment Policy 改造后仍以 152 个正式入口断言通过；六应用 Level A 和 J1～J4 Level B 仍需在 Business Engagement 页面／恢复切片后再跑阶段总回归。

## 明确不负责

- 不复制参考 OA 的 `bidding_info`、`oa2026_bidding_settings` 或宽 `financial_records` 表；
- 不把合同、开票、义务、现金、核销或 Expert Fee 实际付款迁入本应用；
- 不根据客户投资额、地区或页面选择自动推断承接方式；
- 不在缺少证据时实现未中标、流标、复杂审批、自动建合同或多轮投标；
- 不因完整 OA 需要连续页面，就把 Project、Party、Evidence、Contract 或 Money 的事实合并进同一服务。
