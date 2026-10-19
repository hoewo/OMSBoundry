# OA 完整体架构可行性验证计划

2026-10-07 BF-121 验证调序：按用户优先项先验统计 UI、来源分区／精确金额、缺权／来源故障／迟到请求、支付和开票独立导出，以及四层权限属性、草稿审阅和配置导出。浏览器夹具与来源 HTTP 单测只证明各自范围；真实 IAM、源库、部署和整体 C8 不据此升级为通过。项目详情问题延后。[规格](./specs/assembly/oa-statistics-permission-ui-v1.md)。

2026-10-06 BF-120 当前执行基线：用户确认以 `AGENTS.md` 的实现摘要为准，后续中断实验不再直接关闭本计划门禁。先完成 R0 基线收敛和 C1～C8 缺口矩阵；C1～C6 只复验可复演证据并补缺，C7 从 Bonus 核心／PostgreSQL 基线继续补 HTTP／IAM、真实来源、页面、D1／恢复与 Money 建议，C8 随后完成资料／统计／个人工作；最后再做组合／Overlay、真实环境和报告。旧运行记录保留，但未经当前代码、持久证据与干净版本复验，不得作为 `passed`。见[当前基线与下一步](./status/current-baseline-and-next-actions-20261006.md)。

2026-10-06 BF-119 验证口径：本计划不把边界稳定解释为验证期零变化。当前完整 OA 完成度低，真实操作引起较多模型和依赖调整属于预期；但每个闭环必须留下显式版本、兼容／迁移／恢复及反例证据，并在连续闭环中看到改变 Owner 数、公开契约／migration、强制依赖、同步调用链和旧场景回归面总体下降。长期客户差异应逐步收敛到 Overlay／配置／策略／连接器／只读投影；不能出现共享表、双写、Workbench 第二真相或核心长期 fork。当前 Project、Production、Contract 整体仍为候选，不因局部切片通过而提升完整 OA 结论。

2026-10-06 BF-118 状态：原 OA 对外委托主路径已通过 Cost／Party／Contract／Evidence／Money 分责、发生时联系人快照、原 UI、真实文件、未知回执重放、Cost→Money 独立接受、请求上界、精确 Workload Scope、IAM 撤权和八库恢复的私有 10／10。Project 仅创建边界精确读且零写，Production 请求为 0，Contract 仅显式选择时精确读且零写。该证据不关闭 Money 拒绝／真实实付、逐依赖停机、跨 Tenant／全角色、正式 D1、Main／公网、第二客户或完整 OA。下一切片是 F01／F02 资料管理和 S01～S03 统计，要求来源 Owner 单一真相、原页面操作、权限／故障／空数据分离、下载记录语义和恢复逐项有证据。

2026-10-06 BF-117 状态：原 OA 批复／审计已通过 Results 单一事实所有者、Contract 精确来源、Evidence 真文件、正式 IAM、回执丢失重放、纠正／撤回重登记、请求上界、六所有者恢复的私有 11／11。Project／Contract／People 摘要不变，Production 请求为 0；空 Evidence `null` 输出缺陷已在 Owner 端修正并重跑。该证据不关闭 Project Completion Checkpoint、Main／公网、正式 D1、全角色、第二客户或完整 OA。下一切片为对外委托，要求原组件操作、五 Owner 分责、局部成功续办和恢复逐项有证据。

2026-10-06 BF-116 状态：四类型生产总览已通过 Production 单一事实投影、原 OA 页面复用、请求数上界、正式 IAM 撤权、真实 PG18 和五所有者逻辑空目标恢复的私有 10／10 验收；Project／Evidence／Contract 普通总览读取为 0，恢复前后五库摘要和 4 份 Evidence 字节一致。该证据验证一种无第二真相的生产读模型，不关闭 Main／公网、正式 D1、全角色、第二客户、类型重分类或完整 OA。下一切片为 Results 批复／审计，要求原组件操作、独立 Owner、冻结来源、纠正／撤回、撤权／故障／恢复与 Project 完成检查点逐项有证据。

2026-10-06 BF-115 状态：生产阶段资料已从 v2 方案顺序演进至 v6，并以试验／报告验证“Project 当前类型→Production 发生时最小适用性快照”；私有真实链 9/9 与五所有者逻辑空目标恢复通过。该证据只覆盖一个客户场景、一个 Work Item 和既定四类型键，不关闭完整 Production、Contract、跨客户 Overlay、正式 D1／keys、Main／公网、全角色或整套 OA。下一验收切片是四类型生产总览／进度：要求原 OA 完成口径、Production 单一事实来源、零 Workbench 事实、缓存后的请求上界、撤权、来源故障、升级兼容及恢复后同页继续全部有证据。

2026-10-06 BF-114 有限证据：变更／竣工原双列表已通过私有真实入口、正式 IAM、Production／Evidence、远端 PG18 和五所有者逻辑空目标恢复。v5 只在首次真实辅助资料写入由 v4 显式升级，变更可选、竣工必需且 completion 完成不伪造空的前序阶段。Project 无写入／契约变化，Contract 未进入依赖图。该结果增加异类阶段复用证据，但不关闭 Production 整行、Main／公网、正式 D1、全角色、试验／报告、第二客户或完整 OA；整体继续 `insufficient-evidence`。[报告](./reports/oa-change-completion-native-validation-20261006.md)。下一项试验／报告必须按原类型适用性重新验边界。

2026-10-06 BF-113 有限证据：施工图原 UI 已通过真实私有入口、正式 IAM、Production／Evidence、远端 PG18 和五所有者逻辑空目标恢复。v3 前序初设语义保持，v4 及显式 3→4 只在首次真实施工图写入发生；没有新增依赖边。方案、初设、施工图使阶段资料子边界达到同类可重复，但不关闭 Production 整行、Main／公网、正式 D1、全角色、变更竣工以后阶段或第二客户；整体继续 `insufficient-evidence`。[报告](./reports/oa-construction-stage-native-validation-20261006.md)。下一项变更／竣工必须按异类 UI 和完成规则重新验边界。

2026-10-06 BF-112 历史证据：初步设计原 UI 通过真实私有链，建立 v2→v3 首次兼容演进证据；现由 BF-113 的 v4 施工图证据继续演进，原报告范围不变。[报告](./reports/oa-preliminary-stage-native-validation-20261006.md)。

2026-10-05 BF-110 可见性补证：公网 Main 的生产区已将页面导航与原生接线状态分离，10 个当前可见 Tab 全部可进入；初设、施工图和变更／竣工复用原 OA 阶段结构并明确未读取，未发业务写入或无关生产读取。此项验证 UI 壳、URL 选择和边界表达，不验证这些阶段的原生保存、Evidence、权限撤回、故障续办或恢复；整体结论继续 `insufficient-evidence`。[报告](./reports/oa-production-stage-navigation-validation-20261005.md)。

