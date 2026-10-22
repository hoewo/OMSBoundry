# 小系统边界演变账

2026-10-06 BF-120 状态解释：本账保存的 Owner、依赖和兼容分析继续作为候选边界材料；其中 BF-112～BF-118 的“运行通过”不再代表当前实现完成度，须在 R0 清点后按 C1～C8 门禁复验。当前三系统仍为候选边界，下一工作改为[基线收敛与缺口矩阵](../../status/current-baseline-and-next-actions-20261006.md)，不直接进入旧计划中的 F01／F02。

状态：持续维护，当前基线日期 2026-10-06。  
适用范围：参考 OA 仿制期间的 Project Workspace、Production Operations、Contract Ledger 及其直接跨系统依赖。

问题 A → 可验证结果 B：OA 仿制会不断要求小系统增加真实能力，如果只记录“页面完成”或“仍是多个服务”，边界会在不知不觉中漂移 → 每个纵向切片都留下事实所有权、命令契约、依赖方向、兼容方式、失败语义和反例证据，使下一项目能够判断哪些边界已经可复用、哪些仍只是当前客户候选。

## 1. 这份账回答什么

“没有合并成一个大系统”只是最低守卫，不是架构验证成果。本账进一步回答：

1. 小系统在真实 OA 操作压力下新增了什么事实和规则；
2. 新能力是否仍由唯一事实所有者写入，还是形成了双写、共享表或第二套真相；
3. 新依赖是身份授权、写入时精确引用校验，还是工作台只读组合；
4. 上游故障、响应丢失、重放和版本升级时，已确认事实能否保持且可续办；
5. 同一边界能否支持下一种生产类型、下一条项目链或下一客户，而不改写旧事实。

静态审计 `make architecture-boundary-static-audit` 当前通过：15 个应用、跨应用 import 0、跨应用 replace 0、跨数据库 SQL 机制 0。它只证明代码与数据库的最低隔离守卫，没有证明 API 语义稳定、运行时依赖可恢复或跨客户适用。

## 2. 判定等级

| 等级 | 含义 | 当前允许的表述 |
| --- | --- | --- |
| 候选边界 | 已有唯一所有者、版本化接口和至少一个真实纵向切片，但仍可能被同客户后续操作击穿 | “当前 OA 切片下边界成立” |
| 可重复边界 | 同类第二场景或第二类型无需改变所有权，只通过兼容版本、配置或 Overlay 扩展，并完成故障／恢复 | “同类场景可重复” |
| 跨客户稳定边界 | 第二真实客户在不 fork Base、不共享业务表的前提下完成升级与验收 | “跨客户稳定” |

目前 Project、Production、Contract 均只到“候选边界”；不得因为静态审计通过、服务可独立启动或当前 OA 页面可用而提升等级。

### 2.1 边界收敛定义（BF-119）

边界稳定不是“不再变化”，而是变化随真实场景增加逐步变小、变少、局部化并可预测。验证期大量变化本身不是失败；没有记录的变化、跨 Owner 扩散和每个新场景都重写旧事实才是失败。

| 阶段 | 允许的变化 | 必须出现的趋势 |
| --- | --- | --- |
| 探索期 | Owner、契约、依赖时机和内部模型可被真实操作反复证伪 | 每次变化均有决策、版本、兼容／迁移、恢复和反例证据 |
| 收敛期 | 继续扩展能力，但优先复用既有 Owner 和直接端口 | 变化涉及的 Owner、公开契约、migration、强制依赖和旧场景回归面总体下降 |
| 稳定窗口 | 仍可兼容演进，不要求永久冻结 | 大部分新需求落在同一 Owner 内或 Overlay／配置／策略／连接器／只读投影；Base 变化少、局部、可预测且向后兼容 |

每个后续 OA 纵向切片至少记录以下五项，以连续趋势判断而非用单次阈值冒充成熟：

1. 本切片改变了几个事实 Owner，哪些 Owner 只读、哪些发生写边界变化；
2. 新增或修改了多少公开契约、Capability、migration 和版本过渡；
3. 新增了多少强制依赖边，是否扩大逐请求同步调用链；
4. 哪些既有纵向场景需要回归或迁移，旧 ID、revision、摘要和历史是否保持；
5. 变化最终落在 Base、类型化扩展、Customer Overlay、配置、策略、连接器还是只读投影，理由是什么。

以下任一现象持续出现即视为边界未收敛：无关 Owner 必须同时改动；Workbench／Gateway 开始保存业务真相；共享表、跨库写、双写同步或核心长期 fork；为了新场景原位改写旧版本含义；依赖图和失败面随每个页面线性扩张。此时不得用“仍是多个服务”作为通过结论。

当前完整 OA 完成度仍低，处于探索向收敛过渡阶段。Project、Production、Contract 整体继续只标记为候选；局部子边界达到可重复，也不自动提升整个应用或架构等级。

## 3. Project Workspace：从薄项目壳演变为项目上下文所有者

### 初始边界

Project 拥有 Project 身份、生命周期、层级、Assignment、Resource Link、revision 与历史；不拥有合同、资金、文件、主体、生产任务或人员档案。

### 当前演变

- BF-177 为“先维护甲方／施工方档案，后建项目”补充租户级 `project-party-candidate`。该事实由 Project Workspace 拥有，因为它只表达某 Party 是否可进入项目甲方／施工方选择器；Party Directory 仍独占主体档案，具体项目仍以 `client_party` 快照和 `contractor-party` Resource Link 为准。新增一条 Project migration 和 GET／POST 端口，不增加跨库写、共享表或逐请求同步链；登记时通过版本化 Party 引用端口校验稳定 ID 与 revision。
- 立项前派发和待立项草稿允许编号／类型稍后补齐；
- Profile Correction、Initiation Info Change、Basic Info Change 把原 OA 的补录、更正和负责人原子变更纳入 Project 自有历史；
- Counterparty Context 保存“本项目发生时的业务联系快照”，但不替代 Party Directory 当前主档；
- `commercial_contributor` 与六类参考 OA 生产分工作为有界 Overlay，不扩大通用 Base Role；
- Project Completion Profile／Checkpoint 表达项目级闭环条件，不复制 Results、Production、Cost 等来源事实；
- Production、Results、Cost、Contract、Billing、Business Engagement 等仍只通过类型化 Resource Link 接入。

