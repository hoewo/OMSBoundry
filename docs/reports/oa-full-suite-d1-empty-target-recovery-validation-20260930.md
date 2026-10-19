# OA 完整 Suite 公司 D1 隔离空目标恢复验证

日期：2026-09-30  
状态：`empty-target-recovery-post-write-and-rebackup-supported / continuous-c1-c8-journey-pending`  
运行环境：公司 Ubuntu D1 Compact，2 vCPU／2 GiB，`linux/amd64`

## 1. 问题与验收结果

问题 A：`v0.1.0-test.20` 已证明十二个业务应用能够在公司 D1 完整装配并形成联合备份，但还不能证明该备份能在不破坏当前运行栈的情况下恢复到独立空目标、继续写入并再次形成可信备份。  
可验证结果 B：把备份 `20260929T155403Z` 恢复到独立 Compose project、独立宿主端口和全新命名卷；完成 17/17 健康、十二个 schema current、正式入口读写验收、恢复后新写入和第二次联合备份。主运行项目在全过程保持 17/17 healthy。

本轮关闭“公司 Ubuntu 完整 Suite 空目标恢复／恢复后继续写入”门槛，不替代同一业务数据的 C1～C8 连续旅程、真人浏览器关键权限路径、离机加密副本或公网 HTTPS。

## 2. 隔离与安全约束

1. 恢复目标使用独立 Compose project `omsbizfoundry-restore20`、独立入口 `127.0.0.1:18120` 和独立命名卷，不复用主项目数据库卷。
2. 来源备份只在恢复窗口把选中目录临时交给容器内 PostgreSQL 用户；父目录只临时设为可遍历 `0711`，不是可列目录。
3. 恢复结束后目录恢复为宿主 `root:root`，目录 `0700`、文件 `0600`；数据库 dump 没有成为 world-readable。
4. 恢复失败不得修改主项目、删除主项目卷、跳过 checksum 或通过 down migration 适配旧二进制。
5. 本机不启动完整 Docker 组合；目标机也按恢复、验收、备份、清理顺序串行执行。

## 3. 恢复前直接证据

来源备份：

```text
/var/backups/omsbizfoundry/20260929T155403Z
```

它包含十二个已选择业务应用、IAM 签名材料、Suite selection 和 15 项受 checksum manifest 保护的制品；`sha256sum -c checksums.sha256` 全部通过。恢复前主项目状态为：

```text
long-running services: 17
healthy services:      17
GET /health/ready:     200
```

## 4. 恢复过程暴露并修正的问题

### 4.1 非 root 恢复容器无法读取宿主 root 备份

第一轮隔离恢复在读取来源目录时失败。原因不是备份损坏，而是安全模式 `0700／0600` 与容器内非 root PostgreSQL 用户之间缺少一个有界的所有权交接。

`deployments/d1/ops/d1` 现在只对明确选中的备份目录执行临时交接，并以 trap 保证恢复所有权和严格权限。发布包测试固定检查交接与回收逻辑，未采用把整个备份根目录永久放宽为 `0755` 或 dump 放宽为 `0644` 的做法。

### 4.2 完整 Capability 授权必然撞上一分钟限流

完整 Suite 的十二份 Capability Manifest 共需创建 128 项 Grant，而 Gateway 默认每分钟上限为 120。旧 `grant-bootstrap-access` 因此会稳定得到 HTTP 429，并非偶发网络问题。

脚本现在遇到 429 时只等待一个限流窗口并重试一次；第二次仍被限流就失败关闭。没有关闭限流、重启 Gateway 或无限循环重放。

### 4.3 Bonus 浏览器验收错误绑定边缘前缀

旧验收期待 JavaScript 内硬编码 `/bonus/api/v1/bonus-pools`，但应用资产有意使用相对 `/api/v1/bonus-pools`，由浏览器在 `/bonus/` 挂载点解析到边缘路径。验收现检查应用真实契约，不要求业务资产知道 Nginx 前缀。

### 4.4 Contract 镜像与当前 Capability 契约代际不一致

有效 IAM Credential 直接访问 Contract 仍返回 401。Issuer、Audience、Tenant、Subject、Scope、时间、签名公钥摘要均正确；进一步检查发现主备份引用的旧 Contract 镜像不包含当前 `contract-ledger.reference.read` action，而源码和当前 IAM 已包含。

