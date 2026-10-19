#!/usr/bin/env python3
"""Create and verify one complete OMS phase-1 public write journey.

The script uses the same public Nginx -> Gateway -> IAM -> Owner routes as the
browser.  Commands are deterministic for a supplied run key, so an interrupted
run can be repeated without creating a second project or a second money fact.
It never writes an OMS-owned table: Party, Project, Contract, Billing, Money and
Evidence each receive only their own versioned command.
"""

from __future__ import annotations

import argparse
import base64
from decimal import Decimal, InvalidOperation
import hashlib
import http.cookiejar
import json
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


PUBLIC = "https://oatest.antuinfo.cn"
QUALIFICATION_PARTY_ID = "95d3793f-7e0d-4ec9-acb8-9baf8bcb1039"
PNG = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg=="
)


def iam_environment() -> dict[str, str]:
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


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-key", default="oms-phase1-public-20261013-a")
    parser.add_argument("--business-date", default="2026-10-13")
    args = parser.parse_args()
    run_key = args.run_key.strip()
    if not run_key or len(run_key) > 80:
        raise RuntimeError("run key must contain 1-80 characters")

    def command(name: str) -> str:
        return str(uuid.uuid5(uuid.NAMESPACE_URL, f"omsbizfoundry:{run_key}:{name}"))

    environment = iam_environment()
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar)
    )

    def raw_request(
        path_or_url: str,
        method: str = "GET",
        body: bytes | None = None,
        headers: dict[str, str] | None = None,
        expected: tuple[int, ...] = (200,),
    ):
        url = path_or_url if path_or_url.startswith("http") else PUBLIC + path_or_url
        for attempt in range(8):
            try:
                with client.open(
                    urllib.request.Request(
                        url,
                        data=body,
                        headers={"Origin": PUBLIC, **(headers or {})},
                        method=method,
                    ),
                    timeout=60,
                ) as response:
                    content = response.read()
                    if response.status not in expected:
                        raise RuntimeError(f"unexpected HTTP {response.status} for {path_or_url}")
                    return response.status, content, response.headers
            except urllib.error.HTTPError as error:
                detail = error.read().decode("utf-8", "replace")[:500]
                if error.code == 429 and attempt < 7:
                    retry_after = error.headers.get("Retry-After", "")
                    try:
                        delay = max(1.0, min(15.0, float(retry_after)))
                    except ValueError:
                        delay = min(15.0, 2.0 * (attempt + 1))
                    time.sleep(delay)
                    continue
                raise RuntimeError(f"HTTP {error.code} for {path_or_url}: {detail}") from None
        raise RuntimeError(f"rate-limit retry budget exhausted for {path_or_url}")

    def api(
        path: str,
        method: str = "GET",
        payload: dict | None = None,
        expected: tuple[int, ...] = (200,),
        headers: dict[str, str] | None = None,
    ):
        body = None
        request_headers = dict(headers or {})
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode()
            request_headers["Content-Type"] = "application/json"
        _, content, _ = raw_request(path, method, body, request_headers, expected)
        return {} if not content else json.loads(content)

    def cursor_collection(path: str, params: dict[str, str] | None = None) -> list[dict]:
        """Read a complete cursor-paginated Owner collection through Gateway."""
        cursor = ""
        items: list[dict] = []
        seen_ids: set[str] = set()
        seen_cursors: set[str] = set()
        for _ in range(10_000):
            query = {**(params or {}), "limit": "100"}
            if cursor:
                query["cursor"] = cursor
            page = api(path + "?" + urllib.parse.urlencode(query))
            for item in page.get("items") or []:
                item_id = str(item.get("id") or "")
                if not item_id or item_id not in seen_ids:
                    items.append(item)
                    if item_id:
                        seen_ids.add(item_id)
            next_cursor = str(page.get("next_cursor") or "")
            if not next_cursor:
                return items
            if next_cursor in seen_cursors:
                raise RuntimeError(f"cursor collection repeated cursor for {path}")
            seen_cursors.add(next_cursor)
            cursor = next_cursor
        raise RuntimeError(f"cursor collection exceeded safety limit for {path}")

    def offset_collection(path: str, params: dict[str, str] | None = None) -> list[dict]:
        """Read a complete offset-paginated Owner collection through Gateway."""
        offset = 0
        items: list[dict] = []
        seen_ids: set[str] = set()
        for _ in range(10_000):
            query = {**(params or {}), "limit": "100", "offset": str(offset)}
            page = api(path + "?" + urllib.parse.urlencode(query))
            batch = page.get("items") or []
            added = 0
            for item in batch:
                item_id = str(item.get("id") or "")
                if not item_id or item_id not in seen_ids:
                    items.append(item)
                    added += 1
                    if item_id:
                        seen_ids.add(item_id)
            total = page.get("total")
            next_offset = offset + len(batch)
            if not batch or (isinstance(total, int) and next_offset >= total) or page.get("has_more") is False:
                return items
            if added == 0 or next_offset <= offset:
                raise RuntimeError(f"offset collection did not advance for {path}")
            offset = next_offset
        raise RuntimeError(f"offset collection exceeded safety limit for {path}")

    csrf = api("/auth/api/v1/csrf")["csrf_token"]
    principal = api(
        "/auth/api/v1/login",
        "POST",
        {
            "username": environment["BOOTSTRAP_ADMIN_USERNAME"],
            "password": environment["BOOTSTRAP_ADMIN_PASSWORD"],
        },
        headers={"X-CSRF-Token": csrf},
    )["principal"]
    if not principal.get("administrator"):
        raise RuntimeError("public write journey requires the deployment administrator")

    applications = [
        "party-directory",
        "project-workspace",
        "contract-ledger",
        "billing-invoice",
        "money-flow-ledger",
        "evidence-service",
    ]
    for application in applications:
        raw_request(
            "/gateway/authorize/" + application,
            "POST",
            urllib.parse.urlencode({"csrf_token": csrf}).encode(),
            {
                "Content-Type": "application/x-www-form-urlencoded",
                "X-CSRF-Token": csrf,
            },
        )

    qualification = api("/party/api/v1/parties/" + QUALIFICATION_PARTY_ID)
    if qualification.get("status") != "active":
        raise RuntimeError("qualification Party is not active")

    suffix = command("suffix").split("-")[0].upper()
    project_number = f"OMS-PUBLIC-{args.business_date.replace('-', '')}-{suffix}"
    contract_number = f"OMS-UP-{args.business_date.replace('-', '')}-{suffix}"
    downstream_number = f"OMS-DOWN-{args.business_date.replace('-', '')}-{suffix}"

    client_party = api(
        "/party/api/v1/parties",
        "POST",
        {
            "command_id": command("client-party"),
            "kind": "organization",
            "legal_name": f"OMS 一期公网验收甲方 {suffix}",
            "registration_identifier": f"OMS-PUBLIC-CLIENT-{suffix}",
        },
        (200, 201),
    )
    contractor_party = api(
        "/party/api/v1/parties",
        "POST",
        {
            "command_id": command("contractor-party"),
            "kind": "organization",
            "legal_name": f"OMS 一期公网验收施工方 {suffix}",
            "registration_identifier": f"OMS-PUBLIC-CONTRACTOR-{suffix}",
        },
        (200, 201),
    )

    project = api(
        "/project/api/v1/projects",
        "POST",
        {
            "command_id": command("project"),
            "project_number": project_number,
            "name": f"OMS 一期公网闭环验收项目 {suffix}",
            "summary": "公网正式入口六 Owner 纵向闭环验收；仅为测试数据",
            "planned_start_date": args.business_date + "T00:00:00Z",
            "planned_end_date": None,
            "project_type_keys": ["operations"],
            "client_party": {
                "party_id": client_party["id"],
                "source_revision": client_party["revision"],
            },
        },
        (200, 201),
    )

    def create_contract(name: str, number: str, title: str, category: str, amount: str) -> tuple[str, int]:
        result = api(
            "/contract/api/v1/contracts",
            "POST",
            {
                "command_id": command(name + "-contract"),
                "number_namespace": "oms-v1-public",
                "contract_number": number,
                "title": title,
                "category": category,
                "currency": "CNY",
                "total_amount": amount,
                "signed_on": args.business_date,
                "effective_on": args.business_date,
            },
            (200, 201),
        )
        receipt = result["receipt"]
        workspace = api(f"/contract/api/v1/contracts/{receipt['contract_id']}/workspace")
        return receipt["contract_id"], int(workspace["contract"]["draft_version"])

    def link_contract_party(contract_id: str, draft_version: int, name: str, party: dict, role: str) -> int:
        workspace = api(f"/contract/api/v1/contracts/{contract_id}/workspace")
        active = [item for item in workspace.get("parties", []) if item.get("active")]
        if any(
            (item.get("party_reference") or {}).get("party_id") == party["id"]
            and item.get("role") == role
            for item in active
        ):
            return int(workspace["contract"]["draft_version"])
        draft_version = int(workspace["contract"]["draft_version"])
        result = api(
            f"/contract/api/v1/contracts/{contract_id}/parties",
            "POST",
            {
                "command_id": command(name),
                "expected_draft_version": draft_version,
                "party_reference": {"party_id": party["id"], "revision": party["revision"]},
                "role": role,
            },
            (200, 201),
        )
        return int(result["receipt"]["draft_version"])

    def ensure_project_link(
        name: str,
        target_service: str,
        resource_type: str,
        resource_id: str,
        source_revision: int,
        relation_key: str,
        note: str,
    ) -> dict:
        current = api(f"/project/api/v1/projects/{project['id']}")
        if any(
            not item.get("unlinked_at")
            and item.get("target_service") == target_service
            and item.get("resource_type") == resource_type
            and item.get("resource_id") == resource_id
            and item.get("relation_key") == relation_key
            for item in current.get("resource_links", [])
        ):
            return current["project"]
        result = api(
            f"/project/api/v1/projects/{project['id']}/resource-links",
            "POST",
            {
                "command_id": command(name),
                "expected_revision": current["project"]["revision"],
                "operation": "link",
                "link_id": "",
                "target_service": target_service,
                "resource_type": resource_type,
                "resource_id": resource_id,
                "source_revision": source_revision,
                "relation_key": relation_key,
                "note": note,
            },
        )
        return result["project"]

    upstream_id, upstream_version = create_contract(
        "upstream", contract_number, project["name"] + "甲方合同", "upstream-service", "8000000.00"
    )
    upstream_version = link_contract_party(
        upstream_id, upstream_version, "upstream-client", client_party, "甲方"
    )
    upstream_version = link_contract_party(
        upstream_id, upstream_version, "upstream-owner", qualification, "资质单位"
    )
    project = ensure_project_link(
        "project-upstream-contract",
        "contract-ledger",
        "contract",
        upstream_id,
        upstream_version,
        "primary-contract",
        "OMS 一期公网验收甲方合同",
    )

    downstream_id, downstream_version = create_contract(
        "downstream",
        downstream_number,
        project["name"] + "施工协议",
        "downstream-service",
        "2680000.00",
    )
    downstream_version = link_contract_party(
        downstream_id, downstream_version, "downstream-contractor", contractor_party, "施工方"
    )
    downstream_version = link_contract_party(
        downstream_id, downstream_version, "downstream-owner", qualification, "资质单位"
    )
    project = ensure_project_link(
        "project-contractor-party",
        "party-directory",
        "party",
        contractor_party["id"],
        contractor_party["revision"],
        "contractor-party",
        "OMS 一期公网验收施工方",
    )
    project = ensure_project_link(
        "project-downstream-contract",
        "contract-ledger",
        "contract",
        downstream_id,
        downstream_version,
        "downstream-contract",
        "OMS 一期公网验收施工方合同",
    )

    project_source = {
        "kind": "manual_confirmation",
        "manual_summary": f"OMS项目 {project['id']} · {project['name']}",
    }

    def create_invoice(name: str, incoming: bool, amount_minor: int, counterparty: dict, note: str) -> dict:
        issuer = counterparty if incoming else qualification
        recipient = qualification if incoming else counterparty
        return api(
            "/billing/api/v1/invoice-registrations",
            "POST",
            {
                "command_id": command(name),
                "issued_on": args.business_date,
                "amount_minor": amount_minor,
                "currency": "CNY",
                "issuer_party_ref": {"id": issuer["id"], "revision": issuer["revision"]},
                "recipient_party_ref": {"id": recipient["id"], "revision": recipient["revision"]},
                "source_refs": [project_source],
                "note": note,
            },
            (200, 201),
        )

    outgoing_invoice = create_invoice(
        "outgoing-invoice", False, 320000000, client_party, "公网验收：资质单位向甲方开票 320 万"
    )
    incoming_invoice = create_invoice(
        "incoming-invoice", True, 96000000, contractor_party, "公网验收：施工方发票 96 万"
    )

    def create_cash(name: str, receipt: bool, amount: str, counterparty: dict, note: str) -> dict:
        result = api(
            "/money/api/v1/cash-transactions",
            "POST",
            {
                "command_id": command(name),
                "type": "receipt" if receipt else "payment",
                "ledger_owner_party_id": "",
                "counterparty_party_id": "",
                "ledger_owner_party_reference": {
                    "party_id": qualification["id"],
                    "revision": qualification["revision"],
                },
                "counterparty_party_reference": {
                    "party_id": counterparty["id"],
                    "revision": counterparty["revision"],
                },
                "description": note,
                "amount": amount,
                "currency": "CNY",
                "occurred_on": args.business_date,
                "account_reference": "公网验收银行回单",
                "manual_source_reference": "project:" + project["id"],
                "external_transaction_reference": "",
            },
            (200, 201),
        )
        return result["transaction"]

    receipt = create_cash("receipt", True, "2200000.00", client_party, "公网验收：甲方分批回款 220 万")
    payment = create_cash("payment", False, "1200000.00", contractor_party, "公网验收：向施工方支付 120 万")

    def upload(label: str, owner_ref: str, purpose: str, existing_refs: list[dict]) -> dict:
        digest = hashlib.sha256(PNG).hexdigest()
        display_name = label + ".png"
        existing = next(
            (
                item
                for item in existing_refs
                if item.get("display_name") == display_name and item.get("purpose") == purpose
            ),
            None,
        )
        if existing is not None:
            return {
                "file_id": existing["file_id"],
                "version": int(existing["version"]),
                "purpose": purpose,
                "display_name": display_name,
                "sha256": digest,
            }
        create_key = command("evidence-upload-" + label)
        session = api(
            "/evidence/v1/upload-sessions",
            "POST",
            {
                "file_name": display_name,
                "size_bytes": len(PNG),
                "media_type": "image/png",
                "sha256": digest,
                "client_context": {"owner_ref": owner_ref, "purpose": purpose},
            },
            (200, 201),
            {"Idempotency-Key": create_key},
        )
        authorization = session["authorization"]
        upload_headers = dict(authorization.get("headers") or {})
        if "Content-Type" not in upload_headers:
            upload_headers["Content-Type"] = "image/png"
        raw_request(
            urllib.parse.urljoin(PUBLIC, authorization["upload_url"]),
            authorization.get("method", "PUT"),
            PNG,
            upload_headers,
            (200, 201, 204),
        )
        version = api(
            f"/evidence/v1/upload-sessions/{session['session_id']}/complete",
            "POST",
            {"observed_size_bytes": len(PNG), "observed_sha256": digest},
            (200, 201),
            {"Idempotency-Key": command("evidence-complete-" + label)},
        )
        if version.get("status") != "available":
            raise RuntimeError(f"Evidence file {label} is not available")
        return {
            "file_id": version["file_id"],
            "version": int(version["version"]),
            "purpose": purpose,
            "display_name": display_name,
            "sha256": digest,
        }

    upstream_existing = api(f"/contract/api/v1/contracts/{upstream_id}/evidence")["evidence"].get("evidence_refs", [])
    downstream_existing = api(f"/contract/api/v1/contracts/{downstream_id}/evidence")["evidence"].get("evidence_refs", [])
    outgoing_existing = api(f"/billing/api/v1/invoice-registrations/{outgoing_invoice['id']}").get("evidence_refs", [])
    incoming_existing = api(f"/billing/api/v1/invoice-registrations/{incoming_invoice['id']}").get("evidence_refs", [])
    receipt_existing = api(f"/money/api/v1/cash-transactions/{receipt['id']}/evidence")["evidence"].get("evidence_refs", [])
    payment_existing = api(f"/money/api/v1/cash-transactions/{payment['id']}/evidence")["evidence"].get("evidence_refs", [])
    evidence = {
        "upstream": upload("甲方合同", "contract-ledger:" + upstream_id, "contract_document", upstream_existing),
        "downstream": upload("施工方合同", "contract-ledger:" + downstream_id, "contractor_agreement", downstream_existing),
        "outgoing": upload("甲方发票", "billing-invoice:" + outgoing_invoice["id"], "invoice_document", outgoing_existing),
        "incoming": upload("施工方发票", "billing-invoice:" + incoming_invoice["id"], "invoice_document", incoming_existing),
        "receipt": upload("回款凭证", "money-flow-ledger:" + receipt["id"], "receipt_proof", receipt_existing),
        "payment": upload("支付凭证", "money-flow-ledger:" + payment["id"], "payment_proof", payment_existing),
    }

    def business_ref(value: dict) -> dict:
        return {key: value[key] for key in ("file_id", "version", "purpose", "display_name")}

    def replace_contract_evidence(label: str, contract_id: str) -> None:
        current = api(f"/contract/api/v1/contracts/{contract_id}/evidence")["evidence"]
        ref = business_ref(evidence[label])
        if any(item.get("file_id") == ref["file_id"] for item in current.get("evidence_refs", [])):
            return
        api(
            f"/contract/api/v1/contracts/{contract_id}/evidence",
            "PUT",
            {
                "command_id": command("attach-" + label),
                "expected_revision": int(current.get("revision", 0)),
                "reason": "OMS 一期公网闭环验收附件",
                "evidence_refs": [*current.get("evidence_refs", []), ref],
            },
        )

    replace_contract_evidence("upstream", upstream_id)
    replace_contract_evidence("downstream", downstream_id)

    def replace_invoice_evidence(label: str, invoice_id: str) -> None:
        current = api(f"/billing/api/v1/invoice-registrations/{invoice_id}")
        ref = business_ref(evidence[label])
        if any(item.get("file_id") == ref["file_id"] for item in current.get("evidence_refs", [])):
            return
        api(
            f"/billing/api/v1/invoice-registrations/{invoice_id}/evidence",
            "PUT",
            {
                "command_id": command("attach-" + label),
                "expected_revision": int(current["revision"]),
                "reason": "OMS 一期公网闭环验收附件",
                "evidence_refs": [*current.get("evidence_refs", []), ref],
            },
        )

    replace_invoice_evidence("outgoing", outgoing_invoice["id"])
    replace_invoice_evidence("incoming", incoming_invoice["id"])

    def replace_cash_evidence(label: str, transaction_id: str) -> None:
        current = api(f"/money/api/v1/cash-transactions/{transaction_id}/evidence")["evidence"]
        ref = business_ref(evidence[label])
        if any(item.get("file_id") == ref["file_id"] for item in current.get("evidence_refs", [])):
            return
        api(
            f"/money/api/v1/cash-transactions/{transaction_id}/evidence",
            "PUT",
            {
                "command_id": command("attach-" + label),
                "expected_revision": int(current.get("revision", 0)),
                "reason": "OMS 一期公网闭环验收附件",
                "evidence_refs": [*current.get("evidence_refs", []), ref],
            },
        )

    replace_cash_evidence("receipt", receipt["id"])
    replace_cash_evidence("payment", payment["id"])

    # Read every fact back through its public Owner and verify exact money text.
    project_read = api(f"/project/api/v1/projects/{project['id']}")
    upstream_read = api(f"/contract/api/v1/contracts/{upstream_id}/workspace")
    downstream_read = api(f"/contract/api/v1/contracts/{downstream_id}/workspace")
    outgoing_read = api(f"/billing/api/v1/invoice-registrations/{outgoing_invoice['id']}")
    incoming_read = api(f"/billing/api/v1/invoice-registrations/{incoming_invoice['id']}")
    receipt_read = api(f"/money/api/v1/cash-transactions/{receipt['id']}")["transaction"]
    payment_read = api(f"/money/api/v1/cash-transactions/{payment['id']}")["transaction"]

    def exact_decimal(value, expected: str) -> bool:
        try:
            return Decimal(str(value)) == Decimal(expected)
        except InvalidOperation:
            return False

    if not exact_decimal(upstream_read["contract"]["total_amount"], "8000000.00"):
        raise RuntimeError("upstream Contract amount mismatch")
    if not exact_decimal(downstream_read["contract"]["total_amount"], "2680000.00"):
        raise RuntimeError("downstream Contract amount mismatch")
    if outgoing_read.get("amount_minor_decimal") != "320000000":
        raise RuntimeError("outgoing invoice amount mismatch")
    if incoming_read.get("amount_minor_decimal") != "96000000":
        raise RuntimeError("incoming invoice amount mismatch")
    if not exact_decimal(receipt_read.get("amount"), "2200000.00") or receipt_read.get("type") != "receipt":
        raise RuntimeError("receipt amount or type mismatch")
    if not exact_decimal(payment_read.get("amount"), "1200000.00") or payment_read.get("type") != "payment":
        raise RuntimeError("payment amount or type mismatch")

    # Prove that the same facts are discoverable through the complete filtered
    # collection paths used by the Workbench, not only through direct-ID reads.
    collection_reads = {
        "party": cursor_collection("/party/api/v1/parties", {"status": "all", "query": suffix}),
        "project": offset_collection(
            "/project/api/v1/project-directory", {"status": "all", "keyword": suffix}
        ),
        "billing": cursor_collection(
            "/billing/api/v1/invoice-registrations",
            {
                "party_id": qualification["id"],
                "issued_from": args.business_date,
                "issued_to": args.business_date,
            },
        ),
        "money": offset_collection(
            "/money/api/v1/cash-transactions",
            {"status": "all", "source_reference": "project:" + project["id"]},
        ),
    }
    expected_collection_ids = {
        "party": {client_party["id"], contractor_party["id"]},
        "project": {project["id"]},
        "billing": {outgoing_invoice["id"], incoming_invoice["id"]},
        "money": {receipt["id"], payment["id"]},
    }
    for name, expected_ids in expected_collection_ids.items():
        actual_ids = {str(item.get("id") or "") for item in collection_reads[name]}
        if not expected_ids.issubset(actual_ids):
            raise RuntimeError(f"{name} filtered collection is missing created facts")

    active_links = [item for item in project_read.get("resource_links", []) if not item.get("unlinked_at")]
    required_links = {
        ("contract-ledger", "contract", upstream_id, "primary-contract"),
        ("party-directory", "party", contractor_party["id"], "contractor-party"),
        ("contract-ledger", "contract", downstream_id, "downstream-contract"),
    }
    actual_links = {
        (item.get("target_service"), item.get("resource_type"), item.get("resource_id"), item.get("relation_key"))
        for item in active_links
    }
    if not required_links.issubset(actual_links):
        raise RuntimeError("Project resource links are incomplete")

    owner_refs = {
        "upstream": api(f"/contract/api/v1/contracts/{upstream_id}/evidence")["evidence"]["evidence_refs"],
        "downstream": api(f"/contract/api/v1/contracts/{downstream_id}/evidence")["evidence"]["evidence_refs"],
        "outgoing": outgoing_read["evidence_refs"],
        "incoming": incoming_read["evidence_refs"],
        "receipt": api(f"/money/api/v1/cash-transactions/{receipt['id']}/evidence")["evidence"]["evidence_refs"],
        "payment": api(f"/money/api/v1/cash-transactions/{payment['id']}/evidence")["evidence"]["evidence_refs"],
    }
    for label, ref in evidence.items():
        if not any(item.get("file_id") == ref["file_id"] for item in owner_refs[label]):
            raise RuntimeError(f"{label} Owner evidence reference is missing")
        descriptor = api(
            f"/evidence/v1/references/files/{ref['file_id']}/versions/{ref['version']}"
        )
        if descriptor.get("status") != "available":
            raise RuntimeError(f"{label} Evidence descriptor is not available")
        grant = api(
            f"/evidence/v1/files/{ref['file_id']}/versions/{ref['version']}/access-grants",
            "POST",
            {"disposition": "attachment", "delivery_mode": "direct"},
        )
        _, downloaded, _ = raw_request(
            urllib.parse.urljoin(PUBLIC, grant["url"]),
            grant.get("method", "GET"),
            headers=grant.get("headers") or {},
        )
        if hashlib.sha256(downloaded).hexdigest() != ref["sha256"]:
            raise RuntimeError(f"{label} downloaded bytes changed")

    api("/auth/api/v1/logout", "POST", expected=(204,), headers={"X-CSRF-Token": csrf})
    print(
        json.dumps(
            {
                "passed": True,
                "run_key": run_key,
                "tenant_id": principal["tenant_id"],
                "parties": {"client": client_party["id"], "contractor": contractor_party["id"]},
                "project": {
                    "id": project["id"],
                    "number": project_number,
                    "revision": project_read["project"]["revision"],
                },
                "contracts": {
                    "upstream": {"id": upstream_id, "amount": "8000000.00"},
                    "downstream": {"id": downstream_id, "amount": "2680000.00"},
                },
                "invoices": {
                    "outgoing": {"id": outgoing_invoice["id"], "amount_minor": "320000000"},
                    "incoming": {"id": incoming_invoice["id"], "amount_minor": "96000000"},
                },
                "cash": {
                    "receipt": {"id": receipt["id"], "amount": "2200000.00"},
                    "payment": {"id": payment["id"], "amount": "1200000.00"},
                },
                "evidence": {
                    label: {"file_id": value["file_id"], "version": value["version"], "sha256": value["sha256"]}
                    for label, value in evidence.items()
                },
                "verified_owner_reads": applications,
                "verified_filtered_collections": {
                    name: len(items) for name, items in collection_reads.items()
                },
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
            f"public OMS write journey failed at line {line}: {type(error).__name__}: {error}"
        )
