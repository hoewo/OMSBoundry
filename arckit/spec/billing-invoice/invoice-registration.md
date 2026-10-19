# Billing / Invoice：已开票登记

## 元信息

- 路径：`spec/billing-invoice/invoice-registration.md`
- 所属模块：Billing / Invoice
- 状态：🟡 实现中；第一版产品边界、完整生命周期、可移植交付、D1 正式入口、联合恢复、人类页面与 C3 本地 D1 组合已完成，完整双 Tenant、真实浏览器最终点击、Billing-only 与真实 Ubuntu 尚未完成
- 优先级：P0
- 上层旅程：[OA 完全体](../assembly/full-reference-oa-validation.md) 的 C3
- 关联决策：BF-003、BF-005～BF-007、BF-009～BF-010、BF-016～BF-017、BF-036～BF-042、BF-049～BF-053
- 实施状态：规格✅ | 边界✅ | 模型契约✅ | API 契约✅ | 领域核心✅ | PostgreSQL 真库✅ | 上游目标端精确引用✅ | Billing 消费侧 resolver／最小 Policy✅ | 完整生命周期 HTTP✅ | Linux AMD64 非 root 交付✅ | 正式入口／D1✅ | 故障与联合恢复✅ | 页面✅ | C3 本地 D1 组合✅ | 双 Tenant／真实环境补证❌

## 问题与可验证结果

**问题 A**：已开票事实若继续与应收、实际收款、奖金、成本和客户专用统计共存在宽财务记录中，票据纠错会改写资金事实，Money Flow 也会被迫承担不属于资金台账的票据生命周期。

**可验证结果 B**：Billing / Invoice 独立登记我方已经开具的票据及其业务引用、金额、日期、Evidence 引用、revision 和历史；Money Flow 独立拥有应收义务、实际资金和核销，Contract 独立拥有合同与结算，C3 组合通过版本化引用展示开票、应收和回款结果，而不共享表或互相改写事实。

## 证据边界

参考 OA 的当前规格与实现直接支持以下行为：

- 用户记录我方已经开具的票据，至少包含开票日期和金额；
- 票据可以关联一个或多个发票文件，并在项目经营页面和公司票据查询中查看；
- 已登记信息可以被纠正，文件可以在上传失败后单独重试，已经录入的开票信息不因此丢失；
- 参考 OA 同时展示两段经营链中的开票与收款，但票据和付款是不同记录；
- 票据查询支持项目、时间和业务费用上下文筛选；
- 金额、票据文件和经营操作具有独立权限语义，身份管理员不天然拥有这些业务权限。

参考 OA 当前实现中的宽 `FinancialRecord`、二进制浮点金额、物理删除、可原地覆盖和票据／付款一对一约束只证明现状，不进入 BizFoundry 稳定 Base。以下语义没有足够证据，因此不在第一版中推断：

- 红字发票、红冲、作废、退票、换票和跨期开票更正的法律或会计生命周期；
- 发票号码、税额、税率、购销方税号、发票种类和电子票据平台同步是否为必需字段；
- 一张发票与多笔应收／回款、一笔回款与多张发票的正式匹配规则；
- 退款或退回款是否以及如何反向影响票据；
- 票据审批、开票申请、额度控制、税务申报和会计凭证责任；
- “开票完成”是否推动 Project、Contract 或 Business Engagement 状态。

## 第一版稳定范围

### Billing / Invoice 拥有

- Tenant 内稳定 Invoice Registration ID、连续 revision 和当前登记状态；
- 已开票日期、精确金额、币种、备注和可信 Actor；
- 开票方与受票方的 Party ID、Party revision 及业务发生时最小快照；
- 可选 Project、Contract、Settlement 或 Money Obligation 的类型化来源引用及来源 revision；
- Evidence File ID／Version 的票据业务含义引用；
- 登记、资料纠正、登记无效、Evidence 关联变化、命令幂等和不可变历史；
- 服务端根据凭证、资源范围、当前状态和 revision 计算的 `allowed_actions`。

