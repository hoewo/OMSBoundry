# OMS 开票日期局部复读发布验证（2026-11-08）

## 结果

BF-319 已发布到 `oatest`。Invoice Recognition 在整页 OCR 未形成合法开票日期时，对标准票面右上方头部做一次三倍局部复读；结果仍须来自带日期标签的同一行并通过真实日历校验。公网运行镜像：

`invoice-recognition:v0.1.0-test.oms-v1.90@sha256:645c34a07b5f559a8b825cf856f92a994d521a10bebdf2586810a8e89ab910f4`

## 原因与修复

线上同一原始 JPEG 的整页 Tesseract TSV 把票面 `2024年06月05日` 识别成 `2024 年 0 月 05 目`。旧实现因月份不完整而正确留空，但没有利用已知的标准票面头部区域再次读取。

修复仅在整页候选缺日期时执行：裁取页面右上方受限头部、三倍放大、使用同一离线 `chi_sim+eng` Provider 复读，再做日期标签匹配、完整年月日提取和 Gregorian 校验。成功值和置信度映射回原图区域；临时裁图随单次识别临时目录清理，不保存第二份 Evidence。残缺或非法日期仍留空，不做推断。

## 验证证据

- Invoice Recognition 全量 `go test ./...` 与 `go vet ./...` 通过。
- 新增中文日期、噪声标签、缺月份和非法日期单元测试通过。
- 使用线上相同 SHA-256 `a672529719fb685b88018cf594dfea6a87ea2fe70d0c501c4372b5c021b4f80f` 的原始 JPEG 在 Linux AMD64、Tesseract `chi_sim+eng` 容器中回归：候选 `issued_on=2024-06-05`，日期区域置信度 `8880`；原“开票日期未可靠识别”警告消失，税额低置信度警告保留。
- D1 发布包 `v0.1.0-test.oms-v1.90` 校验通过，归档 SHA-256 为 `d675a8bcd5c573a9e8c55dc79881f5eca3bb7149934757f2ef61844f9c1830c3`。
- 升级前恢复点 `/var/backups/omsbizfoundry/20261108T144301Z` 的 21 项 checksum 全部通过。
- Compose 清理阶段遇到已知网络端点竞态；使用同一 v1.90 候选保留卷恢复启动后，独立 `d1 check` 通过。
- 正式 21/21 长运行容器 healthy；Invoice Recognition 运行 Image ID 与发布 digest 一致；公网根入口返回 303 到 Workbench，Workbench 返回 200。

## 未扩大声明

本次没有修改 Evidence、Finance、Billing、IAM、API、schema 或 Grant，没有创建新的线上识别候选、报销或付款事实。该局部策略只覆盖标准票面头部；其他版式仍须人工补充并纳入后续脱敏样本准确率门禁。
