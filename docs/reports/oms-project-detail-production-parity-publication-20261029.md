# OMS 项目详情本地／线上一致性纠偏发布记录

日期：2026-10-29
范围：BF-289
环境：`https://oatest.antuinfo.cn/`
状态：公网发布与真实登录页面验收通过

## 原因与修正

本地候选与线上截图的第一层差异来自浏览器缩放：本地 `127.0.0.1` 标签页为 125%，线上 `oatest.antuinfo.cn` 标签页为 100%。此前仅核对静态资源摘要，未在相同缩放、相同详情标签下做真实页面对照，导致错误地把资源发布成功当成了视觉一致。

在统一为 125% 并切换到“收付款”后，又确认线上生产数据会触发两个候选页没有充分表现的差异：收款／付款在宽屏左右并列，且第一条施工方明细自动展开。BF-289 将收款、付款恢复为有明确阅读顺序的上下两块工作面；施工方保留摘要与未支付金额，但默认收起详细金额和合同入口。完整侧栏、四个详情标签、真实逐笔记录和施工方详情均继续保留。

本次只改变 Unified Workbench 的项目详情装配，不改变 Owner、API、schema、Grant、IAM 或业务事实。

## 验证与发布

- `node --check`、项目详情／成员／Payroll 专项 Node 回归 16/16、Workbench server Go test 和目标差异检查通过。
- Unified Workbench：`v0.1.0-test.oms-v1.59@sha256:f1394ec26dbd3ba015be8c1241bf625ec56bcded7cac41cf3d81b2ddd146807b`，平台 `linux/amd64`。
- 候选环境移除 `D1_WORKBENCH_IMAGE` 后与 v1.58 摘要一致；候选 IAM 契约预检返回 `contracts-valid`。
- 升级前联合备份：`/var/backups/omsbizfoundry/20261029T141518Z`；`checksums.sha256` 全项通过。
- D1 `upgrade` 和独立 `check` 均返回 exit 0；19 个正式长期容器全部 `healthy`。宿主机另有既存的 `omsv152-inspect` 检查容器，不属于 D1 正式长期容器。
- 公网 CSS `20261029.4` SHA-256 `b20310931722714cd156865388a3e3cd6d962aacdf552cfed0d7f5d25d201e45`、JS `20261029.3` SHA-256 `e51ca36f905dfe5ed4ea313a25cbc2a6529caad4aa9796601e6e04a1e5511495`，均与本地完全一致。
- 真实管理员 Chrome 在 125% 下刷新并重新进入“测试项目1／收付款”：收款与付款上下排列，施工方 01 显示为折叠摘要；没有打开编辑、设置应付或逐笔业务动作。
- HTTPS 根入口保持 303 到 `/workbench/?view=oms-v1`，Workbench 返回 200。

本轮真实浏览器验收只读，未提交任何业务命令或写入业务事实。
