# 统计与多层权限配置 UI v1

日期：2026-10-07；关联 BF-121。状态：本地候选实现，实际范围见[验证报告](../../reports/oa-statistics-permission-ui-validation-20261007.md)；真实环境未验。

问题 A → 可验证结果 B：统计只有样例表格，权限模板属性和多层关系难以操作 → 在正常 Workbench 中提供可筛选、可展开、可导出的统计工作面，以及可审阅、保存草稿和导出的分层权限配置。

## 本轮范围

用户明确优先完成统计 UI 和交互、按既定多层设计展示权限配置；项目详情问题延后。这一优先级取代 BF-120 快照中的紧邻实施顺序，不取代基线审计、最终真实环境验收或应用边界。

统计沿用原 OA 五个入口：项目台账、公司统计、人员奖金统计可读取现有来源；考勤与薪酬在原系统未实现，保留准确空状态。台账支持关键词、费用、经营链、记录状态、日期筛选、分页、明细抽屉和分别导出支付／开票。公司统计显示甲方→资质单位、资质单位→我司、未分类三个区域及费用分栏矩阵；已核销和未清余额只用 Money 原生结果，合同不能冒充确认应收。选装 Cost 时按精确项目／经营生产侧读取成本登记和明细；成本不冒充付款，也不与外委或付款重复相加。人员奖金按人员展开项目、经营／生产类别、日期和登记金额，同人不同币种分别汇总。

权限沿用原 OA 49 项规则、7 个分组和全部职位／项目角色，展示身份治理、职位模板、项目角色／临时授权、原生能力／最终授权四层。支持标识／名称／范围／排序／备注、权限搜索、启用／差异／高风险过滤、分组选取、恢复默认、撤销、未保存切换确认、变更审阅、自定义职位、新增／删除配置草稿和导出。第四层展示已登记角色映射和当前组合所选应用的本地 Capability Manifest；无 Overlay 的应用单独说明，目录不代表部署、选装扩展或持有 Grant。身份治理不隐含业务权限；管理员模板全部规则保持锁定。

## 事实与权限约束

- Project 提供可见目录和已有 commercial-summary；Contract、Money、Billing、Cost 和 Bonus 保持各自事实所有权。目录中的项目包不和其子项目重复计财务记录；同一来源资源被多个项目引用时排除公司汇总并提示，未确认分摊口径不得重复计数。
- `reference-oa-statistics-v1` 是客户侧临时只读投影定义：按来源、记录种类、方向、经营链、费用和币种分别加总已经由来源返回的精确金额。不得计算合同−成本结余、合同−实收应收，或跨来源合计。未分类不猜分类，未授权／未知／故障不变成零。日期筛选只包含具有来源记录日期的行，无日期的合同不伪造日期。
- 金额使用十进制字符串和整数分／BigInt；不安全的 JSON int64 开票值显示未知。Bonus 仅统计 `recorded` 的精确项目／人员／类别记录，撤回不计入；登记不等于支付。
- 来源故障在页面单独说明，已有可读来源继续显示；返回、刷新与快速切换不允许迟到请求覆盖当前页签。普通台账只读取一页项目，公司／奖金页显式读取可见目录；当前没有服务端全租户统计产品或跨库查询。
- 导出使用真实来源 `export-view` 再授权并核对来源 ID、Tenant、币种、金额及 revision；变化后要求刷新，拒绝后不下载。Money 复用 `money-flow-ledger.export` 并兼容声明 `cash_transaction` Resource Type（原 `party` 保留）；Billing 增加 `billing-invoice.export`，均还要求金额读取与资源范围。没有自动新增 Grant。Excel 使用 SpreadsheetML XML，文件名 `.xml`，所有值为字符串单元格以保留精确数值并阻止公式注入。
- 权限页面显示的 Overlay／Assignment Policy 是仓库版本化配置参考，不是当前运行环境的 Grant。草稿与真实授权分开显示；草稿只存于 Tenant＋Subject 隔离的浏览器会话，可导出给正式配置流程。
- 当前 IAM 没有职位模板／项目临时授权配置写 API；本轮没有伪造这类写入成功，也不让 Workbench 变成授权事实所有者。正式应用配置、实际 Grant 管理和临时授权写端口仍需后续独立实现／验收。

## 实现与验证入口

- `frontend/oms-workbench/frontend/src/statistics-model.mjs`、`statistics-workspace.tsx`、`permission-workspace.tsx`，随既有 Workbench Vite 制品嵌入。
- Money：`GET /api/v1/cash-transactions/{id}/export-view`；Billing：`GET /api/v1/invoice-registrations/{id}/export-view`。均复用来源自身读取和最终授权，没有 migration 或业务写命令。
- 前端：`npm test`、`npm run check`、`npm run build`。
- 来源保护：Billing HTTP 全量及新增 export 权限／金额／相邻资源矩阵，Money HTTP 全量及相同导出矩阵。
- 浏览器夹具：`node acceptance/oa-initiation-repair/verify-statistics-permissions-ui.mjs`；证据保存在 `acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/`，它不代表正式 IAM、PostgreSQL 或 D1 已通过。
- 发布需携带更新后的 Money／Billing Capability Manifest 和相容制品，并单独验证所声明 Deployment Profile；当前未执行部署。
