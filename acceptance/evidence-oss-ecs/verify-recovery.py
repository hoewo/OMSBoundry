#!/usr/bin/env python3
"""Cold metadata backup/restore of a retained isolated OSS acceptance fixture.

No main database writes, no object deletion. OSS bytes must remain independently
available: an OSS metadata ZIP is not an object backup or deletion recovery proof.
"""
import hashlib
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
import xml.etree.ElementTree as ET


def command(args, *, env=None):
    result = subprocess.run(args, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=120)
    if result.returncode:
        raise RuntimeError("command failed: " + " ".join(args[:2]))
    return result.stdout.strip()


def inspect(name):
    return json.loads(command(["docker", "inspect", name]))[0]


def sql(database, query):
    return command(["docker", "exec", "omsbizfoundry-postgres-1", "psql", "-v", "ON_ERROR_STOP=1",
                    "-U", "oms_admin", "-d", database, "-Atc", query])


def canonical(database):
    return sql(database, "SELECT row_to_json(v)::text FROM file_versions v ORDER BY tenant_id,file_id,version;")


def request(opener, method, url, value=None, headers=None):
    req = urllib.request.Request(url, method=method,
                                 data=None if value is None else json.dumps(value).encode(),
                                 headers={**({"Content-Type": "application/json"} if value is not None else {}),
                                          **(headers or {})})
    try:
        with opener.open(req, timeout=120) as response:
            return response.read(), response.headers
    except urllib.error.HTTPError as error:
        code = ""
        try:
            code = ET.fromstring(error.read()).findtext("Code", "")
        except ET.ParseError:
            pass
        raise RuntimeError(f"{method} HTTP {error.code}; OSS code={code}") from None


def json_request(*args, **kwargs):
    return json.loads(request(*args, **kwargs)[0])


def mark(label, **details):
    print(json.dumps({"check": label, "result": "pass", **details}), flush=True)


probe = os.environ["PROBE_NAME"]
source_db = os.environ["PROBE_DATABASE"]
target_db = os.environ["PROBE_RESTORE_DATABASE"]
target = target_db
network = os.environ["PROBE_NETWORK"]
public_url = os.environ["PROBE_PUBLIC_URL"].rstrip("/")
reuse_restored = os.environ.get("PROBE_REUSE_RESTORED") == "yes"
for value in (probe, source_db, target_db, network):
    if not re.fullmatch(r"evidence_oss_[a-z0-9_]{1,40}", value):
        raise SystemExit("reserved probe names are required")
if source_db == target_db:
    raise SystemExit("restore must use a different empty probe database")
