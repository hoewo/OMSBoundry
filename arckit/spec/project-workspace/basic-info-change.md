# Project Basic Info Change v1

问题 A → 可验证结果 B：原 OA 在同一基本信息抽屉可更换／清除两类负责人，当前仿制只能后补生产负责人 → 同一保存原子更正 Project 自有资料及选定分工，并保留各项授权、并发与历史。

## 约束与范围

- 新增 `POST /api/v1/projects/{id}/basic-info-changes`，旧 profile-correction、assign／end 及其命令 hash 不变；不新增 Capability、数据库字段或小系统。
- 仅 active Project；命令包含 expected revision、固定业务原因、可选核心资料和最多两个明确的 set／clear owner 动作。缺省不改分工，不能把缺失字段解释成清除；只允许 business_owner／production_owner。
- 核心资料须 `project-workspace.profile.correct`；每个分工须已有对应 manage 或通用 assignment.manage，任何一项缺权限则整个命令零写入。客户端仍不决定授权。
- People Person ID／revision 经版本化引用校验，冻结新显示快照。旧 Assignment 仅追加结束信息，新人产生新 Assignment；同人不重写旧快照。清除为历史结束，非物理删除。未来生效的分工不能被此即时命令隐式取消。
- 同一个 Project 事务包含资料、旧分工结束、新分工、一次 revision、可解释历史和命令回执。任何约束失败全部回滚；同 payload 重放返回已存结果，不依赖 People 在线；并发版本不覆盖。
- 未变化的资料／同人／原已空的清除只保存命令回执，不制造 revision 或历史。Client、层级、外部引用、金额和已确认业务不变；结束 Project 分工不改 IAM 账号或 People 名册。
- 历史继续使用已有 `project_details_replaced`，payload 显式标记 `project-basic-info-change-v1`、前 revision 和每项分工变化，不伪造多个旧命令。单独旧更正历史保持原格式。
- 原界面只有同一个“编辑”抽屉，资料和两个负责人分别禁用／授权，保存走上述原生原子动作。生产负责人后补不再要求额外入口。暂无权限／人员目录异常不能清空当前分工。

## 验收

普通项目／包同界面保存、更换两人、清除、同人无变化、旧历史／引用保持；部分缺权限、错误 Tenant／Project、People 不可用／错 revision 零写入；重放离线、并发 stale、失去权限后拒绝、读回失败只重读。源码／单元、独立 PG、实际服务启动和合成浏览器分层记账，不拼成正式公网用户全链。

## 不负责

资质单位、管理费和甲方目标映射、经营／生产、流程推进、IAM Grant 更改及正式部署属于后续切片；本批不关闭 P02 全项或完整 OA。
