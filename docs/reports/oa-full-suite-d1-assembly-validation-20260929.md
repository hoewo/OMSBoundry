# OA 完整 Suite 公司 D1 装配验证

日期：2026-09-29  
状态：`full-selection-and-empty-target-recovery-supported / continuous-c1-c8-journey-pending`  
运行环境：公司 Ubuntu D1 Compact，2 vCPU／2 GiB，`linux/amd64`

## 1. 问题与验收结果

问题 A：此前公司 D1 只运行 Contract、Money、People、Party、Project、Evidence、Business Engagement 和 Production 八个业务应用，不能证明 Billing、Approval & Audit Results、Cost & Outsourcing、Bonus Distribution 能与它们使用同一发布物、同一入口和同一 PostgreSQL 实例共同装配。  
可验证结果 B：使用个人 ACR 的 digest-pinned 镜像，把十二个业务应用同时装入公司 D1；候选经过契约预检、升级前联合备份、全部 migration、17 个长期服务健康检查和 `d1 check`，并保留首次失败、根因和不可变修复版本。

本轮证明“完整选择能被部署并保持应用边界”，不等于 C1～C8 已在同一数据集完成连续业务旅程。

## 2. 不得破坏的约束

1. 服务器不现场编译；所有应用只拉取个人 ACR 中的 `linux/amd64` 固定摘要。
2. 每个业务应用继续使用自己的数据库账号、migration 和事实所有权；同一 PostgreSQL 实例不等于共享业务表。
3. 新增应用不能绕过 Nginx、Gateway、IAM、workload credential 或目标应用最终授权。
4. 升级前必须先解析候选 Capability／Assignment／Workload 契约并建立当前运行选择的联合备份。
5. 失败候选不得覆盖原 tag、删除命名卷、执行 down migration，或通过关闭备份门禁继续。
6. Mac 只按单应用串行测试和构建，不启动完整本地容器组合。

## 3. 缺失应用的低资源准备

四个应用分别执行：

