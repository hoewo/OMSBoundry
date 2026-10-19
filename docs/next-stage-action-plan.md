# 下一阶段行动方案：M7 统一入口与 IAM 最小闭环

2026-10-07 BF-121 当前优先：统计完整版 UI／交互和原 OA 属性对齐的四层权限展示，项目详情问题延后。当前候选需先通过浏览器筛选／明细／导出与权限草稿矩阵，再用真实 IAM 和同一项目来源验证；正式权限配置应用和临时授权写端口不由前端冒充。整体 R0／C1～C8 和真实环境门禁继续开放。[本轮规格](./specs/assembly/oa-statistics-permission-ui-v1.md)。

2026-10-06 BF-120 当前行动取代下方历史“下一切片”：立即执行 R0 基线收敛，盘点未提交／未跟踪的应用、契约、migration、装配、验收和报告，并输出 C1～C8 缺口矩阵。之后只补 C1～C6 真缺口，完成 C7 Bonus 的 HTTP／IAM、真实来源、页面、D1／恢复和 Money 建议，再进入 C8、组合／Overlay、真实环境与最终报告。BF-112～BF-118 的旧后续顺序保留为历史，不再机械续跑。详见[当前基线与下一步](./status/current-baseline-and-next-actions-20261006.md)。

2026-10-06 BF-119 持续门禁：后续 F01／F02、S01～S03 及剩余 OA 切片除了业务闭环，还必须比较边界是否收敛。每批记录改变 Owner 数、公开契约／migration、强制依赖／同步调用链、旧场景回归和 Base／Overlay 归属；优先复用已有 Owner 与直接版本化端口。当前变化较多属验证期正常现象，但若连续切片仍要求无关系统同改、共享事实、核心 fork 或依赖图持续扩散，先重做边界，不继续铺 UI。完整 OA 仍未完成，Project、Production、Contract 整体仍为候选。

2026-10-06 BF-118 后续顺序：对外委托主路径已通过私有 10／10 和八事实 Owner 恢复，停止继续扩该页面。现在进入资料管理／统计：① 先按 F01／F02 只读拆解原项目资料树、展开时读取、下载和下载人员明细，核对“签发地址”不等于“字节已下载”；② 复用现有来源材料 reference 与 Project Resource Link，禁止把文件复制进 Project 或 Workbench；③ 再按 S01～S03 拆解台账筛选／明细／Excel 导出、公司两链统计和人员奖金统计，金额分别由 Contract／Money／Billing／Bonus 的版本化只读投影提供；④ 以正式 IAM、无权／空数据／来源故障分离、缓存上界、同一项目来源一致性和恢复关闭。对外委托的 Money 拒绝／实付、跨 Tenant／全角色与逐依赖停机保留为扩展矩阵，不用它阻塞下一主切片。

2026-10-06 BF-117 后续顺序：Results 批复／审计已完成私有 11／11和六所有者恢复，停止继续扩该页面。立即进入原 OA 对外委托：① 只读拆解 `ExternalCommissionList／Form／Summary` 的受托方、联系人、内容、金额、合同文件、评分、支付和统计；② 对照 Cost & Outsourcing 已有 Engagement／Assessment／Payable Suggestion，Party／Contract／Evidence／Money 的精确端口；③ 直接复用原 UI，禁止原单体“删旧支付再重建”及长期本地 metadata；④ 以委托登记→合同／文件→评分→应付建议→Money 独立确认、部分成功续办、撤权、恢复和请求上界关闭。之后进入资料统计；Project Completion Checkpoint 仍单列补证。

2026-10-06 BF-116 后续顺序：四类型生产总览／进度已按 Production 单一只读投影通过私有 10／10、请求上界、撤权、PG18 和五所有者恢复；不再继续改这条生产读链，也不把它扩大成通用报表平台。现在进入 Results 批复／审计：① 只读拆解原 OA 批复、审计、纠正／撤回和来源冻结；② 对照独立 Approval & Audit Results 已有 Capability／状态机／Project Completion Checkpoint；③ 直接复用原组件，只做端口、权限和操作适配；④ 以正式 IAM、真实来源、失败续办、撤权、恢复和零第二真相关闭。之后依次为对外委托、资料统计；Project 类型重分类仍是独立开放项。

2026-10-06 BF-115 后续顺序：勘察试验与多类型报告已形成 v6 私有候选，现停止继续增加“文件页签”。立即进入原 OA 四类型生产总览／进度：① 从原系统逐项记录咨询／设计／勘察／测绘的适用阶段和完成口径；② 在 Production 增加只读、版本化投影，复用现有 Work Item／Stage／资料结束事实；③ Workbench 复用原总览 UI，仅连接端口和缓存，不保存第二份状态；④ 以正式 IAM、真实 PG18／Evidence、撤权／故障／恢复和请求数上界验收。之后才进入 Results 批复／审计、外委和资料统计。Project 类型重分类的业务动作先记开放项，不在本批静默刷新既有 v6 快照。

2026-10-06 BF-114：变更／竣工已从页面壳升级为私有真实 Production→Evidence 闭环；v5 只在首次写入由 v4 显式升级，原双列表、不同扩展名、无校审／评分、不可变替换、11 场景和五所有者恢复通过。验收抓到并修复数据库约束与应用注册表两项兼容闭包缺陷。Project 无变化，Contract 不依赖，Main／公网／正式 D1 和完整 OA 未通过。下一 OA 切片转到“试验／报告”：先只读核对 `SurveyTestStagePanel`、`ConsultationReportPanel`、项目类型适用性、文件和完成映射，再选择有界 definition／端口；禁止统一套用设计阶段模型。[报告](./reports/oa-change-completion-native-validation-20261006.md)和[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)为比较基线。

2026-10-06 BF-113：施工图已从可见页面壳升级为私有真实 Production→Evidence 闭环；v4 只在首次真实施工图保存时由 v3 显式升级，前序初设保持，10 场景及五所有者恢复通过。阶段资料集合子边界达到同类可重复，但 Main／公网／正式 D1、全角色和完整 OA 未通过。下一 OA 切片转到结构不同的“变更／竣工”：先只读核对 `ChangeFileList`、`CompletionFileList`、`AuxiliaryFilePanel`、两套上传入口、文件类型和完成映射，再决定是否扩现有定义或建立新端口；禁止套施工图校审／评分模型。[施工图报告](./reports/oa-construction-stage-native-validation-20261006.md)和[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)为比较基线。

2026-10-06 BF-112：初步设计已从可见页面壳升级为私有真实 Production→Evidence 闭环；v2 仅方案、v3 方案＋初设，旧 v2 第一次真实写入才显式 2→3。9 场景及五所有者逻辑恢复通过，Main／公网／正式 D1 和完整 OA 未通过。下一 OA 切片转到“施工图设计”：先按原组件、字段和完成规则拆解，再做 Owner／版本／失败续办／恢复；禁止把初设端口无差别复制。每批继续回写[边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)。

2026-10-05 BF-110：初步设计／施工图设计／变更竣工的 Tab 禁用已修复并公网验收，导航不再被误当权限或业务完成度。下一步不再修改 Tab 壳，而是按原 OA 逐阶段建立 Production 自有版本化资料／状态端口、Evidence 引用、IAM 能力、丢回复续办和恢复；在这些完成前页面继续明确“未读取”且写按钮关闭，不复用方案事实。[报告](./reports/oa-production-stage-navigation-validation-20261005.md)。

2026-10-05 BF-109 当前补证：管理员规则49／49与当前Suite原生Grant145／145已闭合，外委新增、奖金分配抽屉及Bonus→Project空候选200已在公网真实入口通过；入口Manifest、用户Grant、Workload Policy和目标最终授权仍分开。下一 OA 主线不再重复扩管理员权，而是补初步设计／施工图设计／变更竣工原生适配器，并在具备冻结来源后验证批复／审计业务动作；不得把disabled误判为Grant缺失或用通配权绕过。[报告](./reports/oa-admin-full-access-validation-20261005.md)。

