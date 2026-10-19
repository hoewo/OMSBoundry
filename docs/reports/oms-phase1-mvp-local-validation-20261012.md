# OMS 一期 MVP 实现、发布与公网验证报告

> 历史快照：本文冻结 BF-149 截至 2026-10-12 的状态，其中“Money／Evidence／施工方应付未完成”已由 2026-10-13 BF-150 关闭。当前结论请以[六 Owner 公网写入与边界验证报告](./oms-phase1-public-write-validation-20261013.md)为准。

日期：2026-10-12  
关联：BF-145、BF-148、[`oms-phase1-mvp-product-design-v1.md`](../specs/assembly/oms-phase1-mvp-product-design-v1.md)、[`oms-phase1-small-system-capability-gap-v1.md`](../specs/assembly/oms-phase1-small-system-capability-gap-v1.md)

状态：**本地正式 D1 与 `oatest.antuinfo.cn` 公网升级、登录和五 Owner 读取验收均通过。** 当前已经证明产品信息架构、受保护入口、真实 Owner 读取组合，以及 Party、Project、Contract、Billing 的原生命令可以在真实 IAM 下形成最小纵向旅程；Money 实际资金写入、附件和施工方应付纵向旅程仍未完成，不能把它描述成 OMS 一期全部真实业务闭环已经完成。

## 1. 问题与结果

问题 A：原 OMS 演示很全但链路不清晰，验证原型又不足以承担真实业务 → 可验证结果 B：在不新建 OMS 共享业务库的前提下，形成四入口的一期工作面，并通过 Gateway、IAM 和各事实 Owner 的版本化 API 读取及写入各自真实业务事实。

## 2. 本地已经取得的证据

