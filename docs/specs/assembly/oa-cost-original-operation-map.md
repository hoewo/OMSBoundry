# 原 OA 成本操作对照与接线缺口

2026-10-04 07:36 UTC，原源码只读操作对照；下表保留接线前差距，不代表当前全部未实现。BF-101 的七类／两侧扩展和原组件已在 Cost schema 3 上线，Workbench 当前 `.6` 已修复连续撤回弹层遮挡。C 成本原生 IAM 授权按批准执行，管理员有限公网连续操作及经营七类／生产五类适用组合、十二记录／24 原生命令收尾通过，旧四条及历史、其他十业务库、Evidence 全部事实／审计和两类 Grant 保持。全角色和整体 OA 未通过。实际证据见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md)，规格见[有界扩展](./oa-cost-receipt-overlay-v1.md)。A→B：用户在同一项目的经营或生产页记录、调整并查看真实费用 → 原列表／抽屉能够保存、复读、留史和安全续办，而不是只有成本总额或不可操作的空页。

## 原操作的事实

参考提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`。直接复用目标是 `ProductionCostList.tsx`、`ProductionCostForm.tsx` 和原 CSS／附件／行内预览布局，不重新设计一套界面。源码摘要：List `d5a3b80ed66e6929d74e3a0d2a37b65f9255c5c7e40091d218d2538338c31635`；Form `2e1a8318f098ddb239b61add48e938142a2427f0a932647fd4f1089d82269ea2`；`services/production.ts` `dce3c3c758888f8ccf1ea6455ad8ee33bfee410f855d8e3db777b2bdfa68019f`。

1. 进入经营／生产成本，按 scope 读列表、总金额和记录数；点“记录成本”或原记录，打开 640px 抽屉。
2. 选择类型、金额，填写可选描述、发生日期；可排队上传多个 PDF／PNG／JPG 发票文件。
3. 提交新建或保存修改；成功关闭，刷新列表／统计。取消不能保存。已存记录可进入查看／修改，管理权限才有新增及删除按钮。
4. 原记录首张发票有预览／下载；编辑表单可移出文件、恢复显示；预览打开时抽屉扩至窗口宽，文件与表单并列。
5. 原删除为 FinancialRecord DELETE，提示不可恢复；原上传异常还会删新文件。这些物理删除／覆盖行为不能照抄到本架构。

原经营列表包括 `cost + cost_scope=business` 及遗留 `financial_type=expert_fee`；生产列表仅 `cost` 且 scope 空或 production。遗留专家费与显式 scope 的优先关系必须在导入／连接器中确认，不能将另一侧费用并进来。原服务计算总额使用 number，目标必须改为精确最小单位；不复用其浮点聚合或 FinancialRecord 共享表。

## 不能用换标签掩盖的差距

| 原 OA 事实／操作 | 接线前通用 Cost 能力 | 当时必须解决的差距 |
| --- | --- | --- |
| business／production 分别列出、统计、授权 | CostEntry 只有 Project 快照，无业务侧别 | Cost 自有版本化客户扩展持久化侧别与适用性；未知旧记录不能默认归经营 |
| 用车、住宿、交通、专家费、投标报名费、中标服务费、其他七类 | vehicle、accommodation、public_transport、other 四类 | 专家费／报名费／服务费不能塞入 other；交通映射要显式；两种投标费只在经营可选，专家费按侧别显示评标／方案评审 |
| 原用车不填里程 | 原生 vehicle 必须 distance_meters > 0 | 不伪造 1 米，不静默改变已有不变量；需明确客户扩展与原生用车语义如何共存 |
| 描述、日期可不填 | 原生 description 非空、occurred_on 必须有效日历日 | 不补“其他”或当前日期冒充用户输入；需显式规范可空的客户事实或明确界面差异 |
| 金额 number、显示小数元，总额按 scope | 原生 int64 最小单位、amount.read 脱敏 | 精确字符串输入／wire／投影、同一分类与权限聚合；缺金额权限总额也不可推算 |
| 原编辑覆盖、删除不可恢复 | Correct／Withdraw 追加 revision、原因、command_id；自有历史 | 复用原表单时明确“更正／撤回留史”，冻结旧金额／分类／引用，不能称物理删除 |
| 原发票多个 File ID，无 Version；失败删物理文件 | EvidenceRef File／Version／slot，记录／更正时复验 | 既有 Evidence SDK 完成身份先保留；业务引用失败复用同命令，移出或撤回不删字节 |

这些是接线前原源码与通用模型核对得出的实施缺口。后续 BF-101 和 Cost 自有 typed Customer Overlay 规格已登记并实施具体语义、兼容演进与授权；不把客户分类放入通用内核、不由 Workbench 持久化侧别、不让 Money 拥有成本申请、不共享原 FinancialRecord 表。

## 纵向接线验收顺序

先补一个明确侧别的新建→原生复读→原列表，再做更正／撤回／历史，最后接 SDK 文件。每段验证 command 重放、未知回复续办、409 保留意图、403 不换身份、来源停机零写、相邻 Project／Tenant 拒绝、只读／脱敏零写；七类型与经营／生产分开统计必须独立反例。用车、可空日期／描述和旧记录归属在 owner 规格落实前不能假称原 OA 等效。数据库迁移须证明旧 ID／revision／金额／审计／引用不变，空目标恢复后仍可续办。候选组件和真库通过后仍需匹配 IAM policy、部署兼容闭包及同项目公网旅程。

本映射初次登记时尚未复制／发布成本组件。当前 `.5` 已复用原组件并显式选装 Cost schema 3 新端口；管理员窄 Scope 有限公网主链已有证据，全角色及整体 OA 未关闭。原 OA 源目录始终只读。范围仍是[完整 OA 清单](./oa-complete-operation-delivery-ledger.md)，成本不是新的总目标。
