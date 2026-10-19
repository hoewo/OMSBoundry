# Money Flow Ledger：本方主体可校验台账导出

里程碑：M4  
功能编号：MF-M4-012  
日期：2026-09-20  
状态：已实现并验证

## 目标

已登录的 Money Flow 本地管理员能够选择一个本方账簿主体，把该主体在本系统中已经形成的业务事实导出为一个可离线检查的版本化 ZIP。归档以固定 CSV 表保存精确原始事实，以 manifest 保存每张表的行数、字节数和 SHA-256，用于客户数据移交、外部分析前的数据准备和人工核验。

本功能是业务数据导出，不是 PostgreSQL 备份、恢复工具、批量导入、会计报表或跨系统拼表。导出不会把 current 汇总另存成第二套事实；接收方应从导出的义务、资金、核销及反向事实重新派生余额。

## 导出单元

一次请求只导出一个明确的 `ledger owner Party ID`：

- Party ID 是边界身份，同名 Party 不合并；
- 导出包含该 Party 作为 `ledger_owner_party_id` 的 Obligation、Cash Transaction 和 Internal Transfer，以及这些根事实的全部历史与关联；
- Allocation 只能同时连接该本方主体下的 Cash Transaction 与 Obligation；若存量数据破坏此边界，整次导出失败，不能产生半闭合归档；
- `parties.csv` 只包含本方主体和本次事实实际引用的往来 Party 当前资料，不包含无关 Party；事实自身仍保留记录时冻结的 Party Snapshot；
- 被选 Party 存在但尚无本方账簿事实时，仍可导出一个只有主体、表头和审计的有效空台账；
- 不导出其他本方主体拥有的业务事实，即使它们引用了同一个往来 Party。

## 归档契约

当前格式标识为：

```text
omsbizfoundry.money-flow-ledger/owner-ledger-export-v2
```

文件名使用系统生成的 Party ID：

```text
money-flow-ledger-<ledger-owner-party-id>.zip
```

ZIP 根目录固定包含 `manifest.json` 和以下十三张 CSV，不含目录、重复路径、软链接或额外文件：

```text
manifest.json
parties.csv
obligations.csv
obligation_revisions.csv
obligation_amount_adjustments.csv
obligation_remissions.csv
obligation_closures.csv
obligation_cash_expectations.csv
cash_transactions.csv
cash_transaction_reversals.csv
allocations.csv
allocation_reversals.csv
internal_transfers.csv
audit_entries.csv
```

每张 CSV：

- 使用 UTF-8、RFC 4180、CRLF、固定英文列名和首行表头；
- 所有金额以数据库精确十进制文本输出，不经过二进制浮点数；
- 时间统一输出 UTC RFC3339Nano，业务日期输出 `YYYY-MM-DD`；
- 可空字段以空单元格表示；空表仍输出表头，记录数为 0；
- 行使用明确的业务时间／序号加稳定 ID 排序，不依赖数据库默认顺序；
- 用户文本保留原值，包括逗号、双引号、换行以及可能以 `= + - @` 开头的内容。

CSV 是精确数据交换文件。为避免电子表格把原始文本误当作公式，页面和 manifest 必须提醒接收方使用“按文本导入”，不要直接双击执行；系统不能通过添加前缀静默改变业务原文。

已交付的 `owner-ledger-export-v1` 固定为十二张 CSV，不包含 Cash Expectation。新增预计日期事实通过 v2 显式发布；实现不继续生成一个内容被静默改变的 v1。

## manifest

`manifest.json` 至少包含：

- 格式标识与版本；
- 生成时间；
- 本方账簿主体的 ID、类型、当前名称和 revision；
- 来源 Money Flow schema version；
- CSV 编码、空值和电子表格公式处理声明；
- 每张固定 CSV 的文件名、媒体类型、记录数、精确字节数和 SHA-256；
- 明确排除的身份凭证、会话、幂等 payload hash、其他本方主体数据、数据库配置与尚未实现事实。

SHA-256 直接覆盖 ZIP 内对应 CSV 的未经改写原始字节。manifest 不保存自引用 ZIP 摘要，也不声称摘要是数字签名、可信时间戳或法律真实性证明。

## 导出事实

### 主体与义务

- `parties.csv`：本方主体和实际引用的往来 Party 当前资料；
- `obligations.csv`：本方主体下全部 draft／open／closed Obligation 根事实和 current 指针；
- `obligation_revisions.csv`：正式义务全部 revision，以及 creditor／debtor 冻结快照、effective amount、来源和记录 Actor；
- `obligation_amount_adjustments.csv`：全部金额调整事实；
- `obligation_remissions.csv`：全部减免事实；
- `obligation_closures.csv`：全部受控关闭事实。
- `obligation_cash_expectations.csv`：全部预计日期 `set`／`clear` 历史、sequence、来源 revision、原因、可信 Actor 与记录时间；预计金额仍从 current outstanding 派生，不进入文件。

### 资金、核销与内部移动

- `cash_transactions.csv`：全部实际收款／付款及 payer／payee 冻结快照；
- `cash_transaction_reversals.csv`：全部错误资金完整冲销；
- `allocations.csv`：全部资金—义务核销关系及形成时 Obligation revision；
- `allocation_reversals.csv`：全部错误 Allocation 完整反向事实；
- `internal_transfers.csv`：全部非经营性内部账户转账。

### 审计

