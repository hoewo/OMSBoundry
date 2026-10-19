#!/usr/bin/env python3
"""Live Local -> OSS -> Local-primary -> mixed restore, confined to new fixtures.

Requires an existing cold LOCAL backup matching current main File Versions.
No stop, write, provider switch, grant change or deletion in the main Suite.
Control API is private probe + actual public IAM, NOT browser/public Evidence QA.
"""
import base64
import email.utils
import hashlib
import hmac
import http.cookiejar
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
import zipfile
import xml.etree.ElementTree as ET


def command(args, env=None):
    result = subprocess.run(args, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=180)
    if result.returncode:
        raise RuntimeError("command failed: " + " ".join(args[:2]) + "; diagnostics suppressed")
    return result.stdout.strip()


def inspect(name):
    return json.loads(command(["docker", "inspect", name]))[0]


def sql(database, query):
    return command(["docker", "exec", "omsbizfoundry-postgres-1", "psql", "-v", "ON_ERROR_STOP=1", "-U", "oms_admin", "-d", database, "-Atc", query])


def canonical(database):
    return sql(database, "SELECT row_to_json(v)::text FROM file_versions v ORDER BY tenant_id,file_id,version;")


def snapshot():
    return sql("evidence_service", "SELECT jsonb_build_object('versions',(SELECT count(*) FROM file_versions),'sessions',(SELECT count(*) FROM upload_sessions),'audit',(SELECT count(*) FROM evidence_audit_entries),'digest',(SELECT md5(string_agg(row_to_json(v)::text,'' ORDER BY tenant_id,file_id,version)) FROM file_versions v));")


def mark(label, **details):
    print(json.dumps({"check": label, "result": "pass", **details}), flush=True)


plain = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def request(method, url, data=None, headers=None, expected=200, opener=None):
    try:
        with (opener or plain).open(urllib.request.Request(url, data=data, headers=headers or {}, method=method), timeout=60) as response:
            status, body, returned = response.status, response.read(), response.headers
    except urllib.error.HTTPError as error:
        status, body, returned = error.code, error.read(), error.headers
    if status != expected:
        code = ""
        try:
            code = ET.fromstring(body).findtext("Code", "")
        except ET.ParseError:
            pass
        if not re.fullmatch(r"[A-Za-z0-9]*", code):
            code = "suppressed"
        raise RuntimeError(f"{method} HTTP {status}, expected {expected}; OSS code={code}; sensitive URL/body suppressed")
    return body, returned


def json_request(method, url, value=None, headers=None, expected=200, opener=None):
    data = None if value is None else json.dumps(value).encode()
    body, _ = request(method, url, data, {**({"Content-Type": "application/json"} if data else {}), **(headers or {})}, expected, opener)
    return json.loads(body) if body else None


image = os.environ["PROBE_IMAGE"]
source = os.environ.get("PROBE_TRANSITION_SOURCE", "evidence_oss_auth58_20261001m1")
target = os.environ.get("PROBE_TRANSITION_TARGET", "evidence_oss_auth58_20261001m2")
network = os.environ.get("PROBE_NETWORK", "evidence_oss_auth58_20261001a")
cold_backup = os.environ.get("PROBE_LOCAL_BACKUP", "/var/backups/omsbizfoundry/20261001T012550Z/evidence-service.zip")
public = os.environ.get("PROBE_PUBLIC_URL", "https://oatest.antuinfo.cn")
files_origin = os.environ.get("PROBE_FILES_ORIGIN", "https://oatestfiles.antuinfo.cn")
for name in (source, target, network):
    if not re.fullmatch(r"evidence_oss_[a-z0-9_]{1,40}", name):
        raise SystemExit("reserved fixture names required")
if source == target or not re.fullmatch(r"[a-z0-9./:_-]+@sha256:[a-f0-9]{64}", image):
    raise SystemExit("distinct targets and digest-pinned image required")
if not cold_backup.startswith("/var/backups/omsbizfoundry/"):
    raise SystemExit("explicit retained cold backup required")
