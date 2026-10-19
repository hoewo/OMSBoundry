#!/usr/bin/env python3
"""User-authorized restore20 stop only: preserve containers, volumes and backups."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess

RESTORE_PROJECT = "omsbizfoundry-restore20"
MAIN_PROJECT = "omsbizfoundry"
OUTPUT = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/restore20-stop-evidence.json")
BEFORE = OUTPUT.with_name("restore20-stop-before.json")
spec = importlib.util.spec_from_file_location("probe", Path(__file__).with_name("rollout-project-feedback.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


def containers(project):
    ids = subprocess.check_output(["docker", "ps", "-a", "--filter", "label=com.docker.compose.project=" + project, "--format", "{{.ID}}"], text=True).splitlines()
    values = json.loads(subprocess.check_output(["docker", "inspect", *ids], text=True))
    # Existing successful one-shot migrations are not targets for this stop.
    return {v["Config"]["Labels"]["com.docker.compose.service"]: v for v in values if v["State"]["Running"]}


def canonical_mounts(values):
    # Docker inspection serializes a mount map; list order is not identity.
    return sorted(values, key=lambda value: json.dumps(value, sort_keys=True))


def main():
    assert os.geteuid() == 0 and os.environ.get("ALLOW_STOP_RESTORE20") == "yes"
    assert not OUTPUT.exists() and not BEFORE.exists(), "refuse to replace previous evidence or repeat a partially completed stop"
    restore = containers(RESTORE_PROJECT)
    primary = containers(MAIN_PROJECT)
    expected = set(probe.SERVICES.values())
    assert set(restore) == expected == set(primary) and len(expected) == 17
    assert all(v["State"].get("Health", {}).get("Status") == "healthy" for v in primary.values())
    assert not {v["Id"] for v in restore.values()} & {v["Id"] for v in primary.values()}
    mounts = {k: canonical_mounts(v["Mounts"]) for k, v in restore.items()}
    volumes = {m["Name"] for values in mounts.values() for m in values if m["Type"] == "volume"}
    before = {"restore_ids": {k: v["Id"] for k, v in restore.items()}, "restore_mounts": mounts,
              "main_ids": {k: v["Id"] for k, v in primary.items()}, "main_images": {k: v["Image"] for k, v in primary.items()}}
    with os.fdopen(os.open(BEFORE, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as destination:
        json.dump(before, destination)
    order = ["nginx", "gateway", "unified-workbench", "contract-ledger", "money-flow-ledger", "people-groups", "party-directory",
             "project-workspace", "business-engagement", "billing-invoice", "production-operations", "approval-audit-results",
             "cost-outsourcing", "bonus-distribution", "evidence-service", "identity-access", "postgres"]
    for index in range(0, len(order), 3):
        batch = order[index:index + 3]
        result = subprocess.run(["docker", "stop", "--time", "20", *[restore[name]["Id"] for name in batch]], capture_output=True, timeout=90)
        assert result.returncode == 0
        print(json.dumps({"phase": "stop_restore20", "stopped": min(index + 3, 17), "total": 17}), flush=True)
    for name, old in restore.items():
        current = probe.inspect(old["Id"])
        assert not current["State"]["Running"] and current["Id"] == old["Id"] and canonical_mounts(current["Mounts"]) == mounts[name]
    for name in volumes:
        probe.run(["docker", "volume", "inspect", name])
    now = containers(MAIN_PROJECT)
    assert set(now) == expected
    assert all(now[k]["Id"] == v["Id"] and now[k]["Image"] == v["Image"] and now[k]["State"].get("Health", {}).get("Status") == "healthy" for k, v in primary.items())
    evidence = {"result": "pass", "stopped_project": RESTORE_PROJECT, "stopped_containers": 17,
                "main_17_healthy_and_same_ids": True, "volumes_preserved": len(volumes),
                "container_ids": {k: v["Id"] for k, v in restore.items()}, "mounts": mounts,
                "deletion_commands_used": False, "restart_command": "docker start <these same retained container IDs in dependency order>"}
    with os.fdopen(os.open(OUTPUT, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as destination:
        json.dump(evidence, destination)
    print(json.dumps({k: v for k, v in evidence.items() if k not in ("container_ids", "mounts", "restart_command")}), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit("restore20 stop/postcheck interrupted: " + type(error).__name__ + " at line " + str(trace.tb_lineno) + "; inspect retained containers, no removal performed")
