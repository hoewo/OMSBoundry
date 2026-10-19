# 完整 OA 操作交付清单

2026-10-07 BF-121 用户调序：当前先交付统计／四层权限 UI，项目详情延后。S01～S03 的本地筛选、明细、导出、公司两链费用矩阵／成本和奖金展开已获得 UI 契约夹具证据；权限属性、49 项规则、分层映射及会话草稿同步补齐。它不关闭真实 IAM／源库、正式权限配置、D1／恢复或整行验收；详见[本轮记录](../../reports/oa-statistics-permission-ui-validation-20261007.md)。下方旧顺序与样例页面状态为历史，不覆盖 BF-121。

2026-10-06 BF-120 状态纠偏：本清单中 BF-112～BF-118 的私有运行结果保留为候选证据，不再直接关闭当前实现门禁。当前完成度以 `AGENTS.md` 为准，下一工作不是直接进入 F01／F02，而是先完成 R0 基线收敛与 C1～C8 缺口矩阵；随后按 C1～C6 补缺、C7、C8、组合／Overlay、真实环境和最终报告推进。只有重新映射并复验的行才能从候选恢复为完成。见[当前基线与下一步](../../status/current-baseline-and-next-actions-20261006.md)。

2026-10-06 BF-118：原 OA 对外委托主路径已由 Cost／Party／Contract／Evidence／Money 原生闭环接通；发生时联系人、精确 Contract／Evidence、评分／纠正、建议→Money 独立接受、历史／撤回、未知回执重放、缓存请求上界、正式 IAM 撤权、精确 Workload Scope 和八库恢复在私有 10／10 通过。[报告](../../reports/oa-external-commission-native-validation-20261006.md)。Project 仅创建边界精确读且零写，Production 调用 0，Contract 仅显式选择精确读且零写；本批不关闭 Money 拒绝／实付、逐依赖停机、全角色／跨 Tenant、Main／公网、正式 D1、第二客户或完整 OA。当前下一工作面为资料管理／统计。

2026-10-06 BF-117：原 OA 批复／审计双列已由 Results 原生闭环接通；精确 Contract／approval 来源、三项金额、多个 Evidence、纠正、撤回重登记、历史、未知回执重放、缓存请求上界、正式 IAM 撤权及六所有者恢复在私有 11／11通过。[报告](../../reports/oa-approval-audit-native-validation-20261006.md)。Project／Contract／People 摘要不变，Production 调用 0；本批不关闭 Project Completion Checkpoint、Main／公网、正式 D1、全角色、第二客户或完整 OA。当前下一工作面为对外委托，之后为资料统计。

2026-10-06 BF-116：原 OA 四类型生产总览已由 Production 单一只读投影接通，咨询／设计／勘察／测绘进度和阶段入口不再由 Workbench 猜测。私有真实入口 10／10、一次读取／页面缓存／显式刷新、正式 IAM 撤权、真实 PG18、五所有者逻辑空目标恢复和 4 份 Evidence 字节保持通过；普通总览的 Project／Evidence／Contract 请求均为 0，业务零写。[报告](../../reports/oa-production-progress-native-validation-20261006.md)。这关闭当前生产总览切片，不关闭 Production 整行、Main／公网、正式 D1、全角色、第二客户或完整 OA。当前下一工作面为 Results 批复／审计，之后为对外委托和资料统计。

2026-10-06 BF-115：原勘察试验与咨询／勘察／测绘报告已在私有真实入口接通 Production／Evidence／正式 IAM；v6 在创建／显式 5→6 时精确验证 Project revision 并冻结最小类型适用性，日常读写不向 Project 扇出，Contract 零依赖。9 场景和五所有者逻辑空目标恢复通过，[报告](../../reports/oa-test-report-native-validation-20261006.md)。Production 整行、Main／公网／正式 D1、全角色、第二客户、类型重判、批复审计和完整 OA 未关闭。当前下一生产切片改为四类型总览／进度，由 Production 事实投影而非继续加文件 Tab。

2026-10-06 BF-114：变更／竣工原双列表及上传抽屉已在私有真实入口接通 Production／Evidence／正式 IAM；不可变 v5、一次 4→5、可选 change／必需 completion、不可变替换、11 场景和五所有者恢复通过。[报告](../../reports/oa-change-completion-native-validation-20261006.md)。Project 零变化、Contract 未依赖；Production 整行、Main／公网／正式 D1、全角色、试验／报告、批复审计和完整 OA 未关闭。当前下一生产切片改为勘察试验与多类型报告。

