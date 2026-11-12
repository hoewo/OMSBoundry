# 发票识别与考勤折算核心验证（2026-11-05）

## 范围

本轮只完成两个新能力的版本化领域核心和本地测试，不包含 HTTP、IAM、数据库、页面或部署。

## 发票 OCR／AI 识别

已验证：

- 只读取精确 Evidence File ID／Version，重新核对字节数和 SHA-256；
- 支持 PDF、OFD、XML、JPEG、PNG，单文件上限 20 MiB；
- Provider 可替换并记录 provider／model／revision；
- 金额使用整数最小货币单位，强制不含税＋税＝价税合计；
- AI 结果只进入 `needs_review`；确认要求完整发票代码／号码和 expected revision；
- 同时交付内容摘要与发票代码＋号码两个独立重复线索。

## 考勤、请假和月中入离职折算

已验证：

- 一个完整日为 10000 整数单位；
- 自然日和显式排班工作日两种规则；
- 月中入职／离职截断、部分日请假、带薪比例和缺勤扣减；
- 重复请假／缺勤 ID 拒绝，单日累计扣减不超过一个完整日；
- 月度单位等式由共享契约校验；
- Payroll 使用大整数中间值和半入法折算整数最小货币单位，不使用浮点数。

## 实际验证命令

```sh
(cd packages/invoice-recognition-contract && GOWORK=off go test ./... && GOWORK=off go vet ./...)
(cd backend/services/invoice-recognition && GOWORK=off go test ./... && GOWORK=off go vet ./...)
(cd packages/attendance-summary-contract && GOWORK=off go test ./... && GOWORK=off go vet ./...)
(cd backend/services/workforce-attendance && GOWORK=off go test ./... && GOWORK=off go vet ./...)
(cd backend/services/payroll && GOWORK=off go test ./... && GOWORK=off go vet ./...)
```

## 未完成

Invoice Recognition 仍缺 PostgreSQL、HTTP／OpenAPI、IAM、正式 Provider、Workbench 校对页、D1 与恢复；Workforce Attendance 仍缺 PostgreSQL、HTTP／OpenAPI、IAM、People 快照端口、规则与考勤页面、工资行确认接入、D1 与恢复。上述能力均未部署。
