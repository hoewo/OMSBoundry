# Contract Ledger：合同文件证据

里程碑：M2  
功能编号：CL-M2-007  
日期：2026-09-19  
状态：已完成并验证

## 目标

用户能够为 Contract 整体或一条已经存在的 Contract Instrument、Settlement、Closure 上传文件，形成可校验、可版本化、不可静默覆盖的 Evidence Reference，并在合同详情中查看历史和下载原始内容。Contract Ledger 自带本地文件存储适配器，不要求对象存储、客户文档系统或 Money Flow Ledger 在线。

## 事实边界

Evidence 证明“某个字节序列被作为某项合同事实的佐证保存”，不自动证明：

- 文件已经合法签署、盖章或通过电子签名验证；
- 文件正文与结构化合同字段一致；
- 文件没有恶意代码；
- 对应业务事实已经生效、结算或清偿。

结构化 Contract、Instrument、Settlement 和 Closure 仍是各自业务事实。Evidence 只引用这些事实，不能通过上传文件改变状态、revision、金额或结算。

## Evidence Document 与 Version

Evidence Document 是一个稳定的逻辑文件槽，保存：

- 独立 Document ID；
- 所属 Contract ID；
- 目标类型和目标 ID；
- 必填标题，最多 200 个字符；
- 必填用途／依据说明，最多 500 个字符；
- 创建 Actor 和时间。

每个 Document 拥有从 1 开始连续增加的 Evidence Version。每个 Version 保存：

- 独立 Version ID 和从 1 开始的版本号；
- 原始文件名，只用于显示和下载名称；
- 服务端探测的媒体类型；
- 实际字节数；
- 完整内容的 SHA-256；
- 存储提供者和不含用户文件名的内部 storage key；
- 上传 Actor 和时间。

Document、Version、上传命令和审计都不可更新、不可删除。内容变化必须追加下一 Version，旧内容继续可读。

## 可引用目标

第一版固定四种目标：

- `contract`：Contract 整体；草稿原始材料或不属于某条正式事实的通用文件使用此目标；target ID 必须等于 Contract ID；
- `instrument`：本 Contract 的原合同、补充协议或终止文书；
- `settlement`：本 Contract 的阶段或最终 Settlement；
- `closure`：本 Contract 的操作性 Closure。

repository 和数据库触发器都必须验证目标真实存在且属于同一 Contract。第一版不接受任意 `type + id`、外部 URL 或跨系统对象，避免形成无法授权和无法校验的万能引用。

## 状态与版本规则

- draft、active、terminated、closed、cancelled Contract 都可以追加 Evidence；
- cancelled／closed 后追加 Evidence 只是补充佐证，不重新打开合同，也不改变任何业务状态；
- 创建 Document 时原子形成 Version 1；
- 新版本命令携带 Expected Latest Version，repository 锁定 Document 后连续形成下一版本；
- 两个命令并发消费同一 Expected Latest Version 时只允许一个成功；
- Evidence 版本号独立于 draft version、Contract revision 和 settlement sequence。

## 文件接收与存储安全

- 单文件默认上限 20 MiB，可由部署配置收紧或放宽为一个正整数；
- 空文件拒绝；原始文件名去除路径语义、控制字符并限制为 255 个字符；
- storage key 由服务端生成，路径永远不使用用户文件名；
- 本地适配器只在配置的根目录内创建临时文件和对象文件，使用限制权限并以原子 rename 完成写入；
- 上传时按实际内容计算字节数、SHA-256 和媒体类型，不信任请求中的大小或 Content-Type；
- 下载前在同一打开文件描述符上重新核对字节数与 SHA-256，校验失败则拒绝交付；
- 下载统一使用 attachment、`application/octet-stream`、`nosniff` 和 `no-store`，不在本功能中内联预览或执行内容；
- 数据库事务失败时删除本次新写对象；进程在文件落盘与数据库提交之间崩溃可能产生不可见孤儿对象，但不会产生指向缺失文件的已提交 Reference，后续交付阶段补充按年龄隔离的安全巡检；
- 数据库与证据目录必须作为同一版本的备份／恢复集合，具体演练在 M6 完成。

