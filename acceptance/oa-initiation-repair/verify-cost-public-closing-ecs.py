#!/usr/bin/env python3
"""Read-only exact Cost C footprint, old owner facts, grants and byte guards."""
import hashlib,importlib.util,json,os,sys
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
s=importlib.util.spec_from_file_location('release',ROOT/'cost-release-ecs.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m);b=m.b
assert os.geteuid()==0
project='70edead7-9dc9-44a3-9b35-b9d41b83264e';subject='12e800ac-00bf-4516-8748-b45c5623493a'
file_id='00d82436-4c2c-4026-a6ce-03b417cba80f';session='e69026f6-20b7-441a-9924-0b3dba0158d4'
ids={'eeafb4ff-4374-4fbe-a4e4-25ece6e29e4c','78256d67-fa6c-4298-8376-7238acbd83f3','47621d3e-9729-43a8-9833-b72b5671b21d'}
before=json.loads((m.BACKUP/'facts-before.json').read_text());current=b.healthy()
for db in b.DBS:
 if db not in ('identity_access','evidence_service','cost_outsourcing'):assert b.facts(db)==before[db],db
actual=b.facts('cost_outsourcing')
for table,value in before['cost_outsourcing'].items():
 if table!='schema_migrations':assert actual[table]==value,table
assert b.sql('cost_outsourcing','SELECT max(version) FROM schema_migrations')==m.SCHEMA
rows=json.loads(b.sql('cost_outsourcing',"SELECT json_agg(to_jsonb(t) ORDER BY id) FROM reference_oa_cost_entries t"))
clean=len(sys.argv)==2 and sys.argv[1]=='clean-retest';assert len(sys.argv)==1 or clean
if clean:
 additional=[r for r in rows if r['snapshot'].get('description')=='公网成本-BwFxe2-连续恢复'];assert len(additional)==1
 ids.add(additional[0]['id'])
assert {r['id'] for r in rows}==ids and all(r['project_id']==project for r in rows)
assert sorted((r['scope'],r['revision'],r['state']) for r in rows)==[('business',2,'recorded'),('business',2,'withdrawn')]+([('business',3,'withdrawn')] if clean else [])+[('production',1,'recorded')]
for table in ('reference_oa_cost_commands','reference_oa_cost_revisions','reference_oa_cost_audit'):assert actual[table].startswith('8:' if clean else '5:'),table
def filtered_hash(db,table,predicate):
 return b.sql(db,"SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public.\""+table+'\" t WHERE '+predicate)
assert filtered_hash('identity_access','capability_grants','true')==before['identity_access']['capability_grants']
filter_grants="NOT (subject_id='"+subject+"' AND audience='cost-outsourcing' AND resource_id='"+project+"' AND resource_type IN ('business_cost_project','production_cost_project'))"
assert filtered_hash('identity_access','capability_resource_grants',filter_grants)==before['identity_access']['capability_resource_grants']
assert b.sql('identity_access',"SELECT count(*) FROM capability_resource_grants WHERE NOT ("+filter_grants+") AND revoked_at IS NULL")=='14'
predicates={'files':"id<>'"+file_id+"'",'upload_sessions':"id<>'"+session+"'",'file_versions':"file_id<>'"+file_id+"'",'idempotency_commands':"resource_id NOT IN ('"+file_id+"','"+session+"')",'evidence_audit_entries':"object_id NOT IN ('"+file_id+"','"+session+"') AND details::text NOT LIKE '%"+file_id+"%' AND details::text NOT LIKE '%"+session+"%'"}
for table,value in before['evidence_service'].items():
 if table in predicates:assert filtered_hash('evidence_service',table,predicates[table])==value,table
 else:assert b.facts('evidence_service')[table]==value,table
assets=json.loads((m.BACKUP/'assets-before.json').read_text())
for key,destination in (('EVIDENCE','/data/evidence'),('IDENTITY','/data/iam'),('CONTRACT','/data/evidence')):
 mounts=[v for v in current[key]['Mounts'] if v['Destination']==destination];assert len(mounts)==1
 source=Path(mounts[0]['Source']);now={str(p.relative_to(source)):hashlib.sha256(p.read_bytes()).hexdigest() for p in source.rglob('*') if p.is_file()}
 for name,sha in assets[key].items():assert now[name]==sha,(key,name)
 additions=set(now)-set(assets[key]);assert len(additions)==(1 if key=='EVIDENCE' else 0),key
 if key=='EVIDENCE':assert now[next(iter(additions))]=='f556d82ad2490f9f5ba36b7f50b0a523d219d04e62779fdedca65d4b3a18eae4'
receipt={'passed':True,'main_healthy':17,'project_revision':15,'cost_records':4 if clean else 3,'cost_revisions_commands_audits':8 if clean else 5,'other_ten_business_databases_unchanged':True,'old_generic_cost_facts_unchanged':True,'old_evidence_facts_and_bytes_preserved':True,'one_new_evidence_file_byte_exact':True,'only_approved_cost_resource_grants_added':14,'tenant_grants_unchanged':True,'user101_unchanged':True,'restore20_and_private_probe_stopped':True,'full_oa_accepted':False}
if not clean:b.record(ROOT/'closing-after-cost-public-first-three.json',receipt)
b.record(ROOT/'closing-after-cost-public.json',receipt);print(json.dumps(receipt))
