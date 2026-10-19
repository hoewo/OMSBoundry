# Contract Ledger：单合同可验证账簿导出

里程碑：M2  
功能编号：CL-M2-009  
日期：2026-09-19  
状态：已完成并验证

## 目标

管理员能够把一份合同在 Contract Ledger 中已经形成的当前投影、不可变正式历史、业务引用、Evidence 元数据和审计时间线导出为一个可离线核验的版本化 ZIP 归档。导出物用于客户交接、审计取证和后续恢复能力的稳定输入，但本功能本身不是数据库备份、恢复工具或通用报表平台。

导出必须保持账簿语义：更正之后仍同时包含旧 Revision、更正文书、新 Revision，以及继续引用旧 Revision 的 Settlement、Relation、Closure 或 Evidence；不能只导出当前详情页看到的最新字段。

## 导出单元与状态

- 一次请求只导出一个稳定 Contract ID 对应的账簿；
- `draft`、`cancelled`、`active`、`terminated` 和 `closed` 都可以导出；
- draft／cancelled 导出当前草稿投影、全部签约主体关联记录和审计，但不会伪造正式 Revision；
- active／terminated／closed 导出当前正式投影、全部 Revision 和每个 Revision 的完整 Party Snapshot；
- 导出不会修改合同状态、draft version、current revision 或任何业务事实，只追加一次成功访问审计。

## 归档格式

第一版格式标识为 `omsbizfoundry.contract-ledger/contract-export-v1`。ZIP 根目录只包含两个普通文件，不包含子目录、软链接或重复路径：

```text
contract-<contract-id>-ledger.zip
├─ manifest.json
└─ ledger.json
```

`manifest.json` 至少包含：

- 格式标识和格式版本；
- 生成时间；
- Contract ID；
- `ledger.json` 的精确字节数；
- `ledger.json` 精确字节的 SHA-256 十六进制摘要；
- Evidence 二进制内容未包含的明确声明；
- 明确排除的敏感或内部字段说明。

校验方对 ZIP 中未经改写的 `ledger.json` 原始字节计算 SHA-256，即可与 manifest 比对。第一版不依赖 JSON 规范化算法，也不在归档中放置自引用的 ZIP 摘要。

`ledger.json` 使用 UTF-8 JSON、固定字段名和确定性数组排序；没有记录的集合写成空数组而不是 `null`。同一事实在导出期间不会因并发写入而来自不同数据库时点。

## 账簿内容

`ledger.json` 至少包含以下范围：

1. 合同稳定身份、生命周期状态、draft version、current revision、当前投影和时间戳；cancelled 合同同时包含取消原因、版本迁移、可信 Actor 和取消时间；
2. 全部草稿签约主体关联，包括已经移除的关联及其加入／移除时间；
3. 全部不可变 Revision，及每个 Revision 的完整 Party Snapshot；
4. 全部 Contract Instrument，包括原合同、补充协议、终止文书与更正文书；
5. 以本合同作为 source 或 target 的全部 Relation，以及关系形成时保存的双方 Revision 引用和快照；
6. 全部 Settlement 和确认主体快照；
7. 可选的 Closure 事实；
8. 全部 Evidence Document 和 Version 元数据，包括目标引用、原文件名、媒体类型、大小、SHA-256 和记录 Actor；
9. 截至一致性快照时点的完整通用审计时间线，包括 action、command ID、可信 Actor、摘要和结构化 details。

数组必须按稳定业务顺序输出：Revision 按 revision，Instrument 按 effective order，Relation／Settlement／Evidence／Audit 依照各自形成时间并用稳定 ID 处理同一时间；嵌套 Party Snapshot 和 Evidence Version 也必须使用明确顺序。

## 排除内容

第一版明确不包含：

- 本地用户密码摘要、会话令牌、登录失败状态或其他身份凭证；
- Evidence 文件二进制内容；
- Evidence 内部 `storage_key`、文件系统路径或对象存储私有定位信息；
- 数据库连接信息、部署密钥、服务器路径或运行日志；
- 仅用于命令去重的内部 payload hash 和没有独立业务含义的存储细节；
- Money Flow 或其他系统的数据。

Evidence 的 SHA-256 和大小随版本元数据导出，使接收方可以在另行取得文件内容后核验；缺少二进制内容不能被误解为完整文件备份。

## 一致性、完整性与审计

