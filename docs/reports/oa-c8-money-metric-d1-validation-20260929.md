# OA C8 Money 指标正式 D1 权限与只读边界验证

日期：2026-09-29  
结论：`c8-money-metric-d1-supported`  
整体阶段结论：仍为 `insufficient-evidence`

## 1. A→B 与范围

问题 A：Money Flow 的经营汇总虽然已有类型化 `MetricProjection v1` 和 Workbench 消费端单元证据，但尚未证明它能在公司 Ubuntu D1 中真实经过 Nginx、Gateway、IAM 与 Money；也未证明统计可见与金额可见能够分权、撤权后重新签发能够收敛，以及读取不会改写 Money 自有事实。

可验证结果 B：在固定摘要的公司 D1 候选中，使用正式入口和正式 IAM 临时账号完成“无统计能力拒绝 → 有统计能力但金额脱敏 → 增加金额能力后精确可见 → 撤销金额能力后重新脱敏”，并证明整个权限读取序列前后 Money 事实集合字节摘要不变。

本轮不负责：

- Workbench 真人浏览器的视觉与交互验收；
- Production 当前 Subject 工作项和 Business Engagement 间接资料的正式 D1；
- 双 Tenant、跨租户拒绝、持续负载、HTTPS 公网入口或公司 D1 空目标恢复；
- 第二指标来源和跨来源公式。Workbench 仍不得跨来源直接合计。

## 2. 环境与制品

| 项目 | 实际值 |
| --- | --- |
| 目标 | 公司阿里云 Ubuntu D1，SSH 隧道内的 Nginx 入口 `http://127.0.0.1:18000` |
| Release | `v0.1.0-test.11` development release |
| Money 镜像 | `money-flow-ledger:v0.1.0-test.11@sha256:9567ada8efd030b927bb5e38cc99156ad7a1e56796880cefb5d77b01289fe6e1` |
| 其余兼容闭包 | IAM `test.8`、Evidence `test.7`、Project `test.6`、Gateway／Workbench `test.10` |
| 运行状态 | 12/12 长期容器 healthy；unhealthy 0；`/health/ready` 200 |
| 升级前备份 | `20260929T133853Z`，checksum 有效 |

`test.11` 只提升 Money 镜像，使正式 D1 包含 `/api/v1/metrics/operating-summary`。发布前以 `GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...` 完成 Money 全量测试；Linux/AMD64 镜像单独串行构建和推送，没有启动本机业务容器。完整 D1 升级执行兼容预检、升级前联合备份、全部 migration 和同一候选重建，最终 12/12 healthy。

## 3. 验证方法

可重复入口为：

```sh
D1_ENV_FILE=/path/to/d1.env \
D1_PROJECT_NAME=omsbizfoundry \
D1_PUBLIC_URL=http://127.0.0.1:18000 \
D1_C8_METRIC_CONFIRM=yes \
C8_METRIC_EVIDENCE_DIR=/path/to/evidence \
./acceptance/d1-c8-money-metric/verify-running
```

脚本只在显式 `D1_C8_METRIC_CONFIRM=yes` 时执行 IAM 临时账号和 Grant 变更。发现新服务器的指标集合为空后，它没有直接写数据库，而是通过 Money 正式页面接口建立两条测试 Party 和一条 `1234.56 CNY` 收款事实。随后才固化业务事实摘要并开始权限读取序列。

测试夹具：

| 对象 | ID |
| --- | --- |
| Ledger Owner Party | `d96a5a7e-77a7-4b12-b513-edd58d01212f` |
| Counterparty | `9692df64-4fa1-4ae6-9f29-fbce7803d6b3` |
| Cash Transaction | `72d4a454-e6ce-4082-a337-01ea3d2da862` |
| 临时 IAM Subject | `feb83344-ccc3-4e9a-afa6-136b74f6530f` |

所有夹具均带 `C8-METRIC` 标识。Money 事实保留为测试环境中的不可变正式事实；临时 IAM Subject 在结束时撤销全部 Money Grant 并停用。

