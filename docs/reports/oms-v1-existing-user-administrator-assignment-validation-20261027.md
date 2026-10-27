# OMS1 既有用户管理员挂载验证记录（2026-10-27）

## 结论

BF-276 本地实现、真实 PostgreSQL 18 门禁和公网 D1 Suite `v0.1.0-test.oms-v1.41` 发布均已通过。系统管理员为既有组织设置管理员时，IAM 不再无条件创建新账号：同组织启用中的普通用户保留 Subject ID 并提升为管理员；已经是管理员时幂等返回；只有登录标识不存在时才创建待激活管理员与 24 小时链接。

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

## 公网发布

发布只更新 Identity 与 Workbench 兼容闭包，未修改 schema、业务 Owner 或业务数据。实际结果如下：

- D1 Suite：`v0.1.0-test.oms-v1.41`；
- 升级前联合备份：`/var/backups/omsbizfoundry/20261027T132851Z`；
- Identity 镜像摘要：`sha256:f75c2cf0b4550d686b259c27edcc04e1bf36893db71fd945dd3b4e156360db00`；
- Workbench 镜像摘要：`sha256:c1677ac094cb15bb1c9bdaa94c0986dc7be2c19917bd7b4874e81864c6bd1aa8`；
- schema 9、D1 `check`、19/19 容器 healthy 通过；
- 公网入口返回 200，资源 `oms-system-admin.js?v=20261027.2` 已核对包含标准错误解析、`promoted`／`already_administrator`／`invited` 三动作和“设置组织管理员”文案；
- 使用真实系统管理员登录，只读打开弹窗并确认新说明与“确认管理员身份”按钮可见，没有提交目标账号、生成激活链接或改变管理员数量。

第一次候选 Identity 镜像因运行用户与现网密钥文件权限不匹配，在预检阶段只读失败，没有进入升级或产生任何状态变化；随后按正式 Identity Dockerfile 重新构建 v41，并完成上述完整发布门禁。该失败被保留为部署装配应继续校验运行用户和密钥权限的证据。
