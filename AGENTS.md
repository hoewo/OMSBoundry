# OMSBizFoundry 开发上下文

本文件适用于仓库根目录和全部子目录。子目录可以增加更严格的局部规则，但不能放宽这里的边界。

## 开始工作前

按最小必要范围读取：

1. `README.md`
2. `docs/decisions.md`
3. `docs/foundation.md`
4. `docs/development-plan.md`
5. `docs/oa-full-scope-validation-plan.md`
6. `docs/next-stage-action-plan.md`
7. `docs/architecture.md`
8. `docs/testing.md`
9. 与当前事项有关的 ADR 和应用目录

## 产品边界

- 本项目不建设通用业务平台。
- 当前已经实现 `contract-ledger`、`money-flow-ledger`、People & Groups、Evidence Service、Party Directory、Project Workspace、Business Engagement、Billing、Production、Results、Cost & Outsourcing 及相应 Nginx、Gateway、Identity & Access 和 Unified Workbench 装配；Bonus Distribution 已完成独立领域核心与 PostgreSQL 18 事实层，但 HTTP／IAM、页面、D1、恢复与 Money 建议尚未实现。
- Contract Ledger、Money Flow Ledger、Project Workspace、Party Directory、Business Engagement 与 People & Groups 必须保持各自数据所有权并可独立迁移、测试、部署和使用；“独立”允许交付包携带显式声明的直接依赖，不等于单进程或零依赖。Evidence Service、Party Directory 与 Business Engagement 均通过版本化端口接入。
- Project、Approval、客户 OA 和其他未声明应用都不是启动前提。应用没有永久的“核心／辅助”商业等级：某应用可以在一个客户组合中承载主要业务结果，在另一个组合中作为显式依赖，或完全不选装；Party Directory 可以单独作为主体管理产品，也可以作为 Contract、Money 或 Project 的已声明依赖随包交付。角色变化不得改变数据所有权，也不得以共享表或隐式同步实现。
- 所有受保护访问必须经过版本化 IAM 契约；正式 IAM、本地模板和测试提供者只能替换实现，不能关闭鉴权或绕过可信 Actor。
- Nginx、Gateway、统一工作台和部署运维包是正式装配组件，不拥有业务事实；完整组合固定使用 Nginx 公网边缘入口和其后的内部 Gateway；“基础”当前只确认独立 Evidence／File Service，不建立万能公共服务。
- 单台 ECS 是完整组合必须支持的最低部署档，不是架构上限。Suite Manifest 决定启用哪些应用，版本化 Deployment Profile 决定节点放置、副本和数据设施；D1 一台、D2 应用／数据两台、D3 边缘／应用／数据三台、D4 高可用多节点使用同一镜像、API、migration 和数据边界。
- 应用端点必须通过配置注入，不得用 `localhost`、固定 IP 或机器名绑定某个部署档；两台或三台只表示隔离，没有冗余入口、应用副本和高可用数据层不得宣称 HA。
- IAM 拥有租户、账号、登录、邀请／激活、会话和能力授权；People & Groups 拥有人员、Group、Membership、组内角色和能力画像，两者不得合并密码与人员名册的所有权。
- Party Directory 目标态拥有主体当前档案、稳定 ID 与 revision；消费应用拥有该主体在本业务中的角色和业务发生时不可变快照。当前两个台账的本地 Party 目录仍是实现事实，完成独立服务和迁移验收前不得假称已统一。
- Business Engagement 目标态独立拥有经营承接适用性、承接方式、材料业务引用、revision／历史和下游建议交付状态；它引用 Project、Party、Evidence 和 IAM，但不得拥有项目、主体、文件、合同或资金事实，`ready` 只表示材料齐备，不表示中标或签约。
- 第一版 Gateway 使用独立、小型 Go 服务和版本化静态路由，可以暴露登录路由、执行限流和基础验签，但不得保存账号凭证、签发业务身份或替业务应用作最终授权；所有客户端身份 Header 必须先删除，再从已验证的签名上下文派生。
- 文件上传下载由独立 Evidence Service 负责；业务应用只保存 File ID／Version 与业务含义，不嵌入 OSS SDK、长期密钥、分片上传、签名 URL 或物理对象清理。阿里云 OSS 使用短期有界 STS、浏览器直传、服务端完成校验和短期签名访问地址。
- 客户界面的施工队、项目组或服务小组只能映射为 Group 类型或显示术语；具体项目任务、合同范围、预算、工资和应付款不得进入 People & Groups 内核。
- Money Flow 只拥有已确认义务、实际资金、核销和余额；报销、奖金等来源应用拥有申请、计算和审批，只能通过来源快照与义务建议连接台账。
- Bonus Distribution 独立拥有经营／生产奖池、策略与输入快照、人员分配和发布／取代／撤回历史；经营来源只读 Contract confirmed Settlement，生产来源只读 Money confirmed receipt set，发布结果只能向 Money 提出版本化建议。公共核心使用整数最小货币单位、整数基点和显式舍入，客户公式只存在于版本化 Customer Overlay。
- 客户岗位、行业专用字段和一次性流程不得进入稳定内核。
- 跨客户交付以“版本化稳定 Base + Customer Overlay”作为验证方向：客户差异优先增加 Suite Manifest、配置、版本化策略、类型化扩展、连接器、只读投影和隔离专项，不覆盖公共内核、共享业务表或长期 fork 核心代码；重复稳定语义和公共缺陷必须兼容演进 Base。当前方向尚未获得跨客户长期升级证据，不得描述为已经验证成功。
- 当前下一阶段主线是参考 OA 完整体架构可行性验证：本地逐步完成 C1～C8、最小组合和 Customer Overlay 证据，公司私有 Ubuntu 或 ICP 生效后的个人试验田至少一条完成真实入口与恢复验收；最终以验证报告结束阶段。环境等待不阻塞本地工作，但缺少真实环境证据不得给出整体通过结论；下一轮参考应用保持待选。
- 2026-10-06 用户确认：本节是当前实现完成度的最高优先级摘要。后续未进入干净可追溯基线的代码、报告、验收脚本和运行记录只作为中断实验或候选证据；与本节冲突的“已实施／已通过”状态不得继续外推。架构和所有权约束仍可保留，完成状态必须按 BF-120 和 [`docs/status/current-baseline-and-next-actions-20261006.md`](./docs/status/current-baseline-and-next-actions-20261006.md)重新审计、复验并登记。
- 2026-10-07 用户调序（BF-121）：紧邻工作优先统计 UI／完整交互与原 OA 属性对齐的四层权限配置展示，项目详情问题延后。统计只读投影、浏览器权限草稿和本地 UI 验证不代表正式 IAM 配置、C7／C8 或真实环境已完成；范围见 [`docs/specs/assembly/oa-statistics-permission-ui-v1.md`](./docs/specs/assembly/oa-statistics-permission-ui-v1.md)。
- 2026-10-08 用户确认 OMS 可先做参考版本（BF-122）：当前唯一确认的产品事实是主体为 OA 业务链中的“资质单位”。第一版 `/oms/` 只能作为 Gateway 到 Workbench 只读预览的装配入口，使用演示数据验证信息结构和交互；不得据此新建业务事实所有者、数据库、Grant、真实命令，或宣称正式 OMS 产品设计和 OA 整体验证已完成。范围见 [`docs/specs/assembly/oms-qualification-unit-demo-v1.md`](./docs/specs/assembly/oms-qualification-unit-demo-v1.md)。
- 2026-10-10 用户确认先要能快速看懂产品的最小原型（BF-123）：OMS 参考页用一份虚构合同贯穿经营总览、合同全景、开票与回款、下游结算四个视角，并在关键节点暴露候选含义和待确认问题。该原型仍完全服从 BF-122 的只读边界；本地完成不等于 `oatest` 已更新，更不等于正式 OMS 产品、真实业务规则或后端已完成。范围见 [`docs/specs/assembly/oms-contract-funds-validation-prd-v0.1.md`](./docs/specs/assembly/oms-contract-funds-validation-prd-v0.1.md)。
- 2026-10-10 用户要求快速看到 OMS 0.2 的产品全貌（BF-124）：本地单文件原型以“今日工作、经营事项、主体库”三个入口覆盖双业务链、资料待办、变更历史、两笔虚构案例和浏览器内模拟回款。这里的“全貌”只表示关键产品表面均可进入，不表示真实规则、持久化、Owner API、IAM、文件、部署或正式闭环完成；范围见 [`acceptance/oms-validation-mvp-v0.2/index.html`](./acceptance/oms-validation-mvp-v0.2/index.html)。
- 2026-10-10 用户要求入口少但必须符合产品本身（BF-125）：0.2 一级入口修正为“待办事项、经营台账、协作单位”。经营台账容纳纵览、事项和单事项双链；协作单位容纳施工单位／班组、负责人、范围、预算、名册和模板。合同、票款、结算、资料、历史与模板不单设侧栏入口；新增入口必须证明其高频、独立且跨事项。BF-125 只修正信息架构，不改变 BF-124 的模拟、无真实后端边界。
- 2026-10-10 用户明确 0.2 由产品侧主导（BF-126）：最终一级入口只有“待办事项、经营事项”；经营事项内设经营总览、全部事项、协作资料。台账、施工单位／班组、范围、预算、名册和模板完整保留但不占一级入口；用户意见作为产品判断输入，不得被机械翻译成新增导航。该修正仍不改变模拟数据、无真实后端的边界。
- 2026-10-10 用户确认资质单位向上游单位开票，并提出收付循环是直接痛点（BF-127）：单事项首要工作面改为“收付循环”，显示缺口、推进动作、事实登记、核销和下一轮待办。循环目前仍是待验证产品假设，不表示回款自动触发付款；应收、回款、应付、授权和付款继续独立记录。
- 2026-10-10 用户要求完成收付循环的最小可操作验证（BF-128）：本地 0.2.1 单文件原型把登记回款、付款授权、实际付款拆为三个独立模拟动作，并同步更新余额、待办、时间线、变更历史和跨事项台账。该验证只证明产品交互能表达独立事实和循环推进，不表示真实审批、付款规则、持久化、正式 IAM、Owner API 或业务闭环已完成。
- 2026-10-10 用户要求继续下一个小目标（BF-129）：本地 0.2.2 增加“记录回款跟进”，把逾期金额拆为已安排预计回款与未安排逾期，并联动待办、循环摘要、时间线、历史、应收台账和预计现金流；预计安排不覆盖逾期事实、不生成实际回款，到账后才单独兑现。该交互仍是浏览器模拟，不确认真实催收／延期权限、凭证、规则或正式实现。
- 2026-10-10 用户要求继续下一个小目标（BF-130）：本地 0.2.3 在应付与付款授权之间增加独立的“付款条件判断”，支持“已核对可授权”和“暂缓待复核”两种候选结果，并联动待办、收付循环、时间线、历史、应付台账和预计现金流。判断不改变应付、不产生授权或实际付款；只有通过判断后才展示独立授权动作，授权后仍须另行登记实际付款。该交互仍是浏览器模拟，不确认真实判断依据、审批权限、付款前置条件或正式实现。
- 2026-10-10 用户要求继续下一个小目标（BF-131）：本地 0.2.4 将“待办事项”从静态汇总改为可执行工作面；每项显示类型、处理时点、触发原因和唯一动作，点击后直接定位对应事项并打开回款跟进／付款判断等模拟表单，资料任务则进入事项资料缺口。动作完成后待办按新的浏览器状态重排。该实现不新增一级入口、通用流程或通知系统，不确认真实任务生成、时限、责任人和权限规则。
- 2026-10-12 OMS 一期公网发布通过（BF-149）：`https://oatest.antuinfo.cn/oms-v1/` 已按 `Nginx → Gateway → IAM → Workbench／Owner` 链路运行，升级前备份、17 个长运行容器健康、D1 全栈检查、正式登录、五 Owner 授权与读取均通过；旧 `/oms/` 参考入口保持。资质单位由 Party Directory 公网原生命令建立并以稳定 ID 配置，不直接改业务库。该状态只证明公网装配与只读组合；Money 实际资金写入、Evidence 附件、施工方应付旅程、全量导出和全部业务闭环仍未完成，范围与证据见 [`docs/reports/oms-phase1-mvp-local-validation-20261012.md`](./docs/reports/oms-phase1-mvp-local-validation-20261012.md)。
- 2026-10-17 用户确认 OMS 一期成为主产品入口（BF-159）：`https://oatest.antuinfo.cn/`、登录默认回跳和 `/oms-v1/` 直接进入服务端首屏 OMS，不再经过旧 OA 侧栏或加载其全量运行时。项目立项、旧项目管理、资料、统计和 OMS 参考版统一收进 `/legacy/`“旧版工作台”，旧版保留显式返回 OMS 的入口。该调整只改变 Gateway／Workbench 装配、导航与首屏资源边界，不改变 IAM、事实 Owner、API、schema、Grant 或业务数据。
- 2026-10-17 OMS 一期短期访问凭证续期兼容已发布（BF-160）：浏览器使用 `fetch(..., redirect: 'manual')` 读取受保护 Owner 时，Gateway 的登录／换票重定向可能被浏览器表示为 `opaqueredirect` 且 `status=0`，Workbench 必须把它与明确 3xx 一并识别为短期访问凭证失效，清除对应页面授权缓存、重新换取该 Audience 的 Capability，并只重放原读取一次。不得把这类响应当作 Owner 空集合、清空已有业务投影或对普通 401／403、校验错误和业务冲突自动重试。公网 Workbench `v0.1.0-test.oms-v1.16` 已发布，Owner 只读复验返回 Project 30、Party 13、Billing 3、Money 5、Contract workspace 6；该修复不改变 IAM、Gateway、Owner、API、schema、Grant 或业务数据。

