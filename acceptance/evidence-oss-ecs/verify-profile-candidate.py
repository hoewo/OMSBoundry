#!/usr/bin/env python3
"""Render an extracted D1 candidate with private cloned config; never roll out."""
import json
import os
from pathlib import Path
import re
import subprocess

root = Path(os.environ["PROBE_PACKAGE_ROOT"])
assert str(root).startswith("/opt/omsbizfoundry/releases/omsbizfoundry-d1_")
source = Path(os.environ["PROBE_MAIN_ENV"])
assert str(source).startswith("/opt/omsbizfoundry/") and source.stat().st_mode & 0o077 == 0
def parse(path):
    return dict(line.split("=", 1) for line in path.read_text().splitlines() if line and not line.startswith("#") and "=" in line)
values = parse(source)
images = {key: value for key, value in parse(root / "deployments/d1/.env.example").items() if re.fullmatch(r"D1_[A-Z_]+_IMAGE", key)}
assert len(images) == 17
version = parse(root / "RELEASE-METADATA")["version"]
assert re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][0-9A-Za-z.-]+)?", version)
values.update(images)
values.update({"SUITE_VERSION": version, "D1_IMAGE_MODE": "pull", "D1_EVIDENCE_STORAGE_PROVIDER": "hybrid",
               "EVIDENCE_PRIMARY_STORAGE_PROVIDER": "aliyun-oss", "EVIDENCE_OSS_REGION": "cn-hangzhou",
               "EVIDENCE_OSS_BUCKET": "oatest-auth58-files-1", "EVIDENCE_OSS_UPLOAD_ROLE_ARN": "acs:ram::1169734041157847:role/auth58evidenceuploadrole",
               "EVIDENCE_OSS_BROWSER_ENDPOINT": "https://oatestfiles.antuinfo.cn", "EVIDENCE_OSS_BROWSER_CNAME": "true",
               "EVIDENCE_OSS_SERVICE_ENDPOINT": "https://oss-cn-hangzhou-internal.aliyuncs.com", "EVIDENCE_OSS_STS_ENDPOINT": "sts.cn-hangzhou.aliyuncs.com",
               "EVIDENCE_ECS_ROLE_NAME": "Auth58EvidenceServiceRole", "EVIDENCE_EGRESS_NETWORK": "evidence_oss_auth58_20261001a"})
runtime = root / "deployments/d1/.runtime"
runtime.mkdir(mode=0o700, exist_ok=True)
environment = runtime / "candidate-profile.env"
descriptor = os.open(environment, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(descriptor, "w") as output:
    output.write("".join(key + "=" + value + "\n" for key, value in values.items()))
command = ["sh", str(root / "deployments/d1/ops/d1"), "config", "--format", "json"]
result = subprocess.run(command, env={**os.environ, "D1_ENV_FILE": str(environment), "D1_PROJECT_NAME": "omsbizfoundry"}, text=True, capture_output=True, timeout=45)
if result.returncode:
    raise SystemExit("candidate render failed; private configuration suppressed")
config = json.loads(result.stdout)
assert [name for name, item in config["services"].items() if "evidence-egress" in item.get("networks", {})] == ["evidence-service"]
assert config["networks"]["app"]["internal"] and config["networks"]["data"]["internal"]
assert [name for name, item in config["services"].items() if item.get("ports")] == ["nginx"]
cloud = config["services"]["evidence-service"]["environment"]
assert cloud["STORAGE_PROVIDER"] == "hybrid" and cloud["PRIMARY_STORAGE_PROVIDER"] == "aliyun-oss"
assert cloud["ALIBABA_CLOUD_ECS_IMDSV2_ENABLE"] == "true" and cloud["IDENTITY_PROVIDER"] == "iam"
print(json.dumps({"result": "pass", "target": "Auth58 ECS", "scope": "extracted fixed-image release render only", "privileged_owner": "Evidence", "rollout": False, "private_config_written": True}))
