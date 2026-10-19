# OA 完全体（全业务范围）重组验证

## 元信息

- 路径：`spec/assembly/full-reference-oa-validation.md`
- 所属模块：`assembly`
- 状态：⚪ 计划中；业务关系与当前缺口已完成证据审计，全业务运行尚未实现
- 优先级：P0
- 依赖能力：IAM、People & Groups、Party Directory、Project Workspace、Contract Ledger、Money Flow Ledger、Evidence Service、Design Operations 及本规格列出的新增责任域
- 回归基线：[参考 OA 高保真业务架构 Demo](./reference-oa-demo.md)

## 功能目标

OA 完全体重组验证证明：现有定制化 OA 中已经确认的项目经营与生产全业务，可以由边界清晰的小应用和类型化扩展共同完成；用户得到连续的项目工作体验，各业务事实仍由自己的应用拥有、纠错和追溯。

“完整”指覆盖参考 OA 已确认的业务生命周期和用户结果，不指复制旧页面、旧表、旧接口、遗留兼容结构或仍未确认的菜单名称。任何尚未取得责任、状态、纠错和验收证据的能力保持未确认，不以占位页面冒充完成。

## 阶段定位与交付

该验证是当前下一阶段产品与架构主线。它同时检验完整 OA 业务覆盖、最小应用组合、显式依赖、客户定制归属和真实环境可交付性；其中任何一项都不能由其他项代替。

阶段最终交付物是架构可行性验证报告。报告对 C1～C8、应用独立性、跨应用契约、Base／Customer Overlay 变化、故障恢复、人类可用体验和真实环境逐项引用直接证据，并使用以下结论之一：

- `supported`：全部核心门槛和至少一套真实 Ubuntu 环境均通过；
- `conditionally-supported`：核心方案成立，但存在明确、有界且不推翻主要结论的条件；
- `not-supported`：出现共享事实、客户核心分支、不可恢复数据或其他架构反证；
- `insufficient-evidence`：关键业务旅程或真实环境证据不足。

下一轮参考应用不属于本规格的预设输入。报告只保留选择条件和待证伪假设，不提前指定 OMS、某个真实客户或某类应用为下一轮目标。

## 来源依据

- 参考 OA 当前前后端路由、领域模型和服务行为；
- `OA_2026_REQUIREMENTS_CONFIRMED.md` 的确认需求；
- 项目、商务、生产、财务、外委、奖金和资料现有规格；
- BizFoundry 已通过的六应用 Level A 与 J1～J4 Level B 验收；
- 已接受的数据所有权、IAM、Evidence、Suite Manifest 和客户差异分层决策。

来源发生冲突时，当前运行行为用于证明“已经存在什么”，较新的确认需求用于确定“目标行为是什么”。历史规格只提供候选线索，不能单独形成实现承诺。

## 业务主关系

参考 OA 的业务主线由同一个 Project 上下文组织，但 Project 不拥有全部业务事实：

1. 租户账号和能力授权决定操作者是谁、能够处理哪些业务；
2. 人员、Group、主体和项目 Assignment 建立参与者与业务对象；
3. 立项和项目包建立工作上下文、责任人及适用业务类型；
4. 承接方式、投标材料和经营责任形成项目前置业务事实；
5. 合同、补充、结算及双向业务链形成法律与经营依据；
6. 应收应付、开票、实际收付和核销形成资金结果；
7. 设计、咨询、勘察和测绘按各自阶段形成生产成果；
8. 批复审计、生产成本和外委形成生产侧结果与义务来源；
9. 经营奖金和生产奖金根据已确认来源事实形成可追溯计算与分配；
10. 资料、统计、待办和提醒只读聚合各事实所有者的正式结果。

## 能力簇与所有权

