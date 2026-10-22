# OMS 一期金额列对齐修复发布报告

日期：2026-10-22  
事项：BF-178  
源码基线：`main@37ec7e3`

## 结论

金额列歪斜来自表头与数值单元格使用了不同的右边界：通用排序按钮带负边距，而数据单元格使用正常内边距；不同金额列也没有共同的最小宽度。

本次为应收、应付、开票、收票、回款和支付列表增加专用 `is-money` 列语义。金额表头与金额单元格统一 132px 最小宽度、右对齐、无负边距的全宽排序按钮、nowrap 和 tabular lining figures；项目“应收状态／应付状态”继续使用状态布局。项目详情中的票据金额也使用稳定数字排版。

这次只修复 Workbench 展示，不改变金额的整数最小货币单位、固定两位小数、计算、排序、筛选、导出、Owner、API、schema、Capability、Grant 或业务数据。

## 发布制品与回滚点

- Unified Workbench：`v0.1.0-test.oms-v1.29@sha256:edc5a289fb4ff5dc03d1363aaa801a370dbfad59f42fc7dd77145369debd731b`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.29.https.env`
- 人工升级前完整备份：`/var/backups/omsbizfoundry/20261022T123518Z`
- D1 升级自动备份：`/var/backups/omsbizfoundry/20261022T123619Z`
- 公网入口：`https://oatest.antuinfo.cn/oms-v1/`

## 验证

本地实际通过：

- 金额列静态回归 3/3；与分页、主体候选合并的 Node 回归 10/10；
- `node --check`、Workbench Go test／vet；
- 前端 TypeScript check、206/206 测试和两个 Vite bundle build；
- 浏览器计算样式确认应收三个金额表头均为 132px、右对齐、`lining-nums tabular-nums`，排序按钮距表头右边界均为 14px。

公网发布后实际通过：

- D1 preflight、升级与全栈 `check`；
- 17/17 个长运行容器 healthy；
- 正式 IAM 登录、Gateway 授权与 Project／Party／Billing／Money／Contract Owner 只读验收；
- 公网 `oms-phase1.css?v=20261022.9` 与 `oms-phase1.js?v=20261022.9` 返回不可变缓存资源，CSS 包含金额列统一右边界规则；
- Project 1、Party 5、Billing 0、Money 0，业务写入为 0；甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[施工方1]` 保持不变。

## 边界

当前真实测试数据没有票据和资金流水，因此公网只读复验能够证明资源、身份链、Owner 读取和现有项目应收投影没有回归；开票、收票、回款和支付的有数据视觉排列由同一 `is-money` 单元格模板及静态回归覆盖。本次未为视觉验证创建、修改或删除任何业务记录。
