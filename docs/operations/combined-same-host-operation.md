# Contract 与 Money Flow 双 Release 同主机运行手册

日期：2026-09-20  
适用功能：M6-007  
适用边界：两个已经分别通过 clean-room 验收的 Contract Ledger 与 Money Flow Ledger release bundle

## 1. 先确认这不是一个合并系统

同主机运行只共享受控主机，不共享业务真相。两个应用始终分别拥有：

| 资源 | Contract Ledger | Money Flow Ledger |
| --- | --- | --- |
| release 与进程 | `contract-ledger-*` | `money-flow-ledger-*` |
| 默认 HTTP | `127.0.0.1:8081` | `127.0.0.1:8083` |
| PostgreSQL 数据库与账号 | Contract 专用 | Money Flow 专用 |
| 业务文件 | 独占 Evidence 根 | 当前没有外部业务文件 |
| Session／CSRF Cookie | `contract_ledger_*` | `money_flow_ledger_*` |
| 配置与密钥 | Contract 专用文件／注入 | Money Flow 专用文件／注入 |
| 备份、恢复和升级 | 独立执行 | 独立执行 |

本手册不建立 Contract → Money Flow 业务连接器、统一账号、统一 Party、跨库查询、联合事务或联合备份。一个应用停止、升级或恢复时，另一个应用应继续使用自己的数据库和文件运行。

## 2. 主机前置条件

- 两个 bundle 与外部 `.sha256` 已从可信渠道取得，且平台与主机匹配；
- PostgreSQL 18 中准备两个不同数据库和两个最小权限账号；不要用同一数据库的不同表前缀冒充隔离；
- Contract Evidence 使用绝对、持久、独占且可备份的目录；Money Flow 不接收该目录；
- 主机有两个不同监听地址／端口；若经反向代理暴露，TLS、域名和访问日志由部署环境负责；
- 两个应用分别有受控配置、日志、备份和进程监督入口；
- `pg_dump`、`pg_restore` 与 PostgreSQL server 使用相同 major；
- 首次管理员密码分别通过密钥机制注入，不写入 release、仓库、命令记录或备份目录。

推荐目录只是示例，部署环境可替换，但所有权必须保持分离：

```text
/opt/omsbizfoundry/releases/contract-ledger/<version>/
/opt/omsbizfoundry/releases/money-flow-ledger/<version>/
/etc/omsbizfoundry/contract-ledger.env
/etc/omsbizfoundry/money-flow-ledger.env
/var/lib/omsbizfoundry/contract-ledger/evidence/
/var/backups/omsbizfoundry/contract-ledger/
/var/backups/omsbizfoundry/money-flow-ledger/
```

配置文件权限应为 `0600`。不要把两个示例 env 合并成一个全局 env；同名 `DATABASE_URL`、`HTTP_ADDR` 和 bootstrap 变量很容易让进程连接错误目标。

## 3. 分别校验和解包

在各自下载目录校验，任何一侧失败都只丢弃失败产物，不影响另一侧当前版本：

```bash
shasum -a 256 -c contract-ledger_v0.1.0_linux_amd64.tar.gz.sha256
shasum -a 256 -c money-flow-ledger_v0.1.0_linux_amd64.tar.gz.sha256
```

分别解包到新的版本目录，禁止覆盖正在运行的目录：

```bash
tar -xzf contract-ledger_v0.1.0_linux_amd64.tar.gz -C /opt/omsbizfoundry/releases/contract-ledger
tar -xzf money-flow-ledger_v0.1.0_linux_amd64.tar.gz -C /opt/omsbizfoundry/releases/money-flow-ledger
```

核对每个 bundle 内三个二进制的 build identity。两个产品之间可以版本不同，但同一产品的三个输出必须一致：

```bash
/opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-server --version
/opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-ops version
/opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-backup --version

/opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-server --version
/opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-ops version
/opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-backup --version
```

单应用完整要求仍以各归档内 `docs/INSTALL-UPGRADE.md` 和 `docs/BACKUP-RESTORE.md` 为准。

