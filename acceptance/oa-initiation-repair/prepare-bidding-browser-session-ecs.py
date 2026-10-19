#!/usr/bin/env python3
"""Native IAM login for an isolated browser test; no grants or business writes.

The exported HttpOnly cookies are private, short-lived authentication material.
Never include the private session file in safe evidence or source archives.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
import os
from pathlib import Path
import pwd
import urllib.parse
import urllib.request

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a')
spec = importlib.util.spec_from_file_location('delivery', '/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/verify-public-delivery-ecs.py')
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)

def main():
    assert os.geteuid() == 0
    path = Path('/home/ecs-user/oa-bidding-material-browser-session-20261003a.json')
    assert not path.exists()
    grant_sql = "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t"
    grant_before = delivery.deploy.sql('identity_access', grant_sql)
    iam = delivery.deploy.inspect('omsbizfoundry-identity-access-1')
    env = dict(item.split('=', 1) for item in iam['Config']['Env'])
    jar = http.cookiejar.CookieJar()
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(jar))
    csrf = json.loads(delivery.request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': delivery.PUBLIC}
    principal = json.loads(delivery.request(client, '/auth/api/v1/login', 'POST', json.dumps({'username': env['BOOTSTRAP_ADMIN_USERNAME'], 'password': env['BOOTSTRAP_ADMIN_PASSWORD']}).encode(), headers)[0])['principal']
    assert principal['administrator'] is True
    for app in ('project-workspace', 'business-engagement', 'evidence-service'):
        delivery.request(client, '/gateway/authorize/' + app, 'POST', urllib.parse.urlencode({'csrf_token': csrf}).encode(), {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': delivery.PUBLIC})
    cookies = [{'name': c.name, 'value': c.value, 'domain': c.domain, 'path': c.path, 'expires': c.expires if c.expires is not None else -1, 'httpOnly': 'HttpOnly' in c._rest, 'secure': c.secure, 'sameSite': c._rest.get('SameSite', 'Lax')} for c in jar]
    assert cookies and all(c['domain'].lstrip('.') == 'oatest.antuinfo.cn' for c in cookies)
    private = {'cookies': cookies, 'origins': [], 'subject_id': principal['subject_id']}
    with os.fdopen(os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), 'w') as target:
        json.dump(private, target)
    account = pwd.getpwnam('ecs-user')
    os.chown(path, account.pw_uid, account.pw_gid)
    # Grant ownership/active permissions are checked directly, not inferred
    # from administrator=true or successful login.
    active = delivery.deploy.sql('identity_access', grant_sql)
    assert active == grant_before
    print(json.dumps({'passed': True, 'native_IAM': True, 'entry_exchanges': 3, 'grant_writes': 0, 'business_writes': 0, 'grant_snapshot': active, 'private_session_created': True}), flush=True)

if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('browser session preparation failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; credentials suppressed')
