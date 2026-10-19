# Contract Ledger 全量备份与恢复运行手册

日期：2026-09-19  
适用格式：`omsbizfoundry.contract-ledger/full-backup-v1`

## 1. 先确认边界

本手册处理 Contract Ledger 自己的 PostgreSQL 和本地 Evidence 目录。它不会备份 Money Flow、反向代理、TLS 证书、环境变量、数据库密码、部署脚本或远程对象存储。

备份 ZIP 含本地用户密码摘要、完整业务数据库和 Evidence 文件，应视为高敏感数据。输出位置必须是受控持久存储，权限默认是 `0600`；不要把归档放入仓库、Web 静态目录或普通共享盘。v1 不内置归档加密，异地复制和静态加密由部署环境负责。

## 2. 运行依赖

- 应用能连接源／目标 PostgreSQL；
- `pg_dump`、`pg_restore` 与 PostgreSQL 服务器使用相同 major version；
- 备份期间 Evidence 根目录可读，恢复期间目标 Evidence 上级目录可写；
- 备份输出路径尚不存在；
- 恢复目标数据库已经创建但完全没有用户 relation，目标 Evidence 根目录尚不存在；
- 备份和恢复期间有足够空间同时存放 ZIP、临时 dump 和 Evidence 暂存副本。

工具使用两个数据库连接值：

- `DATABASE_URL`：Go 工具自身连接数据库；
- `POSTGRES_CLIENT_DATABASE_URL`：外部 `pg_dump/pg_restore` 进程连接同一个集群和数据库。通常与前者相同；本地 Compose wrapper 在数据库容器内执行客户端，因此网络地址不同。

密码通过客户端进程环境传递，不进入 `pg_dump/pg_restore` 参数。生产环境不要使用仓库内的本地 wrapper。

## 3. 创建生产备份

先确认当前 Evidence 配置和可用空间，再执行：

```bash
cd backend/services/contract-ledger

DATABASE_URL='postgres://应用用户:密码@数据库地址:5432/contract_ledger?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://备份用户:密码@数据库地址:5432/contract_ledger?sslmode=require' \
EVIDENCE_STORAGE_DIR='/srv/contract-ledger/evidence' \
go run ./cmd/backup create \
  --output '/srv/contract-ledger/backups/contract-ledger-20260919T120000Z.zip'
```

成功输出包含 schema 版本、Contract 数和 available Evidence 数。以下任一情况都必须视为失败，且不能把临时文件当成备份：

- `pg_dump` 不存在、major version 不一致或 snapshot 连接到错误集群；
- 任一 referenced Evidence 缺失、不是普通文件、大小或 SHA-256 不一致；
- 输出路径已经存在；
- ZIP 写入、同步或最终发布失败。

工具不会覆盖同名备份。备份完成后应把命令成功日志、文件大小、归档 SHA-256、复制位置和保留期限记入部署侧运维记录；归档自身 manifest 已包含内部 dump 和 Evidence 摘要。

## 4. 本地 Compose 备份

宿主机不需要安装 PostgreSQL 客户端；仓库 wrapper 只用于 Compose 中的开发数据库：

```bash
cd backend/services/contract-ledger

DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
POSTGRES_CLIENT_DATABASE_URL='postgres://contract_ledger@127.0.0.1:5432/contract_ledger?sslmode=disable' \
EVIDENCE_STORAGE_DIR='.data/evidence' \
go run ./cmd/backup create \
  --pg-dump-bin ./ops/local-pg-dump \
  --output '/绝对路径/contract-ledger-drill.zip'
```

wrapper 复用 Compose `db` 容器中的 PostgreSQL 18 客户端和开发凭据，不适合生产环境。

## 5. 恢复前检查

1. 停止将要连接目标数据库／Evidence 目录的 Contract Ledger 实例；
2. 保存归档副本，不在原文件上修改；
3. 用与备份相同 major version 的 PostgreSQL 创建一个全新数据库；
4. 不先运行应用 migration；dump 会恢复 schema 和 migration 记录；
5. 选择一个尚不存在的 Evidence 根路径；不要指向旧目录、空壳 `objects/` 或其他应用目录；
6. 确认恢复账号只能作用于明确的目标数据库；
7. 确认目标应用配置不会误连生产源库。

恢复工具拒绝非空数据库和已存在 Evidence 根目录，不提供 `--force`、清空或合并选项。

## 6. 执行生产恢复

```bash
cd backend/services/contract-ledger

DATABASE_URL='postgres://恢复用户:密码@目标数据库地址:5432/contract_ledger_restored?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://恢复用户:密码@目标数据库地址:5432/contract_ledger_restored?sslmode=require' \
EVIDENCE_STORAGE_DIR='/srv/contract-ledger-restored/evidence' \
go run ./cmd/backup restore \
  --input '/srv/contract-ledger/backups/contract-ledger-20260919T120000Z.zip'
```

工具先验证完整归档，再暂存 Evidence，随后原子发布目录并使用 `pg_restore --single-transaction` 恢复数据库：

