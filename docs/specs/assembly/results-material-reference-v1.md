# Results 报告资料引用 v1（BF-073 的 C5 来源切片）

A→B：批复／审计报告的 File 引用已有独立事实，但 Project 资料组合没有 C5 来源端口 → Results 发布单个稳定 Result 当前 revision 的只读引用，后续消费者以版本化端口装配，不复制金额、结果历史或文件对象。

## 本批范围与约束

仅新增 `GET /api/v1/references/approval-audit-results/{result_id}/materials`。保留既有按 Project 的 Completion Reference，不改变其摘要或含义；使用既有 `approval-audit-result.read` 和精确 Result／Project 或 Tenant Scope，由目标最终授权。无新权限动作、数据库 migration、服务或业务写命令。

投影沿用 `MaterialReferenceProjection v1` 基本形状：schema 1、source `approval-audit-results`、resource_type `approval_audit_result`、稳定 ID、Tenant、Project ID、result_kind、status `recorded|withdrawn`、revision、digest 与 items。items 只包含 slot 作为 relation_key、File ID、冻结 Version／相同 Evidence source revision、display_name；按 relation_key／File ID／Version 排序，digest 是当前引用项 canonical JSON 的 SHA-256，不是金额或原字节摘要。

GET 仅读 Results 自有单行完整 snapshot，禁止调用 Project、Contract 或 Evidence，不签下载 URL、不追加下载审计。只发布当前 revision 引用；纠正替代引用、撤回状态及其保留引用明确可解释，不改变历史或擅自删除文件。撤回不是自动反向写 Project。

Tenant／Result／Project／kind／status／revision、File／Version／slot／显示名必须有效；畸形自有事实失败关闭，空成功必须为 `items:[]` 与其确定摘要。实现事实中 slot 是 1～100 字符的业务描述，可在同一 slot 下保留不同 File／Version；新投影不得将它偷偷变成生产阶段键或增加唯一 slot 写约束，只拒绝重复的 slot＋File＋Version。投影最多 100 项，超限失败关闭、不截断伪造完整结果；这不是改变既有写请求上限。不构造扫描全文件的聚合端口。响应 no-store，不含金额、合同金额快照、Actor 密码、Bucket、Object Key、长期密钥或下载地址。

## 验收与非目标

单元／HTTP 验证两 kind、排序／digest、纠正／撤回、空、畸形事实、上游零调用、无写入和未额外授予 amount.read 的精确 Scope 正反例；真 PG18 检查 owned tables canonical，真实目标 verifier／IAM 与正式链分别登记。编译或跳过不算运行通过。

Project canonical Results Link、消费者、Workload Policy 及其 migration、正式兼容发布、联合空目标恢复和原连续 C8 是后续闭包，不由来源端的单项通过关闭。当前不重写 Results 金额／状态机，不建设通用报告平台，不增加 C6 或前端 UI 功能。
