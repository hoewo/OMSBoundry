# 原 OA 阶段资料集合 v1

状态：原生有限候选已演进到 v6，并有方案16、初设9、施工图10、变更／竣工11、试验／报告9个私有真实场景及相应 PG18／五所有者逻辑恢复证据；未发布 Main，真实阶段 D1／keys、公网、全角色和完整多类型进度门槛尚未完成。关联 BF-106／112／113／114／115、[原逐动作对照](./oa-production-stage-original-operation-map.md)与[最新报告](../../reports/oa-test-report-native-validation-20261006.md)。

2026-10-06 BF-115 演进：显式 v6 在不改 v2～v5 含义的前提下增加勘察专有 test 及咨询／勘察／测绘共享 report。创建 v6 或 5→6 在本地事务前精确验证 Project ID／revision，然后由 Production 冻结四个受支持类型键的最小不可变快照解释 Stage 适用性；正常读取不再调 Project。原两组件、真实 Evidence、正式 IAM、PG18 和五所有者逻辑恢复通过 9 场景；Contract 未参与。见[规格](./oa-production-test-report-v1.md)与[报告](../../reports/oa-test-report-native-validation-20261006.md)。

2026-10-06 BF-114 演进：显式 v5 在同一 Work Item 增加可选 change 与必需 completion；二者仅主文件、无校审／评分，替换采用新引用＋旧引用 ending，服务端复验不同扩展名。v2～v4 含义不变，第一次真实写才 4→5。11 场景和五所有者恢复通过，Project 无变化，Contract 未依赖；验收抓到并修复数据库约束和阶段注册表两项兼容闭包缺陷。见[规格](./oa-production-change-completion-v1.md)与[报告](../../reports/oa-change-completion-native-validation-20261006.md)。

2026-10-06 BF-113 演进：显式 v4 在同一 Work Item 增加施工图，v2／v3 含义不变；既有 v3 只在第一次真实施工图保存时显式 3→4，旧 v2 可顺序 2→3→4 但不得跳级。原施工图组件、正式 IAM、真实 Evidence、PG18 和五所有者逻辑恢复通过 10 场景，前序初设保持且无新增依赖边。方案、初设、施工图第三个同类场景使本“阶段资料集合”子边界达到可重复；Production 整体仍为候选。见[施工图规格](./oa-production-construction-stage-v1.md)与[报告](../../reports/oa-construction-stage-native-validation-20261006.md)。

2026-10-06 兼容演进：v2 历史含义继续是“仅方案”；显式 v3 在同一 Work Item 增加初步设计，既有 v2 只在第一次真实初设保存时通过可审计、幂等子命令升级，不自动迁移、不降级。原初设组件、正式 IAM、真实 Evidence、PG18 和五所有者逻辑恢复已通过 9 场景。阶段资料集合子边界获得同类第二场景证据，但本规格的 v2 历史语义不被重写；细节见[初步设计规格](./oa-production-preliminary-stage-v1.md)与[报告](../../reports/oa-preliminary-stage-native-validation-20261006.md)。

问题 A → 可验证结果 B：旧生产任务的单成果槽不能承接原阶段多文件 → 在同一 Production 项目共享阶段集合中，以显式版本原子登记主文件、可选校审单和精确评分，刷新与丢回复续办不重复事实，移出保留历史及 Evidence 字节。

## 唯一所有者与兼容边界

Production 独占 Work Item、Stage、资料业务引用、评分以及为解释本 Work Item 历史适用性所冻结的最小 Project 类型快照。Project 仍独占当前类型／人员与其 revision；Production 不能改 Project 快照、不对外提供当前项目类型。Evidence 仍拥有 File／Version、字节和访问；IAM 最终签发原生能力，Workbench 不保存业务事实。

显式 profile `reference-oa-stage-materials-v1` 才允许新建 Production 自有阶段集合；当前新建使用 `reference-oa-stage-set-v1@6`，而 v2～v5 分别永久保留方案、增初设、增施工图、增变更／竣工的历史含义。这是项目共享资料集合的持久化身份，不是新增客户项目类型。v6 以冻结快照决定：方案对原四类型共享，初设／施工图／变更／竣工只适用设计，试验只适用勘察，报告适用咨询／勘察／测绘；用户不选阶段类型、不重复上传。非适用 Stage 不计缺口。集合 completed 只表示当前类型中必需阶段主文件齐备，不能作为完整生产／项目完成。后续各类型进度须只读消费同一集合；当前尚未实现完整投影。

完整装配的该开关要求Production、Project、People与Evidence四个所有者均选装；Workbench启动门禁、页面条件与D1检查采用同一要求。默认关闭不增加旧Suite启动前提；未知profile或缺所有者显式拒绝，不能让工作台静默启动后再显示空栏目。这是当前显式组合的直接依赖，不把Project设成其他独立应用的永久启动前提。

