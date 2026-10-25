# P9 D1 full-composition acceptance

This target is the bounded runtime gate for the OMS1 P9 iteration. It does not start or stop services. `verify-c0` inspects an already-running D1 composition and writes a secret-free evidence bundle before any business fixture is created.

Required runtime state:

- an official D1 release bundle whose four-field `RELEASE-METADATA` matches `SUITE_VERSION`;
- `pull` image mode with every configured image pinned by `name@sha256:<digest>`;
- all fourteen domain applications selected, plus IAM, Workbench, Gateway, Nginx and PostgreSQL;
- exactly nineteen long-running Compose services, all healthy, with Nginx as the only host-port publisher;
- Feedback V2 configured for project 115, `apiKey` mode and notifications enabled;
- a rendered Suite Manifest from the same environment and a reachable public readiness endpoint.

Run:

```sh
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-p9 \
D1_PUBLIC_URL=https://example.test \
D1_RELEASE_METADATA_PATH=/absolute/path/to/RELEASE-METADATA \
D1_SUITE_MANIFEST_PATH=/absolute/path/to/suite-manifest.json \
P9_EVIDENCE_DIR=/absolute/path/to/evidence/c0 \
acceptance/p9-full-composition/verify-c0
```

The raw Compose model and container/image inspections are never persisted because they can contain deployment secrets. Evidence contains only release metadata, topology, image identities/digests, service health, manifest, readiness, check log and a machine-readable C0 state. Every running container's immutable image ID must match the ID currently resolved by its configured image reference; a repointed tag fails C0. Passing C0 is only a precondition; it does not prove the Tenant/member/payroll/reimbursement/Feedback journey or backup/empty-target restore.

C0 writes a fixed artifact set plus `checksums.sha256`; run `verify-evidence EVIDENCE_DIR` whenever the directory is handed off or reused. The verifier rejects links, extra/missing files and digest drift. This detects accidental corruption but is not a publisher signature and does not make an untrusted storage location trustworthy.

After C0 passes, create two mode-`0600` administrator password files outside the repository and run the shared-origin Tenant/People gate:

```sh
P9_C0_EVIDENCE_DIR=/absolute/path/to/evidence/c0 \
P9_C1_EVIDENCE_DIR=/absolute/path/to/evidence/c1 \
P9_PRIMARY_ADMIN_USERNAME=admin-a \
P9_PRIMARY_ADMIN_PASSWORD_FILE=/restricted/admin-a.password \
P9_SECONDARY_TENANT_ID=22222222-2222-4222-8222-222222222222 \
P9_SECONDARY_TENANT_NAME='Tenant B' \
P9_SECONDARY_ADMIN_USERNAME=admin-b \
P9_SECONDARY_ADMIN_DISPLAY_NAME='Tenant B Administrator' \
P9_SECONDARY_ADMIN_PASSWORD_FILE=/restricted/admin-b.password \
acceptance/p9-full-composition/verify-c1-multitenant
```

For a private CA, set `P9_CURL_CA_BUNDLE` to its PEM bundle; the gate never permits insecure TLS. C1 re-verifies the exact C0 artifact set, discovers the already-running IAM container by the C0 Compose project, and provisions Tenant B only when its public login returns 401. Password bytes travel through protected files/stdin, never process arguments or evidence. Stable Person command IDs make an interrupted rerun safe; existing People grants may return the precise 409 conflict and continue. Both administrators then enter through the same public origin, obtain their own People capability, create/replay one linked Person, see only their Tenant rows, receive 404 for the other Tenant's Person ID, and render the current session Tenant in Workbench. C1 does not start or stop services. Validate a handed-off artifact with `verify-c1-evidence EVIDENCE_DIR`.

After the continuous fixture has been created, `d1 backup` must produce both `backup-consistency.env` and `database-fingerprints.tsv`. The latter contains only database names, table/row counts and SHA-256 values; raw rows and sequence values never enter the artifact. Its schema/fact digests cover public sequence definitions and `last_value`/`is_called` as well as tables. Before deleting target volumes, `d1 restore` validates its fixed format and exact database selection; it then recomputes the same fingerprints before starting business services and fails closed on any mismatch. A passing fingerprint comparison proves database checkpoint equality only; the P9 runtime window must still verify Evidence bytes and repeat key reads and one bounded continuation write after restore.
