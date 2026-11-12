# OMS 最后使用组织本地验证

日期：2026-11-13

事项：BF-336

状态：本地实现与验证通过，尚未发布

## A → B

问题 A：多组织账号每次登录都先进入固定主组织，用户切换后下次仍要重新切换，并使页面先加载错误组织的数据。

可验证结果 B：组织切换成功后由 IAM 服务端记住该组织；下一次登录直接返回目标 Tenant／Subject，Workbench 以该登录上下文首次拉取数据。偏好不可用时安全回退到仍启用组织。

## 实现与不变量

- Identity schema 13 在 `login_accounts` 增加 `last_used_subject_id`，存量账号从 `primary_subject_id` 回填。
- `(last_used_subject_id, account_id)` 复合外键保证偏好 Subject 属于同一登录账号。
- `CreateSession` 与 `RotateSession` 在各自数据库事务中同时写 Session 和最后使用 Subject；任一步失败都整体回滚。
- 凭据查询先过滤启用 Subject，再按 last-used、primary、稳定创建顺序选择。last-used 已停用时不会阻断登录，成功回退后会自愈指针。
- 浏览器不写 Local／Session Storage；Gateway、People 和业务 Owner 不获得该偏好，不复制 Grant、Person 或业务数据。

## 实际验证

- `go test ./...`：通过。
- `go vet ./...`：通过。
- PostgreSQL 18.6 database integration：schema 12→13 回填、空库迁移和跨组织成员关系通过。
- PostgreSQL 18.6 HTTP integration：第二组织切换后，新客户端重新登录直接进入第二组织；停用该组织成员关系后登录回退 primary 并自愈通过。
- 临时 PostgreSQL 容器在测试结束后已删除。

## 未外推

- 尚未构建或部署 Identity 公网镜像，线上 schema 仍为 12。
- 尚未修改线上账号、组织、Grant 或成员关系。
- 公网真实双组织切换与重新登录必须在发布后使用明确授权账号补证。
