# Contract Ledger：独立安装与升级交付

里程碑：M2  
功能编号：CL-M2-012  
日期：2026-09-19  
状态：已实现并验证

## 目标

把 Contract Ledger 从“仓库源码可以运行”收敛为一个可识别版本、可校验、可在新环境独立安装、可对既有数据库显式升级并能说明失败恢复边界的交付物。安装和升级不得要求 Money Flow、共享 IAM、客户 OA、项目系统或通用部署平台。

第一版交付面向一个受控主机上的独立进程和 PostgreSQL 18，不预设 systemd、Kubernetes、云厂商或客户网络拓扑。

## 交付物

一个 release bundle 必须包含：

```text
bin/contract-ledger-server
bin/contract-ledger-ops
bin/contract-ledger-backup
config/contract-ledger.env.example
docs/INSTALL-UPGRADE.md
docs/BACKUP-RESTORE.md
```

- 三个二进制都脱离根 `go.work` 构建，使用 `CGO_ENABLED=0` 和 `-trimpath`；
- bundle 名称显式包含 release version、目标 GOOS 和 GOARCH；
- 三个二进制嵌入同一 version、commit 和 build time，并提供不连接数据库的 `version`／`--version` 输出；
- bundle 生成同名 SHA-256 文件，构建命令拒绝覆盖已有归档或摘要；
- migration 嵌入 server／ops 二进制，安装时不依赖仓库源码；backup 二进制继续承担 CL-M2-011 的全量备份恢复；
- 示例配置和两份运行手册随 bundle 一起交付，但真实密码、Cookie、证书和备份归档不得进入 bundle。

## Schema 状态模型

运维工具和服务使用同一份嵌入 migration 清单，把数据库归为：

- `empty`：没有 `schema_migrations` 且没有用户 relation，可执行首次 migration；
- `pending`：已应用版本是嵌入清单的严格前缀，存在待执行 migration；
- `current`：已应用版本与嵌入清单完全一致；
- `incompatible`：包含二进制未知 migration、版本缺口、乱序状态，或没有 migration 记录却已有用户 relation。

规则：

- PostgreSQL major 必须为 18；
- `ops check` 只检查连接、PostgreSQL major、schema 状态和 Evidence 存储可写性；默认允许合法 `empty`／`pending` 并明确报告 `server_ready=false`；
- `ops check --require-current` 只接受 `current`，适合启动前和升级后门禁；
- `ops migrate` 只接受 `empty`／`pending`／`current`，对全部 pending migration 使用同一个 PostgreSQL 事务和 advisory transaction lock；任一 migration 失败时本次尚未提交的 migration 全部回滚；
- `incompatible` 数据库不得自动修复、补洞或启动服务；需要调查或从可信备份恢复；
- migration 继续是仅前向文件，必须能在普通 PostgreSQL 事务中运行；需要 `CONCURRENTLY` 等非事务操作时必须另立决策和停机方案。

## 服务启动模式

新增 `MIGRATION_MODE`：

- `auto`：默认，仅为当前本地开发兼容；启动时执行合法 pending migration，再要求 schema 为 current；
- `verify`：正式安装和升级使用；启动时绝不修改 schema，只接受 current，否则在监听端口前失败。

无论哪种模式，服务都必须在监听前验证 PostgreSQL 18、schema 状态和 Evidence 存储可写。启动日志包含 build version、commit、migration mode 和当前 schema version，但不输出数据库 URL、密码或 bootstrap 密码。

## 新环境安装流程

1. 校验 bundle SHA-256，并用三个二进制的版本输出核对同一 build identity；
2. 准备 Contract Ledger 独占的 PostgreSQL 18 数据库、Evidence 持久目录和最小权限配置；
3. 使用一次性 bootstrap 管理员凭据执行 `ops check`，确认数据库 `empty` 且 Evidence 可写；
4. 执行 `ops migrate`，再执行 `ops check --require-current`；
5. 以 `MIGRATION_MODE=verify` 启动 server，首次启动原子建立管理员；
6. 移除运行环境中的 bootstrap 密码，重启并确认不会覆盖已有账号；
7. 验证 live／ready、登录、基础写入和 Evidence 上传下载；
8. 确认 Money Flow 未安装或停止时完整流程仍成功。

