# 小系统群 Demo Validation 2

## 元信息

- 路径：`spec/assembly/demo-validation-2.md`
- 所属模块：`assembly`
- 状态：🟢 Level A、Level B、设计交付／商业台账两类子集装配、首个实际 OA 组合变化的定制化六层验证及当前高保真可移植开发包均已通过；完整入口链、真实公司 Ubuntu 和跨客户适配仍待独立验收
- 优先级：P0
- 依赖规格：`reference-oa-demo.md`、`customer-scenario-evidence.md`、`project-workspace/project-initiation.md`、`design-operations/design-production-delivery.md`

## 问题与结果

**问题 A**：现有六应用组合已经能够完成 J1～J4，但“应用可以单独启动”“自动测试通过”和“用户能够独立演示这个产品”仍是三件不同的事；若直接继续扩充组合 OA，就无法判断缺口来自单应用能力、装配方式还是错误的业务边界。

**可验证结果 B**：Contract Ledger、Money Flow Ledger、People & Groups、Evidence Service、Project Workspace 和 Design Operations 分别形成可重复的独立 Demo；全部独立 Demo 通过后，再用同一批业务应用组合参考 OA，并以可追溯覆盖、事实所有权和受控定制证据判断当前边界是否成立。

最终品牌、页面精修、Nginx、Gateway、中心 IAM、Workbench、公网域名和 TLS 不属于本规格的前置条件。独立 Demo 仍必须满足 `demo-experience-baseline.md` 的最小可用入口、任务导航、状态反馈和恢复要求；这些基础交互不是可延期的“美化”。完整入口能力不得被本地结果冒充为已通过。

## 两级验证顺序

### Level A：六个独立小系统 Demo

每个业务小系统作为一个独立产品被演示和验证。独立 Demo 只运行该应用自身拥有的数据库、对象目录和进程；相邻业务系统不是启动前提。

每个 Demo 都具备以下操作者能力：

1. 从全新状态启动应用及其自有持久化依赖；
2. 使用明确的本地模板 IAM Provider 进入应用，仍然执行 tenant、audience、action 和 scope 校验；
3. 构造一组可辨认、无真实客户信息的演示数据；
4. 完成下表定义的主闭环，并展示当前状态、历史和至少一个失败关闭结果；
5. 对同一演示命令安全重放，不生成第二套重复业务事实；
6. 自动核对关键结果，而不是只输出“请求成功”；
7. 清除 Demo 创建的容器、数据库卷、对象和本地状态，或者明确保留数据供人工查看；
8. 把操作者入口、访问方式、预期结果和已知限制写入同一份说明。

独立 Demo 使用真实应用代码、真实 migration 和真实数据库约束。合成数据只替代客户数据，不替代业务执行。依赖桩只允许用于 Project 和 Design 的公开 reference-read 边界，并必须同时支持成功、不存在和暂时不可用响应；桩不得直接写目标应用数据库，也不得替当前应用生成成功状态。

### Level B：参考 OA 组合 Demo

Level B 只有在六个 Level A Demo 全部通过后进入。它复用相同应用版本、API、migration、IAM 契约和数据所有权，不维护 OA 专用核心分支。

组合 Demo 的目标是尽可能恢复参考 OA 中有证据的业务结果，而不是逐页面、逐表或逐字段复制旧系统。每项候选 OA 能力都必须处于以下一种状态：

- `covered`：已有端到端演示证据；
- `partial`：稳定内核存在，但来源流程、项目语义或组合体验不完整；
- `accepted-next`：证据足够且责任位置明确，纳入下一组合切片；
- `needs-business-evidence`：生命周期、纠错或责任尚不清楚，不能猜测实现；
- `rejected`：会破坏数据所有权、引入万能平台或把客户专项伪装成公共内核；
- `deferred`：结论清楚但不影响当前阶段目标。

“尽可能恢复”表示持续处理 `partial` 和 `accepted-next`，直到剩余项都能解释为 `needs-business-evidence`、`rejected` 或 `deferred`；它不表示为了功能数量绕过边界。

## Level A 能力矩阵

