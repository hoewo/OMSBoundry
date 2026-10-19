# OA C8 Project 资料人类体验首批实现与低资源验证

日期：2026-09-29
结论：`c8-human-material-formal-entry-read-supported / dedicated-fixture-ready / browser-write-journey-and-release-gate-pending`

## 本批目标

在不启动完整 D1、不扩大 Docker 并发的前提下，把已经通过真实 IAM／PostgreSQL／独立进程验证的 Project → Evidence 资料组合放进 Project 人类工作区，并保持“元数据读取”和“下载授权”分权。

## 已实现

- Project 记录工作区新增“项目资料”页签，只在服务端 `allowed_actions` 包含 `project-workspace.materials.read` 时出现；
- “资源连接”可建立 `evidence-service/evidence-file` Link，资料页按 Project 调用 `/api/v1/projects/{project_id}/materials`；
- 页面逐段显示文件名、媒体类型、大小、SHA-256、建立时／当前 revision 及 `available`、`revision_changed`、`invalid`、`unavailable` 状态；
- direct 开发入口只展示元数据；Suite 入口中的下载按钮先请求 Evidence Service 的独立应用授权，再进入 Evidence 深链；授权失败停留在 Project，且不把失败解释为 Project 读取权丢失；
- Project 页面不展示 Bucket、Object Key、长期 URL 或存储凭证；
- 交互规格补齐资料建立、读取、来源变化、来源故障、Project 撤权和 Evidence 下载拒绝状态。

## 实际验证

零容器阶段串行执行：

```bash
cd backend/services/project-workspace
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
```

结果通过。页面和静态资源测试明确断言“项目资料”、Evidence 资源类型、`materials.read`、`/materials`、Evidence 独立授权及下载分权文案。嵌入 JavaScript 另经 Node `new Function(...)` 语法检查通过；交互灰度线框的 8 个状态均具有说明、画布、设备、组件和行为区块。

为避免把页面实现建立在失真的后端上，又使用浏览器保持模式串行重跑：

```bash
C8_BROWSER_FIXTURE=/tmp/oms-c8-project-browser-fixture.json \
C8_BROWSER_RELEASE=/tmp/oms-c8-project-browser-release \
acceptance/c8-material-composition/run-isolated
```

该轮仍只有一个 PostgreSQL 18.6 容器，后端四组 HTTP 断言再次得到 `200、403、200、200`，覆盖 current、相邻 Project、Evidence 停机和恢复；脚本退出后容器数为 0，临时浏览器凭证文件和释放哨兵均已删除。

## 首次本地低资源批次的未通过与边界

- 尝试使用真实浏览器受控点击时，macOS 处于锁屏状态，自动解锁失败；因此本批没有把真实浏览器点击记为通过；
- 未运行 Nginx → Gateway → IAM 正式 D1，因而没有实测 Suite 路径中的 Evidence 下载授权按钮；
- 未验证双 Tenant、真实 Ubuntu、公网 HTTPS、间接业务来源 Evidence、指标或 WorkItem；
- 总目标进度继续保持 98%，不因页面代码和单元证据而提前增加。

下一低资源批次应在电脑可交互时只补 Project 资料真实浏览器点击；若环境仍不适合，再从“一个间接 Evidence 来源”与“一个 WorkItem 来源”中只选一项，不并行拉起完整组合。

## 公司环境低负载浏览器验收方案

本轮只复用已经运行的公司 D1 和本机现有 SSH 隧道 `http://127.0.0.1:18090`。不在 Mac 上启动容器，也不停止公司环境中的 Evidence、IAM 或 Project 进程。验收拆成两个顺序批次，前一批完成并记录后才进入后一批。

### 实际预检结果

2026-09-29 通过已授权 SSH 连接只读检查公司 D1：