- `pg_restore` 失败时数据库事务回滚，工具只回收带本次随机标记、由自己发布的 Evidence 目录；
- 恢复成功后工具核对 migration、事实计数、旧会话为零和每个 available Evidence；
- 如果数据库已经提交但最终核验失败，命令返回失败并要求隔离目标。不要启动应用，也不要在该目标上重试；重新创建空数据库和新 Evidence 路径后再恢复。

## 7. 本地恢复演练

以下数据库名和 Evidence 路径必须是本次演练专用的新目标：

```bash
cd backend/services/contract-ledger

docker compose exec -T db \
  createdb -U contract_ledger -T template0 contract_ledger_restore_drill

DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger_restore_drill?sslmode=disable' \
POSTGRES_CLIENT_DATABASE_URL='postgres://contract_ledger@127.0.0.1:5432/contract_ledger_restore_drill?sslmode=disable' \
EVIDENCE_STORAGE_DIR='/绝对的新目录/contract-ledger-restore-drill/evidence' \
go run ./cmd/backup restore \
  --pg-restore-bin ./ops/local-pg-restore \
  --input '/绝对路径/contract-ledger-drill.zip'
```

用另一个端口启动恢复实例：

```bash
HTTP_ADDR='127.0.0.1:18081' \
DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger_restore_drill?sslmode=disable' \
EVIDENCE_STORAGE_DIR='/绝对的新目录/contract-ledger-restore-drill/evidence' \
SESSION_COOKIE_SECURE=false \
go run ./cmd/server
```

核对：

```bash
curl --fail http://127.0.0.1:18081/health/live
curl --fail http://127.0.0.1:18081/health/ready
```

还必须人工完成：

- 用备份前账号重新登录；备份前 Cookie 不应直接恢复会话；
- 核对 Contract／Party 数和一份代表性合同历史；
- 下载至少一个 available Evidence，并核对内容；
- 确认备份前为 `missing` 的 Evidence 仍明确显示缺失；
- 在 Money Flow 停止时完成上述操作。

演练完成后，仅清理本次明确命名的测试数据库和测试 Evidence 路径；不要把通配符、工作区根目录或生产路径用于清理命令。

## 8. Release Clean-room 交付验收

构建 release bundle 后，可在隔离的 PostgreSQL 18 测试入口执行 CL-M6-003。该入口只运行归档内 server、ops 和 backup，并使用真实本地 Evidence、`pg_dump` 与 `pg_restore` 完成空库安装、合同草稿、Evidence 上传下载、在线联合备份、独立交接、空目标恢复、旧会话拒绝、原账号重登、恢复后下载和继续写入：

```bash
cd /仓库根目录

TEST_DATABASE_URL='postgres://测试管理员:密码@127.0.0.1:55432/contract_ledger?sslmode=disable' \
make contract-ledger-clean-install-backup-restore-acceptance \
  BUNDLE='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CHECKSUM='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

默认使用仓库内 Contract Compose 客户端 wrapper。需要改用独立 PostgreSQL 客户端时，向 `make` 进程提供绝对路径环境变量 `DELIVERY_PG_DUMP_BIN` 和 `DELIVERY_PG_RESTORE_BIN`。该验收会创建并删除多个随机测试数据库和临时 Evidence 目录，`TEST_DATABASE_URL` 对应账号必须具有测试库创建／删除权限；不得指向生产环境。

失败门禁还覆盖非空数据库、已存在 Evidence 根和 Evidence 字节摘要不一致。详细验收契约见 [CL-M6-003 Clean-room Release、Evidence 与恢复验收](../specs/contract-ledger/clean-room-release-backup-restore-acceptance.md)。

## 9. 故障判断

| 阶段 | 典型错误 | 处置 |
| --- | --- | --- |
| 备份前 | 客户端不存在或 major 不一致 | 安装与服务器同 major 的客户端，或修正显式 binary 路径 |
| snapshot | `invalid snapshot identifier` | 客户端 URL 连接了不同数据库集群／数据库，修正后重新生成 |
| Evidence 收集 | 文件缺失或摘要不符 | 停止把当前集合视为可备份，先调查存储完整性 |
| 归档发布 | 输出已存在 | 使用新的不可变备份名称，不覆盖旧归档 |
| 恢复预检 | 数据库非空或 Evidence 已存在 | 新建明确的空目标，不使用 `--force` 思路绕过 |
| `pg_restore` | SQL、权限或连接失败 | 确认数据库仍为空、Evidence 已由工具回收，再修正环境重试 |
| 恢复后核验 | 计数、schema 或 Evidence 不一致 | 隔离目标，保留日志和归档用于调查；重建空目标后重试 |

## 10. 建议演练频率与证据

应用只定义“如何生成和验证一个可恢复集合”，不替部署环境决定时间表。真实项目至少应根据可接受数据损失时间确定备份频率，并定期在隔离环境执行完整恢复演练。一次有效演练记录应包含：归档 SHA-256、源／目标 PostgreSQL major、schema 版本、事实计数、Evidence 数、命令起止时间、健康检查结果、抽样登录／下载结果和清理结果。
