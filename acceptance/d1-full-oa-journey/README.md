# D1 Full OA Continuous Journey

This acceptance target proves C1 through C8 against one Tenant and one Project. It is intentionally separate from the existing per-capability acceptance suites: those suites remain the detailed authorization and failure evidence, while this target proves business continuity across their boundaries.

The machine-readable input is `scenario-v1.json`; run its static gate before creating any remote fixture:

```bash
acceptance/d1-full-oa-journey/verify-scenario
```

Runtime automation is added phase by phase and must preserve the state artifact after every phase. Until C1 through C8 have all executed on the same identifiers, the overall result remains `insufficient-evidence`.

Run the read-only C0 baseline against an already-running full D1 composition before creating the shared fixture:

```bash
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c0 \
acceptance/d1-full-oa-journey/verify-c0
```

The C0 gate requires all twelve domain applications, exactly seventeen healthy long-running services, current schemas and valid contracts, a ready public edge, and Nginx as the only host-port publisher. It records the Suite Manifest, rendered Compose configuration, image identities, health inventory, and check-log digest without copying environment secrets.

After C0, create the one shared Party/People/Project fixture used by every later phase:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c1 \
acceptance/d1-full-oa-journey/verify-c1
```

C1 saves stable IDs and revisions in `state-after-c1.json`. Validation-only actor passwords are separated into `state-private-c1.json` with mode `0600`; reports and source control must never copy that private file.

Continue with C2 against the exact C1 state and the same isolated D1 stack:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_DIR=/absolute/path/to/evidence/c1 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c2 \
acceptance/d1-full-oa-journey/verify-c2
```

C2 adds one supplier Party, two immutable Evidence files, one directory-backed Contract and Settlement, and one ready Business Engagement to the same Project. It deliberately stops only Evidence Service, proves the failed Business Engagement command leaves zero new facts, restarts Evidence, and replays the same command to exactly one commit. The final read goes through Project → Business Engagement → Evidence with request-bound workload credentials; it must return the exact two material projections and expose no bucket or object key.

The verifier writes `state-after-c2.json`, `c2-summary.md`, `http-evidence.tsv`, ownership before/after/delta files, Contract Party mapping evidence, and Business Engagement history. `state-partial-c2.json` and `state-partial-c2-after-engagement.json` are bounded checkpoints: resume is allowed only after the script revalidates their immutable IDs, commands, revisions and live server facts. A destructive outage is never injected twice for one checkpoint.

`OA_JOURNEY_SKIP_BOOTSTRAP_GRANTS=true` is only for an already retained and independently verified IAM grant fixture. It skips re-running the grant bootstrap; login and every application authorization are still executed. It must not be used to bypass a missing or unknown authorization state.

Continue C3 from the exact passing C2 state:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_DIR=/absolute/path/to/evidence/c1 \
OA_JOURNEY_C2_DIR=/absolute/path/to/evidence/c2 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c3 \
acceptance/d1-full-oa-journey/verify-c3
```

C3 appends an invoice correction, confirms a receivable, receipt, allocation and Receipt Set, and links only their stable identities to the existing Project. Before Money facts are created, the verifier explicitly reviews every already-referenced local Money Party, maps the two journey snapshots to the exact C1/C2 Directory Parties, and cuts reference authority over to Party Directory. It does not infer identity from a name or registration identifier.

The negative path uses the retained C1 participant account with `billing-invoice.read` but without `billing-invoice.amount.read`; the current and historical amount fields must be `null`, never a fabricated zero. The fault path stops only Billing, proves only the invoice segment degrades, then restores the same container and requires 17/17 healthy services. Plan, Party-cutover and fact checkpoints make command identities stable across an interrupted driver run; completed fault evidence is not injected again.

Continue C4 from the exact passing C3 state:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_DIR=/absolute/path/to/evidence/c1 \
OA_JOURNEY_C3_DIR=/absolute/path/to/evidence/c3 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c4 \
acceptance/d1-full-oa-journey/verify-c4
```

C4 adds one `design-v1` and one `consultation-v1` Production work item to the same retained Project. It proves immutable Evidence versions, type-specific stages, exact Project Assignment ownership, cross-type artifact rejection, stale-revision rejection, explicit artifact replacement, and reopen/recomplete history. Its fault path stops only Evidence Service: the report command must write nothing while the dependency is unavailable, then the exact same command identity must commit once after recovery. Project Workspace and People & Groups remain read-only throughout this phase, and the isolated stack must finish 17/17 healthy.