当前能力目录继续复用既有读取／创建能力；数据库尾部迁移为 `000012_reference_oa_production_personnel.sql`、`000013_party_directory_resource_links.sql`、`000014_project_party_candidates.sql`。当前候选定义应写成：

> Project Workspace 拥有项目身份、生命周期、项目内角色与上下文、外部资源关系及项目级检查点；它不拥有被引用领域的内容、状态机或业务历史。

### 边界变化与风险

这是三者中依赖面扩张最大的系统。当前 Workload Policy 中 Project 向 10 个 audience 发出 11 项权限，6 个 workload 向 Project 发出 7 项权限。风险不在“依赖数量大”本身，而在以下三点：

1. Tenant 专业目录当前随参考 OA 生产分工落在 Project Overlay；若以后脱离项目分工独立演化，应重新判断是否属于 People，而不能默认继续扩 Project；
2. 来源系统的 `project_id` 与 Project Resource Link 同时存在时，必须明确一个是来源自身关联键、一个是 Project 自有装配事实，不得双向同步成两套权威状态；
3. Project 可以发起精确引用校验和保存 Link，但不能成为运行时总线、替目标系统授权或在一次本地事务中要求多个远端同时成功。

### 下一次证伪条件

- 新 OA 页面要求 Project 直接保存合同条款、资金金额、文件字节或生产阶段内容；
- 删除一个 Project Link 会级联删除来源事实；
- 来源停机后既有 Project／Assignment／Link 历史无法读取；
- 同一引用失败必须人工重建来源事实，无法以原 Command／稳定 ID 继续；
- 新客户必须修改 Base Role 或共享 Project 表才能表达专业分工。

## 4. Production Operations：从设计任务演变为类型化生产执行所有者

### 初始边界

原 `design-v1` 只拥有设计 Work Item、负责人、单成果槽、完成／重开、revision 与历史。

### 当前演变

- canonical audience／API 已从单一 Design 扩为 Production Operations；
- code-owned type definition 已覆盖 `design-v1`、`consultation-v1`，不能由客户任意编辑工作流；
- `reference-oa-stage-set-v1@2` 永久承接项目共享方案，`@3` 增加初步设计，`@4` 增加施工图，`@5` 再增加可选变更与必需竣工，`@6` 增加勘察专有试验和咨询／勘察／测绘共享报告；旧版本不原位改义；
- `000002_production_type_definitions.sql`至 `000007_type_scoped_test_report_definition.sql` 将类型、阶段材料、数据库约束、显式顺序升级和项目类型快照纳入 Production 自有数据库；
- 方案、初设和施工图复用多主文件、可选校审单、可选评分、撤回和历史语义；变更／竣工以同一栏目两套主文件列表接入；试验／报告又以项目类型快照决定适用性。四类阶段差异均没有被压成一个万能槽。

当前能力目录有 13 项；Production 向 Project、People、Evidence 三个 audience 发出 4 项权限，只有 Project 一个 workload 反向读取 Production。

当前候选定义应写成：

> Production Operations 拥有类型化生产任务、阶段规则、成果业务引用、评分／阶段历史及完成语义；Project 拥有项目选择和项目分工，Evidence 拥有 File／Version／字节，People 拥有人员档案，Workbench 只装配。

### 边界变化与风险

Production 的变化不是简单加字段，而是从设计专用应用演变为类型化生产执行产品。主要风险：

1. 阶段定义若变成客户可编辑 DSL、任意字段或通用审批流，Production 会滑向通用工作流平台；
2. 旧 Base Work Item 与 OA 阶段集合并存时必须有明确映射和单一可写真相，不能双写后同步；
3. 已注册的 `reference-oa-stage-set-v1@2`～`@5` 没有原位改变；v3～v6 与显式、可审计、幂等的顺序 2→3→4→5→6 已形成连续兼容演进证据，后续版本仍必须遵守同一原则；
4. Project、People、Evidence 任一故障不能破坏已有 Production 历史；新命令应失败关闭并可按同一命令续办；
5. 页面 Tab 可达不等于阶段已实现，只有真实 Production／Evidence 写入、复读、撤回、授权和恢复才算关闭。

### 下一次证伪条件

- 第三种真实生产类型必须 fork 服务或复制整套表／状态机才能接入；
- 页面层决定阶段完成、评分或 allowed actions；
- Production 保存文件字节、长期 OSS 密钥或人员主档；
- 添加新阶段必须改写旧 definition version 或旧 Work Item 历史；
- 来源故障后已保存阶段资料不可读，或成功回复丢失会重复登记。

## 5. Contract Ledger：在合同领域内部加深，而非向相邻领域扩张

### 初始边界

Contract 拥有合同草稿、双方主体快照、revision、变更、结算与审计历史；不拥有 Project、Party 当前档案、Evidence 字节或 Money 实付。

### 当前演变

- 参考 OA 两条合同链、分项费用、nullable 整数费率进入版本化 Customer Overlay；
- 协议草稿与结算意图不冒充 confirmed Contract／Change／Settlement；
- 修订确认显式映射到 Base Change，用原生历史承接正式事实；
- 三类合同材料只保存稳定 File／Version 与业务含义，Evidence 仍拥有字节；
- `000020_reference_oa_contract_drafts.sql`、`000021_reference_oa_amendment_confirmations.sql`、`000022_reference_oa_materials.sql` 追加 Overlay 事实和命令历史。

当前能力目录有 9 项；Contract 只向 Party、Evidence 发出 2 项权限，Project、Billing、Results、Cost、Bonus 五个 workload 各自精确读取 Contract。

当前候选定义仍可保持：

> Contract Ledger 独占合同承诺、修订、结算和合同材料业务引用；Project 只引用合同，Party 提供当前主体，Evidence 提供文件，Money 只承接已确认义务或实际资金。

