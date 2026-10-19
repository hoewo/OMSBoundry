# 阿里云 OSS 首次真实验收清单

日期：2026-09-22  
状态：账号侧核心配置与真实浏览器主链已通过；剩余负例和生产化待继续

## A→B

**Evidence Service 已有 OSS Provider，但尚未接触真实阿里云资源 → 用一套私有 Bucket、最小 RAM 权限和精确 CORS 跑通真实浏览器 multipart，并证明没有公开访问或长期密钥下发。**

本清单只配置 Evidence Service 首次外部验收需要的资源，也是当前 RAM 用户、上传角色、权限策略与 AccessKey 配置流程的唯一操作入口。不要创建公开 Bucket，不要给应用 `AliyunOSSFullAccess`、`AliyunRAMFullAccess` 或主账号 AccessKey；后续页面名称变化时更新本文件，不另存一份容易漂移的口头步骤。

## 当前外部资源状态

- 根域名 `xidou.me` 已注册，DNS、HTTPS 与中国内地部署可能需要的 ICP 备案仍待后续配置；
- `app.xidou.me` 的 DigiCert DV 个人测试 SSL 证书已于 2026-09-22 签发并以 Nginx 格式下载到仓库外受控目录；只读检查确认 PEM 含叶证书与中间证书两段、私钥格式有效且公钥摘要与证书一致，SAN 覆盖 `app.xidou.me` 和 `www.app.xidou.me`，证书有效期为 2026-09-22 00:00:00 至 2026-12-20 23:59:59 UTC。下载 ZIP 已收紧为当前用户读写的 `0600`；证书尚未部署到 Nginx 或形成公网 HTTPS 入口，因此“已签发／已下载”不等于 HTTPS“已上线”。私钥只保存在本机受控目录或部署机 secret，不进入聊天、Git、镜像或普通 Compose 文件；
- 已创建测试 Bucket `xidou-test-files`，Region 为华北 2（北京）／`cn-beijing`，默认公网 Endpoint 为 `https://oss-cn-beijing.aliyuncs.com`，Bucket 域名为 `https://xidou-test-files.oss-cn-beijing.aliyuncs.com`；
- 已创建本地验收 RAM 用户 `omsbizfoundry-evidence-validation`，用户 ARN 为 `acs:ram::1717277548560736:user/omsbizfoundry-evidence-validation`；服务调用策略已经绑定，AccessKey 已写入本机被 Git 忽略的环境文件；此前凭证结果页截图曾显示完整 ID／Secret，若当前环境仍使用截图中的同一组凭证，则该组必须按已泄露处理，在后续继续使用或进入生产前先轮换，不能因截图只出现在当前对话而例外；
- 2026-09-22 已用本机环境文件通过 `GetCallerIdentity` 验证新 AccessKey，返回身份与上述 RAM 用户及账号一致；使用与 Evidence Service 实现一致的参数，对指定上传角色执行 900 秒、单 Object Key 会话策略的 `AssumeRole` 已成功并只输出脱敏后的角色 ARN 与过期时间。一次额外携带应用不会发送的 `SourceIdentity` 的诊断请求曾返回 `403 NoPermission`，去掉该额外参数后成功，因此当前调用用户策略与角色信任链已经通过，无需为探测命令增加额外权限；
- 已人工确认上传角色的权限列表只有自定义策略 `OMSBizFoundryEvidenceUploadPolicy`，不存在 `AliyunOSSFullAccess`；浏览器角色权限上限已按本手册收口；
- 本机 `backend/services/evidence-service/.env.oss-validation` 已配置数据库密码、模板访问令牌和新 RAM 用户 AccessKey；脱敏检查确认四项均已脱离占位符，文件权限为 `600` 且受根 `.gitignore` 的 `.env.*` 规则保护，任何实际值均未写入文档或命令输出；
- 控制台截图已确认：私有、阻止公共访问已开启、`SSE-OSS`／`AES256` 已启用、TLS 版本设置已开启且只允许 TLS 1.2／1.3并选择强加密算法套件、标准存储、同城冗余、版本控制关闭、归档直读关闭；CORS 已按固定本地 Origin 保存并通过真实预检；
- 域名不是 OSS 验收前置条件。Bucket 创建后已有阿里云默认 Endpoint，私有对象通过 STS 或签名 URL 访问即可；首次真实验收不等待 `xidou.me`；
- 后续可以把产品入口放在 `app.xidou.me`／其他确定子域；是否再把 `files.xidou.me` 绑定到 OSS 是独立可选项。绑定前要补做 ICP、证书、签名 Endpoint 和浏览器预览回归，不能直接替换当前已验证 Endpoint。

## 0. 本次实际运行拓扑

本次本机验收不是“全部跑在 Docker 中”。实际边界如下：

