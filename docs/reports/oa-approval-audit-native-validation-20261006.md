# 原 OA 批复／审计原生闭环验证报告（2026-10-06）

结论：`private-native-passed / public-main-and-full-oa-not-accepted`。

问题 A → 可验证结果 B：原 OA 有批复／审计双列、三项金额、报告文件、修改、撤回和历史，但此前 Workbench 只显示浅层样例，且适配器会丢失既有来源／金额／文件 → 直接复用原页面结构，把全部业务动作接到 Approval & Audit Results、Contract、Evidence、Project 和正式 IAM；在私有真实入口用同一项目完成登记、故障续办、纠正、撤回重登记、撤权和六所有者空目标恢复，同时证明 Project、Contract、People 不被反写，Production 不进入依赖链。

## 实际运行范围

- 最终驱动：`node acceptance/oa-initiation-repair/verify-approval-audit-native-browser.mjs /tmp/oa-approval-audit-suite.MXO07p`；
- 最终回执：`/tmp/oa-approval-audit-suite.MXO07p/approval-audit-browser.json`，SHA-256 `5f0da59077064961a7c19ae7c50e88ada930cca515b491e2be7fc809f16aa4e7`；
- 时间：2026-10-06 14:36:21～14:40:37（Asia/Shanghai）；
- 路径：私有临时 Nginx → Gateway → 正式 IAM／Policy → Project／Contract／Evidence／Results，六个所有者数据库使用 PostgreSQL 18；
- Main／公网未改变，本地 Docker 未启动，临时 fixture 已停止；
- 结果：11／11 场景通过，页面错误 0；终态 2 个当前结果、5 个不可变 revision、5 个 command receipt、5 个 audit event。

## 11 个实际通过场景

1. 原 OA 批复／审计双列空态只读显示，Results 零写入；
2. 批复冻结精确 Contract ID／revision、三项整数金额和真实 Evidence 文件；
3. 审计手工登记三项精确金额和两个报告文件；模拟服务端已提交但浏览器回执丢失后，以同一 command ID 重放，事实只写一次；
4. 批复追加纠正保留既有来源、金额和文件，再追加一份报告，历史增量而非覆盖；
5. 审计从 recorded 撤回，再以同一 aggregate 重新登记；revision、command、event 和历史均追加，旧审计金额／文件只留在历史，不伪装成当前事实；
6. 报告预览和下载返回真实 Evidence 字节，Results 摘要零变化；
7. 切出再返回命中页面缓存，Results 列表新增读取 0、Contract 读取 0；显式“刷新”恰好新增 1 次 Results 列表读取；
8. Results／Evidence 写入前后，Project、Contract、People 所有者摘要不变；
9. IAM、People、Project、Evidence、Contract、Results 六库恢复到逻辑空目标，六库 canonical digest 前后一致，当前结果、三段审计历史及 4 个旧 Evidence 字节 SHA-256 一致；
10. 撤销 create／correct／withdraw／evidence.manage 后，当前结果和历史仍可读，写按钮关闭，直接写入 403 且 Results 零写；
11. 整条原页面旅程零 page error，适配资产 SHA-256 为 `c4ae47ac9e239f1f199aba69c6b8ce969343c923a86956582a88cca7be6e13c9`。

## 三个重点小系统的边界变化

| 系统 | 本批真正变化 | 本批没有变化 | 边界判断 |
| --- | --- | --- | --- |
| Project Workspace | 仅继续提供精确 Project 身份／revision／active 校验和已有 Contract Resource Link；Workbench 页面重载时读取项目上下文 | 零 schema、零 Capability、零业务命令、零批复／审计字段、零 Results 反写；本批未执行 Project Completion Checkpoint | Project 仍是上下文与 Link 所有者，不是结果所有者；整体仍为候选边界 |
| Production Operations | 无 | 零 API、零 schema、零 Capability、零调用，未进入 Results→Project／Contract／Evidence 写链 | “批复／审计显示在生产页”不等于 Production 拥有该事实；边界与等级不变 |
| Contract Ledger | 既有精确 reference-read 真正进入批复登记／纠正链，提供合同 revision 与三项金额快照 | 零 schema、零 Capability、零写入；审计直接冻结批复 revision，不重读 Contract；页面复读／缓存／历史也不读 Contract | Contract 仍只拥有法律合同事实；Results 拥有发生时的不可变来源快照；整体仍为候选边界 |

