# OA C5 受保护 HTTP、IAM 与精确引用实验（2026-09-28）

## 判定

- 验证对象：Approval & Audit Results 的版本化 HTTP、目标 IAM 最终授权、Project／Contract／Evidence 精确引用与依赖故障恢复。
- 本轮判定：`protected-http-and-reference-adapters-supported / cross-process-composition-not-tested`。
- C5 判定：`partially-supported`。
- OA 完整体判定：继续为 `insufficient-evidence`。
- 总进度影响：粗略完成度由 64% 上调至 65%；剩余约 35%。

本轮关闭了“Results 只有领域内核，没有受保护网络入口”的最小缺口。它没有验证人类页面、Project 完成协作、Suite Manifest、Workbench、Gateway／Nginx、D1、联合恢复或真实 Ubuntu，也没有证明一个结果聚合多份合同。

## 问题 A → 可验证结果 B

**问题 A**：领域核心即使正确，如果 HTTP 入口不能复验 IAM 的 Tenant／Audience／Capability／Scope，或 Project／Contract／Evidence 引用只靠客户端声明，那么相邻角色、跨项目请求和依赖故障仍可能越权或留下部分事实。

**可验证结果 B**：受保护 HTTP 命令在目标应用最终授权后，只通过版本化引用端口和精确工作负载凭证取得来源；401／403／409／422／503 都失败关闭，依赖失败零写入，恢复后原 Command 只成功一次，成功 Command 的重放不再依赖外部服务。

## 范围与明确不做

本轮实现并验证：

- Results 原生 Capability Manifest 与参考 OA C5 Role Overlay；
- Ed25519 IAM Credential 的 Tenant、Audience、动作和 Project／Result Scope 复验；
- 登记、详情、历史、纠正和撤回的受保护 HTTP；
- 服务端金额脱敏、动态 `allowed_actions` 和客户端身份 Header 清洗；
- Project reference-read 的项目编号和活动 Resource Link 投影；
- Contract／Settlement reference-read 的类别、父合同、币种和精确金额；
- Results 对 Project／Contract／Evidence 的单 audience、单 action、单资源 Credential 请求；
- 401 换票一次、严格 JSON、响应大小、超时、类型／ID／revision／状态／Tenant 复验；
- PostgreSQL 18 中的失败零写入、恢复、幂等、冲突和审计来源链。

本轮不实现或不宣称：

- 人类工作区及真实浏览器点击；
- Project Completion Profile、完成视图和显式完成命令；
- 三个真实来源服务作为独立进程的跨进程组合；
- Suite Manifest、Workbench、Gateway／Nginx、D1 镜像和联合恢复；
- 多 Contract／Settlement 聚合或客户可编辑金额映射；
- C5 整体通过。

## 新增稳定边界

### 1. Project 证明来源归属

Results 不能只检查“Project 存在”和“Contract 存在”。Project reference-read 还返回当前活动 Resource Link。登记时必须找到目标 service、resource type、resource ID 和精确 source revision 一致的链接；已解除、缺失或 revision 不一致的来源被拒绝。

这条边界使“某合同属于哪个 Project”继续由 Project Workspace 拥有，不需要 Results 共享 Project 表，也不允许客户端自己声称关联。

### 2. 严格的单来源金额策略

`reference-oa-v1` 目前只接受一条 Contract／Settlement：

- `design|design_fee` → `design_fee`；
- `survey|survey_fee` → `survey_fee`；
- `consultation|consultation_fee` → `consultation_fee`；
- 其余两项为明确的零；
- 币种必须为 CNY；
- 金额必须能精确换算为分，亚分精度拒绝。

这是第一条可证伪策略，不是“OA 所有合同金额规则”结论。多来源聚合仍是业务未知，后续必须用新版本连接器和逐来源快照验证，不能由客户端先求和后伪装成一个合同。

### 3. 目标应用最终授权

Results 只接受 audience `approval-audit-results` 和原生 `approval-audit-result.*` 动作。Project Scope 可以操作该 Project 的结果；Result Scope 只允许目标结果。金额读取和 Evidence 管理是独立能力，目标应用在响应与命令入口分别执行脱敏和拒绝。

参考 OA Overlay 明确：身份超级管理员无隐式业务权；OA 业务管理员拥有完整动作；生产经理不能撤回；总经理、经营经理和财务仅具有读取、金额读取和历史读取。该 Overlay 只是客户角色映射，不进入 Results 稳定内核。

## 验证方法与实际结果

### 普通测试与所有者端口

执行：

```sh
go test -count=1 \
  ./backend/platform/identity-access/internal/capability \
  ./backend/services/approval-audit-results/... \
  ./backend/services/project-workspace/internal/projects \
  ./backend/services/project-workspace/internal/references \
  ./backend/services/contract-ledger/internal/httpserver
```

结果：全部通过。

该组测试证明：

- IAM Capability Catalog 能实际加载 Results Manifest，并拒绝错误 namespace／resource type；
- IAM Provider 拒绝错 Audience、Tenant 与 Resource Scope；
- HTTP resolver 请求精确 Project／Contract／Evidence Credential，并在 401 后只换票一次；
- Project 最小投影只暴露活动链接，不暴露已解除链接；
- Contract／Settlement 所有者端口返回约定的类别和金额；
- 十进制金额到整数分的换算保持精确，亚分值失败关闭。

### PostgreSQL 18 受保护入口矩阵