2026-10-05 BF-109 权限补证：原 OA 管理员规则49／49、当前 Suite 原生能力145／145及12个 Audience Credential 已对齐；公网外委和奖金实际抽屉可开，Bonus→Project候选在未初始化人员时返回200空集，0个403／429／5xx。该结果验证规则、IAM、工作负载和目标最终授权可以分层协作，不代表三个缺失生产阶段适配器、批复业务前置、完整 OA 或整体架构已经通过；整体仍为 `insufficient-evidence`。[报告](./reports/oa-admin-full-access-validation-20261005.md)。

2026-10-04 BF-106 有限可见闭环：正常原方案面板／抽屉经真实私有Nginx→Gateway→正式IAM→Production／Evidence／PG18完成16场景，含多主文件／校审单／评分、真实预览下载、失败原命令恢复、撤权和五所有者逻辑空目标恢复后原UI继续。[报告](./reports/oa-production-stage-native-validation-20261004.md)明确原UI-only、原生层和恢复层限制。它不关闭生产整行、阶段真实D1／keys／旧会话恢复、Main旧任务过渡／公网、全角色、其他阶段及全站连续旅程；BF-105的所选D1证据不能转作新阶段profile通过。已注册v2不原位增阶段，后续显式新版本／过渡保持旧事实可追溯；整体仍为 `insufficient-evidence`。

2026-10-04 所选 D1 恢复追加：BF-105 人员／专业奖金的本地所选六库实际联合冷恢复、精确授权／原命令保持及原页面继续通过14场景，[报告](./reports/oa-professional-selected-d1-restore-20261004.md)。不关闭完整启用组合、旧主站升级、Main／公网、全角色、完整 OA；下一[原阶段逐动作闭环](./specs/assembly/oa-production-stage-original-operation-map.md)按原 UI 和原生业务两层分别验，整体 `insufficient-evidence` 不变。

2026-10-04 D01→B12有限候选：新类型化分工引用与显式Bonus v3已接正常工作台，私有Nginx／Gateway／正式IAM和各自PG18完成20场景，旧Base／v2保持。[实际报告](./reports/oa-professional-bonus-continuous-validation-20261004.md)。人员及奖金整行不关闭，Main精确授权／匹配发布、其他生产阶段、全角色、全站连续旅程／联合恢复仍待验，整体继续 `insufficient-evidence`。

2026-10-04 D01 原生候选：原专业／审定 UI 复制后已接真实 Project／IAM／People＋PG18，21 场景、315 项 Node、Project 全量真库及并发／单库恢复分层通过；六客户角色及目录只在有界 Overlay，不改旧职责。[实际报告](./reports/oa-production-personnel-native-validation-20261004.md)。正常 Workbench／Main 未启用，新引用／Bonus 消费、全角色、匹配入口发布、其他生产子栏目和全站连续旅程仍待验；不能把人员切片或单库恢复代表完整架构，整体继续 `insufficient-evidence`。

2026-10-04 续轮：B12 已有原组件、实际 Workbench／Gateway／真实 IAM／Bonus 和独立 Project／People 的有限可操作组合；正常保存、撤回、刷新、金额撤权、来源停机和单库恢复续办通过，旧奖池事实保持。BF-103 只补有限入口兼容，不改变入口链或最终授权，[报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)。仍未发布 Main、未跑 Nginx 公网／D1 匹配升级／Suite 联合恢复；B11、生产专业分工、全角色、开票／资金关联及其他全站缺口保留，整体 `insufficient-evidence`。

2026-10-04 08:32 UTC：B12 手工奖金原组件→独立 Bonus HTTP→PG18、单库空目标恢复续办和旧 schema 2→3 保持有限通过，[报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)。未发布，签名测试 issuer／template 与局部来源夹具不冒充真实 Nginx→Gateway→IAM 链。B11、专业生产、全角色、Billing 授权／公网链和其余全站缺口保留，整体仍 `insufficient-evidence`。

2026-10-04 07:36 UTC 当前状态（以下候选未发布是历史）：经营 Owner、Cost schema 3 和 Workbench `.6` 原组件已兼容发布。C 管理员成本有限连续链及十二侧别／分类组合、合同／支付九检查点、实际历史区与文件预览，以及原生边界收尾分别通过，Project revision 18。真正阻断连续撤回的层级故障已复现、修复、上线并正常点击续原记录，不以接口成功代替 UI 通过。完整经营驱动仍因 Billing 单独授权不足为 `passed:false`；没有代替全角色、最终更正、更多材料、奖金、四类型生产／结果／外委、资料统计、OSS 和完整 Suite 恢复运行。整体结论继续 `insufficient-evidence`。当前事实与剩余步骤见[公网报告](./reports/oa-commercial-public-ui-validation-20261004.md)和[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)，不是可见表单或局部 API 拼出的整体通过。

2026-10-04 整体结论仍为 `insufficient-evidence`。合同、支付、开票原组件候选 ae／j／f 与专项 PG18／启动分层通过；Contract 实际 IAM／Evidence／字节恢复 m 和历史夹具 18→22 原子迁移 a、Billing 最新 c 真库及实际 IAM／Evidence／字节恢复 e 已通过。候选未发布，组件内存端口、测试 issuer、正式 issuer＋局部来源夹具与公网不得互替。B04～B06、[成本缺口](./specs/assembly/oa-cost-original-operation-map.md)／奖金、四类型生产、结果／外委、资料统计、全角色、OSS／完整 Suite 恢复均未因此关闭；完整范围见[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)。新事实仍归各小应用，以下早期待验保留历史。 最新构建、截图与 Main 17 服务只读收尾统一见[本轮检查点](./reports/oa-complete-ui-checkpoint-20261004.md)。

2026-10-04 全量范围保持：Contract 原组件 22 场景以及真 IAM＋Contract＋Evidence 联合恢复 m 已分层通过；local 文件、private 网络、候选未发布，不关闭完整 B04～B06。支付继续复用原双列表／640px 表单并接原生实际资金与冲销；本切片不包含开票关联／核销／奖金。开票、成本／奖金、全部生产类型、资料统计、全角色、OSS 与正式交付都按[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)保留，不以新候选覆盖旧公开缺口。

2026-10-04 合同候选推进：原双列／抽屉、原始合同与三类协议显式确认、三类材料及阶段／最终结算已接线，schema 22 真库、应用库空目标恢复及组件分层通过；未部署，真实 IAM＋Evidence 联合、字节恢复、全角色／更正／发布待完成。按[全站交付清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)继续，不把原 UI 或专项证据拼成 B04～B06 或整体通过；当前仍 `insufficient-evidence`。[候选报告](./reports/oa-contract-draft-ui-candidate-20261004.md)与下方历史状态分别记账。

