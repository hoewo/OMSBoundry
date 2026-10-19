# 原 OA 生产人员界面候选

2026-10-04。本报告记录本地 UI／交互测试端口层；最终 16 条有限通过，不是原生或公网证据。随后新增的原生切片单独见[原生验证报告](./oa-production-personnel-native-validation-20261004.md)，D01 未整体验收。

## 本批结果

问题 A→结果 B：通用负责人界面不能表达原专业人员操作 → 直接复用原生产人员 Manager、逐字 CSS、专业／审定抽屉、专业目录抽屉和共享选人／确认控件，在显式测试端口逐动作验证。

只读来源提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`，六份原文件摘要实际一致；`ProductionPersonnelManager.css` SHA `0bc0b4c247c96487e58b2fd80762783a70070bdf2bad0b2411889723f0206db0` 与原文件逐字一致。[来源回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/source-provenance.json)不证明改造后与原站现场完全等效，原站未运行。

保留按专业的设计／参与多选、复核／审核单选，审定／概算独立位置，600px 两种选人抽屉、专业管理嵌套抽屉、原列表／姓名显示／确认窗。原“人数”仍计配置行，不是去重人员。没有把生产负责人放进这些专业角色。

真实来源和保存尚未接通。本批 `ProductionPersonnelUIPort` 是明确的界面回调，不是版本化业务 API；测试服务只有内存名单，`user_id` 也只是原控件兼容形状，不表示创建 IAM 账号或合并 People。专业保存改为一个整组意图，审定／概算改为一个双字段意图，移出改为一个专业意图，不复制原 UI 的逐条提前启动请求；原子提交、幂等、真实身份快照和历史仍须由未来原生端口证明。

## 实际检查

[浏览器回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/browser.json)为 16／16 场景、零页面脚本错误；8 次成功测试端口命令，最终 20 条内存行、18 条结束行、2 条项目级配置。这些数字不是业务数据库、原生历史或审计数量。

| 检查 | 本批证据范围 |
| --- | --- |
| 原空页／专业必选／取消 | 隐藏候选零读，必填错误零写，600px 抽屉取消零写 |
| 新增／编辑／更换专业 | 同人多角色、设计多人、原专业四列、明确旧／新专业意图，正常按钮保存后回读 |
| 审定／概算 | 独立两个位置、同人可选两角色，未保存修改取消不写 |
| 参与不适用 | 保存和组移出均明确 `preserve`；关闭不适用后恢复同一参与行，再显式结束，不能因隐藏清除 |
| 候选／专业目录故障 | 禁用保存、明示错误、不当空目录，正常重试和取消零写 |
| 保存／移出拒绝 | 保存保留原表单；移出失败保留原组并显示错误，零页面未处理 promise；恢复后正常按钮继续 |
| 只读／读取失败 | TEST 权限刷新关闭已有抽屉、隐藏管理动作；名单读取失败不显示为空；重读及 reload 零写 |
| 小视口 | 640×900 中取消按钮经过正常 actionability 等待、实际点击可达，无 force／遮罩穿透 |

权限刷新那一步调用公开 UI 生命周期方法模拟测试 Provider 变化，不是点击被遮罩挡住的背景按钮，也不是正式 IAM 撤权证明。普通业务操作均实际点击；无 `force` 或 DOM 事件注入。

已实际看图：[专业原抽屉](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/discipline-original-drawer.png)、[专业四列](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/professional-roles.png)、[审定／概算原抽屉](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/reviewers-original-drawer.png)、[小视口](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/small-original-drawer.png)。TypeScript 实际通过；最终 UI-only 独立 Vite build JS SHA `98a8bf7fc0fa3e9bf727860fbb94e7342133ff1ee9c47a9d97879fe7b90ab84f`。正常 Workbench 未导入本候选，bundle 仍为 `7df15d8a63a0ac31a308401631aaedba38cbb2f73ba5d1334c4d9e88ed41c357`；主站人员页签仍禁用。

## 修复和保留的失败

首轮暴露原 `DisciplineSelector` 不传 Form ID，标签不能关联控件。目标只增加 ID 转交，不写原仓库。名单初次读取增加 loading，失败不作空名单；目录故障禁用保存；权限只读关闭编辑抽屉。原共享删除按钮不等待 promise，本切片交由 mutation 回调处理失败，避免页面未处理拒绝，不宣称已经有原生撤回或命令恢复。

其余失败为测试定位或时序：图标进入 accessible name、重复的必填消息、已有选项覆盖输入区域、错误 relative locator、关闭嵌套抽屉后 live `.last()` 指向父层、背景刷新被正常遮罩阻断、动画未结束前量位置。八份失败 JSON 全部保留在[证据目录](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/)，没有通过强制点击或放宽授权消除反例。

另保留原 15 条回执并明确取代：旧测试在组移出时错误地结束了隐藏参与人，不能当通过。最终第 16 条验证保存与移出均保留、重新适用恢复原 ID 后再显式结束；[取代说明](../../acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004/superseded-hidden-end-test-contract.json)。

复演：`node acceptance/oa-initiation-repair/verify-production-personnel-ui-browser.mjs "$(mktemp -d /tmp/oa-production-personnel-ui.XXXXXX)"`，然后前端 `npm run check`。驱动用独立临时构建目录，不写正常嵌入 bundle；收集脚本只整理本轮固定路径，不能替代重新运行。凭证、SQL、备份和构建树没有入证据目录。

## 接下来才能关闭 D01

[原操作与门槛](../specs/assembly/oa-production-personnel-clone-gap.md)继续有效。原 monolith 的专业目录为可新增、编辑及停用的全局字典，分工归 Project member；适用性来自独立 `oa2026/applicability` 路径，并非人员密码字段。目标仍须明确 Project 自有有界扩展、专业目录归属、版本／快照、整组原子或分段续办、精确能力／范围、兼容引用和 Bonus 消费，保留全部旧分工与旧策略。

这一测试端口层不证明原生新增／更换／移出、并发单值唯一性、People revision／状态复验、真实历史／审计、IAM 全角色／跨租户、未知回复、恢复及发布。后续哪些原生检查实际完成以独立报告为准；不能将 16 条 UI 检查关闭 D01、生产整页或完整 OA，也不将真实保存门禁改为假数据演示。
