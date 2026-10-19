# D1 双环境制品提升与公网验证运行手册

日期：2026-09-24
状态：可迁移发布包与跨 Registry 提升入口已实现；个人试验田已运行，公司环境等待授权和目标资源输入

## 1. 目标与边界

ICP备案等待期间不停止交付验证，也不放弃 `xidou.me`。本方案同时保留两套彼此独立的环境：

```text
个人长期试验田                         公司并行验证环境（获批后）
xidou.me / 个人 ECS / 个人 ACR          已备案子域名 / 公司 ECS / 公司 Registry
        │                                         │
        └──── 同一版本、同一 linux/amd64 镜像内容 ──┘
                    各自 DNS、证书、秘密、数据与备份
```

这不是把个人环境“搬走并下线”，而是把已经验证的不可变制品提升到第二套环境，证明部署方案能够脱离个人账号复现。两边可以长期并存：

- 个人环境继续承担学习、集成、升级、破坏性恢复和新版本试验；
- 公司环境只在明确授权的隔离范围内承担公网全链验证；
- 公司验证通过不自动等于生产上线，不导入客户真实数据，不接管现有公司服务；
- 公司环境未获批时，个人环境和仓库内验证继续推进；`xidou.me` 备案完成后再执行其外部公网验收。

本手册落实 [BF-024](../decisions.md)、[BF-027](../decisions.md) 与 [BF-028](../decisions.md)。D1 固定入口仍为：

```text
Browser → Nginx:443 → Go Gateway → IAM / 已启用小系统
```

## 2. 什么随版本走，什么必须按环境重建

### 2.1 随版本提升

- PostgreSQL、IAM、Workbench、Gateway、Nginx 五个必需 `linux/amd64` 镜像，以及目标 Suite 明确选用的应用镜像；完整组合当前最多十七个运行镜像；
- 每个随包镜像在目标 Registry 中的 `name@sha256:digest` 清单；未随包的可选应用必须保持关闭；
- D1 Compose、Nginx 模板、Suite Manifest 渲染器、migration／备份／恢复入口；
- IAM Capability Manifest；
- 外部验收脚本；
- 发布包内外两层 SHA-256 清单和 Release Metadata。

### 2.2 不能跨环境复制

- `.env` 和十二个数据库密码；
- 首位管理员密码、IAM 签名私钥和现有 Session；
- TLS 私钥、Registry 密码、SSH 私钥、RAM AccessKey／STS 配置；
- DNS 记录、安全组、ECS 实例 ID、公网 IP、Bucket、RAM ARN；
- 个人环境数据库卷、运行中的 PostgreSQL 目录和个人测试业务数据；
- 个人 ACR 登录缓存或 Docker `config.json`。

### 2.3 发布单元

一次可审计部署由四项共同确定：

1. D1 发布包及其外层 `.sha256`；
2. 包内 `RELEASE-METADATA` 和 `RELEASE-CHECKSUMS.sha256`；
3. 目标 Registry 中五个必需镜像和全部已选应用的摘要引用；
4. 目标环境独立生成、权限为 `0600` 的 `.env` 与证书／秘密。

只有镜像标签、只有 Git 分支或只有 Compose 文件都不是完整发布单元。

发布单元还必须形成兼容闭包。修改 Capability／Suite Manifest、策略或公开 Schema 时，清单中必须同时包含所有会解析该版本的运行组件；增加应用 migration 时，目标镜像必须包含能识别升级后 schema 的 server 与 ops。不能只根据 Git diff 选择“发生业务改动的应用”。`test.6` 实际演练已经证明：新 `capability_namespace` 配旧 IAM 会在启动时拒绝解析，新 Project migration 配旧 Project 二进制会被 schema 门禁拒绝；`test.10` 又证明 Suite Manifest 新字段会同时约束 Gateway 与 Workbench；`test.11` 在保留该闭包的前提下单独提升 Money，并通过正式指标权限验收。完整规则见 [ADR-0035](../adr/0035-d1-release-upgrades-use-compatibility-closure.md)。

