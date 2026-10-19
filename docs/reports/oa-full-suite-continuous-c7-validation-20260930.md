# OA 完整 Suite 连续 C7 验证报告

日期：2026-09-30  
状态：`passed`  
连续门禁：`C0～C7 = 8/9`  
适用范围：参考 OA 完整体第一阶段、同一 Tenant／同一 Project 连续旅程中的奖金计算、分配、应付建议与来源撤回切片

## 1. 验证问题

问题 A：C1～C6 已形成项目、主体、人员、Assignment、合同结算、收款集合和 Money 义务，但仍不能证明 Bonus Distribution 能在不复制这些事实、不共享表、不越权确认资金义务的前提下完成经营／生产奖金闭环。  
可验证结果 B：经营奖池只读取 C2 Settlement，生产奖池只读取 C3 Receipt Set，两者冻结 C1 Person／Assignment 与版本化 Overlay；Bonus 只提出应付建议，Money 独立接受并拥有 Obligation；来源撤回只追加 Bonus 历史，不级联改写 Money 已确认事实。

本轮不验证工资、税务、薪酬发放、自动付款、双 Tenant、经营／生产单类型最小组合、真人浏览器体验或 C8 最终只读装配。

## 2. 环境与基线

- 目标：公司 Ubuntu ECS，隔离 Compose project `omsbizfoundry-restore20`；
- 入口：仅回环地址 `http://127.0.0.1:18120`，链路为 Nginx → Gateway → IAM／目标应用；
- 长期容器：17 个，验证前后均为 `17/17 healthy`；
- Tenant：`6539cd13-7190-40f1-8bd5-57f9e7c9b968`；
- Project：`7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，输入 revision 11，追加一条精确 Settlement Link 后为 revision 12；
- 经营来源：Settlement `39ddaf20-035a-412f-a26e-eb0bcecdac04` revision 1，`50000 CNY`；
- 生产来源：Receipt Set `ea6cd02a-1812-4bc8-977e-15433ec91897` revision 1，`2,500,000` 分；
- 复用 C1 经营／生产 Person 与 Assignment，不新建 People 事实；
- Bonus 修复镜像：`v0.1.0-test.40@sha256:09c11367605d5a818917281e0e1725c0f381240dba7f959df83bce745927ec4a`，远端验证为 `linux/amd64`；
- Workload Policy 补齐 Project Workspace 对 `contract-ledger/settlement` 的精确引用类型。

正式证据目录：

```text
/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c7
```

`state-after-c7.json` SHA-256：

```text
64e17ebf3b5ffa2891a404a9d0fb4fa96b6fb168da4f837c94111fcc2cd76009
```

## 3. 科学验证步骤

验证器：`acceptance/d1-full-oa-journey/verify-c7`。

1. 校验 C1、C2、C5、C6 状态摘要与同一 Tenant／Project，直接复读并摘要唯一的 C3 Receipt Set；
2. 保存 IAM、Bonus、Money、Party、Project、People、Contract 的所有权计数基线，只给 C1 经营负责人补充 C7 最小能力；
3. 使用 stale Settlement revision 和超过 `10000` basis points 的分配提交两个负例，要求 `422` 且 Bonus 零写入；
4. 通过 Project 正式 API 追加精确 Settlement Resource Link，不改库；Receipt Set 自身版本化引用契约独立验证相同 Project ID；
5. 通过正式入口分别创建、发布并重放经营／生产奖池；按 `reference-oa-bonus-v1` 的整数公式 `base × rate / 10000 ÷ 1.065` 核对 half-up 结果；
6. 把既有经营 Person 显式投影为 Party Directory individual Party，再由 Money 建立本地 Party 与不可变 Person→Party 映射；两个操作均执行同命令重放；
7. 只停止 Money，通过 Bonus 提交应付建议，要求 Bonus 保存 `target_unavailable` revision 1，Money intake 计数不变；恢复 Money 后重放同一命令，建议推进为 `proposed`；
8. 由 Money 独立接受建议并创建唯一 Obligation；重放决定与 Bonus 同步均不得重复写入；
9. 撤回经营奖池，要求 Pool／Allocation 追加 revision 3，但 Money 已接受的 open Obligation 保持 revision 1；
10. 核对历史、精确 Workload Credential、所有权增量、25 个 HTTP 状态和 `17/17 healthy`；
11. 在已有 `passed` 检查点上重跑验证器，只复读事实和审计；16 个 HTTP 断言再次零差异，所有 ID／revision／增量不变。

## 4. 首轮发现的真实缺陷

### 4.1 Project 工作负载策略未允许 Settlement 引用

Project Workspace 通过正式 API 追加 Settlement Link 时需要读取目标引用，但其已登记 Workload Policy 只允许 `contract`，IAM 正确拒绝签发 `settlement:{id}` 凭证，入口返回 `503`。

修复：把 Project Workspace 的 `contract-ledger.reference.read` 资源类型从 `contract` 扩为 `contract + settlement`，并同步 Access Contract 断言。策略仍要求单 audience、单 action 和精确资源 ID，没有放宽为 tenant-wide 读取。

### 4.2 Bonus 把合法的契约加字段误判为服务不可用

Contract Settlement reference 已返回 HTTP 200，但 C6 的 Party Directory 切换为同一 schema version 增加了 `party_ids` 和 `party_references`。Bonus 的消费者解码器使用 `DisallowUnknownFields`，因此把合法的向后兼容扩展转换成 `REFERENCE_SERVICE_UNAVAILABLE`。

修复：跨应用引用仍严格复验已知的 schema version、source、Tenant、ID、resource type、revision、状态、金额和 Project Link，但允许同一版本添加未知字段；新增回归测试直接使用带 Party 投影的真实形状，防止生产者与全部消费者被迫锁步部署。

该缺陷是本轮最重要的架构反证：版本化契约不仅要约束写入和身份，还必须定义兼容演进规则。

### 4.3 验收器两处期望与稳定契约不一致

- 奖金金额最初误按简单 10% 断言；正式 Overlay 还要精确除以 `1.065`，正确结果是经营 `469,484` 分、生产 `234,742` 分；
- `target_revision=0` 会按 JSON `omitempty` 省略，验收器最初错误要求显式 `0`。

两处只修验收断言，没有修改已经正确计算和保存的业务事实。

### 4.4 C3 独立状态文件缺失

连续证据目录没有保留 standalone C3 state 文件。C7 没有猜测或重建来源，而是从 Money 自有数据库按 Tenant／Project 复读唯一 Receipt Set，核对状态、revision、金额、币种和摘要后继续，并在 C7 state 中显式记录该证据保留缺陷。它不影响本轮事实正确性，但最终报告必须把“旧阶段证据保留不完整”列为限制。

## 5. 结果

首次正式通过 25 个 HTTP 断言，完成态重跑 16 个断言，状态差异均为 0。

最终事实：

- 经营 Pool `37a30d94-b61c-43d5-921b-c8f07d011f81`：revision 3、`withdrawn`；revision 2 Allocation 金额 `469,484` 分；
- 生产 Pool `966ea321-2bb1-4a0f-8b61-10bdda3a150b`：revision 2、`published`，金额 `234,742` 分；
- People Person → Directory Party `5bb10590-4bf4-4a92-8e27-08dccc9de3c5` → Money Party `c27c0ced-e005-5821-a8ee-10c338234046`；
- Payable Suggestion `58d0db73-3201-4b3d-bd92-56866e73c906`：revision 3、目标 revision 2、`accepted`；
- Money Obligation `44823fe7-c6a4-4312-97c7-57ac6dc76053`：revision 1、`open/payable`，来源固定为经营 Allocation revision 2；
- 撤回 Bonus 来源后，Money Obligation 没有被删除、关闭或改写。

精确所有权增量：

| 所有者 | 新增或推进 | 必须保持不变 |
| --- | --- | --- |
| IAM | 12 个 C7 Grant | 不拥有业务事实 |
| Bonus Distribution | 2 Pools／5 revisions；2 Allocations／5 revisions；1 Suggestion／3 revisions；8 commands／5 audits | 不拥有 Project、Person、Party、Settlement、Receipt Set 或 Money Obligation |
| Money Flow | 1 Party projection；1 本地 Party；1 Person→Party mapping；1 Intake／2 revisions；1 Obligation／1 revision | Receipt Set 0 新增，既有 C3 来源只读复用 |
| Party Directory | 1 individual Party／1 revision | 不拥有 Person 或奖金分配 |
| Project Workspace | 1 Resource Link／1 command／1 history；Project 0 新增 | 不拥有 Bonus 或 Money 事实 |
| People & Groups | Person／Group／Membership 均 0 | C1 Person／Assignment 只读复用 |
| Contract Ledger | Contract／revision／Settlement 均 0 | C2 Settlement 只读复用 |

工作负载审计覆盖 Bonus 对 Project、两名 Person、Settlement、Receipt Set、Payable Suggestion submit/read，以及 Money 对 Directory Party 的精确凭证。没有使用共享表、跨库业务读取或 tenant-wide 工作负载业务凭证。

## 6. 边界判定

本轮对连续 C7 的判定为 `supported`：

- 经营／生产奖金能复用稳定来源和不可变快照，不需要复制 Contract、Money、People 或 Project 领域模型；
- `reference-oa-bonus-v1` 作为 Customer Overlay 冻结公式和取整，不把客户公式塞入公共核心；
- Person→Party 不是隐式按名称拼接，而是经过 Directory 投影和 Money 自有不可变映射；
- Bonus 只提出建议，Money 保留接受权与 Obligation 所有权；
- 目标停机只推进来源应用自己的可恢复状态，不制造半笔 Money 事实；
- Bonus 来源撤回不级联覆盖 Money 已确认事实，历史解释依赖冻结的来源 revision；
- 兼容性缺陷通过“宽读、严验已知不变量”修复，没有共享数据库、核心 fork 或 OA 专用现场补丁；
- 完成态重跑证明验证器和业务命令均可重入。

该结论只关闭连续 C7。阶段最终结论仍须连续通过 C8，再执行静态边界／发布兼容审计并冻结总报告。

## 7. 可复核证据

- `state-after-c7.json`、`state-after-c7-completed-rerun.json`：首次通过与完成态重跑；
- `ownership-before.json`、`ownership-after.json`、`ownership-delta.json`：数据所有权；
- `http-evidence.tsv`、`http-evidence-completed-rerun.tsv`：25／16 个入口状态与耗时；
- `workload-audits.tsv`：精确服务凭证；
- `business-*`、`production-*`：奖池、Allocation 和历史；
- `party-projection*`、`payable-party-map*`：显式 Person→Party 桥接及重放；
- `suggestion-*`、`money-intake-*`、`money-obligation-source.json`：停机恢复、独立决定、同步和无级联；
- `c7-summary.md`、`c7-summary-completed-rerun.md`：机器运行摘要。

下一步只从 `state-after-c7.json` 进入 C8；不得新建 Tenant 或 Project，也不得把已有分项 C8 证据冒充连续 C8 通过。