### 边界变化与风险

Contract 当前主要是在既有领域内加深，方向比 Project、Production 稳定，但存在四组过渡期双表示：

1. Base Contract Draft 与参考 OA Overlay Draft；
2. 本地 Party 与 Party Directory mapping／runtime transition；
3. 遗留内嵌 Evidence 与独立 Evidence Service；
4. Contract 自有 `project_id` 与 Project 自有 Contract Resource Link。

这些双表示必须通过显式 canonical 状态、migration、导出／恢复和退役条件收敛；不能长期靠 Workbench 猜哪套为准。Overlay snapshot JSON 必须有类型和版本约束，不得成为任意字段垃圾桶。

### 下一次证伪条件

- 合同保存需要同步写 Project、Money 或 Billing 才能成功；
- 协议草稿能绕过显式确认直接成为 Base Change／Settlement；
- 移出合同材料会删除 Evidence 字节；
- 旧导出／恢复静默丢弃 Overlay 字段；
- OA 新合同字段只能塞进无类型 JSON 且无法独立校验、迁移或审计。

## 6. 跨系统依赖账

当前 Workload Policy 中与 Project 有 5 组双向引用：Project ↔ Approval/Audit Results、Billing、Business Engagement、Cost、Production。它们不是分布式事务环，但会形成可用性、延迟和升级耦合，必须逐对验证。

依赖只允许落入以下三类：

| 类别 | 允许用途 | 禁止用途 |
| --- | --- | --- |
| IAM 身份／授权 | 受信 Actor、短期原生 Capability Credential、精确 Scope | 页面自报身份、Gateway 代替目标最终授权 |
| 写入时精确引用校验 | 直接向事实所有者验证稳定 ID、Tenant、Project／资源状态 | 远端调用嵌入本地数据库事务、递归扇出、分布式事务 |
| 只读组合 | Workbench 按项目并列读取来源并显示独立 unavailable／forbidden | Workbench 持久化业务事实、跨来源合计或把故障显示为空记录 |

统一失败规则：远端成功而 Link 失败时保留来源事实并仅续 Link；来源离线时已拥有事实和历史仍可读；需要新引用验证的命令失败关闭；未知回复按同一 Command 重试；reference endpoint 不再递归调用其他系统。

### 6.1 2026-10-12 OMS 一期应收／应付只读投影

| 维度 | 本次变化与证据 |
| --- | --- |
| 用户动作 | 在应收／应付入口按项目查看总额、已收付、余额和往来主体；应付记录按关系建立时间倒序，不在列表展示创建／更新时间 |
| 唯一事实所有者 | Money 继续独占 obligation 金额与余额；Party Directory 继续独占主体当前档案；Project 只拥有 Project Resource Link 及其 `linked_at`；Workbench 零持久化 |
| 契约变化 | Money reference 的 Party 快照新增可选 `role=creditor／debtor／payer／payee`；Project commercial summary 新增可选 Party 快照和 `linked_at` 只读字段 |
| 数据与部署 | 无 migration、无共享表、无新数据库、无新 Audience／Capability、无新的强制依赖边；旧消费者可忽略新增 JSON 字段 |
| 失败语义 | Money／Party runtime 不可用时对应 segment 保持 unavailable；Workbench 显示空态／来源不可用，不按名称或金额猜测，不把缺失值当零 |
| 实际证据 | Money reference、Project reference／service／HTTP 和 Workbench 测试通过；本地浏览器实际打开应收／应付首屏，验证紧凑筛选、五列表格和无演示金额空态 |
| 边界判定 | B 级局部兼容扩展，**不是超级大边界改动**；Contract 自动应收／应付编排、Owner 级分页／筛选和公网 IAM 实数复读仍未完成 |
| 下一证伪 | 若必须把 obligation 金额复制进 Project／Workbench、按名称猜主体、用跨库查询排序或要求 reference endpoint 递归调用 Party，当前方案失败 |

### 6.2 2026-10-12 OMS 一期 Billing 双方向登记装配

| 维度 | 本次变化与证据 |
| --- | --- |
| 用户动作 | 在开票管理选择项目后自动限定甲方，在发票管理选择项目后只允许选择已关联施工方；填写精确金额、业务日期和备注后登记实际票据 |
| 唯一事实所有者 | Billing 独占票据登记、金额、主体／项目快照和纠错历史；Party Directory 独占主体当前档案；Project 独占项目与 `contractor-party` Link；Workbench 零持久化 |
| 装配配置 | `OMS_QUALIFICATION_PARTY_ID` 是 Customer Overlay 的稳定 Party ID，只决定我方方向；它不是 IAM Grant、主体副本或名称映射，格式错误会阻止 Workbench 启动，Demo 预览不注入该值 |
| 命令契约 | Workbench 复用 Billing `POST /api/v1/invoice-registrations`，提交随机 Command ID、整数分、CNY、Project exact revision 和双方 Party exact revision；Owner 继续执行来源复验与最终授权 |
| 数据与依赖 | 无 migration、无共享表、无新数据库、无新 Audience／Capability；只增加显式部署配置和 Workbench 调用现有版本化端口 |
| 失败语义 | 未配置 ID、主体不可用／停用、项目无对应主体、缺少 `record` 能力时按钮禁用；Party／Project／Billing 拒绝时零写入，不回退到名称匹配或第二份票据 |
| 实际证据 | Workbench config／server 测试、JavaScript 语法检查通过；本地浏览器实际确认 preview 中 ID 为空且新增开票明确禁用并显示原因。尚未使用正式 IAM 和真实 Owner 执行成功写入 |
| 边界判定 | Customer Overlay 配置＋既有 Owner 命令，**不是超级大边界改动**；Money 主体 runtime cutover 与 Project＋Contract 多 Owner 创建仍是独立显著缺口 |

### 6.3 2026-10-12 OMS 一期单 Owner 更正与本地导出

