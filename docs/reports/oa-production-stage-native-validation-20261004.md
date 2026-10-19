# 原 OA 方案阶段：原 UI 与独立原生事实

2026-10-04 UTC（最终复测于北京时间2026-10-05 00:07结束）。正常 Workbench 的原方案 UI 经真实 Nginx→Gateway→正式 IAM→独立应用／PG18，**16个有限场景通过、零页面脚本错误**；五所有者逻辑空目标恢复及原 UI 继续另有实际证据，安全回执已封存。不是完整 OA／真实 D1／公网验收；Main 零发布／零 Grant。

问题 A → 可验证结果 B：旧单成果槽和空页不能验证原阶段多文件业务 → 在正常 Workbench 的原方案面板／抽屉中完成多主文件、可选校审单、评分、真实文件访问、移出留史和失败原命令续办，事实只由 Production／Evidence 各自拥有。

## 原操作与边界

只读参考提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`。复用 `SchemeStageFileManagement`、`StageFilePanel`、`SchemeStageEditModal`、`ProductionStageFileField`，三个 CSS 逐字保留；不重写界面风格或复制共库后台。原文件表单以 Project＋Stage 共享，不能给四种类型重复建资料；[逐动作对照](../specs/assembly/oa-production-stage-original-operation-map.md)和[BF-106 规格](../specs/assembly/oa-production-stage-materials-v1.md)记录原请求的非原子保存、旧 GET 状态写入及目标适配差异。

Production 显式选装 `reference-oa-stage-materials-v1`，持久化 `reference-oa-stage-set-v1@2` 共享方案集合。它的 completed 仅表示方案集合齐备，不是完整生产／项目完成。四类型共用一份可写真相；旧活动 version 1 任务真实返回并拒绝并排创建，不自动转换或取消。Base version 1 的默认、JSON、命令摘要及历史保持，旧资料消费者不能表达新集合时失败关闭。

方案资料、评分、ending、history 和 command receipt 在 Production 单事务提交；字节与 File／Version 由 Evidence 拥有，用户上传／下载分权，Production 用单 `metadata.read`＋精确 File 工作负载复验。不嵌入 OSS SDK、长期密钥、签名 URL 或物理删除。Project／People 不被 GET 或阶段写入反向修改。

## 已运行的 PG18 与回归

- 真实 PostgreSQL 18：database 1／含子1、Production design 26／含子69、HTTP 11／含子20通过。HTTP 另有2项 skip：未选的旧私有进程探针、未配置的旧 material-reference 正式 IAM fixture；不记通过，也不替代新阶段的真实正式 IAM 浏览器层。
- 覆盖 schema 2→3 的旧 version 1 事实保持、整批原子、精确评分、最后主文件移出、同命令／不同命令并发、跨租户、旧新任务互斥、immutable guards、Production 单库空目标恢复及原命令继续。
- 批内顺序保存 immutable `batch_position`，按 revision＋位置读取；两主文件／两校审单和后续追加的真库回读、历史命令重放顺序稳定。不能用同批相同时间戳＋随机 UUID 判断“最新”。
- 完整前端／Workbench runtime 377项零 skip、Production／Workbench普通 Go及vet、design／HTTP／config定向race、TypeScript／Vite、正式IAM manifest／精确 workload policy preflight、37项profile guard／D1升级选择mock、静态边界与负例自检均实际通过。race不是PG18 integration race，mock不是实际阶段D1恢复。

真库输出位于私有 `/tmp/oa-stage-native.emrujh`；夹具256MiB、0.5CPU、无发布端口，已停止。源码、备份、失败回执保留，未改 Main 或用户数据卷。

收尾实际独立Workbench启动又复现旧二进制在缺People选装时仍接受阶段开关，与D1／页面四所有者条件不一致。已补启动门禁；默认off、完整四所有者on、缺People拒绝、未知profile拒绝和旧缺陷复现共5项真实进程检查通过。它们只验证启动／选择，不冒充业务链或实际D1；[启动回执](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/profile-startup.json)保留新旧二进制及源码摘要。最终16场景已用修复后的同一Workbench二进制重新运行通过，旧16场景保留为被取代的回执。

## 正常 UI、恢复与封存

最终正常 Workbench 浏览器16个场景通过，使用7个真实独立进程、正式 IAM、远端隔离 PG18／Nginx 的 SSH 回环链，不是 API／目录内存替身，也不是公网 TLS 或实际 D1 脚本运行：

1. 原空面板只读，无 Work Item／资料创建；原必填提示和取消零写。
2. 两主文件、一校审单、0分，一份共享 Work Item；刷新原 File／Reference ID 保持。
3. 原面板预览与下载按钮经真实 Evidence 短期授权返回原68字节 PNG，下载名保持。
4. 同批追加两校审单和88.5分，刷新仍显示最后选择的最新校审单；编辑器保留全部3份。评分留空／无新增文件保存零写，不清除原分。
5. Production 已提交而浏览器丢弃回复：reload继续原 command／载荷，零重复字节、业务 revision 和回执。
6. 原确认取消零写；确定移出只追加一个 ending，Evidence 原字节与历史保留。
7. 第二份 Evidence complete 故障：Production零部分写；reload继续已保存原 session／receipt，零字节重传，恢复后原面板真实显示两份新增文件。
8. 实际停止 Evidence 进程：登记503零 Production写，页面显示加载失败而非新建空资料；恢复后继续同一被拒命令，成功后重放零写。
9. 正式 IAM 错 Project403、未选已知阶段422、非法阶段400，全部零事实；撤销写权后原 UI 只读，伪造写403。
10. 五所有者空目标恢复、原命令重放零写、原 File字节及正常 UI刷新；原表单继续90.125分，只追加一个 revision。Project／People业务表摘要保持，页面0 error。

最终 Production为1个共享集合、9条资料引用／1条 ending、5条评分历史、8个成功命令回执；这是私有验收事实，不是 Main 记录。最终有5个主文件和3个校审单，移出的原引用没有物理删除。已实际查看原800px抽屉和失败续办后的正常列表；不是仅验证接口能返回JSON。

五所有者逻辑恢复单独标注：IAM／People／Project／Evidence／Production pg_dump→五个空数据库，逐表 canonical 摘要相同；本地字节复制到空目录并核对路径／SHA，继续使用保存的原签名 key 文件。这不等于 D1 联合 keys 归档恢复、旧会话失效或 clean-host 交付。最终私有 `/tmp/oa-stage-suite.IPs5fa` 保留源码／日志／逻辑备份及字节，配置与密钥不进入仓库证据；7个本地进程、远端PG／Nginx及测试Chrome已停止，Main容器ID／镜像保持。

安全证据：[原生16场景](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/native-browser.json)、[真库](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/pg18.json)、[检查](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/checks.json)、[67份原来源／候选摘要](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/provenance.json)。另[原UI-only13场景](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/ui-only.json)明确为内存端口，不与原生层拼接冒充完整链。实际查看了[正常原抽屉](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/normal-original-stage-upload.png)、[部分上传续办后的原列表](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/normal-original-partial-upload-recovered.png)、[恢复后原列表](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/normal-original-restored-stage.png)和[900px小视口](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/original-stage-small-drawer.png)。原3份CSS逐字一致，原OA目录零写。

复演入口：`verify-stage-native-browser.mjs`（必须 `OA_STAGE_RESTORE=1` 和新 `/tmp/oa-stage-suite.<mktemp>`）、`verify-production-stage-ui-browser.mjs`、`verify-stage-material-remote-pg.mjs`、`verify-stage-profile-startup.mjs`及`collect-stage-native-receipts.mjs`，均在 `acceptance/oa-initiation-repair/`。封存器依次接受原生浏览器、UI-only、PG18、启动检查四个输出目录，并核对最终原生Workbench与启动检查修复二进制摘要相同。原生驱动只创建带任务标签的私有环境，不接受Main目标；远端需既有SSH和固定PG／Nginx镜像，Go构建限`GOMAXPROCS=2`／`-p1`，Chrome串行并关闭。本次本地DockerDesktop／VM全程未重启；另结束22个已核对Docker残留助手，约378MiB RSS，无卷／备份删除。

## 发现的问题与仍未完成

已修复正常工作面 lazy mount 捕获空 root、原摘要选第一份校审单、未知操作恢复后 UI 缓存未失效，以及同批随机 ID 排序。真实全量 PG 又发现移出未注册阶段返回 NotFound；补为 StageInvalid，失败与后续通过分开保留。另有目录 Scope、toast 定位、驱动过短超时及错误预期状态的夹具失败，不放宽业务授权／校验。

未完成项保持明确：本 profile 的真实 D1／keys／旧会话恢复与历史 Main 升级；同项目公网精确授权／匹配发布；全角色／正式双租户；原500MB与当前200MiB的容量差异、更多类型／大文件／OSS；后续原阶段、类型进度／适用性、变更竣工、试验、报告、批复审计、外委、资料统计和全站连续旅程。旧 Base 活动任务到共享集合的显式过渡尚无验收，不能用新项目绕过这个发布门槛。旧Production资料来源端口对v2失败关闭已有真库反例，Project v1消费端的单relation唯一／100项规则尚不能表达整批集合；版本化来源及消费兼容须另补，不能以当前方案UI代替项目资料投影验收。

后续不原位扩大已注册 definition version 2：先明确新版本／过渡，保持原事实 ID、revision、引用、命令与历史可追溯，再逐一复用原初设／施工图等组件。整体架构结论仍为 `insufficient-evidence`，不以本方案切片代表完整 OA 或长期跨客户升级成功。

收尾只读核对 Main Production 存在3个 `design-v1@1 / in-progress` 任务、仍为schema2且没有新集合表，不是空库；对已有旧活动任务的项目，开启共享新集合仍会按唯一真相规则拒绝并排新建。因此旧活动任务的显式过渡是实际发布门槛，不是可用新验收项目避开的文档待办；本次聚合检查未判断某个指定项目的归属。后续施工图完整抽屉、变更／竣工完整列表及620px上传／替换路径已只读拆解，未据此宣称目标实现；[追加源码摘要](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/later-stage-source.json)与原commit一致。[资料消费边界回归](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/material-consumer-boundary.json)为Project 3项／含子20和Production 6项／含子43、零skip，仅验证旧逻辑及类型化测试端口规则保持，不是新集合消费实现。更新后的[全站清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)仍保留全部未关闭项。[只读收尾](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/closeout.json)核对Main17健康／镜像与ID保持、远端任务夹具与本地Docker／测试进程全停、候选22文件／原来源67文件摘要一致、14文档本地链接无缺失。私有备份／密钥／卷没有删除。
