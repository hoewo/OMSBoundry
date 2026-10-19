# 阿里云 ECS D1 宿主机初始化

日期：2026-09-22
状态：宿主机基线已形成可重复自动化入口并在当前 ECS 验证；账号专属 Docker Hub 加速器仅作为辅助；私有 ACR 的 `v0.1.0-test.1` 首次发布和 ECS 摘要拉取已完成，业务启动仍等待 DNS 与 HTTPS 入口完成

## 1. A→B

**一台刚创建或刚重装的 ECS 还不能安全、稳定地承载完整组合 → 得到只接受密钥登录、具备受控 Docker 运行时、有限日志、内存缓冲和可重复验证入口的 D1 宿主机。**

本文只准备宿主机，不部署 BizFoundry 业务代码，不创建 `.env`，不上传证书、数据库密码、IAM 私钥或 OSS AccessKey。D1 应用部署继续使用 [`deployments/d1`](../../deployments/d1/) 和 [D1 Compact 运行手册](./d1-compact.md)。

## 自动化入口（推荐）

仓库提供两个入口：

- `deployments/d1/ops/bootstrap-host`：在目标 Ubuntu 主机内执行 `plan`、`apply`、`check` 或本地 `self-test`；
- `deployments/d1/ops/bootstrap-remote-host`：从受控操作者设备经 SSH 把同一个脚本流式送到目标主机，不在 ECS 留下 AccessKey、私钥或初始化脚本副本。

第一次仍需人工连接并核对 ECS 主机指纹；确认指纹进入 `known_hosts` 后，从仓库根目录先验证脚本：

```bash
make d1-host-bootstrap-test
```

查看计划和只读检查：

```bash
D1_SSH_TARGET=ecs-user@<ECS_PUBLIC_IP> \
D1_SSH_KEY=/absolute/path/to/private-key \
D1_ACR_MIRROR=https://<ACCOUNT_ACCELERATOR>.mirror.aliyuncs.com \
  deployments/d1/ops/bootstrap-remote-host plan

D1_SSH_TARGET=ecs-user@<ECS_PUBLIC_IP> \
D1_SSH_KEY=/absolute/path/to/private-key \
  deployments/d1/ops/bootstrap-remote-host check
```

只有在第二个密钥 SSH 会话已经实际登录成功后，才允许应用基线：

```bash
D1_SSH_TARGET=ecs-user@<ECS_PUBLIC_IP> \
D1_SSH_KEY=/absolute/path/to/private-key \
D1_ACR_MIRROR=https://<ACCOUNT_ACCELERATOR>.mirror.aliyuncs.com \
D1_CONFIRM_SSH_KEY_LOGIN=yes \
  deployments/d1/ops/bootstrap-remote-host apply
```

`apply` 默认执行系统升级，可用 `D1_HOST_APT_UPGRADE=no` 在重复验证时跳过升级；它仍会补齐软件包、Docker、Swap、目录与 SSH 基线，并在末尾运行检查。脚本会合并而不是覆盖已有 Docker JSON，重复执行不追加重复的 Swap 挂载项。

自动化边界是“ECS 已经创建并可以使用密钥登录”之后的主机内部配置。ECS 购买／换镜像、密钥绑定、安全组、DNS、证书、ACR 仓库和应用秘密仍属于云控制面或人工确认事项；下文各人工步骤同时作为首次准备说明、故障排查和审计兜底，不能被脚本成功输出取代。

2026-09-22 已在当前 Ubuntu 22.04 ECS 完成脚本自测、远程只读检查、一次关闭 APT 升级的重复 `apply` 和再次检查；17 项宿主机门禁全部通过，仓库未记录公网 IP、私钥路径或账号专属加速地址。

## 2. 当前验证配置

本节记录的是个人账号下当前可重建的开发验证 ECS，不是公司生产资源规格或所有权结论。未来公司购买服务器时，应在公司控制的账号内重新执行本手册和 D1 验收；仓库脚本不得依赖当前实例 ID、公网 IP 或个人账号资源。

