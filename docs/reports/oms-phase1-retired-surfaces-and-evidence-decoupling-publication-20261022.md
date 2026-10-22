# OMS 一期旧界面退役与 Evidence 前端解耦发布报告

日期：2026-10-22  
事项：BF-175  
源码基线：`main@6ccad2d`

## 结论

旧仿 OA 工作台与旧 OMS 参考 Demo 已停止公开运行。`/legacy[/]`、旧 `/oms[/]`、`view=legacy`、`view=oms-reference` 和 `preview=oa` 统一返回当前 OMS；旧 `reference-oa-*`、`reference-oms-demo.*` 与 `workbench.js` 公网资源返回 `410 Gone`。当前 OMS 未登录页只装载独立 `oms-phase1-login` 资源，业务首屏不执行旧 OA runtime；附件上传、下载与可预览文件的处理只在首次附件动作时按需加载独立 `oms-phase1-evidence` bundle。

本次停止的是历史前端产品面，不是 Evidence Service。IAM、Gateway、Evidence、Contract、Money、Party、Project、Billing 及其余已启用事实 Owner 继续运行；未删除业务数据、账号、Grant、附件或 Owner 事实。

## 发布制品与回滚点

- Unified Workbench：`v0.1.0-test.oms-v1.26@sha256:fbf22ce273e2f63708124463a95cfff562c19b661c0316b157532f9ad617abd7`
- Gateway：`v0.1.0-test.oms-v1.26@sha256:01165eb12a7f7fdf4602dc9585f6c897ecc2f68ff6468501c13ec129b186b7d8`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.26.https.env`
- 升级前完整备份：`/var/backups/omsbizfoundry/20261022T071636Z`
- 公网入口：`https://oatest.antuinfo.cn/oms-v1/`

## 本地门禁

以下检查实际执行并通过：

- `npm run check`
- `npm run test`：206/206
- `npm run build`：历史审计 bundle 与当前独立 Evidence bundle 均可构建；当前页面只引用后者
- `node --check`：`oms_phase1.js`、`oms_phase1_login.js`
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/... ./backend/platform/gateway/...`
- `GOMAXPROCS=2 go vet ./frontend/oms-workbench/... ./backend/platform/gateway/...`
- `python3 -m unittest discover -s acceptance/oms-phase1-public -p 'test_*.py'`：10/10
- Python 语法检查与 `git diff --check`

## 公网验收

候选预检与 IAM contract parser 验证通过；发布后 D1 全栈 `check` 通过，17/17 个长运行容器为 healthy。公网验收完成以下事实复核：

- 匿名登录页只引用 `oms-phase1-login.css/js?v=20261022.6`，未引用旧 OA 登录、立项或 React runtime；
- 正式 IAM 登录成功，当前 OMS shell 与 `20261022.6` 版本化资源返回 immutable 缓存；
- `/legacy/`、旧 `/oms/`、`view=legacy` 与 `view=oms-reference` 均 `303` 到当前 OMS；
- 四类旧界面资源返回 `410 Gone`；
- `oms-phase1-evidence.js` 按独立资源提供，大小 709,021 bytes，不在业务首屏 HTML 中同步执行；
- 五个 Gateway Audience 均完成正式授权和 Owner 只读：Project 1、Party 4、Billing 0、Money 0、Contract workspace 6；
- Contract 与 Money 的 Party Directory authority 均保持有效；
- 页面 SHA-256 为 `532c35f8396785d9f8c2e1fd7069bb566265f4bac90eac84691543408ae6434b`；
- 验收业务写入为 0。

## 边界

历史模板、旧前端源码和构建入口暂留仓库，仅用于审计、回溯和必要的内部测试，不属于公网运行图。恢复这些旧页面需要新的产品决定与发布变更，不能绕过 Gateway 门禁。独立 Evidence bundle 只封装既有 Evidence API、Local upload 或 OSS STS multipart 交互，不拥有文件事实、不新增数据库、Capability、Grant 或跨 Owner 写入。
