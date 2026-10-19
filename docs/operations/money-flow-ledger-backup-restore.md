# Money Flow Ledger 全量备份与恢复运行手册

日期：2026-09-20  
适用格式：`omsbizfoundry.money-flow-ledger/full-backup-v1`

## 1. 先确认边界

本手册只处理 Money Flow Ledger 自己的 PostgreSQL。它不会备份 Contract Ledger、反向代理、TLS 证书、环境变量、数据库密码、部署脚本或运行日志。

备份 ZIP 含本地用户密码摘要、锁定状态、全部本方账簿、幂等命令、内部 payload hash 和审计，应视为高敏感数据。输出位置必须是受控持久存储，归档权限默认为 `0600`；不要把它放入仓库、Web 静态目录或普通共享盘。v1 不内置加密，异地复制和静态加密由部署环境负责。

本方主体 CSV 导出用于业务移交，不能代替本手册中的全量灾难恢复。

## 2. 运行依赖

- Go 工具能连接源／目标 PostgreSQL；
- `pg_dump`、`pg_restore` 与 PostgreSQL server 使用相同 major version；
- 备份输出路径尚不存在；
- 恢复目标数据库已经创建，但完全没有用户 relation；
- 恢复期间没有 Money Flow 实例连接目标数据库；
- 备份和恢复期间有足够空间同时保存 ZIP 和临时 custom dump。

工具使用两个数据库连接值：

- `DATABASE_URL`：Go 工具自身的连接；
- `POSTGRES_CLIENT_DATABASE_URL`：外部 `pg_dump`／`pg_restore` 访问同一数据库。通常与前者相同；本地 Compose wrapper 在数据库容器中执行客户端，因此网络地址不同。

数据库密码通过客户端子进程环境传递，不出现在命令参数或正常日志中。生产环境不要使用仓库内的本地 wrapper。

## 3. 创建生产备份

在线备份不要求停止服务，但会持有只读 snapshot；应根据实际数据量设置执行窗口、容量和监控：

```bash
cd backend/services/money-flow-ledger

DATABASE_URL='postgres://应用用户:密码@数据库地址:5432/money_flow?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://备份用户:密码@数据库地址:5432/money_flow?sslmode=require' \
go run ./cmd/backup create \
  --output '/srv/money-flow-ledger/backups/money-flow-ledger-20260920T030000Z.zip'
```

成功日志包含 schema version 和计数表数量。工具执行以下门禁：

- 用同一 PostgreSQL snapshot 生成 custom dump、schema version 和全部非 session 表计数；
- 排除 `local_sessions` table data；
- 检查 `pg_dump` 与 server major；
- 记录 dump 字节数和 SHA-256；
- 不覆盖同名文件，最终权限设为 `0600`。

成功后应在部署侧记录命令起止时间、归档文件 SHA-256、大小、复制位置和保留期限。manifest 的 dump 摘要不代替整个 ZIP 的外部摘要。

## 4. 本地 Compose 备份

仓库 wrapper 复用 Compose `db` 容器中的 PostgreSQL 18 客户端和开发凭据，宿主机不需要安装客户端：

```bash
cd backend/services/money-flow-ledger

DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
POSTGRES_CLIENT_DATABASE_URL='postgres://money_flow@127.0.0.1:5432/money_flow?sslmode=disable' \
go run ./cmd/backup create \
  --pg-dump-bin ./ops/local-pg-dump \
  --output '/绝对路径/money-flow-ledger-drill.zip'
```

wrapper 只用于当前应用的本地 Compose 数据库，不适合生产环境。

## 5. 恢复前检查

1. 保存归档副本，不在原文件上修改；
2. 用与来源相同 major version 的 PostgreSQL 创建一个全新数据库；
3. 不先运行 Money Flow migration，dump 会恢复 schema 和 migration 记录；
4. 停止所有将要连接目标库的 Money Flow 实例；
5. 确认恢复账号只会作用于明确的新目标数据库；
6. 确认目标应用配置不会误连生产源库。

恢复工具拒绝已有 migration、表、视图、序列或其他用户 relation 的目标，不提供 `--force`、自动清空、覆盖或合并选项。

## 6. 执行生产恢复

```bash
cd backend/services/money-flow-ledger

DATABASE_URL='postgres://恢复用户:密码@目标数据库地址:5432/money_flow_restored?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://恢复用户:密码@目标数据库地址:5432/money_flow_restored?sslmode=require' \
go run ./cmd/backup restore \
  --input '/srv/money-flow-ledger/backups/money-flow-ledger-20260920T030000Z.zip'
```

工具先严格验证 ZIP、manifest、dump 大小和 SHA-256，再使用：

```text
pg_restore --single-transaction --exit-on-error --no-owner --no-privileges
```

