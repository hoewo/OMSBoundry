# 图片发票识别与人工校对公网发布验证（2026-11-06）

## 结论

BF-309 已发布。JPEG／PNG 和 PDF 首页面现在可在 Invoice Recognition 容器内离线识别，Workbench 以“左侧原图与字段标注、右侧字段与置信度”的工作面供人工核对。识别结果仍只生成 `needs_review` 候选，不自动形成发票、报销或付款事实；XML 原能力保留，OFD 仍需外部 Provider。

最终公网 Suite 为 `v0.1.0-test.oms-v1.79`，21 个长运行服务全部 healthy。线上实际样本完成 Evidence 上传、识别、原图预览、8 项位置标注和候选列表复读；人工确认按钮没有点击。

## 原因与修复

- 旧实现只有 XML 本地 Parser；图片／PDF 依赖未配置的外部 Provider，因此页面没有真实图片识别场景。
- 首次本地 OCR 进入公网后，Evidence 返回的本地文件访问授权使用公网 origin，容器内无法解析该公网域名。连接器现在只对本地 Evidence 签名路径改写为内部 `EVIDENCE_BASE_URL`，完整保留 path、query 和签名；外部对象 URL 不改写。
- Invoice Recognition 增加 Tesseract `chi_sim+eng` 和 Poppler。PDF 只处理首页面；识别输出包括字段值、置信度和归一化票面区域。
- Workbench 上传后立即展示原文件与三阶段进度，完成后自动进入人工校对；图片标注与右侧字段可互相定位。开票日期等不可靠字段保持空白并显示警告。
- v1.79 进一步把既有候选中可能保存的内部字段 key 转为“税额”等业务中文，新候选从 Provider 源头直接生成中文警告，不修改历史数据。

## 验证

- Invoice Recognition 契约、Provider、Evidence 连接器、领域和 HTTP 普通测试及 `go vet` 通过。
- 实际样本在 Linux OCR 运行容器中识别出发票号码、购销双方及税号、三项金额和 8 个字段区域；金额等式成立，无法可靠识别的日期未被猜测。
- Workbench Node 回归 `294/294`、服务端 Go test／vet、JS 语法和 `git diff --check` 通过。
- 两个镜像均为 `linux/amd64`，D1 release bundle 回归通过。
- 正式升级前备份为 `/var/backups/omsbizfoundry/20261106T124214Z`；升级后独立 D1 `check` 通过，运行中的 Tesseract 为 5.5.2，语言包含 `chi_sim`／`eng`，Poppler 为 25.12.0。
- Chrome 使用既有管理员 Session 刷新后复验：候选详情显示原图、8 个可点击中文标注、字段值、置信度和“税额识别置信度较低”等中文警告；取消退出，未确认候选。

## 发布制品

- Invoice Recognition：`v0.1.0-test.oms-v1.79@sha256:8b62f6c6952578d5fdbe7463973f0af300a7f29b06ccc80ba4f0ea02dc61ecc3`
- Unified Workbench：`v0.1.0-test.oms-v1.79@sha256:5e65e327abb04f64bd2c120155dce5f2823212bfaf93d301609c09884fa7c3d7`
- 发布包：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.79_linux_amd64.tar.gz`
- 构建元数据：`deployments/d1/.runtime/build-metadata/v79/`

## 明确未完成

- OFD 仍需可撤换 Provider；不能宣称全格式识别。
- PDF 当前只处理首页面，多页选择与逐页标注未实现。
- 当前验证证明一份实际样本链路可用，不等于多版式字段准确率已经建立统计门禁。
- Billing／Internal Finance Requests 尚未消费确认摘要，识别不会自动入账、报销或付款。
