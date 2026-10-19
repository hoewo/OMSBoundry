# OA C7 → Money 应付建议实验（2026-09-29）

结论：`real-cross-process-payable-suggestion-supported / formal-d1-connector-suggestion-recovery-pending`

## 问题 A → 可验证结果 B

**问题 A**：Bonus Distribution 已能发布经营／生产奖金 Allocation，但“奖金为何形成”和“公司是否确认应付”仍是两个不同业务事实。若 C7 直接写 Money 表、按人员姓名寻找财务主体，或把发布奖金等同于已形成应付，就会重新制造共享模型和无法恢复的跨库双写。

**可验证结果 B**：C7 以版本化建议冻结 exact Allocation 与 Person 快照；Money 通过显式、不可变的 Person → Money Party 映射独立受理并接受或拒绝。目标停机可恢复，接受恰好形成一个 Money 自有义务，拒绝形成零个，源奖金后续纠正不跨库删除财务事实。

## 约束与非目标

- Bonus 拥有建议原因、来源快照和同步历史；Money 拥有 intake、决定与 Obligation。
- 应用保持独立数据库，只经 HTTP 和共享 wire contract 协作，不共享表、ORM、领域对象或分布式事务。
- People Person 与 Money Party 不是同一对象；本轮只允许人工审核的 exact ID／revision／digest 映射，禁止按名称自动匹配。
- 本轮不验证正式 Nginx → Gateway → IAM D1 connector、联合备份中的建议引用、经营／生产单类型最小组合、完整双 Tenant、真实 Ubuntu、C8 或完整 OA。
- 实际付款、核销、工资、税务和自动代发不属于本轮。

## 实现结果

### C7 来源侧

- `BonusPayableSuggestionCurrent` 与不可变 revision 保存 `target_unavailable`、`proposed`、`accepted`、`rejected` 和 `withdrawn` 同步状态。
- 只允许 exact published Allocation revision 提出建议；请求冻结 Allocation digest、Pool／Project、Person、金额、币种、到期日与用途。
- 来源标识固定为 `bonus-distribution/bonus_allocation`，不借用 C6 的领域对象。
- 目标不可用时先提交本地历史；恢复后同一 Command 可重试，同摘要重放不重复。

### Money 目标侧

- Money intake 同时支持 C6 与 C7 来源，并以来源类型派生不同 Obligation `source_kind`。
- `payable-party-map` 运维命令把 exact People Person revision／digest 映射到 exact Money `individual` Party revision；映射记录不可更新、不可删除，并保存审核人和理由。
- 未映射 Person、摘要漂移、目标 Party 非个人或 Money Party revision 漂移均失败关闭。
- 接受 C7 建议产生唯一 `bonus_payable_suggestion`、`payable/open` Obligation；拒绝不创建 Obligation。

## 分批、低资源验证

为避免再次让本机 Docker 高 CPU，本轮没有启动完整 D1，也没有并行启动多套容器：

1. 先执行 Go 单元测试、静态 JSON／Shell 检查，Docker 保持 0 容器；
2. 使用单个临时 PostgreSQL 18.6，依次运行 Bonus 与 Money 的目标真库用例；
3. 最终验收仍只使用一个 PostgreSQL 容器，并只启动 Bonus、Money 两个本地进程；
4. 脚本退出时强制停止进程并删除容器，验收后 `docker ps` 为 0。

目标真库用例覆盖：

- published Allocation 提交、Money 初始不可用和恢复；
- exact Person → Party 映射；
- 接受与拒绝两条独立建议；
- 映射防篡改、未映射 revision 失败关闭；
- Bonus current／history 与 Money intake／Obligation 的持久化结果。

跨进程验收命令：

```bash
acceptance/c7-money-payable-flow/run-isolated
```

结果为 11 条 HTTP 断言、0 个状态偏差：

| 证据 | 结果 |
| --- | --- |
| Money 未启动时提出建议 | C7 保存 `target_unavailable`，无跨库半成品 |
| Money 恢复后重放原命令 | 提交成功且不重复 |
| Money 接受第一条建议 | 恰好 1 个 Money 自有 payable/open Obligation |
| Money 拒绝第二条建议 | 0 个 Obligation |
| 决定同步重放 | current／history 不重复 |
| Person 映射被修改 | 数据库拒绝 |
| 未映射 Person revision | 失败关闭 |
| 源 Pool／Allocation 后续撤回 | 已形成 Money Obligation 继续 open，不发生跨库级联 |

运行证据保存于 `deployments/d1/.runtime/c7-money-payable-evidence/`，包含 HTTP 结果、JSON 响应、SQL 摘要、进程清单和日志。

验收脚本的前两次执行分别暴露错误 API 路径和 Money 隔离库缺少 bootstrap 管理员，两次都在断言前或断言中安全失败并完成清理；第三次完成业务断言后暴露结果文档 Shell 变量边界问题。修正验收夹具后，全新一轮从空库返回码为 0。这里不把失败轮次描述为业务通过。

## 判断

C7→Money 的当前最小业务协作成立：来源应用可以只提出建议，目标应用独立确认财务事实；解耦没有要求复制 Party 模型，也没有用名称匹配掩盖跨域身份差异。C7-A13 与 C7-A14 在直接 HTTP 组合范围内通过。

仍不能给出完整 C7 或完整 OA 通过结论。下一轮需要独立验证正式 D1 connector 与建议引用的联合备份／空目标恢复；C7-A15 单类型最小组合、完整双 Tenant、真实 Ubuntu 和 C8 仍保持显式待办。
