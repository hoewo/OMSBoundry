# OMS1 迭代 P8 Feedback SDK V2 源码门禁报告

日期：2026-10-26

结论：P8 的 OMS 宿主、Gateway 适配、D1 配置边界和 Arckit SDK 兼容修改已通过本地源码门禁；BF-246 又以真实 Chromium 和两个随机跨 origin 回环服务完成生产宿主脚本的本地浏览器门禁。真实项目 115 的通知开关、有效部署凭证、两个账号／两个租户、附件、回复／单条已读和外部跨域旅程尚未运行，因此 P8 运行验收未完成，也未部署 D1 或公网。

## 1. 问题与可验证结果

问题 A：主 OMS1 没有正式 Feedback V2 入口，浏览器若直接持有明文 IAM 身份或把项目 Key 写入源码，会破坏租户隔离与秘密边界；SDK 也尚不能接收 BF-237 允许的最小宿主诊断上下文。

结果 B：只有持有 `feedback.use` 的已登录账号才能从 Gateway 取得一次 `no-store` 运行配置；`customUserId` 由 Tenant＋Subject 通过版本化 HMAC 派生，Workbench 只创建一个受精确 origin／source 约束的跨域 iframe，并提供提交、本人状态、未读角标、退出清理和故障降级。SDK 对诊断信息再次执行闭集 allowlist。部署秘密只通过环境变量注入，源码、文档和示例中没有有效 Key。

## 2. OMS 源码范围

- Gateway 新增 `POST /gateway/feedback/config`。请求必须通过同源 CSRF、有效登录会话和 IAM `feedback.use` Credential；响应禁止缓存，不返回 Tenant、Subject 或会话标识。
- 稳定反馈身份格式为 `oms1_<version>_<HMAC-SHA256>`；不同 Tenant 或 Subject 得到不同值，轮换身份算法时必须显式提升版本。
- 首版支持已经接受风险的 `apiKey` 模式；项目专用 Key 只从部署环境读取、只在受保护运行响应中出现、不持久化到浏览器存储。服务端 15 分钟 Session 模式和双 Key 有界轮换回退已经保留，但当前部署未切换。
- Workbench 只在配置与能力检查成功后显示桌面／移动端反馈入口；提交反馈与我的反馈复用一个 iframe。入站消息同时校验精确 origin、iframe window、source、已知消息类型和未读请求 ID。
- 失败的 SDK 预加载、配置、角标或 iframe 不阻塞 OMS 页面；显式操作失败只显示局部提示。透明区域不由宿主关闭弹层，关闭只接受 SDK 的已校验消息。
- 自动诊断逐字段构建，只包含 release、build、模块、枚举路由键、粗粒度浏览器／系统、语言、时间和随机 request ID；不发送 URL、资源 ID、表单、金额、文件、完整 User-Agent、凭证、Cookie 或堆栈。
- D1 Compose 和示例环境增加 SDK URL、Feedback Gateway、项目 ID、认证模式、通知开关、Key 与独立 HMAC Key 的显式配置；示例只含空占位。Grant bootstrap 登记正式 Feedback Capability Manifest。
- 2026-10-26 BF-245 复核纠正：此前 `FEEDBACK_SDK_URL` 在 Compose 中误入 Bonus 环境、未注入 Workbench，且部分配置只会在容器启动后失败。现已把 SDK URL 归还 Workbench，并由 D1 在任何状态变化前成组校验首发 `apiKey`、项目 115、HTTPS／回环 URL、HMAC、通知值及单／双 Key 轮换。该纠正说明此前“D1 配置边界通过”只覆盖变量存在而未覆盖正确消费方；P8 运行结论仍保持未完成。

## 3. Arckit SDK 兼容修改

独立仓库 `/Users/crispydog/Workspace/Hoewo/WebProjects/Arckit/arckit` 使用分支 `codex/oms-feedback-v2-host-context`，本地提交 `08591dd feat(feedback-sdk): add bounded host diagnostics`：

- SDK 配置新增闭集 `hostContext` 类型和二次规范化；未知／越界字段丢弃。
- 新建反馈仅将清洗后的内容放入 `metadata.host_context`。
- V2 构建父页面来源模板加入 `https://oatest.antuinfo.cn`。
- 接入文档明确字段和禁止项。

该提交没有部署或推送，也没有把 OMS 项目 Key 写入 Arckit。

## 4. 已执行验证

以下检查在当前工作树实际通过：

```sh
# Gateway
go test ./...
go vet ./...

# Unified Workbench
go test ./...
go vet ./...
node --test internal/server/*.test.mjs frontend/src/*.test.mjs
node --check internal/server/oms_phase1_feedback.js

# 本地真实 Chromium 跨 origin 宿主门禁
PLAYWRIGHT_MODULE_PATH=/absolute/path/to/playwright/index.mjs \
BROWSER_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' \
make p8-feedback-browser-test

# Identity & Access
go test ./...
go vet ./...

# D1 静态交付门禁（不启动容器）
docker compose --env-file deployments/d1/.env.example -f deployments/d1/compose.yaml config --quiet
deployments/d1/ops/test-release-bundle
bash -n deployments/d1/ops/grant-bootstrap-access

# Arckit Feedback SDK
npm test -- --run src/lib/sdk/config.host-context.test.ts src/lib/feedback/v2.agent.test.ts
npm run build:v2:check

git diff --check
```

Workbench Node 回归为 466／466；新增 Feedback 宿主静态门禁为 16／16；本地真实 Chromium 为 3 个场景、29 项断言；Arckit 定向用例为 8／8。浏览器门禁覆盖跨域单 iframe、项目 115 配置形状、诊断 allowlist、精确消息来源、提交／状态、未读与迟到响应、关闭／换票、退出清理、403 不可发现和 503 局部降级，退出时关闭浏览器及两个临时服务。Gateway 测试另覆盖稳定／隔离身份、不安全配置拒绝、Capability 拒绝、响应不泄露身份、Session Key 轮换回退和 Feedback 服务不可用。

## 5. 尚未完成的运行门禁

完成 P8 仍需在真实 SDK／Workshop API 环境中执行：

1. 为项目 115 显式设置服务端通知 allowlist，并在 OMS 部署配置打开通知；
2. 注入项目专用、可撤销、有限流的运行 Key 和独立随机 HMAC Key，确认它们不进入 Git、命令输出、日志或 URL；
3. 两个账号和两个 Tenant 分别提交、查询与读取未读，证明相互不可见；
4. 验证图片附件、开发者回复、只读当前反馈、0 角标隐藏、错误 Key／Project／Origin、SDK／API 停机降级、Key 轮换和退出登录清理；
5. 在真实外部 SDK／项目 115 环境中确认桌面侧栏与移动入口、生产 Origin 白名单、唯一 iframe、Console 零敏感泄露和 OMS 首屏／其他业务不被 Feedback 阻塞；本地 Chromium 夹具已覆盖宿主行为，但不能替代这一项。

以上运行证据取得前，不得把 P8 描述为已经上线或完整通过。P9 全组合、恢复和连续业务验收也尚未开始。
