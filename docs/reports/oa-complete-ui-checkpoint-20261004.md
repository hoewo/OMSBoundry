# 完整 OA UI 施工检查点

2026-10-04 16:09 UTC（北京时间10月5日00:09）四小时检查点：本续轮12:13 UTC开始，现于16:13上限前停止新实施／验收并保存结果。修复缺People仍能开启阶段profile的启动门禁后，最终正常原方案UI已用修复二进制重跑16个真实场景通过、0页面错误；另有5项独立启动检查，377项Node和Go／定向race／TS等收尾检查实际通过。证据和最终截图已封存／实际查看。[只读收尾](../../acceptance/oa-initiation-repair/evidence/production-stage-native-20261004/closeout.json)确认Main17健康、无本批发布／Grant，自有本地及远端夹具已停，本机Docker未启动，源码／私有备份／密钥／卷保留。Main仍schema2且有3个旧活动任务；新共享集合尚不能直接覆盖旧事实，Project v1资料消费规则也不能表达新多文件集合。下轮先补显式过渡及版本化来源／消费闭包，再验实际阶段D1与keys／旧会话恢复和匹配发布，后续原初设／施工图／变更竣工等按原动作逐项完成。不得用新项目绕开旧任务、弱化v1、单独发布前端或把方案切片标为完整OA通过；整体仍为insufficient-evidence。

2026-10-04 BF-106 最新收口：原方案面板／800px抽屉已在正常Workbench接独立正式IAM／Production／Evidence／PG18，真实私有Nginx→Gateway的16场景零页面错误，真库整批／并发／旧事实保持和五所有者逻辑空目标恢复后原UI继续通过；[报告](./oa-production-stage-native-validation-20261004.md)取代下方13:51“原生未实现”的当时状态。最终原UI-only13项、377项Node及编译／Go／定向race等分层封存；截图已实际查看，原CSS逐字保持。修复了真实正常页空root挂载、最新校审单选错、失败续办缓存及同批文件顺序问题，未重写UI。Main零发布／零Grant，仍不是用户公网看到的新生产工作面；旧活动任务过渡、真实阶段D1／keys／旧会话恢复、精确授权与匹配发布、其他阶段和全站验收未完成。当前本机Docker全程未重启，22个已核对残留助手已结束，自有7进程、测试Chrome、远端PG／Nginx均停止；Main17主服务只读健康、数据卷／备份保留。后续不原位扩大v2，不以新项目避开旧事实。以下早期状态保留历史，完整目标仍未完成。

2026-10-04 13:51 UTC 内存减负交接：用户的 8GB 物理内存出现应用内存不足，当前暂停本地重型验证。核对本地 Docker 仅余本任务一次性 `oms-bonus-overlay-test-20261004` PostgreSQL 18 fixture，已停止并因原有 `AutoRemove=true` 自动移除其临时 tmpfs 测试数据；已有 D1 数据卷、备份、源码和回执没有删除。`docker desktop stop --timeout 30` exit 0，随后 Desktop status 提示未运行；Docker backend／虚拟机及本任务测试浏览器进程检查均无匹配。另按已核对的测试 PID 清理了未响应退出的 `JGjRSm` Chrome／driver；这次未完成运行不记通过。没有关闭用户浏览器或改动线上 ECS。后续本地容器只按需启动、结束即停，不同时跑多套组合／构建／浏览器；恢复、迁移等仍须隔离环境证据，不能为省内存直接在 Main 做破坏性演练。

原方案阶段 UI-only 试片的最新 `/tmp/oa-production-stage-ui.KYXDyY/stage-ui.json` 实际为 13 场景通过、0 页面错误，包含多主文件、可选校审单、零分／小数分、空评分保留、移出确认及失败保留输入。该回执明确 `native_accepted=false`、`main_changed=false`、`full_oa_accepted=false`，仅内存端口，不能冒充 Production／Evidence／IAM 原生闭环；截图尚待视觉检查、此组回执尚未封存。原组件／CSS 候选及[原操作对照](../specs/assembly/oa-production-stage-original-operation-map.md)保留，原生整批保存未实现；最后编辑器状态保留修改后的独立 TypeScript 检查尚未重跑。下一步在资源允许时先补当前候选检查，再明确兼容版本、不变量和验收后实施原生阶段，不把 UI-only 通过拼成完整交付。完整目标保持未完成。

