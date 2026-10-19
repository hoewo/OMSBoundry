# 参考 OA 合同材料引用 v1

2026-10-04，候选实施，未验收、未部署。

问题 A → 可验证结果 B：合同、协议和定案附件仍是禁用控件，合同流程无法继续 → 复用原 AttachmentUploader／预览组件和既有 File Transfer SDK，Evidence 完成不可变文件版本后，由 Contract 原生权限登记自己的业务引用；刷新及未知回复继续仍指向同一版本。

Contract 只保存 Material ID、Tenant／Contract、明确 kind（contract／amendment／finalization）、可选本合同 Amendment ID、File ID／Version、冻结名称／大小／媒体类型／SHA 和追加／移出历史。不保存 Object Key、URL、Provider 或长期凭据，不读写 Evidence 数据库。Workbench 无业务表。该新写入路径只服务 Reference OA 显式扩展，不迁移、不删除、不覆盖旧内嵌 Evidence 历史；M10-004 的冻结／迁移门禁仍独立未完成。

登记要求可信 IAM Actor、`contract-ledger.evidence.manage`、精确 Contract／Tenant Scope、稳定 command ID、expected material revision。追加之前由配置注入的 Evidence 版本端口复验同 Tenant、available、File／Version／revision、摘要及有界元数据；正式配置使用独立 Workload Key 换单 `evidence-service.metadata.read`／精确 File Credential。未选外部端点时结构化合同仍能启动、读写，附件能力明确不可用，不静默写入旧存储。

每合同引用版本从 0 递增，最多 100 个有效引用。同 kind／Amendment／File／Version 不重复；移出仅追加结束标记，旧引用和命令不可修改删除。协议目标必须是本 Overlay 的未取消协议；正式协议可补材料，但不能因此改其条款。取消合同／关闭合同拒绝新操作；已有资料可读。材料保存不确认合同、协议或定案，不产生付款。提交采用本库事务和版本锁；同命令重放先读取自己的 receipt、不要求 Evidence 再在线；依赖故障、错范围和版本冲突零新增事实。

上传与业务登记为两个提交。完成的 File ID／Version 在元数据回读或业务登记失败前保存到按 Tenant／Subject／Project 隔离的前端暂态，显式继续复用原命令与版本，不重新上传、不物理清理。服务端仍复验一切，不信任暂态。下载和预览通过 SDK 向 Evidence 申请短期访问，由 Evidence 最终授权；不能把 metadata 权限当成字节权限。

验收分别包括领域／HTTP负例、真实 PG18／并发与历史、原组件三类上传／预览下载字节／移出、回复丢失／撤权／只读／相邻租户资源拒绝、带引用和 Evidence 字节的联合空目标恢复、正式 Workload／入口以及版本化兼容闭包。未运行项不记通过，单合同旧 Archive v1 仍拒绝遗漏新引用。本批不宣称 OSS 分离、遗留迁移或完整 OA 已完成。
