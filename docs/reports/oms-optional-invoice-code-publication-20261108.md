# 全电发票代码选填发布验证（2026-11-08）

## 结论

BF-317 已部署到公网 D1。新增报销／付款申请的直接录入和上传识别都只要求完整发票号码；旧版纸质发票票面存在“发票代码”时仍可选填，该字段已移到票面信息末尾并明确标注“仅旧版发票有此字段”。现代全电发票没有发票代码时可以确认识别、保存申请，并由 Internal Finance Requests 按号码执行强重复判断。

本轮不改变 Evidence、Invoice Recognition、Internal Finance Requests 或 Billing 的事实所有权。识别候选仍须人工确认，Finance 仍在自身事务内校验票面金额、Evidence 引用和重复线索。公网组合为 `v0.1.0-test.oms-v1.87`；本次没有在线创建报销、付款申请或其他业务事实，因此不把发布验证描述为真实写旅程通过。

## 实现范围

- Workbench 的手工录入顺序改为发票号码、开票日期、销售方、价税合计、原发票文件，最后才是“发票代码（选填）”。
- 图片识别人工校对把发票号码设为必填，发票代码改为选填并放在票面信息末尾。
- Internal Finance Requests 的发票快照只要求号码；代码存在时使用“代码＋号码”作为强重复身份，没有代码时使用号码。
- `000005_optional_invoice_code.sql` 重建发票号码索引，使代码为空的全电发票也进入强重复索引范围。
- Invoice Recognition 交付包的重复 key 与 Finance 规则一致：有代码时为 `invoice:<code>:<number>`，无代码时为 `invoice:<number>`。

## 本地验证

- Workbench 专项 Node：11/11；Workbench 全量 Node：299/299。
- Workbench、Internal Finance Requests、Invoice Recognition Contract 的 `go test ./...` 与 `go vet ./...` 通过。
- PostgreSQL 18 隔离集成测试验证无代码全电发票可以保存，第二张同号码发票命中强重复；原有代码＋号码规则继续通过。
- `test-release-bundle` 通过；发布包内 93 个文件 checksum 全部通过。

## 发布制品

- Internal Finance Requests：`v0.1.0-test.oms-v1.87@sha256:907d03b100b69ea2cdf470abaf90dfd69e8b88c80349fa05df1c4a2603778c25`
- Unified Workbench：`v0.1.0-test.oms-v1.87@sha256:88f888486d2d8e2bbd0d602836216ae8263046b36f73c291666159b84b61dde7`
- 镜像平台：`linux/amd64`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.87_linux_amd64.tar.gz`
- 发布包 SHA-256：`60adaffe2da6a1197303ac181ae09098020fda50494ad2ba9437358313e038cb`
- 候选环境除 Suite 版本及两个变更镜像外的内容 SHA-256：`5fb26d2d4cc85589452ec3c3339ce0b3b97effc451d7af1b664dc39f6fa9426a`

## 升级与恢复证据

- 目标机归档 SHA-256 与本地一致，候选 `preflight-upgrade` 通过，IAM 返回 `contracts-valid`。
- 正式升级前恢复点为 `/var/backups/omsbizfoundry/20261108T130844Z`；`checksums.sha256` 中 21 项备份制品全部通过。
- `upgrade` 在删除旧 Compose 网络时再次遇到一次性 workload-key 容器端点延迟释放并退出。恢复点已完成且业务卷未删除；确认旧正式组合已停止、残留端点释放后，以同一候选执行 `d1 up`，再执行独立 `d1 check` 完成恢复续跑。该过程重新创建正式组合，不能描述为仅重启两个变更服务。
- Finance schema 为 `000005_optional_invoice_code.sql`；所有报告 schema 均为 current。
- 正式组合 21 个长运行服务全部 running 且 healthy；Finance 与 Workbench 实际运行摘要与候选镜像一致。

## 公网复核

- `https://oatest.antuinfo.cn/` 返回 303 并跳转 `/workbench/?view=oms-v1`，Workbench 返回 200。
- 公网 `oms-phase1-finance-owner.js?v=20261108.5` SHA-256 为 `f4bef31fa246e49f3edbf00024d27536eafc78fc2cf82f5e21f4439f7482bcad`，与本地候选一致；可检出“发票代码（选填）”和“仅旧版发票有此字段”。
- 公网 `oms-phase1-workforce.js?v=20261108.4` SHA-256 为 `768448c3df4333393f236f92a5e76f38005a4b1fe4bd0ebbeb403539111097e4`，与本地候选一致；识别校对中的发票代码不再带必填约束。

## 仍待验证

需要使用专门测试成员和全电发票样本完成登录态“上传识别—人工确认—保存—列表与详情回读”，并验证第二次录入同号码时的用户提示。该写旅程必须使用可清理测试数据，不能污染既有业务事实。
