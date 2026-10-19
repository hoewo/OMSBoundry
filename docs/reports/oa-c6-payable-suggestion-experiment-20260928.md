# C6 → Money 应付建议跨应用实验

日期：2026-09-28  
结论：`real-cross-process-payable-suggestion-supported / formal-d1-complete-dual-tenant-real-ubuntu-pending`

## 1. 要解决的问题

问题 A：Cost & Outsourcing 已经拥有成本和外协发生原因，但如果它直接建立 Money 义务或写 Money 数据库，就会把“为什么应付”和“是否由当前账簿承担、怎样形成义务”重新耦合；如果只保存一段备注，又无法可靠恢复、重放和追溯跨应用决定。

可验证结果 B：C6 从精确来源 revision 提出版本化应付建议，Money 独立接收并接受或拒绝；接受恰好创建一条 Money 自有 Obligation，拒绝不创建 Obligation。两个应用使用独立数据库、独立事务和可重放命令，目标停机、请求重放和部分失败都不改写来源事实，也不需要共享表或分布式事务。

## 2. 责任边界

| 责任 | Cost & Outsourcing | Money Flow Ledger |
| --- | --- | --- |
| 外协／成本发生原因 | 拥有 | 不拥有，只冻结来源快照 |
| 应付建议当前状态 | 拥有 `target_unavailable / proposed / accepted / rejected / withdrawn` 投影 | 不直接写 C6 投影 |
| 建议受理与决定 | 只提交、查询和同步 | 拥有 `pending / accepted / rejected / withdrawn` intake |
| 应付义务 | 只保存目标返回的 ID／revision 引用 | 接受时创建并独占 Obligation |
| 主体映射 | 提交 Directory Party 快照 | 使用 Money 来源自有转换账解析本地账簿 Party |
| 票据、实际付款与核销 | 不拥有 | 义务之后仍由 Money 自身生命周期管理；票据仍归 Billing |

稳定契约为：

- [`PayableSuggestion.yaml`](../../arckit/tech/_shared/models/PayableSuggestion.yaml)；
- [`PayableSuggestionIntake.yaml`](../../arckit/tech/_shared/models/PayableSuggestionIntake.yaml)；
- [`payable-suggestion-api.yaml`](../../arckit/tech/_shared/contracts/payable-suggestion-api.yaml)。

## 3. 实际拓扑

实验启动两个真实 HTTP 进程和两个 PostgreSQL 18 数据库：

```text
Cost & Outsourcing + C6 DB
          │ versioned HTTP submission / query
          ▼
Money Flow Ledger + Money DB
```

C6 先在本地保存 `target_unavailable` revision，再尝试提交到 Money；Money 成功持久化 pending intake 后，C6 进入 `proposed`。Money Actor 随后独立接受或拒绝，C6 再显式查询和同步目标决定。跨库之间没有共同事务、内部表读取、ORM 复用或同步双写。

## 4. 验证矩阵

| 编号 | 风险假设 | 实际检查 | 结果 |
| --- | --- | --- | --- |
| P1 | 建议可能脱离来源当前事实 | 建议固定来源 application、type、ID、revision、SHA-256 digest、Project、Party、金额、币种、到期日和用途 | 通过 |
| P2 | Money 停机可能让 C6 丢失业务意图 | 停止 Money 后执行同步得到 503；C6 当前建议和 history 保持不变 | 通过 |
| P3 | 恢复后重试可能重复受理 | Money 恢复后以相同建议摘要重试，只保留一个 intake | 通过 |
| P4 | 接受可能重复生成义务 | 接受后数据库为 `1|payable|open|cost_outsourcing_payable_suggestion|cost-outsourcing|outsourcing_engagement|...|1`；决定和 C6 同步重放不新增义务 | 通过 |
| P5 | 拒绝可能仍残留义务 | 第二个独立 Engagement 的建议被拒绝，SQL 结果为 `0|2`：零 Obligation、两条 intake revision | 通过 |
| P6 | C6 可能替 Money 决策 | C6 只读取 Money intake 并把 accepted／rejected 与目标 revision 写入自身新 revision；没有决定入口或 Money 表写入 | 通过 |
| P7 | 接受写义务成功、写 intake 失败会永久重复 | 单元故障注入在 Obligation 已提交而 intake 未更新处中断；用确定性子命令恢复后只得到同一 Obligation，随后命令重放返回 revision 2 | 通过 |
| P8 | 相同来源 revision 可能有多条活动建议 | PostgreSQL 唯一约束限定同 Tenant、来源 type／ID／revision 只有一条非 withdrawn 建议 | 通过 |
| P9 | 历史可能被改写 | C6 suggestion revision 与 Money intake revision 的数据库防篡改用例拒绝 UPDATE | 通过 |
| P10 | Directory Party 可能按名称猜测 | Money 只接受来源自有、精确的 Party 转换映射；无映射返回 422 | 通过 |
| P11 | 跨应用必须依赖共享数据库或分布式事务 | 两个进程连接不同数据库，验收只经版本化 HTTP 完成接受、拒绝、故障和恢复 | 通过 |
| P12 | 拒绝决定重放可能追加重复历史 | 相同拒绝命令返回同一 rejected intake；revision 数保持 2；C6 相同同步命令返回同一 revision 3 | 通过 |