### Billing / Invoice 只引用

- Party Directory 的当前 Party 档案与精确 revision；
- Project Workspace 的 Project ID、Tenant、状态和 revision；
- Contract Ledger 的 Contract／Settlement ID、金额摘要和 revision；
- Money Flow Ledger 的 Obligation／Cash／Allocation ID 与只读摘要；
- Evidence Service 的 File ID／Version；
- IAM Capability Credential 中的 Subject、Tenant、Audience、Action、Scope、凭证 ID 和有效期。

来源应用不可用时，已经冻结的来源快照和登记历史继续可读；需要验证新鲜来源 revision 的新登记或纠正失败关闭。

### Billing / Invoice 不拥有

- 合同条款、合同有效金额、结算或法律状态；
- 应收／应付义务、实际资金、核销、余额、退款或反向资金事实；
- Party 当前档案、人员名册、账号、项目 Assignment 或客户岗位；
- 文件字节、对象键、摘要、上传会话、STS 或签名 URL；
- 奖金、成本、外委、管理费、税务申报或会计凭证；
- Project、Contract、Money Flow 或 Business Engagement 的完成状态；
- 通用审批、万能财务记录或客户专用宽表。

## 登记事实与现实票据状态分离

第一版管理“系统对一张已开具票据的登记”，不冒充完整税务票据生命周期：

| 登记状态 | 含义 | 允许的核心动作 |
| --- | --- | --- |
| `recorded` | 系统保存一张已经发生的开票事实 | 查看、纠正登记资料、管理 Evidence 引用、标记登记无效 |
| `registration_invalidated` | 该系统登记被确认是误录、重复或不应参与当前业务投影 | 查看与历史；不得被汇总为有效开票金额 |

- `registration_invalidated` 只否定系统登记，不表示现实发票已经红冲、作废、退票或撤销；
- 资料纠正追加 revision，不原地覆盖旧金额、日期、Party 快照或来源引用；
- 登记无效必须记录原因、Actor、Expected Revision 和发生时间；
- 若未来确认现实票据状态机，应增加独立、明确命名的状态与命令，不复用 `registration_invalidated` 偷换含义；
- 有效开票金额只汇总当前状态为 `recorded` 的最新 revision，并能下钻到每条登记。

## 金额与来源规则

- 金额使用精确十进制定点表示，固定保存币种，不使用二进制浮点；
- 开票金额必须大于零；金额纠正形成新 revision 并保留原值与原因；
- 第一版不要求开票金额等于合同、结算或义务金额，也不自动截断超额登记；差异作为可见校验结果返回给用户；
- 来源引用包含应用、资源类型、资源 ID、来源 revision 和最小不可变摘要；
- 手工登记必须显式标记来源种类为 `manual_confirmation` 并保存确认人和说明，不能伪装成 Contract 或 Money Flow 自动生成；
- 一张登记可以引用多个来源，但第一版不建立票据与付款的一对一约束；
- Contract、Money Flow 或 Project 的后续变化不静默改写既有票据登记。

第一版引用可用状态固定为显式白名单，未知状态失败关闭：Party 只接受 `active`；Project 接受 `active`、`pending_completion`；Contract 接受 `active`、`terminated`、`closed`；Settlement 只接受 `recorded`；Money Obligation 接受 `open`、`closed`；Evidence File Version 只接受 `available`。这些状态只回答“能否作为登记来源被冻结”，不授权 Billing 改变上游状态，也不把上游状态复制成 Billing 生命周期。

## Party 与 Evidence 引用

