# Contract Ledger：空业务库单合同账簿恢复

里程碑：M2  
功能编号：CL-M2-010  
日期：2026-09-19  
状态：已实现并验证

## 目标

管理员能够把 `CL-M2-009` 生成的单合同可验证账簿归档恢复到一个已经完成 migration 和本地管理员 bootstrap、但没有任何 Contract／Party 业务数据的 Contract Ledger。恢复前必须完整验证容器、格式、摘要、账簿结构和引用；恢复成功后合同当前投影、不可变正式历史、结算、关闭、Evidence 校验元数据与源审计可查询，整个写入要么全部提交，要么不留下任何业务事实。

本功能解决的是“空业务库中恢复一份自包含合同账簿”的最小闭环，不把不完整输入伪装成全库灾难恢复。它不覆盖现有合同、不合并客户主数据，也不从单合同快照猜测关系另一端的完整历史。

## 输入与目标边界

- 输入只能是格式标识 `omsbizfoundry.contract-ledger/contract-export-v1`、格式版本 1 的 ZIP；
- ZIP 必须且只能包含根目录下 `manifest.json` 和 `ledger.json` 两个普通文件；
- 默认压缩包和解压后单文件都有明确大小上限，拒绝重复路径、目录、软链接、加密项、路径穿越和解压膨胀；
- 目标数据库允许已有 schema migration、本地用户、会话和本地用户审计；
- 目标数据库不得已有任何 Contract 或 Party。发现业务数据时在写入前返回冲突，不能自动合并或覆盖；
- 一次只恢复一个 Contract ID；不接受列表、数据库 dump、CSV、Excel 或任意 JSON。

## 预检顺序

恢复服务在开启写事务前完成以下验证：

1. ZIP 容器安全、文件数、文件名、大小和 JSON 编码；
2. manifest 格式标识、版本、生成时间、Contract ID、ledger 文件名和 Evidence 内容声明；
3. `ledger.json` 精确字节数和 SHA-256 与 manifest 一致；
4. ledger 格式标识和 Contract ID 与 manifest 一致；
5. UUID、时间、日期、文本长度、ISO 风格币种和精确十进制金额有效；
6. 生命周期状态、current revision、Revision／Instrument 连续顺序及 current projection 一致；
7. Revision Party Snapshot、Settlement 确认主体、Closure、Evidence 目标和审计引用完整；
8. ID、顺序、编号和命令引用在归档内不重复或冲突；
9. 归档不包含无法由单合同恢复的外部 Relation；
10. 能找到恢复数据库必要的创建、激活、变更、终止、更正、结算、关闭和 Evidence 命令来源审计。

任何失败都返回明确问题且不开始写入。数据库约束仍作为第二道防线；预检通过不代表可以绕开事务、唯一约束或不可变触发器。

## 单合同自包含规则

Contract Relation 的两端都是独立合同，并由数据库外键引用双方 Revision。单合同归档只带另一端登记时的编号／标题快照，不带其完整 Contract、Instrument、Revision 和 Party Snapshot，因此第一版恢复遇到任意 Relation 都整体拒绝，错误必须说明需要未来的“多合同依赖恢复包”。

不得创建伪合同、占位 Revision、放松外键或静默丢弃 Relation。没有 Relation 的 draft、cancelled、active、terminated、closed 合同均可恢复。

## 恢复事实

成功事务至少恢复：

- Contract 稳定 ID、业务编号、当前投影、生命周期状态、draft version、current revision 和原时间戳；
- 归档内涉及的 Party 当前资料和全部草稿 Party Link，包括 removed_at；
- cancelled 合同的取消命令、原因、Actor 和时间；
- 激活命令、全部 Contract Instrument、全部 Revision 和每版 Party Snapshot；
- 可从归档确定的补充、终止和更正命令，保持原 Command ID 和幂等语义；
- 全部 Settlement、确认主体与结算命令；
- 可选 Closure 与关闭命令；
- Evidence Document、Version 校验元数据和 Evidence command；
- 归档中的全部 Contract 通用审计；
- 一条本地 `contract_ledger_restored` 审计和恢复操作记录。

草稿阶段已经完成但未被导出为完整请求负载的历史编辑命令不凭审计摘要反向猜测；源审计仍完整保留。恢复后的新写入继续使用当前应用规则和新的本地可信 Actor。

## Evidence 内容缺失

单合同归档明确不包含 Evidence 二进制内容。恢复时：

- Document、Version、原始文件名、媒体类型、大小和 SHA-256 仍恢复；
- Version 标记为 `missing`，`storage_provider` 与 `storage_key` 为空；
- 页面明确显示“文件内容待恢复”，不提供可误导的下载链接；
- 直接请求缺失版本返回不可用，不写下载成功审计；
- 后续追加新 Evidence Version 仍可工作，但不会自动补齐旧 Version；
- 再次导出时必须带出 `content_available: false`，不能声称文件内容存在。

