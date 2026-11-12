# OMS Feedback V3 快照非阻塞发布验证（BF-327，2026-11-11）

> 2026-11-11 纠错：本报告消除了 OMS 对辅助快照 502 的硬依赖，但当时只验证 V3 contract，没有发现 SDK 3.0.2 的 React 配置时序竞态；用户截图仍显示 V2 表单。正式 `sdk-v3` 已由 3.0.4 修复并完成可见产品面验收。[纠错报告](./oms-feedback-v3-product-surface-race-repair-20261111.md)

## 结论

本次升级目标确认是 OMS 网站内嵌的 Feedback SDK：OMS 已在 BF-325 从 `sdk-v2` 切换到 `sdk-v3`，稳定版本为 3.0.2。当前“反馈不可用”不是升级了错误对象，也不是 API Key 被撤销；问题是 OMS 宿主在打开 V3 iframe 前等待辅助的 `submission-snapshot`，该请求瞬时失败后把整个反馈层隐藏。

Workbench v1.98 已把快照降为并行可选增强。配置、V3 ready／configured 与 open 是主链；快照失败返回 `null`，SDK 继续打开。只有成功取得基线并且提交层仍打开时，才启动提交后有界轮询。降级影响仅是本次可能不会由宿主自动切到“我的反馈”，SDK 的提交旅程和 OMS 其他功能仍可使用。

## 故障证据与边界

- OMS Gateway 在同一打开操作中返回 Feedback 配置 200，而 `submission-snapshot` 约 5 秒后返回 502。
- Feedback 网关日志显示 V3 首次加载同时请求会话、通知、能力和反馈列表，API Key 校验等待 auth-server 约 9.9～12.9 秒，超过上游 5 秒预算；稍后同一 Key 的通知轮询恢复 200 和毫秒级响应。因此 Key 有效，不能把故障解释为撤销或配置错 Key。
- 本次不放大 Gateway 超时，不绕过 CSRF／Session／Tenant／Subject、固定 origin／window、V3 contract 和 request ID 校验，也不在 OMS 保存 Feedback 事实。
- Feedback 平台的冷启动校验合并、缓存与超时预算仍是相邻运维事项；本次只消除 OMS 对辅助快照的硬依赖。

## 实现与测试

- `openFeedback('submit')` 并行发起快照并捕获失败，不再在 `ensureFrame`、V3 ready、configure、open 和显示反馈层之前等待它。
- 快照成功后再次核对反馈层仍存在、未关闭且仍是 submit 模式，才启动观察；失败不产生轮询计时器。
- Feedback 专项 Node 32 项、Workbench 全量 Node 303/303、Workbench `go test`／`go vet` 通过。
- P8 真实跨域 Chrome 新增 `snapshot-outage-nonblocking`：快照持续 502 时 SDK 仍进入 submit、反馈层可见、关闭后业务按钮可用，5 项断言通过；其余主桥 24、Session 失效 5、服务不可用 5 项也通过，浏览器 console／page error 为零。

## 公网发布

- Suite／Unified Workbench：`v0.1.0-test.oms-v1.98`。
- Workbench OCI index：`sha256:a1f43df548462b626146c4788798ebd276d9556b3c601a2a131a4f99869903f0`；Linux AMD64 manifest：`sha256:ed734688c8ab1cdee52da83bf11f530395d9cd28da1abf162c17865aec3fd696`。
- D1 发布包 SHA-256：`bf0f12240f4ab0e54463bdb89d7594c1b3cd91bae71d4d7c280d4165e9e1e6ec`。
- 正式切换前完整恢复点：`20261111T140730Z`；21 项 `checksums.sha256` 和数据库指纹存在并通过。
- 候选 IAM 契约预检返回 `contracts-valid`；发布后独立 D1 check 与 21/21 长运行容器 healthy 通过。
- 运行 Workbench 的 `FEEDBACK_SDK_URL` 仍为 `https://feedback.feitianchengzi.com/sdk-v3/index.html?embed=web`。
- 公网 `oms-phase1-feedback.js?v=20261111.2` 与本地 SHA-256 均为 `b0b5883b9d3b39c7df16701dac70c095ce9026a97e756bc375398b51360e4efa`。
- 公网根入口返回 303 到 `/workbench/?view=oms-v1`，Feedback JS 与 `sdk-v3/index.html?embed=web` 均返回 200。

升级在备份完成后首次停止旧栈时，Docker Compose 因两周前的 `omsv152-inspect` 调试容器仍占用 `omsbizfoundry_app` 网络而未进入启动阶段；命名数据卷未删除。保留该调试容器和恢复点后，使用同一候选环境执行标准 D1 `up` 恢复成功，并完成上述独立检查。共享浏览器控制仍连接超时，因此本报告不把真人登录后的公网点击描述为通过；本轮未提交反馈，也未写入 OMS 业务事实。