- Compose 项目 `omsbizfoundry` 当前运行 12 个容器，均为 healthy，发布目录标识为 `v0.1.0-test.5`；
- 经真实 `Gateway → IAM` 登录和 `project-workspace` 应用授权均返回 `303` 并签发对应会话／应用 Cookie，Project 列表返回 12 条记录；
- 已部署 `/project/assets/demo.js` 不包含 `project-workspace.materials.read`、`authorizeApplication('evidence-service')` 或“来源已更新”状态文案；已授权 Project 详情的 `allowed_actions` 也不含 `materials.read`，同一 Project 的 `/materials` 请求返回 `404`；
- 因此公司环境当前版本早于本报告验证的 Project 资料页面实现。浏览器即使可操作，也不能用该环境证明当前资料旅程；必须先形成并部署包含 C8 资料页面的可追溯 release，或改用与当前代码一致的专用验收环境；
- 本次预检没有创建、修改或删除公司业务记录，没有重启服务，也没有在 Mac 上启动容器。

该结果把“浏览器被锁”之外的真实前置条件暴露出来：解锁只能恢复 UI 操作能力，不能弥补目标环境版本差异。以下批次步骤在版本对齐后执行。

### 升级前版本对齐准备与 ACR 阻断（历史步骤）

在 Mac 不启动容器的前提下，已串行完成以下准备：

- Project Workspace 与 Evidence Service 分别构建、推送为 `v0.1.0-test.6`，两者都以 ACR 返回的不可变 digest 写入镜像清单；其余当前选中镜像继续复用已验证的 `test.5` digest；
- D1 发布构建器已允许“5 个必需装配镜像 + Suite Manifest 实际选择的应用镜像”，稀疏发布包门禁与完整 17 镜像门禁均通过；未选应用不会因制作发布包而被虚构为已交付；
- 开发验证包 `omsbizfoundry-d1_v0.1.0-test.6_linux_amd64.tar.gz` 的外层 SHA-256、内层逐文件摘要、12 个选中镜像引用和 development provenance 均已校验；它不是 clean-HEAD 正式发布；
- 公司 ECS 切换前使用旧 `test.5` 发布脚本创建联合备份 `20260929T032057Z`，Contract、Money、People、Party、Project、Evidence、Business Engagement、IAM 与签名密钥的全部摘要校验通过，短暂停止的 Evidence 已恢复 healthy；
- 新发布目录和候选环境文件已经独立生成，旧发布目录与旧环境文件未覆盖。候选文件补入 5 个尚未选装应用的服务器本地随机数据库密码，以满足当前 PostgreSQL provisioner 的完整角色配置要求，但相应应用开关仍为 `false`；
- 候选 Compose 静态配置通过，并只列出 12 个 digest 固定镜像；当前服务仍运行 `test.5`，尚未发生容器切换。

当时目标 ECS 尚未保存 ACR 登录凭据，对新 Project digest 的首轮拉取明确返回 `pull access denied`。因此该轮发布在 Registry 认证边界处停止，没有用本地凭据文件绕过交互登录，也没有重启当前服务。后续已由人工完成目标 ECS 的 ACR 登录，并按兼容闭包升级 IAM、Project 与 Evidence；实际过程和结果见下文“公司 D1 实际升级、失败关闭与恢复”。

### 批次 1：正常资料旅程与下载分权

1. 先读取 `/health/ready`，确认现有 D1 可用；失败即停止，不尝试重启共享环境。
2. 从统一工作台登录，并通过工作台进入 Project Workspace；不得直接伪造身份 Header 或绕过 Gateway。
3. 选择专用测试 Project；如果没有合适数据，只创建带明显 `C8-BROWSER-VALIDATION` 标识的测试 Project 和 Evidence 文件，不修改现有业务记录。
4. 在“资源连接”建立 `evidence-service / evidence-file` Link，记录 Project ID、File ID 和建立时 revision，但不把会话凭证、存储位置或签名地址写入报告。
5. 在“项目资料”确认文件名、媒体类型、大小、SHA 摘要、建立时／当前 revision 和来源状态可读，同时页面与网络响应都不出现 `bucket`、`object_key` 或长期下载 URL。
6. 点击“前往 Evidence 获取下载授权”；浏览器必须先调用 Gateway 的 Evidence 应用授权，再进入 `/evidence/`。Project 的 `materials.read` 不能直接产生下载 URL。
7. 返回 Project 后，资料列表仍可读取，证明应用切换没有把 Evidence 下载权写回 Project。

