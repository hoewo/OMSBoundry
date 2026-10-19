# Demo Validation 2：Level A 总入口

这里汇总六个业务应用的独立 Demo。总入口按顺序执行，每次只运行一套 Demo，以降低普通开发机的资源压力：

```bash
deployments/demos/ops/level-a audit --confirm-destroy-demo-data
```

对每个应用，入口都会执行：

1. 初始化本地专属凭证；
2. 删除该 Demo 先前的合成数据；
3. 从空数据库／对象卷构建并运行完整业务场景；
4. 在同一环境执行第二个完整场景或幂等重放；
5. 删除该 Demo 的容器、网络和专属数据卷。

总入口不会启动 Nginx、Gateway、中心 IAM、Workbench 或完整 OA，也不会删除六个 Demo 之外的卷。Project 与 Design 只使用无业务所有权的公开 reference-read 契约桩。

运行门禁和业务能力门禁分开报告：

- `Runtime gate` 说明六套最小环境是否都能完成 fresh-state、run、replay 和 cleanup；
- `Business capability gate` 读取 [`level-a-capabilities.tsv`](./level-a-capabilities.tsv)，说明规格要求的能力簇是否完整；
- 只有两者都通过，输出才可以是 `LEVEL_A_STATUS=passed`。

2026-09-25 的最终聚合审计中，六套运行门禁和六项业务能力门禁均已通过，输出为 `LEVEL_A_STATUS=passed`。这解锁了 Level B 高保真参考 OA 扩展，但不等于 Nginx、Gateway、正式 IAM、公司 Ubuntu 或公网入口已经验收。详细范围以 [Demo Validation 2 规格](../../arckit/spec/assembly/demo-validation-2.md)为准。

同一入口也包含在 `v0.1.0-demo.11` Linux/amd64 无源码开发包中。发布模式不要求目标机安装 Go，不读取源码，不构建镜像，也不联网拉取；六个静态驱动分别在已装载的应用镜像中执行，Project／Design 只额外使用包内的公开 reference-read 契约桩。目标机仍使用上面的同一条 `level-a audit` 命令，并在六项验收完成后继续运行子组合和完整 OA 门禁。

最近一次本地报告保存在被 Git 忽略的 `deployments/demos/.runtime/level-a-report.tsv`，各 Demo 的完整输出也保存在同一目录。查看最近结果：

```bash
deployments/demos/ops/level-a status
```
