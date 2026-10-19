# D1 Release Bundle

This directory is a pull-only D1 deployment description. It runs immutable
`linux/amd64` images already published to a private Registry and never compiles
application source on the target host.

The bundle always includes digest-pinned PostgreSQL, IAM, Workbench, Gateway
and Nginx images plus every application image explicitly present in its image
manifest. Applications omitted from that manifest must remain disabled; they
can only be enabled by promoting a new bundle that contains their verified
digest. Database migrations, Suite Manifest switches, workload identities,
gateway routes, and backup/restore material remain part of the bundle.

The manifest must also be a compatibility closure, not merely a list of images
whose source files changed. A new Capability/Suite Manifest, policy or public
schema must include every runtime parser that consumes it, and a forward
migration must include the server and ops binaries that recognize the resulting
schema. After a migration runs, restarting an older binary is not a rollback;
restore the pre-upgrade backup into a fresh target instead. See ADR-0035.

Backups record the running Project personnel and Bonus profiles alongside the
application switches, not the candidate environment's new selections. Restore
rejects mismatched, unknown or duplicate profile declarations before replacing
any volumes. Pre-profile backups are accepted only with both profiles disabled;
absence is not permission to infer a customer overlay version. These guards do
not replace an actual isolated joint-restore rehearsal for each release closure.

1. Verify the outer archive checksum before extracting it.
   Do not import the build machine's user/group ownership. When extracting as
   root, use `tar --no-same-owner`; keep a privileged deployment directory and
   its parent root-owned and not writable by untrusted users.
2. Verify `RELEASE-CHECKSUMS.sha256` from the extracted bundle root.
3. Copy `deployments/d1/.env.example` to a separate protected `.env` file,
   replace every environment-specific value and keep `D1_IMAGE_MODE=pull`.
4. Provision the target host, Registry access, DNS, certificate, secrets and
   backup destination independently for that environment.
5. For a new empty target, run `deployments/d1/ops/d1 config --quiet`, `images`,
   `prepare-images`, `up` and `check`, then execute the acceptance entry under
   `acceptance/d1-delivery`.
6. For an existing target, run `preflight-upgrade` first. It executes the
   candidate IAM parser against every bundled Capability Manifest, assignment
   policy and workload policy without opening the database. Only after it
   succeeds may `upgrade` create the pre-mutation backup and run migrations;
   `upgrade` repeats this preflight automatically.

The bundle contains no `.env`, certificate, private key, Registry password,
AccessKey, database data or backup. See `docs/operations/d1-compact.md` and
`docs/operations/d1-environment-promotion.md` for the complete operator flow.

The optional Evidence OSS profile keeps app/data networks internal and requires
a pre-provisioned, IMDS-guarded dedicated network. `d1 check` also rejects a
running Evidence mode/Primary that differs from the selected profile. Docker
startup-hook templates are included but never installed automatically; real
daemon/host restart acceptance remains mandatory. See
`docs/operations/d1-evidence-oss-profile.md` before enabling that profile.
