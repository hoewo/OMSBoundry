# Invoice Recognition

问题 A → 可验证结果 B：人工逐字段录入发票容易出错 → 对一个已完成安全检查的 Evidence File Version 生成可复核的识别候选，人工确认后才交给发票或报销事实 Owner。

本服务只拥有识别请求、提供方／模型版本、字段置信度、候选 revision 与人工确认记录；不拥有文件字节、发票登记、报销申请、重复判定结果或付款事实。OCR／AI 输出永远不是发票事实，第一版重复候选同时提供 Evidence 内容摘要和规范化“发票代码＋号码”，最终约束仍由消费 Owner 执行。

当前已实现并部署版本化候选契约、Evidence 精确版本／摘要校验、20 MiB 有界读取、XML 与本地 JPEG／PNG／PDF 首页 Provider、人工确认、审计删除、重复键交付、HTTP／IAM、PostgreSQL、Workbench、D1 和恢复。PDF 校对使用受 `invoice-recognition.tenant.read` 与 Proposal Scope 保护的只读首页审阅图；服务即时栅格化并以 `no-store` 流式返回，不拥有或持久化第二份文件。OFD、PDF 多页选择和多版式准确率统计门禁仍未完成。
