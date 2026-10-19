# ADR-0004：合同 Instrument 与显式 revision 顺序

日期：2026-09-19  
状态：接受，用于 Contract Ledger M2

## 背景

M1 已把原合同激活为不可变 revision 1，但只有单个当前快照，尚不能表达补充协议、金额变化和确定生效顺序。若直接更新 Contract 表，历史会被覆盖；若建立无类型万能变更记录，金额语义、终止和结算又会混成无法验证的自由数据。

## 决策

- 把已经产生法律效力的原合同和后续文书建模为不可变 `Contract Instrument`；
- 每个生效 Instrument 按显式连续顺序产生一个新 revision；
- revision 是完整有效条款快照，Instrument 是导致变化的法律事实；
- revision 1 对应 `original_contract`，M2 首项只新增 `supplemental_agreement`；
- 补充协议金额行为限定为 adjustment、restatement 或 unchanged；
- 当前有效金额从最新 revision 读取，原始金额从 revision 1 读取，结算金额以后由独立结算事实提供；
- Instrument、命令、revision、Party Snapshot 和审计原子提交且全部不可变；
- 当前功能只允许顺序追加，不修改、插入或重新排列历史事实。

## 为什么不直接修改 Contract

Contract 根对象只负责稳定身份、生命周期和当前 revision 指针。直接更新金额会丢失原合同金额和变化依据，也不能回答某个 revision 为什么形成。把完整快照保存在 revision、把变化依据保存在 Instrument，可以同时支持当前查询和历史重放。

## 为什么不建设万能变更表

补充协议、终止、结算和技术性更正具有不同前置条件与结果。共享 Instrument envelope 不代表共享无约束负载。每类行为必须有独立命令和校验；本决策只接受补充协议及三种明确金额语义。

## 顺序规则

原合同顺序为 1。后续 Instrument 的生效顺序与产生的 revision 一致，必须连续消费当前 revision。同日文书依赖显式顺序，不依赖数据库时间或 ID。需要插入过去或纠正顺序时，使用后续受控更正能力，不改写既有事实。

## 后果

- 既有 active 合同需要无损回填原合同 Instrument；
- `contract_revisions` 从只允许 revision 1 扩展为 revision 1+，后续 revision 引用来源 Instrument；
- 查询必须区分原始金额、当前有效金额和未来结算金额；
- revision 会复制 Party Snapshot，换取独立、稳定且易于导出的历史快照；
- 后续终止、结算、关系和 Evidence 功能可以沿用 Instrument 时间线，但仍需各自规格，不能自动获得写权限或复用补充协议命令。

## 系统边界

本决策只影响 Contract Ledger 内部事实。不会创建 Money Flow 义务，也不定义跨系统 API 或事件。首次出现合同到收支的业务流动时，必须另行设计版本化快照契约、幂等接收、失败恢复和联调测试。