- Gateway 新增 `/oms-v1` 与 `/oms-v1/`，只跳转到受 IAM 保护的 `/workbench/?view=oms-v1`；旧 `/oms/` 只读参考入口保持不变。
- 正式本地 D1 已启用 IAM、Gateway、Workbench、Nginx、Party、Project、Contract、Billing、Money 与 Evidence。真实登录后创建资质单位与甲方主体、新建 Project、创建 800 万 Contract 并建立 Project Resource Link；经营总览从 Owner 回读 1 个项目和 800 万未回，不使用浏览器静态业务值。
- 新建项目采用 `Project → Contract → Project Link` 三段原生命令有界编排，每段使用独立 Command ID 并由目标 Owner 最终授权。已经成功的前段不会被后段静默回滚或跨库覆盖；后续仍需补可视化续办和异常恢复旅程。
- Billing 已在同一登录态登记并复读 320 万开票。由于当前 Project 仍为 draft，不能冒充 Billing 已支持的类型化正式 Project 来源；页面使用 Billing 原生 `manual_confirmation` 来源快照冻结 `OMS项目 <稳定ID> · <名称>`，同时冻结双方 Party revision。该兼容不修改 Billing 生命周期，也不会自动生成应收或回款。
- Workbench 增加经营总览、项目经营、应收回款、应付支付四个一级入口；桌面使用左侧导航，窄屏使用底部导航。
- 二级导航已按 BF-146 收入对应一级菜单：桌面侧栏只展开当前组的三个二级入口，项目／应收／应付九个入口逐项点击均定位正确页面，内容区不再重复 Tab；移动端使用底部一级导航与顶栏下紧凑二级导航保持同等可达性。
- 项目、主体、票据和实际资金分别通过 Project Workspace、Party Directory、Billing、Money 的授权入口读取；页面不直连数据库、不复制 Owner 表。
- 应收／应付列表已接 Project 原生 `commercial-summary` 只读组合：只展示项目已稳定关联的 Money obligation，金额来自精确十进制投影，按 Project Link 建立时间倒序；Money reference 为主体快照显式标注 `creditor／debtor`，Project 只瞬时透传，Workbench 据此显示甲方／施工方，不按名称推断。
- 项目、甲方、施工方、应收、应付、开票、收票、回款和支付的当前已加载结果统一使用 20／50／100 条前端分页；记录不超过一页时自动隐藏分页条，避免占用首屏。项目可按甲方、施工方、回款状态和支付状态筛选，应收／应付可按项目和资金状态筛选，票据／资金可按项目、业务日期和有效状态筛选；条件收在统一抽屉内，不占列表首屏。上述能力只作用于当前已加载结果，不替代 Owner 全量游标分页与服务端筛选。
- 应收、应付、开票、收票、回款和支付六类列表已提供统一导出窗口；导出沿用当前搜索／筛选结果、不受列表分页限制，金额使用精确两位小数文本，用户文本做 CSV 公式注入防护。生成的是可由 Excel 直接打开的 UTF-8 CSV，窗口明确提示其只覆盖当前已经安全读取的 Owner 记录，并非后台全量导出。
- Billing 开票／收票页面已经具备双方向搜索、紧凑筛选入口、记录总数和统一详情抽屉；详情展示冻结的项目来源、开收票主体、业务日期、精确金额、状态、备注与附件数量，不展示内部编号。
- Billing 新增开票／收票表单已经接到原生 `POST /api/v1/invoice-registrations`。部署通过规范小写 UUID 的 `OMS_QUALIFICATION_PARTY_ID` 显式声明资质单位；项目选择甲方或 `contractor-party` 施工方，保存使用随机 Command ID、精确整数分以及 Project／双方 Party exact revision。配置为空／非法、主体不可用／停用、项目无对应往来方或 Billing 不返回 `record` 时，提交失败关闭，不按名称猜测。
- 开票／收票详情已经接到 Billing 原生 correction 命令，可更正业务日期、金额和备注并提交 expected revision；不允许在该表单换项目、开票方或收票方。金额无查看权限时页面不呈现或回传金额字段，Owner 不返回 `correct` 或记录已失效时保存按钮禁用。
- 金额格式统一到人民币两位小数；Money 十进制字符串和 Billing 最小货币单位均避免二进制浮点计算。
- Money 新增受 IAM、Tenant Scope 和最终授权保护的 `GET /api/v1/cash-transactions`；默认只读有效流水，支持类型、状态、业务日期、精确来源引用和有界分页，缺少金额能力时金额继续脱敏。
- Project 在既有 Resource Link 内增加 `party-directory/party` 允许项；施工方关系使用 `contractor-party` 和精确 Party revision，写入前由 Party Directory reference-read 复验。项目目录返回授权范围内的当前有效 Link，Workbench 据此区分施工方，不复制 Party 主档或按名称猜角色。
- 项目详情继续使用完整页面但不复制财务明细：甲方侧和施工方侧分别以“总额—票据—已发生—余额”显示只读摘要。义务、核销和余额来自 Project 对 Money 的经营投影；票据只显示当前已加载记录数，并在资质单位 Party ID 未配置时明确显示“待配置”。
- Workbench 的“新增主体档案”已经接到 Party Directory 原生创建命令，只写名称、主体类型和登记标识；同名时强提醒但允许用户确认继续。甲方／施工方详情抽屉也已接原生更正命令，使用 expected revision 追加当前档案 revision；没有权限或主体已停用时失败关闭。创建或更正主体不会自动改变项目角色，也不会重写历史项目、票据或资金快照。
- 项目详情的“编辑项目”已经接到 Project 原生 `PUT /api/v1/projects/{id}`，只开放名称、开启日期和说明，提交 expected revision 并原样保留 Owner 已有的内部项目号、类型和计划结束日期；不在这个命令中修改甲方、应收或应付。缺少原始字段或 `project-workspace.update` 能力时失败关闭，同名项目强提醒但允许确认保存。
- 项目详情的“关联施工方”已经接到 Project 原生 Resource Link 命令，提交稳定 Party ID、精确 Party revision、Project expected revision 和 `contractor-party` 关系键；无权限或无可选主体时明确禁用。
- 经营总览的回款／支付圆环读取 Project commercial summary 中的有效义务，以精确金额计算已核销比例、已发生和余额；项目资金条显示每个项目的回款比例与未回金额，点击进入项目详情。月度收支图不携带静态示意高度；Money 可用且有金额权限时，使用精确十进制转整数分后按业务日期汇总十二个月，Money 不可用或金额脱敏时明确显示不可用／无权限并保持空图。
- 本地浏览器已检查桌面与窄屏布局。第一次极窄视口复核发现项目标题被按钮挤成竖排，修复为标题弹性占位、按钮不换行以及甲方／施工方说明分层；本轮又在 390×844 视口实际打开应收页和导出确认窗，并在浏览器实际折算为 223px 的极窄内容宽度下复核开票列表，搜索、筛选、导出、新增、横向列表及四项底部导航保持可操作。全局“架构透镜”入口在 OMS 移动端改为底部导航上方的 36px 图标，避免遮挡业务入口；浏览器 warning／error 为 0。

