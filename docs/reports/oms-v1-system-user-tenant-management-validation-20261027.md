# OMS1 系统用户与组织管理本地验证报告（2026-10-27）

## 结论

BF-275 的本地实现门禁通过。IAM 用户明确分为平台级系统管理员、组织级管理员与普通用户；People 成员仍是独立业务对象。系统管理员登录后只进入独立系统管理页，可以读取全服务器组织元数据、创建组织与首位待激活管理员、为既有组织新增管理员，但不能取得组织业务 Capability。组织管理员只能管理当前组织用户，不能读取或修改系统管理员。

该结论不等于公网发布通过。线上 migration、现有 `admin` 迁移、升级前备份、D1 健康与真实浏览器登录必须在发布后单独记录。

## 实现范围

- IAM migration 9 增加互斥的 `system_administrator` 标记，把既有兼容用户名 `admin` 迁移为系统管理员，递增 Session Generation 并清除旧业务 Grant。
- 系统管理 API 提供全组织列表、原子创建组织＋首位管理员邀请、既有组织新增管理员；所有写入要求可信系统管理员 Session 与 CSRF。
- 组织创建在一个数据库事务中建立 Tenant、待激活管理员、24 小时一次性 Token 与审计。
- 组织管理员新增账号可以选择普通用户或管理员；提升／降级会使目标旧 Session 失效，并保护本人自降级和最后一名启用管理员。
- Gateway 删除客户端 `X-OMS-*`，从已验证 IAM Session 派生系统管理员上下文。
- Workbench 系统管理页面与 OMS 业务页面、Owner 脚本和业务导航隔离；弹窗遮罩点击不会关闭表单。

## 实际执行

```sh
(cd backend/platform/identity-access && go test ./... && go vet ./...)
(cd backend/platform/gateway && go test ./... && go vet ./...)
(cd frontend/oms-workbench && go test ./... && go vet ./...)
node --check frontend/oms-workbench/internal/server/oms_phase1_identity.js
node --check frontend/oms-workbench/internal/server/oms_system_admin.js
git diff --check
```

以上命令实际通过。

另以短生命周期 `postgres:18-alpine` 容器设置 `TEST_DATABASE_URL`，实际执行：

```sh
go test -tags=integration ./internal/httpserver \
  -run TestFreshDatabaseIdentityLifecycleAndCredentialIssuance -count=1
```

结果通过。该测试从空数据库重放 migration 到 `000009_system_administrators.sql`，验证系统管理员登录与业务拒绝、全组织列表、系统创建组织、首位管理员激活登录、既有组织管理员邀请、普通用户邀请／重签／激活、管理员调整、Tenant 隔离、Capability、密码重置、最后管理员保护和审计不可变；测试容器在命令退出时停止并删除。

## 发布门禁

本轮必须以 Identity、Gateway、Workbench 三个镜像形成兼容闭包；升级前生成联合备份并验证 checksum。发布后必须确认 schema 9、长期容器全部 healthy、运行镜像 digest、HTTPS 页面与资源、`admin` Session 的 `system_administrator=true / administrator=false`，并只读核对组织列表。除用户明确操作外，发布验证不创建真实组织或用户，不写业务数据。
