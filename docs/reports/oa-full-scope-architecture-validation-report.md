# OA 完整体架构可行性验证报告

2026-10-06 BF-120 注记：本报告是 2026-09-23～09-30 的冻结历史报告，不是当前实现完成度来源。用户已确认以 `AGENTS.md` 为当前基线；报告正文及其后追加的 C7／C8、BF-083～BF-119 实施状态只作为候选历史证据，需经当前代码、持久证据和干净版本复验。下一步见[当前实现基线与下一步](../status/current-baseline-and-next-actions-20261006.md)；在新报告生成前总体继续 `insufficient-evidence`。

2026-10-03 追加口径：用户明确招投标只是举例，完整克隆的经营、生产、资料和权限操作全部在[交付清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)逐行保留。经营参与职责兼容版本与原人员组件已发布，有限增减／历史重入／丢回复原命令恢复及 `.6` 公网零写导航通过、旧事实保持；原岗位和全角色矩阵仍缺。Contract 写端口只是真库／独立启动候选，未部署、未接原表单，主 schema 18 保持。分层结果见[本批报告](./oa-complete-operation-progress-20261003.md)。正文冻结记录不据此重写，完整结论仍为 `insufficient-evidence`。

状态：阶段一验证结论已冻结；完整 OA 仍为 `insufficient-evidence`  
报告版本：`1.0-stage1-final`  
验证时间范围：2026-09-23 ～ 2026-09-30

## 1. 报告信息

### 2026-10-02 使用流程证据增补（不改写冻结运行记录）

正文是 2026-09-30 当时定义的架构／连续工程链结果；两项 C8 缺口不是可用 OA 的全部剩余工作。用户试用和[原 OA 使用流程源码梳理](../specs/assembly/oa-original-usage-workflow-baseline.md)又发现派发／正式编号时机、生产负责人后补、完成后同项目详情导航与当前实现不符，并暴露只读表单、UI 可达性及尚未逐动作验收的经营／生产／资料／统计面。因此总体仍为 `insufficient-evidence`，即使 C8 关闭也不能据此转为“完整可用 OA supported”。原站真人复核、克隆操作等效与失败后续是独立未关闭门禁，详见 BF-083；本次仅文档与源码证据，无主站升级或业务测试结果。

| 项目 | 当前事实 |
| --- | --- |
| Base／源码基线 | 工作树基于 Git `61475e54f97342809affb6b3c6a1fa948be9f185`；阶段实现尚未冻结为 clean release |
| 当前公司 D1 候选 | 主运行栈为 `v0.1.0-test.20` development runtime；隔离栈以 `test.26` 恢复数据为基线并逐批应用兼容闭包；C7 使用 Bonus `test.40`，C8 在同一 17 服务隔离栈完成；仓库 release bundle 已补齐 C7／C8 verifier，仍不是生产发布 |
| Customer Overlay | `reference-oa` 角色映射、`reference-oa-c5-v1` 完成 Profile、`reference-oa-bonus-v1` 奖金策略；尚未冻结统一 Overlay 版本 |
| Suite Manifest | 公司 D1 当前同时选择十二个业务应用，主栈与隔离栈 17 个长期容器全部 healthy；完整备份已在独立空目标恢复、继续写入并再次备份；同一数据集连续通过 C0～C7，C8 完整执行后因两项装配缺口判为 `insufficient-evidence` |
| Deployment Profile | 本地隔离组合与公司 Ubuntu D1 Compact；D2～D4 未验证 |
| 本轮公司镜像摘要 | 连续 C6 使用 Cost `878200e8…3ee5`、Money `5262b567…9af`；`test.38` release manifest 固定全部 17 个摘要，其余镜像沿用已验候选 |
| 主要证据目录 | [`docs/reports`](.)、[`docs/testing.md`](../testing.md)、[`docs/decisions.md`](../decisions.md) |

本报告不把“代码存在”“页面存在”或“容器 healthy”单独视为业务通过。状态只来自已运行的正常、拒绝、纠错、故障和恢复证据。

## 2. 当前执行结论

