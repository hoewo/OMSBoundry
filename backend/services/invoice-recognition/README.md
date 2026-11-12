# Invoice Recognition

问题 A → 可验证结果 B：人工逐字段录入发票容易出错 → 对一个已完成安全检查的 Evidence File Version 生成可复核的识别候选，人工确认后才交给发票或报销事实 Owner。

本服务只拥有识别请求、提供方／模型版本、字段置信度、候选 revision 与人工确认记录；不拥有文件字节、发票登记、报销申请、重复判定结果或付款事实。OCR／AI 输出永远不是发票事实，第一版重复候选同时提供 Evidence 内容摘要和规范化“发票代码＋号码”，最终约束仍由消费 Owner 执行。

当前切片实现版本化候选契约、Evidence 精确版本／摘要校验、20 MiB 有界读取、可替换识别 Provider、人工确认和重复键交付。HTTP、IAM、PostgreSQL、正式 Provider、Workbench 页面、D1 和恢复尚未实现，不能描述为已部署功能。