| 能力簇 | 用户结果 | 事实所有者或目标责任域 | 当前覆盖 |
| --- | --- | --- | --- |
| 身份、账号与授权 | 单点登录并按站点、项目和业务能力访问 | IAM；目标应用最终授权 | 入口、Project 资源／Assignment 策略、服务身份、C1／C2 权限矩阵及 C3 经营金额读取、撤权、账号停用与 IAM 故障矩阵已实现；C4 已通过正式入口、Grant 撤销／恢复、账号停用／启用、Assignment 结束、`allowed_actions` 收敛和真实浏览器恢复；C5 已通过目标 IAM、Project／Result Scope、金额／Evidence 独立能力、错误凭证矩阵和 `correct` Grant 撤销／恢复；C6 已通过目标 IAM、Project／精确资源 Scope、金额／Evidence 独立能力、错误凭证矩阵和正式入口组合。C5／C6 的账号停用、完整双 Tenant、真实浏览器、C7～C8、真实 Ubuntu／公网环境仍待验证 |
| 人员与编组 | 管理人员、组、任职和项目参与者 | People & Groups；Project 拥有 Assignment | 基础已实现 |
| 业务主体 | 管理甲方、资质单位、合作方并在历史中冻结当时信息 | Party Directory；消费应用拥有角色与快照 | 独立核心、release、恢复、Workbench 管理页及首个 Project 消费边界已通过；D1 API 入口、用户候选发现、服务端精确 IAM 复验、依赖故障恢复与联合空目标还原已验证；Contract／Money 已具备来源自有映射和含引用数据的显式切换／只读回滚，Contract-linked C6 正式入口与空目标恢复已通过；Money 完整 D1 消费链和完整双 Tenant 待补 |
| 项目立项与项目包 | 管理项目身份、负责人、客户选择、父子结构和转换 | Project Workspace | 单项目、两级项目包、既有项目转换／安全撤销、客户页面资源和真实 D1 入口已实现；客户 Party 精确引用、可选选择器与最小快照已进入 D1，合同金额分配待验证 |
| 经营承接与投标 | 管理承接方式、适用性、材料和经营进度 | 独立 Business Engagement；Project 只保留引用／投影 | 后端闭环、release／Suite Manifest、精确工作负载凭证、Workbench 页面、浏览器撤权刷新、正式入口权限矩阵、本地空目标恢复／继续写入及 409／503 自动化恢复已验；真实浏览器最终点击、下游建议、真实 Ubuntu 和完整故障矩阵待验 |
| 合同与经营链 | 管理合同、补充、终止、结算和两段业务关系 | Contract Ledger；链路上下文使用稳定引用 | 合同内核已实现；双段链路与项目分配待扩展 |
| 开票与票据 | 登记已开票事实、Evidence 引用、资料纠正与登记无效；现实票据反向生命周期保持未知 | 独立 Billing／Invoice 责任域 | 第一版边界、模型、API、PostgreSQL、IAM、D1、恢复和人类工作区已完成；Invoice Registration 已与 Contract、Money 一同进入 C3 Project 只读组合并通过角色／撤权／停机矩阵，不由 Money Flow 冒充；现实红冲／作废／退票仍未知 |
| 义务与实际资金 | 管理应收应付、实际收付、核销、余额和纠错 | Money Flow Ledger | 内核已实现；来源连接仍需逐项确认 |
| 多类型生产 | 分别推进设计、咨询、勘察和测绘并解释完成条件 | 单一 Production Operations 数据所有者＋版本化 Type Definition | `design-v1` 与 `consultation-v1` 已通过代码、真库、权限、最小组合、D1 正式入口、未选缺席、恢复和真实浏览器权限生命周期验证；survey／mapping 仍缺业务证据，完整双 Tenant 与真实 Ubuntu 待补证 |
| 批复与审计 | 记录外部批复审计结果、金额、文件和覆盖原因 | 独立 Approval & Audit Results；Project 只读评估完成条件 | 责任边界、C5-A01～A15、领域核心、独立真库、受保护入口、精确引用、Results 人类工作区、Project Completion 检查点、正式 D1 装配／裁剪／联合恢复与 74 断言真实跨进程业务旅程已确认；完整双 Tenant、真实浏览器与未知业务规则待补 |
| 生产成本与外委 | 管理成本发生、外协约定、评分、付款建议和票据引用 | Cost & Outsourcing；Contract、Money、Billing、Party、Evidence 保存各自正式事实 | C6 独立双聚合核心／PostgreSQL 18、受保护 HTTP／正式 IAM、金额／Evidence 分权、Project／Party／Contract／Evidence 精确 adapter、人类工作区、独立进程、正式 D1、选装缺席和联合空目标恢复已验证；Contract Party 切换／回滚后的外协引用已通过，付款建议和真实 Ubuntu 待实现 |
| 奖金 | 形成经营／生产奖金池、版本化计算和人员分配 | 独立 Bonus Distribution；Money 独立确认应付建议 | C7 独立领域／PostgreSQL、正式 IAM、真实来源、人类工作区、正式 D1、未选缺席、联合空目标恢复与继续写入已验证；C7→Money、经营／生产单类型最小组合、双 Tenant 与真实 Ubuntu 待验证 |
| 文件与证据 | 上传、授权访问、版本追溯和技术审计 | Evidence Service；业务含义归引用应用 | 基础已实现 |
| 项目资料与统计 | 按项目、公司、人员查询正式结果 | 只读投影／报表层 | 局部引用已实现，完整投影缺失 |
| 待办与提醒 | 根据本人责任和正式状态提示未完成事项 | 各事实域产生工作项，Workbench 聚合 | 立项待办语义可定义，回款提醒未确认 |

