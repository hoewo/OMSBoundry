# Contract Ledger：可校验全量备份与空环境恢复

里程碑：M2  
功能编号：CL-M2-011  
日期：2026-09-19  
状态：已实现并验证

## 目标

运维人员能够在 Contract Ledger 正常提供服务时，生成一个同时覆盖 PostgreSQL 事实和本地 Evidence 字节的版本化备份；随后把该备份恢复到一个真正空的 PostgreSQL 数据库和一个尚不存在的 Evidence 根目录，并用数据库计数、schema 版本及每个可用 Evidence 的大小和 SHA-256 验证结果。

这是 Contract Ledger 自身的最小灾难恢复闭环，不建设通用备份平台，不替代 PostgreSQL 的持续归档或基础设施快照，也不连接 Money Flow。

## 一致性边界

- 备份工具开启 `REPEATABLE READ, READ ONLY` 事务并导出 PostgreSQL snapshot；
- schema 版本、事实计数和所有 `content_status = available` 的 Evidence 引用都从该 snapshot 读取；
- `pg_dump` 必须在导出事务仍存活时使用同一个 snapshot，数据库 dump 与引用清单因此处于同一 MVCC 时点；
- 正常 Evidence 上传先把完整字节同步并原子改名到 `objects/`，再提交数据库引用；已经提交的 Version 和 storage key 不可变，应用不会删除其文件；
- 工具只收集 snapshot 中已经存在的可用引用，并在写入归档时重新核对实际大小和 SHA-256；任一文件缺失、不是普通文件或摘要不符时整次备份失败；
- snapshot 之后开始或提交的新上传不进入本次引用清单，留到下一次备份；
- `missing` Evidence Version 只保留在数据库 dump 中，不伪造文件内容。

## 备份格式

格式标识为 `omsbizfoundry.contract-ledger/full-backup-v1`，ZIP 必须且只能包含：

```text
database.dump
manifest.json
evidence/objects/<storage-key 前两位>/<storage-key>  # 每个 available Version 一项
```

- `database.dump` 是当前 PostgreSQL major version 的 custom-format dump；
- dump 使用 `--no-owner --no-privileges`，并排除 `local_sessions` 的表数据；用户和密码摘要属于可恢复身份事实，但已有浏览器会话不得在新环境继续有效；
- manifest 保存格式版本、生成时间、数据库名、PostgreSQL 版本、最新 migration、dump 大小／SHA-256、事实计数，以及每个 Evidence storage key、归档路径、大小和 SHA-256；
- ZIP v1 只接受普通文件和 store method，拒绝目录、软链接、加密项、重复路径、未知路径、路径穿越和 manifest 未声明的 Evidence；
- 最终文件权限为 `0600`，在临时文件完整关闭并同步后才以不覆盖既有目标的方式发布；失败时不得留下可被误认为成功的最终备份；
- 备份包含本地用户密码摘要和业务文件，必须按敏感数据保护，不得提交到 Git 或放入公开下载目录。

## 恢复目标和顺序

- 目标 PostgreSQL 数据库必须已经创建，但不得包含任何用户 schema relation；工具不会清空、覆盖或合并既有数据库；
- 目标 Evidence 根目录必须不存在；工具不会覆盖、合并或猜测既有文件；
- 恢复前完整校验 ZIP 结构、manifest、dump 摘要及全部 Evidence 字节；
- Evidence 先在目标根目录同级的私有临时目录中解包和校验，再原子发布到尚不存在的目标路径；
- `pg_restore` 使用 `--exit-on-error --single-transaction --no-owner --no-privileges`，数据库恢复失败时事务回滚，并且工具只删除带本次恢复标记、由自己刚刚发布的 Evidence 根目录；
- 数据库恢复成功后，工具核对最新 migration、事实计数、`local_sessions = 0`，并逐项核对数据库中的 available Evidence 引用与 manifest；然后移除恢复标记；
- PostgreSQL 与文件系统不能共享一个跨资源事务。若 `pg_restore` 已成功提交但恢复后核验失败，工具不得声称成功，也不得擅自清空目标数据库；目标必须保持隔离，由运维人员重建空数据库和 Evidence 目录后重试。

## PostgreSQL 客户端边界

- `pg_dump` 与 `pg_restore` 是显式运行依赖，major version 不得低于服务器 major version；
- 工具分别接受应用连接 URL 和客户端连接 URL，以支持应用从宿主机连接、客户端在数据库容器内连接同一集群；导出的 snapshot 若连接到了错误集群会被 PostgreSQL 拒绝；
- 密码通过 `PGPASSWORD` 环境传给客户端，不进入命令参数；错误输出不得打印完整数据库 URL；
- 仓库提供只适用于本地 Compose 开发数据库的客户端 wrapper，生产部署必须使用受控安装的 PostgreSQL 客户端及安全凭据注入。

## 恢复后校验

成功至少满足：

1. manifest 格式、数据库名、PostgreSQL 版本、最新 migration 与恢复结果可核对；
2. Contract、Party、Evidence 总数及 available／missing 数量与备份 snapshot 相同；
3. 目标 `local_sessions` 行数为零；
4. 数据库中每个 available storage key 在目标 Evidence 根目录存在，且大小和 SHA-256 与数据库及 manifest 三方一致；
5. manifest 没有数据库未引用的额外 Evidence 文件；
6. 恢复后的应用可以迁移、启动并通过 live／ready；
7. 原有密码可以登录，但备份前浏览器 Cookie 不能恢复会话；
8. Money Flow 停止时仍能完成整个演练。

## 明确不做

- 自动定时、保留周期、远程复制、加密密钥托管、增量备份或 PITR；
- 覆盖或合并现有数据库／Evidence 目录；
- 跨 PostgreSQL major version 的自动升级；
- 仅恢复一部分表、单合同选择性恢复或 Relation 依赖包；
- 为 `missing` Evidence 猜测、下载或重新挂接内容；
- 备份 Money Flow、客户 OA、对象存储或部署密钥；
- 在 Web 管理页面上传或下载全量系统备份。

## 验收标准

1. 在线备份使用同一导出 snapshot 取得数据库 dump 和 Evidence 引用清单。
2. Evidence 缺失、被替换、大小或 SHA-256 不符时不产生最终备份文件。
3. 同名输出不被覆盖，最终归档权限为 `0600`。
4. `local_sessions` 数据不进入 dump，其余 schema 和业务／身份事实完整恢复。
5. 非空数据库、已存在 Evidence 根目录、格式或摘要错误在目标变更前拒绝。
6. `pg_restore` 失败时数据库事务回滚，工具清理且只清理由本次恢复发布的 Evidence。
7. 真实 PostgreSQL 与真实 Evidence 完成一次备份 → 新空数据库／新目录恢复 → 应用健康与文件下载验证。
8. 归档路径穿越、重复、未知、目录、软链接、加密、非 store method 和清单不一致全部拒绝。
9. 客户端缺失或版本／连接错误返回明确运维错误，不泄露密码。
10. 整个实现不引用 Money Flow，也不产生任何跨系统业务事实。