这说明“镜像有 digest”只证明字节不变，不能证明整套镜像与契约属于同一兼容闭包。Contract 当前源码低资源全量测试通过后，在目标构建机生成新候选：

```text
contract-ledger:v0.1.0-test.26
sha256:cf84233d9cb02071e9f03c7fc086f6c1430b4d3074977d697811de6759805af9
```

隔离恢复项目用该摘要前向执行 `000019_party_runtime_transitions.sql`，没有修改来源备份或回退 schema。

### 4.5 macOS AppleDouble 污染首次临时构建上下文

首次目标机构建得到的 `test.25` 包含 `._*.sql`，被 `go:embed migrations/*.sql` 一并嵌入，严格 migration 前缀门禁按预期拒绝启动。该镜像明确作废，没有覆盖 tag 或继续使用。

清除临时构建上下文中的 AppleDouble 文件后产生 `test.26`；二进制确认包含 `000019_party_runtime_transitions.sql` 且不含 `._000001_bootstrap.sql`。正式 release 构建本身已有拒绝 `._*` 的门禁。

## 5. 最终恢复与运行验收

隔离项目最终结果：

```text
long-running services: 17
healthy services:      17
selected applications: 12
Contract schema:       000019_party_runtime_transitions.sql
Money schema:          000022_bonus_payable_suggestions.sql
Party schema:          000001_bootstrap.sql
```

随后执行 `acceptance/d1-delivery/verify-running`，结果为：

```text
D1 running-stack acceptance passed: nginx-only exposure, one login,
enabled-app read/write, Contract/Money Party migration preflight,
People roster snapshot, Party to Project exact-reference lifecycle
and Evidence byte lifecycle
```

该验收实际经过 Nginx、Gateway、IAM 和各目标应用，包含页面／资产、授权、业务读写、迁移预检、精确引用和 Evidence 字节生命周期，不只是容器健康检查。

## 6. 恢复后写入与第二次备份

来源备份中的 Evidence 为：

```text
files=9, versions=10, objects=10
```

恢复并完成正式入口写入后为：

```text
files=10, versions=11, objects=11
```

随后生成新的联合备份：

```text
/var/backups/omsbizfoundry/20260929T171025Z
```

新备份的全部 checksum 通过，记录 Contract schema 000019、Money schema 000022、Party schema 000001，并保留恢复后新增的 Evidence File／Version／Object。目录最终为 `root:root 0700`，文件为 `root:root 0600`。

这形成了可区分的证据链：`备份 A → 空目标恢复 → 真实写入 → 备份 B`，而不是对同一份文件重复校验。

## 7. 对架构主张的影响

本轮支持：

1. 完整十二业务应用的联合备份能够在同一公司 Ubuntu 的独立空目标恢复，并保持应用数据所有权、ID、revision、schema 和 Suite selection。
2. 恢复后的入口链可继续完成受保护业务写入，并再次生成可校验备份。
3. 恢复、限流、前端挂载和镜像代际问题都能在部署工具、验收契约或单应用镜像边界内修正，没有引入共享业务表、跨库事务或客户核心分支。
4. digest pinning 必须与 Capability／migration 兼容闭包同时使用；仅固定摘要不足以证明一组镜像可以共同运行。

本轮仍不支持：

- 同一 Project／Party／Subject／Overlay 已连续完成 C1～C8；
- 全部代表性角色、双 Tenant 和所有撤权路径均已在完整 Suite 复演；
- 当前 development runtime 已达到正式生产发布质量；
- 公网 HTTPS、容量、高可用和离机灾难恢复已经通过。

## 8. 清理与当前状态

验收结束后使用 `d1 down` 停止隔离恢复项目，保留隔离命名卷供审计，不影响主项目。最终复核：

```text
main project:    17 running / 17 healthy
restore project: 0 running
memory used:     about 640 MiB
memory available: about 888 MiB
swap used:       about 182 MiB
```

下一 P0 是在同一 Suite／Overlay／数据集上建立 C1～C8 连续旅程；真人浏览器关键权限路径与剩余正式 connector 作为后续证据轴分批执行。