| 字段 | 结论 |
| --- | --- |
| 当前结论 | `insufficient-evidence` |
| 一句话理由 | 当前边界已在公司 Ubuntu 用同一 Tenant／Project 连续完成 C0～C7，并在 C8 证明个人工作、Money 分权、顺序来源故障恢复和十二库事实不变；但 Project 缺少 C4～C6 active Resource Link／资料投影，且 Gateway 遗留 Evidence `view` 入口使 `metadata.read + download` 不能独立进入，因此连续 C8 未过退出门槛。 |
| 已支持范围 | 小应用数据所有权、版本化契约、多个可裁剪最小组合、资源级 IAM、逐来源故障隔离、参考 OA 的大量 Base／Overlay 加法实现 |
| 尚不支持的结论 | “OA 完整体已经通过”“跨客户 Base 已验证”“公司环境可生产使用”“D1 已具备高可用” |
| 当前阻断项 | C8 的 C4～C6 资料 Link／投影与 Evidence Gateway 细粒度入口；修正摘要的真人浏览器下载；完整双 Tenant 权限矩阵；第二客户反证 |

`insufficient-evidence` 不等于架构失败。C8 暴露的是可通过显式 Link／版本化投影、细粒度入口和发布兼容闭包修正的装配问题；当前没有发现必须共享业务表、长期 fork 公共内核或放弃数据所有权才能继续的反证，但在同一 Project 上复验通过前不能给出完整 OA `supported`。

## 3. 架构主张验证

| 主张 | 当前结果 | 直接证据 | 反证／限制 |
| --- | --- | --- | --- |
| 小应用可独立交付 | `supported-in-tested-scope` | Party、Billing、Production、Results、C6、C7 的独立 migration／数据库／进程／恢复报告；Level A 6/6 | 并非所有应用都已在公司 Ubuntu 单独复演 |
| 最小组合能完成目标业务 | `supported-in-tested-scope` | C4 最小组合、C5 独立进程旅程、C6／C7→Money 双进程旅程、连续 C2 的 Contract／BE／Evidence／Project 与连续 C3 的 Party／Billing／Money／Project 组合 | C6、C7 的部分正式 connector 组合仍不完整 |
| 完整 OA 可由当前边界重组 | `insufficient-evidence` | 十二业务应用已在公司 D1 17/17 healthy；完整备份已独立恢复、继续写入并再备份；同一 Suite／Overlay／数据集已连续通过 C0～C7；C8 的个人工作、Money 分权、故障恢复和十二库不变也通过 | C8 的 C4～C6 资料发现契约和 Evidence Gateway 下载分权未闭合 |
| 跨应用只使用版本化契约 | `supported-in-tested-scope` | 精确 reference API、workload credential、revision／digest 快照、故障恢复报告；15 应用静态门禁为 0 跨应用导入、0 跨应用 replace、0 跨库 SQL 机制 | 仍需完整组合流量证据和 build-tag 扩展持续守卫 |
| Stable Base + Customer Overlay 可承载参考 OA | `conditionally-supported` | OA 角色、C5 Profile、奖金公式均以版本化 Overlay／策略实现；无共享表 | 尚未冻结统一 Overlay 版本，也没有第二客户升级证据 |
| 客户定制没有污染公共内核 | `supported-so-far` | OA 岗位映射、奖金公式和客户装配未写入稳定领域条件 | 只证明一个主要参考客户，不能外推所有客户 |
| 权限层次保持职责分离 | `supported-through-C7-with-C8-edge-gap` | C1 九角色矩阵、C2 撤权／故障、C3 金额分权、C4 浏览器权限生命周期、连续 C5／C6、C7 精确 workload scope，以及 C8 Subject 隔离和金额 grant/revoke | Evidence 目标授权已细分，但 Gateway 仍要求遗留 `view`；完整双 Tenant 与浏览器矩阵仍缺 |
| 同一制品可迁移到真实 Ubuntu | `partially-supported` | 个人 ACR 镜像由另一阿里云账号的公司 ECS 拉取；完整十二应用升级、17/17 healthy；隔离空目标恢复、继续写入和再次备份通过；正式多版本、Money 指标与 Production 个人工作入口通过 | HTTPS、离机备份和完整连续业务旅程尚未在该目标同时通过 |

## 4. C1～C8 业务覆盖

