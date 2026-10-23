import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const html = readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8');
const script = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');
const internalScript = readFileSync(new URL('./oms_phase1_internal.js', import.meta.url), 'utf8');
const styles = readFileSync(new URL('./oms_phase1.css', import.meta.url), 'utf8');

test('all ledger money headers use the dedicated monetary column class', () => {
  const moneyHeaders = html.match(/<th class="is-number is-money">/g) || [];
  assert.equal(moneyHeaders.length, 18);
  assert.match(html, /<th class="is-number"><button[^>]+data-oms-v1-sort-key="receivable"/);
  assert.match(html, /<th class="is-number"><button[^>]+data-oms-v1-sort-key="payable"/);
});

test('all dynamic ledger money cells use the same monetary column class', () => {
  const moneyCells = script.match(/<td class="is-number is-money">/g) || [];
  const internalMoneyCells = internalScript.match(/<td class="is-number is-money">/g) || [];
  assert.equal(moneyCells.length, 5);
  assert.equal(internalMoneyCells.length, 8);
});

test('monetary columns share a fixed right edge and tabular figures', () => {
  assert.match(styles, /th\.is-money,\.oms-v1__table td\.is-money\{[^}]*min-width:132px[^}]*text-align:right[^}]*font-feature-settings:"tnum" 1,"lnum" 1/);
  assert.match(styles, /th\.is-money \.oms-v1__sort\{[^}]*width:100%[^}]*margin:0[^}]*justify-content:flex-end/);
  assert.match(styles, /td\.is-money>strong\{[^}]*width:100%[^}]*max-width:none[^}]*text-align:right/);
  assert.match(styles, /\.oms-v1__invoice-row>b\{[^}]*width:100%[^}]*font-variant-numeric:tabular-nums lining-nums/);
});
