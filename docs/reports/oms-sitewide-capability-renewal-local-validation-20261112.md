# OMS 全站 Capability 无感续期本地验证（2026-11-12）

## 结论

本报告记录发布前本地证据；BF-331 后续已随 v1.99 发布到 `oatest`，公网结果见[发布记录](./oms-capability-feedback-v199-publication-20261112.md)。OMS 的全部 Owner API 继续使用原有“双重校验”链：浏览器持有按 Owner 路径隔离的 HttpOnly Capability Cookie，Gateway 校验后删除任何客户端伪造身份与 Authorization，再注入 `Authorization: Bearer <Capability JWT>`；目标 Owner 重新校验签名、Audience、动作、Scope、Tenant 和业务状态。此次没有把 Feedback 作为业务鉴权核心，也没有把 Token 放进 JavaScript、Local Storage、Session Storage、URL 或业务库。

## 问题 A → 可验证结果 B

- A：5 分钟业务 Capability 只在 Owner 返回重定向／401 后才换票，用户在长时间停留或从后台恢复时会看到业务读取失败。
- B：Workbench 现在按应用 Audience 维护不含凭证的内存租约，在到期前最多 60 秒主动续期；首次读取、标签恢复和写入前都经统一协调器，所有同 Audience 并发换票合并为一次。
- A：主动续期遇到 IAM 短暂 5xx 会清除仍有效 Cookie，反而把可继续验证的读取变成失败。
- B：Gateway 只在 IAM 明确 4xx 拒绝或签发的能力不满足入口时清除旧 Cookie；网络、5xx 或无效上游响应不会覆盖仍有效凭证。读取可在旧租约剩余期限内继续，写请求必须换到新票后才发送。

## 实现边界

1. `POST /gateway/authorize/{application}` 在 `Accept: application/json` 时返回 `application_id`、`audience`、`expires_at` 和 `expires_in_seconds`，不返回 Token、Claims 或 Cookie 内容；原 303 行为保持兼容。
2. 租约的最长可接受剩余时间为 300 秒；浏览器使用相对期限调度，避免依赖客户端时钟与服务端绝对时间完全一致。
3. 读请求在租约仍处于安全窗口时直接复用；写请求始终在发送前强制换票。Owner 的 401 `credential_required` 仍只触发一次换票与一次原请求重放。
4. Feedback 的 15 分钟 `fbs_` Session 仍由 BF-330 的独立 Gateway 中介负责；IAM Session、开放 API Key 管理端口和 Evidence 签名数据面不伪装成业务 Owner Capability。
5. Suite Session、Capability JWT 的 5 分钟 TTL、IAM Grant、Owner API、schema、业务事实及最终授权规则均未改变。

## 已运行验证

- Gateway：`go test ./...`、`go vet ./...`，通过。
- Unified Workbench：`go test ./...`、`go vet ./...`，通过。
- Workbench JavaScript：三个变更文件 `node --check`，通过。
- Workbench Node 全量：306/306，通过；新增覆盖提前续期、并发 single-flight、短暂失败保留、严格写入阻断、明确拒绝清理及无浏览器凭证存储。
- P9：`acceptance/p9-full-composition/test-preflight`，通过。
- D1：`deployments/d1/ops/test-p9-composition`，通过。
- `git diff --check`，通过。

## 未完成

尚未备份或更新 `oatest`，未登记公网发布、真实登录页面、超过 5 分钟的前台持续读取、后台恢复和退出后换票拒绝为通过。
