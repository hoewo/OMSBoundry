# D1 Compact 运行手册

日期：2026-09-29
适用范围：单台 ECS 上的当前 IAM＋Workbench 与可选 People、Party、Project、Contract、Money、Evidence、Business Engagement、Billing、Production、Approval & Audit Results、Cost & Outsourcing、Bonus Distribution 组合

## 1. 前提

- Linux ECS 已安装 Docker Engine 与 Docker Compose v2；
- 阿里云单机实验环境可按[阿里云 ECS D1 宿主机初始化](./aliyun-ecs-d1-bootstrap.md)建立并验证宿主机基线；
- 仓库或同版本发布目录位于受限运维路径；
- 本机具备 `curl`、`jq` 和 `openssl`；
- 生产已有域名、有效证书和只向运维来源开放的 SSH；
- 备份目录位于独立云盘挂载点，或在生成后立即同步到 ECS 之外的加密存储。

不要把 `.env`、证书、IAM 私钥或备份提交到 Git。

仓库内宿主机自动化是推荐前置入口。先在操作者设备执行：

```bash
make d1-host-bootstrap-test

D1_SSH_TARGET=ecs-user@<ECS_PUBLIC_IP> \
D1_SSH_KEY=/absolute/path/to/private-key \
  deployments/d1/ops/bootstrap-remote-host check
```

新 ECS 的首次 `apply`、账号专属 ACR 加速地址和 SSH 安全确认参数见宿主机初始化文档。该入口只建立宿主机基线，不创建本节以下的应用 `.env`、域名、证书、ACR 仓库或业务凭据。

云端部署还必须先按 [ACR 私有镜像供应](./acr-image-supply.md) 完成个人版实例、命名空间和自动创建私有仓库设置，再由发布脚本生成摘要清单。镜像加速器不能代替私有 Registry。

公网入口实施按 [D1 公网 HTTPS 入口实施方案](./d1-https-ingress.md)执行。在该方案的 TLS 配置、证书只读挂载和静态检查完成前，不要把当前明文 Nginx 配置直接发布到宿主机 443 端口。

把同一 D1 版本复制到另一阿里云账号、Registry 或域名时，按 [D1 双环境制品提升与公网验证](./d1-environment-promotion.md)生成 pull-only 发布包并提升不可变镜像。个人试验田与公司验证环境分别持有自己的 `.env`、TLS、Registry、OSS／RAM 和备份；不得复制个人环境秘密或在目标 ECS 重新编译源码。

## 2. 初次配置

```bash
cd deployments/d1
cp .env.example .env
chmod 600 .env
```

必须替换十四个数据库密码（PostgreSQL 管理账号、IAM、People & Groups、Party Directory、Project Workspace、Contract、Money、Evidence、Business Engagement、Billing、Production Operations、Approval & Audit Results、Cost & Outsourcing、Bonus Distribution）、Party 游标 HMAC 密钥、Evidence 本地数据面签名密钥和首位管理员密码；数据库密码各不相同、至少 16 字符，并只使用 `A-Z a-z 0-9 _ . ~ -` 以便安全进入 PostgreSQL URL；Party 游标密钥至少 32 字节；Evidence 签名密钥至少 32 个随机字节并使用无填充 base64url；首管密码至少 12 字节。全部建议由密码管理器随机生成。生产还必须设置：

```text
D1_INGRESS_MODE=https
PUBLIC_BIND=0.0.0.0
PUBLIC_HTTP_PORT=80
PUBLIC_HTTPS_PORT=443
D1_PUBLIC_HOST=你的域名
D1_TLS_CERT_FILE=/opt/omsbizfoundry/secrets/tls/你的域名.pem
D1_TLS_KEY_FILE=/opt/omsbizfoundry/secrets/tls/你的域名.key
COOKIE_SECURE=true
PUBLIC_ISSUER=https://你的域名/auth
EVIDENCE_PUBLIC_BASE_URL=https://你的域名/evidence
D1_BACKUP_DIR=/独立挂载点/omsbizfoundry-backups
```

HTTPS 模式由 `compose.https.yaml` 覆盖基础 HTTP 配置。`d1` 会在任何操作前校验证书有效期、主机名、私钥权限和证书／私钥匹配关系；私钥必须是未加密 PEM 且权限为 `0400` 或 `0600`。具体端口、证书落盘和验收规则以 [D1 公网 HTTPS 入口实施方案](./d1-https-ingress.md)为准。证书私钥不得进入仓库、镜像或 `.env`。

启用应用：