实际数据库：PostgreSQL `18.6`，可丢弃随机 schema。

执行：

```sh
TEST_DATABASE_URL='postgres://…/disposable?sslmode=disable' \
  make approval-audit-results-protected-http-acceptance

TEST_DATABASE_URL='postgres://…/disposable?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./backend/services/approval-audit-results/internal/httpserver \
  ./backend/services/approval-audit-results/internal/results
```

结果：两项均通过；race detector 未报告竞争。覆盖行为：

1. Project／Contract 来源全部 503 时登记返回 `503 REFERENCE_SERVICE_UNAVAILABLE`，四类事实均为 0；
2. 恢复后使用原 Command 返回 201，只形成一组 current／revision／command／audit；
3. 成功后再次让依赖停机，同载荷重放仍返回相同 Result ID，事实数量不增加；
4. 相同 Command 的不同载荷返回 `409 COMMAND_PAYLOAD_CONFLICT`，事实数量不变；
5. 无 Token、错 Audience、错 Tenant 分别返回 401；
6. 相邻只读角色写入、Project B Scope、缺 Evidence 能力和显式金额缺金额能力分别返回 403；
7. 无金额能力的详情同时隐藏 Result 与 Source 金额；有金额能力时两者均返回精确 `12345` 分；
8. Project 链接 source revision 不一致返回 `422 REFERENCE_INVALID`，事实数量不变；
9. stale expected revision 返回 `409 REVISION_CONFLICT`，事实数量不变；
10. Evidence 停机时审计登记返回 503 且已有 approval 不变；恢复后原 Command 只创建一条 audit；再次停机重放仍返回同一 audit；
11. audit 冻结 approval revision 1，不重新读取 Contract；
12. 工作负载调用记录包含精确 Project、Contract 与 Evidence audience／action／resource scope。

### 契约静态校验

Results API、Project reference、Contract reference 和 Settlement reference 四份 YAML 均由 YAML 解析器成功载入；Capability Manifest、Role Overlay 与 Workload Identity Policy 均通过 JSON 解析，IAM Catalog 测试进一步验证语义约束。

## C5-A01～A15 证据增量

| 验收项 | 当前状态 | 本轮新增证据与剩余边界 |
| --- | --- | --- |
| C5-A01 独立所有权 | 部分通过 | 独立 module、数据库边界和受保护进程入口已实现；独立交付／恢复未验 |
| C5-A02 两种结果 | 通过（后端） | approval／audit 通过同一保护入口分别形成事实 |
| C5-A03 批复来源 | 部分通过 | 版本化 HTTP adapter、活动 Project Link、Contract／Settlement 精确 revision 和金额已验；真实独立进程未联调，多来源未验 |
| C5-A04 审计来源 | 通过（后端） | HTTP 路径冻结指定 approval revision，Contract resolver 不参与 |
| C5-A05 精确金额 | 通过（后端） | 来源十进制精确换分、固定三项、服务端合计与权限脱敏已验 |
| C5-A06 人工覆盖 | 通过（后端） | 显式金额差异仍强制原因并进入 revision |
| C5-A07 Evidence | 部分通过 | 精确 HTTP 引用、503 零写入、恢复后原 Command 单次生效和成功重放已验；真实 Evidence 独立进程与多文件页面未验 |
| C5-A08 纠正与撤回 | 通过（后端） | 受保护入口、stale 409 和追加历史已覆盖 |
| C5-A09 权限 | 部分通过 | IAM、相邻角色、Project B、错 Tenant／Audience／Scope、金额与 Evidence 缺权已验；完整角色生命周期、撤权和页面收敛待 D1 |
| C5-A10 Project 完成视图 | 未开始 | 无 Completion Profile 实现 |
| C5-A11 完成命令 | 未开始 | 无 Project 完成命令 |
| C5-A12 最小组合 | 未开始 | 无 Suite Manifest／真实跨进程组合 |
| C5-A13 未选缺席 | 未开始 | 未进入装配 |
| C5-A14 装配恢复 | 未开始 | 无镜像／联合恢复 |
| C5-A15 回归与反耦合 | 部分通过 | 无共享表／跨库 ORM／同步双写，相关包回归通过；Level A／B 与 C1～C4 总回归未运行 |

## 可复现文件

- `backend/services/approval-audit-results/internal/auth/`
- `backend/services/approval-audit-results/internal/httpserver/`
- `backend/services/approval-audit-results/internal/references/`
- `backend/services/approval-audit-results/cmd/server/main.go`
- `backend/services/project-workspace/internal/projects/service.go`
- `backend/services/contract-ledger/internal/httpserver/reference_handlers.go`
- `contracts/access/v1/examples/approval-audit-results-capabilities.json`
- `contracts/access/v1/examples/reference-oa-c5-role-overlay.json`
- `contracts/access/v1/examples/workload-identity-policy.json`
- `arckit/tech/_shared/contracts/approval-audit-results-api.yaml`

## 下一最小问题

下一轮只回答：**在不提前装配 D1 的情况下，人类能否通过清晰页面登记、查看、纠正和撤回结果，并让 Project 以纯读取 Completion Profile 解释批复／审计 requirement，且 GET 零写入、来源 drift 可见？**

页面与 Project 完成协作通过后，再进入真实跨进程最小组合、未选缺席、正式入口和恢复；当前不得跳过这些证据直接宣称 C5 完成。
