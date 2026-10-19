# OMSBizFoundry Project State

Status: active
Revision: 129
Updated: 2026-09-29T00:06:31.265Z
Canonical Record: state.record.json

## Project Intent

建立一组可独立安装、独立运行、快速交付并稳定演进的窄域业务小系统；Complete Suite 0.1 最低支持单台 ECS，同时保持应用数据所有权和独立性。

## Current Focus

Fresh-select the bounded C8 responsibility-boundary audit before implementing any cross-domain projection; continue static-first and run at most one isolated stack at a time.

## Active Work

- Active cases: 4
- Project gaps: 0

## Software Definition

OMSBizFoundry 以可独立交付的小应用组成 Complete Suite；统一入口、IAM、独立数据所有权、Evidence 边界、Suite Manifest 和 D1-D4 Deployment Profiles 由 Architecture Baseline 0.1 约束。

| Decision Area | Status | Revision | Current Decision | Project Gaps |
| --- | --- | ---: | --- | --- |
| product_intent_and_scope | settled | 4 | 建设小而完整、可独立交付的窄域业务应用组合，不建设万能业务平台；阶段一以现有完成度较高、定制化明显的 OA 为参考产品，按可追溯核心旅程验证小系统重组能力；长期成功仍以多个真实场景中的稳定内核、受控变化和无客户核心分支为依据。 当前下一阶段以参考 OA 完整业务范围作为首轮架构可行性验证：使用最小应用组合、显式依赖和 Customer Overlay 逐步覆盖 C1-C8，并以验证报告结束；下一轮参考应用保持待选。 | - |
| product_capabilities | settled | 51 | 当前已运行的数据所有者和 Gateway／Workbench 装配边界保持不变。C1 已完成 Project 责任、Project Package、客户页面和权限恢复；C2 Business Engagement 已有独立后端、D1／Workbench、授权和故障恢复；Party Directory 已有独立核心、D1 正式链、管理页和首个 Project 人类消费流；Billing 已有独立核心、D1、人类工作区和 C3 分段组合；Production Operations 已完成多类型、正式最小组合、权限生命周期、浏览器和空目标恢复。C5 Approval & Audit Results 现已验证领域核心、PostgreSQL 生命周期、受保护版本化 HTTP、原生 IAM 最终授权、精确 Project／Contract／Evidence adapter、按 Project 的人类工作区和 Project reference-oa-c5-v1 Completion Profile。Results 已进入正式 D1，具备独立数据库、工作负载身份、Suite Manifest、Workbench、Gateway 路由、未选缺席和联合空目标恢复。C5 真实独立进程批复→审计→Project 完成确认、撤权／双 Tenant、真实浏览器仍未验证，C6-C8、完整双 Tenant 和真实环境也未完成。客户装配不得产生客户核心分支、共享业务表、客户数据库或客户 migration。 | - |
| runtime_surfaces | settled | 1 | 完整组合的浏览器访问固定经过 Nginx 和内部 Gateway；业务应用保持独立 HTTP/CLI/运维入口，Evidence 另有浏览器直传 OSS 数据面。 | - |
| experience_and_interaction | settled | 10 | 完整组合继续提供统一登录和工作台；每个 Level A 独立 Demo 同时提供可重复的工程操作者入口和人类可读产品入口。产品入口统一投影为 Demo Overview、Records List 和 Record Workspace 三类页面角色，必须说明用途、演示边界、推荐路径、业务状态、结果、历史和错误恢复。页面写入口由目标应用返回的 allowed_actions 驱动；正式入口提供显式权限刷新，刷新或写命令失败后退出编辑态并给出重新授权／返回工作台路径。Billing 固定金额脱敏、409/503 恢复语义；Production 已以统一工作台入口呈现 design-v1 与 consultation-v1，并在未选时整体缺席。C5 Results 页面按 Project 提供登记、纠正、撤回和历史；Project Completion 页面区分纯读取评估、显式确认、drift、冲突、依赖不可用和权限变化，并明确检查点不是完整 OA 完成。Results 的正式 Workbench 入口和未选缺席已通过 D1 结构与入口验收；真实浏览器业务操作、撤权和账号生命周期仍待验证。模板 IAM 不能被关闭。最终客户品牌、Production 专项辅助技术矩阵及超出基础可用性的页面精修可以延期。 | - |
| visual_language | settled | 1 | 六个独立 Demo 采用克制的浅色业务工作台视觉基线：深色正文、中性分区、单一冷色交互强调、低圆角、细边框和标准偏紧凑密度；视觉层级主要依赖排版、留白与位置，状态不只依赖颜色，卡片只承载确有独立边界的摘要、反馈或帮助。该基线不是最终客户品牌，暗色与客户主题需另行确认。 | - |
| identity_and_access | settled | 18 | 所有受保护访问强制使用版本化 IAM 契约。IAM 拥有身份、会话、Capability Catalog、Grant、按 audience 的策略求值、短期用户／工作负载凭证、撤权和不可变审计，但不拥有业务事实；身份管理员无隐式业务权限。应用 audience 与 Capability namespace 可以显式不同；IAM、Gateway 与目标应用必须按目标 Catalog 的动作、Tenant、资源 Scope、本地状态和 revision 失败关闭。C1-C4 证据保持有效。C5 Approval & Audit Results 已验证 approval-audit-results audience、approval-audit-result 原生动作、精确 Project／Result Scope、金额／Evidence 字段级能力和目标最终 allowed_actions；Project 又增加 project-workspace.completion.read／confirm，并用精确 Project Scope 的 Results 工作负载凭证获取最小引用。页面只消费服务端 allowed_actions。C5 正式 D1 撤权／账号生命周期、完整双 Tenant 和真实浏览器收敛仍未验证。 | - |
| data_and_state | settled | 15 | 每个业务应用独占自己的数据库写入权与事实；跨应用只使用版本化 API、快照或事件，已确认事实以追加 revision、调整、冲销或反向记录演进。Project、Party Directory、Business Engagement、Billing、Production Operations 与 Approval & Audit Results 各自拥有已声明事实。Results 在独立 PostgreSQL schema 中拥有外部结果、整数最小货币单位金额、Project／来源 revision 快照、Evidence 引用、幂等命令、审计及追加式纠正／撤回历史。受保护 adapter 通过有界 owner API 复验精确来源；依赖不可用或无效时零部分写入。Project 以 reference-oa-c5-v1 只读消费 Results 最小引用，GET 不写库；显式确认只在 Project 库追加 Project revision、Completion Confirmation、History、Command 和 Audit，并冻结 Project expected revision 与精确 Results source revisions。来源后续纠正或撤回只显示 drift，不回写 Results 或改写历史确认；成功 Command 在 Results 不可用时先本地回放。Project 状态保持 active，C5 检查点不冒充完整 OA 完成。Contract／Money 本地 Party 目录在迁移验收前仍是现状；禁止双写、共享表、按名称自动合并、物理删除确认结果或改写历史。 | - |
| external_integrations | settled | 1 | 阿里云 OSS 通过独立 Evidence Service 的短期有界 STS、浏览器直传、服务端完成校验和短期签名访问接入；其他跨系统连接器只传递版本化来源事实和建议。 | - |
| feedback_and_support | open | 0 | TBD | - |
| commercialization_and_entitlement | deferred | 1 | TBD | - |
| technical_foundation | settled | 1 | 业务与装配服务以 Go、PostgreSQL 18、Docker/Compose 和版本化 HTTP 契约为当前技术基线；最低 D1 使用单机 Compose，业务端点必须配置注入。 | - |
| security_privacy_compliance | settled | 1 | 公网只暴露 Nginx 的 HTTPS 入口；凭据不得入库，IAM/STS/SSH 身份分离，OSS 保持私有并使用最小权限短期凭据，生产合规要求需单独验收。 | - |
| quality_and_validation | settled | 15 | 每个纵向闭环使用风险相称的可执行证据；Demo Validation 2 区分运行真实性、业务完整性和解耦适当性。六个 Level A、Level B 高保真组合、两个中间子组合、首次实际组合变化账目和无源码 clean-room 复演仍是回归基线。Customer Scenario Evidence v1 区分参考基线、单客户结果与跨客户结论。OA 完整体阶段要求 C1-C8、最小组合、Base／Overlay 变化、故障恢复、人类可用体验和至少一套真实 Ubuntu 进入同一报告。C5 已有核心、受保护后端、人类工作区／Project Completion 检查点和正式 D1 装配恢复四份报告；隔离 D1 已通过入口、未选缺席、联合备份、销毁卷恢复、恢复后复验和摘要校验。该证据明确排除真实批复→审计→Project 完成确认、权限生命周期、双 Tenant、真实浏览器和真实 Ubuntu。最终结论固定为 supported、conditionally-supported、not-supported 或 insufficient-evidence。 | - |
| delivery_and_distribution | settled | 15 | Suite Manifest 决定启用应用，Deployment Profile 决定放置；D1-D4 使用同一镜像、API、migration 和数据边界。目标机只装载或拉取不可变制品，不依赖 Docker Hub、源码或现场编译。当前 D1 发布清单包含十五个运行镜像引用、十二个独立数据库账号和十个可选业务应用开关；启用 Production 或 Approval & Audit Results 时，同一 Manifest 同步生成 Gateway 路由、Workbench 入口、migration、工作负载密钥和联合备份恢复范围，关闭时路由与入口一起消失且不删除数据。隔离 D1 已分别证明 Production 与 Results 的正式入口、未选应用缺席和销毁卷后的空目标恢复；Results 恢复后再次通过同一入口验收。个人环境只作可重建试验田；公司环境必须重建 Registry、ECS、域名／证书、OSS、RAM、秘密和备份。OA 完整体 supported 结论仍要求同一发布物在获授权真实 Ubuntu 完成完整入口、升级、离机备份、空目标恢复和继续写入。 | - |
| observability_and_operation | settled | 1 | 运行组件提供健康检查和受限日志；D1 升级前联合备份、只向空目标恢复并验证继续写入，单机档明确是单点且不宣称高可用。 | - |

## Software Invariants

- product-expectations-remain-recoverable: Every materially affected product expectation is accurate, unambiguous, and durably recoverable.
- interaction-expectations-remain-recoverable: Every materially affected interaction expectation is coherent, complete enough to recover its decisions and states, and durably recoverable.
- visual-language-remains-consistent: Every materially affected visual expectation remains intentional, internally consistent, and durably recoverable.
- technical-decisions-remain-explainable: Every materially affected technical decision remains coherent, explainable, and durably recoverable, including its rationale and affected relationships.
- accepted-facts-are-realized: The accepted software state realizes every materially relevant accepted fact and upheld Project decision and invariant.
- material-risks-have-credible-evidence: Every material risk claim accepted in the Case is supported by credible, repeatable, and proportionate evidence.

## Read For Precision

- state.record.json
- arckit/cases/active/CASE-20260923-001-d1-https.md
- arckit/cases/active/CASE-20260923-003-d1.md
- arckit/cases/active/CASE-20260925-010-second-customer-scenario-intake-readiness-gate.md
- arckit/cases/active/CASE-20260925-013-oa.md
