# OA C6 成本与外委领域核心实验（2026-09-28）

## 结论

- 实验判定：`domain-core-and-postgresql-supported / protected-http-and-composition-pending`。
- OA 完整体阶段判定仍为 `insufficient-evidence`。
- 总进度由 73% 上调到 **75%**；增加的是 C6 领域核心与真库证据，不是页面、IAM、D1 或现实业务全流程完成度。

本轮在独立 `backend/services/cost-outsourcing` Go module 和 PostgreSQL 18 数据边界内，实现并验证了 `CostEntry` 与 `OutsourcingEngagement` 两个聚合、追加式 Assessment、精确外部引用、Command 幂等、expected revision、Project 范围授权和失败零部分写入。没有引入宽 `FinancialRecord`，也没有直接写 Contract、Money、Billing、Party、Project 或 Evidence 数据库。

## 问题 A → 可验证结果 B

**问题 A**：C6 边界虽然已经从参考 OA 的宽财务表中拆出，但如果两个概念最终仍共用一张 nullable 表、使用浮点金额、原地覆盖、依赖失败留下半条记录，或为了方便直接写 Money／Billing，那么“一个应用、两个聚合”只停留在文档。

**可验证结果 B**：用独立模块、独立 migration 和 PostgreSQL 18 事务证明两个聚合具有不同 current／revision 表和命令；金额仅使用整数最小货币单位；创建、更正、撤回、评分、重放、并发、权限、精确引用和失败路径留下可复核的数据库证据。

## 实现范围

### Cost Entry

- `vehicle|accommodation|public_transport|other` 固定类别；
- ISO 币种与 `int64 amount_minor`；车辆里程使用整数米；
- 精确 Project 与 Evidence 快照；
- `recorded → corrected → withdrawn` 只追加完整 revision；
- current 投影只能随匹配 revision 前进，物理删除被数据库 trigger 拒绝。

### Outsourcing Engagement

- 精确 Project、Provider Party、可选 Contract 和 Evidence 快照；
- 委托范围、整数金额、币种和备注；
- Party、范围、金额、Contract 或 Evidence 的纠正形成下一完整 revision；
- 评分使用 `reference-oa-five-point-v1`，固定 1～5 整数分、理由、Actor、时间及所评 Engagement revision；评分更正追加 Assessment revision；
- Engagement 撤回不删除评分历史，也禁止继续评分。

### 事务与权限

- Tenant 内 Command ID + Actor + 请求摘要 + 响应类型共同决定重放；同 Command 不同请求返回冲突；
- 数据库 advisory lock 保证并发同 Command 只产生一份 current、revision、receipt 和 audit；
- 修改命令要求 expected revision；
- Actor 必须同时拥有原生 action 和精确 Project 范围；Evidence 引用另外要求 Evidence 管理 action；
- Project／Party／Contract／Evidence resolver 在事务前完成精确 revision、Tenant、状态和摘要复验；失败不写 current、revision、command 或 audit；
- 已成功 Command 的重放先读本地 receipt，依赖随后不可用也不影响结果。

## PostgreSQL 事实结构

| 责任 | 表 |
| --- | --- |
| Cost current／历史 | `cost_entries`、`cost_entry_revisions` |
| Engagement current／历史 | `outsourcing_engagements`、`outsourcing_engagement_revisions` |
| 评分历史 | `outsourcing_assessment_revisions` |
| 全应用幂等与审计 | `cost_outsourcing_commands`、`cost_outsourcing_audit_events` |

两个聚合共享命令收据和技术审计基础设施，但不共享业务 current／revision 表、字段集合或生命周期。这是“一个可部署应用、两个领域聚合”的实际证据。

## 执行证据

### 普通回归

```text
cd backend/services/cost-outsourcing && go test ./...
cd backend/services/cost-outsourcing && go vet ./...
make fmt-check vet test
```

结果：全部通过；根级测试已经把 `backend/services/cost-outsourcing/...` 纳入固定 `GO_PACKAGES`。

### PostgreSQL 18.6 集成实验

使用独立 `postgres:18.6-alpine` 容器和每个测试随机 schema：

```text
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/costtest?sslmode=disable' \
  go test -count=1 -tags=integration ./backend/services/cost-outsourcing/internal/costoutsourcing
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/costtest?sslmode=disable' \
  go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/internal/costoutsourcing
```

结果：通过。直接覆盖：

