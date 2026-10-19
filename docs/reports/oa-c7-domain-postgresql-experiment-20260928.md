# OA C7 Bonus Distribution 领域核心与 PostgreSQL 事实层实验

日期：2026-09-28  
结论：`domain-core-and-postgresql-supported / protected-http-and-composition-pending`

## 1. 要回答的问题

本轮只验证一个收缩问题：独立 Bonus Distribution 是否能在不共享 Contract、Money、People 或 Project 数据表的前提下，同时表达经营奖金和生产奖金，并可靠保存精确计算、人员分配、发布、取代、撤回和不可变历史。

可证伪条件包括：

- 两类奖金必须复制成两个不兼容模型，或必须退回宽 `FinancialRecord`；
- 参考 OA 公式必须进入公共核心条件分支，或必须使用浮点数才能实现；
- 来源、Person 或 Assignment 当前值改变会改写已发布历史；
- 发布、取代、撤回、幂等 receipt 和 audit 不能在同一本地事务提交；
- 并发命令可以重复提交，或历史可以被更新／删除；
- 业务角色可以越权操作另一奖金类型、另一 Project 或另一 Tenant。

任何一项成立，本轮都不能通过。

## 2. 范围与明确排除

本轮包含：

- 独立 `backend/services/bonus-distribution` Go module；
- 一个严格 Pool／Allocation 领域模型中的 `business` 与 `production` 类型；
- Project、来源、Person、Assignment 和 Policy 的精确快照；
- 整数最小货币单位、整数基点和显式 half-up 舍入；
- `reference-oa-bonus-v1` Customer Overlay；
- Pool／Allocation current、revision、command receipt 和 audit；
- 草稿建立、发布、取代、撤回、读取、列表与历史；
- PostgreSQL 18 真库、并发、故障、恢复、本地重放和防篡改测试。

本轮不包含 HTTP、正式 IAM、Capability Manifest、网络 adapter、人类页面、独立进程组合、Suite Manifest、D1、联合恢复、C7→Money Payable Suggestion、Project Completion 或 C8 投影。排除项没有被本报告暗示为已通过。

## 3. 实现结构

| 层 | 本轮实现 | 关键边界 |
| --- | --- | --- |
| 领域 | `internal/bonus` | 一个 Pool 模型，两类奖金使用不同来源 resolver 和 action |
| 客户策略 | `internal/overlays/referenceoa` | OA 除数与公式只存在于版本化 Overlay |
| 数据 | `internal/bonus/postgres_store.go` | 独立事务、expected revision、receipt 与 audit |
| migration | `0001_bonus_distribution.sql` | current 与不可变 revision 分离，trigger 保护历史 |
| 数据库装载 | `internal/database` | 嵌入 migration，独立 schema migration 账 |

没有导入 Contract、Money、People 或 Project 的 ORM／领域对象，也没有访问它们的内部表。测试用 resolver 只实现版本化端口，证明当前核心不要求其他应用进程启动。

## 4. 精确金额与 Overlay 验证

金额全部使用 `int64` 最小货币单位，分配比例使用 0～10000 的整数基点。公共核心只接收 `PolicyEvaluator` 结果，不识别客户公式。

`reference-oa-bonus-v1` 使用精确有理数执行：

```text
pool = base_minor × ratio_basis_points / 10000 ÷ 1.065
```

最终结果显式 half-up 舍入。测试还拒绝未知策略、浮点式输入、零／负比例、重复 Person、重复 Assignment 和总比例超过 100%。未分完金额作为 `unallocated_minor` 明确保留，不自动平均或隐式丢弃。

## 5. PostgreSQL 事实层

首个 migration 建立：

- `bonus_pools`；
- `bonus_pool_revisions`；
- `bonus_allocations`；
- `bonus_allocation_revisions`；
- `bonus_distribution_commands`；
- `bonus_distribution_audit_events`。

Pool 与 Allocation current 只能按下一 revision 推进，并要求对应完整 revision 已存在。revision、receipt 和 audit 的 `UPDATE`／`DELETE` 被数据库 trigger 拒绝；current 的物理删除也被拒绝。

每个命令以 Tenant＋Command ID advisory lock 串行化。相同 Command ID 与相同请求摘要返回原 receipt；相同 ID 不同摘要冲突。成功 receipt 在外部复验前读取，所以已经成功的发布、取代或撤回即使依赖随后离线，仍能稳定返回原结果；新命令仍会复验来源并对 drift 失败关闭。

## 6. 验证步骤

### 6.1 普通与独立模块检查

```bash
gofmt -w backend/services/bonus-distribution
go test ./backend/services/bonus-distribution/...
go vet ./backend/services/bonus-distribution/...

cd backend/services/bonus-distribution
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

结果：全部通过。`GOWORK=off` 证明模块不依赖工作区中其他应用源码才能测试、检查或构建。

### 6.2 PostgreSQL 18.6 真库与 race

使用一次性 PostgreSQL 18.6 容器执行：

```bash
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/postgres?sslmode=disable' \
  go test -count=1 -tags=integration -race ./backend/services/bonus-distribution/...