通过条件：同一浏览器会话完成 `Workbench → Project metadata → Gateway Evidence authorize → Evidence`，且身份、应用能力和资源范围没有被扩宽。

### 批次 2：非破坏性故障与恢复

1. 使用专用测试 Link 填入过期的 Evidence revision，确认页面显示“来源已更新”，保留可解释的 revision 对比，并隐藏下载动作。
2. 将该测试 Link 解除后按当前 revision 重新建立，刷新页面后状态恢复为“已读取”；不得直接修改 Project 或 Evidence 数据库。
3. 对“Project `materials.read` 被撤销”和“Evidence download 被拒绝”使用专用测试 Subject；前者应使资料页签按新的 `allowed_actions` 收敛，后者只能使下载失败，不能清空 Project 可读元数据。
4. 共享公司环境不执行 Evidence 停机。`unavailable → recovery` 继续引用已完成的单容器隔离运行证据；若以后需要浏览器级 unavailable 验收，必须在专用环境或维护窗口执行。

通过条件：stale 可以经显式重连恢复；Project 撤权和 Evidence 下载拒绝互不冒充；共享环境没有因验证发生服务中断。

### 证据记录规则

- 浏览器截图只保留页面状态、Project/File 测试标识和可公开 revision；裁去 Cookie、Token、密码和签名 URL。
- 报告分别记录“真实浏览器已观察”“自动化测试已证明”和“共享环境未执行”三类结果，不用单元测试替代真实点击，也不把隔离环境停机写成公司环境停机。
- 每批结束后检查本机没有新增监听进程或容器；公司环境只保留显式命名的测试记录，清理动作必须可追溯。

## 公司 D1 实际升级、失败关闭与恢复

ACR 登录完成后，目标机串行拉取并校验 Project／Evidence `test.6` digest。首次候选只提升这两个应用，实际升级没有通过：

1. 旧 IAM `test.5` 读取当前 Capability Manifest 时返回 `json: unknown field "capability_namespace"`；这证明 Manifest 的解析者也是发布依赖，不能因为 IAM 业务代码未改而继续复用旧镜像。
2. 升级脚本已经对 Project 执行前向 migration。尝试拉起旧 Project `test.5` 时，它返回 `database migration count 7 does not match expected 3`；严格 schema 门禁正确阻止了旧二进制连接未来 schema。
3. 没有执行 down migration、手改 migration 表、覆盖历史或清理数据库。入口、IAM 与 Evidence 先恢复到旧版；Project 保留能识别新 schema 的 `test.6`，形成有明确记录的临时混合恢复状态。

这次失败没有被记为“升级通过”。它形成 [BF-077](../decisions.md) 与 [ADR-0035](../adr/0035-d1-release-upgrades-use-compatibility-closure.md)：D1 发布集合必须按契约解析者与 migration 读者形成兼容闭包，数据回退只能把升级前联合备份恢复到新的空目标。

目标机后续补入并推送兼容 IAM `test.6`。为控制 2 vCPU／2 GiB ECS 和开发机负载，恢复按以下顺序逐项执行：

- 核验联合备份 `20260929T113133Z` 的 `identity-access.dump`、七个应用 dump／ZIP、签名密钥和 Suite Selection 共十项 SHA-256，全部 `OK`；
- 运行 IAM migration，结果为 `000005_explicit_capability_namespaces.sql / current`，只重建 IAM 服务并等待 healthy；
- 运行 Evidence migration，结果为 `000001_evidence.sql`，只重建 Evidence 并等待 healthy；
- Project 保持 `test.6`，不再尝试旧二进制直连新 schema；
- 执行候选发布的 `d1 check` 和 bootstrap capability grant。