这次变化量最大的不是 Project、Production 或 Contract，而是 **Workbench 操作适配** 与 **Results 公开输出契约**：Results 的领域模型、表、状态机、Capability、Audience 和 Workload Policy 均沿用既有 C5；新页面终于把这些已有原生能力完整接到原 OA 操作。唯一 Owner 代码修正是把合法的空 Evidence 集合稳定序列化为 `[]` 而不是 `null`，没有新增业务事实。

## 跨系统依赖与失败语义

- 用户请求始终使用 `approval-audit-results` audience 的原生 Capability Credential，Results 作最终授权；Gateway 不代授权，Workbench 不按岗位名推断动作。
- 批复创建／纠正：Results 在本地事务前用精确 Project 与 Contract Scope 校验；确认后冻结来源，不在普通读取中回查。
- 审计创建／重登记：Results 从自己的精确 approval revision 读取来源，不形成 Results → Contract 的第二次链。
- Evidence：浏览器先完成上传，Results 只保存 File ID／Version／slot／digest／display name；若上传成功而 Results 未确认，Workbench 只持久化有界待续意图与已上传版本，不重复上传、不删除字节。
- 响应未知：重试同一 command ID；revision 冲突停止自动覆盖；已确认子步骤不回滚。
- 撤回：追加 withdrawn revision；重登记继续同一 aggregate ID 并追加 revision，旧事实留在历史。
- 读取：页面内 30 秒缓存和同键并发合并只减少重复请求，不绕过 IAM；显式刷新仍重新换取并执行 Owner 读取。

## 实际发现并修正的问题

补测“撤回后重新登记且不附新文件”时，Results 把合法的空 Evidence 集合编码为 `null`。页面的严格版本化契约拒绝该回执并保留原待续命令，没有把不完整回执当成功。修复落在 Results HTTP 输出层：集合始终编码为 `[]`；新增回归后从全新 fixture 重跑 11／11。此前两次“刷新”验收失败均来自 Ant Design 将两个汉字的按钮可访问名称渲染为带空格形式，页面和接口没有失败；验收改为语义匹配后重新完整运行，不拼接局部结果。

## 同批工程检查

- Workbench frontend Node：181／181 通过；
- frontend `npm run check` 通过；
- Vite 3204 modules production build 通过；
- Approval & Audit Results 全包 `go test`、`go vet` 通过；
- Unified Workbench 全包 `go test`、`go vet` 通过；
- `make architecture-boundary-static-audit` 通过：15 个应用、跨应用 import 0、replace 0、跨数据库 SQL 机制 0。

## 限制与下一工作面

本报告不关闭 Main／公网匹配发布、正式 D1 archive／keys／旧会话、双 Tenant、全角色、账号停用、第二客户、Project Completion Checkpoint 的本页面联动或完整 OA。验收夹具中的 Contract 当事方属于隔离搭建数据，Contract 本身仍通过正式 Contract API 创建／激活；该搭建便利不能冒充 Party→Contract 全链证据。

下一纵向切片是原 OA **对外委托**。它不能照搬原单体“保存委托后删除并重建支付记录”的做法：Cost & Outsourcing 独占委托／评分／合同和 Evidence 快照，Money 独占确认的应付及实际资金，Party 独占受托方当前主档，Contract／Evidence 只提供精确 revision。先把原委托列表、统计、抽屉、合同文件、评分和支付显示逐项映射到现有原生端口，再以局部成功续办、撤权、恢复和请求上界验收。
