# OMS1 P5 财务申请与付款确认本地验证报告

日期：2026-10-25
状态：本地门禁通过；未部署 D1／公网
决策：BF-240

## 1. A → B

报销和付款申请若继续由浏览器保存，财务就无法冻结申请、收款账户、附件引用和审批策略，也无法在真实付款部分失败或结果未知时安全恢复 → `Internal Finance Requests` 独立拥有申请、revision、敏感读取审计和 Payment Delivery；Money 独立拥有义务、实际资金、核销与超额未分配事实，两者只通过版本化 HTTP 和稳定引用协作。

## 2. 已交付

- Finance migration `000002_finance_execution.sql`：正式申请详情、审批快照、付款投递／事件、费用类别 revision 和敏感访问审计；当前投影与不可变历史继续由数据库约束保护。
- 申请生命周期：报销／付款草稿、多条明细、Owner 计算总额、提交、无付款撤回与重新提交；首笔成功付款后禁止修改或撤回。
- 收款与归属：分别冻结申请人、收款人或 Party／一次性收款方、银行账号、可选部门／项目／合同稳定引用；Workbench 展示申请人、收款方和实际付款操作人。
- 附件边界：申请只保存 Evidence File ID／Version 与业务含义。报销提交要求至少一个引用；付款申请缺附件必须显式确认。P7 才验证真实上传、文件完成状态和重复检测。
- 审批占位：报销与付款均冻结 `not_required / not_applicable`，不伪造审批人或自动批准事实；启用正式审批仍属于后续版本。
- 付款：逐笔或批量确认实际付款，每项使用独立 command 与 expected revision；部分成功不回滚成功项。分次与超额付款分别保存实付、核销和超额，Money 只按申请余额核销。
- Money migration `000026_internal_finance_payments.sql`：每个申请稳定绑定一个义务；每次确认创建独立 Payment／Allocation，并保存不可变交付结果。一次性收款方只在 Money 内建立履行资金事实所需的本地 Party，不回写 Party Directory。
- 敏感边界：所有普通写响应、详情、列表和普通导出均由服务端移除完整账号；完整读取与敏感导出使用不同 Capability，要求用途并逐条审计。敏感导出内容不进入 Workbench 持久状态。
- Workbench：正式 Finance Owner 装配时停用浏览器本地财务事实，提供筛选、排序、分页、草稿／提交／撤回、逐笔／批量付款、费用类别、普通／敏感导出和详情界面。

## 3. 关键不变量

- Finance 不写 Money 表；Money 不拥有申请、审批或费用明细，也不反向修改 Finance。两边以幂等 Payment Delivery 和稳定 Payment／Allocation 引用连接。
- 一张申请可以分多次付款，但只建立一个义务。申请总额使用整数分，实付、核销、超额分别累计，剩余不得成为负数。
- 确定的 4xx Money 拒绝可以标记失败后重试；超时、5xx、限流、非法成功响应等结果未知不得用新命令猜测重付。
- 批量仅编排独立命令；一项冲突或失败不得回滚其他已经成功的 Money 事实。
- 完整账号只能经显式敏感能力和用途读取；普通写响应也不能因为“刚提交”而绕过遮罩。
- 费用归属和 Evidence 引用是快照，不产生预算、会计、付款资格或审批路由，不暗示实时来源已经核验。

## 4. 实际验证

通过的门禁：

1. Finance、Money 和 Workbench 三个 Go module 的 `go test -count=1 ./...`；Finance HTTP 连接器测试覆盖确定拒绝与未知结果分类；
2. Workbench `node --test internal/server/*.test.mjs`：258／258 通过，包含正式 Finance Owner 切换、独立导出、归属引用和逐笔批量字段；
3. PostgreSQL 18.6 integration：Finance lifecycle、幂等、revision、不可变历史、跨租户、类别、敏感审计、分次／超额和确定拒绝通过；Money 26 个 migration、schema gate 和 Finance Payment 集成通过；
4. `./acceptance/p5-finance-payment/run-isolated`：真实 Finance、Money 两进程和两个独立数据库，付款申请、无需审批快照、部分付款、超额门禁、批量部分成功、撤回重提、普通／敏感导出及审计全部匹配；
5. 主申请两笔实际付款只建立一个 Money 义务，核销 10000 分并留下 1000 分超额未分配；批量中的 revision 冲突不回滚另一笔成功付款；
6. Finance 与 Money 数据均由 PostgreSQL 18 原生备份恢复到空目标，恢复后的申请、付款、核销、敏感审计和交付计数一致；验收退出后进程与容器自动清理。

## 5. 验收中修复的问题

- 写命令的即时响应曾返回完整账号，而列表和详情才遮罩。现由 Service 对新建、修改、提交、撤回和付款返回统一脱敏，并以集成验收验证普通导出无完整账号。
- 付款完成的即时响应曾同时包含同一命令的 pending 与 succeeded 两个表现行。现提交 Money 成功后从当前交付投影重新读取，只向页面返回每个命令的一条当前付款记录；内部事件历史仍完整保留。

## 6. 未外推范围

- 没有启动或修改正式 D1，也没有部署 `oatest.antuinfo.cn`；P9 才负责全组合、正式 IAM／Gateway、恢复和公网连续旅程。
- P5 保存 Evidence 稳定引用，但未调用 Evidence 实时验证文件是否完成、是否安全或是否重复；这些属于 P7 发票报销闭环。
- `not_required` 只是当前审批策略快照；没有审批流、审批人或自动批准事实。
- 当前本地模板身份便于隔离验收，不代表生产 Grant；能力 manifest、IAM 和 Gateway 的真实授权组合仍须在 P9 复验。