```text
ENABLE_CONTRACT=true
ENABLE_MONEY=true
ENABLE_PEOPLE=true
ENABLE_PARTY=true
ENABLE_PROJECT=true
ENABLE_EVIDENCE=true
ENABLE_BUSINESS_ENGAGEMENT=false
ENABLE_BILLING=true
ENABLE_PRODUCTION=true
ENABLE_APPROVAL_AUDIT_RESULTS=true
ENABLE_COST_OUTSOURCING=true
ENABLE_BONUS_DISTRIBUTION=true
```

本地开发默认使用：

```text
D1_IMAGE_MODE=build
```

ECS 使用 `D1_IMAGE_MODE=pull`，并把发布方生成的十七个 D1 运行镜像 `name@sha256:digest` 值合并进 `.env`。先执行 `d1 images` 检查引用，再执行 `d1 prepare-images` 验证 Registry 登录和拉取；任何已启用组件缺摘要都会在启动前失败。

使用 `deployments/d1/ops/build-release` 生成的发布包时，包内 `.env.example` 已包含十七个摘要引用并固定为 pull 模式；`.release-bundle` 门禁会拒绝改为 build。发布包不包含真实 `.env`、证书、私钥、Registry 密码、数据库数据或备份。

## 3. 安装与首管授权

从仓库根目录执行：

```bash
deployments/d1/ops/d1 config --quiet
deployments/d1/ops/d1 images
deployments/d1/ops/d1 prepare-images
deployments/d1/ops/d1 up
deployments/d1/ops/d1 check
deployments/d1/ops/grant-bootstrap-access
deployments/d1/ops/d1 status
```

若该部署选择 OA 管理员全权限画像，新增应用或 Capability 后必须重跑 `grant-bootstrap-access`；应验证管理员 Credential 覆盖实际装载 Catalog，而不是只检查 `administrator=true`。

浏览器只访问 Nginx 的统一域名。不要给 Gateway、IAM、应用或 PostgreSQL 增加宿主端口映射。

D1 Compose 固定设置 `TRUST_PROXY_CLIENT_IP=true`，其成立条件是 Gateway 没有宿主机端口、所有入口都先经过本组合的 Nginx。Nginx 会覆盖外来的 `X-Forwarded-For`，只把直接客户端 `$remote_addr` 交给 Gateway；Gateway 只接受一个有效 IP，链式或非法值退回直接对端。`REQUESTS_PER_MINUTE` 默认按这个派生客户端键计数，可按实测容量显式调整，但不得用提高限额掩盖代理地址聚合。若增加云负载均衡、CDN、额外反向代理或 Gateway 直连，先更新并验证可信代理契约，不要直接复用本设置。

本地回环验收可以执行：

```bash
D1_ACCEPTANCE_DISRUPTIVE=yes make d1-delivery-acceptance
```

`D1_ACCEPTANCE_DISRUPTIVE=yes` 会依次短暂停止并恢复 Contract、People & Groups、Party Directory、Project Workspace 与 Evidence，用于确认其余应用的故障隔离，并验证 Project 对 Party、Billing 对 Party／Project／Evidence 的精确作用域工作负载授权、失败关闭与幂等恢复；生产营业时间不要启用该参数。

已启用 Money 时，可以单独验证 C8 指标的正式入口和权限分层。该入口会创建并最终停用一个临时 IAM 账号；如果当前指标集合为空，还会通过 Money 正式页面 API 创建带 `C8-METRIC` 标识的两个测试 Party 和一条测试 Cash Transaction，因此只应在验收环境显式执行：

```bash
D1_ENV_FILE=/受限目录/目标.env \
D1_PROJECT_NAME=omsbizfoundry \
D1_PUBLIC_URL=http://127.0.0.1:18000 \
D1_C8_METRIC_CONFIRM=yes \
C8_METRIC_EVIDENCE_DIR=/受限目录/c8-money-metric-evidence \
  acceptance/d1-c8-money-metric/verify-running
```

通过标准包括：无统计能力 403、只有统计能力时金额与摘要全部脱敏、增加金额能力后只有重新签发的凭证显示精确值、撤权并重新签发后恢复脱敏，以及权限读取前后的 Money 业务事实摘要完全一致。临时账号结束时必须为 disabled 且活动 Grant 为 0。

## 4. 应用开关

修改 `.env` 中对应的 `ENABLE_*` 开关；当前包括 Contract、Money、People、Party、Project、Evidence、Business Engagement、Billing、Production、Approval & Audit Results、Cost & Outsourcing 和 Bonus Distribution。然后执行：

```bash
deployments/d1/ops/d1 apply
```

