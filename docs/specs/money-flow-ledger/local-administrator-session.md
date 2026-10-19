# Money Flow Ledger：首位管理员与本地登录会话

里程碑：M3  
功能编号：MF-M3-001  
日期：2026-09-19  
状态：已实现并验证

## 目标

Money Flow Ledger 在写入第一笔主体、义务或资金事实前，先建立独立于 Contract Ledger、共享 IAM 和客户 OA 的最小可信操作者边界。空用户库通过显式配置建立唯一首位管理员；用户可以登录、访问受保护页面并注销，后续业务写入只能从已验证会话取得 Actor。

本功能只建立首位管理员和登录会话，不提前建设完整用户管理后台，也不创建任何经营收支业务事实。

## 业务事实

- `Local User` 是 Money Flow Ledger 自己拥有的登录身份，不等同于往来 Party；
- 首位用户固定为 active administrator，由空用户库上的一次性 bootstrap 配置建立；
- username 经过去空白和小写标准化，必须匹配 `^[a-z0-9][a-z0-9._-]{2,63}$`；
- display name 去除首尾空白，长度为 1 至 100；省略时使用标准化 username；
- 密码长度按 UTF-8 字节计为 12 至 72，只保存 BCrypt 摘要；
- 登录会话使用高熵随机令牌，客户端只持有原令牌，数据库只保存 SHA-256；
- 会话固定引用创建时的 `session_generation`，用户失效或以后修改密码时可以统一使旧会话失效；
- 连续 5 次失败登录令已知用户锁定 15 分钟；未知用户也执行虚拟 BCrypt 比较，不泄露用户名是否存在；
- bootstrap 用户和对应审计在同一事务创建；已存在任意用户时，后续 bootstrap 配置不覆盖用户名、密码或资料；
- `Local User`、`Session` 和审计均只存在于 Money Flow 数据库。

## 用户流程

```text
空用户库 + 显式 bootstrap 配置
  → 服务启动并原子建立首位管理员
  → 打开受保护页面时跳转登录
  → 提交用户名、密码和 CSRF token
  → 服务验证凭据并建立会话
  → 返回 Money Flow 首页并显示可信 Principal
  → POST 注销，服务端撤销会话并清除 Cookie
```

## 安全与运行规则

- 空用户库缺少 bootstrap 用户名或密码时，服务必须在监听端口前失败；
- `BOOTSTRAP_ADMIN_USERNAME` 与 `BOOTSTRAP_ADMIN_PASSWORD` 必须成对出现；display name 可选；
- `SESSION_TTL` 必须为正时长，默认 12 小时；
- HTTPS 部署必须使用 `SESSION_COOKIE_SECURE=true`；本地 HTTP 才允许 false；
- Session Cookie 使用 `HttpOnly`、`SameSite=Lax`；CSRF Cookie 使用 `HttpOnly`、`SameSite=Strict`；
- 所有状态变更只接受受限大小的 POST，并验证双提交 CSRF token；
- GET `/health/live`、GET `/health/ready` 和 GET/POST `/login` 公开，其余页面先通过 session 验证；
- 登录成功不写业务审计；首位管理员的形成属于稳定身份事实，必须写不可变审计；
- 服务日志、页面和错误响应不得输出密码、原始 session token、密码摘要或数据库 URL。

## 数据与事务不变量

1. 空用户库并发 bootstrap 最多形成一名用户和一条 `local_user_bootstrapped` 审计。
2. 用户名不区分大小写唯一；原始大小写和空白不会成为第二身份。
3. bootstrap 用户、密码摘要和审计要么全部提交，要么全部不存在。
4. 数据库中不保存原始密码或 session token。
5. 被撤销、过期、generation 不匹配或用户非 active 的会话不能解析为 Principal。
6. 审计条目禁止 UPDATE 和 DELETE。
7. HTTP 请求中的用户名、隐藏字段或查询参数不能伪造已登录 Principal。

## 验收标准

1. 空数据库在合规 bootstrap 配置下只建立一名 administrator，重复启动不改变既有用户。
2. 空用户库缺少 bootstrap 配置、配置只给一半或密码不合规时在监听前失败。
3. 正确用户名密码可以登录，错误、未知、停用或锁定用户得到同一认证错误。
4. 登录前访问首页跳转到安全的本地登录路径，外部 `next` URL 不能形成开放重定向。
5. 登录后首页显示服务端 Principal；原始 session token 不出现在数据库。
6. 注销需要 CSRF，成功后服务端会话被撤销，旧 Cookie 不能继续访问。
7. bootstrap 写审计失败时用户回滚；审计直接 UPDATE／DELETE 被 PostgreSQL 拒绝。
8. migration 可重复执行，约束、索引和不可变触发器存在。
9. 单元测试、HTTP 测试、真实 PostgreSQL 集成和独立启动联调通过。
10. Contract Ledger 停止或不存在时完成全部流程，Money Flow 不读取其数据库、Cookie、用户或权限。

## 明确不做

- 创建其他用户、修改角色、停用账号、重置或修改密码；这些在需要第二名真实用户前单独实现；
- 外部 SSO、OAuth、OIDC、LDAP、共享 IAM 或跨系统 Cookie；
- Party 与 Local User 绑定；
- 审批、客户岗位、组织层级或数据行级权限；
- Contract Ledger 用户迁移或账号同步；
- 义务、票据、资金、核销或业务审计页面。
