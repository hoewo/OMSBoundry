# OA 管理员全权限授权与验证（2026-10-05）

## 目标

用户明确要求 OA 管理员拥有当前 OA 的全部权限。这里的“全部”定义为：当前 Suite 中 12 个已安装应用通过版本化 Capability Manifest 发布的每一项能力，均以 tenant scope 显式授予指定 OA 管理员。

这不是把 IAM 的 `administrator` 标志变成业务绕过，也不是让前端按钮决定权限。IAM 仍保存逐 Audience／Action 的正式 Grant、签发短期 Credential 并记录审计；目标应用仍按资源类型、业务状态、expected revision 和事实所有权最终授权。

## 实施前发现

- 当前加载 12 份 Capability Manifest，共 132 项能力；
- `admin` Subject `12e800ac-00bf-4516-8748-b45c5623493a` 为 active IAM administrator；
- 它原有 83 项 active tenant Grant；
- 还缺 49 项，主要来自后续启用的 Billing、Approval & Audit Results、Cost & Outsourcing 和 Bonus Distribution 能力。

原因不是 Capability Catalog 缺失，而是后续模块／Overlay 发布后没有再次执行首位管理员能力对齐。

## 实施方法

运行：

```sh
node acceptance/oa-initiation-repair/grant-oa-admin-full-access.mjs /tmp/oa-admin-full-access.<mktemp后缀>
```

脚本从运行中的 IAM 只读取得实际装载的 Manifest，通过公网正式 IAM 登录，并只对缺失的 49 项调用 `POST /api/v1/admin/grants`；Scope 为 tenant。它不直写 IAM 数据库，不调用业务写 API，也不输出密码、Cookie 或 Credential。

## 验证结果

- 49／49 缺失能力通过正式 IAM 创建，49 条 grant audit 同步增加；
- 管理员 active tenant Grant 从 83 增至 132，与当前 Catalog 132／132 精确相等；
- 对 12 个 Audience 分别请求 tenant-scope Credential，签发动作数量依次为 9、17、9、6、21、4、8、8、9、7、18、16，逐项与对应 Manifest 完全一致；
- 12 个业务数据库 canonical facts／sequence 不变；
- 其他 Subject 的 tenant Grant 不变；
- 全部 resource-scoped Grant 不变；
- 17 个运行容器 ID、镜像与健康状态不变；
- 没有触发 429，也没有新增业务事实。

后续 Project、Production 与 Bonus Capability Manifest 增加原生动作后又执行一次同一对齐入口。2026-10-05 17:49（UTC+8）的最新结果为：12 份 Manifest 共 145 项能力，管理员已有 145／145 项显式 tenant Grant，本轮创建 0 项；12 个 Audience 的 Credential 再次逐项与当前 Manifest 相等。最新动作数为 9、17、9、6、25、4、8、8、13、7、18、21。该复验说明管理员配置已跟上当前 Catalog，而不是继续停在第一次的 132 项快照。

公网 UI 又完成两层独立验收：OA 规则页显示 7 组、49／49 全选且管理员模板不可部分取消；生产页逐项打开人员、批复审计、方案、成本、外委和奖金，人员初始化可用，外委“新增对外委托”和奖金“分配奖金”不仅 enabled，两个真实抽屉都已打开。奖金候选读取最终为 `200`，当前项目因尚未初始化生产人员而返回空 `items`；0 个 403／429／5xx／页面错误，且只读验收没有发出业务写请求。

这一闭包不是单纯补 Grant。首先，Bonus 首次 403 的根因是 Gateway 仍挂旧 `bonus-pool.read` 单入口，已改成 v3 的 `bonus-pool.read`／`bonus-manual-allocation.read` ANY 入场集合；随后 503 的根因是正式 IAM Workload Policy 没有给 `bonus-distribution` 配置 `project-workspace.reference-oa-personnel.read`，已按版本化策略补齐。最后，Project 对真实存在但尚未选装生产人员 profile 的候选列表错误返回 404，导致 Bonus 翻译为 422；现改为列表 `200` 空集、精确分工仍 `404`。三层修改分别属于入口装配、工作负载身份和 Project 自有只读端口，没有把 OA 岗位规则、IAM Grant 或目标应用最终授权合并。全部滚动后仍为 17／17 healthy，业务事实与 Grant 摘要不变。

结构化回执见 [oa-admin-full-access-20261005.json](../../acceptance/oa-initiation-repair/evidence/oa-admin-full-access-20261005.json) 与 [oa-admin-native-ui-closure-20261005.json](../../acceptance/oa-initiation-repair/evidence/oa-admin-native-ui-closure-20261005.json)。

批复／审计页在没有冻结合同来源或既有有效批复时仍会禁用相应登记按钮。这是目标应用的业务前置条件，不是管理员权限缺口；不得通过扩大 IAM Grant 绕过。初步设计、施工图设计和变更竣工后来按 BF-110 拆开“页面可导航”和“原生端口已接通”：Tab 已可进入原 OA 面板，但真实读写仍明确待接，不由管理员权限假装实现；见[后续导航修复报告](./oa-production-stage-navigation-validation-20261005.md)。

## 持续规则

`reference-oa-administrator-v1` 是显式客户装配权限画像，不是所有 IAM 身份管理员的隐式业务权限。每次新增应用、升级 Capability Manifest 或增加原生 Action 后，部署闭包必须再次运行 `deployments/d1/ops/grant-bootstrap-access`（或等价的 Catalog 对齐），并验证该 OA 管理员的新 Credential 与已安装 Catalog 完全一致。

“全部权限”仍不代表可以执行业务上无效的动作。例如已经终结的事实不能被静默覆盖、expected revision 错误仍应冲突、跨 Tenant 仍应拒绝、追加历史仍不可物理删除。