| 项目 | 当前值 | 说明 |
| --- | --- | --- |
| 地域 | 华北 2（北京） | 与当前 OSS Bucket 同地域，Evidence 后端可使用 OSS 内网 Endpoint |
| 实例规格 | 2 vCPU、2 GiB | 实验与最低 D1 档；不是容量结论 |
| 系统盘 | ESSD Entry 40 GiB | 当前足够；必须限制容器日志并监控剩余空间 |
| 公网 | 固定 3 Mbps、公网 IPv4 | 复用既有套餐；只有 Nginx 可以发布业务端口 |
| 操作系统 | Ubuntu 22.04 64 位 | x86_64 公共镜像 |
| 登录用户 | `ecs-user` | 使用 `sudo`，不允许 root 直接 SSH 登录 |
| SSH | 独立 Ed25519 密钥 | 阿里云保存公钥，操作者设备保存私钥 |
| Docker | Engine 29.1.3 | 来自 Ubuntu Jammy 软件源 |
| Compose | v2.40.3 | 使用 `docker compose`，不使用旧版 `docker-compose` |
| Swap | 2 GiB | `vm.swappiness=10`；只作为内存尖峰缓冲，不替代扩容 |

公网 IP、实例 ID、私钥路径和密钥材料不进入仓库。资源规格只记录当前实验事实，不升级为所有客户的固定要求。

## 3. 购买与镜像选择

当前使用已有包年包月实例，没有保留重复购买的短期实例。新环境按实际价格和周期选择，但 D1 最低要求保持：

1. 选择与 OSS 相同的地域；
2. 分配公网 IPv4；
3. 使用 Ubuntu 22.04 64 位公共镜像；
4. 不预装宝塔、LNMP、Docker 应用镜像或其他管理面板；
5. 2 vCPU、2 GiB、40 GiB 只用于实验和最低运行门禁；
6. 自动续费只有在确认续费价格和保留责任后再开启；
7. 中国内地网站正式对公网提供服务前另行完成备案，不用实验可达性代替合规确认。

如果实例已有数据，更换操作系统前必须创建并验证备份。更换系统盘会永久清除原系统盘数据；只有确认目标是空机器时才执行。

## 4. 创建并导入 SSH 密钥对

### 4.1 在受控操作者设备生成

```bash
umask 077
ssh-keygen -t ed25519 \
  -f "$HOME/.ssh/omsbizfoundry_ecs_ed25519" \
  -C 'omsbizfoundry-ecs'
```

形成两个文件：

- 无 `.pub` 后缀的是私钥，只留在操作者设备，不发送、不上传、不写入仓库；
- `.pub` 文件是公钥，可以导入阿里云 ECS 密钥对。

核对本地指纹：

```bash
ssh-keygen -lf "$HOME/.ssh/omsbizfoundry_ecs_ed25519.pub"
ssh-keygen -E md5 -lf "$HOME/.ssh/omsbizfoundry_ecs_ed25519.pub"
```

### 4.2 在阿里云控制台导入

路径：`云服务器 ECS → 网络与安全 → 密钥对 → 创建密钥对`。

1. 密钥对名称使用 `OMSBizFoundry-ECS`；
2. 创建类型选择“导入已有密钥对”；
3. 公钥内容粘贴 `.pub` 文件的完整一行；
4. 标签使用 `Project=OMSBizFoundry`；
5. 创建后用控制台显示的指纹与本地 `ssh-keygen` 输出逐字核对。

标签只用于资源分类，不参与 SSH 认证。RAM AccessKey、OSS STS 角色和 ECS SSH 密钥是三种不同凭据，不得混用。

## 5. 更换操作系统

路径：`ECS 实例详情 → 操作系统 → 更换`。

已验证选择：

1. 镜像：Ubuntu 22.04 64 位；
2. 免费安全加固：保持启用；
3. 安全设置：密钥对；
4. 登录用户：`ecs-user`；
5. 密钥对：`OMSBizFoundry-ECS`；
6. 系统盘：保持 ESSD Entry 40 GiB；
7. 系统设备名：保持平台生成值；
8. 云盘加密：当前个人实验环境不启用。生产合规环境必须单独评审 KMS、恢复和迁移责任，不能直接沿用本结论。

确认系统盘为空后，选择“停止实例并继续”。实例重新进入“运行中”且控制台显示 Ubuntu 22.04 后再连接。

## 6. 首次 SSH 检查

```bash
ssh \
  -i "$HOME/.ssh/omsbizfoundry_ecs_ed25519" \
  -o IdentitiesOnly=yes \
  -o StrictHostKeyChecking=accept-new \
  ecs-user@<ECS_PUBLIC_IP>
```

首次连接只读检查：

