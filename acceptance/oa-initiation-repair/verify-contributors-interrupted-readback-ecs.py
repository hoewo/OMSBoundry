#!/usr/bin/env python3
"""Read-only recovery receipt after the browser route callback was interrupted."""
import importlib.util, json, os
from pathlib import Path
ROOT=Path('/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a')
spec=importlib.util.spec_from_file_location('release',ROOT/'contributors-release-ecs.py')
release=importlib.util.module_from_spec(spec);spec.loader.exec_module(release)
b=release.b
project='70edead7-9dc9-44a3-9b35-b9d41b83264e'
assert os.geteuid()==0
value=json.loads(b.sql('project_workspace',"SELECT json_build_object('command_id',command_id,'status',response_status,'project_revision',(response_json->'project'->>'revision')::integer,'assignment_id',response_json->'assignment'->>'id','subject_id',response_json->'assignment'->>'subject_id','source_revision',(response_json->'assignment'->>'source_revision')::integer)::text FROM project_commands WHERE response_json->'project'->>'id'='"+project+"' AND response_json->'project'->>'revision'='11'"))
assert value['status']==200 and value['project_revision']==11
assert value['subject_id']=='0a2aebfa-eb6e-492a-a5f4-1220b1f2b6a7'
current_revision=int(b.sql('project_workspace',"SELECT revision FROM projects WHERE id='"+project+"'"));assert current_revision in (11,12,13)
target=ROOT/'browser-ownership'/'interrupted-readback.json';assert not target.exists()
b.record(target,{'passed':True,'fixture_id':project,'browser_run':'public-i-continuation','browser_passed':False,'interrupted_before_receipt_export':True,'native_commit_confirmed':value,'observed_current_revision':current_revision,'business_writes_by_checker':0,'full_oa_accepted':False})
print(json.dumps({'passed':True,'native_revision':11,'business_writes_by_checker':0}))