## 5. 实际执行

普通 Go 检查：

```bash
go test ./backend/services/cost-outsourcing/internal/costoutsourcing \
  ./backend/services/money-flow-ledger/internal/payablesuggestion \
  ./packages/payable-suggestion-contract
```

结果：通过。

PostgreSQL 18 集成检查（使用隔离测试数据库，不指向业务库）：

```bash
TEST_DATABASE_URL='postgres://postgres:测试密码@127.0.0.1:测试端口/postgres?sslmode=disable' \
go test -tags=integration \
  ./backend/services/money-flow-ledger/internal/payablesuggestion \
  ./backend/services/cost-outsourcing/internal/costoutsourcing -count=1
```

结果：两个 package 均通过；覆盖接受、拒绝、重放、来源唯一性、历史防篡改、目标不可用和同步恢复。

真实跨进程验收：

```bash
./acceptance/c6-money-payable-flow/run-isolated
```

结果：全部 HTTP 期望码与实际码一致。证据写入 `deployments/d1/.runtime/c6-money-payable-evidence/`，包括接受和拒绝两条 journey 的 JSON、HTTP 表、SQL 摘要与结果说明。

## 6. 实验中暴露并关闭的问题

1. 早期 reference template 的 Actor kind 与真实凭证不一致，已统一为受信模板 Subject，避免测试成功依赖伪造 Header。
2. PostgreSQL 容器第一次启动时端口已监听但服务尚未完全 ready，出现一次 unexpected EOF；验收改为数据库 readiness 成功后再留出稳定窗口。这是测试编排竞态，不是业务失败。
3. 首版验收只走接受路径，无法证明拒绝不会创建义务；本轮增加真库和双进程拒绝路径并得到 `0|2` 证据。
4. 编写拒绝用例时复用了同一来源 revision，数据库唯一约束按设计拒绝第二条活动建议；用例改为第二个真实 Engagement，而不是放宽不变量。

## 7. 本轮明确没有证明

- 尚未把 C6→Money connector 装入完整 D1 Suite Manifest 并经 Nginx→Gateway→IAM 正式入口执行同一 journey；
- 尚未验证两个 Tenant 的交叉拒绝矩阵和账号停用／恢复；
- `withdrawn` 已有领域与 HTTP 实现，但尚未加入真实跨进程验收；
- 尚未补齐 Money／Billing 面向 C6 工作区的只读付款、核销和票据投影；
- 尚未在公司私有 Ubuntu 或个人公网试验田运行同一不可变发布物；
- 因而本轮不能把完整 C6、Complete Suite 或 OA 完整体标为通过。

## 8. 对架构可行性的阶段结论

这次实验证明连接器并不必然导致脆弱的同步链：只要建议本身是来源拥有、版本化、可重放的事实，目标应用独立决定并拥有结果事实，双方分别保存当前投影和不可变历史，就能在无共享表、无分布式事务的条件下恢复目标停机与部分失败。

它同时保留了定制“湿度”：客户差异可以改变谁有决定能力、页面措辞、建议适用条件或选装组合，但不能越权改变 C6 的发生原因所有权、Money 的义务所有权和精确来源身份。该结论只覆盖应付建议这一条业务切片；是否能承载完整 OA 仍要继续由 C7～C8、完整 D1、双 Tenant 和真实 Ubuntu／HTTPS 证据共同回答。
