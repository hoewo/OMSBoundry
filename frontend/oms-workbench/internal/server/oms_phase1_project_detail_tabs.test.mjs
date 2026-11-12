import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const html = readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8');
const script = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');
const styles = readFileSync(new URL('./oms_phase1.css', import.meta.url), 'utf8');

test('project detail uses four intent-based tabs without removing record ledgers', () => {
  for (const key of ['overview', 'parties', 'contracts', 'cash']) {
    assert.match(html, new RegExp(`data-oms-v1-detail-tab="${key}"`));
    assert.match(html, new RegExp(`data-oms-v1-detail-pane="${key}"`));
  }
  for (const key of ['outgoing', 'incoming']) {
    assert.match(html, new RegExp(`data-oms-v1-project-invoice-list="${key}"`));
  }
  for (const key of ['receipt', 'payment']) {
    assert.match(html, new RegExp(`data-oms-v1-project-cash-list="${key}"`));
  }
  assert.match(script, /function setProjectDetailTab\(/);
  assert.match(script, /\['ArrowLeft','ArrowRight','Home','End'\]/);
  assert.match(styles, /\.oms-v1__detail-tabs\{[^}]*position:sticky/);
});

test('project detail change preserves the v1.52 product navigation', () => {
  for (const page of ['overview', 'projects', 'receivable', 'payable', 'internal', 'open-api', 'account', 'feedback']) {
    assert.match(html, new RegExp(`data-oms-v1-nav="${page}"`));
  }
  assert.match(html, /data-oms-v1-tab="members"/);
  assert.match(html, /data-oms-v1-tab="payroll"/);
  assert.match(html, /data-oms-v1-tab="finance"/);
});

test('project list keeps summary, sortable six-column layout and terminal action', () => {
  assert.match(html, /data-oms-v1-project-summary/);
  assert.match(html, /data-oms-v1-sort-list="projects" data-oms-v1-sort-key="name"/);
  assert.match(html, /data-oms-v1-sort-list="projects" data-oms-v1-sort-key="start_date"/);
  assert.match(html, /data-oms-v1-sort-list="projects" data-oms-v1-sort-key="receivable"/);
  assert.match(html, /data-oms-v1-sort-list="projects" data-oms-v1-sort-key="payable"/);
  assert.match(html, /data-oms-v1-project-body><tr><td colspan="6"/);
  assert.match(script, /projectMoneyStatusCell\(item\.id,'receivable'\)/);
  assert.match(script, /projectMoneyStatusCell\(item\.id,'payable'\)/);
  assert.match(script, /<button class="oms-v1__row-action" type="button">详情 ›<\/button>/);
});

test('project detail uses a wider bounded workbench canvas and sequential cash surfaces', () => {
  assert.match(styles, /BF-286\/BF-288/);
  assert.match(styles, /\.oms-v1__project-detail>main\{width:min\(1440px,calc\(100% - 64px\)\);max-width:1440px/);
  assert.match(styles, /data-oms-v1-detail-pane="parties"[^}]*width:min\(980px,100%\)/);
  assert.doesNotMatch(styles, /data-oms-v1-detail-pane="cash"\]\.is-active\{display:grid;grid-template-columns:repeat\(2,minmax\(0,1fr\)\)/);
  assert.match(html, /<details class="oms-v1__detail-meta"><summary>记录信息<\/summary>/);
});

test('project parties and contracts avoid numbered role rows and decorative receipt/payment badges', () => {
  assert.match(script, /class="oms-v1__party-group"/);
  assert.match(script, /合同已关联/);
  assert.doesNotMatch(script, /class="oms-v1__party-row[^']*<span>.*施工方 '\+String\(index\+1\)/);
  assert.doesNotMatch(script, /row\.tone==='upstream'\?'收':'付'/);
  assert.match(script, /items\.slice\(0,4\)/);
});

test('cash summary leads with the unresolved balance instead of four equal metrics', () => {
  assert.match(html, /data-oms-v1-flow-focus="receivable"/);
  assert.match(html, /data-oms-v1-flow-focus="payable"/);
  assert.match(html, /data-oms-v1-flow-progress-bar/);
  assert.match(html, /data-oms-v1-flow-invoice-state/);
  assert.doesNotMatch(html, /class="oms-v1__money-chain/);
  assert.match(script, /function renderFlowFocus\(/);
  assert.match(script, /已全额开票/);
  assert.match(script, /未回款占应收/);
  assert.match(styles, /BF-287 local candidate/);
  assert.doesNotMatch(script, /class="oms-v1__contractor-item" '\+\(index===0\?'open':'\'\)/);
  assert.match(script, /class="oms-v1__contractor-item"><summary>/);
});
