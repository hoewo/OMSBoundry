# OA 完整 Suite 连续旅程 C4 验证

日期：2026-09-30  
状态：`passed-in-current-reference-scope`  
总体阶段结论：仍为 `insufficient-evidence`

## 1. 问题与结果

问题 A：C0～C3 已在同一 Tenant／Project 走到合同、开票、应收和回款，但还没有证明生产交付可以在不复制 Project、People 或 Evidence 事实的前提下继续加入，也没有在该连续数据集上验证不同生产类型的阶段隔离、成果替换和返工。

可验证结果 B：在公司 Ubuntu 的隔离完整 D1 栈中，沿用 C1 的生产负责人和 C3 的原 Project，完成 `design-v1` 与 `consultation-v1` 两条不同生命周期，验证精确 Assignment、不可变 Evidence Version、错误类型阶段拒绝、stale revision 拒绝、依赖故障零写入、成果替换及重开后再完成；Project 与 People 全程零新增事实。

本轮通过。连续主门禁由 C0～C3 推进为 C0～C4（5/9，约 56%）。这不是 C5～C8、完整双 Tenant、真人浏览器全旅程或公网 HTTPS 的通过结论。

## 2. 环境与固定输入

| 项目 | 事实 |
| --- | --- |
| 目标 | 公司 Ubuntu D1 隔离恢复栈 `omsbizfoundry-restore20` |
| 入口 | 服务器回环 Nginx `http://127.0.0.1:18120` |
| 发布包 | `v0.1.0-test.36`，复用 `test.28` 固定镜像摘要，只更新验收驱动 |
| 长期服务 | 验收前后均为 17/17 healthy |
| Tenant | `6539cd13-7190-40f1-8bd5-57f9e7c9b968` |
| Project | `7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，保持 revision 10 |
| 证据根目录 | `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c4` |

私有 actor 状态仍只保留在服务器 C1 私有状态文件中并保持 `0600`；报告不复制密码、Session、Token、私钥或环境文件。

## 3. 连续执行步骤

1. 校验 C3 权威状态、Tenant、Project revision 10 和 C1 私有 actor 文件权限；经 IAM 登录并为保留的生产负责人签发 Project Assignment 约束的 Production 凭证。
2. 通过正式 Evidence 入口建立两个稳定 File ID：设计成果使用 Version 1；咨询报告先完成 Version 1，再为同一 File 追加 Version 2。
3. 在原 Project 上建立 `design-v1` 和 `consultation-v1`，两者分别绑定同一个 active `production_owner` Assignment；未创建第二个 Project 或复制 Person。
4. 向设计工作项提交咨询专属 `report` 阶段得到 422；以旧 expected revision 推进咨询阶段得到 409。两次拒绝均不增加工作项 revision、Command 或历史。
5. 设计路径把 `scheme`、`preliminary-design` 明确标记为不适用，分别登记并完成 `construction-drawing`、`construction-support`，最终 revision 8。
6. 咨询路径把 `scheme` 标记为不适用。仅停止 Evidence 后登记报告返回 503，工作项保持 revision 3 且没有成果；恢复 Evidence 后以同一 Command ID 重试，只提交一次 Version 1 成果。
7. 以 Version 2 显式替换咨询报告，完成 `report`，再因客户复核要求重开并重新完成；最终 revision 8，当前投影只显示 Version 2，历史仍保留 Version 1。
8. 复核两个工作项各 8 条不可变历史、精确 Project／Evidence workload credential audit、数据库所有权增量和 17/17 健康状态，落盘 `state-after-c4.json`。
9. 完成态重跑跳过已经提交的创建与故障注入，只重读最终投影、历史、Scope、增量和健康状态；业务 ID、revision 与增量不变。

## 4. 关键事实

| 事实 | ID／结果 |
| --- | --- |
| Design Work Item | `ea4b71df-f19e-4bed-a3ba-bc4f4c0e0bc9`，`design-v1`，revision 8，completed |
| Consultation Work Item | `bbf17888-fe60-4d65-9ed2-270be735c038`，`consultation-v1`，revision 8，completed |
| Design Evidence | `89ff681c-300a-4aef-b00e-ab4a50bca301`，Version 1 |
| Consultation Evidence | `89d33295-da79-446f-8ef2-7a5949f46136`，当前 Version 2，Version 1 历史保留 |

## 5. 数据所有权增量

| 所有者 | 本轮精确增量 |
| --- | --- |
| Identity & Access | Production capability grant +9 |
| Evidence Service | File +2；Version +3 |
| Production Operations | Work Item +2；Stage +6；Deliverable 记录 +4；Command +16；History Event +16 |
| Project Workspace | Project +0；Resource Link +0 |
| People & Groups | Person +0；Group +0 |

该增量与预期一致。Production 保存成果的稳定 File／Version／source revision 引用，不读取 Evidence 内部表；Project 与 People 没有为生产流程复制事实。

## 6. 架构判断

- 单一 Production Operations 所有者可以用代码拥有、版本化的类型定义承载不同生产生命周期，不需要为设计和咨询复制两个服务，也不需要通用流程解释器；
- “共享 Project 与负责人”不等于共享数据库：Production 通过精确 Project Assignment Snapshot 使用 People／Project 事实，目标应用仍作最终授权；
- 成果属于 Evidence，Production 只拥有成果在某工作阶段中的业务含义与不可变引用；报告更正通过新 Version 和显式替换完成，不覆盖旧字节；
- 跨类型阶段 422 和 stale revision 409 证明类型边界与并发边界都在写入前执行；
- Evidence 单来源不可用时只阻止需要校验成果的命令，没有产生半条 Deliverable、Command 或历史；恢复后不更换 Command ID 即可单次提交；
- 当前没有出现必须共享表、复制领域模型或长期 fork 公共内核才能承载参考 OA C4 的反证。

本轮没有证明任意未来生产类型都能无修改装入。新类型仍必须先形成稳定不变量、类型定义、权限和逐类型验收，不能把客户任意流程解释成配置。

## 7. 验证器与证据

- 验收入口：`acceptance/d1-full-oa-journey/verify-c4`；
- release bundle gate：`deployments/d1/ops/test-release-bundle` 通过；
- 权威状态：服务器 `c4/state-after-c4.json`；
- 完成态重跑：相同 Work Item／File ID、revision、所有权增量与 17/17 健康；当前保留的重跑 HTTP 证据为 10 条、0 mismatch；
- 其他证据：plan、Evidence 与 fact checkpoint、故障结果、两个最终投影、两份历史、所有权前后计数与 delta。

## 8. 仍未证明与下一步

- 同一数据集的 C5～C8；
- C4 完整双 Tenant／跨租户矩阵；
- 完整 Suite 真人浏览器从工作台进入两类生产工作并完成关键动作；
- 公网 HTTPS、离机跨机器恢复、第二真实客户和 clean-HEAD 正式发布。

下一批只从 `state-after-c4.json` 进入 C5，继续使用同一 Tenant／Project；保持一次一个阶段、一次只停止一个来源，不并行做浏览器或备份压力验证。
