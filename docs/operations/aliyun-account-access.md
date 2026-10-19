# 阿里云账号协作管理接入

日期：2026-09-22
状态：方案确认；D1 ECS 已完成密钥接入、宿主机初始化和 ACR 加速器连通验证，控制面 OAuth 与受控私有镜像仓库待接入

## A→B

**目前只能通过截图了解阿里云资源 → 建立不共享密码、不暴露主账号 AccessKey、可审计且可随时撤销的协作管理入口。**

## SSH 与阿里云控制面的边界

- SSH 只能登录一台已经购买并运行的 ECS，管理其操作系统、Docker、Nginx、日志和本地文件。
- SSH 不能列出或配置账号下的 OSS、RAM、STS、DNS、账单和其他云资源；这些属于阿里云控制面，必须通过控制台或阿里云 CLI／OpenAPI。
- 不为“查看阿里云账号”单独购买 ECS，也不把 SSH 22 端口向全网开放。
- Evidence 的 STS 上传角色、Evidence 服务执行身份和协作管理身份是三套不同身份，不复用凭证。

## 推荐接入：本机 CLI + OAuth

1. 主账号开启 MFA，且不创建主账号 AccessKey。
2. 创建 RAM 用户 `omsbizfoundry-operator`：允许控制台登录、强制 MFA、不创建永久 AccessKey。
3. 初始只授予 OSS、RAM、ECS 和 DNS 的只读权限；需要创建或修改资源时，再针对明确资源增加最小自定义权限，完成后撤销。
4. macOS 安装官方阿里云 CLI。当前 Homebrew 前置下载异常缓慢，已改用官方 macOS Universal 包安装到 `/Users/crispydog/.local/bin/aliyun`，并链接到 `/opt/homebrew/bin/aliyun`。
5. 由用户本人在终端启动 OAuth 配置并在浏览器完成登录／MFA：

   ```bash
   aliyun configure --mode OAuth --profile omsbizfoundry-operator
   ```

6. 只验证身份，不显示任何秘密：

   ```bash
   aliyun sts get-caller-identity --profile omsbizfoundry-operator
   ```

7. 后续所有命令显式带 `--profile omsbizfoundry-operator`，不依赖默认 profile，避免误操作其他账号。

OAuth 不要求在本机长期保存 AccessKey。不得把账号密码、MFA、AccessKey、STS Token 或 `~/.aliyun/config.json` 内容发送到聊天或写入仓库。

## 操作权限边界

- 默认先执行只读查询：身份、Bucket 配置、CORS、加密、版本控制、生命周期、RAM 角色和 ECS 状态。
- 创建 RAM 角色、变更权限、开放安全组、购买资源、修改 DNS、删除对象／实例或产生费用前，明确列出目标和影响后再执行。
- 不给协作身份 `AdministratorAccess`、主账号凭证、账单支付权限或无边界的 `AliyunOSSFullAccess`。
- CLI Profile 随时可以在 RAM 控制台禁用该用户或撤销权限；OAuth Session 也可重新登录或删除本地 Profile。

## ECS 创建后的 SSH

完整的购买检查、密钥导入、Ubuntu 重装、Docker／Compose、Swap、SSH 加固、ACR 和重启验收步骤见[阿里云 ECS D1 宿主机初始化](./aliyun-ecs-d1-bootstrap.md)。本节只固定 SSH 与阿里云控制面的责任边界。

只有购买 ECS 后才配置 SSH：

1. 使用独立 Ed25519 SSH 密钥，不发送私钥；公钥可以导入 ECS。
2. 安全组的 22 端口只允许当前管理出口 IP 的 `/32`，不使用 `0.0.0.0/0`。
3. 建立非 root 部署用户，以 `sudo` 执行必要操作；禁止密码登录并保留可恢复的控制台入口。
4. SSH 只管理主机，不承担 OSS/RAM 控制面授权；ECS 上的 Evidence 服务使用绑定到实例的 RAM Role 获取临时凭证。

### 当前 D1 宿主机基线

2026-09-22 已在华北 2（北京）的现有包年包月 ECS 上完成以下初始化；公网 IP、私钥和账号凭据不写入仓库：

- 操作系统更换为 Ubuntu 22.04 64 位，使用非 root 的 `ecs-user` 和独立 Ed25519 公钥登录；
- 已确认新 SSH 会话可登录，并关闭 SSH 密码认证、键盘交互认证和 root 登录；
- 安装 Ubuntu 安全更新、`curl`、`git`、`jq`、`openssl`、`rsync` 与 `unzip`；
- 通过 Ubuntu Jammy 软件源安装 Docker Engine `29.1.3` 和 Docker Compose v2 `2.40.3`，Docker 已启用开机启动；
- `/etc/docker/daemon.json` 将容器 `json-file` 日志限制为单文件 10 MiB、最多 3 个文件，并启用 `live-restore`；
- 建立 2 GiB `/swapfile`，设置 `vm.swappiness=10`，重启后已确认自动挂载；
- 建立 `/opt/omsbizfoundry` 受限部署目录和 `/var/backups/omsbizfoundry` 本机暂存目录。后者位于系统盘，不满足离机备份要求，D1 正式运行前仍须配置 ECS 外备份复制；
- 系统更新后的受控重启已完成；重启后 SSH、Docker 与 Swap 均通过检查。