2026-10-04 13:39 UTC 恢复补证：本续轮所选 D1 10运行容器／六库联合备份及新空目标恢复、原 command 重放、Evidence 原字节和原人员／奖金 UI 继续14场景通过；两套运行容器均已停止、卷／备份保留，[报告](./oa-professional-selected-d1-restore-20261004.md)。Main 零变更。下方“profile联合恢复待验”由这份所选证据取代，完整 Suite／历史主站升级／公网仍待。继续[原方案阶段](../specs/assembly/oa-production-stage-original-operation-map.md)，没有将完整目标标为完成。

2026-10-04 续轮最新：原人员→专业奖金已接正常 Workbench 候选，真实私有 Nginx→Gateway→IAM／Project／People／Bonus连续20场景、347 Node、Project PG18 163／含子418、Bonus PG18和race32／含子93、TS／Go／vet通过。原CSS和60来源摘要保持，5次失败保留。Main无发布／Grant，profile联合恢复及同项目公网另验；[最新报告](./oa-professional-bonus-continuous-validation-20261004.md)优先于下方11:48早期“正常工作台未接线／下一消费切片”状态，完整OA不完成。

2026-10-04 11:48 UTC 本轮人员候选收尾：直接复用原六组件／逐字 CSS，原选人／专业四列／审定概算抽屉接真实 IAM／People／Project PG18，21 场景通过；整组原子、未知回复 reload 续原命令、目录／人员能力隔离、原审定清除留史及 profile off/on 重启分层实测。Node 315／315、Project 普通 119／含子 363、全量 PG18 161／含子 411、专项 race 14／含子 20、TS／Workbench Go／vet通过；早期失败保留。11:41 Main 17 容器只读健康，本批无 Main 发布／Grant／业务写，正常人员页签尚未启用。[原生报告](./oa-production-personnel-native-validation-20261004.md)优先于下面早期候选状态；[收尾回执](../../acceptance/oa-initiation-repair/evidence/production-personnel-native-20261004/closeout.json)。下一闭环为新专业分工版本化引用→Bonus 明确消费→原工作面正常接线／匹配发布，不放宽旧来源角色；全角色、其他生产子栏目、全站和 Suite 恢复仍待验。自有 tmpfs fixture 容器已停止／自动移除，最终一次性测试库不保留，证据和此前恢复检查点保留，不动 Main 数据卷。四小时上限前只做收尾，不启动下一实施切片。

2026-10-04 后续生产布局：原总览／AntD 页签／CSS 候选已挂载，按栏目独立读取，真实组合 17 场景、相邻原表单 13／22／14／15 场景、307 项 Node／编译／Workbench Go及两次启动通过；失败定位／旧夹具保留。未部署 Main／新增 Grant，9 个生产子栏目未接；继续[D01 专业人员](../specs/assembly/oa-production-personnel-clone-gap.md)，不将负责人冒充专业分工。[本批报告](./oa-production-layout-ui-validation-20261004.md)为最新候选证据，下面收尾是此前批次。

2026-10-04 09:13 UTC 本轮收尾：B12 私有原组件／实际 Workbench 的 14 场景、300 项 Node 零 skip及最新证据已归档，相关文档 773 个本地引用零缺失、diff／D1 脚本语法检查通过。Main 再次只读为 17／17 healthy，本轮无部署／Grant；自有临时测试服务和夹具库已清理，恢复检查点 dump 保留。公网开票／奖金精确授权等待答复，完整目标不标完成；[收尾回执](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/closing.json)。