## 4. 准备两个独立配置

Contract 配置至少确认：

```dotenv
HTTP_ADDR=127.0.0.1:8081
DATABASE_URL=postgres://contract_app:<secret>@db-host:5432/contract_ledger?sslmode=require
MIGRATION_MODE=verify
SESSION_COOKIE_SECURE=true
EVIDENCE_STORAGE_DIR=/var/lib/omsbizfoundry/contract-ledger/evidence
```

Money Flow 配置至少确认：

```dotenv
HTTP_ADDR=127.0.0.1:8083
DATABASE_URL=postgres://money_flow_app:<secret>@db-host:5432/money_flow?sslmode=require
MIGRATION_MODE=verify
SESSION_COOKIE_SECURE=true
```

检查清单：

1. 两个 `DATABASE_URL` 的数据库名和账号都不同；
2. 两个 `HTTP_ADDR` 不冲突；
3. `EVIDENCE_STORAGE_DIR` 只出现在 Contract 配置；
4. 正式启动始终使用 `MIGRATION_MODE=verify`，migration 由对应 ops 显式执行；
5. 首次 bootstrap 变量只注入对应进程，两个管理员密码不复用；
6. 如果浏览器通过同一 hostname 访问两个端口，应用内置的四个 Cookie 名称无需修改且不能被反向代理重写为同名；
7. 反向代理若改成不同路径前缀，必须另行验证重定向、Cookie Path 和 CSRF；当前证据只覆盖同一 hostname 的不同端口。

## 5. 全新安装顺序

下面的 subshell 表示每组命令只加载一个应用的配置。实际进程监督器应使用各自的安全环境注入机制。

### 5.1 Contract 预检与迁移

```bash
(
  set -a
  . /etc/omsbizfoundry/contract-ledger.env
  set +a
  /opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-ops check
  /opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-ops migrate
  /opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-ops check --require-current
)
```

第一次 `check` 必须是合法 `empty`，迁移后必须是 `current` 且 Evidence 可写。`incompatible`、unknown 或非空未托管数据库不得继续。

### 5.2 Money Flow 预检与迁移

```bash
(
  set -a
  . /etc/omsbizfoundry/money-flow-ledger.env
  set +a
  /opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-ops check
  /opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-ops migrate
  /opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-ops check --require-current
)
```

第一次 `check` 必须是合法 `empty`，迁移后必须是 `current`。不要让 Contract ops 连接 Money Flow 数据库，反之亦然。

### 5.3 首次启动与 bootstrap

用两个独立的进程监督单元启动。首次启动只向各自服务注入自己的 bootstrap 凭据：

```text
contract-ledger-server  + /etc/omsbizfoundry/contract-ledger.env
money-flow-ledger-server + /etc/omsbizfoundry/money-flow-ledger.env
```

确认：

```bash
curl --fail http://127.0.0.1:8081/health/live
curl --fail http://127.0.0.1:8081/health/ready
curl --fail http://127.0.0.1:8083/health/live
curl --fail http://127.0.0.1:8083/health/ready
```

用同一个浏览器依次登录两个系统，再返回第一个系统确认登录仍有效。建立各自的测试 Party，Contract 还应上传并下载一份测试 Evidence。确认首次管理员存在后，从两个运行环境分别移除 `BOOTSTRAP_ADMIN_USERNAME`、`BOOTSTRAP_ADMIN_DISPLAY_NAME`、`BOOTSTRAP_ADMIN_PASSWORD`，逐个重启并重新验证。不要同时重启，避免掩盖单侧故障范围。

## 6. 日常启动、停止和单侧故障

每次启动前只用同产品 ops 执行 `check --require-current`。建议顺序：

1. 检查目标应用自己的数据库和文件；
2. 启动目标 server；
3. 验证该应用 live、ready、登录和代表性只读页面；
4. 验证另一个应用仍 ready，且原登录态和一次合法写入不受影响。

单侧异常时：

