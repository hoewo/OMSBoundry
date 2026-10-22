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
    "project-workspace": ("/project/api/v1/project-directory?status=all&limit=100", "items"),
    "billing-invoice": ("/billing/api/v1/invoice-registrations?limit=100", "items"),
    "money-flow-ledger": ("/money/api/v1/cash-transactions?limit=100", "items"),
    "contract-ledger": ("/contract/api/v1/contracts/workspace-context", None),
}
RUNTIME_CONTEXTS = {
    "contract-ledger": "/contract/api/v1/contracts/workspace-context",
    "money-flow-ledger": "/money/api/v1/cash-workspace",
}


def require_directory_authority(application, document):
    if application not in {"contract-ledger", "money-flow-ledger"}:
        return
    if not isinstance(document, dict):
        raise RuntimeError(f"{application} workspace context is not an object")
    if (
        document.get("party_runtime_mode") != "directory"
        or document.get("directory_authority_established") is not True
    ):
        raise RuntimeError(f"{application} Party Directory authority is not established")


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
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, file_pointer, code, message, headers, new_url):
            return None

    redirect_client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), NoRedirect()
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
            data = error.read()
            if error.code in expected:
                return error.code, data, error.headers
            raise RuntimeError(f"HTTP {error.code} for {path}") from None

    _, login_page, _ = request("/workbench/?view=oms-v1")
    for marker in (
        b"data-oms-login",
        b"assets/oms-phase1-login.css?v=20261022.6",
        b"assets/oms-phase1-login.js?v=20261022.6",
    ):
        if marker not in login_page:
            raise RuntimeError(f"OMS login page is missing current asset marker: {marker!r}")
    for retired in (
        b"assets/reference-oa-initiation-runtime.js",
        b"assets/reference-oa-login.js",
        b"assets/reference-oa-login.css",
        b"assets/reference-oa-workspace.css",
        b"assets/reference-oms-demo.css",
    ):
        if retired in login_page:
            raise RuntimeError(f"OMS login page still loads retired runtime: {retired!r}")

    for path in (
        "/workbench/assets/oms-phase1-login.css?v=20261022.6",
        "/workbench/assets/oms-phase1-login.js?v=20261022.6",
    ):
        _, asset, asset_headers = request(path)
        if not asset:
            raise RuntimeError(f"current OMS login asset is empty: {path}")
        if asset_headers.get("Cache-Control") != "public, max-age=31536000, immutable":
            raise RuntimeError(f"versioned OMS login asset is not immutable: {path}")

    for path in (
        "/legacy/",
        "/oms/",
        "/workbench/?view=legacy",
        "/workbench/?view=oms-reference",
    ):
        try:
            redirect_client.open(PUBLIC + path, timeout=30)
            raise RuntimeError(f"retired surface did not redirect: {path}")
        except urllib.error.HTTPError as error:
            error.read()
            if error.code != 303 or error.headers.get("Location") != "/workbench/?view=oms-v1":
                raise RuntimeError(f"retired surface does not resolve to current OMS: {path}") from None

    for path in (
        "/workbench/assets/reference-oa-react-bidding.js",
        "/workbench/assets/reference-oa-initiation-runtime.js",
        "/workbench/assets/reference-oms-demo.js",
        "/workbench/assets/workbench.js",
    ):
        status, _, _ = request(path, expected=(410,))
        if status != 410:
            raise RuntimeError(f"retired UI asset is still public: {path}")

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
        b"assets/oms-phase1.css?v=20261022.9",
        b"assets/oms-phase1-pagination.js?v=20261022.6",
        b"assets/oms-phase1.js?v=20261022.9",
    )
    if not all(marker in page for marker in markers):
        raise RuntimeError("OMS v1 page is missing required assembly markers")
    eager_legacy_assets = (
        b'src="assets/reference-oa-react-bidding.js',
        b'src="assets/reference-oa-initiation-runtime.js',
        b'href="assets/reference-oa-workspace.css',
        b'href="assets/reference-oa-business.css',
    )
    if any(marker in page for marker in eager_legacy_assets):
        raise RuntimeError("OMS v1 page still eagerly loads a legacy runtime asset")
    if b'href="/legacy/"' in page or b"view=oms-reference" in page:
        raise RuntimeError("OMS v1 page still advertises a retired public surface")
    if "text/html" not in page_headers.get("Content-Type", ""):
        raise RuntimeError("OMS v1 did not return HTML")

    critical_assets = {}
    for path in (
        "/workbench/assets/oms-phase1.css?v=20261022.9",
        "/workbench/assets/oms-phase1-pagination.js?v=20261022.6",
        "/workbench/assets/oms-phase1.js?v=20261022.9",
    ):
        _, asset, asset_headers = request(path)
        if asset_headers.get("Cache-Control") != "public, max-age=31536000, immutable":
            raise RuntimeError(f"versioned OMS asset is not immutable: {path}")
        critical_assets[path.rsplit("/", 1)[-1]] = len(asset)
        if path.endswith("oms-phase1.js?v=20261022.9") and (b"function partyVisibleForRole" not in asset or b"/api/v1/project-party-candidates" not in asset):
            raise RuntimeError("current OMS script is missing the party directory visibility guard")

    evidence_path = "/workbench/assets/oms-phase1-evidence.js?v=20261022.6"
    _, evidence_asset, evidence_headers = request(evidence_path)
    if evidence_headers.get("Cache-Control") != "public, max-age=31536000, immutable":
        raise RuntimeError("current OMS Evidence runtime is not immutable")
    if b"createEvidenceTransfer" not in evidence_asset:
        raise RuntimeError("current OMS Evidence runtime is missing its narrow factory")

    owner_counts = {}
    owner_items = {}
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
        runtime_path = RUNTIME_CONTEXTS.get(application)
        if runtime_path:
            _, runtime_raw, runtime_headers = request(runtime_path)
            if runtime_headers.get_content_type() != "application/json":
                raise RuntimeError(f"{application} runtime context did not return JSON")
            require_directory_authority(application, json.loads(runtime_raw))
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
            owner_items[application] = items

    operation_projects = [
        item
        for item in owner_items["project-workspace"]
        if "operations" in (item.get("project_type_keys") or [])
    ]
    client_ids = {
        (item.get("client_party") or {}).get("party_id")
        for item in operation_projects
        if (item.get("client_party") or {}).get("party_id")
    }
    contractor_ids = {
        link.get("resource_id")
        for item in operation_projects
        for link in (item.get("resource_links") or [])
        if link.get("target_service") == "party-directory"
        and link.get("resource_type") == "party"
        and link.get("relation_key") == "contractor-party"
        and not link.get("unlinked_at")
        and link.get("resource_id")
    }
    _, candidates_raw, candidates_headers = request(
        "/project/api/v1/project-party-candidates"
    )
    if candidates_headers.get_content_type() != "application/json":
        raise RuntimeError("project party candidates did not return JSON")
    candidates = json.loads(candidates_raw).get("items")
    if not isinstance(candidates, list):
        raise RuntimeError("project party candidates response has no item list")
    client_ids.update(
        item.get("party_id")
        for item in candidates
        if item.get("role_key") == "client" and item.get("party_id")
    )
    contractor_ids.update(
        item.get("party_id")
        for item in candidates
        if item.get("role_key") == "contractor" and item.get("party_id")
    )
    party_names = {
        item.get("id"): item.get("legal_name")
        for item in owner_items["party-directory"]
    }
    role_projection = {
        "clients": sorted(party_names.get(item, item) for item in client_ids),
        "contractors": sorted(party_names.get(item, item) for item in contractor_ids),
    }
    external_ids = set(party_names) - {OPERATING_PARTY_ID}
    directory_projection = {
        "clients": sorted(
            party_names[item]
            for item in external_ids
            if item in client_ids or item not in contractor_ids
        ),
        "contractors": sorted(
            party_names[item] for item in external_ids if item in contractor_ids
        ),
    }

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
                "role_projection": role_projection,
                "directory_projection": directory_projection,
                "project_party_candidates": len(candidates),
                "critical_assets_bytes": critical_assets,
                "lazy_evidence_asset_bytes": len(evidence_asset),
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
