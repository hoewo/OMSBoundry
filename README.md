# OMSBizFoundry

当前 OMS 金额列规则（BF-178）：应收、应付、票据和实际收付款列表中的金额列使用专用 `is-money` 语义，统一最小宽度、右边界、表头排序按钮内边距和 tabular lining figures；币种符号、整数、小数点及两位小数沿同一右边界排列。项目“应收状态／应付状态”不是金额列，不套用该宽度规则。

当前 OMS 主体页规则（BF-177）：Party Directory 继续只保存中性主体档案；Project Workspace 以租户级、追加式 `project-party-candidate` 记录建项前可选的甲方／施工方候选。用户从对应管理页新建主体后，Workbench 先保存 Party，再登记候选角色并刷新当前列表；真实项目关系建立后只补充关联项目数。该记录不是 Party 全局类型，也不是虚构项目关系。BF-177 取代 BF-174／BF-176 对未关联施工方无法表达的临时规则，并已由公网 v1.28 与正式 API 投影验证。[发布报告](./docs/reports/oms-phase1-preproject-party-candidate-publication-20261022.md)

本仓库是 OMS 一期及其底层小系统的可交付源码仓库。为方便客户体验、持续开发和独立部署，源码按“前端工作台、平台后端、业务事实服务”分层；这次整理只改变目录结构，不合并数据库、不改变 API、IAM、业务事实所有权或部署边界。

```text
OMSBoundry/
├── frontend/oms-workbench/       # OMS／OA 统一前端工作台及其 Go 静态服务
├── backend/platform/             # Gateway、Identity & Access
├── backend/services/             # 12 个可独立迁移、测试和部署的业务事实服务
├── contracts/                    # 稳定的版本化 API／能力／投影契约
├── packages/                     # 明确复用的机制包和 wire contract
├── deployments/                  # D1、演示、本地 OA 和阿里云部署装配
├── acceptance/                   # 跨服务与交付验收
├── docs/                         # 决策、规格、运行说明与验证报告
└── arckit/                       # 业务、交互和技术设计材料
```

完整目录职责、开发入口和验证命令见[仓库结构说明](./docs/repository-layout.md)。新开发优先从 `frontend/` 或相应 `backend/services/` 进入；不要把业务事实写进 Workbench、Gateway 或共享包。

2026-10-22 BF-175：旧仿 OA 与旧 OMS 已从活动产品边界移除并发布到 `oatest`。旧入口统一转回当前 OMS，旧界面静态资源在 Gateway 返回 `410 Gone`；当前 OMS 登录、经营工作面和 Evidence 附件能力分别使用自身的版本化资源，不再运行旧 OA React／立项运行时。历史源码仍保留作追溯，IAM、Gateway、Evidence 和业务 Owner 均继续运行，未删除任何业务数据或后端能力。公网 Gateway／Workbench 为 `v0.1.0-test.oms-v1.26`，D1 全栈与 17/17 容器健康通过。[发布报告](./docs/reports/oms-phase1-retired-surfaces-and-evidence-decoupling-publication-20261022.md)

2026-10-22 BF-174：已把 OMS 刷新关键路径上的附件运行时移出首屏。之前即使 BF-173 已减少授权轮次，页面仍会同步下载 2,512,394 bytes、实测约 5.15 秒的附件脚本；现在首屏只同步加载 OMS CSS、分页和主脚本（合计 234,029 bytes），附件／预览首次使用时再加载，并为版本化资源设置 immutable 缓存。甲方与施工方列表也改为按真实项目关系投影：当前公网数据验证为甲方 `甲方1`、施工方空集合，不再把三个甲方误显示为施工方。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.25@sha256:d9df134e7bcadf70418058bba5d84f4646ae96ae28a509bbb9230883aba7e3f5`。[发布报告](./docs/reports/oms-phase1-critical-path-and-party-role-publication-20261022.md)

2026-10-22 BF-173：已优化 OMS 一期刷新体验。有效的既有 Capability Cookie 会直接用于 Owner 只读请求，只有服务明确要求凭证时才按 Audience 重新授权并重放一次；并发的同一 Owner 授权会合并。项目与合作主体先显示，票据、资金、合同和经营汇总随后补齐，重复刷新也只执行一轮。写操作仍先取得新授权，未增加浏览器业务数据缓存。公网只替换 Workbench 为 `v0.1.0-test.oms-v1.24`，D1 全栈、17/17 健康和正式登录／五 Owner 只读复验通过。[发布报告](./docs/reports/oms-phase1-progressive-refresh-publication-20261022.md)

2026-10-22 BF-172：已修复项目详情打开时点击侧栏只切换底层页面、画面仍停留详情的问题。侧栏导航现在会统一退出项目详情、恢复滚动、清除旧项目选择并作废未完成的详情读取；公网只替换 Workbench 为 `v0.1.0-test.oms-v1.23`，D1 全栈、17/17 健康和正式登录／五 Owner 只读复验通过。[发布报告](./docs/reports/oms-phase1-project-detail-navigation-publication-20261022.md)

2026-10-22 BF-171：已修复 BF-170 清理后“合同功能尚未准备完成”。根因是把 Contract／Money 的 Party Directory runtime cutover 误当业务数据删除；服务和 schema 健康并不代表这项写入前提仍存在。两套 Owner 已通过正式 reviewed mapping／cutover 恢复 directory authority，重复执行幂等；公网验收新增强制运行态断言并通过，未创建项目、合同或资金事实。[故障报告](./docs/reports/oms-phase1-party-runtime-reset-repair-20261022.md)

2026-10-22 BF-170：用户明确当前仍是 Demo，可清除全部旧演示与 OA 业务数据后重新走录入流程。`oatest` 已在完整备份 `20261022T023910Z` 后归零十二个业务 Owner 和两个本地附件对象卷；IAM／Grant、schema、部署与不作为外部合作方显示的本方经营主体保留。逐库实数、附件 0、D1 全栈、17/17 健康及公网正式登录／五 Owner 只读复验均通过。[执行报告](./docs/reports/oms-phase1-demo-full-business-reset-20261022.md)

2026-10-22 BF-169：用户进一步明确“可同时展开”不等于“可同时选中”。Workbench 已改为跨全部业务组统一计算二级导航状态，多个组保持展开时只有当前页面对应的一项高亮，非当前项同时移除 `aria-current`。源码 `5779637` 已发布为 Workbench `v0.1.0-test.oms-v1.22@sha256:d9b11412c72a6d4b9d9a9aa8c8786855e4e8f5219503edb8514554c0966a4f42`，静态资源为 `20261022.2`；备份、D1 检查、17/17 容器健康和公网只读验收通过。[验证报告](./docs/reports/oms-phase1-single-current-navigation-publication-20261022.md)

2026-10-22 BF-168：用户确认 BF-167 工作台候选可作为 OMS 一期后续改进基线。侧栏业务组可同时展开，当前页面与展开状态分离；分页条数选择器固定完整宽度，文字和箭头都保持在控件边框内。`main@32d9597` 已发布为 Workbench `v0.1.0-test.oms-v1.21@sha256:6dcba5542c68b0c7970c80cee4282fc3560ece7efab197a87ac0e9088c65e423`，静态资源为 `20261022.1`；升级前备份、候选预检、D1 检查、17/17 容器健康和公网正式登录／五 Owner 只读复验通过，仅替换 Unified Workbench。[验证报告](./docs/reports/oms-phase1-mvp-review-ui-publication-20261022.md)

2026-10-21 BF-167：OMS 一期已在独立分支 `codex/oms1-mvp-review-ui` 完成本地界面候选。实现参考 `/mvp-review` 的导航、层级和操作方式，将筛选改为列表内就地展开，导出保留居中确认并继承当前搜索／筛选／排序，补齐项目与甲乙方主体导出、项目摘要、统一排序分页和不遮挡主菜单的项目详情。该候选只改 Workbench 表现层，未发布到 `oatest`，也没有改变 Owner、API、schema、Grant、精确金额或业务数据。

2026-10-21 BF-166：OMS 项目列表已改为只显示“应收状态／应付状态”，正常为“已完成／未完成”，金额异常仍单独警示；具体应收、未收、应付、未付金额保留在项目详情和专业模块。`main@acce030` 已发布为 Workbench `v0.1.0-test.oms-v1.20@sha256:f78176522965d41a6858ddf6bda7ea9f97332b0dca2b2fece9fe502760f0efc5`，备份、D1 检查、17/17 容器健康及公网正式登录／五 Owner 只读复验通过；未改变业务事实、Owner、API、schema 或 Grant。[验证报告](./docs/reports/oms-phase1-project-settlement-status-publication-20261021.md)

2026-10-21 BF-165：昨天确认的 OMS 易用性基线已经从 `main@99ead97` 发布到 [oatest](https://oatest.antuinfo.cn/)。公网 Workbench 为 `v0.1.0-test.oms-v1.19@sha256:6268306eadb7183ddae1da1229e623f5fab3d6fb0c033244b289d24a4629155c`，静态资源为 `20261020.5`；升级前备份、D1 全栈检查、17/17 容器健康、正式登录及五 Owner 只读复验通过。本次只替换 Unified Workbench，不包含独立交互试验分支，也不改变业务事实、Owner、API、schema 或 Grant。[验证报告](./docs/reports/oms-phase1-usability-baseline-publication-20261021.md)

2026-10-19 BF-164：OMS 产品不以“资质单位”为前提。现实客户的行业角色与系统稳定模型分离，当前使用系统的公司统一称“本方经营主体”，新配置使用 `OMS_OPERATING_PARTY_ID`，旧名称只保留部署兼容。甲方／施工方列表只展示 Party Directory 实际返回的持久化数据，不注入演示、默认或兜底主体；没有人工录入数据时显示空状态。

2026-10-19 BF-163：本仓库已形成客户体验与后续协作所需的前后端分层交付基线。目录整理后的全部活动构建、验收和部署路径已同步更新；`contracts/` 与 `packages/` 有意保留在根目录，以维持稳定契约和 Go module 兼容。该整理没有改变任何 Owner、API、数据库、Grant、部署拓扑或公网环境。[仓库结构说明](./docs/repository-layout.md)

2026-10-19 BF-162：OMS 一期已修复新增开票返回 `INVALID_REQUEST` 的业务关系漏洞。根因是项目把当前资质单位自身选为外部甲方，Billing 正确拒绝了“自己给自己开票”。Workbench 现已在新建项目、关联施工方、双向票据和收付款的候选与提交两层排除资质单位自身，并将 Owner 错误转为可操作的中文提示。既有错误项目不静默篡改，需以真实甲方重新建立。公网已发布 Workbench `v0.1.0-test.oms-v1.18@sha256:2b2e319dfacbc6b84b743f5e3a9dcf09846d37c99f7de8523eba8990adf3f54f`，备份 `20261019T130505Z`、D1 检查、17/17 容器健康和公网只读验证通过。[验证报告](./docs/reports/oms-phase1-self-counterparty-guard-validation-20261019.md)

2026-10-18 BF-161：已按“只清除 OMS1 数据”完成测试环境定向重置。清理前完整备份为 `20261018T041742Z`，六个 OMS 事实 Owner 恢复到首笔 OMS 数据之前的 `20261012T175912Z` 检查点；账号、权限、资质单位和旧 OA 数据均保留。OMS 页面同步增加业务范围过滤，只显示 `operations` 项目及其票款记录，避免旧 OA 事实混入空白 OMS。公网已发布 Workbench `v0.1.0-test.oms-v1.17@sha256:da5f4626cb53a9d8ca6758f2c6852468974221fc4ca440264de4a62a42d5dbfe`；清理后 OMS 项目为 0，D1 检查、17/17 容器健康和公网只读验证通过。[验证报告](./docs/reports/oms-phase1-data-reset-and-scope-validation-20261018.md)

2026-10-17 BF-160：OMS 一期已修复“页面打开一段时间后项目列表误显示 0 条”。业务数据没有丢失；根因是浏览器把 Gateway 的短期访问凭证重定向表示为 `opaqueredirect/status=0`，旧脚本没有进入重授权分支，反而把各 Owner 读取失败投影成空列表。Workbench 现在会清除对应页面授权缓存、重新换取该 Audience 的 Capability，并只重读原请求一次；普通授权拒绝和业务错误不自动重试。公网已发布 Workbench `v0.1.0-test.oms-v1.16@sha256:9a082f08dba5897e5cdba3698e855a6f9e842d772484a63485c68c7f64c567e5`，备份 `20261017T051948Z`，17 个长运行容器 healthy；正式只读验收复读 Project 30、Party 13、Billing 3、Money 5、Contract workspace 6。[验证报告](./docs/reports/oms-phase1-capability-renewal-repair-20261017.md)

2026-10-17 BF-159：OMS 一期已提升为 `https://oatest.antuinfo.cn/` 登录后的默认产品工作面。Gateway 根入口、登录默认回跳与 `/oms-v1/` 均指向服务端直接渲染的 OMS；当前页面不再先输出旧 OA 侧栏，也不启动旧 `workbench.js` 和 OA 全量 runtime。历史项目立项、项目管理、资料、统计和 OMS 参考版统一收进 [`/legacy/`](https://oatest.antuinfo.cn/legacy/)“旧版工作台”，并保留显式返回 OMS 的入口。公网已发布 Gateway／Workbench `v0.1.0-test.oms-v1.14`，备份 `20261017T033416Z`、D1 检查和 17 个长运行容器健康通过；Chrome 从根域名直接进入 OMS，复读 29 个项目并可往返旧版工作台。[验证报告](./docs/reports/oms-phase1-primary-entry-validation-20261017.md)