## 4. 直接结果

| 步骤 | 预期 | 实际 |
| --- | --- | --- |
| 管理员读取空库指标 | 端点存在但允许空集合 | HTTP 200，`items=[]` |
| 通过 Money 正式接口建立夹具 | 不直接写数据库 | 两个 Party、一个 Cash Transaction 均返回 303 |
| 管理员重新读取 | 有精确类型化指标 | HTTP 200，6 项 |
| 临时账号仅有 `view` | 指标读取拒绝 | HTTP 403 |
| 增加 `statistics.read` 并重新签发 | 结构可见、金额与摘要脱敏 | HTTP 200；6/6 `redacted=true`，`value=null`，`source_digest=null` |
| 增加 `amount.read`，不重新签发 | 旧凭证权限不扩张 | HTTP 200；仍为 6/6 脱敏 |
| 重新签发 Money 凭证 | 精确金额可见 | HTTP 200；6/6 `redacted=false`，值为十进制定点字符串，摘要为 `sha256:` |
| 撤销 `amount.read`，不重新签发 | 已签凭证保留最长 300 秒有界窗口 | HTTP 200；仍可见 |
| 撤权后重新签发 | 权限收敛 | HTTP 200；恢复 6/6 脱敏 |
| 清理临时 IAM Subject | 无遗留有效授权 | Subject `disabled`；活动 Grant 数 `0` |

所有指标 HTTP 请求在服务器本机 Nginx 入口测得 `4.250～20.572 ms`。这只证明当前单用户、单来源、回环入口的低负载响应，不构成公网体验或容量结论。

## 5. 数据所有权与只读不变量

脚本在权限读取序列前后，对 Money `public` schema 中除应用元数据、本地会话、用户和审计外的全部基础表按表名和完整 JSON 行排序，再生成 SHA-256：

```text
before = cc60c4c734f4248a64d54f3e96111fc65bce1a76afef0af68ef9f075d09ff8b7
after  = cc60c4c734f4248a64d54f3e96111fc65bce1a76afef0af68ef9f075d09ff8b7
```

逐表行数也完全相同。由此本轮直接支持：

- 指标由 Money 自有事实实时派生，Workbench／Gateway／IAM 不复制 Money 事实；
- 读取、授权、撤权和重新签发没有写入 Money 业务表；
- `statistics.read` 与 `amount.read` 是两个不同能力，不能用页面隐藏替代服务端脱敏；
- Grant 变化不会魔法修改已经签发的短凭证，收敛发生在刷新／重新签发，残留窗口仍受 300 秒 TTL 约束。

## 6. 失败与修正

第一次验收在管理员基线读取后主动失败，因为返回的指标集合为空。该失败没有临时 IAM Subject，也没有业务写入。它证明“端点 200”不能单独作为金额分权证据。第二次运行增加了“仅在空集合时通过正式 Money API 创建显式测试夹具”的步骤，才完成非空投影验证；没有用 SQL 伪造来源事实。

## 7. 结论与剩余门禁

Money 指标这一条 C8 来源已从“单元／契约可行”提升为 `c8-money-metric-d1-supported`：固定摘要已在公司 Ubuntu D1 经 Nginx→Gateway→IAM→Money 完成非空、拒绝、脱敏、授权扩张、撤权残留窗口、刷新收敛和零源写入验证。

这不把 C8 或 OA 整体提升为通过。仍需按低资源、单批次顺序完成：

1. 真人浏览器中的 Project 资料下载、Evidence 独立授权和视觉反馈；
2. Production 当前 Subject 工作项的正式 Gateway／IAM／D1 和 Workbench 浏览器链；
3. Business Engagement 间接资料的正式 D1；
4. 公司 D1 已校验备份的隔离空目标恢复；
5. 完整 C1～C8 同一 Suite／Overlay／数据集连续旅程与双 Tenant。