## 幂等与原子性

- 每次上传携带 Command ID；相同命令和相同标准化元数据、目标、Expected Version、文件名、实际大小与 SHA-256 重放返回原 Version；
- 相同 Command ID 改变任一意图字段或文件内容返回幂等冲突；
- 新的候选文件在重放、冲突或数据库失败后由服务删除；
- Document／Version／command 与 `contract_evidence_created` 或 `contract_evidence_version_added` 审计在一个 PostgreSQL 事务内提交；
- 审计失败不得留下数据库 Evidence 事实；
- 已提交的 Reference 若底层文件缺失或校验失败，下载返回不可用，不伪装为不存在或成功。

## 下载访问与审计

- 所有已登录 administrator、editor、viewer 都可读取当前可见 Contract 的 Evidence；当前最小身份模型尚无更细组织范围；
- 每次通过应用成功取得下载权限时写入 `contract_evidence_downloaded` 审计，包含 Document、Version、版本号、文件名、大小和 SHA-256；
- 下载审计写入失败时拒绝下载，避免无审计读取；
- administrator/editor 可以上传并追加版本；viewer 不显示上传表单，直接 POST 返回 `403`；
- 匿名下载仍跳转登录，不能依靠不可猜测 URL 绕过授权。

## 页面

合同详情新增“文件证据”区域：

- 按 Document 创建时间显示标题、用途、引用目标和全部版本；
- 每个 Version 显示版本号、原始文件名、实际大小、媒体类型、SHA-256、Actor 和时间，并提供受保护下载；
- administrator/editor 可选择当前合同整体或页面上已有的 Instrument、Settlement、Closure 作为目标并上传 Version 1；
- 每个已有 Document 可追加新版本，表单包含 Expected Latest Version；
- 上传成功回到合同详情并显示明确成功消息；
- 文件校验、目标、版本或幂等错误使用明确 HTTP 状态，不把内部存储路径暴露给用户。

## 明确不做

- 文件在线预览、Office/PDF 转换或缩略图；
- 病毒扫描、内容安全扫描、OCR、全文索引或智能抽取；
- 电子签章、签名证书或时间戳验证；
- Evidence 删除、销毁、保留期限和法律留置策略；
- 外部 URL、跨系统 Evidence、公开分享链接或匿名下载；
- MinIO、S3、客户文档系统适配器；
- 用文件内容自动创建或修改 Contract、Instrument、Settlement、Closure 或 Money Flow 事实。

## 验收标准

1. 五种 Contract 状态均可创建 Evidence Document + Version 1，且不改变状态、draft version、current revision、金额或 settlement sequence。
2. instrument、settlement、closure 目标必须存在且属于路径中的 Contract；任意 UUID、错误 Contract 和未知类型明确拒绝。
3. 新版本按 Expected Latest Version 连续追加；过期版本和两个并发命令不能静默覆盖。
4. 相同 Command ID 和相同文件安全重放，改变目标、元数据、Expected Version、文件名或内容产生幂等冲突。
5. 空文件、超限文件、非法文件名和请求体超限明确拒绝，且不留下数据库事实或本地对象。
6. 原始文件名不参与存储路径；路径穿越文件名不能逃离配置根目录。
7. 下载返回原始字节，强制 attachment／octet-stream／nosniff／no-store；内容缺失、大小或 SHA-256 不一致时拒绝。
8. 创建、追加版本与审计原子提交；故意制造审计失败时数据库事实与本次文件都回滚／清理。
9. Document、Version、command 和审计更新／删除均被 PostgreSQL 不可变保护拒绝。
10. administrator/editor 可上传，viewer 可查看和下载但不能上传，匿名不能下载；下载形成可信 Actor 审计。
11. 从空数据库经 HTTP 完成合同创建、Evidence 上传、Version 2、两个版本下载和 viewer 权限验证，页面、数据库、文件内容与审计一致。
12. Money Flow 数据库和应用停止时仍可完成全部验收，不产生跨系统业务流动。
