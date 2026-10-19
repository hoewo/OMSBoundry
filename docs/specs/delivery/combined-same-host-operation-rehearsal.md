# 双 Release 同主机运行手册与操作复演

里程碑：M6  
功能编号：M6-007  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

把已经由 M6-004 自动证明的同主机组合边界转换为操作者可执行的运行手册，并用两个已经分别通过 clean-room 验收的实际 release 复演关键步骤。该手册只编排两个产品各自已有的 ops、server、backup 和归档内单应用手册，不增加统一部署服务或业务连接器。

## 2. 交付内容

[双 Release 同主机运行手册](../../operations/combined-same-host-operation.md) 覆盖：

- 两个 bundle 的独立校验、解包和六二进制 build identity；
- 不同数据库、账号、端口、配置和 Contract Evidence 根；
- 两套配置分别加载，显式 migration 与 `verify` 启动；
- 同一浏览器双登录、四个应用专属 Cookie 和 bootstrap 移除；
- 单侧日常启停、故障定位和另一侧持续健康检查；
- 两份在线备份、两套空目标恢复和两条单侧升级路径；
- 禁止共库、共配置、共 session、共 Party、联合备份和无事实连接器；
- 停用时进程、release 与数据保留职责分离，不提供宽泛删除命令。

## 3. 复演输入

- Contract Ledger `v0.1.0-test.3` darwin/arm64 bundle 及外部 SHA-256；
- Money Flow Ledger `v0.1.0-test.5` darwin/arm64 bundle 及外部 SHA-256；
- 两个独立 PostgreSQL 18 测试入口；
- 仓库内已经验证的 PostgreSQL 18 客户端 wrapper；
- 每个验收生成的随机数据库、端口、临时目录和 Contract Evidence 根。

归档保存在可恢复的废纸篓位置只用于本次本地复演，不是正式制品仓库或未来发布地址。

## 4. 验收标准

1. 组合入口只消费两个实际归档与摘要，严格校验内容和六个二进制 identity。
2. 两个 ops 分别迁移随机空库，两个 actual server 使用不同随机端口同时 ready。
3. 同一 Cookie Jar 保留四个应用 Cookie，两个系统都能独立写入。
4. 交替单侧停机与无 bootstrap 重启时，另一侧 ready、会话和写入保持。
5. 两个数据库没有对方业务表，Money Flow 不创建 Evidence 路径。
6. Contract clean-room 入口再次完成真实 Evidence、数据库联合备份与空目标恢复。
7. Money Flow clean-room 入口再次完成数据库在线备份、交接与空目标恢复。
8. 三组验收都自动停止自身进程、回收随机数据库和临时目录，不影响 8081／8083 开发服务。
9. 运行手册不声称两份备份具有跨系统事务一致性，也不声称已经发生业务联调。

## 5. 复演结果

2026-09-20 使用上述两个实际 bundle 并行执行：

- `make combined-delivery-acceptance`：通过；
- `make contract-ledger-clean-install-backup-restore-acceptance`：通过；
- `make money-flow-ledger-backup-restore-acceptance`：通过。

三组实际归档验收分别约 12 秒完成。结束后两套 PostgreSQL 测试入口没有随机测试数据库残留，原 8081 Contract 与 8083 Money Flow 开发服务仍 ready。

## 6. 未被本切片证明的内容

- 真实新开发者或客户运维人员已经按手册独立操作；
- 生产进程监督器、反向代理、TLS、DNS、日志、告警或密钥系统配置；
- 性能、容量、RPO／RTO 或高可用目标；
- 跨系统业务连接器、统一登录、统一 Party 或联合恢复时点；
- 任一参考客户场景已经完成上线验收。

这些事项需要对应环境、人员或业务输入，不能由本地自动复演代替。
