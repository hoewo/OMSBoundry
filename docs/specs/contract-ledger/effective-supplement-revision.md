# Contract Ledger：生效补充协议与 revision 2+

里程碑：M2  
功能编号：CL-M2-001  
日期：2026-09-19  
状态：已实现并验证

## 目标

用户能够在一份 active 合同上登记已经签署、顺序已经确认的补充协议。每份补充协议是不可变法律文书，必须在一个 PostgreSQL 事务内形成下一条不可变合同 revision、复制当时有效的签约主体快照、推进 Contract 当前 revision，并追加可信写入审计。

本功能建立可重放合同变更时间线的第一条纵向闭环，不建设通用流程、万能变更表或补充协议起草平台。

## 本功能中的 Instrument

M1 的原合同激活事实迁移为时间线中的 `original_contract` Instrument，顺序固定为 1、结果固定为 revision 1。新登记的文书类型固定为 `supplemental_agreement`。

每份补充协议保存：

- 稳定 Instrument UUID；
- 所属 Contract UUID；
- 不区分大小写的合同内唯一文书编号；
- 文书标题和必填变更说明；
- 签署日期与生效日期；
- 显式生效顺序；
- 金额影响语义和可选金额值；
- 来源 Command ID；
- 产生的合同 revision；
- Actor 快照和登记时间。

Instrument 生效后禁止更新和删除。当前功能只登记已经确定顺序的生效文书，不支持补充协议草稿、审批、撤回或物理删除。

## 显式生效顺序

- 原合同 Instrument 的顺序为 1；
- 新补充协议的 `effective_order` 必须等于当前 Contract revision 加 1；
- 产生的 revision 必须等于 `effective_order`；
- 同一天可以有多份补充协议，但必须按法律上确认的顺序逐份登记；
- 不使用数据库创建时间、UUID 或标题猜测同日顺序；
- 过期 Expected Revision 或非连续顺序都拒绝，不写任何部分事实；
- 已登记文书不能重新排序。历史顺序错误由后续受控更正功能处理，不能直接改表。

生效日期不得早于时间线中前一份 Instrument 的生效日期。当前功能不接受需要插入既有历史中间位置的追溯变更。

## 金额影响语义

Contract 继续限定单一币种。补充协议沿用原合同币种，不执行换汇。

金额影响只允许：

| 语义 | 输入 | 结果 |
| --- | --- | --- |
| `adjustment` | 必填非零、有符号十进制差额 | 上一 revision 有效金额加差额 |
| `restatement` | 必填非负十进制金额 | 用新金额替代上一 revision 有效金额 |
| `unchanged` | 金额值必须为空 | 完整复制上一 revision 有效金额 |

规则：

- 金额最多 18 位整数、6 位小数，禁止科学计数法和二进制浮点数；
- 上一 revision 金额未确定时不能使用 `adjustment`，只能重述为确定金额或保持未确定；
- 调整后的有效金额不能小于零，也不能超出精度边界；
- `unchanged` 可以继续保留“金额待确定”；
- 原始金额来自 revision 1，当前有效金额来自最新 revision；Contract 根表中的草稿金额不是 active 合同当前金额的第二真相源；
- 本功能不记录结算金额，结算必须是后续独立事实。

## revision 2+

每份补充协议产生且只产生一个新 revision。新 revision：

- 完整复制上一 revision 的编号命名空间、合同编号、标题、分类、币种、原合同签署日期和原合同生效日期；
- 根据金额影响语义计算并保存新的有效总额；
- 引用产生它的 Instrument；
- 保存执行 Actor 和形成时间；
- 复制上一 revision 的全部 Party Snapshot，保持签约主体历史稳定；
- 不修改任何旧 revision 或旧 Party Snapshot。

当前功能不改变主体、角色、合同标题、分类、币种或原合同日期。将来这些变化必须由类型化 Instrument 行为显式支持，不能借用自由文本绕过。

## 命令、并发与幂等

输入：

- Command ID；
- Contract ID；
- Expected Contract Revision；
- Instrument Number、Title、Summary；
- Signed On、Effective On、Effective Order；
- Amount Effect 与 Amount Value；
- 由登录会话生成的 Actor。

规则：

- Contract 必须为 `active`；
- Expected Contract Revision 必须等于当前 revision；
- 相同 Command ID 和相同完整负载重放时返回原结果，不重复创建 Instrument、revision、主体快照或审计；
- 相同 Command ID 改变任一业务负载或 Actor 时返回幂等冲突；
- 不同命令使用同一文书编号或同一生效顺序时返回冲突；
- 并发命令只有一个能消费当前 revision，另一个得到明确 revision 冲突；
- 幂等重放优先于当前状态与 revision 检查。

## 原子写入与审计

一次成功命令必须原子完成：

1. 保存不可变 Instrument；
2. 保存不可变 change command；
3. 保存新的不可变 revision；
4. 复制新的不可变 revision Party Snapshot；
5. 推进 Contract `current_revision`；
6. 写入 `contract_change_activated` 通用不可变审计。

审计保存 Command ID、Instrument ID、金额语义和值、Expected Revision、Resulting Revision、有效金额结果、Actor 和发生时间。任一步失败都整体回滚。幂等重放不新增审计。

## 查询与页面

- active 合同详情显示当前有效金额，不继续把 Contract 草稿金额当成当前值；
- 显示从原合同到全部补充协议的 Instrument 时间线；
- 显示 revision 历史及每个 revision 的有效金额；
- editor 和 administrator 可以在 active 合同详情登记补充协议；
- viewer 只读，不渲染表单，直接 POST 返回 `403`；
- 表单携带 CSRF、Command ID、Expected Revision 和下一显式生效顺序；
- 成功后页面显示新的当前 revision、金额和审计记录。

## 明确不做

- 补充协议草稿、内部审批、法务审查和电子签章；
- 主体、角色、币种、标题或原合同日期变更；
- 终止、解除、结算、关闭和受控更正；
- 金额分类明细、税额、汇率或会计处理；
- 插入或重排已经形成的历史 Instrument；
- 文件二进制或 Evidence Reference；
- 创建、修改或通知 Money Flow Ledger 的任何义务；
- 事件 outbox 或跨系统联调。

## 验收标准

1. M1 active 合同迁移后拥有顺序 1 的原合同 Instrument，revision 1 内容和 Actor 不被改写。
2. active 合同可以连续登记多份补充协议，分别形成 revision 2、3……；时间线顺序由 `effective_order` 明确决定。
3. adjustment、restatement、unchanged 三种金额语义按十进制精确演算，金额未知、负结果、零调整、超精度和非法组合明确拒绝。
4. 每个新 revision 完整复制上一 revision 的非金额条款和 Party Snapshot，旧事实保持不变。
5. 非 active 合同、过期 revision、非连续顺序、倒退生效日期、重复文书编号均拒绝且不产生部分事实。
6. 命令可安全幂等重放；改变负载时冲突；并发写不会静默覆盖。
7. Instrument、change command、revision、Party Snapshot 和审计在数据库层禁止更新与删除。
8. 故意制造审计或快照写入失败时，整个变更事务回滚。
9. administrator/editor 可以执行，viewer 的页面和直接请求都不能写入；Actor 只能来自会话。
10. 详情页与数据库一致显示当前有效金额、Instrument 时间线、revision 历史和审计。
11. Money Flow 数据库和应用停止时仍能完成全部验收，且不会产生任何跨系统事实。
