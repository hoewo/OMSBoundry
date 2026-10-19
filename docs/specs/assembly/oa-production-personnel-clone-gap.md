# D01 生产人员：原操作与当前缺口

2026-10-04 最新：BF-105已候选补新 schema1分工引用、Bonus显式v3和正常工作台接线，真实私有Nginx／Gateway／正式IAM连续20场景通过，见[报告](../../reports/oa-professional-bonus-continuous-validation-20261004.md)。下方“正常工作台不导入／下一消费代码阻塞”是前一批历史；旧Base／v2保持，新端口及范围见[更新规格](./oa-production-personnel-v1.md)。全角色、原站现场、匹配发布／公网／联合恢复及其他D01组合仍待验，不关闭D01整行。

2026-10-04。状态：原 UI 已复制，最终 16 条 UI-only 交互通过；BF-104 已接受 Project 有界模型，原生 PG18／IAM／People／UI 候选已接通并分层实测，未发布公网、D01 未整体验收。[界面层证据](../../reports/oa-production-personnel-ui-candidate-20261004.md)、[原生证据](../../reports/oa-production-personnel-native-validation-20261004.md)与[有界规格](./oa-production-personnel-v1.md)。

问题 A→可验证结果 B：通用负责人／参与者无法表达原 OA 的专业生产分工 → 在同项目原“人员配置”内完成专业名单、六种分工、编辑／移出及刷新，事实有独立 ID／revision／历史，人员和账号所有权不合并。

## 原页面实际怎么用

来源只读 commit `826e0fc65aa09947fc4f64f62a2644fa5d11a302`：`ProductionPersonnelManager.tsx`、`DisciplinePersonnelModal.tsx`、`ReviewerModal.tsx`、`DisciplineSelector.tsx`、`DisciplineManageModal.tsx`；`projectMember.ts`、`useProjectPermissions.ts`、`project_member.go`、`project_member_service.go`。只有源码证据，未运行原站。

| 原入口／显示 | 原交互及业务请求 | 目标不能替代为 |
| --- | --- | --- |
| 生产信息→人员配置 | 同项目按专业分组，显示设计人、参与人、复核人、审核人；组头“人数”实际是 member 行数 | 跳通用 Project 台账；把记录数说成去重人数 |
| 添加／编辑生产人员 | 600px 抽屉；专业必选，设计／参与多选，复核／审核各单选，四类人员均可不填 | 立项必填；单个生产负责人；全部成员无专业 |
| 参与不适用 | `participant_not_applicable` 来自生产适用性；参与列显示“无/不适用”，抽屉禁止该项，名单过滤该角色 | 结束全部角色；清除负责人；将隐藏当删除 |
| 审定／概算 | 独立头部两位置，共同打开“设置审定人和概算人”600px 抽屉；分别单选，可清除，没有专业必填 | 放进每专业的复核／审核列；要求先完成设计人 |
| 保存 | 读取已存在成员，计算每角色增减；每条走原 member create/remove，成功关闭并复读 | 保存 People 或 IAM；以单页面成功假设服务端原子 |
| 更换专业 | 原抽屉允许更换；旧组移除、新组重建角色名单 | 原位静默改历史专业；结束其他专业组 |
| 删除专业内配置 | 原确认显示专业和行数，逐条 remove；专业目录另有增／改／删入口 | 删除人员、账号或该专业字典；抹奖金历史 |
| 只读及候选故障 | 原 `project.production.member.assign` 控制按钮；候选失败禁用选择并提供重试 | 经营成员权／基本资料权自动扩为生产配置权 |

原 UI 文字是“复核人”，角色 `reviewer`；不以印象改成其他标签。原 professional roles 是 designer／participant／reviewer／auditor，审定 approver／概算 estimator 为项目级独立角色。后端前四者要求有效专业，审定／概算不要求；其中单值角色的唯一性也须看服务端与数据库，不以 Select 单选证明并发安全。

原专业目录为可新增／编辑／停用的全局字典，原 `DeleteDiscipline` 只是 `is_active=false`，不是物理删除；当前原服务未证明引用占用或并发名称唯一。参与适用性从 `oa2026Service.getApplicability` 与 `oa2026_applicability_states` 读取；不以它在生产页显示就推断目标应由 People 或 Production 拥有所有适用性字段。

## 必须修正而不能抄入目标的机制

`DisciplinePersonnelModal` 在构造 promises 时已经启动 create 请求，随后才等待 remove；因此不是可靠的“先删除后新增”，中途失败可能部分成功。专业组删除也逐条异步 mutation，Reviewer 抽屉先批量删再单独新增，均不能当作零写或整批成功。目标可保持原视觉／选人／确认效果，但命令必须有明确原子边界，或保存每段原命令和回执，只续办未完成段。

