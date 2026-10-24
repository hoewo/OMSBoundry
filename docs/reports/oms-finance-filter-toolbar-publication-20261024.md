# OMS 财务筛选与批量选择交互发布报告

日期：2026-10-24  
事项：BF-182／BF-183  
源码基线：`codex/company-internal-management@249e957`

## 结论

工资／财务列表的“全选、取消全选”和财务工具栏“全部类型”选择器排版已经发布到 `https://oatest.antuinfo.cn/`。类型选择器使用现有统一自定义控件并紧邻搜索框，不再被面向普通说明文字的 `margin-left:auto` 推到页面中间；窄视口规则也不会误隐藏该输入控件。

本批只替换 Unified Workbench。公司内部管理数据仍按 Tenant＋登录 Subject 保存于当前浏览器，不写入现有业务 Owner；Party、Project、Billing、Money、Contract、IAM、Gateway、数据库、schema、Capability 和 Grant 均未改变。

## 发布制品与回滚点

- 本地分支提交：`249e957 fix(oms): align finance type filter`
- Unified Workbench：`v0.1.0-test.oms-v1.32@sha256:5f3e3a25396c7b7b0ec803646bc1ff85301ba3b8e2f38c42d6767549f70ca291`
- 平台：`linux/amd64`
- 静态样式版本：`20261024.1`
- D1 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.32.https.env`
- 升级前完整备份：`/var/backups/omsbizfoundry/20261024T034034Z`

备份包含 IAM、十二个业务 Owner、Evidence／Contract 对象、签名密钥和 Suite 选择；`checksums.sha256` 实际校验通过。

## 验证

本地实际通过：

- Workbench `GOWORK=off go test -count=1 ./...` 与 `go vet ./...`；
- 主脚本和内部管理脚本 `node --check`；
- 内部管理模型／公网页面 8/8 项定向 Node 回归；
- `git diff --check`；
- 本地浏览器读取计算布局，确认“全部类型”与搜索框相邻，选择器不再获得自动左边距。

公网实际通过：

- v1.32 候选环境与 v1.31 只相差 `D1_WORKBENCH_IMAGE`；
- IAM 契约预检、发布前与发布后 D1 `check`；
- 17/17 个长运行容器 healthy；除 Workbench 外其余 16 个容器 ID 不变；
- 正式 IAM 登录、五个 Gateway Audience 授权与 Owner 只读复验；
- Owner 读取为 Party 5、Project 1、Billing 2、Money 2、Contract workspace 6；目录投影为甲方 3、施工方 1；
- `oms-phase1.css?v=20261024.1` 返回 HTTP 200、`Cache-Control: public, max-age=31536000, immutable`，且包含对 `.oms-v1__select` 的排除规则；
- 公网页面 SHA-256 为 `9aa5ace6a23f748a66aa01c2be0cc3f296dbea13b97c7a94a10594d6e686649e`；
- 本轮业务写入为 0。

## 边界

该发布确认的是 Workbench 浏览器交互和公网装配，没有把公司内部管理体验数据提升为正式后端事实，也没有实现 Payroll、审批、个税、银行代发、Money 建议或跨设备同步。全选仍只作用于当前筛选结果中的可操作记录；财务类型筛选的业务含义未改变。