Base 默认和 `LatestTypeDefinition` 继续选择原 version 1，旧 Create JSON 及 hash 不变。新创建显式提交上述 key 和 `type_definition_version:2`，旧类型 @2、未知版本／profile 拒绝。数据库按精确 Tenant＋Project 事务锁确保共享集合与任何旧活动任务不能并排成为阶段真相；已有旧任务会真实返回并失败关闭，不自动复制、升级或取消，保留历史的转换另行制定。此前私有候选的按类型 @2 设计没有发布／Main 数据，因与原源码共享关系不符而撤换；那份 PG 回执不作为此修正版验收。

版本 2 的资料写入不走旧 deliverable／complete／reopen／applicability；旧 Design API 仅 version 1。原生 read 的任务基本状态不携带新资料／评分，集合及其历史需新原生动作。旧资料引用消费者尚不能表示新集合时失败关闭，不返回假空数组。当前 Production `ReferenceMaterials` 对v2返回类型无效，Project v1消费端还要求每个relation唯一、最多100项；这不是多文件集合的可用资料投影。匹配发布／完整OA验收还须明确版本化多文件来源端口与Project消费兼容，不能放松旧v1校验、丢掉同类文件或把方案原生UI通过当成资料页通过。

本切片只接方案 UI。勘察／测绘完整类型任务、后续阶段原 UI、适用性、批复审计、外委、奖金及完整进度不因此完成。不创建任意可编辑工作流。

已注册的 definition version 2～5 不在后续功能中原位增加阶段。初步设计使用 v3，施工图 v4，变更／竣工 v5，试验／报告 v6；升级只能 2→3→4→5→6。每一步均保留原 ID／前序 Stage／引用／命令历史；读取未纳入阶段零写，第一次实际保存才升级。v6 的 Project 类型快照与升级同事务提交，不允许缺失、更改或按当前 Project 重算。Main 原 version 1 活动任务的迁移策略也须先验，不能靠复制新项目避开。

## 事实和命令

资料记录包含独立 Reference ID、精确 File ID／Version／source revision、类别 `main-file` 或 `review-sheet`、服务端验证后的名称快照、创建 Work Item revision 与不可变 `batch_position`（每类每命令 1～20）。按 revision＋批内位置读取，不依赖相同时间戳下的随机 UUID；原摘要面板显示最后选择的最新校审单，编辑器保留全部有序集合。移出追加独立 ending，不能改写资料、删除字节或移出其他阶段。评分追加不可变记录，以十进制字符串表示 0～100，不限为整数／两位小数；请求有界最多 128 字符。空值／省略表示 preserve，不隐式 clear。

每批最多 20 主文件和 20 校审单；当前每阶段每类最多 100 个引用，读取发现超限失败而非截断。同阶段／类别／File／Version 的活动重复拒绝；已移出身份可重新登记为新引用。首次保存至少一份主文件，后续主文件存在时可只追加校审单／评分。最后一份主文件移出允许回到缺口，不删校审单／评分。只有主文件计齐备。

`POST .../stages/{stage}/material-collection` 接收 `command_id`、`expected_revision`、`main_files`、`review_sheets`、可选字符串 `score`。Evidence 逐项复验后，本地事务再次锁定 revision，并将所有引用、评分、阶段状态投影、Work Item revision、历史和 command receipt 一次提交。任一失败零部分事实；上传字节与业务登记是不同提交，浏览器必须保存已完成 File 身份，不重复上传。

`POST .../stages/{stage}/material-collection/{reference_id}/withdraw` 带原命令、expected revision 和原因，仅结束该引用。成功回放先于上游访问，提交成功但回复未知只继续同一命令／相同 payload。取消／刷新不是新命令。普通读取与历史只读本地，不在线重验文件、不写 Project 状态。

阶段状态投影在写命令中由当前主文件计算：有主文件为 completed，第一个缺主文件为 ready，其后为 pending；无缺口为 completed Work Item。校审单和评分不推进，移出可重新出现缺口，既有完成时间仅在齐备状态实际改变时更新。此客户版本不以负责人为资料齐备门槛；负责人仍通过独立原生命令维护，旧 version 1 双重门槛保持。

## 权限与验收

新动作 `production-work-item.stage-material.read/register/withdraw/history.read`，仅 Tenant、精确 Project 或精确 Work Item Scope；register／withdraw 同时需要 read，不借旧 stage.manage、reference.read、客户岗位或 Workbench 服务身份扩权。创建 version 2 还需旧 create 和新 read／register。用户文件上传／访问仍分别走 Evidence 能力。所有客户端 Actor Header 清除，HTTP 只从可信 Principal 派生 Actor。

关闭门槛：原组件保存／刷新／预览下载／取消与未知命令恢复；精确权限／撤权／跨租户／类型／阶段；同命令及不同命令并发、stale revision、Evidence 停机零写；真 PG18 schema 2→3 及旧 definition version 1 事实兼容保持、独立启动、正常 Workbench 入口、匹配 D1 备份与空目标恢复后继续。低内存期间串行检查，未运行项明确待验；不能在 Main 做破坏性演练。
