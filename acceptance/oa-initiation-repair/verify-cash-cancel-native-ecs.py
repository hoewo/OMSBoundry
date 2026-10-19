#!/usr/bin/env python3
"""RO boundary guard for six cancelled original Cash forms. No SQL writes."""
import importlib.util,json,os,sys
from pathlib import Path
root=Path('/opt/omsbizfoundry/acceptance/oa-cost-modal-20261004a')
os.environ['OA_WORKBENCH_REPAIR']='cost-modal'
spec=importlib.util.spec_from_file_location('release',root/'login-csrf-release-ecs.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);b=module.b
assert os.geteuid()==0 and len(sys.argv)==2 and sys.argv[1] in ('before','after')
directory=root/'cash-cancel-readonly';directory.mkdir(mode=0o700,exist_ok=True)
def digest(table):
 return b.sql('identity_access',"SELECT count(*)::text||':'||md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM "+table+' t')
b.healthy()
state={'business':{db:b.facts(db) for db in b.DBS if db!='identity_access'},'tenant_grants':digest('capability_grants'),'resource_grants':digest('capability_resource_grants')}
assert b.sql('project_workspace',"SELECT revision FROM projects WHERE id='70edead7-9dc9-44a3-9b35-b9d41b83264e'")=='18'
if sys.argv[1]=='before':
 assert not (directory/'before.json').exists(),'never replace a baseline after operations'
 b.record(directory/'before.json',state)
 print(json.dumps({'passed':True,'phase':'before','healthy':17,'project_revision':18}))
else:
 assert state==json.loads((directory/'before.json').read_text()),'read-only forms changed native facts or grants'
 receipt={'passed':True,'twelve_business_databases_and_sequences_unchanged':True,'both_grant_tables_unchanged':True,'project_revision':18,'main_healthy':17,'business_mutations':0,'full_oa_accepted':False}
 b.healthy();b.record(directory/'closing.json',receipt);print(json.dumps(receipt))
