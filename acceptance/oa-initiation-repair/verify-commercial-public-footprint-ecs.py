#!/usr/bin/env python3
"""Read-only bounded C journey check against the preserved cold restore.

No business SQL writes, grants, migrations, or main container changes. The
already-restored private PG is started only for baseline reads and stopped in
finally; this is not a full Suite runtime restore.
"""
import hashlib, importlib.util, json, os, re, sys, time
from datetime import datetime
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
s=importlib.util.spec_from_file_location('release', ROOT/'cost-release-ecs.py')
m=importlib.util.module_from_spec(s);s.loader.exec_module(m);b=m.b
assert os.geteuid()==0 and len(sys.argv)==2
browser=json.loads(Path(sys.argv[1]).read_text())
project='70edead7-9dc9-44a3-9b35-b9d41b83264e'
subject='12e800ac-00bf-4516-8748-b45c5623493a'
assert browser['fixture_id']==project and browser['run_id']=='35jPS3'
before=json.loads((m.BACKUP/'facts-before.json').read_text())
current=b.healthy();unchanged=[];comparisons=[]
def sqlrows(db,table,where='true',private=False):
 assert re.fullmatch('[a-z_]+',table)
 return json.loads(b.sql(db,"SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM public.\""+table+'" t WHERE '+where,private))
def hashed(db,table,where='true',private=False):
 return b.sql(db,"SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public.\""+table+'" t WHERE '+where,private)
def values(ids):
 assert all(re.fullmatch('[a-f0-9-]{36}',x) for x in ids)
 return '('+','.join("'"+x+"'" for x in ids)+')' if ids else "(NULL)"
def not_in(column,ids):return column+' NOT IN '+values(ids) if ids else 'true'
def commands(prefix):return sorted({v['command_id'] for v in browser['commands'] if v['path'].startswith(prefix) and v.get('command_id')})
for db in b.DBS:
 if db not in ('identity_access','evidence_service','cost_outsourcing','project_workspace','contract_ledger','money_flow','billing_invoice'):
  assert b.facts(db)==before[db],db;unchanged.append(db)
contract_ids=[r['id'] for r in sqlrows('contract_ledger','contracts',"contract_number IN ('OA-PUBLIC-35jPS3-client','OA-PUBLIC-35jPS3-income')")]
assert len(contract_ids)==2
client='dd3a4a04-abde-4a2e-8023-c007aac470a1'
assert client in contract_ids
# contracts retains the original 2.00; the effective 2.02 is revision 4.
# Verify both, rather than requiring an overwrite of the original amount.
assert b.sql('contract_ledger',"SELECT count(*) FROM contracts WHERE id='"+client+"' AND status='active' AND current_revision=4 AND total_amount=2.00")=='1'
assert b.sql('contract_ledger',"SELECT count(*) FROM contract_revisions WHERE contract_id='"+client+"' AND ((revision=1 AND total_amount=2.00) OR (revision=4 AND total_amount=2.02))")=='2'
assert b.sql('contract_ledger',"SELECT count(*) FROM reference_oa_amendment_confirmations WHERE contract_id='"+client+"'")=='3'
assert b.sql('contract_ledger',"SELECT count(*) FROM contract_settlements WHERE contract_id='"+client+"' AND source_revision=4 AND ((settlement_kind='progress' AND total_amount=1.01) OR (settlement_kind='final' AND total_amount=2.02))")=='2'
assert b.sql('contract_ledger',"SELECT count(*) FROM contract_settlement_parties WHERE settlement_id IN (SELECT id FROM contract_settlements WHERE contract_id='"+client+"')")=='4'
for db in ('contract_ledger','money_flow','billing_invoice','project_workspace'):
 actual=b.facts(db)
 for key,value in before[db].items():
  if key.startswith('sequence:'):assert actual[key]==value,(db,key)