- 2026-10-18 用户要求只清除 OMS 一期数据并重新走录入流程（BF-161）：测试环境先生成完整可恢复备份 `20261018T041742Z`，再把 Contract、Money、Party、Project、Evidence、Billing 六个既有 Owner 恢复到首笔 OMS 数据之前的 `20261012T175912Z` 检查点，并重建 Party runtime 映射；IAM、账号、Grant、资质单位、其他应用和旧 OA 数据不清除。OMS 页面只投影 `project_type_keys` 含 `operations` 的项目，以及能解析到这些项目的 Billing／Money 记录；Owner 中保留的旧 OA 事实不得显示为 OMS 数据。公网 Workbench `v0.1.0-test.oms-v1.17` 已发布，清理后 OMS 项目为 0，D1 检查、17/17 容器健康和公网只读验证通过。该动作是测试环境运维清理，不新增产品内批量删除能力，也不改变 Owner、API、schema、Grant 或数据所有权。
- 2026-10-19 OMS 一期自交易业务关系防护已发布（BF-162）：Customer Overlay 配置的资质单位是当前经营主体，不得同时作为同一 OMS 项目的外部甲方或施工方。Workbench 必须从新建项目、关联施工方、双向票据和收付款的候选中排除该稳定 Party ID，并在提交前再次验证；该规则是 OMS 装配角色约束，不是 Party Directory 全局主体规则。已建立的错误项目保留原始快照并显式提示重建，不允许 Workbench 静默改库或改写不可变甲方历史。公网 Workbench `v0.1.0-test.oms-v1.18` 已发布，范围和证据见 [`docs/reports/oms-phase1-self-counterparty-guard-validation-20261019.md`](./docs/reports/oms-phase1-self-counterparty-guard-validation-20261019.md)。
- 2026-10-19 客户交付仓库结构已确认（BF-163）：`OMSBoundry` 以 `frontend/oms-workbench`、`backend/platform/*` 和 `backend/services/*` 分离前端装配、平台入口与业务 Owner；`contracts/`、`packages/` 保留根目录以维持稳定契约和 Go module 兼容。目录整理不得合并 module、数据库或服务，不改变 Owner、API、schema、Grant、Suite Manifest、部署拓扑和业务事实所有权；提交前必须通过完整 Go／前端测试、架构审计、发布包测试和代表性镜像构建。

