import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const html = readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8');
const script = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');
const styles = readFileSync(new URL('./oms_phase1.css', import.meta.url), 'utf8');

test('every operational table reserves its final column for row details', () => {
  const actionHeaders = html.match(/<th><\/th>/g) || [];
  const actionCells = script.match(/<td><button class="oms-v1__row-action"/g) || [];
  assert.equal(actionHeaders.length, 9);
  assert.equal(actionCells.length, 5);
});

test('detail actions use a fixed right-aligned terminal column', () => {
  assert.match(styles, /thead th:last-child\{[^}]*width:76px[^}]*min-width:76px[^}]*text-align:right/);
  assert.match(styles, /tr\[data-oms-v1-row\]>td:last-child\{[^}]*width:76px[^}]*min-width:76px[^}]*text-align:right[^}]*white-space:nowrap/);
  assert.match(styles, /\.oms-v1__row-action\{[^}]*display:inline-flex[^}]*justify-content:flex-end[^}]*text-align:right[^}]*white-space:nowrap/);
});
