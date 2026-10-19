# 原 OA 手工奖金工作面候选验证

2026-10-04 后续生产布局复验：最新原 `ProductionInfo` 总览／页签中按选中栏目读取，真实组合扩为 17 场景，补键盘、深链和 Back／Forward；307 项 Node、相邻原表单及最新两次独立启动通过。最终事实仍为本轮独立 7／9／7／9，不累加下方历史夹具；新候选尚未发布。最新 bundle／dump、失败、图及生产子栏目缺口见[生产布局报告](./oa-production-layout-ui-validation-20261004.md)，下方 09:10 回执保留其当批状态。

2026-10-04 09:10 UTC 更新。B12 原界面及实际 Workbench→Gateway→真实 IAM→独立 Bonus HTTP→PostgreSQL 18 的有限闭环通过，未发布到 Main，不是完整奖金／OA 通过。08:32 的组件／template 证据保留，但不能替代本轮真实进程组合证据。

## A→B 与边界

旧来源比例 Pool 不能操作手填基数、池外固定额和同人多分工 → 直接复用原 `BonusAllocation`／Form／List／Statistics 与 CSS，完成两侧手工分配、刷新和留史撤回。按 BF-102，Bonus 自有显式 `reference-oa-bonus-v2`、schema 3 四表及新 namespace；旧已确认来源 Pool／Allocation／应付建议 API、表及约束保持。Workbench 无奖金库，不导入业务对象；Person／Assignment 通过版本化端口复验并冻结。登记不是实际付款，也不产生 Money 建议。

原界面布局、抽屉、表格、人员选择与确认窗保留；只作数据／权限适配、精确字符串输入和“发放→登记”更名，不复制浮点、客户端自报金额或物理删除。52 份原文件摘要一致、原目录零写，见[来源回执](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/original-source-provenance.json)。逐操作语义见[规格](../specs/assembly/oa-bonus-original-operation-map.md)。

## 实际验证

| 层 | 运行结果 | 证据范围 |
| --- | --- | --- |
| 前端 | 前端 121 项＋嵌入运行时 179 项，共 300／300、0 skip；TypeScript／Vite 通过 | 候选，不是公网 |
| Bonus／PG18 | 普通和 integration 全包通过；整批零写、精确舍入、并发幂等、历史防改、租户／侧别及旧核心回归 | 私有临时库 |
| 受保护 HTTP | Ed25519 测试 issuer 的负例／脱敏通过；后续真实 IAM 进程签发、原生 Grant 撤权、精确侧别与记录 Scope、当前及历史脱敏、来源停机零写通过 | 两类证据分开；不是 Main IAM／公网 |
| 历史升级 | schema 2 原用例建立已发布 Pool、target_unavailable 应付建议；2→3 后八张旧事实／历史／命令／审计表摘要不变，新手工登记后仍保持，旧命令可离线重放 | 历史夹具，不是 Main 升级 |
| 原组件／工作台浏览器 | 最新 14 场景、正常点击、0 页面错误；实际 Workbench／Gateway／IAM／Bonus／Project／People／Contract 各自独立进程，业务应用各自独立 PG18 库；正常登记／撤回／刷新通过 | 来源事实由自有库 SQL 夹具准备，不是来源创建旅程；没有 Nginx、公网或完整 Suite |
| 空目标恢复 | pg_dump→空目标 pg_restore→独立服务重启；原事实、历史、回执摘要一致；丢回复原命令续办不重复，恢复后继续写并刷新 | Bonus 单库，不是 Suite 联合恢复 |
| 配置／边界 | BF-103 显式同 audience namespace 的 Gateway／Workbench／Access Contract 正负例通过；D1 空／显式 profile 解析通过、零启动；Identity、静态边界与自测通过 | 旧 profile／旧 Pool read 入口保持；未跑 D1 升级或运行期整体隔离 |
| 独立启动 | 最新 Workbench 二进制两次启动／退出、匿名入口、五个 JS／CSS HTTP 摘要通过 | 非 DB 或 D1 整体运行证据；[回执](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/workbench-startup.json) |

最新组合最终 7 条登记、9 条 revision、7 个成功命令、9 条审计；8 次 POST 尝试为组件端口 6 次（含一次成功后丢回复原命令重放）＋实际 Workbench 2 次，不是八个命令事实。旧 Pool／Money 建议均为 0。两侧初始基数 1.01×50%=0.51，固定额 2.02 在池外；同人不同 Assignment 各有一行。实际 Workbench 又正常登记生产 4.04×25%=1.01，并明确原因撤回，原记录和两版历史保留；取消、换侧、刷新均无额外写入。

[最新组合回执](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/workbench-native.json)、[工作台生产抽屉](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/workbench-production-form.png)、[保存列表](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/workbench-production-saved.png)、[撤回保留](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/workbench-production-history.png)、[经营金额撤权](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/workbench-business-redacted.png)均已视觉检查。最小组合未选装成本／生产阶段，截图里的相邻缺入口提示和未连接生产任务仍是缺口，不算那些工作面通过。真实金额撤权后五个金额字段在当前及历史响应中不存在，不以零值替代；新登记拒绝。Project／People 事实摘要保持，Contract 空库零新增；Bonus 只通过受保护版本化端口引用，Workbench 不拥有奖金事实。

