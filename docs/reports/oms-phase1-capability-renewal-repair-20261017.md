# OMS 一期短期访问凭证续期修复验证

日期：2026-10-17  
决策：BF-160

## 问题与结论

公网 OMS 的项目列表在页面打开一段时间后显示“0 条／暂无匹配项目”，同时经营总览提示多个来源不可用。业务数据没有丢失；正式只读复验仍能从 Project Workspace 读取 30 个项目。

根因是 Gateway 的路由级 Capability Credential 已过期。浏览器使用 `fetch(..., redirect: 'manual')` 时没有把入口重定向暴露为普通 3xx，而是返回 `Response.type === 'opaqueredirect'`、`status === 0`。旧 Workbench 只检查明确 3xx，随后把该响应当作普通读取结果处理，导致各来源进入失败／空投影，页面看起来像业务数据消失。

## 修复范围

- 业务 Owner 与 Evidence 读取统一识别明确 3xx、`opaqueredirect` 和人工重定向产生的 `status=0`；
- 只删除目标 Audience 的页面授权缓存，重新经 Gateway 换取短期 Capability，并重放原读取一次；
- 第二次仍无有效访问时停止并显示明确错误；
- 普通 401／403、Owner 校验错误、冲突和写入命令不进入该自动重放；
- OMS CSS／JS 资源版本递增到 `20261017.3`，避免既有标签页继续命中一年缓存的旧脚本。

本轮只修改 Unified Workbench，不改变 IAM、Gateway、任何业务 Owner、API、数据库、schema、Grant 或业务事实。

## 实际验证

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`：通过；
- `node --test frontend/oms-workbench/internal/server/*.test.mjs`：235/235 通过；
- `go test -count=1 ./frontend/oms-workbench/internal/server ./backend/platform/gateway/internal/gateway`：通过；
- D1 发布包构建、包内检查、升级前后 `check`：通过；
- 公网静态资源 `https://oatest.antuinfo.cn/workbench/assets/oms-phase1.js?v=20261017.3`：HTTP 200，包含 `credentialRedirect`、`opaqueredirect` 和有界重授权逻辑；
- 正式登录后经 Gateway 授权执行只读验收：Party 13、Project 30、Billing 3、Money 5、Contract workspace 6，业务写入 0；
- 公网 17 个长运行容器全部 healthy，Workbench 日志无 panic、fatal 或 5xx。

发布信息：

- Unified Workbench：`v0.1.0-test.oms-v1.16@sha256:9a082f08dba5897e5cdba3698e855a6f9e842d772484a63485c68c7f64c567e5`
- 发布包 SHA-256：`ec940c632856ef6564b55e93a2ba6aeb6243212bd23268c058a2f446a078b39d`
- 升级前备份：`/var/backups/omsbizfoundry/20261017T051948Z`

## 未外推的结论

当前用户 Chrome 中原 OMS 标签页在最终验收时已经关闭，因此本报告不把该浏览器中的视觉刷新虚报为已执行。正式 API 与运行态证据已经证明截图中的 0 条不是数据丢失；用户重新进入或刷新 OMS 后，新静态资源会执行修复后的续期路径。
