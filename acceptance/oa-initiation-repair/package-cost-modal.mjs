// Scoped immutable no-migration Workbench repair; only generated artifacts.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {copyFileSync,readFileSync,writeFileSync,existsSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../../',import.meta.url)),[out,stage,mode]=process.argv.slice(2);
assert(/^\/tmp\/oa-cost-modal\.[A-Za-z0-9]+$/.test(out)&&/^\/tmp\/oa-cost-release-source\.[A-Za-z0-9]+$/.test(stage));
const archive=out+'/source.tar.gz',artifacts=out+'/artifacts',hash=p=>createHash('sha256').update(readFileSync(p)).digest('hex');
if(mode==='source'){
 assert(!existsSync(archive));
 execFileSync('tar',['-czf',archive,'--exclude=node_modules','--exclude=.npm-cache','--exclude=.runtime','--exclude=.env','--exclude=__pycache__','--exclude=dist','--exclude=bin','--exclude=*.test',
  'frontend/oms-workbench','packages/file-transfer-core','contracts/access/v1/examples/project-workspace-capabilities.json','contracts/access/v1/examples/business-engagement-capabilities.json','backend/services/project-workspace/internal/projects/model.go','backend/services/project-workspace/internal/auth/auth.go','backend/services/business-engagement/internal/httpserver/server.go','deployments/d1/ops/render-suite-manifest',
  'acceptance/oa-initiation-repair/login-csrf-release-ecs.py','acceptance/oa-initiation-repair/package-cost-modal.mjs','acceptance/oa-initiation-repair/contributors-workbench.Dockerfile','acceptance/oa-initiation-repair/verify-cost-components-browser.mjs','go.work','go.work.sum'],{cwd:root});
 execFileSync('tar',['-xzf',archive,'-C',stage]);console.log(JSON.stringify({source_archive_sha256:hash(archive)}));
}else{
 assert(mode==='manifest'&&existsSync(archive)&&!existsSync(artifacts+'/manifest.json'));
 copyFileSync(stage+'/acceptance/oa-initiation-repair/contributors-workbench.Dockerfile',artifacts+'/unified-workbench/Dockerfile');
 const assets=Object.fromEntries(['reference_oa_react_bidding.js','reference_oa_react_bidding.css'].map(n=>{
  assert.equal(hash(stage+'/frontend/oms-workbench/internal/server/'+n),hash(root+'frontend/oms-workbench/internal/server/'+n));return[n,hash(stage+'/frontend/oms-workbench/internal/server/'+n)];
 }));
 const manifest={version:'v0.1.0-oa.20261004.6',official_release:false,source_kind:'saved scoped dirty-worktree; original Cost drawer and confirmation layering fix, binary from extracted source',source_archive_sha256:hash(archive),schema_change:false,closure:['unified-workbench'],applications:{'unified-workbench':{server:hash(artifacts+'/unified-workbench/server')}},assets,full_oa_accepted:false};
 writeFileSync(artifacts+'/manifest.json',JSON.stringify(manifest,null,2)+'\n',{flag:'wx',mode:0o600});console.log(JSON.stringify(manifest));
}
