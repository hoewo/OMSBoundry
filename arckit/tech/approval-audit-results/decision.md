# Approval & Audit Results 技术边界决策

状态：✅ 已采用；领域核心、PostgreSQL 生命周期、受保护 HTTP／IAM、精确引用、人类工作区、Project C5 完成检查点、正式 D1 装配／联合恢复与真实跨进程业务旅程已有运行证据；完整双 Tenant 和真实浏览器待补。

## 决策

C5 使用独立 **Approval & Audit Results** 应用保存批复／审计结果，而不把记录加入 Project Workspace、Production Operations、Contract Ledger 或 Billing／Money 数据库。

该结论来自三个结构事实：结果按 Project 汇总设计／勘察／咨询金额而不是某个 Production Work Item 的阶段事实；它有独立的外部报告、确认人、来源金额、人工覆盖和纠错生命周期；它可作为结果登记产品单独交付。参考 OA 把页面放在生产信息中只说明操作上下文，不证明 Production 应拥有金额和行政／审计结果。

## 不变量

- 独立 PostgreSQL 数据库、账号、migration、备份和恢复边界；
- 每个成功写命令原子形成当前投影、完整不可变 revision、Command 和 Audit；
- 金额使用整数最小货币单位和显式币种，合计只由服务端计算；
- 来源引用保存目标类型、ID、revision、状态、金额组成和稳定摘要；
- File ID／Version 只通过 Evidence reference-read 验证，应用不拥有文件字节；
- 所有受保护访问使用原生 Capability Credential，目标应用最终授权；
- 成功 Command 重放先命中本地结果，不重新访问依赖；
- 外部校验失败、stale revision 或权限拒绝不写部分事实；
- 纠正与撤回只追加 revision，不更新历史、不物理删除；
- Project 完成读取是纯读取，不能在 GET 中写 Project 状态。

## 目标模型

第一版最少包含：

- `ResultRecord`：`id`、`tenant_id`、`project_id`、`result_kind`、`state`、`revision`、`currency`、当前来源与显示摘要；
- `ResultAmountComponent`：固定 profile 下的 `component_key` 与 `amount_minor`；
- `ResultSourceSnapshot`：来源 audience／resource type／ID／revision／state、金额摘要和响应摘要；
- `ResultEvidenceReference`：File ID／Version、摘要、业务槽位和加入 revision；
- `ResultRevision`：每次登记、纠正、Evidence 变化和撤回后的完整快照；
- `ResultCommand`：Command ID、请求摘要、结果 revision 和重放信息；
- `ResultAuditEvent`：Actor、Capability、Scope、旧／新 revision、原因和时间。

当前状态只使用 `recorded|withdrawn`。`pending|approved` 若以后得到真实审批流程证据，应以新契约版本或独立审批协作加入，不能复用系统登记状态偷渡含义。

数据库约束至少保证同 Tenant／Project／Kind 只有一个当前未撤回结果、revision 单调、金额非负且不溢出、Evidence 槽位在同 revision 唯一、历史表不可更新／删除。业务层与数据库层共同保护并发。

## 来源读取端口

结果应用使用独立、可轮换工作负载身份向 IAM 交换单 audience、单 action、单精确资源 Scope 的短期凭证：

- Project：验证 active Project、Tenant、稳定摘要及指向来源精确 revision 的活动 Resource Link；
- Contract／Settlement：为 `approval` 提供精确 revision、类别、币种和金额；
- Result reference-read：为 `audit` 读取精确 `approval` revision；
- Evidence：验证每个 File Version 的 Tenant、状态、版本和摘要。

每个 resolver 都必须设置超时、响应大小上限、严格 JSON、类型／ID／revision／状态复验和明确错误映射。404、类型错误、stale 或不可用状态返回不可重试的引用无效；换票失败、401 重换仍失败、403、网络、5xx、畸形或超大响应返回可重试依赖不可用。两类失败都不得创建成功 Command。

`reference-oa-v1` 的 `approval` 只接受一个被 Project 活动链接的 Contract／Settlement 精确 revision。来源类别 `design|design_fee`、`survey|survey_fee`、`consultation|consultation_fee` 映射到同名三项金额中的唯一一项，另外两项为零；币种固定为 CNY，十进制金额必须能精确换算为分。一个结果需要聚合多个来源时必须发布新的版本化连接器规则并逐个冻结，不能读取“项目下当前合同总和”后只保存一个模糊指针。`audit` 默认只引用本应用已记录的 approval revision，不再次读取合同推导相同金额。

## API 与能力

canonical audience 为 `approval-audit-results`，Capability namespace 为 `approval-audit-result`。第一版受保护 API：

- `GET /api/v1/approval-audit-results?project_id={project_id}`
- `POST /api/v1/approval-audit-results`
- `GET /api/v1/approval-audit-results/{result_id}`
- `GET /api/v1/approval-audit-results/{result_id}/history`
- `POST /api/v1/approval-audit-results/{result_id}/corrections`
- `POST /api/v1/approval-audit-results/{result_id}/withdrawal`
- `GET /api/v1/references/projects/{project_id}/approval-audit-results`

写请求必须带 Command ID；修改请求必须带 expected revision。能力按创建、读取、金额读取、纠正、撤回、Evidence 管理和历史读取分离。详情和历史在服务端省略无权金额；写请求显式提交金额组成时同时要求金额能力，Evidence 引用集合变化时要求 Evidence 管理能力。入口删除所有客户端伪造身份 Header，只信任复验后的 Credential 上下文。

Project Scope 是第一版人类资源边界。IAM／Policy 可以依据 active Project Assignment 产生精确 Scope，但 Project 只提供版本化 Business Fact，不签发 Results 能力。参考 OA 岗位只存在于 Customer Overlay。

