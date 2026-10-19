#!/usr/bin/env python3
"""Boot the pinned strict Workbench reader offline with the actual D1 manifest.

No database, cloud identity, IAM credential, network or published port is used.
This is compatibility readiness, not public browser or authorization evidence.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

NAME = "workbench_entry_auth58_20261001a"
WORK = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001") / NAME
IMAGE = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench@sha256:bbb8612a52fa8bccc022b8baaa25a62a640b89d4fab056f7c8b76f6ba03e0d1e"
CONFIG_ID = "sha256:7d36f573c631854028e3db118562f9aa921f1127c0585251fedc5a8cfc5cbf71"


def run(args):
    result = subprocess.run(args, capture_output=True, timeout=180)
    if result.returncode:
        raise RuntimeError("bounded command failed")
    return result.stdout


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def save(path, content, mode=0o600):
    with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, mode), "wb") as target:
        target.write(content)


def main():
    assert os.geteuid() == 0 and not WORK.exists()
    current = inspect("omsbizfoundry-unified-workbench-1")
    mount = next(value for value in current["Mounts"] if value["Destination"] == "/config/suite-manifest.json")
    original = Path(mount["Source"]).read_bytes()
    suite = json.loads(original)
    evidence = next(app for app in suite["applications"] if app["id"] == "evidence-service")
    assert evidence.pop("entry_action") == "evidence-service.view" and "entry_actions" not in evidence
    evidence["entry_actions"] = ["evidence-service.view", "evidence-service.metadata.read", "evidence-service.upload", "evidence-service.download"]
    run(["docker", "pull", IMAGE])
    meta = json.loads(run(["docker", "image", "inspect", IMAGE]))[0]
    assert meta["Architecture"] == "amd64" and meta["Config"]["User"] == "oms"
    assert IMAGE in meta["RepoDigests"] and meta["Id"] in (CONFIG_ID, IMAGE.split("@", 1)[1])
    WORK.mkdir(mode=0o700)
    save(WORK / "suite.json", json.dumps(suite).encode(), 0o444)
    created = False
    try:
        run(["docker", "run", "--detach", "--name", NAME, "--network", "none", "--memory", "96m", "--cpus", "0.15", "--pids-limit", "32", "--label", "oms.purpose=workbench-entry-candidate", "--mount", "type=bind,src=" + str(WORK / "suite.json") + ",dst=/config/suite-manifest.json,readonly", "--env", "GOMAXPROCS=1", "--env", "HTTP_ADDR=:8080", "--env", "SUITE_MANIFEST_PATH=/config/suite-manifest.json", IMAGE])
        created = True
        for attempt in range(45):
            try:
                result = json.loads(run(["docker", "exec", NAME, "wget", "-q", "-O", "-", "http://127.0.0.1:8080/health/ready"]))
                assert result["status"] == "ready"
                break
            except (RuntimeError, AssertionError):
                if attempt == 44:
                    raise RuntimeError("offline candidate did not become ready")
                time.sleep(1)
        body = run(["docker", "exec", NAME, "wget", "-q", "-O", "-", "http://127.0.0.1:8080/"])
        assert b"/gateway/login" in body
        info = inspect(NAME)
        assert info["HostConfig"]["NetworkMode"] == "none" and not info["HostConfig"]["PortBindings"]
        assert info["HostConfig"]["NanoCpus"] == 150000000 and info["Config"]["User"] == "oms"
        assert not any(value.startswith(("DATABASE_URL=", "BOOTSTRAP_ADMIN_PASSWORD=", "AWS_", "ALIBABA_CLOUD_")) for value in info["Config"]["Env"])
        assert Path(mount["Source"]).read_bytes() == original
        assert inspect("omsbizfoundry-unified-workbench-1")["Id"] == current["Id"]
        result = {"result": "pass", "check": "actual_d1_manifest_new_any_field_boots_pinned_workbench", "image": IMAGE, "anonymous_page_sha256": hashlib.sha256(body).hexdigest(), "network": "none", "published_ports": 0, "main_manifest_unchanged": True, "main_workbench_unchanged": True, "public_browser": "not_run"}
        save(WORK / "results.json", json.dumps(result, indent=2).encode())
        print(json.dumps(result), flush=True)
    finally:
        if created:
            run(["docker", "stop", "--time", "15", NAME])
            assert not inspect(NAME)["State"]["Running"]
            print(json.dumps({"check": "offline_workbench_candidate_stopped", "result": "pass"}), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        raise SystemExit("offline Workbench candidate failed; private diagnostics retained")