2026-10-04 BF-106 续轮最新：原方案UI已在正常Workbench经私有真实Nginx／Gateway／正式IAM／Production／Evidence完成16个场景、零页面错误，真PG18及五所有者逻辑空目标恢复／原UI继续另有证据；[报告](./reports/oa-production-stage-native-validation-20261004.md)取代下方13:51“原生未实现”的当时状态。最终原UI-only13项与原生层分开封存，377项Node零skip，源码／回执／截图保留；所有自有进程和测试Chrome已停止，本机Docker未重启，Main零发布／零Grant。下一门槛是旧活动任务显式过渡＋真实阶段D1／keys／旧会话恢复＋精确授权／匹配发布，再在原项目续验；后续原初设／施工图等须采用明确新版本，不改已注册v2的历史含义。全站其余缺口和整体insufficient-evidence不变。

2026-10-04 13:51 UTC 资源约束：用户 8GB 机器出现内存不足，已停止本任务一次性 PG fixture 和 Docker Desktop、清理遗留测试 Chrome，线上 ECS 不动；本地重型验收暂缓，候选源码、D1 卷／备份和证据保留。原方案 UI-only 试片 13 场景通过但未接原生 Production／Evidence／IAM，也未发布。继续前先复核[减负检查点](./reports/oa-complete-ui-checkpoint-20261004.md)，本地容器／浏览器按需单批启动并收尾停止；未跑的检查不记通过，不借省内存把破坏性演练搬到 Main。

2026-10-04 最新：D01新引用→Bonus v3→正常工作台已通过20个私有真实场景，[报告](./reports/oa-professional-bonus-continuous-validation-20261004.md)取代下方“尚未接线”的早期状态。所选六库的实际 D1 联合备份／新空目标恢复、旧会话拒绝、原命令重放及原人员／奖金 UI 继续另通过14场景，[恢复限制](./reports/oa-professional-selected-d1-restore-20261004.md)不扩为全 Suite 或旧主站升级。Main D01／Bonus授权另待明确，Cost批准不挪用；下一功能是[原方案阶段多主文件／可选校审单／评分](./specs/assembly/oa-production-stage-original-operation-map.md)，先原 UI 再原生整批保存，不复用单成果槽覆盖多份文件。没有 Main 发布，不关闭 D01／奖金整行、全站、全角色或架构结论。

2026-10-04 D01 原人员原生候选已完成：六分工／目录属于 Project 默认关闭 Overlay，原 UI 接正式 IAM／真实 People／PG18，21 场景、315 项 Node及全量 Project PG／专项并发／单库恢复分层通过，[报告](./reports/oa-production-personnel-native-validation-20261004.md)。正常 Workbench／Main 未启用。下一切片为新分工版本化最小引用→Bonus 显式兼容消费→原生产工作面正常装配，再补全角色、匹配发布／Nginx→Gateway／Suite 恢复；不要回到负责人替代专业人员或只读空页。Billing／奖金及新人员 Main 精确 Grant 需各自明确，不挪用 Cost 批准；其余生产子栏目与全站待办继续保留。

2026-10-04 09:10 UTC 续轮最新：B12 原界面已在实际 Workbench→Gateway→真实 IAM→Bonus／PG18 正常登记／撤回／刷新，Project／People 受保护引用及旧事实保持通过，14 场景零页面错误。接下来必须按 BF-103 匹配 Gateway／Workbench／Suite schema／renderer 与 Bonus schema 3／profile／IAM 目录，之后经批准在原 C 精确授权和公网验收；不单独发新表单到旧入口。当前无 Main 发布或 Grant 变更；B11、专业生产、全角色及 Suite 完整恢复继续按全站清单，不以此结束目标。[实际证据](./reports/oa-bonus-manual-ui-candidate-20261004.md)。

2026-10-04 08:32 UTC 已恢复续轮：原 B12 私有 UI→独立 Bonus→PG18、单库恢复／继续写和旧 Pool／建议升级保持有限通过，[报告](./reports/oa-bonus-manual-ui-candidate-20261004.md)。未发布，不给 Main 自动加权。下一门槛是匹配 Owner schema 3／profile／目录／Workbench 的兼容闭包及明确授权后续原 C；等待可继续 B11 和专业生产，但不能用 B12 代表完整奖金、局部恢复代表 Suite。下方 07:50 暂停是上一轮历史，完整目标和全站缺口保留。

2026-10-04 07:50 UTC 四小时上限暂停交接：本续轮 Main `.20261004.6` 已上线，17 主服务健康，D1 check exit 0，私有探针／restore20 停止，完整 OA 不标完成。批准的 Cost 权限已按 C 两侧精确资源落地；Billing 仍等独立授权，不能挪用 Cost 批准。恢复后复用同一 C 已保存的事实继续开票／资金关联；授权等待不阻塞奖金类型化兼容扩展、分类型生产和其余可见工作面。不能回到只读空页或重建新项目绕过旧失败；最新[暂停检查点](./reports/oa-complete-ui-checkpoint-20261004.md)和[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)保留全部未完成项。

2026-10-04 07:36 UTC 继续施工：经营 Owner、Cost schema 3 和 Workbench `.6` 已上线；原 C 成本有限链及十二侧别／分类组合、合同／支付两链、三协议、两定案、单文件预览下载、两 Cash／冲销的九检查点及原生收尾通过，没有重建原事实。成本连续撤回弹层遮挡已实际复现、修复上线并续原记录通过。下一步 Billing Grant 独立等确认后续原 C 开票；同时补合同最终更正及三费用关联，再按原奖金手填基数／固定额／多角色行为落实明确兼容扩展并接原 UI。不绕过授权，不把比例接口冒充原奖金工作面。分类型生产／成果／外委、资料统计、全角色和 OSS 仍按[完整清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)推进。完整经营链／OA 未通过；下方“Main 未更新”是历史批次，实际失败和证据见[公网报告](./reports/oa-commercial-public-ui-validation-20261004.md)。

2026-10-04 最新：三个经营工作面候选组件、PG18／启动已有分层证据；Contract 真 IAM／Evidence／字节恢复 m、历史夹具 18→22 原子迁移 a，Billing 最新 c 真库与实际 issuer／Evidence／字节恢复 e 已通过。下一门禁仍是匹配 owner／Workbench／IAM policy 的正式兼容闭包、全角色及同项目入口，不是仅发布新表单到旧 API。继续 Cost 自有七分类／经营生产、里程及可空字段语义后复用原成本组件，见[逐操作对照](./specs/assembly/oa-cost-original-operation-map.md)；然后奖金、四类型生产／结果／外委、资料统计和人员权限。Main 未更新、完整目标未完成，按[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)逐项关闭；以下旧下一步为历史批次。 最新构建、截图与 Main 17 服务只读收尾统一见[本轮检查点](./reports/oa-complete-ui-checkpoint-20261004.md)。

2026-10-04 当前施工：已从合同继续到支付，不再把举例板块当全部范围。先结束支付原组件实际点击、PG18／独立启动和失败续办证据，再接开票及有界资金关联、成本／奖金、四类型生产、资料统计／人员权限；候选通过后仍须兼容闭包和同项目公网旅程。Contract 真 IAM＋Evidence／字节恢复 m 已通过（private/local），最终更正和整体交付未完成。Main 未更新。当前状态以[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)及[支付报告](./reports/oa-cash-ui-candidate-20261004.md)为准，以下旧描述保留历史。

2026-10-04 当前：两链原组件、原始合同／三类协议显式确认、三类材料与阶段／最终结算已候选接线，schema 22 专项真库、应用库空目标恢复、独立启动与组件分层通过，未发布。下一步补真实 IAM＋Evidence 联合、字节恢复、全角色／冲突、最终更正与兼容闭包；随后接支付／开票及其余[全站清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)。不缩为单个招投标或合同候选；[实际证据](./reports/oa-contract-draft-ui-candidate-20261004.md)。以下旧“类型化／原组件未接线”保留当时状态。