## Project 完成协作

Project 的完成评估使用版本化 `Completion Profile` 和来源最小 reference-read。每个 requirement 返回来源键、精确 revision、`satisfied|not_applicable|pending|unavailable` 与人类可读原因；Project 可以缓存可重建投影，但不能复制可写金额、报告或来源领域对象。

当前 `reference-oa-c5-v1` 已实现 `GET /api/v1/projects/{id}/completion-assessment` 与 `POST /api/v1/projects/{id}/completion-confirmations`。确认命令锁定 Project expected revision、Profile key 和全部 result ID／kind／revision／digest；只追加 Project 自身 revision、不可变确认、命令与历史，不调用来源写接口，也不把 Project 改为 completed。来源后续纠正／撤回只使当前评估显示 drift，不静默篡改完成历史。成功幂等回放在读取 Results 前返回原确认；新的确认在 Results 不可用时返回 503，不得降格为业务条件不满足的 422。

C5 当前只验证 Results 自身及批复审计 requirement；C6／C7 未实现前，不发布“完整 OA Project completed”的通过结论。

## 装配与独立性

Results 作为可选业务应用进入 Suite Manifest，声明 Project、Contract 和 Evidence 的直接依赖；当前参考 OA C5 组合还携带 People & Groups，作为 Project 建立负责人 Assignment 和推进 active 状态的显式依赖，而不是 Results 的数据依赖。若部署只做手工结果登记，Contract 依赖可由显式人工来源模式替代，但仍必须记录来源说明和覆盖原因，不能自动伪造合同快照。Production Operations 不是启动依赖。

未选择 Results 时，Workbench 不显示入口、Gateway 不发布路由、IAM 不装入其 Capability，C1～C4 数据和恢复清单不变化。选择后使用独立镜像、数据库、工作负载密钥和备份项；联合空目标恢复必须保持 ID、revision、来源摘要、Evidence 引用、IAM 签名材料和恢复后继续写入。

## 被拒绝方案

### 放入 Production Operations

批复／审计是外部结果登记，金额跨多个生产类型，且能在不管理生产阶段的组合中独立使用。放入 Production 会扩大 C4 的严格 Work Item 内核，强迫未使用结果登记的生产组合携带金额与合同依赖。

### 放入 Project Workspace

Project 只组织上下文、Assignment、Resource Link 和自己的状态。把报告、金额、合同来源和纠错放入 Project 会让薄容器吸收行业结果域。

### 放入 Contract／Billing／Money

合同是法律依据，Billing 是已开票登记，Money 是义务和实际资金；批复／审计金额可能成为后续建议来源，但本身不等于合同变更、发票、应收应付或现金。

### 通用审批／工作流平台

当前证据只支持外部结果登记，不支持客户自定义节点、脚本、表单和跨应用写动作。先建平台会掩盖两级审批、签章和多轮复审仍未知的事实。

## 当前完成边界

领域模型、Capability Manifest、受保护 HTTP、Project／Contract／Evidence reference-read、金额脱敏、409／503 失败恢复、目标应用 `allowed_actions`、人类工作区和 Project 完成评估已存在运行证据。正式 D1 又验证了独立 Results 数据库／账号、工作负载密钥、显式 `approval-audit-result` Capability namespace、Suite Manifest、Workbench、Gateway／Nginx 入口、镜像供应、未选缺席和联合空目标恢复。隔离 D1 的 74 个正式入口断言已完成真实 Contract／Evidence→approval→audit→Project 确认、Project Scope、金额脱敏、Grant 撤销／恢复、三类依赖停机恢复、来源纠正 drift 和本地幂等回放。尚未验证的是完整双 Tenant、账号停用、真实浏览器动作收敛和真实 Ubuntu／公网环境。

## 已验证实现切片

`backend/services/approval-audit-results` 已实现独立 Go module、嵌入式 PostgreSQL 18 migration、纯领域服务和 Store。当前投影、完整 revision、Command 和 Audit 在同一事务提交；deferred consistency trigger 保证 current 与 latest revision 对齐，不可变 trigger 拒绝历史修改与物理删除。金额 profile 的三项键、非负整数、币种和服务端总额同时受领域校验与数据库检查约束。

真库测试已证明 approval 与 audit 独立共存、audit 只读取指定 approval 历史 revision、后续 approval 更正不改变 audit、stale revision 失败、撤回后沿同一稳定 ID 追加重登 revision、跨 Tenant 不可见、八并发重放只提交一次，以及依赖失败不产生 current／revision／command／audit。

受保护 HTTP 真库测试使用 Ed25519 IAM Credential 和实际 Capability Catalog，覆盖 401、403、409、422、503、金额字段脱敏、Project／Result Scope、错 Tenant／Audience、相邻角色、缺金额或 Evidence 能力、Project 未链接／stale 来源、Evidence 停机恢复和同 Command 重放。Project／Contract／Evidence 适配器请求单 audience、单 action、单精确资源凭证；Project 和 Contract 所有者公开的正式 reference handler 与这些 JSON 投影一致。真实 D1 旅程已经把这些适配器替身替换成独立进程，并验证确认后 Project revision 推进不会阻止 Results 纠正：纠正只要求读取当前同 Tenant、同 ID 的有效 Project，不要求当前 revision 等于结果首次登记快照；每个 Results revision 仍保存各自观测的 Project 快照。

## 复审条件

出现以下直接证据时重新评估：批复／审计只能作为某个 Production Work Item 的不可分阶段完成且无法独立迁移；真实客户明确需要完整内部审批／签章并已有稳定第二使用点；一个 Project 存在有正式业务身份的多轮结果；审计结果必须以独立行政决定链而不是 approval revision 为默认来源。复审必须新增／取代决策，不能在实现中静默加列或改状态含义。
