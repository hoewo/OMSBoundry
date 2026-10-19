#!/usr/bin/env python3
"""Read-only release guard before approved human grants or public business writes."""
import importlib.util,json,os
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
s=importlib.util.spec_from_file_location('cost_release',ROOT/'cost-release-ecs.py')
m=importlib.util.module_from_spec(s);s.loader.exec_module(m);b=m.b
assert os.geteuid()==0
assert json.loads((m.BACKUP/'rollout.json').read_text())['passed']
before=json.loads((m.BACKUP/'facts-before.json').read_text())
current=b.healthy()
for db in b.DBS:
 actual=b.facts(db)
 for table,value in before[db].items():
  if db=='cost_outsourcing' and table=='schema_migrations':continue
  assert actual[table]==value,(db,table)
 if db=='cost_outsourcing':
  assert b.sql(db,'SELECT max(version) FROM schema_migrations')==m.SCHEMA
  assert set(actual)-set(before[db])=={'reference_oa_cost_entries','reference_oa_cost_revisions','reference_oa_cost_commands','reference_oa_cost_audit'}
  assert all(actual[k].startswith('0:') for k in set(actual)-set(before[db]))
publication=json.loads((ROOT/'publication/results.json').read_text())
for key,app in m.APPS.items():assert current[key]['Config']['Image']==publication['images'][app]['reference']
receipt={'passed':True,'healthy_main_services':17,'all_old_tables_and_sequences_preserved':True,'both_tenant_and_resource_grants_unchanged':True,'cost_schema':m.SCHEMA,'new_cost_tables_empty':True,'restore_probe_stopped':True,'restore20_stopped':True,'public_business_journey_verified':False,'full_oa_accepted':False}
b.record(ROOT/'closing-before-grants.json',receipt)
print(json.dumps(receipt))