2026-10-03 当前主线以[完整 OA 交付清单](./specs/assembly/oa-complete-operation-delivery-ledger.md)为准：人员真实增减 → 两链合同／协议／定案 → 支付／开票 → 成本／奖金 → 分类型生产／资料／统计；招投标不是唯一待办。BF-097 已在 `.3` 发布 Project schema 11 和原人员组件，同一 C 的增减及丢回复原命令恢复已有真实证据；Workbench `.6` 已上线隐藏读取修复，终场公网三项零写复验通过、旧事实保持。合同下一步按[原两链映射](./specs/assembly/oa-contract-original-operation-map.md)实现类型化费用／费率／两链、主体和材料，再接原组件；Contract JSON 端口真库及自有空库启动通过但未部署，不是合同页通过。详见[证据与限制](./reports/oa-complete-operation-progress-20261003.md)。以下 B03 唯一下一项、夜间暂停和未部署描述均属历史批次。

2026-10-03 登录阻断已在 `.20261003.2` 关闭：只更新 Workbench，过期／多标签与恢复登录通过，业务／Grant 不变；[实测记录](./reports/oa-login-csrf-repair-validation-20261003.md)。继续下方原 OA 逐操作主线，不扩展新的登录系统或重写 UI；B03 分权及其余 OA 待办不变。

2026-10-03 BF-096 施工更新：原招投标组件已局部挂入 Workbench 并随 `.20261003.1` 发布；实际公网上传、预览／下载字节、单份移除、同项目刷新与丢回复后 reload／原命令继续通过。只更新 BE／Workbench，数据边界及 IAM 不变。下一门禁是 B03 分权／撤权、更多文件和失败矩阵及必要的小文案／透镜缺口，不重写 UI；完整立项、成员、后续经营／生产和 OSS／恢复待办仍在。见[实际结果](./reports/oa-reused-bidding-ui-validation-20261003.md)。以下夜间暂停和候选未部署是历史阶段。

2026-10-03 BF-095：BE 多附件候选后端、旧库／旧命令兼容、独立恢复继续和 IAM 启动已实测通过，尚未部署；当前进入同项目 Evidence 附件 UI 接线，见[报告](./reports/oa-bidding-material-collection-validation-20261003.md)。兼容发布和真实公网文件操作仍是下一门禁，不跳 B03 或其余完整 OA 待办。

四小时上限前已暂停收尾：Workbench 材料／动态透镜候选全量 156 项 Node及独立启动通过，Main `.11` 未更新，k check exit 0、17/17 healthy，私有 PG／探针／restore20 停止。恢复后的唯一下一切片是 Evidence 版本化接线、原 OA 附件交互与未登记完成文件的显式继续；当前 Evidence local 与将来的 OSS 验证分别记账。候选发布需联合备份／恢复及迁移门禁，之后才验公网文件闭环。

2026-10-02 当前施工主线已按 BF-093 明确为完整仿 OA，而非本历史 M7 清单。小系统允许兼容演进，但职责／数据／IAM／Evidence 边界须稳定或可预见；按[完整验证计划](./oa-full-scope-validation-plan.md)和[原 OA 操作基线](./specs/assembly/oa-original-usage-workflow-baseline.md)逐项接通。B03 同项目方式／适用性及刷新已通过 `.11` 公网链；下一步是 BE 多附件兼容演进和 Evidence 附件操作，剩余完整立项、成员、合同、生产、资料统计及交付门禁不被跳过或标为通过。

日期：2026-09-21
状态：已完成；M7-001 至 M7-006 均已验证。当前下一阶段主线见 [OA 完整体架构可行性验证计划](./oa-full-scope-validation-plan.md)。

## 1. 一句话目标

> Contract 与 Money Flow 已能独立使用，但入口和账号仍彼此割裂 → 用户从一个 Nginx 入口登录一次，按同一 IAM 契约安全进入已启用的两个应用，同时两个应用继续保持独立交付能力。

这一步只把已经实现的两个业务应用接入统一入口和统一身份边界，不同时建设 People & Groups、Project、独立 Evidence Service 或完整多机部署。

## 2. 本阶段完成后的可见结果

M7 完成时，必须可以从一个全新环境演示：

```text
打开统一地址
  → 登录 IAM
  → Workbench 只显示 Suite Manifest 已启用应用
  → 进入 Contract 并完成一个受保护操作
  → 进入 Money Flow 并完成一个受保护操作
  → 关闭其中一个应用后，另一个应用和 IAM 继续工作
```

用户不能感知两个本地登录系统；运维仍能把 Contract 或 Money Flow 作为独立产品交付，并用同形本地模板身份提供者完成单应用测试。

## 3. 本阶段范围

### 必须完成

- 版本化 IAM Context、Capability Manifest、Capability Credential 和 Suite Manifest 最小契约；
- IAM 的租户、首位管理员、邀请／激活、登录、注销、密码重置、会话、密钥和审计最小闭环；
- 独立 Go Gateway 的静态版本化路由、身份 Header 清理、基础验签、限流和访问日志；
- Nginx 唯一公网入口与最小 Workbench 壳；
- Contract 先行接入 IAM，再接入 Money Flow；
- 两个应用保留独立模式，并在中心 IAM 模式下不再拥有账号密码真相；
- 当前两应用组合的 D1 Compact 安装、升级、重启、故障隔离和备份恢复验收。

### 明确不做

- People & Groups、Project Workspace 和独立 Evidence Service 的业务实现；
- Contract → Money Flow 连接器；
- D2、D3、D4 实际部署和容量定型；
- Redis、消息队列、服务注册中心、Kubernetes 或独立配置中心；
- 外部 OAuth／OIDC 身份提供者、匿名公共注册和社交登录；
- 为 Workbench 建设完整 SPA 或重写两个应用的现有业务页面；
- 客户岗位、项目成员、数据范围等尚无真实案例的复杂策略。

## 4. 执行顺序

| 顺序 | 事项 | 最小 A→B | 主要产物 | 退出条件 |
| --- | --- | --- | --- | --- |
| M7-001 | 契约与威胁边界（已验证） | 身份链关键语义未落到可测试格式 → 得到可实现、可攻击测试的 v1 契约 | IAM Context、Capability Manifest、Credential、Suite Manifest Schema；Token／Cookie／密钥／撤销／故障规则；威胁模型与 ADR | 正常、伪造、过期、越租户、错误 audience／action 示例均有确定结果；现有 Actor 历史迁移规则明确 |
| M7-002 | IAM 最小纵向闭环（已验证） | 两应用各自保存账号密码 → IAM 独占账号与会话真相 | 独立 Go 应用、独立 PostgreSQL、migration、租户与账号生命周期、会话／Token、审计；release 与恢复在 M7-006 对实际 D1 发布物统一收口 | 全新数据库已完成首位管理员、邀请、激活、登录、注销、重置；实际重启和密钥轮换规则通过测试 |
| M7-003 | Nginx、Go Gateway 与 Workbench 壳（已验证） | 多个入口且通用边界不一致 → 一个公网入口安全到达 IAM 和启用应用 | Nginx 配置、独立 Go Gateway、静态路由清单、最小 Workbench、健康检查与结构化访问日志 | Nginx 配置与真实四进程链路通过；登录经 Gateway 到 IAM；伪造身份 Header 被清除；禁用路由不可达且菜单不显示 |
| M7-004 | Contract IAM 试点迁移（已验证） | Contract 只认识本地账号 → 同一业务能力既能由正式 IAM 也能由本地模板提供 | Contract Capability Manifest、IAM Adapter、历史 Actor 映射与迁移、`allowed_actions` 与服务端授权 | 中心模式单点登录可完成代表性合同读写；独立模式仍可运行；旧审计可解释；Money Flow 停机不影响验收 |
| M7-005 | Money Flow IAM 迁移（已验证） | Money Flow 仍有独立登录边界 → 两个应用共享契约而不共享身份表 | Money Capability Manifest、IAM Adapter、历史 Actor 映射与迁移 | 同一登录会话可依次访问两个应用；Contract 停机时 Money Flow 继续工作；双方没有跨库身份查询 |
| M7-006 | D1 组合交付验收（已验证） | 各组件分别可运行但组合交付未证明 → 当前两应用形成可重复安装和恢复的统一入口候选包 | Suite Manifest、D1 Compose、安装升级手册、联合验收程序、离机备份和恢复步骤 | 全新环境安装、单次登录双应用、应用开关、单侧故障、重启、升级前备份与整机恢复全部通过 |

