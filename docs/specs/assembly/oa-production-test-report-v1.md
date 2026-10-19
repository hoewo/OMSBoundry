# 原 OA 试验／报告原生切片 v1

状态：私有原生候选已通过；Main／公网／正式 D1 与完整 OA 未通过。关联[原生产阶段逐动作对照](./oa-production-stage-original-operation-map.md)、[阶段资料集合](./oa-production-stage-materials-v1.md)、[小系统边界演变账](./small-system-boundary-evolution-ledger.md)与[实际验证报告](../../reports/oa-test-report-native-validation-20261006.md)。

问题 A → 可验证结果 B：原 OA 只给勘察项目显示“试验”，而咨询／勘察／测绘共用“报告”；如果继续把所有 Stage 当成通用列表，Production 会失去类型边界，或每次读取都同步依赖 Project → 在同一 Production Work Item 中显式演进到不可变 v6，按经 Project 精确 revision 验证后冻结的最小类型快照决定适用性，直接复用原两个组件完成多主文件保存、复读、预览下载、移出留史和恢复，正常读取不再调用 Project。

## 原 OA 事实

- `SurveyTestStagePanel` 只在勘察项目出现，允许一次选择多份试验资料；没有校审单和评分。
- `ConsultationReportPanel` 的“报告”由咨询、勘察和测绘共用，允许多附件、保存后预览／下载／移出；没有方案阶段的校审和评分。
- 设计项目不出现试验／报告；咨询、勘察、测绘也不出现设计专有的初设、施工图、变更和竣工。
- 页面项目类型来自 Project 当前事实，但 Production 的历史阶段适用性必须能在 Project 后续改型或离线时复原，不能依赖每次页面读取时重新推断。

## 目标边界

- Project 继续独占当前 `project_type_keys` 及其 revision；Production 不获得项目类型的当前修改权。
- Production 在创建 v6 或显式 5→6 时，通过已有 `project-workspace.read` 工作负载能力精确验证 Work Item 已绑定的 Project ID／Tenant／source revision／active 状态，然后只冻结 `design`、`consultation`、`survey`、`mapping` 四类中实际存在的键，作为该 Work Item 的不可变适用性证据。
- 冻结快照由 Production 自有数据库保存，因为它解释 Production 历史的 Stage 状态；它不是 Project 当前主档副本，也不向 Project 反写。正常 Production 读取、资料写入和完成投影只使用该快照，不建立逐请求同步链。
- v6 的适用规则为：`scheme` 对四类共享；初设／施工图／变更／竣工只适用于设计；`test` 只适用于勘察；`report` 适用于咨询／勘察／测绘。非适用阶段固定投影为 `not-applicable / project-type-snapshot`，不能被页面写成完成。
- Production 继续拥有 Work Item、Stage、File／Version 业务引用、ending、状态与历史；Evidence 独占文件身份、版本、字节和访问；IAM 继续签发已有原生能力；Workbench 只装配原组件。
- Contract 不参与本切片。报告或试验资料不是合同事实；未来若产生合同承诺，只能经显式连接器提交 Contract 原生命令。

## 兼容与失败恢复

- `reference-oa-stage-set-v1@2`～`@5` 含义永久不变；v6 只允许 5→6。旧 v2 必须按 2→3→4→5→6 顺序演进，不跳级、不降级、不批量改写旧 Work Item。
- 读取试验／报告页面不会升级；第一次真实写才发出持久、幂等的升级子命令。升级成功后即使 Evidence 上传或资料登记失败，v6 与类型快照保持，页面只继续原命令的未完成段。
- Project 校验失败／超时发生在 v6 本地事务前，返回可重试 503 或引用错误且不写 snapshot／upgrade；成功回复丢失时重放同一个 Command，不能重新解释当前 Project 类型。
- Project 后续类型变化不会静默重算已有 Work Item。需要重新分类时必须设计显式的新 Production 版本或迁移动作；不得 UPDATE 已冻结快照。
- test／report 只接受主文件，服务端基于 Evidence 可信显示名复验扩展名；移出只结束 Production 引用，不删除 Evidence 字节。

## 验收门槛

1. 直接复用原 `SurveyTestStagePanel` 与 `ConsultationReportPanel` 及其样式，不重写近似表单；页面类型条件与原 OA 一致。
2. v5 读取零写；第一次真实试验保存恰好一次 5→6，Work Item 身份和 v5 历史保持，并冻结精确 Project revision 的类型键。
3. 勘察试验与共享报告均可多主文件保存；review sheet／score、错误类型阶段、错 Tenant／Project／revision 必须失败关闭。
4. 正常回读不访问 Project；Project 后续不可用不影响已确认 v6 资料读取。Project 只在创建／升级边界被调用一次。
5. 预览／下载返回真实 Evidence 字节且零 Production 写；单份移出留下 ending 并保持其余文件。
6. 正式 IAM 撤权后页面只读、服务端 403 且零写；五所有者逻辑空目标恢复后同 ID／revision／类型快照／历史／字节继续。

## 2026-10-06 实际结果

最终全新夹具 9／9 场景通过：既有 v5 保持、原试验面板读零写、一次 5→6 并冻结四个项目类型、试验两主文件、共享报告两主文件、真实字节预览下载、单份报告移出、正式 IAM 撤权，以及 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复后原 UI 继续。终态为 1 Work Item、7 Stage、4 material、1 ending、5 command、1 upgrade、4 type snapshot、0 score；Project／People 摘要保持，Contract 未进入依赖图，页面错误为零。

前两次验收暴露的是夹具与验收判据问题：低资源远端 PG 经 SSH 的 3 秒引用超时把 IAM 工作负载换证延迟表现为 503，而测试又把仍在待选列表中的文件名误判为保存成功，继而读到 v5 的 422。专用夹具保持有界超时但调整为 15 秒，逐次记录换证状态／耗时，并改为等待原组件成功消息后再检查持久化；全新夹具中 Project 换证和四次 Evidence 换证均为 200，随后完整重跑通过。没有把失败轮次拼进最终结果，也没有通过业务自动重试掩盖来源故障。

这次证明“适用性快照”可以作为 Production 自有的历史解释证据，同时 Project 仍拥有当前类型；它是一个明确的边界扩展，不是零变化。Production 整体仍只是候选边界，Main／公网、正式 D1 archive／keys、双 Tenant、全角色、完整多类型进度和跨客户稳定均未证明。
