# OMSBizFoundry 验证入口

## 2026-10-25 OMS1 迭代 P0／P1／P2／P3／P4／P5

P5 财务申请与付款确认提供双 Owner 隔离重复验收入口：

```sh
./acceptance/p5-finance-payment/run-isolated
```

该入口限制 Go 构建并行度，启动一次性 PostgreSQL 18.6、Finance 与 Money 两个真实 HTTP 服务和两个独立数据库；验证付款申请、无需审批快照、分次／超额、批量部分成功、撤回重提、账号遮罩、独立敏感导出及审计，并把两库备份恢复到空目标核对。主申请两笔 Payment 只绑定一个义务、核销 10000 分并留下 1000 分超额未分配；批量 revision 冲突不回滚另一笔成功资金。脚本退出时无论成功失败都关闭进程和容器。该证据不表示 Evidence 实时校验、P6 开放 API、D1 或公网已经完成。[P5 报告](./reports/oms-v1-iteration-p5-finance-payment-validation-20261025.md)

P4 工资执行提供三 Owner 隔离重复验收入口：

```sh
./acceptance/p4-payroll-execution/run-isolated
```

该入口启动一次性 PostgreSQL 18.6，并为 People、Payroll、Money 创建三个独立数据库和三个真实 HTTP 服务；生成两名成员及薪酬快照，预览／提交月度导入，完成核对和批量发放部分成功，再以显式超额确认完成第二次付款。验收证明成功项不会被缺账户失败回滚，两笔 Payment 只绑定一个工资义务，净应发精确核销且 500 分超额留在 Money 未分配；结束时清理自身进程与容器。Payroll 集成测试另覆盖整数舍入、同月并发、幂等、未知恢复、零／负实发、作废／替代、补发／扣回和本人可见性。该证据不表示 D1／公网、正式地区比例或 P5 财务申请已完成。[P4 报告](./reports/oms-v1-iteration-p4-payroll-execution-validation-20261025.md)

P3 成员与薪酬档案提供独立重复验收入口：

```sh
./acceptance/p3-member-payroll-profiles/run-isolated
```

该入口构建 People & Groups 与 Payroll 独立镜像，启动一次性 PostgreSQL 18.6 与两个真实 HTTP 服务，经版本化 People Reference 完成成员新增、离职、离职状态下薪酬 revision、重新入职、薪酬生效月、工资账户缺失、账号遮罩与换卡历史、稳定 Payroll 快照、未知 Person 和匿名拒绝；退出时清理自身资源。PG18 集成测试另覆盖双租户隔离、当前投影一致性、不可变历史、自定义缴费方案复制／修订；HTTP 单测覆盖完整账号精确资源授权和撤权。该证据不表示 P4 工资计算／发放、正式 D1 或公网已完成。[P3 报告](./reports/oms-v1-iteration-p3-member-payroll-profiles-validation-20261025.md)

P2 新增两个独立 Owner，并提供一键隔离验收：

```sh
./acceptance/p2-owner-skeleton/run-isolated
```

该入口构建 `Internal Finance Requests` 与 `Payroll` 两个独立镜像，启动一次性 PostgreSQL 18.6 和两个服务，验证匿名拒绝、草稿写入、同命令重放／冲突、revision 更新、分别备份、恢复到新空库及恢复后继续写入，结束后清理自身资源。两个服务的 PostgreSQL 集成测试还覆盖跨租户读取拒绝、当前投影直接修改拒绝和不可变历史修改拒绝；IAM 单元测试使用签名 JWT／JWK。D1 Compose、Suite Manifest 和发布／提升／升级选择／发布包脚本已通过本地门禁，但没有启动正式 D1 或部署公网。[P2 报告](./reports/oms-v1-iteration-p2-owner-skeleton-validation-20261025.md)

P0 需求和命名基线已冻结，记录见 [`oms-v1-iteration-p0-baseline-validation-20261025.md`](./reports/oms-v1-iteration-p0-baseline-validation-20261025.md)。P1 验证见 [`oms-v1-iteration-p1-multitenant-validation-20261025.md`](./reports/oms-v1-iteration-p1-multitenant-validation-20261025.md)，可重复入口为：

```sh
./acceptance/p1-multitenant/run-isolated
```

该入口创建随机命名的一次性 PostgreSQL 18.6 容器和临时目录，构建并启动真实 IAM／People 二进制，完成双租户、工作负载换票、Subject↔Person、跨租户负例、撤权／停用、分库备份、空目标恢复和恢复后续写；不会使用或停止 `oms-v1-integration-*`、D1 或开发数据库。P1 同时复验全部受影响 Go modules、Capability Manifest、Compose 和镜像构建，以及当时 12 个既有受保护 Owner 的代表性 PostgreSQL 租户隔离、搜索／分页／导出／附件路径。该段只陈述 P1 历史证据；P2 证据见上文，二者均不表示 D1／公网已发布。

2026-10-24 BF-183 已发布并验证：新增 Go 静态回归断言工具栏普通说明与 `.oms-v1__select` 分流，禁止通用 `span` 规则推开或隐藏统一选择器；Workbench Go test／vet、脚本语法、内部管理模型与公网页面 8/8 定向 Node 回归、`git diff --check` 和本地视觉复核通过。公网 Workbench v1.32 镜像为 `sha256:5f3e3a25396c7b7b0ec803646bc1ff85301ba3b8e2f38c42d6767549f70ca291`，备份 `20261024T034034Z` 校验通过；只替换 Workbench，其他 16 个容器 ID 不变，D1 `check` 与 17/17 healthy 通过。正式 IAM 登录、五次 Gateway 授权和 Owner 只读返回 Party 5、Project 1、Billing 2、Money 2、Contract workspace 6；版本化 CSS `20261024.1` 返回 immutable 且包含新规则，业务写入为 0。[报告](./reports/oms-finance-filter-toolbar-publication-20261024.md)

2026-10-23 BF-182 本地交互验证并于 2026-10-24 发布：252 项 Node 回归全部通过，新增静态断言覆盖工资／财务批量条的全选和取消全选标记、工资只纳入未发放行、财务只纳入待支付申请；`node --check`、Workbench Go test／vet 和 `git diff --check` 通过。本地浏览器在工资管理页确认初始显示“已选择 0 条”，取消全选、批量录入绩效和确认已发放均禁用；点击全选后只勾选 2 条未发放工资，1 条已发放工资保持禁用，选择数变为 2，批量动作启用；点击取消全选后复原为 0。财务页现有 2 条均为已支付，因此全选和批量确认保持禁用。该交互随 Workbench v1.32 发布，公网边界和运行证据见 [BF-183 报告](./reports/oms-finance-filter-toolbar-publication-20261024.md)。

2026-10-23 BF-181 已发布并验证：源码 `codex/company-internal-management@13a704d` 的 251 项 Node 回归、Workbench Go test／vet、语法和 diff 检查通过；新增回归断言公网页面按 Tenant＋Subject 形成浏览器存储 Key、展示不落服务端提示且不再执行旧的正式环境失败关闭。公网 Workbench v1.31 发布前人工备份和 D1 自动备份均完成并通过 checksum；发布后 D1 `check`、17/17 长运行容器 healthy、正式 IAM 登录／会话／Workbench 页面和 `.3` 内部管理脚本均返回 200。线上 HTML 与脚本确认账号隔离标记、边界提示和新资产生效，脚本不存在旧失败关闭文案。本次没有写入任何 Owner；成员、工资和财务体验记录继续只在当前账号的当前浏览器保存。[报告](./reports/oms-company-internal-browser-candidate-publication-20261023.md)

2026-10-23 BF-180 本地候选验证记录：249 项 Node 回归全部通过，模型覆盖精确十进制转分、大数、五险一金逐项取整、“到达发薪日不得自动变成已发放”、工资表同步只选择新增在职成员且任一工资行已发放后禁止同步，以及“点击弹窗遮罩不得关闭”；Workbench Go 测试与 `go vet` 通过并覆盖新静态资源和页面标记。浏览器从空数据走通新增成员、确认缴费方案、生成月度工资、批量录入绩效、确认发放、新增报销和付款申请、批量确认支付，列表和操作人同步更新；在成员名称输入“误触保留测试”后点击弹窗外遮罩，弹窗保持打开且输入值不变。另以既有已发放 2026-10 工资表和未发放 2026-11 工资表验证：工资表生成后新增“同步测试成员”，可人工加入 2026-11，工资详情显示“手动同步”及操作人；选择 2026-10 时显示禁止原因且“加入工资表”按钮禁用。本条是发布前证据，公网边界和发布结果由 BF-181 取代；该轮没有修改任何 Owner 数据。复验入口：`http://127.0.0.1:18183/?preview=oa&view=oms-v1`。[规格](./specs/assembly/oms-company-internal-management-local-mvp-v0.1.md)

2026-10-22 BF-179 已发布并验证：操作列回归 2/2 断言九个表头末列、五类动态详情单元格及 76px 右对齐 CSS；与金额、分页和主体候选合并的 Node 回归 9/9 通过。Workbench Go test／vet、前端 TypeScript check、206/206 测试、双 bundle build、公网验收单测 2/2 和 Python 语法检查通过。本地浏览器在应收管理页复核表头末列计算样式为 76px、`text-align:right`，其右边界与表格右边界一致。公网只替换 Workbench 为 v1.30，备份、D1 `check`、17/17 healthy、正式登录与五 Owner 只读复验通过，业务写入为 0；版本化 CSS `.10` 返回不可变缓存资源并包含操作列右对齐规则。[报告](./reports/oms-phase1-detail-action-column-publication-20261022.md)

2026-10-22 BF-178 已发布并验证：金额列静态回归 3/3 断言十个金额表头、五类动态金额单元格、固定最小宽度、无负边距的全宽排序按钮、数值右对齐和 tabular lining figures；与分页、主体候选合并的 Node 回归 10/10 通过。Workbench `node --check`、Go test／vet、前端 TypeScript check、206/206 测试和双 bundle build 通过。本地浏览器在应收管理页读取三个金额表头的计算样式均为 132px、右对齐、`lining-nums tabular-nums`，表头按钮距单元格右边界均为 14px。公网只替换 Workbench 为 v1.29，备份、D1 `check`、17/17 healthy、正式登录与五 Owner 只读复验通过，业务写入为 0；版本化 CSS／JS 均为 `.9` 且不可变缓存生效。[报告](./reports/oms-phase1-money-column-alignment-publication-20261022.md)

2026-10-22 BF-177 已发布并验证：Project Workspace 候选模型／服务／PostgreSQL repository／HTTP 端点／migration 回归覆盖 Party 精确 revision 校验、重放不依赖 Party 服务、非法角色拒绝、租户范围、Capability、命令冲突、重复候选幂等、不可变触发器及跨租户隔离；普通 Go test／vet、integration-tag 编译与真实 PostgreSQL 18 定向集成通过。Workbench `node --check`、角色回归 3/3、Go test／vet、前端 206/206 与双 bundle build 通过。公网完成 migration 000014、D1 `check`、17/17 healthy、正式登录与新候选 API 读写；通过正式命令把已有中性 Party“施工方1”登记为 contractor，复读得到候选 1、甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[施工方1]`。[报告](./reports/oms-phase1-preproject-party-candidate-publication-20261022.md)

2026-10-22 BF-176 甲方可见性已发布并验证：新增纯函数回归覆盖未关联主体进入甲方但不进入施工方、甲方专属、施工方专属与真实双角色四种情况；`node --check`、新测试 2/2、Workbench Go test／vet、前端 check／206 项测试／双 bundle build、公共验收 Python 10/10、语法检查和 `git diff --check` 通过。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.27@sha256:e7906954de08ab389083d82b5df615730f2d0ca9073bada7b136151f63e2bc68`，备份为 `/var/backups/omsbizfoundry/20261022T114210Z`；候选预检、D1 `check`、17/17 healthy、正式登录和五 Owner 只读通过。真实目录投影为甲方 `[甲方1, 甲方2, 甲方3]`、施工方 `[]`，业务写入为 0。[报告](./reports/oms-phase1-client-directory-visibility-publication-20261022.md)

2026-10-22 BF-175 退役界面门禁已发布并验证：当前 OMS 未登录页只引用 `oms-phase1-login`，已登录首屏不引用任何 `reference-oa-*`，附件动作只加载 `oms-phase1-evidence`；Gateway 回归覆盖旧入口 303 到当前 OMS 和旧资源 410。本地前端 check／206 项测试／双 bundle build、Workbench／Gateway Go test 与 vet、公共验收 Python 10/10、语法检查和 `git diff --check` 通过。公网 Gateway／Workbench `v0.1.0-test.oms-v1.26` 发布后，匿名页、正式 IAM 登录、旧入口、旧资源、版本化 immutable 当前资源、五 Owner 只读、Party runtime authority、D1 `check` 和 17/17 健康均通过；业务写入为 0，回滚备份为 `/var/backups/omsbizfoundry/20261022T071636Z`。[报告](./reports/oms-phase1-retired-surfaces-and-evidence-decoupling-publication-20261022.md)

2026-10-22 BF-174 首屏资源与主体角色修复已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、分页回归 4/4、`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、公网验收单测 2/2、Python 语法检查和 `git diff --check` 通过；静态回归断言 OMS 主入口不再同步引用旧 React／initiation 运行时、附件动作按需加载 `.5` 运行时、甲方／施工方分别按项目关系过滤以及版本资源 immutable。公网资源读取确认 OMS CSS 66,674 bytes／0.293 秒、分页 2,874 bytes／0.146 秒、主脚本 164,481 bytes／0.305 秒；2,512,394 bytes／约 5.15 秒的附件运行时仍可用但不再出现在首屏 HTML。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.25@sha256:d9df134e7bcadf70418058bba5d84f4646ae96ae28a509bbb9230883aba7e3f5`，备份为 `/var/backups/omsbizfoundry/20261022T065207Z`；候选预检、D1 `check`、17/17 healthy、正式登录、五次 Gateway 授权与 Owner 只读复验通过，业务写入为 0。验收读取 Project 1、Party 4，并由真实关系计算出甲方 `[甲方1]`、施工方 `[]`。资源时间是单次命令行公网测量，不宣称等同于用户浏览器端完整首屏时间。[报告](./reports/oms-phase1-critical-path-and-party-role-publication-20261022.md)

2026-10-22 BF-173 OMS 渐进刷新优化已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、分页回归 4/4、`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、公网验收单测 2/2 和 `git diff --check` 通过；静态回归断言覆盖按 Audience 的在途授权合并、读请求先直读、明确凭证失效后一次授权／重放、写请求强制新授权、Project／Party 首阶段渲染和重复刷新任务合并。本地 `127.0.0.1:18183` 已加载 `20261022.4` 资产；本地预览没有真实 Owner，因此未把本地等待时间虚报为线上性能基准。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.24@sha256:015ab2b216b47f40d38ac253f89ce8e27afd3fba0c6d6a3a2098cf0c8b3c1054`，备份为 `/var/backups/omsbizfoundry/20261022T062428Z`；候选预检、D1 `check`、17/17 healthy、正式登录、五次 Gateway 授权与 Owner 只读复验通过，业务写入为 0。当前验证证明无条件授权轮次已去除且页面可分阶段就绪；未完成同一已登录浏览器的前后 wall-clock 测量，不声明具体毫秒或百分比。[报告](./reports/oms-phase1-progressive-refresh-publication-20261022.md)

2026-10-22 BF-172 项目详情侧栏导航修复已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、分页回归 4/4、`GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、公网验收单测 2/2 和 `git diff --check` 通过；服务端静态回归断言覆盖详情关闭函数、项目选择清理、异步读取令牌递增、所有页面激活前关闭详情、当前组点击优先退出详情及返回按钮复用同一关闭路径。本地 `127.0.0.1:18183` 已加载 `20261022.3` 资产；预览无真实项目，因此没有把本地详情点击虚报为人工通过。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.23@sha256:3b78fdbeeb273723340b50aa509fcca1aa3620919212f7480ddd590cb95cebc1`，备份为 `/var/backups/omsbizfoundry/20261022T045308Z`；D1 `check`、17/17 healthy、正式登录、五次 Gateway 授权与 Owner 只读复验通过，业务写入为 0。[报告](./reports/oms-phase1-project-detail-navigation-publication-20261022.md)

2026-10-22 BF-171 Party runtime 清理回归已修复并补门禁：线上首先确认 Contract 与 Money 的 `party_runtime_transitions` 均为 0，而 Party、Project 和权限均可读；随后通过正式本地 Party 创建、逐条 reviewed mapping 与 Owner runtime cutover，各建立 1 条指向本方经营主体的精确映射，结果均为 `mode=directory`、`mapped_party_count=1`、`referenced_party_count=0`，第二次执行均为 `changed=false`。新增 `require_directory_authority` 公网验收断言和 2 组单测；Python 8/8、语法检查、D1 `check`、17/17 healthy 通过。增强后的公网验收通过正式登录、五个 Gateway 授权并明确复读 Contract／Money directory authority；Project、Billing、Money 仍为 0，业务写入为 0。[报告](./reports/oms-phase1-party-runtime-reset-repair-20261022.md)

2026-10-22 BF-170 Demo 全量业务数据归零已验证：完整备份 `/var/backups/omsbizfoundry/20261022T023910Z` 的 15 项 checksum 全部通过后，停止公网入口与业务服务，清空十二个已启用业务 Owner 的业务表并删除 Evidence／Contract 本地对象字节；Party Directory 只保留本方经营主体及其一组 revision／command／audit，IAM 未改。逐库实际 `count(*)` 核验：除 Party 技术基准合计 4 行外，其余业务行全部为 0；Evidence 与 Contract 对象文件均为 0。重启后 D1 `check` 通过、17/17 长期容器 healthy；公网验收通过正式登录、五个 Gateway 授权与只读，Project 0、Billing 0、Money 0、Party 仅本方主体 1、业务写入 0，Contract workspace 的 6 是空上下文对象字段数而非合同数量，Contract 业务表实际为 0。[报告](./reports/oms-phase1-demo-full-business-reset-20261022.md)

2026-10-22 BF-169 OMS 导航唯一选中项已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、Workbench Node 235/235、`GOWORK=off go test -count=1 ./...`、`go vet` 和 `git diff --check` 通过。桌面浏览器依次展开三个业务组后，DOM 复核 `expanded=[projects,receivable,payable]`、`activeGroups=[payable]`、桌面 `activeSecondary=[incoming-invoices]`。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.22@sha256:d9b11412c72a6d4b9d9a9aa8c8786855e4e8f5219503edb8514554c0966a4f42`，备份为 `/var/backups/omsbizfoundry/20261022T023910Z`；D1 `check`、17/17 容器健康、正式登录、五次 Gateway 授权与 Owner 只读复验通过，业务写入为 0。[报告](./reports/oms-phase1-single-current-navigation-publication-20261022.md)

2026-10-22 BF-168 OMS 导航与分页控件已发布并验证：本地 `node --check`、Workbench Node 235/235、`GOWORK=off go test -count=1 ./...`、`go vet`、`git diff --check` 和桌面视觉验收通过；三个业务组可同时展开，分页标签与箭头位于统一触发器边界内。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.21@sha256:6dcba5542c68b0c7970c80cee4282fc3560ece7efab197a87ac0e9088c65e423`，备份为 `/var/backups/omsbizfoundry/20261022T021316Z`；候选预检、D1 `check`、17/17 容器健康、正式登录、五次 Gateway 授权与 Owner 只读复验通过，业务写入为 0。公网 HTML SHA-256 为 `a9fd8b254d716d1deeeecc86b869eb39721f84d044a549f3ff73dc7239064e0b`。[报告](./reports/oms-phase1-mvp-review-ui-publication-20261022.md)

2026-10-21 BF-167 OMS 一期列表工作台候选已完成本地验证：静态资源更新为 `20261021.2`；`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、Workbench Node 235/235、`GOWORK=off go test -count=1 ./...` 和 `git diff --check` 通过。本地 `127.0.0.1:18183` 浏览器确认新侧栏与面包屑、项目四项摘要、表头排序、分页、就地展开／收起筛选和居中导出条件均可达；预览环境没有连接真实 Owner，因此空列表只用于交互复核，不外推真实数据结论。本候选尚未发布到公网。[报告](./reports/oms-phase1-mvp-review-ui-local-validation-20261021.md)

2026-10-21 BF-166 OMS 项目列表收付状态已发布并验证：项目列表表头固定为“应收状态／应付状态”，渲染代码不再包含原金额单元格，状态文案固定为“已完成／未完成／金额异常”；精确金额投影与专业模块不变。`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、Workbench Node 235/235、`GOWORK=off go test -count=1 ./...` 和 `git diff --check` 通过。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.20@sha256:f78176522965d41a6858ddf6bda7ea9f97332b0dca2b2fece9fe502760f0efc5`，备份为 `/var/backups/omsbizfoundry/20261021T112852Z`；D1 `check`、17/17 容器健康、正式登录、五次 Gateway 授权和 Owner 只读复验通过，业务写入为 0。[报告](./reports/oms-phase1-project-settlement-status-publication-20261021.md)

2026-10-21 BF-165 OMS 一期详情交互边界已发布并验证：源码为 `main@99ead97`，本地 `node --check frontend/oms-workbench/internal/server/oms_phase1.js`、Workbench Node 235/235、`GOWORK=off go test -count=1 ./...` 和 `git diff --check` 通过。公网只替换 Unified Workbench 为 `v0.1.0-test.oms-v1.19@sha256:6268306eadb7183ddae1da1229e623f5fab3d6fb0c033244b289d24a4629155c`，静态资源为 `20261020.5`；升级前备份为 `/var/backups/omsbizfoundry/20261021T111655Z`。候选 IAM 契约校验、发布后 D1 `check`、17/17 长运行容器 healthy、正式登录、五次 Gateway 授权和 Owner 只读复验通过，读取 Party 7、Project 28、Billing 2、Money 3、Contract workspace 6，业务写入为 0。独立 `codex/oms-interaction-patterns` 试验分支未部署。[报告](./reports/oms-phase1-usability-baseline-publication-20261021.md)

2026-10-19 BF-164 OMS 通用经营主体与真实主体列表已完成本地验证：`make fmt-check && make test && make vet`、Workbench 前端 206/206、TypeScript、Vite build、`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、D1 发布包测试和 shell 语法检查通过。`127.0.0.1:18182` 已重启为本次源码构建；浏览器实际刷新后，甲方和施工方页面均显示“暂无人工录入…”空状态及“系统不会补充演示或默认数据”，活动 OMS 页面显示“本公司项目经营视角”和中性 `data-operating-party-id`。本轮尚未发布到 `oatest`，不得描述为公网已更新。[报告](./reports/oms-phase1-generic-operating-party-validation-20261019.md)

2026-10-19 BF-162 OMS 一期资质单位自交易防护已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、Workbench Node 235/235 和 `go test -count=1 ./frontend/oms-workbench/...` 通过；升级前备份为 `/var/backups/omsbizfoundry/20261019T130505Z`。公网 Workbench 为 `v0.1.0-test.oms-v1.18@sha256:2b2e319dfacbc6b84b743f5e3a9dcf09846d37c99f7de8523eba8990adf3f54f`，静态资源 `20261019.1` 包含对手方校验与明确中文提示；D1 `check`、17/17 长运行容器 healthy、正式登录、五个 Gateway 授权与 Owner 只读复验通过，业务写入为 0。已确认异常项目“测试项目100”的甲方 Party ID 与资质单位 ID 相同，因 Project 快照不可直接覆盖，本轮没有篡改或删除该数据。[报告](./reports/oms-phase1-self-counterparty-guard-validation-20261019.md)

2026-10-18 BF-161 OMS 一期定向清理和范围过滤已验证：清理前备份 `/var/backups/omsbizfoundry/20261018T041742Z` 完成；恢复源 `/var/backups/omsbizfoundry/20261012T175912Z` 校验和通过。六个 Owner 恢复并重建 Party runtime 后，数据库核对为 Project 26（全部旧 OA、`operations` 为 0）、Contract 3、Billing 0、Money 3、Party 6、Evidence 19 files／20 versions。`node --check`、Workbench Node 235/235、Workbench Go 测试、D1 `check` 与 17/17 容器健康通过；公网 Workbench `v0.1.0-test.oms-v1.17@sha256:da5f4626cb53a9d8ca6758f2c6852468974221fc4ca440264de4a62a42d5dbfe` 的只读验证通过且业务写入为 0。浏览器标签页能打开公开 OMS，但自动化 DOM 快照两次超时，因此没有把视觉空列表虚报为已检查；数据库范围、静态资产和公网正式链证据已闭合。[报告](./reports/oms-phase1-data-reset-and-scope-validation-20261018.md)

2026-10-17 BF-160 OMS 一期短期访问凭证续期兼容已发布并验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、235 项 Workbench Node 测试、Gateway／Workbench 定向 Go 测试、D1 发布包检查、升级前后 `check` 均通过。公网 Workbench 为 `v0.1.0-test.oms-v1.16@sha256:9a082f08dba5897e5cdba3698e855a6f9e842d772484a63485c68c7f64c567e5`，发布包 SHA-256 为 `ec940c632856ef6564b55e93a2ba6aeb6243212bd23268c058a2f446a078b39d`，备份为 `20261017T051948Z`，17/17 容器 healthy。公网静态资源 `20261017.3` 已包含 `opaqueredirect/status=0` 有界重授权；正式登录、Gateway 授权和 Owner 复读返回 Party 13、Project 30、Billing 3、Money 5、Contract workspace 6，业务写入为 0。当前用户 Chrome 的 OMS 标签页已关闭，因此本次没有把“浏览器视觉刷新”虚报为已执行；API 与运行态证据证明截图中的 0 条不是数据丢失。[报告](./reports/oms-phase1-capability-renewal-repair-20261017.md)

2026-10-17 BF-159 OMS 一期主入口已发布并通过验证：Gateway／Workbench 定向 Go 测试和 235 项 Workbench Node 测试通过；D1 发布包 SHA-256 与包内清单通过，升级前后 `check` 通过。公网使用 Gateway `v0.1.0-test.oms-v1.14@sha256:d58db150578c9992cd703be316560730945aae35930068628469f3593f29f2a5` 与 Workbench `v0.1.0-test.oms-v1.14@sha256:32cf219f77f7b3da3f9aebb0500fec11c4979806bc7b3dc969c0d3424fe55ff7`，备份为 `20261017T033416Z`，17 个长运行容器 healthy。公网路由确认 `/` 与 `/oms-v1/` 到 OMS、`/legacy/` 到旧版、`/oms/` 到参考版；Chrome 根域名首屏直接显示 OMS、完成 29 个项目与票款资金汇总，旧版入口可往返且保留全部历史模块。[报告](./reports/oms-phase1-primary-entry-validation-20261017.md)

2026-10-17 BF-158 OMS 一期 CSRF 新鲜度修复已发布并通过验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、共享 CSRF 与分页 Node 测试 12/12、`go test ./frontend/oms-workbench/...`、Workbench `go vet`、发布包校验和正式 D1 `check` 均通过。公网使用 `v0.1.0-test.oms-v1.13@sha256:3215e21d963f4f978f036ecad2e1f36d437d6680035ca0cde8097b0b9eb82b80`，备份为 `20261017T030537Z`，17 个长运行容器 healthy；静态脚本确认使用配置 IAM route 和 `await csrfProvider.token()`，不存在退出登录表单令牌查询。Chrome 先筛选确认“微信4123”为 0 条，再以既有甲方、`¥1,243,123.00` 和 2026-10-17 创建成功；项目详情复读同额应收、0 回款、`¥1,243,123.00` 未回、合同草稿与 Project r1／r2 记录。Gateway、Workbench、Project、Contract 的操作后日志未出现 CSRF／error／panic／fatal。[报告](./reports/oms-phase1-csrf-refresh-project-create-validation-20261017.md)

2026-10-14 BF-157 OMS 一期新增与项目汇总反馈修复已发布并通过验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、4 项分页回归、`go test ./frontend/oms-workbench/...`、D1 发布包检查和正式远端 `check` 均通过。只替换 Unified Workbench 为 `v0.1.0-test.oms-v1.12@sha256:a9759a92300718a0851c20a619406992a16e5fdfa198315fcb7de9e12c2f0f31`；CSS／JS 查询版本递增到 `20261014.2`。公网 Chrome 真实确认先前隐藏的甲方重新可见、新增施工方保存后立即显示；项目“微信1111”详情显示累计开票 `¥11,412.00 / 1 笔 / 最近 2026/10/14`、实际回款 `¥123.00 / 1 笔 / 最近 2026/10/14` 和未回款 `¥11,110,988.00`。全部业务写入共用修复后的授权封装，未新增 Owner、数据库、schema 或 Grant。[报告](./reports/oms-phase1-create-and-summary-feedback-validation-20261014.md)

2026-10-14 BF-156 OMS 一期详情与选择控件统一已发布并通过验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、`go test ./frontend/oms-workbench/internal/server`、`go test ./frontend/oms-workbench/...`、定向 `git diff --check`、D1 发布包检查和正式 D1 `check` 均通过；17/17 容器 healthy。仅替换 Unified Workbench 为 `v0.1.0-test.oms-v1.10@sha256:30e4a2a4b7e418eefe8f678cced85e283dff53d50494b58807a672d3d76cf112`。本地浏览器确认白色下拉弹层锚定于触发器下方并在视口右侧向内避让；公网 Chrome 刷新后不再出现系统原生黑色菜单，真实“微信1111”应付详情显示状态、应付总额、已支付、未支付、往来主体、事实来源和备注的分层卡片，底部只保留关闭。[报告](./reports/oms-phase1-unified-detail-select-validation-20261014.md)

2026-10-13 BF-155 可读性与工作区密度调整已发布并通过验证：`go test ./frontend/oms-workbench/...`、4 项分页回归、JavaScript 语法检查、定向 `git diff --check`、发布包检查和远端 D1 `check` 均通过；17/17 容器 healthy。仅替换 Unified Workbench 为 `v0.1.0-test.oms-v1.9@sha256:fd14c22d71694a17c8bb3a822afa49dbb794f9dfa51c4e007c3226a880f6ef38`。公网 Chrome 实际复核经营总览、20 条项目列表和项目 `2a112e09-c0bf-419d-b615-d3420faeeec5` 详情：工作区横向展开，桌面侧栏变窄，列表项目／金额／次级说明、两条资金链和逐施工方分账均无需依赖原小字号读取；顶部与筛选区高度未明显增加。[报告](./reports/oms-phase1-readable-density-validation-20261013.md)

2026-10-13 BF-154 项目详情板块化布局已发布并通过验证：脚本检查、4 项分页回归、`go test ./frontend/oms-workbench/...`、定向 `git diff --check`、D1 发布包检查与远端 `check` 均通过；17/17 容器 healthy。仅替换 Unified Workbench 为 `v0.1.0-test.oms-v1.8@sha256:968877897398e209b4af218602b65a4a9cb1c606b7a04b5c69117a943a99da28`。公网 Chrome 实际打开项目 `2a112e09-c0bf-419d-b615-d3420faeeec5`，确认顶部只剩返回，编辑项目、合同详情、关联施工方和设置应付分别位于对应业务板块；两条金额链、逐施工方分账和历史事实保持原值。[报告](./reports/oms-phase1-project-detail-layout-validation-20261013.md)

2026-10-13 BF-153 统一文件预览已发布并通过验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、4 项分页回归、`go test ./frontend/oms-workbench/...`、定向 `git diff --check`、发布包检查与 D1 `check` 均通过；17/17 容器 healthy。仅替换 Unified Workbench 为 `v0.1.0-test.oms-v1.7@sha256:69fc8def18092af7bf0b45ba2be394d02d51881b109fdbe7b5ac0e4b1c9a707f`。公网 Chrome 实际在项目 `2a112e09-c0bf-419d-b615-d3420faeeec5` 打开甲方合同图片和甲方开票图片，均显示文件名、类型、大小、缩放、下载和关闭；Esc 关闭后合同详情保持打开。合同、票据和资金凭证使用同一绑定函数，因此回款、支付和收票详情继承同一预览行为。该验证不宣称 Word／Excel 在线转换、OCR 或编辑能力。[报告](./reports/oms-phase1-file-preview-validation-20261013.md)

2026-10-13 BF-152 项目详情完整投影已发布并通过验证：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、`node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`、`go test ./frontend/oms-workbench/...`、D1 `check` 和发布包检查通过；17/17 容器 healthy。只替换 Unified Workbench 为 `v0.1.0-test.oms-v1.5@sha256:5baee6d04a02f56f72ca0d4d4268bdba7ac1bbdd85b97de624cb98d99b2c6428`；CSS／JS 使用发布版本查询参数避免旧标签页继续命中上一版缓存。公网 Chrome 实际打开项目 `2a112e09-c0bf-419d-b615-d3420faeeec5`，显示甲方侧 8,000,000.00／3,200,000.00／2,200,000.00／5,800,000.00，施工方侧 2,680,000.00／960,000.00／1,200,000.00／1,480,000.00，以及逐施工方折叠账、金额变更空状态、4 条 Project revision 记录和项目说明。[报告](./reports/oms-phase1-project-detail-validation-20261013.md)

2026-10-13 BF-151 合同详情发布验证通过：`node --check frontend/oms-workbench/internal/server/oms_phase1.js`、`go test ./frontend/oms-workbench/...` 和定向 `git diff --check` 通过；公网 `/workbench/assets/oms-phase1.js` 返回 200 并包含 `openProjectContractEditor`，D1 `check` 通过，17/17 容器 healthy。只替换 Unified Workbench 为 `v0.1.0-test.oms-v1.3@sha256:a688ac264d568d4b981058f1119706a6241046ac49c902f757fee31e15bba1f2`，其余应用、数据库、schema 和 Grant 均未改变。公网 Chrome 实际进入验收项目“合同详情”：上游显示 8,000,000.00 和甲方及附件，下游切换后显示 2,680,000.00 和施工方及附件；进入下游草稿编辑、把金额临时输入 1,000,000.00 后立即出现“低于已支付 1,200,000.00”的强提醒，随后取消，未提交任何业务修改。[报告](./reports/oms-phase1-contract-detail-validation-20261013.md)。

2026-10-13 BF-150 公网六 Owner 写入闭环通过：发布 `v0.1.0-test.oms-v1.2` 后先备份到 `/var/backups/omsbizfoundry/20261012T175912Z`，再完成 Contract／Money 所有本地主体的审计、显式映射与 Party Directory runtime cutover；第二次执行无变更，证明迁移幂等。`verify-public-write-journey.py --run-key oms-phase1-public-20261013-a` 通过公网 Nginx→Gateway→IAM→Owner 创建并复读项目 `2a112e09-c0bf-419d-b615-d3420faeeec5`：上游合同 8,000,000.00、下游合同 2,680,000.00、开票 3,200,000.00、收票 960,000.00、回款 2,200,000.00、支付 1,200,000.00；六份 Evidence 均通过短期访问授权下载，SHA-256 一致。相同 run key 再次运行仍返回同一 ID，无重复事实。Party／Project／Billing／Money 的真实筛选集合分别复读 2／1／2／2 条；Workbench 分页 helper 的 205 条 offset、201 条 cursor、多页重复失败关闭共 4 项 Node 测试通过，六 Owner 依赖边界 2 项和主体迁移 6 项 Python 测试通过。本地真实筛选导出 `OMS-开票记录-2026-10-13.csv` 为 269 bytes，SHA-256 `92f199dbd3847c4afb0a312fb9fafdce8afe0a0776dfc4312fdb0f073af906e2`。发布后 D1 schema check 通过，17/17 容器 healthy，既有只读公网验证也通过（Party 8、Project 27、Billing 2、Money 4、Contract 6）。People、Business Engagement、Production、Approval、Cost、Bonus、Design 没有出现在 OMS runtime 调用集合中，这是一期边界，不是漏接。

2026-10-12 OMS 一期外层入口初始化故障已修复并部署：根因是从 `/workbench/` 点击“OMS 一期”时只替换了工作台内容，而 `oms_phase1.js` 已因初始 URL 不含 `view=oms-v1` 提前退出，导致页面长期显示“正在连接业务系统”且 OMS 侧栏未绑定事件。Workbench 入口现统一导航到受保护的 `/workbench/?view=oms-v1`，由完整页面初始化正式 OMS runtime；`go test ./frontend/oms-workbench/...` 通过，新镜像摘要为 `sha256:3b1935595d14ec7770832c934d3c743e7e355544b9620d2f7dffda1e27080aa8`。公网实际从无查询参数的工作台点击“OMS 一期”后，页面显示“业务来源已同步”、读取 26 个项目，点击“项目经营”可展开三个二级入口并展示项目列表；当前 17 个 `omsbizfoundry` 容器均为 healthy。本轮没有提交 Project、Party、Billing 或 Money 业务写入。

2026-10-12 BF-149 公网验收通过：安全组恢复 `/32` SSH 后先创建完整备份 `20261012T140404Z`，再用 `v0.1.0-test.oms-v1.1` 摘要锁定闭包升级 Auth58 D1。`deployments/d1/ops/d1 check` 通过，17 个长运行容器 healthy；`/oms-v1/` 303 到受保护 Workbench，旧 `/oms/` 兼容入口保持。`acceptance/oms-phase1-public/ensure-qualification-party.py` 通过公网 IAM／Gateway／Party 原生命令创建并回读正式资质单位；`verify-public-oms-v1.py` 通过正式登录、五次 Gateway 授权及 Party 3、Project 26、Billing 0、Money 2、Contract 上下文 4 项读取，验收业务写入为 0。证据在 ECS `/opt/omsbizfoundry/acceptance/oms-v1-public-20261012a`。该验收未执行 Money／Evidence／施工方应付写入、撤权／故障矩阵或发布后人工浏览器逐按钮检查。

2026-10-12 BF-148 已完成本地正式 D1 组合复验：使用真实 IAM 登录，创建资质单位与甲方主体，执行 Project 创建、Contract 800 万精确金额创建及 Project Resource Link，经营总览从 Owner 回读 1 个项目和 800 万未回；随后通过 Billing 原生命令登记 320 万开票并刷新复读成功。正式 `deployments/d1/ops/d1 check`、`deployments/d1/ops/test-release-bundle`、发布包外层和 `RELEASE-CHECKSUMS.sha256` 内层校验、`node --check`、`go test ./frontend/oms-workbench/...` 均通过。15 应用＋4 基础镜像已经以 `v0.1.0-test.oms-v1.1` 的 `linux/amd64` 摘要推送到 ACR，交付包为 `dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.1_linux_amd64.tar.gz`。该证据不包含公网：`https://oatest.antuinfo.cn/oms-v1/` 仍返回 404，`47.98.156.166:22` 连接超时，未执行 Main 备份／升级／公网复读，因此不得标记已上线。

2026-10-12 BF-147 已完成本地浏览器验收：新建项目中的甲方选择器计算样式为浅色、`appearance:none`、统一箭头和 34px 右侧留白，默认占位值为空；“创建甲方”入口实际关闭项目抽屉并切换到“项目经营 → 甲方管理”，页面状态为 `projects / clients`，焦点落在甲方新增入口并显示返回项目列表的操作提示。浏览器 warning／error 为 0。该路径没有提交 Party 或 Project，也没有做名称自动匹配；不提升公网部署或真实 Owner 写入结论。

2026-10-12 BF-146 已完成本地导航验收：桌面逐项点击项目经营、应收回款、应付支付共九个二级入口，全部切换到对应页面，任一时刻只有当前业务组展开，内容区 `.oms-v1__tabs` 数量为 0；窄桌面操作区未被新侧栏裁切。浏览器实际折算为 223px 内容宽度的移动检查中，底部一级导航和顶栏下二级导航连续切换成功，侧栏隐藏，整页无横向溢出；浏览器 warning／error 为 0。该检查不提升 Owner、IAM、D1 或公网部署完成度。

2026-10-12 BF-145 第一轮实现已实际完成：Gateway `GET/HEAD /oms-v1[/]` 只重定向到受保护的 `/?view=oms-v1` Workbench，不携带 `preview=oa`；Workbench 新工作面固定四入口、桌面左栏、手机底栏、Owner 图表总览、项目／主体／票据真实读取适配和统一列表／抽屉。Money Flow 在原事实 Owner 内新增 `GET /api/v1/cash-transactions`，支持收／付款方向、有效／已冲销状态、业务日期、项目来源、offset／limit，默认只返回有效记录并按业务日期、系统记录时间倒序；金额权限不足时保持记录可见但金额字段为 `null`。Money reference 另为 Party 快照显式标注 creditor／debtor／payer／payee；Project commercial summary 仅在请求时透传这些快照与 Link 建立时间，Workbench 据此把已关联 obligation 呈现为应收／应付列表并按关系时间倒序，不按名称猜主体、不保存金额。Billing 双方向搜索、计数、详情、新增和 correction revision 已接通；资质单位使用显式 `OMS_QUALIFICATION_PARTY_ID`，保存只调用 Billing 原生命令并冻结 Project／Party exact revision。配置格式、服务端注入、Demo 不泄漏和脚本门禁单测通过；本地浏览器实际确认 preview 中资质单位 ID 为空、保存按钮禁用且显示明确原因。Project 允许以精确 Party revision 建立 `contractor-party` Link，并在受授权目录返回当前有效关系；Workbench 已接 Party 原生主体创建／更正、Project 原生单施工方关系写入和基本资料编辑。六类资金列表的本地导出窗口已实际打开，确认当前筛选结果跨 UI 分页导出、零记录禁用、精确金额提示和“非后台全量”的边界提示。实际运行 JavaScript 检查，Gateway／Workbench／Money／Project／Billing 单元与 HTTP 测试，以及 `TestPostgresDirectoryPaginationHierarchyScopeAndReadOnly` PG18 集成测试均通过；新增 HTTP 测试明确断言 Party 角色和项目目录稳定关系。浏览器本地复核确认桌面布局完整；极窄视口首次抓到项目标题被按钮挤成竖排，修复后标题、标签、说明区和四项底栏无整页横向溢出。公网 `/oms-v1/` 最新复读仍为 404，且既有 Auth58 SSH 运维入口仍不可达；未绕过主机边界或伪造部署结论。完整证据与缺口见[本地实现与部署前报告](./reports/oms-phase1-mvp-local-validation-20261012.md)。该证据尚未证明公网部署、正式登录、主体 runtime cutover、Project＋Contract 自动应收／应付编排、Owner 级全量分页／导出、带正式 IAM 的真实成功写入或附件闭环。

2026-10-11 BF-143 已实际完成 OMS 一期分批回款验证原型的脚本语法、离线依赖、`git diff --check`、桌面浏览器交互和 390×844 窄屏静态复核。刷新后的事项概览以同一笔 320 万应收为主线，显示 160 万、60 万两笔到账、累计回款 220 万、剩余未回 100 万和“当前无固定到期日”；下游付款以 120／268 万独立支线呈现，没有随回款自动推进。先记录“承诺 40 万、2026-10-18 继续跟进”后，实际回款仍保持 220 万；再登记 40 万真实到账后，页面实际变为 3 笔到账、累计 260 万、81%、剩余 60 万，批次表、事项摘要、完整台账和跨事项收付台账同步更新。跨事项“回款跟进”筛选实际只显示 GJ-2026-001、60 万、2026-10-18 和联系结果。完整经营台账默认折叠，390×844 截图中登记／跟进动作纵向独立成行、四项关键金额为两列、批次表保留局部滚动。该结果只验证单 HTML 浏览器内存态的产品信息结构和模拟联动；不表示 Money Flow schema、正式命令、持久化、IAM、文件、部署、客户规则或正式 OMS 已实现。

2026-10-11 BF-141 已实际完成页面内二级工作区、脚本语法、离线依赖和浏览器验证。经营事项实际可切换事项列表、经营全貌、合同与资料、协作方与人员、预算与结算五页：列表显示 2 件事项，合同资料显示 800 万合同、v1 和 5／6 资料，协作页显示 1 家企业、1 个候选班组和 26 人，预算结算页显示 310／268／268／120／148 万的演示摘要。收付台账上游实际显示发票管理／应收管理／回款记录，下游显示结算管理／应付管理／付款条件／付款授权／支付记录；付款条件只筛出 TJ-2026-002，付款授权只筛出“尚未形成／尚未授权”的前置状态，二者没有自动合并。390×844 下页面宽 375、视口 390，无整页横向溢出，五个事项二级入口和八个收付业务子项保持可访问；恢复桌面后默认回到经营全貌，console warning／error 为 0。该结果只验证本地静态信息架构、筛选和响应式表现，不确认客户岗位、正式权限、人员款项、智能材料、业务规则、持久化、Owner API、IAM 或部署。

2026-10-11 BF-140 已实际完成四工作区信息架构、脚本语法和浏览器验证。刷新默认进入经营事项并保留 BF-139 的两图、分组台账、收付关系和依据抽屉；桌面侧栏与手机底栏均显示经营总览、经营事项、收付台账、待办事项四个入口，切换后面包屑同步。经营总览实际显示 5 项总体指标、2 件事项和 3 项需要介入的例外；收付台账上游实际显示 5 条发票／应收／回款记录，下游实际显示 5 条结算／应付／授权／实付记录，选择下游“授权”只保留 SQ-2026-002，选择北区＋需要处理只保留 YS-2026-002；待办 3 项均能打开对应事实抽屉。1280 桌面无横向溢出；390×844 手机整页宽 390，四入口底栏完整，待办转为紧凑三列，财务台账在 364 宽容器内独立滚动 840 宽表格。重复英文眉题已在视觉复核后删除，console warning／error 为 0。该结果只验证本地虚构数据下的角色化信息结构、筛选和回查，不确认正式岗位、权限、业务口径、会计规则、持久化、Owner API、IAM 或部署。

2026-10-11 BF-139 已实际完成经营台账分组、方向筛选与“收付关系”表达校验。概览台账以浅绿“上游收款”和浅蓝灰“下游履约与付款”两条组带替代每行重复的小字方向标签；七条记录保持原事实，最近记录拆为编号与金额／说明两层。点击“上游”只显示上游组及 3 行，点击“下游”只显示下游组及 4 行，返回“全部”恢复两组。原“业务链”改名“收付关系”，页面明确说明箭头只表示先后和引用关系，开票、应收、回款、应付、付款仍须分别登记，不代表自动生成、审批或付款。桌面视口 1394、页面与台账均无横向溢出；390×844 手机页面宽 375、整页无横向溢出，台账在 351 宽容器内独立滚动 640 宽表格。XLSX 实际保持十列七行原生筛选表，上游／下游行分别使用同组底色，方向、事项、状态、日期均可筛选；ZIP 完整性、结构化表检查、公式错误扫描和渲染预览通过，浏览器实际下载成功，console warning／error 为 0。该结果只证明本地虚构数据下的静态分组、筛选、说明和下载，不确认业务术语、关系、数据口径、客户接受、持久化、后端、IAM 或正式 OMS。

2026-10-11 BF-138 已实际完成脚本语法、离线依赖、桌面与 390×844 手机浏览器检查。刷新默认选择“概览”，两张图下方直接显示七行经营台账，列为业务环节、累计、最近记录、待处理、状态；页面中不存在 BF-137 的“这项记录代表什么”、经营结论、长段落、中英文重复眉题或装饰性副标题。切换“业务链”后台账和图表隐藏、合同起点与双轨恢复；返回概览正常。手机整页宽度 375／视口 390，无页面横向溢出，台账在 351 宽容器内独立滚动 640 宽表格。点击台账“实际回款”打开 60／220／100 万依据；下载链接已改为真正的 `北区道路维护事项-经营台账-2026-09-10.xlsx`，浏览器 DOM 解析为 `.xlsx`。工作簿实测为一张可筛选表、十列七行，累计值为数值、统计日期为日期；实际 HTTP 下载返回正确的 XLSX MIME，下载文件与页面资源 SHA-256 一致，ZIP 完整性检查、公式扫描和渲染预览均通过。console warning／error 为 0。该结果只验证本地静态台账结构与下载，不确认字段已经客户验收或形成正式导出契约。

2026-10-11 BF-137 已实际完成嵌入脚本语法、离线依赖、桌面和 390×844 手机浏览器检查。明细模式显示《经营事项明细单》、经营结论、最需处理事项、上游 3 行和下游 4 行结构化解释；桌面为五列表格，手机隐藏表头并把 7 行转换为带“当前记录／业务说明／剩余与下一步”标签的纵向文档，页面宽度不溢出。点击上游第一行“查看”实际打开“向上游开票”依据，显示本次 120、累计 320、合同余额 480；刷新后默认仍回到图表且明细单隐藏。console warning／error 为 0。该结果只验证本地静态文本分层、响应式格式和依据回查，不确认解释已经客户认可或形成正式业务规则。

2026-10-11 BF-136 已实际完成嵌入脚本语法、离线依赖、桌面与 390×844 手机浏览器检查。刷新默认图表层不再显示当前重点、合同起点和六个阶段金额，只显示 69%／45% 两张环形图、已回款 220／未回 100、已付款 120／未付 148 和闭环条件；手机两图纵向排列，页面宽度不溢出且闭环区在首屏可见。切换“明细”后当前重点、合同起点、手机轨道切换和完整双轨均恢复，再切回图表后重新隐藏；console warning／error 为 0。该结果只验证本地静态信息分层、响应式布局和视图切换，不确认数据口径、客户接受、持久化或正式 OMS 产品。

2026-10-11 BF-135 图表模式已实际完成脚本语法、离线依赖、桌面与 390×844 手机浏览器检查。刷新默认选择“图表”：上游环形图为回款 220／应收 320＝68.75%，显示 69%；下游环形图为付款 120／应付 268≈44.78%，显示 45%；未回 100／其中 60 逾期、未付 148／其中 96 待判断与两侧三阶段金额同时可见。点击上游环形图实际打开回款详情，本次 60、累计 220、未回 100 及应收引用保持；切到明细后双轨完整恢复，切回图表后明细隐藏。手机两图纵向排列且明细轨不重复显示，console warning／error 为 0。该结果只验证本地静态分析表达和交互，不确认真实统计口径、客户接受、持久化或正式系统。

2026-10-11 BF-134 图标模式已实际完成脚本语法和离线依赖检查，并在正常桌面与 390×844 手机尺寸验证。页面刷新默认选中“图标”，桌面显示合同图标及上下游共八个业务图标，节点可切换到“明细”后恢复全部说明，再切回图标不会丢失信息；点击“实际回款”图标仍显示本次 60 万、累计 220 万、当前未回 100 万及对应应收引用。手机默认显示上游四个两列图标，切换下游后实际显示协作、结算、应付、付款四个节点；console warning／error 为 0。该检查只证明本地静态视图切换、响应式布局和详情入口，不证明图标术语已经客户确认或形成正式产品规则。

2026-10-11 BF-133 层级新版对照 Demo 已实际完成嵌入 JavaScript 语法、无外部依赖和 `git diff --check` 检查，并在浏览器验证正常尺寸、1366×768 笔记本尺寸和 390×844 手机尺寸。笔记本页面依次呈现事项身份、60 万逾期重点、800 万合同起点、上游开票／应收／回款／未回、下游协作／结算／应付／付款及闭环条件；闭环区从首屏底部开始可见，页面总高度约 800px。手机侧栏隐藏、底部保留旧版／经营图对照入口，上下游轨实际切换成功；逾期详情正确显示未回 100 万／逾期 60 万／已安排 0 万，回款详情正确区分本次 60 万／累计 220 万／当前未回 100 万，付款详情正确区分应付 268 万／实付 120 万／未付 148 万，console warning／error 为 0。入口为 [`acceptance/oms-validation-mvp-v0.2/hierarchy-demo.html`](../acceptance/oms-validation-mvp-v0.2/hierarchy-demo.html)；既有 `index.html` 未修改。本次只验证本地静态表现、响应式层级和详情交互，不证明真实数据、持久化、业务规则、Owner API、IAM、文件、部署或客户接受。

2026-10-10 BF-132 0.2.10 已实际完成嵌入 JavaScript 语法、离线依赖、`git diff --check --no-index` 和浏览器六段回归。北区新增 20 万开票后累计开票为 340 万、应收仍为 320 万；再独立确认 20 万应收后未回变为 120 万、实际回款仍为 220 万。新增 20 万结算后结算为 288 万、应付仍为 268 万；再独立确认 20 万应付后应付为 288 万、实际付款仍为 120 万。东区合同资料页显示 HT-2026-008、v1、5／6 缺口，补充最终归档说明后变为 6／6 并开放归档。北辰协作档案实际显示负责人／18 人、关联事项、预算／结算和应付／已付／未付并可返回北区事项。北区冲销 20 万回款后实际回款为 200 万，新增反向历史且旧 HK-2026-002 历史仍在。东区完成资料后实际归档，状态变为“经营事项已闭环”、历史出现闭环事实且“已闭环”筛选只返回该事项。重置后页面回到北区业务事实基线，console warning／error 为 0。该证据只覆盖本地虚构数据和浏览器内存态，不覆盖真实文件、刷新持久化、Owner API、IAM、部署、客户规则或正式业务闭环。

2026-10-10 BF-131 0.2.4 已实际完成脚本语法、`git diff --check` 和浏览器回归。初始待办显示三项：今日回款风险、今日付款判断、本周资料补齐，并分别显示触发原因和唯一动作。点击回款风险可直接进入北区事项并打开跟进表单；保存 40 万预计到账后返回待办，队列重排为继续跟进 20 万、10-25 核对 40 万到账、判断 96 万付款条件。第三项可直接打开付款判断，选择暂缓后待办变为 10-18 付款复核。重置后点击资料补齐，直接进入东区事项“资料与待办”并显示最终归档说明缺口。再次重置回到初始待办，console warning／error 为 0。该证据只覆盖本地浏览器模拟，不覆盖正式任务持久化、责任人、SLA、消息通知、IAM、Owner API 或部署。

2026-10-10 BF-130 0.2.3 已实际完成脚本语法、`git diff --check` 和浏览器回归。初始实际回款／付款为 220／120 万、未付 148 万；记录 96 万“已核对可授权”后，实际付款仍为 120 万，付款条件节点、当前建议、事项摘要和时间线显示判断结果，页面只开放独立授权；授权 96 万后实际付款仍为 120 万，页面才开放实际付款；登记实付后实际付款变为 216 万、未付 52 万，并生成下一轮 52 万付款条件判断。重置后另测“暂缓至 10-18 复核”，资金数值保持不变，复核待办、经营观察、应付台账和预计现金流同步；再次重置恢复初始状态，console warning／error 为 0。该证据只覆盖本地浏览器模拟，不覆盖真实付款条件、证据、审批权限、持久化、IAM、Owner API 或部署。

2026-10-10 BF-129 0.2.2 已实际完成脚本语法、离线依赖、`git diff --check` 和浏览器回归。初始 60 万逾期、实际回款／付款 220／120 万；记录 40 万预计 10-25 到账后，资金数值保持不变，循环显示“40 万已安排、仍有 100 万未回”，经营观察拆为逾期 60、已安排 40、未安排 20，待办形成继续跟进 20、等待 40 到账和判断 96 付款条件，应收账与预计现金流同步，时间线和变更历史保留“未改变实际回款”。随后登记 40 万到账，实际回款变为 260 万、逾期降为 20 万，原 40 万计划显示已兑现；继续执行 96 万授权和付款后实际付款为 216 万、未付 52 万。重置恢复初始状态，console warning／error 为 0。该证据只覆盖本地浏览器模拟，不覆盖真实催收权限、凭证、通知、持久化、IAM、Owner API 或部署。

2026-10-10 BF-128 0.2.1 已实际完成脚本语法、`git diff --check` 和浏览器完整链路验收。初始回款／付款为 220／120 万；登记 40 万回款后变为 260／120 万，付款条件仍为“待判断”，证明未自动联动；未授权直接登记付款被阻止；授权 96 万后实际付款仍为 120 万、页面显示“已授权待执行”；登记实际付款后变为 216 万、应付未付由 148 降为 52 万。待办、时间线、变更历史、经营总览、应收账、应付账和经营总账同步更新，其中汇总回款 500 万、未回 60 万、未付 52 万，YF-2026-002 显示已付 96 万／已核销，总账显示收入 500 万、支出 381 万、净流入 119 万。重置后恢复初始数值，浏览器 console warning／error 为 0。该证据仅覆盖本地浏览器内模拟状态，不覆盖真实数据、刷新持久化、正式授权、付款执行、IAM、Owner API 或部署。

2026-10-10 BF-127 收付循环已实际完成脚本语法和页面读取检查；浏览器刷新后单事项首个 Tab 为“收付循环”，首屏显示“本轮重点：收回 100 万 · 判断 148 万未付款”、循环步骤和“不得自动付款”边界，上游链节点明确为“向上游开票 320 万”。浏览器可访问树已同时呈现六项收付指标、两条独立链和下一步，未发现页面脚本错误。该结果只验证表达是否可评审，不证明循环、结束条件或付款规则已获客户确认。

2026-10-10 BF-126 两入口结构已实际通过脚本语法、一级入口计数和无外部依赖检查；浏览器刷新后只显示“待办事项、经营事项”。打开经营事项可见“经营总览、全部事项、协作资料”三个二级页面：总览保留四类台账，全部事项保留两笔事项及筛选，协作资料保留施工单位／班组、名册、预算和四份模板；三页间实际切换成功，console warning／error 为 0。该验收只证明信息架构和浏览器原型行为。

2026-10-10 BF-125 信息架构修正已实际完成脚本语法、三入口数量和无外部依赖静态检查；浏览器刷新后确认侧栏只有“待办事项、经营台账、协作单位”。待办页已无“今日工作”旧名；经营台账可切换上游应收账、施工队应付账、经营总账与预计现金流，并可进入经营事项；协作单位显示企业／班组、负责人、范围、名册、预算、应付信息和四份资料模板，实际点击“施工单位信息模板”成功触发 CSV 下载提示。该结果只验证原型的信息架构和浏览器行为，不验证真实模板字段、导入、业务规则或后端。

2026-10-10 BF-124 OMS 0.2 单文件原型已实际完成静态依赖检查和脚本语法检查，并在浏览器逐项打开“今日工作、经营事项、主体库”、两笔事项、业务全景、资料与待办、变更历史及节点说明抽屉。模拟登记 40 万元回款后，当前事项的累计回款由 220 变为 260、未回款由 100 变为 60，时间线和历史同步出现模拟记录；重置后恢复原始数据。浏览器 console warning／error 为 0。入口为 [`acceptance/oms-validation-mvp-v0.2/index.html`](../acceptance/oms-validation-mvp-v0.2/index.html)，仅为本地内存态原型；真实数据、刷新持久化、后端、IAM、文件、部署和正式业务闭环均未验证。

2026-10-10 BF-123 OMS 合同资金验证 MVP 已实际执行 Workbench `GOWORK=off go test -count=1 ./...`、`GOWORK=off go vet ./...`、Demo JavaScript 语法检查、本地 `/health/ready` 与页面读取；浏览器实际验证四视角切换、详情抽屉、资金筛选和 0 warning／error。首次浏览器检查发现 `[hidden]` 被表格 `display:grid` 覆盖，修复后实际确认只有两笔回款记录可见且筛选按钮 `aria-pressed=true`。公网 `oatest /oms/` 仍是 BF-122 旧参考页，因为本次 SSH 连接 D1 超时；不得描述为已经更新部署。

2026-10-08 BF-122 OMS 资质单位参考 Demo 已实际执行 Gateway／Workbench `go test -count=1 ./...` 与 `go vet ./...`、Demo JavaScript 语法、`git diff --check`、15 应用静态架构边界审计、独立候选探活、桌面／390×844 浏览器交互与公网入口／资源检查；格式化后交叉编译摘要与上线候选一致。D1 上 17／17 容器健康，除 Gateway／Workbench 外的 15 个容器 ID 不变。复演范围、镜像摘要、回滚位置和限制见[验证记录](./reports/oms-qualification-unit-demo-validation-20261008.md)。这不是正式 OMS Owner API、IAM、恢复或 OA C1～C8 验收。

## 统计与四层权限 UI（2026-10-07，BF-121）

本地 `frontend/oms-workbench/frontend` 的 `npm run check`、`npm test` 206／206 和 `npm run build` 通过；Workbench／Billing 普通全量 Go／vet、Money HTTP Go／vet、access-contract Go／vet 与静态边界审计通过。未启用的真库条件测试或 integration tag 不计通过。

`node acceptance/oa-initiation-repair/verify-statistics-permissions-ui.mjs http://127.0.0.1:19117` 完整重跑 24／24：实际 Vite UI／生产桥接配内存契约来源，覆盖筛选、明细、分权导出、两链矩阵、成本／奖金、来源故障、迟到请求、49 项管理员规则、模板属性、意图确认／撤销、配置内容导出、同账号草稿重载、窄屏末项和正常工作台导航。回环独立 Workbench ready，通过预览导航且受保护请求 0。详见[结构化回执](../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/ui-fixture.json)及[验证记录](./reports/oa-statistics-permission-ui-validation-20261007.md)。

这一入口不证明正式 IAM、真实源库、D1／恢复、C7／C8 或完整 OA 通过。未部署／未补授 Grant；正式模板和临时授权写端口另验。项目详情问题依用户要求延后。

2026-10-06 BF-120 证据解释：本文件继续保存历史实际运行命令、失败和结果，但中断实验、临时 `/tmp` 证据、未跟踪代码／报告以及与 `AGENTS.md` 当前完成度冲突的记录，均降级为候选证据。它们只有在映射到当前 C1～C8 门禁、由当前代码和持久证据复演后，才能再次支持“已通过”。下一验证入口不是直接续跑下方 BF-112～BF-118，而是[当前基线与下一步](./status/current-baseline-and-next-actions-20261006.md)的 R0 清点与缺口矩阵。

## 原 OA 对外委托原生闭环（2026-10-06）

`node acceptance/oa-initiation-repair/verify-external-commission-native-browser.mjs /tmp/oa-external-commission-suite.JaPjHm` 已从全新私有夹具实际通过 10／10。长期[结构化回执](../acceptance/oa-initiation-repair/evidence/external-commission-native-20261006/external-commission-browser.json) SHA-256 为 `a408924290740099950a2fb13aefe136dddd93dc941125eeb7da64b32f95d9b4`，并保留[最终原界面截图](../acceptance/oa-initiation-repair/evidence/external-commission-native-20261006/external-commission-final.png)。

真实链覆盖原列表／统计／880px 抽屉、未知回复同 command 重放、Party 与发生时联系人快照、Contract 精确 revision、Evidence 预览下载、评分和纠正、Cost Payable Suggestion → Money 独立接受 → 显式同步、历史／撤回、返回缓存零读／显式刷新一读、八事实库逻辑空目标恢复、正式 IAM 撤权只读和单资源精确 Workload Scope。终态 2 委托、4 revision、1 assessment、1 suggestion、1 Money obligation、8 command／event，页面错误 0，恢复前后八库摘要和 Evidence 字节一致；Main 未改、本地 Docker 未启动、夹具已停止。

同批实际通过 Cost `go test ./...`／`go vet ./...`，Workbench frontend 186／186、TypeScript check 和 Vite 3204 modules build。Project 仅在创建边界被精确读且零写，Production 请求为 0，Contract 仅显式选择时精确读且零写。Money 拒绝／真实实付、逐依赖停机、跨 Tenant／全角色、正式 D1／Main／公网、第二客户和完整 OA 未运行，不得描述为通过。详见[验证报告](./reports/oa-external-commission-native-validation-20261006.md)。下一切片进入 F01／F02 资料管理，再进入 S01～S03 统计。

## 原 OA 批复／审计原生闭环（2026-10-06）

`node acceptance/oa-initiation-repair/verify-approval-audit-native-browser.mjs /tmp/oa-approval-audit-suite.MXO07p` 已从全新私有夹具实际通过 11／11。结构化回执为 `/tmp/oa-approval-audit-suite.MXO07p/approval-audit-browser.json`，SHA-256 为 `5f0da59077064961a7c19ae7c50e88ada930cca515b491e2be7fc809f16aa4e7`，运行时间为 2026-10-06 14:36:21～14:40:37（Asia/Shanghai）。临时路径不是长期制品，长期范围、限制和复演说明以[验证报告](./reports/oa-approval-audit-native-validation-20261006.md)为准。

本轮在原 OA 双列界面完成：空态零 Results 写入；批复冻结精确 Contract revision、金额和真实 Evidence；审计冻结精确 approval revision、三项整数金额和两份报告；响应未知时用同一 command 重放；批复纠正留史；审计撤回后在同一 aggregate 重登记；真实文件预览／下载；返回页缓存零读、显式刷新恰好一次 Results list read；正式 IAM 写权限撤销后只读／历史仍可用且写入为零；IAM／People／Project／Contract／Evidence／Results 六所有者逻辑空目标恢复后当前结果、历史和字节继续可用。终态为 2 个当前结果、5 个 revision、5 个 command、5 个 event，页面错误为 0；恢复前后六库 canonical digest 与 4 份 Evidence 字节 SHA-256 一致。

补测“审计撤回后不附新文件重新登记”真实发现合法空 Evidence 集合被 Owner 输出为 `null`。Workbench 的严格契约没有误把不完整响应当成功，并保留待续命令；修复放在 Results HTTP 输出层，统一序列化为 `[]`，新增回归后才从全新夹具重跑 11／11。两次早期失败仅为 Ant Design 将“刷新”的可访问名称渲染为带空格形式，验收选择器修正后仍执行完整重跑，未拼接局部结果。

同批实际通过 Workbench frontend 181／181、`npm run check`、Vite 3204 modules build，Approval & Audit Results 与 Unified Workbench 的 `go test ./...`／`go vet ./...`，以及 `make architecture-boundary-static-audit`（15 个应用、跨应用 import 0、replace 0、跨数据库 SQL 机制 0）。Project／Contract／People 摘要未变，Production 请求为 0；本批没有执行 Project Completion Checkpoint，也没有验证 Main／公网、正式 D1／keys、双 Tenant、全角色、账号停用、第二客户或完整 OA。

## 四类型生产总览／进度原生投影（2026-10-06）

`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs /tmp/oa-test-report-suite.tSki9f` 已实际通过 10 个场景。在既有 v5→v6、试验／报告、预览下载和精确移出基础上，新增总览验收确认：原 OA 四类型进度及 7 个适用阶段只由 Production 投影；首次进入总览 1 次 Production 请求，切走返回命中页面缓存，显式刷新才出现第 2 次请求；浏览器侧 Project／Evidence 读取均为 0，Production digest 零变化。正式 IAM 撤权保持只读零写；IAM／People／Project／Evidence／Production 五库逻辑空目标恢复前后 canonical digest、4 份 Evidence 字节 SHA-256 和签名密钥身份一致，恢复后原 OA 报告页继续。终态 1 Work Item、7 Stage、4 material（3 active）、1 ending、5 command、1 upgrade、4 snapshot、0 score；页面错误 0，Main 未改，fixture 已停止，本地 Docker 未启动。[报告](./reports/oa-production-progress-native-validation-20261006.md)。

首轮业务和恢复摘要已正确，但浏览器停留在新增“总览”页时，旧断言去等待隐藏“咨询报告”页的文件，导致 45 秒超时。修正为恢复后显式进入原报告 Tab，并使用全新空目标完整重跑 10／10；失败轮次未拼接为通过。

同批实际通过 Production `GOMAXPROCS=2 GOWORK=off go test -p 1 ./...` 和 `go vet ./...`、Workbench server／frontend Node 409／409、TypeScript check、Vite 3204 modules build，以及静态边界审计 15 应用／跨应用 import 0／replace 0／跨库 SQL 机制 0。未运行的 Main／公网、正式 D1／keys、双 Tenant、全角色、第二客户和完整 OA 不得描述为通过。

## 试验／报告原生链与 v6 类型快照（2026-10-06）

`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-test-report-native-browser.mjs /tmp/oa-test-report-suite.UjrJBU` 已实际通过 9 个场景：既有 v5 保持、勘察试验读取 v5 零写、同一 Work Item 一次 5→6 并冻结四类型、两份试验资料、咨询报告两份主文件且无评分／校审、真实预览下载、精确移出后报告仍完成、正式 IAM 撤权只读零写，以及 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复后原 UI 继续。终态 1 Work Item、7 Stage、4 material rows（3 active）、1 ending、5 commands、1 upgrade、4 type snapshots、0 scores；四份 Evidence 字节恢复前后 SHA 一致，五库摘要一致，零页面错误。本机 Docker 未启动、Main 未改变，夹具已停止。结构化回执保留于 `/tmp/oa-test-report-suite.UjrJBU/test-report-browser.json`；临时路径不是长期制品，长期范围以[报告](./reports/oa-test-report-native-validation-20261006.md)为准。

前两轮失败不计通过：低资源远端 PG18 经 SSH 时，3 秒 reference timeout 令 IAM workload exchange 被误判来源不可用；另一次浏览器 oracle 把文件选择器中的本地文件名误当服务端保存成功。验收运行时改用仍有界的 15 秒超时并记录换票状态／耗时，浏览器必须等待原 UI 成功文案后再核对持久事实，随后全新夹具重跑 9/9。

同批实际通过 `cd backend/services/design-operations && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...`、Workbench frontend `npm test` 178／178、`npm run check`、`npm run build`，以及 `make architecture-boundary-static-audit`（15 应用、0 跨应用 import、0 跨应用 replace、0 跨库 SQL 机制）。未运行的 integration tag、正式 D1／keys、Main／公网、全角色和完整 OA 不得描述为通过。

## 变更／竣工原生链与异类边界验证（2026-10-06）

`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-change-completion-native-browser.mjs` 最终在全新夹具 `/tmp/oa-change-completion-suite.1e2zf2` 实际通过 11 个场景：v4 前序保持、原双列表读取零写、取消零写、一次 4→5、变更多文件主文件限定、竣工独立完成、真实 Evidence 字节预览下载、追加式替换留史、移出／重加、正式 IAM 撤权，以及 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复后原 UI 继续。终态 1 Work Item、5 Stage、5 material、2 ending、8 command、1 upgrade、0 score；Project／People 摘要和五份 Evidence 字节保持，Contract 未进入运行组合，页面错误为零。本机 Docker 未启动、Main 未改变，夹具已停止。前轮实际发现数据库 stage key／sequence 约束和代码阶段注册表没有随 v5 同步，两项修复并加回归后才完整重跑。[报告](./reports/oa-change-completion-native-validation-20261006.md)。

同批实际通过 `cd backend/services/design-operations && GOWORK=off go test ./...`、Workbench frontend `npm test` 176／176、`npm run check`、`npm run build` 和 Workbench runtime 29／29。带 `integration` tag 的全套未另跑；真实 PG18／浏览器／恢复证据不能扩大为未运行测试集、Main／公网／正式 D1 或完整 OA 通过。

## 初步设计原生链与边界演变（2026-10-06）

`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-construction-stage-native-browser.mjs /tmp/oa-construction-suite.grfm4G` 已实际通过 10 个场景：既有 v3 保持、原初设建立前序事实、施工图读取零写、必填／取消、同一 Work Item 一次 3→4 且初设保持、两主文件／一校审／0 分、刷新预览下载、追加／评分／精确移出、IAM 撤权，以及 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复后原 UI 继续。终态 1 Work Item、3 Stage、1 upgrade、5 material rows、2 scores、1 ending、6 commands、1 upgrade history；Project／People 摘要和五份 Evidence 字节树保持，零页面错误。本机 Docker 未启动、Main 未改变，夹具已停止。首次运行在 runtime 未派发 construction 处失败，修复并新增 29 项 runtime 回归后才从新夹具完整重跑。[报告](./reports/oa-construction-stage-native-validation-20261006.md)。

`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-preliminary-stage-native-browser.mjs /tmp/oa-preliminary-suite.XQWiZ2` 已实际通过 9 个场景：既有 v2 保持、初设读取零写、必填／取消、同一 Work Item 一次 2→3、两主文件／一校审／0 分、刷新预览下载、追加／评分／精确移出、IAM 撤权，以及 IAM／People／Project／Evidence／Production 五所有者逻辑空目标恢复后原 UI 继续。终态 1 Work Item、2 Stage、1 upgrade、4 active materials、2 scores、1 ending、5 commands、1 upgrade history；Project／People 摘要和四份 Evidence 字节树保持，零页面错误。本机 Docker 未启动、Main 未改变，夹具已停止。[报告](./reports/oa-preliminary-stage-native-validation-20261006.md)。

同批实际通过 `cd backend/services/design-operations && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...`、Workbench `npm test` 169／169、`npm run check`、`npm run build` 和 `make architecture-boundary-static-audit`（15 应用、0 跨应用 import／replace／跨库 SQL 机制）。带 `integration` tag 的 Go 整套没有另跑；真实 PG18 migration／HTTP／浏览器／恢复证据不能被扩大为未运行测试集通过。当前只判定阶段资料子边界获得同类重复证据，Production 整体、正式 D1／keys、Main／公网、全角色与完整 OA 仍未通过。

2026-10-05 BF-110：`npm test` 在 Workbench frontend 实际通过 166／166，`npm run check`、`npm run build`、Workbench `go test ./...`／`go vet ./...` 通过；Business runtime 28／28，其中未接通阶段切换零通用读取。Main 发布 Workbench digest `sha256:be69a0a853608042f29e728876830f852917220c2aed07fb7b0c42297bd604e1` 后，运行 `node acceptance/oa-initiation-repair/verify-admin-production-permissions-browser.mjs /tmp/oa-admin-production-browser.<新后缀>`：10 个生产 Tab 全部 enabled，方案真实端口照常读取，初设／施工图显示明确未接线，变更竣工同时呈现两区；0 个 403／429／5xx／业务写／页面错误。滚动前后业务事实与 Grant 摘要不变，17／17 healthy。[结构化回执](../acceptance/oa-initiation-repair/evidence/oa-production-stage-navigation-20261005.json)与[报告](./reports/oa-production-stage-navigation-validation-20261005.md)。

2026-10-05 BF-109 补证：当前 12 份 Manifest 已增长为 145 项能力；再次运行 `grant-oa-admin-full-access.mjs` 得到 admin 145／145、缺口 0、创建 0，12 个 Audience Credential 与当前 Manifest 完全相等。公网 `verify-oa-admin-permission-page-browser.mjs` 证明 OA 规则层 7 组、49／49 全选、四层分离提示存在；`verify-admin-production-permissions-browser.mjs` 逐项打开人员、批复审计、方案、成本、外委和奖金，实际打开外委新增和奖金分配抽屉，奖金候选读取为 `200` 空集，0 个 403／429／5xx／业务写／页面错误。闭包分别修正 Gateway v3 ANY 入场、IAM 的 Bonus→Project 精确 Workload Policy，以及 Project 未选装人员 profile 时“列表 200 空集／单项 404”的来源端口语义；没有新增 Grant 或业务事实，17／17 healthy。回执见[管理员权限与原生 UI 闭包](../acceptance/oa-initiation-repair/evidence/oa-admin-native-ui-closure-20261005.json)；批复／审计缺少冻结来源时禁用是业务前置条件，初步设计／施工图／变更竣工 disabled 是适配器缺口，均不得用扩权绕过。

2026-10-05 BF-109：运行 `node acceptance/oa-initiation-repair/grant-oa-admin-full-access.mjs /tmp/oa-admin-full-access.<新后缀>`，从 Main IAM 实际装载的 12 份 Capability Manifest 得到 132 项能力；`admin` 原有 83 项，49 项缺口均通过原生 `/api/v1/admin/grants` 以 tenant scope 创建。随后 12 个 Audience 的 `/api/v1/credentials` 全部签发成功，Token 中 action 数与 Manifest 逐项一致，最终 active Grant 132／132。49 条 IAM grant audit 新增；12 个业务数据库、其他 Subject tenant Grant、全部 resource Grant 和 17 个运行容器均不变，0 次限流重试、0 次业务 API 写入。回执见[结构化证据](../acceptance/oa-initiation-repair/evidence/oa-admin-full-access-20261005.json)，定义与持续对齐规则见[报告](./reports/oa-admin-full-access-validation-20261005.md)。

2026-10-05 BF-108：Unified Workbench 页面读取改为 30 秒有界页面内存投影；首次显示 Skeleton，fresh 返回零读，stale 保留旧内容并后台刷新，并发读取合并，显式刷新绕过缓存。frontend Node 158／158、server Node 226／226、TypeScript、Vite 和 Workbench `go test ./...` 实际通过。公网执行 `node acceptance/oa-initiation-repair/verify-page-read-experience-browser.mjs /tmp/oa-page-read-browser.<新后缀>`：返回已读支付页 0 请求、显式刷新 4 请求、生产返回已读经营区 0 请求，0 个 429、0 个业务写入、0 个页面错误。只滚动 Workbench 镜像 `sha256:0dd14307b36810e2e3c692287eb75286734845ef06978b5ac371567bf0c38755`，其他 16 个容器 ID／镜像不变，最终 17／17 healthy；没有数据库、schema、Grant 或 Gateway 变更。结构化证据见[回执](../acceptance/oa-initiation-repair/evidence/page-read-experience-20261005.json)，范围与复演见[报告](./reports/oa-page-read-experience-validation-20261005.md)。

2026-10-05 BF-107：付款页“点几轮后请求过于频繁”确认为客户端请求放大，不是用户每分钟点击 120 次。修复保持 `REQUESTS_PER_MINUTE=120`：同页面同 Audience／Scope 的换票在 30 秒内复用，Scope 改变仍重新换票；不可变 Workbench 资产 GET／HEAD 不占业务预算，其他动态入口仍受限。`go test ./...` 已分别在 Gateway 和 Unified Workbench 模块通过，Workbench／frontend Node 全量 379 项零失败／skip；真实公网入口执行 `node acceptance/oa-initiation-repair/verify-rate-limit-repair-browser.mjs /tmp/oa-rate-limit-browser.<新后缀>`，同一验收项目付款页首次读取＋五次刷新得到 44 请求、8 次换票、0 个 429、0 资产失败、0 业务写入、0 页面错误。Main 仅滚动 Gateway／Workbench，其他 15 个容器 ID 不变，最终 17／17 healthy。证据及不负责范围见[报告](./reports/oa-request-amplification-repair-validation-20261005.md)；脚本读取既有管理员凭据只保存在内存，不输出密码，不创建 Grant 或业务事实。

2026-10-04 BF-106：`OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-stage-native-browser.mjs /tmp/oa-stage-suite.<mktemp后缀>` 已实际跑正常原方案UI和独立Nginx／Gateway／正式IAM／Production／Evidence／PG18，16场景零页面错误；整批保存、真实字节、原命令续办、撤权及五所有者逻辑空目标恢复通过。`verify-stage-material-remote-pg.mjs` 完成database1／含子1、design26／含子69、HTTP11／含子20，HTTP另2项旧fixture skip不记通过。`verify-production-stage-ui-browser.mjs` 原UI-only13项单列；`collect-stage-native-receipts.mjs` 封存当前源码摘要、377项Node零skip、Go／定向race／vet／TS／构建、37项profile guard与D1升级选择mock和正式IAM preflight。[实际回执及复演限制](./reports/oa-production-stage-native-validation-20261004.md)。五库逻辑恢复复用保留key文件，不是实际D1／keys归档／旧会话失效；Main未发布、全OA未通过。本机Docker全程停止，7个独立测试进程、远端夹具和测试Chrome结束后停止；勿将任何驱动改成Main目标。

2026-10-04 所选 D1 补证：`acceptance/oa-initiation-repair/verify-professional-d1-restore.mjs` 已实际构建／启动八应用＋Nginx／PG，六库及 keys／Evidence 本地字节联合备份→新空目标恢复→原奖金／人员 UI 继续，14 场景零页面错误；错 profile 删卷前拒绝、旧会话失效、原 command 重放零新增、逐业务表 canonical 相同。所有运行容器停止，私有卷／备份保留；[实际回执／限制](./reports/oa-professional-selected-d1-restore-20261004.md)。仅本地 HTTP／build-mode 所选组合，非全 Suite／历史升级／Main／公网／全 OA。下一[原阶段对照](./specs/assembly/oa-production-stage-original-operation-map.md)的 UI 层与原生层须分开记账。

2026-10-04 最新 D01→B12：原人员／奖金 UI＋真实7进程及Nginx，20场景零页面错误；隐藏零读、原保存／重载／撤回、错版本和结束来源拒绝、未知回复原命令＋Bonus空目标恢复、撤权、快速切页scope和存储故障分层通过。Project integration PG18 163／含子418、Bonus integration PG18及race各32／含子93，零实际test skip；Node347、TS、Go／vet通过；其中24条部署／恢复profile门禁加升级选择mock，不等于实际Suite恢复。复演入口为 `acceptance/oa-initiation-repair/verify-production-bonus-native-browser.mjs`，安全回执与限制见[连续报告](./reports/oa-professional-bonus-continuous-validation-20261004.md)。不是Main／公网／Suite恢复／全OA验收，旧失败保留。

2026-10-04 D01 原生候选：原人员 UI 接真实独立 IAM／People／Project＋PG18，21 场景通过，零页面错误；专业多人／六角色、跨专业、占用冲突、源版本／停机、未知回复 reload 原命令续办、隐藏参与保留及恢复、目录新增、可信 Actor 和 profile off/on 重启均分层实测。Project 普通 119／含子 363、全量 PG18 161／含子 411、专项 race 14／含子 20 通过，零测试 skip；Node 315／315、TS、Workbench Go／vet 通过。首轮宿主 PG 工具及两种驱动形状失败保留；原 UI-only 最终 16 条单列，旧 15 条隐藏移出约定已取代。[原生报告](./reports/oa-production-personnel-native-validation-20261004.md)和[UI 层](./reports/oa-production-personnel-ui-candidate-20261004.md)。Main／正常 bundle 未更新，角色矩阵、新引用／Bonus 消费、Suite 恢复和公网待验，不能关闭 D01。

2026-10-04 生产布局续验：原 `ProductionInfo` 总览／页签及逐字 CSS 已候选挂载，选中页才读 Owner，`production_tab`／键盘／刷新／Back／Forward 通过。最新真实 Workbench＋Gateway＋IAM＋独立 Bonus／PG18 为 17 有限场景；成本／合同／支付／开票原组件测试端口分别 13／22／14／15 场景、各零页面错误。307 项 Node 零 skip、TS／构建、Workbench Go／vet及两次独立启动通过；失效能力名／POST 回执／定位夹具反例保留，没有放宽校验。Main 未发布／未加权，9 个生产子栏目未接，不能宣布完整生产／OA 通过。[本批证据和复演](./reports/oa-production-layout-ui-validation-20261004.md)。

2026-10-04 续轮 BF-102／103：`verify-bonus-components-browser.mjs /tmp/oa-bonus-components.<mktemp 后缀> --live-sources` 已跑原组件与实际 Workbench→Gateway→真实 IAM→Bonus／PG18，14 有限场景、0 页面错误；两侧导航、正常登记／撤回、金额撤权、来源故障零写、空目标恢复及原命令续办通过，非公网／Nginx／D1 整体恢复。前端 121＋embedded runtime 179＝300 项零 skip、TS／Vite、Bonus `-count=1 -tags integration` 真库、Gateway／Workbench／Identity／Access Contract、独立启动及静态边界＋负例实际通过。匹配 Namespace／entry 的默认及显式 profile loader 正负例保持旧入口；没有给 Main 加 Grant／发布。精确事实、失败及复演以[最新报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)为准。

2026-10-04 08:32 UTC 奖金候选：Bonus 普通／PG18 integration 全包、签名 IAM 测试 HTTP／脱敏、schema 2→3 旧 Pool／建议保持通过；前端 Node 112／112 零 skip、TypeScript／Vite、Workbench、Identity／Access Contract 及静态边界通过。`verify-bonus-components-browser.mjs` 真实独立 Bonus＋PG18 跑原组件九场景，空目标恢复和原命令续办／继续写通过；People／Project 参考夹具、template 身份，不是公网。D1 配置仅解析，零启动；Main 未发布／未加 Grant、17 健康。精确复演、失败与限制见[报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)及[回执](../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/browser-native.json)。

2026-10-04 07:43 UTC 两链三费用原支付表单只读取消复验：`verify-cash-cancel-native-ecs.py before` 冻结十二业务库／序列与两类 Grant 后，`verify-cash-cancel-public-browser.mjs` 六场景通过；原表单、三类可选费用和取消均实际正常点击，零页面错误、零业务写入请求。after 收尾证明全部事实／授权不变，C revision 18、17 健康；[公网回执](../acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004/browser.json)和[原生收尾](../acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004/native-closing.json)。资金开票关联仍明确未接，六组合保存和全角色仍待验，不能把取消矩阵当真实资金登记。此次 `make architecture-boundary-static-audit` 及负例自测实际通过，15 独立应用、零跨应用导入／replace／跨库 SQL，仍不代替运行期整体架构验证。

2026-10-04 07:36 UTC 最新入口状态（优先于以下历史批次）：匹配 Contract 22／Money 23／Billing 1／Project 11、Cost 3 与 Workbench `.6` 已发布。原 C 的合同／支付驱动九检查点有限通过：两链合同、三协议、两定案、单文件上传／预览／精确下载、两 Cash／冲销及真实已提交丢回复原命令续办；可见历史区另有只读复验。原生边界收尾已通过，Project revision 18，其他项目、旧事实、旧文件树及 Tenant Grant 保持，仅批准的 14 条成本精确 Grant。成本有限链又补八个分类场景／16 原生命令，合计十二记录／24 命令／revision／审计，覆盖经营七类／生产五类；[分类原生收尾](../acceptance/oa-initiation-repair/evidence/cost-category-public-20261004/native-closing.json)核对旧四条及历史、十个其他业务库、Evidence 全事实／审计及两类 Grant 不变。连续撤回弹层遮挡真实复现并在 `.6` 修复，12 场景组件端口、282 项 Node 零 skip、Go／vet／前端构建、两次独立启动以及发布门禁分别计账；[单 Workbench 发布收尾](../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/rollout.json)不代替整套 OA。商业驱动整体 `passed:false` 是 Billing 独立授权阻断，不抹为通过。公开[商业回执](../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/native-footprint.json)与[可见复验](../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/visible.json)归档，私有 Cookie／session 不归档。`exercise-commercial-public-browser.mjs`、`exercise-cost-category-public-browser.mjs` 只能用原私有目录和 `OA_CONTINUE_EXISTING=1` 续办，不覆盖分类 before 或重跑旧 Cost 收尾比较合法变化后的 Contract／Money。当前 wrapper `/opt/omsbizfoundry/acceptance/oa-cost-modal-20261004a/d1-active`；17 服务健康，完整 OA／全角色／完整 Suite 恢复及 OSS 未通过，见[报告](./reports/oa-commercial-public-ui-validation-20261004.md)。

2026-10-04 当前公网：匹配经营 Owner 和 `.5` 成本＋Link 回执修复已通过升级／冷备／13 库空目标数据恢复／离机校验并上线，17 服务健康；不等于完整 Suite 恢复运行。103 项前端（0 skip）、TypeScript／Vite、Cost／Workbench Go、两次独立资源启动、成本原组件 11 场景、分批 PG18 6／29 项及 d 的两项正式 Ed25519 IAM HTTP 通过。成本原 C 的经营／生产原 UI 连续操作有限通过：四条记录、八个原生命令／revision／审计、一个实际 Evidence 文件；追加纠正／撤回留史、两侧隔离、字节下载及三个正常刷新丢回复续办已验，九个检查点、24 次可见命令包含失败阶段幂等重放，不能称 24 个事实。最终原生收尾确认其他十个业务库、旧 Cost／Evidence、用户101、全租户 Grant 保持，只有批准的 14 条成本资源 Grant；[收尾](../acceptance/oa-initiation-repair/evidence/cost-public-20261004/closing-after-cost-public.json)。失败／skip 批次保留，原源 47 份摘要一致、零写原目录。`grant-cost-acceptance-project.mjs` 仅执行批准的精确 Scope，`exercise-cost-public-browser.mjs` 为真实成本驱动；`exercise-commercial-public-browser.mjs` 已产生事实的原 C **只能**用 `OA_CONTINUE_EXISTING=1` 和原私有目录续验，不换项目。Cookie／session 不归档。Billing 另等授权，合同／资金仍续验，全角色、完整 OA 与 OSS 未通过；见[公网报告](./reports/oa-commercial-public-ui-validation-20261004.md)。

2026-10-04 BF-101 [成本扩展验收](./specs/assembly/oa-cost-receipt-overlay-v1.md)已有限通过上述同项目公网主链；七类适用性、两侧 Scope、旧 project Scope 拒绝、金额脱敏、null 字段、命令重放／历史另有分层证据。旧事实兼容、空目标数据恢复、匹配发布及正式入口分别计账；完整角色／整体旅程仍待验，不由有限管理员链升格完整 OA。

2026-10-04 最新回归：全量 Workbench frontend／server Node 262 项通过、零 skip；selected-only、隐藏来源零读、故障无替代、旧回复抑制和 PUT／PATCH／DELETE 原生写回执已验。Contract ae 22、Cash j 14、Invoice f 15 场景使用同 bundle；各自真库／启动分层计账，Workbench native c 两次独立 HTTP 资源启动见 [startup c](../acceptance/oa-initiation-repair/evidence/workbench-ui-candidate-startup-20261004c.json)。Contract 真实 issuer＋Evidence／字节恢复 m、历史夹具 18→22 四项原子迁移 a，Billing 最新 c HTTP／最后附件真库两项及真实 issuer＋Evidence／字节恢复 e 均已实际运行通过、无 skip；Main 未升级，局部来源夹具不代表 Project／Party 真进程。入口 `verify-invoice-material-combo-e-ecs.py`、`verify-invoice-candidate-c-ecs.py`、`verify-contract-schema18to22-ecs.py` 串行私有 PG、各自安全回执；普通 Go 内可能 skip 不算专项通过。候选未发布、OSS／完整 OA 未通过，[合同](./reports/oa-contract-draft-ui-candidate-20261004.md)、[支付](./reports/oa-cash-ui-candidate-20261004.md)、[开票](./reports/oa-invoice-ui-candidate-20261004.md)记录证据限制，以下旧计数为历史。 最新构建、截图与 Main 17 服务只读收尾统一见[本轮检查点](./reports/oa-complete-ui-checkpoint-20261004.md)。

2026-10-04 最新候选：Workbench Go＋vet、前端 check／build、Node 235 项（0 skip，新增 Cash adapter 12 项）通过。Contract 最新 bundle 的 ac 浏览器 22 场景／32 命令／0 页面错误仍通过；真实 IAM＋Contract＋Evidence m 已完成三类上传引用、精确下载、401／403／停机零写、应用 DB 与实际 bytes 联合空目标恢复，Main 前后保持且 private runner／PG 停止。仅 local/private，不能称 OSS 或公网全 OA。支付浏览器 a／b 失败保留，修复控件可访问标签后续验；PG18 与独立启动在补证。执行入口与各层限制见[支付报告](./reports/oa-cash-ui-candidate-20261004.md)和[合同报告](./reports/oa-contract-draft-ui-candidate-20261004.md)。

2026-10-04 BF-098 候选：Contract／Workbench Go＋vet、前端 check／build、Node 223 项（0 skip，合同适配器 23 项）通过。schema 22 私有 PG18 专项、完整应用库空目标恢复、独立空库 ops migrate→verify／重启及 14 个匿名拒绝通过；g 均无 skip、Main 前后保持、探针停止。原组件 z 的 22 场景／32 命令／0 页面错误通过，但为内存端口，不能代替真实 IAM／Evidence 联合或公网验收。原始合同、三类型协议、材料与结算已分层接线，最终更正、全角色、联合字节恢复／发布仍待验；[报告与失败回执](./reports/oa-contract-draft-ui-candidate-20261004.md)。

本批入口为 `verify-contract-overlay-candidate-ecs.py`、`verify-contract-overlay-startup-ecs.py` 和 `verify-contract-components-browser.mjs /tmp/oa-contract-components.<新后缀>`。云端仅专用内部网络／数据库、0.2 CPU／无公网端口，不得改为 Main；不能将一项 PG 或四项组件结果称整套 OA 通过。安全回执只归档 JSON／截图，不收私有 env、日志凭据、dump 或 key。下方 schema 19／未接原表单是历史批次。

2026-10-03 BF-097 限定通过：Project 私有 PG18 全量 Project 118／数据库 4／HTTP 98 条含子测试、schema 11 独立启动／重启、十个匿名拒绝通过。原人员组件夹具五场景、Workbench Node＋两个适配器 200 项零 skip、Go／vet、前端 check／build及两次独立 Workbench 启动通过。`.3`～`.6` 四批联合备份／13 库空目标数据恢复／文件和 IAM key 字节／离机 SHA、17 healthy／D1 check 通过；不是完整 Suite 恢复运行。原 C 真实添加／结束／重入和未知回复原命令恢复已验，最终 n 公网三项零业务写入导航／可见来源读取通过，快速切页无 429，所有其他业务事实保持；早期路线缺失、429、定位和 networkidle 失败均保留，不重置项目。完整 B02 与全 OA 仍未通过，见[本批报告](./reports/oa-complete-operation-progress-20261003.md)。

当前运维 wrapper 为 `/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a/d1-active`。单元入口：`node --test frontend/oms-workbench/internal/server/*.test.mjs frontend/oms-workbench/frontend/src/*.test.mjs`。公网只读驱动 `verify-contributors-navigation-browser.mjs` 需 `OA_CONTRIBUTORS_PUBLIC_CHECK=1` 和新目录，固定原 C revision 13；不得重跑已提交的 d／i／j 写入驱动。最终原生冻结门禁为 `verify-contributors-login-metadata-ecs.py` → `verify-contributors-ownership-ecs.py after`，保留原 before，不整表排除账号；安全导出用 `OA_CONTRIBUTORS_COLLECT_FINAL=1 node acceptance/oa-initiation-repair/collect-contributors-evidence.mjs`，禁止把 env、dump、key、凭据和私有备份归档进 Git。只读收尾 `verify-contributors-closing-ecs.py` 实际 check exit 0／17 healthy、所有探针及 restore20 停止；主 Contract schema 18 保持，与隔离候选 19 分开。

Contract 候选 `workspace_handlers.go` 复用原领域用例，普通 Go／vet 通过；私有 PG18 专项 `TestWorkspaceNativePostgresExactMoneyHistoryAndReplay` 实际一项通过，涵盖超 JS 安全整数金额、Tenant／精确 Contract Scope、历史／幂等／相邻拒绝，未部署、没有真实原合同表单点击。入口为 `verify-postgres-ecs.py` 的限定 Contract 专项，不能将一项表述为全量真库。独立启动 c 的自有空库 ops migration→server verify、schema 19、九个匿名拒绝、重启不变通过；a／b 原失败保留，私有 PG／探针已停止、主站保持。[安全真库回执](../acceptance/oa-initiation-repair/evidence/contributors-20261003/contract/results.json)与[独立启动回执](../acceptance/oa-initiation-repair/evidence/contributors-20261003/contract_startup_c/results.json)。

2026-10-03 历史登录故障已随 `.20261003.2` 修复，仅 Workbench 变化；当时入口 `/opt/omsbizfoundry/acceptance/oa-login-csrf-20261003b/d1-active` 已由上方当前 wrapper 取代。公网正常、模拟 CSRF 过期、多标签更新、503／403 手动重试五项通过，另一次全部脚本与原生 Project Directory 冒烟通过。181 项 Node／Go／vet／两次独立启动、13 库空目标恢复及部署前离机校验通过；业务／Grant 保持、17/17 healthy、D1 check 通过。见[实测及限制](./reports/oa-login-csrf-repair-validation-20261003.md)，不改变完整 OA 的待办结论。

登录专项：`node --test frontend/oms-workbench/internal/server/reference_oa_login.test.mjs frontend/oms-workbench/internal/server/reference_oa_csrf.test.mjs`。公网驱动 `acceptance/oa-initiation-repair/verify-login-csrf-browser.mjs` 必须明确 `OA_LOGIN_PUBLIC_CHECK=1` 和新证据目录，`OA_LOGIN_FULL_WORKBENCH_SMOKE=1` 单独复核完整脚本；仅在内存读取已授权私有凭据，不输出密码、不创建或重置账号。

2026-10-03 BF-095／BF-096 当前结果：原招投标 React 组件、样式、预览和确认弹窗已真实挂入 Workbench，`.20261003.1` 发布 BE＋Workbench 两项；当前 local 文件链真实上传、预览／下载字节、单份移除、Project 投影和刷新零写通过。public-h 在同一 G 真实丢弃已提交 append 回复后 reload／相同 command ID 与载荷继续通过，最终 Project revision 4、BE revision 15、七份历史／六份有效引用；f/g 的失败及已提交事实保留，不换夹具或放宽限流。见[本批报告](./reports/oa-reused-bidding-ui-validation-20261003.md)及[续验回执](../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/public-h/public-retry.json)。

该附件批次本地 `node --test frontend/oms-workbench/internal/server/*.test.mjs frontend/oms-workbench/frontend/src/bidding-adapter.test.mjs` 实际 172 项零 skip；Go／vet、SDK 17 项、前端 check／build、原生工作台启停及资源摘要通过。13 库联合备份／空目标数据恢复及 migration 保持通过，不代表完整 Suite 恢复运行；部署时其他 15 ID 保持、17/17 healthy。其运维入口已由本页登录修复条目更新。Main Evidence 仍 local、前端仍 ECS，OSS、B03 完整负例／撤权、原站现场与全 OA 未通过。以下候选未部署和 `.11` 是历史批次。

2026-10-03 BF-095：BE 多附件候选后端在私有 PG18 的 d／e 批次各通过 17／1／22 项，旧 schema／hash／回执兼容、真实空目标恢复继续、同租户相邻承接附件拒绝及独立正式 IAM 启动通过，Main 不变、探针已停。未部署迁移、未验公网附件；见[实测报告](./reports/oa-bidding-material-collection-validation-20261003.md)。下一项为 Evidence→BE→Project 同项目文件闭环，不拼接后端证据为 B03 完成。

同批 Workbench 候选新增 28 项材料和 12 项架构透镜检查，全量 Node 156 项零 skip、Go tests／vet及原生独立启动／重启和嵌入资源核对通过；Evidence 正式适配器尚未接线，附件界面／公网文件链未验。四小时上限前只读收尾：k check exit 0、17/17 healthy，BE 000001／Evidence local 保持，私有探针及 restore20 已停；见[收尾回执](../acceptance/oa-initiation-repair/evidence/bidding-materials-20261003/closing-check.json)。候选与公网 `.11` 必须分开表述。

2026-10-02 BF-093／BF-094：`.11`／k 已部署，真实公网 Chrome 三方式 → 不适用 → 恢复 → 刷新及八阶段原生复读通过；同一 G／同一 BE，最终 Project revision 4、BE revision 7，刷新零新增、用户101／Grant／旧事实保持。仅 Project 既有端点配置和 Workbench 变化，schema／权限不变。13 库空目标恢复、离机 SHA、独立 postflight／当前 k wrapper check 与 17/17 healthy 通过；apply 的挂载顺序误判失败保留，不声称缺失的部署前原生挂载快照已比较。116 项 Node、17 项纯门禁及两项制品完整性检查通过。文件、多附件、原站现场等效、完整 Suite 运行恢复及完整 OA 未通过。见[切片规格](./specs/assembly/oa-bidding-command-slice.md)及[逐项结果和反例](./reports/oa-bidding-command-validation-20261002.md)。

日期：2026-10-02
状态：完整 Suite 已在公司 Ubuntu 同一 Tenant／Project 连续通过 C0～C7；C8 完整运行后保留为 `insufficient-evidence`。C8 已通过 49/49 HTTP 状态、当前 Subject／相邻 Subject 隔离、Money grant/revoke 脱敏、Evidence／Production 顺序故障恢复、十二库 canonical facts 前后相等和 17/17 health；但 C4～C6 输出缺少 active Project Link／资料投影，且 Gateway 遗留 `evidence-service.view` 阻断精确 `metadata.read + download`。Evidence 页面 `redirect` 已在源码修为契约支持的 `direct`，C7/C8 verifier 已进入 D1 release bundle；完整 OA 仍不得标记通过。

## 2026-10-02 同项目业务页签修复（BF-092）

Workbench `.6` 已部署九个经营栏目与生产信息的导航及已有原生只读接口。Node 18＋44＋7＋8 共 77 项、Workbench Go test／vet、JS／Python 语法通过；公网 native Chrome 逐页点击、奖金刷新／Back／Forward 和甲方编辑取消保护通过。13 库空目标数据恢复／离机 SHA、点击后 12 业务库与 Grant 保持、17/17 healthy 和 D1 check 通过。当前项目无商业 Link、Cost／Bonus 有效 Grant 缺失；有金额记录的公网展示、完整经营参与人、写入表单、角色矩阵和 OA 未通过。真实证据以[本批报告](./reports/oa-business-tabs-repair-validation-20261002.md)为准，不把页面可切换当成完整业务通过。

本地入口：`node --test frontend/oms-workbench/internal/server/reference_oa_business_runtime.test.mjs frontend/oms-workbench/internal/server/reference_oa_initiation_runtime.test.mjs frontend/oms-workbench/internal/server/reference_oa_project_directory.test.mjs frontend/oms-workbench/internal/server/reference_oa_csrf.test.mjs`。Auth58 `.6` 专用只读核对为 `verify-public-business-tabs-readonly-ecs.py`，不调用旧补 Grant 的 delivery main、不复用用户修改前的冻结基线；业务库比对本批联合备份，真实点击另外记录。

## 2026-10-02 Auth58 未知回复与抽屉恢复（.5 历史批次）

`.5` 仅 Workbench，Project／IAM／schema／Grant 不改。原生 Chrome 的内存型 opt-in probe 明确模拟“已立项但首个回复丢失”：原版误称未完成，第一修复仍有旧抽屉遮挡，两项修复后重新加载读同一详情才关闭抽屉；保留输入且不重放写入。故障模拟不是公网／真实 IAM／数据库证据。Node runtime 44＋目录 7＋CSRF 8 项、Workbench Go test／vet、Python 纯门禁 6 项及语法检查通过。

`OA_PUBLIC_RUN=e` 串行 publication → prepare → backup → restore → apply → export_backup；13 空库事实／序列、文件／key 和离机 SHA 通过。公网真实旧立项深链、返回、Enter 编号搜索、打开及刷新后，`verify-public-continuous-browser-ecs.py post-ack-message-release` 只读核对精确 `.5`、C revision 7／9 回执和所有旧 Project／用户101／Grant／其他11库保持；不覆盖原 final 或失败。浏览器后 `a/d1-active check` exit 0，17/17 healthy，私有 PG／restore20 已停，本地零 Docker。完整字段／角色／OA、完整 Suite 恢复及 Evidence／Web OSS 未关闭，实际 Evidence provider 仍 local。

见[报告](./reports/oa-acknowledgement-recovery-validation-20261002.md)、[隔离反例](../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/synthetic-browser-observations.json)、[公网原生复核](../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/public-native-readback.json)及[交付复读](../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/deployment-readback.json)。

## 2026-10-02 Auth58 安全令牌修复与连续续验（.4 历史批次）

`.4` 只重建 Workbench，配置注入 IAM 路由；Project／People 授权共享当前 Cookie／缺失更新 helper，不延长安全期限、不关闭鉴权、不改 Grant、不自动重试业务。解锁后明确空选择仍报 CSRF 失败的原生反例保留；正常刷新同会话清除生产为 revision 5、经营为 6。新版本在原表单已填两类负责人的情况下，仅让 CSRF Cookie 到期，确认缺失后直接点保存成功；Gateway 日志是一条更新 GET、正常授权及一次业务 POST。

同一 C 的编号 Enter 搜索、重新打开、详情刷新、页面返回及浏览器 Back／Forward 实测通过；最终 `final` 验证一次 activation、清除／恢复历史不覆盖、取消／无效输入无增量、no-op 仅一个回执、最终 revision 7／九个回执。用户原 Project101、所有旧 Project 行、Grant 和其他 11 业务库保持。13 库空目标数据／sequence、文件／key 恢复、离机归档 SHA、D1 check（浏览器后再次实际执行）、17/17 healthy 通过；不是完整 Suite 恢复运行。

串行 Node runtime 37＋directory 7＋CSRF 8 项零失败／skip；Workbench Go test／vet、Gateway 原有安全回归、Python 纯门禁 6 项与语法检查通过。opt-in 合成浏览器未在此批运行。私有 PG、restore20 保持停止，本地零 Docker。见[续验报告](./reports/oa-csrf-continuation-validation-20261002.md)、[点击观察](../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/browser-observation.json)、[最终原生检查](../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/native-checkpoints.json)及[发布复读](../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/deployment-readback.json)。只关闭当前管理员核心普通项目路径；完整字段、角色／撤权／部分成功、项目包与原站真人未验，不关闭 I01～I04 或完整 OA。

## 2026-10-02 Auth58 公网导航故障与连续操作（.3 历史批次）

真实 Chrome 发现“C 立项抽屉显示正确、URL 却仍指旧 B；刷新进入 B”故障；Workbench 导航已修复并部署 `v0.1.0-oa.20261002.3`。只重建 Workbench，其他 16 容器不变、schema 10／IAM／Grant 不变；13 库及文件空目标数据恢复、离机摘要、D1 check 和 17/17 healthy 通过。私有 PG、restore20 保持停止，本地 Docker 未启动。

同一 C 真实派发自动开抽屉、空编号／空类型拒绝、刷新同抽屉、取消清 ID、无生产负责人完成同 OA 详情、编辑取消／必填拒绝、后补负责人及日期、no-op 已复读。用户原项目及全部旧 Project 行、Grant、其他 11 业务库保持。一次清除尝试仍为 revision 4／两名负责人，不记通过；随后 Mac 锁屏，清除／恢复历史、列表搜索和 Back／Forward 未继续。本批连续旅程未结束，不能关闭 I01～I04。

Workbench Go test／vet、Node 37＋7 项零 skip、Python 纯验收门禁 6 项、JS／Python syntax 和 diff check 通过；新增导航及显式清除控制器用例，后者只是单元证明。复读脚本补上各阶段状态／revision／分工断言，保存失败观察；不把“读取成功／数据未变”当清除成功。原站真人、费率资质等完整字段及角色矩阵仍待验。见[本批报告](./reports/oa-navigation-continuous-validation-20261002.md)、[点击观察](../acceptance/oa-initiation-repair/evidence/navigation-20261002/browser-observation.json)及[原生检查](../acceptance/oa-initiation-repair/evidence/navigation-20261002/native-checkpoints.json)。

## 2026-10-02 Auth58 旧待立项恢复（BF-091，.2 历史批次）

当时 Main `v0.1.0-oa.20261002.2` 已关闭缺经营分工＋已有生产分工时的提交阻断。Project／Workbench Go 测试、Node 29 项，Auth58 私有 PG18 Project 114／数据库 3／HTTP 86 条含子测试 PASS，独立空库 schema 10、重启不变和 9 个匿名拒绝通过。新增命令只使用原生既有能力，本次不改 schema、Manifest／Policy 或 Grant。

仅替换 Project／Workbench，其他 15 容器 ID 不变，D1 check 和 17/17 healthy；13 库逐个空目标恢复的事实／sequence 及 Evidence／IAM key 字节相等，离机归档校验一致。数据恢复不等于完整 Suite 恢复运行。本地 Docker、restore20 未启动。

真实 native Chrome 在 HTTPS 打开用户“测试项目101”的抽屉，明确缺经营分工补齐栏、已有生产负责人回填、重新加载保持，随后取消，未代选／提交。专用夹具 `OA-RECOVERY-20261002-B` 通过 API 预置旧失败状态（准备不是点击证明），真实点击明确补齐经营负责人并更换生产负责人→完成→同 OA 详情→刷新。原生 `include_inactive=true` 复读 active revision 5，旧分工在 revision 4 结束且快照保留；全部既有 Project 行及用户项目／分工／历史摘要保持。首次复读漏带历史参数的断言失败保留，修验收查询后只读重跑，无第二次保存。

见[修复报告](./reports/oa-initiation-recovery-repair-validation-20261002.md)、[部署／复读证据](../acceptance/oa-initiation-repair/evidence/recovery-20261002/deployment-readback.json)、[真实点击观察](../acceptance/oa-initiation-repair/evidence/recovery-20261002/browser-observation.json)及[验收入口](../acceptance/oa-initiation-repair/README.md)。仅该管理员恢复路径通过，费率、资质等字段、全角色撤权、原站真人和完整 I01～I04 未验收；完整 OA 仍未通过。运维仍用 ECS `oa-public-release-20261002a/d1-active` 保留相同 override。

## 2026-10-02 Auth58 Main 首条 OA 修复链（.1 历史批次）

当时结论按 BF-090 收窄：**新夹具正常路径通过，不是立项功能验收通过**。.1 中用户“测试项目101”阻断；2026-10-02 公网只读复核确认 `pending_completion` revision 6、仅生产分工、无经营分工，三次 pending 历史均明确缺 `business_owner`。读取前后 Project 事实摘要相同，业务／Grant 写入为零，不修改该记录。见[只读反例证据](../acceptance/oa-initiation-repair/evidence/user-initiation-counterexample-readonly-20261002.json)及[分步清单](./specs/assembly/oa-original-usage-workflow-baseline.md)。此前诊断轮未运行原站浏览器或修复后的克隆点击；后续 .2 恢复见上一节，不覆盖历史证据。

只读诊断脚本：[inspect-public-initiation-readonly-ecs.py](../acceptance/oa-initiation-repair/inspect-public-initiation-readonly-ecs.py)，在 Auth58 使用现有 release helper 执行，不调用会补 Grant 的 delivery `main()`。只有登录、换票、注销为 POST，业务仅 GET，凭据不输出。它只核对这一反例，不证明按钮、角色矩阵或恢复完成。下一实施门禁先验证旧草稿、已有负责人不变／更换、资料／分工部分成功、刷新／重试、撤权和同项目下一步，不先推进 B02。

已部署 `v0.1.0-oa.20261002.1` 两个 ACR digest，并显式选 IAM Project Manifest 0.6／Policy 1.2；13 库、Evidence／IAM 文件先联合备份、逐个空目标还原，Project 7→10 的旧事实保持和重复 migration 通过。D1 check、17/17 healthy、其他 14 容器 ID 不变。普通项目公网真实 Chrome 点击完成派发、无生产负责人立项、同 OA 详情、后补负责人和资料、甲方保存／刷新／Tab 补全、返回目录；只读复核夹具 revision 5、activation revision 3、原有 Project 记录和其他 11 库不变。见[部署报告](./reports/auth58-oa-first-slice-deployment-validation-20261002.md)和[机器／点击观察证据](../acceptance/oa-initiation-repair/evidence/public-release-results-20261002.json)。

这是管理员健康控制路径与开发测试部署，不是完整角色矩阵、完整 Suite 恢复运行、项目包公网点击、全站仿制或正式 clean-HEAD 交付。Main Evidence 仍是 Local；C8 和 OSS 两条待办未关闭。当前运维必须使用 ECS `/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/d1-active` 保留 override。下方各节“Main 未更新／分层”是此前候选批次的历史状态，不得删除后拼成整站通过。

## 原 OA 历史甲方名称行内补全（BF-089）

本地 Project／Workbench Go 测试与 vet、JS 语法、Node 32 项零 skip 通过。私有 PG18 `r` Project 112／HTTP 85 条含子测试 PASS、零 fail／skip，覆盖自身历史／创建快照名称、可信 Tenant／Project Scope、空资源范围、仅名称输出、字面搜索及 GET 零写；`s` 独立空库 schema 10、八个匿名 401、重启不变。Main 摘要／容器 ID 不变，测试与 PG 已停。

合成浏览器重测输入框样式、Tab 正常焦点／Shift+Tab、前缀／包含提示、清空取消零写、一次显式保存、复读／刷新同项目经营页及无效项目链接可见错误／重读恢复。首次失败保留说明，未归档截图文件；不是正式 IAM／Gateway／PG 联合浏览器、B01 全项或 Main 通过。见[报告](./reports/oa-client-name-clone-repair-validation-20261002.md)、[验收入口](../acceptance/oa-initiation-repair/README.md)。旧 BF-088 的 29 项／七路由属于旧批次，不覆盖重写。

追加 `OA_CLIENT_NAME_READ_FAIL=1` 合成探针 88.12 秒 PASS：名称读取 503 时提示隐藏、Tab 不改手工名称，保存前零写、一次显式保存后复读／刷新同项目保持。名称读两次分别为浏览器一次及只读状态诊断一次；[故障摘要](../acceptance/oa-initiation-repair/browser-client-name-failure-synthetic-state-20261002.json)单列夹具摘要、停机与临时页关闭。延迟／迟到响应与正式撤权浏览器矩阵未由此关闭；最后无容器 Workbench Go／vet、JS 语法和 Node 25＋7 项重跑通过。

## 原 OA 项目内甲方信息（BF-088）

Project／Workbench 无容器 Go 测试与 vet、IAM 相关策略测试／vet、Node 29 项零 skip 和 JS 语法通过。私有 PG18 `o` 的 Project 109／HTTP 75 条含子测试 PASS；`q` 追加联系资料的整库备份／空目标恢复、身份与历史不变、离线重放零写及恢复后修改，全 Project 110 条通过。`p` 独立新二进制空库 schema 10、七个匿名原生路由 401、重启事实不变。各批 Main 13 库／17 容器 ID 不变，探针和私有 PG 已停，restore20 未重开。

合成浏览器分别验证正常、只读和保存后读取 503；行内保存、取消／必填零写、电话校验、no-op、可选项清除、刷新仍在经营页及只重读恢复通过。首轮类型缺失夹具、tab 恢复错误均保留，修正后才记通过。不是正式 IAM／Gateway／PG／公网一次联合链；历史名称补全、经营参与人权限及 B01 全项未关闭。见[报告](./reports/oa-client-info-clone-repair-validation-20261002.md)和[验收入口](../acceptance/oa-initiation-repair/README.md)。

## 原 OA 基本信息与负责人原子保存（BF-087）

Project／Workbench 无容器 Go 测试与 vet、Node 24 项通过；私有 PG18 `j` 同批 104／66 条含子测试 PASS 验证历史、更换／清除、no-op、编号冲突回滚、并发、future 分工保护和离线重放。`k` 新二进制独立空库启动 schema 10、五个匿名路由 401、重启事实不变通过。三类分权编辑和全权普通项目／包使用合成浏览器，不能拼成正式用户全链。追加 HTTP 真库 `l` 的当前／历史断言错误保留；按契约修正后 `m` 66 条含子测试 PASS，覆盖真实 HTTP 原子提交、历史读取及 People 503／恢复／离线重放，见[报告](./reports/oa-basic-info-clone-repair-validation-20261002.md)。Main、正式 IAM、恢复和公网尚未更新／通过。

命令与独立证据入口见[验收说明](../acceptance/oa-initiation-repair/README.md)。浏览器 probe 可设置 `OA_BASIC_INFO_BROWSER_ROLE=profile-only|business-only|production-only`（分别执行，不是字符串整体），默认 detail 与原生一样只返回当前分工；`include_inactive=true` 才读取历史。前期夹具错误、原 HTTP invalid operation 500 修正为 400 均记录，不放宽鉴权和校验。

## 原 OA 项目列表与导航（BF-086）

候选 Directory v1 和统一导航取得 Project／Workbench 普通 Go 测试及 vet、Node 20 项／JS 语法证据。ECS 私有 PG18 `h` 同批 Project 99／HTTP 56 条 PASS 记录（含子测试），106 根跨页、父子 Scope、Tenant、当前 Assignment 与读零写入通过；`i` 实际二进制空库 schema 10、四个匿名路由 401、重启事实不变通过。合成浏览器实际测试年份／分页／搜索、子项刷新返回、列表状态和鼠标滚动；先前 `overflow:hidden` 失败保留，修正后精确返回位置才通过。

隔离浏览器入口加 `OA_DIRECTORY_BROWSER_PROBE=1`，串行读取，不启动本地 Docker。Main 未更新，正式 IAM／Suite／恢复／公网仍待验；P01 进度投影和完整 OA 未通过。命令、分层限制与证据见[验收说明](../acceptance/oa-initiation-repair/README.md)和[报告](./reports/oa-directory-clone-repair-validation-20261002.md)。

## 原 OA 已立项基本资料编辑（BF-085）

核心字段候选已取得 Project／Workbench 回归与 vet、Node 13 项及新 IAM 策略 parser／能力目录证据；ECS 私有 PG18 同批 Project 97／HTTP 53 条 PASS 记录（含子测试），独立空库启动／三个匿名 401／重启事实不变也通过。合成浏览器实际完成普通项目及无编号包的编辑、保存和刷新；修复透镜遮挡按钮后正常点击成功。首轮历史条数误算及两个 UI 遮挡失败均保留，不放宽正式校验。

新能力／Overlay／Policy 不会自动授予当前账号，Main 未部署本轮代码。P02 其余字段、真实 IAM、完整 Suite／恢复／公网仍待独立验收；完整 OA 仍为 `insufficient-evidence`。入口与实际证据见[验收说明](../acceptance/oa-initiation-repair/README.md)和[本轮报告](./reports/oa-profile-clone-repair-validation-20261002.md)。

## 原 OA 完整仿制首条立项修复（BF-084）

2026-10-02 已进入修复，而非只读审计。名称＋版本化经营负责人派发、无生产负责人完成、同一工作台 Project 详情、生产负责人后补及全视口抽屉已形成候选代码。Project／Workbench 普通 Go 测试、vet、Node 9 项及 JavaScript 语法通过；ECS 私有 PG18 的 Project／migration／HTTP 分组分别有 90／3／46 条含子测试 PASS 记录；新二进制独立空库 migration、IAM verifier 启动、匿名拒绝和重启事实不变通过。最初两次测试失败如实保留并修正夹具／旧 schema 预期，正式校验未放宽。

浏览器普通项目／包、后补、刷新及 720px 抽屉实测使用显式合成端口，不是正式 IAM／DB／公网验收。Main 未升级，13 库摘要／17 容器 ID 不变，探针停止、restore20 保持停止；完整 OA 仍未通过。入口见 [立项修复验收](../acceptance/oa-initiation-repair/README.md)，限制及证据见 [修复报告](./reports/oa-initiation-clone-repair-validation-20261002.md)。后续继续按原 OA 操作 ID 接通全站，不以此片段宣布完整仿制。

## 2026-10-02 原 OA 使用流程源码梳理（BF-083）

[操作流程基线](./specs/assembly/oa-original-usage-workflow-baseline.md)记录当前原 OA 路由、表单、保存／跳转、定向后端规则及文档冲突。原仓库提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 干净；本轮只读原代码、整理本文档，不运行原站／克隆业务测试、数据库、Docker 或主站部署。源码里的成功分支不能描述为本轮真人通过。

已确认偏差：派发时填写经营负责人而非先强制正式编号；生产负责人可后补；完成后原 OA 进入 `/projects/:id?tab=basic`，而 Workbench 真实入口跳独立 Project 页面、Project 当前完成规则要求双负责人。之前 2026-09-30／10-01 新建、API 复读、缺口拒绝及双负责人测试继续保留为当前实现的工程证据，不证明原 OA 流程等效。下一轮按基线操作 ID 补正常、后补、返回、分段失败／重试和权限浏览器证据；原站地址及测试身份未取得，不虚记通过。

## Auth58 Evidence 原生入口能力（BF-082）

`verify-entry-capabilities.py`／`entry-capability-probe.sh` 在独立 IAM、Evidence 库和 0.35 CPU／224 MiB 容器中运行真实签发与目标授权；`verify-workbench-entry-candidate.py` 在 network=none、0.15 CPU／96 MiB 镜像中验证当前 D1 清单兼容启动。三账号无需 view、metadata／download 分权、相邻 File Scope、撤权后交换拒绝和主站 13 库／17 ID 冻结通过，探针均停止。源码 Go／vet、Gateway race、D1 renderer、静态边界和发布包通过；当前 C8 离线 Cookie 矩阵用 `python3 acceptance/d1-full-oa-journey/test-c8-native-entry.py` 验证。主站尚未部署，完整 C8、公网和浏览器不是本次通过范围；详见[专项报告](./reports/auth58-evidence-native-entry-validation-20261001.md)。

## Auth58 ECS OSS 适配与恢复

运行入口：[`acceptance/evidence-oss-ecs`](../acceptance/evidence-oss-ecs/README.md)。2026-10-01 使用已部署镜像和正式 IAM 凭证，在独立库／限资源单容器中实际通过单对象与串行 multipart、STS 越权拒绝、内网 complete 摘要、幂等、签名下载和冷备份空库恢复。备份离机副本摘要一致；主栈 10 个 Local Version 未变、17/17 healthy。部署侧 [IMDS guard](../deployments/aliyun/ops/evidence-imds-guard) 实际 apply、重复 apply、四个非 Evidence 应用拒绝与 Evidence 正向复测通过。

该入口是云端适配验收，不是正式公网 Evidence 路由或真人浏览器验收；主 Provider 尚未切换，Local 兼容、凭据到期／刷新、guard 重启持久化、自定义 Files CNAME 和 OSS 字节误删恢复仍有门禁。详细结果见 [报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md)，完整 OA 仍为 `insufficient-evidence`。

## 参考 OA 界面 Overlay

2026-09-30 已把参考 OA 的登录入口、项目型导航、项目列表／详情和经营／生产／资料分区装入 Unified Workbench，并只替换公司 Ubuntu D1 的无状态 Workbench 容器。定向执行 `go test ./...`、`go vet ./...`、运行时 JavaScript 语法检查和浏览器交互检查均通过；部署后 `/health/ready` 为 `ready`。当前入口中的示例项目和部分未接写 API 的按钮仍是明确标识的界面验证数据，因此这一验证不替代连续 C8 和完整 OA 最终验收。完整范围、镜像摘要和下一步见 [参考 OA 界面 Overlay 验证记录](./reports/oa-reference-ui-overlay-validation-20260930.md)。

## OA 完整 Suite 连续 C0～C8

静态场景和发布包门禁：

```bash
acceptance/d1-full-oa-journey/verify-scenario
deployments/d1/ops/test-release-bundle
```

运行时按 C0～C8 顺序执行；详细参数和 checkpoint 语义见 [`acceptance/d1-full-oa-journey/README.md`](../acceptance/d1-full-oa-journey/README.md)。2026-09-30 在公司 Ubuntu 隔离 D1 栈实际通过 C0～C7，并完整执行 C8：C0 证明 12 个业务应用、17/17 健康服务、当前 schema、Nginx 唯一宿主端口；C1 固定同一 Tenant、Party、People、Project 和三项 Assignment；C2 延续这些 ID 建立供应商、两份 Evidence、Contract／Settlement、ready Business Engagement 与两个 Project Resource Link。

C2 还实际停止一次 Evidence Service，证明 503 时 Business Engagement 零写入，恢复后同一 Command 只提交一次；最终 Project 通过 request-bound workload credential 读取 BE material projection，再为两个 File 分别换取 Evidence metadata credential。

C3 从同一 state 继续：显式迁移四个 Money Party 引用并切换到 Party Directory，追加纠正发票、确认应收／回款／核销／Receipt Set，把三项稳定来源 ID 链入原 Project revision 7→10。无金额 Capability 的 Billing 读取返回 null 而非零；只停止 Billing 时仅 invoice segment 不可用，恢复后四段摘要全部可用。40 个 HTTP 断言零差异、数据库所有权增量精确，验收结束仍为 17/17 healthy。

C4 继续复用同一 Project revision 10 与 C1 生产负责人，新增 `design-v1`、`consultation-v1` 两个 Production Work Item 和两个 Evidence File／三个 Version。错误类型 Stage 返回 422，stale revision 返回 409；Evidence 停机时报告登记返回 503 且零写入，恢复后同一 Command 单次提交。咨询报告以 Version 2 替换后又重开／再完成；Project 与 People 增量保持零，结束仍为 17/17 healthy，完成态重跑保持相同 ID、revision 与所有权增量。

C5 复用 C1 业务负责人／参与者、C2 Contract／award Evidence 和 C4 Project。Evidence、Contract 各自停服时批复写入均 503 且 Results 零写入，恢复后同一 Command 只提交一次；审计精确冻结批复 revision 1。Completion Assessment GET 不写 Project，显式确认才把 Project revision 10→11；随后撤权返回 403，恢复 Grant 后追加批复 revision 2，Project 对已确认来源显示 drift。Results 停服时旧 confirmation 只从 Project 本地 receipt 回放，新 confirmation 503 且零写入。首轮 46 条、完成态重跑 14 条 HTTP 断言均为 0 mismatch，所有权增量精确且结束 17/17 healthy。

C6 继续复用原 Tenant／Project、C1 Actor、C2 供应商 Party／Contract 与既有失败建议。Cost 到 revision 2，新增 Engagement、Assessment、Payable Suggestion revision 3；Money 停机时来源保持 `target_unavailable`，恢复后同一命令只创建一个 Money Obligation。首轮 24 条、完成态重跑 15 条 HTTP 断言均为 0 mismatch，Project／Party／Contract／Evidence 零写入，结束 17/17 healthy。

C7 复用 C2 Settlement、C3 Receipt Set 和 C1 Person／Assignment，经营／生产奖池使用整数金额、基点和 `reference-oa-bonus-v1`。Bonus 只提出建议，Money 独立接受并拥有 Obligation；Money 停机可恢复，经营来源撤回不级联改写已确认义务。首轮 25 条、完成态重跑 16 条 HTTP 断言零差异，结束 17/17 healthy。详见 [连续 C7](./reports/oa-full-suite-continuous-c7-validation-20260930.md)。

C8 复用原 Project revision 12，顺序验证资料、个人工作、Money 指标和单来源故障。49 个 HTTP 断言零偏差，十二个业务数据库读取前后完整 canonical TSV 逐字节相同，结束 17/17 healthy。最终仍为 `insufficient-evidence`：C4～C6 输出未形成 active Project Link／资料投影；窄 `metadata.read + download` 在 Gateway 返回 401，只有加入遗留 `view` 才可获得 Evidence 短期地址。正式状态 SHA-256 为 `67c184d6384c7f38e54bf5750f78f00e00cb84e7ed0fbc2a48e8cf5245d88c4a`，证据目录为 `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c8`。详见 [连续 C8](./reports/oa-full-suite-continuous-c8-validation-20260930.md)。

## C8 Production 个人工作正式 D1

定向检查：

```bash
cd backend/services/design-operations
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../identity-access
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

jq empty ../../contracts/access/v1/examples/workload-identity-policy.json
sh -n ../../acceptance/d1-c8-production-work-items/verify-running
```

公司 D1 验收由 `acceptance/d1-c8-production-work-items/verify-running` 经正式 Nginx／Gateway／IAM 入口执行。2026-09-29 运行结果：7/7 步通过，`http-evidence.tsv` 47/47 符合预期；当前 Subject 恰好 1 条，相邻 Subject 为空；action 撤权后新凭证为空，read 撤权后新凭证 403；Production 停机时 Workbench 保持 200，来源独立失败并恢复；Production 事实前后 SHA-256 一致。`test.17` 正式升级前备份 `20260929T151745Z` 的 11 项 checksum 全部 OK，13/13 长期服务 healthy。详细步骤、失败与恢复见 [C8 Production 个人工作正式 D1 验证](./reports/oa-c8-production-personal-work-d1-validation-20260929.md)。

## 应用静态边界门禁

```bash
make architecture-boundary-static-audit
```

该入口先执行临时双应用负例自测，再审计当前 15 个应用。它以 `GOWORK=off go list -deps -test` 拒绝跨应用源码依赖，以 `go.mod` 检查拒绝指向其他应用的本地 replace，并拒绝 SQL 中的 `dblink`／FDW／Foreign Table／`CREATE SERVER`。2026-09-29 实际结果为 15 个应用、0 跨应用导入、0 跨应用 replace、0 跨数据库 SQL 机制。详见 [OA 应用静态边界审计](./reports/oa-static-application-boundary-audit-20260929.md)。

## 全仓快速检查

```bash
make check
```

该命令执行：

- Go 格式检查；
- `go vet`；
- 当前全部 Go 应用与验收模块的单元测试；
- File Transfer Core 的 TypeScript、单元测试、Evidence v1 契约守卫和声明构建；
- D1 宿主机引导脚本与私有镜像 `build`／`pull` 模式的离线门禁。

## 单应用检查

```bash
cd backend/services/contract-ledger
go test ./...
go vet ./...
```

```bash
cd backend/services/money-flow-ledger
go test ./...
go vet ./...
```

```bash
cd backend/services/people-groups
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

```bash
cd backend/services/evidence-service
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

这些命令必须在离开根目录 `go.work` 后也能运行，以证明应用没有隐藏的源码依赖。

## Party Directory 核心与独立交付恢复

机器可读契约门禁：

```bash
go test ./acceptance/access-contract/...
go vet ./acceptance/access-contract/...
jq empty contracts/access/v1/examples/party-directory-capabilities.json
ruby -e 'require "yaml"; ARGV.each { |p| YAML.safe_load(File.read(p), aliases: true); puts p }' \
  arckit/tech/_shared/models/Party.yaml \
  arckit/tech/_shared/models/PartyRevision.yaml \
  arckit/tech/_shared/contracts/party-*.yaml
```

根工作区检查和离开 `go.work` 后的独立模块检查：

```bash
go test ./backend/services/party-directory/...
go vet ./backend/services/party-directory/...
go build ./backend/services/party-directory/...

cd backend/services/party-directory
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

真实 PostgreSQL 检查使用独立测试数据库或测试 schema，禁止指向业务库：

```bash
TEST_DATABASE_URL='postgres://测试用户:测试密码@127.0.0.1:测试端口/测试数据库?sslmode=disable' \
go test -tags=integration -race ./backend/services/party-directory/internal/party
```

发布包、非 root 镜像与完整恢复演练：

```bash
backend/services/party-directory/ops/verify-delivery-recovery
# 或
make party-directory-delivery-recovery-acceptance
```

验收脚本只复用一个已经存在的用户定义 Docker bridge；可用 `PARTY_ACCEPTANCE_DOCKER_NETWORK` 显式指定。它在该网络上启动带 tmpfs 的隔离 PostgreSQL 18，不创建新网络、不读取既有应用数据库，结束时删除临时数据库容器、应用容器、验收镜像和工作目录。

2026-09-27 实际运行结果：以上 Go 检查全部通过。真库用例覆盖创建与精确重放、Command 载荷冲突、更正 revision 2、stale revision、无有效变化、停用 revision 3、停用主体拒绝新引用、历史倒序、默认 active 筛选、跨租户不可见、历史直接篡改拒绝、当前投影与最新 revision 不一致拒绝、失败写无命令／审计残留，以及 8 个并发相同创建只形成一个 Party 与 revision。HTTP 与 IAM 用例覆盖缺失凭证、错误 issuer／audience／tenant／action／scope、过期与未来凭证、伪造身份 Header、精确 Party Scope 和业务错误到 409／422／503 的映射。

还以独立进程和隔离 PostgreSQL schema 实际执行 `ready → create revision 1 → 同命令重放 → update revision 2 → stale 409 → inactive revision 3 → 搜索 → history 3/2/1`，随后关闭进程并删除测试 schema。第一轮 Docker Compose 单独建网曾被本机 Docker “预定义地址池耗尽”阻止，因此核心轮没有把它解释为容器证据。

随后独立交付恢复验收实际通过：构建并校验 `v0.1.0-acceptance.1` 发布归档和 SHA-256，检查三份带版本元数据的可执行文件、7 份 Party API 契约与 2 份模型；构建以 `postgres` 非 root 用户运行且自带同主版本客户端的镜像；在隔离 PostgreSQL 18 中写入 revision 1→3，停服生成含 manifest、逐表行数和 dump SHA-256 的完整备份，恢复到空数据库，确认向非空目标二次恢复失败，再校验 Party ID、revision 3 当前事实与 3 条历史保持，最后成功写入 revision 4。PostgreSQL 18 官方镜像的卷路径同时修正为 `/var/lib/postgresql`。

产品与技术验收口径分别见 [Party Directory 生命周期规格](../arckit/spec/party-directory/party-lifecycle.md)和 [Party Directory 第一版技术方案](../arckit/tech/party-directory/solution.md)。该段首先建立“核心运行时与独立交付恢复可行”；后续 D1 专项又补齐正式 API 入口、管理页面、Project 客户选择、精确 IAM 工作负载授权、Party 停机／恢复、无 Party Base 命令和联合空目标还原。Contract／Money 的只读预检、来源自有映射和含引用数据切换／只读回滚已有独立证据；Contract-linked C6 正式 D1 已复验。完整双租户矩阵与 Money 完整 D1 消费链仍需后续证据。

## Business Engagement 第一条后端闭环

单元、HTTP 权限与引用端口检查：

```bash
go test ./backend/services/business-engagement/... ./backend/platform/identity-access/internal/capability
go vet ./backend/services/business-engagement/...
```

PostgreSQL 18 真库检查使用独立测试数据库入口；测试会建立并销毁随机数据库，不得指向业务数据库：

```bash
TEST_DATABASE_URL='postgres://postgres:仅测试密码@127.0.0.1:测试端口/postgres?sslmode=disable' \
go test -tags=integration -race ./backend/services/business-engagement/...
```

2026-09-27 实际运行结果：两组命令均通过。真库用例覆盖创建、command 幂等／载荷冲突、expected revision、并发恰好一个成功、方式到材料要求、`ready`、材料替换／移除、取消和不可变历史；HTTP／IAM 用例覆盖无凭证、缺原生 action、Project B scope、错误 audience、遗留粗粒度 action 与伪造身份 Header 拒绝。该证据不包含客户页面、备份恢复、正式工作负载交换、Nginx→Gateway→IAM D1 或完整 OA 角色／撤权矩阵。

## Contract Ledger 本地启动

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
BOOTSTRAP_ADMIN_USERNAME=admin \
BOOTSTRAP_ADMIN_DISPLAY_NAME='本地管理员' \
BOOTSTRAP_ADMIN_PASSWORD='替换为至少12字节的首次启动密码' \
SESSION_COOKIE_SECURE=false \
go run ./cmd/server
```

bootstrap 配置只在空数据库首次启动时建立首位管理员。数据库已有用户后可以省略这三个变量，它们不会覆盖既有账号或密码。`SESSION_COOKIE_SECURE=false` 只适用于本地 HTTP；HTTPS 部署必须设为 `true`。

文件证据默认保存在应用工作目录下的 `.data/evidence`，单文件上限 20 MiB。部署时使用 `EVIDENCE_STORAGE_DIR` 指向 Contract Ledger 独占的持久目录，并把该目录与 PostgreSQL 作为同一备份／恢复集合；可用 `EVIDENCE_MAX_UPLOAD_BYTES` 设置正整数上限。

默认地址：`http://127.0.0.1:8081`

验证：

```bash
curl --fail http://127.0.0.1:8081/health/live
curl --fail http://127.0.0.1:8081/health/ready
```

除健康检查和 `/login` 外，其余页面都需要登录。首次登录后可在 `/admin/users` 创建 editor 或 viewer。

## Money Flow Ledger 本地启动

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
BOOTSTRAP_ADMIN_USERNAME=admin \
BOOTSTRAP_ADMIN_DISPLAY_NAME='本地资金管理员' \
BOOTSTRAP_ADMIN_PASSWORD='替换为至少12字节的首次启动密码' \
SESSION_COOKIE_SECURE=false \
go run ./cmd/server
```

bootstrap 配置只在空用户库首次启动时建立首位管理员。数据库已有用户后移除三个 bootstrap 变量再启动，不会覆盖原账号。`SESSION_COOKIE_SECURE=false` 仅限本地 HTTP；HTTPS 必须设置为 `true`。

默认地址：`http://127.0.0.1:8083`

验证：

```bash
curl --fail http://127.0.0.1:8083/health/live
curl --fail http://127.0.0.1:8083/health/ready
```

## People & Groups 本地启动

```bash
cd backend/services/people-groups
docker compose up -d --wait
IDENTITY_PROVIDER=template \
TEMPLATE_TENANT_ID='替换为测试租户 UUID' \
TEMPLATE_SUBJECT_ID='替换为测试 Subject UUID' \
TEMPLATE_DISPLAY_NAME='本地人员管理员' \
TEMPLATE_ACCESS_TOKEN='替换为至少32字节的随机访问令牌' \
SESSION_COOKIE_SECURE=false \
GOWORK=off go run ./cmd/server
```

默认地址：`http://127.0.0.1:8084`。模板模式仍然强制认证、HttpOnly Cookie、CSRF 和业务 Capability，不是关闭鉴权；正式组合必须使用 `IDENTITY_PROVIDER=iam`。

验证：

```bash
curl --fail http://127.0.0.1:8084/health/live
curl --fail http://127.0.0.1:8084/health/ready
```

## M0 独立性验证

1. 只启动 Contract Ledger 数据库和应用，确认两个健康端点成功；
2. 停止 Contract Ledger；
3. 只启动 Money Flow Ledger 数据库和应用，确认两个健康端点成功；
4. 从各应用目录复制源码到不包含根 workspace 的临时目录，分别执行 `go test ./...` 和 `go build ./cmd/server`；
5. 确认两个数据库不存在跨库对象或共享业务表。

后续每个业务功能都需要在这里追加实际验证入口。系统间出现业务流动时，必须增加连接器契约测试和端到端联调测试。

## M10-004 Contract Evidence 迁移只读预检

单元测试与构建：

```bash
cd backend/services/contract-ledger
GOWORK=off go test ./internal/evidencemigration ./cmd/ops
GOWORK=off go build ./...
```

对实际 Contract 数据执行只读预检：

```bash
cd backend/services/contract-ledger
DATABASE_URL='postgres://...' \
EVIDENCE_STORAGE_DIR='/absolute/path/to/legacy/evidence' \
GOWORK=off go run ./cmd/ops evidence-migration-preflight \
  --tenant-id '替换为目标 Evidence Tenant UUID' \
  > evidence-migration-preflight.json
```

该命令只读 Contract 表和旧对象文件；它不会创建目录、上传文件、修改数据库或删除旧对象。只有 JSON 中 `ready=true`，且 available、missing、migratable、字节数和每条确定性 File ID 映射均经人工复核后，才允许进入 M10-004 写入迁移阶段。

## CL-M1-001 主体登记与列表

单元测试：

```bash
cd backend/services/contract-ledger
go test ./internal/party ./internal/httpserver
```

真实 PostgreSQL 集成测试：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -tags=integration ./internal/party
```

手工验收入口：`http://127.0.0.1:8081/parties`

## CL-M1-002 合同草稿创建与列表

单元测试：

```bash
cd backend/services/contract-ledger
go test ./internal/contract ./internal/httpserver
```

真实 PostgreSQL 集成测试：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/contract ./internal/party
```

集成测试只删除自己按本次生成 ID 创建的记录，测试结束后不得在开发库留下测试主体或测试合同。

手工验收入口：`http://127.0.0.1:8081/contracts`

验收至少覆盖：

- 创建金额已确定和金额待确定的草稿；
- 相同 Command ID 与相同负载重放时不增加记录；
- 相同 Command ID 与不同负载时返回冲突；
- 不同 Command ID 使用大小写不同但等价的命名空间和编号时返回冲突；
- 列表显示的数据与 PostgreSQL 中的精确金额及空金额一致。

## CL-M1-003 合同草稿签约主体与角色

单元测试：

```bash
cd backend/services/contract-ledger
go test ./internal/contract ./internal/httpserver
```

真实 PostgreSQL 集成测试：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/contract ./internal/party
```

手工验收入口：从 `http://127.0.0.1:8081/contracts` 进入任一合同草稿详情。

验收至少覆盖：

- 同一主体添加两个不同角色；
- 两个主体添加相同角色；
- 重复有效关联返回冲突；
- 过期 Expected Draft Version 返回冲突且不产生变更；
- 添加和移除使用原 Command ID 重放时不重复执行；
- 同一 Command ID 改变负载或跨添加/移除动作复用时返回冲突；
- 移除后列表不再显示关联，但数据库仍保留 `removed_at` 历史；
- 重新添加已移除的“主体 + 角色”生成新的有效关联；
- 页面展示的版本、主体和角色与 PostgreSQL 一致。

## CL-M1-004 合同草稿编辑与取消

单元测试：

```bash
cd backend/services/contract-ledger
go test ./internal/contract ./internal/httpserver
```

真实 PostgreSQL 集成测试：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/contract ./internal/party
```

手工验收入口：从 `http://127.0.0.1:8081/contracts` 进入任一合同草稿详情。

验收至少覆盖：

- 编辑全部允许字段并核对精确金额；
- 原编辑命令重放时不重复增加版本，同 Command ID 改变负载时返回冲突；
- 无变化提交、重复编号和过期版本均不改变草稿；
- 编辑不改变 Contract ID、创建命令、创建时间和签约主体；
- 使用必填原因取消，原取消命令可安全重放；
- 同 Command ID 跨编辑与取消复用时返回冲突；
- 取消后详情显示原因、时间和原签约主体，但不再渲染写入表单；
- 取消后编辑、增删主体、再次取消均被拒绝；
- 数据库状态为 `cancelled`、`current_revision` 为空，且只有成功命令进入命令表。

## CL-M1-005 合同激活、签约主体快照与 revision 1

单元测试：

```bash
cd backend/services/contract-ledger
go test ./internal/config ./internal/contract ./internal/httpserver
```

真实 PostgreSQL 集成测试：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -count=1 -tags=integration ./internal/contract ./internal/party
```

手工验收入口：从 `http://127.0.0.1:8081/contracts` 进入满足条件的合同草稿详情。

验收至少覆盖：

- 缺少签署日期、生效日期或两个不同主体时不显示可执行激活表单，直接命令也被拒绝；
- 激活命令重放不重复生成 revision、快照或审计，同 Command ID 改变负载时冲突；
- 不同命令再次激活 active 合同时被拒绝；
- active 合同状态、`draft_version` 和 `current_revision` 与 revision 1 一致；
- revision 1 精确保存金额、币种、日期和来源草稿版本；
- Party 当前资料变化后，详情仍显示 revision 1 冻结名称与 Party revision；
- active 页面不渲染编辑、取消、激活、添加或移除主体表单；
- 数据库直接更新/删除不可变激活事实时返回 PostgreSQL `55000`；
- 在事务末端制造写入失败时不留下任何部分激活事实；
- Money Flow 数据库和应用停止时仍能完成激活。

## CL-M1-006 最小本地身份、权限与通用写入审计

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/config ./internal/identity ./internal/httpserver ./internal/party ./internal/contract
```

真实 PostgreSQL repository、迁移和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

集成测试为每组用例建立独立临时数据库并在结束后销毁，不向开发库写入测试用户或业务事实。

验收至少覆盖：

- 空数据库没有 bootstrap 凭据时拒绝初始化；提供合规凭据后只建立一个管理员和一条审计；
- 密码为 BCrypt 摘要，数据库不保存原始会话令牌；会话 Cookie 属性符合配置；
- 用户不存在、密码错误、账号停用或锁定使用统一登录错误；连续 5 次密码错误锁定 15 分钟；
- administrator、editor、viewer 的服务端权限和页面入口与固定权限表一致；
- 伪造请求 Actor 无效，业务写入只使用登录会话中的 `local_user` Actor；
- 七类业务写入和六类身份写入产生通用不可变审计，幂等重放不重复记录；
- 历史写入迁移为明确的 `legacy_unknown`，既有激活 Actor 保留；
- 审计冲突导致业务或身份事务整体回滚；
- 停用、修改密码和重置密码立即使旧会话失效；
- 当前管理员不能停用自己，也不能移除最后一名有效管理员；
- 从空数据库经 HTTP 登录、创建 editor/viewer、editor 写业务、viewer 被拒绝、viewer 查看审计的整条链路通过；
- Money Flow 数据库和应用停止时仍可完成全部测试。

## M1 完整回归与独立性

```bash
make fmt
make check

cd backend/services/contract-ledger
go test -race ./...
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
GOWORK=off go test ./...
GOWORK=off go build -o /dev/null ./cmd/server
```

M1 完成后仍不包含任何 Contract Ledger 与 Money Flow Ledger 之间的业务流动，因此不存在跨系统联调契约。出现首个跨系统业务动作时，必须先定义版本化 API、事件或快照契约，再增加幂等、失败恢复和端到端联调测试。

## CL-M2-001 生效补充协议与 revision 2+

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- M1 active 合同无损回填顺序 1 的原合同 Instrument，revision 1 与原 Actor 不被改写；
- adjustment、restatement、unchanged 三种金额语义精确形成 revision 2+；
- 金额未知时调整、负结果、零调整、超精度、过期 revision、非连续顺序、倒退生效日期和重复文书编号明确拒绝；
- 命令重放不重复写入，改变负载产生幂等冲突；两个命令并发消费同一 revision 时恰好一个成功；
- 新 revision 复制完整非金额条款和 Party Snapshot，旧事实保持不变；
- Instrument、change command、revision、Party Snapshot 和审计都受数据库不可变保护；
- 故意制造事务末端审计失败时不留下 Instrument、命令、revision、快照或 revision 推进；
- administrator/editor 可以登记，viewer 可读时间线但页面不渲染表单且直接 POST 返回 `403`；
- 从空数据库经 HTTP 完成用户、两个主体、合同、激活、补充协议和只读权限整条链路，页面、可信 Actor 审计与数据库事实一致；
- 本地开发库迁移后不存在 revision 来源缺口或 head revision 不一致；
- Money Flow 数据库和应用停止时仍能完成全部验收，不产生跨系统事实。

## CL-M2-002 生效终止文书

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- active 合同登记类型化终止文书后，原子形成下一 Instrument、下一 revision 和 `terminated` 状态；
- 终止 revision 完整复制条款、有效金额和 Party Snapshot，金额已确定与待确定两种情况均不产生结算金额；
- 草稿、已取消、已终止合同，过期 revision、非连续顺序、倒退生效日期和重复文书编号均明确拒绝；
- 相同命令可安全重放，改变负载产生幂等冲突；两个终止命令以及补充协议与终止命令并发时都恰好一个成功；
- 终止命令、Instrument、revision、快照和审计受数据库不可变保护；
- 故意制造事务末端审计失败时，`terminated` 状态、head revision 和所有新事实一起回滚；
- 终止后 editor 页面不再显示补充协议或终止表单，直接补充协议请求返回冲突；viewer 可读时间线但所有直接写入返回 `403`；
- 从空数据库经 HTTP 完成用户、两个主体、激活 revision 1、补充协议 revision 2、终止 revision 3 的完整链路；
- 页面、列表、数据库、可信 Actor 审计一致显示 `terminated`、revision 3 和保持为 `899.75 CNY` 的当前金额；
- 本地开发库迁移后不存在 revision 来源缺口、head revision 不一致或测试数据残留；
- Money Flow 数据库和应用停止时仍能完成全部验收，不产生结算、义务或其他跨系统事实。

## CL-M2-003 合同检索、状态筛选与稳定分页

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- 命名空间、合同编号和标题执行不区分大小写的字面量包含查询，`%`、`_`、反斜杠与单引号不获得 SQL 通配或注入语义；
- 状态、币种和组合条件精确筛选，输入统一去除首尾空白并标准化大小写；
- 超长关键词、未知状态、非法币种、损坏游标和混用筛选条件的游标返回 `422`，且不进入 repository；
- 固定按 `(created_at DESC, id DESC)` 排序，每页最多 50 条；真实 PostgreSQL 用 53 条记录验证两页无重复、无遗漏；
- 获取第一页后新增的更靠前合同不进入旧游标的第二页，刷新第一页后可以看到；
- active、terminated 和未来 closed 合同列表金额来自当前 revision，draft/cancelled 来自草稿；
- HTTP 页面保留筛选值，区分空系统与空结果，仅在存在后续数据时显示绑定原筛选条件的下一页 URL；
- administrator、editor 和 viewer 都可查询，viewer 仍看不到新建表单，匿名访问仍受登录保护；
- migration `000010_contract_search_indexes.sql` 可安全重放，开发库存在两个预期 B-tree 索引；
- 搜索集成测试清理自身记录，开发库无测试残留、revision 来源缺口或 head revision 错配；
- Money Flow 数据库和应用停止时仍能完成全部检索验收，不读取跨系统表。

## CL-M2-004 合同关系图

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- 六种内置类型分别建立不可变有向边，关系保存来源与目标登记时的 revision 外键、可选日期／说明和可信 Actor；
- 自关联、未知类型、非法日期、超长说明、draft/cancelled 合同和两端 revision 不一致明确拒绝；
- 同一 Command ID 相同负载重放不增加事实，改变负载产生幂等冲突；
- 不同命令并发建立同一条边时恰好一个成功，另一个返回重复关系冲突；
- 防环类型拒绝间接环；两个反向命令并发时恰好一个成功，`references` 允许双向边；
- Relation、command 和审计原子提交；故意复用审计 ID 时不留下 Relation 或 command；
- 直接更新／删除 Relation 和 command 均被 PostgreSQL `55000` 不可变触发器拒绝；
- 来源详情显示出向关系，目标详情显示入向关系，并从登记时 revision 读取编号与标题快照；
- 目标选择复用关键词、状态、币种与稳定分页，选定后明确展示方向和两端当前 revision；
- administrator/editor 可以建立关系，viewer 可读取双向关系但看不到入口且直接 POST 返回 `403`；
- 隔离空库 HTTP 联调从两份合同建立、主体关联和激活开始，完成目标检索、关系确认、双向查看、可信 Actor 审计和数据库事实核对；
- migration `000011_contract_relations.sql` 可安全重放；开发库两个关系索引和两个不可变触发器存在；
- 开发库关系测试合同、snapshot 缺口、command/relation 不一致、head revision 错配和 revision 来源缺口均为 0；
- Money Flow 数据库和应用停止时仍能完成全部验收，不创建结算、义务、资金或核销事实。

## CL-M2-005 阶段与最终结算事实

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- progress 与 final 均保存累计金额，当前投影读取最新 sequence，阶段金额之间不求和；
- 原始金额、当前有效金额和最新累计结算金额分别保存与展示，零金额、待确定有效金额以及高于／低于有效金额均精确处理；
- active 和 terminated 可登记；draft、cancelled、closed、final 后继续写入、过期 revision 与过期 sequence 明确拒绝；
- 至少明确选择一个当前 revision 确认主体，不存在或重复 position 拒绝；Party 当前资料和后续 revision 不改写历史快照；
- 同一 Command ID 相同负载重放不增加事实，改变任一字段产生幂等冲突；合同内结算编号不区分大小写唯一；
- 两个命令并发消费同一 sequence 时恰好一个成功；结算与补充／终止命令通过 Contract 行锁形成确定顺序；
- Settlement、command、确认主体和审计原子提交；审计失败不留下部分事实，四类事实均受不可变保护；
- administrator/editor 可登记，viewer 可读取但无表单且直接 POST 返回 `403`；Actor 只能来自登录会话；
- 空数据库 HTTP 流程完成激活、补充协议、阶段结算、终止和最终结算，页面、数据库、主体快照与审计一致；
- migration `000012_contract_settlements.sql` 可安全重放；开发库三个结算索引和三个不可变触发器存在；
- Money Flow 数据库和应用停止时仍能完成全部验收，不创建义务、资金、核销或跨系统事件。

## CL-M2-006 操作性关闭

单元与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party
```

验收至少覆盖：

- active 和 terminated 可关闭；draft、cancelled、closed 明确拒绝；
- 有 final Settlement 时必须引用最新 final，无 final 时必须显式确认 `not_required`，progress 历史可保留；
- 关闭日期不早于当前 Instrument 生效日期或最新 Settlement 确认日期；
- Expected Contract Revision 与 Expected Settlement Sequence 都会校验，关闭与结算并发时恰好一个成功；
- 相同命令可安全重放，改变负载产生幂等冲突；同一合同至多一个 Closure；
- Closure、command、状态更新和 `contract_closed` 审计原子提交，事务末端审计失败时整体回滚；
- Closure、command 和审计受数据库不可变保护，final Settlement 引用由复合外键约束；
- 关闭不推进 revision、不生成 Instrument、不修改合同金额或 Settlement；
- administrator/editor 可关闭，viewer 可读取但无表单且直接 POST 返回 `403`；
- 空数据库 HTTP 流程完成激活、补充协议、阶段结算、终止、最终结算和关闭，页面、数据库与可信 Actor 审计一致；
- closed 后普通变更、终止、结算和再次关闭入口消失且写入被拒绝，既有合同关系能力保持独立；
- migration `000013_contract_operational_closures.sql` 可安全重放，开发库存在两个不可变触发器和四个关键约束；
- Money Flow 数据库和应用停止时仍能完成全部验收，不创建义务、资金、核销或跨系统事件。

## CL-M2-007 合同文件证据

领域、文件适配器、配置与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/config ./internal/evidence ./internal/httpserver
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party ./internal/evidence
```

验收至少覆盖：

- draft、active、terminated、closed、cancelled 均可补充 Evidence，且状态、draft version、revision、合同金额与 settlement sequence 不变化；
- Contract、Instrument、Settlement 和 Closure 四种目标都验证存在性与同合同归属，任意或跨合同目标明确拒绝；
- Document 与连续 Version 不可覆盖；Expected Latest Version、幂等命令和并发竞争不会产生重复或跳号版本；
- 文件按实际字节计算大小、媒体类型和 SHA-256，空文件、超限文件、非法文件名与路径穿越输入不会留下引用或逃离存储根目录；
- 上传先原子落盘再提交数据库；数据库或审计失败清理本次候选文件，Document、Version、command 和审计都受不可变保护；
- 下载前重新校验大小和 SHA-256，缺失或损坏内容拒绝交付；成功下载强制 attachment、octet-stream、nosniff、no-store 并记录可信 Actor 审计；
- administrator/editor 可上传和追加版本，viewer 只读且可审计下载，匿名下载跳转登录；
- 隔离空库 HTTP 流程完成关闭事实挂接、Version 1、Version 2、编辑者与 viewer 下载，页面、数据库、原始字节和审计一致；
- migration `000014_contract_evidence.sql` 可安全重放，三张表、目标校验触发器、三类不可变触发器与关键约束存在；
- Money Flow 数据库和应用停止时仍可完成全部验收，不产生跨系统引用、事件或业务动作。

## CL-M2-008 已生效合同核心条款受控更正

领域、权限与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/contract ./internal/identity ./internal/httpserver ./internal/evidence
```

真实 PostgreSQL migration、repository 和空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party ./internal/evidence
```

验收至少覆盖：

- active、terminated、closed 由 administrator 追加 `correction_instrument` 和 revision N+1，生命周期状态保持不变；draft、cancelled 明确拒绝；
- 第一版只更正标题、通用分类、当前有效金额、合同签署日期和生效日期，编号、币种和全部 Party Snapshot 原样复制；
- 至少一个字段必须变化；文本、精确金额、日期、文书顺序和日期倒退均有明确校验；
- Expected Status 与 Expected Revision 同时参与冲突检测，更正与补充协议并发消费同一 revision 时恰好一个成功；
- 相同 Command ID 安全重放，改变意图产生幂等冲突，合同内文书编号仍不区分大小写唯一；
- Instrument、revision、快照、command、current revision 与审计原子提交；故意制造审计失败时整体回滚，历史事实继续受不可变触发器保护；
- 正式合同列表、检索与详情统一读取 current revision，旧 revision 仍可追溯；
- administrator 独占 `business.correct`，editor、viewer 和匿名直接 POST 均被服务端拒绝，Actor 只来自登录会话；
- 空库 HTTP 流程在关闭后更正 revision 3 为 revision 4，Closure、最终 Settlement 和 Relation 仍引用 revision 3；
- 更正文书自动成为 Evidence 目标，上传佐证不再次推进 revision；
- migration `000015_effective_contract_corrections.sql` 可安全重放，约束、索引和不可变触发器存在；
- Money Flow 数据库和应用停止时仍能完成全部验收，不产生跨系统业务动作。

## CL-M2-009 单合同可验证账簿导出

归档、摘要、权限与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/ledgerexport ./internal/identity ./internal/httpserver
```

真实 PostgreSQL 一致性读取与空库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party ./internal/evidence ./internal/ledgerexport
```

验收至少覆盖：

- `draft`、`cancelled`、`active`、`terminated` 和 `closed` 均可导出，cancelled 归档包含取消原因、版本迁移、可信 Actor 和取消时间；
- ZIP 根目录只含 `manifest.json` 与 `ledger.json`，manifest 中长度和 SHA-256 与 ledger 原始字节精确一致，空集合写成 `[]`；
- closed 更正场景导出全部四份 Instrument、四个 Revision 及每版 Party Snapshot，并保留 Closure、final Settlement 和 Relation 对旧 revision 3 的引用；
- source 与 target 两侧查询都能导出同一不可变 Relation，并保留登记时两端 Revision 快照；
- Evidence 导出 Document／Version 文件名、媒体类型、大小与 SHA-256，但不包含文件字节、`storage_key`、存储 provider 或服务器路径；
- 本地用户密码、会话与其他身份凭证不进入归档；合同状态、draft version、current revision 和业务事实不因导出改变；
- PostgreSQL 使用单个 `REPEATABLE READ, READ ONLY` 事务归集跨表事实，数组按明确业务顺序稳定输出；
- `business.export` 仅 administrator 拥有，editor/viewer 直接 GET 返回 `403`，匿名请求进入登录流程；
- 成功响应使用 attachment ZIP、no-store 与 nosniff；审计失败时不发送归档；
- 第一次导出不包含自身的 `contract_ledger_exported`，第二次导出包含上一次访问审计，且每次审计 Actor 来自管理员会话；
- 隔离空库 HTTP 流程离线解包、重算摘要并核对复杂 closed、draft、cancelled 和关系目标合同的归档内容；
- Money Flow 数据库和应用停止时仍能完成全部验收，不产生跨系统引用、事件或业务动作。

## CL-M2-010 空业务库单合同账簿恢复

归档预检、权限与 HTTP 边界测试：

```bash
cd backend/services/contract-ledger
go test -count=1 ./internal/ledgerrestore ./internal/evidence ./internal/identity ./internal/httpserver
```

真实 PostgreSQL 原子恢复、往返与空业务库 HTTP 纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/ledgerrestore ./internal/httpserver ./internal/evidence ./internal/ledgerexport
```

手工入口：`http://127.0.0.1:8081/admin/restore`，只对 administrator 显示。

验收至少覆盖：

- 只接受格式 `omsbizfoundry.contract-ledger/contract-export-v1`、版本 1、根目录恰好两个普通文件的 ZIP；路径穿越、重复／额外文件、加密、大小超限、未知 JSON 字段和摘要不一致在写库前拒绝；
- 生命周期、连续 Instrument／Revision、金额转换、Party Snapshot、Settlement、Closure、Evidence 目标或必要来源审计不一致时返回 `422` 且目标业务事实保持为空；
- 任意 Relation 整包拒绝，不创建占位合同、不放松外键、不丢弃关系；
- draft、cancelled 及 active／terminated／closed 的正式历史形态可恢复；复杂 closed 往返包含补充、两次结算、终止、关闭、更正和 Evidence；
- 目标 Contract 或 Party 非空时返回 `409`；本地用户、会话和身份审计不阻止恢复，也不被归档覆盖；
- 相同 Restore Command ID 与相同 ledger SHA-256 重放不新增事实或恢复审计；同一 Command ID 改变归档或 Actor 明确冲突；
- 故意在事务末端制造源审计主键冲突时，Contract、Party、子事实、源审计和 restore operation 全部回滚；
- 源审计 Actor 原样保留，当前管理员只进入唯一 `contract_ledger_restored` 审计；恢复事实继续受不可变触发器保护；
- Evidence 元数据和 SHA-256 保留，但旧 Version 标为 `missing`、没有 storage key／下载链接；直接下载不写成功审计，后续追加本地新 Version 正常；
- administrator 页面要求 CSRF、Restore Command ID、ZIP 和显式空库确认；editor/viewer 返回 `403`，匿名请求进入登录边界；
- 只有本地管理员的空业务库经 HTTP 完成上传、恢复、查看和重新导出；目标再导出允许新增恢复审计及 Evidence 可用状态变化；
- Money Flow 数据库和应用停止，不创建跨系统引用、事件、连接器调用或业务动作。

## CL-M2-011 可校验全量备份与空环境恢复

归档格式、安全边界与失败清理测试：

```bash
cd backend/services/contract-ledger
go test -race -count=1 ./internal/systembackup ./cmd/backup
```

真实 PostgreSQL snapshot、真实 Evidence 和备份 → 恢复纵向联调：

```bash
cd backend/services/contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./internal/systembackup
```

集成测试使用临时源／目标数据库和临时 Evidence 目录，并调用 `ops/local-pg-dump`、`ops/local-pg-restore` 中的 Compose wrapper；结束后销毁测试数据库，不修改开发库业务事实。

生产和本地手工命令见 [Contract Ledger 全量备份与恢复运行手册](./operations/contract-ledger-backup-restore.md)。

验收至少覆盖：

- `pg_dump` 与 schema／计数／available Evidence 引用共享同一个 PostgreSQL 导出 snapshot；客户端 major 必须与服务器一致；
- 归档格式固定为 `omsbizfoundry.contract-ledger/full-backup-v1`，只包含 custom dump、严格 manifest 和实际引用的 Evidence；
- Evidence 缺失、不是普通文件、大小或 SHA-256 不符时不发布最终 ZIP；同名输出不覆盖，成功文件权限为 `0600`；
- ZIP 路径穿越、重复／未知 entry、目录、软链接、加密、非 store method、未知 manifest 字段和摘要错误在恢复目标变更前拒绝；
- 恢复只接受零用户 relation 的数据库和不存在的 Evidence 根目录，不提供清空、合并或 `--force`；
- dump 排除 `local_sessions` 数据；恢复后密码身份仍可登录，但备份前会话数归零；
- 真实 Evidence 字节在恢复后与数据库、manifest 的大小／SHA-256 三方一致，`missing` Version 不伪造文件；
- 无效 dump 令真实 `pg_restore` 失败，目标数据库保持空，工具只回收带本次随机标记的 Evidence 根目录；
- 恢复结果可重放 migration，独立启动后 live／ready 通过；
- Money Flow 停止且整个工具不读取、写入或调用 Money Flow。

## CL-M2-012 独立安装与升级交付

schema 状态、显式 migration、构建身份与运行前门禁测试：

```bash
cd backend/services/contract-ledger
go test -race -count=1 ./internal/buildinfo ./internal/config ./internal/database ./internal/evidence
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./internal/database
```

构建独立 release bundle：

```bash
RELEASE_PROVENANCE_MODE=development \
RELEASE_COMMIT='contract-test' \
RELEASE_BUILT_AT='2026-09-19T12:30:00Z' \
make contract-ledger-release \
  VERSION='v0.1.0-dev.2' \
  OUTPUT_DIR='/绝对路径/release-output'
```

全新安装和升级的生产步骤见 [Contract Ledger 独立安装与升级运行手册](./operations/contract-ledger-install-upgrade.md)。

验收至少覆盖：

- build 脚本以 `CGO_ENABLED=0`、`GOWORK=off` 和 `-trimpath` 构建 server／ops／backup，三者输出相同 version、commit、build time 和目标平台；
- `.tar.gz` 只含三二进制、示例配置和两份运行手册，并带可校验 SHA-256；无效版本和同名输出都明确拒绝；
- schema 检查区分 empty、pending、current 和 incompatible；未知 migration、版本缺口和非托管已有 relation 不会被自动修复；
- `ops migrate` 以 advisory transaction lock 串行并发迁移，并在同一事务执行全部 pending 文件；故意令 `000016` 末段失败时，前段 DDL 和 migration 记录都不残留；
- 历史合法 schema 显示 pending，显式升级后 current；既有合同、身份、审计和 Evidence 元数据继续有效；
- `MIGRATION_MODE=verify` 面对 empty schema 时在监听前退出，面对 current schema 才启动；Evidence 不可写和 PostgreSQL major 不匹配也在监听前失败；
- 实际 `v0.1.0-dev.2` bundle 在全新数据库完成 `check → migrate → require-current → server`，通过 live、ready、登录、建合同和 Evidence 上传下载；下载 SHA-256 与 bundle 内原文一致；
- 移除 bootstrap 环境变量后重启同一 bundle，原账号仍可登录、合同仍可见且用户数保持为 1；
- Money Flow 应用停止且没有参与数据库、文件、调用、交付或测试流程。

## M2 完整回归与独立性

```bash
make fmt
make check

cd backend/services/contract-ledger
go test -race ./...
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/contract ./internal/party ./internal/evidence ./internal/ledgerexport ./internal/ledgerrestore ./internal/systembackup
GOWORK=off go test ./...
GOWORK=off go build -o /dev/null ./cmd/server
GOWORK=off go build -o /dev/null ./cmd/ops
GOWORK=off go build -o /dev/null ./cmd/backup
```

`CL-M2-001` 至 `CL-M2-012` 只处理 Contract Ledger 内部的合同文书、revision、生命周期、只读检索、合同关系、结算、操作性关闭、本地文件证据、类型化受控更正、单合同账簿导入导出、本系统全量备份恢复与独立交付，不包含 Contract → Money Flow 业务动作，因此没有伪造跨系统联调。M2 退出门槛已经满足；首次出现真实系统间业务流动时才增加对应契约、幂等、失败恢复和联调矩阵。

## MF-M3-001 首位管理员与本地登录会话

领域、配置和 HTTP 单元／竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/config ./internal/identity ./internal/httpserver
```

历史 migration、真实身份仓储和登录／注销纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver
```

集成测试为每个用例创建带随机后缀的临时数据库，并在结束后精确关闭连接和删除目标；测试后 `money_flow_test_%` 数据库应为零。

验收至少覆盖：

- 空用户库缺少 bootstrap 配置时在监听前失败，但 migration 可安全完成；提供合规配置后只形成一名 administrator 和一条 bootstrap 审计；
- username、display name 和密码边界明确，密码只保存 BCrypt；重复或并发 bootstrap 不覆盖既有账号，恰好一个请求创建首位用户；
- session 原始令牌不落库，数据库只保存 32 字节 SHA-256；撤销、过期、状态或 generation 不匹配时不能解析；
- 正确凭据建立会话，未知／错误／锁定账号返回同一错误，连续 5 次错误后锁定 15 分钟；
- `/health/live`、`/health/ready` 和登录入口公开，首页受保护；`next` 只接受本地路径，防止开放重定向；
- 登录、首页与注销完整验证 CSRF、HttpOnly、SameSite、Secure 配置和服务端 session 撤销；
- bootstrap 用户与审计同事务写入，故意令审计失败时两者均为零；审计 UPDATE／DELETE 被 PostgreSQL `55000` 拒绝；
- 开发库真实启动完成登录和注销，移除 bootstrap 配置重启后原账号仍可登录、用户和审计仍各一条；
- 停止 Contract Ledger 进程后，Money Flow 仍通过 ready、登录和首页验证；两个系统不共享数据库、Cookie 或会话；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

当前不存在 Contract → Money Flow 业务动作，因此本功能验证的是运行与身份独立性，不伪造连接器联调。未来任何真实业务流动必须新增版本化契约、幂等、失败恢复和端到端联调矩阵。

## MF-M3-002 轻量主体登记与列表

领域、HTTP 与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/party ./internal/httpserver
```

历史 migration、真实主体仓储和登录／登记纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party
```

手工验收入口：`http://127.0.0.1:8083/parties`

验收至少覆盖：

- administrator 可登记 organization 与 individual，并从 PostgreSQL 稳定排序列表看到 revision 1；
- 非法类型、空名称、过长字段和非法 Command ID 返回字段级错误，不写数据库；
- 输入标准化先于 SHA-256 幂等负载计算，空登记标识保存为 NULL 并读取为空字符串；
- 同 Command ID、同标准化负载串行或并发重放只形成一个 Party、一个 Command 和一条审计；
- 同 Command ID 改变事实或可信 Actor 返回幂等冲突；
- Party、Command 和审计原子提交，故意令审计失败时全部回滚；Command 和审计 UPDATE／DELETE 被 PostgreSQL `55000` 拒绝；
- GET／POST `/parties` 均要求登录 administrator，POST 要求 CSRF；请求 Actor 字段不能覆盖登录 Principal；
- migration 从历史 `000001` 升至 `000003` 后重复执行不增加版本，预期表、索引、约束和触发器存在；
- 停止 Contract Ledger 后，Money Flow 仍能完成 ready、登录、创建、重放、冲突与列表；本功能没有跨系统业务流动，因此只验证运行和数据独立性，不伪造连接器联调；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-003 人工应收／应付义务草稿

领域、HTTP 与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/httpserver
```

历史 migration、真实义务仓储和登录／主体／草稿纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party ./internal/obligation
```

手工验收入口：`http://127.0.0.1:8083/obligations`

验收至少覆盖：

- administrator 选择两个不同本地 Party，分别建立 receivable 与 payable 草稿；方向正确决定本方是 creditor 或 debtor；
- 创建时冻结双方 Party ID、revision、类型、名称和登记标识；更新 Party 当前资料后详情仍显示原快照；
- 标题、Party ID、方向、严格正数 18+6 精确金额、三字母币种、LocalDate 到期日和人工来源依据的全部边界明确拒绝或标准化；
- 金额不经过浮点数，`000100.500000` 读取为 `100.5`，`0.000001` 可用；
- 同 Command ID 与同标准化负载串行或并发重放只形成一个 Obligation、Command 和审计；改变负载或可信 Actor 返回冲突；
- Party 不存在或审计失败时三类事实全部回滚；Command 和审计 UPDATE／DELETE 被 PostgreSQL `55000` 拒绝；
- GET／POST `/obligations` 与 GET `/obligations/{id}` 要求 administrator，POST 要求 CSRF；请求 Actor 字段不能覆盖登录 Principal；
- migration 从历史 `000003` 升至 `000004` 并可重放，表、索引、外键、约束和触发器存在；
- 隔离空库 HTTP 流程完成 bootstrap、两个 Party、义务草稿创建／重放／冲突、列表、详情和注销，页面、数据库、快照与可信 Actor 一致；
- 停止 Contract Ledger 后，Money Flow 仍能完成 ready、登录、第二主体、草稿创建、重放、冲突、列表与详情；当前没有跨系统业务流动，因此只验证运行和数据独立性，不伪造连接器联调；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-004 义务确认与 revision 1

领域、HTTP 与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/httpserver
```

历史 migration、真实确认仓储和登录／草稿／确认纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party ./internal/obligation
```

手工验收入口：`http://127.0.0.1:8083/obligations`

验收至少覆盖：

- administrator 只可从 draft 详情确认，命令显式消费 Expected Draft Version；确认后状态为 `open`、current revision 为 1，表单消失并显示 revision 历史；
- revision 1 完整复制草稿方向、标题、账簿所有者、双方 Party 快照、精确金额、币种、到期日和人工来源；确认前 Party 当前资料变化不影响 revision；
- 正式义务当前投影来自 current revision；根表草稿列不成为第二真相源；
- 相同 Command ID 和相同标准化负载串行或并发重放只形成一个 revision、一个确认命令和一条审计；改变 Expected Draft Version 或可信 Actor 返回幂等冲突；
- 不同 Command ID 并发确认同一草稿恰好一个成功，另一个返回状态冲突；缺失义务、版本冲突和非 draft 状态均不留下部分事实；
- revision、确认命令、状态推进和审计原子提交；故意令审计失败时全部回滚，revision、命令和审计 UPDATE／DELETE 被 PostgreSQL `55000` 拒绝；
- POST `/obligations/{id}/confirm` 要求 administrator 和 CSRF；路由提供 Obligation ID，Actor 只来自登录 Principal，请求伪造字段无效；
- migration 从历史 `000004` 升至 `000005` 并可重放，既有 draft 保持可确认，表、复合外键、状态约束和不可变触发器存在；
- 隔离空库 HTTP 流程完成 bootstrap、两个 Party、草稿、确认、确认重放、open 列表和 revision 详情，页面、数据库、快照与可信 Actor 一致；
- 停止 Contract Ledger 后，Money Flow 仍能完成 ready、登录、确认、重放、列表与详情；当前没有跨系统业务流动，因此只验证运行和数据独立性，不伪造连接器联调；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-005 人工实际收款／付款登记

领域、HTTP 与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/cashtransaction ./internal/httpserver
```

历史 migration、真实资金仓储和登录／主体／义务／资金纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party \
  ./internal/obligation ./internal/cashtransaction
```

手工验收入口：`http://127.0.0.1:8083/cash-transactions`

验收至少覆盖：

- administrator 可分别登记 receipt／payment；receipt 的本方是 payee，payment 的本方是 payer，双方必须是两个存在的 Money Flow Party；
- 列表与详情显示不可变 `recorded` 事实、双方快照、精确金额、币种、发生日、账户标签、人工来源和可选外部流水号；Party 当前资料变化不改写快照；
- 金额不经过浮点数，严格大于零且最多 18 位整数、6 位小数；类型、币种、日期、长度和 Party 组合错误明确拒绝且不留数据；
- 相同 Command ID 与相同标准化负载串行或并发重放只形成一条资金、一条创建命令和一条审计；改变负载或可信 Actor 返回幂等冲突；
- 不同 Command ID 使用同范围、大小写等价的非空外部流水号时，串行或并发只有一个成功；不同本方主体或账户标签可以复用；空外部流水只按 Command ID 幂等；
- Party 不存在或故意令审计失败时整体回滚；Cash Transaction、创建命令和审计不可变；
- GET／POST `/cash-transactions` 与详情要求 administrator，POST 要求 CSRF；Actor 只来自登录 Principal，请求伪造字段无效；数据库错误不伪装为成功、空列表或 404；
- migration 从历史 `000005` 升至 `000006` 并可重放，既有 Party 与义务保持不变，表、索引、约束、外键和不可变触发器存在；
- 隔离空库 HTTP 流程完成 bootstrap、两个 Party、义务建立与确认、资金登记、原命令重放、负载冲突、外部流水冲突、列表和详情，页面、数据库、快照与可信 Actor 一致；
- 停止 Contract Ledger 后，Money Flow 仍能完成 migration、ready、登录、登记、重放、冲突、列表与详情；当前没有跨系统业务流动，因此只验证运行和数据独立性，不伪造连接器联调；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-006 单笔资金义务分配与余额不变量

领域、HTTP 与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/allocation ./internal/obligation \
  ./internal/cashtransaction ./internal/httpserver
```

历史 migration、真实 Allocation 仓储和完整 Money Flow 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party \
  ./internal/obligation ./internal/cashtransaction ./internal/allocation
```

手工验收入口：`http://127.0.0.1:8083/allocations`

验收至少覆盖：

- administrator 每次只选择一笔仍有未分配额的实际资金和一条仍有剩余额的 open 义务，并显式提交 Expected Obligation Revision；
- receivable／receipt、payable／payment 的方向相符，本方主体、payer／payee Party ID 和币种完全一致；第三方代收付及跨币种明确拒绝；
- 分配金额精确且严格大于 0，同时不超过资金未分配额与义务剩余额；等额分配可用，超过任一侧均冲突且不留数据；
- Cash Transaction 与 Obligation 列表／详情的 allocated、unallocated／outstanding 从 Allocation 求和，原金额、revision 和 open 状态不被改写；
- 相同 Command ID 和相同标准化负载串行或并发重放只形成一条 Allocation、一条命令和一条审计；改变负载或可信 Actor 返回幂等冲突；
- 不同 Command ID 并发争用同一资金、同一义务或同一对事实时，成功金额总和不超过任何一侧；失败命令不留 Command 或审计；
- draft／非 open、Expected Revision 过期、事实缺失、方向／主体／币种错配和故意审计失败全部整体回滚；直接 SQL 也不能绕过插入校验；
- Allocation、创建命令和审计不可更新或删除；客户端 Actor、币种、余额和生效时间字段全部无效；
- migration 从历史 `000006` 升至 `000007` 并可重放，既有事实保持不变，表、索引、外键、校验函数和不可变触发器存在；
- 隔离空库 HTTP 流程完成 bootstrap、两个 Party、义务创建／确认、资金登记、部分分配、重放、冲突、列表、详情和两端余额，页面、数据库与可信 Actor 一致；
- 停止 Contract Ledger 后，Money Flow 仍能完成 migration、ready、登录、分配、重放、超额拒绝和余额查询；当前没有跨系统业务流动，因此不伪造连接器联调；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL `-race` 集成，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-007 多条单笔 Allocation 组合闭环

领域、详情下钻与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/allocation ./internal/httpserver \
  ./internal/obligation ./internal/cashtransaction
```

真实 PostgreSQL 组合仓储和完整 Money Flow HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/identity ./internal/httpserver ./internal/party \
  ./internal/obligation ./internal/cashtransaction ./internal/allocation
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowHTTPMultiAllocationCompositionAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：分别从 `http://127.0.0.1:8083/cash-transactions` 和 `http://127.0.0.1:8083/obligations` 进入父事实详情。

验收至少覆盖：

- 使用三个独立单笔命令形成资金 A `120` → 义务 X `40`、资金 A → 义务 Y `80`、资金 B `80` → 义务 X `60`，不提交批量负载；
- 完成后资金 A allocated／unallocated 为 `120/0`、资金 B 为 `60/20`、义务 X 为 `100/0`、义务 Y 为 `80/0`；三侧守恒总额均为 `180`，两条义务仍为 `open`；
- 资金 A 详情只显示 X／Y 两条 Allocation，资金 B 只显示 X；义务 X 只显示 A／B，义务 Y 只显示 A，无关记录不能泄漏；
- 每条历史包含 Allocation、另一端事实、义务 revision、精确金额、人工依据、可信 Actor 与生效时间，并可双向进入 Allocation 和另一端详情；
- 同一父事实的历史固定按 `(applied_at DESC, allocation_id DESC)` 排序；同一时间也有确定顺序；
- 无效父 UUID 在 repository 前拒绝；父事实不存在返回 404，父读取或历史读取失败返回 500，不能渲染为“尚未形成任何分配”；
- 读取历史不写审计或业务事实；schema 仍为 `000007_single_allocations.sql`，已有父 ID／时间索引被复用；
- MF-M3-006 的幂等、兼容性、双侧超额、并发锁序、直接 SQL 保护和不可变约束全部继续通过；
- Contract Ledger 停止时，隔离空库真实 HTTP 流程仍完成 bootstrap／登录、三次单笔分配、四个父详情下钻、三份 Allocation 详情和数据库守恒核对；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量 `-race`、Money Flow 真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M3-008 人工内部账户转账

领域、HTTP 与仓储单元测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/internaltransfer ./internal/httpserver
```

真实 PostgreSQL migration、仓储并发和完整 Money Flow HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowHTTPInternalTransferIsNonOperatingAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：`http://127.0.0.1:8083/internal-transfers`。

验收至少覆盖：

- administrator 以可信会话 Actor 登记、重放、列表和查看内部转账；伪造 Actor、状态、分类和时间无效；
- 金额使用精确十进制，来源／目标账户必须不同，Party 快照不被当前资料变化改写；
- 相同 Command ID 并发重放只形成一份事实；改变负载／Actor 冲突；同范围大小写等价外部参考串行或并发只允许一个成功；
- 审计失败、Party 缺失和数据库约束失败不留下半成品；Transfer、Command 和审计不可更新或删除；
- migration 从历史 `000007` 升至 `000008` 并可重放，既有 Party、Obligation、Cash Transaction 和 Allocation 保持不变；
- 已有经营 receipt／payment 后登记 Internal Transfer，不改变 Cash Transaction 数量、经营收付金额、Allocation 和未分配资金；
- schema 不存在 Allocation 引用 Internal Transfer 的外键，分配页面不把 Internal Transfer 列为候选资金；
- Contract Ledger 停止时，隔离空库流程仍完成 migration、ready、登录、内部转账、重放、冲突、列表、详情和数据库经营隔离核对；
- `make check`、两个应用全量 `-race`、Money Flow 全量真实 PostgreSQL 集成 `-race`，以及两个应用 `GOWORK=off` 独立测试通过。

## MF-M4-001 Allocation 完整反向核销

领域、HTTP、余额投影与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/allocation ./internal/httpserver \
  ./internal/obligation ./internal/cashtransaction
```

真实 PostgreSQL migration、仓储并发和完整 Money Flow HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/allocations` 进入一条 active Allocation 详情。

验收至少覆盖：

- administrator 只提交原因即可完整反向 active Allocation；Actor 来自登录会话，金额、币种、资金、义务、revision 和时间都从原事实或服务端复制；
- 原 Allocation 永久保留，详情、全局列表、资金历史和义务历史显示 `reversed`、原因、可信 Actor 与时间；已反向详情不再提供反向入口；
- 资金 allocated／unallocated 与义务 allocated／outstanding 按 Allocation 减 Reversal 的精确净额恢复，释放容量可以形成新的正确 Allocation；
- 相同 Command ID／负载串行或并发重放只形成一份 Reversal、Command 和审计；改变负载／Actor 冲突，不同 Command 并发反向恰好一个成功；
- Reverse 与 Apply 使用同一资金／义务锁序；并发后的最终净额不超过任一侧，数据库直接插入不能伪造复制事实或绕过净额校验；
- 空／过长原因、非法／缺失 Allocation、重复反向、CSRF、非 administrator、审计失败和数据库失败不留下半成品；Reversal、Command、Allocation 和审计不可更新或删除；
- migration 从历史 `000008` 升至 `000009` 并可重放，既有事实和余额保持不变；
- 隔离空库 HTTP 流程完成 bootstrap、Party、Obligation、Cash Transaction、Allocation、Reversal、重放／冲突、余额恢复和重新分配；
- Contract Ledger 停止时上述流程与 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M4-002 Cash Transaction 完整冲销

领域、HTTP、Allocation 保护与竞态测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/cashtransaction ./internal/allocation \
  ./internal/httpserver ./internal/obligation
```

真实 PostgreSQL migration、跨模块仓储并发和完整 Money Flow HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

关键锁序可重复压力检查：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -count=20 -tags=integration \
  -run '^(TestPostgresCashTransactionReversalAndAllocationApplySerialize|TestPostgresCashTransactionAndAllocationReversalsSerialize|TestPostgresDirectCashReversalAndAllocationInsertSerialize)$' \
  ./internal/cashtransaction
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/cash-transactions` 进入一条 recorded Cash Transaction 详情。

验收至少覆盖：

- administrator 只提交原因即可完整冲销净 allocated amount 为 0 的资金；Actor 来自登录会话，方向、金额、币种和时间从原事实或服务端复制；
- 原 Cash Transaction、外部流水唯一占用、主体快照和历史 Allocation 永久保留；详情与列表显示原始金额、current effective amount、`reversed`、原因、可信 Actor 与时间；
- active Allocation 阻止 Cash Reversal 且不留 Reversal／Command／审计；逐条追加 Allocation Reversal 后才允许冲销；
- 冲销后 current effective／allocated／unallocated 均为 0，Allocation 页面不再列为候选，构造请求和直接 SQL 都不能重新分配；
- 相同 Command ID／负载串行或并发重放只形成一份 Reversal、Command 和审计；改变负载／Actor 冲突，不同 Command 并发冲销恰好一个成功；
- Cash Reverse／Allocation Apply、Cash Reverse／Allocation Reverse 和直接 SQL Reversal／Allocation Insert 共用 Cash 行锁，结果只能被解释为确定顺序，不出现 reversed 且净分配非零；
- Apply 取得 Cash 行锁后用新语句读取 Reversal 状态，避免等待行锁前的语句快照过期；应用冲突稳定映射为 typed 409，数据库触发器继续保护直接写入；
- 空／过长原因、非法／缺失 Cash Transaction、重复冲销、CSRF、非 administrator、审计失败和数据库失败不留下半成品；Cash Transaction、Reversal、Command 和审计不可更新或删除；
- migration 从历史 `000009` 升至 `000010` 并可重放，既有 Cash、Allocation 与 Allocation Reversal 保持不变；
- 隔离空库 HTTP 流程完成 bootstrap、Party、Obligation、Cash Transaction、Allocation、Allocation Reversal、Cash Reversal、投影核对和冲销后分配拒绝；
- Contract Ledger 停止时上述流程与 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M4-003 Obligation 金额调整

领域、HTTP、余额投影与 Allocation 兼容性测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/allocation ./internal/httpserver
```

真实 PostgreSQL migration、跨模块仓储并发和完整 Money Flow HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

关键锁序和数据库直写保护的可重复压力检查：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -count=20 -tags=integration \
  -run '^(TestPostgresObligationAmountAdjustmentAndAllocationApplySerialize|TestPostgresObligationAmountAdjustmentAndAllocationReversalSerialize|TestPostgresDirectObligationAmountAdjustmentWaitsForAllocationInsert)$' \
  ./internal/obligation
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/obligations` 进入一条 open Obligation 详情。

验收至少覆盖：

- administrator 以正金额和必填原因增加或减少当前有效金额；Expected Current Revision 来自页面，Actor 来自登录会话，客户端伪造前值、结果值、币种、Actor 或时间无效；
- 每次成功调整从 current revision N 连续形成 N+1；original amount、旧 revision 和旧 Allocation 不变，新 revision 除 effective amount 外完整复制 source revision；
- 减少结果必须大于 0 且不得低于 Allocation 净额；等于净额时 outstanding 为 0 但状态仍为 `open`；增加和减少均使用 PostgreSQL `numeric(24,6)` 精确计算；
- 既有 Allocation 保留历史 Obligation revision；调整后的新 Allocation 必须引用 current revision，旧 revision 请求返回 409；
- 相同 Command ID／负载串行或并发重放不重复推进；改变负载／Actor 冲突，不同 Command 并发消费同一 source revision 时恰好一个成功；
- Adjustment／Allocation Apply 与 Adjustment／Allocation Reverse 在 Obligation 行锁上形成确定顺序，最终 effective amount 不低于净分配额；确定性直接 SQL 等待竞态也不能绕过最新净额；
- 状态、revision、前值、币种、精确算术、非金额字段复制、净分配下限与 current revision 推进同时受 repository 和数据库触发器保护；Adjustment、revision、Command 与审计不可更新或删除；
- 缺失义务、非 open、非法类型、空／过长原因、非法金额、零结果、低于净分配、CSRF、非 administrator、审计失败和数据库失败不留下半成品；
- migration 从历史 `000010` 升至 `000011` 并可重放，既有 revision 1、Allocation／Reversal、Cash Reversal 和余额保持不变；
- 隔离空库 HTTP 流程完成确认、增加、减少、Allocation、低于净额拒绝、等于净额、旧 revision 分配拒绝、重放／冲突和全部投影核对；
- Contract Ledger 停止时上述流程与 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量 `-race`、两个应用真实 PostgreSQL 全集成 `-race`、关键并发 20 轮，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M4-004 Obligation 减免

领域、HTTP、余额投影与 Allocation／Adjustment 兼容性测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/allocation ./internal/httpserver
```

两个应用真实 PostgreSQL 全集成回归：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

关键锁序和数据库直写等待竞态的可重复压力检查：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=20 -tags=integration \
  -run '^(TestPostgresObligationRemissionConcurrency|TestPostgresObligationRemissionAndAllocationApplySerialize|TestPostgresDirectObligationRemissionWaitsForAllocationInsert|TestPostgresObligationRemissionAndAmountAdjustmentSerialize|TestPostgresObligationRemissionAndAllocationReversalSerialize)$' \
  ./internal/obligation
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/obligations` 进入一条 open Obligation 详情。

验收至少覆盖：

- administrator 提交正减免金额、必填原因、必填人工来源依据和 Expected Current Revision；记录时 effective amount、币种、Actor 与时间由服务端和数据库可信取得，客户端伪造无效；
- 每笔 Remission 是独立、不可变处置事实，不推进 revision，也不覆盖 original／effective amount、旧 revision 或既有 Allocation；同一 revision 可以连续追加多笔；
- outstanding 按 `effective - net allocated - remitted` 精确计算；减免可以等于但不能超过最新 outstanding，余额归零后 Obligation 仍为 `open`；
- 相同 Command ID／负载串行或并发重放不重复写入；改变负载／Actor 冲突，不同命令并发累计不能超额；
- 新 Allocation 必须扣除累计 Remission；减少 Amount Adjustment 的结果不得低于净 Allocation 与累计 Remission 之和，增加 Adjustment 可以扩大后续余额；
- Remission／Allocation Apply、Remission／Allocation Reverse 与 Remission／Amount Adjustment 共用 Obligation 行锁形成可解释顺序；直接 SQL 等待竞态也必须读取最新聚合余额；
- PostgreSQL 触发器拒绝伪造 revision、币种、记录时有效金额、超额 Remission、超额 Allocation 和穿透已减免下限的 Adjustment；Remission、Command 与审计不可更新或删除；
- 缺失义务、非 open、过期 revision、非法金额、空／过长原因或来源、超额、CSRF、非 administrator、审计失败和数据库失败不留下半成品；
- migration 从历史 `000011` 升至 `000012` 并可重放，既有 revision、Adjustment、Allocation／Reversal、Cash Reversal 和余额保持不变；
- 隔离空库 HTTP 流程覆盖调额、既有 Allocation、连续减免、超额拒绝、减免后继续 Allocation、组合归零、归零后继续减免拒绝、调额下限和完整投影核对；
- Contract Ledger 停止时上述流程与 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`、关键并发 20 轮，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M4-006 Obligation 受控关闭

领域、HTTP、余额序号与终态保护测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/allocation ./internal/httpserver
```

两个应用真实 PostgreSQL 全集成回归：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

关闭命令、关闭／余额写入竞态和数据库直写等待的重复压力检查：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=20 -tags=integration \
  -run '^TestPostgres(ObligationClosureConcurrency|DirectObligationClosureWaitsForAllocationReversal)$' \
  ./internal/obligation
```

聚焦的独立运行验收：先停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/obligations` 进入一条 outstanding 为 `0` 且仍为 `open` 的 Obligation 详情。

验收至少覆盖：

- outstanding 非零时不显示关闭入口；全 Allocation、全 Remission 或混合归零时显示表单，但不会自动关闭；
- administrator 只提交关闭原因、Expected Current Revision 与 Expected Balance Sequence；金额、币种、结清构成、Actor 与时间由服务端和数据库可信派生，客户端伪造无效；
- 成功后形成一条不可变 Closure、Command 和审计，根状态变为 `closed`，current revision 与全部金额事实不变，详情显示冻结事实；
- 相同 Command ID／负载串行或并发重放只形成一份关闭事实；改变负载或 Actor 冲突，不同 Command 并发只有一个成功；
- 余额事实经过 Allocation Reversal 或重新分配后，即使再次为 0，旧 Expected Balance Sequence 也冲突；Amount Adjustment 推进 revision 后旧关闭命令冲突；
- Closure 与 Allocation Reversal、Amount Adjustment 共用 Obligation 行锁形成确定顺序；零余额时新的 Allocation Apply 和 Remission 本身已无可用余额；
- closed 后 Adjustment、Remission、Allocation Apply、Allocation Reversal、再次关闭和直接状态回退均拒绝，关联 Allocation 详情不再展示反向入口；
- 审计失败整体回滚；直接 SQL 不能伪造冻结事实、单独插 Closure、单独推进 `closed`、重开终态或绕过 Allocation Reversal 状态保护；
- migration 从历史 `000012` 升至 `000013` 并可重放，既有 Obligation、revision、Adjustment、Remission、Allocation／Reversal 与余额保持不变；
- 隔离空库 HTTP 流程完成确认、调额、Allocation／Remission 组合归零、显式关闭、重放／冲突、closed 后余额写入拒绝和数据库投影核对；
- Contract Ledger 停止时上述流程与 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不伪造连接器；
- `make check`、两个应用全量单元／集成 `-race`、关闭关键并发 20 轮，以及两个应用 `GOWORK=off` 独立测试／构建通过。

## MF-M4-008 当前 Obligation 逾期与账龄投影

领域分段、精确汇总和 HTTP 边界测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/httpserver
```

两个应用真实 PostgreSQL 全集成回归：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：`http://127.0.0.1:8083/aging`

验收至少覆盖：

- 分类日缺省为服务当前 UTC 日期；严格日期错误或过去日期返回 `422`，今天和未来可查询，页面明确未来分类不是余额预测；
- `due_on` 等于分类日进入未到期，1／30／31／60／61／90／91 天边界分别进入固定 bucket；
- 只纳入 `open` 且 current outstanding 大于 0 的义务，draft、open 零余额和 closed 排除；
- Allocation 后报告显示减少后的余额，Allocation Reversal 后恢复，Amount Adjustment 与 Remission 使用既有统一 current 投影，组合归零和关闭后均不再列出；
- 应收／应付及不同币种分别汇总；六位精确小数用定标大整数累加，不经过二进制浮点数；
- 汇总展示笔数、current outstanding、逾期笔数／金额和五个 bucket 金额，明细按确定顺序并可下钻到 Obligation；
- repository 或组成事实读取失败返回 `500`，不伪装为空报告；非 administrator 在调用领域查询前被拒绝；
- 本功能没有 migration、写命令或审计动作，本地 schema 保持 `000013`；
- Contract Ledger 停止且端口不可达时，真实 PostgreSQL 完整 Money Flow HTTP 流程、账龄变化和 `8083/health/ready` 继续通过；随后 Contract 以 `verify` 恢复，两个应用均 ready；
- `make check`、Money Flow 单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`，以及两个应用 `GOWORK=off` 独立测试／构建通过；两个数据库的临时测试库均归零。

## MF-M4-009 历史 Obligation 余额 `as_of`

领域、查询页面与错误边界测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/obligation ./internal/httpserver
```

recorded-time migration、真实 PostgreSQL 重放和完整 HTTP 纵向联调：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/obligation ./internal/httpserver
```

时间门禁在锁等待后读取最新已提交事实的重复压力检查：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=20 -tags=integration \
  -run '^TestPostgresRecordedTimeGuardRejectsBackdatedFactAfterLockWait$' \
  ./internal/obligation
```

两个应用全量真实 PostgreSQL 回归：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：从 `http://127.0.0.1:8083/obligations` 进入一条已经确认的 Obligation，再选择“按系统记录时刻查询历史余额”。

验收至少覆盖：

- `as_of` 必须是带时区 RFC3339 instant，归一为 UTC，包含同刻事实；缺省为服务当前时刻，未来值与格式错误返回 `422`；
- Obligation 不存在返回 `404`，早于 confirmation 返回 `422`，draft 不形成正式历史余额；
- confirmation、Allocation、Allocation Reversal、连续 Adjustment、Remission 和 Closure 前后都从不可变事实得到精确 effective、net allocated、remitted、outstanding、sequence 和状态；
- 同刻 resulting revision 与 Adjustment 作为不可拆分批次，不按 UUID 创造中间状态；current 根状态和所有后续事实不污染较早快照；
- 查询使用 `REPEATABLE READ READ ONLY`，历史前缀负余额、closed 非零余额或状态矛盾返回 `500`，不能渲染伪成功；
- migration 从 `000013` 升到 `000014_obligation_recorded_time_guards.sql` 时验证既有因果顺序和所有历史前缀，不改写原时间；倒退事实使升级失败并回滚；
- revision、Adjustment、Allocation、Allocation Reversal、Remission 和 Closure 的新时间都必须不早于同一 Obligation 既有最大记录时间；门禁先锁 Obligation 行以覆盖并发和直接 SQL，确定性等待竞态连续 20 轮通过；
- 页面展示 recorded-time 口径、余额公式、冻结主体、revision、sequence 和 Closure，并可返回 current 详情；读取不写审计或命令；
- 本功能只提供单条 Obligation 历史快照，不把它解释为业务生效日、会计期间或跨义务历史账龄；
- Contract Ledger 停止时完整 Money Flow HTTP 历史回放和 `8083/health/ready` 继续通过；当前没有跨系统业务流动，因此不制造连接器联调；
- `make check`、Money Flow 单元 `-race`、两个应用真实 PostgreSQL 全集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建通过；本地 Money Flow schema 为 `000014`，两个数据库临时测试库均归零。

## MF-M4-011 当前经营收支汇总与逐项下钻

领域公式、精确金额与 HTTP 边界测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/operatingsummary ./internal/httpserver
```

真实 PostgreSQL 六项汇总、下钻和完整 HTTP 纵向流程：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/operatingsummary ./internal/httpserver
```

两个应用全量真实 PostgreSQL 回归：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：`http://127.0.0.1:8083/operating-summary`

验收至少覆盖：

- 按 ledger owner Party ID 与 currency 分组，同名不同 Party、同 Party 不同币种均不合并；
- 六项指标严格复用既有 outstanding、current effective 与 unallocated 公式，`0.1 + 0.2 = 0.3` 且不使用浮点数；
- partial Allocation、Allocation Reversal、Remission、Cash Reversal 和 Closure 的 current 结果正确；草稿、零贡献与 Internal Transfer 不进入；
- 六项正金额均可下钻到完整贡献事实，下钻总额等于逐条贡献之和并链接现有详情；零金额没有伪造入口；
- 汇总与下钻分别使用只读 `REPEATABLE READ`；非法参数 `422`、无贡献 `404`、读取或不变量失败 `500`；
- administrator 可读，匿名跳转登录，非 administrator 在调用领域查询前返回 `403`；查询不写命令、审计或业务事实；
- 本功能没有 migration，本地 schema 保持 `000014_obligation_recorded_time_guards.sql`；
- Contract Ledger 停止时完整 Money Flow HTTP 流程、汇总和 `8083/health/ready` 继续通过；当前没有跨系统业务流动，因此不制造连接器联调；
- `make check`、Money Flow 全量单元与真实 PostgreSQL 集成 `-race`、Contract Ledger 全量 PostgreSQL 集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建和文档链接检查通过；两个数据库临时测试库归零。

## MF-M4-012 本方主体可校验台账导出

领域归档、清单哈希、CSV 往返与 HTTP 边界测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/ledgerexport ./internal/httpserver
```

真实 PostgreSQL 一致快照、owner 隔离、损坏关系拒绝和完整 HTTP 下载：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/ledgerexport ./internal/httpserver
```

两个应用全量真实 PostgreSQL 回归：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

聚焦的独立运行验收：停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：`http://127.0.0.1:8083/ledger-exports`

验收至少覆盖：

- 页面只能由 administrator 访问；生成使用 CSRF POST 和会话可信 Actor，匿名跳转登录，viewer 返回 `403`；
- 一个 owner 的当前 v2 导出固定 `manifest.json` 与十三张 CSV，包含完整 `obligation_cash_expectations.csv`；文件名、表头、顺序、权限和 ZIP 根层级稳定，旧 v1 明确不含 Expectation；
- manifest 的 schema、owner、行数、字节数和 SHA-256 与实际 CSV 全部一致，未知格式或列变更必须由新版本表达；
- CSV 是 UTF-8 RFC 4180、CRLF 与固定表头；精确金额、空值、逗号、引号、换行和公式前缀文本往返不改变业务事实；表格软件应将所有列按文本导入；
- Obligation 全 revision、Adjustment、Remission、Closure、Cash Expectation、Cash、两类 Reversal、Allocation、Internal Transfer 和范围内 Audit 都来自一个只读 `REPEATABLE READ` 快照；并发写入不会形成撕裂归档；
- 只包含 owner 与范围内事实实际引用的 Party；其他 owner、身份／会话、幂等 payload hash、派生汇总、Contract 数据均不进入；跨 owner Allocation 等损坏事实使导出整体失败；
- 归档构造完成后才写 `money_flow_ledger_exported` 审计，审计失败不返回文件；当前成功审计不倒灌进已经完成的归档，下次导出才可见；
- 当前 v2 导出运行在 `000015_obligation_cash_expectations.sql` 或更高兼容 schema；它不是数据库备份、恢复包、导入、签名或会计报表；
- Contract Ledger 停止时真实 PostgreSQL 完整 Money Flow HTTP 流程、ZIP 下载／校验和 `8083/health/ready` 继续通过；当前没有跨系统业务流动，因此不制造连接器联调；
- `make check`、两个应用全量 PostgreSQL 集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建和文档链接检查通过；两个数据库临时测试库归零。

## MF-M4-013 可验证全量备份与空环境恢复

严格归档、摘要、路径和 PostgreSQL 客户端边界测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/systembackup ./cmd/backup
```

真实 PostgreSQL 完整业务恢复、失败回滚与同一 snapshot 并发测试：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./internal/systembackup
```

Money Flow 全量单元和真实 PostgreSQL 回归：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./...
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

Contract Ledger 全量真实 PostgreSQL 回归：

```bash
cd backend/services/contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  go test -race -count=1 -tags=integration ./...
```

本地真实备份：

```bash
cd backend/services/money-flow-ledger
DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
POSTGRES_CLIENT_DATABASE_URL='postgres://money_flow@127.0.0.1:5432/money_flow?sslmode=disable' \
go run ./cmd/backup create \
  --pg-dump-bin ./ops/local-pg-dump \
  --output '/绝对的新路径/money-flow-ledger-drill.zip'
```

恢复演练命令和专用端口启动方式见 [Money Flow 全量备份与恢复运行手册](./operations/money-flow-ledger-backup-restore.md)。

聚焦的独立运行验收：停止 Contract Ledger 进程并确认 `127.0.0.1:8081` 不可达，再执行：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration ./internal/systembackup
curl --fail http://127.0.0.1:8083/health/ready
```

验收至少覆盖：

- 创建输出必须是尚不存在的安全路径，最终权限 `0600`；客户端数据库和 Go snapshot 数据库不一致时失败且不留下目标文件；
- ZIP 只能有根目录 `manifest.json` 与 `database.dump`，使用普通 store entry；路径穿越、未知／重复 entry、压缩、加密、未知 JSON 字段和摘要不符拒绝；
- `pg_dump`、schema version 和所有非 session public base table 计数来自同一只读 `REPEATABLE READ` snapshot，并持有与 migration 互斥的 shared advisory lock；
- snapshot 后并发提交的 Party 不进入当次计数或 dump，但保留在源库并进入未来备份；
- dump 排除 `local_sessions` data，保留本地用户、密码摘要、失败锁定状态、命令 payload hash、完整审计和全部 Money Flow 事实；
- 来源／目标／客户端 PostgreSQL major 不一致时拒绝；恢复只接受没有用户 relation 的新数据库，不提供覆盖、合并或 `--force`；
- 无效 custom dump 经 `pg_restore --single-transaction --exit-on-error` 失败后目标仍为空；恢复提交后的核验错误必须隔离目标；
- 恢复后 schema、完整非 session 表集合和逐表计数与 manifest 相同，session 为 0；migration 可重放且不改变事实；
- 旧 Cookie 对应的 token 失效，原账号密码可重新登录；Party、完整 Obligation 历史、Cash／Reversal、Allocation／Reversal、Internal Transfer、命令和审计保持，既有命令可幂等重放且可以继续写新事实；
- 恢复实例在独立端口通过 live／ready；测试数据库和临时归档安全清理；
- Contract Ledger 停止时完整备份恢复集成和 Money Flow ready 继续通过；当前没有跨系统业务流动，因此不制造联合备份或连接器联调；
- `make check`、两个应用全量 PostgreSQL 集成 `-race`、两个应用 `GOWORK=off` 独立测试／构建和文档链接检查通过；临时测试数据库归零。

## MF-M4-014 独立安装与升级交付

schema 状态、单事务 migration 和并发锁测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 ./internal/buildinfo ./internal/config ./internal/database
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run 'TestExplicitSchemaGateAndAtomicPendingMigrations|TestConcurrentExplicitMigrationsSerialize' \
  ./internal/database
```

全量回归：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration ./...

cd ../..
make check
```

release bundle：

```bash
RELEASE_PROVENANCE_MODE=development \
RELEASE_COMMIT='money-flow-test' \
RELEASE_BUILT_AT='2026-09-20T12:00:00Z' \
make money-flow-ledger-release \
  VERSION='v0.1.0' \
  OUTPUT_DIR='/绝对的新目录'
```

全新安装和升级的生产步骤见 [Money Flow 独立安装与升级运行手册](./operations/money-flow-ledger-install-upgrade.md)。验收必须使用实际归档解包后的二进制，而不是源码 `go run`。

验收至少覆盖：

- bundle 只有 server、ops、backup、示例配置和两份文档，SHA-256 正确，同名归档或摘要不覆盖；
- 三个二进制的 version、commit、build time 和 target 完全一致；版本命令不连接数据库或读取运行配置；
- `ops check` 正确区分 empty、pending、current、未知 migration、版本缺口和非托管 relation；PostgreSQL major 不是 18 时拒绝；
- `ops migrate` 只接受 empty／严格前缀／current，全部 pending migration 在同一事务中执行；故意让后续 migration 失败时，前序 pending migration 不留下表或版本记录；
- 两个并发 migrate 由同一 transaction advisory lock 串行化；全量备份的 shared lock 与 migration 独占锁互斥；
- `MIGRATION_MODE=verify` 对 empty、pending、unknown、gap 均在监听前失败；current 时不写 schema 并可启动；`auto` 不修改 incompatible 数据库；
- 全新空库按 `check → migrate → require-current → verify server` 完成管理员 bootstrap、登录、主体写入、live／ready；移除 bootstrap 密码后重启，已有账号不被覆盖；
- 合法历史 schema 经实际 bundle 升级后，身份、Party、Obligation、Cash Transaction、Allocation、幂等命令和 Audit 保持；运行时仍可继续写入；
- 升级前用 MF-M4-013 生成全量备份；需要回退时只恢复到新的空数据库，不执行 down migration、不清理或覆盖失败目标；恢复目标保持原 schema 和完整事实，session 为 0；
- Contract Ledger 停止或不存在时完成构建、安装、升级、恢复、登录和业务验收；没有跨系统数据库、文件、调用或交付依赖。

## MF-M4-015 当前预计现金排期

领域、HTTP 与归档单元测试：

```bash
cd backend/services/money-flow-ledger
go test -race -count=1 \
  ./internal/obligation ./internal/httpserver ./internal/ledgerexport
```

真实 PostgreSQL migration、并发、排期投影、导出和完整 HTTP 纵向流程：

```bash
cd backend/services/money-flow-ledger
docker compose up -d --wait
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./internal/database ./internal/obligation ./internal/ledgerexport ./internal/httpserver
```

两个应用全量回归与独立构建：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration ./...

cd ../contract-ledger
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration ./...

cd ../..
make check
```

独立运行验收时停止 Contract Ledger，确认 `127.0.0.1:8081` 不可达，再执行 Money Flow 的完整 HTTP 集成并检查当前服务：

```bash
cd backend/services/money-flow-ledger
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  -run '^TestMoneyFlowAdministratorPartyAndObligationFlowAgainstPostgreSQL$' \
  ./internal/httpserver
curl --fail http://127.0.0.1:8083/health/ready
```

手工验收入口：`http://127.0.0.1:8083/cash-forecast`

验收至少覆盖：

- administrator 可设置、改期和清除预计日期；匿名进入登录，非 administrator 在调用用例前返回 `403`，所有写入都使用会话可信 Actor 和有效 CSRF；
- Expectation 以连续、不可变 sequence 保存 `set`／`clear`、source revision、原因和系统记录时间；同 Command ID 同负载安全重放，不同负载返回冲突；
- `due_on`、Obligation revision、原金额和余额事实不被排期操作修改；draft、closed、revision 过期、零余额 set、无当前排期 clear 都明确拒绝且无部分写入；
- 两个并发写入在 Obligation 行锁下形成连续 sequence 或明确冲突；直接 SQL 的 sequence gap、状态／余额错误、时间倒退及 UPDATE／DELETE 被数据库门禁拒绝；
- 当前预计金额严格等于查询时 current outstanding；Allocation 或 Remission 后自动减少，余额归零或状态不再 open 后从当前投影消失；
- 没有当前 set 的 open 正余额只进入未排期；已排期按 expected date、本方主体、方向和币种隔离，所有汇总都能从明细精确重建；
- 页面显示已排期、未排期和完整历史；读取使用单个只读 `REPEATABLE READ` 快照，不写命令、审计或派生金额；
- `000015_obligation_cash_expectations.sql` 从真实 `000014` 升级并可重放，既有事实和 historical balance 不变；表、索引、外键和触发器集合符合基线；
- `owner-ledger-export-v2` 固定包含 manifest 与十三张 CSV，Expectation 全历史和对应 audit 在同一快照中，预计金额不进入归档；
- Contract Ledger 停止时 Money Flow 可独立完成排期、历史和 v2 导出；当前没有真实跨系统业务流动，因此不制造连接器联调。

## MF-M6-001 Money Flow 全新环境 Release 验收

先按运行手册构建带明确 version、commit 和 build time 的当前平台 release bundle，然后停止 Contract Ledger 并执行：

```bash
cd /仓库根目录
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
make money-flow-ledger-clean-install-acceptance \
  BUNDLE='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CHECKSUM='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

验收至少覆盖：

- 外部摘要与归档字节完全一致，归档只有固定六个文件；路径穿越、软链接、AppleDouble、未知、缺失或重复 entry 在二进制运行前拒绝；
- server、ops、backup 的 version、commit、build time、Go version 和 target 完全一致且非 `dev`／`unknown`；
- 随机全新数据库由实际 ops 依次报告 `empty → current`，最新 schema 为 `000015_obligation_cash_expectations.sql`；
- 实际 server 使用白名单环境和 `MIGRATION_MODE=verify` 启动，live／ready、登录、Cookie 与 CSRF 通过；环境不包含 Contract Ledger 或连接器配置；
- 代表性链路形成两个 Party、一条 100.50 CNY open receivable、一条 Expected Cash Date、一笔 40.25 CNY receipt 和一条 40.25 CNY Allocation；数据库与页面的 current outstanding 均为 `60.250000`；
- 实际 HTTP v2 导出固定包含 manifest 与十三张 CSV，逐文件字节数／SHA-256 正确，Obligation、Expectation、Cash 和 Allocation 事实存在；
- 首次服务优雅停止后，不再提供 bootstrap 三元组即可重启；原账号仍可登录、用户数量仍为 1、业务事实与投影保持；
- 失败和成功路径都只回收本次进程、随机数据库和临时解包目录，不触碰开发库。
- 与本切片相邻的全量 `-race` 回归必须通过；Remission／Allocation Reversal 并发夹具使用数据库当前最大事实时间之后的同一合法时刻，不能让 goroutine 调度制造倒退时间，也不能放松运行时 recorded-time 门禁。

## MF-M6-002 Money Flow Clean-room 备份交接与空库恢复

先构建带明确 build identity 的当前 release bundle，停止 Contract Ledger，然后执行：

```bash
cd /仓库根目录
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
make money-flow-ledger-backup-restore-acceptance \
  BUNDLE='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CHECKSUM='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

验收至少覆盖：

- 实际 bundle 的 ops 在随机源库迁移，实际 server 经登录、Cookie 与 CSRF 建立两个 Party、100.50 CNY 应收、预计日期、40.25 CNY 实收与 Allocation；
- 源服务在线时，实际 backup binary 调用真实 PostgreSQL 18 `pg_dump`，输出权限为 `0600` 的 full-backup-v1；
- ZIP 只有 store method 的 `manifest.json` 与 `database.dump`，manifest 使用严格 JSON 并记录 current schema、session 排除、完整表计数以及 dump 大小和 SHA-256；
- 整个 ZIP 的外部 SHA-256 在复制到独立交接路径前后不变；
- 实际 backup binary 调用真实 `pg_restore` 恢复到另一个随机空库，恢复后 schema current、session 为 0；
- 实际恢复 server 不接收 bootstrap 配置；源 Cookie 被重定向到登录，原账号可重登并读取预计日期、`60.25 CNY` 余额和完整关联事实；
- 恢复后经 HTTP 新建第三个 Party，源库仍保持两个 Party，证明目标可继续写入且两个实例隔离；
- 已迁移的非空目标在恢复写入前拒绝；dump 被篡改的归档明确失败，失败目标用户 relation 数保持 0；
- 成功和失败路径只回收本次随机数据库、进程与临时目录，不触碰开发库；Contract Ledger 停止时全流程通过。

默认客户端入口是 Money Flow 本地 Compose wrapper；外部客户端可通过绝对路径 `DELIVERY_PG_DUMP_BIN` 和 `DELIVERY_PG_RESTORE_BIN` 覆盖。运行细节见 [Money Flow 全量备份与恢复运行手册](./operations/money-flow-ledger-backup-restore.md)。

## CL-M6-003 Contract Clean-room Release、Evidence 与恢复

构建带明确 build identity 的当前 Contract release bundle，停止 Money Flow，然后执行：

```bash
cd /仓库根目录
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
make contract-ledger-clean-install-backup-restore-acceptance \
  BUNDLE='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CHECKSUM='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

验收至少覆盖：

- 外部摘要、固定六文件 release 和三个实际二进制 build identity；AppleDouble、额外文件、软链接、路径穿越或错误类型在二进制运行前拒绝；
- 随机空库和新 Evidence 根由实际 ops 完成 `empty → current`，实际 server 以 `verify`、一次性 bootstrap 和白名单环境启动；
- 经真实 HTTP／Cookie／CSRF 建立两个 Party、1000 CNY 草稿合同，并把固定文件上传到合同整体；数据库元数据、普通文件权限、下载字节和 SHA-256 一致；
- 源服务在线时，实际 backup 调用真实 `pg_dump` 生成数据库+Evidence full-backup-v1；ZIP 的 manifest、custom dump、唯一 Evidence entry、大小和 SHA-256 通过离线核验；
- whole-ZIP 摘要在独立交接副本上保持，实际 backup 调用真实 `pg_restore` 恢复到空数据库和不存在的 Evidence 根；
- 恢复后 schema current、session 为 0、临时恢复标记移除；无 bootstrap 的实际 server ready，源 Cookie 失效，原账号重登后合同与原 Evidence 字节可读取；
- 恢复实例可新建第三个 Party，源库仍为两个；非空数据库拒绝且不创建目标 Evidence，已存在 Evidence 根保持 sentinel 且数据库为空；
- Evidence entry 被篡改但 ZIP 结构合法时因摘要不一致失败，失败目标用户 relation 为 0，Evidence 根不发布；
- Money Flow 停止时全流程通过；成功和失败路径只回收本次随机数据库、进程和临时目录。

默认客户端入口是 Contract 本地 Compose wrapper；外部客户端可用绝对路径 `DELIVERY_PG_DUMP_BIN` 和 `DELIVERY_PG_RESTORE_BIN` 覆盖。运行细节见 [Contract Ledger 全量备份与恢复运行手册](./operations/contract-ledger-backup-restore.md)。

## M6-004 双 Release 同主机组合安装与故障隔离

使用已经分别通过 clean-room 验收的两个实际 bundle 执行：

```bash
cd /仓库根目录
CONTRACT_TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
MONEY_FLOW_TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
make combined-delivery-acceptance \
  CONTRACT_BUNDLE='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz' \
  CONTRACT_CHECKSUM='/绝对路径/contract-ledger_v0.1.0_darwin_arm64.tar.gz.sha256' \
  MONEY_BUNDLE='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz' \
  MONEY_CHECKSUM='/绝对路径/money-flow-ledger_v0.1.0_darwin_arm64.tar.gz.sha256'
```

验收至少覆盖：

- 两个外部摘要、两个固定 release 文件集合和各自三个二进制一致且可追溯的 build identity；两个 release 不要求同版本发布；
- 两个随机数据库、两个随机监听端口和 Contract 独占 Evidence 根，实际 ops 分别迁移到 current；
- 两个 actual server 同时 ready，同一个 Cookie Jar 依次登录后仍能同时访问两个首页；
- `contract_ledger_session`、`contract_ledger_csrf`、`money_flow_ledger_session`、`money_flow_ledger_csrf` 四个名称同时存在且各一份；
- 两侧先各创建一个本地 Party；Contract 停机时 Money Flow 保持 ready、登录与第二次写入，Contract 无 bootstrap 重启后原会话和第二次写入通过；
- Money Flow 停机时 Contract 保持 ready、登录与第三次写入，Money Flow 无 bootstrap 重启后原会话和第三次写入通过；
- 最终两个数据库各有三个本地 Party；Contract 库没有 Obligation／Cash 表，Money Flow 库没有 Contract／Evidence 表；
- Contract Evidence 的根、`.tmp` 和 `objects` 只由 Contract 创建，Money Flow 的探针路径始终不存在；
- 验收进程环境不传入对方数据库或文件配置，也没有跨系统 HTTP、事件、文件或业务数据流动。

该入口自身是 `GOWORK=off` 可运行的独立 Go 模块。缺少任一 bundle、摘要或数据库入口时不会猜测输入。详细契约见 [M6-004 双 Release 同主机组合安装与故障隔离](./specs/combined-delivery/same-host-install-fault-isolation.md)。

## M6-005 三类参考装配场景边界审计

该切片是产品事实与架构边界审计，不是客户数据、页面或业务流的运行时验收。检查内容为：

- 设计单位 OA、资质施工综合体、小型咨询服务公司三个参考场景逐项区分已验证能力、条件可用、明确缺口和待客户确认；
- 每个“已验证能力”都能追溯到当前规格或 M6 actual release 验收，不把架构设想写成客户接受；
- Project、Approval、生产、奖金、管理费、人员、导入、票据和 OA 身份没有进入两个应用的 migration、核心表或启动依赖；
- 没有创建客户分支、万能字段、空连接器、共享 Party 或共享 IAM；
- 全仓质量门禁、两个应用真实 PostgreSQL `-race`、独立构建、Markdown 本地链接和 shell 语法仍通过。

详细结果见 [M6-005 三类参考装配场景边界审计](./specs/assembly/three-reference-scenarios-boundary-audit.md)。

## M6-006 可交付性退出缺口审计

该切片逐条核对 M6 原验证内容和退出门槛，不新增运行时代码。审计结论必须满足：

- 已通过项都能追溯到 actual release、真实 PostgreSQL 或现有自动化证据；
- `部分通过` 不写成完整客户验收；
- 没有数据规模、硬件和 SLO 时不制造性能通过线；
- 没有 Excel 样例和批次／冲突／纠错语义时不实现通用导入；
- 尚不存在的连接器和对象存储不为故障测试而创建；
- 选出的下一切片只补齐已有证据支持的组合运行手册缺口。

详细结果见 [M6-006 可交付性退出缺口审计](./specs/delivery/m6-exit-gap-audit.md)。

## M6-007 双 Release 同主机运行手册与操作复演

操作步骤见[双 Release 同主机运行手册](./operations/combined-same-host-operation.md)。交付前复演必须同时执行：

1. `make combined-delivery-acceptance`，验证实际归档的独立迁移、双启动、同一 Cookie Jar 双登录、四 Cookie、交替单侧重启和零数据／文件交叉；
2. `make contract-ledger-clean-install-backup-restore-acceptance`，验证 Contract 数据库与 Evidence 的实际联合备份和空目标恢复；
3. `make money-flow-ledger-backup-restore-acceptance`，验证 Money Flow 实际数据库备份、交接和空目标恢复；
4. 查询两个 PostgreSQL 测试入口，确认随机数据库无残留；
5. 检查原 8081／8083 开发服务仍分别 `/health/ready`。

2026-09-20 使用 Contract `v0.1.0-test.3` 与 Money Flow `v0.1.0-test.5` darwin/arm64 actual bundle 并行复演，三项均通过；随机库查询为空，两个开发服务仍 ready。该证据不代替真实新操作者、生产进程监督、TLS／代理、容量目标或客户场景验收。详细契约见 [M6-007 双 Release 同主机运行手册与操作复演](./specs/delivery/combined-same-host-operation-rehearsal.md)。

## M6-008 正式 Release 源代码溯源门禁

根级质量门禁自动执行：

```bash
make release-provenance-test
make check
```

正式构建必须从干净、有 HEAD 的仓库根执行：

```bash
RELEASE_PROVENANCE_MODE=official \
RELEASE_BUILT_AT='2026-09-20T13:00:00Z' \
make contract-ledger-release VERSION='v0.1.0-baseline.1' OUTPUT_DIR='/仓库外绝对目录'

RELEASE_PROVENANCE_MODE=official \
RELEASE_BUILT_AT='2026-09-20T13:00:00Z' \
make money-flow-ledger-release VERSION='v0.1.0-baseline.1' OUTPUT_DIR='/仓库外绝对目录'
```

自动测试覆盖 development 无 Git、official 无 Git／无 HEAD、非根目录、tracked／staged／untracked 脏状态、错误 commit、缺失或错误时间，以及干净 HEAD 成功。2026-09-20 首个本地基线 `cdc7bc8349c8af4efda1b33d2d21f0498bffbffd` 的两个 official-mode 候选包通过摘要、六二进制 identity、组合验收和两套 clean-room 备份恢复；随机库无残留，开发服务仍 ready。详细契约见 [M6-008 正式 Release 溯源门禁](./specs/delivery/official-release-provenance-gate.md)。

## M5 启动前 OA 参考证据审计

该审计不验证跨系统连接器，因为连接器尚未开始。验证只覆盖参考证据的可追溯性、定向行为和本仓库文档基线：

```bash
cd /Users/crispydog/Workspace/Hoewo/WebProjects/RoadSystemOA/road-design-oa-system
git status --short
git rev-parse HEAD

cd backend/internal/services
go test contract_amount_service.go contract_amount_service_test.go -count=1

cd /Users/crispydog/Workspace/Hoewo/WebProjects/OMS/BizFoundry
make check
```

2026-09-20 的 OA 参考源为干净提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`，合同金额计算定向测试通过；OA 的 `internal/services` 整包测试会被其他陈旧测试的编译错误阻断，不能据此声称 OA 全量回归通过。BizFoundry 根级 `make check` 和本地 Markdown 链接检查通过，没有应用、schema、连接器或运行配置变更。详细结论见 [Contract → Money Flow：OA 参考证据审计](./specs/connectors/contract-money-oa-reference-audit.md)。

案例采集表与启动门禁的文档验收还必须确认：九项 R1 至 R9 与采集表字段一一对应；未通过案例不会获得 M5 功能编号；通过只进入契约设计而非直接编码；文档没有要求收集生产凭证或敏感身份数据。入口见[真实业务案例采集表](./templates/contract-money-flow-case-intake.md)和 [M5 启动就绪门禁](./specs/connectors/m5-start-readiness-gate.md)。

## M5 启动前 OMS 雏形参考证据审计

在 OMS 参考项目执行：

```bash
cd /Users/crispydog/Workspace/Hoewo/WebProjects/OMS/OMS
git status --short -- README.md docs arckit/spec arckit/interaction platform/app/oms-workbench.tsx
git rev-parse HEAD

cd platform
npm run lint
npm test
```

2026-09-20 被审计路径与 HEAD `27c614838fe14c16fb38a08fea8a5263eaaccc78` 一致；ESLint、生产构建和 2 项服务端渲染测试通过，只有 chunk 体积警告。原型验证不证明持久化、生产审批或跨系统联调。参考仓库原有 Arckit ledger 改动未被修改，测试产生的 `.npm-cache` 已移到 `/Users/crispydog/.Trash/oms-reference-npm-cache-20260920-01`。详细判定见 [OMS 雏形参考证据审计](./specs/connectors/contract-money-oms-prototype-reference-audit.md)。

## M7-001 身份与权限契约验证

```bash
cd acceptance/access-contract
GOWORK=off go test ./...
```

该独立模块验证四份公开 JSON Schema 可读取且具有稳定 ID，正常样例满足跨字段规则，并固定未知字段／版本、错误 issuer／tenant／audience、越命名空间 action、非法 scope、重复 Capability、重叠路由、过长 TTL 和失效时间等失败关闭行为。它不依赖 IAM、Gateway、Contract 或 Money Flow 源码，因此可在各实现接入时复用。

2026-09-20 已执行通过。对应契约见 [`contracts/access/v1/`](../contracts/access/v1/README.md)，安全边界见 [M7-001 威胁模型](./specs/identity/access-threat-model-v1.md)。

## M7-002 Identity & Access 纵向验收

单元与独立模块检查：

```bash
cd backend/platform/identity-access
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

真实 PostgreSQL 全生命周期：

```bash
cd backend/platform/identity-access
docker compose up -d --wait
TEST_DATABASE_URL='postgres://identity_access:identity_access_dev@127.0.0.1:55434/identity_access?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./internal/httpserver
```

验收会建立并回收随机隔离数据库，覆盖 migration 重放、首位管理员、邀请、一次性激活、登录、未知 Audience 拒绝、Manifest Capability 授予、Ed25519 凭证签名、密码重置、旧 Session Generation 失效、账号停用、最后管理员保护和审计 Trigger 不可变。2026-09-20 已在 PostgreSQL 18.6 实际执行通过。

另以实际 `go run ./cmd/server` 完成首次 `auto`＋bootstrap 启动、ready、停止、`identity-access-keyctl rotate`、公钥原子导出及无 bootstrap 的 `verify` 重启；重启后 JWKS 同时包含 retiring 与 active Key，私钥文件为 0600。详细规格见 [M7-002 IAM 最小纵向闭环](./specs/identity/iam-minimal-vertical-slice.md)。

## M7-003 统一入口与攻击边界

```bash
cd backend/platform/gateway
GOWORK=off go test ./...
GOWORK=off go vet ./...

cd ../unified-workbench
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

Nginx 配置校验：

```bash
docker run --rm --add-host=gateway:127.0.0.1 \
  -v "$PWD/deployments/d1/nginx/nginx.conf:/etc/nginx/nginx.conf:ro" \
  nginx:1.28-alpine nginx -t
```

Gateway 自动测试覆盖登录／Session Cookie 转交、CSRF、外来身份 Header 清除、可信 Workbench 上下文、Route Prefix Capability Cookie、Ed25519 离线验签、错误 issuer／tenant／audience／action／scope／TTL／签名、路径改写、禁用应用和限流。2026-09-20 已通过，并以实际 Nginx 容器、Gateway、IAM＋PostgreSQL、Workbench 进程复演匿名入口、登录 303、伪造 Header 清除、启用菜单和 `/auth/health/ready` 完整链路。2026-09-27 又增加可信边缘限流回归：默认不信任代理 Header；显式可信模式只接受一个有效 IP；逗号链和非法值退回直接对端；两个经同一可信边缘进入的不同客户端不共享限额。`go test ./backend/platform/gateway/...`、`sh deployments/d1/ops/test-ingress-mode` 和 `make d1-image-mode-test` 已通过，Nginx 两种入口配置均覆盖外来 `X-Forwarded-For` 为 `$remote_addr`。详细规格见 [M7-003 统一入口](./specs/assembly/unified-entry-m7-003.md)。

同日 Auth58 的 `v0.1.0-test.5` 目标复验保持正式 `REQUESTS_PER_MINUTE=120`：普通 D1 交付验收通过；Workbench 与 Gateway 两个不同容器地址分别经 Nginx 发出 70 次健康请求，合计 `140/140` 全部成功。C2 的 110 断言因本身是单地址高频合成负载，只在临时 `1200` 下执行，0 个状态不匹配、最大请求 `0.161712s`，并启用 Evidence 503、账号停用、IAM outage 和 Policy outage；验收结束后目标自动恢复 `120`、删除临时环境文件并再次通过 `d1 check`。环境证据见 [Auth58 D1 当前组合升级记录](./reports/auth58-d1-current-suite-upgrade-20260927.md)。

## M7-004 Contract Ledger IAM 试点验收

```bash
cd backend/services/contract-ledger
GOWORK=off go test ./...
GOWORK=off go vet ./...
TEST_DATABASE_URL='postgres://contract_ledger:contract_ledger_dev@127.0.0.1:55432/contract_ledger?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./internal/database
```

自动测试覆盖中心 Credential 离线复验、错误 Audience 拒绝、具体 Capability 到服务端动作与页面 `allowed_actions` 的映射、IAM 模式本地身份路由关闭、前缀路径、IAM Subject 审计，以及显式历史身份映射不可变。2026-09-20 已在 PostgreSQL 18.6 通过。

同日以实际 Nginx、Gateway、IAM＋PostgreSQL、Workbench、Contract＋PostgreSQL 进程完成统一入口验收：登录一次并授予八项 Contract Capability，交换 `contract-ledger` 单 Audience 凭证，创建“`M7 IAM 验收主体-*`”并回读；`audit_entries` 记录 `actor_kind=iam_subject` 和真实 IAM Subject UUID；直接无凭证访问 Contract 返回 401。暂停 Money Flow 进程后 Contract 仍返回 200 并保留刚写入数据，随后恢复 Money Flow。详细规格见 [M7-004 Contract IAM 试点迁移](./specs/contract-ledger/iam-adapter-m7-004.md)。

## M7-005 Money Flow Ledger IAM 迁移验收

```bash
cd backend/services/money-flow-ledger
GOWORK=off go test ./...
GOWORK=off go vet ./...
TEST_DATABASE_URL='postgres://money_flow:money_flow_dev@127.0.0.1:55433/money_flow?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./...
```

自动测试覆盖 Money Flow Credential、具体动作权限、IAM Subject 审计、前缀页面、本地模式和全部历史 migration。2026-09-21 以实际 Nginx、Gateway、IAM、Workbench、Contract、Money Flow 及三个 PostgreSQL 数据库完成同一登录双 Audience 验收：两应用同时 200，Money Flow 代表性主体写入和审计成功，两张应用凭证交叉使用均 401；Contract 完全停止时 Money Flow 继续 200、Contract 路由 503。目标应用页面发起统一注销后 IAM Session 401，随后重新登录成功。详细规格见 [M7-005 Money Flow IAM 迁移](./specs/money-flow-ledger/iam-adapter-m7-005.md)。

## M7-006 D1 组合交付验收

### D1 ECS 宿主机自动化验收

本地语法、确定性与幂等渲染检查：

```bash
make d1-host-bootstrap-test
```

已完成首次 SSH 指纹核对的目标主机可执行只读检查：

```bash
D1_SSH_TARGET=ecs-user@<ECS_PUBLIC_IP> \
D1_SSH_KEY=/absolute/path/to/private-key \
  deployments/d1/ops/bootstrap-remote-host check
```

`apply` 必须显式设置 `D1_CONFIRM_SSH_KEY_LOGIN=yes`，表示操作者已用第二个密钥会话验证不会把自己锁在主机外。2026-09-22 已实际通过脚本语法、自测、远程 `check`、关闭 APT 升级的重复 `apply` 和再次 `check`；17 项主机门禁全部通过。验证覆盖 Ubuntu／架构、免密 `sudo`、Docker／Compose、Docker 配置、Swap、swappiness、部署与备份暂存目录，以及 SSH 密码／键盘交互／root 登录关闭。公网 IP、私钥路径、AccessKey 和账号专属地址没有写入仓库。

### D1 私有镜像模式验收

```bash
make d1-image-mode-test
```

该门禁执行三个脚本的 shell 语法检查、D1 `build`／`pull` 双模式渲染、十个运行镜像摘要引用、已启用应用标签引用拒绝，以及 ACR 发布坐标／可变 `latest` 拒绝的自测。2026-09-26 加入 Project 后重新通过。真实验收还必须按 [ACR 私有镜像供应](./operations/acr-image-supply.md) 完成新版本的 ACR 推送和 ECS 拉取；旧九镜像实机证据不能替代新增 Project 的发布验证。

### D1 制品提升与发布包验收

```bash
make d1-image-promotion-test d1-release-bundle-test
```

制品提升自测覆盖十个摘要输入、目标 Registry／命名空间／版本约束、固定 `linux/amd64` 的 pull／tag／push、Image ID 一致性、目标平台检查和新摘要清单。发布包自测覆盖外层与包内 SHA-256、pull-only 门禁、十个摘要引用、运行时 Compose／五份 Capability Manifest／验收入口完整性，以及 `.env`、`.runtime`、证书、私钥和 AppleDouble 文件排除。

自动化证明发布契约能够脱离个人主机打包，不证明公司 Registry 已获得授权或目标 ECS 已部署。实际跨账号验证必须按 [D1 双环境制品提升与公网验证](./operations/d1-environment-promotion.md)登录源／目标 Registry、生成目标摘要清单、构建目标包，并在隔离公司环境重新执行公网、业务、端口、备份与恢复验收。

### D1 HTTP／HTTPS 入口模式验收

```bash
make d1-ingress-mode-test
```

该门禁生成一次性自签名证书，验证本地 HTTP 模式仍只发布回环 `18000 → 8080`；HTTPS 模式只由 Nginx 发布 `80 → 8080` 与 `443 → 8443`，证书和私钥按绝对路径只读挂载，私钥权限、公私钥匹配、固定域名跳转与 HTTPS 主机名都受校验。Docker daemon 和 `nginx:1.28-alpine` 本地镜像可用时，还会对两种渲染结果实际执行 `nginx -t`。2026-09-23 已在本机执行，两个 `nginx -t` 和全部门禁通过；这证明仓库配置，不替代 ECS 上真实证书与公网链路验收。

静态配置与当前运行组合：

```bash
docker compose \
  --env-file deployments/d1/.env.example \
  --file deployments/d1/compose.yaml \
  --profile contract --profile money config --quiet

deployments/d1/ops/d1 up
deployments/d1/ops/d1 check
deployments/d1/ops/grant-bootstrap-access
D1_ACCEPTANCE_DISRUPTIVE=yes make d1-delivery-acceptance
```

升级、备份和空目标恢复：

```bash
deployments/d1/ops/d1 backup
deployments/d1/ops/d1 upgrade
D1_CONFIRM_EMPTY_RESTORE=yes deployments/d1/ops/d1 restore BACKUP_ID
```

2026-09-21 已在独立 Compose 项目和 PostgreSQL 18.6 实际执行。结果覆盖：空卷初始化三个独立数据库／账号及 current migration；只有 Nginx 发布宿主端口；一次 IAM 登录分别进入两个应用并各完成 Party 写入；Contract 停止时 Money 继续 200；关闭 Money 后容器停止、路由 404、菜单消失且 Contract 继续 200；重新启用保留原数据；联合备份带外层 SHA-256；删除全部验收卷后恢复 IAM 私钥／账号／授权、Contract 数据＋Evidence 和 Money 数据；旧 Session 401、原账号重登、原数据回读及恢复后继续写入通过；`upgrade` 先备份再重放 migration 且数据保持。

验收脚本会写入代表性 Party，并在设置 `D1_ACCEPTANCE_DISRUPTIVE=yes` 时短暂停止 Contract，不能在未安排窗口的生产环境执行。恢复命令会删除指定 Compose 项目的卷，只有显式 `D1_CONFIRM_EMPTY_RESTORE=yes` 才执行。完整证据和限制见 [M7-006 D1 组合交付](./specs/delivery/d1-combined-delivery-m7-006.md)，操作者步骤见 [D1 运行手册](./operations/d1-compact.md)。

### D1 最小权限专项验收

```bash
D1_PROJECT_NAME=omsd1m7006 \
D1_PUBLIC_URL=http://127.0.0.1:18100 \
  make d1-authorization-acceptance
```

2026-09-26 已对当前源码重建后的完整 D1 组合实际执行。脚本新建并激活两个非管理员 Subject：合同只读用户只有 `contract-ledger.view`，主体登记员只有 `contract-ledger.view` 与 `contract-ledger.party.register`。实测结果为：

- 非管理员自授 IAM 权限返回 `403`，证明业务 Capability 不推导 IAM 管理权；
- 两份 Credential 只含各自显式 Grant、单一 Contract Audience、Tenant Scope 和不超过五分钟的 TTL；
- 只读用户可读 Party，直接写入返回 `403` 且记录不存在；
- 主体登记员可读并成功登记 Party；
- 两个用户请求未授权的 Money Audience 均返回 `403`；
- 停用账号后 IAM Session 和重新签发立即返回 `401`；
- 停用前已签发的 Contract Credential 在不超过五分钟的 TTL 残余窗口内仍能读取。

该脚本会留下两个测试账号和一个测试 Party，并停用只读测试账号；应在开发／验收数据库运行。当前证据只覆盖 Tenant Scope 动作级授权。Project Resource Scope、Business Fact Snapshot／Policy、单项撤权和 C1 身份超级管理员反向隔离已由后续专项补齐；C2 首组多角色／资源／状态／撤权矩阵已由后续专项补齐；跨租户、C2 剩余故障与 C3～C8 多角色矩阵仍需后续直接证据。

### D1 Project 资源权限专项验收

```bash
D1_PROJECT_NAME=omsd1resource \
D1_PUBLIC_URL=http://127.0.0.1:18110 \
  make d1-resource-authorization-acceptance
```

2026-09-26 已在启用 Project Workspace 的隔离 D1 中实际通过。脚本从 Nginx 唯一宿主入口执行以下正反例：管理员创建 Project A／B；新建非管理员 Subject；只授予 Project A 的 `project-workspace.read` 与 `project-workspace.update`；省略 scope 和请求 Project B 凭证均 `403`；Project A Resource Credential 的 audience、actions、唯一 Project ID 与最长五分钟 TTL 精确匹配；tenant 列表、Project B 读取／修改拒绝；Project A 读取／修改成功；被拒绝的 B 写入没有改变名称或 revision；`/project/` 页面和前缀内脚本可用。

随后用 `deployments/d1/ops/d1 backup` 生成包含 IAM、签名密钥、People、Project、两个台账和 Evidence 的联合备份，销毁该测试 Compose project 的数据卷并恢复。恢复脚本曾暴露“PostgreSQL 容器本机健康、跨容器 TCP 尚未就绪”的竞态，现已改为在 IAM 实际 `DATABASE_URL` 上等待 `pg_isready`。修复后 restore 的六套应用 schema 检查、资源权限专项和完整 D1 业务验收再次通过。

本专项只证明显式 Resource Grant 和目标应用最终授权，不证明 Business Fact Snapshot／Policy、单项 Grant 撤销、动态项目成员、跨租户或 C1～C8 多角色矩阵。可复验步骤和问题记录见[OA 第一阶段权限实验报告](./reports/oa-stage-1-authorization-experiment-20260926.md)。

### D1 Project Business Fact Policy 专项验收

在启用 Project 和 People 的隔离 D1 中执行：

```bash
D1_PROJECT_NAME=omsd1policy \
D1_ENV_FILE=deployments/d1/.env \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
  make d1-business-fact-authorization-acceptance
```

2026-09-26 已实际通过。脚本经 Nginx 和 Gateway 创建非管理员 Subject、关联 Person、Project A／B 与 Project A 的 `business_owner` Assignment。Gateway 只转交 Assignment source／ID／revision；IAM 用一分钟、精确 Project／Person scope 的服务凭证读取 Project Assignment 与 People Person 版本化事实，并由 `project-assignment-access@1.0.0` 签发 Project A 的 `read/update`。Project A 读取／修改成功，Project B 读取 `403`，目标应用继续执行最终授权。

随后脚本向同一 Subject 增加无关的 Project A `history.read` 直接 Grant：签发结果出现三项动作；只撤销该 Grant 后，新凭证恢复为策略产生的 `read/update`，账号和 Assignment 授权不受影响。结束 Assignment 后事实 revision 推进，旧 revision 的新凭证请求 `403`；结束前已经签发的凭证只在契约规定的最长五分钟 TTL 内继续有效。IAM 数据库中同时验证到策略签发与单项撤权审计。

该脚本会创建测试账号、Person、Project 和 Assignment，但不再注入静态 People Token，也不为换取凭证重建 Project Workspace。Project 使用独立工作负载私钥向 IAM 交换最长一分钟、单 action、精确 Person／Group Scope 的凭证；IAM 只持公钥，并拒绝请求替换与 Assertion 重放。脚本轮换工作负载密钥后验证新旧 Key ID 审计和 Project 容器 ID 不变；分别停止 IAM 与 People 时，新建 Assignment 返回 `503`、既有 Project 仍可读，恢复后无需重启 Project。C1 人类角色与身份超级管理员反向隔离由下节补齐；跨租户及 C2～C8 仍未证明。详细矩阵见[OA 第一阶段权限实验报告](./reports/oa-stage-1-authorization-experiment-20260926.md)。

### D1 C1 项目责任权限矩阵

在启用 IAM、People 和 Project 的独立 D1 中执行：

```bash
D1_PROJECT_NAME=omsd1c1 \
D1_ENV_FILE=deployments/d1/.env \
D1_PUBLIC_URL=http://127.0.0.1:18130 \
C1_EVIDENCE_DIR=deployments/d1/.runtime/c1-evidence \
  make d1-c1-authorization-acceptance
```

2026-09-26 已实际通过。`reference-oa-c1@1.0.0` Overlay 把身份超级管理员、OA 业务管理员、总经理、经营经理、生产经理和财务映射为 Project 自己发布的稳定 Capability；项目经营负责人、项目生产负责人和参与者则通过 `project-assignment-access@1.0.0` 从 Project Assignment 与 People Person revision 获得精确 Project A Scope。

脚本用两个 Project、九类 Subject 执行 116 个 HTTP 断言，0 个状态不匹配，最大本地请求时间 0.404434 秒。实测包括：

- 身份超级管理员可治理账号和 Grant，但撤销全部 Project Grant 后换证为 `403`；伪造身份 Header 仍被 Gateway 清洗并返回 Workbench 授权入口；
- OA 业务管理员依靠显式 tenant actions 创建 Project A／B、装配责任并完成立项；
- 经营经理只能管理 `business_owner`，生产经理只能管理 `production_owner`；跨角色建立／结束返回 `403` 且 Project revision 不变；
- 项目双负责人只读写 Project A，参与者只读；总经理与财务只读；Project B、错误 Fact／Scope 和相邻写动作全部拒绝；
- Project 进入 `active` 后，拥有 `update` 也不能绕过本地状态机，修改返回 `409` 且业务事实不变；
- Assignment 结束后旧 revision 不能签发新凭证，已签凭证只在最长五分钟残余窗口内继续；IAM 存在 Grant、Policy 签发和撤销审计。

本专项新增三项稳定 Assignment 角色能力和一份版本化 Customer Overlay Schema，没有新增数据库 migration、共享角色表、跨应用 ORM 或客户核心分支。它完成 C1 已实现“项目与责任”部分的权限子矩阵，不证明项目包／父子转换／取消后恢复、跨租户、临时代理、页面权限刷新、金额／字段／文件动作或 C2～C8。完整实验设计、异常与变化账见[OA 第一阶段权限实验报告](./reports/oa-stage-1-authorization-experiment-20260926.md)。

## M8 People & Groups 验收

独立代码与真实 PostgreSQL：

```bash
cd backend/services/people-groups
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
TEST_DATABASE_URL='postgres://people_groups:people_groups_dev@127.0.0.1:55435/people_groups?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./internal/peoplegroups
```

D1 运行、故障隔离、开关与联合恢复：

```bash
D1_PROJECT_NAME=omsd1m7006 deployments/d1/ops/d1 check
D1_PROJECT_NAME=omsd1m7006 acceptance/d1-delivery/verify-running
D1_PROJECT_NAME=omsd1m7006 D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
D1_PROJECT_NAME=omsd1m7006 deployments/d1/ops/d1 backup
D1_PROJECT_NAME=omsd1m7006 D1_CONFIRM_EMPTY_RESTORE=yes \
  deployments/d1/ops/d1 restore BACKUP_ID
```

2026-09-21 已实际通过。独立测试覆盖 IAM audience／action、HTTP `401/403`、Person／Group／Membership／Capability／Snapshot 领域规则、租户、幂等、乐观并发与数据库历史不可变；D1 覆盖单次登录、公开 API 写入、People 单侧停机、`ENABLE_PEOPLE` 关闭／重开且数据保留、只有 Nginx 暴露宿主端口，以及销毁测试栈数据卷后的四数据库联合恢复。恢复后原 `3 Person / 3 Group / 3 Membership / 3 Snapshot` 存在且可继续写入。

首次联合恢复演练暴露 IAM 公钥卷由 Compose 预创建后属主为 root 的问题；现已增加一次性 `public-keys-permissions` 任务，并从同一备份重新销毁卷恢复通过。完整范围见 [M8 规格](./specs/people-groups/m8-minimal-vertical-slice.md)。

## M9 至 M11 目标验证入口

M7 与 M8 已全部验证，M9-J1 Project Workspace、M9-J3 Design Operations 及本地 J1～J4 业务组合已通过，M10-001／M10-002／M10-003 已完成独立 Evidence Service 与 D1 装配闭环，真实 OSS 核心浏览器链也已通过。Project D1 入口、直接 Resource Scope、首条 Business Fact Policy、单项撤权、服务凭证生命周期、联合恢复与 C1 项目责任权限矩阵已补齐；Business Engagement 也已进入 D1，经多 audience Assignment Policy 和独立 Customer Overlay 通过 C2 首组 89 条权限／页面断言，并由实际浏览器证明 Workbench 换证、撤权和恢复后的动作收敛；扩展验收先以 95 条断言证明 409 无写入、Evidence 503 无部分写入及恢复后使用新命令单次成功，再以 110 条断言证明账号停用、IAM 502、Policy 事实源 503、失败不签发与恢复后单次签发，真实浏览器已复核恢复交互、Project 返回和撤权退出。本地联合备份、销毁卷后的空目标恢复和继续写入也已通过，同环境 C1 152 条回归仍通过。Design D1／公司 Ubuntu 完整入口总装、完整双租户 D1 矩阵、C1 剩余业务生命周期、C3～C8 多角色矩阵、Evidence 的 Contract 迁移、云端剩余负例与公网入口，以及 M11 Complete Suite 0.1／D2～D4 尚未实现。以下是后续必须补齐或持续回归的门槛；已经满足的入口、IAM、People、Project、Business Engagement C2、Design J3、本地 OA 与 Evidence 条目仍作为回归约束：

- 首个租户管理员、邀请、激活、登录、注销与密码重置形成可审计账号生命周期；匿名公共注册默认关闭；
- 未设置注册配置时仍拒绝匿名注册；登录时不存在的账号不会被静默自动创建；
- 正式 IAM、本地模板和测试提供者的同一 IAM 契约测试；
- Capability Manifest 在安装／升级时完成版本、兼容和未知能力校验；目标应用启动不依赖向 IAM 实时注册；
- 所有受保护端点具备授权声明，缺少上下文返回未认证、能力不足返回拒绝；
- 客户角色名变化不修改 Contract、Money Flow 或 Project 业务代码；
- 能力凭证绑定 issuer、tenant、audience、action、资源范围和有效期，伪造、越租户、越资源与过期请求均被拒绝；
- Business Fact Snapshot 保留来源、revision／有效时点和稳定引用；Project、People & Groups、Workbench、Gateway 或浏览器签发的伪造 Capability Credential 均被目标应用拒绝；
- 实际业务请求经 Gateway 直接到目标应用，不要求逐请求同步调用 Project 或 People & Groups；移除 Project 后，明确资源级或租户级直接授权仍能满足目标应用的独立闭环；
- IAM 内置 Policy Provider 与将来的外部 Provider 必须产生同形凭证；策略不可用时需要新事实的高风险授权失败关闭，已签发凭证只按明确 TTL 和撤销规则处理；
- `allowed_actions` 与实际命令授权使用同一能力语义；从响应中隐藏动作不能让无权限命令成功，拥有能力也不能绕过目标资源状态、revision 或领域不变量；
- 登录／注册经 Gateway 到达 IAM，但 Gateway 数据库不存在账号、密码、验证码或会话真相；IAM 才能签发、刷新和撤销凭证；
- Complete Suite 只暴露 Nginx 的 80／443；Gateway 和所有应用只监听本机或受限内网地址，绕过 Nginx 的公网访问失败；
- Nginx 完成 HTTPS、静态资源、基础反向代理和请求体约束，Gateway 独立完成 Suite 路由、身份 Header 清理、基础验签、限流与访问日志；两层职责均有失败与误配置测试；
- Gateway 删除客户端伪造的全部身份 Header；即使 Gateway 已通过基础验签，目标应用仍拒绝 audience、action、scope 或业务状态不匹配的请求；
- 认证等级由服务端路由清单声明，客户端不能通过把 URL 中的 `public`／`user`／`admin` 改名来选择授权级别；
- 签名密钥重启后可恢复、可轮换且旧凭证按明确宽限期验证；泄露和撤销路径有演练证据；
- People & Groups 只通过公开引用关联 IAM Subject，不读取 IAM 表、不保存密码／会话／角色授权；组内角色不会自动获得系统权限；本应用停机时 IAM 登录仍可工作；
- Person 可以属于多个 Group；Membership 加入、角色变化、移出和 Group 归档保留历史，旧 Roster Snapshot 不被当前名册改写；
- Group Type、Membership Role 和 Capability 可配置但不能改变历史语义；Purpose 可自由描述，具体 Project／Contract Assignment、预算和资金不写入 People & Groups；
- Project 只能通过公开 API 操作 Contract 与 Money Flow，任一目标应用停机时不产生虚假成功；
- Project 停机或未安装时，Contract 与 Money Flow 各自核心闭环继续可用；
- Evidence Service 可以脱离 Contract／Project 独立迁移和启动；业务应用代码与配置中不存在 OSS 长期 AccessKey、OSS SDK、分片上传实现或对象清理逻辑；
- 本地文件与阿里云 OSS Provider 遵守同一 File ID、Version、摘要、受控访问和恢复契约；切换 Provider 不改变业务引用含义；
- OSS 上传会话只能取得限定 Bucket、单一 Object Key、必要 multipart 动作和短 TTL 的权限；长期 AccessKey 不返回浏览器，越租户、越对象键、过期和重放均失败；
- 150 MB 代表性文件由浏览器 multipart 直传 OSS，Nginx、Gateway 和业务 API 均不承载文件字节；上传进度、取消、网络重试和幂等 complete 有直接证据；
- complete 在形成可引用 Version 前复核实际对象大小、服务端媒体类型和摘要；对象缺失、截断、替换、摘要不符或数据库提交失败时不产生虚假可用文件；
- 下载与预览必须先通过 Evidence 授权，再使用短期签名 URL 直接读取 OSS；过期 URL、越权 File ID、错误 disposition 与缓存跨账号复用均被拒绝；
- 过期、取消、崩溃和完成失败的上传会话及孤儿对象可扫描、重试和审计；清理不能删除已被有效 Version 引用的对象；
- Contract 当前本地 Evidence 迁移到独立服务后，File ID、Version、原字节、大小、SHA-256、业务引用、下载授权、备份恢复和历史审计保持一致；
- 有文件能力的单应用 release 能捆绑 Evidence Service 本地模式独立安装；没有文件能力的应用不被强制依赖 Evidence；
- Gateway、Workbench、进程和连接器对同一版本化 Suite Manifest 得出一致启用状态；IAM 不能通过配置关闭；
- 关闭应用后路由和菜单消失、进程可停止、相关连接器暂停，但数据库和文件不删除；重新启用通过版本、migration、健康和权限目录校验；
- 单应用开发不经过完整 Nginx／Gateway 仍可按独立方式启动和验证，不把入口装配变成业务代码依赖；
- Suite Manifest 与 Deployment Profile 使用独立配置：前者改变启用应用，后者改变节点放置、副本和数据设施；两者任意合法组合不生成客户专用代码分支；
- D1 在单台 ECS 上通过 Nginx → Gateway 统一访问各应用，一个 PostgreSQL 实例中的数据库、账号、migration 和备份恢复边界互不越界；
- 不启动 Redis、消息队列、服务注册中心或第二台服务器时，完整核心组合仍可运行；
- D2 在两台 ECS 上分开应用与 PostgreSQL；数据库无业务公网入口，安全组只允许应用节点私网访问，停止任一节点时系统可以中断但数据库不损坏；
- D3 在三台 ECS 上分开 Nginx／Gateway 边缘、内部应用和 PostgreSQL；公网只能到边缘节点，边缘只能到声明的应用端口，应用只能到自己的数据库和公开服务端点；
- D4 的负载均衡、至少两个应用副本和高可用数据层分别经过健康摘除与切换测试；停止任一应用节点后已有会话、幂等命令、上传会话和新请求按契约继续工作；
- D1～D3 的文档、监控和演练不得宣称高可用；故障测试必须证明实际中断范围与档位说明一致，并且恢复后无部分 migration、跨库写入或业务事实丢失；
- 同一 release image、API、migration 和 Suite Manifest 能在 D1～D4 目标档运行；Gateway、应用、数据库、Evidence 和健康检查端点全部由配置注入，自动检查业务代码不存在绑定部署档的 `localhost`、固定 IP 或机器名；
- 从 D1 向 D2／D3／D4 迁移并回退后，ID、revision、文件摘要、审计、业务引用、数据库账号边界和可恢复性保持不变，不复制运行中的 PostgreSQL 数据目录；
- 从创建企业与账号、建立人员与 Group、配置成员角色和能力、由项目引用 Group、创建合同、人工建立合同外义务、登记实际资金、核销到上传 Evidence 的 Complete Suite 演示闭环通过；
- 人工合同外义务和未分配资金不要求 Contract；未来报销或奖金来源不能直接写 Money Flow 数据库；
- 备份离开运行数据所在系统盘，并完成整机丢失后的恢复演练。

具体命令必须随各里程碑的实际实现加入本文件；设计文档和目录存在不构成验证通过。

### M9 J1 项目立项验收

普通质量门禁：

```bash
cd backend/services/project-workspace
go test ./...
go vet ./...
go build ./...
```

真实 PostgreSQL 18、HTTP、外部引用故障和并发幂等：

```bash
cd backend/services/project-workspace
docker compose up -d postgres
TEST_DATABASE_URL='postgres://project_workspace:project_workspace_dev@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -count=1 -tags=integration ./...
```

跨应用普通回归与独立 Docker 交付：

```bash
go test \
  ./backend/services/project-workspace/... \
  ./backend/services/people-groups/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/money-flow-ledger/... \
  ./backend/services/evidence-service/...

docker build -f backend/services/project-workspace/Dockerfile -t omsbizfoundry/project-workspace:j1-local .
cd backend/services/project-workspace
GOOS=linux GOARCH=amd64 ./ops/build-release v0.1.0 /absolute/temporary/output
```

2026-09-25 已实际通过。J1-A01～J1-A18 分别落到领域、HTTP、真实 PostgreSQL、外部引用和组合回归测试；覆盖 Project 草稿／激活／取消、Assignment 唯一负责人和替换留痕、类型化 Resource Link、租户／scope／action、乐观并发、同 command 并发、上游故障前后重放以及模板／正式 IAM 同形上下文。Linux AMD64 发布包包含 server、ops、八份 v1 API 契约、配置样例、备份恢复入口和校验和；数据库已真实导出为 PostgreSQL custom archive、恢复到新空库，并核对 migration、Project 与 History 数量一致。非 root Docker 容器连接独立数据库后 `/health/ready` 与 Bearer 创建 Project 冒烟通过。

这组证据只完成 J1 独立业务切片。后续切片已经补齐 Contract／Money／Evidence／Design 的 Project reference-read，但 Project 仍未进入 D1 Suite Manifest，参考 OA Demo 也还没有完成同一数据集的可操作页面。自动化用例编号、输入、故障和期望证据见 [Project Workspace：项目立项](../arckit/spec/project-workspace/project-initiation.md#j1-可执行验收矩阵)。

### M9 J3 设计生产与交付验收（2026-09-25 已通过）

J3 的稳定边界、四个模型、十个 v1 API 与 J3-A01～J3-A20 固定在 [Design Operations：设计生产与交付](../arckit/spec/design-operations/design-production-delivery.md#j3-可执行验收矩阵)。独立运行时、Project／Assignment／Evidence reference-read 和相应直接证据均已落地。

普通质量门禁：

```bash
cd backend/services/design-operations
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

真实 PostgreSQL 18、HTTP、并发与故障边界：

```bash
cd backend/services/design-operations
docker compose up -d --wait
TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./...
TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration \
  ./internal/design ./internal/httpserver
```

依赖 reference-read 与全仓普通回归：

```bash
cd backend/services/project-workspace
TEST_DATABASE_URL='postgres://project_workspace:project_workspace_dev@127.0.0.1:55437/project_workspace?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration ./...

cd ../..
go test \
  ./acceptance/access-contract/... \
  ./acceptance/combined-delivery/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/design-operations/... \
  ./backend/services/evidence-service/... \
  ./backend/platform/gateway/... \
  ./backend/platform/identity-access/... \
  ./backend/services/money-flow-ledger/... \
  ./backend/services/people-groups/... \
  ./backend/services/project-workspace/... \
  ./frontend/oms-workbench/...
```

独立恢复与制品：

```bash
cd backend/services/design-operations
./ops/verify-backup-restore
docker build -t omsbizfoundry/design-operations:local .
GOOS=linux GOARCH=amd64 ./ops/build-release v0.1.0-local.1 /absolute/temporary/output
```

2026-09-25 已实际通过：

- `internal/design/postgres_repository_integration_test.go` 和 `internal/httpserver/system_integration_test.go` 覆盖创建、唯一性、负责人、成果、四阶段推进、适用性、替换、回退、最终完成、取消、幂等、乐观并发、外部故障和历史不可变；
- `internal/httpserver/server_test.go`、`internal/auth/auth_test.go` 和 `internal/references/http_test.go` 覆盖 401／403、scope、身份 Header 清理、模板／正式 IAM 同形、错误 audience、重复 capability、不安全 issuer 及引用错误分类；
- Project 真库测试覆盖 Project／Assignment 最小投影；Evidence HTTP 测试证明 File Version reference 不泄露 bucket、Object Key、STS 或签名 URL；
- PostgreSQL custom archive 已恢复到新空库并核对 migration、Work Item、Stage、Deliverable 和 History；Linux AMD64 release 摘要和固定文件清单通过；
- Docker image 构建通过，运行用户为非 root，`MIGRATION_MODE=verify` 连接独立数据库后 `/health/ready` 返回 200。

以上检查本身只证明 J3 独立业务闭环；J2／J4 与本地参考 OA 的组合证据由后面的独立验收提供。Project D1 已由后续权限专项补齐，但本检查仍不证明 Design D1 或公司私有 Ubuntu 总装完成。

### M9 J2／J4 稳定引用基础验收（2026-09-25 已通过）

目标是让 Project Workspace 能以公开契约验证已有 Contract、Obligation 和 Transaction，而不读取目标数据库或复制金额等业务事实。实际执行：

```bash
cd backend/services/contract-ledger
GOWORK=off go test ./internal/config ./internal/httpserver

cd ../money-flow-ledger
GOWORK=off go test ./internal/config ./internal/httpserver

cd ../..
make fmt-check
go test \
  ./acceptance/access-contract/... \
  ./acceptance/combined-delivery/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/design-operations/... \
  ./backend/services/evidence-service/... \
  ./backend/platform/gateway/... \
  ./backend/platform/identity-access/... \
  ./backend/services/money-flow-ledger/... \
  ./backend/services/people-groups/... \
  ./backend/services/project-workspace/... \
  ./frontend/oms-workbench/...
go vet \
  ./acceptance/access-contract/... \
  ./acceptance/combined-delivery/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/design-operations/... \
  ./backend/services/evidence-service/... \
  ./backend/platform/gateway/... \
  ./backend/platform/identity-access/... \
  ./backend/services/money-flow-ledger/... \
  ./backend/services/people-groups/... \
  ./backend/services/project-workspace/... \
  ./frontend/oms-workbench/...
```

结果：

- Contract reference 返回 `contract` 的 ID、credential tenant、状态与 draft／contract revision；Money reference 分别返回 `obligation` 和不可变 `transaction` 的最小投影；
- 24+ 字节本地引用 Token 必须与 tenant 成对配置，示例占位符、半配置和错误 Token 被拒绝；正式 IAM 使用目标 audience 的 `*.view`，不接受静态引用凭证配置；
- HTTP 测试覆盖本地模板、正式 IAM、401、404、revision 和响应字段边界；Project 已有 validator 测试继续覆盖目标类型、tenant、status、revision、不可用和无效引用分类；
- 24 份 `arckit/tech/_shared/contracts/*.yaml` 解析成功；Linux AMD64 `project-workspace_v0.1.0-local.3` 实际 release 包含 Contract、Money Obligation／Transaction、Evidence File Version 和 Design Work Item 五份依赖契约；
- 全仓普通测试、vet 和格式门禁通过。

这组检查本身完成 J2／J4 的稳定引用基础；人工义务确认、收款／核销、同一假数据 Project Link 和单服务停机由下一节组合验收提供。

### M9 本地参考 OA Level B 高保真组合验收（2026-09-25 已通过）

启动、重复业务旅程、故障隔离和最终健康检查：

```bash
./deployments/local-oa/ops/local-oa up
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa failure-isolation
./deployments/local-oa/ops/local-oa verify
./deployments/local-oa/ops/local-oa status
```

静态与受影响应用回归：

```bash
sh -n deployments/local-oa/ops/local-oa
(cd acceptance/reference-oa-demo && GOWORK=off go test ./...)
(cd backend/services/design-operations && GOWORK=off go test ./...)
(cd backend/services/project-workspace && GOWORK=off go test ./...)
docker compose --env-file deployments/local-oa/.env \
  -f deployments/local-oa/compose.yaml config --quiet
```

实际结果：

- 六个应用和六个 PostgreSQL service 均通过 Compose health check；宿主机只在 `127.0.0.1:18081,18083-18087` 暴露本地验证端口；
- J1 建立两名人员、设计生产组、双负责人和 People revision 4／8 不可变名册快照；人员资料、IAM Subject、组内角色和任职状态变化后，Project Assignment 仍保留原 Person revision 1／Group revision 2 快照；
- J2 形成 Contract `closed / revision 4` 的补充、更正、终止、阶段／最终结算历史；责任人以 Contract revision 4 明确确认 Money Obligation，调额至 `closed / revision 2 / 125,000 CNY`，两笔有效回款合计 70,000 CNY，错误核销有反向与替代记录，55,000 CNY 减免后关闭，另有误录资金冲销；
- J3 建立三个 Evidence File Version 1，方案成果被替换，初设从不适用恢复并补交，施工图完成后因审查意见重开返工，Design 最终为 `completed / revision 18` 且 18 条历史可核对；J4 把 Contract revision 4、Obligation revision 2、Transaction revision 1、Evidence revision 1 和 Design revision 18 五类稳定引用挂回 Project `active / revision 9`；
- 完全删除本组合容器卷和续跑状态后第一次执行通过；第二次执行返回相同的 Project、Contract、Obligation、Receipt、Allocation、Evidence 和 Design ID，没有生成第二套业务事实；所有写操作由公开接口和各应用鉴权处理，没有数据库编辑或鉴权关闭；
- 本次 clean run 的代表性稳定 ID 为 Project `94d5c6a8-45a3-4ca5-bd3c-30f83bbc3d62`、Contract `50254d20-a262-4602-ab79-03dda2867427`、Obligation `00bb8574-e22b-4bf6-bd57-b346cb05e358`、Design `36a88298-1424-4740-b42b-b826289a3c6c`、首个 Evidence File `aa7e1658-efdd-449c-9275-b8cadc3364d9`；两笔收款、原错误核销、替代核销和误录资金冲销也各自保留独立稳定 ID；
- `failure-isolation` 依次停止 Contract、Money、People、Evidence 和 Design。对应不可用 Resource Link 数量为 1、2、0、1、1；每轮 Project 均保持 `active / revision 9`、两个 Assignment 快照可读，未停止服务继续 ready，停止项恢复 healthy 后才进入下一轮；
- 组合首次暴露 Design reference 缺少统一 `id` 的真实契约不兼容；修复只追加 `id` 并保留 `work_item_id`，Design／Project 回归和重新构建后的完整 Demo 通过。

这些结果证明首个参考 OA 的 Level B 高保真本地业务架构、历史纠错、幂等与五来源局部故障行为。当前高保真可移植开发包由后文独立 clean-room 证据证明；本节本身不证明定制化实际变更、统一视觉演示、Nginx／Gateway／中心 IAM／Workbench、TLS、公网、真实公司 Ubuntu、生产容量或真实客户适配已经通过。

### M9 首次实际组合变化的定制化六层验收（2026-09-25 已通过）

静态、兼容与既有运行证据：

```bash
(cd acceptance/reference-oa-demo && GOWORK=off go test ./...)
(cd acceptance/reference-oa-demo && GOWORK=off go vet ./...)
(cd backend/services/design-operations && GOWORK=off go test ./...)
(cd backend/services/design-operations && GOWORK=off go vet ./...)
git branch --show-current
```

实际检查结果：

- 从最小 J1～J4 到高保真 J1～J4 的变化已在 `docs/specs/assembly/product-boundary-validation.md` 建立逐项账目：People 字典和角色属于配置，`design-v1` 属于类型化扩展，六应用与五类 Resource Link 属于客户装配；
- Contract revision 4 到 Money Obligation 仍由责任人明确确认。参考 OA 与现有 OMS 的触发规则冲突，故本次没有新增自动连接器；
- 管理费、开票、奖金／工资、项目包和通用审批保持 `needs-business-evidence` 或 `deferred`，没有为页面完整度增加万能字段、万能流程或客户条件分支；
- 当前分支为 `main`，长期客户核心分支、客户专属数据库、共享业务表和本增量新增业务 migration 均为 `0`；六个 Level A 基线保持 `6/6`，Level B 来源故障隔离保持 `5/5`；
- Design reference-read 的通用 `id` 是加法兼容字段，旧 `work_item_id` 仍在；Design 系统集成测试同时断言二者；
- `TestReadStateAcceptsPreHighFidelityState` 证明只有最小组合字段的旧演示状态可被当前驱动读取，已有 Project、Contract、Obligation、Transaction、Allocation 和 Evidence 身份不丢失，新增高保真步骤不会被误判为已完成；
- 本结果验证首个参考 OA 的不分叉定制方式和策略放置门槛，不等于真实策略执行、第二客户适配、当前高保真 Ubuntu 包、公司 Ubuntu 或完整入口已经通过。

### Demo Validation 2 当前证据边界（2026-09-25）

[Demo Validation 2](../arckit/spec/assembly/demo-validation-2.md) 把业务验证拆为六个 Level A 独立 Demo 和一个 Level B 参考 OA 组合。当前证据边界按下表解释：

| 应用／组合 | 已有直接证据 | 尚未取得、不得标为通过 |
| --- | --- | --- |
| Contract Ledger | Level A 独立 Demo 已从空库通过真实页面、PostgreSQL、revision 1～4、第二正式合同、合同关系、遗留本地 Evidence v1／v2、阶段／最终结算、操作性关闭、鉴权负例、查询、审计、ZIP 导出、reference-read、重放和清理；既有发布、备份恢复证据继续有效 | 正式 IAM／完整入口与本地 Evidence 向独立 Evidence Service 的迁移属于其他验收轴，不阻塞本地独立 Demo |
| Money Flow Ledger | Level A 独立 Demo 已从空库通过真实页面、PostgreSQL、义务确认与调额、现金排期、两笔收款与多笔有效核销、Allocation Reversal 与替代核销、内部转账、减免、关闭、错误资金冲销、四时点历史余额、账龄排除、经营汇总、ZIP 导出、reference-read、负例、重放和清理；既有发布、备份恢复证据继续有效 | 无；坏账、票据、真实退款与通用导入已按既有业务边界审计延期 |
| People & Groups | Level A 独立 Demo 已从空库通过模板登录、真实 PostgreSQL、配置目录、两名 Person、资料修改、外部 IAM Subject 绑定、人员／目录停用、Group、Membership 加入／换角／结束、Capability 分配／结束、两份不可变 Snapshot、归档、inactive／鉴权／revision 负例、重放和清理 | 无；正式 IAM 与完整入口属于其他验收轴，不阻塞本地独立 Demo |
| Evidence Service | Level A 独立 Demo 已从空卷两次通过 Create、Local PUT、Complete、File／Version 引用、Access Grant、完整／Range 下载、鉴权与签名负例、幂等冲突、Abort 和真实对象清理；既有 OSS、发布、备份恢复证据继续有效 | 正式 IAM、完整入口、Contract 迁移与云端剩余负例属于其他验收轴，不阻塞本地独立 Demo |
| Project Workspace | Level A 独立 Demo 已从空库连续两次通过 draft／active／cancelled revision 1～9、缺口立项、双负责人、Contract／Evidence Link、公开引用、历史、引用不存在／不可用、鉴权／幂等／并发负例和专属卷清理；既有 J1-A01～A18 继续有效 | 真实相邻应用组合由 Level B 证明，不由契约桩冒充 |
| Design Operations | Level A 独立 Demo 已从空库连续两次通过 Project／Assignment／Evidence 引用三态、负责人失效恢复、四阶段、跳过／恢复适用、级联回退、成果替换、完成／重开／取消、公开引用、revision 1～21 历史和鉴权／幂等／终态负例；既有 J3-A01～A20、发布和恢复证据继续有效 | 真实相邻应用组合由 Level B 证明，不由契约桩冒充 |
| 参考 OA | Level A 六项全通过；Level B 从空库通过高保真 J1～J4、立即幂等重放、People／Contract／Money／Design 历史纠错和五来源逐项停机隔离；首次实际组合变化已通过六层账目、无客户核心分支、加法 API 与旧状态读取兼容检查；`v0.1.0-demo.11` 已从同一无源码归档通过六个独立产品、两类子集及全量组合的 clean-room 复演，只读 Ubuntu 预检已通过正反例自测 | 正式 clean-HEAD 发布、真实策略执行、第二客户场景、完整入口、真实公司 Ubuntu 与真实客户验收 |

#### 目标级当前状态审计（2026-09-25）

本轮从当前工作树重新执行六应用统一门禁，并对仍在运行的本地参考 OA 再执行 `verify → demo → demo → failure-isolation → verify → status`。Level A 的运行报告为六项 `passed / fresh-state,run,replay,cleanup`，能力清单为六项 `complete`，最终输出 `LEVEL_A_STATUS=passed`。Level B 两次执行保持同一组业务身份和 revision：Project `94d5c6a8-45a3-4ca5-bd3c-30f83bbc3d62` revision 9、Contract `50254d20-a262-4602-ab79-03dda2867427` revision 4、Obligation `00bb8574-e22b-4bf6-bd57-b346cb05e358` revision 2、Evidence `aa7e1658-efdd-449c-9275-b8cadc3364d9/v1`、Design `36a88298-1424-4740-b42b-b826289a3c6c` revision 18；Contract、Money、People、Evidence、Design 五个来源逐项停机时，Project 始终保持 `active` revision 9，两个 Assignment 快照保持可读，最终十二个容器全部健康。

| 目标维度 | 当前判定 | 证据含义 | 仍不得声称 |
| --- | --- | --- | --- |
| 六个小系统独立运行 | `achieved` | 六套最小环境均从空状态运行、重放并清理；Contract、Money、People、Evidence 不依赖其他业务应用，Project 与 Design 只依赖公开 reference-read 契约桩 | 不代表正式 IAM、统一入口或云资源已参与 |
| 约定业务能力完整性 | `achieved` | Demo Validation 2 能力矩阵的六个能力簇全部为 `complete`，每套均有主闭环、历史、失败关闭和结果断言 | 不代表通用 OA 的所有可能功能；已延期能力仍按现有边界处理 |
| 数据与故障解耦 | `achieved` | 六应用各写自己的 PostgreSQL／对象目录；Level B 五来源逐项停机只使对应实时引用降级，Project 与快照继续可用；设计交付子集和商业台账子集分别在未选应用及其数据库真实缺席时完成组合旅程 | 不代表任意未来扩展天然解耦；每次新增旅程仍须复验 |
| 首个参考 OA 高保真组合 | `achieved` | J1～J4、历史纠错、立即幂等重放、五来源故障隔离和最终健康检查均再次通过 | 不代表逐页面复刻旧 OA、统一视觉完成或真实客户验收 |
| 六层定制方法 | `achieved-for-reference-product` | 首次实际组合变化已按稳定内核、配置、策略、类型化扩展、客户装配、客户专项唯一归类，并由 Customer Scenario Evidence v1 失败关闭越界声明 | 只有一个参考产品，不能据此宣称跨客户平衡点已经成立 |
| 无源码换机候选 | `achieved-as-development-artifact` | `v0.1.0-demo.11` Linux/amd64 包已通过摘要、九镜像离线装载、六个独立产品、两个可裁剪子集、空状态全量启动、两次旅程、五来源隔离与零残留清理；包内只读 Ubuntu 预检已通过正反例自测 | `commit=unknown` 的 development provenance 不是正式发布，也不是公司 Ubuntu 实机证据 |
| Nginx／Gateway／中心 IAM 完整入口 | `separate-not-evaluated` | 本阶段有意先验证业务架构；完整入口仍遵守既有架构基线 | 不得用本地模板 IAM 或宿主机端口冒充完整入口通过 |
| 第二真实客户适配 | `missing-external-evidence` | 仓库已提供严格场景证据契约和不冒充通过的草稿模板 | 在取得第二客户脱敏事实、客户确认和验收证据前，不得宣称方法具有跨客户普适性 |

审计结论：目标级审计确认六个单应用与全量参考 OA 均已达到当前通过口径。随后对“客户装配可选应用”作更细审计时，发现单应用与全量六应用之间缺少直接组合证据；下述设计交付与商业台账两个方向相反的子集已经补齐这一 Agent-owned 缺口。下一项产品证伪仍必须由第二个独立真实客户的脱敏事实触发；下一项环境验收必须先取得公司私有 Ubuntu 的明确授权和资源输入。获得输入后，Agent 仍负责执行场景验证或目标环境验收，但不得在输入到来前编造业务差异或提前修改稳定内核。

#### 设计交付子集装配验收（2026-09-25 已通过）

执行：

```bash
deployments/local-oa/ops/local-oa composition-audit
```

该门禁使用隔离 Compose project 和 `19084`～`19087` 端口，从空卷只启动 `people-groups`、`evidence-service`、`project-workspace`、`design-operations` 及各自数据库。运行 service 集合严格等于这 8 项；`contract-db`、`contract-ledger`、`money-db`、`money-flow-ledger` 均未运行。随后用模板 IAM 和真实公开 API 连续两次执行 J1／J3／J4 子集：Project `106be041-e1b9-4192-ad12-1d09d11fd9b9` 保持 `active / revision 6`，Evidence `37af5432-e034-426f-801f-d10ea3ddc046/v1`、Design `9059f12d-56e2-4c37-9807-6fdcee830675 / completed / revision 18` 均保持同一身份和 revision，状态文件 SHA-256 两次均为 `0f78ceb85c490f0e9d497debef981ccd133e9ef336194d315e4f364b746fbf24`。

命令成功后自动销毁隔离容器、网络、数据库卷、Evidence 对象卷和续跑状态；原十二容器全量参考 OA 随后检查仍全部 healthy。验收驱动 `GOWORK=off go test ./...`、脚本 `sh -n` 和 Compose config 同轮通过。

该结果证明一套真实应用可以在“单应用”和“全量 OA”之间按选装边界组成业务闭环，也证明 Project 的未选 Contract／Money 端点不会成为运行前提。它是合成的客户装配探针，不是第二真实客户证据；`v0.1.0-demo.11` 已从无源码归档以同一门禁重放，仍不代替真实 Ubuntu 主机验收。

#### 商业台账子集装配验收（2026-09-25 已通过）

执行：

```bash
deployments/local-oa/ops/local-oa composition-audit-commercial
```

该门禁使用隔离 Compose project 和 `19081`、`19083` 端口，从空卷只启动 `contract-ledger`、`money-flow-ledger` 及各自数据库。运行 service 集合严格等于这 4 项；People、Evidence Service、Project Workspace、Design Operations 及其数据库均未运行。随后通过 Contract／Money 的真实本地账号、会话、CSRF 和公开接口连续两次执行 J2：Contract `592c77bf-02e8-495d-9894-f441899b499c` 保持 `closed / revision 4`，Money Obligation `b4c5f6d0-52c0-4a1e-9e4f-e0d16cd0661e` 保持 `closed / revision 2`，两笔收款、错误核销反向、替代核销、减免、关闭和错误资金冲销事实均保持相同身份，状态文件 SHA-256 两次均为 `7adc2d81b504db79d19029bbc61ea46f8bee1686fbd2337e2d4cfb96a1fcb262`。

命令成功后自动销毁隔离容器、网络、两个数据库卷和续跑状态，检查结果为零残留；原十二容器全量参考 OA 随后仍通过六应用 readiness 检查。验收驱动 `GOWORK=off go test ./...`、`GOWORK=off go vet ./...`、脚本 `sh -n` 和 Compose config 同轮通过。

该结果从与设计交付子集相反的方向证明：合同与资金闭环不要求项目、人员、文件或设计应用作为启动前提，客户装配可选择商业台账能力而不复制相邻事实。它仍是合成探针，不是第二真实客户证据；`v0.1.0-demo.11` 已从无源码归档以同一门禁重放，仍不代替真实 Ubuntu 主机验收。

因此，现有单元／集成／发布证据可以复用为各独立 Demo 的底层门禁，但不能替代新的操作者 Demo。每个 Level A 项完成后必须在本节追加实际命令、fresh-state 结果、稳定 ID／revision、重放结果、适用负例和清理结果。

#### Contract Ledger Level A 独立 Demo（2026-09-25 已通过）

静态、应用级和 fresh-state 验收：

```bash
make contract-ledger-demo-static-check
go test ./acceptance/contract-ledger-demo/...
go test ./backend/services/contract-ledger/...
go vet ./backend/services/contract-ledger/... ./acceptance/contract-ledger-demo/...

./deployments/demos/contract-ledger/ops/demo reset --confirm-destroy-data
./deployments/demos/contract-ledger/ops/demo run
./deployments/demos/contract-ledger/ops/demo replay
./deployments/demos/contract-ledger/ops/demo reset --confirm-destroy-data
```

实际结果：

- Compose service 集合严格等于 `contract-ledger + db`，未启动 Nginx、Gateway、中心 IAM、Workbench 或相邻业务应用；数据库只在内部网络，应用端口只绑定 `127.0.0.1:28081`；
- fresh-state migration 后建立主合同 `7af6396e-f19c-4f85-ad37-29ee60f35220` 和第二份正式合同；主合同依次得到激活、补充调额、更正和终止，登记阶段／最终结算后操作性关闭，最终为 `closed / revision 4 / 125000.00 CNY`；
- 合同关系、遗留本地 Evidence v1／v2 及最新版原始字节、九类关键审计事件、受保护 ZIP 账簿导出和版本化 reference-read 自动核对通过；
- 未认证访问被重定向、错误密码与错误 reference token 被拒绝、终止后普通变更及关闭后新结算返回状态冲突；
- 原补充协议、最终结算和关闭命令重放及完整驱动第二次运行都保持同一合同和 `revision 4`，没有产生重复业务事实；
- reset 后专属数据库卷、Evidence 卷和 `.runtime` 状态均不存在；Contract 普通测试、`go vet`、验收驱动测试、脚本语法与 Compose 配置同时通过；
- Demo Validation 2 规定的 Contract 能力簇已全部进入同一操作者故事。本地 Evidence 只是当前遗留实现的如实演示，迁移到独立 Evidence Service 仍由单独事项验证。

操作者说明：[Contract Ledger 独立 Demo](../deployments/demos/contract-ledger/README.md)

#### Money Flow Ledger Level A 独立 Demo（2026-09-25 已通过）

静态、应用级和 fresh-state 验收：

```bash
make money-flow-demo-static-check
go test ./acceptance/money-flow-demo/...
go test ./backend/services/money-flow-ledger/...
go vet ./backend/services/money-flow-ledger/... ./acceptance/money-flow-demo/...

./deployments/demos/money-flow-ledger/ops/demo reset --confirm-destroy-data
./deployments/demos/money-flow-ledger/ops/demo run
./deployments/demos/money-flow-ledger/ops/demo replay
./deployments/demos/money-flow-ledger/ops/demo reset --confirm-destroy-data
```

实际结果：

- Compose service 集合严格等于 `money-flow-ledger + db`，未启动 Nginx、Gateway、中心 IAM、Workbench 或相邻业务应用；数据库只在内部网络，应用端口只绑定 `127.0.0.1:28083`；
- fresh-state migration 后建立义务 `1a91ecc2-cc9c-4281-8c73-f20ff1d15838`，从确认 `revision 1 / 120000 CNY` 调额到 `revision 2 / 125000 CNY`，并记录 `2026-10-20` 预计回款日；
- 首笔收款 `6ade6539-da33-48b7-949e-209f3c6a96fe / 40000 CNY` 的初始 Allocation 被完整反向，再建立正确替代 Allocation；第二笔收款 `7c65a8b3-64e0-4fad-9112-fc9129daf222 / 30000 CNY` 形成第二条有效 Allocation，最终净核销 70,000 元；
- 内部转账 `fa3919e5-7cc1-4a3d-91bd-85990e9f01bb / 5000 CNY` 独立保存且不进入义务核销；55,000 元减免后义务归零并关闭；初始核销、反向、两条有效核销和关闭四个时点的历史余额均按系统记录时间重放通过；
- 另建 `1000 CNY` 误录收款并完整冲销；关闭义务排除出当前账龄，现金排期历史、经营汇总、受保护 ZIP 台账导出和两个版本化 reference-read 均通过；
- 未认证访问被重定向、错误密码和错误 reference token 被拒绝、关闭后调额与有 active Allocation 的主收款直接冲销返回状态冲突；
- 原调额命令重放及完整驱动第二次运行都保持同一义务 `closed / revision 2`，没有产生重复 revision 或第二套业务事实；
- reset 只删除本 Demo 的数据库卷和 `.runtime` 状态；Money 普通测试、`go vet`、验收驱动测试、Shell 语法与 Compose 配置同时通过；
- clean run 与 replay 均保持上述义务、收款、Allocation 和内部转账身份不变；Demo Validation 2 规定的 Money 能力簇已全部进入同一操作者故事。坏账、票据、真实退款与通用导入继续按既有边界审计结论延期。

操作者说明：[Money Flow Ledger 独立 Demo](../deployments/demos/money-flow-ledger/README.md)

#### People & Groups Level A 独立 Demo（2026-09-25 已通过）

静态、应用级和 fresh-state 验收：

```bash
make people-groups-demo-static-check
go test ./acceptance/people-groups-demo/...
go test ./backend/services/people-groups/...
go vet ./backend/services/people-groups/... ./acceptance/people-groups-demo/...

./deployments/demos/people-groups/ops/demo reset --confirm-destroy-data
./deployments/demos/people-groups/ops/demo run
./deployments/demos/people-groups/ops/demo replay
./deployments/demos/people-groups/ops/demo reset --confirm-destroy-data
```

实际结果：

- Compose service 集合严格等于 `people-groups + db`，未启动 Nginx、Gateway、中心 IAM、Workbench 或相邻业务应用；数据库只在内部网络，应用端口只绑定 `127.0.0.1:28084`；
- fresh-state migration 后建立 Group `3988d0fe-408f-4de5-bf0e-0866a577a58f`、Alice `4f02ccf8-43c1-4af5-994f-11cc45e8e31d`、Bob `294ca777-c92b-4e29-935b-de14ec5d1628`、两种 Membership Role 和一项 Group Capability；完整生命周期把 Group 推进至 `archived / revision 11`；
- revision 4 Snapshot 保存两名 active 设计师和一项 active 设计交付能力；Alice 资料修改和 IAM Subject 绑定分别推进 Person 与活动 Group revision，revision 9 Snapshot 只保存 `person revision 3` 的主创建筑师且无 active 能力；旧 Snapshot 的 payload 与 SHA-256 逐字节保持不变，快照不包含 IAM Subject；
- Bob 结束 Membership 后停用为 `inactive / revision 2`，重新加入被拒绝；`design_delivery` Definition 停用为 `inactive / revision 2`，新的 Capability Assignment 被拒绝，旧 Membership／Assignment 历史保持可读；
- 未认证访问被重定向，错误模板令牌被拒绝，过期 Group revision、inactive Person／Definition 与归档后成员写入返回冲突；原命令完整重放后 ID、最终 revision、历史和两份 Snapshot 不变；
- 首次 fresh-state 实跑发现模板登录先校验 CSRF、后解析表单，导致有效浏览器登录恒为 403；修复后增加服务端回归测试，空卷首跑和完整重放均实际通过；
- reset 只删除 People Demo 专属数据库卷；People 普通测试、`go vet`、验收驱动测试、Shell 语法与 Compose 配置同时通过；
- clean run 与 replay 均保持上述 Group、Person 和 Snapshot 身份不变；Demo Validation 2 规定的 People 能力簇已全部进入同一操作者故事，IAM Subject 仍只是外部 UUID 引用，不合并 IAM 凭证所有权。

操作者说明：[People & Groups 独立 Demo](../deployments/demos/people-groups/README.md)

#### Evidence Service Level A 独立 Demo（2026-09-25 已通过）

静态与模块检查：

```bash
cd acceptance/evidence-service-demo
GOWORK=off go test ./...
GOWORK=off go vet ./...
cd ../..
make fmt-check evidence-service-demo-static-check
docker compose --env-file deployments/demos/evidence-service/.env \
  -f deployments/demos/evidence-service/compose.yaml config --quiet
```

真实容器首次运行与重复运行：

```bash
deployments/demos/evidence-service/ops/demo run
deployments/demos/evidence-service/ops/demo replay
```

两次均只启动 `db` 与 `evidence-service`，readiness 明确返回 `storage_provider=local`。每次通过匿名／错误令牌拒绝、创建会话、签名 PUT 防篡改、对象不可覆盖、服务端大小／媒体类型／SHA-256 确认、完成幂等、File／Version 引用、完整下载、Range 下载、下载授权防篡改、摘要不匹配失败、Abort 与 Abort 重放；随后 `/app/ops cleanup` 都返回：

```json
{"examined":2,"cleaned":2,"failed":0,"protected":0}
```

首次完成的 File ID 为 `42b9db10-467a-4dbf-ae0a-9b0ba022de46`，重复运行完成的 File ID 为 `f0f70f91-c45b-4072-8f22-3663beeca316`，均为 Version 1。不同 File ID 是每次演示使用新命名场景的显式结果；同一轮中的创建、完成和取消命令仍分别验证了幂等重放。完成对象未进入清理候选，失败完成和已取消对象被实际删除。

操作者说明：[Evidence Service 独立 Demo](../deployments/demos/evidence-service/README.md)

#### Project Workspace Level A 独立 Demo（2026-09-25 已通过）

静态、应用级和 fresh-state 验收：

```bash
cd acceptance/project-workspace-demo
GOWORK=off go test ./...
GOWORK=off go vet ./...
cd ../../backend/services/project-workspace
GOWORK=off go test ./...
GOWORK=off go vet ./...

deployments/demos/project-workspace/ops/demo run
deployments/demos/project-workspace/ops/demo replay
deployments/demos/project-workspace/ops/demo reset --confirm-destroy-data
```

实际结果：

- Compose service 集合严格等于 `db + project-workspace + reference-stub`；stub 只实现版本化 Person／Group／Contract／Evidence reference-read，数据库与 stub 都不暴露宿主机端口，未启动真实相邻业务应用、Nginx、Gateway、中心 IAM 或 Workbench；
- fresh-state Project `8717295b-d0cc-4515-a397-e931d413241b` 从 draft revision 1 更新后，取得经营负责人和生产负责人，完成立项为 active revision 5，建立两条 Resource Link、解除 Evidence Link并取消，最终为 `cancelled / revision 9`；
- 第二次运行创建独立命名场景 Project `6e7dbe9a-0775-45c2-9a71-d8168bfe5054`，同样得到 `cancelled / revision 9 / history_events 9`；每一轮内部的创建、Assignment 和 Resource Link 都实际重放并保持稳定 ID；
- 引用不存在时返回 422、引用暂不可用时返回 503，失败前后 Project revision 与 Assignment／Link 数量一致；同一 Person／Contract 首次可用、随后不可用，进一步证明幂等重放不重新请求上游，而查询路径把不稳定引用明确标为 unavailable；
- 匿名与错误模板令牌、同 Command ID 异载荷、过期 revision、终态写入均被拒绝；Project／Assignment reference-read 与 revision 1～9 历史核对通过，服务端 Actor 未采用伪造客户端 Header；
- reset 后 `omsbizfoundry-project-workspace-demo_db` 不存在，其他 Demo 卷未被目标操作；Project 普通测试、`go vet`、验收驱动测试、Shell 语法、Compose 配置和工作区同步均通过。

操作者说明：[Project Workspace 独立 Demo](../deployments/demos/project-workspace/README.md)

#### Design Operations Level A 独立 Demo（2026-09-25 已通过）

静态、应用级和 fresh-state 验收：

```bash
cd acceptance/design-operations-demo
GOWORK=off go test ./...
GOWORK=off go vet ./...
cd ../../backend/services/design-operations
GOWORK=off go test ./...
GOWORK=off go vet ./...

deployments/demos/design-operations/ops/demo reset --confirm-destroy-data
deployments/demos/design-operations/ops/demo run
deployments/demos/design-operations/ops/demo replay
```

实际结果：

- Compose service 集合严格等于 `db + design-operations + reference-stub`；stub 只实现版本化 Project／Assignment／Evidence reference-read，数据库与 stub 都不暴露宿主机端口，未启动真实相邻业务应用、Nginx、Gateway、中心 IAM 或 Workbench；
- clean run Work Item `5cb73054-6018-4e85-aeb0-e99884fed8e1` 与 replay 场景 `33e0978b-34e7-4a1c-aeb8-180c23d07e74` 都从 draft 推进到 completed，经恢复初设、级联回退、重开施工图和替换失效成果后受控取消，最终均为 `cancelled / revision 21 / history_events 21`；
- Project／Assignment／Evidence 不存在返回 422，暂不可用返回 503；错误负责人角色、缺成果和施工图设为不适用也被拒绝，所有失败写入均保持原 revision；
- 同一负责人或 Evidence 首次可用、随后不可用，证明查询能明确显示降级，完成命令失败关闭；替换成果后才允许继续推进；
- 创建、负责人和成果命令的同键同载荷重放保持稳定事实，同键异载荷返回 409；匿名、错误令牌、伪造身份 Header 和取消后的终态写入均被拒绝；
- 历史逐 revision 覆盖 1～21，并保留同一方案槽位的旧／新 Evidence File ID，证明替换没有静默覆盖；
- 应用普通测试、`go vet`、验收驱动测试、Shell 语法、Compose 配置和工作区同步均通过。真实相邻应用组合继续由 Level B 证明。

操作者说明：[Design Operations 独立 Demo](../deployments/demos/design-operations/README.md)

#### Level A 最终统一门禁（2026-09-25 已通过）

执行：

```bash
make level-a-demo-static-check
deployments/demos/ops/level-a audit --confirm-destroy-demo-data
deployments/demos/ops/level-a status
```

统一入口对 Contract、Money、People、Evidence、Project、Design 依次执行旧状态清除、fresh-state run、完整 replay 和专属状态清理。实际结果为：

```text
contract-ledger      passed  fresh-state,run,replay,cleanup
money-flow-ledger    passed  fresh-state,run,replay,cleanup
people-groups        passed  fresh-state,run,replay,cleanup
evidence-service     passed  fresh-state,run,replay,cleanup
project-workspace    passed  fresh-state,run,replay,cleanup
design-operations    passed  fresh-state,run,replay,cleanup
```

最终聚合审计的代表性稳定结果：Contract `f9a96fdf-7d5a-467e-bb28-6174390b0b64 / closed / revision 4`，Money Obligation `93c9a746-fa5e-4ef5-9a24-2a452338d64b / closed / revision 2`，People Group `528a8711-feda-4dc2-a263-093b2b7f5402 / archived / revision 11`；Project 两轮均为 `cancelled / revision 9 / history 9`，Design 两轮均为 `cancelled / revision 21 / history 21`，Evidence 两轮均完成 File Version 1 且分别清理 2 个无效／已取消对象、0 失败。

`deployments/demos/level-a-capabilities.tsv` 的六项判定全部为 `complete`，同一次最终统一审计中的六套运行报告也全部为 `passed / fresh-state,run,replay,cleanup`，最终输出 `LEVEL_A_STATUS=passed`。审计后再次检查六套 Demo 专属容器和数据卷均无残留；随后 Level B 高保真组合也已通过，完整入口与公司环境仍是独立验收轴。

统一入口：[Demo Validation 2 Level A](../deployments/demos/README.md)

#### Demo 人工可用性第一轮验证（2026-09-25）

应用级回归：

```bash
for app in contract-ledger money-flow-ledger people-groups \
  project-workspace design-operations evidence-service; do
  (cd "backend/services/$app" && GOWORK=off go test ./internal/httpserver)
done
```

六个 `internal/httpserver` 包均实际通过。随后重建 `deployments/local-oa` 中六个业务应用容器，六项 `/health/ready` 均返回 ready；Contract 与 Money 的公共样式资源返回 HTTP 200，Evidence、Project 与 Design 根入口返回 HTML 200。

人工浏览器检查实际覆盖：

- Money Flow 的资金流水页面和首页：确认统一页头、主导航、表单卡片、表格容器、三步首次使用引导和模块入口可见，原有表单动作仍可进入；
- Project Workspace 首页：确认产品目的、首次演示顺序、当前 API 已验证能力、明确的未完成提示和系统边界可读；
- 其余入口由 HTTP 返回、模板解析和对应服务测试共同覆盖。本轮没有把 Evidence／Project／Design 的“可读总览”记作完整浏览器业务工作台通过，记录列表、创建表单和记录详情仍待后续人工操作验收。

该轮只证明第一轮信息架构和视觉框架生效，以及现有服务未因呈现层修改失去健康状态；不证明完整入口 Nginx／Gateway／IAM、公网 HTTPS 或三项待建浏览器工作台已经完成。

#### Contract／Money 导航选中态回归（2026-09-25）

用户路径复现确认旧模板通过“不渲染当前页面链接”表达当前位置；当链接改造成 Tab 视觉后，这一旧逻辑会稳定表现为“点击哪个 Tab，哪个 Tab 消失”。修复后 Contract 与 Money 的业务导航始终渲染固定导航集合，并用单一 `aria-current="page"` 标记和可见高亮表达当前位置；详情页的返回操作与产品主导航分离。

```bash
cd backend/services/contract-ledger
GOWORK=off go test ./internal/httpserver

cd ../money-flow-ledger
GOWORK=off go test ./internal/httpserver
```

两项测试均实际通过；新增回归守卫逐个扫描所有含主导航的模板，要求恰好一个当前项、稳定的导航标签和对应可见样式。重建本地组合中的两个容器后，两项 readiness 均返回 ready。浏览器实测 Contract 的“审计 → 主体 → 合同”和 Money 的“首页 → 账龄”切换，目标 Tab 均保持存在并高亮。People、Evidence、Project 与 Design 当前没有同形多 Tab 主导航，源码扫描未发现相同触发路径。

### M9 本地参考 OA Ubuntu 可移植包验收（2026-09-25 已通过）

目标机只读预检的正负例先独立执行：

```bash
make local-oa-host-preflight-test
```

该测试以隔离的伪 Ubuntu、release marker 和主机命令覆盖正常通过，以及 CPU 架构不匹配、验收端口占用、磁盘不足、Docker daemon 不可达、默认 Compose project 已有资源五类拒绝路径。失败路径必须返回非零且不得执行 Docker 写操作；正常路径必须输出 `HOST_PREFLIGHT_STATUS=passed`。

构建和静态回归：

```bash
make local-oa-release-bundle-test
RELEASE_PROVENANCE_MODE=development \
  make local-oa-release VERSION=v0.1.0-demo.11 OUTPUT_DIR=dist
```

从独立归档进行 clean-room 验收：

```bash
make local-oa-release-acceptance \
  BUNDLE=/absolute/path/to/omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz \
  CHECKSUM=/absolute/path/to/omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz.sha256
```

实际结果：

- 包外摘要及包内 `RELEASE-CHECKSUMS.sha256` 全部通过；归档只包含目标运行文件，九个镜像检查为 `linux/amd64`，没有 `.env`、运行状态、私钥、证书私钥、源码或目标构建 Dockerfile；九镜像由七个完整组合运行镜像和两个仅供 Project／Design Level A 使用的公开契约桩组成；
- 包内携带受 SHA-256 清单保护且可执行的 `preflight-host`，`local-oa preflight` 只读核对 Ubuntu 22.04+、宿主／Docker／release 平台、Compose v2、运行工具、8 GiB 可用磁盘、3 GiB 内存加 Swap、18 个验收端口和九个默认 Compose project；本地正负例只证明门禁行为，真实公司 Ubuntu 仍须亲自输出通过结果；
- 同一归档首先以 `--no-build --pull never` 依次运行 Contract、Money、People、Evidence、Project、Design 六个独立产品；六项都完成 fresh-state、run、replay、cleanup，六项能力均为 complete，实际输出 `LEVEL_A_STATUS=passed`。Contract、Money、People、Evidence 只启动应用及自有 PostgreSQL（Contract／Evidence 另有自己的对象卷），Project／Design 只额外启动一个不拥有业务事实的公开 reference-read 契约桩；
- 同一归档随后运行设计交付子集和商业台账子集；运行 service 集合分别严格为 8 项和 4 项，未选业务应用及数据库不运行，两次重放摘要分别稳定为 `3e048cba031440fb11b19e72edd6a0e1a979e84decd026a2a5f7ff39d7d94c0c` 与 `63e4463303c7a5d7ebd59348b2f216d3d339b3f025f11238bc7c70136aaa2b8e`，退出后无隔离资源残留；
- 从全新临时目录装载镜像后以 `--no-build --pull never` 创建六个应用、六个独立 PostgreSQL 和独立数据卷；最终 PostgreSQL PID 1 检查避免初始化临时 server 被误判 ready；
- 归档 SHA-256 为 `3a70f263f3c9c0e30e8121640e7d99a79b0b036d3a23920ce93c079c41f99a59`，大小约 188 MiB，目标为 `linux/amd64`；
- 第一次高保真 J1～J4 建立 Project `260057d7-b4bb-47bf-9890-3977926c7b80` revision 9、Contract `2d7f8622-0189-4fc4-82f1-38bf57b7bd06` closed revision 4、Obligation `40baa7df-3b46-460e-b0d6-6c8494797c92` closed revision 2、Evidence `03b34da3-0c30-400e-b67c-ac86e56c845c` version 1 和 Design `c8e164ef-2ca3-44b5-ab1a-2754104d0e9a` completed revision 18；第二次保留完全相同的业务 ID 和 revision；
- Contract、Money、People、Evidence 和 Design 依次停机时，Project 始终为 active revision 9，两个 Assignment 快照始终可读，仅依赖已停止来源的实时引用降级；每轮恢复 healthy 后继续，最终十二个容器全部 healthy；
- 构建宿主为 ARM Mac，目标为 Linux AMD64；验收驱动器通过已装载的目标应用镜像执行，证明跨架构构建机不会把“宿主无法直接执行目标二进制”误报为业务失败；
- 七个当前驱动的 `GOWORK=off go test ./...`、`GOWORK=off go vet ./...` 以及 `make local-oa-release-bundle-test` 同轮通过；显式 reset 后六个单系统、两个子集和一个全量组合共九个 Compose project 均无容器、网络或卷残留。

该证据证明 `v0.1.0-demo.11` 能在无源码、无 Registry pull 和无目标机编译路径下复现六个独立产品、两个可裁剪子集，以及当前高保真 J1～J4、幂等重放和五来源隔离，且能在变更目标主机前只读检查 Ubuntu、平台、Docker、资源、端口和 Compose 冲突。`v0.1.0-demo.10` 的运行验收虽通过，但因包内操作文档仍指向 `demo.9` 而未被接受为交付候选；`demo.11` 重建后已额外通过包内版本号一致性核对。其元数据为 development provenance，`commit=unknown`，所以不能称为正式可追溯发布；公司 Ubuntu 主机上的实际硬件、Docker、权限、网络和完整入口链仍须在获得授权后单独验收。

### 客户场景边界证据契约验收

```bash
make scenario-evidence-check

go run ./acceptance/scenario-evidence/cmd/validate \
  /absolute/path/to/customer-scenario.json

go run ./acceptance/scenario-evidence/cmd/readiness \
  --require evidence_ready \
  /absolute/path/to/customer-scenario.json

make scenario-evidence-compare \
  BASELINE=contracts/scenario-evidence/v1/examples/reference-oa.json \
  CANDIDATE=/absolute/path/to/final-customer-scenario.json
```

门禁验证 v1 JSON Schema 可解析且 `$id` 稳定，参考 OA 基准和第二客户草稿模板均通过严格解析；负例固定拒绝未知字段／版本、参考产品冒充客户验收、真实客户最终结果缺少确认、假设探针冒充最终结果、缺少验收证据、`passed` 使用非零边界信号、未知事实引用、无纠错规则、空策略运行证据和不完整／越层连接器。readiness 单元测试另固定参考 OA 为 `final_ready`、空白客户模板为 `draft`、事实齐全但缺客户最终确认的真实场景为 `evidence_ready`，并验证已实现差异缺交付物或运行证据时不能进入实现评审；完整 `failed` 场景的非零边界信号被保留为 `BOUNDARY_FINDING` 且可以达到 `final_ready`。comparison 单元测试固定错误身份、非最终候选和重复 ID 失败关闭，报告按固定顺序覆盖应用、六层、处置和七项信号，保留失败候选的非零反证，并始终输出 `cross_customer_validated=false/result=not_determined` 和人工评审义务。

该门禁通过只说明场景账目满足证据格式、失败关闭规则和所要求的操作阶段，并证明最终证据到来后可生成不越权的比较材料。当前参考 OA 仍是 `reference_product`，模板仍是 `draft`；第二个真实客户、客户确认和跨客户产品判断没有因此变成已通过。人工采集入口见[第二客户采集与就绪判定](./operations/second-customer-scenario-intake.md)。

## M10-001 File Transfer Core 验收

```bash
cd packages/file-transfer-core
npm ci --ignore-scripts --no-audit --no-fund
npm run check
npm audit --omit=dev --audit-level=high
```

`npm run check` 固定执行严格 TypeScript 类型检查、Vitest、Evidence v1 OpenAPI 契约守卫和声明构建。2026-09-21 已通过 4 个测试文件、15 项测试；生产依赖审计未发现漏洞。

该切片只证明业务无关浏览器组件可以按版本化契约工作；服务端事实由 M10-002 单独验证。完整范围见 [M10-001 File Transfer Core](./specs/evidence/m10-001-file-transfer-core.md)。

## M10-002 Evidence Service 验收

普通质量门禁：

```bash
cd backend/services/evidence-service
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

真实 PostgreSQL、HTTP 和备份恢复：

```bash
cd backend/services/evidence-service
docker compose up -d --wait
TEST_DATABASE_URL='postgres://evidence_service:evidence_service_dev@127.0.0.1:55436/evidence_service?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration \
  ./internal/evidence ./internal/httpserver ./internal/storage ./internal/systembackup
```

独立交付入口：

```bash
cd backend/services/evidence-service
docker build --tag omsbizfoundry/evidence-service:validation .
RELEASE_PROVENANCE_MODE=development RELEASE_BUILT_AT=2026-09-21T12:00:00Z \
  ./ops/build-release v0.1.0-rc.1 /absolute/output
```

2026-09-21 已在全新 standalone Compose volumes 上实测 Create → Local PUT → Complete → Access Grant → Range GET，并在测试后销毁临时 volumes；PostgreSQL 18 完整冷备份恢复保留数据库计数、File Version 摘要和真实对象字节。自动化同时覆盖 IAM／模板负例、单对象 OSS Policy、过期和篡改 Token、越租户、越 Session、幂等负载／资源冲突、并发 PUT／Abort、数据库完成失败及孤儿清理。

真实 OSS 浏览器验收入口：

```bash
cd backend/services/evidence-service
EVIDENCE_ENV_FILE=.env.oss-validation \
  docker compose --env-file .env.oss-validation \
  -f compose.standalone.yaml up -d --build --wait

cd ../../packages/file-transfer-core
npm run check

cd ../../acceptance/evidence-oss-browser
npm ci --ignore-scripts --no-audit --no-fund
npm run check
npm run dev
```

2026-09-22 已在 `http://127.0.0.1:5173` 和真实私有 `xidou-test-files` Bucket 执行：CORS `OPTIONS` 返回 `200` 并精确匹配 Origin／Methods／Headers；1 MiB 与 150 MiB 合成文件经浏览器直传并由 Evidence 重读确认，数据库形成两个 `completed` Version，150 MiB SHA-256 与独立零字节流计算一致；另一次 150 MiB multipart 在出现进度后取消并被 cleanup 清理；独立负例脚本使用同一临时 STS 写相邻错误 Object Key，OSS 返回 `403 AccessDenied`，随后取消的会话也被 cleanup 清理，最终状态为两个 `completed`、两个 `cleaned`；1 MiB 短期签名下载字节 SHA-256 一致；未签名对象 GET 返回 `403`。浏览器控制台没有错误，仅有 `ali-oss` 对固定、不可续期 STS 未配置自动刷新回调的提示；当前上传在 15 分钟授权内完成，该提示不作为成功证据，也不应通过把长期凭证交给浏览器来消除。

仍未验证且不得标为通过：人为网络中断后的 multipart／complete 重试、STS／签名 URL 到期后的真实云端拒绝、Contract Ledger 遗留 Evidence 迁移、真实客户 IAM 与公网 HTTPS／Nginx／Gateway 链路。规格见 [M10-002 Evidence Service](./specs/evidence/m10-002-evidence-service.md)。

## M10-003 Evidence Service D1 装配验收

静态契约、Go 回归和两种 Nginx 配置：

```bash
sh -n deployments/d1/ops/d1 deployments/d1/ops/render-suite-manifest \
  deployments/d1/ops/render-nginx-config deployments/d1/ops/grant-bootstrap-access \
  acceptance/d1-delivery/verify-running deployments/d1/postgres/01-create-app-databases.sh
go test ./acceptance/access-contract/... ./backend/services/evidence-service/... \
  ./backend/platform/gateway/... ./frontend/oms-workbench/...
ENABLE_EVIDENCE=true deployments/d1/ops/render-nginx-config /tmp/nginx-enabled.conf
ENABLE_EVIDENCE=false deployments/d1/ops/render-nginx-config /tmp/nginx-disabled.conf
docker run --rm --add-host gateway:127.0.0.1 \
  -v /tmp/nginx-enabled.conf:/etc/nginx/nginx.conf:ro nginx:1.28-alpine nginx -t
docker run --rm --add-host gateway:127.0.0.1 \
  -v /tmp/nginx-disabled.conf:/etc/nginx/nginx.conf:ro nginx:1.28-alpine nginx -t
```

隔离 D1 运行验收：

```bash
D1_ENV_FILE=/absolute/path/to/test.env D1_PROJECT_NAME=isolated \
  deployments/d1/ops/d1 up
D1_ENV_FILE=/absolute/path/to/test.env D1_PROJECT_NAME=isolated \
  D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
D1_ENV_FILE=/absolute/path/to/test.env D1_PROJECT_NAME=isolated \
  deployments/d1/ops/d1 backup
D1_ENV_FILE=/absolute/path/to/test.env D1_PROJECT_NAME=isolated \
  D1_CONFIRM_EMPTY_RESTORE=yes deployments/d1/ops/d1 restore BACKUP_ID
```

2026-09-21 至 2026-09-22 已在隔离 Compose 项目、全新数据卷和 PostgreSQL 18.6 实际执行。全应用栈验证只有 Nginx 暴露宿主端口、一次 IAM 登录、Evidence 单 Audience 授权、PDF Create → PUT → Complete → Access Grant → Range GET、服务端 SHA-256、Workbench 隐藏技术应用、Evidence 停机时其他三个业务应用继续可用，以及 Nginx 数据日志不泄漏 URL Token。

联合备份包含 `evidence-service.zip`；删除测试项目全部卷后恢复，原 File ID／Version／SHA-256／对象字节一致，恢复后继续写入通过。`ENABLE_EVIDENCE=false` 时控制面和本地数据面均为 `404`、Evidence 容器停止、对象卷保留；重新启用后原数据存在。另一次最小空卷回归证明首次 Compose 安装不会因 PostgreSQL 本地健康与容器网络 TCP 的短暂竞态失败。

当前 D1 使用 Local Provider，因此这次 D1 证据本身仍不是 OSS 直传证据。真实阿里云 Bucket／CORS／150 MiB multipart 已由上述独立 OSS 组合补充验证；真实公网 HTTPS／Nginx／Gateway 和 OSS Provider 的整套 D1 装配仍属于外部验收。完整范围见 [M10-003 Evidence D1 装配](./specs/evidence/m10-003-d1-integration.md)。

OSS Endpoint 拆分后的回归还必须证明：`OSS_BROWSER_ENDPOINT` 与 `OSS_SERVICE_ENDPOINT` 可独立配置；旧 `OSS_ENDPOINT` 能同时回退两者；上传授权只暴露浏览器公网地址；签名下载基于公网地址；Evidence 的对象复核／清理只使用服务端地址。2026-09-22 已通过 Evidence 全量 `go test`、`go vet`、三项命令构建、旧环境文件容器重建、真实授权端点探针和原 1 MiB 对象签名下载摘要复核。本机没有 VPC 内网条件，因此同地域 ECS 上 internal Endpoint 的真实连通性仍是部署验收项，不能用单元测试代替。

## Party Directory 独立边界证据审计

2026-09-25 对 Contract Ledger、Money Flow Ledger 和参考 OA 的主体能力执行只读代码／规格审计。两个台账都已经实现 organization／individual、法定名称、可选登记标识、revision、登记、列表和主体审计；两者又分别在合同 revision、结算、义务、实际收付款和内部转账中冻结本地 Party 快照。参考 OA 则把我方、甲方和资质单位保留为现实主体，但按客户要求隐藏独立管理入口并使用项目文本与历史名称自动补全。

实际回归：

```bash
cd backend/services/contract-ledger
GOWORK=off go test ./internal/party

cd ../money-flow-ledger
GOWORK=off go test ./internal/party
```

两项均通过。该证据证明 Party 已具有两个稳定实现点和第三种客户装配方式，足以接受独立 Party Directory 的目标边界；它不证明独立应用、API、迁移、D1 装配或故障行为已经实现。后续实现必须保留现有业务 Party ID／revision／快照／审计的解释力，并新增跨应用契约、备份恢复和组合故障验收。

## Party Directory 首个 Project 消费边界

2026-09-27 依据参考 OA 的 `clients` 与 `projects.client_id` 关系，完成首个 Party 消费实验。结论是 Party Directory 拥有主体当前档案，Project Workspace 拥有项目选择客户的关系，并在创建时冻结 `party_id + source_revision + kind + legal_name`；Business Engagement 不作为默认客户关系所有者。

实际执行：

```bash
cd backend/services/project-workspace
GOWORK=off go test ./...
GOWORK=off go vet ./...

TEST_DATABASE_URL='postgres://project_workspace:project_workspace_dev@127.0.0.1:55437/project_workspace?sslmode=disable' \
  GOWORK=off go test -tags=integration ./internal/projects ./internal/httpserver
```

以上检查通过，覆盖：

- active、同租户、精确 revision 的 Party 验证与最小快照持久化；
- Party 后续改名／推进 revision 不改写 Project 详情和 `project_created` 历史；
- 同 command 重放先命中原 Project，不再次调用 Party；
- stale revision、跨租户、inactive、错误来源和未配置端点均失败关闭；
- Party 503 返回可重试错误且数据库没有部分 Project；
- 正式 IAM 交换使用 `party-directory` audience、`party-directory.reference.read` action 和精确 `party:{id}` scope；
- Project 不选择客户时继续独立创建，基础更新不能夹带客户重选。

该段证据只支持首次本地消费者边界；后续 D1 专项已经证明 Party 正式 API 入口、精确 IAM 工作负载授权、依赖停机／恢复和联合空目标还原；再后的管理／客户选择实验补齐了用户发现凭证与服务端 workload 精确复验分层。Contract／Money 只读预检、来源自有映射和 D1 各 1 条人工确认已证明，含引用数据、运行时切换／回滚和完整双租户矩阵仍未证明。完整矩阵与限制见 [Party→Project 首个消费者实验](./reports/oa-party-project-consumer-experiment-20260927.md)、[Party→Project D1 正式链实验](./reports/oa-party-project-d1-chain-experiment-20260927.md)、[Party 管理与 Project 客户选择实验](./reports/oa-party-project-human-workflow-experiment-20260927.md)、[Contract／Money Party 迁移预检实验](./reports/oa-party-ledger-migration-preflight-experiment-20260927.md)和[Contract／Money Party D1 人工确认映射实验](./reports/oa-party-ledger-d1-reviewed-mapping-experiment-20260927.md)。

## OA 完全体业务关系证据审计

2026-09-25 对参考 OA 的当前确认需求、产品规格、后端领域模型、路由和前端入口执行只读审计，覆盖账号权限、项目／项目包、经营承接、合同链、开票与资金、四类生产、批复审计、生产成本、外委、奖金、文件、统计和待办。

审计形成两项文档结果：

- `arckit/spec/assembly/full-reference-oa-validation.md` 固定完整目标的 C1～C8 业务旅程、当前覆盖、明确缺口、未知项和验收门槛；
- `arckit/tech/assembly/customer-overlay-research.md` 固定稳定 Base 加 Customer Overlay 的候选结构、禁止覆盖边界和跨客户反证信号。

证据结论是“当前架构能够承载 OA 完全体，但当前实现尚不完整”。该结论来自静态业务证据审计，本轮没有运行新的应用测试、浏览器验收或性能测试，因此不能把它记录为 OA 完全体已实现、Customer Overlay 已跨客户通过或运行性能已验证。

## OA C2 经营承接责任边界审计

2026-09-27 继续对参考 OA 的确认需求、适用性／承接方式模型、材料模型、经营进度服务、路由和权限执行只读审计，并以三项候选作反证比较：独立 Business Engagement、Project 类型化扩展、Contract 前置状态。

直接观察包括：阶段不适用可恢复；阶段不适用和单份材料因方式不要求是两类事实；承接方式与材料齐备能够在合同不存在时独立形成阶段结果；参考服务随后才把承接、合同和支付聚合为经营进度；当前代码跨读 Project、Contract 和宽 FinancialRecord 是遗留单体实现而非目标边界。全仓没有找到未中标、流标、多轮承接、自动建合同或专家费完整责任链的明确行为。

审计结果记录在 [OA C2 经营承接责任边界审计](./reports/oa-c2-business-engagement-boundary-audit-20260927.md)，规格与技术边界分别记录在：

- `arckit/spec/business-engagement/engagement-lifecycle.md`；
- `arckit/tech/business-engagement/decision.md`。

本轮接受“独立 Business Engagement”作为目标事实所有者；它拥有适用性、方式、材料业务引用、缺口／完成条件、revision 与历史，Project 只保留引用／投影，Contract／Money 独立确认建议，Evidence 继续拥有文件技术事实。第一版 `ready` 只表示材料条件满足。该结论只来自只读证据审计，没有新增应用代码、数据库、权限矩阵或运行测试，因此状态是目标边界已接受、实现未通过，OA 完整体仍为 `insufficient-evidence`。

## Business Engagement 可执行进程与精确工作负载凭证

2026-09-27 在第一条后端闭环上补齐真实 server 配置、优雅停止、工作负载密钥工具和正式 IAM 出站装配。正式模式禁止 Project／Evidence 静态 Token，只允许独立工作负载身份向 IAM 申请 `project-workspace.read + project:{id}` 和 `evidence-service.view + file:{id}`；本地 template 模式仍要求三份显式测试凭证，不能关闭鉴权。

检查正式链路时发现 Evidence 只接受 tenant scope，与 IAM 已支持的精确 File scope 不一致。修复后，Evidence 只对 `evidence-service.view` 接受 `file:{id}`，并在列表、reference-read 和 access-grant 再次检查 File ID；上传继续要求 tenant scope。测试直接证明相邻 File 返回 `403` 且没有调用读取用例、列表只返回被授权 File、资源级上传被拒绝。共享工作负载身份实现同时由 Project Workspace 和 Business Engagement 使用，两个调用方仍各自持有私钥和 Workload Policy 条目，不共享运行身份。

普通回归、静态检查和 Access Contract：

```bash
go test \
  ./packages/workload-identity/... \
  ./backend/services/project-workspace/... \
  ./backend/services/business-engagement/... \
  ./backend/services/evidence-service/... \
  ./backend/platform/identity-access/internal/capability \
  ./acceptance/access-contract/...

go vet \
  ./packages/workload-identity/... \
  ./backend/services/project-workspace/... \
  ./backend/services/business-engagement/... \
  ./backend/services/evidence-service/...
```

PostgreSQL 18 真库并发与幂等回归：

```bash
TEST_DATABASE_URL='postgres://business_engagement:***@127.0.0.1:55439/business_engagement?sslmode=disable' \
  go test -count=1 -race -tags=integration ./backend/services/business-engagement/...
```

以上全部通过。另在一次性 PostgreSQL 18.6 容器上用 template 装配启动实际 `cmd/server`，`/health/live` 与 `/health/ready` 返回 `200`，无凭证业务读取返回 `401 UNAUTHENTICATED`，随后优雅停止并删除测试数据库容器。Project 的独立镜像、D1 镜像和 Local OA 通用镜像均从仓库根实际构建成功，证明共享技术模块没有破坏既有 GOWORK=off 交付路径。

这组证据在当时不包含 Business Engagement 镜像／release、Suite Manifest、Nginx／Gateway 路由、Customer Overlay、Project Assignment Policy、撤权、跨租户、页面或恢复。镜像、release、正式路由、Workbench 页面、首组权限矩阵和本地空目标恢复随后由下节补齐；跨租户、409／503 浏览器专项和真实环境仍使 C2 与 OA 整体结论继续为 `insufficient-evidence`。

## Business Engagement D1 C2 权限与正式入口

2026-09-27 把 Business Engagement 加入 D1 的 PostgreSQL 初始化、migration、镜像／pull 镜像映射、Suite Manifest、Workbench、Gateway 路由、IAM Capability Manifest、引导 Grant、联合备份／恢复脚本和 release 归档。IAM 的 Project Assignment Policy 加载由单文件扩展为按 audience 选择多份版本化策略；同一 Assignment 事实因此可以分别签发 `project-workspace` 和 `business-engagement` 能力，但不引入共享角色表或目标应用中的客户岗位判断。

隔离 D1 复验命令：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

验收只通过对外 Nginx 端口进入，覆盖：

- 参考 OA C2 Customer Overlay 把经营负责人、生产负责人、经营管理、总经理只读和财务角色映射为 Business Engagement 稳定 Capability；身份管理员在没有显式业务 Grant 时不能访问业务；
- 经营负责人仅取得所属 Project 的精确凭证，相邻 Project 失败；Gateway 清理旧 Capability Cookie，伪造 Authorization 与 `X-OMS-*` Header 都返回 401；
- 经营负责人可建立 Engagement、设置适用性／承接方式、通过真实 Evidence 上传挂接精确 File；Business Engagement 用独立工作负载身份交换 Project 和 File 资源凭证，运行环境没有静态 Project／Evidence Token；
- 直接 Grant 撤销后刷新凭证立即丢失对应 action，旧短期凭证只在 TTL 内残余；Assignment 结束后不再签发新凭证；
- 经营管理者可取消 Engagement，取消后即使凭证仍含 action，目标应用也按本地状态返回 409；历史与 IAM 策略／工作负载／撤权审计均可查。

最终复验结果是 **89 个 HTTP 断言、0 个状态不匹配，最大本地请求时间 0.944209 秒**。新增断言覆盖 Workbench 页面／脚本、列表与详情 `allowed_actions`、撤权后动作收敛、取消状态只读，以及 Assignment 结束后新浏览器入口拒绝。实际浏览器又证明经营经理可从 Workbench 经 Gateway／IAM 进入页面，撤销 `evidence.manage` 后刷新立即隐藏动作，恢复 Grant 后动作重新出现。同一多策略 IAM 装配的 C1 最近回归为 **152 个 HTTP 断言、0 个状态不匹配，最大时间 0.186476 秒**。本机 Docker 地址池已满，验证使用临时 Compose 网络覆盖固定 `10.250.9.0/24`～`10.250.12.0/24`；一次性隔离验收曾把上限设为 1000，仓库默认保持 120。2026-09-27 的真实 D1 升级进一步暴露 Nginx 后按直接对端聚合全部用户的问题，现已由上文可信边缘客户端键契约修复；真实目标在默认 120 下的复验结果记录于对应环境报告，不以提高限额作为最终修复。

完整角色矩阵、证据索引、局限和变化账见 [OA C2 Business Engagement 权限实验报告](./reports/oa-c2-business-engagement-authorization-experiment-20260927.md)。后续 409／503 自动化、真实浏览器故障恢复，以及账号停用、IAM／Policy 故障和目标错租户拒绝见本节下方专项。完整双租户 D1 交叉矩阵与真实 Ubuntu／公网环境仍未通过，OA 整体保持 `insufficient-evidence`。

### Business Engagement D1 联合恢复专项

2026-09-27 在同一隔离 D1 `omsbizfoundry-c2` 中执行真实联合备份和破坏性空目标恢复。为让运维命令复用该验收栈的固定 Docker 子网，`deployments/d1/ops/d1` 增加只接受存在绝对路径的 `D1_COMPOSE_OVERRIDE_FILE`；同一个覆盖文件必须用于 `up/check/backup/restore`，未设置时正式 D1 行为不变。

备份 `20260926T191755Z` 包含 IAM dump／签名密钥、People、Project、Evidence 和 `business-engagement.dump`，全部文件为 `0600` 且外层 SHA-256 逐项通过。删除该 Compose project 的全部数据卷后，恢复前后的 6 个 Engagement、30 条不可变历史、30 条命令、6 条材料引用及其 ID／revision／Project revision 逐条一致；Project 18／Assignment 15、Evidence File 6／Version 6、IAM Grant 102／Audit 333 和签名密钥摘要也保持。IAM Session 按设计从 73 归零，旧会话不随灾难恢复复活。

恢复后 `d1 check` 全部通过，再从 Nginx 单入口重跑 C2：**89 个 HTTP 断言、0 个状态不匹配，最大本地请求时间 0.333473 秒**。新业务闭环使 Business Engagement 的 Engagement／History／Command／Material 从 `6/30/30/6` 增长到 `7/35/35/7`，证明可继续写入。完整步骤、表级证据和限制见 [OA C2 Business Engagement 联合恢复实验](./reports/oa-c2-business-engagement-recovery-experiment-20260927.md)。该本地证据不替代离机副本、pull-only ACR、真实 Ubuntu、公网 HTTPS 或 409／503 浏览器专项。

### Business Engagement 409／503 权限与故障恢复专项

2026-09-27 在同一隔离 D1 中扩展正式入口验收。故障注入必须显式启用，且只作用于指定 Compose project 的 Evidence Service：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
D1_COMPOSE_OVERRIDE_FILE=/tmp/omsbizfoundry-c2-networks.yaml \
C2_ENABLE_DEPENDENCY_FAILURE_TEST=true \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

最新镜像重建后的最终结果为 **95 个 HTTP 断言、0 个状态不匹配，最大本地请求时间 0.598558 秒**；IAM 审计记录为 Policy 27、Project 工作负载 9、Evidence 工作负载 9、撤权 84。旧 expected revision 返回 retryable `409 REVISION_CONFLICT` 后，详情仍为 revision 3、历史仍为三条；停止 Evidence 后材料命令返回 retryable `503 REFERENCE_SERVICE_UNAVAILABLE`，详情仍为 revision 4 且只有一条活动材料。恢复 Evidence 并使用新的 `command_id` 重试后只推进到 revision 5，活动材料仍恰好一条。退出清理保证 Evidence 恢复。

实际工作区脚本从 Nginx／Gateway 路径读取并通过 `node --check`，验收守卫 409 不自动重放、503 显式新命令重试、保留投影以及携带 `project_id` 返回 Project；Project 页面支持自动打开该来源项目。随后在 `http://127.0.0.1:18101` 的真实浏览器中补齐受控点击：旧 revision 提交显示 409 无写入且载入最新投影；Evidence 停机时显示 503、保留 revision 5 与表单输入，恢复后点击“使用新命令重试”推进到 revision 6；数据库只有一条活动 `award_document`；“返回 Project 核对”在新标签打开并自动选中同一 `project_id`；撤销全部 Business Engagement Grant 后刷新权限，页面退出编辑态且 `allowed_actions` 为空。一次性账号随后停用，Suite Session 返回 401，Evidence Service 恢复 healthy。完整证据见 [OA C2 Business Engagement 权限与故障恢复实验](./reports/oa-c2-business-engagement-fault-recovery-experiment-20260927.md)。

### Business Engagement 账号、IAM 与 Policy 边界专项

同一隔离 D1 在重建 IAM 和 Gateway 镜像后启用第二组显式故障开关：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_COMPOSE_OVERRIDE_FILE=/tmp/omsbizfoundry-c2-networks.yaml \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
C2_ENABLE_DEPENDENCY_FAILURE_TEST=true \
C2_ENABLE_IAM_BOUNDARY_TEST=true \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

最终结果为 **110 个 HTTP 断言、0 个状态不匹配，最大本地请求时间 0.910337 秒**；IAM 审计记录为 Policy 31、Project 工作负载 11、Evidence 工作负载 12、撤权 99、账号停用 2。同一装配随后重跑 C1，得到 **152 个 HTTP 断言、0 个状态不匹配，最大时间 0.530457 秒**。

账号停用后 Suite Session、重新登录和新授权立即返回 401；此前已经签发的应用 Credential 不在线回查 IAM，只能在其最长 300 秒签名 TTL 内残余。IAM 停机时显式刷新返回 502 并清除应用 Cookie；Project Policy 事实源停机时 IAM 返回 503 `policy_dependency_unavailable`，Gateway 同样清除 Cookie，且失败前后没有新增 `iam_policy_credential_issued` 审计。依赖恢复后重新授权成功并恰好新增一条签发审计。Business Engagement 目标校验单元测试另以真实签名覆盖 Tenant 不匹配，返回未认证；这证明目标应用拒绝有效签名错租户凭证，但不冒充完整双租户 D1 浏览器矩阵。完整结果见 [OA C2 IAM 边界与故障恢复实验](./reports/oa-c2-iam-boundary-experiment-20260927.md)。

## OA 完整体架构可行性验证报告门禁

### C4 多生产类型边界审计与前三兼容实现轮（2026-09-27）

C4 第一轮已完成需求、参考实现、现有 Design 模型／migration／API／权限和 Project 类型门禁的静态对照。结论采用单一 Production Operations 数据所有者、代码拥有且版本化的 Type Definition，并以 `consultation-v1` 作为 `design-v1` 之外的首个反证切片；拒绝每类型复制服务、把阶段规则写入 Project、双写或任意工作流平台。验收固定为 C4-A01～A14，覆盖定义注册、真库兼容升级、咨询生命周期、同项目双类型隔离、权限、最小组合、未选类型缺席、回归和反万能平台检查。

前两兼容实现轮实际运行以下门禁：

```bash
go test ./backend/services/design-operations/...
go vet ./backend/services/design-operations/...
go build ./backend/services/design-operations/...
docker compose -f backend/services/design-operations/compose.yaml up -d postgres
TEST_DATABASE_URL='postgres://design_operations:design_operations_dev@127.0.0.1:55438/design_operations?sslmode=disable' \
  go test -p=1 -count=1 -tags=integration ./backend/services/design-operations/...
```

全部通过。旧 `000001` 真库 fixture 含 Work Item、四条 Stage、Deliverable、Command 和 History；升级后逐项字段与确定性 digest 不变，历史不可变 trigger 继续生效。同 Project 双类型创建成功，未知、未选类型和错误 Project revision 拒绝后 Work Item／Command 仍为 `2/2`。并行 package 首次运行曾因同时建隔离测试库触发 20 秒环境超时；迁移单包与 `-p=1` 完整套件随后通过，没有业务断言失败。

第二轮同一真库测试让咨询独立经历跨 Project／错误角色负责人拒绝、scheme 跳过、Evidence 503 零写入与同命令恢复、report 成果替换、已结束负责人拒绝完成、完成／重开／再完成、恢复 scheme 后级联回退和最终再完成。咨询最终 revision／历史为 `12/12`，设计保持 revision／历史 `2/2`、`scheme=ready` 且零成果。领域入口现在按 Work Item 保存的 type/version 校验 Stage、可跳过规则和必需成果，外部引用前拒绝错误类型 Stage；成功重放仍先返回本地结果。C4-A01～A09 因此通过。

第三轮增加 canonical `/api/v1/production-work-items` 与精确版本 Reference，并把 IAM 分为 `design-operations` 和 `production-operations` audience。严格契约测试覆盖 `production-work-item.*` Manifest、参考 OA C4 Overlay 和 Project Assignment Policy；Production IAM 拒绝旧 audience／action／resource type；Policy 只允许 active、当前 revision、同 Tenant／Project 的 `production_owner`，拒绝经营负责人、相邻 Project、错 Tenant、stale 和 ended Assignment。HTTP 测试证明旧列表隐藏 consultation、旧 action 访问 canonical 为 403、精确 Work Item Scope 拒绝相邻项；PostgreSQL 18.6 race 集成测试经 canonical API 完成 consultation revision 1→5，旧 Design 路由读取同项为 404。实际通过普通、vet、build、race 和串行真库 race 命令。C4-A10 本地纵向证据因此通过，但正式 Nginx→Gateway→IAM 签发／撤权、最小组合、未选缺席、Level A／B 聚合回归、浏览器、D1 或 Ubuntu 尚未运行；C4 仍为 `partially-supported`，OA 整体继续为 `insufficient-evidence`。完整步骤、逐项状态、负例和限制见 [OA C4 多生产类型责任边界审计](./reports/oa-c4-multi-production-boundary-audit-20260927.md)。

当前下一阶段按 [OA 完整体架构可行性验证计划](./oa-full-scope-validation-plan.md) 组织证据。每个新增纵向闭环必须把以下结果写入现有验收入口或独立的可重复验收目录：

- C1～C8 对应的正常、纠错、撤销／反向、并发和重放结果；
- 应用独立启动、独立 migration、备份恢复和未选应用真实缺席；
- 跨应用契约版本、幂等、来源不可用和恢复后的行为；
- Base 与 Customer Overlay 变化账及客户核心分支、共享业务表、现场核心补丁反证检查；
- 首页、列表、记录工作区、完整旅程和错误恢复的浏览器证据；
- 同一镜像摘要在公司私有 Ubuntu 或 ICP 生效后的个人阿里云试验田至少一处完成 HTTPS 入口、IAM、Evidence、应用开关、故障、升级、离机备份、空目标恢复和继续写入。

最终报告使用 [OA 完整体架构可行性验证报告模板](./templates/oa-full-scope-architecture-validation-report.md)。`supported` 必须引用上述全部直接证据；缺少关键旅程或真实环境时使用 `insufficient-evidence`，出现共享事实、长期客户核心分支、不可升级 Overlay 或不可恢复数据时使用 `not-supported`。文档、静态审计、接口返回成功或页面存在均不能单独把状态提升为通过。

## 六应用 Demo 实际体验基线验收

2026-09-25 完成六个业务应用的实际入口复验。Contract Ledger、Money Flow Ledger 和 People & Groups 继续使用原有真实表单路径；Project Workspace、Design Operations 和 Evidence Service 新增受保护列表读取与记录工作区，页面凭证只写入当前浏览器标签页的 `sessionStorage`，没有内置 Token 或鉴权旁路。

新增读取面的普通回归：

```bash
cd backend/services/project-workspace && GOWORK=off go test ./...
cd ../design-operations && GOWORK=off go test ./...
cd ../evidence-service && GOWORK=off go test ./...
```

六应用独立 Demo 回归：

```bash
go test \
  ./acceptance/contract-ledger-demo/... \
  ./acceptance/money-flow-demo/... \
  ./acceptance/people-groups-demo/... \
  ./acceptance/evidence-service-demo/... \
  ./acceptance/project-workspace-demo/... \
  ./acceptance/design-operations-demo/...
```

本地组合与高保真业务旅程：

```bash
./deployments/local-oa/ops/local-oa verify
./deployments/local-oa/ops/local-oa demo
```

实际结果：六个应用均通过 ready 检查；J1～J4 建立人员与设计组、Project、合同、义务、实际收款与核销、Evidence File Version、完成的 Design Work Item，并把五类稳定引用回挂到同一 active Project，最终输出 `PASS: high-fidelity local OA business journey J1-J4`。Project、Design、Evidence 的列表 API 使用本地模板 IAM 凭证分别返回当前租户中的真实记录，未认证请求失败关闭。

人工浏览器检查覆盖六个根入口的布局和可访问结构、Project Overview → Records List、无效凭证错误反馈与页面上下文保留，以及 Contract／Money 切换后选中 Tab 仍保持可见。Project、Design、Evidence 的 `assets/demo.js` 另经 `node --check` 通过。上述证据不包含正式 IAM、公网 HTTPS、Nginx／Gateway 全链或公司 Ubuntu；这些仍属于真实环境验收，不得由本地结果替代。

## 第二参考产品 OMS 证据验收

OMS 原型验证在相邻 OMS 仓库执行：

```bash
cd ../OMS/platform
npm run lint
npm test
```

2026-09-25 实际结果：lint 通过；生产构建通过；2 项服务端渲染和产品元信息测试通过。构建报告一个大于 500 kB 的 chunk 警告，本轮未把它描述为失败或已经解决。

BizFoundry 的 OMS 场景证据执行：

```bash
go run ./acceptance/scenario-evidence/cmd/validate \
  contracts/scenario-evidence/v1/examples/reference-oms.json
go run ./acceptance/scenario-evidence/cmd/readiness --require evidence_ready \
  contracts/scenario-evidence/v1/examples/reference-oms.json
make scenario-evidence-check
```

validator 输出 `SCENARIO_EVIDENCE_STATUS=passed`；readiness 输出 `SCENARIO_READINESS=evidence_ready`，并保留验收证据和七项生产边界未完成的 final-ready 缺口。该结果证明第二参考产品材料足以用于六层人工判断，不证明 OMS 模块化生产实现、真实客户接受、跨客户边界或真实环境已经通过。

## 2026-09-26 OA 权限架构现有证据复核

执行：

```bash
go test ./acceptance/access-contract/... ./backend/platform/gateway/... ./backend/platform/identity-access/... ./backend/services/contract-ledger/... ./backend/services/money-flow-ledger/...
```

结果：全部通过。当前直接证据覆盖访问契约跨字段校验、Gateway 身份 Header 清洗与基础验签、IAM Capability Manifest／签名凭证、Contract 与 Money 的单 Audience 隔离、动作级 Capability、缺凭证 `401`、只读写入 `403` 和目标应用最终业务校验。

该结果只建立 OA 权限验证的安全回归底座，不构成 OA 完全体权限通过。Project 进入正式 D1、直接 Resource Scope、Business Fact Snapshot／受信策略、身份管理员与业务管理员分离、C1 多角色／撤权／页面恢复，以及 Business Engagement C2 首组经营／生产／管理／财务角色、跨 Project、状态、撤权、账号停用、IAM／Policy 故障和目标错租户凭证拒绝已由后续专项补齐。尚缺 Design D1、完整双租户 D1 交叉矩阵、更广字段／金额／文件动作，以及 C3～C8 逐旅程的允许与拒绝样例。当前 D1 全能力管理员业务验收只作健康控制，不能填补这些缺口。

验证方法和判定规则见 [OA 完整体权限架构验证方案](../arckit/tech/assembly/oa-authorization-validation.md)。

### Project 正式权限语言前置门禁

资源级权限实验开始前执行：

```bash
go test ./acceptance/access-contract/... ./backend/platform/identity-access/internal/capability/... ./backend/services/project-workspace/...
```

2026-09-26 实际结果全部通过。`project-workspace-capabilities.json` 的八项 Capability 均使用 `project-workspace.*`，IAM Catalog 接受 `project-workspace.read` 并拒绝旧 `project.read`，Project 的 IAM Adapter 与 HTTP 授权测试使用同一常量集合。该结果只解除“Project 无法被正式 IAM 清单接纳”的前置阻断；Resource Scope 签发、Gateway 转发和跨 Project 拒绝仍必须由后续运行验收提供证据。

### IAM Resource Grant 与 Gateway 基础验签

执行普通回归，并以隔离 PostgreSQL 18 设置 `TEST_DATABASE_URL` 执行 IAM integration test：

```bash
go test ./backend/platform/gateway/... ./backend/platform/identity-access/...
TEST_DATABASE_URL='postgres://.../postgres?sslmode=disable' \
  go test -tags=integration ./backend/platform/identity-access/internal/httpserver
```

2026-09-26 实际结果全部通过。真库 migration `000002_resource_capability_grants.sql` 建立独立 Resource Grant 存储；测试用户只获得 Project A 的 `project-workspace.read`，省略 scope、请求 Project B 和请求 Capability 未声明的资源类型分别失败，只有 Project A Resource Credential 成功且 claims 精确包含一个 Project。Gateway 测试验证资源请求被原样交给 IAM、合法 Resource Credential 通过基础验签并以原始 JWT 转发；Gateway 没有计算业务范围或签发凭证。

同日又用 `acceptance/d1-resource-authorization/verify-running` 补齐 Nginx → Gateway → IAM → Project 运行闭环：Project A 精确 `read/update` 允许、Project B 凭证／读取／修改拒绝，拒绝写入不改变 B；销毁卷恢复后专项与完整 D1 再次通过。该段运行证据只覆盖显式直接 Resource Grant；后续 `acceptance/d1-business-fact-authorization/verify-running` 已补齐首条 Business Fact Policy、单项撤权和工作负载凭证生命周期，C1～C8 角色矩阵仍未验证。

## 华东 1 Auth58 Ubuntu D1 实机验收

2026-09-26 在一台重装后的 Ubuntu 22.04 x86_64 ECS 上完成 D1 实机部署。目标为 2 vCPU、2 GiB 内存、40 GiB 系统盘和 2 GiB Swap；业务入口仅绑定宿主回环地址，未把本轮结果冒充公网 HTTPS、DNS、备案或公司环境验收。实例 ID、公网 IP、SSH 私钥、ACR 凭证和应用秘密不写入仓库。

实际执行并通过：

- 宿主机 bootstrap 的 16 项门禁、Docker Engine 29.1.3、Compose 2.40.3、日志轮转、Swap 和 SSH 密码／root 登录关闭；
- `v0.1.0-test.2` 的 9 个 digest-pinned ACR 镜像全部在目标机本地存在，注销 ACR 后仍可运行；
- PostgreSQL 18.6、IAM、Contract、Money、People、Evidence、Gateway、Workbench 和 Nginx 全部达到 `healthy`；
- `d1 check` 确认 IAM、Contract 17 个 migration、Money 16 个 migration、People 和 Evidence schema 均为 current；
- 非破坏性 `acceptance/d1-delivery/verify-running` 通过 Nginx 单入口、一次 IAM 登录、全部启用应用读写、People roster snapshot 和 Evidence Create → PUT → Complete → Access Grant → Range GET 字节生命周期；
- `v0.1.0-test.3` 运维包在不改变 9 个镜像摘要的前提下修复真实 Linux 上的 bind-mount 与备份目录 UID 权限，外层和内层 SHA-256、脚本语法及归档权限测试均通过；
- 联合备份 `20260926T115452Z` 包含 IAM dump／签名密钥、Contract ZIP、Money ZIP、People dump、Evidence ZIP 和 checksum manifest，目标机与离机副本逐项 SHA-256 均通过，备份后 Evidence 恢复 `healthy`。

实机首次启动暴露并修复两项 Docker Desktop 未揭示的 Linux 权限缺陷：能力清单在归档中为 `0600` 导致非 root IAM 容器读取失败；宿主备份目录为 `0700` 导致 UID 70 的应用容器无法写入。发布构建现将公开 Capability Manifest 固定为 `0644` 并由归档测试守卫；备份脚本只在备份窗口对单个备份目录做容器／宿主所有权交接，结束或失败时恢复宿主所有权及 `0700/0600` 权限。

本轮尚未验证：域名与证书下的公网 HTTPS、公司私有 Ubuntu、破坏性故障隔离、升级回滚、以及在独立空目标执行联合恢复并继续写入。备份成功不能替代恢复证明；完成这些缺口前，OA 完整体架构报告仍不得给出整体通过结论。

### D1 应用选择集升级与回退回归

2026-09-27 修复 `d1 upgrade` 在候选配置新增应用时用候选 `ENABLE_*` 检查旧运行态的问题。升级前现在从同一 Compose project 的已创建服务识别旧选择集；备份包含参与 SHA-256 的 `suite-selection.env`，因此当前存在但候选关闭的应用不会漏备份，候选新增但尚不存在的应用不会被提前检查。

静态／模拟回归 `make d1-upgrade-selection-test` 覆盖：Contract 旧组合增加 Party、Party 不参加旧备份、候选启动后才检查 Party，以及候选启动失败仍留下可校验的旧选择集备份。随后在独立 Compose project 和 `127.0.0.1:18091` 实跑：先启动 IAM＋Contract＋Gateway＋Workbench＋Nginx，执行 `upgrade` 增加 Party；升级前备份 `20260927T081812Z` 只含 IAM dump／签名材料、Contract ZIP、旧选择集与 checksum，候选 Party migration、健康检查和最终 `d1 check` 通过。

同一隔离项目随后用旧环境文件和该备份执行破坏性空目标 restore：校验和通过，IAM 与 Contract 恢复，Party 从旧选择集中缺席，六个旧长期服务重新健康且 `d1 check` 通过。测试项目容器、卷和备份已清理；Docker Desktop 留下一张无容器记录但报告 ghost endpoint 的空测试网络，未重启 Docker 以免影响其他本地 Demo。该证据证明本地 D1 的“较小组合增加应用—失败保留恢复物—回到旧选择集”路径，不替代 Auth58／公网环境重新发布后的目标机复验。

## OA C1 Project Package 与转换验收

2026-09-26 为 Project Workspace 增加 migration `000002_project_package_hierarchy.sql`、两级 Project／Package 模型、attach／wrap 转换、不可变 conversion 和安全撤销。普通与 PostgreSQL 18 race 集成回归：

```bash
go test ./backend/services/project-workspace/...
TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -race -count=1 -tags=integration ./backend/services/project-workspace/...
```

全部通过。证据覆盖无编号 Package、跨租户拒绝、两级限制、相同 command 重放、双 revision 冲突、原 Project／Resource Link ID 保留、attach 与安全 wrap 撤销、Package 被继续使用后的不安全撤销拒绝和零部分写入。

更新隔离 D1 的 Project 镜像、执行 migration 并让 IAM 重新加载 `project-workspace-capabilities@0.2.0` 后执行：

```bash
D1_ENV_FILE="$PWD/deployments/d1/.env" \
D1_PROJECT_NAME=omsd1c1 \
D1_PUBLIC_URL=http://127.0.0.1:18130 \
  acceptance/d1-c1-authorization/verify-running
```

2026-09-26 结构与权限基线结果为 134 个 HTTP 断言、0 个状态不匹配、最长本地请求 0.247285 秒。新增流程穿过 Nginx、Gateway、IAM 和 Project；只有显式 OA 业务管理员可管理层级，总经理、项目资源角色、旧 revision、第三层和不安全撤销均失败关闭。

### D1 C1 项目包页面与权限变化恢复

2026-09-27 先执行 Project 与 Gateway 普通回归：

```bash
cd backend/services/project-workspace
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...

cd ../gateway
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

Project 页面内嵌脚本另以 `node --check` 检查；随后使用 PostgreSQL 18 执行：

```bash
cd backend/services/project-workspace
TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55437/project_workspace?sslmode=disable' \
  GOWORK=off go test -race -count=1 -tags=integration ./...
```

普通测试、vet、build、脚本语法和 race 集成测试全部通过。集成回归还直接验证详情只在存在 active attach／wrap conversion 时返回该转换，安全撤销后不再返回。

重建 Project 与 Gateway D1 镜像并重启隔离组合后，使用同一正式命令再次执行 `acceptance/d1-c1-authorization/verify-running`。结果为 152 个 HTTP 断言、0 个状态不匹配、最长本地请求 0.263164 秒。新增证据包括：

- Project 客户页面和脚本经正式入口可读取，公开 Package 操作、Gateway 权限刷新且不暴露 HttpOnly Capability Cookie；
- OA 业务管理员、总经理、财务和资源级负责人得到与 Credential、Scope、状态一致的不同 `allowed_actions`；
- 撤销 `project-workspace.hierarchy.manage` 后重新授权，详情动作收敛且结构写命令返回 `403`；
- 撤销剩余 Project 入口能力后重新授权返回 `403`，Gateway 响应清除旧 Project Capability Cookie；
- 此前 C1 的责任、资源、状态、转换、幂等和不安全撤销断言继续通过。

该结果完成 J1-A27～J1-A28 的页面资源契约和正式权限恢复证据，不是像素级浏览器视觉回归，也不证明项目包金额分配、跨租户、真实公网／公司环境或 C2～C8。详细实验见 [OA C1 项目包、转换与权限恢复实验](./reports/oa-c1-project-package-experiment-20260926.md)。

## Party → Project D1 正式链、故障与联合恢复

2026-09-27 在独立本地 D1 组合中启用 Party Directory，公开入口只绑定 `127.0.0.1:18102`。该端口是本地验证入口，不代表真实 Ubuntu 或公网验收。

静态交付门禁执行：

```bash
make d1-image-mode-test \
  d1-image-promotion-test \
  d1-ingress-mode-test \
  d1-release-bundle-test
```

全部通过；摘要模式固定 12 个 D1 运行引用，发布侧固定 4 个基础制品与 10 个应用制品。

D1 schema 检查通过 IAM、Contract、Money、People、Party、Project 和 Evidence 的当前 migration。首管授权后执行破坏性验收：

```bash
D1_ENV_FILE=deployments/d1/.env \
D1_PUBLIC_URL=http://127.0.0.1:18102 \
D1_ACCEPTANCE_DISRUPTIVE=yes \
  acceptance/d1-delivery/verify-running
```

结果通过 Nginx 单入口、一次 IAM 登录、已启用应用读写、People 名册快照、Party→Project 精确引用和 Evidence 字节生命周期，且完成 Party 停机、失败关闭、无 Party Base Project 创建、既有快照读取、恢复和同命令幂等重放。IAM 审计证明 Project workload 只交换单 action、精确 `party:{id}` Scope 的短期 Credential，Party 作最终授权。

联合备份 `20260927T025800Z` 包含校验和清单、IAM 与签名材料、Contract、Money、People、Party、Project 和 Evidence 资产。在该专用合成 D1 上执行空目标还原后：

- Party 数量保持 2；
- Project 数量保持 4；
- 携 Party 快照的 Project 数量保持 3；
- 所有 schema 检查通过；
- 非破坏性 `acceptance/d1-delivery/verify-running` 再次通过。

本结果支持本地 D1 API 链；后续人类工作流实验已补齐 Party 管理页面、Workbench 条目和 Project 客户选择器。本段仍不代表 Contract／Money Party 迁移、完整双 Tenant 矩阵或真实环境已通过。详细证据见 [Party→Project D1 正式链实验](./reports/oa-party-project-d1-chain-experiment-20260927.md)和[Party 管理与 Project 客户选择实验](./reports/oa-party-project-human-workflow-experiment-20260927.md)。

## Party 管理与 Project 客户选择人类工作流

2026-09-27 在同一 D1 组合中完成 Party 管理页、Workbench 导航和 Project 新建客户选择。浏览器使用当前用户的 Party Directory 原生 Capability 读取 active 候选；Project 创建只提交 `party_id + source_revision`，服务端仍以 Project 工作负载身份向 IAM 交换单 action、精确 Party Scope 的短期凭证，再由 Party 最终复验并返回最小快照。

实际执行：

```bash
go test \
  ./backend/services/party-directory/... \
  ./backend/services/project-workspace/... \
  ./frontend/oms-workbench/... \
  ./backend/platform/gateway/...

sh -n acceptance/d1-delivery/verify-running \
  deployments/d1/ops/render-suite-manifest

deployments/d1/ops/d1 up
deployments/d1/ops/d1 check

D1_ACCEPTANCE_DISRUPTIVE=no acceptance/d1-delivery/verify-running
D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
```

两份页面脚本分别抽取并执行 `node --check`，全部通过。D1 非破坏性与破坏性验收均通过页面资源、Party active 列表、列表 `allowed_actions`、Workbench 条目、Project 选择器、精确快照、无 Party Base 立项、Party 停机失败关闭、既有快照读取和恢复后新命令单次成功。

当前 Mac 自动化表面仍报告宿主机锁定，因此本轮没有把可视点击或像素级检查记为通过。该限制不影响已经实际运行的 HTTP、脚本语法、授权、事务和故障恢复证据；可视点击应在自动化会话可读取桌面后补证。完整边界、步骤和限制见 [Party 管理与 Project 客户选择实验](./reports/oa-party-project-human-workflow-experiment-20260927.md)。

## Contract／Money Party 迁移预检

2026-09-27 为 Contract 与 Money 各增加只读 `party-migration-preflight --tenant-id` 运维命令。报告从来源应用自己的 PostgreSQL 读取本地 Party，输出来源 ID／revision、规范化事实 SHA-256、业务引用计数、重复登记标识复核信号和逐条未决人工决策；固定 `automatic_merge_allowed=false` 和 `ready_for_cutover=false`，不写 Party Directory、不保存映射、不改变运行时来源。

执行：

```bash
go test ./backend/services/contract-ledger/...
go vet ./backend/services/contract-ledger/...
go test ./backend/services/money-flow-ledger/...
go vet ./backend/services/money-flow-ledger/...
sh -n acceptance/d1-delivery/verify-running
git diff --check

deployments/d1/ops/d1 check
D1_ACCEPTANCE_DISRUPTIVE=no acceptance/d1-delivery/verify-running
```

全部通过。该轮真实 D1 两侧各读取 5 条 Party、0 条业务引用、5 条未决决策、0 个重复登记标识组和 0 个 blocker；后续 D1 映射专项创建验证样本后，两侧各为 6 条且仍为 0 条业务引用。每条摘要长度为 64，报告均 `ready_for_review=true`、`ready_for_cutover=false`。扩展后的 D1 验收同时通过单入口、一次登录、应用读写、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。

该结果只证明确定性预检和 D1 运维接线。当前样本没有被合同、义务或资金引用的 Party，兼容映射、目标 Party 精确验证、含引用数据保持、显式切换／回滚、Party 故障和完整双 Tenant 矩阵仍未通过。详细证据见 [Contract／Money Party 迁移预检实验](./reports/oa-party-ledger-migration-preflight-experiment-20260927.md)。

## Contract／Money Party 人工确认映射（BF-048）

2026-09-27 为 Contract 与 Money 分别增加不可变 `party_migration_mappings` 和 `party-migration-map --input`。审核文件固定来源预检摘要、目标 Party ID／revision、人工决定、审核人和理由；执行器用工作负载身份交换单 action、精确 Party Scope 的短期 Credential，读取目标 reference 后再在来源事务内锁定 Party 并重算摘要。输出固定 `ready_for_cutover=false`。

实际运行证据：

- 两个应用的 `go test ./...` 均通过；
- 临时 PostgreSQL 18.6 上，两个应用的 `go test -tags=integration ./internal/database ./internal/partymigration` 均通过；
- 集成用例覆盖首次追加、同命令同负载重放且不重复访问 Party、不可变 Trigger、来源名称／revision 漂移失败关闭和失败批次无残留；
- HTTP 用例覆盖精确路径／Bearer、同 Tenant active 精确 revision、stale revision 和 Party 503；
- `Dockerfile.ledger` 的 Contract 与 Money Linux 镜像均构建成功，并显式携带工作负载身份模块。

后续 D1 专项已经为两个台账装配独立工作负载策略／密钥，并对当前 6+6 条样本各人工确认 1 条映射到同一个 Directory Party revision 1；首次追加、等价重放、Command 冲突、目标 stale、Party 停机／恢复、IAM 精确 Scope 审计和数据库不可变保护均通过，结果持续为 `ready_for_cutover=false`。当前 12 条来源均无业务引用，其余 5+5 条保持未决；含引用数据、完整双 Tenant、显式切换和回滚仍不具备通过证据。详细步骤与限制见 [Contract／Money Party 人工确认映射实验](./reports/oa-party-ledger-reviewed-mapping-experiment-20260927.md)和[Contract／Money Party D1 人工确认映射实验](./reports/oa-party-ledger-d1-reviewed-mapping-experiment-20260927.md)。

## Billing / Invoice 核心与 PostgreSQL 事实层

2026-09-27 建立独立 `backend/services/billing-invoice` Go 模块、PostgreSQL migration、领域服务与 Repository。当前投影、完整 revision、Command 结果和业务审计在同一事务提交；revision、command 和 audit 由数据库 Trigger 拒绝更新或删除，延迟一致性约束保证当前投影等于最新 revision。

普通验证：

```bash
go test ./backend/services/billing-invoice/...
go test -race ./backend/services/billing-invoice/...
go vet ./backend/services/billing-invoice/...
```

PostgreSQL 18.6 Alpine 的一次性测试数据库验收：

```bash
TEST_DATABASE_URL='postgres://billing_invoice:***@127.0.0.1:55439/billing_invoice?sslmode=disable' \
  make billing-invoice-core-acceptance
```

上述检查全部通过。真库证据覆盖登记 revision 1、资料纠正、Evidence 引用替换、登记无效、当前详情／历史／只读引用、跨 Tenant 隐藏、非法原地篡改拒绝、相同 Command 重放跳过外部依赖、负载／Actor 冲突、旧 Expected Revision、外部引用失败零本地事实，以及并发相同创建命令只产生一条登记、一条 revision、一条 command 和一条 audit。金额使用正整数 minor units，核心要求非空允许币种集合；参考切片只配置 `CNY`，未启用币种失败关闭。

本轮没有实现或验证 HTTP API、Capability Manifest、IAM／Policy、Nginx／Gateway、Suite Manifest、D1、非 root 镜像、备份／空目标恢复、Billing 页面或 C3 只读组合，因此不能据此宣称 Billing 可部署或 C3 已通过。

### Billing 目标端 IAM 与金额权限视图

同日增加 `billing-invoice@0.1.0` Capability Manifest、Ed25519 IAM Credential 验证器、模板人类凭证、可信身份 Header 清洗中间件，以及当前登记／历史 revision 的服务端权限视图。执行：

```bash
go test -race ./backend/services/billing-invoice/...
go vet ./backend/services/billing-invoice/...
go test ./acceptance/access-contract/...
go vet ./acceptance/access-contract/...
```

全部通过。证据覆盖正确 Billing Audience／Tenant／TTL／Action／Scope，错误 Audience、跨 Tenant、过期、未知宽权限、重复 Action 和非法资源类型失败关闭；模板人类凭证不含工作负载专用 reference action；客户端伪造身份 Header 在鉴权前删除；Project A、错误 Project B、精确 Invoice Registration 和跨 Tenant Scope 分别按预期允许或拒绝；无 `amount.read` 时当前与历史金额／币种均为 `null` 且 `amount_visible=false`，有权限时返回精确 minor units；登记无效后纠正、登记无效和 Evidence 管理动作从服务端 `allowed_actions` 收敛。

本段仍不表示完整 Billing HTTP 路由、IAM／Policy 签发侧、Gateway、D1 或正式角色矩阵已通过；当前只证明目标应用可复用的鉴权与响应投影基础。

### Billing 类型化来源 reference-read 就绪度审计

2026-09-27 在实现 Billing HTTP resolver 前，对 Party、Project、Contract、Settlement、Money Obligation 与 Evidence 的实际 reference 路由、响应模型和 IAM Scope 逐项执行静态交叉审计。结果不是“接口都存在”：

- Party 与 Evidence 已接受精确资源 Scope，并有目标端相邻资源拒绝证据；
- Project 已接受精确 Project Scope，但当前引用响应没有 Billing 要冻结的稳定显示摘要；
- Contract 与 Money 的 IAM Provider 明确只接受 Tenant Scope，不能消费 Billing 所要求的精确 Contract／Settlement／Obligation Credential；
- Contract 没有 Settlement reference endpoint；Contract／Money 引用响应均缺少稳定来源摘要。

因此当前类型化 C3 来源链判定为 `not-ready`。没有通过给 Billing 签发 Tenant Scope 万能凭证、读取相邻数据库、复用 ORM、以 Contract 代替 Settlement 或由 Billing 拼接摘要来绕过。该审计不包含运行时 HTTP、签发、Gateway、D1 或页面证据；完整步骤、直接文件证据和收敛条件见 [OA C3 Billing 来源引用就绪度审计](./reports/oa-c3-billing-source-reference-readiness-audit-20260927.md)。

### Billing 上游精确 reference-read 兼容层

同日按上述审计逐项关闭目标应用侧缺口：Project reference 追加由 Project 自己提供的稳定名称；Contract／Money 发布工作负载专用 `reference.read`，只接受一个精确 Contract／Settlement 或 Obligation／Transaction Scope；Contract 新增独立 Settlement reference endpoint；所有最小投影返回 ID、Tenant、摘要、状态与 revision。不可变 Settlement 和 Transaction 的记录自身固定为 revision 1，不把 Contract source revision、结算 sequence 或 Money 内部事实转移给 Billing。

实际执行：

```bash
go test ./backend/services/project-workspace/internal/auth \
  ./backend/services/project-workspace/internal/projects \
  ./backend/services/project-workspace/internal/httpserver \
  ./backend/services/contract-ledger/internal/contract \
  ./backend/services/contract-ledger/internal/httpserver \
  ./backend/services/contract-ledger/internal/identity \
  ./backend/services/money-flow-ledger/internal/httpserver \
  ./backend/services/money-flow-ledger/internal/identity \
  ./backend/platform/identity-access/internal/capability \
  ./acceptance/access-contract

go test ./backend/services/project-workspace/... \
  ./backend/services/contract-ledger/... \
  ./backend/services/money-flow-ledger/...

(cd acceptance/contract-ledger-demo && GOWORK=off go test ./...)
(cd acceptance/money-flow-demo && GOWORK=off go test ./...)
(cd acceptance/project-workspace-demo && GOWORK=off go test ./...)
```

相同目标包的 `go test -race` 与三个应用、IAM Capability Catalog、Access Contract 的 `go vet` 也已通过；两个 Capability Manifest 可被 JSON 严格解析，三个既有独立 Demo 驱动测试及 `git diff --check` 通过。Demo 回归没有重跑完整 fresh-state／run／replay／cleanup。正反例覆盖 exact resource 允许、adjacent resource、错误资源类型、Tenant `view` 冒充 reference action、错误 action 和跨 Tenant 拒绝，并保留三个应用既有普通回归。当前状态提升为 `target-reference-compatible`，不等于 Billing 已消费这些端口；本轮没有运行 Billing 工作负载交换、resolver、真库来源快照、Gateway、浏览器、D1 或恢复。详细步骤和限制见 [OA C3 上游精确引用兼容实验](./reports/oa-c3-source-reference-compatibility-experiment-20260927.md)。

### Billing 消费侧工作负载身份与精确 HTTP resolver

2026-09-27 为 Billing 增加最小权限工作负载身份、共享 IAM TokenSource 兼容接口和 Party／Project／Contract／Settlement／Money Obligation／Evidence 六类类型化 resolver。每次交换固定目标 Audience、单 action 和单精确资源 Scope；resolver 严格复验 Tenant、资源类型、ID、revision、允许状态与非空摘要。动态凭证首次收到 401 时只失效并重取该资源一次；404／422 与过期引用归类为无效输入，IAM／网络／拒绝／畸形响应与服务异常归类为依赖不可用。

实际执行：

```bash
go test ./backend/services/billing-invoice/... \
  ./backend/platform/identity-access/internal/workload \
  ./acceptance/access-contract \
  ./packages/workload-identity/...

go test -race ./backend/services/billing-invoice/internal/invoice \
  ./backend/services/billing-invoice/internal/references \
  ./backend/platform/identity-access/internal/workload \
  ./acceptance/access-contract \
  ./packages/workload-identity/...

go vet ./backend/services/billing-invoice/... \
  ./backend/platform/identity-access/internal/workload \
  ./acceptance/access-contract \
  ./packages/workload-identity/...

jq empty contracts/access/v1/examples/workload-identity-policy.json
git diff --check
```

上述检查全部通过。证据覆盖六类精确 Credential 请求、跨 Tenant、错误类型、陈旧 revision、不可用状态、空摘要、Evidence version、首次 401 失效重取、IAM 交换失败和无效／不可用错误分离；领域恢复用例证明依赖失败零写入、使用同一 Command ID 恢复后只写一次、成功重放不再访问已故障依赖。

该轮没有启动真实 IAM 进程、Billing HTTP Server、Gateway／Nginx、PostgreSQL 集成标签、浏览器、D1 或恢复演练，因此只证明消费组件可装配，不代表 C3 正式入口已经通过。详细证据和下一步见 [OA C3 Billing 消费侧精确引用实验](./reports/oa-c3-billing-consumer-reference-experiment-20260927.md)。

### Billing 最小受保护 HTTP 用户切片

2026-09-27 将 Billing 原生 IAM Provider、PostgreSQL Store、领域服务和类型化来源 resolver 装入同一 HTTP 请求链，开放创建、详情和历史。创建复验 `billing-invoice.record` 与 Tenant／Project Scope；Project Scope 要求至少一个 Project 来源且每个 Project 来源均获授权。详情和历史复验各自动作并由 Billing 对精确登记／Project Scope 最终授权；当前与历史金额统一在服务端按 `billing-invoice.amount.read` 收敛。客户端伪造的 Actor、Tenant、allowed actions 与 Scope Header 在鉴权前删除。

实际执行：

```bash
gofmt -w \
  backend/services/billing-invoice/internal/httpserver/server.go \
  backend/services/billing-invoice/internal/httpserver/server_test.go \
  backend/services/billing-invoice/internal/httpserver/server_integration_test.go

go test ./backend/services/billing-invoice/...
go test -tags=integration ./backend/services/billing-invoice/internal/httpserver

TEST_DATABASE_URL='postgres://billing_http:***@127.0.0.1:55440/billing_http_test?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./backend/services/billing-invoice/internal/httpserver \
  ./backend/services/billing-invoice/internal/invoice
```

全部通过。真库用例在一次性 PostgreSQL 18 容器的隔离 schema 中运行，结束后容器已停止并删除。它使用临时 Ed25519 Key Set 和真实 `auth.IAMProvider`，依次验证来源 503 时 registrations／revisions／commands／audits 全为 0、来源恢复后同 Command 单次创建、来源再次停止时成功重放仍不访问依赖、相邻角色拒绝、Project B 拒绝、错 Tenant 拒绝、Project 错状态 422、Project 精确工作负载交换，以及当前／历史金额的无泄漏投影。实现复核又增加多 Project 输入负例，确保 Project A Scope 不能同时写入 Project B 来源。

本段只证明 `billing-protected-http-minimum-compatible`。列表、纠正、登记无效、Evidence 管理、正式 IAM／Policy 签发、Nginx→Gateway、浏览器、D1、备份恢复和 C3 组合均未运行；详细边界见 [OA C3 Billing 最小受保护 HTTP 实验](./reports/oa-c3-billing-protected-http-experiment-20260927.md)。

### Billing 完整本地生命周期 HTTP 与可运行进程

2026-09-27 后续切片补齐列表、纠正、登记无效、Evidence 管理和工作负载只读引用，并新增配置失败关闭的 `cmd/server`。Project Scope 列表必须携带匹配 `project_id`；来源纠正的所有 Project 引用均须在 Scope 内；工作负载引用只接受 IAM 单 action、单精确登记 Scope。模板静态依赖 Token 与 IAM 工作负载交换互斥，Party、Project、Contract、Money、Evidence URL 和允许币种均由配置注入。

实际通过：

```bash
go test ./backend/services/billing-invoice/...
go test -race ./backend/services/billing-invoice/...
go vet ./backend/services/billing-invoice/...
go test ./acceptance/access-contract ./packages/workload-identity/...
go vet ./acceptance/access-contract ./packages/workload-identity/...

TEST_DATABASE_URL='postgres://billing_http:***@127.0.0.1:55442/billing_http_test?sslmode=disable' \
  go test -race -count=1 -tags=integration \
  ./backend/services/billing-invoice/internal/httpserver \
  ./backend/services/billing-invoice/internal/invoice
```

真库使用临时 PostgreSQL 18.6 容器，结束后已停止并删除。联合链验证创建后列表有效汇总为 12345，Project B 来源纠正被拒且零写入，合法纠正、Evidence 引用、登记无效依次形成 revision 2、3、4，历史保留四条快照，精确工作负载引用返回 revision 3 摘要，登记无效后有效汇总为 0。普通 HTTP 矩阵另覆盖宽 Scope／多 action 工作负载引用拒绝、列表与汇总金额收敛和无 Project 查询拒绝。

本段只证明 `billing-local-lifecycle-runtime-compatible`。正式 IAM／Policy 签发、Nginx→Gateway、浏览器、非 root 镜像、release、Suite Manifest、D1、备份恢复和 C3 组合仍未运行；详细边界见 [OA C3 Billing 生命周期与运行时实验](./reports/oa-c3-billing-lifecycle-runtime-experiment-20260927.md)。

### Billing D1 正式入口、故障与联合恢复

2026-09-27 将 Billing 作为 API-only 应用装入隔离 D1：独立 PostgreSQL、migration、工作负载密钥、Suite Manifest、Gateway `/billing` 路由、release／publish／promote、联合备份与恢复均使用同一发布物。实际构建的 `linux/amd64` 镜像以 `postgres` 用户运行，容器内 UID／GID 为 `70/70`；服务只加入内部 app／data 网络，不发布宿主端口。

实际通过：

```bash
go test ./backend/services/billing-invoice/... ./acceptance/access-contract/... ./packages/workload-identity/...
go vet ./backend/services/billing-invoice/... ./acceptance/access-contract/... ./packages/workload-identity/...
sh deployments/d1/ops/test-image-mode
sh deployments/d1/ops/test-release-bundle
sh deployments/d1/ops/test-upgrade-selection
sh deployments/d1/ops/test-ingress-mode
deployments/d1/ops/publish-images self-test
deployments/d1/ops/promote-images self-test
acceptance/d1-delivery/verify-running
D1_ACCEPTANCE_DISRUPTIVE=yes acceptance/d1-delivery/verify-running
deployments/d1/ops/d1 backup
D1_CONFIRM_EMPTY_RESTORE=yes deployments/d1/ops/d1 restore 20260927T114829Z
docker buildx build --platform linux/amd64 --load -f deployments/d1/docker/Dockerfile.billing .
```

正式入口从 Nginx 完成一次登录，经 Gateway 清理并派生可信身份，由 IAM／Policy 签发 Billing 原生 Credential，Billing 最终复验 Tenant、Audience、Action、Scope、状态与 Expected Revision；Billing 再以自身工作负载身份读取精确 Party、Project 与 Evidence。验收覆盖登记、纠正、Evidence 关联、登记无效、历史、Project Scope 列表与相邻 Project 拒绝。停止 Party 时新登记返回 `503` 且登记和命令均零写入，恢复后以原 Command ID 只提交一次。

备份 `20260927T114829Z` 的 checksum 全部通过；销毁该隔离 Compose 项目的数据卷并恢复到空卷后，Billing 恢复前后的 `current_count|current_revision_sum|revision_count|audit_count|projection_digest` 均为 `3|9|9|9|27e791a0dd194d69a0dcea9caa459f3a`，恢复后的完整正式入口验收再次通过并继续写入。

本段证明 `billing-d1-formal-entry-recovery-compatible`，不证明 Billing 人类页面、C3 Contract／Money／Billing 分段只读组合、完整角色／撤权／账号停用矩阵、Billing-only、双 Tenant 或真实 Ubuntu／公网环境。详细证据见 [OA C3 Billing D1 交付与恢复实验](./reports/oa-c3-billing-d1-delivery-recovery-experiment-20260927.md)。

### Billing 人类工作区、撤权收敛与故障恢复

2026-09-27 Billing 根路径新增可操作工作区，并在 Suite Manifest 启用时显示于 Unified Workbench。页面覆盖列表、筛选、新建、详情、纠错、Evidence 引用集合、历史与登记无效；动作直接投影服务端 `allowed_actions`。目标应用同时补上字段级门禁：没有 `billing-invoice.amount.read` 时，即使具备 `billing-invoice.correct` 也不能盲写 `amount_minor` 或 `currency`。

实际通过：

```bash
go test ./backend/services/billing-invoice/... ./frontend/oms-workbench/...
go vet ./backend/services/billing-invoice/... ./frontend/oms-workbench/...
sh -n acceptance/d1-delivery/verify-running deployments/d1/ops/render-suite-manifest
# 从 demo_workspace.go 提取同源脚本后执行 node --check

D1_ENV_FILE=deployments/d1/.env D1_PROJECT_NAME=omsbizfoundry \
  deployments/d1/ops/d1 check
D1_ENV_FILE=deployments/d1/.env D1_PROJECT_NAME=omsbizfoundry \
  acceptance/d1-delivery/verify-running
D1_ACCEPTANCE_DISRUPTIVE=yes D1_ENV_FILE=deployments/d1/.env \
  D1_PROJECT_NAME=omsbizfoundry acceptance/d1-delivery/verify-running

deployments/d1/ops/test-image-mode
deployments/d1/ops/test-release-bundle
deployments/d1/ops/test-upgrade-selection
deployments/d1/ops/test-ingress-mode
```

两次 D1 验收均从 Nginx 正式入口执行。非破坏矩阵验证 Workbench 菜单、页面与脚本、完整登记生命周期、stale revision 零改写，以及撤销金额能力后当前详情／四条历史全部脱敏、盲写金额返回 `403`、恢复能力后金额重新可见。破坏矩阵验证 Party 停机时新登记失败关闭且零写入，恢复后新命令只提交一次，并继续通过组合原有故障隔离回归。

本段证明 `billing-human-workspace-d1-compatible`；C3 分段只读组合、完整角色／账号停用／双 Tenant、Billing-only、真实 Ubuntu 和完整浏览器尺寸／辅助技术矩阵仍未证明。详细证据见 [OA C3 Billing 人类工作流实验](./reports/oa-c3-billing-human-workflow-experiment-20260927.md)。

## OA C3 Project 经营链路 D1 组合（2026-09-27 已通过）

实现和静态门禁：

```bash
go test ./backend/services/project-workspace/... ./backend/platform/identity-access/internal/policy ./acceptance/access-contract
node --check /tmp/project-demo-script.js
sh -n acceptance/d1-c3-commercial-summary/verify-running
git diff --check -- deployments/d1/compose.yaml acceptance/d1-c3-commercial-summary backend/services/project-workspace contracts arckit docs
```

D1 正式入口与故障矩阵：

```bash
C3_ENABLE_FAULTS=true \
  C3_EVIDENCE_DIR=/tmp/oa-c3-evidence \
  D1_PROJECT_NAME=omsbizfoundry \
  D1_PUBLIC_URL=http://127.0.0.1:18102 \
  acceptance/d1-c3-commercial-summary/verify-running
```

验收从 Nginx → Gateway → IAM → Project 正式入口建立真实 Project、Contract、receivable Obligation、receipt Transaction 和 Invoice Registration，再以四条 Project Resource Link 形成只读经营分段。IAM 审计证明 Project 每次只申请目标 audience、单 `reference.read` action 和单资源 Scope；目标应用再次复验 Tenant／类型／ID。Project 不保存来源金额，也不跨语义合计。

最终结果为 **100 个 HTTP 断言、0 个状态不匹配**。总经理与财务可读取，只有普通 Project read 的生产经理由 Project 返回 403；撤权后旧凭证只在既定 TTL 内继续，新签发后 403，恢复 Grant 后 200，账号停用后 Session／新签发 401。Contract、Money、Billing 分别停机时只有相应 segment 为 `unavailable`，IAM 停机且清空 Project 凭证缓存后四段均降级；恢复后四段重新 `available`，Project 保持 revision 6，未产生组合写入。

30 次读取的并行组合 p50/p95 为 **13/220 ms**，同批分段耗时的串行等价 p50/p95 为 **19/362 ms**，低于本地 D1 800 ms 门槛。该数字只描述本机样本，不是容量承诺。

本轮先后发现并修复 Project D1 的 Contract／Money／Billing 内部端口 `8080/8080/8092` 与实际 `8081/8083/8091` 不一致，以及运行容器集合与旧 Suite Manifest enabled 集合不一致的问题。后者证明 D1 检查必须同时比较容器和 Manifest，不能只看健康容器。验收 TSV 汇总器也改为显式制表符分隔，并完整重跑取得 100／0。

页面资源、页面脚本与权限显隐已有自动化测试；最终真实点击时 macOS 自动化通道仍报告系统锁定，因此 Workbench → Project → 经营链路 → 四卡片 → 来源下钻未计为通过。完整双 Tenant、真实 Ubuntu、Billing-only、来源 revision 变化的人类恢复以及 Settlement／应开未开／自动业务动作仍是后续证据。详细步骤、业务 ID、角色矩阵、故障表和架构判定见 [OA C3 Project 经营链路 D1 实验报告](./reports/oa-c3-commercial-summary-d1-experiment-20260927.md)。OA 完整体继续为 `insufficient-evidence`。

## OA C4-A11 consultation-only 最小组合（2026-09-27 已通过）

实际执行：

```bash
(cd acceptance/reference-oa-demo && GOWORK=off go test ./...)
(cd backend/services/project-workspace && GOWORK=off go test ./...)

TEST_DATABASE_URL='postgres://project_workspace:***@127.0.0.1:55499/project_workspace?sslmode=disable' \
  GOWORK=off go test -count=1 -tags=integration \
  ./backend/services/project-workspace/internal/projects

sh -n deployments/local-oa/ops/local-oa
docker compose --env-file deployments/local-oa/.env \
  -f deployments/local-oa/compose.yaml \
  -f deployments/local-oa/compose.audit-ipam.yaml config --quiet
./deployments/local-oa/ops/local-oa composition-audit-consultation
```

全部通过。Project 真库回归证明 `project_type_keys=[consultation]` 不再被错误的 `design` 硬编码阻挡，同时保留至少一个类型、经营负责人和生产负责人的立项门槛。空卷组合只运行 People、Evidence、Project、Production 及四个独立 PostgreSQL；Contract 与 Money 未运行。公开 API 完成 consultation-only Project revision 4、Evidence version 1 和 `consultation-v1` Work Item revision 5，canonical Reference 无完成缺口、历史为 5 条，旧 Design 详情返回 404、旧列表为 0。

同一命令完整复跑后 Project、Evidence、Work Item ID 和 revision 均不变，状态文件 SHA-256 两次同为 `27ee80bd9f48058179da2973bd27382bdcece850a79bcbe9632241a2ea0f9777`。随后 `composition-audit` 的 design-only 组合也重跑通过，canonical Production 列表断言只有 `design-v1`，两次状态 SHA-256 同为 `f7baedf1acba990dbb42b95255fe7c877e2c63dae23b25f55e80fb4f8d8076a0`。Docker 默认地址池已被其他项目网段占满；验收使用审计专用固定 `10.252.20.0/24`～`10.252.26.0/24`，没有执行全局 network prune，结束时只删除自身 Compose 项目、卷和状态文件。

该证据关闭 C4-A11，并双向覆盖 C4-A12 的本地数据/API 缺席，但 C4-A12 仍因正式 Suite Manifest／Workbench 导航权限与恢复未验而保持部分通过。它使用真实模板鉴权但未经过 Nginx→Gateway→IAM，也未验证正式 Capability 签发／撤权、release bundle、备份恢复、D1、浏览器或真实 Ubuntu，因此 OA 完整体仍为 `insufficient-evidence`。详细边界见 [C4 多生产类型责任边界审计](./reports/oa-c4-multi-production-boundary-audit-20260927.md)。

## OA C4-A13 聚合回归（2026-09-28 已通过）

实际执行 `make check`、`./deployments/demos/ops/level-a audit --confirm-destroy-demo-data`、`level-a status`，以及当前本地 OA 的 `up → demo → demo → failure-isolation → verify → status`。六个独立 Demo 均从 fresh-state 完成 run、replay 和 cleanup；Design 的复杂纠错旅程两次均为 revision/history `21/21`，六项 Runtime gate 全部 `passed`、Business capability gate 全部 `complete`。

完整 OA J1～J4 在保留数据库上连续两次通过，Project `426b18f8-ec92-4e97-8519-215dc64adf7f` 均保持 active revision 9、五条稳定引用和两条不可变 Assignment 快照，状态 SHA-256 为 `631f231a797dfc8c2b76c94b9d238fda10355c934278205f615b6e78792e26d9`。Contract、Money、People、Evidence、Design 逐项停机时，Project 仍可读且只把相应引用标记 unavailable；全部恢复后六应用均 healthy。

回归实际发现并修复四类验证装配漂移：People 页面文案断言、Project Docker build context、Project Contract reference stub 版本和 Docker 审计网络地址池；同时修复参考 OA 固定命令号跨协议版本复用、Project 业务编号冲突，以及 Assignment 写死 People revision 的问题。没有放松幂等冲突、公开契约或目标应用授权。C4-A13 因此通过；正式 Suite Manifest／Workbench／恢复、Nginx→Gateway→IAM、D1、浏览器和真实 Ubuntu 仍未由本节证明。

## OA C4-A12 正式最小组合与恢复（2026-09-28 已通过）

实际执行：

```bash
acceptance/d1-c4-production-authorization/run-isolated run

D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 backup

D1_CONFIRM_EMPTY_RESTORE=yes \
D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 restore 20260927T181130Z

acceptance/d1-c4-production-authorization/run-isolated verify-restored

go test ./backend/services/design-operations/... ./backend/platform/identity-access/... \
  ./backend/platform/gateway/... ./frontend/oms-workbench/... ./acceptance/access-contract/...

deployments/d1/ops/test-image-mode
deployments/d1/ops/test-ingress-mode
deployments/d1/ops/test-release-bundle
deployments/d1/ops/test-upgrade-selection
deployments/d1/ops/publish-images self-test
deployments/d1/ops/promote-images self-test
```

全部通过。隔离 D1 只启用 People、Project、Evidence、Production；Workbench 和路由中未选 Contract、Money、Party、Business Engagement、Billing 均缺席。正式入口取得 **49 个 HTTP 断言、0 个状态不匹配**：生产负责人获单 Project Scope，经营负责人和相邻 Project 拒绝；Evidence 及 IAM 停机均为 503、零部分写入，恢复后原 Command 单次成功。

联合备份包含 IAM dump／签名密钥、People、Project、Evidence、Production 及选择／校验清单。销毁隔离卷后恢复，consultation Work Item 仍为 completed revision 5、一个成果、5 条历史；canonical SHA-256 恢复前后同为 `592c166db157ee72840f198b3496cab5a8171d86daca08df4259b66645db4e7b`，design 与 IAM 故障恢复产生的记录也保持。恢复演练同时修复 Docker stale edge-network 的有界重试问题。

本段关闭 C4-A12，并证明 C4-A10 的正式入口主矩阵；该轮当时尚未代替 Production 专项权限生命周期、双 Tenant 或真实 Ubuntu，其中权限生命周期与真实浏览器已由下一节补齐。详细证据见 [OA C4 正式最小组合与恢复实验](./reports/oa-c4-formal-minimum-composition-experiment-20260928.md)。

## OA C4 Production 权限生命周期与真实浏览器（2026-09-28 已通过）

`acceptance/d1-c4-production-authorization/run-isolated verify` 在同一隔离 D1 取得 **86 个 HTTP 断言、0 个状态不匹配**。目标应用列表和详情均返回 `allowed_actions`；撤销 `production-work-item.stage.manage` 后，旧凭证在签名残余窗口内可读，显式刷新后新凭证和页面同时删除该动作，直接写入返回 `403`；恢复 Grant 后动作恢复。停用账号后，Suite Session、新授权和重新登录返回 `401`，Gateway 清除应用 Cookie；重新启用后只有新 Session／Credential 可恢复。Assignment 结束后新 Policy Credential 拒发，而已签发凭证明确只保留最长 300 秒残余窗口。

真实浏览器已从统一登录页经 Workbench 进入 Production，直接观察到 `stage / manage` 在撤销／恢复后消失／出现，账号停用后页面显示权限恢复区并退出提交态，重新启用后必须登录与授权。新的联合备份 `20260927T190404Z` 销毁隔离卷后恢复成功，consultation Work Item 保持 completed revision 5、一个成果和 5 条历史，canonical SHA-256 为 `1b74b9a950f2e58e156a9ce75563d5ca922e9b0b64aa54281ffad434c7b89241`。

本段关闭 Production 专项 Grant 撤销、账号停用、Assignment 结束和真实浏览器补证；不替代完整双 Tenant D1 交叉矩阵或真实 Ubuntu／公网环境。详细证据见 [OA C4 Production 权限生命周期与真实浏览器实验](./reports/oa-c4-production-permission-browser-experiment-20260928.md)。

## OA C5 批复审计与项目完成责任边界审计（2026-09-28 已完成）

本轮只读对照参考 OA 的 `production/approval.md`、`ProductionApproval.yaml`、当前模型／服务、`ApprovalAuditView`／`ApprovalAuditModal` 以及 `oa2026.GetProgress`。审计确认页面位置不能决定数据所有权，并记录五类直接矛盾：审计页面宣称引用批复但后端重读合同、金额覆盖原因未强制、`float64` 与客户端合计、原地更新／物理删除，以及仅凭两类记录存在就在 GET 中写入项目完成状态。

当前接受 BF-058：独立 Approval & Audit Results 拥有外部结果、精确金额、来源 revision 快照、Evidence 业务引用、纠正／撤回和不可变历史；批复默认引用 Contract／Settlement，审计默认引用批复结果。Project 完成由版本化 Completion Profile 纯读取评估，最终状态变化只能使用显式命令。C5-A01～A15 已固定，覆盖独立所有权、两条来源链、精确金额、覆盖原因、Evidence 故障、追加纠错、权限、完成评估、最小组合、未选缺席、D1 恢复和既有回归。

该静态审计本身没有运行应用测试；其后续领域核心实验已经新增独立 module、migration、Store 和运行测试，见下一节。边界审计仍只证明“责任边界和验收可执行”，不单独代表 C5 通过。详细证据见 [OA C5 批复审计责任边界审计](./reports/oa-c5-approval-audit-boundary-audit-20260928.md)。

## OA C5 Approval & Audit Results 领域核心实验（2026-09-28 部分通过）

新增 `backend/services/approval-audit-results`，实现固定 `reference-oa-v1` 三项整数金额、Project／Contract／Evidence 精确引用端口、approval／audit 当前投影、完整 revision、Command、Audit、纠正、撤回和撤回后重新登记。审计的默认来源由本地 Store 读取指定 approval 历史 revision；单元测试直接确认该路径不调用外部 Contract source resolver。

实际执行：

```text
go test -count=1 ./backend/services/approval-audit-results/...
go vet ./backend/services/approval-audit-results/...
TEST_DATABASE_URL=postgres://…/postgres18-disposable make approval-audit-results-core-acceptance
```

三项均通过。PostgreSQL 18.4 真库覆盖同 Project 两种结果、批复后续更正不改变既有审计、人工覆盖原因、stale revision、撤回后沿同一 ID 重登、四 revision 历史、跨 Tenant not found、revision／current 防篡改和物理删除拒绝；八并发同 Command 最终只有一组 current／revision／command／audit；Evidence 引用失败时四表均为 0。当前判定 `core-supported / protected-entry-and-composition-not-started`，只覆盖 C5-A01～A08 的核心子集与 C5-A15 局部反耦合，不代表 HTTP／IAM、真实 adapter、Project 完成、页面、D1 或恢复通过。详细步骤与证据见 [OA C5 Results 领域核心实验](./reports/oa-c5-results-core-experiment-20260928.md)。

## OA C5 受保护 HTTP、IAM 与精确引用实验（2026-09-28 部分通过）

Results 已增加版本化登记／详情／历史／纠正／撤回 HTTP、Ed25519 IAM Provider、模板 Provider、原生 Capability Manifest、参考 OA Role Overlay，以及 Project／Contract／Evidence HTTP resolver。Project reference-read 增加项目编号和活动 Resource Link；Contract／Settlement reference-read 增加类别、父合同、币种与精确金额。`reference-oa-v1` 当前严格使用一个已链接来源，把类别映射到三项金额中的一项；多来源聚合仍未证明。

实际执行：

```text
go test -count=1 ./backend/platform/identity-access/internal/capability ./backend/services/approval-audit-results/... ./backend/services/project-workspace/internal/projects ./backend/services/project-workspace/internal/references ./backend/services/contract-ledger/internal/httpserver
TEST_DATABASE_URL=postgres://…/postgres18-disposable make approval-audit-results-protected-http-acceptance
TEST_DATABASE_URL=postgres://…/postgres18-disposable go test -race -count=1 -tags=integration ./backend/services/approval-audit-results/internal/httpserver ./backend/services/approval-audit-results/internal/results
```

全部通过，实际数据库版本为 PostgreSQL 18.6，受保护 HTTP／Results 真库矩阵通过 race detector。矩阵覆盖 401／403／409／422／503、错 Audience／Tenant／Project、相邻角色、金额与 Evidence 独立权限、金额字段脱敏、Project 未链接或 stale source、Command 载荷冲突、Evidence 停机零写入、恢复后原 Command 单次生效，以及成功 Command 在依赖再次停机后的本地重放。IAM Capability Catalog 实际加载 Results Manifest，三类 resolver 请求精确 audience／action／resource credential。

当前判定 `protected-http-and-reference-adapters-supported / cross-process-composition-not-tested`。它补充 C5-A03、A07、A09 的后端直接证据，但 reference 服务器仍是同进程契约替身；其后人类页面和 Project 完成检查点已由下一节补齐，真实独立进程最小组合、未选缺席、D1、恢复和完整回归仍未验，不能声明 C5 通过。详细证据见 [OA C5 受保护 HTTP、IAM 与精确引用实验](./reports/oa-c5-protected-http-reference-experiment-20260928.md)。

## OA C5 人类工作区与 Project Completion Profile 实验（2026-09-28 部分通过）

Results 已增加按 Project 的当前结果列表、最小项目 reference-read 和人类工作区；Project 已增加 `completion.read`／`completion.confirm`、Results 精确工作负载引用、`reference-oa-c5-v1` 只读评估、不可变显式确认和 drift。页面明确 C5 检查点不等于完整 OA Project 完成。

实际执行：

```text
cd backend/services/project-workspace && go test ./...
TEST_DATABASE_URL=postgres://…/postgres18-disposable go test -count=1 -tags=integration ./internal/projects ./internal/database
cd backend/services/approval-audit-results && go test ./...
TEST_DATABASE_URL=postgres://…/postgres18-disposable go test -count=1 -tags=integration ./internal/results ./internal/httpserver
```

全部通过，实际数据库版本 PostgreSQL 18.6。验证覆盖 GET 零写入、`satisfied／pending／unavailable`、精确 Scope、权限收敛、确认只追加 Project revision／confirmation／history／command、Project 保持 active、stale revision／source 失败关闭、确认防篡改、来源变化 drift，以及成功 Command 在 Results 不可用时先本地回放。当前判定 `human-workspace-and-project-checkpoint-supported / cross-process-composition-not-tested`；C5-A10／A11 当前本地切片成立，但真实多进程最小组合、未选缺席、Suite Manifest、Workbench、正式入口、联合恢复和 C6／C7 仍未验。详细证据见 [OA C5 人类工作区与 Project Completion Profile 实验](./reports/oa-c5-human-completion-profile-experiment-20260928.md)。

## OA C5 D1 正式装配与恢复实验（2026-09-28 部分通过）

Results 已接入 D1 的独立 PostgreSQL、migration、workload key、Capability Manifest、Suite Manifest、Workbench、Gateway、镜像发布／提升和联合备份恢复。它显式声明 `approval-audit-results` audience 与 `approval-audit-result` Capability namespace；缺少 namespace 时 Gateway 失败关闭，补齐后才能启动。

实际执行：

```bash
make d1-image-mode-test d1-upgrade-selection-test d1-release-bundle-test d1-image-promotion-test
docker build --file deployments/d1/docker/Dockerfile.approval-audit-results --tag omsbizfoundry/approval-audit-results:c5-validation .
D1_COMPOSE_OVERRIDE_FILE=/tmp/c5/networks.yaml D1_ENV_FILE=/tmp/c5/d1.env D1_PROJECT_NAME=omsd1c5 D1_PUBLIC_URL=http://127.0.0.1:18150 deployments/d1/ops/d1 up
D1_COMPOSE_OVERRIDE_FILE=/tmp/c5/networks.yaml D1_ENV_FILE=/tmp/c5/d1.env D1_PROJECT_NAME=omsd1c5 D1_PUBLIC_URL=http://127.0.0.1:18150 acceptance/d1-delivery/verify-running
D1_COMPOSE_OVERRIDE_FILE=/tmp/c5/networks.yaml D1_ENV_FILE=/tmp/c5/d1.env D1_PROJECT_NAME=omsd1c5 deployments/d1/ops/d1 backup
D1_CONFIRM_EMPTY_RESTORE=yes D1_COMPOSE_OVERRIDE_FILE=/tmp/c5/networks.yaml D1_ENV_FILE=/tmp/c5/d1.env D1_PROJECT_NAME=omsd1c5 deployments/d1/ops/d1 restore 20260927T221203Z
D1_COMPOSE_OVERRIDE_FILE=/tmp/c5/networks.yaml D1_ENV_FILE=/tmp/c5/d1.env D1_PROJECT_NAME=omsd1c5 D1_PUBLIC_URL=http://127.0.0.1:18150 acceptance/d1-delivery/verify-running
```

全部通过。联合备份实际包含 Results dump、Project／IAM dump、Contract／Evidence ZIP、签名密钥、十项 Suite 选择和 SHA-256；销毁隔离栈容器／网络／卷后从空目标恢复，并再次通过 Nginx→Gateway→IAM 入口、Results 页面、已选应用读写、未选应用 `404` 与 Evidence 字节生命周期。当前判定 `formal-d1-assembly-and-recovery-supported / business-cross-process-journey-pending`；该证据还不包含独立进程间的批复→审计→Project 完成确认、撤权／停用、双 Tenant 或真实浏览器动作收敛。详细证据见 [OA C5 D1 正式装配与恢复实验](./reports/oa-c5-d1-assembly-recovery-experiment-20260928.md)。

## OA C5 真实独立进程业务旅程实验（2026-09-28 当前切片通过）

隔离 D1 选择 People、Project、Contract、Evidence 和 Results，关闭 Money、Party、Business Engagement、Billing 与 Production。旅程从正式 Nginx→Gateway→IAM 入口创建真实 People Assignment、active Project、有效 Contract、Evidence 文件和 Project Resource Link，再创建 approval、audit 并显式确认 `reference-oa-c5-v1` 检查点。

实际执行：

```bash
acceptance/d1-c5-results-journey/run-isolated verify
```

结果为 74 个 HTTP 断言、0 个状态不匹配。验证直接覆盖 Evidence／Contract 停机 503 与 Results 零写入、同 Command 恢复和重放、approval→audit 精确 revision、无金额能力服务端脱敏、跨 Project 403、无 create 403、Project 显式确认 revision 5→6、`correct` Grant 撤销／恢复、确认后 approval revision 2 与历史 revision 1 保持、Project drift，以及 Results 停机时 GET unavailable、已成功确认本地回放、新确认 503 且 Project revision 不变。

验收暴露并修复两个跨域问题：Results 纠正不能要求 Project 当前 revision 等于首次登记快照，否则 Project 确认后永远无法纠正；Project 新确认遇到 Results unavailable 必须返回 503，而不是通用 422。两项均新增单元测试并由真实 D1 旅程复验。

当时判定 `real-independent-process-journey-supported / dual-tenant-and-real-browser-pending`。C5 的真实业务组合缺口已关闭，但完整双 Tenant、账号停用、真实浏览器、真实 Ubuntu／公网环境和未知业务规则仍待补；C5 仍不能代表完整 OA Project 完成。2026-10-06 顶部“批复／审计原界面闭环”已经补上该切片的私有真实浏览器证据，但没有补齐 Main／公网、正式 D1、双 Tenant、全角色或第二客户，因此本段保留为当时实验结论而不再作为当前总状态。详细证据见 [OA C5 真实独立进程业务旅程实验](./reports/oa-c5-real-process-journey-experiment-20260928.md)。

## OA C6 成本与外协责任边界审计（2026-09-28 边界通过，运行未开始）

只读审计参考 OA 的需求、`FinancialRecord`、`ExternalCommission`、对应 Service／路由／页面和删除行为。结果确认参考实现把奖金、成本、客户付款、发票、专家费和外协付款混进宽记录，使用浮点金额、可编辑删除；外协服务直接创建支付／票据，并在删除时按供应商名称兜底删除财务记录。该证据只能说明参考 OA 的现状，不能作为目标所有权。

目标接受独立 Cost & Outsourcing，在一个部署边界内使用 Cost Entry 与 Outsourcing Engagement 两个聚合；金额使用整数最小货币单位，Provider 保存 Party 发生时快照，所有纠错追加 revision。Project、Party、Contract、Evidence 只通过版本化精确端口读取；payable suggestion 必须由 Money 独立确认，票据由 Billing 登记。C6-A01～A18 已固定，当前判定 `boundary-supported / implementation-not-started`。本轮未运行应用、数据库、IAM、浏览器、D1 或恢复测试，不得描述为 C6 已实现。详细证据见 [OA C6 成本与外协责任边界审计](./reports/oa-c6-cost-outsourcing-boundary-audit-20260928.md)。

## OA C6 Cost & Outsourcing 领域核心实验（2026-09-28 部分通过）

新增 `backend/services/cost-outsourcing`，以独立 module 和 PostgreSQL 18 migration 实现 Cost Entry、Outsourcing Engagement 和追加式 Assessment。两个聚合拥有不同 current／revision 表与类型化命令，共享表仅保存幂等 receipt 和技术 audit，不承载宽业务字段。Project／Party／Contract／Evidence 通过精确端口复验并冻结快照；核心 Actor 同时检查原生 action 和 Project Scope，Evidence 引用另行判权。

实际执行：

```text
cd backend/services/cost-outsourcing && go test ./...
cd backend/services/cost-outsourcing && go vet ./...
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable make cost-outsourcing-core-acceptance
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/internal/costoutsourcing
make fmt-check vet test
```

最终全部通过。首轮 race detector 发现 Actor 归一化会就地修改共享 Project Scope slice，修复为防御性复制后同一并发矩阵通过。真库测试覆盖 Cost 登记／纠正／撤回、Engagement 登记／纠正／撤回、五分制评分及追加纠正、精确 Party／Contract／Evidence 快照、Command 冲突、stale revision、错 Project／Tenant、撤回后拒绝、依赖失败零写入、恢复后原 Command 单次成功、依赖再次停机时本地重放，以及 8 路并发最终只有 1 current／revision／command／audit。数据库 trigger 直接拒绝 revision UPDATE 和 current 物理 DELETE。

当前判定 `domain-core-and-postgresql-supported / protected-http-and-composition-pending`。A01～A05、A07～A09、A14 在核心层通过；A06／A13／A15／A18 部分通过；A10～A12、A16～A17 尚未开始。该结果不代表受保护 HTTP、正式 IAM、真实 adapters、付款建议、页面、D1、恢复或 Project Completion 通过。详细证据见 [OA C6 领域核心实验](./reports/oa-c6-core-experiment-20260928.md)。

## OA C6 受保护 HTTP、IAM 与精确引用实验（2026-09-28 部分通过）

C6 在同一 canonical audience `cost-outsourcing` 下发布 Cost Entry 与 Outsourcing Engagement 两套原生动作；Capability Manifest 使用显式 `capability_namespaces`，IAM Catalog 同时校验 `cost-entry` 和 `outsourcing-engagement`，没有用 audience 字符串前缀冒充能力所有权。目标应用的 Ed25519 Provider 与模板 Provider 使用同一 Actor／action／Scope 模型；客户端身份 Header 先删除，Service 再按 Tenant、Project 或精确 Cost／Engagement 资源、状态、expected revision、金额和 Evidence 能力作最终授权。

实际执行：

```text
cd backend/services/cost-outsourcing && go test ./...
go test ./acceptance/access-contract
cd backend/platform/identity-access && go test ./internal/capability
cd backend/services/contract-ledger && go test ./internal/httpserver
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -count=1 -tags=integration ./backend/services/cost-outsourcing/...
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/...
make fmt-check vet test
```

全部通过。版本化 HTTP 覆盖 Cost／Engagement 的列表、登记、详情、历史、纠正、撤回和 Assessment；无金额能力时 current／history／冻结 Contract 金额均为 `null`，Evidence 动作单独授权，响应动态返回 `allowed_actions`。Project、Party、Contract、Evidence 四类 adapter 实际交换单 audience／action／精确 resource credential，并复验 Tenant、类型、ID、revision、状态和摘要；Contract 还必须链接同一 Project 且包含 provider Party，Evidence 必须命中精确 File Version。矩阵覆盖无凭证、错 audience／Tenant、相邻角色、相邻资源、401 单次重换票、403、404／422、网络／5xx／畸形／超大响应、依赖停机零写入、恢复后原 Command 单次提交、成功重放不访问依赖和 stale revision 409。

当前判定 `protected-http-and-reference-adapters-supported / human-workspace-and-composition-pending`。C6-A01～A09、A13～A14 后端通过，A15／A18 部分通过；A10～A12、A16～A17 尚未开始。该结果不代表人类工作区、真实独立进程最小组合、付款建议、D1、恢复或 Project Completion 通过。详细证据见 [OA C6 受保护 HTTP、IAM 与精确引用实验](./reports/oa-c6-protected-http-reference-experiment-20260928.md)。

## OA C6 人类工作区与独立进程最小组合（2026-09-28 部分通过）

C6 新增独立服务入口和实际人类工作区，覆盖 Project 载入、Cost／Engagement 页签、列表、详情、登记、纠正、撤回、历史与 Assessment。所有动作由服务端 `allowed_actions`、金额能力、状态和 revision 收敛；令牌仅保存在当前标签页，401／403／409／422／503 均保留页面上下文。真实浏览器检查了概览、双页签、详情、金额和精确引用，并发现、修复 CSP 阻止脚本及已有 Assessment 仍提交 revision 0 两项问题。

实际执行：

```text
cd backend/services/cost-outsourcing && go test ./...
go test ./acceptance/c6-cost-outsourcing-composition/...
sh -n acceptance/c6-cost-outsourcing-composition/run-isolated
acceptance/c6-cost-outsourcing-composition/run-isolated
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -count=1 -tags=integration ./backend/services/cost-outsourcing/...
TEST_DATABASE_URL=postgres://…/postgres18.6-disposable go test -race -count=1 -tags=integration ./backend/services/cost-outsourcing/...
make fmt-check vet test
```

全部通过。隔离夹具真实启动 IAM、Project、Party、Contract、Evidence、C6 与 PostgreSQL 七个独立进程，12 个 HTTP 结果覆盖工作区、Evidence／Party 停机零写入、恢复后原业务意图成功、成功 Command 在依赖再次离线时本地重放、四类精确来源冻结、Assessment、金额脱敏、只读写入拒绝和错 Project Scope 拒绝；进程清单确认 Money、Billing、应付款建议和 C7 均未启动。

当前判定 `human-workspace-and-independent-process-composition-supported / formal-d1-and-payable-suggestion-pending`。人类入口和本地真实多进程组合已经通过，但不代表正式 D1、Workbench／Gateway／Nginx、未选缺席、联合备份恢复、Payable Suggestion、Project Completion 或真实 Ubuntu 通过。详细证据见 [OA C6 人类工作区与独立进程最小组合实验](./reports/oa-c6-human-workspace-independent-composition-experiment-20260928.md)。

## OA C6 正式 D1 装配、故障与恢复实验（2026-09-28 当前切片通过）

隔离 D1 选择 People、Party、Project、Contract、Evidence 和 Cost & Outsourcing，关闭 Money、Business Engagement、Billing、Production、Results 与 C7。C6 使用独立镜像、数据库、migration 和 Workload Key，经正式 Nginx→Gateway→IAM 入口提供 Workbench 页面和 API；Evidence 作为无业务页面的支撑服务被选择但不出现在 Workbench。

实际执行入口：

```bash
acceptance/d1-c6-cost-outsourcing/run-isolated full

go test ./backend/services/cost-outsourcing/... ./backend/platform/gateway/... ./backend/platform/identity-access/... \
  ./backend/services/project-workspace/... ./backend/services/party-directory/... \
  ./backend/services/evidence-service/... ./backend/services/contract-ledger/...
```

首次业务阶段得到 57 个 HTTP 断言、0 个状态不匹配，覆盖 active Project、Directory Party、Evidence、Cost、Engagement、Assessment、金额脱敏、错 Project／无 create 403、Evidence／Party 停机零写入和恢复后同 Command 单次成功。IAM workload audit 分别精确命中 Project、Party、Contract 与 Evidence 的 audience、action、resource type 和 resource ID。旧 Contract 本地主体与 Directory Party ID 不一致的带 Contract Engagement 返回 422 `REFERENCE_INVALID`，没有共享表、名称映射或隐式 ID 替换。

正式联合备份 `20260928T024459Z` 后只销毁隔离 `omsd1c6` 的容器／网络／卷，并从空目标恢复。Cost、Engagement、Assessment 的 ID、revision、精确快照和历史保持，三个 canonical SHA-256 与恢复前一致；恢复后 Cost correction 成功推进 revision 1→2。取消 C6 选装后进程、Workbench 入口、Gateway route 和备份 dump 缺席，备份 `20260928T031009Z` 记录 selection=false；重新选装并完成 Gateway 授权后路由为 200，取消选装没有删除数据库卷。

未选装验收脚本最初把重新选装后的未授权 303 当成 200；修正为先授权再要求 200。完整脚本随后一次 Compose 重建等待被人工中止，但未选装阶段全部断言和重新启动后的“授权 303→路由 200”均有分段直接观察，因此不把它写成修订脚本整段再次运行通过。

该轮判定原为 `formal-d1-assembly-and-recovery-supported / contract-party-runtime-cutover-and-payable-suggestion-pending`；下节新增直接证据已经关闭 Party 切换前置项。C6 仍缺 Payable Suggestion、Project Completion 适用性、双 Tenant 和真实 Ubuntu。该轮原始步骤、摘要和边界见 [OA C6 正式 D1 装配、故障与恢复实验](./reports/oa-c6-d1-formal-composition-recovery-experiment-20260928.md)。

## Contract／Money Party 运行时切换与 Contract-linked C6（2026-09-28 当前切片通过）

Contract migration `000019_party_runtime_transitions.sql` 和 Money migration `000018_party_runtime_transitions.sql` 为每个 Tenant 保存追加不可变 `cutover／rollback` 序列。切换前重新验证同一批次全部目标 Party 精确 revision，并在来源事务内锁定转换状态、映射批次和全部已引用本地 Party；缺映射、目标 stale、批次漂移或 Directory 不可用均失败关闭。Directory authority 一旦建立，回滚只恢复本地兼容读取，本地 Party POST 始终返回 409。

实际运行：

```bash
cd backend/services/contract-ledger && go test ./...
cd backend/services/money-flow-ledger && go test ./...
COMPOSE_PROGRESS=plain acceptance/d1-c6-cost-outsourcing/run-isolated full
```

全部通过。两侧 PostgreSQL integration 使用真实被 Contract revision 或 Money obligation／cash transaction 引用的 Party，覆盖映射、切换、回滚、写入冻结和转换防篡改。C6 串行 D1 为 62 个入口断言、0 个状态不匹配：切换前 Contract-linked Engagement 422，映射并 `cutover` sequence 1 后 201，`rollback` sequence 2 后再次 422且本地 Party 写入 409，`recutover` sequence 3 后恢复 Directory 模式。正式 Contract reference 使用当前 revision 的不可变 Party Snapshots；测试同时保留草稿 active Party 行为。

联合备份 `20260928T052302Z` 后销毁隔离容器／网络／卷并从空目标恢复，转换状态保持 `cutover|<batch>|3|2`，映射、业务 ID、revision、精确快照与历史保持，恢复后 Cost correction revision 1→2。取消 C6 选装时 runtime／Workbench／route／dump 缺席，重新选装后路由恢复且数据库卷未删除。

当前判定更新为 `referenced-runtime-cutover-supported / payable-suggestion-and-complete-dual-tenant-pending`。Money 的机制性真库证据已经通过，但其完整 D1 消费链、完整双 Tenant、C6 Payable Suggestion、C7～C8 与真实 Ubuntu 仍未通过。详细证据见 [Contract／Money Party 运行时切换与 C6 复验](./reports/oa-party-ledger-runtime-cutover-experiment-20260928.md)。

## C6 → Money 应付建议真实跨进程实验（2026-09-28 当前切片通过）

机器可读契约解析、普通单元检查和脚本语法检查：

```bash
ruby -e 'require "yaml"; ARGV.each { |p| YAML.safe_load(File.read(p), aliases: true) }' \
  arckit/tech/_shared/models/PayableSuggestion.yaml \
  arckit/tech/_shared/models/PayableSuggestionIntake.yaml \
  arckit/tech/_shared/contracts/payable-suggestion-api.yaml

go test ./backend/services/cost-outsourcing/internal/costoutsourcing \
  ./backend/services/money-flow-ledger/internal/payablesuggestion \
  ./packages/payable-suggestion-contract
sh -n acceptance/c6-money-payable-flow/run-isolated
```

PostgreSQL 18 真库和真实双进程验收：

```bash
TEST_DATABASE_URL='postgres://postgres:测试密码@127.0.0.1:测试端口/postgres?sslmode=disable' \
go test -tags=integration \
  ./backend/services/money-flow-ledger/internal/payablesuggestion \
  ./backend/services/cost-outsourcing/internal/costoutsourcing -count=1

./acceptance/c6-money-payable-flow/run-isolated
```

全部通过。两个服务连接不同 PostgreSQL 数据库，只经版本化 HTTP 协作。接受路径创建一个 `payable / open` Money Obligation，决定和 C6 同步重放不重复；拒绝路径的 SQL 摘要为 `0|2`，即零 Obligation、两条 intake revision。Money 停机时 C6 同步返回 503 且已有事实保持，恢复后显式同步成功。故障注入还覆盖 Obligation 已提交但 intake 尚未更新的部分失败，确定性子命令恢复到同一义务。

当前判定为 `real-cross-process-payable-suggestion-supported / formal-d1-complete-dual-tenant-real-ubuntu-pending`。本节不声称完整 Nginx→Gateway→IAM D1 connector、双 Tenant、跨进程撤回、Money／Billing 只读投影或真实 Ubuntu 已通过。详细证据见 [C6→Money 应付建议实验](./reports/oa-c6-payable-suggestion-experiment-20260928.md)。

## OA C7 奖金责任边界审计（2026-09-28 边界通过，运行未开始）

本轮只读审计参考 OA 的确认需求、Usage Guide、OA 2026 service、Financial Service、Bonus 模型及删除路径。证据显示同一奖金概念分散在三套表示中：OA 2026 奖池以当前 Contract／FinancialRecord 和 `float64` 计算并原地 upsert；宽 `FinancialRecord` 同时承载奖金、收款、成本和票据；独立 `Bonus` 又只保存 Project／User／Type／金额并允许删除。人员候选分别来自经营／生产 Assignment，但当前实现没有冻结来源、Person 或 Assignment revision。

目标接受独立 Bonus Distribution。经营 Pool 读取 Contract 已确认 Settlement，生产 Pool 读取 Money 已确认公司收款；每次计算冻结来源 ID／revision／digest、Person／Assignment、策略版本、整数最小货币单位输入、比例和舍入结果。经营与生产共用严格生命周期但使用不同 Capability。发布后的结果只能由新 revision 取代或显式撤回，并只向 Money 提交版本化 Payable Suggestion；Money 独立接受或拒绝。参考 OA 的 `base * ratio / 1.065` 登记为 `reference-oa-bonus-v1` Customer Overlay，不进入 Base。

C7-A01～A18 已固定，当前判定 `boundary-supported / implementation-not-started`。本轮没有运行应用、数据库、HTTP、IAM、浏览器、D1、备份或恢复测试；因此只证明责任边界与下一轮验收可执行，不能描述为 C7 已实现。详细证据见 [OA C7 奖金责任边界审计](./reports/oa-c7-bonus-boundary-audit-20260928.md)。

## OA C7 领域核心与 PostgreSQL 事实层（2026-09-28 核心通过）

本轮新增独立 `backend/services/bonus-distribution` module。普通工作区与独立 module 检查：

```bash
gofmt -w backend/services/bonus-distribution
go test ./backend/services/bonus-distribution/...
go vet ./backend/services/bonus-distribution/...
cd backend/services/bonus-distribution
GOWORK=off go test ./...
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

全部通过。随后使用一次性 PostgreSQL 18.6 运行：

```bash
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/postgres?sslmode=disable' \
  go test -count=1 -tags=integration -race ./backend/services/bonus-distribution/...
```

全部通过。验证覆盖经营 confirmed Settlement 与生产 confirmed receipt set 两类端口、精确 `reference-oa-bonus-v1` half-up、整数基点分配与显式未分配余额、业务／生产 action 隔离、来源／Person／Assignment 变化不改写历史、发布／取代／撤回、Pool／Allocation history 防篡改、依赖失败零写入、恢复后原 Command 单次提交、成功重放不再访问依赖、同 Command 不同摘要冲突、八路并发草稿单次提交、并发发布 expected revision 冲突、跨 Tenant 隐藏、错 Project 拒绝和过量分配写前拒绝。

测试暴露了发布／取代／撤回先检查新状态、后查成功 receipt 的顺序缺陷；修复为成功 receipt 优先后，状态推进或上游离线不再破坏原命令幂等，新命令仍复验来源并对 drift 失败关闭。反耦合扫描没有发现正式路径使用 `float64`、共享财务表、跨库 SQL 或当前用户表。

当前判定 `domain-core-and-postgresql-supported / protected-http-and-composition-pending`。C7-A01～A09 已在核心层通过，A11／A12／A18 部分通过；HTTP、正式 IAM、Capability Manifest、真实 adapters、金额脱敏、页面、独立进程、D1、恢复和 Money 建议没有运行，不得描述为通过。详细证据见 [OA C7 领域核心与 PostgreSQL 事实层实验](./reports/oa-c7-domain-postgresql-experiment-20260928.md)。

## OA C7 受保护 HTTP、IAM 与引用适配器（2026-09-28 后端通过）

本轮在独立 `bonus-distribution` module 上增加版本化 HTTP、Ed25519 IAM、Capability Manifest、经营／生产 Assignment Policy、请求绑定工作负载 resolver 和服务端金额脱敏。普通测试和 IAM Catalog 测试：

```bash
cd backend/services/bonus-distribution && go test ./...
cd ../identity-access && go test ./internal/capability
```

全部通过。随后使用一次性 PostgreSQL 18.6：

```bash
TEST_DATABASE_URL='postgres://postgres:***@127.0.0.1:<ephemeral>/bonus_http?sslmode=disable' \
  go test -race -count=1 -tags integration ./internal/httpserver ./internal/bonus
```

全部通过。验证覆盖经营／生产两类入口、依赖失败零写入、恢复单次提交、成功重放零上游、一次 401 换票、404、409、422、503、错 audience／tenant／Project／角色／资源拒绝，以及 current／Pool history／Allocation history 的统一金额脱敏。工作负载调用记录证明 Project、Assignment、Person、Settlement 与 Receipt Set 都使用精确 audience、action、resource type 和 resource ID。

当前判定 `protected-http-iam-and-consumer-adapters-supported / live-source-process-composition-pending`。Project／Contract 已有既有目标 reference 形状；People Person 与 Money Receipt Set 本轮只通过严格 HTTP 来源替身和新契约验证，真实目标端点、独立进程组合、人类工作区、D1、联合恢复与 Payable Suggestion 尚未通过。详细证据见 [OA C7 受保护 HTTP、IAM 与引用适配器实验](./reports/oa-c7-http-iam-adapter-experiment-20260928.md)。

## OA C7 真实来源与独立进程最小组合（2026-09-28 通过）

普通回归：

```bash
cd acceptance/access-contract && go test ./...
cd ../../backend/platform/identity-access && go test ./...
cd ../project-workspace && go test ./...
cd ../people-groups && go test ./...
cd ../money-flow-ledger && go test ./...
cd ../bonus-distribution && go test ./...
```

全部通过。随后运行真实六进程组合：

```bash
acceptance/c7-live-source-composition/run-isolated
```

返回码为 0。IAM、Project、People、Contract、Money、Bonus 使用六个独立数据库；Money Receipt Set 确认、经营草稿／发布、生产草稿／发布的五次 HTTP 状态分别为 `201、201、200、201、200`。来源计数为 `People=2、Project=1、Settlement=1、ReceiptSet=1`；Bonus 数据库摘要 `2|2|2|1` 表示两个 Pool、两个 published、两种 bonus type、一个 Project。两个 `106500` 分来源均得到 `10000` 分奖金。

本轮在真实提交路径发现并修复 Receipt Set 延迟触发器错误，使用前向 `000021` migration；也补齐 Project Settlement resource link 数据库约束。当前判定 `live-source-independent-composition-supported / human-workspace-and-d1-pending`。初始化 SQL 只写每个来源应用自己的前置事实，运行期没有跨库访问；人类页面、D1、联合恢复、Money 建议和 C8 没有运行。详细证据见 [OA C7 真实来源与独立进程最小组合实验](./reports/oa-c7-real-source-independent-composition-experiment-20260928.md)。

## OA C7 人类工作区与直接纠错／故障恢复（2026-09-28 通过）

普通检查：

```bash
go test ./backend/services/bonus-distribution/...
go vet ./backend/services/bonus-distribution/...
sh -n acceptance/c7-live-source-composition/run-isolated
```

六进程业务验收：

```bash
acceptance/c7-live-source-composition/run-isolated
```

返回码为 0。夹具验证工作区和同源脚本、未授权 401、真实经营／生产创建与发布、Contract 停机 503 且 Pool 数保持不变、恢复后替代草稿、原经营 Pool 原子取代与双向历史、生产 Pool 撤回。最终为 3 个 Pool：1 个 published、1 个 superseded、1 个 withdrawn；经营与生产来源仍分别冻结 Settlement／Receipt Set 及 Person／Assignment revision。

浏览器保持模式：

```bash
C7_HOLD_AFTER_ASSERTIONS=true acceptance/c7-live-source-composition/run-isolated
```

真实浏览器从页面读取 3 条奖池，选择被取代经营奖池后显示 Project、100.00 元奖金、0.00 元未分配、Settlement revision／digest、`reference-oa-bonus-v1`、Assignment、Allocation 和替代关系；控制台错误为 0。过期短期凭证明确显示 401 恢复提示，新凭证后数据加载成功。

当前判定 `human-workspace-and-direct-recovery-supported / formal-d1-and-payable-suggestion-pending`。本节不声称正式 Workbench／Gateway／Nginx D1、未选装、联合空目标恢复、C7→Money 建议、C8、双 Tenant 或真实 Ubuntu 已通过。详细证据见 [OA C7 人类工作区与直接纠错／故障恢复实验](./reports/oa-c7-human-workspace-experiment-20260928.md)。

## OA C7 正式 D1 装配、未选缺席与联合恢复（2026-09-28 通过）

本轮使用隔离 Compose project `omsd1c7` 和回环端口 `18180`，只选择 IAM、Workbench、Gateway、Nginx、People、Project、Contract、Money 与 Bonus Distribution。实际执行了 Bonus module test／vet、D1 image／promotion／ingress／upgrade／release 静态门禁，以及：

```bash
D1_C7_ACCEPTANCE_CONFIRM=yes acceptance/d1-c7-bonus-distribution/run-isolated full
```

业务断言完成后，包装脚本先后暴露报告字符串变量边界和恢复校验事件名两项验收代码缺陷；修正后在同一隔离环境中以幂等恢复和一轮全新的未选／重新选装检查完成余下阶段。因此各阶段证据通过，但未把最初包装命令描述为一次返回码 0。

结果为：16 条 Nginx→Gateway→IAM 正式入口断言偏差 0；Project、People、Contract、Money 共 5 条精确 workload audit；经营／生产各发布 1 个 Pool。联合备份 `20260928T143704Z` 经空目标恢复后四份 canonical SHA-256、Pool／Allocation ID、revision、来源／策略／人员快照和历史保持；生产 Pool 随后从 revision 2 追加撤回到 revision 3。取消选装时 Manifest、Workbench、route、runtime、migration、工作负载密钥与 dump 均缺席，重新选装后数据仍在。

当前判定 `formal-d1-assembly-and-recovery-supported / payable-suggestion-and-single-type-minimum-pending`。C7→Money、经营／生产单类型最小组合、C8、双 Tenant 和真实 Ubuntu 没有运行。详细证据见 [OA C7 正式 D1 装配、未选缺席与联合恢复实验](./reports/oa-c7-d1-assembly-recovery-experiment-20260928.md)。

## OA C7 → Money 应付建议直接跨进程旅程（2026-09-29 通过）

为控制本机资源，本轮严格串行：先在 Docker 为 0 容器时运行静态和单元检查；再只启动一个临时 PostgreSQL 18.6，依次执行 Bonus／Money 目标真库测试；最后仍只保留一个 PostgreSQL 容器，并启动 Bonus、Money 两个本地进程完成跨进程验收。没有启动完整 D1 或第二套 Compose。

普通检查使用低并发参数：

```bash
cd packages/payable-suggestion-contract && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
cd backend/services/bonus-distribution && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
cd backend/services/money-flow-ledger && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
```

目标真库用例分别验证 published Allocation 建议的停机恢复、Money 独立接受／拒绝、exact Person→Party 映射、防篡改、未映射 revision 失败关闭和不可变历史。跨进程验收：

```bash
acceptance/c7-money-payable-flow/run-isolated
```

全新空库运行返回码为 0，11 条 HTTP 断言、0 个状态偏差。Money 未启动时 C7 保存 `target_unavailable`；恢复后同命令提交成功。接受生成且仅生成一个 `bonus_payable_suggestion` 来源的 Money payable／open Obligation，拒绝生成零个；决定同步重放不重复。源 Pool／Allocation 后续撤回不级联修改已形成义务。脚本结束后本地进程和临时容器全部清理，`docker ps` 为 0。

当前判定 `real-cross-process-payable-suggestion-supported / formal-d1-connector-suggestion-recovery-pending`。本轮没有运行正式 Nginx→Gateway→IAM D1 connector、建议引用联合备份／空目标恢复、单类型最小组合、双 Tenant、真实 Ubuntu 或 C8。详细证据见 [OA C7→Money 应付建议实验](./reports/oa-c7-money-payable-suggestion-experiment-20260929.md)。

## OA C8 资料、统计与个人工作责任边界（2026-09-29 静态通过）

本轮只读取参考 OA 与现有 BizFoundry 的代码、规格和契约，没有启动 Docker、数据库或应用进程。审计确认 C8 是现有事实所有者之上的只读装配，不新增共享报表数据库或统一可写待办表：

- Project Workspace 只按 Project／Resource Link 组合项目资料；
- Evidence Service 拥有文件元数据、不可变版本与下载授权，metadata／download／export 分权；
- 各领域应用发布带口径、单位、revision／digest 和观测时间的指标与工作项；
- Unified Workbench 只聚合当前 Subject 的导航、工作项引用和来源状态，不推断或改写业务状态；
- 逐来源超时和局部失败明确区分 `empty`、`unavailable`、`forbidden`、`not_selected`、`revision_changed`，无权金额为 `null／redacted` 而不是 0。

静态检查入口：

```bash
git diff --check
rg -n "BF-073|c8-boundary-supported|C8-A(0[1-9]|1[0-8])" \
  docs arckit/spec arckit/tech
docker ps --format '{{.Names}}\t{{.Status}}'
```

当前判定 `c8-boundary-supported / runtime-not-started`。本轮不声称 API、页面、IAM／D1、数据库或恢复通过；第一运行切片必须先兼容拆分 Evidence metadata／download 能力，再实现直接 Evidence Resource Link 的 Project 资料组合。详细依据见 [C8 产品规格](../arckit/spec/assembly/c8-material-statistics-personal-work.md)、[C8 技术决策](../arckit/tech/assembly/c8-read-projection-boundary.md)与[C8 边界审计](./reports/oa-c8-material-statistics-personal-work-boundary-audit-20260929.md)。

## OA C8 首个后端切片（2026-09-29 低资源单元通过）

本批没有启动 Docker、数据库或应用进程。Evidence 把元数据与 Access Grant 分权，同时保留旧 `view` 的有界兼容；Project 只读取自身 active direct Evidence Link，并以最多四路并发返回 provenance 和局部状态。复验命令：

```bash
cd backend/services/evidence-service
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...

cd ../project-workspace
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off go test -p 1 -run 'TestPublishedExamplesConformToCrossFieldRules|TestProjectWorkloadPolicyCanOnlyComposeExactCommercialReferences|TestManifestNegativeCasesFailClosed' ./...
```

三个命令均通过。覆盖 metadata-only 不能下载、download-only 不能读 metadata、旧 view 兼容、精确 File／Project Scope、相邻 Project、错 Tenant、stale revision、403、来源不可用、empty、Project workload 精确 action 与 manifest 负例。`go test -p 1 ./...` 的 Access Contract 全量运行另暴露 Bonus 策略已有 3 个 payable-suggestion 权限而旧测试仍只期望 4 个来源引用权限；该问题未由本批 C8 变更引入，也未在本批顺手修改。

当前判定 `c8-first-backend-slice-unit-supported / runtime-assembly-pending`。下一步必须以低资源方式运行一个临时 PostgreSQL 和 Project／Evidence 最小组合，验证真实 IAM workload 换票、direct Link、相邻 File／Project 拒绝、Evidence 停机／恢复和清理后容器为 0。详细证据见 [C8 首切片单元验证](./reports/oa-c8-first-backend-slice-unit-validation-20260929.md)。

### C8 后续：Access Contract 旧 Bonus 断言收口（2026-09-29 通过）

在不启动 Docker 的独立小批次中，`TestBonusWorkloadPolicyHasOnlyExactSourceReferences` 更名并同步为精确核对 Bonus 的 4 个来源引用权限与 3 个 payable-suggestion 权限。发布策略文件未改，测试继续使用完整顺序比较，因此多权、少权、错 audience、错 action、错 resource type 或顺序漂移都会失败。实际执行：

```bash
cd acceptance/access-contract
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
```

全量通过，复验前后 `docker ps -q` 均为 0。该批只恢复 Access Contract 全量门禁可信度，不增加 C8 运行时证据。

## OA C8 Project → Evidence 最小运行组合（2026-09-29 通过）

本批只运行一个临时 PostgreSQL 18.6 容器；IAM、Evidence、Project 作为三个本地进程顺序启动，不启动 Nginx、Gateway、Workbench 或完整 D1。运行入口：

```bash
acceptance/c8-material-composition/run-isolated
```

脚本实际通过：

- Project workload 使用独立 Ed25519 私钥向真实 IAM 换取 `evidence-service.metadata.read + file:{id}`；IAM 精确审计计数为 1，download 审计为 0；
- direct active Link 返回 `available`，冻结 revision 2 对当前 Evidence version 1 返回 `revision_changed`；
- 只授权相邻 Project 的人类 Credential 访问目标 Project 返回 `403 scope_denied`；
- Evidence 停机时两个分段均为 `unavailable/source_unavailable`，Project API 保持 200；同库重启后当前资料恢复 `available`；
- Project 响应不含 `bucket` 或 `object_key`；
- HTTP 状态序列为 `200、403、200、200`，运行容器数为 `0 → 1 → 0`。

当前判定 `c8-direct-material-runtime-supported / human-page-and-d1-pending`。详细步骤、证据和未覆盖边界见 [C8 Project→Evidence 最小运行组合实验](./reports/oa-c8-project-evidence-runtime-experiment-20260929.md)。

## OA C8 Project 资料人类页面首批验证（2026-09-29 单元通过，浏览器待验）

本批不启动完整 D1。Project 记录工作区新增“项目资料”页签，资源连接支持 Evidence File，页面按 `project-workspace.materials.read` 显示文件名、类型、大小、SHA-256、冻结／当前 revision 和逐段状态；Suite 路径的下载按钮先单独授权 Evidence Service，direct 开发入口不伪造该链。

实际串行执行：

```bash
cd backend/services/project-workspace
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
```

结果通过；嵌入 JavaScript 的 Node 语法检查通过，Project 交互灰度线框 8 个状态的结构计数也通过。随后用 `C8_BROWSER_FIXTURE`／`C8_BROWSER_RELEASE` 保持模式重跑最小后端，仍只有一个 PostgreSQL 容器，四组 HTTP 断言为 `200、403、200、200`，退出后容器数为 0，临时凭证文件已清理。

真实浏览器自动化因 macOS 锁屏且自动解锁失败，没有完成点击。因此当前判定仅为 `c8-human-material-experience-unit-supported / browser-and-d1-pending`；不声称 Suite 下载授权、正式 D1、间接来源、指标／WorkItem、双 Tenant 或真实 Ubuntu 已通过。详细证据见 [C8 资料页面首批实现与低资源验证](./reports/oa-c8-project-material-human-experience-unit-validation-20260929.md)。

同日又通过已授权 SSH 隧道对公司 D1 做了不写业务数据的版本预检：12 个容器均 healthy，真实 Gateway 登录与 Project 应用授权成功，Project 列表返回 12 条记录；但运行发布仍为 `v0.1.0-test.5`，其 Project 静态资源不包含 `project-workspace.materials.read`、Evidence 独立授权或 stale 状态文案，Project 详情没有 `materials.read` 动作，`/materials` 返回 `404`。该环境因此尚不能作为当前 C8 资料页面的浏览器证据，必须先部署包含本实现的可追溯 release。预检未重启服务、未修改记录，也未在 Mac 上启动容器。

随后以零本地容器方式准备 `v0.1.0-test.6` 开发验证发布：Project 与 Evidence 新镜像分别推送并固定 digest，D1 完整／稀疏发布包门禁通过，归档外层与内层摘要及 12 个选中镜像引用通过校验。公司 ECS 又用旧版脚本创建并校验联合备份 `20260929T032057Z`，Evidence 恢复 healthy；新目录和候选环境配置通过且未覆盖旧版。目标机因 ACR 凭据未保存而在首次新镜像拉取处得到 `pull access denied`，因此尚未切换或重启服务，不能把发布准备记为 D1 运行通过。

### C8 公司 D1 兼容闭包、逐服务恢复与真实入口读链（2026-09-29 通过）

ACR 登录完成后，首次只提升 Project／Evidence 的候选实际失败关闭：旧 IAM `test.5` 无法解析新增 `capability_namespace`；Project 前向 migration 已提交后，旧 Project 又因数据库 migration 数量 `7 != 3` 拒绝启动。没有执行 down migration、修改 migration 历史或强迫旧二进制继续运行。目标机先恢复旧入口／IAM／Evidence，并保留能识别新 schema 的 Project `test.6`；随后补发 IAM `test.6`，按 IAM → Evidence 串行替换并逐项等待 healthy。

升级前联合备份 `20260929T113133Z` 的十项 `checksums.sha256` 全部通过。最终 IAM／Project／Evidence 分别运行固定摘要 `468b6484…d610`、`daa0554e…31e47`、`1257a438…e9d28`；12 个长期容器全部 healthy，候选 `d1 check` 和 bootstrap grant 通过。该事件形成 [BF-077](./decisions.md) 与 [ADR-0035](./adr/0035-d1-release-upgrades-use-compatibility-closure.md)：发布集合必须包含契约解析者和 migration 读者的兼容闭包，已执行 migration 后不能把直接拉起旧镜像称作回滚。

真实入口只读验证使用 Nginx 唯一宿主入口和临时 Cookie 文件，结束时注销并删除临时文件。实际结果：ready `200`、IAM 登录成功、Project 授权 `303`、Project 列表 `12` 条、详情包含 `project-workspace.materials.read`、`/materials` `200`、Project 静态资源的三项新标记齐全、Evidence 独立授权 `303`、Evidence 页面 `200`、注销 `204`。本机 Docker 全程为 0 容器。该时点只支持 `c8-project-material-formal-entry-read-supported`；后续同日正式 API 已完成专用 Link、Version 2、stale 重连，真人浏览器下载授权点击和权限分权仍待验。

同日增加候选契约解析预检：IAM `/app/ops validate-contracts` 在不打开数据库的情况下解析全部 Capability Manifest、Assignment Policy 和 Workload Policy；`d1 preflight-upgrade` 和 `d1 upgrade` 都在备份／migration 前使用候选 IAM 镜像执行它。`GOMAXPROCS=2 GOWORK=off go test -p 1 ./cmd/ops ./internal/capability ./internal/policy ./internal/workload` 与 `make d1-upgrade-selection-test` 均通过；模拟负例证明未知契约字段会在创建备份或调用任何应用 backup 之前停止。该证据只覆盖候选 IAM 文档解析，不替代完整 migration／Gateway／Workbench 兼容闭包账。

## OA C8 Business Engagement 间接资料投影（2026-09-29 单元／契约通过）

本批继续采用零容器、串行验证。Business Engagement 发布 `MaterialReferenceProjection v1`，只包含当前 Evidence 引用、Project ID、业务来源 revision／digest 和显示快照；Project 先以精确 `business-engagement.material-reference.read + business-engagement:{id}` 读取来源投影，再为每个 File 独立获取 `evidence-service.metadata.read + file:{id}`，没有跨库读取、共享 ORM 或在 Project 中复制业务领域模型。

实际依次执行：

```bash
cd backend/services/business-engagement
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../project-workspace
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过。覆盖 current-only 确定性投影、精确 IAM action／scope、严格 schema／tenant／project／digest 校验、direct 与 indirect 同时组合、来源整体拒绝的分段隔离、单 File Evidence 不可用的局部隔离、Evidence revision drift，以及跨 Project 来源在读取 Evidence 前失败关闭。全过程未启动 Docker、PostgreSQL、浏览器或持续运行应用。

当前判定 `c8-indirect-business-material-unit-supported / runtime-and-d1-pending`。新的 `000007` migration、真实 IAM／PostgreSQL／Business Engagement→Project→Evidence 独立进程链、浏览器交互、正式 D1／恢复和真实 Ubuntu 均未运行；本批结束时指标与 `WorkItemProjection` 尚未开始，后者由下一节的独立低资源批次补入。详细证据见 [C8 Business Engagement 间接资料投影单元验证](./reports/oa-c8-business-engagement-material-projection-unit-validation-20260929.md)。

## OA C8 Production 工作项 → Workbench 个人工作（2026-09-29 单元／契约通过）

本批继续使用零容器、串行验证。Production Operations 发布 `WorkItemProjection v1`，只根据已经验签的 Credential Subject 选择 active direct Person `production_owner` 的未完成工作项；Work Item ID、来源 revision、Project Scope、深链和服务端 `allowed_actions` 均由来源拥有。Unified Workbench 只从 Suite Manifest 发现已启用来源，在浏览器内经 Gateway 获取来源短凭证，并依靠 path-scoped HttpOnly Cookie 访问投影；页面没有 Subject 输入、没有令牌读取、没有 workload 身份冒充，也没有来源写操作。

实际依次执行：

```bash
cd backend/services/design-operations
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../unified-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../gateway
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

四组均通过；Production 与 Workbench 嵌入 JavaScript 的 `node --check` 通过，`deployments/d1/ops/render-suite-manifest` 的 `sh -n` 通过，`git diff --check` 通过。覆盖当前 Subject 选择、结束 Assignment／相邻 Subject／完成或取消项排除、精确 Project Scope、缺少投影能力 403、动作 Grant 撤销后空结果、引用来源不可用 503、Manifest 投影路径负例，以及 Workbench available／empty／forbidden／unavailable 逐来源状态和有界并发。全过程未启动 Docker、PostgreSQL、浏览器或持续运行应用。

当前判定 `c8-single-source-personal-work-unit-supported / gateway-browser-d1-pending`。正式 Gateway Cookie 链、真实浏览器点击、正式 D1／恢复、第二来源、指标投影、双 Tenant 和真实 Ubuntu 均未运行；这些缺口不能由本批单元证据替代。详细证据见 [C8 Production 工作项与 Workbench 单元验证](./reports/oa-c8-production-work-item-workbench-unit-validation-20260929.md)。

## OA C8 Money Flow 类型化指标来源（2026-09-29 单元／契约通过）

本批保持零容器、串行验证。Money Flow 发布 `MetricProjection v1`，把既有 current Operating Summary 的六个指标按 Tenant／Ledger Owner Party／Currency 输出；组合层不重算金额、不跨来源合计。`money-flow-ledger.statistics.read` 与 `money-flow-ledger.amount.read` 分权：没有金额能力时 `value` 和 `source_digest` 同时为 null 且 `redacted=true`，不能把未知值显示为 0，也不能从摘要反推小值域金额。

实际依次执行：

```bash
cd backend/services/money-flow-ledger
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./internal/identity ./internal/httpserver
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过，覆盖六个稳定 Metric Key、精确金额／摘要、脱敏零泄漏、缺结构权 403 且不调用来源、来源失败 503、`no-store`，以及发布 Schema／示例的严格字段和脱敏负例。全过程未启动 Docker、PostgreSQL、浏览器或持续运行应用。

当前判定 `c8-money-metric-source-unit-supported / consumer-and-d1-pending`。Suite Manifest／Workbench 或 Project 指标消费者、真实 Gateway／IAM、正式 D1、多来源分区、跨来源公式、双 Tenant 和真实 Ubuntu 均未运行；这些缺口不能由本批来源级证据替代。详细证据见 [C8 Money 指标来源单元验证](./reports/oa-c8-money-metric-projection-unit-validation-20260929.md)。

## OA C8 Money 指标 → Workbench 单消费者（2026-09-29 单元／契约通过）

本批继续使用零容器、串行验证。Suite Manifest v1 新增可选 `metric_projection`，发布示例与本地组合为 Money Flow 声明 `/api/v1/metrics/operating-summary` 和 `money-flow-ledger.statistics.read`。Unified Workbench 新增经营指标来源分区，经 Gateway 为当前 Session 取得来源路径短期 Credential，严格校验 schema、source、Tenant、Ledger Owner Party Scope、精确十进制／摘要或 redaction 不变量；页面不持久化指标、不跨来源合计，也不把无权金额显示为 0。

实际依次执行：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../gateway
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../../acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过；Workbench 嵌入 JavaScript 的 `node --check`、Manifest JSON 的 `jq empty`、D1 Manifest 实际渲染／字段断言和 `git diff --check` 通过。覆盖 Manifest 未知版本／非法路径／跨 namespace／平台应用负例、来源发现、Tenant 校验、精确值、金额脱敏、深链限制、来源内排序去重，以及 forbidden／unavailable 只降级单一来源的页面语义。全过程未启动 Docker、PostgreSQL、浏览器或持续运行应用。

当前判定 `c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending`。真实 Gateway→IAM→Money Cookie 链、真实浏览器、Grant 撤销、第二指标来源运行时局部失败、正式 D1／恢复、双 Tenant 和真实 Ubuntu 均未运行；这些缺口不能由本批消费者单元证据替代。详细证据见 [C8 Money 指标 Workbench 消费者单元验证](./reports/oa-c8-money-metric-workbench-consumer-unit-validation-20260929.md)。

## OA C8 Evidence 同 File 多版本、正式 stale 旅程与 D1 升级（2026-09-29 当前切片通过）

专用 C8 Project／Evidence 记录准备完成后，正式 stale 旅程无法继续：Evidence 数据模型虽然包含 Version，migration 和公开写入路径却只允许 Version 1。该缺口不能用改库或测试夹具冒充产品能力，因此新增 [BF-078](./decisions.md)／[ADR-0036](./adr/0036-evidence-files-use-stable-identity-and-immutable-monotonic-versions.md)，实现同 File 的服务端递增版本会话、稳定引用最新版和显式历史引用。Evidence Suite 页面同时识别 `/evidence/` 路径并复用 Gateway／IAM HttpOnly 能力 Cookie；direct 模板入口继续保留人工 Token，两个模式不互相冒充。

低资源验证只启动一个受限 PostgreSQL 18.6 临时容器，限制为 `0.5 CPU / 384 MiB`；应用测试使用 `GOMAXPROCS=2`、`GOWORK=off`、`nice -n 10` 和 `go test -p 1`。实际结果：

- `backend/services/evidence-service` 全量单元测试通过；
- `internal/evidence` 与 `internal/httpserver` 的 integration tag 在真实 PostgreSQL 18.6 上通过，覆盖 Version 1 → Version 2、等价重放／载荷冲突、稳定引用前进、显式 Version 1 不变和 Version 2 访问授权；
- `acceptance/evidence-service-demo` 扩展为 10 步并通过编译／测试，短期授权明确绑定 Version 2；
- Evidence 嵌入 JavaScript 语法检查通过；
- 临时数据库退出并删除后，本机运行容器数回到 0。

`internal/systembackup` 的 integration 用例在本轮隔离数据库命令中没有执行成功，因为该测试的 `local-pg-dump／restore` 固定调用 Evidence Compose 中名为 `postgres` 的服务，而本轮为避免 Mac 负载没有启动第二套 Compose。失败信息为 `service "postgres" is not running`，发生在 `pg_dump --version` 前置探针，不是版本数据或恢复断言失败。测试已扩展为要求一个 File 的两个 Version、两个对象和两个摘要均恢复；公司 D1 随后的联合备份已实际覆盖 9 个 File、10 个 Version 和 10 个对象，但专用空目标恢复仍须单独执行后才能关闭恢复门禁。

Evidence `v0.1.0-test.7` 的 Linux AMD64 digest 为 `sha256:4145253fb806fab382fe03a42dcbda335ed7597095fb4e10421f0a58e858ebdf`。ACR 重新登录后，候选 IAM `test.8` 又补齐无数据库 `validate-contracts`；Gateway 与 Workbench 因 Suite Manifest 新增 `metric_projection` 而必须进入同一兼容闭包，最终分别以 `test.10` 摘要 `sha256:d850dfefe8be4e3b80ffbab4335492723126e8f0d061c3069f80c8fce5c70874` 和 `sha256:808ee99f26f49e596b2aab89f5a41b58f98e18528fc340c1d123e942d467cdfe` 发布。IAM `test.8` 摘要为 `sha256:69806b6fbf055dcdbb50dcc93a135149e0a523684213ef4607bcc1c21a006266`，Project 继续使用 `test.6` 摘要 `sha256:daa0554e338e472e0f192d4a04325495e9fe0b2b59a67c51a0f2b0e9bf331e47`。

正式 `Nginx → Gateway → IAM → Evidence → Project` API 旅程没有直接写库，实际得到：

- Project `c9cefbce-def2-4de1-a7f4-56e2f71dd271` 从 revision 2 前进到 4；
- File `d0d49592-df01-450c-aed6-e396d6149b27` 的 Version 1 摘要 `356083ab…430a` 保持不变，Version 2 摘要为 `0b1e29c2…4e75`；
- Project 资料状态严格经历 `available@v1 → revision_changed@v2 → available@v2`；
- 旧 Link `dfeb1acf-51be-4347-ab3b-5b0a36f5d9be` 被正式重连为 `3da2f74a-fce7-4e10-b7d1-18fc8426807b`，稳定 File ID 未改变。

升级过程还实际暴露并修复两项运维缺口：旧的一次性 migration 容器会被 Compose 复用，Evidence 在备份后用 `compose start` 恢复时又会错误重算 migration 依赖。`d1 upgrade` 现于备份后执行不删除命名卷的 `down --remove-orphans` 再重建候选栈；Evidence 恢复使用 `up --detach --no-deps --no-build --wait`。`test-release-bundle` 已加入静态回归断言。

公司 D1 完整 `v0.1.0-test.10` 升级已通过：升级前备份 `20260929T132146Z` 校验有效，所有 migration 退出 0，最终 12/12 长期容器 healthy，`d1 check` 通过；较早完整备份 `20260929T131201Z` 同样校验有效。`20260929T130746Z` 缺少最终校验清单，明确不可作为恢复材料。当前判定更新为 `evidence-multiversion-and-project-stale-d1-supported / human-browser-and-empty-target-restore-pending`；真人浏览器下载／分权点击和独立空目标恢复仍未通过，C8 与阶段报告继续保持 `insufficient-evidence`。

## OA C8 Money 指标正式 Gateway／IAM／D1（2026-09-29 通过）

Money Flow 全量测试先以 `GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...` 串行通过；随后只构建并推送一个 Linux/AMD64 Money 镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/money-flow-ledger:v0.1.0-test.11@sha256:9567ada8efd030b927bb5e38cc99156ad7a1e56796880cefb5d77b01289fe6e1
```

`v0.1.0-test.11` D1 release 通过 `preflight-upgrade` 和完整 `d1 upgrade`；升级前联合备份 `20260929T133853Z` checksum 有效，所有 migration 完成，最终 12/12 长期容器 healthy、unhealthy 0、Nginx `/health/ready` 200。本机没有启动业务容器。

正式验收入口为 `acceptance/d1-c8-money-metric/verify-running`。第一次运行只得到 HTTP 200 空集合，脚本主动失败且没有创建临时账号；第二次仅在空集合条件下通过 Money 正式页面 API 建立两个 `C8-METRIC` Party 和一条 `1234.56 CNY` Cash Transaction，再开始权限序列。实际结果：

- 管理员经 `Nginx → Gateway → IAM → Money` 得到 6 个非空类型化指标；
- 临时账号只有 `money-flow-ledger.view` 时，指标端点返回 403；
- 增加 `statistics.read` 并重新签发后，6/6 项均为 `redacted=true`，`value`／`source_digest` 均为 null；
- 增加 `amount.read` 但不重新签发时，旧凭证仍保持脱敏；重新签发后 6/6 项返回精确十进制字符串和 `sha256:` 摘要；
- 撤销 `amount.read` 后，旧凭证在最长 300 秒 TTL 内保留有界残留；重新签发立即恢复 6/6 脱敏；
- Money 业务事实逐表行数和完整排序行摘要前后相同：`cc60c4c734f4248a64d54f3e96111fc65bce1a76afef0af68ef9f075d09ff8b7`；
- 临时 Subject 最终为 `disabled`，活动 Grant 为 0；服务器保持 12/12 healthy。

当前判定由 `c8-money-metric-single-consumer-unit-supported / gateway-browser-d1-pending` 更新为 `c8-money-metric-d1-supported / workbench-human-browser-and-second-source-pending`。它关闭 Money 指标的正式 D1 来源链缺口，但不关闭 Workbench 真人浏览器、第二指标来源、Production 工作项、Business Engagement 间接资料、双 Tenant、HTTPS 或空目标恢复。详细证据见 [C8 Money 指标正式 D1 权限与只读边界验证](./reports/oa-c8-money-metric-d1-validation-20260929.md)。

## OA 完整 Suite 公司 D1 装配（2026-09-29 通过）

缺失的 Billing、Approval／Results、Cost、Bonus 先分别以 `GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...` 串行通过，镜像按 Linux/AMD64 单个构建、推送和摘要核验；本机全程没有同时启动业务容器。Bonus 发布实际发现并修复两项会阻断客户部署的问题：Dockerfile 未复制 `packages/payable-suggestion-contract`，以及正式 IAM 模式在 workload credential 建立前错误要求 Money 静态 Token。修复后 Bonus module 全量测试与 `go vet ./...` 通过。

公司服务器使用不可变候选 `v0.1.0-test.20`，同时启用 12 个业务应用。`preflight-upgrade`、正式 `upgrade` 和独立复核结果为：

- 17/17 长期服务 healthy，unhealthy 为 0；
- Nginx `/health/ready` 返回 HTTP 200；
- Contract、Money、People、Party、Project、Business Engagement、Evidence、Production、Results、Billing、Cost、Bonus 十二套 migration 全部 current；
- `d1 check` 通过；
- 升级前备份 `20260929T155403Z` 含 15 项受保护材料，checksum 全部通过；
- `suite-selection.env` 的十二个 `ENABLE_*` 均为 `true`；
- 结束时服务器约 817 MiB available memory、约 2 GiB swap 可用。

该轮判定是 `full-selection-assembly-supported / continuous-c1-c8-journey-and-empty-target-restore-pending`。它证明完整组合能在真实 2 vCPU／2 GiB 档重复装配，但不把独立空目标恢复、恢复后继续写入或同一数据集的 C1～C8 连续业务旅程冒充为已通过。详细命令、镜像摘要、故障与恢复过程见 [完整 Suite D1 装配验证](./reports/oa-full-suite-d1-assembly-validation-20260929.md)。

## OA 完整 Suite 公司 D1 隔离空目标恢复（2026-09-30 通过）

来源备份 `20260929T155403Z` 被恢复到独立 Compose project `omsbizfoundry-restore20`、独立入口 `127.0.0.1:18120` 和全新命名卷；主项目全过程保持 17/17 healthy。恢复工具只在有界窗口临时交接所选备份目录，并在结束时恢复 `root:root`、目录 `0700`、文件 `0600`。

隔离栈最终 17/17 healthy、十二个 schema current；Contract 以前向 `000019_party_runtime_transitions.sql` 和新摘要 `sha256:cf84233d9cb02071e9f03c7fc086f6c1430b4d3074977d697811de6759805af9` 对齐当前 Capability 契约。`acceptance/d1-delivery/verify-running` 完整通过，覆盖 Nginx-only、单次登录、启用应用读写、Contract／Money Party 预检、People 名册、Party→Project 精确引用和 Evidence 字节生命周期。

恢复写入使 Evidence 从 `9 files / 10 versions / 10 objects` 增为 `10 / 11 / 11`；随后新备份 `20260929T171025Z` 的全部 checksum 通过。验收还暴露并修正完整 128 Grant 必然超过 120/min 限流、Bonus 相对 API 路径误判、旧 Contract 镜像代际不一致和 AppleDouble 临时构建上下文污染；失败的 `test.25` 明确作废，接受候选为 `v0.1.0-test.26`。

当前判定更新为 `full-selection-empty-target-recovery-and-post-write-supported / continuous-c1-c8-journey-pending`。它关闭公司 D1 同宿主隔离空目标恢复门槛，但不代替离机跨机器恢复、公网 HTTPS、完整 C1～C8 连续旅程或真人浏览器关键权限路径。详细证据见 [OA 完整 Suite 公司 D1 隔离空目标恢复验证](./reports/oa-full-suite-d1-empty-target-recovery-validation-20260930.md)。

## 参考 OA 源级界面复用与 Workbench D1 替换（2026-09-30 通过）

本轮纠正了“仅做相似后台样式”的错误验收口径。Unified Workbench 直接复用用户自有参考 OA 的登录、侧栏、管理标题、项目列表、项目详情、项目资料和统计 CSS／class 结构；真实 IAM 用户名／密码、CSRF、Gateway 授权表单、个人工作与指标来源契约保持不变。`frontend/oms-workbench` 在 `GOMAXPROCS=2 GOWORK=off go test -p 1 ./...` 与 `go vet ./...` 下通过，浏览器逐页检查登录、立项、项目列表、资料、统计和项目详情的显示与切换通过。

只发布并替换 Linux/AMD64 Unified Workbench 镜像 `v0.1.0-test.42@sha256:5f9a74bbcac129fa7499153bb62a32d303cd162350642c7fbb29091607cdfb3f`；公司 D1 对应容器为 `healthy/running`，其他服务未重建。它证明参考 OA 界面可以装在既有 Nginx→Gateway→IAM→应用装配链上，但静态界面验证数据仍需逐项换成真实投影，不能据此把完整 OA 最终结论从 `insufficient-evidence` 改为通过。

### 管理页面源级扩展（2026-09-30）

在上述批次后继续直接复用参考 OA 的 Settings.css、UserManagement.css、PermissionSettings.css 与 ClientManagement.css，并补齐可点击到达的设置、人员管理和权限配置页面。三页在本地浏览器逐页检查通过，返回路径正常且无浏览器 error/warning；Go test、vet、JavaScript 语法检查与 diff 检查通过。

只替换公司 D1 的 Unified Workbench 为 v0.1.0-test.43@sha256:898a8badc07cf151aeebef10f90a40dfb913c7380158a470596b48c3c2007356。替换后 17/17 服务 healthy，其他服务未重建。页面中的人员和权限条目仍属于界面验证数据，真实 People/IAM 写入尚未绑定，因此这批证据是 UI/UX 路由对齐证据，不是权限业务验收通过。

### 全参考路由补齐与登录页 D1 真人复核（2026-09-30）

继续从用户自有参考 OA 直接复用 Company Revenue、Revenue Project Ledger、Contract Form、Contract Amendment、Client Management 与 Usage Guide 的 CSS／class 结构，补齐甲方、资质单位、公司收入、项目收入、合同详情和使用指南。登录页沿用参考 OA 的品牌标题、水印背景和居中卡片，但表单继续提交真实 IAM 用户名／密码，不伪造短信验证码服务。

本地在单 Workbench 进程和轻量 Actor Header 代理下实际点击项目列表、项目详情经营入口、甲方列表与详情抽屉、公司收入响应式布局、合同文件页签和使用指南折叠问答；1280px 视口没有横向溢出。`frontend/oms-workbench` 的串行单元测试、`go vet`、嵌入 JavaScript `node --check` 与 `git diff --check` 通过。

只替换公司 D1 的 Unified Workbench 为 `v0.1.0-test.44@sha256:58714f4d04c6b7a08250fb19d73859c114ca45a0c4cd865de08b56019971c47b`，AMD64 manifest 为 `sha256:916a8771167374dfcb092a0941f101b77d91d0267b879106f7ed58692166e7b8`。替换后主栈 17/17 healthy，其他 16 个服务未重建。容器内可信 Actor 页面探测确认六类新增路由标记与业务样式随镜像交付；`http://127.0.0.1:18090/workbench/` 返回 200，真人浏览器实际确认登录页品牌、背景与表单版式。登录后的 D1 全路由人工点击仍未执行，因此本批结论是 `reference-oa-ui-route-parity-d1-published / authenticated-d1-click-through-pending`，不是完整 OA 业务通过。逐页矩阵见 [参考 OA UI/UX 逐页对齐审计](./reports/oa-reference-ui-parity-audit-20260930.md)。

### 项目详情深层工作台复用与 D1 单服务替换（2026-09-30）

继续按参考 OA 的 `ProjectDetail`、`BasicInfoTab`、`BusinessInfoTab`、`ProductionInfo` 与 `ProjectWorkspace` 层级补齐项目详情，不再用四张应用入口卡代表完整经营／生产工作面。经营 9 个二级页签和生产 11 个二级页签均进入同一项目详情外壳；正式写入口继续走 Gateway 授权到独立应用，静态金额和项目记录明确标注为界面验证数据。

本地以单 Workbench 进程实际点击基本信息、经营总览、合同、生产总览和施工图阶段；页签切换、快捷入口与文件列表正常，浏览器控制台 0 error/warning。实际执行并通过：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
GOMAXPROCS=2 GOWORK=off nice -n 10 go vet ./...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check -
git diff --check -- frontend/oms-workbench/internal/server
```

随后只构建并发布 Linux/AMD64 Workbench：

```text
v0.1.0-test.45@sha256:d23d4959436c73818b1a265445541553eb2343c13cdc1092dd5c510b5b3ae155
AMD64 manifest sha256:7f6fb9b6b2db0c1da51907bbfee0eeed5d0d0631ac63df9f5f1907b98d027680
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，主栈保持 17/17 healthy；其他 16 个服务未重建。容器内可信 Actor 探测确认经营／生产工作台模板和深层 CSS，页面 `159591` bytes、CSS `15659` bytes。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui45-full.env`。D1 真人浏览器当前停在真实 IAM 登录页，登录后深层页签点击仍待验；结论为 `reference-oa-deep-project-workspace-d1-published / authenticated-d1-click-through-pending`。

### 资料与统计深层交互及 D1 单服务替换（2026-09-30）

按参考 OA 的 `FileManagement.tsx/.css` 与 `Statistics.tsx/.css` 补齐此前没有行为证据的 P0 页面：项目资料从收起状态开始，支持多个项目独立展开、键盘 Enter／Space、文件下载摘要和多人明细弹窗；统计提供五个真实可切换页签，人员奖金可展开项目金额，人事考勤／薪酬使用参考空状态。真实文件下载继续经 Gateway→Evidence 授权，经营指标继续按来源读取，不把静态界面样本写入业务库。

本地真人浏览器实际通过：资料项目展开后看到 3 个文件，下载人员弹窗看到 4 人；统计 `ledger/company/attendance/payroll/personnel-bonus` 五个 pane 均可见，李晨奖金展开看到 3 个项目；浏览器 console 0 error/warning。串行单测、vet、嵌入脚本语法和空白检查通过。

公司 D1 只替换 Workbench 为：

```text
v0.1.0-test.46@sha256:855389824a9af82c1c27becf2fce8d507db796149cc3812090fa064da52db54c
AMD64 manifest sha256:a562b44513d6d6e1cb7c4f8b1beb0160aeefb5986d603ed861af240a4d7bff23
```

最终 `omsbizfoundry` 原项目 17/17 healthy，其他 16 个服务未重建；页面、脚本和 CSS 容器探测分别为 `159813`、`17561`、`75440` bytes，新增模板／交互标记存在，SSH 隧道返回 200。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui46-full.env`。D1 IAM 登录后的逐页人工点击仍待验，结论为 `reference-oa-p0-deep-interactions-d1-published / authenticated-d1-click-through-pending`。

## 参考 OA 项目／人员／权限交互与登录首屏纠偏（2026-09-30）

低资源本地验证只启动一个 Unified Workbench 进程，没有启动本地 Compose 或数据库：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
GOMAXPROCS=2 GOWORK=off nice -n 10 go vet ./...
node -e "const fs=require('fs'); new Function(fs.readFileSync('internal/server/reference_oa_login.js','utf8'))"
git diff --check -- internal/server
```

全部通过。本地浏览器实际完成以下断言：

- 项目列表搜索／经营阶段组合只显示 `S207省道改扩建工程`；分页第 1 页显示两项，第 2 页只显示第三项；
- 人员搜索／角色筛选只显示 `赵倩`，点击后详情抽屉显示同一人员；
- 权限区域切到“项目角色／生产负责人”，取消 `project.write` 后保存可用，确认后出现“未写入真实 IAM”的验证提示；
- 未认证页默认显示参考 OA 的手机号验证码首屏；合法手机号启用“获取验证码”，点击后显示“当前验证环境尚未接入短信服务”；可切换到真实 IAM 用户名／密码表单；
- 1280×720 无横向溢出，浏览器 console 为 0 error／warning。

最终只发布并替换公司 D1 的 Workbench：

- 镜像：`unified-workbench:v0.1.0-test.48@sha256:ad5996bb12975ec4cd73aa3edef17529be01349ce3c60da2b30b08c6c11fa28c`；
- OCI index：`sha256:acb7880ad6e0104f448b0fd5b4bb71922e06504575e742f54bae8332cbba5331`；
- 主项目运行结果：17/17 healthy，其他 16 个服务未重建；
- D1 匿名入口与静态资源探测确认验证码／IAM 双面板、手机号文案、`phonePattern`、`setMode` 和未接短信提示均已交付；
- 回退环境备份：`/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui48-oa-login.env`。

此批证明登录 UI 和切换交互已按参考 OA 纠偏，不证明短信发送、验证码校验或短信 Provider 已实现；真实身份验证仍由 IAM 用户名／密码完成。登录后 D1 全路由真人点击仍是独立待验项。

## 参考 OA 项目立项深层交互（2026-09-30）

低资源本地验证继续只启动单个 Unified Workbench 与轻量 Actor Header 代理，没有启动本地 Compose 或数据库。实际执行：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
GOMAXPROCS=2 GOWORK=off nice -n 10 go vet ./...
node -e 'const fs=require("fs");const s=fs.readFileSync("internal/server/server.go","utf8");const start=s.indexOf("const workbenchScript = `");const end=s.indexOf("`\\n",start+25);new Function(s.slice(start+25,end))'
git -C ../.. diff --check -- frontend/oms-workbench/internal/server
```

全部通过。浏览器实际断言：

- “关于我的”显示 2 条；切换“全部项目”显示 3 条，第 2 页只显示“南城片区综合管廊设计”；搜索“北环”只显示项目包；
- 新建抽屉从普通项目切到项目包后标题、说明、类型提示与按钮同步变化；空项目名称／负责人显示错误，填入后出现“不伪造写入”的校验提示；
- 点击列表条目打开完善立项抽屉，项目名称与编号正确带入；非法编号 `bad number` 被拒绝，合法编号通过；
- 删除确认显示项目名和不可恢复警告，确认后只出现“未写入真实业务数据”的提示，列表事实保持 3 条；
- `scrollWidth=clientWidth=1239`，浏览器 console 0 error/warning。

最终发布：

```text
unified-workbench:v0.1.0-test.49@sha256:50f4a647235dbb3e2bd4ec4b47eb7de22e765e140b453d623b1b14a14fbd7027
OCI index sha256:e3406da07b84a5489314dd0f6f95822f619d51d10a99170a78bec1b98d0b7cd5
```

公司 D1 只替换 Workbench，主栈 17/17 healthy，其他 16 个服务未重建；容器内验证 `data-page-template="initiation-reference"`、create／complete drawer、Project Workspace handoff、`updateInitiationWorkspace`、立项 CSS 和验证码登录标记均存在。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui49-initiation.env`。真实 D1 IAM 登录后的全路由逐项点击和静态样本的版本化投影绑定仍待完成。

## 参考 OA 只读主界面演示入口（2026-09-30）

真人反馈证明匿名登录页不能作为“仿 OA 网站”的验收页。新增 `?preview=oa` 后执行：

```bash
go test -p 1 ./frontend/oms-workbench/...
go vet ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server
curl -fsS 'http://127.0.0.1:18090/workbench/?preview=oa'
```

测试、vet、空白检查和 D1 HTTP 200 均通过。浏览器实际验证：项目年表存在 2026／2025 三条演示记录；进入“京哈高速改造工程”后基本信息可见；切换经营信息后经营概览、金额卡和 9 个二级入口可见；点击 Gateway 表单只显示“当前是只读界面演示；真实业务操作必须通过 IAM 登录”，页面未跳转，console 为 0 error／warning。

安全断言：预览请求未携带真实 Subject 时使用仅供 DOM 的演示标识，服务端主动移除 Work／Metric 受保护投影来源；客户端拦截全部 `/gateway/*` 表单。正式未认证入口仍显示登录页，受保护数据与写入仍必须经过 Nginx → Gateway → IAM。

公司 D1 接受镜像为 `unified-workbench:v0.1.0-test.50@sha256:159af0e3c102c9533ac1bb3c5526e7561898f2575e3110127a7a3ac226f40f1e`，OCI index 为 `sha256:f96c0b817a5dbd70ae60dfb1c815c83bf37a659e70fd194133884f8465024623`。仅 Workbench 重建，原项目 17/17 healthy；回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui50-preview.env`。该证据只关闭“登录页阻挡 UI 验收”，不替代真实登录、投影绑定或业务连续旅程。

## 参考 OA 项目包／子项目层级交互（2026-09-30）

本地仍只启动单个 Unified Workbench，不启动 Compose 或数据库：

```bash
go test -p 1 ./frontend/oms-workbench/...
go vet ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。浏览器在本地和公司 D1 分别实际断言：项目包初始展开且含两条缩进子项目；点击后 `aria-expanded=false` 且子项目容器隐藏；搜索 `S207` 后项目包自动展开、摘要为“共 1 个项目”；点击第一条子项目后 active view 为 `project-detail`，详情顶部为 `S207省道改扩建工程 / S207-2026-006 / 承接日期 2026-08-16 / 经营负责人 王可 / 生产负责人 待设置`，侧栏仍选中项目管理。两处 console 均无 error／warning。

发布结果：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.51@sha256:ae07d038fba4af54f20f64f0423d4003be89061bc51a1a301d850ba0f668836c
```

公司 D1 最终用 `docker compose -p omsbizfoundry ... up -d --no-deps unified-workbench` 只替换主栈 Workbench，主栈 17/17 healthy，其他 16 个主栈容器 ID 与更新前一致。一次未指定 project name 产生的孤立 `d1-unified-workbench-1` 和 `d1_app` 网络已清理，未接入主 Gateway，也未影响主栈。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui51-project-package.env`。

该证据证明参考 OA 的项目层级、展开／搜索和详情上下文交互已经进入 D1，只读样本仍不等于真实 Project 投影或写入通过。

## 参考 OA 项目包／子项目详情层级（2026-09-30）

低资源本地门禁：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。单 Workbench 进程浏览器断言：旧 `initiation/files/statistics/project-detail-legacy` 目标为 0；参考模板目标均完成 hydration；项目包详情 class 为 `project-detail-page--package`，含两条子项目导航；S207 详情 class 为 `project-detail-page--child`，所属项目包路径、正文名称／编号／日期／负责人、经营阶段与生产阶段均正确；返回京哈普通项目后角标和项目包导航均为 0。项目包页 `documentElement.scrollWidth=clientWidth=1280`。验收后本地进程已停止。

公司 D1 以完整 ACR manifest 摘要单服务发布：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.52@sha256:55bb75727845df8e2eb6510919658167645b2202712e74165bbb97882130b83b
```

`docker compose -p omsbizfoundry ... up -d --no-deps unified-workbench` 后主栈 17/17 healthy，其他 16 个容器 ID 逐项不变。D1 真人浏览器复核项目包角标／子项目导航、S207 子项目角标／所属项目包路径和正文上下文；1280 像素视口无横向溢出。更新前环境备份为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui52-project-detail-hierarchy.env`。

首次 pull 使用了终端截断摘要，ACR 在重建前返回 not found；从 tag 重新读取完整 manifest 后成功发布，旧容器在失败期间保持运行。该证据不等同于真实 Project 投影或写动作通过。

## 参考 OA 主体／人员维护抽屉与删除保护（2026-09-30）

低资源门禁继续只运行一个 Workbench 构建，不启动本地 Compose：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。浏览器先在本地验证甲方新增／编辑字段、人员新增／编辑预填和提交后的“不伪造写入”提示；D1 发布后真人点击验证：

- 项目首页和京哈项目详情正常；
- 甲方新增抽屉字段完整，编辑抽屉带入主体既有值；
- 点击“删除甲方”显示“当前档案暂时不能删除”及关联项目处理提示；
- 资质单位新增抽屉字段完整；
- 人员新增抽屉包含账号信息、本地资料、角色与权限三段；
- 李晨编辑抽屉带入 `lichen`、`li.chen@example.cn`、`李晨`、`经营部`、`经营经理` 和启用状态。

删除保护初版随 `test.53` 到 D1 后，真人点击发现弹窗实际位于隐藏的资质单位 view，甲方删除只关闭详情。修正为甲方／资质单位各自页面内的删除弹窗，并增加 `strings.Count(... data-entity-delete-dialog) == 2` 测试后重新发布：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.54@sha256:407e94db48244aabe84b49eb9699fbb83dbd69d88fdb06c624637da3b9b1752a
AMD64 manifest sha256:6f9c53029a7f70165b41894e606df609f95af2cbd15de887039f1f8983482d83
```

最终用 `docker compose -p omsbizfoundry ... up -d --no-deps unified-workbench` 只替换 Workbench；D1 17/17 healthy，其他 16 个容器 ID 与发布前一致。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui54-delete-guard.env`。本节只证明源级管理交互已可见且不会越权伪造写入，不证明 Party Directory、IAM 或 People & Groups 的真实写命令已经由 Workbench 接通。

## 参考 OA 项目基本信息编辑抽屉（2026-09-30）

只运行一个本地 Workbench 进程，没有启动本地 Compose 或数据库。实际通过：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
node -e 'const fs=require("fs");const s=fs.readFileSync("frontend/oms-workbench/internal/server/server.go","utf8");const start=s.indexOf("const workbenchScript = `");const end=s.indexOf("`\\n",start+25);new Function(s.slice(start+25,end))'
git diff --check
```

本地浏览器真人断言：

- 京哈普通项目打开“编辑项目基本信息”后，名称、编号、承接日期、资质单位、15.00% 管理费率、概况、两类项目标签和两名负责人全部正确预填；
- 点击保存出现“界面校验已通过”，且明确不写真实业务；
- 北环项目包打开同一抽屉后，编号标签为“项目编号（选填）”，经营负责人为李晨，生产负责人保持未选择；
- 抽屉层级、遮罩、字段两列布局、字数计数和底部操作与参考 `BasicInfoTab.tsx` 一致。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.55@sha256:915624572ae03063a2f9c101f445960bccff8517069f42c394333275c0992cb4
AMD64 manifest sha256:d68745a3172ced5f99e90c8bc76f9afc0e58705464495f290bb1a5624a6b1db4
```

公司 D1 仅重建 `omsbizfoundry-unified-workbench-1`，最终 17/17 healthy；其他 16 个容器 ID 逐项保持不变。D1 真人浏览器重复验证项目列表 → 京哈详情 → 编辑抽屉并看到全部预填字段。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui55-project-profile-drawer.env`。本项是参考 UI/UX 和表单路径证据，不是 Project Workspace 真实写命令已经通过的证据。

## 参考 OA 项目层级高风险操作（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
node -e 'const fs=require("fs");const s=fs.readFileSync("frontend/oms-workbench/internal/server/server.go","utf8");const start=s.indexOf("const workbenchScript = `");const end=s.indexOf("`\\n",start+25);new Function(s.slice(start+25,end))'
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。浏览器本地实际断言：

- 京哈普通项目显示“项目包调整（试用）”与“删除项目”；转换模式在“新项目包字段”和“目标项目包”间互斥显示，安全预检后必须输入 `确认调整项目层级` 才能执行演示确认；
- 普通项目删除输入错误文字时按钮保持禁用，精确输入 `确认删除项目` 后启用；
- 北环项目包“添加”打开源级添加子项目抽屉，名称／编号校验通过后只显示不写入提示；项目包删除必须输入 `确认删除项目包及全部子项目`，并显示将同时影响 2 个子项目的风险；
- S207 子项目显示“停用子项目”，确认弹窗准确显示当前子项目并说明保留已有生产、收付款和文件事实；
- 浏览器 console 0 error。测试中发现 CSS 网格覆盖 HTML `hidden`，已增加 `.project-package-conversion__fields[hidden]` 等强制隐藏规则并复验。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.56@sha256:17cab9a49dc7b26e18a2d28334ea2f415dd24f09b91802c63e976d3389eedb10
AMD64 manifest sha256:c1a9ca083a13823800d5952a365fd87c174e5b838aa10b42de558530e9a84c79
```

公司 D1 仅重建 `omsbizfoundry-unified-workbench-1`，最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。D1 真人浏览器重复验证转换模式互斥、添加子项目抽屉、项目包删除保护和 S207 停用弹窗，console 0 error。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui56-project-actions.env`。这些交互证明参考页面保护路径已进入 D1，不证明真实项目层级写命令已经通过。

## 参考 OA 经营合同双链与维护抽屉（2026-09-30）

低资源门禁只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
node -e 'fetch("http://127.0.0.1:19090/assets/workbench.js").then(r=>r.text()).then(s=>{new Function(s)})'
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地真人浏览器断言：

- 京哈项目“经营信息 → 合同信息”显示甲方合同／京宏合同两条业务链和各自合同卡片；
- `JH-JF-2026-018` 抽屉预填合同号、日期、费率、设计／勘察／咨询费，原合同金额为 `¥8,600,000.00`，含补充后的结算金额为 `¥8,920,000.00`；
- 修改设计费后原合同金额和同步结算金额联动为 `¥8,700,000.00`，保存只显示“不写真实业务”提示；
- 删除确认准确带入合同号和当前金额，确认只显示保护提示；
- 新增京宏合同时删除入口隐藏，补充／定案区域锁定；填入合同号、日期和费用后校验通过；
- 浏览器 console 0 error／warning，本地进程随后停止。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.57@sha256:13cd4891daf757c20777049ef821e3945891f5e85406b329a1af786299a94498
AMD64 manifest sha256:f06b472dbaedd600728c407be568d9a4c0009965590b5dcf6d41353074f25e4d
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。D1 真人浏览器重复项目列表 → 京哈项目 → 经营信息 → 合同信息 → 甲方合同抽屉，合同号、结算金额、三类费用、补充协议和定案表均正确，console 0 error／warning。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui57-contract-drawer.env`。本节证明参考合同 UI/UX 与交互保护已经进入 D1，不证明 Contract Ledger 真实写命令已经通过。

## 参考 OA 招投标文件工作面（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check
```

全部通过。本地真人浏览器断言：

- 京哈项目“经营信息 → 招投标信息”显示项目承接方式，且招标文件／投标文件／中标文件均为已上传；
- 页面显示 `已处理 3 / 3`、资料已处理、最近更新中标通知书和三张参考 OA 文件槽位；
- `京哈高速改造工程招标文件.pdf` 预览抽屉准确显示招标文件、5.8 MB、2026-02-12 与 Evidence v1；
- 承接方式切换再恢复、继续上传、下载和删除确认都显示不写真实业务的保护提示；
- 浏览器 console 0 error／warning，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.58@sha256:61c140994ca151d653fd84e1bca7f26556f8c5bcbac4c64433683b4c54fc9a17
AMD64 manifest sha256:e6d1c6ef394a8a8f8b5e2adac9bf9b6206221d78433a5f63da4244cc7ba02e73
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。通过 Nginx 入口探测页面／脚本／CSS 标记，D1 真人浏览器重复项目列表 → 京哈项目 → 经营信息 → 招投标信息 → 招标文件预览，字段正确且 console 0 error／warning。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui58-bidding-files.env`。本节证明参考招投标 UI/UX 与交互保护已经进入 D1，不证明 Business Engagement／Evidence 的真实投影或写命令已经通过。

## 参考 OA 支付／开票双链与维护抽屉（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check -
git diff --check
```

全部通过。本地真人浏览器断言：

- 京哈项目“经营信息 → 支付信息”按甲方侧／收入侧展示两条业务链和 5 条记录；设计费支付抽屉准确带入 `2026-09-10`、`¥3,440,000.00`、华北交通建设有限公司、关联发票与备注；
- 更新动作只显示 Money Flow Ledger／IAM 的正式写入边界，不改变界面验证数据；
- “开票信息”同样显示两条业务链和 5 条记录；设计费开票抽屉准确带入 `2026-09-08`、`¥3,440,000.00`、关联支付及 `京哈高速设计费发票-202609.pdf`；
- 发票文件区域明确由 Evidence Service 保存版本与受控访问地址；新增、更新和删除均有只读保护；
- 本地浏览器 console 0 error／warning，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.59@sha256:3ef9575b25a694ac79e0aa25ae8a714e6344f545ef06a79eec9c9f15e446cf17
AMD64 manifest sha256:922118ddd2b914c2bff6911b5820f28f1bf13f35ac918f0f840c3987b9e5529f
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。Nginx 入口的页面／CSS／JavaScript 标记与脚本语法检查通过；D1 真人浏览器重复支付详情与开票详情，字段正确且 console 0 error／warning。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui59-payments-invoices.env`。本节证明参考支付／开票 UI/UX 和应用边界提示已经进入 D1，不证明静态记录已经绑定 Money Flow Ledger／Billing 的真实投影或写命令。

## 参考 OA 经营成本与奖金分配工作面（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check -
git diff --check
```

全部通过。本地真人浏览器断言：

- “经营信息 → 经营成本”显示 `¥620,000.00 / 6 条` 摘要、费用色成本行、日期、金额、说明和发票摘要；中标服务费详情准确带入 `2026-09-28`、`¥320,000.00`、说明和发票名；
- 成本更新、文件选择／预览／下载和删除只显示 Cost & Outsourcing／Evidence Service／IAM 边界，不改静态样本；
- “奖金分配”显示 `¥3,834,000.00` 基数、`¥180,000.00` 奖池、v3 已发布、4 次／4 人和四条人员分配；赵明详情准确带入 35% 与 `¥63,000.00`；
- 发放方式切成固定金额并输入 50000 后，比例池变为 `—`、最终奖金变为 `¥50,000.00`；保存和删除保护明确要求 Bonus Distribution 版本历史；
- 本地浏览器 console 0 error／warning，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.60@sha256:79f821b9ca614fe7a484ae6560b70750ca5e45b312836680982ee5883c44c69a
AMD64 manifest sha256:5d3a77343c5dea5f38f64adc0b1d7808c2411105d5cf7571ff7bb7a754c57215
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`（新容器 ID `932c44a14ed0`），最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。Nginx 入口页面／CSS／JavaScript 标记与脚本语法检查通过，页面 288114 bytes、CSS 81225 bytes、JavaScript 100256 bytes。D1 真人浏览器重复成本详情和奖金详情，字段与金额联动正确，console 0 error／warning。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui60-business-cost-bonus.env`。本节证明参考经营成本／奖金 UI/UX 和业务边界提示已经进入 D1，不证明静态记录已绑定真实投影或写命令。

## 参考 OA 生产成本与生产奖金工作面（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check --input-type=commonjs
git diff --check
```

全部通过。本地真人浏览器断言：

- “生产信息 → 生产成本”显示 `¥775,000.00 / 12 条`，方案评审专家费、住宿、交通与用车记录包含日期、金额、说明和发票摘要；方案评审专家费详情准确带入 `2026-09-29`、`¥240,000.00`、说明和 `施工图方案评审专家费.pdf`；
- “生产信息 → 奖金分配”显示 `¥8,520,000.00` 基数、`¥400,000.00` 奖金池、v2 已发布和 6 次／6 人；周宁按 25% 得到 `¥100,000.00`，其余分配合计与奖池一致；
- 经营／生产成本和奖金的重复抽屉使用当前业务面板作用域，不再由全局查询误连到另一组隐藏表单；
- 页面写动作继续只做界面校验并说明 Cost & Outsourcing、Evidence Service、People & Groups、Bonus Distribution、Money Flow 与 IAM 边界；
- 本地浏览器 console 0 error／warning，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.61@sha256:df86eae492c38ac1c6c20d43b0662d4ae843d77939f7534f0f4445b6a3955b0f
AMD64 manifest sha256:ff56ece5c7f63f3151958130688e61eb0386182f417798aee6361118a8390b67
```

公司 D1 通过有效的 `ecs-user` 运维账号只重建 `omsbizfoundry-unified-workbench-1`（新容器完整 ID `4298d5cf20683c59386fe67610aa8696864250c2b87de093614daead8922e8c8`），最终 17/17 healthy；其他 16 个容器 ID 与发布前逐项一致。Nginx 入口页面标记、外置 JavaScript 标记与脚本语法检查通过。D1 真人浏览器实际完成项目列表 → 京哈项目 → 生产信息 → 生产成本 → 成本详情，再进入生产奖金分配；字段与金额正确，console 0 error／warning。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui61-production-cost-bonus.env`。本节证明参考生产成本／奖金 UI/UX 与应用边界提示已经进入 D1，不证明静态记录已绑定真实投影或写命令。

## 参考 OA 生产人员与批复审计工作面（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check --input-type=commonjs
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地与 D1 真人浏览器断言：

- “生产信息 → 人员配置”显示审定人、概算人及路线／桥涵／交通工程三个专业；编辑路线人员时，专业和多选人员按当前记录带入；
- “批复审计”显示批复 `¥8,200,000.00` 与审计 `¥7,980,000.00`，各自具备报告文件、费用拆分与编辑抽屉；
- 批复编辑勾选“引用甲方当前有效合同金额”后总额更新到 `¥8,600,000.00`，并锁定三类上游金额；
- 文件预览／下载／替换／删除以及人员保存／删除都只验证界面和保护提示，没有读取、修改或删除真实事实；
- 本地与 D1 浏览器 console 均为 0 error／warning。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.62@sha256:e4dff3b94004a7d25c56b53be5b8e5bdcef84a06e4e4a5ad34beed85e4bc6d0a
AMD64 manifest sha256:465b795769b3c107fde2853a66473a4238a58661beaa2fd3e052fcba4f152f3b
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `1b8e9cdd28a1d2741347ac962cdc0c7223924d127ad6f3b83db5e8a8dc13cdb4`；最终 17/17 healthy，其他 16 个容器 ID 与发布前逐项一致。Nginx 隧道入口页面标记与外置 JavaScript 语法检查通过。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui62-production-people-audit.env`。本节证明参考生产人员与批复审计 UI/UX 已进入 D1，不证明静态记录已绑定 People & Groups／Approval & Audit Results／Evidence 的真实投影或写命令。

## 参考 OA 生产阶段文件工作面（2026-09-30）

低资源门禁继续只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
node --check <(curl -fsS http://127.0.0.1:19090/assets/workbench.js)
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地与 D1 真人浏览器断言：

- 方案、初设和施工图页面均显示状态摘要、主文件、校审单、评分和阶段编辑抽屉；方案抽屉准确带入标题与评分 `92`；
- 变更竣工页同时显示 2 份变更文件和 1 份竣工文件，变更上传抽屉标题与字段随入口切换；
- 咨询报告页显示主报告 `京哈高速改造工程交通影响评价报告.pdf`、元数据、附件列表及追加入口；预览保护提示能识别具体文件名；
- 表单、文件操作和删除仅验证源级 UI/UX，不读取或修改真实文件；正式操作边界仍为 Production Operations、Evidence Service 与 IAM；
- 本地与 D1 浏览器 console 均为 0 warning／error，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.63@sha256:3bca615051c3728cfc0b6965b78882ffd9e718b7cd4d0e3bac3fb6cfd980e8a2
AMD64 manifest sha256:0bc90595b752003fb95a25d42f65744fa10f27a128336a690e9715ae48b3c744
```

公司 D1 只重建 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `b730db28e09b2b37302b64d05f16a83a33ef63652def57f68c5f657a36cd876e`；最终 17/17 healthy，其他 16 个容器 ID 与发布前逐项一致。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui63-production-stages.env`。D1 真人浏览器停留在“京哈高速改造工程 → 生产信息 → 方案阶段”，console 0 warning／error。本节证明参考阶段文件 UI/UX 已进入 D1，不证明静态样本已绑定真实版本化投影或写命令。

## 参考 OA 完整使用指南与交付入口纠偏（2026-09-30）

低资源门禁只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl --fail --silent http://127.0.0.1:19090/assets/workbench.js | node --check -
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地浏览器确认：

- 使用指南具有 8 个页面结构区、7 项目录、5 张模块卡、6 步完整流程和 26 个 `[data-guide-toggle]` 展开项；
- 点击“如何新建项目？”后 `aria-expanded=true`，答案区域解除 `hidden` 并显示完整 4 步说明；
- 参考 Ant Design collapse 的服务端 CSS 适配不再出现原生灰色按钮边框，箭头随展开旋转；
- 浏览器 console 0 warning／error；验收后本地服务和临时浏览器标签页关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.64@sha256:8fb118d6c53dd045e82fa805d5500b9efaf26f5a1ac75ad4bd9af06deedc8f0f
AMD64 manifest sha256:59de94c953d6f36704e0b38d7b8cc4dfca3a47c47ca91bbbeb257512c7c866cb
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `feb5cc370d95cde285ee2a5a053fa4c0af0838d9fca7f92a4c2a44c5db26d25f`；主栈 17/17 healthy，其他 16 个主栈容器 ID 与发布前逐项一致，`omsbizfoundry-restore20-*` 恢复演练栈未触碰。容器内部 `:8082` 和宿主 Nginx `127.0.0.1:18000/workbench/?preview=oa` 均探测到“项目管理系统结构化操作手册”“如何新建项目？”和“使用指南”。D1 真人浏览器进一步确认 26 个展开项、8 个结构区、展开答案可见以及 console 0 warning／error。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui64-full-guide.env`。本节证明完整参考指南和业务交付入口已进入 D1，不证明预览静态样本已绑定真实版本化投影或命令。

## 参考 OA 收入与合同详情可达性（2026-09-30）

低资源门禁：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地与 D1 真人浏览器断言：

- 项目统计信息 → 公司统计可见“进入公司收入管理”，并可继续进入项目收入页；
- 项目收入页显示“项目财务统计”、四项统计和设计／勘察／咨询 3 行明细；
- 管理费配置弹窗切换“使用公司默认”后显示 `12.00%` 默认提示并隐藏自定义比例字段，切回后表单校验提示可见；
- 项目列表 → 京哈项目 → 经营信息 → 合同信息 → 第一份合同 → 查看完整详情可达，抽屉同步关闭；
- 完整合同详情显示补充协议、合同文件、文件列表／上传文件两个页签；本地与 D1 console 均为 0 warning／error。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.65@sha256:d5c68d79e1c579a78b8f8c1f3ea05334722a329ead1f3e69fe963bafb60badee
AMD64 manifest sha256:62539efe05e99ee32db240fc99530cf36e2633468df95bf257502642027bed8d
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `f0146235c9d5e06a9479732259d2a982ffd8741d4ca1764ee2b9a71463307a10`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，`omsbizfoundry-restore20-*` 恢复演练栈未触碰。宿主 Nginx `127.0.0.1:18000/workbench/?preview=oa` 命中“进入公司收入管理”“项目财务统计”和“查看完整详情”。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui65-revenue-contract-route.env`。用户可见标签页保留在 OA 项目收入页；本节只证明参考 UI/UX 和可达路径，不证明静态演示数据已由真实投影／命令替换。

## 参考 OA 生产对外委托工作面（2026-09-30）

低资源门禁只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check
```

全部通过。本地与 D1 真人浏览器断言：

- 项目列表 → 京哈项目 → 生产信息 → 对外委托可达，页面显示 2 条委托、`¥400,000.00` 总额和 `89.5` 平均评分；
- 第一条编辑抽屉正确回填受托方、联系人、委托内容、`¥240,000.00`、备注、合同文件和两条 `¥120,000.00` 支付；
- 动态支付记录实际完成 `2 → 3 → 2`，计数同步更新；表单校验后显示只读保护说明；
- `京哈高速桥梁检测委托合同.pdf` 可进入 Evidence v1 预览抽屉，删除确认显示正确受托方与金额，点击确认只显示保护提示；
- 新增抽屉为空表单、支付计数 0，无旧合同时“预览现有文件”正确隐藏；
- 本地与 D1 浏览器 console 均为 0 warning／error，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.66@sha256:9b4c10ddc7b3ad754181d5e9123e92a70d1b7025aab4847f7a3c293c761829ce
AMD64 manifest sha256:213fb32c5918a1900b1e7c0f4e4697fbace591c14335f1234dab6e541ea4e8f3
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `5fb1acfbefd983436eeba7c5eeb0b407dc7a849ae7c12cece429c505d873e0e0`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，`omsbizfoundry-restore20-*` 恢复演练栈未触碰。宿主 Nginx `127.0.0.1:18000/workbench/?preview=oa` 命中委托页面、抽屉与脚本标记。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui66-external-commission.env`。用户可见标签页保留在 OA 生产对外委托工作面；本节只证明参考 UI/UX 与保护交互已进入 D1，不证明静态记录已由真实版本化投影／写命令替换。

## 参考 OA 经营参与人工作面（2026-09-30）

低资源门禁只运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
git diff --check
```

全部通过。本地与 D1 真人浏览器断言：

- 项目列表 → 京哈项目 → 经营信息 → 经营参与人可达，经营负责人没有再混入参与人列表；
- 王可、赵清、陈雪三行均显示账号、部门、项目角色和公司角色；
- 添加抽屉初始为空，可选择“周宁 · 生产部 · 技术总工”，提交后显示 People & Groups／Project Workspace／IAM 边界提示；
- 王可删除确认正确显示“经营专员、项目专员”，确认后只显示删除保护说明，不删除演示或真实业务数据；
- 本地与 D1 浏览器 console 均为 0 warning／error，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.67@sha256:15961d97403d6d867bab96e6b0e1edeb9f4dcf3577240f889052fcbb928ef8aa
AMD64 manifest sha256:1b40cafec32d84bdca036183a3bae24f1cfe369f4188b13e39db1c1e73d175ab
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `1f5ea4cba6108b20e73cd26faf023aa46799f9d81e39f8fe0d1b4a2d4513eca6`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。页面入口命中“维护项目经营参与人，并展示其项目角色和公司角色。”和 `data-business-personnel-drawer` 标记。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui67-business-personnel.env`。用户可见标签页保留在 OA 经营参与人工作面；本节只证明参考 UI/UX 与保护交互进入 D1，不证明静态样本已由真实版本化投影／写命令替换。

## 参考 OA 甲方信息内联维护与默认登录面（2026-09-30）

低资源门禁仅运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl -fsS http://127.0.0.1:18090/assets/workbench.js | node --check -
git diff --check
```

本地与 D1 真人浏览器断言：

- 项目列表 → 京哈项目 → 经营信息 → 甲方信息可达，点击编辑后带入华北交通建设有限公司／张工／13800138721；
- 甲方名称有三条历史候选，替换为“北方高速建设集团有限公司／李经理／010-88886666”后表单校验通过；
- 取消恢复既有只读值；边界提示正确区分 Project Workspace 的项目快照、Party Directory 的主体主数据与 IAM 的正式授权；
- D1 未登录入口默认显示参考 OA 的公司全称、水印背景、手机号和验证码按钮；11 位手机号使按钮启用，IAM 备用入口可切换并可返回 OA 默认界面；
- 本地与 D1 浏览器 console 均为 0 warning／error；本地进程和临时页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.68@sha256:55c6dbbe790aecd76139195cf3e56b3e8f55375d9c3dfb45a079ef260b1b98d9
AMD64 manifest sha256:f9c8302131a4928beb560c327354d88c475755e86c715b99f88856ccf3c7459f
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器完整 ID 为 `ac2ec8ad3897231f2cc63d8f2f728d9d3db28a02d2f963349f0e65ecf31628f9`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui68-project-client.env`。本节只证明参考界面、必要交互和边界说明进入 D1，不证明甲方样本已由真实投影／写命令接管，也不把登录页当成完整 OA 业务验收。

## 参考 OA 项目类型驱动生产阶段与勘察试验页（2026-09-30）

低资源门禁仅运行一个本地 Workbench 进程：

```bash
GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...
GOMAXPROCS=2 go vet ./frontend/oms-workbench/...
curl -fsS http://127.0.0.1:19090/assets/workbench.js | node --check -
git diff --check -- frontend/oms-workbench/internal/server
```

全部通过。本地与 D1 真人浏览器断言：

- 项目列表 → 临河新区市政道路勘察设计 → 基本信息显示“设计／勘察”，生产信息显示“试验阶段”；
- 试验阶段无文件点击保存时显示必选保护；选择单个 `README.md` 后显示 `1 待保存` 和文件名，保存只显示 UI 校验通过及正式边界；
- `临河新区岩土勘察试验报告.pdf` 的下载入口显示 Evidence Service 受控访问提示，不触发真实下载；
- 返回京哈高速改造工程后显示“设计／咨询”，试验阶段 `hidden`，咨询报告仍可见；
- 本地与 D1 浏览器 console 均为 0 warning／error，本地进程和临时标签页随后停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.69@sha256:1608a506c95529599e4303eb1e8c4f84e505bbb2d980492348cb92de4fcdb69a
AMD64 manifest sha256:d9f50679ce581b7a6502faa6ba792f4260d5efaccdc345ffd3b3455961754d19
```

公司 D1 只重建主栈 `omsbizfoundry-unified-workbench-1`，新容器 ID 为 `d0deeddd3185`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，17 个 `omsbizfoundry-restore20-*` 恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui69-survey-test.env`。D1 Nginx 入口命中 `data-survey-test-panel`、既有试验报告和 `updateSurveyTestSelection` 标记，用户可见页保留在 OA 勘察试验阶段。本节不证明只读样本已绑定真实版本化投影／命令；临河顶部生产概览仍需下一批切换为本项目上下文，完整 OA 结论继续为 `insufficient-evidence`。

## 参考 OA 生产概览项目上下文一致性（2026-09-30）

沿用上一节的低资源门禁，`go test`、`go vet`、JavaScript `node --check` 与 `git diff --check` 均通过。本地和 D1 真人浏览器分别断言：

- 临河项目生产概览为“生产进度 5/5／施工配合／5/5 阶段 · 16 份文件／陈工”；
- 临河阶段资料依次包含方案 2、初设 3、施工图 6、咨询 1、试验 1、变更 2、竣工 1；“核对试验阶段资料”可直达试验页；
- S207 子项目生产概览为“生产进度 0/5／待启动／0/5 阶段 · 0 份文件／待设置”，五个设计阶段均为未设置，试验阶段隐藏；
- 两端浏览器 console 均为 0 warning／error，本地进程和临时页已停止／关闭。

发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.70@sha256:65b3df424e60089e45fb7c445010ef20aa996741e1bb64fd29c2ce1c88093145
AMD64 manifest sha256:3b1c078441fc4617979e14559c47e99eb85fc8d690e70a6942fd0ddf9dbea79f
```

公司 D1 只重建主栈 Workbench，新容器 ID 为 `645d117f751c`；主栈 17/17 healthy，其他 16 个主栈容器 ID 不变，17 个恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui70-contextual-production.env`。用户可见页保留在“临河新区市政道路勘察设计 → 生产信息 → 试验阶段”。本节证明已知的概览串项目问题消除，不证明演示数据已经替换为真实投影／命令，完整 OA 结论继续为 `insufficient-evidence`。

## 参考 OA 生产快捷入口与按需维护折叠（2026-09-30）

沿用低资源门禁，本地只运行一个 Workbench 进程。实际断言：

- 临河项目生产概览显示方案、初设、施工图、变更竣工、试验、咨询 6 个快捷入口，试验入口可直达试验页；
- S207 只显示方案、初设、施工图和变更竣工 4 个设计链入口；
- “按需维护”默认隐藏，展开和再次收起均成功；浏览器 console 0 warning／error；
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`、`GOMAXPROCS=2 go vet ./frontend/oms-workbench/...`、JavaScript 语法与变更空白检查通过。

中间发布版本：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.71@sha256:5732a1988d9b23242762f9d0bff187ef42f3218315d6593f76c63173d1533d79
AMD64 manifest sha256:b4eb5b6b7b48302f1c1c3eeb0dd3eb5026fb82d5543ee7cde2eace43fbd9a628
```

v71 已在公司 D1 真人点击通过，随后由 v72 取代。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui71-production-navigation.env`。

## 参考 OA 可达组件审计、产品反馈与流程助手（2026-09-30）

参考前端源级 import 审计得到 14 个可达页面、79 个可达组件；补齐 `FeedbackEntry` 与 `ProjectWorkflowAssistant` 后，本地和 D1 真人浏览器分别断言：

- 侧栏“产品反馈”可展开“提交反馈／我的反馈”，未接反馈 SDK 时显示明确边界说明；
- 临河项目经营信息和生产信息均显示“进度与设置”，基本信息不显示；
- 生产助手显示设计／勘察当前进度及 8 项无／不适用设置，切换状态后明确说明正式设置必须经过 Project Workspace 与 IAM；
- 助手折叠／展开成功，浏览器 console 0 warning／error；
- 串行 `go test`、`go vet`、嵌入 JavaScript 语法和 `git diff --check -- frontend/oms-workbench/internal/server` 全部通过。

最终发布并固定：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.72@sha256:69b59db42fe2a678e74942786856cc20d1208cdb92bb20157d3a944a62004ea7
AMD64 manifest sha256:4a9e6e79547c91da9e80658738d8b87e8b622982ec7759dad8aa7617a45466ae
```

公司 D1 仅重建主栈 Workbench，新容器 ID 为 `eda28550134f`；主栈 17/17 healthy，其他 16 个主栈容器 ID 逐项不变，17 个恢复演练容器未触碰。回滚环境为 `/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui72-workflow-feedback.env`。D1 用户可见页保留在“临河新区市政道路勘察设计 → 生产信息”的业务工作台，不是登录页。本节证明参考 UI/UX 与保护性交互覆盖通过，不证明静态演示记录已由真实版本化投影／命令接管。

## 参考 OA 首个真实写入切片：项目立项（2026-09-30）

本轮只运行一个本地 Workbench 进程，没有启动 Docker。代码把项目立项从“前端校验后再跳转”改为真实链路：Workbench 先经 `/gateway/authorize/project-workspace` 获取 IAM 能力 Cookie，再向 Project Workspace 提交 `POST /api/v1/projects` 或 `POST /api/v1/project-packages`；成功后重新读取真实 Project 列表。正式页面不再渲染待立项静态样本，创建与项目包入口分别按 `project-workspace.create` 和 `project-workspace.hierarchy.manage` 收敛。

低资源门禁：

```bash
cd frontend/oms-workbench
GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
GOMAXPROCS=2 GOWORK=off go vet ./...
node -e '<从 server.go 提取 workbenchScript 并交给 new Function>'

cd backend/services/project-workspace && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
cd backend/platform/gateway && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
cd backend/platform/identity-access && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...
```

全部通过。本地 `?preview=oa` 真人浏览器只读检查又确认：项目立项页可达；创建抽屉使用完整视口、内部滚动且不再被父级 transform 限高；项目编号、名称、类型、概况与真实边界提示可见；管理员架构透镜按 Nginx、Workbench、Gateway、IAM、Project 五步展示且普通用户不渲染入口。预览模式仍明确拒绝真实写入。

用户已澄清目标就是现有公司 ECS Auth58。随后已完成 `Nginx → Gateway → IAM → Project → PostgreSQL` 管理员正向创建、工作台列表刷新、Project 页面与自有数据库复读；镜像、容器不变性与测试数据见 [Auth58 首个真实立项切片记录](./reports/oa-real-project-initiation-auth58-20260930.md)。相邻角色拒绝、Grant 撤销、IAM／Project 故障时零写入、恢复重试和备份恢复仍未在本批执行；公网 HTTPS 另行验收。在这些证据完成前完整 OA 状态仍为 `insufficient-evidence`，不得提升为正式产品通过。

## Auth58 `oatest.antuinfo.cn` 公网 HTTPS 入口（2026-10-01）

证书域名、公私钥匹配和 Nginx 配置测试通过；主栈 D1 启动及所有启用应用的 `d1 check` 通过。从 Mac 外部访问：HTTPS Workbench 返回 200 且证书校验通过，HTTP 返回 308，匿名 Project 页返回 303 到 Workbench，错误 Host 返回 421。业务备份校验、配置与回退位置及尚未验收的登录／OSS／完整 OA 项目旅程见 [Auth58 HTTPS 入口记录](./operations/auth58-oatest-https-20261001.md)。

## Auth58 Evidence Local／OSS 兼容（2026-10-01，BF-081）

入口为 [`acceptance/evidence-oss-ecs/verify-transition.py`](../acceptance/evidence-oss-ecs/verify-transition.py)，必须固定镜像摘要、使用新的 `evidence_oss_` 夹具库／卷，并指定原 Local 冷备份。实际 `test.41` 候选通过 10 个原 Local Version 双读、CNAME 单对象 STS 上传／SDK 内网完整性复核／签名下载、Local-primary 回退、混合 v2 冷备份与空目标恢复；共 12 个 Version canonical 行和所有字节一致，主库不变。

Evidence 全量单元、vet、原 Local v1／混合 v2 的真实 PostgreSQL 18 冷备份集成测试通过；File Transfer Core typecheck、16 项测试和契约检查通过。实际 SDK HTTP 回归覆盖文本压缩表示与原长度，页面回归覆盖 V1 签名 Date 与 CNAME 地址。云端逐轮失败记录及保留证据见 [报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md)。该验证使用私网控制 API，不等于正式浏览器；混合 ZIP 外部引用不等于 OSS 字节备份。

补充：单份 4,736 B 合成 OSS 对象的官方 SDK 导出、离机复制、实际误删与单对象 STS 恢复通过；保持 12 个 Version canonical 行与主库不变，不等于全 Bucket 定期备份。D1 云档渲染、候选包 ECS 摘要／配置、新 Linux netns 的 Guard boot／幂等与失败拒绝通过；真实主机／Docker 重启未运行。

实际 `expiry` 等待 900 秒 STS 到期加 10 秒，通过到期前 409、旧凭证 `403 SecurityTokenExpired`、重新签发后的 409 与原字节复读；强制 IMDSv2、无时钟注入，不是 ECS 基础身份的长时自动刷新。`test.44` 包在 ECS 通过内外摘要、云档渲染和 IAM 契约预检；实际 D1 `check` 拒绝配置 hybrid／运行 Local 的错档，主环境未切换。发布包与五项运行时配置／错 Primary 拒绝测试通过。

## Auth58 公网浏览器项目操作（2026-10-01）

实际 Chrome 从正式 HTTPS Workbench 登录、点击创建验收项目、刷新复读、进入真实 Project，并验证缺负责人时拒绝完成。Project `b6e26691-cfd6-4ef4-ba53-8d9f9a8c2cc2` 创建为 `draft / revision 1`，立项尝试记录为 `pending_completion / revision 2`；透镜显示真实创建结果。拒绝后的页面旧 revision 与能力过期反馈缺陷已记录，不能把未变为 active 描述为没有写入。报告与明确未通过的 UX／权限范围见 [浏览器验证记录](./reports/auth58-public-project-browser-validation-20261001.md)。不是所有界面或 OSS 文件链的浏览器通过证据。

后续 Project 页面 `test.43` 的嵌入 JS 回归、普通 Go 测试／vet 通过，并在 Auth58 单组件部署；部署前冷备份恢复到空库，8 张表完整摘要及更新后源库摘要一致，其他 16 个容器 ID 不变，17/17 healthy、Evidence 仍 Local。18:12～18:16 的真实 Chrome 回归又确认拒绝后显示缺失负责人及 revision 3、自然到期关闭编辑、刷新权限恢复历史。只读 `verify-project-browser-feedback.py before/after/frozen` 确认恰好一次缺口命令及一条历史、其他项目不变、恢复不重放；不是撤权或完整立项矩阵。

正式 HTTPS HTML／外部 JS／原 Project 只读复读通过，新页面脚本确实送达，仍为 revision 2、事实不变。伪造管理员／Subject／Tenant Header 与无签名 Bearer 被公网入口、IAM 和目标 API 拒绝，匿名透镜按钮不渲染，重建后 Project 的 IMDS 读取仍拒绝；不是相邻角色／撤权／浏览器矩阵。入口为 `verify-project-feedback-delivery.py`、`verify-project-ingress-negative.py`，只作用于明确 Auth58 目标。

## Auth58 Project 负责人选择与弹窗后检（2026-10-01）

Project 负责人选择与 UI 后续证据：`node backend/services/project-workspace/scripts/verify-assignment-picker.mjs`（13 项）、`verify-initiation-feedback.mjs`（5 项）、`verify-dialog-cancel.mjs`（四弹窗处理器），以及 Project 普通 Go 测试／vet 串行通过。正式 Chrome 选择经营／生产负责人、完成立项为 revision 6，并验证标签展开／收起、横排按钮和空项目取消；`verify-project-picker-browser.py before/after/frozen` 后检确认命令／Assignment 精确增量、来源名册及其他项目不变。固定主镜像当前 `test.48`，46／47／48 三份冷备份的空库摘要、离机副本 SHA-256 和 0600 均通过，见[专项记录](./reports/auth58-public-project-browser-validation-20261001.md)。其他弹窗真人复测因锁屏尚未完成；不代替完整 OA／Files 浏览器验收。

## Auth58 C4 生产资料来源／Project 消费者（2026-10-01）

Production 普通 Go／vet／domain+HTTP race、Project 普通 Go／vet／四包 race、Access Contract、D1 release bundle、15 应用架构静态审计与负例自测实际通过。专项 ECS 串行运行三项来源和两项消费者真 PG18 测试，无 SKIP；测试二进制固定 Linux AMD64，私网／零发布端口，结束停止且保留诊断和数据卷。

来源三项为 `TestMaterialReferencePostgresCurrentVersionsAndZeroWrites`、`TestMaterialReferencePostgresSnapshotSurvivesConcurrentReplacement`、`TestProductionMaterialHTTPPostgresNativeIAM`，使用真实 IAM8 和 Production verifier。消费者两项为 `TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent`、`TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore`，使用真实 Project PG18、显式 upstream HTTP／Credential 测试提供者；验证 schema 7→8、旧业务事实不变、两层资料、drift／空／403／503、零 GET 写入和单库空目标恢复后继续追加。

入口分别见 `acceptance/d1-c4-production-material-source` 与 `acceptance/d1-c4-project-material-consumer`；恢复测试需要 PostgreSQL 18 的 pg_dump／pg_restore，缺工具会明确 SKIP，ECS driver 则拒绝 SKIP。报告保留首次错误夹具路径的失败和第二轮成功，不把编译／SKIP 当作真库通过。主站 13 库／17 容器保持，restore20 不运行；没有主站发布、完整 Nginx／Gateway／IAM／Workload 跨进程、浏览器或 C5/C6 的通过结论。详见[来源报告](./reports/auth58-production-material-source-validation-20261001.md)与[消费者报告](./reports/auth58-project-production-material-consumer-validation-20261001.md)。

## Auth58 C5 Results 资料来源（2026-10-01）

Results 全包 Go／vet、领域与 HTTP race 通过。`acceptance/d1-c5-results-material-source` 的 Linux AMD64 二进制在私有 PG18／真实 IAM8 中串行运行 `TestResultsMaterialPostgresCurrentCorrectionWithdrawalAndZeroWrites` 与 `TestResultsMaterialHTTPPostgresNativeIAM`，二者 PASS、无 SKIP：更正 v2、v1 历史、撤回保留引用、五表只读 canonical，以及单原生 read／Result／Project／Tenant Scope 正反例和无 amount.read／无上游调用。主站不变、结束停止、restore20 不运行。第一轮金额夹具错误的失败保留；第二轮通过。不是 Project 消费、完整 Nginx／Gateway／Workload 旅程或 C8，详见[报告](./reports/auth58-results-material-source-validation-20261001.md)。

## Auth58 C6 来源与 Project C5／C6 消费组件（2026-10-01）

C6 来源通过源码／vet／race 和 ECS 两条无 SKIP 的真实 PG18／IAM8 用例，覆盖 Cost／Engagement 更正／撤回／历史、十表 GET canonical 不变、精确 Scope、匿名／相邻资源／只有金额／交叉聚合 read 拒绝。入口 `acceptance/d1-c6-cost-material-source`；限额 0.55 CPU／416 MiB、无公开端口，结束全部停止，主站 13 库／17 ID 不变。详见[C6 实测](./reports/auth58-cost-material-source-validation-20261001.md)。

消费者入口 `acceptance/d1-c5-c6-project-material-consumer` 的四项真 PG18 用例无 SKIP 通过：历史 7→8 仅执行 000008、新 8→9 保持旧 Production Link／七表事实、C4 current-schema 恢复、三类型 HTTP 测试端口及单库空目标恢复。来源／文件 drift、空漂移、403／503 局部降级、撤回不查询 Evidence、成功命令离线重放和全表 GET canonical 不变，恢复后 ID／revision／Link 保持且可追加 unlink。Project Go／vet／四包 race、Access Contract、15 应用静态审计／自测、D1 release bundle 和三组既有 UI Node 回归通过。限额 0.5 CPU／352 MiB、零公开端口、结束停止，Main 不变；不代表真实 Workload／正式联合恢复／主站发布／浏览器／连续 C8，详见[消费者实测](./reports/auth58-project-results-cost-material-consumer-validation-20261001.md)。

## Auth58 C4～C6 真 Workload 只读链（2026-10-01）

入口 `acceptance/d1-c5-c6-material-workload`，显式 C4 模式 `MATERIAL_INCLUDE_PRODUCTION=yes`。b／c 串行私网运行真实 IAM8、Project、Evidence7，以及正式来源 handler／verifier 的独立 Results／Cost／Production 测试进程：三／四 Link、四／五原生单动作精确资源、相邻 Project 403／匿名 401／错 Project Link 422，来源暂停局部降级／恢复、Evidence 暂停整体 unavailable／恢复、成功命令离线重放和 owned GET 不变均通过。来源过程测试无 SKIP，限额 0.55 CPU／512 MiB、零公开端口；停止后无 OOM，主站 13 库／17 ID 不变、17/17 healthy、restore20 不运行。详见[实测](./reports/auth58-material-workload-validation-20261001.md)。

Source seed 的相邻引用为合成提供者，Evidence 为独立库 SQL 元数据夹具；不代表真实来源写旅程、上传／下载、公网／Gateway／浏览器、联合恢复、Main 升级或连续 C8。首次 a 的脚本失败及诊断保留；普通 Go／vet／编译与真实进程检查分别记账。本机未启动 Docker。

## Auth58 C4～C6 候选镜像与实际启动（2026-10-01）

入口为 `acceptance/d1-c5-c6-material-workload` 的 prepare／publish／image-startup 脚本。Go 1.26.8 本机串行交叉编译、ECS 固定 runtime 的替代组装完成四个 `test.50` 候选；Registry tag 缺席预检、不覆盖、push／digest pull、Linux AMD64／非 root／命令和 12 个文件 SHA 相等通过。逐个实际 server＋独立空库通过 migration、ready、ops check、keyctl、五条匿名 401 及 owned GET 不变；限额 0.4 CPU／352 MiB，结束均停止无 OOM，Main 13 库／17 ID 不变、17/17 healthy、restore20 未启动。

相邻端点为明确不可用测试地址；这不是完整最小组合或 D1 Profile／Workload／正式业务旅程。开发 D1 archive 固定 17 个 digest，包含 IAM8、Gateway／Workbench49 和新契约；不具有 clean-HEAD 或 BuildKit provenance。首次 Docker Entrypoint 字段遗漏的 publication 失败保留，成功是 publication-b。包内配置／IAM parser 另验，联合恢复、Main 切换和浏览器／C8 仍待完成。离机 SHA 和详细范围见[制品报告](./reports/auth58-material-candidate50-validation-20261001.md)。

随后 `verify-bundle-preflight-ecs.py` 实际通过候选 archive 内外 checksum、17 个镜像全选渲染／精确引用、Project Production／Cost 端点注入和 14 个 enabled Manifest 项／Evidence 原生入口。IAM8 在 network-none／read-only、96 MiB／0.15 CPU 条件下解析 12 份 Capability Manifest、3 份 Assignment Policy 和 Workload Policy，exit 0／无 OOM，Main 不变。配置使用明确合成凭据且不连接数据库，不加载 Workload 公钥，不启动 Suite，不能用它关闭正式 D1 运行／升级／联合恢复。

## Auth58 演练栈停止后检（2026-10-01）

用户明确授权后，`stop-restore20.py` 只停止精确 Compose project 的 17 个演练服务，不删除数据。首轮 stop 完成而末尾断言失败，独立 `verify-restore20-stopped.py` 后检确认 17 个 exit 0、28 个卷及配置／bind 来源存在，主站 17/17 healthy 且启动时间早于停止。只读 `--mount-order-diagnostic` 发现两次 inspect 挂载集合相同、7 个列表顺序变化，且 17 项重启策略均为 `unless-stopped`。停止脚本已增加操作前快照与规范挂载比较；`python3 acceptance/evidence-oss-ecs/test-stop-restore20.py` 的 8 项无 Docker 单元测试通过，但修正版未再次操作云端栈，不把单元或 Python 编译检查当作停止重演。完整边界见 [报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md)。

## 小系统边界演变账核对（2026-10-06）

入口为[小系统边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。本轮只读核对三份 Capability Manifest 和 Workload Policy：Project／Production／Contract 分别有 25／13／9 项能力；Project 当前出站 10 个 audience、入站 6 个 workload，Production 为出站 3／入站 1，Contract 为出站 2／入站 5；Project 与 Approval、Billing、Business Engagement、Cost、Production 存在 5 组双向引用。`make architecture-boundary-static-audit` 实际通过 15 应用、0 跨应用 import、0 replace、0 跨库 SQL 机制。该结果只登记为最低隔离守卫，不是运行时边界、第二客户或完整 OA 通过。