| 部件 | 当前运行位置 | 当前网络路径 |
| --- | --- | --- |
| 浏览器验收页 | 宿主机 Vite 进程，`http://127.0.0.1:5173` | 控制面先到 Vite 本地代理；文件字节直接经公网 HTTPS 到 OSS |
| Evidence Service | Docker 容器，宿主机只映射 `127.0.0.1:8085` | 经公网访问阿里云 STS；当前本机也经公网 OSS Endpoint 做 Complete 复核、签名和清理 |
| PostgreSQL | Docker 容器 | 只在 Compose 私有网络中与 Evidence 通信，不发布宿主端口 |
| 阿里云 OSS／STS | 阿里云外部服务 | 不在 Docker 或 ECS 内；浏览器与当前本机后端都通过公网 HTTPS 访问 |

```text
宿主机浏览器
  ├─ 控制面 → Vite :5173 → Evidence 容器 :8085 → PostgreSQL 容器
  └─ 文件字节 ───────────────────────────────→ OSS 公网 Endpoint

Evidence 容器
  ├─ AssumeRole ─────────────────────────────→ STS 公网 Endpoint
  └─ 复核／读取／清理 ───────────────────────→ OSS 公网 Endpoint（仅当前本机验收）
```

正式同地域 ECS 的目标路径不同：浏览器不在阿里云 VPC 内，所以直传和短期签名下载必须继续使用公网 `OSS_BROWSER_ENDPOINT`；Evidence 后端的 Complete 复核、读取和清理使用 `OSS_SERVICE_ENDPOINT` 内网地址，避免后端对象流量绕公网。两个地址属于同一个私有 Bucket，不是两份数据。

```text
互联网浏览器 ── HTTPS ──→ OSS 公网 Endpoint
同地域 ECS 上的 Evidence ── VPC 内网 ──→ OSS internal Endpoint
```

`OSS_ENDPOINT` 仍作为旧配置的兼容回退，同时填充上述两个地址；本机现有 `.env.oss-validation` 因此无需立刻修改。新部署必须显式区分两项，绝不能把 `*-internal.aliyuncs.com` 返回给公网浏览器。

## 1. 账号先做什么

1. 为阿里云主账号开启 MFA；日常操作另建 RAM 管理员或使用已有受控管理员。
2. 不为主账号创建 AccessKey；本地验收只使用专用 RAM 用户的一组临时保管 AccessKey。
3. 凭证只保存在本机密码管理器和被 Git 忽略的环境文件中，不进入聊天、仓库、镜像、Compose 文件或浏览器。

## 2. 创建 Bucket

当前测试 Bucket：`xidou-test-files`。

| 配置 | 首次验收值 | 原因 |
| --- | --- | --- |
| Region | `cn-beijing` | 已创建于华北 2（北京）；未来 ECS 优先选同地域 |
| ACL | 私有 | 文件只能经 Evidence 授权访问 |
| 阻止公共访问 | 开启 | 拒绝公共 ACL／Bucket Policy 意外开放 |
| 存储类型 | 标准存储 | 首次验收不引入归档恢复语义 |
| 冗余类型 | 本地冗余即可 | 首次验收控制成本；生产档另行选型 |
| 服务端加密 | SSE-OSS／AES-256 | Bucket 落盘加密，不增加 KMS 前置依赖 |
| 版本控制 | 暂不开启 | 当前清理语义是删除对象；开启版本控制前需单独验证 Delete Marker 和成本 |

可以增加一条生命周期规则：1 天后中止未完成的 multipart 上传。该规则只是 OSS 侧兜底，不能替代 Evidence Service 自己的过期会话清理和审计。

## 3. RAM 身份模型与最终资源

### 3.1 四个对象分别负责什么

| 对象 | 当前名称 | 作用 | 是否拥有长期 AccessKey |
| --- | --- | --- | --- |
| 上传权限策略 | `OMSBizFoundryEvidenceUploadPolicy` | 定义浏览器临时身份最多能在指定前缀上传、查询分片和取消分片 | 否；策略不是身份 |
| 浏览器上传角色 | `OMSBizFoundryEvidenceUploadRole` | 承载上传权限上限和信任边界，被扮演后形成临时身份 | 否；角色只有临时 STS 凭证 |
| 服务调用策略 | `OMSBizFoundryEvidenceServiceCallerPolicy` | 允许 Evidence 后端只扮演上传角色，并读取／清理指定对象 | 否；策略不是身份 |
| 本地验收用户 | `omsbizfoundry-evidence-validation` | 在尚无 ECS 实例身份时，让本地 Evidence 进程用 AccessKey 证明调用者身份 | 是；仅本地验收创建一组 |

权限策略才是“权限包”；RAM 用户和 RAM 角色都是身份。用户可以拥有长期 AccessKey，角色没有长期凭证，但 App 可以通过 STS、ECS 实例元数据或联合身份获得角色的短期凭证。因此不能把角色理解为“不能对接 App 的权限包”：浏览器最终正是使用上传角色的临时身份直传 OSS。

一个 RAM 用户的 AccessKey 技术上可以申请扮演多个角色，前提是用户策略和每个角色的信任策略分别允许；但每次 `AssumeRole` 只产生一个角色的独立临时会话，用户权限与多个角色权限不会合并。当前服务调用策略只列出一个上传角色 ARN，不提前开放其他角色，也不绑定允许扮演所有角色的 `AliyunSTSAssumeRoleAccess`。

