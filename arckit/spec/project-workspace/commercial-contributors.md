# Project 商业参与分工

问题 A → 可验证结果 B：通用 participant 无法区分商业参与与生产参与 → 项目可以单独添加、移出商业参与人员，原 OA 的经营参与人栏目使用同一项目的真实名单，分权和历史可复核。

## 事实和不变量

Project 拥有 Assignment，不新增成员服务或人员副本。增加有界稳定职责 `commercial_contributor`；原三个 role、负责人门槛、幂等 hash 和历史含义保持。参考界面显示“经营参与人”，只选已有 active Person 的 ID／revision；People 继续拥有人员、IAM 继续拥有账号及能力。当前名册不含登录账号、部门及公司岗位，界面不得用引用码或 IAM 授权标签冒充这些字段。

新增 `project-workspace.assignment.commercial-contributor.manage`，精确 Project 或 Tenant scope。旧 `assignment.manage` 和旧 participant／owner 管理能力不能管理新职责；新能力也不能管理旧职责。读取沿用 Project read，取消项目不允许新增／结束。结束必须由服务端读取 Assignment 的实际职责，不能信任请求附带的 role。

添加只允许 Person、无自定义专业或日期。Project 在自己的事务中追加 Assignment、Project revision、历史和回执；同人同职责 active 唯一，负责人不自动成为参与人，同人不同职责可以共存。移出只结束 Assignment，不改 People／IAM／合同／奖金；重入使用新 ID。失败零写，成功命令先于外部 People 调用重放。未知回复只继续相同 command ID、expected revision 和内容；409 明确复核，不自动换 revision 覆盖。

## 兼容及验收

000011 仅扩大本库 role CHECK 并限制新职责为 person，保留旧三类全部行和约束／索引。旧二进制不支持 schema 11，部署必须走兼容闭包及备份／空目标恢复；不以旧二进制直接回滚新库。Project reference／policy fact v1 增加已知 enum，所有消费者检查自身需要的职责；Production 仍只接受 production_owner，Bonus 仍按其类型校验。IAM 旧策略不授新职责能力，不自动选新 Policy 或扩大旧 Grant。

验收覆盖：原 520px 抽屉选择、取消零写、添加关闭回读、确认移出、重入；重复／并发／丢回复／People 故障；管理与只读、生产成员、旧全分工管理员的反向拒绝、相邻 Project／Tenant；数据库历史、旧 hash／数据保持、恢复后继续、独立启动和正式入口浏览器分层留证。

本切片不负责父包权限向子项传播、离职人员新指派、公司岗位资料、成员自动开户或完整 OA。原站现场与这些缺口保持单列，不用全权夹具代替权限验收。
