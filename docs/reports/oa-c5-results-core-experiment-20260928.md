# OA C5 Approval & Audit Results 领域核心实验（2026-09-28）

## 判定

- 验证对象：独立 Approval & Audit Results 的领域模型与 PostgreSQL 18 生命周期。
- 本轮判定：`core-supported / protected-entry-and-composition-not-started`。
- OA 完整体判定：继续为 `insufficient-evidence`。
- 总进度影响：C5 已从“只有责任边界”进入“领域核心和独立真库有运行证据”，但没有 HTTP、IAM、人类页面、Project 完成协作、Suite Manifest、D1 或恢复证据。

## 问题 A → 可验证结果 B

**问题 A**：如果批复与审计仍依赖当前合同值、共享表或原地更新，那么审计结果会随着合同或批复的后续变化悄悄漂移，无法验证应用边界，也无法解释纠错历史。

**可验证结果 B**：一个独立模块和独立 PostgreSQL schema 能原子保存批复／审计当前投影、完整 revision、Command 与 Audit；审计冻结批复的精确历史 revision，之后更正批复不改变既有审计；失败、重放、并发和撤回都产生确定结果。

## 本轮范围与明确不做

本轮实现：

- `backend/services/approval-audit-results` 独立 Go module；
- `reference-oa-v1` 的设计费、勘察费、咨询费三项精确金额；
- Project、Contract／Settlement、Evidence 的精确引用解析边界；
- approval → Contract／Settlement revision，audit → approval revision；
- 当前投影、完整 revision、Command、Audit 四类 PostgreSQL 事实；
- 登记、纠正、撤回、撤回后重新登记、查询和历史；
- 幂等、并发、expected revision、租户隔离、失败零写入和数据库防篡改。

本轮不实现：

- HTTP API、Capability Manifest、IAM Provider 和 `allowed_actions`；
- 真实 Contract／Project／Evidence HTTP 工作负载交换；
- 人类页面、Gateway／Nginx、Workbench 和 Suite Manifest；
- Project Completion Profile、完成视图或完成命令；
- 镜像、备份恢复、D1／真实 Ubuntu、C5-A09～A15。

## 假设与反证条件

| 假设 | 可观察证据 | 反证条件 |
| --- | --- | --- |
| Results 是独立事实所有者 | 独立 module、migration、表与 Store；其他应用无可写 Results 表 | 必须跨库写 Project／Contract 才能完成生命周期 |
| 审计默认依赖批复 revision | audit source 指向本应用 approval revision；后续批复更正不改变 audit | audit 必须重读当前合同才能解释自身金额 |
| 金额可以稳定版本化 | 三项 `int64` 最小货币单位，服务端求和，负数／重复／缺项／溢出拒绝 | 只能靠 `float64` 或客户端总数才能完成 |
| 纠错不需要覆盖历史 | correction／withdrawal／reregistration 各追加完整 revision | 必须更新或删除历史行才能继续使用 |
| 外部故障可以失败关闭 | 引用解析失败时四类本地事实均为 0 | 依赖失败留下 current、command 或 audit 半记录 |

## 科学验证步骤

### 1. 先固定纯领域不变量

执行普通测试，验证金额 profile、来源类型、UTC 归一、审计精确 revision、人工覆盖原因、引用故障和幂等短路：

```sh
go test -count=1 ./backend/services/approval-audit-results/...
go vet ./backend/services/approval-audit-results/...
```

结果：通过。关键单元场景：

- `TestNormalizeComponentsRequiresExactReferenceOAProfile`
- `TestNormalizeRecordEnforcesKindSpecificExactSource`
- `TestAuditCopiesExactApprovalRevisionWithoutRereadingContract`
- `TestAmountOverrideRequiresReason`
- `TestEvidenceFailureLeavesNoPartialResult`
- `TestReplayReturnsBeforeDependencies`

### 2. 使用可丢弃 PostgreSQL 18 真库

启动全新 `postgres:18-alpine`，把 `TEST_DATABASE_URL` 指向该测试数据库。每个测试创建独立随机 schema、执行嵌入 migration，结束后删除 schema：

```sh
TEST_DATABASE_URL='postgres://…/disposable?sslmode=disable' \
  make approval-audit-results-core-acceptance
```

结果：通过，实际服务端版本为 PostgreSQL 18.4。关键集成场景：

- `TestPostgresExactSourceLifecycleHistoryIsolationAndImmutability`
- `TestPostgresDependencyFailureLeavesNoFacts`
- `TestConcurrentRecordReplayProducesOneResult`

### 3. 验证来源不会漂移

1. 登记 approval revision 1，总额 125；
2. audit revision 1 精确引用该 approval revision 1；
3. 把 approval 更正为 revision 2，总额 175；
4. 重新读取 audit，仍是总额 125、来源 revision 1；
5. 读取 audit 历史，首个完整快照仍能解释原来源和金额。

