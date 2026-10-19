#!/usr/bin/env python3
"""Source-only real PG18/IAM checks; no main upgrade or restore20 restart.

All new workloads live on their own internal bridge without published ports.
Containers, network, diagnostics and the isolated PG data volume are retained,
and both running containers are always stopped. Synthetic per-test databases
are created/dropped by the application's existing integration-test helper.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time

ROOT = Path("/opt/omsbizfoundry/acceptance/production-material-source-20261001")
NAME = "production_material_source_auth58_20261001a"
PG = NAME + "_pg"
NETWORK = NAME + "_isolated"
VOLUME = NAME + "_pgdata"
WORK = ROOT / NAME
BASE = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/"
RUNTIME = BASE + "base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66"
IAM = BASE + "identity-access@sha256:69806b6fbf055dcdbb50dcc93a135149e0a523684213ef4607bcc1c21a006266"

spec = importlib.util.spec_from_file_location("entry_checks", "/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/verify-entry-capabilities.py")
entry = importlib.util.module_from_spec(spec)
spec.loader.exec_module(entry)
run, inspect, save, snapshot = entry.run, entry.inspect, entry.save, entry.snapshot
events = []


def mark(check, **metadata):
    event = {"check": check, "result": "pass", **metadata}
    events.append(event)
    print(json.dumps(event), flush=True)


def main():
    assert os.geteuid() == 0 and not WORK.exists()
    for binary in ("production-material-domain.test", "production-material-http.test"):
        assert (ROOT / binary).is_file()
    for image in (IAM, RUNTIME):
        meta = json.loads(run(["docker", "image", "inspect", image]))[0]
        assert image in meta["RepoDigests"] and meta["Architecture"] == "amd64"
    before = snapshot()
    assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
    WORK.mkdir(mode=0o700)
    save(WORK / "main-before.json", json.dumps(before, indent=2).encode())
    for subdir in ("config", "bin", "data"):
        (WORK / subdir).mkdir(mode=0o700)
    for source, target in (("production-material-domain.test", "domain.test"), ("production-material-http.test", "http.test")):
        save(WORK / "bin" / target, (ROOT / source).read_bytes(), 0o500)
    source = NAME + "_source_iam"
    run(["docker", "create", "--name", source, "--label", "oms.purpose=production-material-source", IAM])
    run(["docker", "cp", source + ":/app/server", str(WORK / "bin/iam")])
    os.chmod(WORK / "bin/iam", 0o500)
    save(WORK / "config/production-capabilities.json", (ROOT / "production-capabilities.json").read_bytes())
    password = secrets.token_hex(24)
    iam_password = secrets.token_hex(24)
    admin_password = secrets.token_hex(24)
    save(WORK / "config/pg.env", ("POSTGRES_USER=probe_admin\nPOSTGRES_PASSWORD=" + password + "\nPOSTGRES_DB=postgres\n").encode())
    config = {
        "TEST_DATABASE_URL": "postgres://probe_admin:" + password + "@" + PG + ":5432/postgres?sslmode=disable",
        "IAM_DATABASE_URL": "postgres://iam_owner:" + iam_password + "@" + PG + ":5432/production_material_iam?sslmode=disable",
        "PRODUCTION_MATERIAL_IAM_URL": "http://127.0.0.1:8080",
        "PRODUCTION_MATERIAL_IAM_ISSUER": "https://production-material-fixture.example.test/auth",
        "PRODUCTION_MATERIAL_IAM_KEYS": "/probe/data/jwks.json",
        "PRODUCTION_MATERIAL_IAM_PASSWORD": admin_password,
    }
    save(WORK / "config/fixture.env", "".join(key + "='" + value + "'\n" for key, value in config.items()).encode())
    hashes = {binary.name: hashlib.sha256(binary.read_bytes()).hexdigest() for binary in (WORK / "bin").iterdir()}
    save(WORK / "binaries.json", json.dumps({"sha256": hashes, "iam_image": IAM, "runtime_image": RUNTIME}, indent=2).encode())
    run(["chown", "-R", "70:70", str(WORK)])
    pg_started = runner_started = False
    succeeded = False
    try:
        run(["docker", "network", "create", "--internal", "--label", "oms.purpose=production-material-source", NETWORK])
        run(["docker", "volume", "create", "--label", "oms.purpose=production-material-source", VOLUME])
        run(["docker", "run", "--detach", "--name", PG, "--network", NETWORK, "--memory", "192m", "--cpus", "0.20", "--pids-limit", "80", "--env-file", str(WORK / "config/pg.env"), "--label", "oms.purpose=production-material-source", "--mount", "type=volume,src=" + VOLUME + ",dst=/var/lib/postgresql", RUNTIME, "postgres", "-c", "shared_buffers=24MB", "-c", "max_connections=24"])
        pg_started = True
        for attempt in range(45):
            status = subprocess.run(["docker", "exec", PG, "pg_isready", "-U", "probe_admin"], capture_output=True)
            if status.returncode == 0:
                break
            assert attempt != 44, "private PostgreSQL did not become ready"
            time.sleep(1)
        statement = "CREATE ROLE iam_owner LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + iam_password + "'; CREATE DATABASE production_material_iam OWNER iam_owner;"
        run(["docker", "exec", "-i", PG, "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1", "-U", "probe_admin", "-d", "postgres"], statement.encode())
        run(["docker", "run", "--detach", "--name", NAME, "--network", NETWORK, "--memory", "224m", "--cpus", "0.35", "--pids-limit", "80", "--user", "70:70", "--label", "oms.purpose=production-material-source", "--mount", "type=bind,src=" + str(WORK / "bin") + ",dst=/probe/bin,readonly", "--mount", "type=bind,src=" + str(WORK / "config") + ",dst=/probe/config,readonly", "--mount", "type=bind,src=" + str(WORK / "data") + ",dst=/probe/data", "--mount", "type=bind,src=" + str(ROOT / "probe.sh") + ",dst=/probe/start.sh,readonly", "--entrypoint", "sh", RUNTIME, "/probe/start.sh"])
        runner_started = True
        info = inspect(NAME)
        net = json.loads(run(["docker", "network", "inspect", NETWORK]))[0]
        assert net["Internal"] and not info["HostConfig"]["PortBindings"] and info["HostConfig"]["NanoCpus"] == 350000000
        mark("isolated_pg18_and_real_iam_started", published_ports=0, max_total_cpu=0.55, memory_limits_mb=416)
        deadline = time.monotonic() + 290
        while inspect(NAME)["State"]["Running"]:
            assert time.monotonic() < deadline, "bounded source validation timed out"
            time.sleep(2)
        info = inspect(NAME)
        log = run(["docker", "logs", NAME])
        save(WORK / "test.log", log.encode())
        assert info["State"]["ExitCode"] == 0 and not info["State"]["OOMKilled"], "source integration failed; inspect private test.log"
        required = ("TestMaterialReferencePostgresCurrentVersionsAndZeroWrites", "TestMaterialReferencePostgresSnapshotSurvivesConcurrentReplacement", "TestProductionMaterialHTTPPostgresNativeIAM")
        assert all("--- PASS: " + test in log for test in required) and "--- SKIP" not in log
        mark("real_pg18_and_real_iam_native_source_tests", tests=list(required), owned_read_digest_unchanged=True, concurrent_revision_and_version_coherent=True)
        assert snapshot() == before
        assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
        mark("main_thirteen_databases_and_seventeen_container_ids_unchanged", restore20_running=0)
        succeeded = True
    finally:
        for started, name in ((runner_started, NAME), (pg_started, PG)):
            if started:
                run(["docker", "stop", "--time", "15", name])
                assert not inspect(name)["State"]["Running"]
        mark("source_fixture_stopped_data_retained", removed_containers=0, removed_volumes=0)
        save(WORK / "results.json", json.dumps({"passed": succeeded, "events": events}, indent=2).encode())


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit("source validation failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno) + "; private diagnostics retained")
