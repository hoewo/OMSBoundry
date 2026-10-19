# OA 完整 Suite 连续旅程 v1

日期：2026-09-30  
状态：已接受的阶段一 P0 验收输入；运行证据待生成  
机器可读场景：[`acceptance/d1-full-oa-journey/scenario-v1.json`](../../../acceptance/d1-full-oa-journey/scenario-v1.json)

## 1. 问题与可验证结果

问题 A：C1～C8 已分别拥有大量直接证据，但历史验收大多各自创建 Project、Party、Person 和来源事实；把八份通过报告相加，不能证明一家公司的一项真实业务能够跨小应用连续完成。  
可验证结果 B：在完整十二应用 Suite 中，用同一 Tenant、同一 Project、同一客户 Party、同一组 Person／Assignment 和版本化 Overlay，连续形成 C1～C8 事实；每阶段保存输入／输出 ID、revision、digest、HTTP 证据、重放和纠错结果，并证明事实仍由各自应用拥有。

## 2. 核心连续性约束

1. 从 C1 创建后，后续 C2～C8 只能引用同一个 `project_id`，不得为方便调用历史验收而创建第二套 Project。
2. C1 冻结的客户 Party、经营负责人和生产负责人是后续默认来源；应用可以保存自己的业务角色快照，但不得复制可写 Party／Person 目录。
3. 每个跨应用引用必须携带稳定 ID 与 source revision；有 digest 的契约同时保存 digest。
4. 所有业务写入必须经过 Nginx→Gateway→IAM→目标应用正式入口。仅为创建“已有目标 API 尚不能表达的前置来源事实”才允许使用受清单约束的 seed；seed 必须写入事实所有者自己的数据库、单独列账且不得写消费应用。
5. 正常路径通过不够。每阶段至少包含一个拒绝、一个幂等重放或 stale revision、一个追加式纠错／恢复证据；不适用项必须在场景文件说明原因。
6. 统一工作台、Project 和只读投影只能组合和下钻，不得持久化第二份可写业务事实。
7. 旅程完成后对十五个应用执行静态边界门禁，并对各数据库账号／表行变化建立所有权账；任何跨库写入、共享业务表或目标应用被连接器越权确认都使验证失败。

## 3. 固定业务故事

参考客户是一家设计咨询企业。客户 Party 委托一个项目；经营负责人完成承接材料并形成合同与结算；财务登记票据、应收、收款和核销；生产负责人完成设计与咨询两类成果；办公室登记批复／审计结果；项目发生一笔生产成本和一项外委；经营／生产奖金分别从已确认结算和收款集合计算；最后由 Project 与 Workbench 只读组合资料、指标和当前人员工作。

该故事只用于把已经实现的 Base、类型化扩展和 `reference-oa` Overlay 连成一条可证伪链，不新增行业规则，不把 `ready` 冒充中标或签约，也不把 Project active 冒充所有业务已完成。

## 4. 分阶段步骤

### C0：完整组合预检

- 十二个业务应用在 Suite Manifest 中启用，只有 Nginx 暴露宿主端口；
- 17 个长期服务 healthy，十二个 schema current；
- 候选 Capability、Assignment Policy、Workload Policy 和 Suite Manifest 严格解析；
- 固化开始前的按应用表行计数、migration 和运行镜像摘要。

### C1：项目与责任建立

- Party Directory 创建客户 Party revision 1；People 创建经营／生产人员并关联 IAM Subject；
- Project 引用精确 Party revision，创建同一个 Project；
- 建立经营负责人、生产负责人和参与者 Assignment，完成立项；
- 重放创建命令只得到同一业务身份；相邻 Project Scope 与 stale revision 写入必须拒绝。

输出：`party_id`、三个 `person_id`、三个 `assignment_id`、`project_id`、当前 Project revision。

### C2：经营承接与合同链

- Evidence 创建承接材料 File Version；Business Engagement 在同一 Project 上设置 applicable、选择方式、引用材料并达到 `ready`；
- stale revision 不写历史；Evidence 暂时不可用时新鲜引用失败关闭，恢复后同一意图仅提交一次；
- Contract 独立建立合同、激活并确认 Settlement。Business Engagement 不直接写 Contract。

输出：`evidence_file_id/version`、`engagement_id/revision`、`contract_id/revision`、`settlement_id/revision`。

### C3：开票、应收与回款

- Billing 依据同一 Project／Contract／Party 快照登记已开票事实；
- Money 独立确认应收义务、记录收款并核销，形成同一 Project 的 Receipt Set；
- Billing 纠正追加 revision，Money 重放不重复；金额撤权时只脱敏，不返回伪 0；
- Project 经营摘要只读组合来源，任一来源停机仅降级对应分段。