- 只停止异常进程，不停止另一应用或其数据库；
- 不删除数据库、Evidence、配置或 session 来尝试“重置”；
- 先检查该应用的 build identity、配置目标、ops schema 报告和日志；
- 已有用户库重启时不再提供 bootstrap 密码；
- 另一应用 ready 不能证明异常应用已恢复，只能证明故障没有扩散。

两个应用的 session 不互通。登录一个系统不会登录另一个系统，退出一个系统也不应清除另一个系统的 Cookie。

## 7. 分别备份

两侧在线备份可以由部署调度器分别执行，但不会形成跨系统同一业务时点，也不能合并成一个 ZIP 后称为一致性备份。

Contract：

```bash
DATABASE_URL='postgres://contract_app:<secret>@db-host:5432/contract_ledger?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://contract_backup:<secret>@db-host:5432/contract_ledger?sslmode=require' \
EVIDENCE_STORAGE_DIR='/var/lib/omsbizfoundry/contract-ledger/evidence' \
PG_DUMP_BIN='/usr/lib/postgresql/18/bin/pg_dump' \
/opt/omsbizfoundry/releases/contract-ledger/<version>/bin/contract-ledger-backup create \
  --output '/var/backups/omsbizfoundry/contract-ledger/contract-ledger-<UTC>.zip'
```

Money Flow：

```bash
DATABASE_URL='postgres://money_flow_app:<secret>@db-host:5432/money_flow?sslmode=require' \
POSTGRES_CLIENT_DATABASE_URL='postgres://money_flow_backup:<secret>@db-host:5432/money_flow?sslmode=require' \
PG_DUMP_BIN='/usr/lib/postgresql/18/bin/pg_dump' \
/opt/omsbizfoundry/releases/money-flow-ledger/<version>/bin/money-flow-ledger-backup create \
  --output '/var/backups/omsbizfoundry/money-flow-ledger/money-flow-ledger-<UTC>.zip'
```

每个 ZIP 分别计算并保存外部 SHA-256、大小、schema、事实计数、生成时间、执行人、复制位置和保留期限。Contract 备份同时覆盖自己的 Evidence；Money Flow 当前只有数据库。任何一侧备份失败都不能用另一侧成功归档替代。

## 8. 单侧恢复

恢复永远进入该应用的新空数据库；Contract 还要求新的、不存在的 Evidence 根。推荐流程：

1. 保持未恢复应用继续运行并记录 ready；
2. 只停止待恢复应用；
3. 校验该应用备份的外部 SHA-256；
4. 创建明确命名的新空数据库；Contract 另选尚不存在的 Evidence 根；
5. 用同产品 backup 二进制执行归档内 `docs/BACKUP-RESTORE.md` 的 restore；
6. 用同产品 ops `check --require-current`；
7. 把该应用配置切到恢复目标，以 `MIGRATION_MODE=verify` 且无 bootstrap 启动；
8. 验证旧 Cookie 失效、原账号可重登、代表性事实可读并能继续写入；Contract 抽样下载 Evidence；
9. 再确认另一应用全程 ready、会话和数据未变化；
10. 保留原目标直至恢复验收和部署侧回退窗口结束。

不得把 Contract 备份恢复到 Money Flow 数据库，或把两个应用分别恢复到新目标后宣称它们共享同一事务时点。当前没有跨系统业务流，也没有联合恢复顺序依赖。

## 9. 单侧升级

升级一次只处理一个产品：

1. 校验该产品新 bundle 和 build identity；
2. 新 ops 对当前目标执行只读 `check`；
3. 使用该产品 backup 创建升级前归档；
4. 停止该产品 server；
5. 新 ops 执行 `migrate` 和 `check --require-current`；
6. 启动新 server 并验证代表性事实；
7. 确认另一产品在全过程持续 ready 并可使用；
8. 另一产品需要升级时另开维护步骤，不共用 migration 或回退。

详细失败与恢复规则分别见两个 release 内的 `docs/INSTALL-UPGRADE.md`。

## 10. 故障定位矩阵

