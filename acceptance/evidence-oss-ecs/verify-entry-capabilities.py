#!/usr/bin/env python3
"""Real IAM -> Gateway -> Evidence admission, in one private bounded container.

Never emits passwords, tokens, signed URLs, application logs or database URLs.
No main configuration, IAM grants, files, containers or restore20 are changed.
Fresh restricted roles/databases and diagnostic artifacts are retained; the
test container is always stopped, never deleted. Not public Nginx/browser/C8 QA.
"""
import base64
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

PG = "omsbizfoundry-postgres-1"
NAME = os.environ.get("ENTRY_FIXTURE_NAME", "entry_capability_auth58_20261001a")
ROOT = Path("/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001")
WORK = ROOT / NAME
TENANT = "77777777-7777-4777-8777-777777777777"
ISSUER = "https://entry-fixture.example.test/auth"
BASE = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/"
IAM_IMAGE = BASE + "identity-access@sha256:69806b6fbf055dcdbb50dcc93a135149e0a523684213ef4607bcc1c21a006266"
EVIDENCE_IMAGE = BASE + "evidence-service@sha256:4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf"
RUNTIME = BASE + "base-postgres@sha256:d8703cd7fba306b9fec9268ecedfa8a966846c053036a60e3635791957eb2f66"


def run(args, data=None):
    result = subprocess.run(args, input=data, capture_output=True, timeout=180)
    if result.returncode:
        raise RuntimeError("command failed: " + " ".join(args[:2]))
    return result.stdout.decode().strip()


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def sql(db, statement):
    return run(["docker", "exec", "-i", PG, "psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-U", "oms_admin", "-d", db], statement.encode())


def save(path, data, mode=0o600):
    with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, mode), "wb") as output:
        output.write(data)


def mark(name, **values):
    value = {"check": name, "result": "pass", **values}
    events.append(value)
    print(json.dumps(value), flush=True)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def opener():
    jar = http.cookiejar.CookieJar()
    return urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect(), urllib.request.HTTPCookieProcessor(jar)), jar


def request(client, method, url, value=None, headers=None, expected=200):
    headers = dict(headers or {})
    if value is not None and not isinstance(value, bytes):
        value = json.dumps(value).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, value, headers, method=method)
    try:
        with client.open(req, timeout=20) as response:
            status, body, returned = response.status, response.read(), response.headers
    except urllib.error.HTTPError as error:
        status, body, returned = error.code, error.read(), error.headers
    if status != expected:
        # No response or URL output: both can contain credentials.
        raise RuntimeError("HTTP " + str(status) + ", expected " + str(expected) + " for " + method)
    return body, returned


def api(client, method, path, value=None, csrf=None, expected=200):
    headers = {"X-CSRF-Token": csrf} if csrf else {}
    if method == "POST" and path.startswith("/evidence/"):
        headers["Idempotency-Key"] = str(uuid.uuid4())
    body, _ = request(client, method, gateway + path, value, headers, expected)
    return json.loads(body) if body else None


def login(username):
    client, jar = opener()
    csrf = api(client, "GET", "/auth/api/v1/csrf")["csrf_token"]
    principal = api(client, "POST", "/auth/api/v1/login", {"username": username, "password": password}, csrf)["principal"]
    assert principal["tenant_id"] == TENANT
    return client, jar, csrf


def authorize(client, jar, csrf, file_id=None, expected=303):
    fields = {"csrf_token": csrf}
    if file_id:
        fields.update(resource_type="file", resource_id=file_id)
    body, headers = request(client, "POST", gateway + "/gateway/authorize/evidence-service",
                            urllib.parse.urlencode(fields).encode(), {"Content-Type": "application/x-www-form-urlencoded"}, expected)
    caps = [cookie for cookie in jar if cookie.name == "oms_cap_evidence_service"]
    if expected == 303:
        assert len(caps) == 1 and caps[0].path == "/evidence"
        assert "HttpOnly" in headers.get("Set-Cookie", "")
        payload = caps[0].value.split(".")[1]
        claims = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
        assert claims["tid"] == TENANT and 0 < claims["exp"] - time.time() <= 301
        assert "evidence-service.view" not in claims["actions"] or file_id is None
        return claims
    assert not caps, "failed renewal must clear previous capability cookie"
    return None