2026-10-03 完整性门禁重申：按用户要求，招投标只是例子，新增[全站工作面交付清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)。BF-097 兼容演进商业参与职责、显式新能力和原人员组件，真实名单／增减／历史重入／丢回复原命令恢复已验；当前 `.6` 公网三项零写导航续验通过，旧事实保持，失败反例保留。合同原生写端口仅真库及独立启动候选，尚未部署或接原表单。完整角色、父包传播、两链合同／资金、生产及资料统计没有因单个切片关闭。分层证据见[当前报告](./reports/oa-complete-operation-progress-20261003.md)，整套仍为 `insufficient-evidence`。

2026-10-03 BF-096 首个真实组件切片已验证：原招投标组件局部接入 Workbench，Auth58 `.20261003.1` 上完成上传、原预览／下载、单份结束引用、Project 当前资料复读、刷新零写及未知回复后同命令继续。范围和失败见[本批报告](./reports/oa-reused-bidding-ui-validation-20261003.md)。它不是全 B03／原站现场等效或全站验收；当前文件 local、前端 ECS，OSS 分离和完整 Suite 恢复运行待验。后续继续直接复用现成 UI，按原 OA 操作、来源、权限与恢复逐项关闭，不把组件数量或局部闭环算成架构整体通过。

2026-10-02 BF-093 验收澄清：完整 OA 的可见使用效果与完整架构验证是当前首要结果，小系统可在过程中成长。每项本体变化必须能预测其所有权、契约、兼容、失败和恢复影响；不把本体零修改作为成功前提，不把职责未合并当作成熟证明。逐操作 UI／真实保存与复读、原生权限、独立性／故障隔离、兼容发布／恢复共同构成最终门禁；跨客户长期演进仍另需证据。下一命令切片 B03 见[规格](./specs/assembly/oa-bidding-command-slice.md)，其余缺口仍保留。

2026-10-02 BF-092 补充：用户完成立项后的页签不可点击先作插入修复，验证同项目切换、真实只读来源、权限／局部失败和刷新保留；完整立项、经营成员类型／分权和所有录入表单并未因此通过。来源缺 Link 不冒充空记录，项目全部成本不冒充经营成本。见[页签验证](./reports/oa-business-tabs-repair-validation-20261002.md)。

日期：2026-09-29  
状态：已接受；下一阶段产品与架构验证主线  
关联决策：BF-029、BF-030（有限页面范围已由 BF-084 取代）、BF-033～BF-042、BF-083、BF-084、BF-090、BF-091

2026-10-02 范围升级：本轮已获完整仿制修复授权，原 OA 当前可用页面及操作不再只选代表旅程；以[原 OA 使用流程基线](./specs/assembly/oa-original-usage-workflow-baseline.md)的逐动作入口、字段时机、导航、结果和权限验收为准。C1～C8 的工程证据继续保留，但不能替代原流程等效或真实浏览器验证。

当前执行门禁（BF-090／BF-091）：Auth58 `.2` 已关闭缺经营分工＋已有生产分工的旧待立项恢复阻断，专用同类夹具真实点击完成补齐／更换、同一 OA 详情和刷新复读；用户“测试项目101”仍保持其原资料／历史，待用户明确选人提交，见[修复报告](./reports/oa-initiation-recovery-repair-validation-20261002.md)。I01～I04 的其余可录入字段、全角色撤权和完整失败矩阵仍未关闭，继续按上述基线第 9.3 节执行，不先跳新的 B02。之后逐步推进项目包、经营、分类型生产、资料统计、权限及连续恢复；原站实际环境未验时明确记源码证据，不能宣称已全面掌握或完整 OA 已通过。

## 1. 问题与可验证结果

当前 J1～J4 已证明六个小系统能够独立运行并重组出参考 OA 的首个高保真业务切片，但尚不能回答当前架构是否能够承载参考 OA 的完整业务范围，也不能证明“稳定 Base + Customer Overlay”能在真实交付环境中成立。

本阶段把问题收口为：

> 当前架构只通过了局部参考旅程，完整业务覆盖、定制边界和真实部署仍缺少连续证据 → 使用最小应用集合、显式依赖和受控客户覆盖层重组参考 OA 的完整业务范围，并以可复核报告判断架构方案是否可行。

最终结果不是“页面看起来像 OA”，而是以下五类证据形成同一条链：

1. 参考 OA 的 C1～C8 业务旅程能够使用非真实数据完成、纠错和重放；
2. 每个事实只有一个所有者，小应用仍可独立交付，跨应用只使用版本化契约；
3. 客户差异可以唯一归入 Base、配置、策略、类型化扩展、连接器、客户装配或隔离专项；
4. 同一不可变发布物在至少一个真实 Ubuntu 目标环境通过完整入口、升级、备份和恢复验收。
5. OA 的站点职位、项目角色、临时授权、资源范围和业务状态在 Nginx、Gateway、IAM／Policy 与目标应用之间保持可解释、可撤销、可审计且不能被客户端绕过。

## 2. 本阶段验证什么

### 2.1 业务覆盖轴

本阶段覆盖 [OA 完全体业务重组验证](../arckit/spec/assembly/full-reference-oa-validation.md) 中的 C1～C8：

- 项目与责任建立；
- 经营承接与合同链；
- 开票、应收与回款；
- 多类型生产交付；
- 批复、审计与项目完成；
- 成本与外委；
- 奖金计算与分配；
- 资料、统计与个人工作。

“完整”以原 OA 当前可用页面和完整用户结果为准：布局、交互、字段时机和导航需要仿制；旧表、旧内部接口、技术债和已识别缺陷不直接复制。未取得证据的规则保持未知，不用通用流程、万能字段或占位成功填满清单。

### 2.2 架构边界轴

2026-10-02 增补独立的**用户操作等效门禁**（BF-083）：业务覆盖必须按[原 OA 使用流程基线](./specs/assembly/oa-original-usage-workflow-baseline.md)逐动作记录原入口、使用者、有效权限、此时必填／可后补、保存事实、状态、下一步／返回和失败继续。原文档与代码冲突、原站未实现项、克隆未知项分列；源码梳理不等于原站真人通过，API／数据库与 C0～C8 通过不等于克隆用户可用。先验证生产负责人留空完成立项、同项目基本详情导航和后补，再覆盖项目包、两链经营、四类型生产、不适用恢复、附件／外委部分成功、资料统计及权限。统一界面与独立系统边界需同时成立，不复制旧共享表或技术债。关闭原 C8 两项不自动关闭此门禁。

每个新增能力必须同时证明：

- 它有明确的事实所有者、稳定 ID、revision、生命周期和纠错方式；
- 它是现有应用增量、类型化扩展还是新的独立应用，理由可由业务不变量解释；
- 它不要求共享业务表、跨库外键、共享 ORM、领域对象复制或长期客户核心分支；
- 未选择的应用真实缺席时，已选择组合仍可启动、迁移、备份、恢复和完成主要闭环；
- 连接器只转交来源快照、事件或建议，不替目标应用确认事实；
- IAM、Evidence、Party 等直接依赖通过 Suite Manifest、版本和故障验收显式声明。

