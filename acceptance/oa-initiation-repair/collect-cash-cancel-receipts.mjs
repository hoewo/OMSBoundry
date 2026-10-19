// Curated UI cancellation evidence, without private browser or native state.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {readFileSync,writeFileSync,mkdirSync,copyFileSync} from 'node:fs';
const source='/tmp/oa-cash-cancel.sVqKIe',destination='acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004';
mkdirSync(destination,{recursive:true});
function save(name,value){const text=JSON.stringify(value,null,2)+'\n';assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(text));writeFileSync(destination+'/'+name+'.json',text);}
const browser=JSON.parse(readFileSync(source+'/browser.json'));assert.equal(browser.passed,true);assert.equal(browser.scenarios.length,6);assert.equal(browser.blocked_business_mutations.length,0);save('browser',browser);
const path='/opt/omsbizfoundry/acceptance/oa-cost-modal-20261004a/cash-cancel-readonly/closing.json';
const code=Buffer.from('import json,pathlib;print(json.dumps(json.loads(pathlib.Path('+JSON.stringify(path)+').read_text())))').toString('base64');
const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${code}"))'`],{encoding:'utf8',timeout:30000});
assert.equal(r.status,0,'native zero-write closing required');const closing=JSON.parse(r.stdout);assert.equal(closing.passed,true);save('native-closing',closing);
for(const chain of ['client','income'])for(let fee=0;fee<3;fee++)copyFileSync(source+'/'+chain+'-'+fee+'-form.png',destination+'/'+chain+'-'+fee+'-form.png');
console.log(JSON.stringify({destination,private_state_copied:false,full_oa_accepted:false}));
