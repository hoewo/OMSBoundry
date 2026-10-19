# OA 阿里云 OSS 文件传输参考审计

日期：2026-09-20  
参考仓库：`RoadSystemOA/road-design-oa-system`  
参考提交：`826e0fc65aa09947fc4f64f62a2644fa5d11a302`  
状态：参考路径在审计时无工作区改动；仅证明可复用信号，不证明能够原样移植

## 审计目的

确认 OA 中已经实际使用的阿里云 OSS 上传下载方案，哪些机制可以成为 BizFoundry Evidence Service 的输入，哪些内容属于 OA 定制或仍需补强。

## 已核对的实现

- `docs/FILE_TRANSFER_UNIFICATION_PLAN.md`：统一传输目标和验收；
- `packages/oss-file-transfer/`：浏览器端 STS、multipart、下载和预览客户端；
- `backend/internal/services/file_service.go`：上传会话、STS、完成、取消、签名 URL 和 OA 权限；
- `backend/pkg/storage/interface.go`、`oss.go`、`minio.go`：存储端口与 OSS／MinIO 实现；
- `backend/internal/models/multipart_upload.go`：上传任务状态；
- `backend/internal/handlers/file_handler.go`、`router/router.go`：现有 HTTP 入口。

## 已成立的可复用信号

OA 已经形成以下完整主干：

1. 后端建立上传会话，预留 File ID 与 Object Key；
2. 后端通过 RAM Role 取得短期 STS；
3. 浏览器使用 `ali-oss` multipart 直传 OSS，支持进度、并发分片和取消；
4. 浏览器调用 complete，后端检查对象存在并建立文件记录；
5. 下载和预览先经过后端权限检查，再取得短期签名 URL；
6. 文件字节不再经过业务 API 和 Gateway；
7. 前端传输包把 endpoint 适配与 OSS 传输机制分开。

这些机制直接支持 BizFoundry 的目标：大文件数据面直达对象存储，授权、会话和元数据确认仍由受信服务控制。

## 必须剥离的 OA 定制

- API 路径和上传任务强制以 Project ID 为父级；
- Object Key 内包含 OA 的项目和 File Category；
- 文件类别枚举绑定合同、招投标、生产、审计和发票；
- PermissionService 读取 OA 项目和岗位规则；
- FileService 直接读取多个 OA 业务表计算文件引用；
- GORM 模型、旧单附件兼容字段和 OA 软删除策略。

Evidence Service 应只接受稳定的租户、调用应用、File ID、Upload Session、对象键和技术元数据。业务附件类型及其生命周期属于调用应用。

## 移植前必须补强

| 风险 | OA 当前证据 | Evidence Service 要求 |
| --- | --- | --- |
| 上传完成真实性 | complete 当前主要检查对象是否存在 | 同时核对预期大小、服务端对象元数据、摘要或可信校验结果 |
| STS 权限范围 | 已证明使用 RAM Role 短期凭证，未在所审代码中证明按单一 Object Key 收窄 | 每个会话只允许约定 Bucket／Object Key 和必要 multipart 动作，短 TTL，禁止列举与越权写入 |
| 状态原子性 | File 记录创建与 Upload 状态更新不是一个明确的单事务提交 | complete 必须幂等；版本、会话状态和审计保持可恢复的一致状态 |
| 取消与遗留对象 | 前端会尝试 abort，现有清理仍可能遗留 OSS 对象或会话 | 定期扫描过期 session、未确认对象和失败清理，全部可审计、可重试 |
| 内容可信度 | 文件类型主要来自扩展名／客户端 MIME | 服务端探测类型，保存实际大小和摘要；危险内容策略另行定义 |
| 业务解耦 | 上传、权限和引用仍在 OA FileService 中交织 | Evidence 只管理技术文件事实；业务授权通过稳定 Capability 与有界引用证明进入 |
| 多租户对象键 | 当前以 Project 为主要路径范围 | 对象键必须包含不可猜测、不可跨租户覆盖的服务端生成边界，不信任客户端路径 |
| 删除和保留 | OA 已识别软删除、孤儿文件和引用清理不完整 | 文件删除、版本保留、法律保留与物理回收必须显式分开 |

## 结论

OA 的 `STS → 浏览器 multipart 直传 OSS → complete → 短期签名 URL` 可以作为 BizFoundry Evidence Service 的第一实现参考，前端传输包也可作为候选复用来源。

它不是可以直接复制的独立服务：当前实现仍绑定 OA Project、分类、权限和数据库对象，并有完成校验、最小权限、原子状态与孤儿清理缺口。进入 M10 编码前必须重新定义 Evidence API、IAM Capability、对象键策略、威胁模型和契约测试；任何代码复用还需核对许可证、依赖与版本维护责任。
