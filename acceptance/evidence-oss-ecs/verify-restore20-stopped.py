#!/usr/bin/env python3
"""Read-only postcheck after the user-authorized restore20 stop.

Does not repeat stop/start, remove anything, or assert an unavailable pre-stop
mount snapshot. Private IDs and paths stay in the root-only evidence file.
"""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

spec = importlib.util.spec_from_file_location("probe", Path(__file__).with_name("rollout-project-feedback.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)
OUTPUT = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/restore20-stopped-postcheck.json")


def inventory(project):
    ids = subprocess.check_output(["docker", "ps", "-a", "--filter", "label=com.docker.compose.project=" + project, "--format", "{{.ID}}"], text=True).splitlines()
    assert ids
    values = json.loads(probe.run(["docker", "inspect", *ids]))
    expected = set(probe.SERVICES.values())
    values = [v for v in values if v["Config"]["Labels"]["com.docker.compose.service"] in expected]
    result = {v["Config"]["Labels"]["com.docker.compose.service"]: v for v in values}
    assert len(values) == len(result) == 17 and set(result) == expected
    return result


def main():
    assert os.geteuid() == 0 and not OUTPUT.exists()
    restore = inventory("omsbizfoundry-restore20")
    primary = inventory("omsbizfoundry")
    assert all(not v["State"]["Running"] and v["State"]["ExitCode"] == 0 for v in restore.values())
    assert all(v["State"]["Running"] and v["State"].get("Health", {}).get("Status") == "healthy" for v in primary.values())
    assert not {v["Id"] for v in restore.values()} & {v["Id"] for v in primary.values()}
    volumes = {m["Name"] for v in restore.values() for m in v["Mounts"] if m["Type"] == "volume"}
    assert volumes
    for name in volumes:
        probe.run(["docker", "volume", "inspect", name])
    for v in restore.values():
        for m in v["Mounts"]:
            if m["Type"] == "bind":
                assert Path(m["Source"]).exists()
        labels = v["Config"]["Labels"]
        assert all(Path(p).is_file() for p in labels["com.docker.compose.project.config_files"].split(","))
        env = labels.get("com.docker.compose.project.environment_file")
        if env:
            assert Path(env).is_file()
    # Main starts predate the stop; no main container was recreated by this action.
    first_stop = min(v["State"]["FinishedAt"] for v in restore.values())
    assert all(v["State"]["StartedAt"] < first_stop for v in primary.values())
    backup = Path("/var/backups/omsbizfoundry/project-ui-test43-20261001/project-workspace.dump")
    assert backup.is_file() and backup.stat().st_size == 57060
    evidence = {"result": "pass", "restore20_stopped_exit_zero": 17, "main_healthy": 17,
                "main_starts_predate_stop": True, "retained_named_volumes": len(volumes),
                "restore_configuration_and_bind_sources_present": True, "project_dump_present": True,
                "pre_stop_exact_mount_comparison": "not_recorded_by_failed_initial_helper",
                "docker_or_business_mutations": False,
                "restore_ids": {k: v["Id"] for k, v in restore.items()},
                "main_ids": {k: v["Id"] for k, v in primary.items()},
                "restore_mounts": {k: v["Mounts"] for k, v in restore.items()}}
    with os.fdopen(os.open(OUTPUT, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as target:
        json.dump(evidence, target)
    print(json.dumps({k: v for k, v in evidence.items() if k not in ("restore_ids", "main_ids", "restore_mounts")}))


def mount_order_diagnostic():
    first = inventory("omsbizfoundry-restore20")
    second = inventory("omsbizfoundry-restore20")
    same_sets = all(sorted(json.dumps(m, sort_keys=True) for m in v["Mounts"]) ==
                    sorted(json.dumps(m, sort_keys=True) for m in second[name]["Mounts"]) for name, v in first.items())
    order_changes = sum(v["Mounts"] != second[name]["Mounts"] for name, v in first.items())
    policies = {}
    for value in second.values():
        policy = value["HostConfig"]["RestartPolicy"]["Name"]
        policies[policy] = policies.get(policy, 0) + 1
    print(json.dumps({"two_reads_mount_sets_identical": same_sets, "mount_order_changes": order_changes,
                      "restore_restart_policies": policies, "read_only": True}))


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["--mount-order-diagnostic"]:
            mount_order_diagnostic()
        else:
            assert not sys.argv[1:]
            main()
    except Exception as error:
        # No configuration, subprocess output, or credentials in public diagnostics.
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit("read-only restore20 postcheck failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno))
