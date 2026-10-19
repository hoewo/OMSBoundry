# 原 OA 经营界面：公网实际操作验证

2026-10-04。问题 A → 可验证结果 B：只读连接和候选组件测试不能证明用户能连续操作 → 在同一验收项目中通过原 OA 表单、正式入口、实际 IAM 和原生业务服务保存，再刷新、续办和核对历史。本批实际结果不是完整 OA 通过报告。

## 发布范围

合同、支付、开票的初始四镜像兼容闭包已经发布，Contract schema 18→22、Money 22→23、Billing 1，Project 11 不变。13 个应用数据库与 3 份文件／密钥树完成冷联合备份、空目标字节／事实／序列核对，并保存离机私有副本。此恢复未重新运行完整 Suite，不等于完整恢复运行验收。

Workbench `.2` 补经营路由，`.3` 修正资源管理能力名称并对开票申请当前项目 Scope。两次发布仅替换 Workbench，17 个主站容器健康，其余 16 个容器 ID、12 个业务库事实及序列不变。IAM 人类 Grant、Assignment Policy 和工作负载策略未因这两批修改。

## 实际失败，不计通过

1. 初始开票路由缺失，`.2` 修正。组件端口测试没有覆盖真实装配路由。
2. 三张列表读出不代表可新增：错误的 `project-workspace.resource.manage` 别名隐藏新增按钮。`.3` 改为实际 Manifest 的 `project-workspace.resource-link.manage`，新测试直接读取 Manifest。
3. 管理员没有 Billing 原生授权。身份管理员不隐含业务权限；已请求仅验收项目的明确授权，批准前不添加 Grant、不绕过 IAM。
4. 真实合同的 Base 与 OA 扩展保存成功，但 Link 返回 400：关系键包含冒号／下划线，不满足现有 `^[a-z][a-z0-9-]{0,63}$`。第二次续办仍被拒绝，没有重新创建合同。只有一条真实合同、没有关联，不算闭环。
5. 真实入口暴露 429。驱动节流不修改网关限流；用户连续操作的请求成本仍需检查。
6. `.4` 已把同一 Contract 关联到同一 C 项目，Project revision 14→15，但前端错误地把 POST 的 `ResourceLinkResult {project, resource_link}` 当作 GET Details。已提交的 Link 被显示为“回执尚未确认”。不得重新创建合同或项目；`.5` 候选校验真实单条命令回执，保存已确认阶段后再 GET，断读只继续读取。

## 修正与门禁

采用闭合的 14 个短键：合同 `reference-oa-contract-v1-{client|income}`；支付／开票分别为 `reference-oa-cash-v1-{client|income}-{design|survey|consultation}`、`reference-oa-invoice-v1-{client|income}-{design|survey|consultation}`。不放宽 Project，不改变 Money／Billing 事实。已知被拒的旧合同 Link 由用户显式点“修复无效关联并续办”，保留 Contract ID、Base 和扩展命令，只重新提出合法 Link 命令；旧被拒意图留在续办状态，不静默替换。

`.4` 的 90 项前端测试、TypeScript、Vite、Go 与独立启动已通过并切换 Main；其余 16 容器 ID 和业务事实保持。源归档摘要 `8bdce5272cfd642a92bd6ce2f26d007032791e2f089b79364bc35158e8c456d9`，离机冷备摘要 `e6b78fb82f376b036052db9422af26811dcd3f30625156601407fe001d472d0a`。实际续办暴露第 6 项错误，不能计合同链完成。原 Contract `dd3a4a04-abde-4a2e-8023-c007aac470a1` 和 Link 命令 `45a562d4-5c65-4458-897b-728b621bdf10` 保留，继续同事实验收。

成本页复用原 ProductionCostList／Form，显式选装 Cost 自有 `reference-oa-cost-v1`，不伪装通用四类成本。当前 103 项前端测试、TypeScript／Vite、Cost／Workbench Go 及 Workbench 两次独立启动通过；成本原组件端口测试 11 场景通过，明确不当公网证据。真实 PG18 扩展 6 项、通用成本 29 项，以及独立批次 d 的两项正式 Ed25519 IAM／HTTP 集成通过；较早失败／skip 批次不追认全组通过。

