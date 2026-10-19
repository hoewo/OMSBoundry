# Contract Ledger 独立安装与升级运行手册

日期：2026-09-19  
适用功能：CL-M2-012

## 1. 交付边界

Contract Ledger release bundle 是一个独立交付单元，包含：

- `contract-ledger-server`：HTTP 服务；
- `contract-ledger-ops`：版本、环境预检和显式 migration；
- `contract-ledger-backup`：CL-M2-011 全量备份／恢复；
- 示例环境配置；
- 本安装升级手册和备份恢复手册。

它只要求 PostgreSQL 18 和本系统独占的 Evidence 持久目录。Money Flow、共享 IAM、客户 OA、Project、Approval、对象存储和消息队列都不是安装条件。

应用不创建生产数据库账号、不配置 TLS／反向代理、不安装操作系统服务，也不执行 DNS 或流量切换。这些属于目标部署环境；本手册只定义应用能验证的边界。

## 2. 构建 release bundle

从仓库根目录执行：

```bash
RELEASE_PROVENANCE_MODE=official \
RELEASE_BUILT_AT='2026-09-19T12:00:00Z' \
make contract-ledger-release \
  VERSION='v0.1.0' \
  OUTPUT_DIR='/绝对路径/release-output'
```

也可以直接调用 `backend/services/contract-ledger/ops/build-release VERSION OUTPUT_DIRECTORY`。脚本默认构建当前 GOOS／GOARCH；第一版支持 `linux` 和 `darwin`，交叉构建时显式提供 `GOOS`、`GOARCH`。脚本使用 `CGO_ENABLED=0`、`GOWORK=off`、`-trimpath`，拒绝覆盖同名 `.tar.gz` 或 `.sha256`。

正式模式要求当前路径就是 Git worktree 根、存在 `HEAD` 且 tracked、staged、untracked 状态全部为空；脚本从 Git 读取完整 HEAD，调用方若额外提供 `RELEASE_COMMIT`，必须完全一致。`RELEASE_BUILT_AT` 由发布流水线显式注入。任一门禁失败都发生在创建输出和编译前。

本地开发与 clean-room 测试默认使用 `RELEASE_PROVENANCE_MODE=development`；未提供 commit 时会标记为 `unknown`，也可显式注入测试标识。这类产物不得冒充正式可追溯发布。详细决策见 [ADR-0032](../adr/0032-official-release-provenance-requires-clean-git-head.md)。

## 3. 校验和解包

在归档目录执行其中一种校验：

```bash
shasum -a 256 -c contract-ledger_v0.1.0_linux_amd64.tar.gz.sha256
```

```bash
sha256sum -c contract-ledger_v0.1.0_linux_amd64.tar.gz.sha256
```

校验成功后解包到版本目录，不覆盖当前运行目录：

```bash
tar -xzf contract-ledger_v0.1.0_linux_amd64.tar.gz
cd contract-ledger_v0.1.0_linux_amd64
```

核对三个二进制输出完全相同的 `version`、`commit`、`built_at` 和目标平台：

```bash
bin/contract-ledger-server --version
bin/contract-ledger-ops version
bin/contract-ledger-backup --version
```

版本命令不连接数据库、不创建目录，也不读取业务配置。

## 4. 准备运行配置

把 `config/contract-ledger.env.example` 复制到 bundle 外的受控配置目录，设置权限 `0600`。至少逐项确认：

- `DATABASE_URL` 只指向 Contract Ledger 自己的 PostgreSQL 18 数据库；
- `EVIDENCE_STORAGE_DIR` 是本系统独占、持久、可备份的绝对路径；
- 正式 HTTPS 使用 `SESSION_COOKIE_SECURE=true`；
- 正式运行使用 `MIGRATION_MODE=verify`；
- 首次安装的 bootstrap 用户名和密码成对设置，密码通过部署密钥机制注入；
- `HTTP_ADDR`、会话时长、关闭超时和文件上限符合部署环境。

不要把真实配置写回 bundle、仓库或备份归档。下面命令中的 `source` 只是示例；实际进程监督器应通过其安全环境机制注入：

```bash
set -a
. /受控配置目录/contract-ledger.env
set +a
```

## 5. 全新环境安装

### 5.1 准备数据库和 Evidence

由数据库管理员创建 PostgreSQL 18 数据库和最小可用账号。数据库必须没有用户 relation。创建 Evidence 上级目录并让运行 Contract Ledger 的专用操作系统账号拥有它；Evidence 根目录本身可由预检创建。

不要先手工执行仓库中的 SQL。migration 已嵌入 ops/server 二进制。

### 5.2 预检空环境

```bash
bin/contract-ledger-ops check
```

成功报告至少应为：

- PostgreSQL `major: 18`；
- schema `state: empty`；
- `server_ready: false`；
- Evidence `writable: true`。

如果数据库已有 relation 却没有可信 `schema_migrations`，状态是 `incompatible`，不得用 migration 猜测修复。

### 5.3 显式迁移并执行启动门禁

```bash
bin/contract-ledger-ops migrate
bin/contract-ledger-ops check --require-current
```

