# OMS1 P4 工资表、核对与实际发放本地验证报告

日期：2026-10-25
状态：本地门禁通过；未部署 D1／公网
决策：BF-239

## 1. A → B

工资表若继续由浏览器计算和保存，发放确认就无法冻结成员、薪酬、缴费方案、收款账户和实际资金引用，也无法在批量操作部分失败时安全续办 → Payroll 独立拥有工资期间、工资行、计算快照、核对和发放历史，经版本化 HTTP 把每名成员的实际付款交付给 Money；Money 独立保存义务、实际资金、核销和超额未分配事实。

## 2. 已交付

- Payroll migration `000003_payroll_execution.sql`：常规工资期间月度唯一，工资行和 revision 冻结 Person、Compensation、Payment Profile、Contribution Scheme 及计算明细；金额使用整数分，比例使用整数万分比。
- 计算与录入：固定工资、绩效、带原因正负调整、个人／单位五险一金逐项、外部个税快照、应发、实发与企业成本；导入先生成有时效的预览，再以 expected revision 和幂等命令提交。
- 生命周期：草稿、待核对、无需审批、待发放、部分发放、已发放、无需发放、待扣回、作废／替代和补发／扣回调整批次；计划发薪日不产生已发放事实。
- 同步：工资表发生任何成功或未知付款后禁止重算／同步；未付款工资表只在显式预览和提交后追加新 revision，不覆盖原快照。
- 实际发放：逐人和批量均按工资行独立命令执行。批量部分失败返回逐项结果，不回滚或重复成功项；缺少账户、零实发、负实发、超额和结果未知均有独立恢复路径。
- Money migration `000025_payroll_payments.sql`：每个 Payroll Line 稳定绑定一个工资义务；每次实际发放创建独立 Payment，按剩余实发额核销，超额保留为未分配资金；交付与绑定不可变且可幂等重放。
- People Reference：Payroll 只通过稳定 Tenant＋Person 引用读取成员快照；People 和 Payroll 仍分库、分 Owner，不用人员状态替代工资资格规则。
- Workbench：当 People 与 Payroll 正式装配时，成员和工资页面调用版本化 Owner API，展示生成、导入预览、核对、同步、逐人／批量发放、部分失败、超额确认和恢复；不再把正式工资事实写入浏览器。

## 3. 关键不变量

- Payroll 不直接写 Money 表；Money 不计算工资，也不改 Payroll 状态。两个 Owner 只通过版本化交付契约和稳定引用协作。
- 一名成员一张工资行可分多次支付，但只形成一个工资义务；实际支付、核销和超额分别保存，剩余应发不出现负值。
- 成功 Money 事实绝不因同批其他成员失败而回滚；失败／未知项只能使用原命令恢复。
- 普通成员只能读取本人已实际发放的工资单；草稿、未发、失败／未知和他人工资不因页面可见性而放宽 Owner 授权。
- 零实发只能显式确认无需发放；负实发进入待扣回，不伪造 Payment；超额必须显式确认并填写原因。
- 验收方案只用于计算测试，不代表任何地区当前法定比例。

## 4. 实际验证

通过的门禁：

1. Payroll、Money、People、Workbench 四个 Go module 的 `go test ./...` 与 `go vet ./...`；
2. Workbench `node --test internal/server/*.test.mjs`：256／256 通过，包含正式 Payroll Owner 切换、版本化 API、预览、部分失败和超额恢复；
3. Money PostgreSQL 18.6 `go test -tags=integration ./internal/database`：25 个 migration 的 bootstrap、历史升级、并发与 schema gate 通过；
4. Payroll PostgreSQL integration：整数计算与舍入、同月并发创建、幂等重放、未知结果恢复、零／负实发、超额、缺账户、部分发放、作废／替代、补发／扣回和本人已发工资可见性通过；
5. `./acceptance/p4-payroll-execution/run-isolated`：真实 People、Payroll、Money 三进程和三个独立数据库，14 个 HTTP 断言全部匹配；两次实际付款只产生一个工资义务，精确核销净应发并保留 500 分超额未分配余额；
6. `git diff --check`、格式和秘密模式扫描作为提交前门禁执行。

## 5. 未外推范围

- 没有启动或变更正式 D1，也没有部署 `oatest.antuinfo.cn`；P9 才负责全组合、备份恢复和公网连续旅程。
- Money 标准 Owner 导出当前仍导出正式义务／实际资金／核销事实；Payroll 专用交付绑定表尚未加入跨应用交付导出，P9 恢复门禁前必须补齐或明确独立恢复策略。
- 财务申请、附件、审批占位和付款确认属于 P5；员工 API Key、开放 API、发票报销闭环和 Feedback 分属 P6～P8。
- 仓库没有把测试缴费比例宣称为国家规定或生产默认值；正式方案仍需地区、权威来源和核对时间。
