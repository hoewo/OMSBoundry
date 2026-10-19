#!/usr/bin/env python3
"""Bounded Cost overlay + Workbench closure. No human grant or user fact edits."""
import copy, hashlib, importlib.util, json, os, pwd, re, secrets, subprocess, sys, tarfile, time
from pathlib import Path
from urllib.parse import quote
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a')
BACKUP=Path('/var/backups/omsbizfoundry/oa-cost-release-20261004a')
PREVIOUS=Path('/opt/omsbizfoundry/acceptance/oa-commercial-links-20261004a')
VERSION='v0.1.0-oa.20261004.5'
spec=importlib.util.spec_from_file_location('commercial','/opt/omsbizfoundry/acceptance/oa-commercial-release-20261004a/commercial-release-ecs.py')
c=importlib.util.module_from_spec(spec);spec.loader.exec_module(c)
b=c.b;b.ROOT,b.BACKUP=ROOT,BACKUP
run,record=b.run,b.record
APPS={'COST_OUTSOURCING':'cost-outsourcing','WORKBENCH':'unified-workbench'}
SCHEMA='0003_reference_oa_cost_entries.sql'
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def publish():
 c.ROOT,c.BACKUP,c.VERSION,c.APPS=ROOT,BACKUP,VERSION,APPS
 c.publish()
def policies(current):
 directory=ROOT/'contracts';directory.mkdir(mode=0o700)
 def mount(key,dest):
  values=[m for m in current[key]['Mounts'] if m['Destination']==dest];assert len(values)==1;return values[0]
 original=json.loads(Path(mount('IDENTITY','/config/workload-identity-policy.json')['Source']).read_text())
 candidate=copy.deepcopy(original)
 for identity in candidate['identities']:
  if identity['id']=='cost-outsourcing':
   evidence=[p for p in identity['permissions'] if p['audience']=='evidence-service'];assert len(evidence)==1
   identity['permissions']=[p for p in identity['permissions'] if p['audience']!='evidence-service']+[{'audience':'evidence-service','action':'evidence-service.metadata.read','resource_types':['file']}]
 assert sum(i['id']=='cost-outsourcing' for i in candidate['identities'])==1
 for old,new in zip(original['identities'],candidate['identities']):
  assert old['id']==new['id']
  if old['id']!='cost-outsourcing':assert old==new
  else:
   assert {k:v for k,v in old.items() if k!='permissions'}=={k:v for k,v in new.items() if k!='permissions'}
   assert [p for p in old['permissions'] if p['audience']!='evidence-service']==[p for p in new['permissions'] if p['audience']!='evidence-service']
 policy=directory/'workload-identity-policy.json';record(policy,candidate);policy.chmod(0o444)
 dest='/config/cost-outsourcing-capabilities.json'
 old=json.loads(Path(mount('IDENTITY',dest)['Source']).read_text());new=json.loads((ROOT/'artifacts/contracts/cost-outsourcing-capabilities.json').read_text())
 assert old.keys()==new.keys() and {k:v for k,v in old.items() if k!='capabilities'}=={k:v for k,v in new.items() if k!='capabilities'}
 assert len(old['capabilities'])==len(new['capabilities'])
 for prior,next in zip(old['capabilities'],new['capabilities']):
  if prior['id'].startswith('cost-entry.'):
   assert {k:v for k,v in prior.items() if k!='resource_types'}=={k:v for k,v in next.items() if k!='resource_types'}
   assert next['resource_types']==prior['resource_types']+['reference_oa_cost_entry','business_cost_project','production_cost_project']
  else:assert prior==next
 manifest=directory/'cost-outsourcing-capabilities.json';record(manifest,new);manifest.chmod(0o444)
 replacements={dest:manifest,'/config/workload-identity-policy.json':policy}
 iam=current['IDENTITY'];name='oa_cost_release_20261004a_iam_parser'
 args=['docker','create','--name',name,'--network','none','--read-only','--memory','96m','--cpus','.15','--cap-drop','ALL']
 for m in iam['Mounts']:args+=['--mount','type=bind,src='+str(replacements.get(m['Destination'],m['Source']))+',dst='+m['Destination']+',readonly']
 for v in iam['Config']['Env']:args+=['--env',v]
 run(args+[iam['Image'],'/app/ops','validate-contracts'])
 assert json.loads(run(['docker','start','--attach',name]))=={'service':'identity-access','status':'contracts-valid'}
 return replacements