| 小系统 | 独立 Demo 展示的业务结果 | 必须覆盖的能力簇 | 允许的最小外部替身 | 当前状态 |
| --- | --- | --- | --- | --- |
| Contract Ledger | 从主体和合同草稿形成可追溯的有效合同，并展示后续变更、结算、纠错或终止／关闭结果 | 主体；草稿与合同方；激活；补充／更正；结算；关系；终止或关闭；业务导出；审计 | 本地模板 IAM；文件能力暂按现有本地实现演示并显式标记待迁移 | Level A 独立 Demo 已从空库通过全部能力簇：主体、合同方、激活、补充调额、更正、第二正式合同、合同关系、遗留本地 Evidence v1／v2、终止、阶段／最终结算、操作性关闭、查询、审计、导出、鉴权负例、幂等重放和清理；本地 Evidence 仅如实演示既有实现，迁移到独立 Evidence Service 仍是另一验证轴 |
| Money Flow Ledger | 从正式义务和实际资金形成核销余额，并展示调整、减免、冲销和经营投影 | 主体；义务草稿／确认；金额调整／减免／关闭；收付款；多笔核销；资金和核销冲销；内部转账；账龄；历史余额；现金排期；经营汇总；导出 | 本地模板 IAM | Level A 独立 Demo 已从空库通过全部能力簇：主体、应收义务确认与调额、现金排期、两笔收款与多笔有效核销、错误 Allocation 完整反向及替代核销、内部账户转账、减免、关闭、错误资金冲销、四个时点历史余额、账龄排除、经营汇总、导出、鉴权负例、非法状态、幂等重放和清理；坏账、票据、真实退款与通用导入按既有边界审计结论延期，不由 Demo 猜测实现 |
| People & Groups | 建立人员和组，形成带时效的成员关系、组能力与不可变名册快照 | 配置定义；Person；Group；Membership 加入／换角／结束；Capability 分配／结束；Roster Snapshot；归档；历史与并发拒绝 | 本地模板 IAM | Level A 独立 Demo 已从空库通过全部能力簇：配置定义、两名 Person、资料修改、外部 IAM Subject 绑定、人员停用、目录停用、Group、Membership 加入／换角／结束、Capability 分配／结束、两份不可变 Roster Snapshot、归档、鉴权／过期 revision／inactive／归档后写入负例、幂等重放和清理；IAM Subject 不进入名册快照且不改变 IAM 凭证所有权 |
| Evidence Service | 完成受控上传、服务端校验、短期访问和取消清理 | Create；Local PUT；Complete；File／Version reference-read；Access Grant；Range GET；Abort；cleanup；摘要、大小和媒体类型校验 | 本地模板 IAM；Local Storage Provider | Level A 独立 Demo 已从空库通过全部能力簇：本地签名上传、服务端大小／媒体类型／SHA-256 确认、稳定引用、完整与 Range 下载、上传／下载授权防篡改、幂等重放、失败完成、取消和真实对象清理；OSS、正式 IAM 与完整入口不属于该独立 Demo 的通过声明 |
| Project Workspace | 从项目草稿、资料和责任分配形成 active Project，并展示资源关联、部分不可用、历史和取消 | 创建／更新；Person／Group Assignment；双负责人；缺口立项；Resource Link；引用不可用；历史；取消；幂等与并发拒绝 | 本地模板 IAM；只实现版本化 People 与目标资源 reference-read 的契约桩 | Level A 独立 Demo 已从空库连续两次通过全部能力簇：draft 更新、缺口立项、双负责人、active、Contract／Evidence Link、引用不存在／不可用、解除关联、公开引用、revision 1～9 历史、取消、鉴权／幂等／并发负例和专属卷清理；真实相邻应用组合已由 Level B 证明 |
| Design Operations | 为 active design Project 建立工作项，绑定生产负责人，登记成果并按阶段完成、跳过、回退或取消 | Work Item；负责人；四阶段；允许的不适用；成果登记／替换；完成；重开；恢复适用；历史；取消；外部不可用失败关闭 | 本地模板 IAM；只实现 Project、Assignment、Evidence reference-read 的契约桩 | Level A 独立 Demo 已从空库连续两次通过全部能力簇：Project／Assignment／Evidence 引用三态、负责人失效恢复、四阶段、初设跳过与恢复、级联回退、成果替换、完成／重开／取消、公开引用、revision 1～21 历史、鉴权／幂等／并发与终态负例；真实相邻应用组合已由 Level B 证明 |

“尽量完整”按能力簇判定，不要求一个故事同时走遍互斥终态。Contract 可以用多份合成合同分别展示正常关闭、终止和纠错；Money 可以用多份合成义务与资金分别展示正常核销和冲销。每个能力簇至少有一个可观察结果，风险高的状态变化还要有失败或纠错证据。

## 独立性的判定

一个 Demo 同时满足以下条件才算独立：

