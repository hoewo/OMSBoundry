# OMS 一期首屏关键路径与主体角色修复发布报告（2026-10-22）

## 结论

用户实际刷新确认 BF-173 的优化体感不明显是有效反馈。BF-173 已经减少了 IAM 授权轮次，但主页面仍同步加载附件功能专用的旧 React 运行时，导致首屏继续被一个大资源阻塞；同时，甲方管理和施工方管理都从同一份外部 Party 候选集合生成，造成施工方页显示甲方数据。

BF-174 已修复并发布到 `https://oatest.antuinfo.cn/`：

- OMS 首屏只同步加载 `oms-phase1.css`、分页脚本和 OMS 主脚本；
- 2,512,394 bytes 的 `reference-oa-react-bidding.js` 只在首次附件上传或预览动作时加载；
- 带版本号的静态资源返回 `Cache-Control: public, max-age=31536000, immutable`；
- 甲方列表由 OMS 项目的 `client_party.party_id` 投影；
- 施工方列表由活动的 `contractor-party` Project 资源关系投影；
- 未关联的 Party 不出现在角色列表，但仍可作为后续新建项目或关联施工方的候选；
- 不向 Party Directory 写入永久甲方／施工方类型。

## 问题 A → 可验证结果 B

1. 每次刷新仍提前下载附件运行时 → 首屏 HTML 不再引用该运行时，只有附件动作触发动态加载。
2. 版本资源重复下载 → OMS `.5` 静态资源返回长期 immutable 缓存头。
3. 施工方页误显示甲方集合 → 公网真实 Project 关系计算为甲方 `[甲方1]`、施工方 `[]`，两页不再共享同一候选全集。

## 实现范围

源码提交：`105b7c1 fix(oms): trim first paint and separate party roles`

主要变更：

- `frontend/oms-workbench/internal/server/server.go`
  - OMS 主入口移除旧 OA CSS、附件 React 运行时和 initiation runtime 的同步引用；
  - 版本化静态资源增加 immutable 缓存。
- `frontend/oms-workbench/internal/server/oms_phase1.js`
  - 增加最小 Gateway CSRF 客户端；
  - Evidence 附件能力按需加载旧 React 运行时；
  - `partyListData` 按 Project 事实关系区分甲方与施工方；
  - 未形成对应关系时显示角色空状态和后续操作说明。
- `frontend/oms-workbench/internal/server/server_test.go`
  - 断言首屏不再同步引用旧运行时；
  - 断言版本资源缓存策略、动态附件运行时和角色过滤标记。
- `acceptance/oms-phase1-public/verify-public-oms-v1.py`
  - 断言关键资源版本及 immutable 缓存；
  - 输出关键资源体积和当前真实角色投影。

本次不改变 Owner、API、schema、Capability、Grant、数据库、Party／Project 数据所有权或部署拓扑，也不新增浏览器业务数据缓存。

## 本地验证

以下检查实际运行并通过：

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`
- `node --test frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`：4/4
- `GOMAXPROCS=2 go test -p 1 ./frontend/oms-workbench/...`
- `python3 -m unittest acceptance/oms-phase1-public/test_verify_public_oms_v1.py`：2/2
- `python3 -m py_compile acceptance/oms-phase1-public/verify-public-oms-v1.py`
- `git diff --check`

## 发布

- Workbench：`v0.1.0-test.oms-v1.25`
- 镜像摘要：`sha256:d9df134e7bcadf70418058bba5d84f4646ae96ae28a509bbb9230883aba7e3f5`
- 静态资源版本：`20261022.5`
- 升级前备份：`/var/backups/omsbizfoundry/20261022T065207Z`

候选部署预检通过，D1 全栈 `check` 通过；发布后 17 个长运行容器均为 healthy。

## 公网验收

正式登录、公网页面标记、五个 Gateway 授权、Owner 只读和 Party Directory authority 检查通过，业务写入为 0：

```json
{
  "passed": true,
  "owner_reads": {
    "billing-invoice": 0,
    "contract-ledger": 6,
    "money-flow-ledger": 0,
    "party-directory": 4,
    "project-workspace": 1
  },
  "critical_assets_bytes": {
    "oms-phase1.css?v=20261022.5": 66674,
    "oms-phase1-pagination.js?v=20261022.5": 2874,
    "oms-phase1.js?v=20261022.5": 164481
  },
  "role_projection": {
    "clients": ["甲方1"],
    "contractors": []
  },
  "business_writes": 0
}
```

独立公网资源读取结果：

| 资源 | 体积 | 单次读取时间 | 首屏同步 |
|---|---:|---:|---|
| OMS CSS | 66,674 bytes | 0.293 s | 是 |
| 分页脚本 | 2,874 bytes | 0.146 s | 是 |
| OMS 主脚本 | 164,481 bytes | 0.305 s | 是 |
| 附件 React 运行时 | 2,512,394 bytes | 5.147 s | 否，首次附件动作加载 |

首屏版本化资源合计 234,029 bytes。上表是同一时段的命令行公网单次测量，用于确认大资源已经移出关键路径；不把它冒充为用户浏览器中包含 HTML、Owner 数据读取和渲染的完整首屏 wall-clock。

## 后续观察

用户需要在版本切换后至少强制刷新一次取得 `.5` HTML；之后相同版本资源会由 immutable 缓存复用。如果真实浏览器体感仍慢，下一步采集该浏览器网络瀑布，继续区分 HTML／静态资源、Project／Party 首阶段和后台 Owner 汇总耗时，不以合并 Owner 或绕过 IAM 换取表面速度。
