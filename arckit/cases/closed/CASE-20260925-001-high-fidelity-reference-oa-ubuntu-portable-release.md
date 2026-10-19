# High-fidelity reference OA Ubuntu portable release

Case: CASE-20260925-001
Status: closed
Artifact Type: mixed
Selected Gap: none
Updated: 2026-09-25T03:09:12.867Z

## User Intent

Package the currently verified six-application high-fidelity reference OA as a source-free, offline-loadable Ubuntu x86_64 demo release without adding Nginx, Gateway, central IAM, TLS or public ingress.

## Structured Record

```json
{
  "schema_version": "development-case-record/v5",
  "id": "CASE-20260925-001",
  "title": "High-fidelity reference OA Ubuntu portable release",
  "status": "closed",
  "artifact_type": "mixed",
  "created_at": "2026-09-25T02:41:29.615Z",
  "updated_at": "2026-09-25T03:09:12.867Z",
  "user_intent": "Package the currently verified six-application high-fidelity reference OA as a source-free, offline-loadable Ubuntu x86_64 demo release without adding Nginx, Gateway, central IAM, TLS or public ingress.",
  "expected_outcome": "A versioned portable archive containing the current high-fidelity J1-J4 driver and seven required images passes checksum, platform, clean-room startup, idempotent replay, five-source failure isolation and final health verification without registry pulls or target-host builds.",
  "project_state_ref": "arckit/project/state.record.json",
  "current_round": {
    "goal": "",
    "selected_gap": null
  },
  "facts": [
    {
      "id": "FACT-20260925-001-001",
      "revision": 1,
      "status": "accepted",
      "statement": "The current six-application high-fidelity reference OA is verified locally, while the last accepted portable bundle v0.1.0-demo.3 represents the earlier J1-J4 baseline and cannot prove transport of the current driver and five-source failure-isolation behavior.",
      "basis": "The canonical Project focus and Ubuntu operations guide explicitly require regeneration and clean-room verification before reuse of the portable-delivery claim.",
      "evidence": [
        "arckit/project/state.record.json",
        "docs/operations/local-oa-ubuntu-demo.md",
        "deployments/local-oa/ops/build-release",
        "deployments/local-oa/ops/test-release-bundle"
      ]
    },
    {
      "id": "FACT-20260925-001-002",
      "revision": 1,
      "status": "accepted",
      "statement": "On 2026-09-25 the source-free v0.1.0-demo.5 Linux amd64 development bundle for the current six-application high-fidelity reference OA passed archive integrity, seven-image platform identity, offline image loading, fresh-secret initialization, no-build/no-pull startup of six applications and six independent databases, two J1-J4 executions retaining the same Project, Contract, Obligation, Evidence and Design identities, Contract/Money/People/Evidence/Design isolated outage recovery, and final readiness of all twelve containers. The archive SHA-256 is 0eb71872f921e101139f306df136e39f3557c92564069a4e2d1be5d92bfac7fe and metadata records commit 61475e54f97342809affb6b3c6a1fa948be9f185-dirty. This proves portable development-Demo behavior, not official clean-HEAD provenance, real company Ubuntu host compatibility, Nginx/Gateway/central-IAM entry, TLS or public ingress.",
      "basis": "The generated archive, its external and internal checksums, embedded release metadata and direct clean-room execution output agree with the stable technical, product and operations documentation.",
      "evidence": [
        "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
        "deployments/local-oa/RELEASE.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/spec/assembly/reference-oa-demo.md",
        "docs/testing.md",
        "docs/progress.md",
        "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
      ]
    }
  ],
  "state_impacts": [
    {
      "id": "IMPACT-20260925-001-001",
      "fact_id": "FACT-20260925-001-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "product_capabilities",
        "revision": 21
      },
      "effect": "upheld",
      "reason": "The same independently owned six-application capability set and high-fidelity journey survives source-free transfer without adding a shared business owner or infrastructure shortcut.",
      "gap_ids": [],
      "evidence": [
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "docs/testing.md"
      ]
    },
    {
      "id": "IMPACT-20260925-001-002",
      "fact_id": "FACT-20260925-001-002",
      "fact_revision": 1,
      "target": {
        "kind": "software_decision",
        "ref": "quality_and_validation",
        "revision": 6
      },
      "effect": "upheld",
      "reason": "Clean-room no-build/no-pull execution provides the separate artifact portability evidence required by the validation decision while retaining explicit company-host and official-release exclusions.",
      "gap_ids": [],
      "evidence": [
        "docs/testing.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "deployments/local-oa/ops/verify-release-bundle"
      ]
    }
  ],
  "gaps": [
    {
      "id": "GAP-20260925-001-001",
      "status": "resolved",
      "goal": "Generate a new Ubuntu x86_64 portable release from the current high-fidelity reference OA and prove it in an isolated clean-room run, including archive integrity, offline image loading, fresh-secret initialization, six-application startup, J1-J4 execution, idempotent replay, five-source failure isolation and final health.",
      "reason": "Local Compose success does not prove that a source-free package contains the current driver, correct target-platform images and all target-host runtime inputs.",
      "derived_from": [
        "FACT-20260925-001-001"
      ],
      "blocked_by": [],
      "priority_basis": {
        "blocking": "Blocks a trustworthy handoff to the future company-private Ubuntu host.",
        "risk": "An outdated or incomplete archive can appear healthy locally but omit current behavior, depend on local build context or fail on Linux amd64.",
        "user_impact": "Provides the demo artifact that can be transported and started when the private server becomes available.",
        "evidence": "Build metadata, checksums, platform inspection and an isolated no-build/no-pull clean-room acceptance are directly observable now."
      },
      "responsibility": "agent",
      "evidence_required": [
        "A new versioned archive and external SHA-256 are generated from the current workspace.",
        "Bundle verification proves required files, release metadata, checksums and Linux amd64 image/platform identity.",
        "An isolated clean-room environment loads only bundled images, initializes new secrets and starts all six applications and databases without source builds or registry pulls.",
        "The packaged driver passes high-fidelity J1-J4 twice with stable business IDs, five-source failure isolation and final readiness."
      ],
      "resolution": {
        "id": "GAP-20260925-001-001",
        "status": "resolved",
        "outcome": "v0.1.0-demo.5 is a verified source-free Linux amd64 development Demo bundle for the current six-application high-fidelity reference OA.",
        "reason": "The archive passed outer and inner digests, target-platform image checks, offline loading, fresh-secret initialization, no-build/no-pull startup, two J1-J4 runs with identical business IDs, five independent source outages with Project remaining active and final twelve-container readiness.",
        "evidence": [
          "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
          "deployments/local-oa/RELEASE.md",
          "docs/operations/local-oa-ubuntu-demo.md",
          "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
          "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-25T03:07:23.932Z"
      }
    }
  ],
  "content_revision": 1,
  "completion_review": {
    "status": "clean",
    "policy": {
      "initial_max_cycles": 3,
      "source": "User authorized autonomous completion of current locally executable Demo Validation 2 work.",
      "snapshotted_at": "2026-09-25T02:41:29.615Z"
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
          "Implementation correctness: the final v0.1.0-demo.5 archive, embedded metadata/docs, external checksum and all seven linux/amd64 image identities agree",
          "Problem resolution: source-free no-build/no-pull startup created six applications and six independent databases with fresh secrets and completed the current high-fidelity driver",
          "Verification credibility: the clean-room run executed J1-J4 twice with stable IDs, stopped and recovered five sources independently, and ended with all twelve containers healthy",
          "Regression risk: reference-oa-demo go test/go vet and local-oa release-bundle tests passed; stale demo.4 current-document references were removed and checksum reverified",
          "Minimality: no Nginx, Gateway, central IAM, TLS, public ingress or company-environment write was added; official provenance and target-host acceptance remain explicitly excluded",
          "Arckit transition validation/dry-run and scoped git diff --check on 2026-09-25: passed"
        ],
        "occurred_at": "2026-09-25T03:09:12.867Z"
      }
    ],
    "evidence": [
      "Implementation correctness: the final v0.1.0-demo.5 archive, embedded metadata/docs, external checksum and all seven linux/amd64 image identities agree",
      "Problem resolution: source-free no-build/no-pull startup created six applications and six independent databases with fresh secrets and completed the current high-fidelity driver",
      "Verification credibility: the clean-room run executed J1-J4 twice with stable IDs, stopped and recovered five sources independently, and ended with all twelve containers healthy",
      "Regression risk: reference-oa-demo go test/go vet and local-oa release-bundle tests passed; stale demo.4 current-document references were removed and checksum reverified",
      "Minimality: no Nginx, Gateway, central IAM, TLS, public ingress or company-environment write was added; official provenance and target-host acceptance remain explicitly excluded",
      "Arckit transition validation/dry-run and scoped git diff --check on 2026-09-25: passed"
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
      "goal": "Accept v0.1.0-demo.5 as the current source-free Linux amd64 development Demo after complete clean-room verification while preserving the distinction from an official release and a real company-Ubuntu deployment.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "The portable high-fidelity package is the current Case's only ready agent-owned candidate and is the last locally executable prerequisite for a trustworthy future Ubuntu handoff.",
        "snapshot_token": "7c990276f8ac4534dc89e2be4700ab6c381d1db5b96877d9fc9930fe1c1d0f6f",
        "selected_ref": "case-gap:CASE-20260925-001:GAP-20260925-001-001",
        "comparison_summary": "This Case contains one persisted candidate. ICP filing and company authorization remain in separate human-owned Cases and do not block source-free local clean-room proof.",
        "fresh_discovery_summary": "The final development bundle can preserve current documentation, identify its dirty-base provenance and execute the target linux/amd64 driver through a bundled target image on an ARM host; no additional runtime component or deployment-surface work was required.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-001:GAP-20260925-001-001",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "Blocks a trustworthy handoff to the future company-private Ubuntu host.",
              "uncertainty": "Whether the current high-fidelity driver and all target runtime inputs survive a source-free transfer.",
              "risk": "A stale or incomplete archive could hide a local build or registry dependency.",
              "user_impact": "Provides the immediately transportable business-architecture Demo artifact.",
              "evidence": "Archive digests, metadata, platform inspection and isolated no-build/no-pull acceptance are directly observable."
            },
            "reason": "v0.1.0-demo.5 now has complete archive, provenance and clean-room execution evidence."
          }
        ]
      },
      "selected_gap": {
        "id": "GAP-20260925-001-001",
        "responsibility": "agent",
        "goal": "Generate a new Ubuntu x86_64 portable release from the current high-fidelity reference OA and prove it in an isolated clean-room run, including archive integrity, offline image loading, fresh-secret initialization, six-application startup, J1-J4 execution, idempotent replay, five-source failure isolation and final health.",
        "reason": "Local Compose success does not prove that a source-free package contains the current driver, correct target-platform images and all target-host runtime inputs.",
        "derived_from": [
          "FACT-20260925-001-001"
        ],
        "blocked_by": [],
        "priority_basis": {
          "blocking": "Blocks a trustworthy handoff to the future company-private Ubuntu host.",
          "uncertainty": "",
          "risk": "An outdated or incomplete archive can appear healthy locally but omit current behavior, depend on local build context or fail on Linux amd64.",
          "user_impact": "Provides the demo artifact that can be transported and started when the private server becomes available.",
          "evidence": "Build metadata, checksums, platform inspection and an isolated no-build/no-pull clean-room acceptance are directly observable now."
        },
        "evidence_required": [
          "A new versioned archive and external SHA-256 are generated from the current workspace.",
          "Bundle verification proves required files, release metadata, checksums and Linux amd64 image/platform identity.",
          "An isolated clean-room environment loads only bundled images, initializes new secrets and starts all six applications and databases without source builds or registry pulls.",
          "The packaged driver passes high-fidelity J1-J4 twice with stable business IDs, five-source failure isolation and final readiness."
        ]
      },
      "planned_transition": {
        "goal": "Accept v0.1.0-demo.5 as the current source-free Linux amd64 development Demo after complete clean-room verification while preserving the distinction from an official release and a real company-Ubuntu deployment.",
        "expected_state_change": "Resolve the portable-release Gap, update Project capability and validation decisions to include the accepted development artifact, and move this Case to completion review."
      },
      "accepted_state_delta": {
        "resolved_gap": {
          "id": "GAP-20260925-001-001",
          "status": "resolved",
          "outcome": "v0.1.0-demo.5 is a verified source-free Linux amd64 development Demo bundle for the current six-application high-fidelity reference OA.",
          "reason": "The archive passed outer and inner digests, target-platform image checks, offline loading, fresh-secret initialization, no-build/no-pull startup, two J1-J4 runs with identical business IDs, five independent source outages with Project remaining active and final twelve-container readiness.",
          "evidence": [
            "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
            "deployments/local-oa/RELEASE.md",
            "docs/operations/local-oa-ubuntu-demo.md",
            "docs/testing.md#M9-本地参考-OA-Ubuntu-可移植包验收2026-09-25-已通过",
            "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
          ]
        },
        "facts_added": [
          {
            "id": "FACT-20260925-001-002",
            "revision": 1,
            "status": "accepted",
            "statement": "On 2026-09-25 the source-free v0.1.0-demo.5 Linux amd64 development bundle for the current six-application high-fidelity reference OA passed archive integrity, seven-image platform identity, offline image loading, fresh-secret initialization, no-build/no-pull startup of six applications and six independent databases, two J1-J4 executions retaining the same Project, Contract, Obligation, Evidence and Design identities, Contract/Money/People/Evidence/Design isolated outage recovery, and final readiness of all twelve containers. The archive SHA-256 is 0eb71872f921e101139f306df136e39f3557c92564069a4e2d1be5d92bfac7fe and metadata records commit 61475e54f97342809affb6b3c6a1fa948be9f185-dirty. This proves portable development-Demo behavior, not official clean-HEAD provenance, real company Ubuntu host compatibility, Nginx/Gateway/central-IAM entry, TLS or public ingress.",
            "basis": "The generated archive, its external and internal checksums, embedded release metadata and direct clean-room execution output agree with the stable technical, product and operations documentation.",
            "evidence": [
              "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
              "deployments/local-oa/RELEASE.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/testing.md",
              "docs/progress.md",
              "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
            ]
          }
        ],
        "facts_superseded": [],
        "impacts_added": [
          {
            "id": "IMPACT-20260925-001-001",
            "fact_id": "FACT-20260925-001-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "product_capabilities",
              "revision": 21
            },
            "effect": "upheld",
            "reason": "The same independently owned six-application capability set and high-fidelity journey survives source-free transfer without adding a shared business owner or infrastructure shortcut.",
            "gap_ids": [],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/testing.md"
            ]
          },
          {
            "id": "IMPACT-20260925-001-002",
            "fact_id": "FACT-20260925-001-002",
            "fact_revision": 1,
            "target": {
              "kind": "software_decision",
              "ref": "quality_and_validation",
              "revision": 6
            },
            "effect": "upheld",
            "reason": "Clean-room no-build/no-pull execution provides the separate artifact portability evidence required by the validation decision while retaining explicit company-host and official-release exclusions.",
            "gap_ids": [],
            "evidence": [
              "docs/testing.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "deployments/local-oa/ops/verify-release-bundle"
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
        "software_definition_changes": [
          {
            "area_ref": "product_capabilities",
            "observed_revision": 20,
            "set_decision": {
              "status": "settled",
              "statement": "当前七个运行数据所有者以及 Gateway／Workbench 装配边界保持不变；Contract、Money、People、Evidence、Project、Design 六个业务应用的 Level A 独立 Demo、Level B 高保真参考 OA、首个实际组合变化的定制化六层验证以及 v0.1.0-demo.5 高保真可移植开发包均已通过。配置、design-v1 类型化扩展与六应用客户装配没有产生客户核心分支、共享表、客户数据库或客户 migration；规则冲突下没有创建自动连接器，无证据客户能力保持延期。当前通过结论不包含真实策略执行、第二客户适配、正式 clean-HEAD 发布、Nginx／Gateway／正式 IAM、公司 Ubuntu、公网入口或真实客户验收。",
              "reason": "BF-029、BF-031、BF-033、Level A／B、首份实际差异账目和 v0.1.0-demo.5 clean-room 证据共同证明阶段一参考产品可以用稳定内核、受控变化和同一目标制品交付。",
              "evidence": [
                "docs/decisions.md#BF-033",
                "arckit/spec/assembly/demo-validation-2.md",
                "arckit/tech/assembly/local-reference-oa-composition.md",
                "docs/testing.md",
                "docs/progress.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "The current high-fidelity portable-development-bundle exclusion is now supported by direct clean-room evidence and must be removed while all stronger deployment claims remain excluded.",
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "docs/testing.md",
              "docs/progress.md"
            ]
          },
          {
            "area_ref": "quality_and_validation",
            "observed_revision": 5,
            "set_decision": {
              "status": "settled",
              "statement": "每个纵向闭环使用风险相称工程证据；Demo Validation 2 分别证明运行真实性、业务完整性和解耦适当性。六个 Level A、Level B 高保真组合、首份实际组合变化的六层账目以及 v0.1.0-demo.5 无源码 clean-room 复演均已通过；后者覆盖摘要、目标平台、离线装载、新秘密、两次同 ID 旅程、五来源隔离和最终健康。真实策略执行、第二客户适配、正式 clean-HEAD 发布、完整入口、公司环境或商业证据仍必须单独验证，未取得的证据不得宣称通过。",
              "reason": "BF-029、BF-033、实际差异账目、兼容测试、Level A／B 实跑和当前制品 clean-room 复演共同固定业务、定制化、制品和环境证据的不同层次。",
              "evidence": [
                "docs/decisions.md#BF-033",
                "arckit/spec/assembly/demo-validation-2.md",
                "docs/operations/local-oa-ubuntu-demo.md",
                "docs/testing.md"
              ],
              "confidence": "high",
              "resume_condition": ""
            },
            "gap_refs": [],
            "reason": "Artifact portability is now verified for the current driver, while official provenance and target-environment validation remain explicitly separate.",
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "docs/testing.md"
            ]
          }
        ],
        "software_invariant_changes": [],
        "project_gap_changes": [],
        "selection_context_change": {
          "current_focus": "当前高保真 OA 的 v0.1.0-demo.5 Linux/amd64 无源码开发包已通过 clean-room 复演。下一步等待第二个真实客户场景或获授权的公司 Ubuntu 环境输入；正式交接版本从干净 Git HEAD 重建，完整 Nginx／Gateway／IAM 入口和公网仍独立验收。",
          "project_priorities": [
            "用第二个独立真实客户场景继续证伪产品边界",
            "获授权后在公司私有 Ubuntu 验证完整 Nginx／Gateway／IAM 入口",
            "正式交接前从干净 Git HEAD 重建并复验同一发布物",
            "ICP备案完成后复验个人试验田公网入口"
          ]
        },
        "evidence": [
          "docs/decisions.md",
          "docs/development-plan.md",
          "docs/progress.md",
          "docs/testing.md",
          "docs/operations/local-oa-ubuntu-demo.md"
        ]
      },
      "invariant_assessment": {
        "project_revision": 42,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The verified development-bundle scope and explicit exclusions are synchronized across product, decision, plan, progress and operations documents.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md",
              "docs/development-plan.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "The archive retains the documented init, verify, up, repeated J1-J4, failure-isolation and recovery operator flow without adding an implicit infrastructure dependency.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "deployments/local-oa/README.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "acceptance/reference-oa-demo/verify-running"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "This release work packages and verifies existing business/runtime behavior and does not establish or modify a shared visual language.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Archive contents, platform target, offline execution, six independent databases, secret creation, provenance boundary and omitted infrastructure are explicit in technical and operations evidence.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "deployments/local-oa/RELEASE.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "The portable-bundle claim is supported by the generated artifact and direct clean-room execution rather than by documentation alone.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
              "docs/testing.md",
              "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Stale content, corruption, wrong architecture, hidden registry/build reliance, copied secrets, non-idempotence and coupled source outages were exercised directly; official provenance and real-host risks remain expressly excluded.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/test-release-bundle",
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
        "deployments/local-oa/RELEASE.md",
        "deployments/local-oa/README.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/spec/assembly/reference-oa-demo.md",
        "docs/decisions.md",
        "docs/development-plan.md",
        "docs/progress.md",
        "docs/testing.md",
        "make local-oa-release-bundle-test on 2026-09-25: passed",
        "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed",
        "reference-oa-demo go test and go vet on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T03:07:23.932Z"
    },
    {
      "round": 2,
      "transition_schema_version": "arckit-case-transition/v8",
      "goal": "Perform the required five-dimensional completion review of the current high-fidelity Ubuntu portable development Demo without adding functionality or broadening its accepted scope.",
      "outcome": "completed",
      "gap_selection": {
        "mode": "candidate",
        "basis": "All ordinary Case work and state impacts are closed; completion review is the Case's only ready candidate.",
        "snapshot_token": "7ecd54aa55e46beffea530494ec88bbc69eed40a2650ccaa3d23b241e70d89f4",
        "selected_ref": "case-gap:CASE-20260925-001:CASE-20260925-001:completion-review:1",
        "comparison_summary": "This Case contains only the completion-review candidate. Company authorization, ICP filing, official release provenance and second-customer validation remain separate Project or Case axes and are not silently included in this review.",
        "fresh_discovery_summary": "Five-dimensional review found no error, omission or excess within the accepted portable-development-Demo scope; the artifact, documentation and ledger consistently exclude real company Ubuntu and the full entry chain.",
        "considered": [
          {
            "ref": "case-gap:CASE-20260925-001:CASE-20260925-001:completion-review:1",
            "source": "persisted",
            "eligibility": "ready",
            "disposition": "selected",
            "priority_basis": {
              "blocking": "high",
              "uncertainty": "low",
              "risk": "high",
              "user_impact": "high"
            },
            "reason": "The content result must pass the required semantic completion gate before the Case can close."
          }
        ]
      },
      "selected_gap": {
        "id": "CASE-20260925-001:completion-review:1",
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
        "goal": "Perform the required five-dimensional completion review of the current high-fidelity Ubuntu portable development Demo without adding functionality or broadening its accepted scope.",
        "expected_state_change": "Record a clean completion review and resolve CASE-20260925-001; target-host, official-release and full-entry verification remain separate future work."
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
            "Implementation correctness: the final v0.1.0-demo.5 archive, embedded metadata/docs, external checksum and all seven linux/amd64 image identities agree",
            "Problem resolution: source-free no-build/no-pull startup created six applications and six independent databases with fresh secrets and completed the current high-fidelity driver",
            "Verification credibility: the clean-room run executed J1-J4 twice with stable IDs, stopped and recovered five sources independently, and ended with all twelve containers healthy",
            "Regression risk: reference-oa-demo go test/go vet and local-oa release-bundle tests passed; stale demo.4 current-document references were removed and checksum reverified",
            "Minimality: no Nginx, Gateway, central IAM, TLS, public ingress or company-environment write was added; official provenance and target-host acceptance remain explicitly excluded",
            "Arckit transition validation/dry-run and scoped git diff --check on 2026-09-25: passed"
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
        "project_revision": 43,
        "judgments": [
          {
            "invariant_ref": "product-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Completion review confirms that the accepted development-Demo capability and all stronger excluded claims are consistent and recoverable across canonical and stable documents.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "arckit/spec/assembly/demo-validation-2.md",
              "arckit/spec/assembly/reference-oa-demo.md",
              "docs/decisions.md",
              "docs/development-plan.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "interaction-expectations-remain-recoverable",
            "disposition": "upheld",
            "reason": "Completion review confirms the operator flow and repeated business journey can be recovered from the bundle and durable runbook.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "deployments/local-oa/README.md",
              "docs/operations/local-oa-ubuntu-demo.md",
              "acceptance/reference-oa-demo/verify-running"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "visual-language-remains-consistent",
            "disposition": "not_relevant",
            "reason": "The Case does not modify or claim acceptance of a shared visual language.",
            "fact_refs": [],
            "evidence": [],
            "gap_refs": []
          },
          {
            "invariant_ref": "technical-decisions-remain-explainable",
            "disposition": "upheld",
            "reason": "Completion review confirms the package boundary, six-database ownership, target platform, secret boundary, provenance and infrastructure exclusions remain explicit and coherent.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "arckit/tech/assembly/local-reference-oa-composition.md",
              "deployments/local-oa/RELEASE.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "accepted-facts-are-realized",
            "disposition": "upheld",
            "reason": "Completion review traces the accepted portability claim directly to the verified archive and clean-room results, with no unexecuted environment claim promoted to fact.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
              "docs/testing.md",
              "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed"
            ],
            "gap_refs": []
          },
          {
            "invariant_ref": "material-risks-have-credible-evidence",
            "disposition": "upheld",
            "reason": "Completion review confirms all risks accepted as controlled were exercised repeatably and that official provenance, target host and full entry remain outside the claim.",
            "fact_refs": [
              "FACT-20260925-001-002"
            ],
            "evidence": [
              "deployments/local-oa/ops/test-release-bundle",
              "deployments/local-oa/ops/verify-release-bundle",
              "docs/testing.md",
              "docs/operations/local-oa-ubuntu-demo.md"
            ],
            "gap_refs": []
          }
        ]
      },
      "evidence": [
        "dist/omsbizfoundry-local-oa_v0.1.0-demo.5_linux_amd64.tar.gz.sha256",
        "deployments/local-oa/RELEASE.md",
        "deployments/local-oa/README.md",
        "docs/operations/local-oa-ubuntu-demo.md",
        "arckit/tech/assembly/local-reference-oa-composition.md",
        "arckit/spec/assembly/demo-validation-2.md",
        "arckit/spec/assembly/reference-oa-demo.md",
        "docs/decisions.md",
        "docs/development-plan.md",
        "docs/progress.md",
        "docs/testing.md",
        "make local-oa-release-bundle-test on 2026-09-25: passed",
        "make local-oa-release-acceptance for v0.1.0-demo.5 on 2026-09-25: passed",
        "reference-oa-demo go test and go vet on 2026-09-25: passed",
        "Arckit transition validation/dry-run and scoped git diff --check on 2026-09-25: passed"
      ],
      "runtime_result_ref": "",
      "occurred_at": "2026-09-25T03:09:12.867Z"
    }
  ],
  "case_resolution": {
    "status": "resolved",
    "stage": "resolved",
    "satisfied": [
      "GAP-20260925-001-001"
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
    "updated_at": "2026-09-25T03:09:12.867Z"
  }
}
```

## Round Notes

- Case history is canonical in Structured Record.rounds; keep prose notes exceptional.