| 旅程 | 状态 | 事实所有者 | 已有直接证据 | 仍缺 |
| --- | --- | --- | --- | --- |
| C1 项目与责任建立 | `passed` | Project；People 提供人员／分组事实；Party 提供客户当前档案 | Project Package、转换／撤销、152 条入口断言、九角色、Party→Project 精确复验与恢复 | 完整双 Tenant 交叉矩阵 |
| C2 经营承接与合同链 | `passed-in-current-reference-scope` | Business Engagement；Contract 独立确认法律事实 | 同一 C1 Tenant／Project 的 Party、Evidence、Contract／Settlement、ready BE、Project links；409、Evidence 503 零写入、同命令恢复重放、精确 workload audit、所有权增量和 BE 间接资料运行链 | 可选下游 Money 建议、完整双 Tenant 和更广字段／代理动作 |
| C3 开票、应收与回款 | `passed-in-current-reference-scope` | Billing、Contract、Money 各自拥有事实；Project 只读组合 | 同一 C2 Tenant／Project 的 Party 显式迁移、发票追加纠正、应收／回款／核销／Receipt Set、四来源分段、金额脱敏、Billing 局部故障恢复、40/40 与所有权精确增量 | 真实浏览器最终点击、双 Tenant、Settlement／应开未开扩展 |
| C4 多类型生产交付 | `passed-in-current-reference-scope` | Production Operations；Project／People／Evidence 只提供版本化事实 | 同一 C3 Tenant／Project 的 `design-v1`＋`consultation-v1` 各 revision 8、两个 File／三个 Version、类型与 stale 拒绝、Evidence 503 零写入／同命令恢复、成果替换／返工、精确所有权增量；另有最小组合、正式 D1、86 条权限断言与浏览器证据 | 双 Tenant；更多生产类型仍需按证据逐项加入 |
| C5 批复、审计与项目完成 | `passed-in-current-reference-scope` | Approval & Audit Results；Project 拥有自身确认检查点 | 独立 74 条真实入口断言与 D1 空目标恢复；同一 C4 Tenant／Project 的批复／审计、显式检查点、金额脱敏、撤权 403、append-only correction、来源 drift、Evidence／Contract／Results 故障恢复、46/46 首轮与 14/14 完成态重跑、精确所有权增量 | 双 Tenant、账号停用和该旅程真实浏览器 |
| C6 成本与外委 | `passed-in-current-reference-scope` | Cost & Outsourcing；Money 独立接受应付建议 | 同一 C5 Tenant／Project 的 Cost revision 2、Engagement、Assessment、Payable Suggestion revision 3 与 Money Obligation；正式 Gateway／IAM connector、目标停机恢复、接受、精确所有权增量、24/24 首轮与 15/15 完成态重跑、17/17 healthy | 跨进程撤回、完整双 Tenant 和该旅程真人浏览器 |
| C7 奖金计算与分配 | `passed-in-current-reference-scope` | Bonus Distribution；Contract／Money／People／Project 提供精确来源；Money 独立确认建议 | 同一 Project 的经营 Settlement＋生产 Receipt Set、整数 Overlay、正式 workload connector、Money 停机恢复、独立接受、来源撤回不级联、完成态重跑，17/17 healthy | 单来源最小组合、双 Tenant、真人连续浏览器 |
| C8 资料、统计与个人工作 | `insufficient-evidence` | Evidence／各业务来源拥有事实；Project／Workbench 只读组合 | 同一 Project 的 C2 资料、Subject 工作隔离、Money grant/revoke 分权、Evidence／Production 顺序故障恢复、49/49 HTTP、十二库 canonical digest 不变、17/17 healthy | C4～C6 active Link／资料投影缺失；Gateway 遗留 `view` 阻断独立 download；修正镜像的真人浏览器复验 |

“passed”只表示当前已确认的参考 OA 范围具有正常、拒绝、纠错或恢复证据，不表示该领域所有未来客户规则都已实现。

## 5. 应用与最小组合