```bash
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

Billing、Approval & Audit Results、Cost & Outsourcing、Bonus Distribution 全部通过。随后每次只构建一个镜像，完成后独立读取 BuildKit metadata 并用 `imagetools inspect` 核对平台，再进入下一个。

| 应用 | 固定镜像摘要 | 平台 |
| --- | --- | --- |
| Billing | `sha256:a7dc473b425df2aef440d933b0de03b58be6115f552a7facc56965a03ad66d14` | `linux/amd64` |
| Approval & Audit Results | `sha256:116013e9addce14d5b3fdaffd3072540c29ac40daab0a182cf32c0e29a042719` | `linux/amd64` |
| Cost & Outsourcing | `sha256:e931b85ef1c57994b458da44acaafbc77158fe7432bae1eaa925c0e1e74b63de` | `linux/amd64` |
| Bonus Distribution（修复后） | `sha256:a623859773a864584791e5c6d38323c6796dd1d6e1ac0da0f9c11cdf324dc9cb` | `linux/amd64` |

构建期间本机 `docker ps` 始终为空。

## 4. 第一处失败：Bonus 交付镜像漏装契约 package

首轮 Bonus 镜像在 `go mod download` 失败：

```text
omsbizfoundry/payable-suggestion-contract@v0.0.0
replaced by ../../packages/payable-suggestion-contract
open /src/packages/payable-suggestion-contract/go.mod: no such file or directory
```

原因是源码模块正确声明了版本化应付建议 wire contract，但 `Dockerfile.bonus-distribution` 只复制 `workload-identity`，没有把 `packages/payable-suggestion-contract` 放入镜像构建上下文。本地模块测试因仓库目录完整而不能暴露该问题。

修复只增加显式 package 复制；没有把 Money 领域代码或数据库模型复制到 Bonus。`publish-images self-test`、Bonus 全模块测试和 `go vet ./...` 随后通过，证明修复仍维持“共享 wire contract、独立领域实现”的边界。

## 5. `test.19` 首次完整组合失败

`v0.1.0-test.19` release 包外部 SHA-256 和包内清单通过。公司 ECS 候选预检成功：

```json
{"service":"identity-access","status":"contracts-valid"}
```

正式 upgrade 先生成当前八应用选择的备份：

```text
/var/backups/omsbizfoundry/20260929T154510Z
```

其中 11 个受清单保护的制品全部 checksum OK，选择文件准确记录八个业务应用为 `true`、四个新增应用为 `false`。

切换后 16 个长期服务健康，Bonus 持续退出：

```text
configure exactly one Money static token or workload credential source
```

### 5.1 根因

Bonus 的启动代码在判断 IAM 模式并构造 workload credential 之前，就立即用 `MONEY_BASE_URL + 空静态 token` 创建 Money 建议客户端。配置层已正确禁止 IAM 模式携带静态 token，因此完整 D1 首次启动必然失败。此前局部连接器测试直接构造客户端，未覆盖服务器启动时的依赖装配顺序。

这不是通过补一个静态 token 解决的问题。那样会绕过 workload identity，并让长期静态凭证重新进入 D1。

### 5.2 修复

客户端构造被收敛为显式二选一：

- template 模式只使用静态 token；
- IAM 模式只使用请求绑定 workload credential；
- IAM 模式缺少 credential 必须失败关闭。

新增 server 层回归测试覆盖 template、IAM 和 IAM 缺失 credential 三种情况。Bonus 全模块测试与 vet 通过后，只发布新的不可变 `v0.1.0-test.20` Bonus 镜像，没有覆盖失败过的 `test.19` tag。

## 6. 安全恢复与正式重跑

失败候选留下 Bonus 停止、其余 16 个服务健康。为避免重启风暴，先显式停止 Bonus。备份门禁会把已创建的 Bonus 容器视为运行选择的一部分并拒绝不健康备份，因此没有绕过门禁直接再次 upgrade。

恢复顺序为：

1. `test.20 preflight-upgrade` 再次返回 `contracts-valid`；
2. `test.20 apply` 使用新 Bonus 摘要恢复候选健康；
3. 独立确认 17/17 长期服务 healthy、十二应用 schema current、`d1 check` 通过；
4. 再执行正式 `test.20 upgrade`；
5. 对完整十二应用选择创建联合备份；
6. 保留命名卷移除全部旧容器和 one-shot 容器；
7. 重跑 provision、全部 migration、工作负载密钥和健康门禁；
8. 最终 `d1 check` 再次通过。

## 7. 最终直接证据

### 7.1 完整选择

最终长期服务为 17 个，全部 healthy：

- PostgreSQL、IAM、Unified Workbench、Gateway、Nginx；
- Contract、Money、People、Party、Project、Evidence；
- Business Engagement、Billing、Production；
- Approval & Audit Results、Cost & Outsourcing、Bonus Distribution。

Nginx 入口：

```text
GET http://127.0.0.1:18000/health/ready → 200
```

完整检查覆盖十二个应用，所有 schema 均为 current。

### 7.2 完整联合备份

备份目录：

```text
/var/backups/omsbizfoundry/20260929T155403Z
```

`sha256sum -c checksums.sha256` 全部通过。备份含：

- IAM dump 与签名密钥；
- Contract、Money、Party、Evidence 归档；
- People、Project、Business Engagement、Billing、Production、Results、Cost、Bonus dump；
- Suite selection。

十二个 `ENABLE_*` 均为 `true`。目录共 16 个文件：15 个受 checksum manifest 保护的制品加 `checksums.sha256` 本身。

### 7.3 资源余量

完整组合健康后：

```text
memory total:     1.7 GiB
memory used:      690 MiB
memory available: 817 MiB
swap free:        about 2.0 GiB
disk available:   about 30 GiB
```

该数据只证明当前空闲／轻量验收负载可以在 D1 运行，不代表容量规划、并发性能或高可用。

## 8. 对解耦平衡点的结论

本轮新增两项有价值的反证：交付镜像必须显式携带共享 wire contract；IAM 运行模式不能先走静态凭证路径。两处问题都能在 Bonus 自身的打包和组合根修正，没有修改 Money 数据库、Money 领域模型、其他应用 API 或共享业务表。

因此本轮支持：

1. 十二个业务应用能够在同一 D1 组合中共存，同时保持独立数据库、migration 和目标应用授权。
2. `packages/payable-suggestion-contract` 属于有限的版本化通信契约，不是共享 Money 领域模型；这类受审计的少量共享比复制业务实现更稳妥。
3. 显式依赖和 workload credential 增加了装配检查，但失败边界清晰、可定位、可单应用修复，未迫使架构退回共享表。
4. 2 GiB D1 仍可作为最低验证档，但只能串行升级与验收，不能据此宣称生产容量。

本轮仍不证明：

- 同一业务数据已经连续走完 C1～C8；
- 完整备份已在独立空目标成功恢复并继续写入；
- 双 Tenant、全部 connector、完整浏览器权限旅程已通过；
- 当前 development runtime 是 clean production release。

## 9. 下一收缩点

1. 在当前完整 `test.20` 选择上建立单一 C1～C8 旅程 ID 与最小连续数据集；
2. 分段验证正常、拒绝、纠错、幂等重放和单来源停机，不一次制造全部压力；
3. 将 `20260929T155403Z` 复制到隔离 Compose project，执行空目标恢复与恢复后写入；
4. 最后补真人浏览器资料下载／分权、完整双 Tenant 和剩余正式 connector，再冻结阶段一报告。

## 10. 2026-09-30 空目标恢复跟进

上述第 3 项已在同一公司 Ubuntu 上用独立 Compose project、独立端口和全新命名卷关闭。来源备份 `20260929T155403Z` 恢复后 17/17 长期服务 healthy、十二个 schema current；正式 `acceptance/d1-delivery/verify-running` 通过 Nginx→Gateway→IAM 完成启用应用读写、Party 迁移预检、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。

恢复过程真实暴露并修正了备份目录非 root 读取、128 个 Grant 超过一分钟 120 限额、Bonus 相对 API 路径验收错误，以及旧 Contract 镜像与当前 Capability 契约代际不一致四类问题。Contract 修复候选为 `v0.1.0-test.26@sha256:cf84233d9cb02071e9f03c7fc086f6c1430b4d3074977d697811de6759805af9`；被 AppleDouble 污染的 `test.25` 明确作废。

Evidence 事实由来源备份的 `9 files / 10 versions / 10 objects` 增至恢复写入后的 `10 / 11 / 11`，随后新备份 `20260929T171025Z` 的 checksum 全部通过。主项目全过程保持 17/17 healthy，隔离项目验收后已停止并保留卷。

详细证据见 [OA 完整 Suite 公司 D1 隔离空目标恢复验证](./oa-full-suite-d1-empty-target-recovery-validation-20260930.md)。当前下一 P0 改为同一 Suite／Overlay／数据集的 C1～C8 连续旅程；空目标恢复不再列为缺口。
