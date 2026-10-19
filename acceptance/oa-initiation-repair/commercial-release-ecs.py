#!/usr/bin/env python3
"""Four matching test images; cold joint backup, empty restore, forward-only upgrade.

Never alters grants, assignment policies, user projects or saved business facts.
Private diagnostics remain mode 0600. Not an official clean-HEAD release.
"""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import pwd
import re
import secrets
import subprocess
import sys
import tarfile
import time
from urllib.parse import quote

ROOT = Path('/opt/omsbizfoundry/acceptance/oa-commercial-release-20261004a')
BACKUP = Path('/var/backups/omsbizfoundry/oa-commercial-20261004a')
PREVIOUS = Path('/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a')
VERSION = 'v0.1.0-oa.20261004.1'
BASE = 'crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/'
APPS = {'CONTRACT':'contract-ledger', 'MONEY':'money-flow-ledger', 'BILLING':'billing-invoice', 'WORKBENCH':'unified-workbench'}
SCHEMAS = {'CONTRACT':'000022_reference_oa_materials.sql', 'MONEY':'000023_party_directory_runtime_projections.sql', 'BILLING':'000001_billing_invoice.sql'}
DATABASES = {'CONTRACT':'contract_ledger', 'MONEY':'money_flow', 'BILLING':'billing_invoice'}
spec = importlib.util.spec_from_file_location('base', '/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a/contributors-release-ecs.py')
prior = importlib.util.module_from_spec(spec); spec.loader.exec_module(prior)
b = prior.b
b.ROOT, b.BACKUP = ROOT, BACKUP
run, record = b.run, b.record

def batch_facts(db, private=False, exclude=()):
    # The SAME canonical table/sequence digests as the prior helper, in one
    # session. Temporary probe output never writes an application table.
    excluded=','.join("'"+v.replace("'","''")+"'" for v in exclude) or "''"
    query="""CREATE TEMP TABLE oa_probe_digest_result(value jsonb);
DO $probe$ DECLARE r record; value text; result jsonb := '{}'::jsonb;
BEGIN
 FOR r IN SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename NOT IN (EXCLUDED) ORDER BY tablename LOOP
  EXECUTE format($q$SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,chr(10) ORDER BY to_jsonb(t)::text),'')) FROM public.%I t$q$,r.tablename) INTO value;
  result := result || jsonb_build_object(r.tablename,value);
 END LOOP;
 FOR r IN SELECT sequencename FROM pg_sequences WHERE schemaname='public' ORDER BY sequencename LOOP
  EXECUTE format($q$SELECT last_value::text || ':' || is_called::text FROM public.%I$q$,r.sequencename) INTO value;
  result := result || jsonb_build_object('sequence:' || r.sequencename,value);
 END LOOP;
 INSERT INTO oa_probe_digest_result VALUES(result);
END $probe$;
SELECT value FROM oa_probe_digest_result;""".replace('EXCLUDED',excluded)
    return json.loads(b.sql(db,query,private))

b.facts=batch_facts

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def environment(container):
    return dict(value.split('=',1) for value in container['Config']['Env'])

