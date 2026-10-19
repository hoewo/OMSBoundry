#!/usr/bin/env python3
"""Run cross-compiled backup integration tests on ECS PostgreSQL, not on Mac Docker.

The test executable creates/drops ONLY its own random databases. Cloud storage is
a fixture in this test; live SDK/OSS acceptance is a separate required gate.
"""
import hashlib
import json
import os
import re
import subprocess
import sys
import urllib.parse


def command(args, env=None):
    result = subprocess.run(args, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=240)
    if result.returncode:
        raise RuntimeError("command failed; sensitive diagnostics suppressed")
    return result.stdout.strip()


def inspect(name):
    return json.loads(command(["docker", "inspect", name]))[0]


def snapshot():
    return command(["docker", "exec", "omsbizfoundry-postgres-1", "psql", "-U", "oms_admin", "-d", "evidence_service", "-Atc",
                    "SELECT md5(string_agg(row_to_json(v)::text,'' ORDER BY tenant_id,file_id,version)) FROM file_versions v;"])


before = snapshot()
try:
    binary = os.environ["PROBE_TEST_BINARY"]
    if not binary.startswith("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/") or not os.path.isfile(binary):
        raise RuntimeError("explicit acceptance executable required")
    postgres = inspect("omsbizfoundry-postgres-1")
    evidence = inspect("omsbizfoundry-evidence-service-1")
    env = dict(item.split("=", 1) for item in postgres["Config"]["Env"])
    network = next(name for name in evidence["NetworkSettings"]["Networks"] if name.endswith("_data"))
    test_url = "postgres://oms_admin:" + urllib.parse.quote(env["POSTGRES_PASSWORD"], safe="") + "@postgres:5432/postgres?sslmode=disable"
    args = ["docker", "run", "--rm", "--name", "evidence_oss_backup_tests_20261001", "--network", network,
            "--memory", "192m", "--cpus", "0.5", "--label", "oms.purpose=evidence-oss-acceptance",
            "--mount", "type=bind,src=" + binary + ",dst=/acceptance/systembackup.test,readonly",
            "--env", "TEST_DATABASE_URL", "--env", "TEST_PG_DUMP_BIN=/usr/local/bin/pg_dump",
            "--env", "TEST_PG_RESTORE_BIN=/usr/local/bin/pg_restore", "--env", "GOMAXPROCS=2",
            "--entrypoint", "/acceptance/systembackup.test", evidence["Image"], "-test.v",
            "-test.run", "TestFullBackupRestore|TestMixedBackupRestore", "-test.timeout", "180s", "-test.parallel", "1"]
    result = subprocess.run(args, env={**os.environ, "TEST_DATABASE_URL": test_url}, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=240)
    for line in result.stdout.splitlines():
        if re.fullmatch(r"(?:=== RUN   Test[A-Za-z0-9]+|--- PASS: Test[A-Za-z0-9]+ \([0-9.]+s\)|PASS)", line):
            print(line, flush=True)
    if result.returncode:
        raise RuntimeError("PostgreSQL backup integration failed; diagnostics suppressed")
    assert snapshot() == before
    with open(binary, "rb") as artifact:
        hasher = hashlib.sha256()
        for block in iter(lambda: artifact.read(1024 * 1024), b""):
            hasher.update(block)
        digest = hasher.hexdigest()
    print(json.dumps({"result": "pass", "scope": "real PostgreSQL local-v1 and mixed-v2 cold backup/empty restore",
                      "cloud_storage": "test fixture, NOT live OSS evidence", "main_versions_unchanged": True,
                      "test_binary_sha256": digest, "image": evidence["Image"]}), flush=True)
except Exception as error:
    print(json.dumps({"result": "failed", "reason": str(error) if isinstance(error, RuntimeError) else type(error).__name__}), flush=True)
    sys.exit(1)
finally:
    if snapshot() != before:
        print(json.dumps({"result": "failed", "reason": "main evidence changed during isolated backup tests"}), flush=True)
        sys.exit(1)
