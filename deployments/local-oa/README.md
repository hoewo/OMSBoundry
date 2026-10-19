# Local OA business-validation composition

This composition runs the six business applications required by the reference OA Demo without Nginx, Gateway or central IAM:

- Contract Ledger: `http://127.0.0.1:18081`
- Money Flow Ledger: `http://127.0.0.1:18083`
- People & Groups: `http://127.0.0.1:18084`
- Evidence Service: `http://127.0.0.1:18085`
- Project Workspace: `http://127.0.0.1:18086`
- Design Operations: `http://127.0.0.1:18087`

Each application owns a separate PostgreSQL service and volume. The services share only the `app` network for versioned HTTP references. Contract and Money Flow use their supported local account provider; the other applications use the authenticated template provider. There is no `AUTH_DISABLED` path.

## Start and verify

```sh
./deployments/local-oa/ops/local-oa up
./deployments/local-oa/ops/local-oa status
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa failure-isolation
./deployments/local-oa/ops/local-oa composition-audit
./deployments/local-oa/ops/local-oa composition-audit-commercial
./deployments/local-oa/ops/local-oa composition-audit-consultation
```

The first command creates `deployments/local-oa/.env` with local-only random credentials, builds the application images, waits for all health checks, and runs the readiness verifier. The `1808x` host ports intentionally avoid colliding with individually started applications on their usual `808x` ports. Show the two browser-login credentials with:

```sh
./deployments/local-oa/ops/local-oa credentials
```

`TEMPLATE_ACCESS_TOKEN` is also the Bearer token for the JSON applications. It is a real local authentication credential, not a bypass.

`demo` uses only public application interfaces to create one repeatable fake OA dataset. It creates and changes People／Membership facts while preserving two Roster Snapshots and the original Project Assignment snapshots; evolves a Contract through supplement, correction, termination, settlement and close; explicitly confirms and adjusts a receivable, records two receipts, reverses and replaces a wrong allocation, remits the balance, closes the obligation and reverses one erroneous receipt; uploads three Evidence versions; completes, revises and re-completes the Design stages; then links all five stable references back to the Project. Fixed command IDs exercise each application's idempotency behavior. The local IDs needed to resume an interrupted journey are stored with mode `0600` under the ignored `.runtime/` directory.

`failure-isolation` stops Contract, Money, People, Evidence and Design one at a time. For each outage it proves that Project remains readable, preserves its two immutable responsibility snapshots, marks only the affected live resource references unavailable, and restores the stopped service before moving on. A People outage leaves all five resource links available because Project reads its already-captured Assignment snapshots rather than synchronously reading the personnel directory. Run `demo` first.

`composition-audit` is an isolated gate for selectable assembly. It starts only People, Evidence, Project and Design plus their four owned databases on ports `19084`–`19087`, proves Contract and Money are absent, runs the authenticated J1/J3/J4 subset twice with stable IDs and revisions, compares the state digest, and then removes only this temporary Compose project and its volumes. The normal twelve-container reference OA is neither stopped nor reset. Inside a release bundle it uses only the bundled images and static driver with `--no-build --pull never`. This is synthetic assembly evidence, not a second-customer acceptance result.

`composition-audit-commercial` is the orthogonal isolated gate. It starts only Contract and Money plus their two owned databases on ports `19081` and `19083`, proves People, Evidence, Project and Design are absent, and runs the authenticated J2 ledger journey twice with stable IDs, revisions and state digest. It uses the applications' real local account, session and CSRF paths and performs the same isolated cleanup. It is also synthetic evidence, not a second-customer result.

`composition-audit-consultation` is the C4-A11 minimum-production gate. It starts only People, Evidence, Project and the compatible Production Operations owner plus their four databases on ports `19184`–`19187`; Contract and Money remain absent. From empty state it activates a Project whose only selected type is `consultation`, uploads the required report evidence, completes the canonical `consultation-v1` workflow at revision 5, proves that the legacy Design item and list routes expose none of that consultation data, and replays the same commands with an unchanged state digest. The audit-only Compose override assigns explicit `10.252.20.0/24`–`10.252.26.0/24` subnets so this disposable evidence run does not depend on Docker's exhausted automatic address pool; the normal composition is unchanged.

## Stop, restart and inspect a boundary

```sh
./deployments/local-oa/ops/local-oa stop-service contract-ledger
./deployments/local-oa/ops/local-oa start-service contract-ledger
./deployments/local-oa/ops/local-oa logs project-workspace
```

Only the six application service names are accepted by `stop-service` and `start-service`. This will support the later J4 partial-unavailability demonstration without stopping unrelated databases.

## Stop or reset

```sh
./deployments/local-oa/ops/local-oa down
./deployments/local-oa/ops/local-oa reset --confirm-destroy-data
```

`down` preserves data volumes. `reset` deletes all local Demo databases and evidence files and therefore requires the explicit confirmation argument.

## Scope boundary

Passing `verify` proves only that the isolated application group starts and each database is ready. Passing `demo` twice plus `failure-isolation` proves the high-fidelity local J1-J4 reference journey, idempotent replay, append-only corrections and the five-source outage matrix. Passing the design-delivery and commercial-ledger composition audits additionally proves two orthogonal synthetic middle-granularity assemblies with unselected business systems genuinely absent. Passing the consultation audit plus the strengthened design-only audit proves the local template-authenticated C4-A11 minimum composition and both directions of C4-A12 data/API absence; it does not prove formal IAM issuance, Gateway/Workbench navigation, release-bundle portability, backup/restore or D1. Development bundle `v0.1.0-demo.11` also runs all six Level A products independently from the extracted source-free archive before the earlier subset and full-suite gates, and carries a read-only Ubuntu target-host preflight. None of these results proves the deferred Nginx → Gateway → central IAM deployment path, actual company Ubuntu hardware, a second real customer or an official clean-HEAD release.

## Portable Ubuntu demonstration bundle

Build a target-build-free `linux/amd64` archive on the trusted development machine:

```sh
make local-oa-release VERSION=v0.1.0-demo.11 OUTPUT_DIR=dist
```

The archive contains the six application images, PostgreSQL, two public-contract-only Level A stub images, seven static acceptance drivers, Compose, runtime commands and checksums. The two stubs are used only by the independent Project and Design demos; they are not part of the subset or full OA runtime. The archive contains no source tree, `.env`, runtime state or database data. The target Ubuntu host verifies and loads the same images instead of compiling source:

```sh
sha256sum -c omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz.sha256
tar -xzf omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz
cd omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64
./deployments/local-oa/ops/local-oa preflight
./deployments/local-oa/ops/local-oa verify-release
./deployments/local-oa/ops/local-oa load-images
./deployments/demos/ops/level-a audit --confirm-destroy-demo-data
./deployments/local-oa/ops/local-oa init
./deployments/local-oa/ops/local-oa composition-audit
./deployments/local-oa/ops/local-oa composition-audit-commercial
./deployments/local-oa/ops/local-oa composition-audit-consultation
./deployments/local-oa/ops/local-oa up
./deployments/local-oa/ops/local-oa demo
```

`preflight` is release-bundle-only and read-only. It refuses to proceed when the Ubuntu version or CPU platform is unsupported, the Docker daemon/Compose v2 is unavailable, required tools are missing, disk or memory plus swap is below the documented floor, an acceptance port is busy, or default Demo Compose resources already exist. It does not install packages, start containers, delete data or configure the full entry chain.

The complete transfer, security and cleanup procedure is in `docs/operations/local-oa-ubuntu-demo.md`.
