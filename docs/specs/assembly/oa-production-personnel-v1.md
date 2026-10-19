# Project 自有原 OA 专业分工扩展 v1

2026-10-04。BF-104 的受控实施选择。本地候选已实现内部规则、Project 自有 PostgreSQL 存储、默认关闭的 HTTP／IAM 端口及原 UI 独立原生接线；单库恢复与真实进程验收分层记录。未发布 Main、不是完整 D01 或公网可用承诺，见[原生报告](../../reports/oa-production-personnel-native-validation-20261004.md)。

问题 A→可验证结果 B：原六种生产分工不能靠通用 participant／负责人标签表达，原逐条保存又会部分成功 → Project 在显式 `reference-oa-production-personnel-v1` Customer Overlay 中拥有真实专业分工，以整组意图准备原子差量，保留每条身份、姓名／专业快照及结束历史。

## 所有权和兼容

Project 拥有本项目的分工目的和有效期；专业目录也先作为该有界 Overlay 的 Tenant 内字典归 Project，不建立公共字典平台。People 只拥有真实 Person，IAM 只拥有账号和凭证，Production 只拥有任务／阶段，Bonus 只冻结它实际验证过的来源分工。Workbench 仍无业务事实。

扩展仅有 designer／participant／reviewer／auditor／approver／estimator 六个枚举；前四种绑定专业，后两种是项目级。它们是该版本的客户业务语义，不加入旧 Base Assignment 的 Role enum、CHECK 或旧 v1 引用，不把四种稳定职责重命名。禁止同一事实双写或将生产负责人自动复制成设计人。

专业拥有稳定 ID、revision、名称／描述与 active／archived 历史；分工冻结选择时的专业和 Person ID／revision／名称。目录后来改名或 People 后来改名不能静默改旧快照。候选存储采用不可变目录版本、current head、Tenant 内有效名称唯一索引，命令去重和 revision CAS；分工引用本应用的确切目录版本，事务锁定目录 head 后复验有效性。停用不删除旧分工或旧引用，重新启用同一 ID 追加 revision。内部 planner 单独运行仍不证明并发／持久化。

参与不适用仅是此 Project 分工的类型化适用性，不搬入原 monolith 的全部 `ApplicabilityState`。开关只改变可用操作与显示，不自动结束隐藏分工；保存及组移出均明确 preserve 隐藏参与人。恢复适用后应再次显示原身份，而不是创建新记录。

## 意图和事务规则

命令带固定 profile、command ID、expected Project revision 与完整角色操作；replace 的空数组为显式清除，缺少角色／nil 不默认为清除，preserve 不携带被忽略的人员载荷。专业组四角色及项目级两角色分开提交，操作不能越组或隐式改变负责人、经营参与人、别的专业或别的 Project。

设计／参与允许多人，同人不同角色或不同专业可共存；同角色／专业／Person active 唯一，复核／审核每专业各一，审定／概算每项目各一。更换专业若目标已有配置，拒绝隐式覆盖，不能模仿原部分成功缺陷。结束只追加结束生命周期，重入用新 ID；未变化的事实保持原 ID 和快照，不作无意义取代。

内部 planner 接受 Project 自有当前快照和服务端已精确解析的 People／目录结果，不接受 IAM 账号、客户岗位或未经复验的显示名作为事实。它只返回需要结束的 ID 和需要新增的快照，不提交数据库、签发身份、制造 audit 或证明并发安全。正式服务必须先以原命令 hash 查已提交回执，再调用上游；raw 意图 hash 排除解析后的姓名／时钟，重放不依赖来源在线。

候选持久化在单一 Project 数据库事务内锁定命令和 Project、检查 revision／唯一性，追加新分工／结束历史、Project revision、回执及可信 Actor 事件。no-op 仅记原命令回执，不新增事实／Project revision／事件；结束记录只允许一次追加 ending。原生服务先查已提交回执，再调用 People，真实 HTTP 源验证 Tenant、ID、active、revision 和名称；People 故障不得形成空人员或部分提交。未知回复续相同命令，409 明确复核，不自动改 revision 重试。取消项目不可更改，普通 draft／pending／active 均不凭页面位置改变能力。

候选 migration `000012_reference_oa_production_personnel.sql` 仅新增五张 Overlay 表及相关约束；11→12 不自动给任何旧项目选装、不改旧分工和历史。代码默认不注册这些 HTTP 路由；只有显式配置 `REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE=reference-oa-production-personnel-v1`，并通过受保护命令给指定项目选装后才能使用。Main 当前仍为 schema 11；新二进制、migration、IAM 目录／策略、工作台及恢复必须作为匹配发布闭包另验。

## 精确权限与消费门禁