def publish():
    current=b.healthy(); ids={k:v['Id'] for k,v in current.items()}
    manifest=json.loads((ROOT/'artifacts/manifest.json').read_text())
    assert manifest['version']==VERSION and manifest['official_release'] is False
    assert digest(ROOT/'source.tar.gz')==manifest['source_archive_sha256']
    work=ROOT/'publication';work.mkdir(mode=0o700,exist_ok=True)
    result={'passed':False,'official_release':False,'images':{}}
    for key,app in APPS.items():
        directory=ROOT/'artifacts'/app
        for name,sha in manifest['applications'][app].items(): assert digest(directory/name)==sha
        running_image=json.loads(run(['docker','image','inspect',current[key]['Image']]))[0]
        refs=[r for r in running_image['RepoDigests'] if r.startswith(BASE+app+'@sha256:')]
        assert len(refs)==1 and re.fullmatch(re.escape(BASE+app)+'@sha256:[a-f0-9]{64}',refs[0])
        runtime=refs[0]
        tag=BASE+app+':'+VERSION
        absent=subprocess.run(['docker','manifest','inspect',tag],capture_output=True,timeout=45)
        if absent.returncode==0:
            run(['docker','pull',tag])
            image=json.loads(run(['docker','image','inspect',tag]))[0]
            assert image['Config']['Labels']['oms.source-archive-sha256']==manifest['source_archive_sha256']
            refs=[r for r in image['RepoDigests'] if r.startswith(BASE+app+'@')];assert len(refs)==1
            reference=refs[0]
        else:
            assert any(x in absent.stderr.decode().lower() for x in ('no such manifest','manifest unknown','not found')), 'publication state uncertain'
            output=run(['docker','build','--platform','linux/amd64','--memory','128m','--cpu-period','100000','--cpu-quota','15000','--build-arg','RUNTIME_IMAGE='+runtime,'--label','oms.release='+VERSION,'--label','oms.official-release=false','--label','oms.source-archive-sha256='+manifest['source_archive_sha256'],'-t',tag,str(directory)])
            record(work/(app+'-build-private.log'),output,True)
            output=run(['docker','push',tag]);record(work/(app+'-push-private.log'),output,True)
            matches=re.findall(r'digest: (sha256:[a-f0-9]{64})',output.decode());assert len(matches)==1
            reference=BASE+app+'@'+matches[0]
        run(['docker','pull',reference]);image=json.loads(run(['docker','image','inspect',reference]))[0]
        assert image['Architecture']=='amd64' and image['Config']['User']==('oms' if key=='WORKBENCH' else 'postgres')
        name='oa_commercial_20261004a_'+key.lower()+'_extract'
        if name not in run(['docker','ps','-a','--format','{{.Names}}']).decode().splitlines():
            run(['docker','create','--name',name,'--network','none','--memory','64m',reference])
        dest=work/app;dest.mkdir(mode=0o700,exist_ok=True)
        for binary,sha in manifest['applications'][app].items():
            run(['docker','cp',name+':/app/'+binary,str(dest/binary)]);assert digest(dest/binary)==sha
        result['images'][app]={'reference':reference,'image_id':image['Id'],'binary_sha256':manifest['applications'][app]}
        record(work/'results.json',result)
        print(json.dumps({'phase':'published-byte-verified','app':app,'reference':reference}),flush=True)
    assert {k:v['Id'] for k,v in b.healthy().items()}==ids
    result.update(passed=True,main_unchanged=True);record(work/'results.json',result)

def policy_candidate(current):
    mounts=[m for m in current['IDENTITY']['Mounts'] if m['Destination']=='/config/workload-identity-policy.json'];assert len(mounts)==1
    original=json.loads(Path(mounts[0]['Source']).read_text());candidate=copy.deepcopy(original)
    for identity in candidate['identities']:
        if identity['id'] in ('contract-ledger','billing-invoice'):
            identity['permissions']=[p for p in identity['permissions'] if p['audience']!='evidence-service']
            identity['permissions'].append({'audience':'evidence-service','action':'evidence-service.metadata.read','resource_types':['file']})
    assert sum(i['id']=='contract-ledger' for i in candidate['identities'])==1
    assert sum(i['id']=='billing-invoice' for i in candidate['identities'])==1
    for old,new in zip(original['identities'],candidate['identities']):
        assert old['id']==new['id']
        assert {k:v for k,v in old.items() if k!='permissions'}=={k:v for k,v in new.items() if k!='permissions'}
        if old['id'] not in ('contract-ledger','billing-invoice'): assert old==new
        else: assert [p for p in old['permissions'] if p['audience']!='evidence-service']==[p for p in new['permissions'] if p['audience']!='evidence-service']
    directory=ROOT/'contracts';directory.mkdir(mode=0o700)
    path=directory/'workload-identity-policy.json';record(path,candidate);path.chmod(0o444)
    iam=current['IDENTITY'];args=['docker','create','--name','oa_commercial_20261004a_iam_parser','--network','none','--read-only','--memory','96m','--cpus','.15','--cap-drop','ALL']
    for m in iam['Mounts']:
        source=str(path) if m['Destination']=='/config/workload-identity-policy.json' else m['Source']
        args+=['--mount','type=bind,src='+source+',dst='+m['Destination']+',readonly']
    for value in iam['Config']['Env']: args+=['--env',value]
    run(args+[iam['Image'],'/app/ops','validate-contracts'])
    assert json.loads(run(['docker','start','--attach','oa_commercial_20261004a_iam_parser']))=={'service':'identity-access','status':'contracts-valid'}
    assert b.inspect('oa_commercial_20261004a_iam_parser')['State']['ExitCode']==0
    return path

