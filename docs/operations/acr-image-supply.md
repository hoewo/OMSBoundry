# ACR 私有镜像供应运行手册

日期：2026-09-23  
状态：通用构建、发布和 D1 拉取自动化已实现；`v0.1.0-test.2` 的 9 个运行镜像已在华北 2 ECS 以摘要启动，全部长期容器健康并通过业务验收，ACR 登录缓存随后已清除

## 1. 最小根因

**ECS 直接访问 Docker Hub 不稳定、在目标机现场编译也难以复现 → 把固定平台的基础镜像和应用镜像提前放入同地域私有 ACR，并让 D1 只拉取摘要锁定的制品。**

ACR 在这里不是运行应用的服务器，而是 Docker 镜像仓库：

```text
开发机／构建机
  ├─ 获取并固定 Linux AMD64 基础镜像
  ├─ 构建 BizFoundry 应用镜像
  └─ 推送到私有 ACR
                    ↓
                同地域 ECS
                  └─ 按 image@sha256:digest 拉取并运行
```

账号专属 `https://<ACCOUNT_ACCELERATOR>.mirror.aliyuncs.com` 是 Docker Hub 拉取加速器，不是私有 ACR Registry：它没有项目命名空间、仓库、登录身份或发布版本，不能代替本手册中的受控镜像供应。

## 2. 当前选择与边界

- 当前个人实验环境可以使用 ACR 个人版；个人版免费但没有生产 SLA，不作为客户生产环境的默认承诺；
- ACR 个人版官方不提供 API 操作，因此不能用阿里云 CLI、OpenAPI 或 Terraform 自动创建个人版实例、命名空间和仓库；自动化从控制台完成一次性前置配置后开始；
- ACR 与 ECS 使用华北 2（北京）；跨地域会增加流量、时延和故障面；
- 发布平台固定为 `linux/amd64`，匹配当前 ECS；
- 标签用于人读版本，D1 实际运行必须使用 `name@sha256:<64 位摘要>`；
- 发布脚本不接收、不保存、不打印 Registry 密码；操作者必须先交互执行 `docker login`；
- ACR 密码、Docker 登录配置、应用 `.env` 和生成的镜像清单均不得提交 Git；
- ECS 只拉取镜像，不从 Docker Hub 获取运行前置镜像，也不在目标机编译 Go 应用。

## 3. 环境所有权与迁移边界

当前阿里云账号、ECS 和 ACR 由个人持有，只用于学习、开发、集成和部署链验证。它们必须被视为**可重建的验证场**，不能成为公司正式交付的隐式前提。

当前验证环境与未来公司环境的关系是：

```text
个人开发／验证环境
  ├─ 构建并验证固定版本镜像
  ├─ 验证 D1 初始化、部署、升级、备份恢复和回滚
  └─ 形成摘要锁定的发布清单
                         ↓ 制品提升，不在目标机重编译
公司控制的 Registry 与 ECS
  ├─ 使用公司自己的账号、域名、证书、OSS、RAM 和秘密
  ├─ 按同一版本清单拉取并运行已验证制品
  └─ 重新执行公司环境安装与验收
```

必须遵守以下边界：

- 源码、Compose、初始化脚本和应用配置不得写死个人阿里云账号 ID、ECS 实例 ID、公网 IP、Registry 域名、OSS Bucket、RAM ARN 或域名；这些值只能由目标环境配置注入；
- 个人 ACR 个人版可以跑通首次发布链，但没有生产 SLA，不作为公司生产镜像仓库承诺；正式部署优先使用公司账号控制的 Registry，并重新评估企业版、网络、SLA、凭据轮换和审计；
- 从个人环境迁移时，优先复制／提升已经验证的不可变镜像制品及其摘要清单，不在公司 ECS 现场从源码重新编译；如果必须重新构建，必须作为新的发布制品重新完成验证；
- `.env`、数据库密码、IAM 密钥、OSS AccessKey、Registry 密码和 TLS 私钥不随镜像迁移，也不得进入仓库；它们在公司环境重新生成和配置；
- 业务数据不在 ACR 中。存在需要保留的数据时，必须使用各应用的备份／恢复入口迁移到空目标并校验；不能复制正在运行的数据库目录；
- 个人环境的通过证据证明发布方案可行，不替代公司环境的主机、网络、域名、HTTPS、OSS、IAM、备份恢复和业务健康验收。

