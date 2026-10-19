# 原 OA 生产人员原生候选验证

2026-10-04。本地独立候选有限通过：原 UI 实际点击连到正式 IAM、真实 People HTTP 和 Project PostgreSQL 18。未接入正常 Workbench、未发布 Main，D01／完整生产／完整 OA 未验收。

## 本批可见结果

问题 A→结果 B：复制人员界面仍只能操作测试名单 → 保留原专业四列、专业／审定两种 600px 抽屉和嵌套专业目录，在真实所有者上保存、同项目回读、保留结束历史，并能明确续办未收到回复的原命令。

原 Manager／CSS 来源和 UI-only 16 条仍见[前一层报告](./oa-production-personnel-ui-candidate-20261004.md)，不能把内存名单计作数据库证据。本批没有改原仓库，没有手写新业务布局，也没有把负责人替换成六种专业角色。UI 适配器只携带版本化引用／意图；Project 自有 Tenant 目录和项目分工，People 拥有人，IAM 拥有账号及签名身份。BF-104 的[有界规格](../specs/assembly/oa-production-personnel-v1.md)同步了这些实现和选装门槛。

已实际查看本轮图片：[真实选人抽屉](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/native-professional-drawer.png)、[原专业四列及两项目级角色](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/native-seven-facts.png)、[正式新凭证撤权后的只读界面](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/native-formal-readonly.png)、[项目人员管理权不含目录管理权](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/native-personnel-without-catalogue-authority.png)。不是原 OA 现场等效或公网截图。

## 已实现的候选边界

Project migration 12 新增五张有界 Overlay 表，不改旧四种 Base Role、旧 Assignment v1 引用或原策略。新表保存六种分工、目录不可变版本、当前 head、Project profile／参与适用性和可信 Actor 事件；完整整组操作在同一 Project 事务内完成，失败不留部分 ending。no-op 只有回执，未变化的事实不取代；结束只追加 lifecycle，重入新 ID。

服务 profile 默认关闭，不注册人员路由；配置明确版本后仍需受保护的逐项目选装命令。四种新 action 已进入候选 Capability Manifest 0.8.0，仅本地测试 IAM 实际注册／签发；Main Grant 未改变。人员使用确切 Project 凭证，目录管理另外需要 Tenant 原生能力；旧读／assignment.manage 不能继承，客户岗位或页面按钮不是授权条件。

People 验证走真实工作负载交换：`people-groups.view`、单个 `person` 资源。客户端 Actor／显示名不入命令，服务冻结实际 ID／Tenant／active／revision／姓名；目录版本在 Project 事务内锁定并复验。来源故障和过期引用明确拒绝，不能当空名单。结束既有分工不删除 Person／IAM／目录，也不改 Bonus 的旧来源协议。

原生适配器保留未知结果的确切 command ID／body／revision；刷新／reload 后有明确的原命令继续按钮，不用新的 revision 重造请求。续办键作用于当前可信用户／项目，不保存 Token、Cookie、Actor 或密码；明确 4xx 拒绝解除待办，不同意图不能覆盖不明确结果。

## 实际运行分层

| 层 | 实际结果 | 不能据此声称 |
| --- | --- | --- |
| 原界面测试端口 | 16／16、零页面脚本错误，保存和移出均保留隐藏参与人 | 内存数据是原生事实 |
| Project 普通 Go | 119 个顶层／363 个含子测试 PASS，vet 通过 | 全部为真实 PG 场景 |
| Project 全量 PG18 | 161 个顶层／411 个含子测试 PASS，零测试 FAIL／SKIP | 合成上游的历史测试是正式多角色链 |
| 新切片专项 race | 14 个顶层／20 个含子测试 PASS，含同命令／竞争版本／目录同名并发 | 任意压力／多节点性能已定型 |
| 原组件＋真实三进程 | 21／21、零页面错误，IAM／People／Project 三库独立启动 | Nginx／Gateway／正常 Workbench／公网已经接通 |
| 前端及工作台回归 | Node 315／315、零 skip；TypeScript、Workbench Go／vet 通过 | 正常 bundle 含新人员页 |

Go 结果见[普通检查](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/go-default.json)、[PG18](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/go-pg18.json)、[race](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/go-race.json)，前端见[检查回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/checks.json)。`go test -json` 中 `[no test files]` 的包级 skip 与实际测试 skip 分开，表内“零 skip”仅指实际测试。

