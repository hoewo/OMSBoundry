#!/usr/bin/env python3
"""RO native checks for eight individually named additional C UI records.

No SQL business writes, IAM grants, migrations or container start/stop.
"""
import importlib.util,json,os,sys
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
s=importlib.util.spec_from_file_location('release',ROOT/'cost-release-ecs.py')
m=importlib.util.module_from_spec(s);s.loader.exec_module(m);b=m.b
assert os.geteuid()==0 and len(sys.argv)==2 and sys.argv[1] in ('before','after')
directory=ROOT/'cost-category-matrix';directory.mkdir(mode=0o700,exist_ok=True)
project='70edead7-9dc9-44a3-9b35-b9d41b83264e'
prefix='公网成本分类-20261004-'
pairs=[('business',v) for v in ('accommodation','expert_fee','bid_signup_fee','bid_service_fee')]+[('production',v) for v in ('vehicle','accommodation','transport','other')]
def rows(table,predicate='true'):
 return json.loads(b.sql('cost_outsourcing',"SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id),'[]'::jsonb)::text FROM "+table+' t WHERE '+predicate))
def digest(db,table,predicate='true'):
 return b.sql(db,"SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM "+table+' t WHERE '+predicate)
b.healthy()
if sys.argv[1]=='before':
 assert not (directory/'before.json').exists(),'preserve the baseline; never replace it after writes'
 assert b.sql('project_workspace',"SELECT project_number||':'||revision::text FROM projects WHERE id='"+project+"'")=='OA-VERIFY-20261002-C:18'
 assert b.facts('cost_outsourcing')['reference_oa_cost_entries'].startswith('4:')
 data={'business':{db:b.facts(db) for db in b.DBS if db not in ('identity_access','evidence_service','cost_outsourcing')},'cost':b.facts('cost_outsourcing'),'old_cost_rows':rows('reference_oa_cost_entries'),'evidence':b.facts('evidence_service'),'tenant_grants':digest('identity_access','capability_grants'),'resource_grants':digest('identity_access','capability_resource_grants')}
 b.record(directory/'before.json',data)
 print(json.dumps({'passed':True,'phase':'before','main_healthy':17,'cost_records':4,'project_revision':18}))
else:
 before=json.loads((directory/'before.json').read_text());current=rows('reference_oa_cost_entries')
 for old in before['old_cost_rows']:assert old in current,'existing Cost snapshot changed'
 extra=[r for r in current if r not in before['old_cost_rows']]
 assert len(extra)==8 and {(r['scope'],r['snapshot']['category']) for r in extra}==set(pairs)
 for r in extra:
  assert r['project_id']==project and r['revision']==2 and r['state']=='withdrawn'
  assert r['snapshot']['amount_minor']==1 and r['snapshot']['occurred_on'] is None and not r['snapshot']['evidence']
  assert r['snapshot']['description']==prefix+r['scope']+'-'+r['snapshot']['category']
 own='('+','.join("'"+r['id']+"'" for r in extra)+')'
 for table,value in before['cost'].items():
  if table.startswith('reference_oa_cost_'):
   column='id' if table=='reference_oa_cost_entries' else ("response->>'id'" if table=='reference_oa_cost_commands' else 'cost_entry_id')
   assert digest('cost_outsourcing',table,column+' NOT IN '+own)==value,table
  else:assert b.facts('cost_outsourcing')[table]==value,table
 for db,facts in before['business'].items():assert b.facts(db)==facts,db
 evidence=b.facts('evidence_service')
 # No Evidence uploads, grants or mutations are part of this matrix.
 # Compare the complete audit as well, not just files and versions.
 for table,value in before['evidence'].items():
  assert evidence[table]==value,table
 assert digest('identity_access','capability_grants')==before['tenant_grants']
 assert digest('identity_access','capability_resource_grants')==before['resource_grants']
 facts=b.facts('cost_outsourcing')
 assert facts['reference_oa_cost_entries'].startswith('12:')
 for table in ('reference_oa_cost_commands','reference_oa_cost_revisions','reference_oa_cost_audit'):assert facts[table].startswith('24:'),table
 receipt={'passed':True,'project_id':project,'project_revision':18,'additional_records':[{k:r[k] for k in ('id','scope','revision','state')}|{'category':r['snapshot']['category']} for r in extra],'additional_native_commands':16,'total_cost_records':12,'total_native_commands_revisions_audits':24,'old_four_cost_rows_and_histories_unchanged':True,'other_ten_business_databases_unchanged':True,'evidence_facts_and_audit_unchanged':True,'grants_unchanged':True,'no_new_evidence_versions_or_uploads':True,'main_healthy':17,'full_role_acceptance':False,'full_oa_accepted':False}
 b.healthy();b.record(directory/'closing.json',receipt);print(json.dumps(receipt))