def prepare():
 assert not BACKUP.exists() and b.ENV.stat().st_mode&0o077==0
 current=b.healthy();assert json.loads((ROOT/'publication/results.json').read_text())['passed']
 assert b.sql('cost_outsourcing','SELECT max(version) FROM schema_migrations')=='0002_payable_suggestions.sql'
 assert not c.environment(current['COST_OUTSOURCING']).get('REFERENCE_OA_COST_PROFILE')
 replacements=policies(current);original=b.ENV.read_bytes();lines=original.decode().splitlines();images=json.loads((ROOT/'publication/results.json').read_text())['images']
 envkeys={'D1_'+key+'_IMAGE':images[app]['reference'] for key,app in APPS.items()}
 assert all(sum(line.startswith(k+'=') for line in lines)==1 for k in envkeys)
 candidate=('\n'.join(k+'='+envkeys[k] if (k:=line.split('=',1)[0]) in envkeys else line for line in lines)+'\n').encode()
 override=json.loads((PREVIOUS/'compose.oa-public.json').read_text())
 for service in ('identity-access','gateway'):
  settings=override.setdefault('services',{}).setdefault(service,{})
  for destination,path in replacements.items():
   if service=='gateway' and destination=='/config/workload-identity-policy.json':continue
   target=':'+destination+':ro';settings['volumes']=[v for v in settings.get('volumes',[]) if not v.endswith(target)]+[str(path)+target]
 for service in ('cost-outsourcing','cost-outsourcing-migrate'):
  override['services'].setdefault(service,{}).setdefault('environment',{})['REFERENCE_OA_COST_PROFILE']='reference-oa-cost-v1'
 record(ROOT/'compose.oa-public.json',override)
 BACKUP.mkdir(mode=0o700);record(BACKUP/'original.env',original,True);record(BACKUP/'candidate.env',candidate,True)
 record(BACKUP/'preflight.json',{'passed':True,'closure':list(APPS.values())+['IAM/Gateway same binaries, expanded typed Cost scopes; Cost workload exact metadata policy'],'grant_writes':0,'assignment_policy_unchanged':True,'other_workload_permissions_unchanged':True,'old_binary_direct_rollback_forbidden':True,'official_release':False})
 print(json.dumps({'phase':'preflight','passed':True,'grant_writes':0}),flush=True)
def backup():
 c.ROOT,c.BACKUP=ROOT,BACKUP;c.backup()
def cost_process(command,private=True):
 current=b.inspect('omsbizfoundry-cost-outsourcing-1');values=c.environment(current)
 values['REFERENCE_OA_COST_PROFILE']='reference-oa-cost-v1'
 if private:values['DATABASE_URL']=(BACKUP/'restore-cost-url').read_text()
 image=json.loads((ROOT/'publication/results.json').read_text())['images']['cost-outsourcing']['reference']
 name='oa_cost_release_20261004a_'+('private' if private else 'main')+'_'+command+'_'+secrets.token_hex(3)
 args=['docker','create','--name',name,'--network',b.NETWORK if private else next(n for n in current['NetworkSettings']['Networks'] if n.endswith('_data')),'--memory','192m','--cpus','.2','--read-only','--cap-drop','ALL']
 for m in current['Mounts']:
  if m['Destination']=='/backups':continue
  args+=['--mount','type=bind,src='+m['Source']+',dst='+m['Destination']+',readonly']
 for k,v in values.items():args+=['--env',k+'='+v]
 run(args+[image,'/app/ops',command]);output=run(['docker','start','--attach',name]);assert b.inspect(name)['State']['ExitCode']==0
 return output
def standalone():
 current=b.inspect('omsbizfoundry-cost-outsourcing-1');values=c.environment(current);values['DATABASE_URL']=(BACKUP/'restore-cost-url').read_text();values['REFERENCE_OA_COST_PROFILE']='reference-oa-cost-v1'
 image=json.loads((ROOT/'publication/results.json').read_text())['images']['cost-outsourcing']['reference'];name='oa_cost_release_20261004a_standalone'
 args=['docker','create','--name',name,'--network',b.NETWORK,'--memory','128m','--cpus','.2','--read-only','--cap-drop','ALL']
 for m in current['Mounts']:
  if m['Destination']=='/backups':continue
  args+=['--mount','type=bind,src='+m['Source']+',dst='+m['Destination']+',readonly']
 for k,v in values.items():args+=['--env',k+'='+v]
 run(args+[image,'/app/server'])
 for cycle in (1,2):
  run(['docker','start',name]);deadline=time.monotonic()+30
  while True:
   try:
    health=json.loads(run(['docker','exec',name,'wget','-q','-O','-','http://127.0.0.1:8096/health/ready']));assert health['status']=='ready';break
   except (RuntimeError,AssertionError,json.JSONDecodeError):assert time.monotonic()<deadline;time.sleep(1)
  run(['docker','stop','--time','10',name]);assert b.inspect(name)['State']['ExitCode']==0
 record(BACKUP/'standalone.json',{'passed':True,'cycles':2,'own_restored_database':True,'separate_process':True,'identity_provider':'iam','public_journey':False})