输出：`invoice_id/revision`、`obligation_id/revision`、`cash_transaction_id/revision`、`allocation_id`、`receipt_set_id/revision`。

### C4：多类型生产交付

- 在同一 Project 创建 `design-v1` 与 `consultation-v1` 两个 Work Item；
- 使用 C1 的生产负责人 Assignment，推进各自类型阶段并引用 Evidence 成果；
- 两类型状态、frontier、revision 与历史独立；stale 写和跨类型成果错误失败关闭；
- 至少执行一次追加式返工或成果替换，不覆盖旧版本。

输出：两个 `work_item_id/revision`、类型、frontier 和成果 File Version。

### C5：批复、审计与项目完成检查点

- Results 依据 C2 Contract／Settlement 与 Evidence 登记批复，审计再精确引用批复 revision；
- 纠正与撤回只追加历史；金额／Evidence 能力分权；
- Project 以 `reference-oa-c5-v1` 纯读取评估，GET 前后事实摘要相同；显式确认只写 Project 自有检查点；
- 来源 revision 变化后旧检查点必须显示 drift，不静默改写。

输出：`approval_result_id/revision`、`audit_result_id/revision`、Completion evaluation digest 和 Project checkpoint revision。

### C6：成本与外委

- Cost & Outsourcing 在同一 Project 登记一项 Cost Entry 和一项 Outsourcing Engagement；
- Provider 使用 C1 Party 或另一个由 Party Directory 拥有的精确 Party revision；Contract／Evidence 只读引用；
- stale revision、错误 Project Scope、依赖停机均零写入；评分或成本纠正追加 revision；
- 付款建议经版本化 connector 交给 Money，由 Money 独立接受或拒绝，C6 不直接创建义务。

输出：`cost_id/revision`、`outsourcing_engagement_id/revision`、`assessment_id/revision`、`payable_suggestion_id/revision` 与 Money decision。

### C7：奖金计算与分配

- 经营奖池引用 C2 Settlement；生产奖池引用 C3 Receipt Set；两者使用 C1 的 Person／Assignment；
- `reference-oa-bonus-v1` 作为版本化 Overlay 冻结策略、整数金额、基点与 half-up 结果；
- 发布后不原地修改；至少一个奖池执行取代或撤回并保留历史；
- 应付建议交给 Money 独立接受／拒绝；源奖池后续变化不级联改写已确认义务。

输出：经营／生产 `bonus_pool_id/revision`、Allocation、策略与来源 digest、Money decision。

### C8：资料、统计与个人工作

- Project 资料页只读组合 C2/C3/C4/C5/C6 的 active Resource Link 和 Evidence metadata；下载另行授权；
- Workbench 只返回当前 Subject 对同一 Project 的工作项，邻近 Subject 为空；
- Money 指标按来源／Party／Currency 分区，金额撤权后为 null/redacted 而不是 0；
- 停止一个只读来源时 Workbench／Project 仍返回 200，并把该分段标记 unavailable；恢复后重新可读；
- C8 读取前后各事实所有者的行数和 canonical digest 不变。

输出：资料段状态、下载授权证据、工作项投影、指标投影、读取前后事实 digest。

## 5. 执行顺序与负载控制

1. 使用独立 Compose project 和已有恢复卷；主运行栈只做健康对照，不承载破坏性故障注入。
2. 一次只执行一个 C 阶段。阶段完成后落盘 `state-after-cN.json`、HTTP TSV 和数据库所有权摘要，再进入下一阶段。
3. 故障注入每次只停止一个来源容器，确认恢复后再执行下一项；不并行停止服务。
4. 每阶段允许中断和继续，但恢复时必须读取已落盘 state，并用服务端事实复核 ID／revision，不能仅信任本地状态文件。
5. C1～C8 全部通过后再执行一次联合备份；是否恢复该“旅程后备份”作为独立验证项，不与本轮同时增加资源压力。

## 6. 退出门槛

只有同时满足以下条件才把“连续 C1～C8 旅程”记为通过：

- 八阶段使用同一 `tenant_id` 与 `project_id`；
- 场景规定的正常、拒绝、重放／纠错和单来源故障证据全部存在且状态吻合；
- 每个输出 ID、revision 与 digest 能从事实所有者重新读取；
- Project／Workbench 读取没有改变来源事实摘要；
- 数据库所有权账与静态边界门禁没有跨应用写入；
- 验收结束时主栈健康，隔离栈没有容器重启风暴；
- 报告列出失败尝试、根因、修复版本和仍未证明的范围。

任一阶段失败时，最终报告保持 `insufficient-evidence`，但已完成阶段的事实和失败证据保留，不通过重建全新 Project 掩盖问题。
