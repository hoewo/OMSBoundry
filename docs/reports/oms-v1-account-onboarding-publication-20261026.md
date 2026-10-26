# OMS1 账号开通前后端版本修复发布记录（2026-10-26）

## 问题与根因

用户在公网“账号与租户 → 新增账号”提交手机号账号时看到“账号服务暂时不可用”。公网 Gateway 直接证据显示该请求到达 `/auth/api/v1/admin/invitations` 后返回 400。

根因不是手机号或表单校验，而是 2026-10-26 较早一次发布只更新了 Unified Workbench 和 Payroll：Workbench 已按 BF-272 提交 `login_identifier`，Identity Access 仍运行 `v0.1.0-test.oms-v1.33`，只读取旧字段 `username`。前后端契约版本不一致使有效手机号被旧服务当作空用户名拒绝。

## 修复与发布

- 源码分支：`codex/company-internal-management`
- BF-272 功能提交：`6ae5921`
- 发布源码提交：`e047c970ac503e03f53009b34fe5b6f62f629c41`
- D1 Suite：`v0.1.0-test.oms-v1.38`
- Identity Access 镜像：`sha256:ec81f583eaaf3686d4f9230454da9c1a9690c03f26d1276ed8a937c0f721d890`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261026T145451Z`

本次只补齐 Identity Access 镜像与 migration 8；Payroll、Workbench、Gateway 和其他业务服务继续使用上一发布已经固定的 digest。Identity 新接口同时接受 `login_identifier` 与兼容旧客户端的 `username`，并增加待激活链接重签端点。登录、账号、一次性 Token、Tenant 和审计仍只归 IAM。

## 实际验证

- Identity Access `go test ./...` 与 `go vet ./...` 通过。
- 新镜像以固定 Go／PostgreSQL 基础镜像构建并推送，Registry 核对为 `linux/amd64`。
- 正式 D1 release bundle 外层 SHA-256 与内部 checksum 通过，release bundle 门禁通过。
- 升级前 `config`、`images`、`prepare-images` 和 `preflight-upgrade` 通过；服务器使用本地 digest 镜像，不依赖升级时临时从 Registry 拉取。
- 联合备份 checksum 在升级后重新核对通过；临时 Registry 登录已退出并确认未保留本轮服务器凭据。
- 发布后独立 D1 `check` 通过，19/19 个长期容器 healthy。
- 运行 Identity 镜像 ID 与上述 digest 一致，真实数据库 schema 为 `000008_login_identifiers.sql`。
- 公网 HTTPS 根入口返回登录重定向，Workbench 与 Identity 页面脚本均返回 200，HTTP 根入口按预期 308 到 HTTPS。

为避免向甲方／成员列表写入非人工测试数据，发布后没有代替用户创建账号。真实手机号／邮箱创建、一次性链接交付和本人激活由用户在页面重试；对应服务行为已有单元与系统集成测试覆盖，但这份发布记录不把未实际执行的用户激活旅程写成公网通过。

## 用户复验

刷新 `https://oatest.antuinfo.cn/workbench/?view=oms-v1`，进入“账号与租户 → 新增账号”，重新提交原表单。成功结果应显示一次性激活链接；系统不会虚假宣称已经发送短信或邮件，需要复制链接通过可信渠道交给本人设置密码。
