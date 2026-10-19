# Read Projection Contracts v1

这里保存统一工作台等组合层只读消费的版本化投影契约。投影不转移业务事实所有权，也不允许组合层通过投影回写来源应用。

- `metric-projection.schema.json`：指标卡片的最小稳定结构；当前第一个来源是 Money Flow 的经营汇总。
- `examples/money-flow-operating-summary.json`：具有金额读取能力时的发布样本。

金额读取和指标结构读取是两个独立能力。只有结构读取能力时，来源仍可返回指标身份与范围，但 `value` 和 `source_digest` 必须同时为 `null`，避免用小值域摘要反推金额。
