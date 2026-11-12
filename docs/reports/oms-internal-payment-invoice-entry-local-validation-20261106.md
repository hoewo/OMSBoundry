# 内部付款申请发票录入本地验证
日期：2026-11-06
决策：BF-313
状态：本地候选，未部署

## 问题与结果

付款申请新增时无法直接录入或识别发票，识别列表又把公司内部候选暴露给外部 Billing 关联 → 新增付款申请现在可选择暂不录入、直接录入或识别录入；识别人工确认后恢复原草稿、自动回填票面信息并引用同一 Evidence 版本。识别列表不再显示“关联使用”。

## 实现边界

- 只有尚未带发票的新付款申请显示三种录入方式；编辑既有申请不开放补票或换票。
- 直接录入保存发票代码、号码、日期、销售方、价税合计和可选原件引用。
- 识别录入复用 Invoice Recognition 原图校对；确认后回填付款金额、日期、一次性收款方和发票摘要，并引用精确 File ID／Version。
- Internal Finance Requests 要求付款申请金额等于发票价税合计，并把付款发票纳入内容摘要、代码／号码及销售方＋日期＋金额重复检查。
- Invoice Recognition 仍只拥有候选，Evidence 仍只拥有文件，Finance 仍只拥有申请和消费快照，Money 不在本次旅程中写入。
- BF-312 的外部 Billing 编排代码保留但页面不可达；本次没有删除或改写已存在的 Billing 事实。

## 已执行验证

```text
node --check oms_phase1_finance_owner.js                         PASS
node --check oms_phase1_workforce.js                             PASS
node --test frontend/oms-workbench/internal/server/*.test.mjs    PASS（296 项）
GOWORK=off go test ./...                                         PASS（Unified Workbench）
GOWORK=off go vet ./...                                          PASS（Unified Workbench）
GOWORK=off go test ./...                                         PASS（Internal Finance Requests）
GOWORK=off go vet ./...                                          PASS（Internal Finance Requests）
PostgreSQL 18.6 integration TestInvoiceStrongAndSuspectedDuplicateRules PASS
git diff --check                                                  PASS
```

PostgreSQL 集成测试以一笔带发票的付款申请建立首个指纹，再验证后续报销的强重复和疑似重复规则，证明付款发票不再被指纹逻辑跳过。

## 尚未验证

- 本机没有可用 Chromium／Playwright，Work 设置也禁用了本地 Computer Use，因此未执行真实浏览器文件选择、识别确认、草稿恢复和详情回读。
- 未构建 D1 发布候选，未运行全组合检查、备份恢复或公网部署。
- 未写入线上付款申请、识别候选、Billing 或 Money 数据。
