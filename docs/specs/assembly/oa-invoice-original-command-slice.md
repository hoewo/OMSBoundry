# 原 OA 开票组件接线

问题 A → 可验证结果 B：同项目开票仍是只读投影，不能登记和纠错 → 复用 InvoiceRecordList、OurInvoiceForm 和原附件预览，登记、刷新、纠正、登记无效及文件业务引用分别得到原生回执，故障只续办未完成段。

2026-10-04 候选施工，未部署。原站提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`；原目录只读。遵循 BF-050～BF-054、BF-084、BF-096，Billing 仍只登记已开具票据，不产生税务作废、红冲、应付款或实际资金。

## 不变量和不负责的相邻问题

Billing 独占金额、双方快照、Project 来源、revision／命令／历史／审计和 Evidence 引用。Project 只通过有界 `reference-oa-invoice-v1-{client|income}-{design|survey|consultation}` 六种关系键归类，不把客户费用字段加入 Billing Base；Workbench 无事实库。关系键须满足现有 Project 短键约束，旧冒号编码不是合法契约。选择 Party Directory 的 active ID／revision，Billing 仍以精确工作负载凭证复验；不按甲方名称匹配，不自动创造主体。

金额使用整数分和 CNY。原 v1 int64 JSON 请求保持；前端逐字段编码经过 int64 范围校验的数字字面量，不经 Number 转换。响应兼容增加 `amount_minor_decimal` 十进制字符串，当前／历史金额无权时所有金额表示均为 null。登记需要原生 record 与 amount.read，列表 record 动作同样收敛，不能盲写。日期明确输入日历字符串，不默认今天。

创建→保留 Invoice ID→Project Link→原生 GET 分段幂等。纠正使用 expected revision、明确原因并追加历史；登记无效不是删除文件或税务作废。未知回复保留 scoped 原命令／负载；409 不自动换版本，403 不换身份。已有 Project Link 的 source revision 表示关联发生时版本，纠正不静默改旧引用，当前详情明确显示新 revision。

附件只能先由 Evidence SDK 上传并完成校验，再由 Billing 原 PUT evidence 命令保存版本集合；上传和业务登记不混报成功。Billing 工作负载复验使用单一 `evidence-service.metadata.read`／精确 File，不借遗留 view 获得下载权；正式装配必须显式匹配 workload policy，不能单独发布不兼容二进制。保留已完成 File／Version，在回复未知或元数据失败后续办，不重上传；移出仅更正 Billing 业务引用，不删除字节。原一对一支付关联仍未实现，不以禁用控件关闭缺口；不复制共享 FinancialRecord、fileService 物理删除或客户角色判断。

## 门禁

原两链三费用的登记／刷新；明确双方／日期／精确大额；纠正原因／revision／历史；登记无效保留原事实；文件先完成再登记、预览／下载／移出；创建、Link、纠正及附件丢回复续办；只读／脱敏零写和跨项目拒绝。逻辑、原组件、PG18、正式 issuer 联合、发布和公网分别记录，未运行不称通过。