## 必需业务旅程

### C1：项目与责任建立

用户建立普通项目或项目包，补全基础资料、业务类型、经营负责人、生产负责人和参与人员。负责人变化、父子转换、取消和恢复具有显式状态与历史；Project 不复制人员当前档案。

### C2：经营承接与合同链

用户选择比选、竞争性谈判、招投标或不适用，提交相应材料并形成可解释的承接阶段状态；Business Engagement 独立拥有该阶段的适用性、方式、材料业务引用、revision 和历史。当前直接证据只支持“材料条件满足”的 `ready`，不把它冒充中标或签约。合同 revision、补充、终止和结算由 Contract Ledger 独立处理；未中标／流标、多轮承接和自动创建合同在业务证据确认前保持未知。

### C3：开票、应收与回款

用户依据合同、结算或人工确认形成应收，记录我方开票、甲方付款、资质单位向我方付款及核销结果。开票事实与现金事实分开；错误票据、错误收款和错误核销由各自所有者纠正。

第一版 Billing / Invoice 只管理“系统对已开具票据的登记”：日期、精确金额、币种、开票／受票 Party 快照、来源引用、Evidence 引用、revision 和历史。资料纠正追加 revision；误录或重复登记进入 `registration_invalidated`，但该状态不表示现实票据已红冲、作废或退票。Contract／人工确认提供应收来源，Money Flow 独立确认义务、现金和核销，组合页面只读聚合并下钻。详细边界见 [已开票登记](../billing-invoice/invoice-registration.md)。

### C4：多类型生产交付

同一项目可以选择一个或多个生产类型，每个类型拥有自己的阶段、负责人、适用性、必需成果和完成条件。留空不等于不适用；恢复适用、替换成果和阶段返工保留历史。

C4 采用一个 Production Operations 数据所有者，不按类型复制应用，也不把阶段规则迁入 Project。现有 Design Operations 是 `design-v1` 兼容基线，第一增量 `consultation-v1` 固定为方案（可不适用）和报告（必需）两阶段；同一 Project 的不同类型必须各有 Work Item、frontier、revision 和历史。类型定义随代码／契约发布，不允许用户编辑任意工作流。产品细则见[多生产类型交付](../production-operations/multi-type-production.md)，当前直接证据见[C4 边界审计](../../../docs/reports/oa-c4-multi-production-boundary-audit-20260927.md)。

### C5：批复、审计与项目完成

用户记录批复或审计金额、对应文件和人工覆盖原因。项目完成状态从经营与生产正式事实派生，并能下钻到未满足条件；Project 不直接修改来源状态。

C5 采用独立 Approval & Audit Results 应用，而不扩大 Production、Project、Contract 或 Money 内核。批复默认冻结精确 Contract／Settlement revision，审计默认冻结精确批复 revision；手工金额或与来源不同的金额必须记录覆盖原因。所有金额使用整数最小货币单位，报告只保存 Evidence File ID／Version 业务引用，纠正与撤回追加 revision 而不物理删除。

