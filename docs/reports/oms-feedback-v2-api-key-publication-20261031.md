# OMS Feedback V2 API Key 接入与公网发布记录

- 日期：2026-10-31
- 决策：BF-299
- 结果：通过
- 公网入口：`https://oatest.antuinfo.cn/`
- Feedback 项目：115

## 问题与结论

API Key、项目 115、稳定脱敏用户 ID 和 Feedback API 网关原本已经通过 D1 部署配置注入，Gateway 的受保护运行时配置端点也能返回完整配置。实际断点在公网 Feedback SDK V2 的旧构建：其宿主来源允许清单没有 `https://oatest.antuinfo.cn`，因此拒绝 Workbench 发出的 `configure` 和 `open` 消息；iframe 默认页面仍会显示，造成“弹窗已打开但提示未检测到 API Key”的假连接。

本次同时修复 SDK 发布和 Workbench 宿主握手：

1. Feedback SDK V2 公网构建显式允许 `https://oatest.antuinfo.cn`；
2. SDK 接受 `feedback-sdk:configure` 后，以相同 `request_id` 回传 `feedback-sdk:configured`，回复中不包含 API Key 或其它配置值；
3. Workbench 只有收到来源、窗口和 `request_id` 都匹配的配置确认后，才允许打开“提交反馈”“我的反馈”或读取未读数；
4. 配置确认 10 秒超时即失败关闭，不再把 iframe 可见误判为接入成功；
5. Session 刷新后重新获取受保护配置并重新握手，退出登录时清理配置状态。

## 安全边界

- API Key 仍只存在于部署环境和登录后受 CSRF 保护的 `/gateway/feedback/config` 响应中，没有写入 Git、HTML、静态 JavaScript、URL、日志或本报告。
- `customUserId` 仍由可信 Tenant＋Subject 派生为稳定脱敏值，不使用手机号、邮箱或页面自报身份。
- Workbench 只装配入口与宿主通信；反馈、消息、附件、通知和处理状态继续由 Feedback 平台拥有。
- 本次公网验收只打开提交页和状态页，没有填写或提交反馈；Feedback API 写请求为 0。

## 代码与发布物

### Feedback SDK V2

- SDK 源码增加配置确认协议和宿主测试。
- 公网 `sdk-v2/index.html` 已更新，生成资源：
  - `assets/index-20261030154449-z8UiFuKV.js`
  - `assets/index-20261030154449-BldpMKPQ.css`
- 仅发布 `sdk-v2/`，未覆盖旧 `/sdk` 或其它 OSS 对象。

### Unified Workbench

- 版本：`v0.1.0-test.oms-v1.68`
- 镜像摘要：`sha256:e54991203a63fd7c6d7730f06c96dc6f72da9cf40fc4cf610d1f9cd5bd14a082`
- Feedback 静态资源版本：`20261030.2`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T154728Z`

## 验证证据

本地与跨域浏览器：

- Feedback SDK V2 专项测试 37/37 通过；
- SDK V2 生产构建检查通过；
- Workbench `go test ./...`、`go vet ./...`、JavaScript 语法检查通过；
- P8 真实 Chromium 双来源验收通过：配置确认、提交／状态入口、未读数、Session 刷新与失效、来源伪造拒绝均通过；
- `git diff --check` 通过。

公网：

- D1 正式 upgrade 成功，独立 `d1 check` 通过；
- 19/19 长运行容器健康；
- Workbench 运行镜像与上述不可变摘要一致；
- 公网 Feedback 宿主 JavaScript 与本地源码 SHA-256 一致；
- 登录后的 `/gateway/feedback/config` 返回：V2 启用、`apiKey` 模式、项目 115、存在专用 Key、存在稳定用户 ID、网关为 `https://api.feitianchengzi.com`、主题为 `system`；验证输出不包含具体凭证；
- 从 `https://oatest.antuinfo.cn` 对公网 SDK 完成带 `request_id` 的真实配置确认；“提交反馈”页面和 `/sdk-v2/status`“我的反馈”页面均可打开，未出现“未检测到 API Key”；Feedback 写请求为 0。

## 未改变事项

本次不改变 IAM、Gateway 鉴权、Feedback 数据所有权、Owner API、schema、Grant、业务数据或通知已读规则；也没有创建测试反馈。`apiKey` 客户端可见的既有安全边界仍按 BF-235 执行，后续如迁移到短期服务端 Session，需要另行决策和验收。
