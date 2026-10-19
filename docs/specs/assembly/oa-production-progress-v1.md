# 原 OA 四类型生产总览／进度 v1

状态：BF-116 已接受；本地工程检查及私有真实 Nginx→Gateway→正式 IAM→五所有者／PostgreSQL 18 的 10 场景、撤权和逻辑空目标恢复通过。Main／公网、正式 D1／keys、全角色、第二客户与完整 OA 未通过。关联 BF-111／BF-115、[试验／报告规格](./oa-production-test-report-v1.md)、[运行报告](../../reports/oa-production-progress-native-validation-20261006.md)、[阶段资料集合](./oa-production-stage-materials-v1.md)与[小系统边界演变账](./small-system-boundary-evolution-ledger.md)。

问题 A → 可验证结果 B：原 OA 的生产总览同时读取项目类型、适用性、自动进度和文件列表，若在分离架构中照搬会让一次开页扇出 Project／Production／Evidence，并容易在 Workbench 再算出第二份进度真相 → Production 从同一 Work Item、冻结类型快照、Stage 和有效资料引用发布一个版本化只读投影；Workbench 直接复用原 `ProductionInfo` 的生产状态、阶段资料、各类型进度和阶段入口结构，只负责显示、缓存与导航，不保存或重算业务状态。

## 原 OA 事实与本次纠正

- 原 `ProductionInfo.tsx` 的顶部显示完成类型数／类型总数、生产状态和阶段文件数；总览卡按项目类型显示阶段资料行，并提供阶段快捷入口。
- 原 `oa2026.GetProgress` 为咨询、设计、勘察、测绘各生成一条进度线；共享的方案／报告会在多条线上显示。
- 原单体实现会在读取进度时继续查询 Project 类型、适用性及多个相邻业务来源，并把整体状态写回 Project。这种实现事实可作为界面和操作参照，不能照搬为新架构的数据所有权。
- 本次只接阶段与四类型进度。批复、审计、外委、生产成本和奖金仍由各自 Owner 独立接通；未知金额保持“未读取”，不得以 `0` 补齐总览。

## 唯一事实与边界

- Project 继续拥有当前项目类型、Project revision 和 lifecycle；本端口不读取 Project。Production v6 仅使用创建／升级时已验证并冻结的 `project_type_keys`。
- Production 独占 Work Item、类型定义版本、冻结类型快照、Stage 状态、资料业务引用、评分、ending、revision 和完成语义。本投影只读取这些事实。
- Evidence 继续拥有 File／Version 和字节。进度只统计 Production 中仍有效的引用，不在读取时访问 Evidence，也不因 Evidence 暂时不可用抹掉已确认阶段历史。
- Contract 对本投影保持零依赖。“变更洽商”是 Production Stage，不读取或写入合同变更。
- Workbench 不持久化进度、不按当前 Project 类型重判、不将共享 Stage 复制成多套事实，也不把缺权／故障显示为空或零。

## 版本化端口

`GET /api/v1/production-stage-progress?project_id={ProjectID}&type_key=reference-oa-stage-set-v1`

- profile 固定为 `reference-oa-production-progress-v1`，schema version 固定为 `1.0`；沿用 `production-work-item.stage-material.read` 和精确 Project Scope，不新增 Capability、Audience 或 Workload Policy。
- 一次响应返回全局适用／完成／必需计数，以及唯一活动 Work Item 的 revision、definition version、Project source revision、冻结类型键、按类型进度和阶段文件计数。
- 四类型进度是同一 Stage 集合的只读视图：咨询为方案＋报告；设计为方案＋初设＋施工图＋必需竣工，变更保持可选；勘察为方案＋试验＋报告；测绘为方案＋报告。共享阶段不产生第二份可写记录。
- `type_progress` 的完成只由该类型必需阶段决定；Work Item 完成仍由 Production 自有状态决定。页面不能自行把“有一份文件”解释成完整项目完成。
- 同项目出现多套活动阶段真相、遗留 v1 与新阶段集合并列、错 Tenant／Project、快照不一致、未知 Stage 或不兼容版本时失败关闭，禁止合并或选一条猜测。

