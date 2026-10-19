# OA C7 真实来源与独立进程最小组合实验

日期：2026-09-28  
结论：`live-source-independent-composition-supported / human-workspace-and-d1-pending`

## 1. 本轮问题与结果

**问题 A**：C7 Bonus Distribution 已有领域核心、PostgreSQL、受保护 HTTP、正式 IAM 和消费 adapter，但 People Person 与 Money Receipt Set 仍只有目标契约和严格替身。若不运行真实来源进程，无法证明解耦后的六个应用能在不共享业务表的情况下完成经营／生产奖金。

**可验证结果 B**：真实 Identity & Access、Project Workspace、People & Groups、Contract Ledger、Money Flow Ledger 与 Bonus Distribution 以六个独立进程、六个独立数据库运行。Bonus 仅通过 IAM 工作负载换票和版本化 HTTP 引用读取 Project／Person／Settlement／Receipt Set，分别建立并发布经营与生产奖金池。

本轮不验证人类页面、Nginx／Gateway／Workbench、D1 Suite Manifest、联合恢复、C7→Money 应付建议或 C8 投影。

## 2. 实现增量

### 2.1 People Person 精确引用

- 新增 `GET /api/v1/references/persons/{person_id}`；
- 只接受 `people-groups.person.reference.read`；
- 只接受精确 `person:{id}` Resource Scope；
- 输出稳定 Person ID、reference code、display name、status 与 revision，不泄露 IAM subject mapping；
- 错 action、错资源、错 Tenant、未找到和依赖不可用分别失败关闭。

### 2.2 Money Receipt Set 事实

- 新增不可变 `receipt_sets`、`receipt_set_items` 与幂等确认命令；
- `POST /api/v1/receipt-sets` 只接受 Tenant Scope 的 `money-flow-ledger.receipt-set.confirm`；
- `GET /api/v1/references/receipt-sets/{id}` 只接受 `money-flow-ledger.receipt-set.reference.read + receipt_set:{id}`；
- 只允许未冲销的 CNY receipt allocations，冻结 transaction／allocation revision、金额、币种和状态；
- 金额以整数分保存，集合总额由数据库延迟一致性约束复验。

### 2.3 组合兼容性修复

- Project 数据库资源连接约束补充 `settlement`，与已经接受该类型的领域／HTTP 契约一致；
- Money 新增前向 migration `000021_receipt_set_total_trigger.sql`，修复 Receipt Set 延迟触发器在提交时解析错误 `NEW` 字段的问题；
- canonical workload policy 为 Bonus 固定四个最小来源权限，不授予宽 `view` 或静态来源令牌。

## 3. 可重复步骤

执行：

```sh
acceptance/c7-live-source-composition/run-isolated
```

夹具按以下顺序工作：

1. 构建 IAM、Project、People、Contract、Money、Bonus 和 workload key 工具；
2. 启动 PostgreSQL 18.6，创建 `identity`、`project`、`people`、`contract`、`money`、`bonus` 六个数据库；
3. 生成 Bonus Ed25519 工作负载密钥和仅含四个精确来源权限的运行策略；
4. 启动真实 IAM，并导入五份 Capability Manifest；
5. 启动 Project、People、Contract、Money，各自执行自己的 migration；
6. 仅作为验收前置数据，在每个来源应用自己的数据库内写入该应用拥有的最小事实；运行期没有应用跨库读写；
7. 由 IAM 管理员授予 Money Receipt Set 确认能力和 Bonus 的 Project 精确人类能力；
8. 经 Money HTTP 确认一个真实 Receipt Set；
9. 启动 Bonus，由其工作负载私钥向 IAM 交换单 action、单资源凭证；
10. 经真实 Project／People／Contract HTTP 引用建立并发布经营奖金；
11. 经真实 Project／People／Money HTTP 引用建立并发布生产奖金；
12. 校验 HTTP 状态、冻结快照、金额、revision、两个已发布 Pool 和六数据库隔离事实，再停止全部进程并移除容器。

## 4. 实际证据

最终一次执行返回码为 0。运行进程与数据库记录在 `deployments/d1/.runtime/c7-live-source-evidence/processes.tsv`，关键 HTTP 结果为：

| 请求 | 预期 | 实际 |
| --- | ---: | ---: |
| Money 确认 Receipt Set | 201 | 201 |
| Bonus 建立经营草稿 | 201 | 201 |
| Bonus 发布经营奖金 | 200 | 200 |
| Bonus 建立生产草稿 | 201 | 201 |
| Bonus 发布生产奖金 | 200 | 200 |

来源数据库计数依次为 People `2`、Project `1`、Contract Settlement `1`、Money Receipt Set `1`。Bonus 数据库为 `2|2|2|1`：两个 Pool、两个已发布、两种奖金类型、一个 Project。

经营与生产来源均为 `106500` 分；`reference-oa-bonus-v1` 以 1000 基点和精确 half-up 得到 `10000` 分。发布后 Pool 与 Allocation revision 均为 2，来源、Person 和 Assignment 快照仍为 revision 1。

本夹具聚焦真实来源进程的正常组合；依赖失败、恢复单次提交、成功重放零上游调用及 401 后单次换票由上一轮真实 Bonus PostgreSQL + 严格 HTTP 来源集成矩阵继续承担。两组证据合并关闭“真实提供者兼容性”和“依赖失败／恢复原子性”两个不同风险，不把尚未执行的六进程联合恢复冒充为已通过。

证据目录：`deployments/d1/.runtime/c7-live-source-evidence/`。该目录是本地运行产物；可重复命令和本报告才是长期验收入口。

## 5. 运行中发现并关闭的问题

1. PostgreSQL 官方镜像初始化阶段会短暂开放 Unix socket；原就绪检查可能撞上初始化服务器关闭窗口。夹具改为探测最终 TCP 服务器。
2. Money Receipt Set 延迟一致性函数用一个 `CASE` 引用两种 trigger record 字段，提交时报 `record NEW has no field receipt_set_id`。用前向 migration 的显式 `IF TG_TABLE_NAME` 修复。
3. Project 的领域／HTTP 已允许 Settlement link，但数据库 check constraint 未包含该类型。补充 migration 后真实持久化成立。
4. 验收夹具按旧 JSON 字段和旧 Money revision 字段取值，已按实际公开契约和当前 schema 修正；没有为通过测试而放宽业务不变量。

## 6. 边界结论

本轮支持以下判断：

- Bonus 不需要共享 Project、People、Contract 或 Money 表，也能完成两类奖金；
- Person 与 Receipt Set 可以由事实所有者发布窄、版本化、精确授权的引用端口；
- IAM 工作负载换票把来源权限限制为单 action、精确资源，Bonus 不需要持有静态来源 Token；
- 同一个 Bonus 内核可以复用稳定生命周期，同时由类型化来源和 Customer Overlay 保留定制“湿度”；
- 六个应用各自迁移和保存自己的事实，当前没有复制来源领域模型或引入跨库事务。

本轮不能支持以下判断：

- 操作者已经能通过页面完整使用 C7；
- C7 已进入 Nginx→Gateway→IAM 的正式 D1；
- 联合备份恢复、未选装缺席和重新选装已通过；
- 已发布奖金已经形成 Money 应付义务；
- C8 跨项目奖金投影或整个 OA 第一阶段已经完成。

下一轮应只处理 C7 人类工作区及直接正常／纠错／失败证据；D1、恢复和 Money 建议继续作为后续独立收缩。
