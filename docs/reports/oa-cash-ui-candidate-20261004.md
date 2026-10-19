# OA 支付原组件候选接线

2026-10-04。未发布，不是完整 OA／公网支付验收。招投标是举例，范围仍为[完整 OA 清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)。本切片接原双列表、费用行、640px 抽屉及表单到 Money 实际资金、Project 显式关系及原生冲销；开票关联／核销／Receipt Set／奖金未完成。

最新重构建收尾见[整轮检查点](./oa-complete-ui-checkpoint-20261004.md)：bundle `f716e1cc6de47984272582de5055b1900c87941afe79f2259f611e22a1e0cfb6` 的 Contract ae／Cash j／Invoice f 共 51 场景、0 页面错误及 Workbench native c 两次独立资源启动通过；较早回执／摘要保留原版本，不混称新构建。所有组件检查仍是本地明确测试端口，未发布、不代表公网或完整 OA。

## 事实、原组件和边界

BF-099 与[端口规格](../specs/money-flow-ledger/oa-cash-command-port-v1.md)：Money 既有 Cash Transaction 自有事实／双方发生时快照／审计／冲销，不新增通用 OA FinancialRecord。Project relation key 只承载六种两链／三费用归类，不决定资金双方或金额。两链分别登记，显式选择 Money 当前本目录主体、账户、原始依据；不按甲方名称匹配身份、不自动复制合同或另一链金额。确认事实不能原地编辑／删除，原删除按钮改为有原因的显式冲销；已核销事实由原用例拒绝冲销。

原 OA 提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 PaymentRecordList、ClientPaymentForm、BusinessTransactionsPanel.css、BusinessTransactionForms.css 已复制，原目录只读。保留双列表／卡片、三费用徽标、日期／金额行、640px 抽屉、表单列布局和原样式；端口、精确金额、明确主体／账户／日期及不可变历史按目标约束适配。原文件 SHA 与必要差异见 `frontend/oms-workbench/frontend/source-provenance.json`。

严格 JSON 原生端口使用可信 IAM Actor；新建租户范围，查看／冲销接受精确 cash_transaction 人类原生范围。金额读取缺失时金额及币种为 null，不能盲写／冲销。单 reference.read／transaction 工作负载能力仍独立，不能借只读连接器签发人类命令。创建 Money→保留 ID→Project Link→原生 GET 各自提交；回复未知复用稳定命令，Link 故障不重建资金，已提交后回读故障只补 GET，403 不换身份，409 不自动覆盖。关闭／刷新不写入。

## 实际检查与限制

- Money Go 单元／vet、Workbench Go／vet、TypeScript check／build 已通过。
- 全量 Workbench／frontend Node 最新 262 项，0 skip；资金适配器 12 项涵盖六分类、金额精度、日期／主体／账户拒绝、原命令跨 reload 续办、Link 部分成功、只读／脱敏、原事实冲销留史和未声明旧关系拒绝；新增 owner selected-only 路由及架构写回执分类检查。
- 原组件浏览器 a：下拉关闭动画期间旧／新菜单定位重叠，零命令失败，改为等待正常关闭；b：禁用发票 Select 无显式可访问标签，零命令失败，已补 aria-label。失败目录保留，不覆盖为通过。
- 私有 PG18 b：`TestCashWorkspacePostgresExactMoneyScopeHistoryAndReplay` 实际通过、无 skip；schema 23，自有空测试库、真实 IAM verifier／测试 issuer，精确大额 9007199254740993.01、可信 Actor、原生双方快照／稳定命令、冲销冻结原金额、范围拒绝及脱敏零写。另顺序运行最新 Contract 结算确认主体投影专项，通过、无 skip。Main 前后保持，私有 PG 已停止。
- Money 独立启动 a：空自有库 ops migrate→server verify、schema 23、四个匿名原生端点 401、重启事实保持通过；配置正式 IAM verifier 和公钥，没有关闭鉴权，无公网端口。不是实际 issuer／Project／Money 联合写入旅程。
- 原组件 i：与合同 ad、开票 e 同 bundle，14 场景／20 命令／8 条原生资金／0 页面错误。两链三费用、大额／小额、刷新零写、不可变详情、创建／Link／冲销回复未知后 reload／原命令继续、只读和金额脱敏已实际点击；发票关联仍禁用且明确命名。不代表 PostgreSQL／真实 issuer／公网组合。
- 实际 IAM issuer＋Project＋Money 联合、旧发布兼容与匹配能力目录、正式公网及全角色证据尚未完成，不能把分层结果拼成整站业务通过。

## 未变化和下一步

Main Workbench `.20261003.6`、Money 数据／Grant 未改；用户测试项目101未写入，restore20 没有重启。支付当前是候选源码，不是用户公网已经看到的效果。原生编辑／删除的可见替代仍需用户操作等效验证；开票关联不能以禁用控件记完成。已补原组件／PG18／启动门禁；继续正式联合／兼容发布及全角色，按完整清单推进开票关联、成本／奖金、生产、资料统计，不以这批替代整个目标。

安全回执：[真库 b](../../acceptance/oa-initiation-repair/evidence/cash-20261004/postgres-b.json)、[独立启动 a](../../acceptance/oa-initiation-repair/evidence/cash-20261004/startup-a.json)、[原组件 i](../../acceptance/oa-initiation-repair/evidence/cash-20261004/components-i.json)、[双列表截图](../../acceptance/oa-initiation-repair/evidence/cash-20261004/two-chain-payments-i.png)。统一前端 SHA `e9f67ef24d4d396d6c1df1d48aeecb63178d7cd4ba733c0591096962b6609861`；Money test SHA `6fd91f3fdc74d395b49238e8cbbb08c3706fa9d2da45e75cf6b8315cecd2d11a`，server／ops 摘要分别见启动回执。截图已视觉检查，只证明候选显示。

后续失败保留：真库 a 测试 SQL 误用了不存在的 Actor 列名，修正测试后 b 实际通过；组件 c／d 是组合框／关闭按钮定位歧义，e／f 在失败回复完成前抓取命令，g 的 Toast／Alert 同文案定位歧义。改为真实失败 Alert 和续办按钮完成后检查，不强制点击或改变 DOM；h／i 通过。各后缀失败目录未覆盖，不删除其已保存测试事实。

运行入口：`verify-cash-candidate-ecs.py`、`verify-cash-startup-ecs.py`；本地 `node acceptance/oa-initiation-repair/verify-cash-components-browser.mjs /tmp/oa-cash-components.<新后缀>`。云端驱动固定私有网络／数据库，无 Main 写入与公网端口。
