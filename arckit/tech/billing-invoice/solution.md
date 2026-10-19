# Billing / Invoice 技术方案

状态：🟡 已采用并部分实现；独立核心、PostgreSQL migration／Repository、Capability Manifest、目标端 IAM 凭证复验、权限视图、上游精确 reference-read、消费侧工作负载 resolver、完整本地生命周期 HTTP、Linux AMD64 非 root 发布、D1 正式入口、故障隔离、联合恢复、Billing 人类工作区与 Workbench 入口以及 C3 本地 D1 分段只读组合已完成；完整登录态浏览器矩阵、完整双 Tenant、Billing-only 和真实 Ubuntu 尚未完成

日期：2026-09-27

## 方案目标

Billing / Invoice 使用独立 Go 服务和独立 PostgreSQL 数据库登记“我方已经开具的票据”。它拥有登记事实、精确金额、Party／来源快照、Evidence 引用、revision 和不可变历史，但不拥有应收义务、实际资金、核销、合同或现实票据的红冲／作废／退票状态。

第一条可执行纵向切片必须同时证明：

- Billing-only 最小组合可用，不要求 Project、Contract 或 Money Flow 启动；
- C3 组合能把开票、应收、已收和余额只读聚合，并能下钻到各事实所有者；
- 所有受保护请求经过 Nginx、Gateway、IAM／Policy，再由 Billing 最终授权；
- 身份管理员没有隐式票据业务权，金额、文件、纠正和历史分别判权；
- 纠正、登记无效、幂等、依赖失败和恢复都不产生半写或伪造的资金事实。

## 运行边界

```text
Browser
  → Nginx (TLS / edge)
  → Gateway (session boundary / header cleanup / static route)
  → IAM / Policy (Billing-native short credential)
  → Billing / Invoice
      → billing database

Billing command
  → workload assertion
  → IAM credential exchange
  → exact Party / Project / Contract / Money / Evidence credential
  → source reference API
  → Billing-owned immutable snapshot and revision
```

- Nginx 只终止 TLS、执行边缘限制和转发，不解释发票、金额或客户岗位。
- Gateway 删除客户端身份 Header，只接受可信 Suite Session／Token，并把 `/billing` 与 `/api/v1/invoice-registrations` 路由到 Billing。
- IAM／Policy 只依据显式 Grant、Customer Overlay 和版本化 Business Fact 签发 `aud=billing-invoice` 的短期凭证，不拥有票据登记。
- Billing 复验签名、Issuer、Tenant、Audience、Action、Scope、TTL、Credential ID，再结合当前状态、资源归属和 Expected Revision 最终授权。
- Billing 不读取其他应用数据库、不复用其 ORM／领域对象，也不把外部当前投影保存为第二个可写目录。
- D1～D4 使用相同镜像、migration、API 和数据边界；端点全部由配置注入。

## 数据模型

### 当前投影 `invoice_registrations`

`InvoiceRegistration.yaml` 是 Tenant 内当前有效解释。主要字段包括稳定 ID、状态、开票日期、精确金额 minor units、币种、双方 Party 快照、类型化来源快照、Evidence 引用、revision、Actor 和时间。

主要约束：

- `(tenant_id, id)` 唯一，revision 从 1 单调递增；
- 状态只允许 `recorded`、`registration_invalidated`；
- `amount_minor > 0`，currency 使用 ISO 4217 三字母大写代码；
- 开票方与受票方必须不同且来自同 Tenant 的 active Party；
- 来源类型只允许 `manual_confirmation`、`project`、`contract`、`settlement`、`money_obligation`；
- 来源快照只保存稳定 ID、source revision、显示摘要和确认信息，不保存外部完整模型；
- Evidence 引用只保存 File ID、Version、业务含义和显示名，不保存对象键、签名 URL 或字节；
- 不提供物理删除命令。

金额第一版使用整数 minor units 加 currency。该方案不假定所有货币都是两位小数；显示和输入换算必须依赖版本化货币元数据。核心服务要求装配时提供非空允许币种集合，未配置的币种失败关闭；参考 OA 第一切片只配置 `CNY`，增加币种前必须先固定其指数元数据。

### 不可变 `invoice_registration_revisions`

