# Cross-scenario product-boundary comparison report

Case: CASE-20260925-011
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T05:48:50.449Z

## User Intent

Add a deterministic, conservative comparison report for the reference OA and one final anonymized real-customer scenario so the eventual cross-customer review is based on the same application, customization-layer, disposition and boundary evidence without automating the product judgment.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-011",
  "title": "Cross-scenario product-boundary comparison report",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T05:35:36.497Z",
  "updated_at": "2026-09-25T05:48:50.449Z",
  "user_intent": "Add a deterministic, conservative comparison report for the reference OA and one final anonymized real-customer scenario so the eventual cross-customer review is based on the same application, customization-layer, disposition and boundary evidence without automating the product judgment.",
  "expected_outcome": "Given a valid final reference baseline and a distinct valid final real-customer scenario, one command produces a stable comparison report and explicit human review obligations; incomplete, wrong-identity or duplicate inputs fail closed, and the report never claims cross-customer universality by itself.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-011-001",
      "revision": 1,
      "status": "accepted",
      "statement": "Customer Scenario Evidence v1 can strictly validate and assess readiness for one scenario, but the repository has no deterministic command that compares the final reference OA with a distinct final anonymized real-customer scenario; the technical specification explicitly lists this as a current CLI limitation even though the product-boundary method requires comparison before any cross-customer judgment.",
      "basis": "Direct inspection of acceptance/scenario-evidence, its two CLIs, the Customer Scenario Evidence specification and the product-boundary validation method after the failed-evidence readiness correction.",
      "evidence": [
        "acceptance/scenario-evidence",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "arckit/tech/assembly/scenario-evidence-validation.md",
        "docs/specs/assembly/product-boundary-validation.md"
      ]
    },
    {
      "id": "FACT-20260925-011-002",
      "revision": 1,
      "status": "accepted",
      "statement": "Customer Scenario Evidence now has a deterministic comparison API and CLI. It rejects an invalid or non-final baseline or candidate and duplicate scenario identity; reports enabled-application intersection and differences, all six customization-layer counts, all four disposition counts and all seven boundary values in fixed order; preserves a failed candidate and every non-zero boundary finding; and always emits cross_customer_validated=false with result=not_determined and six explicit human review obligations. This establishes comparison readiness only and does not provide the missing second-customer evidence or a cross-customer product conclusion.",
      "basis": "Direct implementation inspection, positive and negative API tests, command integration test, scenario gate, vet and complete repository regression on 2026-09-25.",
      "evidence": [
        "acceptance/scenario-evidence/comparison.go",
        "acceptance/scenario-evidence/comparison_test.go",
        "acceptance/scenario-evidence/cmd/compare/main.go",
        "acceptance/scenario-evidence/cmd/compare/main_test.go",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "arckit/tech/assembly/scenario-evidence-validation.md",
        "docs/testing.md"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-011-001",
      "fact_id": "FACT-20260925-011-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 26
      },
      "effect": "upheld",
      "reason": "The product-capability evidence process can now create one repeatable, conservative comparison artifact instead of ad hoc collation. This upholds comparison readiness only; actual cross-customer adaptability remains explicitly unproven and is still gated by the human evidence gap in CASE-20260925-010.",
      "gap_ids": [],
      "evidence": [
        "acceptance/scenario-evidence/comparison.go",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "docs/specs/assembly/product-boundary-validation.md",
        "make check on 2026-09-25: passed"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-011-001",
      "status": "resolved",
      "goal": "Add a tested cross-scenario comparison API and CLI that accepts only a final reference baseline plus a distinct final anonymized real-customer scenario, reports application, customization-layer, disposition and boundary differences, and always leaves semantic product judgment as explicit human review obligations.",
      "reason": "The goal requires proving whether decoupling and the customization model survive another business context; deterministic collation reduces omission and confirmation bias without pretending software can decide business equivalence.",
      "derived_from": [
        "FACT-20260925-011-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Prepares the exact evidence review required after the human scenario packet arrives",
        "uncertainty": "Cross-customer semantic equivalence remains high and must not be automated",
        "risk": "Ad hoc comparison can hide core drift, rejected requirements or non-zero boundary signals",
        "user_impact": "Shortens the eventual real-customer validation while keeping the conclusion honest"
      },
      "responsibility": "agent",
      "evidence_required": [
        "Reference and real-customer identity, final readiness and distinct scenario identity fail closed",
        "Report deterministically compares enabled applications, six customization layers, dispositions and seven boundary signals",
        "Non-zero boundary evidence and failed customer status remain visible",
        "Report contains explicit human review obligations and cannot output a universal pass claim",
        "Unit tests, vet, scenario gate and full repository check pass"
      ],
      "resolution": {
        "id": "GAP-20260925-011-001",
        "status": "resolved",
        "outcome": "A tested API and CLI now accept only a final reference baseline plus a distinct final anonymized real-customer scenario, deterministically compare the required dimensions, preserve failed and non-zero evidence, and force the semantic conclusion back to explicit human review.",
        "reason": "The implementation and durable documentation satisfy every evidence requirement while deliberately retaining cross-customer validity as not determined.",
        "evidence": [
          "acceptance/scenario-evidence/comparison.go",
          "acceptance/scenario-evidence/comparison_test.go",
          "acceptance/scenario-evidence/cmd/compare/main_test.go",
          "make scenario-evidence-check on 2026-09-25: passed",
          "make check on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-25T05:45:11.746Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 3,
      "source": "Persistent Demo Validation 2 goal and post-readiness completion audit",
      "snapshotted_at": "2026-09-25T05:35:36.497Z"
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
          "Implementation correctness: CompareScenarios validates both documents, enforces exact evidence identities and final readiness before any pointer dereference, sorts application deltas and emits all fixed dimensions",
          "Problem resolution: the report collates applications, six layers, four dispositions and seven signals while preserving a failed candidate and making every semantic product judgment an explicit human obligation",
          "Verification credibility: API positive, failed-candidate, determinism and fail-closed tests plus a real go run command integration test all passed; make scenario-evidence-check and make check passed on 2026-09-25",
          "Regression risk: the comparison code is additive to the existing v1 parser, validator and readiness API; the blank customer template and reference baseline continue through the unchanged gate",
          "Minimality: no new customer schema fields, database, service, UI, policy engine or cross-customer conclusion was added; delivery-time and semantic-equivalence judgments remain explicitly external"
        ],
        "occurred_at": "2026-09-25T05:48:50.449Z"
      }
    ],
    "evidence": [
      "Implementation correctness: CompareScenarios validates both documents, enforces exact evidence identities and final readiness before any pointer dereference, sorts application deltas and emits all fixed dimensions",
      "Problem resolution: the report collates applications, six layers, four dispositions and seven signals while preserving a failed candidate and making every semantic product judgment an explicit human obligation",
      "Verification credibility: API positive, failed-candidate, determinism and fail-closed tests plus a real go run command integration test all passed; make scenario-evidence-check and make check passed on 2026-09-25",
      "Regression risk: the comparison code is additive to the existing v1 parser, validator and readiness API; the blank customer template and reference baseline continue through the unchanged gate",
      "Minimality: no new customer schema fields, database, service, UI, policy engine or cross-customer conclusion was added; delivery-time and semantic-equivalence judgments remain explicitly external"
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
      "goal": "Complete the deterministic cross-scenario comparison mechanism without making a cross-customer product claim.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The comparison report is the only ready Agent-owned candidate in this Case, and its implementation, contract documentation, command integration test and repository regression are now complete.",
        "snapshot_token": "d976563cf26e5935487a30d84cbe81980465806ba6ca36001a00cacd8deb60ac",
        "selected_ref": "case-gap:CASE-20260925-011:GAP-20260925-011-001",
        "comparison_summary": "The selected Case has one persisted candidate. The human-owned second-customer evidence remains in CASE-20260925-010 and is not required to prove that the comparison mechanism itself fails closed and preserves counterevidence.",
        "fresh_discovery_summary": "Implementation confirmed that a complete failed customer scenario must remain comparable, and that empty difference collections need stable empty arrays for downstream consumers.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-011:GAP-20260925-011-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Prepares the exact evidence review required after the human scenario packet arrives",
              "uncertainty": "Cross-customer semantic equivalence remains high and must not be automated",
              "risk": "Ad hoc comparison can hide core drift, rejected requirements or non-zero boundary signals",
              "user_impact": "Shortens the eventual real-customer validation while keeping the conclusion honest"
            },
            "reason": "The API, CLI, fail-closed input rules, deterministic report, failed-candidate preservation and conservative conclusion are implemented and verified."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-011-001",
        "responsibility": "agent",
        "goal": "Add a tested cross-scenario comparison API and CLI that accepts only a final reference baseline plus a distinct final anonymized real-customer scenario, reports application, customization-layer, disposition and boundary differences, and always leaves semantic product judgment as explicit human review obligations.",
        "reason": "The goal requires proving whether decoupling and the customization model survive another business context; deterministic collation reduces omission and confirmation bias without pretending software can decide business equivalence.",
        "derived_from": [
          "FACT-20260925-011-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Prepares the exact evidence review required after the human scenario packet arrives",
          "uncertainty": "Cross-customer semantic equivalence remains high and must not be automated",
          "risk": "Ad hoc comparison can hide core drift, rejected requirements or non-zero boundary signals",
          "user_impact": "Shortens the eventual real-customer validation while keeping the conclusion honest"
        },
        "evidence_required": [
          "Reference and real-customer identity, final readiness and distinct scenario identity fail closed",
          "Report deterministically compares enabled applications, six customization layers, dispositions and seven boundary signals",
          "Non-zero boundary evidence and failed customer status remain visible",
          "Report contains explicit human review obligations and cannot output a universal pass claim",
          "Unit tests, vet, scenario gate and full repository check pass"
        ]
      },
      "planned_transition": {
        "goal": "Complete the deterministic cross-scenario comparison mechanism without making a cross-customer product claim.",
        "expected_state_change": "Resolve the comparison-tool gap and establish that the product-boundary evidence process is repeatable; actual adaptability remains unproven until CASE-20260925-010 receives authorized customer evidence."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-011-001",
          "status": "resolved",
          "outcome": "A tested API and CLI now accept only a final reference baseline plus a distinct final anonymized real-customer scenario, deterministically compare the required dimensions, preserve failed and non-zero evidence, and force the semantic conclusion back to explicit human review.",
          "reason": "The implementation and durable documentation satisfy every evidence requirement while deliberately retaining cross-customer validity as not determined.",
          "evidence": [
            "acceptance/scenario-evidence/comparison.go",
            "acceptance/scenario-evidence/comparison_test.go",
            "acceptance/scenario-evidence/cmd/compare/main_test.go",
            "make scenario-evidence-check on 2026-09-25: passed",
            "make check on 2026-09-25: passed"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-011-002",
            "revision": 1,
            "status": "accepted",
            "statement": "Customer Scenario Evidence now has a deterministic comparison API and CLI. It rejects an invalid or non-final baseline or candidate and duplicate scenario identity; reports enabled-application intersection and differences, all six customization-layer counts, all four disposition counts and all seven boundary values in fixed order; preserves a failed candidate and every non-zero boundary finding; and always emits cross_customer_validated=false with result=not_determined and six explicit human review obligations. This establishes comparison readiness only and does not provide the missing second-customer evidence or a cross-customer product conclusion.",
            "basis": "Direct implementation inspection, positive and negative API tests, command integration test, scenario gate, vet and complete repository regression on 2026-09-25.",
            "evidence": [
              "acceptance/scenario-evidence/comparison.go",
              "acceptance/scenario-evidence/comparison_test.go",
              "acceptance/scenario-evidence/cmd/compare/main.go",
              "acceptance/scenario-evidence/cmd/compare/main_test.go",
              "arckit/spec/assembly/customer-scenario-evidence.md",
              "arckit/tech/assembly/scenario-evidence-validation.md",
              "docs/testing.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260925-011-001",
            "fact_id": "FACT-20260925-011-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 26
            },
            "effect": "upheld",
            "reason": "The product-capability evidence process can now create one repeatable, conservative comparison artifact instead of ad hoc collation. This upholds comparison readiness only; actual cross-customer adaptability remains explicitly unproven and is still gated by the human evidence gap in CASE-20260925-010.",
            "gap_ids": [],
            "evidence": [
              "acceptance/scenario-evidence/comparison.go",
              "arckit/spec/assembly/customer-scenario-evidence.md",
              "docs/specs/assembly/product-boundary-validation.md",
              "make check on 2026-09-25: passed"
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
        "software_definition_changes": [],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": null,
        "evidence": []
      },
      "invariant_assessment": {
        "project_revision": 71,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The product specification now states exactly what is compared, what remains a human judgment and why no report can claim cross-customer validity.",
            "fact_refs": [
              "FACT-20260925-011-002"
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
            "reason": "The operator guide and Make target give a recoverable final-evidence comparison action and explicitly state the required human follow-up.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md",
              "Makefile"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This transition adds a JSON report and command-line workflow without changing user-interface visual language.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Input identity, final readiness, deterministic ordering, counterevidence preservation and the non-automated conclusion are explicit in code and the technical specification.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison.go",
              "arckit/tech/assembly/scenario-evidence-validation.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The accepted comparison behavior is implemented in the API and CLI and covered by positive, negative, determinism and command integration tests.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison_test.go",
              "acceptance/scenario-evidence/cmd/compare/main_test.go",
              "make scenario-evidence-check on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Tests fail closed on wrong identity, non-final input and duplicate IDs, preserve failed non-zero evidence, prove deterministic output and prohibit an automated universal conclusion; full repository regression also passed.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison_test.go",
              "make check on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "acceptance/scenario-evidence/comparison.go",
        "acceptance/scenario-evidence/comparison_test.go",
        "acceptance/scenario-evidence/cmd/compare/main_test.go",
        "contracts/scenario-evidence/v1/README.md",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "arckit/tech/assembly/scenario-evidence-validation.md",
        "docs/operations/second-customer-scenario-intake.md",
        "docs/specs/assembly/product-boundary-validation.md",
        "docs/testing.md",
        "make check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:45:11.746Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Review the comparison mechanism for correctness, actual problem resolution, evidence credibility, regression risk and minimality without adding features.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "All ordinary Case gaps and impacts are closed, so the derived completion review is the only ready candidate in this Case.",
        "snapshot_token": "f4bec5f021c02b41825410f24359d8fd24707e3ba7d25fc22bb4f5cb70f9c191",
        "selected_ref": "case-gap:CASE-20260925-011:CASE-20260925-011:completion-review:1",
        "comparison_summary": "The Case has no remaining ordinary work. Company-environment authorization and second-customer evidence remain independent human-owned candidates in other Cases and are not prerequisites for reviewing this comparison mechanism.",
        "fresh_discovery_summary": "Five-dimensional review found no correctness, scope, verification, regression or minimality finding. The report deliberately does not compare business meaning automatically and documents that as a human obligation rather than pretending counts are semantic equivalence.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-011:CASE-20260925-011:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "high",
              "uncertainty": "low",
              "risk": "high",
              "user_impact": "high"
            },
            "reason": "Implementation, operator behavior, automated evidence and deliberately excluded product judgment are ready for final review."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260925-011:completion-review:1",
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
        "goal": "Review the comparison mechanism for correctness, actual problem resolution, evidence credibility, regression risk and minimality without adding features.",
        "expected_state_change": "Record a clean completion review, close CASE-20260925-011 and return Project focus to the two external evidence paths that can advance the overall goal."
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
            "Implementation correctness: CompareScenarios validates both documents, enforces exact evidence identities and final readiness before any pointer dereference, sorts application deltas and emits all fixed dimensions",
            "Problem resolution: the report collates applications, six layers, four dispositions and seven signals while preserving a failed candidate and making every semantic product judgment an explicit human obligation",
            "Verification credibility: API positive, failed-candidate, determinism and fail-closed tests plus a real go run command integration test all passed; make scenario-evidence-check and make check passed on 2026-09-25",
            "Regression risk: the comparison code is additive to the existing v1 parser, validator and readiness API; the blank customer template and reference baseline continue through the unchanged gate",
            "Minimality: no new customer schema fields, database, service, UI, policy engine or cross-customer conclusion was added; delivery-time and semantic-equivalence judgments remain explicitly external"
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
        "selection_context_change": {
          "current_focus": "Await explicit company Ubuntu authorization and target details for complete entry-chain validation, or obtain a second independent real-customer scenario for the next product-boundary test.",
          "project_priorities": [
            "取得公司私有 Ubuntu 的明确授权、目标资源和秘密边界后验证完整 Nginx／Gateway／IAM 入口",
            "取得第二个独立真实客户的脱敏事实、确认引用并执行场景证据验证",
            "正式交接前从干净 Git HEAD 重建并复验同一发布物",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "arckit/cases/active/CASE-20260925-010-second-customer-scenario-intake-readiness-gate.md",
          "arckit/cases/active/CASE-20260923-003-d1.md",
          "docs/progress.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 71,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Completion review confirms the durable specification accurately separates deterministic collation from the still-unproven cross-customer product conclusion.",
            "fact_refs": [
              "FACT-20260925-011-002"
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
            "reason": "Completion review confirms the command, required inputs, output meaning and human follow-up are recoverable from the operator guide and contract README.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "docs/operations/second-customer-scenario-intake.md",
              "contracts/scenario-evidence/v1/README.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "Completion review confirms no visual artifact or presentation rule changed.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Completion review confirms input gates, ordering, failure preservation, report fields and known limitations are coherent in code and technical documentation.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison.go",
              "arckit/tech/assembly/scenario-evidence-validation.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "Completion review confirms the accepted comparison fact is realized by both the package API and executable command with direct automated coverage.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison_test.go",
              "acceptance/scenario-evidence/cmd/compare/main_test.go",
              "make check on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Completion review confirms identity confusion, incomplete evidence, duplicate scenarios, hidden failure signals and automated overclaim each have a fail-closed rule or direct regression test.",
            "fact_refs": [
              "FACT-20260925-011-002"
            ],
            "evidence": [
              "acceptance/scenario-evidence/comparison_test.go",
              "acceptance/scenario-evidence/cmd/compare/main_test.go",
              "make scenario-evidence-check on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "acceptance/scenario-evidence/comparison.go",
        "acceptance/scenario-evidence/comparison_test.go",
        "acceptance/scenario-evidence/cmd/compare/main_test.go",
        "arckit/spec/assembly/customer-scenario-evidence.md",
        "arckit/tech/assembly/scenario-evidence-validation.md",
        "docs/testing.md",
        "make check on 2026-09-25: passed",
        "git diff --check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:48:50.449Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260925-011-001"
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
    "updated_at": "2026-09-25T05:48:50.449Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