- 新登记要求开票方和受票方均为同 Tenant 的 active Party，并保存 Party ID、精确 revision、类型、名称和登记标识快照；
- Party 后续更名、停用或恢复不改变既有票据快照；
- Evidence 文件不是登记成功的强制前提；上传失败时票据登记保持 `recorded`，页面保留文件意图并允许以新命令重试；
- 关联 File Version 前必须验证同 Tenant、状态可用和精确 revision；
- 替换或移除 Evidence 引用追加 Billing revision，不删除 Evidence 字节；
- Evidence 不可用时，既有 File ID／Version 和业务含义仍可解释，新的文件确认返回明确可重试失败。

## C3 组合关系

C3 的最小连续结果由三个事实所有者组成：

1. Contract Ledger 的合同／结算，或用户的明确人工确认，提供应收来源；
2. Money Flow Ledger 独立确认 Obligation，登记 Cash Transaction 并通过 Allocation 形成余额；
3. Billing / Invoice 独立登记已开票事实及 Evidence 引用。

Workbench 或 Project 只读投影可以把三类结果放在同一项目页面，但必须显示来源应用、资源 ID、revision、状态和不可用原因。组合层不得：

- 直接联表计算开票、应收和回款；
- 让 Billing 创建或修改 Money Obligation／Cash／Allocation；
- 让 Money Flow 把附件或描述冒充发票；
- 用“开票金额等于已收金额”推导付款成功；
- 用客户端合计替代服务端来源摘要和下钻。

第一版 C3 允许人工选择来源引用和人工确认义务，不建立 Contract → Money 或 Billing → Money 自动连接器。连接器只有在来源、目标确认、幂等、拒绝、重试和纠错规则得到独立证据后才进入后续范围。

## 用户场景

### US-1：登记已开票事实 [P0]

- **Given** 开票方与受票方 Party 引用有效，操作者对目标 Project 或来源资源具有登记权限；
- **When** 用户提交开票日期、精确金额、币种、Party 快照和可选来源引用；
- **Then** 系统形成稳定 ID、revision 1、`recorded` 状态和不可变历史，不创建现金或核销事实。

### US-2：补充或替换票据文件 [P0]

- **Given** 票据登记存在且 Evidence File Version 可验证；
- **When** 用户登记、替换或移除文件引用；
- **Then** Billing 追加 revision 并保留旧引用历史，Evidence 字节和技术状态仍由 Evidence Service 管理。

### US-3：纠正登记资料 [P0]

- **Given** 用户发现日期、金额、Party 快照、来源引用或备注录入错误；
- **When** 用户以当前 Expected Revision 和必填原因提交纠正；
- **Then** 系统追加下一 revision，历史保留原登记，相关投影按最新有效 revision 重算。

### US-4：标记错误登记无效 [P0]

- **Given** 票据登记为误录、重复或不应参与当前业务投影；
- **When** 有权限的用户以当前 Expected Revision 和原因标记登记无效；
- **Then** 系统进入 `registration_invalidated` 并从有效开票金额中排除，但不声称现实票据已红冲或作废。

### US-5：查看 C3 开票、应收与回款 [P0]

- **Given** Contract／人工确认、Money Flow 和 Billing 分别存在正式事实；
- **When** 用户从项目经营入口查看 C3 摘要；
- **Then** 页面分别展示有效开票、应收、已收和余额，允许下钻到各事实所有者，并在任一来源不可用时明确降级而不返回伪造的零值。

## 权限与身份

第一版稳定 Capability 使用领域动作，不使用“财务”“经营负责人”等客户岗位名：

- `billing-invoice.read`：读取非敏感登记元数据；
- `billing-invoice.amount.read`：读取金额和金额汇总；
- `billing-invoice.record`：登记已开票事实；
- `billing-invoice.correct`：纠正当前登记资料；
- `billing-invoice.registration.invalidate`：标记系统登记无效；
- `billing-invoice.evidence.manage`：登记、替换或移除 Evidence 引用；
- `billing-invoice.history.read`：读取不可变 revision 历史。

Customer Overlay 把财务、经营经理、项目经营负责人、总经理只读等客户角色映射为这些稳定动作；目标应用不认识客户职位名称。身份超级管理员默认没有 Billing 业务动作。

