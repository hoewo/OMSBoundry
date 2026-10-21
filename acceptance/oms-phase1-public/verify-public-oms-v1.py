#!/usr/bin/env python3
"""Read-only public acceptance for the deployed OMS v1 assembly."""

import hashlib
import http.cookiejar
import json
import subprocess
import urllib.error
import urllib.parse
import urllib.request


PUBLIC = "https://oatest.antuinfo.cn"
OPERATING_PARTY_ID = "95d3793f-7e0d-4ec9-acb8-9baf8bcb1039"
APPLICATIONS = {
    "party-directory": ("/party/api/v1/parties?status=all&limit=100", "items"),
    "project-workspace": ("/project/api/v1/projects?limit=100", "items"),
    "billing-invoice": ("/billing/api/v1/invoice-registrations?limit=100", "items"),
    "money-flow-ledger": ("/money/api/v1/cash-transactions?limit=100", "items"),
    "contract-ledger": ("/contract/api/v1/contracts/workspace-context", None),
}


def iam_environment():
    output = subprocess.check_output(
        [
            "sudo",
            "docker",
            "inspect",
            "omsbizfoundry-identity-access-1",
            "--format",
            "{{range .Config.Env}}{{println .}}{{end}}",
        ],
        text=True,
    )
    return dict(line.split("=", 1) for line in output.splitlines() if "=" in line)


def main():
    environment = iam_environment()
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
    )

    def request(path, method="GET", body=None, headers=None, expected=(200,)):
        try:
            with client.open(
                urllib.request.Request(
                    PUBLIC + path,
                    data=body,
                    headers={"Origin": PUBLIC, **(headers or {})},
                    method=method,
                ),
                timeout=30,
            ) as response:
                data = response.read()
                if response.status not in expected:
                    raise RuntimeError(f"unexpected HTTP {response.status} for {path}")
                return response.status, data, response.headers
        except urllib.error.HTTPError as error:
            error.read()
            raise RuntimeError(f"HTTP {error.code} for {path}") from None

    _, csrf_raw, _ = request("/auth/api/v1/csrf")
    csrf = json.loads(csrf_raw)["csrf_token"]
    login_body = json.dumps(
        {
            "username": environment["BOOTSTRAP_ADMIN_USERNAME"],
            "password": environment["BOOTSTRAP_ADMIN_PASSWORD"],
        }
    ).encode()
    _, login_raw, _ = request(
        "/auth/api/v1/login",
        "POST",
        login_body,
        {"Content-Type": "application/json", "X-CSRF-Token": csrf},
    )
    principal = json.loads(login_raw)["principal"]
    if not principal.get("administrator"):
        raise RuntimeError("public login did not return the deployment administrator")

    _, page, page_headers = request("/workbench/?view=oms-v1")
    markers = (
        b"data-oms-v1",
        b"data-oms-v1-nav=\"overview\"",
        b"data-oms-v1-nav=\"projects\"",
        b"data-oms-v1-nav=\"receivable\"",
        b"data-oms-v1-nav=\"payable\"",
        (b"data-operating-party-id=\"" + OPERATING_PARTY_ID.encode() + b"\""),
        b"assets/oms-phase1.css",
        b"assets/oms-phase1.js",
    )
    if not all(marker in page for marker in markers):
        raise RuntimeError("OMS v1 page is missing required assembly markers")
    if "text/html" not in page_headers.get("Content-Type", ""):
        raise RuntimeError("OMS v1 did not return HTML")

    owner_counts = {}
    for application, (path, list_key) in APPLICATIONS.items():
        form = urllib.parse.urlencode({"csrf_token": csrf}).encode()
        request(
            "/gateway/authorize/" + application,
            "POST",
            form,
            {
                "Content-Type": "application/x-www-form-urlencoded",
                "X-CSRF-Token": csrf,
            },
        )
        _, raw, headers = request(path)
        if headers.get_content_type() != "application/json":
            raise RuntimeError(f"{application} did not return JSON")
        document = json.loads(raw)
        if list_key is None:
            owner_counts[application] = len(document) if isinstance(document, dict) else 0
        else:
            items = document.get(list_key)
            if not isinstance(items, list):
                raise RuntimeError(f"{application} response has no item list")
            owner_counts[application] = len(items)

    _, party_raw, _ = request("/party/api/v1/parties/" + OPERATING_PARTY_ID)
    party = json.loads(party_raw)
    if party.get("legal_name") != "北京京宏勘察设计有限公司" or party.get("status") != "active":
        raise RuntimeError("configured operating Party is not active or has the wrong name")

    request(
        "/auth/api/v1/logout",
        "POST",
        b"",
        {"X-CSRF-Token": csrf},
        expected=(204,),
    )
    print(
        json.dumps(
            {
                "passed": True,
                "url": PUBLIC + "/oms-v1/",
                "native_login": True,
                "gateway_authorizations": sorted(APPLICATIONS),
                "owner_reads": owner_counts,
                "operating_party_id": OPERATING_PARTY_ID,
                "page_sha256": hashlib.sha256(page).hexdigest(),
                "business_writes": 0,
            },
            ensure_ascii=False,
            sort_keys=True,
        )
    )


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace and trace.tb_next:
            trace = trace.tb_next
        line = trace.tb_lineno if trace else 0
        raise SystemExit(
            f"public OMS v1 verification failed: {type(error).__name__} at line {line}; private diagnostics suppressed"
        )