`.5` Cost＋Workbench 已通过门禁并于 2026-10-04 06:09 UTC 切换 Main，源归档摘要 `cd63ea61296fa49ea65e33941739ef748ae90c4ee7200abfd9e8debee47ce2a9`。Cost schema 2→3，显式选装 `reference-oa-cost-v1`；新侧别资源目录和窄 metadata 工作负载权限使用旧 IAM／Gateway 二进制解析，不改身份边界。17 服务健康，只重建 Cost、Workbench 以及加载新目录的 IAM／Gateway；其他 13 容器相对冷备重启后保持。13 库及 3 份文件树空目标恢复一致、旧事实／序列保持、迁移重跑零变化、Cost 使用自有恢复库独立启停两轮通过；未运行完整 Suite 恢复。离机冷备摘要 `334e60bf7a73f97810e59e19d7e2e65bb56e18e608fca7ef997cc8937f51288d`。人类 Grant 前的只读收尾确认两类 Grant、旧业务事实均保持；发布本身不写 Grant。

成本公网有限主链已通过：经营用车 1.01→1.03 的追加纠正、生产专家费 0 元和两侧隔离、交通成本 1.02 的实际上传／预览／字节精确下载、撤回后金额／文件／历史保留、刷新零业务写、Project revision 15 保持。前三条原生库核对为三条记录、五个命令／revision／审计；旧通用费用和十个其他业务库、旧 Evidence 事实／字节、用户101保持，只有批准的 14 条成本资源 Grant 和一个新文件，17 服务健康。

前两次驱动失败保留：第一次过早把处理中提示当作已丢回复；第二次恢复脚本在每次导航重置旧 sessionStorage，造成六次同一纠正命令的幂等重放，未增加原生事实。第三次修复为一次恢复；两个被驱动覆盖的已知验收意图按原序列化载荷、原 command ID 恢复后显式继续，不能把这种修复当正常自动恢复。第四次在同一项目追加一条独立连续恢复测试，正常浏览器刷新下的新建／纠正／撤回三次真实已提交丢回复全部原命令继续，金额 0.07→0.08→撤回，三版历史保持。合并九个驱动检查点、24 次可见命令（包括 Evidence 及失败阶段重复重放），不是 24 个业务事实。四条记录／八个命令的[最终原生收尾](../../acceptance/oa-initiation-repair/evidence/cost-public-20261004/closing-after-cost-public.json)已通过：十个其他业务库、旧通用成本、旧 Evidence 事实／字节、用户101及全租户 Grant 保持，17 服务健康，private PG／restore20 停止。[失败和复验](../../acceptance/oa-initiation-repair/evidence/cost-public-20261004/browser.json)、[可见预览](../../acceptance/oa-initiation-repair/evidence/cost-public-20261004/cost-file-preview.png)分别归档。这只是管理员有限成本链，不关闭全角色或完整 OA。

## 连续成本弹窗故障与 `.6` 实际修复

07:32 UTC `.6` 已上线。补全成本分类的真实连续操作发现：第二次正常撤回时，重新创建的 Drawer 遮挡仍复用的确认 Modal，确认按钮不能点击；不是接口已成功就算页面通过。旧 bundle 的原组件回归同样在第二轮失败，保留[旧失败](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/old-component-covered-confirmation.json)和[实际公网遮挡](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/public-covered-confirmation.png)。只给复用的成本 Drawer／Modal 显式设置 1000／1100 层级，不改成本规则、权限或数据。

修复 bundle 通过[12 个原组件端口场景](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/fixed-component-normal-confirmations.json)，包括三次正常新建→撤回→重新打开；`elementFromPoint` 只读检查真实点击目标，未强制点击或改 DOM。282 项 Node 测试零失败／skip、Workbench Go test／vet、TypeScript／Vite 和[两次独立启动](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/standalone-startup.json)通过。这些候选测试不计正式 IAM／PG／公网通过。测试夹具另补已有 Billing Controller 的显式上下文和拒绝反例，没有关闭鉴权。原 OA 47 份来源摘要一致，零写原目录。

源归档摘要 `0b725687b7d762d2d27272e14e862ca06f73b97edec1eba480a45242cf30ef59`；固定镜像摘要 `d396ab8837c0a2d8efa6c1a8b89dd1744e7ffa8f478e2b202dd1fedfceeca89a`。发布前 13 库事实／序列、3 份文件／密钥树在空目标恢复一致，离机备份 441854 字节，SHA256 `31b1ced571b74fbcafa75d9eada145ae8f8707942827680b45840ab134d0af2a`，私有副本不入仓库。[发布收尾](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/rollout.json)确认仅 Workbench 重建，其余 16 容器和 12 业务库事实保持，17 健康、D1 check 通过；schema、IAM／Gateway 策略与 override 不变，没有人类 Grant 写入。此空目标数据恢复仍不是完整 Suite 恢复运行。

