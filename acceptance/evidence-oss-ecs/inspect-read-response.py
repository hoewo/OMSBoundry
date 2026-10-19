#!/usr/bin/env python3
"""Read-only diagnosis of an isolated OSS upload; never print credentials/URLs."""
import base64
import email.utils
import hashlib
import hmac
import json
import os
import re
import subprocess
import urllib.parse
import urllib.request

database = os.environ["PROBE_DATABASE"]
if not re.fullmatch(r"evidence_oss_[a-z0-9_]{1,40}", database):
    raise SystemExit("isolated fixture required")
query = "SELECT jsonb_build_object('bucket',bucket,'key',object_key,'size',expected_size,'sha',declared_sha256) FROM upload_sessions WHERE provider='aliyun-oss' ORDER BY created_at DESC LIMIT 1;"
row = json.loads(subprocess.check_output(["docker", "exec", "omsbizfoundry-postgres-1", "psql", "-U", "oms_admin", "-d", database, "-Atc", query], text=True))
client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
def get(method, url, headers=None, data=None):
    return client.open(urllib.request.Request(url, headers=headers or {}, data=data, method=method), timeout=30)
with get("PUT", "http://100.100.100.200/latest/api/token", {"X-aliyun-ecs-metadata-token-ttl-seconds": "60"}) as response:
    token = response.read().decode()
with get("GET", "http://100.100.100.200/latest/meta-data/ram/security-credentials/Auth58EvidenceServiceRole", {"X-aliyun-ecs-metadata-token": token}) as response:
    credential = json.loads(response.read())
for encoding in ("identity", "gzip"):
    date = email.utils.formatdate(usegmt=True)
    headers = {"x-oss-date": date, "x-oss-security-token": credential["SecurityToken"]}
    signed = "GET\n\n\n" + date + "\n" + "".join(k + ":" + headers[k] + "\n" for k in sorted(headers)) + "/" + row["bucket"] + "/" + row["key"]
    signature = base64.b64encode(hmac.new(credential["AccessKeySecret"].encode(), signed.encode(), hashlib.sha1).digest()).decode()
    headers.update({"Authorization": "OSS " + credential["AccessKeyId"] + ":" + signature, "Accept-Encoding": encoding})
    url = "https://" + row["bucket"] + ".oss-cn-hangzhou-internal.aliyuncs.com/" + urllib.parse.quote(row["key"], safe="/")
    with get("GET", url, headers) as response:
        body = response.read()
        print(json.dumps({"accept_encoding": encoding, "status": response.status, "content_length": response.headers.get("Content-Length"), "content_encoding": response.headers.get("Content-Encoding"), "received_bytes": len(body), "expected_bytes": row["size"], "sha_matches": hashlib.sha256(body).hexdigest() == row["sha"]}), flush=True)