created = False
before = canonical("evidence_service")
try:
    deployed = inspect(probe)
    assert not deployed["State"]["Running"], "all source writers must be stopped"
    env = dict(item.split("=", 1) for item in deployed["Config"]["Env"])
    assert env["STORAGE_PROVIDER"] == "aliyun-oss"
    assert urllib.parse.urlsplit(env["DATABASE_URL"]).path == "/" + source_db
    assert env["POSTGRES_CLIENT_DATABASE_URL"] == env["DATABASE_URL"]
    data_network = next(name for name in deployed["NetworkSettings"]["Networks"] if name.endswith("_data"))
    source_facts = canonical(source_db)
    assert len(source_facts.splitlines()) == 2
    facts_digest = hashlib.sha256(source_facts.encode()).hexdigest()
    backup_dir = "/var/backups/omsbizfoundry/" + source_db
    uid_gid = command(["docker", "run", "--rm", "--network", "none", deployed["Image"],
                       "sh", "-c", "id -u; id -g"]).splitlines()
    assert len(uid_gid) == 2 and all(value.isdecimal() for value in uid_gid)
    command(["install", "-d", "-m", "0700", "-o", uid_gid[0], "-g", uid_gid[1], backup_dir])

    def run(arguments, named=None):
        args = ["docker", "run", "--rm" if named is None else "--detach", "--network", data_network,
                "--memory", "192m", "--cpus", "0.5", "--label", "oms.purpose=evidence-oss-acceptance"]
        if named:
            args.extend(["--name", named])
        for key in env:
            args.extend(["--env", key])
        args.extend(["--mount", "type=bind,src=" + backup_dir + ",dst=/backups"])
        for mount in deployed["Mounts"]:
            if mount["Destination"] == "/public":
                args.extend(["--mount", "type=volume,src=" + mount["Name"] + ",dst=/public,readonly"])
        args.extend([deployed["Image"], *arguments])
        return command(args, env={**os.environ, **env})

    backup_name = "/backups/evidence-service-oss.zip"
    if not reuse_restored:
        run(["/app/backup", "create", "--output", backup_name, "--confirm-service-stopped"])
    checksum = command(["sha256sum", backup_dir + "/evidence-service-oss.zip"]).split()[0]
    mark("cold_oss_metadata_backup", path=backup_dir + "/evidence-service-oss.zip", sha256=checksum,
         warning="ZIP contains database, not OSS bytes")
    if not reuse_restored:
        if sql("postgres", f"SELECT 1 FROM pg_database WHERE datname='{target_db}';"):
            raise RuntimeError("refuse non-empty restore target")
        sql("postgres", f'CREATE DATABASE "{target_db}" OWNER evidence_service;')
    env["DATABASE_URL"] = urllib.parse.urlunsplit(urllib.parse.urlsplit(env["DATABASE_URL"])._replace(path="/" + target_db))
    env["POSTGRES_CLIENT_DATABASE_URL"] = env["DATABASE_URL"]
    if not reuse_restored:
        run(["/app/backup", "restore", "--input", backup_name])
    assert canonical(target_db) == source_facts
    mark("empty_database_restore", database=target_db, versions=2, canonical_sha256=facts_digest)
    if reuse_restored:
        restored = inspect(target)
        assert not restored["State"]["Running"]
        assert restored["Image"] == deployed["Image"]
        assert next(v for v in restored["Config"]["Env"] if v.startswith("DATABASE_URL=")) == "DATABASE_URL=" + env["DATABASE_URL"]
        command(["docker", "start", target])
    else:
        run(["/app/server"], target)
    created = True
    if network not in inspect(target)["NetworkSettings"]["Networks"]:
        command(["docker", "network", "connect", network, target])
    address = inspect(target)["NetworkSettings"]["Networks"][data_network]["IPAddress"]
    private_url = f"http://{address}:8085"
    plain = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for attempt in range(30):
        try:
            ready = json_request(plain, "GET", private_url + "/health/ready")
            assert ready["storage_provider"] == "aliyun-oss"
            break
        except (RuntimeError, urllib.error.URLError):
            if attempt == 29:
                raise RuntimeError("restored container not ready")
            time.sleep(1)
    mark("restored_sdk_ready", published_ports=0)
    iam_env = dict(item.split("=", 1) for item in inspect("omsbizfoundry-identity-access-1")["Config"]["Env"])
    authenticated = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                                urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json_request(authenticated, "GET", public_url + "/auth/api/v1/csrf")["csrf_token"]
    json_request(authenticated, "POST", public_url + "/auth/api/v1/login",
                 {"username": iam_env["BOOTSTRAP_ADMIN_USERNAME"], "password": iam_env["BOOTSTRAP_ADMIN_PASSWORD"]},
                 {"X-CSRF-Token": csrf})
    credential = json_request(authenticated, "POST", public_url + "/auth/api/v1/credentials",
                              {"audience": "evidence-service"}, {"X-CSRF-Token": csrf})["token"]
    mark("restored_formal_iam_credential")
    for line in source_facts.splitlines():
        version = json.loads(line)
        grant = json_request(plain, "POST", f'{private_url}/v1/files/{version["file_id"]}/versions/1/access-grants',
                             {"disposition": "attachment", "delivery_mode": "direct"}, {"Authorization": "Bearer " + credential})
        mark("restored_access_grant", file_id=version["file_id"], expires_at=grant["expires_at"])
        content, returned = request(plain, "GET", grant["url"])
        assert len(content) == version["size_bytes"]
        assert hashlib.sha256(content).hexdigest() == version["sha256"]
        assert returned["x-oss-server-side-encryption"] == "AES256"
        mark("restored_reference_download", file_id=version["file_id"], version=version["version"],
             sha256=version["sha256"], encryption="AES256")
    assert canonical("evidence_service") == before
    mark("main_evidence_unchanged", versions=10)
    print(json.dumps({"result": "pass", "scope": "metadata restore plus live retained OSS bytes; not deleted object recovery"}), flush=True)
except Exception as error:
    safe = str(error) if isinstance(error, RuntimeError) else type(error).__name__
    print(json.dumps({"result": "failed", "reason": safe}), flush=True)
    sys.exit(1)
finally:
    if created:
        command(["docker", "stop", "--time", "10", target])