| 现象 | 首先检查 | 不应采取的动作 |
| --- | --- | --- |
| 两个服务都无法监听 | 端口、主机资源、进程监督和反向代理 | 把两个服务改成同一进程 |
| 只有 Contract 不 ready | Contract DB schema、Evidence 权限、Contract 配置 | 重启或迁移 Money Flow |
| 只有 Money Flow 不 ready | Money Flow DB schema、Money Flow 配置 | 删除 Contract Evidence |
| 登录第二个系统后第一个掉线 | 代理是否重写 Cookie 名称／Path／Domain | 共享 session 表或 Cookie 名称 |
| 数据出现在错误系统 | 立即停止相关进程，核对两个 `DATABASE_URL` 和操作记录 | 跨库搬表或直接修改对方库 |
| Contract Evidence 下载失败 | Contract Evidence 路径、文件摘要、权限和引用状态 | 把 Money Flow 目录当作替代存储 |
| 一侧备份失败 | 该侧客户端 major、连接、空间及归档日志 | 用另一侧备份代替或拼接 ZIP |
| 一侧恢复失败 | 该侧目标是否真正为空、归档摘要和客户端 major | 清空原生产目标或使用 `--force` 思路 |

## 11. 明确禁止

- 两个应用共用业务数据库、schema、账号或 migration 表；
- Money Flow 读取 Contract 内部表，或 Contract 读取 Money Flow 内部表；
- 两个进程加载同一份 env；
- 两个服务使用同一端口、同一进程生命周期或同一 bootstrap 密码；
- 把 Contract Evidence 路径传给 Money Flow；
- 把两份独立备份拼成“联合一致性备份”；
- 为统一登录而共享 session 表、Cookie 或本地用户表；
- 把同名 Party 当作自动对齐的跨系统主数据；
- 在没有真实业务流时创建空连接器、事件或定时同步；
- 用同机部署结果声称合同已经自动形成收支义务。

## 12. 停用与数据保留

停止一个应用不等于删除数据。停用前应：

1. 只停止目标应用进程；
2. 生成并校验目标应用自己的最终备份；
3. 记录数据库、Evidence（如有）、配置、release、日志和备份的保留责任；
4. 先解除流量和自动启动，再处理 release 目录；
5. 数据库、Evidence 和备份的删除必须由部署侧单独授权并使用明确目标；本手册不给出递归删除命令；
6. 验证另一应用仍能独立启动和使用。

## 13. 交付前复演

仓库内使用两个实际 bundle 的自动化组合复演：

```bash
cd /仓库根目录
CONTRACT_TEST_DATABASE_URL='postgres://测试账号:密码@127.0.0.1:55432/contract_ledger?sslmode=disable' \
MONEY_FLOW_TEST_DATABASE_URL='postgres://测试账号:密码@127.0.0.1:55433/money_flow?sslmode=disable' \
make combined-delivery-acceptance \
  CONTRACT_BUNDLE='/绝对路径/contract-ledger_<version>_<platform>.tar.gz' \
  CONTRACT_CHECKSUM='/绝对路径/contract-ledger_<version>_<platform>.tar.gz.sha256' \
  MONEY_BUNDLE='/绝对路径/money-flow-ledger_<version>_<platform>.tar.gz' \
  MONEY_CHECKSUM='/绝对路径/money-flow-ledger_<version>_<platform>.tar.gz.sha256'
```

该入口覆盖严格解包、独立迁移、同时启动、同一 Cookie Jar 双登录、交替单侧停机／无 bootstrap 重启、另一侧继续写入以及数据库／Evidence 零交叉。两侧备份恢复仍分别执行 Contract 与 Money Flow 的 clean-room 验收，不能由组合入口替代。

一次有效复演记录应包含：两个 bundle 名称和 SHA-256、六个二进制 identity、两个 PostgreSQL 入口、两个 schema、端口、Contract Evidence 路径、四 Cookie、单侧重启结果、两份备份摘要、恢复演练结果、临时数据库／目录清理结果，以及仍未发生任何跨系统业务流的声明。