Continue C5 from the exact passing C4 state while reusing the C2 Contract and award Evidence:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_DIR=/absolute/path/to/evidence/c1 \
OA_JOURNEY_C2_DIR=/absolute/path/to/evidence/c2 \
OA_JOURNEY_C4_DIR=/absolute/path/to/evidence/c4 \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c5 \
acceptance/d1-full-oa-journey/verify-c5
```

C5 records one approval and one audit result against the retained Project, then lets Project Workspace write one explicit completion checkpoint. The audit freezes approval revision 1; a later append-only approval correction advances the Project reference to revision 11 without rewriting the audit and must make the confirmed source set visibly drifted. The phase proves amount redaction is omission rather than fabricated zero, capability revocation returns 403, exact workload credentials are resource-scoped, Evidence and Contract outages produce zero Results writes before same-command recovery, and a Results outage permits only local replay of the already successful Project command while a new confirmation fails closed. Contract, Evidence, People & Groups and Production facts remain unchanged, and the isolated stack must finish 17/17 healthy.

Continue C6 from the exact passing C5 state while reusing the C1 customer Party, C2 supplier Party, active Contract and award Evidence:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_STATE=/absolute/path/to/evidence/c1/state-after-c1.json \
OA_JOURNEY_C1_PRIVATE_STATE=/absolute/path/to/evidence/c1/state-private-c1.json \
OA_JOURNEY_C2_STATE=/absolute/path/to/evidence/c2/state-after-c2.json \
OA_JOURNEY_C4_STATE=/absolute/path/to/evidence/c4/state-after-c4.json \
OA_JOURNEY_C5_STATE=/absolute/path/to/evidence/c5/state-after-c5.json \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c6 \
acceptance/d1-full-oa-journey/verify-c6
```

C6 records one direct Cost and one Outsourcing Engagement on the retained Project. The Engagement models customer-advanced field-material work so the phase can reuse the already migrated C1/C2 Party pair rather than invent a post-cutover Party mapping. Evidence and Party outages must write no C6 fact; the same commands commit once after recovery. A Cost correction appends revision 2, stale Cost and Assessment commands and a neighboring Project are rejected. C6 submits a versioned payable suggestion, Money independently accepts it and owns the resulting obligation, while C6 only synchronizes the decision. A Money outage may not advance the C6 suggestion, and recovery plus replay must not duplicate either suggestion or obligation. Project, Party, Contract and Evidence facts remain unchanged, exact workload credentials are resource-scoped, and the isolated stack must finish 17/17 healthy.

Continue C7 from the same passing C6 state. It reuses the C2 Settlement, the retained C3 Receipt Set and the C1 People assignments; it must not create another Project:

```bash
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_STATE=/absolute/path/to/evidence/c1/state-after-c1.json \
OA_JOURNEY_C1_PRIVATE_STATE=/absolute/path/to/evidence/c1/state-private-c1.json \
OA_JOURNEY_C2_STATE=/absolute/path/to/evidence/c2/state-after-c2.json \
OA_JOURNEY_C5_STATE=/absolute/path/to/evidence/c5/state-after-c5.json \
OA_JOURNEY_C6_STATE=/absolute/path/to/evidence/c6/state-after-c6.json \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c7 \
acceptance/d1-full-oa-journey/verify-c7
```

C7 freezes the two source revisions and the versioned `reference-oa-bonus-v1` policy, computes integer allocations, submits only a payable suggestion, and leaves Money to accept and own the obligation. The verifier proves a target outage is recoverable, replays do not duplicate facts, a later Bonus withdrawal does not rewrite the accepted Money obligation, and all workload credentials stay resource-scoped.

Finally run C8 as a bounded read/fault probe. This phase deliberately exits non-zero with `insufficient-evidence` when any assembly gate is missing; the non-zero result and its artifacts are evidence and must not be hidden by creating a new Project:

```bash
OA_JOURNEY_C8_CONFIRM=yes \
D1_DIR=/absolute/path/to/release/deployments/d1 \
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-journey \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
OA_JOURNEY_C1_STATE=/absolute/path/to/evidence/c1/state-after-c1.json \
OA_JOURNEY_C1_PRIVATE_STATE=/absolute/path/to/evidence/c1/state-private-c1.json \
OA_JOURNEY_C2_STATE=/absolute/path/to/evidence/c2/state-after-c2.json \
OA_JOURNEY_C4_STATE=/absolute/path/to/evidence/c4/state-after-c4.json \
OA_JOURNEY_C5_STATE=/absolute/path/to/evidence/c5/state-after-c5.json \
OA_JOURNEY_C6_STATE=/absolute/path/to/evidence/c6/state-after-c6.json \
OA_JOURNEY_C7_STATE=/absolute/path/to/evidence/c7/state-after-c7.json \
OA_JOURNEY_EVIDENCE_DIR=/absolute/path/to/evidence/c8 \
acceptance/d1-full-oa-journey/verify-c8
```

C8 compares canonical rows in all twelve business databases before and after read-only composition, stops only one source at a time, verifies recovery and `17/17 healthy`, checks same-Subject work items versus a neighboring Subject, and proves Money values return to `null/redacted` after amount revocation. Project materials must cover active C2–C6 links and Evidence download must be authorized separately from metadata; either failure keeps the continuous C1–C8 result at `insufficient-evidence`.

The current C8 driver requires the BF-082 native entry contract. It asserts the exact signed File scope and action set for metadata-only, download-only and combined credentials, with no legacy `evidence-service.view` grant. Metadata-only must read metadata and receive download 403; download-only must request an access grant and receive metadata 403. The old 2026-09-30 runtime evidence remains a historical failed gate, not a passing rerun. This source change does not authorize restarting a stopped restore stack. Its offline assertion test is `python3 acceptance/d1-full-oa-journey/test-c8-native-entry.py`; the full runtime sequence must still be executed separately in an authorized isolated stack.
