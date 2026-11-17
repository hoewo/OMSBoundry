# OMS PDF 发票识别与标注恢复本地验证

日期：2026-11-17（GMT+8）
事项：BF-341
状态：本地实现与回归通过；公网发布待执行

## 问题与根因

- 截图对应最新 Proposal 实际保存了 9 组 `field_regions`；标注不是在 OCR 或数据库中丢失，而是 Workbench 的 PDF 分支直接渲染浏览器原生 `iframe`，绕过了图片标注层。
- 同一 OCR 行同时包含左侧购买方和右侧销售方时，旧逻辑先把整行放入候选，再取第一／最后名称行，导致买方混入双方文本；Tesseract TSV 又以空格拼接词块，`nameAfterLabel` 没有清理中文字符间空格。

## 修复

- Provider 过滤跨越大半页的整行，优先从左／右虚拟行分别选取购买方、销售方及税号；名称清除标签噪声和中文 OCR 分隔空格，保留英文单词间空格。
- Invoice Recognition 新增受现有读取 Capability 与 Proposal Scope 保护的首页审阅图读取。服务重新通过 Evidence 端口验证原版本大小和 SHA-256，即时调用 Poppler，返回 `image/png`／`no-store`，不保存派生文件。
- Workbench 的 PDF 分支读取审阅图 Blob 并复用现有缩放、拖动、字段标注和双向定位；弹窗关闭即撤销 Blob URL。图片与 XML 路径保持原行为。

## 已运行验证

- Invoice Recognition：`go test ./...`、`go vet ./...`。
- 新增回归：同一行左右双方、竖排角色噪声、中文字符间空格、双方税号及独立半页区域。
- 新增 HTTP 回归：带凭证返回 `image/png`／`no-store`；无凭证返回 401。
- Unified Workbench：`go test ./...`、`go vet ./...`、两个 JavaScript 文件语法检查、`node --test internal/server/*.test.mjs` 310/310。
- 组合门禁：`make p9-full-composition-preflight-test`、`make d1-p9-composition-test`。
- 生产只读复核：最新两条截图对应 Proposal 均保存 9 组区域；买方 Candidate 含左右双方，卖方 Candidate 为右侧名称。复核没有创建、确认、删除或修改任何 Proposal／Finance 事实。

## 边界

- 既有 Proposal 保持不可变；发布后只会恢复其 PDF 标注显示，旧 Candidate 仍由用户人工校对。重新上传产生的新 Proposal 才使用改进后的购销双方解析。
- 本项仍只处理 PDF 首页，不增加多页选择，不实现 OFD，也不宣称建立了多版式准确率统计门禁。
