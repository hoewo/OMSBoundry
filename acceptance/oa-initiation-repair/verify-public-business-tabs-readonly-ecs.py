#!/usr/bin/env python3
"""Auth58 .6 read-only API/asset/fact check. Native clicks are separate evidence.

Never creates grants, projects, resource links, or any business fact.
Login, capability exchange, and logout are the only POST requests.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002f')
BACKUP = Path('/var/backups/omsbizfoundry/oa-public-20261002f')
PUBLIC = 'https://oatest.antuinfo.cn'
USER_PROJECT = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda'
os.environ['OA_PUBLIC_RUN'] = 'f'
spec = importlib.util.spec_from_file_location('tabs_deploy', ROOT / 'deploy-public-slice-ecs.py')
deploy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deploy)


def request(client, path, method='GET', body=None, headers=None, expected=200):
    with client.open(urllib.request.Request(PUBLIC + path, data=body, headers=headers or {}, method=method), timeout=30) as response:
        assert response.status == expected and urllib.parse.urlsplit(response.url).netloc == 'oatest.antuinfo.cn'
        return response.read()


def main():
    assert os.geteuid() == 0
    assert len(sys.argv) == 1 or sys.argv[1:] == ['post-browser']
    output = ROOT / ('post-browser-readback.json' if len(sys.argv) == 2 else 'public-business-tabs-readback.json')
    assert json.loads((BACKUP / 'rollout.json').read_text())['passed']
    assert not output.exists()
    current = deploy.healthy()
    manifest = json.loads((ROOT / 'artifacts/manifest.json').read_text())
    published = json.loads((ROOT / 'publication/results.json').read_text())
    assert current['WORKBENCH']['Image'] == published['images']['unified-workbench']['image_id']
    before_containers = json.loads((BACKUP / 'containers-before.json').read_text())
    assert all(current[key]['Id'] == value['id'] for key, value in before_containers.items() if key != 'WORKBENCH')
    frozen = json.loads((BACKUP / 'facts-before.json').read_text())
    assert all(deploy.facts(db) == frozen[db] for db in deploy.DBS if db != 'identity_access')
    assert deploy.facts('identity_access')['capability_grants'] == frozen['identity_access']['capability_grants']
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(request(client, '/auth/api/v1/csrf'))['csrf_token']
    env = dict(item.split('=', 1) for item in current['IDENTITY']['Config']['Env'])
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': PUBLIC}
    request(client, '/auth/api/v1/login', 'POST', json.dumps({'username': env['BOOTSTRAP_ADMIN_USERNAME'], 'password': env['BOOTSTRAP_ADMIN_PASSWORD']}).encode(), headers)
    result = {'passed': False, 'browser_click_proof': False, 'full_oa_accepted': False, 'business_api_writes': 0, 'grant_writes': 0, 'reads': {}}
    def read(path):
        return json.loads(request(client, path))
    def authorize(app):
        token = read('/auth/api/v1/csrf')['csrf_token']
        request(client, '/gateway/authorize/' + app, 'POST', urllib.parse.urlencode({'csrf_token': token}).encode(), {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': PUBLIC})
    try:
        page = request(client, '/workbench/').decode()
        for key in ('overview', 'client', 'personnel', 'bidding', 'contracts', 'payments', 'invoices', 'costs', 'bonus'):
            assert re.search(r'<button\b[^>]*data-real-business-tab="' + key + r'"[^>]*>', page)
            assert 'disabled' not in re.search(r'<button\b[^>]*data-real-business-tab="' + key + r'"[^>]*>', page).group()
        result['assets'] = {}
        for asset, digest in manifest['assets'].items():
            actual = hashlib.sha256(request(client, '/workbench/assets/' + asset)).hexdigest()
            assert actual == digest
            result['assets'][asset] = actual
        routes = dict(re.findall(r'data-real-application="([a-z-]+)" data-real-route="(/[a-z/-]+)"', page))
        authorize('project-workspace')
        ids = [USER_PROJECT]
        linked_ids = json.loads(deploy.sql('project_workspace', "SELECT coalesce(json_agg(id),'[]') FROM projects WHERE status='active' AND id IN (SELECT project_id FROM project_resource_links WHERE unlinked_at IS NULL)"))
        ids += [value for value in linked_ids if value != USER_PROJECT][:2]
        for project_id in ids:
            detail = read('/project/api/v1/projects/' + project_id)
            assert detail['project']['id'] == project_id
            summary = read('/project/api/v1/projects/' + project_id + '/commercial-summary')
            assert summary['project_id'] == project_id
            result['reads'][project_id] = {'commercial_segments': len(summary['segments']), 'commercial_source_types': sorted({v['resource_type'] for v in summary['segments']}), 'sources': {}}
            for app, api in (('cost-outsourcing', '/api/v1/cost-entries'), ('bonus-distribution', '/api/v1/bonus-pools')):
                try:
                    authorize(app)
                    value = read(routes[app] + api + '?project_id=' + project_id)
                    assert value['project_id'] == project_id
                    assert all(item['project']['id'] == project_id and item['tenant_id'] == detail['project']['tenant_id'] for item in value['items'])
                    result['reads'][project_id]['sources'][app] = {'state': 'available', 'records': len(value['items'])}
                except urllib.error.HTTPError as error:
                    assert error.code == 403
                    result['reads'][project_id]['sources'][app] = {'state': 'denied', 'status': 403}
            for link in detail.get('resource_links', []):
                app = link['target_service']
                if link.get('unlinked_at') or app not in ('business-engagement', 'production-operations'):
                    continue
                api, key = ('/api/v1/business-engagements/', 'engagement') if app == 'business-engagement' else ('/api/v1/production-work-items/', 'work_item')
                try:
                    authorize(app)
                    value = read(routes[app] + api + link['resource_id'])
                    assert value[key]['project_id'] == project_id and value[key]['tenant_id'] == detail['project']['tenant_id']
                    result['reads'][project_id]['sources'][app] = {'state': 'available', 'revision': value[key]['revision']}
                except urllib.error.HTTPError as error:
                    assert error.code == 403
                    result['reads'][project_id]['sources'][app] = {'state': 'denied', 'status': 403}
        assert all(deploy.facts(db) == frozen[db] for db in deploy.DBS if db != 'identity_access')
        assert deploy.facts('identity_access')['capability_grants'] == frozen['identity_access']['capability_grants']
        result.update(passed=True, healthy=17, other_container_ids_unchanged=16, all_12_business_databases_unchanged=True, capability_grants_unchanged=True, official_release=False)
    finally:
        token = read('/auth/api/v1/csrf')['csrf_token']
        request(client, '/auth/api/v1/logout', 'POST', b'', {'Content-Type': 'application/json', 'X-CSRF-Token': token, 'Origin': PUBLIC}, 204)
        deploy.record(output, result)
    print(json.dumps(result), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('read-only tabs check failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; no credentials printed')
