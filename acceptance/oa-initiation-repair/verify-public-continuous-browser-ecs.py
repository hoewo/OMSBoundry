#!/usr/bin/env python3
"""Read-only native API proof for a separately observed public browser journey.

Creates no project, grant or directory facts. Login/authorize/logout only.
Keep earlier fixture A/B and user facts frozen; only fixture C may change.
"""
import hashlib
import http.cookiejar
import importlib.util
import json
from pathlib import Path
import sys
import urllib.parse
import urllib.request
import uuid

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-public-release-20261002b/continuous-browser')
NUMBER = 'OA-VERIFY-20261002-C'
NAME = 'OA 连续操作复测 20261002C（验收夹具）'
USER = 'f75d4570-5d6a-42ba-ade6-d1cda913ddda'
EXPECTED = {
    'blank-number': ('draft', 1), 'blank-types': ('draft', 1),
    'completed': ('active', 3), 'cancelled': ('active', 3),
    'invalid-edit': ('active', 3), 'edited': ('active', 4),
    'no-op': ('active', 4), 'production-clear': ('active', 5),
    'business-clear': ('active', 6), 'owners-restored': ('active', 7),
    'final': ('active', 7),
    'post-ack-message-release': ('active', 7),
}
spec = importlib.util.spec_from_file_location('delivery', ROOT.parent.parent / 'oa-public-release-20261002a/verify-public-delivery-ecs.py')
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)
deploy = delivery.deploy


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()


def phase_state_matches(phase, project, assignments):
    actual = (project['status'], project['revision'])
    roles = {a['role_key'] for a in assignments if not a.get('ended_at')}
    expected_roles = ({'business_owner'} if phase in ('blank-number', 'blank-types', 'completed', 'cancelled', 'invalid-edit', 'production-clear')
                      else set() if phase == 'business-clear'
                      else {'business_owner', 'production_owner'})
    return actual == EXPECTED[phase] and roles == expected_roles