2026-10-06 BF-113：施工图原组件已在私有真实入口接通 Production／Evidence／正式 IAM，v3 初设事实保持、同一 Work Item 显式升级 v4，10 场景和五所有者逻辑空目标恢复通过。[报告](../../reports/oa-construction-stage-native-validation-20261006.md)。方案、初设、施工图使阶段资料集合子边界达到同类可重复；Main／公网／正式 D1、全角色和 Production 整行仍未关闭。当前下一生产切片改为结构不同的变更／竣工，不套施工图校审／评分表单。

2026-10-06 BF-112：初步设计原组件已在私有真实入口接通 Production／Evidence／正式 IAM，v2 方案保持、同一 Work Item 显式升级 v3，9 场景和五所有者逻辑空目标恢复通过。[报告](../../reports/oa-preliminary-stage-native-validation-20261006.md)。只为阶段资料子边界增加同类复用证据；Main／公网／正式 D1、施工图以后阶段、全角色和生产整行均未关闭。当前下一生产切片改为施工图设计，不能把方案／初设实现无差别复制。

2026-10-04 BF-106：原方案面板／800px抽屉在正常工作台接真实独立IAM／Production／Evidence，私有Nginx→Gateway的16场景、真PG18和五所有者逻辑空目标恢复后原UI继续有限通过，[报告](../../reports/oa-production-stage-native-validation-20261004.md)。不是新空壳页面或只有接口；共享多主文件／可选校审单／精确评分、真实字节、移出留史和失败续办均实测。Main未发布，旧活动任务过渡、真实阶段D1／keys／旧会话恢复、全角色、公网和其他生产阶段仍缺；既有v2不原位扩阶段。下表生产整行未关闭，完整目标不缩为方案一个切片。

2026-10-04 最新 D01→B12：新分工独立引用、Bonus显式v3兼容及正常 Workbench原人员／奖金连续操作候选完成，真实私有 Nginx→Gateway→正式 IAM20场景及PG18／race分层通过；[报告](../../reports/oa-professional-bonus-continuous-validation-20261004.md)。另已实际通过所选六库的[D1空目标联合恢复／原 UI 继续14场景](../../reports/oa-professional-selected-d1-restore-20261004.md)，不等于全 Suite／公网恢复。下方“专业分工未消费／正常工作台未启用”为早期记录。Main仍未发布／授权，D01、B11／完整奖金、各类型阶段、全角色、同项目公网及全站目标均不关闭；下一方案功能见[逐动作对照](./oa-production-stage-original-operation-map.md)。

2026-10-04 D01 原生本批：直接复用原专业／审定／目录 UI，Project 有界六分工／正式 IAM／真实 People／PG18 接线、21 场景及 315 项 Node、全量真库／并发／单库恢复有限通过，见[报告](../../reports/oa-production-personnel-native-validation-20261004.md)。正常 Workbench／Main 未启用；生产整行不能关闭，新分工引用／Bonus 消费、全角色、其他阶段工作面及匹配发布／Suite 恢复待验。[D01 对照](./oa-production-personnel-clone-gap.md)和全站逐行账目继续有效，不把局部候选当完整 OA。

2026-10-04 续轮 B12 补证：原组件及实际 Workbench→Gateway→真实 IAM→Bonus／PG18 共 14 有限场景正常点击，7 条登记／9 版历史／7 成功命令／9 审计；金额撤权、旧入口兼容、错误侧别／伪造身份拒绝、来源停机零写、恢复续办和刷新保持通过。BF-103 补有限同 audience namespace 准入，不能扩权。仍是私有候选，没有 Main 发布／Grant、B11／生产专业分工、公网和 Suite 恢复通过；不能关闭奖金整行。[最新报告](../../reports/oa-bonus-manual-ui-candidate-20261004.md)优先于以下早期局部来源说明。

2026-10-04 08:32 UTC 候选更新：B12 经营／生产手工奖金已直接复用原四组件及 CSS，以 Bonus 自有显式 profile／schema 3 和新 namespace 完成保存、刷新、同人不同分工、比例＋池外固定额、留史撤回、丢回复继续及单库空目标恢复。9 场景原组件浏览器／实际 PG18、受保护 HTTP 和旧 schema 2→3 八表摘要保持分别通过，全部为有限 private/local 候选证据。未发布、未新增 Main Grant；B11 来源奖池、生产专业分工、正式同项目入口、全角色和 Suite 恢复继续缺口，不能以此关闭奖金整行。见[候选报告](../../reports/oa-bonus-manual-ui-candidate-20261004.md)。