### 2.3 权限与身份治理轴

OA 权限是本阶段的独立高风险验证轴，不以统一登录、全能力管理员或页面按钮显隐代替。详细技术方法见 [OA 完整体权限架构验证方案](../arckit/tech/assembly/oa-authorization-validation.md)。

每条 C1～C8 旅程同时验证：

- 身份超级管理员、OA 业务管理员、全局只读角色、业务经理、项目负责人、普通成员和财务／办公角色之间的职责差异；
- Tenant、Audience、Capability、Project／Resource Scope、业务状态和 expected revision 共同参与结果；
- Nginx 不解释权限，Gateway 清洗外来身份并执行入口门禁，IAM／Policy 签发有界凭证，目标应用作最终授权；
- 客户职位和项目角色只在 Overlay／策略层映射，目标应用只认识自身稳定业务动作；
- 前端 `allowed_actions` 与服务端命令使用同一动作语义，直接构造请求、旧页面按钮和伪造 Header 均不能绕过；
- 授权撤销、账号停用、Assignment 结束、凭证过期、IAM／Policy 故障和目标应用故障均有明确行为与审计；
- 权限变化不产生共享表、跨库读取、万能管理员旁路、客户核心分支或目标应用中的客户角色条件。

当前证据已经覆盖统一入口、Header 清洗、单 Audience、Tenant Scope、Contract／Money 动作级 401／403、Project Resource／Assignment Policy、工作负载凭证，以及 C1 项目责任、项目包／父子转换／安全恢复、客户页面 `allowed_actions` 和撤权恢复的身份管理员反向隔离、九角色、多 Project／Package、状态与撤权矩阵。C2 又完成 Business Engagement 的独立 Overlay／Assignment Policy、经营／生产／管理／财务／身份管理角色、跨 Project、Grant 撤销、Assignment 结束、目标状态拒绝、精确 Project／File 工作负载凭证、Workbench 页面与实际浏览器撤权刷新；最新正式入口达到 110 条断言，覆盖 409 无写入、Evidence 503、账号停用、IAM 502、Policy 事实源 503、失败不签发和恢复后单次签发，并以真实浏览器复核 409、503、Project 上下文返回和撤权退出。Billing / Invoice 已完成独立领域核心、PostgreSQL 真库、完整生命周期 HTTP、Linux AMD64 非 root 交付、正式 Nginx→Gateway→IAM 入口、逐引用工作负载授权、故障恢复、联合空目标还原和人类页面，覆盖 Tenant／Project Scope、相邻资源、金额收敛、Evidence 和 revision 1→4；C3 Project 经营链路又通过四类来源分段、独立金额 Capability、三角色、撤权／停用及来源／IAM 故障恢复。C4 Production 当前本地轮次已通过 canonical 权限、多类型隔离、正式 D1 最小组合、未选应用缺席、IAM／Evidence 故障恢复和联合空目标恢复；新的 86 条断言和真实浏览器已通过 Grant 撤销／恢复、账号停用／启用、Assignment 结束与页面 `allowed_actions` 收敛。连续 C5 又通过 Results 金额分权、Grant 撤销 403／恢复、四类精确 workload scope 和目标应用最终授权；连续 C6 进一步通过 Cost 与 Money 的完整 action namespace、exact suggestion resource scope、目标停机恢复和双目标最终授权。目标应用已直接拒绝有效签名但 Tenant 不匹配的凭证；仍未证明完整双租户 D1 交叉矩阵、C3／C5／C6 真实浏览器最终点击、字段／更广文件动作和 C7～C8 完整权限旅程。全能力管理员通过业务验收只作健康控制，不计为该轴通过。

### 2.4 定制化轴

每次 OA 增量都建立变化账目：

| 归属层 | 可接受内容 | 失败信号 |
| --- | --- | --- |
| Base | 已稳定复用的事实、状态机、历史、契约和公共缺陷修正 | 客户名称或岗位进入公共内核 |
| 配置 | 术语、显示、枚举和启用选择 | 配置改变事实含义或纠错规则 |
| 版本化策略 | 明确输入、输出、版本和追溯的计算／判断 | 隐藏 SQL、页面公式或不可重放脚本 |
| 类型化扩展 | 有独立生命周期和完成条件的业务类型 | 通用 Project 吸收全部行业字段 |
| 连接器 | 公开契约间的幂等搬运与确认 | 直接写目标数据库或替目标确认 |
| 客户装配 | Suite Manifest、旅程、菜单和角色映射 | 修改公共镜像或绕过服务端权限 |
| 客户专项 | 可删除的导入、迁移、报表或一次性适配 | 长期 fork、现场核心补丁或第二套内核 |

如果某个能力无法唯一归属，先停在规格与证据阶段，不进入实现。

### 2.5 真实环境轴

真实环境采用二选一门禁，任一路径通过即可形成阶段最终部署证据：

- **公司私有 Ubuntu 环境**：取得明确授权和隔离目标后，使用公司拥有的域名、证书、Registry／镜像副本、秘密、数据和备份执行完整验收；
- **个人阿里云试验田**：`xidou.me` ICP 备案生效后，使用个人 ECS、ACR、OSS、域名和证书执行完整公网验收。

两条路径可以先后都做，但证据不得混用。公司环境不会继承个人环境的秘密、数据库或备份；个人试验田也不会因公司环境可用而废弃。

环境等待期间继续完成本地业务、契约、故障、发布包、备份恢复和只读目标预检。没有至少一条真实环境证据时，可以生成阶段中期报告，但最终结论只能是 `insufficient-evidence`。

## 3. 不在本阶段证明

- 不证明整个行业或商业圈子的通用产品边界；
- 不提前选择下一轮参考应用或真实客户；
- 不用一个参考 OA 宣称跨客户产品线已经成立；
- 不要求 D2～D4 或高可用成为阶段通过前提；
- 不把最终品牌精修、全量性能优化或所有外部集成作为业务架构通过条件；
- 不把成熟标准 Web 前端、全站 OSS 静态托管或完全客户定制前端作为本轮 C1～C8 通过的替代证据；它们是 BF-080 的独立产品化验证轴。网页静态资源迁出可以作为单独部署实验，但不能代替真实业务按钮、权限、纠错和恢复验收；
- 不实现缺少责任、状态、纠错和验收证据的猜测功能。

## 4. 推进方式

### 阶段 A：冻结基线与证据账

已完成的六应用 Level A、J1～J4 Level B、五来源故障隔离、Customer Scenario Evidence v1 和 `v0.1.0-demo.11` 可移植包是回归底座。后续工作不得用新的“完整体”脚本替换或降低这些门禁。

现有 M7 权限测试同时作为安全回归底座，但只代表 IAM／Gateway 与两个台账的基础能力。阶段证据账单独记录权限假设、角色夹具、资源范围、允许／拒绝结果、撤权与故障行为，禁止把系统管理员路径外推为 OA 权限完成。