最终三个目标摘要为：

- IAM `sha256:468b6484dd02deabfc56a6ba3bc1dde6d1250c18feeca2beb42dc5242329d610`；
- Project `sha256:daa0554e338e472e0f192d4a04325495e9fe0b2b59a67c51a0f2b0e9bf331e47`；
- Evidence `sha256:1257a43806a94ca78056cf7ea1afd1b504a8fbacb2ffa9ace16444ff23de9d28`。

12 个长期容器全部 healthy；未涉及的 Gateway、Nginx、Workbench 和其他业务应用没有重建。本机 Docker 容器数始终为 0。升级后再次执行 IAM capability 定向单元测试和 D1 稀疏／完整发布包门禁，均通过。目标机资源快照显示 1.7 GiB 内存中约 901 MiB available、Swap 仅使用 21 MiB；本轮没有并发重建整套服务。

针对这次实际故障，候选 IAM 新增无数据库 `validate-contracts`，D1 又新增 `preflight-upgrade` 并把它放到 `upgrade` 的联合备份和 migration 之前。它会用候选 IAM 解析包内全部 Capability Manifest、Assignment Policy 与 Workload Policy。定向 Go 测试和 `d1-upgrade-selection-test` 均通过；模拟旧解析器负例在任何 backup 命令之前停止。该门禁直接防住本次“新 Manifest + 旧 IAM”，但完整 migration／Gateway／Workbench 消费者账仍是待完成门禁，不能据此把 ADR-0035 全部自动化标为完成。

## 真实入口只读验收

在目标 ECS 的 Nginx 唯一宿主入口上使用真实管理员会话执行不写业务数据的读链，敏感凭证只从目标机受限环境读取，未输出到报告：

| 检查 | 实际结果 |
| --- | --- |
| `/health/ready` | `200` |
| IAM 登录 | 用户 `admin`，成功 |
| Gateway 授权 Project | `303` |
| Project 列表 | `12` 条 |
| Project 详情动作 | `project-workspace.materials.read` 恰好存在一次 |
| Project `/materials` | `200`；当前所选既有 Project 为 0 个资料分段 |
| Project 静态页面标记 | `materials.read`、Evidence 独立授权、stale 文案三项均存在 |
| Gateway 授权 Evidence | `303` |
| Evidence 页面 | `200` |
| IAM 注销 | `204` |

该结果当时只支持 `c8-project-material-formal-entry-read-supported`，还不等于完整人类旅程通过。后续章节已经通过正式 API 为专用 `C8-BROWSER-VALIDATION` 数据完成 Link、Version 2、stale 与重连；浏览器现在只需观察资料展示、Evidence 独立下载授权和权限分权。共享环境不执行 Evidence 停机；该负例继续引用隔离最小运行证据。

## 专用浏览器验收数据

2026-09-29 再次通过公司 D1 的正式 `Nginx → Gateway → IAM → Project／Evidence` 入口执行只读清点，确认先前的数据准备已经成功，而且没有因重试产生重复记录：

| 类型 | 测试标识 | ID | revision／version | 状态 |
| --- | --- | --- | --- | --- |
| Project | `C8-BROWSER-VALIDATION 20260929T121515Z` | `c9cefbce-def2-4de1-a7f4-56e2f71dd271` | `revision 1` | `draft` |
| Evidence | `C8-BROWSER-VALIDATION-20260929T121515Z.pdf` | `d0d49592-df01-450c-aed6-e396d6149b27` | `version 1` | `available` |