M7 已按 M7-001 → M7-006 顺序完成。后续进入 M8 时仍不得回退这些入口、身份、独立交付和恢复边界。

## 5. M7-001 立即行动清单

M7-001 已按以下清单完成并由 `acceptance/access-contract` 自动验证；这些结果是 M7-002 的输入，不再把协议决策留给实现阶段：

1. 盘点 Contract 与 Money Flow 当前用户、角色、Session、Cookie、CSRF、Actor 和审计实现；
2. 从两个应用已经存在的服务端动作提取稳定 Capability，不使用菜单、按钮或客户岗位命名；
3. 定义 IAM Context v1 和 Capability Credential v1 的 issuer、tenant、subject、audience、action、scope、签发时间、过期时间、key ID 与唯一标识；
4. 定义 Capability Manifest v1 的版本、兼容、弃用和未知能力处理；
5. 定义 Suite Manifest v1 的应用 ID、版本、安装／启用状态、路由、健康端点和依赖校验；
6. 完成登录 Cookie／Token、CSRF、Gateway 基础验签、目标应用复验、密钥保存／轮换、撤销和 IAM 不可用行为的威胁模型；
7. 明确现有本地账号迁移到 IAM 后，密码、Subject 映射、历史 Actor、旧 Session 和回退分别如何处理；
8. 用契约样例和负例测试固定结论，再允许 M7-002 建立代码目录。

实现入口：[`contracts/access/v1/`](../contracts/access/v1/README.md)。规格与证据：

- [IAM 与 Capability 契约 v1](./specs/identity/access-contract-v1.md)；
- [统一入口与 IAM 威胁模型 v1](./specs/identity/access-threat-model-v1.md)；
- [现有本地身份实现审计](./specs/identity/current-local-identity-audit.md)；
- [ADR-0033：IAM Session 与 Ed25519 单 Audience 短期凭证](./adr/0033-access-credentials-use-ed25519-audience-tokens.md)。

M7-001 不要求先完成 People、Project 或复杂业务范围策略。第一版只需证明租户级和明确资源级能力能够安全到达目标应用，并为未来 Business Fact Snapshot 保留版本化扩展位置。

## 6. M7-006 D1 组合交付结果

当前 D1 交付位于 `deployments/d1/`，已形成 Compose 编排、环境模板、Suite Manifest 渲染、首管授权、安装／开关／检查／升级／备份／恢复命令和联合运行验收。实际空环境、单次登录双应用、单侧故障、应用开关、销毁卷后的整机恢复及恢复后继续写入均已通过。

详细实现和证据见 [M7-006 D1 组合交付验收](./specs/delivery/d1-combined-delivery-m7-006.md)，操作者入口见 [D1 Compact 运行手册](./operations/d1-compact.md)。M7 当时只完成 IAM＋Workbench＋Contract＋Money 组合的 D1 候选包；此后 People & Groups、Evidence Service、Project Workspace 和 Business Engagement 已装配进 D1。Project 的显式 Resource Scope、Assignment Business Fact Policy、单项撤权、Project→People 工作负载凭证生命周期，以及 C1 项目责任、两级项目包、父子转换／安全撤销、客户页面、`allowed_actions` 刷新和撤权恢复，已通过隔离 D1 的身份管理员反向隔离、九角色／多 Project／状态矩阵。C2 Business Engagement 又完成 PostgreSQL 18 真库、精确 Project／File 工作负载凭证、镜像／release／Suite Manifest、Workbench 工作区、服务端 `allowed_actions` 和实际浏览器撤权刷新；95 条 Nginx→Gateway→IAM／Policy 正式入口断言覆盖 409 无写入、Evidence 503 停机／恢复和新命令单次写入，并已由真实浏览器复核恢复交互、Project 返回和撤权退出。扩展到 110 条后又覆盖账号停用、IAM 502、Policy 事实源 503、失败不签发、恢复后单次签发和目标错租户拒绝。联合恢复、IAM 会话归零、原业务引用保持和恢复后继续写入也已通过。`ready` 只表示材料齐备，不冒充中标或签约。下一业务切片从完整双租户 D1 矩阵或下一个证据充分的业务缺口重新选择。项目包合同金额分配继续等待业务证据，不以猜测阻塞 C2。Design、Contract 遗留文件迁移以及 D2～D4 仍按后续里程碑推进。

## 7. 不得破坏的架构边界

2026-09-27 补充：启用 Business Engagement 的隔离 D1 已完成联合备份、销毁专属卷后的空目标恢复、IAM 会话归零、原业务引用保持和恢复后继续写入；409／503 自动化故障恢复、页面恢复契约及真实浏览器受控点击随后通过。账号停用、IAM／Policy 故障、失败不签发、恢复后单次签发和目标错租户拒绝也已通过。下一业务切片不再重复证明这些本地边界，只保留完整双租户 D1 矩阵并按 OA 业务证据选择下一闭环。

同日 C3 Billing 也完成 Linux AMD64 非 root 发布、Suite Manifest、正式 Nginx→Gateway→IAM 入口、精确工作负载凭证、Party 故障零写入／恢复单次提交和联合空目标还原。Billing 人类工作区随后进入 Workbench，并通过服务端动作收敛、金额撤权／恢复、历史脱敏、盲写金额拒绝、409 和依赖故障恢复。Project 的 Contract／Obligation／Transaction／Invoice Registration 分段只读组合现已从同一正式入口通过 100 个断言、三角色、撤权／停用、Contract／Money／Billing／IAM 故障恢复和 13/220 ms p50/p95。下一业务切片转向 C4；C3 的真实浏览器最终点击、双 Tenant、Billing-only 和真实 Ubuntu 保持独立补证轴。

2026-09-28 C4 已把下一业务切片固定为单一 Production Operations 数据所有者的兼容演进：保留 `design-v1` 既有事实与旧入口，新增受控 `consultation-v1`，同项目双类型各自维护 Work Item／frontier／revision／历史。C4-A01～A14 当前本地轮次已通过；除最小组合和 Level A／B 回归外，People＋Project＋Evidence＋Production 的隔离 D1 已补齐 Suite Manifest／Workbench、完整未选缺席、正式入口主链、IAM／Evidence 故障恢复和联合空目标恢复。同日 86 条权限生命周期断言与真实浏览器又补齐 Grant 撤销／恢复、账号停用／启用、Assignment 结束与页面动作收敛。下一轮保留双 Tenant 和真实 Ubuntu 为 C4 环境补证，主业务切片可按证据进入 C5。当前 OA 整体仍标记 `insufficient-evidence`，不得把本地 D1 结果外推为阶段完成。

同日 C5 已完成五轮实现收缩：独立 Approval & Audit Results 领域核心与 PostgreSQL 18 生命周期；版本化 HTTP、目标 IAM、金额脱敏及 Project／Contract／Evidence 精确 adapters；Results 人类工作区与 Project `reference-oa-c5-v1` 完成检查点；正式 D1 的独立数据库／工作负载身份、Suite Manifest、Workbench、入口、未选缺席和联合空目标恢复；以及 74 断言真实独立进程的 Contract／Evidence→approval→audit→Project 确认、权限撤销／恢复和局部故障恢复。C6 随后完成四轮收缩：责任边界和双聚合核心；受保护 HTTP／正式 IAM／精确引用；人类工作区和独立进程最小组合；以及正式 D1 的独立镜像、数据库、migration、Workload Key、Suite Manifest、Workbench、入口、未选缺席和联合空目标恢复。57 个正式入口断言验证 Cost、Engagement、Assessment、Project Scope、金额脱敏、Evidence／Party 停机零写入、恢复、精确 workload audit 和成功重放。旧 Contract 本地主体与 Directory Party ID 不一致时明确 422，说明 Contract／Money Party 运行时切换是下一前置缺口；之后再完成 C6 Payable Suggestion，随后进入 C7～C8。C5 完整双 Tenant、账号停用、真实浏览器和真实 Ubuntu 继续作为独立证据轴，不宣称完整 OA Project 已完成。