```bash
whoami
hostname
. /etc/os-release && printf '%s %s\n' "$NAME" "$VERSION_ID"
uname -r
free -h
df -h /
sudo cloud-init status --wait
```

通过条件：用户为 `ecs-user`、系统为 Ubuntu 22.04、系统盘约 40 GiB、`cloud-init` 状态为 `done`。

## 7. 系统更新与基础工具

```bash
sudo env DEBIAN_FRONTEND=noninteractive apt-get update
sudo env DEBIAN_FRONTEND=noninteractive apt-get -y upgrade
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ca-certificates curl git jq openssl rsync unzip
```

安装结束后检查 `/var/run/reboot-required`。存在时，在没有业务容器的初始化阶段完成受控重启；已有业务时必须进入维护窗口。

## 8. 安装 Docker Engine 与 Compose v2

中国内地 ECS 直连 Docker 官方 APT 和 Docker Hub 可能发生 TLS 重置、DNS 异常或超时。当前实例从 Ubuntu Jammy 软件源安装经过签名的软件包：

```bash
sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y \
  docker.io docker-compose-v2
sudo systemctl enable docker
sudo systemctl start docker
sudo usermod -aG docker ecs-user
```

退出并重新建立 SSH 会话，让 `docker` 用户组生效。不要通过来源不明的 `curl | sh` 或公共镜像代理绕过网络问题。

## 9. Docker 日志与重启行为

当前 `/etc/docker/daemon.json` 的基线为：

```json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "live-restore": true
}
```

写入后执行：

```bash
sudo systemctl restart docker
docker version
docker compose version
```

以后增加 `registry-mirrors` 时必须合并 JSON，不能覆盖日志限制和 `live-restore`。

## 10. 配置 2 GiB Swap

```bash
if [ ! -e /swapfile ]; then
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
fi
sudo swapon /swapfile
if ! grep -q '^/swapfile[[:space:]]' /etc/fstab; then
  printf '%s\n' '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
fi
printf '%s\n' 'vm.swappiness=10' | \
  sudo tee /etc/sysctl.d/99-omsbizfoundry.conf
sudo sysctl --system
```

上述命令会在 `/swapfile` 已存在或 `/etc/fstab` 已有对应记录时跳过创建和追加，可重复执行。验证：

```bash
swapon --show
free -h
```

Swap 只缓冲短时构建或启动峰值；持续使用 Swap、OOM 或容器反复重启时，应降低同时构建并发或升级内存，不能把 Swap 当作正常容量。

## 11. SSH 加固

在已验证密钥新会话可以登录之后，创建 `/etc/ssh/sshd_config.d/99-omsbizfoundry.conf`：

```text
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
```

先校验再重载：

```bash
sudo sshd -t
sudo systemctl reload ssh
```

保持原会话不退出，另开终端重新使用 Ed25519 私钥登录。只有新会话成功后才关闭原会话。

## 12. 部署与备份目录

```bash
sudo install -d -o ecs-user -g ecs-user -m 0750 /opt/omsbizfoundry
sudo install -d -o ecs-user -g ecs-user -m 0700 /var/backups/omsbizfoundry
```

`/var/backups/omsbizfoundry` 当前仍位于同一系统盘，只是备份生成暂存路径。D1 正式运行前，备份必须自动复制到 ECS 之外的受控加密存储，并实际完成空目标恢复演练。

## 13. 配置 ACR 镜像加速

直连 `registry-1.docker.io` 在该 ECS 上出现异常 DNS 结果和 HTTPS 超时。账号本人进入：

`容器镜像服务 ACR → 镜像工具 → 镜像加速器`

复制账号专属的 `https://<account-id>.mirror.aliyuncs.com` 地址。该地址可以用于配置，不是 AccessKey 或登录密码。

把地址合并到 `/etc/docker/daemon.json`：

```json
{
  "registry-mirrors": [
    "https://<ACCOUNT_ACCELERATOR>.mirror.aliyuncs.com"
  ],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "live-restore": true
}
```

然后执行：

```bash
sudo systemctl restart docker
docker info
docker run --rm hello-world
docker pull postgres:18.6-alpine
docker pull nginx:1.28-alpine
```

2026-09-22 实测结果：

