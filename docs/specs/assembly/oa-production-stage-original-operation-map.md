# 原 OA 生产阶段：逐动作对照与下一闭环

最新实施：共享方案 v2、初设 v3、施工图 v4、变更／竣工 v5 的原 UI 切片已分别形成私有真实链证据；试验／报告又以显式 v6／5→6、原两组件和冻结 Project 类型快照通过 9 场景，[最新报告](../../reports/oa-test-report-native-validation-20261006.md)。这些证据仍不代表完整多类型进度、原站现场、Main／公网、全角色或完整 OA 已完成。

2026-10-04。来源只读 commit `826e0fc65aa09947fc4f64f62a2644fa5d11a302`。本轮已读取原方案面板／抽屉／阶段文件控件／传输 helpers、生产文件服务与类型进度算法，以及目标 Production 的模型／类型定义／服务。只有源码对照，不是原站现场验收，也不是目标阶段功能已经接通。

问题 A→可验证结果 B：项目里只能看见通用生产任务、不能按原 OA 维护阶段资料 → 在同项目原阶段 UI 中新增多份主文件、可选校审单与评分，保存、复读、预览下载、移出留史及失败原命令续办均有真实 owner 回执，且不改变旧任务、文件、分工和奖金事实。

## 已确认的原动作

| 动作 | 原代码事实 | 目标验收门槛 |
| --- | --- | --- |
| 打开方案阶段 | `SchemeStageFileManagement` 使用 `StageFilePanel`；不是跳转另一个小系统 | 复用面板、字段、抽屉和 CSS；当前项目上下文不变 |
| 新建／编辑 | 空状态“新建阶段资料”，非空“编辑阶段资料”；抽屉常态 800px，预览时展开 | 按最终原生 `allowed_actions` 显示；无权／来源故障不是空数据 |
| 主文件 | 多文件列表；初次保存至少一份；已有主文件时可只追加其他资料或修改评分 | 不以一个 `primary-output` 替换槽实现多份列表；主文件有独立业务引用 ID |
| 校审单 | 独立可选多文件列表，最多 20；编辑器读取 `review_sheets`，旧单值回退 | 与主文件引用区分；仅校审单不计主成果齐备 |
| 评分 | 可选、0～100；0 是有效值；原 InputNumber 未限定小数位 | 不使用 truthy 判断丢失 0；原精度尚无稳定上限，不自行限定整数／两位小数 |
| 保存 | 原抽屉依次上传并登记主文件、校审单，再单独更新评分；成功清空待传列表并回读 | 原视觉可复用，但这些原请求不是事务；目标需整批意图或显式分段回执与续办，不能重传已经确认段 |
| 预览／下载 | 原面板及抽屉支持已保存文件和本次待上传文件预览；CAD 提示下载 | Evidence 精确 File ID／Version；复用现有传输边界，不由 Production 签 URL／读 OSS |
| 移出 | 每份有原确认窗；原 ProductionFile 为删除／软删，原附件关联删除 | 目标追加撤回／结束及原因、保留快照和历史，不删 Evidence 字节／Person／Project |
| 取消 | 原取消清空未提交列表与预览并关闭 | 无业务写；已经确认上传与未保存引用须分别说明，不谎称字节回滚 |
| 刷新进度 | 原 `productionStageComplete` 看有效主文件存在；校审单和评分不是必需；顺序算法显示第一个缺口为 current | 只读投影按真实资料计算，不复制原 GET 内的项目状态写入，不反向改 Project |
| 多类型 | 原四种类型各有一条进度线，但文件 API／查询仅以 Project＋Stage 定位；方案资料被各类型共用 | 一个项目只有一份可写方案集合，不增加类型选择或复制文件；各类型进度后续只读消费同一身份 |

原源码路径：`frontend/src/components/production/{SchemeStageFileManagement,StageFilePanel,SchemeStageEditModal,ProductionStageFileField}.tsx`、`fileTransferHelpers.ts`；`backend/internal/services/production_file_service.go`（创建／分类／评分／更新／移出）、`backend/internal/oa2026/service.go`（类型阶段注册及完成投影）。原 StageFilePanel 只显示最新单个 `review_sheet`，抽屉可见全部 `review_sheets`；这项不一致要保留为对照差异，不凭印象换语义。

## 后续阶段已确认差异（源码层，不是目标已实现）

