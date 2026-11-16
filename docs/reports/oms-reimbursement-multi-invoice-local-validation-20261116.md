# OMS 多发票报销本地实现与验证（BF-338）

日期：2026-11-16（GMT+8）  
状态：本地实现完成；后续已随 Suite v1.104 发布，见[发布记录](./oms-reimbursement-multi-invoice-v104-publication-20261116.md)

## 问题与可验证结果

一张现实报销常包含多张发票，但既有表单只能表达一个“报销内容”和一条发票 → 新增报销可在单项与多项之间选择；多项使用一个整单名称和多条费用明细，每条明细独立关联一张发票，整单金额自动精确合计。

## 已实现

- Workbench 增加“单项发票／多项发票”选择；多项模式默认两条，最多 20 条。
- 整单保留“报销内容”；明细名称选填，空值保存为“内容1、内容2……”。
- 每条明细独立填写金额、日期，并选择暂不录入、直接录入或上传识别。
- 每条明细最多一个发票关联；直接录入和识别结果都保存原票 Evidence，识别来源另存 Proposal ID／revision。
- 正式提交逐条要求发票 Evidence，且票面价税合计必须等于对应明细金额；整单总额由明细整数分求和。
- Finance 的强重复／疑似重复按每张关联发票独立核对，结果包含 Invoice ID、Line ID 和受权限约束的占用申请引用；任一强重复阻断整单。
- Evidence 保留检查已覆盖申请级 `invoices[].evidence`，历史 revision 引用不会被未引用文件清理误删。
- 旧 `line.invoice` 继续读取、展示并参与重复判断；编辑未提交旧草稿时可转换为新关联结构，历史 revision 不改写。
- 付款申请继续使用原单发票表单和 API，未被本批改造成多发票。

## 所有权与不变量

- Internal Finance Requests 拥有报销申请、费用明细、申请内发票关联和业务时快照。
- Invoice Recognition 拥有识别请求、Provider 输出、候选与确认历史；Finance 只保存稳定 Proposal ID／revision。
- Evidence Service 拥有文件字节与版本；Finance 只保存 File ID／Version 和经解析验证的摘要元数据。
- 没有共享表、跨库查询或通用 Invoice Owner；本次扩展写入既有 JSONB 快照，不需要 schema migration。
- 一条明细最多一张发票；一张发票关联只能指向当前申请内存在的一条明细。

## 已运行验证

- Internal Finance Requests：`go test ./...` 通过；`go vet ./...` 通过。
- Unified Workbench：`go test ./...` 通过；`go vet ./...` 通过。
- Unified Workbench：`node --test internal/server/*.test.mjs`，309/309 通过。
- Finance Owner JavaScript：`node --check` 通过。
- PostgreSQL 多发票集成用例已加入；发布准备阶段使用临时 PostgreSQL 18.6 实例实际运行 `go test -count=1 -tags=integration ./internal/finance ./internal/httpserver` 并通过，实例随后停止并自动移除。

## 后续发布结果

- PostgreSQL 18.6 集成、P9 preflight、D1 composition、发布包、升级前联合备份、正式 D1 check、21/21 长运行容器和静态资源逐字节摘要均已通过。
- 公网版本为 `v0.1.0-test.oms-v1.104`；本地记录本身不替代[发布记录](./oms-reimbursement-multi-invoice-v104-publication-20261116.md)。
- 发布验收没有创建或修改报销数据；真实 Chrome 的单项／多项写旅程交由用户使用自有测试数据完成。
