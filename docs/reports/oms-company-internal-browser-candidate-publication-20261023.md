# OMS 公司内部管理浏览器体验版发布报告

日期：2026-10-23  
事项：BF-181  
源码基线：`codex/company-internal-management@13a704d`

## 结论

公司内部管理的成员、工资和财务管理候选已发布到 `https://oatest.antuinfo.cn/`。它是受正式 IAM 登录保护的浏览器体验版，不是新的服务端业务系统：记录按 Tenant＋登录 Subject 写入当前浏览器 `localStorage`，不会写入既有 Owner，也不会跨浏览器或设备同步。页面显式展示该限制。

功能范围沿用 BF-180：成员档案与固定工资／缴费基数、五险一金方案快照、月度工资、绩效录入、人工确认发放、生成后手动同步尚未发放期间的新成员，以及报销／付款申请的草稿、待支付和人工确认。金额继续使用整数最小单位，比例使用整数万分比。

## 发布制品与回滚点

- 本地分支提交：`13a704d feat(oms): publish internal management browser candidate`
- Unified Workbench：`v0.1.0-test.oms-v1.31@sha256:f28cd22b18c49ccfc876d6a32756e0ce75badee9d74cb77b26db32060d9e4785`
- `linux/amd64` 应用 manifest：`sha256:4f31ef6bbfef996d8a34e4decafae64093f118f8a8f645ef84b914c2740a6888`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.31.https.env`
- 人工升级前完整备份：`/var/backups/omsbizfoundry/20261023T123759Z`
- D1 升级自动备份：`/var/backups/omsbizfoundry/20261023T123910Z`

两个备份均包含十二个业务 Owner、IAM、Evidence／Contract 对象、签名密钥和 Suite 选择；`checksums.sha256` 实际校验通过。

## 验证

本地实际通过：

- `node --check`；
- 251/251 项 Workbench Node 回归；
- `go test ./...`；
- `go vet ./...`；
- `git diff --check`。

公网实际通过：

- v1.31 候选环境与 v1.30 只相差 `D1_WORKBENCH_IMAGE`；
- D1 `preflight-upgrade`、升级和发布后独立 `check`；
- 17/17 个长运行容器 healthy；
- 正式 IAM CSRF、账号登录、会话、Workbench 页面和版本化内部管理脚本均返回 HTTP 200；
- 公网页面包含 Subject 标记和“仅保存在当前登录账号的本浏览器”提示；
- 公网脚本按 Tenant＋Subject 构造存储 Key，且不包含旧的“尚未接入正式业务服务”失败关闭分支。

## 边界

本次只替换 Unified Workbench，没有增加数据库、Owner、API、Capability、Grant、审批、Money 建议、银行代发或跨设备同步，没有写入、修改或清除任何业务 Owner 数据。浏览器体验记录不应作为正式工资发放或财务记账依据；真实后端必须在 People、未来 Payroll、申请来源应用与 Money Flow 的既有所有权边界内另行设计。
