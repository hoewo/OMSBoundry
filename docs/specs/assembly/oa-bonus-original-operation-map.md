# 原 OA 奖金工作面与类型化适配

问题 A → 结果 B：原项目奖金栏目只有比例奖池接口，无法执行原 OA 手工分配 → 在同一项目使用原表单完成经营／生产分配、复读和留史撤回，不把登记当付款。

## 原站事实

只读参照 `road-design-oa-system` commit `826e0fc65aa09947fc4f64f62a2644fa5d11a302`：`shared/BonusAllocation{,Form,List,Statistics}.tsx`、`oa2026/OA2026Controls.tsx` 和 `backend/internal/services/financial_service.go:501`。

- B11 奖池：来源金额、比例、除数 1.065、计算额和可空日期。原 GET 的隐式写入及浮点计算不复制。
- B12 分配：用户单独填写比例池基数和日期；按比例与固定额混合，固定额不占比例池。比例合计不超过 100%；同一人员不同工作分工可以各有一行。
- 原表单和列表保留，替换有界数据适配。原“发放”文案更正为“分配登记”，不宣称资金已付。
- 原后台按人员＋角色文字防重且信任客户端金额；目标按稳定 Assignment 防重、复验 Person／Assignment，并由服务端精确计算。

## 实施选择与不变量

Bonus 自有 `reference-oa-bonus-v2` 显式选装扩展，B12 使用独立手工分配对象，不伪造来源确认奖池。旧 Pool、比例分配、来源确认、应付建议端口及表保持原约束。手填基数只是本次人工声明，不是 Contract 结算或 Money 收款事实；B11 和 B12 不隐式同步。

一次命令原子登记整批，冻结 Project、Person、Assignment、基数、日期、工作角色、方法、输入值和结果。整数分／基点、half-up，溢出拒绝；固定额可超过手填比例池基数。以当前有效且侧别适用的 Assignment 为候选，客户端不能任填角色。撤回需 expected revision 和原因，追加 revision／审计，不能物理删除。未知回复复用原命令，刷新 GET 零写入。

能力使用独立 `bonus-manual-allocation` namespace；经营／生产分别为 `business_bonus_project`／`production_bonus_project`，精确记录为 `reference_oa_bonus_allocation`。旧 project、旧奖池能力不自动扩权；金额、历史分别脱敏和授权。Workbench 不保存奖金业务事实，不导入 Bonus 领域代码，不调用 Money 代付。

## 验收门槛与当前状态

2026-10-04 最新 BF-105：独立 Project schema 1 专业分工引用、Bonus schema4／显式v3和正常工作台已候选接通，真实私有 Nginx→Gateway→正式 IAM连续20场景、Project／Bonus PG18及旧v2兼容／Bonus race通过，[实际报告](../../reports/oa-professional-bonus-continuous-validation-20261004.md)。下方“下一切片阻塞”保留其早期状态，不是最新接口事实；Main／公网／B11／全角色／Suite恢复未通过。

### 显式 v3 生产专业消费

输入只对新来源增加 `assignment_source=reference-oa-production-personnel-v1`，旧 v2输入不得多出此字段，原 JSON和hash不变。Bonus v3同时接受旧真实Assignment与新专业分工，但按source＋分工ID区分，同Person或namespace内同ID不混成一条。四专业角色必须冻结完整专业，项目级审定／概算没有专业；不是扩旧Pool资格枚举。

新来源通过 Project 的原生 personnel.read／精确 Project读取独立 resource type，逐项校验当前Project与Person引用再整批提交。Bonus保存自身的最小Person／分工／专业快照，消费端不写源或同步一份名册。active参与人即使“不适用”仍按原奖金表单保留资格；ended来源只能读取历史，不能新登记。未知提交保留确切原命令；后续撤权也不能把尚未知结果当未发生而丢弃。

Workbench两侧使用同一显式Bonusprofile，生产人员另需D01profile／应用选装；人员保存后奖金加载先复读Project revision。两者默认不启用新消费或新增授权。原目录／人员能力隔离、隐藏零读、快切scope、金额撤权、原撤回历史及空目标Bonus恢复均有限实测；不是整套联合恢复。

2026-10-04 D01 新六分工已在 Project 有界 Overlay 的原 UI／正式 IAM／People／PG18 本地候选跑通，21 条包括目录能力隔离和原审定清除；不是 Bonus 已兼容。当前 Bonus 发现／复验仍只走旧 Base Assignment，输入也无来源 namespace；因此不能直接把设计／复核等角色塞入旧 participant。下一切片的代码阻塞及最小联动旅程见[D01 对照](./oa-production-personnel-clone-gap.md#下一消费切片的具体阻塞代码事实不是已实现接口)，新版本化引用／消费／原表单及恢复未验前，B12 专业分工门槛保持未通过。

2026-10-04 续轮：原组件加实际 Workbench→Gateway→真实 IAM→Bonus／PG18 已跑 14 有限场景，正常登记／撤回／刷新及金额撤权可见，Project／People 受保护引用通过。BF-103 用显式同 audience namespace 补旧／新 read ANY 准入，默认 profile 不变、不扩权；尚未匹配发布到 Main、未跑公网／D1 联合恢复。B11／专业分工及全角色仍未关闭，[报告](../../reports/oa-bonus-manual-ui-candidate-20261004.md)。

2026-10-04 08:32 UTC：原 B12 私有组件→独立 Bonus HTTP／PG18 及空目标恢复／续办通过；schema 2→3 保持旧 Pool／建议，签名 IAM 测试提供者拒绝及脱敏通过，详见[候选报告](../../reports/oa-bonus-manual-ui-candidate-20261004.md)。非 Main／完整奖金。当前生产候选仅真实存在的 production_owner／participant，不能冒充原专业分工；业务侧为 business_owner／commercial_contributor。

本文件是施工规格，不是通过报告。至少验证原表单正常点击、比例／固定额混合、同人不同分工、日期／备注、批内防重、比例超额、上游故障整批零写、并发命令重放、错租户／项目／侧别、缺金额脱敏、撤回历史和恢复后继续写。候选、隔离真库、独立启动、匹配发布闭包和同项目公网证据分别登记。

B11 来源选择、开票关联、实际付款、税务／薪酬、全角色、完整 OA 与 OSS 分离不由 B12 登记冒充完成。B11 不回退到未确认合同金额，生产奖池不把单笔 Cash 冒充已确认 Receipt Set。