已有环境升级前必须先执行：

```bash
D1_ENV_FILE=/受限目录/目标.env deployments/d1/ops/d1 preflight-upgrade
```

该命令先准备摘要固定的候选镜像，再用候选 IAM 解析发布包中的全部 Capability Manifest、Assignment Policy 和 Workload Policy；它不连接数据库。`d1 upgrade` 会在联合备份和任何 migration 之前自动重复同一预检，解析失败立即停止。此门禁能阻止 `test.6` 已出现的“新 Capability Manifest + 旧 IAM”；`test.10` 已把 Gateway／Workbench 纳入当前选装集合闭包，`test.11` 已证明单应用提升可沿用该兼容闭包完成升级，但命令还不能自动推导所有 Suite／migration 消费者，发布评审仍须核对 ADR-0035。

## 3. 备案等待期内立即完成的工作

以下工作不依赖公司域名或公司云账号：

- [x] D1 运行镜像固定为 `linux/amd64` 并以摘要运行；
- [x] 个人 ECS 内既有 Nginx → Gateway → IAM → Contract／Money／People／Evidence Local 主链通过；Project 已在本地隔离 D1 通过同一入口和资源权限，仍需随十镜像新版本提升到 ECS 复验；
- [x] HTTPS 配置、证书匹配、TLS 1.2／1.3、Secure Cookie 和仅 Nginx 暴露端口通过；
- [x] 增加 pull-only D1 发布包，目标机不能切回源码构建；
- [x] 增加跨 Registry `pull → tag → push` 制品提升入口，目标清单重新记录摘要；
- [x] 发布包自动排除 `.env`、`.runtime`、证书、私钥和 AppleDouble 文件；
- [x] 发布包包含 Compose 运行所需的九份 Capability Manifest 和外部验收入口；
- [x] 用当前 `v0.1.0-test.2` 摘要清单生成一次实际开发验证包并保存外层摘要；
- [ ] 在隔离 Compose 项目或第二台空主机复演“解包 → 配置 → 拉取 → 启动 → 验收”；
- [ ] 把一次联合备份复制到 ECS 之外的受控加密存储；
- [ ] 在隔离空目标完成当前版本联合恢复复演；
- [ ] 确认公司验证是否要求 Evidence 直接使用 OSS。当前 D1 整栈使用 Local Provider，真实 OSS 浏览器链已经独立验证，但两者尚未合为一套 D1 运行实例。

其中最后四项可继续推进，即使公司环境最终没有获批。

## 4. 公司会议需要确认的最小输入

不要索要或把密码、私钥、Cookie、AccessKey 发到文档或聊天。会议只需确认资源归属与非敏感坐标：

| 类别 | 必须确认 | 安全边界 |
| --- | --- | --- |
| 授权 | 允许在隔离范围部署验证版，明确负责人和可操作时段 | 不默认拥有现有生产系统修改权 |
| 域名 | 一个已备案、可控制 DNS 的验证子域名 | 不复用正在承载生产的主机名 |
| ECS | 实例或批准新建的实例、地域、`linux/amd64`、Ubuntu 22.04、磁盘容量 | 优先独立 ECS；不得覆盖现有服务 |
| Registry | 公司控制的 ACR／Registry 域名、命名空间、仓库策略 | 使用公司凭据；不把个人 ACR 作为长期生产依赖 |
| 网络 | VPC、安全组、允许的 SSH 来源、80／443 公网入口 | Gateway、IAM、应用和 PostgreSQL 不开放公网端口 |
| TLS | 公司域名对应的有效证书或批准的签发方式 | 私钥只落目标主机受限目录 |
| Evidence | 本次用 Local Provider 还是同时验证公司 OSS | 若用 OSS，Bucket、RAM、CORS 和 STS 全部由公司账号创建 |
| 数据 | 默认使用空库和合成验证数据 | 未另行批准不得导入公司或个人真实业务数据 |
| 备份 | ECS 外的加密落点、访问责任和保留期 | 同机目录只作暂存，不算离机备份 |

