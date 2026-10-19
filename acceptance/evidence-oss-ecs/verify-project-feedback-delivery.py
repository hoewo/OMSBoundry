#!/usr/bin/env python3
"""Read-only public HTTPS delivery check; explicitly NOT a browser click test."""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import traceback
import urllib.parse
import urllib.request

PUBLIC = "https://oatest.antuinfo.cn"
PROJECT_ID = "b6e26691-cfd6-4ef4-ba53-8d9f9a8c2cc2"
helper = Path(__file__).with_name("rollout-project-feedback.py")
spec = importlib.util.spec_from_file_location("project_probe", helper)
project_probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(project_probe)


def request(opener, path, method="GET", body=None, headers=None, expected=200):
    with opener.open(urllib.request.Request(PUBLIC + path, data=body, headers=headers or {}, method=method), timeout=30) as response:
        assert response.status == expected and urllib.parse.urlsplit(response.url).netloc == "oatest.antuinfo.cn"
        return response.read(), response.headers


def main():
    before = project_probe.facts("project_workspace")
    iam = project_probe.inspect("omsbizfoundry-identity-access-1")
    values = dict(v.split("=", 1) for v in iam["Config"]["Env"])
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(request(opener, "/auth/api/v1/csrf")[0])["csrf_token"]
    headers = {"Content-Type": "application/json", "X-CSRF-Token": csrf, "Origin": PUBLIC}
    login = json.dumps({"username": values["BOOTSTRAP_ADMIN_USERNAME"], "password": values["BOOTSTRAP_ADMIN_PASSWORD"]}).encode()
    request(opener, "/auth/api/v1/login", "POST", login, headers)
    try:
        form = urllib.parse.urlencode({"csrf_token": csrf}).encode()
        request(opener, "/gateway/authorize/project-workspace", "POST", form,
                {"Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": csrf, "Origin": PUBLIC})
        html, _ = request(opener, "/project/?project_id=" + PROJECT_ID)
        assert b'assets/demo.js' in html
        script, _ = request(opener, "/project/assets/demo.js")
        for marker in (b"async function refreshRejectedInitiation(", b"state.commandInFlight", b"e.status=r.redirected?401:r.status"):
            assert marker in script, "new page code was not delivered"
        detail = json.loads(request(opener, "/project/api/v1/projects/" + PROJECT_ID)[0])
        assert detail["project"]["id"] == PROJECT_ID
        assert detail["project"]["project_number"] == "OA-VERIFY-20261001-BROWSER-1634"
        assert detail["project"]["status"] == "pending_completion" and detail["project"]["revision"] == 2
        assert isinstance(detail["allowed_actions"], list) and project_probe.facts("project_workspace") == before
        print(json.dumps({"result": "pass", "scope": "public HTTPS HTML/API read only; NOT browser regression", "project_id": PROJECT_ID,
                          "revision": 2, "status": "pending_completion", "new_feedback_code_delivered": True,
                          "page_sha256": hashlib.sha256(html).hexdigest(), "script_sha256": hashlib.sha256(script).hexdigest(),
                          "project_facts_unchanged": True}), flush=True)
    finally:
        request(opener, "/auth/api/v1/logout", "POST", b"", headers, expected=204)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(json.dumps({"result": "failed", "error_type": type(error).__name__,
                          "code_lines": [frame.lineno for frame in traceback.extract_tb(error.__traceback__)],
                          "sensitive_diagnostics": "suppressed"}))
        raise SystemExit(1)
