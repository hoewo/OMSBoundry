# OA 招投标多附件候选验证

后续状态：本文保留夜间候选及 01:10 暂停时的实际结果，不改写失败或未运行项。2026-10-03 下午已直接复用原 OA 组件、发布 BE 000002／Workbench `.20261003.1`，真实 local 文件链和 reload／原命令继续通过；最新部署、所有权与待办以[后续报告](./oa-reused-bidding-ui-validation-20261003.md)为准。

日期：2026-10-03。BF-095；状态：候选后端、Workbench 自动化及独立启动通过，未部署 Main，未通过公网附件操作。规格见[多附件兼容切片](../specs/assembly/oa-bidding-material-collection-slice.md)。

问题 A → 可验证结果 B：原 OA 每类材料允许多份、逐份移除，旧 BE 每类仅一份 → BE 自有材料集合兼容追加和单份结束引用，旧命令、历史和 Project 消费保持不变。

## 实际边界变化

只演进 Business Engagement Base 的材料集合能力：既有 v1 materials 路由新增明确 `append`／`remove_one`，追加 migration `000002_material_collections.sql`。旧 `put` 整类替换、旧 `remove` 整类移除及旧幂等 hash／回执不变。requirements 保留单份代表字段并可返回当前多份数组；全量 materials 继续包含已结束引用。

BE 仍只拥有业务引用；Evidence 仍拥有字节、版本、上传完成校验和授权地址。单份移除不调用 Evidence 删除。Project 既有材料投影本来允许同 relation_key 的不同 File／Version，本轮只补消费者测试，不改 Project 产品代码、schema 或所有权。没有新增小系统、共享表、跨库事务、IAM Grant 或客户内核分支。

## 已实际运行

- 本地 BE 全量 Go tests、vet 和 integration-tag vet 通过；Project `internal/references` 检查通过。未运行的本地真库检查不算通过。
- 私有 PG18 批次 d 实际通过经营承接 17 项、数据库兼容 1 项、HTTP 22 项（包含子测试），零 skip。覆盖同类多份、重复拒绝、逐份移除、旧 put-remove、错租户／目标、100 条上限、并发单赢家、零部分写入及原生能力／scope。见[实际回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/d/postgres.json)。
- 数据库兼容测试先建立真实 000001 旧 schema、旧材料与旧 JSON hash／回执，升级 000002 并重复 migration 后业务行摘要不变；旧命令在升级及追加新材料后仍复读原回执、零写入。
- 真实 pg_dump → 自有空目标 pg_restore 保持五表 canonical facts、revision、hash、详情和投影；同命令重放零新增，恢复库可以继续单份移除，原库不变。这里是独立 BE 数据恢复，不是完整 Suite 恢复运行。
- 独立启动 i 使用候选 Linux 二进制、新自有库和正式 IAM 配置，七个匿名原生读写入口均拒绝，重启事实保持；见[启动回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/i/startup.json)。合计上限 0.4 CPU／352 MiB，无公开探针端口，结束后服务及私有 PG 已停止。
- 上述真库使用合成 Project／Evidence 引用验证 BE 边界，不冒充实际文件上传、真实 Evidence 依赖链或浏览器旅程。各批 Main 前后快照保持；restore20 未启动。
- 批次 e 加入同租户另一条真实承接记录及其有效附件 ID：用该 ID 在原记录移除被拒绝，两条记录完整 wire 详情不变；17／1／22 项再次通过，Main 保持，探针及私有 PG 已停。见[补强回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/e/postgres.json)。不以纯校验器模拟代替这条真库反向证据。

## 保留的失败和修正

批次 b 以 Go 内部对象 DeepEqual 比较幂等回复失败；检查改成完整 JSON wire 比较，不削减公开字段。后续 d 的同回执、详情和投影检查通过。产品保存逻辑没有为此修改；[原失败](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/b/failed-run.json)保留。

批次 c 的恢复用例误用 pgx 原始 ConnString，它仍指向最初解析的 postgres，而非 helper 随后设置的随机自有库，导致恢复后找不到 schema。修正为查询实际 current_database、验证自有前缀、构造源／目标地址并检查目标为空；d 真实恢复随后通过。[原失败](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/c/failed-run.json)保留，不能把该次恢复说成通过。

## 发布门禁和剩余工作

Workbench 候选另补材料原生命令状态机：仅当前已关联来源、选定方式的必需材料及原生 evidence.manage 可操作；追加前须读取 Evidence v1 精确 available File／Version、Tenant 和 revision，已登记版本及 100 条上限提前拒绝。单份移除按当前材料引用 ID，无 Evidence 删除或 Project 写入。未知写回复／后续原生读取失败重放原 command ID 和完整载荷，409 需显式重读；已有新集合可逐份展示和移除，旧单份响应保持只读。Evidence 正式适配器尚未接线，上传／查看／下载未实现；当前候选列表和确认交互也不是原 OA 附件界面等效验收。

