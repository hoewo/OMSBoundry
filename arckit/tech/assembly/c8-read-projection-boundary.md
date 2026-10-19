# C8 只读投影与个人工作装配边界

## 元信息

- 状态：🟡 direct Project→Evidence 真实运行支持，Project 资料页面、首个 Business Engagement 间接资料源、Production→Workbench 个人工作，以及 Money 指标来源与 Workbench 单消费者完成零容器实现验证，真实浏览器／间接运行链／正式 Gateway／IAM／D1 待验
- 关联规格：[C8 项目资料、统计与个人工作](../../spec/assembly/c8-material-statistics-personal-work.md)
- 决策：第一版采用 Project 范围实时组合、来源自有统计／工作项投影和 Workbench 导航聚合，不新增持久化报表数据库

## 现有证据

参考 OA 当前把 Project、File、FinancialRecord、Bonus 和用户数据放在同一数据库中：资料页先读项目目录，再按展开项目读取文件与下载统计；公司统计按权限读取公司汇总和项目台账；人员奖金按人员／项目聚合；立项待办由当前账号关联的 draft Project 查询产生。该实现提供了可学习的用户结果，但直接联表、`float64` 金额、统一 FinancialRecord 和站点级权限不能成为新架构契约。

BizFoundry 已具备可复用的前置能力：

- Project Workspace 拥有 Project、Assignment、Resource Link，并已有 C3 有界并行组合；
- `evidence-service/evidence-file` 已是合法 Resource Link，Evidence 发布精确 File Version 引用；
- Evidence 生成短期 Access Grant，不向业务应用暴露物理对象；
- 各领域应用已有原生 Capability、精确资源工作负载凭证和局部失败语义；
- Workbench 从 Suite Manifest 发现入口，但尚未拥有工作项聚合能力。

## 采用的形态

```text
Browser
  -> Nginx -> Gateway -> IAM
  -> Project Workspace / Unified Workbench
       -> IAM workload exchange (single action + exact scope)
       -> source read projections
       -> Evidence metadata only

Download:
Browser -> Gateway -> IAM -> Evidence access grant -> object data plane

Personal work:
Browser -> Gateway (current Suite Session -> path-scoped HttpOnly capability)
        -> source WorkItemProjection v1 -> source deep link
```

Project Workspace 只组合一个 Project 的 Resource Link、来源投影和 Evidence 元数据。Workbench 只组合当前 Subject 的来源工作项。两者都不保存来源业务事实；第一版进程重启不需要重建报表库。

## 版本化端口

### `ProjectMaterialSegment v1`

最小输出：Project ID／revision、Link ID、来源应用、资源类型／ID／revision、关系键、Evidence File ID／Version、文件显示元数据、分段状态、错误码、观测时间、耗时和来源深链。Project 不返回 Object Key、AccessKey、长期 URL 或来源内部表 ID。

首个间接来源采用 `BusinessEngagement MaterialReferenceProjection v1`。Business Engagement 只发布 active Evidence 引用、所属 Project、source revision／digest、requirement key 和显示快照；Project 先以精确 Engagement Scope 读取该投影，再以自己的精确 File Scope 读取 Evidence metadata。Project 不持久化投影，不导入 Engagement 领域模型，也不因为获得列表能力而获得下载能力。

### `MetricProjection v1`

最小输出：来源、稳定 metric key、Project／Tenant／Subject 等显式 scope、单位、精确值或 redacted、口径版本、来源 revision／digest、时间范围、观测时间和深链。聚合器只对契约明确声明可合计的相同 metric key／unit／currency／definition version 执行精确运算。

当前发布 Schema 位于 `contracts/read-projections/v1/metric-projection.schema.json`。Money 首来源固定 `GET /api/v1/metrics/operating-summary` 与 `money-flow-operating-summary-v1`；范围为 Tenant＋Ledger Owner Party＋Currency，时间口径为 current。`statistics.read` 和 `amount.read` 分权：前者缺失直接 403，后者缺失时仍可返回结构，但值与摘要必须同时为 null，避免用摘要字典反推小值域金额。来源计算失败为 503，响应不可缓存。

