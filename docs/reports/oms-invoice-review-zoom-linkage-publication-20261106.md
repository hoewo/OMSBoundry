# 发票人工校对查看器公网发布验证（2026-11-06）

## 结论

BF-310 已发布。Workbench 的图片发票校对工作面不再把原图压成不可操作缩略图：左侧原图区域扩大，并支持适应窗口、按钮／滚轮／键盘缩放和拖动平移；右侧字段悬停、聚焦或点击会定位对应原图标注，点击标注会返回对应字段。三项金额字段使用显式 OCR 区域 key 映射，恢复了金额区域的联动反馈。

公网 Unified Workbench 为 `v0.1.0-test.oms-v1.80`，Invoice Recognition 保持 `v0.1.0-test.oms-v1.79`。本次只改变 Workbench 呈现与静态资源版本，没有修改 Owner、API、schema、Grant、识别候选或正式业务数据。

## 实现与边界

- 校对弹层桌面最大宽高调整为 1580 × 960，原图列优先获得空间；窄屏仍回落为上下布局。
- 图片查看器以原始像素尺寸渲染并由统一变换控制缩放和平移；标注与图片共用变换层，因此不会在放大后漂移。
- 工具栏提供缩小、适应窗口、当前倍率与放大；滚轮围绕指针缩放，拖动查看，键盘支持 `+`、`-`、`0`。
- 字段卡片与标注共用识别字段 key。`amount_excluding_tax`、`tax_amount`、`total_amount` 分别显式映射到 `amount_excluding_tax_minor`、`tax_amount_minor`、`total_amount_minor`。
- 关闭或取消仍不确认候选；只有既有“确认识别结果”命令才追加确认 revision。

## 本地验证

- `node --check frontend/oms-workbench/internal/server/oms_phase1_workforce.js` 通过。
- Workbench 专项 Node 测试 3/3、全量 Node 测试 294/294 通过。
- `go test ./frontend/oms-workbench/...` 与 `go vet ./frontend/oms-workbench/...` 通过。
- D1 v1.80 release bundle 检查通过；发布镜像平台为 `linux/amd64`。

## 公网验证

- 正式升级前恢复点：`/var/backups/omsbizfoundry/20261106T130859Z`。
- `checksums.sha256` 中 21 项备份制品全部通过。
- 发布后独立 `d1 check` 通过；21 个正式长运行服务全部 healthy。
- 运行中的 Workbench 镜像与期望摘要一致。
- 公网 `/workbench/assets/oms-phase1-workforce.js?v=20261106.4` 包含查看器控制、金额区域显式映射和双向定位逻辑；`/workbench/assets/oms-phase1.css?v=20261106.3` 包含大尺寸工作面与查看器工具栏样式。
- 公网未执行候选确认，没有产生发票、报销或付款事实。

## 发布制品

- Unified Workbench：`v0.1.0-test.oms-v1.80@sha256:09a3afcd65488ad42c6c2af697e8623e4bd2fd240f79512d195f34dd71d3274f`
- Invoice Recognition（未变更）：`v0.1.0-test.oms-v1.79@sha256:8b62f6c6952578d5fdbe7463973f0af300a7f29b06ccc80ba4f0ea02dc61ecc3`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.80_linux_amd64.tar.gz`

## 明确未完成

- OFD 仍需可撤换 Provider；PDF 仍只处理首页面。
- 多版式脱敏样本的字段准确率门禁尚未建立。
- Billing／Internal Finance Requests 尚未消费确认摘要，识别不会自动入账、报销或付款。