### 原生界面 21 场景

[真实浏览器回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/browser.json)逐条保留结果；普通业务操作没有 force、DOM 事件注入或直接调用控件私有保存方法。

1. 三应用通过各自原生 API 建立人员／项目／目录，显式 profile 选装；没有 SQL 灌入业务名单。
2. 正式 IAM 原 native-read 撤回而 manage 保留时读拒绝、旧 Project read 不继承；错误 Project scope 拒绝。
3. 原空页、正常打开和取消零业务写。
4. 专业设计多人、同人设计／参与、复核／审核保存到真实分工。
5. 审定／概算两个独立位置保存，reload 回读，零额外写。
6. 同人跨专业保留不同分工身份。
7. 更换到已占用专业显示 409、保留原抽屉、零写；明确结束目标组后方可继续。
8. 原生 People revision 不匹配返回 422，Project 全局 digest 不变；这是 API 故障注入，不声称 UI 会产生过期版本。
9. 原按钮保存成功后代理丢回复；reload 后点击“继续上一笔”，命令 body 完全相同、事实／事件／回执不重复。
10. 通过受保护适用性命令开启“不适用”，原参与字段禁用；正常保存保持原身份。适用性命令不是伪称来自尚未接好的原总览开关。
11. 原未修改保存只追加一条回执，不增加 Project revision／分工／事件。
12. 原组移出保留隐藏参与人及全局角色；恢复适用显示原 ID，再显式结束，结束历史可回读。
13. 原嵌套抽屉新增真实目录专业；取消父抽屉不产生人员分工。
14. 实际停止 People 进程，新命令 503 且 Project digest 不变；恢复后续同一命令成功。这是服务故障 API 注入。
15. 正式 IAM 撤回管理 Grant，新签发凭证写 403；刷新后原管理按钮隐藏，零写。
16. 保留目录管理、撤回人员管理：目录 allowed_actions 明确可管理，但项目人员写 403；以 Project scope 请求 Tenant 目录读／写也 403，digest 不变。
17. 仅保留项目人员管理、撤回目录读／管：Tenant 目录读／写 403；项目级专业发现仍可选已有专业，原增／改／删按钮不出现，正常取消零写。
18. 原审定人字段 hover 后正常点击清除／保存：空数组是显式结束，旧分工 ID 保留 ended history，无新增分工；事件、命令各一条。没有借私有 Save 或直接 API 伪称 UI 清除。
19. 浏览器代理对每个请求携带伪造身份 Header，实际人员事件及目录版本的 Actor 均为受信 IAM subject。
20. 实际停止 Project 并以空 profile 重启：人员路由 404，旧 Base Project read 正常，所有事实保持。
21. 重新启用 profile 再重启：相同 ID／revision／全部结束历史／回执可读取，数据库 digest 不变。

最终隔离 fixture：Project revision 13，14 条分工（0 active／14 ended），12 个 Overlay 事件，17 个 Project 命令回执（包含建项／选装／适用性等，不全是人员保存），3 个目录版本。浏览器代理 POST 12 次包含正常写、冲突和丢回复重放，不能把次数当事实数量。People 当前档案／revision／命令／审计 digest 未变化，旧 Base Assignment 始终零。

这里的撤权代理每次重新向正式 IAM 取得限定凭证，不证明已签发旧 Token／Gateway Cookie 会即时失效；两种能力组合用同一隔离 subject 的精确 Grant 变更验证，不冒充多账号／跨租户矩阵。现有凭证生命周期／刷新契约和真实入口撤权另验。浏览器代理仅转交原生请求、不伪造业务结果，但仍是测试装配，不是正式 Gateway。

## 持久化、兼容和恢复范围

专项 PG 测试另验：六角色／单值唯一、SQL 不完整专业快照拒绝、先结束后插入故障整笔回滚、原命令恢复、同命令只提交一次、不同命令同 revision 一赢一冲突、快照／事件／命令不可变、目录改名不改旧分工、停用／重新启用和并发有效名称唯一。

