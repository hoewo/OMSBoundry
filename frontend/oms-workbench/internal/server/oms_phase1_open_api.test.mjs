import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const source=readFileSync(new URL('./oms_phase1_open_api.js',import.meta.url),'utf8');
const html=readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');

assert.match(source,/const code=value\.code\|\|value\.error\?\.code;/,'nested IAM problem codes must remain visible to the user');
assert.match(source,/loaded=false;revealSecret\(value\);void load\(true\);/,'one-time secret must be revealed before the non-essential list refresh');
assert.doesNotMatch(source,/await load\(true\);revealSecret\(value\);/,'a failed list refresh must never consume the only secret display');
assert.match(source,/dismissOnly:true/,'the one-time secret dialog must not be dismissed by submitting another action');
assert.match(source,/value="180" selected/,'the default API Key lifetime must remain 180 days');
assert.match(source,/value="365"/,'the optional 365-day lifetime must remain available');
assert.match(source,/person_mapping_required:'当前账号尚未关联成员档案，请在“成员管理”新增或编辑成员并关联登录账号'/,'a missing mapping must identify the repair path');
assert.match(source,/error\.recovery=value\.recovery\|\|value\.error\?\.recovery\|\|null/,'machine-readable recovery data must survive the browser error wrapper');
assert.match(source,/memberLink\?\.addEventListener\('click'.+member-account-link/,'the Open API page must expose the stable member-link recovery entry');
assert.match(source,/创建 Key 不要求关联成员；报销等确实依赖员工身份的接口会在调用时单独校验/,'key creation must not depend on employee mapping');
assert.doesNotMatch(source,/成员引用 r/,'the key list must not present a legacy Person snapshot as its identity');
assert.match(html,/为当前组织账号创建和管理独立 API Key/,'the Open API page must describe account-scoped keys');
assert.match(html,/data-open-api-member-link/,'the Open API page must render a member-link recovery action');
assert.doesNotMatch(html,/本人 API Key|本人 API 使用记录/,'the Open API page must not present generic keys as employee-bound');

console.log('oms_phase1_open_api.test.mjs: 14 assertions passed');
