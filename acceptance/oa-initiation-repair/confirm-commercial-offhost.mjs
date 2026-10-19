// Generated private backup/receipt artifacts only; never writes source or keys.
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {chmodSync,existsSync,mkdirSync,readFileSync,statSync,writeFileSync} from 'node:fs';
const family=process.argv[2];assert(['commercial-links','cost-release','cost-modal'].includes(family));
const name='oa-'+family+'-20261004a',root='/opt/omsbizfoundry/acceptance/'+name;
const host='ecs-user@47.98.156.166',ssh=['-o','BatchMode=yes','-o','ConnectTimeout=8','-o','StrictHostKeyChecking=yes'];
const directory='/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/auth58-private-backups';
const local=directory+'/'+name+'-backup.tar.gz',receiptPath=directory+'/'+name+'-offhost-confirmed.json';
function run(cmd,args){const r=spawnSync(cmd,args,{encoding:'utf8',timeout:60000});assert.equal(r.status,0,'private backup operation failed');return r.stdout;}
assert(!existsSync(local)&&!existsSync(receiptPath));mkdirSync(directory,{mode:0o700,recursive:true});chmodSync(directory,0o700);
const expected=JSON.parse(run('ssh',[...ssh,host,'sudo cat '+root+'/offhost-export.json']));assert(/^[a-f0-9]{64}$/.test(expected.archive_sha256)&&Number.isSafeInteger(expected.bytes)&&expected.bytes>0);
run('scp',[...ssh,host+':/home/ecs-user/'+name+'-backup.tar.gz',local]);chmodSync(local,0o600);
const digest=createHash('sha256').update(readFileSync(local)).digest('hex');assert.equal(digest,expected.archive_sha256);assert.equal(statSync(local).size,expected.bytes);
const receipt={passed:true,archive_sha256:digest,bytes:expected.bytes,private_archive_mode:'0600',private_parent_mode:'0700',outside_repository:true};
writeFileSync(receiptPath,JSON.stringify(receipt,null,2)+'\n',{flag:'wx',mode:0o600});
run('scp',[...ssh,receiptPath,host+':/home/ecs-user/'+name+'-offhost-confirmed.json']);
run('ssh',[...ssh,host,'sudo install -m 0600 /home/ecs-user/'+name+'-offhost-confirmed.json '+root+'/offhost-confirmed.json']);console.log(JSON.stringify(receipt));
