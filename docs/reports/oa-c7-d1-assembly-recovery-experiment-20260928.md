# OA C7 正式 D1 装配、未选缺席与联合恢复实验

日期：2026-09-28  
结论：`formal-d1-assembly-and-recovery-supported / payable-suggestion-and-single-type-minimum-pending`

## 问题与结果

**Bonus Distribution 已有独立核心、正式 IAM、真实来源和人类工作区，但尚未证明能作为可选应用进入正式 D1 并随组合备份恢复 → 把它接入独立镜像、数据库、migration、工作负载身份、Suite Manifest、Workbench、Nginx／Gateway／IAM 与联合备份恢复，并在隔离 D1 中验证正式入口、未选缺席、空目标恢复和恢复后继续写入。**

## 验证组合

本轮使用独立 Compose project `omsd1c7` 和回环端口 `18180`。选装 Identity & Access、Unified Workbench、Gateway、Nginx、People & Groups、Project Workspace、Contract Ledger、Money Flow Ledger 与 Bonus Distribution；Party、Evidence、Business Engagement、Billing、Production、Approval & Audit Results、Cost & Outsourcing 均关闭。

Bonus Distribution 使用独立 Linux/amd64 非 root 镜像、`bonus_distribution` 数据库、migration、工作负载私钥卷、`bonus-distribution` audience 和 Capability Manifest。经营奖池经 Contract Settlement，生产奖池经 Money Receipt Set；Person 与 Assignment 分别经 People 和 Project 的精确版本化端口读取。正式运行期没有共享业务表、跨库 ORM、静态来源 Token 或跨库事务。

## 执行证据

以下证据均由实际运行产生：

1. 独立 module 的 `GOWORK=off go test ./...`、`go vet ./...`，以及 D1 image-mode、promotion、ingress、upgrade-selection、release-bundle 静态门禁通过；
2. `Dockerfile.bonus-distribution`、Compose、数据库初始化、migration、工作负载密钥、Capability Catalog、Suite Manifest、Workbench、Gateway 路由、备份与恢复脚本均纳入正式 D1；
3. 经 `Nginx → Gateway → IAM → Bonus Distribution` 执行 16 条 HTTP 断言，期望值与实际值偏差为 0；
4. 正式工作负载换票留下 5 条精确审计：Project 1 条、People Person 2 条、Contract Settlement 1 条、Money Receipt Set 1 条；
5. 经营与生产各建立并发布一个 Pool，共 2 个 Pool、2 个 Allocation；两类来源、Person／Assignment、策略 revision／digest 和发布历史均冻结；
6. 联合备份 `20260928T143704Z` 包含 `bonus-distribution.dump`、应用选择与外层 SHA-256；
7. 销毁隔离目标卷后从空目标恢复成功，Pool／Allocation ID、revision、来源／策略／人员快照和历史保持；
8. 恢复前后经营 Pool、生产 Pool、经营历史和生产历史的 canonical SHA-256 分别保持：
   - `f3145c0f6df0dea8437baa0f0c81f49d65bdbc2bede79298b9442c291e34b50b`；
   - `a01d04199b61b46f12c55d391efbef48890b370a7af684652ade03b469e5abcd`；
   - `33506aa0d75231c416e1ddec5f2dd25b90d3cafc4c1d37020a27c602906c029e`；
   - `7a0d287222a2e7f9f059827638413e2fa1aac513e858e2711d6afeb40625fa66`；
9. 恢复后生产 Pool 从 `published revision 2` 追加为 `withdrawn revision 3`，没有覆盖既有事实；
10. 取消选装后 Suite Manifest 为 `enabled=false`，Workbench 无入口，Gateway 返回 404，runtime、migration、工作负载密钥容器和备份 dump 均缺席；重新选装后原数据仍在。

一次完整包装命令在业务断言已经落盘后，先后暴露了报告字符串变量边界和恢复校验事件名两项验收脚本缺陷。两项均已修正，并在同一隔离环境中用幂等恢复和一轮全新的未选／重新选装检查完成余下阶段；没有发现业务数据缺陷。本报告据此声明各阶段证据通过，但不虚构“一次从头到尾的包装命令返回码为 0”。

## 当前能声明什么

- Bonus Distribution 已是正式 D1 可选应用，拥有独立镜像、数据库、migration、工作负载身份、Capability、路由、菜单和备份边界；
- C7-A16 已通过：未选时入口、进程、migration、工作负载密钥和备份项缺席，重新选装不删除数据；
- C7-A17 在 Pool／Allocation、来源／策略／人员快照、revision、摘要和历史范围内通过；
- 联合恢复已经使用真实空目标演练，并完成恢复后继续写入；
- 正式入口仍执行目标应用最终授权和请求绑定的精确来源读取。

## 仍不能声明什么

- C7→Money Payable Suggestion 尚未实现，因此 C7-A13、C7-A14 以及 C7-A17 的建议引用部分没有通过；
- 本轮同时选择 Contract 与 Money，尚未分别证明“仅经营来源”和“仅生产来源”的最小组合，C7-A15 仍待验；
- C8 只读投影、完整双 Tenant、账号停用、真实 Ubuntu／HTTPS 和阶段最终报告仍未通过；
- 本轮不能外推为 OA 完整体已经完成。

下一业务收缩应在新鲜选择后从 C7→Money Payable Suggestion 或 C8 中选一项；单类型最小组合、双 Tenant 与真实环境继续作为独立补证轴。