本轮同时修复 BF-079 架构透镜的固定 Project 模板：候选会显示当前 BE 原生调用、Gateway／IAM 和引用／字节边界；新操作清除旧流程，迟到请求按操作 epoch 丢弃。仅受信管理员界面显示；浏览器解释按 Tenant／Subject 隔离，有界保存标题／说明／状态，不保存业务表单、路径、命令体或凭证，不新增服务、业务库或 IAM Capability。存储受阻仍保持内存解释，不影响业务保存。这是浏览器观察和架构说明，不是服务器审计或独占某一账号的新权限。

156 项 Node 检查实际通过、零 skip：28 项新材料、12 项透镜加 116 项已有检查；Workbench 全量 Go tests／vet、两份 JavaScript 语法和 diff 检查通过。材料负例首次四项失败是测试 reader 覆盖了注入的错误 ID／版本，修正为对原生回复字段逐项注入后通过，不修改产品校验来迎合夹具。独立原生 Workbench 二进制启动、停止、重启及四份嵌入 runtime 对源码摘要通过，匿名仅见登录、不可见透镜；见[启动回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/workbench-startup.json)。此为本机隔离进程、声明的 example Suite，未关闭鉴权、未运行受保护浏览器旅程或替代 Auth58 部署。驱动第一次在启动前被 macOS `/tmp`→`/private/tmp` 路径门禁拒绝；按规范化临时根校验修正后运行，不涉及产品或 Main。

候选 server SHA-256：`cc3c4db93d7008bbc547900db0fd295c162672a6b10ff511bcf39205f6fbfbe4`。Main 仍运行此前 `.11` 方式链版本，尚未执行本次迁移。000002 后旧二进制的 schema 门禁和单份假设不兼容，不能直接换回旧镜像；回退必须使用升级前数据库与原二进制的联合备份，不删新材料凑旧约束。

下一项是在同项目 OA 页接 Evidence 文件选择／完成校验 → BE 追加登记 → Project 资料复读，以及逐份查看、下载、移除。文件已完成而业务登记失败时必须保留明确 File／Version 和原命令供显式继续，不自动重传、物理删除或伪称成功。未知回复、错项目／租户、撤权、刷新及部分成功须单独验收；之后才做兼容制品、联合备份／空目标恢复、Main 发布和真实公网点击。

已发现下一接线门槛：现 Workbench 控制面 helper 只接受 `/api/v1`，Evidence 是独立 `/v1`；现页面 CSP 的 connect-src 仅 self，不能直接进行跨域 OSS 数据上传。必须显式接 Evidence 版本化适配器、复用 file-transfer-core 并按实际部署配置精确的上传／预览来源，不能放宽为任意 URL、关闭 CSP、嵌入长期密钥或以另一套手写 OSS 流程替代。当前没有为此改变 Manifest、Provider 或公网安全配置；刷新后未登记完成文件的继续信息，以及原 OA 文件选择／预览／确认形态仍须完成。

公网架构透镜仍是旧版本，候选修复尚未发布／真人验收；B03 文件链、完整立项字段、经营成员、合同／财务／成本／奖金、分类型生产、资料统计、角色矩阵、OSS 前端分离和完整 Suite 恢复均未由本轮关闭。完整 OA 结论仍为 `insufficient-evidence`。

## 四小时暂停收尾

按用户“最多四个小时后暂停 check”的要求，本轮在 2026-10-03 北京时间 01:10 上限前收尾，不继续启动发布或新探针。现有密钥 SSH 连通；实际执行 k 的 `d1-active check` exit 0，随后读取容器状态，Main 17/17 healthy，只有 Main 的 17 个容器运行。d／e 测试、i 独立启动、私有 PG 和 restore20 均停止，未删除容器、卷或数据。见[只读收尾回执](../../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/closing-check.json)。这不是新的业务事实前后比对或公网点击证据。

收尾 check 的 BE 仍为 000001；Evidence 仍使用 `local`，不是已切换 OSS。因此上文 CSP／OSS 数据上传是后续启用 OSS 时的门禁，不能描述为当前 local Provider 的已复现上传故障。恢复工作时先完成 Evidence 版本化适配与原 OA 附件交互／部分成功继续，再分别验证声明的文件 Provider；备份、迁移及候选发布之后才进行公网同项目文件闭环。