会议结果只需回答“获批／未获批／还缺什么”。获批后再通过密码管理器或公司认可的秘密渠道完成凭据交接。

## 5. 构建源环境发布包

发布前先运行仓库门禁：

```bash
make d1-image-mode-test \
     d1-image-promotion-test \
     d1-ingress-mode-test \
     d1-release-bundle-test
```

使用发布脚本已经生成的摘要清单构建 D1 包：

```bash
make d1-release \
  VERSION=v0.1.0-test.2 \
  IMAGE_MANIFEST=/absolute/path/to/published-images-v0.1.0-test.2.env \
  OUTPUT_DIR=dist
```

开发验证允许默认 `RELEASE_PROVENANCE_MODE=development`。对外正式发布必须从干净 Git HEAD 执行：

```bash
RELEASE_PROVENANCE_MODE=official \
RELEASE_BUILT_AT=2026-09-24T00:00:00Z \
make d1-release VERSION=v0.1.0 IMAGE_MANIFEST=/absolute/path/to/images.env
```

输出为：

```text
dist/omsbizfoundry-d1_<version>_linux_amd64.tar.gz
dist/omsbizfoundry-d1_<version>_linux_amd64.tar.gz.sha256
```

包内 `.env.example` 已固定 `D1_IMAGE_MODE=pull`，包含 PostgreSQL、IAM、Workbench、Gateway、Nginx 五个必需摘要引用，以及镜像清单显式选择的应用摘要引用；未提供镜像的可选应用必须保持未选状态。`.release-bundle` 门禁会拒绝目标机把它改为 `build`。

## 6. 将制品提升到公司 Registry

公司环境优先运行公司 Registry 中的副本。提升过程复制已经验证的镜像内容，不在公司 ECS 重新编译源码。

先分别交互登录源 Registry 和公司 Registry：

```bash
docker login <SOURCE_REGISTRY>
docker login <COMPANY_REGISTRY>
```

先预览，不写 Registry：

```bash
D1_SOURCE_IMAGE_MANIFEST=/absolute/path/to/source-images.env \
D1_TARGET_REGISTRY=<COMPANY_REGISTRY> \
D1_TARGET_REGISTRY_NAMESPACE=omsbizfoundry \
D1_RELEASE_VERSION=v0.1.0-test.2 \
D1_IMAGE_MANIFEST_OUT=/absolute/path/to/company-images.env \
  deployments/d1/ops/promote-images plan
```

确认目标坐标后执行：

```bash
D1_SOURCE_IMAGE_MANIFEST=/absolute/path/to/source-images.env \
D1_TARGET_REGISTRY=<COMPANY_REGISTRY> \
D1_TARGET_REGISTRY_NAMESPACE=omsbizfoundry \
D1_RELEASE_VERSION=v0.1.0-test.2 \
D1_IMAGE_MANIFEST_OUT=/absolute/path/to/company-images.env \
  deployments/d1/ops/promote-images promote
```

脚本逐个执行固定平台的 `pull → tag → push`，比较本地 Image ID，检查目标 Registry 的平台，并输出新的十五镜像摘要清单。脚本不接收、不保存 Registry 密码。成功后使用 `company-images.env` 再构建一份目标发布包；不要手工把个人 Registry 域名替换成公司域名。

## 7. 公司 ECS 部署步骤

### 7.1 建立宿主机基线

从操作者设备执行只读检查：

```bash
D1_SSH_TARGET=ecs-user@<COMPANY_ECS_IP> \
D1_SSH_KEY=/absolute/path/to/company-private-key \
  deployments/d1/ops/bootstrap-remote-host check
```

