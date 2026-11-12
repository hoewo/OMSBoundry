# OMS Feedback 短期 Session 无感续期本地验证

日期：2026-11-12

决策：BF-330

## 问题与结果

问题 A：Feedback 的项目 API Key 直接进入浏览器；迁移到 15 分钟 Session 后，宿主又只在 Token 已过期并收到 SDK 401 时被动换票，用户会遇到短时中断，并且多个前台／恢复事件可能重复触发换票。

可验证结果 B：正式 D1 只允许 `session` 模式。长期项目 API Key 仅由 OMS Gateway 保存；浏览器内只存在项目＋稳定匿名用户限域的 15 分钟 `fbs_` Session，宿主在到期前约 2 分钟主动续期，并对并发续期做 single-flight。后台页恢复时重新检查，SDK 明确过期时强制换票且原请求最多重放一次；临时主动续期失败时，只在旧 Session 仍有效的剩余窗口内继续使用并有界重试。

## 方案边界

- 采用 token-mediating backend／BFF 形态，不把长期 API Key 或 Refresh Token 交给 iframe、浏览器存储、URL、日志或 OMS 业务库。
- `fbs_` 仍是短期访问凭证，不通过简单延长 TTL 解决体验问题；Gateway 拒绝少于 1 分钟可用期或超过 20 分钟的异常换票响应。
- Gateway 继续从当前可信 OMS Session 派生 Tenant＋Subject 的稳定匿名身份；账号退出、切换、停用或 OMS Session 失效后不能继续换取新 Session。
- Capability Credential 的 5 分钟上限、Workload Credential 的 1 分钟上限以及 OMS 登录 Session 的 12 小时默认值均未改变。
- Feedback 事实、消息、智能客服会话和未读仍由 Feedback 小系统拥有；OMS 只负责宿主装配和短期凭证中介。

该选择与 [RFC 10017 的 Token-Mediating Backend](https://www.rfc-editor.org/rfc/rfc10017.html#section-6.2)一致：长期续期能力留在后端，浏览器按需取得短期访问令牌且不持久化 Refresh Token；短期、限域访问凭证也符合 [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html#section-4.14) 的泄漏影响收敛原则。

## 实现

- `backend/platform/gateway/internal/gateway/feedback.go`
  - Session 换票继续由服务端持有的当前／上一把 API Key 完成；
  - 校验 `fbs_` family、Bearer 类型、Project、Custom User 及 1～20 分钟剩余期限；
  - `/gateway/feedback/config` 只向浏览器返回短期 Token、`expiresAt` 与服务端计算的相对剩余秒数，不返回 API Key；Workbench 使用相对期限调度，避免终端时钟偏差造成误过期。
- `frontend/oms-workbench/internal/server/oms_phase1_feedback.js`
  - 把宿主租约元数据与发送给 SDK 的配置分离；
  - 到期前 2 分钟主动刷新，标签页重新可见时复核；
  - 同一时刻只允许一次配置／换票请求；
  - SDK 401 触发强制换票，主动续期的临时失败只在旧 Token 仍有效时有界退避；
  - 退出、页面销毁和身份变化清理计时器与内存凭证。
- `deployments/d1/ops/d1` 与 P9 门禁
  - 正式 D1 从允许浏览器 `apiKey` 收敛为必须 `session`；
  - API Key 仍作为 Gateway 的服务端换票凭证并支持最多两把有界轮换。

## 实际验证

- Gateway 与 Workbench 分别运行 `go test ./...`、`go vet ./...`：通过；覆盖 Session 配置中介、API Key 不下发、正确范围、过短／过长／错 Project／错用户拒绝及轮换回退。
- `node frontend/oms-workbench/internal/server/oms_phase1_feedback.test.mjs`：47 项断言通过；动态验证首次换票、约 13 分钟续期调度、SDK 配置不携带宿主到期元数据、两个并发过期事件只产生一次服务端换票。
- `node --test frontend/oms-workbench/internal/server/*.test.mjs`：303 项通过、0 失败、0 skip。
- `node --check frontend/oms-workbench/internal/server/oms_phase1_feedback.js`：通过。
- `acceptance/p9-full-composition/test-preflight`：通过。
- `deployments/d1/ops/test-p9-composition`：通过；正式组合接受 Session、拒绝 apiKey，并保持备份／恢复选择闭包。
- `git diff --check`：通过。

## 尚未完成

- 本轮没有部署 `oatest`，没有修改线上 `FEEDBACK_AUTH_MODE`，也没有写入 Feedback 或 OMS 业务事实。
- 仍需在发布前备份后，把线上 Gateway 环境切到 `session`，构建 Gateway／Workbench，新开真实 Chrome 保持超过 15 分钟并验证智能客服、提交、我的反馈、未读、后台恢复和退出后的换票拒绝。
- 本轮不引入浏览器 Refresh Token、DPoP、OAuth Authorization Code 流或新的通用认证平台；OMS 与 Feedback 是既有可信服务端换票关系。