### 阶段 A2：建立 OA 权限验证基线

在新增业务责任域前先建立可复用权限夹具与断言：至少包含身份超级管理员、OA 业务管理员、总经理只读、经营／生产经理、项目经营／生产负责人、普通业务成员和财务角色；至少准备两个 Project、两个 Tenant 边界和一个状态不允许写入的业务对象。

每项新增能力在实现前声明 Capability、资源类型、Business Fact Snapshot、策略映射和目标应用最终判断；实现后执行允许、相邻角色拒绝、跨项目拒绝、跨租户拒绝、状态拒绝、撤权后旧页面提交和直接 API 绕过测试。不存在真实资源级策略实现时，对应结果保持 `insufficient-evidence`，不得用 Tenant Scope 补写通过。

### 阶段 B：建立共享业务依赖边界

独立 Party Directory 是结构化主体的目标所有者；第一版生命周期、模型、API、IAM、故障和迁移边界已经定义，独立 Go／PostgreSQL 核心、release、契约随包、非 root 镜像、完整备份和空目标恢复已验证。首个 Project 消费边界又已进入正式 D1 API 链：Project 用请求绑定、单 action、精确 Party Scope 的工作负载凭证读取 active 同租户主体，并冻结最小快照；Party 停机时新引用失败关闭、既有快照可读、Base 的无 Party 创建仍可用，恢复后新命令只写一次。包含 Party 的 D1 联合备份和空目标还原已保持 Party／Project ID、revision 和快照。Party 管理页和 Project 客户选择器已进入 Workbench；浏览器使用当前用户的 Party 原生 Capability 发现候选，Project 服务仍以工作负载凭证复验所选 ID／revision。Base 保持客户 Party 可选，Overlay 可收紧为必填。Contract／Money 已完成确定性只读迁移预检、来源应用自有不可变映射和精确 Party revision 验证，并新增来源应用自有、追加不可变的运行时转换账。两侧含引用数据的 PostgreSQL 18.6 用例已通过切换／只读回滚、本地写权冻结和防篡改；Contract-linked C6 的正式 D1 又完成 `cutover → rollback → recutover`、切换前后失败／成功门禁和联合空目标恢复。完整双 Tenant 与 Money 完整 D1 消费链仍待补证。Party 必须既能作为主体管理产品单独使用，也能作为其他组合的显式依赖。C2 的 Business Engagement 默认只依赖 Project／Evidence；只有以后出现与项目客户不同且证据充分的独立主体角色时才直接接入 Party，不得在 Engagement 内建立第二套主体目录。

### 阶段 C：逐个增加纵向业务闭环

每次只选择一个证据最充分、能够端到端验收的缺口。候选包括经营承接、Billing／Invoice、项目包、第二生产类型、批复审计、成本外委、奖金和跨域只读投影。选择依据依次是：

1. 参考 OA 是否已有可追溯行为和异常样例；
2. 责任人、状态、纠错和历史是否足够明确；
3. 是否能形成用户可完成的纵向结果；
4. 是否能暴露当前应用边界或 Overlay 假设的真实风险；
5. 是否能独立验收而不要求同时铺开多个未知领域。

2026-09-30 已在公司 Ubuntu 隔离完整 D1 中用同一 Tenant／Project 连续通过 C0～C7：C2～C6 保留原有 Party、Evidence、Contract、Money、Production、Results 与 Cost 所有权；C7 又复用 Settlement、Receipt Set、Person／Assignment，以版本化 Overlay 完成两类奖金、正式建议 connector、Money 停机恢复、独立接受和来源撤回无级联。C8 随后完整执行：当前 Subject 工作、相邻 Subject 空结果、Money grant/revoke、Evidence／Production 顺序故障恢复、49/49 HTTP、十二库 canonical facts 不变和 17/17 health 均通过；但 C4～C6 输出没有 active Project Link／资料投影，Gateway 遗留 `evidence-service.view` 又阻断窄 `metadata.read + download`。因此阶段报告冻结为 `insufficient-evidence`；下一步只在原数据集关闭两项 P0 并重跑，不新建 Project，也不让第二客户掩盖失败。

C3 的第一版 Billing / Invoice 产品边界已经完成证据审计：它只登记已开票事实、Party／来源快照、Evidence 引用、revision 与历史，明确与 Money Flow 的义务、现金、核销和余额分离。第一版把系统登记纠正与现实红冲／作废／退票分开，拒绝复制参考 OA 宽 `FinancialRecord`、物理删除和一对一票款关联。独立模型、API Port、精确依赖引用、Nginx→Gateway→IAM→Billing 最终授权、故障语义、C3 只读组合和 D1 门槛已经形成技术契约；领域核心与 PostgreSQL 事实层也已通过真库生命周期、幂等、并发、依赖失败、跨 Tenant 和不可变保护。Project、Contract、Settlement 与 Money Obligation 的目标端兼容缺口已按事实所有者边界关闭；Billing 消费侧补齐工作负载 Policy、逐引用精确 Credential、类型化 resolver、严格响应复验和恢复幂等。完整生命周期 HTTP、可运行进程、Linux AMD64 非 root 镜像、pull-only release、Suite Manifest、正式 IAM／Gateway 入口、故障恢复与联合空目标还原已经组合验证。Billing 人类工作区也已进入 Workbench，并通过服务端动作收敛、金额撤权／恢复、历史脱敏、盲写金额拒绝、409 和依赖故障恢复。Project 的 C3 经营链路随后以独立人类金额 Capability 和逐资源工作负载凭证并行组合 Contract、Money Obligation／Transaction 与 Billing Invoice Registration；本地 D1 100 个 HTTP 断言、三角色、撤权／停用、四类故障恢复和 13/220 ms p50/p95 已通过。2026-09-30 的连续 C3 又在公司 Ubuntu 完整隔离栈关闭 Party 显式迁移、Billing／Money／Project 同数据旅程、金额脱敏和 Billing 局部故障恢复，40 条 HTTP 断言零差异且结束 17/17 healthy。当前仍缺 C3 双 Tenant、真实浏览器最终点击与 Billing-only 组合；记录见 [Billing 生命周期与运行时实验](./reports/oa-c3-billing-lifecycle-runtime-experiment-20260927.md)、[Billing D1 交付与恢复实验](./reports/oa-c3-billing-d1-delivery-recovery-experiment-20260927.md)、[Billing 人类工作流实验](./reports/oa-c3-billing-human-workflow-experiment-20260927.md)、[Project 经营链路 D1 实验](./reports/oa-c3-commercial-summary-d1-experiment-20260927.md)和[连续 C3 验证](./reports/oa-full-suite-continuous-c3-validation-20260930.md)。