| 目标业务结果 | 主要应用 | 显式直接依赖 | 可真实缺席的相邻应用 | 当前证据 |
| --- | --- | --- | --- | --- |
| 主体管理 | Party Directory | IAM | Contract、Money、Project 等 | 独立 release／恢复与管理页通过 |
| 项目立项与责任 | Project | IAM；按选择使用 People、Party | Contract、Money、Billing、Production | C1 最小权限链与 Party 故障恢复通过 |
| 多类型生产交付 | Production | IAM、Project、People、Evidence | Contract、Money、Billing、BE | consultation-only 最小组合、未选缺席和恢复通过 |
| 批复审计检查点 | Results | IAM、Project、Contract、Evidence | Billing、Money、Production | C5 正式组合、未选缺席和空目标恢复通过 |
| 成本／外委到应付建议 | C6 | IAM、Project、Party、Evidence；建议阶段连接 Money | Billing、Production、Bonus | 主应用 D1、正式 Gateway／IAM connector、目标停机恢复、接受和完成态重跑均通过 |
| 奖金到应付建议 | C7 | IAM、Project、People；Contract 或 Money 至少一类来源；建议阶段连接 Money | C6、Billing、BE | 两类来源组合和 C7→Money 通过；单来源正式组合待补 |
| 项目资料浏览 | Project | IAM、Evidence；可选业务来源投影 | Money、Billing、Production 等 | direct 运行链、BE 间接运行链和正式入口只读通过，浏览器写旅程待补 |

这些组合没有发现隐藏共享表、跨库外键或共享 ORM。所谓“独立”允许交付包携带完成该业务结果所需的显式直接依赖，不等于单进程或零依赖。

## 6. Base 与 Customer Overlay 变化账

| 变化 | 唯一归属层 | 理由 | 兼容与回退 | 证据 |
| --- | --- | --- | --- | --- |
| 项目／业务资源 Capability | Base 契约 | 所有客户都需要稳定业务动作与资源类型 | 向后兼容 Manifest；候选 IAM 预解析 | C1～C8 capability reports、BF-077 |
| OA 站点角色到 Capability | Customer Overlay | 客户岗位名不属于领域内核 | 版本化映射，可移除 | C1／C2 权限实验 |
| `consultation-v1` | 类型化 Base 扩展 | 有独立阶段与完成条件，不是客户任意字段 | additive migration；保留 `design-v1` | C4 报告 |
| `reference-oa-c5-v1` | 版本化策略／Profile | OA 完成解释不等于通用 Project 状态 | 只读评估；显式确认追加历史 | C5 报告 |
| `reference-oa-bonus-v1` | Customer Overlay 策略 | `÷1.065` 是客户计算规则 | 精确版本、输入／输出 digest，可换策略不改历史 | C7 报告 |
| Suite 选装、菜单和入口 | 客户装配 | 不同客户选择不同业务结果 | Manifest 控制，不删除既有数据 | D1 各最小组合报告 |

当前直接证据支持：客户核心长期分支 `0`、共享业务表 `0`、跨应用外键 `0`、现场核心补丁 `0`。Base／Overlay 最终变更数量尚未冻结，必须在 clean release 前由版本差异和 Overlay 清单生成，不在草案中猜数。

## 7. 数据、契约与故障边界

| 验证项 | 当前结果 | 证据 | 风险 |
| --- | --- | --- | --- |
| 数据所有权 | `supported-in-tested-scope` | 各应用独立 DB／账号／migration；Project／Workbench 只读组合；静态边界门禁与负例自测通过 | 最终完整组合仍需运行流量与数据库权限审计 |
| 契约兼容 | `partially-supported` | 版本化 API、严格解码、候选 IAM `validate-contracts`；C2 从缺失 Project BE URL 与旧 BE reference API 404 逐层定位到 `test.31` verifier 闭包，未放松契约 | 通用消费者配置／API／migration 依赖图仍未自动生成 |
| 幂等与重放 | `supported-in-tested-scope` | C1～C7 多项 Command receipt、摘要冲突、恢复后单次提交；C6／C7 目标停机后重放未重复创建；C8 恢复同一工作与投影 | C8 是只读装配，不覆盖新的跨应用写 connector |
| 故障隔离 | `supported-in-tested-scope` | J1～J4 五来源停机；C2～C7 逐来源故障；连续 C8 顺序停止 Evidence／Production，Workbench 保持 200、Project 分段降级并恢复 | 不证明多来源同时故障、HA 或持续负载 |
| 备份恢复 | `supported-on-host / off-host-pending` | 公司完整备份 `20260929T155403Z` 已在独立空目标恢复，17/17 healthy；正式入口继续写入后新备份 `20260929T171025Z` checksum 全部通过，Evidence 由 9/10/10 增至 10/11/11 | 备份仍在同一宿主机；离机加密副本和跨机器恢复未证明 |
| 用户可感知性能 | `partially-supported` | C3 本地 p50/p95 `13/220 ms`；C8 Money 指标服务器回环入口单次 `4.250～20.572 ms` | 没有完整组合、公司浏览器、公网链和持续负载测量 |

