# 新增报销发票录入与识别本地验证（2026-11-08）

## 结论

BF-316 本地候选已把发票录入从“新增付款申请专用”扩展为新增报销／付款申请共用。新增报销不再只显示一个不会识别的文件框，而是在申请表内提供“暂不录入、直接录入、上传并识别”；上传会进入既有原图校对窗口，人工确认后恢复报销草稿、回填票面字段并保留精确 Evidence File ID／Version，用户明确保存后才创建 Finance 申请。

本候选没有新增 Owner、API、schema、Capability 或数据库。Internal Finance Requests 已有的行级 `InvoiceSnapshot`、Evidence 校验、金额一致性和重复指纹同时适用于报销与付款；Workbench 只把既有组合旅程扩展到新报销。识别列表仍不显示 Billing 外部关联，也没有既有申请补票／换票入口。

## 实现与不变量

- 新建报销和付款申请共用 `invoiceEntryMarkup`，显示暂不录入、直接录入和上传并识别。
- 表单草稿提取、取消恢复、确认回填和识别错误恢复按当前申请类型返回，不再写死 `payment`。
- `OMSInvoiceRecognitionBridge` 提供通用 `recognizeForFinance`，并保留 `recognizeForPayment` 兼容别名。
- 直接录入可上传原票；报销保存并提交仍要求至少一份已完成 Evidence，保存草稿可暂不录入。
- 识别确认只填充当前未保存申请；不会自动创建申请、Billing 票据、Money 资金或会计事实。
- 既有申请只保留原有编辑能力，未开放识别补票或换票。

## 已执行验证

- `node --check`：Finance 与 Workforce 两个 Workbench 脚本通过。
- 专项 Node：11/11 通过，覆盖两类新申请入口、通用识别桥、取消／确认返回标记、Evidence 严格契约和付款兼容。
- Workbench 全量 Node：299/299 通过。
- Workbench：`GOWORK=off go test ./...` 与 `go vet ./...` 通过。
- Internal Finance Requests：`GOWORK=off go test ./internal/finance/...` 与 `go vet` 通过；新增报销发票快照规范化用例通过。
- 本地 Workbench 使用包含 `internal-finance-requests` 的 Suite Manifest 装配正式 Owner 界面：新增报销与新增付款申请均可见三种发票方式；报销切换“直接录入”会展开票面与原票字段，切换“上传并识别”会显示识别入口和确认后回填说明；付款申请显示同一组入口。浏览器 console 0 warning／error。
- `git diff --check` 通过。

## 尚未完成

- 尚未构建发布包、部署公网或创建线上报销数据。
- 本地没有可提交的正式成员身份和独立测试发票，因此本轮未在浏览器创建 Finance 事实；上传后的 OCR、人工确认、保存与回读仍由发布后专门测试数据完成最终验收。
- 既有报销／付款申请补票、换票和多张发票拆分仍不在本轮范围。
