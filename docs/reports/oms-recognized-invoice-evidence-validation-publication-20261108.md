# 识别发票 Evidence 校验档案修复发布验证（2026-11-08）

## 结论

BF-318 已部署到公网 D1。新增报销／付款申请的“上传并识别”现在以 Evidence 用途 `invoice` 创建原始发票版本，因此 Evidence 会形成 `invoice-v1` 校验档案和 `passed` 安全状态；人工确认后保存申请不再因识别文件被误标为通用资料而返回 `invalid invoice Evidence reference`。

本轮只修正 Unified Workbench 对 Evidence 上传契约的调用，不修改 Evidence、Invoice Recognition 或 Internal Finance Requests 的 Owner、API、schema、Grant 和业务事实。既有错误版本仍是不可变的 `general/not_required` Evidence，不能静默改写；用户必须在原申请中点击“重新识别”，生成新的发票用途 Evidence 后再保存。

## 根因证据

- Workbench 的直接上传路径一直使用 `purpose: invoice`，但识别上传路径使用了未被 Evidence 发票校验器识别的 `purpose: invoice_recognition_source`。
- Evidence Service 只把规范用途 `invoice` 映射为 `invoice-v1/passed`；其他用途落入 `general/not_required`。
- Internal Finance Requests 保存申请时要求 Evidence 为 `available`、`invoice-v1`、`passed`，并复验摘要、媒体类型和字节数，因此正确拒绝了错误分类的版本。
- 公网只读检查确认报错申请引用的 File `8767faa2-4749-4632-96b6-dec2a56d37a5` Version 1 为 `general/not_required`；没有修改该文件、识别候选或申请。

## 实现与回归

- 识别上传改用 `purpose: invoice`，保留原始文件、人工校对、确认候选和 Finance 显式保存的既有顺序。
- 专项测试固定要求识别路径使用 `invoice`，并禁止旧的 `invoice_recognition_source` 回归。
- Workbench 专项 Node 11/11、全量 Node 299/299、`go test ./...` 与 `go vet ./...` 通过。
- Evidence Service `go test ./...` 与 `go vet ./...` 通过；Evidence 后端代码未改动。
- D1 发布包检查通过，包内 93 项 checksum 全部通过。

## 发布制品

- Unified Workbench：`v0.1.0-test.oms-v1.88@sha256:cb9d684e4261bc8c8801ebcb89423f3a1a43b6511cbf24bda0feb4f0ddcd87a2`
- 镜像平台：`linux/amd64`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.88_linux_amd64.tar.gz`
- 发布包 SHA-256：`ef5876f81580a0d8f8dc24a97d3eb5c866ff16ecd886cb519021fe41b8730610`

## 升级与恢复证据

- 候选 `preflight-upgrade` 通过，IAM 返回 `contracts-valid`。
- 正式升级前恢复点为 `/var/backups/omsbizfoundry/20261108T134055Z`；`checksums.sha256` 中 21 项全部通过。
- `upgrade` 在删除旧 Compose 网络时再次遇到一次性 workload-key 容器端点延迟释放并退出。备份已完成、命名数据卷未删除；使用同一 v1.88 候选执行 `d1 up` 恢复续跑，再执行独立 `d1 check`，全部 schema 为 current。
- 正式组合 21 个长运行容器全部 running 且 healthy；Workbench 实际运行镜像摘要与候选一致。

## 公网复核

- `https://oatest.antuinfo.cn/` 返回 303，`/workbench/?view=oms-v1` 返回 200。
- 公网 `oms-phase1-workforce.js?v=20261108.5` SHA-256 为 `1d0b55656306c7a63dd718de0437626194add0599098a069e85427384f6b48a8`，与本地候选一致。
- 公网脚本可检出 `purpose:'invoice'`，不能检出 `purpose:'invoice_recognition_source'`。
- 本次没有在线创建、修改或提交报销／付款申请，也没有改写既有 Evidence；修复后的真实写旅程由用户用“重新识别”显式触发。
