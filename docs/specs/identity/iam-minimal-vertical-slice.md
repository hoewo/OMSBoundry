# M7-002 Identity & Access 最小纵向闭环

日期：2026-09-20  
状态：implemented and verified；2026-10-26 按 BF-272 扩展登录标识与账号开通交互

## 1. 最小根因

> 两个业务应用各自保存账号、密码和会话 → IAM 独占统一组合的登录身份、账号生命周期和 Capability 签发真相。

## 2. 已实现边界

`backend/platform/identity-access` 是独立 Go module、独立 HTTP 进程和独立 PostgreSQL 数据库，拥有：

- 一个安装 Tenant 及其稳定 UUID；
- Subject、手机号／邮箱／兼容用户名登录标识、BCrypt 密码摘要、账号状态、失败锁定和 Session Generation；
- 空库首位管理员 bootstrap，已有账号时不会覆盖；
- 管理员邀请、一次性激活、登录、注销、管理员发起密码重置、重置完成和账号停用；
- SHA-256 保存的 opaque Suite Session，默认绝对有效期 12 小时；
- 从严格解析的目标应用 Capability Manifest 建立租户级授权，不能授予未发布动作；
- 按单一 Audience 签发最长 5 分钟的 Ed25519 Capability Credential；
- 0600 私钥文件、active／retiring 公钥窗口、原子轮换、公钥导出和过期 Key 裁剪；
- 账号、邀请、激活、重置、状态和授权操作的不可变 PostgreSQL 审计。

IAM 不拥有 Person／Group、Project、合同、收支、文件、客户岗位或页面按钮，不从业务数据库读取任何事实。

## 3. 当前 HTTP 入口

| 方法与路径 | 身份要求 | 用途 |
| --- | --- | --- |
| `GET /health/live`、`GET /health/ready` | 无 | 进程与数据库／签名 Key 就绪检查 |
| `GET /api/v1/csrf` | 无 | 建立 IAM 双提交 CSRF Token |
| `POST /api/v1/login`、`POST /api/v1/logout` | CSRF；注销另需 Session | 登录与注销 |
| `POST /api/v1/activate` | CSRF＋一次性凭据 | 完成邀请激活并建立密码 |
| `POST /api/v1/password-reset/complete` | CSRF＋一次性凭据 | 完成密码重置并撤销旧 Session Generation |
| `GET /api/v1/session` | Suite Session | 返回当前可信 Principal |
| `POST /api/v1/credentials` | Suite Session＋CSRF | 为已授予的单一 Audience 签发 Credential |
| `GET /api/v1/.well-known/jwks.json` | 无 | 只发布当前 active／retiring 公钥 |
| `/api/v1/admin/*` | Suite Session＋IAM 管理员＋写入 CSRF | 新增待激活账号、重签激活链接、重置、Subject 列表、状态与 Capability 授予 |

所有 JSON 输入限制为 32 KiB、拒绝未知字段和尾随值。Session Cookie 名为 `oms_suite_session`，使用 `HttpOnly`、`SameSite=Lax`，生产必须启用 `Secure`；IAM CSRF Cookie 独立使用 `SameSite=Strict`。

M7-002 的 Credential 入口返回原始签名 Token 给受信调用方。M7-003 的 Gateway 将代表浏览器完成交换并设置 Audience 专属、Route Prefix 限定的 HttpOnly Cookie，Workbench 不读取 Token。

## 4. 数据与不可变规则

- `tenants`、`subjects`、`suite_sessions`、`activation_tokens`、`capability_grants` 和 `iam_audit_entries` 只存在 IAM 数据库；
- Subject 登录标识全局唯一；手机号去除受支持的展示分隔符和中国区号后规范为 11 位，邮箱与兼容用户名统一小写；
- pending Subject 没有密码摘要；激活才原子写入摘要并转为 active；
- 密码重置、停用账号会递增 Session Generation，旧 Session 即时失效；
- 邀请和重置 Token 只保存 SHA-256，原文仅在建立时返回一次；
- Capability 必须属于 Manifest 的 Audience 命名空间并真实存在；M7 只保存 `tenant` Scope；
- 审计表由数据库 Trigger 禁止更新和删除。

## 5. 密钥运维

`identity-access-keyctl` 提供：

```bash
SIGNING_KEY_FILE=/protected/signing-keys.json identity-access-keyctl public /published/public-keys.json
SIGNING_KEY_FILE=/protected/signing-keys.json identity-access-keyctl rotate
SIGNING_KEY_FILE=/protected/signing-keys.json identity-access-keyctl prune
```

轮换时旧 active Key 进入至少 5 分 30 秒的 retiring 窗口，新凭证只由新 Key 签发；公钥采用临时文件加原子 rename 发布。私钥文件权限宽于 0600 时服务拒绝启动。生产轮换流程为：轮换 → 原子导出公钥集合 → Gateway／目标应用加载并验证新集合 → 重启或刷新 IAM → 超过验证窗口后 prune。M7-003 继续实现验证方加载与攻击测试。

## 6. 已验证结果

- 单元测试验证 Capability Manifest 严格解析、未知能力拒绝、私钥权限、签名、轮换和裁剪；
- 真实 PostgreSQL 隔离库从空库执行 migration 两次，完成 bootstrap、邀请、一次性激活、两次登录、授权、签名凭证、无授权 Audience 拒绝、密码重置、旧 Session 失效、旧密码拒绝、新密码登录、最后管理员保护、账号停用和审计不可变；
- 实际进程首次以 `auto` migration 和 bootstrap 启动并 ready；轮换后以 `verify` migration、无 bootstrap 配置重启，ready 且同时发布一把 retiring 和一把 active 公钥；
- 应用可在 `GOWORK=off` 下独立测试和 `go vet`。

独立 release bundle、整套备份恢复和最终运行手册仍由 M7-006 对实际 D1 发布物统一验收；这不改变 IAM 当前已验证的独立数据库、进程和 migration 边界。
