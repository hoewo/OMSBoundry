// Generated backup receipt only; archive is private and never staged in Git.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {mkdirSync,readFileSync,writeFileSync,existsSync,chmodSync,statSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
const host='ecs-user@47.98.156.166',options=['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes'];
const root='/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a';
const local=fileURLToPath(new URL('.runtime/projection-visibility-20261003a/',import.meta.url));
function run(command,args){const result=spawnSync(command,args,{encoding:'utf8',timeout:40000});assert.equal(result.status,0,'private backup operation failed');return result.stdout}
assert.equal(spawnSync('git',['check-ignore',local+'backup.tar.gz'],{encoding:'utf8'}).status,0);
mkdirSync(local,{recursive:true,mode:0o700});chmodSync(local,0o700);assert(!existsSync(local+'backup.tar.gz'));
const expected=JSON.parse(run('ssh',[...options,host,'sudo cat '+root+'/offhost-export.json']));
assert(/^[a-f0-9]{64}$/.test(expected.archive_sha256));assert.equal(expected.bytes,240901);
run('scp',[...options,host+':/home/ecs-user/oa-projection-visibility-20261003a-backup.tar.gz',local+'backup.tar.gz']);chmodSync(local+'backup.tar.gz',0o600);
const digest=createHash('sha256').update(readFileSync(local+'backup.tar.gz')).digest('hex');assert.equal(digest,expected.archive_sha256);assert.equal(statSync(local+'backup.tar.gz').size,expected.bytes);
const receipt={passed:true,archive_sha256:digest,bytes:expected.bytes,private_archive_mode:'0600',local_archive_git_ignored:true};
writeFileSync(local+'offhost-confirmed.json',JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});
run('scp',[...options,local+'offhost-confirmed.json',host+':/home/ecs-user/oa-projection-visibility-20261003a-offhost-confirmed.json']);
run('ssh',[...options,host,'sudo install -m 0600 /home/ecs-user/oa-projection-visibility-20261003a-offhost-confirmed.json '+root+'/offhost-confirmed.json']);
console.log(JSON.stringify(receipt));
