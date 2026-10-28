# OMSBizFoundry 开发计划

2026-10-28 管理员全权限画像公网发布（BF-279）：IAM `oms-administrator-full-access-v1`、Workbench 强制刷新和六个 OMS Owner 的当前多租户鉴权闭包已经发布。线上诊断确认管理员 176 条精确 Grant 已存在，实际拒绝来自部分旧 Owner 把 Credential 固定在早期 Tenant；Party、Contract、Money、Billing、Evidence、Project 已升级，Evidence／Money 缺失 migration 已补齐。Project `v0.1.0-test.oms-v1.48` 及完整同版本 D1 编排发布后，联合备份、19 个正式容器、0 unhealthy、独立 D1 `check` exit 0 和真实管理员浏览器“新增甲方”保存动作均通过；复验没有提交测试业务事实。[验证及发布记录](./reports/oms-administrator-default-access-profile-validation-20261028.md)

2026-10-27 账号操作列排版修复（BF-278）：Workbench 账号表为最后一列增加明确的“操作”表头，并使用 190px 最小宽度、右对齐、禁止动作容器与按钮文字换行；窄视口由既有表格滚动容器承接。Go 单测、全模块测试、vet、diff、release bundle 与 checksum 已通过；D1 Suite `v0.1.0-test.oms-v1.43` 仅替换 Unified Workbench 发布，正式 D1 `check`、19/19 running、0 unhealthy、精确镜像断言和公网真实管理员页面走查通过。本项不改变 IAM、账号级别、组织、Owner、API、schema、Grant 或业务数据。[验证及发布记录](./reports/oms-v1-account-action-column-layout-validation-20261027.md)

2026-10-27 组织设置修订（BF-277）：IAM 已增加本组织管理员改名命令与 `POST /api/v1/admin/tenant`，使用 Session Tenant、Tenant 行锁和不可变审计；同名保存幂等，普通用户和系统管理员越权拒绝。Workbench“账号与租户”页增加“组织设置”，可编辑组织显示名称并只读展示稳定 ID，同时明确企业证照资料属于主体档案。Identity／Workbench Go、vet、Node、diff、页面资源回归及短生命周期 PostgreSQL 18 完整 HTTP 生命周期通过；D1 Suite `v0.1.0-test.oms-v1.42` 的联合备份、正式升级、D1 检查、19/19 healthy 和公网管理员页面复验也已通过，发布验收未修改现有组织名称。[验证及发布记录](./reports/oms-v1-tenant-display-name-settings-validation-20261027.md)

2026-10-27 既有用户管理员挂载修复（BF-276）：Identity 已把系统端“新增组织管理员”收敛为一个分支明确的命令：同组织启用普通用户提升、已有管理员幂等、无账号才邀请，跨组织／系统管理员／非启用稳定拒绝；提升会写审计并使目标旧 Session 失效。Workbench 改为“设置组织管理员”，分别反馈提升、幂等和激活链接，并透传 IAM 业务错误。本地 Identity／Workbench Go、vet、Node、diff 以及短生命周期 PostgreSQL 18 migration 9／HTTP 生命周期均通过；修复已随 D1 Suite `v0.1.0-test.oms-v1.41` 发布，升级前联合备份 `20261027T132851Z`、schema 9、D1 检查、19/19 healthy、公网页面及资源版本核对通过。发布验收没有替用户提升真实账号，下一步由系统管理员在新页面明确确认目标账号。[验证及发布记录](./reports/oms-v1-existing-user-administrator-assignment-validation-20261027.md)

2026-10-27 系统用户管理修订（BF-275，管理员授权部分由 BF-279 修订）：登录页公开创建组织候选已撤销，账号页面统一使用“普通用户／管理员”。IAM 已显式建模系统管理员，系统管理页、全组织列表、原子创建组织＋首位管理员激活链接、现有 `admin` 迁移、组织至少一名管理员保护、Gateway 可信上下文及系统管理员业务 Capability 拒绝均随 D1 Suite `v0.1.0-test.oms-v1.39` 发布；本地真实 PostgreSQL migration／HTTP 生命周期、Go、vet、JS 与页面隔离，以及公网联合备份、schema 9、19/19 healthy、真实系统管理员登录、组织读取和零业务写入验收均通过。组织管理员由 BF-279 改为自动挂载当前已安装功能全业务权限画像；People 成员页面与职位／业务角色不随本项修改，用户—成员关联另走稳定引用。[账号规格](./specs/identity/oms-v1-account-onboarding-v1.md)、[管理员权限画像](./specs/identity/oms-administrator-access-profile-v1.md)、[BF-275 验证及发布报告](./reports/oms-v1-system-user-tenant-management-validation-20261027.md)

2026-10-26 BF-274 的公开注册候选未部署，并已由 BF-275 取代；兼容的管理员级别调整已按“用户”术语完成验证。

2026-10-26 工资可用性修订（BF-273）：北京 2026 参考方案、Payroll 固定额／整元取整计算，以及工资页“设置比例”的查看、复制、调整和确认已随 D1 Suite `v0.1.0-test.oms-v1.37` 发布。发布后 schema 4、19/19 healthy、真实标准方案读取和公网资源核对通过；未在验证中复制租户方案或写业务数据。下一步由用户登录体验并按本公司实际工伤档位、公积金比例创建 Tenant 方案；全国地区政策平台和自动合规判定仍不进入本轮。[本地验证](./reports/oms-v1-beijing-contribution-scheme-validation-20261026.md)、[发布记录](./reports/oms-v1-beijing-contribution-scheme-publication-20261026.md)

2026-10-26 账号开通修订（BF-272）：管理员直接录入手机号／邮箱、生成 24 小时一次性激活链接、本人设置密码的 IAM 流程，以及待激活重签、旧链接失效和手机号／邮箱登录已随 D1 Suite `v0.1.0-test.oms-v1.38` 发布。该版本修正首次发布中 Workbench／Identity 字段契约不一致的问题；schema 8、19/19 healthy、备份与公网入口核对通过，未创建测试账号。下一步由用户完成真实新增／激活旅程；自动发送、同账号多租户和公共注册仍不进入本轮。[规格](./specs/identity/oms-v1-account-onboarding-v1.md)、[发布记录](./reports/oms-v1-account-onboarding-publication-20261026.md)

2026-10-26 页面与接入修订（BF-268～BF-271）：本地分支把报销／付款拆为两套表单，按现金／银行卡约束收款信息，并统一经营列表列语义；账号／租户改为 IAM 实际页面；Feedback 改为所有有效登录账号可用，移除 `feedback.use` Manifest／Grant，并把 C10／C11 改为 Session 门禁。BF-271 进一步把反馈和退出接到侧栏／顶部可见入口，并让侧栏业务导航独立滚动。受影响 Go／Node、真实本地 Chrome、P9 C10／C11 离线驱动、D1 bundle 和入口模板回归已经通过。下一步是本地人工页面走查及用户确认；未部署 D1／公网，外部 project 115 与真实恢复仍待原 P9 窗口执行。[验证报告](./reports/oms-v1-ui-identity-feedback-adjustment-validation-20261026.md)

2026-10-26 P9 增量（BF-267，授权部分由 BF-270 取代）：C11 真实 Chromium 驱动、固定证据 verifier、`run-c0-c11` 与全链 verifier 随 D1 release bundle 交付。C11 双 Tenant 打开 Feedback submit／status并核对跨域 SDK；主 Tenant 退出后配置必须 401，重新登录后入口恢复，Feedback 不再执行 Capability 撤销／恢复。总入口先验证凭证并真实启动／关闭浏览器，再允许进入早期阶段或破坏性 C9；已有完整 C11 时只读复验全链并零执行退出。下一步只在真实 D1 使用 `run-c0-c11` 完成全组合并单独补回滚记录；不再新增业务阶段，也不自行增加未确认的 soak 时长。

2026-10-26 P9 增量（BF-266）：新增随 D1 release bundle 交付的 `run-c0-c10` 与只读 `verify-run-c0-c10`，把 C0～C8 连续业务、C9 原项目空目标恢复和 C10 外部 Feedback 收成一个可中断恢复入口；C9 仍需独立显式确认，C10 的开发者回复等待仍以 exit 3 暂停。离线完整制品复用、checksum-valid C9 断链和未授权 C9 拒绝通过。下一步只执行真实全组合窗口并补真实浏览器 Console／点击与回滚记录；需求没有规定 soak 时长，实施不得自行制造时长门槛，也不得继续增加业务阶段。

2026-10-26 P9 增量（BF-265）：新增随 D1 release bundle 交付的 C10 Feedback V2 真实旅程门禁。它绑定 C0／C1／C9，以两个 Tenant 的既有管理员和 Gateway 配置分别创建或复用两条 project 115 会话，幂等追加带 URL 附件的客户消息；没有平台开发者回复时以 `PENDING` 停止且不出具证据，回复后复跑验证开发者消息、两条未读以及逐会话已读隔离。离线 pending／resume 与证据正反例通过；下一步只在真实 P9 窗口执行 C0～C10、完成人工回复和最终有界稳定运行，不再增加新的业务阶段。

2026-10-26 P9 增量（BF-264）：新增随 D1 release bundle 交付的 C9 联合恢复门禁。它只接受同一 C0～C8 证据链与显式 `P9_C9_CONFIRM_IN_PLACE_RESTORE=yes`，独立核对完整备份闭集、停写声明、十四应用选择和十五库指纹，再对原 Compose project 执行清卷恢复；恢复后必须通过 D1 检查、两份 C6 发票逐字节摘要、C7／C8 稳定命令重放与业务事实等价，以及两 Tenant 新增 Open API usage audit。备份本体、签名密钥、凭证和短期 URL 不进入 C9 证据。离线 verifier 正反例通过；真实 D1 C9 尚未执行，下一段进入真实 Feedback V2 旅程和最终有界运行。

2026-10-26 P9 增量（BF-263）：新增随 D1 release bundle 交付的 `run-c0-c8`，把九个已有阶段收敛为可恢复的单一连续入口；既有阶段先独立复验再复用，最终逐边核对 project、Origin 和全部前序 checksum 绑定。离线完整链与 checksum-valid 断链负例通过。下一步不再扩展业务阶段，转入同一链完成后的联合备份／原项目空目标恢复、恢复后关键读取／续写及 Feedback 真实旅程；真实 D1 尚未执行。

2026-10-26 P9 增量（BF-262）：C8 工资实际发放驱动和固定证据 verifier 已进入源码及 D1 release bundle。它绑定 C0～C7，为两个 Tenant 的既有 C3 工资行分别建立 Payment Profile、显式同步支付快照、完成工资核对并经 Payroll→Money 发放精确净应发金额；验证完整核销、零超额、本人工资单和跨 Tenant 404。完整银行卡号不落证据，成功支付命令可安全重放；该阶段不直写数据库且明确沿用 Money 本地兼容 Party，不宣称 Party Directory 已统一。下一阶段进入同一连续链的联合备份／空目标恢复、恢复后读取／续写和 Feedback 真实旅程；真实 D1 C8 尚未执行。

2026-10-26 P9 增量（BF-261）：C7 报销实际付款驱动和固定证据 verifier 已进入源码及 D1 release bundle。它绑定 C0～C6，为两个 Tenant 分别经 Money 公网命令登记本地兼容 Party、经 Money 审计 ops 设置 Finance 账簿主体及精确 Person 收款映射，再由授权财务管理员确认两笔完整实际付款；验证 Money Obligation／Payment／Allocation 引用、本人 Open API `paid` 回读和跨 Tenant 404。该阶段不直写数据库、不宣称 Party Directory 已统一。下一连续阶段单独补齐工资支付档案、工资核对与实际发放，再进入联合备份／空目标恢复和 Feedback；真实 D1 C7 尚未执行。

2026-10-26 P9 增量（BF-260）：C6 员工发票上传／报销驱动和固定证据 verifier 已进入源码及 D1 release bundle。它绑定 C0～C5，用两把既有员工 Key 分别完成虚构 PDF 实字节直传、Evidence `invoice-v1` 校验、稳定引用冻结、本人报销提交／列表／详情和跨 Tenant 404；报销停在待支付，未调用实际付款或 Money。下一连续阶段由授权财务人员确认两笔实际付款并核对 Money 唯一事实，再进入联合备份／空目标恢复和 Feedback；真实 D1 C6 尚未执行。

2026-10-26 P9 增量（BF-259）：C5 员工 API Key 运行驱动、固定证据 verifier 和受限 Secret 恢复约定已进入源码及 D1 release bundle。它绑定 C0～C4，为两个既有 Person 各建立或复用一把默认 180 天 Key，验证脱敏自助列表、可信身份绑定、Webhook 占位和本人调用审计；Secret、密码、Cookie 与 Authorization 不进入证据，C5 不做业务写。下一连续阶段用同一 Key 进入 Evidence 发票上传与本人报销，再由财务确认实际付款；真实 D1、恢复与 Feedback 仍待有界运行窗口。

2026-10-26 P9 增量（BF-258）：C4 财务申请运行驱动与固定证据 verifier 已进入源码和 D1 release bundle。它绑定 C0～C3，复用两 Tenant 的 C1 Person 快照，分别创建无附件但显式确认的付款申请，并验证 `payable`／`not_required`、敏感账号脱敏、本人列表隔离、交叉 404 和零付款／零 Money。下一连续阶段才进入员工 API Key＋Evidence 发票上传＋本人报销；真实 D1、实际付款、恢复和 Feedback 仍待有界运行窗口。

2026-10-26 BF-257 完成 P9 C3 工资表运行驱动、独立证据 verifier、双 Tenant 幂等重跑／快照／隔离／秘密扫描回归和 release bundle 装配。C3 使用明确非法律依据的六项零比例 fixture，只证明工资周期生成、上游 revision 冻结、无需审批与未支付边界；不发薪、不写 Money。下一段应从同一 C0→C3 链进入财务申请与开放 API 发票报销，再连接实际付款；真实 D1 C3 尚未执行。

2026-10-26 BF-256 完成 P9 C2 成员到薪酬档案运行驱动、独立证据 verifier、双 Tenant 幂等重跑／隔离／秘密扫描回归和 release bundle 装配。下一段应在同一 C0→C2 链上显式准备版本化缴费方案并生成工资周期，再进入 Finance／开放 API 报销；比例来源未确认前只可使用清楚标记的验收方案，不能冒充现行法定规则。真实 D1 C2 尚未执行。

2026-10-26 BF-255 完成 P9 C1 双租户运行驱动、独立证据 verifier、离线首次创建／秘密不落证据回归和 release bundle 装配。下一真实窗口必须先通过 C0，再用正式密码文件执行 C1；后续仍需把 Payroll、Finance、开放 API 报销、Feedback、联合备份恢复和恢复后续写串到这两个 Tenant，不能把 C1 工具存在等同于运行通过。

2026-10-26 BF-254 修复 P9 前置的同域名多租户入口缺口：Gateway 不再把签名会话／Capability 的 Tenant 强制等于 Manifest 默认 Tenant，Workbench 改用可信会话 Tenant，成功登录统一清除旧能力 Cookie。Gateway／Workbench 回归通过；下一步必须在 P9 正式 D1 运行窗口创建第二 Tenant，连续验证两个账号的 Owner 读写、跨租户拒绝、Feedback 与联合恢复，当前仍不能宣称 D1 多租户完成。

2026-10-26 BF-253 补齐 P9 发布来源链：C0／证据验证器随 D1 bundle 交付，要求正式 Git commit、匹配 Suite version、`linux/amd64`、pull mode 和全量 digest-pinned image；开发 provenance 与 mutable tag 负例通过。下一真实窗口必须从干净 HEAD 构建正式包，不能直接拿工作树或目标机 build 充当验收版本。

2026-10-26 BF-252 修复 P9 C0 镜像证据歧义：运行容器实际 ID 必须匹配配置引用解析 ID，标签漂移直接失败；独立 verifier 不只验 checksum，还复核 19／19 健康、镜像对应关系和 Nginx 唯一发布端口。mock 的运行时漂移及 checksum-valid 语义漂移负例通过。

2026-10-26 BF-251 为 P9 C0 增加固定闭集证据清单与独立复验：健康、镜像、readiness、Manifest、拓扑、检查日志和状态任一被修改都失败，额外文件也拒绝；mock 篡改／额外文件负例通过。该源码门禁不把 C0 记为真实通过。

2026-10-26 BF-250 把 public sequence 定义与当前位置加入 BF-248 数据库指纹，防止“表数据恢复一致但下一次自增写入错位”。PostgreSQL 18 临时容器已验证 `nextval` 改变摘要且退出后删除；P9 仍须在真实恢复后执行有界续写。

2026-10-26 BF-249 把数据库指纹语义校验前移到恢复的破坏性操作之前：格式、表／行计数、摘要和数据库选择集合任一异常都不得调用 `down --volumes`。mock 已覆盖“篡改表数并重算 checksum”仍提前拒绝，以及旧格式无指纹继续兼容；真实恢复不因此记为通过。

2026-10-26 BF-248 补齐 P9 的自动事实比对门禁：新 D1 联合备份在 BF-247 停写窗口内生成十五库全选时的无秘密 schema／事实指纹，空目标恢复在业务服务启动前重算并严格比较；IAM Session 数据按原 dump 契约排除，旧备份兼容。无状态 mock 已覆盖匹配通过和故意漂移失败关闭，PostgreSQL 18 短生命周期实测覆盖空表、事实行和敏感值不出库；真实十五库备份／恢复仍待 P9 运行窗口。

2026-10-26 BF-247 修复 P9 联合备份的停写缺口：原 D1 在 Nginx 仍开放时按 Owner 顺序 dump，Finance／Payroll 与 Money 可能跨检查点漂移；现显式备份先关闭唯一公网入口、等待可配置有界 drain、写入受 checksum 保护的一致性声明，再执行全套 dump，成功或失败恢复原入口。升级预备份成功后保持入口关闭直至新组合接管。mock 已验证停入口早于首个 dump、重启晚于最后一个 dump、Payroll dump 失败仍恢复入口、非法 drain 失败关闭；真实跨库空目标恢复仍属于 P9 运行窗口。

2026-10-26 BF-246 补齐 P8 本地真实浏览器桥接门禁：生产 Feedback 宿主脚本已在真实 Chromium、两个随机跨 origin 回环服务中通过 29 项断言，覆盖配置、单 iframe、精确消息来源、提交／状态、未读与迟到响应、换票、退出清理、403 隐藏和 503 局部降级；运行结束无常驻服务。该证据不连接项目 115，P8 仍缺有效运行凭证、真实通知／附件／回复／单条已读、双账号／双租户与生产 Origin／公网 Console 证据；当前低资源工作继续保持 D1 容器为零。

2026-10-26 BF-245 修正 P8-P9 运行前门禁：Feedback SDK URL 已从错误的 Bonus 环境迁回 Workbench，D1 对项目 115、`apiKey`、URL、HMAC、通知和轮换 Key 成组失败关闭；P9 另增加只读 C0 入口，真实窗口将先核对十四应用、十九服务、镜像 digest、健康、入口与 schema，再创建业务夹具。上述离线自测只消除启动后才暴露的配置／证据泄露风险，不提高 P8／P9 运行完成度；下一步仍是取得非占位运行配置后，在一次有界窗口执行真实全组合、跨域 Feedback、联合恢复和清理。

2026-10-26 当前实施检查点：P9 的 D1 源码装配门禁已补齐 Finance／Payroll 下游短期凭证、依赖选择、key lifecycle 与静态组合回归（BF-244），但没有启动全组合，不能登记 P9 完成。P8 已增加本地真实 Chromium 跨 origin 宿主门禁，但项目 115 通知、真实运行凭证、双账号／双租户与外部跨域验收仍未完成；P9 还需一次有界全组合连续旅程、联合备份／空目标恢复、恢复后复跑、镜像 digest 和浏览器 Console 证据。由于当前开发机资源紧张，先保持常驻容器为零，在具备运行配置后再一次性执行并清理，不得以本地夹具、源码或 Compose 通过替代 P8／P9 运行结论。[P9 装配源码报告](./reports/oms-v1-iteration-p9-composition-source-validation-20261026.md)、[P8 源码报告](./reports/oms-v1-iteration-p8-feedback-v2-source-validation-20261026.md)