2026-09-28 后续补证：Contract／Money 已实现来源自有、追加不可变的 Party 运行时转换账；两侧含引用数据真库用例通过切换、只读回滚、本地写权冻结和防篡改。C6 隔离 D1 的正式入口断言增至 62 个，Contract-linked Engagement 在切换前／回滚后 422，在 Directory 模式下 201；`cutover 1 → rollback 2 → recutover 3`、联合备份 `20260928T052302Z`、空目标恢复和重新选装均通过。该前置缺口已关闭，下一主业务切片转为 C6 Payable Suggestion；完整双 Tenant、Money 完整 D1 消费链、C7～C8 和真实 Ubuntu 保持独立待办。

2026-09-28 C6 应付建议补证：C6 与 Money 已以两个独立进程／数据库完成版本化 Payable Suggestion 的接受与拒绝。接受生成且仅生成一个 Money 自有 Obligation，拒绝生成零个；Money 停机时 C6 保留目标未确认事实，恢复后显式重试，决定／同步重放和“义务已提交、intake 未更新”的部分失败均可恢复。该轮关闭 C6-A10 当前双进程范围，下一主业务切片须 fresh selection 后进入 C7；C6 connector 的完整正式 D1、双 Tenant、跨进程撤回、Money／Billing 只读投影和真实 Ubuntu继续作为独立补证。

2026-09-28 C7 领域核心更新：参考 OA 的奖金事实存在三个互相冲突的实现，不能直接复制。BF-066 已接受独立 Bonus Distribution、经营／生产类型化来源、精确来源与 Person／Assignment 快照、版本化 `reference-oa-bonus-v1` Overlay、整数金额和追加式发布历史；BF-067 又完成独立 Go 领域核心和 PostgreSQL 18 事实层，验证精确 half-up、发布／取代／撤回、失败零写入、成功重放、并发幂等、租户／Project 隔离和历史防篡改。Money 仍须独立确认应付建议。C7-A01～A09 已在核心层通过，A11／A12／A18 部分通过；下一轮从受保护 HTTP、正式 IAM、真实 adapters 与金额脱敏收缩，不把页面、D1、C8 报表、税务／薪酬或自动付款混入。

2026-09-28 C7 受保护后端更新：BF-068 已实现版本化 HTTP、`bonus-distribution` 正式 IAM、Capability Manifest、经营／生产 Assignment Policy、请求绑定 Project／People／Contract／Money adapters 和 current／history 统一金额脱敏。真实 PostgreSQL 18 加严格 HTTP 来源替身通过依赖失败零写入、恢复单次提交、成功重放零上游、一次 401 换票及 404／409／422／503；错 audience／tenant／Project／相邻角色／资源失败关闭。下一轮先实现 People Person 与 Money Receipt Set 的真实来源端点并完成独立进程组合，再进入人类工作区；D1、Payable Suggestion 和 C8 不混入该轮。

2026-09-28 C7 真实来源组合更新：BF-069 已实现 People Person 精确引用和 Money 不可变 Receipt Set；IAM、Project、People、Contract、Money、Bonus 的六进程六数据库组合已从真实 Settlement／Receipt Set、Person 与 Assignment 完成两类奖金发布。下一唯一业务缺口切换为 C7 人类工作区及直接正常／纠错／失败证据。正式 D1／未选装、联合恢复、C7→Money Payable Suggestion 与 C8 继续保持后续独立轮次，不能因最小组合通过而提前宣称完整 C7 或完整 OA。

2026-09-28 C7 人类工作区更新：BF-070 已完成 Project 范围页面、经营／生产草稿、发布、取代、撤回、历史、金额权限与 401／403／409／422／503 恢复；六进程夹具和真实浏览器证明 3 条 `published／superseded／withdrawn` 状态、精确来源快照、Contract 停机零新增事实与恢复。下一轮从 C7 正式 D1／未选装／联合恢复或 C7→Money Payable Suggestion fresh selection；C8、双 Tenant 与真实 Ubuntu 继续保持独立待办。

2026-09-28 C7 正式 D1 更新：BF-071 已把 Bonus Distribution 装入独立镜像、数据库、migration、工作负载身份、Suite Manifest、Workbench、正式入口和联合备份恢复。16 条入口断言、5 条精确 workload audit、备份 `20260928T143704Z`、空目标恢复、恢复后 revision 2→3 继续写入及未选／重新选装均通过。下一轮须 fresh selection 后在 C7→Money Payable Suggestion 与 C8 中选择；经营／生产单类型最小组合、双 Tenant 与真实 Ubuntu 保持独立补证。

2026-09-29 C7→Money 更新：BF-072 已完成 Payable Suggestion 的直接跨进程收缩。Bonus 与 Money 使用独立数据库，以 `bonus-distribution/bonus_allocation` 和 exact Person revision／digest 协作；Money 通过人工审核、不可变的 Person→individual Party 映射解析自身财务主体后独立接受或拒绝。一个 PostgreSQL 容器、两个应用进程和 11 条 HTTP 断言覆盖目标停机恢复、接受唯一义务、拒绝零义务、幂等、映射防篡改和源撤回无级联，退出后无运行容器。下一主业务轮转向 C8；正式 D1 connector／建议引用联合恢复、单类型最小组合、双 Tenant 和真实 Ubuntu 继续作为独立补证，不能由本轮结果代替。

2026-09-29 C8 边界更新：BF-073 已确认 C8 不新增业务事实所有者。执行按低资源批次推进：先静态／单元验证 Evidence `metadata.read` 与 `download` 的兼容分权，再实现 Project 范围直接 Evidence Resource Link 的资料组合；每批最多运行一个目标数据库或一组最小本地进程并立即清理。指标与个人工作随后各只接入一个来源，验证来源口径、局部失败和权限后再扩展。当前没有运行 C8，不得把边界审计记为功能通过。

2026-09-29 C8 首个后端切片更新：Evidence 已兼容拆分 `metadata.read`／`download`，Project 已增加受独立能力与精确 Project Scope 保护的 direct Evidence 资料组合，并保留 revision、digest、source status 和逐段失败。Evidence、Project 两个 module 以 `GOMAXPROCS=2`、`go test -p 1 ./...` 串行通过，定向 Access Contract 通过，全程 Docker 容器为 0。下一批只启动一个临时 PostgreSQL 和 Project／Evidence 最小进程做真实 IAM 换票、停机／恢复与零残留；页面、来源 Evidence 引用、WorkItem 和统计继续分轮。

2026-09-29 C8 最小运行组合更新：上述下一批已完成。脚本 `acceptance/c8-material-composition/run-isolated` 在 Docker 初始为 0 时只启动一个 PostgreSQL 18.6 容器，IAM、Evidence、Project 使用本地独立进程；Project workload 通过真实 IAM 取得 `metadata.read + file:{id}`，无 download 权。direct Link、stale revision、相邻 Project 403、Evidence 停机局部分段 unavailable、恢复 available 和 `0 → 1 → 0` 容器清理均通过。下一批不启动完整 D1，应在资料人类页面、一个业务来源的间接 Evidence 引用和一个 WorkItem 来源中只选一项。

2026-09-29 C8 首个间接资料源更新：已选择 Business Engagement 并完成零容器单元／契约验证。来源应用发布 current-only `MaterialReferenceProjection v1`；Project 使用精确来源凭证读取后，再逐 File 使用 Evidence metadata 凭证组合，保留来源与 Evidence 两层 revision／digest 和局部失败。下一批只在该链的单容器真实运行、一个 WorkItem 来源或已解锁后的资料页真实浏览器中选择一项；不得同时启动完整 D1 或多组应用。