before = snapshot()
main_facts = canonical("evidence_service")
started = []

try:
    deployed = inspect("omsbizfoundry-evidence-service-1")
    env = dict(item.split("=", 1) for item in deployed["Config"]["Env"])
    iam = dict(item.split("=", 1) for item in inspect("omsbizfoundry-identity-access-1")["Config"]["Env"])
    data_network = next(name for name in deployed["NetworkSettings"]["Networks"] if name.endswith("_data"))
    keys = next(item["Name"] for item in deployed["Mounts"] if item["Destination"] == "/public")
    current_url = urllib.parse.urlsplit(env["DATABASE_URL"])
    manifest = json.loads(zipfile.ZipFile(cold_backup).read("manifest.json"))
    assert manifest["storage"]["provider"] == "local"
    assert manifest["database"]["counts"]["versions"] == len(main_facts.splitlines()) == 10
    for name in (source, target):
        assert not sql("postgres", f"SELECT 1 FROM pg_database WHERE datname='{name}';"), "fixture database already exists"
        sql("postgres", f'CREATE DATABASE "{name}" OWNER evidence_service;')
    command(["docker", "pull", image])
    uid, gid = command(["docker", "run", "--rm", "--network", "none", image, "sh", "-c", "id -u; id -g"]).splitlines()
    assert uid.isdecimal() and gid.isdecimal()
    backup_dir = "/var/backups/omsbizfoundry/" + source
    assert not os.path.exists(backup_dir), "fixture backup directory already exists"
    command(["install", "-d", "-m", "0700", "-o", uid, "-g", gid, backup_dir])
    command(["install", "-m", "0600", "-o", uid, "-g", gid, cold_backup, backup_dir + "/source-local.zip"])

    def fixture_env(database, primary, mode="hybrid"):
        values = {**env, "DATABASE_URL": urllib.parse.urlunsplit(current_url._replace(path="/" + database)),
                  "STORAGE_PROVIDER": mode, "PRIMARY_STORAGE_PROVIDER": primary if mode == "hybrid" else "",
                  "PUBLIC_BASE_URL": "http://evidence-probe:8085", "LOCAL_STORAGE_ROOT": "/data/evidence/store",
                  "OSS_REGION": "cn-hangzhou", "OSS_BUCKET": "oatest-auth58-files-1",
                  "OSS_ROLE_ARN": "acs:ram::1169734041157847:role/auth58evidenceuploadrole",
                  "OSS_BROWSER_ENDPOINT": files_origin, "OSS_BROWSER_CNAME": "true",
                  "OSS_SERVICE_ENDPOINT": "https://oss-cn-hangzhou-internal.aliyuncs.com",
                  "OSS_STS_ENDPOINT": "sts.cn-hangzhou.aliyuncs.com", "ALIBABA_CLOUD_ECS_METADATA": "Auth58EvidenceServiceRole"}
        values["POSTGRES_CLIENT_DATABASE_URL"] = values["DATABASE_URL"]
        if mode == "local":
            for key in list(values):
                if key.startswith("OSS_"):
                    values.pop(key)
        return values

    def run(database, primary, arguments, named=None, mode="hybrid"):
        values = fixture_env(database, primary, mode)
        args = ["docker", "run", "--rm" if named is None else "--detach", "--network", data_network,
                "--memory", "192m", "--cpus", "0.5", "--label", "oms.purpose=evidence-oss-acceptance",
                "--mount", f"type=volume,src={database},dst=/data/evidence",
                "--mount", f"type=volume,src={keys},dst=/public,readonly",
                "--mount", f"type=bind,src={backup_dir},dst=/backups"]
        if named:
            args += ["--name", named]
        for key in values:
            args += ["--env", key]
        args += [image, *arguments]
        result = command(args, env={**os.environ, **values})
        if named:
            started.append(named)
            command(["docker", "network", "connect", network, named])
            address = inspect(named)["NetworkSettings"]["Networks"][data_network]["IPAddress"]
            origin = f"http://{address}:8085"
            for attempt in range(45):
                try:
                    ready = json_request("GET", origin + "/health/ready")
                    assert ready["storage_mode"] == "hybrid" and ready["storage_provider"] == primary
                    break
                except (RuntimeError, urllib.error.URLError):
                    if attempt == 44:
                        raise RuntimeError("mixed probe readiness failed; diagnostics suppressed")
                    time.sleep(1)
            assert not inspect(named)["HostConfig"]["PortBindings"]
            return origin
        return result

    run(source, "local", ["/app/backup", "restore", "--input", "/backups/source-local.zip"], mode="local")
    assert canonical(source) == main_facts
    mark("old_local_backup_restore", versions=10, immutable_rows_unchanged=True)
    authenticated = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json_request("GET", public + "/auth/api/v1/csrf", opener=authenticated)["csrf_token"]
    json_request("POST", public + "/auth/api/v1/login", {"username": iam["BOOTSTRAP_ADMIN_USERNAME"], "password": iam["BOOTSTRAP_ADMIN_PASSWORD"]}, {"X-CSRF-Token": csrf}, opener=authenticated)

    def credential():
        return json_request("POST", public + "/auth/api/v1/credentials", {"audience": "evidence-service"}, {"X-CSRF-Token": csrf}, opener=authenticated)["token"]

    def api(origin, method, path, value=None, expected=200):
        return json_request(method, origin + path, value,
                            {"Authorization": "Bearer " + token, **({"Idempotency-Key": str(uuid.uuid4())} if method == "POST" else {})}, expected)

    def local_address(url, origin):
        parsed = urllib.parse.urlsplit(url)
        assert parsed.netloc == "evidence-probe:8085" and parsed.path.startswith("/v1/local-"), "unexpected local capability origin"
        return origin + parsed.path + ("?" + parsed.query if parsed.query else "")

    def verify_downloads(database, origin):
        for line in canonical(database).splitlines():
            fact = json.loads(line)
            grant = api(origin, "POST", f'/v1/files/{fact["file_id"]}/versions/{fact["version"]}/access-grants', {"disposition": "attachment", "delivery_mode": "direct"})
            download_url = grant["url"]
            if fact["provider"] == "local":
                download_url = local_address(download_url, origin)
            else:
                assert urllib.parse.urlsplit(download_url).netloc == urllib.parse.urlsplit(files_origin).netloc
            content, _ = request("GET", download_url)
            assert len(content) == fact["size_bytes"] and hashlib.sha256(content).hexdigest() == fact["sha256"]

    token = credential()
    origin = run(source, "aliyun-oss", ["/app/server"], named=source)
    verify_downloads(source, origin)
    mark("oss_primary_reads_all_legacy_local", versions=10)
    payload = b"Auth58 isolated OSS transition bytes\n" * 128
    digest = hashlib.sha256(payload).hexdigest()
    session = api(origin, "POST", "/v1/upload-sessions", {"file_name": "auth58-transition-cloud.txt", "size_bytes": len(payload), "media_type": "text/plain", "sha256": digest}, expected=201)
    auth = session["authorization"]
    assert auth["provider"] == "aliyun-oss" and auth["endpoint_is_cname"] is True and auth["endpoint"] == files_origin
    date = email.utils.formatdate(usegmt=True)
    oss_headers = {"x-oss-date": date, "x-oss-security-token": auth["security_token"], "x-oss-forbid-overwrite": "true"}
    canonical_headers = "".join(key + ":" + oss_headers[key] + "\n" for key in sorted(oss_headers))
    signing = "PUT\n\ntext/plain\n" + date + "\n" + canonical_headers + "/" + auth["bucket"] + "/" + auth["object_key"]
    signature = base64.b64encode(hmac.new(auth["access_key_secret"].encode(), signing.encode(), hashlib.sha1).digest()).decode()
    object_url = files_origin + "/" + urllib.parse.quote(auth["object_key"], safe="/")
    _, returned = request("PUT", object_url, payload, {**oss_headers, "Content-Type": "text/plain", "Authorization": "OSS " + auth["access_key_id"] + ":" + signature})
    cloud_version = api(origin, "POST", "/v1/upload-sessions/" + session["session_id"] + "/complete", {"observed_size_bytes": len(payload), "observed_sha256": digest, "etag": returned.get("ETag", "").strip('"')})
    assert cloud_version["sha256"] == digest
    grant = api(origin, "POST", f'/v1/files/{session["file_id"]}/versions/1/access-grants', {"disposition": "attachment", "delivery_mode": "direct"})
    partial, _ = request("GET", grant["url"], headers={"Range": "bytes=0-31"}, expected=206)
    assert partial == payload[:32]
    request("GET", object_url, expected=403)
    verify_downloads(source, origin)
    mark("custom_cname_live_sdk_and_sts", host=urllib.parse.urlsplit(files_origin).netloc, file_id=session["file_id"], sha256=digest, versions=11, unsigned_status=403, range_status=206)
    command(["docker", "stop", source])
    token = credential()
    rollback_name = source + "_local"
    origin = run(source, "local", ["/app/server"], named=rollback_name)
    verify_downloads(source, origin)
    local_payload = b"local primary rollback keeps historical OSS readable\n"
    local_digest = hashlib.sha256(local_payload).hexdigest()
    local_session = api(origin, "POST", "/v1/upload-sessions", {"file_name": "auth58-transition-rollback.txt", "size_bytes": len(local_payload), "media_type": "text/plain", "sha256": local_digest}, expected=201)
    assert local_session["authorization"]["provider"] == "local-http"
    request("PUT", local_address(local_session["authorization"]["upload_url"], origin), local_payload, local_session["authorization"]["headers"], expected=204)
    api(origin, "POST", "/v1/upload-sessions/" + local_session["session_id"] + "/complete", {"observed_size_bytes": len(local_payload), "observed_sha256": local_digest})
    verify_downloads(source, origin)
    command(["docker", "stop", rollback_name])
    mark("local_primary_rollback", historical_oss_readable=True, versions=12, local_file_id=local_session["file_id"])
    source_facts = canonical(source)
    run(source, "local", ["/app/backup", "create", "--output", "/backups/mixed-v2.zip", "--confirm-service-stopped"])
    mixed = json.loads(zipfile.ZipFile(backup_dir + "/mixed-v2.zip").read("manifest.json"))
    assert mixed["version"] == 2 and mixed["storage"]["provider"] == "hybrid" and len(mixed["storage"]["objects"]) == 11 and len(mixed["storage"]["external_objects"]) == 1
    run(target, "aliyun-oss", ["/app/backup", "restore", "--input", "/backups/mixed-v2.zip"])
    assert canonical(target) == source_facts
    token = credential()
    origin = run(target, "aliyun-oss", ["/app/server"], named=target)
    verify_downloads(target, origin)
    assert snapshot() == before
    checksum = command(["sha256sum", backup_dir + "/mixed-v2.zip"]).split()[0]
    mark("mixed_empty_target_restore", versions=12, local_objects=11, external_references=1,
         canonical_sha256=hashlib.sha256(source_facts.encode()).hexdigest(), backup=backup_dir + "/mixed-v2.zip", backup_sha256=checksum)
    print(json.dumps({"result": "pass", "main_unchanged": True, "image": image,
                      "scope": "private probe Local/OSS compatibility, CNAME SDK, primary rollback and mixed restore; not browser or external deletion recovery"}), flush=True)
except Exception as error:
    print(json.dumps({"result": "failed", "reason": str(error) if isinstance(error, RuntimeError) else type(error).__name__}), flush=True)
    sys.exit(1)
finally:
    for name in reversed(started):
        command(["docker", "stop", name])
    if snapshot() != before:
        print(json.dumps({"result": "failed", "reason": "main suite changed during isolated transition"}), flush=True)
        sys.exit(1)
