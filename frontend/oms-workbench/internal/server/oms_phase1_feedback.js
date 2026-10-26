(function () {
  'use strict';

  var root = document.querySelector('[data-oms-v1]');
  var page = document.querySelector('[data-feedback-page]');
  if (!root || !page) return;

  var notice = page.querySelector('[data-feedback-notice] p');
  var badges = Array.prototype.slice.call(document.querySelectorAll('[data-feedback-unread]'));
  var sdkURL = String(root.dataset.feedbackSdkUrl || '').trim();
  var sdkOrigin = '';
  try {
    var parsedSDKURL = new URL(sdkURL);
    var loopback = parsedSDKURL.hostname === 'localhost' || parsedSDKURL.hostname === '127.0.0.1' || parsedSDKURL.hostname === '[::1]';
    if (parsedSDKURL.protocol !== 'https:' && !(parsedSDKURL.protocol === 'http:' && loopback)) throw new Error('invalid protocol');
    sdkOrigin = parsedSDKURL.origin;
  } catch (_) {
    sdkURL = '';
  }

  var layer = null;
  var frame = null;
  var ready = false;
  var readyWaiters = [];
  var runtimeConfig = null;
  var configRequest = null;
  var identityKey = currentIdentityKey();
  var unreadRequestID = '';
  var toastTimer = 0;

  function currentIdentityKey() {
    return String(root.dataset.tenantId || '') + '\u0000' + String(root.dataset.subjectId || '');
  }

  function randomID() {
    if (window.crypto && typeof window.crypto.randomUUID === 'function') return window.crypto.randomUUID();
    var bytes = new Uint8Array(16);
    window.crypto.getRandomValues(bytes);
    return Array.prototype.map.call(bytes, function (value) { return value.toString(16).padStart(2, '0'); }).join('');
  }

  function csrfToken() {
    var match = document.cookie.match(/(?:^|;\s*)oms_iam_csrf=([A-Za-z0-9_-]{16,256})(?:;|$)/);
    return match ? match[1] : '';
  }

  function isLoopbackURL(value) {
    try {
      var parsed = new URL(value);
      var loopback = parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1' || parsed.hostname === '[::1]';
      return parsed.protocol === 'https:' || (parsed.protocol === 'http:' && loopback);
    } catch (_) {
      return false;
    }
  }

  function validateConfig(value) {
    if (!value || typeof value !== 'object') throw new Error('invalid feedback config');
    var mode = value.feedbackV2AuthMode;
    if (value.feedbackV2Enabled !== true || (mode !== 'apiKey' && mode !== 'session')) throw new Error('invalid feedback mode');
    if (!Number.isInteger(value.projectId) || value.projectId < 1) throw new Error('invalid feedback project');
    if (!/^oms1_v[1-9][0-9]{0,3}_[A-Za-z0-9_-]{43}$/.test(String(value.customUserId || ''))) throw new Error('invalid feedback identity');
    if (!isLoopbackURL(value.gatewayUrl)) throw new Error('invalid feedback gateway');
    if (mode === 'apiKey' && !/^ak_[A-Za-z0-9_-]{16,}$/.test(String(value.apiKey || ''))) throw new Error('invalid feedback key');
    if (mode === 'session' && String(value.feedbackSessionToken || '').length < 16) throw new Error('invalid feedback session');
    return {
      feedbackV2Enabled: true,
      feedbackV2NotificationsEnabled: value.feedbackV2NotificationsEnabled === true,
      feedbackV2AuthMode: mode,
      apiKey: mode === 'apiKey' ? String(value.apiKey) : undefined,
      feedbackSessionToken: mode === 'session' ? String(value.feedbackSessionToken) : undefined,
      projectId: value.projectId,
      customUserId: String(value.customUserId),
      gatewayUrl: String(value.gatewayUrl),
      theme: value.theme === 'dark' || value.theme === 'light' ? value.theme : 'system',
      locale: value.locale === 'en-US' ? 'en-US' : 'zh-CN',
      parentOrigins: [window.location.origin],
      hostContext: buildHostContext()
    };
  }

  function setEntryVisible(visible) {
    if (notice) notice.textContent = visible ? '反馈服务已连接；当前登录账号可直接使用。' : '反馈服务暂时不可用，OMS 其他功能不受影响。';
  }

  function buildHostContext() {
    var activePage = document.querySelector('[data-oms-v1-page].is-active');
    var allowedRoutes = ['overview', 'projects', 'receivable', 'payable', 'internal', 'open-api', 'account', 'feedback'];
    var route = activePage && allowedRoutes.indexOf(activePage.dataset.omsV1Page) >= 0 ? activePage.dataset.omsV1Page : 'unknown';
    var browser = 'browser';
    if (navigator.userAgentData && Array.isArray(navigator.userAgentData.brands)) {
      browser = navigator.userAgentData.brands.map(function (brand) { return String(brand.brand || '').slice(0, 40); }).filter(Boolean).join(',').slice(0, 120) || browser;
    }
    return {
      release: 'oms1',
      build: '20261026.1',
      module: 'workbench',
      route: route,
      browser: browser,
      os: String((navigator.userAgentData && navigator.userAgentData.platform) || 'unknown').slice(0, 40),
      locale: String(navigator.language || 'zh-CN').slice(0, 20),
      clientTimestamp: new Date().toISOString(),
      requestId: randomID()
    };
  }

  async function fetchRuntimeConfig(force) {
    if (currentIdentityKey() !== identityKey) clearRuntime();
    if (!force && runtimeConfig) return runtimeConfig;
    if (!force && configRequest) return configRequest;
    var csrf = csrfToken();
    if (!csrf) throw new Error('missing csrf');
    configRequest = fetch('/gateway/feedback/config', {
      method: 'POST',
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      headers: { Accept: 'application/json', 'X-CSRF-Token': csrf }
    }).then(function (response) {
      if (!response.ok) {
        var error = new Error('feedback config unavailable');
        error.status = response.status;
        throw error;
      }
      return response.json();
    }).then(function (payload) {
      runtimeConfig = validateConfig(payload && payload.config);
      setEntryVisible(true);
      return runtimeConfig;
    }).catch(function (error) {
      setEntryVisible(false);
      if (notice && error && error.status === 401) notice.textContent = '登录已失效，请重新登录后使用产品反馈。';
      throw error;
    }).finally(function () {
      configRequest = null;
    });
    return configRequest;
  }

  function ensureFrame() {
    if (frame) return frame;
    layer = document.createElement('div');
    layer.className = 'oms-v1__feedback-layer';
    layer.hidden = true;
    layer.setAttribute('data-feedback-layer', '');
    frame = document.createElement('iframe');
    frame.className = 'oms-v1__feedback-frame';
    frame.title = '产品反馈';
    frame.referrerPolicy = 'no-referrer';
    frame.setAttribute('allow', 'clipboard-read; clipboard-write');
    frame.src = sdkURL;
    layer.appendChild(frame);
    document.body.appendChild(layer);
    return frame;
  }

  function awaitReady() {
    if (ready) return Promise.resolve();
    return new Promise(function (resolve, reject) {
      var waiter = { resolve: resolve, reject: reject, timer: 0 };
      waiter.timer = window.setTimeout(function () {
        readyWaiters = readyWaiters.filter(function (candidate) { return candidate !== waiter; });
        reject(new Error('feedback sdk timeout'));
      }, 10000);
      readyWaiters.push(waiter);
    });
  }

  function resolveReady() {
    ready = true;
    var waiters = readyWaiters.slice();
    readyWaiters = [];
    waiters.forEach(function (waiter) { window.clearTimeout(waiter.timer); waiter.resolve(); });
  }

  function post(type, extra) {
    if (!frame || !frame.contentWindow || !sdkOrigin) return;
    frame.contentWindow.postMessage(Object.assign({ source: 'feedback-sdk-host', type: type }, extra || {}), sdkOrigin);
  }

  function sendConfiguration(config) {
    post('feedback-sdk:configure', { config: config });
  }

  function setUnread(value) {
    var count = Number.isFinite(Number(value)) ? Math.max(0, Math.floor(Number(value))) : 0;
    badges.forEach(function (badge) {
      badge.hidden = count === 0;
      badge.textContent = count > 99 ? '99+' : String(count);
      badge.setAttribute('aria-label', count + ' 条未读反馈消息');
    });
  }

  function requestUnread() {
    unreadRequestID = randomID();
    post('feedback-sdk:get-unread-count', { request_id: unreadRequestID });
  }

  function showFailure() {
    var old = document.querySelector('[data-feedback-toast]');
    if (old) old.remove();
    var toast = document.createElement('div');
    toast.className = 'oms-v1__feedback-toast';
    toast.setAttribute('data-feedback-toast', '');
    toast.setAttribute('role', 'status');
    toast.textContent = '产品反馈暂时不可用，OMS 其他功能不受影响。';
    document.body.appendChild(toast);
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () { toast.remove(); }, 4200);
  }

  async function openFeedback(mode) {
    if (!sdkURL || (mode !== 'submit' && mode !== 'status')) return showFailure();
    try {
      var config = await fetchRuntimeConfig(false);
      ensureFrame();
      await awaitReady();
      sendConfiguration(config);
      post('feedback-sdk:open', { mode: mode });
      layer.hidden = false;
    } catch (_) {
      if (layer) layer.hidden = true;
      showFailure();
    }
  }

  async function prepareUnread() {
    if (!sdkURL) {
      setEntryVisible(false);
      return;
    }
    if (document.visibilityState === 'hidden') return;
    try {
      var config = await fetchRuntimeConfig(false);
      ensureFrame();
      await awaitReady();
      sendConfiguration(config);
      requestUnread();
    } catch (_) {
      // Feedback is auxiliary. Preparation failure must never affect OMS.
    }
  }

  function clearRuntime() {
    identityKey = currentIdentityKey();
    if (notice) notice.textContent = '反馈会话已结束。';
    runtimeConfig = null;
    configRequest = null;
    unreadRequestID = '';
    setUnread(0);
    ready = false;
    readyWaiters.forEach(function (waiter) { window.clearTimeout(waiter.timer); waiter.reject(new Error('feedback cleared')); });
    readyWaiters = [];
    if (frame) frame.src = 'about:blank';
    if (layer) layer.remove();
    frame = null;
    layer = null;
  }

  window.addEventListener('message', function (event) {
    if (!frame || event.origin !== sdkOrigin || event.source !== frame.contentWindow) return;
    var message = event.data;
    if (!message || message.source !== 'feedback-sdk-web' || typeof message.type !== 'string') return;
    if (message.type === 'feedback-sdk:ready') {
      resolveReady();
      if (runtimeConfig) sendConfiguration(runtimeConfig);
      return;
    }
    if (message.type === 'feedback-sdk:close') {
      if (layer) layer.hidden = true;
      return;
    }
    if (message.type === 'feedback-sdk:refresh-session') {
      runtimeConfig = null;
      void fetchRuntimeConfig(true).then(sendConfiguration).catch(showFailure);
      return;
    }
    if (message.type === 'feedback-sdk:unread-changed') {
      setUnread(message.unread_count);
      return;
    }
    if (message.type === 'feedback-sdk:unread-count' && message.request_id === unreadRequestID) {
      unreadRequestID = '';
      setUnread(message.unread_count);
    }
  });

  page.addEventListener('click', function (event) {
    var button = event.target.closest('[data-feedback-mode]');
    if (button) void openFeedback(button.dataset.feedbackMode);
  });
  Array.prototype.forEach.call(document.querySelectorAll('form[action="/gateway/logout"]'), function (form) {
    form.addEventListener('submit', clearRuntime);
  });
  window.addEventListener('pagehide', clearRuntime, { once: true });

  if (window.requestIdleCallback) window.requestIdleCallback(function () { void prepareUnread(); }, { timeout: 3000 });
  else window.setTimeout(function () { void prepareUnread(); }, 1800);
})();