2026-09-29 C8 资料页面首批更新：Project 记录页已新增按 `allowed_actions` 收敛的“项目资料”，资源连接可建立 Evidence File Link，页面显示文件元数据、冻结／当前 revision、摘要与逐段失败；Suite 下载入口只在取得 Evidence 独立授权后进入深链。Project 串行测试、嵌入 JavaScript 语法和 8 个交互状态结构通过。浏览器保持模式重跑后端仍遵守单 PostgreSQL 容器并清回 0；真实点击因 macOS 锁屏未完成，不得记为浏览器通过。下一批优先只补该点击；若环境仍不可交互，再从间接 Evidence 或单一 WorkItem 来源中选一项。

2026-09-29 C8 首个个人工作来源更新：Production Operations 已发布 current-only `WorkItemProjection v1`，稳定 Work Item ID、revision、Project Scope、深链和 `allowed_actions` 全部由来源拥有，只返回当前 Credential Subject 的 active direct Person Owner 未完成工作项。Suite Manifest 以可选声明发现来源；Workbench 浏览器通过 Gateway 换取人员短凭证并以 path-scoped HttpOnly Cookie 读取，不接收 Subject 输入、不读取令牌、不使用 workload 身份冒充人员，也不写来源状态。Production、Workbench、Gateway 与 Access Contract 已按 `GOMAXPROCS=2`、`go test -p 1` 串行通过，嵌入脚本语法通过，全程 Docker 为 0。下一批只能从工作项正式 Gateway／浏览器链、一个指标来源或 Business Engagement 间接资料单容器运行链中选一项，不能并行启动完整 D1。

2026-09-29 C8 首个指标来源及消费者更新：Money Flow 已完成 `MetricProjection v1` 来源级实现；Suite Manifest 与 Workbench 又完成单消费者装入。六个 current Operating Summary 指标保持 Tenant／Ledger Owner Party／Currency Scope、精确十进制、口径版本、观测时间、摘要和深链；`statistics.read` 与 `amount.read` 分权，无金额权时值与摘要同时隐藏。Workbench 严格校验来源／Tenant／Scope，按来源分区且不跨来源合计，来源失败只降级自身。Money、Workbench、Gateway Manifest 与 Access Contract 均低资源串行通过，Docker 为 0。下一批优先补真实 Gateway→IAM→Money／浏览器证据，或处理无需整栈的 Project 资料浏览器缺口；不并行启动完整 D1。

2026-09-30 连续完整 Suite 更新：公司 Ubuntu 隔离恢复栈已按 `acceptance/d1-full-oa-journey` 连续通过 C0～C3。C2 沿用同一 Tenant／Project／客户 Party，新增供应商 Party、两份 Evidence、Contract／Settlement、ready Business Engagement 与两条 Project Resource Link；C3 又显式迁移并切换 Money Party Directory 权威，新增 Billing 追加纠正、Money 应收／回款／核销／Receipt Set 与三条 Project Resource Link，Project revision 7→10。Evidence 与 Billing 分别只做一次单来源故障，均局部降级并恢复；C3 40 条 HTTP 断言零差异、金额脱敏为 null 而非零、所有权增量精确且结束 17/17 healthy。完成态重跑保持相同 ID、revision 与增量；完整旅程 verifier 已进入 `test.34` release bundle。下一批只从 `state-after-c3.json` 进入 C4；不得创建新 Project、并行执行 C5 或同时做浏览器／备份压力验证。

同日连续 C4 已从该 `state-after-c3.json` 在相同 Tenant／Project 上通过。`design-v1` 与 `consultation-v1` 各形成 revision 8，两个 Evidence File 保存三个不可变 Version；错误类型阶段 422、stale revision 409、Evidence 503 零写入、同 Command 恢复单次提交、报告替换和返工均通过。Project／People 零新增事实，Production／Evidence 增量精确，完成态重跑保持原 ID／revision，隔离栈仍为 17/17 healthy。连续门禁现为 C0～C4（5/9）；下一批只从 `state-after-c4.json` 进入 C5，不新建 Project，也不并行执行 C6 或浏览器／备份压力验证。

同日连续 C5 已从该 `state-after-c4.json` 在相同 Tenant／Project 上通过。C1 Actor、C2 Contract／award Evidence 直接复用；Results 登记批复 revision 1 与审计 revision 1，Project 显式确认后 revision 10→11，再追加批复 correction revision 2，审计仍冻结 revision 1 且 Project 显示 source drift。Assessment GET 零写入、金额脱敏、Grant 撤销 403、Evidence／Contract 故障零写入同命令恢复、Results 故障旧命令本地回放／新命令失败关闭均通过。首轮 46 条、完成态重跑 14 条 HTTP 断言零差异，所有权增量精确，隔离栈仍为 17/17 healthy。连续门禁现为 C0～C5（6/9）；下一批只从 `state-after-c5.json` 进入 C6，不新建 Project，也不并行执行 C7 或浏览器／备份压力验证。

同日连续 C6 已从该 `state-after-c5.json` 在相同 Tenant／Project 上通过。C1 Actor、C2 供应商 Party／Contract 与既有失败建议直接复用；Cost 到 revision 2，新增一个 Outsourcing Engagement、Assessment、Payable Suggestion revision 3，并由 Money 独立接受为一个 Obligation。Money 停机时建议保持 `target_unavailable`，恢复后同一命令单次完成；首轮 24 条、完成态重跑 15 条 HTTP 断言零差异，所有权增量精确，隔离栈仍为 17/17 healthy。验证中发现并修复 Cost／Bonus 的 Money workload action namespace 与 Money exact suggestion resource scope，不以放宽授权绕过。`test.38` 已固定 17 个镜像摘要、C0～C6 verifier 与可校验 release bundle。连续门禁现为 C0～C6（7/9，约 78%）；下一批只从 `state-after-c6.json` 进入 C7，不新建 Project，也不并行执行 C8 或浏览器／备份压力验证。

- Nginx 是唯一公网入口；Gateway 只监听内网，不取代 Nginx；
- Gateway 不保存账号、密码、验证码、角色授权或业务事实，也不签发最终权限结论；
- IAM 不转发每个业务请求，不直接读取 Contract 或 Money Flow 内部表；
- 目标应用复验凭证并结合本地资源状态、revision 和领域不变量作最终授权；
- 两个业务应用继续拥有独立数据库、migration、release 和备份恢复边界；
- 正式 IAM、本地模板和测试提供者使用同一契约，不能增加 `AUTH_DISABLED`；
- Suite Manifest 关闭应用只影响路由、菜单、进程和连接器，不删除业务数据；
- 所有端点由配置注入，为 D2～D4 保留拆分能力，不在代码中固化 `localhost`；
- 历史 Actor 与审计必须保持可解释，迁移不能把旧操作改写成新 IAM 用户行为。

## 8. 阶段验收与停止条件

### M7 总退出门槛

- 一个用户通过统一入口登录一次，可以按授权访问两个已启用应用；
- 伪造 Header、伪造签名、过期凭证、错误 audience、错误 action、越租户和越 scope 请求均被服务端拒绝；
- Gateway 通过基础验签不代表业务命令必然成功，目标应用仍执行自身授权和业务不变量；
- 停止 Contract、Money Flow 或 Workbench 任一组件时，其余组件的实际影响符合职责边界，不产生虚假成功或跨库写入；
- 两个应用仍能脱离完整入口以本地模板 Provider 独立构建、启动和测试；
- 一个 PostgreSQL 实例中的 IAM、Contract、Money 数据库、账号、migration 和恢复边界互不越界；
- 全新 D1 环境安装、升级、备份和恢复可由实际发布物复演；
- 没有为了完成 M7 引入 People、Project、Evidence、连接器或客户专用权限逻辑。

### 必须暂停实现的情况

- IAM Context 或 Credential 无法明确区分 issuer、tenant、audience、action 和 scope；
- 现有 Actor 历史无法在不改写审计的情况下迁移；
- 登录 Cookie／Token、CSRF、密钥轮换或撤销仍存在未定的高风险路径；
- Gateway 开始拥有用户、策略或业务数据；
- 为统一入口必须破坏任一应用的独立启动和交付能力。

出现这些情况时回到 M7-001 修正规格或提交新的架构决策，不用临时代码绕过。