本轮重要反证从 `test.6` 延伸到连续 C6 的 `test.38`：新 Capability Manifest 需要新 IAM，Project 前向 migration 后旧二进制按 schema 门禁拒绝启动；新 Suite 字段要求同步提升严格解析它的 Gateway／Workbench，遗留 one-shot migration 容器不能复用；Production 新 People 依赖曾因 Compose 环境变量错位失败；完整选择又暴露 Bonus 镜像遗漏版本化 wire contract，以及 IAM 启动路径在 workload credential 建立前错误构造静态 Money 客户端。连续 C2 首次最终资料读取又先因 Project 未注入 BE URL 返回 503；补齐后，精确 IAM audit 已存在但目标 reference API 404，进一步定位到 BE 运行镜像落后。连续 C6 则发现 Cost 与 Bonus 把 Money workload action 缩写为 `payable-suggestion.*`，同时 Money IAMProvider 不接受建议资源的精确 Scope；两处均以完整命名空间 action 和受限 exact resource scope 修复，未放宽目标应用最终授权。修正后的规则是按消费者配置、API 契约和 migration 兼容闭包发布，升级前联合备份，失败后不削弱备份门禁。详见 [ADR-0035](../adr/0035-d1-release-upgrades-use-compatibility-closure.md)、[连续 C2 验证](./oa-full-suite-continuous-c2-validation-20260930.md)、[连续 C6 验证](./oa-full-suite-continuous-c6-validation-20260930.md)、[C8 Production 个人工作正式 D1 验证](./oa-c8-production-personal-work-d1-validation-20260929.md)和 [OA 完整 Suite 公司 D1 装配验证](./oa-full-suite-d1-assembly-validation-20260929.md)。

## 8. 权限架构专项验证

### 8.1 信任链责任

| 层级 | 当前结果 | 已证责任 | 限制 |
| --- | --- | --- | --- |
| Nginx | `supported-in-tested-scope` | 唯一入口、HTTP／HTTPS、反代边界；公司只读链实际经过 Nginx | 公司当前通过 SSH 隧道使用 HTTP 宿主入口，公网 HTTPS 仍待目标域名 |
| Gateway | `supported-in-tested-scope` | 静态路由、外来身份 Header 清洗、应用授权、限流；C8 Money 指标与 Production 工作项正式路由通过 | 完整组合流量与持续负载仍待验 |
| IAM／Policy | `supported-in-tested-scope` | Subject／Tenant、Grant、Assignment Fact、短期有界 Credential、workload 换票；连续 C5～C7 精确 scope，C8 Subject 隔离与金额 grant/revoke 刷新收敛通过 | Evidence Gateway entry action 仍过宽；双 Tenant 和完整浏览器矩阵待补 |
| 目标应用 | `supported-in-tested-scope` | Audience／Action／Scope／状态／revision 复验和 `allowed_actions` | 仍需 C8 浏览器直接请求负例 |

### 8.2 代表性角色证据

| 角色 | 已证明的允许／拒绝 | 撤权／故障 |
| --- | --- | --- |
| identity-super-admin | 不因身份管理权自动拥有 Project／BE 等业务权 | C1／C2 反向隔离 |
| oa-business-admin | C1／C2 显式 tenant actions | Grant 撤销后页面与服务端收敛 |
| general-manager | C2／C3 全局只读或金额读取 | 写入拒绝、账号停用／恢复 |
| business／production-manager | Project／BE／Production 职责分离 | Assignment／Grant 生命周期 |
| project owner／member | 精确 Project Scope；相邻 Project 拒绝 | Assignment 结束后新凭证拒发 |
| finance／office | Billing／Money 所需动作；经营／生产治理拒绝 | 金额撤权与恢复 |

