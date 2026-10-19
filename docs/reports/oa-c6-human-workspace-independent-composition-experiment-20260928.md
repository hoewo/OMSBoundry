# OA C6 人类工作区与独立进程最小组合实验

日期：2026-09-28  
状态：`human-workspace-and-independent-process-composition-supported / formal-d1-and-payable-suggestion-pending`

## 结论

- C6 的 Cost Entry、Outsourcing Engagement 与 Assessment 已有可实际操作的人类工作区；页面以服务端 `allowed_actions`、金额可见性、当前状态和 revision 决定动作，不把隐藏按钮当作授权。
- IAM、Project、Party、Contract、Evidence、Cost & Outsourcing 和 PostgreSQL 以独立进程运行的最小组合已经通过。Money、Billing、应付款建议和 C7 未启动，也不是该组合的隐式前提。
- Evidence 或 Party 停机时新命令返回 503 且零写入；恢复后原业务意图成功。已经成功的 Command 在依赖再次停机时仍从本地幂等重放，不产生第二条 revision。
- 无金额能力时金额和修改动作同步收敛；只读凭证不能写入，错误 Project Scope 被拒绝。Project、Party、Contract 与 Evidence 的精确 ID／revision 快照仍由目标应用复验和冻结。
- 本轮把 OA 完整体架构可行性验证的粗略进度从 77% 推进到 **80%**。增加的是 C6 人类入口与本地真实多进程组合证据，不是正式 D1、联合备份恢复、应付款建议或真实 Ubuntu 完成度。

## 实现范围

新增或完成：

- `backend/services/cost-outsourcing/cmd/server` 与环境配置，使 C6 可作为独立 HTTP／PostgreSQL 进程启动；
- C6 概览、Project 载入、Cost／Engagement 页签、列表、详情、登记、纠正、撤回、历史和 Assessment；
- 页面只把访问令牌保存于当前标签页，401／403／409／422／503 都保留业务上下文并给出可恢复反馈；
- `acceptance/c6-cost-outsourcing-composition` 独立进程夹具与可重复运行入口；
- 稳定交互事实 `arckit/interaction/cost-outsourcing-workspace/`。

未进入本轮：

- Payable Suggestion 与 Money 的独立确认；
- Billing 票据只读投影；
- 正式 D1 Suite Manifest、Workbench、Gateway／Nginx 入口与联合恢复；
- Project Completion 适用性；
- C7 奖金与真实 Ubuntu／公网环境。

## 独立进程拓扑

| 进程 | 本地端口 | 责任 |
| --- | ---: | --- |
| IAM fixture | 19170 | 签发 C6 用户凭证与精确工作负载凭证 |
| Project fixture | 19171 | 复验精确 Project revision |
| Party fixture | 19172 | 复验 Provider Party revision |
| Contract fixture | 19173 | 复验 Contract、Project 与 Provider Party 关系 |
| Evidence fixture | 19174 | 复验精确 File Version |
| Cost & Outsourcing | 19175 | C6 页面、API 与最终业务授权 |
| PostgreSQL 18.6 | 55466 | C6 独立事实库 |

这些进程只通过版本化 HTTP 与 PostgreSQL 边界协作，没有共享业务表、跨应用 ORM 或进程内替身。fixture 只提供相邻系统的公开契约事实，不代替 C6 真实状态机和数据库。

## 实际验证

执行：

```text
cd backend/services/cost-outsourcing && go test ./...
go test ./acceptance/c6-cost-outsourcing-composition/...
sh -n acceptance/c6-cost-outsourcing-composition/run-isolated
acceptance/c6-cost-outsourcing-composition/run-isolated
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -count=1 -tags=integration ./backend/services/cost-outsourcing/...
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/...
make fmt-check vet test
```

全部通过。独立进程脚本保存 12 个 HTTP 结果，实际覆盖：

1. 工作区与脚本均返回 200；
2. Evidence 停机时 Cost 登记返回 503，列表仍为空；恢复后同一业务意图返回 201；
3. 已成功 Cost Command 在 Evidence 再次停机时返回同一 ID／revision；
4. Party 停机时 Engagement 登记返回 503，恢复后返回 201，并冻结四类精确来源；
5. Assessment 返回 201；
6. 金额脱敏读取返回 200，只读写入和错误 Project Scope 分别返回 403；
7. 进程清单不包含 Money、Billing、payable 或 bonus。

真实浏览器又检查了概览、Project 读取、Cost／Engagement 页签、详情、金额、Evidence／Party／Contract 冻结引用和 Assessment 入口。检查期间发现并修复两项真实页面问题：Content Security Policy 最初阻止工作区脚本执行；已有 Assessment 时页面仍错误提交 `expected_assessment_revision: 0`。修复后页面从最新 Assessment 计算 expected revision，脚本语法与完整测试重新通过。

## 边界判断

该结果支持“C6 作为一个应用、内部两个严格聚合”的当前最小边界：两类记录可以共享无业务查询含义的技术幂等与审计设施，同时继续拥有各自 current、revision、动作和权限。真实运行没有要求复制 Party／Contract／Evidence 模型，也没有要求 Money 或 Billing 同时启动。

它仍不证明 C6 已经完整交付。下一收缩应把同一发布物装入正式 D1，验证 Suite Manifest、Workbench、Nginx→Gateway→IAM、未选缺席、独立数据库／工作负载身份和联合空目标恢复；Payable Suggestion 继续作为随后独立的跨域业务切片，不与交付装配混做。
