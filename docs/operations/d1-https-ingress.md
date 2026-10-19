# D1 公网 HTTPS 入口实施方案

日期：2026-09-23  
状态：D1 整栈、HTTPS 与业务链已在 ECS 运行并通过主机侧验收；中国大陆外部公网访问被 ICP 备案网关阻断，尚未对外上线  
适用范围：个人阿里云验证环境中的 `app.xidou.me`；未来迁移到公司账号时沿用同一结构，但重新签发证书并替换资源绑定。

## 1. 当前结论

- 阿里云权威 DNS 已解析到 ECS，安全组边界已经收紧；D1 的 9 个长期容器已从摘要镜像启动并全部健康，`d1 check`、`status` 与业务验收通过；
- ECS 自身经公网域名访问时，证书、统一登录、Gateway／IAM 路由、各启用应用读写、Secure Session 和 Local Evidence 字节链均通过；
- 中国大陆外部来源现在被阿里云备案网关拦截：HTTP 由 `Server: Beaver` 返回 `403 Non-compliance ICP Filing`，HTTPS 在 ClientHello 后、到达 Nginx 前被复位。此前短暂取得的 308 和有效证书响应不能覆盖当前阻断状态，因此仍不能表述为已对外上线；
- 已签发的证书包包含 `app.xidou.me.pem` 与 `app.xidou.me.key`；证书覆盖 `app.xidou.me` 和 `www.app.xidou.me`，有效期至 2026-12-20；
- 当前基础 Compose 只把宿主端口映射到 Nginx 容器的明文 `8080`。即使把宿主端口写成 `443`，也不会自动获得 HTTPS；
- 基础 Compose 继续服务本地 HTTP 开发；生产必须显式设置 `D1_INGRESS_MODE=https`，由 `compose.https.yaml` 覆盖端口和证书挂载，不能只把 `PUBLIC_PORT` 改成 443。
- 2026-09-23 的 ECS 实机检查曾发现：目标主机是 `linux/amd64`，7 个应用镜像是 `linux/amd64`，但摘要锁定的 `base-nginx` 与 `base-postgres` 误发为 `linux/arm64`，导致 Nginx 启动前检查以 `exec format error` 失败。发布脚本现已显式执行 `docker push --platform linux/amd64` 并校验 Registry 结果；两个基础镜像已重发为新摘要，ECS 上 9 个启用镜像均为 `linux/amd64`，真实证书 `nginx -t` 已通过。

## 2. 目标链路

```text
浏览器
  │ HTTPS :443
  ▼
ECS 安全组
  ▼
Nginx（唯一公网容器）
  ├─ 终止 TLS、出示 app.xidou.me 证书
  ├─ HTTP :80 固定跳转到 https://app.xidou.me
  └─ 反向代理到 Docker 内网 Gateway
          ▼
       Go Gateway
          ▼
      IAM／业务小应用
```

Gateway、IAM、业务应用和 PostgreSQL 不发布宿主机端口。当前 D1 实例使用 Local Provider：文件控制面经 Gateway，上传／下载字节由 Nginx 直接代理到 Evidence，不经过 Gateway。独立 OSS 验收环境已经证明 OSS Provider 的浏览器直传核心链，但本次 D1 尚未切换到 OSS Provider；切换后浏览器字节应通过 HTTPS 直连私有 OSS。

## 3. 实现约定

当前实现采用显式的 HTTPS 入口模式，并保留现有本地 HTTP 开发模式：

1. `compose.https.yaml` 是生产 HTTPS 覆盖文件，由 D1 运维脚本按入口模式选择；
2. HTTPS 模式只为 Nginx 发布：
   - 宿主 `80` → 容器 `8080`，仅作固定域名的 HTTPS 跳转；
   - 宿主 `443` → 容器 `8443`，提供 TLS 业务入口；
3. Nginx 另设不发布到宿主机的容器内健康检查端口，避免健康检查依赖公网 DNS 或绕过证书校验；
4. HTTPS server 使用 `server_name app.xidou.me`，并把 `X-Forwarded-Proto` 明确设为 `https`；
5. 80 端口跳转使用固定目标 `https://app.xidou.me$request_uri`，不使用未经校验的 Host 头拼接跳转地址；
6. TLS 最低版本只允许 TLS 1.2 与 TLS 1.3；HSTS 在 HTTPS 全链验证稳定后再启用；
7. `d1` 在启动前验证证书格式、有效期、域名、未加密私钥、`0400`／`0600` 权限及公私钥匹配关系；
8. 本地开发继续使用回环地址和明文端口，不要求开发机持有生产证书。

建议的运行参数边界：

```text
D1_INGRESS_MODE=https
D1_PUBLIC_HOST=app.xidou.me
D1_TLS_CERT_FILE=/opt/omsbizfoundry/secrets/tls/app.xidou.me.pem
D1_TLS_KEY_FILE=/opt/omsbizfoundry/secrets/tls/app.xidou.me.key
PUBLIC_BIND=0.0.0.0
PUBLIC_HTTP_PORT=80
PUBLIC_HTTPS_PORT=443
COOKIE_SECURE=true
PUBLIC_ISSUER=https://app.xidou.me/auth
EVIDENCE_PUBLIC_BASE_URL=https://app.xidou.me/evidence
```

参数只记录文件路径，不得把证书正文或私钥写入 `.env`。

## 4. 证书落盘规则

ECS 使用独立受限目录：