2026-10-17 BF-158：OMS 一期已关闭长期开页后的 CSRF 令牌失配。业务写入在向 Gateway 换取应用 Capability Credential 前，统一从当前 Cookie／配置的 IAM 路由取得并核对新鲜 CSRF；不得再复用首次渲染 HTML 中的退出登录表单令牌。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.13@sha256:3215e21d963f4f978f036ecad2e1f36d437d6680035ca0cde8097b0b9eb82b80`，发布前备份为 `20261017T030537Z`，D1 检查和 17 个长运行容器健康通过。Chrome 真实确认失败操作未留下“微信4123”半成品，随后以同名、同甲方和 `¥1,243,123.00` 成功完成 Project → Contract → Project Link，并在项目详情复读合同应收与 r2 关联记录；本轮未改变 Owner、API、schema、Grant 或业务事实。[验证报告](./docs/reports/oms-phase1-csrf-refresh-project-create-validation-20261017.md)

2026-10-14 BF-157：OMS 一期已修复长期开页后新增失败和主体保存后不可见问题。所有变更请求在发送前重新换取短期应用凭证，读取只对 Gateway 明确的入口凭证失效有界恢复；甲方／施工方管理显示全部可选 Party 并标注项目使用情况。项目详情的开票、收票、回款和支付补齐笔数、最近业务日期及保存后的累计反馈，静态资源版本同步递增，普通刷新不会继续命中旧脚本。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.12@sha256:a9759a92300718a0851c20a619406992a16e5fdfa198315fcb7de9e12c2f0f31`，D1 检查通过；浏览器已真实新增施工方并复核既有甲方、开票和回款投影。[验证报告](./docs/reports/oms-phase1-create-and-summary-feedback-validation-20261014.md)

2026-10-14 BF-156：OMS 一期已统一简单业务详情和选择控件。应收／应付、回款／支付、票据状态及合同详情按“状态 → 核心金额 → 主体／日期／来源 → 备注／附件”渐进展示，纯查看抽屉只保留关闭动作；全部原生 `select` 的可见交互由同一套白色组合框和视口级弹层承载，支持准确锚定、边缘避让、上下翻转、键盘、禁用与动态选项。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.10`，D1 检查通过且 17/17 容器 healthy；没有改变 Owner、API、数据库、schema、Grant 或业务事实。[验证报告](./docs/reports/oms-phase1-unified-detail-select-validation-20261014.md)

2026-10-13 BF-155：OMS 一期已建立可读性与工作区密度基线：桌面侧栏由 216px 收到 196px，主内容改为占满可用工作区，项目详情上限由 1080px 放宽到 1720px；列表正文、表头、金额链、详情字段和抽屉表单同步提高字号，同时保持 50px 顶栏和紧凑筛选区。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.9`，D1 检查通过且 17/17 容器 healthy；没有改变 Owner、API、数据库、schema、Grant 或业务事实。[验证报告](./docs/reports/oms-phase1-readable-density-validation-20261013.md)

