(() => {
  'use strict'

  async function boundedRequest(url, options) {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 12000)
    try {
      return await fetch(url, {...options, signal: controller.signal})
    } finally {
      clearTimeout(timer)
    }
  }

  class GatewayCSRF {
    constructor({route, request = (...args) => fetch(...args), cookies = () => document.cookie}) {
      this.route = route
      this.request = request
      this.cookies = cookies
      this.pending = null
    }

    current() {
      const value = this.cookies().split(';').map(item => item.trim()).find(item => item.startsWith('oms_iam_csrf='))
      const token = value ? value.slice('oms_iam_csrf='.length) : ''
      return /^[A-Za-z0-9_-]{16,256}$/.test(token) ? token : ''
    }

    async token() {
      const current = this.current()
      if (current) return current
      if (!this.pending) this.pending = this.refresh().finally(() => { this.pending = null })
      return this.pending
    }

    async refresh() {
      if (!/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(this.route || '')) {
        throw new Error('安全令牌更新入口未配置；输入已保留，请刷新后重试')
      }
      try {
        const response = await this.request(this.route + '/api/v1/csrf', {
          method: 'GET', credentials: 'same-origin', cache: 'no-store', redirect: 'error',
          headers: {Accept: 'application/json'}
        })
        if (!response.ok || response.redirected) throw new Error('csrf unavailable')
        const data = await response.json()
        const current = this.current()
        if (!current || data.csrf_token !== current) throw new Error('csrf cookie mismatch')
        return current
      } catch {
        throw new Error('安全令牌暂时无法更新；输入已保留，请重试，不会自动提交业务')
      }
    }
  }

  class LoginFlow {
    constructor({csrf, request = boundedRequest, location = globalThis.location}) {
      this.csrf = csrf
      this.request = request
      this.location = location
      this.busy = false
    }

    async submit(values) {
      if (this.busy) return null
      this.busy = true
      try {
        const token = await this.csrf.token()
        const body = new URLSearchParams({...values, csrf_token: token})
        const response = await this.request('/gateway/login', {
          method: 'POST', credentials: 'same-origin', cache: 'no-store',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body
        })
        if (!response.ok) {
          const message = response.status === 403 ? '安全令牌已变化，请再次点击登录；输入已保留。'
            : response.status === 401 ? '登录账号或密码不正确，请检查后重试。'
            : response.status === 429 ? '登录请求过于频繁，请稍后重试。'
            : '身份服务暂时不可用，请稍后重试；输入已保留。'
          throw new Error(message)
        }
        const target = new URL(response.url)
        if (!response.redirected || target.origin !== this.location.origin || target.pathname === '/gateway/login') {
          throw new Error('登录结果未确认，请重新打开工作台；不会自动重复提交。')
        }
        this.location.assign(target.href)
        return target.href
      } finally {
        this.busy = false
      }
    }
  }

  globalThis.OMSPhase1GatewayCSRF = GatewayCSRF
  globalThis.OMSPhase1LoginFlow = LoginFlow

  const page = document.querySelector('[data-oms-login]')
  if (!page) return

  const smsPanel = page.querySelector('[data-login-panel="sms"]')
  const iamPanel = page.querySelector('[data-login-panel="iam"]')
  const phone = page.querySelector('[data-sms-phone]')
  const send = page.querySelector('[data-sms-send]')
  const notice = page.querySelector('[data-sms-notice]')
  const modeButtons = [...page.querySelectorAll('[data-login-mode]')]
  const phonePattern = /^1[3-9]\d{9}$/
  const form = iamPanel.querySelector('form[action="/gateway/login"]')
  const errorNotice = iamPanel.querySelector('[data-login-error]')
  const login = new LoginFlow({
    csrf: new GatewayCSRF({route: page.dataset.iamRoutePrefix, request: boundedRequest})
  })

  const tokenParameters = new URLSearchParams(location.hash.replace(/^#/, ''))
  const registrationToken = tokenParameters.get('registration_invitation')
  const activationToken = tokenParameters.get('activation_token')
  const resetToken = tokenParameters.get('password_reset_token')
  if (registrationToken) {
    const panel = document.createElement('div')
    panel.dataset.loginPanel = 'credential'
    panel.innerHTML = '<div class="login-page__card-header"><span class="login-page__card-kicker">组织邀请</span><h2>接受邀请并注册</h2><p class="login-page__card-copy">正在核对邀请信息，请稍候。</p></div><div class="login-page__environment-note" role="status" aria-live="polite" tabindex="-1" data-registration-status>正在读取邀请…</div>'
    page.querySelector('.login-page__card')?.append(panel)
    smsPanel.hidden = true
    iamPanel.hidden = true
    const status = panel.querySelector('[data-registration-status]')
    ;(async () => {
      try {
        const csrfToken = await login.csrf.token()
        const previewResponse = await boundedRequest(page.dataset.iamRoutePrefix + '/api/v1/invitations/preview', {method:'POST', credentials:'same-origin', cache:'no-store', headers:{'Content-Type':'application/json','X-CSRF-Token':csrfToken}, body:JSON.stringify({token:registrationToken})})
        const previewBody = await previewResponse.json().catch(() => ({}))
        if (!previewResponse.ok) throw new Error(previewBody?.error?.message || '邀请链接无效、已使用或已经过期。')
        const invitation = previewBody.invitation || {}, expires = invitation.expires_at ? new Intl.DateTimeFormat('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}).format(new Date(invitation.expires_at)) : '—'
        panel.innerHTML = '<div class="login-page__card-header"><span class="login-page__card-kicker">组织邀请</span><h2>加入 ' + escapeHTML(invitation.tenant_name || '当前组织') + '</h2><p class="login-page__card-copy">受邀身份：' + (invitation.administrator ? '管理员' : '普通用户') + ' · 有效期至 ' + escapeHTML(expires) + '</p></div><form class="login-page__form ant-form ant-form-vertical" data-registration-form><div class="ant-form-item"><div class="ant-form-item-label"><label>姓名</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="display_name" maxlength="100" autocomplete="name" placeholder="请输入你的姓名" required></div></div><div class="login-page__registration-row"><div class="ant-form-item"><div class="ant-form-item-label"><label>登录方式</label></div><div class="ant-form-item-control"><select class="ant-input ant-input-lg" name="identifier_kind"><option value="phone">手机号</option><option value="email">邮箱</option></select></div></div><div class="ant-form-item"><div class="ant-form-item-label"><label data-registration-identifier-label>手机号</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="login_identifier" type="tel" inputmode="tel" autocomplete="tel" placeholder="请输入手机号" required></div></div></div><div class="ant-form-item"><div class="ant-form-item-label"><label>密码</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="password" type="password" minlength="12" maxlength="72" autocomplete="new-password" required></div></div><div class="ant-form-item"><div class="ant-form-item-label"><label>确认密码</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="confirmation" type="password" minlength="12" maxlength="72" autocomplete="new-password" required></div></div><p class="login-page__registration-help">新账号将使用此密码完成注册；若手机号或邮箱已经注册，请填写原密码，系统只加入当前组织，不会修改原密码。</p><div class="ant-form-item"><button class="ant-btn ant-btn-primary ant-btn-lg ant-btn-block" type="submit"><span>完成注册并加入组织</span></button></div></form><div class="login-page__environment-note" role="status" aria-live="polite" tabindex="-1" data-registration-status hidden></div>'
        const registrationForm = panel.querySelector('[data-registration-form]'), kind = registrationForm.querySelector('[name="identifier_kind"]'), identifier = registrationForm.querySelector('[name="login_identifier"]'), label = registrationForm.querySelector('[data-registration-identifier-label]')
        kind.addEventListener('change', () => { const email = kind.value === 'email'; label.textContent = email ? '邮箱' : '手机号'; identifier.type = email ? 'email' : 'tel'; identifier.placeholder = email ? 'name@example.com' : '请输入手机号'; identifier.inputMode = email ? 'email' : 'tel'; identifier.autocomplete = email ? 'email' : 'tel' })
        registrationForm.addEventListener('submit', async event => {
          event.preventDefault()
          if (!registrationForm.reportValidity()) return
          const values = new FormData(registrationForm), password = String(values.get('password') || ''), confirmation = String(values.get('confirmation') || ''), registrationStatus = panel.querySelector('[data-registration-status]'), button = registrationForm.querySelector('button[type="submit"]'), method = String(values.get('identifier_kind') || 'phone'), raw = String(values.get('login_identifier') || '').trim(), loginIdentifier = method === 'phone' ? raw.replace(/[\s()-]/g, '').replace(/^\+86/, '').replace(/^0086/, '') : raw.toLowerCase()
          registrationStatus.hidden = true
          if (method === 'phone' && !/^1[3-9]\d{9}$/.test(loginIdentifier)) { registrationStatus.textContent = '请输入有效的中国大陆手机号。'; registrationStatus.hidden = false; registrationStatus.focus(); return }
          if (method === 'email' && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(loginIdentifier)) { registrationStatus.textContent = '请输入有效的邮箱地址。'; registrationStatus.hidden = false; registrationStatus.focus(); return }
          if (password !== confirmation) { registrationStatus.textContent = '两次输入的密码不一致。'; registrationStatus.hidden = false; registrationStatus.focus(); return }
          button.disabled = true
          try {
            const currentCSRF = await login.csrf.token()
            const response = await boundedRequest(page.dataset.iamRoutePrefix + '/api/v1/invitations/register', {method:'POST', credentials:'same-origin', cache:'no-store', headers:{'Content-Type':'application/json','X-CSRF-Token':currentCSRF}, body:JSON.stringify({token:registrationToken,display_name:String(values.get('display_name')||'').trim(),login_identifier:loginIdentifier,password})})
            const body = await response.json().catch(() => ({}))
            if (!response.ok) throw new Error(body?.error?.message || '注册未完成，请检查信息后重试。')
            history.replaceState(null, '', location.pathname + location.search)
            registrationForm.hidden = true
            registrationStatus.textContent = body.action === 'joined' ? '原账号已验证并加入组织，现在可以登录。' : '注册完成并已加入组织，现在可以登录。'
            registrationStatus.hidden = false
            const loginButton = document.createElement('button')
            loginButton.type = 'button'; loginButton.className = 'ant-btn ant-btn-primary ant-btn-lg ant-btn-block'; loginButton.textContent = '返回登录'
            loginButton.addEventListener('click', () => { panel.hidden = true; iamPanel.hidden = false; const username = iamPanel.querySelector('input[name="username"]'); if (username) username.value = loginIdentifier; username?.focus() })
            panel.append(loginButton)
          } catch (error) {
            registrationStatus.textContent = error instanceof Error ? error.message : '注册未完成，请重试。'; registrationStatus.hidden = false; registrationStatus.focus()
          } finally { button.disabled = false }
        })
        registrationForm.querySelector('[name="display_name"]')?.focus()
      } catch (error) {
        status.textContent = error instanceof Error ? error.message : '邀请链接暂时无法核对。'
        status.focus()
      }
    })()
  } else if (activationToken || resetToken) {
    const panel = document.createElement('div')
    panel.dataset.loginPanel = 'credential'
    panel.innerHTML = '<div class="login-page__card-header"><span class="login-page__card-kicker">IAM 账号安全</span><h2>' + (activationToken ? '激活组织账号' : '设置新密码') + '</h2><p class="login-page__card-copy">密码长度为 12 至 72 个字符。一次性凭证成功使用后立即失效。</p></div><form class="login-page__form ant-form ant-form-vertical"><div class="ant-form-item"><div class="ant-form-item-label"><label>新密码</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="password" type="password" minlength="12" maxlength="72" autocomplete="new-password" required></div></div><div class="ant-form-item"><div class="ant-form-item-label"><label>确认新密码</label></div><div class="ant-form-item-control"><input class="ant-input ant-input-lg" name="confirmation" type="password" minlength="12" maxlength="72" autocomplete="new-password" required></div></div><div class="ant-form-item"><button class="ant-btn ant-btn-primary ant-btn-lg ant-btn-block" type="submit"><span>' + (activationToken ? '激活账号' : '确认新密码') + '</span></button></div></form><div class="login-page__environment-note" role="status" aria-live="polite" tabindex="-1" data-credential-status hidden></div>'
    page.querySelector('.login-page__card')?.append(panel)
    smsPanel.hidden = true
    iamPanel.hidden = true
    const tokenForm = panel.querySelector('form')
    tokenForm.addEventListener('submit', async event => {
      event.preventDefault()
      if (!tokenForm.reportValidity()) return
      const values = new FormData(tokenForm), password = String(values.get('password') || ''), confirmation = String(values.get('confirmation') || ''), status = panel.querySelector('[data-credential-status]'), button = tokenForm.querySelector('button[type="submit"]')
      status.hidden = true
      if (password !== confirmation) { status.textContent = '两次输入的密码不一致。'; status.hidden = false; status.focus(); return }
      button.disabled = true
      try {
        const csrfToken = await login.csrf.token()
        const response = await boundedRequest(page.dataset.iamRoutePrefix + (activationToken ? '/api/v1/activate' : '/api/v1/password-reset/complete'), {method:'POST', credentials:'same-origin', cache:'no-store', headers:{'Content-Type':'application/json','X-CSRF-Token':csrfToken}, body:JSON.stringify({token:activationToken||resetToken,password})})
        const body = await response.json().catch(() => ({}))
        if (!response.ok) throw new Error(body.detail || '一次性凭证无效或已经过期。')
        history.replaceState(null, '', location.pathname + location.search)
        status.textContent = activationToken ? '账号已激活，请使用新密码登录。' : '密码已更新，请使用新密码登录。'
        status.hidden = false
        tokenForm.hidden = true
        const loginButton = document.createElement('button')
        loginButton.type = 'button'; loginButton.className = 'ant-btn ant-btn-primary ant-btn-lg ant-btn-block'; loginButton.textContent = '返回登录'
        loginButton.addEventListener('click', () => { panel.hidden = true; iamPanel.hidden = false; iamPanel.querySelector('input[name="username"]')?.focus() })
        panel.append(loginButton)
      } catch (error) {
        status.textContent = error instanceof Error ? error.message : '账号操作未完成，请重试。'; status.hidden = false; status.focus()
      } finally { button.disabled = false }
    })
    panel.querySelector('input[name="password"]')?.focus()
  }

  function escapeHTML(value) {
    return String(value ?? '').replace(/[&<>'"]/g, character => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[character]))
  }

  form?.addEventListener('submit', async event => {
    event.preventDefault()
    if (login.busy || !form.reportValidity()) return
    const button = form.querySelector('button[type="submit"]')
    if (button) button.disabled = true
    if (errorNotice) errorNotice.hidden = true
    try {
      await login.submit(Object.fromEntries(new FormData(form)))
    } catch (error) {
      if (errorNotice) {
        errorNotice.textContent = error.message?.startsWith('安全令牌') || error.message?.startsWith('登录') || error.message?.startsWith('身份服务')
          ? error.message : '登录结果待确认，请重试或重新打开工作台；不会自动重复提交。'
        errorNotice.hidden = false
        errorNotice.focus()
      }
    } finally {
      if (button) button.disabled = false
    }
  })

  function setMode(mode) {
    const credentialPanel = page.querySelector('[data-login-panel="credential"]')
    if (credentialPanel && !credentialPanel.hidden) return
    const iam = mode === 'iam'
    smsPanel.hidden = iam
    iamPanel.hidden = !iam
    for (const button of modeButtons) button.setAttribute('aria-pressed', String(button.dataset.loginMode === mode))
    const target = iam ? iamPanel.querySelector('input[name="username"]') : phone
    target?.focus()
  }

  function updatePhoneState() {
    const valid = phonePattern.test(phone.value.trim())
    send.disabled = !valid
    phone.setAttribute('aria-invalid', String(phone.value.length > 0 && !valid))
    if (notice) notice.hidden = true
  }

  phone?.addEventListener('input', updatePhoneState)
  send?.addEventListener('click', () => {
    if (send.disabled || !notice) return
    notice.hidden = false
    notice.focus()
  })
  for (const button of modeButtons) button.addEventListener('click', () => setMode(button.dataset.loginMode))
  updatePhoneState()
})()