1. Cost 登记、幂等重放、请求冲突、纠正、过期 revision、不可变历史；
2. Engagement 登记、Party／Contract／Evidence 快照、纠正、撤回；
3. Assessment 首次评分和追加更正，撤回后评分拒绝；
4. 错 Project 被目标服务拒绝，错 Tenant 查找表现为 not found；
5. 缺 Evidence action、Evidence unavailable 均为零写入；依赖恢复后原 Command 只提交一次，之后依赖再次离线仍可本地成功重放；
6. 8 路并发同 Command 返回同一 ID，数据库只有 1 current、1 revision、1 command、1 audit；
7. revision UPDATE 和 current 物理 DELETE 被数据库 trigger 拒绝。

首轮 race detector 暴露 Actor 归一化会就地修改调用方 `ProjectIDs` slice；并发请求共享同一个 Actor fixture 时产生数据竞争。实现已改为先防御性复制再归一化，随后相同 PostgreSQL 18 并发矩阵在 `-race` 下通过。该缺陷和修复被保留为核心并发证据，而不是隐藏首轮失败。

## C6-A01～A18 当前状态

| ID | 本轮状态 | 证据／剩余 |
| --- | --- | --- |
| A01 独立所有权 | 通过（核心） | 独立 module、migration 和表；其他应用未新增 C6 可写表 |
| A02 双聚合隔离 | 通过（核心） | 两套模型、命令、current 和 revision；无宽表 |
| A03 精确金额 | 通过（核心） | Go `int64`、PostgreSQL `bigint`、非负约束；本轮没有金额运算，因而不存在客户端浮点合计 |
| A04 Cost 生命周期 | 通过（核心） | 登记／纠正／撤回、历史不可变、禁止物理删除 |
| A05 Party 来源 | 通过（核心端口） | 精确 ID／revision／Tenant／active／快照复验；真实 Party HTTP adapter 待下一轮 |
| A06 Contract 来源 | 部分 | 可选精确 Contract 校验与冻结已实现；真实 adapter、stale／停机矩阵待下一轮 |
| A07 Evidence | 通过（核心端口） | 精确版本、多引用、故障零写入、恢复后单次提交和成功本地重放；真实 adapter 待下一轮 |
| A08 外委纠错 | 通过（核心） | Party／范围／金额／Contract／Evidence 形成新 revision，旧快照保持 |
| A09 评分 | 通过（核心） | 五分制规则版本、理由、Actor、Engagement revision 和追加更正 |
| A10 义务建议 | 未开始 | 需要 Money 目标确认契约 |
| A11 付款读取 | 未开始 | 需要 Money 最小只读投影 |
| A12 票据读取 | 未开始 | 需要 Billing 最小只读投影 |
| A13 权限 | 部分 | action、Project、Tenant、Evidence 分权已验证；受保护 HTTP、Audience／Credential、金额脱敏和完整角色矩阵待验证 |
| A14 状态拒绝 | 通过（核心） | stale／withdrawn／冲突／越权均拒绝且无额外 revision |
| A15 最小组合 | 部分 | 内存 reference ports 已证明依赖集合；独立服务、真实入口和故障矩阵待验证 |
| A16 未选缺席 | 未开始 | 等 Suite Manifest／D1 |
| A17 装配恢复 | 未开始 | 等镜像、Workbench、入口和联合恢复 |
| A18 回归反耦合 | 部分 | 根级 vet/test 通过且无共享业务表；Level A／B、D1 和恢复回归待后续 |

## 本轮发现与收缩

1. “同一个应用”不要求同一张业务表。共享部署和技术 receipt，不共享领域 current／revision，能降低装配成本而不模糊事实。
2. Party／Contract／Evidence 只在成功命令冻结最小快照；后续来源变化不会改写旧 Cost／Engagement revision。
3. 当前 Project 只提供范围与快照，不据“存在成本／外委”推导完成。C6 是否进入 Completion Profile 继续保持未知。
4. 核心 Actor 已证明 Project/action 边界，但不能冒充正式 IAM。受保护 HTTP 必须继续完成 Audience、Credential、资源 Scope、金额字段和服务端 `allowed_actions`。
5. Payable Suggestion、付款和票据没有被“顺手”塞进本轮；它们仍分别等待 C6→Money 命令契约以及 Money／Billing 只读投影。

## 下一轮

下一最小收缩是 **C6 受保护 HTTP、正式 IAM 和真实精确引用 adapter**：

1. 版本化 Cost／Engagement／Assessment API 与错误分类；
2. `cost-outsourcing` Audience、原生 Capability、Project／资源 Scope、金额脱敏、Evidence 独立动作和 `allowed_actions`；
3. Project／Party／Contract／Evidence 精确工作负载凭证与 adapter；
4. 401／403／409／422／503、依赖恢复和零写入矩阵；
5. 完成后再进入人类页面、D1 装配、未选缺席和联合恢复。

本轮不得被描述为 C6 已完成；更不能据此宣称 OA 完整体已经通过。