## 系统交互

- 每个应用拥有并写入自己的数据库。
- 禁止跨应用直接读写内部表、复用 ORM 模型或导入领域对象。
- 系统间业务流动必须通过版本化 API、快照或事件完成。
- 交互必须有幂等、失败恢复和联调测试。
- 连接器只能提出或转交业务动作，不能越权覆盖另一个系统已经确认的事实。
- IAM 管理身份、租户、角色与能力授权；业务应用拥有稳定能力目录并结合资源归属和当前业务状态作最终授权，不根据客户角色名或页面按钮位置写业务条件。
- 权限协作不是 `IAM → 业务应用 → 目标应用` 的逐请求同步链。目标应用发布 Capability Manifest，业务应用只提供版本化 Business Fact Snapshot，只有受信 IAM／Policy Provider 签发目标应用原生 Capability Credential，目标应用再返回并执行 `allowed_actions`。

## 开发方式

- 每个小应用、业务板块、功能、数据字段和技术机制先用一句好懂的话写清“问题 A → 可验证结果 B”；无法说明它服务哪个上层 A→B 的事项不进入核心。
- A→B 只确定方向；进入实现前还必须明确不能破坏的约束、验收证据和不负责的相邻问题。
- 一个功能一个功能完成；每个功能先明确事实、不变量和验收，再实现和验证。
- 优先交付纵向闭环，不先铺设公共平台。
- 允许少量局部重复；没有两个稳定使用点前不抽取公共业务代码。
- 已确认事实采用追加 revision、调整、冲销或反向记录，不静默覆盖历史。
- 金额必须使用精确表示，禁止二进制浮点数。
- 修改范围保持在当前里程碑；明确延后的内容不得顺手加入。

## 文档与验证

- `docs/decisions.md` 与 `docs/architecture.md` 共同投影当前 Architecture Baseline 0.1；后续实现不得静默改变应用职责、数据所有权、入口链、权限协作、Evidence 边界、Suite Manifest 或 Deployment Profiles。
- `docs/decisions.md` 是后续开发必须读取的唯一决策索引；任何规格、代码和评审不得违反其中状态为“已接受”的约束。
- 新讨论形成共识时，必须在同一变更中登记或取代稳定决策，并同步受影响的基线、架构、计划或验证文档；不得只写讨论稿后依赖记忆。
- 当前共识更新到 `docs/foundation.md`。
- 开发路线更新到 `docs/development-plan.md`。
- 稳定架构事实写入 `docs/architecture.md` 或 ADR。
- 实际验证入口写入 `docs/testing.md`。
- 部署相关变更必须验证其声明支持的 Deployment Profile；从低档迁移到高档时，ID、revision、摘要、审计、业务引用和应用数据所有权不得变化。
- 代码修改必须运行与风险相称的单元、集成和独立启动验证。
- 未实际运行的检查不得描述为通过。
