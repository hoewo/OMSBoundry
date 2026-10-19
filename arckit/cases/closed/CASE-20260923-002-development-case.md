# 验证小系统群的产品化边界与定制平衡

Case: CASE-20260923-002
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-24T19:34:57.974Z

## User Intent

把可独立组合的小系统群从工程可运行推进到商业目标可验证：既不以万能抽象消除全部差异，也不因每个客户差异反复修改或分叉稳定内核。

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260923-002",
  "title": "验证小系统群的产品化边界与定制平衡",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-23T15:00:14.018Z",
  "updated_at": "2026-09-24T19:34:57.974Z",
  "user_intent": "把可独立组合的小系统群从工程可运行推进到商业目标可验证：既不以万能抽象消除全部差异，也不因每个客户差异反复修改或分叉稳定内核。",
  "expected_outcome": "形成可证伪的产品边界与定制平衡验证方法，并以真实 OA／目标商业场景选择下一条证据链；在取得证据前不把当前工程贯通误报为商业适配成立。",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260923-002-001",
      "revision": 1,
      "status": "accepted",
      "statement": "项目最终目标不是仅把系统做出来，而是面向 OA 所处的现实商业圈子，以可独立运行和组合的窄域小系统快速搭建客户方案；Nginx、Gateway 与 IAM 构成组合基础设施，具体业务应用按客户需要选择。",
      "basis": "用户在 2026-09-23 明确确认的两个总方向与最终目的。",
      "evidence": [
        "User statement on 2026-09-23",
        "docs/foundation.md",
        "docs/decisions.md"
      ]
    },
    {
      "id": "FACT-20260923-002-002",
      "revision": 1,
      "status": "accepted",
      "statement": "解耦与复用的平衡不能由当前整栈可运行直接证明：过度 DRY 会把客户差异压成复杂抽象，过少解耦会导致反复重构或分叉；该平衡必须用真实业务场景和可观察失败信号验证。",
      "basis": "用户明确指出当前开发对其仍是黑盒，并把能否找到该平衡点视为最大不确定性。",
      "evidence": [
        "User statement on 2026-09-23",
        "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
        "docs/development-plan.md"
      ]
    },
    {
      "id": "FACT-20260923-002-003",
      "revision": 1,
      "status": "accepted",
      "statement": "产品边界验证方法已经明确区分独立产品轴与商业适配轴，使用稳定内核、配置、策略、类型化扩展、客户装配和客户专项六层放置差异，并以过度抽象和解耦不足的可观察信号允许真实案例否定当前边界；当前工程贯通不代表商业适配已经成立。",
      "basis": "用户确认的最终目标与 OA、OMS、三类参考场景审计共同支持这一验证责任，稳定决策 BF-029 和验证文档已经落地。",
      "evidence": [
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/decisions.md",
        "docs/foundation.md",
        "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
        "docs/specs/connectors/contract-money-oa-reference-audit.md",
        "docs/specs/connectors/contract-money-oms-prototype-reference-audit.md"
      ]
    },
    {
      "id": "FACT-20260923-002-004",
      "revision": 1,
      "status": "accepted",
      "statement": "阶段一以现有完成度较高、定制化明显的 OA 为参考产品，先从其规格、代码和行为盘点核心旅程，再验证独立小系统能否重组出可用 OA；该阶段不要求用户预先掌握整个行业，不做逐页面逐表复制，也不把单一 OA 验证冒充多客户通用性。",
      "basis": "用户在 2026-09-23 明确说明自身行业认知有限，并确认成功用当前小系统搭建参考 OA 可以作为阶段第一目标。",
      "evidence": [
        "User statement on 2026-09-23",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/decisions.md#BF-030",
        "docs/foundation.md#13-阶段一目标重组参考-oa"
      ]
    },
    {
      "id": "FACT-20260923-002-005",
      "revision": 1,
      "status": "accepted",
      "statement": "参考 OA 的业务主脊柱是项目立项与项目角色下并行推进经营链、生产链、资料和统计；BizFoundry 已覆盖统一入口、IAM、People、Contract、Money 和 Evidence，主要缺口是薄 Project Workspace 与设计项目运营类型化扩展。阶段一首个集合提案由项目立项、经营合同与回款、生产交付、资料追溯四条旅程组成，M9 仍是下一方向但必须收窄；提案在用户确认前不进入规格或实现。",
      "basis": "Reference OA commit 826e0fc 的当前路由、模型和规格与 2026-08-17 已确认需求，以及 BizFoundry 现有应用边界和参考审计共同支持该结论。",
      "evidence": [
        "docs/specs/assembly/oa-reference-capability-journey-audit.md",
        "reference OA commit 826e0fc65aa09947fc4f64f62a2644fa5d11a302",
        "docs/architecture.md",
        "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
        "docs/specs/connectors/contract-money-oa-reference-audit.md"
      ]
    },
    {
      "id": "FACT-20260923-002-006",
      "revision": 1,
      "status": "accepted",
      "statement": "阶段一首个演示接受 J1 项目立项、J2 合同与回款、J3 设计生产交付、J4 资料追溯四条旅程；设计是第一生产类型，第二生产类型在首个演示通过后再选；合同或结算事实进入 Money Flow 前必须由责任人明确确认；先用各应用的本地模板 IAM 与独立入口验证业务架构，之后使用同一业务发布物验证 Nginx、Gateway 与中心 IAM 完整入口链。",
      "basis": "用户在 2026-09-25 明确要求先完成最小、稳定、不出错的本地 OA 演示，允许完整入口链后移并计划在公司私有 Ubuntu 环境验证。",
      "evidence": [
        "User statement on 2026-09-25",
        "docs/decisions.md#BF-031",
        "arckit/spec/assembly/reference-oa-demo.md",
        "docs/foundation.md",
        "docs/development-plan.md"
      ]
    },
    {
      "id": "FACT-20260923-002-007",
      "revision": 1,
      "status": "accepted",
      "statement": "J1 薄 Project Workspace 的正式规格已经固定：独立服务只拥有 Project、Assignment、类型化 Resource Link、幂等命令和不可变历史；本地模板 IAM 与正式 IAM 使用同形 v1 契约；四个模型、八个 API 及 J1-A01 至 J1-A18 验收矩阵明确覆盖成功、幂等、并发、越权、非法引用、上游故障、取消、历史与独立运行。运行时代码尚未实现。",
      "basis": "已接受的 J1 范围已经被展开为可验证的产品规则、技术边界、模型和 API，且通过结构与链接检查。",
      "evidence": [
        "arckit/spec/project-workspace/project-initiation.md",
        "arckit/tech/project-workspace/solution.md",
        "arckit/tech/INDEX.md",
        "arckit/spec/INDEX.md",
        "docs/testing.md"
      ]
    },
    {
      "id": "FACT-20260923-002-008",
      "revision": 1,
      "status": "accepted",
      "statement": "J1 薄 Project Workspace 已实现为独立 Go/PostgreSQL 服务：Project、Assignment、类型化 Resource Link、幂等命令和不可变历史由八个 v1 API 提供；模板与正式 IAM 保持同形授权；上游引用故障不破坏既有读取；J1-A01 至 J1-A18、真库、并发、跨应用回归、Docker、备份恢复和 Linux release 已通过。它尚未进入 D1，现有业务应用的全部 reference-read 端点与最终演示页面仍未完成。",
      "basis": "运行代码、真实 PostgreSQL／HTTP 测试、跨应用回归、容器冒烟、空库恢复和发布包校验共同形成直接证据。",
      "evidence": [
        "backend/services/project-workspace",
        "docs/testing.md#M9-J1-项目立项验收",
        "docs/progress.md#M9-J1-Project-Workspace-独立业务切片",
        "arckit/spec/project-workspace/project-initiation.md",
        "arckit/tech/project-workspace/solution.md"
      ]
    },
    {
      "id": "FACT-20260923-002-009",
      "revision": 1,
      "status": "superseded",
      "statement": "J3 稳定边界已经固定：独立 Design Operations 以代码拥有的 design-v1 类型管理设计 Work Item、方案／初设／施工图／施工配合四阶段、明确不适用、Evidence 成果引用、显式重开／回退和不可变历史；它只读取 active design Project、production_owner Assignment 与 Evidence File Version，不复制外部事实，不自动完成 Project、生成资金义务或演变为通用工作流。十个 v1 API 与 J3-A01 至 J3-A20 已定义，但运行时尚未实现。",
      "basis": "产品规格、技术方案、模型、API 契约、全局决策和验收入口共同固定了可实现且可证伪的边界。",
      "evidence": [
        "arckit/spec/design-operations/design-production-delivery.md",
        "arckit/tech/design-operations/solution.md",
        "arckit/tech/INDEX.md",
        "arckit/_map/feature-matrix.md",
        "docs/decisions.md#BF-032",
        "docs/testing.md#M9-J3-设计生产与交付验收待执行"
      ]
    },
    {
      "id": "FACT-20260923-002-010",
      "revision": 1,
      "status": "superseded",
      "statement": "J3 Design Operations 已实现为第七个独立运行数据所有者：design-v1 Work Item、固定四阶段、负责人、Evidence 成果、适用性、替换、显式回退、取消和不可变历史由独立 PostgreSQL 拥有；Project／Assignment／Evidence 只通过版本化 reference-read 校验。J3-A01 至 J3-A20、正式／模板 IAM、故障关闭、并发、备份恢复、Linux AMD64 release 和非 root Docker 已通过，但 J2／J4 操作者组合与完整入口仍未完成。",
      "basis": "独立运行时代码、跨应用契约、真实 PostgreSQL 和系统测试、全仓回归、恢复、发布物与容器运行证据相互一致。",
      "evidence": [
        "backend/services/design-operations",
        "arckit/spec/design-operations/design-production-delivery.md",
        "arckit/tech/design-operations/solution.md",
        "arckit/tech/_shared/contracts/project-reference-get.yaml",
        "arckit/tech/_shared/contracts/project-assignment-reference-get.yaml",
        "arckit/tech/_shared/contracts/evidence-file-version-reference-get.yaml",
        "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过",
        "docs/decisions.md#BF-032"
      ]
    },
    {
      "id": "FACT-20260923-002-011",
      "revision": 1,
      "status": "accepted",
      "statement": "首个参考 OA 本地业务架构 Demo 已实现：Contract、Money、People、Evidence、Project、Design 以六个应用和六个独立 PostgreSQL 数据库运行，同一组非真实数据经公开接口连续完成 J1～J4；重复执行保持相同业务身份，Contract 停机时 Project 保持 active 且只有合同引用 unavailable。组合未关闭鉴权、未跨库写入、未创建自动 Contract→Money 连接器；完整入口和公司 Ubuntu 总装仍需另行验收。",
      "basis": "两次 Demo、Contract 停机隔离、恢复后健康、全工作区测试、Compose 和规格证据一致；发现的 Design reference id 缺陷已兼容修复。",
      "evidence": [
        "deployments/local-oa",
        "acceptance/reference-oa-demo",
        "arckit/spec/assembly/reference-oa-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/progress.md",
        "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260923-002-001",
      "fact_id": "FACT-20260923-002-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_intent_and_scope",
        "revision": 3
      },
      "effect": "upheld",
      "reason": "The updated product intent retains the falsifiable long-term success basis while adding the reference-OA stage-one proof target.",
      "gap_ids": [],
      "evidence": [
        "docs/decisions.md#BF-029",
        "docs/decisions.md#BF-030",
        "docs/foundation.md",
        "docs/specs/assembly/product-boundary-validation.md"
      ]
    },
    {
      "id": "IMPACT-20260923-002-002",
      "fact_id": "FACT-20260923-002-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "quality_and_validation",
        "revision": 2
      },
      "effect": "upheld",
      "reason": "The validation strategy now covers both engineering correctness and real-scenario product-boundary evidence with explicit failure signals and follow-up inputs.",
      "gap_ids": [],
      "evidence": [
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/development-plan.md",
        "docs/specs/assembly/three-reference-scenarios-boundary-audit.md"
      ]
    },
    {
      "id": "IMPACT-20260923-002-003",
      "fact_id": "FACT-20260923-002-004",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_intent_and_scope",
        "revision": 3
      },
      "effect": "upheld",
      "reason": "The product intent now has a concrete first proof target while preserving the longer-term multi-scenario falsification requirement.",
      "gap_ids": [],
      "evidence": [
        "docs/decisions.md#BF-030",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/foundation.md#13-阶段一目标重组参考-oa"
      ]
    },
    {
      "id": "IMPACT-20260923-002-004",
      "fact_id": "FACT-20260923-002-011",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 7
      },
      "effect": "upheld",
      "reason": "J1-J4 is now realized locally while full-entry deployment remains distinct.",
      "gap_ids": [],
      "evidence": [
        "deployments/local-oa",
        "acceptance/reference-oa-demo",
        "arckit/spec/assembly/reference-oa-demo.md"
      ]
    },
    {
      "id": "IMPACT-20260923-002-005",
      "fact_id": "FACT-20260923-002-011",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "experience_and_interaction",
        "revision": 3
      },
      "effect": "upheld",
      "reason": "The direct-entry journey and partial-unavailable behavior are executable without weakening unified-entry requirements.",
      "gap_ids": [],
      "evidence": [
        "deployments/local-oa/README.md",
        "acceptance/reference-oa-demo"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260923-002-001",
      "status": "resolved",
      "goal": "建立并沉淀一套可证伪的产品边界验证框架：明确独立交付、稳定内核、配置／策略／类型化扩展／客户装配的分层判据，定义过度抽象与核心分叉的失败信号，并选出下一条真实业务证据链。",
      "reason": "没有该框架，继续实现 Project 或新业务功能可能只扩大黑盒，无法回答当前架构是否真的适合目标商业圈子。",
      "derived_from": [
        "FACT-20260923-002-001",
        "FACT-20260923-002-002"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "决定下一业务里程碑应验证什么，而不是继续增加什么",
        "risk": "可避免万能平台化和按客户反复 fork 两个相反方向的高成本错误",
        "user_impact": "让项目方向可以由业务证据解释，而不只由代码完成度解释",
        "evidence": "现有 OA 审计、三类参考装配审计和当前已实现的小系统可作为第一轮输入"
      },
      "responsibility": "agent",
      "evidence_required": [
        "稳定文档明确最终商业目标、分层放置规则、成功与失败判据",
        "下一条真实业务证据链具有具体输入、对照方法和输出，而不是抽象口号",
        "文档明确当前工程贯通不等于产品适配已经成立"
      ],
      "resolution": {
        "id": "GAP-20260923-002-001",
        "status": "resolved",
        "outcome": "A falsifiable product-boundary validation framework is now durable and the first real-business evidence chain is explicitly selected without claiming commercial fit.",
        "reason": "The stable documents now distinguish independent delivery from commercial adaptation, classify variation placement, define over-abstraction and under-decoupling signals, and specify the next evidence input.",
        "evidence": [
          "docs/specs/assembly/product-boundary-validation.md",
          "docs/decisions.md#BF-029",
          "docs/foundation.md#12-最终成功标准",
          "docs/development-plan.md",
          "README.md",
          "git diff --check on 2026-09-23",
          "relative link validation for five affected documents on 2026-09-23"
        ],
        "occurred_at": "2026-09-23T15:07:28.681Z"
      }
    },
    {
      "id": "GAP-20260923-002-002",
      "status": "cancelled",
      "goal": "提供并确认一条真实、脱敏的 OA 业务案例，包含首次由合同／结算依据形成应收或应付，以及同一来源后续发生金额或状态变化时的实际处理。",
      "reason": "参考代码只能证明候选业务信号；没有真实案例就无法判断触发事实、责任人、账簿主体、金额、重复身份和后续变化属于稳定内核、策略还是客户装配。",
      "derived_from": [
        "FACT-20260923-002-003"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the first commercial-boundary conclusion and any justified connector or Project implementation",
        "risk": "Invented examples would reproduce the black-box problem under a different name",
        "user_impact": "Lets the business owner test the architecture against a process they actually recognize",
        "evidence": "A short anonymized case can supply the missing facts without a complete requirements document"
      },
      "responsibility": "human",
      "evidence_required": [
        "One anonymized initial-obligation case with source fact, responsible confirmer, ledger owner, parties, amount, currency and due or expected date",
        "One subsequent-change case describing what happens after the obligation is confirmed, partially allocated, settled or closed",
        "Confirmation that the examples reflect actual or deliberately chosen target business behavior and contain no customer secrets"
      ],
      "resolution": {
        "status": "cancelled",
        "outcome": "Superseded by OA-evidence-first discovery",
        "reason": "The user is not confident enough in the full business domain to author the first case from memory. The reference OA should be audited first, after which only unresolved meanings are returned for human confirmation.",
        "evidence": [
          "User statement on 2026-09-23",
          "docs/specs/assembly/oa-reference-stage-1.md",
          "docs/development-plan.md"
        ],
        "occurred_at": "2026-09-23T15:38:14.994Z"
      }
    },
    {
      "id": "GAP-20260923-002-003",
      "status": "cancelled",
      "goal": "Use the confirmed real case to produce the six-layer boundary mapping, identify existing coverage and explicit gaps, and decide the smallest justified next implementation or no-code outcome.",
      "reason": "The framework becomes useful only when applied to confirmed business facts; this analysis must not precede the human evidence it interprets.",
      "derived_from": [
        "FACT-20260923-002-003",
        "GAP-20260923-002-002"
      ],
      "blocked_by": [
        "GAP-20260923-002-002"
      ],
      "priority_basis": {
        "blocking": "Determines whether M9, M5, a typed extension or no new implementation is justified",
        "risk": "Prevents roadmap order from replacing business evidence",
        "user_impact": "Produces an understandable architecture verdict tied to a real workflow"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Every case requirement is assigned to one of the six boundary layers with rationale",
        "Existing product coverage and missing responsibilities are separated",
        "Over-abstraction and under-decoupling signals are explicitly checked",
        "The next implementation decision cites the confirmed case and states what remains unknown"
      ],
      "resolution": {
        "status": "cancelled",
        "outcome": "Superseded by whole-reference-product boundary audit",
        "reason": "The former single Contract-to-Money mapping depended on the cancelled human-first gap. The new audit starts from the whole reference OA and may later create narrower evidence gaps.",
        "evidence": [
          "docs/specs/assembly/oa-reference-stage-1.md",
          "docs/specs/assembly/product-boundary-validation.md",
          "docs/development-plan.md"
        ],
        "occurred_at": "2026-09-23T15:38:14.994Z"
      }
    },
    {
      "id": "GAP-20260923-002-004",
      "status": "resolved",
      "goal": "Define and persist stage one as reconstructing a usable reference OA from independent small systems, with explicit scope-selection and acceptance rules that do not treat the OA as an industry standard or require feature-for-feature copying.",
      "reason": "The project needs a concrete first proof target that does not depend on the team already understanding the full commercial domain.",
      "derived_from": [
        "FACT-20260923-002-001",
        "FACT-20260923-002-002"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Controls the next product audit and implementation choice",
        "risk": "Avoids technical completion, wholesale copying or invented industry abstractions as success",
        "user_impact": "Makes the first entrepreneurial milestone concrete and reviewable"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Stable documents name the existing OA as the stage-one reference product",
        "Acceptance distinguishes business-result reconstruction from page, table or full legacy-feature copying",
        "The next evidence activity does not require the user to invent domain behavior from scratch"
      ],
      "resolution": {
        "id": "GAP-20260923-002-004",
        "status": "resolved",
        "outcome": "Stage one is durably defined as reconstructing a usable reference OA with independent small systems, under falsifiable scope and acceptance gates.",
        "reason": "The new stage specification and synchronized decision, baseline, plan and README distinguish this first proof from both Complete Suite delivery and broad market validation.",
        "evidence": [
          "docs/specs/assembly/oa-reference-stage-1.md",
          "docs/decisions.md#BF-030",
          "docs/foundation.md#13-阶段一目标重组参考-oa",
          "docs/development-plan.md",
          "README.md",
          "git diff --check on 2026-09-23",
          "relative link validation for six affected documents on 2026-09-23"
        ],
        "occurred_at": "2026-09-23T15:38:14.994Z"
      }
    },
    {
      "id": "GAP-20260923-002-005",
      "status": "resolved",
      "goal": "Audit the reference OA's complete capability surface and representative end-to-end journeys, map each item to current BizFoundry ownership or an explicit gap, and propose the smallest stage-one core-journey set without implementing new business code.",
      "reason": "The stage-one target is clear, but scope and implementation order must come from traceable OA evidence rather than menu order, roadmap order or the user's incomplete domain memory.",
      "derived_from": [
        "FACT-20260923-002-004"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Determines whether Project Workspace, a connector, approval adaptation or another typed extension is actually needed next",
        "risk": "Prevents a feature clone and prevents existing architecture from remaining a black box",
        "user_impact": "Produces an understandable reference-product map that the user can review without first becoming an industry expert",
        "evidence": "The reference OA repository, its specifications, current behavior and existing BizFoundry audits are available"
      },
      "responsibility": "agent",
      "evidence_required": [
        "A traceable OA capability and journey inventory separates observed behavior, BizFoundry coverage, explicit gaps and unresolved business meaning",
        "Each selected candidate journey states the user result, fact owner, state transitions, failure or correction behavior and six-layer variation placement",
        "A minimal stage-one journey set is proposed with inclusions, exclusions and reasons",
        "The resulting next implementation choice remains a proposal until documented evidence supports it"
      ],
      "resolution": {
        "id": "GAP-20260923-002-005",
        "status": "resolved",
        "outcome": "The reference OA capability surface and representative journeys are mapped to current BizFoundry ownership, explicit gaps and a smallest proposed stage-one set without implementing business code.",
        "reason": "The audit traces the project-centered OA spine, distinguishes current code from newer confirmed requirements and historic specifications, maps each major capability, defines four end-to-end journeys with ownership and recovery behavior, and leaves the resulting scope as a proposal.",
        "evidence": [
          "docs/specs/assembly/oa-reference-capability-journey-audit.md",
          "reference OA commit 826e0fc65aa09947fc4f64f62a2644fa5d11a302",
          "reference OA docs/OA_2026_REQUIREMENTS_CONFIRMED.md",
          "reference OA docs/OA_PERMISSION_MODEL.md",
          "reference OA arckit/spec/INDEX.md and linked capability specifications",
          "docs/specs/connectors/contract-money-oa-reference-audit.md",
          "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
          "git diff --check on affected tracked documents on 2026-09-23",
          "git diff --no-index --check for the new audit document on 2026-09-23",
          "relative link validation for seven affected documents on 2026-09-23"
        ],
        "occurred_at": "2026-09-23T15:48:35.421Z"
      }
    },
    {
      "id": "GAP-20260923-002-006",
      "status": "resolved",
      "goal": "Confirm or revise the proposed stage-one J1-J4 core journey set, select the first production type and decide whether the first Contract or Settlement to Money Flow obligation step is explicitly human-confirmed rather than automatic.",
      "reason": "The audit can establish evidence and options, but these three scope choices determine the product promise and must be accepted by the product owner before formal Project Workspace and design-operations specifications or implementation begin.",
      "derived_from": [
        "FACT-20260923-002-005"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks formal M9 and design-project-operations scope",
        "risk": "Avoids silently turning an audit proposal into product requirements or prematurely automating an unconfirmed financial trigger",
        "user_impact": "Lets the owner make three understandable product choices without needing full industry expertise",
        "evidence": "The audit presents the proposed set, exclusions and tradeoffs in reviewable form"
      },
      "responsibility": "human",
      "evidence_required": [
        "Explicit acceptance or revision of J1 project initiation, J2 business-contract-cash, J3 production delivery and J4 project evidence traceability as the first stage-one set",
        "Selection of design or another reference production type for the first complete production journey and a second type for extensibility proof",
        "Acceptance or revision of explicit human confirmation before Contract or Settlement facts become Money Flow obligations in the first slice"
      ],
      "resolution": {
        "id": "GAP-20260923-002-006",
        "status": "resolved",
        "outcome": "The owner accepted J1-J4 as the first demo set, design as the first production type, explicit human confirmation before Money Flow obligation creation, and local business-architecture validation before full ingress validation.",
        "reason": "The latest direction explicitly authorizes the minimum local OA demo, defers the second production-type proof until that demo passes, and preserves the complete Nginx-Gateway-IAM chain for later environment validation.",
        "evidence": [
          "User statement on 2026-09-25",
          "docs/decisions.md#BF-031",
          "arckit/spec/assembly/reference-oa-demo.md",
          "docs/specs/assembly/oa-reference-stage-1.md"
        ],
        "occurred_at": "2026-09-24T17:06:37.362Z"
      }
    },
    {
      "id": "GAP-20260923-002-007",
      "status": "resolved",
      "goal": "把已接受的 J1 项目立项范围细化为薄 Project Workspace 的稳定产品规格、数据与 API 契约、权限边界和可执行验收矩阵，不在同一轮提前实现运行时代码。",
      "reason": "Project Workspace 尚不存在；先固定最小事实、不变量、失败恢复和不负责范围，才能避免把参考 OA 的所有字段与流程复制成万能 Project 平台。",
      "derived_from": [
        "FACT-20260923-002-006"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the first new runtime slice of the accepted demo",
        "risk": "Prevents project scope creep, cross-application ownership leakage and premature framework extraction",
        "user_impact": "Defines the smallest visible project-initiation result that the demo must reliably show",
        "evidence": "The accepted reference demo and existing application contracts provide a bounded specification input"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Stable Project Workspace product specification states Project, Assignment, resource-link and history behavior",
        "Stable technical contract defines owned data, versioned APIs, idempotency, authorization and cross-application references",
        "Executable acceptance matrix covers success, duplicate, unauthorized, invalid reference and history cases",
        "The specification preserves direct local entry with template IAM and the later complete-suite entry chain"
      ],
      "resolution": {
        "id": "GAP-20260923-002-007",
        "status": "resolved",
        "outcome": "J1 now has a stable product specification, technical solution, four owned data models, eight versioned API contracts and an eighteen-case executable acceptance matrix.",
        "reason": "The artifacts define Project ownership, states, commands, IAM boundary, external references, failure recovery, deployment portability and explicit exclusions without implementing runtime code in the same round.",
        "evidence": [
          "arckit/spec/project-workspace/project-initiation.md",
          "arckit/tech/project-workspace/solution.md",
          "arckit/tech/_shared/models/Project.yaml",
          "arckit/tech/_shared/models/ProjectAssignment.yaml",
          "arckit/tech/_shared/models/ProjectResourceLink.yaml",
          "arckit/tech/_shared/models/ProjectHistoryEvent.yaml",
          "arckit/tech/_shared/contracts/project-create.yaml",
          "arckit/tech/_shared/contracts/project-update.yaml",
          "arckit/tech/_shared/contracts/project-assignment-put.yaml",
          "arckit/tech/_shared/contracts/project-resource-link-put.yaml",
          "arckit/tech/_shared/contracts/project-initiation-complete.yaml",
          "arckit/tech/_shared/contracts/project-cancel.yaml",
          "arckit/tech/_shared/contracts/project-get.yaml",
          "arckit/tech/_shared/contracts/project-history-list.yaml",
          "docs/testing.md",
          "YAML parse validation and git diff --check on 2026-09-25",
          "relative Markdown link validation on 2026-09-25"
        ],
        "occurred_at": "2026-09-24T17:16:15.293Z"
      }
    },
    {
      "id": "GAP-20260923-002-008",
      "status": "resolved",
      "goal": "实现并验证 J1 薄 Project Workspace 最小纵向闭环，包括独立 Go 服务、PostgreSQL migration、v1 模板／正式 IAM 边界、Project／Assignment／Resource Link／历史 API、独立交付入口和 J1-A01 至 J1-A18 证据。",
      "reason": "稳定规格已经完成，但实际软件尚不能创建、指派、立项、取消或追溯 Project，阶段一 OA Demo 仍缺第一条运行时业务脊柱。",
      "derived_from": [
        "FACT-20260923-002-007"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks all later J2-J4 project-context composition",
        "risk": "Implementation is the first proof that the thin boundary works without cross-database ownership leakage",
        "user_impact": "Produces the first usable local OA journey rather than another design artifact",
        "evidence": "The complete J1 specification and existing Go application patterns make implementation ready"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Independent Project Workspace Go module, migrations, configuration, health endpoint and local PostgreSQL startup are implemented",
        "All J1 command, query, IAM and external-reference behaviors are covered by proportional unit and integration tests",
        "J1-A01 through J1-A18 have direct test or repeatable acceptance evidence, with any environment-only item explicitly separated",
        "Existing People, Contract, Money and Evidence tests remain unaffected and Project owns no copied external facts",
        "Testing and progress documents record only checks actually run"
      ],
      "resolution": {
        "id": "GAP-20260923-002-008",
        "status": "resolved",
        "outcome": "J1 is implemented as an independently runnable and portable Project Workspace vertical slice with direct acceptance evidence.",
        "reason": "The Go service, PostgreSQL schema, IAM providers, eight APIs, reference failure behavior, idempotency, immutable history, Docker image, backup/restore and Linux release all passed proportional checks without joining D1 or copying external facts.",
        "evidence": [
          "backend/services/project-workspace",
          "arckit/spec/project-workspace/project-initiation.md",
          "arckit/tech/project-workspace/solution.md",
          "docs/testing.md#M9-J1-项目立项验收",
          "go test -race ./backend/services/project-workspace/... on 2026-09-25: passed",
          "PostgreSQL 18 integration go test -race -tags=integration ./backend/services/project-workspace/... on 2026-09-25: passed",
          "Project plus People, Contract, Money and Evidence regression on 2026-09-25: passed",
          "Project PostgreSQL custom archive restored into a fresh empty database on 2026-09-25: migration, Project and History counts verified",
          "Linux amd64 v0.1.0 release checksum and file manifest on 2026-09-25: passed",
          "omsbizfoundry/project-workspace:j1-local non-root container health and create-project smoke on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-24T17:54:46.271Z"
      }
    },
    {
      "id": "GAP-20260923-002-009",
      "status": "resolved",
      "goal": "固定 J3 设计生产与交付的最小稳定规格、数据所有权、状态机、类型化 API 和可执行验收矩阵，使其以 Project ID 和 Evidence File ID 组合但不复制 Project、People、Contract、Money 或文件事实。",
      "reason": "J1 已提供项目身份和责任脊柱；阶段一 OA Demo 当前最大的业务缺口是设计工作项、生产阶段、成果要求和交付事实，直接编码前必须先固定不变量和验收边界。",
      "derived_from": [
        "FACT-20260923-002-006",
        "FACT-20260923-002-008"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the J3 journey and the first recognizable OA production demo",
        "uncertainty": "The minimal boundary must avoid becoming a generic task or workflow platform",
        "risk": "Coding without stable ownership would either copy Project/Evidence facts or over-generalize customer-specific production flows",
        "user_impact": "Turns the existing ledgers and Project identity into an actual design-business journey",
        "evidence": "The accepted reference OA audit and completed J1 boundary provide enough input for a bounded specification"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Stable product specification states J3 user outcomes, owned facts, exclusions, states, corrections and failure recovery",
        "Technical solution and versioned models/API contracts preserve application ownership and configurable endpoints",
        "Acceptance matrix covers the minimum happy path, authorization, idempotency, concurrency, invalid references, Evidence outage and immutable delivery history",
        "Reference OA Demo, indexes, relations, plan and testing documents remain synchronized without claiming runtime completion"
      ],
      "resolution": {
        "id": "GAP-20260923-002-009",
        "status": "resolved",
        "outcome": "J3 is fixed as an independent Design Operations capability with one versioned design-v1 type, explicit ownership, state and correction rules, ten v1 APIs and J3-A01 through J3-A20 acceptance criteria.",
        "reason": "The stable product and technical artifacts now make the minimum business result, ownership exclusions, failure behavior and proof threshold recoverable without pretending the runtime already exists.",
        "evidence": [
          "arckit/spec/design-operations/design-production-delivery.md",
          "arckit/tech/design-operations/solution.md",
          "arckit/tech/_shared/models/DesignWorkItem.yaml",
          "arckit/tech/_shared/models/DesignStage.yaml",
          "arckit/tech/_shared/models/DesignDeliverable.yaml",
          "arckit/tech/_shared/models/DesignHistoryEvent.yaml",
          "arckit/tech/_shared/contracts/design-*.yaml",
          "docs/decisions.md#BF-032",
          "docs/testing.md#M9-J3-设计生产与交付验收待执行"
        ],
        "occurred_at": "2026-09-24T18:05:10.702Z"
      }
    },
    {
      "id": "GAP-20260923-002-010",
      "status": "resolved",
      "goal": "实现并验证 Design Operations 最小纵向闭环以及它实际需要的 Project／Evidence reference-read API，覆盖 J3-A01 至 J3-A20、独立备份恢复、Linux release 和非 root Docker 冒烟，同时保持 Nginx／Gateway／中心 IAM 与 D1 总装为后续独立验收。",
      "reason": "J3 的稳定业务和技术契约已经成立，但当前软件仍不能创建、推进、交付或追溯设计工作项，也无法用真实 Project Assignment 与 Evidence Version 完成组合验证。",
      "derived_from": [
        "FACT-20260923-002-008",
        "FACT-20260923-002-009"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the J3 journey and first end-to-end local OA business demo",
        "uncertainty": "Runtime integration must prove the stable boundary works with real reference services",
        "risk": "A mock-only implementation could hide ownership, outage or idempotency defects",
        "user_impact": "Produces the first runnable design-production capability for the demo",
        "evidence": "The accepted J3 specification and existing Go service patterns make implementation ready"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Independent Design Operations Go module, PostgreSQL migration, IAM providers, ten v1 APIs and configurable Project/Evidence reference clients are implemented",
        "Project and Evidence expose only the minimum versioned reference-read facts required by J3 without leaking internal tables or storage credentials",
        "J3-A01 through J3-A20 have direct unit, HTTP, real PostgreSQL and system-composition evidence",
        "Backup/restore, Linux release checksum, non-root Docker smoke and existing application regressions pass",
        "Progress and testing documents record only checks actually run"
      ],
      "resolution": {
        "id": "GAP-20260923-002-010",
        "status": "resolved",
        "outcome": "Design Operations now runs as an independent Go and PostgreSQL application with real Project, Assignment and Evidence reference reads, complete J3 behavior and repeatable delivery evidence.",
        "reason": "The runtime, cross-application read boundaries, A01-A20 tests, regression checks, backup restore, Linux release and non-root container evidence satisfy every declared requirement without pulling the entry stack into the business slice.",
        "evidence": [
          "backend/services/design-operations",
          "backend/services/project-workspace/internal/projects/service.go",
          "backend/services/evidence-service/internal/httpserver/server.go",
          "arckit/spec/design-operations/design-production-delivery.md#实现与验证证据",
          "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过",
          "docs/progress.md#M9-J3-Design-Operations-独立业务切片"
        ],
        "occurred_at": "2026-09-24T18:40:39.882Z"
      }
    },
    {
      "id": "GAP-20260923-002-011",
      "status": "resolved",
      "goal": "复用已通过的 J1／J3 与既有 Contract／Money／Evidence，补齐 J2 最小稳定引用和 J4 项目追溯，在同一组非真实数据上形成可重复的本地参考 OA 操作者 Demo；不引入自动 Contract→Money 连接器，也不以 Nginx／Gateway／中心 IAM／D1 为本轮前提。",
      "reason": "独立业务切片已经证明项目和设计边界，但尚未证明小系统群能连续完成项目、合同与回款、设计交付和资料追溯，也没有供演示者重复执行的最小交互入口。",
      "derived_from": [
        "FACT-20260923-002-006",
        "FACT-20260923-002-008",
        "FACT-20260923-002-010"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the phase-one reference OA Demo and the main business-architecture hypothesis",
        "uncertainty": "J2/J4 composition may expose missing reference contracts or operator-flow gaps",
        "risk": "Adding a broad portal or automatic connector could violate ownership and invent business decisions",
        "user_impact": "Produces the first recognizable, repeatable OA demonstration before company deployment",
        "evidence": "J1 and J3 now run independently and existing Contract, Money and Evidence capabilities are available for composition"
      },
      "responsibility": "agent",
      "evidence_required": [
        "One documented non-real demo dataset completes J1, explicit J2 obligation confirmation, receipt/allocation, J3 and J4 traceability through versioned APIs",
        "Minimum Contract, Money, Design and Evidence reference reads expose only stable IDs, revisions, statuses and failure states",
        "An operator can repeat the journey locally with template IAM credentials and without editing databases or bypassing authorization",
        "Stopping one referenced service yields explicit partial-unavailable behavior without false success or cross-database writes",
        "Automated composition tests and a concise demo runbook pass while Nginx/Gateway/central IAM remain a separate later acceptance"
      ],
      "resolution": {
        "id": "GAP-20260923-002-011",
        "status": "resolved",
        "outcome": "Six authenticated applications and six isolated databases complete the same fake-data J1-J4 journey, replay idempotently and expose Contract partial unavailability.",
        "reason": "Public-interface composition, outage recovery, tests and runbook satisfy all evidence without a connector or database edits.",
        "evidence": [
          "deployments/local-oa",
          "acceptance/reference-oa-demo",
          "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
        ],
        "occurred_at": "2026-09-24T19:32:17.724Z"
      }
    }
  ],
  "content_revision": 9,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 3,
      "source": "Default project completion review budget",
      "snapshotted_at": "2026-09-23T15:00:14.018Z"
    },
    "additional_cycles_authorized": 0,
    "cycle_count": 1,
    "reviewed_content_revision": 9,
    "dimensions": {
      "implementation_correctness": "clean",
      "problem_resolution": "clean",
      "verification_credibility": "clean",
      "regression_risk": "clean",
      "minimality": "clean"
    },
    "findings": [],
    "cycles": [
      {
        "cycle": 1,
        "autonomous_cycle": 1,
        "reviewer": "agent",
        "outcome": "clean",
        "content_revision": 9,
        "dimensions": {
          "implementation_correctness": "clean",
          "problem_resolution": "clean",
          "verification_credibility": "clean",
          "regression_risk": "clean",
          "minimality": "clean"
        },
        "finding_ids": [],
        "evidence": [
          "deployments/local-oa/compose.yaml: six independently persisted authenticated applications bound to localhost",
          "acceptance/reference-oa-demo: public-interface J1-J4 driver with fixed command IDs and owner-only resume state",
          "two consecutive local OA demo executions retained the same business IDs and ended at Project revision 9",
          "Contract Ledger outage isolation passed with Project active, four references available and one unavailable, followed by successful recovery",
          "make test, make vet, make fmt-check, Compose config validation, YAML parsing and git diff --check passed on 2026-09-25",
          "Design stable-reference compatibility regression is covered by Design and Project tests",
          "arckit/spec/assembly/reference-oa-demo.md and arckit/tech/assembly/local-reference-oa-composition.md preserve the deferred Nginx/Gateway/central-IAM boundary"
        ],
        "occurred_at": "2026-09-24T19:34:57.974Z"
      }
    ],
    "evidence": [
      "deployments/local-oa/compose.yaml: six independently persisted authenticated applications bound to localhost",
      "acceptance/reference-oa-demo: public-interface J1-J4 driver with fixed command IDs and owner-only resume state",
      "two consecutive local OA demo executions retained the same business IDs and ended at Project revision 9",
      "Contract Ledger outage isolation passed with Project active, four references available and one unavailable, followed by successful recovery",
      "make test, make vet, make fmt-check, Compose config validation, YAML parsing and git diff --check passed on 2026-09-25",
      "Design stable-reference compatibility regression is covered by Design and Project tests",
      "arckit/spec/assembly/reference-oa-demo.md and arckit/tech/assembly/local-reference-oa-composition.md preserve the deferred Nginx/Gateway/central-IAM boundary"
    ],
    "escalation": null,
    "human_authorizations": []
  },
  "open_questions": [],
  "decisions": [],
  "pending_handoffs": [],
  "process_notes": [],
  "rounds": [
    {
      "round": 1,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "把用户确认的最终商业目标转成可证伪、可恢复的产品边界验证方法，并确定下一条需要真实业务输入的证据链。",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The product-boundary validation gap was the only ready Agent-owned candidate that could proceed while the public D1 case remained at a human ICP prerequisite, and it directly controls what business implementation should happen next.",
        "snapshot_token": "b944deb745e78caeb387a892f604af520545d0de9e8eb49967b972450d1991e8",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-001",
        "comparison_summary": "The ICP candidate in CASE-20260923-001 remains human-owned and was deferred. GAP-20260923-002-001 was selected because its accepted user facts expose a higher-level direction risk that can be resolved from existing project evidence without waiting for the external filing process.",
        "fresh_discovery_summary": "The accepted validation framework shows that the next high-information business step requires one real anonymized initial-obligation case and one subsequent-change case; the dependent mapping work is persisted rather than executed from unconfirmed business facts.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-001:GAP-20260923-001-009",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks external public acceptance but not product-boundary analysis",
              "responsibility": "human",
              "user_impact": "External users cannot yet reach app.xidou.me"
            },
            "reason": "It requires the account owner to complete ICP filing or choose an alternate ingress and cannot be advanced by repository work in this round."
          },
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Determines which business implementation is justified next",
              "risk": "Controls both over-generalization and customer-specific core forks",
              "user_impact": "Makes the project direction explainable instead of a technical black box"
            },
            "reason": "Existing OA, OMS and assembly audits are sufficient to define a falsifiable method now, while actual customer fit remains explicitly unclaimed."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-001",
        "responsibility": "agent",
        "goal": "建立并沉淀一套可证伪的产品边界验证框架：明确独立交付、稳定内核、配置／策略／类型化扩展／客户装配的分层判据，定义过度抽象与核心分叉的失败信号，并选出下一条真实业务证据链。",
        "reason": "没有该框架，继续实现 Project 或新业务功能可能只扩大黑盒，无法回答当前架构是否真的适合目标商业圈子。",
        "derived_from": [
          "FACT-20260923-002-001",
          "FACT-20260923-002-002"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "决定下一业务里程碑应验证什么，而不是继续增加什么",
          "uncertainty": "",
          "risk": "可避免万能平台化和按客户反复 fork 两个相反方向的高成本错误",
          "user_impact": "让项目方向可以由业务证据解释，而不只由代码完成度解释",
          "evidence": "现有 OA 审计、三类参考装配审计和当前已实现的小系统可作为第一轮输入"
        },
        "evidence_required": [
          "稳定文档明确最终商业目标、分层放置规则、成功与失败判据",
          "下一条真实业务证据链具有具体输入、对照方法和输出，而不是抽象口号",
          "文档明确当前工程贯通不等于产品适配已经成立"
        ]
      },
      "planned_transition": {
        "goal": "把用户确认的最终商业目标转成可证伪、可恢复的产品边界验证方法，并确定下一条需要真实业务输入的证据链。",
        "expected_state_change": "项目决策与长期文档明确独立产品轴、商业适配六层、双向失败信号和首条 OA 业务验证链；当前工程贯通不再被误解为商业适配已成立。"
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-001",
          "status": "resolved",
          "outcome": "A falsifiable product-boundary validation framework is now durable and the first real-business evidence chain is explicitly selected without claiming commercial fit.",
          "reason": "The stable documents now distinguish independent delivery from commercial adaptation, classify variation placement, define over-abstraction and under-decoupling signals, and specify the next evidence input.",
          "evidence": [
            "docs/specs/assembly/product-boundary-validation.md",
            "docs/decisions.md#BF-029",
            "docs/foundation.md#12-最终成功标准",
            "docs/development-plan.md",
            "README.md",
            "git diff --check on 2026-09-23",
            "relative link validation for five affected documents on 2026-09-23"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-003",
            "revision": 1,
            "status": "accepted",
            "statement": "产品边界验证方法已经明确区分独立产品轴与商业适配轴，使用稳定内核、配置、策略、类型化扩展、客户装配和客户专项六层放置差异，并以过度抽象和解耦不足的可观察信号允许真实案例否定当前边界；当前工程贯通不代表商业适配已经成立。",
            "basis": "用户确认的最终目标与 OA、OMS、三类参考场景审计共同支持这一验证责任，稳定决策 BF-029 和验证文档已经落地。",
            "evidence": [
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/decisions.md",
              "docs/foundation.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
              "docs/specs/connectors/contract-money-oa-reference-audit.md",
              "docs/specs/connectors/contract-money-oms-prototype-reference-audit.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-001",
            "fact_id": "FACT-20260923-002-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_intent_and_scope",
              "revision": 2
            },
            "effect": "upheld",
            "reason": "The product intent now includes an explicit falsifiable success basis: independent delivery plus controlled variation across real scenarios, without claiming current commercial fit.",
            "gap_ids": [],
            "evidence": [
              "docs/decisions.md#BF-029",
              "docs/foundation.md#12-最终成功标准",
              "docs/specs/assembly/product-boundary-validation.md"
            ]
          },
          {
            "id": "IMPACT-20260923-002-002",
            "fact_id": "FACT-20260923-002-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "quality_and_validation",
              "revision": 2
            },
            "effect": "upheld",
            "reason": "The validation strategy now covers both engineering correctness and real-scenario product-boundary evidence with explicit failure signals and follow-up inputs.",
            "gap_ids": [],
            "evidence": [
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/development-plan.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-002",
            "status": "open",
            "goal": "提供并确认一条真实、脱敏的 OA 业务案例，包含首次由合同／结算依据形成应收或应付，以及同一来源后续发生金额或状态变化时的实际处理。",
            "reason": "参考代码只能证明候选业务信号；没有真实案例就无法判断触发事实、责任人、账簿主体、金额、重复身份和后续变化属于稳定内核、策略还是客户装配。",
            "derived_from": [
              "FACT-20260923-002-003"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the first commercial-boundary conclusion and any justified connector or Project implementation",
              "risk": "Invented examples would reproduce the black-box problem under a different name",
              "user_impact": "Lets the business owner test the architecture against a process they actually recognize",
              "evidence": "A short anonymized case can supply the missing facts without a complete requirements document"
            },
            "responsibility": "human",
            "evidence_required": [
              "One anonymized initial-obligation case with source fact, responsible confirmer, ledger owner, parties, amount, currency and due or expected date",
              "One subsequent-change case describing what happens after the obligation is confirmed, partially allocated, settled or closed",
              "Confirmation that the examples reflect actual or deliberately chosen target business behavior and contain no customer secrets"
            ],
            "resolution": null
          },
          {
            "id": "GAP-20260923-002-003",
            "status": "open",
            "goal": "Use the confirmed real case to produce the six-layer boundary mapping, identify existing coverage and explicit gaps, and decide the smallest justified next implementation or no-code outcome.",
            "reason": "The framework becomes useful only when applied to confirmed business facts; this analysis must not precede the human evidence it interprets.",
            "derived_from": [
              "FACT-20260923-002-003",
              "GAP-20260923-002-002"
            ],
            "blocked_by": [
              "GAP-20260923-002-002"
            ],
            "priority_basis": {
              "blocking": "Determines whether M9, M5, a typed extension or no new implementation is justified",
              "risk": "Prevents roadmap order from replacing business evidence",
              "user_impact": "Produces an understandable architecture verdict tied to a real workflow"
            },
            "responsibility": "agent",
            "evidence_required": [
              "Every case requirement is assigned to one of the six boundary layers with rationale",
              "Existing product coverage and missing responsibilities are separated",
              "Over-abstraction and under-decoupling signals are explicitly checked",
              "The next implementation decision cites the confirmed case and states what remains unknown"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_intent_and_scope",
            "observed_revision": 1,
            "set_decision": {
              "status": "settled",
              "statement": "建设小而完整、可独立交付的窄域业务应用组合，不建设万能业务平台；成功以独立产品可交付、真实场景中的稳定内核加受控变化、无客户核心分支为依据，并允许业务证据否定当前边界。",
              "reason": "BF-001、BF-002、BF-017 与 BF-029 共同固定最终商业目标和可证伪边界。",
              "evidence": [
                "docs/decisions.md",
                "docs/foundation.md",
                "docs/specs/assembly/product-boundary-validation.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The user explicitly confirmed that commercial adaptability rather than mere implementation completion is the final objective.",
            "evidence": [
              "User statement on 2026-09-23",
              "docs/decisions.md#BF-029",
              "docs/specs/assembly/product-boundary-validation.md"
            ]
          },
          {
            "area_ref": "quality_and_validation",
            "observed_revision": 1,
            "set_decision": {
              "status": "settled",
              "statement": "每个纵向闭环必须用与风险相称的工程证据证明；产品边界还必须用真实、脱敏业务案例检查独立交付、六层变化放置、过度抽象和核心分叉信号，未取得的工程或商业证据均不得宣称通过。",
              "reason": "工程正确性不能替代商业适配验证，BF-029 和产品边界验证方法补充了长期质量责任。",
              "evidence": [
                "AGENTS.md",
                "docs/testing.md",
                "docs/specs/assembly/product-boundary-validation.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The accepted final objective requires validation of the productization balance in addition to runtime correctness.",
            "evidence": [
              "User statement on 2026-09-23",
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/development-plan.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "在 ICP 备案由账号所有者推进期间，先用一条真实、脱敏的 OA 业务链验证稳定内核、配置、策略、类型化扩展与客户装配边界；取得证据前不直接启动 M9 或 M5 实现。",
          "project_priorities": [
            "取得真实业务案例并完成产品边界走查",
            "完成 ICP 后复验 D1 外部公网入口",
            "依据边界证据选择 Contract-only 验收、M9、M5 或类型化扩展的下一实现"
          ]
        },
        "evidence": [
          "docs/decisions.md",
          "docs/foundation.md",
          "docs/development-plan.md",
          "docs/specs/assembly/product-boundary-validation.md",
          "README.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 10,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The final commercial objective, its non-goals, falsifiable boundary rules and first evidence chain are now durable in the decision index and product baseline.",
            "fact_refs": [
              "FACT-20260923-002-003"
            ],
            "evidence": [
              "docs/decisions.md#BF-029",
              "docs/foundation.md#12-最终成功标准",
              "docs/specs/assembly/product-boundary-validation.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "This round defines product-boundary validation and does not establish or change a user interaction journey. The future real-case mapping remains open.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "No visual language, component or presentation rule is established or changed by the product-boundary framework.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The framework preserves the accepted Nginx, Gateway, IAM, application ownership and versioned-contract boundaries while explicitly separating infrastructure composition from business variation placement.",
            "fact_refs": [
              "FACT-20260923-002-003"
            ],
            "evidence": [
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/foundation.md",
              "docs/decisions.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "not_relevant",
            "reason": "This round explicitly does not claim that the current software already realizes broad commercial fit; it establishes the method and required evidence for making that claim later.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Both over-generalization and under-decoupling now have observable failure signals grounded in the existing OA, OMS and reference-scenario audits, and unverified business fit remains an open human evidence gap.",
            "fact_refs": [
              "FACT-20260923-002-003"
            ],
            "evidence": [
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
              "docs/specs/connectors/contract-money-oa-reference-audit.md",
              "docs/specs/connectors/contract-money-oms-prototype-reference-audit.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/decisions.md",
        "docs/foundation.md",
        "docs/development-plan.md",
        "README.md",
        "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
        "docs/specs/connectors/contract-money-oa-reference-audit.md",
        "docs/specs/connectors/contract-money-oms-prototype-reference-audit.md",
        "git diff --check on 2026-09-23",
        "relative link validation for five affected documents on 2026-09-23"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-23T15:07:28.681Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Make reference-OA reconstruction the durable stage-one product objective and replace the direct human-first evidence route with an OA-evidence-first audit.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "fresh",
        "basis": "The user supplied a higher-information product decision: because the team does not yet claim broad domain expertise, the completed customized OA becomes the stage-one reference product for testing whether the small-system architecture can reconstruct a real application.",
        "snapshot_token": "2fb276c958f47a538c1248259265c375974e39483cd01b722487e92e54918882",
        "selected_ref": "fresh-gap:CASE-20260923-002:GAP-20260923-002-004",
        "comparison_summary": "The persisted real-case request assumed the user could supply domain facts directly. The fresh stage-goal clarification provides a safer evidence-first route: audit the existing OA before asking the user to resolve only remaining ambiguities. The ICP candidate belongs to another Case and remains human-owned.",
        "fresh_discovery_summary": "Durable project documents must distinguish the stage-one reference-OA target from both Complete Suite engineering delivery and the later claim of multi-customer commercial fit.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-002",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Originally blocked the first commercial-boundary walkthrough",
              "risk": "Asking a domain-novice owner to invent the first case can turn assumptions into product facts",
              "user_impact": "The existing OA can supply a more concrete starting point"
            },
            "reason": "The user stated limited domain confidence and selected the existing OA as the phase-one reference. This direct human-first gap is superseded by an OA evidence audit and will be cancelled in the accepted delta."
          },
          {
            "ref": "fresh-gap:CASE-20260923-002:GAP-20260923-002-004",
            "source": "fresh",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Defines stage one before new business implementation or audit",
              "risk": "Prevents both feature-for-feature copying and unsupported industry generality",
              "user_impact": "Turns uncertainty into a testable first product objective",
              "evidence": "The explicit user decision plus current OA and boundary-audit documents"
            },
            "reason": "This goal can be completed now and determines the object and acceptance boundary of the next audit."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-004",
        "status": "open",
        "goal": "Define and persist stage one as reconstructing a usable reference OA from independent small systems, with explicit scope-selection and acceptance rules that do not treat the OA as an industry standard or require feature-for-feature copying.",
        "reason": "The project needs a concrete first proof target that does not depend on the team already understanding the full commercial domain.",
        "derived_from": [
          "FACT-20260923-002-001",
          "FACT-20260923-002-002"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Controls the next product audit and implementation choice",
          "risk": "Avoids technical completion, wholesale copying or invented industry abstractions as success",
          "user_impact": "Makes the first entrepreneurial milestone concrete and reviewable"
        },
        "responsibility": "agent",
        "evidence_required": [
          "Stable documents name the existing OA as the stage-one reference product",
          "Acceptance distinguishes business-result reconstruction from page, table or full legacy-feature copying",
          "The next evidence activity does not require the user to invent domain behavior from scratch"
        ],
        "resolution": null
      },
      "planned_transition": {
        "goal": "Make reference-OA reconstruction the durable stage-one product objective and replace the direct human-first evidence route with an OA-evidence-first audit.",
        "expected_state_change": "Project decisions, baseline, plan and validation documents define stage-one scope and gates; obsolete human-first gaps are cancelled and a ready OA capability-and-journey audit gap is created."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-004",
          "status": "resolved",
          "outcome": "Stage one is durably defined as reconstructing a usable reference OA with independent small systems, under falsifiable scope and acceptance gates.",
          "reason": "The new stage specification and synchronized decision, baseline, plan and README distinguish this first proof from both Complete Suite delivery and broad market validation.",
          "evidence": [
            "docs/specs/assembly/oa-reference-stage-1.md",
            "docs/decisions.md#BF-030",
            "docs/foundation.md#13-阶段一目标重组参考-oa",
            "docs/development-plan.md",
            "README.md",
            "git diff --check on 2026-09-23",
            "relative link validation for six affected documents on 2026-09-23"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-004",
            "revision": 1,
            "status": "accepted",
            "statement": "阶段一以现有完成度较高、定制化明显的 OA 为参考产品，先从其规格、代码和行为盘点核心旅程，再验证独立小系统能否重组出可用 OA；该阶段不要求用户预先掌握整个行业，不做逐页面逐表复制，也不把单一 OA 验证冒充多客户通用性。",
            "basis": "用户在 2026-09-23 明确说明自身行业认知有限，并确认成功用当前小系统搭建参考 OA 可以作为阶段第一目标。",
            "evidence": [
              "User statement on 2026-09-23",
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/decisions.md#BF-030",
              "docs/foundation.md#13-阶段一目标重组参考-oa"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [
          {
            "id": "IMPACT-20260923-002-003",
            "fact_id": "FACT-20260923-002-004",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_intent_and_scope",
              "revision": 3
            },
            "effect": "upheld",
            "reason": "The product intent now has a concrete first proof target while preserving the longer-term multi-scenario falsification requirement.",
            "gap_ids": [],
            "evidence": [
              "docs/decisions.md#BF-030",
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/foundation.md#13-阶段一目标重组参考-oa"
            ]
          },
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-004",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 1
            },
            "effect": "undetermined",
            "reason": "The current capability portfolio is the candidate assembly, but the OA-wide audit must determine which existing applications, extensions and gaps are required for selected core journeys.",
            "gap_ids": [
              "GAP-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md"
            ]
          },
          {
            "id": "IMPACT-20260923-002-005",
            "fact_id": "FACT-20260923-002-004",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "experience_and_interaction",
              "revision": 1
            },
            "effect": "undetermined",
            "reason": "Unified login and application entry are established, but actual stage-one OA user journeys and recovery states have not yet been selected from reference evidence.",
            "gap_ids": [
              "GAP-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md"
            ]
          }
        ],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-001",
            "fact_id": "FACT-20260923-002-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_intent_and_scope",
              "revision": 3
            },
            "effect": "upheld",
            "reason": "The updated product intent retains the falsifiable long-term success basis while adding the reference-OA stage-one proof target.",
            "gap_ids": [],
            "evidence": [
              "docs/decisions.md#BF-029",
              "docs/decisions.md#BF-030",
              "docs/foundation.md",
              "docs/specs/assembly/product-boundary-validation.md"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-005",
            "status": "open",
            "goal": "Audit the reference OA's complete capability surface and representative end-to-end journeys, map each item to current BizFoundry ownership or an explicit gap, and propose the smallest stage-one core-journey set without implementing new business code.",
            "reason": "The stage-one target is clear, but scope and implementation order must come from traceable OA evidence rather than menu order, roadmap order or the user's incomplete domain memory.",
            "derived_from": [
              "FACT-20260923-002-004"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Determines whether Project Workspace, a connector, approval adaptation or another typed extension is actually needed next",
              "risk": "Prevents a feature clone and prevents existing architecture from remaining a black box",
              "user_impact": "Produces an understandable reference-product map that the user can review without first becoming an industry expert",
              "evidence": "The reference OA repository, its specifications, current behavior and existing BizFoundry audits are available"
            },
            "responsibility": "agent",
            "evidence_required": [
              "A traceable OA capability and journey inventory separates observed behavior, BizFoundry coverage, explicit gaps and unresolved business meaning",
              "Each selected candidate journey states the user result, fact owner, state transitions, failure or correction behavior and six-layer variation placement",
              "A minimal stage-one journey set is proposed with inclusions, exclusions and reasons",
              "The resulting next implementation choice remains a proposal until documented evidence supports it"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [
          {
            "id": "GAP-20260923-002-002",
            "outcome": "Superseded by OA-evidence-first discovery",
            "reason": "The user is not confident enough in the full business domain to author the first case from memory. The reference OA should be audited first, after which only unresolved meanings are returned for human confirmation.",
            "evidence": [
              "User statement on 2026-09-23",
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/development-plan.md"
            ]
          },
          {
            "id": "GAP-20260923-002-003",
            "outcome": "Superseded by whole-reference-product boundary audit",
            "reason": "The former single Contract-to-Money mapping depended on the cancelled human-first gap. The new audit starts from the whole reference OA and may later create narrower evidence gaps.",
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/development-plan.md"
            ]
          }
        ],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_intent_and_scope",
            "observed_revision": 2,
            "set_decision": {
              "status": "settled",
              "statement": "建设小而完整、可独立交付的窄域业务应用组合，不建设万能业务平台；阶段一以现有完成度较高、定制化明显的 OA 为参考产品，按可追溯核心旅程验证小系统重组能力；长期成功仍以多个真实场景中的稳定内核、受控变化和无客户核心分支为依据。",
              "reason": "BF-001、BF-002、BF-017、BF-029 与 BF-030 共同固定长期目标、阶段一验证载体和可证伪边界。",
              "evidence": [
                "docs/decisions.md",
                "docs/foundation.md",
                "docs/specs/assembly/product-boundary-validation.md",
                "docs/specs/assembly/oa-reference-stage-1.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The user selected successful reference-OA reconstruction as the first product-validation milestone while retaining uncertainty about wider market fit.",
            "evidence": [
              "User statement on 2026-09-23",
              "docs/decisions.md#BF-030",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "在 ICP 备案由账号所有者推进期间，先审计现有参考 OA 的全景能力与核心旅程，形成阶段一最小旅程集合和小系统边界映射；审计前不按旧 OA 菜单或路线图编号启动新业务实现。",
          "project_priorities": [
            "完成参考 OA 全景能力与核心旅程审计",
            "完成 Contract-only 独立组合验收",
            "完成 ICP 后复验 D1 外部公网入口",
            "依据参考 OA 证据选择 Project Workspace、连接器、审批适配、类型化扩展或无需新增实现"
          ]
        },
        "evidence": [
          "docs/decisions.md",
          "docs/foundation.md",
          "docs/development-plan.md",
          "docs/specs/assembly/oa-reference-stage-1.md",
          "docs/specs/assembly/product-boundary-validation.md",
          "README.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 11,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Long-term objective, stage-one target, non-goals and acceptance gates are recoverable from stable documents.",
            "fact_refs": [
              "FACT-20260923-002-004"
            ],
            "evidence": [
              "docs/decisions.md#BF-030",
              "docs/foundation.md#13-阶段一目标重组参考-oa",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "undetermined",
            "reason": "The stage target requires representative user journeys, but the reference OA audit has not selected them or documented recovery states.",
            "fact_refs": [
              "FACT-20260923-002-004"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-005"
            ]
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The stage objective concerns business-result reconstruction and does not establish visual parity or a visual language.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The stage specification preserves Nginx, Gateway, IAM, application ownership, versioned-contract and deployment-profile boundaries.",
            "fact_refs": [
              "FACT-20260923-002-004"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/decisions.md",
              "docs/architecture.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "undetermined",
            "reason": "The accepted fact establishes the stage target, not its completion; the capability and journey audit is required before coverage can be judged.",
            "fact_refs": [
              "FACT-20260923-002-004"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-005"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "The gates guard against feature cloning, unsupported generalization, customer core branches, cross-database violations and hidden gaps.",
            "fact_refs": [
              "FACT-20260923-002-004"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "User statement on 2026-09-23",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/decisions.md#BF-030",
        "docs/foundation.md#13-阶段一目标重组参考-oa",
        "docs/development-plan.md",
        "docs/specs/assembly/product-boundary-validation.md",
        "README.md",
        "git diff --check on 2026-09-23",
        "relative link validation for six affected documents on 2026-09-23"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-23T15:38:14.994Z"
    },
    {
      "round": 3,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Complete the reference-OA capability and journey audit, derive the smallest defensible stage-one journey proposal and stop before implementation for product-owner confirmation.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The accepted stage-one target requires an evidence-grounded map of the reference OA before any new business implementation. The repository, current code, confirmed 2026 requirements and prior BizFoundry audits are available and sufficient for that bounded audit.",
        "snapshot_token": "c516df3c0b0d438636996e64bf75f456e1d01f96e3c928c2bc247ce806837743",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-005",
        "comparison_summary": "GAP-20260923-002-005 is the only ready candidate in the selected Case and directly determines whether Project Workspace, a connector or a typed extension is justified. The public-hosting candidate belongs to the separate D1 HTTPS Case and remains human-owned.",
        "fresh_discovery_summary": "The audit found that the reference OA is project-centered, while BizFoundry already covers identity, people, contract, money and evidence. A thin Project Workspace plus a design-project-operations typed extension is the smallest plausible next shape, but the proposed journey set requires product-owner confirmation before specifications or code.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-005",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Determines the next justified business application and its boundary",
              "risk": "Prevents copying the OA menu or hiding customer-specific behavior in a universal Project module",
              "user_impact": "Makes the black-box product direction reviewable through concrete user journeys",
              "evidence": "Reference OA commit 826e0fc, confirmed 2026 requirements, current routes and existing BizFoundry boundary audits"
            },
            "reason": "The capability inventory, journey map and smallest proposed set can be completed from existing evidence without inventing business behavior or implementing code."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-005",
        "responsibility": "agent",
        "goal": "Audit the reference OA's complete capability surface and representative end-to-end journeys, map each item to current BizFoundry ownership or an explicit gap, and propose the smallest stage-one core-journey set without implementing new business code.",
        "reason": "The stage-one target is clear, but scope and implementation order must come from traceable OA evidence rather than menu order, roadmap order or the user's incomplete domain memory.",
        "derived_from": [
          "FACT-20260923-002-004"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Determines whether Project Workspace, a connector, approval adaptation or another typed extension is actually needed next",
          "uncertainty": "",
          "risk": "Prevents a feature clone and prevents existing architecture from remaining a black box",
          "user_impact": "Produces an understandable reference-product map that the user can review without first becoming an industry expert",
          "evidence": "The reference OA repository, its specifications, current behavior and existing BizFoundry audits are available"
        },
        "evidence_required": [
          "A traceable OA capability and journey inventory separates observed behavior, BizFoundry coverage, explicit gaps and unresolved business meaning",
          "Each selected candidate journey states the user result, fact owner, state transitions, failure or correction behavior and six-layer variation placement",
          "A minimal stage-one journey set is proposed with inclusions, exclusions and reasons",
          "The resulting next implementation choice remains a proposal until documented evidence supports it"
        ]
      },
      "planned_transition": {
        "goal": "Complete the reference-OA capability and journey audit, derive the smallest defensible stage-one journey proposal and stop before implementation for product-owner confirmation.",
        "expected_state_change": "The audit becomes durable, GAP-20260923-002-005 resolves, and a human-owned confirmation gap records the three product choices that must be accepted before Project Workspace or the design operations extension is specified."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-005",
          "status": "resolved",
          "outcome": "The reference OA capability surface and representative journeys are mapped to current BizFoundry ownership, explicit gaps and a smallest proposed stage-one set without implementing business code.",
          "reason": "The audit traces the project-centered OA spine, distinguishes current code from newer confirmed requirements and historic specifications, maps each major capability, defines four end-to-end journeys with ownership and recovery behavior, and leaves the resulting scope as a proposal.",
          "evidence": [
            "docs/specs/assembly/oa-reference-capability-journey-audit.md",
            "reference OA commit 826e0fc65aa09947fc4f64f62a2644fa5d11a302",
            "reference OA docs/OA_2026_REQUIREMENTS_CONFIRMED.md",
            "reference OA docs/OA_PERMISSION_MODEL.md",
            "reference OA arckit/spec/INDEX.md and linked capability specifications",
            "docs/specs/connectors/contract-money-oa-reference-audit.md",
            "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
            "git diff --check on affected tracked documents on 2026-09-23",
            "git diff --no-index --check for the new audit document on 2026-09-23",
            "relative link validation for seven affected documents on 2026-09-23"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-005",
            "revision": 1,
            "status": "accepted",
            "statement": "参考 OA 的业务主脊柱是项目立项与项目角色下并行推进经营链、生产链、资料和统计；BizFoundry 已覆盖统一入口、IAM、People、Contract、Money 和 Evidence，主要缺口是薄 Project Workspace 与设计项目运营类型化扩展。阶段一首个集合提案由项目立项、经营合同与回款、生产交付、资料追溯四条旅程组成，M9 仍是下一方向但必须收窄；提案在用户确认前不进入规格或实现。",
            "basis": "Reference OA commit 826e0fc 的当前路由、模型和规格与 2026-08-17 已确认需求，以及 BizFoundry 现有应用边界和参考审计共同支持该结论。",
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md",
              "reference OA commit 826e0fc65aa09947fc4f64f62a2644fa5d11a302",
              "docs/architecture.md",
              "docs/specs/assembly/three-reference-scenarios-boundary-audit.md",
              "docs/specs/connectors/contract-money-oa-reference-audit.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-004",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 1
            },
            "effect": "undetermined",
            "reason": "The audit identifies a thin Project Workspace and a design-project-operations typed extension as the smallest plausible missing capabilities, but the proposed J1-J4 scope has not yet been accepted by the product owner.",
            "gap_ids": [
              "GAP-20260923-002-006"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ]
          },
          {
            "id": "IMPACT-20260923-002-005",
            "fact_id": "FACT-20260923-002-004",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "experience_and_interaction",
              "revision": 1
            },
            "effect": "undetermined",
            "reason": "The audit now documents four representative user journeys and recovery boundaries, but their inclusion and first production-type order remain a product-owner proposal rather than an accepted interaction scope.",
            "gap_ids": [
              "GAP-20260923-002-006"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#6-代表性端到端旅程",
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#9-仍需用户确认的最小问题"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-006",
            "status": "open",
            "goal": "Confirm or revise the proposed stage-one J1-J4 core journey set, select the first production type and decide whether the first Contract or Settlement to Money Flow obligation step is explicitly human-confirmed rather than automatic.",
            "reason": "The audit can establish evidence and options, but these three scope choices determine the product promise and must be accepted by the product owner before formal Project Workspace and design-operations specifications or implementation begin.",
            "derived_from": [
              "FACT-20260923-002-005"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks formal M9 and design-project-operations scope",
              "risk": "Avoids silently turning an audit proposal into product requirements or prematurely automating an unconfirmed financial trigger",
              "user_impact": "Lets the owner make three understandable product choices without needing full industry expertise",
              "evidence": "The audit presents the proposed set, exclusions and tradeoffs in reviewable form"
            },
            "responsibility": "human",
            "evidence_required": [
              "Explicit acceptance or revision of J1 project initiation, J2 business-contract-cash, J3 production delivery and J4 project evidence traceability as the first stage-one set",
              "Selection of design or another reference production type for the first complete production journey and a second type for extensibility proof",
              "Acceptance or revision of explicit human confirmation before Contract or Settlement facts become Money Flow obligations in the first slice"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "等待产品负责人确认参考 OA 阶段一的 J1-J4 首个核心旅程集合、首条生产类型和合同／结算到 Money Flow 的人工确认边界；确认前不创建 Project 或设计运营扩展代码。",
          "project_priorities": [
            "确认或修订阶段一 J1-J4 首个核心旅程集合",
            "确认首条生产类型与第二类型扩展验证对象",
            "确认首切片合同／结算到 Money Flow 义务采用人工明确确认",
            "确认后分别建立薄 Project Workspace 与设计项目运营扩展的正式规格和验收矩阵",
            "完成 Contract-only 独立组合验收",
            "完成 ICP 后复验 D1 外部公网入口"
          ]
        },
        "evidence": [
          "docs/specs/assembly/oa-reference-capability-journey-audit.md",
          "docs/specs/assembly/oa-reference-stage-1.md",
          "docs/specs/assembly/product-boundary-validation.md",
          "docs/development-plan.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 12,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The reference-product goal, complete capability inventory, proposed core set, explicit exclusions and confirmation boundary are recoverable from stable documents.",
            "fact_refs": [
              "FACT-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md",
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/decisions.md#BF-030"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "undetermined",
            "reason": "The proposed journeys, states, failures and recovery behaviors are documented, but the product owner has not yet accepted the set or its first production-type order.",
            "fact_refs": [
              "FACT-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#6-代表性端到端旅程"
            ],
            "gap_refs": [
              "GAP-20260923-002-006"
            ]
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The audit establishes business capability and journey boundaries, not a visual parity or visual-language decision.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The proposed thin Project and typed design-operations split preserves IAM, application ownership, versioned APIs, target-app authorization and Evidence boundaries while explaining why a universal Project module is rejected.",
            "fact_refs": [
              "FACT-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#5-临时六层归属",
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#8-对下一实现顺序的影响",
              "docs/architecture.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "undetermined",
            "reason": "The accepted fact proves the audit and proposal exist; stage-one user journeys are not implemented and cannot be planned as accepted scope until the owner resolves the confirmation gap.",
            "fact_refs": [
              "FACT-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-006"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "The audit identifies stale-source conflicts, capability gaps, unsupported automation, universal-Project risk and explicitly deferred customer-specific formulas and advanced modules.",
            "fact_refs": [
              "FACT-20260923-002-005"
            ],
            "evidence": [
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#2-证据层级与冲突处理",
              "docs/specs/assembly/oa-reference-capability-journey-audit.md#7-阶段一首个核心集合提案",
              "docs/specs/connectors/contract-money-oa-reference-audit.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "docs/specs/assembly/oa-reference-capability-journey-audit.md",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/development-plan.md",
        "README.md",
        "reference OA commit 826e0fc65aa09947fc4f64f62a2644fa5d11a302",
        "git diff --check on affected tracked documents on 2026-09-23",
        "git diff --no-index --check for the new audit document on 2026-09-23",
        "relative link validation for seven affected documents on 2026-09-23"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-23T15:48:35.421Z"
    },
    {
      "round": 4,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Accept the stage-one OA demo scope and establish the next specification gap without changing the complete-suite ingress architecture.",
      "outcome": "partial",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The owner explicitly accepted a local-first minimum OA demo, authorized proceeding from the documented analysis, selected design as the first production type, and required business-architecture validation before the complete ingress chain.",
        "snapshot_token": "0580e700e04fdf44d72c143049e064c9e3f017253793b2f615bba2b91515e16c",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-006",
        "comparison_summary": "Within CASE-20260923-002 this is the only persisted candidate. The public ICP and company-environment candidates belong to separate delivery cases and do not block local business-architecture validation.",
        "fresh_discovery_summary": "The accepted choice adds a two-stage validation order: local business journeys first with contract-compatible local IAM, then the unchanged complete Nginx-Gateway-IAM deployment chain. The next independent gap is to formalize J1 before runtime implementation.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-006",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "reason": "The user's latest direction resolves the outstanding product-owner choices and unblocks a concrete local OA demo.",
            "priority_basis": {
              "blocking": "Unblocks formal Project Workspace and design-operation work",
              "risk": "Prevents implementing an implicit or overbroad OA promise",
              "user_impact": "Turns the black-box architecture concern into a demonstrable business journey",
              "evidence": "The user explicitly authorized the local-first minimum demo and later full-entry validation"
            }
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-006",
        "responsibility": "human",
        "goal": "Confirm or revise the proposed stage-one J1-J4 core journey set, select the first production type and decide whether the first Contract or Settlement to Money Flow obligation step is explicitly human-confirmed rather than automatic.",
        "reason": "The audit can establish evidence and options, but these three scope choices determine the product promise and must be accepted by the product owner before formal Project Workspace and design-operations specifications or implementation begin.",
        "derived_from": [
          "FACT-20260923-002-005"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks formal M9 and design-project-operations scope",
          "uncertainty": "",
          "risk": "Avoids silently turning an audit proposal into product requirements or prematurely automating an unconfirmed financial trigger",
          "user_impact": "Lets the owner make three understandable product choices without needing full industry expertise",
          "evidence": "The audit presents the proposed set, exclusions and tradeoffs in reviewable form"
        },
        "evidence_required": [
          "Explicit acceptance or revision of J1 project initiation, J2 business-contract-cash, J3 production delivery and J4 project evidence traceability as the first stage-one set",
          "Selection of design or another reference production type for the first complete production journey and a second type for extensibility proof",
          "Acceptance or revision of explicit human confirmation before Contract or Settlement facts become Money Flow obligations in the first slice"
        ]
      },
      "planned_transition": {
        "goal": "Accept the stage-one OA demo scope and establish the next specification gap without changing the complete-suite ingress architecture.",
        "expected_state_change": "J1-J4, design-first, human-confirmed obligations, and local-business-first validation become durable accepted facts; GAP-006 resolves and a J1 specification gap becomes the next Agent-owned candidate."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-006",
          "status": "resolved",
          "outcome": "The owner accepted J1-J4 as the first demo set, design as the first production type, explicit human confirmation before Money Flow obligation creation, and local business-architecture validation before full ingress validation.",
          "reason": "The latest direction explicitly authorizes the minimum local OA demo, defers the second production-type proof until that demo passes, and preserves the complete Nginx-Gateway-IAM chain for later environment validation.",
          "evidence": [
            "User statement on 2026-09-25",
            "docs/decisions.md#BF-031",
            "arckit/spec/assembly/reference-oa-demo.md",
            "docs/specs/assembly/oa-reference-stage-1.md"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-006",
            "revision": 1,
            "status": "accepted",
            "statement": "阶段一首个演示接受 J1 项目立项、J2 合同与回款、J3 设计生产交付、J4 资料追溯四条旅程；设计是第一生产类型，第二生产类型在首个演示通过后再选；合同或结算事实进入 Money Flow 前必须由责任人明确确认；先用各应用的本地模板 IAM 与独立入口验证业务架构，之后使用同一业务发布物验证 Nginx、Gateway 与中心 IAM 完整入口链。",
            "basis": "用户在 2026-09-25 明确要求先完成最小、稳定、不出错的本地 OA 演示，允许完整入口链后移并计划在公司私有 Ubuntu 环境验证。",
            "evidence": [
              "User statement on 2026-09-25",
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/foundation.md",
              "docs/development-plan.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-006",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 2
            },
            "effect": "upheld",
            "reason": "The accepted scope makes a thin Project Workspace and design production extension explicit stage-one capabilities while retaining existing application ownership boundaries.",
            "gap_ids": [],
            "evidence": [
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ]
          },
          {
            "id": "IMPACT-20260923-002-005",
            "fact_id": "FACT-20260923-002-006",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "experience_and_interaction",
              "revision": 2
            },
            "effect": "upheld",
            "reason": "J1-J4 and their recovery boundaries are now accepted, and local direct entry is explicitly a validation mode using the same IAM contract rather than an authentication bypass.",
            "gap_ids": [],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md#BF-031",
              "docs/specs/assembly/oa-reference-capability-journey-audit.md"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-007",
            "status": "open",
            "goal": "把已接受的 J1 项目立项范围细化为薄 Project Workspace 的稳定产品规格、数据与 API 契约、权限边界和可执行验收矩阵，不在同一轮提前实现运行时代码。",
            "reason": "Project Workspace 尚不存在；先固定最小事实、不变量、失败恢复和不负责范围，才能避免把参考 OA 的所有字段与流程复制成万能 Project 平台。",
            "derived_from": [
              "FACT-20260923-002-006"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the first new runtime slice of the accepted demo",
              "risk": "Prevents project scope creep, cross-application ownership leakage and premature framework extraction",
              "user_impact": "Defines the smallest visible project-initiation result that the demo must reliably show",
              "evidence": "The accepted reference demo and existing application contracts provide a bounded specification input"
            },
            "responsibility": "agent",
            "evidence_required": [
              "Stable Project Workspace product specification states Project, Assignment, resource-link and history behavior",
              "Stable technical contract defines owned data, versioned APIs, idempotency, authorization and cross-application references",
              "Executable acceptance matrix covers success, duplicate, unauthorized, invalid reference and history cases",
              "The specification preserves direct local entry with template IAM and the later complete-suite entry chain"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 1,
            "set_decision": {
              "status": "settled",
              "statement": "当前完整组合包含统一入口与工作台、IAM、People & Groups、Contract Ledger、Money Flow Ledger、Evidence Service，以及阶段一需要实现的薄 Project Workspace 和设计生产类型扩展。首个演示覆盖 J1-J4；设计先行，第二生产类型在首个演示通过后再选择。",
              "reason": "BF-031 与参考 OA 演示规格把审计提案收敛为已接受的阶段一能力集合，同时明确不建设万能 Project 平台。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "arckit/spec/assembly/reference-oa-demo.md",
                "docs/specs/assembly/oa-reference-stage-1.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [
              "GAP-20260923-002-007"
            ],
            "reason": "The user accepted the J1-J4 demo scope and design-first order.",
            "evidence": [
              "User statement on 2026-09-25",
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md"
            ]
          },
          {
            "area_ref": "experience_and_interaction",
            "observed_revision": 1,
            "set_decision": {
              "status": "settled",
              "statement": "完整组合提供统一登录和工作台式应用入口；阶段一本地业务验证允许各应用通过独立本地入口和版本化模板 IAM 完成 J1-J4，但不得关闭鉴权或改变业务事实。J2 的合同或结算事实只有经责任人明确确认后才能形成 Money Flow 义务；之后同一业务发布物再验证完整 Nginx、Gateway 与中心 IAM 入口链。",
              "reason": "BF-031 将业务架构验证与部署入口验证分阶段，既减少首个演示变量，也保留正式交互和鉴权契约。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "arckit/spec/assembly/reference-oa-demo.md",
                "docs/specs/assembly/oa-reference-capability-journey-audit.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [
              "GAP-20260923-002-007"
            ],
            "reason": "The user accepted the four journeys, human confirmation point and local-first validation sequence.",
            "evidence": [
              "User statement on 2026-09-25",
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "先在本地以模板 IAM 和独立应用入口完成参考 OA 的最小业务架构演示；立即先固定并实现 J1 薄 Project Workspace，再按 J3、J2、J4 接入现有小系统。Nginx、Gateway、中心 IAM、公司私有 Ubuntu 与公网入口作为第二阶段部署架构验证。",
          "project_priorities": [
            "固定 J1 薄 Project Workspace 规格与验收矩阵",
            "实现并验证 J1 本地纵向闭环",
            "实现设计生产类型并完成 J3",
            "连接 Contract、Money Flow 与 Evidence 完成 J2 和 J4",
            "通过本地完整 OA 演示后在公司私有 Ubuntu 验证完整入口链"
          ]
        },
        "evidence": [
          "docs/decisions.md#BF-031",
          "docs/foundation.md",
          "docs/development-plan.md",
          "arckit/spec/assembly/reference-oa-demo.md",
          "docs/specs/assembly/oa-reference-stage-1.md",
          "README.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 15,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The accepted journey set, production order, human confirmation point, exclusions and validation stages are durable in the decision index and reference-demo specification.",
            "fact_refs": [
              "FACT-20260923-002-006"
            ],
            "evidence": [
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "J1-J4 user results, confirmation and recovery behavior, and local-versus-complete entry modes are explicitly documented.",
            "fact_refs": [
              "FACT-20260923-002-006"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/specs/assembly/oa-reference-capability-journey-audit.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This scope decision does not establish or revise visual-language rules.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The local validation mode preserves versioned IAM and application ownership while the formal complete-suite ingress architecture remains unchanged and explicitly deferred rather than removed.",
            "fact_refs": [
              "FACT-20260923-002-006"
            ],
            "evidence": [
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/architecture.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "threatened",
            "reason": "The scope is accepted and durable, but the thin Project Workspace and the complete J1-J4 demo are not yet implemented.",
            "fact_refs": [
              "FACT-20260923-002-006"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-007"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "The documents explicitly control authentication bypass, cross-application ownership, automatic financial obligation creation, scope creep and false claims about full deployment validation.",
            "fact_refs": [
              "FACT-20260923-002-006"
            ],
            "evidence": [
              "docs/decisions.md#BF-031",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/foundation.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "User statement on 2026-09-25",
        "docs/decisions.md#BF-031",
        "arckit/spec/assembly/reference-oa-demo.md",
        "docs/foundation.md",
        "docs/development-plan.md",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/specs/assembly/oa-reference-capability-journey-audit.md",
        "README.md",
        "git diff --check on affected documents on 2026-09-25",
        "relative Markdown link validation on affected documents on 2026-09-25"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T17:06:37.362Z"
    },
    {
      "round": 5,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Accept the J1 specification package and establish runtime implementation as the next evidence obligation.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "GAP-007 is the only ready candidate in CASE-20260923-002 and the required stable product and technical artifacts have now been created and validated.",
        "snapshot_token": "c3da9e12a81dc634c45914bc00d4210c154eefefa40f99ddfb659781171fd3fd",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-007",
        "comparison_summary": "The selected Case has one ready gap. Public ICP and company-environment validation remain separate human-owned delivery cases and do not supersede J1 business implementation.",
        "fresh_discovery_summary": "The formal specification confirms that a minimal independent Project Workspace can implement J1 without copying People, Contract, Money or Evidence facts. The next independent gap is runtime realization and J1-A01 through J1-A18 evidence.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-007",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "reason": "All four required specification evidence categories are present and checked.",
            "priority_basis": {
              "blocking": "Unblocks the first runtime slice of the OA demo",
              "risk": "Fixes ownership, auth, idempotency and failure behavior before code",
              "user_impact": "Defines exactly what the first demonstrable project-initiation journey must do",
              "evidence": "Product spec, technical solution, models, API contracts and acceptance matrix are durable"
            }
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-007",
        "responsibility": "agent",
        "goal": "把已接受的 J1 项目立项范围细化为薄 Project Workspace 的稳定产品规格、数据与 API 契约、权限边界和可执行验收矩阵，不在同一轮提前实现运行时代码。",
        "reason": "Project Workspace 尚不存在；先固定最小事实、不变量、失败恢复和不负责范围，才能避免把参考 OA 的所有字段与流程复制成万能 Project 平台。",
        "derived_from": [
          "FACT-20260923-002-006"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks the first new runtime slice of the accepted demo",
          "uncertainty": "",
          "risk": "Prevents project scope creep, cross-application ownership leakage and premature framework extraction",
          "user_impact": "Defines the smallest visible project-initiation result that the demo must reliably show",
          "evidence": "The accepted reference demo and existing application contracts provide a bounded specification input"
        },
        "evidence_required": [
          "Stable Project Workspace product specification states Project, Assignment, resource-link and history behavior",
          "Stable technical contract defines owned data, versioned APIs, idempotency, authorization and cross-application references",
          "Executable acceptance matrix covers success, duplicate, unauthorized, invalid reference and history cases",
          "The specification preserves direct local entry with template IAM and the later complete-suite entry chain"
        ]
      },
      "planned_transition": {
        "goal": "Accept the J1 specification package and establish runtime implementation as the next evidence obligation.",
        "expected_state_change": "GAP-007 resolves with stable product, technical, model, API and acceptance artifacts; GAP-008 becomes the Agent-owned implementation candidate."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-007",
          "status": "resolved",
          "outcome": "J1 now has a stable product specification, technical solution, four owned data models, eight versioned API contracts and an eighteen-case executable acceptance matrix.",
          "reason": "The artifacts define Project ownership, states, commands, IAM boundary, external references, failure recovery, deployment portability and explicit exclusions without implementing runtime code in the same round.",
          "evidence": [
            "arckit/spec/project-workspace/project-initiation.md",
            "arckit/tech/project-workspace/solution.md",
            "arckit/tech/_shared/models/Project.yaml",
            "arckit/tech/_shared/models/ProjectAssignment.yaml",
            "arckit/tech/_shared/models/ProjectResourceLink.yaml",
            "arckit/tech/_shared/models/ProjectHistoryEvent.yaml",
            "arckit/tech/_shared/contracts/project-create.yaml",
            "arckit/tech/_shared/contracts/project-update.yaml",
            "arckit/tech/_shared/contracts/project-assignment-put.yaml",
            "arckit/tech/_shared/contracts/project-resource-link-put.yaml",
            "arckit/tech/_shared/contracts/project-initiation-complete.yaml",
            "arckit/tech/_shared/contracts/project-cancel.yaml",
            "arckit/tech/_shared/contracts/project-get.yaml",
            "arckit/tech/_shared/contracts/project-history-list.yaml",
            "docs/testing.md",
            "YAML parse validation and git diff --check on 2026-09-25",
            "relative Markdown link validation on 2026-09-25"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-007",
            "revision": 1,
            "status": "accepted",
            "statement": "J1 薄 Project Workspace 的正式规格已经固定：独立服务只拥有 Project、Assignment、类型化 Resource Link、幂等命令和不可变历史；本地模板 IAM 与正式 IAM 使用同形 v1 契约；四个模型、八个 API 及 J1-A01 至 J1-A18 验收矩阵明确覆盖成功、幂等、并发、越权、非法引用、上游故障、取消、历史与独立运行。运行时代码尚未实现。",
            "basis": "已接受的 J1 范围已经被展开为可验证的产品规则、技术边界、模型和 API，且通过结构与链接检查。",
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/tech/project-workspace/solution.md",
              "arckit/tech/INDEX.md",
              "arckit/spec/INDEX.md",
              "docs/testing.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-007",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 3
            },
            "effect": "upheld",
            "reason": "The Project capability is now bounded by stable specifications and contracts; runtime realization remains explicitly open rather than implied.",
            "gap_ids": [
              "GAP-20260923-002-008"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/tech/project-workspace/solution.md",
              "docs/testing.md"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-008",
            "status": "open",
            "goal": "实现并验证 J1 薄 Project Workspace 最小纵向闭环，包括独立 Go 服务、PostgreSQL migration、v1 模板／正式 IAM 边界、Project／Assignment／Resource Link／历史 API、独立交付入口和 J1-A01 至 J1-A18 证据。",
            "reason": "稳定规格已经完成，但实际软件尚不能创建、指派、立项、取消或追溯 Project，阶段一 OA Demo 仍缺第一条运行时业务脊柱。",
            "derived_from": [
              "FACT-20260923-002-007"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks all later J2-J4 project-context composition",
              "risk": "Implementation is the first proof that the thin boundary works without cross-database ownership leakage",
              "user_impact": "Produces the first usable local OA journey rather than another design artifact",
              "evidence": "The complete J1 specification and existing Go application patterns make implementation ready"
            },
            "responsibility": "agent",
            "evidence_required": [
              "Independent Project Workspace Go module, migrations, configuration, health endpoint and local PostgreSQL startup are implemented",
              "All J1 command, query, IAM and external-reference behaviors are covered by proportional unit and integration tests",
              "J1-A01 through J1-A18 have direct test or repeatable acceptance evidence, with any environment-only item explicitly separated",
              "Existing People, Contract, Money and Evidence tests remain unaffected and Project owns no copied external facts",
              "Testing and progress documents record only checks actually run"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 2,
            "set_decision": {
              "status": "settled",
              "statement": "当前完整组合包含统一入口与工作台、IAM、People & Groups、Contract Ledger、Money Flow Ledger、Evidence Service，以及阶段一需要实现的薄 Project Workspace 和设计生产类型扩展。首个演示覆盖 J1-J4；设计先行，第二生产类型在首个演示通过后再选择。J1 的产品规格、技术方案、数据模型、API 契约与验收矩阵已经固定，运行时尚未实现。",
              "reason": "BF-031 与 J1 正式规格共同固定阶段一能力集合和 Project 最小边界，同时明确规格完成不等于软件完成。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "arckit/spec/project-workspace/project-initiation.md",
                "arckit/tech/project-workspace/solution.md",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [
              "GAP-20260923-002-008"
            ],
            "reason": "J1 has moved from accepted scope to a complete stable specification package while runtime remains open.",
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/tech/project-workspace/solution.md",
              "arckit/tech/INDEX.md",
              "arckit/spec/INDEX.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "按已经固定的 J1 规格实现薄 Project Workspace 的本地最小纵向闭环，以模板 IAM 和独立入口取得 J1-A01 至 J1-A18 证据；完成前不启动设计运营、J2-J4 或完整入口链集成。",
          "project_priorities": [
            "实现并验证 J1 薄 Project Workspace",
            "实现设计生产类型并完成 J3",
            "连接 Contract、Money Flow 与 Evidence 完成 J2 和 J4",
            "通过本地完整 OA Demo 后在公司私有 Ubuntu 验证完整入口链",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "arckit/spec/project-workspace/project-initiation.md",
          "arckit/tech/project-workspace/solution.md",
          "arckit/spec/INDEX.md",
          "arckit/tech/INDEX.md",
          "docs/development-plan.md",
          "docs/progress.md",
          "docs/testing.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 16,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "J1 user outcomes, owned facts, exclusions, states and acceptance are recoverable from the stable product specification.",
            "fact_refs": [
              "FACT-20260923-002-007"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/spec/INDEX.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Create, assign, complete, link, cancel, failure and recovery states are defined as Given/When/Then behavior and API outcomes.",
            "fact_refs": [
              "FACT-20260923-002-007"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This round defines business and technical behavior, not a visual design system or page composition.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Service ownership, tables, transaction order, IAM enforcement, API errors, reference validation and deployment portability are explicit and traceable to concrete contracts.",
            "fact_refs": [
              "FACT-20260923-002-007"
            ],
            "evidence": [
              "arckit/tech/project-workspace/solution.md",
              "arckit/tech/_shared/models/Project.yaml",
              "arckit/tech/_shared/contracts/project-create.yaml"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "threatened",
            "reason": "The specifications are realized as durable artifacts, but the accepted J1 software behavior has no runtime implementation or test evidence yet.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-007"
            ],
            "evidence": [
              "docs/testing.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-008"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "The acceptance matrix directly covers ownership leakage, authentication bypass, idempotency conflict, stale revision, invalid cross-app reference, upstream outage, immutable history and independent operation.",
            "fact_refs": [
              "FACT-20260923-002-007"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md#j1-可执行验收矩阵"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "arckit/spec/project-workspace/project-initiation.md",
        "arckit/tech/project-workspace/solution.md",
        "arckit/spec/INDEX.md",
        "arckit/tech/INDEX.md",
        "arckit/_map/feature-matrix.md",
        "arckit/_map/RELATIONS.md",
        "docs/specs/assembly/oa-reference-capability-journey-audit.md",
        "docs/specs/assembly/oa-reference-stage-1.md",
        "docs/development-plan.md",
        "docs/progress.md",
        "docs/testing.md",
        "YAML parse validation and git diff --check on 2026-09-25",
        "relative Markdown link validation on 2026-09-25"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T17:16:15.293Z"
    },
    {
      "round": 6,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Finish the independent J1 Project Workspace runtime and its credible delivery evidence without prematurely integrating D1 or implementing later OA journeys.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "J1 是当前唯一可立即完成的 Case 内持久候选，也是 J2-J4 组合的运行时前提；公网和公司环境事项分别受备案与授权控制，不应阻塞本地业务架构验证。",
        "snapshot_token": "edd6ea6213d1487ffed0217db93fe9598cb9b6b84852817eedfb39e38e64fc43",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-008",
        "comparison_summary": "本 Case 范围内只有 GAP-20260923-002-008 一个 ready 持久候选，故选择完成 J1；其他 active Case 的备案和公司环境候选不属于本轮 Case scope。",
        "fresh_discovery_summary": "J1 完成后暴露出下一条业务脊柱是 J3 设计生产交付；真实应用 reference-read 端点和演示页面仍未完成，将在后续 fresh-read 中按依赖拆分。",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-008",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks all later J2-J4 project-context composition",
              "risk": "First proof that the thin Project boundary works without copying other systems' facts",
              "user_impact": "Produces the first runnable local OA business spine"
            },
            "reason": "The accepted specification and current implementation evidence now make the complete J1 claim directly verifiable."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-008",
        "responsibility": "agent",
        "goal": "实现并验证 J1 薄 Project Workspace 最小纵向闭环，包括独立 Go 服务、PostgreSQL migration、v1 模板／正式 IAM 边界、Project／Assignment／Resource Link／历史 API、独立交付入口和 J1-A01 至 J1-A18 证据。",
        "reason": "稳定规格已经完成，但实际软件尚不能创建、指派、立项、取消或追溯 Project，阶段一 OA Demo 仍缺第一条运行时业务脊柱。",
        "derived_from": [
          "FACT-20260923-002-007"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks all later J2-J4 project-context composition",
          "uncertainty": "",
          "risk": "Implementation is the first proof that the thin boundary works without cross-database ownership leakage",
          "user_impact": "Produces the first usable local OA journey rather than another design artifact",
          "evidence": "The complete J1 specification and existing Go application patterns make implementation ready"
        },
        "evidence_required": [
          "Independent Project Workspace Go module, migrations, configuration, health endpoint and local PostgreSQL startup are implemented",
          "All J1 command, query, IAM and external-reference behaviors are covered by proportional unit and integration tests",
          "J1-A01 through J1-A18 have direct test or repeatable acceptance evidence, with any environment-only item explicitly separated",
          "Existing People, Contract, Money and Evidence tests remain unaffected and Project owns no copied external facts",
          "Testing and progress documents record only checks actually run"
        ]
      },
      "planned_transition": {
        "goal": "Finish the independent J1 Project Workspace runtime and its credible delivery evidence without prematurely integrating D1 or implementing later OA journeys.",
        "expected_state_change": "The J1 runtime becomes an accepted implemented fact, the realization impact is updated, the completed gap is resolved, and J3 specification becomes the next open Case gap."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-008",
          "status": "resolved",
          "outcome": "J1 is implemented as an independently runnable and portable Project Workspace vertical slice with direct acceptance evidence.",
          "reason": "The Go service, PostgreSQL schema, IAM providers, eight APIs, reference failure behavior, idempotency, immutable history, Docker image, backup/restore and Linux release all passed proportional checks without joining D1 or copying external facts.",
          "evidence": [
            "backend/services/project-workspace",
            "arckit/spec/project-workspace/project-initiation.md",
            "arckit/tech/project-workspace/solution.md",
            "docs/testing.md#M9-J1-项目立项验收",
            "go test -race ./backend/services/project-workspace/... on 2026-09-25: passed",
            "PostgreSQL 18 integration go test -race -tags=integration ./backend/services/project-workspace/... on 2026-09-25: passed",
            "Project plus People, Contract, Money and Evidence regression on 2026-09-25: passed",
            "Project PostgreSQL custom archive restored into a fresh empty database on 2026-09-25: migration, Project and History counts verified",
            "Linux amd64 v0.1.0 release checksum and file manifest on 2026-09-25: passed",
            "omsbizfoundry/project-workspace:j1-local non-root container health and create-project smoke on 2026-09-25: passed"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-008",
            "revision": 1,
            "status": "accepted",
            "statement": "J1 薄 Project Workspace 已实现为独立 Go/PostgreSQL 服务：Project、Assignment、类型化 Resource Link、幂等命令和不可变历史由八个 v1 API 提供；模板与正式 IAM 保持同形授权；上游引用故障不破坏既有读取；J1-A01 至 J1-A18、真库、并发、跨应用回归、Docker、备份恢复和 Linux release 已通过。它尚未进入 D1，现有业务应用的全部 reference-read 端点与最终演示页面仍未完成。",
            "basis": "运行代码、真实 PostgreSQL／HTTP 测试、跨应用回归、容器冒烟、空库恢复和发布包校验共同形成直接证据。",
            "evidence": [
              "backend/services/project-workspace",
              "docs/testing.md#M9-J1-项目立项验收",
              "docs/progress.md#M9-J1-Project-Workspace-独立业务切片",
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/tech/project-workspace/solution.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-008",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 4
            },
            "effect": "upheld",
            "reason": "The thin Project capability is now both durably specified and realized as independently tested software; later J2-J4 composition remains explicitly separate.",
            "gap_ids": [],
            "evidence": [
              "backend/services/project-workspace",
              "arckit/spec/project-workspace/project-initiation.md",
              "docs/testing.md#M9-J1-项目立项验收"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-009",
            "status": "open",
            "goal": "固定 J3 设计生产与交付的最小稳定规格、数据所有权、状态机、类型化 API 和可执行验收矩阵，使其以 Project ID 和 Evidence File ID 组合但不复制 Project、People、Contract、Money 或文件事实。",
            "reason": "J1 已提供项目身份和责任脊柱；阶段一 OA Demo 当前最大的业务缺口是设计工作项、生产阶段、成果要求和交付事实，直接编码前必须先固定不变量和验收边界。",
            "derived_from": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the J3 journey and the first recognizable OA production demo",
              "uncertainty": "The minimal boundary must avoid becoming a generic task or workflow platform",
              "risk": "Coding without stable ownership would either copy Project/Evidence facts or over-generalize customer-specific production flows",
              "user_impact": "Turns the existing ledgers and Project identity into an actual design-business journey",
              "evidence": "The accepted reference OA audit and completed J1 boundary provide enough input for a bounded specification"
            },
            "responsibility": "agent",
            "evidence_required": [
              "Stable product specification states J3 user outcomes, owned facts, exclusions, states, corrections and failure recovery",
              "Technical solution and versioned models/API contracts preserve application ownership and configurable endpoints",
              "Acceptance matrix covers the minimum happy path, authorization, idempotency, concurrency, invalid references, Evidence outage and immutable delivery history",
              "Reference OA Demo, indexes, relations, plan and testing documents remain synchronized without claiming runtime completion"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 3,
            "set_decision": {
              "status": "settled",
              "statement": "当前完整组合包含统一入口与工作台、IAM、People & Groups、Contract Ledger、Money Flow Ledger、Evidence Service、已实现 J1 的薄 Project Workspace，以及阶段一下一项需要实现的设计生产类型扩展。首个演示覆盖 J1-J4；设计先行，第二生产类型在首个演示通过后再选择。J1 已通过独立运行、真库、IAM、外部引用故障、Docker、备份恢复和 Linux release 验证，但尚未进入 D1；J3 运行时、跨应用 reference-read 和最终演示页面仍未完成。",
              "reason": "BF-031、J1 稳定规格和 2026-09-25 的直接运行证据共同固定当前能力实况与下一业务缺口。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "arckit/spec/project-workspace/project-initiation.md",
                "arckit/tech/project-workspace/solution.md",
                "backend/services/project-workspace",
                "docs/testing.md#M9-J1-项目立项验收"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The previous decision still said the J1 runtime did not exist; direct implementation evidence now makes that statement stale.",
            "evidence": [
              "backend/services/project-workspace",
              "docs/testing.md#M9-J1-项目立项验收"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "固定 J3 设计生产与交付的最小稳定边界和验收契约，再实现其本地业务纵向闭环；J1 已完成，暂不把 Project 接入 D1 或提前要求 Nginx／Gateway／中心 IAM。",
          "project_priorities": [
            "固定并实现 J3 设计生产交付",
            "补齐业务应用 reference-read 并连接 J2 与 J4",
            "形成可操作的本地参考 OA Demo",
            "本地业务 Demo 通过后在公司私有 Ubuntu 验证完整入口链",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "FACT-20260923-002-008",
          "backend/services/project-workspace",
          "docs/development-plan.md",
          "docs/progress.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 17,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "J1 outcomes, exclusions and actual completion state are synchronized across the stable specification, indexes and progress evidence.",
            "fact_refs": [
              "FACT-20260923-002-008"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "arckit/spec/INDEX.md",
              "docs/progress.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The J1 command, query, failure and recovery behavior remains explicit and is now exercised through HTTP acceptance tests.",
            "fact_refs": [
              "FACT-20260923-002-008"
            ],
            "evidence": [
              "arckit/spec/project-workspace/project-initiation.md",
              "backend/services/project-workspace/internal/httpserver/system_integration_test.go"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "J1 intentionally completes an API business slice; no visual language or cross-application page composition was added in this round.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Runtime ownership, transaction boundaries, reference validation, IAM, portability, backup and release behavior are documented and traceable to executable code.",
            "fact_refs": [
              "FACT-20260923-002-008"
            ],
            "evidence": [
              "arckit/tech/project-workspace/solution.md",
              "backend/services/project-workspace",
              "docs/testing.md#M9-J1-项目立项验收"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "threatened",
            "reason": "J1 is now realized, but the accepted J1-J4 reference OA Demo still lacks J3 design production, cross-application composition and a final operator-facing journey.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008"
            ],
            "evidence": [
              "docs/testing.md#M9-J1-项目立项验收",
              "arckit/spec/assembly/reference-oa-demo.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-009"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Authorization, ownership leakage, idempotency, optimistic concurrency, invalid or unavailable upstream references, immutable history, regression, container startup, release integrity and database recovery have direct evidence.",
            "fact_refs": [
              "FACT-20260923-002-008"
            ],
            "evidence": [
              "go test -race ./backend/services/project-workspace/... on 2026-09-25: passed",
              "PostgreSQL 18 integration tests on 2026-09-25: passed",
              "Cross-application regression on 2026-09-25: passed",
              "Fresh-database restore and Linux release checksum on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "backend/services/project-workspace",
        "arckit/spec/project-workspace/project-initiation.md",
        "arckit/tech/project-workspace/solution.md",
        "docs/testing.md#M9-J1-项目立项验收",
        "docs/progress.md#M9-J1-Project-Workspace-独立业务切片",
        "J1 unit, race, PostgreSQL integration, cross-application regression, Docker smoke, backup/restore and Linux release checks on 2026-09-25"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T17:54:46.271Z"
    },
    {
      "round": 7,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Accept one stable, typed J3 design-production boundary and its executable acceptance contract without claiming or performing runtime implementation in the same round.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "J3 稳定边界是当前 Case 内唯一 ready 候选，也是避免把参考 OA 设计生产需求误塞进通用 Project 或万能工作流的实现前置；公网与公司环境分别由其他 Case 管理，不阻塞本地业务规格推进。",
        "snapshot_token": "ddd542717a6505c3c8192a6a25e711cf49157ef72f7dd6d784b39fa44642261e",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-009",
        "comparison_summary": "本 Case 范围内只有 GAP-20260923-002-009 一个 ready 持久候选，故选择固定 J3；其他 active Case 的备案与公司环境候选不属于本轮 Case scope。",
        "fresh_discovery_summary": "规格完成后，下一项直接可执行工作是实现 Design Operations 独立运行时，并补齐实际组合所需的 Project／Evidence reference-read API；该下游结果留待 fresh-read 后单独选择。",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-009",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks the J3 journey and first recognizable OA production demo",
              "uncertainty": "The minimal boundary must avoid a generic task or workflow platform",
              "risk": "Unbounded implementation would copy Project/Evidence facts or hard-code a customer workflow",
              "user_impact": "Creates a stable implementable contract for the first design-production journey"
            },
            "reason": "J1 and the reference OA journey now provide enough accepted facts to settle one bounded design-production type before implementation."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-009",
        "responsibility": "agent",
        "goal": "固定 J3 设计生产与交付的最小稳定规格、数据所有权、状态机、类型化 API 和可执行验收矩阵，使其以 Project ID 和 Evidence File ID 组合但不复制 Project、People、Contract、Money 或文件事实。",
        "reason": "J1 已提供项目身份和责任脊柱；阶段一 OA Demo 当前最大的业务缺口是设计工作项、生产阶段、成果要求和交付事实，直接编码前必须先固定不变量和验收边界。",
        "derived_from": [
          "FACT-20260923-002-006",
          "FACT-20260923-002-008"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks the J3 journey and the first recognizable OA production demo",
          "uncertainty": "The minimal boundary must avoid becoming a generic task or workflow platform",
          "risk": "Coding without stable ownership would either copy Project/Evidence facts or over-generalize customer-specific production flows",
          "user_impact": "Turns the existing ledgers and Project identity into an actual design-business journey",
          "evidence": "The accepted reference OA audit and completed J1 boundary provide enough input for a bounded specification"
        },
        "evidence_required": [
          "Stable product specification states J3 user outcomes, owned facts, exclusions, states, corrections and failure recovery",
          "Technical solution and versioned models/API contracts preserve application ownership and configurable endpoints",
          "Acceptance matrix covers the minimum happy path, authorization, idempotency, concurrency, invalid references, Evidence outage and immutable delivery history",
          "Reference OA Demo, indexes, relations, plan and testing documents remain synchronized without claiming runtime completion"
        ]
      },
      "planned_transition": {
        "goal": "Accept one stable, typed J3 design-production boundary and its executable acceptance contract without claiming or performing runtime implementation in the same round.",
        "expected_state_change": "The J3 specification gap becomes resolved, a durable fact and accepted capability decision record the new boundary, and an implementation gap becomes the next open Case obligation."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-009",
          "status": "resolved",
          "outcome": "J3 is fixed as an independent Design Operations capability with one versioned design-v1 type, explicit ownership, state and correction rules, ten v1 APIs and J3-A01 through J3-A20 acceptance criteria.",
          "reason": "The stable product and technical artifacts now make the minimum business result, ownership exclusions, failure behavior and proof threshold recoverable without pretending the runtime already exists.",
          "evidence": [
            "arckit/spec/design-operations/design-production-delivery.md",
            "arckit/tech/design-operations/solution.md",
            "arckit/tech/_shared/models/DesignWorkItem.yaml",
            "arckit/tech/_shared/models/DesignStage.yaml",
            "arckit/tech/_shared/models/DesignDeliverable.yaml",
            "arckit/tech/_shared/models/DesignHistoryEvent.yaml",
            "arckit/tech/_shared/contracts/design-*.yaml",
            "docs/decisions.md#BF-032",
            "docs/testing.md#M9-J3-设计生产与交付验收待执行"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-009",
            "revision": 1,
            "status": "accepted",
            "statement": "J3 稳定边界已经固定：独立 Design Operations 以代码拥有的 design-v1 类型管理设计 Work Item、方案／初设／施工图／施工配合四阶段、明确不适用、Evidence 成果引用、显式重开／回退和不可变历史；它只读取 active design Project、production_owner Assignment 与 Evidence File Version，不复制外部事实，不自动完成 Project、生成资金义务或演变为通用工作流。十个 v1 API 与 J3-A01 至 J3-A20 已定义，但运行时尚未实现。",
            "basis": "产品规格、技术方案、模型、API 契约、全局决策和验收入口共同固定了可实现且可证伪的边界。",
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md",
              "arckit/tech/design-operations/solution.md",
              "arckit/tech/INDEX.md",
              "arckit/_map/feature-matrix.md",
              "docs/decisions.md#BF-032",
              "docs/testing.md#M9-J3-设计生产与交付验收待执行"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-009",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 5
            },
            "effect": "upheld",
            "reason": "The reference OA capability set now has a durable J3 ownership and behavior decision while preserving the distinction between accepted target and implemented runtime.",
            "gap_ids": [],
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md",
              "arckit/tech/design-operations/solution.md",
              "docs/decisions.md#BF-032"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-010",
            "status": "open",
            "goal": "实现并验证 Design Operations 最小纵向闭环以及它实际需要的 Project／Evidence reference-read API，覆盖 J3-A01 至 J3-A20、独立备份恢复、Linux release 和非 root Docker 冒烟，同时保持 Nginx／Gateway／中心 IAM 与 D1 总装为后续独立验收。",
            "reason": "J3 的稳定业务和技术契约已经成立，但当前软件仍不能创建、推进、交付或追溯设计工作项，也无法用真实 Project Assignment 与 Evidence Version 完成组合验证。",
            "derived_from": [
              "FACT-20260923-002-008",
              "FACT-20260923-002-009"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the J3 journey and first end-to-end local OA business demo",
              "uncertainty": "Runtime integration must prove the stable boundary works with real reference services",
              "risk": "A mock-only implementation could hide ownership, outage or idempotency defects",
              "user_impact": "Produces the first runnable design-production capability for the demo",
              "evidence": "The accepted J3 specification and existing Go service patterns make implementation ready"
            },
            "responsibility": "agent",
            "evidence_required": [
              "Independent Design Operations Go module, PostgreSQL migration, IAM providers, ten v1 APIs and configurable Project/Evidence reference clients are implemented",
              "Project and Evidence expose only the minimum versioned reference-read facts required by J3 without leaking internal tables or storage credentials",
              "J3-A01 through J3-A20 have direct unit, HTTP, real PostgreSQL and system-composition evidence",
              "Backup/restore, Linux release checksum, non-root Docker smoke and existing application regressions pass",
              "Progress and testing documents record only checks actually run"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 4,
            "set_decision": {
              "status": "settled",
              "statement": "当前完整组合包含统一入口与工作台、IAM、People & Groups、Contract Ledger、Money Flow Ledger、Evidence Service、已实现 J1 的薄 Project Workspace，以及已固定稳定边界但尚待实现的独立 Design Operations。首个演示覆盖 J1-J4；J3 第一生产类型固定为 design-v1 四阶段，第二生产类型在首个演示通过后再选择。J1 已通过独立运行、真库、IAM、外部引用故障、Docker、备份恢复和 Linux release 验证但尚未进入 D1；J3 运行时、真实 Project／Evidence reference-read、J2／J4 组合和最终演示页面仍未完成。",
              "reason": "BF-031、BF-032、已完成 J1 和新接受的 J3 稳定规格共同固定当前能力实况与下一运行时缺口。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "docs/decisions.md#BF-032",
                "arckit/spec/project-workspace/project-initiation.md",
                "arckit/spec/design-operations/design-production-delivery.md",
                "arckit/tech/design-operations/solution.md",
                "docs/testing.md#M9-J3-设计生产与交付验收待执行"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The previous decision left the design extension undefined; the accepted J3 type, ownership, state and acceptance boundary now replace that ambiguity while preserving its unimplemented status.",
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md",
              "docs/decisions.md#BF-032"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "实现并验证 J3 Design Operations 独立运行时与最小 Project／Evidence reference-read 组合；继续优先本地业务架构验证，不以 Nginx／Gateway／中心 IAM、ICP备案或公司环境作为阻塞条件。",
          "project_priorities": [
            "实现并验证 J3 设计生产交付",
            "用现有 J2 台账能力和 J4 Evidence／Project 引用组成可操作本地 OA Demo",
            "本地业务 Demo 通过后在公司私有 Ubuntu 验证完整入口链",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "FACT-20260923-002-009",
          "arckit/spec/design-operations/design-production-delivery.md",
          "docs/development-plan.md",
          "docs/progress.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 18,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The J3 user result, type, owned facts, exclusions, states, correction behavior and unimplemented status are synchronized across stable product artifacts and global decisions.",
            "fact_refs": [
              "FACT-20260923-002-009"
            ],
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md#BF-032"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Create, owner assignment, deliverable replacement, stage completion, not-applicable, reopen, cancel, read and failure behavior are explicit in ten versioned contracts and the acceptance matrix.",
            "fact_refs": [
              "FACT-20260923-002-009"
            ],
            "evidence": [
              "arckit/tech/_shared/contracts/design-*.yaml",
              "arckit/spec/design-operations/design-production-delivery.md#J3-可执行验收矩阵"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This round fixes a business/API capability boundary and deliberately does not define the later operator-facing demo page or visual language.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Independent service ownership, fixed type definition, transactions, reference validation, IAM, portability and failure behavior are recorded in a stable technical solution, models and contracts.",
            "fact_refs": [
              "FACT-20260923-002-009"
            ],
            "evidence": [
              "arckit/tech/design-operations/solution.md",
              "arckit/tech/INDEX.md",
              "arckit/_map/RELATIONS.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "threatened",
            "reason": "The J3 target is now accepted and recoverable, but Design Operations and the real Project/Evidence reference APIs do not yet exist as running software.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008",
              "FACT-20260923-002-009"
            ],
            "evidence": [
              "docs/testing.md#M9-J3-设计生产与交付验收待执行",
              "arckit/_map/feature-matrix.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-010"
            ]
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "threatened",
            "reason": "The specification identifies authorization, ownership, outage, concurrency, idempotency, rollback and immutable-history risks, but runtime evidence for them cannot exist until the implementation gap is completed.",
            "fact_refs": [
              "FACT-20260923-002-009"
            ],
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md#J3-可执行验收矩阵",
              "arckit/tech/design-operations/solution.md"
            ],
            "gap_refs": [
              "GAP-20260923-002-010"
            ]
          }
        ]
      },
      "evidence": [
        "arckit/spec/design-operations/design-production-delivery.md",
        "arckit/tech/design-operations/solution.md",
        "arckit/tech/_shared/models/Design*.yaml",
        "arckit/tech/_shared/contracts/design-*.yaml",
        "arckit/spec/INDEX.md",
        "arckit/tech/INDEX.md",
        "arckit/_map/feature-matrix.md",
        "arckit/_map/RELATIONS.md",
        "docs/decisions.md#BF-032",
        "docs/development-plan.md",
        "docs/progress.md",
        "docs/testing.md#M9-J3-设计生产与交付验收待执行",
        "YAML syntax validation and git diff --check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T18:05:10.702Z"
    },
    {
      "round": 8,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Accept the implemented and verified J3 runtime as a real local business slice while preserving separate acceptance for the OA composition and complete entry chain.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "J3 运行时是当前 Case 内唯一 ready 候选；完成它可以在不依赖公网入口或公司环境的前提下验证第一种真实生产类型。",
        "snapshot_token": "c241319c11e1780347642a9203ee2bdbaf834c55f6b356aecd16f731f91a45c4",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-010",
        "comparison_summary": "本 Case 范围内只有 GAP-20260923-002-010 一个 ready 持久候选；备案与公司环境属于其他 Case，不阻塞本地业务实现。",
        "fresh_discovery_summary": "J3 通过后，参考 OA 仍缺同一组演示数据上的 J2 最小引用、J4 项目追溯和可重复操作者旅程；该组合结果作为下一独立 Gap，而不提前切换到 Nginx／Gateway／中心 IAM。",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-010",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks J3 and the first end-to-end local OA business demo",
              "uncertainty": "Real reference services must prove the accepted boundary",
              "risk": "Mock-only success could hide authorization, ownership and outage defects",
              "user_impact": "Makes design production runnable before company deployment"
            },
            "reason": "The stable J3 specification, existing Project and Evidence services, and established Go delivery patterns make this the smallest executable business gap."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-010",
        "responsibility": "agent",
        "goal": "实现并验证 Design Operations 最小纵向闭环以及它实际需要的 Project／Evidence reference-read API，覆盖 J3-A01 至 J3-A20、独立备份恢复、Linux release 和非 root Docker 冒烟，同时保持 Nginx／Gateway／中心 IAM 与 D1 总装为后续独立验收。",
        "reason": "J3 的稳定业务和技术契约已经成立，但当前软件仍不能创建、推进、交付或追溯设计工作项，也无法用真实 Project Assignment 与 Evidence Version 完成组合验证。",
        "derived_from": [
          "FACT-20260923-002-008",
          "FACT-20260923-002-009"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks the J3 journey and first end-to-end local OA business demo",
          "uncertainty": "Runtime integration must prove the stable boundary works with real reference services",
          "risk": "A mock-only implementation could hide ownership, outage or idempotency defects",
          "user_impact": "Produces the first runnable design-production capability for the demo",
          "evidence": "The accepted J3 specification and existing Go service patterns make implementation ready"
        },
        "evidence_required": [
          "Independent Design Operations Go module, PostgreSQL migration, IAM providers, ten v1 APIs and configurable Project/Evidence reference clients are implemented",
          "Project and Evidence expose only the minimum versioned reference-read facts required by J3 without leaking internal tables or storage credentials",
          "J3-A01 through J3-A20 have direct unit, HTTP, real PostgreSQL and system-composition evidence",
          "Backup/restore, Linux release checksum, non-root Docker smoke and existing application regressions pass",
          "Progress and testing documents record only checks actually run"
        ]
      },
      "planned_transition": {
        "goal": "Accept the implemented and verified J3 runtime as a real local business slice while preserving separate acceptance for the OA composition and complete entry chain.",
        "expected_state_change": "The J3 runtime gap becomes resolved, its previous specification-only fact is superseded, the product capability decision reflects seven running data owners, and a J2/J4 local composition gap becomes the next obligation."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-010",
          "status": "resolved",
          "outcome": "Design Operations now runs as an independent Go and PostgreSQL application with real Project, Assignment and Evidence reference reads, complete J3 behavior and repeatable delivery evidence.",
          "reason": "The runtime, cross-application read boundaries, A01-A20 tests, regression checks, backup restore, Linux release and non-root container evidence satisfy every declared requirement without pulling the entry stack into the business slice.",
          "evidence": [
            "backend/services/design-operations",
            "backend/services/project-workspace/internal/projects/service.go",
            "backend/services/evidence-service/internal/httpserver/server.go",
            "arckit/spec/design-operations/design-production-delivery.md#实现与验证证据",
            "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过",
            "docs/progress.md#M9-J3-Design-Operations-独立业务切片"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-010",
            "revision": 1,
            "status": "accepted",
            "statement": "J3 Design Operations 已实现为第七个独立运行数据所有者：design-v1 Work Item、固定四阶段、负责人、Evidence 成果、适用性、替换、显式回退、取消和不可变历史由独立 PostgreSQL 拥有；Project／Assignment／Evidence 只通过版本化 reference-read 校验。J3-A01 至 J3-A20、正式／模板 IAM、故障关闭、并发、备份恢复、Linux AMD64 release 和非 root Docker 已通过，但 J2／J4 操作者组合与完整入口仍未完成。",
            "basis": "独立运行时代码、跨应用契约、真实 PostgreSQL 和系统测试、全仓回归、恢复、发布物与容器运行证据相互一致。",
            "evidence": [
              "backend/services/design-operations",
              "arckit/spec/design-operations/design-production-delivery.md",
              "arckit/tech/design-operations/solution.md",
              "arckit/tech/_shared/contracts/project-reference-get.yaml",
              "arckit/tech/_shared/contracts/project-assignment-reference-get.yaml",
              "arckit/tech/_shared/contracts/evidence-file-version-reference-get.yaml",
              "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过",
              "docs/decisions.md#BF-032"
            ]
          }
        ],
        "facts_superseded": [
          {
            "id": "FACT-20260923-002-009",
            "revision": 1,
            "reason": "J3 的稳定规格事实仍由新事实保留，但其中‘运行时尚未实现’的阶段状态已被真实实现与验证结果取代。",
            "evidence": [
              "FACT-20260923-002-010",
              "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过"
            ]
          }
        ],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-010",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 6
            },
            "effect": "upheld",
            "reason": "The accepted reference OA capability set now has directly realized J1 and J3 business slices while preserving J2/J4 and complete-entry acceptance as distinct obligations.",
            "gap_ids": [],
            "evidence": [
              "backend/services/design-operations",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md#BF-032",
              "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过"
            ]
          }
        ],
        "gaps_added": [
          {
            "id": "GAP-20260923-002-011",
            "status": "open",
            "goal": "复用已通过的 J1／J3 与既有 Contract／Money／Evidence，补齐 J2 最小稳定引用和 J4 项目追溯，在同一组非真实数据上形成可重复的本地参考 OA 操作者 Demo；不引入自动 Contract→Money 连接器，也不以 Nginx／Gateway／中心 IAM／D1 为本轮前提。",
            "reason": "独立业务切片已经证明项目和设计边界，但尚未证明小系统群能连续完成项目、合同与回款、设计交付和资料追溯，也没有供演示者重复执行的最小交互入口。",
            "derived_from": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008",
              "FACT-20260923-002-010"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the phase-one reference OA Demo and the main business-architecture hypothesis",
              "uncertainty": "J2/J4 composition may expose missing reference contracts or operator-flow gaps",
              "risk": "Adding a broad portal or automatic connector could violate ownership and invent business decisions",
              "user_impact": "Produces the first recognizable, repeatable OA demonstration before company deployment",
              "evidence": "J1 and J3 now run independently and existing Contract, Money and Evidence capabilities are available for composition"
            },
            "responsibility": "agent",
            "evidence_required": [
              "One documented non-real demo dataset completes J1, explicit J2 obligation confirmation, receipt/allocation, J3 and J4 traceability through versioned APIs",
              "Minimum Contract, Money, Design and Evidence reference reads expose only stable IDs, revisions, statuses and failure states",
              "An operator can repeat the journey locally with template IAM credentials and without editing databases or bypassing authorization",
              "Stopping one referenced service yields explicit partial-unavailable behavior without false success or cross-database writes",
              "Automated composition tests and a concise demo runbook pass while Nginx/Gateway/central IAM remain a separate later acceptance"
            ],
            "resolution": null
          }
        ],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 5,
            "set_decision": {
              "status": "settled",
              "statement": "当前已有七个运行数据所有者以及 Gateway／Workbench 两个无业务数据库装配应用。参考 OA 首个演示覆盖 J1-J4：J1 Project Workspace 与 J3 Design Operations 已通过独立运行、真实 PostgreSQL、IAM、引用故障、Docker、备份恢复和 Linux release 验证但尚未进入 D1；J2 复用既有 Contract／Money 并坚持人工确认正式义务，J4 通过稳定 reference-read 组合追溯。当前下一项是 J2／J4 本地操作者 Demo；Nginx／Gateway／中心 IAM 和公司 Ubuntu 总装在业务 Demo 稳定后用同一发布物单独验收。",
              "reason": "BF-031、BF-032、J1 和 J3 的直接运行证据共同固定当前能力实况及业务优先顺序。",
              "evidence": [
                "docs/decisions.md#BF-031",
                "docs/decisions.md#BF-032",
                "arckit/spec/assembly/reference-oa-demo.md",
                "arckit/spec/project-workspace/project-initiation.md",
                "arckit/spec/design-operations/design-production-delivery.md",
                "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The previous decision described Design as unimplemented; direct runtime and delivery evidence now requires an accurate capability inventory and moves focus to composition rather than infrastructure.",
            "evidence": [
              "FACT-20260923-002-010",
              "docs/progress.md",
              "arckit/_map/feature-matrix.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "以已通过的 J1／J3 和既有 Contract／Money／Evidence 形成 J2／J4 本地参考 OA 操作者 Demo；继续先验证业务架构，不以 Nginx／Gateway／中心 IAM、ICP备案或公司环境为阻塞条件。",
          "project_priorities": [
            "完成 J2／J4 本地参考 OA 操作者 Demo",
            "用同一业务发布物在公司私有 Ubuntu 验证完整入口链",
            "ICP备案完成后复验个人试验田公网入口",
            "首个 Demo 后再选择第二生产类型验证扩展平衡"
          ]
        },
        "evidence": [
          "FACT-20260923-002-010",
          "arckit/spec/assembly/reference-oa-demo.md",
          "docs/development-plan.md",
          "docs/progress.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 19,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The realized J3 outcome, exclusions and remaining OA-composition boundary are synchronized across product specification, decisions and progress evidence.",
            "fact_refs": [
              "FACT-20260923-002-010"
            ],
            "evidence": [
              "arckit/spec/design-operations/design-production-delivery.md",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md#BF-032"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "All J3 actions, responses, authorization failures, stage transitions and recovery behavior are represented by versioned contracts and executable HTTP tests.",
            "fact_refs": [
              "FACT-20260923-002-010"
            ],
            "evidence": [
              "arckit/tech/_shared/contracts/design-*.yaml",
              "backend/services/design-operations/internal/httpserver/system_integration_test.go",
              "backend/services/design-operations/internal/httpserver/server_test.go"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This transition realizes the independent API and delivery slice; the operator-facing demo and its visual expectations remain in the next gap.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Independent ownership, migration, transaction, IAM, reference-read, failure, backup, container and release boundaries are recoverable from stable technical artifacts and code.",
            "fact_refs": [
              "FACT-20260923-002-010"
            ],
            "evidence": [
              "arckit/tech/design-operations/solution.md",
              "arckit/tech/INDEX.md",
              "arckit/_map/RELATIONS.md",
              "backend/services/design-operations/README.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The accepted design-v1 ownership, behavior and external-reference constraints are implemented and exercised against real PostgreSQL with Project and Evidence boundaries.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008",
              "FACT-20260923-002-010"
            ],
            "evidence": [
              "backend/services/design-operations",
              "backend/services/project-workspace/internal/projects/service.go",
              "backend/services/evidence-service/internal/httpserver/reference_test.go",
              "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Authorization, scope, ownership, reference outage, stale revision, concurrency, idempotency, rollback, immutable history, restore, platform and non-root runtime risks have repeatable direct evidence.",
            "fact_refs": [
              "FACT-20260923-002-010"
            ],
            "evidence": [
              "backend/services/design-operations/internal/auth/auth_test.go",
              "backend/services/design-operations/internal/design/postgres_repository_integration_test.go",
              "backend/services/design-operations/internal/httpserver/system_integration_test.go",
              "backend/services/design-operations/ops/verify-backup-restore",
              "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "backend/services/design-operations",
        "backend/services/project-workspace/internal/projects/service.go",
        "backend/services/evidence-service/internal/httpserver/reference_test.go",
        "arckit/spec/design-operations/design-production-delivery.md",
        "arckit/tech/design-operations/solution.md",
        "arckit/tech/_shared/contracts/project-reference-get.yaml",
        "arckit/tech/_shared/contracts/project-assignment-reference-get.yaml",
        "arckit/tech/_shared/contracts/evidence-file-version-reference-get.yaml",
        "docs/decisions.md#BF-032",
        "docs/progress.md#M9-J3-Design-Operations-独立业务切片",
        "docs/testing.md#M9-J3-设计生产与交付验收2026-09-25-已通过",
        "Go ordinary, PostgreSQL integration, race, backup/restore, Linux release, non-root Docker readiness, YAML and shell validation on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T18:40:39.882Z"
    },
    {
      "round": 9,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Accept the repeatable local J1-J4 composition as the first reference OA business-architecture Demo.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "GAP-20260923-002-011 是当前 Case 唯一 ready 候选；六应用组合、同数据 J1-J4、重复执行和故障隔离现已提供全部直接证据。",
        "snapshot_token": "f47f6a09a98c353a7ff636acb9903515f8634144cea3b0b8ebe1be798204447e",
        "selected_ref": "case-gap:CASE-20260923-002:GAP-20260923-002-011",
        "comparison_summary": "本 Case 只有这一个 ready 候选；公司环境与 ICP 事项由其他 Case 承接。",
        "fresh_discovery_summary": "组合发现并修复 Design reference 缺统一 id；修复后无须扩大业务范围，完整入口仍由既有交付 Case 承接。",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:GAP-20260923-002-011",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks the phase-one reference OA Demo",
              "uncertainty": "Composition could expose contract gaps",
              "risk": "A broad portal or connector could violate ownership",
              "user_impact": "Produces the first repeatable OA demonstration"
            },
            "reason": "The implementation now exercises every requirement without the deferred entry stack."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260923-002-011",
        "responsibility": "agent",
        "goal": "复用已通过的 J1／J3 与既有 Contract／Money／Evidence，补齐 J2 最小稳定引用和 J4 项目追溯，在同一组非真实数据上形成可重复的本地参考 OA 操作者 Demo；不引入自动 Contract→Money 连接器，也不以 Nginx／Gateway／中心 IAM／D1 为本轮前提。",
        "reason": "独立业务切片已经证明项目和设计边界，但尚未证明小系统群能连续完成项目、合同与回款、设计交付和资料追溯，也没有供演示者重复执行的最小交互入口。",
        "derived_from": [
          "FACT-20260923-002-006",
          "FACT-20260923-002-008",
          "FACT-20260923-002-010"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks the phase-one reference OA Demo and the main business-architecture hypothesis",
          "uncertainty": "J2/J4 composition may expose missing reference contracts or operator-flow gaps",
          "risk": "Adding a broad portal or automatic connector could violate ownership and invent business decisions",
          "user_impact": "Produces the first recognizable, repeatable OA demonstration before company deployment",
          "evidence": "J1 and J3 now run independently and existing Contract, Money and Evidence capabilities are available for composition"
        },
        "evidence_required": [
          "One documented non-real demo dataset completes J1, explicit J2 obligation confirmation, receipt/allocation, J3 and J4 traceability through versioned APIs",
          "Minimum Contract, Money, Design and Evidence reference reads expose only stable IDs, revisions, statuses and failure states",
          "An operator can repeat the journey locally with template IAM credentials and without editing databases or bypassing authorization",
          "Stopping one referenced service yields explicit partial-unavailable behavior without false success or cross-database writes",
          "Automated composition tests and a concise demo runbook pass while Nginx/Gateway/central IAM remain a separate later acceptance"
        ]
      },
      "planned_transition": {
        "goal": "Accept the repeatable local J1-J4 composition as the first reference OA business-architecture Demo.",
        "expected_state_change": "Resolve the composition gap, record the realized capability and move focus to preserving it before authorized full-entry validation."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260923-002-011",
          "status": "resolved",
          "outcome": "Six authenticated applications and six isolated databases complete the same fake-data J1-J4 journey, replay idempotently and expose Contract partial unavailability.",
          "reason": "Public-interface composition, outage recovery, tests and runbook satisfy all evidence without a connector or database edits.",
          "evidence": [
            "deployments/local-oa",
            "acceptance/reference-oa-demo",
            "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260923-002-011",
            "revision": 1,
            "status": "accepted",
            "statement": "首个参考 OA 本地业务架构 Demo 已实现：Contract、Money、People、Evidence、Project、Design 以六个应用和六个独立 PostgreSQL 数据库运行，同一组非真实数据经公开接口连续完成 J1～J4；重复执行保持相同业务身份，Contract 停机时 Project 保持 active 且只有合同引用 unavailable。组合未关闭鉴权、未跨库写入、未创建自动 Contract→Money 连接器；完整入口和公司 Ubuntu 总装仍需另行验收。",
            "basis": "两次 Demo、Contract 停机隔离、恢复后健康、全工作区测试、Compose 和规格证据一致；发现的 Design reference id 缺陷已兼容修复。",
            "evidence": [
              "deployments/local-oa",
              "acceptance/reference-oa-demo",
              "arckit/spec/assembly/reference-oa-demo.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/progress.md",
              "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
            ]
          }
        ],
        "facts_superseded": [
          {
            "id": "FACT-20260923-002-010",
            "revision": 1,
            "reason": "J3 事实保留，但其中 J2/J4 尚未完成的阶段状态已被组合验收取代。",
            "evidence": [
              "FACT-20260923-002-011",
              "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
            ]
          }
        ],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260923-002-004",
            "fact_id": "FACT-20260923-002-011",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 7
            },
            "effect": "upheld",
            "reason": "J1-J4 is now realized locally while full-entry deployment remains distinct.",
            "gap_ids": [],
            "evidence": [
              "deployments/local-oa",
              "acceptance/reference-oa-demo",
              "arckit/spec/assembly/reference-oa-demo.md"
            ]
          },
          {
            "id": "IMPACT-20260923-002-005",
            "fact_id": "FACT-20260923-002-011",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "experience_and_interaction",
              "revision": 3
            },
            "effect": "upheld",
            "reason": "The direct-entry journey and partial-unavailable behavior are executable without weakening unified-entry requirements.",
            "gap_ids": [],
            "evidence": [
              "deployments/local-oa/README.md",
              "acceptance/reference-oa-demo"
            ]
          }
        ],
        "gaps_added": [],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": null,
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 6,
            "set_decision": {
              "status": "settled",
              "statement": "当前已有七个运行数据所有者以及 Gateway／Workbench 两个无业务数据库装配应用。参考 OA 首个 J1～J4 本地业务架构 Demo 已由 Contract、Money、People、Evidence、Project、Design 六应用／六独立数据库用同一组非真实数据通过；人工确认义务、幂等重放和 Contract 停机部分不可用均有直接证据。Project／Design 尚未进入完整 D1；完整入口和公司 Ubuntu 总装另行验收。",
              "reason": "Direct composition evidence replaces the former next-gap status without conflating it with entry deployment.",
              "evidence": [
                "FACT-20260923-002-011",
                "arckit/spec/assembly/reference-oa-demo.md",
                "docs/progress.md",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The local reference OA composition is realized.",
            "evidence": [
              "FACT-20260923-002-011",
              "deployments/local-oa"
            ]
          },
          {
            "area_ref": "experience_and_interaction",
            "observed_revision": 2,
            "set_decision": {
              "status": "settled",
              "statement": "完整组合提供统一登录和工作台入口；阶段一本地业务验证已通过独立入口、Contract／Money 本地账号和其余应用模板 IAM 完成 J1～J4，没有关闭鉴权。操作者明确建立义务、收款和核销；目标应用停机时 Project 显示单项 unavailable。随后同一发布物再验证完整入口链。",
              "reason": "Local journey and recovery evidence now realize the staged interaction strategy.",
              "evidence": [
                "FACT-20260923-002-011",
                "deployments/local-oa/README.md",
                "arckit/spec/assembly/reference-oa-demo.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The prospective local journey is now realized.",
            "evidence": [
              "FACT-20260923-002-011",
              "acceptance/reference-oa-demo"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "保持已通过的本地 J1～J4 参考 OA 组合作为回归基线；获得公司私有 Ubuntu 授权和环境输入后，用同一发布物验证完整入口、TLS 与恢复，不提前扩张业务范围。",
          "project_priorities": [
            "持续回归本地 J1～J4 业务组合",
            "获授权后在公司 Ubuntu 验证完整入口与恢复",
            "备案完成后复验个人试验田公网入口",
            "有新业务证据后再选择第二生产类型"
          ]
        },
        "evidence": [
          "FACT-20260923-002-011",
          "arckit/spec/assembly/reference-oa-demo.md",
          "arckit/tech/assembly/local-reference-oa-composition.md",
          "docs/development-plan.md",
          "docs/progress.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 20,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Realized outcome and exclusions are synchronized across stable product documents.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/specs/assembly/oa-reference-stage-1.md",
              "docs/decisions.md#BF-031"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Executable sequence, authentication, replay and recovery are documented.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "deployments/local-oa/README.md",
              "acceptance/reference-oa-demo"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This validates service composition and does not alter visual expectations.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Ownership, databases, references, authentication, failures and migration boundaries are explicit.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "deployments/local-oa/compose.yaml"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "Accepted J1-J4 rules run against real processes and databases without bypass.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008",
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "acceptance/reference-oa-demo",
              "deployments/local-oa",
              "docs/testing.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Idempotency, authorization, references, isolated persistence, outage and restoration have direct evidence.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "make test and make vet on 2026-09-25: passed",
              "demo replay and failure isolation on 2026-09-25: passed",
              "Compose and YAML validation on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "deployments/local-oa",
        "acceptance/reference-oa-demo",
        "arckit/spec/assembly/reference-oa-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/testing.md",
        "All declared checks on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T19:32:17.724Z"
    },
    {
      "round": 10,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Independently close out the local reference-OA stage after reviewing all five completion dimensions.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "All ordinary gaps are closed, so the deterministic completion-review candidate is the only ready item for this Case.",
        "snapshot_token": "9a186f30a2b23d8dffe0af2970915149144017157943bb32656a1b8129bacfb5",
        "selected_ref": "case-gap:CASE-20260923-002:CASE-20260923-002:completion-review:1",
        "comparison_summary": "This Case has exactly one ready candidate and no competing persisted gap.",
        "fresh_discovery_summary": "Review of the composition, driver, failure test, documentation and executed evidence found no new error, omission or excess within the agreed local business-validation scope.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260923-002:CASE-20260923-002:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "high",
              "uncertainty": "low",
              "risk": "high",
              "user_impact": "high"
            },
            "reason": "A clean independent closeout review is required before the implemented stage can be accepted as complete."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260923-002:completion-review:1",
        "responsibility": "agent",
        "goal": "Review the completed implementation for correctness, real problem resolution, verification credibility, regression risk, and minimality.",
        "reason": "All ordinary Case gaps and state impacts are closed.",
        "derived_from": [
          "case_result",
          "content_revision:9"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "high",
          "uncertainty": "low",
          "risk": "high",
          "user_impact": "high"
        },
        "evidence_required": [
          "review evidence for all five completion dimensions"
        ]
      },
      "planned_transition": {
        "goal": "Independently close out the local reference-OA stage after reviewing all five completion dimensions.",
        "expected_state_change": "Mark the review clean and close the Case without changing product or technical state."
      },
      "accepted_state_delta": {
        "resolved_gap": null,
        "facts_added": [],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [],
        "gaps_added": [],
        "gaps_cancelled": [],
        "resolved_open_questions": [],
        "completed_handoffs": [],
        "completion_review_result": {
          "outcome": "clean",
          "reviewer": "agent",
          "reviewed_content_revision": 9,
          "dimensions": {
            "implementation_correctness": "clean",
            "problem_resolution": "clean",
            "verification_credibility": "clean",
            "regression_risk": "clean",
            "minimality": "clean"
          },
          "findings": [],
          "evidence": [
            "deployments/local-oa/compose.yaml: six independently persisted authenticated applications bound to localhost",
            "acceptance/reference-oa-demo: public-interface J1-J4 driver with fixed command IDs and owner-only resume state",
            "two consecutive local OA demo executions retained the same business IDs and ended at Project revision 9",
            "Contract Ledger outage isolation passed with Project active, four references available and one unavailable, followed by successful recovery",
            "make test, make vet, make fmt-check, Compose config validation, YAML parsing and git diff --check passed on 2026-09-25",
            "Design stable-reference compatibility regression is covered by Design and Project tests",
            "arckit/spec/assembly/reference-oa-demo.md and arckit/tech/assembly/local-reference-oa-composition.md preserve the deferred Nginx/Gateway/central-IAM boundary"
          ]
        },
        "resolved_review_findings": [],
        "review_budget_extension": null
      },
      "project_state_delta": {
        "software_definition_changes": [],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": null,
        "evidence": []
      },
      "invariant_assessment": {
        "project_revision": 21,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The realized J1-J4 capability and the explicitly deferred full-entry scope remain recoverable from stable specifications.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/specs/assembly/oa-reference-stage-1.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The operator sequence, authentication modes, rerun behavior and partial-unavailable state are documented and executable.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "deployments/local-oa/README.md",
              "acceptance/reference-oa-demo"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The closeout review makes no visual-language change.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Database ownership, application boundaries, authentication, reference contracts, failure behavior and migration boundary remain explicit.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "deployments/local-oa/compose.yaml"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "Accepted J1-J4 behavior is exercised against real application processes and isolated PostgreSQL databases through public interfaces.",
            "fact_refs": [
              "FACT-20260923-002-006",
              "FACT-20260923-002-008",
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "acceptance/reference-oa-demo",
              "docs/testing.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Authorization, idempotency, isolated persistence, compatibility, service outage and recovery all have executed evidence.",
            "fact_refs": [
              "FACT-20260923-002-011"
            ],
            "evidence": [
              "make test and make vet on 2026-09-25: passed",
              "demo replay and Contract outage isolation on 2026-09-25: passed",
              "Compose config, YAML and diff validation on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "deployments/local-oa",
        "acceptance/reference-oa-demo",
        "arckit/spec/assembly/reference-oa-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/testing.md",
        "Independent five-dimension completion review on 2026-09-25: clean"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T19:34:57.974Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260923-002-001",
      "GAP-20260923-002-002",
      "GAP-20260923-002-003",
      "GAP-20260923-002-004",
      "GAP-20260923-002-005",
      "GAP-20260923-002-006",
      "GAP-20260923-002-007",
      "GAP-20260923-002-008",
      "GAP-20260923-002-009",
      "GAP-20260923-002-010",
      "GAP-20260923-002-011"
    ],
    "remaining": [],
    "blocked": [],
    "reason": "All dynamic gaps and state impacts are closed and the current implementation passed completion review.",
    "candidate_gaps": [],
    "loop_handoff": {
      "version": "loop-handoff/v2",
      "status": "done",
      "next_responsibility": "none",
      "agent_continuation_available": false,
      "human_decision_required": false,
      "trigger_mode": "none",
      "responsibility_reason": "The current Case revision passed completion review.",
      "next_prompt": "",
      "human_gate": {
        "required": false,
        "reason": "",
        "decision_needed": ""
      }
    },
    "updated_at": "2026-09-24T19:34:57.974Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