全新 Ubuntu 22.04 主机按 [阿里云 ECS D1 宿主机初始化](./aliyun-ecs-d1-bootstrap.md)执行 `apply`。现有公司主机不得直接 `apply`；必须先确认不会覆盖 Docker 配置、SSH 策略、目录权限或现有服务。

### 7.2 验证并解包

```bash
sha256sum -c omsbizfoundry-d1_<version>_linux_amd64.tar.gz.sha256
tar --no-same-owner -xzf omsbizfoundry-d1_<version>_linux_amd64.tar.gz
cd omsbizfoundry-d1_<version>_linux_amd64
sha256sum -c RELEASE-CHECKSUMS.sha256
```

macOS 可使用 `shasum -a 256 -c`。校验失败时丢弃文件，不继续部署。

### 7.3 生成公司环境配置

```bash
cp deployments/d1/.env.example /opt/omsbizfoundry/company-validation.env
chmod 600 /opt/omsbizfoundry/company-validation.env
```

至少重新生成并核对：

- `TENANT_ID`、租户显示名和首位管理员密码；
- 十二个互不相同的数据库密码；
- Evidence Local 签名密钥；
- `D1_PUBLIC_HOST`、`PUBLIC_ISSUER`、`EVIDENCE_PUBLIC_BASE_URL`；
- 公司证书和私钥的绝对路径；
- 公司 Registry 的十五个摘要引用；
- ECS 外备份暂存／同步目标；
- 应用开关。

保持 `D1_IMAGE_MODE=pull`、`D1_INGRESS_MODE=https`、`COOKIE_SECURE=true`。不要复用个人环境的 `.env`。

### 7.4 DNS、TLS 和安全组

- 验证子域名 A／AAAA 记录只指向目标入口；
- 确认阿里云已识别该域名的备案／接入状态；
- 公网入方向只开放 `80/443`，SSH `22` 只开放给批准的运维来源；
- 删除或拒绝公网 `3389`、Gateway、IAM、应用和 PostgreSQL 端口；
- TLS 私钥权限为 `0400` 或 `0600`，不进入发布目录；
- 保留 80 仅用于固定主机名的 HTTPS 跳转。

### 7.5 拉取、启动和首管授权

在公司 ECS 交互登录公司 Registry，拉取结束后再 `docker logout`：

```bash
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/d1 config --quiet
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/d1 images
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/d1 prepare-images
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/d1 up
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/d1 check
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env deployments/d1/ops/grant-bootstrap-access
docker logout <COMPANY_REGISTRY>
```

选择 `reference-oa-administrator-v1` 的环境必须在每次新增应用、替换 Capability Manifest 或增加 Action 后再次执行 `grant-bootstrap-access`。完成条件不是脚本曾经运行过，而是指定 OA 管理员针对实际装载 Catalog 的 tenant Grant 与 Credential 动作达到逐项一致；IAM `administrator` 标志本身不代表业务全权限。

已有数据环境升级不要绕过 `d1 upgrade` 手工整栈并行重建。先运行 `config`／`images`／`prepare-images` 和预检；正式命令会先创建联合备份，随后保留命名卷清理旧长期容器和 one-shot migration 容器，再由同一候选重新执行 migration、重建服务、等待 health 并执行 `check`。故障定位可以按兼容单元串行恢复，但该混合状态不能登记为发布完成。只要某应用已执行前向 migration，就不得用旧二进制连接该数据库来模拟回滚；需要数据级回退时，把升级前联合备份恢复到新的空目标，再运行上一发布包。

## 8. 验收矩阵

### 8.1 目标机验收

- `d1 status` 与 `d1 check` 全部通过；
- 全部已启用长期容器健康，migration 容器成功退出；
- 宿主监听仅包括批准的 SSH、80 和 443；
- Nginx、Gateway、IAM 和各业务应用日志不存在持续错误；
- Docker 登录缓存不保留公司 Registry 密码；
- 创建一次联合备份，校验全部外层摘要。

