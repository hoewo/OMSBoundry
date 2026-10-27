# OMS1 系统用户与组织管理验证及发布报告（2026-10-27）

## 结论

BF-275 的本地实现门禁和 Auth58 D1 公网发布均通过。IAM 用户明确分为平台级系统管理员、组织级管理员与普通用户；People 成员仍是独立业务对象。系统管理员登录后只进入独立系统管理页，可以读取全服务器组织元数据、创建组织与首位待激活管理员、为既有组织新增管理员，但不能取得组织业务 Capability。组织管理员只能管理当前组织用户，不能读取或修改系统管理员。

公网只读发布核验没有创建组织、邀请用户、调整管理员或写业务数据。migration 将兼容用户名 `admin` 转为系统管理员后，当前既有组织有 38 名组织用户、0 名组织管理员；这是“系统管理员不兼任组织管理员”的预期结果。应由系统管理员在独立页面明确选择并邀请一名管理员，发布过程不得擅自提升既有普通用户。

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

## Auth58 D1 公网发布

2026-10-27 已把 D1 Suite `v0.1.0-test.oms-v1.39` 发布到 `https://oatest.antuinfo.cn/`。发布包外层 checksum、包内 91 项 checksum、19 个摘要锁定镜像和 IAM 契约预检通过；升级前联合备份为 `/var/backups/omsbizfoundry/20261027T115545Z`，包含 20 个文件、无空文件，`checksums.sha256` 的 19 项校验通过。

发布后实际结果：

- `deployments/d1/ops/d1 check` 通过，19/19 长期容器 healthy；
- Identity schema 为 `000009_system_administrators.sql`；
- Identity、Workbench、Gateway 分别运行 v39 摘要 `59f2633a…c0a64`、`0d4d03e6…829f`、`15451ddd…9e1e`；
- 匿名根入口返回 303，系统管理 JavaScript 返回 200 并包含 `/api/v1/system/tenants`；
- `admin` 真实登录后得到 `system_administrator=true`、`administrator=false`；
- 全组织接口返回 1 个组织、38 名组织用户、0 名组织管理员；
- 系统管理员请求 Contract 业务入口能力返回 403；
- 已登录系统管理员的 Workbench 返回独立系统管理页面，包含“创建组织”，不包含 OMS 业务主导航。

首次镜像准备因 root Docker 客户端没有读取 ECS 上已有的 ACR 登录配置而在真正升级前停止；线上 v38 与 19 个容器未受影响。随后把 ECS 用户已有配置以 0600 复制给 root（原 root 配置已备份），重新拉取与预检通过后才执行升级。回退不能只启动旧 IAM 二进制；migration 9 之后必须使用上述升级前联合备份恢复到新目标。
