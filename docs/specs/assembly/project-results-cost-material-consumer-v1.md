# Project C5／C6 资料消费者 v1

问题 A：Results／Cost 的资料来源已经能读，但 Project 材料清单仍缺它们。可验证结果 B：Project 通过显式 canonical Link、目标原生精确能力读取来源，再单独读取 Evidence 元数据；清单保留两层来源、版本和失败状态，不复制来源事实。

## 约束与不负责

只增加 `approval-audit-results / approval_audit_result`、`cost-outsourcing / cost_entry`、`cost-outsourcing / outsourcing_engagement` 三种 Link。Project 000009 前向 migration 仅扩大自己的约束，保留旧 Design／Production Link、ID、revision、命令和历史；不推断或自动新建关联。Results 仍拥有完成结果，Cost 仍拥有成本／外协与评分／付款建议，Evidence 独占文件与下载；不改这些流程、金额、统计或业务数据库。

链接命令先校验同 Tenant／Project、来源 ID／类型、状态 recorded 和可选 revision，再由 Project 自有命令提交；成功命令重放优先本地 receipt，不依赖来源在线。现有链接遇来源 withdrawn 时保留其 ID／revision／digest／状态，标记 invalid／source_withdrawn 且不读取 Evidence；不悄悄删除关联，也不把撤回材料显示为正常可用。

三个来源都使用配置注入端点；Cost 可选 `COST_BASE_URL`，未选不是启动前提。正式 IAM 禁止长期静态 Token：Results 使用单 `approval-audit-result.read`／精确 Result，Cost 根据固定 resource_type 分别使用单 `cost-entry.read` 或 `outsourcing-engagement.read`／精确目标资源；不能为方便签发两种动作或金额／写能力。Results 既有 Completion Project Scope 保持，Workload Policy 显式增加 Result 类型及两个 Cost 精确类型。

typed HTTP reader 验证 schema/source/type/Tenant/ID/Project、正 revision、recorded／withdrawn、Results kind、明确且最多 100 个 items、canonical File UUID、slot 1～100、冻结 Version 等于 Evidence source revision、trim 后不超过 500 字的 display_name、按 slot／File／Version 排序无重复 tuple，以及完全相等的 items JSON SHA-256。一个 slot 多文件是合法业务事实，不套用 Production 唯一 relation 约束。失败不是 empty。

只有 active Link 参与材料清单；来源 revision 与文件版本变化明确 drift，空来源漂移不忽略。Evidence 只使用 metadata.read／单 File Scope；没有 download、Object Key、OSS SDK 或长期密钥。单来源故障局部降级，不改其他材料和任何 GET 业务事实。页面只扩充现有类型选择，不增加新 UI 平台。

## 验收门禁

单元／HTTP 测试覆盖三类型、两个原生 Cost 动作互斥、错来源／Project／Tenant／revision／摘要、明确空／撤回／漂移、未选／unlink 缺席、局部故障、零写入和离线重放。真实 PG18 验证 8→9 旧事实保持与重复 migration，以及资料读取 canonical 相等、单库 dump→空目标恢复并继续追加。测试提供者只证明契约消费者，不能与独立来源 IAM 证据拼成完整正式链。

正式交付仍须 Source 新端口＋Project 二进制／000009＋IAM Policy＋D1 端点的兼容闭包、联合备份／空目标恢复、真实 Workload、公网入口／浏览器与原连续 C8。主站未升级前不宣称 C5／C6 或 OA 整体完成；迁移后不能直接换回旧二进制冒充回退。

## 消费组件通过后的下一层检查

单库／测试端口通过后，以一个私有 PG 和有界 runner 启动真实 IAM8、Project／Evidence 服务及 Results／Cost 原生 HTTP 进程；Project 使用正式 Workload Key 向 IAM 换取单动作／精确资源 Credential，由各目标正式 verifier 最终验证。来源种子必须由各自应用 Service／Store 生成，写入前相邻引用可使用明确测试提供者；不能写畸形 snapshot 或跨应用导入领域模型。该层只证明受保护只读连接，不冒充来源写业务流程、公网／Gateway、联合恢复或原连续 C8。保持主站摘要／容器不变、零公开端口，停止探针并保留结果。
