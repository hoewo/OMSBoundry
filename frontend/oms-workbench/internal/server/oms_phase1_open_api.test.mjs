import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const source=readFileSync(new URL('./oms_phase1_open_api.js',import.meta.url),'utf8');

assert.match(source,/const code=value\.code\|\|value\.error\?\.code;/,'nested IAM problem codes must remain visible to the user');
assert.match(source,/loaded=false;revealSecret\(value\);void load\(true\);/,'one-time secret must be revealed before the non-essential list refresh');
assert.doesNotMatch(source,/await load\(true\);revealSecret\(value\);/,'a failed list refresh must never consume the only secret display');
assert.match(source,/dismissOnly:true/,'the one-time secret dialog must not be dismissed by submitting another action');
assert.match(source,/value="180" selected/,'the default API Key lifetime must remain 180 days');
assert.match(source,/value="365"/,'the optional 365-day lifetime must remain available');

console.log('oms_phase1_open_api.test.mjs: 6 assertions passed');
