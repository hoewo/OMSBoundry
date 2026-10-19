#!/usr/bin/env python3
"""Export/restore only the synthetic OSS file from the isolated mixed fixture.

Run export first, carry bytes off ECS, verify SHA, return a separate copy, then
run restore. Explicit removal applies only to that exact synthetic object's key.
This is an acceptance drill, not a general or scheduled production backup tool.
"""
import hashlib
import json
import os
import re
import subprocess
import sys


def command(args, env=None):
    result = subprocess.run(args, text=True, env=env, capture_output=True, timeout=1100)
    if result.returncode:
        # Go tests deliberately suppress SDK diagnostics; inspect still stays private.
        raise RuntimeError("isolated recovery command failed; secrets suppressed")
    return result.stdout.strip()


def sql(database, query):
    return command(["docker", "exec", "omsbizfoundry-postgres-1", "psql", "-v", "ON_ERROR_STOP=1", "-U", "oms_admin", "-d", database, "-Atc", query])


def facts(database):
    return sql(database, "SELECT row_to_json(v)::text FROM file_versions v ORDER BY tenant_id,file_id,version;")


phase = os.environ["RECOVERY_PHASE"]
database = os.environ["PROBE_DATABASE"]
source = os.environ["PROBE_SOURCE_DATABASE"]
for name in (database, source):
    if not re.fullmatch(r"evidence_oss_[a-z0-9_]{1,40}", name):
        raise SystemExit("reserved isolated fixture required")
if phase not in ("export", "restore", "verify", "expiry") or database == source:
    raise SystemExit("explicit phase and distinct restored fixture required")
container = json.loads(command(["docker", "inspect", database]))[0]
assert container["Config"]["Labels"]["oms.purpose"] == "evidence-oss-acceptance"
assert not container["HostConfig"]["PortBindings"]
before = facts(database)
assert before == facts(source) and len(before.splitlines()) == 12
main_before = facts("evidence_service")
query = """SELECT jsonb_build_object('file_name',v.file_name,'file_id',v.file_id,
 'session_id',s.id,'version',v.version,'size_bytes',v.size_bytes,'sha256',v.sha256,
 'object',jsonb_build_object('TenantID',v.tenant_id,'Provider',v.provider,
 'Bucket',v.bucket,'ObjectKey',v.object_key)) FROM file_versions v
 JOIN upload_sessions s ON s.tenant_id=v.tenant_id AND s.file_id=v.file_id AND s.version=v.version
 WHERE v.provider='aliyun-oss' AND v.file_name='auth58-transition-cloud.txt';"""
rows = sql(database, query).splitlines()
assert len(rows) == 1, "exact synthetic file required"
record = json.loads(rows[0])
assert record["sha256"] == hashlib.sha256(b"Auth58 isolated OSS transition bytes\n" * 128).hexdigest()
assert record["size_bytes"] == 4736 and record["version"] == 1
blob = "/backups/oss-fixture-export.bin" if phase == "export" else "/backups/oss-fixture-returned.bin"
runtime = {**os.environ, "RECOVERY_PHASE": phase, "RECOVERY_RECORD": rows[0], "RECOVERY_BLOB": blob,
           "ALIBABA_CLOUD_ECS_IMDSV2_ENABLE": "true"}
if phase == "restore":
    assert os.environ.get("RECOVERY_ALLOW_REMOVE_ISOLATED") == "yes", "explicit isolated deletion confirmation required"
started_here = not container["State"]["Running"]
try:
    if started_here:
        command(["docker", "start", database])
    args = ["docker", "exec", "--env", "RECOVERY_PHASE", "--env", "RECOVERY_RECORD", "--env", "RECOVERY_BLOB",
            "--env", "ALIBABA_CLOUD_ECS_IMDSV2_ENABLE"]
    if phase == "restore":
        args += ["--env", "RECOVERY_ALLOW_REMOVE_ISOLATED"]
    args += [database, "/tmp/object-recovery.test", "-test.v", "-test.timeout", "20m", "-test.run", "^TestIsolatedOSSObjectOffHostRecovery$"]
    if phase == "expiry":
        print(json.dumps({"phase": phase, "state": "waiting_for_real_900_second_expiry", "clock_injected": False}), flush=True)
    output = command(args, env=runtime)
    assert "PASS" in output and "SKIP" not in output
    if phase == "restore":
        assert "not found after removal" in output
    if phase == "expiry":
        assert "403 SecurityTokenExpired" in output
    assert facts(database) == before == facts(source) and facts("evidence_service") == main_before
    print(json.dumps({"result": "pass", "phase": phase, "file_id": record["file_id"], "version": 1,
                      "bytes": record["size_bytes"], "sha256": record["sha256"], "facts_unchanged": True,
                      "scope": "exact synthetic object only; not scheduled production backup or disaster recovery"}), flush=True)
except Exception as error:
    print(json.dumps({"result": "failed", "reason": str(error) if isinstance(error, RuntimeError) else type(error).__name__}), flush=True)
    sys.exit(1)
finally:
    if started_here:
        command(["docker", "stop", database])
