# OMS1 迭代 P9 D1 装配源码门禁报告

日期：2026-10-26

结论：P9 的 D1 选择、下游身份与静态交付缺口已经补齐并通过低资源源码门禁；本次没有启动容器，也没有执行全组合业务、联合备份／恢复或公网部署，因此 P9 尚未完成。

## 1. 审计发现

P2 已把 `Internal Finance Requests` 与 Payroll 放入 D1 profile、数据库、镜像和备份恢复，但按 P5／P7 的正式闭环复核后发现：

- Finance 的 D1 环境没有 Money／Evidence 端点，也没有独立工作负载身份；进程健康时付款和发票实时复验仍可能未接线；
- Payroll 的 D1 环境有 People 端点但缺 Money 端点，工资发放闭环不能由 D1 配置保证；
- D1 允许在缺少上述 Owner 依赖时选择 Finance／Payroll，错误会延迟到业务操作；
- Finance pull profile 没有覆盖新增 key helper 镜像。
- Finance／Payroll 的 Money 交付绑定虽已进入各自整库 dump，但原 D1 联合备份保持 Nginx 开放并顺序导出各 Owner，跨 Owner 写入可能落在不同检查点；升级预备份后短暂重开入口还可能接收不在回滚点内的新写入。

## 2. 已完成修改

