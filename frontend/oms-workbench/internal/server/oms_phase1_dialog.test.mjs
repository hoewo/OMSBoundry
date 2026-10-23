import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const script = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');

test('dialog backdrop never dismisses an in-progress form', () => {
  assert.match(
    script,
    /querySelectorAll\('\[data-oms-v1-dialog-close\]'\)\)close\.addEventListener\('click',closeDialog\)/,
  );
  assert.doesNotMatch(script, /event\.target===dialog\)closeDialog\(\)/);
  assert.doesNotMatch(script, /dialog\.addEventListener\('click'/);
});