Project 使用版本化 Completion Profile 逐来源返回 `satisfied`、`not_applicable`、`pending` 或 `unavailable` 及下钻原因；读取完成视图不得产生写入，最终状态变化必须是带 expected revision 和来源 revision 集合的显式命令。当前 `reference-oa-c5-v1` 已验证 Results 精确来源、GET 零写入、显式确认、幂等回放、stale／依赖失败关闭和来源变化 drift；确认只形成 Project 自有检查点且 Project 保持 active。隔离 D1 又以真实 Contract／Evidence／Results／Project 独立进程和 74 个 HTTP 断言验证依赖故障零写入、权限撤销／恢复、确认后纠正和 Results 停机语义。C6／C7 尚未确认时不得把 C5 结果冒充完整 Project 完成。产品细则见[批复审计结果登记](../approval-audit-results/result-registration.md)，运行证据见[C5 真实独立进程业务旅程实验](../../../docs/reports/oa-c5-real-process-journey-experiment-20260928.md)。

### C6：成本与外委

用户记录交通、住宿等生产成本，建立个人或公司的外委事项、合同、评分、付款建议和票据引用。来源域说明为什么发生，Money Flow 只确认和处理正式义务与实际付款。

C6 接受一个独立 **Cost & Outsourcing** 应用，但不接受参考 OA 的宽 `FinancialRecord`。应用内部用两个严格聚合表达不同不变量：Cost Entry 记录成本类型、发生原因、精确整数金额、发生日期、Project／Evidence 快照和追加纠错；Outsourcing Engagement 记录服务范围、Provider Party 快照、外协约定／Contract 引用、评分、Evidence 与付款建议。二者各有稳定 ID、revision、历史和幂等命令，不能互相冒充。

Contract 继续拥有法律约定，Money Flow 继续拥有义务、支付、核销和余额，Billing 继续拥有票据登记，Party Directory 继续拥有主体当前档案，Evidence Service 继续拥有文件字节与版本。Cost & Outsourcing 只通过版本化端口读取这些来源，并向 Money 提出带精确来源 revision／digest 的 payable suggestion；Money 必须独立确认，C6 不直接创建付款、发票或跨库删除记录。当前已完成责任边界、C6-A01～A18 定义、领域核心、受保护 HTTP／IAM、人类工作区、正式 D1、Party `cutover → rollback → recutover` 与联合恢复。C6→Money 又以两个真实进程／数据库通过接受、拒绝、目标停机／恢复、幂等、历史防篡改和部分失败恢复：接受只创建一个 Money 自有 Obligation，拒绝创建零个。Money、Billing、建议 connector 和 C7 仍不是 Cost／Engagement 登记的隐式前提。connector 完整正式 D1、跨进程撤回、Money／Billing 只读投影和 Project 完成规则仍未验证，Money 完整 D1 Party 消费链、完整双 Tenant 与真实 Ubuntu 仍待补证。详见[成本与外委生命周期](../cost-outsourcing/cost-outsourcing-lifecycle.md)、[C6 责任边界审计](../../../docs/reports/oa-c6-cost-outsourcing-boundary-audit-20260928.md)、[C6→Money 应付建议实验](../../../docs/reports/oa-c6-payable-suggestion-experiment-20260928.md)和[Party 运行时切换实验](../../../docs/reports/oa-party-ledger-runtime-cutover-experiment-20260928.md)。

### C7：奖金计算与分配

经营奖金使用 Contract 的已确认客户结算，生产奖金使用 Money 的已确认公司收款。Bonus Distribution 独立拥有奖池、计算策略快照、输入快照、人员分配、发布／取代／撤回和不可变历史；People 继续拥有人员当前档案，Project 继续拥有 Assignment，Money 继续拥有义务、支付、核销和余额。

第一版把经营与生产奖金作为同一严格生命周期中的两种类型，而不是复制两个应用。两类奖池使用不同来源端口和 Capability；候选人员分别来自经营或生产 Assignment。每次计算必须冻结来源 ID／revision／digest、Person 与 Assignment revision、策略版本、精确整数金额、比例和舍入结果。参考 OA 的 `base * ratio / 1.065` 只登记为 `reference-oa-bonus-v1` Customer Overlay，不进入 Base 金额内核。

