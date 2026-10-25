# OMSBizFoundry 开发进度

2026-10-26：完成 BF-266 的 P9 C0～C10 可恢复总入口和独立全链 verifier。总入口复用已有阶段，缺失 C9 时仍要求显式清卷确认，C10 等开发者回复时保留 `PENDING`；最终核对十一套固定制品、单一 project／Origin 和跨 C0～C10 checksum 绑定。离线完整复用、重算 checksum 后的 C9 断链与未确认恢复负例通过，脚本进入 D1 release bundle。未启动 Docker 或 OMS 常驻服务；真实 D1、真实浏览器 Console／点击与回滚记录仍待执行，P9 未完成，且未擅自增加未确认的 soak 时长。

2026-10-26：完成 BF-261 的 P9 C7 报销实际付款运行入口和低资源离线回归。C7 串联 C0～C6 checksum，分别为两个 Tenant 走 Money 受保护命令和审计 ops 建立本地兼容账簿主体／员工收款主体及精确 Person 映射，再由 Finance 确认 23456 分实际付款，验证 Money Obligation／Payment／Allocation 稳定引用、完整核销、员工 Open API `paid` 回读和跨 Tenant 404。脚本没有直接数据库写入，也明确不宣称 Party Directory 已统一；摘要漂移、缺失 HTTP 断言及重算 checksum 后注入 Key 的负例均被 verifier 拒绝，并已纳入 D1 release bundle。未启动真实全组合，工资实际发放与恢复仍待后续，P9 未完成。

2026-10-26：完成 BF-260 的 P9 C6 员工发票报销运行入口和低资源离线回归。C6 串联 C0～C5 checksum，使用两把既有员工 Key 各自上传一份不同的虚构 PDF 实字节，完成 Evidence 发票校验并冻结 File ID／Version，随后创建／提交本人报销，验证 `payable`／无需审批、账号脱敏、本人列表隔离、跨 Tenant 404 和零实际付款／零 Money。Key 与签名 URL 不进入命令参数或固定证据；摘要漂移、缺失 HTTP 断言及重算 checksum 后注入 Key 的负例均被独立 verifier 拒绝，脚本已纳入 D1 release bundle。未启动真实全组合，P9 仍未完成。

2026-10-26：完成 BF-259 的 P9 C5 员工 API Key 运行入口和低资源离线回归。C5 串联 C0～C4 checksum，为两个 Tenant 的既有 Subject／Person 各创建或复用一把默认 180 天 Key，验证自助列表脱敏与隔离、Open API profile 精确身份、Webhook 未开放占位及本人调用审计。一次性 Secret 仅写入证据外 `0700`／`0600` 恢复位置，证据扫描和独立 verifier 拒绝 Secret；首次、重跑、既有 Grant、摘要漂移、缺失 HTTP 断言及重算 checksum 后注入 Secret 的负例通过，并已纳入 D1 release bundle。未执行发票／报销业务写，未启动真实全组合，P9 仍未完成。

2026-10-26：完成 BF-258 的 P9 C4 财务申请运行入口和低资源离线回归。C4 串联 C0～C3 checksum、同一 Origin、两个 Tenant／管理员／Person，在最小 Finance 本人能力下分别形成一笔 `payable` 付款申请，固定无需审批快照、明确无附件确认、账号脱敏、本人列表隔离和跨 Tenant 404；未调用支付或 Money。首次、重复运行、既有 Grant、秘密扫描、摘要漂移及重算 checksum 后删断言的负例通过，并已纳入 D1 release bundle。真实全组合未启动，P9 仍未完成。

2026-10-26 BF-257：新增绑定 C0～C2 的 P9 C3 运行驱动和固定证据校验器；通过 Payroll 自有 ops 幂等导入明确非法律依据的六项零比例验收方案，两个管理员经同一 Origin 分别生成待核对、无需审批且未支付的常规工资表，并验证 C1 Person／C2 Compensation revision 快照、整数计算、Tenant 列表隔离与交叉 404。离线 Docker／curl 首次／重跑、既有 Grant 409、密码扫描、摘要漂移和重算摘要但 HTTP 矩阵缺失负例通过；脚本进入 D1 release bundle。未启动服务，未执行真实 D1、真实缴费规则、发薪或 Money 写入。

2026-10-26 BF-256：新增绑定 C0／C1 的 P9 C2 运行驱动和固定证据校验器；两个管理员经同一 Origin 分别取得 Payroll 最小能力，以稳定 Command ID 为 C1 Person 建立独立薪酬档案，并验证 Person source revision、整数最小货币单位、Tenant 列表隔离与交叉 404。离线 curl 首次／重跑、既有 Grant 409、密码扫描、摘要漂移和重算摘要但 HTTP 矩阵缺失负例通过；脚本进入 D1 release bundle。未启动服务，未验证真实 D1、缴费比例、工资生成或发放。

2026-10-26 BF-255：新增 P9 C1 双租户运行入口及固定证据校验器，覆盖 C0 绑定、第二 Tenant 创建／复用、同 Origin 双登录、双 People Grant、Person 幂等重放、两侧列表隔离、交叉 ID 404、Workbench 会话 Tenant 和秘密不落 argv／证据；离线 Docker／curl 回归及篡改负例通过，并进入 D1 release bundle。未启动容器，尚无真实 D1 C1 证据。

2026-10-26 BF-254：P9 审计发现 IAM／Owner 可处理多个 Tenant，但 Gateway 仍以 Suite Manifest 单一 Tenant 拒绝第二租户，会让“同域名双租户”在入口层失败。现已移除该错误白名单语义，Workbench 使用可信会话 Tenant；Gateway 登录和直接 IAM 登录成功后都清除旧 Capability Cookie。Gateway／Workbench 全模块测试通过，未启动容器。真实双租户 D1、跨 Owner／附件／导出负例和恢复仍待 P9，完成度不提高为整轮通过。

2026-10-26 BF-253：P9 C0 已要求正式 release metadata、Suite 版本一致、pull 模式和全量 digest-pinned image，并把 metadata 纳入固定证据；C0／verifier／说明同步进入 D1 release bundle。开发 commit 与 mutable tag 负例、bundle 完整性测试通过，未构建真实 19 镜像或启动服务。

2026-10-26 BF-252：P9 C0 现交叉核对 19 个运行容器的实际 Image ID 与各自配置镜像引用解析结果，避免标签更新后错报运行版本；证据 verifier 独立复核镜像、服务健康和唯一发布端口。mock 标签漂移与重算 checksum 后的语义漂移都被拒绝，未启动容器，真实 C0 未执行。

2026-10-26 BF-251：P9 C0 证据已改为固定闭集并由 SHA-256 清单覆盖，新增独立 `verify-evidence`；状态 schema version 修正为数值 1。mock 正例、readiness 篡改和额外文件拒绝均通过，未启动容器。清单不是签名，真实 19 服务 C0 仍待运行窗口。

2026-10-26 BF-250：P9 联合恢复指纹已补入 public sequence 的结构参数与 `last_value`／`is_called` 状态，避免只比对表事实却遗漏下一次自增写入位置。短生命周期、无发布端口的 PostgreSQL 18 实测证明 `nextval` 改变摘要，原始业务值未输出，容器已删除；真实十五库恢复与恢复后续写仍未执行。

2026-10-26 BF-249：D1 Restore 现于删除目标卷前校验新指纹的格式、唯一数据库名、表／行计数、摘要和 Suite 选择集合。新增 mock 把 Payroll 表数改成 0 并重算外层 checksum，仍在任何 `down --volumes` 前失败；同时保留无指纹旧备份恢复正例。该源码门禁降低误删风险，但尚未执行真实空目标恢复。

2026-10-26 BF-248：P9 新格式联合备份现在生成受 checksum 保护的 `database-fingerprints.tsv`；全选组合覆盖 IAM＋十四 Owner，记录表／行数量及 schema／事实 SHA-256，原始行先在 PostgreSQL 内散列，不把账号、密码哈希、API Key verifier、金额或业务内容写入证据。空目标 restore 在服务开放前重算，任何漂移失败关闭；旧备份仍可恢复。无状态 mock 的匹配／故意漂移两路和短生命周期 PostgreSQL 18 的 2 表／1 行／零敏感输出均通过，临时容器已删除。真实十五库恢复、Evidence 字节及恢复后业务复跑仍未执行，P9 结论不变。

2026-10-26 BF-247：P9 联合备份不再在公网入口开放时顺序 dump。D1 现在记录 Nginx 原运行态，关闭唯一发布端口、执行 0～30 秒有界 drain，再生成全 Owner／Evidence 检查点；`backup-consistency.env` 与全部对象一起进入 checksum。普通备份成功或任一 Owner dump 失败均恢复原入口；升级预备份成功后保持停写直到 Compose 替换。P9 mock 已覆盖顺序、异常 trap、非法 drain、Finance／Payroll dump、checksum 和空目标 restore 命令图；upgrade selection 与 release bundle 回归通过。未启动容器，真实联合恢复和恢复后业务复跑仍待执行。

2026-10-26 BF-246：P8 新增低资源真实浏览器验收。真实 Chromium 在两个随机回环端口加载生产 Feedback 宿主并通过 29 项断言，覆盖跨 origin 单 iframe、项目 115 运行配置形状、诊断 allowlist、提交／状态、未读、迟到响应拒绝、精确 source／origin、关闭／换票、退出清理、403 不可发现与 503 局部降级；其他业务按钮始终可操作。脚本退出已关闭浏览器和临时服务，未启动 Docker。该夹具不连接外部项目，项目 115 通知、有效凭证、双账号／双租户、附件／回复／单条已读和公网 Console 仍待一次真实窗口，P8／P9 状态不提高为完成。

2026-10-26 BF-245：P9 运行前审计发现 D1 把 `FEEDBACK_SDK_URL` 错放在 Bonus 环境而未交给 Workbench，且部分 Feedback 参数会延迟到容器启动才失败。现已修正配置归属，并在 `d1` 状态变化前成组校验首发 `apiKey`、项目 115、HTTPS／回环 URL、HMAC、通知布尔值及单／双 Key 轮换；实际 Compose JSON 正例和七组负例通过。另新增只读 P9 C0 入口与 mock 自测，真实窗口将先核对十四应用、十九服务、19／19 健康、镜像、入口与 schema，且不落盘原始 Compose／inspect 或 Feedback secret。本批未启动容器；P8 真实运行和 P9 全组合仍待执行。

2026-10-26 P9 装配源码门禁已完成，真实全组合与恢复待执行。审计发现 D1 的 Finance／Payroll 虽可健康启动，却未完整接入下游：现已为 Finance 增加独立工作负载身份、Evidence／Money 短期 Capability、401 单次换票和 D1 key lifecycle；Payroll 补上 Money 端点；选择器强制 Finance→Money＋Evidence、Payroll→People＋Money。静态 Compose、相关 Go test／vet、IAM policy、release bundle 与 P9 组合测试通过，未启动容器。P8 真实 Feedback、P9 连续旅程、联合恢复、digest 和浏览器证据仍待执行，不能标记整轮完成。[P9 装配源码报告](./reports/oms-v1-iteration-p9-composition-source-validation-20261026.md)

2026-10-26 P8 Feedback V2 源码门禁和本地真实浏览器宿主门禁已完成，外部运行验收待执行。Gateway 已提供受会话、CSRF 和 `feedback.use` 约束的运行配置，Workbench 已接入桌面／移动入口、单 iframe、未读、精确消息校验、退出清理和降级；D1 只接受环境秘密。Arckit SDK 独立本地提交已增加闭集诊断上下文和 OMS Origin。Go／Node／SDK 构建／release bundle 及本地 Chromium 29 项断言均通过且未启动常驻服务。项目 115 通知、有效 Key、双账号／双租户与外部真实跨域旅程尚未验收，未部署 D1／公网；完成这些门禁后才能关闭 P8。[P8 源码报告](./reports/oms-v1-iteration-p8-feedback-v2-source-validation-20261026.md)

2026-10-26 P7 本地完成，进入 P8。员工 API Key 已可经正式 `/open-api/v1/` 上传发票、建立／提交本人报销并查询结果；Evidence 完成有界格式／结构／危险主动内容校验与失败关闭清理，Finance 完成重复识别、草稿生命周期和本人资源授权，财务确认后 Money 保存唯一付款／核销。真实 TLS 五进程／四库验收 13／13 通过，故障恢复、危险 PDF、强重复、付款唯一性和撤权均有证据；相关单元、vet 与 PostgreSQL 集成测试通过。没有部署 D1／公网，下一步为 P8 Feedback V2，随后 P9 全组合／恢复／公网验收。[P7 报告](./reports/oms-v1-iteration-p7-invoice-reimbursement-validation-20261026.md)

2026-10-25 P5 本地完成，进入 P6。Internal Finance Requests 已扩展为正式报销／付款申请 Owner：明细与类别、收款和费用归属快照、Evidence 引用、无需审批快照、撤回 revision、敏感访问审计、逐笔／批量付款及 Money 稳定引用均已落库。Money 为申请建立稳定义务，保存实际付款、整单核销和超额未分配余额；批量部分失败不回滚成功项。双数据库／双进程隔离验收覆盖分次／超额、批量 revision 冲突、撤回重提、普通／敏感导出、账号遮罩和空目标恢复；Workbench 258 项 Node 测试通过。没有部署 D1／公网，Evidence 实时校验与员工开放 API 留待 P7／P6。[P5 报告](./reports/oms-v1-iteration-p5-finance-payment-validation-20261025.md)

2026-10-25 P4 本地完成，进入 P5。Payroll 已从薪酬档案扩展到正式工资执行：月度唯一期间、不可变工资行 revision、成员／薪酬／账户／缴费方案快照、整数五险一金计算、绩效／调整／外部个税、导入预览、核对、同步、作废／替代、调整批次以及逐人／批量发放。Money 新增工资交付契约，每个工资行一个稳定义务，多次实际付款分别登记并按剩余净应发核销，超额保留未分配；批量部分失败不回滚成功项。三数据库／三进程验收 14／14 HTTP 断言通过，四个受影响 Go module、256 项 Workbench Node 测试、Money 25 migration PostgreSQL 18 门禁通过。没有部署 D1／公网，正式缴费比例仍须按地区权威来源核验，跨应用恢复／导出留待 P9。[P4 报告](./reports/oms-v1-iteration-p4-payroll-execution-validation-20261025.md)

2026-10-25 P2 本地完成，进入 P3。新增 `Internal Finance Requests` 与 `Payroll` 两个独立 Go／PostgreSQL Owner：各自拥有 migration、版本化 API、IAM／Tenant／Capability／资源范围校验、幂等命令、不可变 revision／audit、Docker 镜像和备份／空目标恢复。真实 PostgreSQL 18.6 集成测试覆盖跨租户和直接篡改拒绝；隔离验收完成两个独立镜像的写入、重放、冲突、revision、分别恢复和恢复后续写。D1 可选 profile、路由、Suite Manifest、19 镜像发布／提升及恢复清单门禁通过，但未执行正式 D1／公网部署；业务仍只是草稿骨架。[P2 报告](./reports/oms-v1-iteration-p2-owner-skeleton-validation-20261025.md)

2026-10-25 P1 本地完成。新增可重复验收实际启动 IAM 与 People 两个进程及 PostgreSQL 18.6，在同一服务实例内建立 Tenant A／B，完成独立登录、能力签发、Subject↔Person、列表、跨租户 ID／Subject 拒绝、撤权和停用；随后将 IAM／People 分库备份恢复到两个新空库，确认 Tenant、稳定 ID、revision、Subject 状态快照不变并继续写入。12 个当时既有受保护 Owner 普通测试和代表性 PostgreSQL 租户隔离／搜索／分页／导出／附件路径通过，秘密模式扫描无命中。该结论仅是本地 P1，不表示 D1 或公网发布完成。[P1 报告](./reports/oms-v1-iteration-p1-multitenant-validation-20261025.md)

2026-10-13 BF-150：OMS 一期当前六项接入目标已闭合。`v0.1.0-test.oms-v1.2` 公网真实链使用同一幂等 run key 两次通过 Party→Project→上下游 Contract→双向 Billing→Money 回款／支付→六份 Evidence 上传、业务引用、短期下载和字节校验；金额分别为 800／268／320／96／220／120 万。Contract／Money Party runtime 已切到 Party Directory；完整分页 helper、真实筛选集合、本地筛选 CSV 导出和六 Owner 静态依赖边界均有自动证据。D1 check 与 17/17 health 通过。这里的“接上”精确指 Party、Project、Contract、Billing、Money、Evidence；People、BE、Production、Approval、Cost、Bonus、Design 属于完整 Suite 的其他独立应用，未被 OMS 一期调用且不应为了数量强接。详见[公网写入报告](./reports/oms-phase1-public-write-validation-20261013.md)。

2026-10-12 BF-149：OMS 一期 `v0.1.0-test.oms-v1.1` 已发布到 `https://oatest.antuinfo.cn/oms-v1/`。升级前备份、镜像摘要、17 服务健康、D1 schema 检查、正式资质单位 Party 建立与配置、IAM 登录、Gateway 五 Owner 授权和真实只读复读均通过；旧 `/oms/` 入口保持。下一开发项转为 Money 实际收付款、施工方应付与 Evidence 附件，不能把当前公网可访问误报为一期全部闭环。

2026-10-07 BF-121 本地 UI 候选：统计项目台账、公司两链费用矩阵／成本／明细、人员奖金展开及支付／开票独立导出接线；权限保留原 OA 49 项、7 组和全部模板，并按四层展示属性、Overlay／Policy、本地能力目录、会话草稿及导出。前端 206／206、TypeScript／Vite、来源 HTTP／契约矩阵、Workbench／vet、静态边界审计通过；浏览器内存来源夹具＋独立 Workbench 预览 24／24、页面错误 0。没有正式 IAM／源库／D1 验收、生产变更或部署；正式模板／临时授权写端口、项目结余口径仍待办，考勤／薪酬保持原系统未实现态，项目详情延后。[记录](./reports/oa-statistics-permission-ui-validation-20261007.md)。

2026-10-06 当前状态由 BF-120 重置：以 `AGENTS.md` 为实现完成度基线。已实现应用范围保持其摘要；Bonus Distribution 当前只确认领域核心与 PostgreSQL 18，HTTP／IAM、页面、D1、恢复和 Money 建议列为未完成。下方 BF-068～BF-119 的局部运行记录保留为中断实验／候选证据，未经当前代码、持久证据和干净版本复验，不计入完成状态。下一项不是继续旧“下一切片”，而是 R0 工作树／证据清点和 C1～C8 缺口矩阵；详见[当前基线与下一步](./status/current-baseline-and-next-actions-20261006.md)。

