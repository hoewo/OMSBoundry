# 原 OA 初步设计阶段原生验证（2026-10-06）

## 结论

原 OA 初步设计面板和 800px 编辑抽屉已经接到真实独立 Production／Evidence／正式 IAM，并在私有 Nginx → Gateway 入口、远端临时 PostgreSQL 18 和五所有者逻辑空目标恢复中通过 9 个浏览器场景。既有 `reference-oa-stage-set-v1@2` 没有被改写；第一次初设保存对同一 Work Item 执行一次显式 2→3 升级，随后登记两份主文件、一份校审单和 `0` 分。

本结论是私有原生候选，不是 Main／公网发布，不是正式 D1 全包恢复，也不是完整 Production 或完整 OA 通过。

## 实际运行

```text
OA_STAGE_RESTORE=1 node acceptance/oa-initiation-repair/verify-preliminary-stage-native-browser.mjs /tmp/oa-preliminary-suite.XQWiZ2
```

执行时间：2026-10-06 10:40:46～10:44:53（Asia/Shanghai）。本机 Docker 未启动，线上 Main 未修改；夹具结束后自有进程和临时运行资源已停止。原始回执保留在 `/tmp/oa-preliminary-suite.XQWiZ2/preliminary-stage-browser.json`。

同时实际通过：

- `cd backend/services/design-operations && GOMAXPROCS=2 GOWORK=off go test -p 1 ./...`
- `cd frontend/oms-workbench && npm test`：169／169
- `cd frontend/oms-workbench && npm run check`
- `cd frontend/oms-workbench && npm run build`
- `make architecture-boundary-static-audit`：15 应用，跨应用 import 0、replace 0、跨库 SQL 机制 0

带 `integration` tag 的整套 Go 集成测试没有在本批另跑；实际 migration、命令、HTTP、浏览器和恢复路径由上述真实 PostgreSQL 18 夹具执行，不能把未单独运行的测试集写成通过。

## 九个场景

1. 既有 v2 Work Item 在任何初设操作前保持仅方案定义。
2. 原初设面板把 v2 显示为已知但尚未纳入，读取零写入。
3. 主文件必填和取消均为零写入。
4. 同一 Work Item 恰好一次 2→3 升级，保存两主文件、一校审单和 `0` 分。
5. 刷新、原组件预览和下载读取真实 Evidence 字节，Production 零写入。
6. 追加校审、改为 `88.5` 分并精确移出一份主文件；历史保留，Evidence 字节不删除。
7. IAM、People、Project、Evidence、Production 五所有者逻辑空目标恢复后，v3 升级、引用、字节和原 UI 保持。
8. 撤销 register／withdraw 后仍可读，按钮收敛，伪造写入 403 且零事实变化。
9. Project／People 摘要保持，升级行和升级历史各一条，页面错误为零。

终态计数：1 个 Work Item、2 个 Stage、1 次升级、4 个活动资料、2 条评分、1 条 ending、5 个命令回执、1 条升级历史。五库恢复前后 canonical SHA-256 一致，四份本地 Evidence 字节树逐项一致，签名密钥身份保持。

## 边界解释

新增事实全部在 Production 自有库；Evidence 仍拥有字节，Project／People 没有被同步写入，Workbench 没有业务表。没有新增跨系统 Audience、Capability 或 Workload Policy，只复用已有 Production → Evidence 精确校验和正式 IAM 能力。

这证明“阶段资料集合”子边界可以在第二个同类阶段中通过兼容版本演进复用；它没有证明施工图、变更竣工、试验、报告也适合相同语义，更没有证明 Production 整体或跨客户边界稳定。下一项以施工图真实操作差异作为证伪输入。

## 未关闭门槛

- Main／公网匹配发布和管理员 Catalog 重新对齐；
- 正式 D1 archive、keys、旧会话及整包空目标恢复；
- 全角色、双 Tenant、撤权残余窗口和来源中断完整矩阵；
- Project 多文件资料消费者的版本化兼容；
- 施工图、变更竣工、试验、报告及完整生产进度；
- 第二客户不 fork Base 的升级证据。