- Finance 新增独立 Ed25519 workload key lifecycle 和短期 IAM Credential Source；正式 IAM 模式拒绝静态 Money／Evidence Token。
- Finance→Evidence 只申请 `evidence-service.metadata.read`＋精确 File ID；Finance→Money 只申请 `money-flow-ledger.internal-finance-payment.record`＋精确 Command ID。
- 下游返回 401 时仅失效该 Tenant／Audience／Action／Resource 缓存，换票后最多重放一次；网络或不确定结果继续返回 unknown，由原幂等命令恢复。
- D1 为 Finance 注入 Money／Evidence／IAM 端点、私钥只读 volume 与健康依赖；Payroll 补上 Money 端点和 People／Money 健康依赖。
- Suite 选择在任何镜像构建、停服务或数据动作前强制 Finance→Money＋Evidence、Payroll→People＋Money。
- build／pull、停用清理、示例环境、workload policy、Makefile tidy 与静态组合门禁同步更新。
- BF-245 运行前复核又修正 `FEEDBACK_SDK_URL` 错注入 Bonus、Workbench 缺失的问题，并把项目 115、首发 `apiKey`、URL、HMAC、通知及单／双 Key 轮换改为 D1 状态变化前的成组失败关闭。
- BF-247 让 D1 联合备份先关闭唯一公网 Nginx、等待 0～30 秒的可配置 drain，再开始首个 dump；普通备份成功或失败恢复原入口，升级成功备份保持停写直到 Compose 替换。一致性模式、入口原状态与 drain 秒数写入受 checksum 保护的 `backup-consistency.env`，旧备份保持可恢复。
- BF-248 在同一停写窗口为 IAM 与实际选装 Owner 生成 `database-fingerprints.tsv`：表集合、行数、列／约束／索引／触发器和逐行事实都进入 SHA-256，但原始值不出 PostgreSQL；空目标恢复在业务服务启动前重算并严格比较。IAM Session 数据按原 dump 契约排除，旧备份无该文件时继续兼容。
- BF-249 将新指纹的格式、摘要、唯一数据库集合和 Suite 选择一致性校验前移到 `down --volumes` 之前，避免 checksum 正确但语义畸形的备份先破坏目标环境。
- BF-250 把 public sequence 定义与 `last_value`／`is_called` 加入 schema／事实摘要，补上“当前读取一致但下一次自增写入可能错位”的恢复缺口；PostgreSQL 18 短生命周期实测已证明序列推进改变摘要，mock 也证明仅序列漂移、表事实不变时 Restore 失败关闭。
- BF-251 将 C0 证据收敛为固定闭集并用 SHA-256 清单覆盖健康、镜像、readiness、Manifest、脱敏拓扑、检查日志和数值版本状态；独立验证器与篡改／额外文件负例通过。清单用于发现意外损坏，不冒充发布者签名。
- BF-252 要求运行容器实际 Image ID 与配置引用当前解析 ID 相等，并由证据 verifier 再次交叉核对；mock 标签漂移和“篡改镜像证据后重算 checksum”两类负例均失败关闭。
- BF-253 要求真实 C0 来自正式 D1 release bundle：release metadata、Suite version、pull mode、十九个 digest-pinned image 与运行 Image ID 形成追溯链；P9 C0／verifier／说明也随 release bundle 交付。开发 commit 与 mutable tag 负例失败。
- BF-254 发现并修复同域名双租户入口缺口：Gateway 原先把会话和 Capability Tenant 强制等同于 Manifest 默认 Tenant，现改为信任同一 IAM issuer 的签名 Tenant，并让 Workbench 使用当前会话 Tenant；成功登录同步清除旧应用能力 Cookie。源码回归已过，真实 D1 双租户仍待执行。
- BF-255 增加绑定 C0 的 P9 C1 驱动和独立证据 verifier：同一公网 Origin 下安全创建／复用第二 Tenant，分别验证 IAM 登录、People 能力、Person 稳定重放、列表隔离、交叉 ID 404 与 Workbench 会话 Tenant；密码只经 `0600` 文件、stdin 和临时文件传递。离线 Docker／curl 回归及证据篡改负例通过并进入 release bundle，但真实 D1 尚未执行。
- BF-256 增加绑定 C0／C1 的 P9 C2 驱动和独立证据 verifier：两个管理员经同一 Origin 取得 Payroll 最小能力，以稳定命令把各自 C1 Person 引用保存为独立 Compensation Profile，核对 source revision、整数金额、列表隔离与交叉 404。离线首次／重跑、既有 Grant 冲突、秘密扫描和证据篡改负例通过并进入 release bundle；该阶段不声明法定比例、不生成或发放工资，真实 D1 尚未执行。
- BF-257 增加绑定 C0～C2 的 P9 C3 驱动和独立证据 verifier：Payroll 自有 ops 幂等导入明确非法律依据的六项零比例验收方案，两个管理员经同一 Origin 各生成一张待核对、无需审批且未支付的常规工资表；验证 C1 Person／C2 Compensation／Scheme revision 快照、整数结果、列表隔离与交叉 404。离线首次／重跑、既有 Grant、秘密扫描和两类篡改负例通过并进入 release bundle；未调用发放或 Money，真实 D1 尚未执行。
- BF-258 增加绑定 C0～C3 的 P9 C4 驱动和独立证据 verifier：两名管理员刷新各自 C1 Person 快照，只取得 Finance 本人读写能力，并各自建立一笔无附件但显式确认缺失事实的付款申请；验证 `payable`／`not_required`、账号脱敏、零付款／零 Money、本人列表隔离和交叉 404。报销 Evidence 强制条件没有放松。离线首次／重跑、Grant 冲突、秘密扫描和两类篡改负例通过并进入 release bundle；真实 D1 尚未执行。
- BF-259 增加绑定 C0～C4 的 P9 C5 驱动和独立证据 verifier：两名既有员工分别通过 IAM 正式入口创建或复用一把默认 180 天 API Key，一次性 Secret 只写入证据外的 `0700`／`0600` 恢复位置；验证本人 Key 列表脱敏与隔离、Open API Tenant／Subject／Person／Key 绑定、Webhook 未开放占位及本人调用审计。离线首次／重跑、Grant 冲突、密码／Secret 扫描、摘要漂移、HTTP 矩阵缺失和重算 checksum 后注入 Secret 的负例通过并进入 release bundle；未做业务写，真实 D1 尚未执行。
- BF-260 增加绑定 C0～C5 的 P9 C6 驱动和独立证据 verifier：两个 Tenant 的员工使用 C5 Key 分别上传不同的虚构 PDF 实字节，完成 Evidence `invoice-v1` 校验，以稳定 File ID／Version 建立并提交本人报销；验证正确 Person／Evidence 快照、`payable`／`not_applicable`、账号脱敏、本人列表、交叉 404 和零付款／零 Money。Key 与签名 URL 只进入受限临时 curl 配置。离线字节上传、摘要漂移、HTTP 矩阵缺失和重算 checksum 后 Secret 注入负例通过并进入 release bundle；真实 D1 尚未执行。
- BF-261 增加绑定 C0～C6 的 P9 C7 驱动和独立证据 verifier：两个 Tenant 的授权财务管理员经 Money 公网命令登记验收专用本地兼容 Party，经 Money 自有审计 ops 设置 Tenant 账簿主体及精确 Person revision／digest 收款映射，再由 Finance 确认 C6 报销完整实际付款；验证 Money Obligation／Payment／Allocation 稳定引用、23456 分完整核销、员工 Open API `paid` 回读和交叉 404。全程无直接数据库写入，且证据明确 `party_directory_unified=false`。离线双 Tenant、摘要漂移、HTTP 矩阵缺失和重算 checksum 后 Secret 注入负例通过并进入 release bundle；真实 D1 尚未执行。
- BF-262 增加绑定 C0～C7 的 P9 C8 驱动和独立证据 verifier：两个 Tenant 的既有 C3 工资行分别建立 Payroll Payment Profile，经显式预览／确认同步支付快照并核对，再由 Payroll→Money 按精确净应发金额完成实际发放；验证 `paid`、实付等于核销、零超额、员工本人工资单和交叉 404。完整银行卡号不进入证据；相同稳定支付命令在成功后重跑保持业务结果不变。全程无直接数据库写入，复用 C7 Money 本地兼容 Party 且明确 `party_directory_unified=false`。离线双 Tenant、摘要漂移、HTTP 矩阵缺失和重算 checksum 后完整账号注入负例通过并进入 release bundle；真实 D1 尚未执行。
- BF-263 增加随 release bundle 交付的 C0～C8 可恢复连续执行器：首次按序调用九个阶段；中断后逐阶段通过独立 verifier 才复用并从首个缺失阶段继续；完成后再次核对同一 Compose project、公共 Origin 及每阶段对全部前序 checksum 的绑定。C5 Secret 目录强制位于证据根外，执行器不管理服务生命周期。离线完整制品复用和重算 checksum 后的断链负例通过；真实 D1 尚未执行。
- BF-264 增加显式确认后才可执行的 C9 原项目联合备份／清卷恢复门禁，并在恢复后核对十五库指纹、Evidence 字节、C7／C8 稳定重放和 Open API 审计续写；离线正反例通过，真实 D1 尚未执行。
- BF-265 增加 C10 Feedback V2 真实旅程门禁：绑定 C0／C1／C9，两个 Tenant 各两条会话，没有开发者回复时保持 `PENDING`，回复后核对逐会话未读／已读隔离；离线 pending／resume 与证据负例通过，真实 project 115 尚未执行。
- BF-266 增加随 release bundle 交付的 `run-c0-c10` 与 `verify-run-c0-c10`：总入口复用已有 C0～C8，再执行或复用 C9／C10；C9 仍需显式破坏性确认，C10 仍可在人工回复点安全暂停。最终 verifier 独立核对十一套固定制品、单一 Compose project／Origin、C0～C8 checksum DAG、C9 对 C0～C8 及 C10 对 C0／C1／C9 的绑定。离线完整复用、checksum-valid 断链和未确认 C9 负例通过；执行器不管理服务生命周期，也没有擅自引入未确认的 soak 时长。
- BF-267 增加 C11 真实浏览器门禁、`run-c0-c11` 与十二阶段全链 verifier：两个 C1 Tenant 管理员分别在真实 Chromium 中核对页面身份并点击 Feedback submit／status，SDK iframe 必须来自同一配置跨域 Origin；主 Tenant 撤销 `feedback.use` 后入口隐藏、OMS 导航仍可用，恢复授权后入口返回。低资源双 Origin 夹具实际运行生产 Feedback bridge 并确认最终 Grant 恢复，摘要漂移、非预期 Console 错误和 API Key 注入负例通过；真实 D1 尚未执行。

