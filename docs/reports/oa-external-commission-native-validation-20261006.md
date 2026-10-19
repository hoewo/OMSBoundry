# 原 OA 对外委托原生闭环验证报告（2026-10-06）

## 结论

原 OA“生产信息 → 对外委托”已在全新私有夹具完成 10／10 场景，判定为 `private-native-slice-supported`。这关闭的是**对外委托这一纵向切片**，不是完整 OA、Main／公网、正式 D1、全角色或跨客户稳定结论。

问题 A → 可验证结果 B：原页面把受托方、发生时联系人、法律合同／附件、评分和可删除重建的付款数组混在一个保存动作中 → 保留原列表、三项统计和 880px 抽屉，但把事实分别写入 Cost、Party、Contract、Evidence 和 Money；局部成功可续办，Owner 失败不抹成空数据，任何相邻系统不获得第二份可写真相。

## 实际运行

运行入口：

```sh
node acceptance/oa-initiation-repair/verify-external-commission-native-browser.mjs /tmp/oa-external-commission-suite.JaPjHm
```

运行时间为 2026-10-06 16:29:18～16:34:14（Asia/Shanghai）。链路为私有 Nginx → Gateway → 正式 IAM／Policy → People、Project、Party、Contract、Evidence、Cost、Money → PostgreSQL 18；浏览器使用真实 Chrome。Main 未改变，本地 Docker 未启动，夹具结束后已停止。

长期证据：

- [结构化回执](../../acceptance/oa-initiation-repair/evidence/external-commission-native-20261006/external-commission-browser.json)，SHA-256 `a408924290740099950a2fb13aefe136dddd93dc941125eeb7da64b32f95d9b4`；
- [最终原界面截图](../../acceptance/oa-initiation-repair/evidence/external-commission-native-20261006/external-commission-final.png)，SHA-256 `cd28329843e65d1233ba17788851f9baa7a5312d479de605fe1e5f31536b93e0`。

## 10 个通过场景

1. 原三项统计、列表空态和添加抽屉可用；
2. 模拟服务端成功但回复丢失后，同一 command 重放只形成一条委托，冻结 Party、发生时联系人、Contract revision、Evidence File／Version 和整数最小货币单位；
3. 合同附件由 Evidence 提供真实预览和下载，字节一致；
4. 评分追加，联系人纠正产生新 engagement revision，旧快照仍在历史；
5. Cost 提出应付建议，Money 独立核验并接受后形成 Obligation，Cost 再显式同步结果；未把义务冒充实付；
6. 历史可见，第二条委托撤回后只退出有效列表，不物理删除；
7. 返回 Tab 命中页面缓存，Cost 列表新增读取为 0；显式刷新恰好新增一次读取，Project／Party／Contract owner digest 不变；
8. IAM、People、Project、Party、Contract、Evidence、Cost、Money 八库恢复到全新空目标，八库 canonical digest、Evidence 字节和签名密钥身份保持，恢复后原页面继续；
9. 正式 IAM 撤销七项写能力后，页面只读和历史仍可用，直接 API 写返回 403，Cost digest 零变化；
10. Project、Party、Contract、Evidence、Money 的工作负载换证均为单资源精确 Scope，页面错误为 0。

终态为 2 个 engagement、4 个 engagement revision、1 个 assessment、1 个 suggestion、3 个 suggestion revision、8 个 command／event；Money 为 1 个 intake、2 个 intake revision、1 个 obligation／revision。

## 三个重点系统的边界演变

