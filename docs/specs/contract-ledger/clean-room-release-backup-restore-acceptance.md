# Contract Ledger：Clean-room Release、Evidence 与恢复验收

里程碑：M6  
功能编号：CL-M6-003  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

证明 Contract Ledger 的实际 release bundle 能够在不依赖仓库源码运行时、Money Flow、客户 OA、共享 IAM 或既有业务环境的前提下，从随机全新 PostgreSQL 18 数据库和新的本地 Evidence 根目录完成独立安装，形成最小合同与真实文件证据，再把数据库和 Evidence 的联合备份交接并恢复到另一组空目标。

恢复实例必须拒绝源会话，允许原账号重新登录，能够读取合同、下载与原字节完全一致的 Evidence，并继续写入。本切片验证交付物路径，不以源码 server、`httptest`、内存 Blob Store、既有开发库或只调用 `internal/systembackup` 的测试代替实际二进制和真实 PostgreSQL 客户端。

## 2. 验收输入与隔离

验收只接受：

- 一个由 Contract Ledger `ops/build-release` 生成的 `.tar.gz`；
- 与归档同名的外部 `.sha256`；
- 一个可创建和删除临时数据库的 PostgreSQL 18 测试入口；
- 与 PostgreSQL 18 兼容、能够访问本次随机数据库的 `pg_dump` 和 `pg_restore` 可执行入口；
- 本次测试生成的临时 Evidence 父目录、监听端口和一次性 bootstrap 密码。

源库、恢复目标和失败目标必须是本次测试随机创建的不同数据库。每个 Evidence 根路径也必须相互独立。验收不得迁移、清空、覆盖或复用开发数据库与开发 Evidence，不读取 Money Flow 配置、数据库或进程。

## 3. 成功链路

自动化入口必须按以下顺序执行：

1. 校验 release 外部 SHA-256，严格解包固定六个文件，并核对 server、ops、backup 的 build identity；
2. 新建随机源数据库和新的 Evidence 根路径，由实际 ops 完成 `empty → migrate → current`；
3. 以 `MIGRATION_MODE=verify` 启动实际 server，建立首位 administrator 并经真实 HTTP 登录；
4. 建立两个通用 Party 和一份金额为 `1000.00 CNY` 的合同草稿；
5. 向合同整体上传一份固定测试文件，核对数据库元数据、Evidence 普通文件、下载响应和 SHA-256；
6. 保持源 server 在线，调用实际 backup 和真实 `pg_dump` 创建 full-backup-v1；
7. 离线核验归档权限、manifest、custom dump 及唯一 Evidence entry，把 whole-ZIP 外部 SHA-256 与独立交接副本核对一致；
8. 优雅停止源 server，创建另一个真正空的随机数据库和不存在的目标 Evidence 根路径；
9. 调用实际 backup 和真实 `pg_restore` 恢复交接副本；
10. 确认目标 schema current、session 为 0、Evidence 根没有恢复标记，再以无 bootstrap 配置的实际 server 启动；
11. 使用源 Cookie 访问恢复实例必须进入登录；使用原账号重登后，合同页面和 Evidence 下载均保持；
12. 通过恢复实例 HTTP 新建第三个 Party，证明目标可继续写入；源库仍保持两个 Party；
13. 优雅停止恢复实例，回收本次随机数据库、进程和临时目录。

## 4. 代表性事实与文件

第一版固定使用不绑定行业的最小事实：

```text
本方组织 + 往来组织
  → 合同草稿 1000.00 CNY
  → Evidence target = contract
  → 文件名 delivery-proof.txt
  → 固定 UTF-8 文件字节
```

备份前和恢复后都必须直接核对：一个本地 administrator、两个 Party、一份 draft Contract、一个 Evidence Document、一个 available Evidence Version、对应创建命令和写入审计。文件大小与 SHA-256 必须在 HTTP 下载、数据库、归档 manifest 和本地 Evidence 普通文件之间一致。

草稿足以验证合同事实与 Evidence 存储边界。本切片不为了“显得完整”额外引入补充协议、终止、结算、关闭或更正；它们已经由 M2 的领域与真实 PostgreSQL 测试覆盖。

