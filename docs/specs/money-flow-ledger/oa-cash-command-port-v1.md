# OA 实际资金原生端口与原组件

问题 A → 可验证结果 B：OA 的两列支付只有只读连接，无法在项目内登记实际资金 → 复用原支付列表／640px 表单，在 Money 保存精确实际收款，Project 单独保存有界项目关系；未知回复只续原未完成命令。

## 事实与不变量

- Money 自有 Cash Transaction、双方发生时快照、实际金额／账户／日期、冲销、命令及审计。实际资金不能原地编辑或物理删除；纠错先明确冲销，已核销资金须先反向 Allocation。原 Cash 用例不新增客户字段或 OA 宽 FinancialRecord。
- Reference OA 装配仅把两链和三费用分类编码为 Project 自有关系键 `reference-oa-cash-v1-{client|income}-{design|survey|consultation}`；该关系不改变 Money 的金额、付款人或收款人，不是 Money 业务表。只有这六种关系进入本组件。没有显式链的旧连接不猜分类。旧冒号编码被真实 Project 拒绝，不能作为合法规范；修复符合现有短键规则，不放宽 Project 契约。
- 甲方实付与公司实收是两条独立实际收款，必须显式选择 Money 当前本目录的收款账簿主体、付款主体及账户，不以甲方文本／公司名称匹配身份，不从合同金额自动造资金，不隐式复制另一列。
- CNY 金额使用十进制字符串；日历日期必填，不能用今天掩盖缺失。主体和原生能力由目标 Money 验证。无金额读取能力不能盲写／冲销；原组件只显示服务端收敛视图。
- 新登记使用租户凭证；查看和冲销接受原生 `cash_transaction` 精确范围。只读 reference.read 仍为单 action／单 `transaction` 范围，不能取得人类命令能力。
- 上传／开票、发票关联、义务／Allocation、Receipt Set／奖金发布不是本资金切片负责的问题。关联发票未接通时明确禁用，不把资金创建解释成开票完成。

## API 与阶段

`GET /api/v1/cash-workspace` 返回权限与当前本目录主体；精确凭证不给全目录。`GET /api/v1/cash-transactions` 是受 IAM、Tenant Scope 和 Money 最终授权保护的跨项目只读投影，支持 `type`、`status`、业务日期区间、精确 `source_reference`、`offset` 与最多 100 条的 `limit`；默认只返回 `recorded`，按业务日期、登记时间和稳定 ID 倒序，缺少金额读取能力时继续返回记录但金额为 `null`。`POST /api/v1/cash-transactions` 接受稳定 command_id 和 Cash 原生字段；`GET /api/v1/cash-transactions/{id}` 返回自有资金及原生 allowed_operations；`POST /api/v1/cash-transactions/{id}/reversal` 接受稳定 command_id／原因，调用已有事务用例。严格 JSON，可信 Actor，身份 Header 清洗。不存在匿名或静态 reference Token 写入口。

列表接口不拥有 OMS 项目关系，也不按名称猜项目。只有来源应用已经写入稳定且版本化的 `source_reference` 时，装配层才能把该引用解释为项目来源；否则必须显示“来源待核对”，不能跨库补表或静默归组。该列表只增加 Money 自有读取端口，不改变 Cash Transaction 数据结构、资金事实所有权或冲销规则。

创建 Money → 保留 ID → Project Link → GET 原生复读。每段独立提交、稳定命令；Link 失败不重建资金，回复未知不换 ID；409 不自动覆盖，403 不换模板身份。只读／刷新／关闭零写入。Project 仅在明确范围、版本及目标 reference 成功后记录连接。

验收必须覆盖精确金额、两链／六分类、可信 Actor、相邻 Scope、金额脱敏与盲写拒绝、冲销／已核销冲突、未知回复／Link 故障继续、原组件真实点击、PG18 及独立启动。测试端口不冒充真实组合，未发布只能称候选；整站交付按完整 OA 清单继续。