原 member remove 为删除请求。目标必须追加结束／取代历史，不物理删除 Assignment、Person、账号或历史奖金；清空字段是显式结束，不是无输入默认清除。取消必须零业务写，刷新只读。姓名快照、真实 Person revision、角色／专业和有效期须区分，不能用公司岗位代替项目分工或用 reference_code 冒充账号。

## 当前应用缺口和实现前门槛

Project 旧 Base 角色仍只有 business_owner／production_owner／participant／commercial_contributor；六种专业分工只存在显式 Overlay 的新表及新端口，不扩旧 enum／v1 引用。Production Work Item 的 owner 也只是 production_owner，不是设计／复核／审核名册。Bonus B12 当前只接受它能复验的旧真实 Assignment，不能先扩展示标签伪造专业候选。

参与目的、项目分工仍按已接受边界由 Project 拥有；People 拥有人／组织，IAM 拥有账号与凭证，Production 拥有任务／阶段，Bonus 冻结被引用分工。BF-104 已登记有界版本、Tenant 专业目录、显式选装、追加历史和四种独立能力；11→12／旧事实保持已本地实测。新引用投影及 Bonus 消费兼容尚待完成。不得将六个客户角色名直接加入稳定 Base，或新增无限角色／任意字段平台；不得把已实现的存储说成已完成所有消费和发布。

## 逐动作验收顺序

1. 直接复用原 Manager／CSS、两抽屉、Personnel 控件和确认窗；先验打开、焦点、专业选择、多选、清除、取消及小视口按钮可达。
2. 一个专业的添加→原生保存→同项目名单／刷新，再验同人多角色、多专业、编辑角色／更换专业、单值唯一性、审定／概算、参与不适用；不要一次吞下阶段评分／奖金。
3. 空名单、无权、候选故障、People 状态／revision 变化、重复／并发、未知回复／刷新原命令继续、冲突不得自动覆盖；原生事实／历史／审计和来源零写分别核对。
4. 结束后重入留新身份、旧事实及旧策略不变、精确撤权和错 Project／Tenant、恢复后重放／继续、独立启动和匹配正式入口分层留证。

此对照不能关闭 D01、生产页或全站；[完整交付清单](./oa-complete-operation-delivery-ledger.md)仍有效。下一切片完成新分工版本化引用／Bonus 消费、全角色及匹配入口发布；不能重复上次把负责人误放进专业表单的问题。

第 1 步最终 16／16 UI-only 点击通过，Source CSS 逐字一致；后续原生切片 21／21 已完成六角色、整组事务、幂等、PG 并发／单库恢复和真实 IAM／People 选人操作的有限证据，新增两种目录／人员精确能力组合及原审定人正常清除留史。专业单字段清除／其他全局清除组合／更多名册／全角色／原站现场等效仍须补证。正常 Workbench 不导入独立 preview，正式人员页签保持禁用；只有通过匹配发布、恢复和真实入口验收后才能给公网用户打开。

## 下一消费切片的具体阻塞（代码事实，不是已实现接口）

Bonus `internal/references/http.go` 的 `ResolveManualParticipant` 固定读旧 `/references/projects/{id}/assignments/{assignment_id}`，`ListManualCandidates` 固定从 Project 旧详情的 `assignments` 发现，再逐项复验。`referencebonus/model.go` 的生产 `EligibleRole` 仅接受旧 production_owner／participant，`ItemInput` 又复用没有来源类型的旧 AssignmentRef。生产人员新 ID／专业六角色因此既不能通过旧引用路径，也不能用改显示名绕过资格检查。

下一纵向切片仍按 BF-104：先给 Project 新分工提供显式、最小、版本化且有原生能力的引用投影；Bonus 再明确区分旧 Base 与新 Overlay 来源，冻结专业／Person／分工身份并决定当前适用性，不能把新来源偷换成旧 participant。旧记录、请求 hash、重放与撤回必须兼容；新路径、角色资格和来源类型不得顺手扩至旧 Pool。具体 wire schema／升级 profile 需在该切片实现前固定，本批没有虚构新 HTTP 路由已经存在。

最小联动验收是“同一真实 Person 的两专业分工 → 原奖金表单两条独立候选 → 原生整批分配 → 新旧引用更换／结束后旧奖金快照不变 → 来源离线原命令重放 → 联合恢复后同 ID 继续”。再接正常 Workbench／公网；不以放宽旧 resolver、直接读取 Project 表或复制一份分工来消除阻塞。