Evidence 文件为 93 bytes，SHA-256 为 `356083ab988d62832d0d84e25acd055cb58877cff97e8475f09f53322503430a`。清点时 `/health/ready`、IAM 登录、Project 授权和 Evidence 授权依次得到 `200、303、303、303`；远端 12 个长期容器全部 healthy，本机 Docker 容器数仍为 0。上述记录只用于 C8 浏览器验收；后续 Link、stale 和重连都必须通过正式 API／页面完成，不直接修改数据库。

在统一工作台仍等待人工输入 IAM 密码期间，又通过同一正式入口对上述专用记录执行了可重复的机器链路，不触碰既有业务记录：

1. 先读取 Project 详情和 `/materials`，确认初始 `revision 1`、零条 Link、零个资料分段；
2. 以 `expected_revision=1` 建立 `evidence-service/evidence-file` Link，关系键为 `c8-primary-material`；返回 Link ID `dfeb1acf-51be-4347-ab3b-5b0a36f5d9be`，Project 前进到 `revision 2`；
3. 再读 `/materials` 得到唯一 `available` 分段，业务来源建立时／当前 revision 和 Evidence 建立时／当前 revision 均为 `1`，文件名、媒体类型、93 bytes 大小和 SHA-256 与 Evidence 事实一致；
4. 响应没有 `bucket`、`object_key`、`download_url`、`upload_url` 或 `signed_url`，只返回 `/evidence/?file_id=…` 深链；
5. Project 授权后 Cookie 集只有 Suite Session、IAM CSRF 和 `oms_cap_project_workspace`；再次经 Gateway 授权 Evidence 后才新增独立的 `oms_cap_evidence_service`，Evidence 页面返回 `200`。Project 元数据读取没有顺带获得 Evidence 下载能力。

这证明正常 Link 和分权机器链已在真实 D1 成立，但不替代真人浏览器观察。浏览器仍需确认页面动作、视觉状态、应用切换和 stale／重连；报告继续把这两类证据分开。

当前结论更新为：`c8-human-material-formal-entry-read-supported / dedicated-fixture-ready / browser-write-journey-and-release-gate-pending`。

## 正式 stale 旅程暴露的 Evidence 缺口与 test.7 修复

准备对上述专用 Link 执行 `available → revision_changed → relink → available` 时，没有继续使用夹具或直接改库。代码与 migration 审计发现：Evidence 虽然拥有 `File + Version` 模型，但 `upload_sessions.version`、`file_versions.version` 和公开创建 API 都只允许 Version 1；稳定 File reference 也固定读取 Version 1。由此，旧运行实验的 stale 只能证明 Project 消费逻辑，不能证明真实产品可以产生同 File 修订。

本批据此接受 [BF-078](../decisions.md) 与 [ADR-0036](../adr/0036-evidence-files-use-stable-identity-and-immutable-monotonic-versions.md)，并完成：

- `POST /v1/files/{file_id}/versions/upload-sessions` 由服务端为已有 File 分配单调、不复用的下一 Version；
- 同一 File 只允许一个活动追加会话，失败版本号不回收，已确认 Version 保持不可变；
- 稳定 reference 返回最新已完成 Version，显式版本 reference 仍返回原 Version；
- Access Grant、Local Data Plane 和备份模型支持任意正版本；
- Evidence 文件库增加“上传新版本”，Suite 路径复用 Gateway／IAM HttpOnly 能力 Cookie，直连模板路径才显示人工 Token；
- OpenAPI、领域／HTTP／备份测试和 10 步 Evidence 验收同步更新。

低资源实测只使用一个 `0.5 CPU / 384 MiB` PostgreSQL 18.6 容器；Evidence 全量单元、领域／HTTP 真库集成和验收程序通过，容器随后删除，本机回到 0 个运行容器。备份恢复 integration 因其脚本固定依赖另一套 Compose `postgres` 服务而没有在同批启动；用例已扩展为两个 Version／两个对象，需在后续单独恢复批次或 D1 联合空目标恢复中实际通过。

