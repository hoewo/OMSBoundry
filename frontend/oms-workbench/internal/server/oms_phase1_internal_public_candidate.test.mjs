import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

const html=readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');
const script=readFileSync(new URL('./oms_phase1_internal.js',import.meta.url),'utf8');
const payrollOwner=readFileSync(new URL('./oms_phase1_payroll_owner.js',import.meta.url),'utf8');

test('internal management declares formal member and payroll owners with a local finance fallback',()=>{
  assert.match(html,/data-subject-id="\{\{\.SubjectID\}\}"/);
  assert.match(html,/data-internal-storage-notice/);
  assert.match(script,/root\.dataset\.tenantId[^\n]+root\.dataset\.subjectId/);
  assert.match(script,/成员与工资数据由正式业务服务保存；财务申请仍为当前账号的浏览器体验数据/);
  assert.match(script,/if\(!ownerPayroll\)\{/);
  assert.ok(payrollOwner.includes("ownerAPI?.app?.('people-groups')"));
  assert.ok(payrollOwner.includes("ownerAPI?.app?.('payroll')"));
});

test('public internal candidate does not silently fail closed after login',()=>{
  assert.doesNotMatch(script,/dataset\.demoPreview!=='true'/);
  assert.doesNotMatch(script,/尚未接入正式业务服务/);
});

test('batch tables expose explicit select all and clear controls',()=>{
  for(const marker of ['data-internal-payroll-select-all','data-internal-payroll-clear','data-internal-finance-select-all','data-internal-finance-clear'])assert.match(html,new RegExp(marker));
  assert.match(payrollOwner,/state\.selectable=items\.filter\(item=>selectable\(item\.line\)\)/);
  assert.match(script,/view\.selectableFinance=items\.filter\(item=>item\.status==='pending'\)/);
  assert.match(payrollOwner,/data-internal-payroll-select-all[^\n]+state\.selectable/);
  assert.match(script,/data-internal-finance-select-all[^\n]+view\.selectableFinance/);
});
