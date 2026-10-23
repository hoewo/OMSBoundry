import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

const html=readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');
const script=readFileSync(new URL('./oms_phase1_internal.js',import.meta.url),'utf8');

test('public internal candidate declares browser-only persistence',()=>{
  assert.match(html,/data-subject-id="\{\{\.SubjectID\}\}"/);
  assert.match(html,/体验版数据仅保存在当前登录账号的本浏览器中/);
  assert.match(script,/root\.dataset\.tenantId[^\n]+root\.dataset\.subjectId/);
});

test('public internal candidate does not silently fail closed after login',()=>{
  assert.doesNotMatch(script,/dataset\.demoPreview!=='true'/);
  assert.doesNotMatch(script,/尚未接入正式业务服务/);
});
