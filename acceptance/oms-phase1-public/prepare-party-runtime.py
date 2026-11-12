#!/usr/bin/env python3
"""Explicitly establish Contract and Money Party Directory authority for OMS.

Run this on the Docker host.  Every legacy Party is reviewed into one immutable
migration batch per ledger before the ledger-owned cutover command is allowed.
Exact identifiers and unique exact names are preferred.  The historical D1
fixtures additionally share their stable timestamp/random suffix across the
Party, Contract and Money bootstrap records; that suffix is the only accepted
cross-application fixture match.  Anything else is created as a distinct Party
Directory record, so this script never guesses from a fuzzy name.

Passwords and capability credentials are never printed or written to disk.
"""

from __future__ import annotations

import argparse
import http.cookiejar
import json
from pathlib import Path
import re
import subprocess
import urllib.parse
import urllib.error
import urllib.request
import uuid


D1_FIXTURE_IDENTIFIER = re.compile(r"^D1-(?:PARTY|C|M)-(.+)$", re.IGNORECASE)


def compact(value):
    if value is None:
        return None
    normalized = str(value).strip()
    return normalized or None


def d1_fixture_key(value):
    normalized = compact(value)
    if normalized is None:
        return None
    matched = D1_FIXTURE_IDENTIFIER.fullmatch(normalized)
    return matched.group(1).casefold() if matched else None


def select_directory_party(local: dict, parties: list[dict]) -> dict | None:
    """Return a single evidence-backed target or None; never choose ambiguously."""
    active = [item for item in parties if item.get("status") == "active"]
    registration = compact(local.get("registration_identifier"))
    if registration is not None:
        exact = [
            item
            for item in active
            if compact(item.get("registration_identifier")) == registration
        ]
        if len(exact) == 1:
            return exact[0]
        if len(exact) > 1:
            raise RuntimeError(f"ambiguous Party registration identifier: {registration}")

        fixture_key = d1_fixture_key(registration)
        if fixture_key is not None:
            fixture = [
                item
                for item in active
                if d1_fixture_key(item.get("registration_identifier")) == fixture_key
            ]
            if len(fixture) == 1:
                return fixture[0]
            if len(fixture) > 1:
                raise RuntimeError(f"ambiguous D1 fixture Party suffix: {fixture_key}")

    legal_name = str(local.get("legal_name", "")).strip()
    kind = str(local.get("kind", "")).strip()
    exact_name = [
        item
        for item in active
        if str(item.get("legal_name", "")).strip() == legal_name
        and str(item.get("kind", "")).strip() == kind
    ]
    if len(exact_name) == 1:
        return exact_name[0]
    if len(exact_name) > 1:
        raise RuntimeError(f"ambiguous Party legal name and kind: {kind}/{legal_name}")
    return None


def container_environment(container: str) -> dict[str, str]:
    output = subprocess.check_output(
        ["docker", "inspect", container, "--format", "{{range .Config.Env}}{{println .}}{{end}}"],
        text=True,
    )
    return dict(line.split("=", 1) for line in output.splitlines() if "=" in line)


def run_ops(container: str, operation: str, tenant_id: str, body: dict | None = None) -> dict:
    tenant_environment = ["-e", f"IAM_TENANT_ID={tenant_id}"]
    if body is None:
        output = subprocess.check_output(
            [
                "docker",
                "exec",
                *tenant_environment,
                container,
                "/app/ops",
                "party-migration-preflight",
                "--tenant-id",
                tenant_id,
            ],
            text=True,
        )
    else:
        process = subprocess.run(
            [
                "docker",
                "exec",
                "-i",
                *tenant_environment,
                container,
                "/app/ops",
                operation,
                "--input",
                "/dev/stdin",
            ],
            input=json.dumps(body, ensure_ascii=False),
            text=True,
            capture_output=True,
            check=False,
        )
        if process.returncode != 0:
            raise RuntimeError(f"{container} {operation} failed: {process.stderr.strip()}")
        output = process.stdout
    return json.loads(output)