当前没有在宿主机单独安装 Nginx。D1 的 Nginx、Gateway、IAM、Workbench、业务应用和 PostgreSQL 均由 `deployments/d1/compose.yaml` 管理，只有 Nginx 可以发布公网端口。

### Docker Hub 与 ACR 镜像加速

该中国内地 ECS 直连 `registry-1.docker.io` 时发生异常 DNS 解析和 HTTPS 超时，不得用来源不明的公共代理继续部署。账号专属 `https://*.mirror.aliyuncs.com` 地址已经配置；该地址不是账号密码或 AccessKey。

配置时必须在保留现有日志限制和 `live-restore` 的基础上，把专属地址加入 `/etc/docker/daemon.json` 的 `registry-mirrors`，重启 Docker 后依次验证：

```bash
docker info
docker run --rm hello-world
docker pull postgres:18.6-alpine
docker pull nginx:1.28-alpine
```

2026-09-22 验证中，`hello-world` 已成功拉取并运行，但项目所需的 PostgreSQL 与 Nginx 标签均返回 `not found`。阿里云已说明个人镜像加速器停止同步最新镜像，因此它只作为当前开发实验的连通性入口。仓库已提供 Linux AMD64 构建、摘要固定、发布和 D1 拉取自动化；ACR 个人版不支持控制面 API，账号本人仍需在华北 2（北京）用控制台建立实例／密码／命名空间并启用默认私有的仓库自动创建，随后脚本完成首次推送，避免部署依赖 Docker Hub 的即时可达性。

## 当前本机状态

- `aliyun` CLI：已安装，版本 `3.5.1`；
- `~/.aliyun/config.json`：不存在；
- ECS／SSH：D1 ECS 已完成 Ubuntu 22.04、Ed25519 密钥登录、Docker Compose、2 GiB Swap 和 SSH 密码／root 登录关闭验证；安全组 22 来源收敛仍需在控制台复核；
- 容器镜像：Docker 服务与 ACR 加速链路正常，`hello-world` 通过；加速器缺少项目必需镜像，发布自动化已就绪，等待完成个人版控制台前置和首次真实推拉；
- 当前已知资源：`xidou-test-files`，`cn-beijing`，私有 Bucket。

## `xidou.me` DNS 当前决策

- 当前继续使用云解析 DNS 免费版，不购买付费实例；免费版已经支持 A、AAAA、CNAME、MX、TXT 等普通记录，能够解析 `app.xidou.me`、`api.xidou.me` 和 `files.xidou.me`。
- 套餐里的“实例支持绑定 1 个域名”是指一个付费实例默认绑定一个主域名，例如 `xidou.me`，不是只允许一个子域名。
- 免费版单主域最多 100,000 条解析记录；同一主机／线路／记录类型最多 10 个值；最多 10 级子域名，远高于当前需要。
- 当前正式入口优先使用 `app.xidou.me`，浏览器页面与 `/api/...` 同源进入 Nginx／Gateway；暂不为了形式单独创建 `api.xidou.me`，避免无必要的 CORS、Cookie 和证书边界。
- `files.xidou.me` 仍是未来绑定 OSS 的可选域名；真实 OSS 验收继续使用 Bucket 默认 Endpoint。
- 只有在正式商业上线需要 DNS SLA／DNSSEC／海外节点、更低 TTL、攻击防护，或免费版日解析量接近 100,000 次时再升级；开发、内部验收和早期演示不需要升级。

## 官方依据

- [安装阿里云 CLI](https://help.aliyun.com/en/cli/install-update-alibaba-cloud-cli)
- [CLI OAuth／RAM Role／AK 凭据方式](https://help.aliyun.com/en/cli/configure-credentials/)
- [客户端通过服务端 STS 直传 OSS](https://help.aliyun.com/en/oss/user-guide/uploading-objects-to-oss-directly-from-clients/)
- [创建独立 RAM 用户](https://help.aliyun.com/en/ram/user-guide/create-a-ram-user)
- [RAM 最小权限授权](https://help.aliyun.com/en/ram/user-guide/grant-permissions-to-the-ram-user)
- [云解析 DNS 版本和容量比较](https://help.aliyun.com/zh/dns/pubz-example-selection-guidance)
- [Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- [阿里云 ACR 官方镜像加速](https://help.aliyun.com/zh/acr/user-guide/accelerate-the-pulls-of-docker-official-images)
- [阿里云 ACR 个人版镜像推送与拉取](https://help.aliyun.com/zh/acr/user-guide/use-a-container-registry-personal-edition-instance-to-push-and-pull-images)