结果：通过。该证据直接关闭参考 OA 后端“审计时重新读取当前合同”的已知耦合风险，但尚未证明真实 Contract HTTP adapter。

### 4. 验证纠错、撤回和重新登记

1. audit revision 1 登记；
2. revision 2 以原因补录外部确认信息；
3. stale expected revision 1 被拒绝；
4. revision 3 显式撤回；
5. 相同 Project／Kind 以 expected revision 3 重新登记成 revision 4，沿用同一稳定 Result ID；
6. 历史顺序为 `recorded → corrected → withdrawn → reregistered`。

结果：通过。直接更新 revision、直接更新不匹配 current、物理删除 current 均被数据库拒绝。

### 5. 验证幂等、并发、隔离与故障原子性

- 同 Command ID／同摘要重放返回原 ID 和 revision；同 Command ID／不同摘要返回冲突；
- 8 个并发相同登记命令最终只有 1 个 current、1 个 revision、1 个 command、1 个 audit；
- 错 Tenant 查询返回 not found；
- Evidence 依赖失败后 current／revision／command／audit 都为 0；
- 失败的 stale、篡改和删除没有改变已提交事实数量。

结果：通过。

## C5-A01～A15 当前证据映射

| 验收项 | 本轮状态 | 证据边界 |
| --- | --- | --- |
| C5-A01 独立所有权 | 部分通过 | 独立 module、migration、schema 与 Store 已存在；独立进程／数据库账号／备份尚未验 |
| C5-A02 两种结果 | 通过（核心层） | 同 Project 的 approval／audit 各自唯一并可同时存在 |
| C5-A03 批复来源 | 部分通过 | 精确外部 Source Snapshot 与不漂移已验；真实 Contract／Settlement HTTP 凭证未验 |
| C5-A04 审计来源 | 通过（核心层） | 直接读取本应用 approval 历史 revision，单测确认不调用外部 Source resolver |
| C5-A05 精确金额 | 通过（核心层） | `int64`、固定三项、服务端合计、负数／重复／缺项／溢出和 DB 约束 |
| C5-A06 人工覆盖 | 通过（核心层） | 与精确来源不同必须有原因，并进入完整 revision |
| C5-A07 Evidence | 部分通过 | 精确 File ID／Version／Slot 快照与失败零写入；真实服务、恢复和多文件入口未验 |
| C5-A08 纠正与撤回 | 通过（核心层） | append-only、expected revision、不可变触发器、无物理删除、撤回后重登 |
| C5-A09 权限 | 未开始 | 无 HTTP／IAM 入口 |
| C5-A10 Project 完成视图 | 未开始 | 无 Completion Profile 实现 |
| C5-A11 完成命令 | 未开始 | 无 Project 完成命令 |
| C5-A12 最小组合 | 未开始 | 未进入 Suite Manifest／D1 |
| C5-A13 未选缺席 | 未开始 | 未进入装配 |
| C5-A14 装配恢复 | 未开始 | 无镜像／备份恢复 |
| C5-A15 回归与反耦合 | 部分通过 | 核心代码无共享表／跨库 ORM／同步双写；Level A／B、C1～C4 全回归待跑 |

## 发现的问题与收缩

1. 仅写“来源 ID”不足以防漂移；审计必须从自身历史库读取 approval 的指定 revision，并冻结金额和摘要。
2. 只在 Go 中计算合计不足以抵抗旁路写入；migration 增加 JSON 金额结构、币种与总额一致性检查，current 与 latest revision 再用 deferred trigger 对齐。
3. “撤回后重新登记”若生成新 ID 会割裂历史；本轮收缩为同一稳定 ID 追加 `reregistered` revision。
4. 本轮 resolver 是受控测试端口，不是网络适配器；因此 C5-A03／A07 只能标记部分通过，不能外推真实工作负载授权或 503 恢复。
5. 核心通过不等于应用可用。下一收缩应是版本化 HTTP + 目标 IAM 最终授权 + 真实 Project／Contract／Evidence adapters，不应提前做界面或 D1。

## 复现文件

- `backend/services/approval-audit-results/internal/results/model.go`
- `backend/services/approval-audit-results/internal/results/service.go`
- `backend/services/approval-audit-results/internal/results/postgres_store.go`
- `backend/services/approval-audit-results/internal/database/migrations/000001_approval_audit_results.sql`
- `backend/services/approval-audit-results/internal/results/model_test.go`
- `backend/services/approval-audit-results/internal/results/service_test.go`
- `backend/services/approval-audit-results/internal/results/postgres_store_integration_test.go`

## 下一轮最小问题

下一轮只回答：**受保护的 HTTP 命令能否在目标应用最终授权后，通过真实版本化端口取得 Project／Contract／Evidence 精确引用，并在 401／403／409／503 后保持零部分写入和原 Command 恢复？**

在该问题通过前，不进入 Workbench、D1、Project Completion Profile 或“C5 完成”结论。
