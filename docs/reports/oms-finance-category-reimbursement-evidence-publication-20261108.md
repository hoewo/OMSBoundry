# 财务类别、报销附件与折算入口发布验证（2026-11-08）

## 结论

BF-315 已部署到公网 D1。费用类别维护现在使用紧凑的启停清单，平台类别只允许启停，自定义类别继续单独新增；报销上传附件后，Workbench 会把 Evidence 上传完成回执显式转换为 Internal Finance Requests 严格接受的 `EvidenceRef`，不再把 `display_name`、`content_sha256`、`mime_type` 等传输层字段直接提交给 Finance。考勤和请假登记继续保留，含义尚未解释清楚的折算政策入口及无有效政策时的摘要生成动作暂时隐藏。

公网 Unified Workbench 为 `v0.1.0-test.oms-v1.85`。本次升级完成了升级前备份、候选预检、正式升级、独立 D1 检查、运行镜像摘要和公网静态资源逐字节校验；没有在线创建报销申请或其他业务事实，因此不把发布验证写成真实报销写旅程通过。

## 发布制品

- Unified Workbench：`v0.1.0-test.oms-v1.85@sha256:a287edf454d4f0a4580faedce73e69fc591e85e1beec45eadfbf3cafed76dbc4`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.85_linux_amd64.tar.gz`
- 发布包 SHA-256：`7ac14a2c283a65b93d8354d4d5360385a2ef8f9bcf376bee5955c3d15ea92680`

其余正式服务沿用 v1.84 环境固定的原镜像摘要；本次没有改变 Owner、API、schema、Grant、IAM 或业务数据。

## 升级与恢复证据

- 发布包 `test-release-bundle` 通过。
- 目标机 `preflight-upgrade` 通过，候选 IAM 返回 `contracts-valid`。
- 正式升级前恢复点：`/var/backups/omsbizfoundry/20261108T120825Z`。
- `checksums.sha256` 中 21 项备份制品全部通过。
- 正式 `upgrade` 返回 0；Internal Finance Requests schema 保持 `000004_draft_retention.sql`，Workforce Attendance schema 保持 `000001_workforce_attendance.sql`，Invoice Recognition schema 保持 `000002_discard_proposals.sql`。
- 升级后独立执行 `d1 check` 返回 0；所有报告的 schema 均为 current，没有 pending migration。
- Compose 正式组合共有 21 个长运行服务，全部 healthy；Workbench 运行镜像及摘要与候选一致。

## 公网复核

- `https://oatest.antuinfo.cn/` 返回 303 并跳转 `/workbench/?view=oms-v1`，Workbench 返回 200。
- 公网 `oms-phase1.css?v=20261108.2` SHA-256 为 `68f628b0172072979a6ae3c35a120110b6e6fced08a8b42b331584b7a5b3bb93`，与本地候选一致。
- 公网 `oms-phase1-finance-owner.js?v=20261108.2` SHA-256 为 `8ea6ee763dc59a252290dd18b661a831efa8447ae05ee937747592da44b24cee`，与本地候选一致。
- 公网 `oms-phase1-workforce.js?v=20261108.2` SHA-256 为 `0ad9fe47b65e08a4b61d649ecd2f1038edfc818ff183bd8e9c85458634de30b9`，与本地候选一致。

## 已验证与仍待验证

- 本地专项 Node 11/11、Workbench Go test／vet、`git diff --check` 和引用实际 CSS 的 headless Chrome 静态截图复验均通过；详见[本地验证](./oms-finance-category-reimbursement-evidence-local-validation-20261108.md)。
- 旧故障已由线上只读日志定位为 Evidence upload session 创建／完成成功后，Finance 因严格 JSON 契约拒绝未归一化附件字段并返回 400。
- 本次发布没有代填线上账号密码，也没有执行“上传发票—保存草稿—弹窗关闭—列表与详情回读”的真实登录写旅程。
- 折算政策的产品解释、权威口径和重新开放条件仍待后续单独确认。