受保护访问固定经过 Nginx、Gateway、IAM／Policy 和 Billing：Nginx 不解释权限；Gateway 清除客户端身份 Header；IAM／Policy 根据显式 Grant、Project Assignment 和版本化策略签发 Billing 原生短期凭证；Billing 复验 Tenant、Audience、Action、Scope 和有效期，再结合 Invoice／Project、当前状态和 Expected Revision 作最终授权。

当前实现已经把 Billing 原生 IAM／模板 Provider、PostgreSQL Store、领域服务和类型化来源 resolver 装入可配置进程，开放创建、列表、详情、纠正、登记无效、Evidence 管理、历史和工作负载只读引用。Project Scope 创建要求每个 Project 来源均获授权；列表必须显式匹配 `project_id`；来源纠正不能越权换挂相邻 Project；工作负载引用只接受单 action、单精确登记 Scope。真实 PostgreSQL 18 联调已覆盖 revision 1→4、Evidence 精确引用、不可变历史、服务端条目／汇总金额收敛及登记无效后有效汇总归零。Linux AMD64 非 root 镜像、pull-only release、Nginx→Gateway→IAM 正式签发、D1、Party 故障零写入／恢复重试和联合空目标还原也已通过。人类工作区现已进入 Workbench，并验证服务端 `allowed_actions`、金额撤权／恢复、历史脱敏、盲写金额拒绝、409 和依赖故障恢复。Project 的 C3 组合实现已通过类型化契约、精确工作负载凭证、并发与局部降级自动测试，但 D1 真实链路、完整角色／账号停用／双 Tenant 与 Billing-only 矩阵仍未执行，不能把实现就绪当成完整 C3 通过。

金额与文件动作分别判权。完整金额只存在 Billing 自有数据库事实和服务端内存中；API 必须使用权限收敛视图，没有 `billing-invoice.amount.read` 时返回显式 `amount_visible=false` 且 `amount_minor`／`currency` 均为 `null`，不得返回 `0`、空串、真实金额或依赖前端遮盖。历史响应的每个快照应用同一规则，不能从旧 revision 泄露金额。没有 `billing-invoice.evidence.manage` 时读取权限不允许替换文件。页面 `allowed_actions` 与服务端命令使用同一动作名，旧页面按钮和直接 API 请求不能绕过。

## 最小组合与故障边界

- 独立票据组合：IAM／同形本地 Provider + Party Directory + Billing / Invoice + 独立 PostgreSQL；Evidence 是文件能力的显式直接依赖；
- Project、Contract 和 Money Flow 不安装时，Billing 仍能登记双方 Party、日期、金额和文件，并使用 `manual_confirmation` 来源；
- C3 参考 OA 组合增加 Project、Contract 和 Money Flow，但各自数据库、migration、备份和历史保持独立；
- Party 不可用时，新登记和需要 Party 纠正的命令失败关闭，既有票据快照可读；
- Evidence 不可用时，票据元数据仍可登记和读取，文件关联命令明确失败并可重试；
- IAM／Policy 不可用时，高风险新授权失败关闭，已签凭证只在既定 TTL 和 Scope 内继续；
- Contract、Money Flow 或 Project 不可用时，既有来源快照仍可读，需验证新鲜来源的命令失败关闭；
- Billing 不可用时，Contract 和 Money Flow 继续完成各自闭环，组合页面只把票据能力标为不可用。

## 人类工作区