| 系统 | 本切片真实变化 | 未变化／禁止变化 | 当前判断 |
| --- | --- | --- | --- |
| Project Workspace | 仅在 Cost 新建边界被精确读取当前 Project ID／revision／active 状态 | 零 schema、零 Capability、零业务写、零委托字段、零反向同步；普通委托复读不请求 Project | 当前项目主档边界保持；依赖时机获得一次可重复证据，Project 整体仍为候选 |
| Production Operations | 仅保留原 OA 页面导航位置和 UI 装配 | 零 API、零 Capability、零数据库、零请求、零写；“位于生产信息”不等于 Production 拥有委托 | 证明页面归类与事实所有权可分离，Production 整体等级不变 |
| Contract Ledger | 用户显式选择正式合同时，Cost 以精确单 Contract Scope 读取并冻结 revision | Contract 零写；附件不能冒充法律合同；编辑委托不能改合同或删除合同历史 | 法律合同边界保持；入站精确引用增加运行证据，Base／Overlay 等长期收敛仍未完成 |

本切片扩展最多的是 Cost & Outsourcing：`Outsourcing Engagement` 增加可选 `contact_name`／`contact_phone` 发生时快照，列表增加 owner-side 最新评分投影，并实际使用既有 Assessment／Payable Suggestion 状态机。联系人不是 Party 主档，不跨委托共享，不反向同步；这是有界 Base 演进，不是任意 JSON 或客户专用 fork。

## 跨系统依赖和“小系统进化”结论

- 写入边界依赖：Cost → Project／Party／可选 Contract／Evidence，全部使用正式 IAM 交换的目标原生能力和单资源 Scope；目标应用最终授权。
- 建议边界依赖：Cost → Money 只提交版本化 Payable Suggestion；Money 独立决定是否形成 Obligation。实际 Cash／Allocation 仍未在本切片浏览器中执行。
- Money 对 Party Directory 的精确引用不能靠名称或本地 ID 猜测。夹具实际执行来源自有 migration preflight、不可变 mapping 和 runtime cutover 后才允许接受 suggestion。这是可部署依赖，不是共享表。
- 普通读取由 Cost 冻结快照回答，不递归回查 Project／Party／Contract／Evidence；文件只在用户预览／下载时读取 Evidence。
- Workbench 只保存页面内缓存、已完成 File Version 和有界待续意图，不保存长期业务事实或跨来源合计。

由此可复用的经验是：页面上的一个“保存”不等于一个跨库事务。应先按 Owner 拆成稳定子命令，冻结发生时快照，保留已确认段，以同一 command 续办；展示聚合必须与业务所有权分开。

## 实际发现与修复

1. 可选联系人若客户端省略、服务端返回空字符串，严格未知回执核对会误报不一致；协议现统一按可选空值比较并有回归。
2. 撤回记录仍由 Owner 返回，UI 必须只从有效状态形成当前列表，同时历史保留撤回项。
3. Money 接受建议前要求 Party Directory 精确映射；测试旁路会掩盖真实交付依赖，因此改为正式 migration／cutover 路径。
4. Tab 返回不能每次重新换证和读取；成功 owner 读取现在刷新页面缓存 freshness，返回零读，显式刷新一读。
5. 浏览器验收最初把无布局装配根当成可见交互根；按钮实际可见但自动化命中隐藏副本。最终按可见按钮定位并从全新夹具重跑，失败轮次没有拼接成通过。

## 仍未关闭

- Money 拒绝路径已有 owner/service 证据，本次浏览器只实际走接受；真实 Cash／Allocation、冲销和付款文件未纳入本切片；
- Party、Contract、Evidence、Money 逐一停机的全部 UI 矩阵、跨 Tenant／相邻 Project、金额脱敏和非管理员角色仍需专门验收；
- Project Completion Checkpoint、资料统计、正式 D1／keys、Main／公网、第二客户和完整 OA 仍为 `insufficient-evidence`；
- 联系人跨委托共享、供应商联系人目录或复杂采购合同如果出现，必须重新证伪 Party／Cost 边界，不能静默扩大这两个可选字段。

## 下一步

停止继续扩对外委托页面，进入原 OA 资料管理与统计：先按 F01／F02 拆解“按项目展开才读取、下载、下载人员记录”，再按 S01～S03 核对台账筛选／导出、公司两链统计和人员奖金统计。只读投影必须来自来源 Owner；Project 只拥有 Link／上下文，Workbench 不建立统计数据库或第二份金额真相。
