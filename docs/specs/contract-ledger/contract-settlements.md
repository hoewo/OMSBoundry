# Contract Ledger：阶段与最终结算事实

里程碑：M2  
功能编号：CL-M2-005  
日期：2026-09-19  
状态：已完成并验证

## 目标

用户能够在已经形成正式 revision 的合同上，按确定顺序记录经过业务主体确认的阶段结算或最终结算事实。结算金额与原始合同金额、当前有效合同金额分别保存；结算不修改合同文书和 revision，不自动创建应收应付，也不要求 Money Flow Ledger 在线。

## 解决“单次最终还是分阶段结算”

第一版同时支持：

- `progress`：阶段结算，表示该文书截至当次确认的**累计结算金额**；
- `final`：最终结算，表示合同最终确认的**累计结算金额**。

每一条结算都是独立、不可变的 Settlement Fact，并使用从 1 开始、严格连续的 `settlement_sequence` 排序。阶段结算不是增量金额，不能把多条阶段金额相加；当前结算投影只读取最新 sequence 的累计金额。最终结算最多一条，一旦存在，不再接受新的阶段或最终结算。

这套模型既不强迫所有合同只有一次结算，也不预设行业计量、进度款或费用明细算法。

## 结算事实

每条 Settlement 保存：

- 独立 Settlement ID 和幂等 Command ID；
- Contract ID；
- 建立结算时的 source contract revision；
- `progress` 或 `final` 类型；
- 严格连续的 settlement sequence；
- 合同内不区分大小写唯一的结算文书编号；
- 标题；
- 精确、非负的累计结算金额；
- 从 source revision 继承的币种；
- 确认日期；
- 必填结算依据或口径说明，最多 1000 个字符；
- 用户明确勾选的一个或多个确认主体快照；
- 可信 Actor 和记录时间。

金额以十进制文本进入领域层并保存为 PostgreSQL `numeric(24,6)`，允许 `0`，禁止负数、指数形式、超过 18 位整数或超过 6 位小数。即使当前有效合同金额待确定，只要结算文书已经确认金额，仍可记录结算；系统不猜测两者差额原因。

## 三种金额必须分离

- 原始金额：revision 1；
- 当前有效金额：当前 contract revision；
- 当前／最终结算金额：最新 Settlement，若类型为 `final` 则为最终结算金额。

结算不得更新 Contract 根表金额、任何 revision 金额或 Instrument 金额。当前有效金额与结算金额可以相同、增加、减少或一方为空。页面必须并列展示，不能使用“合同金额”一个字段混淆。

## 确认主体快照

- 表单列出当前 source revision 的 Party Snapshot；
- 用户至少选择一个确认主体 position；
- repository 在同一事务中核对所选 position 属于 Expected Contract Revision；
- 成功后复制所选主体的 Party ID、Party revision、名称、登记标识和合同角色；
- 后续 Party 当前资料或合同 revision 变化不改写结算确认主体；
- 当前功能不把“所有合同签约主体”自动解释成“所有人都确认了结算”。

## 状态、顺序与并发

- `active` 和 `terminated` 合同可以记录结算；
- `draft`、`cancelled` 尚无正式合同事实，不能结算；
- `closed` 表示常规业务已经结束，当前功能只读，不允许新增结算；
- 命令携带 Expected Contract Revision 和 Expected Settlement Sequence；
- repository 锁定 Contract，核对 current revision 和当前最大 settlement sequence；
- 成功 sequence 必须等于 Expected Settlement Sequence + 1；
- 同一合同并发结算命令只能有一个消费当前 sequence；
- 结算与补充协议或终止命令并发时，以 Contract 行锁形成确定顺序：若合同 revision 先变化，旧结算命令得到 revision 冲突；若结算先提交，它明确引用提交时的旧 revision；
- 结算不会推进 Contract revision 或状态。

## 幂等、唯一性和不可变性

- 同一 Command ID 与相同负载重放返回原 Settlement，不新增 sequence、快照或审计；
- 同一 Command ID 改变合同、revision、sequence、类型、编号、金额、日期、依据、确认主体或 Actor 时返回幂等冲突；
- 同一合同内大小写不同但等价的结算编号返回冲突；
- Settlement、command、确认主体快照和通用审计在一个事务内提交；
- 所有已确认结算事实禁止更新和删除；
- 阶段结算可以由后续更高 sequence 的阶段或最终结算给出新的累计口径，但旧事实仍保留；
- 最终结算如有错误，只能等待独立的受控纠错规格，不能重新打开或直接修改。

## 页面与权限

- 正式合同详情增加结算时间线；
- 页面并列展示当前有效金额和最新累计结算金额；
- 每条结算显示 sequence、类型、编号、标题、金额、确认日期、依据、source revision、确认主体、Actor 和记录时间；
- active/terminated 且尚无 final Settlement 时，administrator/editor 可以看到登记表单；
- 表单携带 CSRF、Command ID、Expected Contract Revision 和 Expected Settlement Sequence；
- viewer 只能读取；直接 POST 返回 `403`；
- 成功后重定向详情并显示结果；
- 校验错误返回 `422`，状态、revision、sequence、重复编号、final 后继续写入和幂等冲突返回 `409`，不存在返回 `404`；
- Actor 只来自登录会话，请求字段不能覆盖。

## 审计

成功写入 `contract_settlement_recorded` 通用审计，归属 Contract，包含 Settlement ID、类型、sequence、编号、source revision、累计金额、币种、确认日期和确认主体 positions。幂等重放不重复审计。

## 明确不做

- 应收、应付、发票、付款、收款、退款、赔偿或核销；
- 自动调用 Money Flow Ledger 或发布义务建议；
- 行业进度计量、费率、税额、质保金和结算明细算法；
- 把阶段金额相加得到累计金额；
- 多币种结算或汇率换算；
- 结算文件上传、Evidence Reference 或电子签章；
- 最终结算自动关闭合同；
- 修改、撤销、删除最终结算或受控纠错。

## 验收标准

1. active 与 terminated 合同可以按连续 sequence 记录多条 progress 和最多一条 final Settlement。
2. draft、cancelled、closed、final 后继续写入、过期 contract revision 和过期 settlement sequence 明确拒绝。
3. 阶段金额按累计口径读取，不求和；最终金额与原始／当前有效金额三者分别保存和展示。
4. 零金额、当前有效金额待确定和结算金额高于／低于当前金额均能精确保存；非法金额明确拒绝。
5. 至少选择一个当前 revision 的确认主体；不存在、重复或属于其他 revision 的 position 拒绝。
6. 确认主体复制不可变快照，Party 当前资料和后续 contract revision 变化不改写旧结算。
7. 同一 Command ID 重放不重复写入；改变任一负载字段产生幂等冲突。
8. 合同内编号大小写唯一；两个并发命令消费同一 sequence 时恰好一个成功。
9. Settlement、command、确认主体和审计原子提交；故意制造末端审计失败时没有部分事实。
10. 直接更新或删除四类结算事实时 PostgreSQL 返回不可变事实错误。
11. administrator/editor 可登记；viewer 可读但页面无表单，直接 POST 为 `403`。
12. 从空数据库经 HTTP 完成合同激活、阶段结算、终止、最终结算，页面、数据库和可信 Actor 审计一致。
13. Money Flow 数据库和应用停止时仍能完成全部验收，不产生任何跨系统事实。
