import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const html=fs.readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');
const css=fs.readFileSync(new URL('./oms_phase1.css',import.meta.url),'utf8');
const identity=fs.readFileSync(new URL('./oms_phase1_identity.js',import.meta.url),'utf8');

test('sidebar exposes current organization without adding another management page',()=>{
  assert.match(html,/data-identity-rail-organization/);
  assert.match(html,/data-identity-rail-tenant-name>正在读取/);
  assert.match(html,/data-identity-rail-organization-menu[^>]+role="listbox"/);
  assert.doesNotMatch(html,/data-oms-v1-nav="organization"/);
});

test('organization switch is only interactive for multiple active memberships',()=>{
  assert.match(identity,/activeOrganizations=organizations\.filter\(item=>item\.status==='active'\)/);
  assert.match(identity,/switchable=ready&&activeOrganizations\.length>1/);
  assert.match(identity,/trigger\.disabled=!switchable/);
  assert.match(identity,/data-identity-rail-organization-option/);
  assert.match(identity,/await post\('\/api\/v1\/session\/organization',\{tenant_id:tenantID\}\);location\.reload\(\)/);
});

test('organization context retries a bounded number of times and exposes manual recovery',()=>{
  assert.match(html,/data-identity-reload hidden>重新加载/);
  assert.match(identity,/for\(let attempt=0;attempt<3;attempt\+=1\)/);
  assert.match(identity,/requireOrganizationReady\(values\)/);
  assert.match(identity,/组织信息未就绪/);
  assert.match(identity,/data-identity-retry-load/);
  assert.match(identity,/data-identity-refresh-page/);
  assert.match(identity,/page\.querySelector\('\[data-identity-reload\]'\)/);
});

test('organization menu opens above the persistent rail footer',()=>{
  assert.match(css,/\.oms-v1__rail-organization-menu\{[^}]*bottom:calc\(100% \+ 7px\)/);
  assert.match(css,/\.oms-v1__rail-organization-menu\[hidden\]\{display:none\}/);
  assert.match(css,/\.oms-v1__rail-organization-option\[aria-selected="true"\]/);
});

test('account organization controls size the enhanced select wrappers instead of overflowing the page',()=>{
  assert.match(css,/\.oms-v1__identity-switch>\.oms-v1__select\{[^}]*width:260px[^}]*min-width:0/);
  assert.match(css,/\.oms-v1__identity-switch button\{[^}]*flex:none[^}]*white-space:nowrap/);
  assert.match(css,/\.oms-v1__identity-toolbar>\.oms-v1__select\{[^}]*width:clamp\(128px,13vw,180px\)[^}]*min-width:128px/);
  assert.match(css,/@media\(max-width:1000px\)\{[\s\S]*?\.oms-v1__identity-toolbar>\.oms-v1__select\{flex:1 1 180px;width:auto;min-width:160px\}/);
});
