# OMS 发票票面项目名称识别本地验证

日期：2026-11-17（GMT+8）  
事项：BF-343  
状态：本地已实现；待发布

## 问题与结果

问题 A：发票明细表已经显示“项目名称”，但 Invoice Recognition 候选、原图标注和人工校对都没有该字段，用户只能看到原票，无法核对或交付结构化名称。  
可验证结果 B：候选新增可选 `invoice_item_name`，图片／PDF 首页面从“项目名称”表头下的第一条明细提取并定位，Workbench 使用“项目名称（票面明细）”展示、人工修改、确认和导出。

## 本地证据

- Contract 接受可选 `invoice_item_name` 并拒绝超过 500 字符的确认值；Proposal／Confirmation 仍使用兼容的 v1 JSONB，不需要 migration。
- 专项 TSV 模拟“项目名称、规格型号、单位”表头，以及同行 `*生产生活服务*餐饮服务`、数量和 `330.69`；结果只保留名称，区域宽度没有覆盖数量／金额列，并生成字段置信度。
- XML Provider 同时兼容 `invoiceitemname`、`itemname`、`projectname`、`xmmc`、`spmc` 与 `goodsname` 的首个结构化值。
- Workbench 显示中文标签、字段置信度和原图标注，确认请求、搜索、CSV 与既有 JSON 交付都保留该字段；字段含义明确为票面明细，不是 OMS 经营项目。
- `packages/invoice-recognition-contract` 与 `backend/services/invoice-recognition` 全量 Go test、Workbench 专项 Node、Server Go test、JavaScript syntax 和 `git diff --check` 已通过。

## 边界

- 当前只提取首条票面项目名称；多明细结构化数组不在本批。
- 不按名称查询、创建或关联 Project，不自动创建报销、付款或 Billing 票据。
- 历史 Proposal／Confirmation 保持原值；部署后必须重新上传原 PDF 才会使用新 Provider revision。
