#!/usr/bin/env python3
"""Read actual deployed image identities; prepare candidate manifest, no rollout."""
import json
import os
from pathlib import Path
import re
import subprocess

services = {"POSTGRES": "postgres", "IDENTITY": "identity-access", "CONTRACT": "contract-ledger",
            "MONEY": "money-flow-ledger", "PEOPLE": "people-groups", "PARTY": "party-directory",
            "PROJECT": "project-workspace", "EVIDENCE": "evidence-service", "BUSINESS_ENGAGEMENT": "business-engagement",
            "BILLING": "billing-invoice", "PRODUCTION": "production-operations", "APPROVAL_AUDIT_RESULTS": "approval-audit-results",
            "COST_OUTSOURCING": "cost-outsourcing", "BONUS_DISTRIBUTION": "bonus-distribution",
            "WORKBENCH": "unified-workbench", "GATEWAY": "gateway", "NGINX": "nginx"}
evidence = os.environ["PROBE_IMAGE"]
assert re.fullmatch(r"[a-z0-9./:_-]+@sha256:[a-f0-9]{64}", evidence)
lines = ["D1_IMAGE_MODE=pull"]
for key, service in services.items():
    container = json.loads(subprocess.check_output(["docker", "inspect", "omsbizfoundry-" + service + "-1"], text=True))[0]
    image = json.loads(subprocess.check_output(["docker", "image", "inspect", container["Image"]], text=True))[0]
    assert image["Id"] == container["Image"]
    refs = [ref for ref in image.get("RepoDigests", []) if ".aliyuncs.com/omsbizfoundry/" in ref]
    assert len(refs) == 1, "one verified deployed ACR digest required"
    reference = evidence if key == "EVIDENCE" else refs[0]
    lines.append("D1_" + key + "_IMAGE=" + reference)
destination = Path(os.environ["PROBE_IMAGE_MANIFEST"])
assert str(destination).startswith("/opt/omsbizfoundry/acceptance/")
descriptor = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(descriptor, "w") as target:
    target.write("\n".join(lines) + "\n")
print(json.dumps({"result": "pass", "deployed_images_resolved": 17, "only_candidate_change": "Evidence", "rollout": False}))