### `WorkItemProjection v1`

最小输出：来源、稳定工作项 ID、种类、当前 Subject、目标资源 Scope、原因、状态、原生 action、资源 revision、可选期限、观测时间、深链和目标端 allowed actions。来源端点必须按当前 Subject 与资源授权过滤；Workbench 不接受浏览器提交的 Subject 作为真值。

## 权限链

Project 材料读取、人类文件下载、统计金额读取和工作项读取使用不同 Capability。Project 的后端组合使用 workload token，只允许调用来源只读端口。个人工作第一版必须体现当前人 Grant 与 Subject，因此由 Workbench 页面经 Gateway 使用 Suite Session 换取来源短期人类 Credential；Credential 保存在来源路径限定的 HttpOnly Cookie，Workbench 不读取 Token、不提交 Subject、不签发能力，也不能代替人类获得下载、导出或业务写权限。若未来改为 Workbench 后端聚合，必须先有受信 Policy Provider 的显式 on-behalf-of 契约，不得直接用服务身份伪装个人。

Evidence 已把当前共用的 `evidence-service.view` 兼容演进为 `metadata.read` 与 `download` 两个动作：元数据端点接受 `metadata.read` 或旧 `view`，Access Grant 端点接受 `download` 或旧 `view`；新 Project workload 只获 `metadata.read`。旧动作仅作为现有消费者的有界迁移兼容，不会让新 metadata-only 凭证生成下载授权，也不会扩大到相邻 File。

## 实时组合算法

1. 目标应用先复验人类 Credential 与 Project／Subject Scope；
2. 从本地事实读取 active Link 或已选来源清单；
3. 为每个来源交换单 action、精确资源 workload Credential；
4. 使用固定并发上限、逐来源超时、1 MiB 响应上限和严格 JSON；
5. 校验 schema、source、Tenant、resource ID、revision、unit 和摘要；
6. 保持输入顺序，返回逐段状态、耗时与统一 `observed_at`；
7. 401 只允许失效缓存凭证并换票一次；404／422 为 invalid，未选为 not_selected，其他目标失败为 unavailable；
8. 不因一个来源失败取消已完成分段，也不把失败分段写成 empty／zero。

缓存若启用，只保存短期展示结果，并同时受来源 revision、Subject、Tenant、Scope 和授权到期约束。缓存命中不能跳过目标端授权，撤权后旧缓存不能继续显示敏感金额或生成下载。

## 放置决策

- 项目资料与项目级状态放在 Project Workspace，因为它已经拥有 Project 与 Resource Link 上下文；
- 个人工作入口放在 Unified Workbench，但工作项事实与状态仍归来源应用；
- 公司／人员统计第一版按来源分区呈现，不在 Workbench 或 Project 中建立统一可写模型；
- 只有跨来源公式经过业务确认且实时组合有测量证据不足时，才考虑独立的可重建 reporting-view；该服务仍不是业务事实所有者。

## 第一实现顺序

1. ✅ 兼容拆分 Evidence metadata／download Capability，并补精确 Scope 负例；
2. 🟡 Project Workspace direct Evidence Resource Link 的项目资料只读 API 已通过真实 IAM／PostgreSQL／独立进程和停机恢复；人类建立／浏览入口已实现并通过零容器单元验证，真实浏览器和正式 D1 点击待补；
3. 🟡 Business Engagement 已补 Evidence 引用投影并通过直接 Link／来源 Link、两层 revision 和局部失败单元验证；真实 IAM／独立进程与正式 D1 待验；
4. ✅ Production Operations 发布当前 Subject 的 `WorkItemProjection v1`，Workbench 依据 Manifest 聚合单来源；零容器单元／契约已通过，正式入口运行待验；
5. ✅ Money Flow 发布首个 `MetricProjection v1` 来源，指标／金额能力分权并通过零容器单元／契约验证；
6. 🟡 Suite Manifest 与 Unified Workbench 装入首个 Money 指标消费者，来源分区、脱敏、严格 Tenant／契约校验和局部失败完成零容器单元验证，真实入口待验；
6. 把单一指标来源按 Manifest 装入消费者并验证脱敏、来源失败和未选缺席；
7. 增加第二工作项或指标来源与局部故障／撤权，证明不是硬编码；跨来源公式继续失败关闭。

