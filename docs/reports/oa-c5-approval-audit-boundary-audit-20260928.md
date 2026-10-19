# OA C5 批复审计与项目完成责任边界审计（2026-09-28）

## 判定

- 审计问题：批复／审计结果应属于 Production、Project、Contract／Money，还是形成独立可交付结果域；Project 完成应由谁计算、何时写入。
- 责任边界判定：建立独立 **Approval & Audit Results** 应用，拥有外部结果、精确金额、来源快照、Evidence 业务引用、revision 与纠错；Project 使用版本化 Completion Profile 形成可解释的只读完成评估，任何最终状态变化使用显式命令。
- 当前判定：`boundary-supported / implementation-not-started`。
- OA 完整体阶段判定：`insufficient-evidence`。

## 问题 A → 可验证结果 B

**问题 A**：参考 OA 的同一页面和数据库同时承载生产、批复审计、合同读取和项目完成。前端、后端与规格对金额来源和状态的解释互相冲突，照搬会把偶然实现耦合固化成新架构。

**可验证结果 B**：固定一个能被代码、权限、故障和恢复证伪的 C5 边界与 C5-A01～A15 验收矩阵；下一实现轮不再猜测事实所有者，也不把“页面能显示”当作项目完成。

## 审计方法

1. 对照参考 OA 产品规格、技术模型、前端交互、后端服务和项目进度计算；
2. 把用户结果与单体数据库位置分开；
3. 逐项寻找来源、revision、纠错、撤回、权限、故障和完成语义；
4. 明确记录实现之间的矛盾，不用最新一层代码静默覆盖其他证据；
5. 比较独立结果域、Production 扩展、Project 内置和财务域四种方案；
6. 固定第一版不变量、明确未知和可执行验收。

## 直接证据

| 证据 | 观察 | 对目标边界的影响 |
| --- | --- | --- |
| 参考 OA `arckit/spec/production/approval.md` | 批复／审计各有报告、设计／勘察／咨询金额，默认引用合同并允许覆盖 | 结果具有独立金额与文件生命周期；页面归在生产不等于 Production Work Item 所有 |
| `backend/internal/models/production_approval.go` | 结果直接外键 Project／User／File／Contract，金额为 `float64`，只有更新时间 | 这是单体便利关系，缺少跨服务 revision、精确金额和不可变纠错，不能复制 |
| `backend/internal/services/production_approval_service.go` | 服务直接调用 ContractService；更新原地 `Save`，删除物理 `Delete` | 证明现状耦合与历史缺口，不满足新架构数据所有权和追加修正规则 |
| 同服务 `GetApprovalAndAuditByProject` | 一个 Project 返回一个 approval 和一个 audit，但数据库未见唯一性与稳定排序门禁 | 第一版需要明确当前唯一结果和并发约束；多轮结果保持未知 |
| `ApprovalAuditModal.tsx` | 前端把批复默认来源标为当前合同，把审计默认来源标为批复 | 支持两类不同来源链，不能共用“重新读当前合同”的实现 |
| 同一前端提交与后端处理 | 审计勾选“引用批复”仍提交 `use_contract_amount`；后端无类型分支，对审计也重新取合同金额 | 形成可复现语义冲突；目标必须用类型化精确来源而非布尔开关 |
| 前端金额说明 | 手工金额后的说明仍是“可选” | 与参考规格“覆盖原因”要求冲突；目标把差异原因设为必填 |
| `backend/internal/oa2026/service.go` | 项目完成只统计 distinct `approval_type >= 2`，不看状态、文件、来源或 revision | “两行存在”不是可靠完成证据，必须返回逐来源可解释条件 |
| 同一 `GetProgress` | 读取进度时调用 `storeProjectState` 写 overall status | GET 产生业务写入，来源变化可使历史不可解释；目标明确禁止 |
| `ProductionApproval.yaml` | 文档声称 `pending／approved` 与两级在线审批 | 运行页面没有对应操作，测试也不足；保留为未知，不预建通用审批 |
| 现有 C4 BF-056 | Production 只拥有 Work Item、Stage、成果与历史，不拥有批复审计金额 | C5 不能在无新决策的情况下扩大 C4 内核 |

## 方案比较

| 方案 | 数据所有权与独立闭环 | 主要风险 | 判定 |
| --- | --- | --- | --- |
| 独立 Approval & Audit Results | 结果、金额、来源、文件含义和纠错在一个窄域内；可单独交付 | 多一个应用与装配项，需要精确依赖契约 | 采用 |
| Production 类型化扩展 | 页面与操作者路径接近参考 OA | 金额跨类型且依赖 Contract；污染 C4 Work Item 内核 | 不采用 |
| Project 内置 | 完成页面读取简单 | 薄 Project 吸收报告、金额、合同来源和行业规则 | 不采用 |
| Contract／Billing／Money | 金额相关组件已有 | 把外部结果误写成法律依据、票据、义务或现金 | 不采用 |
| 通用审批平台 | 表面可覆盖两级审批 | 当前缺节点、签章、多轮与稳定第二用例证据 | 明确拒绝 |

## 已接受边界

Results 拥有 `approval|audit` 当前结果、精确金额组成、外部确认登记、来源快照、Evidence 引用、revision、纠正、撤回和历史。Project、Contract、Production、Evidence、Billing 和 Money 继续拥有自己的事实。

参考 OA 第一版金额来源固定为：

- `approval`：精确 Contract／Settlement revision；
- `audit`：精确 approval revision；
- 人工登记或调整：显式来源说明／覆盖原因；
- 设计、勘察、咨询金额：版本化 `reference-oa-v1` 组成，以整数最小货币单位保存。

Results 不自动修改合同、结算、开票、义务、现金、核销、奖金或 Production 状态。以后若需要传递业务影响，只能产生带来源 revision 的建议，由目标应用独立确认。

Project 完成评估由版本化 Completion Profile 驱动，逐来源返回 `satisfied|not_applicable|pending|unavailable` 和原因。GET 永远不写状态；显式完成命令只能更新 Project 自身并冻结使用的规则与来源 revision。C6／C7 未确认前不宣称完整 Project 已完成。

## 第一实现轮验收

下一轮使用 [产品规格](../../arckit/spec/approval-audit-results/result-registration.md) 的 C5-A01～A15：覆盖独立所有权、两类结果、两条来源链、精确金额、覆盖原因、Evidence 故障、追加纠错、权限、纯读完成评估、显式完成命令、最小组合、未选缺席、D1 恢复和全部既有回归。

实现顺序固定为：契约／模型 → 领域和真库 → 精确工作负载引用 → 人类页面／权限 → D1 装配恢复 → Project 完成只读协作。任何一步发现需要共享表、跨库 ORM、物理删除、Production 内置合同金额或通用审批引擎，都停止并复审边界。

## 明确未知

- 真实两级审核／审定是否是内部流程、外部结果还是文档遗留；
- 多轮批复、复审和撤回的正式业务身份；
- 一个结果是否需要多个合同及其分摊优先级；
- 外部签章和确认人真实性验证；
- 审计结果是否以及何时形成结算、义务或奖金建议；
- 已完成 Project 的来源后来撤回时，业务上应重开、告警还是保持历史完成。

这些未知不阻止第一版结果登记，但禁止把默认级联、自由 JSON 或客户专用列写进 Base。

## 本轮未执行

本轮没有新增应用代码、数据库、API、Capability、页面、D1 服务或运行测试。它只关闭责任边界与可执行验收定义，因此不会把总进度从 63% 人为上调；C5 仍需用真实实现和运行证据逐项关闭。