cash=sqlrows('money_flow','cash_transactions',"description IN ('公网连续验收-35jPS3-client','公网连续验收-35jPS3-income')")
cash_ids=[r['id'] for r in cash];assert len(cash_ids)==2
assert b.sql('money_flow',"SELECT count(*) FROM cash_transactions WHERE (description='公网连续验收-35jPS3-client' AND amount=1.01) OR (description='公网连续验收-35jPS3-income' AND amount=2.02)")=='2'
assert b.sql('money_flow',"SELECT count(*) FROM cash_transaction_reversals r JOIN cash_transactions c ON c.id=r.cash_transaction_id WHERE c.description='公网连续验收-35jPS3-client' AND r.amount=1.01")=='1'
invoice=sqlrows('billing_invoice','invoice_registrations',"note IN ('公网开票验收-35jPS3-client','公网开票验收-35jPS3-income')")
invoice_ids=[r['id'] for r in invoice];assert len(invoice_ids) in (0,1,2)
try:
 b.run(['docker','start',b.PRIVATE_PG]);deadline=time.monotonic()+45
 while True:
  try:b.sql('postgres','SELECT 1',True);break
  except RuntimeError:assert time.monotonic()<deadline;time.sleep(1)
 def compare(db,table,predicate='true'):
  old=hashed('oa_cost_release_20261004a_'+db,table,predicate,True)
  actual=hashed(db,table,predicate);assert old==actual,(db,table)
  comparisons.append({'database':db,'table':table,'other_rows_unchanged':True})
 # Permit only the two named contracts, their explicitly owned children and
 # the recorded command IDs. Local Party/identity/migration facts stay exact.
 for table in before['contract_ledger']:
  if table.startswith('sequence:'):continue
  columns=b.sql('contract_ledger',"SELECT string_agg(column_name,',') FROM information_schema.columns WHERE table_schema='public' AND table_name='"+table+"'").split(',')
  predicate='true'
  if table=='contracts':predicate=not_in('id',contract_ids)
  elif 'contract_id' in columns:predicate=not_in('contract_id',contract_ids)
  elif table=='contract_settlement_parties':predicate='settlement_id NOT IN (SELECT id FROM contract_settlements WHERE contract_id IN '+values(contract_ids)+')'
  elif table=='audit_entries':predicate=not_in('object_id',contract_ids)
  compare('contract_ledger',table,predicate)
 reverse_ids=[r['id'] for r in sqlrows('money_flow','cash_transaction_reversals','cash_transaction_id IN '+values(cash_ids))] if cash_ids else []
 for table in before['money_flow']:
  if table.startswith('sequence:'):continue
  predicate='true'
  if table=='cash_transactions':predicate=not_in('id',cash_ids)
  elif table in ('cash_transaction_create_commands','cash_transaction_reversals','cash_transaction_reversal_create_commands'):predicate=not_in('cash_transaction_id',cash_ids)
  elif table=='audit_entries':predicate=not_in('object_id',cash_ids+reverse_ids)
  compare('money_flow',table,predicate)
 for table in before['billing_invoice']:
  if table.startswith('sequence:'):continue
  predicate='true'
  if table=='invoice_registrations':predicate=not_in('id',invoice_ids)
  elif table in ('invoice_registration_revisions','invoice_registration_audit_events'):predicate=not_in('invoice_registration_id',invoice_ids)
  elif table=='invoice_registration_commands':predicate=not_in('command_id',commands('/billing/api/'))
  compare('billing_invoice',table,predicate)
 for table in before['project_workspace']:
  if table.startswith('sequence:'):continue
  predicate='true'
  if table=='projects':predicate="id<>'"+project+"'"
  elif table in ('project_history_events','project_resource_links'):predicate="project_id<>'"+project+"'"
  elif table=='project_commands':predicate=not_in('command_id',commands('/project/api/'))
  compare('project_workspace',table,predicate)
 old_project=sqlrows('oa_cost_release_20261004a_project_workspace','projects',"id='"+project+"'",True)[0]
 now_project=sqlrows('project_workspace','projects',"id='"+project+"'")[0]
 assert {k:v for k,v in old_project.items() if k not in ('revision','updated_at')}=={k:v for k,v in now_project.items() if k not in ('revision','updated_at')}
 old_links=sqlrows('oa_cost_release_20261004a_project_workspace','project_resource_links',"project_id='"+project+"'",True)
 now_links=sqlrows('project_workspace','project_resource_links',"project_id='"+project+"'")
 for old in old_links:assert old in now_links
 added=[r for r in now_links if r not in old_links]
 assert len(added)==1+len(cash_ids)+len(invoice_ids)
 assert now_project['revision']==old_project['revision']+len(added)
 assert {r['resource_id'] for r in added}==set(contract_ids+cash_ids+invoice_ids)-{r['resource_id'] for r in old_links}
 old_history=sqlrows('oa_cost_release_20261004a_project_workspace','project_history_events',"project_id='"+project+"'",True)
 now_history=sqlrows('project_workspace','project_history_events',"project_id='"+project+"'")
 for old in old_history:assert old in now_history
 added_history=[r for r in now_history if r not in old_history]
 assert len(added_history)==len(added) and all(r['event_type']=='resource_linked' for r in added_history)
 # Fixed Cost footprint from the independently completed public cost slice.
 costs=sqlrows('cost_outsourcing','reference_oa_cost_entries')
 assert len(costs)==4 and all(r['project_id']==project for r in costs)
 for table,value in before['cost_outsourcing'].items():
  if table!='schema_migrations':assert b.facts('cost_outsourcing')[table]==value,table
 for table in ('reference_oa_cost_revisions','reference_oa_cost_commands','reference_oa_cost_audit'):assert b.facts('cost_outsourcing')[table].startswith('8:')
 assert hashed('identity_access','capability_grants')==before['identity_access']['capability_grants']
 grant_filter="NOT (subject_id='"+subject+"' AND audience='cost-outsourcing' AND resource_id='"+project+"' AND resource_type IN ('business_cost_project','production_cost_project'))"
 assert hashed('identity_access','capability_resource_grants',grant_filter)==before['identity_access']['capability_resource_grants']
 assert b.sql('identity_access','SELECT count(*) FROM capability_resource_grants WHERE NOT ('+grant_filter+') AND revoked_at IS NULL')=='14'
 # A normal successful IAM login resets attempts and advances updated_at.
 # Verify the observed exact exception, not a blanket identity exemption:
 # password, status, administrator, generations and every other Subject stay
 # byte-exact; only this admin's monotonic login timestamp may advance.
 old_subjects={r['id']:r for r in sqlrows('oa_cost_release_20261004a_identity_access','subjects',private=True)}
 now_subjects={r['id']:r for r in sqlrows('identity_access','subjects')}
 assert set(old_subjects)==set(now_subjects)
 identity_differences=[]
 for sid,old in old_subjects.items():
  now=now_subjects[sid];changed=sorted(k for k in set(old)|set(now) if old.get(k)!=now.get(k))
  if changed:
   assert sid==subject and changed==['updated_at'],(sid,changed)
   assert datetime.fromisoformat(now['updated_at'])>=datetime.fromisoformat(old['updated_at'])
   identity_differences.append({'subject_id':sid,'changed_fields':changed,'native_successful_login_timestamp_only':True})
 for table in ('tenants','activation_tokens','schema_migrations'):compare('identity_access',table)
 # Evidence allows the one Cost file plus at most one named Contract and
 # one named Invoice file. Uploads and access do not delete old bytes.
 cost_file='00d82436-4c2c-4026-a6ce-03b417cba80f'
 versions=sqlrows('evidence_service','file_versions',"file_id='"+cost_file+"' OR file_name IN ('公网验收合同-35jPS3.png','公网验收发票-35jPS3.png')")
 file_ids=[r['file_id'] for r in versions];assert 1<=len(file_ids)<=3 and len(set(file_ids))==len(file_ids)
 session_ids=[r['id'] for r in sqlrows('evidence_service','upload_sessions','file_id IN '+values(file_ids))]
 assert len(session_ids)==len(file_ids)
 for table in before['evidence_service']:
  predicate='true'
  if table=='files':predicate=not_in('id',file_ids)
  elif table in ('file_versions','upload_sessions'):predicate=not_in('file_id',file_ids)
  elif table=='idempotency_commands':predicate=not_in('resource_id',file_ids+session_ids)
  elif table=='evidence_audit_entries':predicate=' AND '.join("to_jsonb(t)::text NOT LIKE '%"+i+"%'" for i in file_ids+session_ids)
  compare('evidence_service',table,predicate)
 assets=json.loads((m.BACKUP/'assets-before.json').read_text())
 for key,destination in (('EVIDENCE','/data/evidence'),('IDENTITY','/data/iam'),('CONTRACT','/data/evidence')):
  mounts=[v for v in current[key]['Mounts'] if v['Destination']==destination];assert len(mounts)==1
  source=Path(mounts[0]['Source']);now={str(p.relative_to(source)):hashlib.sha256(p.read_bytes()).hexdigest() for p in source.rglob('*') if p.is_file()}
  for name,sha in assets[key].items():assert now[name]==sha,(key,name)
  assert len(set(now)-set(assets[key]))==(len(file_ids) if key=='EVIDENCE' else 0),key
 receipt={'passed':True,'fixture_id':project,'project_revision':now_project['revision'],'contract_ids':contract_ids,'confirmed_amendments':3,'formal_contract_revision':4,'confirmed_settlements':2,'frozen_settlement_parties':4,'cash_ids':cash_ids,'cash_original_amounts_preserved':True,'cash_reversal_count':1,'invoice_ids':invoice_ids,'cost_records':4,'cost_native_commands':8,'evidence_files':file_ids,'other_business_databases_unchanged':unchanged,'other_owner_rows_unchanged':comparisons,'other_project_assignments_and_user101_unchanged':True,'identity_differences':identity_differences,'only_approved_cost_resource_grants_added':14,'tenant_grants_unchanged':True,'old_three_file_trees_preserved':True,'main_healthy':17,'full_role_acceptance':False,'full_oa_accepted':False,'full_suite_runtime_restore':False}
finally:
 if b.inspect(b.PRIVATE_PG)['State']['Running']:b.run(['docker','stop','--time','15',b.PRIVATE_PG])
b.healthy();b.record(ROOT/'commercial-public-footprint.json',receipt)
print(json.dumps(receipt))
