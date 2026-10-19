# M10-002 Evidence Service 最小纵向闭环

日期：2026-09-21  
状态：工程内实现与验证、D1 装配和真实 OSS 核心浏览器链已完成；同 File 不可变多版本已完成本地 PostgreSQL 18 集成，D1 提升待验；Contract 迁移、云端剩余负例和公网入口独立推进

## A→B

**文件字节、上传状态和访问凭据散落在业务应用中会重复、失控且难以追溯 → 一个可独立部署的 Evidence Service 统一确认文件版本并只向业务应用返回稳定的 File ID／Version。**

## 边界

Evidence Service 拥有：

- File ID、不可变 Version、实际大小、服务端识别媒体类型和 SHA-256；
- Upload Session、服务端生成的 Object Key、Provider 状态和清理状态；
- 本地文件或 OSS 对象的技术访问、访问授权和不可变技术审计；
- 自己的 PostgreSQL 数据库、migration、备份恢复和 Provider 配置。

Evidence Service 不拥有：

- Project、合同、台账、报销、票据等业务对象和业务状态；
- 文件在业务中的名称、分类、审批意义或删除规则；
- IAM 的账号、密码、角色和授权事实；
- 业务应用数据库中的 File Reference。

`client_context` 只保存可选、不透明的关联诊断信息，不能成为资源授权或业务事实来源。新 File 上传会话生成稳定 File ID 和 Version 1；同一 File 的后续内容使用 `POST /v1/files/{file_id}/versions/upload-sessions`，由服务端在锁内分配单调、不复用的下一 Version。调用方不能指定版本号，已确认 Version 不能覆盖。

## IAM 契约

Audience 固定为 `evidence-service`：

| Capability | 含义 |
| --- | --- |
| `evidence-service.view` | 读取已确认版本元数据并申请短期下载／预览授权 |
| `evidence-service.upload` | 创建、完成、取消上传会话，以及为同一 File 追加不可变版本 |

正式 IAM 和本地 Template Provider 验证同一 Principal 契约。所有控制面请求必须认证；Template 只替换凭证来源，不能关闭鉴权。`upload` 使用 Tenant Scope；metadata／download 可以使用 Tenant Scope 或精确 `file:{id}` Scope。服务端始终用 Principal 的 Tenant ID 隔离查询并复验 File Scope，客户端提交的租户、Subject、角色或按钮名无效。

本地数据面的 PUT／GET 使用 Evidence 在控制面鉴权后签发的短期 HMAC capability URL。Token 绑定动作、Tenant、Session 或 File ID、Version、Object Key 和到期时间；它不是 IAM 替代品，也不能被换到另一对象。

## 数据模型

### `files`

- `(tenant_id, file_id)` 为稳定身份；
- 不保存可变“当前版本”事实；稳定引用从已完成 Version 中确定最大版本；
- 不保存 Project ID 或业务分类。

### `file_versions`

- `(tenant_id, file_id, version)` 唯一；
- 每个已确认版本的 `status` 为 `available`；版本号为正整数并按 File 单调递增，允许因失败／取消会话形成空洞但禁止复用；
- 保存服务端确认的 `file_name`、`size_bytes`、`media_type`、`sha256`、Provider、Bucket／Object Key、创建 Actor 和时间；
- 成功后禁止 update／delete。

### `upload_sessions`

- 保存预期元数据、Provider、Object Key、状态、过期时间和创建 Actor；
- 状态为 `pending → completing → completed`，或 `pending／completing → aborted／expired／cleanup_pending → cleaned`；
- `completed` 必须引用唯一不可变 Version；
- 客户端观察的大小、摘要、ETag 和 parts 只作审计线索，不是完成依据。

### `idempotency_commands`

- 唯一键为 `(tenant_id, subject_id, action, idempotency_key)`；
- 同键同规范化负载返回同一结果；同键不同负载返回冲突；
- File Create、Version Create、Complete 和 Abort 各自独立命名空间。

### `audit_entries`

- Create、Complete、Abort、Access Grant、清理成功／失败均追加技术审计；
- 记录可信 Actor、Tenant、目标、结果、Provider 和时间；
- 禁止 update／delete，不保存长期密钥或 STS Secret。

## Storage Provider 端口

服务内核只依赖以下技术能力：

1. 为服务端生成的单一 Object Key 创建短期上传授权；
2. 打开对象流并返回 Provider 元数据；
3. 为已确认对象创建短期 GET URL；
4. 删除未被可用 Version 引用的对象；
5. 报告 Provider 就绪状态。

### Local Provider

- 根目录必须为专用绝对目录，Object Key 经过安全 join 后仍位于根目录内；
- 上传先写同目录临时文件，校验 Content-Length 和最大大小后以不可覆盖的原子发布生成正式对象；
- 短期 URL 绑定单一上传会话或 File Version；
- 下载支持 `Range`、正确 Content-Type 和 Content-Disposition；
- 本地上传字节经过 Evidence 的数据面端点，但不经过业务 API。完整组合可由 Nginx 直接路由该数据面；该事实不冒充 OSS 直传。

### Aliyun OSS Provider

- 服务端凭据使用阿里云默认凭据链或受限配置，不进入浏览器；
- `AssumeRole.Policy` 只允许 `acs:oss:*:*:{bucket}/{object_key}`；
- multipart RAM Action 只允许 `oss:PutObject`、`oss:AbortMultipartUpload`、`oss:ListParts`；
- STS 有效期遵守阿里云当前 900～3600 秒范围，上传会话不得晚于 STS 过期时间；
- 服务端使用 OSS Go SDK v2 读取、复核、签名 GET 和清理；
- Bucket 必须私有，CORS 只允许部署声明的 Origin、必要方法和 Header。

