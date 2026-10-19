# 统计与多层权限配置 UI 本地验证

日期：2026-10-07；关联 BF-121。[实现规格](../specs/assembly/oa-statistics-permission-ui-v1.md)。

结论：当前工作树中的统计／权限 UI、只读来源适配和独立导出保护获得本地证据；不是正式 IAM、真实源库、D1、C7／C8 或完整 OA 验收通过。未提交干净版本，未部署或修改现有 Grant／生产数据。项目详情问题保持延后。

## 本轮交付

- 统计保留原 OA 五个入口。项目台账支持项目名称／编号、费用、经营链、状态和日期筛选，分页、台账抽屉，支付／开票分别授权导出 Excel XML。应收应付显示 Money 原生已核销和未清余额，不从合同推算。
- 公司统计包含两条经营链和未分类区域、费用分栏矩阵、选装 Cost 的经营／生产成本与来源明细下钻。已撤回成本不计入，登记不是付款；同一来源资源关联多个项目时提示并排除汇总，防止重复计数。
- 人员奖金按人员展开经营／生产记录；精确整数分按币种分别加总，金额缺权或来源不可用不变成零。奖金应用未选装时显示缺席，不要求其启动。原 OA 未实现的考勤、薪酬保持准确空状态；项目结余没有已验证来源口径，不能以跨应用金额简单相减冒充结果。
- 权限保留原 OA 49 项／7 组／全部职位和项目角色，展示身份治理、职位模板、项目角色／临时授权、原生能力／最终授权四层。补齐标识、名称、范围、排序、备注、自定义职位、搜索、差异、高风险、分组选取、恢复默认、撤销、切换确认、改动审阅及草稿导出。
- 配置仅保存 Tenant＋Subject 隔离的浏览器会话草稿；校验系统模板完整、管理员全部权限、重复键及字段边界。能力目录是当前组合所选应用的本地 Manifest，已登记 Overlay／Policy 与无映射应用分别显示，不宣称是当前 Grant。
- Money／Billing 增加自身保护的 GET `export-view`。Money 原 export 增加精确 cash_transaction 范围并保留 party，Billing 增加独立 export；均检查金额读取与资源范围，Workbench 再核对来源 ID、Tenant、金额、币种及 revision。不自动授予权限。

界面按现有工作台和 frontend-skill 的应用界面指导保持克制：统计以矩阵／明细为主，权限以模板列表／编辑器为主；不加入装饰图表或虚构 KPI。旧统计演示块进入惰性模板；React 加载失败也不把样例金额当真实来源展示。

## 实际运行

| 检查 | 实际结果与证明范围 |
| --- | --- |
| Workbench frontend `npm run check`、`npm test`、`npm run build` | TypeScript、206／206 Node 测试、Vite 构建通过；覆盖精确金额、来源／币种隔离、原生余额、重复资源排除、成本／奖金范围、草稿完整性及生产桥接的精确授权请求 |
| Unified Workbench 普通 `go test -count=1 -p 1 ./...`／`go vet ./...` | 通过；包含嵌入资源、真实页面挂载桥接和统计安全 fallback；不等于正式 Gateway／IAM 联验 |
| Billing 普通 `go test -count=1 -p 1 ./...`／`go vet ./...` | 通过；新增 HTTP export／amount／精确范围矩阵 |
| Money 普通 HTTP `go test -count=1 -p 1 ./internal/httpserver`／`go vet ./internal/httpserver` | 通过；新增读权限不能导出、export＋amount＋精确资源允许、相邻资源和撤回金额权限拒绝矩阵 |
| access-contract 普通 `go test -count=1 -p 1 ./...`／`go vet ./...` | 通过；新增两来源 export 资源类型及旧 Money party 兼容断言 |
| `make architecture-boundary-static-audit` | 自测与审计通过：15 应用，跨应用 import／replace、跨库 SQL 机制均为 0 |
| 独立 Workbench 启动与 `/health/ready` | 通过；本机回环端口、现有 local Suite Manifest，无源库或业务写入 |
| Chrome 浏览器 UI 契约夹具＋独立 Workbench 预览导航 | 完整重跑 24／24，通过；页面错误 0，390px 窄屏整页宽度 390px，末项权限可滚动到达，预览受保护请求 0 |

Go 的普通检查未启用真实 PostgreSQL 集成环境／integration tag，默认跳过的条件项不得计入真库通过。浏览器来源为明确的内存契约夹具；它执行实际 Vite 制品和生产桥接，但不把替身视为真实 IAM 或业务持久化。

浏览器入口：

```sh
node acceptance/oa-initiation-repair/verify-statistics-permissions-ui.mjs http://127.0.0.1:19117
```

可省略最后的 URL，仅验 UI 契约夹具；提供 URL 时只接受本机回环工作台，并增加正常侧栏／设置／返回导航及预览零受保护请求检查。浏览器脚本使用本机已安装 Chrome 和 Codex Playwright 运行库，测试结束关闭自建浏览器和夹具 HTTP 服务。

持久证据：[结构化回执](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/ui-fixture.json)、[公司统计](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/statistics-company.png)、[权限编辑器](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/permission-editor.png)、[权限窄屏](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/permission-mobile.png)、[窄屏末项](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/permission-mobile-bottom.png)、[正常工作台预览](../../acceptance/oa-initiation-repair/evidence/statistics-permissions-ui-20261007/workbench-statistics-preview.png)。同目录保存支付／开票 XML 和带 `draft-not-applied` 标记的配置导出。

结构化回执 SHA-256：`9079552a56c16a7f4031efc7a837a617c4bb46fa107d911718e6fbb9d8edf9d2`。

## 失败与修正

早轮浏览器检查遇到 Ant Design 两字按钮可访问名称含空格、抽屉关闭按钮中文化、确认标题／保留的隐藏表单命中多个节点；定位修正后均重新从头运行，没有拼接局部成功。扩展窄屏检查真实发现 rail 的 content-box 宽度超出 18px，修正为 border-box 后回执验证 390／390；修正前截图保留为 `failed-mobile-before-box-sizing.png`。保存意图切换／放弃逻辑同步修正，导出不再静默忽略当前未保存改动。

## 下一步与未完成范围

1. 固定干净可追溯检查点；用同一匹配的 Workbench、Money／Billing 制品和 Capability Manifest，验证真实 IAM／同一项目的来源、读与导出分权、撤权、跨 Tenant、源故障和恢复。未经允许不更新线上环境或补授 Grant。
2. 正式职位模板配置和临时授权写端口仍不存在，须按独立所有者先明确事实、不变量、配置 revision 和审计，再形成纵向闭环；本页草稿不能作为“正式权限已配置”交付。
3. 明确来源拥有的项目结余及统计口径；若要新增考勤／薪酬，须另行定义业务范围。资料、个人工作、C7／C8、D1／恢复、第二客户及完整 OA 仍按 BF-120 基线审计推进。

本轮仅是用户优先的本地 UI 候选，不改变 AGENTS.md 中 Bonus 的完成度，不关闭总体 `insufficient-evidence`。