因此，本手册当前开通 ACR 仍然有价值：它先把“固定制品 → Registry → 全新 ECS”链路跑通；以后更换为公司账号时，替换的是环境所有权和配置，不改变小应用镜像、Suite Manifest、Deployment Profile 与数据所有权。

跨账号或跨 Registry 不允许手工替换清单中的域名。使用 [D1 双环境制品提升与公网验证](./d1-environment-promotion.md)中的 `promote-images` 入口逐个执行固定平台的 `pull → tag → push`，核对本地 Image ID 与目标 Registry 平台，并以目标 Registry 实际返回的摘要生成新清单。目标 ECS 只接收目标清单和 pull-only D1 发布包。

## 4. 一次性云控制面配置

进入 `容器镜像服务 ACR`，在华北 2（北京）完成：

1. 创建或确认个人版实例，并记录控制台显示的独立登录域名；不要根据镜像加速器地址推测 Registry 域名；
2. 设置实例访问凭证中的固定 Registry 登录密码，保存到密码管理器；
3. 创建命名空间 `omsbizfoundry`；
4. 在命名空间设置中保持“自动创建仓库”开启，并确认“默认仓库类型”为私有。

个人版不支持控制面 API，因此以上三项不能由仓库脚本代办。不要为此给脚本配置主账号 AccessKey，也不要把企业版 API 当作个人版 API 调用。

2026-09-23，账号所有者通过控制台截图确认当前个人验证实例位于华北 2（北京），固定访问凭证已设置，`omsbizfoundry` 命名空间已开启自动创建仓库且默认私有，并提供了不含密码的公网 Registry 登录域名和登录名。同日已完成 `v0.1.0-test.1` 首次真实发布与同地域 ECS 拉取；仓库只记录结果和通用做法，不固化个人 Registry 域名或登录名。

当前版本首次执行 `publish-images publish` 时，推送动作会自动创建以下 17 个私有仓库；发布后在控制台逐项核对：

```text
base-postgres
base-nginx
base-golang
base-alpine
identity-access
contract-ledger
money-flow-ledger
people-groups
party-directory
project-workspace
evidence-service
business-engagement
billing-invoice
production-operations
approval-audit-results
unified-workbench
gateway
```

如果不希望开启自动创建，也可以逐个手工建立上述仓库，但这会增加无意义的控制台操作，不是当前实验环境的推荐路径。个人版使用固定密码，不支持用企业版临时 Token 的流程；生产环境在选择 ACR 企业版后必须重新评估身份、网络、SLA、API 自动化和凭据轮换。

只需把下列非秘密信息记录到部署清单：

```text
Registry 登录域名，例如 crpi-....cn-beijing.personal.cr.aliyuncs.com
Registry 用户名
命名空间 omsbizfoundry
```

不要把 Registry 密码发送到聊天、写进仓库或作为发布脚本参数。

## 5. 本地构建与发布

先运行离线门禁：

```bash
make d1-image-mode-test
```

再交互登录。密码只输入 Docker 提示，不放入命令行：

```bash
docker login --username='<REGISTRY_USERNAME>' <REGISTRY_HOST>
```

先查看确定性计划：

```bash
D1_REGISTRY=<REGISTRY_HOST> \
D1_REGISTRY_NAMESPACE=omsbizfoundry \
D1_RELEASE_VERSION=v0.1.0-test.2 \
  deployments/d1/ops/publish-images plan
```