### 3.2 当前本地调用链

```text
Evidence 进程
  │ 使用专用 RAM 用户 AccessKey 证明“我是谁”
  ▼
阿里云 STS AssumeRole
  │ 同时校验：调用用户允许扮演 + 上传角色信任调用者
  ▼
OMSBizFoundryEvidenceUploadRole
  │ 角色权限上限 ∩ 本次单 Object Key 会话策略
  ▼
约 15 分钟的临时 AK / Secret / SecurityToken
  │ 只交给当前浏览器上传会话
  ▼
浏览器 multipart 直传 xidou-test-files
```

上传角色不负责“颁发 STS”；阿里云 STS 服务负责签发临时凭证。RAM 用户不是 OMS 的登录用户，它只是本地 Evidence 后端的机器身份。Evidence Service 每次申请临时凭证时还会提交单一 Object Key 的会话策略，因此浏览器最终权限是“角色策略与会话策略的交集”，不是整个角色权限的复制。

## 4. 完整创建与核对流程

以下顺序可以在新账号完整复演。当前个人验证账号已经创建上传角色并完成真实验收；已存在的步骤按“核对”执行，不重复创建同名资源。仓库中的策略文件使用 `<ALIYUN_ACCOUNT_ID>` 和 `<PRIVATE_BUCKET_NAME>` 占位符，应用前必须替换为目标账号和目标私有 Bucket，不能把当前个人账号值复制到公司环境。

### 4.1 创建浏览器上传权限策略