已实际运行：

```text
node --check frontend/oms-workbench/internal/server/oms_phase1.js
go test ./frontend/oms-workbench/...
go test ./backend/platform/gateway/internal/gateway ./frontend/oms-workbench/internal/config ./frontend/oms-workbench/internal/server ./backend/services/money-flow-ledger/internal/httpserver
go test ./backend/services/project-workspace/internal/references ./backend/services/project-workspace/internal/projects ./backend/services/project-workspace/internal/httpserver
go test ./backend/services/billing-invoice/internal/httpserver ./backend/services/billing-invoice/internal/invoice
go test -tags=integration ./backend/services/project-workspace/internal/projects -run TestPostgresDirectoryPaginationHierarchyScopeAndReadOnly -count=1
git diff --check -- <本批代码与文档范围>
deployments/d1/ops/test-release-bundle
D1_PROJECT_NAME=oms-v1-integration D1_ENV_FILE=deployments/d1/.runtime/oms-v1-local.env deployments/d1/ops/d1 check
cd dist && shasum -a 256 -c omsbizfoundry-d1_v0.1.0-test.oms-v1.1_linux_amd64.tar.gz.sha256
cd <解包目录> && shasum -a 256 -c RELEASE-CHECKSUMS.sha256
```

结果均通过。没有把未执行的空目标恢复、真实生产资料或公网测试写成通过。

## 3. 当前明确未完成

- 甲方主体、Project、Contract 合同应收和 Billing 开票已在本地正式 IAM 组合中完成真实写入与复读；施工方应付、Money 回款／支付、Evidence 附件仍未形成同等级纵向写入旅程，因此仍不能计为 OMS 一期全部真实业务闭环通过。
- Party Directory 继续只拥有主体主档；Workbench 已开放主体档案创建。甲方由 Project 创建时快照确定，施工方由 Project `contractor-party` Link 持有，Workbench 已开放一次只关联一个施工方的原生命令；尚未做移除施工方和下游合同编排。
- 新建项目已经把页面应收总额保存为 Contract 承诺并建立稳定 Project Link；它不是 Money 已确认义务，页面不把两者冒充为同一事实。新增施工方后的下游合同／应付编排尚未闭合，Project／Contract 任一后段失败的用户可见续办也仍待补齐。
- 应收／应付当前按 Project Link 的 `linked_at` 隐式排序，不在列表展示创建／更新时间；页面分页和本地 CSV 导出目前只作用于各 Owner 首批最多 100 条已加载结果，Owner 级全量翻页、筛选、原生 `.xlsx` 和完整导出仍待接通，不能把首批读取冒充全量记录。
- Money 的项目归属只接受稳定 `source_reference`；缺失时显示“项目来源待核对”，不按名称猜测。
- 公网部署已通过 Party Directory 正式接口建立“北京京宏勘察设计有限公司”，稳定 Party ID 为 `95d3793f-7e0d-4ec9-acb8-9baf8bcb1039`，并由 `OMS_QUALIFICATION_PARTY_ID` 显式注入正式 Workbench；Demo 预览继续不得暴露该值。该主体名称与稳定 ID 已验证，不代表统一社会信用代码等未提供资料已经确认。
- 公网 D1 已同时启用 Contract、Money、Party、Project、Evidence 与 Billing 并通过全栈 `check`；Money 实际资金命令仍按主体 runtime cutover 门禁保持关闭，不用前端假写绕过。

