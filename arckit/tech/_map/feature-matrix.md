# 技术事实矩阵

| 产品规格 | 技术方案 | 主要模型／契约 |
| --- | --- | --- |
| `spec/project-workspace/project-initiation.md` | `project-workspace/solution.md` | `Project.yaml`、`ProjectPackageConversion.yaml`、`project-create.yaml`、`party-list.yaml`、`party-reference-get.yaml`、`project-package-create.yaml`、`project-package-convert.yaml`、`project-package-rollback.yaml`、`project-get.yaml` |
| `spec/design-operations/design-production-delivery.md` | `design-operations/solution.md` | `DesignWorkItem.yaml`、`DesignStage.yaml`、`DesignDeliverable.yaml`、`design-*.yaml` |
| `spec/assembly/full-reference-oa-validation.md` | `assembly/oa-authorization-validation.md`、`assembly/local-reference-oa-composition.md` | `customer-role-capability-overlay.yaml`、`service-credential-exchange.yaml` |
| `spec/approval-audit-results/result-registration.md` | `approval-audit-results/decision.md`、`project-workspace/solution.md` | 独立领域模型、受保护 API、人类工作区、Project 级最小 reference-read、C5 Completion Profile 只读评估／显式确认、幂等、历史防篡改、正式 D1 装配／恢复与 74 断言真实跨进程旅程已实现；完整双 Tenant 和真实浏览器待补 |
| `spec/cost-outsourcing/cost-outsourcing-lifecycle.md` | `cost-outsourcing/decision.md` | 独立 `cost-outsourcing` module 和 PostgreSQL 18 已实现双聚合、Assessment、受保护 HTTP／IAM、多 namespace Capability、精确 Project／Party／Contract／Evidence adapter、人类工作区、独立进程、正式 D1／恢复、Contract Party 运行时，以及使用 `PayableSuggestion.yaml`、`PayableSuggestionIntake.yaml` 与 `payable-suggestion-api.yaml` 的 C6→Money 接受／拒绝／故障恢复／幂等旅程；Money 最终拥有 Obligation，Billing 边界不变 |
| `spec/bonus-distribution/bonus-lifecycle.md` | `bonus-distribution/decision.md` | C7 独立核心、PostgreSQL 18、正式 IAM、真实 People／Money 来源、人类工作区、正式 D1、未选缺席、联合恢复与继续写入已实现；Money 建议和单类型最小组合仍待验证 |
| `spec/business-engagement/engagement-lifecycle.md` | `business-engagement/decision.md` | Project／Evidence reference-read 已实现；Party 仅在出现独立主体角色证据后接入，Contract／Money suggestion 契约待实现 |
| `spec/billing-invoice/invoice-registration.md` | `billing-invoice/solution.md` | 完整事实模型、权限收敛 API 视图、`invoice-registration-*.yaml` 和精确来源引用；消费侧 HTTP／IAM、非 root 发布、D1 正式入口、故障与联合恢复已实现，页面和 C3 分段只读组合待完成 |
| `spec/party-directory/party-lifecycle.md` | `party-directory/decision.md`、`party-directory/solution.md` | `Party.yaml`、`PartyRevision.yaml`、`party-*.yaml`、Party Capability Manifest、Workbench 页面与 Project 选择器 |
| `spec/assembly/customer-scenario-evidence.md` | `assembly/scenario-evidence-validation.md`、`assembly/customer-overlay-research.md` | 版本化场景与 Overlay 契约 |
| D1 环境提升 | `delivery/d1-environment-promotion.md` | 不可变发布制品、Registry 摘要与恢复流程 |