第二条必须报告 `state: current`、最新 migration 和 `server_ready: true`。一次 migrate 的全部 pending migration 在同一 PostgreSQL 事务内执行，失败时不会提交本次部分 migration。

### 5.4 首次启动

确认 `MIGRATION_MODE=verify` 后启动：

```bash
bin/contract-ledger-server
```

服务只在 PostgreSQL major、schema 和 Evidence 检查通过后监听。空用户库必须提供合规 bootstrap 凭据；首次启动原子建立一个 administrator。成功后：

```bash
curl --fail http://127.0.0.1:8081/health/live
curl --fail http://127.0.0.1:8081/health/ready
```

登录管理员，创建一个测试主体／合同并上传、下载一份 Evidence。确认后从运行环境移除 `BOOTSTRAP_ADMIN_USERNAME`、`BOOTSTRAP_ADMIN_DISPLAY_NAME`、`BOOTSTRAP_ADMIN_PASSWORD`，重启服务并再次验证登录和 health。已有用户时服务不会用 bootstrap 配置覆盖账号，但长期保留明文启动密码仍是不必要风险。

### 5.5 独立性验收

保持 Money Flow 数据库和应用停止，重复登录、合同查看、Evidence 下载和 health 检查。Contract Ledger 安装不得出现另一个 BizFoundry 系统的连接配置或运行依赖。

## 6. 日常启动门禁

正式启动前可由进程监督流程执行：

```bash
bin/contract-ledger-ops check --require-current
```

`MIGRATION_MODE=verify` 下 server 会再次执行相同核心门禁，但不会迁移。以下情况都在监听前失败：

- PostgreSQL major 不是 18；
- schema 是 empty、pending 或 incompatible；
- 数据库含当前二进制未知 migration；
- applied migration 不是嵌入清单的严格前缀；
- Evidence 根目录无法创建或不可写；
- bootstrap、Cookie 或其他配置格式无效。

仓库源码本地开发仍可使用默认 `MIGRATION_MODE=auto`。auto 只迁移合法 empty／pending 状态，不会绕过 unknown、gap 或非托管数据库检查。

## 7. 升级

假设旧版本运行正常，新 bundle 已经完成第 3 节校验。

### 7.1 在线预检新版本

用新 bundle、但仍指向当前数据库和 Evidence：

```bash
/新版本/bin/contract-ledger-ops check
```

允许结果：

- `current`：新版本没有新增 migration；
- `pending`：当前 applied 列表是新版本嵌入清单的严格前缀。

`incompatible`、unknown、gap 或 PostgreSQL major 错误必须在停服前解决。此时不要执行 migrate。

### 7.2 生成升级前全量备份

保持旧服务正常运行，使用新 bundle 或已验证的 backup 工具执行 `docs/BACKUP-RESTORE.md` 中的完整备份。记录：

- 归档绝对路径和外部 SHA-256；
- 源 schema version；
- Contract／Party／Evidence 计数；
- 生成时间与执行人；
- 备份存储位置和恢复演练记录。

备份失败时不得停服或迁移。

### 7.3 停服、迁移、启动

1. 停止旧 `contract-ledger-server`，确认没有其他业务写入进程；
2. 执行新版本 `bin/contract-ledger-ops migrate`；
3. 执行新版本 `bin/contract-ledger-ops check --require-current`；
4. 以新版本 server 和 `MIGRATION_MODE=verify` 启动；
5. 验证 live、ready、登录、代表性合同 revision／结算／关闭历史和至少一份 Evidence 下载；
6. 记录新 build identity 和 schema version。

不要在旧服务仍接受写入时执行 migration。本轮没有设计多版本滚动兼容协议。

## 8. 失败处理和回退

| 失败点 | 处理 |
| --- | --- |
| bundle SHA／version 不一致 | 丢弃该 bundle，旧服务不变 |
| 新版预检失败 | 不停服、不迁移；调查连接、schema 或 Evidence |
| 升级前备份失败 | 不停服、不迁移；先修复备份完整性 |
| 已停服但未迁移 | 可以重新启动旧 server |
| migrate 事务失败 | 本次 pending migration 整体回滚；保留错误证据，修正后重试 |
| migration 成功但新版启动失败 | 优先修复并前滚；不要让旧二进制连接它不认识的 schema |
| migration 成功但必须回退 | 把升级前全量备份恢复到新的空数据库和新 Evidence 目录，再让旧 bundle 指向恢复目标 |

本项目不提供 down migration、不自动清空失败目标，也不覆盖原数据库／Evidence。恢复和切换步骤需要显式的新目标与部署侧操作，详见 `docs/BACKUP-RESTORE.md`。

CL-M2-012 之前构建的旧二进制没有完整 unknown-schema 门禁，尤其不能把“旧进程似乎能启动”当作兼容证据。

## 9. 每次安装／升级应保存的证据

- bundle 文件名、SHA-256 和三个二进制 build identity；
- PostgreSQL version 和升级前／后的 schema 报告；
- Evidence 根目录检查结果；
- 升级前备份 SHA-256；
- migrate 输出；
- live／ready、登录、代表性合同和 Evidence 验收结果；
- Money Flow 停止或不存在的独立性确认；
- 失败、前滚或恢复到新目标的处置记录。
