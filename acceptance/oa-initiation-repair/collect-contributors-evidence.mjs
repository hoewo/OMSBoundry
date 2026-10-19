// Copy only declared safe receipts. Never export env, dumps, keys or private logs.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync,copyFileSync,readdirSync,existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
const base=fileURLToPath(new URL('evidence/contributors-20261003/',import.meta.url));
const roots={
 pg:'/opt/omsbizfoundry/acceptance/oa-initiation-repair-20261002y',
 startup:'/opt/omsbizfoundry/acceptance/oa-initiation-repair-20261002x',
 contract:'/opt/omsbizfoundry/acceptance/oa-contract-native-20261003a',
 release:'/opt/omsbizfoundry/acceptance/oa-contributors-release-20261003a',
 backup:'/var/backups/omsbizfoundry/oa-contributors-20261003a',
 route:'/opt/omsbizfoundry/acceptance/oa-personnel-route-20261003a',
 route_backup:'/var/backups/omsbizfoundry/oa-personnel-route-20261003a',
 refresh:'/opt/omsbizfoundry/acceptance/oa-personnel-refresh-20261003a',
 refresh_backup:'/var/backups/omsbizfoundry/oa-personnel-refresh-20261003a',
 visibility:'/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a',
 visibility_backup:'/var/backups/omsbizfoundry/oa-projection-visibility-20261003a',
 contract_startup_a:'/opt/omsbizfoundry/acceptance/oa-contract-native-startup-20261003a',
 contract_startup_b:'/opt/omsbizfoundry/acceptance/oa-contract-native-startup-20261003b',
 contract_startup_c:'/opt/omsbizfoundry/acceptance/oa-contract-native-startup-20261003c',
};
const files={pg:['results.json'],startup:['results.json'],contract:['results.json'],release:['artifacts/manifest.json','publication/results.json','offhost-export.json','offhost-confirmed.json','browser-ownership/before.json'],backup:['preflight.json','cold-backup.json','restore.json','rollout.json']};
if(process.env.OA_CONTRIBUTORS_COLLECT_FINAL==='1'){
 files.release.push('browser-ownership/after.json','browser-ownership/interrupted-readback.json','browser-ownership/mismatch-diagnostic-initial.json','browser-ownership/login-metadata.json');
 files.route=['artifacts/manifest.json','publication/results.json','offhost-export.json','offhost-confirmed.json'];
 files.route_backup=['preflight.json','cold-backup.json','restore.json','rollout.json'];
 files.refresh=['artifacts/manifest.json','publication/results.json','offhost-export.json','offhost-confirmed.json'];
 files.refresh_backup=['preflight.json','cold-backup.json','restore.json','rollout.json'];
 files.visibility=['artifacts/manifest.json','publication/results.json','offhost-export.json','offhost-confirmed.json','closing-check.json'];
 files.visibility_backup=['preflight.json','cold-backup.json','restore.json','rollout.json'];
 for(const key of ['contract_startup_a','contract_startup_b','contract_startup_c'])files[key]=['results.json'];
}
function inspect(value){
 if(!value||typeof value!=='object')return;
 for(const [key,item] of Object.entries(value)){
  assert(!/^(?:password|secret|access_token|private_key|database_url|connection_string|credential_value)$/i.test(key),'private value not permitted in safe receipt');
  inspect(item);
 }
}
let copied=0;const exports=[];
for(const [kind,names] of Object.entries(files))for(const name of names){
 const remote=roots[kind]+'/'+name;
 const r=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166','sudo cat '+remote],{encoding:'utf8',timeout:20000});
 assert.equal(r.status,0,'declared safe receipt unavailable');
 const value=JSON.parse(r.stdout);inspect(value);
 const digest=spawnSync('ssh',['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes','ecs-user@47.98.156.166','sudo sha256sum '+remote],{encoding:'utf8',timeout:20000});
 assert.equal(digest.status,0);const hash=createHash('sha256').update(r.stdout).digest('hex');assert.equal(digest.stdout.split(/\s+/)[0],hash,'safe receipt changed during export');
 const dest=base+kind+'/'+name;mkdirSync(dest.slice(0,dest.lastIndexOf('/')),{recursive:true});
 writeFileSync(dest,r.stdout,{mode:0o600});exports.push({file:kind+'/'+name,sha256:hash,remote_bytes_equal:true});copied++;
}
for(const [directory,names] of Object.entries({
 '/tmp/oa-complete-operations.TEfzlpv7':['component-browser-a','component-browser-b','component-browser-c','component-browser-d','public-a','public-b','public-c','public-d','public-e-readonly','public-f-continuation','public-g-continuation','public-h-continuation','public-i-continuation','public-j-continuation','public-k-navigation','public-l-navigation','public-m-navigation','public-n-navigation'],
 '/tmp/oa-personnel-route.17emhSdL':['standalone-startup.json'],
 '/tmp/oa-personnel-refresh.pdmMm5uI':['standalone-startup.json','provenance-checks.json'],
 '/tmp/oa-projection-visibility.nCFRYc8X':['standalone-startup.json'],
}))for(const name of names){
 const path=directory+'/'+name;if(!existsSync(path))continue;
 if(name.endsWith('.json')){const dest=base+(directory.includes('projection-visibility')?'visibility-':directory.includes('personnel-refresh')?'refresh-':'')+name;copyFileSync(path,dest);copied++;continue;}
 mkdirSync(base+name,{recursive:true});
 for(const f of readdirSync(path).filter(f=>/^(?:results|browser)\.json$|\.png$/.test(f))){copyFileSync(path+'/'+f,base+name+'/'+f);copied++;}
}
writeFileSync(base+'safe-export-checks.json',JSON.stringify({passed:true,private_backups_exported:false,files:exports},null,2)+'\n',{mode:0o600});
console.log(JSON.stringify({safe_receipts_and_screenshots:copied,private_backups_exported:false}));