奖池在草稿阶段可以重新计算；发布后只能由新的 revision 取代或显式撤回，不能原地修改或物理删除。发布分配可以向 Money 提交版本化 Payable Suggestion，但 Money 必须独立接受或拒绝，C7 不直接创建工资、奖金付款或核销。当前已完成独立领域／PostgreSQL、受保护 HTTP／IAM、真实 People／Money 来源、人类工作区和正式 D1；16 条正式入口断言、5 条精确 workload audit、联合备份、空目标恢复、恢复后继续写入以及未选／重新选装已通过。状态为 `formal-d1-assembly-and-recovery-supported / payable-suggestion-and-single-type-minimum-pending`；C7→Money、单类型最小组合、双 Tenant 和真实 Ubuntu 仍未通过。详见[奖金生命周期](../bonus-distribution/bonus-lifecycle.md)与[C7 D1 正式装配和恢复实验](../../../docs/reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)。

### C8：资料、统计与个人工作

用户从项目入口查找有权访问的合同、票据和生产成果，并按项目、公司、人员查看统计。首页工作项由各领域事实所有者发布，Workbench 只按当前 Subject 聚合导航和来源状态；报表读取版本化投影，不跨库联表或复制可写业务事实。

第一版边界已固定为：Project Workspace 按 active Resource Link 无状态组合 Project 范围资料与状态；Evidence 最终拥有 File Version、摘要和短期下载授权；各领域应用拥有统计口径和工作项状态；公司／人员统计先按来源分区，不在没有版本化公式时跨来源合计。资料搜索必须带 Project Scope，metadata、download 和 export 分权；`empty`、`unavailable`、`forbidden`、`not_selected` 与 `revision_changed` 不得混淆，无金额能力时返回 null／redacted 而不是 0。

当前已完成责任边界、Evidence 分权、direct Project→Evidence 真实最小运行链，以及资料页面、Business Engagement 间接资料、Production 个人工作来源和 Money 首个类型化指标来源的零容器实现／契约验证。真实资料点击、间接资料运行链、个人工作正式入口、指标消费者、完整 D1／恢复、双 Tenant 与真实 Ubuntu 仍待验。完整规格见 [C8 项目资料、统计与个人工作](./c8-material-statistics-personal-work.md)，证据见 [C8 边界审计](../../../docs/reports/oa-c8-material-statistics-personal-work-boundary-audit-20260929.md)与[C8 Money 指标来源单元验证](../../../docs/reports/oa-c8-money-metric-projection-unit-validation-20260929.md)。

## 完整性规则

- 页面连续性不改变事实所有权；统一工作台只聚合导航、状态和允许动作。
- 每个金额来源必须能追溯到责任域、来源 revision、确认人和纠错方式。
- 每个生产完成状态必须能追溯到类型、阶段、适用性和成果版本。
- 每个跨应用动作都具有版本化契约、幂等身份、失败恢复和目标确认。
- 客户自由文本和自动补全可以替代完整 Party 管理界面，但不能建立第二套可变主体目录。
- 客户覆盖层只增加装配、策略、类型化扩展、连接器、投影和专项包，不覆盖公共内核的数据所有权、历史规则或鉴权契约。
- 一个来源应用不可用时，已有快照和历史继续可解释；需要新鲜事实的写入失败关闭。

## 权限与身份治理验收

OA 完全体把权限治理作为贯穿 C1～C8 的业务能力验收，不把“能够登录”或“全能力管理员可以完成旅程”视为权限通过。每个旅程同时验证身份、租户、目标应用能力、资源范围和目标应用当前状态；任一条件不成立时均拒绝业务动作且不产生部分写入。

权限协作保持以下职责：

