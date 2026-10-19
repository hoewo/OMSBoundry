# ADR-0035：D1 升级制品必须形成契约与 Migration 兼容闭包

日期：2026-09-29  
状态：Accepted

## 背景

公司 D1 从 `v0.1.0-test.5` 提升 C8 Project／Evidence 实现时，第一份候选发布只替换 Project Workspace 与 Evidence Service。实际升级暴露了两个不能由“只发布发生业务改动的镜像”覆盖的边界：

- 新 Capability Manifest 增加 `capability_namespace` 后，旧 IAM 解析器以未知字段失败，说明契约发布者变化会同时约束清单消费者；
- Project 新镜像已完成前向 migration 后，旧 Project 二进制因数据库 migration 数量高于自身嵌入清单而拒绝启动，说明数据库 schema 与可运行二进制也构成兼容集合。

目标机已有完整联合备份，故没有修改历史数据或执行 down migration。现场先恢复旧 IAM／入口，再保留能识别新 schema 的 Project 镜像；随后补发兼容 IAM，并逐服务恢复健康。这次事件证明镜像摘要正确、单服务测试通过和 Compose 可解析仍不足以证明发布集合可升级。

## 决策

1. 一次 D1 发布以“兼容闭包”而不是“源码 diff 中的服务列表”确定镜像集合。闭包至少覆盖：
   - 修改后的契约、Manifest、策略或 Schema 的全部运行时解析者；
   - 应用 migration 的执行者，以及升级后必须继续读取该 schema 的服务二进制；
   - 受该变化约束的 Gateway、Workbench、连接器与恢复工具；
   - Suite Manifest 实际选中的运行应用。
2. release 构建必须记录每个变更制品的直接消费者，并在打包门禁中拒绝“新契约／新 migration + 旧解析者／旧 schema 清单”的已知不兼容组合。
3. D1 升级执行 `preflight → 联合备份 → 保留命名卷并移除候选项目容器／遗留 one-shot 容器 → migration → 候选栈重建 → health/check → 入口验收`。调试阶段仍按兼容单元串行定位；一旦闭包和备份门禁通过，正式升级用同一候选发布整体重建容器，避免 Compose 复用旧 migration 任务或混合旧解析者。
4. migration 只前向。只要任何应用已经写入新 schema，就不得通过直接重启旧二进制声称回滚成功；真正的数据回退必须把升级前联合备份恢复到新的空目标，再运行旧发布。
5. 运行环境可以在事故恢复期间短暂处于记录清楚的混合版本，但只有兼容闭包中的目标镜像、`d1 check`、入口授权和业务读路径全部通过后，才能把该候选记为已部署。混合恢复状态不是可推广发布物。
6. 发布报告必须保留失败尝试、实际 migration、恢复顺序、最终镜像摘要和未执行的破坏性回退，不能只记录最终绿色状态。

## 原因

- Capability Manifest、Suite Manifest 和版本化 Schema 都是跨进程契约；解析者虽未发生业务代码改动，也可能必须随契约共同提升；
- migration 与二进制清单的严格门禁正在正确阻止旧程序误读未来 schema，不能为了方便回滚而放松；
- 串行替换让 2 vCPU／2 GiB D1 和开发机都保持可观察，并把失败影响限制在单个兼容单元；
- 兼容闭包仍允许稀疏发布：未选应用和真正无关的镜像不需要重发，但“无关”必须由契约依赖证明，而不是凭文件改动推断。

## 后果

- release 构建和评审需要一份机器可校验的契约消费者／migration 兼容账；`test.10` 已证明 IAM、Gateway、Workbench、migration 执行者和目标应用都可能属于闭包。候选 IAM 已能在数据库变更前自动解析全部 Capability／Assignment／Workload 文档，发布包回归也固定了遗留 one-shot 容器清理与 Evidence 备份恢复方式，但通用消费者依赖图仍待自动生成；
- 小改动可能要求连带发布 IAM、Gateway 或其他解析者，但不改变任何应用的数据所有权；
- 直接二进制回退不再作为已执行 migration 后的恢复方案；联合备份与空目标恢复是必须保留的运维能力；
- 目标机验收除容器 health 外，还必须经过真实 `Nginx → Gateway → IAM → Target App` 登录／授权，以及受影响的跨应用读路径。

## 被否决方案

### 只发布 Git diff 中改动的应用

它忽略 Manifest、策略、Schema 和解析器之间的运行兼容关系，本次旧 IAM 对新 Capability Manifest 的拒绝已经直接否定该方案。

### migration 后直接拉起旧应用镜像

旧二进制不认识未来 schema；放松门禁会把明确失败变成静默误读。需要回退数据时必须恢复升级前备份到空目标。

### 未验证闭包和备份就一次性重建整套 D1

这会扩大共享环境的失败面，也不利于定位哪个兼容边界被破坏。低资源 D1 先逐兼容单元定位并通过预检、备份和候选健康门禁；正式升级再保留命名卷、清理旧容器并以同一发布整体重建，避免旧 one-shot 任务和旧解析者被复用。

## 2026-09-29 运行证据补充

`v0.1.0-test.10` 又提供三条直接反证与修正：

- Suite Manifest 新增 `metric_projection` 后，旧 Gateway 与旧 Workbench 都因严格未知字段拒绝启动，因此两者进入同一发布闭包；
- Compose 会复用同名、已经退出的旧 migration 容器，不能把“重新执行 up”当成候选 migration 已运行；
- Evidence 备份后的单服务恢复不能用会重新求值 migration 依赖的 `compose start`，必须明确 `up --no-deps --no-build --wait`。

修正后的完整升级以校验有效的 `20260929T132146Z` 为前置备份，所有 migration 退出 0，12/12 长期容器 healthy，`d1 check` 通过，正式 Project／Evidence 多版本事实在重建后保持。该证据关闭当前选装集合的手工兼容闭包，但不关闭通用消费者账和独立空目标恢复门禁。

同日 Production 个人工作投影加入 People Policy Fact 依赖时又验证了失败恢复规则。第一份 `test.16` 组合把 `PEOPLE_BASE_URL` 误放入相邻服务环境块，Production migration 在写业务数据前拒绝启动；命名卷和升级前备份保持。由于 Production 未运行，下一次完整升级按门禁正确拒绝创建可能遗漏 Production 数据的新备份，没有把“忽略未运行服务”作为修复。现场先校验上一份完整备份，再用修正后的不可变 `test.17` 候选恢复 Production 健康，最后重新执行完整升级。正式备份 `20260929T151745Z` 的 11 项 checksum 全部通过，全部 migration 成功，13/13 长期服务 healthy，`d1 check` 通过。该事故支持“先恢复候选健康，再重新走完整备份与升级”，不支持削弱备份门禁。
