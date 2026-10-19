// Immutable scoped source first; binaries are built from its extracted tree.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {copyFileSync,readFileSync,writeFileSync,existsSync,mkdirSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../../',import.meta.url)),[out,stage,mode]=process.argv.slice(2);
assert(/^\/tmp\/oa-cost-release\.[A-Za-z0-9]+$/.test(out)&&/^\/tmp\/oa-cost-release-source\.[A-Za-z0-9]+$/.test(stage));
const archive=out+'/source.tar.gz',artifacts=out+'/artifacts';
const hash=p=>createHash('sha256').update(readFileSync(p)).digest('hex');
if(mode==='source'){
 assert(!existsSync(archive));
 execFileSync('tar',['-czf',archive,'--exclude=node_modules','--exclude=.npm-cache','--exclude=.runtime','--exclude=.env','--exclude=__pycache__','--exclude=dist','--exclude=bin','--exclude=*.test',
  'backend/services/cost-outsourcing','frontend/oms-workbench','backend/services/project-workspace/internal/projects/model.go','packages/workload-identity','packages/payable-suggestion-contract','packages/file-transfer-core',
  'contracts/access/v1/examples/cost-outsourcing-capabilities.json','contracts/access/v1/examples/project-workspace-capabilities.json','deployments/d1/ops/render-suite-manifest','deployments/d1/compose.yaml',
  'acceptance/oa-initiation-repair/cost-release-ecs.py','acceptance/oa-initiation-repair/package-cost.mjs','acceptance/oa-initiation-repair/commercial-billing.Dockerfile','acceptance/oa-initiation-repair/contributors-workbench.Dockerfile'],{cwd:root});
 execFileSync('tar',['-xzf',archive,'-C',stage]);console.log(JSON.stringify({source_archive_sha256:hash(archive)}));
}else{
 assert(mode==='manifest'&&existsSync(archive)&&!existsSync(artifacts+'/manifest.json'));
 const applications={};
 for(const app of ['cost-outsourcing','unified-workbench']){
  const template=app==='unified-workbench'?'contributors-workbench.Dockerfile':'commercial-billing.Dockerfile';
  copyFileSync(stage+'/acceptance/oa-initiation-repair/'+template,artifacts+'/'+app+'/Dockerfile');
  applications[app]=Object.fromEntries((app==='unified-workbench'?['server']:['server','ops','workload-keyctl']).map(n=>[n,hash(artifacts+'/'+app+'/'+n)]));
 }
 mkdirSync(artifacts+'/contracts',{recursive:true});copyFileSync(stage+'/contracts/access/v1/examples/cost-outsourcing-capabilities.json',artifacts+'/contracts/cost-outsourcing-capabilities.json');
 for(const name of ['reference_oa_react_bidding.js','reference_oa_react_bidding.css'])assert.equal(hash(stage+'/frontend/oms-workbench/internal/server/'+name),hash(root+'frontend/oms-workbench/internal/server/'+name));
 const manifest={version:'v0.1.0-oa.20261004.5',official_release:false,source_kind:'saved scoped dirty-worktree candidate; binaries built from extracted source',source_archive_sha256:hash(archive),applications,
  closure:['Cost selected typed reference-oa-cost-v1 schema2->3','Workbench reused original business/production cost lists/forms and native commercial Link receipt fix','IAM/Gateway same binaries; typed Cost capability resource kinds','Cost workload exact-file metadata permission'],
  assets:Object.fromEntries(['reference_oa_react_bidding.js','reference_oa_react_bidding.css'].map(n=>[n,hash(stage+'/frontend/oms-workbench/internal/server/'+n)])),grant_writes_by_release:0,full_oa_accepted:false};
 writeFileSync(artifacts+'/manifest.json',JSON.stringify(manifest,null,2)+'\n',{flag:'wx',mode:0o600});console.log(JSON.stringify(manifest));
}