## 升级流程

1. 保持旧服务运行，用新 bundle 的 `ops check` 读取目标数据库，确认仅为合法 pending 或 current；
2. 使用 CL-M2-011 生成升级前 PostgreSQL／Evidence 全量备份并记录外部 SHA-256；
3. 停止旧 server，确认没有业务写入进程；
4. 执行新 bundle 的 `ops migrate`；
5. 执行 `ops check --require-current`；
6. 以新 server、`MIGRATION_MODE=verify` 启动；
7. 验证 health、登录、代表性合同历史和 Evidence 下载；
8. 成功后保留升级前备份，按部署侧策略观察和归档记录。

## 失败与回退边界

- bundle 校验、预检或升级前备份失败：不停止旧服务，不执行 migration；
- 停服后但 migration 尚未开始：可以重新启动旧 server；
- `ops migrate` 事务失败：本次 pending migrations 整体回滚；修正原因后可以重试，旧 server 仍只能在它认可 schema 时启动；
- migration 已成功但新 server／业务验收失败：不提供 down migration，也不让旧二进制冒险连接未知 schema；优先修复并前滚；必须回退时，把升级前 CL-M2-011 备份恢复到新的空数据库和新 Evidence 目录，再让旧 bundle 指向恢复目标；
- 不清空或覆盖升级失败的目标，保留其日志和状态用于调查；切换、DNS、反向代理和进程监督属于部署环境责任。

## 安全与配置

- release bundle 不含真实 `.env`、数据库密码、bootstrap 密码、TLS 私钥或备份文件；
- 运行配置文件建议 `0600`，Evidence 根目录和备份目录由专用操作系统账号拥有；
- 正式 HTTPS 必须使用 `SESSION_COOKIE_SECURE=true`；反向代理和证书终止不由应用内置；
- bootstrap 用户名和密码必须成对出现，只在空用户库首次启动使用；已有用户时不会覆盖，但部署完成后仍要从环境移除；
- 运维输出不得打印数据库 URL 或密码。

## 明确不做

- Kubernetes／Helm、云托管模板、通用安装器、自动滚动升级或多节点编排；
- 自动创建生产数据库账号、TLS 证书、DNS、反向代理或操作系统服务；
- down migration、原地清库、自动切换数据库或自动恢复备份；
- 跨 PostgreSQL major 自动升级；
- 把 Money Flow、连接器、共享 IAM 或客户专用配置塞入 Contract Ledger bundle；
- 宣称 bit-for-bit 可复现构建；当前只要求确定内容、嵌入 build identity、去除本地路径并提供 SHA-256。

## 验收标准

1. build 脚本生成不覆盖的版本化 bundle 和 SHA-256，内容只有三二进制、示例配置和两份文档。
2. 三个二进制脱离源码即可输出完全一致的 version／commit／build time。
3. 空数据库经 `check → migrate → require-current → server` 可用，首次管理员只建立一次。
4. `MIGRATION_MODE=verify` 对 empty／pending／unknown／gap schema 都在监听前失败。
5. `MIGRATION_MODE=auto` 只迁移合法前缀；unknown、gap 或非托管已有 relation 不自动修改。
6. 全部 pending migration 在一个事务中执行；故意失败时不留下部分新 migration。
7. 从历史合法 schema 升级到当前 schema 后，既有合同、身份、审计和 Evidence 元数据保持有效。
8. 新 bundle 升级前可以读取 pending 状态，升级后报告 current；旧／不兼容二进制边界有明确证据。
9. 从实际 release bundle 启动恢复／新装实例并通过 live、ready、登录和最小业务／Evidence 验收。
10. Money Flow 停止或不存在时完成全部流程，没有跨系统表、调用或交付依赖。
