# OA C8 项目资料、统计与个人工作边界审计

日期：2026-09-29  
结论：`boundary-accepted / runtime-not-started`

## 本轮问题

在不启动完整 Docker 组合、不增加共享数据库的前提下，确认参考 OA 的项目资料、公司／项目／人员统计和个人待办如何落入现有小应用架构，并给出下一实现可以直接验收的最小边界。

## 审计证据

参考 OA 静态读取范围：

- `frontend/src/pages/FileManagement.tsx`：先取项目目录，展开时按 Project 读取文件，下载为独立 POST，并展示下载统计；
- `frontend/src/pages/Statistics.tsx`、`CompanyRevenueStatistics.tsx`、`RevenueProjectLedger.tsx`：公司经营、项目台账、人员奖金以不同查询和权限呈现；
- `frontend/src/pages/ProjectInitiation.tsx`：默认呈现与当前账号有关的 draft Project，可选权限下查看全部；
- `backend/internal/oa2026/service.go`：资料目录、项目文件、下载统计、人员奖金和立项待办直接查询同库表；
- `backend/internal/services/financial_service.go`：公司与项目经营统计直接组合 Contract、FinancialRecord、Project 等内部模型，并使用 `float64`；
- `docs/OA_PERMISSION_MODEL.md`：项目列表必须按访问范围过滤，文件搜索必须带 `project_id`，列出／下载／上传依项目角色和全局角色区分，身份超级管理员不自动获得业务权；旧公司收入算法已被明确否定。

BizFoundry 静态读取范围：

- Project Workspace 已拥有 Project、Assignment、Resource Link、Evidence File 引用和 C3 无持久化并行组合；
- Evidence Service 已拥有 File Version、摘要、精确 File Scope 与短期 Access Grant，但 metadata 与 download 当前仍共用 view action；
- Unified Workbench 只读取 Suite Manifest 并显示入口，当前没有统一待办存储；
- 各业务应用已形成原生 Capability、目标最终授权、精确工作负载凭证和来源故障分类。

本轮没有启动 Docker、数据库或应用进程，没有把静态审计描述为运行通过。

## 关键判断

1. C8 是只读装配旅程，不新增业务事实所有者；第一版不建报表数据库。
2. Project Workspace 负责 Project 范围资料与状态的实时组合，Evidence 只负责技术文件事实和下载授权。
3. 资料列表与下载必须分权；当前 Evidence `view` 同时覆盖 metadata／Access Grant 是进入 C8 实现前的兼容演进缺口。
4. 公司、项目和人员统计先使用来源应用自己的类型化投影并分区呈现；没有版本化公式时不跨来源合计。
5. 个人待办必须由事实所有者发布 `WorkItemProjection`，Workbench 只按当前 Subject 聚合导航与局部故障，不保存完成状态或代理写命令。
6. `empty`、`unavailable`、`forbidden`、`not_selected` 和 `revision_changed` 是不同事实；金额不可见时为 redacted/null，不是 0。
7. 第一版使用有界实时 fan-out；只有真实延迟、离线或规模证据证明不足时才审查可重建物化投影。

## 对“最小应用 + 可扩展”的影响

最小组合可以只选 Project、Evidence 和一个产生资料／工作项的领域应用；新增第二应用时只需装入 Suite Manifest、IAM Catalog、来源投影和连接器，不修改现有应用数据库。某个客户只需要资料目录时，不必启动公司统计；只需要项目经营分段时，不必安装个人工作聚合。可扩展性来自版本化投影和显式选装，不来自提前建设万能报表平台。

## 下一批实现

下一批保持低资源串行：先完成 Evidence metadata/download 分权与 Project 单项目资料组合，只运行相关单元测试；随后用一个临时 PostgreSQL、Project 与 Evidence 两个进程验证直接 Evidence Link、相邻 File 拒绝、目标停机／恢复和零残留。个人工作与多来源统计在该批接受后另开轮次，避免同时启动完整组合。

## 未通过项

- C8 运行 API、页面、正式 IAM Catalog、D1 与恢复尚未实现；
- 业务应用还没有统一 Evidence 引用集合、MetricProjection 和 WorkItemProjection；
- 公司跨来源统计公式没有业务确认；
- 完整双 Tenant、真实 Ubuntu 和公网入口仍是独立环境证据轴。

## 稳定载体

- [C8 产品规格](../../arckit/spec/assembly/c8-material-statistics-personal-work.md)
- [C8 技术决策](../../arckit/tech/assembly/c8-read-projection-boundary.md)
- [OA 完整体规格](../../arckit/spec/assembly/full-reference-oa-validation.md)
- [决策总表](../decisions.md)
