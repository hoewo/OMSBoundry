# Money Flow Ledger：Clean-room 备份交接与空库恢复验收

里程碑：M6  
功能编号：MF-M6-002  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

证明 Money Flow Ledger 的实际 release bundle 不只能够安装，还能够从一个正在运行且具有代表性事实的独立实例创建全量备份，把备份作为独立文件交接，并由同一 bundle 恢复到另一个全新空 PostgreSQL 18 数据库。恢复实例必须能够拒绝旧会话、使用原账号重新登录、完整读取备份时点事实并继续写入。

本切片验证“交付物的备份与恢复路径可用”。它不以源码内 mock、内存客户端、既有开发库或只调用 `internal/systembackup` 的测试代替实际 `money-flow-ledger-backup` 二进制、真实 `pg_dump`／`pg_restore` 和实际 server 的纵向验收。

## 2. 验收输入与隔离

验收只接受：

- 一个由 Money Flow `ops/build-release` 生成的 `.tar.gz`；
- 与归档同名的外部 `.sha256`；
- 一个可创建和删除临时数据库的 PostgreSQL 18 测试入口；
- 与 PostgreSQL 18 同 major、能够访问本次随机数据库的 `pg_dump` 和 `pg_restore` 可执行入口；
- 测试进程生成的临时目录、端口和一次性 bootstrap 密码。

源库、空目标库、非空拒绝目标和损坏归档拒绝目标都必须使用本次测试随机生成的不同数据库。验收不得迁移、清空、覆盖或复用开发数据库，也不得读取 Contract Ledger 配置、数据库或进程。

## 3. 成功链路

自动化入口必须按以下顺序执行：

1. 校验 release 外部 SHA-256、严格解包固定文件，并核对 server、ops、backup 的 build identity；
2. 新建随机源数据库，由实际 bundle 的 ops 完成 migration；
3. 启动实际 bundle server，建立首位 administrator，并经真实 HTTP 完成代表性业务链路；
4. 保持源 server 在线，调用实际 bundle backup 的 `create` 和真实 `pg_dump` 生成新 ZIP；
5. 离线核验归档权限、严格双文件集合、store method、manifest、dump 大小和 SHA-256；
6. 计算整个 ZIP 的外部 SHA-256，把字节复制到另一个临时交接路径，再核对摘要未变；
7. 优雅停止源 server，创建另一个真正空的随机目标数据库；
8. 调用实际 bundle backup 的 `restore` 和真实 `pg_restore` 恢复交接副本；
9. 确认目标 schema current、`local_sessions` 为空，再以 `MIGRATION_MODE=verify` 且无 bootstrap 配置启动实际 server；
10. 使用源实例发出的旧 Cookie 访问恢复实例，必须被重定向到登录页；
11. 使用备份中的原 administrator 账号重新登录，核对代表性事实、预计现金投影和精确余额；
12. 通过恢复实例 HTTP 新建一个合法 Party，证明数据库可继续写入；源库相应计数不得变化；
13. 优雅停止恢复实例并回收本次随机数据库与临时文件。

任一步失败都必须终止验收并提供可定位的命令或 server 输出，不能跳过步骤后报告成功。

## 4. 代表性事实

备份前固定建立不绑定客户行业的最小应收链路：

```text
本方组织 + 往来个人
  → 人工应收义务 100.50 CNY
  → 确认形成 revision 1
  → 设置预计收款日期 2099-10-20
  → 登记实际收款 40.25 CNY
  → 核销 40.25 CNY
  → 当前未结与预计金额均为 60.25 CNY
```

恢复后必须直接核对一个本地用户、两个源 Party、一条 Obligation revision、一条 Cash Expectation、一条 Cash Transaction、一条 Allocation 和 `60.250000` outstanding。继续写入第三个 Party 后，目标库 Party 数为 3，源库仍为 2。

## 5. 归档与移交核验

生成的全量备份必须满足 `omsbizfoundry.money-flow-ledger/full-backup-v1`：