def preserved(fixture=None):
    result = {}
    tables = json.loads(deploy.sql('project_workspace', "SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public'"))
    for table in tables:
        columns = json.loads(deploy.sql('project_workspace', "SELECT json_agg(column_name) FROM information_schema.columns WHERE table_schema='public' AND table_name='" + table + "'"))
        where = ''
        if fixture:
            assert str(uuid.UUID(fixture)) == fixture
            if table == 'projects':
                where = " WHERE id <> '" + fixture + "'"
            elif table == 'project_commands':
                # No-op receipts have no history row: also inspect response ID.
                where = " WHERE coalesce(response_json->>'id',response_json->'project'->>'id','') <> '" + fixture + "' AND command_id NOT IN (SELECT command_id FROM project_history_events WHERE project_id='" + fixture + "')"
            elif 'project_id' in columns:
                where = " WHERE project_id <> '" + fixture + "'"
        quoted = '"' + table.replace('"', '""') + '"'
        result[table] = deploy.sql('project_workspace', "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + quoted + ' t' + where)
    return result


def main():
    assert len(sys.argv) == 2
    phase = sys.argv[1]
    assert phase in set(EXPECTED) | {'baseline', 'paused-readonly'}
    ROOT.mkdir(mode=0o700, exist_ok=True)
    path = ROOT / (phase + '.json')
    assert not path.exists(), 'never replace an earlier observation'
    matches = json.loads(deploy.sql('project_workspace', "SELECT coalesce(json_agg(id),'[]') FROM projects WHERE name='" + NAME + "' OR project_number='" + NUMBER + "'"))
    assert len(matches) <= 1
    fixture = matches[0] if matches else None
    assert phase != 'baseline' or fixture is None
    assert phase == 'baseline' or fixture is not None
    current = deploy.healthy()
    containers = {key: value['Id'] for key, value in current.items()}
    # Full cross-app facts only at the batch boundaries; intermediate checks
    # remain serial and small on this two-core ECS.
    stable_databases = ({db: deploy.facts(db) for db in deploy.DBS if db not in ('project_workspace', 'identity_access')}
                        if phase in ('baseline', 'final', 'paused-readonly', 'post-ack-message-release') else None)
    grants = deploy.sql('identity_access', "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    before = preserved(fixture)
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    csrf = json.loads(delivery.request(client, '/auth/api/v1/csrf')[0])['csrf_token']
    iam_env = dict(item.split('=', 1) for item in current['IDENTITY']['Config']['Env'])
    headers = {'Content-Type': 'application/json', 'X-CSRF-Token': csrf, 'Origin': delivery.PUBLIC}
    delivery.request(client, '/auth/api/v1/login', 'POST', json.dumps({'username': iam_env['BOOTSTRAP_ADMIN_USERNAME'], 'password': iam_env['BOOTSTRAP_ADMIN_PASSWORD']}).encode(), headers)
    def read(url): return json.loads(delivery.request(client, url)[0])
    try:
        delivery.request(client, '/gateway/authorize/project-workspace', 'POST', urllib.parse.urlencode({'csrf_token': csrf}).encode(), {'Content-Type': 'application/x-www-form-urlencoded', 'Origin': delivery.PUBLIC})
        user = read('/project/api/v1/projects/' + USER + '?include_inactive=true')
        user_history = read('/project/api/v1/projects/' + USER + '/history?limit=100')
        runtime = delivery.request(client, '/workbench/assets/reference-oa-initiation-runtime.js')[0]
        result = {'phase': phase, 'fixture_id': fixture, 'existing_project_tables': before, 'user_facts_sha256': digest({'detail': user, 'history': user_history}), 'other_business_databases': stable_databases, 'grants_digest': grants, 'container_ids': containers, 'runtime_sha256': hashlib.sha256(runtime).hexdigest(), 'browser_observation_recorded_separately': True, 'grant_writes': 0, 'business_api_writes': 0, 'full_oa_accepted': False}
        if fixture:
            native = '/project/api/v1/projects/' + fixture
            detail = read(native + '?include_inactive=true')
            history = read(native + '/history?limit=100')
            result.update({'project': detail['project'], 'assignments': detail['assignments'], 'history': history['items'], 'native_allowed_actions': detail['allowed_actions']})
            result['receipt_count'] = int(deploy.sql('project_workspace', "SELECT count(*) FROM project_commands WHERE coalesce(response_json->>'id',response_json->'project'->>'id','')='" + fixture + "' OR command_id IN (SELECT command_id FROM project_history_events WHERE project_id='" + fixture + "')"))
            baseline = json.loads((ROOT / 'baseline.json').read_text())
            keys = ['existing_project_tables', 'user_facts_sha256', 'grants_digest']
            released_run = next(run for run in ('e', 'd', 'c') if Path('/var/backups/omsbizfoundry/oa-public-20261002' + run + '/rollout.json').exists())
            rollout = Path('/var/backups/omsbizfoundry/oa-public-20261002' + released_run + '/rollout.json')
            if rollout.exists():
                release = ROOT.parent.parent / ('oa-public-release-20261002' + released_run)
                deployed = json.loads(rollout.read_text())
                assert deployed['passed'] and deployed['changed_containers'] == ['unified-workbench']
                published = json.loads((release / 'publication/results.json').read_text())
                manifest = json.loads((release / 'artifacts/manifest.json').read_text())
                assert current['WORKBENCH']['Image'] == published['images']['unified-workbench']['image_id']
                assert result['runtime_sha256'] == manifest['assets']['reference-oa-initiation-runtime.js']
                assert all(value == baseline['container_ids'][key] for key, value in containers.items() if key != 'WORKBENCH')
                result['workbench_only_verified_release'] = manifest['version']
            else:
                keys.extend(['container_ids', 'runtime_sha256'])
            if stable_databases is not None:
                keys.append('other_business_databases')
            for key in keys:
                assert result[key] == baseline[key], 'frozen baseline changed: ' + key
            result['all_existing_rows_preserved'] = True
            # A successful read or unchanged frozen data is not proof that the
            # browser command took effect. Persist a failed observation too.
            if phase in EXPECTED:
                result['phase_assertions_passed'] = phase_state_matches(phase, result['project'], result['assignments'])
                if not result['phase_assertions_passed']:
                    result['passed'] = False
                    result['failed_expectation'] = {'status_revision': EXPECTED[phase], 'current_owner_roles_must_match_phase': True}
                    deploy.record(path, result)
                    raise AssertionError('browser result did not match phase expectation')
            if phase == 'post-ack-message-release':
                final = json.loads((ROOT / 'final.json').read_text())
                assert final['passed'] and final['continuous_journey_assertions_passed']
                assert all(result[key] == final[key] for key in ('project', 'assignments', 'history', 'receipt_count'))
                result['same_facts_and_receipts_as_prior_final'] = True
            if phase == 'final':
                checkpoints = {name: json.loads((ROOT / (name + '.json')).read_text()) for name in ('blank-number', 'blank-types', 'completed', 'cancelled', 'invalid-edit', 'edited', 'no-op', 'production-clear', 'business-clear', 'owners-restored')}
                for name, saved in checkpoints.items():
                    assert saved['passed'] and saved['fixture_id'] == fixture
                    assert (saved['project']['status'], saved['project']['revision']) == EXPECTED[name]
                def same_facts(a, b):
                    assert all(a[key] == b[key] for key in ('project', 'assignments', 'history'))
                same_facts(checkpoints['blank-number'], checkpoints['blank-types'])
                for name in ('cancelled', 'invalid-edit'):
                    same_facts(checkpoints['completed'], checkpoints[name])
                    assert checkpoints[name]['receipt_count'] == checkpoints['completed']['receipt_count']
                same_facts(checkpoints['edited'], checkpoints['no-op'])
                assert checkpoints['no-op']['receipt_count'] == checkpoints['edited']['receipt_count'] + 1
                completed = checkpoints['completed']
                assert completed['project']['project_number'] == NUMBER
                assert set(completed['project']['project_type_keys']) == {'design', 'consultation'}
                assert [a['role_key'] for a in completed['assignments']] == ['business_owner']
                assert len([event for event in result['history'] if event['event_type'] == 'project_activated']) == 1
                old_business = completed['assignments'][0]
                edited = checkpoints['edited']
                assert edited['project']['planned_start_date'] == '2026-10-02T00:00:00Z'
                assert edited['project']['summary'] == '公网编辑复测：基本资料与生产负责人一次保存。'
                production = next(a for a in edited['assignments'] if a['role_key'] == 'production_owner' and not a.get('ended_at'))
                assert production['subject_id'] != old_business['subject_id'] and production['created_project_revision'] == 4
                assert {a['role_key'] for a in checkpoints['production-clear']['assignments'] if not a.get('ended_at')} == {'business_owner'}
                assert not any(not a.get('ended_at') for a in checkpoints['business-clear']['assignments'])
                preserved_production = next(a for a in result['assignments'] if a['id'] == production['id'])
                preserved_business = next(a for a in result['assignments'] if a['id'] == old_business['id'])
                assert preserved_production['ended_project_revision'] == 5 and preserved_business['ended_project_revision'] == 6
                for old, retained in ((production, preserved_production), (old_business, preserved_business)):
                    assert all(retained[key] == value for key, value in old.items() if key not in ('ended_at', 'ended_project_revision'))
                assert {a['role_key'] for a in result['assignments'] if not a.get('ended_at')} == {'business_owner', 'production_owner'}
                assert all(a['created_project_revision'] == 7 for a in result['assignments'] if not a.get('ended_at'))
                same_facts(checkpoints['owners-restored'], result)
                assert result['receipt_count'] == checkpoints['owners-restored']['receipt_count']
                result['continuous_journey_assertions_passed'] = True
        assert preserved(fixture) == before, 'reads must not alter Project facts'
        result['passed'] = True
        deploy.record(path, result)
        print(json.dumps({key: result[key] for key in ('phase', 'passed', 'fixture_id', 'grant_writes', 'business_api_writes', 'full_oa_accepted')} | {'project': result.get('project'), 'assignment_count': len(result.get('assignments', [])), 'history_count': len(result.get('history', []))}, ensure_ascii=False), flush=True)
    finally:
        delivery.request(client, '/auth/api/v1/logout', 'POST', b'', headers, 204)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('public continuous readback failed: ' + type(error).__name__ + ' at line ' + str(trace.tb_lineno) + '; no secrets emitted')
