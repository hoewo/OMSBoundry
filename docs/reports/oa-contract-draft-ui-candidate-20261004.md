# 参考 OA 合同草稿组件候选验证

2026-10-04。结论：原双列／抽屉、签约主体与原始合同显式生效、三类协议意图→正式确认、三类附件引用及阶段／最终结算已候选接线。schema 22 私有 PG18、完整应用库空目标恢复、独立启动和原组件分层验证通过；真实 IAM＋Contract＋Evidence 三进程上传／引用／字节联合空目标恢复 m 已通过。未部署；最终结算更正、全角色／冲突核对、兼容发布和同项目公网链未完成，B04～B06 和完整 OA 仍不通过。

最新重构建收尾见[整轮检查点](./oa-complete-ui-checkpoint-20261004.md)：bundle `f716e1cc6de47984272582de5055b1900c87941afe79f2259f611e22a1e0cfb6` 的 Contract ae／Cash j／Invoice f 共 51 场景、0 页面错误及 Workbench native c 两次独立资源启动通过；较早回执／摘要保留原版本，不混称新构建。所有组件检查仍是本地明确测试端口，未发布、不代表公网或完整 OA。

完整任务没有缩为招投标或本合同草稿。经营参与人、合同、支付／回款、开票、成本／奖金、分类型生产、资料统计、人员权限与交付仍按[全站清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)逐操作推进，空页、能点击或仅有 API 不记完成。

## 实际修改和边界

原站提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 15 个合同／文件预览相关源文件直接复用，样式和原 780px／1360px 抽屉保留；原站目录未改。源摘要见 `frontend/oms-workbench/frontend/source-provenance.json`。必要修改为精确字符串金额、nullable 零费率、日历日期、原生权限／数据端口、分段继续及未确认状态提示。

BF-098 的 Contract 自有 Customer Overlay（schema 20）保存两链、分项、费率、协议草稿和结算意图，Base 拥有原生合同事实。schema 21 在本 Contract 事务内复用原生 Change 并追加不可变意图→Instrument 关联；三类型显式确认，原始费用和已确认协议冻结，不按编号猜对应。每次保存追加命令历史／审计；取消留史。阶段／最终结算单独冻结当前来源 revision、主体位置和金额，不复制可编辑预览，最终结算后的更正尚未实现。详见[草稿／确认规格](../specs/contract-ledger/reference-oa-contract-drafts-v1.md)。

schema 22 的三类业务材料由 Contract 原生 evidence.manage、稳定命令和 material revision 登记／移出；通过配置 Evidence 版本端口复验，正式模式要求自己的 Workload 单 metadata.read／精确 File 凭证。Evidence 独立拥有 File／Version、字节和访问。浏览器使用既有 SDK 直传，完成与业务登记分开；未知回复复用完成身份，不再上传；移出只结束业务引用、不删除文件。未选外部端点时结构化合同仍可独立启动，材料明确不可用。Workbench 无业务库，业务应用不嵌 OSS SDK／长期密钥；不覆盖、删除或假称已迁移旧内嵌 Evidence。见[材料规格](../specs/contract-ledger/reference-oa-materials-v1.md)。

Base→扩展→Project Link 各自事务，未知回复复用原 ID／版本；GET 不自动创建。附件与正式确认均不自动付款／发布奖金，也不能拼成完整合同或完整 OA 已验收。

原抽屉新增小型原生准备区：选择 Contract 当前本目录主体、合同内角色和生效日期，再明确确认原始合同。两主体和条款由 Base 原用例冻结；没有自动造主体、代填生效日期或把协议意图一并生效。精确 Contract 凭证可以读不含业务事实的权限上下文，但不获租户创建／全目录权限，相邻合同仍拒绝。

逻辑回归与组件测试修复保存后仍为只读、延迟数字全选夺回焦点、重复表单 ID／日期标签错指。分项和逐条预览精确，排序与服务端一致，不依赖浏览器语言；预览不写成 confirmed 事实。回车不能误触发父表单保存；确认回复失败展示并保留显式续办，不再由弹窗抛未处理异常。合同列表分开显示正式冻结结算与未确认预览。原协议文件区域复用 SDK 控件，不保留旧物理删除逻辑或双份假空附件列表。

