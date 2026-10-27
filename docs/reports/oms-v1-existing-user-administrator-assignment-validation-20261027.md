# OMS1 既有用户管理员挂载验证记录（2026-10-27）

## 结论

BF-276 本地实现与真实 PostgreSQL 18 门禁通过，公网尚未发布。系统管理员为既有组织设置管理员时，IAM 不再无条件创建新账号：同组织启用中的普通用户保留 Subject ID 并提升为管理员；已经是管理员时幂等返回；只有登录标识不存在时才创建待激活管理员与 24 小时链接。

该修复没有修改 People 成员、职位或业务 Capability，也没有在验证中提升公网真实用户。

## 根因与修复

v39 的 `POST /api/v1/system/administrators` 无论手机号／邮箱是否已存在都会进入邀请创建，因全局登录标识唯一而冲突。Workbench 又只读取顶层 `detail`／`title`，没有读取 IAM 标准错误对象的 `error.message`，因此把真实的“手机号、邮箱或用户名已被使用”掩盖成“系统管理服务暂时不可用”。

修复后的命令返回三个明确动作：

- `promoted`：同 Tenant 启用普通用户原地提升，revision 与 Session Generation 递增，旧 Session 失效并写提升审计；
- `already_administrator`：同一用户已是管理员，无重复写入或审计；
- `invited`：标识不存在，建立待激活管理员并返回一次性 Token。

跨 Tenant、系统管理员账号、pending／disabled 用户稳定拒绝。Workbench 对三个动作分别展示结果，并优先显示 IAM 的标准错误消息。

## 实际验证

以下命令已实际通过：

```sh
(cd backend/platform/identity-access && go test ./... && go vet ./...)
(cd frontend/oms-workbench && go test ./... && go vet ./...)
node --check frontend/oms-workbench/internal/server/oms_system_admin.js
git diff --check
```

另以短生命周期 `postgres:18-alpine` 容器设置 `TEST_DATABASE_URL`，从空数据库重放 migration 9 并执行：

```sh
go test -tags=integration ./internal/httpserver \
  -run TestFreshDatabaseIdentityLifecycleAndCredentialIssuance -count=1
```

结果通过，容器随后停止并删除。集成旅程覆盖既有普通用户提升、相同 Subject 保持、旧 Session Generation 变化、幂等重试、跨 Tenant 拒绝，以及原有无账号邀请与系统管理员边界。

## 公网门禁

发布时只更新 Identity 与 Workbench 兼容闭包。必须先生成并校验联合备份，再验证 schema 9、19/19 healthy、D1 `check`、资源版本、系统管理员真实登录和全组织只读接口。发布验收不代替用户点击，不得擅自把公网普通用户提升为管理员。
