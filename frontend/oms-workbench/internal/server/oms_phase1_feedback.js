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
  var readyFailure = null;
  var requiredSDKContract = 'v3';
  var readyWaiters = [];
  var runtimeConfig = null;
  var configRequest = null;
  var csrfRequest = null;
  var configuredConfig = null;
  var configurationRequest = null;
  var identityKey = currentIdentityKey();
  var unreadRequestID = '';
  var toastTimer = 0;
  var lastOpenedMode = 'submit';
  var dismissedBySDK = false;
  var submissionWatchGeneration = 0;
  var submissionWatchTimer = 0;

  function currentIdentityKey() {
    return String(root.dataset.tenantId || '') + '\u0000' + String(root.dataset.subjectId || '');
  }

  function randomID() {
    if (window.crypto && typeof window.crypto.randomUUID === 'function') return window.crypto.randomUUID();
    var bytes = new Uint8Array(16);
    window.crypto.getRandomValues(bytes);
    return Array.prototype.map.call(bytes, function (value) { return value.toString(16).padStart(2, '0'); }).join('');
  }

  function currentCSRFToken() {
    var match = document.cookie.match(/(?:^|;\s*)oms_iam_csrf=([A-Za-z0-9_-]{16,256})(?:;|$)/);
    return match ? match[1] : '';
  }

  function csrfToken() {
    var current = currentCSRFToken();
    if (current) return Promise.resolve(current);
    if (csrfRequest) return csrfRequest;
    var route = String(root.dataset.identityRoutePrefix || '').replace(/\/$/, '');
    if (!/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(route)) return Promise.reject(new Error('invalid identity route'));
    csrfRequest = fetch(route + '/api/v1/csrf', {
      method: 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      headers: { Accept: 'application/json' }
    }).then(function (response) {
      if (!response.ok || response.redirected) {
        var error = new Error('csrf unavailable');
        error.status = response.status;
        throw error;
      }
      return response.json();
    }).then(function (payload) {
      var refreshed = currentCSRFToken();
      if (!refreshed || !payload || payload.csrf_token !== refreshed) throw new Error('invalid csrf response');
      return refreshed;
    }).finally(function () {
      csrfRequest = null;
    });
    return csrfRequest;
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
    if (notice) notice.textContent = visible ? '智能客服与反馈服务已连接；当前登录账号可直接使用。' : '智能客服与反馈服务暂时不可用，OMS 其他功能不受影响。';
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
      build: '20261111.2',
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
    configRequest = csrfToken().then(function (csrf) {
      return fetch('/gateway/feedback/config', {
        method: 'POST',
        credentials: 'same-origin',
        cache: 'no-store',
        redirect: 'error',
        headers: { Accept: 'application/json', 'X-CSRF-Token': csrf }
      });
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
    frame.title = '智能客服与产品反馈';
    frame.referrerPolicy = 'no-referrer';
    frame.setAttribute('allow', 'microphone; clipboard-read; clipboard-write');
    frame.src = sdkURL;
    layer.appendChild(frame);
    document.body.appendChild(layer);
    return frame;
  }

  function awaitReady() {
    if (ready) return Promise.resolve();
    if (readyFailure) return Promise.reject(readyFailure);
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
    readyFailure = null;
    var waiters = readyWaiters.slice();
    readyWaiters = [];
    waiters.forEach(function (waiter) { window.clearTimeout(waiter.timer); waiter.resolve(); });
  }

  function rejectReady(message) {
    ready = false;
    readyFailure = new Error(message);
    var waiters = readyWaiters.slice();
    readyWaiters = [];
    waiters.forEach(function (waiter) { window.clearTimeout(waiter.timer); waiter.reject(readyFailure); });
    if (notice) notice.textContent = '反馈 V3 资源校验失败，请稍后重试；OMS 其他功能不受影响。';
  }

  function post(type, extra) {
    if (!frame || !frame.contentWindow || !sdkOrigin) return;
    frame.contentWindow.postMessage(Object.assign({ source: 'feedback-sdk-host', type: type }, extra || {}), sdkOrigin);
  }

  function configureFrame(config) {
    if (configuredConfig === config) return Promise.resolve();
    if (configurationRequest && configurationRequest.config === config) return configurationRequest.promise;
    if (configurationRequest) {
      window.clearTimeout(configurationRequest.timer);
      configurationRequest.reject(new Error('feedback configuration superseded'));
    }
    var requestID = randomID();
    var resolveRequest;
    var rejectRequest;
    var promise = new Promise(function (resolve, reject) {
      resolveRequest = resolve;
      rejectRequest = reject;
    });
    configurationRequest = {
      config: config,
      requestID: requestID,
      promise: promise,
      resolve: resolveRequest,
      reject: rejectRequest,
      timer: window.setTimeout(function () {
        if (!configurationRequest || configurationRequest.requestID !== requestID) return;
        configurationRequest = null;
        rejectRequest(new Error('feedback configuration timeout'));
      }, 10000)
    };
    post('feedback-sdk:configure', { config: config, request_id: requestID });
    return promise;
  }

  function acceptConfiguration(requestID) {
    if (!configurationRequest || configurationRequest.requestID !== requestID) return;
    var accepted = configurationRequest;
    window.clearTimeout(accepted.timer);
    configurationRequest = null;
    configuredConfig = accepted.config;
    accepted.resolve();
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

  function showToast(message, successful) {
    var old = document.querySelector('[data-feedback-toast]');
    if (old) old.remove();
    var toast = document.createElement('div');
    toast.className = 'oms-v1__feedback-toast' + (successful ? ' is-success' : '');
    toast.setAttribute('data-feedback-toast', '');
    toast.setAttribute('role', 'status');
    toast.textContent = message;
    document.body.appendChild(toast);
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () { toast.remove(); }, 4200);
  }

  function showFailure() {
    showToast('产品反馈暂时不可用，OMS 其他功能不受影响。', false);
  }

  async function fetchSubmissionSnapshot() {
    var csrf = await csrfToken();
    var response = await fetch('/gateway/feedback/submission-snapshot', {
      method: 'POST',
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      headers: { Accept: 'application/json', 'X-CSRF-Token': csrf }
    });
    if (!response.ok) {
      var error = new Error('feedback snapshot unavailable');
      error.status = response.status;
      throw error;
    }
    var payload = await response.json();
    if (!payload || !Array.isArray(payload.feedback_ids) || !/^[0-9]+$/.test(String(payload.feedback_count || ''))) throw new Error('invalid feedback snapshot');
    return {
      ids: new Set(payload.feedback_ids.map(function (id) { return String(id); }).filter(Boolean)),
      count: String(payload.feedback_count).replace(/^0+(?=\d)/, '')
    };
  }

  function decimalGreater(left, right) {
    if (left.length !== right.length) return left.length > right.length;
    return left > right;
  }

  function stopSubmissionWatch() {
    submissionWatchGeneration += 1;
    window.clearTimeout(submissionWatchTimer);
    submissionWatchTimer = 0;
  }

  function completeFeedbackSubmission() {
    stopSubmissionWatch();
    lastOpenedMode = 'status';
    dismissedBySDK = false;
    post('feedback-sdk:open', { mode: 'status' });
    requestUnread();
    showToast('反馈已提交，已进入“我的反馈”查看处理进度。', true);
  }

  function startSubmissionWatch(baseline) {
    stopSubmissionWatch();
    var generation = submissionWatchGeneration;
    var deadline = Date.now() + 5 * 60 * 1000;
    var poll = async function () {
      if (generation !== submissionWatchGeneration || !layer || layer.hidden || lastOpenedMode !== 'submit') return;
      var delay = 1200;
      try {
        var current = await fetchSubmissionSnapshot();
        var created = decimalGreater(current.count, baseline.count);
        if (created) {
          completeFeedbackSubmission();
          return;
        }
      } catch (error) {
        delay = 2600;
        if (error && (error.status === 401 || error.status === 403)) {
          stopSubmissionWatch();
          return;
        }
      }
      if (generation !== submissionWatchGeneration || Date.now() >= deadline) return;
      submissionWatchTimer = window.setTimeout(poll, delay);
    };
    submissionWatchTimer = window.setTimeout(poll, 900);
  }

  async function openFeedback(mode) {
    if (!sdkURL || (mode !== 'submit' && mode !== 'status')) return showFailure();
    stopSubmissionWatch();
    try {
      var config = await fetchRuntimeConfig(false);
      // The snapshot only improves the post-submit transition. A cold or
      // temporarily slow Feedback API must never prevent the V3 SDK itself
      // from opening; the SDK remains the authority for the submit journey.
      var submissionBaselineRequest = mode === 'submit'
        ? fetchSubmissionSnapshot().catch(function () { return null; })
        : Promise.resolve(null);
      ensureFrame();
      await awaitReady();
      await configureFrame(config);
      post('feedback-sdk:open', { mode: mode });
      lastOpenedMode = mode;
      dismissedBySDK = false;
      layer.hidden = false;
      if (mode === 'submit') {
        void submissionBaselineRequest.then(function (submissionBaseline) {
          if (!submissionBaseline || !layer || layer.hidden || lastOpenedMode !== 'submit') return;
          startSubmissionWatch(submissionBaseline);
        });
      }
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
      await configureFrame(config);
      requestUnread();
    } catch (_) {
      // Feedback is auxiliary. Preparation failure must never affect OMS.
    }
  }

  function clearRuntime() {
    stopSubmissionWatch();
    identityKey = currentIdentityKey();
    if (notice) notice.textContent = '反馈会话已结束。';
    runtimeConfig = null;
    configRequest = null;
    configuredConfig = null;
    if (configurationRequest) {
      window.clearTimeout(configurationRequest.timer);
      configurationRequest.reject(new Error('feedback configuration cleared'));
      configurationRequest = null;
    }
    unreadRequestID = '';
    dismissedBySDK = false;
    setUnread(0);
    ready = false;
    readyFailure = null;
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
      if (!Array.isArray(message.supported_contracts) || message.supported_contracts.indexOf(requiredSDKContract) < 0) {
        rejectReady('feedback sdk does not support ' + requiredSDKContract);
        return;
      }
      resolveReady();
      return;
    }
    if (message.type === 'feedback-sdk:configured' && typeof message.request_id === 'string') {
      acceptConfiguration(message.request_id);
      return;
    }
    if (message.type === 'feedback-sdk:close') {
      stopSubmissionWatch();
      if (layer) layer.hidden = true;
      dismissedBySDK = true;
      return;
    }
    if (message.type === 'feedback-sdk:refresh-session') {
      runtimeConfig = null;
      configuredConfig = null;
      void fetchRuntimeConfig(true).then(configureFrame).catch(showFailure);
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
  document.addEventListener('click', function (event) {
    if (!dismissedBySDK || !event.target.closest('[data-oms-v1-nav="feedback"]')) return;
    void openFeedback(lastOpenedMode);
  }, true);
  Array.prototype.forEach.call(document.querySelectorAll('form[action="/gateway/logout"]'), function (form) {
    form.addEventListener('submit', clearRuntime);
  });
  window.addEventListener('pagehide', clearRuntime, { once: true });

  if (window.requestIdleCallback) window.requestIdleCallback(function () { void prepareUnread(); }, { timeout: 3000 });
  else window.setTimeout(function () { void prepareUnread(); }, 1800);
})();
