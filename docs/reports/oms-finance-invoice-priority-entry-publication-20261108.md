# OMS 财务申请发票优先入口发布验证（BF-320，2026-11-08）

## 结论

新增报销与新增付款申请的共同发票区已发布到公网 Workbench `v0.1.0-test.oms-v1.91`。两个表单都在基本信息之前呈现“上传发票并识别”主按钮，暂不录入、直接录入和上传识别仍保持互斥；主按钮复用 BF-316 的草稿保存、原图人工校对、确认回填和精确 Evidence 引用链。

本次只发布 Unified Workbench 呈现与前端编排，没有修改 Owner、API、schema、Capability 或业务数据，也没有执行线上报销、付款申请、识别候选或其他业务写入。

## 构建与发布物

- Workbench 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.91`
- 镜像摘要：`sha256:d72d05de985bbbee15ab3fb9eebbc0cc4ef8028c858c22c044a72466babf07cf`
- D1 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.91_linux_amd64.tar.gz`
- 发布包 SHA-256：`0e8db26335d0c4a99860640f88bdb6adce4e90341a4469d7024d7cccd90c070f`
- 发布包内部 93 项 checksum 全部通过。
- 升级前预检拉取固定摘要镜像并通过 IAM capability／policy 合同解析。

## 发布过程与恢复证据

- 升级前恢复点：`20261108T151241Z`，目录 `/var/backups/omsbizfoundry/20261108T151241Z`。
- 恢复点 21 项 checksum 全部通过。
- `upgrade` 在备份完成后遇到 Compose 旧网络端点清理竞态；未删除命名数据卷，也未执行恢复。随后使用同一候选 env 和同一发布包执行幂等 `d1 up`，完整栈成功启动。
- 独立 `d1 check` 通过，IAM、Contract、Money、People、Party、Project、Evidence、Business Engagement、Billing、Production、Approval／Audit、Cost、Bonus、Internal Finance、Payroll、Workforce Attendance 与 Invoice Recognition 均报告当前 schema。
- 21 个长运行容器全部为 `running` 且 `healthy`。
- 运行中 Workbench 的配置镜像与实际 Image ID 均为上述 v1.91 摘要。

## 公网复验

- `https://oatest.antuinfo.cn/` 返回 `303` 到 `/workbench/?view=oms-v1`。
- `oms-phase1-finance-owner.js?v=20261108.6` 返回 `200`，公网与本地 SHA-256 同为 `69db1219a9fbf7a4faa35f64ba01cc0648f5bbc767022843f40d03115be5d566`。
- `oms-phase1.css?v=20261108.3` 返回 `200`，公网与本地 SHA-256 同为 `72cd494794c052ce4a803066e0759ffafbbd0704ed2978e63a0b8c8c34c023e5`。
- 公网脚本包含“发票优先录入”“上传发票，自动填写基本信息”和统一的 `[data-finance-recognize]` 绑定；公网样式包含顶部主按钮与窄屏单列规则。

## 实现与测试证据

- 发票区在 DOM 顺序中先于“基本信息”，报销和付款申请共用 `invoiceEntryMarkup` 与同一个识别桥。
- 顶部主按钮点击会先选择上传识别方式，再触发文件选择；取消或确认后恢复当前申请草稿。
- 窄屏静态视觉复核确认主按钮全宽、三种方式单列且没有横向溢出。
- `node --check`、专项 Node 11/11、Workbench 全量 Node 299/299、Workbench `GOWORK=off go test ./internal/server/...`、`go vet ./internal/server/...` 和 `git diff --check` 通过。

真实登录态下的报销／付款申请保存仍应使用专门测试数据另行验收；本报告不把静态资源与装配发布验证外推为业务写入已经完成。
