// Exact C category matrix receipts; never copy storage, sessions or backups.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,readFileSync,writeFileSync,copyFileSync} from 'node:fs';
const source='/tmp/oa-cost-category.HvqnNw',destination='acceptance/oa-initiation-repair/evidence/cost-category-public-20261004';
mkdirSync(destination,{recursive:true});
function save(name,value){const text=JSON.stringify(value,null,2)+'\n';assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(text));writeFileSync(destination+'/'+name+'.json',text);}
const result=JSON.parse(readFileSync(source+'/browser.json'));assert.equal(result.passed,true);assert.equal(result.scenarios.length,8);assert.equal(result.commands.length,16);save('browser',result);
for(let n=1;n<=6;n++)save('browser-attempt'+n,JSON.parse(readFileSync(source+'/browser-attempt'+n+'.json')));
const path='/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/cost-category-matrix/closing.json';
const code=Buffer.from('import json,pathlib;print(json.dumps(json.loads(pathlib.Path('+JSON.stringify(path)+').read_text())))').toString('base64');
const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${code}"))'`],{encoding:'utf8',timeout:30000});
assert.equal(r.status,0,'native closing is required');const closing=JSON.parse(r.stdout);assert.equal(closing.passed,true);save('native-closing',closing);
for(const name of ['business-dropdown.png','production-dropdown.png','business-all-categories.png','production-all-categories.png','modal-covered-before.png'])copyFileSync(source+'/'+name,destination+'/'+name);
console.log(JSON.stringify({destination,private_state_copied:false,full_role_acceptance:false,full_oa_accepted:false}));
