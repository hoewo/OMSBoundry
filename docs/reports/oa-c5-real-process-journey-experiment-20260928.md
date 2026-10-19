# OA C5 真实独立进程业务旅程实验（2026-09-28）

## 结论

在隔离 D1 中，People & Groups、Project Workspace、Contract Ledger、Evidence Service、Approval & Audit Results、IAM、Gateway、Nginx 和 Workbench 以独立进程与独立业务数据库完成了 C5 真实业务旅程。验收得到 **74 个 HTTP 断言、0 个状态不匹配**，当前结论为：

`real-independent-process-journey-supported / dual-tenant-and-real-browser-pending`

这证明 C5 可以通过版本化连接器完成“真实合同与文件来源 → 批复 → 审计 → Project 检查点确认 → 来源纠正后 drift”的闭环，并在局部故障和权限变化下失败关闭。它不等于完整 OA Project 完成，也不替代 C6～C8、完整双 Tenant、真实浏览器、真实 Ubuntu／公网入口和第二客户证据。

## 问题 A → 可验证结果 B

**问题 A**：此前已有 Results 领域、HTTP、页面、Project Completion Profile、正式 D1 装配和联合恢复，但跨应用来源主要在同进程契约替身中验证，D1 入口只证明“能打开／能恢复”。因此无法确认真实独立 Contract、Evidence、Results 和 Project 能否在 Nginx → Gateway → IAM 下完成一条可恢复业务旅程。

**可验证结果 B**：隔离 D1 使用正式入口创建 People、Project、真实 Contract、真实 Evidence、Project Resource Link、approval 和 audit；随后显式确认 C5 检查点。依赖停机、权限撤销和来源纠正必须产生明确 403／503／drift，且不能产生部分写入或篡改已确认来源。

## 组合与直接依赖

本次选择：

- 公网边缘与入口：Nginx、Gateway、IAM、Unified Workbench；
- 业务应用：People & Groups、Project Workspace、Contract Ledger、Evidence Service、Approval & Audit Results；
- 明确不选：Money、Party、Business Engagement、Billing、Production。

People & Groups 不是 Results 自身直接依赖，而是 Project 建立业务／生产负责人 Assignment 和推进立项状态所需的显式依赖。Production 不参与本次组合。这修正了原 C5-A12 对最小组合的遗漏，但没有改变应用数据所有权。

## 实际旅程

1. 通过正式入口创建业务负责人、生产负责人和 Project，并建立两类 Project Assignment；
2. 将 Project 从 draft 推进至 active；
3. 在 Contract Ledger 中建立本方／对方主体、有效 Contract，并从页面完成签署激活；
4. 在 Evidence Service 中创建上传会话并完成文件版本；
5. Project 通过显式 Resource Link 冻结 Contract ID／revision；
6. IAM 邀请并激活 Results 操作者和无金额观察者，为两者签发 Project 精确 Scope 的能力；
7. 停止 Evidence 后登记 approval 返回 503 且 Results 仍为空；恢复后再停止 Contract，登记仍为 503 且零写入；两个依赖恢复后以同一 Command 成功一次并可幂等重放；
8. 创建 audit，默认冻结 approval revision 1；无金额观察者读取成功但金额字段脱敏，对其他 Project 返回 403，无 create 能力的写入返回 403；
9. Project 评估为 satisfied，显式确认把 Project revision 从 5 推进至 6，但 Project 状态保持 active；
10. 撤销 `approval-audit-result.correct` 后刷新凭证，纠正返回 403；恢复 Grant 并重新签发后纠正成功；
11. approval 纠正为 revision 2，历史仍为 `recorded@1 → corrected@2`，audit 继续冻结 approval revision 1，Project 当前评估显示 drift；
12. 停止 Results 后，Project GET 报告 unavailable；已成功 confirmation 的同 Command 仍由 Project 本地幂等记录返回，新的 confirmation 返回 503 且 Project 保持 revision 6；恢复 Results 后评估重新可用并仍显示 drift。

## 验证中发现并修复的问题

### Results 纠正错误使用冻结 Project revision

Project 确认会合法推进自身 revision。原 Results 纠正路径却把结果登记时冻结的旧 Project revision 当作“当前 Project 必须仍等于该 revision”的条件，因此在 Project 确认之后必然返回 `REFERENCE_INVALID`，无法形成预期 drift。

修复后，纠正命令仍复验同 Tenant、同 Project、active 状态与资源归属，但读取当前 Project revision；历史 result revision 保留原 Project 快照，新增 revision 冻结纠正时的当前 Project 快照。这样既不放松资源边界，也允许两个独立应用各自合法推进。

### 新确认把依赖不可用误报为业务条件未满足

Results 停机时，Project 的只读评估已正确返回 unavailable，但新 confirmation 原先落入通用 `completion_incomplete` 422。修复后，新确认在依赖不可用时返回 503；已经成功的同 Command 仍优先从 Project 本地记录回放，不依赖 Results 在线。

两项修复都新增了针对性单元测试，并由本次真实 D1 旅程复验。

## 直接证据

- 验收入口：`acceptance/d1-c5-results-journey/run-isolated verify`；
- 可复用运行门禁：`make d1-c5-results-journey-acceptance`（针对已启动隔离栈）；
- 运行摘要：`deployments/d1/.runtime/c5-journey-evidence/result.md`；
- HTTP 明细：`deployments/d1/.runtime/c5-journey-evidence/http-evidence.tsv`；
- 确认、drift、unavailable 三态证据：同目录的 `assessment-*.json`。

关键观测：

- approval 当前 revision 2，audit 当前 revision 1；audit 冻结 approval revision 1；
- Project confirmation 冻结 approval／audit revision 1，Project 当前 revision 6；
- 来源纠正后当前评估仍 satisfied，但 `drifted=true`；
- Results 停机时两个 requirement 均为 unavailable；
- 74 个 HTTP 状态断言全部匹配。

## 边界结论

本轮支持的“解耦平衡点”是：每个应用独占可写事实和历史，通过精确 ID／revision／digest 的版本化 reference-read 协作；Project 只保存自己的确认，不复制 Results 可写金额或状态；Results 不写 Project、Contract 或 Evidence。局部故障通过 503 和零写入暴露，成功幂等结果由事实所有者本地回放。

当前仍不支持：完整双 Tenant 交叉矩阵、账号停用专项、Results 真实浏览器动作收敛、多 Contract／Settlement 聚合、真实 `not_applicable`、两级在线审批、多轮复审、签章和自动生成下游建议。下一业务切片可进入 C6 责任边界审计，同时把双 Tenant／真实浏览器／真实 Ubuntu 保留为独立证据轴。