已覆盖无凭证、错误签名、错误 Audience／Action／Scope、相邻资源、状态拒绝、伪造 Header、客户端隐藏按钮不替代服务端授权、撤权、账号停用及 IAM／Policy 故障。尚未把全部 C1～C8 角色在两个 Tenant 中跑成一张完整矩阵，因此本节不能给出全局 `passed`。

## 9. 人类可用体验

| 页面／旅程 | 当前结果 | 浏览器或运行证据 | 未决问题 |
| --- | --- | --- | --- |
| 六个独立 Demo | `passed` | Level A 6/6；统一页面骨架与导航回归 | 后续新增应用继续保持体验基线 |
| C1／C2 工作区 | `passed-in-tested-scope` | 真实浏览器撤权／恢复与错误状态 | 双 Tenant |
| Billing／Production／Bonus | `passed-in-tested-scope` | 相应人类工作流与 Production／Bonus 真实浏览器 | 完整跨应用连续旅程 |
| C5／C6 | `partially-supported` | 页面结构、独立进程、正式入口；C6 正式 connector 自动化链已通过 | C5/C6 真人浏览器连续旅程 |
| C8 Project 资料 | `failed-current-assembly-gate` | 同一 Project 的 C2 BE→Evidence 两段运行、Evidence 停机分段 unavailable／恢复已通过 | C4～C6 Link／投影不存在；Gateway 独立 download 入口失败；页面 `redirect` 已在源码修为 `direct` 但未以新摘要复演 |
| 统一 Workbench | `supported-in-tested-automation-scope` | 同一 Subject 仅见自身工作，相邻 Subject 为空；Money 指标授权前后精确变化；Evidence／Production 单源故障时 Workbench 200 | 真人浏览器和双 Tenant 仍待验 |

## 10. 真实环境验证

| 验证项 | 当前结果 | 证据 |
| --- | --- | --- |
| 环境所有权与授权 | `passed-for-validation` | 公司允许使用的阿里云 ECS；不作为生产结论 |
| ACR 跨账号镜像供应 | `passed` | 个人 ACR 的固定摘要由另一账号 ECS 拉取并运行 |
| 镜像摘要与目标架构 | `passed-for-current-selection` | `linux/amd64` pull-only；完整选择固定摘要由 `test.20` release manifest 保存 |
| 入口就绪 | `passed` | 17/17 healthy；Nginx `/health/ready` 200；十二应用 schema current |
| IAM→Evidence→Project 多版本链 | `passed` | 正式 Version 2、历史 Version 1、Project `available → revision_changed → available`、revision 2→4 |
| 浏览器业务写旅程 | `partially-passed` | 自动化正式入口连续 C0～C7 已在恢复栈通过；C1／C2／C4／C7 等既有真人浏览器切片已有证据；C8 自动化完整执行并保留失败证据 | 修正摘要下的 C8 独立下载与全旅程人工复核仍待验 |
| HTTPS／公网 DNS | `pending-on-current-company-D1` | 现阶段使用 SSH 隧道；不能据此宣称公网 HTTPS |
| Money 指标正式权限链 | `passed-in-tested-scope` | 6 项非空投影；无统计权 403；统计／金额分权；撤权刷新；事实摘要不变 | Workbench 真人浏览器、第二来源、双 Tenant 待补 |
| Production 个人工作权限链 | `passed-in-tested-scope` | People→Project→Production 正式夹具；47/47；连续 C8 又复用同一 Project／Subject，邻近 Subject 为空，来源停机恢复且事实摘要不变 | 真人浏览器、Group Assignment、第二工作项来源与双 Tenant 待补 |
| 升级 | `passed-for-current-selection` | `test.20` 对十二业务应用完成预检、完整备份、全部 migration、17 服务重建和 `d1 check`；恢复闭包又以 `test.26` Contract 前向迁移验证 | 通用消费者依赖图仍待自动化 |
| 联合备份 | `passed-on-host` | 来源 `20260929T155403Z` 与恢复后新备份 `20260929T171025Z` checksum 全部 OK，十二个应用选择均为 true |
| 空目标恢复 | `passed-on-current-host` | `20260929T155403Z` 恢复到独立项目后 17/17 healthy，正式入口读写通过，并形成 `20260929T171025Z` 新备份 |
| 离机备份 | `pending` | 两份备份均已校验但仍在同一宿主机；尚无受控加密离机副本和跨机器恢复 |

