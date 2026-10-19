// Curated evidence only: never copy login state, dumps, env, keys, or SQL data.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,readFileSync,writeFileSync,copyFileSync,existsSync} from 'node:fs';
const destination='acceptance/oa-initiation-repair/evidence/cost-public-20261004';
mkdirSync(destination,{recursive:true});
const root='/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a',backup='/var/backups/omsbizfoundry/oa-cost-release-20261004a';
const paths={publication:root+'/publication/results.json','closing-before-grants':root+'/closing-before-grants.json','closing-after-cost-public-first-three':root+'/closing-after-cost-public-first-three.json','closing-after-cost-public':root+'/closing-after-cost-public.json','offhost-confirmed':root+'/offhost-confirmed.json',preflight:backup+'/preflight.json','cold-backup':backup+'/cold-backup.json',restore:backup+'/restore.json',standalone:backup+'/standalone.json',rollout:backup+'/rollout.json'};
const code='import json,pathlib; p='+JSON.stringify(paths)+';print(json.dumps({k:json.loads(pathlib.Path(v).read_text()) for k,v in p.items()}))';
const encoded=Buffer.from(code).toString('base64');
const remote=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${encoded}"))'`],{encoding:'utf8',timeout:30000});
assert.equal(remote.status,0,'curated release receipts unavailable');
function save(name,value){const serialized=JSON.stringify(value,null,2)+'\n';assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(serialized),'private login/configuration state is forbidden');writeFileSync(destination+'/'+name+'.json',serialized);}
for(const [name,value]of Object.entries(JSON.parse(remote.stdout)))save(name,value);
save('native-iam-cost-grants',JSON.parse(readFileSync('/tmp/oa-cost-grants.gEsnme/grants.json')));
for(const name of ['browser-attempt1','browser-attempt2','browser-attempt3','browser'])if(existsSync('/tmp/oa-cost-public.BwFxe2/'+name+'.json'))save(name,JSON.parse(readFileSync('/tmp/oa-cost-public.BwFxe2/'+name+'.json')));
for(const name of ['business-history.png','production-cost.png','cost-file-preview.png','withdrawn-cost-history.png','clean-continuation-history.png','business-final.png','production-final.png'])if(existsSync('/tmp/oa-cost-public.BwFxe2/'+name))copyFileSync('/tmp/oa-cost-public.BwFxe2/'+name,destination+'/'+name);
console.log(JSON.stringify({destination,private_state_copied:false}));