后续增量已补齐 Billing 的列表、纠正、登记无效、Evidence 管理、历史、精确工作负载引用、可配置进程和 D1 正式装配；真实 PostgreSQL 18 完成 revision 1→4 与无效后有效金额归零，正式入口完成原生 IAM 用户授权、逐引用工作负载审计、Party 503 零写入／恢复重试以及联合空目标还原。Project 的 C3 分段只读组合和首组完整角色／撤权／账号停用／来源故障矩阵也已通过；C3 因双 Tenant、Settlement／应开未开等扩展、真实浏览器最终点击和真实环境仍缺证据，总判定继续为 `insufficient-evidence`。下一业务切片转向 C4 多类型生产，同时把上述 C3 缺口保留为独立补证轴。

C4 静态审计已经完成并接受目标边界：现有 Design Operations 以单一事实源兼容演进为 Production Operations，类型差异由代码拥有、版本化、逐类型验收的定义承载；`consultation-v1` 是第一条增量。C4-A01～A14 当前本地轮次已通过，并取得 canonical 权限、consultation-only 最小组合、Level A／B 回归、双向未选缺席，以及隔离 D1 的 Suite Manifest／Workbench、Nginx→Gateway→IAM 主链、IAM／Evidence 故障恢复和联合空目标恢复证据。86 条权限生命周期断言与真实浏览器又已补齐 Production Grant 撤销／恢复、账号停用／启用、Assignment 结束与页面动作收敛。双 Tenant 与真实 Ubuntu 仍单独补证。

C5 静态边界审计已经完成：参考 OA 的页面、后端和完成计算对审计金额来源、覆盖原因、状态和完成语义存在冲突，不能直接移植。目标采用独立 Approval & Audit Results，批复冻结精确 Contract／Settlement revision，审计冻结精确批复 revision，纠正／撤回只追加历史；Project 用版本化 Completion Profile 纯读取评估，GET 不写状态。C5-A01～A15 已固定，领域核心与 PostgreSQL 18 真库证明两种结果、整数金额、精确来源、覆盖原因、幂等／并发、追加历史、防篡改和依赖失败零写入；版本化 HTTP、目标 IAM 最终授权和 Project／Contract／Evidence 精确 adapters 又通过 401／403／409／422／503、失败零写入和原 Command 恢复。第三轮补齐 Results 人类工作区与 Project `reference-oa-c5-v1` Completion Profile；第四轮把 Results 装入正式 D1 并通过入口、未选缺席和联合空目标恢复；第五轮以 74 个正式入口断言完成真实 Contract／Evidence→approval→audit→Project 确认、纠正 Grant 撤销／恢复、依赖停机、来源纠正 drift 和 Results unavailable 幂等边界；第六轮用私有原 OA 浏览器 11／11 验证真实双列操作、四份 Evidence、未知回执重放、纠正／撤回重登记、缓存请求上界、正式 IAM 撤权和六所有者空目标恢复，并修复 Owner 空 Evidence 集合输出 `null` 的契约缺陷。当前状态为 `private-original-browser-supported / main-d1-dual-tenant-role-matrix-pending`；该浏览器轮没有执行 Project Completion Checkpoint。Main／公网、正式 D1／keys、完整双 Tenant、全角色、账号停用和第二客户继续作为独立补证轴。详见 [C5 责任边界审计](./reports/oa-c5-approval-audit-boundary-audit-20260928.md)、[C5 领域核心实验](./reports/oa-c5-results-core-experiment-20260928.md)、[C5 受保护 HTTP 与精确引用实验](./reports/oa-c5-protected-http-reference-experiment-20260928.md)、[C5 人类工作区和完成检查点实验](./reports/oa-c5-human-completion-profile-experiment-20260928.md)、[C5 D1 正式装配和恢复实验](./reports/oa-c5-d1-assembly-recovery-experiment-20260928.md)、[C5 真实独立进程旅程实验](./reports/oa-c5-real-process-journey-experiment-20260928.md)与[原 OA 批复审计私有浏览器报告](./reports/oa-approval-audit-native-validation-20261006.md)。

C6 静态边界审计已经完成：参考 OA 的 `FinancialRecord` 同时容纳奖金、成本、客户付款、票据、专家费和外协付款，使用浮点金额、可编辑删除；`ExternalCommissionService` 又直接创建付款／票据，并在删除外协时按供应商名称兜底删除财务记录。目标拒绝照搬，接受一个独立 Cost & Outsourcing 应用，并在同一部署边界中保留 Cost Entry 与 Outsourcing Engagement 两个严格聚合。Contract、Money、Billing、Party、Evidence 保持原所有权。C6-A01～A18 已固定；前四轮已完成领域真库、受保护后端、工作区／独立进程和正式 D1；Party 专项完成 Contract `cutover → rollback → recutover` 与恢复。第五轮 Payable Suggestion 现以两个真实进程／数据库完成精确来源提交、Money 独立接受／拒绝、接受创建唯一 Money Obligation、拒绝零 Obligation、目标停机／恢复、幂等重放、历史防篡改和部分失败恢复。当前判定为 `real-cross-process-payable-suggestion-supported / formal-d1-complete-dual-tenant-real-ubuntu-pending`。下一主业务轮在重新选择后进入 C7；C6 的正式 D1 connector、跨进程撤回、Money／Billing 只读投影和 Project 完成适用性不混入 C7。详见 [C6 责任边界审计](./reports/oa-c6-cost-outsourcing-boundary-audit-20260928.md)、[C6→Money 应付建议实验](./reports/oa-c6-payable-suggestion-experiment-20260928.md)与[Party 运行时切换实验](./reports/oa-party-ledger-runtime-cutover-experiment-20260928.md)。

C7 已完成六轮实现收缩：独立领域／PostgreSQL、版本化 HTTP／正式 IAM、真实 People／Money 来源、人类工作区、正式 D1，以及 C7→Money Payable Suggestion。第六轮以一个 PostgreSQL 容器、Bonus 与 Money 两个独立进程／数据库完成 11 条 HTTP 断言：Money 停机恢复、接受生成唯一义务、拒绝零义务、重放幂等、exact Person→Party 人工映射防篡改和源撤回无级联。当前判定 `real-cross-process-payable-suggestion-supported / formal-d1-connector-suggestion-recovery-pending`；下一业务轮转向 C8，不混入税务、薪酬或 Project Completion。详见 [C7→Money 应付建议实验](./reports/oa-c7-money-payable-suggestion-experiment-20260929.md)与 [C7 D1 正式装配与恢复实验](./reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)。

C8 已完成静态责任边界并接受 BF-073。Project 只按 Link 组合，Evidence 拥有 metadata／version／download，业务应用发布类型化指标／工作项，Workbench 只按当前 Subject 聚合；不建共享报表库、跨库联表或统一可写待办。连续 C8 已在公司 D1 运行并形成明确反证：个人工作、Money 分权、局部降级和事实不变可行，但 C4～C6 Link／投影与 Evidence Gateway download 分权未闭合。当前判定为 `c8-runtime-partially-supported / assembly-gates-failed`。详见 [连续 C8 验证](./reports/oa-full-suite-continuous-c8-validation-20260930.md)和[阶段总报告](./reports/oa-full-scope-architecture-validation-report.md)。

