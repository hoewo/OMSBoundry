# OA 完整 Suite 连续旅程 C3 验证

日期：2026-09-30  
状态：`passed-in-current-reference-scope`  
总体阶段结论：仍为 `insufficient-evidence`

## 1. 问题与结果

问题 A：C0～C2 已证明同一 Tenant／Project 可以走到经营承接、合同、结算和材料，但尚不能证明开票、应收、回款能在不共享表、不复制来源事实的条件下继续接入，也没有证明 Money 的本地主体引用可以安全切换到 Party Directory。  
可验证结果 B：在公司 Ubuntu 的隔离完整 D1 栈中，沿用 C2 的稳定 ID，完成 Party 显式迁移、发票追加纠正、应收确认、回款、核销、Receipt Set、Project 类型化经营汇总、金额脱敏和单来源故障恢复，并证明各应用只增加自己拥有的事实。

本轮结果为通过。它关闭同一数据集的连续 C3 与 Money 完整 D1 Party 消费缺口，不代表 C4～C8、完整双 Tenant、真人浏览器或公网 HTTPS 已通过。

## 2. 环境与固定输入

| 项目 | 事实 |
| --- | --- |
| 目标 | 公司 Ubuntu D1 隔离恢复栈 `omsbizfoundry-restore20` |
| 入口 | 服务器回环 Nginx `http://127.0.0.1:18120` |
| 发布包 | `v0.1.0-test.34`，复用已固定的 `test.28` 镜像摘要；只新增验收驱动与文档 |
| 长期服务 | 验收前后均为 17/17 healthy |
| Tenant | `6539cd13-7190-40f1-8bd5-57f9e7c9b968` |
| Project | `7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，revision 7 → 10 |
| 客户 Party | `320fdfee-8c96-4f7c-aa50-58d3c79eaae1` revision 1 |
| 经营主体 Party | `af8827ab-7940-4110-be11-c359d23c7047` revision 1 |
| 证据根目录 | `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c3` |

私有 actor 状态继续保留在服务器的 C1 证据目录并保持 `0600`；报告不复制密码、Session、Token、私钥或环境文件。

## 3. 连续执行步骤

1. 校验 C2 state 摘要、Tenant、Project revision 7、Contract／Settlement 和两个 Party，再从正式 IAM 登录并为 Party、Money、Billing、Project 换取应用凭证。
2. 保存四个应用的写前计数，并执行 Money Party 迁移预检；只选择已经被历史业务事实引用的本地主体进入显式迁移计划。
3. 为 C1／C2 两个 Directory Party 建立 Money 自有兼容快照；对两个已有引用主体建立独立 Directory 目标。四条映射全部记录本地 snapshot digest、目标 Party ID／revision 和显式 command，不按名称或登记号自动合并。
4. Money 在所有已引用主体均有可审计映射后，追加一次 runtime transition，把读权威切换到 Party Directory；重新预检证明映射完整。
5. Billing 按精确 Project、Contract、Settlement 与 Party snapshot 登记发票 revision 1，再以追加纠正形成 revision 2、`3200000 CNY`；一次 stale expected revision 返回 409，历史仍恰好两条。
6. Money 使用已映射主体建立并确认 `50000.00 CNY` 应收，登记 `25000.00 CNY` 回款，核销 `25000.00 CNY`，再确认一个 Receipt Set。相同核销 Command 与 Receipt Set Command 重放保持同一业务 ID，只提交一次。
7. 复用 C1 participant，只授予 `billing-invoice.read` 而不授予 `billing-invoice.amount.read`；读取发票时金额与币种均为 `null`，没有把无权金额伪装成零。
8. Project 只追加 obligation、transaction、invoice registration 三条 Resource Link，revision 7 → 10；经营摘要组合已有 Contract 与三项 C3 来源，共四个类型化分段。
9. 只停止隔离栈 Billing：摘要仍返回 200，只有 invoice segment 为 `unavailable`，Contract／Obligation／Transaction 保持 `available`；Billing 恢复后四段重新全部可用。
10. 逐项核对 Project Workspace 为四个来源换取的 request-bound workload credential audit、数据库精确增量、40 个 HTTP 断言和最终 17/17 健康状态，再落盘 `state-after-c3.json`。

## 4. 关键事实 ID

| 事实 | ID／结果 |
| --- | --- |
| Invoice Registration | `7412663d-7c6c-4cf3-ac81-ec95d06d190a` revision 2，`3200000 CNY` |
| Receivable Obligation | `f69968f4-5ff1-4b8c-86da-436866792994` revision 1，50000／已核销 25000／余额 25000 |
| Cash Transaction | `6d53efba-8f6a-4b37-a42b-1feb1c74e571` revision 1，25000 |
| Allocation | `cb3232fc-cd1d-4f49-9ef3-360082d92748` revision 1 |
| Receipt Set | `ea6cd02a-1812-4bc8-977e-15433ec91897` revision 1，`2500000` 最小货币单位 |

## 5. 数据所有权增量

| 所有者 | 本轮精确增量 |
| --- | --- |
| Party Directory | Party +2；revision +2（两个既有引用主体的显式迁移目标） |
| Money Flow Ledger | local Party snapshot +2；Party mapping +4；runtime transition +1；Obligation／revision／create command／confirm command 各 +1；Transaction／command 各 +1；Allocation／command 各 +1；Receipt Set／item／command 各 +1 |
| Billing Invoice | Registration +1；revision +2；command +2；audit event +2 |
| Project Workspace | Project +0；Resource Link +3 |

该增量与预期完全一致。未发现 Project 复制来源事实、Billing 写 Money、Money 写 Party Directory、共享业务表、跨库外键或跨应用 ORM。

## 6. 架构判断

本轮支持以下当前范围结论：

- `Party + Contract + Billing + Money + Project` 可以在同一真实 D1 中通过版本化端口和稳定引用继续一条业务主线，不需要共享表；
- Party Directory 拥有主体当前档案，Money 仍拥有历史业务快照、映射和切换账，两者没有因统一主体而合并数据所有权；
- Billing 的登记与纠正是追加历史，Money 的应收、现金与核销仍由 Money 独立确认，Project 只读取类型化投影；
- IAM 的金额 Capability 可以让同一发票“可见但金额不可见”，目标应用返回缺失值而不是业务上错误的零；
- 一个来源停机只降低对应摘要分段，不要求把其他应用拖成同步调用链，也不产生业务写入；
- “稳定 Base + 显式装配 + 局部 Overlay”的方向在 C3 仍未出现必须共享表或长期 fork 公共内核的反证。

本轮同时暴露了解耦的真实成本：旧应用切换共享目录不能靠猜测匹配，必须保留来源快照、显式人工决定、不可变映射、运行时转换账和可恢复 checkpoint。这个成本属于迁移与装配，而不是取消应用边界的理由。

## 7. 验证器与证据

- 验收入口：`acceptance/d1-full-oa-journey/verify-c3`；
- release bundle gate：`deployments/d1/ops/test-release-bundle` 通过；
- 完成态重跑：复用原 plan／Party／fact／fault checkpoint，通过且业务 ID、revision、所有权增量不变；
- HTTP：40 assertions，0 mismatch；
- 权威状态：服务器 `c3/state-after-c3.json`；
- 其余证据包括 Party preflight／map／cutover、Billing history／redacted projection、Project 正常／故障／恢复摘要、所有权前后计数与 delta；
- 最终健康：17/17。

## 8. 仍未证明与下一步

- 同一数据集的 C4～C8；
- C3 完整双 Tenant／跨租户矩阵和真人浏览器连续点击；
- Settlement／应开未开等更宽业务口径；
- 公网 HTTPS、离机备份与跨机器恢复；
- 第二真实客户下 Base／Overlay 的长期升级稳定性。

下一批从 `state-after-c3.json` 进入 C4，继续使用同一 Tenant／Project，不创建新 Project 掩盖连续性问题；仍保持一次一个阶段、一次只停止一个来源。
