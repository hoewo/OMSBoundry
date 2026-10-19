# 客户场景证据校验技术方案

## 目标与边界

该方案把客户场景边界账目固定为目标无关 JSON，并以独立 Go 验证器执行结构和跨字段规则。验证器不连接业务数据库、不读取客户系统、不调用应用 API，也不自动替业务人员归层。

## 工件

| 工件 | 责任 |
| --- | --- |
| `contracts/scenario-evidence/v1/scenario-evidence.schema.json` | 发布 v1 字段、枚举、必填项和未知字段关闭边界 |
| `contracts/scenario-evidence/v1/examples/reference-oa.json` | 保存首个参考 OA 的真实基准账目，不冒充客户验收 |
| `contracts/scenario-evidence/v1/examples/reference-oms.json` | 保存第二参考产品 OMS 的 evidence-ready 账目，未知生产边界保持未测量 |
| `docs/reports/reference-product-comparison-oa-oms.md` | 人工比较两个参考产品的稳定共性、六层变化和保留门槛 |
| `docs/templates/customer-scenario-evidence-v1.json` | 提供第二真实客户的空白草稿入口 |
| `docs/templates/customer-scenario-intake-v1.md` | 提供不接触秘密的人工访谈工作表 |
| `docs/operations/second-customer-scenario-intake.md` | 固定授权、事实采集、归层、验收和停止条件 |
| `acceptance/scenario-evidence` | 严格解析、跨字段规则、就绪度评估、保守比较、正负例和 CLI |
| `make scenario-evidence-check` | 重复执行测试并校验发布样例与模板 |

契约文件与验证器版本必须共同演进。未知 `schema_version` 和未知字段失败关闭；兼容新增字段需要先发布新契约和负例，再允许生产证据使用。

## 校验层

### 结构层

JSON Schema 固定对象形状、六层枚举、证据身份、状态、标识格式和必填字段。所有对象使用 `additionalProperties: false`，避免任意客户字段绕过归层规则。

### 语义层

Go 验证器执行 JSON Schema 无法独立表达或项目需要更强约束的规则：

- 来源身份、claim 和最终状态相互匹配；
- 真实客户最终结果存在客户确认引用；
- 每个事实包含 owner、状态和 correction，差异只能引用已声明事实；
- 启用和禁用应用不重叠；
- 已实现差异具有交付物和证据；
- 已实现策略具有版本、输入、输出、纠错和运行证据；
- 连接器隶属于已实现的客户装配或客户专项差异，并具有完整传递语义；
- `passed` 的七项边界信号全部已知且为零；
- 非草稿具有问题结果、来源、事实、启用应用和差异，最终结果另有验收证据。

### 就绪度层

`AssessReadiness` 不修改证据文件，也不放宽 `Validate`。它把合法文档按操作义务分为：

- `draft`：仍缺可观察结果、来源证据、事实证据、最小应用选择或差异归层／实现证据；
- `evidence_ready`：足以进入人工业务判断和实现，但仍缺最终验收、真实客户确认或边界信号；
- `final_ready`：最终义务均已存在，可以再由严格 validator 判断 `passed` 或 `failed` 声明是否合法。

七项边界信号只以 `null` 表示尚未测量；非零值表示已经测得反证。readiness 对非零值输出 `BOUNDARY_FINDING`，但不把它计入 `FinalGaps`；严格 validator 继续保证 `passed` 只能使用全零信号，而 `failed` 可以保存非零事实。

模板占位只匹配明确的 `replace-`、`replace ` 或“替换为”前缀，不能把真实业务中的“成果替换”等词误判为占位内容。

## 运行方式

仓库门禁执行：

```sh
make scenario-evidence-check
```

单文件检查执行：

```sh
go run ./acceptance/scenario-evidence/cmd/validate \
  /absolute/path/to/customer-scenario.json

go run ./acceptance/scenario-evidence/cmd/readiness \
  --require evidence_ready \
  /absolute/path/to/customer-scenario.json
```

严格校验成功输出 `SCENARIO_EVIDENCE_STATUS=passed` 和文件路径。就绪度命令输出 `SCENARIO_READINESS`，以及所有 `EVIDENCE_READY_GAP`、`FINAL_READY_GAP` 和已测得的 `BOUNDARY_FINDING`；提供 `--require` 时，未达到最低阶段返回非零。任一命令都不生成客户结论或自动修改 JSON。

最终真实客户场景完成后，生成确定性比较报告：

```sh
go run ./acceptance/scenario-evidence/cmd/compare \
  contracts/scenario-evidence/v1/examples/reference-oa.json \
  /absolute/path/to/final-customer-scenario.json
```

比较器先执行严格校验，再要求输入分别为 `passed` 的 `reference_product/reference_baseline` 和 ID 不同、状态为 `passed` 或 `failed` 的 `real_customer_anonymized/single_customer_scenario`，且两者均为 `final_ready`。输出顺序固定：启用应用交并差、六层差异数量、四类处置数量和七项边界信号；失败场景的非零边界值和 finding 原样保留。报告只把材料整理为一致格式，固定输出 `cross_customer_validated: false`、`result: not_determined` 和六项人工评审义务。

第二参考产品不进入 compare CLI。它先通过同一 v1 validator 和 `evidence_ready` 门槛，再由人工报告逐项比较事实含义、所有权、纠错、候选应用和未知项；这样既复用证据结构，也避免把“两个产品样本”误报成“两个真实客户”。

## 数据与隐私

账目只保存脱敏业务含义和仓库内或受控外部证据引用，不保存凭证、原始客户文件或生产个人数据。真实客户证据引用必须指向已经获准保存的位置；若材料不能进入仓库，引用只记录受控证据标识和确认结果。

## 版本与迁移

v1 只接受 `customer-scenario-evidence/v1`。字段含义、允许 claim 或失败规则变化时发布新版本，不静默放宽 v1。旧账目保持原版本可复验；升级通过显式转换产生新文件并保留原证据。

## 已知限制

- JSON Schema 当前由发布检查确认可解析和拥有稳定 `$id`；跨字段一致性由 Go 验证器执行；
- 验证通过只说明账目满足证据契约，不证明账目中的业务判断正确；
- compare CLI 可以确定性整理一份参考基准和一份真实客户账目的差异，但不比较事实语义本身、不编码交付时长或核心改动范围，也不生成跨客户结论；
- 当前参考 OA 样例证明格式能承载最终参考基准；OMS 样例证明格式能诚实保存 evidence-ready 的第二参考产品，第二个真实客户仍需要外部输入。
