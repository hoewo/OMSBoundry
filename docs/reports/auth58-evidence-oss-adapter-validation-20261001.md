# Auth58 Evidence OSS 云端适配与恢复验证

日期：2026-10-01。结论：**ECS 身份、真实 OSS 文件链、Local／OSS 兼容回退、混合空库恢复和单份合成对象的离机误删恢复通过；正式 OA 文件页面尚未接入 OSS，完整 OA 结论仍为 `insufficient-evidence`。**

问题 A：RAM 控制台配置完成，实际容器的身份、字节和恢复链仍未知。结果 B：不切换主库、不改旧 File 引用，用独立库和现有镜像取得直接证据。

## 实际范围

目标是公司 Auth58 ECS，不是 Mac 上的业务容器。复用主栈已部署镜像 `sha256:4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf`；没有重新编译或更新主栈镜像。

登录／Capability Credential 签发经 `https://oatest.antuinfo.cn` 的 Nginx → Gateway → IAM。随后 Evidence 控制请求访问隔离容器私网地址，Python 脚本直接向 OSS 传输字节；因此本批不冒充正式公网 Evidence 路由或真人浏览器验收。

Files Bucket 为 `oatest-auth58-files-1`，杭州地域；服务端读取使用 `oss-cn-hangzhou-internal.aliyuncs.com`，首批脚本上传和签名下载使用公网 Bucket Endpoint。后续混合兼容批次已实际使用 `oatestfiles.antuinfo.cn` 完成上传、SDK 签名下载与 Range；正式浏览器仍待验。

一次只运行一个额外的验证容器，限制 0.5 CPU、192 MiB 内存，分片并发为 1，没有新增公网或宿主端口。上述 OSS 批次未停止主栈或既有 `restore20` 栈，额外验证容器均已停止。随后用户授权停止 `restore20` 降低负载；当前仅主栈 17/17 healthy，演练栈 17 个长期容器已停止并保留，详见下方记录。

## 已通过检查

| 检查 | 直接结果 |
| --- | --- |
| 容器内官方 SDK 实例身份 | 专用出口网络下 `ready / aliyun-oss`；无长期 AK |
| 正式 IAM | 真实登录和签发 Evidence Credential；匿名 Evidence API 为 401 |
| 上传最小权限 | 900 秒 STS，三项上传动作限定到单对象；越对象 PUT 与使用上传凭证 GET 均为 403 |
| 禁止覆盖 | 第一次 PUT 200，再次携带 `x-oss-forbid-overwrite` PUT 为 409 |
| 小文件 | 64 KiB 实际上传，服务端完成校验，SHA-256 一致 |
| 分片文件 | 21 MiB，8 MiB／8 MiB／5 MiB 三段串行；完成合并和摘要复核通过 |
| 幂等 | create 复用 Session，complete 复用 Version；2 个 Version、2 条完成审计 |
| 下载 | SDK 短期签名 URL 实际下载与摘要一致；Range 206；无签名匿名请求 403 |
| 加密 | 实际下载响应 `x-oss-server-side-encryption: AES256` |
| 冷备份与空目标恢复 | 2 份完整 Version canonical JSON 一致，再经 IAM 授权下载原 OSS 字节成功 |
| 主库保护 | 原库 10 个 Version、10 个 Session、23 条审计保持；Version 完整行摘要前后相同 |

主库 Version canonical MD5 为 `d073ef8be3b69c5bc1a51ec01dec2e57`。独立源库与恢复库 Version canonical SHA-256 为 `4fd317aea91833611c4422afa2f44665510b9efe9000fe3dbbcf11b14aac7388`。

保留的测试 File／Version：

| File ID | Version | 大小 | SHA-256 |
| --- | --- | --- | --- |
| `ceb4efe1-fe7f-4ccf-acf6-37a9b65f5921` | 1 | 65,536 B | `bdf860298d05af0e0b5a6a1fbf54e4a0c3a261425473c6dc4dc1df1bf65321cd` |
| `307f0470-f50a-4029-8a24-37364f1490c1` | 1 | 22,020,096 B | `4c6f4152bbfa81c0fe72ef27517a45847d09ee46c005532d5a00a61b8c3a867c` |

## 发现的问题与处理

