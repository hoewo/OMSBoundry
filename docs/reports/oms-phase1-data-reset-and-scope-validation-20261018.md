# OMS 一期数据清理与范围验证记录（2026-10-18）

## 目标与边界

用户要求只清除 OMS1 数据，以便从空白状态重新录入并走查一期流程。账号、登录、IAM、Grant、资质单位、旧 OA 数据、其他应用事实和部署配置必须保留。

OMS1 没有独立数据库，它由 Party、Project、Contract、Billing、Money、Evidence 六个既有 Owner 装配。清理前审计确认首笔 OMS 数据之后，这六个 Owner 的新增业务事实均来自 OMS 走查或其 Party runtime 映射，因此本次采用首笔 OMS 数据之前的完整检查点恢复；没有给 Workbench 增加跨库删除能力。

## 恢复点与可回滚性

- 清理前完整备份：`/var/backups/omsbizfoundry/20261018T041742Z`
- 恢复源检查点：`/var/backups/omsbizfoundry/20261012T175912Z`
- 恢复前已验证检查点校验和。
- 恢复范围：Contract Ledger、Money Flow Ledger、Party Directory、Project Workspace、Evidence Service、Billing Invoice。
- 不在恢复范围：IAM、账号、Grant、Gateway、其他业务应用及其数据库。
- Evidence 与 Contract 自有 Evidence 的命名卷按精确卷名清理后恢复；Party runtime 映射随后通过正式准备脚本重建。

若需要找回本次清理前状态，可从 `20261018T041742Z` 按同一正式恢复路径回滚。

## OMS 范围修正

Workbench 过去读取 Owner 全量集合，空白 OMS 仍可能显示旧 OA 项目。现改为：

- 项目必须在 `project_type_keys` 中包含 `operations` 才进入 OMS。
- Billing 和 Money 记录必须能通过稳定项目引用解析到上述 OMS 项目才进入 OMS。
- 过滤只发生在 Workbench 投影层；Owner 中的旧 OA 事实不删除、不复制、不改写。

静态资源版本更新为 `20261018.1`，发布镜像为 `v0.1.0-test.oms-v1.17@sha256:da5f4626cb53a9d8ca6758f2c6852468974221fc4ca440264de4a62a42d5dbfe`。

## 清理结果

恢复后的 Owner 事实计数：

| Owner | 保留事实 | OMS1 结果 |
| --- | ---: | --- |
| Project Workspace | 26 个旧 OA 项目 | `operations` 项目 0 个 |
| Contract Ledger | 3 份旧合同 | OMS 合同 0 份 |
| Billing Invoice | 0 条 | OMS 开／收票 0 条 |
| Money Flow | 3 条旧资金事实 | OMS 回款／支付 0 条 |
| Party Directory | 6 个主体 | 资质单位与必要 runtime 主体保留 |
| Evidence Service | 19 个文件、20 个版本 | OMS 新增附件已移除 |

账号、权限、登录入口和资质单位未清除。用户可以直接从 OMS“项目列表 → 新建项目”重新开始。

## 实际验证

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`：通过。
- Workbench Node 测试：235/235 通过。
- `go test -count=1 ./frontend/oms-workbench/...`：通过。
- D1 全栈检查：通过。
- 长运行容器：17/17 healthy。
- 公网正式登录、换票和 Owner 只读：通过；业务写入为 0。
- 公网静态资源确认包含 `isOMSProject` 且版本为 `20261018.1`。

浏览器标签页能够打开公开 OMS 页面，但本轮自动化 DOM 快照两次超时，因此没有把“视觉确认列表为空”列为通过证据。数据库范围、Workbench 过滤、静态资产、公网授权读取和运行健康证据已完成。

## 不代表的事项

- 不代表产品新增“清空全部数据”功能。
- 不允许 Workbench 跨 Owner 删除事实。
- 不改变历史更正、冲销、revision 或 Owner 数据所有权规则。
- 不清除旧 OA，也不证明旧 OA 与 OMS 已形成统一业务产品。