2026-10-26 P7 已完成本地门禁（BF-242）：开放 API 已提供 Evidence 发票上传完成、本人报销草稿／提交／撤回／归档恢复／删除、稳定列表／详情及已支付投影；Evidence、Finance、Money 保持独立事实所有权。真实 TLS 五进程／四库验收 13 个 HTTP 断言零偏差，覆盖 Evidence／Finance／Money 故障与原幂等标识恢复、危险 PDF、强重复、唯一资金／核销、本人 paid 查询和 Key 撤销；相关单元、vet 与 PostgreSQL 集成测试通过。下一阶段进入 P8 Feedback SDK V2，P7 未部署 D1／公网。[P7 报告](./reports/oms-v1-iteration-p7-invoice-reimbursement-validation-20261026.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P6 已完成本地门禁（BF-241）：IAM migration 7 已建立员工 API Key、事件、脱敏调用记录和 180 天清理审计；正式 People Reference、当前 Grant／账号复验、一次 secret、期限／数量上限和不可逆撤销已接通。Gateway 已提供 `/open-api/v1/` allowlist、HTTPS、Key／Tenant 限流、统一错误、OpenAPI／中文示例及审计回写；Workbench 已提供开放平台页面和 Webhook 未开放占位。下一阶段进入 P7 Evidence 发票上传、报销命令、本人记录查询和 Finance→Money 纵向闭环；P6 未部署 D1／公网。[P6 报告](./reports/oms-v1-iteration-p6-open-api-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P5 已完成本地门禁（BF-240）：Internal Finance Requests 的报销／付款草稿、明细、类别、收款与费用归属快照、Evidence 引用、无需审批快照、撤回 revision、敏感访问审计及逐笔／批量付款已经形成独立 PostgreSQL 事实；Finance 经版本化端口把实际付款交付 Money，Money 稳定建立义务、Payment、整单 Allocation 和超额未分配余额。双进程隔离验收覆盖分次／超额、批量部分成功、撤回重提、普通／敏感导出及空目标恢复；Workbench 已切换正式 Owner。下一阶段进入 P6 员工 API Key 与 `/open-api/v1/`；本批未部署 D1／公网。[P5 报告](./reports/oms-v1-iteration-p5-finance-payment-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P4 已完成本地门禁（BF-239）：Payroll 的月度工资表、冻结成员／薪酬／账户／方案快照、整数计算、导入预览、核对、同步、状态机和调整批次已形成独立 PostgreSQL 事实；逐人／批量实际发放经版本化连接器进入 Money，形成稳定工资义务、实际资金、核销和超额未分配事实。三进程隔离验收覆盖部分成功、缺账户、二次超额确认和幂等引用；Owner 集成测试覆盖同月并发、未知恢复、零／负实发、作废／替代、补发／扣回和本人可见性。Workbench 正式工资页已接 Owner API。下一阶段进入 P5 财务申请、审批占位与付款确认；本批未部署 D1／公网。[P4 报告](./reports/oms-v1-iteration-p4-payroll-execution-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P3 已完成本地门禁（BF-238）：People & Groups 的成员任职生命周期、Payroll 的 Compensation／Payment Profile 和标准／自定义 Contribution Scheme 已形成独立 migration、API、能力、幂等 revision、审计和 Workbench 组合旅程；Payroll→People 使用版本化资源引用和独立工作负载身份。新增隔离验收覆盖新增／离职／重入职、离职期间薪酬维护、生效月、缺账号、账号遮罩／历史、未知 Person、稳定快照和匿名拒绝；双租户、撤权与不可变历史由 PostgreSQL／HTTP 测试覆盖。下一阶段进入 P4 工资表、核对与实际发放；本批未部署 D1／公网。[P3 报告](./reports/oms-v1-iteration-p3-member-payroll-profiles-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P2 已完成本地门禁：`Internal Finance Requests` 与 `Payroll` 已作为两个独立 Owner 建立最小草稿聚合、PostgreSQL schema、版本化 API、可信 Actor／Tenant／Capability／resource scope 授权、expected revision、幂等、不可变审计、镜像、D1 可选装配和备份恢复。两个独立镜像的写入、重放、冲突、备份、空目标恢复及续写通过；发布工具链已扩展到 19 镜像。当前进入 P3 成员与薪酬档案，不代表新服务已部署 D1／公网，也不代表工资／财务闭环已完成。[P2 报告](./reports/oms-v1-iteration-p2-owner-skeleton-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P1 已完成本地门禁：IAM 支持同一进程多 Tenant 登录／会话／授权和平台侧 Tenant＋首管理员创建；People 保存同租户 Subject 稳定快照；所有受保护 Owner 与工作负载适配器均从可信凭证透传请求 Tenant。真实 IAM＋People 进程、PostgreSQL 18 双租户、跨租户稳定 ID／Subject 拒绝、撤权、停用、双库空目标恢复和恢复后续写通过；12 个当时既有 Owner 的代表性租户隔离、搜索／分页／导出／附件路径也以当前代码复验。该阶段未部署 D1／公网。[P1 报告](./reports/oms-v1-iteration-p1-multitenant-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 P0 已完成：需求基线提交为 `abe170f`，BF-184～BF-237 已映射到 P0～P9、Owner、migration、API、页面和验收证据，并冻结新应用 ID、数据库、端口、路由和 Capability 命名；秘密扫描无命中。[P0 记录](./reports/oms-v1-iteration-p0-baseline-validation-20261025.md)、[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 BF-237 已确认 Feedback 自动诊断上下文使用严格非敏感 allowlist，不携带身份、业务数据、凭证、完整 UA 或自动截图；本轮 BF-184～BF-237 产品需求已经闭合，开始按 P0～P9 实施，当前先完成 P0 可追溯基线并进入 P1 IAM 双租户审计／实现。[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 已把 BF-184～BF-236 映射为 P0～P9 工程实施分期，覆盖 IAM 双租户、两个新 Owner、成员／薪酬、工资、财务申请、开放 API、发票报销闭环、Feedback V2 和 D1 恢复验收；该准备步骤随后由 BF-237 关闭最后待确认项并进入实施。[实施拆解](./specs/assembly/oms-v1-iteration-implementation-plan-20261025.md)

2026-10-25 BF-236 已确认 Feedback 项目 115 首阶段开启 V2 通知和“产品反馈”未读角标；0 隐藏、读计数不改已读、进入单条只清该条，通知故障不阻塞 OMS。自动诊断上下文随后由 BF-237 确认。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-235 已确认 Feedback V2 首阶段使用跨域 iframe＋`apiKey` 直连并绑定项目 115；Key 仅部署注入且按浏览器可见的项目专用凭证治理，`customUserId` 由 Tenant＋Subject 稳定脱敏派生，反馈事实继续由 Feedback 平台拥有。通知／未读随后由 BF-236 确认。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-234 已确认本轮闭环为管理员授权→员工 Key→CLI／Skill 发票上传与报销提交→无需审批待支付→财务确认→Money→本人 API 已支付；正式验收须覆盖真实入口、幂等、故障恢复、撤权、隔离和文件安全。下一步确认该闭环的实施顺序与阶段验收切片。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-233 已确认开放 API 统一错误结构、HTTP 语义和恢复规则：不可重试错误、429 退避、超时／503 原幂等键重试、`UNKNOWN_OUTCOME` 查询恢复，并清洗所有内部／敏感信息。下一步进入“走通闭环”，确认本轮闭环的起点和完成事实。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-232 已确认草稿不随会话／Key 丢失，90 天未修改自动归档且可恢复；仅从未提交、无付款的草稿可标记删除，提交记录不可删除，归档／删除不级联删文件。下一步确认开放 API 的标准错误模型与调用方恢复指引。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-231 已确认上传授权 15 分钟、会话 24 小时，失败临时对象过期后 24 小时内清理；完成但从未引用的文件保留 7 天，被任一业务 revision 引用后禁止自动物理删除。下一步确认报销草稿自身的过期、归档和删除规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-230 已确认发票文件首版格式与容量：PDF／OFD／XML／JPG／PNG，20 MB／文件、20 文件／单、100 MB／单；Evidence 完成真实类型、哈希和安全扫描，未通过不能提交。下一步确认上传失败、过期和未引用文件的清理规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-229 已确认 API Key 首版不强制 IP 白名单、只允许 HTTPS；IP／CIDR 限制以后作为默认关闭的增强。Key 只能在登录网页中二次确认创建／撤销，开放 API 不提供生命周期写端点。下一步确认发票文件上传格式、大小和安全校验边界。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-228 已确认开放平台 API 使用记录：员工只看本人，持独立能力的租户管理员看本租户；记录最小脱敏元数据并保留 180 天，不保存正文、业务内容或密钥，也不建设统计／计费平台。下一步确认 API Key 创建和调用是否需要额外的网络来源限制。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-227 已确认开放 API 使用 `/open-api/v1/`，交付 OpenAPI、中文说明和 `curl` 示例；V1 只兼容新增，破坏性变化发布新主版本，旧版停服至少提前 180 天公告。下一步确认开放平台是否允许租户管理员查看本租户 API 使用记录。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-226 已确认 Webhook 首版只保留“暂未开放”的设置入口，不接收回调地址、不建立订阅／投递能力；未来按独立安全和可靠性契约启用。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-225 已确认本人提交查询契约：提交返回 ID／revision／状态；列表支持状态、日期和增量查询，稳定游标分页默认 20、最多 100，按更新时间倒序＋ID；支持条件读取，首版不做 Webhook、任意排序和全文搜索。下一步确认开放 API 文档、示例和兼容性发布规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-224 已确认开放 API 必须提供当前租户内员工本人的完整提交记录列表和单笔详情，身份由 Key 派生且不能切换他人；列表覆盖所有状态并稳定分页。下一步确认列表筛选、排序和增量查询契约。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-223 已确认报销提交后冻结，当前无需审批直接进入待支付；无成功／未知付款时可撤回修订，开始付款后不可修改。实际付款由授权财务人员确认并写入 Money，申请人 API 只读本人状态。下一步确认发票报销 API 的状态通知与查询方式。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-222 已确认发票按租户防重：相同文件哈希或发票代码＋号码阻止提交，开票方＋日期＋金额相同仅警告并人工核对；驳回／撤回重提必须关联原单。下一步确认 API 提交成功后的状态、可修改范围和付款闭环起点。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-221 已确认所有开放 API 写请求强制幂等键，按租户＋员工＋动作生效且不绑定具体 Key；相同内容返回原结果，不同内容 `409`，草稿至少保留 7 天、正式提交长期防重。下一步确认发票重复识别和人工处置规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-220 已确认开放 API 普通请求按每 Key 60 次／分钟、每租户 600 次／分钟双层限流，上传采用独立并发限制；超限返回 `429` 和重试时间。认证失败仅临时限制来源，不自动封禁账号或永久撤销 Key。下一步确认发票报销 API 的幂等和重复提交规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-219 已确认 API Key 默认有效期 180 天，可选 30／90／180／365 天，不提供永久 Key；到期失效，轮换只能新建 Key 并替换调用方配置，不能延长或恢复旧秘密。下一步确认开放 API 的调用限流和异常封禁规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-218 已确认每员工每租户最多 5 个有效命名 Key，完整 secret 仅创建时显示且服务端只存 verifier；页面展示前缀和使用元数据，每个 Key 独立、不可逆撤销。下一步确认 Key 有效期和轮换规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-217 已确认 API Key 不单独审批：员工已有发票报销 API 能力即可创建，无能力由租户管理员先授权；Key 每次调用服从账号与当前能力，停用／撤权／撤销后失效，不能扩权。下一步确认每个员工可持有的 Key 数量、展示和独立撤销方式。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-216 已确认开放平台首版只做员工账号／租户绑定的发票报销 API Key 入口：供外部 CLI、Agent Skill 或其他工具上传 Evidence 发票、建立并提交本人报销和查询结果。AI／Skill／CLI 不在 OMS1 开发范围；其他业务 API 后续逐项扩展。下一步确认 API Key 申请是否需要额外管理员审批及签发规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-215 已确认缺少工资账户时仍可生成／核对并持续标记，但正数工资禁止发放；补齐后需手动同步并重新核对，批量不阻断其他合格成员。公司内部管理第一轮核心边界已形成，下一步开始澄清 SDK／API 开放平台与 AI 的首版对象和范围。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-214 已确认已生成工资行可手动同步新账户：仅目标成员无成功／未知发放时，在预览后同步并退回待核对；其他成员已发放不阻止。目标成员已有部分／全部发放时禁止换卡。下一步确认成员缺少有效工资账户时的生成与发放规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-213 已确认第一版工资账户由授权工资经办人维护，成员只读脱敏值；变更需生效月份、原因和二次确认并追加 revision，仅自动影响之后生成的工资表。下一步确认已生成但未发放工资行是否允许手动同步新账户快照。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-212 已确认工资收款账户由 Payroll 薪酬支付档案拥有并挂载 Person ID，版本化、默认脱敏、完整读取／导出分别授权和审计；工资表生成冻结账户快照，后续变化不回写历史。下一步确认支付档案由成员本人还是工资经办人维护。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-211 已确认工资超额发放规则：逐笔强提醒和原因，Money 保存完整实付、仅分配剩余工资，Payroll 显示已发放＋超额异常并分别保存实付／分配／超额。后续扣回调整引用原事实，异常处理前持续可见。下一步确认成员工资收款账户的数据归属与敏感权限。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-210 已确认单成员工资支持分次实际发放：每次独立记录并写入 Money，工资行按累计分配额显示待发放、部分发放或已发放；批量默认剩余金额但逐项幂等，失败项独立续办。下一步确认工资超额发放的异常处理。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-209 已确认零／负实发处理：零实发显式确认无需发放、不写 Money，但成员可查看工资单；负实发禁止付款并转待处理扣回，后续以引用原行的调整处理。下一步确认单个成员工资是否支持分多次实际发放。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-208 已确认工资表作废规则：无成功或未知发放时可带原因作废并保留历史，可新建引用原表的同月替代表；任一付款成功后整表不可作废，只能补发／扣回，未知付款须先查明。下一步确认实发金额为零或负数时的处理规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-207 已确认同租户同月只允许一张有效常规工资表，重复创建返回已有记录；补发／扣回使用可多批的独立调整批次并引用原工资行。下一步确认未发放工资表的作废规则及发生付款后的处理边界。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-206 已确认工资表生成和录入：自动带出固定工资／基数并计算缴费，绩效和调整默认 0 但状态仍待核对；支持表格录入和带预览的稳定成员标识模板导入。显式核对后才进入待发放，后续编辑回到待核对。下一步确认同一月份常规工资表与补发／扣回批次的唯一性规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-205 已确认成员工资单金额构成：展示固定工资、绩效、调整、应发、个人五险一金、个税、实发和实际日期；单位承担五险一金单列企业成本。Payroll 精确计算并保存可追溯方案与 revision，补发／扣回不改写旧单。下一步确认工资表的绩效与调整录入方式。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-204 已确认工资数据权限：成员只查看本人已实际发放的工资行，按成员行判断而非等待整表；工资全量读取、编辑、逐条／批量发放和导出分别授权，部门负责人和租户管理员不自动取得权限。下一步确认成员工资单需要展示的金额构成。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-203 已确认财务申请权限：成员只查看和操作自己的申请；财务全量读取、付款、批量付款和敏感信息能力分别授权；部门负责人和租户管理员不因名称自动获得财务数据权限。下一步确认工资单对成员本人和工资经办人的查看范围。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-202 已确认财务申请可选费用归属：部门默认当前部门但可改选或清空，项目和合同可选；提交冻结稳定引用。第一版只支持展示、筛选和导出，不做统计或产生其他业务效果。下一步确认申请数据的查看范围与财务角色权限。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-201 已确认第一版报销和付款申请均按整单付款与核销；明细只表达金额构成，不单独保存支付状态。逐明细核销延后，未来需要独立版本化分配模型。下一步确认申请与费用承担部门、项目的关联方式。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-200 已确认付款申请支持多条用途明细，由 Owner 汇总总额；整单只能有一个收款方和账户，不同收款方必须拆单。提交后变更走 revision，首笔付款后冻结。下一步确认分次付款是按整张申请核销还是要求逐条用途明细分配。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-199 已确认费用类别字典规则：平台预置基础类别，租户管理员可新增或停用本租户类别；改名追加 revision，历史引用不得删除或改写。报销提交冻结类别标识、名称和版本。下一步确认付款申请采用单一金额还是多条付款用途明细。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-198 已确认报销采用多条费用明细：每条记录日期、类别、精确金额、说明和凭证引用；申请总额由 Owner 精确求和，页面不允许独立填写。提交后修改走撤回与 revision，付款后冻结。下一步确认费用类别采用租户可维护字典还是第一版固定枚举。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-197 已确认财务申请附件规则：草稿均可无附件；报销提交必须引用至少一份 Evidence 已完成保存的票据／费用凭证；付款申请第一版缺附件强提示、显式确认后可提交，并记录缺失事实。附件要求与引用随提交冻结，后续按申请类型扩展策略。下一步确认报销是否采用多条费用明细以及总额计算方式。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-196 已确认收款账号敏感信息规则：列表、普通详情和普通导出仅显示后四位；完整查看与完整导出分别授权给财务能力并逐次审计，普通申请人仍看脱敏值。服务端不能把完整值下发后仅由页面遮挡，完整账号不得进入日志、搜索、URL、埋点或前端持久化。下一步确认报销／付款申请提交时的附件要求。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-195 已确认付款申请收款方采用双轨：优先引用 Party Directory，允许一次性手工录入但不自动创建主体；提交冻结名称、开户行、账号和来源，后续资料变更不回写旧申请。未付款时换账户须撤回修订，疑似重复只提醒。下一步确认银行账户等敏感字段的查看和导出权限。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-194 已确认报销收款人规则：默认申请人本人，允许选择本租户其他内部成员代收但必须说明原因；外部单位或个人付款统一使用付款申请。申请人、收款人和操作人分别留痕，提交时冻结人员快照。下一步确认付款申请的外部收款方与账户快照来源。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-193 已确认财务申请批量付款交互：只选择待支付／部分支付申请，逐笔默认剩余应付并可修改金额和实际日期；提交前预览总额、明细和异常。后端逐笔幂等写入 Money，部分失败保留成功项并只续办失败／未知项。下一步确认两类财务申请的收款人和账户快照规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-192 已确认财务申请金额修订规则：草稿可改，提交后无付款可撤回、追加 revision 并重新提交；未来审批模式下金额变化重新审批。首笔 Money 付款成功后金额冻结，差额通过引用原申请的新申请／调整申请处理。下一步确认财务申请批量付款的交互和部分失败恢复。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-191 已确认内部财务申请支持分次付款：每次外部付款由用户独立确认，经幂等连接器写入 Money 并保存稳定引用；申请按累计分配额显示待支付、部分支付或已支付，单笔失败不影响其他成功付款。超额实付需强提醒和原因，Money 保存完整资金、只核销申请余额并保留未分配超额，申请显示“已支付＋超额异常”。下一步确认申请金额的修订规则。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-190 已确认成员离职／停用采用追加生命周期：People & Groups 保存状态 revision 和生效日期，Payroll 只展示该状态，不把离职与工资生成、补发或付款自动绑定；当期工资包含关系和金额由显式工资操作决定。月中入离职第一版不自动按天折算，由经办人录入带原因的正／负调整；发放后差额只走补发／扣回。重新入职沿用 Person ID 并追加新任职周期和新薪酬 revision。新建工资表默认勾选在职成员，离职成员仍可检索和手动加入；IAM 账号恢复单独授权。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-189 已确认第一版个税采用外部结果录入：Payroll 工资行支持人工录入和稳定成员标识批量导入，预览后幂等追加 revision，并把个税纳入最终实发金额。第一版不实现累计预扣、专项附加扣除、汇算、申报或缴税，页面明确不是自动计税；发放后差额继续通过补发／扣回。自动税务策略延后独立设计。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-188 已确认五险一金方案由 Payroll 正式承接：实现平台标准方案、租户选择和从标准 revision 复制的租户自定义方案；按地区与生效月份版本化比例、上下限、舍入和来源。工资表冻结并展示个人／单位逐项承担额和合计；个人部分减少成员应付，单位部分只增加企业用工成本。正式实现前必须核对权威来源，不能继续把体验版默认值当作全国统一规定；最终实发的个税输入由 BF-189 收口。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-187 已确认成员与薪酬字段的正式归属：People & Groups 接入成员基础信息；Payroll 以稳定 Person ID 将版本化薪酬档案“挂载”到成员，保存固定工资、社保基数和公积金基数；绩效在每月工资行录入。该挂载不能实现成共享表、跨库外键或 People 的任意属性包，三个薪酬字段当前也不拆成三个小系统。每次薪酬变更必须选择生效月份并追加 revision，只自动作用于该月及以后尚未生成的工资表；已生成且完全未发放的工资表允许在预览差额后人工同步。任一成员已发放后整表禁止同步，差额只能作为引用原工资行的补发／扣回调整进入后续工资表。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-186 已确认正式 Payroll 必须保留工资审批与人工发放确认：第一版工资表／批次默认并实际采用“无需审批”，但状态机、API、页面和历史均预留审批位置；正式版本可把审批改为常规路径。计划发薪日不得自动形成已发放，只有外部实际发薪后才能由用户确认，并以 Money 幂等登记成功作为资金事实依据。确认支持逐成员和勾选多人批量处理；部分成功显示“部分发放”，仅续办失败／未知成员，全部完成才显示“已发放”。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 BF-185 已确认公司内部管理中的报销／付款申请需要独立 `Internal Finance Requests` Owner：申请系统拥有草稿、提交、审批占位、历史和 Money 投递状态，Money 继续独占义务与实际付款。报销与付款申请分别配置审批策略；本版本两者均默认并实际采用“无需审批”，但保留完整审批位置并冻结每笔申请提交时的策略快照。正式版本预计以审批为常规路径，具体审批规则后续确认。外部实际付款后仍必须人工确认，并以幂等连接器成功写入 Money 作为“已支付”依据。成员／薪酬正式接入继续逐项澄清。[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)

2026-10-25 OMS1 新一轮迭代需求开始逐项澄清：主线为“账号＋租户系统 → 公司内部管理接入小系统 → SDK／API 开放平台＋AI → 走通闭环 → 反馈 V2 接入”。账号／租户首项已确认采用同域名、同服务、同服务器下的组织级逻辑数据隔离；第一版包含平台创建租户和首个管理员、企业管理员邀请／停用成员及分配角色，用户登录时不选择组织；同一账号加入多个租户和租户切换延后。当前只登记范围与验收方向，未实施或验证的能力不得写成完成。后续会议答案持续更新到[本轮迭代计划](./specs/assembly/oms-v1-iteration-plan-20261025.md)。

2026-10-24 BF-183 已修复并发布：财务“全部类型”选择器不再继承工具栏普通说明文字的自动左边距，保持紧邻搜索框；移动端规则只隐藏普通说明，不隐藏统一选择器。源码为 `codex/company-internal-management@249e957`，公网 Workbench 为 v1.32；完整备份、候选预检、D1 检查、17/17 健康、正式登录、五 Owner 只读和不可变 CSS 复验通过。[发布报告](./reports/oms-finance-filter-toolbar-publication-20261024.md)

2026-10-23 BF-182 已实现并发布：工资和财务批量操作条增加“全选／取消全选”，全选覆盖当前搜索／筛选结果的全部可操作记录而不包含已完成或不可批量处理项；筛选改变时清空旧选择，空选择时批量动作禁用。该交互随公网 Workbench v1.32 发布，正式业务边界保持浏览器体验版不变。[发布报告](./reports/oms-finance-filter-toolbar-publication-20261024.md)

2026-10-23 BF-181 公网体验发布已完成：公司内部管理已由“正式环境失败关闭”调整为登录后可用的浏览器体验版，按 Tenant＋Subject 隔离本地存储并显示不落服务端的边界；`oatest` Unified Workbench v1.31 已完成升级前备份、D1 检查、17/17 健康、正式登录和版本化页面资源复验。下一步由用户体验字段与流程；真实 Owner、数据库、审批、Money、跨设备同步和已发放后的补发仍需独立设计。[发布报告](./reports/oms-company-internal-browser-candidate-publication-20261023.md)

2026-10-23 BF-180 本地候选已实现：独立分支提供成员新增／编辑、五险一金方案、月度工资生成、生成后手动同步新增成员、绩效批量录入、已发放批量确认，以及报销／付款申请的草稿、待支付和已支付流程；三个列表均支持筛选、排序和分页。同步只允许尚未发生任何实际发放的期间，已发放后补发仍待后续设计。交互范围保持不变，发布边界由 BF-181 继续收口；未作后端决定前不得把浏览器模型提升为正式架构。

2026-10-22 BF-179 已完成并发布：九张业务表格的末列统一为 76px 右对齐详情操作列，五类动态数据行共用同一终端单元格规则，空状态不受影响。新增操作列回归 2/2，与金额、分页和主体候选合并的 Node 回归 9/9、Workbench Go test／vet、前端 TypeScript check、206/206 测试、双 bundle build及公网验收单测 2/2 通过；本地浏览器计算样式确认表头末列为 76px、右对齐且右边界与表格一致。源码为 `main@cd84894`，公网 Workbench 为 v1.30；升级前完整备份、D1 检查、17/17 健康、正式登录／五 Owner 只读验收和公网不可变 CSS 复核通过。[发布证据](./reports/oms-phase1-detail-action-column-publication-20261022.md)

2026-10-22 BF-178 已完成并发布：十个金额表头和五类动态金额单元格已收口为专用 `is-money` 语义，统一 132px 最小宽度、同一右边界、排序按钮内边距、nowrap 和 tabular lining figures；项目状态列保持独立。新增金额列静态回归 3/3，本地 Node 回归 10/10、Workbench Go test／vet、前端 TypeScript check、206/206 测试、双 bundle build 和浏览器计算样式复核通过。源码为 `main@37ec7e3`，公网 Workbench 为 v1.29；升级前完整备份、D1 检查、17/17 健康、正式登录／五 Owner 只读验收和公网不可变资源复核通过。[发布证据](./reports/oms-phase1-money-column-alignment-publication-20261022.md)

2026-10-22 BF-177 已完成并发布：Project Workspace 新增 `000014_project_party_candidates.sql`、`GET/POST /api/v1/project-party-candidates` 及 Party revision 精确校验；Workbench 新增主体时以稳定的 Party 命令和候选登记命令分段续办，刷新时并发读取候选，甲方／施工方列表和新建项目／关联施工方选择器共用同一角色投影。源码为 `main@6903934`，公网 Project Workspace／Workbench 为 v1.28；升级前完整备份、migration、D1 检查、17/17 健康及正式 API 读写复验通过。此前残留的中性 Party“施工方1”已通过正式候选命令补登记，最终目录投影为甲方 3、施工方 1。[发布证据](./reports/oms-phase1-preproject-party-candidate-publication-20261022.md)

2026-10-22 BF-176 已完成并发布：撤销 BF-174 对甲方列表的过度关系过滤，恢复尚未关联项目的人工录入甲方可见性；施工方列表继续按活动项目关系过滤。源码为 `main@ee7862d`，公网 Workbench 为 `v0.1.0-test.oms-v1.27@sha256:e7906954de08ab389083d82b5df615730f2d0ca9073bada7b136151f63e2bc68`；备份、候选预检、D1 全栈、17/17 健康、正式登录和五 Owner 只读验收通过，甲方投影恢复为 3 条且业务写入为 0。[发布证据](./reports/oms-phase1-client-directory-visibility-publication-20261022.md)

2026-10-22 BF-175 已完成并发布：旧仿 OA／旧 OMS 已停止公开运行，后端 Owner 保持不变。Gateway 收口旧入口并拒绝旧静态资源；Workbench 把登录从旧 75 KB 立项运行时拆为专用轻量脚本，把附件从旧 2.5 MB OA React bundle 拆为独立 Evidence bundle并保持首次使用按需加载。源码为 `main@6ccad2d`，公网 Gateway／Workbench 为 `v0.1.0-test.oms-v1.26`；升级前备份 `20261022T071636Z`、候选预检、D1 全栈、17/17 健康、匿名／正式登录、旧入口、旧资源、Evidence bundle 与五 Owner 只读验收均通过。下一步继续优化当前 OMS，不再维护旧界面的活动部署路径。[发布证据](./reports/oms-phase1-retired-surfaces-and-evidence-decoupling-publication-20261022.md)

2026-10-22 BF-174 已修复并发布：在用户确认 BF-173 体感不明显后，公网测量定位到首屏仍同步加载 2,512,394 bytes 的附件 React 运行时，单资源约 5.15 秒。该运行时现改为第一次附件／预览动作时按需加载，首屏版本化 OMS 资源合计 234,029 bytes 并启用 immutable 缓存；同时按 Project `client_party`／活动 `contractor-party` 关系拆分甲方和施工方列表。源码提交为 `105b7c1`，公网 Workbench 为 `v0.1.0-test.oms-v1.25@sha256:d9df134e7bcadf70418058bba5d84f4646ae96ae28a509bbb9230883aba7e3f5`，静态资源为 `20261022.5`。备份、候选预检、D1 全栈、17/17 健康和公网正式登录／五 Owner 只读验收通过；当前数据角色投影为甲方 `甲方1`、施工方空集合。下一步由用户实际刷新体验确认；如果仍慢，再基于浏览器网络瀑布继续定位 Owner 响应，不回退数据所有权边界。[发布证据](./reports/oms-phase1-critical-path-and-party-role-publication-20261022.md)

2026-10-22 BF-173 已优化并发布：去除 Owner 只读请求在每次页面刷新前的无条件授权；已有 Capability 先直接读取，只有明确凭证失效时才按 Audience 合并换票并重放一次，写入仍强制先授权。六个来源仍并发启动，Project／Party 就绪后先显示可用页面，Billing／Money／Contract 与逐项目汇总随后补齐；重复刷新合并为一轮。源码提交为 `00c5d6b`，公网 Workbench 为 `v0.1.0-test.oms-v1.24@sha256:015ab2b216b47f40d38ac253f89ce8e27afd3fba0c6d6a3a2098cf0c8b3c1054`，静态资源为 `20261022.4`。升级前备份、候选预检、D1 全栈、17/17 健康和公网正式登录／五 Owner 只读复验通过。当前证据证明请求轮次收敛与渐进可用，尚未完成同一已登录浏览器的前后毫秒基准，下一步由用户实际刷新体验确认，不虚报精确提速比例。[发布证据](./reports/oms-phase1-progressive-refresh-publication-20261022.md)

2026-10-22 BF-172 已修复并发布：项目详情打开时，任一侧栏一级／二级导航都会先统一关闭详情、恢复滚动、清除项目选择并使旧详情请求失效，再显示目标页面；点击当前业务组也不再被详情层截留。源码修复提交为 `149cfff`，公网 Workbench 为 `v0.1.0-test.oms-v1.23@sha256:3b78fdbeeb273723340b50aa509fcca1aa3620919212f7480ddd590cb95cebc1`，静态资源为 `20261022.3`。升级前备份、候选预检、D1 全栈、17/17 健康和公网正式登录／五 Owner 只读复验通过。下一步由用户直接刷新公网或本地页面，打开项目详情后逐项切换侧栏做人工体验确认。[发布证据](./reports/oms-phase1-project-detail-navigation-publication-20261022.md)

2026-10-22 BF-171 已修复：BF-170 清理后新建项目被禁用，根因不是 Contract 容器、权限或用户输入，而是清理误删 Contract／Money 的 Party Directory runtime cutover。两套 Owner 已通过正式 migration／cutover 恢复 `directory` authority，重复执行均为 `changed=false`；D1 `check`、17/17 healthy 和增强后的公网正式登录／五 Owner 只读验收通过。下一步直接继续用户的新项目录入；后续任何 Demo reset 必须把主体 runtime baseline 列为保留项，并以增强后的 `verify-public-oms-v1.py` 作为交付门禁。[修复报告](./reports/oms-phase1-party-runtime-reset-repair-20261022.md)

2026-10-22 BF-170 已执行：在完整备份 `20261022T023910Z` 校验通过后，`oatest` 十二个业务 Owner 的用户录入／业务事实及 Evidence／Contract 本地附件对象已归零；IAM、Grant、schema、部署和本方经营主体保留。数据库逐表实数核验除 Party 基准的 1 条主体／revision／command／audit 外均为 0，附件文件 0，D1 `check`、17/17 长期容器健康和公网正式登录／五 Owner 只读复验通过。下一步直接从空项目、空外部甲方／施工方状态走一遍 OMS 真实录入旅程；若发现流程问题只修复交互或正式 Owner 命令，不回灌旧 Demo 数据。[执行报告](./reports/oms-phase1-demo-full-business-reset-20261022.md)

2026-10-22 BF-169 已完成并发布：切换任一二级页面时，Workbench 现在会对全部二级导航重新计算选中态，而不是只清理当前业务组；三个业务组可继续同时展开，但桌面实测只有当前组及其一个当前二级入口保留路径高亮。非当前导航不再保留 `aria-current`。源码 `5779637` 已构建为 Workbench `v0.1.0-test.oms-v1.22@sha256:d9b11412c72a6d4b9d9a9aa8c8786855e4e8f5219503edb8514554c0966a4f42`，静态资源为 `20261022.2`；备份、预检、D1 检查、17/17 健康及公网登录／五 Owner 只读验收通过。下一步只在用户确认测试数据边界后执行数据重置，不把登录、权限和本方经营主体误删。[发布证据](./reports/oms-phase1-single-current-navigation-publication-20261022.md)

2026-10-22 BF-168 已合并并发布：项目经营、应收回款和应付支付可同时保持展开，点击当前组只切换本组展开状态；分页条数控件使用固定统一触发器，标签与箭头均在边框内。静态资源为 `20261022.1`，公网 Workbench 为 `v0.1.0-test.oms-v1.21@sha256:6dcba5542c68b0c7970c80cee4282fc3560ece7efab197a87ac0e9088c65e423`。备份、预检、D1 检查、17/17 健康和公网只读验收均通过；下一步以 v1.21 为界面基线继续逐项收集真实使用反馈，不回退到旧版布局。

2026-10-21 BF-167 已在 `codex/oms1-mvp-review-ui` 完成本地候选：按 `/mvp-review` 重组 OMS1 的侧栏、面包屑、页面标题、项目摘要、操作栏和表格层级；所有现有列表继续支持排序和分页，筛选改为当前列表内展开，导出沿用当前搜索／筛选／排序并补齐项目、甲方、施工方 CSV，项目详情在桌面不再覆盖主菜单。静态资源版本为 `20261021.2`。下一步只做本地真实数据走查与用户确认；未确认前不合并 `main`、不部署 `oatest`，不修改 Owner、API、schema、Grant 或业务数据。

2026-10-21 BF-166 已完成并发布：项目列表把“应收情况／应付情况”收敛为“应收状态／应付状态”，列表只显示已完成、未完成或金额异常；详细金额继续留在项目详情和专业模块。`main@acce030` 已构建为 Workbench `v0.1.0-test.oms-v1.20@sha256:f78176522965d41a6858ddf6bda7ea9f97332b0dca2b2fece9fe502760f0efc5` 并部署到 `oatest`，静态资源为 `20261021.1`。升级前备份、D1 检查、17/17 容器健康和公网正式登录／五 Owner 只读复验通过；未改 Owner、API、schema、Grant 或数据。

2026-10-21 BF-165 已完成并发布：昨天确认的 OMS 易用性基线已从 `main@99ead97` 构建为 Workbench `v0.1.0-test.oms-v1.19@sha256:6268306eadb7183ddae1da1229e623f5fab3d6fb0c033244b289d24a4629155c` 并部署到 `oatest`，静态资源为 `20261020.5`。升级前备份、候选预检、D1 检查、17/17 容器健康和公网正式登录／五 Owner 只读复验通过；独立的 `codex/oms-interaction-patterns` 试验分支未部署。下一步由用户在公网继续走查，新增反馈继续逐项确认，不把试验分支自动合入主线。

2026-10-19 BF-164 当前修正：把 OMS 活动界面、配置和新写入的合同角色从单客户“资质单位”称谓改为中性的“本方经营主体”；`OMS_OPERATING_PARTY_ID` 成为新配置名，旧变量只兼容既有部署。甲方／施工方列表继续从 Party Directory Owner 分页读取，增加明确无假数据空状态和回归约束，不新增角色表、共享主体库或演示数据注入。完成 Workbench、D1 配置、文档、测试和本地 18182 复验后提交交付仓库；公网发布只有在实际执行后才能登记。

2026-10-19 BF-163 已完成客户交付仓库整理：OMS 一期工作台进入 `frontend/oms-workbench`，Gateway／IAM 进入 `backend/platform`，十二个业务服务进入 `backend/services`；`contracts/` 与 `packages/` 为兼容稳定契约和 Go module 路径继续保留在根目录。所有活动构建、验收和部署脚本已经更新为新路径，并以完整 Go／前端测试、架构边界审计、发布包测试和代表性容器构建作为提交门禁。本轮不修改产品功能、Owner、API、schema、Grant、部署拓扑或公网环境。

2026-10-19 BF-162 已完成并发布：修复当前资质单位被选为项目外部甲方后，Billing 拒绝自开自收票据而页面只显示 `INVALID_REQUEST` 的问题。Workbench 已在项目、施工方、票据和收付款入口统一过滤并提交前校验资质单位 ID，同时补齐中文错误解释。公网 Workbench 为 `v0.1.0-test.oms-v1.18@sha256:2b2e319dfacbc6b84b743f5e3a9dcf09846d37c99f7de8523eba8990adf3f54f`；升级前备份、D1 检查和 17 容器健康通过。下一步由用户以真实甲方重新建立当前错误项目并继续开票流程；如以后确需更正已建项目的甲方，应另行设计 Project 追加 revision 能力，不用直接改库代替。

2026-10-18 BF-161 已完成并发布：清理前建立全量备份 `20261018T041742Z`，随后将 OMS 使用的 Contract、Money、Party、Project、Evidence、Billing 六个 Owner 恢复到首笔 OMS 数据前的 `20261012T175912Z` 检查点，并重新建立 Contract／Money 到 Party Directory 的 runtime 映射。Workbench 增加 `operations` 项目范围及关联票款过滤，发布为 `v0.1.0-test.oms-v1.17@sha256:da5f4626cb53a9d8ca6758f2c6852468974221fc4ca440264de4a62a42d5dbfe`。当前 OMS 项目为 0，账号、权限、资质单位和旧 OA 数据保留；下一步由用户从空白 OMS 重新录入并走查一期流程，不扩展为产品内跨库批量删除功能。

2026-10-17 BF-160 已完成并发布：修复 OMS 读取在短期 Capability 过期后把浏览器 `opaqueredirect/status=0` 误判为普通成功响应、继而把各 Owner 投影清空的问题。业务与 Evidence 读取现在按目标 Audience 清除授权缓存、重新换票并只重读一次；CSS／JS 查询版本递增到 `20261017.3`。公网 Workbench 为 `v0.1.0-test.oms-v1.16@sha256:9a082f08dba5897e5cdba3698e855a6f9e842d772484a63485c68c7f64c567e5`，升级前备份、D1 检查和 17 容器健康通过；只读验收复读 30 个 Project 及其 Party／Billing／Money／Contract 来源。下一步继续收集实际操作反馈，不把续期处理扩展为写入重放、普通拒绝重试或客户端长期授权。

2026-10-17 BF-159 已完成并发布：Gateway 根入口、登录默认回跳与 `/oms-v1/` 统一进入 OMS 一期；Workbench 对 OMS 使用服务端首屏并裁掉旧 OA 侧栏、`workbench.js` 和无关运行时。原项目立项、项目管理、资料、统计及 OMS 参考版收进 `/legacy/`，旧版保留返回 OMS 的显式入口。Gateway／Workbench `v0.1.0-test.oms-v1.14` 已发布，备份、D1 `check`、17 容器健康和 Chrome 根域名往返验证通过。下一步继续优化 OMS 本身，不再向当前主入口重新铺开历史模块。

2026-10-17 BF-158 已完成并发布：OMS 的 Gateway 授权前置已从首次 HTML 退出表单令牌切换为共享 CSRF Provider，按当前 Cookie／配置 IAM 路由刷新并核对，失败即保留表单且不发送 Owner 命令。Workbench `v0.1.0-test.oms-v1.13@sha256:3215e21d963f4f978f036ecad2e1f36d437d6680035ca0cde8097b0b9eb82b80` 已发布；备份、D1 `check`、17 容器健康、静态制品和 Chrome 真实项目创建均通过。“微信4123”已以 `¥1,243,123.00` 应收完成 Project → Contract → Project Link，并在详情复读。下一步回到一期实际操作反馈，不扩展 CSRF 为业务自动重试，也不新增 Owner 或共享状态。

2026-10-14 BF-157 已完成并发布：修复 OMS 页面长期开启后的短期 Capability 失效，所有变更命令发送前刷新应用凭证，读取仅对 Gateway 明确的入口凭证失效做一次有界恢复；甲方／施工方管理改为显示全部可选 Party 并标注项目使用情况；项目详情和保存反馈补齐票款笔数、最近业务日期和更新后累计值，CSS／JS 资源版本随发布递增。公网已真实复核既有甲方可见、新增施工方成功以及“微信1111”开票／回款投影，Workbench 为 `v0.1.0-test.oms-v1.12@sha256:a9759a92300718a0851c20a619406992a16e5fdfa198315fcb7de9e12c2f0f31`。下一步继续按实际反馈检查一期表单与明细可读性，不新增角色表、项目内流水列表或任何业务 Owner。

2026-10-14 BF-156 已完成并发布：盘点全部 OMS 右侧抽屉和页面内 `select`，把应收／应付、回款／支付、票据状态和合同详情收敛到同一渐进层级；筛选、分页、新建项目、票据、资金、主体、合同切换等动态选择器全部复用同一白色弹层，并完成触发器锚定、视口碰撞、键盘和禁用状态处理。公网 Workbench 为 `v0.1.0-test.oms-v1.10@sha256:30e4a2a4b7e418eefe8f678cced85e283dff53d50494b58807a672d3d76cf112`。下一步继续在不改变 Owner 边界的前提下精进一期业务页面，不为展示统一顺手增加新字段、命令或数据库结构。

2026-10-13 BF-155 已完成并发布：OMS 一期统一收窄桌面侧栏、放宽普通工作区与项目详情，并把列表正文、表头、辅助说明、金额链、详情字段、文件行和抽屉表单提升到可读字号；顶栏、标题和筛选区未增高。公网 Workbench 为 `v0.1.0-test.oms-v1.9@sha256:fd14c22d71694a17c8bb3a822afa49dbb794f9dfa51c4e007c3226a880f6ef38`。下一步以实际使用反馈微调列宽和信息优先级，不再整体缩小字体换取“塞得下”。

2026-10-13 BF-154 已完成并发布：项目详情已按业务板块重排，顶部工具栏收口为返回；编辑项目、合同详情、关联施工方和设置应付分别进入基本信息、甲方业务和施工方业务标题区。项目说明与弱化元数据并入基本信息，资金链与逐施工方分账保持原 Owner 只读投影。公网 Workbench 为 `v0.1.0-test.oms-v1.8@sha256:968877897398e209b4af218602b65a4a9cb1c606b7a04b5c69117a943a99da28`。下一步先收集实际评审反馈，不为视觉完整继续增加项目页动作。

2026-10-13 BF-153 已完成并发布：统一附件行、预览层、PDF／图片展示、图片缩放、下载降级、Esc／关闭返回原详情和临时 URL 回收已经覆盖合同、双向票据、回款、支付四类详情。公网 Workbench 为 `v0.1.0-test.oms-v1.7@sha256:69fc8def18092af7bf0b45ba2be394d02d51881b109fdbe7b5ac0e4b1c9a707f`。下一步不扩展 Office 转换或 OCR；只有真实使用确认必须在线查看这些格式后，才单独设计受控转换服务、安全隔离和缓存生命周期。

2026-10-13 BF-152 已完成并发布：项目详情已补齐产品设计要求的完整只读经营投影，包括项目身份、甲方四段金额链、施工方四段金额链、逐施工方折叠分账、Contract 金额变更、Project revision 历史和项目说明。Billing 金额与笔数使用精确整数分聚合，Money 继续提供实际收付与余额；页面不复制事实，也不在项目页重复实现票据／资金新增。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.5`，OMS 静态资源附带发布版本标识，旧标签页重新进入后不再沿用上一版缓存。下一步只根据实际业务评审修正字段密度和术语，不把具体流水、审批或内部成本顺手塞回项目详情。

2026-10-13 BF-151 已完成并发布：项目详情中的合同入口升级为渐进式“合同详情”，由 Contract Ledger 原生数据先展示合同对象、状态、精确金额、往来主体和日期，再展示 Evidence 附件；草稿合同可进入独立编辑表单追加版本。唯一上游合同金额低于项目已回款、或唯一“项目＋施工方”下游合同金额低于已支付时，输入态红色提示并在保存前二次确认；多合同且资金不能可靠归属时不猜测。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.3`，不含数据库、schema、Grant 或其他 Owner 镜像变更。下一步由业务评审确认合同详情的最小字段是否足够；没有新结论前不继续增加合同编号、结算规则或票款强绑定。

2026-10-13 OMS 一期六 Owner 接入已按 BF-150 关闭：Party、Project、Contract、Billing、Money、Evidence 的公网真实写入、Owner 复读、完整分页、筛选和附件字节校验均通过，发布闭包为 `v0.1.0-test.oms-v1.2`。下一阶段不再横向接入与一期无关的小系统；只在明确的新业务目标下进入撤权／局部故障恢复、无固定期限义务、内部成本或审批等独立里程碑，并继续保持现有 Owner 和追加历史边界。

2026-10-12 BF-149：OMS 一期公网发布步骤已完成，不再把恢复 SSH、备份、升级或 `/oms-v1/` 复读列为下一步。当前最近功能顺序调整为：先完成 Party runtime cutover 下的 Money 实际回款／支付安全写入与可恢复验收，再补施工方合同／应付纵向旅程和 Evidence 附件，随后补 Owner 级全量筛选分页／原生导出、撤权／故障矩阵和发布后人工浏览器逐按钮验收。每一步继续由原 Owner 写入，Workbench 只做有界编排与投影。

2026-10-12 BF-148：OMS 一期已经从页面接线推进到本地正式组合。新建项目按 `Project → Contract → Project Link` 三段原生命令有界编排；Contract 金额回读进入项目与应收投影；Billing 开票在真实 IAM 下完成保存和复读，草稿 Project 使用 Billing 既有人工确认来源快照而不修改 Owner 生命周期。正式 D1 `check`、发布包自测、包内校验和、Workbench 测试均通过，15 应用＋4 基础镜像已经以 `v0.1.0-test.oms-v1.1` 的 `linux/amd64` 不可变摘要推送。下一步只做 Main 现场发布：恢复受控主机入口后，使用现有 `/opt/omsbizfoundry/acceptance/oa-public-release-20261002k/d1-active` 和 HTTPS 环境先备份再升级，随后验证 `/oms-v1/`、登录、Owner 读取／写入和旧 `/oms/` 兼容。SSH 未恢复、公共 URL 仍为 404 时不得宣称上线。

2026-10-12 BF-147：OMS 一期项目表单补齐甲方前置建档路径。实现仅包含统一浅色选择器，以及从“创建甲方”关闭项目表单、切换到甲方管理并聚焦新增入口；不在该路径提交 Party 或 Project，不加入名称自动匹配、跨应用双写或半成品恢复逻辑。验证覆盖浏览器实际样式、导航落点、焦点和 console。

2026-10-12 BF-146：OMS 一期导航收口为“侧栏一级＋当前组二级”。实现范围仅为 Workbench HTML／CSS／JS、响应式导航和相应测试；内容区同名 Tab 删除，Owner 接口、事实模型、IAM、Gateway 路由和数据库保持不变。验证覆盖桌面三个分组逐项切换、组间展开收起、总览跳转以及移动端一级／二级连续切换。

2026-10-12 BF-145 实现进展：能力映射、Gateway `/oms-v1/` 入口、Workbench 四入口响应式工作面、Project／Party／Billing 读取适配，以及 Money 自有受 IAM 保护的跨项目资金列表端口已落地。Project 进一步把施工方实现为精确 Party revision 的 `contractor-party` Resource Link，受授权的项目目录返回当前有效关系；Party 原生主体创建、Party 当前档案 revision 更正、Project 原生单施工方关系写入和 Project 基本信息编辑也已接到统一抽屉。应收／应付不再是静态占位：Money reference 显式标记往来 Party 角色，Project commercial summary 瞬时透传主体快照和 Link 时间，Workbench 只显示已稳定关联 obligation 的总额／已核销／余额并按关系时间倒序。Billing 双方向搜索、详情、新增和 correction revision 已接入原生命令；资质单位由规范 `OMS_QUALIFICATION_PARTY_ID` 显式配置，Party／Project revision 精确冻结，未配置、停用或无相应能力时失败关闭。经营总览已用 Owner 投影绘制余额圆环、月度收支和项目回款条；六类资金列表已能把当前筛选后的已加载记录导出为 Excel 兼容 CSV，不受 UI 分页限制且明确不是全库导出。上述未新增表或 migration；单元、HTTP、PG18 目录集成、脚本和本地浏览器门禁检查通过，但尚未在正式 IAM／公网执行真实成功写入。下一步依次是：在部署环境配置真实资质单位 Party ID 与两档 IAM Grant；完成 Project＋Contract 可恢复项目创建和 Contract 项目方向／金额只读投影；在已登记的主体 runtime cutover 门禁内接通 Money 安全写入；补 Owner 级筛选分页／原生导出和 Evidence 附件；最后发布到 `oatest.antuinfo.cn/oms-v1/` 并执行登录／撤权／故障和公网复读。未完成项不得由页面演示值冒充。

2026-10-11 BF-145 当前执行顺序：先形成 OMS 一期页面到 Project／Party／Contract／Billing／Money／Evidence／IAM 的能力映射与改动分级，再新增 Gateway `/oms-v1/` 受保护入口和 Workbench 一期工作面，随后按项目与主体 → 发票 → 实际资金 → 汇总图表／导出／附件完成纵向闭环。公网必须经过 Nginx、Gateway 和 IAM；旧 `/oms/` 参考 Demo 保留。Workbench 只能编排与投影，若需要共享表、跨库写、双写或大规模改变既有小系统语义，先记入边界演变账并停止隐式实现。本轮连续工作上限八小时，到时保存可复验状态并等待用户检查。

2026-10-11 BF-144 当前 OMS 一期 MVP 不实施期限、逾期、催收和自动提醒，只登记实际开票／收票／回款／支付并计算余额与超额异常。Money Flow 现有固定到期日账龄投影保持不变且不强制装入 OMS；后续待开发项单列为：可表达“无固定期限／有明确期限依据”的兼容义务输入、追加式跟进事实、可选提醒／通知及相应投影。上述待办开始前必须取得真实使用规则并设计旧 `due_on`、API、历史、导出和恢复兼容，当前不改 schema。第一版保留登录：预置本地管理员或后台／既有 IAM 注册在会议后选择；不得实现 `AUTH_DISABLED` 或为真实数据自动注入匿名管理员。省略 Nginx 只可作为本地／受控验证形态，不能据此声明完整公网部署通过。

2026-10-11 BF-143 当前 OMS 一期优化顺序：停止继续横向增加一级模块，先把分批回款做成产品最清楚、最可操作的核心循环。第一步修正演示语义：没有合同固定期限或其他可靠依据时只显示“未回／待跟进／承诺待兑现”，不得笼统标成逾期；第二步在单事项经营全貌中用一个回款进程展示履约／完工依据、开票、应收、每笔实际到账、核销后余额、最近回款和下一步跟进，默认层只保留少量关键数字，批次和依据按需展开；第三步提供浏览器内模拟的“登记一笔回款／记录一次跟进”动作，证明多次循环后余额和待办正确变化；第四步在收付台账增加跨事项的未回、待跟进、承诺待兑现和已结清工作面，并能返回原事项；第五步用一笔多批在途和一笔已结清案例在桌面／手机复核，再以同样的信息层级收敛下游付款链。此轮只改验证原型和规格，不提前修改 Money Flow schema、正式 API、IAM、数据库或部署。

2026-10-11 BF-142 一期范围固定并启动行业缺口审计：当前不实现内部成本或统一财务报告，先以官方结算办法、合同示范文本、履约验收、分包／资质、工资专户和公路养护资质资料检查现有 OMS 原型。第一轮已保持四个一级工作区不变，在经营事项内补入“合同边界 → 履约／计量 → 收付循环”，把“预算与结算”修订为“履约与结算”，并同步补充台账、协作资料和 Excel 导出。下一轮继续验证付款节点类型、变更调整、质保／担保、下游付款路径、异常更正和历史导入；这些先作为可推翻候选，不自动写成客户规则，也不以增加首屏字段完成调研任务。

2026-10-11 BF-141 页面内二级工作区已完成：经营事项增加事项列表、经营全貌、合同与资料、协作方与人员、预算与结算五个可切换页面；收付台账将原泛化业务类型改为完整的上游三项和下游五项，并把付款条件与付款授权分别展示。下一轮评审重点不再问“入口有没有”，而是观察经办人能否在事项内部找到资料、协作和结算，财务能否不进入事项就完成横向定位；没有真实反馈前不把二级项继续升级为侧栏入口，也不照搬人员款项、智能材料等未确认模块。

2026-10-11 BF-140 四工作区信息架构已完成本地验证实现：保留 BF-139 的单事项经营图和分组台账，新增经营总览、跨事项收付台账和待办事项；桌面侧栏与手机底栏均固定四个入口。收付台账提供上游／下游、业务类型、经营事项和状态筛选，并从同一演示事实进入依据详情或返回事项。下一评审只验证四类用户能否快速选择正确入口、财务是否仍需要更直接的批量动作，以及“收付台账”命名是否清楚；未获得真实业务反馈前不增加会计科目、正式权限或更多侧栏模块。

2026-10-11 BF-139 台账分组与关系命名修正：默认台账增加上游收款／下游履约与付款两条组带、方向快筛和分层最近记录；页面不再逐行重复方向小标签。Excel 同步使用两种浅色区分方向，并保留事项编号、事项名称、方向、状态和日期的原生筛选。“业务链”改为“收付关系”，关系页首句明确箭头只表示先后、各事实分别登记。下一评审验证长列表中能否快速定位方向和风险，以及客户是否理解该视图不是自动流程。

2026-10-11 BF-138 图表下方可导出台账已完成并取代 BF-137：默认概览直接呈现两图、七行五列表格和闭环条件，长文本明细单与重复副标题已删除；“图表／明细”重命名为“概览／业务链”，双轨只在次级视图出现。下载入口提供真正的 `.xlsx`，包含一张可筛选经营台账、十列和七行业务数据；累计值为数值、统计日期为日期。下一评审只验证表格列是否足够用于实际经营判断和导出，不再通过增加解释性文字解决业务理解问题。

2026-10-11 BF-137 经营明细单已完成：明细模式在既有合同与双轨下新增文档式说明，桌面用两张五列表格展示上游三环节、下游四环节，手机转为逐项带标签的纵向文本；经营结论、当前处理重点和 Demo 边界保持显式。表格每行可以打开对应依据抽屉，默认图表仍保持 BF-136 的极简状态。下一评审比较业务人员是否能从图表快速判断、再从明细准确复述事实关系；未取得反馈前不继续增加更多报表或字段。

2026-10-11 BF-136 经营观测层已完成：默认图表移除重复的当前重点、合同起点和上下游六个阶段金额，只保留两张完成度图、四个实际／缺口金额和三项闭环条件；“明细”仍完整恢复重点、合同、双轨、阶段与节点依据。桌面并排和 390×844 手机纵排均已复核。下一评审只验证用户能否先在数秒内看懂收付现状，再主动进入明细解释原因；未获得反馈前不增加第三张图、更多摘要数字或新的业务功能。

2026-10-11 BF-135 图表模式已完成并取代 BF-134：页面默认显示 69% 上游收款完成度和 45% 下游付款完成度，两图分别标明口径、实际金额、剩余金额和单一风险；各自下方只保留三阶段金额。图表／明细、桌面／手机和图表进入依据均已复核，误加的图标模式及其资源已经移除。下一评审重点是业务人员能否在数秒内复述两项完成度、两项缺口和风险，不继续增加图表数量。

2026-10-11 BF-134 图标模式已完成：层级新版默认显示合同图标和上下游八个业务图标，每个节点仅保留名称及核心数字；右上角可无损切回明细，图标本身继续打开原节点详情。手机端保持单轨切换，图标以两列排列。下一评审只需比较两种模式能否分别承担“快速扫读”和“核对依据”，不再通过继续堆字段解决理解问题。

2026-10-11 BF-133 层级新版对照 Demo 已完成：既有 0.2.10 页面不改，新入口独立展示事项身份、单一经营重点、合同起点、上下游双轨和闭环条件。笔记本 1366×768 首屏已能看见完整双轨与闭环起点；手机 390×844 自动收起侧栏，通过“上游收款／下游履约”切换单轨，并以底部详情承载节点记录。下一步先由用户和业务方对照两版判断：是否能在数秒内说出当前卡点、两条轨各自进度和闭环缺口；未取得判断前不继续改写旧版结构或扩展业务功能。

2026-10-10 BF-132 六个核心小目标已完成：0.2.10 已在原有“待办事项、经营事项”两入口内加入独立开票／应收、独立结算／应付、合同版本与资料补充、协作方经营档案、追加更正和条件归档；东区事项可以在资金余额归零后补齐第六份资料并完成归档，北区可以新增开票／应收与结算／应付而不自动推动相邻事实，并能冲销回款且保留原历史。下一步停止扩功能，直接用该原型向客户走查一笔在途业务和一笔已完成业务，判断对象、名词、关系、闭环门槛和异常处理是否成立；未取得反馈前不新增后端、IAM、文件上传或审批引擎。

2026-10-10 BF-131 可执行待办小目标已完成：0.2.4 初始显示回款风险、付款判断、资料补齐三类待办，每项都有时点、原因和单一动作。回款风险可直接打开北区跟进表单；记录 40 万预计到账后，队列重排为继续跟进 20 万、10-25 核对 40 万到账和判断 96 万付款条件；付款判断暂缓后自动变为 10-18 付款复核；东区资料待办直接定位到“最终归档说明”缺口。下一轮客户评审重点确认谁每天使用待办、真实优先级、截止时间来源、逾期升级和跨人转交，不先建设通用流程或通知平台。

2026-10-10 BF-130 付款条件判断小目标已完成：0.2.3 可对北区 96 万应付独立记录“已核对可授权”或“暂缓至 10-18 复核”。判断通过后页面只开放付款授权，授权后才开放实际付款；完成 96 万实付后实际付款从 120 万变为 216 万、未付从 148 万变为 52 万，并回到下一轮 52 万付款条件判断。暂缓分支不改变任何资金数值，并同步复核待办、应付台账和预计现金流。下一轮客户评审重点确认真实付款条件、证据、判断人、授权人、复核和例外处理，不先建设通用审批引擎。

2026-10-10 BF-129 异常跟进小目标已完成：0.2.2 可把北区 60 万逾期中的 40 万记录为预计 10-25 到账，立即显示 40 万已安排、20 万未安排，且实际回款／付款保持 220／120 万；后续单独登记 40 万到账才把计划标为已兑现并把逾期降为 20 万。下一轮客户评审重点确认承诺依据、责任人、预计日期变更、失约、部分兑现和多次跟进的真实处理方式，不先做通用催收模块。

2026-10-10 BF-128 最小可操作验证已完成：0.2.1 可在北区事项依次模拟登记 40 万回款、授权 96 万付款、登记 96 万实际付款；每一步都独立留痕并驱动余额、待办和台账变化。下一轮评审不再讨论“页面是否能点”，而是让客户指出三个动作的真实经办人、依据、必填材料、可撤回／冲销方式和异常场景；没有真实样本前不把模拟表单扩成后端命令或正式审批流。

2026-10-10 BF-127 收付循环修正：单事项第一详情由“业务全景”调整为“收付循环”，在两条独立事实链之前直接显示本轮未回和未付缺口，以及查看缺口→推进动作→登记事实→核销余额→下一轮待办的控制循环；开票节点明确为资质单位向上游开票。下一次评审重点验证客户是否确实按循环管理多轮开票、回款、结算和付款，以及循环批次、结束条件和上下游资金条件，而不是据此先开发自动联动。

2026-10-10 BF-126 最终 0.2 信息架构：侧栏固定为待办事项、经营事项两个入口；经营事项内使用经营总览、全部事项、协作资料三级二级导航，单事项继续进入业务全景、资料待办、变更历史。下一轮评审先观察用户能否从待办或总览顺利找到一笔事项，再从事项定位协作、台账和资料；不再因为评审者提及某个名词就把它升级成侧栏模块。该修正不新增真实后端或业务规则。

2026-10-10 BF-125 信息架构修正：0.2 一级入口按待办事项、经营台账、协作单位收敛，不把评审中出现的台账纵览、施工队管理和模板下载逐项升级成侧栏模块。经营台账内保留四类总账视角、事项列表和单事项双链；协作单位内保留施工单位／班组档案、范围、预算、名册、应付款及四份可下载 CSV 模板。下一轮客户评审重点判断三个工作对象是否成立以及对象内层级是否易找，不以侧栏入口越少越好作为独立目标。

2026-10-10 BF-124 快速验证切片：本地 OMS 0.2 已补成可完整走查的产品表象，三个一级入口、两笔事项、事项双链、资料待办、变更历史和模拟回款均可进入。明日评审只回答“这是否像一个值得继续验证的产品”和“主链是否看得懂”；现场优先记录客户对名词、缺失节点、异常场景和入口层级的确认／否定。真实录入、附件、审批、检索导出、持久化、IAM、Owner API 和公网部署不在本切片内，收到反馈后再决定 0.3，而不是先扩大功能面。

2026-10-10 BF-123 快速验证切片：OMS 参考页已在本地收敛为四个核心视角，用同一份 800 万元虚构合同展示合同、附件、开票、应收、回款、未收、下游候选类型、应付、授权与实付的显式关系。下一步优先让用户和客户实际浏览并指出概念错误；在取得真实业务样本前，不继续扩完整流程、自动计算、付款规则或真实后端。公网 `oatest /oms/` 更新仍须取得 D1 运维入口，不以本地验证冒充已部署。

2026-10-08 BF-122 插入的评审切片：OMS M0 资质单位参考 Demo 已完成本地交互验证并部署到 `oatest /oms/`。下一步不是直接建设真实 OMS 后端，而是由用户实际浏览五个工作区，确认保留／删除／改名的概念、正式上下游角色和第一条真实纵向旅程；在这些结论形成前，不新增 OMS 数据库、业务命令、IAM Capability 或跨应用同步。OA 的 BF-120／BF-121 主线状态不由本切片改变。

2026-10-07 BF-121 用户调序：当前先完成统计工作面和权限配置四层展示，项目详情问题延后。随后以本轮本地实现、浏览器夹具和来源导出矩阵明确候选范围，再进入真实 IAM／来源数据验收；正式模板／临时授权写端口另立纵向闭环。R0／C1～C8 整体审计仍保留，不因 UI 完成而关闭门禁。见[规格](./specs/assembly/oa-statistics-permission-ui-v1.md)。

2026-10-06 BF-120 重排：当前不从 BF-112～BF-118 的旧中断点继续新增页面。先执行 R0 工作树／证据基线收敛，输出 C1～C8 缺口矩阵；再对 C1～C6 只补真实缺口，按 HTTP／IAM→真实来源组合→页面→D1／恢复→Money 建议补全 C7，之后完成 C8 资料／统计／个人工作、最小与完整组合、Customer Overlay、真实环境和最终报告。旧实验代码与报告保留为候选，未经复验不得计为完成。完整安排见[当前基线与下一步](./status/current-baseline-and-next-actions-20261006.md)。

2026-10-06 BF-119 执行规则：当前完整 OA 完成度低，允许并预期边界在真实操作验证中频繁调整，但每个纵向切片除完成功能外都必须更新[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)，记录改变的 Owner、公开契约／Capability／migration、强制依赖／同步调用链、旧场景回归和 Base／Overlay 归属。连续切片应表现为变化频率和范围总体下降、复用增加、稳定窗口变长；否则暂停扩页面并重新判断边界。Project、Production、Contract 整体仍为候选，不把局部闭环或静态隔离记成成熟。

2026-10-06 BF-118 主路径完成：原 OA 对外委托列表、统计、抽屉、文件、评分、纠正、应付建议、历史和撤回已接 Cost／Party／Contract／Evidence／Money 原生边界，私有 10／10、精确工作负载 Scope、缓存、正式 IAM 撤权和八库恢复通过。Cost 仅增加可选发生时联系人快照；Project 零写、Production 零依赖、Contract 仅精确读。扩展权限／停机矩阵和真实实付仍开放。下一纵向闭环进入资料管理 F01／F02，再进入统计 S01～S03：复用原页面，展开才读取，下载地址与下载记录分开，所有金额和文件投影来自来源 Owner，Project／Workbench 不建第二份统计真相。

2026-10-06 BF-117 完成：原 OA 批复／审计双列已接 Results 原生能力，批复、审计、文件、三项金额、纠正、撤回重登记和历史在私有 11 场景闭环；补测修正空 Evidence 集合输出契约。Project／Contract／People 零业务写，Production 不在链路；本批没有执行 Project Completion Checkpoint。下一纵向闭环进入对外委托：先复用原列表、统计和抽屉，再把委托／评分归 Cost & Outsourcing、主体归 Party、合同／文件归 Contract／Evidence、应付及实付归 Money；禁止照搬原单体删除重建支付或 Workbench 双写。资料统计随后推进，完整 OA 仍不关闭。

2026-10-06 BF-116 完成：四类型生产总览已经从页面壳升级为 Production owner-native 只读投影；原 OA 的类型进度、阶段资料和快捷入口由同一组 v6 事实生成。私有 10 场景证明一次读取、页面缓存、显式刷新、撤权、PG18 和五所有者恢复，且 Project／Evidence／Contract 普通读取为 0、业务零写。下一纵向闭环进入 Results 批复／审计：先按原组件拆解来源选择、批复、审计冻结、纠正／撤回、权限和 Project 完成检查点，再复用独立 Results Owner；不得让 Workbench 或 Project 复制 Results 事实。外委和资料统计随后独立推进，完整 OA 仍不关闭。

2026-10-06 BF-115 当前增量：试验／报告 v6 已按 Project 精确类型 revision 冻结 Production 适用性快照，并通过私有 9 场景与五所有者恢复；没有新增 Contract 边或日常 Project 同步依赖。下一纵向闭环改为“四类型生产总览／进度”：先逐项比对原 OA 的完成口径，再由 Production 的 Work Item、Stage、active material 与 ending 投影，只读 UI 必须支持 Skeleton／缓存／刷新且不重复授权风暴；禁止 Workbench 保存进度、禁止按 Tab 是否可见猜完成、禁止为了总览跨库读取。该闭环完成前不把 Production 整行或 OA 生产域标为完成；Approval／Audit、外委和资料统计继续独立排队。

2026-10-06 BF-114 完成：原变更／竣工双列表和两套上传抽屉接真实 Production／Evidence；显式 v5、一次 4→5、可选 change／必需 completion、不可变替换、IAM 撤权与五所有者恢复在 11 场景通过。过程中修复数据库阶段约束与应用注册表没有随定义闭合两项真实缺陷。Project 无变化、Contract 不依赖，Production 整体仍为候选。下一纵向功能进入原 OA 勘察“试验”和咨询／勘察／测绘“报告”：先固定适用类型、文件集合、完成映射和边界，再施工，不把 v5 变成通用流程。[规格](./specs/assembly/oa-production-change-completion-v1.md)、[报告](./reports/oa-change-completion-native-validation-20261006.md)。

2026-10-06 BF-113 完成：施工图原组件接真实 Production／Evidence，显式 v4 和同一 Work Item 3→4（旧 v2 可顺序 2→3→4）已通过 171 项前端测试、后端／Workbench 测试、构建、静态审计，以及私有 10 场景和五所有者恢复。阶段资料集合子边界现有方案、初设、施工图三份同类证据，标记为可重复；Production 整体仍为候选。下一纵向功能改为原 OA “变更／竣工”，先冻结两套列表、文件类型、替换／删除及完成映射，不套现有校审／评分表单。[规格](./specs/assembly/oa-production-construction-stage-v1.md)、[报告](./reports/oa-construction-stage-native-validation-20261006.md)。

2026-10-06 BF-112 初步设计私有候选完成：原初设面板／抽屉已经接真实 Production／Evidence／正式 IAM，v2 方案定义保持不变，第一次初设保存显式升级同一 Work Item 到 v3。9 个浏览器／恢复场景、Production Go、169 项 Node、TS／build 和静态边界审计通过；Main／公网、正式 D1、全角色和完整 OA 未关闭。阶段资料子边界获得同类第二场景证据，Production 整体仍为候选。下一纵向切片是施工图设计：先复核原字段、提交和完成规则，再决定复用或新增版本，不先复制初设实现；同步更新[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-05 BF-110 阶段导航修复已发布：生产 Tab 的 UI 可用性不再等同于原生 Owner 端口完成度，初设／施工图／变更竣工均可进入原 OA 面板，缺少事实接线时显示真实未读取状态且零业务写入；切换未接通页不再请求通用生产投影。公网 10 个生产 Tab 全部可点击，17／17 healthy，事实与 Grant 不变。[报告](./reports/oa-production-stage-navigation-validation-20261005.md)。下一主线仍是逐阶段实现真实 Production＋Evidence 端口、权限与恢复；不可把本次页面开放记成业务闭环完成。

2026-10-05 BF-109 OA 管理员全权限已实施：当前 Main 12 份已安装 Capability Manifest 已增长到 145 项；`admin` 首轮从83项补齐后，本轮再次对齐为145／145、缺口0，12个 Audience Credential 与 Manifest 逐项一致。OA规则页另为7组49／49；Gateway入场、IAM用户Grant、Workload Policy与目标应用最终授权继续分层。公网已真实打开外委新增和奖金分配抽屉，奖金候选为200空集；17健康且业务事实／Grant摘要不变。后续每次增加应用或 Action，发布闭包必须重跑管理员 Catalog 对齐；不能以 IAM 身份管理员标志绕过业务能力，也不能因全权限跳过目标应用状态／revision／所有权校验。见[报告](./reports/oa-admin-full-access-validation-20261005.md)。

2026-10-05 BF-108 页面读取体验基线已发布：OA 现有视觉／组件保持不重设计，首次读取用 Skeleton，30 秒内返回已读栏目零网络，过期内容保留并后台刷新，重复读取合并，显式刷新仍直达事实所有者。公网支付返回 0 请求、显式刷新 4 请求、生产返回经营 0 请求，0 个 429／写入／页面错误；仅 Workbench 滚动，17／17 healthy。后续完整 OA 逐栏目施工必须复用该模式，先呈现稳定页面结构再读真实投影，不再把“点击后清空等待”当独立小应用的必然体验；本批不减少任何 OA 缺项，见[报告](./reports/oa-page-read-experience-validation-20261005.md)。

2026-10-04 BF-106 最新：正常工作台直接复用原方案面板／800px 抽屉，经真实 Nginx→Gateway→正式 IAM→Production／Evidence 完成16个有限场景；整批多文件、精确评分、预览下载、移出留史、丢回复／上传中断续办和五所有者逻辑空目标恢复实际通过。PG18全量、377项Node及原UI-only13项分层记录，见[报告](./reports/oa-production-stage-native-validation-20261004.md)。Main未发布，真实阶段D1／keys／旧会话恢复、旧活动任务显式过渡和全角色仍是发布门槛；不能借已有BF-105所选恢复或新项目绕过。后续阶段先核对原差异，再定义新版本／过渡，不原位扩充已注册 version 2。初设／施工图／试验／报告、变更竣工、批复审计、外委、资料统计和全站连续旅程继续保留；本机Docker维持停止、浏览器串行收尾。

2026-10-04 所选恢复门槛追加：真实 D1 六库联合备份／新空目标恢复、原命令重放和原人员／奖金 UI 继续通过14场景，[限制](./reports/oa-professional-selected-d1-restore-20261004.md)不扩为全 Suite／旧主站升级。下一功能是[原方案阶段](./specs/assembly/oa-production-stage-original-operation-map.md)多主文件／独立校审单／可选评分：先验证原 UI，再固定旧 Base 不变的新版本原生事务／权限／Evidence，不能套用旧单槽或仅复制界面称完成。Main 精确授权和匹配发布仍待。

2026-10-04 最新 D01→B12：版本化独立引用、Bonus v3兼容消费及正常工作台原 UI 连续接线已候选完成；20真实私有场景、347 Node、Project163／含子418、Bonus32／含子93 PG18及 Bonus race、TS／Go／vet通过。[报告](./reports/oa-professional-bonus-continuous-validation-20261004.md)。下一门槛为精确公网授权、匹配镜像／profile／恢复闭包与同项目公网操作；原各类型阶段、批复审计／外委、B11、资料统计和全角色继续推进，不重复重做已完成引用或把本地当发布。

2026-10-04 D01 原生候选：原 600px 专业／审定抽屉、专业四列及目录接上 Project 有界六分工／PG18、正式 IAM 和真实 People；21 场景点击／故障／重启、全量 PG18、原子／并发及单库恢复分层通过，Node 315／315。没有改旧 Base Role／v1 引用，不自动扩 Grant。下一步新分工最小版本化引用及 Bonus 显式兼容消费、全角色、正常 Workbench 接线和匹配发布；不将仅复制界面或只有 API 算验收，也不把本地证据当公网。[原生报告](./reports/oa-production-personnel-native-validation-20261004.md)与[门槛](./specs/assembly/oa-production-personnel-clone-gap.md)，全站缺口保留。

2026-10-04 生产页续轮：原总览、AntD 页签／类型并集及 CSS 已候选接入，Cost／Bonus 不再被总览同时读取；真实组合 17 场景、相邻四工作面回归及 307 项 Node 通过。未部署 Main、未新增 Grant；不是全部生产 UI。下一纵向切片按[原生产人员对照](./specs/assembly/oa-production-personnel-clone-gap.md)复用专业组／两抽屉，先完成有界分工表示、专业归属和兼容／权限，再接真实保存；不把负责人冒充专业角色。[实际报告](./reports/oa-production-layout-ui-validation-20261004.md)，全站其他缺口保持。

2026-10-04 09:10 UTC 续轮最新：B12 实际 Workbench／Gateway／IAM／Bonus＋Project／People 原生端口的 14 有限场景通过，300 项 Node 零 skip、真实 PG18、恢复后继续登记／撤回及来源摘要保持。BF-103 修旧／新 namespace 精确入口兼容，下一发布闭包必须同时匹配 Gateway／Workbench／Suite schema／renderer 和 Bonus profile／migration／IAM 目录。Main 未发布／未新增 Grant；Billing／奖金精确授权仍待答复，不能挪用 Cost 批准。B11、专业生产、原 C 公网及完整 Suite 恢复仍缺，[最新报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)。

2026-10-04 08:32 UTC 续轮：BF-102 B12 原奖金组件候选完成两侧混合登记／刷新／撤回及 PG18 空目标恢复续办，schema 2→3 保持旧 Pool／建议；未发布、未新增 Main Grant。后续匹配发布闭包及批准的 admin＋原 C 精确授权／公网链，继续 B11、专业生产及全站工作面；Billing 独立授权待答复，不扩大此前 Cost 批准。[候选报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)。完整 OA 和长期边界证据仍未完成。

2026-10-04 07:36 UTC 公网优先状态：匹配 Contract 22／Money 23／Billing 1、Cost schema 3 和 Workbench `.6` 已发布，17 服务健康。C 成本有限连续链、十二侧别／分类组合及原生收尾通过；已修复真正阻断连续撤回的弹层遮挡，没有强制点击或改原生事实。合同／支付九检查点及原生边界收尾通过；可见两链合同、三协议、两定案、文件预览下载、两 Cash 和冲销保留原金额。下一步同 C 开票精确授权及公网链、合同最终更正和三费用关联，再落实奖金手填基数／固定额／多角色的兼容语义并复用原 UI，继续分类型生产、成果、外委、资料统计和全角色。成本批准不扩到 Billing，原事实不重建，旧事实与边界保持。全站仍未完成；[完整清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)和[公网报告](./reports/oa-commercial-public-ui-validation-20261004.md)为准，下方未发布描述为历史批次。

2026-10-04 成本下一切片按 BF-101 [规格](./specs/assembly/oa-cost-receipt-overlay-v1.md)：Cost 自有七类／两侧扩展先完成原组件新建→原生复读，再补更正／撤回、文件、分权和恢复，保持旧通用费用兼容。规格不是 UI 完成；全站范围不缩为合同／支付／开票／成本四栏。

2026-10-04 最新施工：合同 ae 22、支付 j 14、开票 f 15 个原组件场景与各自 PG18／启动分层通过，262 项 Node 零 skip，Workbench 最新资源两次独立启动通过。Contract 实际 issuer／Evidence／字节恢复 m 和历史夹具 18→22 原子迁移 a、Billing 最新 c 真库与实际 issuer／Evidence／字节恢复 e 已补证；修复移出最后附件 500。候选均未发布，不等于完整 OA。下一步匹配 API／migration／IAM policy 的发布兼容闭包、全角色／同项目公网；继续按[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)接[成本实际缺口](./specs/assembly/oa-cost-original-operation-map.md)、奖金、四类型生产／结果／外委、资料统计与人员权限，不用静态表单代替业务。分层限制及原操作差异分别见合同／支付／开票报告，以下较早施工状态保留历史。 最新构建、截图与 Main 17 服务只读收尾统一见[本轮检查点](./reports/oa-complete-ui-checkpoint-20261004.md)。

2026-10-04 后续批次：Contract 真 IAM＋Evidence 上传／引用／签名下载／停机拒绝／联合冷备空目标恢复 m 已通过，Main 保持；全角色、最终更正、升级和公网门禁未闭合。已继续复用 OA 支付原双列表／表单，接 Money 实际收款→Project 六分类关系及原生冲销，235 项逻辑回归通过，浏览器／PG18／独立启动正在验证。未部署；开票关联、成本／奖金、生产全过程、资料统计和权限按完整清单继续，不能收缩为招投标或合同。见[支付批次](./reports/oa-cash-ui-candidate-20261004.md)。以下旧待验／未接线是各自历史批次。

2026-10-04：合同原组件、显式原始合同／三类协议、材料与阶段／最终结算已候选接线；schema 22 真库、应用库空目标恢复和独立启动已通过，未部署。先补真实 IAM＋Evidence 联合及字节恢复、全角色／冲突、更正与兼容发布，再验同项目公网合同链；支付／开票、成本／奖金、分类型生产、资料统计及全角色仍全部保留，不把原表单外观算完成。见[当前候选](./reports/oa-contract-draft-ui-candidate-20261004.md)及[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)；以下旧未接线描述为历史批次。

2026-10-03 用户以招投标作例子并要求全站继续：按[完整操作交付清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)施工，不把 B03 当全部范围。BF-097 已发布独立经营参与职责、原组件、schema 11 和显式新能力；同一 C 的真实增减／历史重入／原命令恢复已有证据。Workbench `.6` 已部署隐藏来源读取修复，终场公网三项零写续验通过、快速切页无 429、旧事实保持；完整 B02 分权／父包传播仍未完成。Contract 原生端口已通过聚焦真库及自有空库启动，尚未部署或接原 UI，不能关闭 B04～B06。

人员本批之后，下一纵向闭环为原两链合同及协议／定案：先把两链上下文、正式主体、分项费用／费率、Evidence 和部分成功续办映射清楚，再接原组件，不把空表单当成完成。支付／开票、成本／奖金、分类型生产、资料／统计和全角色矩阵继续在同一清单；完整立项字段、B03 分权及 OSS／恢复仍保留，批次验收不替代整体结论。实际变更和反例见[本批报告](./reports/oa-complete-operation-progress-20261003.md)。

2026-10-03 BF-096 首批已落地：直接复用原 OA 承接方式、材料列表、预览及确认组件，在 Auth58 `.20261003.1` 真实跑通 Evidence→BE→Project 的文件链、单份移除、刷新及未知回复 reload／原命令继续。两个应用兼容发布、BE 前向 migration 和升级前 13 库数据恢复通过；当前使用 local 文件 Provider，不算 OSS。见[本批报告](./reports/oa-reused-bidding-ui-validation-20261003.md)。

接下来仍按原 OA 逐动作补证，不开展 UI 重设计：先补 B03 角色／撤权、读写失败及文件类型／大小矩阵、Evidence 架构透镜 step 与少量不准确旧文案；完整立项字段、B02 成员及合同等既有缺口不被跳过。后续板块继续复用现成组件、最小适配，不先铺新模块；OSS 静态前端／文件切换、完整 Suite 运行恢复独立验收。下方“Main 未迁移／尚未挂载”均为此前候选历史。

2026-10-03 BF-095 后端门禁已关闭：多份追加／单份移除及旧命令兼容、真 PG18 反向权限与并发、旧库升级和单库恢复继续、独立 IAM 启动通过；Main 未迁移。见[实测报告](./reports/oa-bidding-material-collection-validation-20261003.md)。当前只进入 B03 同项目附件 UI 接线，仍需真实文件、部分成功恢复、兼容交付及公网点击，不开始其余新大块。

同批 Workbench 材料命令与动态架构透镜候选完成 156 项 Node、Go tests／vet和独立启动检查，但未接完整 Evidence 文件适配、未等效原 OA 附件界面、未部署。按四小时上限暂停收尾，Main `.11` 17/17 healthy，BE 000001／Evidence local 不变；下一次先补文件选择／查看／下载和部分成功继续，再进行兼容发布及公网验证，不为候选测试提前关闭 B03。

2026-10-02 BF-093／BF-094 最新执行口径：完整 OA 为主，小系统能力可以逐步演进，职责／事实／鉴权边界不得静默漂移。B03 方式／适用性 `.11` 已在同一 G／来源通过公网点击及刷新原生复读；下一项先兼容扩展 BE 自有多材料集合，再接 Evidence 附件全链。完整立项字段、B02、角色矩阵、OSS、C8 和整个 OA 不因此通过。每批以来源版本、接口、migration／兼容和真实用户结果记账，不以“零内核改动”替代目标，也不以组件测试替代整站验收。

2026-10-02 BF-092 插入修复：按用户立即修复要求，先关闭同项目页签硬禁用，接已有原生读取、错误／权限区分、刷新与 URL 恢复。此批不推进 B02 新角色，不伪装完整表单；完整立项字段、经营参与映射、成本归属及各页新增／编辑／资料操作仍须按原 OA 逐项关闭。候选与真实部署证据见[页签验证](./reports/oa-business-tabs-repair-validation-20261002.md)。下面 `.5` 是此前修复批次的历史范围。

2026-10-02 最新 `.5`：未知提交回复和重新加载旧抽屉遮挡已修。三次隔离原生浏览器观察保留两次反例，最后恢复同一项目通过；Auth58 只更新 Workbench，公网旧立项深链、编号搜索、详情刷新及原生冻结门禁通过，C revision 7／9 回执不变。13 库空目标数据恢复和浏览器后 D1 check 通过，未制造公网未知回复、未验完整 Suite 恢复运行。见[恢复报告](./reports/oa-acknowledgement-recovery-validation-20261002.md)。下方 `.4` 保留当批证据；下一切片先明确待立项甲方／资质／精确费率的兼容保存、失焦输入和部分成功，当前三个禁用占位不能算可用，不跳 B02 或更改内核边界。

此前 `.4` 批次（BF-090／BF-091）：Auth58 Main 已更新 Workbench `.4`。导航修复后在同一 C 明确复现长期开页 CSRF 过期拒绝，修复为读取当前 Cookie／缺失时经配置 IAM 更新后正式授权；不自动重试业务写入。真实清除／恢复负责人、编号搜索、刷新、返回和 Back／Forward 及最终原生连续门禁通过；用户原记录、全部旧 Project 行、Grant、其他 11 业务库不变，见[续验报告](./reports/oa-csrf-continuation-validation-20261002.md)。下一步仍补第一步的完整字段、人员筛选、权限／部分成功矩阵和原站对照，不先跳 B02；I01～I04 全项未关闭。之前两次清除失败及锁屏阶段保留，不换夹具绕过。

此前 `.2` 旧待立项缺经营分工、已有生产分工恢复的真实补齐／更换、完成和同 OA 刷新证据仍保留，用户原记录未代选或提交，见[恢复报告](./reports/oa-initiation-recovery-repair-validation-20261002.md)。当前 `.4` 只重建 Workbench，Project／IAM／schema 不变，不改变各小系统边界；完整链仍按[原 OA 分步清单](./specs/assembly/oa-original-usage-workflow-baseline.md)逐操作关闭。

BF-084～BF-089 已实现切片确已在 Auth58 Main 公网发布；专用新夹具的正常路径、13 库／文件备份及空目标数据恢复、前向迁移和 health 的历史证据保留，范围见[修正后的报告](./reports/auth58-oa-first-slice-deployment-validation-20261002.md)。它取代下方候选批次“Main 尚未更新”的部署状态，不代表立项或完整产品通过。之后按项目管理／包 → 同项目经营 → 四类型及多类型生产 → 资料统计 → 角色及连续恢复顺序关闭动作；C8、OSS、正式制品独立待办。

BF-089 候选已补原甲方输入框的只读历史名称幽灵提示／Tab，可信范围、GET 零写、原生 HTTP 真库、独立启动及合成浏览器焦点／保存／刷新取得[分层证据](./reports/oa-client-name-clone-repair-validation-20261002.md)。B02 已形成[原源码对照与验收输入](./specs/assembly/oa-business-personnel-clone-gap.md)，未接受新的角色架构或实现；其兼容规格与添加／移出实现现排在立项阻断关闭之后。B01 全项及正式发布继续待办；不以剩余时间启动未经定义的新大块。

2026-10-02 BF-088 当前切片：原 B01 同项目经营页内的甲方名称／联系人行内编辑已候选接通，以项目专属联系上下文与 Party 身份分离保存；原子、权限、历史、失败恢复、真库空目标恢复和合成浏览器分层证据见[报告](./reports/oa-client-info-clone-repair-validation-20261002.md)。其余经营栏目仍逐动作待接通，名称历史补全、经营参与人分权、正式 Overlay 升级及公网未由此关闭。

版本：0.89
日期：2026-10-01
状态：Architecture Baseline 0.1 已确认；公司 Ubuntu 完整 Suite 已装入 12 个业务应用与 Nginx、Gateway、IAM、Workbench、PostgreSQL，联合备份／空目标恢复／继续写入和 17/17 health 已验证。2026-09-30 同一 Tenant／Project 连续通过 C0～C7；C8 又通过当前 Subject 工作隔离、Money grant/revoke 分权、Evidence／Production 顺序故障恢复、49/49 HTTP 状态和十二库 canonical facts 前后不变，但因 C4～C6 输出没有 active Project Link／资料投影、Evidence Gateway 仍要求遗留 `view` 解锁 download，最终判定为 `insufficient-evidence`。阶段一报告已冻结为“解耦方向在当前闭环中可行、完整 OA 装配尚未闭合”；当前 P0 只修这两项装配合同并在原 Project 重跑 C8，不用第二客户掩盖失败。公网 HTTPS、离机跨机器恢复、正式 clean-HEAD 发布、Contract 遗留文件迁移、双 Tenant 与第二真实客户作为后续独立证据轴保留。

状态补充：C3 Project 经营链路已从正式入口完成 Contract／Obligation／Transaction／Invoice Registration 四类分段、独立人类金额 Capability、三角色、撤权／停用、四类故障恢复和性能门槛；本地 D1 结果为 100 个 HTTP 断言、0 个差异、13/220 ms p50/p95。公司 Ubuntu 连续 C3 又完成同一 C2 state 的 Party 显式迁移、Billing／Money／Project 业务链、金额脱敏和局部故障恢复，40 条断言零差异。C3 保留真实浏览器最终点击、双 Tenant 和 Billing-only 为补证轴，连续主线转向 C4。

状态补充：C4 已完成静态责任边界审计并接受 BF-056／BF-057；C4-A01～A14 当前本地轮次已通过。Project 立项门槛已从错误的 design 硬编码收敛为“至少一个规范化类型”，注册与项目选型仍由 Production 最终校验；Level A／B 聚合回归保持。People＋Project＋Evidence＋Production 的隔离 D1 已取得 Suite Manifest／Workbench、完整未选缺席、正式 Nginx→Gateway→IAM 主链、IAM／Evidence 故障恢复和联合空目标恢复证据。Production 专项 Grant 撤销／恢复、账号停用／启用、Assignment 结束、服务端 `allowed_actions` 和真实浏览器恢复也已通过 86 条断言与受控点击。下一实现顺序是按证据进入 C5，同时保留双 Tenant 和真实 Ubuntu 为环境补证轴；C4-A14 持续检查没有通用流程解释器、共享表、双写或客户核心分支。当前不得把本地 D1 结果写成 C4 或 OA 整体完成。

状态补充：2026-09-30，公司 Ubuntu 隔离完整 Suite 又从 `state-after-c3.json` 连续通过 C4。原 Tenant／Project revision 10 和 C1 生产负责人被直接复用；`design-v1` 与 `consultation-v1` 各到 revision 8，Evidence 保留两个 File／三个不可变 Version。错误类型阶段 422、stale revision 409、Evidence 503 零写入、同 Command 恢复单次提交、报告 Version 2 替换和重开／再完成均通过；Project／People 零新增事实、所有权增量精确、完成态重跑不改变 ID／revision，最终仍为 17/17 healthy。连续主门禁现为 C0～C4（5/9，约 56%），下一批只从 `state-after-c4.json` 进入 C5。

状态补充：同日隔离完整 Suite 又从 `state-after-c4.json` 连续通过 C5。复用原 Actor、Contract、award Evidence 与 Project；Results 新增两条结果、三条 revision，Project 只在显式确认时新增一个检查点并前进到 revision 11。Assessment GET 零写入、金额脱敏、Grant 撤销 403、append-only correction／audit frozen revision、source drift、Evidence／Contract 503 零写入同命令恢复，以及 Results 停机时旧命令本地回放／新命令失败关闭均通过。首轮 46 条、完成态重跑 14 条 HTTP 断言零差异，所有权增量精确，最终仍为 17/17 healthy。连续主门禁现为 C0～C5（6/9，约 67%），下一批只从 `state-after-c5.json` 进入 C6。

状态补充：同日隔离完整 Suite 又从 `state-after-c5.json` 连续通过 C6。复用同一 Tenant／Project、C1 Actor、C2 供应商 Party／Contract 与既有失败建议；Cost 形成 revision 2，Outsourcing Engagement、Assessment、Payable Suggestion revision 3 与 Money Obligation 均在各自数据库追加。Money 停机时建议保持 `target_unavailable`，恢复后同一命令只创建一个义务；首轮 24 条、完成态重跑 15 条 HTTP 断言零差异，所有权增量精确，最终仍为 17/17 healthy。过程修正 Cost／Bonus 的完整 Money action namespace 与 Money exact suggestion resource scope，未放宽目标应用最终授权。连续主门禁现为 C0～C6（7/9，约 78%），下一批只从 `state-after-c6.json` 进入 C7。

状态补充：C5 已完成独立 Results、Project Completion Profile、正式 D1 和 74 断言真实旅程，并已在连续主数据集再次通过 46 条首轮和 14 条完成态重跑断言。C6 随后完成责任边界、领域核心／PostgreSQL 18、受保护 HTTP／正式 IAM、精确 Scope、人类工作区、正式 D1、Party 运行时切换，以及 C6→Money Payable Suggestion 真实双进程旅程；现在又在连续主数据集通过正式 Gateway／IAM connector、目标停机恢复和完成态重跑。C7 又完成责任边界、领域／PostgreSQL、受保护 HTTP／IAM、真实来源端点、六进程六数据库组合、人类工作区、正式 D1及 C7→Money 直接跨进程建议；后者以 11 条 HTTP 断言验证目标停机恢复、接受／拒绝、exact Person→Party 映射、幂等和源撤回无级联。C8 静态边界审计已通过 BF-073 固定资料、指标和工作项所有权及局部失败语义；direct Project→Evidence、Money 指标、Production `WorkItemProjection v1` 与 Business Engagement 间接资料运行链均已有正式公司 D1 或真实运行证据。当前工程实现约 98%；阶段实现与分项验收约 96%～97%，但最新主门禁改为“同一数据集连续 C1～C8”后只完成 C0～C6，不能把剩余比例误读成几个小修。C7～C8 连续复演、C8 真人浏览器和第二来源、C7 正式 D1 connector／建议引用恢复、单类型最小组合、完整双 Tenant 和离机跨机器恢复仍是主要剩余项。

## 2026-10-02 当前优先级纠偏：先还原原 OA 的使用流程

BF-087 进一步按原站把核心资料与两类负责人合入同一编辑抽屉；原子更换／清除、权限分开、旧分工历史、冲突零部分写入已有候选和分层验证，见[基本信息报告](./reports/oa-basic-info-clone-repair-validation-20261002.md)。这取代下方“两类负责人调整尚未实现”的当前状态，不取代历史验证记录。下一项核对资质／费率及甲方的事实归属和原用户输入路径、负责人候选人员筛选，再接经营与分类型生产；P01 进度投影继续待办。Main 与整站验收仍未完成，不越过正式兼容／恢复门禁。

BF-086 已补 P01 目录／导航候选及分层证据：只读原生目录、年份／分页／搜索、包语境、返回和刷新，另修实际滚动容器。见[目录报告](./reports/oa-directory-clone-repair-validation-20261002.md)。P01 尚缺经营／生产进度筛选，P02 尚缺两类负责人调整／甲方／资质费用；后续逐原站动作接通，不把本批当 P01 全部通过。正式兼容发布、联合恢复、显式授权版本和公网浏览器保持门禁；Main 未更新。

立项切片之后，P02 的 active 核心基本资料编辑已使用 BF-085 Profile Correction 接通；普通项目／无编号包的保存、复读、刷新和抽屉按钮可达已在合成界面实测，独立真库与启动也通过。见[本轮报告](./reports/oa-profile-clone-repair-validation-20261002.md)。下一片先补 P01 导航、筛选与返回上下文及 P02 分工更换；资质、财务字段和其余业务各自按证据补齐，不把基础字段通过描述为 P02 全部通过。串行限资源验证，Main 未更新。

本轮新要求（BF-084）：目标升级为完整仿制原 OA 界面和使用效果，已经开始代码修复。下方“只梳理／保持暂停”指上一轮证据审计；本轮按操作 ID 逐项实现，不再以有限代表旅程代替整站。第一组件修复立项后补门槛：显式 `business-owner-v1` 保留经营负责人要求而不强制生产负责人，默认旧 `dual-owner-v1` 行为与幂等账兼容；随组件结果再修派发／统一工作区。全站、正式部署与浏览器均未通过前不写完成。

用户试用指出的立项问题不是两个孤立按钮缺陷。已完成[第一轮全站使用流程源码梳理](./specs/assembly/oa-original-usage-workflow-baseline.md)，覆盖入口、项目包、经营／生产、资料、统计、人员与权限，登记输入时机、跳转、原站冲突和部分成功路径；原站真人复核与克隆逐操作映射尚未完成。此前约 98%／96%～97% 只描述当时定义的工程任务，不能用于估计当前可用 OA 完成度；“只剩两项 P0”只描述当时 C8 阻断，不再描述全部产品剩余工作。

当前顺序：原流程基线 → 逐操作差异与验收 → 先闭合派发／完成立项／同项目详情／后补负责人 → 项目包及经营 → 分类型生产 → 资料统计与权限。每次只做一个完整可操作切片，同时保留独立边界和既有回归；原部署验证目标保持暂停，不在这次文档梳理中升级候选镜像。见 BF-083。

## 1. 计划目标

2026-10-01 当前切片：BF-082 源码、固定 test.49 Gateway 的隔离真实 IAM／Evidence 三账号分权、错误 File Scope 和撤权后拒绝，以及 Workbench 的当前 D1 清单离线启动已通过；C8 驱动原生三权限断言和离线负例已替代 view 补授及固定 fail，主站兼容交付及完整 C8 尚未运行。C4 Production 当前成果只读来源随后通过隔离真 PG18／真 IAM；Project canonical Link／000008／typed reader／单 native Action Policy 通过源码、7→8 旧事实保持、HTTP 测试端口和单库空目标恢复后追加。主站 13 库摘要／17 容器 ID 不变，探针全部停止；来源与消费者组件证据不拼接为完整跨进程／正式发布结论。下一批先补真实链与 C5／C6 来源，正式升级前保留兼容闭包及联合备份门禁。见[入口验证](./reports/auth58-evidence-native-entry-validation-20261001.md)、[C4 来源](./reports/auth58-production-material-source-validation-20261001.md)及[消费者](./reports/auth58-project-production-material-consumer-validation-20261001.md)。

20:15 补充：C5 Results 资料来源源码、真 PG18 更正／撤回和真实 IAM8 原生 read／Result／Project／Tenant Scope 已通过，Main 未更新且 Project 消费／C8 未关闭，见[C5 来源实测](./reports/auth58-results-material-source-validation-20261001.md)。下一批 C6 只补相同有界来源，不增加服务或改变付款／评分／完成语义。

20:25 C6 来源切片通过真 PG18／真实 IAM8，两类 read 互斥、金额分权、十表只读不变，见[实测](./reports/auth58-cost-material-source-validation-20261001.md)。Project C5／C6 消费按[规格](./specs/assembly/project-results-cost-material-consumer-v1.md)开始，正式链与整体结论保持待验。

20:38 消费组件源码／契约、真 PG18 8→9 旧事实保持与单库空目标恢复通过，见[实测](./reports/auth58-project-results-cost-material-consumer-validation-20261001.md)。下一层独立验证真实 Workload 与目标原生验签；Main、正式联合恢复和连续 C8 未关闭。

21:05 只读真实 Workload／目标验签进程链已通过，包含 Production、Results、Cost／Engagement 与独立 Evidence 元数据；探针停止、Main 保持，见[报告](./reports/auth58-material-workload-validation-20261001.md)。下一步先固定四应用不可变候选镜像与兼容闭包，随后按 BF-077 联合备份／空目标恢复及真实入口分批交付；不复活 restore20，也不以合成来源种子外推完整业务链。本轮最迟 22:07 暂停检查。

四应用 `test.50` ACR 候选与实际空库启动已通过，开发 D1 bundle 的内外摘要、17 镜像配置、新端点、Manifest 原生入口和 IAM8 无数据库 parser 预检也通过；详见[制品报告](./reports/auth58-material-candidate50-validation-20261001.md)。本轮不切换 Main。联合多库／文件备份、空目标恢复、兼容 closure 部署与真人提交／完整 C8 是随后仍待完成的不同证据轴，不把发布准备计为产品验收。

总计划的最小根因是：

> 每个客户都从头定制一套系统，交付慢且难复用 → 把反复出现且已经稳定的问题做成独立小应用，让新项目主要通过组合和少量定制交付，而不是从头重做。

本计划不是建设通用平台的路线图。已经交付两个小而完整的窄域业务系统，当前工程交付基线是把它们与必要平台、基础能力和统一体验装配为 **Complete Suite 0.1**：最低一台 ECS 可以完整运行，同一发布物可以按 Deployment Profile 扩展到两台、三台或高可用多节点。产品验证的阶段一目标则是用这些独立能力重组参考 OA；两者分别回答“能否可靠交付”和“能否承载真实参考业务”。当前组合按以下职责推进：

1. Nginx：完整组合唯一公网 HTTPS 边缘入口；
2. Gateway：Nginx 后方的内部应用入口；
3. Identity & Access：账号生命周期、所有受保护访问的强制 IAM 契约与平台应用；
4. People & Groups：以 Group 为核心管理人员、Membership、组内角色、能力画像和名册快照；
5. Party Directory：独立主体领域应用，独立拥有主体当前档案；可在主体管理组合中作为主要产品，也可作为其他业务产品的显式依赖；第一版生命周期、模型、API、IAM 与迁移边界已定义，独立 Go／PostgreSQL 核心、可执行 release、非 root 镜像、空目标恢复、可操作管理页、Project 客户选择及 D1 API 链／联合恢复已通过；Contract／Money 的只读预检、来源自有人工确认映射和 D1 各 1 条真实审核已实现，含引用数据与运行时切换尚未实现；
6. Project Workspace：可选项目容器应用；
7. Business Engagement：独立经营承接应用，拥有适用性、方式、材料业务引用、revision 和历史；后端闭环、migration、原生 Capability、真库、正式装配、Workbench 页面、浏览器撤权刷新、本地 D1 空目标恢复／继续写入、409／503 自动化与真实浏览器恢复，以及账号停用、IAM／Policy 故障恢复已实现，下游建议待完成；
8. Contract Ledger：已经实现的合同台账；
9. Money Flow Ledger：已经实现的经营收支台账；
10. Evidence Service：独立文件与证据小服务，支持本地文件与阿里云 OSS Provider；
11. Unified Workbench 与 Deployment Profiles：统一体验，以及一台、两台、三台和高可用多节点的可配置组合交付；
12. Business Engagement → Contract／Money 与 Contract → Money Flow：等待真实业务流的可选连接器。

每个业务小系统都必须能够作为独立产品安装、运行、升级和使用，并在不部署未声明业务系统、审批或客户 OA 的情况下完成自己的业务闭环。应用没有永久的“核心／辅助”商业等级；具体组合选择能够完成目标业务结果的最小应用集合及其直接依赖。“独立产品”允许安装包携带显式声明的 IAM、Party、Evidence 等应用，不等于单进程或零依赖；依赖必须有版本化契约、Suite Manifest、数据所有权和故障验收。所有受保护访问始终依赖 IAM 契约；没有外部 IAM 时由同形本地模板提供者满足该契约。

A→B 只规定开发方向，不单独构成可执行计划。每个里程碑和实现事项还必须明确不能破坏的约束，以及证明 B 已经实现的验收证据。

每个新规格和实现事项还必须引用受影响的 [`docs/decisions.md`](./decisions.md) 决策 ID，或明确说明与现有决策无冲突。讨论形成的新共识若未登记到决策总表，不得作为隐含开发前提。

ICP备案等待期按 [D1 双环境制品提升与公网验证](./operations/d1-environment-promotion.md)并行推进：先完成目标无关发布包、制品提升、离机备份和空目标恢复；公司域名与阿里云资源获批后在隔离环境复现同一摘要版本并执行完整公网链验收。未获批时不触碰公司资源，也不暂停个人试验田和阶段一参考 OA 验证。

## 2. 初始工程形态

为减少早期管理成本，同时保留拆分能力，第一阶段推荐使用一个代码仓库中的两个独立应用：

```text
OMSBoundry/
├─ backend/services/
│  ├─ contract-ledger/       # 独立应用、配置、数据、迁移和测试
│  └─ money-flow-ledger/     # 独立应用、配置、数据、迁移和测试
├─ connectors/
│  └─ contract-money/        # 可选连接器，不属于任一系统内核
├─ packages/
│  └─ primitives/            # 仅在出现真实复用后放入极少量稳定值对象
└─ docs/
```

约束：

- 两个应用不得共享业务数据库表；
- 两个应用不得相互引用内部领域对象或 ORM 模型；
- 每个应用有自己的启动入口、环境配置、迁移、测试和部署产物；
- `packages/primitives` 不提前建设公共框架，只允许 Money、Currency、ID、时间、Revision、幂等键和来源快照等稳定原语；
- 连接器只使用公开 API、事件和快照；
- 以后拆成独立仓库或独立部署时，不重新设计业务模型。

这是一项 M0 阶段需要验证和确认的工程假设，不代表已经选择具体技术栈。

目标应用组合形成后，仓库可以按下列职责扩展；目录只在对应里程碑启动时创建：

```text
OMSBoundry/
├─ frontend/
│  └─ oms-workbench/         # 统一体验，不拥有业务事实
├─ backend/
│  ├─ platform/
│  │  ├─ identity-access/
│  │  └─ gateway/            # 内部应用入口，不拥有业务事实
│  └─ services/
│     ├─ people-groups/
│     ├─ project-workspace/
│     ├─ business-engagement/
│     ├─ contract-ledger/
│     ├─ money-flow-ledger/
│     └─ evidence-service/
├─ deployments/              # Nginx、D1 与其他装配
├─ connectors/               # 只保存有真实业务流的定向连接器
└─ deploy/
   ├─ manifests/             # Suite Manifest：安装／启用哪些应用
   └─ profiles/              # Deployment Profile：D1～D4 节点放置与副本配置
```

Nginx、Gateway、Unified Workbench 和部署运维包是正式装配组件，不增加业务数据所有者。完整组合固定使用 Nginx＋Gateway，单应用开发可以直接访问应用入口。“基础”不建立万能应用，当前只确认独立 Evidence／File Service。

## 3. 推进原则

### 3.1 最小根因先于方案

任何小应用、业务板块、功能、数据字段或技术机制进入计划前，先用一句好懂的话写清“问题 A → 结果 B”，再定义约束和验收：

```text
问题 A：当前哪里不清楚、不可靠或需要重复劳动？
结果 B：完成后谁能够明确知道什么，或者系统能够保证什么？
约束：过程中哪些既有事实和独立性不能被破坏？
验收：用什么证据证明 B 已经实现？
不负责：哪些相邻问题不属于本事项？
```

无法说明它服务哪个上层 A→B 的事项不进入核心；没有真实证据的问题保持开放。实现可以复杂，但每一层复杂性都必须能回溯到并保护已经确认的最小根因。

### 3.2 纵向闭环优先

每个阶段优先完成一个从界面或 API 到数据库、审计和测试的真实闭环，不先横向铺设公共平台。

### 3.3 核心事实优先

先保证合同 revision、义务、现金、核销等核心事实正确，再增加批量操作、复杂搜索、外部集成和视觉优化。

### 3.4 默认可用优先

所有受保护访问强制经过同一个版本化 IAM 契约；标准生产使用正式 IAM，客户明确不接外部 IAM 时使用本地模板提供者，自动化使用受测试边界限制的测试提供者。提供者可以替换，鉴权不能关闭。文件和事件等能力先提供满足本系统的默认实现；对象存储、审批或消息平台是可选适配器。

### 3.5 独立性持续验证

每个里程碑都要验证：移除另一个业务应用和所有可选适配器后，当前系统仍然能够完成核心业务；未接外部 IAM 时必须由本地模板提供者满足同一 IAM 契约。

### 3.6 有证据再抽象

允许少量局部重复。只有两个系统都出现、语义确实一致且已经稳定的内容，才进入共享包。

## 4. 里程碑总览

| 里程碑 | 最小 A→B | 主要交付结果 |
| --- | --- | --- |
| M0 | 工程结构是否支持独立小应用尚未证明 → 得到可重复验证的最小工程基线 | 技术选型记录、两个独立应用骨架、质量与部署基线 |
| M1 | 合同根事实模型尚未被真实操作证明 → 跑通一个合同最小纵向闭环 | 主体、合同草稿、激活、revision、查询与审计 |
| M2（已完成） | 最小闭环还不能独立交付 → 形成可安装、使用和恢复的 Contract Ledger v1 | 变更、关系、结算、终止、证据、权限、导入导出、备份恢复与独立安装升级 |
| M3（已完成） | 收支根事实模型尚未被真实操作证明 → 跑通“该收付、已收付、还剩多少”的最小闭环 | 义务、实际资金、部分核销、多对多核销、非经营性内部转账 |
| M4（已完成） | 最小收支闭环还不能独立交付 → 形成可安装、使用和恢复的 Money Flow Ledger v1 | Allocation 反向核销、Cash Transaction 完整冲销、Obligation 金额调整、减免、受控关闭、当前账龄、单条历史余额、当前经营汇总下钻、本方主体可校验台账导出、全量备份恢复、独立安装升级和当前预计现金排期已完成；预收／预付与超额由未分配资金安全承载，坏账、票据、真实退款与通用导入因关键语义不足而延期 |
| M5 | 两个系统间可能存在重复录入 → 有真实案例时验证可关闭、可恢复且不替目标系统作决定的自动传递 | 合同快照、义务建议、幂等连接和失败恢复 |
| M6（工程内收口，外部待验） | 代码能够运行但可交付和可复用尚未证明 → 用实际发布物和真实场景验证独立交付 | 两个 actual release 的独立安装、备份恢复、同主机故障隔离、场景与退出缺口审计、双 Release 运行手册、实物复演和干净 Git HEAD 正式溯源门禁均完成；真实操作者、容量、客户权限与客户场景仍需外部输入 |
| M7（已完成） | 多应用入口与身份边界不一致 → 用户从一个入口完成账号生命周期并以同一 IAM 契约访问已启用应用 | Nginx 公网边缘、Suite Manifest 草案、内部 Gateway、Workbench 壳、IAM 登录／邀请／激活、能力目录、正式／模板／测试提供者和现有应用迁移 |
| M8（已完成） | 人员集合的成员、分工和能力信息散落 → 随时知道有哪些组、组里是谁、各自承担什么及组具备什么能力 | People & Groups 最小闭环、可配置类型／角色／能力、Roster Snapshot、IAM Subject 关联和独立数据边界 |
| M9 | 同一项目资料散落在多个应用 → 从项目入口组织并操作外部业务记录 | Project 最小闭环、类型化关联、公开 API 调用、降级与失败恢复 |
| M10 | 文件能力重复且追溯边界不一致 → 由独立 Evidence Service 提供统一文件能力 | 独立进程与数据库、文件版本、摘要、上传会话、访问控制、引用契约、本地文件／阿里云 OSS Provider、迁移和备份恢复 |
| M10-P（核心、页面、D1 API 链、恢复及台账含引用数据运行时切换完成，完整双 Tenant 待验） | Contract、Money 和客户装配重复维护主体目录与 ID 映射 → 形成可单独使用、也可随业务产品交付的 Party Directory | 已实现独立数据库 migration、current／不可变 revision、登记／搜索／更正／状态／历史／精确引用 API、IAM 双 Provider、并发幂等、真库约束、可执行 release、非 root 容器、完整备份与空目标恢复；Party 管理页与 Project 客户选择已进入 Workbench，浏览器用用户凭证发现候选、Project 服务以精确 Party Scope 工作负载凭证复验并冻结快照；Party 停机／恢复、无 Party Base Project、幂等重试和联合空目标还原已通过；Contract／Money 已完成确定性预检、独立最小权限工作负载接线、来源自有映射和含引用数据的显式切换／只读回滚；Contract-linked C6 又通过 62 个正式入口断言、联合空目标恢复和本地写权永久冻结；待完成 Money 完整 D1 消费链和完整双 Tenant 矩阵 |
| M10-BE（目标） | OA C2 的承接方式、适用性和材料事实混在 Project／单体财务中 → 形成可独立完成承接材料闭环的 Business Engagement | 独立数据库、revision／历史、Project／Evidence 引用、细粒度 Capability、材料策略、页面、备份恢复、D1 权限矩阵和可确认下游建议 |
| M11 | 各组件存在但还不是一套可交付产品 → 形成最低一台 ECS、可按需求扩展到多节点的 Complete Suite 0.1 | Suite Manifest、D1～D4 Deployment Profiles、完整业务旅程、独立数据库、资源预算、升级、迁移、离机备份、恢复和故障影响验收 |

顺序原则：每个里程碑通过退出门槛后再进入下一个；发现事实模型错误时优先修正，不用后续兼容层掩盖问题。

## 5. M0：最小工程基线

### 目标

用最小技术验证确定能够支撑两个独立小系统的工程方案，避免在业务代码开始后反复更换基础结构。

### 工作内容

1. 盘点团队熟悉的技术栈和目标客户的部署限制；
2. 对候选栈验证以下能力：
   - 精确十进制金额；
   - 数据库迁移与事务；
   - API 与管理界面开发效率；
   - 自动化测试；
   - 文件上传与下载；
   - 最小认证授权；
   - 单机或容器化部署；
3. 确认一个仓库、两个独立应用的目录边界；
4. 建立最小持续集成：格式、静态检查、测试、构建和迁移检查；
5. 定义日志、错误返回、审计字段、时间和金额的基本规范；
6. 为两个应用分别建立空数据库启动和健康检查；
7. 记录关键 ADR：技术栈、数据所有权、部署形态、文件策略和认证策略。

### 不做

- 不在 M0 预建统一权限中心实现；本地可信身份先证明业务闭环，目标 IAM 契约按第 8 轮讨论另行设计；
- 不建设流程引擎；
- 不建设消息平台；
- 不设计全量组件库；
- 不提前实现合同或收支公共父类；
- 不复制 OMS/OA 技术方案作为默认答案。

### 退出门槛

- 两个空应用可以分别启动、测试和迁移数据库；
- 停止其中一个应用不影响另一个应用启动；
- 两个应用没有共享业务表；
- 开发者能够按文档在全新环境完成启动；
- 技术选择和未解决风险已记录，而不是隐含在代码中。

## 6. M1：合同最小纵向闭环

### 目标

尽快交付第一个真实可操作闭环，验证合同事实模型、revision 和小系统工程结构。

### 核心用例

```text
建立主体
  → 创建合同草稿
  → 填写双方、金额、币种和关键日期
  → 激活合同
  → 形成不可变 revision 1
  → 查看合同详情、历史和审计记录
```

### 最小范围

- 轻量主体目录；
- 签约主体快照；
- 合同编号命名空间；
- 合同草稿的创建、修改和取消；
- 合同激活；
- 精确金额和币种；
- 签署日期、生效日期和履约期间；
- revision 乐观并发控制；
- 最小合同列表、详情、创建和激活界面；
- 操作审计和基础权限；
- 核心领域测试与 API 测试。

### 退出门槛

- 从空数据库能够独立完成上述闭环；
- 激活后的合同事实不能直接修改或物理删除；
- 主体当前资料变化不会修改已激活合同中的历史快照；
- 金额不使用二进制浮点数；
- 并发修改可以被识别，不会静默覆盖；
- 不存在 Money Flow、Project 或 Approval 依赖。

## 7. M2：Contract Ledger v1

### 目标

把合同最小闭环补齐为可以独立试用和交付的第一版。

### 核心范围

- 原合同、补充协议、重述、更正、终止和结算文书；
- 显式生效顺序；
- 金额调整、金额重述和不影响金额的变更；
- 原始金额、当前有效金额和结算金额；
- 合同之间的资金来源、委托、分包、框架/具体、替代和引用关系；
- 合同文件与 Evidence Reference；
- 关闭、终止和受控纠错；
- 合同搜索、筛选、历史时间线和关系查看；
- 基础导入、导出、备份和恢复说明；
- 最小组织范围授权；
- 独立安装和升级文档。

### 明确延后

- 电子签章；
- OCR 和智能抽取；
- 合同模板设计器；
- 法务审查平台；
- 项目管理；
- 合同自动生成应收应付；
- 行业专用费率和结算规则。

### 退出门槛

- 任意时点的合同有效状态可以由文书序列重放；
- 同日多份变更有确定顺序，顺序不明时不能擅自生效；
- 上下游合同仍是独立合同，不形成固定层级表结构；
- 结算不覆盖原合同或当前有效金额；
- 安装 Money Flow 不是完成合同业务的条件；
- v1 数据可以导出并按文档恢复。

## 8. M3：收支最小纵向闭环（已完成）

### 目标

在不连接合同系统的情况下，跑通应收应付、实际资金和核销的核心闭环。

### 核心用例

```text
建立主体
  → 人工建立应收或应付义务
  → 登记实际收款或付款
  → 将资金部分或全部分配给义务
  → 查看已核销金额与剩余余额
```

### 最小范围

- 轻量主体目录与当时快照；
- 人工 Source Snapshot Reference；
- 应收、应付义务草稿与确认；
- 债权方、债务方、金额、币种和到期日；
- 实际收款和付款；
- 一笔资金对多个义务、多笔资金对一个义务；
- 部分核销；
- 未分配资金；
- 最小义务、资金和核销界面；
- 幂等录入和外部流水号重复检查；
- 余额不变量测试。

### 退出门槛

- 不安装 Contract Ledger 也能完成整个用例；
- Allocation 不超过资金可分配余额；
- 未匹配资金不会丢失；
- 义务余额由事实计算，不通过手工字段维护；
- 一对多和多对一核销均有自动化测试；
- 内部账户转账不会被重复计入经营收支。

## 9. M4：Money Flow Ledger v1

### 目标

补齐经营收支系统的纠错、票据、历史查询和交付能力。

### 核心范围

- 义务调整、减免、坏账和受控关闭（Obligation 金额调整 `MF-M4-003`、减免 `MF-M4-004` 与受控关闭 `MF-M4-006` 已完成；坏账 `MF-M4-005` 审计后延期）；
- Billing Document 与义务的多对多 Billing Link；
- 现金冲销、退款和更正（Cash Transaction 完整冲销 `MF-M4-002` 已完成；真实退款 `MF-M4-010` 审计后延期，不用纠错事实冒充）；
- Allocation 反向核销（`MF-M4-001` 已完成）；
- 预收、预付和超额处理（当前由未分配实际资金安全承载，但不自动赋予会计分类；边界审计已完成）；
- 指定 `as_of` 时点的单条 Obligation 余额（`MF-M4-009` 已完成）；
- 逾期和账龄（当前余额分类投影 `MF-M4-008` 已完成；跨义务历史账龄汇总不在 `MF-M4-009` 范围）；
- 预计流入、预计流出、实际流入和实际流出（`MF-M4-011` 已完成当前义务余额与当前有效实际收付投影；`MF-M4-015` 已用独立 expected date 和查询时 outstanding 完成当前预计排期）；
- 统计下钻（`MF-M4-011` 已完成六项 current 指标下钻）；
- Excel 或通用表格导入导出（`MF-M4-012` 已完成本方主体范围的固定 CSV 可校验导出；导入因批次权威、业务键、冲突、部分失败和纠错语义未定义而延期）；
- 权限、审计、备份、恢复和升级说明（`MF-M4-013` 已完成可验证 PostgreSQL 全量备份与空库恢复；`MF-M4-014` 已完成带 build identity、显式 schema 门禁和单事务 migration 的独立安装升级交付）。

### 明确延后

- 会计总账与会计凭证；
- 税务申报和完整税务计算；
- 银行付款执行；
- 工资、报销和奖金；
- 自动收入确认；
- 跨币种核销与汇兑损益；
- 行业利润和专项成本模型。

### 退出门槛

- 错误事实只能通过调整、冲销或反向记录纠正；
- 任意汇总都能下钻到义务、资金和核销明细；
- 指定历史时点的余额可重建；
- 如果启用导入，重复稳定批次或业务键不会重复记账；
- 删除 Contract Ledger 不影响 Money Flow 独立运行；
- v1 数据可以导出并按文档恢复。

## 10. M5：可选 Contract → Money Flow 连接器

### 目标

证明两个小系统可以组合，但互不控制、互不越界，也不互为运行前提。

### 启动前证据状态

[OA 参考证据审计](./specs/connectors/contract-money-oa-reference-audit.md)已经确认一个真实参考信号：合同有效／结算金额在既有 OA 内被直接用作应收统计基础，而收款和开票仍由用户分别人工登记。该项目没有版本化跨系统交换、义务建议、幂等消费、失败恢复或人工对账流程，并且其固定两层经营链、Project、费种和计算公式明显属于客户定制。

[OMS 雏形参考证据审计](./specs/connectors/contract-money-oms-prototype-reference-audit.md)提供了相反且同样重要的信号：合同、发票和结算材料只作为依据，应收、应付和付款授权分别在审批通过时形成。两个参考源无法支持同一套“合同金额自动形成义务”规则，却共同支持合同依据、义务和实际资金必须分离。

因此两个参考都不足以启动连接器实现。M5 只有在取得一条脱敏真实样例并明确触发事实、账簿所有者、债权债务方、金额／币种／到期日、确认责任、变更处置与失败恢复后才分配首个功能编号。

业务输入统一使用[真实业务案例采集表](./templates/contract-money-flow-case-intake.md)，并按 [M5 启动就绪门禁](./specs/connectors/m5-start-readiness-gate.md)逐项判定。只有九项证据全部通过并进入 `ready_for_contract_design`，才先设计公开契约和联调矩阵；契约评审通过后才开始代码实现。

### 第一版场景

```text
合同激活或结算
  → Contract 发布版本化快照／事实
  → 连接器形成义务建议
  → 用户或策略在 Money Flow 中确认
  → 形成独立 Obligation
```

合同后续变化：

```text
新合同 revision
  → 连接器比较已引用快照
  → 形成义务调整建议
  → Money Flow 独立确认
  → 不覆盖既有义务、票据、资金或核销
```

### 核心范围

- 版本化 Contract Snapshot；
- SourceSnapshotRef；
- 幂等事件或拉取游标；
- 创建义务建议；
- 创建义务调整建议；
- 待处理、失败、重试和人工对账状态；
- 重复交付保护；
- 契约兼容性测试。

### 实现约束

- 第一版不强制引入外部消息队列；
- 可以使用本地 outbox 加轮询或明确 API；
- 连接器拥有过程状态，但不拥有合同和义务事实；
- 连接器不能自动覆盖 Money Flow 中已经确认的数据；
- 任一系统离线时，另一系统核心操作仍可继续。

### 退出门槛

- 关闭连接器后两个系统仍可独立使用；
- 重放同一事件不会重复创建义务；
- 合同 revision 变化不会静默修改历史义务；
- 连接失败可恢复且能人工对账；
- 两个系统均不存在跨库写入。

## 11. M6：可交付性与真实场景验证

### 目标

证明产物不仅在开发环境运行，而且可以重复安装、升级、恢复并适配不同客户场景。

### 验证内容

- 全新环境独立安装 Contract Ledger；
- 全新环境独立安装 Money Flow Ledger；
- 同环境组合安装但保持独立数据所有权；
- 备份与恢复演练；
- 数据库迁移和版本升级演练；
- 最小权限与组织隔离检查；
- 文件访问控制和审计检查；
- 基础性能与大批量导入验证；
- 错误注入：外部文件、连接器或另一个系统不可用；
- 三类装配场景验证：设计单位、资质施工综合体、小型咨询服务公司。

### 退出门槛

- 安装、升级、备份和恢复步骤可由新开发者按文档复现；
- 三种场景不需要修改两个系统的核心表结构；
- 客户术语和角色变化只影响配置或装配层；
- 一个客户增加专项业务时，不向通用内核添加专用字段；
- 已知限制和后续候选能力有明确清单。

## 11.1 Complete Suite 0.1

### M7：系统壳、Nginx、Gateway 与 IAM

M7 已按 [下一阶段行动方案](./next-stage-action-plan.md) 的 M7-001 至 M7-006 严格顺序完成。契约、独立 IAM、统一入口、Contract／Money 双 Provider、历史 Actor 映射、同一登录双应用和当前组合 D1 交付均已验证；D1 还通过了应用开关、单侧故障、升级前备份和销毁卷后的整机恢复。

- 建立版本化 Suite Manifest 草案，声明已安装和已启用应用，不引入独立配置中心；
- Nginx 作为唯一公网入口，提供 HTTPS 终止、域名与静态资源、基础反向代理、连接和请求体约束；Gateway 只监听内网地址；
- Gateway 提供 Suite Manifest 应用路由、基础限流、访问日志、身份 Header 清理和 Access Token 基础验签，Workbench 只显示已启用应用；
- 登录／注册依次经 Nginx 和 Gateway 到达 IAM，租户、账号、密码／验证码、邀请激活、会话和令牌仍由 IAM 拥有；
- Gateway 删除客户端身份 Header 并只从已验证签名上下文派生内部信息，业务应用仍复验凭证和自己的业务授权；
- 完整组合固定经过 `Nginx → Gateway`，单应用开发可以直接访问应用 API；
- 完成租户建立、首位管理员、邀请、激活、登录、注销和密码重置的最小账号生命周期；
- 匿名注册默认关闭且不能由缺省配置自动开启；
- 定义不可为空的版本化 IAM 上下文、可信 Actor 和能力凭证；
- Contract 与 Money Flow 分别发布版本化 Capability Manifest，不识别客户岗位、页面、菜单或按钮名称；
- 定义带来源、revision／有效时点和稳定引用的 Business Fact Snapshot；Project、People & Groups 只提供事实，不能签发权限结论；
- 由 IAM 内受信授权模块实现第一版 Policy Provider 端口，把 IAM 授权、Capability Manifest 和业务事实收敛为目标应用原生的短期签名 Capability Credential；不强制增加独立微服务；
- 服务间读取版本化事实或验证稳定引用时，使用独立 Ed25519 工作负载身份向 IAM 交换请求绑定、一次性、最长一分钟且精确到资源的 Capability Credential；不配置长期 Bearer Token，不复用用户 Session，不因密钥轮换重启业务进程；
- 业务请求保持 `Browser → Nginx → Gateway → Target App`，不得为了权限形成 `IAM → Project／People & Groups → Target App` 的逐请求同步链；
- 目标应用复验 Credential，并结合资源状态、revision 和领域不变量返回且服务端执行 `allowed_actions`；
- 正式 IAM、本地模板和测试提供者通过同一契约测试；
- 迁移现有本地身份时保留历史 Actor 和审计，不制造万能管理员旁路；
- 自动化发现没有授权声明的新受保护端点。

NebulaAuth 只作为候选参考：可以借鉴 Auth／Gateway 分工、Token 生命周期与内部端口隔离；不直接采用其五服务＋Redis＋Registry 最低部署、URL 认证等级、默认开放自动注册或 `user/admin` 粗粒度模型。进入编码前先完成令牌、Cookie、密钥、撤销、IAM 故障、限流和重放威胁模型。

### M8（已完成）：People & Groups 最小闭环

- 建立最小 Person、Group 和带有效期的 Membership，成员加入、角色变更和移出保留历史；
- 配置稳定 Group Type、Membership Role 和 Capability Definition，同时保留 Group Purpose 自由文本；
- 提供版本化 GroupRef、PersonRef 和 Roster Snapshot，业务应用不直接读取本应用数据库；
- 通过公开契约可选关联 IAM Subject，不读取或写入 IAM 内部表，组内角色不自动形成 IAM 权限；
- Project／Contract 拥有具体 Assignment，Budget／Money Flow 拥有预算和资金；不把这些字段放入 Group；
- 不把密码、会话、角色授权、工资、奖金、考勤、绩效、招聘、组织树或技能矩阵放入 v1；
- People & Groups 停机或未安装时，IAM 登录和两个既有业务应用继续按自身边界工作。
- 实际退出证据：独立测试／构建、真实 PostgreSQL 历史与并发测试、正式 IAM 与本地模板、D1 单次登录、应用开关、故障隔离及销毁卷后的四数据库联合恢复均通过；详见 [M8 行动方案](./m8-people-groups-action-plan.md)。

### M9：Project Workspace 最小闭环（J1 独立业务切片已完成）

- 建立项目身份、基础资料和外部资源类型化关联；
- 从项目页面调用 Contract 与 Money Flow 的正式 API，不直接写入对方数据库；
- 目标应用停机时明确显示失败或待处理状态，不伪造成功；
- Project 停机或未安装时，Contract 与 Money Flow 继续独立使用。

J1 已于 2026-09-25 交付独立 Go module、PostgreSQL 18 migration、八个 v1 API、模板／正式 IAM、People／目标资源 Reference Validator、不可变历史、Dockerfile 与 Compose。J1-A01～J1-A18 已通过单元、HTTP、真库、系统、组合与容器冒烟验证。Contract、Money Obligation／Transaction、Evidence 和 Design 的 reference-read 已补齐，并已进入同一数据集 J1～J4 本地组合。2026-09-26 Project 又接入 D1 Suite Manifest、镜像、独立数据库、首管授权、联合备份恢复和 `/project` Gateway 前缀；直接 Resource Grant、首条 Assignment Business Fact Policy、单项撤权与结束 Assignment 后拒绝新凭证均已通过。Project→People 引用验证已经改为自动工作负载凭证：一分钟精确资源凭证自动获取／续期，密钥轮换不重建 Project；IAM 或 People 停机时新 Assignment 失败而既有事实保持可读，恢复后无需重启。下一切片是 C1 多角色／多资源矩阵，不把三个稳定 Project 角色键冒充完整 OA 权限模型。

J3 已完成 Design Operations 独立业务切片：独立拥有 `design-v1` Work Item、四个固定 Stage、适用性、Evidence 成果引用和不可变历史，只引用 Project 的 active `production_owner` Assignment 与 Evidence File Version。产品规格、技术方案、四个模型、十个 v1 API、运行时、Project／Assignment／Evidence reference-read 和 J3-A01～J3-A20 自动化证据均已落地；Demo Validation 2 的六个 Level A 独立 Demo、Level B 高保真 J1～J4／幂等重放／五来源隔离、两类可裁剪子集、首次实际组合变化的定制化六层账目和 `v0.1.0-demo.11` 无源码可移植开发包也已通过。同一归档可按单系统、子组合和完整 OA 三个层级验收，并在变更目标机前先做只读 Ubuntu 预检。Design 继续保持独立责任域；完整入口与真实公司 Ubuntu 仍单独验收。

### M10：Evidence Service

- `M10-001` 已完成：先建立业务无关 File Transfer Core 与 Evidence v1 控制面契约，覆盖浏览器 multipart、进度、取消、幂等 complete 重试、下载／预览与身份范围缓存；该组件不等于服务应用；
- `M10-002` 已完成工程内验证：独立 Evidence Service 进程、数据库、migration、版本化 API、IAM／模板鉴权、本地文件／阿里云 OSS Storage Provider、清理、独立发布和备份恢复已经实现；
- `M10-003` 已完成工程内验证：Evidence 作为可选技术应用接入 D1 的 Suite Manifest、IAM、Gateway、Nginx 本地数据面、应用开关和联合备份恢复；关闭时控制面和数据面同时消失但数据保留；
- `M10-004` 已建立迁移规格并开始只读预检：每条旧 Contract Evidence Version 确定性映射为一个 Evidence File Version 1，原 Document／版本序号／业务审计留在 Contract；写入迁移必须等待预检、备份和回退门禁通过；
- 业务应用只保存 File ID／Version 与业务含义，不把 OSS SDK、长期密钥、分片和签名 URL 嵌入业务应用；
- OSS 采用上传会话、对象键级短期 STS、浏览器 multipart 直传、幂等 complete 和短期签名 URL 下载／预览；OSS 文件字节不经过 Nginx、Gateway 或业务 API；D1 本地 Provider 的字节经 Nginx 直达 Evidence、绕过 Gateway；
- complete 复核对象大小、服务端媒体类型和摘要；过期／取消／失败会话与孤儿对象可以扫描、重试和审计；
- 后续独立迁移切片负责迁移 Contract 当前本地 Evidence，保留 File ID、版本、摘要、业务引用和历史审计，不制造丢失或重复文件；
- 验证上传失败、数据库提交失败、越租户／越 Object Key、下载校验、备份恢复和 Provider 切换；
- 单应用独立 release 可以捆绑 Evidence Service 本地模式，但不得回退为业务应用内嵌存储实现；
- OA 的 OSS 客户端和存储代码只作为候选参考，复用前完成许可证、依赖、安全和契约审计；
- 不把通知、字典、审批或客户业务规则顺手放入“基础”应用。

### M11：Complete Suite 0.1 与 Deployment Profiles 验收

- Nginx 只把应用请求交给内网 Gateway；Gateway 和 Unified Workbench 按同一 Suite Manifest 控制路由、菜单、进程和连接器；
- 应用关闭时保留数据，重新启用前校验版本、migration、健康状态和权限目录；
- Suite Manifest 只决定安装／启用哪些应用，Deployment Profile 只决定节点放置、副本和数据设施；切换档位不产生业务代码分支；
- D1 Compact 在一台 ECS 上承载完整组合，一个 PostgreSQL 实例内仍保持各应用独立数据库、账号、migration 和恢复边界；
- D1 不依赖 Kubernetes、服务注册中心、Redis、消息队列或第二台服务器；
- D2 Data Split 在两台 ECS 上分开应用和 PostgreSQL，数据库只接受应用安全组的私网访问；
- D3 Role Split 在三台 ECS 上分开 Nginx／Gateway 边缘、内部应用和 PostgreSQL，节点间只走 VPC 私网；
- D4 HA 使用冗余流量入口、至少两个可健康摘除的无本地真相应用节点和经过验收的高可用数据层；
- D1～D3 明确是单点拓扑，只验证故障影响与数据不损坏；D4 才验证自动摘除、切换和业务连续性；
- 所有服务端点由配置注入，同一镜像和 migration 可以在各档运行，不在业务代码中硬编码 `localhost`、固定 IP 或机器名；
- 完成“企业与账号 → 人员与编组 → 项目引用组 → 合同 → 合同外义务／资金 → 核销 → Evidence”的演示闭环；
- 证明关闭 Project 或 Contract 后其余已启用应用继续工作，且关闭不删除数据；
- 完成各档安装、升级、资源占用、节点故障、离机备份和恢复验收；完成 D1 → D2 → D3 → D4 迁移与回退演练，保持 ID、revision、摘要、审计和业务引用。

M7 至 M11 只定义方向和退出含义。每个里程碑编码前仍要建立独立规格、接口契约和可执行验收，不因目录名称预先生成空服务。Reimbursement、Bonus／Compensation 和会计应用等待真实案例，不属于 Complete Suite 0.1 的伪装完成项。

### Evidence Local／OSS 转换（BF-081，当前部署切片）

问题与结果：直接替换 Provider 会使存量 Local 引用不可读 → Evidence 内部兼容两种位置，新上传显式选择 Primary，历史事实和业务引用不变。先验收隔离副本的双读、Local-primary 回退与混合空目标恢复，再固化专属出口／IMDS 和重启边界、独立 OSS 字节保护，最后完成正式公网浏览器链。主环境在这些门禁关闭前保持 Local。此项不新增小系统，不代替 Web 静态分离或 C8／OA 操作链验收。

### 标准 Web 前端产品化（BF-080，远期目标）

问题与结果：每个低定制客户都必须先制作一套专用页面，交付成本仍高 → 提供与业务 Base 兼容的标准 Web 制品，客户选装、配置后即可完成已支持业务，同时让少量差异和完全定制前端各有受控位置。此方向不以当前参考 OA 页面、Go Workbench 或新建 OSS Bucket 冒充完成，也不改变正在收敛的 C8 P0 和真实业务操作链优先级。

按证据分批推进，不把“静态资源迁出”和“完整前后端分离”合并宣称：

1. 先对 Workbench 的 CSS／JS／图片做有界静态交付实验：版本化静态文件独立发布到网页 Bucket，验证域名／HTTPS、缓存、旧版回退和不重建业务镜像；服务端 HTML、Suite／会话与业务入口保持现状。这只证明静态交付链。
2. 另立标准前端规格和安全设计：把登录后当前 Subject、可见应用、CSRF／会话、来源状态与服务端 `allowed_actions` 经版本化接口供给客户端；不得在 OSS 对象或浏览器脚本内放入私有页面数据、长期凭证或授权判断。先完成一个可选装小应用的真实创建／查询／撤权／恢复旅程，再逐领域扩大覆盖。
3. 在 BF-019 当前入口基线与公开 OSS 静态入口的关系被显式决策后，验证标准 Web 独立发布、最小组合和不同 Suite 裁剪、API 兼容、权限负例、浏览器体验、回滚和受控轻定制；完全客户定制前端再运行同一契约／权限验收，不复制后端或维护客户核心分支。是否接 CDN、如何处理同源、Cookie／CSRF 与缓存均在该规格中定案。

标准前端产品化需要真实低定制客户验证“装好就能用”、交付时长与升级成本；一套 OA 参考界面或单次静态部署不足以证明商业目标成立。

## 12. 全程质量门槛

每个里程碑都要按风险补齐以下检查，而不是最后集中补测试：

### 领域正确性

- 金额精度和币种约束；
- revision 与并发冲突；
- 不可变事实和受控纠错；
- 多对多核销与余额不变量；
- 历史时点重放。

### 工程稳定性

- 数据库迁移测试；
- API 契约测试；
- 权限边界测试；
- 导入幂等测试；
- 备份恢复演练；
- 独立启动测试。

### 边界约束

- 禁止跨应用读取内部表做正式业务判断；
- 禁止共享领域实体和 ORM 模型；
- 禁止客户岗位、行业字段和页面动作进入公共契约；
- 禁止汇总字段成为第二真相源；
- 禁止可选业务应用或外部基础设施成为启动前提；IAM 契约由正式或本地模板提供者强制满足。

## 13. 优先级

### P0：必须先完成

- M0 工程基线；
- Contract Ledger 独立闭环与 v1；
- Money Flow Ledger 独立闭环与 v1；
- 核心领域不变量、权限、审计、迁移和备份恢复。

### P1：核心稳定后完成

- M7：Nginx 公网边缘、Suite Manifest 草案、Gateway／Workbench 最小系统壳、IAM 账号生命周期、版本化上下文、能力目录和现有本地身份迁移；
- M8：People & Groups 最小闭环；
- M9：Project Workspace 最小闭环；
- M10：独立 Evidence Service、本地文件／阿里云 OSS Provider 与 Contract 现有 Evidence 迁移；
- M11：可配置应用开关，以及 D1 一台、D2 两台、D3 三台和 D4 高可用多节点的 Complete Suite 0.1；
- Contract → Money Flow 连接器；
- 批量导入导出增强；
- 安装升级自动化；
- 三类装配场景验证。

### P2：有真实客户证据后再评估

- 审批适配器；
- 客户专用 SSO 和 IAM 提供者适配；
- 外部对象存储；
- 银行流水和发票平台适配；
- 电子签章、OCR 和智能抽取；
- 行业专项扩展。
- Reimbursement、Bonus／Compensation 和会计应用。

## 14. 主要风险与控制方式

| 风险 | 早期信号 | 控制方式 |
| --- | --- | --- |
| 再次平台化 | 核心功能出现未声明、无独立价值或无法验收的公共服务依赖 | 只接受有稳定使用证据和独立 A→B 的应用；依赖写入契约、Suite Manifest、故障预算和交付验收 |
| 过度建模 | 长期讨论对象但没有可操作闭环 | 每阶段先交付纵向用例，再扩模型 |
| 客户污染内核 | 出现具体岗位、行业费种和客户页面字段 | 放到配置、策略、扩展或客户装配 |
| 过早共享 | 为消除少量重复建立公共框架 | 允许局部重复；复用稳定后再抽取 |
| 双重真相 | 业务明细和汇总都能手工修改 | 汇总只做可重建投影 |
| 集成脆弱 | 同步失败导致两个系统都不能工作 | outbox、幂等、重试、过程状态和人工对账 |
| 历史被覆盖 | 合同或金额修改直接更新旧记录 | revision、调整、冲销和反向事实 |
| 首版范围失控 | 同时加入审批、项目、会计和行业模块 | 严格执行明确延后清单 |

## 15. 计划变更规则

出现以下情况时才调整里程碑范围：

- 新证据证明核心业务事实模型错误；
- 独立运行门槛无法满足；
- 真实客户场景重复出现同一稳定需求；
- 安全、数据恢复或合规风险要求提前处理；
- 当前技术方案不能支撑可重复交付。

单个客户的页面偏好、岗位名称或一次性流程不直接改变核心计划，而是进入客户装配候选清单。

任何计划变更还必须说明它修正了哪个上层 A→B。只能说明页面、字段、技术方案或“成熟系统一般都有”，但不能说明问题 A、结果 B 和验收证据的内容，不进入里程碑核心范围。

已经接受的计划变更必须同时登记到 `docs/decisions.md` 并引用稳定决策 ID；如果取代旧决策，保留旧 ID、状态和指向关系。只更新讨论稿或单份计划文件不构成稳定决策变更。

## 16. 下一步

2026-10-06 边界基线补充：按 BF-111 建立[小系统边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。后续每个 OA 纵向切片除页面和业务验收外，必须同步记录 Project／Production／Contract 的事实变化、依赖变化、兼容升级、局部失败和证伪条件；静态隔离不计作稳定边界成果。初步设计 BF-112 和施工图 BF-113 已依次形成 v2→v3→v4 的同一 Work Item 兼容证据；当前下一功能是结构不同的“变更／竣工”，先固定两套原 UI 和完成语义，再决定是否扩阶段定义，不以复制施工图代替建模。

M0 工程基线、M1 合同最小纵向闭环、M2 Contract Ledger v1 和 M3 Money Flow 最小纵向闭环均已完成。M4 已完成十二个实现切片：`MF-M4-001` 以完整 Reversal 纠正 Allocation，`MF-M4-002` 在 Allocation 净额为 0 后完整冲销错误 Cash Transaction，`MF-M4-003` 以 Adjustment 连续推进 Obligation revision 并保持历史 Allocation 可解释，`MF-M4-004` 以独立 Remission 记录已生效减免并纳入统一余额约束，`MF-M4-006` 以显式 Closure 冻结零余额义务并形成 `closed` 操作终态，`MF-M4-008` 用 current open 正余额形成按方向／币种隔离且可下钻的当前账龄投影，`MF-M4-009` 以单调 recorded-time 重放单条 Obligation 的历史余额，`MF-M4-011` 从现有事实派生按本方主体与币种隔离的六项当前经营指标并逐项下钻，`MF-M4-012` 以本方主体为闭合边界生成固定 ZIP／CSV 台账并逐文件校验，`MF-M4-013` 以同一数据库 snapshot 形成可校验 full backup 并恢复到全新空库，`MF-M4-014` 用版本化 release bundle、显式 schema 门禁和单事务 migration 完成独立安装升级，`MF-M4-015` 以独立 Expectation 历史和 current outstanding 形成预计现金排期。`MF-M4-005` 坏账、`MF-M4-007` Billing Document／Billing Link、`MF-M4-010` 真实退款／退回款和通用导入均完成边界审计后延期，不以错误模型换取表面进度。下一阶段按以下顺序继续：

1. `MF-M4-010` 已确认真实退款是引用原资金的第二次反向资金移动，但参考材料没有定义它对既有 Allocation、Obligation outstanding 与 Closure 的影响，因此审计后延期；
2. `MF-M4-011` 当前经营收支汇总与下钻已经完成：只从现有 Obligation、Cash Transaction、Allocation／Reversal、Remission 与 Cash Reversal 派生明确命名的 current 指标；
3. `MF-M4-012` 可校验台账导出已经完成：一个本方主体的闭合事实集合从同一快照进入固定 CSV，manifest 保存逐文件行数、字节数和 SHA-256；
4. `MF-M4-013` Money Flow 可验证全量备份与恢复已经完成：完整 PostgreSQL、非 session 逐表计数、dump 摘要、空库门禁和恢复后业务验证形成闭环；
5. `MF-M4-014` Money Flow 独立安装升级交付已经完成：三二进制统一 build identity、SHA-256 release bundle、显式 schema 状态、`verify` 启动、单事务 migration 和升级前备份恢复路径均有实际演练；
6. `MF-M4-015` 当前预计现金排期已经完成：预计日期独立于到期日，预计金额只取查询时 outstanding，未设置日期的 open 正余额单列为未排期；
7. M4 剩余范围审计已经完成：预收／预付与超额由 unallocated 事实安全承载但不赋予会计分类；通用导入与坏账／票据／退款等待真实规则，M4 退出门槛对未启用导入不作虚假要求；
8. `MF-M6-001` Money Flow 全新环境 release 验收已经完成：实际 bundle 在随机空库完成摘要／内容校验、迁移、`verify` 启动、核心业务、v2 导出和无 bootstrap 重启；
9. `MF-M6-002` Money Flow clean-room 备份恢复验收已经完成：实际 bundle 从在线随机源库生成严格全量备份，经 whole-ZIP 摘要交接到随机空目标；旧会话失效、原账号重登、事实保持、继续写入、非空目标拒绝和损坏归档拒绝均通过；
10. `CL-M6-003` Contract clean-room release 验收已经完成：实际 bundle 从随机空库和新 Evidence 根完成安装、合同文件上传下载、在线联合备份、空目标恢复、重登、原字节下载和继续写入；三类跨资源失败门禁均通过；
11. `M6-004` 双 release 同主机组合安装已经完成：同一浏览器四 Cookie 共存，交替单侧停机／无 bootstrap 重启时另一侧健康、会话和写入不变，数据库与 Evidence 没有交叉；
12. 两个台账已经收敛到强制 IAM 契约、稳定业务能力目录和正式 IAM／本地模板可替换 Provider；后续不再按客户岗位扩张本地身份，具体组织角色范围等待 M8 与真实第二用户；
13. 每个实现功能使用真实 PostgreSQL 做纵向与并发验证，并在另一个应用停止时证明当前系统可独立完成；
14. 只有出现真实 Contract → Money Flow 业务流动时才进入连接器契约和联调矩阵；在此之前不制造空连接器；
15. `M6-005` 三类参考装配场景边界审计已经完成：设计单位、资质施工综合体和小型咨询服务公司的已验证能力、条件假设、明确缺口与待客户确认事项已经分开；没有修改核心表、生成客户定制代码或伪造跨系统业务流；
16. `M6-006` 可交付性退出缺口审计已经完成：安装、升级、恢复和同机故障隔离有直接证据；Money Flow 多用户、性能线、通用导入和客户装配属于外部待验，不用无事实代码追求形式关闭；
17. `M6-007` 双 Release 同主机运行手册与操作复演已经完成：独立配置、迁移、启动、双登录、单侧重启、分别备份／恢复／升级和禁止项已形成操作者步骤；实际 `test.3 + test.5` 组合及两套 clean-room 备份恢复验收再次通过；
18. `M6-008` 正式 Release 溯源门禁已经完成：两个构建脚本共用 development／official 模式；official 只接受干净 Git HEAD。首个本地基线 `cdc7bc8…` 的两个 `v0.1.0-baseline.1` 候选包通过六二进制 identity、组合与两套恢复验收；
19. M6 工程内可验证范围到此收口。下一步必须取得至少一种外部输入：真实新操作者复现记录、数据规模与 SLO、带纠错期望的导入样例、第二用户与组织授权模型、真实客户装配验收，或真实 Contract → Money Flow 业务流；取得前不制造客户功能或空连接器；
20. 后续每个实现仍需通过全仓质量门禁、竞态测试和 `GOWORK=off` 独立测试／构建后再更新进度。

根据第 8 至 15 轮已经确认的新架构约束，完整应用方向仍按 M7 Nginx／Gateway／IAM → M8 People & Groups → M9 Project Workspace → M10 独立 Evidence Service 与 OSS → M11 Complete Suite 0.1 与 Deployment Profiles 推进。M7 与 M8 已完成并验证；用户明确确认“先组件、后应用”后，M10-001、M10-002 与 M10-003 已依次完成公共组件、独立服务内核和 D1 装配，没有预建 M11 空骨架。真实 OSS 核心浏览器链已于 2026-09-22 通过。

产品验证的阶段一目标是：以现有完成度较高、定制化明显的 OA 为参考产品，验证能否用独立小系统和正式装配组件重组出一套可用 OA。[全景能力与核心旅程审计](./specs/assembly/oa-reference-capability-journey-audit.md)已经区分参考行为、现有覆盖、明确缺口和待确认事实。J1 项目立项、J2 经营合同与回款、J3 设计生产交付、J4 资料追溯已经确认为首个组合集合；六应用／六数据库已经完成该集合、幂等重放和单引用服务故障隔离。

已完成的继续顺序为：第一，Level A 最终总门禁实际输出 `LEVEL_A_STATUS=passed`，六套独立 Demo 的运行门禁为 6/6，Contract、Money、People、Evidence、Project、Design 六项能力簇全部为 complete。第二，Level B 在同一数据集完成高保真 J1～J4、历史纠错、幂等重放与五来源逐项停机隔离。第三，从最小组合到高保真组合的实际变化完成稳定内核、配置、策略门槛、类型化扩展、连接器拒绝、客户装配和客户专项反证的六层账目，没有客户核心分支、共享表或客户 migration；设计交付与商业台账两个方向相反的子集也在未选应用真实缺席时通过。第四，Customer Scenario Evidence v1 把该账目固化为 `reference_product` 基准，并提供不会冒充已通过的第二真实客户草稿与失败关闭校验。第五，`v0.1.0-demo.11` 目标无关开发包从同一归档依次复现六个独立产品、两个子集和同一高保真全量基线，并以只读预检把公司 Ubuntu 目标机条件收口。第二真实客户和 OMS 对照继续提供跨场景证据，但不再替代当前 OA 完整体主线；下一轮正式参考应用保持待选。M5 自动连接器、通用审批、奖金、开票等没有稳定规则的内容不能因“高保真”被猜测实现。详细标准见 [Demo Validation 2](../arckit/spec/assembly/demo-validation-2.md)、[客户场景边界证据](../arckit/spec/assembly/customer-scenario-evidence.md)、[阶段一参考 OA 小系统重组验证](./specs/assembly/oa-reference-stage-1.md)、[参考 OA Demo 规格](../arckit/spec/assembly/reference-oa-demo.md)和[小系统群产品边界验证](./specs/assembly/product-boundary-validation.md)。

OA 完全体是当前下一阶段产品与架构验证主线，但不替代公网入口和第二客户两条独立证据轴。C1～C6 已完成当前纵向切片，且 C6 正式 connector 已由连续主数据集关闭；C7 已完成领域／真库、受保护入口、真实来源、人类工作区、正式 D1、未选缺席、联合恢复、恢复后继续写入及 C7→Money 直接跨进程建议。C8 已完成静态责任边界和若干正式来源链。C7 正式 D1 connector／建议引用恢复、单类型最小组合和完整双租户仍待验证；Money 完整 D1 Party 消费链已由连续 C3 关闭。每次增量都必须保留 Level A／B 和可裁剪子集回归，并把变化唯一归入 Base、策略、类型化扩展、连接器、客户装配或客户专项。最新证据见[连续 C6 验证](./reports/oa-full-suite-continuous-c6-validation-20260930.md)、[C8 边界审计](./reports/oa-c8-material-statistics-personal-work-boundary-audit-20260929.md)、[C7→Money 应付建议实验](./reports/oa-c7-money-payable-suggestion-experiment-20260929.md)与[C7 D1 正式装配与恢复实验](./reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)；全业务范围与未知项见[OA 完全体业务重组验证](../arckit/spec/assembly/full-reference-oa-validation.md)，权限专项方法见[OA 完整体权限架构验证方案](../arckit/tech/assembly/oa-authorization-validation.md)，推进与报告门槛见[OA 完整体架构可行性验证计划](./oa-full-scope-validation-plan.md)。

2026-09-28 C5 D1 更新：上述总览中关于 C5 未选缺席／装配恢复待办的表述已被新证据取代。Results 已通过正式 Suite Manifest／Workbench／Gateway、独立数据库／工作负载身份、未选缺席、联合备份、销毁卷恢复和恢复后复验；当前仍缺真实批复→审计→Project 完成确认、撤权／双 Tenant、真实浏览器与真实 Ubuntu。证据见[C5 D1 正式装配与恢复实验](./reports/oa-c5-d1-assembly-recovery-experiment-20260928.md)。

C3 Billing 已完成上游精确引用、消费侧工作负载 resolver、完整生命周期 HTTP 和可配置运行进程。创建、列表、详情、纠正、登记无效、Evidence 管理、历史与精确工作负载引用由 Billing 原生 IAM／模板 Provider 保护；真实 PostgreSQL 18 已验证 Project Scope 列表、相邻 Project 来源纠正拒绝、revision 1→4、金额服务端收敛、Evidence 精确引用与登记无效后有效汇总归零。Linux AMD64 非 root 镜像、pull-only release、Suite Manifest、Nginx→Gateway→IAM 正式入口、精确工作负载凭证、Party 故障零写入／恢复重试及联合空目标还原也已通过。人类工作区已进入 Workbench，服务端 `allowed_actions`、金额撤权／恢复与历史脱敏、盲写金额拒绝、409 和 503 恢复行为经过正式 D1 入口验收。Project 的经营链路进一步按 active Resource Link 并行组合四类来源，独立金额 Capability、目标最终授权、局部降级、角色／撤权／停用和 Contract／Money／Billing／IAM 故障矩阵均已通过；本地 30 次采样 p50/p95 为 13/220 ms。下一业务切片进入 C4，多 Tenant、真实浏览器最终点击、Billing-only 和真实 Ubuntu 作为 C3 补证轴保留。证据见[C3 Billing 生命周期与运行时实验](./reports/oa-c3-billing-lifecycle-runtime-experiment-20260927.md)、[C3 Billing D1 交付与恢复实验](./reports/oa-c3-billing-d1-delivery-recovery-experiment-20260927.md)、[C3 Billing 人类工作流实验](./reports/oa-c3-billing-human-workflow-experiment-20260927.md)和[C3 Project 经营链路 D1 实验](./reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。

C4 已把“多类型生产”从页面功能收缩为可证伪的领域边界：一个 Production Operations 数据所有者，现有 `design-v1` 原位兼容，`consultation-v1` 作为信息增益最高的第二类型；拒绝每类型复制应用、Project 内置阶段和用户可编辑通用工作流。C4-A01～A14 当前本地轮次已通过：除了定义、迁移、双类型生命周期和 canonical 权限，consultation-only 最小组合、双向未选缺席、六应用 Level A／完整 OA Level B 回归，以及 People＋Project＋Evidence＋Production 的 D1 Suite Manifest／Workbench、正式入口、依赖故障恢复和联合空目标恢复均有直接证据。86 条权限生命周期断言与真实浏览器又补齐 Grant 撤销／恢复、账号停用／启用、Assignment 结束和页面动作收敛。后续按 [C4 多生产类型责任边界审计](./reports/oa-c4-multi-production-boundary-audit-20260927.md) 保留双 Tenant 与真实 Ubuntu 为环境补证轴，主业务线按证据进入 C5。

C5 已把“批复审计页面”收缩为新的窄域所有者与 Project 完成协作：Approval & Audit Results 独立拥有外部结果、精确金额、来源快照、Evidence 引用和不可变纠错，Project 只按版本化规则纯读取评估，不把来源状态复制成第二真相。参考 OA 已发现“审计页面引用批复、后端却重读合同”“只要两类记录存在即完成”“GET 写项目状态”三类反证，目标实现明确拒绝照搬。第一实现收缩完成独立领域模型、migration 和 PostgreSQL 18 真库闭环；第二收缩完成版本化 HTTP、目标 IAM、金额脱敏和精确 adapters；第三收缩完成 Results 人类工作区和 Project `reference-oa-c5-v1` Completion Profile；第四收缩完成正式 D1 装配／裁剪与联合空目标恢复；第五收缩以 74 个入口断言完成真实独立进程业务旅程、局部故障恢复和纠正 Grant 生命周期。C5 完整双 Tenant、账号停用与真实浏览器仍按独立权限轴补证；主业务线进入 C6 成本外委责任边界，C7 奖金不与 C6 混做。

C6 已完成成本与外协责任边界审计并接受 BF-059。参考 OA 的宽 `FinancialRecord` 混合奖金、成本、收款、票据、专家费和外协付款，使用浮点金额、可编辑删除并按供应商名称级联删除，构成直接反证。目标建立独立 Cost & Outsourcing，在一个部署边界内分别实现 Cost Entry 与 Outsourcing Engagement 两个聚合；Project／Party／Contract／Evidence 只经精确引用使用，Money 独立确认 payable suggestion，Billing 独立登记票据。C6-A01～A18 已固定；领域核心、受保护 HTTP／IAM、人类工作区、独立进程和正式 D1 装配／缺席／联合恢复已经依次通过。BF-064 关闭 Party 切换前置项；BF-065 又完成 Payable Suggestion：C6 与 Money 使用独立数据库和版本化 HTTP，接受恰好创建一个 Money 自有义务，拒绝创建零个，停机／恢复、重放、不可变历史和 Obligation 已提交而 intake 未更新的部分失败均可恢复。连续 C6 进一步在同一 Tenant／Project 通过正式 Gateway／IAM connector、目标停机恢复、24 条首轮与 15 条完成态重跑断言，并关闭 action namespace 与 exact suggestion scope 两项真实契约缺陷。下一主业务切片进入连续 C7；C6 的双 Tenant、跨进程撤回、Money／Billing 只读投影和 Project Completion 适用性继续作为独立补证轴，C7 奖金不并入 C6。

C7 已完成责任边界审计并接受 BF-066；BF-067～BF-070 依次完成独立领域／PostgreSQL、受保护 HTTP／IAM、真实来源组合和人类工作区，BF-071 完成正式 D1，BF-072 又完成 C7→Money Payable Suggestion 直接跨进程旅程。经营／生产奖金共享 Pool→Calculate→Allocate→Publish→Supersede／Withdraw 生命周期，但分别冻结 Contract Settlement 或 Money Receipt Set、Person／Assignment 与策略 revision；`reference-oa-bonus-v1` Overlay 使用整数最小货币单位、整数基点和精确 half-up。应付建议冻结 exact Allocation／Person 快照，由 Money 经不可变人工 Person→Party 映射后独立接受或拒绝；目标停机恢复、接受唯一义务、拒绝零义务、重放和源撤回无级联已通过 11 条断言。下一业务轮进入 C8；正式 D1 connector／建议引用恢复、经营／生产单类型最小组合、双 Tenant 和真实 Ubuntu 保持独立补证，不把任一项提前算作通过。

C8 已完成静态责任边界审计并接受 BF-073。Project Workspace 负责 Project Scope 和 Resource Link，Evidence 负责元数据、版本与下载授权，各领域应用负责指标和工作项，Workbench 只聚合当前 Subject；不新增持久化报表库或统一可写待办。统计不得把不同来源金额或百分比直接相加，工作项不得由 Workbench 推断，局部失败必须保留来源状态。首轮后端实现已按低资源顺序拆分 Evidence metadata／download 能力并建立 direct Evidence Resource Link 的 Project 资料只读 API；随后一个 PostgreSQL 容器、真实 IAM、Evidence 与 Project 三进程证明精确 workload 换票、相邻 Project 拒绝、stale revision、停机分段降级、恢复和零残留。资料页面现已实现 Evidence Link 建立、元数据状态和独立下载授权入口，并通过零容器单元验证；真实浏览器仍待补。BF-074 固定的个人工作第一条链现已在公司 D1 通过：Production 经 Project Assignment 和 People Policy Fact 把 Person 责任解释为当前 IAM Subject，再发布 current-only `WorkItemProjection v1`；47/47 覆盖相邻 Subject、撤权刷新和来源停机。Money `MetricProjection v1` 也已正式证明统计／金额分权、撤权刷新和源事实只读。之后每批只补资料／个人工作浏览器、间接资料运行链、第二来源或正式完整 D1 连续旅程中的一项，不得同时铺开。

2026-09-29 C8 首个间接来源更新：Business Engagement 已发布 current-only `MaterialReferenceProjection v1`，Project 使用独立来源凭证读取投影，再逐 File 使用 Evidence metadata 凭证组合；两层 revision／digest、关系键和局部失败分别保留。Business Engagement、Project Workspace 与 Access Contract 三组测试以 `GOMAXPROCS=2`、`go test -p 1` 串行通过，覆盖来源拒绝、单 File 失败、drift、错 Tenant／Project／source 和跨 Project 失败关闭，全程未启动 Docker。当前只判定 `c8-indirect-business-material-unit-supported / runtime-and-d1-pending`；下一轮在该链的单容器真实运行、一个 WorkItem 来源或已解锁后的资料页面浏览器验证中只选一项，不并行铺开。

2026-09-29 C8 首个个人工作来源更新：Production Operations 发布稳定 Work Item ID、来源 revision、深链和服务端 `allowed_actions`，只返回当前 Credential Subject 的 active direct Person Owner 工作项；结束 Assignment、相邻 Subject、已完成／取消项、动作撤销和引用来源不可用均失败关闭或按来源降级。Suite Manifest 以可选 `work_item_projection` 声明来源，Workbench 在浏览器中经 Gateway 换取来源短凭证，按来源最多四路有界并发聚合并分别呈现 available／empty／forbidden／unavailable；它不读取令牌、不接受 Subject 输入、不写来源状态。Production、Workbench、Gateway 与 Access Contract 均以 `GOMAXPROCS=2`、`go test -p 1` 串行通过，嵌入脚本语法检查通过，全程未启动 Docker。当前只判定 `c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`；下一批只能从工作项正式 Gateway／浏览器链、一个指标来源或间接资料单容器运行链中选择一项。

2026-09-29 C8 首个类型化指标来源更新：Money Flow 发布 `GET /api/v1/metrics/operating-summary` 和独立 `MetricProjection v1` Schema，把六个 current 经营指标按 Tenant／Ledger Owner Party／Currency、口径版本、观测时间、精确十进制值、摘要和深链输出。`statistics.read` 与 `amount.read` 分权；无金额权时值和摘要同时为 null，缺结构权 403，来源失败 503。Money 全模块与 Access Contract 在 `GOMAXPROCS=2`、`go test -p 1` 下串行通过，全程未启动 Docker。随后 Suite Manifest 与 Workbench 装入首个单消费者，严格验证 source／Tenant／Scope／redaction，按来源分区展示并隔离单一来源失败；Workbench、Gateway Manifest、Access Contract 和嵌入 JavaScript 继续零容器串行通过。之后 `test.11` 在公司 Ubuntu D1 通过正式入口建立非空投影：只有 view 为 403，统计权返回 6 项全脱敏，金额权只在重新签发后显示精确值，撤权后新凭证恢复脱敏；Money 事实摘要前后一致。当前判定提升为 `c8-money-metric-d1-supported / workbench-human-browser-and-second-source-pending`；下一批转向其他已就绪的独立缺口，不同时启动完整 D1。

2026-09-29 C8 公司 D1 入口更新：升级前联合备份 `20260929T113133Z` 的十项摘要全部通过。首个只替换 Project／Evidence 的候选实际暴露旧 IAM 无法解析新 `capability_namespace`，而前向 Project migration 后旧 Project 二进制又按 schema 门禁拒绝启动；因此接受 BF-077 与 ADR-0035，把 Manifest 解析者和 migration 读者纳入发布兼容闭包。随后以低资源串行方式补发 IAM，并依次替换 IAM、Evidence，保留已识别新 schema 的 Project；12 个长期容器全部 healthy，`d1 check` 通过。真实入口只读旅程得到 ready 200、IAM 登录、Project／Evidence 授权 303、12 个 Project、`materials.read`、`/materials` 200 和 Evidence 页面 200，结束后注销 204；本地 Docker 始终为 0 容器。候选 IAM 现新增无数据库 `validate-contracts`，`d1 preflight-upgrade`／`upgrade` 会在备份和 migration 前解析全部 Capability／Assignment／Workload 文档，模拟回归证明失败不会进入备份命令；完整消费者账和 migration 闭包仍待自动化。下一步只补浏览器人类点击、专用 C8 测试资料和非破坏性 stale／权限分权；空目标数据回退复演、间接资料与工作项 D1 仍分批处理，Money 指标正式 D1 已在后续 `test.11` 关闭。

同日 C8 专用资料旅程进一步暴露 Evidence 只能形成 Version 1，导致产品路径无法产生 stale。当前已按 BF-078／ADR-0036 实现同 File 追加版本、最新版稳定引用、显式历史引用和 Suite Cookie 鉴权；真实 PostgreSQL 18.6 集成与低并发测试通过。公司 D1 又以正式 API 完成 `available@v1 → revision_changed@v2 → available@v2`，Project revision 2→4，稳定 File ID 与显式 Version 1 摘要保持不变。最终 `v0.1.0-test.10` 兼容闭包包含 Evidence `test.7`、IAM `test.8` 以及能解析 `metric_projection` 的 Gateway／Workbench `test.10`；升级前备份 `20260929T132146Z` 校验有效，所有 migration 退出 0，12/12 长期容器 healthy，`d1 check` 通过。升级中发现并修复旧 one-shot migration 容器复用和 Evidence 备份恢复错误依赖两项运维缺口。后续 `test.11` 又关闭 Money 指标正式 D1 缺口。C8 仍保持 pending 的范围已收缩为真人浏览器下载／分权点击、独立空目标恢复、间接资料／个人工作正式 D1，以及指标真人浏览器／第二来源，不再把远端多版本、stale 重连或 Money 指标来源链列为缺口。

同日建立 [OA 完整体架构可行性验证报告](./reports/oa-full-scope-architecture-validation-report.md) `0.1-draft`。草案没有把四十多份局部报告直接相加成“完成”，而是按架构主张、C1～C8、最小组合、Base／Overlay、数据／契约、权限、人类体验和真实环境逐项归并直接证据；当前总体结论固定为 `insufficient-evidence`。P0 收口顺序为 C8 公司浏览器专用资料旅程、公司 D1 独立空目标恢复，以及同一 Suite／Overlay／数据集的 C1～C8 连续旅程。

稳定 Base 加 Customer Overlay 只作为当前产品线验证方向。下一次实际客户或 OA 完全体增量同时记录 Overlay 新增内容、Base 兼容修改、客户专项数量、业务 migration、交付改动面和回归结果；任何共享表、现场核心补丁、长期客户分支或无法升级的 Overlay 都使该方向失败关闭。

2026-09-27，C2 隔离 D1 已补齐 Business Engagement 联合备份、销毁专属卷后的空目标恢复、原 ID／revision／历史／Project 与 Evidence 引用保持，以及恢复后正式入口回归和继续写入；随后 95 条正式入口验收又证明 409 不写入、Evidence 503 不产生部分事实、恢复后以新 `command_id` 单次写入，并补齐页面恢复控件与 Project 返回路径；真实浏览器最终点击也已复核。继续扩展到 110 条后，又证明账号停用立即阻止 Session／新签发、IAM 与 Policy 依赖故障失败关闭并清除旧 Cookie、失败不写签发审计、恢复后只签发一份，以及目标有效签名错租户拒绝。此前计划中的“实际空目标恢复”“409／503 自动化故障恢复”“真实浏览器最终点击”和“C2 当前 IAM 故障边界”不再是本地缺口；完整双租户 D1 矩阵和真实 Ubuntu／离机恢复仍分别保留。

阶段最终生成 `docs/reports/oa-full-scope-architecture-validation-report.md`。只有 C1～C8、独立组合、定制变化、权限角色／资源／撤权矩阵、故障恢复、人类可用体验和至少一个真实 Ubuntu 环境都具备直接证据时，报告才能给出 `supported`；缺少公司环境授权且个人 ICP 尚未生效时继续本地工作，但报告只能保留为中期或 `insufficient-evidence`。

2026-09-29，C8 Production 个人工作正式公司 D1 已关闭：People→Project→Production 使用两个版本化读端口和请求绑定 workload credential，不再把 Person ID 误作 IAM Subject；47/47 正式入口证据覆盖相邻 Subject、action/read 撤权刷新、来源停机恢复和事实只读。`test.17` 又以 `20260929T151745Z` 完整备份、全部 migration、13/13 healthy 和 `d1 check` 证明当前选装集合升级。下一收缩顺序保持为：C8 真人浏览器与间接资料运行链 → 公司隔离空目标恢复／恢复后写入 → 同一 Suite／Overlay／数据集的 C1～C8 连续旅程 → 双 Tenant 与最终静态依赖审计；不因个人工作通过而提前进入第二客户。

Contract-only 独立组合仍是独立产品轴的必要验收；真实 Contract → Money Flow 案例仍是连接器启动门禁，但不再要求用户先具备完整行业知识。可以先从参考 OA 的已有规格、代码和行为提取候选流程，只有无法由证据确定的责任、规则和异常处理再交由业务人员确认。Evidence 后续只推进 Contract 迁移、云端剩余负例和生产入口；报销、奖金和会计能力等待各自真实 A→B。

### 当前 D1 云端落地目标计划（2026-09-22）

最小根因：**本地已有完整组合但还不能稳定地在客户可购买的云资源上重复交付 → 形成从空 ECS 到可经 HTTPS 访问、可升级、可离机备份和可恢复的 D1 交付闭环。**

按依赖顺序执行：

1. **宿主机可重复初始化（已完成）**：仓库提供 `plan/apply/check/self-test`，当前 ECS 已通过本地自测、远程检查、重复应用和 17 项复验；
2. **受控镜像供应（自动化已完成，真实 ACR 待验）**：已实现 Linux AMD64 基础镜像复制、BizFoundry 应用构建发布、摘要清单和 D1 `pull` 门禁；ACR 个人版不支持控制面 API，下一步由账号本人在同地域创建实例／密码／命名空间并启用默认私有的仓库自动创建，随后脚本完成 11 个仓库的首次推送和 ECS 拉取；
3. **D1 应用部署**：在受限目录配置 `.env`，使用 OSS 内网 Endpoint、正式 IAM、独立数据库账号与应用开关启动当前组合；
4. **公网入口**：把 `app.xidou.me` 解析到唯一 Nginx 入口，安装已签发证书，验证 HTTP 跳转 HTTPS、Gateway／IAM 链和内部端口不公开；
5. **真实环境验收**：执行 D1 健康、登录、应用开关、单应用故障、OSS 上传下载、升级前备份与恢复后继续写入；
6. **离机恢复闭环**：把备份自动复制到 ECS 之外的受控加密位置，并在空目标完成恢复演练后，才把 D1 云端交付标记为完成。

阶段 2 的云控制面前置条件是明确 ACR 个人版实例、`omsbizfoundry` 命名空间、默认私有的仓库自动创建设置和登录域名；这些值不得根据加速器地址猜测。个人版官方不提供 API，因此不伪造控制面 CLI 自动化；仓库提供 `publish-images plan|publish` 与 `d1 images|prepare-images` 自动完成其余步骤，Registry 密码仍只通过交互登录提供。

M3 不从 Contract Ledger 自动生成义务，也不读取其数据库或领域对象。如果后续出现真实 Contract → Money Flow 业务流动，必须另立 M5 事项，先定义版本化快照或 API、幂等键、失败恢复和联调矩阵。日历工期仍需等待团队容量和首个真实客户证据，避免制造虚假排期。

2026-09-29，完整选装装配缺口在公司 Ubuntu D1 关闭。`v0.1.0-test.20` 同时启用 Contract、Money、People、Party、Project、Business Engagement、Evidence、Production、Results、Billing、Cost 与 Bonus，连同 Nginx、Gateway、IAM、Workbench、PostgreSQL 共 17 个长期服务；正式升级前联合备份包含 15 项受保护材料，checksum 全部有效，所有 migration、17/17 health 和 `d1 check` 通过。过程暴露并修复 Bonus 镜像缺少公共契约 package、IAM 模式错误提前要求 Money 静态 Token 两项真实缺陷。该证据只证明完整选择可重复装配，不替代业务连续旅程；下一顺序固定为独立空目标恢复／恢复后写入 → 同一数据集 C1～C8 连续旅程 → 真人浏览器关键权限路径。证据见 [完整 Suite D1 装配验证](./reports/oa-full-suite-d1-assembly-validation-20260929.md)。

2026-09-30，完整 Suite 的独立空目标恢复／恢复后写入缺口在公司 Ubuntu D1 关闭。来源备份 `20260929T155403Z` 恢复到独立 project、端口和全新卷，17/17 healthy；正式入口验收覆盖启用应用读写、Party 迁移预检、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。Evidence 从 `9/10/10` 增为 `10/11/11`，新备份 `20260929T171025Z` checksum 全部通过。主栈始终健康，隔离栈验收后停止。过程中修正有界备份权限交接、Grant 限流重试、Bonus 相对路径验收和 Contract 镜像／Capability 兼容闭包；接受 release 为 `v0.1.0-test.26`。下一顺序收缩为同一 Suite／Overlay／数据集 C1～C8 连续旅程 → 真人浏览器关键权限路径 → 剩余 connector／双 Tenant；离机跨机器恢复独立执行。证据见 [完整 Suite D1 隔离空目标恢复验证](./reports/oa-full-suite-d1-empty-target-recovery-validation-20260930.md)。
2026-10-28 管理员全权限 P0（BF-279）已发布：已关闭“账号已是管理员但新增甲方／创建项目仍缺权”的 IAM 装配断点。Identity 已增加管理员画像来源、启动／登录／提升对齐和降级定向撤销；Workbench 已纠正管理员说明并更新静态资源版本。Identity 单元／vet、真实 PostgreSQL 18 fresh/repeat migration 与 HTTP 生命周期、Workbench Go／JS／弹窗门禁和 diff 检查通过，临时容器已清理；D1 公网补齐六个 OMS Owner 的多租户校验镜像、完整 `v48` 编排和缺失 migration，并通过 19 容器健康、独立 `check` 与真实管理员浏览器“新增甲方”动作。People 正式职位目录、任职历史、权限模板页面和职位映射属于后续 P1～P3，本项不以职位文本直接授权。[规格](./specs/identity/oms-administrator-access-profile-v1.md)、[验证及发布记录](./reports/oms-administrator-default-access-profile-validation-20261028.md)
