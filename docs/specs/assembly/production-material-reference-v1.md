# Production 成果资料引用 v1（BF-073 的 C4 来源切片）

问题 A：C4 的设计／咨询成果在 Production 中存在，但不能由 Project 的资料组合读取。可验证结果 B：Production 发布单个 Work Item 当前有效的 File ID／Version 引用，Project 后续只通过版本化端口和精确工作负载能力装配，不复制生产表或文件。

## 本轮范围与不变量

先交付来源端：`GET /api/v1/references/production-work-items/{id}/materials`；返回 `MaterialReferenceProjection v1` 的既有基本形状，source 为 `production-operations`、resource_type 为 `production-work-item`。不改旧标量 reference-read 响应，不新增写命令、migration、服务或权限动作。使用既有 `production-work-item.reference.read` 和目标最终授权；其既有 Capability Manifest 只允许精确 Work Item Resource Scope（或 Tenant Scope），不能用 Project Scope 代替。返回 Project ID 供消费者再次检查关联归属；调用方从配置注入端点。

投影只读 Production 自己的当前事实；不在线调用 Project、People、Evidence，不让一次文件资料读取变成依赖链或写入下载审计。保留源 ID、Tenant、Project、status、revision 和 digest；items 按 relation_key／File ID／Version 确定性排序，digest 是排序后 items 的 JSON SHA-256，不伪装为文件字节摘要。

每项包含 `relation_key = stage-key + "." + requirement-key`、`file_id`、不可变 `version`、`evidence_source_revision`、`display_name`。只发布未被取代的引用；not-applicable 阶段不发布当前资料，重开但仍保留当前引用的阶段继续发布。设计与咨询按各自注册类型／版本解释阶段，不把未知阶段当成空。取消状态仍显式返回，当前未取代引用不是自动删除的历史事实。

错误 Tenant／Work Item／Project、未知类型／阶段／requirement、非法 File／Version／source revision、重复当前 stage＋requirement 或超过 100 项都失败关闭。空成功为 `items:[]` 与其确定摘要；来源失败是错误，不返回伪造的 empty。响应 `Cache-Control:no-store`，不含金额、人员密码、Bucket、Object Key、AccessKey 或下载 URL。

Production 的 owned-details 多表读取在只读 repeatable-read 事务中完成，让 Work Item revision、阶段状态和成果引用来自同一个数据库快照；不引入锁写、审计写入或新事实。阶段必须与注册类型的完整阶段集合一致。

## 验收与后续消费者

来源单元验证两类型、确定排序／摘要、替代与 not-applicable、空、重复／错范围／畸形输入及零来源调用、零写入；HTTP 验证匿名 401、缺 action／邻资源／Project-only Scope 403、合法精确 Work Item、Tenant Scope、来源错误和禁止缓存。随后以真实 PostgreSQL 18 和真实目标 IAM 凭证验证 GET 前后 canonical facts 相同，才能记为来源运行通过。

下一切片才增加 Project 的 canonical Production Resource Link、必要前向约束 migration、配置端点／单 action 精确 Workload Policy 和两层资料消费者；必须验证错 Project／来源、revision drift、局部失败、未选缺席、联合备份和空目标恢复。当前来源端通过也不能关闭 C4 资料装配、C5／C6、原连续 C8 或浏览器。本轮不建设通用资料平台、报表库、扫描全部 Evidence 或替来源修改已确认事实。

## 来源完成后的 Project 消费切片

2026-10-01 来源隔离真库／真 IAM 验证通过后，开始第二切片：新增闭合的 `production-operations / production-work-item` Resource Link；Project migration 000008 仅扩充自己的关联约束，不复制生产表或新增资料事实。链接写入先由精确工作负载票据读取来源，校验同 Tenant／Project、ID 和可选 source revision；幂等重放不重新依赖来源在线。旧 Design Link 仍保留原义，不自动迁移或猜测。

Project 以配置注入 `PRODUCTION_BASE_URL`（正式 IAM 禁止长期 `PRODUCTION_ACCESS_TOKEN`），单 action `production-work-item.reference.read`／单 Work Item Scope 读取资料投影，再单独以 Evidence metadata.read／单 File Scope 查元数据。typed reader 校验固定来源、类型、Project、revision、确定排序、唯一 relation、digest 和明确 items 数组；失败关闭，不将缺字段或错误摘要当成空。当前来源 revision 或文件版本变化展示 drift，不写回双方事实；局部失败不隐藏独立成功资料，空来源的 revision 变化也不能当成无变化的 empty。

部署闭包包括 Production 新端口、Project 新二进制／000008 执行者、IAM Workload Policy、D1 配置和恢复工具；BF-082 的入口闭包独立保留。源码／隔离测试通过不等于主站发布，正式升级前联合备份及空目标恢复、实际入口／浏览器和连续 C8 仍是门禁。

## 真实 Workload 只读连接补证

C5／C6 同类链取得运行证据后，下一小批将 Production HTTP 加入同一个有界专项：Project 自有 Workload Key 向真实 IAM8 申请单 `production-work-item.reference.read`／精确 Work Item，再独立申请 Evidence metadata.read／单 File。Production source seed 经自己的 Service／Store 创建、设置负责人和交付引用；创建前 Project／Assignment／Evidence 引用明确为合成测试提供者，资料 GET 使用正式目标 verifier、自己的 PG18 和零上游调用。

验收四种来源材料组合、Production 单独暂停后的局部降级与恢复、成功 Link 本地重放、Evidence 故障，以及五种单动作精确资源审计与各自零 GET 写入。总限额不超过 0.55 CPU／512 MiB、内部网络／零公开端口；保持主站 13 库／17 ID，结束停止专项、保留失败和数据。此层不修改 Main／restore20，不代表真实来源写流程、浏览器、公网、OSS、联合恢复或原连续 C8。
