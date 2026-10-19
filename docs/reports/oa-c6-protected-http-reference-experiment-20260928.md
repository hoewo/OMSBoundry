# OA C6 受保护 HTTP、IAM 与精确引用实验

日期：2026-09-28  
状态：`protected-http-and-reference-adapters-supported / human-workspace-and-composition-pending`  
整体结论：`insufficient-evidence`

## 本轮结果

- 总进度由 75% 上调到 **77%**；增加的是 C6 后端受保护入口和精确依赖边界，不是页面、付款建议、D1、恢复或真实 Ubuntu 完成度。
- `cost-outsourcing` 发布版本化 `/api/v1/cost-entries` 与 `/api/v1/outsourcing-engagements` 生命周期 API，并为 Assessment 提供追加写入和历史读取入口。
- 目标应用直接验证 Ed25519 IAM Credential，canonical audience 固定为 `cost-outsourcing`；伪造身份 Header 会在验证前删除。
- Tenant、Project、`cost_entry:{id}` 和 `outsourcing_engagement:{id}` Scope 由领域服务再次复验，不把 Gateway 或 HTTP middleware 当作最终授权者。
- Cost 与 Engagement 分别发布原生 amount、Evidence、correct、withdraw、history 和 assess Capability；身份超级管理员在 Customer Overlay 中保持空业务能力。
- 无金额能力时 Cost、Engagement 和冻结 Contract 金额字段为 `null`；服务端依据 Credential 与当前状态返回 `allowed_actions`。
- Project、Party、Contract／Settlement、Evidence adapter 使用单 audience、单 action、单精确资源工作负载凭证，包含超时、1 MiB 响应上限、一次 401 换票、严格状态／revision／Tenant 复验和 422／503 分类。
- Contract 引用增加稳定 `party_ids` 投影。C6 只有同时证明 Project 精确链接该 Contract revision、Provider Party 是合同参与方、币种和金额有效时才冻结 Contract 快照。
- Capability Manifest v1 兼容增加 `capability_namespaces`：一个 audience 可以显式发布多个原生动作命名空间；旧单 namespace 和默认 audience 回退保持有效。

## 直接验证

普通测试：

```text
cd backend/services/cost-outsourcing && go test ./...
go test ./acceptance/access-contract
cd backend/platform/identity-access && go test ./internal/capability
cd backend/services/contract-ledger && go test ./internal/httpserver
```

PostgreSQL 18.6 集成与 race：

```text
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:56788/costtest?sslmode=disable' \
  go test -count=1 -tags=integration ./backend/services/cost-outsourcing/...

TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:56788/costtest?sslmode=disable' \
  go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/...
```

以上命令实际通过。受保护 HTTP 真库矩阵覆盖：

1. Project 依赖停机返回 503，Cost current／revision／command／audit 均为 0；
2. 依赖恢复后原 Command 创建 revision 1；再次停机后同 Command 从本地 receipt 返回，不调用依赖；
3. 相同 Command 不同 payload 返回 409；stale expected revision 返回 409；
4. 无凭证、错 Audience、错 Tenant 返回 401；相邻业务动作、错误 Scope、缺金额或缺 Evidence 能力返回 403；
5. Project Scope 和精确 `cost_entry` Scope 均可在各自边界内读取，邻接资源 Scope 被拒绝；
6. 无 `amount.read` 的当前响应不包含金额和币种，具备能力后返回精确整数最小货币单位；
7. Contract 未出现在 Project 精确链接或 Provider Party 不在 `party_ids` 时返回 422 且 Engagement 零写入；修复来源后同 Command 成功一次；
8. 出站凭证调用实际记录了 Project、Party、Contract 和 File 的 audience／action／resource type／resource ID；
9. core 真库继续证明精确资源 Scope、撤回后拒绝、Evidence 删除需独立能力、历史防篡改和八并发单次提交。

## C6-A01～A18 更新

| 验收 | 状态 | 本轮证据 |
| --- | --- | --- |
| A01～A05 | 通过 | 延续核心／真库证据；Party HTTP adapter 补充精确外部复验 |
| A06 Contract 来源 | 通过（后端契约） | Project 精确链接、Contract revision／状态／币种／金额与 Provider Party 同时复验；无效 422、不可用 503、失败零写入 |
| A07～A09 | 通过 | Evidence HTTP adapter、外委纠错／评分及独立权限继续通过 |
| A10～A12 | 未开始 | Payable Suggestion、Money 付款投影和 Billing 票据投影尚未实现 |
| A13 权限 | 通过（本地后端矩阵） | IAM signature／Audience／Tenant、Project／资源 Scope、相邻动作、金额／Evidence 分权、服务端收敛；D1 撤权生命周期仍属 A17 |
| A14 状态拒绝 | 通过 | 延续核心／真库证据 |
| A15 最小组合 | 部分 | API 与四类 adapter 已组合；人类工作区、独立进程和模板最小组合未验 |
| A16 未选缺席 | 未开始 | 尚未进入 Suite Manifest／D1 |
| A17 装配恢复 | 未开始 | 尚无镜像、D1、Workbench 或联合恢复 |
| A18 回归反耦合 | 部分 | 普通全仓回归、访问契约和真库测试通过；D1 与 Level A／B 组合回归待后续 |

## 本轮发现并关闭的设计缺口

1. **笼统 Evidence action**：原核心使用 `cost-outsourcing.evidence.manage`，无法证明两个聚合的独立权限。现已拆成 `cost-entry.evidence.manage` 与 `outsourcing-engagement.evidence.manage`。
2. **只支持 Project Scope**：现已把精确资源 Scope 带入领域 Actor 并由服务复验；HTTP 不是唯一安全边界。
3. **清空 Evidence 可绕过权限**：纠正现在比较当前与请求引用，增加、替换或移除都需要对应 Evidence action。
4. **Contract 无 Provider 证据**：Contract reference-read 现显式投影参与 Party ID；C6 同时复验 Project 链接和 Provider 成员关系。
5. **Manifest 只能单 namespace**：兼容扩展 `capability_namespaces`，避免伪造 audience 前缀或拆出共享数据库的假应用。

## 明确未证明

- HTTP reference server 仍是 `httptest` 契约替身，不是四个真实独立进程；
- 尚无人类工作区、浏览器键盘／权限恢复、模板最小组合和可启动 server 配置；
- 尚无 Payable Suggestion、Money／Billing 只读投影；
- 尚无 release、镜像、Suite Manifest、Gateway／Nginx、Workbench、未选缺席、联合恢复和真实 Ubuntu；
- 尚未执行 C6 的账号停用、Grant 撤销／恢复和完整双 Tenant D1 矩阵；
- C7、C8 与最终整体报告未开始。

## 下一收缩

下一项不是直接铺 D1，而是先完成 **C6 人类工作区与本地独立进程最小组合**：使用同一版本化 API 和 `allowed_actions` 完成 Cost／Engagement／Assessment 的可操作页面，真实启动 Project、Party、Contract、Evidence 与 C6 进程，验证 409／422／503 输入保留和恢复。之后再实现 Payable Suggestion 与 Money／Billing 投影，最后进入 D1 装配、未选缺席与恢复。
