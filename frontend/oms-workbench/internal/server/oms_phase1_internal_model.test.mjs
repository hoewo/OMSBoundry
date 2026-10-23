import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source=readFileSync(new URL('./oms_phase1_internal_model.js',import.meta.url),'utf8');
vm.runInThisContext(source,{filename:'oms_phase1_internal_model.js'});
const model=globalThis.OMSInternalModel;

test('amounts use exact minor-unit parsing and display',()=>{
  assert.equal(model.decimalMinor('1234567890123456.78'),'123456789012345678');
  assert.equal(model.money('123456789012345678'),'¥1,234,567,890,123,456.78');
  assert.equal(model.decimalMinor('1.234'),null);
});

test('contributions round in integer minor units',()=>{
  assert.equal(model.contribution('100','50'),'1');
  assert.equal(model.contribution('1000000','800'),'80000');
});

test('payroll calculation separates employee deduction and employer cost',()=>{
  const member={fixed_salary_minor:'1000000',social_base_minor:'1000000',housing_base_minor:'1000000'};
  const scheme={rates:{
    pension:{employee_bps:800,employer_bps:1600},medical:{employee_bps:200,employer_bps:980},
    unemployment:{employee_bps:50,employer_bps:50},injury:{employee_bps:0,employer_bps:20},
    maternity:{employee_bps:0,employer_bps:80},housing:{employee_bps:1200,employer_bps:1200},
  }};
  const line=model.payrollLine(member,'200000',scheme);
  assert.equal(line.gross_minor,'1200000');
  assert.equal(line.employee_contribution_minor,'225000');
  assert.equal(line.payable_minor,'975000');
  assert.equal(line.employer_contribution_minor,'393000');
  assert.equal(line.employer_cost_minor,'1593000');
});

test('planned date never creates a paid fact without confirmation',()=>{
  assert.equal(model.lineStatus({paid_at:''},'2026-10-20','2026-10-23'),'confirm');
  assert.equal(model.lineStatus({paid_at:''},'2026-10-30','2026-10-23'),'planned');
  assert.equal(model.lineStatus({paid_at:'2026-10-23T01:00:00Z'},'2026-10-20','2026-10-23'),'paid');
});

test('member sync only proposes new active members before any payment',()=>{
  const members=[
    {id:'member-a',name:'已在工资表',status:'active'},
    {id:'member-b',name:'新成员',status:'active'},
    {id:'member-c',name:'停用成员',status:'inactive'},
  ];
  assert.deepEqual(model.payrollSyncCandidates(members,[{member_id:'member-a',paid_at:''}]).map(item=>item.id),['member-b']);
  assert.equal(model.canSyncPeriod([{member_id:'member-a',paid_at:''}]),true);
  assert.equal(model.canSyncPeriod([{member_id:'member-a',paid_at:'2026-10-23T01:00:00Z'}]),false);
});
