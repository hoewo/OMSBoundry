#!/usr/bin/env python3
"""C6 owned source-only PG18/real IAM check, not a Main deployment or C8.

Reuse only the stopped, labelled private C4 PostgreSQL. New IAM owns a new
restricted role/database; its keys, data and diagnostics are retained. Stop
the two workloads on success or failure; never start restore20 or Main apps.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shlex
import subprocess
import time

ROOT = Path("/opt/omsbizfoundry/acceptance/cost-material-source-20261001")
NAME = "cost_material_source_auth58_20261001a"
WORK = ROOT / NAME
SOURCE = Path("/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a")
PG = "production_material_source_auth58_20261001a_pg"
NETWORK = "production_material_source_auth58_20261001a_isolated"
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
    pg = inspect(PG)
    assert not pg["State"]["Running"] and not pg["HostConfig"]["PortBindings"]
    assert pg["Config"]["Labels"]["oms.purpose"] == "production-material-source"
    assert pg["Config"]["Image"] == RUNTIME and pg["HostConfig"]["NanoCpus"] == 200000000
    assert set(pg["NetworkSettings"]["Networks"]) == {NETWORK}
    assert json.loads(run(["docker", "network", "inspect", NETWORK]))[0]["Internal"]
    assert not inspect("production_material_source_auth58_20261001a")["State"]["Running"]
    assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
    source_hashes = json.loads((SOURCE / "binaries.json").read_text())
    assert source_hashes["iam_image"] == IAM
    assert hashlib.sha256((SOURCE / "bin/iam").read_bytes()).hexdigest() == source_hashes["sha256"]["iam"]
    before = snapshot()
    WORK.mkdir(mode=0o700)
    for subdir in ("bin", "config", "data"):
        (WORK / subdir).mkdir(mode=0o700)
    save(WORK / "main-before.json", json.dumps(before, indent=2).encode())
    for source, target in (("cost-material-domain.test", "domain.test"), ("cost-material-http.test", "http.test")):
        save(WORK / "bin" / target, (ROOT / source).read_bytes(), 0o500)
    save(WORK / "bin/iam", (SOURCE / "bin/iam").read_bytes(), 0o500)
    save(WORK / "config/cost-capabilities.json", (ROOT / "cost-outsourcing-capabilities.json").read_bytes())
    env = dict(line.split("=", 1) for line in (SOURCE / "config/fixture.env").read_text().splitlines())
    database_url = shlex.split(env["TEST_DATABASE_URL"])[0]
    assert "@" + PG + ":5432/postgres?" in database_url
    role, db = "cost_material_iam_owner_a", "cost_material_iam_a"
    iam_password, admin_password = secrets.token_hex(24), secrets.token_hex(24)
    config = {
        "TEST_DATABASE_URL": database_url,
        "IAM_DATABASE_URL": "postgres://" + role + ":" + iam_password + "@" + PG + ":5432/" + db + "?sslmode=disable",
        "COST_MATERIAL_IAM_URL": "http://127.0.0.1:8080",
        "COST_MATERIAL_IAM_ISSUER": "https://cost-material-fixture.example.test/auth",
        "COST_MATERIAL_IAM_KEYS": "/probe/data/jwks.json",
        "COST_MATERIAL_IAM_PASSWORD": admin_password,
    }
    save(WORK / "config/fixture.env", "".join(key + "=" + shlex.quote(value) + "\n" for key, value in config.items()).encode())
    hashes = {binary.name: hashlib.sha256(binary.read_bytes()).hexdigest() for binary in (WORK / "bin").iterdir()}
    save(WORK / "binaries.json", json.dumps({"sha256": hashes, "iam_image": IAM, "runtime_image": RUNTIME}, indent=2).encode())
    run(["chown", "-R", "70:70", str(WORK)])
    pg_started = runner_started = succeeded = False
    try:
        run(["docker", "start", PG]); pg_started = True
        for attempt in range(45):
            if subprocess.run(["docker", "exec", PG, "pg_isready", "-U", "probe_admin"], capture_output=True).returncode == 0:
                break
            assert attempt != 44, "private PG readiness timeout"
            time.sleep(1)
        statement = "CREATE ROLE " + role + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '" + iam_password + "'; CREATE DATABASE " + db + " OWNER " + role + ";"
        run(["docker", "exec", "-i", PG, "psql", "-X", "-q", "-v", "ON_ERROR_STOP=1", "-U", "probe_admin", "-d", "postgres"], statement.encode())
        run(["docker", "run", "--detach", "--name", NAME, "--network", NETWORK, "--memory", "224m", "--cpus", "0.35", "--pids-limit", "80", "--user", "70:70", "--label", "oms.purpose=cost-material-source", "--mount", "type=bind,src=" + str(WORK / "bin") + ",dst=/probe/bin,readonly", "--mount", "type=bind,src=" + str(WORK / "config") + ",dst=/probe/config,readonly", "--mount", "type=bind,src=" + str(WORK / "data") + ",dst=/probe/data", "--mount", "type=bind,src=" + str(ROOT / "probe.sh") + ",dst=/probe/start.sh,readonly", "--entrypoint", "sh", RUNTIME, "/probe/start.sh"])
        runner_started = True
        info = inspect(NAME)
        assert not info["HostConfig"]["PortBindings"] and info["HostConfig"]["NanoCpus"] == 350000000
        mark("private_pg18_and_real_iam_source_runner", published_ports=0, max_total_cpu=0.55, memory_limits_mb=416)
        deadline = time.monotonic() + 290
        while inspect(NAME)["State"]["Running"]:
            assert time.monotonic() < deadline, "bounded source test timeout"
            time.sleep(2)
        info = inspect(NAME)
        log = run(["docker", "logs", NAME]); save(WORK / "test.log", log.encode())
        required = ("TestCostMaterialPostgresBothAggregatesCurrentHistoryAndZeroWrites", "TestCostMaterialHTTPPostgresNativeIAMBothAggregates")
        assert info["State"]["ExitCode"] == 0 and not info["State"]["OOMKilled"], "source test failed; private diagnostics retained"
        assert all("--- PASS: " + test in log for test in required) and "--- SKIP" not in log
        mark("real_pg18_real_iam_cost_material_source", tests=list(required), current_history_preserved=True, owned_read_digest_unchanged=True, amount_read_not_required=True)
        assert snapshot() == before
        assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
        mark("main_thirteen_databases_and_seventeen_container_ids_unchanged", restore20_running=0)
        succeeded = True
    finally:
        for started, name in ((runner_started, NAME), (pg_started, PG)):
            if started:
                run(["docker", "stop", "--time", "15", name])
                assert not inspect(name)["State"]["Running"]
        assert snapshot() == before
        mark("main_after_stop_unchanged_even_on_probe_failure")
        mark("source_fixture_stopped_data_retained", removed_containers=0, removed_volumes=0)
        save(WORK / "results.json", json.dumps({"passed": succeeded, "events": events}, indent=2).encode())

if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit("cost source validation failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno) + "; private diagnostics retained")