每一步独立验证且结束后停止进程。D1 完整组合只在小批次通过后执行，不把多个尚未稳定的来源同时拉起。

## 首个间接来源单元证据（2026-09-29）

- Business Engagement 发布确定性排序、带 digest 的当前 Evidence 引用投影，并使用独立 Capability；
- Project 新增 `business-engagement/business-engagement` Resource Link 与前向 migration，只经版本化 HTTP 端口组合；
- Project 分别保存来源 linked/current revision 与 Evidence linked/current revision，来源或单个文件失败只影响自己的分段；
- 错 Tenant、错 Project、错 source、非法 digest 和相邻 Scope 失败关闭；响应模型没有物理存储坐标；
- Business Engagement、Project 和 Access Contract 三组测试使用 `GOMAXPROCS=2`、`go test -p 1` 串行通过，全程未启动 Docker。

该证据只支持单元／契约层结论，不替代 migration 真库、真实 IAM 换票、浏览器、D1 装配和恢复。

## 首个个人工作来源单元证据（2026-09-29）

- Production Operations 新增独立 `production-work-item.work-items.read`，从 IAM Credential 读取 Subject，结合 current Work Item、active direct-person `production_owner` Assignment、资源 Scope 和仍获授权的写动作发布投影；
- ended／stale Assignment、相邻 Subject／Project、完成／取消状态和已撤销可执行动作不产生新可执行条目；Project 引用服务不可用返回来源级 503，不伪装为空；
- Suite Manifest 可选 `work_item_projection v1` 声明来源路径和 action。Workbench 只发现 enabled 来源，以四路上限和 3.5 秒逐来源超时读取，严格校验 schema／source／Subject，并在当前来源路由内构造深链；
- Workbench 不保存业务状态、不代发写命令。Production 深链载入 `work_item_id` 后仍重读目标详情、当前 revision 和 `allowed_actions`；
- Production、Workbench、Gateway 与 Access Contract 使用 `GOMAXPROCS=2`、`go test -p 1` 串行通过，两个嵌入 JavaScript 语法通过，全程未启动 Docker。

当前判定为 `c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`。真实 Gateway→IAM→Production Credential、Grant 撤销时序、账号停用、正式 D1、第二来源局部失败、Group Assignment→Person 解析和真实 Ubuntu 尚未验证。

## 首个类型化指标来源单元证据（2026-09-29）

- Money Flow 发布六个 current Operating Summary 指标，复用来源既有精确十进制计算，不在组合层复制公式；
- 指标身份由 metric key、Tenant、Ledger Owner Party 和 Currency 共同确定，口径版本、观测时间、相对深链及来源摘要显式返回；
- `money-flow-ledger.statistics.read` 与 `money-flow-ledger.amount.read` 分权；没有金额能力时响应不含值，也不含可能被字典枚举的摘要；
- 缺结构能力 403 且不读取来源，来源失败 503，响应 `no-store`；
- Money Flow 全模块与 Access Contract 使用 `GOMAXPROCS=2`、`go test -p 1` 串行通过，全程未启动 Docker。

当前判定为 `c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending`。Suite Manifest／Workbench 已完成单来源消费者的静态与单元证据；真实 Gateway／IAM／Money Credential、浏览器、第二来源运行时分区、正式 D1、跨来源公式与真实 Ubuntu 尚未验证。

## 首切片实现证据（2026-09-29）

