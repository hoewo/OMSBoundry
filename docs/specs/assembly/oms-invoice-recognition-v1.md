# OMS 发票 OCR／AI 识别 v1

## 问题与结果

人工逐字段录入发票容易出错 → 对一个已完成安全检查的 Evidence File Version 生成可复核候选，只有人工确认后的结构化结果才允许交给发票或报销事实 Owner。

## 事实与边界

- Evidence Service 继续唯一拥有文件字节、File ID、Version、安全检查状态和内容摘要。
- Invoice Recognition 独立拥有识别请求、提供方／模型版本、字段置信度、候选 revision 与人工确认记录。
- Billing 或 Internal Finance Requests 继续拥有正式发票登记、报销申请、最终重复约束和业务状态。
- OCR／AI 输出永远是 `needs_review` 候选，不是发票事实，也不得自动提交报销、创建票据或触发付款。
- Provider 通过端口替换；SDK、模型 Secret、固定 IP 和厂商数据结构不得进入公共核心。

## v1 输入与输出

输入必须引用一个精确 Evidence File ID／Version，并携带规范 SHA-256、MIME 和字节数。识别服务重新做 20 MiB 有界读取并核对大小与摘要，然后才调用 Provider。

候选字段包括发票代码、号码、开票日期、购销双方名称与税号、不含税金额、税额、价税合计、币种及逐字段置信度。图片 Provider 还返回以页面万分比表示的字段区域、识别文本和区域置信度，供 Workbench 在原图上定位；区域不是业务事实，也不能替代字段值的人工确认。金额使用整数最小货币单位，币种首版只接受 CNY；当三项金额同时存在时必须满足“不含税金额＋税额＝价税合计”。

人工确认要求完整发票号码；发票代码只对票面确实存在该字段的旧版发票选填。系统交付两个彼此独立的重复线索：

1. Evidence 内容 SHA-256；
2. 规范化发票身份：有代码时为代码＋号码，无代码的全电发票按号码识别。

识别服务只提供线索；最终消费 Owner 必须在自己的事务和业务范围内执行重复判断。

## 删除、幂等与恢复

- 开始识别和确认各自使用独立 Command ID。
- Repository 以请求意图摘要防止同键异义重放。
- Provider 超时或结果未知时，不得用新 Command 静默创建第二个候选；调用方应先按原 Command 查询或重放。
- 确认必须校验 expected revision，已确认或 revision 不符时失败关闭。
- 只有 `needs_review` 候选可通过独立 Command、expected revision 和必填原因追加为 `discarded`；同义重放返回同一结果，换原因或目标的同 Command 拒绝。
- `discarded` 保留 Proposal、Evidence 精确引用、Provider 输出、全部 revision、Actor、时间、原因和审计，只从默认工作列表隐藏；它不是 Evidence 物理删除。
- `confirmed` 不允许在识别域删除。若已登记为正式发票，作废／冲销继续由 Billing 的追加式命令负责。

## 交付、导出与内部财务申请消费

- 批量 CSV 面向人工汇总，只导出已确认记录；单条 JSON 使用 `invoice-recognition.delivery/v1`，包含 Proposal ID／revision、确认候选、精确 Evidence File ID／Version、Provider／模型、警告与两个重复线索。
- 导出是可追溯快照，不是正式发票，也不改变候选状态。消费方仍必须校验 schema、来源 Tenant、revision、重复线索和自己的业务不变量。
- BF-316 当前产品入口在“新增报销”和“新增付款申请”中都提供暂不录入、直接录入或上传并识别。识别入口从当前申请草稿发起，人工确认后恢复原草稿并回填金额、日期、销售方、代码和号码；原 Evidence File ID／Version 以 `invoice` 用途交给 Internal Finance Requests。确认候选不会自行创建申请，用户仍须明确保存。
- BF-320 把这两类申请的“上传发票并识别”提升为表单最上方、基本信息之前的主入口；暂不录入和直接录入继续作为同一区域的次级路径。入口点击只选择既有识别方式并调用共同桥，不新增识别、保存或 Owner 语义。
- BF-318 明确识别原图从创建 Evidence 时就必须使用规范用途 `invoice`，使文件版本进入 `invoice-v1/passed`；不得先用自定义“识别来源”用途创建 `general/not_required` 再把它冒充发票 Evidence。既有错误版本保持不可变，只能由用户重新识别生成新版本。
- Internal Finance Requests 保存自己的发票快照，要求申请金额等于价税合计，并在自身事务内按 Evidence 摘要、发票身份（有代码时为代码＋号码，无代码时为号码）及销售方＋日期＋金额执行重复判断。它不保存识别 Provider 输出或文件字节；Invoice Recognition 不保存报销或付款申请。
- 只有尚未附带发票的新报销／付款申请开放该入口。报销正式提交继续要求 Evidence；既有申请补票／换票、识别列表直接外关联、自动创建项目进／销项票据、会计入账和自动付款均不在当前范围。
- BF-312 已发布的 Billing 外部登记编排代码暂时保留，但入口从页面隐藏且不可达；不得把保留代码描述为当前开放能力。未来重新开放必须重新确认公司内外业务边界、类型化来源和 Owner 最终约束。