| 维度 | 本次变化与证据 |
| --- | --- |
| 用户动作 | 更正 Party 当前档案；编辑 Project 名称、开启日期和说明；更正 Billing 业务日期、金额和备注；按当前筛选导出六类台账 |
| 唯一事实所有者 | Party、Project、Billing 分别处理本域原生命令并追加 revision；Workbench 只持有表单临时状态，CSV 只由已授权读取结果在浏览器即时生成 |
| 契约与数据 | 复用既有 `PUT /api/v1/parties/{id}`、`PUT /api/v1/projects/{id}` 和 Billing correction；无 migration、共享表、跨库读取、新 Capability、Audience 或依赖边 |
| 失败语义 | 缺少能力、记录失效、原字段不完整或 expected revision 冲突时失败关闭；项目编辑不改甲方／金额，票据更正不换项目／双方主体；导出明确不冒充 Owner 全库 |
| 精确与安全 | 项目／Party 原样保留 Owner 隐藏字段；金额由整数最小单位或十进制字符串转换为两位小数，不使用浮点；CSV 文本做公式注入防护 |
| 实际证据 | JavaScript 语法、Gateway／Workbench／Money／Project／Billing 范围测试和 `git diff --check` 通过；本地浏览器实际打开六类中的应收导出窗口并在 390×844 复核，页面 warning／error 为 0 |
| 边界判定 | 既有 Owner 内原生命令＋Workbench 只读／临时装配，**不是超级大边界改动**。Project＋Contract 可恢复新建、Money 主体运行时切换与 Owner 全量导出仍是必须单独设计的显著缺口 |

### 6.4 2026-10-13 OMS 一期六 Owner 公网闭环

| 维度 | 本次变化与证据 |
| --- | --- |
| 用户动作 | 建甲方／施工方、建项目、登记上下游合同、开票／收票、回款／支付、上传并下载六类附件、筛选和导出台账 |
| 唯一事实所有者 | Party、Project、Contract、Billing、Money、Evidence 各自只写本域；Workbench 无业务表，Nginx／Gateway／IAM 只负责入口和身份 |
| 主体兼容 | Contract 4 条、Money 5 条本地主体经显式 mapping 切到 Party Directory；歧义失败关闭，重复执行无变更，历史快照不改写 |
| 依赖边 | OMS runtime 只调用上述六 Owner；People、BE、Production、Approval、Cost、Bonus、Design 没有进入调用集合，即使完整 Suite 同机运行也不是一期依赖 |
| 分页与导出 | cursor／offset helper 读取完整授权集合，205／201 条多页与重复页失败关闭测试通过；公网筛选集合找到本轮 2 Party／1 Project／2 Billing／2 Money；本地筛选 CSV 字段和哈希已核对 |
| 实际证据 | 同一公网 run key 两次得到同一项目／合同／票据／资金／附件 ID；800／268／320／96／220／120 万精确金额复读；六文件短期下载 SHA-256 一致；D1 check 与 17/17 healthy |
| 边界判定 | 六 Owner 的一期纵向闭环已通过，且没有新 OMS 数据库、共享业务表或跨应用直写。无固定期限义务、内部成本和通用审批仍是未来独立范围，不能由本结论外推 |

## 7. 每个 OA 切片的强制记账模板

以后任何触及 Project、Production、Contract 或新增依赖边的 OA 切片，必须在同一变更中回答：

1. 原 OA 的实际动作和可复读结果是什么；
2. 唯一事实所有者是谁，新增长期事实／字段／状态是什么；
3. 属于 Base、版本化类型、Customer Overlay、连接器还是纯 Workbench 装配；
4. 新增或改变了哪些 API、Capability、migration、导出和恢复协议；
5. 入站／出站依赖及其类别是什么，是否形成新的双向引用；
6. 局部成功、超时、重放、撤权、来源停机和旧版本升级如何处理；
7. 哪些证据实际运行，哪些仍是 UI、源码或候选证据；
8. 哪个未来场景会证伪当前边界，达到哪个判定等级。

## 8. 2026-10-06 初步设计切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 复用原初设面板与 800px 抽屉；多主文件、可选多校审、评分 0、取消、刷新、预览、下载、逐份移出 |
| 唯一事实所有者 | Production 新增 definition upgrade／初设 Stage／资料引用／评分／ending／history；Evidence 仍独占字节；Project／People 零写入 |
| 扩展归属 | code-owned `reference-oa-stage-set-v1@3`，不是 Customer Overlay 任意字段、连接器或 Workbench 事实 |
| API／migration | 新增有界升级端点和 `000004_preliminary_stage_definition.sql`；复用既有四项 stage-material Capability，没有新增 Action |
| 依赖变化 | 无新增 Audience、Workload Policy 或双向引用；仍为 Production 写入时向 Evidence 精确校验，Project／People 只提供原引用上下文 |
| 失败与重放 | 升级和资料登记为两个持久子命令；升级成功而上传／登记失败时保留 v3，续原未完成命令，不回滚、不重复升级或字节 |
| 兼容 | v2 永久仅方案；v3 为方案＋初设；旧行不自动迁移，只在第一次真实初设写入时精确 2→3；禁止降级和跨版本跳跃 |
| 实际证据 | Go 全包、169 项 Node、TS／build、静态审计；私有 Nginx→Gateway→正式 IAM→五所有者／PG18 的 9 场景，五库逻辑空目标恢复、四份字节保持、零页面错误 |
| 当前判定 | 阶段资料集合子边界获得第一份“同类场景可重复”证据；Production 整体仍为候选，Project／Contract 等级不变 |
| 下一证伪 | 施工图若要求不同完成规则、结构化成果或文件关系，应新增明确语义；若需要任意流程 DSL、第二套可写真相、跨库事务或 Production 持有字节，则当前边界失败 |

详细规格和运行范围见[初步设计规格](./oa-production-preliminary-stage-v1.md)与[验证报告](../../reports/oa-preliminary-stage-native-validation-20261006.md)。