## 9. 进度汇报方式

下一阶段只按以下状态汇报，不使用模糊百分比：

- `未开始`：尚无规格或代码证据；
- `契约已确认`：规格、威胁模型和负例已经通过评审；
- `实现中`：已进入纵向代码闭环但退出条件未全部通过；
- `已验证`：实际 PostgreSQL、实际发布物和目标验收均通过；
- `受阻`：缺少必须由人确认的业务或安全决策，并已写明阻塞事实。

M7 与 M8 已完成。用户确认的“先组件、后应用”原则已经落实：M10-001 先交付业务无关 File Transfer Core 与 Evidence v1 契约，M10-002 再消费该契约建立独立 Evidence 服务进程、数据库与存储 Provider，M10-003 完成 D1 入口、开关和联合恢复装配；2026-09-22 又完成真实 OSS 核心浏览器链验收。下一业务应用里程碑仍是 M9 Project；Evidence 的 Contract 迁移、云端剩余负例和生产入口继续作为独立后续切片，M11 Complete Suite 保持在这些必要工作之后。

## 2026-10-02 优先级修正：原 OA 怎么用

当前检查点（BF-090／BF-091）：Auth58 `.2` 已修复旧待立项恢复阻断；同类非真实夹具已在公网明确补齐经营／更换生产负责人、完成同一 Project、进入同 OA 详情并刷新复读，旧历史保持，见[修复报告](./reports/oa-initiation-recovery-repair-validation-20261002.md)。用户“测试项目101”未代选或提交，其 revision 6 及原分工／历史不变。继续补[原 OA 逐步执行清单](./specs/assembly/oa-original-usage-workflow-baseline.md)的 I01～I04 其余字段和完整权限／失败矩阵，再推进 B02 等经营、生产、资料统计；本次故障关闭不等于整个立项功能或完整 OA 通过。原站现场未验，源码拆解不能记成真人全站通过。

此前专用新夹具的公网 Chrome／IAM／Gateway／PG 正常路径以及联合备份／空目标数据恢复、迁移、health 保留有限证据，见[修正后的部署报告](./reports/auth58-oa-first-slice-deployment-validation-20261002.md)。下方“Main 尚未更新”是候选批次历史部署状态；C8、OSS、完整恢复运行和正式交付仍未关闭。

BF-089 已候选补历史甲方名称的无下拉／Tab 交互，范围只来自既有资料读取权及可信 Project Scope；不扩权、自动绑 Party 或建跨库查询。真库 `r` 112／85 条含子测试 PASS、独立启动 `s`、Node 32 项与合成浏览器分层通过，见[报告](./reports/oa-client-name-clone-repair-validation-20261002.md)。B02 已完成[原源码操作／权限对照](./specs/assembly/oa-business-personnel-clone-gap.md)：固定经营参与、520px 只选人、与资料编辑分权、名单含离职、父包经营授权传子项；均不能将通用 participant 冒充经营人员。其类型化事实／权限兼容规格和成员操作现在排在立项阻断关闭之后；Main 发布仍需既有兼容与恢复门禁。

BF-088 本批补 B01 甲方行内录入：Project 专属联系资料与 Party 身份隔离，保持原操作不跳系统；候选真库、单库恢复、独立启动及合成浏览器分层结果见[报告](./reports/oa-client-info-clone-repair-validation-20261002.md)。历史名称补全由后续 BF-089 取得候选证据；仍需原站经营参与人权限及经营其他动作，不能把一张可保存表单当成整条经营链验收。

BF-087 现补同一基本信息抽屉内的两类负责人原子更换／清除，保持分权与旧分工历史，候选分层证据见[基本信息报告](./reports/oa-basic-info-clone-repair-validation-20261002.md)。下一切片核对其余 P02 字段及候选人员规则、P01 进度投影，再逐原站经营／生产操作接通；不是再设计一个用户流程。Main 发布仍先过兼容闭包、备份／空目标恢复与正式入口门禁。

本轮用户已明确“修复仿 OA，目标完全仿制”（BF-084），进入逐动作实施；不改代码／不部署的限制仅描述上一轮源码审计。首条派发、立项后补、统一项目详情和抽屉候选已实现，代码、隔离真库、独立启动及合成浏览器分别通过，见 [报告](./reports/oa-initiation-clone-repair-validation-20261002.md)。Main 尚未更新；完整功能映射、布局和业务结果均在目标范围，原站占位／冲突不作假完成。

BF-085 的已立项核心资料编辑现已完成候选及分层验证，修复抽屉保存被透镜遮挡的问题，见[本轮报告](./reports/oa-profile-clone-repair-validation-20261002.md)。BF-086 又补 P01 年份／分页／搜索、包语境、返回／刷新状态和真实滚动容器，见[目录报告](./reports/oa-directory-clone-repair-validation-20261002.md)；不将合成点击算作 P01 全项／公网通过。下一切片补 P02 两类负责人调整和其余字段、P01 经营／生产进度投影，再逐原站业务操作推进。新授权版本需显式部署选择，不能静默改当前 IAM。

本节修正下方第 10、11 节的“只关 C8 两项／先创建再配齐两负责人”顺序，不删除历史工程证据。此前只读梳理已保存[使用流程源码基线](./specs/assembly/oa-original-usage-workflow-baseline.md)，原站真人复核与克隆全量差异账仍待完成。实施轮不启动本地 Docker、不复活 restore20，不越过兼容闭包直接更新 Auth58；逐切片真实 API 与浏览器验证继续进行。

后续实施以前述基线的操作 ID 为单位，先核实派发名称＋经营负责人、普通项目完成前补正式编号、生产负责人后补、成功留在同一项目基本详情，再依次比对项目包、经营、四类型生产、资料统计和权限。每项同时验保存、刷新／返回、失败后续和撤权，不让用户代替团队找全站流程问题。合同／外委等分段保存要验证部分成功与安全继续；旧文档、现代码和确认需求冲突先登记，不能自行挑一种抄写。C8 工程链和这些人类操作证据分别记账，全部缺口不得再用原 98% 估算。

## 10. 2026-09-30 第一阶段 OA 验证收口

公司 Ubuntu 的同一 Tenant／Project 已连续通过 C0～C7，并完整执行 C8。C8 的个人工作隔离、Money 分权、顺序来源故障恢复、十二库事实不变和 17/17 health 均通过；Project 缺少 C4～C6 active Link／资料投影，以及 Evidence Gateway 依赖遗留 `view` 解锁独立 download 两项 P0 未通过。阶段报告已经以 `insufficient-evidence` 冻结。

下一批按以下顺序单批执行：

1. 在不共享表、不导入领域模型的前提下补 C4～C6 版本化 Project Link／Evidence metadata 投影；
2. 收窄 Evidence Gateway entry contract，使 `metadata.read + download` 可独立工作且目标服务继续复验 file scope；
3. 发布包含 Evidence 页面 `direct` 修正、C7/C8 verifier 的固定摘要；
4. 在原 Project 重跑 C8，再做一次真人浏览器资料／下载／金额／个人工作复核；
5. P0 关闭后才进入“资质单位＋施工方”候选第二客户，不用新场景覆盖第一阶段失败。

## 11. 参考 OA 正式产品化启动

边界记账要求：执行本节每个功能时同步维护[小系统边界演变账](./specs/assembly/small-system-boundary-evolution-ledger.md)，尤其记录 Project、Production、Contract 的事实扩展和跨系统依赖。当前从生产阶段导航壳继续进入“初步设计”真实链；不得原位修改已经登记的阶段定义版本，也不得把 UI 可点击记为 Production／Evidence 闭环完成。

第一阶段报告的 `insufficient-evidence` 不再通过继续堆静态页面解决。用户已澄清目标服务器就是现有公司 ECS Auth58。下一批以 Auth58 D1 为目标环境，按一条真实业务链一批推进；Auth58 的回环／SSH 隧道入口可以验证正式 D1 内部链路，公网域名与 HTTPS 仍需独立验收，不得以本地预览替代正式部署结论。

执行顺序固定为：

