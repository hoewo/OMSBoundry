import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const owner=readFileSync(new URL('./oms_phase1_finance_owner.js',import.meta.url),'utf8');
const local=readFileSync(new URL('./oms_phase1_internal.js',import.meta.url),'utf8');
const server=readFileSync(new URL('./server.go',import.meta.url),'utf8');

test('formal finance owner replaces browser-local finance when assembled',()=>{
  assert.match(local,/const ownerFinance=Boolean\(ownerAPI\?\.app\?\.\('internal-finance-requests'\)\)/);
  assert.match(local,/if\(!ownerFinance\)renderFinance\(\)/);
  assert.match(server,/oms-phase1-finance-owner\.js\?v=20261025\.1/);
});

test('finance owner covers revisioned requests, payment, independent exports and categories',()=>{
  for(const marker of [
    '/api/v1/finance-requests',
    '/api/v1/finance-payments/batch',
    '/sensitive-account?purpose=',
    '/api/v1/finance-request-exports',
    '/api/v1/finance-request-sensitive-exports',
    '/api/v1/expense-categories',
    'missing_evidence_acknowledged',
    'expected_revision',
    'payment_unknown_count',
    'operator_subject_id',
    'department_id',
    'contract_id',
    'amount_',
    'date_',
  ])assert.ok(owner.includes(marker),`missing ${marker}`);
});
