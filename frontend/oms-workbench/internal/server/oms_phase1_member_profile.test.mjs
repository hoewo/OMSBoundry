import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const shell = fs.readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8')
const internal = fs.readFileSync(new URL('./oms_phase1_internal.js', import.meta.url), 'utf8')
const html = fs.readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8')

test('capability authorization accepts the gateway redirect without following an application root', () => {
  assert.match(shell, /fetch\('\/gateway\/authorize\/'\+encodeURIComponent\(id\),\{method:'POST',credentials:'same-origin',body,redirect:'manual'\}\)/)
  assert.match(shell, /if\(!credentialRedirect\(response\)&&!response\.ok\)await problem\(response\)/)
  assert.doesNotMatch(shell, /gateway\/authorize[^\n]+redirect:'follow'/)
})

test('wage account remains optional and partial failures identify the exact profile', () => {
  assert.match(internal, /if\(compensationChanged\)\{pendingStage='薪酬档案';await ownerAPI\.api\('payroll','\/api\/v1\/compensation-profiles\/'/)
  assert.match(internal, /if\(accountNumber\)\{pendingStage='工资账户档案';/)
  assert.match(internal, /成员基础资料已保存，但'\+pendingStage\+'未完成/)
})

test('member operator uses the latest People audit actor instead of a service placeholder', () => {
  assert.match(internal, /person\.last_actor_display_name/)
  assert.match(internal, /actorID===root\.dataset\.subjectId\?root\.dataset\.displayName/)
  assert.doesNotMatch(internal, /operator:'Owner 服务'/)
})

test('member form owns the explicit IAM Subject to Person link', () => {
  assert.match(internal, /identityRoute\+'\/api\/v1\/admin\/subjects'/)
  assert.match(internal, /field\('登录账号'/)
  assert.match(internal, /iam_subject_id:iamSubjectID/)
  assert.match(internal, /\/iam-subject'/)
  assert.match(internal, /pendingStage='登录账号关联'/)
  assert.match(internal, /item\.status==='active'\|\|item\.status==='pending'/)
  assert.match(internal, /系统不会按姓名自动匹配/)
  assert.match(html, /data-internal-link-current-account/)
  assert.match(html, /<th>登录账号<\/th>/)
})

test('missing employee identity has a stable explicit recovery entry', () => {
  assert.match(shell, /window\.location\.hash!=='#member-account-link'/)
  assert.match(shell, /oms-v1:open-current-account-link/)
  assert.match(internal, /function openCurrentAccountLink\(\)/)
  assert.match(internal, /administrator/)
  assert.match(internal, /不会按姓名自动匹配/)
  assert.match(internal, /\/iam-subject/)
})

test('member linking refreshes the current IAM session before rejecting an administrator', () => {
  assert.match(internal, /function refreshAccountManagementCapability\(\)/)
  assert.match(internal, /identityRoute\+'\/api\/v1\/session'/)
  assert.match(internal, /principal\.administrator===true/)
  assert.match(internal, /if\(!canManageAccounts\)await refreshAccountManagementCapability\(\)/)
})
