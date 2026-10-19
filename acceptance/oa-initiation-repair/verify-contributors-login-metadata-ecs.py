#!/usr/bin/env python3
"""Prove login bookkeeping separately without exporting account rows or hashes."""
import importlib.util, json, os, time
from pathlib import Path

ROOT=Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
spec=importlib.util.spec_from_file_location('ownership',ROOT/'verify-contributors-ownership-ecs.py')
ownership=importlib.util.module_from_spec(spec);spec.loader.exec_module(ownership)
b=ownership.b
OUT=ROOT/'browser-ownership'

def main():
    assert os.geteuid()==0
    project=ownership.fixture();prior=json.loads((OUT/'before.json').read_text())
    current={db:ownership.preserved(db,project) for db in b.DBS}
    changes=[{'database':db,'table':table,'before':prior['facts'][db].get(table),'after':digest}
             for db,tables in current.items() for table,digest in tables.items()
             if prior['facts'][db].get(table)!=digest]
    initial=OUT/'mismatch-diagnostic-initial.json'
    if not initial.exists():
        original=OUT/'mismatch-diagnostic.json'
        b.record(initial,json.loads(original.read_text()) if original.exists() else {'changed_tables':changes,'business_writes':0,'passed':False})
    print(json.dumps({'phase':'diagnostic','changed_tables':changes}),flush=True)
    assert [(c['database'],c['table']) for c in changes]==[('identity_access','subjects')]
    pg=b.inspect(b.PRIVATE_PG);assert not pg['State']['Running'] and not pg['HostConfig']['PortBindings']
    b.run(['docker','start',b.PRIVATE_PG])
    try:
        deadline=time.monotonic()+45
        while True:
            try:b.sql('postgres','SELECT 1',True);break
            except RuntimeError:assert time.monotonic()<deadline;time.sleep(1)
        target='contributors_20261003a_identity_access'
        frozen=b.sql(target,"SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public.subjects t",True)
        assert frozen==prior['facts']['identity_access']['subjects'],'restored snapshot must match the frozen original guard'
        old=json.loads(b.sql(target,"SELECT json_agg(to_jsonb(t) ORDER BY id)::text FROM public.subjects t",True))
        new=json.loads(b.sql('identity_access',"SELECT json_agg(to_jsonb(t) ORDER BY id)::text FROM public.subjects t"))
        assert len(old)==len(new)
        column_changes=[]
        for left,right in zip(old,new):
            assert left['id']==right['id']
            changed=[key for key in set(left)|set(right) if left.get(key)!=right.get(key)]
            assert not set(changed)-{'updated_at'},'non-login account field changed'
            if changed:
                assert right.get('administrator') is True
                assert right['updated_at']>left['updated_at'],'login metadata must advance monotonically'
            if changed:column_changes.append({'subject_id':left['id'],'columns':changed,'administrator':right.get('administrator')})
        receipt={'passed':True,'restored_baseline_full_digest_equals_frozen_before':frozen==prior['facts']['identity_access']['subjects'],
                 'baseline_subjects_digest':frozen,'frozen_before_subjects_digest':prior['facts']['identity_access']['subjects'],
                 'current_subjects_digest':current['identity_access']['subjects'],
                 'account_count':len(old),'changed_columns':column_changes,
                 'all_account_fields_except_updated_at_equal':True,'business_writes':0,
                 'other_preserved_tables_equal':True,'full_oa_accepted':False}
        b.record(OUT/'login-metadata.json',receipt)
        print(json.dumps(receipt),flush=True)
    finally:
        if b.inspect(b.PRIVATE_PG)['State']['Running']:b.run(['docker','stop','--time','15',b.PRIVATE_PG])

if __name__=='__main__':
    try:main()
    except Exception as error:
        trace=error.__traceback__
        while trace.tb_next:trace=trace.tb_next
        raise SystemExit('login metadata verification failed: '+type(error).__name__+' line '+str(trace.tb_lineno)+'; account details suppressed')
