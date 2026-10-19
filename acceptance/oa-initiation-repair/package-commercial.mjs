// Build output only; no credentials or unrelated dirty source are packaged.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {copyFileSync,readFileSync,writeFileSync,existsSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
const root=fileURLToPath(new URL('../../',import.meta.url)),out=process.argv[2];
assert(/^\/tmp\/oa-commercial-release\.[A-Za-z0-9]+$/.test(out));
const artifacts=out+'/artifacts',archive=out+'/source.tar.gz';
assert(!existsSync(archive)&&!existsSync(artifacts+'/manifest.json'));
const applications={};
const hash=p=>createHash('sha256').update(readFileSync(p)).digest('hex');
for(const app of ['contract-ledger','money-flow-ledger','billing-invoice','unified-workbench']){
  const template=app==='unified-workbench'?'contributors-workbench.Dockerfile':app==='billing-invoice'?'commercial-billing.Dockerfile':'commercial-ledger.Dockerfile';
  copyFileSync(fileURLToPath(new URL(template,import.meta.url)),artifacts+'/'+app+'/Dockerfile');
  const names=app==='unified-workbench'?['server']:app==='billing-invoice'?['server','ops','workload-keyctl']:['server','ops','backup','workload-keyctl'];
  applications[app]=Object.fromEntries(names.map(n=>[n,hash(artifacts+'/'+app+'/'+n)]));
}
execFileSync('tar',['-czf',archive,'--exclude=node_modules','--exclude=.npm-cache','--exclude=.runtime','--exclude=.env','--exclude=__pycache__','--exclude=dist','--exclude=bin','--exclude=*.test',
  'backend/services/contract-ledger','backend/services/money-flow-ledger','backend/services/billing-invoice','frontend/oms-workbench','packages/workload-identity','packages/payable-suggestion-contract','packages/file-transfer-core',
  'acceptance/oa-initiation-repair/commercial-release-ecs.py','acceptance/oa-initiation-repair/package-commercial.mjs','contracts/access/v1/examples/workload-identity-policy.json','go.work','go.work.sum'],{cwd:root});
const manifest={version:'v0.1.0-oa.20261004.1',official_release:false,source_kind:'saved scoped dirty-worktree candidate',source_archive_sha256:hash(archive),applications,
  closure:['Contract server/ops/backup schema18->22','Money server/ops/backup schema22->23','Billing server/ops schema1 unchanged','Workbench original contract/payment/invoice UI','IAM same binary; exact-file metadata policy for Contract/Billing'],
  assets:{'reference-oa-react-bidding.js':hash(root+'frontend/oms-workbench/internal/server/reference_oa_react_bidding.js'),'reference-oa-react-bidding.css':hash(root+'frontend/oms-workbench/internal/server/reference_oa_react_bidding.css')},
  grant_writes_by_release:0,full_oa_accepted:false};
writeFileSync(artifacts+'/manifest.json',JSON.stringify(manifest,null,2)+'\n',{flag:'wx',mode:0o600});
console.log(JSON.stringify(manifest));