`audit_entries.csv` 只包含本次导出范围内 Party、Obligation、Cash Expectation、Cash Transaction、Cash Reversal、Allocation、Allocation Reversal、Internal Transfer 和既往本方台账导出的审计。保留 command ID、可信 Actor、摘要与 `details_json`，但不导出身份登录审计或无关对象。

当前这次成功导出的审计在归档快照之后写入，因此不出现在它自身的 `audit_entries.csv`；下一次导出可以包含它。

## 一致性、审计与失败边界

- repository 在 Money Flow 自己的 PostgreSQL 中使用一个只读 `REPEATABLE READ` 事务读取 schema version、主体、全部事实和审计；
- 并发写入可以出现在下一次导出，但单次归档不能混合两个数据库时点；
- 服务先在内存中构造全部 CSV、摘要、manifest 和 ZIP，再追加 `money_flow_ledger_exported` 审计；
- 导出审计至少保存格式、本方主体、各表行数与 SHA-256、归档字节数和来源 schema version；
- 审计失败时请求整体失败，HTTP 不能发送归档；
- 读取失败、跨主体 Allocation、未知／重复表、列数错误或非法归档结构使整次请求返回 `500`，不能跳过损坏事实；
- 导出只增加访问审计，不改变 Party、Obligation、Cash、Allocation、Internal Transfer 或其状态和历史。

第一版同步在内存中构造单个本方主体归档。真实规模证明需要后台任务或流式处理前，不建设导出队列。

## 权限与 HTTP

- GET `/ledger-exports` 显示导出说明和 Party 选择，只允许 Money Flow 本地 administrator；
- POST `/ledger-exports` 消费有效 CSRF token 和 `ledger_owner_party_id` 后生成下载，避免 GET 预取或爬虫意外追加导出审计；
- 匿名请求进入本地登录，非 administrator 返回 `403`；
- Party ID 非法返回 `422`，不存在返回 `404`，读取、构造或审计失败返回 `500`；
- 成功响应使用 `application/zip`、安全的 attachment 文件名、`Cache-Control: no-store` 和 `X-Content-Type-Options: nosniff`；
- 页面明确说明归档边界、摘要局限和 CSV 按文本导入要求。

## 排除内容

- 本地用户名单、密码摘要、session token、锁定状态和登录审计；
- 所有幂等命令表及内部 `payload_hash`；
- PostgreSQL dump、数据库连接、服务配置、运行日志和部署密钥；
- 其他本方账簿主体拥有的业务事实；
- current 汇总、账龄和余额等可重建投影；
- 尚未实现的退款、坏账、Billing Document、银行余额与会计凭证；
- Contract Ledger、客户 OA 或其他系统的数据；
- 导入、恢复、合并、覆盖、定时导出、邮件发送、加密或数字签名。

## 验收标准

1. administrator 能从 Party 列表中选择一个存在的 Party 并下载该 Party 的本方台账；匿名不能取得文件，非 administrator 返回 `403`。
2. POST 必须通过 CSRF；非法 Party ID 返回 `422`，不存在返回 `404`，错误响应不包含部分 ZIP。
3. ZIP 只包含 manifest 和十三张固定根目录 CSV，文件名无用户输入、路径穿越、目录、软链接或重复项。
4. 每张 CSV 都有固定表头；空表只有表头且 manifest 行数为 0；CSV 可被标准解析器读取。
5. manifest 的格式、版本、本方主体、schema version、文件集合、行数、字节数和 SHA-256 与 ZIP 原始内容完全一致。
6. 金额是精确十进制文本，`0.1`、`0.2` 和六位小数不经过浮点数；日期和时刻使用约定格式。
7. 逗号、引号、换行和公式样式用户文本可无损 CSV round-trip；页面及 manifest 明确要求电子表格按文本导入。
8. 导出包含本方主体下 draft／open／closed Obligation、全部 revision、Adjustment、Remission、Closure、Cash Expectation 历史和冻结 Party Snapshot；不保存由 current outstanding 派生的预计金额。
9. 导出包含全部 Cash Transaction、Cash Reversal、Allocation、Allocation Reversal 和 Internal Transfer；已反向事实与原事实同时保留。
10. `parties.csv` 包含本方主体与实际引用的往来 Party，不包含无关 Party；其他本方主体拥有的事实不进入任何业务 CSV。
11. 审计 CSV 只覆盖导出事实、引用 Party 和既往同一主体导出审计，不泄露身份凭证、会话、登录审计或内部 payload hash。
12. 全部读取来自单个只读 `REPEATABLE READ` 快照；并发写入测试证明单次归档不会跨时点拼接。
13. 跨本方主体 Allocation 或 repository 返回未知／重复表、错误列数时整体失败，不生成看似完整的归档。
14. 成功生成后追加可信 Actor 的 `money_flow_ledger_exported` 审计；审计失败时不发送归档，本次审计不出现在本次 ZIP。
15. 导出前后所有业务事实数量、状态、revision 和金额不变；v2 与 Cash Expectation 一起运行在 `000015_obligation_cash_expectations.sql` 或更高兼容 schema。
16. Contract Ledger 停止时仍能完成页面、下载和离线摘要核验；当前没有跨系统业务流动，因此不制造连接器联调。

## 明确不做

- 把 CSV 当作可直接恢复或覆盖数据库的输入；
- 全数据库备份、Evidence 文件或身份配置备份；
- Excel 格式、可配置列、筛选条件、日期范围或汇总报表；
- 退款后净现金、会计收入、利润、税额或银行对账口径；
- Contract 与 Money Flow 的联合归档或分布式一致快照。
