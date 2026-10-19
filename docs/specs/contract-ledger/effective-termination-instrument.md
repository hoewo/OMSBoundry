# Contract Ledger：生效终止文书

里程碑：M2  
功能编号：CL-M2-002  
日期：2026-09-19  
状态：已实现并验证

## 目标

用户能够在一份 active 合同上登记已经确定生效的终止文书。该文书作为合同时间线中的下一份不可变 Instrument，在一个 PostgreSQL 事务内形成下一条 revision、复制当时有效的签约主体快照、把合同状态推进为 `terminated`，并追加可信写入审计。

本功能只记录已经发生并由用户确认的终止事实，不判断文书的法律效力，不建设终止申请、审批、通知送达、结算或关闭流程。

## 终止与其他状态的边界

- `cancelled`：草稿在激活前取消，没有正式 revision；
- `active`：合同已经激活，仍可追加受支持的生效文书；
- `terminated`：终止文书已经生效，合同不再接受新的补充协议或再次终止；
- `closed`：保留给后续完成结算、归档等操作性关闭，不在本功能中使用。

终止不等于结算。终止时不得录入结算金额、已履行金额、应收应付、退款或核销，不创建或修改 Money Flow Ledger 事实。

## 终止 Instrument

Instrument 类型固定为 `termination_instrument`，保存：

- 稳定 Instrument UUID；
- 所属 Contract UUID；
- 合同内不区分大小写的唯一文书编号；
- 文书标题；
- 必填终止原因或依据说明；
- 签署或出具日期；
- 终止生效日期；
- 显式生效顺序；
- 固定的金额语义 `unchanged`；
- 来源 Command ID；
- 产生的合同 revision；
- Actor 快照和登记时间。

终止文书生效后禁止更新和删除。系统不通过自由文本把补充协议伪装成终止，也不允许把终止文书重新排序或插入历史中间位置。

## 顺序与日期

- Contract 必须为 `active`；
- Expected Revision 必须等于当前 revision；
- `effective_order` 必须等于当前 revision 加 1；
- 产生的 revision 必须等于 `effective_order`；
- 终止生效日期不得早于前一份 Instrument 的生效日期；
- 同日存在补充协议和终止文书时，必须按已确认法律顺序逐份登记；
- 终止成功后，不再接受后续补充协议或终止文书。

本功能不允许先登记未来计划再自动到期生效；操作者提交即确认该终止事实已经生效。系统保存输入的业务日期，不根据服务器时区推断法律日期。

## revision 与金额

终止文书产生且只产生一个新 revision。新 revision：

- 完整复制上一 revision 的编号命名空间、合同编号、标题、分类、币种、原合同签署日期和原合同生效日期；
- 完整复制上一 revision 的有效总额，包括“金额待确定”；
- 完整复制上一 revision 的 Party Snapshot；
- 引用终止 Instrument，保存执行 Actor 和形成时间；
- 不修改任何旧 revision、旧 Instrument 或旧 Party Snapshot。

终止后的当前有效金额仍表示终止前最新 revision 的合同金额，不是结算金额。后续结算功能必须建立独立事实，不能覆盖该金额。

## 命令、并发与幂等

输入：

- Command ID；
- Contract ID；
- Expected Revision；
- Instrument Number、Title、Termination Reason；
- Signed On、Effective On、Effective Order；
- 由登录会话生成的 Actor。

规则：

- 相同 Command ID 和相同完整负载重放时返回原结果，不重复创建 Instrument、revision、快照或审计；
- 相同 Command ID 改变任一业务负载或 Actor 时返回幂等冲突；
- 不同命令使用重复文书编号、重复生效顺序或过期 revision 时拒绝；
- 两个终止命令并发消费同一 revision 时只有一个成功；
- 补充协议与终止命令并发消费同一 revision 时也只有一个成功，失败方得到明确状态或 revision 冲突；
- 幂等重放优先于当前 `terminated` 状态检查。

## 原子写入与审计

一次成功终止必须原子完成：

1. 保存不可变 `termination_instrument`；
2. 保存不可变 termination command；
3. 保存新的不可变 revision；
4. 复制新的不可变 revision Party Snapshot；
5. 同时推进 Contract `current_revision` 和状态 `terminated`；
6. 写入 `contract_terminated` 通用不可变审计。

审计保存 Command ID、Instrument ID、原因、Expected Revision、Resulting Revision、保持不变的有效金额、Actor 和发生时间。任一步失败都整体回滚。幂等重放不新增审计。

## 查询与页面

- 合同列表和详情明确显示“已终止”及当前 revision；
- 时间线把该文书显示为“终止文书”，并展示编号、原因、签署／出具日期、终止生效日期和 Actor；
- revision 历史保留终止前后的完整快照；
- administrator 和 editor 可以在 active 合同详情登记终止文书；
- viewer 只读，不渲染终止表单，直接 POST 返回 `403`；
- terminated 页面不渲染补充协议或终止写入表单；
- 成功页面显示新 revision、终止状态和可信审计。

## 明确不做

- 终止申请、审批、法务判断、通知送达和电子签章；
- 预约未来自动终止；
- 撤销终止、恢复 active 或受控更正；
- 自然到期、履约完成和操作性关闭；
- 结算金额、退款、赔偿、应收应付和核销；
- 文件二进制或 Evidence Reference；
- 创建、修改或通知 Money Flow Ledger 的任何事实；
- 事件 outbox 或跨系统连接器。

## 验收标准

1. active 合同可以登记终止文书，形成下一 Instrument、下一 revision 和 `terminated` 状态。
2. 终止 revision 完整复制上一 revision 条款、有效金额和全部 Party Snapshot，旧事实不变。
3. 金额已确定和金额待确定两类合同终止后都保持原值，不产生结算金额。
4. 草稿、已取消或已终止合同，过期 revision、非连续顺序、倒退生效日期和重复文书编号均拒绝且不产生部分事实。
5. 命令可安全幂等重放；改变负载时冲突；终止与终止、补充协议与终止的并发写不会静默覆盖。
6. Instrument、termination command、revision、Party Snapshot 和审计在数据库层禁止更新与删除。
7. 故意制造审计或快照写入失败时，状态、current revision 和全部新事实一起回滚。
8. 终止后所有补充协议和再次终止入口消失，直接命令也被拒绝。
9. administrator/editor 可以执行，viewer 的页面和直接请求都不能写入；Actor 只能来自会话。
10. 页面、列表和数据库一致显示终止状态、当前金额、Instrument 时间线、revision 历史和审计。
11. Money Flow 数据库和应用停止时仍能完成全部验收，且不会产生任何跨系统事实。