候选 Capability Manifest `application_version=0.8.0` 发布四种原生 action：`project-workspace.reference-oa-personnel.read/manage` 与 `project-workspace.reference-oa-discipline.read/manage`，仅已在隔离真实 IAM 中签发验收，Main 目录和 Grant 未改变。人员读／管使用 Project scope，服务也接受明确授予的 Tenant scope但不自动生成；目录管理独立要求 Tenant 原生能力，不能从旧 assignment.manage、Project read、身份超级管理员或项目人员配置权继承。

| 候选路径（均在 `/api/v1` 下） | 能力和意图 |
| --- | --- |
| GET `/projects/{id}/reference-oa/production-personnel` | personnel.read；当前事实及可选 `include_ended=true`，返回确切 Project ID／revision／allowed_actions |
| POST 同路径 | personnel.manage；完整 `replace_discipline`／`replace_reviewers`／`end_discipline` 意图，拒绝 Actor／姓名注入 |
| POST `/projects/{id}/reference-oa/production-profile` | personnel.manage；显式命令 ID／expected_revision 选装，不能读取时自动选装 |
| POST `/projects/{id}/reference-oa/participant-applicability` | personnel.manage；显式 `not_applicable`，不自动清除任何分工 |
| GET `/projects/{id}/reference-oa/production-disciplines` | personnel.read；为该项目只读发现有效专业，不扩目录管理权 |
| GET／POST `/reference-oa/production-disciplines` | discipline.read／manage＋Tenant scope；读含独立 allowed_actions，写 create／update／archive／reactivate＋command／revision |

原 UI 的 `user_id` 仅兼容控件形状，实际值为 People Person ID，不是 IAM 账号。两个权限发现分别读取确切 Project 人员端口和受保护 Tenant 目录端口，不把按钮位置或名称变成授权。独立适配器命令只发原生引用和完整意图；当前可信会话／项目作用域的续办存储不含凭证或 Actor。结果不明确时保留原命令／revision，刷新／reload 后显式“继续上一笔未明确结果的保存”，不得重新构造当前意图并重复写；已明确拒绝的命令解除待办，换意图不覆盖未明确结果。

BF-105 的新引用已候选实现，仍只在上述显式服务 profile 下注册，不改旧 Assignment v1：

| 新只读路径（均在 `/api/v1` 下） | 契约 |
| --- | --- |
| `/references/projects/{project_id}/reference-oa/production-assignments` | schema 1，source_application=project-workspace，profile=reference-oa-production-personnel-v1，resource_type=reference_oa_production_assignment；最多1000活动分工，超过则失败，不截断 |
| 同路径 `/{assignment_id}` | 相同原生 personnel.read／确切 Project 范围，可读 ended 以拒绝新业务且保留消费者历史 |

每项含 Tenant、Project ID／当前 revision／status、分工 ID／lifecycle revision、role_key、Person ID／revision／冻结 name、可选专业 ID／revision／冻结 name、state、eligible及参与不适用标志。活动 lifecycle revision 为创建时 Project revision，ended 为结束时 Project revision；不要把整体 Project 当前 revision 当分工自身 revision。eligible 只按 active 分工和 active Project，不因参与不适用而结束或取消候选。Store 以 repeatable-read只读事务提供一致 Project／分工，读取零写；服务不拥有奖金。

列表路径同时承担有界候选发现：Project 真实存在但尚未选装生产人员 profile 时，返回带确切 Project revision／status 的 `200` 空 `items`，不自动选装、不新增分工，也不把业务前置条件误报为引用损坏；单条分工路径在该状态下仍返回 `404`。这只定义 Project 自有只读端口的空集语义，不授予 Bonus、Workbench 或 IAM 任何额外权限。

Bonus v2仍只接受旧真实 Assignment，旧 JSON／hash／角色检查不变；显式 v3 才接新 source-qualified引用并另验 People。新消费、正常工作台及私有 Nginx→Gateway20场景已分层通过，见[连续报告](../../reports/oa-professional-bonus-continuous-validation-20261004.md)。Main、全角色、Suite恢复和其他生产阶段／适用性仍未完成。请求前先保存 uncertain意图，只有首次明确拒绝可解除；未知后遇到新撤权或成功后的复读失败仍保留原命令。

## 验收账目

原页面已有[16 条 UI-only 候选证据](../../reports/oa-production-personnel-ui-candidate-20261004.md)，不可混作原生业务。内部差量、PG18 原子／并发／11→12／单库恢复、正式 IAM＋真实 People 原点击及重启另见[原生报告](../../reports/oa-production-personnel-native-validation-20261004.md)。本轮新引用／Bonus v3／正常工作台私有链20场景及全量真库已完成；权限跨角色／跨租户矩阵、原站现场、联合恢复与公网匹配发布仍待验，D01及完整OA保持未验收。