2026-10-13 BF-154：OMS 一期项目详情已按业务归属重新分板块：顶部只保留返回，编辑项目进入“项目资料／基本信息”，合同详情进入“甲方业务／合同与回款”，关联施工方和设置应付进入“施工方业务／合同与支付”；项目说明并入基本信息，金额变更与项目记录保持独立。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.8`，17/17 容器 healthy；没有新增命令、Owner、数据库或 Grant。[验证报告](./docs/reports/oms-phase1-project-detail-layout-validation-20261013.md)

2026-10-13 BF-153：OMS 一期的合同、双向票据、回款和支付凭证已统一接入 Evidence 安全预览。PDF 与常见图片在同一轻量预览层中打开，图片支持缩放；不满足既有安全策略的格式或大小明确降级为下载，不伪造 Office 在线预览。预览关闭后返回原详情，临时 URL 随即释放。公网 Workbench 已更新到 `v0.1.0-test.oms-v1.7`，17/17 容器 healthy；没有新增文件库、转换服务、数据库或 Grant。[验证报告](./docs/reports/oms-phase1-file-preview-validation-20261013.md)

2026-10-13 BF-152：OMS 一期项目详情已按产品设计完整落地并发布到 [oatest `/oms-v1/`](https://oatest.antuinfo.cn/oms-v1/)：基础身份、甲方侧“应收→开票→回款→未回款”、施工方侧“应付→收票→支付→未支付”、逐施工方折叠账、合同金额变更、项目历史和项目说明均由既有 Owner 实时投影；具体票据／资金流水及新增动作仍留在专业模块，避免把项目页重新做成杂乱台账。此次只替换 Unified Workbench，不新增 OMS 数据库、schema、Grant 或事实 Owner。[验证报告](./docs/reports/oms-phase1-project-detail-validation-20261013.md)

2026-10-13 BF-151：OMS 一期项目内的“合同附件”已升级为 Contract Owner 驱动的“合同详情”：默认显示合同金额、名称、状态、往来主体和日期，附件保持在下层；草稿合同可追加编辑版本，金额低于可唯一归属的已回款／已支付时输入红色提醒并在保存前二次确认。Project／Workbench 不复制合同或资金事实，多合同且资金无法可靠归属时不做自动判断。

2026-10-13 BF-150：OMS 一期需要的六个业务事实 Owner 已在 [oatest `/oms-v1/`](https://oatest.antuinfo.cn/oms-v1/) 完成真实接入：Party、Project、Contract、Billing、Money、Evidence 经正式 Nginx→Gateway→IAM→Workbench 链路完成同一幂等项目的上下游合同、双向发票、回款／支付、六份附件、完整分页筛选和导出验证；17/17 容器 healthy。People、Business Engagement、Production、Approval、Cost、Bonus、Design 虽可随完整 Suite 部署，但不是 OMS 一期运行时依赖，不为“全部接上”的表象强行串入。证据见[公网写入报告](./docs/reports/oms-phase1-public-write-validation-20261013.md)。

2026-10-12 BF-149（历史发布快照，已由 BF-150 取代）：OMS 一期已发布到 [oatest `/oms-v1/`](https://oatest.antuinfo.cn/oms-v1/)。正式入口保持 `Browser → Nginx → Gateway → IAM → Workbench`，17 个长运行容器和 D1 全栈检查通过；当时只完成五 Owner 读取，尚未完成的资金、附件和施工方应付随后已由 BF-150 关闭。

2026-10-12 BF-147：OMS 一期新建项目表单的甲方字段已补“创建甲方”入口，点击后退出项目表单并进入“项目经营 → 甲方管理”；甲方选择器统一为浅色、固定箭头样式。该路径不隐式创建 Party、不按名称自动匹配，也不产生半套项目。

2026-10-12 BF-146：OMS 一期的九个二级工作面已从内容区 Tab 收入对应侧栏一级菜单。桌面只展开当前工作区的三个二级入口，避免一次铺开全部层级；移动端保留四项底部一级导航，并在顶栏下方提供当前工作区的紧凑二级导航。该调整只改变 Workbench 信息架构，不新增业务 Owner、数据库、命令或授权。

2026-10-12 BF-145：OMS 一期 MVP 产品访谈已收口为正式 v1 基线并进入实现。公网入口固定采用 `Browser → Nginx → Gateway → IAM → Unified Workbench OMS`；新增 [oatest `/oms-v1/`](https://oatest.antuinfo.cn/oms-v1/) 作为一期验收路由，旧 `/oms/` 参考 Demo 暂留用于对照。四入口、Owner 读取、Party 建档、Project 施工方关系和 Billing 双方向登记已完成本地接线；Billing 以 `OMS_QUALIFICATION_PARTY_ID` 显式声明我方稳定 Party 身份，不配置或无原生能力即失败关闭。Workbench 只做编排与投影，项目、主体、合同、发票、资金和文件继续由既有小系统拥有；Money 主体 runtime cutover 和无固定期限义务仍是显著边界缺口，公网尚未发布。产品范围见[一期产品设计](./docs/specs/assembly/oms-phase1-mvp-product-design-v1.md)。

2026-10-11 BF-144：OMS 一期 MVP 进一步收口为“只登记实际发生并展示余额／超额异常”，不要求填写期限，不计算逾期，也不实现催收或自动提醒。Money Flow 已有固定 `due_on` 下的逾期／账龄只读投影，但没有催收流程和通知；无固定期限、期限依据、跟进事实与提醒能力作为版本化兼容待办保留，不能为接入现有接口伪造到期日。第一版接受必须登录；账号采用预置本地管理员还是后台／既有 IAM 注册留待会议确认，不提供免登录旁路。

2026-10-11 BF-143：OMS 一期把“完工后由资质单位向上游开票、工程款按不确定节奏分批回收直至结清”作为当前高优先级业务事实／产品主线。Money Flow 继续保留既有义务、实际资金、Allocation 和余额模型，以版本化兼容扩展期限依据、分批回款投影与催收跟进，不用新需求否定或替换已有闭环；下一轮原型先把单事项回款循环和跨事项催收工作面做清楚。范围见[第 18 轮讨论](./docs/discussions/round-18-oms-phase1-industry-research-scope.md)和[开发计划](./docs/development-plan.md)。

2026-10-08 BF-122：已按“资质单位是 OA 业务链中的当前主体”做出第一版只读 OMS 参考 Demo，并通过 Gateway 在 [oatest `/oms/`](https://oatest.antuinfo.cn/oms/) 提供入口。五个工作区只验证信息结构和交互，不读取或写入真实业务，不新建业务所有者、数据库、IAM Grant 或正式 OMS 契约；OA 完成度仍按 BF-120 保持 `insufficient-evidence`。范围与证据见[规格](./docs/specs/assembly/oms-qualification-unit-demo-v1.md)和[验证记录](./docs/reports/oms-qualification-unit-demo-validation-20261008.md)。

2026-10-07 BF-121：按用户最新优先级，本地补齐统计台账／公司两链矩阵／成本／人员奖金交互，以及原 OA 属性对齐的四层权限配置展示。配置保存为浏览器会话草稿，不冒充 IAM 生效；项目详情延后，未部署或修改生产 Grant。实际检查和后续真实环境缺口见[本轮记录](./docs/reports/oa-statistics-permission-ui-validation-20261007.md)及[规格](./docs/specs/assembly/oa-statistics-permission-ui-v1.md)。BF-120 的当前完成度基线继续有效。

2026-10-06 BF-120 当前基线纠偏：用户确认以仓库根目录 `AGENTS.md` 为当前实现完成度基线。其后形成但未进入干净可追溯版本的代码、报告和运行记录保留为中断实验／候选证据，不再直接支持“已完成”结论；其中与当前边界一致的架构规则继续有效。当前已实现应用范围和 Bonus Distribution 的缺口以 `AGENTS.md` 为准，下一实际工作是基线收敛与 C1～C8 缺口矩阵，再依次补 C7、C8、组合／Overlay、真实环境和最终报告。见[正式决策](./docs/decisions.md)、[当前基线与下一步](./docs/status/current-baseline-and-next-actions-20261006.md)。下方 BF-112～BF-119 及其他后续运行记录按各自当时范围保留，不表示当前完成状态。

2026-10-06 BF-119：本项目把“边界稳定”定义为真实验证中的变化逐步变少、变小、局部化和可预测，而不是永久冻结。当前完整 OA 完成度低，较多边界调整属探索期正常现象，但每次都必须显式版本化并验证兼容、迁移和恢复；后续切片应看到涉及 Owner、公开契约、migration、强制依赖和旧场景回归面总体下降。Project、Production、Contract 整体仍是候选边界，详细标准见[正式决策](./docs/decisions.md)和[小系统边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-06 BF-118：原 OA 对外委托已按 Cost／Party／Contract／Evidence／Money 五 Owner 接通，联系人作为 Cost 发生时快照而非 Party 主档；Project 只在创建边界精确读且零写，Production 零依赖，Contract 仅显式选择时精确读且零写。私有真实入口 10／10、未知回执同 command 重放、真文件、评分／纠正、Cost→Money 独立接受、撤回历史、缓存请求上界、八库空目标恢复、正式 IAM 撤权和精确 Scope 通过。Money 拒绝／实付、全角色／跨 Tenant、正式 D1、Main／公网、第二客户和完整 OA 未关闭。见[规格](./docs/specs/assembly/oa-external-commission-native-v1.md)、[报告](./docs/reports/oa-external-commission-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一切片进入资料管理／统计。

2026-10-06 BF-117：原 OA 批复／审计双列已接 Approval & Audit Results 原生闭环；批复冻结精确 Contract revision，审计冻结精确 approval revision，多报告文件归 Evidence，纠正／撤回／重登记保留不可变历史。私有真实入口 11／11、回执丢失同 command 重放、返回缓存 0 读／显式刷新 1 读、正式 IAM 撤权、六所有者逻辑空目标恢复和 4 份 Evidence 字节保持通过。补测发现并修复 Results 空 Evidence 被编码为 `null` 的输出契约缺陷。Project／Contract／People 摘要不变，Production 调用为 0；Project Completion Checkpoint、Main／公网、正式 D1、全角色、第二客户和完整 OA 未关闭。见[规格](./docs/specs/assembly/oa-approval-audit-native-v1.md)、[报告](./docs/reports/oa-approval-audit-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一切片进入对外委托。

2026-10-06 BF-116：原 OA 四类型生产总览已由 Production 自有 Work Item、冻结类型快照、Stage 和有效资料引用形成单一版本化只读投影；Workbench 只复用原界面、导航和 30 秒页面缓存，不保存或重算进度。私有真实入口 10／10、正式 IAM 撤权、真实 PG18、五所有者逻辑空目标恢复和 4 份 Evidence 字节保持通过；首次总览 1 次 Production 请求，返回页缓存 0 次，显式刷新才重读，Project／Evidence／Contract 普通读取均为 0，Production 零写。Production 的公开读边界扩大但写边界不变；Project 零 schema／命令／读链变化，Contract 零依赖。Main／公网、正式 D1、全角色、第二客户和完整 OA 未关闭。见[规格](./docs/specs/assembly/oa-production-progress-v1.md)、[报告](./docs/reports/oa-production-progress-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一切片进入 Results 批复／审计。

2026-10-06 BF-115：原勘察试验与咨询／勘察／测绘报告组件已通过私有真实 Nginx→Gateway→正式 IAM→Production／Evidence／PG18 的 9 场景及五所有者逻辑空目标恢复。Production 以不可变 v6 在创建或显式 5→6 时精确验证 Project ID、Tenant、revision 与 active 状态，仅冻结 `consultation`／`design`／`mapping`／`survey` 适用性快照；正常读写不再同步调用 Project。试验只适用于勘察，报告适用于咨询／勘察／测绘；两者都不引入 Contract。由此证明“可变主数据在 Project、发生时最小快照在 Production”的依赖时机，但只是一轮单客户证据，不代表三系统边界或跨客户演进已稳定。见[规格](./docs/specs/assembly/oa-production-test-report-v1.md)、[报告](./docs/reports/oa-test-report-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一切片不再增加文件页签，转为核对四类型生产总览／进度如何从 Production 自有事实投影，避免 Workbench 再造第二份状态。

2026-10-06 BF-114：原变更／竣工双列表及两套上传抽屉已通过私有真实链 11 场景和五所有者逻辑空目标恢复；不可变 v5 以可选 change＋必需 completion 承接无校审／评分、不同扩展名和不可变替换。验收实际抓到 migration 数据库约束与代码阶段注册表没有随定义闭合两项缺陷，修复后才从全新夹具通过。Production 阶段资料子边界获得首份异类结构复用证据，整体仍为候选；Project 零写入／零契约变化，Contract 未引入，Main／公网／完整 D1 未动。见[规格](./docs/specs/assembly/oa-production-change-completion-v1.md)、[报告](./docs/reports/oa-change-completion-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一项按原 OA 核对勘察试验与多类型报告，不能把 v5 外推成通用工作流。

2026-10-06 BF-113：原施工图组件已通过私有真实链 10 场景和五所有者逻辑空目标恢复；既有 v3 初设事实保持，同一 Work Item 首次施工图保存才显式 3→4。首次验收抓到“有 Tab／宿主但 runtime 未派发”的装配缺口，修复后从新夹具完整重跑才计通过。方案、初设、施工图第三个同类场景使“阶段资料集合”子边界达到可重复，Production 整体仍为候选；无新增 Audience、Capability、Workload Policy 或事实所有者，Main／公网／完整 D1 未动。见[规格](./docs/specs/assembly/oa-production-construction-stage-v1.md)、[报告](./docs/reports/oa-construction-stage-native-validation-20261006.md)和[边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)。下一项转向结构不同的“变更／竣工”，重新验证边界而非复制现有表单。

2026-10-06 BF-112：原初步设计组件已通过私有真实 Nginx→Gateway→正式 IAM→Production／Evidence／PG18 的 9 场景及五所有者逻辑空目标恢复。既有 `reference-oa-stage-set-v1@2` 保持仅方案，同一 Work Item 第一次真实初设保存才显式幂等升级 v3；没有新增跨系统依赖或事实所有者。阶段资料子边界获得首份同类可重复证据，但整个 Production 仍为候选，Main／公网／完整 D1 和完整 OA 未通过。边界演变见[持续账本](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)，运行范围见[报告](./docs/reports/oa-preliminary-stage-native-validation-20261006.md)；下一项按原 OA 实际差异施工“施工图设计”。

2026-10-04 最新方案切片：正常工作台直接复用原方案面板／抽屉，私有真实Nginx→Gateway→正式IAM→Production／Evidence／PG18完成16场景，含整批多文件、精确评分、真实预览下载、移出留史和失败续办；五所有者逻辑空目标恢复后原UI继续、377项Node等分层通过。[实际报告](./docs/reports/oa-production-stage-native-validation-20261004.md)不是Main发布或完整OA结论。旧活动任务显式过渡、多文件资料来源／消费兼容、阶段真实D1／keys恢复、精确授权及匹配公网发布仍是门槛；初设／施工图／试验／报告等按原差异和明确新版本继续，不改已注册v2。本机Docker维持停止，测试串行收尾；完整范围见[全站清单](./docs/specs/assembly/oa-complete-operation-delivery-ledger.md)。以下较早记录保留历史。

2026-10-04 最新续轮：原专业人员配置→奖金登记／刷新／撤回已接入正常 Workbench 候选，私有 Nginx→Gateway→正式 IAM→独立所有者完成 20 个场景；347 项 Node、Project／Bonus 全量 PG18、Bonus race、TS／Go／vet通过。随后[所选 D1 六库联合空目标恢复及原 UI 继续的 14 场景](./docs/reports/oa-professional-selected-d1-restore-20261004.md)实际通过，运行容器已停止，私有卷／备份保留。显式 D01＋Bonus v3 类型化引用不改旧 Base／v2 hash。没有发布 Main或增加 Grant，完整 OA／公网／全 Suite 恢复未验收；[实际结果和剩余门槛](./docs/reports/oa-professional-bonus-continuous-validation-20261004.md)与[全站清单](./docs/specs/assembly/oa-complete-operation-delivery-ledger.md)为当前状态。下一功能按[原阶段逐动作对照](./docs/specs/assembly/oa-production-stage-original-operation-map.md)推进。下方早期候选段落保留历史，不代表当前已发布的 Contract22／Money23 等仍是旧 schema。

2026-10-04 施工：合同原双列／抽屉、显式原始合同与三类协议确认、三类附件及阶段／最终结算已候选接线，未部署。schema 22 的私有 PG18、应用库空目标恢复及独立启动已通过；原组件 22 场景通过，使用测试端口，不能代替正式 IAM／Evidence 联合与公网验收。最终结算更正、全角色、兼容发布仍待完成；Main 保持 Workbench `.20261003.6` 和 Contract schema 18。继续按[完整 OA 清单](./docs/specs/assembly/oa-complete-operation-delivery-ledger.md)推进，不以招投标或合同候选代替全站；[分层证据](./docs/reports/oa-contract-draft-ui-candidate-20261004.md)。以下旧状态按各自日期保留。

2026-10-03 当前部署：[Auth58 工作台](https://oatest.antuinfo.cn/workbench/) 的 Workbench 为 `.20261003.6`，Project 为 `.3`／schema 11、IAM catalog 0.7／旧 Assignment Policy 1.2。原经营参与人组件已接真实添加、历史结束、重入和原命令恢复；最终公网三项零写续验通过，快速切页／刷新／重载无 429，隐藏投影零抢发，可见统计仍发起原生授权读取。200 项 Node、Go／vet、四批联合备份／13 库空目标恢复及离机摘要、17 healthy／D1 check 通过；旧业务事实及用户101保持。Contract 原生端口真库及独立启动通过，但未部署、原合同表单未接通；完整 OA 仍未完成，见[实测与完整剩余范围](./docs/reports/oa-complete-operation-progress-20261003.md)。当前运维入口 `oa-projection-visibility-20261003a/d1-active`，旧 wrapper 不再用于 up。

历史 `.20261003.2` 登录修复：旧 CSRF 令牌故障已修复，公网正常／过期模拟／多标签及完整脚本 Project Directory 冒烟通过，账号密码和 Grant 不改；[登录实测](./docs/reports/oa-login-csrf-repair-validation-20261003.md)保持其当时范围。

2026-10-03 UI 执行要求与目标（BF-093～BF-096）：直接复用原 OA 组件／样式／弹窗，只做必要的数据、权限和操作适配，优先完成完整界面与实际操作链。原招投标组件和 BE 多附件已在 `.20261003.1` 部署，当前 local 文件上传、预览／下载、单份移除、同项目投影及丢回复后原命令继续通过；见[组件与文件链实测](./docs/reports/oa-reused-bidding-ui-validation-20261003.md)。小系统可以兼容演进，但职责、数据所有权、版本化协作、IAM／Evidence 和历史边界必须稳定或可预见；局部证据不等于完整 OA，OSS 分离仍未完成。

OMSBizFoundry 是用于研究、验证和打磨窄域业务小系统的工程试验场。

它不以建设通用业务平台为目标，而是希望形成一组可以独立安装、独立运行、快速交付、稳定演进的小而精业务系统。OMS 雏形和既有 OA 项目只作为案例与信息来源，不自动成为本项目的产品事实。

工程贯通不等于商业适配已经成立。最终目标是让目标商业圈子的客户主要通过小系统选装、配置、策略、类型化扩展和客户装配完成交付，同时避免万能平台与客户核心分支两个极端；该平衡按[小系统群产品边界验证](./docs/specs/assembly/product-boundary-validation.md)使用真实场景持续检验。

跨客户产品线采用“稳定 Base + Customer Overlay”作为待验证方向：客户差异优先以 Suite Manifest、配置、策略、类型化扩展、连接器、只读投影和隔离专项做加法，不覆盖公共内核或长期 fork 核心代码。首个参考 OA 只证明该方向可以开始验证，不能代替第二客户和升级证据。

远期商业默认形态还包括一套可直接使用、独立发布的成熟标准 Web 前端：低定制客户选装小应用后先用标准产品，少量差异走受控界面扩展；确需完全定制的客户可替换前端，但仍使用同一业务契约和 IAM。OSS 静态托管是标准前端的目标部署选项，当前 Go Workbench 和网页 Bucket 尚不构成这项能力；边界与验证门槛见 [BF-080](./docs/decisions.md) 和 [产品边界验证](./docs/specs/assembly/product-boundary-validation.md)。

产品验证的阶段一以现有完成度较高、定制化明显的 OA 为参考产品：先盘点其真实能力和核心旅程，再验证当前小系统能否重组出一套可用、可解释、可独立交付的参考 OA。该目标不要求团队预先掌握整个行业，也不把旧 OA 当成行业标准或逐页面复制清单；验收边界见[阶段一参考 OA 小系统重组验证](./docs/specs/assembly/oa-reference-stage-1.md)。

2026-10-02 用户进一步明确要求**完整仿制原 OA 的界面与操作效果**，修复工作已经开始；上段代表旅程／非逐页范围限制按 [BF-084](./docs/decisions.md) 被取代。界面、全部当前可用动作、字段时机、导航与权限逐项对应；原站占位和冲突单列，独立小系统边界及无技术债照搬仍保持。

2026-10-02 历史页签修复批次 `v0.1.0-oa.20261002.6`：[公网 OA 工作台](https://oatest.antuinfo.cn/workbench/)。按 BF-092 修复同项目九个经营页签及生产入口不可点击，native Chrome 逐个点击、刷新、Back／Forward 和甲方编辑取消保护通过。只替换 Workbench，其他 16 容器、Project／IAM／schema／Grant 保持；13 库备份／空目标恢复和离机 SHA 通过。该批项目缺商业 Link，Cost／Bonus 未授原生读取 Grant，经营成员及完整表单未接通，不能称业务全可用或完整 OA 通过；见[页签修复报告](./docs/reports/oa-business-tabs-repair-validation-20261002.md)。该历史批次和后续 `.11`／k 已由上方最新部署状态取代，原反例与未完成范围保留。

此前 `.4` 关闭同一 C 的核心立项／编辑连续路径及长期开页 CSRF 阻断；`.5` 修复未知提交误称“尚未完成”和重新加载后的旧抽屉遮挡。其历史冻结范围见[恢复报告](./docs/reports/oa-acknowledgement-recovery-validation-20261002.md)及[.4 连续证据](./docs/reports/oa-csrf-continuation-validation-20261002.md)，不能拿用户后来完成后的项目去比旧待立项快照。未知回复未在公网制造；备份／空目标数据恢复不是完整 Suite 恢复运行。**完整立项字段、权限矩阵及完整 OA 均未验收**，继续按[分步清单](./docs/specs/assembly/oa-original-usage-workflow-baseline.md)验证，不用新夹具绕过失败。

当前第一阶段 **OA 完整体架构可行性验证** 已在公司 Ubuntu 用同一 Tenant／Project 连续执行到 C8，并冻结 [阶段验证报告](./docs/reports/oa-full-scope-architecture-validation-report.md)。C0～C7 通过；C8 的个人工作、Money 分权、单来源恢复和十二库事实不变通过，但 C4～C6 资料 Link／投影与 Evidence Gateway 独立下载入口未闭合，因此完整结论保持 `insufficient-evidence`。这两项是当时连续工程链的阻断，不是可用 OA 的全部剩余工作。2026-10-02 用户试用又暴露立项输入时机、双负责人门槛及跳出统一项目界面的偏差；下一步先按[原 OA 使用流程基线](./docs/specs/assembly/oa-original-usage-workflow-baseline.md)逐操作比对，再分批实现和真人验收，不能以关闭 C8 自动宣布产品通过。第二参考客户仍待选择。

首个参考 OA Demo 采用 J1 项目立项、J2 合同与回款、J3 设计生产交付、J4 资料追溯，现已用六个独立应用／数据库、同一组非真实数据、模板 IAM／本地账号和公开接口通过本地业务架构验收；从最小组合到高保真组合的实际变化也已通过六层归属、无客户核心分支和兼容性检查。携带当前驱动的 `v0.1.0-demo.11` Linux/amd64 开发包已从同一无源码归档依次复现六个独立产品、设计交付／商业台账两类子集、两次高保真旅程、五来源隔离和最终健康状态。Nginx、Gateway、中心 IAM、正式 clean-HEAD 发布、真实公司 Ubuntu 与公网入口继续使用同一业务发布物在后续环境单独验收。该顺序不允许关闭鉴权，也不改变完整组合的固定入口链。

当前 **Architecture Baseline 0.1 已确认**：应用组合、数据所有权、`Nginx → Gateway → IAM／目标应用` 入口链、IAM 权限协作、独立 Evidence Service、Suite Manifest 和 D1～D4 Deployment Profiles 都是后续开发必须遵循的架构基线。基线确认表示方案不再作为候选反复摇摆，不表示尚未编码的服务已经实现。

## 当前应用组合

- Nginx：完整组合唯一公网 HTTPS 边缘入口，负责域名、静态资源和基础反向代理；M7-003 D1 配置已通过 Nginx 1.28 容器校验与真实链路验收；
- Gateway：Nginx 后方的独立、小型 Go 应用入口，使用版本化静态路由，负责限流、基础验签和身份 Header 清理，但不拥有登录账号和授权事实；M7-003 最小入口已实现并验证；
- Identity & Access：拥有租户、账号、邀请／激活、登录、会话、密码重置、Capability 授予和 Ed25519 凭证的强制平台应用；M7-002 最小纵向闭环已实现并通过真实 PostgreSQL 验收；
- People & Groups：以 Group 为核心管理 Person、Membership、组内角色、可配置能力画像和不可变名册快照；M8 最小闭环已实现并通过独立 PostgreSQL 与 D1 恢复验收；
- Party Directory：独立领域应用，拥有主体当前档案、稳定 ID 与不可变 revision；已实现登记、查询、更正、停用／恢复、历史、IAM Capability、精确引用和可操作管理页，并通过真库／并发／权限、release／独立恢复、Project 消费边界、D1 API 正式链／故障／联合恢复及 Project 客户选择验证。在主体管理场景中可以是主要产品，在 Contract、Money、Project 或 Business Engagement 组合中也可作为已声明依赖随包交付。Contract／Money 已具备确定性预检、来源自有不可变映射和追加运行时转换账；含引用数据的真库切换／只读回滚、本地写权冻结与防篡改均已通过，Contract-linked C6 又在正式 D1 完成 `cutover → rollback → recutover`、联合备份和空目标恢复。完整双租户矩阵和 Money 完整 D1 消费链尚未验证；
- Business Engagement：独立经营承接应用，拥有适用性、承接方式、材料业务引用、revision／历史和下游建议交付状态；后端闭环、PostgreSQL 18 真库、精确 Project／File 工作负载凭证、D1 镜像／release／Suite Manifest、Workbench 客户页面、浏览器撤权刷新、Nginx→Gateway→IAM C2 权限矩阵、本地 D1 空目标恢复／继续写入、409／503 自动化与真实浏览器恢复，以及账号停用、IAM／Policy 故障和恢复审计专项已通过；下游建议和完整双租户 D1 矩阵仍待实现；不把材料齐备冒充中标、签约或资金事实；
- Billing / Invoice：独立拥有已开票登记、金额、Party／来源快照、Evidence 引用和不可变纠错历史；领域／真库／精确引用、完整生命周期 HTTP、可移植交付、联合恢复与正式 D1 入口已通过，人类工作区现已进入 Workbench，并验证金额撤权／恢复、历史脱敏、无权盲写拒绝、409 和依赖故障恢复；C3 分段只读组合、完整角色／双 Tenant、Billing-only 与真实 Ubuntu 仍待验；
- Approval & Audit Results：独立拥有批复／审计结果、精确金额与来源 revision、Evidence 引用和不可变纠错历史；领域核心、PostgreSQL 18、受保护 HTTP／IAM、精确引用、按 Project 的人类工作区、正式 D1 装配／裁剪／联合恢复和 74 断言真实跨进程旅程已通过，原 OA 双列又在私有真实浏览器完成 11／11、回执重放、纠正／撤回重登记、撤权、六所有者恢复和 Evidence 字节保持；Project 以 `reference-oa-c5-v1` Completion Profile 纯读取评估并用显式命令追加检查点确认，但本次浏览器切片未执行该检查点；Main／公网、正式 D1／keys、完整双 Tenant、全角色、账号停用与第二客户仍待验；
- Cost & Outsourcing（C6 当前参考范围通过）：独立双聚合核心、PostgreSQL 18、版本化 HTTP、正式 IAM、精确 adapter、人类工作区、正式 D1、Party 切换与恢复均已验证；连续旅程又通过正式 Cost→Money connector、目标停机恢复、独立接受和完成态重跑。完整双 Tenant、跨进程撤回与真人浏览器仍为补证轴；
- Bonus Distribution（C7 当前参考范围通过）：独立领域／PostgreSQL、正式 IAM、真实来源、人类工作区、正式 D1 与恢复均已验证；连续旅程复用同一 Project 的 Settlement／Receipt Set 和 C1 Person／Assignment，完成整数 Overlay、正式建议 connector、Money 停机恢复、独立接受、来源撤回不级联和完成态重跑。单类型最小组合、双 Tenant 与真人连续浏览器仍为补证轴；
- C8 资料、统计与个人工作（退出门禁失败）：同一 Project 的 BE→Evidence 资料、当前 Subject 工作、Money grant/revoke、Evidence／Production 顺序故障恢复和十二库 canonical digest 不变已经在公司 D1 通过；但 C4～C6 输出没有 active Project Link／资料投影，Gateway 又要求遗留 `evidence-service.view` 才允许独立 download 进入。Evidence 页面使用的无效 `redirect` 已在源码修为 `direct`，仍须以新摘要和原 Project 重跑；
- Project Workspace：可选项目容器应用；J1 独立运行时已实现 Project、Assignment、类型化 Resource Link、不可变历史、模板／正式 IAM 和外部引用故障降级；现已接入 D1 Suite Manifest、Gateway、IAM Resource Scope、独立数据库、镜像与联合恢复，真实公司 Ubuntu 总装仍待验；
- Design Operations／目标 Production Operations：J3 独立运行时已实现固定 `design-v1` 四阶段、生产负责人、Evidence 成果引用、显式回退和不可变历史，并通过 J3-A01～J3-A20、PostgreSQL 18、Docker、恢复、release 及本地 OA J1～J4 组合验收。C4 又通过版本化 Type Definition、`consultation-v1`、旧 Design 隔离、完整 fixture migration、同 Project 双类型生命周期、canonical 权限、最小组合、Level A／B 回归，以及隔离 D1 的 Suite Manifest／Workbench、正式入口、未选缺席、依赖故障与联合恢复；Production 专项撤权／停用、双 Tenant、真实浏览器和 Ubuntu 待补证；
- Contract Ledger：合同台账小系统，已经实现 v1；
- Money Flow Ledger：经营收支台账小系统，已经实现 v1；
- Evidence Service：独立的文件与证据小服务；M10-001 File Transfer Core、M10-002 独立服务内核和 M10-003 D1 装配均已完成工程验证，支持本地文件与阿里云 OSS Provider、强制 IAM／模板凭证、服务端完整性确认、清理和备份恢复。Contract Ledger 当前仍使用嵌入式本地实现，迁移属于后续独立切片。

Nginx、Gateway、统一工作台和部署运维包属于正式装配组件，不拥有业务事实；完整组合固定使用 `Nginx → Gateway`，单应用开发可以直接访问应用入口。小应用的独立交付以清晰数据所有权、显式依赖、独立 migration／升级和可验证失败行为为准，不要求单进程或零依赖。应用没有永久的“核心／辅助”商业等级；角色由具体客户的业务结果决定，但数据所有权和契约边界保持稳定。最小搭建选择能够完成目标业务结果的最小应用集合及其直接依赖，并允许以后按版本化契约扩展。Project、审批、客户 OA 和其他未声明应用不是 Contract Ledger 或 Money Flow Ledger 的隐式前提；IAM、Party、Evidence 等依赖按版本化契约和 Suite Manifest 随实际产品包声明。有文件功能的交付附带 Evidence Service，而不是在业务代码中再实现一套上传下载。

工程交付基线是 **Complete Suite 0.1**：最低用一台 ECS 即可形成可安装、可登录、可配置应用开关、可完成业务演示并可备份恢复的完整组合；同一套发布物还必须能按 Deployment Profile 拆为两台、三台或高可用多节点。Suite Manifest 决定启用哪些应用，Deployment Profile 决定服务放在哪些节点和运行多少副本。报销、奖金等应用负责“为什么产生这笔钱”，Money Flow 只负责已经确认的应收应付、实际资金、核销和余额；合同外收支可以通过人工或其他来源快照直接进入 Money Flow，不要求依赖 Contract。

## 当前开发状态

M0、M1、M2、M3、M4、M7、M8、M9、M10 已完成各自声明范围的工程验证。公司 D1 已装入十二个业务应用及 Nginx、Gateway、IAM、Workbench、PostgreSQL，连续旅程在同一 Tenant／Project 通过 C0～C7，并完整执行 C8。阶段报告以 49 个 C8 HTTP 断言、十二库前后完整事实摘要和 17/17 health 保留失败证据；当前不能宣称完整 OA 已通过，只能宣称解耦方向在已验证闭环中可行且没有发现必须共享业务表或长期 fork Base 的反证。Contract 遗留文件迁移、双 Tenant、公网 HTTPS、离机恢复和第二客户仍为独立后续轴：

- `backend/platform/identity-access`、`backend/services/people-groups`、`backend/services/party-directory`、`backend/services/project-workspace`、`backend/services/business-engagement`、`backend/services/billing-invoice`、`backend/services/design-operations`（Production Operations）、`backend/services/approval-audit-results`、`backend/services/cost-outsourcing`、`backend/services/bonus-distribution`、`backend/services/contract-ledger`、`backend/services/money-flow-ledger` 和 `backend/services/evidence-service` 是十三个独立数据所有者；Gateway 与 Unified Workbench 另为无业务数据库的独立 Go 装配应用；
- 每个应用拥有独立 PostgreSQL 数据库、migration、配置和测试；
- Identity & Access 的 `M7-002` 已完成并验证：空库首管、邀请激活、登录注销、密码重置、Session 失效、Capability Manifest 授权、Ed25519 单 Audience 凭证、密钥轮换与不可变审计形成独立闭环；
- Nginx、Go Gateway 与 Unified Workbench 的 `M7-003` 已完成并验证：统一登录入口、静态应用路由、禁用应用隐藏、身份 Header 清洗、Ed25519 基础验签、Route Prefix Cookie、限流和访问日志形成最小入口闭环；
- `M7-004` 与 `M7-005` 已让 Contract、Money Flow 同时支持正式 IAM 和独立本地模板模式；同一登录双应用、单 Audience 隔离、IAM Subject 审计和单侧停机均已验证；
- `M7-006` 已完成，并由 M8、M10-003 与 Project D1 装配扩展组合交付：Compose 只暴露 Nginx，一个 PostgreSQL 实例保持六个应用数据库／账号，应用开关不删数据，联合备份可在销毁卷后的空目标恢复并继续写入；
- `M8` 已完成并验证：可配置 Group Type、Membership Role 与 Capability，Person 可加入多个 Group，Membership 的加入／改角／结束／重入保留历史，Group 可发布带 revision 与 SHA-256 的不可变 Roster Snapshot；正式 IAM、本地模板、公开 API、最小页面、故障隔离、应用开关和联合恢复均通过；
- `M9-J1` 已完成独立业务切片与 D1 资源权限验证：Project 草稿、责任 Assignment、立项、类型化外部 Link、取消和不可变历史在独立 PostgreSQL 中闭环；A01～A18、两种 IAM Provider、引用服务故障、独立 Docker image 与容器冒烟均通过；D1 中 Project A 精确 `read/update` 允许、Project B 凭证／读写拒绝及恢复后回归通过；
- `M9-J3` 已完成独立业务切片验证：Design Work Item、固定四阶段、有效生产负责人、Evidence File Version 成果、适用性、替换、回退、取消和不可变历史在独立 PostgreSQL 中闭环；J3-A01～J3-A20、两种 IAM Provider、Project／Assignment／Evidence reference-read、故障关闭、备份恢复、Linux release 和非 root Docker 冒烟均通过；
- `M9` 本地参考 OA 组合已通过：`deployments/local-oa` 启动六个应用及六个独立数据库，`demo` 在同一组非真实数据上连续完成高保真 J1～J4且重复执行保持相同业务身份，`failure-isolation` 依次停止 Contract、Money、People、Evidence、Design 时 Project 仍保持 active、Assignment 快照可读且仅对应实时引用降级；`v0.1.0-demo.11` 又从同一无源码 Linux/amd64 归档复现六个独立产品、两个可裁剪子集和同一全量结果。Project、Business Engagement、Billing 与 Production 后续均已进入 D1 正式入口验证；其中 C4 Production 最小组合完成 49 条入口断言、未选应用缺席、依赖故障恢复和联合空目标恢复。Production 专项撤权／停用、双 Tenant、真实浏览器、真实公司 Ubuntu 与公网 HTTPS 仍待验；
- `M10-002` 已完成独立应用工程验证：Evidence 拥有独立 PostgreSQL、Upload Session、不可变 File Version、技术审计与清理；本地 Provider 真实 HTTP 闭环、OSS 单对象 STS Policy、IAM／模板鉴权、并发幂等、冷备份空库恢复、Docker 独立启动和 release 构建均通过；
- `M10-003` 已完成 D1 装配与破坏性恢复验证：Evidence 接入 Suite Manifest、IAM、Gateway、可选 Nginx 本地数据面、应用开关和联合备份恢复；2026-09-22 又在真实私有阿里云 Bucket 上通过精确 CORS、1 MiB 上传下载、150 MiB multipart、取消清理、服务端 SHA-256 复核和匿名拒绝。Contract 迁移、网络中断重试等剩余负例及真实公网入口仍是后续项；
- 独立启动和交付边界已经验证，当前按单个纵向功能推进 Money Flow；
- Contract Ledger 的 `CL-M1-001` 至 `CL-M1-006` 已完成并验证：从本地登录、主体登记、合同草稿、签约主体、编辑取消到激活并形成不可变 revision 1；
- Contract Ledger 的 `CL-M2-001` 已完成并验证：生效补充协议按显式顺序形成 revision 2+，支持精确金额调整、金额重述和金额不变；
- Contract Ledger 的 `CL-M2-002` 已完成并验证：生效终止文书形成下一 revision 和 `terminated` 终态，金额与结算事实保持分离；
- Contract Ledger 的 `CL-M2-003` 已完成并验证：按关键词、状态和币种检索合同，并通过绑定筛选条件的 keyset 游标稳定分页；
- Contract Ledger 的 `CL-M2-004` 已完成并验证：以引用两端 revision 快照的不可变有向边记录合同关系，支持类型化防环、双向查看和稳定并发；
- Contract Ledger 的 `CL-M2-005` 已完成并验证：阶段／最终结算以连续的不可变累计金额事实独立于合同金额保存，并冻结明确确认主体快照；
- Contract Ledger 的 `CL-M2-006` 已完成并验证：active／terminated 合同经明确结算处置形成不可变操作性关闭事实，不生成新 revision，也不改写金额或结算；
- Contract Ledger 的 `CL-M2-007` 已完成并验证：合同、文书、结算或关闭事实可挂接版本化文件证据，按实际字节保存 SHA-256、受保护下载并形成可信 Actor 审计；
- Contract Ledger 的 `CL-M2-008` 已完成并验证：管理员以类型化更正文书追加当前正式合同的 revision，旧 revision 与结算、关系、关闭和证据引用保持不变；
- Contract Ledger 的 `CL-M2-009` 已完成并验证：管理员可下载单合同版本化 ZIP 账簿，以 SHA-256 离线核验完整历史，Evidence 文件内容与内部存储键不会进入归档；
- Contract Ledger 的 `CL-M2-010` 已完成并验证：管理员可把通过严格预检且不含外部 Relation 的 v1 单合同账簿原子恢复到空业务库，恢复操作幂等可审计，缺失 Evidence 字节不会伪装为可下载；
- Contract Ledger 的 `CL-M2-011` 已完成并验证：以同一 PostgreSQL 导出 snapshot 生成数据库与 available Evidence 的可校验全量备份，并可恢复到新空数据库和新 Evidence 目录；
- Contract Ledger 的 `CL-M2-012` 已完成并验证：生成带统一 build identity 与 SHA-256 的独立 release bundle，以显式 schema 门禁完成空环境安装、历史升级、失败回滚和 `verify` 模式启动；
- Contract Ledger 的三种固定角色执行服务端授权，业务写入只能使用登录会话中的可信 Actor，并与不可变审计原子提交；
- Money Flow Ledger 的 `MF-M3-001` 已完成并验证：空用户库原子建立首位本地管理员，以 BCrypt、哈希会话、失败锁定、CSRF 和不可变身份审计形成独立登录边界；
- Money Flow Ledger 的 `MF-M3-002` 已完成并验证：管理员可登记本系统自己的 organization／individual 主体，以 Command ID 幂等、可信 Actor 和原子审计形成 revision 1；
- Money Flow Ledger 的 `MF-M3-003` 已完成并验证：管理员可相对于明确本方主体建立人工应收／应付草稿，冻结债权方／债务方 Party 快照并保存精确金额、到期日和人工来源；
- Money Flow Ledger 的 `MF-M3-004` 已完成并验证：管理员显式消费 Expected Draft Version，把草稿事实冻结为不可变 Obligation revision 1，并原子推进为 `open`；
- Money Flow Ledger 的 `MF-M3-005` 已完成并验证：管理员可登记独立、不可变的实际收款／付款，冻结付款方／收款方快照，并以 Command ID 和外部流水唯一范围防止重复资金事实；
- Money Flow Ledger 的 `MF-M3-006` 已完成并验证：管理员可把一笔兼容资金部分或全部分配给一条 open 义务，两侧余额从不可变 Allocation 求和，并在并发下保持不超额；
- Money Flow Ledger 的 `MF-M3-007` 已完成并验证：重复使用单笔 Allocation 命令可组成一对多、多对一核销，资金和义务详情可下钻各自完整、确定排序的分配历史；
- Money Flow Ledger 的 `MF-M3-008` 已完成并验证：同一本方主体下的内部账户转账以独立、不可变、非经营性事实保存，不生成两笔虚假收付款，也不进入 Allocation；
- Money Flow Ledger 的 `MF-M4-001` 已完成并验证：错误 Allocation 通过独立、不可变、一次性完整 Reversal 纠正，原事实和历史保留，资金与义务余额按净额恢复并可重新分配；
- Money Flow Ledger 的 `MF-M4-002` 已完成并验证：只在 Allocation 净额为 0 时以独立、不可变 Reversal 完整冲销错误 Cash Transaction，当前有效金额归零且永久禁止重新分配；
- Money Flow Ledger 的 `MF-M4-003` 已完成并验证：open Obligation 的金额增加或减少以独立 Adjustment 连续追加 revision，原始金额和历史 Allocation 保留，当前余额按最新有效金额与净分配额投影；
- Money Flow Ledger 的 `MF-M4-004` 已完成并验证：已经生效的义务减免以独立、不可变 Remission 保存，不改写 revision，当前余额统一按有效金额减净分配额再减累计减免额投影；
- `MF-M4-005` 坏账处置已完成边界审计但延期实现：现有材料不足以区分应履行余额、预计可收回额、会计减值和追回款规则，系统不会用减免或普通调额冒充坏账；
- Money Flow Ledger 的 `MF-M4-006` 已完成并验证：余额由 Allocation／Reversal 与 Remission 精确归零后，管理员以 Expected Current Revision 和单调 Balance Sequence 显式形成不可变 Closure，并把义务推进为禁止继续余额写入的 `closed` 操作终态；
- `MF-M4-007` Billing Document／Billing Link 已完成边界审计但延期实现：参考材料不足以定义票号唯一范围、作废／红冲／更正链和票据—义务多对多匹配／撤销规则，系统不会复制 OA 的统一 FinancialRecord 或用附件、现金关联冒充票据；
- Money Flow Ledger 的 `MF-M4-008` 已完成并验证：管理员可把查询时的 open 正余额义务按今天或未来分类日划入未到期、1–30、31–60、61–90、91+ 天账龄，按方向与币种精确汇总并下钻；过去日期明确拒绝，不用 current 余额冒充历史 `as_of`；
- Money Flow Ledger 的 `MF-M4-009` 已完成并验证：管理员可按带时区 RFC3339 记录时刻重放单条正式义务的 revision、Allocation／Reversal、Remission、Closure 与精确余额；数据库以单调 recorded-time 门禁拒绝倒退事实；
- `MF-M4-010` 真实退款／退回款已完成边界审计但延期实现：现有材料只能确认它是引用原资金的第二次真实资金移动，尚不能确定已核销 Allocation、Obligation 余额和已关闭义务如何处理；系统不会用负金额、Cash Reversal 或错误核销 Reversal 冒充退款；
- Money Flow Ledger 的 `MF-M4-011` 已完成并验证：管理员可按本方账簿主体与币种查看当前应收、应付、有效实际收付和未分配资金六项精确投影，并逐项下钻到构成金额的 Obligation 或 Cash Transaction；
- Money Flow Ledger 的 `MF-M4-012` 已完成并验证：管理员可按单一本方账簿主体导出固定 ZIP／CSV 台账，manifest 对每张事实表记录行数、字节数和 SHA-256；归档保留完整历史与关联，但不包含身份凭证、其他账簿或派生汇总；
- Money Flow Ledger 的 `MF-M4-013` 已完成并验证：运维人员可从同一 PostgreSQL snapshot 生成带完整逐表计数与 dump SHA-256 的版本化全量备份，并安全恢复到同 major 的全新空数据库；本地会话数据明确排除；
- Money Flow Ledger 的 `MF-M4-014` 已完成并验证：三个二进制共享 build identity 并组成带 SHA-256 的独立 release bundle，以显式 schema 状态、单事务 migration 和 `verify` 启动完成空库安装、历史升级、失败回滚与升级前备份恢复演练；
- Money Flow Ledger 的 `MF-M4-015` 已完成并验证：管理员可用不可变 `set`／`clear` 事实维护独立于到期日的预计现金日期，并按当前未结余额查看隔离到本方主体、方向和币种的已排期／未排期现金；
- M4 剩余范围已完成边界审计：未分配资金足以安全承载尚未归类的预收／预付与超额；通用导入因批次权威、业务键、冲突和纠错语义不足而延期，不以一个会绕过既有不变量的上传入口冒充完整能力；
- M6 的 `MF-M6-001` 已完成并验证：实际 Money Flow release bundle 能在随机全新 PostgreSQL 数据库完成严格解包、迁移、`verify` 启动、首次登录、代表性收支链路、v2 导出和无 bootstrap 重启；该验收发现并修复了 macOS release 中意外 AppleDouble entry 和临时数据库 URL 两项交付缺陷；
- M6 的 `MF-M6-002` 已完成并验证：实际 Money Flow release bundle 能从在线随机源库创建严格全量备份，经独立摘要移交后恢复到全新空库；旧会话失效、原账号重登、代表性事实和继续写入均通过，非空目标与损坏归档被安全拒绝；
- M6 的 `CL-M6-003` 已完成并验证：实际 Contract release bundle 能从随机空库和新 Evidence 根完成安装、合同草稿与文件上传下载，并将数据库+Evidence 联合备份交接恢复到另一组空目标；非空库、已存在 Evidence 根与损坏文件归档均被安全拒绝；
- M6 的 `M6-004` 已完成并验证：两个实际 release 可在同一主机、不同随机数据库与端口同时运行；同一浏览器双登录、四个应用专属 Cookie、交替单侧停机／无 bootstrap 重启、另一侧持续健康与写入、数据库／Evidence 零交叉均通过；
- M6 的 `M6-005` 已完成边界审计：设计单位 OA、资质施工综合体和小型咨询服务公司只是参考装配探针；现有台账能力、条件假设、明确缺口和待客户确认事项已经分开，未修改核心表或制造客户定制与跨系统业务流；
- M6 的 `M6-006` 已完成退出缺口审计：核心安装、升级、恢复与组合隔离已有直接证据；审计识别出的双 Release 同主机运行手册缺口已由 `M6-007` 补齐，性能、导入、客户权限和真实场景验收继续等待明确输入；
- M6 的 `M6-007` 已完成并验证：双 Release 同主机运行手册明确独立配置、迁移、启停、备份、恢复、升级和禁止项；实际 Contract `test.3`、Money Flow `test.5` 组合与两套 clean-room 备份恢复复演均再次通过；
- M6 的 `M6-008` 已完成并验证：正式构建只能绑定干净 Git HEAD；首个本地基线 `cdc7bc8…` 的 Contract 与 Money Flow `v0.1.0-baseline.1` official-mode 候选包具有一致六二进制 identity，并通过组合与两套恢复验收；
- M3 与 M4 退出门槛均已有直接自动化和独立运行证据；延期项有明确事实缺口，不以会计总账、客户定制流程或空连接器扩大 v1；
- 运行和验证方式见 [验证入口](./docs/testing.md)；
- 稳定工程事实见 [当前工程架构](./docs/architecture.md)。

## 文档入口

- [小系统边界演变账](./docs/specs/assembly/small-system-boundary-evolution-ledger.md)：持续记录 OA 仿制对 Project、Production、Contract 及跨系统依赖的真实影响、兼容策略和下一证伪条件。

- [开发决策总表](./docs/decisions.md)：后续设计、实现和评审必须首先遵循的唯一决策索引；
- [当前讨论基线](./docs/foundation.md)：后续讨论首先以此为共同上下文；
- [开发计划](./docs/development-plan.md)：按独立可交付成果组织的当前开发路线；
- [阶段一参考 OA 小系统重组验证](./docs/specs/assembly/oa-reference-stage-1.md)：明确第一阶段为什么选择现有 OA、如何确定核心旅程以及什么才算重组成功；
- [参考 OA 全景能力与核心旅程审计](./docs/specs/assembly/oa-reference-capability-journey-audit.md)：盘点参考 OA 的实际能力、当前小系统覆盖、明确缺口，并提出阶段一首个核心旅程集合；
- [原 OA 怎么用：操作流程基线](./docs/specs/assembly/oa-original-usage-workflow-baseline.md)：按原站入口、输入时机、保存／跳转、权限和失败路径梳理；明确源码证据、原站冲突及真人未验范围；
- [OA 完全体业务重组验证](./arckit/spec/assembly/full-reference-oa-validation.md)：固定全业务 OA 的主关系、当前覆盖、明确缺口和 C1～C8 验收门槛；
- [OA 完整体架构可行性验证计划](./docs/oa-full-scope-validation-plan.md)：当前下一阶段主线，串联本地业务重组、Base／Overlay 反证、真实环境门槛和最终报告；
- [OA 完整体架构可行性验证报告模板](./docs/templates/oa-full-scope-architecture-validation-report.md)：固定阶段结论等级、业务覆盖、组合边界、环境证据和下一轮输入；
- [OA C2 IAM 边界与故障恢复实验](./docs/reports/oa-c2-iam-boundary-experiment-20260927.md)：账号停用、IAM／Policy 故障、失败不签发、恢复后单次签发与目标错租户凭证拒绝的直接证据；
- [稳定 Base 与客户覆盖层调研](./arckit/tech/assembly/customer-overlay-research.md)：说明加法式定制能放什么、不能放什么，以及何时必须演进 Base；
- [第二个真实客户场景采集与就绪判定](./docs/operations/second-customer-scenario-intake.md)：用脱敏访谈、六层归类和确定性 readiness report 准备跨客户边界验证；
- [参考 OA Demo 规格](./arckit/spec/assembly/reference-oa-demo.md)：固定 J1-J4、本地业务验证模式、设计首类型、人工义务确认和最终通过条件；
- [本地参考 OA 业务组合](./deployments/local-oa/README.md)：启动六个独立应用／数据库，重复执行 J1～J4 并验证单引用服务故障隔离；
- [Design Operations 规格](./arckit/spec/design-operations/design-production-delivery.md)：固定 J3 `design-v1` 的四阶段、负责人／成果引用、纠错边界和 A01～A20 验收；
- [M7 已完成行动方案](./docs/next-stage-action-plan.md)：M7 统一入口与 IAM 最小闭环的历史执行顺序、范围和退出门槛；
- [M8 People & Groups 行动方案](./docs/m8-people-groups-action-plan.md)：Person、Group、Membership、能力画像、名册快照和 D1 装配的执行顺序与退出门槛；
- [M8 People & Groups 规格](./docs/specs/people-groups/m8-minimal-vertical-slice.md)：v1 数据、不变量、API、权限和验收矩阵；
- [M7-002 IAM 最小纵向闭环](./docs/specs/identity/iam-minimal-vertical-slice.md)：当前 IAM 的职责、API、数据、安全规则与真实验收证据；
- [M7-003 统一入口](./docs/specs/assembly/unified-entry-m7-003.md)：Nginx、Go Gateway、Workbench 的职责、路由、安全与真实链路证据；
- [M7-006 D1 组合交付](./docs/specs/delivery/d1-combined-delivery-m7-006.md)：当前组合的安装、开关、故障、升级、联合备份与整机恢复证据；
- [M10-003 Evidence D1 装配](./docs/specs/evidence/m10-003-d1-integration.md)：Evidence 的入口、开关、字节路径、故障隔离和联合恢复证据；
- [M10-004 Contract Evidence 迁移](./docs/specs/evidence/m10-004-contract-evidence-migration.md)：旧附件到独立 Evidence 的确定性映射、预检、切换和回退边界；
- [阿里云 OSS 首次真实验收](./docs/operations/aliyun-oss-first-validation.md)：私有 Bucket、RAM 策略／上传角色／本地调用用户的完整创建流程、CORS、本机凭证、ECS 身份替换和联合验收清单；
- [阿里云账号协作管理接入](./docs/operations/aliyun-account-access.md)：区分 CLI 控制面与 ECS SSH，使用 OAuth／RAM 最小权限建立可撤销管理入口；
- [阿里云 ECS D1 宿主机初始化](./docs/operations/aliyun-ecs-d1-bootstrap.md)：记录购买与镜像选择、SSH 密钥、Ubuntu 重装、Docker／Compose、Swap、SSH 加固、安全组、ACR 和重启验收；
- [ACR 私有镜像供应](./docs/operations/acr-image-supply.md)：记录个人版云控制面、Linux AMD64 基础／应用镜像发布、摘要锁定和 ECS 拉取门禁；
- [D1 Compact 运行手册](./docs/operations/d1-compact.md)：单台 ECS 配置、安装、升级、备份和恢复步骤；
- [D1 双环境制品提升与公网验证](./docs/operations/d1-environment-promotion.md)：个人试验田与公司并行环境的发布包、跨 Registry 提升、输入清单、验收、恢复与备案等待分支；
- [公司私有 Ubuntu 环境验证手册](./docs/operations/company-environment-validation-playbook.md)：授权输入、独立秘密／数据、制品提升、完整入口和恢复验收；
- [ICP 等待期与并行验证清单](./docs/status/2026-09-24-icp-wait-and-parallel-validation.md)：未来两天与备案等待期的可执行 Todo、公司会议输入和获批／未获批分支；
- [开发进度](./docs/progress.md)：已完成里程碑、功能和验证证据；
- [Deployment Profiles](./docs/specs/assembly/deployment-profiles.md)：同一产品在一台、两台、三台和高可用多节点上的放置与验收规则；
- [Contract → Money Flow：OA 参考证据审计](./docs/specs/connectors/contract-money-oa-reference-audit.md)：区分可复用业务信号、客户定制和 M5 启动缺口；
- [Contract → Money Flow：OMS 雏形参考证据审计](./docs/specs/connectors/contract-money-oms-prototype-reference-audit.md)：验证审批确认与合同直接统计之间的关键反证；
- [Contract → Money Flow 真实业务案例采集表](./docs/templates/contract-money-flow-case-intake.md)：用于提交第一条脱敏业务流；
- [M5 启动就绪门禁](./docs/specs/connectors/m5-start-readiness-gate.md)：九项事实通过后才进入契约设计与联调方案；
- [讨论过程索引](./docs/discussions/README.md)：保留每轮讨论的问题、推导和开放事项；
- [第 1 轮：总体业务与模块拆分](./docs/discussions/round-01-contract-income-expense-foundation.md)；
- [第 2 轮：合同内核](./docs/discussions/round-02-contract-kernel.md)；
- [第 3 轮：经营收支内核](./docs/discussions/round-03-money-flow-kernel.md)；
- [第 4 轮：可组合小系统蓝图](./docs/discussions/round-04-composable-foundation-blueprint.md)；
- [第 5 轮：小系统产品化与耦合约束](./docs/discussions/round-05-small-system-productization.md)；
- [第 6 轮：开发计划与里程碑顺序](./docs/discussions/round-06-development-plan.md)；
- [第 7 轮：最小根因与总计划校正](./docs/discussions/round-07-minimum-cause-principle.md)；
- [第 8 轮：强制 IAM 契约与权限解耦](./docs/discussions/round-08-mandatory-iam-and-permission-decoupling.md)；
- [第 9 轮：应用组合与单节点架构](./docs/discussions/round-09-application-portfolio-and-single-node-architecture.md)。
- [第 10 轮：完整组合、人员管理与收支来源边界](./docs/discussions/round-10-complete-suite-and-financial-source-boundaries.md)。
- [第 11 轮：人员编组与可配置能力](./docs/discussions/round-11-people-groups-and-configurable-capabilities.md)。
- [第 12 轮：Gateway、IAM 与参考认证工程适配](./docs/discussions/round-12-gateway-iam-and-nebula-auth-fit.md)。
- [第 13 轮：IAM、业务事实与小应用权限链解耦](./docs/discussions/round-13-permission-chain-decoupling.md)。
- [第 14 轮：固定入口链与 OSS Evidence Service](./docs/discussions/round-14-fixed-entry-chain-and-oss-evidence-service.md)。
- [第 15 轮：单台 ECS 是最低档，不是架构上限](./docs/discussions/round-15-deployment-profiles-not-single-node-limit.md)。
- [OA 阿里云 OSS 文件传输参考审计](./docs/specs/evidence/oa-oss-transfer-reference-audit.md)。
- [M10-002 Evidence Service 最小纵向闭环](./docs/specs/evidence/m10-002-evidence-service.md)。

## 交付验收入口

- `acceptance/combined-delivery` 是只消费两个实际 release 的同主机组合验收模块，不属于任一业务系统，也不实现业务连接器；
- 三类参考装配场景的当前可复用范围和事实缺口见 [M6-005 边界审计](./docs/specs/assembly/three-reference-scenarios-boundary-audit.md)；
- 双 Release 操作者步骤见 [同主机运行手册](./docs/operations/combined-same-host-operation.md)；
- 正式构建来源规则见 [M6-008 Release 溯源门禁](./docs/specs/delivery/official-release-provenance-gate.md)；
- 单应用与组合交付验收命令见 [验证入口](./docs/testing.md)。

## 文档约定

1. `docs/decisions.md` 用稳定编号集中保存已经接受、后续开发必须遵循的约束，是唯一强制决策索引。
2. `docs/foundation.md` 保存当前共识的完整自然语言解释；架构、计划和测试文档分别把决策投影为实现边界、推进顺序和证据入口。
3. `docs/discussions/` 按轮次保留问题、推导、反例和开放事项，避免只保留结论而丢失原因，但不要求开发时逐份重读。
4. 新讨论形成共识时，必须在同一变更中更新决策总表及受影响的投影文档；修正旧结论时保留旧 ID 和来源并显式标为被取代，不静默覆盖。
5. 讨论结论不自动等于客户需求、实现承诺或当前代码事实；目标是否完成仍以代码、进度和实际验证证据为准。
