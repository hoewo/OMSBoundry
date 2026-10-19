// Curate actual UI receipts; private credentials, SQL and backups stay in /tmp.
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,copyFileSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
const destination='acceptance/oa-initiation-repair/evidence/production-layout-20261004';
mkdirSync(destination,{recursive:true});
const safe=(name,value)=>{
  const text=JSON.stringify(value,null,2)+'\n';
  assert(!/"(?:password|access_token|refresh_token|private_key|database_url|cookies|storage|session)"\s*:/i.test(text),'private field in '+name);
  writeFileSync(destination+'/'+name+'.json',text);
};
const receipts=[
  ['native-workbench','/tmp/oa-bonus-components.jiONro/browser.json'],
  ['cost-components','/tmp/oa-cost-components.2ngylA/browser.json'],
  ['contract-components','/tmp/oa-contract-components.aI9BoJ/browser.json'],
  ['cash-components','/tmp/oa-cash-components.4l9nKV/browser.json'],
  ['invoice-components','/tmp/oa-invoice-components.ZPVPgg/browser.json'],
  ['standalone-startup','/tmp/oa-material-workbench.vFXboE/startup.json'],
  ['public-commercial-readonly','/tmp/oa-commercial-visible.j0E0tC/visible.json'],
  ['failed-tab-icon-locator','/tmp/oa-bonus-components.hAHrGN/browser.json'],
  ['failed-tab-live-label-locator','/tmp/oa-bonus-components.R7n5nF/browser.json'],
  ['failed-contract-stale-capability','/tmp/oa-contract-components.QuzXAP/browser.json'],
  ['failed-cash-stale-capability','/tmp/oa-cash-components.yjb9RH/browser.json'],
  ['failed-invoice-stale-capability','/tmp/oa-invoice-components.A3xX1G/browser.json'],
  ['failed-contract-stale-receipt','/tmp/oa-contract-components.9VrAaa/browser.json'],
  ['failed-cash-stale-receipt','/tmp/oa-cash-components.1KBtQT/browser.json'],
  ['failed-invoice-stale-receipt','/tmp/oa-invoice-components.l3uFCD/browser.json'],
  ['failed-invoice-stale-card-key','/tmp/oa-invoice-components.ah3LxT/browser.json'],
];
for(const [name,path] of receipts)safe(name,JSON.parse(readFileSync(path)));
const screenshots=[
  ['production-overview','/tmp/oa-bonus-components.jiONro/workbench-production-overview.png'],
  ['production-bonus-form','/tmp/oa-bonus-components.jiONro/workbench-production-form.png'],
  ['production-bonus-saved','/tmp/oa-bonus-components.jiONro/workbench-production-saved.png'],
  ['production-bonus-history','/tmp/oa-bonus-components.jiONro/workbench-production-history.png'],
  ['business-bonus-redacted','/tmp/oa-bonus-components.jiONro/workbench-business-redacted.png'],
  ['production-cost','/tmp/oa-cost-components.2ngylA/production-layout-cost.png'],
  ['contract-settlements','/tmp/oa-commercial-visible.j0E0tC/contract-settlement-history.png'],
  ['billing-scope-required','/tmp/oa-commercial-visible.j0E0tC/billing-scope-required.png'],
];
for(const [name,path] of screenshots){assert(existsSync(path));copyFileSync(path,destination+'/'+name+'.png')}
const tests=readFileSync('/tmp/oa-material-workbench.vFXboE/node-tests.log','utf8');
assert(/pass 307\b/.test(tests)&&/fail 0\b/.test(tests)&&/skipped 0\b/.test(tests));
safe('node-tests',{passed:true,tests:307,failed:0,skipped:0,frontend:125,embedded_runtime:182,public_ui_accepted:false});
const source='/Users/crispydog/Workspace/Hoewo/WebProjects/RoadSystemOA/road-design-oa-system/frontend';
const provenance=JSON.parse(readFileSync('frontend/oms-workbench/frontend/source-provenance.json'));
for(const file of provenance.files)assert.equal(createHash('sha256').update(readFileSync(source+'/'+file.path)).digest('hex'),file.sha256);
const css='src/components/production/ProductionInfo.css',digest=provenance.files.find(f=>f.path===css).sha256;
assert.equal(createHash('sha256').update(readFileSync('frontend/oms-workbench/frontend/vendor/components/production/ProductionInfo.css')).digest('hex'),digest);
safe('original-source-provenance',{passed:true,copied_files:provenance.files.length,source_writes:0,production_css_verbatim:true,production_css_sha256:digest,adapted_ui_equivalence:false,public_ui_verified:false});
console.log(JSON.stringify({destination,private_state_copied:false,public_new_candidate_deployed:false,full_oa_accepted:false}));