真实 Ubuntu 已满足“完整入口＋升级＋同宿主隔离空目标恢复＋恢复后继续写入＋再备份”；离机备份、公网 HTTPS 和完整连续业务旅程仍未满足，因此最终结论不能为 `supported`。

## 11. 发现与反证

### 11.1 支持当前架构的证据

- 不同业务结果可以由不同最小组合完成，未选择应用可以真实缺席；
- Party、Project、Contract、Money、Billing、Production、Results、C6、C7、Evidence 仍保持独立事实和历史；
- 连接器以建议／快照交付，目标应用独立确认；
- 资源级 IAM、目标应用最终授权和页面 `allowed_actions` 可以同时成立；
- OA 特有角色、完成规则和奖金公式目前都能以 Overlay／策略加法实现；
- 个人 ACR 到另一阿里云账号 ECS 的固定摘要供给成功，证明客户 ECS 不必为每个交付复制一套 ACR。

### 11.2 已要求修正的边界

- Party 从两个台账内部目录演进为独立应用时，必须显式映射、切换和回滚，不能按名称自动合并；
- Production 不是第二套 Design 数据库，而是同一事实所有者的类型化兼容演进；
- C5 不能让 GET 自动写 Project 完成，必须显式确认 Project 自有检查点；
- C6／C7 不能直接写 Money，应只提出版本化建议；
- C8 不能建立统一可写待办／报表库，只能组合来源投影；
- D1 发布不能按 Git diff 猜服务，必须按契约和 migration 兼容闭包；Gateway／Workbench 等 Suite 消费者也在闭包内。
- Compose 不能复用旧 one-shot migration 容器；正式升级需保留卷清理旧容器，再以同一候选重建。
- Evidence 备份后恢复写服务不得重新等待已经退出的 migration 依赖，必须显式无依赖启动并等待健康。
- Assignment 中的 People Person ID 不得直接当作 IAM Subject；消费方必须经 People 的版本化 Policy Fact 解释当前可信关联。

### 11.3 当前明确失败信号

- 首个 `test.6` 候选的旧 IAM 拒绝新 `capability_namespace`；
- Project 已前向 migration 后旧二进制拒绝未来 schema；
- 首个含 `metric_projection` 的候选被旧 Gateway／Workbench 严格拒绝；
- 旧升级脚本复用了遗留 migration 容器，旧 Evidence 备份恢复命令又错误重算 migration 依赖；
- `test.16` 首次 Production 升级因 People URL 被放入错误服务环境块而在 migration 前失败；门禁随后正确拒绝在 Production 未运行时创建可能不完整的新备份，现场先恢复健康再重跑完整升级；
- 第一轮个人工作验收的 bootstrap 请求与权限矩阵叠加命中正式限流 429；驱动改为等待固定窗口自然翻页，没有重启 Gateway 或提高限流值；
- `test.19` 首次完整选择先因 Bonus Dockerfile 未复制 `payable-suggestion-contract` 而无法构建；补齐显式 wire contract 后镜像成功，未引入 Money 领域代码；
- `test.19` 公司完整组合随后因 Bonus 在 IAM workload credential 建立前错误构造空静态 Token 客户端而停在 16/17；`test.20` 把模板静态 Token 与 IAM workload credential 严格二选一，恢复 17/17 后重新完成完整备份与正式升级；
- 连续 C7 首次读取带 additive Party 字段的 Settlement 时，Bonus 的严格 JSON 解码把合法扩展误判成来源不可用；改为“宽读未知字段、严验已知 schema／ID／revision／金额／状态”，未放宽业务不变量；
- 连续 C8 证明 Project 只有 C2／C3／C7 的六条 active Resource Link，C4～C6 输出无法被最终资料装配发现；
- 连续 C8 又证明 Evidence 目标服务虽区分 metadata／download，Gateway 仍要求遗留 `evidence-service.view` 才允许进入；窄 `metadata.read + download` 在入口返回 401；
- Evidence 人类页面使用契约不存在的 `delivery_mode: redirect`，实际会返回 422；仓库已改为 `direct` 并加入回归测试，但运行摘要尚未复演；
- D1 release 审计发现连续旅程发布包只携带 C0～C6 verifier；已补入 C7／C8，release bundle 与 upgrade selection 测试通过；
- 当前没有证据支持通过放松严格解码或 down migration 绕开问题；
- 这些失败被保留为架构证据，没有从报告中删除。

