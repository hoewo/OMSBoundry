# D1 C2 Business Engagement authorization acceptance

This acceptance drives an already-running isolated D1 stack only through its
public Nginx endpoint. It proves the first C2 role, Project scope, target state,
revocation, and Project/Evidence workload-identity matrix without bypassing
Gateway or IAM.

Required D1 selection:

- People & Groups;
- Project Workspace;
- Evidence Service;
- Business Engagement;
- IAM, Gateway, Workbench, Nginx, and PostgreSQL.

Run:

```bash
D1_ENV_FILE=/absolute/path/to/d1.env \
D1_PROJECT_NAME=isolated-compose-project \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
C2_ENABLE_DEPENDENCY_FAILURE_TEST=true \
C2_ENABLE_IAM_BOUNDARY_TEST=true \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

The script creates acceptance accounts and business records, changes grants and
Assignments, and writes HTTP evidence to a temporary directory printed by the
run. Use it only against an isolated acceptance environment. The script alone
does not prove browser interaction, empty-target restore, or a real
Ubuntu/public deployment. With `C2_ENABLE_IAM_BOUNDARY_TEST=true` it also proves
that account disablement invalidates the Suite Session, IAM and Project-policy
outages fail closed on explicit refresh, failed Policy evaluation writes no
issuance audit, and recovery issues exactly one new credential. Previously
issued application credentials remain offline-verifiable only inside their
signed maximum 300-second lifetime. Cross-tenant rejection is covered by the
target verifier test with a valid signature and a mismatched tenant. Empty-target
restore is covered separately by the recovery experiment below.

The accepted evidence and limitations are recorded in
[`docs/reports/oa-c2-business-engagement-authorization-experiment-20260927.md`](../../docs/reports/oa-c2-business-engagement-authorization-experiment-20260927.md).
The destructive recovery evidence is recorded in
[`docs/reports/oa-c2-business-engagement-recovery-experiment-20260927.md`](../../docs/reports/oa-c2-business-engagement-recovery-experiment-20260927.md).