数据所有权没有改变：Finance 只保存 Evidence／Money 稳定引用，Payroll 只保存 People／Money 快照与引用；没有共享表、跨库写或 Workbench 业务事实。

## 3. 已执行验证

以下命令实际通过：

```sh
go test ./backend/services/internal-finance-requests/...
go vet ./backend/services/internal-finance-requests/...
go test ./backend/platform/identity-access/... ./packages/workload-identity/... ./backend/services/payroll/...
go vet ./backend/platform/identity-access/... ./packages/workload-identity/... ./backend/services/payroll/...
sh -n deployments/d1/ops/d1 deployments/d1/ops/test-p9-composition
python3 -m json.tool contracts/access/v1/examples/workload-identity-policy.json
deployments/d1/ops/test-p9-composition
deployments/d1/ops/test-release-bundle
acceptance/p9-full-composition/test-c2-member-payroll
acceptance/p9-full-composition/test-c3-payroll-periods
acceptance/p9-full-composition/test-c4-finance-requests
acceptance/p9-full-composition/test-c5-api-keys
acceptance/p9-full-composition/test-c6-invoice-reimbursements
acceptance/p9-full-composition/test-c7-reimbursement-payments
acceptance/p9-full-composition/test-c8-payroll-payments
acceptance/p9-full-composition/test-run-c0-c8
acceptance/p9-full-composition/test-c9-recovery
node acceptance/p9-full-composition/test-c10-feedback.mjs
acceptance/p9-full-composition/test-run-c0-c10
PLAYWRIGHT_MODULE_PATH=... BROWSER_EXECUTABLE=... node acceptance/p9-full-composition/test-c11-browser.mjs
git diff --check
```