## 9. 2026-10-06 施工图切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 复用原施工图面板与 800px 抽屉；多主文件、可选多校审、评分 0、取消、刷新、预览、下载、逐份移出 |
| 唯一事实所有者 | Production 新增 v4 upgrade／施工图 Stage／资料引用／评分／ending／history；Evidence 仍独占字节；Project／People 零写入 |
| 扩展归属 | code-owned `reference-oa-stage-set-v1@4`，不是任意 Customer Overlay、连接器或 Workbench 事实 |
| API／migration | 复用既有升级端点和四项 stage-material Capability；新增 `000005_construction_stage_definition.sql` 的 3→4 守卫，没有新增 Action |
| 依赖变化 | 无新增应用、Audience、Workload Policy 或双向引用；仍仅是正式 IAM、Production 写入时向 Evidence 精确校验和 Workbench 装配 |
| 失败与重放 | 升级、Evidence 上传和资料登记保持独立持久子命令；已确认段不回滚、不重做。首次全链因 runtime 未派发 construction 失败，修复并新增专门回归后从新夹具完整重跑 |
| 兼容 | v2 仅方案、v3 方案＋初设、v4 再加施工图；v3 第一次真实施工图写才 3→4，旧 v2 只能顺序 2→3→4；禁止跳级、降级和无审计直接 UPDATE |
| 实际证据 | Go 全包、Workbench 171 项 Node、29 项 runtime、TS／build、静态审计；私有 Nginx→Gateway→正式 IAM→五所有者／PG18 的 10 场景，五库逻辑空目标恢复、五份字节保持、零页面错误 |
| 当前判定 | 阶段资料集合子边界达到“同类场景可重复”；Production 整体仍为候选，Project／Contract 等级不变，跨客户稳定未证明 |
| 下一证伪 | 变更／竣工若不能由明确资料类别、历史和完成映射表达，须新增有界模型；若需要任意流程 DSL、第二套真相、跨库事务或 Production 持有字节，则当前边界失败 |

详细规格和运行范围见[施工图规格](./oa-production-construction-stage-v1.md)与[验证报告](../../reports/oa-construction-stage-native-validation-20261006.md)。

## 10. 2026-10-06 变更／竣工异类切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 同一栏目直接复用 `ChangeFileList`、`CompletionFileList`、`AuxiliaryFilePanel` 和两套 620px 抽屉；多文件、取消、刷新、预览下载、单文件替换和确认移出；无校审／评分 |
| 唯一事实所有者 | Production 新增 v5、change／completion Stage、资料引用、ending 和完成投影；Evidence 仍独占字节；Project／People 零写入；Contract 未参与 |
| 扩展归属 | code-owned `reference-oa-stage-set-v1@5`；change sequence 40 可选，completion sequence 50 必需，不是 Customer Overlay 任意流程或合同变更事实 |
| API／migration | 复用既有升级端点和四项 stage-material Capability；新增 `000006_change_completion_definition.sql` 的 4→5、stage key／sequence 数据库守卫和主文件／扩展名服务端规则，没有新增 Action |
| 依赖变化 | 无新增应用、Audience、Workload Policy、双向引用或跨库事务；IAM 最终授权、Production→Evidence 精确引用和 Workbench 装配不变；Contract 依赖为零 |
| 失败与重放 | 替换拆为“新引用登记＋旧引用结束”两个持久子命令；部分成功只续未完成段。旧引用和 Evidence 字节不覆盖、不删除 |
| 兼容 | v2 方案、v3 增初设、v4 增施工图、v5 增变更／竣工；只允许 2→3→4→5；completion 完成不伪造仍为空的前序必需阶段 |
| 实际发现 | 首轮抓到 migration 未同步 stage key／sequence 约束及代码阶段注册表漏识别 v5 两项真实缺陷；修复并回归后从全新夹具重跑，不拼接局部结果 |
| 实际证据 | Go 全包、Workbench 176 项 Node、29 项 runtime、TS／build；私有 Nginx→Gateway→正式 IAM→五所有者／PG18 的 11 场景，五库逻辑空目标恢复、五份字节保持、零页面错误 |
| 三系统判定 | Production 阶段资料子边界增加首份**异类结构**复用证据，Production 整体仍为候选；Project 无新 schema／能力／Link／写入，Contract 未加入依赖图，二者等级不变 |
| 下一证伪 | 勘察专有试验或咨询／勘察／测绘报告若迫使 Project 存阶段事实、Production 持字节、引入任意流程 DSL、复用合同 Change 表或改写旧 definition，则当前边界失败 |

详细规格和运行范围见[变更／竣工规格](./oa-production-change-completion-v1.md)与[验证报告](../../reports/oa-change-completion-native-validation-20261006.md)。

## 11. 2026-10-06 试验／报告类型适用性切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 直接复用 `SurveyTestStagePanel` 与 `ConsultationReportPanel`；勘察试验及咨询／勘察／测绘报告均为多主文件，无校审／评分，支持保存、刷新、预览、下载和单份移出 |
| 唯一事实所有者 | Project 独占当前项目类型；Production 独占 Work Item 及为解释历史适用性而冻结的最小类型快照、Stage、资料引用、ending 与历史；Evidence 独占字节；Contract 未参与 |
| 扩展归属 | code-owned `reference-oa-stage-set-v1@6`；`test` sequence 60、`report` sequence 70，非适用 Stage 为 `not-applicable / project-type-snapshot`，不是可编辑 DSL 或 Workbench 猜测 |
| API／migration | 复用已有升级端点和四项 stage-material Capability；`000007_type_scoped_test_report_definition.sql` 增加 5→6、不可变快照表、延迟完整性触发器和数据库写入守卫，无新 Action |
| 依赖变化 | **运行时语义变强但未新增边**：Production 在创建 v6／5→6 时使用已有 `project-workspace.read` 精确校验一次，后续读写不同步请求 Project；Evidence 仍为逐 File 精确校验 |
| 边界为何没有漂移 | Production 快照只记录业务发生时必须固定的适用性证据，不提供 Project 类型更改、当前投影或反向同步；这与合同中冻结主体快照的原则一致 |
| 失败与重放 | Project 校验在 Production 本地事务前；失败零 upgrade／快照写。升级、Evidence 上传、资料登记各是持久子命令，已确认段不回滚，未知回复只续原命令 |
| 兼容 | v2～v5 永久不改义；只允许 2→3→4→5→6。Project 后续改型不重算历史，若要重分类必须新增显式 Production 演进命令 |
| 实际证据 | Go 全包、Workbench 178 项 Node、TS／build；私有 Nginx→Gateway→正式 IAM→Project／Production／Evidence／PG18 的 9 场景，五库逻辑空目标恢复、4 份字节保持、零页面错误 |
| 实际发现 | 前两轮的 3 秒慢链超时与“待选文件名即已保存”验收误判被如实保留；专用夹具调为有界 15 秒、记录换证耗时且必须等原组件成功回执，不以业务自动重试掩盖故障 |
| 三系统判定 | Production 内部新增“冻结适用性证据”子边界，不是零变化；Project 当前主档所有权未变，Contract 依赖仍为零。三者整体仍是候选，跨客户稳定未证 |
| 下一证伪 | 完整多类型进度若迫使 Workbench 自建第二套完成真相、Project 反写 Production Stage、每次读取扇出远端，或通过任意 DSL 解决适用性，则当前边界失败 |