同日 BF-074 完成 C8 个人工作的首个来源级实现：Production Operations 发布 current-only `WorkItemProjection v1`，稳定标识、来源 revision、Project Scope、深链和服务端 `allowed_actions` 均由来源拥有；只把当前 Credential Subject 作为 direct Person `production_owner` 的未完成工作项返回。Workbench 不接收 Subject 输入，也不持有 workload 身份替人授权，而是在浏览器中经 Gateway 获取该人员对来源的短凭证，并由 path-scoped HttpOnly Cookie 访问投影。结束 Assignment、相邻人员、动作撤销、来源拒绝／不可用、空结果和逐来源降级已经零容器串行单元／契约验证。当前判定推进到 `c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`；不把它写成正式入口、真实浏览器、正式 D1、跨来源统计或完整 OA 已通过。详见 [C8 Production 工作项与 Workbench 单元验证](./reports/oa-c8-production-work-item-workbench-unit-validation-20260929.md)。

同日 BF-075 完成 C8 统计的首个来源级实现：Money Flow 从自身 current Operating Summary 发布六个 `MetricProjection v1` 项，保留 Tenant／Ledger Owner Party／Currency Scope、精确十进制、口径版本、观测时间、摘要和深链。指标结构与金额能力独立；无金额权时 `value`／`source_digest` 同时为空且 `redacted=true`，来源失败为 503。Money 全模块和 Access Contract 零容器串行通过。随后 BF-076 用 Suite Manifest 可选 `metric_projection` 把这一来源装入 Workbench：严格校验 source／Tenant／Scope／精确值或 redaction，按来源分区且不跨来源合计，单一来源失败只降级自身。Workbench、Gateway Manifest 与 Access Contract 同样零容器串行通过。当前判定推进到 `c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending`；真实 Gateway／IAM／Money、浏览器、第二来源运行时、正式 D1 和跨来源公式均未由本批证明。详见 [C8 Money 指标来源单元验证](./reports/oa-c8-money-metric-projection-unit-validation-20260929.md)与[C8 Money 指标 Workbench 消费者单元验证](./reports/oa-c8-money-metric-workbench-consumer-unit-validation-20260929.md)。

### 阶段 D：形成 OA 完整组合

当 C1～C8 的必要责任域具备后，使用版本化 Suite Manifest 和 Customer Overlay 装入完整参考 OA。统一 Workbench 提供连续操作体验，但不拥有业务事实。组合验收包含：

- 从空环境运行完整旅程并幂等重放；
- 使用多 Subject、多角色、多 Project 的权限矩阵运行同一旅程，并逐项保存允许与拒绝证据；
- 每个来源逐项停机和恢复；
- 至少两个不同粒度的可裁剪子集；
- Overlay 移除与重新装入；
- Base 升级、应用 migration、备份恢复和历史解释；
- 六个独立 Demo 和既有 J1～J4 全量回归。

### 阶段 E：提升到真实环境

复用阶段 D 验证过的同一镜像摘要、migration、Suite Manifest 和 Overlay，不在目标机重新编译或修改业务代码。真实环境至少验证：

- `Browser → Nginx → Gateway → IAM／目标应用`；
- HTTPS、HTTP 跳转、内部端口不可公网访问；
- 登录、Capability、服务端最终授权和身份 Header 清理；
- 项目级资源范围、字段／金额／文件动作、撤权残余窗口和 IAM／Policy 故障行为；
- OSS／本地 Evidence 的目标配置与文件完整性；
- 应用开关、单应用故障、升级前备份、空目标恢复和恢复后继续写入；
- 目标环境的域名、证书、秘密、Registry、数据库和备份独立所有。

### 阶段 F：出具架构可行性验证报告

最终使用 [OA 完整体架构可行性验证报告模板](./templates/oa-full-scope-architecture-validation-report.md) 生成 [OA 完整体架构可行性验证报告](./reports/oa-full-scope-architecture-validation-report.md)。阶段一 `1.0-stage1-final` 已冻结为 `insufficient-evidence`，并明确区分已通过切片、支持方向的证据、C8 反证和 P0 恢复门槛；后续修复不得覆盖这份失败证据，只能追加重跑报告。

## 5. 每个纵向闭环的进入与退出门槛

进入实现前必须具备：

- 一句清楚的 A→B；
- 事实所有者和不负责范围；
- 正常、纠错、撤销／反向和并发行为；
- 目标应用与直接依赖；
- Capability、资源 Scope、策略／业务事实输入和最终授权者；
- 可执行验收数据和失败条件；
- 对 BF-001～BF-039 的影响说明。

退出时必须具备：

- 单元、真库、契约和独立启动证据；
- 人类可读页面关键路径与错误恢复；
- 组合幂等、依赖故障和恢复证据；
- 至少一个允许、相邻角色拒绝、跨资源拒绝、状态拒绝和撤权后拒绝的权限证据；
- 变化账目及反证检查；
- 既有 Level A、Level B 和相关业务回归；
- 文档、实现与当前状态一致。

## 6. 阶段总门禁

只有同时满足以下条件，报告才可以给出 `supported`：

- C1～C8 的已确认范围全部为 `passed` 或有证据支持的 `not-applicable`；
- 最小组合与完整组合均能从空状态运行、重放、备份和恢复；
- 应用数据所有权、公开契约、IAM、Evidence 和历史边界没有被破坏；
- C1～C8 已实现范围的角色、资源、字段和动作矩阵通过，且跨租户、伪造身份、万能管理员旁路和客户端绕过数量为 `0`；
- OA 差异没有形成共享业务表、客户核心 fork 或无法升级的现场补丁；
- 至少一个真实 Ubuntu 目标环境通过完整入口和运维验收；
- 最终报告的所有主张都有直接证据，限制与未知项被明确列出。

任一核心边界反证都会把结论降为 `not-supported`；核心业务成立但存在有界条件时为 `conditionally-supported`；缺少真实环境或关键旅程证据时为 `insufficient-evidence`。

## 7. 当前状态与近期工作面

