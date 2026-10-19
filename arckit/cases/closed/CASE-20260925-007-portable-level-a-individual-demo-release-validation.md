# Portable Level A individual demo release validation

Case: CASE-20260925-007
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T05:03:06.685Z

## User Intent

Make each of the six small systems independently runnable and business-verifiable from the same source-free Linux/amd64 development bundle before composing the reference OA.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-007",
  "title": "Portable Level A individual demo release validation",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T04:33:13.640Z",
  "updated_at": "2026-09-25T05:03:06.685Z",
  "user_intent": "Make each of the six small systems independently runnable and business-verifiable from the same source-free Linux/amd64 development bundle before composing the reference OA.",
  "expected_outcome": "One immutable source-free bundle can execute fresh-state, business-acceptance, replay-stability and cleanup checks for Contract Ledger, Money Flow Ledger, People & Groups, Evidence Service, Project Workspace and Design Operations individually, while preserving the already accepted two-subset and full-suite paths.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-007-001",
      "revision": 1,
      "status": "accepted",
      "statement": "All six Level A demos pass from the source worktree, but the current v0.1.0-demo.8 source-free bundle contains only the two subset compositions and the complete six-application OA path; it does not carry the six individual acceptance drivers or the Project Workspace and Design Operations public-contract reference stubs.",
      "basis": "Direct inspection of deployments/demos, acceptance/*-demo and deployments/local-oa/ops/build-release after the accepted demo.8 clean-room release.",
      "evidence": [
        "deployments/demos/ops/level-a",
        "deployments/demos/level-a-capabilities.tsv",
        "deployments/local-oa/ops/build-release",
        "deployments/local-oa/ops/verify-release-bundle"
      ]
    },
    {
      "id": "FACT-20260925-007-002",
      "revision": 1,
      "status": "accepted",
      "statement": "The source-free Linux/amd64 v0.1.0-demo.9 development archive with SHA-256 85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b passed integrity and nine-image platform verification, all six isolated Level A product audits, both exact subset assemblies, complete J1-J4 replay, five-source outage isolation and zero-residue cleanup from one clean extraction.",
      "basis": "Direct execution of the extracted archive with pull-disabled Compose, seven bundled static drivers, nine immutable images, fresh secrets, exact service-set assertions, repeated business flows and post-run Docker residue inspection.",
      "evidence": [
        "make local-oa-release-acceptance: PASS on 2026-09-25",
        "docs/testing.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/tech/assembly/local-reference-oa-composition.md"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-007-001",
      "fact_id": "FACT-20260925-007-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "delivery_and_distribution",
        "revision": 8
      },
      "effect": "upheld",
      "reason": "The same source-free artifact now proves the requested individual-product layer as well as the previously accepted subset and full-suite layers without source, target compilation or Registry pull.",
      "gap_ids": [],
      "evidence": [
        "docs/decisions.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "docs/testing.md"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-007-001",
      "status": "resolved",
      "goal": "Produce and clean-room verify a source-free Linux/amd64 bundle that independently runs all six Level A demos with exact service boundaries, existing business acceptance, replay stability and cleanup, without regressing subset or full-suite acceptance.",
      "reason": "The current release cannot yet prove the users required validation order on a clean Ubuntu-compatible host without repository source and Go tooling.",
      "derived_from": [
        "FACT-20260925-007-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks completion of the portable small-system-first demo layer",
        "risk": "A release may appear portable while individual products still depend on source or neighboring applications",
        "user_impact": "The company Ubuntu demonstration cannot start with isolated products as requested",
        "evidence": "A clean-room no-source run can deterministically prove all six products, both subsets and the full OA"
      },
      "responsibility": "agent",
      "evidence_required": [
        "All six individual demo flows pass from an extracted source-free linux/amd64 archive",
        "Project Workspace and Design Operations run only with public-contract reference stubs rather than neighboring business applications",
        "Both existing subset audits and the complete OA audit still pass",
        "Cleanup leaves no bundle-owned containers, volumes or networks"
      ],
      "resolution": {
        "id": "GAP-20260925-007-001",
        "status": "resolved",
        "outcome": "verified_portable_level_a_release",
        "reason": "The final v0.1.0-demo.9 archive ran all six isolated Level A products with exact boundaries and cleanup before passing both subset audits and the full J1-J4 plus outage-isolation acceptance.",
        "evidence": [
          "make local-oa-release-acceptance: PASS on 2026-09-25",
          "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
          "dist/omsbizfoundry-local-oa_v0.1.0-demo.9_linux_amd64.tar.gz.sha256",
          "deployments/local-oa/ops/verify-release-bundle"
        ],
        "occurred_at": "2026-09-25T05:01:10.613Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 3,
      "source": "User-requested autonomous local implementation with completion review",
      "snapshotted_at": "2026-09-25T04:33:13.640Z"
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
          "Final v0.1.0-demo.9 clean-room release acceptance: PASS on 2026-09-25",
          "All six source-free Level A products passed exact-boundary fresh-state, run, replay and cleanup gates",
          "Design-delivery and commercial-ledger subsets passed exact service selection and stable replay from the same archive",
          "Full J1-J4 replay and five-source outage isolation passed from the same archive",
          "Archive sha256=85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b",
          "Seven acceptance drivers: go test and go vet passed",
          "make local-oa-release-bundle-test, shell syntax and git diff --check passed",
          "Post-acceptance Docker inspection found zero bundle-owned clean-room containers, volumes and networks",
          "Protocol reconciliation preserved the unresolved ICP Case and returned canonical compatibility to compatible"
        ],
        "occurred_at": "2026-09-25T05:03:06.685Z"
      }
    ],
    "evidence": [
      "Final v0.1.0-demo.9 clean-room release acceptance: PASS on 2026-09-25",
      "All six source-free Level A products passed exact-boundary fresh-state, run, replay and cleanup gates",
      "Design-delivery and commercial-ledger subsets passed exact service selection and stable replay from the same archive",
      "Full J1-J4 replay and five-source outage isolation passed from the same archive",
      "Archive sha256=85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b",
      "Seven acceptance drivers: go test and go vet passed",
      "make local-oa-release-bundle-test, shell syntax and git diff --check passed",
      "Post-acceptance Docker inspection found zero bundle-owned clean-room containers, volumes and networks",
      "Protocol reconciliation preserved the unresolved ICP Case and returned canonical compatibility to compatible"
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
      "goal": "Accept v0.1.0-demo.9 as the current development artifact proving six independent products, two subsets and the complete reference OA from one source-free archive.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The portable Level A release gap is the only ready persisted candidate in this Case and all required implementation and clean-room evidence is now available.",
        "snapshot_token": "b4428ec79157310d000ada3709b01f66311cb49ebfc4a08c51dfa402842dc6b2",
        "selected_ref": "case-gap:CASE-20260925-007:GAP-20260925-007-001",
        "comparison_summary": "Selected the only Agent-owned Case gap; the company Ubuntu and public-ingress gaps remain Human-owned in their separate Cases and do not block this local artifact transition.",
        "fresh_discovery_summary": "Final clean-room acceptance and focused regressions found no additional release-runtime gap after correcting the rejected first candidate image.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-007:GAP-20260925-007-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks the requested portable small-system-first demo",
              "risk": "Hidden source or neighboring-app dependencies"
            },
            "reason": "The final source-free archive and all required evidence are available for deterministic resolution."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-007-001",
        "responsibility": "agent",
        "goal": "Produce and clean-room verify a source-free Linux/amd64 bundle that independently runs all six Level A demos with exact service boundaries, existing business acceptance, replay stability and cleanup, without regressing subset or full-suite acceptance.",
        "reason": "The current release cannot yet prove the users required validation order on a clean Ubuntu-compatible host without repository source and Go tooling.",
        "derived_from": [
          "FACT-20260925-007-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks completion of the portable small-system-first demo layer",
          "uncertainty": "",
          "risk": "A release may appear portable while individual products still depend on source or neighboring applications",
          "user_impact": "The company Ubuntu demonstration cannot start with isolated products as requested",
          "evidence": "A clean-room no-source run can deterministically prove all six products, both subsets and the full OA"
        },
        "evidence_required": [
          "All six individual demo flows pass from an extracted source-free linux/amd64 archive",
          "Project Workspace and Design Operations run only with public-contract reference stubs rather than neighboring business applications",
          "Both existing subset audits and the complete OA audit still pass",
          "Cleanup leaves no bundle-owned containers, volumes or networks"
        ]
      },
      "planned_transition": {
        "goal": "Accept v0.1.0-demo.9 as the current development artifact proving six independent products, two subsets and the complete reference OA from one source-free archive.",
        "expected_state_change": "Resolve the portable Level A gap, record the final artifact evidence, uphold the delivery decision at its new revision and move the Case to completion review."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-007-001",
          "status": "resolved",
          "outcome": "verified_portable_level_a_release",
          "reason": "The final v0.1.0-demo.9 archive ran all six isolated Level A products with exact boundaries and cleanup before passing both subset audits and the full J1-J4 plus outage-isolation acceptance.",
          "evidence": [
            "make local-oa-release-acceptance: PASS on 2026-09-25",
            "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
            "dist/omsbizfoundry-local-oa_v0.1.0-demo.9_linux_amd64.tar.gz.sha256",
            "deployments/local-oa/ops/verify-release-bundle"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-007-002",
            "revision": 1,
            "status": "accepted",
            "statement": "The source-free Linux/amd64 v0.1.0-demo.9 development archive with SHA-256 85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b passed integrity and nine-image platform verification, all six isolated Level A product audits, both exact subset assemblies, complete J1-J4 replay, five-source outage isolation and zero-residue cleanup from one clean extraction.",
            "basis": "Direct execution of the extracted archive with pull-disabled Compose, seven bundled static drivers, nine immutable images, fresh secrets, exact service-set assertions, repeated business flows and post-run Docker residue inspection.",
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25",
              "docs/testing.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/tech/assembly/local-reference-oa-composition.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260925-007-001",
            "fact_id": "FACT-20260925-007-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "delivery_and_distribution",
              "revision": 8
            },
            "effect": "upheld",
            "reason": "The same source-free artifact now proves the requested individual-product layer as well as the previously accepted subset and full-suite layers without source, target compilation or Registry pull.",
            "gap_ids": [],
            "evidence": [
              "docs/decisions.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "docs/testing.md"
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
            "area_ref": "delivery_and_distribution",
            "observed_revision": 7,
            "set_decision": {
              "status": "settled",
              "statement": "Suite Manifest 决定启用应用，Deployment Profile 决定放置；D1-D4 使用同一镜像、API、migration 和数据边界。目标机只装载或拉取已验证的不可变制品，不依赖 Docker Hub、源码或现场编译。参考 OA 业务组合的 v0.1.0-demo.9 开发包包含七个正式组合运行镜像、两个仅供 Project／Design Level A 使用的公开契约桩镜像、双层摘要、pull-disabled Compose 和七个静态验收驱动；同一 linux/amd64 离线归档可依次复演六个独立产品、设计交付子集、商业台账子集和完整六应用参考 OA。可先在授权 Ubuntu 主机复验业务架构，再单独装配 Nginx、Gateway、中心 IAM、Workbench、TLS 与 Evidence Provider。个人环境只作可重建试验田；公司环境必须由公司控制并重新配置 Registry、ECS、域名／证书、OSS、RAM、秘密和备份。",
              "reason": "BF-014、BF-015、BF-022、BF-024、BF-026、BF-027、BF-028、BF-033 及 v0.1.0-demo.9 的最终 clean-room 验收共同固定交付、环境所有权和分阶段验收边界。",
              "evidence": [
                "docs/decisions.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/operations/local-oa-ubuntu-demo.md",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The delivery decision now records the verified source-free individual-product layer and distinguishes the two Level A stubs from the seven full-composition runtime images.",
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25",
              "docs/decisions.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "docs/testing.md"
            ]
          }
        ],
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
          "docs/decisions.md",
          "arckit/tech/assembly/local-reference-oa-composition.md",
          "docs/operations/local-oa-ubuntu-demo.md",
          "docs/testing.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 60,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The single-product, subset and full-suite capability levels and their evidence limits are explicit in the authoritative specification and decision index.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "docs/decisions.md",
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "This release-runtime change does not alter durable end-user navigation or interaction semantics.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "No visual-language or presentation rule changes in this release-runtime work.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Release mode, image roles, cross-platform driver execution, exact service boundaries and cleanup are durably documented.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The final extracted archive directly reproduced all six independent products, both subsets and the complete reference OA.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Integrity, no-source execution, target platform, exact boundaries, replay, outage recovery and cleanup were checked directly by repeatable gates.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "make local-oa-release-acceptance: PASS on 2026-09-25",
        "archive sha256=85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b",
        "portable design-delivery replay sha256=f1e521377018e76fb7fb566c00f5cfb9a98d32471f5047633eaba70acf880781",
        "portable commercial-ledger replay sha256=36b322bbbfceca0c4f11a3c99845010142f947906a3d80ae6e0d3c1031e248f9",
        "seven driver go test and go vet: passed",
        "make local-oa-release-bundle-test: passed",
        "shell syntax, git diff --check, archive checksum and zero-residue inspection: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:01:10.613Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Complete the implementation-focused review of the portable six-product development release.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "All ordinary gaps and impacts are closed, leaving the required implementation-focused completion review as the only ready candidate in this Case.",
        "snapshot_token": "a0fee28222719b2aa88c4dd13b9e9be729887a09ef4adeb4fdd7451f6bd33dc5",
        "selected_ref": "case-gap:CASE-20260925-007:CASE-20260925-007:completion-review:1",
        "comparison_summary": "Selected the only Case-local completion-review candidate; company Ubuntu and public ingress remain separate Human-owned Cases.",
        "fresh_discovery_summary": "Independent review of the final artifact, release scripts, focused driver regressions, documentation, resource cleanup and protocol state found no implementation error, omission or excess.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-007:CASE-20260925-007:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Required before Case closure",
              "risk": "Confirms artifact correctness and evidence credibility"
            },
            "reason": "All five review dimensions have direct local evidence."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260925-007:completion-review:1",
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
        "goal": "Complete the implementation-focused review of the portable six-product development release.",
        "expected_state_change": "Record a clean review across correctness, problem resolution, verification credibility, regression risk and minimality, then resolve this bounded Case without altering implementation content."
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
            "Final v0.1.0-demo.9 clean-room release acceptance: PASS on 2026-09-25",
            "All six source-free Level A products passed exact-boundary fresh-state, run, replay and cleanup gates",
            "Design-delivery and commercial-ledger subsets passed exact service selection and stable replay from the same archive",
            "Full J1-J4 replay and five-source outage isolation passed from the same archive",
            "Archive sha256=85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b",
            "Seven acceptance drivers: go test and go vet passed",
            "make local-oa-release-bundle-test, shell syntax and git diff --check passed",
            "Post-acceptance Docker inspection found zero bundle-owned clean-room containers, volumes and networks",
            "Protocol reconciliation preserved the unresolved ICP Case and returned canonical compatibility to compatible"
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
        "project_revision": 61,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The reviewed implementation matches the documented single-product, subset and full-suite capability levels without expanding claims to company Ubuntu or a second customer.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "docs/decisions.md",
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "The reviewed release-runtime work does not change durable end-user interaction semantics.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The reviewed work has no visual-language scope.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Image roles, static drivers, release mode, target-platform behavior, exact boundaries and cleanup remain directly inspectable and documented.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The accepted artifact fact is realized by the final archive and complete clean-room execution.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "Final v0.1.0-demo.9 clean-room release acceptance: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "The review confirms proportionate repeatable evidence for source-free execution, platform identity, isolation, replay, outages, regression and cleanup.",
            "fact_refs": [
              "FACT-20260925-007-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "Final v0.1.0-demo.9 clean-room release acceptance: PASS on 2026-09-25",
        "All six source-free Level A products passed exact-boundary fresh-state, run, replay and cleanup gates",
        "Design-delivery and commercial-ledger subsets passed exact service selection and stable replay from the same archive",
        "Full J1-J4 replay and five-source outage isolation passed from the same archive",
        "Archive sha256=85dffce7a10e5e854394c6333fe02bcb30fcacc9214436aadd4761d76326ea7b",
        "Seven acceptance drivers: go test and go vet passed",
        "make local-oa-release-bundle-test, shell syntax and git diff --check passed",
        "Post-acceptance Docker inspection found zero bundle-owned clean-room containers, volumes and networks",
        "Protocol reconciliation preserved the unresolved ICP Case and returned canonical compatibility to compatible"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T05:03:06.685Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260925-007-001"
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
    "updated_at": "2026-09-25T05:03:06.685Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
