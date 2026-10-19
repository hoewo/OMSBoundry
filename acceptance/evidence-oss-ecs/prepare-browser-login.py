#!/usr/bin/env python3
"""Create a short-lived owner-only UI clipboard helper, without logging credentials.

Use only for the user's authorized login to the deployed Auth58 test environment.
Remove the helper after entering the existing IAM credentials; do not save them in
the browser. No credential or token is placed in a URL or returned on stdout.
"""
import html
import json
import os
from pathlib import Path
import subprocess
import tempfile

command = ["ssh", "-F", "/dev/null", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
           "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=/Users/crispydog/.ssh/omsbizfoundry_auth58_known_hosts",
           "-o", "ConnectTimeout=8", "-i", "/Users/crispydog/.ssh/omsbizfoundry_auth58_ed25519",
           "ecs-user@47.98.156.166", "sudo -n docker inspect omsbizfoundry-identity-access-1"]
result = subprocess.run(command, capture_output=True, text=True, timeout=30)
if result.returncode:
    raise SystemExit("private IAM credential lookup failed; diagnostics suppressed")
record = json.loads(result.stdout)[0]
values = dict(item.split("=", 1) for item in record["Config"]["Env"])
username = values["BOOTSTRAP_ADMIN_USERNAME"]
password = values["BOOTSTRAP_ADMIN_PASSWORD"]
assert username and password
directory = Path(tempfile.mkdtemp(prefix="auth58-private-browser-login-"))
os.chmod(directory, 0o700)
path = directory / "login-helper.html"
document = '''<!doctype html><meta charset="utf-8"><title>Auth58 临时登录助手</title>
<h1>Auth58 临时登录助手</h1><p>仅复制已部署的验证账号，不修改或保存密码。</p>
<input id="u" type="password" autocomplete="off" readonly value="%s">
<button onclick="copy('u')">复制用户名</button>
<input id="p" type="password" autocomplete="off" readonly value="%s">
<button onclick="copy('p')">复制密码</button>
<p id="status">登录后删除此临时文件，并清除剪贴板。</p>
<script>function copy(id){const field=document.getElementById(id),temp=document.createElement('textarea');
temp.setAttribute('aria-hidden','true');temp.style.position='fixed';temp.style.left='-9999px';
temp.value=field.value;document.body.appendChild(temp);temp.select();
const ok=document.execCommand('copy');temp.remove();document.getElementById('status').textContent=
ok?'已复制':'复制失败';}</script>''' % (html.escape(username, quote=True), html.escape(password, quote=True))
descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(descriptor, "w") as output:
    output.write(document)
print(json.dumps({"path": str(path), "mode": "0600", "credentials_logged": False}))