`test-p9-composition` 使用实际 `docker compose config --format json`，但不 build、不 pull、不启动容器；它结构化验证 Workload、端点、volume 和健康依赖，覆盖 Finance 缺 Money、Finance 缺 Evidence、Payroll 缺 People、Payroll 缺 Money 四个失败关闭场景，并验证 Feedback 单 Key、双 Key 轮换正例及部分配置、错误项目、不安全 URL、歧义 Key、过量轮换 Key、短 HMAC、Session 模式负例。测试还用无状态 Docker mock 走过一次同选装的 D1 联合备份与空目标恢复命令图，核对 Finance／Payroll dump、选择声明、校验清单和 restore 命令均被纳入；BF-247 增加停入口→全部 dump→恢复入口的严格顺序、Payroll dump 故障 trap、非法 drain 和一致性文件 checksum，BF-248 增加指纹 checksum、匹配恢复和故意漂移失败关闭，BF-249 证明篡改并重算 checksum 的畸形指纹仍在删卷前拒绝且无指纹旧备份继续兼容。另用短生命周期 PostgreSQL 18 实际执行指纹 SQL，验证 2 张表（含空表）、1 行事实及原始敏感值不出库，退出后容器删除。上述仍只证明运维编排和 SQL 可执行，不是十五库真实恢复证据。

`acceptance/p9-full-composition/verify-c0` 已提供真实运行窗口的只读前置入口：它要求十四个领域应用、十九个长运行服务精确匹配且全部健康，要求项目 115 通知配置，核对 Nginx 唯一发布端口、Suite Manifest、公共 readiness 与 D1 check，并记录镜像 ID／repo digest。原始 Compose 和 inspect 不落盘，防止部署秘密进入证据；`test-preflight` 用 mock 覆盖 19／19 正例、缺 Payroll 负例和 Feedback secret 不落盘。C0 尚未在真实全组合运行，不能记为通过。

## 4. P9 尚未完成

仍需在一次有界运行窗口内完成并在退出后清理：

1. 使用非占位部署秘密和当前镜像启动全组合，记录镜像 digest 与全部容器健康；
2. 使用 `run-c0-c11` 连续执行 Tenant、账号／Person、薪酬、工资、财务申请、员工 API Key、发票上传／报销、报销付款、工资实际发放、恢复、Feedback API 旅程和双租户真实浏览器撤权恢复；
3. 创建带 `public-ingress-quiesced-v1` 声明和十五库无秘密事实指纹的联合备份，恢复到空目标并通过自动指纹比对，再次执行 Evidence 字节、关键读取与续写；
4. 对比恢复前后 Tenant、稳定 ID、revision、精确金额、File Version、Money 引用、Key 非秘密元数据、反馈身份映射与审计；
5. 核对 C11 真实环境 Console／点击／撤权证据，并单独记录回滚点。

P8／P9 的真实环境仍缺项目 115 完整外部回复／通知、双账号／双租户 C11 浏览器证据和回滚记录；本地真实 Chrome 夹具不替代这些运行证据。该外部门禁未关闭前，不能把“连续 Feedback”或整轮 P9 记为通过。当前需求没有规定最小 soak 时长，因此源码门禁不凭空增加时间条件。

## 5. 资源状态

本批只执行 Go／shell／JSON／Compose 渲染与一个短生命周期 Chrome／双回环 HTTP 夹具，没有启动 D1、PostgreSQL 或其他常驻容器；浏览器与端口均在测试退出时关闭。全组合构建与恢复留到运行参数齐备后一次性执行，避免当前开发机长期占用资源。
