# Contract Ledger：已生效合同核心条款受控更正

里程碑：M2  
功能编号：CL-M2-008  
日期：2026-09-19  
状态：已完成并验证

## 目标

当当前正式 Contract Revision 中的标题、通用分类、合同金额、签署日期或生效日期因录入、迁移或确认错误而不准确时，管理员能够追加一份类型化更正文书，形成新的不可变 Revision。旧 Revision 和依赖旧 Revision 形成的结算、关系、关闭、证据与审计继续保留，系统不执行原地修改、删除或伪装回填。

本功能修正的是 Contract Ledger 中的结构化记录，不自行判断法律文书是否真实有效，也不把新的商业协商包装成“技术性更正”。当双方实际改变合同条款时，仍应登记补充协议。

## 纠错边界

第一版只更正当前 Revision 中以下完整快照字段：

- 标题；
- 通用分类，可改为空；
- 当前有效合同金额，可改为待确定；
- 合同签署日期；
- 合同生效日期。

编号命名空间、合同编号、币种和签约主体快照从当前 Revision 原样复制，不能通过本命令改变。原因是这些字段分别涉及业务身份唯一性、跨币种金额语义和主体身份纠错，不能混入一个无约束表单。

关系、Settlement、Closure 和 Evidence 也是独立事实。本功能不撤销、不重算、不重挂这些事实；它们继续引用形成时的 Revision 或目标。若旧事实本身错误，需要后续各自的类型化反向或更正能力。

## Correction Instrument 与新 Revision

每次成功更正追加：

- 一条 `correction_instrument` Contract Instrument；
- 一条从当前 Revision 完整演算出的 Revision N+1；
- 一条不可变更正命令；
- 一条 `contract_correction_recorded` 通用审计。

更正文书保存：

- 独立 Instrument ID；
- 文书编号和标题；
- 必填更正原因或依据，最多 1000 个字符；
- 文书签署／出具日期和生效／确认日期；
- 显式 Effective Order，必须等于当前 Revision + 1；
- 被更正的当前 Revision 和结果 Revision；
- 更正后的完整字段快照；
- 可信 Actor 和记录时间。

Instrument 的文书生效日期不得早于前一份 Instrument 的生效日期，保证追加时间线有确定顺序。更正后的“合同签署日期／合同生效日期”属于 Revision 内容，可以与更正文书日期不同。

## 状态规则

- `active`、`terminated`、`closed` 可以追加更正；
- `draft` 应直接走草稿编辑，不创建正式更正；
- `cancelled` 没有正式 Revision，不能更正；
- 更正后 Contract 生命周期状态保持不变；
- terminated 不恢复 active，closed 不重新打开；
- 更正不会自动允许新的补充协议、Settlement 或 Closure，后续能力仍按更正后的状态执行。

命令必须同时携带 Expected Status 与 Expected Revision。repository 在同一 Contract 行锁下核对二者，避免操作者基于 active 页面提交时，另一事务已经完成终止或关闭，而更正仍被静默接受。操作者刷新并看到最新状态后，可以明确地对 terminated 或 closed 合同重新提交更正。

## 更正字段规则

- 标题必填，最多 200 个字符；
- 分类可空，最多 100 个字符；
- 金额为空表示待确定，否则必须是非负、最多 18 位整数和 6 位小数的十进制数；
- 合同签署日期和生效日期必填且使用 `YYYY-MM-DD`；
- 更正后的五个字段至少一个与当前 Revision 不同，否则返回“没有变化”；
- 金额仍使用 PostgreSQL `numeric(24,6)`，不得经过二进制浮点数；
- 新 Revision 完整复制编号、币种和全部 Party Snapshot，再写入更正后的五个字段。

## 幂等、并发与原子性

- 每次提交使用 Command ID；相同命令与相同标准化意图重放返回原结果，不重复产生事实或审计；
- 相同 Command ID 改变任一输入字段、Expected Status、Expected Revision 或 Actor，返回幂等冲突；
- 更正与补充协议、终止、Settlement 和 Closure 共用 Contract 行锁；
- 更正成功推进 current revision，其他仍基于旧 revision 的写入明确冲突；
- Settlement 或 Closure 若先提交但不推进 revision，更正还必须核对 Expected Status；允许的同状态顺序由 Contract 行锁确定，既有 Settlement／Closure 继续引用其原 source revision；
- Instrument、Revision、Party Snapshot、command、current revision 投影和审计在一个 PostgreSQL 事务中提交；
- 审计失败时不得留下部分更正事实；所有新增事实继续受不可更新、不可删除保护。

## 权限与页面

- 只有 `administrator` 拥有 `business.correct` 权限；
- `editor` 和 `viewer` 可以读取更正文书、Revision 历史和审计，但不能提交更正；
- 直接 POST 必须在服务端返回 `403`，不能只隐藏页面；
- active、terminated、closed 详情对管理员显示“受控更正当前 Revision”表单；
- 表单默认填入当前 Revision 的五个可更正字段，并显示固定不变的编号、币种和主体边界；
- 成功后返回详情，明确显示新 Revision 和“生命周期状态未改变”；
- 更正文书进入统一 Instrument 时间线，也自动成为 Evidence 可引用的 Instrument 目标。

## 明确不做

- 直接 UPDATE／DELETE 已确认 Contract、Instrument、Revision 或快照；
- 更正非当前 Revision、插入过去顺序或重新排列历史 Instrument；
- 修改编号命名空间、合同编号、币种或 Party Snapshot；
- 撤销终止、重新打开 closed、撤销 Settlement／Closure／Relation；
- 自动重算既有 Settlement、关系、关闭或 Money Flow 事实；
- 任意 JSON Patch、管理员 SQL、万能 `type + payload` 纠错接口；
- 审批流程、电子签章或法律效力判断。

## 验收标准

1. active、terminated、closed 均可由 administrator 追加更正文书并形成 Revision N+1，原状态保持不变。
2. draft、cancelled 拒绝更正；editor、viewer 和匿名请求不能写入。
3. 标题、分类、金额、签署日期和生效日期按完整快照更正，编号、币种和 Party Snapshot 原样复制。
4. 至少一个字段必须变化；非法文本、日期、金额、Effective Order 或更正文书日期明确拒绝。
5. Expected Status 或 Expected Revision 过期时拒绝，不静默覆盖并发补充、终止或关闭。
6. 相同命令安全重放；改变任一意图字段产生幂等冲突。
7. Correction Instrument、Revision、Party Snapshot、command、current revision 和审计原子提交；故意制造审计失败时整体回滚。
8. 新旧 Instrument、Revision、command 和审计均不可更新或删除。
9. 更正不修改既有 Settlement、Relation、Closure 或 Evidence，它们仍指向原始 source revision／目标。
10. 更正后的标题、分类、金额和日期在详情、列表与检索中读取当前 Revision，不从 Contract 草稿根表形成第二真相源。
11. 更正文书可被 Evidence 目标校验和页面选中，文件上传不会再次推进 Revision。
12. 从空数据库经 HTTP 完成激活、补充、结算、终止、关闭、Evidence 和 closed 后更正，页面、数据库、历史引用、权限及可信 Actor 审计一致。
13. Money Flow 数据库和应用停止时仍可完成全部验收，不产生跨系统业务动作。
