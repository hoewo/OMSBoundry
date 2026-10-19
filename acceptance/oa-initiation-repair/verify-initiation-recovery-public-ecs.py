#!/usr/bin/env python3
"""Prepare one labelled legacy fixture; then read back real browser completion.

Never alters the user's project, grants or People directory. No secrets emitted.
Fixture preparation is API evidence, NOT browser proof. Keep every created fact.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002b')
OLD = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002a')
USER_PROJECT = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda'
NUMBER = 'OA-RECOVERY-20261002-B'
spec = importlib.util.spec_from_file_location('delivery', OLD / 'verify-public-delivery-ecs.py')
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)
deploy = delivery.deploy


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def preserved_facts(fixture=None):
    result = {}
    for table in json.loads(deploy.sql('project_workspace', "SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public'")):
        columns = json.loads(deploy.sql('project_workspace', "SELECT json_agg(column_name) FROM information_schema.columns WHERE table_schema='public' AND table_name='" + table + "'"))
        where = ''
        if fixture:
            assert str(uuid.UUID(fixture)) == fixture
            if table == 'projects': where = " WHERE id <> '" + fixture + "'"
            elif table == 'project_commands': where = " WHERE command_id NOT IN (SELECT command_id FROM project_history_events WHERE project_id='" + fixture + "')"
            elif 'project_id' in columns: where = " WHERE project_id <> '" + fixture + "'"
        quoted = '"' + table.replace('"', '""') + '"'
        result[table] = deploy.sql('project_workspace', "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + quoted + ' t' + where)
    return result


def main():
    assert len(sys.argv) == 2 and sys.argv[1] in ('prepare-fixture', 'readback')
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(delivery.request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    iam = deploy.inspect('omsbizfoundry-identity-access-1')
    env = dict(item.split('=', 1) for item in iam['Config']['Env'])
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': delivery.PUBLIC}
    delivery.request(client, '/auth/api/v1/login', 'POST', json.dumps({'username':env['BOOTSTRAP_ADMIN_USERNAME'], 'password':env['BOOTSTRAP_ADMIN_PASSWORD']}).encode(), headers)
    def read(path): return json.loads(delivery.request(client, path)[0])
    def command(path, body, expected=200):
        return json.loads(delivery.request(client, path, 'POST', json.dumps({'command_id':str(uuid.uuid4()), **body}).encode(), headers, expected)[0])
    def user_facts():
        detail = read('/project/api/v1/projects/' + USER_PROJECT)
        history = read('/project/api/v1/projects/' + USER_PROJECT + '/history?limit=100')
        return digest({'project':detail['project'], 'assignments':detail['assignments'], 'history':history})
    try:
        for app in ('project-workspace', 'people-groups'):
            delivery.request(client, '/gateway/authorize/' + app, 'POST', urllib.parse.urlencode({'csrf_token':csrf}).encode(), {'Content-Type':'application/x-www-form-urlencoded', 'Origin':delivery.PUBLIC})
        runtime = delivery.request(client, '/workbench/assets/reference-oa-initiation-runtime.js')[0]
        assert b'/initiation-info-changes' in runtime and b'data-initiation-recovery-business-owner' in runtime
        if sys.argv[1] == 'prepare-fixture':
            assert not (ROOT / 'recovery-fixture.json').exists()
            people = [p for p in read('/people/api/v1/persons') if p['status'] == 'active']
            assert len(people) >= 2 and people[0]['id'] != people[1]['id']
            before = preserved_facts(); user_before = user_facts()
            p = command('/project/api/v1/projects', {'project_number':NUMBER, 'name':'OA 旧待立项恢复 20261002B（验收夹具）', 'project_type_keys':['consultation','survey'], 'summary':'仅用于复现缺经营负责人、已有生产分工的故障；保留全部验收历史。'}, 201)
            fixture = p['id']; path = '/project/api/v1/projects/' + fixture
            old = command(path + '/assignments', {'expected_revision':p['revision'], 'operation':'assign', 'subject_kind':'person', 'subject_id':people[0]['id'], 'source_revision':people[0]['revision'], 'role_key':'production_owner'})
            try:
                command(path + '/complete-initiation', {'expected_revision':old['project']['revision'], 'initiation_profile':'business-owner-v1'})
                raise AssertionError('fixture did not reproduce missing owner')
            except urllib.error.HTTPError as error:
                assert error.code == 422
                error.read()
            detail = read(path)
            assert detail['project']['status'] == 'pending_completion' and detail['project']['revision'] == 3
            assert len(detail['assignments']) == 1 and detail['assignments'][0]['role_key'] == 'production_owner'
            assert preserved_facts(fixture) == before and user_facts() == user_before
            fixture_info = {'fixture_id':fixture, 'project_number':NUMBER, 'pending_revision':3, 'original_production_owner_id':people[0]['id'], 'original_production_assignment_id':old['assignment']['id'], 'replacement_person_id':people[1]['id'], 'replacement_person_display':people[1]['display_name'], 'existing_project_rows':before, 'user_project_facts_sha256':user_before, 'public_runtime_sha256':hashlib.sha256(runtime).hexdigest(), 'browser_click_test':False, 'grant_writes':0}
            deploy.record(ROOT / 'recovery-fixture.json', fixture_info)
            print(json.dumps({'phase':'fixture-prepared-not-browser-proof', 'fixture_id':fixture, 'project_number':NUMBER, 'status':'pending_completion', 'revision':3, 'old_production_display':people[0]['display_name'], 'select_business_and_production_display':people[1]['display_name'], 'all_existing_project_rows_unchanged':True, 'user_project_untouched':True}, ensure_ascii=False), flush=True)
        else:
            info = json.loads((ROOT / 'recovery-fixture.json').read_text()); fixture = info['fixture_id']; path = '/project/api/v1/projects/' + fixture
            # The normal detail endpoint only returns current assignments;
            # explicitly request inactive rows to verify retained owner history.
            detail = read(path + '?include_inactive=true'); p = detail['project']; history = read(path + '/history?limit=100')['items']
            assert p['status'] == 'active' and p['revision'] == 5 and p['project_number'] == NUMBER
            owners = detail['assignments']; assert len(owners) == 3
            previous = next(a for a in owners if a['id'] == info['original_production_assignment_id'])
            assert previous['subject_id'] == info['original_production_owner_id'] and previous['ended_at'] and previous['ended_project_revision'] == 4
            current = [a for a in owners if not a.get('ended_at')]
            assert {a['role_key'] for a in current} == {'business_owner','production_owner'}
            assert all(a['subject_id'] == info['replacement_person_id'] and a['created_project_revision'] == 4 for a in current)
            assert len([e for e in history if e['event_type'] == 'project_activated']) == 1
            assert len([e for e in history if e.get('payload',{}).get('schema_version') == 'project-initiation-info-change-v1']) == 1
            assert preserved_facts(fixture) == info['existing_project_rows'] and user_facts() == info['user_project_facts_sha256']
            assert hashlib.sha256(runtime).hexdigest() == info['public_runtime_sha256']
            result = {'passed':True, 'fixture_id':fixture, 'status':p['status'], 'revision':p['revision'], 'same_project_completed':True, 'business_owner_recovered':True, 'production_replaced_with_history':True, 'existing_project_rows_unchanged':True, 'user_project_untouched':True, 'grant_writes':0, 'runtime_sha256':info['public_runtime_sha256'], 'browser_observation_recorded_separately':True, 'full_oa_accepted':False}
            deploy.record(ROOT / 'recovery-browser-readback.json',result)
            print(json.dumps(result), flush=True)
    finally:
        delivery.request(client, '/auth/api/v1/logout', 'POST', b'', headers, 204)


if __name__ == '__main__':
    try: main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next: trace = trace.tb_next
        raise SystemExit('public recovery failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; private diagnostics retained')
