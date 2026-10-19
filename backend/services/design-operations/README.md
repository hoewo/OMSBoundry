# Design Operations

Design Operations 是 OA Demo 的设计生产兼容入口，也是目标 Production Operations 的当前运行实现。它拥有 Production Work Item、版本化类型定义、阶段进度、Evidence 成果版本引用和不可变历史，不拥有 Project、人员名册或文件对象。旧 `/api/v1/design-work-items` 仍固定代理 `design-v1`；canonical `/api/v1/production-work-items` 使用独立 `production-operations` audience 与 `production-work-item.*` Capability。`consultation-v1` 已进入同一领域与数据库内核，并已通过双类型生命周期、失败恢复、历史隔离和 canonical 权限本地真库验证；正式入口签发／撤权和完整组合尚待验证。

## 本地启动

```bash
docker compose up -d postgres
cp .env.example .env
# 替换所有 replace-with-* 值，并确保 Project Workspace 与 Evidence Service 已启动。
set -a; source .env; set +a
go run ./cmd/server
```

- HTTP：`http://127.0.0.1:8087`
- PostgreSQL：`127.0.0.1:55438`
- 健康检查：`/health/live`、`/health/ready`
- API 前缀：`/api/v1/design-work-items`

本地模板 IAM 仍执行 tenant、action 和 resource scope 鉴权。Project、Assignment 与 Evidence 只通过各自的 reference-read API 在线校验；Design 数据库不复制这些系统的内部事实。

## 验证

资料来源切片：C4 成果不能进入项目资料 → 发布单个 Work Item 当前有效的 File ID／Version 只读投影。canonical `.../references/production-work-items/{id}/materials` 使用既有原生 reference.read／精确 Work Item（或 Tenant）Scope；不改生产写入、schema、旧 reference 响应或下载权限，不向外部事实源发请求。owned-details 用只读 repeatable-read 保持同快照。单元、HTTP、race、隔离真 PG18／真实 IAM8 签发及零读取写入已通过，主站尚未部署。见 [规格](../../../docs/specs/assembly/production-material-reference-v1.md)和[来源实测](../../../docs/reports/auth58-production-material-source-validation-20261001.md)；来源通过不能当作完整 C8 通过。

```bash
go test ./...
go vet ./...
go build ./...

TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  go test -p=1 -count=1 -tags=integration ./...
```

## 发布、备份与恢复

```bash
GOOS=linux GOARCH=amd64 ./ops/build-release v0.1.0 /absolute/output
./ops/local-pg-dump --format=custom --no-owner --no-privileges > /tmp/design-operations.dump
PGDATABASE=design_operations_restore ./ops/local-pg-restore \
  --exit-on-error --no-owner --no-privileges --dbname=design_operations_restore < /tmp/design-operations.dump
```

发布包不携带 `.env`、凭证或数据库内容；迁移到公司 Ubuntu 时重新注入数据库、IAM 和引用端点即可，Work Item ID、revision、成果引用与历史不变。