`InvoiceRegistrationRevision.yaml` 以 `(tenant_id, invoice_registration_id, revision)` 唯一，保存每次完整登记快照、事件类型、原因、Command ID、可信 Actor 和发生时间。数据库触发器拒绝 UPDATE／DELETE。

事件类型只有：

- `recorded`：首次登记；
- `details_corrected`：日期、金额、Party／来源快照或备注纠正；
- `evidence_changed`：Evidence 引用增加、替换或移除；
- `registration_invalidated`：误录、重复或不应进入当前投影；

这些事件均不表示现实票据已经红冲、作废、退票或换票。未来若取得法律／会计证据，必须增加独立命名的模型、状态和命令，不能复用当前字段偷换含义。

### 命令与审计

`invoice_registration_commands` 保存 Tenant、Command ID、负载摘要、Actor、目标 ID、来源／结果 revision 和序列化结果。Tenant + Command ID 唯一：同摘要重放返回原结果，同 ID 不同摘要返回 `409 COMMAND_PAYLOAD_CONFLICT`。

当前投影推进、revision 追加、命令结果和业务审计必须在一个 PostgreSQL 事务提交。外部引用校验在事务前执行；进入本地事务后锁定当前登记、校验 Expected Revision，再写本地事实。外部服务超时或拒绝时不创建成功命令记录，不消耗 revision。

## API Port

| 契约 | 用途 | Capability |
| --- | --- | --- |
| `invoice-registration-create.yaml` | 登记 revision 1 | `billing-invoice.record` |
| `invoice-registration-list.yaml` | 稳定查询和汇总入口 | `billing-invoice.read`；金额另验 `billing-invoice.amount.read` |
| `invoice-registration-get.yaml` | 读取详情和允许动作 | `billing-invoice.read` |
| `invoice-registration-correct.yaml` | Expected Revision 纠正 | `billing-invoice.correct` |
| `invoice-registration-invalidate.yaml` | 标记系统登记无效 | `billing-invoice.registration.invalidate` |
| `invoice-registration-evidence.yaml` | 增加、替换或移除文件引用 | `billing-invoice.evidence.manage` |
| `invoice-registration-history-list.yaml` | 读取不可变历史 | `billing-invoice.history.read` |
| `invoice-registration-reference-get.yaml` | 向只读组合层提供精确摘要 | `billing-invoice.reference.read` |

`InvoiceRegistration.yaml` 与 `InvoiceRegistrationRevision.yaml` 是 Billing 内部完整事实；对外列表、详情、写命令结果和历史分别使用 `InvoiceRegistrationView.yaml` 与 `InvoiceRegistrationRevisionView.yaml`，不能把数据库结构直接序列化给调用者。视图根据当前 Credential、Scope、状态及金额／文件权限返回 `allowed_actions`。缺少 `billing-invoice.amount.read` 时固定返回 `amount_visible=false` 且金额／币种为 `null`，而不是用 `0`、空串、真实值或客户端遮盖；历史中的所有 snapshot 使用同一收敛规则。所有写端点仍复验原生 Capability、Scope、状态和 Expected Revision，前端显隐不构成授权。

## 来源验证端口

### Party

每个新登记和涉及 Party 的纠正都使用工作负载身份交换两个独立的 `party-directory.reference.read + party:{id}` 短期凭证，读取精确 revision。Billing 保存 Party ID、revision、kind、legal name 和登记标识最小快照；Party 后续更名、停用或恢复不改写历史登记。

### Project、Contract 与 Money

类型化来源引用分别调用目标应用的 reference-read：

- `project`：Project Workspace 的精确 Project reference；
- `contract`／`settlement`：Contract Ledger 的精确 Contract／Settlement reference；
- `money_obligation`：Money Flow 的精确 Obligation reference。

每次只请求单 audience、单 action、单资源 Scope、最长 60 秒凭证。来源 API 必须返回 Tenant、稳定 ID、revision、状态和最小摘要。Billing 不通过一个万能聚合 Token 读取多个应用，也不自动创建义务、现金或核销。

`manual_confirmation` 不调用外部来源，但必须保存确认 Actor、说明和确认时间，不能伪装成系统自动来源。

### 上游引用兼容性门禁

Billing 的类型化来源 HTTP adapter 只有在目标应用同时满足精确资源 Credential 和最小响应契约时启用。目标端兼容层现已完成：