- Nginx 只承担公网 HTTPS 边缘、请求边界和到 Gateway 的转发，不解释用户、角色或业务权限；
- Gateway 清除客户端身份 Header，校验入口凭证和路由边界，不拥有账号、角色、策略或最终授权；
- IAM 拥有 Subject、Tenant、账号状态、角色到 Capability 的授予和短期凭证签发；
- Project、People & Groups 等事实所有者只提供带来源、revision 和有效时点的 Business Fact Snapshot，不签发目标应用权限；
- 受信 IAM 授权模块或 Policy Provider 把 IAM 授予、目标应用 Capability Manifest 和业务事实收敛为目标应用原生的 Capability Credential；
- 目标应用复验凭证，并结合租户、资源归属、当前状态、revision 和领域不变量返回并执行 `allowed_actions`，是最终授权者。

参考 OA 的职位和项目角色是 Customer Overlay／策略层的验收样本，不进入目标应用内核。至少覆盖以下差异：

| 参考身份 | 预期范围 | 必须证明的边界 |
| --- | --- | --- |
| 身份超级管理员 | 身份、账号和权限治理 | 不因身份管理权自动获得项目、财务、文件或奖金业务权 |
| OA 业务管理员 | 显式授予的站点业务管理范围 | 不能用万能管理员旁路替代 Capability、资源范围和目标状态判断 |
| 总经理 | 已确认的全局只读范围 | 能看不等于能改；金额与敏感字段仍按明确能力显示 |
| 经营经理／生产经理 | 对应业务域全局查看和负责人配置 | 经理职位不自动获得所有项目执行写权限 |
| 项目经营负责人／生产负责人 | 指定项目和对应业务域 | 同一 Subject 对未分配项目默认拒绝，结束 Assignment 后新授权失败 |
| 经营人员／生产人员 | 指定项目的日常业务动作 | 人员组织、负责人指定和奖金分配等高风险动作保持拒绝 |
| 财务／办公室角色 | 明确财务或办公动作 | 不因能维护金额或办公事实而获得经营、生产或项目治理写权限 |

参考 OA C1 的第一份版本化 Overlay 已把 `identity_super_admin`、`oa_business_admin`、`general_manager`、`business_manager`、`production_manager` 和 `finance` 映射到 Project 自己发布的稳定 Capability。`identity_super_admin` 显式映射为空；经营／生产经理分别只获得对应负责人 Assignment 管理能力。项目负责人和参与者不靠站点岗位获得 Project A，而是通过 Project Assignment Business Fact 和 `project-assignment-access@1.0.0` 取得精确 Project Scope。客户改岗位名或重新组合动作时，允许修改 Overlay 版本，禁止修改 Project 的角色判断、数据库或历史语义。

每条 C1～C8 旅程至少具有以下权限证据：

1. 一个符合预期的允许样例和至少一个相邻角色拒绝样例；
2. 同一角色对已授权资源允许、对另一项目或另一租户资源拒绝；
3. 前端入口、字段、金额、文件动作和后端命令使用同一稳定业务动作语义，直接构造请求不能绕过服务端；
4. 授权撤销、账号停用、Assignment 结束、凭证过期或策略 revision 变化后的明确行为；
5. 错误 issuer、tenant、audience、action、scope、签名、有效期和客户端伪造身份 Header 的失败关闭证据；
6. IAM／Policy 暂时不可用时，未过期凭证只在既定 TTL 和范围内继续，高风险新授权失败关闭；
7. 审计能够追溯 Subject、Tenant、目标应用、动作、资源、凭证或决策标识及使用的策略／事实 revision，不记录密码、Session、JWT 或 Cookie 原文；
8. 客户职位改名或重新组合权限时，只改变 Overlay／IAM／策略配置，不修改目标应用 Capability、状态机或数据库。

页面权限快照和 `allowed_actions` 只改善体验。旧页面在撤权后仍显示按钮时，服务端必须拒绝，前端收到 `403` 后退出编辑／提交状态、提示权限变化并刷新权限快照。金额无权查看时不得用 `0` 冒充业务事实；文件查看、下载、上传和删除分别授权。

以下任一现象使权限架构验收失败：客户岗位名、菜单或按钮 ID 进入业务应用判权；Gateway 代替目标应用最终授权；Project／People 直接签发目标能力；业务应用实时读取其他应用内部表；全能力管理员成为唯一业务验收身份；项目范围、字段级动作、撤权或拒绝路径没有直接证据。

### 当前 C1 权限证据边界

