import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

const identity=fs.readFileSync(new URL('./oms_phase1_identity.js',import.meta.url),'utf8');

test('account directory keeps IAM account and People member edits explicit',()=>{
  assert.match(identity,/data-identity-edit-account/);
  assert.match(identity,/data-identity-edit-member/);
  assert.match(identity,/编辑账号姓名/);
  assert.match(identity,/编辑成员资料/);
  assert.match(identity,/\/api\/v1\/admin\/subjects\/.*\/profile/);
  assert.match(identity,/expected_revision:Number\(item\.revision\)/);
  assert.match(identity,/ownerAPI\.api\('people-groups','\/api\/v1\/persons\/'\+encodeURIComponent\(person\.id\)\+'\/profile'/);
  assert.match(identity,/expected_revision:Number\(person\.revision\)/);
  assert.match(identity,/账号姓名属于 IAM/);
  assert.match(identity,/这里修改 People 员工档案/);
});

test('pending registration placeholders cannot be renamed as accounts',()=>{
  assert.match(identity,/if\(item\.status!=='pending'\)buttons\.push\('<button type="button" data-identity-edit-account/);
});
