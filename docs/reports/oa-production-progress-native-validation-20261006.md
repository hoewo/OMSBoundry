# 原 OA 四类型生产总览／进度验证报告（2026-10-06）

结论：`private-native-passed / public-main-and-full-oa-not-accepted`。

问题 A → 可验证结果 B：原 OA 总览会按咨询、设计、勘察、测绘显示完成类型、阶段资料和快捷入口；若直接照搬单体读取，会在一次开页中扇出 Project／Production／Evidence 并把进度反写 Project，Workbench 也容易产生第二份真相 → Production 用同一 Work Item、冻结类型快照、Stage 与有效资料引用发布一个版本化只读投影，Workbench 只复用原界面、缓存和导航；私有真实入口证明一次 Owner 读取即可还原四类型进度，且普通读取不访问 Project／Evidence／Contract、不写任何业务事实。

## 实际运行范围

- 驱动：`acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs`；
- 最终命令：`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs /tmp/oa-test-report-suite.tSki9f`；
- 最终回执：`/tmp/oa-test-report-suite.tSki9f/test-report-browser.json`；
- 时间：2026-10-06 13:10:49～13:15:01（Asia/Shanghai）；
- 运行位置：私有临时远端，临时 Nginx、Gateway、正式 IAM、五个应用所有者和 PostgreSQL 18；本地 Docker 未启动；
- Main／公网未改变，临时 fixture 已停止。

## 10 个实际通过场景

1. 既有 v5 事实在类型化阶段使用前保持；
2. 直接复用的勘察试验面板读取 v5 时零业务写；
3. 同一 Work Item 一次 5→6，冻结四个 Project 类型并保存两份试验主文件；
4. 原共享报告面板保存两份主文件且不制造校审或评分；
5. 报告预览／下载返回真实 Evidence 字节且零 Production 写；
6. 单份报告移出只结束该引用，剩余报告保持完成；
7. 原 `ProductionInfo` 总览显示四类型进度和 7 个适用阶段；进入总览只有一次 Production 读取，切走再返回命中页面缓存，显式刷新才发起第二次读取；浏览器侧 Project／Evidence 读取均为 0，Production digest 不变；
8. IAM、People、Project、Evidence、Production 五个独立数据库逻辑空目标恢复后，v6 类型快照、阶段、文件和原 OA 页面继续；
9. 撤销正式 IAM 的资料登记／移出权限后仍可读，页面不显示写动作，服务端 403 且零写；
10. Project／People 所有权摘要不变，只有一次 definition upgrade，页面错误为 0。

终态为 1 个 Work Item、7 个 Stage、4 个资料引用（3 个有效）、1 个 ending、5 个 command receipt、1 个 upgrade／upgrade event、4 个不可变类型快照、0 个 score。`test` 与 `report` 都为 completed；整个 Work Item 仍为 in-progress，因为本夹具的其他必需阶段未完成。

## 四类型只读语义

| 类型 | 必需阶段视图 | 本夹具结果 |
| --- | --- | --- |
| 咨询 | 方案、报告 | 1／2 |
| 设计 | 方案、初设、施工图、竣工 | 0／4；变更是可选视图 |
| 勘察 | 方案、试验、报告 | 2／3 |
| 测绘 | 方案、报告 | 1／2 |

这些类型线是同一组 Production Stage 的只读视图。共享方案／报告不会复制为四套可写记录，类型完成也不会反向改 Project。

## Project／Production／Contract 边界演变

### Project Workspace

本批零 schema、零 Capability、零 Link、零业务写、零总览读取。Project 仍独占当前项目类型、revision 和 lifecycle；Production 只消费此前 v6 创建／升级时冻结的来源 revision 与类型键。Project 不拥有生产进度，也不接收总览反写。

### Production Operations

Production 的**公开读边界发生了有意扩展**：由逐阶段明细增加 `reference-oa-production-progress-v1`，能够从自己的 Work Item、类型快照、Stage 和有效资料引用给出四类型进度。这没有增加 migration、可写事实、Capability、Audience、Workload Policy 或外部 Owner；写边界仍是既有 v6。多套活动阶段真相、错 Tenant／Project、未知版本和快照漂移均失败关闭。

### Contract Ledger

本批零 schema、零 Capability、零入站／出站调用。Production 的“变更洽商”和“进度”不会因为同名业务词读取合同变更或结算；如果将来需要法律事实，只能走显式连接器和 Contract 原生命令。

## 跨系统与体验结论

- 浏览器总览：一次精确 Project Scope 的 Production GET；普通读取不调用 Project、Evidence 或 Contract；
- 服务端总览：只读 Production 自库，不向外交换 workload credential；
- Workbench：首次显示原 OA 结构占位；成功结果页面内缓存 30 秒；同键并发合并；显式刷新绕过缓存；迟到回复不能覆盖已切换页面；
- IAM：复用 `production-work-item.stage-material.read`，不以页面缓存绕过签发或目标应用最终授权；
- Evidence：总览只统计已确认业务引用，不在线检查字节；实际预览／下载仍走 Evidence 原边界；
- 恢复：五库恢复前后 canonical digest 一致，4 份 Evidence 字节 SHA-256 一致，签名密钥身份保持。

## 验收中发现并修正的问题

首轮业务与恢复事实已经成功，但恢复后浏览器仍停在新加入的“总览”页，旧验收断言却直接等待隐藏的“咨询报告”文件，45 秒后超时。失败截图中的总览已显示勘察 2／3、咨询 1／2，服务端恢复摘要也一致，因此这是验收导航缺陷，不是业务或恢复缺陷。验收改为恢复后显式回到原 OA 的“咨询报告”页，再从全新空目标完整重跑 10／10；不能拼接首轮局部结果。

## 同批工程检查

- `cd backend/services/design-operations && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...` 通过；
- `cd backend/services/design-operations && GOMAXPROCS=2 GOWORK=off go vet ./...` 通过；
- Workbench server／frontend Node 回归 409／409 通过；
- frontend `npm run check` 和 Vite 3204 modules production build 通过；
- `make architecture-boundary-static-audit` 通过：15 个应用、跨应用 import 0、replace 0、跨数据库 SQL 机制 0。

## 判定和未关闭范围

本批证明 Production 能在不扩大写所有权和不增加运行时远端读链的前提下，从自己的阶段事实提供原 OA 四类型进度；这是一项可复用的只读边界证据，不是“服务没有合并”的最低守卫。

Project、Production、Contract 整体仍只到候选边界。Main／公网匹配发布、正式 D1 archive／keys／旧会话、双 Tenant、全角色、第二真实客户、Project 类型重分类、Results 批复／审计、对外委托、资料统计和完整 OA 均未关闭，总判断继续为 `insufficient-evidence`。