| 工作面 | 实际原来源／键 | 不可误套方案的差异 |
| --- | --- | --- |
| 初步设计 | `PreliminaryStageFileManagement`／`PreliminaryStageEditModal`；文件键 `preliminary` | 私有原生候选已实现；800px 原抽屉、多主文件／可选校审／评分，独立取消／回读；v2 只读、首次真实写显式升级 v3；Main／完整类型进度仍未闭合 |
| 施工图 | `ConstructionStageFileManagement`／`ConstructionStageEditModal`；文件键 `construction` | 私有原生候选已实现；原800px抽屉、多主文件／可选20校审／可选评分，已有主文件允许只追加校审或改分；v3 第一次真实写显式升级 v4，原分段保存改为有回执可续办的升级／Evidence／Production 子命令；Main／完整类型进度仍未闭合 |
| 变更／竣工 | `ChangeFileList`＋`CompletionFileList`／`AuxiliaryFilePanel`及两套Upload；文件键 `change`／`completion` | 私有原生候选已实现；原两套列表／620px 抽屉、无校审／评分、不同扩展名与竣工完成映射由不可变 v5 承接。原位替换改为新引用登记＋旧引用 ending；Main／完整类型进度仍未闭合 |
| 试验 | `SurveyTestStagePanel`；`oa2026` 的 `survey/test` | 私有原生候选已实现；只对勘察，多主文件、无校审／评分，v5 第一次真实写显式升 v6 并冻结 Project 类型证据 |
| 报告 | `ConsultationReportPanel`；文件键 `consultation`、进度键 `report` | 私有原生候选已实现；咨询／勘察／测绘共用，原多选附件、保存后预览、精确下载／移出，不套方案评分 |

上述原组件、`ProductionInfo` 栏目条件及 `oa2026.productionStages`／`legacyProductionRequirement` 已实际读取。目标没有复制原文件替换的原位覆盖或并行部分成功后重传整个队列，而以不可变引用、原命令续办和 v6 类型快照完成当前切片。原进度优先类型化 `oa2026_stage_files`／StageEvidence，再回退共享 ProductionFile；不能笼统说所有原路径都共享。BF-106／112／113／114／115 已承接六类可见阶段路径；下一步必须把它们组成原四类型的完整进度，不在 Workbench 建第二套可写真相。

## 不得直接照抄的部分

原方案保存是多次写入，中途失败会留下已保存主文件；原评分更新选择最新一条资料原位更新。原 `GetProgress` 会保存跨经营／生产的整体项目状态，原查询又直接读各业务表。目标必须保留原操作效果，但不能复制共享表、GET 写业务状态、静默改历史、前端提供 Actor 或 Production 管理文件字节。

原当前校审单／评分均可选，早期“校审单必填”说明不能作为当前实现依据。评分清空的原表单输入不会发送 null 更新，原服务也没有显式 clear operation；不得把用户删掉输入自动解释成撤销既有评分。独立清除语义须在实现前单独固定并标注原差异。

## 与当前 Production 的真实差距

当前 `design-v1@1` 与 `consultation-v1@1` 是可用旧 Base：单个 `primary-output` 槽、显式负责人、显式完成／重开和自身 revision／历史。已经取得的 C4 任务生命周期证据仍有效，但它们**不是**原阶段多文件／独立校审单／可选评分的 UI 等效证据。

仅给方案 tab 换原排版、将多份文件逐次替换同一个槽，或保存文件后偷偷调用旧 complete，都不能关闭这项缺口。旧 Work Item、definition version、command hash、历史和旧引用必须保持；同一业务事实不能再建另一套平行可写“阶段真相”然后同步。

Production 继续拥有生产任务／阶段／成果业务引用；Project 只拥有项目类型和项目分工，Evidence 拥有文件身份／版本／字节，IAM 拥有原生能力，Workbench 是装配。客户固定阶段规则要显式版本化，不能追加任意 JSON 字段、可编辑工作流或无限角色目录。2026-10-04 复核原 `getProductionFilesByStage`、`GetProductionFilesByStage` 及 `oa2026.productionStageComplete` 后，纠正早期将集合按类型拆开的推断；BF-106 的项目共享集合规格必须先于正常入口接入。

## 下一功能的关闭条件

1. 固定“旧 Base 保持、新资料规则只在显式版本中启用”的有界模型、事实唯一所有者、原生权限、精确引用和恢复协议。
2. 直接复用方案的原组件和 CSS，先验打开、多文件、可选校审单、0 分、取消、已有资料编辑、预览及小视口按钮；只能称 UI 层证据。
3. 方案至报告已分别完成同一项目真实 IAM→Project／Production→Evidence 保存；v6 又验证勘察专有试验和多类型报告的冻结适用性。下一步对四类型完整进度和概览映射重新验证，不从单 Tab 保存直接外推。
4. 中途上传失败、提交成功回复丢失、页面重载、精确撤权、错 Tenant／Project／类型／阶段、并发 revision 冲突；未知结果仅继续原命令，不自动创建下一批。
5. 真 PG18 兼容升级、独立启动、正常 Nginx／Gateway／Workbench、匹配 D1 联合备份及空目标恢复后同 ID／revision 继续。未经过匹配发布的组件不在 Main 声称可用。

方案、初设、施工图、变更／竣工、试验／报告现已分别形成私有原生候选。下一步收口原四类型完整阶段进度；批复审计、外委、资料统计、全角色和公网链仍在[完整交付清单](./oa-complete-operation-delivery-ledger.md)保持未关闭。不能用局部候选代替完整 OA。
