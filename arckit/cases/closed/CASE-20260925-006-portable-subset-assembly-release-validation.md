# Portable subset assembly release validation

Case: CASE-20260925-006
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T04:29:29.412Z

## User Intent

Make both verified middle-granularity OA compositions reproducible from the source-free Ubuntu release bundle using the same immutable application images and acceptance driver.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-006",
  "title": "Portable subset assembly release validation",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T04:08:34.665Z",
  "updated_at": "2026-09-25T04:29:29.412Z",
  "user_intent": "Make both verified middle-granularity OA compositions reproducible from the source-free Ubuntu release bundle using the same immutable application images and acceptance driver.",
  "expected_outcome": "An extracted Linux release bundle can load its offline images, run the design-delivery and commercial-ledger composition audits with exact selected service sets and deterministic replay, clean their isolated state, and still pass the full-suite clean-room acceptance without source, registry access or target compilation.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-006-001",
      "revision": 1,
      "status": "accepted",
      "statement": "Both isolated subset composition audits pass from the source worktree, and the portable release already contains all seven images plus the same static acceptance driver, but the release runtime explicitly rejects both subset commands and its clean-room verifier exercises only the full six-application composition.",
      "basis": "Fresh inspection of the current runtime guards, release builder, bundle verifier and durable operating documentation.",
      "evidence": [
        "deployments/local-oa/ops/local-oa",
        "deployments/local-oa/ops/build-release",
        "deployments/local-oa/ops/verify-release-bundle",
        "deployments/local-oa/README.md",
        "arckit/tech/assembly/local-reference-oa-composition.md"
      ]
    },
    {
      "id": "FACT-20260925-006-002",
      "revision": 1,
      "status": "accepted",
      "statement": "The source-free Linux/amd64 v0.1.0-demo.8 development archive with SHA-256 c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e passed integrity verification, exact design-delivery and commercial-ledger subset assembly with stable replay and zero residue, full J1-J4 replay, five-source outage isolation and final twelve-container health from one clean extraction.",
      "basis": "Direct execution of the extracted archive with pull-disabled Compose, bundled immutable images, bundled static driver, fresh secrets, post-run Docker residue inspection and focused source regressions.",
      "evidence": [
        "make local-oa-release-acceptance: PASS on 2026-09-25",
        "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
        "docs/operations/local-oa-ubuntu-demo.md",
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/tech/assembly/local-reference-oa-composition.md"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-006-001",
      "fact_id": "FACT-20260925-006-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "delivery_and_distribution",
        "revision": 7
      },
      "effect": "upheld",
      "reason": "The portable archive now carries and verifies both selectable assemblies and the complete reference OA with the same immutable images and static acceptance driver.",
      "gap_ids": [],
      "evidence": [
        "docs/operations/local-oa-ubuntu-demo.md",
        "deployments/local-oa/README.md",
        "docs/testing.md"
      ]
    },
    {
      "id": "IMPACT-20260925-006-002",
      "fact_id": "FACT-20260925-006-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 25
      },
      "effect": "upheld",
      "reason": "Both orthogonal middle-granularity application selections are now part of the same portable demonstration capability as the complete reference OA, without expanding the claim to a second real customer.",
      "gap_ids": [],
      "evidence": [
        "arckit/spec/assembly/demo-validation-2.md",
        "docs/testing.md"
      ]
    },
    {
      "id": "IMPACT-20260925-006-003",
      "fact_id": "FACT-20260925-006-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "quality_and_validation",
        "revision": 8
      },
      "effect": "upheld",
      "reason": "The final release acceptance now checks selected-service exactness, source-free execution, stable replay, cleanup, complete-suite behavior and failure isolation from one archive.",
      "gap_ids": [],
      "evidence": [
        "deployments/local-oa/ops/verify-release-bundle",
        "docs/testing.md"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-006-001",
      "status": "resolved",
      "goal": "Extend and verify the portable release runtime so both isolated subset audits use only bundled immutable images and the bundled static driver, retain exact service-selection and replay assertions, clean their own resources, and coexist with the full clean-room acceptance.",
      "reason": "A company Ubuntu rehearsal should be able to demonstrate both business completeness and real application optionality without receiving source code or recompiling on the target host.",
      "derived_from": [
        "FACT-20260925-006-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Closes the gap between local decoupling evidence and the portable handoff artifact",
        "risk": "Source-only subset tests could hide target packaging or no-build failures until company deployment",
        "user_impact": "Enables a smaller, safer business-architecture demonstration before the full ingress stack is authorized",
        "evidence": "All required images, Compose services and static driver already exist locally"
      },
      "responsibility": "agent",
      "evidence_required": [
        "An extracted bundle runs the design-delivery audit without source, pull or build and proves Contract and Money absent",
        "The same extracted bundle runs the commercial-ledger audit and proves the other four applications absent",
        "Both audits replay with stable IDs, revisions and state digests and leave no isolated runtime residue",
        "The existing full-suite clean-room release acceptance still passes from the same bundle",
        "Release integrity checks and durable operator/spec/technical documentation describe the portable subset behavior and retained evidence limits"
      ],
      "resolution": {
        "id": "GAP-20260925-006-001",
        "status": "resolved",
        "outcome": "verified_portable_subset_release",
        "reason": "The final v0.1.0-demo.8 archive loaded seven offline images, ran both exact subset selections twice with stable state, cleaned all isolated resources, and then passed the full J1-J4 and outage-isolation acceptance from the same extracted source-free bundle.",
        "evidence": [
          "make local-oa-release-acceptance: PASS on 2026-09-25",
          "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
          "dist/omsbizfoundry-local-oa_v0.1.0-demo.8_linux_amd64.tar.gz.sha256",
          "deployments/local-oa/ops/verify-release-bundle"
        ],
        "occurred_at": "2026-09-25T04:27:49.579Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 2,
      "source": "Default autonomous review policy for a bounded release-runtime and documentation change.",
      "snapshotted_at": "2026-09-25T04:08:34.665Z"
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
          "Final v0.1.0-demo.8 clean-room release acceptance: PASS on 2026-09-25",
          "Design-delivery and commercial-ledger subsets each used exact selected services, stable replay and zero residue",
          "Full J1-J4 replay and five-source outage isolation passed from the same extracted archive",
          "Archive SHA-256 c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e and embedded release metadata/integrity verified",
          "make local-oa-release-bundle-test: passed",
          "acceptance/reference-oa-demo: go test and go vet passed from its module directory",
          "sh -n, Compose config, full-suite verify and git diff --check: passed",
          "Post-acceptance Docker inspection found zero clean-room containers, volumes and networks",
          "Protocol reconciliation preserved earlier D1 obligations and compatibility returned to compatible"
        ],
        "occurred_at": "2026-09-25T04:29:29.412Z"
      }
    ],
    "evidence": [
      "Final v0.1.0-demo.8 clean-room release acceptance: PASS on 2026-09-25",
      "Design-delivery and commercial-ledger subsets each used exact selected services, stable replay and zero residue",
      "Full J1-J4 replay and five-source outage isolation passed from the same extracted archive",
      "Archive SHA-256 c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e and embedded release metadata/integrity verified",
      "make local-oa-release-bundle-test: passed",
      "acceptance/reference-oa-demo: go test and go vet passed from its module directory",
      "sh -n, Compose config, full-suite verify and git diff --check: passed",
      "Post-acceptance Docker inspection found zero clean-room containers, volumes and networks",
      "Protocol reconciliation preserved earlier D1 obligations and compatibility returned to compatible"
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
      "goal": "Accept v0.1.0-demo.8 as the current development artifact that proves both portable subset assemblies and the complete reference OA from one source-free archive.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The portable subset release gap is the only ready Agent-owned candidate in this Case and directly closes the source-free handoff risk with all required local evidence now available.",
        "snapshot_token": "bc7b5057d9ed4b2a028dbf17d4794dce005597c1a9d5e8adbb18c3fe53fdfe27",
        "selected_ref": "case-gap:CASE-20260925-006:GAP-20260925-006-001",
        "comparison_summary": "Compared the executable portable subset release gap with the owner-controlled ICP/public-ingress prerequisite and company authorization prerequisite. The local artifact work is complete and verifiable now without assuming external authority.",
        "fresh_discovery_summary": "Three clean-room candidate iterations exposed and corrected an internal Evidence URL and mode-specific state-file bug; the final archive passed both subset audits, full-suite replay, failure isolation and cleanup. No additional Case-scoped gap remains.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-006:GAP-20260925-006-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Closes the source-free subset handoff gap",
              "risk": "Packaging and release-mode differences were directly exercised"
            },
            "reason": "The same immutable archive now provides direct evidence for both optional assemblies and the complete reference OA."
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
            "reason": "ICP filing or an approved alternate ingress remains controlled by the owner and is independent of this artifact proof."
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
        "id": "GAP-20260925-006-001",
        "responsibility": "agent",
        "goal": "Extend and verify the portable release runtime so both isolated subset audits use only bundled immutable images and the bundled static driver, retain exact service-selection and replay assertions, clean their own resources, and coexist with the full clean-room acceptance.",
        "reason": "A company Ubuntu rehearsal should be able to demonstrate both business completeness and real application optionality without receiving source code or recompiling on the target host.",
        "derived_from": [
          "FACT-20260925-006-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Closes the gap between local decoupling evidence and the portable handoff artifact",
          "uncertainty": "",
          "risk": "Source-only subset tests could hide target packaging or no-build failures until company deployment",
          "user_impact": "Enables a smaller, safer business-architecture demonstration before the full ingress stack is authorized",
          "evidence": "All required images, Compose services and static driver already exist locally"
        },
        "evidence_required": [
          "An extracted bundle runs the design-delivery audit without source, pull or build and proves Contract and Money absent",
          "The same extracted bundle runs the commercial-ledger audit and proves the other four applications absent",
          "Both audits replay with stable IDs, revisions and state digests and leave no isolated runtime residue",
          "The existing full-suite clean-room release acceptance still passes from the same bundle",
          "Release integrity checks and durable operator/spec/technical documentation describe the portable subset behavior and retained evidence limits"
        ]
      },
      "planned_transition": {
        "goal": "Accept v0.1.0-demo.8 as the current development artifact that proves both portable subset assemblies and the complete reference OA from one source-free archive.",
        "expected_state_change": "Resolve the portable subset release gap, record the final archive and replay evidence, uphold the affected delivery, capability and validation decisions, and move the Case to completion review."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-006-001",
          "status": "resolved",
          "outcome": "verified_portable_subset_release",
          "reason": "The final v0.1.0-demo.8 archive loaded seven offline images, ran both exact subset selections twice with stable state, cleaned all isolated resources, and then passed the full J1-J4 and outage-isolation acceptance from the same extracted source-free bundle.",
          "evidence": [
            "make local-oa-release-acceptance: PASS on 2026-09-25",
            "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
            "dist/omsbizfoundry-local-oa_v0.1.0-demo.8_linux_amd64.tar.gz.sha256",
            "deployments/local-oa/ops/verify-release-bundle"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-006-002",
            "revision": 1,
            "status": "accepted",
            "statement": "The source-free Linux/amd64 v0.1.0-demo.8 development archive with SHA-256 c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e passed integrity verification, exact design-delivery and commercial-ledger subset assembly with stable replay and zero residue, full J1-J4 replay, five-source outage isolation and final twelve-container health from one clean extraction.",
            "basis": "Direct execution of the extracted archive with pull-disabled Compose, bundled immutable images, bundled static driver, fresh secrets, post-run Docker residue inspection and focused source regressions.",
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25",
              "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
              "docs/operations/local-oa-ubuntu-demo.md",
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/tech/assembly/local-reference-oa-composition.md"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [
          {
            "id": "IMPACT-20260925-006-002",
            "fact_id": "FACT-20260925-006-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 25
            },
            "effect": "upheld",
            "reason": "Both orthogonal middle-granularity application selections are now part of the same portable demonstration capability as the complete reference OA, without expanding the claim to a second real customer.",
            "gap_ids": [],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "docs/testing.md"
            ]
          },
          {
            "id": "IMPACT-20260925-006-003",
            "fact_id": "FACT-20260925-006-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "quality_and_validation",
              "revision": 8
            },
            "effect": "upheld",
            "reason": "The final release acceptance now checks selected-service exactness, source-free execution, stable replay, cleanup, complete-suite behavior and failure isolation from one archive.",
            "gap_ids": [],
            "evidence": [
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md"
            ]
          }
        ],
        "impacts_updated": [
          {
            "id": "IMPACT-20260925-006-001",
            "fact_id": "FACT-20260925-006-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "delivery_and_distribution",
              "revision": 7
            },
            "effect": "upheld",
            "reason": "The portable archive now carries and verifies both selectable assemblies and the complete reference OA with the same immutable images and static acceptance driver.",
            "gap_ids": [],
            "evidence": [
              "docs/operations/local-oa-ubuntu-demo.md",
              "deployments/local-oa/README.md",
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
            "area_ref": "product_capabilities",
            "observed_revision": 24,
            "set_decision": {
              "status": "settled",
              "statement": "当前七个运行数据所有者以及 Gateway／Workbench 装配边界保持不变；Contract、Money、People、Evidence、Project、Design 六个业务应用的 Level A 独立 Demo、Level B 高保真参考 OA、People＋Evidence＋Project＋Design 的设计交付子集、Contract＋Money 的商业台账子集、首个实际组合变化的定制化六层验证以及 v0.1.0-demo.8 无源码可移植开发包均已通过；同一开发包可分别复演两个精确子组合和完整参考 OA。Customer Scenario Evidence v1 继续提供参考基准和失败关闭的真实客户账目。配置、类型化扩展与客户装配不得产生客户核心分支、共享业务表、客户数据库或客户 migration。当前通过结论不包含第二真实客户、正式 clean-HEAD 发布、完整入口、公司 Ubuntu、公网入口或真实客户验收。",
              "reason": "六个独立应用、全量参考 OA、两个方向相反且未选应用真实缺席的中间组合以及同一无源码归档的复演共同提供当前参考产品的装配证据；Customer Scenario Evidence 继续约束跨客户结论。",
              "evidence": [
                "arckit/spec/assembly/demo-validation-2.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/testing.md",
                "arckit/spec/assembly/customer-scenario-evidence.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The capability statement now records that one portable artifact reproduces both selected subsets and the complete reference OA.",
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25",
              "arckit/spec/assembly/demo-validation-2.md",
              "docs/testing.md"
            ]
          },
          {
            "area_ref": "quality_and_validation",
            "observed_revision": 7,
            "set_decision": {
              "status": "settled",
              "statement": "每个纵向闭环使用风险相称工程证据；Demo Validation 2 分别证明运行真实性、业务完整性和解耦适当性。六个 Level A、Level B 高保真组合、两个中间子组合、首份实际组合变化的六层账目以及 v0.1.0-demo.8 无源码 clean-room 复演均已通过；同一归档的发布验收还覆盖精确选择集、稳定重放、零残留、完整 J1-J4、五来源隔离与恢复。Customer Scenario Evidence v1 用严格结构、跨字段语义和正负例区分参考基线、单客户结果与跨客户结论；真实客户缺少确认或存在客户核心分支、共享业务表、跨应用内部读取、现场核心补丁或隐藏强制依赖时不得通过。第二客户、正式 clean-HEAD、完整入口、公司环境与商业证据仍必须单独验证。",
              "reason": "BF-029、BF-033、BF-034、运行证据和失败关闭校验共同固定业务、定制化、制品、场景证据与环境证据的不同层次。",
              "evidence": [
                "docs/decisions.md#BF-034",
                "arckit/spec/assembly/customer-scenario-evidence.md",
                "arckit/tech/assembly/scenario-evidence-validation.md",
                "acceptance/scenario-evidence/validate_test.go",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "Release acceptance now proportionately covers the two portable subsets and the complete composition from one artifact.",
            "evidence": [
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md"
            ]
          },
          {
            "area_ref": "delivery_and_distribution",
            "observed_revision": 6,
            "set_decision": {
              "status": "settled",
              "statement": "Suite Manifest 决定启用应用，Deployment Profile 决定放置；D1-D4 使用同一镜像、API、migration 和数据边界。目标机只装载或拉取已验证的不可变制品，不依赖 Docker Hub、源码或现场编译。参考 OA 业务组合的 v0.1.0-demo.8 开发包包含七镜像、双层摘要、pull-disabled Compose 和静态验收驱动，可从同一 linux/amd64 离线归档分别复演设计交付子集、商业台账子集和完整六应用参考 OA；可先在授权 Ubuntu 主机复验业务架构，再单独装配 Nginx、Gateway、中心 IAM、Workbench、TLS 与 Evidence Provider。个人环境只作可重建试验田；公司环境必须由公司控制并重新配置 Registry、ECS、域名／证书、OSS、RAM、秘密和备份。",
              "reason": "BF-014、BF-015、BF-022、BF-024、BF-026、BF-027、BF-028 及已验证的本地 OA 可移植包共同固定交付、环境所有权和分阶段验收边界。",
              "evidence": [
                "docs/decisions.md",
                "docs/architecture.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/operations/local-oa-ubuntu-demo.md",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The delivery decision now explicitly records portable subset operation from the same no-source archive.",
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25",
              "docs/operations/local-oa-ubuntu-demo.md",
              "docs/testing.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": null,
        "evidence": [
          "arckit/spec/assembly/demo-validation-2.md",
          "arckit/tech/assembly/local-reference-oa-composition.md",
          "docs/operations/local-oa-ubuntu-demo.md",
          "docs/testing.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 57,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The portable full and subset capabilities and their synthetic-versus-real-customer evidence boundary are explicit in the authoritative assembly specification.",
            "fact_refs": [
              "FACT-20260925-006-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "This release-runtime change does not alter a durable end-user navigation or interaction contract.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "No visual language or presentation rule changes in this release-runtime proof.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Release mode, exact service selections, network-facing Evidence URL, state-file handling, replay, cleanup and evidence limits are durably documented.",
            "fact_refs": [
              "FACT-20260925-006-002"
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
            "reason": "The final extracted archive directly reproduced both optional assemblies and the complete reference OA with the bundled images and driver.",
            "fact_refs": [
              "FACT-20260925-006-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Source-free execution, image integrity, exact selection, stable replay, missing state failure, cleanup, full-suite behavior and outage recovery were checked directly and proportionately.",
            "fact_refs": [
              "FACT-20260925-006-002"
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
        "portable design-delivery replay sha256=e4b5c08a2e4320e4a9d0a761e107f18e8b66529a0a16d91a70173bde4fb47444",
        "portable commercial-ledger replay sha256=83c935935e3f9a49400c0a91ccc66e754d0bfe2fef68cdaa97c96aac3a5f371f",
        "post-acceptance Docker inspection: zero clean-room containers, volumes and networks",
        "make local-oa-release-bundle-test: passed",
        "acceptance/reference-oa-demo: go test and go vet passed",
        "sh -n, Compose config, full-suite verify and git diff --check: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T04:27:49.579Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Complete the implementation-focused review of the portable subset assembly release.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The portable release implementation gap and all targeted impacts are closed, so the completion review is the only ready Agent-owned obligation in this Case.",
        "snapshot_token": "9c1ba25d9e1a27638727b610e280c77a9f4e395a3dfb55e2223e51a373141cb6",
        "selected_ref": "case-gap:CASE-20260925-006:CASE-20260925-006:completion-review:1",
        "comparison_summary": "Selected the required implementation-focused completion review over the two Human-owned environment prerequisites; all review evidence is local and complete while the external prerequisites remain unavailable.",
        "fresh_discovery_summary": "Fresh archive acceptance, focused regressions, residue inspection, embedded-document inspection and ledger compatibility repair found no new implementation error, omission or excess.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-006:CASE-20260925-006:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Required Case completion review",
              "risk": "Checks correctness, evidence credibility, regression and minimality"
            },
            "reason": "All five review dimensions can be assessed from direct final-artifact and source-regression evidence."
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
            "reason": "ICP filing or alternate ingress remains an owner-controlled prerequisite outside this review."
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
        "id": "CASE-20260925-006:completion-review:1",
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
        "goal": "Complete the implementation-focused review of the portable subset assembly release.",
        "expected_state_change": "Record a clean review across all five dimensions and close this bounded Case without changing its implementation content."
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
            "Final v0.1.0-demo.8 clean-room release acceptance: PASS on 2026-09-25",
            "Design-delivery and commercial-ledger subsets each used exact selected services, stable replay and zero residue",
            "Full J1-J4 replay and five-source outage isolation passed from the same extracted archive",
            "Archive SHA-256 c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e and embedded release metadata/integrity verified",
            "make local-oa-release-bundle-test: passed",
            "acceptance/reference-oa-demo: go test and go vet passed from its module directory",
            "sh -n, Compose config, full-suite verify and git diff --check: passed",
            "Post-acceptance Docker inspection found zero clean-room containers, volumes and networks",
            "Protocol reconciliation preserved earlier D1 obligations and compatibility returned to compatible"
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
        "project_revision": 58,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The reviewed implementation matches the authoritative portable full-and-subset capability and preserves the explicit synthetic-versus-real-customer evidence boundary.",
            "fact_refs": [
              "FACT-20260925-006-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "not_relevant",
            "reason": "The reviewed implementation has no durable end-user navigation or interaction-contract change.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The reviewed implementation has no visual-language scope.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "The final runtime behavior, release-mode networking, state handling, service selections, cleanup and limitations remain directly inspectable and documented.",
            "fact_refs": [
              "FACT-20260925-006-002"
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
            "reason": "A fresh final archive execution reproduced both selected subsets and the complete reference OA exactly within the accepted scope.",
            "fact_refs": [
              "FACT-20260925-006-002"
            ],
            "evidence": [
              "make local-oa-release-acceptance: PASS on 2026-09-25"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Integrity, no-source execution, exact selection, replay stability, fail-closed state handling, cleanup, complete-suite regression and outage recovery are covered by repeatable gates.",
            "fact_refs": [
              "FACT-20260925-006-002"
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
        "Final v0.1.0-demo.8 clean-room release acceptance: PASS on 2026-09-25",
        "portable design-delivery replay sha256=e4b5c08a2e4320e4a9d0a761e107f18e8b66529a0a16d91a70173bde4fb47444",
        "portable commercial-ledger replay sha256=83c935935e3f9a49400c0a91ccc66e754d0bfe2fef68cdaa97c96aac3a5f371f",
        "archive sha256=c635959fa69aae7d81eafa9e36c9c7f03cad1254a12cacd6736edc1289277a0e",
        "post-acceptance Docker inspection: zero clean-room containers, volumes and networks",
        "make local-oa-release-bundle-test: passed",
        "acceptance/reference-oa-demo: go test and go vet passed",
        "sh -n, Compose config, full-suite verify and git diff --check: passed",
        "development-case and project-state compatibility: compatible"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T04:29:29.412Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260925-006-001"
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
    "updated_at": "2026-09-25T04:29:29.412Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
