# Auth58 `oatest.antuinfo.cn` HTTPS 入口记录

日期：2026-10-01。范围：公司借用的 Auth58 D1 测试环境；不涉及 `antuinfo.cn` 根域名和已有 `api.antuinfo.cn`。

## 当前状态

- DNS：`oatest.antuinfo.cn` A 记录指向 `47.98.156.166`。公网 `https://oatest.antuinfo.cn/workbench/` 返回 200，TLS 证书校验通过；公网 HTTP 返回 308 并跳转同名 HTTPS 地址。
- 证书：主机名匹配，证书和私钥匹配，实际有效期为 2026-10-01 00:00:00 UTC 至 2026-12-29 23:59:59 UTC。应在 2026-11-29 前启动续签，不以控制台显示的额度/订阅到期日代替证书到期日。
- Nginx：公网只监听 80/443；错误 Host 返回 421。匿名访问 `/project/` 返回 303 到 `/workbench/?return_to=%2Fproject%2F`。Gateway、IAM、业务应用和数据库没有发布公网宿主端口。
- D1：主栈 `d1 up` 完成，Nginx、Gateway、IAM、Workbench 均健康，`d1 check` 对所有启用应用通过。公网连通与匿名保护已验证，**未在本次执行已登录的完整 OA 用户旅程和表单写入验收**。
- 文件：Evidence Service 仍使用 `local` provider。新建的私有 OSS Bucket `oatest-auth58-files-1` 尚未接入 D1，不得把 HTTPS 入口通过视为 OSS 字节链通过。
- 2026-10-01 文件域名只读复核：`oatestfiles.antuinfo.cn` CNAME 与 HTTPS 均连通，证书校验成功；匿名 OSS 请求返回 `403 AccessDenied`。网站 Origin 的 PUT 上传预检仍返回 `403`，需按 [Auth58 控制台准备](./aliyun-oss-first-validation.md#71-auth58-公网环境控制台准备2026-10-01) 补齐 CORS 与目标账号 RAM 角色；没有在本次写入对象或切换 Provider。
- 同日 CORS 保存后复测：PUT 上传预检返回 `200`，精确 Origin 为 `https://oatest.antuinfo.cn`，五种 Methods、实际请求 Headers、`ETag`／`x-oss-request-id`、`Max-Age: 600` 与 `Vary: Origin` 均返回。用户已报告两个 RAM 角色和策略完成；随后以既有 `ecs-user` SSH 连接 Auth58 检查，主栈 17 个长期容器均 healthy，IMDSv2 Token 获取成功，但角色名称路径返回 `404`，实例角色尚未取得。仍需把 `Auth58EvidenceServiceRole` 授予该 ECS，再验真实 STS／OSS 调用；未重建服务、修改业务记录或输出云凭证。
- 用户完成角色授予后再次实测：Auth58 主机 IMDSv2 角色名称路径返回 `200` 和 `Auth58EvidenceServiceRole`；临时身份读取成功，使用该身份对 `acs:ram::1169734041157847:role/auth58evidenceuploadrole` 的真实 `AssumeRole` 调用成功，时长 900 秒、会话 Policy 限定到 `tenants/preflight-no-write/permission-check` 单对象及三项上传动作。用服务身份从杭州 OSS internal Endpoint 读取随机不存在的测试 Key 返回 `404 NoSuchKey`，证明本次读取已通过授权并连通。该批没有写入 OSS 对象，所有凭证仅存在远端进程内存，未输出值或签名请求地址。
- Evidence 主进程仍为 `ready / storage_provider=local`，对象目录有 10 个文件；尚未部署 D1 OSS Provider，也未完成浏览器字节上传／完整性确认／授权下载。源码审计发现运行时与备份工具当前都只接受单一 Provider，因此切换前必须完成存量 Local 引用兼容和备份恢复设计，不能直接改环境变量使旧文件不可读。主机 IMDS 成功不等于 Evidence 容器凭据链已通过；容器侧 SDK 获取／刷新与其他应用的元数据隔离仍须单独验证。
- 随后独立云端适配批次已通过：现有 Evidence 镜像在专用出口网络取得 ECS 身份和正式 IAM Credential；64 KiB 单对象、21 MiB 串行 multipart、900 秒单对象 STS 权限拒绝、内网 complete 摘要、幂等、签名下载、Range 和 AES256 均通过。独立库冷备份／空库恢复保持 File ID、Version、完整行和摘要，恢复后实际下载也通过；ZIP 已复制到 Mac 并核对 checksum。主库 10 个 Local 文件与引用保持，主栈 17/17 healthy；这不是正式公网 Evidence 路由或真人浏览器验收。
- 容器网络审计发现主 Evidence 的内部网络无法取得 IMDS 身份，而有出口的 Nginx／Gateway 可读取角色名称路径。已应用仅针对 Docker 转发目标 `100.100.100.200/32` 的 Evidence IMDS guard：四个非 Evidence 应用复测不可读，独立 Evidence 恢复容器正向 SDK／下载仍通过。仅运行时生效，未验收主机重启持久化；不能开放整个 `app` 网络或给业务应用长期 AK。详细证据、隔离库／备份位置和切换门禁见 [云端适配与恢复报告](../reports/auth58-evidence-oss-adapter-validation-20261001.md)。

## 配置与回退点

2026-10-02 运维更新：当前 Main `.11` 的入口必须使用 `/opt/omsbizfoundry/acceptance/oa-public-release-20261002k/d1-active`，显式保留 IAM override 和 Project→Business Engagement 引用端点；不要用旧 a wrapper 或裸旧 Compose 执行 up。用户补 SSH 安全组后，现有 `ecs-user` 密钥连接已成功，不需要新增账号／密码或依赖 Mac 前台云 Shell。本次方式／适用性真实公网链及失败历史见[10 月 2 日报告](../reports/oa-bidding-command-validation-20261002.md)；不把下方 10 月 1 日观察改写成当时已通过。Evidence Main 仍为 local，Web OSS 尚未切换。

- 证书和私钥只位于 Auth58 的 `/opt/omsbizfoundry/secrets/tls/`，目录权限 `0700`、文件权限 `0600`，不进入 Git 或镜像。证书源是用户提供的 Nginx ZIP；未记录或输出私钥正文。
- 当前环境：`/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.https.env`，权限 `0600`。入口设为 `D1_INGRESS_MODE=https`、`D1_PUBLIC_HOST=oatest.antuinfo.cn`、`PUBLIC_BIND=0.0.0.0`、`PUBLIC_HTTP_PORT=80`、`PUBLIC_HTTPS_PORT=443`、`COOKIE_SECURE=true`，IAM Issuer 和 Evidence 公网基址均使用同一 HTTPS 域名。
- 原回环 HTTP 环境文件 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.env` 保留未修改；原 Nginx 渲染配置保存为 `.runtime/nginx.conf.pre-oatest-https-20261001`。业务备份 `/var/backups/omsbizfoundry/20261001T012550Z` 已生成并通过 SHA-256 清单校验。
- 如仅回退入口配置，用原环境文件对同一 release 执行 `d1 up`，会重新渲染回环 HTTP 并保留数据卷；不要运行 `d1 down --volumes` 或 `restore` 作为普通入口回退。原先 Mac 上的 `127.0.0.1:18090` SSH 隧道依赖远端 `127.0.0.1:18000`，HTTPS 切换后不再是有效入口。

## 后续门禁

1. 通过正式 IAM 测试账号从公网完成登录、Secure Session、Project 新建与复读、相邻角色拒绝、文件上传下载等真实用户旅程。
2. 将 OSS Provider 接入 D1 时单独验证短期 STS、浏览器直传、完成校验和签名访问；保持 Bucket 私有，并确保 ECS 与 Bucket 都在杭州地域。
3. OA 完整体仍按既有验证报告标记为 `insufficient-evidence`，不得因为公网入口成功而提升为业务验收通过。
