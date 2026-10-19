# Evidence HTTP Contract v1

该目录定义 File Transfer Core 与未来 Evidence Service 之间的业务无关控制面契约。

最小根因：**业务页面各自拼装上传、取消、下载和预览流程容易产生差异 → 所有调用方通过同一版本化契约获得可复用且可验证的文件传输行为。**

## 边界

- 契约只认识 File ID、Version、Upload Session、文件技术元数据、短期上传授权和短期访问授权；
- `client_context` 是不透明的调用方关联信息，Evidence 不把 `owner_ref` 或 `purpose` 解释成项目、合同或台账规则；
- 不包含 Project ID、OA 文件分类、客户角色、页面权限或业务状态；
- 身份凭证由调用方通过标准请求头注入，File Transfer Core 不签发或解释 IAM 凭证；
- OSS Object Key、Bucket 和授权动作只能来自服务端响应，浏览器不能自行选择；本地模式同样只接受服务端签发的单对象 PUT URL；
- `complete` 请求只提交客户端观察值，未来 Evidence Service 必须自己复核对象、大小、媒体类型和摘要。

## 固定端点

| 动作 | 端点 |
| --- | --- |
| 建立上传会话 | `POST /v1/upload-sessions` |
| 为同一 File 追加服务端分配的不可变版本 | `POST /v1/files/{file_id}/versions/upload-sessions` |
| 完成上传 | `POST /v1/upload-sessions/{session_id}/complete` |
| 取消上传 | `POST /v1/upload-sessions/{session_id}/abort` |
| 获取短期访问授权 | `POST /v1/files/{file_id}/versions/{version}/access-grants` |

创建 File、追加 Version、完成和取消均携带 `Idempotency-Key`。追加版本时客户端只提交文件技术元数据，版本号由 Evidence Service 单调分配且不得复用；具体请求、响应和失败结构见 [`openapi.json`](./openapi.json)。该契约已经由 Evidence Service 实现；业务应用仍只通过稳定 File ID／显式 Version 使用它，不拥有存储位置。

`authorization.provider` 是显式判别字段：`aliyun-oss` 返回对象键级 STS 和 multipart 参数，`local-http` 返回对象键级短期 PUT URL。OSS multipart 的 RAM 动作固定为 `oss:PutObject`、`oss:AbortMultipartUpload` 和 `oss:ListParts`；初始化、上传分片和完成操作都由阿里云映射到 `oss:PutObject`，不得虚构更宽或不存在的动作。