1. 主 Evidence 只有 `app`／`data` 两个 `internal` 网络。用其容器内实际 `/app/ops check` 检查 OSS 时，SDK 获取 IMDS 临时身份报 `network is unreachable`。本批为隔离探针创建专用出口网络，**没有开放整个应用网络，也没有直接切换主 Provider**。
2. Nginx／Gateway 原先能读取 IMDS 的角色名称路径；这是元数据隔离缺口。本批通过 [`evidence-imds-guard`](../../deployments/d1/ops/evidence-imds-guard) 在 `DOCKER-USER` 中只保护 `100.100.100.200/32`，仅显式 Evidence bridge 可继续。复测 Gateway、Nginx、IAM、Project 的角色路径均不可读；Evidence 恢复容器仍可取得 SDK 身份、登录后签发地址并下载。重复 apply 通过。**规则目前仅是运行时配置，主机重启后的持久化没有验收。**
3. 首轮空库恢复下载为 `403 SignatureDoesNotMatch`。定位为验收脚本给无请求体的 OSS GET 多加了 `Content-Type: application/json`，与签名请求不一致；移除后两份下载均通过。修正发生在验收脚本，不是业务内核或云策略。

## 保留证据与恢复限制

- 源库／停止容器：`evidence_oss_auth58_20261001a`。
- 恢复库／停止容器：`evidence_oss_auth58_20261001r`。
- 专用网络：`evidence_oss_auth58_20261001a`，标签 `oms.purpose=evidence-oss-acceptance`；没有其他应用加入。
- 冷备份：`/var/backups/omsbizfoundry/evidence_oss_auth58_20261001a/evidence-service-oss.zip`。
- 离机副本：`deployments/aliyun/.runtime/auth58-evidence-oss-20261001/evidence-service-oss.zip`；本机目录 0700、文件 0600，Git ignore 生效。
- 两端 ZIP SHA-256：`83d1c2d1de28d97071a4d067a2ea0356fa14eb7504de6cb374a3182c89f9e429`，文件大小 42,172 B。

ZIP 仅备份数据库与 Provider 标识，**不备份 OSS 对象字节**。本批证明“恢复元数据后仍能使用未被删除的对象”，不证明对象误删、Bucket 丢失或跨地域灾难后的恢复。两份测试对象与独立库保留，未删除任何已确认 Version 或主库 Local 文件。

## 后续批次：Local／OSS 兼容、CNAME 与混合恢复

ACR 独立 Evidence 候选 `v0.1.0-test.41`，固定索引摘要 `sha256:824b2086d1327ec7198f7e0af8292b0b404f81813a38cff0020a87cee36a38fb`，由本机串行构建、ECS 只拉取；主 Suite 未升级。不是全 Suite 新发布包或 clean-HEAD 发布证据。

1. 原 Local 冷备份恢复到隔离副本，10 个完整 Version 行与主库一致；OSS-primary 逐一下载这 10 个本地文件并复核大小／SHA-256。
2. 实际 CNAME PUT 成功，Evidence 经 internal Endpoint 完成摘要复核；SDK 签名地址保持 `oatestfiles.antuinfo.cn`，Range 206、匿名 403。控制请求仍是探针私网地址，不是正式浏览器。
3. 同一兼容镜像改为 Local-primary；旧 OSS 文件仍可读，新 Local 文件完成上传。两种历史 Provider 共 12 个 Version 全部复读通过，没有重写历史位置。
4. 混合 v2 冷备份恢复到另一空库，12 个完整 Version canonical 行与源库相同，并逐一读取真实字节；ZIP 含 11 个 Local 对象和 1 个 OSS 外部引用，不含 OSS 字节。
5. `go test -p 1 ./...`、`go vet -p 1 ./...` 通过；真实 PostgreSQL 18 的原 Local v1 与混合 v2 冷备份／空库恢复集成测试各通过。File Transfer Core typecheck、16 项单元测试、契约检查通过。没有运行全仓其他应用测试。

