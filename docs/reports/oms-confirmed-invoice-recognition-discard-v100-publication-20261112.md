# OMS 已确认发票识别记录审计删除 v1.100 发布记录

日期：2026-11-12  
事项：BF-332  
目标：`https://oatest.antuinfo.cn`  
结论：已发布；本地、真实 PostgreSQL、D1、公网静态资源和真实浏览器只读验收通过。

## 发布内容

- Invoice Recognition 的 `needs_review` 与 `confirmed` Proposal 都可用既有 `invoice-recognition.discard` Capability、独立 Command、expected revision 和必填原因追加为 `discarded`。
- 删除只隐藏识别域当前投影；不可变 Confirmation、Evidence 精确引用、Provider 输出、全部 revision、Actor、Credential、时间、原因和审计继续保留。
- Workbench 已为每条“已确认”记录增加“删除”，删除原因不预填；弹窗强提示识别记录删除不会撤销已建立的报销、付款申请或 Billing 正式票据。
- “已删除”详情显示删除原因、操作人和时间。下游事实仍须在各自 Owner 中追加删除、作废或冲销，不做跨库级联。

## 交付物

- 源码提交：`a38a2c882145604f1ee7fd73f395f30d46ddb6a8`
- Invoice Recognition：`v0.1.0-test.oms-v1.100@sha256:7d8906ccd8ccde72f6c033f11d52cee77e2df2b4870f3bcd6d2949261d8bea87`
- Unified Workbench：`v0.1.0-test.oms-v1.100@sha256:c7c72f13fd18a3cc5ac957c9fafe9759823daa8e03ffbfac5dc13546c73c2e06`
- Linux AMD64 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.100_linux_amd64.tar.gz`
- 发布包 SHA-256：`5ac90a6e9c8982386f1ae204157656867dc8cf9548153b084dac2c5c242ffacb`
- 发布前联合备份：`/var/backups/omsbizfoundry/20261112T145036Z`

## 实际验证

本地与发布包：

- Invoice Recognition `go test ./...`、`go vet ./...` 通过。
- 一次性 PostgreSQL 18 集成测试通过：confirmed 删除后仍有 1 条 Confirmation、3 段 Proposal revision、3 条审计事件。
- Workbench Node 306/306、`go test ./...`、`go vet ./...` 通过。
- P9 preflight、D1 P9 composition、正式发布包内外层 checksum 通过；发布元数据绑定上述干净提交，目标为 `linux/amd64`。

目标机与公网：

- 候选镜像拉取、Suite Manifest 渲染和 IAM `contracts-valid` 预检通过。
- 联合备份 21 项 checksum 全部通过；D1 `check` 通过。
- 21/21 长运行服务 healthy；运行中的 Invoice Recognition 与 Workbench 镜像摘要和上述交付物一致。
- `/` 返回 303 到 `/workbench/?view=oms-v1`，`/health/ready` 返回 ready。
- 公网 `oms-phase1-workforce.js?v=20261112.1` 与本地产物逐字节一致，SHA-256 为 `61f2fede1c6b3a87246bbaba327aa720df49b012178ac93e655fe11756a2eddb`。
- 真实 Chrome 强制刷新后，6 条已确认记录均展示“查看／导出／删除”。打开首条删除弹窗可见空白必填原因和跨 Owner 不级联警告，随后点击取消；没有删除线上数据。

## 发布过程异常与恢复

标准 `d1 upgrade` 已先成功创建并校验联合备份。停栈时，历史一次性 Cost／Outsourcing workload-key 容器仍占用 `omsbizfoundry_default` 网络，网络删除返回非零；业务卷与备份未删除。随后使用同一 v1.100 候选执行 `d1 up`，全栈重建成功并通过 D1 check、21/21 healthy、公网资源和浏览器验证，无需恢复数据。

## 明确边界

- 本次没有代用户删除任何已确认记录，也没有改变既有 Grant、Capability 名、API 路径、schema 或数据所有权。
- `discarded` 不是物理删除；删除后审计、原文件和确认历史仍可回看。
- 识别记录删除不会自动撤销下游事实；报销、付款申请和正式票据必须在各自 Owner 中按其生命周期处理。