- 未启动的业务应用不会导致当前应用 migration、readiness 或自身主闭环失败；
- 应用只写自己的数据库和对象目录；
- 不导入相邻应用的 ORM、领域对象或内部表；
- 外部引用使用版本化公开契约，暂时不可用时失败关闭或明确降级；
- 模板 IAM 是正式 IAM 的同形替代实现，不存在 `AUTH_DISABLED`、万能 Header 或跳过服务端授权；
- fixture、凭证和端口均由 Demo 自己生成或配置，不依赖开发者机器上的既有状态；
- 重置一个 Demo 不删除另一个应用的数据；
- Demo 通过不依赖 Nginx、Gateway、中心 IAM、Workbench、域名或云资源。

Project 和 Design 使用契约桩不表示它们拥有外部事实，也不证明真实组合已经通过。契约桩只隔离 reference-read 端口，使当前应用可以验证自己的状态机、持久化和失败行为；Level B 必须重新使用真实相邻应用。

## Level A 证据等级

每个独立 Demo 同时保存三类结论：

1. **运行证据**：fresh-state migration、readiness、认证、主闭环、重放、清理和应用级回归实际通过；
2. **业务完整性证据**：能力簇中每个主要状态变化都有可观察结果，延期项有明确理由，不用空页面或测试数量冒充业务完成；
3. **解耦证据**：数据写入、引用契约、相邻系统缺席、失败关闭、历史和恢复行为符合应用责任。

三类证据互不替代。单元测试通过不能代替可操作 Demo，人工浏览页面不能代替状态断言，应用能启动不能代替业务闭环。

## Level A 统一通过条件

- 六个 Demo 都能从空状态由统一形态的操作者入口启动和验证；
- 每个 Demo 都使用非真实数据，并在输出中列出创建的稳定 ID 和关键 revision；
- 每个 Demo 都能重复执行，结果为复用同一事实或明确创建新的命名场景，不发生意外重复；
- 每个 Demo 至少证明一次未认证／越权／非法状态／外部不可用中的适用失败场景；
- Contract、Money、People 和 Evidence 不使用相邻业务应用；Project 和 Design 只使用公开契约桩；
- 每个应用原有测试、备份恢复和发布门禁保持通过；
- 能力矩阵中的未覆盖项被显式列出，不以“后续再说”隐藏；
- 六个 Demo 全部通过后，Level B 才能从 `blocked` 变为 `ready`。

## Level B 已通过基线与后续门禁

参考 OA 当前已经覆盖并实际验证：

- J1 两名人员、组内任职、两份不可变名册快照、人员资料／IAM Subject／组内角色变化、成员结束任职与停用，以及 Project 双负责人快照不被上游变化静默改写；
- J2 Contract 补充、更正、终止、阶段／最终结算与关闭，人工确认 Money Obligation、调额、两笔回款、错误核销反向与替代、减免、关闭和误录资金冲销；
- J3 `design-v1` 四阶段生产、Evidence 成果替换、阶段不适用与恢复、补交成果、完成后重开返工和再次完成；
- J4 Contract、Obligation、Transaction、Evidence、Design 五类稳定引用和来源 revision 追溯；
- 完全空库运行、立即幂等重放，以及 Contract、Money、People、Evidence、Design 五个来源逐项真实停机、恢复和非故障项继续可用。

参考 OA 还包含一个合成的设计交付子集装配探针。该探针只启动 People、Evidence、Project、Design 及各自数据库，不启动 Contract、Money 及其数据库；它从空状态完成 J1、J3 和仅含 Evidence／Design 两类 Resource Link 的 J4，Project 最终为 `active / revision 6`，并以完全相同的业务 ID、revision 和状态摘要重放。探针使用真实应用、migration、模板 IAM 和公开 API，运行状态与全量参考 OA 隔离，结束后只清理自己的容器、卷和续跑状态。

第二个合成探针只启动 Contract、Money 及各自数据库，不启动 People、Evidence、Project、Design 及其数据库；它从空状态完成 J2 的正式合同、应收义务、两笔收款、错误核销反向与替代、减免、关闭和错误资金冲销，并以完全相同的业务 ID、revision 和状态摘要重放。该探针同时证明商业台账闭环不以项目、人员、文件或设计应用在场为启动条件。

两类方向相反的探针共同证明客户装配可以位于“单应用”和“六应用全量组合”之间，未选应用可以真实缺席而不是只在界面隐藏。它们均为 `hypothetical_probe` 性质的合成装配证据，不代表第二个真实客户、真实客户验收或跨客户稳定边界。

