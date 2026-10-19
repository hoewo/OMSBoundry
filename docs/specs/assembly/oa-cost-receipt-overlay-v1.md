# 参考 OA 成本记录扩展

2026-10-04 07:36 UTC，实施规格。七类／两侧扩展、原组件、普通测试和分批真实 PG18／IAM HTTP 已实现并验证；匹配发布通过升级／空目标数据恢复上线，Cost schema 3 显式选装。Workbench `.6` 已修复连续撤回弹层遮挡，候选十二场景与公网正常点击续原记录分别记账。C 两侧原生 IAM 授权按批准执行，管理员有限公网新增／纠正／附件预览下载／撤回留史／两侧隔离／正常刷新原命令续办、十二分类适用组合和原生收尾已通过，共十二记录／24 命令；全角色和完整 OA 未通过。实际状态以[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md)为准。问题 A → 可验证结果 B：同一项目经营或生产页记录原 OA 七类费用 → 原列表和表单能保存、复读、追加更正及撤回，两侧不混账、金额精确、文件和历史保留。

## 所有权和兼容

Cost 在自己的数据库中拥有独立、类型化 Reference OA Cost Entry v1，不是通用 CostEntry 的另一组标签。旧四类、用车里程、必填日期／描述、API、ID、revision、建议和审计保持原规则；不创建影子通用费用，不将专家费／投标费塞入 other，不伪造里程／日期／描述。Workbench 不存成本事实，Project、Money 和 Evidence 所有权不变，原 OA 共表和目录只读。

显式选装 `REFERENCE_OA_COST_PROFILE=reference-oa-cost-v1` 才注册 `/api/v1/reference-oa/cost-entries`。新增专属表和前向 migration 不改旧表，未选扩展仍可使用旧应用；不建设任意字段或插件平台。

## 事实和命令

稳定 UUID／Tenant、创建时 Project ID／revision／名称快照、不可变 business 或 production 侧别、recorded 或 withdrawn 状态、递增 revision。七类为 vehicle、accommodation、transport、expert_fee、bid_signup_fee、bid_service_fee、other；后两类仅经营侧。专家费的评标／方案评审中文显示由界面决定，不是两条费用。CNY 是此封闭客户版本的明确币种，金额用十进制字符串传整数分，内部 int64，禁止浮点输入／合计。日期／描述可 null；非空日期须有效，文字有界；用车不含里程事实。

新建、更正、附件变更、撤回都要求 command_id；后三者还要求 expected_revision 和原因。摘要、不可变 revision 快照、命令回执和审计在本库原子提交。同 Actor／同命令／同意图重放返回原回执，不依赖 Project／Evidence 在线；改 Actor／意图、旧 revision、撤回后修改均拒绝。重放仍先执行当前权限。新建显式验证 Project 快照，后续不覆盖历史快照。撤回／移出只结束业务引用，不删除记录、历史或 Evidence 字节。付款、审批、奖金及原 FinancialRecord 迁移不属本切片；旧通用费用侧别未知，不能默认为经营或生产。

## 原生授权

复用 Cost 的 create、read、amount.read、correct、withdraw、evidence.manage、history.read 稳定动作，不按 OA 岗位或按钮授权。新增资源类型 `reference_oa_cost_entry`、`business_cost_project`、`production_cost_project`；后两种 ID 是 Project ID。旧 project Resource Scope 不自动扩为新两侧权限；Tenant 级明确成本授权覆盖本租户成本。查询／更正不得换侧别扩大 Scope，单条 scope 取已保存事实。IAM Catalog 显式声明新资源类型，IAM／Policy Provider 决定签发，本切片不写人类 Grant 或暗改 Assignment Policy。

Cost 返回并执行 allowed_actions。没有 amount.read 时当前、历史金额和总额都为空；不能从统计或缓存推算。Evidence 引用只保存 File ID／Version、显示名和用途，经配置端口复验精确版本的完成状态；业务应用不保存物理地址或签名 URL。

## 原组件和验收

直接复用 ProductionCostList、ProductionCostForm、CSS、AttachmentUploader、InlineFilePreviewLayout，只换版本化数据／权限／文件端口。覆盖／删除改为追加更正／撤回留史；保留排队多文件和表单内预览，失败不清理新字节。上传与费用登记分别确认，未知回复续原命令，冲突保留意图等待核对。

先验明确一侧的新建→原生复读→原列表，再验七类适用性、两侧隔离、旧 project Scope 拒绝、金额脱敏、null 字段、历史／撤回、文件预览下载、409／403／停机、同命令续办、双 Tenant、只读零写。单元、真库、独立启动、旧 schema 兼容／空目标恢复、匹配 Cost／Workbench／IAM 发布及同项目正式入口分别记证；组件内存端口或成本一栏通过都不能宣布完整 OA 完成。
