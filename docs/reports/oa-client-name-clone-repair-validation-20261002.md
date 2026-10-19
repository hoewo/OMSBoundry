# 原 OA 历史甲方名称补全：候选修复验证

日期：2026-10-02  
决策：BF-089  
结论：此只读辅助切片取得分层证据；B01、正式入口和完整 OA 仍未验收。Main 未升级。

后续部署补记：以上是候选批次结束时的状态。随后同日已在 Auth58 Main 部署首条 OA 修复链，正常公网点击和空目标数据恢复见[独立部署报告](./auth58-oa-first-slice-deployment-validation-20261002.md)。本报告原合成／私有验证记录不升级为当时的公网证据，B01 全项与完整 OA 仍未验收。

## 修复什么

问题 A → 可验证结果 B：原 OA 输入甲方名称时可以用历史文字补全，而候选只有普通输入框 → 同一经营页显示无下拉的幽灵文字，普通 Tab 明确接受并正常移到联系人，只有点击保存才提交。

只读对照原仓库提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302` 的 `ProjectPartyNameInput.tsx`／CSS、`BusinessInfoTab.tsx`、`oa2026.SuggestNames`。原控件优先前缀、其次包含匹配；排除与当前文字相同的候选，不排除其他包含该文字的名称。不是全站像素一致或原站真人验证。

[规格](../../arckit/spec/project-workspace/counterparty-name-suggestions.md)中的 GET 只返回有界名称：Project 自有联系历史及创建时已经验证的最小主体名称快照。Tenant 和资源范围来自原生可信凭证，复用既有 `counterparty-context.read`；无新授权、应用、表或迁移。不照搬原项目库联查主体主档／无范围搜索，不返回联系人、电话、项目 ID 或 Party ID，文字仍不是正式身份。

前端查询失败不阻断手工输入；失焦、换项目和迟到结果有请求代次保护。Tab 只改输入文字，不自动保存；Shift+Tab 保持原值。三字段清空行为和原行内表单保留。

## 实际验证

| 层次 | 结果 | 不代表 |
| --- | --- | --- |
| 本地无容器 | Project／Workbench Go 测试与 vet、JS 语法、Node 32 项零 skip，通过 | 正式身份、真联合链 |
| 私有 PG18 `r` | Project 112／HTTP 85 条含子测试 PASS，零 fail／skip；范围、历史名称、仅名称、空资源范围、数量边界、字面 `%`／`_`、原生 HTTP→服务→PG、读零写及旧回归 | 公网或正式 IAM |
| 独立启动 `s` | 新二进制自有空库 schema 10，八个匿名原生路由 401，重启事实不变，非 root／只读／不发布端口 | 新 Suite 联合恢复 |
| 合成浏览器 | 原实际页面输入、Tab／Shift+Tab／模糊提示、清空／取消、显式保存一次、回读／刷新同项目经营页、无效链接可见错误和重读列表恢复；控制台零 error | 真实用户／Gateway／PG 一次联合操作 |

读零写断言覆盖历史摘要、revision、command 与 assignment 数量，不夸大为本接口做了全表 canonical 扫描。原 HTTP 未授资料读能力返回 403，即使有普通 Project 读取权也不能代替；Header 和 URL 不能改写 Scope。生产端查询只读，不产生业务事实。

浏览器第一次发现表单缺少样式类（输入框 24px）、无效项目初始链接错误落到隐藏立项面板。修正后重新启动探针：输入框 40px、圆角 6px，1280×720 下保存按钮可见且中心命中；错误出现在可见项目列表，明确没有创建／修改项目，可重读恢复。保存后同一资料和经营 tab 刷新保持。

初次探针 359.47 秒、零写；修正后的探针 259.20 秒、甲方保存 1 次／复读 3 次／名称查询 7 次／目录 2 次，其余写命令零。截图仅通过工具内联检查，未归档截图文件；不能声称有可复用截图包。

随后追加独立故障探针 `OA_CLIENT_NAME_READ_FAIL=1`，88.12 秒 PASS：浏览器输入触发一次名称读取 503，额外只读 HTTP 诊断一次确认 503；提示隐藏、手工名称不变、Tab 正常移到联系人、保存前零写。显式保存一次成功，复读／刷新共三次仍为同项目经营页原手工名称／联系人／电话，控制台零 error；结束探针并关闭临时页。见[故障状态](../../acceptance/oa-initiation-repair/browser-client-name-failure-synthetic-state-20261002.json)。这只补合成故障分支，不代表正式 IAM 故障联验；提示接口延迟／迟到响应、撤权及真人全矩阵仍待验。

两个开发夹具问题保留说明：服务单测漏 mandatory ReferenceValidator 导致构造器拒绝，真库夹具误用不存在的 NodeType 字段导致交叉编译失败；修正夹具而未放宽生产门槛。一条本地 Node 命令误用不存在的 `.cjs` 路径，实际 `.test.mjs` 两组重跑后才记 32 项通过。不是这些失败已经作为远端正式验收通过。

故障夹具追加后 Workbench 全 Go／vet、JS 语法重跑通过。最后组合 Node 命令中目录测试路径写错，只实际运行 25 项；随后单独按真实 `reference_oa_project_directory.test.mjs` 重跑 7 项通过，故最终 32 是 25＋7 的实际结果，不将前一条组合命令记成 32 项。

## 隔离及证据

远端 `r`／`s` 的 Main 13 库摘要和 17 容器 ID 前后相同；诊断位于 `/opt/omsbizfoundry/acceptance/oa-initiation-repair-20261002r` 与 `...20261002s`。两个测试容器、独立 server 和私有 PG 均停止、exit 0、无 OOM。runner＋PG 上限 0.45 CPU／448 MiB，启动 server＋PG 0.4 CPU／352 MiB，串行且无公网端口；本地没有启动 Docker，restore20 未重开。临时浏览器已关闭，未修改用户主站页。

机器摘要见[真库／启动证据](../../acceptance/oa-initiation-repair/evidence/client-name-results-20261002.json)，[浏览器分层状态](../../acceptance/oa-initiation-repair/browser-client-name-synthetic-state-20261002.json)，运行入口见[验收说明](../../acceptance/oa-initiation-repair/README.md)。正常探针历史夹具摘要保留，追加故障夹具摘要单列，未覆盖前一轮记录。此前联系资料写入及空目标恢复结果保留在[BF-088 报告](./oa-client-info-clone-repair-validation-20261002.md)，本次只读辅助没有新增迁移或恢复权威事实。

## 尚未完成

原经营参与人／生产人员的类型化身份和权限映射、其余经营与生产操作、资质费率、项目进度筛选、资料统计、原站真人全量差异、正式 IAM 版本选择与发布兼容闭包、联合恢复及公网用户操作均继续待办。不能把历史名称补全与能保存的甲方表单升级成 B01 或完整 OA 通过；总报告仍为 `insufficient-evidence`。
