# OA C7 人类工作区与直接纠错／故障恢复实验

日期：2026-09-28  
结论：`human-workspace-and-direct-recovery-supported / formal-d1-and-payable-suggestion-pending`

## 1. 本轮问题与结果

**问题 A**：C7 Bonus Distribution 已有独立领域核心、PostgreSQL、受保护 HTTP、正式 IAM 和六进程真实来源组合，但操作者仍不能在页面中理解来源、策略、分配、权限、纠错关系与失败恢复。若只保留 API，无法验证“参考 OA 奖金业务可以由可用的小应用完成”。

**可验证结果 B**：C7 现在提供 Project 范围的人类工作区。操作者可查看经营／生产奖池，建立草稿，发布、取代、撤回并查看 Pool／Allocation 历史；页面动作来自目标服务的 `allowed_actions`，金额可见性由服务端独立控制。六个真实应用进程和六个自有数据库又完成正常发布、追加式纠错、依赖停机零写入、恢复后成功、撤回与浏览器实看。

本轮不验证正式 D1 Suite Manifest、Workbench／Gateway／Nginx 装配、联合备份恢复、C7→Money Payable Suggestion、C8 或真实 Ubuntu。

## 2. 实现增量

- `bonus-distribution` 根页面与同源脚本，使用当前版本化 API，不建立第二套页面后端；
- Project 范围的经营／生产筛选、列表、详情、来源与策略快照、参与人分配、纠错关系和历史；
- 建立经营／生产草稿、发布、取代和撤回操作；
- 页面只根据服务端 `allowed_actions` 展示动作，金额缺权时显示“金额未授权”；
- Token 只保存于当前标签页 `sessionStorage`，没有写入仓库、日志或长期证据；
- 401、403、409、422、503 都保留页面上下文并给出可重试反馈；
- 稳定交互事实记录于 `arckit/interaction/bonus-distribution-workspace/`。

## 3. 可重复步骤

普通检查：

```sh
go test ./backend/services/bonus-distribution/...
go vet ./backend/services/bonus-distribution/...
sh -n acceptance/c7-live-source-composition/run-isolated
```

六进程组合：

```sh
acceptance/c7-live-source-composition/run-isolated
```

夹具会清理旧证据、启动 PostgreSQL 18.6，并按顺序启动 IAM、Project、People、Contract、Money 和 Bonus。顺序启动是验收资源控制，不改变应用依赖；每个应用仍有自己的进程、数据库和 migration 边界。

可选浏览器保持模式：

```sh
C7_HOLD_AFTER_ASSERTIONS=true acceptance/c7-live-source-composition/run-isolated
```

脚本完成业务断言后保持服务，仅把短期浏览器 Token 写入临时运行目录。操作者打开脚本输出的本地地址，Token 进入当前标签页后即可检查真实页面；结束脚本会删除临时目录并停止全部进程。

## 4. 自动化业务证据

最终一次非保持执行返回码为 0，保持执行也在浏览器检查前完成全部断言。结果包括：

| 场景 | 直接证据 |
| --- | --- |
| 人类入口 | 页面和同源脚本返回 200；未授权 API 返回 401 |
| 正常经营 | 真实 Settlement／Person／Assignment 建立并发布经营奖池 |
| 正常生产 | 真实 Receipt Set／Person／Assignment 建立并发布生产奖池 |
| 精确金额 | 两个 `106500` 分来源均按 `reference-oa-bonus-v1` 得到 `10000` 分 |
| 依赖故障 | Contract 停机时新经营草稿返回 503，Pool 数保持 2 |
| 恢复 | Contract 恢复后同一业务意图建立替代草稿 |
| 纠错 | 原经营 Pool 被新 Pool 原子取代，双向关系与 revision 可查 |
| 撤回 | 生产 Pool 追加撤回 revision，历史保持 |
| 最终状态 | 3 个 Pool：1 published、1 superseded、1 withdrawn；2 种类型、1 个 Project |

运行期没有共享业务表、静态来源 Token、跨库事务或按姓名匹配。工作负载凭证仍由 Bonus 使用私钥向 IAM 换取单 action、精确资源的短期凭证。

## 5. 真实浏览器结果

浏览器实际加载 3 条奖池，并显示：

- 经营奖金 `superseded`，revision 3，金额 100.00 元；
- 生产奖金 `withdrawn`，revision 3，金额 100.00 元；
- 替代后的经营奖金 `published`，revision 2，金额 100.00 元。

选中被取代经营奖池后，页面正确显示服务端动作、Project revision、奖金与未分配金额、冻结 Settlement revision／digest、版本化策略、Person／Assignment、Allocation 和“被新 Pool 取代”的纠错关系。浏览器控制台没有错误。

首次浏览器尝试使用了已过期的五分钟短期凭证，页面明确显示“访问凭证已失效”，没有冒充空数据。换取新凭证后读取成功，证明 401 恢复提示和真实数据加载都工作。

## 6. 运行中发现并关闭的问题

1. 同时启动多个 Go 服务和 migration 会在本机 PostgreSQL 上造成就绪误判。夹具改为逐服务等待 ready；没有放宽业务断言。
2. Money 在服务启动路径内执行 migration 会超过统一 15 秒等待。夹具改为先独立执行 migration，再以 verify-only 启动服务。
3. 高负载验收中 Bonus 的 3 秒来源超时会把健康来源误判为 503。验收环境显式使用 10 秒上限，生产默认和失败关闭语义不变。
4. 保持模式中的浏览器 Token 只有五分钟有效期。页面正确拒绝过期凭证；验证时重新由 IAM 签发短期凭证，没有把 Token 固化为测试秘密。

## 7. 边界结论

本轮支持以下判断：

- C7 已从“可调用 API”推进为操作者可理解和可恢复的人类业务工作区；
- 经营与生产可复用同一稳定生命周期，同时保持来源端口和 Capability 分离；
- 纠错是追加式取代，撤回是追加 revision，不覆盖历史；
- 页面没有成为授权者，真实写入仍由目标应用按 Credential、Scope、状态和 expected revision 决定；
- 真实依赖停机不会留下部分 Pool，恢复后可继续原业务意图；
- Project、People、Contract、Money 继续拥有各自事实，C7 只冻结精确引用和计算结果。

本轮不能支持以下判断：

- C7 已进入正式 `Nginx → Gateway → IAM → Bonus` D1 装配；
- 未选装缺席、重新选装或联合空目标恢复已通过；
- 发布奖金已经生成 Money 自有应付义务；
- C8 跨项目投影、完整双 Tenant 或真实 Ubuntu 已通过；
- OA 第一阶段已经完成。

下一收缩应在 fresh selection 后处理 C7 正式 D1／未选装／联合恢复，或把 C7 发布结果作为版本化 Payable Suggestion 交由 Money 独立接受；两者不混在同一轮实现。
