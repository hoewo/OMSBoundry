# People & Groups 独立 Demo

这套入口只启动 People & Groups 和它自己的 PostgreSQL，用来验证业务架构，不启动 Nginx、Gateway、中央 IAM、统一工作台或其他业务应用。

```bash
deployments/demos/people-groups/ops/demo run
```

脚本会生成仅保存在本机、权限为 `0600` 的 `.env`，构建并启动两个容器，然后通过真实登录表单和受保护 API 自动完成：

1. 未登录与错误模板令牌失败关闭；
2. 设计交付组类型、设计师／主创建筑师角色和设计交付能力目录；
3. 两名人员、一个设计交付组、成员加入和群组能力声明；
4. 发布包含两名成员与群组能力的 revision 4 花名册快照；
5. 修改负责人资料、绑定外部 IAM Subject，并证明活动 Group revision 随人员引用变化推进；
6. 调整负责人角色，以及过期 revision 写入被拒绝；
7. 结束普通成员、停用该人员，并证明 inactive Person 不能重新加入；
8. 结束群组能力、停用能力目录，并证明 inactive Definition 不能产生新分配；
9. 发布只含一名主创建筑师的 revision 9 快照，证明 revision 4 快照内容和 SHA-256 不变；
10. 核对名册快照不包含 IAM Subject、旧 Membership／Capability 历史仍可读；
11. 结束最后一名成员、归档群组，并拒绝归档后写入；
12. Command ID 幂等重放、人员／目录终态、历史与浏览器页面核对。

常用命令：

```bash
deployments/demos/people-groups/ops/demo replay
deployments/demos/people-groups/ops/demo status
deployments/demos/people-groups/ops/demo logs people-groups
deployments/demos/people-groups/ops/demo credentials
deployments/demos/people-groups/ops/demo down
deployments/demos/people-groups/ops/demo reset --confirm-destroy-data
```

数据库不暴露宿主机端口，应用只绑定回环地址。`run` 成功后会打印本次群组的浏览器地址；访问令牌可由 `credentials` 查看。

当前 Demo 已覆盖 Demo Validation 2 规定的 People & Groups Level A 能力簇。IAM Subject 只作为外部 UUID 引用保存，不读取 IAM 数据库、不持有密码／会话／角色授权，也不进入 Roster Snapshot。People 能力清单可以标记为 `complete`；六套独立 Demo 的 Level A 最终结论仍须由统一总门禁重新运行后产生。
