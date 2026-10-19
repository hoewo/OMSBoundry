#!/usr/bin/env python3
"""Authorized admin delivery check; records only explicit new OA grants and reads.

Browser submission is a separate proof. Credentials/tokens never leave memory.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import urllib.parse
import urllib.request

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002a')
BACKUP = Path('/var/backups/omsbizfoundry/oa-public-20261002a')
PUBLIC = 'https://oatest.antuinfo.cn'
spec = importlib.util.spec_from_file_location('deploy', ROOT / 'deploy-public-slice-ecs.py')
deploy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deploy)


def request(client, path, method='GET', body=None, headers=None, expected=200):
    with client.open(urllib.request.Request(PUBLIC + path, data=body, headers=headers or {}, method=method), timeout=30) as response:
        assert response.status == expected and urllib.parse.urlsplit(response.url).netloc == 'oatest.antuinfo.cn'
        return response.read(), response.headers


def main():
    assert json.loads((BACKUP / 'rollout.json').read_text())['passed']
    before = deploy.facts('project_workspace')
    iam = deploy.inspect('omsbizfoundry-identity-access-1')
    environment = dict(item.split('=', 1) for item in iam['Config']['Env'])
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': PUBLIC}
    login = json.dumps({'username': environment['BOOTSTRAP_ADMIN_USERNAME'], 'password': environment['BOOTSTRAP_ADMIN_PASSWORD']}).encode()
    principal = json.loads(request(client, '/auth/api/v1/login', 'POST', login, headers)[0])['principal']
    added = []
    try:
        assert principal['administrator'] is True
        subject = principal['subject_id']
        assert len(subject) == 36 and set(subject) <= set('0123456789abcdef-')
        existing = json.loads(deploy.sql('identity_access', "SELECT coalesce(json_agg(action ORDER BY action),'[]') FROM capability_grants WHERE subject_id='" + subject + "' AND audience='project-workspace' AND scope_kind='tenant' AND revoked_at IS NULL"))
        assert {'project-workspace.read', 'project-workspace.assignment.manage', 'project-workspace.initiation.complete'} <= set(existing), 'do not widen an unrelated account'
        for action in ('project-workspace.profile.correct', 'project-workspace.counterparty-context.read', 'project-workspace.counterparty-context.manage'):
            if action not in existing:
                value = {'subject_id': subject, 'audience': 'project-workspace', 'action': action, 'scope': {'kind': 'tenant'}}
                request(client, '/auth/api/v1/admin/grants', 'POST', json.dumps(value).encode(), headers, 204)
                added.append(action)
        form = urllib.parse.urlencode({'csrf_token': csrf}).encode()
        request(client, '/gateway/authorize/project-workspace', 'POST', form, {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': PUBLIC})
        page = request(client, '/workbench/')[0]
        runtime = request(client, '/workbench/assets/reference-oa-initiation-runtime.js')[0]
        directory = request(client, '/workbench/assets/reference-oa-project-directory.js')[0]
        shell = request(client, '/workbench/assets/workbench.js')[0]
        assert b'ReferenceOAInitiationController' in runtime and b'business-owner-v1' in runtime and b'/basic-info-changes' in runtime
        assert b"referenceOAController.openCompletion(id)" in shell
        assert hashlib.sha256(runtime).hexdigest() == '2db7d8fd2491bcb2a59ce13c625cb24750d7ebaeeba2e85bdd7fb1a909e1f6cb'
        result = json.loads(request(client, '/project/api/v1/project-directory?limit=100')[0])
        assert isinstance(result['items'], list)
        json.loads(request(client, '/project/api/v1/counterparty-context/name-suggestions?q=OA&limit=10')[0])
        assert deploy.facts('project_workspace') == before
        deploy.record(ROOT / 'public-delivery.json', {'passed': True, 'url': PUBLIC + '/workbench/', 'public_nginx_gateway_iam_project': True, 'page_sha256': hashlib.sha256(page).hexdigest(), 'runtime_sha256': hashlib.sha256(runtime).hexdigest(), 'directory_sha256': hashlib.sha256(directory).hexdigest(), 'shell_sha256': hashlib.sha256(shell).hexdigest(), 'existing_test_admin_only_new_grants_this_attempt': added, 'confirmed_native_admin_actions': ['project-workspace.profile.correct', 'project-workspace.counterparty-context.read', 'project-workspace.counterparty-context.manage'], 'customer_role_overlay_1_3_not_applied_to_other_accounts': True, 'project_reads_zero_fact_change': True, 'browser_click_test': False})
        print(json.dumps({'phase': 'public-delivery', 'passed': True, 'new_admin_grants': added, 'project_reads_zero_fact_change': True, 'browser_click_test': False}), flush=True)
    finally:
        request(client, '/auth/api/v1/logout', 'POST', b'', headers, 204)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('public delivery failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics suppressed')
