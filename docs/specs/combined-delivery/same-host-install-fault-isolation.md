# 双 Release 同主机组合安装与故障隔离

里程碑：M6  
功能编号：M6-004  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

证明 Contract Ledger 与 Money Flow Ledger 的实际 release bundle 可以在同一受控主机上同时安装和运行，并保持端口、数据库、Evidence、浏览器 Cookie、进程生命周期和业务写入边界彼此独立。任一服务优雅停机或重启时，另一服务必须继续 ready、保持自己的登录态并接受合法写入。

本切片验证“可组合部署”和“故障隔离”，不是 Contract → Money Flow 业务连接器。两个系统不会互相生成、复制或同步 Party、Contract、Obligation、Cash 或 Evidence；没有真实业务流动就不制造接口、事件、幂等键或所谓联调成功。

## 2. 验收输入

验收只接受：

- 一个实际 Contract release `.tar.gz` 及同名外部 `.sha256`；
- 一个实际 Money Flow release `.tar.gz` 及同名外部 `.sha256`；
- 各自独立、可创建和删除随机数据库的 PostgreSQL 18 测试入口；
- 本次测试生成的 Contract Evidence 根、随机监听端口和一次性 bootstrap 凭据。

两个 release 可以有不同版本、commit 和 build time，但各自内部的 server、ops、backup 三个二进制 identity 必须一致且可追溯。验收不从仓库源码启动业务 server，也不复用开发数据库或开发 Evidence。

## 3. 组合安装顺序

1. 分别核验两个外部 SHA-256 和固定 release 文件集合；
2. 为两个应用创建不同随机数据库，为 Contract 分配新的 Evidence 根，为两个 server 分配不同随机端口；
3. 分别调用归档内 ops 完成 migration 和 current 门禁；
4. 同时启动两个归档内 server，均使用 `MIGRATION_MODE=verify` 和各自一次性 bootstrap 配置；
5. 等待两个 ready，并使用同一个 HTTP Cookie Jar 依次登录 Contract 和 Money Flow；
6. 登录第二个系统后再次读取第一个系统，两个登录态都必须仍有效；
7. 分别经真实 HTTP 在各自系统建立一个 Party，并直接核对各自数据库计数；
8. 优雅停止 Contract，确认 Money Flow ready、登录态和第二次 Party 写入不受影响；
9. Contract 在不提供 bootstrap 配置的情况下重启，原会话继续有效并可写入；
10. 优雅停止 Money Flow，确认 Contract ready、登录态和第三次 Party 写入不受影响；
11. Money Flow 在不提供 bootstrap 配置的情况下重启，原会话继续有效并可写入；
12. 优雅停止两个 server，回收本次随机数据库、解包目录和 Evidence 目录。

## 4. 浏览器边界

Cookie 不按端口隔离，因此同一主机的两个 HTTP 服务必须使用不同名称：

```text
contract_ledger_session
contract_ledger_csrf
money_flow_ledger_session
money_flow_ledger_csrf
```

验收必须使用同一个真实 Cookie Jar，不能用两个客户端掩盖名称碰撞。完成双登录和访问两个写入页面后，四个 Cookie 名称必须同时存在且唯一；任一服务清除自己的 CSRF 或 session 时不得删除另一服务 Cookie。

这只证明 Cookie 名称隔离。生产 HTTPS 的域名、反向代理路径、Secure、SameSite 和证书仍由部署环境按实际拓扑验证。

## 5. 数据与文件边界

- Contract 和 Money Flow 使用不同数据库 URL 和随机数据库名；
- Contract 数据库不得出现 Money Flow 的 `obligations`、`cash_transactions` 等表；
- Money Flow 数据库不得出现 Contract 的 `contracts`、`contract_evidence_versions` 等表；
- 同名或相似 Party 是各系统自己的本地事实，不进行跨库 ID 对齐；
- 只有 Contract server 接收 `EVIDENCE_STORAGE_DIR`，文件只出现在该目录；Money Flow 不接收也不读取此路径；
- 子进程环境采用显式白名单，不传入另一个系统的数据库 URL、端口、bootstrap 或业务配置。

## 6. 故障隔离

- 单侧停机不得影响另一侧进程、数据库连接、ready、Cookie 或写入；
- 单侧无 bootstrap 重启不得重复或覆盖本地管理员；
- 停止服务不等于删除数据库或 Evidence，重启后原 session 和业务事实应继续有效；
- 测试只终止自己启动的实际 bundle 进程，不终止开发服务或其他主机进程；
- 端口冲突、数据库共享、Evidence 路径复用都由验收输入显式避免，不建立自动猜测或全局注册中心。

## 7. 验收标准

1. 两个实际 release 的摘要、固定文件集合和各自三二进制 identity 通过。
2. 两个随机数据库名、两个监听端口和 Contract Evidence 路径均不相同且绝对明确。
3. 两个实际 ops 分别完成 empty → current；两个 actual server 同时 ready。
4. 同一 Cookie Jar 同时保有四个应用特定 Cookie，双登录互不覆盖。
5. 两个系统都能独立建立 Party；每个数据库只增加自己的记录。
6. Contract 停机期间 Money Flow ready、会话和写入保持；Contract 无 bootstrap 重启后原会话和事实保持。
7. Money Flow 停机期间 Contract ready、会话和写入保持；Money Flow 无 bootstrap 重启后原会话和事实保持。
8. Contract 数据库没有 Money Flow 业务表，Money Flow 数据库没有 Contract 业务表；Evidence 只存在于 Contract 路径。
9. 没有 HTTP 调用、数据库读取、事件、文件或字段在两个系统之间流动；本切片不伪称业务联调。
10. 聚焦 actual bundle 验收、全仓、两个应用全量 PostgreSQL `-race`、独立构建、文档与脚本基线通过；临时进程、数据库和目录归零。

## 8. 明确不做

- Contract → Money Flow 合同快照、义务建议、事件、API、幂等与失败恢复；
- 共享 IAM、统一用户、单点登录、统一 Party 主数据或跨系统权限；
- 统一反向代理、域名、TLS、systemd、Kubernetes 或服务发现；
- 高可用、负载均衡、崩溃恢复、网络分区或资源压测；
- 客户行业页面、审批、项目、发票、银行或 OA 装配。