### 8.2 独立公网验收

从不在目标 VPC 的网络执行：

- HTTP 固定跳转到批准的 HTTPS 主机；
- 证书主机名、链、有效期和 TLS 1.2／1.3 通过；
- 无效 Host 被拒绝；
- 匿名访问受保护页面被拒绝；
- 首位管理员完成一次 IAM 登录，Cookie 为 Secure／HttpOnly／SameSite=Lax；
- Workbench 只显示已启用应用；
- Gateway 能到达 IAM 和已启用小系统；
- Contract、Money、People 至少各完成一次代表性写入／读取；
- Evidence Local 完成上传、complete、授权和范围下载；若会议明确要求 OSS，则追加真实 STS、multipart、complete、签名下载与匿名拒绝；
- 停止一个业务应用时，其路由失败但其他应用保持可用；
- 外网无法直接访问 Gateway、IAM、应用和 PostgreSQL 端口。

验收脚本会写入合成数据并可短暂停止业务容器。只有隔离验证环境才运行：

```bash
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env \
D1_PUBLIC_URL=https://<COMPANY_VALIDATION_HOST> \
D1_ACCEPTANCE_DISRUPTIVE=yes \
  acceptance/d1-delivery/verify-running
```

### 8.3 恢复证明

“备份成功”不等于“可恢复”。在隔离 Compose 项目或另一台空主机执行：

```bash
D1_ENV_FILE=/opt/omsbizfoundry/company-validation.env \
D1_PROJECT_NAME=omsbizfoundry-company-restore \
D1_CONFIRM_EMPTY_RESTORE=yes \
  deployments/d1/ops/d1 restore <BACKUP_ID>
```

恢复目标必须是空卷；不得对正在运行的验证项目执行破坏性恢复。恢复后重新登录、核对事实并继续写入。

## 9. 两天后分支决策

### 9.1 公司环境获批

1. 冻结待验证版本与源摘要清单；
2. 在公司 Registry 提升十五个镜像并生成目标摘要清单；
3. 用目标清单构建公司 D1 发布包；
4. 在隔离 ECS 建立宿主基线、DNS、TLS、安全组和公司秘密；
5. 启动并完成目标机、公网、业务、端口、备份和恢复验收；
6. 记录结果，但不自动把验证环境标为生产。

### 9.2 公司环境未获批或信息不全

1. 不修改公司现有资源，不使用端口或 IP 绕过备案；
2. 完成个人试验田的离机备份和空目标恢复复演；
3. 在隔离环境重复发布包安装，持续消除个人坐标硬编码；
4. 继续阶段一参考 OA 的范围确认与小系统业务验证；
5. `xidou.me` 备案通过后，按 [D1 HTTPS 入口方案](./d1-https-ingress.md)重新执行独立公网验收。

## 10. 回退与清理

- 公司验证失败时保留日志、发布包摘要和备份，不修改镜像标签指向；
- 回退使用上一份摘要清单和上一发布包，不在目标 ECS 现场修镜像；
- DNS 切换只在新环境完整验收后执行；本次并行验证默认没有生产流量切换；
- 删除验证环境前先按授权规则处理合成数据、备份、证书和 Registry 凭据；
- 个人 `xidou.me` 环境继续保留，不因公司验证成功而删除。

## 11. 当前仍然开放的事实

- 公司域名、云账号和隔离资源是否获批；
- 公司验证使用 Local Evidence 还是同时完成 OSS Provider 的整栈接入；
- 公司 ECS 是否为独立空主机，还是已有服务的共享主机；
- 公司备份的离机落点、保留期和恢复责任；
- `xidou.me` 备案完成时间。

这些事项没有明确答案前保持开放，不写入脚本默认值，也不阻塞发布包、制品提升、内部验收和个人试验田工作。