```

结果：全部通过；`internal/bonus` 用时约 6.7 秒，`internal/overlays/referenceoa` 用时约 1.9 秒。测试数据库只用于本轮验证，结束后删除。

### 6.3 反耦合检查

对 module 检索 `float64`、共享财务表、跨库 SQL、当前用户表和其他应用 ORM。结果只命中测试中的主动篡改／删除负例和领域文本，没有发现正式运行路径使用二进制浮点、共享业务表或跨库写入。

## 7. 观察结果

| 验证项 | 结果 |
| --- | --- |
| 经营来源 | 只接受精确 confirmed Settlement snapshot |
| 生产来源 | 只接受精确 confirmed receipt set snapshot |
| 类型隔离 | 经营 Actor 不能建立生产 Pool，反向同理 |
| 快照稳定性 | 来源变化不改写已发布 Pool／Allocation 历史 |
| 生命周期 | draft → published → superseded；另有显式 withdrawn |
| 分配守恒 | 60%＋30% 得到精确金额与显式 10% 余额 |
| 失败关闭 | Project／来源／人员无效或依赖不可用时零业务写入 |
| 恢复 | 依赖恢复后原 Command 单次提交 |
| 成功重放 | 依赖再次离线时仍从本地 receipt 返回原结果 |
| 摘要冲突 | 同 Command 不同请求被拒绝 |
| 并发草稿 | 8 路相同 Command 只建立一个 Pool |
| 并发发布 | 两个 expected revision 发布只有一个成功 |
| drift | 发布前来源改变被拒绝，草稿保持 revision 1 |
| 租户与 Project | 跨 Tenant 隐藏，错 Project 拒绝 |
| 防篡改 | Pool／Allocation history 更新删除和 current 物理删除被拒绝 |

## 8. 测试发现并修复的真实缺陷

第一版发布／取代／撤回路径先检查 current 状态，再查询本地成功 receipt。命令已经成功后状态必然发生变化，因此同一个成功命令重放会在到达 receipt 前被错误拒绝。

修复后，服务先按 Tenant＋Command ID＋request digest 读取成功 receipt；只有新命令才执行状态、revision 和上游快照复验。新增真库用例证明发布、取代和撤回均可在上游离线时成功重放，同时不同摘要仍冲突。这个缺陷说明幂等不仅是“重复插入不发生”，还必须保证状态推进后仍能复演原命令结果。

## 9. C7-A 验收映射

| 验收 | 本轮结论 | 证据 |
| --- | --- | --- |
| C7-A01 | 通过 | Settlement revision／digest 冻结，来源变化不改历史 |
| C7-A02 | 通过 | receipt set 组成、revision、金额与 digest 冻结 |
| C7-A03 | 通过 | 精确有理数＋half-up Overlay 测试 |
| C7-A04 | 通过 | receipt 重放、摘要冲突、8 路并发 |
| C7-A05 | 通过 | 发布冻结，取代／撤回只追加 revision |
| C7-A06 | 核心层通过 | 来源 resolver、action 与 Project 责任隔离；正式 IAM 待验 |
| C7-A07 | 通过 | 仅接受 resolver 返回的 active Person／Assignment 精确快照 |
| C7-A08 | 通过 | 人员／Assignment 后续变化不改历史 |
| C7-A09 | 通过 | 基点约束、金额守恒、未分配余额显式 |
| C7-A10 | 未验证 | 需要受保护 HTTP 与金额脱敏 |
| C7-A11 | 核心层通过 | 无效引用／依赖失败零业务写入；真实 HTTP adapters 待验 |
| C7-A12 | 核心层通过 | 恢复单次提交和成功本地重放；独立进程待验 |
| C7-A13～A17 | 未验证 | 需要 Money、D1 和恢复组合 |
| C7-A18 | 核心层通过 | 客户公式只在 Overlay；页面术语仍待后续验证 |

## 10. 本轮结论与下一步

实验没有触发可证伪条件。一个独立 Bonus Distribution 可以在保持应用数据所有权的前提下复用严格生命周期，同时用类型化端口隔离经营／生产来源，并用 Customer Overlay 承接参考 OA 公式。当前证据支持 `domain-core-and-postgresql-supported`。

下一轮应只推进版本化受保护 HTTP、正式 IAM／Capability、请求绑定的 Project／People／Contract／Money adapters 和金额脱敏，验证 C7-A06、A10～A12 的运行层部分。人类页面、D1、恢复和 Money Payable Suggestion 应继续分轮验证，避免在缺陷尚未暴露前把整个 C7 一次装入正式组合。