Evidence `v0.1.0-test.7` 已推送，Linux AMD64 digest 为 `sha256:4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf`。第一次 `preflight-upgrade` 确实因目标机 root Docker 的 ACR 登录过期而在拉取阶段停止，未创建新备份、未运行 migration、未替换容器；重新登录 ACR 后继续执行，而不是绕过预检。

## 公司 D1 正式多版本、兼容闭包与完整升级

远端后续分三次收缩问题，没有在失败后继续盲目推进：

1. `test.8` 先加入 IAM `validate-contracts`，在数据库和备份变更前解析全部候选 Capability／Assignment／Workload 文档；旧 one-shot migration 容器会被 Compose 复用的问题暴露后，现场删除遗留任务容器并恢复健康；
2. Evidence 联合备份完成后，旧脚本用 `compose start evidence-service` 恢复会重新求值已经退出的 migration 依赖。修复后改为 `up --detach --no-deps --no-build --wait evidence-service`；完整备份 `20260929T131201Z` 的全部摘要通过，包含 9 个 File、10 个 Version、10 个对象。较早的 `20260929T130746Z` 因缺少最终 `checksums.sha256` 明确不可用于恢复；
3. 新 Suite Manifest 的 `metric_projection` 又被旧 Gateway／Workbench 严格拒绝。这证明兼容闭包必须包含所有 Manifest 消费者，而不只是 IAM。两者按当前源码低并发测试通过后分别发布为 `test.10`。

最终公司候选为 `v0.1.0-test.10`，受影响镜像摘要如下：

| 组件 | 摘要 |
| --- | --- |
| Identity & Access `test.8` | `sha256:69806b6fbf055dcdbb50dcc93a135149e0a523684213ef4607bcc1c21a006266` |
| Evidence Service `test.7` | `sha256:4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf` |
| Project Workspace `test.6` | `sha256:daa0554e338e472e0f192d4a04325495e9fe0b2b59a67c51a0f2b0e9bf331e47` |
| Gateway `test.10` | `sha256:d850dfefe8be4e3b80ffbab4335492723126e8f0d061c3069f80c8fce5c70874` |
| Unified Workbench `test.10` | `sha256:808ee99f26f49e596b2aab89f5a41b58f98e18528fc340c1d123e942d467cdfe` |

正式 API 旅程没有直接修改数据库：

- Project `c9cefbce-def2-4de1-a7f4-56e2f71dd271` 从 revision 2 前进到 4；
- File `d0d49592-df01-450c-aed6-e396d6149b27` 保持稳定 ID；Version 1 摘要为 `356083ab988d62832d0d84e25acd055cb58877cff97e8475f09f53322503430a`，Version 2 摘要为 `0b1e29c293574f0fbd9c0d2195978e1e004789f6720a1e1e50332155145f4e75`；
- 资料状态依次为 `available@v1 → revision_changed@v2 → available@v2`；
- 旧 Link `dfeb1acf-51be-4347-ab3b-5b0a36f5d9be` 被正式重连为 `3da2f74a-fce7-4e10-b7d1-18fc8426807b`。

随后完整运行 `d1 upgrade`：升级前备份 `20260929T132146Z` 校验有效，脚本保留命名卷执行 `down --remove-orphans` 后从候选制品重建，全部 migration 退出 0，12/12 长期容器 healthy，`d1 check` 通过。稳定 Version 1、最新 Version 2、Project revision 4 和 `available` 资料状态在完整重建后保持。

因此远端多版本和 stale／relink 已通过，结论更新为 `c8-project-evidence-version-drift-d1-supported`。但这仍不是完整人类体验通过：真人浏览器中的文件下载、独立 Evidence 能力授权、权限分权视觉反馈，以及已校验备份的独立空目标恢复尚未完成；间接资料、指标和个人工作也未全部进入正式 D1。报告继续保持 `insufficient-evidence`。