详细规格和运行范围见[试验／报告规格](./oa-production-test-report-v1.md)与[验证报告](../../reports/oa-test-report-native-validation-20261006.md)。

## 12. 三个重点系统截至 v6 的边界变化摘要

| 系统 | 从 OA 压力下获得的进化 | 跨系统依赖 | 稳定与未稳定边界 |
| --- | --- | --- | --- |
| Project | 从薄项目壳扩为项目身份、生命周期、分工、上下文、Resource Link 和项目级检查点所有者；仍不存来源业务内容 | 依赖面最宽；本切片仅被 Production 在 v6 边界精确读一次，零写入 | “当前 Project 类型归 Project”稳定；专业目录长期归属、多来源 Link 升级和完整进度消费仍需继续证伪 |
| Production | 从单设计任务进化为 code-owned 类型化执行所有者；v2～v6 已承接同形阶段、异类双列表和类型适用性 | 正式 IAM；写入时 Evidence 精确校验；v6 边界 Project 精确校验；正常读不扇出 Project | 阶段资料子边界已有同类／异类／类型化证据；整个 Production、完整进度、正式 D1 和跨客户升级仍未稳定 |
| Contract | 在合同内部加深为承诺、修订、结算、材料引用及 Overlay 历史所有者，未向生产阶段扩张 | 仅 Party／Evidence 出站；Project／Billing／Results／Cost／Bonus 精确入站读；本切片零依赖 | “法律合同变更不等于 Production change”稳定；Base／Overlay、本地 Party／Directory、遗留 Evidence 的退役闭包仍未完成 |

### 可供下一项目复用的五条规则

1. 可变主档归上游，但解释本域历史的最小、带来源 revision 的不可变快照可归消费域；二者不是双主。
2. 跨系统校验只放在创建、确认或显式升级边界，不把正常页面读取变成递归扇出。
3. 一次 OA 按钮若跨多 Owner，必须拆成有持久身份的子命令；保留已确认段，未知回复续原命令，不用分布式事务或“全重来”。
4. 同名业务词不建立依赖；Production 变更资料、Contract 法律变更及 Project 资源 Link 必须各有 ID／revision，转化只能经显式连接器。
5. “边界稳定”要用第二场景、失败、恢复、兼容升级和第二客户逐步证明；服务没合并、静态审计通过或一页 UI 可见都不够。

## 13. 2026-10-06 四类型总览／进度切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 复用 `ProductionInfo` 的顶部完成类型数、生产状态、阶段资料行、各类型进度和阶段快捷入口；不照搬单体内一次读取扇出多来源及回写 Project 的实现 |
| 唯一事实所有者 | Production 从一个 Work Item、冻结类型快照、Stage 和有效资料引用生成投影；Project 当前类型、Evidence 字节、Contract 事实均未被复制 |
| 扩展归属 | 版本化只读 profile `reference-oa-production-progress-v1`；不是 migration、Customer Overlay、可写缓存或通用报表平台 |
| API／权限 | 新增精确 Project Scope 的 GET；复用 `production-work-item.stage-material.read`，无新 Capability、Audience、Workload Policy 或业务写命令 |
| 依赖变化 | 对外依赖边未增加；普通读取 Project／Evidence／Contract 为 0。Workbench 一次读 Production 并以 30 秒页面缓存抑制重复换证／请求 |
| 四类型语义 | 咨询／设计／勘察／测绘各有一条只读进度线；共享方案／报告只投影、不复制可写 Stage；非适用阶段不显示，必需阶段决定类型完成 |
| 失败语义 | 多套活动阶段真相、遗留冲突、错 Scope／Tenant／Project、快照漂移和未知版本失败关闭；来源失败保留最后成功投影，不冒充 0 或空列表 |
| 实际证据 | Production Go／vet、Workbench 409 项 Node、TS、Vite 3204 modules build 与 15 应用静态边界审计通过；私有真实入口 10／10，单 Owner 读取／缓存／显式刷新、正式 IAM 撤权、真实 PG18、五所有者逻辑空目标恢复、4 份 Evidence 字节保持和零页面错误通过 |
| 三系统判定 | Production 的**公开读边界**新增有业务含义的四类型进度，但写边界不变；Project 零变化且不在读链；Contract 零依赖。三者整体仍为候选 |
| 下一证伪 | 若真实总览必须回查 Project、让 Workbench 再算进度、为共享 Stage 建多套写记录、把 Contract 拉入链路或无法从同一恢复事实重建，则本边界失败 |

详细规格见[四类型生产总览／进度](./oa-production-progress-v1.md)，运行范围见[验证报告](../../reports/oa-production-progress-native-validation-20261006.md)。

## 14. 2026-10-06 批复／审计原生闭环切片记账

