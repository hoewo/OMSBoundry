# Second-customer scenario intake readiness gate

Case: CASE-20260925-010
Status: handoff
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T05:34:29.562Z

## User Intent

Prepare a privacy-safe, human-usable intake workflow and deterministic readiness report for the second independent real-customer scenario required to falsify or uphold the current customization boundaries.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-010",
  "title": "Second-customer scenario intake readiness gate",
  "status": "handoff",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T05:21:17.093Z",
  "updated_at": "2026-09-25T05:34:29.562Z",
  "user_intent": "Prepare a privacy-safe, human-usable intake workflow and deterministic readiness report for the second independent real-customer scenario required to falsify or uphold the current customization boundaries.",
  "expected_outcome": "A business interviewer can collect the minimum anonymized evidence, map it into Customer Scenario Evidence v1, and run one command that reports every missing evidence-ready and final-acceptance obligation without weakening the existing fail-closed validator.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-010-001",
      "revision": 1,
      "status": "accepted",
      "statement": "Customer Scenario Evidence v1 and a valid draft JSON template already exist, but the repository has no operator-facing interview workflow and no deterministic report that distinguishes a structurally valid draft from evidence-ready or final-ready real-customer evidence; the second independent real-customer result therefore remains unproven and unnecessarily hard to collect.",
      "basis": "Direct inspection of the scenario-evidence specification, template, validator, tests and current Project focus after demo.9 completion.",
      "evidence": [
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "docs/templates/customer-scenario-evidence-v1.json",
        "acceptance/scenario-evidence/validate.go",
        "docs/specs/assembly/product-boundary-validation.md"
      ]
    },
    {
      "id": "FACT-20260925-010-002",
      "revision": 1,
      "status": "accepted",
      "statement": "Customer Scenario Evidence v1 now has a privacy-safe human interview guide and worksheet plus a tested readiness API, CLI and Make target that separate a schema-valid draft from evidence-ready and final-ready obligations; the reference OA is final-ready, the blank customer template is draft, and no second real-customer evidence or cross-customer conclusion has been fabricated.",
      "basis": "Direct inspection of the implemented code and durable documents together with targeted positive, negative, test, vet and contract-gate execution on 2026-09-25.",
      "evidence": [
        "docs/operations/second-customer-scenario-intake.md",
        "docs/templates/customer-scenario-intake-v1.md",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "arckit/tech/assembly/scenario-evidence-validation.md",
        "acceptance/scenario-evidence/readiness.go",
        "acceptance/scenario-evidence/cmd/readiness/main.go",
        "acceptance/scenario-evidence/validate_test.go",
        "make scenario-evidence-check on 2026-09-25: passed"
      ]
    },
    {
      "id": "FACT-20260925-010-003",
      "revision": 1,
      "status": "accepted",
      "statement": "Scenario readiness now separates absent boundary measurements from measured violations: null values remain final-readiness gaps, non-zero values are emitted as BOUNDARY_FINDING evidence, complete failed scenarios can reach final_ready, and strict passed validation still requires all seven signals to equal zero.",
      "basis": "Direct code correction and regression execution against both the existing published reference/template and a synthetic failed-scenario test derived from the valid reference document.",
      "evidence": [
        "acceptance/scenario-evidence/readiness.go",
        "acceptance/scenario-evidence/cmd/readiness/main.go",
        "acceptance/scenario-evidence/validate_test.go",
        "make scenario-evidence-check on 2026-09-25: passed"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-010-001",
      "fact_id": "FACT-20260925-010-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 51
      },
      "effect": "undetermined",
      "reason": "The collection mechanism is ready, but product adaptability across customers remains unproven until an independent real customer supplies authorized anonymized evidence and completes scenario acceptance.",
      "gap_ids": [
        "GAP-20260925-010-002"
      ],
      "evidence": [
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/operations/second-customer-scenario-intake.md"
      ]
    },
    {
      "id": "IMPACT-20260925-010-002",
      "fact_id": "FACT-20260925-010-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "quality_and_validation",
        "revision": 15
      },
      "effect": "upheld",
      "reason": "The readiness layer adds repeatable positive and negative evidence without weakening the existing fail-closed scenario contract.",
      "gap_ids": [],
      "evidence": [
        "acceptance/scenario-evidence/validate_test.go",
        "docs/testing.md",
        "make scenario-evidence-check on 2026-09-25: passed"
      ]
    },
    {
      "id": "IMPACT-20260925-010-003",
      "fact_id": "FACT-20260925-010-003",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "quality_and_validation",
        "revision": 15
      },
      "effect": "upheld",
      "reason": "The validation method now accepts truthful failure evidence while preserving the stronger zero-only requirement for passed claims.",
      "gap_ids": [],
      "evidence": [
        "acceptance/scenario-evidence/validate_test.go",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "make scenario-evidence-check on 2026-09-25: passed"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-010-001",
      "status": "resolved",
      "goal": "Add a privacy-safe second-customer interview guide and a tested readiness command that reports draft-to-evidence-ready and evidence-ready-to-final obligations while preserving the existing scenario-evidence contract and final acceptance rules.",
      "reason": "The project cannot test cross-customer customization until real evidence is collected, and a schema-valid placeholder draft is not sufficient guidance for that work.",
      "derived_from": [
        "FACT-20260925-010-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Reduces the next required human input to a precise evidence packet",
        "uncertainty": "High cross-customer business uncertainty remains",
        "risk": "A free-form interview can mix assumptions, customer facts and implementation claims",
        "user_impact": "Makes the next real scenario actionable without exposing sensitive data"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Readiness API and CLI distinguish draft, evidence-ready and final-ready obligations",
        "Published reference OA is final-ready while the blank customer template reports concrete gaps",
        "Human intake guide maps every question to the six-layer evidence contract and privacy boundary",
        "Existing fail-closed validation and negative tests remain green"
      ],
      "resolution": {
        "id": "GAP-20260925-010-001",
        "status": "resolved",
        "outcome": "A privacy-safe second-customer interview guide, reusable worksheet, readiness API and CLI now distinguish draft, evidence-ready and final-ready obligations while the original fail-closed validator remains intact.",
        "reason": "The implementation reports actionable missing evidence, the durable product and technical rules describe the same stages, and all targeted positive and negative verification passed.",
        "evidence": [
          "acceptance/scenario-evidence/readiness.go",
          "acceptance/scenario-evidence/cmd/readiness/main.go",
          "acceptance/scenario-evidence/validate_test.go",
          "docs/operations/second-customer-scenario-intake.md",
          "docs/templates/customer-scenario-intake-v1.md",
          "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
          "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed",
          "make scenario-evidence-check on 2026-09-25: passed",
          "reference OA final_ready and blank template draft readiness checks on 2026-09-25: passed",
          "required evidence_ready negative gate for the blank template on 2026-09-25: rejected as expected"
        ],
        "occurred_at": "2026-09-25T05:28:43.261Z"
      }
    },
    {
      "id": "GAP-20260925-010-002",
      "status": "open",
      "goal": "Provide one explicitly authorized, privacy-safe and anonymized second-customer interview packet whose scenario JSON reaches evidence_ready without guessing business meaning, ownership, correction rules or customization layer.",
      "reason": "Cross-customer adaptability is the remaining product-boundary uncertainty and cannot be established from the reference OA, synthetic probes or repository work alone.",
      "derived_from": [
        "FACT-20260925-010-002"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks the next cross-customer product-boundary test",
        "uncertainty": "Highest remaining product uncertainty",
        "risk": "Invented or over-detailed customer data would invalidate the conclusion or expose private information",
        "user_impact": "Determines whether the six-layer customization balance survives a second real business context"
      },
      "responsibility": "human",
      "evidence_required": [
        "Explicit authorization to use anonymized business facts and controlled evidence references",
        "Completed interview worksheet with observable outcomes, fact ownership, lifecycle, correction and smallest application set",
        "Customer scenario JSON passes strict validation and reaches evidence_ready",
        "Unknown meanings remain needs_business_evidence and no credentials, raw contracts or personal data enter the repository"
      ],
      "resolution": null
    },
    {
      "id": "GAP-20260925-010-003",
      "status": "resolved",
      "goal": "Correct the readiness gate so measured non-zero boundary signals are preserved as final failure findings rather than treated as missing evidence, without weakening passed validation.",
      "reason": "The accepted specification requires failed scenarios to retain counterevidence, but the current implementation prevents them from reaching final_ready.",
      "derived_from": [
        "FACT-20260925-010-002"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Required before using the workflow with real customer evidence",
        "risk": "A success-only evidence gate would invalidate product-boundary falsification",
        "user_impact": "Allows honest failed outcomes without rewriting measurements"
      },
      "responsibility": "agent",
      "evidence_required": [
        "A failed real-customer scenario with a measured non-zero boundary signal validates and reaches final_ready",
        "The readiness CLI emits a distinct boundary finding rather than a missing-evidence gap",
        "Passed scenarios with non-zero boundary signals remain rejected by the strict validator",
        "Specifications and operator guidance explain the distinction"
      ],
      "resolution": {
        "id": "GAP-20260925-010-003",
        "status": "resolved",
        "outcome": "Readiness now treats null boundary signals as missing measurements and non-zero signals as explicit BOUNDARY_FINDING counterevidence; a complete failed scenario can be final_ready while strict passed validation remains zero-only.",
        "reason": "The API, CLI, tests and durable guidance now consistently preserve failed evidence instead of requiring the operator to erase or falsify it.",
        "evidence": [
          "acceptance/scenario-evidence/readiness.go",
          "acceptance/scenario-evidence/cmd/readiness/main.go",
          "acceptance/scenario-evidence/validate_test.go",
          "arckit/spec/assembly/customer-scenario-evidence.md",
          "arckit/tech/assembly/scenario-evidence-validation.md",
          "docs/operations/second-customer-scenario-intake.md",
          "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
          "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed",
          "make scenario-evidence-check on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-25T05:34:29.562Z"
      }
    }
  ],
  "content_revision": 2,
  "completion_review": {
    "status": "pending",
    "policy": {
      "initial_max_cycles": 3,
      "source": "Persistent Demo Validation 2 goal and post-demo completion audit",
      "snapshotted_at": "2026-09-25T05:21:17.093Z"
    },
    "additional_cycles_authorized": 0,
    "cycle_count": 0,
    "reviewed_content_revision": null,
    "dimensions": {
      "implementation_correctness": "unknown",
      "problem_resolution": "unknown",
      "verification_credibility": "unknown",
      "regression_risk": "unknown",
      "minimality": "unknown"
    },
    "findings": [],
    "cycles": [],
    "evidence": [],
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
      "goal": "Accept the privacy-safe interview workflow and deterministic readiness gate as implemented and verified.",
      "outcome": "needs_human",
      "gap_selection": {
        "mode": "candidate",
        "basis": "This is the only ready Agent-owned gap in the selected Case and directly removes ambiguity from the next required real-customer input without inventing customer facts.",
        "snapshot_token": "dd3b7fd8679d086612065e21d536b5e806eaaccd980d63e5ffa72f7eba1ecad8",
        "selected_ref": "case-gap:CASE-20260925-010:GAP-20260925-010-001",
        "comparison_summary": "The selected Case has one persisted candidate. Company Ubuntu authorization and ICP public ingress belong to separate human-owned Cases and cannot replace the product-boundary evidence workflow.",
        "fresh_discovery_summary": "Implementation confirmed that completion of the intake gate exposes a distinct human obligation: provide one authorized anonymized second-customer evidence packet; that obligation is recorded as a new gap and is not executed in this round.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-010:GAP-20260925-010-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Makes the next real-customer input precise",
              "uncertainty": "High cross-customer business uncertainty remains",
              "risk": "Prevents a valid placeholder draft from being mistaken for evidence",
              "user_impact": "Provides a safe interview and handoff path"
            },
            "reason": "The readiness API, CLI, guide, worksheet and regression evidence now satisfy the gap as one coherent acceptance claim."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-010-001",
        "responsibility": "agent",
        "goal": "Add a privacy-safe second-customer interview guide and a tested readiness command that reports draft-to-evidence-ready and evidence-ready-to-final obligations while preserving the existing scenario-evidence contract and final acceptance rules.",
        "reason": "The project cannot test cross-customer customization until real evidence is collected, and a schema-valid placeholder draft is not sufficient guidance for that work.",
        "derived_from": [
          "FACT-20260925-010-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Reduces the next required human input to a precise evidence packet",
          "uncertainty": "High cross-customer business uncertainty remains",
          "risk": "A free-form interview can mix assumptions, customer facts and implementation claims",
          "user_impact": "Makes the next real scenario actionable without exposing sensitive data"
        },
        "evidence_required": [
          "Readiness API and CLI distinguish draft, evidence-ready and final-ready obligations",
          "Published reference OA is final-ready while the blank customer template reports concrete gaps",
          "Human intake guide maps every question to the six-layer evidence contract and privacy boundary",
          "Existing fail-closed validation and negative tests remain green"
        ]
      },
      "planned_transition": {
        "goal": "Accept the privacy-safe interview workflow and deterministic readiness gate as implemented and verified.",
        "expected_state_change": "Resolve the Agent-owned intake-readiness gap, preserve the unproven cross-customer boundary as a new human-owned evidence gap, and restore Project focus to the two external validation inputs."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-010-001",
          "status": "resolved",
          "outcome": "A privacy-safe second-customer interview guide, reusable worksheet, readiness API and CLI now distinguish draft, evidence-ready and final-ready obligations while the original fail-closed validator remains intact.",
          "reason": "The implementation reports actionable missing evidence, the durable product and technical rules describe the same stages, and all targeted positive and negative verification passed.",
          "evidence": [
            "acceptance/scenario-evidence/readiness.go",
            "acceptance/scenario-evidence/cmd/readiness/main.go",
            "acceptance/scenario-evidence/validate_test.go",
            "docs/operations/second-customer-scenario-intake.md",
            "docs/templates/customer-scenario-intake-v1.md",
            "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
            "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed",
            "make scenario-evidence-check on 2026-09-25: passed",
            "reference OA final_ready and blank template draft readiness checks on 2026-09-25: passed",
            "required evidence_ready negative gate for the blank template on 2026-09-25: rejected as expected"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-010-002",
            "revision": 1,
            "status": "accepted",
            "statement": "Customer Scenario Evidence v1 now has a privacy-safe human interview guide and worksheet plus a tested readiness API, CLI and Make target that separate a schema-valid draft from evidence-ready and final-ready obligations; the reference OA is final-ready, the blank customer template is draft, and no second real-customer evidence or cross-customer conclusion has been fabricated.",
            "basis": "Direct inspection of the implemented code and durable documents together with targeted positive, negative, test, vet and contract-gate execution on 2026-09-25.",
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md",
              "docs/templates/customer-scenario-intake-v1.md",
              "arckit/spec/assembly/customer-scenario-evidence.md",
              "arckit/tech/assembly/scenario-evidence-validation.md",
              "acceptance/scenario-evidence/readiness.go",
              "acceptance/scenario-evidence/cmd/readiness/main.go",
              "acceptance/scenario-evidence/validate_test.go",
              "make scenario-evidence-check on 2026-09-25: passed"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [
          {
            "id": "IMPACT-20260925-010-001",
            "fact_id": "FACT-20260925-010-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 26
            },
            "effect": "undetermined",
            "reason": "The collection mechanism is ready, but product adaptability across customers remains unproven until an independent real customer supplies authorized anonymized evidence and completes scenario acceptance.",
            "gap_ids": [
              "GAP-20260925-010-002"
            ],
            "evidence": [
              "docs/specs/assembly/product-boundary-validation.md",
              "docs/operations/second-customer-scenario-intake.md"
            ]
          },
          {
            "id": "IMPACT-20260925-010-002",
            "fact_id": "FACT-20260925-010-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "quality_and_validation",
              "revision": 9
            },
            "effect": "upheld",
            "reason": "The readiness layer adds repeatable positive and negative evidence without weakening the existing fail-closed scenario contract.",
            "gap_ids": [],
            "evidence": [
              "acceptance/scenario-evidence/validate_test.go",
              "docs/testing.md",
              "make scenario-evidence-check on 2026-09-25: passed"
            ]
          }
        ],
        "impacts_updated": [],
        "gaps_added": [
          {
            "id": "GAP-20260925-010-002",
            "status": "open",
            "goal": "Provide one explicitly authorized, privacy-safe and anonymized second-customer interview packet whose scenario JSON reaches evidence_ready without guessing business meaning, ownership, correction rules or customization layer.",
            "reason": "Cross-customer adaptability is the remaining product-boundary uncertainty and cannot be established from the reference OA, synthetic probes or repository work alone.",
            "derived_from": [
              "FACT-20260925-010-002"
            ],
            "blocked_by": [],
            "priority_basis": {
              "blocking": "Blocks the next cross-customer product-boundary test",
              "uncertainty": "Highest remaining product uncertainty",
              "risk": "Invented or over-detailed customer data would invalidate the conclusion or expose private information",
              "user_impact": "Determines whether the six-layer customization balance survives a second real business context"
            },
            "responsibility": "human",
            "evidence_required": [
              "Explicit authorization to use anonymized business facts and controlled evidence references",
              "Completed interview worksheet with observable outcomes, fact ownership, lifecycle, correction and smallest application set",
              "Customer scenario JSON passes strict validation and reaches evidence_ready",
              "Unknown meanings remain needs_business_evidence and no credentials, raw contracts or personal data enter the repository"
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
          "current_focus": "Await explicit company Ubuntu authorization and target details for complete entry-chain validation, or obtain a second independent real customer scenario for the next product-boundary test.",
          "project_priorities": [
            "取得公司私有 Ubuntu 的明确授权、目标资源和秘密边界后验证完整 Nginx／Gateway／IAM 入口",
            "取得第二个独立真实客户的脱敏事实、确认引用并执行场景证据验证",
            "正式交接前从干净 Git HEAD 重建并复验同一发布物",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "docs/progress.md",
          "docs/operations/second-customer-scenario-intake.md",
          "arckit/cases/active/CASE-20260923-003-d1.md",
          "arckit/cases/active/CASE-20260923-001-d1-https.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 69,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The product meaning of draft, evidence-ready, final-ready and the still-unproven cross-customer claim is explicit and durable.",
            "fact_refs": [
              "FACT-20260925-010-002"
            ],
            "evidence": [
              "arckit/spec/assembly/customer-scenario-evidence.md",
              "docs/specs/assembly/product-boundary-validation.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The human workflow now defines authorization, interview order, status feedback, stop conditions and recovery from missing evidence.",
            "fact_refs": [
              "FACT-20260925-010-002"
            ],
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md",
              "docs/templates/customer-scenario-intake-v1.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This change adds command-line and documentation workflow only and establishes no visual-language decision.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The readiness evaluator is separate from strict validation, does not mutate evidence, and documents its exact stage rules and placeholder boundary.",
            "fact_refs": [
              "FACT-20260925-010-002"
            ],
            "evidence": [
              "arckit/tech/assembly/scenario-evidence-validation.md",
              "acceptance/scenario-evidence/readiness.go",
              "acceptance/scenario-evidence/cmd/readiness/main.go"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The implementation, CLI, Make target, tests and published guidance directly realize the accepted readiness workflow without claiming a missing customer result.",
            "fact_refs": [
              "FACT-20260925-010-002"
            ],
            "evidence": [
              "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
              "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed",
              "make scenario-evidence-check on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Privacy leakage, placeholder drafts, premature final claims, false placeholder matching and missing implementation evidence are controlled by durable instructions and repeatable positive and negative checks.",
            "fact_refs": [
              "FACT-20260925-010-002"
            ],
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md",
              "acceptance/scenario-evidence/validate_test.go",
              "required evidence_ready negative gate for the blank template on 2026-09-25: rejected as expected"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "docs/operations/second-customer-scenario-intake.md",
        "docs/templates/customer-scenario-intake-v1.md",
        "acceptance/scenario-evidence/readiness.go",
        "acceptance/scenario-evidence/cmd/readiness/main.go",
        "acceptance/scenario-evidence/validate_test.go",
        "make scenario-evidence-check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:28:43.261Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Make readiness distinguish missing boundary measurements from measured non-zero failure evidence.",
      "outcome": "needs_human",
      "gap_selection": {
        "mode": "fresh",
        "basis": "A fresh correctness defect in the accepted readiness implementation would exclude truthful failed customer evidence, so it has higher priority than the human evidence handoff and can be repaired without customer input.",
        "snapshot_token": "a3801ae9ddae950bbb51f630df9d0c10113a9f882650dbc238ec58f1387fa57a",
        "selected_ref": "fresh-gap:CASE-20260925-010:GAP-20260925-010-003",
        "comparison_summary": "The persisted second-customer evidence gap remains human-owned and is deferred. The fresh Agent-owned correctness gap is selected because the tool must support both confirming and falsifying evidence before real customer data arrives.",
        "fresh_discovery_summary": "Direct comparison of the accepted status rules with boundaryReadinessGaps found that every non-zero boundary measurement was treated as incomplete, although failed scenarios are required to preserve such counterevidence.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-010:GAP-20260925-010-002",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "deferred",
            "priority_basis": {
              "blocking": "Blocks the real cross-customer test",
              "responsibility": "Requires authorized human evidence"
            },
            "reason": "The evidence packet cannot be supplied by the Agent and remains the next human obligation after the correctness repair."
          },
          {
            "ref": "fresh-gap:CASE-20260925-010:GAP-20260925-010-003",
            "source": "fresh",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Would make a truthful failed real-customer scenario impossible to finalize",
              "risk": "Biases the validation method toward successful evidence",
              "information_gain": "Restores the required falsification path"
            },
            "reason": "The contradiction is directly observable in accepted code and specification and has a bounded, testable repair."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-010-003",
        "status": "open",
        "goal": "Correct the readiness gate so measured non-zero boundary signals are preserved as final failure findings rather than treated as missing evidence, without weakening passed validation.",
        "reason": "The accepted specification requires failed scenarios to retain counterevidence, but the current implementation prevents them from reaching final_ready.",
        "derived_from": [
          "FACT-20260925-010-002"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Required before using the workflow with real customer evidence",
          "risk": "A success-only evidence gate would invalidate product-boundary falsification",
          "user_impact": "Allows honest failed outcomes without rewriting measurements"
        },
        "responsibility": "agent",
        "evidence_required": [
          "A failed real-customer scenario with a measured non-zero boundary signal validates and reaches final_ready",
          "The readiness CLI emits a distinct boundary finding rather than a missing-evidence gap",
          "Passed scenarios with non-zero boundary signals remain rejected by the strict validator",
          "Specifications and operator guidance explain the distinction"
        ],
        "resolution": null
      },
      "planned_transition": {
        "goal": "Make readiness distinguish missing boundary measurements from measured non-zero failure evidence.",
        "expected_state_change": "A complete failed scenario can reach final_ready with BOUNDARY_FINDING output, while passed still requires all boundary signals to be zero."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-010-003",
          "status": "resolved",
          "outcome": "Readiness now treats null boundary signals as missing measurements and non-zero signals as explicit BOUNDARY_FINDING counterevidence; a complete failed scenario can be final_ready while strict passed validation remains zero-only.",
          "reason": "The API, CLI, tests and durable guidance now consistently preserve failed evidence instead of requiring the operator to erase or falsify it.",
          "evidence": [
            "acceptance/scenario-evidence/readiness.go",
            "acceptance/scenario-evidence/cmd/readiness/main.go",
            "acceptance/scenario-evidence/validate_test.go",
            "arckit/spec/assembly/customer-scenario-evidence.md",
            "arckit/tech/assembly/scenario-evidence-validation.md",
            "docs/operations/second-customer-scenario-intake.md",
            "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
            "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed",
            "make scenario-evidence-check on 2026-09-25: passed"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-010-003",
            "revision": 1,
            "status": "accepted",
            "statement": "Scenario readiness now separates absent boundary measurements from measured violations: null values remain final-readiness gaps, non-zero values are emitted as BOUNDARY_FINDING evidence, complete failed scenarios can reach final_ready, and strict passed validation still requires all seven signals to equal zero.",
            "basis": "Direct code correction and regression execution against both the existing published reference/template and a synthetic failed-scenario test derived from the valid reference document.",
            "evidence": [
              "acceptance/scenario-evidence/readiness.go",
              "acceptance/scenario-evidence/cmd/readiness/main.go",
              "acceptance/scenario-evidence/validate_test.go",
              "make scenario-evidence-check on 2026-09-25: passed"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [
          {
            "id": "IMPACT-20260925-010-003",
            "fact_id": "FACT-20260925-010-003",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "quality_and_validation",
              "revision": 9
            },
            "effect": "upheld",
            "reason": "The validation method now accepts truthful failure evidence while preserving the stronger zero-only requirement for passed claims.",
            "gap_ids": [],
            "evidence": [
              "acceptance/scenario-evidence/validate_test.go",
              "arckit/spec/assembly/customer-scenario-evidence.md",
              "make scenario-evidence-check on 2026-09-25: passed"
            ]
          }
        ],
        "impacts_updated": [],
        "gaps_added": [],
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
        "selection_context_change": null,
        "evidence": []
      },
      "invariant_assessment": {
        "project_revision": 70,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The durable specification now explicitly states that a complete failed scenario is a valid final result and preserves non-zero counterevidence.",
            "fact_refs": [
              "FACT-20260925-010-003"
            ],
            "evidence": [
              "arckit/spec/assembly/customer-scenario-evidence.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Operator guidance now distinguishes a missing measurement from a boundary finding and states the permitted response.",
            "fact_refs": [
              "FACT-20260925-010-003"
            ],
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The correction changes validation semantics and CLI text only, with no visual-language effect.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Readiness completeness and boundary findings are separate fields with documented roles, while strict validation remains authoritative for final claim legality.",
            "fact_refs": [
              "FACT-20260925-010-003"
            ],
            "evidence": [
              "arckit/tech/assembly/scenario-evidence-validation.md",
              "acceptance/scenario-evidence/readiness.go"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The corrected API and CLI behavior is directly covered by automated validation and readiness tests.",
            "fact_refs": [
              "FACT-20260925-010-003"
            ],
            "evidence": [
              "go test ./acceptance/scenario-evidence/... on 2026-09-25: passed",
              "go vet ./acceptance/scenario-evidence/... on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "A dedicated regression proves failed evidence remains final-ready and the pre-existing strict validator still rejects a passed claim with any non-zero signal.",
            "fact_refs": [
              "FACT-20260925-010-003"
            ],
            "evidence": [
              "acceptance/scenario-evidence/validate_test.go",
              "make scenario-evidence-check on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "acceptance/scenario-evidence/readiness.go",
        "acceptance/scenario-evidence/cmd/readiness/main.go",
        "acceptance/scenario-evidence/validate_test.go",
        "docs/operations/second-customer-scenario-intake.md",
        "make scenario-evidence-check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:34:29.562Z"
    }
  ],
  "case_resolution": {
    "status": "unresolved",
    "stage": "working",
    "satisfied": [
      "GAP-20260925-010-001",
      "GAP-20260925-010-003"
    ],
    "remaining": [
      "GAP-20260925-010-002",
      "impact:IMPACT-20260925-010-001"
    ],
    "blocked": [],
    "reason": "2 Case obligation(s) remain.",
    "candidate_gaps": [
      {
        "id": "GAP-20260925-010-002",
        "responsibility": "human",
        "goal": "Provide one explicitly authorized, privacy-safe and anonymized second-customer interview packet whose scenario JSON reaches evidence_ready without guessing business meaning, ownership, correction rules or customization layer.",
        "reason": "Cross-customer adaptability is the remaining product-boundary uncertainty and cannot be established from the reference OA, synthetic probes or repository work alone.",
        "derived_from": [
          "FACT-20260925-010-002"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks the next cross-customer product-boundary test",
          "uncertainty": "Highest remaining product uncertainty",
          "risk": "Invented or over-detailed customer data would invalidate the conclusion or expose private information",
          "user_impact": "Determines whether the six-layer customization balance survives a second real business context"
        },
        "evidence_required": [
          "Explicit authorization to use anonymized business facts and controlled evidence references",
          "Completed interview worksheet with observable outcomes, fact ownership, lifecycle, correction and smallest application set",
          "Customer scenario JSON passes strict validation and reaches evidence_ready",
          "Unknown meanings remain needs_business_evidence and no credentials, raw contracts or personal data enter the repository"
        ]
      }
    ],
    "loop_handoff": {
      "version": "loop-handoff/v2",
      "status": "needs_human",
      "next_responsibility": "human",
      "agent_continuation_available": false,
      "human_decision_required": true,
      "trigger_mode": "user_decision",
      "responsibility_reason": "Cross-customer adaptability is the remaining product-boundary uncertainty and cannot be established from the reference OA, synthetic probes or repository work alone.",
      "next_prompt": "",
      "human_gate": {
        "required": true,
        "reason": "Cross-customer adaptability is the remaining product-boundary uncertainty and cannot be established from the reference OA, synthetic probes or repository work alone.",
        "decision_needed": "Provide one explicitly authorized, privacy-safe and anonymized second-customer interview packet whose scenario JSON reaches evidence_ready without guessing business meaning, ownership, correction rules or customization layer."
      }
    },
    "updated_at": "2026-09-25T05:34:29.562Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