def load_login_credentials(identity: dict[str, str], credentials_file: str | None) -> dict[str, str]:
    if credentials_file is None:
        return {
            "username": identity["BOOTSTRAP_ADMIN_USERNAME"],
            "password": identity["BOOTSTRAP_ADMIN_PASSWORD"],
        }

    path = Path(credentials_file)
    mode = path.stat().st_mode
    if not path.is_file() or mode & 0o077:
        raise RuntimeError("login credentials file must be a regular file readable only by its owner")
    payload = json.loads(path.read_text(encoding="utf-8"))
    username = str(payload.get("username", "")).strip()
    password = str(payload.get("password", ""))
    if not username or not password:
        raise RuntimeError("login credentials file must contain non-empty username and password fields")
    return {"username": username, "password": password}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:18184")
    parser.add_argument("--identity-container", required=True)
    parser.add_argument("--contract-container", required=True)
    parser.add_argument("--money-container", required=True)
    parser.add_argument(
        "--operating-party-id",
        "--qualification-party-id",
        dest="operating_party_id",
        required=True,
    )
    parser.add_argument("--login-credentials-file")
    parser.add_argument("--expected-tenant-id")
    args = parser.parse_args()
    if args.login_credentials_file and not args.expected_tenant_id:
        parser.error("--login-credentials-file requires --expected-tenant-id")

    identity = container_environment(args.identity_container)
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
    )

    def request(path: str, method="GET", body: bytes | None = None, headers=None):
        try:
            with client.open(
                urllib.request.Request(
                    args.base_url.rstrip("/") + path,
                    data=body,
                    headers={"Origin": args.base_url.rstrip("/"), **(headers or {})},
                    method=method,
                ),
                timeout=30,
            ) as response:
                return response.status, response.read(), response.headers
        except urllib.error.HTTPError as error:
            error.read()
            raise RuntimeError(f"{method} {path} returned HTTP {error.code}") from None

    _, raw, _ = request("/auth/api/v1/csrf")
    csrf = json.loads(raw)["csrf_token"]
    login = json.dumps(load_login_credentials(identity, args.login_credentials_file)).encode()
    _, raw, _ = request(
        "/auth/api/v1/login",
        "POST",
        login,
        {"Content-Type": "application/json", "X-CSRF-Token": csrf},
    )
    principal = json.loads(raw)["principal"]
    tenant_id = principal["tenant_id"]
    reviewer_id = principal["subject_id"]
    reviewer_name = principal["display_name"]
    if args.expected_tenant_id and tenant_id != args.expected_tenant_id:
        raise RuntimeError(
            f"authenticated tenant {tenant_id} does not match expected tenant {args.expected_tenant_id}"
        )

    def authorize(application: str) -> None:
        request(
            "/gateway/authorize/" + application,
            "POST",
            urllib.parse.urlencode({"csrf_token": csrf}).encode(),
            {"Content-Type": "application/x-www-form-urlencoded", "X-CSRF-Token": csrf},
        )

    authorize("party-directory")
    _, raw, _ = request("/party/api/v1/parties/" + urllib.parse.quote(args.operating_party_id))
    operating_target = json.loads(raw)
    if operating_target.get("status") != "active":
        raise RuntimeError("operating Party must be active")

    def load_directory_parties() -> list[dict]:
        parties: list[dict] = []
        cursor = None
        seen = set()
        while True:
            query = {"limit": "100", "status": "active"}
            if cursor:
                query["cursor"] = cursor
            _, page_raw, _ = request("/party/api/v1/parties?" + urllib.parse.urlencode(query))
            page = json.loads(page_raw)
            parties.extend(page.get("items", []))
            cursor = compact(page.get("next_cursor"))
            if cursor is None:
                return parties
            if cursor in seen:
                raise RuntimeError("Party Directory returned a repeated cursor")
            seen.add(cursor)

    directory_parties = load_directory_parties()

    def create_directory_party(local: dict) -> dict:
        payload = json.dumps(
            {
                "command_id": str(uuid.uuid4()),
                "kind": local["kind"],
                "legal_name": local["legal_name"],
                "registration_identifier": compact(local.get("registration_identifier")),
            },
            ensure_ascii=False,
        ).encode()
        _, created_raw, _ = request(
            "/party/api/v1/parties",
            "POST",
            payload,
            {"Content-Type": "application/json", "X-CSRF-Token": csrf},
        )
        created = json.loads(created_raw)
        if created.get("status") != "active" or int(created.get("revision", 0)) < 1:
            raise RuntimeError(f"Party Directory returned an invalid created Party for {local['local_party_id']}")
        directory_parties.append(created)
        return created

    applications = [
        {
            "id": "contract-ledger",
            "container": args.contract_container,
            "context": "/contract/api/v1/contracts/workspace-context",
            "mapping_format": "omsbizfoundry.contract-party-mapping-review/v1",
            "transition_format": "omsbizfoundry.contract-party-runtime-transition/v1",
        },
        {
            "id": "money-flow-ledger",
            "container": args.money_container,
            "context": "/money/api/v1/cash-workspace",
            "mapping_format": "omsbizfoundry.money-flow-party-mapping-review/v1",
            "transition_format": "omsbizfoundry.money-flow-party-runtime-transition/v1",
        },
    ]
    results = []
    for application in applications:
        authorize(application["id"])
        _, raw, _ = request(application["context"])
        context = json.loads(raw)
        if context.get("party_runtime_mode") == "directory" and context.get("directory_authority_established") is True:
            results.append({"application": application["id"], "mode": "directory", "changed": False})
            continue

        preflight = run_ops(
            application["container"], "party-migration-preflight", tenant_id
        )
        items = preflight.get("items", [])
        if not preflight.get("ready_for_review") or not items:
            raise RuntimeError(f"{application['id']} Party preflight is not ready for explicit review")

        batch_id = str(uuid.uuid4())
        decisions = []
        targets = set()
        created_count = 0
        reused_count = 0
        for local in items:
            target = select_directory_party(local, directory_parties)
            decision = "map_existing_directory_party"
            if target is None:
                target = create_directory_party(local)
                decision = "create_directory_party"
                created_count += 1
            else:
                reused_count += 1
            if target["id"] in targets:
                raise RuntimeError(
                    f"{application['id']} would map multiple local Parties to target {target['id']}"
                )
            targets.add(target["id"])
            decisions.append(
                {
                    "command_id": str(uuid.uuid4()),
                    "local_party_id": local["local_party_id"],
                    "local_snapshot_sha256": local["local_snapshot_sha256"],
                    "decision": decision,
                    "target_party_id": target["id"],
                    "target_party_revision": target["revision"],
                    "reason": (
                        "OMS 一期历史主体迁移：按精确登记标识、唯一名称或显式 D1 测试后缀完成复核；"
                        "无可信对应项时建立独立 Party"
                    ),
                }
            )
        mapping = {
            "format": application["mapping_format"],
            "tenant_id": tenant_id,
            "migration_batch_id": batch_id,
            "reviewed_by_subject_id": reviewer_id,
            "reviewed_by_display_name": reviewer_name,
            "mappings": decisions,
        }
        mapped = run_ops(application["container"], "party-migration-map", tenant_id, mapping)
        transition = {
            "format": application["transition_format"],
            "tenant_id": tenant_id,
            "command_id": str(uuid.uuid4()),
            "action": "cutover",
            "migration_batch_id": batch_id,
            "reviewed_by_subject_id": reviewer_id,
            "reviewed_by_display_name": reviewer_name,
            "reason": "OMS 一期使用 Party Directory 精确主体引用",
        }
        cutover = run_ops(
            application["container"], "party-runtime-transition", tenant_id, transition
        )
        results.append(
            {
                "application": application["id"],
                "mode": cutover["state"]["mode"],
                "changed": True,
                "mapped_party_count": mapped["recorded"] + mapped["replayed"],
                "referenced_party_count": cutover["referenced_party_count"],
                "created_directory_party_count": created_count,
                "reused_directory_party_count": reused_count,
            }
        )

    print(json.dumps({"passed": True, "tenant_id": tenant_id, "results": results}, ensure_ascii=False, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        raise SystemExit(f"Party runtime preparation failed: {type(error).__name__}: {error}")