关闭后的业务应用从 Gateway 路由和 Workbench 菜单消失，目标容器停止，但数据库、对象卷和备份不会删除。Evidence 是不显示在 Workbench 的技术应用；关闭它还会从生成的 Nginx 配置中移除本地上传／下载数据路由。Billing、Production、Approval & Audit Results、Cost & Outsourcing 与 Bonus Distribution 已提供人类工作区，关闭后路由、菜单和容器一起缺席。Bonus Distribution 要求 People、Project 以及 Contract／Money 至少一个来源启用；当前正式 D1 验收同时启用两类来源，单来源最小组合仍需独立验证。重新启用会先执行 current-schema 与健康门禁。IAM、Gateway 和 Workbench 不是可选业务开关。

当前 D1 的 Evidence 控制面经 `Nginx → Gateway → Evidence`。Local Provider 的上传／下载字节使用 Evidence 签发的对象级短期 URL，经 Nginx 直接到 Evidence，不经过 Gateway；访问日志不记录 URL 查询 Token。生产使用 OSS Provider 时，字节应由浏览器直连私有 OSS，控制面仍走统一入口。

## 5. 日常检查与日志

如目标环境需要额外 Compose 覆盖文件（例如为并行隔离演练固定不冲突的 Docker 子网），必须在该环境的全部 `up`、`check`、`backup`、`restore` 命令中传入同一个绝对路径：

```bash
D1_COMPOSE_OVERRIDE_FILE=/absolute/path/d1.networks.yaml deployments/d1/ops/d1 check
```

覆盖文件只用于显式的部署差异，不得承载密码或替代 Suite Manifest；未设置时仍只使用正式 D1 Compose 文件。

```bash
deployments/d1/ops/d1 status
deployments/d1/ops/d1 check
docker compose --env-file deployments/d1/.env -f deployments/d1/compose.yaml logs --tail=200
```

健康不等于业务正确；异常时同时核对 Nginx、Gateway、IAM、目标应用和 PostgreSQL 日志，但不得把 Cookie、Authorization、数据库密码或私钥复制到工单。

若多个真实用户同时出现 `429`，先检查 Nginx 渲染配置是否使用单值 `$remote_addr`，以及 Gateway 是否只在私网且启用了可信代理模式。不要改回 `$proxy_add_x_forwarded_for`，它会把客户端可伪造的转发链带入信任边界。完成拓扑变更后至少运行 `go test ./backend/platform/gateway/...`、`sh deployments/d1/ops/test-ingress-mode` 和对应 D1 入口验收。

### 5.1 开放 API 使用记录保留

开放 API 使用记录固定保留 180 天。由受控运维任务定期执行以下命令；`OPERATOR_ID` 使用能回溯到本次运维任务的非秘密标识，不填写账号密码、API Key 或工单正文：

```bash
deployments/d1/ops/d1 purge-open-api-usage retention-job-20261025
```

命令只删除到期的脱敏调用元数据，并追加不可变清理记录；不会删除 API Key 元数据、报销、Evidence 或 Money 事实。运行结果必须保存 `run_id`、cutoff 和删除数量。该命令要求已有运行中的 Identity & Access 容器，不会启动全套服务。

### 5.2 Contract／Money Party 人工映射

该操作只记录人工确认的兼容映射，不会切换 Contract／Money 的运行时 Party 来源。先分别在台账容器内运行 `party-migration-preflight`，保存完整 JSON，并逐条人工核对来源摘要、业务引用计数、目标 Directory Party ID／revision 和理由。禁止按名称或登记标识自动合并。

审核 JSON 的字段和人工门禁见 [Contract／Money Party 人工确认映射实验](../reports/oa-party-ledger-reviewed-mapping-experiment-20260927.md)。复核后从仓库根目录执行：

```bash
deployments/d1/ops/d1 party-map contract /absolute/path/contract-review.json
deployments/d1/ops/d1 party-map money /absolute/path/money-review.json
```

首次成功应显示 `recorded`，重复同一文件应只显示 `replayed`；任何冲突、stale revision、inactive／错租户目标、Party 不可用或来源摘要漂移都必须失败且不增加映射。输出中的 `ready_for_cutover` 必须保持 `false`。审核文件含真实标识和审计依据，应放在受控运维存储，不提交 Git。已验证 D1 证据见 [D1 人工确认映射实验](../reports/oa-party-ledger-d1-reviewed-mapping-experiment-20260927.md)。

## 6. 备份

```bash
deployments/d1/ops/d1 backup
```

