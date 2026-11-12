import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(new URL('./oms_phase1_identity.js', import.meta.url), 'utf8')
const html = fs.readFileSync(new URL('./oms_phase1.html', import.meta.url), 'utf8')
const css = fs.readFileSync(new URL('./oms_phase1.css', import.meta.url), 'utf8')

test('account page is a compact directory with association-aware filters', () => {
  assert.match(html, /oms-v1__identity-context/)
  assert.match(html, /data-identity-search/)
  assert.match(html, /data-identity-link-filter/)
  assert.match(html, /用户 \/ 登录账号/)
  assert.match(html, /成员关联/)
  assert.doesNotMatch(html, /oms-v1__summary oms-v1__identity-summary/)
  assert.match(source, /personBySubject/)
  assert.match(source, /当前账号/)
  assert.match(source, /没有匹配的账号/)
  assert.match(css, /accounts are a working directory, not a dashboard/)
  assert.match(css, /\.oms-v1__identity-actions/)
})

test('administrator creates a scoped link without entering another persons credentials', () => {
  const inviteSource=source.slice(source.indexOf('function invite()'),source.indexOf('function editTenant()'))
  assert.match(inviteSource, /await post\('\/api\/v1\/admin\/invitations'/)
  assert.match(inviteSource, /title:'创建邀请链接'/)
  assert.match(inviteSource, /加入后的用户级别/)
  assert.match(inviteSource, /受邀人打开后可自行选择手机号或邮箱/)
  assert.doesNotMatch(inviteSource, /name="login_identifier"/)
  assert.doesNotMatch(inviteSource, /name="display_name" required/)
})

test('pending registration stays distinct from a member association', () => {
  assert.match(source, /registrationInvite=item=>/)
  assert.match(source, /手机号 \/ 邮箱由本人填写/)
  assert.match(source, /注册后关联/)
  assert.match(source, /成员关系不随邀请预设/)
})

test('registration, legacy activation and password reset use separate link fragments', () => {
  assert.match(source, /registration_invitation/)
  assert.match(source, /activation_token/)
  assert.match(source, /password_reset_token/)
  assert.match(source, /association=\{status:'not_applicable'\}/)
  assert.match(source, /const associationBlock=!activation\|\|association\.status==='not_applicable'\?'':/)
})
