# Contract Ledger：操作性关闭

里程碑：M2  
功能编号：CL-M2-006  
日期：2026-09-19  
状态：已完成并验证

## 目标

用户能够在一份已经形成正式 revision、且不再需要普通合同业务写入的合同上，记录显式、不可变的操作性关闭事实。关闭把 Contract 状态推进为 `closed`，但不伪造法律终止、自然到期、履约完成、最终结算或财务清偿，不形成新 Contract Instrument 或 revision，也不调用 Money Flow Ledger。

## 关闭与其他事实的边界

- `cancelled`：草稿从未生效即取消；
- `terminated`：终止文书已经生效，是法律文书驱动的状态；
- `final Settlement`：确认最终累计结算金额，不自动关闭合同；
- `closed`：操作者确认本系统内的常规合同维护已结束，后续只读并等待未来受控纠错能力。

关闭可以从 `active` 或 `terminated` 发生：

- active 合同可以因履约已经完成、期限已经届满且无需终止文书等原因关闭；
- terminated 合同可以在终止后的结算或收尾确认完成后关闭；
- 系统不根据日期自动判断到期，也不从自由文本推导法律效果。

## 结算处置必须明确

关闭不能静默忽略结算，也不能要求所有合同伪造一条零金额最终结算。命令必须明确选择以下一种处置：

- `final_recorded`：本系统已经存在 final Settlement；repository 必须验证最新 Settlement 确实为 final，并在关闭事实中引用它；
- `not_required`：操作者明确确认该合同无需另行登记 final Settlement；repository 必须验证当前不存在 final Settlement。

若仍需要未来登记结算，就不应关闭合同。已有 progress Settlement 但不再需要独立 final 文书时，可以选择 `not_required`；既有 progress 历史保持不变。

## 关闭事实

每条 Contract Closure 保存：

- 独立 Closure ID 和幂等 Command ID；
- Contract ID；
- 关闭时的 source contract revision；
- 关闭命令观察到的 settlement sequence；
- `final_recorded` 或 `not_required` 结算处置；
- 若为 `final_recorded`，引用 final Settlement ID；
- 必填关闭日期；
- 必填关闭依据或原因，最多 1000 个字符；
- 可信 Actor 和记录时间。

同一 Contract 最多一条 Closure。关闭事实、命令和审计不可更新、不可删除。

## 日期规则

- 关闭日期使用用户明确输入的本地业务日期，不从服务器时间推断；
- 关闭日期不得早于当前 revision 来源 Instrument 的生效日期；
- 存在 Settlement 时，关闭日期不得早于最新 Settlement 的确认日期；
- 当前功能不接受未来预约并自动关闭，提交即表示关闭事实已经成立。

## 状态、顺序与并发

- 只有 `active` 或 `terminated` 可以关闭；
- `draft`、`cancelled`、`closed` 拒绝新的关闭命令；
- 命令携带 Expected Contract Revision 和 Expected Settlement Sequence；
- repository 锁定 Contract 后核对 current revision、当前最大 settlement sequence 和结算处置；
- 关闭与补充协议、终止、结算共用 Contract 行锁形成确定顺序；
- 若其他写入先提交，过期关闭命令得到 revision、sequence 或状态冲突；
- 若关闭先提交，后续补充协议、终止和结算因 `closed` 状态被拒绝；
- 关闭只改变 Contract 生命周期投影，不推进 revision，不修改任何历史事实。

## 幂等、原子性与不可变性

- 同一 Command ID 与相同负载重放返回原 Closure，不重复审计；
- 同一 Command ID 改变合同、revision、settlement sequence、处置、日期、原因或 Actor 时返回幂等冲突；
- Closure、command、Contract 状态更新和 `contract_closed` 审计在一个事务内提交；
- 审计写入失败时不得留下 Closure、command 或 `closed` 状态；
- 已关闭合同不重新打开；错误关闭等待未来独立的受控纠错事实。

## 页面与权限

- active/terminated 合同详情显示“操作性关闭”表单；
- 页面显示当前 revision、最新结算状态及即将采用的结算处置；
- 有 final Settlement 时只能按 `final_recorded` 关闭；没有 final 时必须明确勾选“无需单独最终结算”；
- 关闭成功后详情和列表显示“已关闭”，并显示关闭日期、依据、结算处置、source revision、Actor 和记录时间；
- closed 页面不显示补充协议、终止、结算或再次关闭表单；
- administrator/editor 可以关闭，viewer 只读且直接 POST 返回 `403`；
- 请求必须包含 CSRF、Command ID、Expected Contract Revision 和 Expected Settlement Sequence；Actor 只来自登录会话；
- 校验错误返回 `422`，状态/revision/sequence/结算处置/幂等冲突返回 `409`，不存在返回 `404`。

## 审计

成功写入 `contract_closed` 通用审计，归属 Contract，至少包含 Closure ID、source revision、settlement sequence、结算处置、final Settlement ID（若有）、关闭日期和原因。幂等重放不重复审计。

## 明确不做

- 自动根据有效期或服务器任务关闭；
- 把关闭解释为法律终止、自然到期、履约完成或财务清偿的自动证明；
- 自动生成 final Settlement 或把 progress 改成 final；
- 创建应收、应付、资金、核销或 Money Flow 动作；
- 文件上传、Evidence Reference、归档包或销毁策略；
- 重新打开、撤销、删除或直接修改关闭事实；
- 在本功能内实现受控纠错。

## 验收标准

1. active 与 terminated 合同可关闭，状态推进为 closed，但 current revision、Instrument、金额和 Settlement 历史不变。
2. final 已存在时只接受 `final_recorded` 并引用真实 final Settlement；没有 final 时只接受明确的 `not_required`。
3. progress 存在但无需独立 final 时可选择 `not_required`，旧 progress 事实保持不变。
4. draft、cancelled、closed、过期 revision、过期 settlement sequence 和不一致结算处置明确拒绝。
5. 关闭日期早于当前 Instrument 生效日期或最新 Settlement 确认日期时拒绝。
6. 同一 Command ID 可安全重放，改变任一负载字段产生幂等冲突。
7. 关闭与补充、终止或结算并发时只按 Contract 锁定顺序接受仍满足前置条件的一方，不静默覆盖。
8. Closure、command、状态和审计原子提交；审计失败不留下部分事实。
9. Closure、command 和审计更新／删除均被 PostgreSQL 不可变保护拒绝。
10. administrator/editor 可关闭；viewer 可读但无表单，直接 POST 为 `403`。
11. 从空数据库经 HTTP 完成激活、阶段结算、终止、最终结算和关闭，页面、列表、数据库及可信 Actor 审计一致。
12. Money Flow 数据库和应用停止时仍能完成全部验收，不产生跨系统事实。
