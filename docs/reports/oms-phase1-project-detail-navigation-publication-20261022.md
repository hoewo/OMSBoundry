# OMS 一期项目详情导航修复公网发布验证（2026-10-22）

## 结论

BF-172 已发布到 `https://oatest.antuinfo.cn/`。项目详情打开后点击任一侧栏入口，Workbench 现在先退出详情，再显示目标页面；不再出现底层页面已切换但详情层仍覆盖画面的假死状态。

本次只修改 Unified Workbench。Project、Contract、Billing、Money、Party、Evidence、IAM、Gateway 的事实、API、schema、Grant 和数据均未改变，公网验收业务写入为 0。

## 问题与修复

问题 A：`activatePage()` 只更新页面和导航状态，而项目详情的关闭逻辑只绑定在“返回项目列表”按钮。详情打开时点击侧栏，目标页面虽已在底层激活，用户仍只能看到旧详情，因此表现为“页面不刷新”。

可验证结果 B：

- 所有页面激活前统一调用 `closeProjectDetail()`；
- 关闭详情时隐藏详情层、清除 `selectedProjectID`、恢复 `body` 滚动；
- 递增 `projectDetailToken`，使切换前尚未完成的异步详情读取不能回写当前页面；
- 点击当前已展开业务组时，若详情可见则优先退出详情；
- “返回项目列表”与侧栏导航复用同一关闭路径，避免两套状态清理规则分叉。

## 制品与发布

- 源码修复提交：`149cfff fix(oms): close project detail on navigation`
- 静态资源版本：`20261022.3`
- Workbench 镜像：`v0.1.0-test.oms-v1.23`
- 镜像摘要：`sha256:3b78fdbeeb273723340b50aa509fcca1aa3620919212f7480ddd590cb95cebc1`
- 线上环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.23.https.env`
- 升级前完整备份：`/var/backups/omsbizfoundry/20261022T045308Z`

发布只重建了 `unified-workbench` 容器；其余长期运行服务未替换。

## 验证证据

本地通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`
- `node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`：4/4
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`
- `python3 -m unittest acceptance/oms-phase1-public/test_verify_public_oms_v1.py`：2/2
- `git diff --check`
- 本地 `127.0.0.1:18183` 返回的脚本为 `assets/oms-phase1.js?v=20261022.3`

服务端静态回归还明确断言：详情关闭函数存在、项目选择会清除、详情令牌会递增、页面激活前会关闭详情、当前组点击优先退出详情、返回按钮复用统一关闭函数。

公网通过：

- 候选镜像预检与 IAM `contracts-valid`
- D1 全栈 `check`
- 17/17 长运行容器 healthy
- 正式登录、五次 Gateway 授权及五个 Owner 只读复验
- 读取：Party 4、Project 1、Billing 0、Money 0、Contract workspace 6
- `business_writes=0`
- 公网页面 SHA-256：`4ccbba26941e6ddab2d86a27eadd4f59a3c23d83b41b6263a95af3b82849b262`

本地预览当前没有真实项目，未把“本地人工打开详情再逐项点击侧栏”虚报为已完成；交互修复由代码回归断言覆盖，公网真实认证链、资产版本和 Owner 读取已闭合。用户可直接刷新公网页面进行最终体验确认。
