#!/usr/bin/env python3
"""Explicitly whitelist public receipt metadata; no env, DB dumps or key files."""
import hashlib,json,os,pwd,tarfile
from pathlib import Path
assert os.geteuid()==0
root=Path('/opt/omsbizfoundry/acceptance/oa-bidding-material-release-20261003a')
backup=Path('/var/backups/omsbizfoundry/oa-bidding-material-20261003a')
selected=[(root/name,'release/'+name) for name in ('artifacts/manifest.json','publication/results.json','offhost-export.json','material-browser-ownership/before.json','material-browser-ownership/after.json')]
selected += [(backup/(name+'.json'),'backup/'+name+'.json') for name in ('preflight','cold-backup','restore','rollout')]
assert all(path.is_file() for path,_ in selected)
for path,_ in selected:json.loads(path.read_text())
assert json.loads((backup/'rollout.json').read_text())['passed']
assert json.loads((root/'material-browser-ownership/after.json').read_text())['passed']
target=Path('/home/ecs-user/oa-bidding-material-safe-evidence-20261003a.tar.gz')
assert not target.exists(),'do not overwrite a receipt export'
with tarfile.open(target,'x:gz') as archive:
    for path,name in selected:archive.add(path,arcname=name,recursive=False)
target.chmod(0o600);account=pwd.getpwnam('ecs-user');os.chown(target,account.pw_uid,account.pw_gid)
print(json.dumps({'selected_json_files':len(selected),'archive_sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'bytes':target.stat().st_size,'contains_private_backup':False}),flush=True)