本轮只读原目录再次核对 52 份摘要一致。bundle SHA `64911ad1d580dda8fa8813313a40d7d942bd4065a31a772fee55d386ba29de7f`；最新 Workbench native binary SHA `9777a2dd5a0e1ba2352a7e02f17b615b3c0ee58e04b9a6311cec01cd577fd9c0`。最新私有 dump `/tmp/oa-bonus-components.e41EpU/bonus-native.dump` 为 48,257 bytes、SHA `7c4de41c4e79701cf86b2f04f962be589de2c134cc667225b7e212d927f433fc`；这是恢复检查点，早于恢复后继续登记及 Workbench 两次写入，不是最终 7／9 全部事实的备份。恢复检查点摘要相等和后续继续写分别验，私有 dump／密码／密钥／Cookie 不入 Git。

较早 template 层[浏览器回执](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/browser-native.json)、[经营表单](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/form-business.png)、[生产表单](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/form-production.png)、[双侧列表](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/two-sides.png)、[撤回历史](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/withdrawal-history.png)、[恢复继续](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/restored-continued.png)已视觉检查。该层是 9 场景、6 登记／7 revision／5 命令／7 审计，不能与最新组合混算。其 dump 48,168 bytes、SHA `6cd97f8ec8f8b3ddd4bdc0fd8095b9f7b7d3576411e6a1993c1b24c070d73ad5`，只留 `/tmp/oa-bonus-components.nm6llJ/bonus-native.dump`，不入库。

首轮图标使 exact 按钮定位不匹配，次轮退出动画导致两份下拉同时匹配，均零 POST；[失败 1](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/failed-button-locator.json)／[失败 2](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/failed-dropdown-transition.json)保留。调整语义定位、等待隐藏后正常点击，没有 force 或业务写注入。PG 并发夹具修为独立 batch ID，升级摘要改读实际八表；Catalog 旧否定断言与已实现 C7 Suggestion 目录矛盾，改验已声明资源及手工对象不得冒用该能力，没有修改 Main Grant／策略。

真实进程组合保留三次反例：[历史定位选择了旧 r1 行](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/failed-history-locator.json)、[旧入口夹具未带精确 Project scope](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/failed-legacy-scope-fixture.json)、[Workbench 目录读取与夹具 Grant 范围不匹配](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/live/failed-workbench-directory-fixture.json)。后两次 IAM 正确拒绝，未放宽生产策略；仅隔离测试 Tenant 的 Project 目录 read fixture 调整为该测试租户范围，其内只有一个项目，奖金仍精确两侧 Scope。早期停止进程的清理断言／测试出口也已修正，最新驱动在失败和成功出口均停止自有服务，不改 Main。

## 复演及下一门槛

设置私有 `TEST_DATABASE_URL` 后，在 Bonus 目录运行 `GOWORK=off go test -count=1 -tags integration ./...`；缺 URL 的 skip 或 Go cache 不计本轮数据库通过。前端目录运行 `node --test src/*.test.mjs`、`npm run check`、`npm run build`，另运行 Workbench 嵌入 JS 的 179 项测试。Gateway／Workbench／IAM／Access Contract 的实际全包回归及相关 vet 已通过，静态边界可执行检查和 self-test 已通过；它们不是完整运行期隔离结论。

最新浏览器入口 `node acceptance/oa-initiation-repair/verify-bonus-components-browser.mjs /tmp/oa-bonus-components.<mktemp 后缀> --live-sources`，要求本轮 task-owned `oms-bonus-overlay-test-20261004`、PG18 loopback 127.0.0.1:55477。脚本创建随机测试库、启动 native 服务，不连接 Main、不删除已有用户库。不带 `--live-sources` 只复演较早 template 层。结束停止本轮临时容器，保留 dump；不能当作生产 template 配置。

09:06 UTC Main 只读仍有 17 个健康服务，本轮没有发布镜像、migration 或新增 Grant。Billing／奖金 admin＋原 C 精确授权尚未答复，不能挪用此前 Cost 批准。发布必须匹配 Bonus schema 3／profile、能力目录、Gateway＋Workbench 多 namespace reader、Suite schema／renderer，并走备份／兼容／还原门禁；BF-103 的 ANY 准入不授予新能力，不把 record 推导为 read，不扩大 Owner Scope。授权后续原 C，不新建项目绕过失败。

09:13 UTC [收尾](../../acceptance/oa-initiation-repair/evidence/bonus-candidate-20261004/closing.json)：再次 SSH 只读核对 Main 17／17 healthy，零本轮部署／Grant 变更；773 个相关文档本地引用零缺失，`git diff --check` 及 D1 两脚本语法检查通过。所有本轮 native 测试进程已停止，精确 label／ID 核对后停止并自动移除自有临时 PG18 容器／夹具库，恢复检查点 dump 保留；没有删除 Main 或用户数据库。继续公网验收前仍需单独确认精确 Billing／奖金授权。

B11 来源选择、生产专业分工、全部角色／撤权、来源创建旅程、正式同项目公网链、资金／票据关联、OSS 和 Suite 联合恢复仍未完成。当前生产候选只有真实存在的 production_owner／participant，不能冒充原设计／复核／审核等工作角色。[全站清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)继续有效。