恢复成功后强制核对 schema、完整非 session 表集合与逐表行数，并确认 `local_sessions` 为 0。

- `pg_restore` 失败时，单事务应使目标保持为空；检查并修正环境后可对同一空目标重试；
- 如果数据库已经提交但最终核验失败，命令会失败并要求隔离目标。不要启动应用，也不要擅自清库；保留日志和归档，重新创建空目标后再恢复。

## 7. 本地恢复演练

数据库名称必须是本次演练专用的新目标：

```bash
cd backend/services/money-flow-ledger

docker compose exec -T db \
  createdb -U money_flow -T template0 money_flow_restore_drill

DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow_restore_drill?sslmode=disable' \
POSTGRES_CLIENT_DATABASE_URL='postgres://money_flow@127.0.0.1:5432/money_flow_restore_drill?sslmode=disable' \
go run ./cmd/backup restore \
  --pg-restore-bin ./ops/local-pg-restore \
  --input '/绝对路径/money-flow-ledger-drill.zip'
```

用另一个端口启动恢复实例：

```bash
HTTP_ADDR='127.0.0.1:18083' \
DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow_restore_drill?sslmode=disable' \
SESSION_COOKIE_SECURE=false \
go run ./cmd/server
```

核对健康状态：

```bash
curl --fail http://127.0.0.1:18083/health/live
curl --fail http://127.0.0.1:18083/health/ready
```

还必须完成：

- 用备份前账号重新登录；
- 确认备份前 Cookie 不能解析为恢复后的 session；
- 核对至少一条带 Adjustment／Remission／Closure 的 Obligation 历史；
- 核对 Cash Reversal、Allocation Reversal 和 Internal Transfer；
- 重放一个既有 Command ID，确认仍幂等且不产生第二条事实；
- 创建一条新的合法事实，证明恢复实例可以继续工作；
- 在 Contract Ledger 停止时重复健康和代表性业务检查。

演练结束后，只清理本次明确命名的测试数据库和临时归档；不要使用通配符、工作区根目录或生产路径作为清理目标。

## 8. Release 备份恢复交付验收

构建 release bundle 后，可在隔离的 PostgreSQL 18 测试入口执行 MF-M6-002。该入口只运行归档内的 server、ops 和 backup，并使用真实 `pg_dump`／`pg_restore` 完成在线源备份、独立文件交接、空目标恢复、旧会话拒绝、原账号重登、事实核对、继续写入、非空目标拒绝和损坏归档拒绝：

```bash
cd /仓库根目录

TEST_DATABASE_URL='postgres://测试管理员:密码@127.0.0.1:55433/money_flow?sslmode=disable' \
make money-flow-ledger-backup-restore-acceptance \
  BUNDLE='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CHECKSUM='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

默认使用仓库内 Compose 客户端 wrapper。需要改用独立 PostgreSQL 18 客户端时，向 `make` 进程提供绝对路径环境变量 `DELIVERY_PG_DUMP_BIN` 和 `DELIVERY_PG_RESTORE_BIN`。该验收会创建并删除多个随机测试数据库，`TEST_DATABASE_URL` 对应账号必须具有测试库创建／删除权限；不得指向生产环境。

详细验收契约见 [MF-M6-002 Clean-room 备份交接与空库恢复验收](../specs/money-flow-ledger/clean-room-backup-restore-acceptance.md)。

## 9. 故障判断

| 阶段 | 典型错误 | 处置 |
| --- | --- | --- |
| 备份前 | 客户端不存在或 major 不一致 | 安装与 server 同 major 的客户端，或修正显式 binary 路径 |
| snapshot | `invalid snapshot identifier` | 客户端 URL 指向不同集群／数据库，修正后重新创建备份 |
| 归档发布 | 输出已存在 | 使用新的不可变备份名称，不覆盖旧归档 |
| 恢复预检 | 数据库非空 | 创建明确的新空目标，不绕过门禁 |
| 归档校验 | 路径、大小或摘要不一致 | 隔离损坏归档，从可信副本重新取得 |
| `pg_restore` | SQL、权限或连接失败 | 确认目标仍为空，修正环境后重试 |
| 恢复后核验 | schema、表集合、行数或 session 不一致 | 隔离目标并保留证据；重建空目标后再试 |

## 10. 演练证据和调度

应用只定义如何产生和证明一个可恢复集合，不替部署环境决定备份周期。真实项目应依据可接受数据损失时间和恢复时间确定 RPO／RTO、调度、远端复制、加密与保留策略，并定期完成隔离恢复演练。

一次有效演练记录至少包含：归档 SHA-256、来源／目标 PostgreSQL major、schema version、表计数、创建与恢复命令起止时间、健康检查、登录与旧 session 结果、代表性业务核对、恢复后写入结果和测试目标清理结果。
