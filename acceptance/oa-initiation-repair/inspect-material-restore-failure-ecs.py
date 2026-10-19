#!/usr/bin/env python3
"""Read-only classification; raw diagnostics remain in private backup only."""
import json, subprocess, os, re
from pathlib import Path
name='oa_material_20261003a_restore_migrate_bc5f6f'
assert os.geteuid()==0
info=json.loads(subprocess.check_output(['docker','inspect',name]))[0]
raw=subprocess.run(['docker','logs',name],capture_output=True)
path=Path('/var/backups/omsbizfoundry/oa-bidding-material-20261003a/failed-restore-migration-private.log')
if not path.exists():
    with os.fdopen(os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'wb') as target:
        target.write(raw.stdout+raw.stderr)
text=(raw.stdout+raw.stderr).decode(errors='replace').lower()
phrases=['permission denied','syntax error','unknown migration','checksum','dirty','already exists','duplicate','does not exist','connection refused','password authentication failed','migration','index','constraint','role','unsupported','failed']
safe=(raw.stdout+raw.stderr).decode(errors='replace')
for item in info['Config']['Env']:
    key,value=item.split('=',1)
    if value and any(term in key for term in ('PASSWORD','TOKEN','SECRET','DATABASE_URL','PRIVATE')):safe=safe.replace(value,'[REDACTED]')
safe=re.sub(r'postgres(?:ql)?://\S+','[PRIVATE DSN]',safe)
print(json.dumps({'exit':info['State']['ExitCode'],'oom':info['State']['OOMKilled'],'matching_terms':[x for x in phrases if x in text],'log_bytes':len(raw.stdout+raw.stderr),'raw_private_log_retained':True,'redacted_error':safe[:500]}))
