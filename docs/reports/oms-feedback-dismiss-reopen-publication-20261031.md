# OMS Feedback 弹层关闭与恢复修复发布记录

- 日期：2026-10-31
- 决策：BF-300
- 结果：通过
- 公网入口：`https://oatest.antuinfo.cn/`

## 问题与结论

公网 Feedback 弹层存在两个相邻缺陷：点击反馈卡片以外的 iframe 空白区域会意外发送关闭消息；弹层隐藏后，反馈页仍是当前导航页，再次点击侧边栏“产品反馈”只重复导航而不会恢复弹层，用户只能刷新网页。

本次将关闭和恢复改为显式状态流：

1. SDK 在嵌入模式捕获反馈卡片外的 `pointerdown` 和 `click`，将空白区域定义为惰性区域；
2. 只有卡片右上角关闭按钮可以发送一次 `feedback-sdk:close`；
3. Workbench 收到显式关闭后隐藏 iframe，并记录上次打开的是“提交反馈”还是“我的反馈”；
4. 再次点击侧边栏、顶部栏或移动端“产品反馈”入口，按上次模式重新打开同一已确认会话；
5. 退出登录和页面销毁继续彻底清理 iframe、配置与未读状态。

## 发布物

### Feedback SDK V2

- 公网资源：
  - `assets/index-20261030161710-C-gS5r-L.js`
  - `assets/index-20261030161710-BldpMKPQ.css`
- 仅更新 `sdk-v2/`，未修改旧 `/sdk` 或存储桶根对象。

### Unified Workbench

- 版本：`v0.1.0-test.oms-v1.69`
- 镜像摘要：`sha256:c131fb07448b48fe41ab5693924024ab0f9fa33b339b3fd6331ef92cad32fe5a`
- Feedback 静态资源版本：`20261030.3`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T162048Z`

## 验证证据

- Feedback SDK 测试 38/38 通过，V2 TypeScript 与生产构建通过；
- Workbench `go test ./frontend/oms-workbench/internal/server` 通过；
- P8 真实 Chromium 双 Origin 验收通过：3 个场景、32 项断言，新增“关闭→侧边栏再次打开”链路；
- 公网 SDK 使用显然无效的夹具 Key 做零写入验证：空白点击关闭计数为 0，显式关闭按钮计数为 1；
- Workbench 正式 upgrade 前完成联合备份，升级后的 D1 全栈检查通过；
- 19 个长运行容器健康，无 unhealthy 或 starting 容器；
- 运行中 Workbench 使用上述不可变镜像，公网 Feedback 宿主 JavaScript 与本地 SHA-256 一致。

## 未改变事项

本次不改变 Feedback API Key、项目 115、稳定用户 ID、IAM、Gateway、数据所有权、通知／已读规则、Owner API、schema、Grant 或业务数据；公网验收没有创建、提交、修改或删除任何反馈。
