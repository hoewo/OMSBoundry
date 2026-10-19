#!/usr/bin/env python3
"""Sequential ECS/OSS adapter acceptance. Never emits credentials or signed URLs.

Uses the deployed Evidence image, an isolated empty database, a dedicated outbound
bridge, and credentials issued by the existing public Nginx -> Gateway -> IAM.
Evidence control requests target the private probe container: this is NOT public
end-to-end or browser acceptance. Leaves diagnostic facts/objects for recovery QA.
"""
import base64
import datetime
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
import xml.etree.ElementTree as ET


def command(args, *, env=None, data=None):
    result = subprocess.run(args, env=env, input=data, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
    if result.returncode:
        # SDK / Docker diagnostics may contain sensitive URLs or env values.
        raise RuntimeError("command failed: " + " ".join(args[:2]))
    return result.stdout.strip()


def inspect(name):
    return json.loads(command(["docker", "inspect", name]))[0]


def sql(database, query):
    return command(["docker", "exec", postgres, "psql", "-v", "ON_ERROR_STOP=1",
                    "-U", "oms_admin", "-d", database, "-Atc", query])


def request(opener, method, url, data=None, headers=None, expected=200):
    req = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
    try:
        with opener.open(req, timeout=45) as response:
            status, body, returned = response.status, response.read(), response.headers
    except urllib.error.HTTPError as error:
        status, body, returned = error.code, error.read(), error.headers
    # Do not render body/URL in errors: authorization responses contain credentials.
    if status != expected:
        code = ""
        try:
            code = ET.fromstring(body).findtext("Code", "")
        except ET.ParseError:
            pass
        raise RuntimeError(f"{method} HTTP {status}, expected {expected}; OSS code={code}")
    return body, returned


def json_request(opener, method, url, value=None, headers=None, expected=200):
    headers = {"Content-Type": "application/json", **(headers or {})}
    body, _ = request(opener, method, url,
                      None if value is None else json.dumps(value).encode(), headers, expected)
    return json.loads(body) if body else None


def mark(label, **details):
    print(json.dumps({"check": label, "result": "pass", **details}), flush=True)


def main_snapshot():
    return sql("evidence_service", "SELECT jsonb_build_object('versions',"
               "(SELECT count(*) FROM file_versions),'sessions',(SELECT count(*) FROM upload_sessions),"
               "'audit',(SELECT count(*) FROM evidence_audit_entries),'versions_digest',"
               "(SELECT md5(string_agg(row_to_json(v)::text,'' ORDER BY tenant_id,file_id,version)) "
               "FROM file_versions v));")


postgres = os.environ.get("PROBE_POSTGRES", "omsbizfoundry-postgres-1")
main_evidence = os.environ.get("PROBE_MAIN_EVIDENCE", "omsbizfoundry-evidence-service-1")
main_iam = os.environ.get("PROBE_MAIN_IAM", "omsbizfoundry-identity-access-1")
probe = os.environ["PROBE_NAME"]
probe_db = os.environ["PROBE_DATABASE"]
probe_network = os.environ["PROBE_NETWORK"]
public_url = os.environ["PROBE_PUBLIC_URL"].rstrip("/")
bucket = os.environ["PROBE_BUCKET"]
region = os.environ["PROBE_REGION"]
upload_role = os.environ["PROBE_UPLOAD_ROLE_ARN"]
ecs_role = os.environ["PROBE_ECS_ROLE_NAME"]
for value in (probe, probe_db, probe_network):
    if not re.fullmatch(r"evidence_oss_[a-z0-9_]{1,40}", value):
        raise SystemExit("probe names must use reserved evidence_oss_ prefix")

before = main_snapshot()
created = False
try:
    deployed = inspect(main_evidence)
    env = dict(item.split("=", 1) for item in deployed["Config"]["Env"])
    db_url = urllib.parse.urlsplit(env["DATABASE_URL"])
    iam_env = dict(item.split("=", 1) for item in inspect(main_iam)["Config"]["Env"])
    public_keys = next(item["Name"] for item in deployed["Mounts"] if item["Destination"] == "/public")
    data_network = next(name for name in deployed["NetworkSettings"]["Networks"] if name.endswith("_data"))
    if sql("postgres", f"SELECT 1 FROM pg_database WHERE datname='{probe_db}';"):
        raise RuntimeError("refuse to reuse non-empty probe database")
    sql("postgres", f'CREATE DATABASE "{probe_db}" OWNER evidence_service;')
    command(["docker", "network", "create", "--label", "oms.purpose=evidence-oss-acceptance", probe_network])
    env.update({"DATABASE_URL": urllib.parse.urlunsplit(db_url._replace(path="/" + probe_db)),
                "STORAGE_PROVIDER": "aliyun-oss", "LOCAL_SIGNING_KEY": "",
                "OSS_REGION": region, "OSS_BUCKET": bucket, "OSS_ROLE_ARN": upload_role,
                "OSS_BROWSER_ENDPOINT": f"https://oss-{region}.aliyuncs.com",
                "OSS_SERVICE_ENDPOINT": f"https://oss-{region}-internal.aliyuncs.com",
                "OSS_STS_ENDPOINT": f"sts.{region}.aliyuncs.com",
                "ALIBABA_CLOUD_ECS_METADATA": ecs_role})
    env["POSTGRES_CLIENT_DATABASE_URL"] = env["DATABASE_URL"]
    run_env = {**os.environ, **env}
    args = ["docker", "run", "--detach", "--name", probe, "--network", data_network,
            "--memory", "192m", "--cpus", "0.5", "--label", "oms.purpose=evidence-oss-acceptance",
            "--mount", f"type=volume,src={public_keys},dst=/public,readonly"]
    for key in env:
        args.extend(["--env", key])
    # The shell is only the deployed image's migration/server startup, no source build.
    args.extend([deployed["Image"], "sh", "-eu", "-c", "/app/ops migrate >/dev/null && exec /app/server"])
    command(args, env=run_env)
    created = True
    command(["docker", "network", "connect", probe_network, probe])
    address = inspect(probe)["NetworkSettings"]["Networks"][data_network]["IPAddress"]
    private_url = f"http://{address}:8085"
    plain = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for attempt in range(30):
        try:
            ready = json_request(plain, "GET", private_url + "/health/ready")
            assert ready["storage_provider"] == "aliyun-oss"
            break
        except (RuntimeError, urllib.error.URLError):
            if attempt == 29:
                raise RuntimeError("probe not ready; inspect SDK network without exposing credentials")
            time.sleep(1)
    assert not inspect(probe)["HostConfig"]["PortBindings"]
    mark("deployed_sdk_container_ready", provider="aliyun-oss", image=deployed["Image"], published_ports=0)

    jar = http.cookiejar.CookieJar()
    authenticated = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar))
    csrf = json_request(authenticated, "GET", public_url + "/auth/api/v1/csrf")["csrf_token"]
    principal = json_request(authenticated, "POST", public_url + "/auth/api/v1/login",
                             {"username": iam_env["BOOTSTRAP_ADMIN_USERNAME"], "password": iam_env["BOOTSTRAP_ADMIN_PASSWORD"]},
                             {"X-CSRF-Token": csrf})["principal"]
    credential = json_request(authenticated, "POST", public_url + "/auth/api/v1/credentials",
                             {"audience": "evidence-service"}, {"X-CSRF-Token": csrf})["token"]
    actor_headers = {"Authorization": "Bearer " + credential}
    request(plain, "GET", private_url + "/v1/files", expected=401)
    mark("formal_iam_and_anonymous_denial", login_entry="Nginx -> Gateway -> IAM", evidence_entry="private probe")

    def evidence(method, path, value=None, expected=200, command_id=None):
        headers = {**actor_headers}
        if method == "POST":
            headers["Idempotency-Key"] = command_id or str(uuid.uuid4())
        return json_request(plain, method, private_url + path, value, headers, expected)

    def oss(auth, method, key=None, data=None, expected=200, query=None, extra=None):
        key = key or auth["object_key"]
        date = email.utils.formatdate(usegmt=True)
        content_type = "application/octet-stream" if data is not None else ""
        headers = {"Date": date, "x-oss-security-token": auth["security_token"], **(extra or {})}
        if content_type:
            headers["Content-Type"] = content_type
        canonical_headers = "".join(k.lower() + ":" + headers[k].strip() + "\n"
                                    for k in sorted(headers) if k.lower().startswith("x-oss-"))
        canonical_resource = "/" + bucket + "/" + key
        if query:
            canonical_resource += "?" + "&".join(k + ("=" + v if v else "") for k, v in sorted(query.items()))
        signing = f"{method}\n\n{content_type}\n{date}\n{canonical_headers}{canonical_resource}"
        signature = base64.b64encode(hmac.new(auth["access_key_secret"].encode(), signing.encode(), hashlib.sha1).digest()).decode()
        headers["Authorization"] = "OSS " + auth["access_key_id"] + ":" + signature
        url = f"https://{bucket}.oss-{region}.aliyuncs.com/" + urllib.parse.quote(key, safe="/")
        if query:
            url += "?" + urllib.parse.urlencode(query)
        return request(plain, method, url, data, headers, expected)

    completed = []
    for label, size in (("single", 65536), ("multipart", 21 * 1024 * 1024)):
        content = (b"Auth58 isolated Evidence OSS acceptance\n" * (size // 39 + 1))[:size]
        # Avoid accidental short test input if the marker length changes.
        content = (content * (size // len(content) + 1))[:size]
        digest = hashlib.sha256(content).hexdigest()
        create_key = str(uuid.uuid4())
        create = {"file_name": "oss-probe-" + label + ".bin", "size_bytes": size,
                  "media_type": "application/octet-stream", "sha256": digest,
                  "client_context": {"purpose": "isolated-ecs-oss-acceptance"}}
        session = evidence("POST", "/v1/upload-sessions", create, 201, create_key)
        replay = evidence("POST", "/v1/upload-sessions", create, 201, create_key)
        assert replay["session_id"] == session["session_id"]
        authorization = session["authorization"]
        assert authorization["scope"]["object_key"] == authorization["object_key"]
        assert authorization["scope"]["allowed_actions"] == ["oss:PutObject", "oss:AbortMultipartUpload", "oss:ListParts"]
        assert 0 < (datetime.datetime.fromisoformat(authorization["expires_at"].replace("Z", "+00:00")) - datetime.datetime.now(datetime.timezone.utc)).total_seconds() <= 901
        if label == "single":
            oss(authorization, "PUT", authorization["object_key"] + "-unauthorized", b"blocked", 403)
            oss(authorization, "GET", expected=403)
            oss(authorization, "PUT", data=content, extra={"x-oss-forbid-overwrite": "true"})
            oss(authorization, "PUT", data=content, expected=409, extra={"x-oss-forbid-overwrite": "true"})
            mark("single_object_sts_scope_and_overwrite_denial", ttl_seconds=900)
        else:
            body, _ = oss(authorization, "POST", query={"uploads": ""})
            upload_id = ET.fromstring(body).findtext("UploadId")
            assert upload_id
            parts = []
            part_size = authorization["multipart"]["part_size_bytes"]
            for offset in range(0, len(content), part_size):
                number = len(parts) + 1
                _, returned = oss(authorization, "PUT", data=content[offset:offset + part_size],
                                  query={"partNumber": str(number), "uploadId": upload_id})
                parts.append((number, returned["ETag"]))
            oss(authorization, "GET", query={"uploadId": upload_id})
            manifest = ET.Element("CompleteMultipartUpload")
            for number, etag in parts:
                part = ET.SubElement(manifest, "Part")
                ET.SubElement(part, "PartNumber").text = str(number)
                ET.SubElement(part, "ETag").text = etag
            oss(authorization, "POST", data=ET.tostring(manifest), query={"uploadId": upload_id},
                extra={"x-oss-forbid-overwrite": "true"})
            mark("multipart_upload", bytes=size, parts=len(parts), concurrency=1)
        complete_key = str(uuid.uuid4())
        completion = {"observed_size_bytes": size, "observed_sha256": digest}
        version = evidence("POST", "/v1/upload-sessions/" + session["session_id"] + "/complete",
                           completion, command_id=complete_key)
        assert version["sha256"] == digest and version["size_bytes"] == size and version["version"] == 1
        replay_version = evidence("POST", "/v1/upload-sessions/" + session["session_id"] + "/complete",
                                  completion, command_id=complete_key)
        assert replay_version == version
        grant = evidence("POST", f'/v1/files/{version["file_id"]}/versions/1/access-grants',
                         {"disposition": "attachment", "delivery_mode": "direct"})
        downloaded, _ = request(plain, "GET", grant["url"])
        assert hashlib.sha256(downloaded).hexdigest() == digest
        ranged, _ = request(plain, "GET", grant["url"], headers={"Range": "bytes=0-3"}, expected=206)
        assert ranged == content[:4]
        url = urllib.parse.urlsplit(grant["url"])
        request(plain, "GET", urllib.parse.urlunsplit(url._replace(query="")), expected=403)
        mark("completion_replay_and_signed_download", kind=label, file_id=version["file_id"], sha256=digest, bytes=size)
        completed.append({"file_id": version["file_id"], "sha256": digest, "size_bytes": size, "version": 1})
    assert sql(probe_db, "SELECT count(*) FROM file_versions;") == "2"
    assert sql(probe_db, "SELECT count(*) FROM evidence_audit_entries WHERE action='upload_session.completed';") == "2"
    assert main_snapshot() == before
    mark("isolated_ownership_and_no_duplicate_facts", versions=2, main_evidence_unchanged=json.loads(before))
    print(json.dumps({"result": "pass", "scope": "ECS SDK / IAM credential / OSS adapter; not browser or public Evidence cutover",
                      "database": probe_db, "container": probe, "retained_versions": completed}), flush=True)
except Exception as error:
    # Exception repr / tracebacks can include signed URLs and request secrets.
    safe = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    print(json.dumps({"result": "failed", "stage": "ecs-oss-adapter", "reason": safe}), flush=True)
    sys.exit(1)
finally:
    if created:
        command(["docker", "stop", "--time", "10", probe])
    if main_snapshot() != before:
        print(json.dumps({"result": "failed", "reason": "main Evidence snapshot changed"}), flush=True)
        sys.exit(1)
