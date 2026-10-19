#!/usr/bin/env python3
"""Create or reuse the configured OMS qualification Party through public IAM/Gateway APIs.

Run this on the deployed host. Bootstrap credentials are read from the running IAM
container and never written to disk or printed.
"""

import http.cookiejar
import json
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


PUBLIC = "https://oatest.antuinfo.cn"
PARTY_NAME = "北京京宏勘察设计有限公司"
PARTY_ACTIONS = (
    "party-directory.read",
    "party-directory.create",
    "party-directory.update",
    "party-directory.status.manage",
    "party-directory.history.read",
    "party-directory.reference.read",
)


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
    username = environment["BOOTSTRAP_ADMIN_USERNAME"]
    password = environment["BOOTSTRAP_ADMIN_PASSWORD"]
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
    )

    def request(path, method="GET", payload=None, form=None, expected=(200,), extra_headers=None):
        headers = {"Accept": "application/json", "Origin": PUBLIC, **(extra_headers or {})}
        body = None
        if payload is not None:
            headers["Content-Type"] = "application/json"
            body = json.dumps(payload, ensure_ascii=False).encode()
        elif form is not None:
            headers["Content-Type"] = "application/x-www-form-urlencoded"
            body = urllib.parse.urlencode(form).encode()
        try:
            with client.open(
                urllib.request.Request(PUBLIC + path, data=body, headers=headers, method=method),
                timeout=30,
            ) as response:
                data = response.read()
                if response.status not in expected:
                    raise RuntimeError(f"unexpected HTTP {response.status} for {path}")
                if not data:
                    return response.status, None
                content_type = response.headers.get_content_type()
                return response.status, json.loads(data) if content_type == "application/json" else data
        except urllib.error.HTTPError as error:
            error.read()
            if error.code in expected:
                return error.code, None
            raise RuntimeError(f"HTTP {error.code} for {path}") from None

    _, csrf_document = request("/auth/api/v1/csrf")
    csrf = csrf_document["csrf_token"]
    _, login_document = request(
        "/auth/api/v1/login",
        method="POST",
        payload={"username": username, "password": password},
        extra_headers={"X-CSRF-Token": csrf},
    )
    principal = login_document["principal"]
    if not principal.get("administrator"):
        raise RuntimeError("bootstrap account is not an administrator")
    subject_id = principal["subject_id"]

    # Make the default deployment administrator usable for Party maintenance. The
    # endpoint is idempotent: an existing active grant returns conflict and is kept.
    for action in PARTY_ACTIONS:
        headers = {"X-CSRF-Token": csrf}
        payload = {
            "subject_id": subject_id,
            "audience": "party-directory",
            "action": action,
            "scope": {"kind": "tenant"},
        }
        body = json.dumps(payload).encode()
        req = urllib.request.Request(
            PUBLIC + "/auth/api/v1/admin/grants",
            data=body,
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
                "Origin": PUBLIC,
                **headers,
            },
            method="POST",
        )
        try:
            with client.open(req, timeout=30) as response:
                if response.status != 204:
                    raise RuntimeError(f"grant failed for {action}")
        except urllib.error.HTTPError as error:
            error.read()
            if error.code != 409:
                raise RuntimeError(f"grant HTTP {error.code} for {action}") from None

    request(
        "/gateway/authorize/party-directory",
        method="POST",
        form={"csrf_token": csrf},
        extra_headers={"X-CSRF-Token": csrf},
    )
    _, page = request(
        "/party/api/v1/parties?status=all&limit=100&query="
        + urllib.parse.quote(PARTY_NAME)
    )
    matches = [item for item in page.get("items", []) if item.get("legal_name") == PARTY_NAME]
    created = False
    if matches:
        party = matches[0]
    else:
        _, party = request(
            "/party/api/v1/parties",
            method="POST",
            payload={
                "command_id": str(uuid.uuid4()),
                "kind": "organization",
                "legal_name": PARTY_NAME,
                "registration_identifier": None,
            },
            expected=(201,),
        )
        created = True

    party_id = party.get("id", "")
    if (
        len(party_id) != 36
        or party.get("legal_name") != PARTY_NAME
        or party.get("status") != "active"
    ):
        raise RuntimeError("qualification Party verification failed")
    _, verified = request("/party/api/v1/parties/" + party_id)
    if verified.get("id") != party_id or verified.get("legal_name") != PARTY_NAME:
        raise RuntimeError("qualification Party read-back failed")

    # Logout is best-effort; its CSRF check is intentionally explicit.
    request("/auth/api/v1/logout", method="POST", expected=(204,), extra_headers={"X-CSRF-Token": csrf})
    print(
        json.dumps(
            {
                "passed": True,
                "party_id": party_id,
                "legal_name": PARTY_NAME,
                "created": created,
                "public_gateway_iam_party": True,
            },
            ensure_ascii=False,
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
            f"qualification Party setup failed: {type(error).__name__} at line {line}; private diagnostics suppressed"
        )