```text
/opt/omsbizfoundry/secrets/tls/
├── app.xidou.me.pem
└── app.xidou.me.key
```

- 目录不位于 Git 工作树、镜像构建上下文或备份公开目录；
- 当前证书与私钥已复制到上述 ECS 目录，两者权限均为 `0600`；目录和文件不在 Git 或镜像中；
- 两个文件分别只读挂载到 Nginx 容器的 `/etc/nginx/tls/fullchain.pem` 和 `/etc/nginx/tls/privkey.pem`；
- 启动前用 `nginx -t` 验证配置，并在不打印私钥内容的前提下校验证书与私钥匹配；
- 证书更新采用“写入临时文件 → 校验 → 原子替换 → Nginx reload”，失败时保留上一套可用证书；
- 当前证书到期前至少 30 天安排续签，不依赖临近到期的人工记忆。

## 5. 人工与自动化职责

需要账号持有人在阿里云控制台完成：

- 为 `app.xidou.me` 添加 A 记录，指向当前 ECS 公网 IPv4；
- 安全组仅向公网开放 TCP 80、443；22 只允许可信运维来源；
- 域名、证书或 ECS 变更后，把新的非秘密标识提供给部署流程。

可由仓库自动化或受控 SSH 完成：

- 检查 DNS 是否生效；
- 安全复制证书文件、设置权限并校验匹配关系；
- 渲染 Nginx HTTPS 配置与 Compose 覆盖文件；
- 执行配置测试、启动、健康检查和回归验收；
- 记录证书到期时间，但不记录私钥、密码或 Token。

## 6. 上线顺序与验收

- [x] DNS：阿里云权威 DNS 已将 `app.xidou.me` 的 A 记录解析到目标 ECS 公网 IPv4；
- [x] 网络：安全组仅保留公网 80/443、受限来源的 22 和临时诊断 ICMP；全网 SSH 与 RDP 已删除，Gateway、IAM、应用和数据库端口未开放；
- [x] 秘密：证书和私钥已进入受限目录，私钥权限为 `0600`，Git 与镜像中不存在副本；
- [x] 本地静态检查：HTTP／HTTPS Compose 渲染、一次性证书 `nginx -t`、只读挂载、固定域名跳转、公私钥不匹配拒绝均通过，只有 Nginx 发布宿主端口；
- [x] 远端静态检查：生产 `.env` 已切换到 HTTPS，证书域名、公私钥匹配、权限和 Compose 合成通过；原 `.env` 已以 `0600` 权限备份；
- [x] 镜像架构：`base-nginx` 与 `base-postgres` 已按 `linux/amd64` 重发并更新摘要；ECS 上 9 个启用镜像的平台检查和真实证书 `nginx -t` 均通过，随后已清除 ACR 登录缓存；
- [x] 运行：`d1 up`、`check`、`status` 通过，9 个长期容器健康；宿主机只监听 22、80、443，内部应用与 PostgreSQL 没有宿主端口；
- [x] 入口与身份（ECS 侧）：`https://app.xidou.me` 经 Nginx 到 Gateway／IAM，证书域名与有效期正确，TLS 1.0／1.1 被拒绝且 1.2／1.3 可用，匿名业务访问跳转登录，登录成功，Session Cookie 带 `HttpOnly; Secure; SameSite=Lax`，错误 Host 返回 421；
- [x] 路由与业务（ECS 侧）：Workbench、People & Groups、Contract、Money 与 Evidence 控制面均完成真实读写验收；
- [x] 文件（当前 D1 Local Provider）：Evidence 上传、完成、授权及 Range 下载通过，字节经 Nginx 直达 Evidence，不经过 Gateway；
- [x] 运维：Nginx 访问日志未记录 Cookie 或 Authorization；Local Evidence 数据面日志格式不记录短期 URL 查询参数；
- [ ] HTTP／HTTPS（中国大陆外部来源）：完成 ICP 备案或替换合规入口后，重新验证 80 固定跳转、443 完整链、TLS 版本与浏览器可访问性；
- [ ] 文件（D1 OSS Provider）：把已独立验证的 OSS 配置接入 D1 后，重新验证浏览器直传、ECS 内网 Complete／读取／清理和公网短期下载。

所有外部公网检查完成后，才能把 D1 标记为“公网 HTTPS 入口可用”。

## 7. 2026-09-23 ICP 阻断证据与恢复条件

完整业务验收完成后，外部来源再次访问得到以下确定性证据：

- `http://app.xidou.me/` 返回 `403`，响应头为 `Server: Beaver`，页面标题为 `Non-compliance ICP Filing`；
- ECS 抓包可见外部 ClientHello 到达云主机，但连接在 Nginx 处理前被复位；Nginx 访问日志中没有对应请求；
- 同时所有容器继续保持 healthy，ECS 自身访问域名、Gateway 健康检查和完整业务验收仍通过。

因此恢复条件不是重启 Docker 或修改 Nginx，而是账号所有者完成以下二选一事项：

1. 为中国大陆 ECS 上的 `app.xidou.me` 完成 ICP 备案并等待阿里云接入状态生效；或
2. 将公网入口迁移到满足目标用户和合规要求的非中国大陆地域／服务，再重新配置 DNS、证书和安全边界。

恢复后必须从独立外部网络重新执行本节全部 HTTP、TLS、登录与路由验收；ECS 自访问结果不能替代公网证据。
