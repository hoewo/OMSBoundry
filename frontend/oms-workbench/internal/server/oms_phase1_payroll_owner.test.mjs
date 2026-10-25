import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

const html=readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');
const shell=readFileSync(new URL('./oms_phase1.js',import.meta.url),'utf8');
const internal=readFileSync(new URL('./oms_phase1_internal.js',import.meta.url),'utf8');
const payroll=readFileSync(new URL('./oms_phase1_payroll_owner.js',import.meta.url),'utf8');
const server=readFileSync(new URL('./server.go',import.meta.url),'utf8');

test('formal payroll execution replaces browser persistence only when both owners are assembled',()=>{
  assert.match(internal,/const ownerPayroll=Boolean\(ownerAPI\?\.app\?\.\('people-groups'\)&&ownerAPI\?\.app\?\.\('payroll'\)\)/);
  assert.match(internal,/ownerPayroll\?\{payroll_periods:\[\]\}/);
  assert.match(internal,/renderMembers\(\);if\(!ownerPayroll\)renderPayroll\(\);if\(!ownerFinance\)renderFinance\(\)/);
  assert.ok(payroll.includes("if(!root||!model||!ownerAPI?.app?.('people-groups')||!ownerAPI?.app?.('payroll'))return"));
  assert.match(server,/oms-phase1-payroll-owner\.js\?v=20261025\.1/);
});

test('payroll owner journey uses versioned owner APIs and revision guards',()=>{
  for(const marker of [
    "api('/api/v1/payroll-periods')",
    "api('/api/v1/contribution-schemes')",
    "post('/api/v1/payroll-periods'",
    "'/members:sync'",
    "'/imports:preview'",
    "'/imports:commit'",
    "'/review'",
    "'/confirm-no-payment'",
    "'/payments'",
    "'/payments:batch'",
    "kind+'-sync-preview'",
    "'compensation:sync'",
    "kind==='compensation'",
    "'payment-profile:sync'",
    "post('/api/v1/payroll-adjustment-runs'",
    "'/void'",
    'expected_run_revision',
    'expected_line_revision',
    'expected_revision',
  ])assert.ok(payroll.includes(marker),`missing ${marker}`);
});

test('payroll UI keeps preview, partial failure and overage recovery visible',()=>{
  assert.match(payroll,/preview\.confirmable/);
  assert.match(payroll,/成功项不会因其他成员失败而回滚/);
  assert.match(payroll,/结果未知项也不会自动重试/);
  assert.match(payroll,/overage_confirmed/);
  assert.match(payroll,/金额超过未发部分时仍确认/);
  assert.match(payroll,/新建补发／扣回/);
  assert.match(payroll,/工资表生成后补入成员/);
  assert.match(html,/value="recovery_pending">待扣回/);
});

test('gateway API path validator admits versioned command suffixes used by payroll',()=>{
  assert.ok(shell.includes('[A-Za-z0-9_/?=&%.:-]+$'));
});