2026-10-04 续轮实际进展：B12 原 UI 在实际 Workbench→Gateway→真实 IAM→Bonus／PG18 完成正常登记、撤回留史及两侧刷新；14 有限场景、0 页面错误，最终 7 记录／9 revision／7 成功命令／9 审计，旧 Pool／Money 建议零新增，来源摘要保持。撤销金额后当前与历史脱敏、源停机零写、空目标单库恢复／原命令继续通过。BF-103 修正式兼容入口（仅候选），300 项 Node 零 skip、真实 PG18／Go／TS／构建及匹配 reader 正负例通过。Main 只读仍 17 健康、本轮无部署／Grant；奖金和 Billing 的精确授权仍待答复。完整 OA、B11／专业生产、公网、OSS 和 Suite 完整恢复不标完成，[实际报告](./oa-bonus-manual-ui-candidate-20261004.md)。

2026-10-04 08:32 UTC 恢复续轮：原 B12 手工奖金已候选实现，两侧正常原组件→独立 Bonus HTTP／PG18、单库空目标恢复／丢回复续办／继续写通过；schema 2→3 保持旧 Pool／建议，前端 112 项零 skip。新 namespace 精确分权，未发布 Main、未新增 Grant，17 服务只读健康；原来源增至 52 份且摘要一致。专业生产、B11、正式授权／发布／原 C 公网与完整 OA 未完成，[本批证据](./oa-bonus-manual-ui-candidate-20261004.md)。以下四小时暂停为上轮历史，不代表本轮已经暂停。

2026-10-04 07:50 UTC 四小时收尾暂停：本续轮从 03:55 UTC 开始，按用户四小时上限在 07:55 UTC 前停止新实施／验收。Main 已是 Workbench `.20261004.6`，不是下方历史段落的旧镜像。07:43 的支付六表单取消矩阵及原生零写收尾通过，07:45 D1 check exit 0，07:46 实际容器清单为 17 个健康主服务，private PG／restore20 保持停止；07:48 `git diff --check` 和关键文档 35 个本地引用检查通过。原 OA 47 份来源摘要一致、零写原目录。没有把成本批准推及 Billing、没有为收尾新增业务事实或授权。完整 OA 目标仍未完成；恢复后按[全站清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)继续，开票独立授权待答复，奖金语义扩展与分类型生产工作面可以在授权等待期间推进。

2026-10-04 07:36 UTC 续轮实际状态（下方“Main 未变化”属于上一轮暂停检查点）：Contract 22／Money 23／Billing 1、Cost 3 和 Workbench `.6` 已发布，17 服务健康。同一原 C 的合同／支付九检查点、可见历史／文件预览及原生收尾通过；成本有限连续链和七类／两侧十二组合、十二记录／24 原生命令收尾通过。修复真正阻断成本连续撤回的弹层遮挡，旧失败保留，正常点击续原记录而非重建或强制点击。新候选 282 项 Node 零 skip、12 场景成本原组件与两次独立启动通过；13 库／3 文件树空目标数据恢复、离机备份核对、仅 Workbench 的发布门禁通过，不代表完整 Suite 恢复。原 OA 47 份摘要一致，零写原目录。Billing 精确授权仍待单独确认，完整 UI／OA 和全角色仍未完成；最新[公网实测报告](./oa-commercial-public-ui-validation-20261004.md)与[全站剩余清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)优先。

以下为 2026-10-04 上一轮暂停检查点，保留当时证据，不取代上方续轮状态。这不是完整 OA 交付报告。范围仍是[全站操作清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)，招投标只是举例；当时没有完成全部 UI，也没有发布新的经营工作面到 Main。

## 本轮实际推进

合同、支付、开票直接复用原组件／样式，接各自 owner 的版本化命令、权限、精确金额、追加历史和未知回复续办。不是用 Workbench 数据库代替小系统；Base／Customer Overlay、IAM 与 Evidence 边界仍保持。原 OA 目录只读，44 个复用源文件摘要复核一致。

| 工作面 | 最新原组件检查 | 原生分层证据／剩余缺口 |
| --- | --- | --- |
| 原始合同、补充协议、定案、附件、阶段／最终结算 | ae：22 场景／32 命令，0 页面错误 | schema22 真库／自有库恢复；真实 IAM＋Contract＋Evidence 字节恢复 m；历史夹具 18→22 四项迁移／冲突原子回滚通过。最终结算更正、全角色与发布闭包未完成 |
| 甲方实付／公司实收，两链三费用、冲销 | j：14 场景／20 命令，0 页面错误 | Money PG18 b／独立启动 a。实际 issuer＋Project 联合、开票关联、核销／Receipt Set 和全角色未完成 |
| 两链开票、纠正／历史、登记无效、文件引用 | f：15 场景／25 命令，0 页面错误 | 最新 Billing c 真库两项＋真实 IAM／Billing／Evidence 字节恢复 e 通过；修复移出最后附件 500。Project／Party 仍明确 HTTP 参考夹具，支付关联和原排队上传／内联预览等效未完成 |

