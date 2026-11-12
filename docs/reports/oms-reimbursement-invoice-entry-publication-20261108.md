# 新增报销／付款申请发票录入发布验证（2026-11-08）
## 结论

BF-316 已部署到公网 D1。公司内部“新增报销”和“新增付款申请”现在都在申请表内提供“暂不录入、直接录入、上传并识别”三种发票方式；识别必须先完成人工确认，随后恢复原申请草稿、回填票面字段，并在用户明确保存时把同一 Evidence File ID／Version 和 Finance 发票快照提交给 Internal Finance Requests。识别列表仍不开放公司外部 Billing 关联，既有申请补票／换票仍不在本轮范围。

公网 Unified Workbench 为 `v0.1.0-test.oms-v1.86`。本次只改变 Workbench 运行镜像，没有改变 Owner、API、schema、Grant、IAM 或既有业务数据；没有在线创建报销、付款申请或其他业务事实，因此不把发布验证写成真实写旅程通过。

## 发布制品

- Unified Workbench：`v0.1.0-test.oms-v1.86@sha256:785b500314b839dc0b736debada5cd947b9bbe8b6ee27d65cbc5b34e65bf8a3d`
- 镜像平台：`linux/amd64`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.86_linux_amd64.tar.gz`
- 发布包 SHA-256：`f861e06e29b8a19f44a0350a22bc64c0f3eb66db0fe322903c634b6c29b0ff8e`
- 候选环境除 `SUITE_VERSION` 和 `D1_WORKBENCH_IMAGE` 外的内容 SHA-256：`efb7342dbd22fe58c283f105d8821c3ef032e7b93b1dd8b42ec5d6b24c7ff489`

其余 20 个正式服务沿用 v1.85 环境固定的原镜像摘要。

## 升级与恢复证据

- 本地 `test-release-bundle` 通过；目标机外层归档和 93 个包内文件校验通过。
- 目标机 `preflight-upgrade` 通过，候选 IAM 返回 `contracts-valid`。
- 正式升级前恢复点：`/var/backups/omsbizfoundry/20261108T124447Z`；`checksums.sha256` 中 21 项备份制品全部通过。
- 首次 `upgrade` 在 Compose 清理旧的一次性 workload-key 容器时遇到残留网络端点并退出；当时恢复点已经创建，数据卷未删除。确认主组合没有残留运行容器后，以同一候选环境执行 `d1 up` 和独立 `d1 check` 完成恢复续跑。由于发布脚本已进入全组合替换阶段，本次 21 个长运行容器均重新创建，不能描述为只重建 Workbench。
- 续跑后 `d1 check` 返回 0；所有报告的 schema 均为 current，没有 pending migration。
- Compose 正式组合共有 21 个长运行服务，全部 healthy；Workbench 运行镜像、摘要和候选一致。

## 公网复核

- `https://oatest.antuinfo.cn/` 返回 303 并跳转 `/workbench/?view=oms-v1`，Workbench 返回 200。
- 公网 `oms-phase1-finance-owner.js?v=20261108.4` SHA-256 为 `3f5b11d1b1b81b8c23133ab5a01b105749626e007e722d6eae910e4686b4d04c`，与本地候选一致；脚本中可检出“暂不录入、直接录入、上传并识别”三种入口。
- 公网 `oms-phase1-workforce.js?v=20261108.3` SHA-256 为 `8cbfb4bc1eb1db4d09c951ed0b65ad17990b75f2fcd33c50414cdeeece24ee26`，与本地候选一致。

## 已验证与仍待验证

- 本地专项 Node 11/11、Workbench 全量 Node 299/299、Workbench／Internal Finance Requests Go test 与 vet、正式 Owner 浏览器三入口复验和 `git diff --check` 均已通过；详见[本地验证](./oms-reimbursement-invoice-entry-local-validation-20261108.md)。
- 本次发布证明发布包、IAM 预检、恢复点、D1 运行组合和公网静态资源正确，不证明线上真实发票可被当前 OCR 适配器完整识别。
- 仍需使用专门测试成员和测试发票完成登录态“新增报销／付款申请—上传识别—人工确认—回填—保存—列表与详情回读”；该验证必须避免污染既有正式业务事实。