## 12. 未决事项与优先级

| 优先级 | 未决事项 | 对结论的影响 | 恢复条件 |
| --- | --- | --- | --- |
| P0 | C4～C6 Project Link／资料投影 | 阻断连续 C8 和“完整 OA” | 在原 Project 上增加显式版本化 Link／投影，C2～C6 coverage 全为 true，不读共享表 |
| P0 | Evidence Gateway 独立 download 入口 | 阻断下载分权 | `metadata.read + download` 不依赖遗留 `view` 即可从唯一入口获得短期地址，目标服务仍复验 file scope |
| P0 | C8 修正摘要与真人资料旅程 | 阻断 C8 人类体验 | 发布 `direct` 页面与细粒度入口，真人确认不可下载反馈、授权后下载、金额脱敏和相邻 Subject 空工作 |
| P1 | C8 第二来源／工作项类型 | 降低外推强度，不改变当前失败 | 在 P0 关闭后逐来源单批加入，不并行扩容 |
| P1 | 完整双 Tenant／跨租户矩阵 | 阻断权限全局通过 | C1～C8 已实现动作至少一组交叉 Tenant 拒绝 |
| P1 | 兼容闭包消费者账 | 影响升级可靠性 | 自动关联契约／migration 与所有运行消费者 |
| P2 | 离机加密备份与公网 HTTPS | 影响生产准备，不阻断本地业务判断 | 受控存储副本和已备案域名／证书入口 |
| P2 | 第二参考客户 | 阻断跨客户 Base 结论 | 使用不同甲方角色与业务差异完成第二轮证伪 |

## 13. 下一轮验证输入

第二阶段不直接复制 OMS 页面。候选甲方可采用“资质单位＋施工方”主体结构，重点挑战：

1. 同一 Party 在不同业务中的角色和快照是否仍无需复制主体模型；
2. OA 的设计方角色映射能否被完全移除，而不修改 Base；
3. 资质、施工、项目、合同、资金和 Evidence 的责任边界是否仍成立；
4. 哪些差异属于新类型化扩展，哪些只是 Overlay、配置或连接器；
5. 从本阶段 Base 升级到第二客户版本时，既有客户是否无需核心 fork 和共享表。

第二阶段只有在本报告 P0 项完成并在原 Project 上重跑 C8 后才正式进入，避免用新场景掩盖第一阶段已发现的装配缺口。

## 14. 当前建议

1. 继续保留应用独立数据所有权、版本化契约、显式依赖、目标应用最终授权和 Evidence 分权；
2. 把兼容闭包、Overlay 版本与 Suite 选择提升为发布一等制品；
3. 保留已经通过的 C0～C7 与 C8 失败证据，不新建 Tenant／Project；先补 C4～C6 Link／资料投影和 Evidence Gateway 入口，再按同一 C8 verifier 重跑；
4. 下一次只有在 C8 coverage、独立 download、十二库事实不变、顺序来源恢复和 17/17 health 同时通过后，才重新评估完整 OA 为 `conditionally-supported`；
5. 把两份已校验备份复制到受控加密离机位置，并单独安排跨机器恢复，不与当前业务旅程并行制造负载；
6. 在第二客户证据出现前，不把 Stable Base + Customer Overlay 描述为跨客户已验证成功。

当前报告已经完成第一阶段的科学解答：方案在 C0～C7 和 C8 多数切片中显示出可行性，没有出现必须共享业务表或长期 fork Base 的反证；但最终资料发现与 Evidence 边缘分权未闭合，所以“完整 OA 已通过”为假，当前必须保持 `insufficient-evidence`。

静态边界证据见 [OA 应用静态边界审计](./oa-static-application-boundary-audit-20260929.md)，连续失败证据见 [连续 C8 验证](./oa-full-suite-continuous-c8-validation-20260930.md)。静态门禁已经进入 `make check`，但不能替代同一 Suite 的运行流量证据。