分类公网驱动从已保存的三条矩阵记录继续，不重建项目、费用或文件。07:36 UTC 分类[原生收尾](../../acceptance/oa-initiation-repair/evidence/cost-category-public-20261004/native-closing.json)已通过：新增八条明确非真实费用的 0.01 验收记录全部正常撤回，两版历史／原金额保留，恰好 16 个新原生命令；合计十二记录、24 命令／revision／审计。结合此前四条有限链，覆盖经营七类和生产五类适用组合；经营投标报名费／中标服务费不出现在生产选择框。[实际公网驱动](../../acceptance/oa-initiation-repair/evidence/cost-category-public-20261004/browser.json)八场景、零页面错误，所有六次修复后的撤回确认均只读核对点击目标后正常点击；没有 force／DOM 修改。实取页面 script 的配置路径和 SHA256，确认公网正在执行修复 bundle，不用根路径猜测资产入口。

实际[经营列表](../../acceptance/oa-initiation-repair/evidence/cost-category-public-20261004/business-all-categories.png)与[生产列表](../../acceptance/oa-initiation-repair/evidence/cost-category-public-20261004/production-all-categories.png)已视觉检查；原成本统计分别保持 1.03／0 元的未撤回费用，撤回记录仍显示原金额，不参与有效总额。两侧新建抽屉取消零写；八条撤回记录正常重新打开为只读，并逐条展开两版历史。旧四条成本及历史、通用成本、其他十个业务库、完整 Evidence 事实及审计、两类 Grant 完全不变，Project revision 18，17 主服务健康、private PG／restore20 停止。未上传新文件、未扩大授权，分类覆盖不等于全角色／完整 OA。

较早三次选择框快照竞态、一次误等已经正常关闭的编辑抽屉、一次实际弹层遮挡和一次驱动错误猜根资产路径的 404 均保留。最后一次失败在任何新业务写入前发生；改为读取页面实际 script 路径后续原记录通过。没有改原生数据来通过断言。生产截图上方仍明确显示阶段／任务工作面未接线，不能把下方成本列表完成当作生产全过程完成。

## 同一 C 的合同、支付有限公网链与可见复读

07:06 UTC 原生收尾实际通过，Project revision 18。原甲方 Contract `dd3a4a04-abde-4a2e-8023-c007aac470a1` 没有重建；第二链 Contract `beb67167-60a8-446b-a6ca-744bb2ddfde9` 仍是草稿。甲方原始分项 1.01＋0.09＋0.90＝2.00，正式确认后依次确认调整＋0.01、修订目标 2.02、无金额变化三种协议。原始 Contract 行仍为 2.00，有效 revision 4 为 2.02；不能修改原行来“修正”验收。阶段定案 1.01、最终定案 2.02 各冻结两方、来源 revision 4 和确认日期。第一次取消定案零新增，显式确认后才有原生记录。

两个真实 Cash 分别为甲方实付 1.01 和公司实收 2.02。第一次创建在真实提交后故意丢回复，刷新后通过页面“继续原支付操作”重放原 command ID，仅产生一条事实；甲方记录随后明确原因冲销，原金额 1.01 保留、有效金额 0，公司实收 2.02 保持。两个列表和原抽屉实际可见，不跳其他系统；来源实际金额不由 Project Link 生成。

真实样本 `公网验收合同-35jPS3.png` 经 Evidence 上传、完成校验、登记 Contract 引用、字节精确下载。后续只读浏览器复验实际载入 640×400 预览，并分别拍摄三协议和两定案的历史区域，而非仅截抽屉顶端。[可见历史与预览复验](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/visible.json)、[三协议](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/contract-amendment-history.png)、[两次定案](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/contract-settlement-history.png)、[实际预览](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/contract-visible-preview.png)已归档。只读复验零业务写、零页面错误，Project 前后相同；只允许既有 Gateway 换取能力及该文件访问许可。

