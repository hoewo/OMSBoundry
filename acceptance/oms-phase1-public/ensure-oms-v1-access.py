#!/usr/bin/env python3
"""Idempotently grant the OMS phase-one operator capabilities through IAM.

The bootstrap password is read from the selected IAM container and is never
printed or written to disk.  This script changes IAM facts only through the
public, versioned admin API; it does not edit IAM tables directly.
"""

from __future__ import annotations

import argparse
import http.cookiejar
import json
import subprocess
import urllib.error
import urllib.request


OMS_ACTIONS = {
    "party-directory": (
        "party-directory.read",
        "party-directory.create",
        "party-directory.update",
        "party-directory.status.manage",
        "party-directory.history.read",
        "party-directory.reference.read",
    ),
    "project-workspace": (
        "project-workspace.read",
        "project-workspace.create",
        "project-workspace.update",
        "project-workspace.history.read",
        "project-workspace.resource-link.manage",
        "project-workspace.commercial-summary.read",
    ),
    "contract-ledger": (
        "contract-ledger.view",
        "contract-ledger.draft.manage",
        "contract-ledger.lifecycle.manage",
        "contract-ledger.reference.read",
        "contract-ledger.evidence.manage",
    ),
    "money-flow-ledger": (
        "money-flow-ledger.view",
        "money-flow-ledger.amount.read",
        "money-flow-ledger.cash.record",
        "money-flow-ledger.cash.evidence.manage",
        "money-flow-ledger.statistics.read",
        "money-flow-ledger.export",
    ),
    "billing-invoice": (
        "billing-invoice.read",
        "billing-invoice.amount.read",
        "billing-invoice.record",
        "billing-invoice.correct",
        "billing-invoice.history.read",
        "billing-invoice.reference.read",
        "billing-invoice.evidence.manage",
        "billing-invoice.export",
    ),
    "evidence-service": (
        "evidence-service.view",
        "evidence-service.metadata.read",
        "evidence-service.upload",
        "evidence-service.download",
    ),
}


def container_environment(container: str) -> dict[str, str]:
    output = subprocess.check_output(
        ["docker", "inspect", container, "--format", "{{range .Config.Env}}{{println .}}{{end}}"],
        text=True,
    )
    return dict(line.split("=", 1) for line in output.splitlines() if "=" in line)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="https://oatest.antuinfo.cn")
    parser.add_argument("--identity-container", required=True)
    args = parser.parse_args()
    base_url = args.base_url.rstrip("/")
    environment = container_environment(args.identity_container)
    username = environment["BOOTSTRAP_ADMIN_USERNAME"]
    password = environment["BOOTSTRAP_ADMIN_PASSWORD"]

    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
    )

    def request(path: str, method: str = "GET", payload: dict | None = None, expected=(200,)):
        body = None
        headers = {"Accept": "application/json", "Origin": base_url}
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(base_url + path, data=body, headers=headers, method=method)
        try:
            with client.open(req, timeout=30) as response:
                raw = response.read()
                if response.status not in expected:
                    raise RuntimeError(f"unexpected HTTP {response.status} for {path}")
                return response.status, json.loads(raw) if raw else None
        except urllib.error.HTTPError as error:
            error.read()
            if error.code in expected:
                return error.code, None
            raise RuntimeError(f"HTTP {error.code} for {path}") from None

    _, csrf_document = request("/auth/api/v1/csrf")
    csrf = csrf_document["csrf_token"]
    login_body = json.dumps({"username": username, "password": password}).encode()
    login_request = urllib.request.Request(
        base_url + "/auth/api/v1/login",
        data=login_body,
        headers={
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Origin": base_url,
            "X-CSRF-Token": csrf,
        },
        method="POST",
    )
    with client.open(login_request, timeout=30) as response:
        principal = json.loads(response.read())["principal"]
    if not principal.get("administrator"):
        raise RuntimeError("bootstrap account is not an administrator")

    subject_id = principal["subject_id"]
    applied = 0
    existing = 0
    for audience, actions in OMS_ACTIONS.items():
        for action in actions:
            grant_request = urllib.request.Request(
                base_url + "/auth/api/v1/admin/grants",
                data=json.dumps(
                    {
                        "subject_id": subject_id,
                        "audience": audience,
                        "action": action,
                        "scope": {"kind": "tenant"},
                    }
                ).encode(),
                headers={
                    "Accept": "application/json",
                    "Content-Type": "application/json",
                    "Origin": base_url,
                    "X-CSRF-Token": csrf,
                },
                method="POST",
            )
            try:
                with client.open(grant_request, timeout=30) as response:
                    if response.status != 204:
                        raise RuntimeError(f"unexpected grant HTTP {response.status} for {action}")
                    applied += 1
            except urllib.error.HTTPError as error:
                error.read()
                if error.code != 409:
                    raise RuntimeError(f"grant HTTP {error.code} for {action}") from None
                existing += 1

    logout_request = urllib.request.Request(
        base_url + "/auth/api/v1/logout",
        headers={"Origin": base_url, "X-CSRF-Token": csrf},
        method="POST",
    )
    try:
        client.open(logout_request, timeout=30).close()
    except urllib.error.HTTPError as error:
        error.read()
        if error.code != 204:
            raise RuntimeError(f"logout HTTP {error.code}") from None

    print(
        json.dumps(
            {
                "passed": True,
                "subject_id": subject_id,
                "capabilities_required": sum(len(actions) for actions in OMS_ACTIONS.values()),
                "capabilities_applied": applied,
                "capabilities_existing": existing,
            },
            ensure_ascii=False,
        )
    )


if __name__ == "__main__":
    main()