def restore():
 b.healthy();assert not (BACKUP/'restore.json').exists()
 for name,sha in json.loads((BACKUP/'cold-backup.json').read_text())['checksums'].items():assert digest(BACKUP/name)==sha
 before=json.loads((BACKUP/'facts-before.json').read_text());assert not b.inspect(b.PRIVATE_PG)['State']['Running'];run(['docker','start',b.PRIVATE_PG])
 try:
  deadline=time.monotonic()+45
  while True:
   try:b.sql('postgres','SELECT 1',True);break
   except RuntimeError:assert time.monotonic()<deadline;time.sleep(1)
  for db in b.DBS:
   target='oa_cost_release_20261004a_'+db;owner=target+'_owner'
   if b.sql('postgres',"SELECT count(*) FROM pg_database WHERE datname='"+target+"'",True)=='0':
    password=secrets.token_hex(24);b.sql('postgres','CREATE ROLE '+owner+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+password+"';",True);b.sql('postgres','CREATE DATABASE '+target+' OWNER '+owner,True)
    with (BACKUP/(db+'.dump')).open('rb') as source:
     r=subprocess.run(['docker','exec','-i',b.PRIVATE_PG,'pg_restore','-U','probe_admin','--role',owner,'-d',target,'--single-transaction','--exit-on-error','--no-owner','--no-privileges'],stdin=source,capture_output=True,timeout=120);assert r.returncode==0
    if db=='cost_outsourcing':record(BACKUP/'restore-cost-url',('postgres://'+owner+':'+quote(password,safe='')+'@'+b.PRIVATE_PG+':5432/'+target+'?sslmode=disable').encode(),True)
   assert b.facts(target,True)==before[db]
   print(json.dumps({'phase':'empty-restore','database':db,'facts_sequences_equal':True}),flush=True)
  for key in ('EVIDENCE','IDENTITY','CONTRACT'):
   directory=BACKUP/('empty-restored-'+key.lower());directory.mkdir(mode=0o700,exist_ok=True)
   with tarfile.open(BACKUP/(key.lower()+'-files.tar.gz')) as archive:
    for member in archive.getmembers():
     p=Path(member.name);assert p.parts[0]=='data' and not p.is_absolute() and '..' not in p.parts and (member.isdir() or member.isfile());target=directory/p
     if member.isdir():target.mkdir(mode=0o700,parents=True,exist_ok=True)
     elif not target.exists():target.parent.mkdir(mode=0o700,parents=True,exist_ok=True);record(target,archive.extractfile(member).read(),True)
   assert {str(p.relative_to(directory/'data')):digest(p) for p in (directory/'data').rglob('*') if p.is_file()}==json.loads((BACKUP/'assets-before.json').read_text())[key]
  target='oa_cost_release_20261004a_cost_outsourcing';old=b.facts(target,True,('schema_migrations',));cost_process('migrate')
  assert b.sql(target,'SELECT max(version) FROM schema_migrations',True)==SCHEMA
  after=b.facts(target,True,('schema_migrations',));assert all(after[k]==v for k,v in old.items());assert all(v.startswith('0:') for k,v in after.items() if k not in old and not k.startswith('sequence:'))
  cost_process('migrate');assert b.facts(target,True,('schema_migrations',))==after;cost_process('check');standalone();assert b.facts(target,True,('schema_migrations',))==after
  record(BACKUP/'restore.json',{'passed':True,'empty_databases':13,'three_file_trees_equal':True,'old_facts_sequences_equal':True,'cost2to3':True,'migration_rerun_zero_changes':True,'standalone_cycles':2,'full_suite_runtime_restore':False})
 finally:
  if b.inspect(b.PRIVATE_PG)['State']['Running']:run(['docker','stop','--time','15',b.PRIVATE_PG])
 print(json.dumps({'phase':'restore-preflight','passed':True}),flush=True)
