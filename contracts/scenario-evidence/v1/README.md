# Customer Scenario Evidence Contract v1

本目录把产品边界验证所需的场景账目固定为机器可读契约。它不替代客户访谈、业务判断或签字验收，只防止证据状态、事实所有权和定制归层被口头记忆或演示结果混淆。

固定内容：

- `scenario-evidence.schema.json`：结构、枚举和必填字段；
- `examples/reference-oa.json`：当前参考 OA 的基准账目，证据类型明确为 `reference_product`；
- `examples/reference-oms.json`：第二参考产品 OMS 的 `evidence_ready` 账目；生产边界未知项保持 `null`，不冒充最终通过；
- `docs/templates/customer-scenario-evidence-v1.json`：第二个真实客户场景的可填写草稿，不包含客户数据，也不声明通过；
- `docs/templates/customer-scenario-intake-v1.md`：面向访谈者的脱敏采集工作表；
- `docs/operations/second-customer-scenario-intake.md`：从授权、访谈到最终结论的操作手册；
- `acceptance/scenario-evidence`：严格 JSON 解析、跨字段失败关闭规则、就绪度报告和保守的跨场景比较报告。

场景差异只允许落入 `stable_core`、`configuration`、`policy`、`typed_extension`、`customer_assembly` 或 `customer_specific` 中的一层。连接器不是第七个业务层，只能由客户装配或客户专项差异引用，并且必须说明来源／目标契约、幂等、失败恢复和目标确认规则。

单份场景账目只能声明 `reference_baseline` 或 `single_customer_scenario`。跨客户产品结论必须比较至少两个独立真实场景，不能在单份文件中声明。

多个 `reference_product` 可以帮助识别共性与未知项，但不能作为 compare CLI 的真实客户候选。参考产品之间的人工对照也不得升级为跨客户结论；当前 OA／OMS 对照见 [`docs/reports/reference-product-comparison-oa-oms.md`](../../../docs/reports/reference-product-comparison-oa-oms.md)。

验证入口：

```sh
make scenario-evidence-check

go run ./acceptance/scenario-evidence/cmd/validate \
  /absolute/path/to/customer-scenario.json

make scenario-evidence-readiness \
  SCENARIO=/absolute/path/to/customer-scenario.json \
  SCENARIO_TARGET=evidence_ready

make scenario-evidence-compare \
  BASELINE=contracts/scenario-evidence/v1/examples/reference-oa.json \
  CANDIDATE=/absolute/path/to/final-customer-scenario.json
```

`validate` 回答“文件是否遵守 v1 契约”，readiness 回答“还缺哪些访谈或最终验收义务”。合法 `draft` 仍可以通过前者，但会由后者逐项报告缺口。compare 只接受最终就绪的参考基准和 ID 不同的最终真实客户场景，固定列出应用差异、六层计数、处置计数、七项边界信号和人工评审义务；它的 `automated_conclusion.cross_customer_validated` 永远为 `false`，不能代替业务判断。

完整产品规则见 [`docs/specs/assembly/product-boundary-validation.md`](../../../docs/specs/assembly/product-boundary-validation.md) 和 [`arckit/spec/assembly/customer-scenario-evidence.md`](../../../arckit/spec/assembly/customer-scenario-evidence.md)。