- Party Directory 与 Evidence Service 接受精确 `party:{id}`／`file:{id}` Scope，并返回 Billing 所需的最小快照字段；
- Project Workspace 保留 `project-workspace.read + project:{id}`，Project reference 追加事实所有者给出的 `display_summary`；
- Contract Ledger 发布 `contract-ledger.reference.read`，只接受一个 `contract:{id}` 或 `settlement:{id}`，Contract 与新增 Settlement endpoint 均返回最小摘要；
- Money Flow Ledger 发布 `money-flow-ledger.reference.read`，只接受一个 `obligation:{id}` 或 `transaction:{id}`，Obligation／Transaction reference 返回最小摘要；
- 目标端测试覆盖相邻资源、错误类型、错误动作和跨 Tenant 拒绝；普通 Tenant Scope `view` 不能代替工作负载 reference action。

Settlement 是追加且不可变的独立事实，每个 Settlement 记录自身在引用契约中固定为 revision 1；其 `source_revision` 和序列仍是 Contract 内部历史解释，不被 Billing 重定义。以上变化没有让 Billing 获得 Tenant Scope 万能读取，也没有跨库查询、领域对象导入或本地拼接摘要。

Billing 消费侧现已建立独立 `billing-invoice` 工作负载身份 Policy 上限，并由类型化 HTTP resolver 兼容共享 `workload-identity` Token Source。每次解析只交换一个 audience、一个 action 和一个精确资源：`party-directory.reference.read + party:{id}`、`project-workspace.read + project:{id}`、`contract-ledger.reference.read + contract|settlement:{id}`、`money-flow-ledger.reference.read + obligation:{id}`、`evidence-service.view + file:{id}`。resolver 复验 Tenant、类型、ID、revision、稳定摘要和状态白名单；401 只失效对应缓存并重换一次。404／422 返回 `REFERENCE_INVALID`；Credential 交换失败、401 重试失败、403、网络、5xx、畸形、尾随 JSON和超过 1 MiB 的响应返回 `REFERENCE_SERVICE_UNAVAILABLE`。

可冻结状态白名单为：Party `active`；Project `active|pending_completion`；Contract `active|terminated|closed`；Settlement `recorded`；Money Obligation `open|closed`；Evidence `available` 且 response revision 等于 File Version。未知状态失败关闭。服务在解析外部引用前先检查本地 Command 重放；失败不写成功 Command、登记或 revision，恢复后原 Command 可安全重试，成功后再次重放不调用依赖。

完整本地生命周期 HTTP 现已开放创建、列表、详情、纠正、登记无效、Evidence 管理、历史和工作负载只读引用，并把原生 IAM／模板 Provider、PostgreSQL Store、领域服务和上述 resolver 组合在同一条实际请求链中。创建只接受 Tenant Scope，或至少包含一个 Project 来源且所有 Project 来源均落在 Credential Project Scope 内；Project Scope 列表必须显式匹配 `project_id`；来源纠正要求每个 Project 引用均在 Scope 内；精确登记／Project Scope 的详情、写命令和历史由 Billing 最终授权。工作负载引用只接受 IAM 单 action、单精确 `invoice_registration:{id}` Scope。所有外来身份 Header 在鉴权前清除；严格 JSON 上限为 1 MiB；当前、列表、汇总及每条历史 revision 都通过服务端视图收敛金额。真实 PostgreSQL 18 集成测试证明相邻角色／Project、跨 Tenant 和错误来源状态失败关闭，503 不写四类本地事实，恢复后同 Command 只写一次，并完成 correction→evidence→invalidation 的 revision 1→4 和无效后汇总归零。该结论是 `billing-local-lifecycle-runtime-compatible`，仍不是正式 IAM／Policy 签发、Gateway、D1、恢复或 C3 通过。

### Evidence

文件关联命令使用 `evidence-service.view + file:{id}` 精确凭证验证不可变 File Version。Billing 只保存引用；上传会话、STS、对象摘要和物理清理由 Evidence Service 拥有。Evidence 故障不阻止无文件的票据元数据登记，但文件关联命令返回可重试 `503 REFERENCE_SERVICE_UNAVAILABLE`，不产生 Billing revision。

## IAM、资源范围与最终授权

Capability Manifest audience 固定为 `billing-invoice`。第一版发布以下用户动作：