def export_backup():
 assert json.loads((BACKUP/'restore.json').read_text())['passed'];target=Path('/home/ecs-user/'+ROOT.name+'-backup.tar.gz');assert not target.exists()
 with tarfile.open(target,'x:gz') as archive:archive.add(BACKUP,arcname=BACKUP.name)
 target.chmod(0o600);user=pwd.getpwnam('ecs-user');os.chown(target,user.pw_uid,user.pw_gid);record(ROOT/'offhost-export.json',{'archive_sha256':digest(target),'bytes':target.stat().st_size});print(json.dumps({'phase':'backup-export','archive_sha256':digest(target),'bytes':target.stat().st_size}),flush=True)
def apply():
 current=b.healthy();assert json.loads((BACKUP/'restore.json').read_text())['passed'];confirmed=json.loads((ROOT/'offhost-confirmed.json').read_text());assert confirmed['passed'] and confirmed['archive_sha256']==json.loads((ROOT/'offhost-export.json').read_text())['archive_sha256'];assert b.ENV.read_bytes()==(BACKUP/'original.env').read_bytes()
 old={db:b.facts(db,exclude=('schema_migrations',) if db=='cost_outsourcing' else ()) for db in b.DBS if db!='identity_access'}
 grants=b.sql('identity_access',"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
 run(['docker','stop','--time','30','omsbizfoundry-cost-outsourcing-1'])
 try:
  cost_process('migrate',False);assert b.sql('cost_outsourcing','SELECT max(version) FROM schema_migrations')==SCHEMA
  after=b.facts('cost_outsourcing',exclude=('schema_migrations',));assert all(after[k]==v for k,v in old['cost_outsourcing'].items())
  record(b.ENV,(BACKUP/'candidate.env').read_bytes(),True);configuration=json.loads(b.compose('config','--format','json'))
  for app in APPS.values():assert not configuration['services'][app].get('ports')
  assert configuration['services']['cost-outsourcing']['environment']['REFERENCE_OA_COST_PROFILE']=='reference-oa-cost-v1'
  b.compose('up','-d','--no-deps','--no-build','--pull','never','--wait','--wait-timeout','100','identity-access','gateway',*APPS.values())
  final=b.healthy();changed=('IDENTITY','GATEWAY',*APPS.keys());assert all(final[k]['Id']!=current[k]['Id'] for k in changed);assert all(final[k]['Id']==v['Id'] for k,v in current.items() if k not in changed)
  for db,facts in old.items():
   actual=b.facts(db,exclude=('schema_migrations',) if db=='cost_outsourcing' else ())
   assert all(actual[k]==v for k,v in facts.items()),db
  assert b.sql('identity_access',"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")==grants
  run(['env','D1_ENV_FILE='+str(b.ENV),'D1_COMPOSE_OVERRIDE_FILE='+str(ROOT/'compose.oa-public.json'),str(b.OLD/'ops/d1'),'check'],timeout=240)
  wrapper='#!/bin/sh\nexport D1_ENV_FILE='+str(b.ENV)+'\nexport D1_COMPOSE_OVERRIDE_FILE='+str(ROOT/'compose.oa-public.json')+'\nexec '+str(b.OLD/'ops/d1')+' "$@"\n';record(ROOT/'d1-active',wrapper.encode(),True);(ROOT/'d1-active').chmod(0o700)
  record(BACKUP/'rollout.json',{'passed':True,'version':VERSION,'healthy':17,'changed_containers':[b.SERVICES[k] for k in changed],'old_business_facts_preserved':True,'grants_unchanged':True,'d1_check':True,'public_business_journey_verified':False,'full_oa_accepted':False});print(json.dumps({'phase':'main-rollout','passed':True,'healthy':17,'public_journey_pending':True}),flush=True)
 except Exception:
  migrated=b.sql('cost_outsourcing','SELECT max(version) FROM schema_migrations')==SCHEMA
  if not migrated:run(['docker','start','omsbizfoundry-cost-outsourcing-1'])
  record(BACKUP/'rollout-failure.json',{'migration_applied':migrated,'old_binary_rollback_forbidden':migrated,'backup_volumes_retained':True});raise
if __name__=='__main__':
 try:
  assert os.geteuid()==0 and len(sys.argv)==2
  {'publish':publish,'prepare':prepare,'backup':backup,'restore':restore,'export_backup':export_backup,'apply':apply}[sys.argv[1]]()
 except Exception as error:
  trace=error.__traceback__
  while trace.tb_next:trace=trace.tb_next
  if ROOT.is_dir():record(ROOT/('failed-'+sys.argv[1]+'-'+str(time.time_ns())+'.json'),{'passed':False,'phase':sys.argv[1],'failure_type':type(error).__name__,'line':trace.tb_lineno,'old_binary_direct_rollback_forbidden':True})
  raise SystemExit('cost release failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private diagnostics retained')
