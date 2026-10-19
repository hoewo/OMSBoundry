# Auth58 D1 当前组合升级记录（2026-09-27）

状态：`v0.1.0-test.5` 已在 Auth58 完成保留数据升级、可信边缘限流修复、回环入口和 IAM 权限链验收；公网 HTTPS 仍待独立环境条件满足后验证。

## 目标与边界

**旧 Auth58 只运行 9 个 `v0.1.0-test.2` 摘要镜像，无法承载当前 Party、Project、Business Engagement 和新版 IAM／Gateway 能力 → 从当前工作树发布一套固定 `linux/amd64`、摘要锁定的完整 D1 开发验证制品，在已有可恢复备份的前提下执行保留数据升级。**

本记录不把制品发布等同于远端升级成功，也不把回环入口等同于公网 HTTPS。当前版本来自有未提交变更的开发工作树，只能作为验证版本，不能冒充正式可追溯发行。

## 升级前事实

- 目标：已授权的 Auth58 Ubuntu 22.04 x86_64 ECS；
- 旧组合：PostgreSQL、IAM、Contract、Money、People、Evidence、Gateway、Workbench、Nginx，共 9 个运行镜像；
- 入口：仅绑定 `127.0.0.1:18000`，本轮不会擅自开放公网 HTTP／HTTPS；
- 可恢复点：`/var/backups/omsbizfoundry/20260927T060314Z`；
- 备份内的 IAM dump／签名材料、Contract ZIP、Money ZIP、People dump、Evidence ZIP 和 checksum manifest 已逐项通过 SHA-256；
- 旧组合在备份后继续通过健康检查。

## 初次完整组合制品供应证据

发布版本：`v0.1.0-test.4`

发布器实际完成：

- 4 个受控基础镜像：PostgreSQL、Nginx、Go、Alpine；
- 10 个应用镜像：IAM、Contract、Money、People、Party、Project、Evidence、Business Engagement、Workbench、Gateway；
- 12 个 D1 运行镜像引用全部使用 `name@sha256:<digest>`；
- 每个运行镜像均由 Registry 配置复核为 `linux/amd64`；
- 生成清单权限为 `0600`，清单 SHA-256 为 `8ae14a281ba06bd24a10a6aee49321be09eaecb292f6682c4c17b6566d422fa1`。

发行包：

```text
deployments/d1/.runtime/releases/omsbizfoundry-d1_v0.1.0-test.4_linux_amd64.tar.gz
```

- 大小：43,653 bytes；
- 外层 SHA-256：`65ac28a6e2130ec01465288b43b3f4bfe6bd076900b9216ea7be42ee9584cb35`；
- 外部 `.sha256` 校验：通过；
- `RELEASE-CHECKSUMS.sha256` 全部内层文件校验：通过；
- 元数据：`commit=worktree-61475e54f973`、`target=linux/amd64`、development provenance；
- `make d1-image-mode-test`、`publish-images self-test`、`make d1-release-bundle-test`：通过。

## 远端升级结果

- 目标机已拉取 12 个摘要锁定镜像，并逐一验证为 `linux/amd64`；拉取完成后已执行 Registry logout，root Docker 配置中不再保留该 Registry 凭据；
- 原 `auth58.env` 已先复制为权限 `0600` 的时间戳回退文件，再用候选配置原子替换；当前正式环境文件权限仍为 `0600`；
- 由于候选配置首次启用 Party、Project 和 Business Engagement，而现有 `d1 upgrade` 会在启动新服务前按候选清单执行 `check_stack`，会错误要求尚不存在的新服务先通过检查，本次没有伪造 `upgrade` 成功；在已验证升级前联合备份 `20260927T060314Z` 的保护下，使用 `d1 up` 执行迁移和容器替换；
- 当前 12 个长期容器全部健康：PostgreSQL、IAM、Contract、Money、People、Party、Project、Evidence、Business Engagement、Gateway、Workbench 和 Nginx；
- 数据库已扩展为 8 个应用库，新增 `party_directory`、`project_workspace`、`business_engagement`；
- Contract schema 从 17 升到 18，Money schema 从 16 升到 17，IAM 为 `000004_workload_identity_exchange.sql`，其余启用应用均通过各自 current 检查；
- 升级前的 IAM tenant、Contract Party、Money Party、People person 和 Evidence file 均仍存在；升级后验收新增数据没有覆盖这些既有事实；
- 宿主机仍只公开 SSH 22；组合入口继续仅绑定 `127.0.0.1:18000`，未擅自打开公网 80／443。

## 运行验收

基础交付验收通过，实际覆盖：