该结果完成本阶段约定的 Level B 高保真本地业务架构验证，并成为后续回归基线；它不等于参考 OA 全功能复刻、真实客户验收或完整入口通过。后续 OA 能力仍必须从参考 OA 或真实客户证据中选择，至少说明：用户结果、事实所有者、状态与纠错、六层归属、对现有应用的变更、故障影响和验收方式。

Level B 的每次扩展必须继续满足：

- 参考 OA 的项目上下文可以组合事实，但 Project 不复制合同、资金、人员、文件或设计事实；
- 新类型化扩展可以拥有自己的状态机，但不能把客户岗位、万能字段或可编辑脚本写入稳定内核；
- 来源应用或连接器只能提交版本化快照、事件、建议或正式命令，不能越权写目标数据库；
- 移除非必需应用后，其他小系统仍能完成自己的 Level A 主闭环；
- 已确认事实以 revision、调整、冲销或反向记录纠错，不静默覆盖；
- 新增 OA 能力或定制化变化后，六个 Level A Demo 和当前 Level B 旅程继续通过。
- 客户装配声称某应用可选时，至少一个组合探针必须在该应用及其数据库真实缺席的情况下完成相应业务闭环；只是不调用已启动服务不算可选性证据。

## 定制化开发模式

每项客户差异必须进入唯一责任层：

| 层次 | 使用条件 | 交付形式 | 不允许 |
| --- | --- | --- | --- |
| 稳定内核 | 多个独立场景拥有相同含义、所有权与纠错生命周期 | 业务应用版本化能力 | 为单客户增加可选列、角色名或条件分支 |
| 配置 | 只改变术语、字典、显示、编号和简单阈值 | 可校验配置与默认值 | 改变事实含义或状态机 |
| 策略 | 同一事实输入采用不同、可版本化的计算或决定 | 有输入、输出、版本和追溯的策略 | 隐藏在页面公式、SQL 或客户分支 |
| 类型化扩展 | 差异拥有独立对象、状态、责任人和纠错生命周期 | 独立扩展应用或窄域模块 | 塞入万能 Project、Contract 或 People 字段 |
| 客户装配 | 改变应用选择、旅程、角色映射和外部接口 | Suite Manifest、装配配置和集成声明 | 修改核心镜像或共享数据库 |
| 客户专项 | 只在一个客户成立且复用证据不足 | 独立、可替换、可删除的专项包 | 伪装成公共内核或阻止核心升级 |

连接器是层与层之间传递事实的技术机制，不自动成为公共业务层。只有当来源、目标、幂等、失败恢复和目标确认规则稳定时才建立连接器；客户专用连接器可以属于客户装配或客户专项，但仍只能使用公开契约。

定制化模式只有在实际 OA 组合变更中同时证明以下结果才算通过：

- 新能力能够指出唯一责任层；
- 不产生客户核心分支、跨库访问或现场修改镜像；
- 相同核心版本仍能运行独立 Demo；
- 升级只替换版本化制品并迁移配置、策略或扩展；
- 如果需求无法自然归层，记录为反证并重新检查应用边界，而不是继续增加抽象。

### 首次实际变更验证结果（2026-09-25）

从最小 J1～J4 扩展到当前高保真 J1～J4 的变化已经作为第一份实际账目完成验证：

- People 的设计组类型、组内角色和设计交付能力落在现有配置定义；人员资料、任职和名册仍用 revision 与不可变快照表达；
- 四个设计阶段、成果适用性、替换和返工落在独立 `design-v1` 类型化扩展，没有向通用 Project 增加设计字段；
- 六应用／六数据库及五类 Project Resource Link 落在客户装配，未新增共享表、跨库写入或客户核心分支；
- Contract revision 4 到 Money Obligation 的流动继续由责任人明确确认，未在规则冲突尚未解决时制造自动连接器；
- Design reference-read 增加通用 `id` 并保留 `work_item_id`，是所有组合消费者可用的加法兼容修正，不是客户条件分支；旧版最小 Demo 状态也由兼容测试证明可被当前驱动读取；
- 管理费、开票、奖金／工资、项目包和通用审批因缺少稳定生命周期证据保持延期或候选状态，没有被塞入万能配置、策略或核心字段。

量化信号为：长期客户核心分支 `0`、客户专属数据库 `0`、共享业务表 `0`、该增量新增业务 migration `0`、自动连接器 `0`；六个 Level A 回归基线为 `6/6`，Level B 来源停机隔离为 `5/5`。完整逐项账目见 `docs/specs/assembly/product-boundary-validation.md`。