命令返回 UTC 备份 ID。确认目录包含 `identity-access.dump`、`signing-keys.json`、`suite-selection.env`、启用台账应用的 ZIP、启用 People & Groups 时的 `people-groups.dump`、启用 Party Directory 时的 `party-directory.zip`、启用 Project Workspace 时的 `project-workspace.dump`、启用 Evidence 时的 `evidence-service.zip`、启用 Business Engagement 时的 `business-engagement.dump`、启用 Billing 时的 `billing-invoice.dump`、启用 Production 时的 `production-operations.dump`、启用 Approval & Audit Results 时的 `approval-audit-results.dump`、启用 Cost 时的 `cost-outsourcing.dump`、启用 Bonus 时的 `bonus-distribution.dump` 和 `checksums.sha256`，随后把整个 ID 目录复制到 ECS 之外的受控加密存储。`suite-selection.env` 记录实际覆盖的 12 个应用开关、公开选装 profile，以及 Workbench 实际使用的 `OMS_QUALIFICATION_PARTY_ID`；文件参与校验和，恢复时必须与旧环境核对。profile 与 OMS 资质单位 ID 都来自运行中的所有者／Workbench 容器，不取候选新环境；未知 profile、非法 Party UUID、v3 缺 D01 配套或恢复时 ID 不一致都会失败关闭。Evidence 备份期间写入进程会被短暂停止，以形成数据库和本地对象的一致集合。仅有云盘快照、仅有数据库 dump 或仅有业务 ZIP 都不构成当前组合的完整恢复材料。

## 7. 升级

先把新代码或发布物放到独立目录并核对来源，再执行：

```bash
deployments/d1/ops/d1 upgrade
```

该命令先从当前 Compose project 的实际服务识别旧应用选择集，检查并创建联合备份；备份成功后保留命名卷执行 `down --remove-orphans`，清除长期容器与遗留 one-shot migration 容器，再按候选 `.env` 构建／拉取镜像、重新运行各应用 migration、重建服务并执行候选 schema 检查。候选新启用但旧组合不存在的应用不会参加升级前检查；旧组合存在但候选关闭的应用仍会先备份。输出的备份目录内 `suite-selection.env` 是回退时旧开关及运行 profile 的依据；例如候选 v3 不能覆盖旧 v2 的备份声明。该重建流程不会删除卷；不得手工补 `-v`。

升级失败不会自动在原卷上反向 migration。不要直接修改 migration 表或复用半恢复数据库；保留旧发布目录、旧 `.env`、日志和升级前备份，使用与 `suite-selection.env` 相符的旧配置在全新空目标演练恢复后再决定切回。

## 8. 整机恢复演练

恢复会删除当前 `D1_PROJECT_NAME` 对应的 Compose 数据卷，只能在已确认的空目标或灾难恢复环境执行：

```bash
D1_CONFIRM_EMPTY_RESTORE=yes deployments/d1/ops/d1 restore 备份ID
```

脚本先校验外层 SHA-256；新格式备份逐项核对恢复配置中的 `ENABLE_*` 和两个公开 profile 与 `suite-selection.env`，错配、未知值或重复 profile 声明在删除任何卷前失败。旧格式备份缺该文件或 profile 声明时，仅兼容两项 profile 均关闭的恢复配置，且操作者仍必须人工核对原选择集；不能凭缺失元数据推断已启用版本，不能直接用候选 v3 配置恢复旧 v2 检查点。随后脚本重建空 PostgreSQL、恢复 IAM dump 与签名私钥、恢复全部已启用业务应用的独立 dump／台账备份、Party Directory 与 Evidence 数据库＋本地对象，最后启动并检查全部已启用应用数据库 schema 与 Evidence Provider。恢复前脚本还会在 IAM 使用的实际跨容器数据库地址上等待 PostgreSQL 可连接，避免“容器本机健康但业务容器 TCP 尚未就绪”的恢复竞态。恢复后：

- 旧浏览器 Session 应返回 401；
- 原账号可以重新登录；
- 原授权和业务事实应存在；
- 原 Evidence File ID、Version、SHA-256 和对象字节应一致；
- 已启用业务应用应各完成一次继续写入；
- 完整验收通过后才切换生产域名。

演练必须在另一台 ECS 或隔离 Compose 项目中定期完成；“备份命令成功”不能代替恢复证明。

## 9. 停止与卸载边界

普通停止保留数据：

```bash
deployments/d1/ops/d1 down
```

不得把 `docker compose down -v` 当作普通卸载命令。删除卷只允许在明确的空目标恢复流程或已完成离机备份、确认永久销毁的场景执行。D1 是单节点：ECS、系统盘或 Docker 整体故障会中断全部入口，因此不能宣称高可用。
