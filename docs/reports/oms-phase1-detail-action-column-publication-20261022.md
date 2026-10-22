# OMS 一期详情操作列对齐修复发布报告

日期：2026-10-22  
事项：BF-179  
源码基线：`main@cd84894`

## 结论

用户确认列表中的“详情”属于最右侧操作列，应统一居右。此前详情按钮所在单元格使用普通左对齐布局，金额列修正后，该偏差更加明显。

本次把项目、甲方、施工方、应收、应付、开票、收票、回款和支付九张业务表格的末列统一为 76px 右对齐操作列。表头空占位与详情按钮共用同一最右边界；规则只作用于真实数据行，空状态继续横跨完整表格。整行点击与“详情”按钮仍进入同一详情。

这次只修复 Workbench 排版，不改变详情内容、Owner、API、schema、Capability、Grant 或业务数据。

## 发布制品与回滚点

- Unified Workbench：`v0.1.0-test.oms-v1.30@sha256:b093a9d6986edd0fcd67efcfec93042b1f5aaf7a44f97dd8ae610bd838051be6`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.30.https.env`
- 人工升级前完整备份：`/var/backups/omsbizfoundry/20261022T131502Z`
- D1 升级自动备份：`/var/backups/omsbizfoundry/20261022T131614Z`
- 公网入口：`https://oatest.antuinfo.cn/oms-v1/`

## 验证

本地实际通过：

- 操作列回归 2/2；与金额、分页和主体候选合并的 Node 回归 9/9；
- Workbench Go test／vet；
- 前端 TypeScript check、206/206 测试和两个 Vite bundle build；
- 公网验收单测 2/2 和 Python 语法检查；
- 浏览器计算样式确认应收管理表头末列宽 76px、右对齐，末列右边界与表格右边界一致。

公网发布后实际通过：

- D1 preflight、升级与全栈 `check`；
- 17/17 个长运行容器 healthy；
- 正式 IAM 登录、Gateway 授权与 Project／Party／Billing／Money／Contract Owner 只读验收；
- 公网 `oms-phase1.css?v=20261022.10` 返回不可变缓存资源并包含 76px 详情末列右对齐规则；
- Project 1、Party 5、Billing 0、Money 0，业务写入为 0；甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[施工方1]` 保持不变。

## 边界

当前真实测试数据没有票据和资金流水；它们与项目、主体和应收／应付列表共用同一末列 CSS 及五类动态详情单元格模板，由静态回归覆盖。本次未为视觉验证创建、修改或删除任何业务记录。
