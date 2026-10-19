# C8 项目资料、统计与个人工作

## 元信息

- 路径：`spec/assembly/c8-material-statistics-personal-work.md`
- 所属模块：`assembly`
- 状态：🟡 direct Project→Evidence 已通过真实 IAM／PostgreSQL／独立进程验证；Project 资料页面、首个 Business Engagement 间接资料源、首个 Production WorkItemProjection→Workbench，以及首个 Money MetricProjection 来源与 Workbench 单消费者已通过零容器单元验证，真实浏览器／间接运行链／正式 Gateway／IAM／D1 待验
- 优先级：P0
- 依赖能力：IAM、Project Workspace、Evidence Service、Unified Workbench，以及被选装业务应用的版本化只读投影
- 上层旅程：[OA 完整体 C8](./full-reference-oa-validation.md#c8资料统计与个人工作)

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 的资料目录、公司／项目／人员统计和首页待办依赖同一数据库中的 Project、File、FinancialRecord、Bonus 与人员表；这种实现能快速形成页面，但会把读取权限、金额语义、文件访问和待办状态绑在一个定制系统中，无法证明小应用仍可独立交付。

**可验证结果 B**：用户从 Project 或统一工作台进入后，只看到自己有权访问的项目资料、类型化统计分段和真实可执行待办；每个值都能追溯到唯一来源应用、资源 ID、revision、观测时间和纠错入口，一个来源故障只使对应分段降级，不伪装成空、零或已完成。

## 第一版产品边界

C8 是只读装配旅程，不是新的业务事实所有者，也不是通用 BI 平台。第一版不建立报表数据库、共享数据仓库、跨应用 ORM、全量同步任务、通用查询语言或可写待办表。

- Project Workspace 负责项目身份、Assignment、Resource Link 和项目范围的无状态组合；
- Evidence Service 负责文件字节、版本、摘要、存储状态和短期访问授权；
- Contract、Money、Billing、Production、Results、Cost、Bonus 等应用分别拥有其统计输入和工作状态；
- Unified Workbench 负责当前 Subject 的应用入口、工作项导航和来源状态聚合，不拥有业务完成事实；
- IAM／Policy Provider 负责把 Subject、Tenant、Capability Manifest 与 Project Assignment 等事实收敛成短期目标应用凭证；目标应用继续最终授权。

“只读”只描述 C8 组合层。用户从分段或工作项进入来源应用后，仍可在目标应用 `allowed_actions` 允许时执行写命令；C8 自身不能代理这些命令或改变来源状态。

## 资料旅程

### 项目资料目录

第一版资料目录从一个已授权 Project 开始，不允许按租户全局扫描 Evidence：

1. Project Workspace 读取自身 active Resource Link；
2. 直接 `evidence-file` Link 使用精确 File ID／Version，业务应用资源则读取来源自有的最小引用投影及其中显式 Evidence 引用；
3. Project Workspace 以工作负载凭证逐文件读取 Evidence 元数据，只返回文件名、版本、类型、大小、摘要、业务来源、关系键、来源 revision、状态和深链；
4. 浏览器请求下载时必须另行取得 `evidence-service.download` 的精确 File Scope，Evidence 再生成短期 Access Grant；列出资料不自动获得下载能力；
5. 文件与业务事实的含义、分类和纠错入口由引用它的业务应用拥有，Evidence 不解释“合同、票据、成果或批复”。

Project Workspace 可以提供项目分组和局部降级，但不得复制文件字节、物理 Object Key、长期 URL 或业务应用内部文件表。资料搜索必须带 Project Scope；跨项目搜索留待出现稳定用例后再审查。

### 资料分段状态

每个资料分段必须显式返回以下状态之一：

- `available`：来源引用和 Evidence 版本均可读取；
- `revision_changed`：来源 current revision 与 Project Link 冻结 revision 不一致；
- `invalid`：资源不存在、Tenant／类型不匹配或引用不符合契约；
- `unavailable`：IAM、来源应用或 Evidence 暂不可用；
- `forbidden`：当前 Subject 没有读取该分段的业务能力；
- `not_selected`：Suite Manifest 未选择来源应用；
- `empty`：来源成功返回且当前项目确实没有对应资料。

`unavailable`、`forbidden`、`not_selected` 和 `empty` 不得相互替代。已有业务快照可以解释历史，但需要新鲜授权的下载在 IAM／Policy／Evidence 故障时失败关闭。

## 统计旅程

统计不以“把所有金额拉到一起相加”为默认目标。第一版只展示来源域明确发布的类型化指标或项目分段：

| 统计视角 | 第一版来源 | 组合责任 | 禁止行为 |
| --- | --- | --- | --- |
| 项目经营 | Project 的 C3 分段，来源为 Contract、Money、Billing | Project Workspace 保持来源顺序和局部状态 | 不跨来源猜测利润或把未知金额当 0 |
| 项目生产／结果／成本／奖金 | Production、Results、Cost、Bonus 的版本化投影 | Project Workspace 分区呈现 | 不用 Project 表复制当前业务状态 |
| 公司经营 | 各来源应用自己的公司范围指标分段 | 独立只读页面按来源分区；第一版不形成跨来源总数 | 不复制参考 OA 的“全收入 × 管理费率”算法 |
| 人员奖金／个人结果 | Bonus 等事实所有者的 Subject／Person 范围投影 | 统计页按明确 Capability 展示 | 不以姓名关联、不把无权金额显示为 0 |

每个指标至少包含 `schema_version`、`source_application`、`metric_key`、`scope`、`unit`、`value` 或 `redacted`、`source_revision`／`source_digest`、`observed_at` 和 `deep_link`。金额使用来源应用的精确表示；不同币种、口径、时间范围或语义版本不得静默合计。

如果以后确实需要跨来源公式，必须先形成独立版本化 `report_definition`，声明公式、单位、时间口径、来源指标版本、舍入、缺失值和纠错责任。没有该证据时不建立持久化报表服务；测得的延迟或可用性压力出现后，才评审可重建的物化只读模型。

## 个人工作旅程

各业务应用发布版本化 `WorkItemProjection`，说明当前 Subject 为什么可以或需要执行一个动作。最小字段为：

- `schema_version`、`source_application`、`work_item_id`、`kind`；
- `tenant_id`、`subject_id`、可选 `project_id` 与目标资源 Scope；
- `reason`、`state`、`action`、`resource_revision`、可选 `due_at`；
- `observed_at`、`deep_link` 和目标应用计算的 `allowed_actions`。

Workbench 只对当前会话 Subject 并行读取已选装来源，按来源分组、去重和排序，不自行从“草稿”“未付款”“奖金已发布”等状态猜测待办。人类页面通过 Gateway 为当前 Session Subject 取得来源短期 Capability Credential；Token 只存在于来源路径限定的 HttpOnly Cookie，Workbench 不接收 Subject 输入，也不读取或签发 Token。点击工作项只进入来源应用；真正命令仍由目标应用复验能力、Scope、当前状态和 expected revision。

Project Assignment 只帮助 Policy Provider 限定候选 Subject／Project，不直接证明某个领域动作尚待执行。Assignment 结束或 Grant 撤销后不得产生新的可执行工作项；历史业务记录仍按来源规则可追溯。来源故障显示为局部 `unavailable`，不能把工作项静默删除或标记完成。

## 授权与敏感信息

第一版至少区分：

- `project-workspace.materials.read`：读取一个 Project 的资料分段；
- `evidence-service.metadata.read`：读取精确 File Version 元数据；
- `evidence-service.download`：生成精确 File Version 的短期下载授权；
- 各来源应用自己的 `statistics.read`、`amount.read` 与 `work-items.read`；其中个人工作读取必须绑定当前 IAM Subject，不能用 Workbench 服务身份冒充个人；
- `unified-workbench.work-items.read`：读取当前 Subject 的工作项装配结果。

列表、元数据、预览、下载和导出不是同一能力。无金额能力时值必须为 `null`／`redacted`，不返回真实值也不使用 0 占位；导出使用独立 Capability、服务端再次授权和版本化 manifest。身份超级管理员不因管理账号自动获得项目、文件、统计或待办业务权。

## 技术约束与失败恢复

- 第一版使用有界并发、逐来源超时的实时组合，不持久化外部业务事实；
- 缓存只允许保存短时呈现结果，键必须包含 Tenant、Subject、Scope、来源版本与授权上下文；缓存不得用于签发能力或延长撤权窗口；
- 一个来源超时只降级对应分段，其余来源继续返回；整体响应说明 `observed_at`、每段耗时和错误码；
- `GET` 纯读，不记录“已处理”、不增加下载次数、不改变待办状态；下载统计只在 Evidence Access Grant／真实访问边界按明确规则记录；
- 未选装来源不发起网络请求；重新选装后从来源 current 投影恢复，不回填共享表；
- 任何物化读模型若后续被证明必要，必须可从版本化来源重建、保留 provenance／as-of、按 Tenant 隔离，并且仍不能成为写命令依据或业务真值。

## C8 第一实现验收

- C8-A01：资料目录只返回当前 Subject 有权访问的 Project；
- C8-A02：资料搜索和展开始终带 Project Scope，不扫描租户全部 Evidence；
- C8-A03：直接 Evidence Link 与来源业务 Evidence 引用均保留来源应用、关系键和 revision；
- C8-A04：资料列表能力不能生成下载授权，下载能力不能扩大到相邻 File；
- C8-A05：文件名、版本、摘要和短期 Access Grant 由 Evidence 提供，Object Key 与长期凭证不泄露；
- C8-A06：来源／Evidence／IAM 单点故障只降级对应分段，其他分段仍可读；
- C8-A07：`empty`、`unavailable`、`forbidden` 与 `not_selected` 可区分；
- C8-A08：Project 经营统计复用类型化来源分段，不建立跨库联表或可写汇总；
- C8-A09：未经版本化公式确认，不生成跨来源公司利润、收入或余额总数；
- C8-A10：不同币种、单位、口径或语义版本不静默合计；
- C8-A11：无金额能力时金额为 `null`／`redacted`，不能以 0 或真实值泄露；
- C8-A12：人员统计使用稳定 Subject／Person ID 与来源 revision，不按姓名关联；
- C8-A13：个人工作项只由事实所有者声明，Workbench 不推断、不保存完成状态；
- C8-A14：Assignment 结束、Grant 撤销、账号停用和凭证过期后新工作项／动作失败关闭；
- C8-A15：工作项点击后由目标应用按 current state 和 expected revision 再授权；
- C8-A16：导出与页面读取分权，归档带版本化 manifest、来源和时间口径；
- C8-A17：实时组合有并发／超时上限，缓存不延长授权且没有隐藏强依赖；
- C8-A18：未选 C8 页面或任一来源时，进程、路由、菜单和请求均真实缺席，其他应用独立工作。

## 当前实现事实

Project Workspace 已新增受 `project-workspace.materials.read` 与精确 Project Scope 保护的 `GET /api/v1/projects/{project_id}/materials`，以最多四路并发组合 active direct Evidence Link 和受支持的业务资料源。Evidence Service 已兼容拆出 `evidence-service.metadata.read` 与 `evidence-service.download`；metadata-only 不能生成 Access Grant，download-only 不能读取元数据，旧 `evidence-service.view` 暂时保留有界兼容。direct 最小运行组合已用一个 PostgreSQL 容器、IAM／Evidence／Project 三个独立进程验证真实 workload 换票、精确 File／Project Scope、stale revision、Evidence 停机逐段 unavailable、恢复 available 和清理回 0 个容器。Project workload 没有 download 权，响应不暴露 Bucket／Object Key。Project 人类工作区可建立 Evidence 或 Business Engagement Resource Link、按需浏览资料状态，并在 Suite 路径中先独立授权 Evidence 再进入下载深链。

首个间接来源由 Business Engagement 发布 `MaterialReferenceProjection v1`：只含当前 Evidence 引用、Project ID、业务来源 revision／digest 和显示快照。Project workload 使用独立 `business-engagement.material-reference.read + business-engagement:{id}` 读取投影，再为每个 File 独立请求 `evidence-service.metadata.read + file:{id}`；两层 revision／digest、业务关系键和局部失败分别保留。Business Engagement、Project 与 Access Contract 串行测试已覆盖错 Tenant／Project／source、业务来源拒绝、单 File 不可用和 drift，全程零容器。

首个个人工作来源由 Production Operations 发布 `WorkItemProjection v1`。来源按当前 IAM Credential Subject、精确 Project／Work Item Scope、active direct-person `production_owner` Assignment、当前工作状态和仍获授权的可执行动作过滤；结束／stale Assignment、相邻 Subject、无可执行动作均不产生条目，Project 引用服务不可用则来源失败关闭。Suite Manifest 以版本化 `work_item_projection` 声明路径和 action，Workbench 浏览器最多四路、每来源 3.5 秒读取，校验 schema／source／Subject，按来源分组、排序、去重并只进入来源深链。Production 页面收到 `work_item_id` 后重读 current 详情与 `allowed_actions`。Production、Workbench、Gateway 和 Access Contract 的串行单元测试及嵌入 JavaScript 语法已通过，全程零容器。

首个类型化指标来源由 Money Flow 发布 `MetricProjection v1`：从自身 current Operating Summary 输出应收未结、应付未结、有效收款、有效付款、未核销收款和未核销付款。每项保留 Tenant、Ledger Owner Party、Currency、`money-flow-operating-summary-v1`、精确十进制值、观测时间、来源摘要和深链。`money-flow-ledger.statistics.read` 只允许读取指标结构；只有同时具有 `money-flow-ledger.amount.read` 才返回金额与摘要，否则 `value`／`source_digest` 同时为 null 且 `redacted=true`。来源失败返回 503，不伪装为空或零。Suite Manifest 已以可选 `metric_projection` 声明来源；Workbench 严格校验 source／Tenant／Scope／精确值或 redaction，按来源、Ledger Owner Party、Currency 分区展示，不形成跨来源总数，并把单一来源失败限制在自身分区。来源、Workbench、Gateway Manifest 与 Access Contract 已低资源串行通过；正式 Gateway→IAM→Money、真实浏览器、第二来源运行时和 D1 仍待验。

当前缺口包括：Project 资料页尚未完成真实浏览器点击；间接来源尚未经过真实 PostgreSQL／IAM／独立进程或正式 Nginx→Gateway 入口；个人工作尚未经过真实 Gateway→IAM→Production 浏览器链、正式 D1、第二来源局部失败或 Group Assignment 解析；Money 指标消费者尚未验证真实 Gateway→IAM→Money Credential、真实浏览器、Grant 撤销、第二来源运行时分区或正式 D1；D1 装配／未选缺席、联合恢复、双 Tenant 和真实 Ubuntu 仍未验证；除 Money 外其他类型化指标和除 Production 外其他 `WorkItemProjection` 尚未实现；公司统计公式与跨来源语义也未获得足够证据。旧 `evidence-service.view` 的兼容撤除必须等所有既有消费者迁移并复验后单独决定。

## 不进入第一版

- 通用 BI、数据仓库、OLAP、任意报表设计器或跨库 SQL；
- 可写统一待办、通用工作流、消息中心或自动替来源应用完成动作；
- 以页面菜单、客户岗位名、文件目录名或姓名作为授权／关联真值；
- 全租户 Evidence 搜索、永久下载 URL、客户端长期 OSS 密钥；
- 未经语义确认的公司收入、利润、管理费、工资或奖金总额；
- 把投影缓存、导出文件或统计表作为业务写入依据。

## 复审条件

真实数据证明实时组合无法满足延迟／可用性目标；跨来源公式得到业务责任人确认并在第二个客户中稳定复用；工作项数量或离线要求需要可重建索引；资料跨项目检索成为稳定需求；或目标环境要求审计留存每次报表快照时，重新审查是否引入独立、可重建且只读的物化投影服务。
