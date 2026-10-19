# Project Workspace 独立 Demo

这套入口只启动 Project Workspace、它自己的 PostgreSQL，以及一个只实现公开 reference-read 契约的最小桩；不启动 People、Contract、Money、Evidence、Design、Nginx、Gateway、中央 IAM 或 Workbench。

```bash
deployments/demos/project-workspace/ops/demo run
```

引用桩不是业务系统，不写 Project 数据库，也不替 Project 生成成功状态。它只用固定的版本化 JSON 表示 Person、Group、Contract 和 Evidence 三种结果：可用、不存在、暂不可用。

自动演示会完成：

1. 模板 IAM 认证，以及匿名和错误令牌拒绝；
2. Project 草稿创建、更新、Command ID 重放、同键异载荷冲突和过期 revision 拒绝；
3. 不完整立项进入 `pending_completion`，不会伪装为 active；
4. Person／Group Assignment，引用不存在和不可用时零部分写入；
5. 经营负责人和生产负责人齐备后完成立项；
6. Contract／Evidence Resource Link、同命令重放、解除关联；
7. 查询时同时显示 `available` 与 `unavailable` 引用状态；
8. Project／Assignment 公开引用、revision 1～9 历史和伪造身份 Header 清除；
9. 受控取消，以及取消后的写入拒绝。

常用命令：

```bash
deployments/demos/project-workspace/ops/demo replay
deployments/demos/project-workspace/ops/demo status
deployments/demos/project-workspace/ops/demo logs project-workspace
deployments/demos/project-workspace/ops/demo credentials
deployments/demos/project-workspace/ops/demo down
deployments/demos/project-workspace/ops/demo reset --confirm-destroy-data
```

数据库与引用桩都不暴露宿主机端口，Project HTTP 只绑定 `127.0.0.1`。每次 `run`／`replay` 创建一个带唯一编号的合成项目；同一轮内的创建、Assignment 和 Resource Link 会使用相同 Command ID 重放并核对稳定身份。`reset` 只删除本 Demo 的数据库卷。
