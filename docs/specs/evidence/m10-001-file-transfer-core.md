# M10-001 File Transfer Core 公共组件

日期：2026-09-21
状态：组件已实现并完成本地自动验证；后续 M10-002 Evidence Service 已消费该契约并完成独立应用工程验证

## A→B

**业务页面各自实现上传、取消、下载和预览会产生重复、安全和行为差异 → 所有调用方通过一个业务无关组件完成同形、版本化、可测试的文件传输。**

## 本切片为什么先于服务应用

OA 已经证明 STS、浏览器 multipart、complete 和短期访问 URL 的主链长期可用，但现有实现仍夹带 Project、文件分类、OA 权限和默认路径。直接复制为服务会把这些定制固化进 Evidence API。

本切片先冻结调用方能够依赖的最小传输协议与浏览器行为，再由后续 Evidence Service 实现服务端事实、权限和存储。组件不是空骨架：它已经可构建、可打包并以模拟 Evidence API 完成上传与访问闭环测试。

## 已实现范围

- 新增独立 TypeScript 包 `packages/file-transfer-core`；
- 新增 `contracts/evidence/v1/openapi.json` 版本化控制面契约；
- HTTP Adapter 负责创建、完成、取消上传会话和申请访问授权；
- Aliyun OSS Adapter 使用服务端给出的 STS、Bucket 和唯一 Object Key 执行 multipart；
- 上传成功调用幂等 complete，临时失败使用同一 Idempotency Key 重试；
- 上传失败和用户取消都会尽力取消服务端会话，OSS Adapter 同时取消 multipart；
- 下载使用短期访问 URL；预览支持小文件 Blob、中等 PDF 直连和大文件下载策略；
- 访问授权缓存绑定调用方提供的身份范围；没有身份范围时默认不缓存；
- 组件输出 File ID 与 Version，业务应用自行保存业务引用。

## 稳定边界

组件和契约不得包含：

- `project_id` 或 Project 必填依赖；
- OA 文件类别、合同类型、票据类型或客户岗位；
- 角色、菜单、按钮或页面权限判断；
- 长期 OSS AccessKey；
- 数据库、IAM 签发、对象完成校验、孤儿对象清理或备份恢复实现。

可选 `client_context` 只包含不透明 `owner_ref`、`purpose` 和字符串 metadata。Evidence 不能把它们解释成权威业务状态，也不能依此绕过服务端 Capability 与资源授权。

## 控制面与数据面

```text
业务 UI
  → File Transfer Core
    → Evidence v1 控制面：create / complete / abort / access-grant
    → Aliyun OSS 数据面：multipart 文件字节
```

组件永远使用 Evidence 响应中的 Object Key。业务 UI 不能传入 Bucket、Object Key 或 OSS Action；文件字节不经过业务 API。

## 验收

```bash
cd packages/file-transfer-core
npm ci --ignore-scripts --no-audit --no-fund
npm run check
npm audit --omit=dev --audit-level=high
```

自动测试至少覆盖：

- 业务无关 create／complete HTTP 映射；
- 同一 complete 幂等键的临时失败重试；
- 上传失败与用户取消后的 abort；
- 身份范围变化时不复用访问授权；
- 未提供身份范围时不缓存；
- Blob／直连／下载预览策略；
- OSS 只使用服务端 Object Key；
- 额外或过宽 OSS Action 失败关闭；
- OpenAPI 路径、主版本、未知字段关闭和禁止业务字段守卫；
- TypeScript 声明与可发布构建。

## 明确未完成

- Evidence Service 进程、数据库、migration 和 IAM Capability；
- 服务端对象键生成与单对象 STS Policy；
- complete 的对象大小、服务端媒体类型与 SHA-256 复核；
- 本地文件 Provider、OSS Provider 服务端实现与契约测试；
- 过期会话／孤儿对象扫描、重试和审计；
- Contract Ledger 现有本地 Evidence 迁移；
- 真实 OSS Bucket／CORS／150 MiB 文件验收不属于本切片完成门槛；该外部验收已于 2026-09-22 作为后续 Evidence 组合证据通过核心链，剩余负例见 `docs/testing.md`。

上述内容属于后续 M10 服务切片，不能因为浏览器组件通过而宣称 Evidence Service 已完成。
