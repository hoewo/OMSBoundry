# 第 14 轮：固定入口链与 OSS Evidence Service

日期：2026-09-20  
状态：已进入开发决策总表；目标设计尚未实现

## 本轮要解决的问题

此前完整组合只确认“默认启用 Gateway”，Nginx 仍被描述为 Gateway 的一种候选实现，文件能力也保留了嵌入业务应用的本地 Provider。这会留下两个模糊点：

1. Nginx 与 Gateway 到底是替代关系还是固定的两层入口；
2. 每个业务应用是否还要重复实现上传、下载、对象存储和临时访问地址。

本轮把这两个问题正式收敛，并把既有 OA 的阿里云 OSS 方案作为可审计的参考输入。

## 决策一：完整组合固定为三种不同责任

完整组合的外部访问链固定为：

```text
Browser
  → Nginx
  → Gateway
  → IAM 或目标业务应用
```

- **Nginx** 是唯一公网边缘入口，负责 HTTPS 终止、域名和静态资源、基础反向代理、请求体与连接级约束；
- **Gateway** 是内部应用入口，负责 Suite Manifest 路由、基础 Token 验证、身份 Header 清理、基础限流和统一访问日志；
- **IAM** 是身份与授权事实所有者，负责账号生命周期、登录、Token 与能力凭证，不承担所有业务请求的转发；
- **目标应用** 复验凭证并结合自己的状态、revision 和业务不变量作最终授权。

两条典型链路：

```text
登录：Browser → Nginx → Gateway → IAM
业务：Browser → Nginx → Gateway → Contract / Money Flow / Project / Evidence
```

Nginx 与 Gateway 的部分能力会重叠，但在完整组合中不再互相替代。单应用开发测试可以绕过完整入口链直接访问应用；正式 Complete Suite 必须按上述边界部署。

## 决策二：文件传输成为独立小服务

文件能力的最小根因是：

> 多个应用重复实现上传、下载和对象存储会产生安全、可靠性和维护差异 → 由一个独立小服务提供统一、可验证的文件传输与保存能力。

该小服务沿用 **Evidence Service** 名称。业务应用不再包含阿里云 OSS SDK、长期密钥、上传分片、签名 URL、文件字节下载代理或对象清理逻辑，只保存：

- Evidence／File ID；
- 本地业务对象 ID；
- 业务附件类型、显示名称和必要的业务说明；
- 形成业务事实时所引用的 Evidence version。

Evidence Service 拥有：

- 文件 ID、版本、原始文件名、服务端确认的大小、媒体类型和摘要；
- 上传会话、对象键、传输状态、过期与遗留对象清理状态；
- 本地文件系统或阿里云 OSS 等存储 Provider；
- 上传完成确认、受控下载、预览临时地址和技术访问审计；
- 文件字节备份、恢复和完整性校验。

它不理解“合同正本”“付款凭证”“施工图”等业务含义，也不决定文件是否足以让合同生效或付款通过。

## 阿里云 OSS 第一目标方案

OA 已经走通的主干可作为 Evidence Service 的首个对象存储参考：

```text
1. 业务应用请求创建 Evidence 上传意图
2. Evidence Service 校验 IAM 能力与目标对象范围
3. Evidence Service 预留 File ID 和 Upload Session
4. Evidence Service 返回只允许指定对象键、短时有效的 OSS STS／上传参数
5. 浏览器分片直传 OSS，文件字节不经过 Nginx、Gateway 或业务 API
6. 浏览器提交 complete
7. Evidence Service 从 OSS 复核对象大小、媒体类型与摘要后形成可引用版本
8. 业务应用用 File ID／Version 建立自己的业务引用
```

下载和预览：

```text
Browser → Nginx → Gateway → Evidence Service：申请访问
Evidence Service：校验能力与调用方提供的业务授权证明
Evidence Service → Browser：返回短期签名 URL
Browser → OSS：直接下载或预览字节
```

大文件字节不经过应用入口链，但创建会话、完成确认和取得访问地址仍经过 IAM、Gateway 与 Evidence Service 的控制面。

## 对 OA 方案的取舍

可复用的机制：

- STS 短期凭证；
- 浏览器 multipart 直传；
- `initiate → direct upload → complete／abort` 生命周期；
- 短期签名 URL 下载与预览；
- 进度、取消、重试和预览内存阈值；
- OSS／本地存储端口思想。

不能直接带入的内容：

- `/projects/:id` 路由、`project_id` 强依赖和 OA 文件分类；
- OA PermissionService 和客户岗位规则；
- OA 的数据库模型、GORM 对象与业务附件表；
- 仅检查对象存在、不核对可信大小与摘要的完成条件；
- 没有证明对象键级最小权限的宽泛 STS 策略；
- 未完成遗留上传、软删除对象与数据库不一致治理的清理路径。

详细证据见 [OA 阿里云 OSS 文件传输参考审计](../specs/evidence/oa-oss-transfer-reference-audit.md)。

## 独立交付不等于重复嵌入

Evidence Service 是独立运行单元，但仍支持小公司的一台 ECS：一个进程、一套自己的数据库／migration，以及一个本地目录或 OSS Bucket。Contract 等单应用若需要独立交付，可以把 Evidence Service 一并放入交付包并以本地文件模式启动，但业务应用本身不再复制存储实现。

这会增加一个进程，却消除每个业务应用各写一套文件安全和 OSS 集成的长期成本。没有文件需求的小应用不强制依赖 Evidence Service。

## 仍然开放

- Evidence API 与引用授权证明的具体 Schema；
- STS 使用限制对象键的 inline policy 还是预签名 multipart；
- 服务端如何高效取得并校验 SHA-256；
- 防病毒／恶意文件扫描是否进入 v1；
- 文件版本、删除保留期、孤儿对象回收与法律保留规则；
- 单应用 release 如何编排 Evidence 进程、数据库和本地目录。