[真实入口驱动](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/browser.json)九个检查点通过、34 次可见命令包含故障阶段重放，不是 34 个事实。整体 `passed:false` 保留：开票阶段被独立业务授权阻断，不能把合同／支付有限通过冒充整个经营链通过。[原生事实与边界收尾](../../acceptance/oa-initiation-repair/evidence/commercial-public-20261004/native-footprint.json)核对两合同、三协议、两定案、四个冻结主体、两 Cash 与一冲销；其他六库完整不变，Contract／Money／Project／Billing／Evidence 的相邻行及旧文件树不变，C 只增加三条已确认资源 Link 和对应历史，用户101不变。仅批准的 14 条 Cost Resource Grant；Tenant Grant 未变。成功登录只推进 admin 的 `updated_at`，密码、状态、generation 和所有其他账号字段未变。主站 17 健康，private PG／restore20 停止；未验全角色或完整 Suite 恢复运行。

失败驱动和诊断也保留：首次只读复验在 Gateway 正常 bootstrap 前读取 Project，误遇 HTML 重定向；第二次关闭预览后动态 `:visible.last()` 改指向父合同抽屉，造成等待超时，修正为观察预览自身隐藏后第三次通过。原生收尾第一次错误地把成功登录时间当业务改动，第二次错误地要求原始 Contract 行等于有效 revision 金额；均根据领域代码和只读原生诊断修正断言，未改实际业务数据，也未排除整个账号或合同表。公开归档只含安全回执与截图，不含会话、凭据或备份。

## 两链三费用的原支付表单取消复验

07:43 UTC [六个公网原表单场景](../../acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004/browser.json)通过：甲方／公司两链分别打开设计费、勘察费、咨询费，核对原主体／账户／依据／金额／备注控件，填 0.01 后只点取消；保留原两 Cash 和 Project revision 18。请求防护会拒绝意外业务写入，但实际没有任何写入请求被阻止，零页面错误；并非拦截写请求冒充取消通过。实际[公司咨询费表单](../../acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004/income-2-form.png)已视觉检查，关联发票仍明确禁用／未接通，不计资金开票关联完成。[原生收尾](../../acceptance/oa-initiation-repair/evidence/cash-cancel-public-20261004/native-closing.json)确认十二业务库事实／序列和两类 Grant 完全不变，17 健康、private PG／restore20 停止。取消矩阵只证明原可见表单和零写，不替代六组合保存、开票关联或全角色验证。

本轮静态边界审计及其负例自测再次通过：15 应用独立 `GOWORK=off` 解析、零跨应用源码依赖／replace、零跨库 SQL 机制；[日志](../../acceptance/oa-initiation-repair/evidence/cost-modal-20261004/boundary-audit.log)。它不能代替运行期边界、D2～D4、长期跨客户升级或完整 OA 验收。

## 验收授权记录

用户明确批准：通过正式 IAM，仅给 admin、验收项目 C 的经营／生产成本读取、金额读取、登记、纠正、撤回、附件和历史权限；并允许后续逐个验收项目同样授权。实施必须逐个识别验收项目，再授予两侧精确 Project ID Scope，不扩成 Tenant、不包括真实客户项目、不推及 Billing 或其他应用。发布脚本不写人类 Grant；原生 IAM 授权另有独立回执并核对其他 Grant／业务事实不变。Billing 授权请求仍待单独答复。

该批准已执行：原生 IAM 七次 POST 均返回 204，产生 C 的 `business_cost_project`／`production_cost_project` 两侧各七项精确资源授权，共 14 行。其他 Resource Grant、全部 Tenant Grant、十二个业务数据库事实与序列保持。后续项目批准是逐项目授权范围，不是租户级预授权；本驱动只实现 C。

## 整体缺口

以[完整操作清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)为准：全字段、全角色、合同最终更正、资金与开票关联、成本完整角色／负例、奖金、生产各类型／阶段、成果、外委、资料与统计、OSS 文件与前端分离、完整连续旅程及完整恢复运行均保留缺口。后台测试不能代替可见界面、连续点击和持久化证据。

## 四小时上限暂停交接

本续轮 03:55 UTC 开始，07:50 UTC 收尾，按用户要求在四小时上限前暂停后续实施／验收，完整目标不标完成。最新实际 D1 check 为 07:45 UTC exit 0；07:46 UTC 容器清单为 17 个健康主服务，private PG／restore20 未运行。07:48 UTC 差异空白检查和关键文档 35 个本地引用检查通过。保留已保存验收事实、失败回执、截图和离机备份，不新增租户／Billing／Bonus 授权，不重跑旧驱动来重建事实。恢复时继续同一 C 的已保存合同、资金和附件；Billing 等待独立授权答复，奖金的固定额／手填基数／多角色兼容扩展、分类型生产和其余全站缺口仍需实施与真实 UI 验收。