2026-09-26 的隔离 D1 最初以两个 Project、九类 Subject 和 116 个 HTTP 断言通过 C1 项目责任子矩阵；加入 Project Package Base 后为 134 个；2026-09-27 加入客户页面资源、状态收敛 `allowed_actions`、权限刷新和撤权恢复后，同一正式入口扩展为 152 个断言：身份超级管理员无隐式业务权，业务／生产经理不能互相管理负责人，项目双负责人和参与者只作用于 Project A，总经理与财务只读，总经理不能治理层级，只有显式 OA 业务管理员可创建／转换／撤销 Package；伪造 Header、跨 Project、错误 Fact／Scope、旧双 revision、第三层、`active` 状态写入、不安全撤销和 Assignment 结束后的新签发均失败关闭，且拒绝不产生部分写入。撤销结构能力后动作投影收敛，撤销全部入口能力后 Gateway 清除旧应用 Cookie。最长本地请求时间为 0.263164 秒；该数字只描述本机样本，不作容量承诺。

这份证据支持“复杂 OA 权限可以沿 Nginx、Gateway、IAM／Policy 和目标应用边界工作”，但只覆盖 C1 已实现的项目责任、Project Package Base 和页面权限变化。项目包／父子转换／安全恢复已经穿过正式入口，但仍未验证跨租户、临时代理、组织范围、金额／字段／文件动作以及 C2～C8；阶段总判定继续为 `insufficient-evidence`。

### 当前 C3 权限与组合证据边界

2026-09-27 的隔离 D1 通过 100 个 HTTP 断言、0 个状态差异完成 C3 项目经营链路：Project 使用独立 `project-workspace.commercial-summary.read` 人类 Capability 和精确 Project Scope，按 Resource Link 并行读取 Contract、Money Obligation／Transaction 与 Billing Invoice Registration。总经理与财务允许，只有普通 Project read 的生产经理由目标 Project 返回 403；撤权后旧凭证只在既定残余窗口有效，重新签发后 403，恢复 Grant 后 200，账号停用后 Session 和新签发均 401。Contract、Money、Billing 和 IAM 分别停机时只把对应 segment 标为 `unavailable`，恢复后无需改 Link 或重放业务命令；Project revision 保持 6。