## 运行过的检查

| 检查 | 实际结果 | 不能证明什么 |
| --- | --- | --- |
| Contract 与 Workbench `go test ./...`、`go vet ./...` | 通过 | 不是全量真实数据库或全站浏览器 |
| 前端 `npm run check`、`npm run build` | 通过 | 不是业务闭环 |
| 全量 Workbench `.test.mjs`＋前端适配器 Node | 最新 262 项通过，0 skip；合同适配器 23 项；新增 selected-only 路由与架构写回执分类检查 | 故障为测试端口模拟，不是公网故障 |
| 私有 PG18 的 `TestWorkspaceNativePostgresExactMoneyHistoryAndReplay`，g | 实际 1 项专项通过、无 skip；schema 22，精确大额、原生 IAM verifier／Actor、三类协议确认、三类材料复验／重放／拒绝／移出、追加历史／旧导出防遗漏 | Evidence 复验为显式 fixture；不是实际 IAM issuer、文件字节或组件整合 |
| 完整应用库备份→空目标恢复，g | schema 22 客户草稿／正式关联／材料引用／命令历史保持，原命令重放及继续移出；专项实际运行、无 skip | 不包含外部 Evidence bytes，不是联合文件恢复或旧 schema 前向升级 |
| schema 22 候选独立进程，g | 自有空库 ops migrate→server verify、重启事实保持、14 个匿名端点拒绝通过；私有网络、无公网端口 | 空库重启不等于带数据恢复或正式升级 |
| 本地原组件浏览器 ad（与支付 i／开票 e 同构建） | 22 场景、32 个业务命令、0 页面错误；两链／大额／零费率、草稿取消留史、原始确认、三类型正式协议、三类附件／字节预览下载／移出、阶段及最终结算、未知回复原命令续办、刷新／只读零写 | 内存端口；没有 PostgreSQL／实际 IAM issuer 或公网入口；含列表正式结算显示；完整全角色仍未通过 |
| 实际 IAM＋Contract＋Evidence 与 local 字节联合恢复 m | 三实际进程／自有 PG18；三类文件直传完成／材料引用／重放／精确下载、401／403／503 零业务写、五次单 metadata.read／精确 File 工作负载审计；Contract 与 Evidence 独立备份→空目标恢复，ID／历史／实际字节保持，恢复后继续移出且可下载 | 未经过 Nginx／Project／公网，不是 OSS 或全站恢复；无全角色验收 |
| 最新结算确认主体 GET 投影 b | `TestWorkspaceNativePostgresExactMoneyHistoryAndReplay` 实际运行、无 skip，包含冻结 confirming_parties | 测试 issuer；不单独证明公网 |
| 私有历史 schema 18→22 升级 a | 四项迁移专项实际运行、通过、无 skip；30 张既有业务表 ID／revision／快照／命令／审计保持；最新迁移冲突整段回滚、重放不造事实、并发迁移序列化；候选拒绝未迁移 schema 18 | 历史事实夹具，不是 Main 实际升级、旧运行二进制降级兼容或完整发布闭包 |

安全回执：[真库 g](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/postgres-g.json)、[应用库恢复 g](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/backup-g.json)、[独立启动 g](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/startup-g.json)、[组件 z](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/components-z.json)、[组件 aa](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/components-aa.json)、[材料预览](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/material-preview-z.png)。旧回执保留各自日期／schema，不用新结果覆盖旧失败。新增安全回执：[同构建组件 ad](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/components-ad.json)、[真实三进程与字节恢复 m](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/combo-m.json)、[最新 GET 真库 b](../../acceptance/oa-initiation-repair/evidence/cash-20261004/postgres-b.json)。截图只证明候选显示，不证明正式业务事实。

