# Design Operations 独立 Demo

这套入口只启动 Design Operations、它自己的 PostgreSQL，以及一个只实现公开 Project／Assignment／Evidence reference-read 契约的最小桩；不启动真实 Project、People、Contract、Money、Evidence、Nginx、Gateway、中央 IAM 或 Workbench。

```bash
deployments/demos/design-operations/ops/demo run
```

引用桩不是业务系统，不写 Design 数据库，也不替 Design 生成成功状态。它只返回成功、不存在和暂不可用三类版本化引用结果；真实四阶段状态机、负责人、成果引用、revision、历史与约束都由 Design Operations 执行。

自动演示会完成：

1. 模板 IAM 认证，以及匿名和错误令牌拒绝；
2. Project 引用三态、Work Item 创建、Command ID 重放与 active 唯一约束；
3. Assignment 引用三态、错误角色、负责人失效和显式换人恢复；
4. Evidence 引用三态、缺成果、成果登记、幂等重放和替换历史；
5. `scheme` 完成、`preliminary-design` 显式跳过，并在其余阶段完成后恢复补做；
6. 不可跳过的施工图约束，以及恢复早期阶段时对后续阶段的级联回退；
7. Evidence 降级展示、失效成果替换、施工图重开和再次完成；
8. 稳定 reference-read、revision 1～21 不可变历史、受控取消和终态写入拒绝。

常用命令：

```bash
deployments/demos/design-operations/ops/demo replay
deployments/demos/design-operations/ops/demo status
deployments/demos/design-operations/ops/demo logs design-operations
deployments/demos/design-operations/ops/demo credentials
deployments/demos/design-operations/ops/demo down
deployments/demos/design-operations/ops/demo reset --confirm-destroy-data
```

数据库与引用桩都不暴露宿主机端口，Design HTTP 只绑定 `127.0.0.1`。每次 `run`／`replay` 创建一个带唯一 Project ID 的合成 Work Item；同一轮内的创建、负责人和成果命令会使用相同 Command ID 重放并核对稳定身份。`reset` 只删除本 Demo 的数据库卷。
