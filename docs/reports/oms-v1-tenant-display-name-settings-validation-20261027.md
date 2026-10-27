# OMS1 组织显示名称设置验证记录（2026-10-27）

## 结论

BF-277 本地实现、真实 PostgreSQL 18 门禁和 Auth58 D1 公网发布均已通过。组织管理员现在可以从“账号与租户 → 组织设置”修改本人所属组织的 IAM 显示名称；组织稳定 ID、账号归属和业务事实引用均不改变。

该功能只维护登录与工作台显示资料。营业执照名称、统一社会信用代码、法定代表人等企业主体事实继续归 Party Directory，不因本功能复制到 IAM。

## 实现边界

- IAM 提供 `POST /api/v1/admin/tenant`，请求只接受 `name`，不接受客户端传入 Tenant ID；
- 服务从可信 Session Principal 取得 Tenant，只有非系统级的组织管理员可以调用；
- 名称去除首尾空白后必须为 1～100 个字符；
- PostgreSQL 在事务中以 `FOR UPDATE` 锁定 Tenant 行，真实变化写 `iam_tenant_name_changed` 不可变审计，审计包含变更前后名称；
- 相同名称重复保存视为幂等成功，不产生额外审计；
- Workbench 提供“组织设置”入口，显示可编辑名称和只读稳定 ID，并读取 IAM 标准 `{error:{message}}` 错误。

## 实际验证

以下命令已实际通过：

```sh
(cd backend/platform/identity-access && go test ./... && go vet ./...)
(cd frontend/oms-workbench && go test ./... && go vet ./...)
node --check frontend/oms-workbench/internal/server/oms_phase1_identity.js
git diff --check
```

另启动限 1 CPU／512 MiB 的短生命周期 `postgres:18.6-alpine`，从空数据库重放 Identity schema 9 后运行：

```sh
go test -tags=integration ./internal/httpserver \
  -run TestFreshDatabaseIdentityLifecycleAndCredentialIssuance -count=1
```

集成旅程实际覆盖组织管理员改名、首尾空白规范化、GET 立即读回、稳定 Tenant ID、单条不可变审计、空名称 422、普通用户 403 和系统管理员 403。测试结束后临时容器已停止并删除。

## 公网发布与复验

发布使用 D1 Suite `v0.1.0-test.oms-v1.42`，目标镜像为：

- Identity：`v0.1.0-test.oms-v1.42@sha256:58deffefec42f7121f44d9d345f347a00922c4f8741564dd3e4b4c8d6ba63afe`；
- Workbench：`v0.1.0-test.oms-v1.42@sha256:c69118560d44cf23b1d77045598fdec851a2df1c0f5c935dee5e3d4e9e1994ce`。

实际执行并通过：

1. 使用受保护 D1 环境生成升级前联合备份并校验 checksum；
2. 执行正式 `preflight-upgrade`、`upgrade` 与 `check`；
3. Identity schema 保持当前 `000009_system_administrators.sql`，无业务 schema 变化；
4. 升级后 19 个容器全部运行，`UNHEALTHY=0`，Identity 与 Workbench 均为 v42 且 healthy；
5. 真实管理员登录 `https://oatest.antuinfo.cn/workbench/?view=oms-v1`，进入“账号与租户”，确认“组织设置”入口可见；
6. 打开弹窗确认当前名称、只读组织稳定 ID、Party Directory 边界说明和“保存设置”动作正确，随后直接关闭，现有组织名称未被修改。

ECS 证据目录为 `/opt/omsbizfoundry/acceptance/oms-v1-tenant-settings-v42-20261027`，包含 `preflight.log`、`upgrade.log` 与 `check.log`。受保护运行环境记录的联合备份根目录为 `/var/backups/omsbizfoundry`。

远端现存正式运维包版本较旧，两个 People 辅助服务缺少镜像 pin。发布前先确认它们是仅有的未解析镜像，并以当前已运行的 People v33 精确摘要写入兼容 override；没有替换 People、其他 Owner 或业务数据。该兼容项是远端运维包清单缺口，不改变 BF-277 的 Identity／Workbench 发布范围。