def prepare():
    assert not BACKUP.exists() and b.ENV.stat().st_mode&0o077==0
    current=b.healthy();published=json.loads((ROOT/'publication/results.json').read_text());assert published['passed']
    assert b.sql('contract_ledger','SELECT max(version) FROM schema_migrations')=='000018_party_migration_mappings.sql'
    assert b.sql('money_flow','SELECT max(version) FROM schema_migrations')=='000022_bonus_payable_suggestions.sql'
    assert b.sql('billing_invoice','SELECT max(version) FROM schema_migrations')==SCHEMAS['BILLING']
    policy=policy_candidate(current)
    replacements={'D1_'+key+'_IMAGE':published['images'][app]['reference'] for key,app in APPS.items()}
    original=b.ENV.read_bytes();lines=original.decode().splitlines()
    assert all(sum(line.startswith(k+'=') for line in lines)==1 for k in replacements)
    candidate=('\n'.join(k+'='+replacements[k] if (k:=line.split('=',1)[0]) in replacements else line for line in lines)+'\n').encode()
    override=json.loads((PREVIOUS/'compose.oa-public.json').read_text())
    iam=override.setdefault('services',{}).setdefault('identity-access',{})
    target=':/config/workload-identity-policy.json:ro'
    iam['volumes']=[v for v in iam.get('volumes',[]) if not v.endswith(target)]+[str(policy)+target]
    for name in ('contract-ledger','contract-migrate'):
        override['services'].setdefault(name,{}).setdefault('environment',{})['EXTERNAL_EVIDENCE_BASE_URL']='http://evidence-service:8085'
    record(ROOT/'compose.oa-public.json',override)
    BACKUP.mkdir(mode=0o700);record(BACKUP/'original.env',original,True);record(BACKUP/'candidate.env',candidate,True)
    record(BACKUP/'containers-before.json',{k:{'id':v['Id'],'image':v['Image']} for k,v in current.items()})
    record(BACKUP/'preflight.json',{'passed':True,'closure':list(APPS.values())+['IAM same binary, exact-file metadata policy'],'grant_writes':0,'assignment_policies_unchanged':True,'other_workload_permissions_unchanged':True,'old_binary_direct_rollback_forbidden':True,'official_release':False})
    print(json.dumps({'phase':'preflight','passed':True,'human_grants_unchanged':True}),flush=True)

