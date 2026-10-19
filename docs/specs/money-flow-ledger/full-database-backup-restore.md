# Money Flow Ledger：可验证全量备份与空环境恢复

里程碑：M4  
功能编号：MF-M4-013  
日期：2026-09-20  
状态：已实现并验证

## 目标

运维人员能够在 Money Flow Ledger 在线运行时，从它自己的 PostgreSQL 生成一个可校验的版本化全量备份；发生灾难、迁移或恢复演练时，可以把该归档恢复到同一 PostgreSQL major version 下的全新空数据库，并证明 schema、全部表及代表性业务行为与备份时点一致。

本功能是环境级灾难恢复入口，不是本方主体业务导出、跨客户数据迁移、合并导入、数据库升级器或 Contract Ledger 联合备份。

## 备份边界

备份包含 Money Flow PostgreSQL 中的完整 schema 与运行时事实：

- `schema_migrations`、`app_metadata`；
- 本地用户、密码摘要、失败锁定状态及身份审计；
- Party、Obligation、全部 revision、Adjustment、Remission、Closure；
- Cash Transaction、Cash Reversal、Allocation、Allocation Reversal、Internal Transfer；
- 全部幂等命令及内部 payload hash；
- 全部业务与访问审计；
- 数据库函数、触发器、约束、索引和其他由 migration 建立的对象。

备份明确排除：

- `local_sessions` 的数据；表结构仍随 dump 保留，恢复后必须为空，备份前 Cookie 不得继续有效；
- 数据库连接密码、环境变量、TLS 证书、反向代理、部署脚本和运行日志；
- Contract Ledger、客户 OA 或其他系统的数据库；
- PostgreSQL 集群级角色、权限、tablespace 和其他数据库；
- 尚未进入 Money Flow 的退款、坏账和 Billing Document 事实。

Money Flow 当前没有业务文件目录，因此第一版归档只包含数据库 dump。若将来新增外部不可变对象存储，必须升级备份格式并重新定义跨资源一致性，不能假定 v1 已覆盖。

## 归档契约

第一版格式标识：

```text
omsbizfoundry.money-flow-ledger/full-backup-v1
```

ZIP 根目录必须且只能包含：

```text
manifest.json
database.dump
```

- `database.dump` 是 PostgreSQL custom-format dump；
- ZIP 使用固定根路径、普通文件和 store method，不接受目录、重复路径、路径穿越、加密 entry、软链接或额外文件；
- 最终备份文件权限为 `0600`，输出路径必须尚不存在；工具不覆盖旧归档；
- v1 不内置加密或数字签名，部署环境负责受控存储、传输和静态加密；
- 归档包含密码摘要、身份状态、所有账簿和审计，属于高敏感运维资产。

## manifest

`manifest.json` 使用严格 JSON，未知字段拒绝，至少包含：

- 格式标识和版本；
- UTC RFC3339Nano 生成时刻；
- 来源数据库名称、PostgreSQL server version 和 schema version；
- `database.dump` 的路径、字节数和 SHA-256；
- `local_sessions` 数据已排除的明确标志；
- 来源 snapshot 中每一张 `public` 普通／分区表的名称和精确行数，`local_sessions` 除外。

表计数按名称严格升序、不能重复、不能为负。计数集合不是硬编码的业务子集：它覆盖备份时 schema 中所有 public base table，包括 migration、身份、命令和审计表。这样恢复核验不会只证明几张“主要业务表”存在，却遗漏内部不变量所依赖的数据。

manifest 的 SHA-256 直接覆盖 ZIP 内 `database.dump` 的精确字节。它不是签名、可信时间戳或法律真实性证明。

## 一致快照

1. 工具在源数据库开启只读 `REPEATABLE READ` 事务并调用 `pg_export_snapshot()`；
2. 数据库身份、PostgreSQL version、schema version 和逐表计数都在该事务内读取；
3. `pg_dump --snapshot=<snapshot-id>` 连接同一个数据库和集群，生成 custom dump，同时排除 `public.local_sessions` 的 table data；
4. dump 完成并同步后才提交 snapshot 事务；
5. 工具计算 dump 大小与 SHA-256，构造严格 manifest 和 ZIP，再以不覆盖旧文件的方式发布。

并发提交可以进入下一次备份，但同一次备份的 dump、schema version 和表计数必须来自同一 MVCC 时点。客户端连接到不同数据库、snapshot 失效、dump 失败、摘要失败或最终发布失败时都不能留下看似成功的目标归档。

## PostgreSQL 客户端约束

