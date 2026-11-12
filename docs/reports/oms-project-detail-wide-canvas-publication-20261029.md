# OMS 项目详情宽画布发布记录

日期：2026-10-29
范围：BF-288
环境：`https://oatest.antuinfo.cn/`
状态：公网发布与 D1 技术验收通过

## 结果

在 BF-286／BF-287 的紧凑项目详情基础上，桌面详情画布最大宽度从 1180px 调整为 1440px，仍保留 32px 单侧安全留白；往来单位工作面最大宽度从 780px 调整为 980px。中等屏幕继续使用 920px 受控宽度，720px 及以下继续使用单列全宽规则。

本次只改变详情页宽屏空间利用率，没有放大字号或卡片间距，也没有改变四标签、侧边栏、项目列表、票款逐笔事实、Owner、API、schema、Grant 或业务数据。

## 验证与发布

- 本地 `node --check`、项目详情／成员／Payroll 专项 Node 回归 16/16、Workbench server Go test 和目标差异检查通过。
- Unified Workbench：`v0.1.0-test.oms-v1.58@sha256:7a6eb43929102d6c61af58904a41e4489b64c312696d9ef60ce349543a2e4a8a`，平台 `linux/amd64`。
- 候选环境移除 `D1_WORKBENCH_IMAGE` 后与 v1.57 摘要一致，确认只改变 Workbench；IAM 契约预检返回 `contracts-valid`。
- 升级前联合备份：`/var/backups/omsbizfoundry/20261029T135027Z`；`checksums.sha256` 全项通过。
- D1 `upgrade` 和随后独立 `check` 均返回 exit 0；19/19 长期容器全部 `healthy`。
- 公网 CSS 版本 `20261029.3`，SHA-256 为 `5696aabb7b8a4a66639d38757155f58bfc91ca934e465d8000a341f4502e0008`，与本地完全一致并包含 1440px 宽画布规则。
- HTTPS 根入口保持 303 到 `/workbench/?view=oms-v1`，未登录 Workbench 返回 200 登录页。

本轮没有使用用户凭据登录，没有执行业务命令或写入业务事实。