- Suite Manifest 启用并显示 Billing 时，统一工作台提供正式入口；Gateway 以当前 IAM Session 签发 Billing 原生短期凭证后进入 `/billing/`。
- 左侧列表显示状态、双方 Party、日期、revision 和有权金额；列表级 `allowed_actions` 决定“登记已开票”，详情级动作分别决定纠正、Evidence、登记无效与历史。
- 页面完整支持新登记、详情、纠正、Evidence 引用集合替换、登记无效和历史读取；成功后保持当前登记并重新读取当前投影与汇总。
- 缺少 `billing-invoice.amount.read` 时，工作区明确显示金额已收敛，不把未知渲染为零，不显示金额纠正输入，也不从历史泄露。
- 401／403 立即退出编辑态并刷新权限；409 读取最新 revision 但不自动重放；422 提示核对引用；503 保留输入并允许同负载沿用原 Command ID 重试。
- 独立开发入口允许本标签页模板 Token；正式 D1 入口只依赖 Gateway 设置的 HttpOnly Capability Cookie，浏览器页面不保存密码或长期凭证。

## 第一版验收门槛

1. 空库能够登记双方 Party、日期、精确金额和币种，形成 revision 1 与不可变历史。
2. 相同 Command ID 同负载重放返回原结果；改变负载或 Actor 返回幂等冲突；并发相同命令只写一次。
3. 资料纠正和登记无效要求当前 Expected Revision；旧 revision 返回 409 且不产生部分写入。
4. `registration_invalidated` 从有效汇总中排除，但历史仍能解释原金额和无效原因。
5. Evidence 上传失败不删除票据登记；恢复后用新命令关联文件只追加一次 revision。
6. Party、Project、Contract、Money 和 Evidence 精确引用拒绝错 Tenant、错资源、stale revision 与不可用状态。
7. 允许角色、相邻角色拒绝、Project B 拒绝、跨 Tenant 拒绝、状态拒绝、撤权、账号停用和伪造 Header 均有无部分写入证据。
8. 金额读取、文件管理和登记纠错分别使用稳定动作；身份管理员没有隐式业务权。
9. C3 页面分别展示开票、应收、已收和余额并支持来源下钻；来源故障显示明确降级，不显示伪造零值。
10. Billing-only 最小组合不安装 Project／Contract／Money 仍可运行；C3 组合逐项停止 Billing、Contract、Money 后的影响符合所有权边界。
11. 独立 migration、release、非 root 容器、备份、空目标恢复、恢复后继续写入和 Suite Manifest 开关通过。
12. 人类工作区可从 Workbench 进入，并完成列表、登记、详情、纠正、Evidence、登记无效和历史；金额收敛、409、503 与撤权恢复可被观察。
13. 既有六个 Level A、J1～J4、C1、C2 和 IAM／Gateway 安全回归保持通过。

## 明确不负责

- 不复制参考 OA 的宽 `financial_records` 表、物理删除或原地覆盖行为；
- 不把票据、义务、付款、成本、奖金、管理费和客户统计合并为万能财务域；
- 不把参考 OA 当前一对一票款关联升级为 Base 规则；
- 不实现未经确认的红冲、作废、退票、换票、跨期更正、税务平台或审批；
- 不用票据文件存在推导票据合法有效，也不用开票金额推导已收款；
- 不在 Project、Contract、Money Flow 或 Workbench 中建立第二套可写票据事实；
- 不因连续页面需要而共享数据库、复用 ORM 或允许客户职位进入目标应用判断。

## 来源依据

- 参考 OA `docs/OA_2026_REQUIREMENTS_CONFIRMED.md`：回款提醒仍未确认，不能作为 C3 第一版条件；
- 参考 OA `arckit/spec/business/finance.md`：已开票日期、金额、文件、纠正、查看及文件重试行为；
- 参考 OA `backend/internal/models/financial_record.go`：当前宽财务记录及两段经营链实现事实；
- 参考 OA `backend/internal/services/financial_service.go`：票款关联、文件关联、编辑／删除和汇总的当前行为及其边界风险；
- BizFoundry `docs/foundation.md`：事实所有权、精确金额、revision、纠错、IAM 和跨应用契约边界；
- BizFoundry `arckit/spec/assembly/full-reference-oa-validation.md`：C3 目标、未知项和完全体验收要求。
