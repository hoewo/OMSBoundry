#!/usr/bin/env python3
"""Bounded Project PG18 migration/typed-port/empty-restore check, NOT C8.

Reuse only the stopped, separately owned source probe PostgreSQL. Do not start
its IAM/source runner, restore20 or any main service. All created runner state
and dumps are retained; temporary test DBs are owned by the test helper.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shlex
import subprocess
import time

ROOT = Path("/opt/omsbizfoundry/acceptance/recorded-material-consumer-20261001")
NAME = "recorded_material_consumer_auth58_20261001a"
WORK = ROOT / NAME
SOURCE_ROOT = Path("/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a")
PG = "production_material_source_auth58_20261001a_pg"
NETWORK = "production_material_source_auth58_20261001a_isolated"
RUNTIME = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66"
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
    network = json.loads(run(["docker", "network", "inspect", NETWORK]))[0]
    assert network["Internal"]
    assert not inspect("production_material_source_auth58_20261001a")["State"]["Running"]
    before = snapshot()
    assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
    WORK.mkdir(mode=0o700)
    for subdir in ("config", "bin", "data"):
        (WORK / subdir).mkdir(mode=0o700)
    save(WORK / "main-before.json", json.dumps(before, indent=2).encode())
    for source, target in (("project-material-consumer.test", "consumer.test"), ("project-material-migration.test", "migration.test")):
        save(WORK / "bin" / target, (ROOT / source).read_bytes(), 0o500)
    env = dict(line.split("=", 1) for line in (SOURCE_ROOT / "config/fixture.env").read_text().splitlines())
    database_url = shlex.split(env["TEST_DATABASE_URL"])[0]
    assert "@" + PG + ":5432/postgres?" in database_url
    save(WORK / "config/fixture.env", ("TEST_DATABASE_URL=" + shlex.quote(database_url) + "\n").encode())
    hashes = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in (WORK / "bin").iterdir()}
    save(WORK / "binaries.json", json.dumps({"sha256": hashes, "runtime_image": RUNTIME}, indent=2).encode())
    run(["chown", "-R", "70:70", str(WORK)])
    pg_started = runner_started = succeeded = False
    try:
        run(["docker", "start", PG]); pg_started = True
        for attempt in range(40):
            if subprocess.run(["docker", "exec", PG, "pg_isready", "-U", "probe_admin"], capture_output=True).returncode == 0:
                break
            assert attempt != 39, "private PG readiness timeout"
            time.sleep(1)
        run(["docker", "run", "--detach", "--name", NAME, "--network", NETWORK, "--memory", "160m", "--cpus", "0.30", "--pids-limit", "64", "--user", "70:70", "--label", "oms.purpose=project-material-consumer", "--mount", "type=bind,src=" + str(WORK / "bin") + ",dst=/probe/bin,readonly", "--mount", "type=bind,src=" + str(WORK / "config") + ",dst=/probe/config,readonly", "--mount", "type=bind,src=" + str(WORK / "data") + ",dst=/probe/data", "--mount", "type=bind,src=" + str(ROOT / "probe.sh") + ",dst=/probe/start.sh,readonly", "--entrypoint", "sh", RUNTIME, "/probe/start.sh"])
        runner_started = True
        info = inspect(NAME)
        assert not info["HostConfig"]["PortBindings"] and info["HostConfig"]["NanoCpus"] == 300000000
        mark("private_pg18_and_single_consumer_runner", published_ports=0, max_total_cpu=0.5, memory_limits_mb=352, real_upstream_iam=False)
        deadline = time.monotonic() + 235
        while inspect(NAME)["State"]["Running"]:
            assert time.monotonic() < deadline, "bounded consumer test timeout"
            time.sleep(2)
        info = inspect(NAME)
        log = run(["docker", "logs", NAME]); save(WORK / "test.log", log.encode())
        required = ("TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent", "TestRecordedMaterialMigration8To9KeepsFactsAndAllThreeLinks", "TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore", "TestRecordedMaterialProjectPostgresThreeSourcesAndEmptyRestore")
        assert info["State"]["ExitCode"] == 0 and not info["State"]["OOMKilled"], "consumer test failed; private diagnostics retained"
        assert all("--- PASS: " + test in log for test in required) and "--- SKIP" not in log
        dumps = list((WORK / "data/checkpoints").glob("project_workspace_test_*.dump"))
        assert len(dumps) == 2
        mark("historical7to8_current8to9_three_sources_and_empty_restore", tests=list(required), dump_sha256={path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted(dumps)}, append_after_restore=True, current_schema=9)
        assert snapshot() == before
        assert not inspect("production_material_source_auth58_20261001a")["State"]["Running"]
        assert not any("restore20" in name for name in run(["docker", "ps", "--format", "{{.Names}}"]).splitlines())
        mark("main_thirteen_databases_and_seventeen_container_ids_unchanged", restore20_running=0)
        succeeded = True
    finally:
        for started, name in ((runner_started, NAME), (pg_started, PG)):
            if started:
                run(["docker", "stop", "--time", "15", name])
                assert not inspect(name)["State"]["Running"]
        assert snapshot() == before, "Main changed after private fixture stop"
        mark("main_after_stop_unchanged", restore20_running=0)
        mark("consumer_fixture_stopped_data_retained", removed_containers=0, removed_volumes=0)
        save(WORK / "results.json", json.dumps({"passed": succeeded, "events": events}, indent=2).encode())

if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit("consumer validation failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno) + "; private diagnostics retained")
