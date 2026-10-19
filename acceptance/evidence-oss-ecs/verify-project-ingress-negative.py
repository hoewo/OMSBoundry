#!/usr/bin/env python3
"""Read-only forged-header denial and rebuilt Project IMDS isolation probe."""
import importlib.util
import ipaddress
import json
from pathlib import Path
import subprocess
import urllib.error
import urllib.request

spec = importlib.util.spec_from_file_location("project_probe", Path(__file__).with_name("rollout-project-feedback.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def fetch(url, headers):
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        with opener.open(urllib.request.Request(url, headers=headers), timeout=15) as response:
            return response.status, response.read(), response.headers
    except urllib.error.HTTPError as error:
        return error.code, error.read(), error.headers


def main():
    before = probe.facts("project_workspace")
    forged = {"X-OMS-Administrator": "true", "X-OMS-Subject-ID": "11111111-1111-4111-8111-111111111111",
              "X-OMS-Tenant-ID": "11111111-1111-4111-8111-111111111111", "Authorization": "Bearer unsigned-forgery"}
    public = "https://oatest.antuinfo.cn"
    status, _, headers = fetch(public + "/project/api/v1/projects", forged)
    assert status == 303 and headers.get("Location", "").startswith("/workbench/")
    assert fetch(public + "/auth/api/v1/session", forged)[0] == 401
    status, html, _ = fetch(public + "/workbench/", forged)
    assert status == 200 and b'<button class="architecture-lens-trigger"' not in html
    container = probe.inspect(probe.PROJECT)
    network = next(name for name in container["NetworkSettings"]["Networks"] if name.endswith("_data"))
    address = container["NetworkSettings"]["Networks"][network]["IPAddress"]
    assert ipaddress.ip_address(address).is_private
    assert fetch("http://" + address + ":8086/api/v1/projects", forged)[0] == 401
    probe.run(["docker", "exec", probe.PROJECT, "sh", "-c", "command -v wget >/dev/null"])
    result = subprocess.run(["docker", "exec", probe.PROJECT, "wget", "-T", "3", "-q", "-O", "-",
                             "http://100.100.100.200/latest/meta-data/ram/security-credentials/"], capture_output=True, timeout=10)
    assert result.returncode != 0 and not result.stdout
    assert probe.facts("project_workspace") == before
    print(json.dumps({"result": "pass", "public_forged_actor_denied": True, "IAM_session_denied": True,
                      "anonymous_admin_lens_absent": True, "target_app_forged_actor_denied": True,
                      "rebuilt_Project_IMDS_denied": True, "project_facts_unchanged": True,
                      "scope": "header forgery only; NOT neighboring-role/revocation or browser matrix"}))


if __name__ == "__main__":
    try:
        main()
    except Exception:
        raise SystemExit("Project ingress negative check failed; private diagnostics suppressed")