1. **项目立项**：Workbench 真实读取 Project 列表并提交创建命令，经 Gateway／IAM 后由 Project Workspace 写入；修复抽屉全视口布局。
2. **Party／People**：创建后选择客户主体、经营负责人和生产负责人，不再提交自由文本姓名伪造引用。
3. **Contract／Evidence**：从项目进入合同与资料真实读写，保留各自事实所有权和 File ID／Version 边界。
4. **Billing／Money／Cost**：按已验证的建议、独立确认和金额分权连接，不跨库联表。
5. **真实统计与验收报告**：只读投影按来源展示，完成浏览器、撤权、故障恢复、备份恢复和改动归属账。

每批只启动当前最小组合，串行验收后清理，避免本地 Docker 再次出现高 CPU。第一批现已在 Auth58 完成管理员真实创建、列表／Project 页面刷新和自有数据库复读，详见[实测记录](./reports/oa-real-project-initiation-auth58-20260930.md)。撤权、相邻角色拒绝、IAM／Project 故障与恢复、公网 HTTPS 仍待独立验证；这些缺口未关闭前不把首批或完整 OA 写成“全部通过”。管理员“架构透镜”第一版只展示 Nginx→Workbench→Gateway→IAM→Project 的职责和本次状态，不建新服务或数据库。

2026-10-01 补充：Auth58 已在独立库完成 ECS SDK／单对象 STS／真实 OSS 字节／内网 complete／签名下载／空库恢复；后续 `test.41` 候选又通过原 10 个 Local 文件、真实 CNAME、Local-primary 回退及 12 个 Version 混合空目标恢复，见 [报告](./reports/auth58-evidence-oss-adapter-validation-20261001.md) 和 BF-081。单份合成 OSS 字节离机误删恢复已通过，D1 专用出口／Guard 已随 `test.42` 候选包在 ECS 渲染与新 netns boot 检查通过；主机／Docker 重启和全量字节保护仍待验。主栈仍使用旧 Local 镜像，10 个 Evidence Version 与引用保持。下一 OSS 批次完成候选 IAM 预检、自动持久化和重启维护选择，再分批切换、做正式文件浏览器旅程；不直接改环境变量，不以适配结果覆盖 C8 或 UI 缺口。标准 Web OSS 静态制品分离仍按 BF-080 独立推进。Chrome 公网项目创建／复读与缺口拒绝已取得单条直接证据，详见 [记录](./reports/auth58-public-project-browser-validation-20261001.md)。

同日后续：实际 900 秒 STS 到期拒绝与重新签发已通过；`test.44` 候选完成 IAM 契约预检、内外摘要、云档渲染及配置／运行时错档拒绝，Guard 启动模板随包但未激活。Project 页面反馈 `test.43` 已单组件部署，冷备份、离机副本、空库逐表恢复和部署后事实不变通过；正式 HTML／JS／API 复读与 Header 伪造拒绝通过。Mac 锁屏时不能进行点击复测，不能以这些检查替代。

18:40 增补：解锁期间已补真实缺口反馈／自然过期恢复，受保护 People 双负责人选择与完整夹具立项（revision 6）；Project 当前 `test.48` 又修正能力标签布局及取消按钮。空项目取消已通过，其他弹窗因再次锁屏留待真人复测。三次单 Project 冷备份、空库恢复和离机副本核对通过；主 Evidence 仍 Local、演练栈仍停止。接下来优先独立关闭 Evidence 入口粗能力耦合与 C4～C6 资料 Link／投影，再按原连续链补证，不启动双栈或重启 Docker。

后续顺序不扩大为全 Suite 同时更新：

19:20 更新：BF-082 三进程真实权限矩阵和 Workbench 离线兼容启动已通过，13 个主库摘要／17 主容器 ID 不变，探针已停止；[专项证据](./reports/auth58-evidence-native-entry-validation-20261001.md) 不代表主站切换或连续 C8 通过。C8 源码已删除 view 补授及固定 fail 结论，改为三种原生能力的真实断言，离线负例通过。正式交付仍须联合备份和兼容闭包，不擅自重启 restore20／Docker。

20:00 增补：C4 来源的真 PG18／真 IAM 和 Project 消费者的源码、7→8 migration／typed HTTP 测试端口／单库空目标恢复分别通过，主站仍未升级；[来源](./reports/auth58-production-material-source-validation-20261001.md)与[消费者](./reports/auth58-project-production-material-consumer-validation-20261001.md)是两份范围明确的组件证据，不能拼成完整运行链。下一批补真实 Workload／正式链和 C5/C6 来源，再按兼容闭包交付；不复活 restore20、不启动双栈，后续升级不能把 schema 8 直接换回旧二进制冒充回退。

20:15 增补：C5 Results 资料来源的真 PG18／真 IAM 与五表零读写增量通过，主站未更新；下一批 C6 同样只补来源端口，再补 Project C5/C6 消费与正式链。[C5 来源实测](./reports/auth58-results-material-source-validation-20261001.md)不能关闭连续 C8。

20:25 增补：C6 双来源端口真 PG18／真 IAM 及十表零读写增量通过，探针已停止且主站保持。下一批按[消费者规格](./specs/assembly/project-results-cost-material-consumer-v1.md)补 Project 三类显式 Link／000009／只读材料组合，先源码与隔离恢复，再真实 Workload／兼容闭包；不复活 restore20、不同时更新全 Suite。最迟 22:07 按本轮授权暂停检查。

20:38 三类型消费者源码、精确权限契约、真 PG18 8→9／历史 7→8 回归与两份 current-schema 单库空目标恢复已通过，结束探针停止、主站保持。下一批按同一规格补真实 IAM 换票／目标验签的有界进程链；上游写入种子必须明确测试引用，不外推完整业务／公网／联合恢复／连续 C8。[组件实测](./reports/auth58-project-results-cost-material-consumer-validation-20261001.md)。

21:05 真实 Workload 只读链通过，补上四类型资料来源与独立 Evidence 元数据、五种精确原生权限、来源／Evidence 暂停和恢复、成功 Link 本地重放及自有表零 GET 增量。所有专项停止、Main／restore20 保持。[进程链报告](./reports/auth58-material-workload-validation-20261001.md)明确来源种子引用／Evidence fixture，不代表正式写业务。下一步固定四应用候选，联合备份与兼容闭包仍是 Main 切换门禁；本轮到最迟 22:07 留停供用户检查。

候选制品已固定：四个 `test.50` 镜像完成 ACR digest／12 文件核对和 actual server 空库启动，D1 bundle 摘要／17 镜像配置／新端点／Manifest／IAM8 无数据库 parser 预检也通过，见[报告](./reports/auth58-material-candidate50-validation-20261001.md)。Main 仍未切换。下一阶段按联合备份→离机→空目标兼容恢复→真实入口／真人提交分批推进；不以旧 Project48 强行读 schema 9，不混入 OSS Primary 切换，不重新启动 restore20。

1. Project 缺口反馈、自然过期恢复、负责人选择和立项已取得单条切片证据；解锁后补其他三类弹窗取消，不重复发立项命令。分批验证 BF-082 Evidence 入站原生能力与目标动作／资源范围仍分离，再推进 C4～C6 Link／投影。
2. 用户确认维护窗口后才激活 Guard 启动模板、重启 Docker 并重验主栈健康及正／负隔离；已授权停止的 `restore20` 保持停止并保留数据，不重新运行双栈。主机 reboot 另行明确，不把 Docker restart 当作主机恢复证据。
3. 再确定 OSS 全量字节保护／保留方案，完成主 Evidence 冷备份、兼容镜像切换及正式文件浏览器旅程。
4. Standard Web OSS 制品分离与合同等真实页面逐条实现；人员选择器已有 Project 单条证据，不把它或本批基础设施通过当作完整 OA 产品通过。

Standard Web 的 OSS 分离是独立验证轴，不以 Files Provider 切换完成为技术前提；必须先明确静态制品与 SSR／身份数据边界、API 入口和 Cookie／CORS／缓存策略，不将现有动态页面直接公开到 OSS。每轮仍只实施一个可验收切片。