2026-10-04 07:36 UTC 公网更新（取代下段早期候选状态）：Contract 22、Money 23、Billing 1、Cost schema 3 和 Workbench `.6` 已匹配发布。原 C 成本有限连续链及经营七类／生产五类组合通过，共十二记录／24 命令；真正阻断连续撤回的弹层遮挡已修复上线，并续原记录正常点击通过，旧事实／Grant 保持。合同／支付实际完成九检查点、两链合同、三协议、两定案、一个真实文件预览下载、两 Cash 和一冲销，历史区截图与原生边界收尾通过。Project revision 18，其他项目／旧事实保持、17 服务健康。开票缺单独授权，驱动整体仍 `passed:false`，没有提升为完整经营链／OA；全角色和剩余工作面继续缺口。详见[实际公网记录](../../reports/oa-commercial-public-ui-validation-20261004.md)。

2026-10-04 当前：经营参与人的有限真实主链已上线；合同、支付、开票三个经营工作面复用原组件，候选接线及分层检查通过，均未发布。合同实际 IAM＋Evidence／字节空目标恢复 m 及历史夹具 schema 18→22 原子迁移 a 通过；支付 PG18／独立启动／浏览器 i 通过；开票最新 c 的实际 IAM＋Evidence／字节恢复 e、真库 c 和组件 e 通过，修复最后附件移出 500。最终更正、全角色、匹配发布闭包及剩余页签没有因此完成；原组件内存端口、测试 issuer、真实 issuer＋局部来源夹具与公网分别记账。用户指出招投标只是举例，交付范围一直是 BF-084 完整 OA，不以单个例子、能点击页签或小系统 API 通过代替可用。原组件视觉层直接复用，只替换数据／权限端口。

下表是当前施工缺口，不是完成率。原流程证据和详细步骤仍以[操作基线](./oa-original-usage-workflow-baseline.md)为准；每行必须验新增／编辑／取消、保存／刷新、权限和失败继续后才能关闭。

