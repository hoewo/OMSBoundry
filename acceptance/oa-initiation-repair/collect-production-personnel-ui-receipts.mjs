// Fixed, already-run UI candidate evidence only. Never copy credentials, SQL,
// memory fixture payloads, a database dump, or the private preview build tree.
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,copyFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const destination='acceptance/oa-initiation-repair/evidence/production-personnel-ui-20261004';
mkdirSync(destination,{recursive:true});
const safe=(name,value)=>{
 const serialized=JSON.stringify(value,null,2)+'\n';
 assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(serialized),'private field in '+name);
 writeFileSync(destination+'/'+name+'.json',serialized);
};
const receipt=JSON.parse(readFileSync('/tmp/oa-production-personnel-ui.6oxE30/browser.json'));
assert.equal(receipt.passed,true);assert.equal(receipt.scenarios.length,16);
assert.equal(receipt.native_business_accepted,false);assert.equal(receipt.production_iam,false);
assert.equal(receipt.postgresql,false);assert.equal(receipt.public_ui_accepted,false);
assert.equal(receipt.page_errors.length,0);
safe('browser',receipt);
const superseded=JSON.parse(readFileSync('/tmp/oa-production-personnel-ui.zFFDb7/browser.json'));
safe('superseded-hidden-end-test-contract',{...superseded,superseded:true,reason:'Earlier test incorrectly ended a hidden participant. The 16-scenario receipt verifies preserve on both save and group end, then restoration before explicit end.'});
const failures=[
 ['failed-original-discipline-form-id','S5ANzh'],
 ['failed-edit-icon-locator','dwtBeF'],
 ['failed-required-message-locator','0GY8c8'],
 ['failed-selected-input-click-target','T3WTts'],
 ['failed-relative-locator','I3GrgJ'],
 ['failed-nested-drawer-live-locator','NJacYQ'],
 ['failed-background-refresh-behind-mask','WbC1CJ'],
 ['failed-viewport-during-transition','HEobNs'],
];
for(const [name,id] of failures){const value=JSON.parse(readFileSync('/tmp/oa-production-personnel-ui.'+id+'/browser.json'));assert.equal(value.passed,false);safe(name,value)}
for(const name of ['discipline-original-drawer','professional-roles','reviewers-original-drawer','readonly-original-panel','small-original-drawer'])copyFileSync('/tmp/oa-production-personnel-ui.6oxE30/'+name+'.png',destination+'/'+name+'.png');
const source='/Users/crispydog/Workspace/Hoewo/WebProjects/RoadSystemOA/road-design-oa-system/frontend';
const provenance=JSON.parse(readFileSync('frontend/oms-workbench/frontend/production-personnel-source-provenance.json'));
for(const file of provenance.files)assert.equal(createHash('sha256').update(readFileSync(source+'/'+file.path)).digest('hex'),file.sha256);
const cssDigest=provenance.files.find(f=>f.path.endsWith('/ProductionPersonnelManager.css')).sha256;
assert.equal(createHash('sha256').update(readFileSync('frontend/oms-workbench/frontend/vendor/components/production/personnel/ProductionPersonnelManager.css')).digest('hex'),cssDigest);
const normalBundle=createHash('sha256').update(readFileSync('frontend/oms-workbench/internal/server/reference_oa_react_bidding.js')).digest('hex');
assert.equal(normalBundle,'7df15d8a63a0ac31a308401631aaedba38cbb2f73ba5d1334c4d9e88ed41c357');
assert(!/production-personnel-ui|ProductionPersonnelUI|ProductionPersonnelManager/.test(readFileSync('frontend/oms-workbench/frontend/src/workspace.tsx','utf8')));
safe('source-provenance',{...provenance,passed:true,source_writes:0,personnel_css_sha256:cssDigest,normal_bundle_sha256:normalBundle,main_workbench_imported:false,public_ui_accepted:false});
console.log(JSON.stringify({destination,ui_scenarios:16,private_state_copied:false,native_business_accepted:false,main_deployed:false,full_oa_accepted:false}));