30 次组合读取的本地 p50/p95 为 13/220 ms，同批来源分段耗时串行等价为 19/362 ms。该结果证明 C3 金额读取可以在版本化契约、目标最终授权和局部降级边界内工作，不证明生产容量、完整双 Tenant、Settlement、应开未开、自动核销或真实 Ubuntu。真实浏览器最终点击因 macOS 自动化通道仍报告锁屏而未计为通过。详细证据见 [OA C3 项目经营链路 D1 实验报告](../../../docs/reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。

## 当前可行性结论

当前架构能够承载 OA 完全体，原因是身份、人员、项目、合同、资金、文件和生产事实已经具有独立所有权与正式组合方式，新增业务可以继续沿相同契约和历史规则扩展。2026-09-27 的 C2 审计进一步确认经营承接不应塞入 Project 或 Contract，而应作为独立 Business Engagement；其后端、D1、页面、权限与本地恢复边界已通过，后续建议和完整双租户矩阵仍待验。同日 Party→Project 实验证明项目客户关系应由 Project 拥有，Party 只提供当前档案，Project 在选择时冻结最小快照；随后的人类工作流又证明用户可用 Party 凭证发现候选，而 Project 服务仍以 workload credential 精确复验，不把浏览器列表当成可信事实。C3 又证明 Project 可以只根据自己拥有的 Resource Link 并行读取 Contract、Money 与 Billing 的实时分段，并用独立人类金额能力、逐资源工作负载凭证和局部降级形成连续体验，而不取得来源事实所有权。该结论否定了把默认客户关系直接放进 Business Engagement，以及为了综合页面建立共享经营表的两个候选。

当前系统尚不能被称为 OA 完全体。现有运行证据主要覆盖身份基础、人员编组、Party Directory 核心生命周期／交付恢复／Project 消费边界、两个台账的来源映射／含引用数据切换与只读回滚、单项目立项、项目包 Base 事实及页面、合同、义务与回款、设计生产、资料引用、Business Engagement 运行闭环、Billing 的独立生命周期和 C3 Project 只读组合、C5 批复审计真实独立进程旅程，以及 C6 成本外协正式 D1／Party 切换／C6→Money 应付建议真实双进程旅程。C7 已完成独立 Bonus Distribution 领域／真库、受保护 HTTP／IAM、真实来源、人类页面、正式 D1、未选缺席、联合恢复、恢复后继续写入及 C7→Money 直接跨进程建议。全业务目标仍需要 C7 正式 D1 connector／建议引用恢复和单类型最小组合、C8 跨域投影、项目包金额分配验证、更多生产类型、C6 connector 完整正式 D1 与只读投影、Money 完整 D1 Party 消费链、完整双 Tenant 和真实 Ubuntu 复演。

因此“能够实现”是架构可行性结论，不是当前完成度结论，也不是仅靠 Suite Manifest 配置即可完成的承诺。新增责任域必须逐个取得生命周期和纠错证据；如果 OA 完全体迫使应用共享业务表、复制可写事实或长期维护客户核心分支，则当前边界验证失败并需要修正。

## 验收门槛

OA 完全体只有同时满足以下条件才算通过：

- C1～C8 使用非真实客户数据从空环境完成，并能幂等重放；
- 已确认 OA 能力逐项映射为 `implemented`、`extended`、`customer-overlay` 或有理由的 `not-applicable`，不存在占位成功；
- 各应用仍可独立启动、迁移、备份、恢复和执行自己的主要闭环；
- 全组合只通过公开契约交换事实，数据库、ORM 和领域对象保持独立；
- 关键来源逐项停机时只影响对应实时能力，不损坏其他事实；
- 客户覆盖层可以移除并重新装入，公共应用镜像和数据库不包含客户名称或客户条件分支；
- 完整入口使用 Nginx、Gateway、IAM 和 Workbench，并复用业务验证通过的同一应用版本；
- 所有未知项在验收前得到证据确认或明确排除，不用推测填满“完整”清单。

## 真实环境门槛

业务重组和边界验证可以在本地持续完成；阶段最终的 `supported` 结论还要求同一不可变发布物在以下至少一个目标通过：

- 获明确授权的公司私有 Ubuntu 环境；
- `xidou.me` ICP 备案生效后的个人阿里云试验田。

目标环境拥有自己的域名、证书、Registry／镜像副本、秘密、数据库、OSS／文件存储和备份。两套环境的凭据、数据和验收记录不混用。真实环境验收覆盖 HTTPS 固定入口链、IAM 与目标应用授权、内部端口隔离、Evidence、应用开关、单应用故障、升级前备份、空目标恢复和恢复后继续写入。

ICP备案或公司授权等待不阻塞本地业务验证；缺少上述任一真实环境的通过证据时，阶段报告只能给出 `insufficient-evidence` 或中期结论。

## 仍需业务确认

- 项目包合同金额向子项目分配、回退和结算的准确规则；
- 未中标、流标、承接终止、重开、多轮承接及承接结果确认的准确规则；
- Business Engagement `ready` 后是否以及由谁明确提出 Contract 草稿建议；
- 专家费的申请、审批、义务建议和实际付款分别由谁拥有；
- 开票红冲、作废、退票和跨期更正的生命周期；
- 退款、退回款和反向核销的实际业务语义；
- 批复审计是否改变合同、义务或只形成独立结果；
- 奖金固定金额分配、多角色重复分配、强制审批、税务／薪酬衔接和舍入余数归属是否形成稳定规则；
- 考勤和工资是否属于目标 OA 自有业务，还是外部系统只读数据；
- 回款提醒的触发人、触发事实、关闭条件和纠错方式。

## 明确不负责

- 复制旧 OA 的数据库表、宽 FinancialRecord、兼容 sidecar 或历史技术债；
- 把所有业务改造成通用流程、低代码表单或万能字段平台；
- 用页面齐全、接口返回成功或菜单存在代替业务状态验收；
- 以一个参考 OA 推导整个商业圈子的最终通用产品边界。