这是可验证的元数据恢复，不是 Evidence 文件备份。按原 Version SHA-256 重新挂接文件内容是后续独立功能。

## 幂等、冲突与原子性

- 表单提交携带 Restore Command ID；相同命令、相同 ledger SHA-256 和相同 Actor 重放返回原结果；
- 同一 ledger SHA-256 使用新 Command ID 再提交也返回既有恢复结果，不新增业务事实或审计；
- 相同 Command ID 改变归档或 Actor 返回幂等冲突；
- Contract ID、合同业务编号、Party ID、命令 ID、审计 ID或其他事实冲突时整体失败；
- repository 使用数据库级恢复互斥锁和单个 PostgreSQL 事务；
- Party、Contract、Instrument、Revision、Snapshot、Settlement、Closure、Evidence、源审计、恢复操作和恢复审计在同一事务提交；
- 任一步或恢复审计失败都完整回滚；
- 成功后原有不可变触发器继续保护恢复的历史事实。

恢复操作保存归档格式、ledger SHA-256、Contract ID、缺失 Evidence Version 数、执行 Actor 和时间，不保存上传 ZIP 或 Evidence 内容。

## 权限与 HTTP

- 新增 `business.restore` 权限，只授予 `administrator`；
- editor 和 viewer 看不到入口，直接 POST 返回 `403`；匿名请求进入登录流程；
- 管理入口为 `GET /admin/restore`，提交入口为 `POST /admin/restore`；
- 表单要求上传 ZIP、携带 CSRF／Command ID，并显式勾选“目标业务库为空且允许写入”；
- 成功后跳转到恢复的合同详情，显示恢复成功和缺失 Evidence 数量；
- 校验错误返回 `422`，目标非空或幂等意图冲突返回 `409`，请求／解压大小超限返回 `413`；
- 不在错误页面回显完整账簿、文件字节、数据库细节或堆栈。

## 独立系统边界

恢复只写 Contract Ledger 自己的 PostgreSQL，不连接 Money Flow、客户 OA、外部主体中心或对象存储，不生成义务、资金或核销。该功能没有 Contract → Money Flow 业务流动，因此不建设连接器；验收时确认 Money Flow 数据库和应用停止。

## 明确不做

- 覆盖、删除、清空或合并已有 Contract／Party 业务数据；
- 恢复含 Relation 的单合同归档，或创建伪造的关系端点；
- 多合同依赖图排序、批量导入或跨归档去重；
- 恢复 Evidence 二进制、自动从外部路径下载文件或忽略 SHA-256；
- 恢复本地用户、密码、会话、部署配置和数据库密钥；
- PostgreSQL 全库备份／时间点恢复、自动灾难恢复或跨版本 schema 迁移平台；
- 接受未知格式版本、宽松忽略未知顶层字段或“尽力而为”部分写入。

## 验收标准

1. administrator 能把没有 Relation 的 draft、cancelled、active、terminated、closed v1 归档恢复到空业务库。
2. ZIP 路径穿越、重复／多余／缺失文件、加密／非普通项、超限、非法 JSON、未知格式或未知顶层字段在写库前拒绝。
3. ledger 字节数或 SHA-256、manifest／ledger Contract ID 或格式不一致时返回 `422` 且数据库保持空业务状态。
4. Revision、Instrument、Party Snapshot、Settlement、Closure、Evidence 和审计的结构／引用不一致在预检拒绝。
5. 任意 Relation 导致整个单合同恢复拒绝，不创建占位合同或丢弃关系。
6. active／terminated／closed 恢复后当前详情、文书时间线、Revision 历史、金额、主体快照、结算和关闭与源归档一致。
7. cancelled 恢复取消原因、版本、Actor 和时间；draft／cancelled 不伪造正式 Revision。
8. Evidence 元数据和 SHA-256 恢复，但所有源 Version 明确标记内容缺失、页面无下载链接、直接下载返回不可用且无下载审计。
9. 归档源审计完整保留，并追加且只追加一条当前管理员 Actor 的 `contract_ledger_restored` 审计。
10. 相同命令或相同 ledger SHA-256 安全重放；改变命令意图、目标业务库非空和业务唯一冲突明确拒绝。
11. 故意制造中段约束或恢复审计失败时 Party、Contract 及全部子事实整体回滚。
12. 恢复前后对源归档和目标重新导出的业务事实做语义比较；允许新增恢复审计、Evidence 可用状态变化和新导出生成信息。
13. 新恢复合同继续接受符合其状态的正常本地写入，且恢复事实继续受不可变保护。
14. 从只有本地管理员的空业务库经 HTTP 上传归档、恢复、查看、重新导出并核验；Money Flow 停止且没有跨系统业务动作。
