# People & Groups

独立管理 Person、Group、Membership、组能力与不可变 Roster Snapshot 的 Go 应用。

最小根因：人员和团队关系散落、无法被其他应用稳定引用 → 统一管理人员、组与成员关系，并发布可校验的名册快照。

## 独立运行

1. 复制 `.env.example` 并替换数据库密码、UUID 与模板访问令牌；
2. 启动 PostgreSQL：`docker compose up -d postgres`；
3. 导入环境后运行 `go run ./cmd/server`；
4. 打开 `http://127.0.0.1:8084/login`，使用配置中的模板访问令牌进入。

正式组合使用 `IDENTITY_PROVIDER=iam`，经 Nginx 与 Gateway 从 `/people/` 访问。应用不保存 IAM 密码、Session 或角色授权。