- `billing-invoice.read`
- `billing-invoice.amount.read`
- `billing-invoice.record`
- `billing-invoice.correct`
- `billing-invoice.registration.invalidate`
- `billing-invoice.evidence.manage`
- `billing-invoice.history.read`

工作负载专用 `billing-invoice.reference.read` 不映射给普通客户角色。用户凭证最长 300 秒，工作负载凭证最长 60 秒。

支持的用户 Scope：

- Tenant Scope：创建、列表和被 Overlay 明确允许的全租户操作；
- `project:{id}`：只允许来源或快照属于该 Project 的登记；
- `invoice_registration:{id}`：精确读取或写入单条登记。

Customer Overlay 只把客户岗位映射到上述稳定动作；Project Assignment Policy 可以为 `project:{id}` 产生动作，但不得把 Project 角色名传给 Billing 判断。金额查看、文件管理、纠正和登记无效必须分别赋权。`identity_super_admin` 默认动作集为空；IAM 治理能力不能推导 Billing 业务能力。

Billing 最终授权至少检查：

1. Credential audience 与请求动作精确匹配；
2. Tenant 与目标登记、来源及依赖引用一致；
3. Scope 覆盖目标 Invoice Registration 或 Project；
4. 当前状态允许命令；
5. Expected Revision 等于当前 revision；
6. 涉及金额或文件时存在对应细粒度动作；
7. 来源引用仍满足该命令要求的精确 revision 和可用状态。

Gateway 不扩展动作或 Scope，不缓存业务授权结论。客户端自带的 Tenant、Actor、Role、allowed actions 和身份 Header 一律在入口被删除。

## 并发与错误语义

- `401 UNAUTHENTICATED`：缺失、过期、签名错误或账号已不可用；
- `403 CAPABILITY_DENIED`：缺少目标应用原生动作；
- `403 SCOPE_DENIED`：Tenant、Project 或登记资源范围不匹配；
- `404 INVOICE_REGISTRATION_NOT_FOUND`：目标不存在或不可向调用者暴露；
- `409 REVISION_CONFLICT`：Expected Revision 过期，返回最新可见投影且零写入；
- `409 COMMAND_PAYLOAD_CONFLICT`：Command ID 被不同负载或 Actor 重用；
- `409 REGISTRATION_INVALIDATED`：无效登记不允许继续纠正或管理 Evidence；
- `422 NO_EFFECTIVE_CHANGE`：纠正没有有效变化；
- `422 REFERENCE_INVALID`：来源错 Tenant、stale、inactive 或类型不匹配；
- `503 REFERENCE_SERVICE_UNAVAILABLE`：依赖不可用且本命令要求新鲜引用；
- `502 AUTHORIZATION_SERVICE_UNAVAILABLE`：入口无法从 IAM／Policy 得到新授权。

任何 401／403／409／422／502／503 都不得追加 revision、成功命令或业务审计。明确 retryable 的 502／503 可以在依赖恢复后以原 Command ID 重试，因为失败没有占用成功幂等记录；新的业务意图必须使用新 Command ID。409 不自动重放旧命令。

## C3 只读组合

Workbench／Project C3 页面不建立可写聚合数据库。组合后端或 BFF 通过各应用只读 reference／summary 契约并行读取：

- Contract／人工确认：应收来源；
- Money Flow：义务、已收、核销和余额；
- Billing：有效开票和登记状态。

响应为分段结果，每段携带 `source_application`、资源 ID、revision、状态、观测时间和错误状态。某来源不可用时只降级对应分段，不能把未知渲染为零，也不能用上一次缓存结果冒充当前事实。组合层只导航到各事实所有者执行写动作。

第一版不建立 Contract → Money、Contract → Billing 或 Billing → Money 自动连接器。后续连接器必须另行证明目标确认、Outbox、幂等、拒绝、重试、纠错和版本兼容。

## 浏览器工作区与正式入口

Billing 进程公开静态工作区根页面和同源 `/assets/demo.js`，业务 API 仍全部经过原有 `Protect` 鉴权。CSP 只允许同源脚本与连接、内联样式并拒绝 frame；页面本身不携带业务事实，因此可以先渲染恢复入口，任何列表、详情或写入仍需要有效能力凭证。