- 只有 Nginx 发布组合端口；
- 一次 IAM 登录后由 Gateway 为 Contract、Money、People、Party、Project 和 Evidence 建立受保护访问；
- 各启用应用的代表性读写；
- Contract／Money 的 Party migration preflight；
- People roster snapshot；
- Party 到 Project 的精确引用与不可变快照；
- Evidence 上传、完成、授权和字节范围读取。

Business Engagement 的完整 IAM 矩阵也在同一真实 D1 上通过：

- 110 个 HTTP 状态断言，0 个不匹配，最大本机请求时间 `0.166174s`；
- Project Assignment policy 只签发目标应用需要的动作和精确 Project scope，凭证 TTL 不超过 300 秒；
- Finance 无业务权限、IAM 管理员不自动获得业务角色、伪造身份 Header、错 Project、已结束 Assignment、撤权动作和取消后非法状态变更均 fail closed；
- Evidence、IAM 和 Project Policy 依赖故障注入均得到预期拒绝并恢复；
- Business Engagement 通过短期工作负载身份访问 Project／Evidence，不携带静态跨应用 Token；
- IAM 审计至少记录 policy 7、Project workload 2、Evidence workload 1、撤权 8、账号禁用 1 条相关事件。

验收第一次在第 120 个以上的高频请求处触发 Gateway `429`。根因不是 IAM 拒绝，而是 `v0.1.0-test.4` Gateway 按其 `RemoteAddr` 限流；在固定 Nginx 前置结构中，所有用户被聚合到 Nginx 容器地址。为先完成真实权限矩阵，当时临时把 `REQUESTS_PER_MINUTE` 设为 `1200`。该临时绕行随后由下一节的 `v0.1.0-test.5` 可信边缘修复取代，目标正式配置已恢复为 `120`。

## `v0.1.0-test.5` 可信边缘限流修复

代码根因与真实症状一致：Nginx 后所有请求在 Gateway 看起来都来自同一个代理容器。修复采用显式、最小的边缘信任边界：

- Nginx 覆盖外来 `X-Forwarded-For`，只写入与其直接连接的单值 `$remote_addr`；
- Gateway 默认不信任代理 Header，D1 因 Gateway 无宿主机端口且 Nginx 是唯一入口而显式设置 `TRUST_PROXY_CLIENT_IP=true`；
- Gateway 只接受单个语法有效 IP，逗号链、空值和非法值退回直接对端，避免 Header 伪造逃逸限流；
- 单元测试证明两个经同一可信边缘进入的不同客户端不共享配额；入口配置测试拒绝重新使用 `$proxy_add_x_forwarded_for`。

修复制品重新发布全部 12 个 D1 运行镜像，均为摘要锁定 `linux/amd64`。镜像清单 SHA-256 为 `c982a83925fe3cc6ec4143d3f76eab43f3a4f1c2c683d13156326007cb7bc1cc`；发行包大小 `44,272` bytes，外层 SHA-256 为 `9c0f2b042d42bd2f4ab9f721408126a630ce94164d54de906ebfbcdfaa66118d`，内外层校验和、`make d1-release-bundle-test` 均通过。该包仍来自开发工作树，元数据为 `commit=unknown`，不能冒充正式可追溯发行。

Auth58 先验证并解包到独立 `v0.1.0-test.5` 目录，候选环境把限额恢复为 `120` 后再原子切换；失败路径会恢复旧环境并重建 `v0.1.0-test.4`。实际切换和 `d1 check` 均通过，Registry 拉取后已 logout，目标机不保留该 Registry 登录项。当前 12 个长期容器健康，运行配置为 `trust=true`、`rpm=120`，Nginx 渲染配置只出现 `proxy_set_header X-Forwarded-For $remote_addr`。

目标机复验证据：

- 默认 `120` 下，普通 D1 交付验收通过；
- Gateway 计数清空后，分别从 Workbench 和 Gateway 两个不同容器地址各经 Nginx 发出 70 次 `/auth/health/ready`，两边均为 `70/70`，合计 `140/140`；旧实现会在合计第 120 次后失败；
- C2 全矩阵仍是同一验收客户端的人工高频负载，因此只在临时验收环境中使用 `1200`，实际得到 110 个 HTTP 断言、0 个状态不匹配、最大本机请求时间 `0.161712s`；Evidence 503、账号停用、IAM outage 和 Policy outage 均启用并失败关闭；
- 验收结束自动重建正式配置并删除临时环境文件，最终再次确认 `REQUESTS_PER_MINUTE=120`、`TRUST_PROXY_CLIENT_IP=true` 和 12 个长期容器健康。

## 升级后恢复资产