- 文件权限为 `0600`；
- ZIP 根目录只能有普通文件 `manifest.json` 和 `database.dump`，均使用 store method；
- manifest 使用格式版本 1、UTC 生成时间和当前 schema，明确 `sessions_excluded=true`；
- 表计数按名称严格升序，不含 `local_sessions`，但包含 `schema_migrations`、身份、命令、审计及全部当前业务表；
- `database.dump` 的实际字节数和 SHA-256 与 manifest 完全一致；
- whole-ZIP 外部摘要在复制到交接路径前后完全相同。

外部 whole-ZIP 摘要是本次交接完整性证据，不替代签名、可信时间戳、部署侧加密、远端存储或保留策略。

## 6. 失败边界

同一个自动化验收还必须证明：

- 已经由实际 ops 建立 migration 的非空目标在 `pg_restore` 前被拒绝，原 schema 和事实不变；
- 交接 ZIP 任一字节被篡改后，恢复命令失败；用于该失败用例的全新目标仍没有用户 relation；
- 恢复过程没有 `--force`、自动清库、覆盖、合并或回写源库路径；
- 失败清理只影响本次随机数据库、进程和临时目录。

损坏归档用例不承诺错误一定在 ZIP、manifest 或 dump 的某个固定层次出现；只要求命令明确失败且空目标不被部分写入。

## 7. 独立性与安全边界

- server 和 backup 子进程使用显式白名单环境，不继承 Contract Ledger、连接器或其他业务系统配置；
- 数据库密码只存在于测试输入 URL 或客户端进程环境，不写入 bundle、备份 manifest、仓库或验收输出；
- 备份包含密码摘要、幂等命令、审计和全部账簿，应作为高敏感资产处理；
- 本切片没有 Contract → Money Flow 业务动作，因此验证 Money Flow 在 Contract 停止时独立备份恢复，不制造联合备份或伪连接器联调；
- owner CSV 不是本切片恢复输入，恢复只消费 full-backup-v1。

## 8. 验收标准

1. 实际 release 摘要、固定文件集合和三二进制 build identity 通过核验。
2. 源实例只使用随机数据库和实际 bundle 完成迁移、启动、登录及代表性业务事实。
3. 实际 backup binary 在源服务在线时调用真实 `pg_dump` 创建归档。
4. 归档权限、双文件集合、entry 类型与 method、manifest、表集合、dump 大小及 SHA-256 全部通过离线核验。
5. whole-ZIP 外部 SHA-256 在独立交接副本上保持一致。
6. 实际 backup binary 只向真正空目标调用真实 `pg_restore`，恢复后 schema current、完整表计数匹配且 session 为 0。
7. 恢复实例在没有 bootstrap 配置时 ready；旧 Cookie 被拒绝，原账号可以重新登录。
8. 恢复后的 HTTP 页面与数据库都显示预计日期和 `60.25 CNY`／`60.250000`，关联事实完整。
9. 恢复实例可以继续创建合法 Party，目标与源数据库保持隔离。
10. 非空目标和损坏归档均明确失败；失败目标没有发生不受控覆盖或部分恢复。
11. Contract Ledger 停止时完整链路通过；测试结束后本次进程、随机数据库和临时文件归零。
12. 聚焦验收、Money Flow／Contract 全量回归、`-race`、`GOWORK=off` 和文档基线检查通过。

## 9. 明确不做

- 定时备份、RPO／RTO 承诺、保留策略、远端复制、KMS 加密或自动灾备切换；
- 恢复到非空库、覆盖源库、合并实例、选择性 owner／表／日期恢复；
- 跨 PostgreSQL major 恢复或数据库升级；
- 用 owner CSV 恢复数据库；
- Contract Ledger 与 Money Flow 联合快照、分布式事务或客户环境编排；
- 外部 TLS、反向代理、systemd、Kubernetes、DNS 或生产存储设施验收。
