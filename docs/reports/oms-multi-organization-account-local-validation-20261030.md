# 多组织账号本地验证记录

日期：2026-10-30
事项：BF-294
结论：本地实现和 PostgreSQL 18 集成验证通过；未部署公网

## 实现摘要

- Identity schema 12 新增全局 `login_accounts`，存量 Subject 一对一回填账号并保留原稳定 Subject ID。
- `subjects` 增加 `account_id` 和 `(tenant_id, account_id)` 唯一约束；移除 Subject 层的全局登录标识唯一索引。
- 系统创建组织、设置管理员和组织邀请会复用既有激活账号，在目标 Tenant 建立新的 Subject。
- 新增组织列表和 Session 组织切换端口；切换原子撤销旧 Session，Gateway 清除旧 Audience Capability Cookie。
- Workbench 账号页显示组织切换；系统管理和账号邀请区分“既有账号直接加入”与“新账号待激活”。
- People 继续使用既有 `(tenant_id, iam_subject_id)` 唯一索引；补充 PostgreSQL 集成测试证明同 Tenant 唯一、跨 Tenant 允许。

## 实际执行

```sh
(cd backend/platform/identity-access && go test ./...)
(cd backend/platform/gateway && go test ./...)
(cd frontend/oms-workbench && node --check internal/server/oms_phase1_identity.js && node --check internal/server/oms_system_admin.js && go test ./...)
(cd backend/services/people-groups && go test ./...)
```

以上均实际通过。

使用临时 `postgres:18-alpine` 执行：

```sh
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:55439/postgres?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/httpserver ./internal/apikey

TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:55441/postgres?sslmode=disable' \
  go test -count=1 -v -tags=integration ./internal/database \
  -run '^TestSchemaTwelveBackfillsAccountsAndAllowsCrossTenantMembership$'

TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:55440/postgres?sslmode=disable' \
  go test -count=1 -v -tags=integration ./internal/database \
  -run '^TestPersonIAMSubjectLinkIsUniqueInsideTenantOnly$'
```

Identity HTTP／API Key 集成测试、schema 11 存量 Subject 到 schema 12 的回填／重放测试和 People 关联唯一性测试均实际通过；临时容器已停止并由 `--rm` 清理。

## 已验证行为

- 既有账号创建第二组织返回 `joined`，不返回 token；新组织 Subject ID 与原组织不同。
- 账号可读取两个组织，并切换到第二组织；切换后的 Principal 使用第二组织 Tenant／Subject。
- 组织切换清除旧应用 Capability Cookie。
- 同一 Tenant 的第二个 Person 复用同一 Subject 被数据库唯一约束拒绝；另一 Tenant 的独立 Subject 可关联自己的 Person。

## 尚未声明

- 未构建或发布 D1 镜像，未运行公网备份、schema 12 升级、19 容器健康或真实浏览器账号切换。
- 未修改公网账号、组织、成员、Grant 或业务数据。
- 公网发布需另行执行备份、migration 预检、D1 `check`、真实双组织切换与跨 Tenant 资源隔离复验。