| 维度 | 本次变化与证据 |
| --- | --- |
| 原 OA 动作 | 直接复用批复／审计双列、三项金额、多个报告文件、追加修改、撤回、重登记和历史；不再显示样例金额／文件，也不把审计误接成 Contract 当前值 |
| 唯一事实所有者 | Results 独占当前结果、精确金额、来源快照、Evidence 引用、revision 与历史；Contract 独占法律合同事实；Evidence 独占字节；Project 只拥有上下文／Link；Production 不参与 |
| 扩展归属 | Workbench 原界面操作适配＋Results 已有 Base 能力的真实装配；不是新 Customer Overlay、通用表单或 Project／Production 字段 |
| API／migration | Results schema、状态机、Capability、Audience、Workload Policy 和 migration 均未增加；只修正 HTTP 空 Evidence 集合稳定输出 `[]`，并增加页面 adapter、缓存和待续命令协议 |
| 依赖变化 | **没有新增边，但既有边首次被原页面完整使用**：Results 写边界精确读 Project；批复精确读 Contract；文件精确读 Evidence；审计只读 Results 自身 approval revision。普通页面复读、返回缓存和历史均不读 Contract；Production 请求为 0 |
| 失败与重放 | Evidence 上传、Results 命令分别持久；上传成功而登记失败保留 File Version。服务端已提交但回复丢失时同 command 重放；冲突失败关闭；撤回追加 revision，重登记复用 aggregate ID；不回滚已确认段 |
| 兼容与体验 | 页面首次显示骨架；30 秒页面缓存和同键合并；返回 Tab 新增 Results 读取 0，显式刷新恰好 1；缓存不绕过 IAM，迟到回复不能覆盖已切换项目 |
| 实际发现 | 撤回后无附件重登记暴露 Results 把空集合编码为 `null`；严格页面正确拒绝并保留待续意图。Owner 输出修为 `[]` 后，从全新 fixture 完整重跑通过 |
| 实际证据 | Workbench 181 项 Node、TS、3204 modules build；Results／Workbench Go test＋vet；15 应用静态审计；私有 Nginx→Gateway→正式 IAM→六所有者／PG18 11／11，六库逻辑空目标恢复、4 份字节保持、零页面错误 |
| 三系统判定 | Project 零 schema／Capability／业务命令／结果反写，本批未执行 Completion Checkpoint；Production 完全不在依赖图；Contract 只提供精确批复来源且零写。三者整体仍为候选，不因页面完成而升级 |
| 下一证伪 | 若项目完成必须把结果复制进 Project、审计需要每次重读 Contract、Production 必须保存批复事实、无附件再次被当作无效、或第二客户需要 fork Results／共享表，则当前边界失败 |

详细规格见[批复／审计原生闭环](./oa-approval-audit-native-v1.md)，运行证据见[验证报告](../../reports/oa-approval-audit-native-validation-20261006.md)。

## 15. 2026-10-06 对外委托切片边界与运行证据

| 维度 | 本次设计与待验事实 |
| --- | --- |
| 原 OA 动作 | 复用三项统计、列表、880px 抽屉、受托方／联系人／内容／金额、合同文件、评分和支付显示；拒绝编辑时删除重建全部旧支付及按供应商名称兜底关联 |
| 唯一事实所有者 | Cost 拥有委托、发生时联系人快照、评分和应付建议；Party 拥有当前受托方；Contract 拥有法律合同；Evidence 拥有字节；Money 拥有义务、实付和核销 |
| Cost 边界变化 | Engagement 增加可选 `contact_name`／`contact_phone`，属于本次业务发生快照并随 revision 保留；不是 Party 当前联系人，也不反向同步 |
| Project 变化 | 无 schema／Capability／业务字段；仅在新建 Cost 事实边界按既有端口精确校验，零反写 |
| Production 变化 | 零 schema、零 API、零 Capability、零读写；页面归在生产信息不改变事实所有权 |
| Contract 变化 | 零写；只有用户显式选择正式合同时精确读并冻结 revision。仅上传合同附件不冒充法律合同 |
| 支付拆分 | Cost 先提出 Payable Suggestion，Money 独立接受／拒绝；suggestion、义务和实际现金不可互相冒充，编辑委托不删除 Money 历史 |
| 失败与重放 | Evidence 上传、Engagement、Assessment、Suggestion、Money sync 为独立持久子命令；保留已确认段，同 command 续办，冲突失败关闭 |
| 体验约束 | 原结构骨架、30 秒页面缓存、同键合并；普通列表不回查 Project／Party／Contract／Evidence，预览下载按点击读取 Evidence |
| 当前证据 | Cost／Workbench 定向测试与构建通过；私有 Nginx→Gateway→正式 IAM→八事实 Owner／PG18 的 10／10 通过，含未知回执重放、精确工作负载 Scope、返回缓存零读／显式刷新一读、八库逻辑空目标恢复、IAM 撤权和零页面错误 |
| 下一证伪 | 若联系人必须成为跨委托共享主档，应另评 Party 联系人能力；若实付必须写入 Cost／Workbench、普通读取递归扇出或 Production 接管委托，则边界失败 |

详细规格见[原 OA 对外委托原生闭环 v1](./oa-external-commission-native-v1.md)，运行范围见[验证报告](../../reports/oa-external-commission-native-validation-20261006.md)。

## 16. 当前下一验证

对外委托主路径已完成私有真实入口 10／10、精确工作负载 Scope、请求上界、正式 IAM 撤权、PG18 和八事实 Owner 逻辑空目标恢复，并登记 BF-118 的首轮运行证据。它验证 Cost 的发生时联系人快照和五 Owner 拆分，但没有关闭 Money 拒绝／实付、全角色／跨 Tenant、依赖逐项停机、Main／公网、正式 D1、第二客户或完整 OA。下一切片进入资料管理／统计：先按 F01／F02 接通项目资料展开、下载和下载人员记录，再按 S01～S03 接通台账筛选／导出、公司统计和人员奖金统计；任何聚合只读来源 Owner，Project 只保留 Link／上下文，Workbench 不拥有第二份事实。

## 17. 截至 BF-118 的三系统边界演变总结

### Project Workspace

