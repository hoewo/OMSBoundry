# P9 D1 full-composition acceptance

This target is the bounded runtime gate for the OMS1 P9 iteration. It does not start or stop services. `verify-c0` inspects an already-running D1 composition and writes a secret-free evidence bundle before any business fixture is created.

Required runtime state:

- all fourteen domain applications selected, plus IAM, Workbench, Gateway, Nginx and PostgreSQL;
- exactly nineteen long-running Compose services, all healthy, with Nginx as the only host-port publisher;
- Feedback V2 configured for project 115, `apiKey` mode and notifications enabled;
- a rendered Suite Manifest from the same environment and a reachable public readiness endpoint.

Run:

```sh
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry-p9 \
D1_PUBLIC_URL=https://example.test \
D1_SUITE_MANIFEST_PATH=/absolute/path/to/suite-manifest.json \
P9_EVIDENCE_DIR=/absolute/path/to/evidence/c0 \
acceptance/p9-full-composition/verify-c0
```

The raw Compose model and container/image inspections are never persisted because they can contain deployment secrets. Evidence contains only topology, image identities/digests, service health, manifest, readiness, check log and a machine-readable C0 state. Passing C0 is only a precondition; it does not prove the Tenant/member/payroll/reimbursement/Feedback journey or backup/empty-target restore.
