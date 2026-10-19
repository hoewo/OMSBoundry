# OA 完整 Suite 连续旅程 C5 验证

日期：2026-09-30  
状态：`passed-in-current-reference-scope`  
总体阶段结论：仍为 `insufficient-evidence`

## 1. 问题与结果

问题 A：C0～C4 已在同一 Tenant／Project 形成客户、合同、经营资金与两类生产交付，但还没有证明批复／审计结果可以在不接管 Contract、Evidence 或 Project 事实的前提下继续加入，也没有证明“读取完成条件”与“确认项目检查点”是两个明确动作。

可验证结果 B：在公司 Ubuntu 的隔离完整 D1 栈中，复用 C1 Actor、C2 Contract／award Evidence 和 C4 Project，登记批复与审计；Results 只写自身结果与历史，Project 只在显式确认时追加一个完成检查点。随后更正批复而不覆盖旧版，审计继续冻结批复 revision 1，Project 把来源变化显示为 drift。

本轮通过。连续主门禁由 C0～C4 推进为 C0～C5（6/9，约 67%）。这不是 C6～C8、完整双 Tenant、真人浏览器全旅程或公网 HTTPS 的通过结论。

## 2. 环境与固定输入

| 项目 | 事实 |
| --- | --- |
| 目标 | 公司 Ubuntu D1 隔离恢复栈 `omsbizfoundry-restore20` |
| 入口 | 服务器回环 Nginx `http://127.0.0.1:18120` |
| 发布包 | `v0.1.0-test.37`，复用 `test.28` 固定镜像摘要，只更新验收驱动 |
| 长期服务 | 验收前后均为 17/17 healthy |
| Tenant | `6539cd13-7190-40f1-8bd5-57f9e7c9b968` |
| Project | `7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，revision 10 → 11，保持 active |
| Contract | `d784e608-1388-416d-bf24-b5321182fee4`，revision 1 |
| award Evidence | `40973231-38db-4f02-804b-5f02b99096b1`，Version 1 |
| 证据根目录 | `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c5` |

私有 actor 状态仍只保留在服务器 C1 私有状态文件中并保持 `0600`；报告不复制密码、Session、Token、私钥或环境文件。由于当前 SSH 用户的 `sudo` 需要交互密码，验收包在用户目录执行；只读复制所需输入后立即删除用户目录中的环境文件和 C1 私有状态副本，权威 C5 证据再以 root 所有权写回 `/opt`。该运维绕行没有改变运行容器或业务数据边界。

## 3. 连续执行步骤

1. 校验 C4 状态摘要、C2 状态摘要、Tenant、Project revision 10 和 C1 私有 actor 文件权限；复用 C1 的业务负责人和参与者，不创建新账号。
2. 只为 Results 补齐七项业务负责人能力和一项参与者只读能力；经 Gateway／IAM 为原 Project 重新签发资源级 Credential。
3. 仅停止 Evidence，使用固定批复 Command 写入返回 503，Results 仍为空；恢复后仅停止 Contract，同一 Command 再次返回 503，Results 仍为空。
4. 恢复 Contract 后以相同 Command 成功登记批复 revision 1，再等价重放并验证没有第二次提交。
5. 以批复 revision 1 为精确来源登记审计 revision 1；参与者读取两条结果时金额字段被省略，而不是伪造为 0。
6. GET Project Completion Assessment 返回 satisfied，但 Project revision 和 confirmation 行数均不变；证明读操作不暗中完成项目。
7. 显式 POST Completion Confirmation，把 Project revision 10 推进到 11，并保存两条精确来源 revision／digest。
8. 撤销业务负责人的 `approval-audit-result.correct` 后刷新 Credential，更正返回 403；恢复 Grant 并重新签发后，同一更正 Command 追加批复 revision 2。
9. 再读 Assessment：当前结果仍满足 Profile，但已确认来源集显示 drift；审计仍冻结批复 revision 1。
10. 仅停止 Results：Assessment 局部降级为 unavailable；已经成功的 confirmation Command 可以从 Project 本地 receipt 回放，新 Command 则 503 且 Project 仍为 revision 11。恢复后 Assessment 回到 satisfied 并继续显示 drift。
11. 复核四类精确 workload credential audit、不可变历史、数据库所有权增量和 17/17 健康，落盘 `state-after-c5.json`。
12. 完成态重跑跳过所有创建、撤权和停服，仅重读终态、Scope、增量和健康状态；业务 ID、revision 与增量保持不变。

## 4. 关键事实

| 事实 | ID／结果 |
| --- | --- |
| Approval Result | `6066c187-46e2-4d1c-95b8-879e7d607f50`，revision 2，recorded |
| Audit Result | `cf069904-37f2-4ff9-b734-e25c95194953`，revision 1，冻结 Approval revision 1 |
| Project Completion Confirmation | `e5920855-f34e-4a74-b44d-e2bfc55fc9ea`，confirmed Project revision 11 |
| 首轮 HTTP | 46 条，0 mismatch |
| 完成态重跑 HTTP | 14 条，0 mismatch；未再次注入故障 |

## 5. 数据所有权增量

| 所有者 | 本轮精确增量 |
| --- | --- |
| Identity & Access | capability grant +8 |
| Approval & Audit Results | Result +2；Revision +3；Command +3；Audit Event +3 |
| Project Workspace | Completion Confirmation +1；Command +1；History Event +1；Project 行 +0；Resource Link +0 |
| Contract Ledger | Contract +0；Revision +0 |
| Evidence Service | File +0；Version +0 |
| People & Groups | Person +0；Group +0 |
| Production Operations | Work Item +0 |

该增量与预期一致。Results 保存来源快照和自身追加历史；Project 只保存自己对精确来源集的确认，不复制批复、审计、合同或文件内容。

## 6. 架构判断

- “完成条件读取”可以保持纯只读；只有显式确认才允许 Project 写自己的检查点，因此 Results 不能越权把 Project 标成完成；
- 审计引用精确 Approval revision 1。Approval 后续 correction 形成 revision 2 时，审计事实不被级联改写，Project 通过 digest drift 暴露变化；
- 金额能力与结构读取能力分离。无金额权时省略金额字段，避免未知金额被页面或组合层误解为 0；
- Results 对 Project、Contract、Evidence 的读取都使用 request-bound、资源级 workload credential，目标应用继续作最终授权；
- Evidence／Contract 故障发生在 Results 写入之前，固定 Command 在恢复后只提交一次；Results 故障只影响依赖它的 Assessment，新 Project Command 失败关闭，已成功 Command 的本地回放不依赖远端；
- 当前没有出现必须共享表、跨应用外键、复制领域模型或长期 fork 公共内核才能承载参考 OA C5 的反证。

本轮没有证明所有客户的审批流程都可由同一个 Profile 配置完成。`reference-oa-c5-v1` 仍是版本化客户策略；新审批语义必须先区分稳定领域事实、客户 Overlay 和新应用责任，不能把任意流程塞进通用解释器。

## 7. 验证器与证据

- 验收入口：`acceptance/d1-full-oa-journey/verify-c5`；
- release bundle：`v0.1.0-test.37`，外层与内层 checksum 均通过；
- 权威状态：服务器 `c5/state-after-c5.json`；
- 完成态重跑：服务器 `c5/state-after-c5-completed-rerun.json`；
- 其他证据：plan、四个可恢复 checkpoint、依赖故障与 Results 故障结果、最终两个 Result 投影、两份历史、Assessment、所有权前后计数与 delta、首轮和重跑 HTTP TSV。

## 8. 仍未证明与下一步

- 同一数据集的 C6～C8；
- C5 完整双 Tenant／账号停用／真实浏览器矩阵；
- 公网 HTTPS、离机跨机器恢复、第二真实客户和 clean-HEAD 正式发布；
- C6 正式 connector 与跨进程撤回等独立补证轴。

下一批只从 `state-after-c5.json` 进入 C6，继续使用同一 Tenant／Project；保持一次一个阶段、一次只停止一个来源，不并行做浏览器或备份压力验证。