def backup():
    current=b.healthy();assert not (BACKUP/'cold-backup.json').exists()
    assert b.ENV.read_bytes()==(BACKUP/'original.env').read_bytes()
    stopped=[v['Name'].lstrip('/') for k,v in current.items() if k not in ('POSTGRES','GATEWAY','WORKBENCH','NGINX')]
    run(['docker','stop','--time','30',*stopped])
    try:
        facts={}
        for db in b.DBS:
            facts[db]=b.facts(db)
            with os.fdopen(os.open(BACKUP/(db+'.dump'),os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'wb') as output:
                result=subprocess.run(['docker','exec',b.PG,'pg_dump','-U','oms_admin','-d',db,'-Fc','--no-owner','--no-privileges'],stdout=output,stderr=subprocess.PIPE,timeout=120)
                assert result.returncode==0
        assets={}
        for key,destination in (('EVIDENCE','/data/evidence'),('IDENTITY','/data/iam'),('CONTRACT','/data/evidence')):
            matches=[m for m in current[key]['Mounts'] if m['Destination']==destination];assert len(matches)==1
            source=Path(matches[0]['Source']);assert source.is_dir() and '/var/lib/docker/volumes/' in str(source)
            archive=BACKUP/(key.lower()+'-files.tar.gz')
            with tarfile.open(archive,'x:gz') as target: target.add(source,arcname='data',recursive=True)
            archive.chmod(0o600)
            assets[key]={str(p.relative_to(source)):digest(p) for p in source.rglob('*') if p.is_file()}
        record(BACKUP/'facts-before.json',facts);record(BACKUP/'assets-before.json',assets)
        record(BACKUP/'cold-backup.json',{'passed':True,'databases':13,'checksums':{p.name:digest(p) for p in BACKUP.iterdir() if p.is_file()},'file_trees':3,'evidence_provider':'local'})
        print(json.dumps({'phase':'cold-joint-backup','databases':13,'file_trees':3,'passed':True}),flush=True)
    finally:
        run(['docker','start',*stopped]);deadline=time.monotonic()+100
        while True:
            try: b.healthy();break
            except AssertionError: assert time.monotonic()<deadline;time.sleep(2)

def candidate_ops(key,command,private=False):
    current=b.inspect('omsbizfoundry-'+APPS[key]+'-1');values=environment(current)
    if key=='CONTRACT':values['EXTERNAL_EVIDENCE_BASE_URL']='http://evidence-service:8085'
    if private:
        values['DATABASE_URL']=(BACKUP/('restore-'+key.lower()+'-url')).read_text()
        if 'POSTGRES_CLIENT_DATABASE_URL' in values: values['POSTGRES_CLIENT_DATABASE_URL']=values['DATABASE_URL']
    name='oa_commercial_20261004a_'+('restore' if private else 'main')+'_'+key.lower()+'_'+command+'_'+secrets.token_hex(3)
    image=json.loads((ROOT/'publication/results.json').read_text())['images'][APPS[key]]['reference']
    args=['docker','create','--name',name,'--network',b.NETWORK if private else next(n for n in current['NetworkSettings']['Networks'] if n.endswith('_data')),'--memory','192m','--cpus','.2','--read-only','--cap-drop','ALL']
    if key=='CONTRACT':
        # The migration binary probes local storage even when external Evidence
        # is enabled. Give ONLY that probe private, ephemeral writable storage;
        # neither main nor restored legacy file bytes may be mutated by ops.
        uid=run(['docker','run','--rm','--network','none','--entrypoint','id',image,'-u']).decode().strip()
        assert re.fullmatch(r'[0-9]+',uid)
        values['EVIDENCE_STORAGE_DIR']='/migration-evidence'
        args+=['--tmpfs','/migration-evidence:rw,noexec,nosuid,size=4m,mode=0700,uid='+uid]
    for m in current['Mounts']:
        if m['Destination']=='/backups':continue
        args+=['--mount','type=bind,src='+m['Source']+',dst='+m['Destination']+',readonly']
    for k,v in values.items():args+=['--env',k+'='+v]
    run(args+[image,'/app/ops',command]);output=run(['docker','start','--attach',name])
    state=b.inspect(name)['State'];assert state['ExitCode']==0 and not state['OOMKilled']
    return output.decode().strip()

def restore():
    b.healthy();assert not (BACKUP/'restore.json').exists()
    for name,sha in json.loads((BACKUP/'cold-backup.json').read_text())['checksums'].items():assert digest(BACKUP/name)==sha
    before=json.loads((BACKUP/'facts-before.json').read_text())
    pg=b.inspect(b.PRIVATE_PG);assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    run(['docker','start',b.PRIVATE_PG])
    try:
        deadline=time.monotonic()+45
        while True:
            try:b.sql('postgres','SELECT 1',True);break
            except RuntimeError:assert time.monotonic()<deadline;time.sleep(1)
        for db in b.DBS:
            target='commercial_20261004a_'+db;owner=target+'_owner'
            exists=b.sql('postgres',"SELECT count(*) FROM pg_database WHERE datname='"+target+"'",True)=='1'
            if not exists:
                password=secrets.token_hex(24)
                b.sql('postgres','CREATE ROLE '+owner+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD '"+password+"';",True)
                b.sql('postgres','CREATE DATABASE '+target+' OWNER '+owner,True)
                with (BACKUP/(db+'.dump')).open('rb') as source:
                    result=subprocess.run(['docker','exec','-i',b.PRIVATE_PG,'pg_restore','-U','probe_admin','--role',owner,'-d',target,'--single-transaction','--exit-on-error','--no-owner','--no-privileges'],stdin=source,capture_output=True,timeout=120)
                    assert result.returncode==0
                for key,source_db in DATABASES.items():
                    if db==source_db:record(BACKUP/('restore-'+key.lower()+'-url'),('postgres://'+owner+':'+quote(password,safe='')+'@'+b.PRIVATE_PG+':5432/'+target+'?sslmode=disable').encode(),True)
            # A restart validates the already restored bytes; it never drops,
            # overwrites or silently adopts a partially restored database.
            assert b.facts(target,True)==before[db]
            for key,source_db in DATABASES.items():
                if db==source_db:assert (BACKUP/('restore-'+key.lower()+'-url')).is_file()
            print(json.dumps({'phase':'empty-restore','database':db,'equal':True}),flush=True)
        for key in ('EVIDENCE','IDENTITY','CONTRACT'):
            directory=BACKUP/('empty-restored-'+key.lower())
            if not directory.exists():
                directory.mkdir(mode=0o700)
                with tarfile.open(BACKUP/(key.lower()+'-files.tar.gz')) as archive:
                    for member in archive.getmembers():
                        p=Path(member.name);assert p.parts[0]=='data' and not p.is_absolute() and '..' not in p.parts and (member.isdir() or member.isfile())
                        target=directory/p
                        if member.isdir():target.mkdir(mode=0o700,parents=True,exist_ok=True)
                        else:target.parent.mkdir(mode=0o700,parents=True,exist_ok=True);record(target,archive.extractfile(member).read(),True)
            assert {str(p.relative_to(directory/'data')):digest(p) for p in (directory/'data').rglob('*') if p.is_file()}==json.loads((BACKUP/'assets-before.json').read_text())[key]
        for key,db in DATABASES.items():
            target='commercial_20261004a_'+db;old=b.facts(target,True,('schema_migrations',))
            candidate_ops(key,'migrate',True)
            assert b.sql(target,'SELECT max(version) FROM schema_migrations',True)==SCHEMAS[key]
            after=b.facts(target,True,('schema_migrations',))
            assert all(after[k]==v for k,v in old.items())
            assert all(v.startswith('0:') for k,v in after.items() if k not in old and not k.startswith('sequence:'))
            candidate_ops(key,'migrate',True);assert b.facts(target,True,('schema_migrations',))==after
            print(json.dumps({'phase':'candidate-migration','app':APPS[key],'old_facts_unchanged':True}),flush=True)
        record(BACKUP/'restore.json',{'passed':True,'empty_databases':13,'all_old_facts_sequences_equal':True,'three_file_trees_equal':True,'contract18to22':True,'money22to23':True,'migration_rerun_zero_changes':True,'full_suite_runtime_restore':False})
    finally:
        if b.inspect(b.PRIVATE_PG)['State']['Running']:run(['docker','stop','--time','15',b.PRIVATE_PG])
    print(json.dumps({'phase':'restore-preflight','passed':True}),flush=True)

def export_backup():
    assert json.loads((BACKUP/'restore.json').read_text())['passed']
    target=Path('/home/ecs-user/oa-commercial-20261004a-backup.tar.gz');assert not target.exists()
    with tarfile.open(target,'x:gz') as archive:archive.add(BACKUP,arcname=BACKUP.name)
    target.chmod(0o600);account=pwd.getpwnam('ecs-user');os.chown(target,account.pw_uid,account.pw_gid)
    record(ROOT/'offhost-export.json',{'archive_sha256':digest(target),'bytes':target.stat().st_size})
    print(json.dumps({'phase':'offhost-export','archive_sha256':digest(target),'bytes':target.stat().st_size}),flush=True)

def apply():
    current=b.healthy()
    assert json.loads((BACKUP/'restore.json').read_text())['passed']
    assert json.loads((ROOT/'offhost-confirmed.json').read_text())['passed']
    assert json.loads((ROOT/'offhost-confirmed.json').read_text())['archive_sha256']==json.loads((ROOT/'offhost-export.json').read_text())['archive_sha256']
    assert b.ENV.read_bytes()==(BACKUP/'original.env').read_bytes()
    migrating=('contract_ledger','money_flow')
    original={db:b.facts(db,exclude=('schema_migrations',) if db in migrating else ()) for db in b.DBS if db!='identity_access'}
    grants=b.sql('identity_access',"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")
    run(['docker','stop','--time','30','omsbizfoundry-contract-ledger-1','omsbizfoundry-money-flow-ledger-1'])
    try:
        for key in ('CONTRACT','MONEY'):
            candidate_ops(key,'migrate')
            db=DATABASES[key]
            assert b.sql(db,'SELECT max(version) FROM schema_migrations')==SCHEMAS[key]
            after=b.facts(db,exclude=('schema_migrations',));assert all(after[k]==v for k,v in original[db].items())
        record(b.ENV,(BACKUP/'candidate.env').read_bytes(),True)
        configuration=json.loads(b.compose('config','--format','json'))
        for app in APPS.values():assert not configuration['services'][app].get('ports')
        for key,app in APPS.items():assert configuration['services'][app]['image']==json.loads((ROOT/'publication/results.json').read_text())['images'][app]['reference']
        b.compose('up','-d','--no-deps','--no-build','--pull','never','--wait','--wait-timeout','100','identity-access',*APPS.values())
        final=b.healthy();changed=('IDENTITY',*APPS.keys())
        assert all(final[k]['Id']!=current[k]['Id'] for k in changed)
        assert all(final[k]['Id']==v['Id'] for k,v in current.items() if k not in changed)
        for db,old in original.items():
            actual=b.facts(db,exclude=('schema_migrations',) if db in migrating else ())
            assert all(actual[k]==v for k,v in old.items()),db
        assert b.sql('identity_access',"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM capability_grants t")==grants
        run(['env','D1_ENV_FILE='+str(b.ENV),'D1_COMPOSE_OVERRIDE_FILE='+str(ROOT/'compose.oa-public.json'),str(b.OLD/'ops/d1'),'check'],timeout=240)
        wrapper='#!/bin/sh\nexport D1_ENV_FILE='+str(b.ENV)+'\nexport D1_COMPOSE_OVERRIDE_FILE='+str(ROOT/'compose.oa-public.json')+'\nexec '+str(b.OLD/'ops/d1')+' "$@"\n'
        record(ROOT/'d1-active',wrapper.encode(),True);(ROOT/'d1-active').chmod(0o700)
        record(BACKUP/'rollout.json',{'passed':True,'version':VERSION,'healthy':17,'changed_containers':[b.SERVICES[k] for k in changed],'old_business_facts_preserved':True,'grants_unchanged':True,'d1_check':True,'public_business_journey_verified':False,'full_oa_accepted':False})
        print(json.dumps({'phase':'main-rollout','passed':True,'healthy':17,'public_journey_pending':True}),flush=True)
    except Exception:
        migrated={key:b.sql(DATABASES[key],'SELECT max(version) FROM schema_migrations')==SCHEMAS[key] for key in ('CONTRACT','MONEY')}
        for key,applied in migrated.items():
            if not applied:run(['docker','start','omsbizfoundry-'+APPS[key]+'-1'])
        record(BACKUP/'rollout-failure.json',{'migration_applied':migrated,'old_binary_rollback_forbidden':any(migrated.values()),'backup_volumes_retained':True})
        raise

if __name__=='__main__':
    try:
        assert os.geteuid()==0 and len(sys.argv)==2
        {'publish':publish,'prepare':prepare,'backup':backup,'restore':restore,'export':export_backup,'apply':apply}[sys.argv[1]]()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        if ROOT.is_dir():
            record(ROOT/('failed-'+sys.argv[1]+'-'+str(time.time_ns())+'.json'),{'passed':False,'phase':sys.argv[1],'failure_type':type(error).__name__,'line':trace.tb_lineno,'old_binary_direct_rollback_forbidden':True})
        raise SystemExit('commercial release failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; private diagnostics retained')
