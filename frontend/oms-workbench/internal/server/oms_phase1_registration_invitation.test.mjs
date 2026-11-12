import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const source = fs.readFileSync(new URL('./oms_phase1_login.js', import.meta.url), 'utf8')
const css = fs.readFileSync(new URL('./reference_oa_login.css', import.meta.url), 'utf8')

test('registration invitation is previewed without putting its token in a query string', () => {
  assert.match(source, /tokenParameters\.get\('registration_invitation'\)/)
  assert.match(source, /\/api\/v1\/invitations\/preview/)
  assert.match(source, /body:JSON\.stringify\(\{token:registrationToken\}\)/)
  assert.doesNotMatch(source, /invitations\/preview\?token=/)
})

test('recipient chooses phone or email and owns the account fields', () => {
  assert.match(source, /<option value="phone">手机号<\/option>/)
  assert.match(source, /<option value="email">邮箱<\/option>/)
  assert.match(source, /name="display_name"/)
  assert.match(source, /name="login_identifier"/)
  assert.match(source, /identifier\.type = email \? 'email' : 'tel'/)
  assert.match(source, /name="password"/)
  assert.match(source, /\/api\/v1\/invitations\/register/)
  assert.match(source, /不会修改原密码/)
})

test('registration form remains usable on narrow screens', () => {
  assert.match(css, /\.login-page__registration-row/)
  assert.match(css, /grid-template-columns: 138px minmax\(0, 1fr\)/)
  assert.match(css, /@media \(max-width: 720px\)[\s\S]*\.login-page__registration-row[\s\S]*grid-template-columns: 1fr/)
})
