# Cost／Outsourcing 资料引用 v1（BF-073 的 C6 来源切片）

A→B：成本凭证、外协合同／说明已有 C6 自有 File 引用，但 Project 资料组合缺少来源端口 → 每个 Cost Entry／Engagement 发布当前 revision 的有界只读资料投影，由后续连接器装配，不复制金额、受托方、合同、建议、评分或文件对象。

## 最小范围与不能破坏的约束

新增 `/api/v1/references/cost-entries/{id}/materials` 与 `/api/v1/references/outsourcing-engagements/{id}/materials`，分别使用既有 `cost-entry.read`／`outsourcing-engagement.read`；目标 Service 复验 Actor／Tenant 与精确 `cost_entry`／`outsourcing_engagement`、Project 或 Tenant Scope。amount.read、evidence.manage、另一个聚合的 read 均不是这两个端口的隐含附加权限；禁止额外授予以通过测试。

沿用 MaterialReferenceProjection v1 基本形状：schema 1、source `cost-outsourcing`、ID、Tenant、Project ID、resource_type `cost_entry|outsourcing_engagement`、status `recorded|withdrawn`、revision、digest、items。items 只为 slot relation_key、File ID、冻结 Version／相同 Evidence source revision、display_name；排序 relation_key／File ID／Version，摘要为 items canonical JSON SHA-256。不带成本金额／类别／里程、Party 档案、Contract、Money 建议、Assessment 或对象地址／凭证。

只读取聚合自己的单行完整 snapshot，无上游调用、业务写入、下载审计、新服务或 migration。纠正发布新当前引用，旧 revision 不改写；撤回保留引用并明确 withdrawn，不自动反向写 Project。空必须为 `items:[]`；身份、状态、正 revision、正 File Version、1～100 字符 slot 与 1～500 字符 display_name 失败关闭，UUID 必须 canonical、文本不得有首尾空白。最多 100 项，超限拒绝且不截断；相同 slot 的不同 File／Version 合法，仅拒绝相同 tuple。这些有界投影检查不改变现有写域规则。

## 验收与非目标

源码／HTTP 验证两聚合、排序／摘要、空、撤回、畸形事实、金额独立、能力和精确 Scope 正反例；真 PG18 验证纠正／撤回／历史以及全部 owned tables 在读前后的 canonical；真实 IAM 签发与目标 verifier 单列，SKIP 不算通过。上游 bootstrap 使用测试提供者时明确标记，不拼成正式链。

Project canonical C6 Link、消费者／Workload Policy／migration、兼容发布、联合恢复、连续 C8 和浏览器后续单独验收；本批不扩展报销、付款、评分、完成 Profile 或前端，也不将两个聚合拆成新服务。
