# OMS1 组织显示名称设置验证记录（2026-10-27）

## 结论

BF-277 本地实现与真实 PostgreSQL 18 门禁通过，公网尚未发布。组织管理员现在可以从“账号与租户 → 组织设置”修改本人所属组织的 IAM 显示名称；组织稳定 ID、账号归属和业务事实引用均不改变。

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

## 公网门禁

候选发布只应更新 Identity 与 Workbench 兼容闭包。发布前必须生成并校验联合备份；发布后验证 schema 9、D1 `check`、19/19 healthy、资源版本、真实管理员打开“组织设置”和只读读回。发布验收不得替用户修改线上组织名称。