确认仓库、版本和平台后发布：

```bash
D1_REGISTRY=<REGISTRY_HOST> \
D1_REGISTRY_NAMESPACE=omsbizfoundry \
D1_RELEASE_VERSION=v0.1.0-test.2 \
  deployments/d1/ops/publish-images publish
```

`publish` 会执行：

1. 通过本机 Docker daemon 拉取 PostgreSQL、Nginx、Go 和 Alpine 的 Linux AMD64 基础镜像，再标记并使用 `docker push --platform linux/amd64` 推送到四个 `base-*` 私有仓库；推送后从 Registry 读取配置并拒绝任何非 `linux/amd64` 结果。显式限定推送平台是必要门禁：Apple Silicon 上的 containerd image store 可能在同一标签下保留多平台索引，未限定平台的 push 曾把 arm64 子镜像发布到 ACR；
2. 使用这些受控基础镜像构建并推送十二个应用镜像；每个 Dockerfile 都通过 BuildKit 的 `TARGETOS`／`TARGETARCH` 显式编译 Go 程序，避免 Apple Silicon 构建节点把 ARM64 二进制装进 AMD64 镜像；
3. 从 Registry 或构建元数据读取真实摘要，并再次检查每个应用镜像的 Registry 配置必须为 `linux/amd64`；目标 ECS 首次接收新制品时还要抽取可执行文件检查为 x86-64，镜像配置与二进制架构两层都通过才算兼容；
4. 以 `0600` 权限生成 `deployments/d1/.runtime/published-images-<version>.env`。

生成文件只含镜像坐标，不含登录密码，但仍按部署材料保管，不提交 Git。

## 6. 把发布结果交给 D1

把生成文件中的 `D1_IMAGE_MODE` 和 `D1_*_IMAGE` 项合并到目标环境的 `deployments/d1/.env`，不要留下重复键。所有已启用应用以及 PostgreSQL、IAM、Workbench、Gateway、Nginx 都必须有摘要引用。

在 ECS 上使用同一 Registry 用户交互登录，然后检查将要使用的镜像：

```bash
docker login --username='<REGISTRY_USERNAME>' <REGISTRY_HOST>

D1_ENV_FILE=/opt/omsbizfoundry/deployments/d1/.env \
  /opt/omsbizfoundry/deployments/d1/ops/d1 images
```

只拉取镜像、不启动服务：

```bash
D1_ENV_FILE=/opt/omsbizfoundry/deployments/d1/.env \
  /opt/omsbizfoundry/deployments/d1/ops/d1 prepare-images
```

拉取成功后，如果当前主机不需要持续自动拉取新版本，应退出 Registry 登录：

```bash
docker logout <REGISTRY_HOST>
```

Linux 上直接使用 `docker login` 且没有额外配置 credential helper 时，Docker 会提示凭证以可解码形式保存在 `~/.docker/config.json`。这条 warning 不表示登录或拉取失败，但说明凭证仍驻留在主机上。当前验证环境采用“交互登录 → 拉取摘要镜像 → `docker logout`”的短生命周期方式；退出登录只删除 Registry 登录缓存，不删除已经拉取的镜像。后续获取新版本时重新交互登录，不把密码写入脚本、环境文件或命令行。

确认成功后才进入 D1 安装：

```bash
D1_ENV_FILE=/opt/omsbizfoundry/deployments/d1/.env \
  /opt/omsbizfoundry/deployments/d1/ops/d1 up
```

`pull` 模式会拒绝只有标签、没有摘要的已启用镜像。`d1 up` 会先核对每个启用的摘要引用：全部已存在于本机时直接使用缓存并跳过 Registry，缺少任一镜像时才执行 pull。因此可以安全采用“交互登录 → `prepare-images` → `docker logout` → `up`”，重启已验证版本不要求长期保存 ACR 密码。可选应用关闭时不强求其镜像引用，但重新启用前必须补齐对应摘要。