发现并修复两项真实缺陷：文件页 OSS V1 签名漏掉 `x-oss-date` 对应的 Date 段，导致 403；规则由[阿里云签名文档](https://www.alibabacloud.com/help/zh/oss/developer-reference/include-signatures-in-the-authorization-header)核对并加入回归测试。文本对象 GET 在自动 gzip 解压后丢失原长度，导致 complete 500；实测 identity 响应为 4,736 B／SHA 一致，gzip 响应没有 Content-Length。服务端 SDK 改为请求原始表示，保留大小／摘要严格校验，并以真实 HTTP SDK 回归测试覆盖。失败夹具保留；不把失败轮次算作通过。

保留证据：源 `evidence_oss_auth58_20261001m7`、恢复 `evidence_oss_auth58_20261001m8`，测试容器已停止。OSS File `b22bd9d5-8204-4241-94e1-40981b403050@1`，SHA-256 `c785b523695a4bcf7a905757b3250815e0859e5c98f709a2fdd90107ad9bf4cc`；新增 Local File `7b47622a-3b02-4b5d-a3f9-b3c45aa8e22f@1`。12 个 Version canonical SHA-256 `7f4c29b81ae55ad5ed19ca3cd8e919cce03a5387ddda686eadd2660267c8f853`。混合 ZIP `/var/backups/omsbizfoundry/evidence_oss_auth58_20261001m7/mixed-v2.zip`，60,925 B，离机副本 `deployments/aliyun/.runtime/auth58-evidence-oss-20261001/mixed-v2.zip`，两端 SHA-256 `341b29b35553bd04584962a9b65e8be71fa39bfe19ccc482644b7a24cfcb6145`，文件 0600、Git ignore 生效。主库计数及完整 Version 行摘要始终不变。

## 单份 OSS 字节的离机误删恢复

仅针对上述 4,736 B 合成 File `b22bd9d5-8204-4241-94e1-40981b403050@1`，官方 SDK 导出原始字节，复制到 Mac 并核对 SHA-256，再用独立路径送回 ECS。显式校验文件名、File ID／Version、单对象 Key、大小和合成内容后才删除该测试对象；实际 GET 观察到不存在，再以单对象 STS 上传离机副本，重新读取大小／摘要一致。源库与恢复库 12 个不可变 Version 行，以及主 Evidence 10 个 Version，均未改变。

离机字节副本为 `deployments/aliyun/.runtime/auth58-evidence-oss-20261001/oss-fixture-export.bin`，0600；SHA-256 为 `c785b523695a4bcf7a905757b3250815e0859e5c98f709a2fdd90107ad9bf4cc`。只删除了这份合成对象，已经恢复；没有删除正式文件。

这是单份精确夹具的恢复机制证明，不是全 Bucket、定时备份、跨地域灾难或生产保留策略的验收。Mixed ZIP 仍不含 OSS 字节，Bucket 版本控制仍关闭。

## D1 部署配置候选

`compose.evidence-oss.yaml` 与 Guard 已纳入 D1 包。Local 默认档保持不变；hybrid 两种 Primary 和纯 OSS 配置渲染、六项非法配置拒绝通过。只有 Evidence 加入显式外部出口，`app`／`data` 仍是 internal，只有 Nginx 发布端口；云档明确启用官方凭据 SDK 的 `ALIBABA_CLOUD_ECS_IMDSV2_ENABLE=true`。

`test.42` 固定镜像包从主栈实际 17 项镜像摘要生成，仅 Evidence 替换为 `test.41` 候选。ECS 上外层 ZIP、内层文件摘要与私有环境渲染通过；本节不是正式 rollout 或 clean-HEAD 发布证据。真实 Linux 新网络命名空间中的 boot／幂等、防绕过、陌生链保留及非法边界拒绝通过；未重启主机或 Docker，未启用 systemd 持久化。

后续 `test.44` 包以实际部署的 Project `test.43` 及其余 16 项镜像重新生成，只将 Evidence 替换为已验 `test.41`。包包含每次 Docker 启动前运行的 Guard drop-in 模板及运行时 Provider／Primary 核对器。ECS 外层 SHA-256 `7bc09bf4c7f64f921565516eec603ac23992f3accc76e57e986ff60174d396e2`、内层清单、云档渲染与 IAM 契约预检通过；预检不打开业务数据库、不 rollout。真实执行 D1 `check` 时，“配置 hybrid／实际 Local”被准确拒绝；本地五项匹配配置与错 Primary／非法配置拒绝、发布包测试通过。模板未安装或激活，不能描述为完成重启持久化。

以临时 unit／非敏感环境文件把模板路径绑定到现有候选 Guard，`systemd-analyze verify` 返回 0、夹具无诊断；主机既有 snapd 的 `RestartMode` 未识别警告单独记录，不算作本模板警告。没有启动 unit、调用 boot、变更 iptables 或重启 Docker。首次提取保留了构建机 UID 501（ECS 无此账号）；只对校验过的精确 `test.44` 候选目录修正为 root 所有，内容摘要不变，主／恢复栈不受影响。后续提取规范使用 `--no-same-owner`，启动钩子与配置必须由 root 控制。

## 实际 STS 到期与刷新

在恢复副本 `m8` 的官方 SDK 中强制 IMDSv2，取得实际 900 秒上传 STS。对上述已存在合成对象，禁止覆盖 PUT 在到期前返回 409；按真实 UTC 等待到期加 10 秒后，旧凭证返回 `403 SecurityTokenExpired`。再次取得较晚到期、不同 AccessKey ID 的短期 STS，PUT 返回 409，再读取大小／摘要一致。没有注入时钟、关闭鉴权、覆盖对象或增加 Version；源／恢复 12 个 Version 行和主库完整行保持不变，额外容器停止。

最初 SSH 前台等待未取得终态，未计为通过；第二次由独立后台进程完成，并保留 `/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/sts-expiry-repeat.jsonl` 的通过终态。此处证明上传 STS 到期拒绝与重新签发，不证明 ECS 基础身份在数小时跨度内的自动刷新。

## 正式切换前仍需完成

1. D1 云档、候选 IAM 契约预检和配置／运行时错档拒绝已通过；仍需主环境冷备份及分批切换验收。正式主环境仍使用旧 Local 镜像，不能直接改变量造成旧引用不可读。
2. 专用出口与 Guard 启动模板已随包；仍需维护窗口内激活、真实 Docker／主机重启与隔离复验。上传 STS 的真实到期拒绝与刷新已通过；ECS 基础身份的长时自动刷新仍未验。
3. 自定义 Files 域名的 SDK CNAME 适配与私网控制链已通过；剩余正式页面端到端验收，不能把内网地址返回给浏览器。
4. 在正式公网 Nginx → Gateway → IAM → Evidence 控制链上，以真人浏览器验证上传、取消清理、错误摘要、断网重试、授权下载和相邻权限拒绝。
5. 单份合成对象的离机误删恢复已通过；仍需实现完整 OSS 字节保护、保留和定期恢复策略。不能将夹具演练推广为全部文件均已有备份。

标准 Web 的 OSS 静态制品分离、Project／Contract 页面真实业务操作与 C8 缺口仍按既有计划执行，不由本批后端适配证据替代。

## 授权停止还原演练栈

2026-10-01 17:28 左右，按用户“允许停止 restore20 演练栈”的授权，只对 Compose project `omsbizfoundry-restore20` 的 17 个长期容器分批执行 stop。17 个容器均正常退出（exit 0）；主站 17/17 healthy，启动时间均早于此次停止，没有重建主站。28 个命名卷、全部已声明 bind 来源及 Compose 配置仍存在，Project 冷备份仍在；没有执行 remove、down、prune、删卷或数据库恢复。演练容器的重启策略均为 `unless-stopped`，后续维护不应主动将其启动。

首个停止脚本已输出 17/17，但末尾校验失败，不能将该运行记为完整通过。独立只读后检通过并保存为 ECS root-only `restore20-stopped-postcheck.json`。连续两次 inspect 发现 7 个容器挂载列表顺序不同，但挂载集合相同；原脚本按列表顺序比较存在缺陷，已改为规范排序并增加操作前快照。8 项无 Docker 单元测试通过，覆盖精确目标、已有证据、缺服务、主站不健康、ID 重叠、失败后禁止重跑及真实挂载变化拒绝。修正版停止脚本未再次对已停止栈执行；原运行未保存前快照，因此不声称已取得逐项挂载的精确前后比较证据。

停止前 34 个主／演练容器的 healthcheck interval 均为 2 秒；短时 vmstat 采样 CPU idle 约 1%，load average 为 10.44／10.68／11.32。17:30 停止后两个一秒采样 CPU idle 为 65%／52%，load average 回落至 5.51／7.70／9.66。它支持停止双栈演练可缓解当前压力，但不是有业务负载的 D1 容量验收，也不能据此判断单套 2 核 2 GiB 足够或不足。没有改 healthcheck、云安全代理或主站资源限制。

复现入口：[`acceptance/evidence-oss-ecs`](../../acceptance/evidence-oss-ecs/README.md)。首批只执行脚本／部署验证，未修改应用 Go；后续兼容批次修改了 Evidence 存储路由、备份、配置及文件页，并运行上述专项测试。两批均未运行全仓所有应用测试。