1. 进入“RAM 访问控制 → 权限管理 → 权限策略”。
2. 点击“创建权限策略”，选择“脚本编辑”。
3. 删除默认内容，粘贴下面 JSON。
4. 点击“确定”或“继续编辑基本信息”，名称填写 `OMSBizFoundryEvidenceUploadPolicy`，备注说明它只用于 Evidence 浏览器直传。

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "oss:PutObject",
        "oss:AbortMultipartUpload",
        "oss:ListParts"
      ],
      "Resource": [
        "acs:oss:*:*:xidou-test-files/tenants/*"
      ]
    }
  ]
}
```

同一份可直接粘贴的 JSON 也保存在 [`deployments/aliyun/evidence-browser-upload-policy.json`](../../deployments/aliyun/evidence-browser-upload-policy.json)。

### 4.2 创建并配置浏览器上传角色

1. 进入“RAM 访问控制 → 身份管理 → 角色 → 创建角色”。
2. 可信实体类型选择“阿里云账号”，可信账号选择“当前阿里云账号”。
3. 角色名称填写 `OMSBizFoundryEvidenceUploadRole`，完成创建。
4. 打开角色详情，在“权限管理 → 新增授权”中把策略类型筛选为“自定义策略”，只绑定 `OMSBizFoundryEvidenceUploadPolicy`。
5. 如果列表中存在 `AliyunOSSFullAccess`，立即解除；角色不需要 `GetObject`、`DeleteObject`、列举 Bucket、管理 Bucket 或管理 RAM 的权限。

角色“信任策略”应允许当前账号 `acs:ram::1717277548560736:root` 执行 `sts:AssumeRole`。可核对的 JSON 保存在 [`deployments/aliyun/evidence-browser-upload-role-trust-policy.json`](../../deployments/aliyun/evidence-browser-upload-role-trust-policy.json)。这里的 `root` 只表示信任当前账号中的 RAM 身份，不表示应用使用主账号 AccessKey；真正调用者还必须通过自身策略获得精确到本角色 ARN 的 `sts:AssumeRole` 权限。两侧任意一侧不允许，STS 都应拒绝。

上传角色的最终权限列表必须只有 `OMSBizFoundryEvidenceUploadPolicy`。这只是浏览器临时身份的权限上限；Evidence Service 仍会把每次会话缩到唯一 Object Key 和约 15 分钟 TTL。

### 4.3 创建服务调用权限策略

1. 进入“RAM 访问控制 → 权限管理 → 权限策略”。
2. 点击“创建权限策略”，选择“脚本编辑”。
3. 删除默认内容，粘贴下面 JSON。
4. 点击“确定”或“继续编辑基本信息”，名称填写 `OMSBizFoundryEvidenceServiceCallerPolicy`。

这份策略只允许申请扮演当前上传角色，以及读取／删除当前 Bucket 的 `tenants/*` 对象。它不允许直接上传、不允许管理 Bucket，也不允许扮演其他角色。

```json
{
  "Version": "1",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["sts:AssumeRole"],
      "Resource": [
        "acs:ram::1717277548560736:role/omsbizfoundryevidenceuploadrole"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "oss:GetObject",
        "oss:DeleteObject"
      ],
      "Resource": [
        "acs:oss:*:*:xidou-test-files/tenants/*"
      ]
    }
  ]
}
```

同一份可直接粘贴的 JSON 保存在 [`deployments/aliyun/evidence-service-caller-policy.json`](../../deployments/aliyun/evidence-service-caller-policy.json)。建议策略名称为 `OMSBizFoundryEvidenceServiceCallerPolicy`。

服务身份需要读对象，是因为 Complete 会重新读取真实字节并核对大小、媒体类型和 SHA-256；需要删对象，是因为取消、过期和失败会话必须清理。它不需要直接 `PutObject`，浏览器上传必须经过受限角色。

### 4.4 创建本地验收 RAM 用户

1. 进入“RAM 访问控制 → 身份管理 → 用户 → 创建用户”。
2. 登录名称填写 `omsbizfoundry-evidence-validation`，显示名称填写“Evidence 本地验收服务”。
3. 不开启控制台登录，不设置登录密码；如果页面提供“OpenAPI／编程访问”选项，只选择这一项。如果创建页不再提供访问方式，先创建用户，后面再到“凭证管理”单独创建 AccessKey。
4. 打开用户详情，进入“权限管理 → 新增授权”。
5. 授权范围选择“整个云账号”，策略类型选择“自定义策略”，只选择 `OMSBizFoundryEvidenceServiceCallerPolicy` 并确认。

用户的最终权限列表必须只有 `OMSBizFoundryEvidenceServiceCallerPolicy`。不要绑定 `AliyunOSSFullAccess`、`AliyunSTSAssumeRoleAccess`、`AliyunRAMFullAccess` 或 `AdministratorAccess`。

### 4.5 创建并保管 AccessKey

1. 在该用户详情页进入“凭证管理 → AccessKey → 创建 AccessKey”。“认证管理”是相邻但不同的页签，不是当前控制台创建 AccessKey 的入口。
2. 第一个“创建 AccessKey”用途弹窗选择“本地开发环境中使用”。当前凭证供本机 Evidence Service／本机 Docker 内的 Evidence Service 调用 SDK 使用，不选“CLI”，也不选“在阿里云 ECS 上运行的服务”。
3. 勾选“我确认必须创建 AccessKey”，点击“继续创建”。用途选择用于风险提示，不会改变本手册定义的 RAM 权限策略。
4. 按控制台要求完成 MFA、短信或其他安全验证，只创建一组 AccessKey。
5. 创建成功后会出现“保存 AccessKey”结果页，显示 `AccessKey ID` 与 `AccessKey Secret` 两行；这是正常结果，不需要点击下方两个“前往设置”才能获得凭证。
6. Secret 通常只完整显示一次。优先直接保存到本机密码管理器；如下载 CSV，只能保存在受控本机并避免进入网盘、聊天、截图、Git 或共享目录。确认安全保存后再关闭结果页。
7. 结果页的“最大闲置时间”可以稍后设为 90 天未使用自动禁用；本地网络出口不稳定时先不设置 IP 网络限制，生产 ECS 改用实例角色后不再依赖这组长期 AccessKey。
8. 不把 AccessKey ID、AccessKey Secret 或 Security Token 发到聊天，不放进截图、Git、镜像、Compose 文件、前端代码或浏览器存储。
9. 只要完整凭证进入截图、聊天、工单、日志或其他非秘密载体，就必须视为已泄露：关闭结果页，在“凭证管理 → AccessKey”中立即禁用并删除该 AccessKey，再新建一组；泄露的凭证不能继续写入 `.env.oss-validation`，也不能因为聊天是私有的而保留使用。

AccessKey 只证明“当前请求来自这个 RAM 用户”。它本身不等于上传角色；Evidence 使用它调用 STS，STS 再根据用户调用策略和角色信任策略签发上传角色的短期凭证。

### 4.6 完成后的权限审计

| 核对对象 | 必须存在 | 必须不存在 |
| --- | --- | --- |
| 上传角色 | `OMSBizFoundryEvidenceUploadPolicy`；已确认角色 ARN | `AliyunOSSFullAccess`、读删对象、Bucket 管理、RAM 管理权限 |
| 本地验收用户 | `OMSBizFoundryEvidenceServiceCallerPolicy`；一组启用的 AccessKey | 控制台登录、允许扮演所有角色、OSS 全权限、RAM 管理权限 |
| Bucket | 私有、阻止公共访问、精确 CORS、约定 `tenants/*` 前缀 | 公共读、公共读写、通配 Origin |
| 浏览器 | 仅接收单 Object Key、短 TTL 的 STS 临时三元组 | RAM 用户的长期 AccessKey |

如果上传角色曾经选择过 `AliyunOSSFullAccess`，必须在权限列表中确认它已经解除，而不能只依赖“创建弹窗可能没有提交”的记忆。完成审计后，只把角色 ARN、策略名称和是否通过核对记录到验收结果；任何凭证值都不进入文档。

### 4.7 生产 ECS 替换

本地验收用户只是尚无 ECS 工作负载身份时的过渡方案。生产上线后：

1. 给运行 Evidence Service 的 ECS 绑定专用实例 RAM 角色；
2. 把与 `OMSBizFoundryEvidenceServiceCallerPolicy` 等价的最小服务端权限授予该实例角色；
3. Evidence 通过实例元数据和默认凭据链获取短期身份，不再读取长期 AccessKey；
4. 验证 STS、Complete 读取、清理删除和浏览器直传后，禁用并最终删除 `omsbizfoundry-evidence-validation` 的 AccessKey；
5. 不把浏览器上传角色直接当成 ECS 服务角色，两个角色职责继续分离。

## 5. 配置 Bucket 安全与 CORS

### 5.1 阻止公共访问

1. 进入“对象存储 OSS → Bucket 列表 → `xidou-test-files`”。
2. 在左侧进入“权限控制”，找到“阻止公共访问”。
3. 设置为开启；Bucket ACL 继续保持“私有”。
4. 核对没有允许匿名用户或 `*` Principal 读取／写入对象的 Bucket Policy。

“阻止公共访问”是防止误配公共 ACL／Policy 的安全门禁，不会阻止 RAM 用户、角色 STS 或签名 URL 的受控访问。

### 5.2 服务器端加密

1. 必须先回到“Bucket 列表 → `xidou-test-files`”，确认页面面包屑中仍有当前 Bucket 名称；不要在 OSS 全局“数据安全”产品页操作。
2. 在当前 Bucket 左侧点击“数据安全”右侧的小箭头展开子菜单，再进入“服务器端加密”。如果页面显示“开通数据安全”、敏感数据识别或按扫描量计费，说明进入了另一个付费产品，应直接返回且不要点击“立即开通”。
3. 点击“设置”，加密方式选择“OSS 完全托管”（即 `SSE-OSS`），加密算法选择 `AES256`，保存为 Bucket 默认加密。
4. `SSE-OSS` 由 OSS 完全托管密钥，阿里云当前费用说明为免费，不要求开通上述敏感数据保护产品或 KMS；当前不选择 KMS。后续只有在客户合规要求、密钥生命周期和成本均明确后才另行设计自选 KMS 密钥。

SSE-OSS 保护的是 OSS 中静态保存的对象：上传时由 OSS 自动加密，授权下载时自动解密，Evidence Service 与浏览器不需要持有或管理加密密钥。它不替代 HTTPS、RAM／STS、阻止公共访问、备份或应用授权；这些机制分别保护传输、访问边界、误公开、恢复和业务权限。Bucket 默认加密只自动作用于启用后新写入的对象；首次验收 Bucket 当前无对象，因此不存在存量对象迁移问题。

### 5.3 最低 TLS 版本

1. 在当前 Bucket 左侧选择“数据安全 → TLS 版本设置”，开启 TLS 版本设置。
2. “允许的 TLS 版本”只选择 TLS 1.2 和 TLS 1.3，不选择 TLS 1.0／1.1；这表示最低版本为 TLS 1.2，同时允许现代客户端协商 TLS 1.3。
3. “TLS 加密算法套件”选择“强加密算法套件”；当前浏览器 SDK 和 Go 服务不需要兼容只支持弱套件的遗留客户端。
4. 保存并确认。阿里云说明配置可能在 30 分钟内生效，刚保存后不能因一次即时探测仍看到旧结果就判定失败。
5. 若真实客户环境以后出现明确的遗留客户端兼容需求，必须先识别客户端和风险再单独决策，不能直接恢复 TLS 1.0／1.1 或全部弱套件。

### 5.4 配置 CORS

首次本地浏览器验收固定使用 `http://127.0.0.1:5173`。这个值已经作为验收契约确定；浏览器验收页运行在该 Origin，不同时使用 `localhost`，不添加尾部 `/` 或路径，也不使用 `*`。实现位于 `acceptance/evidence-oss-browser`，只消费正式 File Transfer Core，不是产品 UI。

| 字段 | 值 |
| --- | --- |
| Allowed Origins | `http://127.0.0.1:5173` |
| Allowed Methods | `GET`、`PUT`、`POST`、`DELETE`、`HEAD` |
| Allowed Headers | `authorization`、`content-type`、`content-md5`、`x-oss-*` |
| Expose Headers | `ETag`、`x-oss-request-id` |
| Max Age | `600` 秒 |
| Vary: Origin | 开启（控制台提供时） |

CORS 只决定浏览器是否允许跨域调用，不授予 OSS 权限。实际读写仍必须同时通过 RAM／STS 校验。

2026-09-22 以 `Origin: http://127.0.0.1:5173`、Method `PUT` 和实际认证／OSS Headers 对 Bucket 默认域名执行 `OPTIONS`，返回 `200`、精确 `Access-Control-Allow-Origin`、全部五种 Methods、请求所需 Headers、`ETag`／`x-oss-request-id`、`Max-Age: 600` 与 `Vary: Origin`，证明规则已生效。

## 6. 本机配置

配置文件固定为 `backend/services/evidence-service/.env.oss-validation`，模板为同目录的 `oss-validation.env.example`。真实文件必须受 Git 忽略且只允许当前本机用户读写；不要提交。该模板已移除 OSS 模式禁止出现的 `LOCAL_SIGNING_KEY`。

首次创建顺序：

1. 以 `oss-validation.env.example` 为基础创建 `.env.oss-validation`，预填当前 Region、Endpoint、Bucket 和 Role ARN；
2. 在自己的终端执行两次 `openssl rand -hex 32`，第一项写入 `EVIDENCE_DB_PASSWORD`，第二项写入 `TEMPLATE_ACCESS_TOKEN`；两个输出不能发送到聊天；
3. 从新 AccessKey 的本机 CSV 中只复制值，分别写入 `ALIBABA_CLOUD_ACCESS_KEY_ID` 和 `ALIBABA_CLOUD_ACCESS_KEY_SECRET`；CSV 不上传到聊天、Git、网盘或共享目录；
4. 执行 `chmod 600 backend/services/evidence-service/.env.oss-validation`；
5. 执行 `git check-ignore -v backend/services/evidence-service/.env.oss-validation`，必须命中根 `.gitignore` 中的 `.env.*` 规则；
6. 只做“缺失／占位符／长度”脱敏检查，不能用 `cat`、`echo` 或调试日志把真实值输出到终端记录。

当前至少包含：

```dotenv
STORAGE_PROVIDER=aliyun-oss
OSS_REGION=cn-beijing
OSS_BROWSER_ENDPOINT=https://oss-cn-beijing.aliyuncs.com
# 本机验收使用公网；同地域 ECS 改为 https://oss-cn-beijing-internal.aliyuncs.com
OSS_SERVICE_ENDPOINT=https://oss-cn-beijing.aliyuncs.com
OSS_BUCKET=xidou-test-files
OSS_ROLE_ARN=acs:ram::1717277548560736:role/omsbizfoundryevidenceuploadrole
OSS_STS_DURATION=15m

ALIBABA_CLOUD_ACCESS_KEY_ID=<只放本机>
ALIBABA_CLOUD_ACCESS_KEY_SECRET=<只放本机>
```

独立容器启动时同时让 Compose 和 Evidence 容器读取这一个文件：

```bash
cd backend/services/evidence-service
EVIDENCE_ENV_FILE=.env.oss-validation \
  docker compose --env-file .env.oss-validation \
  -f compose.standalone.yaml up -d --build --wait
```

本地验收完成后，生产 ECS 不沿用这组长期 AccessKey。生产部署给 ECS 绑定实例 RAM 角色，并给该角色授予与上面“本地验收 RAM 用户”相同的最小服务端权限；应用通过实例元数据默认凭据链取短期凭证。

Endpoint 的职责固定为：

- `OSS_BROWSER_ENDPOINT`：进入上传授权并用于生成浏览器短期访问 URL，只能配置浏览器可达的公网 HTTPS 地址；
- `OSS_SERVICE_ENDPOINT`：Evidence 后端执行 `GetObject`／`DeleteObject` 的地址；本机用公网，同地域 ECS 用内网；
- `OSS_ENDPOINT`：只保留为旧环境文件兼容项；当新配置未提供时同时回退给前两项，不应出现在新生产配置中。

## 7. 双方同步点

账号侧完成后，只需提供以下非敏感信息：

- Region ID；
- Bucket 名称；
- 上传角色 Role ARN；
- 测试 Origin；
- 确认 ACL=私有、阻止公共访问=开启、SSE-OSS=开启、CORS 已保存。

不要提供 AccessKey ID、AccessKey Secret、Security Token、账号密码或 MFA 恢复码。

### 7.1 Auth58 公网环境控制台准备（2026-10-01）

问题与结果：Auth58 已有公网 OA 入口，但 Evidence 仍使用 Local Provider → 在目标账号准备独立私有文件 Bucket 和最小云权限，供后续 D1 接入并验收真实上传、完成校验、下载与故障恢复。

本节沿用上述身份模型和权限边界，资源限定为 Auth58 账号与杭州 Bucket；不复制个人试验田的账号 ARN 或 AccessKey。2026-10-01 从 Mac 实测 `oatestfiles.antuinfo.cn` CNAME 指向 `oatest-auth58-files-1.cn-hangzhou.taihangrda.cn`，HTTPS 证书校验成功、OSS 匿名请求返回 `403 AccessDenied`；这些只证明域名／TLS 连通和本次匿名拒绝，不证明已上传字节、D1 Provider 接入或全部权限已通过。以 `https://oatest.antuinfo.cn` 为 Origin 的 PUT 上传 OPTIONS 探针返回 `403`（`0003-00000601`），尚未形成可用跨域上传预检。

#### 7.1.1 网页 Bucket

如还未完成创建，创建 `oatest-auth58-workbench`：杭州 `cn-hangzhou`、标准存储、本地冗余、私有、阻止公共访问开启、版本控制关闭、服务端加密 OSS 完全托管／AES256。本地冗余表示阿里云地域内的冗余类型，不表示存储在 Mac。本次先准备资源，不把 `oatest.antuinfo.cn` A 记录改成这个 Bucket 的 CNAME。

该 Bucket 只为 BF-080 第一批 CSS／JS／图片独立交付实验准备；当前 Go Workbench 的登录后 HTML 仍由服务器生成。直接全站静态托管、网页读取／发布身份、域名和缓存需要后续规格明确，不能把创建 Bucket 记录为前端分离已完成。

#### 7.1.2 文件 Bucket 的精确 CORS

在 `oatest-auth58-files-1` 的跨域设置新增或修正一条规则：

| 字段 | 值 |
| --- | --- |
| 来源 | `https://oatest.antuinfo.cn`（不加末尾斜杠或路径） |
| 允许方法 | `GET`、`PUT`、`POST`、`DELETE`、`HEAD` |
| 允许 Headers | `authorization`、`content-type`、`content-md5`、`x-oss-*`（每项单独一行） |
| 暴露 Headers | `ETag`、`x-oss-request-id`（每项单独一行） |
| 缓存秒数 | `600` |
| Vary: Origin | 开启（控制台提供时） |

保持文件 Bucket 私有和阻止公共访问开启。已实际通过 HTTPS 的文件域名无需再次申请证书，也不需要把其证书安装到 ECS Nginx。CORS 与证书托管依据 [OSS CORS 文档](https://www.alibabacloud.com/help/en/oss/user-guide/configure-cross-origin-resource-sharing/) 和 [OSS HTTPS 文档](https://www.alibabacloud.com/help/en/oss/user-guide/access-oss-by-https-protocol)。

#### 7.1.3 创建上传策略和上传角色

1. RAM → 权限管理 → 权限策略 → 创建自定义策略 → 脚本编辑；粘贴 [Auth58 上传策略](../../deployments/aliyun/auth58-evidence-upload-policy.json)，名称 `Auth58EvidenceUploadPolicy`。
2. RAM → 身份管理 → 角色 → 创建角色；信任主体选择“阿里云账号／当前账号”，名称 `Auth58EvidenceUploadRole`。
3. 只给该角色绑定 `Auth58EvidenceUploadPolicy`，复制详情页完整角色 ARN。角色 ARN 是非秘密标识，可交给部署者；不手工拼接账号 UID 或角色大小写。

#### 7.1.4 创建服务策略、ECS 角色并授予 Auth58

1. 打开 [Auth58 服务策略模板](../../deployments/aliyun/auth58-evidence-service-policy.json)，在控制台脚本编辑器中把 `<UPLOAD_ROLE_ARN>` 替换为上一步实际复制的完整 ARN；策略名 `Auth58EvidenceServicePolicy`。模板占位符必须替换后才可提交。
2. 创建角色 `Auth58EvidenceServiceRole`；信任主体选择“云服务”，服务选择“云服务器 ECS”，只绑定 `Auth58EvidenceServicePolicy`。上传角色与服务角色分开。
3. ECS → 杭州 → Auth58 实例详情 → 全部操作 → 实例设置 → 授予／收回 RAM 角色，授予 `Auth58EvidenceServiceRole`。如果实例已经绑定其他角色，先保留并记录现有角色，再由部署者核对其用途，不直接覆盖。
4. 完成后提供：网页 Bucket 是否已创建、CORS 是否已保存、上传角色 ARN、服务角色是否已授予 Auth58。这批使用实例临时凭证，不创建 RAM 用户或长期 AccessKey。

实例角色的创建和控制台入口依据 [阿里云 ECS 实例 RAM 角色文档](https://www.alibabacloud.com/help/zh/ecs/user-guide/attach-an-instance-ram-role-to-an-ecs-instance)。后续部署必须实测 Evidence 容器能够获取并刷新实例临时凭证，同时约束其他应用容器对实例元数据的访问；控制台授予角色本身不构成该证据。

#### 7.1.5 部署者后续工作与当前限制

控制台准备完成后，部署者需完成 D1 OSS 配置注入（当前 Compose 的 `STORAGE_PROVIDER` 固定为 `local`）、实例身份和对象 Endpoint 适配、存量 Local 对象保留／迁移方案，再分批执行直传、complete 摘要复核、私有下载、取消清理、权限拒绝与恢复。不能直接切换 Provider 后让已有 Local File 引用失效。

2026-10-01 Auth58 隔离容器批次已实际通过 ECS SDK 身份、单对象 STS、64 KiB／21 MiB 字节链、内网 complete、签名下载和独立空库恢复；同时补了仅运行时生效的容器 IMDS 定向隔离。主栈仍为 Local，正式公网浏览器、存量兼容、重启持久化和 OSS 字节误删恢复尚未通过。证据和可复现入口见 [Auth58 OSS 适配报告](../reports/auth58-evidence-oss-adapter-validation-20261001.md)，不以该后端批次替代上述切换门禁。

网页静态交付单独实施版本目录、发布身份、读取入口、缓存和回退；当前不需要用户先增加 `oatestapi` 或改动既有 `api.antuinfo.cn`，新增域名由已审定的交付路径决定。

当前 Region、Bucket、上传角色 ARN、上传角色最小权限、本地验收 RAM 用户身份、该用户携带单 Object Key 会话策略扮演上传角色的 STS 链、本机环境、Bucket 安全设置、CORS 和真实浏览器主链均已确认。

已完成：小文件上传下载、150 MiB multipart、进度、取消、取消会话清理、Complete 摘要复核、同一 STS 越 Object Key 拒绝、短期下载 URL 和匿名访问拒绝。待继续：人为网络中断重试、STS／签名 URL 到期拒绝、真实公网入口和生产监控／恢复。验收产生的两个已确认测试对象当前保留为证据；完成全部证据留存后再按明确 File ID 清理。RAM AccessKey 在转为 ECS 实例角色后禁用并删除。

## 8. 2026-09-22 首次真实验收证据

- Evidence standalone 容器健康，`/health/ready` 明确返回 `storage_provider=aliyun-oss`；
- File Transfer Core 重新通过 4 个测试文件、15 项测试、Evidence v1 契约守卫和声明构建；浏览器验收页通过严格 TypeScript 与 Vite 生产构建；
- 1 MiB 合成文件形成可用 Version，服务端 SHA-256 为 `30e14955ebf1352266dc2ff8067e68104607e750abb9d3b36582b8af909fcb58`；短期签名下载的实际字节摘要一致；
- 150 MiB 合成零字节文件触发 multipart，上传进度可见，服务端重读后 SHA-256 为 `12ba578486fc98e3d601b534901ce1e0cb2743f02de2adbba06a4ab860f85415`，与独立生成的同尺寸零字节流摘要一致；
- 另一次 150 MiB multipart 在出现进度后取消，Evidence 记录 `aborted`；cleanup 返回 `examined=1`、`cleaned=1`、`failed=0`、`protected=0`；
- 独立负例脚本使用本次临时 STS 故意写入相邻错误 Object Key，OSS 返回 `403 AccessDenied`；随后取消并清理该测试会话，最终数据库为两个 `completed`、两个 `cleaned`；脚本全程不输出临时凭证；
- 对已确认 Object Key 发起未签名 GET 返回 `403`，证明 CORS 没有把私有对象变成匿名可读；
- 页面和文档没有输出 RAM AccessKey、模板令牌、STS Secret 或短期签名 URL。浏览器只有 `ali-oss` 关于未配置 STS 自动刷新回调的非致命提示；当前固定、单 Object Key、15 分钟会话均在授权期内完成，不能为消除提示向浏览器下发长期凭证。
- 端点职责拆分后，使用仍只含旧 `OSS_ENDPOINT` 的现有 `.env.oss-validation` 重建容器成功，证明兼容回退有效；新建后取消的探针会话只向浏览器返回 `https://oss-cn-beijing.aliyuncs.com`，cleanup 返回 `examined=1`、`cleaned=1`、`failed=0`；再为原 1 MiB 对象签发访问授权，URL Host 为公开 Bucket 域名且实际下载 SHA-256 仍为 `30e14955ebf1352266dc2ff8067e68104607e750abb9d3b36582b8af909fcb58`。本机无法证明 ECS 内网链路，`OSS_SERVICE_ENDPOINT=*-internal.aliyuncs.com` 必须在同地域 ECS 上另行验收。

尚未通过的项目必须继续保持未完成：人为网络中断后的 multipart／complete 重试、真实到期拒绝、Contract 迁移、真实 IAM 与公网 `Nginx → Gateway → Evidence` 链路。

## 9. 官方依据

- [OSS 地域和 Endpoint](https://help.aliyun.com/en/oss/user-guide/regions-and-endpoints)
- [创建 Bucket 与默认私有访问](https://help.aliyun.com/en/oss/user-guide/create-a-bucket-4)
- [阻止公共访问](https://help.aliyun.com/en/oss/user-guide/block-public-access)
- [OSS 跨域 CORS](https://help.aliyun.com/en/oss/user-guide/configure-cross-origin-resource-sharing)
- [Multipart Upload 权限映射](https://help.aliyun.com/en/oss/user-guide/multipart-upload)
- [服务端签发临时凭证后直传 OSS](https://help.aliyun.com/en/oss/user-guide/obtain-signature-information-from-the-server-and-upload-data-to-oss)
- [STS 临时访问凭证](https://help.aliyun.com/en/oss/developer-reference/use-temporary-access-credentials-provided-by-sts-to-access-oss)
- [创建 RAM 自定义权限策略](https://help.aliyun.com/zh/ram/create-a-custom-policy)
- [RAM 角色、用户、信任策略与临时身份](https://help.aliyun.com/zh/ram/user-guide/ram-role-overview)
- [扮演 RAM 角色及双向授权条件](https://help.aliyun.com/zh/ram/user-guide/assume-a-ram-role)
- [ECS 实例 RAM 角色](https://help.aliyun.com/en/ecs/user-guide/attach-an-instance-ram-role-to-an-ecs-instance)