本结果只证明首个参考 OA 的本地定制化开发方式可以成立。策略层尚无被实际执行的真实计算样例，第二个独立客户场景、真实公司 Ubuntu 和完整入口仍是独立门禁。当前高保真开发包只证明该业务组合可以脱离源码和 Registry 复现，不扩大跨客户结论。

首个参考 OA 的六层账目同时发布为 Customer Scenario Evidence v1 基准；它的来源身份固定为 `reference_product`。第二个场景使用真实客户草稿模板，只有客户确认、实际验收和边界信号齐全后才能形成 `single_customer_scenario` 最终结果。详细规则见 `customer-scenario-evidence.md`。

## 明确延期

- 最终客户品牌、营销视觉、暗色主题和超出基础可用性的页面精修；
- Nginx、Gateway、中心 IAM、Workbench、TLS 与公网入口展示；
- 公司私有 Ubuntu 和个人阿里云试验田的完整入口部署；
- 没有参考证据或真实生命周期的通用审批、通知、低代码、表单或流程平台；
- 用单个参考 OA 宣称整个商业圈子的产品边界已经成立。

延期项不降低业务正确性、服务端授权、历史、数据所有权、幂等或失败恢复要求。

## 失败判据

出现以下任一结果时，相应 Demo 或边界验证失败：

- 静态输出或直接写库代替真实业务命令；
- 关闭鉴权、信任客户端身份 Header 或使用万能管理员绕过；
- 为独立 Demo 启动全部真实业务应用；
- Project 或 Design 的契约桩越过公开 reference-read 或替应用确认业务状态；
- 为恢复 OA 页面而复制目标应用事实、共享数据库或维护 OA 专用核心分支；
- “可配置”实际承载不同事实含义或不同纠错生命周期；
- 一个应用停止后，其他应用记录虚假成功；
- 只报告测试数量、页面截图或 HTTP 200，没有核对最终业务状态；
- 把 UI 延期解释为可以没有可操作入口或结果说明。

## 当前事实

- 六个应用都已有独立运行、数据所有权和本地鉴权基础；
- Project J1、Design J3 及六应用 J1～J4 组合已有自动化和真库证据；
- `deployments/demos/ops/level-a audit --confirm-destroy-demo-data` 已顺序执行六套独立 Demo；每套都通过 fresh-state、run、replay 和专属数据清理，运行门禁为 6/6；
- Contract Ledger、Money Flow Ledger、People & Groups、Evidence Service、Project Workspace 与 Design Operations 均已覆盖完整必需能力簇；补齐后的最终统一审计已于 2026-09-25 实际输出 `LEVEL_A_STATUS=passed`，运行报告为六项 `passed / fresh-state,run,replay,cleanup`，清理后无专属容器和数据卷残留；
- Level B 已在完全空库形成 People revision 4／8 名册快照、Contract `closed / revision 4`、Money Obligation `closed / revision 2`、Design `completed / revision 18` 和 Project `active / revision 9`，立即重放复用同一批稳定业务 ID；
- Contract、Money、People、Evidence 和 Design 五个来源逐项真实停机时，Project 始终保持 `active / revision 9`、两个 Assignment 快照可读，只有对应实时引用明确不可用，恢复后全部服务重新 ready；
- 设计交付子集装配只启动 People、Evidence、Project、Design 及四个独立数据库，Contract、Money 及其数据库真实缺席；从空状态连续两次完成 J1／J3／J4 子集，保持同一 Project `active / revision 6`、同一 Design `completed / revision 18`、同一 Evidence Version 和相同状态文件摘要，随后清理隔离容器与卷；
- 商业台账子集装配只启动 Contract、Money 及两个独立数据库，People、Evidence、Project、Design 及其数据库真实缺席；从空状态连续两次完成 J2，保持同一 Contract `closed / revision 4`、同一 Money Obligation `closed / revision 2`、同一批收款与纠错事实和相同状态文件摘要，随后清理隔离容器与卷；
- 首个实际 OA 组合变化已经通过六层归属、无客户核心分支、向后兼容与反证检查；跨客户适配和真实策略运行仍未验证。
- `v0.1.0-demo.11` Linux/amd64 开发包从归档解压后通过摘要、离线镜像装载、新秘密初始化、六个独立产品 6/6 验收、设计交付与商业台账两类子集稳定重放、两次同 ID 高保真 J1～J4、五来源隔离、最终 12 容器健康检查和九个验收 Compose project 的零残留检查；发布模式全程禁止目标编译与联网拉取，包内只读预检也已通过正反例验证，该证据仍不等于真实公司 Ubuntu 或正式 clean-HEAD 发布通过。