def facts(db):
    tables = json.loads(sql(db, "SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public';"))
    rows = []
    for table in tables:
        ident = '"' + table.replace('"', '""') + '"'
        rows.append([table, sql(db, "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + ident + " t;")])
    return hashlib.sha256(json.dumps(rows, separators=(",", ":")).encode()).hexdigest()


def snapshot():
    names = ("postgres", "identity-access", "evidence-service", "project-workspace", "people-groups", "party-directory", "contract-ledger", "money-flow-ledger", "business-engagement", "billing-invoice", "production-operations", "approval-audit-results", "cost-outsourcing", "bonus-distribution", "gateway", "unified-workbench", "nginx")
    containers = [inspect("omsbizfoundry-" + name + "-1") for name in names]
    assert all(c["State"].get("Health", {}).get("Status") == "healthy" for c in containers)
    return {"containers": sorted(c["Id"] for c in containers), "facts": {db: facts(db) for db in ("identity_access", "evidence_service", "project_workspace", "people_groups", "party_directory", "contract_ledger", "money_flow", "business_engagement", "billing_invoice", "design_operations", "approval_audit_results", "cost_outsourcing", "bonus_distribution")}}


def main():
    global gateway, password, retained_file
    retained_file = None
    assert NAME in ("entry_capability_auth58_20261001a", "entry_capability_auth58_20261001b", "entry_capability_auth58_20261001c")
    assert os.geteuid() == 0
    reuse = os.environ.get("ENTRY_FIXTURE_REUSE") == "yes"
    finish = os.environ.get("ENTRY_FIXTURE_FINISH") == "yes"
    attempt = os.environ.get("ENTRY_FIXTURE_ATTEMPT", "1")
    assert attempt in ("1", "2")
    if WORK.exists():
        assert reuse and NAME in ("entry_capability_auth58_20261001b", "entry_capability_auth58_20261001c")
        assert not (WORK / ("results-final.json" if finish else "results-resume" + attempt + ".json")).exists()
        info = inspect(NAME)
        assert not info["State"]["Running"] and info["Config"]["Labels"]["oms.purpose"] == "entry-capability-fixture"
        if finish:
            assert NAME.endswith("c")
            previous = json.loads((WORK / "results-resume2.json").read_text())["events"]
            assert [event["check"] for event in previous] == ["real_three_process_fixture_ready", "native_metadata_without_view", "native_download_without_view", "native_combined_without_view", "neighbor_scope_rejected_by_real_evidence", "fixture_stopped_diagnostic_data_retained"]
            assert all(event["result"] == "pass" for event in previous)
            assert sql(NAME + "_iam", "SELECT string_agg(username,',' ORDER BY username) FROM subjects;") == "fixture-admin,fixture-combined,fixture-download,fixture-metadata"
            assert sql(NAME + "_evidence", "SELECT count(*) FROM file_versions;") == "1"
            retained_file = json.loads(sql(NAME + "_evidence", "SELECT json_build_object('file_id',file_id,'version',version,'status',status,'sha256',sha256) FROM file_versions;"))
            assert sql(NAME + "_iam", "SELECT count(*) FROM capability_resource_grants g JOIN subjects s ON s.id=g.subject_id WHERE s.username='fixture-download' AND g.action='evidence-service.download' AND g.resource_id='" + retained_file["file_id"] + "' AND revoked_at IS NOT NULL;") == "1"
            assert sql(NAME + "_iam", "SELECT count(*) FROM capability_resource_grants g JOIN subjects s ON s.id=g.subject_id WHERE s.username='fixture-download' AND revoked_at IS NULL;") == "0"
        elif NAME.endswith("c"):
            # Resume only the exact retained partial fixture: one completed file,
            # two activated narrow actors, and no unrelated grants or subjects.
            assert attempt in ("1", "2")
            assert sql(NAME + "_iam", "SELECT string_agg(username,',' ORDER BY username) FROM subjects;") == "fixture-admin,fixture-download,fixture-metadata"
            assert sql(NAME + "_evidence", "SELECT count(*) FROM file_versions;") == "1"
            retained_file = json.loads(sql(NAME + "_evidence", "SELECT json_build_object('file_id',file_id,'version',version,'status',status,'sha256',sha256) FROM file_versions;"))
            assert retained_file["version"] == 1 and retained_file["status"] == "available"
            narrow = json.loads(sql(NAME + "_iam", "SELECT json_agg(json_build_object('username',s.username,'action',g.action,'type',g.resource_type,'id',g.resource_id) ORDER BY s.username) FROM capability_resource_grants g JOIN subjects s ON s.id=g.subject_id WHERE g.revoked_at IS NULL;"))
            assert narrow == [{"username": "fixture-download", "action": "evidence-service.download", "type": "file", "id": retained_file["file_id"]}, {"username": "fixture-metadata", "action": "evidence-service.metadata.read", "type": "file", "id": retained_file["file_id"]}]
        else:
            assert sql(NAME + "_iam", "SELECT count(*) FROM subjects;") == "1"
            if sql(NAME + "_evidence", "SELECT to_regclass('public.file_versions');"):
                assert sql(NAME + "_evidence", "SELECT count(*) FROM file_versions;") == "0"
            else:
                assert sql(NAME + "_evidence", "SELECT count(*) FROM pg_tables WHERE schemaname='public';") == "0"
    else:
        assert not reuse
    candidate = os.environ["ENTRY_GATEWAY_IMAGE"]
    assert candidate.startswith(BASE + "gateway@sha256:") and len(candidate.rsplit(":", 1)[1]) == 64
    main_evidence = inspect("omsbizfoundry-evidence-service-1")
    main_env = dict(item.split("=", 1) for item in main_evidence["Config"]["Env"])
    assert main_env["STORAGE_PROVIDER"] == "local"
    network = next(n for n in main_evidence["NetworkSettings"]["Networks"] if n.endswith("_data"))
    before = snapshot()
    if reuse:
        assert before == json.loads((WORK / "main-before.json").read_text())
        sources = json.loads((WORK / "source-images.json").read_text())
        assert sources["gateway"]["image"] == candidate
        for kind, source in sources.items():
            assert hashlib.sha256((WORK / "bin" / kind).read_bytes()).hexdigest() == source["binary_sha256"]
        config = dict(line.split("=", 1) for line in (WORK / "config/fixture.env").read_text().splitlines())
        password = config["FIXTURE_PASSWORD"].strip("'")
        return verify_runtime(network, before, reuse=True, finish=finish)
    WORK.mkdir(mode=0o700)
    save(WORK / "main-before.json", json.dumps(before, indent=2).encode())
    for subdir in ("bin", "config", "data"):
        (WORK / subdir).mkdir(mode=0o700)
    password = secrets.token_hex(24)
    config = {"FIXTURE_TENANT": TENANT, "FIXTURE_ISSUER": ISSUER, "FIXTURE_PASSWORD": password,
              "FIXTURE_LOCAL_KEY": base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip("=")}
    for kind in ("iam", "evidence"):
        name = NAME + "_" + kind
        assert not sql("postgres", "SELECT 1 FROM pg_database WHERE datname='" + name + "';")
        assert not sql("postgres", "SELECT 1 FROM pg_roles WHERE rolname='" + name + "';")
        secret = secrets.token_hex(24)
        sql("postgres", 'CREATE ROLE "' + name + '" LOGIN PASSWORD \'' + secret + "';")
        sql("postgres", 'CREATE DATABASE "' + name + '" OWNER "' + name + '";')
        config[kind.upper() + "_DATABASE_URL"] = "postgres://" + name + ":" + secret + "@postgres:5432/" + name + "?sslmode=disable"
    # These generated configurations are private diagnostic artifacts, not source.
    save(WORK / "config/fixture.env", ("\n".join(key + "='" + value + "'" for key, value in config.items()) + "\n").encode())
    save(WORK / "config/evidence-capabilities.json", (ROOT / "evidence-service-capabilities.json").read_bytes())
    suite = {"schema_version": "1", "suite_id": "entry-capability-fixture", "tenant_id": TENANT, "applications": [
        {"id": "identity-access", "audience": "identity-access", "version": "0.1.0", "enabled": True, "required": True, "route_prefix": "/auth", "upstream": "http://127.0.0.1:8080", "readiness_path": "/health/ready"},
        {"id": "evidence-service", "audience": "evidence-service", "version": "0.1.0", "enabled": True, "required": True, "route_prefix": "/evidence", "upstream": "http://127.0.0.1:8085", "readiness_path": "/health/ready", "entry_actions": ["evidence-service.view", "evidence-service.metadata.read", "evidence-service.upload", "evidence-service.download"]}]}
    save(WORK / "config/suite.json", json.dumps(suite).encode())
    sources = {}
    for kind, image in (("iam", IAM_IMAGE), ("evidence", EVIDENCE_IMAGE), ("gateway", candidate)):
        image_meta = json.loads(run(["docker", "image", "inspect", image]))[0]
        assert image in image_meta["RepoDigests"] and image_meta["Architecture"] == "amd64"
        source = NAME + "_source_" + kind
        run(["docker", "create", "--name", source, "--label", "oms.purpose=entry-capability-fixture-source", image])
        run(["docker", "cp", source + ":/app/server", str(WORK / "bin" / kind)])
        os.chmod(WORK / "bin" / kind, 0o500)
        sources[kind] = {"image": image, "binary_sha256": hashlib.sha256((WORK / "bin" / kind).read_bytes()).hexdigest()}
    save(WORK / "source-images.json", json.dumps(sources, indent=2).encode())
    run(["chown", "-R", "70:70", str(WORK)])
    verify_runtime(network, before)


def verify_runtime(network, before, reuse=False, finish=False):
    global gateway
    created = False
    try:
        if reuse:
            run(["docker", "start", NAME])
        else:
            run(["docker", "run", "--detach", "--name", NAME, "--network", network, "--memory", "224m", "--cpus", "0.35", "--pids-limit", "80", "--user", "70:70", "--label", "oms.purpose=entry-capability-fixture", "--mount", "type=bind,src=" + str(WORK / "bin") + ",dst=/probe/bin,readonly", "--mount", "type=bind,src=" + str(WORK / "config") + ",dst=/probe/config,readonly", "--mount", "type=bind,src=" + str(WORK / "data") + ",dst=/probe/data", "--mount", "type=bind,src=" + str(ROOT / "entry-capability-probe.sh") + ",dst=/probe/start.sh,readonly", "--entrypoint", "sh", RUNTIME, "/probe/start.sh"])
        created = True
        address = inspect(NAME)["NetworkSettings"]["Networks"][network]["IPAddress"]
        gateway = "http://" + address + ":8088"
        plain, _ = opener()
        for attempt in range(60):
            try:
                api(plain, "GET", "/health/ready")
                request(plain, "GET", "http://" + address + ":8080/health/ready")
                request(plain, "GET", "http://" + address + ":8085/health/ready")
                break
            except (RuntimeError, urllib.error.URLError):
                if attempt == 59:
                    raise RuntimeError("bounded fixture startup failed; inspect private diagnostics")
                time.sleep(1)
        info = inspect(NAME)
        assert not info["HostConfig"]["PortBindings"] and info["HostConfig"]["NanoCpus"] == 350000000
        mark("real_three_process_fixture_ready", published_ports=0, cpu_limit=0.35, memory_mb=224)
        if finish:
            client, jar, csrf = login("fixture-download")
            authorize(client, jar, csrf, retained_file["file_id"], 403)
            api(client, "POST", "/evidence/v1/files/" + retained_file["file_id"] + "/versions/1/access-grants", {"disposition": "attachment", "delivery_mode": "direct"}, expected=401)
            mark("revoked_renewal_rejected_and_unauthorized_post_closed", immediate_revocation=False, prior_three_actor_matrix_retained=True)
            assert snapshot() == before
            mark("main_thirteen_databases_and_seventeen_container_ids_unchanged", main_evidence_provider="local")
            return
        request(plain, "GET", gateway + "/evidence/v1/files", expected=303)
        admin, _, csrf = login("fixture-admin")
        admin_subject = api(admin, "GET", "/auth/api/v1/session")["principal"]["subject_id"]
        if reuse:
            active = json.loads(sql(NAME + "_iam", "SELECT json_agg(action ORDER BY action) FROM capability_grants WHERE revoked_at IS NULL;"))
            assert active == ["evidence-service.download", "evidence-service.metadata.read", "evidence-service.upload"]
        else:
            for action in ("evidence-service.upload", "evidence-service.metadata.read", "evidence-service.download"):
                api(admin, "POST", "/auth/api/v1/admin/grants", {"subject_id": admin_subject, "audience": "evidence-service", "action": action, "scope": {"kind": "tenant", "resources": []}}, csrf, 204)
        authorize(admin, list_cookie_jar(admin), csrf)
        content = b"synthetic native entry capability acceptance\n"
        digest = hashlib.sha256(content).hexdigest()
        if retained_file:
            version = api(admin, "GET", "/evidence/v1/references/files/" + retained_file["file_id"] + "/versions/1")
            assert version["file_id"] == retained_file["file_id"] and retained_file["sha256"] == digest
        else:
            session = api(admin, "POST", "/evidence/v1/upload-sessions", {"file_name": "entry-capability-fixture.txt", "size_bytes": len(content), "media_type": "text/plain", "sha256": digest, "client_context": {"purpose": "isolated-native-entry-acceptance"}}, expected=201)
            request(plain, "PUT", session["authorization"]["upload_url"], content, {"Content-Type": "text/plain"}, expected=204)
            version = api(admin, "POST", "/evidence/v1/upload-sessions/" + session["session_id"] + "/complete", {"observed_size_bytes": len(content), "observed_sha256": digest})
        assert version["sha256"] == digest and version["version"] == 1
        file_id = version["file_id"]
        scope = {"kind": "resources", "resources": [{"type": "file", "id": file_id}]}
        actors = {}
        for label, actions in (("metadata", ["evidence-service.metadata.read"]), ("download", ["evidence-service.download"]), ("combined", ["evidence-service.metadata.read", "evidence-service.download"])):
            if retained_file and label in ("metadata", "download"):
                subject = sql(NAME + "_iam", "SELECT id FROM subjects WHERE username='fixture-" + label + "';")
            else:
                invited = api(admin, "POST", "/auth/api/v1/admin/invitations", {"username": "fixture-" + label, "display_name": "fixture-" + label}, csrf, 201)
                activation_csrf = api(plain, "GET", "/auth/api/v1/csrf")["csrf_token"]
                api(plain, "POST", "/auth/api/v1/activate", {"token": invited["token"], "password": password}, activation_csrf)
                subject = invited["subject"]["id"]
                for action in actions:
                    api(admin, "POST", "/auth/api/v1/admin/grants", {"subject_id": subject, "audience": "evidence-service", "action": action, "scope": scope}, csrf, 204)
            client, jar, user_csrf = login("fixture-" + label)
            claims = authorize(client, jar, user_csrf, file_id)
            assert sorted(claims["actions"]) == sorted(actions) and claims["scope"] == scope
            api(client, "GET", "/evidence/v1/references/files/" + file_id, expected=403 if label == "download" else 200)
            grant = api(client, "POST", "/evidence/v1/files/" + file_id + "/versions/1/access-grants", {"disposition": "attachment", "delivery_mode": "direct"}, expected=403 if label == "metadata" else 200)
            # A correctly denied HTTP 403 can contain a non-empty error object.
            # Its presence is not an access grant and must not trigger a GET.
            if label != "metadata":
                assert isinstance(grant, dict) and grant.get("url")
                body, _ = request(plain, "GET", grant["url"])
                assert hashlib.sha256(body).hexdigest() == digest
            else:
                assert not grant or "url" not in grant
            actors[label] = (client, jar, user_csrf, subject)
            mark("native_" + label + "_without_view", metadata_allowed=label != "download", download_allowed=label != "metadata", unchanged_actions=True)
        client, jar, user_csrf, subject = actors["combined"]
        neighbor = str(uuid.uuid4())
        neighbor_scope = {"kind": "resources", "resources": [{"type": "file", "id": neighbor}]}
        for action in ("evidence-service.metadata.read", "evidence-service.download"):
            api(admin, "POST", "/auth/api/v1/admin/grants", {"subject_id": subject, "audience": "evidence-service", "action": action, "scope": neighbor_scope}, csrf, 204)
        authorize(client, jar, user_csrf, neighbor)
        api(client, "GET", "/evidence/v1/references/files/" + file_id, expected=403)
        api(client, "POST", "/evidence/v1/files/" + file_id + "/versions/1/access-grants", {"disposition": "attachment", "delivery_mode": "direct"}, expected=403)
        mark("neighbor_scope_rejected_by_real_evidence")
        client, jar, user_csrf, subject = actors["download"]
        api(admin, "POST", "/auth/api/v1/admin/grants/revoke", {"subject_id": subject, "audience": "evidence-service", "action": "evidence-service.download", "scope": scope}, csrf, 204)
        # Published short-lived credentials remain valid until TTL; do not claim immediate revocation.
        api(client, "POST", "/evidence/v1/files/" + file_id + "/versions/1/access-grants", {"disposition": "attachment", "delivery_mode": "direct"})
        authorize(client, jar, user_csrf, file_id, 403)
        api(client, "POST", "/evidence/v1/files/" + file_id + "/versions/1/access-grants", {"disposition": "attachment", "delivery_mode": "direct"}, expected=401)
        mark("revoked_renewal_rejected_and_old_cookie_cleared", immediate_revocation=False)
        assert snapshot() == before
        mark("main_thirteen_databases_and_seventeen_container_ids_unchanged", main_evidence_provider="local")
    finally:
        if created:
            run(["docker", "stop", "--time", "20", NAME])
            assert not inspect(NAME)["State"]["Running"]
            mark("fixture_stopped_diagnostic_data_retained")
        attempt = os.environ.get("ENTRY_FIXTURE_ATTEMPT", "1")
        save(WORK / ("results-final.json" if finish else "results-resume" + attempt + ".json" if reuse else "results.json"), json.dumps({"events": events}, indent=2).encode())


def list_cookie_jar(client):
    for handler in client.handlers:
        if isinstance(handler, urllib.request.HTTPCookieProcessor):
            return handler.cookiejar
    raise AssertionError("missing cookie jar")


events = []
if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        detail = ": " + str(error) if isinstance(error, RuntimeError) else ""
        raise SystemExit("entry acceptance failed: " + type(error).__name__ + " at line " + str(trace.tb_lineno) + detail)