Project 是三者中跨系统依赖面最大的系统，但本轮没有继续吞入业务字段。它从最初项目壳演进为项目身份、生命周期、分工、上下文、Resource Link 和检查点 Owner；对于 Production v6、Results 和 Cost，只在创建／确认边界提供精确 Project 身份、revision 与状态。正常 Production 读、Results 历史和 Cost 委托复读都不递归回查 Project，也没有任何来源把金额、文件或阶段事实反写 Project。

当前稳定部分是“项目当前上下文归 Project、外部事实只通过 Link／精确引用装配”；仍不稳定的是资料目录首屏跨来源计数、专业目录长期归属、Project Completion Checkpoint 和多来源 Link 的升级／退役。下一 F01／F02 切片必须优先证伪 Project 是否会因全站资料索引而膨胀成总线或共享文件库。

### Production Operations

Production 的本体变化最大：从单设计任务演进为 code-owned v2～v6 类型化生产执行，覆盖同形阶段、异类变更／竣工、项目类型适用性快照和四类型进度投影。演进一直通过不可变 definition、顺序升级、来源 File／Version 和追加历史完成，没有使用任意流程 DSL。批复／审计和对外委托虽然位于原 OA“生产信息”页面，但运行时均保持 Production 零依赖，证明 UI 信息架构不能决定事实所有权。

当前稳定部分是阶段资料子边界、发生时类型适用性快照和 Production 自有进度投影；仍不稳定的是跨客户阶段扩展、类型重判、正式 D1／keys 恢复以及资料管理对 Production material-reference 的批量消费。F01／F02 只能读取公开材料投影，不得调用写命令或把下载审计写回 Production。

### Contract Ledger

Contract 主要在领域内部加深：合同承诺、修订、结算、材料引用和参考 OA Overlay 历史。Results 批复精确读取 Contract／Settlement，Cost 外委只在用户显式选择正式合同时精确读取 Contract；两者均冻结来源 revision、普通复读零 Contract 请求、Contract 零写。Production change 与 Contract Change 继续明确是两个不同领域词。

当前稳定部分是“法律合同归 Contract、消费方冻结精确来源、附件归 Evidence”；仍不稳定的是 Base／Overlay 双表示、本地 Party→Directory、遗留 Evidence 和 Project Link 的退役闭包。资料管理只能读取 Contract material-reference，不得以下载或目录排序改变合同 revision。

### 跨系统依赖的可复用判断

1. 依赖边是否可接受，不只看“没有共享表”，还要看调用时机；创建／确认边界的精确校验可接受，普通页面复读的递归扇出通常不可接受。
2. 页面归类、导航和视觉组件属于 Workbench 装配，不授予事实所有权；批复和外委均验证了这一点。
3. 消费方可拥有解释自身历史所需的最小不可变快照，但不能拥有上游当前主档；Production 类型快照、Results 来源快照和 Cost 联系人／Party／Contract 快照遵循同一规则。
4. 一个按钮跨多个 Owner 时，正确演进是有身份的子命令、局部成功保留和同 command 续办，不是跨库事务或失败后删除重建。
5. 版本化只读投影是解耦工具，不是复制真相的许可；必须携带 source、resource ID／revision、digest 和 unavailable／forbidden 语义，并可从 Owner 重建。
6. 边界成熟度仍是：Production 阶段资料子边界有同类／异类重复证据；Project、Production 整体和 Contract 仍只到候选。没有第二客户和长期升级证据，不能提升为跨客户稳定。

下一设计门禁见[项目资料管理 F01／F02](./oa-file-management-native-v1.md)。

## 18. 2026-10-11 OMS 一期能力映射与边界影响（BF-145）

| 维度 | 本次判定与约束 |
| --- | --- |
| 产品动作 | 四个一级入口承接项目、甲方／施工方、约定应收／应付、开票／收票、实际回款／支付、总览和导出；工程进度、审批、内部成本、固定期限与催收不进入一期 |
| 唯一事实所有者 | Project 拥有项目上下文；Party 拥有当前主体；Contract 拥有上下游承诺及合同总额；Billing 拥有发票；Money 拥有实际资金；Evidence 拥有字节；IAM 拥有身份授权；Workbench 只装配 |
| 直接复用 | IAM 登录／换证、Nginx／Gateway 入口、Project 基本资料、Party 最小档案、Contract 承诺、Billing 发票、Money Cash、Evidence File／Version 和精确金额基础均可复用 |
| 有界扩展 | 新增 `/oms-v1/` 与 Workbench 页面；Money 增加 JSON 列表／筛选投影；Project／Contract 增加稳定方向／关系投影；Party 可在自身边界兼容增加可选档案字段；各 Owner 补 OMS 导出视图 |
| 显著边界事项 | Contract、Money 的本地 Party 到 Party Directory 正式 runtime cutover 尚未关闭；Money obligation 固定 `due_on` 无法表达本产品的无固定期限。二者必须单独评审、迁移和恢复验收，不得随页面隐式改写 |
| 禁止方案 | OMS／Workbench 自建项目、主体、应收、应付、发票或资金表；按名称拼主体；跨库 SQL；三方双写；用假到期日复用逾期模型；把 Contract 金额复制到 Project 获得伪原子保存 |
| 跨 Owner 失败 | 项目、合同、Link、Evidence 等使用稳定子命令；前段成功后保留事实并续办后段。未知回复重放同 command；不使用跨库事务或删除重建 |
| 当前证据 | 已完成产品基线和源码级能力／缺口映射；未运行 OMS v1 公网、正式 IAM、真实写入、故障恢复和空目标恢复，不得描述为闭环已通过 |
| 边界影响结论 | **不是全盘大改**：大部分能力可由既有 Owner 组合和局部兼容扩展完成；主体 runtime cutover 和可选期限义务是两个被明确隔离的显著变更，当前不隐式实施 |
| 下一证伪 | 如果真实项目必须让 Workbench 持久化金额、一次保存必须跨库原子提交、主体只能靠名称匹配、无期限义务只能改写旧账龄含义，或普通总览读取无限扇出，则当前组合边界失败 |

详细映射、实施顺序和验收门禁见[OMS 一期小系统能力映射与缺口矩阵 v1](./oms-phase1-small-system-capability-gap-v1.md)。
