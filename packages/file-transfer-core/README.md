# @omsbizfoundry/file-transfer-core

业务无关的浏览器文件传输组件。它统一执行“建立上传会话 → 浏览器 multipart 直传 → 幂等 complete → 短期授权下载／预览”，但不保存文件事实，也不是可部署的 Evidence Service。

最小根因：**业务页面各自实现上传下载会产生重复、安全和行为差异 → 通过一个稳定 SDK 完成同形文件传输。**

## 组件负责什么

- 调用版本化 Evidence HTTP 契约；
- 根据服务端判别字段把字节发送到 Aliyun OSS 或单对象本地 HTTP 上传地址；
- 上传进度、浏览器取消和 OSS multipart 中止；
- 使用同一幂等键重试 complete；
- 失败或取消时尽力终止服务端上传会话；
- 申请短期访问授权并触发下载；
- 小文件 Blob 预览、中等 PDF 直连预览和大文件下载策略；
- 只有调用方提供非空身份范围时才缓存访问授权，避免跨账号复用。

## 组件不负责什么

- 不解释 Project、合同、台账、OA 分类、客户角色或页面按钮；
- 不签发 IAM 凭证，也不决定调用方是否有权访问文件；
- 不生成 Bucket 或 Object Key；
- 不校验 OSS 对象真实性，不持久化 File／Version／Upload Session；
- 不清理孤儿对象，不替代未来 Evidence Service；
- 不提供 React/Vue 上传控件。UI 只消费 SDK 的进度和结果。

`clientContext.ownerRef`、`purpose` 和 `metadata` 只是可选的不透明关联信息。Evidence Service 不应根据这些字符串执行客户业务规则；权威业务含义仍由引用文件的业务应用保存。

## 安装与验证

```bash
npm ci --ignore-scripts
npm run check
```

`check` 依次执行严格 TypeScript 检查、单元测试、Evidence v1 契约守卫和可发布构建。

## 使用

```ts
import {
  FileTransferClient,
  createAliOssMultipartUploader,
  createDirectUploaderRouter,
  createFetchFileTransferApi,
  createLocalHttpUploader,
} from '@omsbizfoundry/file-transfer-core'

const api = createFetchFileTransferApi({
  baseUrl: '/evidence',
  getHeaders: () => ({
    Authorization: `Bearer ${getEvidenceCredential()}`,
  }),
})

const client = new FileTransferClient({
  api,
  uploader: createDirectUploaderRouter({
    aliyunOss: createAliOssMultipartUploader(),
    localHttp: createLocalHttpUploader(),
  }),
  // 必须同时包含租户和可信 Subject；省略时 SDK 不缓存访问授权。
  getCacheScope: () => `${currentTenantId()}:${currentSubjectId()}`,
})

const result = await client.uploadFile({
  file,
  clientContext: {
    ownerRef: 'opaque-resource-reference',
    purpose: 'supporting-document',
  },
  onProgress: ({ percent }) => renderProgress(percent),
})

// 业务应用只保存 result.version.fileId 和 result.version.version。
```

取消上传：

```ts
const controller = new AbortController()

const pending = client.uploadFile({ file, signal: controller.signal })
controller.abort()
await pending
```

下载和预览：

```ts
await client.downloadFile(fileDescriptor)

const policy = client.getPreviewPolicy(fileDescriptor)
if (policy.mode !== 'download') {
  const previewUrl = await client.resolvePreviewUrl(fileDescriptor, policy.mode)
  renderPreview(previewUrl)
}
```

退出登录或切换租户时必须调用：

```ts
client.clearCache()
```

## HTTP 契约

公开契约位于 [`../../contracts/evidence/v1/`](../../contracts/evidence/v1/README.md)。SDK 默认使用：

- `POST /v1/upload-sessions`
- `POST /v1/upload-sessions/{session_id}/complete`
- `POST /v1/upload-sessions/{session_id}/abort`
- `POST /v1/files/{file_id}/versions/{version}/access-grants`

这些路径相对于 `baseUrl`。认证 Header 由 `getHeaders` 注入，因此 SDK 不依赖某一种 IAM 实现。

## 安全约束

- Aliyun OSS endpoint 默认必须为 HTTPS；
- 必须存在临时 `security_token` 和未来有效的过期时间；
- 授权范围中的 Bucket／Object Key 必须与上传目标完全一致；
- OSS 授权动作必须恰好是 `oss:PutObject`、`oss:AbortMultipartUpload` 和 `oss:ListParts`，不接受 `oss:*`、列举对象或额外动作；
- 本地模式只接受短期、单对象、单 `evidence:PutObject` 动作的 PUT URL；
- 浏览器提交的大小、ETag、分片和摘要都只是观察值，服务端仍须独立复核；
- 上传完成、取消和访问授权的服务端鉴权、幂等、审计及租户隔离属于下一阶段 Evidence Service。
