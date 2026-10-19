# 本地参考 OA 可移植 Ubuntu 演示包

Case: CASE-20260924-001
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-24T20:04:20.805Z

## User Intent

把已通过的六应用 J1～J4 本地业务组合固化为不在目标机编译源码、可校验并可在空白 Ubuntu Docker 主机启动的演示交付包，同时继续排除 Nginx、Gateway、中心 IAM 和公网配置。

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260924-001",
  "title": "本地参考 OA 可移植 Ubuntu 演示包",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-24T19:37:41.907Z",
  "updated_at": "2026-09-24T20:04:20.805Z",
  "user_intent": "把已通过的六应用 J1～J4 本地业务组合固化为不在目标机编译源码、可校验并可在空白 Ubuntu Docker 主机启动的演示交付包，同时继续排除 Nginx、Gateway、中心 IAM 和公网配置。",
  "expected_outcome": "生成并验证 linux/amd64 可移植演示包；包内包含六应用与 PostgreSQL 镜像、pull-disabled Compose、随机环境初始化、健康检查、J1～J4 演示和故障隔离入口，不包含运行秘密、数据库数据或源码构建依赖。",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260924-001-001",
      "revision": 1,
      "status": "superseded",
      "statement": "六应用本地参考 OA 已完成 J1～J4、幂等重放和 Contract 停机隔离，但当前 deployments/local-oa 仍从仓库源码构建镜像，尚无可以在公司 Ubuntu 空主机直接校验、装载和启动的独立交付包。",
      "basis": "本地组合的运行证据已接受；compose.yaml 仍含六个 build 段，运行脚本的 demo 入口依赖仓库中的 Go 源码。",
      "evidence": [
        "FACT-20260923-002-011",
        "deployments/local-oa/compose.yaml",
        "deployments/local-oa/ops/local-oa",
        "docs/testing.md#M9-本地参考-OA-J1J4-组合验收2026-09-25-已通过"
      ]
    },
    {
      "id": "FACT-20260924-001-002",
      "revision": 1,
      "status": "accepted",
      "statement": "The reference-OA business composition has a self-contained linux/amd64 release archive with seven prebuilt images, a static journey driver, pull-disabled Compose, per-host secret initialization and two-layer checksums; a clean extracted copy passed readiness, J1-J4 twice with stable IDs, Contract outage isolation and recovery on an ARM build host using target-platform emulation.",
      "basis": "The actual v0.1.0-demo.3 archive was independently extracted, verified, loaded and executed from empty databases; the final run completed every declared gate.",
      "evidence": [
        "deployments/local-oa",
        "docs/operations/local-oa-ubuntu-demo.md",
        "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
        "make check on 2026-09-25: passed"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260924-001-001",
      "fact_id": "FACT-20260924-001-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "delivery_and_distribution",
        "revision": 6
      },
      "effect": "upheld",
      "reason": "The local OA composition is now distributed as a verified prebuilt target artifact and refuses target-host source build or image pull.",
      "gap_ids": [],
      "evidence": [
        "deployments/local-oa/ops/build-release",
        "deployments/local-oa/ops/local-oa",
        "docs/operations/local-oa-ubuntu-demo.md",
        "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260924-001-001",
      "status": "resolved",
      "goal": "Create and verify one self-contained linux/amd64 local-OA demonstration bundle that loads prebuilt images, starts six authenticated applications with six isolated databases, runs the same J1-J4 journey and Contract outage check, and requires no source compilation on the target host.",
      "reason": "Without a verified target-build-free bundle, the locally proven business architecture cannot yet be moved to the future company Ubuntu environment with the same artifacts and controlled variables.",
      "derived_from": [
        "FACT-20260924-001-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks direct transfer of the proven Demo to a company Ubuntu host",
        "uncertainty": "Image loading, platform pinning, bundle integrity and extracted execution have not been exercised together",
        "risk": "Rebuilding source on the target would reintroduce environment drift and could leak runtime state",
        "user_impact": "Makes the business Demo ready for a later controlled company-host demonstration"
      },
      "responsibility": "agent",
      "evidence_required": [
        "A versioned archive and external checksum contain only target runtime files, no .env, runtime state, private key or database data",
        "All seven images are linux/amd64 and the extracted bundle refuses source build or image pull",
        "A clean isolated Compose project created from the extracted bundle passes readiness, J1-J4 twice and Contract outage recovery",
        "Repository tests, shell syntax, Compose rendering and release-bundle regression tests pass"
      ],
      "resolution": {
        "id": "GAP-20260924-001-001",
        "status": "resolved",
        "outcome": "A self-contained linux/amd64 archive now loads seven prebuilt images, starts six authenticated applications with isolated databases, replays J1-J4 idempotently, survives Contract outage and requires no target source build or image pull.",
        "reason": "The final v0.1.0-demo.3 archive passed outer and inner integrity, platform checks, fresh extracted Compose startup, two journey runs, outage isolation, recovery and repository regression gates.",
        "evidence": [
          "deployments/local-oa/ops/build-release",
          "deployments/local-oa/ops/verify-release-bundle",
          "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
          "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
          "make check on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-24T20:02:26.710Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 2,
      "source": "User requested a minimum-error portable OA demonstration before full entry-chain validation.",
      "snapshotted_at": "2026-09-24T19:37:41.907Z"
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
          "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
          "make check on 2026-09-25: passed",
          "outer and inner SHA-256 verification on 2026-09-25: passed",
          "protocol compatibility and Case/Project validation on 2026-09-25: passed",
          "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
        ],
        "occurred_at": "2026-09-24T20:04:20.805Z"
      }
    ],
    "evidence": [
      "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
      "make check on 2026-09-25: passed",
      "outer and inner SHA-256 verification on 2026-09-25: passed",
      "protocol compatibility and Case/Project validation on 2026-09-25: passed",
      "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
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
      "goal": "Accept the verified source-build-free linux/amd64 reference-OA bundle and close its portability gap.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The portable Ubuntu bundle gap is the only ready candidate in this Case and its implementation and clean-room evidence are complete.",
        "snapshot_token": "777c168c7d1eaba029acc827420e64fca44dc26de94a457301ce5b13e397af92",
        "selected_ref": "case-gap:CASE-20260924-001:GAP-20260924-001-001",
        "comparison_summary": "No competing candidate exists in this Case; the two cross-project external gaps remain human-owned and outside this local delivery Case.",
        "fresh_discovery_summary": "Clean-room execution exposed and fixed two portability defects: PostgreSQL temporary-init false readiness and cross-architecture driver execution. The rebuilt bundle then passed the full acceptance.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260924-001:GAP-20260924-001-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks direct transfer of the proven Demo to an authorized company Ubuntu host",
              "risk": "Target-host builds, platform drift and false readiness would invalidate reproducibility",
              "user_impact": "Produces the requested deployable minimum OA demonstration"
            },
            "reason": "All required implementation and verification evidence is now available."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260924-001-001",
        "responsibility": "agent",
        "goal": "Create and verify one self-contained linux/amd64 local-OA demonstration bundle that loads prebuilt images, starts six authenticated applications with six isolated databases, runs the same J1-J4 journey and Contract outage check, and requires no source compilation on the target host.",
        "reason": "Without a verified target-build-free bundle, the locally proven business architecture cannot yet be moved to the future company Ubuntu environment with the same artifacts and controlled variables.",
        "derived_from": [
          "FACT-20260924-001-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks direct transfer of the proven Demo to a company Ubuntu host",
          "uncertainty": "Image loading, platform pinning, bundle integrity and extracted execution have not been exercised together",
          "risk": "Rebuilding source on the target would reintroduce environment drift and could leak runtime state",
          "user_impact": "Makes the business Demo ready for a later controlled company-host demonstration"
        },
        "evidence_required": [
          "A versioned archive and external checksum contain only target runtime files, no .env, runtime state, private key or database data",
          "All seven images are linux/amd64 and the extracted bundle refuses source build or image pull",
          "A clean isolated Compose project created from the extracted bundle passes readiness, J1-J4 twice and Contract outage recovery",
          "Repository tests, shell syntax, Compose rendering and release-bundle regression tests pass"
        ]
      },
      "planned_transition": {
        "goal": "Accept the verified source-build-free linux/amd64 reference-OA bundle and close its portability gap.",
        "expected_state_change": "The Case records a clean extracted-bundle replay, the delivery decision includes the portable local-OA artifact, and the previous missing-bundle impact becomes upheld."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260924-001-001",
          "status": "resolved",
          "outcome": "A self-contained linux/amd64 archive now loads seven prebuilt images, starts six authenticated applications with isolated databases, replays J1-J4 idempotently, survives Contract outage and requires no target source build or image pull.",
          "reason": "The final v0.1.0-demo.3 archive passed outer and inner integrity, platform checks, fresh extracted Compose startup, two journey runs, outage isolation, recovery and repository regression gates.",
          "evidence": [
            "deployments/local-oa/ops/build-release",
            "deployments/local-oa/ops/verify-release-bundle",
            "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
            "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
            "make check on 2026-09-25: passed"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260924-001-002",
            "revision": 1,
            "status": "accepted",
            "statement": "The reference-OA business composition has a self-contained linux/amd64 release archive with seven prebuilt images, a static journey driver, pull-disabled Compose, per-host secret initialization and two-layer checksums; a clean extracted copy passed readiness, J1-J4 twice with stable IDs, Contract outage isolation and recovery on an ARM build host using target-platform emulation.",
            "basis": "The actual v0.1.0-demo.3 archive was independently extracted, verified, loaded and executed from empty databases; the final run completed every declared gate.",
            "evidence": [
              "deployments/local-oa",
              "docs/operations/local-oa-ubuntu-demo.md",
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
              "make check on 2026-09-25: passed"
            ]
          }
        ],
        "facts_superseded": [
          {
            "id": "FACT-20260924-001-001",
            "revision": 1,
            "reason": "The missing portable-bundle condition has been replaced by direct clean-room release evidence.",
            "evidence": [
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
              "deployments/local-oa/ops/verify-release-bundle"
            ]
          }
        ],
        "impacts_added": [],
        "impacts_updated": [
          {
            "id": "IMPACT-20260924-001-001",
            "fact_id": "FACT-20260924-001-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "delivery_and_distribution",
              "revision": 6
            },
            "effect": "upheld",
            "reason": "The local OA composition is now distributed as a verified prebuilt target artifact and refuses target-host source build or image pull.",
            "gap_ids": [],
            "evidence": [
              "deployments/local-oa/ops/build-release",
              "deployments/local-oa/ops/local-oa",
              "docs/operations/local-oa-ubuntu-demo.md",
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed"
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
            "observed_revision": 5,
            "set_decision": {
              "status": "settled",
              "statement": "Suite Manifest 决定启用应用，Deployment Profile 决定放置；D1-D4 使用同一镜像、API、migration 和数据边界。目标机只装载或拉取已验证的不可变制品，不依赖 Docker Hub、源码或现场编译。参考 OA 业务组合另有七镜像、双层摘要、pull-disabled Compose 的 linux/amd64 离线演示包，可先在授权 Ubuntu 主机复验业务架构，再单独装配 Nginx、Gateway、中心 IAM、Workbench、TLS 与 Evidence Provider。个人环境只作可重建试验田；公司环境必须由公司控制并重新配置 Registry、ECS、域名／证书、OSS、RAM、秘密和备份。",
              "reason": "BF-014、BF-015、BF-022、BF-024、BF-026、BF-027、BF-028 及已验证的本地 OA 可移植包共同固定交付、环境所有权和分阶段验收边界。",
              "evidence": [
                "docs/decisions.md",
                "docs/architecture.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/operations/local-oa-ubuntu-demo.md",
                "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The accepted delivery model now has direct target-build-free evidence for the six-application business Demo in addition to the full D1 artifact path.",
            "evidence": [
              "deployments/local-oa/ops/build-release",
              "deployments/local-oa/ops/verify-release-bundle",
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": null,
        "evidence": [
          "arckit/tech/assembly/local-reference-oa-composition.md",
          "docs/operations/local-oa-ubuntu-demo.md",
          "docs/testing.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 23,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The package preserves the accepted six-application J1-J4 scope and explicitly excludes the still-deferred full entry chain.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "arckit/tech/assembly/local-reference-oa-composition.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The same authenticated J1-J4 operator journey was replayed twice with stable IDs and the same partial-unavailability behavior.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "acceptance/reference-oa-demo",
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The portable runtime packaging does not alter any visual artifact or presentation rule.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Artifact contents, platform policy, no-build startup, secret boundaries, cross-architecture verification and deferred full-entry responsibilities are documented.",
            "fact_refs": [
              "FACT-20260924-001-002"
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
            "reason": "The requested portable minimum OA Demo exists as an actual checked archive and its full declared clean-room behavior was executed.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
              "docs/testing.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Integrity, secret exclusion, platform identity, target build prohibition, first-boot database readiness, idempotency, outage isolation and recovery all have direct gates.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/verify-release",
              "deployments/local-oa/ops/verify-release-bundle",
              "make check on 2026-09-25: passed"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "dist/omsbizfoundry-local-oa_v0.1.0-demo.3_linux_amd64.tar.gz and external SHA-256 created on 2026-09-25",
        "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
        "make check on 2026-09-25: passed",
        "docs/progress.md",
        "docs/testing.md"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T20:02:26.710Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Close the portable local-OA delivery Case after an independent five-dimension completion review.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "All ordinary gaps and impacts are closed, making the deterministic completion review the only ready candidate.",
        "snapshot_token": "d6e7e5c640aa4ca918ddd11e3241493449d176ac71e07e7a204febeb59eca085",
        "selected_ref": "case-gap:CASE-20260924-001:CASE-20260924-001:completion-review:1",
        "comparison_summary": "This Case has no competing ordinary gap or external dependency.",
        "fresh_discovery_summary": "Independent review of the release contents, startup controls, clean-room transcript, regression results and documentation found no remaining error, omission or excess within the portable business-Demo boundary.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260924-001:CASE-20260924-001:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "high",
              "uncertainty": "low",
              "risk": "high",
              "user_impact": "high"
            },
            "reason": "The implementation requires a clean five-dimension review before the Case can close."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260924-001:completion-review:1",
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
        "goal": "Close the portable local-OA delivery Case after an independent five-dimension completion review.",
        "expected_state_change": "Record a clean review and close the Case without adding scope or changing implementation."
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
            "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
            "make check on 2026-09-25: passed",
            "outer and inner SHA-256 verification on 2026-09-25: passed",
            "protocol compatibility and Case/Project validation on 2026-09-25: passed",
            "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
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
        "project_revision": 24,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The final archive, technical specification and operator guide recover the exact six-application Demo scope and deferred entry-chain boundary.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "arckit/spec/assembly/reference-oa-demo.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Two clean-room replays and the Contract outage check preserve the accepted authenticated journey and recovery semantics.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "acceptance/reference-oa-demo",
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "No visual artifact or visual-system rule changed in this delivery-only Case.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The no-build artifact model, contents, secret handling, platform behavior and later full-entry boundary are explicit.",
            "fact_refs": [
              "FACT-20260924-001-002"
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
            "reason": "The actual extracted archive, rather than repository source mode, passed the claimed full journey and recovery checks.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Direct gates cover checksums, forbidden contents, target platform, pull/build prohibition, database readiness, idempotency, isolation, cleanup and full repository regression.",
            "fact_refs": [
              "FACT-20260924-001-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
              "make check on 2026-09-25: passed",
              "outer and inner SHA-256 verification on 2026-09-25: passed",
              "protocol compatibility and Case/Project validation on 2026-09-25: passed",
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "make local-oa-release-acceptance on v0.1.0-demo.3 on 2026-09-25: passed",
        "make check on 2026-09-25: passed",
        "outer and inner SHA-256 verification on 2026-09-25: passed",
        "protocol compatibility and Case/Project validation on 2026-09-25: passed",
        "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-24T20:04:20.805Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260924-001-001"
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
    "updated_at": "2026-09-24T20:04:20.805Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