页面在独立模式使用当前标签页 `sessionStorage` 中的模板 Token；在 Suite 路径下根据路由前缀请求 `/billing/api/...`，由 Gateway 删除客户端 Authorization／身份 Header，并把 HttpOnly Billing Capability Cookie 转换为上游 Bearer Token。权限刷新只调用 `/gateway/authorize/billing-invoice`，不自行构造 Capability。Suite Manifest 的 Billing `entry_action` 固定为 `billing-invoice.read`，启用后默认显示在 Workbench。

列表与详情只消费服务端 `allowed_actions`。金额显示依赖服务端 `amount_visible` 和 nullable 金额字段；无金额能力时 JavaScript 不建立金额纠正字段。页面为每次新业务意图生成 Command ID；409 不自动重放，503 则保留完整请求并用原 Command ID 重试同一意图。写入成功后重新读取列表、详情和动态动作，避免沿用旧状态判断。

## 部署与恢复

Suite Manifest 使用显式 `ENABLE_BILLING`。Billing-only 最小组合需要 IAM／同形本地 Provider、Party Directory、Billing 和独立 PostgreSQL；Evidence 只在启用文件能力时是直接依赖。C3 组合可另外启用 Project、Contract 与 Money Flow。

D1 目标形态：

- 独立 migration 和 server 镜像键；
- 独立数据库、账号和最小权限；
- 只加入内部 app／data 网络，不发布宿主端口；
- Nginx 单公网入口，Gateway 静态 `/billing` 路由；
- 独立 Ed25519 工作负载密钥和版本化 Workload Policy；
- 联合备份记录启用选择并包含 Billing 数据库，独立恢复也可用；
- 空目标恢复保持 ID、revision、金额、快照、Evidence 引用、命令结果和审计，恢复后可继续写入。

当前候选已经通过 migration 与 PostgreSQL 18 真库下的领域和 HTTP 生命周期、幂等、并发、依赖失败、跨 Tenant、金额收敛和不可变保护；上游目标端、Billing 消费侧工作负载 Policy／resolver、全部第一版 HTTP Port 与可配置进程已经组合。Linux AMD64 非 root 镜像、pull-only release、Suite Manifest、Nginx→Gateway→IAM 正式入口、Party 故障零写入／恢复单次提交、联合备份与销毁卷后的空目标恢复也已实际通过。Billing 工作区与 Workbench 入口已通过正式 D1 HTTP 生命周期、金额撤权／恢复、历史脱敏、盲写拒绝、409／503 恢复和 1440×1050 渲染检查，可据此确认人类工作流已实现。Project 的 C3 经营链路进一步通过四类来源分段、独立金额 Capability、三角色、撤权／停用、来源／IAM 局部降级恢复和性能门槛；正式登录态的完整浏览器点击、尺寸与辅助技术矩阵、完整双 Tenant、Billing-only 最小组合和真实 Ubuntu 环境仍是独立待验证项。

## 第一实现切片

建议按以下顺序，不并行铺开未知语义：

1. 当前投影、revision、command、audit 的 PostgreSQL migration 与数据库不变量；
2. Go 核心的登记、读取、纠正、登记无效和历史；
3. Template Provider 下的 Party 精确引用、幂等、并发和依赖失败测试；
4. Capability Manifest、目标应用 Credential 验证和正式 IAM 正负矩阵；
5. Evidence 引用管理与失败恢复；
6. release、非 root 镜像、Suite Manifest、Gateway 路由、备份恢复；
7. Billing 管理页与 Workbench 入口；
8. C3 只读组合页；
9. Billing-only、C3、既有 C1／C2 和六个 Level A 回归。

## 未决事项

- 现实红字、红冲、作废、退票、换票和跨期更正的状态与会计责任；
- 发票号码、税额、税率、税号、票种和电子税务平台同步；
- 多发票、多应收、多回款之间的正式匹配和差额处理；
- 票据申请／审批、额度和会计凭证；
- Project Assignment 哪些责任事实可签发金额、文件和登记无效动作；
- CNY 之外币种的指数元数据来源与启用审批；
- C3 组合 BFF 是 Workbench 适配器还是独立只读投影服务。

这些未决事项不阻塞“登记、纠正、登记无效、Evidence 引用和只读 C3 摘要”的第一切片，但不得由实现者自行猜测。
