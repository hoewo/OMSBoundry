// Seal only already-run, non-secret receipts. Backups/env/keys stay private.
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,copyFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const dir=process.argv[2];assert(/^\/tmp\/oa-professional-d1\.[A-Za-z0-9]+$/.test(dir||''));
const destination='acceptance/oa-initiation-repair/evidence/professional-d1-20261004';mkdirSync(destination,{recursive:true});
function safe(name,value){const text=JSON.stringify(value,null,2)+'\n';assert(!/"(?:password|token|access_token|refresh_token|private_key|database_url|cookies|session)"\s*:/i.test(text));writeFileSync(destination+'/'+name+'.json',text)}
const result=JSON.parse(readFileSync(dir+'/d1-browser.json'));assert(result.passed&&result.scenarios.length===14&&result.page_errors.length===0);assert(result.cleanup.runtime_stopped&&Object.values(result.cleanup.remaining).every(v=>v===0));assert.deepEqual(result.before,result.restored);
for(const key of ['main_changed','public_tls_accepted','clean_host_release_accepted','full_suite_accepted','full_oa_accepted'])assert.equal(result[key],false);
safe('d1-browser',result);
for(const [name,suffix] of [['failed-auto-app-network','cNnIkv'],['failed-auto-default-network','WLEuDg'],['failed-client-bearer-edge','3f8i1t'],['failed-original-remark-locator','V7M3LZ']]){const failed=JSON.parse(readFileSync('/tmp/oa-professional-d1.'+suffix+'/d1-browser.json'));assert.equal(failed.passed,false);safe(name,failed)}
for(const name of ['source-original-bonus','restored-original-bonus','restored-original-history','restored-original-personnel'])copyFileSync(dir+'/'+name+'.png',destination+'/'+name+'.png');
const sha=path=>createHash('sha256').update(readFileSync(path)).digest('hex');
const paths=['acceptance/oa-initiation-repair/verify-professional-d1-restore.mjs','deployments/d1/ops/d1','deployments/d1/compose.yaml','deployments/d1/ops/render-suite-manifest','frontend/oms-workbench/internal/server/reference_oa_react_bidding.js','frontend/oms-workbench/internal/server/reference_oa_initiation_runtime.js'];
assert.equal(sha('frontend/oms-workbench/internal/server/reference_oa_react_bidding.js'),result.normal_asset_sha256);
safe('provenance',{source_commit:'826e0fc65aa09947fc4f64f62a2644fa5d11a302',files:paths.map(path=>({path,sha256:sha(path)})),private_backup_copied:false,private_configuration_copied:false,fixture_mode:'Actual D1 script, selected composition, local HTTP/build mode. Native APIs seed facts; readonly per-owner SQL computes digests. No Main or registry push.',runtime_stopped:true});
console.log(JSON.stringify({destination,passed:true,scenarios:14}));