## 4. 小系统边界评估

总体结论：**不是全盘重构，但有两个不能靠前端绕过的显著边界缺口。**

1. Contract／Money 的本地 Party 运行时要切到 Party Directory 稳定 ID；未完成前不能从 OMS 发起会产生双重主体事实的写入。
2. Money 当前义务模型的固定 `due_on` 不适合“分批回款、期限不确定”的业务；一期不能伪造到期日。实际 Cash Transaction 可以复用，义务／应收应付模型需单独兼容演进。

此次实现没有新增 OMS 数据库、共享业务表、跨库读取、名称匹配、migration 或双写。新增的 Money 主体角色和 Project 经营摘要字段均是可选只读投影；Billing 新增表单只增加 Customer Overlay 的稳定 Party ID 配置并复用原生命令。二者都不是超级大边界改动；详细分级见能力缺口矩阵与边界演变账。

## 5. 公网发布状态

2026-10-12 实测：

- 安全组补入当前来源 `60.221.123.55/32` 后，既有 `ecs-user` 密钥恢复受控 SSH；没有开放宽网段或绕过主机边界。
- 升级前完整备份 ID 为 `20261012T140404Z`，Contract、Money、Party、各应用数据库以及 Evidence 的 19 个文件、20 个版本、20 个对象均进入备份证据。
- 目标 ECS 使用发布包 `omsbizfoundry-d1_v0.1.0-test.oms-v1.1_linux_amd64` 和摘要锁定镜像完成升级；一次发布包执行位缺失导致 `postgres-provision` 退出 126，修正包内脚本权限后重新执行并通过，没有回滚或覆盖业务事实。
- 公网 D1 `check` 通过，17 个长运行容器均为 healthy；Contract 当前 migration 为 `000022_reference_oa_materials.sql`，Money 为 `000023_party_directory_runtime_projections.sql`，Project 为 `000012_reference_oa_production_personnel.sql`，其余选装应用均为当前 schema。
- `https://oatest.antuinfo.cn/oms-v1/` 返回 303 到受保护的 `/workbench/?view=oms-v1`；旧 `/oms/` 继续 303 到只读参考原型，`/workbench/` 返回 200 与安全响应头。
- 通过公网 IAM 登录后，Gateway 分别为 Party、Project、Billing、Money、Contract 换取原生能力凭证；五个 Owner 读取均成功，验收时分别读到 3 个主体、26 个项目、0 条票据、2 条实际资金及 4 项合同上下文，业务写入数为 0。
- 资质单位通过公网 Party Directory 正式命令创建并回读为 active，随后注入 Workbench 与相关选装应用；公网页面 HTML 已确认四个一级入口、正式资质单位 ID 以及 OMS CSS／JS 资源。该轮是登录态 HTTP／Owner 复读，不冒充发布后的人工浏览器逐按钮验收。
- 公网证据保存在 `/opt/omsbizfoundry/acceptance/oms-v1-public-20261012a`；验证脚本不记录密码、Token 或 Cookie。

## 6. 阶段结论

一期“产品长什么样”和“Owner 如何组合”已经进入公网正式 D1：主体、项目、合同金额与开票能够在真实 IAM 下写入和复读，`/oms-v1/` 已可通过 Nginx、Gateway 与 IAM 访问。上线不等于完整业务闭环：资金实际写入、附件、施工方应付纵向旅程、Owner 级全量分页／导出以及发布后人工逐按钮验收仍待完成。功能侧随后按 Party runtime cutover → Money 真实命令 → Evidence 附件的顺序补齐，持续保留幂等、失败续办与独立事实所有权。
