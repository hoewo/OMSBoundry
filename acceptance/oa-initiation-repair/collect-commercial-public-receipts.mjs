// Explicit curated receipts and visible screenshots; never private recovery.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,readFileSync,writeFileSync,copyFileSync,existsSync} from 'node:fs';
const destination='acceptance/oa-initiation-repair/evidence/commercial-public-20261004';
const source='/tmp/oa-commercial-journey.35jPS3';
const visibleSource='/tmp/oa-commercial-visible.CKxo3l';
mkdirSync(destination,{recursive:true});
function save(name,value){
 const serialized=JSON.stringify(value,null,2)+'\n';
 assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(serialized),'private login/configuration state is forbidden');
 writeFileSync(destination+'/'+name+'.json',serialized);
}
for(const name of ['browser-attempt1','browser-attempt2','browser-attempt3','browser-attempt4','browser-attempt4-before-wb5','browser-attempt5','browser-attempt6','browser-attempt7','browser-attempt8','browser-attempt9','browser-attempt10','browser']){
 if(existsSync(source+'/'+name+'.json'))save(name,JSON.parse(readFileSync(source+'/'+name+'.json')));
}
const receipt='/opt/omsbizfoundry/acceptance/oa-cost-release-20261004a/commercial-public-footprint.json';
const encoded=Buffer.from('import json,pathlib;print(json.dumps(json.loads(pathlib.Path('+JSON.stringify(receipt)+').read_text())))').toString('base64');
const remote=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166',`sudo python3 -c 'import base64;exec(base64.b64decode("${encoded}"))'`],{encoding:'utf8',timeout:30000});
assert.equal(remote.status,0,'native footprint receipt unavailable; do not publish unverified closing');
const footprint=JSON.parse(remote.stdout);assert.equal(footprint.passed,true);save('native-footprint',footprint);
for(const name of ['contract-three-confirmed-amendments.png','contract-final-settlement.png','contract-active-file.png','cash-both-chains.png','contracts-final.png','payments-final.png','invoices-final.png','invoice-preview.png','invoice-history.png','failure.png']){
 if(existsSync(source+'/'+name))copyFileSync(source+'/'+name,destination+'/'+name);
}
for(const name of ['visible-attempt1','visible-attempt2','visible']){
 if(existsSync(visibleSource+'/'+name+'.json'))save(name,JSON.parse(readFileSync(visibleSource+'/'+name+'.json')));
}
for(const name of ['contract-settlement-history.png','contract-amendment-history.png','contract-visible-preview.png','billing-scope-required.png'])copyFileSync(visibleSource+'/'+name,destination+'/'+name);
console.log(JSON.stringify({destination,private_state_copied:false,full_oa_accepted:false}));
