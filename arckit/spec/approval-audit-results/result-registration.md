# 批复审计结果登记

## 元信息

- 路径：`spec/approval-audit-results/result-registration.md`
- 所属模块：`approval-audit-results`
- 状态：🟡 开发中；责任边界、领域核心、PostgreSQL 生命周期、受保护 HTTP／IAM、精确引用、人类工作区、Project C5 完成检查点、正式 D1 装配／裁剪／联合恢复与真实跨进程业务旅程已验证，完整双 Tenant 和真实浏览器待补
- 优先级：P0
- 依赖能力：IAM、Project Workspace、Contract Ledger、Evidence Service；审计结果默认引用批复结果
- 上层旅程：[OA 完全体 C5](../assembly/full-reference-oa-validation.md#c5批复审计与项目完成)

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 把批复、审计金额、报告文件、合同金额引用和项目完成判断放在同一数据库与生产页面中。当前实现会直接读取合同表、原地修改或删除结果，并且只因“批复和审计两种记录都存在”就把该项视为完成，无法解释使用了哪个来源版本、谁调整过金额以及撤回后为什么仍算完成。

**可验证结果 B**：用户可以独立登记、纠正和撤回批复／审计结果；每个当前结果都能追溯到精确来源 revision、金额构成、Evidence 版本、操作者和不可变历史。Project 只通过版本化只读结果判断完成条件，不改写结果来源，也不因读取页面产生业务写入。

## 用户结果与边界

第一版提供一个独立的 **Approval & Audit Results（批复审计结果）** 应用。它拥有：

- 项目下批复和审计的当前登记结果；
- 金额组成、币种、来源快照和人工覆盖原因；
- 外部确认人／确认时间的登记值；
- 报告 Evidence File ID／Version 的业务含义；
- revision、幂等命令、纠正、撤回和不可变历史；
- 当前结果和最小 reference-read 的动态允许动作。

它不拥有：

- Project 身份、项目类型、Assignment 或项目完成状态；
- Production Work Item、Stage、成果或生产完成事实；
- Contract／Settlement 当前事实；
- 文件字节、预览地址或物理清理；
- 应收应付、开票、实际收付款、核销、奖金或会计事实；
- 客户内部在线审批流、任意流程引擎或电子签章。

应用可以作为单独的结果登记产品交付，也可以在参考 OA 组合中作为 Project／Contract／Evidence 的显式消费者。页面位于“生产信息”或项目工作区不改变事实所有权。

## 第一版结果类型与金额口径

第一版固定两种结果，不允许客户在线新增任意类型：

| 类型 | 含义 | 默认金额来源 |
| --- | --- | --- |
| `approval` | 外部批复结果登记 | 指定 Contract／Settlement 的精确 revision 快照 |
| `audit` | 外部审计结果登记 | 当前有效批复结果的精确 revision 快照 |

参考 OA 客户配置使用版本化 `reference-oa-v1` 金额组成：`design_fee`、`survey_fee`、`consultation_fee`。金额以整数最小货币单位和 ISO 币种保存，合计由服务端计算；不得使用二进制浮点数，也不得信任客户端合计。

该 profile 的单来源规则要求 Project 存在指向目标 Contract／Settlement 及其精确 revision 的活动 Resource Link。来源类别 `design|design_fee`、`survey|survey_fee` 或 `consultation|consultation_fee` 分别把来源金额装入唯一对应项，其余两项为明确的零。来源缺类别、非 CNY、存在小于分的精度、未被 Project 活动链接或 revision 不一致时拒绝登记。一个结果需要聚合多份 Contract／Settlement 时必须使用新的版本化连接器规则，不能由客户端先求和后伪装成单一来源。

默认引用不是实时联表：登记命令逐个读取精确来源，复验 Tenant、类型、ID、revision、状态、币种和摘要后，把最小来源快照冻结在本应用 revision 中。来源以后变化不会静默改写既有结果。

用户可以不采用默认来源而手工登记，或在引用后调整一个或多个金额组成；只要最终金额与来源快照不同，覆盖原因必填。`0` 是明确金额，未知或无权查看必须返回 `null`／隐藏，不得伪装为零。

## 生命周期与纠错

第一版把“登记状态”与外部行政／审计流程分开：

- 成功登记形成 `recorded` revision；
- 资料或金额纠正追加下一完整 revision，旧 revision 永久可读；
- 错录或外部结果失效使用显式 `withdrawn` revision，不物理删除；
- 同一 Project、同一结果类型只有一个当前未撤回结果；并发命令以 expected revision 决定唯一胜者；
- 重复 Command ID 且请求摘要相同返回原结果，请求摘要不同返回冲突；
- 重新登记已撤回类型形成后续 revision 链，不复活或覆盖旧行。

参考 OA 文档提到 `pending／approved` 和“两级在线审批”，但当前页面没有该操作，运行代码也只把记录存在视为完成。第一版不把这项冲突证据写成公共状态机；取得真实操作者证据后再决定它属于本应用策略、独立审批产品还是客户专项。

## Evidence 行为

- 业务记录只保存经过精确引用验证的 File ID／Version、稳定摘要和显示快照；
- 文件上传由 Evidence Service 完成，结果应用不嵌入存储 SDK 或长期密钥；
- 新 revision 引用的任一文件不可用、错 Tenant、错版本或摘要不符时，命令失败且不写结果、Command 或历史；
- 已登记历史在 Evidence 暂时不可用时仍可解释，预览／下载明确降级；
- 替换或撤回业务引用不直接删除 Evidence 字节。

## Project 完成判定

Project 拥有“这个 Project 是否满足所选完成规则”的解释与投影，但不拥有来源事实。参考 OA 使用版本化完成规则配置；规则逐项读取 Business Engagement、Contract／Settlement、Production、Approval & Audit Results 以及后续 C6／C7 的最小投影，并返回：

- `satisfied`：精确来源 revision 满足规则；
- `not_applicable`：由对应事实所有者明确登记不适用；
- `pending`：缺少或状态不满足，并给出来源与原因；
- `unavailable`：来源暂不可用，不能冒充未发生或已完成。

读取完成视图不得改变 Project 或来源应用。最终 Project 状态变化必须使用显式命令、expected revision 和所用规则／来源 revision 集合；来源后来被纠正或撤回时，投影显示失配并要求显式重评，不静默反向修改历史 Project 状态。

C5 首轮只装入批复审计结果及其完成条件，不因为 C6 成本外委和 C7 奖金尚未实现而伪造整个 Project 已完成。完整参考 OA 的最终完成规则要等 C6／C7 责任边界确认后联合验收。

## 权限要求

目标应用发布稳定业务动作，而不识别客户岗位名：

- `approval-audit-result.create`
- `approval-audit-result.read`
- `approval-audit-result.amount.read`
- `approval-audit-result.correct`
- `approval-audit-result.withdraw`
- `approval-audit-result.evidence.manage`
- `approval-audit-result.history.read`

参考 OA Overlay 可以把生产负责人映射为项目内登记／纠正角色，把总经理、项目负责人或财务映射为不同只读范围；身份超级管理员默认没有业务权限。目标应用仍复验 Tenant、Project Scope、结果资源、状态和 revision，并返回服务端收敛的 `allowed_actions`。金额读取、文件查看、文件管理和结果写入分别授权。

## 第一实现轮验收矩阵

| ID | 场景 | 必须证明 |
| --- | --- | --- |
| C5-A01 | 独立所有权 | 结果应用使用独立数据库；Project、Production、Contract 不新增可写结果表 |
| C5-A02 | 两种结果 | 同 Project 的 approval／audit 分别登记且当前投影不互相覆盖 |
| C5-A03 | 批复来源 | 精确 Contract／Settlement revision 被复验并冻结，来源变化不静默改写 |
| C5-A04 | 审计来源 | 审计默认冻结精确 approval revision，不重新误取当前合同金额 |
| C5-A05 | 精确金额 | 整数最小货币单位、同币种合计、负数／溢出／客户端伪造合计拒绝 |
| C5-A06 | 人工覆盖 | 与来源不同必须填写原因；原因和差异进入每个 revision |
| C5-A07 | Evidence | 多文件精确版本；故障零写入，恢复后原 Command 只成功一次 |
| C5-A08 | 纠正与撤回 | append-only revision、expected revision 冲突、无物理删除 |
| C5-A09 | 权限 | 允许角色、相邻角色、Project B、错 Tenant／Audience／Scope 和无金额权限矩阵 |
| C5-A10 | Project 完成视图 | 逐来源返回满足／待办／不适用／不可用和下钻原因；GET 零写入 |
| C5-A11 | 完成命令 | 显式 expected revision 与来源集合；来源失配失败关闭且不改其他应用 |
| C5-A12 | 最小组合 | IAM／模板 + People & Groups + Project + Contract + Evidence + Results 可完成登记；People 是 Project Assignment 的显式依赖，Production 可不选装 |
| C5-A13 | 未选缺席 | 未选 Results 时既有 C1～C4 组合可启动、备份、恢复且无空菜单／路由 |
| C5-A14 | 装配恢复 | Suite Manifest、Workbench、D1 正式入口、联合空目标恢复保持 ID／revision／摘要 |
| C5-A15 | 回归与反耦合 | Level A／B、C1～C4 回归；无共享表、跨库 ORM、同步双写、物理删除或通用审批引擎 |

## Base／Overlay 变化账

| 层 | C5 第一版归属 |
| --- | --- |
| Base | 结果、精确金额、来源快照、revision、纠正／撤回、Evidence 引用与权限内核 |
| 版本化策略 | `reference-oa-v1` 金额组成和 Project 完成要求集合 |
| Customer Overlay | 客户角色映射、术语、启用哪种完成规则和不适用条件 |
| Suite Manifest | 是否安装 Results、Project、Contract、Evidence 和 Workbench |
| 连接器／投影 | Project 只读完成评估；不得写 Results、Contract 或 Production |
| 客户专项 | 一次性旧 OA 数据迁移或报表，不进入公共状态机 |

## 明确未知与退出边界

以下事实没有足够证据，第一版保持未知：多轮批复／复审的正式身份、两级在线审核／审定、外部签章真实性、一个结果引用多个合同的业务优先级、审计结果自动生成结算／义务、撤回后已完成 Project 的业务处置。实现不得用通用流程、默认级联或自由 JSON 字段填补这些未知。

## 当前实现证据

领域核心使用独立 module、migration 和 PostgreSQL Store，已经验证同 Project 的两种结果、固定三项整数金额、精确来源 revision、人工覆盖原因、幂等、并发、租户隔离、追加纠正、撤回后重登、历史防篡改和引用故障零写入。审计从本应用的 approval 历史 revision 取得默认金额，不调用外部 Contract resolver，也不随之后的批复更正漂移。结果纠正复验当前 Project，而不是要求 Project 仍停留在结果首次登记时的 revision；旧 Results revision 继续保留当时的 Project 快照，新 revision 冻结纠正时的当前快照。

受保护 HTTP 验收以正式 Ed25519 IAM Credential 驱动目标应用最终授权，并由 IAM Capability Catalog 实际装入 Results Manifest。矩阵覆盖允许主体、相邻角色、Project B、错 Tenant、错 Audience、金额与 Evidence 缺权；服务端金额脱敏与动态 `allowed_actions` 不依赖页面隐藏。Project、Contract／Settlement 与 Evidence 的版本化 reference-read 使用精确资源工作负载凭证，依赖 503 时 current／revision／command／audit 零写入，恢复后原 Command 只成功一次；成功 Command 重放不重新依赖外部服务。stale revision、未链接来源、命令载荷冲突和 expected revision 冲突分别失败关闭。

Results 人类工作区现在支持按 Project 读取当前两类结果、登记、纠正、撤回与历史；写入口完全依赖服务端 `allowed_actions`，无金额能力仍由服务端脱敏。Project Workspace 已实现 `reference-oa-c5-v1` 只读评估与显式检查点确认：GET 返回两项 requirement、`satisfied／pending／unavailable`、最近确认和 drift，不写 Project 或 Results；确认要求 Project expected revision 与精确 result ID／kind／revision／digest 一致，只追加 Project revision、确认快照、命令和历史，Project 保持 `active`。成功命令回放在访问 Results 前命中本地结果。

该证据覆盖 C5-A01～A11 的当前本地实现切片与 C5-A15 的局部反耦合检查；`not_applicable` 已保留为公共状态，但 C5 两个 requirement 当前没有伪造不适用策略。

Results 现已作为正式 D1 可选应用接入独立数据库、migration、工作负载密钥、IAM Catalog、Suite Manifest、Workbench、Gateway、镜像供应和联合备份恢复。隔离组合已通过 Nginx→Gateway→IAM 一次登录、Results 页面、已选应用读写、未选应用 `404`、十项选择清单、SHA-256、销毁卷后的空目标恢复和恢复后复验；因此 C5-A13／A14 的装配与恢复子集成立。

真实独立进程验收又以 74 个 HTTP 断言完成 Contract／Evidence→approval→audit→Project 确认，覆盖 Evidence／Contract 503 零写入、原 Command 恢复、无金额脱敏、跨 Project 拒绝、无 create 拒绝、`correct` Grant 撤销／恢复、确认后来源纠正产生 drift，以及 Results 停机时 GET unavailable、旧成功确认本地回放、新确认 503 且 Project revision 不变。C5-A12 及 C5-A09 的 Project Scope／金额／纠正权限生命周期子集成立；完整双 Tenant、账号停用、真实浏览器和一个结果聚合多份合同仍未证明。产品整体继续处于开发中，不能声明 C5 全部通过。