升级并完成验收后创建联合备份 `20260927T071029Z`，包含：

- IAM dump 和签名密钥；
- Contract、Money、Party、Evidence ZIP；
- People、Project、Business Engagement dump；
- checksum manifest。

9 个资产已在目标机逐项通过 SHA-256，并连同 110 条 HTTP 验收证据复制到本机权限隔离目录：

```text
/Users/crispydog/.ssh/omsbizfoundry-migrations/auth58-20260927/omsbizfoundry-auth58-20260927T071029Z.tar.gz
```

离机包 SHA-256：`f40c867f37651ea5b62768bf5e0250247faecff68cc6b406cc26eecc7eacb49a`。本机解包后再次逐项校验通过。

限流修复、完整 C2 故障注入和正式限额恢复后又创建联合备份 `20260927T075229Z`。同样的 9 个资产在目标机逐项通过 SHA-256，并复制到：

```text
/Users/crispydog/.ssh/omsbizfoundry-migrations/auth58-20260927/omsbizfoundry-auth58-20260927T075229Z.tar.gz
```

该离机包 SHA-256 为 `c146884e7f73b6fd0a3c67985959e18ba69b889bdccc0f53a0f43288a946e4f4`，归档目录和 9 个资产均可枚举。此时保持的可观测事实包括 1 个 Tenant、26 个 Subject、233 条 IAM Audit、Contract／Money 各 3 个 Party、13 个 Person、8 个 Evidence File、2 个 Directory Party、12 个 Project 和 5 个 Business Engagement。

## 尚未成立的结论

- 本轮没有在 Auth58 上执行破坏性的空目标 restore；备份可读、资产完整和 checksum 通过，不等同于本轮实际回滚演练；
- 本次 Auth58 历史迁移仍是以已验证的升级前备份加 `d1 up` 完成，不能事后改写成 `d1 upgrade` 成功；该运维缺口已在后续源码中修复，并用独立本地 Compose project 实测“旧 Contract 组合增加 Party、候选检查、空目标恢复旧选择集”通过，但 `v0.1.0-test.5` 目标包尚未重新发布该脚本，也没有在 Auth58 重做新增应用升级／回退；
- C2 自动矩阵本身会从单个验收地址在一分钟内发出超过默认配额的请求，仍应使用显式、临时且执行后恢复的验收限额；这不等于生产默认值需要提高，也不削弱两客户端 `140/140` 的目标机隔离证据；
- 域名、TLS、80／443 和独立外网访问均不在本阶段已通过范围内；
- 该版本来自 development provenance，不能冒充正式 release。

## Auth58 升级前置门禁

发行包已上传到 `/opt/omsbizfoundry/releases` 下的独立 `v0.1.0-test.4` 目录。目标机验证结果：

- 外层归档 SHA-256：通过；
- `RELEASE-CHECKSUMS.sha256` 的全部内层文件：通过；
- 解包目录权限：`0700`；
- GNU tar 忽略了 macOS `LIBARCHIVE.xattr.com.apple.provenance` 扩展头，但文件字节和清单一致，也没有用该提示替代 checksum 门禁；
- 原 `auth58.env` 未修改，另建 `auth58-test4.env`，权限 `0600`、57 个环境键、0 个重复键；
- 候选环境保留原有租户、管理员、旧数据库、Evidence、入口和备份设置；新增的 Party／Project／Business Engagement 数据库密码和 Party cursor key 在目标机随机生成，未进入日志或仓库；
- Party、Project、Business Engagement 均显式启用，Compose 静态渲染得到 12 个摘要镜像；
- 候选环境文件 SHA-256 为 `fe503915c865c0f7dec3dc38c670746f07010b6edce77caa80466c6c66f74652`，用于切换前检测意外变化，不表示可以从摘要反推出秘密；
- 旧组合 `d1 check` 返回 0，IAM、Contract `000017`、Money `000016`、People 和 Evidence 均为 current；
- `20260927T060314Z` 的 6 个备份资产再次逐项通过 SHA-256；
- 升级前数据库仅有 `identity_access`、`contract_ledger`、`money_flow`、`people_groups`、`evidence_service`，新三个数据库尚未创建；
- 目标机仍只有 9 个长期容器，31 GiB 根盘可用、约 914 MiB 内存可用和 2 GiB Swap；静态准备未重启或替换任何容器。

该前置门禁已经转化为上面的真实升级证据。本阶段结论仅为：ACR 跨账号供给、Auth58 保留数据升级、回环 Nginx → Gateway → IAM → 应用链和当前 C2 权限模型可运行；公网 HTTPS 和恢复演练仍需后续独立证据。