| 工作面 | 复用组件或现有工作面 | 事实所有者及当前关键缺口 |
| --- | --- | --- |
| 派发、完成、基本信息、项目包 | 原立项／项目详情抽屉、列表 | Project；部分链已公网通过，管理费／资质／完整字段、包操作与权限尚未等效 |
| 甲方 | 原行内编辑及历史名称提示 | Project 项目联系上下文；正式 Party 身份另选，完整角色待验 |
| 经营参与人 | BusinessPersonnelList、PersonnelSelect、确认弹窗 | Project 新商业参与职责；管理员名单／添加／历史结束／重入／未知回复继续已公网通过；岗位、离职候选、全角色分权与父包传播未等效 |
| 招投标 | BiddingFileList、BiddingMethodControl、原预览抽屉 | BE＋Evidence；当前 local 主链已通过，完整分权／更多失败及 OSS 待验 |
| 原始合同、补充协议、定案 | ContractAndAmendments、ContractForm、ContractAmendmentForm、原附件／预览组件 | Contract schema 22 已发布；原 C 真实两链保存、甲方正式激活、三类协议确认、阶段／最终结算和单文件上传／640×400 预览／字节下载已有限通过，历史实际可见且原生收尾核对旧事实。三类材料／停机／字节恢复 m 仍仅 private/local。最终更正、全角色／冲突、更多材料和完整链未通过；见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md) |
| 支付、回款 | BusinessTransactionsPanel、PaymentRecordList、ClientPaymentForm | Money schema 23 已发布；同一 C 的甲方实付 1.01／公司实收 2.02、两链列表、保存关联复读、真实提交丢回复后原命令续办、明确原因冲销及原金额保留已有限公网通过。六个两链／三费用原表单可见和取消零写另通过，十二业务库／两类 Grant 保持；不替代六组合保存、开票关联、核销／Receipt Set 和全角色，后者仍未通过。见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md)及[分层报告](../../reports/oa-cash-ui-candidate-20261004.md) |
| 开票 | InvoiceRecordList、OurInvoiceForm、原附件／预览组件 | Billing schema 1 及原组件已发布；组件 15 场景、PG18 和正式 issuer＋Evidence／字节恢复 e 有分层证据，最后附件移出修复已验。C 无原生项目授权，实际页面明确缺权、登记按钮不可用，公网登记尚未执行；成本授权不能替代。支付关联、原排队上传／内联预览差异、全角色和同项目公网链未完成；见[公网报告](../../reports/oa-commercial-public-ui-validation-20261004.md) |
| 经营／生产成本 | 原 ProductionCostList／Form、AttachmentUploader／InlineFilePreviewLayout | Cost 自有 `reference-oa-cost-v1` schema 3 已发布，明确七类／两侧、整数分、nullable 日期／描述、冻结 Project、追加纠正／撤回及版本化附件，不伪造旧四类／里程。12 项适配器、12 场景原组件、分批 PG18／原生 IAM HTTP、旧事实兼容／空目标数据恢复分别通过。C 管理员实际新增／更正／文件预览下载／撤回留史／两侧隔离／正常刷新原命令续办及十二分类适用组合／原生收尾有限通过；Workbench `.6` 修复连续撤回遮挡。全角色／撤权、更多负例及外委／付款仍缺。见[规格](./oa-cost-receipt-overlay-v1.md)和[逐操作对照](./oa-cost-original-operation-map.md) |
| 经营／生产奖金 | 原 BonusPoolEditor、BonusAllocation／Form／List／Statistics | Bonus；B12四原组件显式profile及整数舍入、同人不同Assignment、池外固定额、整批幂等、刷新、撤回留史已有有限候选证据；D01六真实分工版本化引用→Bonus显式v3→正常原UI20私有场景及所选D1恢复14项另通过。旧Pool／API／确认来源保持，不伪造Money建议或付款。Main未发布／未授权；B11来源奖池、全角色、同项目公网及完整启用组合恢复仍未通过，手填基数不冒充确认来源。见[连续报告](../../reports/oa-professional-bonus-continuous-validation-20261004.md)和[所选恢复限制](../../reports/oa-professional-selected-d1-restore-20261004.md) |
| 生产人员及四类型阶段 | ProductionPersonnelManager、阶段文件组件、Report／试验工作面 | Project／Production；D01六分工→Bonus显式v3→正常原UI已有20私有场景及所选D1恢复14项；方案16、初设v3九、施工图v4十、变更／竣工v5十一、试验／报告v6九场景依次通过。v6 只在创建／升级冻结 Project 类型适用性，正常读写无 Project 扇出，Contract 零依赖；阶段资料子边界已有同类、异类和多类型适用性证据。Main未发布／未授权；正式阶段D1／keys恢复、完整角色、四类型总览／进度、类型重判和第二客户仍缺。见[试验／报告报告](../../reports/oa-test-report-native-validation-20261006.md)、[边界演变账](./small-system-boundary-evolution-ledger.md)及[原逐操作映射](./oa-production-stage-original-operation-map.md) |
| 变更洽商、竣工、批复、审计 | 原文件列表、结果双列与表单 | Production 的变更／竣工资料已有限私有通过；Results 的批复／审计又以原双列私有浏览器 11／11验证精确 Contract／approval 来源、真实文件、纠正、撤回重登记、撤权和六所有者恢复。两域仍各自留史；本批未执行 Project Completion Checkpoint，Main／公网／正式 D1／全角色仍待验 |
| 外委及付款 | ExternalCommissionForm／List／Summary | Cost＋Party＋Contract＋Evidence＋Money；原列表／统计／抽屉、真实登记、合同文件、评分、纠正、建议→Money 独立接受、历史撤回、缓存、撤权和八库恢复已私有 10／10。Money 拒绝／真实 Cash／Allocation、逐依赖停机、全角色／跨 Tenant、正式 D1／Main／公网仍待验；部分成功只续未完成段，禁止删除并重建已保存支付事实 |
| 项目资料、统计、个人工作 | 原资料检索／统计、已有投影 | 来源＋Project只读装配；隐藏面板零读取／可见来源正式授权已验，项目统计上部仍有标注的演示数据。新方案集合不能用旧单成果资料v1表示，当前来源失败关闭／消费端relation唯一规则不是整批资料通过；须补版本化来源与Project消费兼容，不丢文件、不放松旧契约。真实资料／统计逐项替换，Link、无权、故障、空数据分别显示 |
| 人员、账号与权限管理 | 原人员／权限工作面 | People 与 IAM 分开；候选人员不等于账号，全角色和双租户待验 |
| 正式交付 | 当前 D1 发布链 | 联合备份、空目标恢复、故障续办、OSS 分离、完整连续旅程；局部 API 证据不能拼成整体通过 |

## 当前执行顺序

商业参与名单、成本十二分类适用组合、合同／支付有限主链、生产 v2→v6 阶段、四类型总览、Results 批复／审计以及对外委托私有原 UI 闭环已有分层证据；这些证据不拼接成完整 OA 通过。当前下一纵向切片是资料管理／统计：F01／F02 复用原项目资料树、按展开读取、下载及下载人员明细；S01～S03 复用台账筛选／导出、公司两链和人员奖金统计。所有投影读取来源 Owner，Project 只拥有 Link／上下文，Workbench 不建统计库或第二份金额真相。Project Completion Checkpoint、阶段 D1／keys 恢复、Main／公网、全角色和第二客户门槛继续保留；对外委托扩展矩阵、B11 来源奖池、合同最终更正与三费用关联仍单列。

## 每批交付证据

记录原组件路径与摘要、目标版本化端口、Base／Overlay 变化、涉及镜像／schema／策略、实际浏览器操作及原生复读、失败反例、旧事实保持与恢复。未部署只能称候选；未实测只能称待验；不以静态演示或表单外观升级状态。
