# OMS 一期统一文件预览验证报告

日期：2026-10-13  
决策：BF-153  
入口：<https://oatest.antuinfo.cn/oms-v1/>

## 结论

OMS 一期已经把甲方／施工方合同附件、开票／收票附件、回款／支付凭证收敛为同一套文件操作：可安全原生预览的 PDF 和常见图片显示“预览＋下载”，其他格式保留下载。预览覆盖原详情之上，关闭后回到原位置，不新建文件页面或额外业务入口。

该能力复用现有 Evidence Service 和 File Transfer Client。Workbench 只使用业务记录中的稳定 File ID／Version，读取描述后按既有格式、大小与访问策略申请短期授权；预览关闭时释放 Object URL／缓存。没有新增文件数据库、公开永久 URL、跨库读取、转换服务、OCR、在线编辑或 IAM Grant。

## 实现范围

- 附件行显示紧凑类型标识、文件名、预览和下载，长文件名截断但保留完整标题；
- PDF 使用浏览器内嵌查看器；PNG、JPG、JPEG、WebP、GIF、BMP 使用图片查看器并支持 50%～300% 缩放；
- 不满足当前安全预览策略的格式或大小显示降级原因并提供下载；
- 预览支持关闭按钮、遮罩和 Esc；Esc 事件在预览层截断，避免同时关闭底层业务详情；
- 桌面使用居中大预览层，窄屏使用全屏预览，不增加侧栏或业务入口。

## 自动验证

实际执行并通过：

```text
node --check frontend/oms-workbench/internal/server/oms_phase1.js
node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs
go test ./frontend/oms-workbench/...
git diff --check -- frontend/oms-workbench/internal/server/oms_phase1.js frontend/oms-workbench/internal/server/oms_phase1.css frontend/oms-workbench/internal/server/oms_phase1.html frontend/oms-workbench/internal/server/server.go frontend/oms-workbench/internal/server/server_test.go
deployments/d1/ops/test-release-bundle dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.7_linux_amd64.tar.gz
```

发布后 D1 `check` 通过，17 个 `omsbizfoundry` 长运行容器 healthy。只替换 Unified Workbench：

```text
v0.1.0-test.oms-v1.7
sha256:69fc8def18092af7bf0b45ba2be394d02d51881b109fdbe7b5ac0e4b1c9a707f
```

## 公网浏览器验证

在正式 HTTPS、既有登录会话和项目 `2a112e09-c0bf-419d-b615-d3420faeeec5` 中实际完成：

1. 从项目详情进入甲方合同，附件 `甲方合同.png` 显示独立“预览”和“下载”；
2. 打开预览后，页面显示文件名、`PNG`、70 B、缩放、下载和关闭，文件由短期授权解析为浏览器预览源；
3. 从“应收回款 → 开票管理”进入真实开票记录，附件 `甲方发票.png` 使用同一预览层打开；
4. Esc 关闭预览后，合同详情仍保持打开，返回路径未丢失。

合同、Billing 票据和 Money 资金详情都调用同一个 `evidenceMarkup`／`wireEvidenceManager` 动作链，因此收票、回款和支付凭证不另设分支实现。

## 明确边界

- 本轮没有为 Word、Excel 等文件建立在线转换；它们继续下载后查看；
- 没有把静态文件名或扩展名当作最终安全判断，真正打开前仍由描述与既有预览策略复核；
- 没有修改 Evidence、Contract、Billing、Money 的 schema、API 所有权或业务事实；
- 当前验收附件是 70 B 的测试图片，用于验证授权、渲染、控件和返回路径，不代表大尺寸真实业务文件的视觉内容验收。
