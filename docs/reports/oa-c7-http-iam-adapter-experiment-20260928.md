# OA C7 受保护 HTTP、IAM 与引用适配器实验

日期：2026-09-28  
结论：`protected-http-iam-and-consumer-adapters-supported / live-source-process-composition-pending`

## 本轮问题与结果

问题 A：C7 领域核心虽然已经能正确计算和持久化奖金，但没有正式网络入口、目标端最终授权、跨应用读取协议和服务端金额收敛，无法证明它能作为解耦应用安全运行。

可验证结果 B：Bonus Distribution 通过版本化 HTTP 和正式 IAM 接收经营／生产命令，只用请求绑定、精确资源的工作负载凭证读取 Project、Assignment、Person、Settlement 与 Receipt Set；依赖失败零写入，恢复只提交一次，成功重放不再访问上游，金额在 current 和全部历史视图中由服务端统一脱敏。

## 实现范围

- 新增 `cmd/server`、`cmd/workload-keyctl` 和模板／IAM 两种互斥运行配置；
- canonical audience 固定为 `bonus-distribution`，Ed25519 凭证严格校验 issuer、tenant、audience、时间、action 与 Project／Pool／Allocation Scope；
- 新增 Pool 列表／详情／历史、经营／生产草稿、发布、取代、撤回和 Allocation 历史入口；
- 新增 `bonus-pool`、`bonus-allocation` Capability Manifest 和经营／生产 Assignment Policy；
- 新增 Project、Assignment、People Person、Contract Settlement、Money Receipt Set HTTP resolver；
- resolver 使用 1 MiB 响应上限、严格 JSON、显式超时、一次 401 换票、404／422 无效引用和其余上游失败 503 分类；
- 生产来源同时冻结 Transaction 与 Allocation 的 ID／revision，不用 Allocation 代替 Transaction；
- 无 `bonus-allocation.amount.read` 时，Pool 金额、未分配金额、来源总额／条目金额和 Allocation 金额在 current／history 中均为 `null`；
- 外部传入的身份 Header 被清除，只有验签后的 Principal 能形成领域 Actor。

## 实际验证

普通检查：

```bash
cd backend/services/bonus-distribution
go test ./...
go test -tags integration ./...
go vet ./...
go build ./...
```

正式 IAM Catalog：

```bash
cd backend/platform/identity-access
go test ./internal/capability
```

一次性 PostgreSQL 18.6：

```bash
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/bonus_http?sslmode=disable' \
  go test -race -count=1 -tags integration ./internal/httpserver ./internal/bonus
```

以上全部通过。实际矩阵覆盖：

- 经营 Settlement 与生产 Receipt Set 两类草稿、发布、读取和历史；
- 缺失凭证、错 audience、错 tenant、错 Project、相邻角色、相邻资源拒绝；
- 无金额能力时 current、Pool history 与 Allocation history 同时脱敏；
- 首次 401 后只换票一次并成功，404 → 无效引用，超时 → 暂不可用；
- 上游不可用返回 503 且业务表零新增；恢复后相同意图创建一次；成功 Command 在上游再次离线时本地重放且上游调用数不增加；
- 未知资源 404、无效来源 422、stale expected revision 409；
- 工作负载交换命中精确 audience、action、resource type 和 resource ID。

## 证据边界

真实的是 Bonus Distribution HTTP／IAM、PostgreSQL 18、事务、脱敏和消费侧网络逻辑。Project／Contract 已有既有 reference-read 形状；People Person 与 Money Receipt Set 在本轮由严格 HTTP 模拟来源提供，并已发布版本化目标契约，但真实 People／Money 进程尚未实现并联调这些端点。因此本轮不能宣称“四来源真实跨进程组合”或 D1 已完成。

本轮没有实现人类工作区、Suite Manifest、Gateway／Workbench 路由、联合备份恢复、C7→Money Payable Suggestion、C8 只读投影、双 Tenant 或真实 Ubuntu。它们是后续独立验证项，不通过扩宽本轮结论补齐。

## 验收映射

- C7-A10：当前／历史／Allocation history 的服务端金额脱敏通过；
- C7-A11：引用无效／不可用时零部分写入通过；
- C7-A12：依赖恢复单次提交与成功重放零上游通过；
- C7-A06：经营／生产 action、Assignment role 与来源隔离在 HTTP／IAM 层补强通过；
- C7-A13～A17：Payable Suggestion、最小装配、未选缺席和联合恢复仍未通过。

## 下一收缩建议

优先实现 People Person 与 Money Receipt Set 的真实来源端点，并完成 IAM／Project／People／Contract／Money／Bonus 的独立进程最小组合；随后再做 Bonus 人类工作区。这样先消除“模拟来源”的最大证据缺口，再进入 D1、联合恢复和 Payable Suggestion，不把多个风险同时混入一个实验。