g server SHA `9582847ea11cd9b1e1c3aa248427ba1805340bce005f74600c3c8866da09efeb`，ops SHA `dd2aea486b267347897d80b3c2848ebc29cd02aad9473ece35307ce4c584ef62`，PG test SHA `3168f7b69a7e6221fe4558c259f7a88a6531119adbd885f2659cb28de5e0fd25`，backup test SHA `7be15eb16f30b0787c436a99b88260f7247fcbd9cb5f682d7a258f3f59e03391`。g server 先于新增结算确认主体 GET 投影构建；该小增量已由 b 的真库 test SHA `8b2f1e0b5aaa88ced7ffc92ea69482fdf2519ac4f6cf79fb1f7b4636c8599a03` 和 m 的实际 server SHA `70f2adc8f60fdbab3c4abc840b218c9d052d6b8a0b4843345cdf89e0ac517049` 分层补证，不能混报 g 二进制一致。z bundle SHA `eaa2a55ea46d7eaed6d21c31df6bbf804b6dd3133498117b387443e6038a72b7`；aa 追加正式列表显示，SHA `8bd8d617333f8b7101a162d33467811ebd2e361da346b978c4e8dca7150f3bb5`。

## 失败保留

组件 a 为图标名称定位，b／c 为中文空格定位；d 暴露数字全选焦点，e 暴露表单 ID／日期标签；f 因日期按 Enter 提前提交。q～s 为附件按钮可访问名称定位；t／v／w 的复选框检查早于受控渲染完成（w 诊断最终已选中），u 的 Escape 关闭原抽屉；改为普通点击后等待真实 checked，不强制点、不改 DOM 状态。y 暴露回复未知时 Modal 拒绝 Promise 抛未处理页面异常；修复后 z／aa 为 0 页面错误。失败目录独立保留，不覆盖，不将已提交测试事实隐藏。

真实组合 h 已失败：IAM 启动成功，但 Contract ops migrate 未注入旧 Evidence 兼容目录，试图在只读根创建 `/.data`。i／l 是反例期望把缺少 File 精确权限误记为 422，而实际 403；j 重复 runner 被终止且保存失败，j／k 的已有二进制复制独占冲突修正为精确复制并加锁。所有失败结果／日志保留；m 才是完整通过回执；h 没有证明 Contract＋Evidence 联合通过，Main 保持，私有 PG／runner 已停止。早期应用库恢复 a～c 的失败及后续 d／e／g 成功也分层保留。

## 未变化和后续门禁

Main 保持 Workbench `.20261003.6`、Contract schema 18 和既有入口，未发布本候选、未修改 Main IAM Grant 或生产业务。g 三探针比较 Main 前后事实／容器保持且私有 PG 已停止；真实组合 m 最终 Main 保持、私有 runner／PG 已停止，不启 restore20。用户“测试项目101”未写入。Evidence 仍 local、前端仍 ECS，OSS 分离未完成。

实际三进程／字节恢复与确认主体 GET 已补证。新增 [schema 18→22 安全回执 a](../../acceptance/oa-initiation-repair/evidence/contracts-20261004/schema18to22-a.json)，迁移 test SHA `413a991cdf781f1a4cbd753e883c5104441d39f2c40827956103c2d59fd642bb`；四项均真实运行、零 skip，Main 前后保持、私有 PG 已停止。它关闭历史夹具的前向迁移／原子失败回滚子项，不关闭匹配二进制／策略／完整发布与降级兼容门禁，也不代表 Main 已升到 22。下一步仍是全角色／相邻范围与冲突核对、正式兼容闭包和公网链。最终结算后的更正原生用例仍缺；旧单合同 Ledger Archive v1 遇扩展拒绝，版本化单合同扩展导出／恢复待实现，但完整应用库 schema 22 恢复已通过，二者不能混称。未部署就明确候选，未完成就保留缺口；之后继续支付／开票、成本／奖金、生产全过程和全站，不以这批合同取代完整目标。

最新统一前端候选摘要 `e9f67ef24d4d396d6c1df1d48aeecb63178d7cd4ba733c0591096962b6609861`；ad 22 场景与支付 i 14、开票 e 15 场景来自同构建，均 0 页面错误。它们仍使用明确测试端口，不能与 m 拼成经过公网的同项目端到端通过。