## 读取体验与请求上界

- 进入生产总览只发一次精确 IAM 授权后的 Production 请求；Production 内部只读本库，不回查 Project、Evidence 或 Contract。
- 首次读取立即显示原 OA 结构的占位状态；回复后原位填充，不用空白页等待。成功结果在当前页面内缓存 30 秒；同键并发只共享一个进行中请求，返回总览不会再次换证／读取；显式刷新才主动重读。
- 切换到阶段栏目只加载该栏目 Owner；隐藏栏目不预取。迟到的总览响应不能覆盖用户已经切换到的奖金、成本或阶段页面。
- 来源故障时保留最后一次成功投影并标注后台更新失败；没有缓存时显示可重试错误，不能显示“0 个阶段”或“没有记录”。

## Project／Production／Contract 本次边界变化

| 系统 | 本次新增或改变 | 明确未改变 |
| --- | --- | --- |
| Project | 无 schema、命令、Capability、Link 或读取流量；只保留此前冻结快照的来源 revision | 不拥有生产进度，不被普通总览同步调用，不接收反写 |
| Production | 新增 owner-native 只读投影和四类型视图；阶段集合第一次成为可供完整总览消费的稳定读模型 | 不新增可写事实、migration、外部 Owner 或任意流程 DSL；v2～v6 语义不变 |
| Contract | 无变化、零依赖 | 不因生产“变更”或进度显示加入调用链 |

这不是“边界没有变化”：Production 的公开读边界从逐 Stage 明细扩展为有业务含义的四类型进度投影。但它没有改变写入所有权，也没有让 Project、Workbench 或 Contract 成为第二事实源。

## 验收门槛

1. 咨询、设计、勘察、测绘及多类型组合均显示正确类型线、适用 Stage、主文件／校审计数和完成状态；非适用 Stage 不以 0／待完成出现。
2. 原 OA 总览结构、阶段入口和 Tab 切换可见可操作；首次加载有占位，缓存命中、显式刷新、迟到回复和失败保留行为可见。
3. 一次总览选择至多一次 Production 业务请求和一次同 Scope 凭证更新；普通读取 Project／Evidence／Contract 请求数均为 0。
4. 精确 Project Scope、错 Tenant／Project、撤权、平行真相、遗留冲突和未知版本全部失败关闭且零写。
5. 真实 PG18 中已确认 v6 资料与类型快照重启／逻辑空目标恢复后，同 ID／revision／计数／类型线继续；Evidence 停机不影响本地已确认投影，预览字节另行保持原边界。
6. 静态审计继续为跨应用 import 0、replace 0、跨库 SQL 0；完整 OA、正式 D1、公网和第二客户仍需独立证据，不能由本切片外推。

## 当前实际证据

- Production Go 全包通过，覆盖不访问 Project／Evidence、v6 适用阶段、四类型视图、错 Owner 投影和平行阶段真相拒绝。
- Workbench 原生 transport／runtime 覆盖精确 Project Scope、单请求、不回查 Project、跨项目拒绝、30 秒页面缓存和迟到响应隔离。
- Workbench 409 项 Node 全量回归、Go test／vet、TypeScript check、Vite 3204 modules build 通过。
- `make architecture-boundary-static-audit` 通过：15 个应用、跨应用 import 0、跨应用 replace 0、跨数据库 SQL 机制 0。
- `OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs /tmp/oa-test-report-suite.tSki9f` 在私有真实入口通过 10／10：总览首次一次 Production 请求、返回缓存零读、显式刷新第二次读取、浏览器侧 Project／Evidence 读取 0、Production digest 零变化、正式 IAM 撤权只读，以及 IAM／People／Project／Evidence／Production 五库逻辑空目标恢复后原 OA 页面继续。
- 恢复前后五库 canonical digest、4 份 Evidence 字节 SHA-256 和签名密钥身份一致；页面错误 0，Main 未改变，fixture 已停止。完整范围与首轮验收导航缺陷见[验证报告](../../reports/oa-production-progress-native-validation-20261006.md)。
