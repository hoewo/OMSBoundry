# 原 OA 试验／报告原生切片验证报告（2026-10-06）

结论：`private-native-passed / public-main-and-full-oa-not-accepted`。

问题 A →可验证结果 B：原工作台已有试验／报告入口，但没有真实类型适用性、文件保存和恢复链 → 直接复用原 OA 两个组件，在私有 Nginx→Gateway→正式 IAM→Project／Production／Evidence→PostgreSQL 18 链完成 v6 类型快照、试验／报告资料、撤权和五所有者空目标恢复。

## 实际运行范围

- 驱动：`acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs`；
- 最终命令：`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs /tmp/oa-test-report-suite.UjrJBU`；
- 最终回执：`/tmp/oa-test-report-suite.UjrJBU/test-report-browser.json`；
- 时间：2026-10-06 12:35:01～12:38:55（Asia/Shanghai）；
- 运行位置：私有临时远端，临时 Nginx 与受限 PostgreSQL 18；本地 Docker 未启动；
- Main／公网未改变，临时容器和本地进程均已停止。

## 9 个实际通过场景

1. 既有 v5 Work Item、5 Stage 和历史在类型化阶段使用前保持；
2. 直接复用的勘察试验面板读取 v5 时零业务写；
3. 一次 5→6 冻结精确 Project revision 的四个类型键，并保存两份试验主文件；
4. 原共享报告面板保存两份主文件，无校审／评分；
5. 报告预览／下载返回真实 Evidence 字节且零 Production 写；
6. 单份报告移出只结束该引用，其余报告保持完成；
7. Identity、People、Project、Evidence、Production 五库逻辑空目标恢复后 v6、类型快照、文件、历史和原 UI 继续；
8. 撤销正式 IAM 登记／移出能力后仍可读，页面不显示写动作，服务端 403 且零写；
9. Project／People 所有权摘要不变，升级只有一次，页面错误为零。

最终 Production 计数为 1 个 Work Item、7 个 Stage、4 个不可变资料引用、1 个 ending、5 个 command receipt、1 个 upgrade／upgrade event、4 个不可变 Project type snapshot、0 个 score。当前 2 份试验和 1 份报告引用有效；Work Item 仍为 `in-progress`，因为本夹具的 scheme 等必需阶段未完成，试验／报告完成不会伪造整个生产完成。

恢复校验覆盖五个独立数据库和 4 份本地 Evidence 字节；恢复前后每库 canonical digest、每份字节 SHA-256 和签名密钥身份一致。

## 失败轮次及修正

| 轮次 | 现象 | 结论与修正 |
| --- | --- | --- |
| 1／2 | 5→6 在约 2 秒后返回 `REFERENCE_SERVICE_UNAVAILABLE`，随后验收又显示 test 422 | 低资源远端 PG＋SSH 夹具的 3 秒引用预算不足；而验收只等待待选文件名，误把前端本地状态当成保存成功。业务升级未提交，未产生跨边界部分写 |
| 修正 | 增加专用夹具可观测性 | 保持产品允许范围内的有界超时，将该慢链设为 15 秒；记录每次工作负载换证的 status／duration；浏览器必须等原组件成功消息后才读取 Owner 事实，不增加自动业务重试 |
| 最终 | 全新夹具完整重跑 | 9／9 通过；Project 换证 200／210ms，四次 Evidence 换证均 200／212～392ms，不能用前两轮局部结果拼接 |

## Project／Production／Contract 边界结论

### Project Workspace

Project 继续拥有当前类型键与 revision。本轮零 schema、零 Capability、零业务写；Production 只在 v6 创建／升级边界使用既有 `project-workspace.read` 精确校验一次。Project 后续改型不会改写已冻结的 Production 历史。

### Production Operations

Production 新增不可变 v6、test／report Stage 和 `production_project_type_snapshots`。后者只保存解释本 Work Item 适用性所需的四类键及已验证 Project revision，是 Production 历史证据，不是 Project 当前主档副本。正常阶段读取／投影不再同步访问 Project。资料引用、ending、完成投影和历史仍由 Production 独占，文件字节仍归 Evidence。

### Contract Ledger

Contract 本轮零 schema、零 Capability、零运行依赖。报告或试验材料不自动成为合同事实；名称相近不能建立同步、共享表或隐式 Link。

## 跨系统依赖结论

- IAM：沿用既有 Production stage-material 与 `project-workspace.read`／`evidence-service.metadata.read`，目标应用最终授权；
- Project：只在 v6 边界作精确、一次性引用校验；正常读取不用 Project；
- Evidence：按 File／Version 精确校验和访问真实字节；业务移出不删除字节；
- Workbench：只承接原组件、类型导航和持久子命令续办，不保存业务事实；
- Contract：零依赖。

## 未关闭范围

Main／公网匹配发布、正式 D1 archive／keys／旧会话、双 Tenant、全角色、Project 多类型进度消费者、批复审计、外委、资料统计、完整 Production 行和完整 OA 均未通过。当前总判断继续为 `insufficient-evidence`。
