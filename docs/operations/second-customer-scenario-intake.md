# 第二个真实客户场景采集与就绪判定

## 目的

这份手册用于取得第二个独立真实客户的最小脱敏业务证据，并把它转换为 `Customer Scenario Evidence v1`。目标不是尽快证明现有架构正确，而是让真实事实能够推翻、修正或支持当前的六层定制边界。

本流程不会收集客户账号、密码、AccessKey、身份证号、真实合同、完整人员名册、生产文件或其他秘密。若原始材料不能进入仓库，只记录经过批准的受控证据编号、存放位置和确认结论。

## 开始条件

开始访谈前确认：

- 已获准讨论并使用脱敏业务事实；
- 明确谁能确认业务含义，谁能确认最终验收；
- 在仓库外建立受控的原始材料位置；
- 从 [`customer-scenario-intake-v1.md`](../templates/customer-scenario-intake-v1.md) 复制一份工作表；
- 从 [`customer-scenario-evidence-v1.json`](../templates/customer-scenario-evidence-v1.json) 复制一份 JSON 草稿，敏感内容不得直接写回公共模板。

没有授权时停止采集，不用公开资料、旧系统字段名或我们的猜测补齐客户事实。

## 访谈顺序

### 1. 先固定问题 A 与可观察结果 B

询问：

1. 现在由谁在什么时点完成什么工作？
2. 哪个结果最容易观察到“问题已经解决”？
3. 哪些相邻问题本次明确不解决？

写入 `problem.statement`、`problem.observable_outcomes` 和 `problem.out_of_scope`。结果必须可观察，例如“被授权的经营人员能看到已确认应收及其剩余余额”，不能只写“提高效率”。

### 2. 只登记脱敏来源

每个重要判断至少关联一个 `source_evidence.evidence_refs`，例如批准后的访谈纪要编号、脱敏流程截图编号或受控资料编号。引用说明“在哪里可复核”，不复制原始客户材料。

客户确认引用 `customer_confirmation_refs` 只在最终结果已经由有权人员确认后填写；访谈开始、演示完成或我们自行判断都不能冒充客户确认。

### 3. 识别事实，不从页面和表名倒推模型

对每类业务事实询问：

- 它在业务上表示什么；
- 谁是唯一负责确认和纠错的系统或责任边界；
- 会经过哪些真实状态；
- 错了以后是更正、撤销、调整、冲销还是反向记录；
- 哪份来源材料支持这些结论。

分别写入 `facts[].owner`、`meaning`、`states`、`correction` 和 `evidence_refs`。不知道的内容保持未知，不用客户岗位名、页面按钮或数据库表名代替事实含义。

### 4. 选择最小应用集合

根据可观察结果只启用必要应用，并明确未启用应用：

- Contract Ledger：合同与合同 revision；
- Money Flow Ledger：已确认义务、实际资金、核销与余额；
- People & Groups：人员、编组、成员关系和能力画像；
- Evidence Service：File／Version 与受控访问；
- Project Workspace：项目上下文、Assignment 快照和稳定资源引用；
- Design Operations：设计生产阶段、成果与纠错。

写入 `applications.enabled` 和 `applications.disabled`。某个应用未启用时，它不能成为隐藏启动前提。

### 5. 每项客户差异只归入一层

| 层 | 判断问题 | 典型交付物 |
| --- | --- | --- |
| `stable_core` | 是否对所有目标客户都成立，并保护同一事实不变量？ | 公共 API、状态机、兼容 migration |
| `configuration` | 是否只改变名称、启用项、角色或稳定目录？ | 配置、manifest、定义目录 |
| `policy` | 是否用版本化输入计算输出，且可重放和纠错？ | 版本化策略及运行证据 |
| `typed_extension` | 是否是一类稳定行业业务，而非任意字段？ | 独立类型化模块或扩展 |
| `customer_assembly` | 是否只是选择应用、路由和连接关系？ | Suite Manifest、装配配置 |
| `customer_specific` | 是否只对该客户成立，且不应进入公共核心？ | 隔离的客户专项实现 |

每个 `differences[]` 只能选择一层，并写清 `requirement`、`rationale`、关联 `fact_refs` 和处置：