最新 51 个组件场景均使用 bundle SHA `f716e1cc6de47984272582de5055b1900c87941afe79f2259f611e22a1e0cfb6`。这只是本地浏览器显式内存原生测试端口，不能与真库／issuer 层拼成经过 Nginx 的同项目公网验收。较早 ad／i／e 的 `e9f67e…` 回执保留，只代表其当时构建；没有改摘要冒充新构建通过。安全回执：[合同 ae](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/components-ae.json)、[支付 j](../../acceptance/oa-initiation-repair/evidence/cash-20261004/components-j.json)、[开票 f](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/components-f.json)。[合同截图](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/formal-settlements-ae.png)、[支付截图](../../acceptance/oa-initiation-repair/evidence/cash-20261004/two-chain-payments-j.png)、[开票截图](../../acceptance/oa-initiation-repair/evidence/invoices-20261004/two-chain-invoices-f.png)已视觉检查；它们是测试事实，不是主站业务。

最新 Workbench native c 两次独立启动／匿名登录入口／五个 JS 与 CSS 实际 HTTP 摘要符合当前源码，进程停止；server SHA `9fda6496df9bea41cc25a0eb7c3122b6b43877d978d3f462f2d3ac82b07bae5d`，[startup c](../../acceptance/oa-initiation-repair/evidence/workbench-ui-candidate-startup-20261004c.json)。262 项 Node 零 skip、TypeScript check／build，Contract／Money／Billing／Workbench Go test＋vet 均通过；普通 Go 中可能 skip 的 DB 检查不记真库证据。静态边界检查实际通过：15 应用 GOWORK=off 独立解析、零跨应用源码依赖／本地 replace、零跨库 SQL 机制；不是运行时边界、HA 或全架构已经验证成功。

## 公网没有变化

[只读 Main 收尾](../../acceptance/oa-initiation-repair/evidence/ui-candidate-closing-20261004a.json)：17 主服务健康、D1 check exit0；Workbench 仍 `.20261003.6` 的既有 digest，Contract schema18／Project schema11／BE schema2；验收夹具 C revision13 保持。候选私有探针与 restore20 均停止，未修改生产业务、用户101或 Main Grant，未切换 Main 镜像／schema。Evidence 仍 local、前端仍 ECS，OSS 分离未完成。用户在公网看到的旧空页不能因为本候选通过而称为已修复。

## 继续工作的明确顺序

1. 对这三个经营工作面补匹配 owner API／migration／IAM manifest＋policy 的兼容闭包、全角色与同项目正式入口；不能单独发布 Workbench 表单到没有对应新 API 的旧 Main。
2. 按[成本逐操作对照](../specs/assembly/oa-cost-original-operation-map.md)补 Cost 自有七分类／经营生产、用车里程与可空字段语义，再直接接原列表／表单／附件；当前只核对源码，没有冒充成本 UI 已完成。
3. 继续奖金、四类型生产阶段／人员、结果／外委及付款、资料统计／个人工作、完整人员权限；基本字段／项目包／经营参与人分权及已有工作面的未等效操作仍须补齐。
4. 以连续原 OA 旅程、失败续办、同项目真实入口与完整 Suite 备份／空目标恢复结束阶段；局部组件或接口通过不结束目标。跨客户 Overlay 的可预见边界和长期升级仍没有整体通过证据。

详细限制分别见[合同报告](./oa-contract-draft-ui-candidate-20261004.md)、[支付报告](./oa-cash-ui-candidate-20261004.md)、[开票报告](./oa-invoice-ui-candidate-20261004.md)。目标保留未完成，不以本检查点替代完整仿 OA。