- 账号专属加速地址已与日志限制、`live-restore` 一起写入配置，Docker 配置校验和重启通过；
- `hello-world` 成功拉取并运行，证明 ECS → ACR 加速器 → Docker 的基础链路已经畅通；
- `postgres:18.6-alpine`、`postgres:18.4-alpine`、`postgres:18-alpine` 和 `nginx:1.28-alpine` 均返回 `not found`；同名目标镜像可在开发机直接从 Docker Hub 获取，因此失败不是项目标签拼写或 Docker 守护进程故障，而是加速器没有相应制品；
- 阿里云已明确说明个人镜像加速器停止同步最新镜像，所以不通过随意降低基础镜像版本规避门禁。

加速器只保留为开发连通性辅助。2026-09-23 已通过受控发布流程把固定基础镜像和应用镜像推入华北 2 私有 ACR，并在同地域 ECS 按摘要拉取 9 个 D1 运行镜像；运行时不再依赖加速器是否同步特定 Docker Hub 标签。详细操作与证据见 [ACR 私有镜像供应](./acr-image-supply.md)。若以后改用 ACR 企业版，可评估其海外制品订阅，但当前实验阶段不把付费企业版作为前置条件。

## 14. 安全组边界

在部署业务前复核 ECS 安全组：

| 端口 | 来源 | 用途 |
| --- | --- | --- |
| TCP 22 | 当前可信运维出口 IP `/32` | SSH；不要开放 `0.0.0.0/0` |
| TCP 80 | `0.0.0.0/0`、`::/0`（如启用 IPv6） | 仅用于 HTTP 到 HTTPS 跳转和证书验证 |
| TCP 443 | `0.0.0.0/0`、`::/0`（如启用 IPv6） | Nginx 唯一公网业务入口 |

不得向公网开放 PostgreSQL、Gateway、IAM、Workbench、Contract、Money、People 或 Evidence 的容器端口。Docker 发布端口可能绕过 UFW 规则，因此公网边界以阿里云安全组和 Compose 仅发布 Nginx 两层共同控制。

## 15. 重启后验收

```bash
id
systemctl is-active docker
docker version
docker compose version
swapon --show
free -h
df -h /
sudo sshd -T | grep -E \
  '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin) '
```

已验证结果：

- `ecs-user` 属于 `docker` 组；
- Docker 为 `active`；
- 2 GiB Swap 在重启后自动挂载；
- `passwordauthentication no`；
- `kbdinteractiveauthentication no`；
- `permitrootlogin no`；
- 当前系统盘仍有约 32 GiB 可用空间。

宿主机基线、私有 ACR 首次发布以及 ECS 摘要拉取均已通过；拉取后已执行 `docker logout`，Registry 登录缓存已清除，9 个本地镜像仍保留。当前阶段状态为“宿主机与镜像供应链已完成，业务部署未完成”：在 DNS、HTTPS、`d1 up`、`check`、备份和空目标恢复通过前，不能宣称 D1 云端部署已完成。

## 16. 凭据与记录规则

- 不把 SSH 私钥、RAM AccessKey、STS Token、Cookie、证书私钥、数据库密码或 `.env` 写入仓库、聊天、命令历史和工单；
- SSH 公钥、密钥指纹、ACR 加速地址和域名不是认证秘密，但仍只记录完成工作所需内容；
- 每次变更记录时间、操作者、目标实例、变更内容和验证结果，不保存完整命令输出或敏感日志；
- 业务部署完成后，宿主机基线验证不能替代 D1 的 `up`、`check`、备份和空目标恢复验收。

## 官方依据

- [Docker Engine on Ubuntu](https://docs.docker.com/engine/install/ubuntu/)
- [Docker Linux 安装后配置](https://docs.docker.com/engine/install/linux-postinstall/)
- [阿里云 ECS 更换操作系统](https://help.aliyun.com/zh/ecs/user-guide/replace-the-operating-system-of-an-instance-1)
- [阿里云 ACR 官方镜像加速](https://help.aliyun.com/zh/acr/user-guide/accelerate-the-pulls-of-docker-official-images)
- [阿里云 ACR 个人版镜像推送与拉取](https://help.aliyun.com/zh/acr/user-guide/use-a-container-registry-personal-edition-instance-to-push-and-pull-images)
- [阿里云 ACR 企业版制品订阅](https://help.aliyun.com/zh/acr/user-guide/subscribe-to-image-tags)
- [阿里云 ECS 公网带宽](https://help.aliyun.com/zh/ecs/public-bandwidth)
