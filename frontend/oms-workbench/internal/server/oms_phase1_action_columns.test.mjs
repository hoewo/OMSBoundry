import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const html = readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8');
const script = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');
const internalScript = readFileSync(new URL('./oms_phase1_internal.js', import.meta.url), 'utf8');
const styles = readFileSync(new URL('./oms_phase1.css', import.meta.url), 'utf8');

test('every operational table reserves its final column for row details', () => {
  const actionHeaders = html.match(/<th><\/th>/g) || [];
  const actionCells = script.match(/<td><button class="oms-v1__row-action"/g) || [];
  const internalActionCells = internalScript.match(/class="oms-v1__row-actions"/g) || [];
  assert.equal(actionHeaders.length, 15);
  assert.equal(actionCells.length, 5);
  assert.equal(internalActionCells.length, 3);
  assert.match(html, /<table class="oms-v1__table"><thead><tr><th>名称<\/th>[^]*<th><\/th><\/tr><\/thead><tbody data-open-api-key-body>/, 'API Key rows keep a terminal revoke-action column');
  assert.match(html, /<table class="oms-v1__table oms-v1__table--plain">[^]*<tbody data-open-api-usage-body>/, 'read-only usage history is explicitly marked as a table without row actions');
});

test('detail actions use a fixed right-aligned terminal column', () => {
  assert.match(styles, /thead th:last-child\{[^}]*width:76px[^}]*min-width:76px[^}]*text-align:right/);
  assert.match(styles, /tr\[data-oms-v1-row\]>td:last-child\{[^}]*width:76px[^}]*min-width:76px[^}]*text-align:right[^}]*white-space:nowrap/);
  assert.match(styles, /\.oms-v1__row-action\{[^}]*display:inline-flex[^}]*justify-content:flex-end[^}]*text-align:right[^}]*white-space:nowrap/);
  assert.match(styles, /\.oms-v1__table--plain thead th:last-child\{[^}]*width:auto[^}]*text-align:left/, 'plain read-only tables must not inherit the terminal action-column geometry');
});