## 完成确认

`complete` 在产生可引用 Version 前必须：

1. 锁定租户内 Upload Session 并校验状态、到期和幂等键；
2. 从 Provider 重新打开对象，不信任客户端大小、摘要或媒体类型；
3. 流式计算 SHA-256 和实际大小，并从文件前 512 字节识别媒体类型；
4. 与 Create 的预期大小、可选预期摘要以及 Complete 的客户端观察值比较；
5. 在一个数据库事务中写入不可变 Version、完成 Session、幂等结果和审计；
6. 数据库提交失败时不返回成功；对象由后续清理扫描回收。

同一 Complete 命令重放返回同一 Version。不同命令对同一已完成 Session 也只能读取既有结果，不能形成第二个 Version。

同一 File 同时最多存在一个 `pending／completing／cleanup_pending` 的追加版本会话。稳定引用 `/v1/references/files/{file_id}` 只返回最新已完成 Version；显式引用 `/v1/references/files/{file_id}/versions/{version}` 始终返回指定不可变事实。业务应用据此分别完成 drift 检测和历史重现，完整规则见 [ADR-0036](../../adr/0036-evidence-files-use-stable-identity-and-immutable-monotonic-versions.md)。

## 清理

独立 cleanup 命令扫描已过期、已取消、完成失败或长时间停留在 `completing` 的 Session：

- 先确认 Object Key 没有被 `available` Version 引用；
- 每个 Session 通过 PostgreSQL advisory lock 取得跨进程清理权，未取得锁的 Worker 跳过，避免多实例重复处理同一对象；
- Provider 删除成功后标记 `cleaned` 并审计；
- Provider 失败时标记／保留 `cleanup_pending`、累计尝试次数和最近错误，下一轮重试；
- 清理绝不通过列举整个 Bucket 推断对象归属，只处理数据库已知 Object Key。

## 迁移、备份与发布边界

- Evidence 数据库与本地对象根组成一致性备份；OSS 模式备份数据库与配置清单，对象耐久性由 Bucket 策略负责，恢复后逐条校验存在性和摘要；
- Contract Ledger 迁移是单独切片：保留原 File ID、Version、字节、大小、SHA-256、业务引用和历史审计映射；未通过迁移验收前不得删除原目录；
- 独立 release 包含 server、ops、migration、Capability Manifest、配置样例和本地 Provider；Contract release 可以装配它，但不得复制内核实现。

## 首个实现切片的验收

- OpenAPI 与 SDK 同时覆盖 `aliyun-oss`、`local-http` 判别授权；
- 独立空库 migration、verify 启动、健康检查和 Template／IAM 鉴权通过；
- 本地 Provider 通过真实 HTTP 的 Create → PUT → Complete → Access Grant → GET 闭环；
- 同一 File 的 Version 1 → 追加 Version 2 → 稳定引用前进，同时显式 Version 1 摘要与字节不变；
- 对象缺失、大小不符、摘要不符、过期 Token、越 Tenant、越 Session／Object Key、幂等负载冲突均失败关闭；
- 完成后数据库 Version 的大小、媒体类型和摘要来自服务端复核；
- Abort、过期和数据库完成失败留下的对象可重试清理，已确认 Version 不被删除；
- Provider 端口契约测试覆盖 OSS 单对象 Policy、精确 Action 和签名访问，不要求自动测试连接真实 Bucket；
- 真实 OSS Bucket、CORS 和 150 MiB 浏览器 multipart 作为需要云账号与资源的外部验收，未执行时明确标记，不伪装为通过；该项已于 2026-09-22 通过，范围与剩余负例见 `docs/testing.md`。

## 实现证据

- `backend/services/evidence-service` 可脱离其他业务应用完成 migration、Template／IAM 鉴权、健康检查和控制面服务；
- 本地 Provider 已通过真实 HTTP Create → PUT → Complete → Access Grant → Range GET，确认大小、媒体类型和 SHA-256 均由服务端重读对象得到；
- PostgreSQL 集成与 race 测试覆盖租户隔离、Session／Object 绑定、幂等资源绑定、并发取消、过期、完成提交失败、重试清理和不可变事实；2026-09-29 又在受限单 PostgreSQL 18.6 容器中通过同 File Version 2 分配、完成、稳定／显式引用和精确访问授权；
- 阿里云 Provider 契约测试固定单 Object Key Policy、`oss:PutObject`／`oss:AbortMultipartUpload`／`oss:ListParts`、临时凭据和 HTTPS；
- 本地完整冷备份和空库／新目录恢复已用 PostgreSQL 18 及真实对象字节验证；独立 Docker Compose、三个带 build identity 的二进制与 release bundle 已构建；
- 2026-09-22 已另行使用真实私有 OSS Bucket、精确 CORS 和浏览器通过 150 MiB multipart 核心链，并用独立负例脚本证明同一临时 STS 写相邻错误 Object Key 返回 `403 AccessDenied`；不以该证据冒充尚未执行的网络中断、凭证过期、公网入口或 Contract 迁移。Complete Suite D1 装配由 [M10-003](./m10-003-d1-integration.md) 单独完成并验证。
