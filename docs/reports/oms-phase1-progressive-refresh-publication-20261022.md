# OMS 一期渐进刷新优化公网发布验证（2026-10-22）

## 结论

OMS 一期多 Owner 装配必须继续读取 Project、Party、Billing、Money、Contract 与 Evidence，但页面刷新不需要无条件先为每个 Owner 重新走一轮授权。BF-173 已把暖刷新改为“既有 Capability 先读、明确失效再授权一次”，并让项目与主体先形成可用页面，票据、资金、合同和经营汇总随后补齐。

源码提交 `00c5d6b` 已构建并发布为：

- Workbench：`v0.1.0-test.oms-v1.24`
- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.24`
- 发布摘要：`sha256:015ab2b216b47f40d38ac253f89ce8e27afd3fba0c6d6a3a2098cf0c8b3c1054`
- 静态资源版本：`20261022.4`

## 原因

此前每次页面刷新都会重置 JavaScript 内存中的授权标记。虽然浏览器中的 HttpOnly Capability Cookie 仍可能有效，Workbench 仍会在每次 Owner 读取前无条件调用 Gateway／IAM 授权；Money 等同一 Owner 的并发读取也可能重复发起授权。六类来源全部完成后页面才统一渲染，最慢的 Owner 和逐项目经营汇总会阻塞首个可用画面，“正在连接业务系统”又把这一过程表现成整页不可用。

这不是多小系统架构的必要代价。必要的是每个 Owner 继续独立授权和最终校验，不必要的是在凭证仍有效时重复换票，以及等待所有来源完成后才展示任何数据。

## 实现语义

1. Owner 只读请求先携带浏览器已有的 HttpOnly Capability Cookie 直接读取。
2. 只有响应为 `opaqueredirect`、状态 0、3xx 或 `401 credential_required` 时，才对该 Audience 发起授权，随后只重放原读取一次。
3. 同一 Audience 同时需要授权时，共享一项在途任务，避免同页重复换票。
4. 创建、编辑、上传等写请求仍在发送前强制取得新授权，没有降低写入安全边界。
5. 六类来源仍同时启动；Project 与 Party 完成后立即渲染项目和合作主体，状态切换为“项目与主体已就绪 · 正在汇总票款”。Billing、Money、Contract 与逐项目经营摘要完成后再补齐财务页面和总览。
6. 用户连续点击刷新时共享同一加载任务，按钮在任务结束前禁用。
7. 页面未增加 `localStorage`、`sessionStorage` 或 IndexedDB 业务数据缓存；刷新后仍从 Owner 读取最新事实。

## 本地验证

以下检查实际运行并通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`
- `node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`：4/4
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`
- `python3 -m unittest acceptance/oms-phase1-public/test_verify_public_oms_v1.py`：2/2
- `git diff --check`

服务端静态回归同时断言：读请求不再包含旧的无条件 `await authorize(app)` 路径，写请求仍强制授权，按 Audience 的授权任务去重存在，Project／Party 首阶段存在，重复刷新合并存在。`127.0.0.1:18183` 已确认加载 `20261022.4` 资源。

本地预览没有真实 Owner，不能用于计算线上前后耗时，因此没有把本地等待时间作为性能结论。

## 公网发布与恢复点

- 发布前 D1 全栈检查通过。
- 完整备份：`/var/backups/omsbizfoundry/20261022T062428Z`
- 候选 `preflight-upgrade` 通过，IAM contract 检查为 `contracts-valid`。
- 仅替换 `unified-workbench`，其他 16 个长期容器、数据库和业务数据未改。
- 发布后 D1 全栈检查通过，`running=17`、`unhealthy=0`。

公网正式登录和五 Owner 只读验收通过，结果为：

```json
{
  "business_writes": 0,
  "gateway_authorizations": [
    "billing-invoice",
    "contract-ledger",
    "money-flow-ledger",
    "party-directory",
    "project-workspace"
  ],
  "native_login": true,
  "owner_reads": {
    "billing-invoice": 0,
    "contract-ledger": 6,
    "money-flow-ledger": 0,
    "party-directory": 4,
    "project-workspace": 1
  },
  "passed": true,
  "url": "https://oatest.antuinfo.cn/oms-v1/"
}
```

## 边界与后续观察

本次证据能够确认无条件预授权轮次已从读路径移除、授权并发已收敛、项目／主体可以先于完整财务聚合进入可用阶段，且公网安全链和 Owner 读取未回归。

尚未在同一已登录浏览器上记录改动前后的 wall-clock、网络瀑布图和不同数据规模下的百分位耗时，因此不声明具体减少了多少毫秒或百分比。用户的实际刷新体验是下一项验收；若仍慢，应依据浏览器网络瀑布继续定位具体 Owner 或逐项目摘要，而不是重新合并 Owner 数据边界。
