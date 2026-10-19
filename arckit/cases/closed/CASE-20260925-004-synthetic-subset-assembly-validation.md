# Synthetic subset assembly validation

Case: CASE-20260925-004
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T03:53:15.425Z

## User Intent

Prove that a selected subset of the real business applications can form a coherent authenticated business composition without starting unrelated Contract or Money services, while preserving the full reference OA and without claiming second-customer evidence.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-004",
  "title": "Synthetic subset assembly validation",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T03:45:45.393Z",
  "updated_at": "2026-09-25T03:53:15.425Z",
  "user_intent": "Prove that a selected subset of the real business applications can form a coherent authenticated business composition without starting unrelated Contract or Money services, while preserving the full reference OA and without claiming second-customer evidence.",
  "expected_outcome": "A repeatable synthetic design-delivery assembly starts only People, Evidence, Project and Design with their owned databases, completes and replays a coherent J1/J3 subset journey, verifies Contract and Money remain absent, and documents the precise evidence boundary.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-004-001",
      "revision": 1,
      "status": "accepted",
      "statement": "The current worktree proves all six applications independently and as one full reference OA, but the local OA Compose topology requires Project Workspace to start Contract Ledger and Money Flow Ledger, so it does not yet directly prove a coherent selected-app composition with those unrelated systems absent.",
      "basis": "Fresh inspection of deployments/local-oa/compose.yaml and the current Level A and Level B evidence shows independent runtime and all-six composition evidence but no runnable subset composition proof for Project and Design.",
      "evidence": [
        "deployments/local-oa/compose.yaml",
        "deployments/local-oa/ops/local-oa",
        "deployments/demos/.runtime/level-a-report.tsv",
        "docs/testing.md#目标级当前状态审计2026-09-25"
      ]
    },
    {
      "id": "FACT-20260925-004-002",
      "revision": 1,
      "status": "accepted",
      "statement": "A fresh isolated synthetic design-delivery assembly using People, Evidence, Project and Design completed twice with stable IDs, revisions and state digest while Contract and Money services and databases were absent; cleanup removed the isolated runtime and the full reference OA remained healthy.",
      "basis": "Direct execution of the current source-worktree composition gate, post-cleanup Docker inspection and full-suite J1-J4 regression provide runtime and regression evidence.",
      "evidence": [
        "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
        "deployments/local-oa/ops/local-oa demo: full J1-J4 PASS after subset cleanup",
        "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-004-001",
      "fact_id": "FACT-20260925-004-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 23
      },
      "effect": "upheld",
      "reason": "Direct middle-granularity runtime evidence now supports the accepted customer-assembly capability without extending it to a second real customer.",
      "gap_ids": [],
      "evidence": [
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-004-001",
      "status": "resolved",
      "goal": "Implement and verify one synthetic design-delivery assembly using only People, Evidence, Project and Design plus their owned databases, with template IAM, deterministic replay, explicit absence of Contract and Money, isolated runtime state and cleanup.",
      "reason": "A middle-granularity composition is required to distinguish real modular assembly from merely having independently runnable applications and one fixed full-suite composition.",
      "derived_from": [
        "FACT-20260925-004-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Directly tests the current product goal of selectable small-system composition",
        "risk": "Without it, hidden startup coupling can survive both Level A and full-suite validation",
        "user_impact": "Provides a concrete demo shape for customers who need project/design collaboration without contract or finance modules",
        "evidence": "All required images, APIs and local Docker infrastructure are available now"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Only the selected four applications and their four databases are running in an isolated Compose project",
        "A coherent authenticated project/design/evidence/person journey succeeds from fresh state",
        "The same journey replays idempotently with stable identities and revisions",
        "Contract and Money services are proven absent rather than merely unused",
        "Cleanup removes only the isolated synthetic composition state",
        "Durable specification, technical and testing documentation states that this is synthetic assembly evidence, not second-customer proof"
      ],
      "resolution": {
        "id": "GAP-20260925-004-001",
        "status": "resolved",
        "outcome": "verified_isolated_subset_assembly",
        "reason": "The isolated gate started exactly four selected applications and four owned databases, proved Contract and Money absent, completed and replayed the authenticated J1/J3/J4 subset with stable identities and state digest, cleaned its state, and left the full OA healthy.",
        "evidence": [
          "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
          "docs/testing.md#设计交付子集装配验收2026-09-25-已通过",
          "acceptance/reference-oa-demo/main.go",
          "acceptance/reference-oa-demo/traceability.go"
        ],
        "occurred_at": "2026-09-25T03:51:28.426Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 2,
      "source": "Default autonomous review policy for a bounded synthetic composition proof.",
      "snapshotted_at": "2026-09-25T03:45:45.393Z"
    },
    "additional_cycles_authorized": 0,
    "cycle_count": 1,
    "reviewed_content_revision": 1,
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
        "content_revision": 1,
        "dimensions": {
          "implementation_correctness": "clean",
          "problem_resolution": "clean",
          "verification_credibility": "clean",
          "regression_risk": "clean",
          "minimality": "clean"
        },
        "finding_ids": [],
        "evidence": [
          "deployments/local-oa/ops/local-oa composition-audit rerun: PASS on 2026-09-25 with stable replay",
          "post-audit Docker inspection: zero isolated containers, volumes, networks and state file",
          "deployments/local-oa/ops/local-oa verify: all six full-suite applications ready",
          "deployments/local-oa/ops/local-oa demo: full J1-J4 regression passed",
          "make local-oa-release-bundle-test: passed",
          "acceptance/reference-oa-demo: go test and go vet passed",
          "sh -n, Compose config and git diff --check: passed",
          "development-case and project-state validation: passed",
          "protocol compatibility: compatible"
        ],
        "occurred_at": "2026-09-25T03:53:15.425Z"
      }
    ],
    "evidence": [
      "deployments/local-oa/ops/local-oa composition-audit rerun: PASS on 2026-09-25 with stable replay",
      "post-audit Docker inspection: zero isolated containers, volumes, networks and state file",
      "deployments/local-oa/ops/local-oa verify: all six full-suite applications ready",
      "deployments/local-oa/ops/local-oa demo: full J1-J4 regression passed",
      "make local-oa-release-bundle-test: passed",
      "acceptance/reference-oa-demo: go test and go vet passed",
      "sh -n, Compose config and git diff --check: passed",
      "development-case and project-state validation: passed",
      "protocol compatibility: compatible"
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
      "goal": "Accept the isolated synthetic design-delivery composition as direct middle-granularity assembly evidence while preserving all external proof limits.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The subset-composition gap is the only ready Agent-owned candidate and directly tests the modular assembly claim; the other two persisted candidates remain Human-owned external prerequisites.",
        "snapshot_token": "fac6e4f0b7e6ff7d1bb47a5ef134409aa43d985c7b4a7c8f3692a8af80dbacbe",
        "selected_ref": "case-gap:CASE-20260925-004:GAP-20260925-004-001",
        "comparison_summary": "Compared the local subset proof with personal public-ingress compliance and company-environment authorization. The local proof is executable now and reduces product-boundary uncertainty without assuming external authority.",
        "fresh_discovery_summary": "Implementation and live execution exposed no additional Case-scoped gap; the remaining second-customer and company-environment claims retain their existing external evidence boundaries.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-004:GAP-20260925-004-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Tests selectable application assembly with available local infrastructure"
            },
            "reason": "The source, images, APIs and isolated Docker runtime are available and the result directly addresses the current modularity uncertainty."
          },
          {
            "ref": "case-gap:CASE-20260923-001:GAP-20260923-001-009",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks personal mainland public acceptance",
              "responsibility": "human"
            },
            "reason": "ICP filing or alternate ingress remains controlled by the owner and is independent of the local subset proof."
          },
          {
            "ref": "case-gap:CASE-20260923-003:GAP-20260923-003-002",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks company Ubuntu deployment",
              "responsibility": "human"
            },
            "reason": "Company authorization and target resource details have not yet been supplied."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-004-001",
        "responsibility": "agent",
        "goal": "Implement and verify one synthetic design-delivery assembly using only People, Evidence, Project and Design plus their owned databases, with template IAM, deterministic replay, explicit absence of Contract and Money, isolated runtime state and cleanup.",
        "reason": "A middle-granularity composition is required to distinguish real modular assembly from merely having independently runnable applications and one fixed full-suite composition.",
        "derived_from": [
          "FACT-20260925-004-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Directly tests the current product goal of selectable small-system composition",
          "uncertainty": "",
          "risk": "Without it, hidden startup coupling can survive both Level A and full-suite validation",
          "user_impact": "Provides a concrete demo shape for customers who need project/design collaboration without contract or finance modules",
          "evidence": "All required images, APIs and local Docker infrastructure are available now"
        },
        "evidence_required": [
          "Only the selected four applications and their four databases are running in an isolated Compose project",
          "A coherent authenticated project/design/evidence/person journey succeeds from fresh state",
          "The same journey replays idempotently with stable identities and revisions",
          "Contract and Money services are proven absent rather than merely unused",
          "Cleanup removes only the isolated synthetic composition state",
          "Durable specification, technical and testing documentation states that this is synthetic assembly evidence, not second-customer proof"
        ]
      },
      "planned_transition": {
        "goal": "Accept the isolated synthetic design-delivery composition as direct middle-granularity assembly evidence while preserving all external proof limits.",
        "expected_state_change": "Resolve the subset-composition gap, record current runtime evidence, update the product capability decision and move the Case to completion review."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-004-001",
          "status": "resolved",
          "outcome": "verified_isolated_subset_assembly",
          "reason": "The isolated gate started exactly four selected applications and four owned databases, proved Contract and Money absent, completed and replayed the authenticated J1/J3/J4 subset with stable identities and state digest, cleaned its state, and left the full OA healthy.",
          "evidence": [
            "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
            "docs/testing.md#设计交付子集装配验收2026-09-25-已通过",
            "acceptance/reference-oa-demo/main.go",
            "acceptance/reference-oa-demo/traceability.go"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-004-002",
            "revision": 1,
            "status": "accepted",
            "statement": "A fresh isolated synthetic design-delivery assembly using People, Evidence, Project and Design completed twice with stable IDs, revisions and state digest while Contract and Money services and databases were absent; cleanup removed the isolated runtime and the full reference OA remained healthy.",
            "basis": "Direct execution of the current source-worktree composition gate, post-cleanup Docker inspection and full-suite J1-J4 regression provide runtime and regression evidence.",
            "evidence": [
              "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
              "deployments/local-oa/ops/local-oa demo: full J1-J4 PASS after subset cleanup",
              "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260925-004-001",
            "fact_id": "FACT-20260925-004-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 23
            },
            "effect": "upheld",
            "reason": "Direct middle-granularity runtime evidence now supports the accepted customer-assembly capability without extending it to a second real customer.",
            "gap_ids": [],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
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
            "observed_revision": 22,
            "set_decision": {
              "status": "settled",
              "statement": "当前七个运行数据所有者以及 Gateway／Workbench 装配边界保持不变；Contract、Money、People、Evidence、Project、Design 六个业务应用的 Level A 独立 Demo、Level B 高保真参考 OA、People＋Evidence＋Project＋Design 的合成可裁剪装配、首个实际组合变化的定制化六层验证以及 v0.1.0-demo.5 高保真可移植开发包均已通过。Customer Scenario Evidence v1 已把六层方法固化为失败关闭的可执行账目，并提供参考 OA 基准和不冒充通过的真实客户草稿。配置、类型化扩展与客户装配不得产生客户核心分支、共享业务表、客户数据库或客户 migration。当前通过结论不包含第二真实客户、正式 clean-HEAD 发布、完整入口、公司 Ubuntu、公网入口或真实客户验收。",
              "reason": "六个独立应用、全量参考 OA 和一个未选应用真实缺席的中间组合共同提供当前参考产品的装配证据；Customer Scenario Evidence 继续约束跨客户结论。",
              "evidence": [
                "arckit/spec/assembly/demo-validation-2.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/testing.md#设计交付子集装配验收2026-09-25-已通过",
                "arckit/spec/assembly/customer-scenario-evidence.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The accepted capability statement now includes direct selected-subset runtime evidence and retains the second-customer limit.",
            "evidence": [
              "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
              "arckit/spec/assembly/demo-validation-2.md",
              "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": null,
        "evidence": [
          "arckit/spec/assembly/demo-validation-2.md",
          "arckit/tech/assembly/local-reference-oa-composition.md",
          "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
        ]
      },
      "invariant_assessment": {
        "project_revision": 51,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The selected-subset capability and its synthetic evidence boundary are explicit in the authoritative assembly specification.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "This gate changes runtime assembly evidence and does not define a user interaction or navigation contract.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "No visual language or presentation rule changes in this composition proof.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The isolated project, selected service set, authentication mode, ports, replay and cleanup boundaries are durably documented.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The current source, script and live run realize the claimed subset behavior using real services, migrations and APIs.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Hidden startup coupling, accidental full-suite use, replay drift, cleanup contamination and full-suite regression were all checked directly.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "deployments/local-oa/ops/local-oa composition-audit: PASS on 2026-09-25",
        "acceptance/reference-oa-demo: go test and go vet passed",
        "deployments/local-oa/ops/local-oa: sh -n passed",
        "deployments/local-oa/compose.yaml: docker compose config --quiet passed",
        "deployments/local-oa/ops/local-oa demo: full J1-J4 regression passed",
        "git diff --check: passed"
      ],
      "runtime_result_ref": "direct-agent:2026-09-25-subset-assembly-resolution",
      "occurred_at": "2026-09-25T03:51:28.426Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Complete the implementation-focused review of the synthetic subset assembly proof.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The ordinary composition gap is resolved and the completion review is the only ready Agent-owned obligation in this Case.",
        "snapshot_token": "ad9642b9cd2c59ca40d0f15a858a91a99f859cf21f9237738e3e65fbf5782f96",
        "selected_ref": "case-gap:CASE-20260925-004:CASE-20260925-004:completion-review:1",
        "comparison_summary": "The completion review is selected over the two Human-owned environment prerequisites because it is required to close the just-completed local implementation and all review evidence is available now.",
        "fresh_discovery_summary": "Fresh rerun, residue inspection, full-suite verification and release-bundle regression found no new implementation gap.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-004:CASE-20260925-004:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Required Case completion review",
              "risk": "Checks regression and evidence credibility"
            },
            "reason": "All five review dimensions can be assessed from current direct evidence."
          },
          {
            "ref": "case-gap:CASE-20260923-001:GAP-20260923-001-009",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks personal mainland public acceptance",
              "responsibility": "human"
            },
            "reason": "The owner-controlled ICP or alternate-ingress prerequisite remains independent of this review."
          },
          {
            "ref": "case-gap:CASE-20260923-003:GAP-20260923-003-002",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks company Ubuntu deployment",
              "responsibility": "human"
            },
            "reason": "Company authorization and target resource details remain external prerequisites."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260925-004:completion-review:1",
        "responsibility": "agent",
        "goal": "Review the completed implementation for correctness, real problem resolution, verification credibility, regression risk, and minimality.",
        "reason": "All ordinary Case gaps and state impacts are closed.",
        "derived_from": [
          "case_result",
          "content_revision:1"
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
        "goal": "Complete the implementation-focused review of the synthetic subset assembly proof.",
        "expected_state_change": "Record a clean review across correctness, problem resolution, evidence credibility, regression risk and minimality, then close the Case."
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
          "dimensions": {
            "implementation_correctness": "clean",
            "problem_resolution": "clean",
            "verification_credibility": "clean",
            "regression_risk": "clean",
            "minimality": "clean"
          },
          "findings": [],
          "evidence": [
            "deployments/local-oa/ops/local-oa composition-audit rerun: PASS on 2026-09-25 with stable replay",
            "post-audit Docker inspection: zero isolated containers, volumes, networks and state file",
            "deployments/local-oa/ops/local-oa verify: all six full-suite applications ready",
            "deployments/local-oa/ops/local-oa demo: full J1-J4 regression passed",
            "make local-oa-release-bundle-test: passed",
            "acceptance/reference-oa-demo: go test and go vet passed",
            "sh -n, Compose config and git diff --check: passed",
            "development-case and project-state validation: passed",
            "protocol compatibility: compatible"
          ],
          "reviewed_content_revision": 1
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
        "project_revision": 52,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The reviewed implementation still matches the authoritative selected-subset product expectation and explicit synthetic-evidence limit.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "The reviewed change has no durable user interaction contract.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The reviewed change has no visual-language scope.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The implementation, isolation strategy, replay and cleanup remain documented and directly inspectable.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "A fresh rerun reproduced the accepted selected-subset behavior and full-suite regression remained green.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/local-oa composition-audit rerun: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Fresh-state startup, exact service-set checks, stable replay, zero residue, full-suite verification and release-bundle regression proportionately cover the material risks.",
            "fact_refs": [
              "FACT-20260925-004-002"
            ],
            "evidence": [
              "docs/testing.md#设计交付子集装配验收2026-09-25-已通过"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "completion review evidence set passed on 2026-09-25",
        "CASE-20260925-004 and Project State validate",
        "protocol compatibility snapshot 906269803be0d254aa5c9aa5fae4c499bfeec0034af783cc615725aa851d8de6"
      ],
      "runtime_result_ref": "direct-agent:2026-09-25-subset-assembly-completion-review",
      "occurred_at": "2026-09-25T03:53:15.425Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260925-004-001"
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
    "updated_at": "2026-09-25T03:53:15.425Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