- `pg_dump` 与源 PostgreSQL server major version 必须一致；
- `pg_restore`、目标 PostgreSQL server 和备份来源 PostgreSQL 的 major version 必须一致；
- Go 工具连接和外部客户端连接可以使用不同网络地址，但必须指向同一个明确数据库；
- 密码通过子进程环境传递，不放进命令参数或日志；
- dump 使用 `--no-owner --no-privileges`，恢复使用 `--single-transaction --exit-on-error --no-owner --no-privileges`。

跨 major PostgreSQL 升级不由本切片隐式处理，必须另做升级与恢复演练。

## 恢复边界

恢复只允许：

- 归档先通过路径、entry 集合、manifest、大小和 SHA-256 完整校验；
- 目标连接与 `pg_restore` 客户端指向同一个明确数据库；
- 目标数据库已经存在，但没有任何用户 relation；
- 不先运行 Money Flow migration，dump 自己恢复 schema 与 migration 记录；
- Money Flow 服务在恢复期间不连接目标数据库。

恢复拒绝非空目标，不提供覆盖、清空、合并、选择性表恢复或 `--force`。归档被解压到受限临时目录；`pg_restore` 失败时单个数据库事务回滚，目标仍应为空。恢复成功后强制复核：

- schema version 与 manifest 相同；
- 除 `local_sessions` 外的 public base table 名称集合与逐表行数完全相同；
- `local_sessions` 行数为 0；
- migration 可以无变化重放；
- 恢复前用户可以重新登录，并能继续读取和写入代表性业务链。

如果 `pg_restore` 已提交但最终核验失败，命令必须返回失败并要求隔离目标；工具不得擅自清空已经提交的数据库。运维人员应保留证据并重新创建空目标后再试。

## CLI

独立命令：

```text
money-flow-ledger-backup create  --output <new-backup.zip>
money-flow-ledger-backup restore --input  <backup.zip>
```

支持：

- `DATABASE_URL`：Go 工具读取源／目标数据库；
- `POSTGRES_CLIENT_DATABASE_URL`：`pg_dump`／`pg_restore` 访问同一数据库，缺省等于 `DATABASE_URL`；
- `PG_DUMP_BIN`、`PG_RESTORE_BIN`：显式客户端路径；
- 命令行参数覆盖上述环境入口。

生产命令必须打印操作结果、schema version 和表数，不打印数据库 URL、密码、manifest 全文或业务内容。

## 验收标准

1. 空配置、非法路径、已存在输出或客户端数据库与 Go 连接数据库不一致时，创建失败且不覆盖任何文件。
2. 归档只包含严格的 `manifest.json` 与 `database.dump`，最终权限为 `0600`。
3. manifest 格式、时间、数据库身份、PostgreSQL version、schema、逐表计数、dump 大小与 SHA-256 全部通过严格校验；未知字段、重复／不安全路径、额外 entry、压缩或加密 entry 拒绝。
4. dump、schema version 和除 session 外的所有 public base table 计数来自同一个只读 `REPEATABLE READ` snapshot；确定性并发测试证明 snapshot 后提交的事实不会只进入其中一侧。
5. `local_sessions` 数据不进入 dump；本地用户密码摘要、锁定状态、命令、payload hash、审计和全部业务事实进入。
6. 目标必须是真正空数据库；已有 migration、表、视图、序列或其他用户 relation 时恢复拒绝且不改变目标。
7. PostgreSQL major 或客户端 major 不一致时拒绝；错误 dump 通过 `pg_restore --single-transaction` 失败后目标仍为空。
8. 完整恢复后 schema、全部非 session 表集合与计数和 manifest 完全一致，session 为 0；migration 重放不改变事实。
9. 恢复前用户能够重新登录；恢复前 Cookie 无效；代表性的 Party、Obligation 全历史、Cash、两类 Reversal、Allocation、Internal Transfer、命令与审计保持一致。
10. 恢复实例能够通过 live／ready，并能继续创建合法的新事实；备份源仍保持可用且不因备份被修改。
11. Contract Ledger 停止时可以创建、恢复和验证 Money Flow 备份；本功能没有跨系统业务流动，因此不制造连接器或联合一致快照。
12. 单元、真实 PostgreSQL、失败回滚、全仓、`-race` 和 `GOWORK=off` 验证通过；测试数据库和临时归档清理归零。

## 明确不做

- 从本方主体 CSV 导出恢复数据库；
- 恢复到非空库、覆盖生产源库、合并两个 Money Flow 实例；
- 选择性 owner、日期、表或记录恢复；
- 跨 PostgreSQL major 的原地升级；
- 定时调度、保留策略、远端上传、KMS 加密或自动灾备切换；
- Contract Ledger 与 Money Flow 的联合备份、分布式 snapshot 或恢复编排；
- 将备份格式承诺为外部稳定业务 API。