自 2026-10-06 起，完整 OA 的每个闭环同时更新[小系统边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。页面可达、前后端打通、服务分离和静态审计都不能单独证明边界稳定；退出证据必须说明事实所有者、依赖类别、失败续办、旧版本兼容与下一证伪条件。Project、Production、Contract 当前均只判定为候选边界。

除单切片结果外，还必须比较相邻切片的收敛趋势：改变的事实 Owner 数、公开契约／Capability／migration 数、新增强制依赖和逐请求同步调用、需要重跑或迁移的旧场景，以及变化落在 Base 还是 Overlay／配置／策略／连接器／只读投影。指标不设脱离业务的单次硬阈值，但长期不下降、持续跨 Owner 扩散或反复改写旧版本时，不得给出边界成熟或整体架构通过结论。

- [x] 六个小系统 Level A 独立运行门禁；
- [x] J1～J4 高保真组合、幂等重放与五来源故障隔离；
- [x] 首份六层定制变化账与目标无关可移植包；
- [x] OA C1～C8 业务关系和当前缺口审计；
- [x] 六个 Demo 的实际页面达到已接受体验基线；
- [x] IAM／Gateway／Contract／Money 的基础凭证、入口攻击与动作级权限回归；
- [x] C1 已实现项目责任范围的九角色、双 Project、状态拒绝、Assignment 撤权和身份管理员反向隔离矩阵；
- [x] C1 两级 Project Package、客户页面、`allowed_actions`、安全撤销和撤权恢复的 152 条正式入口断言；
- [x] C2 经营承接责任边界审计：接受独立 Business Engagement 目标，保留未中标／流标／多轮／自动建合同等未知；
- [x] OMS 作为第二参考产品形成 evidence-ready 六层账；Party 共性加强，Billing／Invoice 成为 Party 后优先证据审计候选；
- [x] Party Directory 独立应用核心、真库、release、非 root 镜像、空目标恢复及首个 Project 消费边界；
- [x] Party Directory → Project D1 正式 API 链、IAM 精确工作负载授权、依赖停机／恢复与联合空目标还原；
- [x] Party 管理／Project 客户选择人类工作流、用户发现凭证与服务端 workload 精确复验分层；
- [x] Contract／Money Party 确定性只读迁移预检；
- [x] C3 Project／Contract／Settlement／Money Obligation 目标端精确 reference-read、稳定摘要与相邻资源拒绝；
- [x] C4 多生产类型目标责任边界、`consultation-v1` 第一增量和 C4-A01～A14 验收矩阵静态审计；
- [x] C4-A01～A09：定义注册、完整 fixture 真库升级、咨询创建、类型门禁、双类型隔离、负责人、适用性、成果替换、完成／重开和失败恢复；
- [x] C4-A10 本地部分：Production Operations canonical API／双 audience／Capability／Overlay／Assignment Policy、精确 Scope 与真库 HTTP；
- [x] C4-A11：People／Project／Evidence／Production consultation-only 最小组合空库闭环、Contract／Money 缺席和稳定复跑；
- [x] C4-A13：`make check`、六应用 Level A、完整 OA Level B 双重放和五类依赖故障隔离聚合回归；
- [x] C4-A10 正式入口主矩阵及 A12：精确 Project 签发、相邻角色／Project 拒绝、IAM／Evidence 故障恢复、完整未选应用缺席、Suite Manifest／Workbench 与联合空目标恢复；
- [x] C4 权限生命周期补证：Production 专项 Grant 撤销／恢复、Assignment 结束、账号停用／启用、`allowed_actions` 收敛与真实浏览器恢复；
- [x] C5 批复审计与 Project 完成责任边界、来源矛盾审计及 C5-A01～A15 验收矩阵；
- [x] C5 独立领域模型、PostgreSQL 真库生命周期、批复／审计精确来源核心、幂等并发与追加纠错；
- [x] C5 版本化 HTTP／契约、IAM 与 Project／Contract／Evidence 精确 adapter 的同进程 HTTP／真库验证；
- [x] C5 Results 人类页面、Project Completion Profile 只读评估、显式检查点确认与来源变化 drift；
- [x] C5 正式 D1 装配恢复与未选缺席；
- [x] C6 正式 D1 装配、正式入口、未选缺席与联合空目标恢复；
- [x] Contract／Money Party 显式运行时切换、回滚与含引用数据验收；
- [x] C6 Payable Suggestion → Money 独立确认（真实双进程及连续完整 D1 正式 connector 均通过；双 Tenant、跨进程撤回与真人浏览器仍待补证）；
- [x] C7 奖金责任边界、类型化来源、版本化 Overlay、整数金额、追加历史和 C7-A01～A18；
- [x] C5 真实 Contract／Evidence→approval→audit→Project 检查点确认、局部故障恢复和纠正 Grant 生命周期；
- [x] C5 在同一 Tenant／Project 连续通过真实跨进程批复→审计→Project 显式完成确认、金额脱敏、权限撤销／恢复、来源 drift、三类依赖故障和完成态重跑；
- [ ] C4 环境补证：双 Tenant 与真实 Ubuntu；C4-A14 持续反万能平台检查；
- [ ] Contract／Money Party 兼容映射、显式切换／回滚与完整双 Tenant 故障矩阵；
- [ ] Business Engagement 第一版完整验收；模型、migration、API、细粒度权限、Project／Evidence Port、真库、release、工作负载凭证、Suite Manifest、Workbench 页面、浏览器撤权刷新、C2 正式入口权限矩阵、本地 D1 空目标恢复／继续写入、409／503 自动化与真实浏览器恢复，以及账号停用、IAM／Policy 故障恢复已通过；下游建议、真实 Ubuntu 和完整双租户矩阵待完成；
- [x] C7 独立领域核心、PostgreSQL 18、精确 Overlay、追加历史、并发幂等与核心故障恢复；
- [x] C7 受保护 HTTP／IAM、消费 adapters、服务端金额脱敏及后端故障恢复；
- [x] C7 真实 People／Money 来源端点、六进程独立组合、人类页面、直接纠错／撤回、依赖故障零写入与恢复、真实浏览器；
- [x] C7 正式 D1／未选装／联合空目标恢复与恢复后继续写入；
- [x] C7→Money 直接跨进程建议：停机恢复、独立接受／拒绝、exact Person→Party 映射、幂等与源撤回无级联；
- [ ] C8 P0 收敛：公司 Ubuntu 同一 Tenant／Project 已连续通过 C0～C7，并完整执行 C8；当前只剩 C4～C6 active Project Link／资料投影、Evidence Gateway 独立 download 入口和修正摘要真人复验。C8 的工作项、Money 分权、顺序来源恢复、十二库事实不变及 17/17 health 已通过；
- [ ] C2 已有角色／资源／状态／撤权矩阵，账号停用、IAM／Policy 故障恢复与目标错租户拒绝已通过；完整双租户 D1、组织／代理、字段／金额／更广文件动作以及 C3～C8 多角色矩阵待完成；
- [ ] 公司私服或个人 ICP 路径至少一条通过真实环境验收；
- [x] 第一阶段架构可行性验证报告；`1.0-stage1-final` 已以 `insufficient-evidence` 冻结，P0 修复后另行追加复验版本，不回写历史失败。

下一轮正式参考应用保持 `TBD`。OMS 当前只作为第二参考产品补充 OA 完整体的共性、候选差异和未知项，不等于下一轮正式验证对象，更不等于第二真实客户。
