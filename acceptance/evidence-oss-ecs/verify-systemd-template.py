#!/usr/bin/env python3
"""Parse the shipped drop-in in an isolated unit fixture; NEVER start a unit."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path("/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.44_linux_amd64/deployments/d1")


def main():
    assert os.geteuid() == 0
    template = (ROOT / "systemd/evidence-imds-guard.conf").read_text()
    assert "EnvironmentFile=/etc/omsbizfoundry/evidence-imds-guard.env" in template
    assert "ExecStartPre=/usr/local/libexec/omsbizfoundry/evidence-imds-guard boot" in template
    assert (ROOT / "ops/evidence-imds-guard").stat().st_mode & 0o111
    with tempfile.TemporaryDirectory(prefix="oms-evidence-unit-verify-") as directory:
        fixture = Path(directory)
        os.chmod(fixture, 0o700)
        environment = fixture / "guard.env"
        environment.write_text("EVIDENCE_IMDS_BRIDGE=br-fixture-01\nEVIDENCE_IMDS_SUBNET=172.29.0.0/24\n")
        os.chmod(environment, 0o600)
        rendered = template.replace("/etc/omsbizfoundry/evidence-imds-guard.env", str(environment))
        rendered = rendered.replace("/usr/local/libexec/omsbizfoundry/evidence-imds-guard", str(ROOT / "ops/evidence-imds-guard"))
        unit = fixture / "oms-evidence-unit-verify.service"
        unit.write_text("[Unit]\nDescription=Read-only Evidence template parser fixture\nDefaultDependencies=no\n" + rendered + "\nType=oneshot\nExecStart=/usr/bin/true\n")
        result = subprocess.run(["systemd-analyze", "verify", "--man=no", str(unit)], capture_output=True, text=True, timeout=30)
        relevant = [line for line in result.stderr.splitlines() if fixture.name in line or "evidence-imds-guard" in line]
        if result.returncode or relevant:
            # Only our generated unit, non-secret environment, and public paths
            # are inputs; no IAM/Docker environment or credential is read here.
            print(json.dumps({"parser_returncode": result.returncode, "fixture_diagnostic": result.stderr[:1500]}))
        assert result.returncode == 0 and not relevant, "systemd parser rejected the fixture"
        unrelated = len(result.stderr.splitlines()) - len(relevant)
    print(json.dumps({"result": "pass", "scope": "systemd parser fixture with explicit existing paths",
                      "template_installed": False, "unit_started": False, "Docker_or_host_restarted": False,
                      "iptables_changed": False, "unrelated_host_unit_warnings": unrelated}))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        raise SystemExit("systemd template parser check failed; private diagnostics suppressed")