11→12 连跑两次，旧项目／四角色 ID、revision、摘要／命令／历史及关系逐表 canonical 保持，新 profile／目录／分工／事件均为零，不自动选装或扩 Grant。Project 单库使用真实 PG18 custom dump→新空库 restore：profile、目录所有版本、分工／结束历史、旧 Base／新事件／回执 canonical 相同；来源离线重放不写，再新命令继续。备份在测试临时内存／目录中，不复制到证据目录；不是 Suite 恢复或跨应用引用验收。

## 修复、失败和可复演入口

首轮正式 IAM 回执将 Tenant scope 规范化为 `resources:[]`，驱动误要求字面完全相同而失败；修正的是测试形状，不放宽范围。新增 profile-off 重启时，未注册路由按 Go HTTP 正常返回纯文本 404，驱动误当 JSON；修正为仅允许未注册 404 的非 JSON，保留两份已运行失败。清除场景首轮保存已完成，驱动误用不存在的 header-card 类等待而失败；改为原 key-role 选择器后整轮通过。四份[失败回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/)均未删除；18／20 条先前通过回执也单独保留，不混作最终 21 条。

全量 PG 首轮有一失败／二跳过，原因为本机没有 pg_dump／pg_restore；[首轮反例](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/failed-host-pg-tools.json)保留。新增受限测试适配器只调用任务自有 PG18 容器，只接受 loopback 55477 和随机 `project_workspace_test_*` 数据库及 Go 测试临时 dump；没有安装宿主工具或操作 Main。补跑全量已零失败／跳过。

其他施工中修复：专业 Form ID 转交、隐藏参与人移出语义、读取失败不作空名单、未知回复重试不能使用刷新的 revision、独立目录权限不能从项目管理权推断、读响应必须匹配当前 Project。旧 15 条 UI-only“通过”已因隐藏移出测试约定错误明确取代，不掩盖反例。

复演依赖真实 PG18 和本机 Chrome，测试容器名／ID／label 均检查，非通用生产操作脚本：

```sh
node acceptance/oa-initiation-repair/verify-production-personnel-native-browser.mjs \
  "$(mktemp -d /tmp/oa-production-personnel-native.XXXXXX)"
```

普通 Go／Node／TypeScript 入口保持原应用说明；PG18 全量恢复需 pg_dump／restore 18，缺少时使用明确验证过的任务自有工具适配器，不能把 SKIP 计 PASS。固定路径收集器只整理已经运行的回执，不替代复演。

收尾已核验精确 ID／任务 label，停止并自动移除了该一次性 PG18 tmpfs 容器；生成的最终 fixture 数据库不保留，截图／回执和此前独立恢复证据保留，不涉及 Main 数据卷。下一轮原生浏览器复演须先重新创建同名、同 label、仅 loopback 55477 的隔离 PG18 容器，并等待就绪；不能直接对 Main 运行。宿主工具适配器刻意绑定本轮已移除的精确 ID，下一轮要重新登记／验证新容器 ID 或使用实际安装的 PG18 工具，不能把旧容器 ID／旧回执当新运行。

[只读与清理检查点](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/closeout.json)记录 11:41 UTC Main 17 容器健康、857 个本地文档引用存在、候选源码摘要无漂移、原六来源摘要及逐字 CSS 不变；容器健康不证明页面业务通过。

独立 native preview JS SHA `9d31ffe252cdb087698cb6cf60c825c7009886aba43493ed0962cf8452711fe0`；正常 Workbench JS 仍为 `7df15d8a63a0ac31a308401631aaedba38cbb2f73ba5d1334c4d9e88ed41c357`，没有导入新人员入口。[候选 provenance](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/candidate-provenance.json)保存源码／migration／目录摘要，但不含凭证、构建树或备份内容。

## 尚未关闭的门槛

先完成新分工的版本化最小引用端口／Bonus 明确兼容消费，再接正常 Workbench 同项目工作面；不得放松 Bonus 旧 hash／角色校验或复制分工事实。接着补全专业单字段清除／其他全局字段组合、更多名册、跨角色／跨租户真实签名矩阵、原适用性工作面、原站现场对照、匹配 migration／profile／IAM／工作台发布闭包、Nginx→Gateway 入口与全 Suite 恢复。九个生产子栏目和全站清单中的合同／开票／资金／资料统计等剩余项仍不能由这个人员切片替代。

Main 此批未写业务、未改 Grant／部署；C 与 Test101 不参与该隔离 fixture。D01、完整生产、完整 OA 和架构整体结论继续 `insufficient-evidence`。