- repository 必须在一个 PostgreSQL `REPEATABLE READ, READ ONLY` 事务中读取该合同的全部导出事实；
- 导出期间的并发写入可以在下一次导出中出现，但不能产生半新半旧的单次归档；
- 服务先完成 `ledger.json`、摘要、manifest 和 ZIP，再写入 `contract_ledger_exported` 通用审计；
- 导出审计至少记录格式、ledger SHA-256、ledger 字节数、归档字节数、快照状态和 current revision；
- 审计写入失败时请求整体失败，不能向客户端发送未审计的归档；
- 当前这次导出审计不在它自己的快照中。下一次导出会包含之前的导出审计，因此即使业务事实未变，两个归档摘要也允许不同；
- 导出内容完全在内存中准备并审计成功后才写 HTTP 响应，避免失败后出现半个下载文件。

SHA-256 用于发现传输或存储中的非预期变化，不等同于数字签名、可信时间戳或法律真实性证明。

## 权限与 HTTP

- 新增 `business.export` 权限，并只授予 `administrator`；
- `editor` 和 `viewer` 即使能查看合同详情，也不能下载完整可移交账簿；
- 匿名浏览器请求进入登录流程，已登录但无权请求返回 `403`；
- 下载入口为 `GET /contracts/{contract-id}/export`；
- 成功响应使用 `application/zip`、安全的 attachment 文件名、`Cache-Control: no-store` 和 `X-Content-Type-Options: nosniff`；
- 合同详情页只对 administrator 显示导出操作，但权限控制必须由服务端再次执行；
- 文件名只使用系统生成的稳定 Contract ID，不把未经约束的合同编号直接拼入响应头。

## 独立系统边界

导出只读取 Contract Ledger 自己拥有的数据库和 Evidence 元数据，不调用 Money Flow、客户 OA、对象存储公网接口或外部身份平台。它不是 Contract → Money Flow 业务连接器，不会创建义务、现金或核销事实，因此本功能不需要跨系统联调；验收时仍需确认 Money Flow 完全停止后功能可用。

## 明确不做

- 导入、恢复、回滚或用导出物覆盖现有数据库；
- 全系统／全租户备份或批量合同导出；
- CSV、Excel、PDF 报表或可配置字段选择；
- Evidence 二进制打包、外部对象存储拉取或公开分享链接；
- 数字签名、可信时间戳、加密归档或密钥托管；
- Money Flow 数据拼接、跨系统一致性快照或外部引用在线解析；
- 通用报表引擎、导出任务队列或后台调度平台。

## 验收标准

1. administrator 可以导出 draft、cancelled、active、terminated 和 closed 合同；不存在的 Contract ID 返回 `404`。
2. editor、viewer 的直接下载请求返回 `403`，匿名浏览器请求不能获得归档。
3. ZIP 只含根目录下 `manifest.json` 和 `ledger.json`，二者都能由标准工具读取，且响应头禁止缓存与类型嗅探。
4. manifest 的格式标识、Contract ID、ledger 字节数和 SHA-256 与归档中的原始 `ledger.json` 精确一致。
5. active／terminated／closed 导出包含全部 Instrument、Revision 和每个 Revision 的 Party Snapshot；空集合编码为 `[]`。
6. 更正场景同时保留旧 Revision、更正文书、新 Revision，以及仍引用旧 Revision 的 Settlement、Relation、Closure 和 Evidence。
7. Relation 同时覆盖本合同作为 source 与 target 的记录；Settlement 包含确认主体快照；Closure 只在存在时输出。
8. Evidence 导出全部 Document／Version 校验元数据，但不包含文件字节、`storage_key` 或服务器路径。
9. 本地身份凭证、会话、数据库密钥、内部存储位置和仅用于幂等的 payload hash 不出现在归档中。
10. 并发写入期间的单次导出来自一个 `REPEATABLE READ, READ ONLY` 快照，不产生跨时点拼接结果。
11. 成功下载追加一条可信 Actor 的 `contract_ledger_exported` 审计；故意制造审计失败时 HTTP 返回失败且不发送归档。
12. 导出前后合同状态、draft version、current revision 和业务事实不变；当前导出审计不在本次 ledger 中，后续导出可以包含它。
13. 从空数据库经 HTTP 建立包含激活、补充、关系、结算、终止、关闭、Evidence 和受控更正的合同后，下载归档并离线验证结构、摘要、历史引用、权限和审计均一致。
14. Money Flow 数据库和应用停止时仍可完成全部验收，不产生跨系统业务动作。
