#!/usr/bin/env python3
"""Read only the reported initiation counterexample; never repair its facts.

Login/credential exchange/logout are the only POSTs. No grant or business
mutation is performed. Private credentials and full responses stay in memory.
Run on Auth58 with the existing release's delivery helpers present.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import urllib.parse
import urllib.request

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002a')
PROJECT = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda'
spec = importlib.util.spec_from_file_location('delivery', ROOT / 'verify-public-delivery-ecs.py')
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)


def main():
    before = delivery.deploy.facts('project_workspace')
    iam = delivery.deploy.inspect('omsbizfoundry-identity-access-1')
    env = dict(item.split('=', 1) for item in iam['Config']['Env'])
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(delivery.request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': delivery.PUBLIC}
    login = json.dumps({'username': env['BOOTSTRAP_ADMIN_USERNAME'], 'password': env['BOOTSTRAP_ADMIN_PASSWORD']}).encode()
    delivery.request(client, '/auth/api/v1/login', 'POST', login, headers)
    try:
        delivery.request(client, '/gateway/authorize/project-workspace', 'POST', urllib.parse.urlencode({'csrf_token': csrf}).encode(), {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': delivery.PUBLIC})
        path = '/project/api/v1/projects/' + PROJECT
        detail = json.loads(delivery.request(client, path)[0])
        project = detail['project']
        assert project['id'] == PROJECT and project['project_number'] == 'OA-2026-101'
        history = json.loads(delivery.request(client, path + '/history?limit=100')[0])['items']
        runtime = delivery.request(client, '/workbench/assets/reference-oa-initiation-runtime.js')[0]
        owners = [{key: item.get(key) for key in ('role_key', 'subject_kind', 'created_project_revision', 'ended_project_revision', 'ended_at')} for item in detail['assignments']]
        result = {
            'schema_version': 'oa-initiation-counterexample-inspection-v1',
            'project_id': PROJECT, 'project_number': project['project_number'],
            'status': project['status'], 'revision': project['revision'],
            'project_type_keys': project['project_type_keys'],
            'owners': owners,
            'history': [{
                'event_type': event['event_type'],
                'project_revision': event['project_revision'],
                'missing': event.get('payload', {}).get('missing', []),
                'initiation_profile': event.get('payload', {}).get('initiation_profile'),
            } for event in history],
            'completion_action_available': 'project-workspace.initiation.complete' in detail.get('allowed_actions', []),
            'business_owner_assignment_action_available': any(value in detail.get('allowed_actions', []) for value in ('project-workspace.assignment.manage', 'project-workspace.assignment.business-owner.manage')),
            'public_runtime_sha256': hashlib.sha256(runtime).hexdigest(),
            'project_facts_unchanged_during_readback': delivery.deploy.facts('project_workspace') == before,
            'business_writes': 0, 'grant_writes': 0, 'browser_click_test': False,
            'full_oa_accepted': False,
        }
        print(json.dumps(result, ensure_ascii=False), flush=True)
    finally:
        delivery.request(client, '/auth/api/v1/logout', 'POST', b'', headers, 204)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('read-only counterexample inspection failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics suppressed')
