#!/usr/bin/env python3
"""Read-only close of the exact .6 deployment; no login or business commands."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a')
ORIGINAL = Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
FIXTURE = '70edead7-9dc9-44a3-9b35-b9d41b83264e'


def main():
    assert os.geteuid() == 0
    target = ROOT / 'closing-check.json'
    assert not target.exists(), 'retain the original closing receipt'
    spec = importlib.util.spec_from_file_location('closure', ORIGINAL / 'contributors-release-ecs.py')
    closure = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(closure)
    b = closure.b
    current = b.healthy()
    published = json.loads((ROOT / 'publication/results.json').read_text())
    assert published['passed']
    image = published['images']['unified-workbench']
    published_id = json.loads(b.run(['docker', 'image', 'inspect', image['reference']]))[0]['Id']
    assert current['WORKBENCH']['Image'] == published_id
    assert current['WORKBENCH']['Config']['Image'] == image['reference']
    running = b.run(['docker', 'ps', '--format', '{{.Names}}']).decode().splitlines()
    main_names = ['omsbizfoundry-' + name + '-1' for name in b.SERVICES.values()]
    assert sorted(running) == sorted(main_names), 'unexpected running probe or restore stack'
    after = json.loads((ORIGINAL / 'browser-ownership/after.json').read_text())
    assert after['passed'] and after['fixture_id'] == FIXTURE
    assert after['all_other_business_owner_facts_unchanged'] and after['iam_login_updated_at_only_proved']
    assert b.sql('project_workspace', "SELECT revision FROM projects WHERE id='" + FIXTURE + "'") == '13'
    schemas = {db: b.sql(db, 'SELECT max(version) FROM schema_migrations') for db in
               ('project_workspace', 'contract_ledger', 'business_engagement')}
    assert schemas['project_workspace'].startswith('000011_')
    # Main Contract is the retained schema-18 binary. Schema 19 belongs to the
    # separately tested repository candidate and must not be reported live.
    assert schemas['contract_ledger'] == '000018_party_migration_mappings.sql'
    assert schemas['business_engagement'].startswith('000002_')
    evidence_env = dict(value.split('=', 1) for value in current['EVIDENCE']['Config']['Env'])
    assert evidence_env['STORAGE_PROVIDER'] == 'local'
    result = subprocess.run([str(ROOT / 'd1-active'), 'check'], capture_output=True, timeout=180)
    assert result.returncode == 0, 'current D1 check failed; private output suppressed'
    final = b.healthy()
    assert {k: v['Id'] for k, v in final.items()} == {k: v['Id'] for k, v in current.items()}
    value = {'passed': True, 'd1_check_exit': 0, 'healthy_main_services': len(final),
             'workbench_reference': image['reference'], 'active_wrapper': str(ROOT / 'd1-active'),
             'schemas': schemas, 'fixture_id': FIXTURE, 'fixture_revision': 13,
             'ownership_after_passed': True, 'business_writes': 0,
             'restore20_and_all_probes_stopped': True, 'evidence_provider': 'local',
             'static_frontend': 'ECS', 'full_suite_runtime_restore': False,
             'full_oa_accepted': False}
    b.record(target, value)
    print(json.dumps(value), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        trace = error.__traceback__
        while trace.tb_next:
            trace = trace.tb_next
        raise SystemExit('contributors close failed: ' + type(error).__name__ +
                         ' line ' + str(trace.tb_lineno) + '; private details suppressed')