- Evidence 与 Project 两个 Go module 分别以 `GOMAXPROCS=2`、`go test -p 1 ./...` 串行通过；
- metadata-only／download-only／legacy-view、精确 File Scope、相邻 Project、错 Tenant、stale revision、403、来源不可用与 empty 均有定向断言；
- Access Contract 的发布示例、Project workload 精确 `evidence-service.metadata.read:file` 权限和 Manifest 负例定向通过；
- 本批 Docker 容器数始终为 0，没有数据库或真实应用进程证据；正式判定为 `c8-first-backend-slice-unit-supported / runtime-assembly-pending`。

## 最小运行组合证据（2026-09-29）

- `acceptance/c8-material-composition/run-isolated` 强制 Docker 从 0 开始，只启动一个 PostgreSQL 18.6 容器；IAM、Evidence、Project 为三个本地独立进程，退出后容器回到 0；
- Project workload 使用独立 Ed25519 私钥向真实 IAM 换取单 `evidence-service.metadata.read`、精确 `file:{id}` 凭证，IAM audit 为 1，download audit 为 0；
- direct current Link 返回 `available`，冻结旧 revision 返回 `revision_changed`，相邻 Project Credential 返回 `403 scope_denied`；
- Evidence 停机时分段返回 `unavailable/source_unavailable`，同库恢复后重新为 `available`，Project 响应始终不暴露 Bucket／Object Key；
- 当前判定提升为 `c8-direct-material-runtime-supported / human-page-and-d1-pending`。证据见 [运行实验](../../../docs/reports/oa-c8-project-evidence-runtime-experiment-20260929.md)。

## 人类资料工作区首批证据（2026-09-29）

- Project 记录页新增按 `allowed_actions` 收敛的“项目资料”页签，资源连接支持 Evidence File，资料列表展示来源状态、revision、摘要、类型与大小；
- Suite 下载入口先为 Evidence Service 独立授权，Project workload 仍没有 download 权；direct 开发入口不伪造 Gateway 下载链；
- Project module 以 `GOMAXPROCS=2 GOWORK=off go test -p 1 ./...` 串行通过，嵌入 JavaScript 语法和 8 个交互线框状态结构通过；
- 浏览器保持模式重跑后端时仍只使用一个 PostgreSQL 容器并清回 0；macOS 锁屏阻止真实浏览器点击，因此没有将该项标记通过；
- 当前判定为 `c8-human-material-experience-unit-supported / browser-and-d1-pending`。证据见 [首批实现与低资源验证](../../../docs/reports/oa-c8-project-material-human-experience-unit-validation-20260929.md)。

## 被拒绝方案

- Workbench 保存统一待办表：会接管来源状态并制造双写；
- Workbench workload 直接携带浏览器 Subject：服务凭证不代表人的 Grant，会形成身份代理与撤权漏洞；第一版改由 Gateway 颁发当前人的短期来源凭证；
- 新建共享 Reporting 数据库：当前没有性能或离线证据，先制造同步与恢复问题；
- Project 复制业务应用表：破坏独立迁移和数据所有权；
- Evidence 全局搜索再按名称猜项目：Evidence 不拥有业务含义，且会扩大权限范围；
- 浏览器并行直连所有应用后自行合计：无法统一复验契约、故障语义、脱敏与审计；
- 无权限金额置 0：会把“未知／不可见”伪装为真实零并泄露聚合关系。

## 后续实现门槛

`ProjectMaterialSegment v1`、Evidence Capability 兼容策略、`WorkItemProjection v1`、首个 `MetricProjection v1` 来源及其 Workbench 单消费者已固定。下一步优先验证真实 Gateway→IAM→Money Credential、真实浏览器和正式 D1；第二来源只用于证明运行时分区与局部失败，不得借机实现未经确认的跨来源公式。公司跨来源指标在业务公式、币种、时间口径、缺失值和纠错责任未确认时不得实现。
