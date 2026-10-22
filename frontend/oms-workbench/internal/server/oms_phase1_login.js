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
            : response.status === 401 ? '用户名或密码不正确，请检查后重试。'
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
        errorNotice.textContent = error.message?.startsWith('安全令牌') || error.message?.startsWith('登录') || error.message?.startsWith('用户名') || error.message?.startsWith('身份服务')
          ? error.message : '登录结果待确认，请重试或重新打开工作台；不会自动重复提交。'
        errorNotice.hidden = false
        errorNotice.focus()
      }
    } finally {
      if (button) button.disabled = false
    }
  })

  function setMode(mode) {
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
