#!/usr/bin/env python3
"""Run candidate IAM contract parsing only, without Suite rollout or DB writes."""
import json
import os
from pathlib import Path
import subprocess

root = Path(os.environ["PROBE_PACKAGE_ROOT"])
assert str(root).startswith("/opt/omsbizfoundry/releases/omsbizfoundry-d1_")
environment = root / "deployments/d1/.runtime/candidate-profile.env"
assert environment.is_file() and environment.stat().st_mode & 0o077 == 0
# One extra validation container at a time; wait until every OSS drill is stopped.
running = subprocess.run(["docker", "ps", "--filter", "label=oms.purpose=evidence-oss-acceptance", "--format", "{{.Names}}"], capture_output=True, text=True, check=True)
assert not running.stdout.strip(), "finish the isolated cloud drill before preflight"
result = subprocess.run(["sh", str(root / "deployments/d1/ops/d1"), "preflight-upgrade"],
                        env={**os.environ, "D1_ENV_FILE": str(environment), "D1_PROJECT_NAME": "omsbizfoundry"},
                        capture_output=True, text=True, timeout=180)
if result.returncode:
    raise SystemExit("candidate IAM preflight failed; private diagnostics suppressed")
assert "validating candidate IAM parsers" in result.stdout
print(json.dumps({"result": "pass", "target": "Auth58 ECS", "scope": "fixed-image candidate IAM contract preflight", "rollout": False, "database_opened_by_validator": False}))