## 图片识别与校对工作面

- JPEG／PNG 使用容器内 Tesseract `chi_sim+eng`；PDF 通过 Poppler 只栅格化首页面后走同一 OCR。该实现不依赖外部网络，不保存第二份原文件。
- 整页 OCR 未得到合法开票日期时，只允许对标准票面右上方头部做一次三倍局部复读；结果必须来自带“日期”标签的行、包含完整年月日并通过 Gregorian 日历校验。成功后区域换算回原图万分比，失败继续留空，不得从票号、金额或残缺日期猜值。
- 原图预览在左侧占主要面积，提供适应窗口、放大、缩小、滚轮缩放与拖动查看；识别区域以可点击标注覆盖。桌面校对窗口在可用视口内接近全屏，宽屏状态由遮罩层持有并定位内部内容框，不能因选择器作用对象不一致回退为基础 720px。右侧按票面信息、购销双方、金额分组展示字段、置信度和警告，字段名与区域 key 必须显式映射，点击标注、悬停或聚焦字段必须双向定位。
- OCR 不可靠的日期或字段保持空白并提示人工补充，不得为提高“完整率”猜值。低置信度使用业务中文字段名，不能暴露内部 key。
- XML 没有图片坐标时仍使用结构化字段校对；OFD 仍需可替换 HTTP Provider。PDF 多页选择不在 v1，当前明确只处理首页面。
- 关闭或取消校对不确认候选；只有显式“确认识别结果”才追加确认 revision。

## 当前完成与明确未完成

当前公网已完成版本化契约、独立 PostgreSQL Repository／migration、HTTP／IAM Capability、Evidence 精确版本／摘要连接器、20 MiB 有界读取、电子发票 XML 本地解析、JPEG／PNG 与 PDF 首页面本地 OCR、可替换 HTTP AI Provider、字段区域、候选规范化、金额不变量、可缩放／拖动且桌面近全屏的原图标注人工确认、候选审计删除、CSV／JSON 交付、D1 装配和备份恢复。BF-317 已将现代全电发票收敛为“号码必填、旧版代码选填”，BF-318 已修复识别原图的 Evidence 用途，BF-319 已为标准票面头部增加严格校验的日期局部复读，BF-320 已把报销与付款申请的共同识别入口提升到基本信息之前。当前公网 Invoice Recognition 为 `v0.1.0-test.oms-v1.90`，Finance 为 v1.87，Workbench 为 v1.91。

BF-312 已发布审计删除、已删除筛选、CSV／JSON 交付和显式 Billing 进／销项登记代码；BF-313 已把该外部关联入口从当前产品表面隐藏并实现新增付款申请消费，BF-316 已把同一申请内旅程扩展到新增报销，BF-317 已发布全电发票代码选填兼容。两类申请都在人工确认后回填并引用精确 Evidence，Finance 继续实施票面校验与重复指纹；真实登录写旅程仍待使用专门测试数据验收。

当前部署没有外部 AI Provider URL／凭证，但 XML、JPEG／PNG 和 PDF 首页面可真实解析；OFD 明确失败关闭。后续还需要补多版式脱敏样本准确率门禁、PDF 多页选择和 OFD Provider。识别域不会自动入账、创建报销或付款；BF-316 的 Finance 消费入口也只创建用户明确保存的新报销或付款申请。

[BF-320 发布验证](../../reports/oms-finance-invoice-priority-entry-publication-20261108.md)、[BF-320 本地验证](../../reports/oms-finance-invoice-priority-entry-local-validation-20261108.md)、[BF-319 发布验证](../../reports/oms-invoice-date-local-reread-publication-20261108.md)、[BF-318 发布验证](../../reports/oms-recognized-invoice-evidence-validation-publication-20261108.md)、[BF-317 发布验证](../../reports/oms-optional-invoice-code-publication-20261108.md)、[BF-316 发布验证](../../reports/oms-reimbursement-invoice-entry-publication-20261108.md)、[BF-313 发布验证](../../reports/oms-internal-payment-invoice-entry-publication-20261108.md)、[BF-313 本地验证](../../reports/oms-internal-payment-invoice-entry-local-validation-20261106.md)、[BF-312 发布验证](../../reports/oms-invoice-recognition-discard-delivery-publication-20261106.md)、[校对窗口尺寸发布验证](../../reports/oms-invoice-review-canvas-sizing-publication-20261106.md)、[校对查看器发布验证](../../reports/oms-invoice-review-zoom-linkage-publication-20261106.md)、[图片识别发布验证](../../reports/oms-invoice-recognition-visual-review-publication-20261106.md)、[领域与考勤发布验证](../../reports/oms-invoice-recognition-attendance-proration-publication-20261106.md)