## 5. 归档与移交核验

release 归档只能包含：

```text
bin/contract-ledger-server
bin/contract-ledger-ops
bin/contract-ledger-backup
config/contract-ledger.env.example
docs/INSTALL-UPGRADE.md
docs/BACKUP-RESTORE.md
```

full-backup-v1 必须满足：

- 最终文件权限为 `0600`；
- ZIP 根包含 `manifest.json`、`database.dump` 和 manifest 唯一声明的 Evidence 普通文件，没有目录、软链接、压缩、加密、重复或未知 entry；
- manifest 使用格式版本 1、UTC 生成时间、当前 schema、`sessions_excluded=true`，合同／主体／Evidence 计数精确；
- database dump 和 Evidence entry 的实际字节数、SHA-256 与 manifest 一致；
- Evidence storage key 和路径符合 `local-store-v1`，归档字节与源下载字节相同；
- whole-ZIP 外部摘要在复制到独立交接路径前后完全相同。

## 6. 失败边界

同一个自动化验收还必须证明：

- 已经迁移的非空目标在任何恢复写入前拒绝，目标 Evidence 根不被创建；
- 空数据库配合已经存在的 Evidence 根时拒绝，已有目录内容保持，数据库仍为空；
- 保留 ZIP 合法结构但篡改 Evidence 字节时，恢复因摘要不一致失败，空目标不产生用户 relation，目标 Evidence 根不发布；
- 恢复没有 `--force`、自动清库、覆盖、合并或回写源路径；
- 失败清理只删除带本次随机所有权标记的暂存或发布目录，不触碰预先存在的目录。

## 7. 独立性与安全边界

- server、ops 和 backup 子进程使用显式白名单环境，不继承 Money Flow、客户 OA、连接器或共享服务配置；
- bootstrap 密码只存在于首次 server 子进程环境，不写入 bundle、备份、仓库或正常日志；
- 备份包含本地密码摘要、合同事实、审计和业务文件，必须作为高敏感运维资产处理；
- Money Flow 停止时完整链路必须通过；当前没有跨系统业务流，因此不制造联合备份或伪连接器联调；
- 单合同账簿导出不是本切片的全量恢复输入。

## 8. 验收标准

1. 实际 release 摘要、固定文件集合和三二进制 build identity 通过核验。
2. 实际 ops 对随机源库依次报告 empty 和 current，并核对新的 Evidence 根可写。
3. 实际 server 使用 `verify` 启动，经 HTTP 完成登录、两个 Party、合同草稿和 Evidence 上传下载。
4. 实际 backup 在源服务在线时调用真实 `pg_dump` 创建包含唯一 Evidence 的严格 full-backup-v1。
5. 归档权限、entry 集合、manifest、dump 和 Evidence 大小／SHA-256、whole-ZIP 交接摘要全部通过。
6. 实际 backup 只向真正空数据库和不存在的 Evidence 根调用真实 `pg_restore`，恢复后 schema、计数、文件和 session 状态正确。
7. 无 bootstrap 的恢复 server ready；源 Cookie 无效，原账号可重新登录，合同和 Evidence 文件可读取。
8. 恢复实例可继续创建 Party，源和目标计数彼此隔离。
9. 非空数据库、已存在 Evidence 根和损坏 Evidence 归档均明确失败且不造成越界修改。
10. Money Flow 停止时全链路通过；测试结束后本次进程、随机数据库和临时目录归零。
11. 聚焦验收、两个应用全量回归、`-race`、`GOWORK=off`、文档与脚本基线检查通过。

## 9. 明确不做

- 定时备份、RPO／RTO 承诺、保留策略、远端复制、KMS 加密或自动灾备切换；
- 恢复到非空库、覆盖源库、合并 Evidence 根、选择性合同或文件恢复；
- 跨 PostgreSQL major 恢复或数据库升级；
- 外部对象存储、病毒扫描、OCR、电子签章或法律真实性判断；
- Money Flow 联合快照、分布式事务、组合安装或客户环境编排；
- TLS、反向代理、systemd、Kubernetes、DNS 或生产存储设施验收。