## 7. 验收

首次真实发布必须保存以下非敏感证据：

- ACR 控制台中当前版本需要的 16 个仓库均为私有，且发布版本存在；
- `published-images-<version>.env` 的十五个运行镜像均含 `@sha256:`；
- ECS 的 `prepare-images` 完成，且运行时没有访问 Docker Hub；
- `docker image inspect <digest-reference>` 能定位相同摘要和 `linux/amd64`；
- `deployments/d1/ops/d1 up`、`check` 和 `status` 通过；
- 退出 `docker login` 后，已下载镜像仍可在 `--pull never` 边界内重建；需要获取新版本时必须重新具备 Registry 拉取权限。

2026-09-23 的当前个人验证环境已经取得以下证据：

- `v0.1.0-test.1` 首次形成 11 个私有仓库；未认证访问仓库 manifest 返回 `401`，证明仓库保持私有；
- 首次 ECS 启动揭示了重要的双层校验边界：应用镜像配置可报告 `linux/amd64`，但使用 ARM64 Go 构建基础镜像时，内部 `/app/server`／`/app/ops` 仍可能是 ARM64，目标机将以 `exec format error` 拒绝运行；
- 发布器随后同时限定基础镜像 push 平台、Go `TARGETOS/TARGETARCH` 和发布后 Registry 平台，并以新版本 `v0.1.0-test.2` 重建 7 个应用；
- `v0.1.0-test.2` 发布清单权限为 `0600`，9 个 D1 运行镜像和 2 个构建基础镜像均通过 Registry `linux/amd64` 检查，本地从 7 个应用镜像抽取的可执行文件均由 `file` 识别为 x86-64；
- ECS `.env` 已更新为 `v0.1.0-test.2` 的不可变摘要并通过 Compose 静态校验；9 个运行镜像均在 ECS 报告 `linux/amd64`，从 7 个应用镜像抽取的程序均为 x86-64，Identity 真实迁移返回当前 schema `000001_identity_access.sql`；验证结束后 ACR 登录缓存为零。
- 在没有保留 ACR 登录凭证的状态下，D1 使用本机已缓存的摘要镜像成功执行 `up`、`check`、`status`；9 个长期容器全部健康，统一登录和全部启用应用的真实读写验收通过。

上述实机数字是未包含 Project、Party、Business Engagement、Billing 与 Production 的 `v0.1.0-test.2` 历史证据。当前发布器已经形成 16 个仓库和 14 个运行摘要；下一版必须在目标 ECS 重新执行平台、程序架构、业务与恢复验收，不能把旧 9 镜像证据自动沿用到新版本。

这证明新制品已经通过构建端、Registry 端、ECS 执行门禁和整栈业务验收。中国大陆公网当前仍受 ICP 备案网关阻断；离机备份与空目标恢复也尚未验收，两者不由镜像供应链结果替代。

## 8. 官方依据

- [ACR 个人版推送与拉取镜像](https://help.aliyun.com/zh/acr/user-guide/use-a-container-registry-personal-edition-instance-to-push-and-pull-images)
- [创建 ACR 个人版实例及 API 限制](https://help.aliyun.com/en/acr/user-guide/create-a-container-registry-personal-edition-instance)
- [ACR 个人版与企业版差异](https://help.aliyun.com/en/acr/product-overview/differences-between-personal-edition-instances-and-enterprise-edition-instances)
- [ACR 命名空间与自动创建私有仓库](https://help.aliyun.com/zh/acr/user-guide/manage-namespaces)
- [ACR 计费说明](https://help.aliyun.com/en/acr/product-overview/billing-description)
- [ACR 个人版独立域名与限制](https://help.aliyun.com/zh/acr/user-guide/individual-edition-instance-independent-domain-name-capacity-limit)
- [Docker Buildx imagetools inspect](https://github.com/docker/buildx/blob/master/docs/reference/buildx_imagetools_inspect.md)