2026-10-06 最新 BF-117：原 OA 批复／审计双列已接 Results 原生端口；最终私有真实入口 11／11，完成精确 Contract→批复、approval revision→审计、三项金额、4 个 Evidence 文件、回执丢失同命令、批复纠正、审计撤回重登记、预览下载、缓存／显式刷新、六库空目标恢复和正式 IAM 撤权。终态 2 当前结果、5 revision／command／event，零页面错误；补测发现并修复空 Evidence `null` 输出契约。Project／Contract／People 摘要不变，Production 调用 0；Main 未改、fixture 已停、本地 Docker 未启动。下一项是对外委托，整体继续 `insufficient-evidence`。[报告](./reports/oa-approval-audit-native-validation-20261006.md)、[演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-06 最新 BF-116：原 OA 四类型生产总览已接 Production 单一只读投影；咨询、设计、勘察、测绘进度和 7 个适用阶段由同一 v6 Work Item／类型快照／Stage／有效引用生成。私有真实入口 10／10、撤权、真实 PG18、五所有者逻辑空目标恢复和 4 份 Evidence 字节保持通过；总览首次 1 次 Production 读取、返回缓存 0 次、显式刷新第 2 次，Project／Evidence 浏览器读取 0，Production digest 零变化，页面错误 0。首轮恢复后仍停在总览却等待隐藏报告文件的验收导航缺陷已修复，并从全新夹具重跑通过。Main／公网未改，fixture 已停，本地 Docker 未启动。Production 读边界有意扩大、写边界不变；Project 零变化，Contract 零依赖。下一项是 Results 批复／审计，整体继续 `insufficient-evidence`。[报告](./reports/oa-production-progress-native-validation-20261006.md)、[演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-06 最新 BF-115：原勘察试验和咨询／勘察／测绘报告已接入 Production v6；新建或显式 5→6 时精确验证 Project ID／Tenant／revision／active，并冻结四个受支持类型键，之后日常读写不依赖 Project 在线。私有正常入口 9/9 场景、真实 Evidence 字节、撤权零写和 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复通过；终态 1 Work Item、7 Stage、4 material（3 active）、1 ending、5 command、1 upgrade、4 snapshot、0 score。两次失败夹具如实保留：3 秒跨 SSH workload exchange 超时导致升级 503，以及浏览器脚本把本地文件名误当持久成功；验收夹具改为 15 秒有界超时并等待原 UI 成功文案后重跑通过。Contract 无依赖，Project 无 schema／业务写；Main／公网／正式 D1、全角色、第二客户和完整 OA 均未关闭。下一项是四类型生产总览／进度，不再铺新文件 Tab。[报告](./reports/oa-test-report-native-validation-20261006.md)、[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

更新日期：2026-10-06

会议用简短阶段清单见 [`docs/status`](./status/README.md)。

## 当前状态

2026-10-06 最新：原变更／竣工双列表已在正常私有工作台接通真实 Production／Evidence／正式 IAM。不可变 v5、一次 4→5、可选 change／必需 completion、多文件、预览下载、追加式替换、移出重开、撤权零写及五所有者逻辑空目标恢复共 11 场景通过；最终 1 Work Item、5 Stage、5 资料引用、2 ending、8 command、1 upgrade、0 score。验收实际抓到数据库约束与应用阶段注册表漏同步两项缺陷，修复后从全新夹具重跑。Project／People 摘要保持，Contract 未进入依赖图；只增强 Production 阶段资料子边界的异类复用证据，Production 整体、跨客户稳定、Main／公网／正式 D1、全角色及完整 OA 仍未通过。下一切片进入原 OA 勘察试验与多类型报告。[报告](./reports/oa-change-completion-native-validation-20261006.md)、[演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-06 最新：方案、初步设计和施工图三个原 OA 同类阶段资料工作面已依次形成私有原生候选。Production 在同一 Work Item 内以不可变 v2→v3→v4 顺序升级承接，Project／People 摘要、Evidence 字节、前序阶段事实和五所有者逻辑空目标恢复均保持；施工图 10 场景首次运行还抓到并修复了“Tab／组件存在但 Workbench runtime 未派发”的装配缺口。阶段资料集合子边界因此达到同类场景可重复，但 Production 整体、跨客户稳定、Main／公网／正式 D1、全角色和完整 OA 仍未通过。下一切片是结构不同的变更／竣工，必须重新核对双列表、文件类型、替换／撤回和完成映射，不能复制施工图的校审／评分模型。经营 Owner／成本等既有公网状态不由本轮改变；本机 Docker 未启动，私有夹具已停止。见[施工图报告](./reports/oa-construction-stage-native-validation-20261006.md)、[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)和[完整清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)。整体结论继续 `insufficient-evidence`；以下日期条目是历史，不取代上述当前状态。

2026-10-03 完整 OA 仍为主线，招投标仅是其中一项。BF-097 的原经营参与人组件及 Project schema 11／catalog 0.7 已发布，原 C 添加／结束／重入／丢回复原命令继续实测，当前 Workbench `.6` 三项公网零业务写入续验通过，用户101和所有其他业务事实保持。200 项 Node、Go／vet、四批联合备份／13 库空目标数据恢复／离机摘要及收尾 D1 check／17 healthy 通过；restore20 和所有探针已停止，数据不删。Contract 原生命令端口聚焦真库及 schema 19 空库启动通过但未部署，主 Contract schema 18 不变，原两链表单、主体／材料／费率映射仍需接通。完整角色、合同／资金、分类型生产、资料统计、OSS 和完整恢复运行未关闭。详细反例和逐工作面剩余任务见[本批报告](./reports/oa-complete-operation-progress-20261003.md)及[完整交付清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)。当前运维 wrapper `oa-projection-visibility-20261003a/d1-active`；以下日期条目保留当时状态。

2026-10-02 本轮完整仿制修复检查点：按用户“最多四小时后暂停 check”保留待继续状态，不标完成。BF-084～089 均为候选切片／分层证据，Auth58 Main 尚未升级；所有本轮专用探针、私有 PG 和临时浏览器已收起，不重开本地 Docker／restore20。最后追加名称查询 503 浏览器探针，88.12 秒 PASS，手工录入／一次显式保存／刷新同项目保持，本地 Workbench Go／vet 和 Node 25＋7 回归通过。下一项固定 [B02 原操作对照](./specs/assembly/oa-business-personnel-clone-gap.md)，先明确类型化参与事实与兼容权限，再实施添加／移出整条交互；其余经营、生产、资料统计、正式入口及联合恢复仍待验，完整结论继续 `insufficient-evidence`。

2026-10-02 BF-089：历史甲方名称在原行内表单候选提供幽灵文字／Tab 显式接受，范围只来自可信 Project 资料读取权；无新应用／事实／授权。私有真库 `r` Project 112／HTTP 85 条含子测试 PASS、零 skip，`s` 独立空库 schema 10／八个匿名 401／重启不变、本地 Go／vet／Node 32 项、合成浏览器 prefix／fuzzy／Tab／Shift+Tab、清空取消零写及一次保存／复读刷新通过。首轮输入样式、初始无效项目错误不可见两项缺陷已修并重测。Main 13 库／17 容器 ID 不变，所有专用探针已停，无本地 Docker；B01／全站／正式发布未关闭，见[报告](./reports/oa-client-name-clone-repair-validation-20261002.md)。下方 BF-088 为当时快照，不删除旧失败证据。

2026-10-02 BF-088：同项目经营页的甲方名称／联系人／电话已候选接通行内保存，不跳 Party 或改主档。Project 不可变联系快照与原身份分开；真库 `o` 109／75 条含子测试 PASS，`q` 全 Project 110 条含联系资料整库空目标恢复／历史身份不变／重放零写／恢复后追加，`p` 独立空库启动及七个匿名 401／重启不变通过。本地 Go／vet、IAM 策略相关测试、Node 29 项与合成浏览器正常／只读／回读 503 分别验证。首轮夹具缺类型、tab 刷新错误已修正且保留，见[报告](./reports/oa-client-info-clone-repair-validation-20261002.md)。名称补全、经营参与人权限、B01 全项、正式 IAM、Main 与全站未完成；全部私有探针已停，无本地 Docker。

2026-10-02 BF-087：同一“编辑基本信息”抽屉已候选接通 core＋business owner＋production owner 的原子保存，去除自创单独后补入口。Project 104／HTTP 66 条含子测试 PASS、schema 10 独立启动／五个匿名 401／重启事实不变通过；本地 Go／vet、Node 24 项和合成浏览器普通项目／包、清除、no-op、必填拒绝及三类分权编辑分别验证。追加 HTTP 真库回读首轮误以默认当前分工应包含结束历史而失败，已按 `include_inactive` 修正并重跑，结果见[报告](./reports/oa-basic-info-clone-repair-validation-20261002.md)。P02 候选人员策略、资质费率、甲方和全站仍未关闭；Main 未更新。

2026-10-02 BF-086 补齐 P01 目录／导航候选：年份来自编号，分页和搜索保留原时机，包／子项在同一项目上下文，返回／刷新／历史恢复列表。实际鼠标暴露并修复外层限高无法滚动；返回恢复第 2 页／scroll 406。本地 Go／vet、Node 20 项，独立 PG18 同批 99／56 条含子测试 PASS，schema 10 独立启动／四个匿名 401／重启事实不变通过。合成浏览器 26 根／28 项、零写命令通过；并非正式用户全链。Main 未更新，13 库摘要／17 容器 ID 不变；探针 exit 0／无 OOM，私有 PG 停止、临时页关闭，restore20 未重开。P01 进度筛选、P02 其余字段和全站仍未关闭，见[报告](./reports/oa-directory-clone-repair-validation-20261002.md)。

2026-10-02 用户明确“修复仿 OA，目标完全仿制”（BF-084）。已实现首条候选链：只填名称＋经营负责人派发，补核心资料后无生产负责人也可完成，进入同一工作台项目详情并后补生产负责人；抽屉全视口可滚动。代码／Node 回归、三组独立 PG18 回归及新 Project 空库独立启动通过；合成浏览器实际点击通过，不代表正式 IAM／数据库／公网链。Auth58 Main 未部署本轮代码、13 库／17 容器 ID 未变，探针停止，restore20 保持停止。其余原 OA 字段和操作仍逐项实施，完整结论继续 `insufficient-evidence`，不再使用旧工程百分比。见 [修复报告](./reports/oa-initiation-clone-repair-validation-20261002.md)。

同日 BF-085 又接通 active Project 的核心资料更正：普通项目和无编号包保存、同 ID 复读／刷新；资料与分工分别授权，不加额外更正说明输入。独立真库同批 97／53 条含子测试 PASS、独立启动及 Node 13 项通过；实际 UI 修复透镜遮挡保存按钮后正常点击成功。仍为 P02 部分候选验证，Main／新策略版本未升级，经营、生产、资料统计等全站操作没有因此完成。见[本轮报告](./reports/oa-profile-clone-repair-validation-20261002.md)。

2026-10-01 20:00，C4 Production 成果资料来源通过独立真 PG18／真 IAM8，Project 消费切片通过源码、7→8 原事实不变、类型化 HTTP 测试端口、资料 GET 零写入、离线幂等重放及单库空目标恢复后追加。没有新服务、业务事实复制或下载扩权；主站仍未升级、13 库摘要与 17 容器 ID 不变、17/17 healthy，探针全部停止。消费者的上游及凭证为显式测试提供者，不把组件证据拼成真实全链／浏览器通过。C5/C6 material source、正式兼容闭包与联合恢复、原连续 C8 和 OSS 仍待验。见 [来源](./reports/auth58-production-material-source-validation-20261001.md)与[消费者报告](./reports/auth58-project-production-material-consumer-validation-20261001.md)。

2026-10-01 19:20，BF-082 原生 Evidence 入口源码及隔离真实 IAM→Gateway→Evidence 三账号／精确 File 矩阵通过；Workbench 固定 test.49 在 network=none 读取当前 D1 清单副本启动通过。主站 13 库摘要／17 容器 ID 不变、17/17 healthy，所有探针停止，restore20 保持停止。C8 驱动已去掉 view 补授及固定 fail，改为原生三权限组合，离线负例通过；主站兼容升级与原连续 C8 尚未运行，不提高总报告结论。开始的下一切片仅为 C4 Production 只读资料来源，职责与验收先写入规格；Project Link／消费者、C5／C6 后续分批。见 [入口证据](./reports/auth58-evidence-native-entry-validation-20261001.md)及[来源规格](./specs/assembly/production-material-reference-v1.md)。

2026-10-01 Auth58 ECS 的 Evidence OSS 独立适配与恢复批次通过：已部署镜像使用实例角色临时身份与正式 IAM Credential，完成 64 KiB／21 MiB 上传、摘要、权限拒绝、幂等、签名下载和空库恢复。后续 `test.41` 隔离候选通过 10 个原 Local 文件读取、真实 CNAME／SDK、新 OSS 文件、Local-primary 回退和 12 个 Version 混合空目标恢复；离机 ZIP 摘要一致，主 Evidence 10 个 Local Version 不变。单份合成 OSS 字节已完成离机误删恢复，实际 900 秒 STS 到期拒绝与刷新通过；D1 `test.44` 候选包 ECS 摘要、云档渲染、IAM 契约预检及配置／运行时错档拒绝通过，Guard boot 的真实 netns 检查通过。修复文件页签名 Date 和文本 GET 压缩导致 complete 500 两项真实缺陷。运行时 Docker IMDS 定向隔离已实施，正式主 Provider／镜像未切换；正式文件浏览器、Guard 重启持久化、基础身份长时自动刷新和全量 OSS 字节保护仍待完成，总体 `insufficient-evidence` 不变。见 [实测报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md)。

同日 Chrome 正式公网 IAM 登录、Workbench 点击新建验收项目、刷新复读、进入真实 Project 与缺负责人拒绝通过；架构透镜显示真实创建结果。单组件部署 Project 页面反馈 `test.43` 后，Node 回归、普通 Go 测试／vet、8 表冷备份空库恢复和更新后事实不变通过，其他 16 个容器不变。18:12～18:16 的真实 Chrome 回归已关闭拒绝后旧 revision 和能力过期编辑态问题：显示缺失负责人及 pending revision 3，自然到期后收回编辑，刷新权限后历史可读；数据库确认只新增一次缺口命令，恢复没有重放。仍存在人员选择器、仅本人透镜授权及其他 Overlay 业务接入缺口，不是完整产品可用。见 [浏览器验证记录](./reports/auth58-public-project-browser-validation-20261001.md)。

18:25～18:40 更新取代上一段“人员选择器未有”的表述：Project `test.46` 已从受保护 People 目录真实选择两名无 IAM 关联的合成负责人，并完成同一夹具立项为 `active / revision 6`；新增命令 3、Assignment 2、历史 3，其他 Project 与 People 14 表不变。`test.47` 折叠能力标签、恢复横排按钮，`test.48` 修复取消触发必填校验；空项目取消真人通过，其他三类弹窗仅处理器回归、待解锁后复测。三次仅 Project 发布均冷备份／空库恢复、离机摘要一致、其他 16 服务不变。仅本人透镜、其他真实 OA 控件、完整权限与 C8 仍待验，不提高整体结论。

同日 17:28 左右按用户授权停止 `restore20` 的 17 个演练容器；全部 exit 0，28 个命名卷、配置与 bind 来源保留，没有删除。主站 17/17 healthy，未重建；停止后短时 CPU idle 从约 1% 回升至 52%～65%，不作为正式容量通过证据。演练栈保持停止，不在后续维护时自动恢复为双栈运行。初始后检失败与独立只读后检通过均如实记录于 [OSS 报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md)。Docker／主机重启维护仍未获确认、未执行。

2026-09-30 参考 OA 界面 Overlay 已完成首轮并部署到公司 Ubuntu D1：Unified Workbench 登录页已改为参考产品的公司／产品双栏入口，登录后提供项目立项、项目管理、项目资料管理和项目统计信息，以及项目列表、项目详情和经营／生产／资料分区。真实 IAM 登录、Gateway 应用授权、个人工作和指标读取仍沿用既有契约；示例项目与尚未接入写 API 的控件明确属于界面验证数据。这一结果只关闭“工作台不像 OA”的界面问题，不改变总体 `insufficient-evidence` 结论。详见 [参考 OA 界面 Overlay 验证记录](./reports/oa-reference-ui-overlay-validation-20260930.md)。

早期汇报曾使用工程实现 98%、分项 96%～97%、阶段总体 85% 的估算；这些不是当前验收比例，不继续沿用。2026-09-30 同一 Tenant／Project 的连续 C0～C7 已通过，C8 实际执行后保留两项 P0 失败，总报告以 `insufficient-evidence` 冻结。完整选装 17/17 healthy、联合空目标恢复等证据存在，但不能抵消 C8、真人关键权限、完整双 Tenant、第二客户及 clean-HEAD 发布缺口。2026-10-01 的公网、Project 页面和 OSS 新证据按上方专项切片登记，不自动提升完整 OA 结论。

| 里程碑 | 状态 | 已验证结果 |
| --- | --- | --- |
| M0 最小工程基线 | 完成 | 两个独立应用可分别测试、构建、迁移和启动；独立数据库健康检查通过 |
| M1 合同最小纵向闭环 | 完成 | `CL-M1-001` 至 `CL-M1-006` 已完成；空库登录、角色授权、合同 revision 1 与不可变审计链路通过 |
| M2 Contract Ledger v1 | 完成 | `CL-M2-001` 至 `CL-M2-012` 已完成：合同业务闭环、可验证导入导出、全量备份恢复和独立安装升级交付全部通过 |
| M3 收支最小纵向闭环 | 已完成 | `MF-M3-001` 至 `MF-M3-008` 已完成：本地身份、主体、正式义务、实际资金、单笔 Allocation、多对多历史下钻与非经营性内部账户转账；全部退出门槛已有直接证据 |
| M4 Money Flow Ledger v1 | 完成 | `MF-M4-001` 至 `MF-M4-004`、`MF-M4-006`、`MF-M4-008`、`MF-M4-009`、`MF-M4-011` 至 `MF-M4-015` 已完成：纠错、调额、减免、受控关闭、当前账龄、单条 historical `as_of` 余额、当前经营汇总下钻、本方主体可校验台账导出、全量备份恢复、独立安装升级和当前预计现金排期均已验证；坏账、票据、真实退款与通用导入审计后延期 |
| M5 可选连接器 | 未开始；启动前准备已完成 | OA 与 OMS 双参考审计、真实案例采集表和九项启动门禁已就绪；两套参考的义务触发规则相反，当前仍等待第一条达到 `ready_for_contract_design` 的脱敏业务流 |
| M6 可交付性验证 | 工程内收口，外部待验 | `MF-M6-001`、`MF-M6-002`、`CL-M6-003`、`M6-004`、`M6-007`、`M6-008` actual release 验收及 `M6-005`、`M6-006` 审计均完成；真实操作者、容量、客户权限与客户场景等待外部输入 |
| M7 统一入口与 IAM | 完成；OA 权限矩阵持续验证 | M7-001 至 M7-006 已验证；直接 Resource Grant、多目标 Project Assignment Policy、单项撤权、Project→People 工作负载凭证，以及 C1 项目责任九角色／双 Project／状态矩阵已通过隔离 D1。C2 最新从正式 Nginx→Gateway→IAM／Policy 入口通过 110 条角色／资源／状态／撤权／页面／故障断言；C3 又以 100 条正式入口断言通过 Project 经营金额独立 Capability、总经理／财务允许、生产经理拒绝、撤权／恢复、账号停用及 IAM／来源故障恢复。完整双租户 D1 交叉矩阵仍待完成 |
| M8 People & Groups | 完成 | Person、Group、带时效与组内角色的 Membership、配置定义、组能力及不可变 Roster Snapshot 已形成独立闭环；正式 IAM／本地模板、公开 API／页面、历史与并发、D1 单次登录、开关、故障隔离和四数据库联合恢复均通过 |
| M9 Project Workspace／参考 OA 业务组合 | Level A、Level B、定制化六层验证、场景证据 v1、可移植开发包与 C1～C7 当前切片已完成相应证据；C8 direct Project→Evidence 的正式 Version 2／stale／relink、Money 指标正式权限链和公司 D1 完整升级已确认，完整 OA 仍待验 | 六应用／六数据库 J1～J4 与可移植 Demo 已通过。C4～C7 已分别形成当前范围的正式入口、装配／恢复和跨进程证据。C8 已固定为无新事实库的只读装配，通过 Evidence 分权、多版本、Project direct 组合、Business Engagement indirect 单元、个人工作和指标消费者；公司 Ubuntu `test.11` 已完成完整升级、保持多版本事实并验证 Money 指标的 403／脱敏／金额授权／撤权刷新。真人浏览器、其余 C8 正式 D1、独立空目标恢复、完整 connector／双 Tenant／连续旅程、第二真实客户和正式 clean-HEAD 发布仍待验 |
| M10 Evidence Service | 独立应用、D1 装配与真实 OSS 核心链完成，迁移／剩余负例待办 | `M10-001` 已交付 File Transfer Core 与 v1 契约；`M10-002` 已交付独立服务、IAM／模板鉴权、PostgreSQL、本地／OSS Provider、服务端完整性确认、清理、备份恢复、Docker 和 release；`M10-003` 已完成 D1 入口、开关、隔离和联合恢复；2026-09-22 已通过真实私有 Bucket 的 CORS、150 MiB multipart、取消清理、签名读取和匿名拒绝。Contract 迁移、网络中断等剩余负例和公网入口尚未完成 |
| M10-P Party Directory | 独立核心、页面、D1 API 链、恢复及台账运行时切换完成，完整双 Tenant 待验 | 已实现独立 Go／PostgreSQL 应用、migration、稳定 Party ID、current＋不可变 revision、登记／搜索／更正／状态／历史／精确引用、IAM 双 Provider、并发幂等、稳定签名游标和失败关闭；单元、HTTP、真库 race、数据库防篡改、跨租户、独立启动、release、契约随包、非 root 镜像、完整备份与空目标恢复通过。Party 管理页与 Project 客户选择已进入 Workbench：浏览器以用户 Capability 发现候选，Project 服务以精确 Party Scope 工作负载凭证复验并冻结快照；Party 停机时新引用失败关闭、既有快照与无 Party Base Project 可用，恢复后新命令只写一次。联合备份／空目标还原保持 2 个 Party、4 个 Project 和 3 个客户快照关系。Contract／Money 已完成确定性预检、来源自有映射和含引用数据切换／只读回滚；Contract-linked C6 又以 62 个正式入口断言完成 `cutover → rollback → recutover`、联合空目标恢复和本地写权永久冻结，Money 完整 D1 消费链已由连续 C3 关闭。完整双 Tenant 仍待补证 |

## 已完成功能

当前下一阶段：Demo Validation 2、场景证据、可移植开发包和 C1～C7 当前可实现切片已逐步收缩。C1 专项为 152 个 HTTP 断言；C2 已完成正式入口、页面、联合恢复与 IAM 故障边界；C3 以 100 个断言通过经营只读组合；C4 完成多类型、正式装配恢复及 86 条权限生命周期断言；C5 完成正式 D1 及 74 条真实旅程断言。C6 已通过正式 D1、Party 运行时切换和 C6→Money，并在连续主数据集关闭正式 connector 与目标停机恢复。C7 又完成正式 D1，并以 11 条直接跨进程断言证明 C7→Money 的停机恢复、接受／拒绝、exact Person→Party 映射、幂等和无级联。C8 已完成静态边界、Evidence 能力分权、Project direct Evidence 运行组合、资料页面首批实现、首个间接资料源、单来源个人工作，以及 Money 单来源指标与 Workbench 单消费者；Money 指标正式 D1 已通过，仍按低资源顺序推进其余缺口。报告结论仍为 `insufficient-evidence`，因为连续 C7～C8、C8 真人浏览器与第二来源、C7 正式 D1 connector／建议引用恢复与单类型最小组合、完整双租户 D1 交叉矩阵、项目包合同金额规则、第二真实客户和公网／离机恢复总门槛尚未完成。

2026-09-27 C3 Billing／Invoice 已从上游／消费侧精确引用推进到完整生命周期、正式 D1 入口和 Project 经营链路组合，但 OA 整体仍严格保持 `insufficient-evidence`。Project、Contract、Contract Settlement 与 Money Obligation 提供版本化精确 reference-read；Billing 对 Party、Project、Contract、Settlement、Obligation、Evidence 分别交换单 action／单精确资源 Credential，并复验 Tenant、类型、ID、revision、摘要和状态。真实 PostgreSQL race 集成完成 revision 1→4 与无效后有效汇总归零；Linux AMD64 非 root 镜像、pull-only release、Suite Manifest、Nginx→Gateway→IAM 正式用户凭证、精确工作负载凭证审计、Party 503 零写入／恢复单次提交、联合备份和销毁卷后的空目标还原均已通过。Billing 页面和 Project 的 Contract／Money／Billing 四类分段只读组合也已通过本地 D1；完整双 Tenant、C3 真实浏览器最终点击、Billing-only 与真实 Ubuntu 仍未完成。证据见 [Billing 生命周期与运行时实验](./reports/oa-c3-billing-lifecycle-runtime-experiment-20260927.md)、[Billing D1 交付与恢复实验](./reports/oa-c3-billing-d1-delivery-recovery-experiment-20260927.md)和[C3 项目经营链路 D1 实验](./reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。

2026-09-27 Party Directory 第一版可实现定义、独立核心运行时、自身交付恢复和首个 Project 消费边界已经形成：产品规格固定创建、查询／搜索、并发更新、停用／恢复、历史和“不自动合并”，技术方案固定独立数据库所有权、当前投影＋不可变 revision、版本化 API、消费者业务角色／历史快照、原生 IAM Capability、失败关闭与迁移顺序。`backend/services/party-directory` 已通过单元、HTTP、独立模块构建、PostgreSQL 18 race、并发重放、数据库 revision 防篡改、跨租户、精确 Scope、独立进程 API 旅程，以及 release SHA-256／版本、契约随包、非 root 镜像、revision 1→3 完整备份、空目标恢复、非空目标拒绝和 revision 4 继续写入。Project Workspace 进一步用可选 `client_party` 证明 active／同租户／精确 revision 验证、最小快照、旧历史不随 Party 当前档案变化、重放先于外部调用、stale／跨租户／停用／503 无部分写入和精确工作负载凭证；Business Engagement 不再作为默认客户关系所有者。同日的 D1 正式链已把 Party 与 Project 置于 Nginx→Gateway→IAM 之后，通过精确 Party Scope 工作负载授权、Party 停机／恢复、无 Party Base Project 继续可用、既有快照可读和恢复后单次写入验收；联合空目标还原后 2 个 Party、4 个 Project 和 3 个客户快照关系原样保持，再次验收通过。随后 Party 管理页、Workbench 条目和 Project 可选客户选择器补齐；浏览器发现候选与 Project 服务端精确复验使用不同凭证，页面只提交 ID／revision，不发明客户改绑／解绑语义。Contract／Money 消费者迁移和完整双 Tenant 矩阵继续单独验收，不能把两个本地 Party 表改成共享表。证据见 [Party→Project 首个消费者实验](./reports/oa-party-project-consumer-experiment-20260927.md)、[Party→Project D1 正式链实验](./reports/oa-party-project-d1-chain-experiment-20260927.md)和[Party 管理与 Project 客户选择实验](./reports/oa-party-project-human-workflow-experiment-20260927.md)。

随后 C3 Billing／Invoice 又完成可移植交付与 D1 恢复：实际 `linux/amd64` 镜像以 `postgres`（UID/GID 70）运行，正式入口完成登记→纠正→Evidence→登记无效→历史，Project Scope 与相邻 Project 拒绝生效；Party 停机时 503 且零写入，恢复后原 Command 单次提交。联合备份中的 Billing 当前 3 条、revision／audit 各 9 条在空目标恢复前后摘要一致，恢复后正式入口再次通过。Billing 人类工作区现已进入 Workbench，并通过金额撤权／恢复、历史脱敏、盲写金额拒绝、409 与 503 恢复。Project 的 C3 经营链路已进一步完成四类来源分段、精确工作负载凭证、总经理／财务允许与生产经理拒绝、撤权／恢复／账号停用、Contract／Money／Billing／IAM 停机恢复；30 次本地采样 p50/p95 为 13/220 ms。下一业务切片进入 C4，C3 双 Tenant、真实浏览器最终点击、Billing-only 与真实 Ubuntu 保留补证。

2026-09-28 C4 责任边界审计与权限生命周期验证完成：参考 OA 的设计／咨询／勘察／测绘不是四条相同流水线，当前证据支持一个严格 Production Operations 内核和代码拥有的版本化类型定义，而不支持复制四个服务或建立可任意编辑的流程平台。C4-A01～A14 当前本地轮次已通过，覆盖不可变注册、兼容迁移、双类型生命周期、canonical 权限、最小组合、双向本地缺席、Level A／B 回归，以及 People＋Project＋Evidence＋Production 的 D1 Suite Manifest／Workbench、正式入口、完整未选缺席、IAM／Evidence 故障恢复和联合空目标恢复。新的 86 条断言与真实浏览器又通过了 Grant 撤销／恢复、账号停用／启用、Assignment 结束、页面 `allowed_actions` 收敛和新 Session 恢复。C4 当前仍为 `partially-supported`，因为双 Tenant 和真实 Ubuntu 尚未通过；OA 总结论保持 `insufficient-evidence`。证据见 [C4 边界审计](./reports/oa-c4-multi-production-boundary-audit-20260927.md)、[正式最小组合实验](./reports/oa-c4-formal-minimum-composition-experiment-20260928.md)和[权限与浏览器实验](./reports/oa-c4-production-permission-browser-experiment-20260928.md)。

2026-09-28 C5 责任边界审计后完成第一个可运行收缩：新增独立 Approval & Audit Results module、嵌入 migration 与 PostgreSQL Store，批复冻结精确 Contract／Settlement revision，审计直接冻结批复历史 revision，不再重读当前合同。固定三项金额使用整数最小货币单位并由服务端与数据库共同校验；纠正、撤回和撤回后重登追加完整历史。单元与 PostgreSQL 18 真库验证覆盖同 Project 两种结果、人工覆盖原因、幂等冲突、八并发单次提交、跨租户隔离、历史防篡改和 Evidence 故障四表零写入。该轮当时状态为 `core-supported / protected-entry-and-composition-not-started`，总进度为 64%；其后 HTTP／IAM 和精确 adapter 已由下一条记录补齐，Project 完成协作、页面、D1 和恢复仍待实现。证据见 [C5 责任边界审计](./reports/oa-c5-approval-audit-boundary-audit-20260928.md)与[C5 领域核心实验](./reports/oa-c5-results-core-experiment-20260928.md)。

同日 C5 第二个收缩补齐受保护 HTTP、Ed25519 IAM、原生 Capability Manifest、Project／Result Scope、金额脱敏、动态 `allowed_actions` 和 Project／Contract／Evidence 精确 HTTP adapter。PostgreSQL 18.6 矩阵覆盖 401／403／409／422／503、相邻角色、Project B、错 Tenant／Audience、金额与 Evidence 缺权、Project 未链接／stale source、依赖停机零写入、恢复后原 Command 单次生效及成功重放不再访问依赖。该时点状态为 `protected-http-and-reference-adapters-supported / cross-process-composition-not-tested`，总进度为 65%。证据见 [C5 受保护 HTTP、IAM 与精确引用实验](./reports/oa-c5-protected-http-reference-experiment-20260928.md)。

同日 C5 第三个收缩补齐 Results 人类工作区、Project 级 Results 最小引用、`reference-oa-c5-v1` 纯读取评估、显式确认、不可变来源快照、drift 与成功回放先于依赖。普通测试和 PostgreSQL 18.6 集成测试通过页面结构、权限收敛、GET 零写入、确认只写 Project、stale revision／source 失败关闭、确认防篡改和依赖离线幂等回放。当前状态为 `human-workspace-and-project-checkpoint-supported / cross-process-composition-not-tested`，总进度为 67%；真实独立进程组合、Suite Manifest、Workbench、正式入口、恢复和 C6～C8 仍待实现。证据见 [C5 人类工作区与 Project Completion Profile 实验](./reports/oa-c5-human-completion-profile-experiment-20260928.md)。

同日 C5 第四个收缩把 Results 装入正式 D1：新增独立镜像、数据库、migration、工作负载密钥、Capability Manifest、显式 Capability namespace、Suite Manifest、Workbench、Gateway 路由、依赖门禁和联合备份恢复。隔离 D1 经正式入口验收后生成联合备份，销毁容器／网络／卷，在空目标恢复并再次通过同一入口验收；Results dump 和十项选择清单参与 SHA-256。当前状态为 `formal-d1-assembly-and-recovery-supported / business-cross-process-journey-pending`，总进度为 69%；C5 仍需完成真实批复→审计→Project 完成确认、撤权／双 Tenant 和真实浏览器。证据见 [C5 D1 正式装配与恢复实验](./reports/oa-c5-d1-assembly-recovery-experiment-20260928.md)。

同日 C5 第五个收缩完成真实独立进程旅程：People、Project、Contract、Evidence、Results 在隔离 D1 经正式入口完成来源建立、approval→audit、Project 检查点确认、纠正后 drift 和故障恢复。74 个 HTTP 断言全部匹配，覆盖 Evidence／Contract／Results 停机、零部分写入、原 Command 恢复、无金额脱敏、跨 Project、无 create、`correct` Grant 撤销／恢复，以及旧成功确认本地回放和新确认 503。运行中还修复“Project 确认推进 revision 后 Results 无法纠正”和“依赖 unavailable 被误报为 422”两个真实跨域缺陷。当前状态为 `real-independent-process-journey-supported / dual-tenant-and-real-browser-pending`，总进度为 73%；下一业务切片可进入 C6，C5 的双 Tenant／账号停用／真实浏览器保留为独立补证轴。证据见 [C5 真实独立进程业务旅程实验](./reports/oa-c5-real-process-journey-experiment-20260928.md)。

同日 C6 完成静态责任边界审计：参考 OA 把奖金、成本、客户付款、票据、专家费和外协付款混入可编辑删除、浮点金额的 `FinancialRecord`，外协删除还会按供应商名称兜底删除财务记录；该设计不能直接移植。目标接受独立 Cost & Outsourcing 应用，在一个部署边界内用 Cost Entry 与 Outsourcing Engagement 两个严格聚合分别管理成本发生和外协事项，并通过版本化精确引用使用 Project、Party、Contract、Evidence；对 Money 只提出 payable suggestion，由 Money 独立确认，Billing 继续拥有票据。当前状态为 `boundary-supported / implementation-not-started`；C6-A01～A18 已固定，但领域核心、数据库、IAM、页面、D1、恢复与 Project 完成适用性均未实现，因此总进度保持 73%。证据见 [C6 成本与外协责任边界审计](./reports/oa-c6-cost-outsourcing-boundary-audit-20260928.md)。

同日 C6 第一实现收缩完成独立 `cost-outsourcing` module、双聚合模型和 PostgreSQL 18 事实层。Cost 登记／纠正／撤回、Engagement 登记／纠正／撤回、追加式五分制 Assessment、Project／Party／Contract／Evidence 精确快照、Project/action/Evidence 分权、失败零写入和成功本地重放均已验证；8 路并发同 Command 只提交 1 current／revision／receipt／audit，数据库 trigger 拒绝历史篡改和物理删除。当前状态为 `domain-core-and-postgresql-supported / protected-http-and-composition-pending`，总进度为 75%；下一轮进入受保护 HTTP、正式 IAM 与真实引用 adapter。证据见 [C6 领域核心实验](./reports/oa-c6-core-experiment-20260928.md)。

随后 C6 第二实现收缩完成受保护版本化 HTTP、正式 Ed25519 IAM、Project／精确 Cost／Engagement Scope、金额与 Evidence 分权、动态 `allowed_actions`，以及 Project／Party／Contract／Evidence 四类请求绑定 HTTP adapter。真实 PostgreSQL 18 集成矩阵通过 401／403／409／422／503、相邻角色／资源拒绝、金额脱敏、Contract 的 Project＋Provider Party 复验、依赖失败零写入、恢复后原 Command 单次提交和成功本地重放；访问契约与 IAM Catalog 又以 `capability_namespaces` 支持 C6 两个原生聚合。当前状态为 `protected-http-and-reference-adapters-supported / human-workspace-and-composition-pending`，总进度为 77%；下一轮进入 C6 人类工作区和真实独立进程最小组合。证据见 [C6 受保护 HTTP、IAM 与精确引用实验](./reports/oa-c6-protected-http-reference-experiment-20260928.md)。

随后 C6 第三实现收缩完成实际人类工作区和真实独立进程最小组合。页面覆盖 Project、Cost、Engagement、详情、历史、纠正、撤回和 Assessment，并用服务端 `allowed_actions`、金额能力和 revision 驱动动作；真实浏览器检查修复 CSP 与 Assessment expected revision 两项问题。IAM、Project、Party、Contract、Evidence、C6 和 PostgreSQL 分进程运行，12 个 HTTP 结果覆盖依赖停机零写入、恢复、成功 Command 在依赖离线时本地重放、金额脱敏、只读写入拒绝和错 Project Scope 拒绝；Money、Billing、应付款建议和 C7 明确未启动。当前状态为 `human-workspace-and-independent-process-composition-supported / formal-d1-and-payable-suggestion-pending`，总进度为 80%；下一轮进入 C6 正式 D1 装配、未选缺席与联合恢复。证据见 [C6 人类工作区与独立进程最小组合实验](./reports/oa-c6-human-workspace-independent-composition-experiment-20260928.md)。

### Demo Validation 2 验收基线（2026-09-25 已确认）

- Level A 分别验证六个小系统，不启动 Nginx、Gateway、中心 IAM 或其他真实业务应用；所有受保护访问仍使用同形模板 IAM；
- Project／Design 可以使用公开 reference-read 契约桩隔离相邻应用，但真实状态机、migration、数据库、历史、幂等和失败关闭不得替换；
- 六个独立 Demo 均已在各自最小环境中通过；Project 与 Design 只额外使用无业务所有权的公开 reference-read 契约桩。补齐后的最终统一审计再次顺序运行全部六套 Demo，六项能力簇均为 complete，Level A 总门禁已通过；
- Level B 复用同一业务发布物；当前高保真 J1～J4、幂等重放和五来源逐项停机隔离已经通过，并成为后续组合变更回归基线；
- 定制化必须把差异唯一归入稳定内核、配置、策略、类型化扩展、客户装配或客户专项；连接器只传递版本化事实、建议或命令，不获得目标事实所有权；
- Customer Scenario Evidence v1 已提供参考 OA 基准、第二真实客户草稿模板和严格校验；单份场景不得宣称跨客户边界，真实客户最终结果没有确认引用时失败关闭；
- 六应用 Demo 实际体验基线已落地：Contract、Money 与 People 保留真实表单路径并补齐统一页面骨架、任务导航和稳定选中态；Evidence、Project 与 Design 已补齐受保护记录列表、创建／上传入口、记录工作区、历史／版本和明确错误反馈。凭证只保存在当前标签页，新增列表 API 保持 tenant／resource scope 和各自数据所有权；六个独立验收包、本地组合 J1～J4、入口浏览器检查和错误态恢复均通过。

规格：[小系统群 Demo Validation 2](../arckit/spec/assembly/demo-validation-2.md)

### Demo 人工可用性基线（2026-09-25 已通过）

- 把 OMS 明确登记为继参考 OA 之后的第二套参考产品样本；它提供另一种业务表达和交互参考，但不冒充独立真实客户证据，也不直接决定 BizFoundry 稳定内核；
- 六个本地组合入口采用同一套中性工作台视觉语言，优先回答“这个系统解决什么、第一次从哪里开始、当前页面能做什么”，避免把内部 API、状态码或技术对象直接当成产品首页；
- Contract Ledger、Money Flow Ledger 和 People & Groups 保留原有真实表单与业务路径，在其上补齐清晰导航、首页任务顺序、模块说明、边界说明和可读布局；
- Evidence Service、Project Workspace 和 Design Operations 新增受保护列表 API 与真实浏览器工作台，支持记录选择、创建或上传、历史／版本查看及业务动作反馈；页面不内置令牌，凭证只存当前标签页；
- Contract 与 Money 的页面切换后选中 Tab 保持可见；Project 的无效凭证路径在不丢失页面上下文的情况下给出明确反馈；六个入口已人工检查布局与可访问结构；
- 本轮新增的列表契约属于各应用自有读取面，没有改变应用职责、数据库所有权、migration、Suite Manifest、Deployment Profile 或鉴权边界。

### Contract Ledger Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/contract-ledger` 只启动 Contract Ledger 与其独占 PostgreSQL；数据库仅连接内部网络，应用只向本机 `127.0.0.1:28081` 发布演示入口；
- 从空卷经真实 migration、登录和网页表单建立双方主体、主合同与第二份正式合同；主合同依次形成激活 `revision 1`、补充调额 `revision 2`、受控更正 `revision 3`、终止 `revision 4`，随后登记阶段／最终结算并操作性关闭为 `closed / revision 4`；
- 建立不可变合同关系，上传遗留本地 Evidence v1／v2 并核对最新版原始字节；自动核对查询、九类关键审计事件、受保护 ZIP 账簿导出和版本化 reference-read；
- 未认证访问、错误密码、错误 reference token、终止后普通变更与关闭后新结算均失败关闭；最终结算、关闭与原补充协议命令重放不新增事实；
- 总门禁 clean run 的主合同 ID 为 `7af6396e-f19c-4f85-ad37-29ee60f35220`；完整 replay 保持同一 ID、`closed / revision 4`，最终 cleanup 删除本 Demo 的数据库卷、Evidence 卷和续跑状态；
- Contract 应用普通测试、`go vet`、验收驱动单元测试、Shell 语法与 Compose 配置均通过；Demo Validation 2 规定的 Contract 能力簇现已完整。演示的本地 Evidence 明确是待迁移遗留能力，不改变独立 Evidence Service 的目标所有权。

运行入口：[Contract Ledger 独立 Demo](../deployments/demos/contract-ledger/README.md)

### Money Flow Ledger Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/money-flow-ledger` 只启动 Money Flow Ledger 与其独占 PostgreSQL；数据库仅连接内部网络，应用只向本机 `127.0.0.1:28083` 发布演示入口；
- 从空卷经真实 migration、登录和网页表单登记本方与往来方，形成应收义务 `revision 1`，增加 5,000 元形成 `revision 2`，并为当前 125,000 元余额设置预计回款日；
- 首笔 40,000 元收款先形成错误 Allocation，再完整反向并建立正确替代 Allocation；第二笔 30,000 元收款形成另一条有效 Allocation，最终以两条有效核销合计 70,000 元；
- 登记 5,000 元内部账户转账并证明其不进入经营收支或义务核销；随后减免 55,000 元并受控关闭，自动重放初始核销、反向、两笔有效核销和关闭四个时点的精确历史余额；
- 自动核对现金排期历史、零余额终态、账龄排除、经营汇总、受保护 ZIP 台账导出及 Obligation／Transaction reference-read；另以 1,000 元误录收款验证完整 Cash Reversal；
- 未认证访问、错误密码、错误 reference token、关闭后调额与已核销收款直接冲销均失败关闭；同一调额命令重放及完整 Demo 再次执行均保持主业务义务 `closed / revision 2`；
- clean run 与 replay 均保持义务 ID `1a91ecc2-cc9c-4281-8c73-f20ff1d15838`、首笔收款 ID `6ade6539-da33-48b7-949e-209f3c6a96fe`、第二笔收款 ID `7c65a8b3-64e0-4fad-9112-fc9129daf222` 和内部转账 ID `fa3919e5-7cc1-4a3d-91bd-85990e9f01bb`；Demo Validation 2 规定的 Money 能力簇现已完整；
- 坏账、票据、真实退款和通用导入继续按既有业务边界审计结论延期，不由本 Demo 猜测实现。

运行入口：[Money Flow Ledger 独立 Demo](../deployments/demos/money-flow-ledger/README.md)

### People & Groups Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/people-groups` 只启动 People & Groups 与其独占 PostgreSQL；数据库仅连接内部网络，应用只向本机 `127.0.0.1:28084` 发布演示入口；
- 从空卷经真实 migration 和模板身份登录建立设计交付组类型、两种组内角色、设计交付能力、两名 Person 和一个 Group，完成 Membership 加入／换角／结束、Capability 分配／结束、人员资料修改、IAM Subject 外部引用、人员／目录停用、两份不可变 Roster Snapshot 与 Group 归档；
- revision 4 快照含两名 active 成员与一项能力；负责人改名并绑定 IAM Subject 后，活动 Group 推进 revision，revision 9 快照只含该名 `person revision 3` 主创建筑师且无 active 能力；旧快照 payload 与 SHA-256 保持不变，两个快照都不泄露 IAM Subject；
- 普通成员结束后停用为 `inactive / revision 2`，重新入组被拒绝；设计交付 Definition 停用为 `inactive / revision 2`，新的 Capability Assignment 被拒绝，旧 Membership／Assignment 历史仍可读；
- 未认证访问、错误模板令牌、过期 Group revision、inactive Person／Definition 和归档后 Membership 写入均失败关闭；clean run 与完整 replay 保持同一 Group `3988d0fe-408f-4de5-bf0e-0866a577a58f / archived / revision 11`、Alice `4f02ccf8-43c1-4af5-994f-11cc45e8e31d`、Bob `294ca777-c92b-4e29-935b-de14ec5d1628` 及 revision 4／9 两份快照；
- reset 只删除本 Demo 的数据库卷；People 普通测试、`go vet`、验收驱动测试、Shell 语法与 Compose 配置均通过。Demo Validation 2 规定的 People 能力簇现已完整，IAM 凭证所有权仍属于 IAM。

运行入口：[People & Groups 独立 Demo](../deployments/demos/people-groups/README.md)

### Evidence Service Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/evidence-service` 只启动 Evidence Service、其独占 PostgreSQL 与本地对象卷；数据库只在内部网络，对外 HTTP 只绑定本机 `127.0.0.1:28085`，没有启动 OSS、Nginx、Gateway、中心 IAM 或其他业务应用；
- 从空卷经真实 migration 和模板身份完成 Create → Local PUT → Complete，服务端实际复读对象并确认大小、`application/pdf` 与 SHA-256，再形成不可变 File Version 1；
- 自动核对 File／Version reference-read、短期 Access Grant、完整下载和 `Range: bytes=0-3` 的 `206 %PDF`；上传 Token 和下载 `file_name` 被篡改后均返回 401；
- 创建与完成命令同键同载荷返回相同 Session／Version，同键异载荷返回 409；重复 PUT、无效标识、匿名／错误模板令牌、完成文件 Abort 均失败关闭；
- 另建一份摘要不匹配上传和一份已取消上传，清理任务每次实际得到 `examined=2 / cleaned=2 / failed=0`，已经完成的 Version 保持受保护；
- 完整容器场景连续执行两遍均通过：第一遍 File ID `42b9db10-467a-4dbf-ae0a-9b0ba022de46`，第二遍 File ID `f0f70f91-c45b-4072-8f22-3663beeca316`；验收驱动单元测试、`go vet`、Shell 语法、Compose 配置和工作区同步均通过；
- 本结果只证明 Local Storage Provider 下的独立 Evidence 契约。真实 OSS 核心链已有独立证据，但正式 IAM、完整入口、Contract 迁移和云端剩余负例不由本 Demo 冒充。

运行入口：[Evidence Service 独立 Demo](../deployments/demos/evidence-service/README.md)

### Project Workspace Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/project-workspace` 只启动 Project Workspace、它自己的 PostgreSQL 和一个无业务所有权的公开 reference-read 契约桩；数据库与契约桩都不发布宿主机端口，Project 只绑定 `127.0.0.1:28086`；
- 从空卷经真实 migration 和模板身份创建 Project 草稿、更新至 revision 2、补齐 Person 经营负责人和 Group 生产负责人后完成立项为 `active / revision 5`，再建立 Contract／Evidence Resource Link、解除 Evidence Link 并受控取消为 `cancelled / revision 9`；
- 不完整项目完成立项时进入 `pending_completion / revision 2`；Person／Contract 不存在返回 422，暂不可用返回 503，且 Project revision、Assignment 和 Link 数量保持不变；
- 同一创建、Assignment 和 Resource Link 命令重放都返回原稳定事实；契约桩令同一 Person／Contract 在后续读取变为 unavailable，证明命令重放不会重新依赖上游，而详情查询能显式降级；
- 自动核对 Project／Assignment 公开 reference-read、available／unavailable 状态、revision 1～9 不可变历史、匿名／错误令牌、同键异载荷、过期 revision、终态写入拒绝和伪造身份 Header 清除；
- 完整容器场景连续执行两遍均通过：Project ID 分别为 `8717295b-d0cc-4515-a397-e931d413241b` 与 `6e7dbe9a-0775-45c2-9a71-d8168bfe5054`，均为 `cancelled / revision 9 / history_events 9`；应用测试、`go vet`、验收驱动测试、Shell 语法与 Compose 配置同时通过；
- reset 只删除 Project Demo 专属数据库卷，未触碰仍保留的 Evidence Demo 卷；真实 People／Contract／Evidence 组合不由本 Demo 冒充，继续由 Level B 证明。

运行入口：[Project Workspace 独立 Demo](../deployments/demos/project-workspace/README.md)

### Design Operations Level A 独立 Demo（2026-09-25 已通过）

- `deployments/demos/design-operations` 只启动 Design Operations、它自己的 PostgreSQL 和一个无业务所有权的 Project／Assignment／Evidence reference-read 契约桩；数据库与契约桩不发布宿主机端口，Design 只绑定 `127.0.0.1:28087`；
- 从空卷经真实 migration 和模板身份创建 Work Item，在负责人引用第一次可用、随后失效后显式换为稳定生产负责人；方案成果登记、同命令重放和新成果替换后完成方案阶段；
- 初步设计先被明确设为不适用，施工图与施工配合完成后 Work Item 到达 `completed / revision 11`；随后恢复初步设计适用性，系统把其后的两个阶段级联回退并用既有成果逐级重做；
- 施工配合 Evidence 在查询中降级为 unavailable，完成命令返回 503 且 revision 保持 15；替换为稳定成果后完成至 revision 17，再重开施工图、级联回退施工配合并重新完成，最后受控取消为 `cancelled / revision 21`；
- 自动核对 Project／Assignment／Evidence 不存在与暂不可用、错误 Assignment 角色、缺成果、不允许跳过施工图、匿名／错误令牌、同键异载荷、终态写入拒绝、伪造身份 Header 清除，以及包含两版方案成果的 revision 1～21 不可变历史；
- 清空专属数据库卷后的首次运行产生 Work Item `5cb73054-6018-4e85-aeb0-e99884fed8e1`，同环境完整重放产生独立命名场景 `33e0978b-34e7-4a1c-aeb8-180c23d07e74`；两者均为 `cancelled / revision 21 / history_events 21`；
- Design 普通测试、`go vet`、验收驱动测试、Shell 语法、Compose 配置和工作区同步均通过；真实 Project／People／Evidence 组合仍由 Level B 证明，不由契约桩冒充。

运行入口：[Design Operations 独立 Demo](../deployments/demos/design-operations/README.md)

### Demo Validation 2 Level A 最终总门禁（2026-09-25 已通过）

- 新增统一入口 `deployments/demos/ops/level-a`，顺序运行六套 Demo；每套都先删除旧合成状态，再执行 run、replay 和定向 cleanup，任一命令失败都会停止并尝试清理当前 Demo；
- Contract `f9a96fdf-7d5a-467e-bb28-6174390b0b64`、Money Obligation `93c9a746-fa5e-4ef5-9a24-2a452338d64b` 与 People Group `528a8711-feda-4dc2-a263-093b2b7f5402` 在 replay 中保持原稳定事实；三者分别达到 `closed / revision 4`、`closed / revision 2` 与 `archived / revision 11`；
- Evidence 两次 File ID 为 `cbe6d9a4-6f4c-4aef-8491-eb8c5704f3af`、`60ed7b88-3f23-4fb9-b9e6-dde94374d81a`；Project 两次 Project ID 为 `0198e9bd-f0ef-4704-896f-620df53a7051`、`afea010c-d960-4982-8471-4ee263f1ba69`；Design 两次 Work Item ID 为 `2654e09a-61a7-45f9-9cd3-d526c7b67e07`、`cfa84e26-a551-483a-8d44-01f2b72e8349`；
- 六项均通过 `fresh-state,run,replay,cleanup`，执行结束后六套容器、网络和专属卷均已清理；运行门禁明确为 6/6；
- 能力清单六项全部为 `complete`，最终统一报告实际输出 `LEVEL_A_STATUS=passed`；清理后没有六套 Demo 的专属容器和数据卷残留，随后完成的 Level B 高保真 OA 继续使用真实相邻应用而非这些契约桩。

统一入口与判定说明：[Level A 总入口](../deployments/demos/README.md)

### M9 本地参考 OA Level B 高保真组合（2026-09-25 已通过）

范围与结果：

- 新增 `deployments/local-oa`，在不启动 Nginx、Gateway、中心 IAM 和 Workbench 的条件下运行 Contract、Money、People、Evidence、Project、Design 六个应用；每个应用拥有独立 PostgreSQL database／service／volume，宿主机端口只绑定 `127.0.0.1`；
- `acceptance/reference-oa-demo` 只通过公开 HTTP／表单接口建立非真实数据：J1 形成两次不可变名册快照、人员／任职变化和不被上游静默改写的 Project 双负责人；J2 形成合同补充／更正／终止／结算／关闭、人工确认与调额义务、两笔回款、错误核销反向与替代、减免／关闭和误录资金冲销；J3 形成 Evidence 替换、不适用恢复、补交和返工重开；J4 回挂并追溯五类稳定引用；
- 固定 Command ID 与本地 `0600` 续跑状态支持中断恢复和幂等重放；同一 Demo 连续执行两次返回相同 Project、Contract、Obligation、Transaction、Allocation、Evidence 和 Design 身份；
- 从完全删除本组合容器卷与运行状态开始一次通过，并立即完整重放；最终状态为 People revision 4／8 名册快照、Contract `closed / revision 4`、Money Obligation `closed / revision 2`、Design `completed / revision 18`、Project `active / revision 9`，重放保持相同稳定 ID；
- 故障隔离依次真实停止 Contract、Money、People、Evidence 和 Design；每轮 Project 都保持 `active / revision 9`、两个 Assignment 快照可读，只有依赖停止服务的实时引用明确不可用，未停止服务继续 ready，停止项恢复 healthy 后才进入下一轮；
- 组合测试发现并修复 Design reference-read 缺少 Project 统一 `id` 字段的问题，保留 `work_item_id` 兼容字段；Design 与 Project 回归通过；
- 本轮没有自动 Contract → Money 连接器、共享数据库、鉴权旁路或 OA 专用核心分支。该实际组合变化已纳入六层账目并通过向后兼容测试；随后生成的 `v0.1.0-demo.11` 已从同一无源码归档独立通过六个产品、两类子集与当前高保真全量 clean-room 复验，并包含只读 Ubuntu 预检。完整入口架构、真实公司 Ubuntu 和正式 clean-HEAD 发布仍是独立验收。

运行入口：[本地参考 OA 业务组合](../deployments/local-oa/README.md)／[参考 OA Demo 规格](../arckit/spec/assembly/reference-oa-demo.md)

### 第二参考产品 OMS 六层对照（2026-09-25 已形成 evidence-ready 证据）

- OMS 已接受业务规格、架构边界和单节点部署决策完成只读审计；其 React 原型 `npm run lint` 与 `npm test` 通过，2 项服务端渲染／产品元信息测试通过，构建只有已知的大 chunk 警告；
- 新增 `reference-oms.json`，以 `reference_product / reference_baseline / evidence_ready` 记录七类候选事实、八项六层差异和全部未测量生产边界；validator 通过，readiness 正确停在 `evidence_ready`；
- 与参考 OA 的人工对照确认合同、主体／人员、义务与资金、Evidence、历史纠错和单节点最低部署是稳定共性候选；施工履约是类型化扩展候选，Billing／Invoice 是独立责任域候选；
- 该次 OMS 对照形成时 Party Directory 仍是下一项；当前其独立核心、Project 消费边界和 D1 API 链已经完成，Billing／Invoice 成为后续优先证据审计的 C3 候选；审批矩阵、管理费、税务、人员支付、经营汇总和 OMS 生产架构保持未知；
- OMS 不是第二真实客户，两个参考产品不能建立跨客户结论，真实客户门槛和真实环境门槛均未关闭。

对照账：[参考 OA 与 OMS 六层边界对照](./reports/reference-product-comparison-oa-oms.md)

### M9 本地参考 OA Ubuntu 可移植包（2026-09-25 已通过）

范围与结果：

- 新增版本化 `linux/amd64` 发布包，包含六个应用镜像、一个 PostgreSQL 镜像、静态 J1～J4 驱动器、Compose、运维入口、元数据和内外两层 SHA-256；不包含源码、`.env`、数据库、Evidence 字节、证书、私钥或 Registry 凭据；
- 发布模式强制 `--no-build --pull never`，九个镜像逐一检查目标平台，缺失或平台不匹配时拒绝启动；其中两个公开契约桩只供 Project／Design Level A 使用，不进入子集或完整 OA；目标机 `init` 独立生成 `0600` 随机凭据，不复制个人试验田秘密；
- `v0.1.0-demo.11` 从归档解压到全新临时目录、装载九个 Linux/amd64 镜像并生成新秘密后，先分别通过六个独立产品的 fresh-state／run／replay／cleanup，再通过设计交付与商业台账子集的精确选择集、两次稳定重放和清理，最后创建全新六数据库，通过当前高保真 J1～J4 两次同 ID 重放、Contract／Money／People／Evidence／Design 五来源隔离、逐项恢复、最终十二容器健康检查和九个验收 Compose project 的零残留断言；包内有七个静态验收驱动、两个只供 Project／Design Level A 使用的契约桩镜像和一个只读 Ubuntu 目标机预检；最终归档 SHA-256 为 `3a70f263f3c9c0e30e8121640e7d99a79b0b036d3a23920ce93c079c41f99a59`；
- 第一份 `demo.9` 候选在 Evidence 独立验收中暴露通用应用镜像未携带 `/app/ops` 的问题；后一份候选虽通过运行验收，但包内操作文档仍标注 `demo.8`。两份候选均未被接受并已移入系统废纸篓，可恢复；最终包在文档同步后重新构建，并再次通过完整 clean-room 验收与包内文档检查；
- clean-room 验收在 ARM Mac 上通过 Docker 的 x86_64 模拟实际执行目标镜像；异构宿主把同一个静态驱动器临时挂载进已包含的目标应用镜像，不增加发布镜像或正式组件；
- 发布包现已加入只读目标机 `preflight`：在装载镜像前检查 Ubuntu／平台、Docker daemon／Compose v2、运行工具、磁盘、内存加 Swap、三层验收端口和九个默认 Compose project 冲突；正例及架构、端口、磁盘、Docker、既有资源五类负例已自动化通过，预检不写目标机，也不冒充真实公司 Ubuntu 验收；
- 首轮验收发现 PostgreSQL 初始化临时 server 被过早判为 healthy，现由健康检查同时确认 PID 1 为最终 `postgres`；第二轮发现 ARM 宿主不能直接运行 Linux AMD64 驱动器，现按宿主／目标平台选择直接或容器化执行。两项均在最终 clean-room 复验中通过。

交接手册：[本地参考 OA 的 Ubuntu 可移植演示包](./operations/local-oa-ubuntu-demo.md)

### M9-J2／J4 稳定引用基础

范围与结果：

- Contract Ledger 新增 `GET /api/v1/references/contracts/{id}`；Money Flow Ledger 新增 Obligation 和 Transaction 两个 reference-read；三者只返回 ID、credential tenant、resource type、status 和 revision，不复制金额、相对方、账户或证据内容；
- 本地组合必须配置成对的 `REFERENCE_TENANT_ID`／随机 24+ 字节 Bearer Token，不存在关闭鉴权的开关；正式 IAM 模式拒绝静态引用凭证，使用目标应用 audience 的 `*.view` Capability Credential；
- Project Workspace 的目标端口样例已纠正为 Contract `8081`、Money `8083`，Linux release 同时携带 Contract、Money、Evidence 和 Design 的五份依赖 reference-read 契约；
- 无凭证、错误凭证、无查看能力、缺失 tenant、资源不存在和服务内部失败分别产生确定的 401／403／404／5xx，不写 Project 或目标应用数据库；
- 该切片只完成跨应用“可验证关联”基础。人工确认义务、登记收款／核销、关联 Project、展示部分不可用状态的同一假数据操作者旅程仍是下一步。

验证证据：

- Contract／Money config 与 HTTP reference 测试通过；全仓普通 `go test`、`go vet` 和 `make fmt-check` 通过；
- 24 份 Arckit YAML API 契约全部解析通过；Project Linux AMD64 `v0.1.0-local.3` 发布包实际包含五份依赖 reference-read 契约。

规格：[参考 OA Demo](../arckit/spec/assembly/reference-oa-demo.md)

### M9-J1 Project Workspace 独立业务切片

范围与结果：

- 新增 `backend/services/project-workspace` 独立 Go 应用与 PostgreSQL 18 数据库，只拥有 Project、Assignment、类型化 Resource Link、幂等命令和不可变历史；
- 八个 `/api/v1/projects` 命令／查询 API 强制 Bearer Credential、tenant、action 与 scope，模板 Provider 和正式 IAM Ed25519 Provider 产生同形上下文；
- Person／Group Assignment 在写入前验证 People 的 ID、tenant、状态与 revision；合同、资金、Evidence 和设计运营 Link 使用固定 service／type reference-read 契约，不读取目标数据库；
- 上游不可用时新 Assignment／Link 失败关闭且不部分写入，既有 Project 仍可读并把目标 Link 标为 `unavailable`；同 command 的成功重放不再依赖上游继续在线；
- Project 草稿、缺口立项、双负责人激活、负责人结束后重建、取消终态、乐观并发、编号唯一、历史不可变和 concurrent command 幂等均由数据库约束与测试共同保护；
- 提供独立 `.env.example`、Compose、Dockerfile、健康检查、数据库备份恢复和 Linux release；Project 后续已加入 D1 Suite Manifest、完整入口与联合恢复，J2～J4 组合由后续本地 OA 验收覆盖。

验证证据：

- `go test ./...`、`go vet ./...`、`go build ./...` 通过；
- PostgreSQL 18 `go test -tags=integration ./...` 通过，覆盖 J1-A01～J1-A16 的真库／HTTP／故障链；
- Project、People、Contract、Money 与 Evidence 普通测试在同一 workspace 共同通过，证明 Project 加入不破坏既有应用；
- `omsbizfoundry/project-workspace:j1-local` image 构建通过，容器以非 root 用户连接独立数据库后 `/health/ready` 和 Bearer 创建 Project 冒烟通过；
- PostgreSQL custom archive 已恢复到一座新空库，migration、Project 和 History 数量一致；Linux AMD64 release 的 SHA-256 与固定文件清单通过；
- J1-A17、A18 分别由既有应用组合回归和模板／正式 IAM 同形契约测试覆盖。

规格：[Project Workspace：项目立项](../arckit/spec/project-workspace/project-initiation.md)

### M9-J3 Design Operations 独立业务切片

范围与结果：

- 新增 `backend/services/design-operations` 独立 Go 应用与 PostgreSQL 18 数据库，只拥有 `design-v1` Work Item、固定 Stage、适用性、Evidence 成果引用、幂等命令和不可变历史；
- 十个 v1 路由强制 Bearer Credential、tenant、action 与 scope；模板 Provider 和正式 IAM Ed25519 Provider 产生同形上下文，正式 issuer 强制 HTTPS；
- Project Workspace 新增 Project／Assignment 最小 reference-read，Evidence Service 新增 File／Version reference-read；Design 只保存稳定 ID、revision 与业务显示快照，不复制项目、人员或 OSS 技术事实；
- Stage 按固定顺序推进，允许方案／初设显式不适用，完成前在线复核负责人和 active 成果，替换、重开、恢复不适用、取消都保留历史并失败关闭；
- 提供独立 `.env.example`、Compose、Dockerfile、健康检查、数据库备份恢复和 Linux release；该条记录形成时 Design 尚未加入 D1，后续已由 Production Operations 兼容演进并进入 D1 Suite Manifest；J2／J4 不由该应用独自宣称完成。

验证证据：

- `go test ./...`、`go vet ./...`、`go build ./...` 通过；
- PostgreSQL 18 `go test -race -tags=integration ./internal/design ./internal/httpserver` 及完整 integration 通过，覆盖 J3-A01～J3-A20；
- Project reference-read 真库回归、Evidence reference 不泄露 Object Key、跨应用普通测试和 `make fmt-check` 通过；
- `omsbizfoundry/design-operations:local` image 构建通过，容器以非 root 用户连接独立数据库后 `/health/ready` 返回 200；
- PostgreSQL custom archive 已恢复到新空库并核对 migration、Work Item、Stage、Deliverable 和 History；Linux AMD64 release 的 SHA-256 与固定文件清单通过。

规格：[Design Operations：设计生产与交付](../arckit/spec/design-operations/design-production-delivery.md)

### 阿里云 D1 ECS 宿主机初始化

当前结果：

- 复用现有包年包月 ECS，并把空系统盘更换为 Ubuntu 22.04；独立 Ed25519 公钥与本地私钥指纹一致，`ecs-user` 新会话登录通过；
- 完成系统安全更新，安装 Docker Engine 29.1.3、Docker Compose v2 2.40.3 和 D1 运维所需基础工具；Docker 已开机启动，容器日志设为 10 MiB × 3 上限；
- 建立并验证 2 GiB Swap，关闭 SSH 密码认证、键盘交互认证和 root 登录；更新后的受控重启通过 SSH、Docker 与 Swap 复验；
- 未在宿主机安装第二套 Nginx，保持当前 D1 Compose 只有 Nginx 发布公网端口的边界；
- 账号专属 ACR 加速器已配置，Docker 配置校验、重启及 `hello-world` 运行通过；加速器未同步项目所需 PostgreSQL／Nginx 镜像，不能替代私有 Registry；
- 已实现 D1 `build`／`pull` 双镜像模式、摘要引用强制门禁、Linux AMD64 基础镜像复制／七个应用构建发布和摘要清单输出；`v0.1.0-test.1` 已真实发布到 11 个私有仓库，9 个 D1 运行引用均通过摘要与 `linux/amd64` 检查，同地域 ECS 拉取成功并在清除 Registry 登录缓存后保留全部镜像；
- 已增加 `bootstrap-host` 和 `bootstrap-remote-host`，把 Ubuntu 支持门禁、基础包、Docker 合并配置、2 GiB Swap、目录权限与 SSH 加固收口为可重复的 `plan/apply/check`；本地自测、远程只读检查、重复 `apply` 和再次检查均通过，17 项检查无失败；
- 当前备份目录仍位于系统盘，只是部署前暂存位置，不构成已完成的离机备份。

运维入口：[阿里云 ECS D1 宿主机初始化](./operations/aliyun-ecs-d1-bootstrap.md)／[ACR 私有镜像供应](./operations/acr-image-supply.md)／[阿里云账号协作管理接入](./operations/aliyun-account-access.md)

### M10-004 Contract Evidence 迁移（Phase A）

当前结果：

- 已建立迁移规格，固定“一条旧 Contract Evidence Version → 一个 Evidence File Version 1”，确定性复用旧 Version ID 作为新 File ID；Contract 继续拥有 Document、版本序号、目标、用途和历史审计；
- Contract `ops evidence-migration-preflight` 已实现只读扫描，查询真实 PostgreSQL，逐文件检查普通文件、大小和 SHA-256，输出确定性 JSON 候选映射；
- 单元测试、`go vet` 和全 Contract 模块构建通过；对当前本地 Contract 开发库的真实查询通过，该库当前没有历史附件，因此结果为 0 条候选；
- 写入导入、Contract 引用映射、双读切换、回退和含附件历史样本尚未执行；真实 OSS 核心链虽已通过，但 M10-004 与 M10 总里程碑仍未完成。

规格：[M10-004 Contract Evidence 迁移](./specs/evidence/m10-004-contract-evidence-migration.md)

运维准备：[阿里云 OSS 首次真实验收](./operations/aliyun-oss-first-validation.md)

### M10-003 Evidence Service D1 装配

范围与结果：

- Evidence 使用独立数据库、账号、migration 和对象卷，作为 `ENABLE_EVIDENCE` 控制的 Compose profile 接入 Suite Manifest；
- IAM 加载 Evidence Capability Manifest，Gateway 路由控制面；本地 PUT／GET 字节由 Nginx 直接转发到 Evidence、绕过 Gateway，并使用不含查询 Token 的专用访问日志；
- Workbench 支持 `show_in_workbench: false`，技术服务可被路由和授权但不作为最终用户业务菜单显示；
- D1 `check`、首管授权、升级、备份与恢复都包含 Evidence；备份期间停止写入进程并生成 `evidence-service.zip`；
- 全应用隔离栈完成 PDF Create → PUT → Complete → Access Grant → Range GET、服务端 SHA-256、单侧停机隔离、联合备份、销毁卷恢复及继续写入；
- 开关回归证明关闭后控制面和本地数据面均返回 `404`、进程停止、数据卷保留，重新开启后原 Version 保持；
- 空卷首次安装回归还修复了 PostgreSQL 本地健康已通过但 Compose 网络 TCP 尚未就绪的短暂竞态。

规格：[M10-003 Evidence D1 装配](./specs/evidence/m10-003-d1-integration.md)

### M10-002 Evidence Service 独立应用

范围与结果：

- 新增 `backend/services/evidence-service` 独立 Go 应用、PostgreSQL 18 migration、控制面 API、Template／IAM Ed25519 Provider 和 `evidence-service.view`／`evidence-service.upload` Capability Manifest；
- Upload Session 只接受服务端生成的 Object Key；Complete 从 Provider 重读对象并确认实际大小、媒体类型和 SHA-256，再原子形成不可变 File Version、幂等结果和审计；
- 本地 Provider 使用绑定 Tenant／Session／Object／动作／到期时间的短期 HMAC URL，并以不可覆盖发布保证并发 PUT 不改写对象；阿里云 Provider 使用官方 OSS／STS SDK、默认凭据链和单对象最小 Policy；
- Abort、过期、验证失败和数据库最终提交失败均进入可重试清理；已确认 Version 的对象受引用保护，多实例清理使用 PostgreSQL advisory lock 避免重复处理同一 Session；
- 提供 PostgreSQL＋本地对象一致性冷备份、严格空目标恢复、独立 Docker Compose、三个带 build identity 的二进制和可校验 release bundle。

验证证据：

- `go test ./...`、`go vet ./...` 和 `go build ./...` 通过；15 项普通测试覆盖配置、IAM、Provider、build identity 与备份格式；
- 真实 PostgreSQL `-race -tags=integration` 通过，包含 Evidence 生命周期、HTTP 数据面和完整备份恢复；
- 全新 Docker volumes 上跑通 Create → PUT → Complete → Access Grant → Range GET，随后完整销毁临时容器和 volumes；
- 本地冷备份恢复保留数据库事实与真实对象字节；Docker image 与开发模式 `v0.1.0-rc.1` release bundle 构建通过；
- 2026-09-22 已使用真实私有阿里云 Bucket `xidou-test-files` 通过精确 CORS 预检、1 MiB 上传下载、150 MiB 浏览器 multipart、进度、取消后清理、服务端 SHA-256 复核、同一临时 STS 越 Object Key `403 AccessDenied`、短期签名下载和匿名 GET `403`；Contract 遗留 Evidence 迁移、网络中断重试、凭证过期和公网入口为后续独立切片，因此 M10 总里程碑仍为部分完成。D1 Suite 装配已由 M10-003 完成。

规格：[M10-002 Evidence Service](./specs/evidence/m10-002-evidence-service.md)

运维：[安装升级](./operations/evidence-service-install-upgrade.md)／[备份恢复](./operations/evidence-service-backup-restore.md)

### M10-001 File Transfer Core 公共组件

范围与结果：

- 新增独立 `@omsbizfoundry/file-transfer-core` TypeScript 包，不依赖 Project、OA 分类、客户角色或页面权限；
- 新增 Evidence v1 OpenAPI 控制面契约，固定 create、complete、abort 和 access-grant 四类动作；
- Aliyun OSS Adapter 只接受 HTTPS、临时 Security Token、完全匹配的 Bucket／Object Key 和 `oss:PutObject`／`oss:AbortMultipartUpload`／`oss:ListParts`；Local HTTP Adapter 使用同一控制面契约；
- 浏览器 SDK 覆盖上传进度、用户取消、失败终止、同幂等键 complete 重试、短期下载、Blob／直连预览和按身份范围隔离缓存；
- 没有身份范围时不缓存访问授权；业务应用只接收 File ID／Version 并自行保存业务含义；
- 本切片本身不拥有服务端事实；后续 `M10-002` 已消费该契约建立独立服务，`M10-003` 已完成 D1 装配，Contract 迁移仍未完成，因此不宣称 M10 总里程碑完成。

验证证据：

- 严格 TypeScript 类型检查通过；
- 4 个测试文件、15 项测试全部通过；
- OpenAPI 主版本、固定端点、未知字段关闭和禁止业务字段守卫通过；
- 可发布构建与 `npm audit --omit=dev --audit-level=high` 通过，生产依赖未发现漏洞。

规格：[M10-001 File Transfer Core](./specs/evidence/m10-001-file-transfer-core.md)

### M8 People & Groups 最小纵向闭环

范围与结果：

- 新增独立 `backend/services/people-groups` Go 应用与 PostgreSQL 数据库，拥有 Person、Group、Membership、Group Type、Membership Role、Capability Definition、Group Capability 和 Roster Snapshot；
- 一名 Person 可同时属于多个 Group；同组当前 Membership 唯一，结束后可重新加入；角色变化、能力变化、Group revision、业务命令和审计全部保留不可变历史；
- Group 当前 revision 可发布唯一、原始 JSON 字节和 SHA-256 均冻结的 Roster Snapshot；Person 资料变化会推进其当前所属 Group revision，旧快照不变；
- 正式 IAM Ed25519 单 Audience 凭证与显式本地模板 Provider 产生同形 Principal；Membership Role 和 Group Capability 不参与 IAM 授权；
- D1 增加独立数据库账号、migration、应用路由、Workbench 入口、Capability Manifest、应用开关、故障隔离和联合备份恢复；只有 Nginx 暴露宿主端口。

验证证据：

- `GOWORK=off go test ./...`、`go vet ./...`、`go build ./...` 全部通过；
- 真实 PostgreSQL 集成测试覆盖多组、改角、结束／重入、并发冲突、幂等重放、历史 Trigger、能力与快照摘要；
- IAM 与 HTTP 负例覆盖错误 audience、缺少入口 action、无凭证 `401` 和缺少具体写 action `403`；
- D1 正常与破坏性验收均通过：停止 People 时 Contract／Money 继续，关闭 People 后路由与菜单消失且重新启用数据仍在；
- 联合备份生成 `people-groups.dump` 与 SHA-256；销毁 `omsd1m7006` 数据卷后恢复得到 `3 Person / 3 Group / 3 Membership / 3 Snapshot`，current-schema 和继续写入通过；恢复演练发现并修复 IAM 公钥卷首次创建时的目录属主缺陷。

规格：[M8 People & Groups 最小纵向闭环](./specs/people-groups/m8-minimal-vertical-slice.md)

### CL-M1-001 主体登记与列表

范围：

- 组织和个人主体；
- 法定名称或姓名；
- 可选登记标识；
- revision 1；
- PostgreSQL 持久化；
- 服务端列表和创建表单；
- CSRF 防护和输入大小限制。

验证证据：

- `make check` 通过；
- Party 领域校验单元测试通过；
- HTTP 创建、列表、校验错误和 CSRF 单元测试通过；
- PostgreSQL migration 重复执行测试通过；
- PostgreSQL repository 创建和读取集成测试通过；
- 本地 HTTP 表单创建后，页面和数据库均读取到清理后的数据；
- Contract Ledger 在 Money Flow 数据库停止时独立运行。

规格：[主体登记与列表](./specs/contract-ledger/party-registration.md)

### CL-M1-002 合同草稿创建与列表

范围：

- 稳定 Contract UUID 和创建 Command ID；
- 编号命名空间、合同编号、标题和通用分类；
- 精确十进制可选总额和 ISO 风格三字母币种；
- 可选签署日期和生效日期；
- `draft` 状态、draft version 1，尚不产生正式 revision；
- PostgreSQL 持久化、服务端创建表单和列表；
- 相同命令幂等重放、命令负载冲突和不区分大小写的合同编号冲突；
- CSRF 防护和输入大小限制。

验证证据：

- `make check` 通过；
- 合同领域字段、金额、币种、日期和创建幂等规则单元测试通过；
- HTTP 创建、列表、校验错误、编号冲突、CSRF 和请求大小限制单元测试通过；
- PostgreSQL migration 重复执行测试通过；
- PostgreSQL repository 创建、读取、精确金额、幂等重放、命令负载冲突和编号冲突集成测试通过；
- 本地 HTTP 验收覆盖金额已确定与金额待确定两类草稿，页面与数据库结果一致；
- Contract Ledger 在不启动 Money Flow 的情况下独立完成验收。

规格：[合同草稿创建与列表](./specs/contract-ledger/contract-draft-create.md)

### CL-M1-003 合同草稿签约主体与角色

范围：

- 合同草稿详情页面；
- 从本系统轻量主体目录选择签约主体；
- 不绑定客户岗位或行业枚举的自由文本合同角色；
- 同一主体多角色、多个主体同角色；
- 有效“合同 + 主体 + 角色”唯一约束；
- 草稿阶段读取 Party 当前资料，激活时再冻结快照；
- 添加与移除命令幂等；
- Expected Draft Version 乐观并发检查；
- 每次成功变更递增 `draft_version`；
- 移除保留历史记录，不物理删除关联。

验证证据：

- 领域输入、角色和幂等规则单元测试通过；
- HTTP 详情、添加、移除、并发冲突和 CSRF 单元测试通过；
- 真实 PostgreSQL migration 重复执行通过；
- PostgreSQL 集成测试覆盖同主体多角色、同角色多主体、重复关联、缺失主体、过期版本、添加/移除重放、跨动作命令冲突、软移除后重新添加和非草稿拒绝；
- 本地 HTTP 与数据库联合验收得到草稿版本 6、3 个有效关联、1 个移除历史和 5 条成功命令，幂等重放未增加记录或版本；
- Contract Ledger 在不启动 Money Flow 的情况下独立完成验收。

规格：[合同草稿签约主体与角色](./specs/contract-ledger/contract-draft-parties.md)

### CL-M1-004 合同草稿编辑与取消

范围：

- 完整替换合同草稿基本字段；
- 与创建一致的编号、标题、分类、币种、精确金额和日期规则；
- 无变化提交拒绝且不产生空版本；
- 编号变更继续执行不区分大小写的唯一约束；
- Expected Draft Version 乐观并发检查；
- 编辑与取消命令幂等；
- 带必填原因的草稿取消；
- 取消后保留签约主体和命令历史，并进入只读状态；
- 取消不生成正式 revision。

验证证据：

- 编辑、取消、字段标准化、校验和幂等规则单元测试通过；
- HTTP 编辑、取消、只读详情、空变更、CSRF 和请求大小限制单元测试通过；
- 真实 PostgreSQL migration 重复执行通过；
- PostgreSQL 集成测试覆盖编辑重放、负载冲突、无变化、重复编号、过期版本、取消重放、跨动作命令冲突及取消后拒绝全部写入；
- 本地 HTTP 与数据库联合验收确认：编辑后版本 3，取消后版本 4，`current_revision` 为空，保留 1 个签约主体和 2 条成功变更命令；
- Contract Ledger 在不启动 Money Flow 的情况下独立完成验收。

规格：[合同草稿编辑与取消](./specs/contract-ledger/contract-draft-edit-cancel.md)

### CL-M1-005 合同激活、签约主体快照与 revision 1

范围：

- 签署日期、生效日期和至少两个不同签约主体的激活前置条件；
- Expected Draft Version 乐观并发；
- 激活命令幂等；
- 原子冻结 revision 1 合同条款；
- 冻结 Party ID、Party revision、名称、登记标识和合同角色；
- 状态转为 `active`，`current_revision` 设为 1；
- 最小 `contract_activated` Actor 审计；
- 数据库级不可更新、不可删除保护；
- active 详情只读取 revision 与 Party Snapshot；
- 初始验收使用过渡的本地单操作者来源；`CL-M1-006` 已替换为登录会话中的可信 Actor。

验证证据：

- 激活输入、Actor Ref、幂等和负载冲突单元测试通过；
- HTTP 激活准备提示、提交、active 只读详情、CSRF 和请求大小限制单元测试通过；
- 真实 PostgreSQL migration 重复执行通过；
- PostgreSQL 集成测试覆盖缺少日期、主体不足、过期版本、激活重放、不同命令重复激活及激活后写入拒绝；
- 直接修改 Party 当前资料后，revision 1 主体快照保持原名称和原 Party revision；
- 直接更新或删除 activation command、revision、Party Snapshot 和 audit 均被 PostgreSQL `55000` 不可变触发器拒绝；
- 故意制造审计 ID 冲突时，合同状态、revision、快照、命令和审计全部回滚；
- 本地 HTTP 与数据库联合验收确认：`active|draft_version 3|revision 1`、2 个主体快照、1 条激活审计；
- Money Flow 未启动且没有产生应收应付等跨系统事实。

规格：[合同激活、签约主体快照与 revision 1](./specs/contract-ledger/contract-activation-revision-1.md)

### CL-M1-006 最小本地身份授权与通用写入审计

范围：

- 空库显式 bootstrap 首位管理员，重复启动不覆盖账号；
- 本地用户、BCrypt 密码摘要、SHA-256 会话令牌摘要和绝对会话有效期；
- 固定 administrator、editor、viewer 角色及服务端授权；
- 登录失败统一响应、5 次失败锁定 15 分钟、注销和旧会话失效；
- 登录会话生成可信 `local_user` Actor，请求不能覆盖；
- 七类业务写入和六类身份写入的通用不可变审计；
- 业务事实、身份变化与审计原子提交；
- 历史写入审计投影，不猜测缺失 Actor；
- 管理用户、修改自己的密码和查看全局审计的服务端页面；
- Contract Ledger 内置默认实现，不形成跨系统身份平台。

验证证据：

- 身份领域单元测试覆盖角色权限、bootstrap、BCrypt、通用认证错误、令牌摘要和密码边界；
- HTTP 测试覆盖匿名边界、Cookie 属性、CSRF、三角色权限、隐藏写表单和可信会话 Actor；
- PostgreSQL 身份集成测试覆盖登录锁定、会话解析与撤销、停用/启用、角色变化、密码变化、最后管理员保护、审计不可变和失败回滚；
- migration 集成测试从 `000006` 历史库升级，核对历史审计数量、未知 Actor 标记、激活 Actor 保留和 migration 重放；
- 业务 repository 集成测试覆盖七类写入审计、幂等不重复和审计失败时业务回滚；
- 隔离空库 HTTP 联调完成管理员登录、创建 editor/viewer、editor 创建主体、viewer 写入被拒绝和 viewer 查看可信审计；
- 单元测试、`go vet` 与真实 PostgreSQL 集成测试均通过；
- Money Flow 未启动，Contract Ledger 仍独立完成全部链路。

规格：[最小本地身份、权限与写入审计](./specs/contract-ledger/local-identity-authorization-audit.md)  
决策：[ADR-0003：Contract Ledger 内置最小本地身份](./adr/0003-contract-ledger-local-identity.md)

### CL-M2-001 生效补充协议与 revision 2+

范围：

- 原合同与补充协议组成不可变 Contract Instrument 时间线；
- 既有 active 合同无损回填顺序 1 的原合同 Instrument；
- Expected Revision 与显式连续生效顺序控制 revision 2+；
- adjustment、restatement、unchanged 三种精确金额语义；
- 每个新 revision 复制非金额条款和全部 Party Snapshot；
- active 合同当前金额从最新 revision 读取；
- 命令幂等、并发冲突、合同内文书编号唯一和生效日期不倒退；
- Instrument、change command、revision、Party Snapshot 与审计原子写入并受数据库不可变保护；
- administrator/editor 可登记，viewer 只读，Actor 只能来自会话；
- Instrument 时间线、revision 历史、当前金额和审计的服务端页面。

验证证据：

- 金额规范化和精确加减、金额未知、负结果、零调整与超精度领域测试通过；
- PostgreSQL 集成测试覆盖连续多份补充协议、三种金额语义、幂等重放、负载冲突、过期 revision、倒退日期和大小写等价编号冲突；
- 两个命令并发消费同一 revision 时恰好一个形成下一 revision，另一个得到明确 revision 冲突；
- 直接修改或删除 Instrument、change command、revision、Party Snapshot 和审计均被数据库不可变触发器拒绝；
- 故意制造审计冲突时，Instrument、命令、revision、快照与 head revision 全部回滚；
- 隔离空库 HTTP 联调完成 editor 创建两个主体、合同、激活 revision 1、登记金额调整补充协议形成 revision 2，以及 viewer 读时间线但写入被拒绝；
- 页面与数据库共同确认原金额 `1000` 经 `-100.25` 调整后为 `899.75`，revision 2 保留两个主体快照，审计 Actor 为登录 editor；
- `make check`、Contract Ledger 全量 `-race` 单元测试和真实 PostgreSQL `-race` 集成测试通过；
- 两个应用在 `GOWORK=off` 下分别通过测试和构建；
- 本地开发库迁移到 `000008_effective_supplement_revisions.sql` 后，active 合同与原合同 Instrument 数量一致，无 revision 来源缺口或 head revision 不一致；
- Money Flow 数据库和应用均停止，Contract Ledger 本地服务健康并独立完成验收。

规格：[生效补充协议与 revision 2+](./specs/contract-ledger/effective-supplement-revision.md)  
决策：[ADR-0004：合同 Instrument 与显式 revision 顺序](./adr/0004-contract-instrument-revision-sequence.md)

### CL-M2-002 生效终止文书

范围：

- 独立 `termination_instrument` 与类型化终止命令；
- Expected Revision、显式连续顺序和生效日期不倒退；
- 终止文书形成下一完整 revision 并复制全部 Party Snapshot；
- 合同状态从 `active` 原子推进为 `terminated`；
- 已确定金额和待确定金额都保持上一 revision 原值，不产生结算金额；
- 终止、取消、未来操作性关闭和结算事实相互分离；
- 命令幂等、合同内文书编号唯一以及终止／补充协议跨动作并发保护；
- termination command、Instrument、revision、快照和审计原子写入且不可变；
- administrator/editor 可登记，viewer 只读，Actor 只能来自会话；
- 列表、详情、Instrument 时间线、revision 历史和审计明确显示终止终态。

验证证据：

- 终止输入标准化、字段边界、连续顺序和幂等冲突单元测试通过；
- migration 集成测试从历史数据库连续升级并重复执行 `000009`，既有合同与文书事实保持有效；
- PostgreSQL 集成测试覆盖 active 成功终止，草稿、已取消和已终止拒绝，倒退日期、重复编号、重放与负载冲突；
- 金额 `15000.125` 和金额待确定两类合同终止后均保持原值，终止 revision 完整复制 Party Snapshot；
- 两个终止命令并发时恰好一个成功；补充协议与终止命令并发消费同一 revision 时也恰好一个成功且只形成一份顺序事实；
- 直接修改 termination command 被 PostgreSQL `55000` 不可变触发器拒绝，Instrument、revision、快照和审计沿用并通过既有不可变验证；
- 故意复用审计 ID 使事务末端失败时，合同仍为 `active` revision 1，未留下终止 Instrument、命令、revision、快照或审计；
- 隔离空库 HTTP 联调完成 editor 从激活 revision 1、金额调整 revision 2 到终止 revision 3 的完整链路；
- 终止页面和合同列表显示 `terminated` revision 3，当前金额保持 `899.75 CNY`；终止后 editor 追加补充协议被拒绝，viewer 仍只能读取；
- `make check`、Contract Ledger 全量 `-race` 单元测试和真实 PostgreSQL `-race` 集成测试通过；
- 两个应用在 `GOWORK=off` 下分别通过测试和构建；
- 本地开发库升级到 `000009_effective_termination_instrument.sql`，无 revision 来源缺口、head revision 不一致或测试合同残留；
- Money Flow 数据库和应用均停止，Contract Ledger 最新服务健康并独立完成验收。

规格：[生效终止文书](./specs/contract-ledger/effective-termination-instrument.md)  
决策：[ADR-0005：合同终止是独立的终态 Instrument](./adr/0005-contract-termination-is-a-terminal-instrument.md)

### CL-M2-003 合同检索、状态筛选与稳定分页

范围：

- 对编号命名空间、合同编号和标题执行不区分大小写的字面量包含检索；
- 按 draft、active、terminated、closed、cancelled 状态和三字母币种精确筛选；
- 关键词、状态和币种在领域用例层统一标准化与校验；
- 固定 `(created_at DESC, id DESC)` 顺序和每页 50 条的 keyset 分页；
- 版本化 URL-safe 游标绑定标准化筛选条件，拒绝损坏或混用游标；
- 正式合同列表金额只读取当前 revision，草稿和取消合同读取草稿金额；
- 查询表单、筛选值保留、清除筛选、空结果提示和条件化下一页入口；
- administrator、editor、viewer 共享只读检索能力，写入权限保持不变；
- PostgreSQL 参数化查询和状态／币种分页索引；
- 不引入全文搜索服务、跨系统统一搜索、导出或批量写入。

验证证据：

- 领域单元测试覆盖输入标准化、所有非法边界、游标筛选绑定、51 条探测和通配字符转义；
- HTTP 单元测试覆盖筛选参数传递、值保留、下一页 URL、`422` 错误、终止状态显示和 viewer 只读查询；
- migration 集成测试连续升级并重复执行 `000010`，核对两个预期索引存在；
- 真实 PostgreSQL 以 53 条同筛选记录验证第一页 50 条、第二页 3 条且无重复遗漏；第一页后新增的更靠前记录不会污染旧游标第二页；
- `%`、`_`、反斜杠、单引号和大小写混合标题的真实 PostgreSQL 查询各只命中字面量目标记录；
- 已终止合同按状态和币种检索时显示最新 revision 6 及精确金额 `15000.125`；
- 隔离空库 HTTP 联调从合同建立、激活、补充协议到终止后，再按关键词、状态和币种检索并验证非法游标 `422`；
- `make check`、Contract Ledger 全量 `-race` 单元测试和真实 PostgreSQL `-race` 集成测试通过；
- 两个应用在 `GOWORK=off` 下分别通过测试和构建；
- 本地开发库升级到 `000010_contract_search_indexes.sql`，两个索引存在，检索测试残留、revision 来源缺口和 head revision 错配均为 0；
- Money Flow 数据库和应用均停止，Contract Ledger 最新服务健康并独立完成验收。

规格：[合同检索、状态筛选与稳定分页](./specs/contract-ledger/contract-search-filter-pagination.md)

### CL-M2-004 合同关系图

范围：

- 合同关系作为独立不可变有向边，不把 Contract 放入固定父子树；
- 内置资金来源、委托依据、分包、框架下具体合同、替代和引用六种明确方向；
- 关系保存来源和目标登记时的 revision 复合外键、可选生效日期／说明及可信 Actor；
- 关系不产生或推进合同 revision，不传播金额、主体、条款、状态或生命周期；
- active、terminated、closed 正式合同可以参与，draft/cancelled 明确拒绝；
- 两端 Expected Revision、固定合同行锁顺序、Command ID 幂等和唯一边约束；
- 五种结构性类型按类型事务锁和递归路径检查防环，`references` 允许双向；
- Relation、command 和通用审计在一个事务内提交并受数据库不可变保护；
- 目标合同检索／稳定分页、方向确认，以及详情出向／入向关系查看；
- administrator/editor 可建立，viewer 只读；不建立关系类型平台、图数据库或跨系统动作。

验证证据：

- 领域单元测试覆盖六种类型、方向标签、防环策略、输入标准化、全部字段边界和幂等负载冲突；
- HTTP 单元测试覆盖目标检索／分页、选定确认、两端 Expected Revision、可信会话 Actor、冲突重渲染、双向详情和 viewer 拒绝；
- migration 集成测试连续升级并重复执行 `000011`，核对两张表、两个索引和两个不可变触发器；
- PostgreSQL 集成测试建立六种关系，验证 source/target revision 快照、出向／入向投影、相同命令重放和不同命令重复边冲突；
- draft 与 cancelled 状态、来源与目标 revision 过期都明确拒绝；
- 三节点间接环被拒绝；并发反向结构边恰好一个成功；双向普通引用均成功；
- 两个命令并发建立相同边时恰好一个成功；审计失败时 Relation 和 command 全部回滚；
- Relation 与 command 的更新和删除都被 PostgreSQL `55000` 拒绝；
- 隔离空库 HTTP 联调新增并激活关系目标合同，从 terminated revision 3 合同检索目标、建立 `references`、从两端查看，并确认审计 Actor 为登录 editor；
- `make check`、Contract Ledger 全量 `-race` 单元测试和真实 PostgreSQL `-race` 集成测试通过；
- 两个应用在 `GOWORK=off` 下分别通过测试和构建；
- 本地开发库升级到 `000011_contract_relations.sql`，关系测试合同、snapshot 缺口、command/relation 不一致、head revision 错配和 revision 来源缺口均为 0；
- Money Flow 数据库和应用均停止，Contract Ledger 最新服务健康且未产生跨系统事实。

规格：[合同关系图](./specs/contract-ledger/contract-relations.md)  
决策：[ADR-0006：合同关系是引用 revision 快照的独立有向边](./adr/0006-contract-relations-are-versioned-graph-edges.md)

### CL-M2-005 阶段与最终结算事实

范围：

- `progress` 阶段结算和 `final` 最终结算均按截至当次确认的累计金额保存；
- 同一合同使用从 1 开始的连续 settlement sequence，阶段金额不求和，final 后不再接受新结算；
- 结算独立于 Contract Instrument、revision 和生命周期，不覆盖原始金额或当前有效金额；
- active 与 terminated 合同可登记，draft、cancelled、closed 明确拒绝；
- 结算引用 source revision、继承币种，并冻结用户明确勾选的确认主体快照；
- Expected Contract Revision、Expected Settlement Sequence、Contract 行锁、Command ID 幂等和合同内编号唯一；
- Settlement、command、确认主体快照和通用审计原子提交并受数据库不可变保护；
- administrator/editor 可登记，viewer 只读；详情并列显示原始金额、当前有效金额和最新累计结算金额；
- 不创建义务、资金、核销、文件或跨系统事件。

验证证据：

- 领域单元测试覆盖两种类型、全部字段边界、零金额、18+6 精度、主体位置排序／重复拒绝和完整幂等负载比较；
- HTTP 单元测试覆盖金额分离、结算时间线、终止后登记、final 后隐藏表单、可信会话 Actor、错误状态映射和 viewer 直接 POST `403`；
- migration 集成测试连续升级并重复执行 `000012`，核对三张表、三个索引和三个不可变触发器；
- 真实 PostgreSQL 验证当前有效金额待确定时记录零金额，重述为 `1500` 后记录高于它的 `2000`，最终记录低于它的 `1200.5`，三种金额始终分离；
- active 阶段结算、terminated 最终结算、draft/cancelled/closed/final 后拒绝、大小写等价编号冲突和错误主体 position 均通过；
- Party 当前名称变化及后续 contract revision 不改写旧确认主体快照；只选择一方和明确选择双方都按原选择保存；
- 同一命令重放不增加事实，改变负载产生幂等冲突；两个命令并发消费同一 sequence 时恰好一个成功；
- 故意复用审计 ID 时 Settlement、command 和确认主体全部回滚；直接更新或删除 Settlement、command、快照和审计均被 PostgreSQL `55000` 拒绝；
- 隔离空库 HTTP 联调完成激活、补充协议、阶段结算、终止和最终结算，页面、数据库、确认主体与可信 Actor 审计一致；
- `make check`、两个应用全量 `-race`、Contract Ledger 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过；
- 本地开发库升级到 `000012_contract_settlements.sql`，三个结算索引和三个不可变触发器存在，集成测试结算事实残留为 0；
- Money Flow 数据库和应用均停止，Contract Ledger 最新服务健康并独立完成全部结算验收。

规格：[阶段与最终结算事实](./specs/contract-ledger/contract-settlements.md)  
决策：[ADR-0007：合同结算是独立的累计金额事实序列](./adr/0007-contract-settlements-are-cumulative-facts.md)

### CL-M2-006 操作性关闭

范围：

- `active` 或 `terminated` 合同可形成唯一、不可变的操作性关闭事实并进入 `closed`；
- 关闭与法律终止、履约完成、最终结算和财务清偿保持分离；
- 必须明确选择引用当前 final Settlement，或确认无需独立 final Settlement；
- 关闭日期不得早于当前 revision 来源文书生效日或最新 Settlement 确认日；
- 同时消费 Expected Contract Revision 与 Expected Settlement Sequence，并与补充、终止、结算共用 Contract 行锁；
- 关闭不生成 Instrument 或 revision，不修改原始金额、当前有效金额或 Settlement；
- Closure、command、状态投影和通用审计原子提交且受不可变保护；
- administrator/editor 可关闭，viewer 只读；closed 页面收起常规变更、终止、结算和再次关闭入口；
- 不创建 Money Flow 义务、资金、核销或跨系统事件，也不提供重新打开能力。

验证证据：

- 领域与 HTTP 单元测试覆盖输入标准化、结算处置、日期前置条件、幂等、可信 Actor、页面收口、错误映射和 viewer 直接 POST `403`；
- migration 集成测试连续升级并重复执行 `000013`，核对两张表、两个不可变触发器和四个关键约束；
- 真实 PostgreSQL 覆盖 active 无 final、active 有 progress、terminated 有 final、draft/cancelled/closed 拒绝，以及 revision/sequence/日期过期冲突；
- 相同命令重放不重复事实，改变负载产生幂等冲突；关闭与结算并发消费同一状态时恰好一个成功；
- Closure、command 和审计均不可更新或删除；故意复用审计 ID 时 Closure、command 与 `closed` 状态整体回滚；
- 隔离空库 HTTP 联调完成激活、补充协议、阶段结算、终止、最终结算和关闭，关闭后仍为 revision 3、当前金额仍为 `899.75 CNY`，并引用真实 final Settlement；
- closed 合同仍可按既有规则建立关系，但不能再执行普通变更、终止、结算或再次关闭；viewer 可读取关闭事实但不能写入；
- `make check`、两个应用全量 `-race`、Contract Ledger 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过；
- 本地开发库升级到 `000013_contract_operational_closures.sql`，两个触发器和四个约束存在，关闭事实残留、命令不一致、final 引用错配、revision 来源缺口和 head 错配均为 0；
- Money Flow 数据库和应用均停止，Contract Ledger 最新服务健康并独立完成全部关闭验收。

规格：[操作性关闭](./specs/contract-ledger/contract-operational-closure.md)  
决策：[ADR-0008：合同关闭是独立的操作性终态事实](./adr/0008-contract-closure-is-an-operational-terminal-fact.md)

### CL-M2-007 合同文件证据

范围：

- Evidence Document 表示引用 Contract、Instrument、Settlement 或 Closure 的稳定逻辑文件，Evidence Version 保存不可变字节版本；
- 每个 Version 保存服务端探测的媒体类型、实际大小、SHA-256、原始文件名、内部 storage key、Actor 和时间；
- 内置本地文件适配器使用服务端 UUID 路径、限制权限、临时文件和原子 rename，不把用户文件名用于存储路径；
- 创建 Document 时形成 Version 1，后续按 Expected Latest Version 连续追加；相同 Command ID 可安全重放，改变意图或内容产生冲突；
- 数据库失败后补偿删除候选文件，Document、Version、command、上传审计原子提交且受不可变保护；
- 下载前在同一文件描述符重新校验大小和 SHA-256，并强制 attachment、octet-stream、nosniff、no-store；下载审计失败时拒绝交付；
- administrator/editor 可上传和追加版本，viewer 可查看和下载但不能上传，匿名不能下载；
- Evidence 追加不改变合同状态、draft version、revision、金额或 settlement sequence，也不依赖 Money Flow Ledger、对象存储或共享文件平台。

验证证据：

- 领域与本地存储单元测试覆盖输入标准化、路径安全、空文件／超限文件、实际媒体类型和 SHA-256、幂等重放、版本冲突、损坏内容拒绝与候选文件清理；
- HTTP 单元测试覆盖页面历史、四类目标选项、可信会话 Actor、Version 1／追加版本输入、强制下载响应头、viewer 只读权限和错误映射；
- migration 集成测试连续升级并重复执行 `000014`，核对三张 Evidence 表、目标校验触发器、三类不可变触发器、显式索引与关键约束；
- 真实 PostgreSQL 覆盖五种 Contract 状态、四种本地目标、跨合同／不存在目标拒绝、顺序与并发版本、命令重放／冲突、下载审计、审计失败回滚和不可变保护；
- 隔离空库 HTTP 联调在已关闭合同上完成 Version 1 与 Version 2，验证页面历史、数据库元数据、原始字节、编辑者与 viewer 下载审计，以及匿名／viewer 写入边界；
- `make check`、两个应用全量 `-race`、Contract Ledger 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过；
- 本地开发库升级到 `000014_contract_evidence.sql`，Money Flow 数据库和应用均停止，Contract Ledger 使用自身 Evidence 目录独立完成全部验收。

规格：[合同文件证据](./specs/contract-ledger/contract-evidence.md)  
决策：[ADR-0009：合同证据使用版本化引用与内置文件适配器](./adr/0009-contract-evidence-uses-versioned-references.md)

### CL-M2-008 已生效合同核心条款受控更正

范围：

- administrator 使用独立 `correction_instrument` 更正 current revision，并连续形成 revision N+1；
- active、terminated、closed 可更正且生命周期状态不变，draft/cancelled 拒绝；
- 第一版只完整重述标题、通用分类、当前有效金额、合同签署日期和生效日期；
- 编号、币种与 Party Snapshot 从上一 revision 原样复制，旧 revision 永久保留；
- Expected Status、Expected Revision、连续 Effective Order、文书日期顺序与至少一个字段变化共同构成前置条件；
- Instrument、revision、快照、不可变 command、current revision 投影与可信 Actor 审计原子提交；
- 正式合同列表、检索和详情从 current revision 读取，旧 Settlement、Relation、Closure 和 Evidence 引用不漂移；
- `business.correct` 仅授予 administrator，editor/viewer 只读；更正文书自动成为 Evidence 目标；
- 不提供任意 JSON Patch，不修改编号、币种、主体，不撤销终止或重新打开 closed，不创建跨系统动作。

验证证据：

- 领域单元测试覆盖输入标准化、精确金额、日期、完整字段边界、无变化拒绝和幂等意图比较；
- HTTP 单元测试覆盖 active/terminated/closed 表单、当前字段默认值、可信会话 Actor、错误映射，以及 administrator/editor/viewer/匿名授权边界；
- migration 集成测试连续升级并重复执行 `000015`，核对 correction command 表、索引、约束和不可变触发器；
- PostgreSQL 集成测试覆盖三种正式状态成功更正、draft/cancelled 拒绝、过期状态/revision、日期倒退、重复编号、命令重放与负载冲突；
- 新 revision 的编号、币种和两份 Party Snapshot 与上一 revision 一致，旧 revision 保持原值，列表和检索只命中更正后的当前标题；
- 更正与补充协议并发消费 revision 1 时恰好一个成功；审计失败不留下 Instrument、revision 或 command；直接修改新旧事实被 PostgreSQL `55000` 拒绝；
- 隔离空库 HTTP 联调在激活、补充、结算、终止、关闭、Evidence 与关系事实之后执行 closed 更正，状态仍为 closed 且旧 Closure、final Settlement 与 Relation 继续引用 revision 3；
- 更正文书 Evidence 上传成功且不推进 revision，页面、数据库与管理员审计 Actor 一致；
- 全仓质量门禁、竞态测试、真实 PostgreSQL 集成测试、两应用 `GOWORK=off` 独立测试／构建与最新本地服务健康检查通过；
- Money Flow 数据库和应用停止，Contract Ledger 独立完成全部验收。

规格：[已生效合同核心条款受控更正](./specs/contract-ledger/effective-contract-correction-revision.md)  
决策：[ADR-0010：合同核心条款更正追加新 Revision](./adr/0010-contract-correction-appends-a-new-revision.md)

### CL-M2-009 单合同可验证账簿导出

范围：

- administrator 通过 `business.export` 下载一份 Contract ID 的版本化 ZIP，editor/viewer 即使可读合同页面也不能导出；
- 归档格式固定为 `omsbizfoundry.contract-ledger/contract-export-v1`，根目录只含 `manifest.json` 与 `ledger.json`；
- manifest 保存生成时间、Contract ID、ledger 字节数和精确字节 SHA-256，可用标准工具离线核验；
- ledger 包含当前投影、取消事实、全部草稿主体关联、全部 Instrument／Revision／Party Snapshot、双向 Relation、Settlement、Closure、Evidence 校验元数据和通用审计；
- PostgreSQL 在一个 `REPEATABLE READ, READ ONLY` 事务中归集全部事实，集合按确定顺序输出且空集合编码为 `[]`；
- Evidence 二进制、`storage_key`、存储 provider、服务器路径、身份凭证和内部 payload hash 明确排除；
- 服务先完成归档，再追加 `contract_ledger_exported` 可信 Actor 审计；审计失败时不发送归档，当前审计不进入自身快照；
- 下载强制 attachment、`application/zip`、no-store 和 nosniff，文件名只使用稳定 Contract ID；
- 导出不修改状态、draft version、current revision 或任何业务事实，不访问 Money Flow；
- 本功能不包含导入、恢复、全库备份、Evidence 文件打包、CSV/Excel 或通用报表。

验证证据：

- 归档单元测试覆盖双文件结构、文件权限、JSON 可读性、精确 SHA-256、Evidence 脱敏、可信 Actor 审计以及审计失败时零 Artifact；
- 五种生命周期状态均通过服务边界，空事实集合稳定输出 `[]`，非法 Contract ID、未知合同和无效 Actor 明确拒绝；
- HTTP 单元测试覆盖管理员入口／响应头／可信 Actor、editor/viewer 页面隐藏与直接 GET `403`、匿名登录跳转及错误时不发送 ZIP；
- 隔离空库真实 PostgreSQL 联调导出复杂 closed 合同，核对四份 Instrument、四个 Revision 及每版两份 Party Snapshot；
- 更正后的 Closure、final Settlement 和 Relation 仍引用 revision 3，当前投影为 revision 4，Evidence 只保留文件名、媒体类型、大小与 SHA-256；
- 同一 Relation 在来源合同与目标合同导出中均存在；draft 导出无伪造 Revision，cancelled 导出保留取消原因和 editor Actor；
- 第一次导出不含自身审计，第二次包含前次导出审计；成功审计落库后 current revision 仍为 4；
- `make check`、Contract Ledger 全量 `-race`、真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过；
- Money Flow 数据库和应用停止，Contract Ledger 独立完成全部导出验收。

规格：[单合同可验证账簿导出](./specs/contract-ledger/verifiable-contract-ledger-export.md)  
决策：[ADR-0011：合同导出采用可验证的单合同快照归档](./adr/0011-contract-export-is-verifiable-snapshot-archive.md)

### CL-M2-010 空业务库单合同账簿恢复

范围：

- administrator 通过独立 `business.restore` 权限上传 `contract-export-v1` ZIP，editor/viewer 看不到入口且直接访问返回 `403`；
- ZIP 在写库前严格验证根目录双文件、普通文件与路径安全、压缩／解压大小、严格 JSON、格式版本、Contract ID、ledger 字节数和 SHA-256；
- 预检验证生命周期、连续 Instrument／Revision、精确金额转换、Party Snapshot、Settlement、Closure、Evidence 目标和来源审计；
- 目标只允许已有 schema 与本地身份、但 Contract 和 Party 同时为空；不覆盖、不合并、不清空已有业务数据；
- 任意 Relation 令整个单合同包拒绝，不创建占位合同或静默丢边；
- 单个 `SERIALIZABLE` PostgreSQL 事务恢复合同、主体／关联、取消、正式文书／Revision／Snapshot、结算、关闭、Evidence 元数据、必要命令链和源审计；
- Evidence Version 恢复为 `missing`，`storage_provider`／`storage_key` 为空，页面无下载链接且直接请求不写成功下载审计；
- Restore Command ID 和 ledger SHA-256 提供幂等重放；成功事务追加唯一 `contract_ledger_restored` 本地管理员审计；
- 恢复不读取或写入 Money Flow，不产生义务、资金、核销、事件或连接器调用。

验证证据：

- 归档单元测试覆盖非 ZIP、多余／重复／穿越路径、摘要变化、未知字段、外部 Relation、生命周期和引用破坏；
- 真实 PostgreSQL 验证 draft、cancelled 及复杂 closed 往返；复杂场景包含两主体、补充协议、阶段／最终结算、终止、关闭、更正和 Closure Evidence；
- 目标重新导出后当前投影、四份 Instrument／Revision、Party Snapshot、两条 Settlement、Closure 和源审计保持语义一致；只允许新增恢复审计和 Evidence 可用状态变化；
- 相同 Command ID、相同 ledger SHA-256 的不同请求均安全重放；改变同一命令 Actor 冲突，非空业务库明确拒绝；
- 在源审计写入阶段故意制造主键冲突，Contract、Party、Audit 和 restore operation 均为零，证明末端失败整体回滚；
- 恢复事实继续受不可变触发器保护；缺失 Evidence 下载拒绝且无下载审计，随后追加本地 Version 2 正常可用；
- HTTP 单元测试覆盖管理员入口、可信 Actor、CSRF、显式空库确认和 `413`／`422`／`409` 映射；
- 只有本地管理员的空业务库经真实 HTTP 完成上传、恢复、查看和再导出；migration 000016 可重放且约束／不可变触发器存在；
- 全仓质量门禁、竞态测试、真实 PostgreSQL 集成和两个应用 `GOWORK=off` 独立测试／构建通过；Money Flow 停止。

规格：[空业务库单合同账簿恢复](./specs/contract-ledger/empty-database-single-contract-restore.md)  
决策：[ADR-0012：单合同恢复采用空业务库原子导入](./adr/0012-restore-is-atomic-empty-business-database-import.md)

### CL-M2-011 可校验全量备份与空环境恢复

范围：

- `backup create` 通过导出 PostgreSQL snapshot 让 custom dump、schema／事实计数和 available Evidence 引用处于同一 MVCC 时点；
- 版本化 `full-backup-v1` ZIP 严格包含 custom-format `database.dump`、manifest 和 snapshot 实际引用的 Evidence 普通文件；
- Evidence 逐项核对 storage key、大小和 SHA-256，缺失、替换或非普通文件令整次备份失败且不发布最终文件；
- dump 排除 `local_sessions` 表数据，保留本地用户、密码摘要、审计和全部业务事实；
- 最终归档 `0600`、不覆盖同名输出，PostgreSQL 客户端 major 与服务器 major 显式一致；
- `backup restore` 只接受完全空的数据库和不存在的 Evidence 根目录，拒绝清空、覆盖与合并；
- 恢复前严格校验 ZIP 路径、普通文件、store method、未知／重复项、manifest、dump 和 Evidence 摘要；
- Evidence 在同级私有目录暂存后原子发布，`pg_restore --single-transaction` 失败时只凭本次随机标记回收自己发布的目录；
- 恢复成功后核对 migration、事实计数、零旧会话及数据库／manifest／文件三方 Evidence 完整性；
- 提供生产边界、本地 Compose wrapper 和完整恢复演练运行手册，不扩展成通用备份平台。

验证证据：

- 单元／竞态测试覆盖严格 manifest、摘要变化、路径穿越、未知或压缩 entry、同名输出保护、恢复目录所有权标记及 PostgreSQL major 解析；
- 真实 PostgreSQL 集成测试建立临时源／目标数据库，写入本地管理员、有效登录会话、合同和真实 Evidence 字节，在线备份后完整恢复；
- 恢复结果保留用户密码、合同和 Evidence 内容，备份前会话从 1 条变为 0 条，migration 可安全重放；
- 暂时移走 snapshot 引用的 Evidence 时备份明确失败且没有最终 ZIP；输入结构有效但 dump 无效时，真实 `pg_restore` 失败，目标数据库保持零用户 relation，已发布 Evidence 被安全回收；
- 现有开发库真实备份包含 6 份 Contract、6 个 Party 和 schema 000016，恢复到新空数据库后计数一致且旧会话为 0；
- 恢复实例使用独立端口启动并通过 live／ready；Money Flow 未参与，也没有跨系统事实；
- `make check`、Contract Ledger 全量 `-race`、完整真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过。

规格：[可校验全量备份与空环境恢复](./specs/contract-ledger/verifiable-full-backup-restore.md)  
决策：[ADR-0013：全量备份是同一快照下的数据库与 Evidence 集合](./adr/0013-backup-is-a-versioned-database-and-evidence-set.md)  
运行手册：[Contract Ledger 全量备份与恢复](./operations/contract-ledger-backup-restore.md)

### CL-M2-012 独立安装与升级交付

范围：

- server、ops、backup 三个二进制嵌入同一 version、commit、build time，版本查询不连接数据库；
- release bundle 脱离 `go.work` 构建，固定包含三二进制、示例配置、安装升级和备份恢复手册，并生成 SHA-256；
- schema 状态显式区分 empty、pending、current 和 incompatible，未知 migration、缺口和非托管 relation 不自动修复；
- `ops check`、`ops migrate` 和 `check --require-current` 把预检、修改与启动分离；全部 pending migration 在同一事务和 advisory lock 下执行；
- server 的 `verify` 模式只接受 PostgreSQL 18、current schema 和可写 Evidence，正式启动不隐式迁移；
- 新装、升级、失败前滚和从升级前全量备份恢复到新目标的边界写入独立运行手册；
- 不引入 Kubernetes、systemd 专用包、共享 IAM、Money Flow 或客户专用基础设施。

验证证据：

- schema 单元／集成测试覆盖空库、合法前缀、current、unknown、gap、非托管 relation、并发迁移和故意失败后的整体回滚；
- 历史 `000015` 数据库经显式 `000016` 升级后既有事实保持有效，migration 可重放；
- `v0.1.0-dev.2` darwin/arm64 bundle 内容与 SHA-256 通过，三个二进制报告相同 build identity，同名输出和非法版本被拒绝；
- 实际 bundle 在全新数据库完成 empty 预检、16 个 migration、current 门禁和 `verify` 启动；empty schema 下 verify 在监听前退出；
- 浏览器等价 HTTP 流程完成管理员登录、合同创建、Evidence 上传下载，下载内容 SHA-256 与 bundle 手册原文一致；
- 移除 bootstrap 变量重启后原管理员仍可登录、原合同仍可见、用户数量保持为 1；
- 全仓 `make check`、两个应用全量竞态测试、Contract Ledger 完整真实 PostgreSQL 集成测试，以及两个应用 `GOWORK=off` 独立测试／构建全部通过；
- Money Flow 应用保持停止，安装和运行没有跨系统数据库、文件、调用或交付依赖。

规格：[独立安装与升级交付](./specs/contract-ledger/independent-install-upgrade-delivery.md)  
决策：[ADR-0014：版本化交付包使用显式 Schema 门禁](./adr/0014-release-bundle-uses-explicit-schema-gates.md)  
运行手册：[Contract Ledger 独立安装与升级](./operations/contract-ledger-install-upgrade.md)

### MF-M3-001 首位管理员与本地登录会话

范围：

- 空用户库通过显式 bootstrap 配置原子建立唯一首位 active administrator，已有用户时不覆盖；
- 用户名标准化、密码 12 至 72 字节并只保存 BCrypt 摘要；
- 32 字节随机 session token 只交给客户端，数据库只保存 SHA-256、过期时间和 session generation；
- 未知、错误、锁定和停用用户使用同一认证错误，连续 5 次失败登录锁定 15 分钟；
- 首页改为受保护页面，登录前安全跳转本地 `/login`，登录后 Principal 只来自服务端会话；
- 登录与注销表单使用双提交 CSRF，Cookie 设置 HttpOnly 与明确 SameSite，HTTPS 可强制 Secure；
- bootstrap 用户和 `local_user_bootstrapped` 审计原子提交，审计禁止 UPDATE／DELETE；
- 本轮不创建其他用户、不绑定 Party，也不实现任何义务、资金或核销事实。

验证证据：

- 单元测试覆盖配置组合、用户名／显示名／密码边界、bootstrap 重放、密码哈希、session 哈希、解析／注销、失败锁定和通用认证错误；
- HTTP 测试覆盖公开 health、受保护首页、开放重定向防护、CSRF、Cookie 属性、可信 Principal、无效 session 清理和服务端注销；
- migration 从仅有 `000001` 的历史库升级到 `000002`，重复执行保持两条版本记录，并核对用户／会话／审计表、索引、约束和不可变触发器；
- 真实 PostgreSQL 故意令 bootstrap 审计失败，用户与审计均为零；两个并发 bootstrap 恰好一个创建用户，最终用户和审计各一条；
- 真实 PostgreSQL 登录只保存 32 字节 token hash，解析、注销和 5 次失败锁定均通过；临时测试数据库全部清理；
- 真实 HTTP 联调完成匿名跳转、登录、受保护首页、注销和旧 Cookie 拒绝；开发库移除 bootstrap 配置重启后原账号仍可登录且用户数保持 1；
- Contract Ledger 进程停止时 Money Flow 仍通过 ready、登录和首页验收，随后 Contract 以 `verify/current` 恢复；两个应用 Cookie 名称、数据库和会话完全独立；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过。

规格：[首位管理员与本地登录会话](./specs/money-flow-ledger/local-administrator-session.md)  
决策：[ADR-0015：Money Flow 在财务事实前建立独立身份边界](./adr/0015-money-flow-establishes-identity-before-financial-facts.md)

### MF-M3-002 轻量主体登记与列表

范围：

- Money Flow 自己拥有 organization／individual 主体当前资料，不复用或同步 Contract Party；
- 法定名称／姓名、可选登记标识和 revision 1 使用领域与数据库双层校验，空登记标识持久化为 NULL；
- UUID Command ID 与标准化负载 SHA-256 提供幂等；同命令改变事实或可信 Actor 时显式冲突；
- PostgreSQL advisory transaction lock 使相同命令并发只形成一个 Party；
- Party、创建命令和 `party_created` 可信 Actor 审计原子提交，命令与审计禁止更新或删除；
- GET／POST `/parties` 均要求有效本地 administrator 会话，POST 同时要求 CSRF，请求 Actor 字段不能覆盖 Principal；
- 本轮没有主体编辑、停用、删除、合并、导入、义务快照或 Contract 连接。

验证证据：

- 领域单元测试覆盖 organization／individual、输入标准化、全部长度和类型边界、负载哈希、可信 Actor、重放与错误传播；
- HTTP 测试覆盖管理员列表和登记、可信会话 Actor、CSRF、字段保留、`422`、幂等 `409`、非管理员 `403` 和列表故障；
- migration 从仅有 `000001` 的历史库升级到 `000003` 并可重复执行，核对 Party／Command 表、索引、约束和不可变触发器；
- 临时真实 PostgreSQL 验证组织与个人、稳定排序、NULL 登记标识、串行与并发重放、负载冲突、事务末端审计失败整体回滚和命令不可变；
- 隔离空库真实 HTTP 联调完成 bootstrap、登录、登记、等价重放、改变意图冲突、列表和注销，数据库 Actor 与实际登录用户一致；
- 本地开发库升级到 `000003_parties.sql`，真实页面形成一条 Party、一条 Command 和一条审计；伪造 Actor 未生效；
- Contract Ledger 进程停止期间 Money Flow 仍通过 ready、登录、创建、重放、冲突和列表验收，随后两个应用均恢复 ready；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过。

规格：[轻量主体登记与列表](./specs/money-flow-ledger/party-registration.md)

### MF-M3-003 人工应收／应付义务草稿

范围：

- `receivable`／`payable` 相对于用户明确选择的本方账簿主体解释，并选择不同的往来主体；
- 应收将本方映射为 creditor，应付将本方映射为 debtor，数据库同时约束方向与所有者关系；
- 创建时冻结 creditor／debtor 的 Party ID、revision、类型、名称和登记标识，Party 当前资料变化不改写草稿；
- 标题、严格正数 `numeric(24,6)` 精确金额、三字母币种、必填 LocalDate 到期日和 `manual` 人工来源依据形成 draft version 1；
- 标准化负载 SHA-256、Command ID advisory transaction lock、不可变创建命令和可信 Actor 审计提供串行与并发幂等；
- Obligation、Command 和 `obligation_draft_created` 审计原子提交，缺失 Party 或审计失败时整体回滚；
- administrator 可在 `/obligations` 创建和查看列表，并从 `/obligations/{id}` 核对完整快照；CSRF 与服务端授权生效；
- 本轮不编辑、取消或确认草稿，不形成 open 义务、余额、资金、核销或 Contract 引用。

验证证据：

- 领域单元测试覆盖方向映射、全部字段边界、`000100.500000 → 100.5`、最小金额、负载哈希、可信 Actor、列表与详情错误传播；
- HTTP 测试覆盖两个主体前置、创建／列表／详情、字段保留、CSRF、可信 Actor、`422`、幂等 `409`、非管理员 `403`、404 和数据库故障；
- migration 明确从 `000003` 升至 `000004` 并可重复执行，核对 Obligation／Command 表、索引、四个外键、约束和不可变触发器；
- 临时真实 PostgreSQL 验证应收／应付映射、Party 快照、精确金额、稳定列表、快照不受 Party 当前资料更新影响、串行与并发重放、负载／Actor 冲突、缺失 Party 回滚、审计失败整体回滚和命令不可变；
- 隔离空库真实 HTTP 联调完成 bootstrap、两个 Party、人工应收草稿、等价重放、改变意图冲突、列表、详情和注销；数据库 Actor 与实际登录用户一致；
- 本地开发库升级到 `000004_obligation_drafts.sql`，Contract 停机时真实页面建立一条 `2500.75 CNY` 应收草稿；数据库中 Obligation、Command、Audit 各一条，伪造 Actor 未生效；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成测试和两个应用 `GOWORK=off` 独立测试／构建通过；两个应用最终均恢复 ready。

规格：[人工应收／应付义务草稿](./specs/money-flow-ledger/manual-obligation-draft.md)

### MF-M3-004 义务确认与 revision 1

范围：

- administrator 在义务详情显式提交独立 Command ID 和 Expected Draft Version，只有 `draft` 可以确认；
- 确认直接复制草稿中已经冻结的方向、标题、账簿所有者、creditor／debtor Party 快照、精确金额、币种、到期日与人工来源，不重新读取 Party 当前资料；
- 确认形成不可变 Obligation revision 1，并把根状态原子推进为 `open`、`current_revision = 1`；正式投影从 current revision 读取；
- Command ID advisory transaction lock 先解决重放，再通过 Obligation 行锁串行化不同命令；Expected Draft Version 过期和非 draft 状态明确冲突；
- revision、确认命令、`obligation_confirmed` 审计和状态推进同事务提交；revision、命令和审计禁止更新或删除；
- 详情页只在 draft 显示确认表单，open 显示 revision 历史和确认 Actor／时间；登录、administrator 与 CSRF 边界继续生效；
- 本轮不编辑或取消草稿，不登记实际资金，不计算余额、账龄或核销，不访问 Contract Ledger。

验证证据：

- 领域单元测试覆盖输入标准化、Expected Draft Version、确认负载哈希、可信 Actor、审计构造、重放与类型化错误传播；
- migration 明确从 `000004` 升至 `000005` 并可重复执行，升级前 draft 保持 draft／无 current revision，新增 revision／确认命令表、复合外键、状态约束与不可变触发器；
- 真实 PostgreSQL 验证 Party 当前资料变化不改写 revision 1、正式投影来自 revision、同命令并发只形成一次、不同命令并发恰好一个成功，以及缺失义务、版本冲突、状态冲突、审计失败整体回滚和不可变保护；
- HTTP 单元测试覆盖详情确认入口、可信会话 Actor、CSRF、administrator、成功跳转、404、`422`、版本／状态／幂等 `409`，open 详情不再显示确认表单；
- 隔离空库 HTTP 联调完成 bootstrap、两个 Party、草稿创建、确认、原命令重放、open 列表和 revision 历史；数据库 revision、Command、Audit 各一条，Actor 与实际登录用户一致；
- 本地开发库在 Contract 停机时升级到 `000005_obligation_confirmation.sql`，把既有 `2500.75 CNY` 应收草稿确认成 `open / revision 1`；原命令重放后 revision、Command、Audit 仍各一条，伪造 Actor 未生效；
- Money Flow 与 Contract 全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成均通过；临时数据库归零，两个应用最终均恢复 ready。

规格：[义务确认与 revision 1](./specs/money-flow-ledger/obligation-confirmation.md)

### MF-M3-005 人工实际收款／付款登记

范围：

- administrator 相对于明确的本方账簿主体登记 `receipt` 或 `payment`；收款把本方映射为 payee，付款把本方映射为 payer；
- 登记时冻结 payer／payee 的 Party ID、revision、类型、名称和登记标识，Party 当前资料变化不改写资金事实；
- 正金额不经浮点数，以 `numeric(24,6)` 保存；币种、LocalDate 发生日、账户标签、`manual` 来源依据和可选外部流水号一并形成不可变 `recorded` 事实；
- Command ID advisory transaction lock 保证创建幂等；非空外部流水号在“本方主体 + 不区分大小写的账户标签 + 来源类型”范围内不区分大小写唯一，并由独立 advisory lock 和部分唯一索引保护并发；
- Cash Transaction、创建命令和 `cash_transaction_recorded` 审计同事务提交，Actor 只来自 administrator 会话；资金与命令禁止更新或删除；
- 列表、创建和详情页面继续执行本地登录、administrator 与 CSRF 边界；
- 本轮不选择 Obligation、不形成 Allocation、不计算资金或义务余额，不实现编辑、删除、退款、冲销、银行核验或 Contract 连接。

验证证据：

- 领域单元测试覆盖方向映射、输入标准化、十进制精度、Party 快照、可信 Actor、负载哈希、外部流水范围、重放和类型化错误；
- migration 明确从 `000005` 升至 `000006` 并可重复执行，新增资金与创建命令表、方向／金额／来源约束、外键、索引和不可变触发器，既有 Party 与义务事实保持不变；
- 真实 PostgreSQL 验证收款／付款映射、快照稳定、同命令串行及并发重放、负载／Actor 冲突、同范围外部流水串行及并发冲突、跨账户／本方范围复用、缺失 Party 和审计失败整体回滚，以及资金与命令不可变；
- HTTP 单元与隔离空库联调覆盖登录、administrator、CSRF、两主体前置条件、登记、重放、幂等冲突、外部流水冲突、列表、详情、404、数据库故障与可信会话 Actor；
- 本地开发库在 Contract 停机时升级到 `000006_manual_cash_transactions.sql`，真实登记 `2500.75 CNY` 收款；原命令重放、改变负载和大小写等价外部流水验证后，数据库只有一条资金、一条命令和一条审计，伪造 Actor 未生效；
- `make check`、Money Flow 全量单元与真实 PostgreSQL 集成 `-race`、Contract 全量 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过；临时测试数据库归零，两个应用最终恢复 ready。

规格：[人工实际收款／付款登记](./specs/money-flow-ledger/manual-cash-transaction.md)

### MF-M3-006 单笔资金义务分配与余额不变量

范围：

- administrator 每次把一笔 `recorded` Cash Transaction 部分或全部分配给一条 `open` Obligation current revision，并显式消费 Expected Obligation Revision；
- `receivable` 只接受同一 payer／payee Party 对的 `receipt`，`payable` 只接受同一 Party 对的 `payment`；本方主体和币种也必须一致；
- 分配金额使用精确 `numeric(24,6)`，同时不得超过资金未分配金额和义务剩余金额；等于任一侧剩余额时允许；
- Cash allocated／unallocated 与 Obligation allocated／outstanding 均从不可变 Allocation 求和，不修改资金、义务 revision 或根表金额，余额归零不自动关闭义务；
- Command ID 先解决重放，再固定锁资金和义务；不同命令争用任一侧时串行检查余额，数据库插入触发器再次保护兼容性、current revision 和不超额；
- Allocation、创建命令和 `allocation_applied` 审计同事务提交且不可更新或删除，Actor 和生效时间只来自服务端可信边界；
- 列表、创建、详情及两端余额页面执行本地 administrator、CSRF 和数据库错误边界；
- 本轮每个命令只处理一对事实，不实现批量写、第三方代收付、跨币种、反向核销、义务调整、资金冲销或 Contract 连接。

验证证据：

- 领域单元测试覆盖 UUID、Expected Revision、精确金额、人工依据、可信 Actor、负载哈希、审计构造、重放和全部类型化错误；
- migration 明确从 `000006` 升至 `000007` 并可重复执行，新增 Allocation／命令表、三类查询索引、复合外键、插入校验函数和不可变触发器，既有 Party、Obligation 与 Cash Transaction 保持不变；
- 真实 PostgreSQL 验证部分／全额分配、两端精确余额、重放、负载／Actor 冲突、状态／revision／方向／主体／币种错配、两侧超额、缺失事实、审计失败整体回滚、直接 SQL 保护和不可变约束；
- 并发测试分别覆盖相同 Command、同一资金争用不同义务、不同资金争用同一义务和同一对事实争用，两侧成功总额均不超限；
- HTTP 单元与隔离空库联调覆盖候选事实、可信会话 Actor、CSRF、登记、重放、幂等冲突、超额冲突、列表、详情、两端余额、404、数据库故障和 administrator 边界；
- 本地开发库在 Contract 停机时升级到 `000007_single_allocations.sql`，真实把 `1000.25 CNY` 分配给既有应收；重放、改变负载和超额尝试后，数据库只有一条 Allocation、一条命令和一条审计，资金未分配与义务剩余均为 `1500.5 CNY`，伪造 Actor、币种、余额和时间均未生效；
- `make check`、Contract Ledger 与 Money Flow 全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过；Markdown 相对链接有效、临时测试数据库归零，两个应用最终均恢复 live／ready。

规格：[单笔资金义务分配与余额不变量](./specs/money-flow-ledger/single-allocation.md)

### MF-M3-007 多条单笔 Allocation 组合闭环

范围：

- 不新增批量写协议；三个独立单笔命令组成一笔资金对多条义务、多笔资金对一条义务的核销关系；
- Allocation 服务按 Cash Transaction ID 或 Obligation ID 返回各自完整历史，无效父 ID 在进入 repository 前拒绝；
- PostgreSQL 复用 `000007` 的 Allocation 表和父 ID／时间索引，按 `(applied_at DESC, allocation_id DESC)` 确定排序，不新增 migration、余额列或汇总表；
- 实际资金详情显示目标义务与 Allocation 历史，义务详情显示来源资金与 Allocation 历史，逐条包含金额、revision、人工依据、可信 Actor 和生效时间；
- 父事实不存在继续返回 404，历史读取失败返回 500，不能伪装成空历史；
- 余额归零后资金仍为 `recorded`、义务仍为 `open`，本轮不实现批量、自动分配、反向核销、调整、冲销或 Contract 连接。

验证证据：

- 领域单元测试覆盖两类父 ID 标准化、无效 ID 不调用 repository、结果透传和数据库错误包装；
- 真实 PostgreSQL 以资金 A `120`、资金 B `80`、义务 X `100`、义务 Y `80` 完成 A→X `40`、A→Y `80`、B→X `60` 三条 Allocation；
- 组合完成后 A 为 `120/0`、B 为 `60/20`、X 为 `100/0`、Y 为 `80/0`，资金侧、义务侧和 Allocation 总额均精确为 `180`，两条义务仍保持 `open`；
- repository 测试证明资金 A／B 与义务 X／Y 的四组历史彼此隔离、确定倒序且全局列表一致；既有幂等、并发双侧不超额、直接 SQL 保护和不可变测试继续通过；
- HTTP 单元测试覆盖两端详情历史、跨端链接、空历史、父事实 404、父查询失败和历史查询失败 500；非 administrator 在用例调用前拒绝；
- 隔离空库真实 HTTP 联调登录后执行三次独立 Allocation，核对四个详情页面不泄漏无关记录、三份 Allocation 详情、全局列表、数据库计数和全部余额守恒；
- Contract Ledger 进程停止且端口不可达时，上述真实 PostgreSQL HTTP 组合流程与 Money Flow ready 继续通过；随后两个应用恢复 live／ready；
- `make check`、Contract Ledger 与 Money Flow 全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过。

规格：[多条单笔 Allocation 组合闭环](./specs/money-flow-ledger/multi-allocation-composition.md)

### MF-M3-008 人工内部账户转账

范围：

- 以一条独立、不可变的 Internal Transfer 表达同一本方主体名下账户间移动，不生成经营 payment／receipt，不虚构往来 Party；
- 冻结本方 Party revision 快照，保存精确金额、同币种、LocalDate、不同的来源／目标人工账户标签、人工依据和可选外部转账参考；
- Command ID 提供串行／并发幂等；非空外部参考在本方 Party、来源账户、目标账户和来源类型范围内不区分大小写唯一；
- Transfer、创建命令和可信 Actor 审计原子提交并禁止更新／删除；管理员页面提供创建、列表和详情；
- 经营收付款汇总、未分配资金和 Allocation 结构上只读取 Cash Transaction；Internal Transfer 没有 Allocation 引用路径；
- 不建设 Account 主数据、账户余额、银行接口、跨币种、反向转账、会计凭证或 Contract 连接器。

验证证据：

- 领域单元测试覆盖 UUID、Party、精确金额、币种、日期、不同账户、来源、大小写规范键、审计分类、幂等透传、读取与错误包装；
- migration 从 `000007` 升至 `000008_internal_transfers.sql` 并可重复执行，既有 Cash Transaction 保持不变；新增两表、三索引、两个不可变触发器和必要外键；
- 真实 PostgreSQL 覆盖 Party 快照稳定、串行／并发重放、负载／Actor 冲突、外部参考并发唯一与方向作用域、缺失 Party、审计失败回滚、同账户数据库约束和不可变保护；
- 经营隔离测试先建立 receipt `125.5 CNY` 与 payment `25.25 CNY`，再登记 Internal Transfer `100.5 CNY`；Cash Transaction 数量仍为 2、经营金额不变、Allocation 为 0，Transfer 单独为 1；
- HTTP 单元与隔离空库联调覆盖 ready、登录、CSRF、可信 Principal Actor、创建、重放、幂等／外部参考冲突、列表、详情、404、数据库失败和 administrator 边界；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，上述真实 PostgreSQL HTTP 流程仍完整通过；随后两个应用恢复 live／ready；
- `make check`、两个应用全量 `-race`、Money Flow 全量真实 PostgreSQL 集成 `-race`，以及两个应用 `GOWORK=off` 独立测试通过；本地 Money Flow schema 为 `000008`，临时测试数据库归零。

规格：[人工内部账户转账](./specs/money-flow-ledger/manual-internal-transfer.md)  
决策：[ADR-0016：内部账户转账是独立的非经营事实](./adr/0016-internal-transfer-is-a-separate-non-operating-fact.md)

### MF-M4-001 Allocation 完整反向核销

范围：

- 以引用原 Allocation 的独立、不可变 Reversal 表达一次完整纠错；原 Allocation 保留，一条 Allocation 最多反向一次；
- Reversal 的 Cash Transaction、Obligation、Obligation revision、精确金额与币种只从原 Allocation 复制，管理员只提交必填原因；
- Allocation 状态由 Reversal 是否存在派生为 `active`／`reversed`，资金和义务的 allocated 统一按 Allocation 减 Reversal 的净额重建；
- 反向释放的资金可分配额和义务剩余额可以形成新的正确 Allocation，原 Allocation 与 Reversal 仍保留在全局、资金、义务和详情历史；
- Reverse 使用独立 Command ID、标准化负载哈希、可信会话 Actor 和原子审计；Apply 与 Reverse 固定使用同一资金／义务锁序；
- 本切片不包含部分反向、Reversal 撤回、Cash 冲销、退款、义务调整、票据、历史时点投影或 Contract 连接器。

验证证据：

- migration 从 `000008` 升至 `000009_allocation_reversals.sql` 并可重放，既有 Allocation 初始保持 active；新增 Reversal／Command 表、查询索引、复合外键、数据库复制校验、净额超额校验和不可变触发器；
- 领域单元测试覆盖原因与 UUID 规范、可信 Actor、幂等哈希、审计分类、同负载重放、改负载冲突、缺失 Allocation 和重复反向；
- 真实 PostgreSQL repository 测试覆盖余额完整释放、重新分配、相同 Command 串行／并发重放、不同 Command 并发互斥、Reverse／Apply 并发锁序、审计失败回滚、数据库复制约束与不可变保护；
- 管理端只在 active Allocation 详情显示完整反向表单；HTTP 单元测试覆盖 administrator、CSRF、可信 Principal Actor、404、422、409、500、错误后保留原因并换新 Command，以及已反向详情不再提供重复入口；
- 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation、Cash Transaction、Allocation、空原因拒绝、Reversal、同命令重放、改负载冲突、第二次反向冲突、四类历史展示、余额恢复和新 Allocation；客户端伪造金额、币种、两端、revision、Actor 与时间均未生效；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，上述真实 PostgreSQL HTTP 流程与 Money Flow ready 仍通过；随后两个应用恢复 live／ready；当前没有跨系统业务流动，因此未制造连接器联调；
- `make check`、两个应用全量单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过；本地 Money Flow schema 为 `000009`，临时测试数据库归零。

规格：[Allocation 完整反向核销](./specs/money-flow-ledger/allocation-reversal.md)  
决策：[ADR-0017：Allocation Reversal 是不可变的完整抵销事实](./adr/0017-allocation-reversal-is-an-immutable-full-offset.md)

### MF-M4-002 Cash Transaction 完整冲销

范围：

- 以引用原 Cash Transaction 的独立、不可变 Reversal 表达原人工资金事实不应继续生效；原登记、外部流水唯一占用、主体快照和审计链永久保留；
- Reversal 的方向、精确金额与币种只从原 Cash Transaction 复制，管理员只提交必填原因；状态由 Reversal 是否存在派生为 `recorded`／`reversed`；
- 只有 Allocation 净额为 0 的资金可以冲销；仍有 active Allocation 时必须先逐条执行 MF-M4-001，系统不隐式批量级联；
- recorded 资金的 current effective amount 为原金额，reversed 资金为 0；unallocated amount 使用有效金额减 Allocation 净额，冲销后两者都为 0；
- Cash Reversal 生效后，Allocation 页面不再列为候选，应用仓储和 PostgreSQL 触发器共同拒绝构造请求与直接 SQL；
- 本切片不包含部分冲销、真实退款、拒付扣回、反向付款、银行状态、义务调整、票据、历史投影或 Contract 连接器。

验证证据：

- migration 从 `000009` 升至 `000010_cash_transaction_reversals.sql` 并可重放，既有 Cash Transaction、Allocation 与 Allocation Reversal 保持不变；新增 Reversal／Command 表、时间线索引、复制校验、净分配前置校验、Allocation 禁用保护和不可变触发器；
- 领域单元测试覆盖 UUID、原因长度、可信 Actor、标准化幂等哈希、审计分类、同负载重放、改负载冲突、事实缺失、已有 Reversal 与非零净分配冲突；
- 真实 PostgreSQL repository 测试覆盖原金额／current effective／allocated／unallocated 投影、外部流水继续唯一、冲销后应用与直接 SQL Allocation 拒绝、审计失败回滚、错误复制约束和 Reversal／Command 不可变；
- 跨模块测试先建立 active Allocation，确认 Cash 冲销拒绝且无半成品；追加 Allocation Reversal 后 Cash 冲销成功，原 Allocation 历史仍可下钻；
- 相同 Command 串行／并发重放、不同 Command 并发冲销、Cash Reverse／Allocation Apply、Cash Reverse／Allocation Reverse 和直接 SQL Cash Reverse／Allocation Insert 均有确定结果；关键并发用例连续运行 20 次；
- 全量集成曾捕获等待 Cash 行锁前的语句快照会令同语句状态子查询过期；实现已拆成“锁 Cash”和“新语句读取 Reversal”，使等待后的冲突稳定映射为 typed 409，数据库触发器继续作为第二层保护；
- HTTP 单元与隔离空库流程覆盖 administrator、CSRF、可信 Actor、空原因、404、422、409、500、错误恢复、冲销前置提示、同命令重放、改负载冲突、第二次冲销、列表／详情和冲销后构造 Allocation 拒绝；客户端伪造方向、金额、币种、Actor 与时间均无效；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，包含 Allocation Reversal → Cash Reversal 的真实 PostgreSQL HTTP 流程和 Money Flow ready 仍通过；随后两个应用恢复 live／ready；当前没有跨系统业务流动，因此未制造连接器联调；
- `make check`、两个应用全量单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过；本地 Money Flow schema 为 `000010`，临时测试数据库归零。

规格：[Cash Transaction 完整冲销](./specs/money-flow-ledger/cash-transaction-reversal.md)  
决策：[ADR-0018：Cash Transaction Reversal 只使净分配为零的原资金事实失效](./adr/0018-cash-transaction-reversal-invalidates-only-unallocated-facts.md)

### MF-M4-003 Obligation 金额调整

范围：

- 对 `open` Obligation 使用独立、不可变的 `increase`／`decrease` Adjustment，从 current revision N 连续形成完整 revision N+1；原始金额、旧 revision 和既有 Allocation 历史不被改写；
- 正调整金额、必填原因和 Expected Current Revision 来自管理员意图，前值、结果值、币种、Actor 与时间由服务端和数据库可信计算或生成；
- 新 revision 只改变 effective amount，方向、标题、主体快照、币种、到期日和来源依据从 source revision 完整复制；列表与详情同时展示 original、current effective、allocated 与 outstanding；
- 减少结果必须大于 0 且不得低于 Allocation／Reversal 净额；等于净额时 outstanding 为 0 但 Obligation 仍为 `open`；
- 既有 Allocation 保留形成时引用的历史 revision，新 Allocation 必须消费最新 revision；Adjustment 不自动创建、反向或迁移 Allocation；
- 本切片不包含减免、坏账、折扣、争议、关闭、退款、票据、账龄、导入导出或 Contract 连接器。

验证证据：

- migration 从 `000010` 升至 `000011_obligation_amount_adjustments.sql` 并可重放；既有 revision 1 金额、Actor、时间、Allocation／Reversal、Cash Reversal 与余额无损保留，revision 字段统一为 effective amount 和 recorded Actor／time；
- 领域与 HTTP 单元测试覆盖精确增减、正数／范围／零结果校验、标准化幂等哈希、可信 Actor、CSRF、administrator、404、422、409、500、重放与错误恢复；
- 真实 PostgreSQL repository 测试覆盖连续 revision、原始／当前金额、历史与新 revision Allocation、余额下限、同命令重放、不同命令竞争、审计失败回滚、不可变事实和直接 SQL 约束；
- Adjustment／Allocation Apply 和 Adjustment／Allocation Reverse 使用稳定的锁序形成可解释结果；确定性直写竞态先令 Adjustment 阻塞于 Obligation 行锁，再提交 Allocation，数据库仍读取最新净分配额并拒绝错误减额；三类关键竞态连续运行 20 次通过；
- 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation 确认、增加、减少、Allocation、低于净额拒绝、等于净额归零投影、过期 revision 分配拒绝、重放／冲突和完整历史核对；客户端伪造前值、结果值、币种、Actor 与时间均无效；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，上述真实 PostgreSQL HTTP 流程与 Money Flow ready 仍通过；随后两个应用恢复 ready；当前没有跨系统业务流动，因此未制造连接器联调；
- `make check`、两个应用全量单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建均通过；本地 Money Flow schema 为 `000011`，临时测试数据库归零。

规格：[Obligation 金额调整](./specs/money-flow-ledger/obligation-amount-adjustment.md)  
决策：[ADR-0019：Obligation 金额调整追加连续 revision](./adr/0019-obligation-amount-adjustment-appends-revision.md)

### MF-M4-004 Obligation 减免

范围：

- 对 `open` Obligation 追加独立、不可变 Remission；它冻结 current revision、记录时有效金额、币种、正减免金额、原因、人工来源依据、可信 Actor 与时间，不推进或改写 Obligation revision；
- 列表与详情统一展示 original、current effective、net allocated、remitted 与 outstanding，余额按 `effective - net allocated - remitted` 精确投影；全部或组合归零后 Obligation 仍保持 `open`；
- 同一 revision 允许多笔 Remission，但每笔都在 Obligation 行锁内使用最新余额；新增 Allocation 同时扣除累计减免，金额减少 Adjustment 不得低于净分配额与累计减免额之和；
- Remission 不取得 Cash Transaction 锁，不改变稳定的 Cash → Obligation 锁序；既有 Allocation、旧 revision、原始金额和当前有效金额不被改写；
- 本切片不包含坏账、资产减值、争议、审批、会计凭证、Remission Reversal、受控关闭、退款、票据、账龄、导入导出或 Contract 连接器。

验证证据：

- migration 从 `000011` 升至 `000012_obligation_remissions.sql` 并可重放；历史 Adjustment、revision、Allocation／Reversal、Cash Reversal 和余额保持不变，历史义务的累计减免为 0；本地 Money Flow 已迁移到 schema `000012`；
- 领域与 HTTP 单元测试覆盖精确金额、必填原因与人工依据、标准化幂等哈希、可信 Actor、administrator、CSRF、404、422、409、500、重放、错误恢复和减免历史展示；客户端伪造 effective amount、币种、Actor 与时间均无效；
- 真实 PostgreSQL repository 测试覆盖同 revision 多笔减免、部分与全部减免、调额扩容、Allocation 上限、调额下限、命令重放／冲突、不同命令竞争、审计失败回滚、不可变事实和直接 SQL 约束；
- Remission／Allocation Apply、Remission／Amount Adjustment 与 Remission／Allocation Reverse 在 Obligation 行锁上形成确定顺序；确定性直写竞态证明等待 Allocation 提交后的 Remission 会读取最新净额并拒绝超额；五类关键并发测试连续运行 20 次通过；
- 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation 确认与调额、既有 Allocation、两笔同 revision Remission、超额拒绝、减免后继续 Allocation、组合归零、归零后继续减免拒绝、低于已分配与已减免合计的调额拒绝，以及完整页面和数据库投影核对；
- 全量真实 PostgreSQL 集成发现旧义务确认测试使用固定当日时刻，运行时间越过该时刻后会违反正确的时间单调约束；测试已改为相对草稿创建时间，单独复现与全集成均通过；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，上述 Money Flow 真实 PostgreSQL HTTP 流程与 Money Flow ready 仍通过；随后 Contract 以 `verify` 模式恢复，两个应用均为 ready；当前没有跨系统业务流动，因此未制造连接器联调；
- `make check`、两个应用全量单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`、关键并发 20 轮，以及两个应用 `GOWORK=off` 独立测试／构建均通过；临时测试数据库由测试清理。

规格：[Obligation 减免](./specs/money-flow-ledger/obligation-remission.md)  
决策：[ADR-0020：Obligation 减免是独立处置事实](./adr/0020-obligation-remission-is-a-separate-disposition-fact.md)

### MF-M4-005 Obligation 坏账处置边界审计（延期）

审计结论：

- 当前材料只能确认坏账面向 receivable 的可收回性，未必消灭原债权；应付义务被免除属于 Remission；
- 直接从现有 outstanding 扣减会使坏账在运行时等同 Remission，并错误阻止未来对原义务形成 Allocation；
- 只记录事实但不影响投影无法形成可用纵向功能；完整模型至少需要 settlement outstanding、expected collectible、转回、追回款、调额与关闭关系；
- OMS 参考雏形和现有讨论没有提供上述口径的真实样例，当前也没有会计期间、科目、凭证或税务事实支持实现减值模型；
- 因此不创建 migration、领域类型、页面或写入入口，不使用 Adjustment、Remission、负 Allocation 或 Closure 冒充坏账；现有余额公式保持不变；
- 当真实客户案例能说明处置前后权利、业务余额、账面口径、转回和追回款后，再重新进入正式规格与实现。

本次仅改变产品与架构文档，没有运行时代码或 schema 变更。

审计：[Obligation 坏账处置边界审计](./specs/money-flow-ledger/obligation-bad-debt-audit.md)  
决策：[ADR-0021：坏账在可收回性模型明确前延期](./adr/0021-bad-debt-is-deferred-until-recoverability-is-defined.md)

### MF-M4-006 Obligation 受控关闭

范围：

- 只有 settlement outstanding 已由 Allocation／Reversal 净额与 Remission 累计额精确归零的 `open` Obligation 可以显式关闭；余额归零本身不会自动改变状态；
- Closure 是独立、不可变的操作终态事实，不生成 revision，也不修改金额、Allocation、Reversal 或 Remission；它冻结 current revision、balance sequence、effective、net allocated、remitted、币种、结清构成、原因、可信 Actor 与服务端时间；
- 关闭命令同时消费 Expected Current Revision 与由 Allocation、Allocation Reversal、Remission 数量形成的单调 Expected Balance Sequence，旧页面不能在余额事实被替换后静默关闭；
- Closure、Command、`closed` 根状态和 `obligation_closed` 审计原子提交；数据库触发器重算冻结事实，延迟约束禁止只写 Closure，不可变触发器禁止修改或删除；
- `closed` 禁止 Amount Adjustment、Remission、Allocation Apply、Allocation Reversal 与再次关闭，第一版不提供重新打开；关联 Allocation 详情也不再显示反向入口；
- 关闭只表示 Money Flow 不再接受该义务的常规余额写入，不证明法律终止、履约完成、会计结账、坏账处置或 Contract Ledger 已同步。

验证证据：

- migration 从 `000012` 升至 `000013_obligation_closures.sql` 并可重放；专门的 `000012 → 000013` 测试证明既有 open Obligation、revision、Remission 与余额保持不变，本地 Money Flow 已迁移到 schema `000013`；
- 领域与 HTTP 单元测试覆盖输入校验、Expected Revision／Balance Sequence、三种结清构成、可信 Actor、CSRF、administrator、404、422、409、500、重放、错误恢复、closed 只读详情与关联 Allocation 入口收敛；客户端伪造金额、币种、构成、Actor 与时间均无效；
- 真实 PostgreSQL repository 测试覆盖全 Allocation、全 Remission、混合归零、非零拒绝、旧 sequence 冲突、同命令重放、改负载冲突、不同命令并发、关闭与 Allocation Reversal／Amount Adjustment 竞争，以及关闭后所有余额写入拒绝；
- 审计失败会回滚 Closure、Command 和状态；直接 SQL 不能伪造冻结事实、单独提交 Closure、单独推进状态、重开 `closed` 或反向已关闭义务；确定性等待竞态证明直接 Closure 会等待在先的 Allocation Reversal，并依据提交后的最新余额拒绝；
- 隔离空库真实 HTTP 流程完成 bootstrap、Party、Obligation 确认、连续调额、Allocation 与两笔 Remission 组合归零、显式关闭、同命令重放、改负载冲突、closed 后调额／减免／再次关闭／Allocation Reversal 拒绝，以及页面与数据库冻结事实核对；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，上述完整 Money Flow 流程与 `8083/health/ready` 继续通过；随后 Contract 以 `verify` 模式恢复，两个应用均为 ready。当前没有跨系统业务流动，因此未制造连接器联调；
- `make check`、Money Flow 全量单元与真实 PostgreSQL 全集成 `-race`、Contract Ledger 真实 PostgreSQL 全集成 `-race`、关闭关键并发 20 轮，以及两个应用 `GOWORK=off` 独立测试／构建均通过；两个数据库的临时测试库均归零。

规格：[Obligation 受控关闭](./specs/money-flow-ledger/obligation-controlled-closure.md)  
决策：[ADR-0022：Obligation 关闭是显式零余额终态事实](./adr/0022-obligation-closure-is-an-explicit-zero-balance-terminal-fact.md)

### MF-M4-007 Billing Document 与 Billing Link 边界审计

范围与结论：

- OMS 雏形只提供我方向甲方开票、多次开票和附件的候选信号，税率、税点、税额、抵扣、申报和外部开票平台均明确待确认；
- OA 已实现我方／对方开票、金额、日期、往来方和附件，但把票据与支付、奖金、成本合并进可编辑删除的 `FinancialRecord`，金额使用浮点数，并通过单个自引用把发票一对一关联付款；
- 两份材料共同支持“票据独立于义务和现金”以及 issued／received 方向候选，却不能定义票号唯一范围、recorded 的可信含义、作废／红冲／更正链、Billing Link 匹配金额、主体兼容、上限和撤销；
- 因此不创建 migration、领域类型、HTTP 入口或页面，不复制 OA 模型，也不使用 Evidence、Cash Transaction、Allocation 或自由文本冒充票据；
- 当前余额、Allocation 和 Closure 公式保持不变。获得一票据多义务、一义务多票据、红冲／更正和错误 Link 撤销的真实案例后，再形成正式规格进入实现。

本次仅改变产品与架构文档，没有运行时代码或 schema 变更。

审计：[Billing Document 与 Billing Link 边界审计](./specs/money-flow-ledger/billing-document-link-audit.md)  
决策：[ADR-0023：票据在生命周期与义务匹配语义明确前延期](./adr/0023-billing-documents-are-deferred-until-lifecycle-and-link-semantics-are-defined.md)

### MF-M4-008 当前 Obligation 逾期与账龄投影

范围：

- 管理员可选择今天或未来 LocalDate，把查询时的 current open 正余额按 `due_on` 分类为未到期、1–30、31–60、61–90、91+ 天；过去日期拒绝，避免把 current outstanding 冒充历史余额；
- 只读取既有 Obligation current revision、Allocation／Reversal 净额和 Remission 累计额，不增加 migration、业务事实、写命令、审计动作或定时更新字段；
- draft、open 零余额和 closed 排除；汇总按 direction 与 currency 分组，应收／应付不净额、币种不隐式换算；
- 金额在应用层转换为六位小数定标大整数精确累加；页面展示分组笔数／金额与确定排序的明细，并下钻现有 Obligation 详情；
- 未来分类只改变到期分段，不预测期间内的调额、核销、减免或关闭；历史 `as_of` 保持独立切片。

验证证据：

- 领域单元测试覆盖缺省日期、严格格式、过去日期拒绝、0／1／30／31／60／61／90／91 天边界、draft／zero／closed 排除、方向／币种隔离、六位精确小数汇总、确定排序和损坏存储事实失败；
- HTTP 单元测试覆盖 administrator、可信会话、查询参数、汇总／明细／下钻渲染、CSRF logout、`422` 验证错误、`500` 读取失败和非 administrator 在调用领域前拒绝；
- 隔离空库真实 PostgreSQL HTTP 流程先确认义务，再证明 Allocation 后账龄余额从 `100.5` 变为 `40.25`、Allocation Reversal 后恢复 `100.5`，调额与两笔 Remission 组合归零后排除，显式关闭后继续排除；过去日期返回 `422`；
- Money Flow schema 保持 `000013_obligation_closures.sql`，本地新版服务已启动，`/aging` 未登录返回登录跳转且 live／ready 正常；
- Contract Ledger 进程停止且 `127.0.0.1:8081` 不可达时，包含账龄变化的完整 Money Flow HTTP 流程和 `8083/health/ready` 通过；随后 Contract 以 `verify` 模式恢复，两个应用均 ready；
- `make check`、Money Flow 全量单元 `-race`、Money Flow 与 Contract Ledger 真实 PostgreSQL 全集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建均通过；生成的临时二进制已清理，两个数据库的临时测试库均归零。

规格：[当前 Obligation 逾期与账龄投影](./specs/money-flow-ledger/current-obligation-aging-projection.md)  
决策：[ADR-0024：账龄是当前余额的日期分类投影](./adr/0024-aging-is-a-current-balance-classification-projection.md)

### MF-M4-009 历史 Obligation 余额 `as_of`

范围：

- administrator 可从正式 Obligation 详情进入历史余额页面，使用带时区 RFC3339 `as_of`；缺省为服务当前 UTC，未来时刻拒绝；
- 查询只从截至时点的不可变 revision、Allocation／Reversal、Remission 与 Closure 重放 effective、net allocated、remitted、outstanding、balance sequence 和 open／closed，不读取 current 根状态倒推；
- 义务不存在返回 `404`，确认前返回 `422`，无效时刻返回 `422`，历史前缀矛盾返回 `500`；只读查询不写命令或审计；
- 同刻事实作为不可拆分批次；revision 同刻时取连续 revision 最大值，不用随机 UUID 虚构先后；
- `000014_obligation_recorded_time_guards.sql` 先验证既有事实因果顺序及所有事件时点前缀，再为 revision、Adjustment、Allocation、Allocation Reversal、Remission 与 Closure 安装共用 Obligation 行锁的非递减时间门禁；
- 本切片不包含 draft 历史、业务有效日期、会计期间、历史账龄汇总、Cash 历史余额、票据、退款、预测或 Contract 连接器。

验证证据：

- 领域单元测试覆盖 ID 与 RFC3339 校验、时区归一、未来拒绝、缺省当前时刻、义务不存在、确认前与 repository 失败；HTTP 单元测试覆盖页面公式、Closure、查询参数、CSRF logout、`404`／`422`／`500` 和非 administrator 拒绝；
- 真实 PostgreSQL repository 测试按确认 → Allocation → Reversal → Adjustment → Remission → Allocation → Closure 的记录时刻逐点重放，分别得到 `100/0/0/100`、`100/40/0/60`、`100/0/0/100`、`120/0/0/120`、`120/0/20/100`、`120/100/20/0` 和 closed 零余额；直接补写倒退时间 Remission 返回 `23514`；“较早时间写入等待较晚事实提交”的确定性竞态连续 20 轮均被门禁拒绝；
- migration 专测从真实 `000013` 事实升级到 `000014`，证明时间戳和事实不被改写、六个门禁与索引／函数存在且可重放；预置因果倒置 Remission 时升级整体失败并保持 version 13；
- 隔离空库完整 HTTP 流程在最终 closed 后仍回看 confirmation、首次 Allocation、其 Reversal 和 Closure 四个精确切片，证明后续 revision、Remission 和关闭不会污染较早快照；确认前页面返回 `422`；
- 本地 Money Flow 已迁移到 `000014_obligation_recorded_time_guards.sql`，六个 temporal trigger 存在；新版服务 live／ready，历史页面未登录返回本地登录跳转；
- Contract Ledger 停止且 `127.0.0.1:8081` 不可达时，带历史切片断言的 Money Flow 完整 HTTP 流程与 `8083/health/ready` 继续通过；随后 Contract 以 `verify` 恢复，两个应用均 ready；当前没有跨系统业务流动，因此未制造连接器；
- `make check`、Money Flow 全量单元 `-race`、Money Flow 与 Contract Ledger 真实 PostgreSQL 全集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建均通过；临时二进制已清理，两个数据库临时测试库均为 0。

规格：[历史 Obligation 余额 recorded-time as_of](./specs/money-flow-ledger/historical-obligation-balance-as-of.md)  
决策：[ADR-0025：历史余额使用单调 recorded-time 重放](./adr/0025-historical-balance-uses-monotonic-recorded-time.md)

### MF-M4-010 真实退款／退回款边界审计

范围与结论：

- OMS 中的“退回”只指审批退回，没有资金退款样例；OA 的 FinancialRecord 没有退款类型、原资金引用、累计上限或义务影响，且支付可编辑／删除，不能提升为稳定模型；
- 确认真正退款必须保留原 Cash Transaction 与第二次反向资金两个不可变事实，并使用正金额、独立流水、来源、Actor 和审计；
- 现有 Cash Reversal 只纠正错误资金，Allocation Reversal 只纠正错误核销，均不得改名复用；
- 关键缺口是原资金已分配后，部分／多次退款如何影响多个 Allocation、Obligation outstanding 与 closed 终态；只增加反向 Cash Transaction 会留下“钱已退、义务仍结清”的半闭环；
- `MF-M4-010` 因此完成边界审计后延期，不新增 migration、代码、页面或汇总，不声明系统已支持退款；
- 已记录重新进入实现所需的收款退款、付款退回、多次／多义务、拒付分类、纠错与并发真实时间线。

验证证据：

- 对 OMS 全仓检索确认“退回”仅出现在审批原型和待确认工作流语义，没有资金退款对象或余额规则；
- 对 OA 财务规格、`FinancialRecord` 模型和服务检索确认只有支付／开票／费用等统一记录，没有 refund 类型、原资金引用或退款—核销关系；
- 对 Money Flow 的 Cash Transaction、Cash Reversal、Allocation／Reversal 规格、迁移和仓储核对，确认现有不变量不能无损表达“原核销正确、后来发生部分退款”；
- 文档链接与全仓质量门禁通过；运行时 schema 保持 `000014`，两个服务和业务数据未因本审计改变。

审计：[真实退款／退回款边界审计](./specs/money-flow-ledger/cash-refund-return-audit.md)  
决策：[ADR-0026：真实退款在义务结清影响明确前延期](./adr/0026-refunds-are-deferred-until-settlement-effects-are-defined.md)

### MF-M4-011 当前经营收支汇总与逐项下钻

范围：

- 新增无持久化的只读汇总，按本方账簿主体 Party ID 与币种隔离当前应收、当前应付、当前有效实际收款、当前有效实际付款、未分配收款和未分配付款六项指标；
- Obligation 指标继续使用 `effective - net allocated - remitted`，Cash 指标继续使用 Cash Reversal 后的 current effective 与 `effective - net allocated`，不建立可写汇总字段；
- 每项只统计正贡献事实并显示数量，正金额可以下钻到全部构成金额的 Obligation 或 Cash Transaction，明细总额由同批事实按六位定标整数精确重算；
- 汇总与下钻分别使用只读 `REPEATABLE READ` 快照；同名 Party 不合并、不同币种不相加，Internal Transfer、草稿、零余额、已冲销资金和延期的退款／坏账／票据全部排除；
- 新增 administrator 页面 `/operating-summary` 与 `/operating-summary/details`，不新增 migration、写命令或审计事实，Money Flow schema 保持 `000014`；
- 本功能不是会计收入、利润、银行余额、历史报表、现金预测或退款后的净现金。

验证证据：

- 领域单元测试覆盖六项公式、同名不同 Party、不同币种、精确小数、确定排序、非法指标／身份／币种和损坏投影拒绝；
- PostgreSQL 集成覆盖两本方主体、两币种、应收／应付、部分核销、Allocation Reversal、Remission、Cash Reversal、零余额 Closure 和 Internal Transfer 排除，并逐一核对六项下钻总额与事实集合；
- HTTP 单元与隔离空库完整流程覆盖汇总、Obligation／Cash 下钻、零值无链接、匿名跳转、非管理员 `403`、参数 `422`、空明细 `404` 和读取失败 `500`；
- Contract Ledger 停止且 `127.0.0.1:8081` 不可达时，带当前经营汇总断言的 Money Flow 完整 HTTP 流程和 `8083/health/ready` 继续通过；当前没有跨系统业务流动，因此未制造连接器；
- `make check`、Money Flow 全量单元与真实 PostgreSQL 全集成 `-race`、Contract Ledger 真实 PostgreSQL 全集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建和 Markdown 相对链接检查通过；两个数据库临时测试库归零。

规格：[当前经营收支汇总与逐项下钻](./specs/money-flow-ledger/current-operating-summary.md)  
决策：[ADR-0027：经营汇总是按本方主体与币种隔离的 current 投影](./adr/0027-operating-summary-is-a-current-owner-currency-projection.md)

### MF-M4-012 本方主体可校验台账导出

范围：

- administrator 以一个本方账簿主体 Party ID 为闭合边界，通过带 CSRF 的 `POST /ledger-exports` 生成版本化 ZIP；导出访问使用登录会话中的可信 Actor 并在归档构造成功后写入不可变审计，审计失败不返回文件；
- repository 在同一个只读 `REPEATABLE READ` 快照中读取本方主体、实际引用的往来 Party，以及 Obligation、revision、Adjustment、Remission、Closure、Cash Transaction、Cash Reversal、Allocation、Allocation Reversal、Internal Transfer 和范围内 Audit Entry 的原始事实；
- 归档固定包含 `manifest.json` 与十二张 UTF-8 RFC 4180 CSV；manifest 保存格式版本、owner、数据库 schema version，以及每张 CSV 的固定列、数据行数、字节数和 SHA-256；
- 金额保持精确十进制文本，空值保持空字段，原始文本不做会改变事实的电子表格转义；使用表格软件时必须把所有列按文本导入；
- 导出排除其他本方主体、身份凭证、会话、幂等 payload hash、可重建汇总、Contract Ledger 数据和未被本账簿引用的 Party；遇到跨本方主体 Allocation 等损坏事实时整体失败；
- 本功能是可离线核对的业务数据移交，不是签名、法律真实性证明、全量数据库备份、恢复包、批量导入或会计报表；无 migration，Money Flow schema 保持 `000014`。

验证证据：

- 领域单元测试覆盖固定 13 个根文件、逐文件行数／字节数／SHA-256、空表表头、公式前缀与引号／换行 CSV 往返、六位金额、审计、审计失败不泄露文件、输入错误、读取失败与损坏快照拒绝；
- PostgreSQL 集成覆盖全部十二类表、其他 owner 隔离、上一次导出审计进入下一次快照、业务表行数不变，以及并发新增 Cash Transaction 在重复读快照中不产生撕裂、下一次导出才可见；
- 测试数据库在临时禁用 Allocation 保护后注入跨 owner 损坏关系，导出明确失败且不返回字节、不写成功审计；
- HTTP 单元覆盖页面、可信 Actor、ZIP 响应头、匿名跳转、viewer `403`、CSRF、`404`／`422`／`500`；隔离空库完整 HTTP 流程下载真实 ZIP，重新计算全部文件哈希并核对义务、资金、核销与反向事实，当前导出审计只在数据库中成功追加，不伪造进已完成的快照；
- Contract Ledger 停止且 `127.0.0.1:8081` 不可达时，包含归档下载与逐文件校验的 Money Flow 完整 HTTP 流程和 `8083/health/ready` 继续通过；当前没有跨系统业务流动，因此未制造连接器；
- `make check`、两个应用全量真实 PostgreSQL 集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建、Markdown 相对链接与本地运行状态均通过；两个数据库临时测试库归零。

规格：[本方主体可校验台账导出](./specs/money-flow-ledger/verifiable-owner-ledger-export.md)  
决策：[ADR-0028：Money Flow 导出是本方主体范围的可校验 CSV 归档](./adr/0028-money-flow-export-is-an-owner-scoped-verifiable-csv-archive.md)

### MF-M4-013 可验证全量备份与空环境恢复

范围：

- 新增独立 `cmd/backup create|restore` CLI；备份格式 `omsbizfoundry.money-flow-ledger/full-backup-v1` 固定包含严格 `manifest.json` 与 PostgreSQL custom-format `database.dump`，最终文件权限 `0600` 且不覆盖已有路径；
- 创建工具在只读 `REPEATABLE READ` 事务中持有 Money Flow migration shared advisory lock 并导出 snapshot，使 `pg_dump`、schema version 和除 `local_sessions` 外所有 public base table 的逐表计数使用同一 MVCC 时点；
- dump 保留本地用户密码摘要、锁定状态、全部账簿、幂等命令、payload hash、审计、函数、触发器和约束，明确排除 session table data；
- manifest 保存来源数据库、PostgreSQL server version、schema、完整表名／行数、dump 字节数和 SHA-256；ZIP 拒绝未知字段、额外／重复／不安全路径、压缩、加密或非普通文件 entry；
- 恢复只允许同 PostgreSQL major 的真正空数据库，拒绝覆盖、合并和 `--force`；`pg_restore` 使用 `--single-transaction --exit-on-error --no-owner --no-privileges`；
- 恢复后复核 schema、完整非 session 表集合、逐表行数和 session 为 0；已提交后的最终核验失败要求隔离目标，工具不擅自清库；
- Money Flow 当前没有外部业务文件目录；v1 只覆盖本系统 PostgreSQL，不包含环境密码、部署配置、Contract 数据或跨系统一致 snapshot。

验证证据：

- 单元测试覆盖严格双文件归档、权限、dump 摘要、内容变化拒绝、路径穿越、未知／压缩／重复 entry、严格 JSON、表计数排序／范围、session 排除标志、已存在输出不覆盖和 PostgreSQL major 不匹配；
- 真实 PostgreSQL 完整恢复测试覆盖本地用户与失败登录状态、Party、Obligation confirmation／Adjustment／Remission／Closure、两笔 Cash、Allocation／Reversal、Cash Reversal、Internal Transfer、全部命令表和审计；
- 恢复后旧 session token 不可用、原账号密码可重新登录、既有 Party Command ID 幂等重放到同一事实、完整历史可读取，并可继续创建新 Party；migration 重放不改变恢复事实；
- 客户端连接错库、已有 migration 的非空目标和已经恢复过的目标均在写入前拒绝；无效 custom dump 的 `pg_restore` 失败后目标用户 relation 为 0；
- 确定性并发测试在 snapshot 建立后提交第二个 Party，证明 manifest 计数和实际 dump 都只包含 snapshot 前 Party，而源库保留两条；
- 当前本地 `money_flow` 在线生成真实备份后成功恢复到专用新库，恢复 schema `000014`、session 0，恢复实例在 `18083` 通过 live／ready；演练数据库已删除，临时归档移入系统废纸篓；
- `make check`、Money Flow 与 Contract Ledger 全量 PostgreSQL 集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建、Contract 停止时备份恢复聚焦集成和文档链接检查通过；测试数据库归零。

规格：[Money Flow 可验证全量备份与空环境恢复](./specs/money-flow-ledger/full-database-backup-restore.md)  
决策：[ADR-0029：Money Flow 全量备份是版本化数据库快照](./adr/0029-money-flow-backup-is-a-versioned-database-snapshot.md)  
运行手册：[Money Flow 全量备份与恢复](./operations/money-flow-ledger-backup-restore.md)

### MF-M4-014 独立安装与升级交付

范围：

- 新增 Money Flow 独立 build identity，server、ops、backup 三个二进制都支持不连接数据库的 `version`／`--version`；
- 新增 `cmd/ops check|migrate`，统一报告 PostgreSQL 18、嵌入 migration 清单和 `empty`／`pending`／`current`／`incompatible` schema 状态；
- migration 只接受空库或严格前缀，在同一个事务和独占 advisory transaction lock 中执行全部 pending 文件；该锁与 MF-M4-013 snapshot shared lock 使用同一 key；
- server 新增 `MIGRATION_MODE=auto|verify`；正式 `verify` 在监听前只读验证 current schema，不修改数据库；
- 新增不覆盖的版本化 release bundle 与外部 SHA-256，固定包含三个二进制、示例配置、安装升级和备份恢复手册；
- 升级回退不使用 down migration 或覆盖原库，失败时优先前滚；必须回退时把升级前全量备份恢复到新的空数据库。

验证证据：

- schema 单元与真实 PostgreSQL 集成覆盖 empty、pending、current、未知 migration、版本缺口、非托管 relation、两个并发 migrate 串行化；
- 从合法 `000012` 同时升级 `000013`／`000014` 时故意制造 `000014` 函数冲突，确认 `000013` 的表和 migration 记录也未提交；移除冲突后可重试到 current；
- 实际构建 `v0.1.0-test.1` darwin/arm64 bundle，SHA-256 校验通过，归档只含固定七项，三个二进制的 version、commit、build time 和 target 完全一致，同名输出拒绝覆盖；
- 从 bundle 对全新数据库执行 `check → require-current 拒绝 → migrate → require-current`，以 `verify` 启动，完成首次管理员、登录、主体写入、live／ready；移除 bootstrap 凭据后重启，已有账号仍可登录且未被覆盖；
- 实际 bundle 的 `verify` 模式对 empty、pending、unknown、gap 均以非零状态在监听前退出；源码集成同时证明 `auto` 不修改 incompatible 数据库；
- 用现有真实账簿构造合法 `000013` 历史库，新 bundle 升级到 `000014` 前后本地用户、Party、Obligation、Cash Transaction、Allocation 和 Audit 计数保持 `1|2|1|1|1|7`；
- 从 `000013` 在线生成升级前全量备份，升级后把该备份恢复到另一个新空数据库；恢复目标保持 pending、六类事实计数一致且 session 为 0，证明回退不覆盖升级目标；
- Money Flow 全量真实 PostgreSQL `-race` 回归曾发现旧多对多夹具的固定时间早于确认事实；夹具改为从数据库最大 `recorded_at` 继续后，聚焦测试和全量回归通过，未放松运行时因果门禁；
- Contract Ledger 全量真实 PostgreSQL `-race` 回归通过；两个系统没有共享数据库、文件、调用、发布包或升级步骤。

规格：[Money Flow 独立安装与升级交付](./specs/money-flow-ledger/independent-install-upgrade-delivery.md)  
决策：[ADR-0030：Money Flow 版本化交付包使用显式 Schema 门禁](./adr/0030-money-flow-release-bundle-uses-explicit-schema-gates.md)  
运行手册：[Money Flow 独立安装与升级](./operations/money-flow-ledger-install-upgrade.md)

### MF-M4-015 当前预计现金排期

范围：

- 新增独立于 Obligation `due_on` 的不可变 Cash Expectation；管理员通过连续 `set`／`clear` sequence 设置、改期和清除预计日期，旧判断、原因、可信 Actor 与记录时间永久保留；
- Expectation command 消费 Expected Current Revision，并与 Adjustment、Allocation、Remission 和 Closure 共用 Obligation 行锁；`set` 只接受 open 正余额，命令幂等、负载冲突和直接 SQL 均有应用层与数据库双重保护；
- 新增 `/cash-forecast` 页面，在一个 `REPEATABLE READ` 快照中按 expected date、本方主体、方向与币种展示已排期 current outstanding，并把没有当前日期的 open 正余额单列为未排期；
- 部分 Allocation 或 Remission 后预计金额自动跟随 current outstanding，余额归零或关闭后从当前投影消失；Expectation 不保存计划金额，也不修改到期日、revision 或历史余额；
- owner 台账导出升级为 `owner-ledger-export-v2`，新增 `obligation_cash_expectations.csv`，固定包含 manifest 与十三张 CSV；v1 不含该事实，避免静默改变既有格式。

验证证据：

- 领域测试覆盖规范化、动作／日期配对、原因和 revision 校验、确定性 payload hash、六位定标精确汇总、隔离维度、未排期与历史读取；
- PostgreSQL 集成覆盖设置、幂等重放、部分核销后预计金额变化、清除、连续历史、不可变保护、draft／零余额拒绝、直接 SQL sequence gap 拒绝和并发 sequence 连续性；
- migration 从真实 `000014` 升级到 `000015_obligation_cash_expectations.sql`，重放不改写既有事实；新增两张表、索引、外键、状态／余额／sequence／时间门禁和不可更新删除保护；
- HTTP 单元和真实 PostgreSQL 纵向流程覆盖管理员设置、改期／清除、可信 Actor、CSRF、权限、错误状态、未排期／已排期下钻、完整历史，以及导出中的 Expectation CSV；
- `make check` 通过；Money Flow 与 Contract Ledger 的 `GOWORK=off` 全量真实 PostgreSQL 集成 `-race` 均通过，测试数据库归零；
- 实际构建并校验 `v0.1.0-test.2` darwin/arm64 bundle，三个二进制 build identity 一致；本地 schema 先以旧 bundle 生成 v14 升级前备份，再由新 bundle 从 `000014` 原子迁移到 `000015` 并以 `verify` 模式启动；
- Contract Ledger 停止时，Money Flow 完整 HTTP 场景再次通过，`/cash-forecast` 进入本地登录且 live／ready 正常；随后两个本地服务均恢复独立运行。

规格：[当前预计现金排期](./specs/money-flow-ledger/current-expected-cash-schedule.md)  
决策：[ADR-0031：预计现金日期是独立经营事实](./adr/0031-expected-cash-date-is-an-independent-operating-fact.md)  
范围审计：[M4 剩余范围边界审计](./specs/money-flow-ledger/m4-remaining-scope-audit.md)

### MF-M6-001 Money Flow 全新环境 Release 验收

范围：

- 新增只消费实际 `.tar.gz` 与外部 `.sha256` 的 clean-room 验收器，不用源码 server 或既有开发数据库冒充交付产物；
- 严格拒绝路径穿越、软链接、未知／缺失／重复文件及 AppleDouble `._*` entry，核对三个二进制完全一致且可追溯的 build identity；
- 在随机临时数据库执行实际 ops 的 `empty → migrate → current`，再由实际 server 以 `MIGRATION_MODE=verify` 和一次性 bootstrap 凭据启动；
- 通过真实 HTTP／Cookie／CSRF 完成本方与往来 Party、100.50 CNY 应收、revision 1、预计日期、40.25 CNY 实收及 Allocation，核对 current outstanding 和预计金额均为 60.25 CNY；
- 从实际进程下载 `owner-ledger-export-v2`，离线核对固定十四个根文件、十三张 CSV 的逐文件长度／SHA-256 和四类代表性事实；
- 优雅停服并在不提供 bootstrap 三元组的情况下重启，同一管理员重新登录且用户数保持 1，既有事实与预计排期保持不变；
- 新增 `make money-flow-ledger-clean-install-acceptance BUNDLE=... CHECKSUM=...` 可重复入口。

验证证据：

- 第一轮用原 `v0.1.0-test.2` 归档执行时，严格解包发现 macOS `tar` 隐藏写入 `._money-flow-ledger_*` AppleDouble entry 并正确失败；构建脚本增加 `COPYFILE_DISABLE=1`，不以验收器容忍额外文件掩盖问题；
- 第二轮发现临时数据库 helper 返回了原始连接串而不是随机目标 URL；修正 URL 后，ops 不再错误连接开发库；
- 修正文档后重新构建最终验收用 `v0.1.0-test.4` darwin/arm64 bundle；Contract Ledger 进程停止时，从摘要校验到空库安装、业务链路、v2 导出、无 bootstrap 重启和临时库清理全流程通过；
- server 子进程只收到白名单环境，不包含 Contract Ledger、OA、共享 IAM、Evidence 或连接器配置。
- Money Flow 全量真实 PostgreSQL `-race` 回归发现既有 Remission／Allocation Reversal 竞态夹具会因 goroutine 预取不同时间而触发正确的单调时间门禁；改为从数据库最大事实时间之后给两个竞争事实同一合法时刻后，聚焦竞态连续 20 轮和全量回归均通过，未放松运行时约束；Contract Ledger 全量真实 PostgreSQL `-race` 回归同时通过。

规格：[Money Flow 全新环境 Release 验收](./specs/money-flow-ledger/clean-room-release-acceptance.md)  
运行手册：[Money Flow 独立安装与升级](./operations/money-flow-ledger-install-upgrade.md)

### MF-M6-002 Money Flow Clean-room 备份交接与空库恢复

范围：

- 新增只消费实际 release、外部摘要和真实 PostgreSQL 客户端的备份恢复验收，不用源码 backup、内存替身或开发库冒充交付路径；
- 在随机源库由实际 ops／server 建立代表性应收、预计日期、收款与 Allocation，服务在线时由实际 backup 创建 full-backup-v1；
- 离线核对 `0600`、严格双文件、store method、manifest identity、完整表计数、dump 大小与 SHA-256，再以 whole-ZIP 摘要把字节复制到独立交接路径；
- 用实际 backup／`pg_restore` 恢复到随机空目标，由无 bootstrap 的实际 server 验证旧 Cookie 失效、原账号重登、预计日期和 60.25 CNY 余额保持并可继续写入；
- 已迁移的非空目标在写入前拒绝；重写 dump 字节但保留 ZIP 结构的损坏归档因摘要不一致失败，空失败目标不产生用户 relation；
- 新增 `make money-flow-ledger-backup-restore-acceptance BUNDLE=... CHECKSUM=...` 可重复入口，客户端 wrapper 可由绝对路径覆盖。

验证证据：

- 先使用上一切片实际 `v0.1.0-test.4` bundle 验证新验收器，Contract Ledger 停止时成功完成源端业务、在线备份、交接、空库恢复、旧会话拒绝、重登和继续写入；
- 更新归档内运行手册后重新构建 `v0.1.0-test.5` darwin/arm64 bundle，三个二进制 identity 一致；同一 bundle 的 MF-M6-001 clean install 与 MF-M6-002 backup restore 两条验收均通过；
- 恢复目标 schema 为 `000015_obligation_cash_expectations.sql`、session 为 0；恢复前两个 Party，恢复后新增第三个 Party，源库仍为两个；
- 非空目标保持 current schema；损坏 dump 的目标用户 relation 为 0；成功和失败用例的随机测试数据库均由各自清理器回收；
- server、ops 和 backup 子进程只收到白名单环境，不包含 Contract Ledger、OA、共享 IAM、Evidence 或连接器配置；
- `make check`、Money Flow 与 Contract Ledger 全量真实 PostgreSQL `-race`、两个应用 `GOWORK=off` 独立构建、shell 语法、Markdown 本地链接和归档内运行手册检查通过；两个数据库的随机测试库归零。

规格：[Money Flow Clean-room 备份交接与空库恢复](./specs/money-flow-ledger/clean-room-backup-restore-acceptance.md)  
运行手册：[Money Flow 全量备份与恢复](./operations/money-flow-ledger-backup-restore.md)

### CL-M6-003 Contract Clean-room Release、Evidence 与恢复

范围：

- 新增只消费实际 Contract `.tar.gz`、外部摘要与真实 PostgreSQL 客户端的 clean-room 验收，不用源码 server、`httptest`、内存 Blob Store 或开发库冒充交付路径；
- 在随机源库和新 Evidence 根由实际 ops／server 完成迁移、登录、两个 Party、1000 CNY 合同草稿和固定文件上传下载；
- 实际 backup 在线生成包含 custom dump 和唯一 Evidence 普通文件的 full-backup-v1，离线核对严格 entry、manifest、计数及逐文件摘要，再以 whole-ZIP 摘要复制到独立交接路径；
- 恢复到另一随机空库和不存在的 Evidence 根后，以无 bootstrap 的实际 server 验证旧 Cookie 失效、原账号重登、合同保持、Evidence 原字节下载和继续写入；
- 非空数据库在发布 Evidence 前拒绝；已存在 Evidence 根保持 sentinel 且空数据库不变；篡改 Evidence entry 的归档失败且不产生 relation 或目标根；
- 新增 `make contract-ledger-clean-install-backup-restore-acceptance BUNDLE=... CHECKSUM=...` 可重复入口。

验证证据：

- 首次构建 `v0.1.0-test.1` 后，严格解包发现 macOS `tar` 写入 `._contract-ledger_*` AppleDouble 根文件并正确失败；Contract 构建脚本增加 `COPYFILE_DISABLE=1`，没有放宽固定六文件契约；
- 修正后 `v0.1.0-test.2` 在 Money Flow 停止时通过完整成功链路和三类失败门禁；测试夹具同时根据真实页面的金额规范化和缓存头收紧为语义断言；
- 更新归档内运行手册后重新构建最终 `v0.1.0-test.3` darwin/arm64 bundle，三个二进制 build identity 一致，actual bundle 全流程再次通过；
- 恢复目标 schema 为 `000016_empty_database_contract_restore.sql`、session 为 0、恢复标记不存在；恢复前两个 Party，恢复后新增第三个，源库仍为两个；
- 数据库、源普通文件、备份 Evidence entry 和恢复后 HTTP 下载的固定文件字节及 SHA-256 完全一致；server、ops、backup 环境不包含 Money Flow、OA、共享 IAM 或连接器配置；
- `make check`、Contract 与 Money Flow 全量真实 PostgreSQL `-race` 回归通过，新增 delivery acceptance 和 testdatabase 包均包含在 Contract 集成回归中。

规格：[Contract Clean-room Release、Evidence 与恢复](./specs/contract-ledger/clean-room-release-backup-restore-acceptance.md)  
运行手册：[Contract Ledger 全量备份与恢复](./operations/contract-ledger-backup-restore.md)

### M6-004 双 Release 同主机组合安装与故障隔离

范围：

- 新增根目录独立 Go 模块 `acceptance/combined-delivery`，只消费两个实际 release、外部摘要和两个 PostgreSQL 测试入口，不被任一业务应用导入；
- 严格校验两个 bundle 固定内容和各自 build identity，在不同随机数据库、随机端口及 Contract 独占 Evidence 根完成 actual ops migration 和 actual server 同时启动；
- 使用同一个 Cookie Jar 登录两个系统，验证应用专属 session／CSRF 四 Cookie 在同一 hostname、不同端口下共存；
- 两侧分别经真实 HTTP 写入 Party；交替停止 Contract 和 Money Flow 后，另一侧保持 ready、会话和写入，停机侧无 bootstrap 重启后原会话和事实继续有效；
- 直接查询两个数据库没有对方业务表，验证 Money Flow 不创建 Evidence 探针路径；全流程没有跨系统 HTTP、事件、文件或业务数据复制；
- 新增 `make combined-delivery-acceptance CONTRACT_BUNDLE=... CONTRACT_CHECKSUM=... MONEY_BUNDLE=... MONEY_CHECKSUM=...` 可重复入口。

验证证据：

- 首次以 `GOWORK=off` 执行时发现新验收模块的 `go.sum` 只在 workspace 内够用；使用独立模块模式补齐依赖摘要后，`GOWORK=off go test ./...` 通过；
- 实际 Contract `v0.1.0-test.3` 与 Money Flow `v0.1.0-test.5` bundle 同时运行，双登录后四 Cookie 各一份；
- Contract 停机期间 Money Flow 创建第二个 Party；Contract 重启后创建第二个；Money Flow 停机期间 Contract 创建第三个；Money Flow 重启后创建第三个；
- 最终两个随机库 Party 计数均为 3，Contract 库的 Obligation／Cash 表和 Money Flow 库的 Contract／Evidence 表均不存在；
- Contract Evidence 三个目录存在，Money Flow Evidence 探针路径不存在；所有实际 bundle 进程优雅停止，随机数据库由测试清理器回收。
- 收口复验通过根级 `make check`、两个应用全量真实 PostgreSQL `-race`、各自 `GOWORK=off` 三二进制独立构建、验收模块独立测试、shell 语法与 Markdown 本地链接检查；实际 `v0.1.0-test.3 + v0.1.0-test.5` 组合验收再次通过，测试数据库查询无残留，8081／8083 开发服务仍分别 ready。

规格：[双 Release 同主机组合安装与故障隔离](./specs/combined-delivery/same-host-install-fault-isolation.md)

### M6-005 三类参考装配场景边界审计

范围：

- 把第 4 轮的设计单位 OA、资质施工综合体和小型咨询服务公司从架构探针重新按当前实现证据审计；
- 为每项场景诉求标记“已验证能力”“条件可用”“明确缺口”或“待客户确认”，不把设计可组合写成客户已验收；
- 确认设计单位的 Project／生产／奖金／审批、综合体的人员／管理费／付款过程、小型服务公司的 Excel 导入／票据／Money Flow 多用户都不属于当前两个稳定内核；
- 确认三个场景都没有提供修改 Contract 或 Money Flow 核心表、增加行业枚举、建立共享主数据或启动 M5 连接器的事实依据。

验证证据：

- Contract 的合同关系和 Evidence、Money Flow 的 owner 隔离收支与预计日期、两个 actual release 同机故障隔离均引用现有已验证规格；
- 小型咨询场景被识别为最接近当前实现，但人工分别建账、无通用导入、无完整票据、Money Flow 单管理员和无跨系统流动仍是显式前提；
- 本切片没有修改应用代码、migration、运行配置或 release；没有建立客户分支、空连接器、共享 Party 或共享 IAM；
- M6-004 收口时的全仓、两应用 PostgreSQL `-race`、独立构建和实际双 release 验收全部通过；本切片追加后再次执行文档与全仓基线检查。

规格：[三类参考装配场景边界审计](./specs/assembly/three-reference-scenarios-boundary-audit.md)

### M6-006 可交付性退出缺口审计

范围与结果：

- 逐条核对 M6 九类验证内容和五项退出门槛；安装、升级、单应用与组合恢复、文件证据、另一应用不可用均已有直接证据；
- 最小权限与组织隔离、三类装配场景只部分通过：稳定产品边界已经证明，但 Money Flow 多用户、客户 SSO／权限和真实客户接受没有输入；
- 性能与大批量导入保持外部待验：没有容量／硬件／SLO 和真实文件／批次／纠错语义，不设置任意通过线，不开发绕过现有不变量的上传入口；
- 连接器和外部对象存储尚未启用，对应故障注入不适用，不创建空基础设施；
- 识别出唯一可立即补齐的工程缺口：缺少操作者视角的双 Release 同主机运行手册与按手册复演。

规格：[M6 可交付性退出缺口审计](./specs/delivery/m6-exit-gap-audit.md)

### M6-007 双 Release 同主机运行手册与操作复演

范围：

- 新增操作者视角的组合运行手册，明确两个 release 的独立校验、配置、数据库、端口、Contract Evidence、显式 migration、`verify` 启动和 bootstrap 移除；
- 明确日常单侧启停、故障定位、分别在线备份、分别空目标恢复和一次只升级一个产品；
- 明确禁止共库、共 env、共 session、共 Party、联合一致性备份、跨库读写和无事实连接器；
- 停用只先停止进程并保留数据责任，不提供宽泛递归删除命令。

验证证据：

- actual Contract `v0.1.0-test.3` 与 Money Flow `v0.1.0-test.5` 再次通过组合验收，覆盖独立迁移、同时启动、双登录、四 Cookie、交替单侧停机／无 bootstrap 重启和零交叉；
- 同两个 actual bundle 并行重跑 Contract 数据库+Evidence 与 Money Flow 数据库 clean-room 备份恢复验收，三组命令全部成功；
- 复演结束后两个 PostgreSQL 测试入口的随机测试数据库查询为空，原 8081 Contract 与 8083 Money Flow 开发服务仍 ready；
- 手册命令和变量逐项对照两个实际归档内配置示例及安装手册，没有增加不存在的部署工具或联合恢复能力。

规格：[双 Release 同主机运行手册与操作复演](./specs/delivery/combined-same-host-operation-rehearsal.md)  
运行手册：[双 Release 同主机运行手册](./operations/combined-same-host-operation.md)

### M6-008 正式 Release 源代码溯源门禁

范围：

- 新增根级 `ops/resolve-release-provenance`，把本地 development 构建与 official 构建明确分开；
- official 模式必须从仓库根读取真实完整 HEAD，并要求 tracked、staged、untracked 状态全部为空；显式 commit 必须与 HEAD 完全一致，build time 必须显式且为 UTC；
- Contract 和 Money Flow 的 build-release 在创建输出与编译前调用同一门禁；
- 根级 `make check` 纳入隔离临时 Git 仓库测试，不依赖当前工作区恰好处于某种状态。

验证证据：

- 自动测试通过 development 无 Git，以及 official 的无 Git、无 HEAD、非根目录、tracked／staged／untracked、错误 commit、缺时间及干净 HEAD 成功路径；
- 门禁实现前当前仓库因无 HEAD 被准确拒绝；随后建立首个本地 Git 基线 `cdc7bc8349c8af4efda1b33d2d21f0498bffbffd`；
- 基于该干净 HEAD 生成 Contract 与 Money Flow `v0.1.0-baseline.1` darwin/arm64 official-mode 候选包，六个二进制均报告相同完整 commit、构建时间和目标；
- 两个摘要、组合安装／故障隔离、Contract 数据库+Evidence clean-room 恢复及 Money Flow 数据库 clean-room 恢复全部通过；
- 测试数据库无残留，8081／8083 开发服务仍 ready；候选产物只保存在可恢复的废纸篓目录，没有远端发布或推送。

规格：[正式 Release 源代码溯源门禁](./specs/delivery/official-release-provenance-gate.md)

决策：[ADR-0032](./adr/0032-official-release-provenance-requires-clean-git-head.md)

### M5 启动前 OA 参考证据审计

范围与结果：

- 固定到 OA 干净提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`，只读取合同、补充／修订、结算、支付、开票与经营统计的规格和实现，没有读取客户数据库或生产数据；
- 还原出该 OA 的实际关系：合同有效／结算金额直接构成 Project 应收统计基础，开票与甲方付款由经营人员分别人工登记，未收金额在查询时派生；
- 确认 OA 不存在可直接迁移的 Contract → Money Flow 自动联动：没有公开版本化 snapshot、义务建议、幂等消费、失败恢复或人工对账责任；
- 将固定 Project、两层经营链、`float64` 金额、设计／勘察／咨询费、管理费／奖金算法、费种启发式推断和万能 `FinancialRecord` 标记为不得进入稳定内核的客户定制；
- OA 金额计算定向测试通过；`internal/services` 整包测试被其他陈旧测试的编译错误阻断，因此没有把 OA 整体写成已回归通过；
- M5 保持未开始，不创建空连接器、事件基础设施、共享 Party 或跨库读取；审计已经把下一步收敛为一条脱敏真实流程的九项最小事实。

审计：[Contract → Money Flow：OA 参考证据审计](./specs/connectors/contract-money-oa-reference-audit.md)

### M5 启动前 OMS 雏形参考证据审计

范围与结果：

- 固定 OMS HEAD `27c614838fe14c16fb38a08fea8a5263eaaccc78`，确认被审计的 README、决策、架构、规格、交互和原型路径与 HEAD 一致；没有把参考仓库原有 Arckit ledger 改动当作产品证据；
- 还原出已接受的 OMS 意图：应收确认、应付确认和付款授权是三个独立审批时点，合同、发票、结算材料和人员名册只作依据，实际到账／付款再通过独立资金事实核销；
- 按 R1 至 R9 判定：审批触发、主要参与方和版本保留提供部分证据，但来源身份、币种／分期、重复识别、已核销后变化、跨系统失败恢复和真实验收样例仍缺失；
- 识别出 OA 与 OMS 的关键反证：OA 直接用合同／结算金额统计应收，OMS 明确要求独立审批形成义务，因此不能把任何一方的规则固化成通用自动连接器；
- 收紧候选责任边界：Contract Snapshot 只说明依据，Confirmation Decision 说明谁决定形成义务；没有外部工作流时可由 Money Flow 管理员明确接受建议承担决定，但 Contract 不得越权创建正式义务；
- OMS 原型 lint、生产构建和 2 项渲染测试通过；只有 chunk 体积警告，测试产生的未跟踪缓存已移到废纸篓，原有工作区改动未触碰。

审计：[Contract → Money Flow：OMS 雏形参考证据审计](./specs/connectors/contract-money-oms-prototype-reference-audit.md)

### M5 真实案例采集表与启动就绪门禁

范围与结果：

- 新增可直接交给业务人员填写的结构化案例采集入口，覆盖来源 revision／结算、账簿所有者与债权债务方、金额与到期日、确认责任、重复识别、来源变化、故障恢复及两个 Given／When／Then 样例；
- 明确只收脱敏业务事实，不要求业务人员设计 API、数据库或幂等键，也不收集真实姓名、账号、银行卡、证件或原始敏感文件；
- 以 R1 至 R9 九项证据定义 `draft`、`needs_business_confirmation`、`ready_for_contract_design` 和 `rejected_as_reference_only` 四种状态；任一必需事实缺失都不能创建 M5 功能编号；
- 明确案例通过只允许先设计版本化 snapshot、幂等规则、状态机和联调矩阵，不等于直接开始编码；
- 把 OA 参考判定为 `rejected_as_reference_only`，没有用默认值或技术猜测补全客户责任边界。

填写入口：[Contract → Money Flow 真实业务案例采集表](./templates/contract-money-flow-case-intake.md)

门禁：[M5 Contract → Money Flow 启动就绪门禁](./specs/connectors/m5-start-readiness-gate.md)

### M10-P Contract／Money D1 人工确认映射

范围与结果：

- Contract 与 Money 各自使用独立 Ed25519 工作负载密钥，IAM Policy 只允许 `party-directory.reference.read` 和精确 `party:{id}` Scope；没有共享密钥、共享表或通配业务动作；
- 当前 D1 两侧预检各有 6 条无业务引用的 Party；人工核对带唯一时间戳／随机后缀的测试来源后，各选择 1 条映射到同一个 Directory Party revision 1，其余 5+5 条保持未决；
- 首次执行均为 `recorded=1`，重复审核文件均为 `replayed=1`；同 Command 改负载、目标 revision 2、Party 停机和直接 SQL 更新全部失败，两个来源映射表计数始终各为 1；
- IAM 审计证明 Contract 1 次、Money 2 次交换均使用预期 audience、action 和精确 Party Scope；D1 恢复后全部服务 healthy，schema 分别达到 `000018` 和 `000017`；
- 输出持续为 `ready_for_cutover=false`。当前样本没有合同、义务、资金或结算引用，只验证一个 Tenant，运行时继续使用本地 Party；含引用数据、双 Tenant、显式切换／回滚和联合恢复仍待验。

报告：[Contract／Money Party D1 人工确认映射实验](./reports/oa-party-ledger-d1-reviewed-mapping-experiment-20260927.md)

## 下一功能

参考 OA Demo 已按高保真 J1-J4、设计首类型、人工确认 Money Flow 义务、历史纠错、幂等重放和五来源逐项故障隔离通过本地业务验收；从最小组合扩展到该基线的实际变化也已通过定制化六层账目、向后兼容和反证检查。`v0.1.0-demo.11` 目标无关开发包已从同一无源码归档复现六个独立产品、设计交付／商业台账两个子集及同一全量基线，并以只读预检收口目标机条件，成为公司私有 Ubuntu 验证前的可移植候选；正式交接仍须从干净 Git HEAD 重建。只有取得明确授权、目标主机、域名、Registry、TLS 和 Evidence Provider 输入后才执行完整入口环境写入。组合层继续只调用版本化 API，不吸收 Project、People、Contract、Money、Design、Evidence 的事实所有权。M5 自动连接器继续等待真实业务流，不因 Demo 需要而提前创建。

2026-09-28 C6 第四个收缩完成正式 D1 装配与恢复。C6 现有独立 Linux AMD64 非 root 镜像、数据库、migration、Workload Key、Suite Manifest、Workbench／Gateway 路由和联合备份项。隔离 D1 的 57 个正式入口断言全部匹配；Evidence／Party 停机时新命令 503 且零写入，恢复后同一业务意图只提交一次；observer 的金额被服务端脱敏，无 create 与错误 Project 均 403；IAM 审计精确命中 Project／Party／Contract／Evidence 四个资源。联合备份后销毁专属卷并从空目标恢复，Cost／Engagement／Assessment 的稳定 ID、revision、快照、历史和 canonical SHA-256 保持，恢复后 Cost 继续写入 revision 1→2。未选装时进程、菜单、路由和备份 dump 缺席，重新选装后授权与数据恢复。实验同时暴露旧 Contract 本地主体与 Directory Party ID 不一致：带 Contract 的 Engagement 返回 422，证明当前不会隐式名称映射，也把 Party 运行时切换／回滚定位为下一前置缺口。当前总进度更新为 85%；详细证据见 [C6 正式 D1 装配、故障与恢复实验](./reports/oa-c6-d1-formal-composition-recovery-experiment-20260928.md)。

2026-09-28 C6 第五个收缩完成 Payable Suggestion → Money 的真实跨进程业务旅程。C6 与 Money 分别连接独立 PostgreSQL 18 数据库，只经版本化 HTTP 协作；建议固定精确来源 revision／digest 与 Party／金额快照。Money 接受生成且仅生成一个自有 payable／open Obligation，拒绝生成零个；停机时 C6 保留 `target_unavailable`，恢复后显式同步，命令重放不重复，义务已提交而 intake 未更新的故障可用确定性子命令恢复。当前状态为 `real-cross-process-payable-suggestion-supported / formal-d1-complete-dual-tenant-real-ubuntu-pending`，总进度更新为 89%；证据见 [C6→Money 应付建议实验](./reports/oa-c6-payable-suggestion-experiment-20260928.md)。

2026-09-28 C7 完成奖金责任边界审计。参考 OA 的 OA 2026 奖池、宽 `FinancialRecord` 奖金行和独立 `Bonus` 表使用互相冲突的事实表达、当前 User 关系、浮点计算及更新／删除，不能直接复制。目标接受独立 Bonus Distribution：经营／生产共用严格奖池生命周期但使用 Contract Settlement／Money receipt 两类来源和不同 Capability；冻结来源、Person／Assignment、策略与舍入结果；发布后只追加取代／撤回，并向 Money 提交由其独立确认的建议。参考 OA 公式只作为 `reference-oa-bonus-v1` Overlay。当前状态为 `boundary-supported / implementation-not-started`，C7-A01～A18 已固定，总进度更新为 90%；未运行代码、数据库、IAM、页面、D1 或恢复。证据见 [C7 奖金责任边界审计](./reports/oa-c7-bonus-boundary-audit-20260928.md)。

同日 C7 第一实现收缩完成独立 `bonus-distribution` module、经营／生产类型化来源、精确 Project／来源／Person／Assignment／Policy 快照和 PostgreSQL 18 current／revision／receipt／audit。`reference-oa-bonus-v1` 使用精确有理数与 half-up，比例只使用整数基点；发布、取代和撤回只追加历史。真库 race 覆盖失败零写入、恢复、成功命令在依赖离线时本地重放、摘要冲突、八路并发单次提交、并发发布冲突、来源 drift、跨 Tenant／Project 隔离和数据库防篡改，并在测试中发现并修复“状态检查早于成功 receipt 导致发布后无法幂等重放”的缺陷。当前状态为 `domain-core-and-postgresql-supported / protected-http-and-composition-pending`，总进度更新为 92%；C7-A01～A09 已在核心层通过，A11／A12／A18 部分通过，HTTP／IAM、页面、D1、恢复与 Money 建议未通过。证据见 [C7 领域核心与 PostgreSQL 实验](./reports/oa-c7-domain-postgresql-experiment-20260928.md)。

同日 C7 第二实现收缩完成版本化 HTTP、`bonus-distribution` Ed25519 IAM、Capability Manifest、经营／生产 Assignment Policy、请求绑定 Project／People／Contract／Money resolver 和服务端金额脱敏。真实 PostgreSQL 18 加严格 HTTP 来源替身的 race 用例覆盖依赖失败零写入、恢复单次提交、成功重放零上游、一次 401 换票、404／409／422／503、错 audience／tenant／Project／相邻角色／资源，以及 current／Pool history／Allocation history 脱敏。当前状态为 `protected-http-iam-and-consumer-adapters-supported / live-source-process-composition-pending`，总进度更新为 94%；People Person 与 Money Receipt Set 的真实来源端点、人类工作区、独立进程组合、D1、联合恢复、C7→Money 建议和 C8 仍待完成。证据见 [C7 受保护 HTTP、IAM 与引用适配器实验](./reports/oa-c7-http-iam-adapter-experiment-20260928.md)。

同日 C7 第三实现收缩补齐 People Person 精确引用、Money 不可变 Receipt Set 及真实独立进程组合。IAM、Project、People、Contract、Money、Bonus 以六个进程和六个数据库运行；Bonus 仅用请求绑定工作负载凭证读取精确 Project／Person／Assignment／Settlement／Receipt Set，经营与生产奖金均从 1065.00 CNY 来源计算为 100.00 CNY 并发布。运行期没有共享表、静态来源 Token 或跨库事务。实验还以前向 migration 修复 Receipt Set 提交触发器，并补齐 Project Settlement link 数据库约束。当前状态为 `live-source-independent-composition-supported / human-workspace-and-d1-pending`，总进度更新为 95%；下一轮进入 C7 人类工作区，正式 D1、恢复、Money 建议和 C8 继续分轮验证。证据见 [C7 真实来源与独立进程最小组合实验](./reports/oa-c7-real-source-independent-composition-experiment-20260928.md)。

同日 C7 第四实现收缩补齐 Project 范围人类工作区。页面复用版本化 API 和目标 IAM，以服务端 `allowed_actions`、金额能力、状态和 revision 驱动经营／生产草稿、发布、取代、撤回与历史。六进程夹具完成正常两类发布、Contract 停机 503 且零新增 Pool、恢复、经营原子取代和生产撤回；真实浏览器读取 3 条 `published／superseded／withdrawn` 记录并显示精确来源、策略、分配与纠错关系，控制台错误为 0。当前状态为 `human-workspace-and-direct-recovery-supported / formal-d1-and-payable-suggestion-pending`，总进度更新为 96%；下一轮进入 C7 正式 D1／未选装／联合恢复，Money 建议与 C8 保持后续独立轮次。证据见 [C7 人类工作区与直接纠错／故障恢复实验](./reports/oa-c7-human-workspace-experiment-20260928.md)。

同日 C7 第五实现收缩完成正式 D1。Bonus Distribution 现有独立非 root 镜像、数据库、migration、工作负载身份、Capability Catalog、Suite Manifest、Workbench、Nginx→Gateway→IAM 入口和联合备份；16 条入口断言偏差为 0，5 条精确 workload audit 命中 Project／People／Contract／Money。备份 `20260928T143704Z` 经空目标恢复保持四份 canonical digest、ID、revision、快照和历史，恢复后生产 Pool 又从 revision 2 追加撤回至 revision 3；取消选装时菜单、路由、进程、migration、工作负载密钥和 dump 全部缺席，重新选装保留数据。当前状态为 `formal-d1-assembly-and-recovery-supported / payable-suggestion-and-single-type-minimum-pending`，总进度更新为 97%；下一轮 fresh selection 在 C7→Money 与 C8 中选择。证据见 [C7 正式 D1 装配、未选缺席与联合恢复实验](./reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)。

2026-09-29 C7 第六实现收缩完成 C7→Money Payable Suggestion 的直接跨进程业务旅程。Bonus 与 Money 使用独立数据库，只经版本化 HTTP 传递 exact Allocation／Person 快照；People Person 经人工审核、不可变的 revision／digest 映射解析为 Money individual Party。一个 PostgreSQL 容器和两个本地应用进程完成 11 条 HTTP 断言：目标离线时保存 `target_unavailable`，恢复后重试；接受生成唯一 Money payable／open Obligation，拒绝生成零个；重放不重复；映射篡改和未映射 revision 失败关闭；源奖金撤回不级联修改义务。验收结束后 Docker 运行容器为 0。当前状态为 `real-cross-process-payable-suggestion-supported / formal-d1-connector-suggestion-recovery-pending`，总进度更新为 98%；下一业务切片转向 C8，C7 正式 D1 connector／建议引用恢复和单类型最小组合保持独立补证。证据见 [C7→Money 应付建议实验](./reports/oa-c7-money-payable-suggestion-experiment-20260929.md)。

同日 C8 完成资料、统计与个人工作的静态责任边界审计。参考 OA 的项目文件页、公司收入／个人奖金统计和个人工作入口当前直接跨共享表查询，混用浮点金额且把查询与下载权限合并，不能直接复制。目标不新增共享报表库或统一可写待办：Project 只按 Resource Link 组合资料，Evidence 拥有 metadata／version／download，各领域应用发布指标与工作项，Workbench 只按当前 Subject 有界聚合。C8-A01～A18、局部失败状态、redaction 和第一实现顺序已固定。当前状态为 `c8-boundary-supported / runtime-not-started`，总进度保持 98%；没有运行 API、页面、正式 IAM／D1、数据库或恢复。证据见 [C8 边界审计](./reports/oa-c8-material-statistics-personal-work-boundary-audit-20260929.md)。

同日 C8 首个后端切片完成低资源单元验证。Evidence 将 metadata 与 Access Grant 兼容拆为 `metadata.read`／`download`，旧 view 只保留迁移兼容；Project 新增 `materials.read` 和 direct active Evidence Link 组合，保留 relation、linked/current revision、SHA-256、source status、耗时和局部失败。Evidence、Project 两个 module 均以 `GOMAXPROCS=2`、`go test -p 1 ./...` 串行通过，Access Contract 定向通过，全程 Docker 容器为 0。当前状态为 `c8-first-backend-slice-unit-supported / runtime-assembly-pending`，总进度仍为 98%；真实 PostgreSQL、IAM workload 换票、独立进程、页面、D1 与恢复尚未验证。证据见 [C8 首切片单元验证](./reports/oa-c8-first-backend-slice-unit-validation-20260929.md)。

随后 C8 direct Project→Evidence 最小运行组合通过。唯一 PostgreSQL 18.6 容器承载三个隔离数据库，IAM、Evidence、Project 为三个本地进程；Project workload 使用独立私钥向真实 IAM 换取单 `metadata.read`、精确 File Scope 凭证，IAM audit 为 1，download audit 为 0。HTTP 断言依次为 `200／403／200／200`，覆盖 current direct Link、stale revision、相邻 Project Scope、Evidence 停机局部 unavailable 和恢复 available；响应不含 bucket／object_key。脚本强制 Docker `0 → 1 → 0`。当前状态提升为 `c8-direct-material-runtime-supported / human-page-and-d1-pending`，总进度仍为 98%；页面、间接 Evidence、指标／工作项、正式 D1、双 Tenant 和真实 Ubuntu 仍未验证。证据见 [C8 Project→Evidence 运行实验](./reports/oa-c8-project-evidence-runtime-experiment-20260929.md)。

随后 C8 资料人类页面完成首批实现。Project 记录页按服务端 `allowed_actions` 显示“项目资料”，支持建立 Evidence Resource Link、浏览文件元数据／revision／摘要／局部状态，并把 Evidence 下载保留为 Suite 下的独立授权链。Project module 串行测试、嵌入 JavaScript 语法和 8 个交互状态结构通过；浏览器保持模式重跑后端时仍只有一个 PostgreSQL 容器，结束后清回 0。macOS 锁屏阻止真实浏览器点击，所以当前只提升为 `c8-human-material-experience-unit-supported / browser-and-d1-pending`，总进度保持 98%。证据见 [C8 资料页面首批验证](./reports/oa-c8-project-material-human-experience-unit-validation-20260929.md)。

随后 C8 完成首个业务来源间接资料切片。Business Engagement 发布带 Project、current Evidence 引用、source revision 和确定性 digest 的 `MaterialReferenceProjection v1`；Project 以专属 `business-engagement.material-reference.read` 读取后，再使用自己的 `evidence-service.metadata.read` 逐文件组合，不读来源表、不保存来源列表，也不获得下载权。Business Engagement、Project 和 Access Contract 三组测试按 `GOMAXPROCS=2`、`go test -p 1` 串行通过，覆盖错 Tenant／Project／source、两层 revision drift、业务来源拒绝、单个 Evidence 不可用和其他分段继续可读；全程未启动 Docker。当前状态为 `c8-indirect-business-material-unit-supported / runtime-and-d1-pending`，总进度仍为 98%。证据见 [C8 Business Engagement 间接资料投影单元验证](./reports/oa-c8-business-engagement-material-projection-unit-validation-20260929.md)。

随后 C8 完成首个个人工作来源的低资源切片。Production Operations 以独立 `production-work-item.work-items.read` 按 IAM Credential Subject、精确资源 Scope、active direct-person `production_owner` Assignment、current Work Item 状态和仍获授权的目标动作发布 `WorkItemProjection v1`；ended／stale Assignment、相邻 Subject／Project 和已撤销可执行动作不产生条目，引用服务失败返回来源级 503。Suite Manifest 新增可选版本化投影声明，Workbench 以最多四路和逐来源 3.5 秒读取、校验 Subject／source、排序／去重并只进入来源深链；Token 保留在 Gateway 的来源路径 HttpOnly Cookie 中，Workbench 不接收 Subject 或代发写命令。Production、Workbench、Gateway、Access Contract 四组测试串行通过，两个嵌入 JavaScript 语法及 D1 Manifest shell 语法通过，全程未调用 Docker。当前状态为 `c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`，总进度仍为 98%；真实 Gateway→IAM→Production 浏览器、正式 D1、第二来源、Group Assignment 和真实环境均待验。证据见 [C8 Production→Workbench 个人工作单元验证](./reports/oa-c8-production-work-item-workbench-unit-validation-20260929.md)。

随后 C8 完成首个类型化指标来源与单消费者。Money Flow 从自身 current Operating Summary 发布六个 `MetricProjection v1` 项，指标结构与金额能力分权；无金额权时值与摘要同时隐藏。Suite Manifest 再以可选 `metric_projection` 装入该来源，Workbench 严格校验 source／Tenant／Scope／精确值或 redaction，按来源、Ledger Owner Party 和 Currency 分区，不持久化指标、不跨来源合计，来源失败只降级自身。Money、Workbench、Gateway Manifest 与 Access Contract 分批串行通过，嵌入 JavaScript／JSON／diff 检查通过，全程未启动 Docker。当前状态为 `c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending`，总进度仍为 98%；真实 Gateway→IAM→Money、浏览器、第二来源运行时、正式 D1 和真实环境均待验。证据见 [C8 Money 指标来源单元验证](./reports/oa-c8-money-metric-projection-unit-validation-20260929.md)与[C8 Money 指标 Workbench 消费者单元验证](./reports/oa-c8-money-metric-workbench-consumer-unit-validation-20260929.md)。

同日 C8 公司 Ubuntu 完成正式多版本资料与完整升级。Evidence `test.7`、IAM `test.8`、Project `test.6`、Gateway／Workbench `test.10` 组成当前兼容闭包；正式 API 在稳定 File ID 下产生 Version 2，Project 资料严格经历 `available@v1 → revision_changed@v2 → available@v2`，Project revision 2→4，显式 Version 1 摘要保持。升级过程实际发现并修复旧 one-shot migration 容器复用、Evidence 备份恢复错误依赖和旧 Gateway／Workbench 拒绝新 Suite Manifest 三项问题。最终升级前备份 `20260929T132146Z` 校验有效，全部 migration 退出 0，12/12 长期容器 healthy，`d1 check` 通过；本机未启动整套 Docker。当前状态为 `c8-project-evidence-version-drift-d1-supported / human-browser-and-empty-target-restore-pending`，总进度仍为 98%，因为人类体验与阶段总门槛尚未关闭。证据见 [C8 Project 资料页面验证](./reports/oa-c8-project-material-human-experience-unit-validation-20260929.md)和 [OA 完整体架构验证报告](./reports/oa-full-scope-architecture-validation-report.md)。

同日 C8 Money 指标完成正式公司 D1 验证。Money 全量测试按 `GOMAXPROCS=2`、`go test -p 1` 串行通过，随后只发布 `test.11` Linux/AMD64 镜像；`v0.1.0-test.11` 以备份 `20260929T133853Z`、全部 migration 和同候选重建完成升级，12/12 长期容器 healthy。首次正式验收因新库 `items=[]` 主动失败；第二次只通过 Money 正式页面 API 建立显式 `C8-METRIC` 收款夹具，再以临时 IAM Subject 证明仅 `view` 为 403、`statistics.read` 返回 6 项全脱敏、增加 `amount.read` 后只有新凭证显示精确值、撤权后只有重新签发才恢复脱敏。Money 业务事实逐表行数与完整 SHA-256 前后同为 `cc60c4c734f4248a64d54f3e96111fc65bce1a76afef0af68ef9f075d09ff8b7`；临时 Subject 已停用且活动 Grant 为 0。当前状态提升为 `c8-money-metric-d1-supported / workbench-human-browser-and-second-source-pending`，总进度仍保持 98%；指标来源正式 D1 缺口关闭，但 Production 工作项、Business Engagement 间接资料、C8 真人浏览器、空目标恢复和完整连续旅程仍未关闭。证据见 [C8 Money 指标正式 D1 权限与只读边界验证](./reports/oa-c8-money-metric-d1-validation-20260929.md)。

同日 C8 Production 个人工作完成正式公司 D1 验证，并修正了一个真实语义边界：Project Assignment 保存 People Person ID，不是 IAM Subject ID；Production 现经独立 Project 与 People 工作负载凭证取得责任关系和版本化 Person Policy Fact，再与当前 Credential Subject 匹配，不共享表、不复制映射。定向模块测试通过，只发布 Production `v0.1.0-test.16@sha256:1eba80b9…f70`。首份组合因 `PEOPLE_BASE_URL` 放入错误服务环境块在 migration 前失败；备份与命名卷保持，后续门禁又正确拒绝在 Production 未运行时创建可能遗漏数据的新备份。校验前一完整备份并以不可变 `test.17` 恢复健康后，重新执行正式升级：备份 `20260929T151745Z` 的 11 项 checksum 全部 OK，全部 migration 成功，13/13 长期服务 healthy，`d1 check` 通过。第一轮验收因 bootstrap 与矩阵叠加命中固定一分钟限流 429；驱动只等待窗口自然翻页，没有重启 Gateway 或放宽限流。`test.18` 验收包随后在相同运行镜像上 7/7、HTTP 47/47 通过，覆盖当前 Subject 恰好一条、相邻 Subject 空、action/read 撤权的旧凭证窗口与刷新收敛、Production 停机时 Workbench 保持 200、来源恢复，以及事实摘要前后不变。当前状态提升为 `c8-production-personal-work-d1-supported / human-browser-second-source-and-full-journey-pending`；工程实现仍约 98%，阶段验收按未关闭 P0/P1 证据约 92%～94%，两者不再混用。证据见 [C8 Production 个人工作正式 D1 验证](./reports/oa-c8-production-personal-work-d1-validation-20260929.md)。

同日补齐全工作树静态应用边界门禁。`architecture-boundary-static-audit` 对 15 个应用分别用 `GOWORK=off go list -deps -test` 建立真实依赖图，拒绝 App→App 源码导入；应用本地 replace 只允许显式 `packages/*`；SQL 拒绝 dblink、FDW、Foreign Table 和跨库 Server。临时双 module 自测证明 clean fixture 通过、跨应用依赖和 dblink 两类负例均失败。实际工作树结果为 0 跨应用导入、0 跨应用 replace、0 跨数据库 SQL 机制，并已进入 `make check`。该项关闭总报告的静态依赖缺口，但完整运行流量、数据库账号权限和 C1～C8 连续旅程仍须独立验证。证据见 [OA 应用静态边界审计](./reports/oa-static-application-boundary-audit-20260929.md)。

同日完整 Suite 在公司 Ubuntu D1 完成装配验证。Billing、Approval／Results、Cost、Bonus 四个此前未进入该栈的应用先按低资源模式逐个完成全量测试并发布 Linux/AMD64 镜像；Bonus 过程中真实发现并修复 Dockerfile 漏带公共契约 package，以及 IAM 模式启动时错误预先要求 Money 静态 Token。最终不可变候选 `v0.1.0-test.20` 同时启用 12 个业务应用，17/17 长期服务 healthy；升级前备份 `20260929T155403Z` 的 15 项受保护材料 checksum 全部有效，全部 migration 和 `d1 check` 通过，服务器仍保留约 817 MiB available memory 与约 2 GiB swap。当前工程实现仍约 98%，直接证据提升为约 94%～96%；完整装配已证实，独立空目标恢复／恢复后写入、同数据连续 C1～C8 旅程及真人浏览器关键路径仍是 P0。证据见 [完整 Suite D1 装配验证](./reports/oa-full-suite-d1-assembly-validation-20260929.md)。

2026-09-30 完整 Suite 在公司 Ubuntu 完成隔离空目标恢复、恢复后写入和再次备份。`20260929T155403Z` 被恢复到独立 project／端口／全新卷，最终 17/17 healthy、十二个 schema current；正式 D1 运行验收通过 Nginx→Gateway→IAM 完成启用应用读写、Party 迁移预检、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。Evidence 从 `9/10/10` 增为 `10/11/11` 后生成新备份 `20260929T171025Z`，全部 checksum 通过；主栈始终 17/17 healthy，隔离栈验收后已停止。过程修正了非 root 恢复权限、128 Grant 超过 120/min、Bonus 相对路径误判、旧 Contract 镜像与当前 Capability 代际不一致及 AppleDouble 构建污染；失败 `test.25` 作废，接受候选为 `test.26`。工程实现仍约 98%，阶段直接证据提升到约 96%～97%；剩余最高价值门槛是同一 Suite／Overlay／数据集的 C1～C8 连续旅程，其次是真人浏览器关键权限路径、剩余正式 connector、双 Tenant 和离机跨机器恢复。证据见 [完整 Suite D1 隔离空目标恢复验证](./reports/oa-full-suite-d1-empty-target-recovery-validation-20260930.md)。

同日完整 Suite 连续旅程从 C2 推进到 C3。验收沿用同一 Tenant 和 Project，没有重建第二套主线；Money 对四个已引用主体建立显式不可变映射并切换 Party Directory 读权威，Billing 追加纠正发票，Money 独立确认应收／回款／核销／Receipt Set，Project 只新增三条稳定 Resource Link 并组合四来源经营摘要。无金额 Capability 时发票金额／币种返回 null；只停止 Billing 时只有 invoice segment 降级，恢复后四段全部可用。HTTP 40/40、所有权增量精确、最终 17/17 healthy；`test.34` 只封装可完成态重跑的 verifier 并复用固定镜像摘要。重跑保持相同业务 ID、revision 与增量。连续门禁现为 C0～C3，通过比例不能代替 C4～C8 的实际执行；阶段结论仍为 `insufficient-evidence`。证据见 [OA 完整 Suite 连续旅程 C3 验证](./reports/oa-full-suite-continuous-c3-validation-20260930.md)。

随后连续旅程从原 `state-after-c3.json` 推进到 C4。相同 Tenant／Project 和 C1 `production_owner` 被直接复用；Production 建立 `design-v1` 与 `consultation-v1` 两个 revision 8 工作项，Evidence 建立两个 File／三个不可变 Version。跨类型成果返回 422，stale revision 返回 409；只停止 Evidence 时报告命令 503 且零写入，恢复后同一 Command 单次提交。咨询报告又以 Version 2 替换并完成一次重开／再完成。Project／People 增量为零，Production／Evidence 增量精确，完成态重跑保持同一 ID／revision，最终 17/17 healthy。连续门禁现为 C0～C4（5/9，约 56%），阶段结论仍为 `insufficient-evidence`；下一批从 `state-after-c4.json` 单批次进入 C5。证据见 [OA 完整 Suite 连续旅程 C4 验证](./reports/oa-full-suite-continuous-c4-validation-20260930.md)。

随后连续旅程从原 `state-after-c4.json` 推进到 C5。相同 Tenant／Project、C1 业务负责人／参与者、C2 Contract 与 award Evidence 被直接复用；Results 只新增一条批复、一条审计和三条不可变 revision，Project 只在显式确认时新增一个 Completion Confirmation 并从 revision 10 前进到 11。Evidence／Contract 停服时批复 503 且零写入，恢复后同一 Command 单次提交；Assessment GET 零写入，参与者金额字段省略而非伪造零，Grant 撤销后纠正 403；恢复后 append-only correction 形成批复 revision 2，审计仍冻结 revision 1，Project 显示来源 drift。Results 停服时旧确认只做本地回放，新确认 503 且零写入。首轮 46 条和完成态重跑 14 条 HTTP 断言均零差异，精确 workload scope 与所有权增量通过，最终 17/17 healthy。连续门禁现为 C0～C5（6/9，约 67%），阶段结论仍为 `insufficient-evidence`；下一批从 `state-after-c5.json` 单批次进入 C6。证据见 [OA 完整 Suite 连续旅程 C5 验证](./reports/oa-full-suite-continuous-c5-validation-20260930.md)。

随后连续旅程从原 `state-after-c5.json` 推进到 C6。相同 Tenant／Project、C1 Actor、C2 供应商 Party／Contract 与既有 `target_unavailable` 建议被直接复用；Cost 形成 revision 2，Cost 自有 Engagement、Assessment、Payable Suggestion 到 revision 3，Money 独立接受并只新增一个 Obligation。Money 停服时来源保持可恢复失败状态，恢复后同一命令只完成一次；Cost 的 7 个 command／audit、Money intake 2 个 revision 与 Obligation revision 1 精确匹配，Project／Party／Contract／Evidence 均零新增事实。首轮 24 条和完成态重跑 15 条 HTTP 断言均零差异，最终 17/17 healthy。过程发现 Cost／Bonus 使用缩写 Money action 以及 Money IAMProvider 未接受 exact suggestion resource scope，两处均以版本化完整 action 与受限 exact scope 修复，未绕过目标应用最终授权。Cost `sha256:878200e8…3ee5`、Money `sha256:5262b567…9af` 已进入 `test.38` 固定摘要 release bundle。连续门禁现为 C0～C6（7/9，约 78%），阶段结论仍为 `insufficient-evidence`；下一批从 `state-after-c6.json` 单批次进入 C7。证据见 [OA 完整 Suite 连续旅程 C6 验证](./reports/oa-full-suite-continuous-c6-validation-20260930.md)。

随后连续旅程从原 C6 状态推进到 C7。经营奖池只读 C2 Settlement，生产奖池只读 C3 Receipt Set，两者复用 C1 Person／Assignment 并冻结 `reference-oa-bonus-v1`。Bonus 只提出建议，Money 独立接受并拥有唯一 Obligation；Money 停机形成可恢复的 `target_unavailable`，恢复后同一命令只提交一次；经营奖池撤回不级联改写已确认义务。首轮 25 条、完成态重跑 16 条 HTTP 断言零差异，最终 17/17 healthy。过程中修复 Project workload 对 Settlement 的精确类型和 Bonus 对合法 additive Settlement 字段的兼容读取，未放宽稳定不变量。连续门禁推进到 C0～C7（8/9）。证据见 [OA 完整 Suite 连续 C7 验证](./reports/oa-full-suite-continuous-c7-validation-20260930.md)。

同日连续 C8 在原 Tenant／Project revision 12 上完成 8 步运行。当前 production Subject 仅见同一 Project 的保留工作项，相邻 Subject 为空；Money 指标在 grant/revoke 与 fresh credential 后按预期在精确值和 `null/redacted` 间转换；Evidence／Production 每次只停一个，Workbench 始终 200，Project 分段或目标 API 局部失败并恢复。49 个 HTTP 断言零偏差，十二库完整 canonical TSV 前后逐字节相同，事实摘要清单为 `05491ab…92e`，结束 17/17 healthy。退出门禁仍失败：C4～C6 输出没有 active Project Link／资料投影；窄 `metadata.read + download` 因 Gateway 遗留 `view` 入口仍为 401。C8 状态 SHA-256 为 `67c184d6…c4a`，正式证据已固化到 `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c8`。阶段一总报告冻结为 `insufficient-evidence`，不以 8/9 冒充完整通过。详见 [连续 C8 验证](./reports/oa-full-suite-continuous-c8-validation-20260930.md)与[阶段总报告](./reports/oa-full-scope-architecture-validation-report.md)。

同批完成发布与静态审计：15 应用静态门禁及负例 self-test 通过，跨应用源码导入／replace／跨库 SQL 机制均为 0；Evidence 页面把契约不支持的 `redirect` 修为 `direct` 并加入测试；D1 release bundle 补入 C7／C8 verifier。Evidence HTTP package、D1 release bundle 和 upgrade selection 测试全部通过。上述仓库修正尚未成为公司栈新固定摘要，不能反写成 C8 浏览器已通过。

同日参考 OA 界面验收口径由“视觉近似”纠正为“源级复用”。Workbench 已直接复用参考项目的登录、侧栏、项目列表、项目详情、资料管理和统计页面 CSS／class 结构，并保留真实 IAM／Gateway 协议；本地逐页浏览器检查和 Workbench 测试／vet 通过。Linux/AMD64 `unified-workbench:v0.1.0-test.42@sha256:5f9a74bb…cdfb3f` 已单服务替换到公司 D1，容器 healthy，其他服务未重建。该项关闭“入口不像参考 OA”的问题，但静态示例事实、尚未绑定的写动作和 C8 退出门禁仍在，因此总体结论继续为 `insufficient-evidence`。

随后完成参考 OA 路由审计并明确“完整仿 OA”不能只靠已做的五类页面。设置、人员管理、权限配置已按源 TSX/CSS 结构补入同一 Workbench，并通过本地浏览器逐页点击、返回、版式和控制台检查；Linux/AMD64 unified-workbench:v0.1.0-test.43@sha256:898a8bad…07356 已单服务替换到公司 D1，17/17 healthy，其他服务保持连续运行。缺少的甲方、资质单位、公司收入、项目收入、合同详情参考页和操作手册已登记在逐页审计中，不能再用总进度数字掩盖。

随后补齐甲方、资质单位、公司收入、项目收入、合同详情和使用指南，并把登录入口纠正为参考 OA 的品牌标题、水印背景和居中登录卡片；认证仍使用真实 IAM 用户名／密码。上述页面在本地完成关键点击与响应式检查，`unified-workbench:v0.1.0-test.44@sha256:58714f4d…c47b` 已只替换公司 D1 的 Workbench，主栈 17/17 healthy，其他 16 个服务未重建；容器内全路由／样式探测和 D1 登录页真人浏览器复核通过。当前界面结论为 `reference-oa-ui-route-parity-d1-published / authenticated-d1-click-through-pending`；静态“界面验证数据”、未绑定写动作和登录后的 D1 全路由人工点击仍在，不能据此把完整 OA 业务结论改为通过。

随后补齐项目详情的深层经营／生产工作台，纠正“只有少量应用入口卡，不像参考 OA 实际工作面”的缺口。基本信息、经营 9 个二级页签和生产 11 个二级页签复用参考组件层级；本地实际点击基本信息、经营总览、合同、生产总览和施工图阶段，控制台无错误。Linux/AMD64 `unified-workbench:v0.1.0-test.45@sha256:d23d4959…e155` 已只替换公司 D1 的 Workbench，主栈 17/17 healthy，其他 16 个服务未重建；容器内页面／深层 CSS 探测通过。当前界面状态更新为 `reference-oa-deep-project-workspace-d1-published / authenticated-d1-click-through-pending`；静态界面数据、真实投影绑定和 D1 登录后全页人工点击仍是独立待验项。

随后补齐参考 OA 资料与统计的深层交互：资料项目从收起态支持多项目独立展开、文件下载摘要和多人明细弹窗；统计五个页签均有对应页面，人员奖金可展开项目金额，人事考勤／薪酬保留参考空状态。本地真人点击与 console 复核通过，Linux/AMD64 `unified-workbench:v0.1.0-test.46@sha256:85538982…b54c` 已只替换公司 D1 的 Workbench，原项目 17/17 healthy，其他 16 个服务未重建；容器内页面／脚本／CSS 标记与 SSH 隧道 200 通过。当前界面状态为 `reference-oa-p0-deep-interactions-d1-published / authenticated-d1-click-through-pending`，真实投影绑定和登录后 D1 全页人工点击仍待验。

随后继续补齐项目列表年表／分页／组合筛选、人员目录筛选与详情抽屉、职位／项目角色权限编辑状态，并纠正登录页虽然外观接近但字段仍不是参考 OA 的问题。默认首屏现复用源级手机号验证码布局；因当前 IAM 没有短信 Provider，点击获取验证码明确显示未接入提示，同卡片可切换到真实 IAM 用户名／密码，未用固定验证码冒充完成。全部本地点击、串行测试／vet／脚本检查通过；最终 `unified-workbench:v0.1.0-test.48@sha256:ad5996bb…a28c` 只替换公司 D1 Workbench，主项目仍 17/17 healthy。当前界面状态为 `reference-oa-primary-list-admin-login-ui-d1-published / authenticated-d1-click-through-and-sms-provider-pending`；登录后全路由、真实投影／写动作和短信能力仍分别待验。

随后纠正项目立项仍只是静态列表和直接跳转的问题，按参考 `ProjectInitiation.tsx` 补齐范围筛选、搜索、分页、新建／完善立项抽屉、普通项目／项目包切换、字段校验和删除确认。所有业务写入仍明确交还 Project Workspace 与 IAM，删除确认不会伪造删除事实。本地单进程真人浏览器完成完整交互、无横向溢出且 console 0 error/warning；串行测试、vet、JavaScript 和空白检查通过。`unified-workbench:v0.1.0-test.49@sha256:50f4a647…d7027` 已只替换公司 D1 Workbench，主项目 17/17 healthy，其他 16 个服务未重建；D1 容器内立项模板、抽屉、Project Workspace handoff、脚本、CSS 和验证码登录标记均通过。当前界面状态为 `reference-oa-initiation-deep-ui-d1-published / authenticated-d1-full-route-click-through-and-real-projection-binding-pending`，完整 OA 结论仍为 `insufficient-evidence`。

随后根据真人验收再次纠偏：匿名登录页不能被称为“仿 OA 网站”。Workbench 新增 `?preview=oa` 隔离只读演示，直接展示项目年表、详情和经营／生产工作区；预览不调用受保护投影，所有 Gateway 提交均被拦截并提示必须 IAM 登录。项目列表→京哈高速详情→经营信息已在本地和公司 D1 真人浏览器实际点击，console 0 error/warning。`unified-workbench:v0.1.0-test.50@sha256:159af0e3…40f1e` 已只替换公司 D1 Workbench，17/17 healthy，其他 16 个服务未重建。当前可见入口为 `http://127.0.0.1:18090/workbench/?preview=oa`；这关闭“只能看到登录页”的验收阻断，但真实 IAM 登录后的全路由、真实投影／写动作仍待验，完整 OA 结论继续为 `insufficient-evidence`。

随后继续按参考 `ProjectList.tsx` 修正只读 OA 项目列表：平铺的 S207 样本被装入“北环快速路工程项目包”，项目包可展开／收起两条子项目，搜索 `S207` 会自动展开并只保留该项目包；点击子项目后详情顶部同步名称、编号、日期和负责人，不再错误沿用京哈项目上下文。本地与公司 D1 真人浏览器点击、console、串行测试／vet／空白检查均通过。`unified-workbench:v0.1.0-test.51@sha256:ae07d038…836c` 已用正确的 `omsbizfoundry` Compose project 只替换主栈 Workbench，最终 17/17 healthy，其他 16 个主栈容器 ID 不变；一次因缺少 `-p` 产生的孤立 `d1` Workbench／网络已清理。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui51-project-package.env`。该批只关闭项目包层级与详情上下文 UI 缺口，完整 OA 结论仍为 `insufficient-evidence`。

随后继续对照参考 `ProjectDetail.tsx`／CSS 修正项目详情：项目包、子项目与普通项目现在有各自角标、配色、返回路径和子项目导航，正文名称、编号、日期、负责人、类型、概况及经营／生产阶段随选中项同步，不再仅替换标题而残留京哈项目内容；同时移除由参考模板替代的旧占位 DOM。本地和公司 D1 均实际完成项目包→S207→所属项目包→京哈普通项目往返，1280 像素视口无横向溢出。`unified-workbench:v0.1.0-test.52@sha256:55bb7572…0b83b` 已只替换主栈 Workbench，17/17 healthy，其他 16 个容器 ID 不变；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui52-project-detail-hierarchy.env`。该批关闭详情层级与正文上下文 UI 缺口，但深层静态样本、真实投影／写动作和最终验收仍在，完整 OA 结论不变。

随后按参考 OA 源码补齐甲方、资质单位与人员管理的新增／编辑抽屉及主体删除保护，不再把“跳转到真实应用”冒充源页面本身的维护交互。D1 真人浏览器已实际打开甲方新增、带既有资料的甲方编辑、已关联项目删除阻断、资质单位新增、人员新增，以及带 `lichen / 李晨 / 经营部 / 经营经理` 的人员编辑抽屉。预览提交只验证字段和流程，真实主体、账号与人员写入仍分别由 Party Directory、IAM 和 People & Groups 承担。首次 `test.53` 回归发现删除弹窗位于隐藏 view，已改为每个目录各自持有作用域弹窗并以 `test.54@sha256:407e94db…752a` 重新发布。公司 D1 最终 17/17 healthy，其他 16 个容器 ID 不变；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui54-delete-guard.env`。该批显著提高可见 OA 的 UI/UX 完整度，但真实投影／命令绑定和正式 IAM 连续旅程仍待证，阶段结论继续为 `insufficient-evidence`。

随后根据“展示的仍像登录／只读外壳而不是可操作 OA”反馈，对照参考 `BasicInfoTab.tsx` 补齐项目基本信息编辑抽屉。普通项目与项目包分别验证编号必填／选填，九类字段、负责人、日期、项目类型和概况字数都从当前项目上下文带入；提交只完成源级表单校验并明确把正式写入交还 Project Workspace 与 IAM。`unified-workbench:v0.1.0-test.55@sha256:91562457…2cb4` 已只替换公司 D1 Workbench，17/17 healthy，其他 16 个容器 ID 不变；D1 真人浏览器最终停留在“京哈高速改造工程 → 基本信息 → 编辑项目基本信息”，而不是登录页。该批关闭可见编辑入口缺口，真实写命令和完整 OA 最终验收仍未因此自动通过。

随后继续对照参考 `ProjectDetail.tsx` 补齐项目层级高风险操作：普通项目可进入项目包调整预检和删除确认，项目包可打开添加子项目抽屉及“删除项目包和全部子项目”精确确认，子项目可进入停用确认。浏览器测试发现并修复转换模式两组字段同时显示的 `hidden` 样式覆盖问题；本地与 D1 均逐项点击且 console 0 error。`unified-workbench:v0.1.0-test.56@sha256:17cab9a…edb10` 已只替换公司 D1 Workbench，17/17 healthy，其他 16 个容器 ID 不变；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui56-project-actions.env`。这些演示操作不会伪造 Project Workspace 写入，真实命令和完整 OA 连续验收仍待完成，阶段结论继续为 `insufficient-evidence`。

随后纠正“只看到登录页”和经营合同页仍是简表的两处真人验收偏差：可见标签已直接切到 `?preview=oa` 的 OA 项目业务界面，并按参考 `ContractAndAmendments`／`ContractForm` 补齐甲方合同、京宏合同、合同卡片、结算联动、780px 维护抽屉、补充／定案锁定和删除保护。本地与公司 D1 均实际点击，console 0 error／warning；`unified-workbench:v0.1.0-test.57@sha256:13cd4891…4498` 只替换主栈 Workbench，17/17 healthy，其他 16 个容器 ID 不变，回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui57-contract-drawer.env`。当前可见页停留在京哈项目的甲方合同抽屉，而不是登录页；真实 Contract Ledger 写命令和完整 OA 最终验收仍待完成，结论继续为 `insufficient-evidence`。

随后继续对照参考 `BiddingMethodControl`／`BiddingFileList`，把招投标页从单句空状态补为项目承接方式、三类材料要求、完成摘要、蓝／橙／绿文件槽位及预览／上传／下载／删除保护。所有写动作仍明确交还 Business Engagement、Evidence Service 与 IAM，不在 Workbench 伪造文件或承接事实。本地与公司 D1 均实际完成关键点击，console 0 error／warning；`unified-workbench:v0.1.0-test.58@sha256:61c14099…c9a17` 已只替换公司 D1 Workbench，17/17 healthy，其他 16 个容器 ID 不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui58-bidding-files.env`，当前可见页停留在京哈项目招标文件预览抽屉。真实招投标投影／命令和完整 OA 最终验收仍待完成，结论继续为 `insufficient-evidence`。

随后纠正经营支付／开票页仍只是金额卡片的问题，直接复用参考 OA `BusinessTransactionsPanel` 体系的双业务链分组、费用色记录行和 640px 维护抽屉。支付详情带入关联发票，开票详情带入关联支付和 Evidence 发票文件；所有保存／删除只做界面校验，真实资金、开票和文件事实仍分别属于 Money Flow Ledger、Billing 和 Evidence Service。低资源本地真人点击与公司 D1 真人回归均通过，console 0 error／warning；`unified-workbench:v0.1.0-test.59@sha256:3ef9575b…6cf17` 已只替换公司 D1 Workbench，17/17 healthy，其他 16 个容器 ID 不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui59-payments-invoices.env`，可见页停留在京哈项目开票详情抽屉。真实版本化投影／写命令和最终连续 OA 验收仍待证，整体结论继续为 `insufficient-evidence`。

随后把经营成本和奖金分配从摘要卡补成参考 OA 的真实工作面：成本具有统计、费用色记录、详情／新增抽屉、发票文件和删除保护；奖金具有奖池、比例／发布时间、统计、人员分配、按比例／固定金额联动和已发布历史保护。正式事实边界保持 Cost & Outsourcing／Evidence Service／Bonus Distribution／Money Flow／IAM，不在 Workbench 共享表或伪造写入。本地低负载真人点击和公司 D1 回归均通过，console 0 error／warning；`unified-workbench:v0.1.0-test.60@sha256:79f821b9…c44c69a` 只替换公司 D1 Workbench，17/17 healthy，其他 16 个容器 ID 不变。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui60-business-cost-bonus.env`，可见页停留在经营奖金详情抽屉。生产成本／奖金等剩余浅层页和真实投影／命令仍待继续，完整 OA 结论保持 `insufficient-evidence`。

随后继续把生产成本与生产奖金从摘要占位补为参考 OA 工作面，并再次明确登录页只是 IAM 入口、不是仿 OA 本体。生产成本现有 `¥775,000.00 / 12 条`、费用记录与维护抽屉；生产奖金现有 `¥8,520,000.00` 基数、`¥400,000.00` 奖金池及 6 人分配。经营／生产重复抽屉改为按当前作用域就近查找，避免数据串线。`unified-workbench:v0.1.0-test.61@sha256:df86eae4…955b0f` 已只替换公司 D1 Workbench，新容器 ID `4298d5cf2068`，主项目 17/17 healthy，其他 16 个容器 ID 不变；D1 真人浏览器经 Nginx 入口完成项目列表 → 京哈项目 → 生产信息 → 成本详情／奖金分配，console 0 error／warning。当前可见页直接保留在 OA 生产奖金工作面；真实版本化投影／写命令与正式 IAM 连续业务旅程仍待证，完整 OA 结论继续为 `insufficient-evidence`。

随后根据“仿 OA 不是登录页”的再次反馈，继续把公司 D1 的可见交付页留在真实项目工作台，并补齐生产人员与批复审计两块浅层页面。生产人员现有关键角色、专业分工、维护抽屉与删除保护；批复／审计现有双栏报告、Evidence 文件入口、费用拆分、总额和上游金额引用联动。本地与 D1 真人浏览器实际验证编辑及 `¥8,200,000.00 → ¥8,600,000.00` 联动，console 0 error／warning。`unified-workbench:v0.1.0-test.62@sha256:e4dff3b9…bc6d0a` 已只替换公司 D1 Workbench，新容器 ID `1b8e9cdd28a1`，主栈 17/17 healthy，其他 16 个容器 ID 不变；当前可见页停留在“京哈高速改造工程 → 生产信息 → 人员配置”。真实 People & Groups、Approval & Audit Results、Evidence、IAM 写入仍未由只读演示冒充，完整 OA 结论继续为 `insufficient-evidence`。

随后把方案、初步设计、施工图设计、变更竣工和咨询报告从文件名占位／空状态补为参考 OA 的完整阶段文件工作面。三个设计阶段现有状态摘要、主文件、校审单、评分和编辑抽屉；变更与竣工各自有列表及上传／替换入口；咨询报告具有主报告、多附件和追加入口。所有动作只验证 UI/UX 和 Evidence／Production／IAM 边界，不伪造文件写入。本地低资源真人点击和 D1 回归均通过，console 0 warning／error；`unified-workbench:v0.1.0-test.63@sha256:3bca6150…0e8a2` 已只替换公司 D1 Workbench，新容器 ID `b730db28e09b`，主栈 17/17 healthy，其他 16 个容器 ID 不变。当前可见页停在“京哈高速改造工程 → 生产信息 → 方案阶段”；真实版本化投影／命令和正式 IAM 连续旅程仍待证，结论保持 `insufficient-evidence`。

随后再次纠正“把登录页当成仿 OA 网站”的验收偏差：登录只保留为 IAM 身份入口，用户可见交付页固定为 `?preview=oa` 的参考 OA 业务工作台。本批直接对照参考 `UsageGuide.tsx`／CSS，将 3 条摘要 FAQ 补为 7 项目录、5 张模块卡、6 步流程和 26 个可展开问题；本地及公司 D1 真人点击“如何新建项目”均成功，console 0 warning／error。`unified-workbench:v0.1.0-test.64@sha256:8fb118d6…8f0f` 已只替换主栈 Workbench，新容器 ID `feb5cc370d95`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui64-full-guide.env`。当前可见页保留在完整操作手册；真实投影／命令与完整 OA 最终验收仍待证，结论保持 `insufficient-evidence`。

随后按“登记路由不等于可用页面”重新审计深层入口，补齐公司统计 → 公司收入 → 项目收入，以及项目经营 → 合同抽屉 → 完整合同详情两条链。项目收入现有管理费默认／自定义切换、四项统计和三类费用表；完整合同页现有金额、补充协议和 Evidence 文件页签。低资源本地验证和 D1 真人点击均通过，console 0 warning／error；`unified-workbench:v0.1.0-test.65@sha256:d5c68d79…0badee` 只替换主栈 Workbench，新容器 ID `f0146235c9d5`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui65-revenue-contract-route.env`，可见页直接保留在 OA“项目收入信息”；真实版本化投影／命令与最终完整 OA 验收仍待证，结论继续为 `insufficient-evidence`。

随后继续审计“已有页面是否只是浅层占位”，将生产信息的对外委托由两行简表补成参考 OA 工作面：委托汇总、受托方、委托内容、金额、Evidence 合同、Money Flow 支付、新增／编辑抽屉、动态支付明细和删除保护均可用。本地及 D1 真人浏览器均完成 `2 → 3 → 2` 支付增删、保存校验、合同预览与删除保护，console 0 warning／error。`unified-workbench:v0.1.0-test.66@sha256:9b4c10dd…829829ce` 只替换主栈 Workbench，新容器 ID `5fb1acfbefd9`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui66-external-commission.env`，可见页保留在 OA“生产信息 → 对外委托”。真实投影／写命令与最终完整 OA 验收仍待证，整体结论继续为 `insufficient-evidence`。

随后纠正经营参与人仍是头像摘要且混入经营负责人的问题，对照参考 `BusinessPersonnelList` 补齐账号、部门、项目角色、公司角色、候选人抽屉和删除保护；经营负责人继续在项目基本信息维护。正式人员／公司角色属于 People & Groups，项目角色快照属于 Project Workspace，正式命令经过 IAM。`unified-workbench:v0.1.0-test.67@sha256:15961d97…ef8aa` 已只替换公司 D1 Workbench，新容器 ID `1f5ea4cba610`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰。本地与 D1 真人浏览器均完成添加周宁校验和王可删除保护，console 0 warning／error；当前可见页保留在 OA“经营信息 → 经营参与人”。真实投影／写命令与最终完整 OA 连续验收仍待证，整体结论继续为 `insufficient-evidence`。

随后对照参考 `BusinessInfoTab.tsx` 把“经营信息 → 甲方信息”从三字段摘要补成内联维护工作面，包含历史名称候选、联系人、电话、取消和保存校验。Project Workspace 只保存项目快照，Party Directory 继续拥有主体主数据，正式命令继续由 IAM 保护。`unified-workbench:v0.1.0-test.68@sha256:55c6dbbe…f98d9` 已只替换公司 D1 Workbench，新容器 ID `ac2ec8ad3897`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰。本地与 D1 均实际完成“北方高速建设集团有限公司／李经理／010-88886666”校验，console 0 warning／error。同时从 D1 未登录入口复验了参考 OA 默认登录面：公司全称、水印、手机号和验证码是默认界面，IAM 账号密码只是当前无短信环境的显式备用入口。完整 OA 仍不会因登录页相似就结论通过，真实投影／写命令和连续业务验收尚待完成，整体结论保持 `insufficient-evidence`。

随后按参考项目类型规则纠正生产导航：设计、咨询、勘察不再共用固定页签；“临河新区市政道路勘察设计”显示设计／勘察类型与勘察专用试验阶段，“京哈高速改造工程”隐藏试验阶段。试验页已有多文件选择、无文件保护、待保存计数、既有 Evidence v1 报告与受控下载提示，正式阶段与文件事实仍分别属于 Production Operations／Evidence Service 并受 IAM 保护。`unified-workbench:v0.1.0-test.69@sha256:1608a506…db69a` 已只替换公司 D1 Workbench，新容器 ID `d0deeddd3185`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰；本地和 D1 真人浏览器均通过，console 0 warning／error。当前可见页保留在 OA“临河项目 → 生产信息 → 试验阶段”。审计同时记录临河顶部概览仍有京哈演示数字，下一批继续消除跨项目上下文残留，整体结论继续为 `insufficient-evidence`。

随后立即关闭该跨项目上下文缺口：生产概览的阶段、进度、文件数、负责人、阶段资料和首要入口全部改为随当前项目 fixture 切换。临河显示施工配合／5/5／16 份文件／陈工并直达试验阶段，S207 显示待启动／0/5／0 份文件／待设置且各设计阶段未设置。`unified-workbench:v0.1.0-test.70@sha256:65b3df42…093145` 已只替换公司 D1 Workbench，新容器 ID `645d117f751c`，主栈 17/17 healthy，其他 16 个容器 ID 不变，恢复演练栈未触碰；本地与 D1 真人浏览器均通过且 console 0 warning／error。可见页继续保留在临河试验阶段；界面上下文一致性已修复，但真实版本化投影／命令与完整 OA 最终连续验收仍待证，结论保持 `insufficient-evidence`。

随后对照参考 `ProductionInfo.tsx` 补回随项目类型变化的生产阶段快捷入口，并把“按需维护”恢复为默认折叠结构；v71 在本地和 D1 均完成临河／S207 差异、直达和展开收起验证。紧接着从参考前端真实 import 图审计 14 个页面、79 个可达组件，关闭最后两个明确遗漏：侧栏产品反馈和项目详情流程助手。v72 已只替换公司 D1 Workbench，新容器 ID `eda28550134f`，主栈 17/17 healthy，其他 16 个容器 ID 不变，17 个恢复演练容器未触碰；D1 真人浏览器确认临河生产业务页、8 项无／不适用设置、反馈菜单和助手折叠／展开，console 0 warning／error。当前用户可见页直接保留在 OA 生产业务工作台，不是登录页。参考 OA 的 UI/UX 与保护性交互可单独判定通过；真实投影／命令和完整 OA 连续业务旅程仍按原报告保持 `insufficient-evidence`。
