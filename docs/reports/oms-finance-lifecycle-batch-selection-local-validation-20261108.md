# 财务生命周期批量选择本地验证（BF-321，2026-11-08）

## 问题与结果

财务列表原先只把 `confirm_payment` 作为可勾选条件，导致处于草稿状态的报销和付款申请全部显示为禁用，也没有从列表进入既有 `submit` 生命周期的入口。当前本地候选改为状态自适应：有凭证的报销草稿与付款草稿可选择后提交，待支付／部分支付记录可选择后确认实际支付。

## 不变量

- Workbench 只消费 Internal Finance Requests 返回的 `allowed_actions`，不自行授予提交或付款能力。
- 缺少有效 Evidence 的报销草稿仍不可提交，并在复选框说明中提示补充凭证。
- 付款申请缺少凭证时必须随提交显式发送 `missing_evidence_acknowledged`。
- 提交与实际付款不能混入同一批次；实际付款、核销与结果未知恢复继续由 Finance／Money 既有端口负责。
- 每笔提交使用独立且在当前弹窗重试期间稳定的 command ID 与 expected revision；部分成功后的重试依赖 Owner 幂等语义，不重复登记。

## 实现范围

- 批量主按钮随选择状态显示“提交申请”或“确认已支付”。
- 行内与详情均补齐草稿“提交”入口。
- 选中一种生命周期动作后，另一种状态的复选框暂时禁用，清空选择后恢复。
- 报销缺凭证、付款未知结果或无对应 `allowed_actions` 时保持不可选，并给出原因。
- 前端静态资源版本更新为 `oms-phase1-finance-owner.js?v=20261108.7`。

## 验证证据

- `node --check internal/server/oms_phase1_finance_owner.js`：通过。
- `node --test internal/server/oms_phase1_finance_owner.test.mjs`：6/6 通过。
- `node --test internal/server/*.test.mjs`：300/300 通过。
- `GOWORK=off go test ./internal/server/...`：通过。
- `GOWORK=off go vet ./internal/server/...`：通过。
- `git diff --check`：通过。

## 未覆盖

本轮没有构建发布包、部署 D1、修改 Owner／API／schema／Capability，也没有在线提交或支付任何真实财务申请。仍需用本地正式 Owner 数据完成“勾选草稿—提交—状态刷新—再次勾选—确认支付”的浏览器写旅程，才可进入发布。
