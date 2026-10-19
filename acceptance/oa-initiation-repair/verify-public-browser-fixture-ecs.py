#!/usr/bin/env python3
"""Read back a fixture created by native browser clicks; never fabricate UI proof.

No business writes or grants. Credentials and full responses stay in memory.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import urllib.parse
import urllib.request

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002a')
FIXTURE = 'b4283968-99d5-40c0-9d1f-3802d3d2c9fb'
spec = importlib.util.spec_from_file_location('delivery', ROOT / 'verify-public-delivery-ecs.py')
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)
deploy = delivery.deploy


def main():
    deploy.healthy()
    before = deploy.facts('project_workspace')
    iam = deploy.inspect('omsbizfoundry-identity-access-1')
    env = dict(item.split('=', 1) for item in iam['Config']['Env'])
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(delivery.request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': delivery.PUBLIC}
    login = json.dumps({'username': env['BOOTSTRAP_ADMIN_USERNAME'], 'password': env['BOOTSTRAP_ADMIN_PASSWORD']}).encode()
    delivery.request(client, '/auth/api/v1/login', 'POST', login, headers)
    try:
        delivery.request(client, '/gateway/authorize/project-workspace', 'POST', urllib.parse.urlencode({'csrf_token': csrf}).encode(), {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': delivery.PUBLIC})
        path = '/project/api/v1/projects/' + FIXTURE
        detail = json.loads(delivery.request(client, path)[0])
        project = detail['project']
        assert project['id'] == FIXTURE and project['status'] == 'active' and project['revision'] == 5
        assert project['name'] == 'OA 公网立项复核 20261002A（验收夹具）' and project['project_number'] == 'OA-VERIFY-20261002-A'
        assert project['project_type_keys'] == ['design']
        owners = {item['role_key']: item for item in detail['assignments']}
        assert set(owners) == {'business_owner', 'production_owner'}
        history = json.loads(delivery.request(client, path + '/history?limit=100')[0])['items']
        activation = [event for event in history if event['event_type'] == 'project_activated']
        assert len(activation) == 1 and activation[0]['payload']['initiation_profile'] == 'business-owner-v1'
        active_revision = activation[0]['project_revision']
        assert owners['business_owner']['created_project_revision'] < active_revision
        assert owners['production_owner']['created_project_revision'] > active_revision
        contact = json.loads(delivery.request(client, path + '/counterparty-context')[0])
        assert contact['project_revision'] == 5
        assert contact['context']['name'] == 'OA 公网验收甲方 20261002A（虚拟）'
        assert contact['context']['contact_name'] == '验收联系人（虚拟）'
        assert contact['context']['contact_phone'] == '000-00000000'
        names = json.loads(delivery.request(client, '/project/api/v1/counterparty-context/name-suggestions?q=' + urllib.parse.quote('OA 公网验收甲方') + '&limit=10')[0])['names']
        assert contact['context']['name'] in names
        directory = json.loads(delivery.request(client, '/project/api/v1/project-directory?limit=100')[0])['items']
        assert any(item['id'] == FIXTURE for item in directory)
        assert deploy.facts('project_workspace') == before

        original = json.loads((deploy.BACKUP / 'facts-before.json').read_text())
        untouched_databases = []
        for db in deploy.DBS:
            if db not in ('identity_access', 'project_workspace'):
                assert deploy.facts(db) == original[db]
                untouched_databases.append(db)

        # Exclude exactly this fixture and its commands, not arbitrary newer rows.
        existing_project_rows = {}
        for table, digest in original['project_workspace'].items():
            if table == 'schema_migrations' or table.startswith('sequence:'):
                continue
            quoted = '"' + table.replace('"', '""') + '"'
            columns = json.loads(deploy.sql('project_workspace', "SELECT json_agg(column_name) FROM information_schema.columns WHERE table_schema='public' AND table_name='" + table + "'"))
            where = ''
            if table == 'projects':
                where = " WHERE id <> '" + FIXTURE + "'"
            elif table == 'project_commands':
                where = " WHERE command_id NOT IN (SELECT command_id FROM project_history_events WHERE project_id='" + FIXTURE + "')"
            elif 'project_id' in columns:
                where = " WHERE project_id <> '" + FIXTURE + "'"
            value = deploy.sql('project_workspace', "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + quoted + ' t' + where)
            assert value == digest
            existing_project_rows[table] = True

        deploy.record(ROOT / 'browser-fixture-readback.json', {
            'passed': True, 'fixture_id': FIXTURE, 'project_number': project['project_number'],
            'status': project['status'], 'revision': project['revision'],
            'activation_revision': active_revision, 'initiation_profile': 'business-owner-v1',
            'production_owner_created_after_activation': True,
            'virtual_contact_readback': True, 'native_name_suggestion_contains_fixture': True,
            'public_directory_contains_active_fixture': True, 'readback_project_zero_fact_change': True,
            'other_11_database_facts_and_sequences_unchanged': untouched_databases,
            'all_existing_project_table_rows_unchanged': existing_project_rows,
            'contact_response_sha256': hashlib.sha256(json.dumps(contact, sort_keys=True).encode()).hexdigest(),
            'browser_click_observation_recorded_separately': True,
            'full_oa_accepted': False,
        })
        print(json.dumps({'phase': 'browser-fixture-readback', 'passed': True, 'revision': project['revision'], 'other_11_databases_unchanged': True, 'existing_project_rows_unchanged': True}), flush=True)
    finally:
        delivery.request(client, '/auth/api/v1/logout', 'POST', b'', headers, 204)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('fixture readback failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics suppressed')