- `implemented`：已经有可复演交付物和证据；
- `accepted_deferred`：证据成立，但本轮明确延期；
- `rejected`：会破坏边界或不在产品范围；
- `needs_business_evidence`：含义、责任或纠错尚不清楚。

只有名称、公式设想或旧系统行为时保持 `needs_business_evidence`，不得为了让报告变绿而选择 `implemented`。

### 6. 连接器只在语义完整时登记

只有真实流程确实跨应用，且已明确以下五项时才写入 `connectors[]`：来源契约、目标契约、幂等身份、失败恢复、目标应用的最终确认。连接器只能隶属于 `customer_assembly` 或 `customer_specific` 差异，不能替目标应用确认业务事实。

### 7. 分两次完成门禁

先运行结构和语义校验：

```bash
go run ./acceptance/scenario-evidence/cmd/validate \
  /absolute/path/to/customer-scenario.json
```

再查看采集就绪度：

```bash
make scenario-evidence-readiness \
  SCENARIO=/absolute/path/to/customer-scenario.json
```

三个输出阶段的含义：

| 阶段 | 含义 | 允许的下一步 |
| --- | --- | --- |
| `draft` | 仍有访谈事实、来源、应用选择或差异归层缺口 | 继续访谈，不开始实现 |
| `evidence_ready` | 已具备人工业务判断和实现选择所需的最小证据 | 评审边界、实现并执行验收 |
| `final_ready` | 验收证据、客户确认和七项边界信号也已完整 | 才可把状态置为最终 `passed` 或 `failed` 并运行最终校验 |

`BOUNDARY_FINDING` 表示某项边界信号已经测量但非零。它不是“资料没填完”，因此不阻止 `final_ready`；它是必须保留的反证，最终状态只能为 `failed`，除非一次新的实际验收把该信号测为零。不得直接把原记录改成零来消除提示。

可在 CI 或交接时要求最低阶段：

```bash
make scenario-evidence-readiness \
  SCENARIO=/absolute/path/to/customer-scenario.json \
  SCENARIO_TARGET=evidence_ready
```

### 8. 最终结论前测量边界信号

完成实现和验收后，实际测量七项 `boundary_signals`：客户核心分支、客户数据库、共享业务表、客户业务 migration、跨应用内部读、现场核心补丁和隐藏强制依赖。`passed` 要求七项全部已知且为零；非零值本身仍是完整的最终证据，会由 readiness 输出 `BOUNDARY_FINDING`，应重新归层并重新验收，或者如实记为 `failed`。

同时填写：

- `acceptance_evidence`：实际运行、重放、纠错、隔离和恢复证据；
- `customer_confirmation_refs`：获准保存的最终确认引用；
- `limitations`：这份场景仍不能证明的事情。

最终 validator 通过后，生成与参考 OA 的比较材料：

```bash
make scenario-evidence-compare \
  BASELINE=contracts/scenario-evidence/v1/examples/reference-oa.json \
  CANDIDATE=/absolute/path/to/customer-scenario.json \
  > /absolute/path/to/customer-scenario-comparison.json
```

命令成功只表示两份最终证据可以按同一维度整理。必须逐项完成报告中的 `human_review_obligations`，尤其比较事实含义、唯一所有者、状态与纠错，复核进入 `stable_core` 的变化是否真对所有客户成立，并补充 v1 尚未编码的交付时长和核心改动范围。无论候选为 `passed` 还是 `failed`，工具都输出 `cross_customer_validated: false`；产品负责人另行记录“边界保持、需要修订或已被反证”的决定。

## 停止条件

遇到以下任一情况时停止实现，保留为 `draft` 或 `needs_business_evidence`：

- 无法确认事实所有者或纠错方式；
- 只能从旧页面、SQL 或角色名称猜业务含义；
- 要求业务应用直接写另一个应用的表；
- 要求用客户分支、共享表或现场补丁才能成立；
- 连接器没有幂等、失败恢复或目标确认；
- 未获准保存或引用客户材料。

## 本流程完成与产品目标完成的区别

手册、模板和命令完成，只表示第二客户证据“可以被安全收集”。只有取得真实脱敏事实、完成实现与验收、获得客户确认，并与参考 OA 比较后，才能评价当前六层边界是否跨客户成立。
