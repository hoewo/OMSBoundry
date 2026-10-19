// Curated non-private receipts only; deliberately exclude archives and sessions.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {readFileSync,writeFileSync,mkdirSync,copyFileSync,existsSync} from 'node:fs';
const destination='acceptance/oa-initiation-repair/evidence/cost-modal-20261004';
mkdirSync(destination,{recursive:true});
const safe=(name,value)=>{const text=JSON.stringify(value,null,2)+'\n';assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(text));writeFileSync(destination+'/'+name+'.json',text);};
for(const[name,path]of [
 ['old-component-covered-confirmation','/tmp/oa-cost-components.ujSOOp/browser.json'],
 ['fixed-component-normal-confirmations','/tmp/oa-cost-components.lH8Fq5/browser.json'],
 ['standalone-startup','/tmp/oa-cost-modal.xxGT4L/startup.json'],
 ['original-source-provenance','/tmp/oa-cost-modal.xxGT4L/original-source-provenance.json'],
 ['candidate-manifest','/tmp/oa-cost-modal.xxGT4L/artifacts/manifest.json'],
 ['public-covered-confirmation','/tmp/oa-cost-category.HvqnNw/browser-attempt5.json']
])safe(name,JSON.parse(readFileSync(path)));
copyFileSync('/tmp/oa-cost-category.HvqnNw/modal-covered-before.png',destination+'/public-covered-confirmation.png');
const tests=readFileSync('/tmp/oa-cost-modal.xxGT4L/node-tests.log','utf8');
assert(/pass 282\b/.test(tests)&&/fail 0\b/.test(tests)&&/skipped 0\b/.test(tests));
safe('node-tests',{passed:true,tests:282,failures:0,skipped:0,source:'extracted immutable candidate',public_ui_accepted:false});
const root='/opt/omsbizfoundry/acceptance/oa-cost-modal-20261004a',backup='/var/backups/omsbizfoundry/oa-cost-modal-20261004a';
for(const[name,path]of [
 ['publication',root+'/publication/results.json'],['preflight',backup+'/preflight.json'],
 ['cold-backup',backup+'/cold-backup.json'],['empty-target-restore',backup+'/restore.json'],
 ['offhost-confirmed',root+'/offhost-confirmed.json'],['rollout',backup+'/rollout.json']
]){
 const code=Buffer.from('import json,pathlib;print(json.dumps(json.loads(pathlib.Path('+JSON.stringify(path)+').read_text())))').toString('base64');
 const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${code}"))'`],{encoding:'utf8',timeout:30000});
 assert.equal(r.status,0,'required release receipt missing: '+name);const value=JSON.parse(r.stdout);assert.equal(value.passed,true);safe(name,value);
}
console.log(JSON.stringify({destination,private_state_copied:false,full_suite_restore_runtime:false,full_oa_accepted:false}));
