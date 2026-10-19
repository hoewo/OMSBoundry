# OMS 一期默认主入口发布验证（2026-10-17）

## 结论

OMS 一期已经从旧 OA 侧栏中的一个二次入口，提升为 `https://oatest.antuinfo.cn/` 登录后的默认产品工作面。根域名、登录默认回跳和 `/oms-v1/` 都进入同一 OMS 页面；旧项目立项、项目管理、资料、统计和 OMS 参考版集中到 `/legacy/`“旧版工作台”，并在两边保留显式返回入口。

这不是只改菜单名称。OMS 请求由 Workbench 服务端直接输出 OMS shell，不再先渲染旧 OA 侧栏，也不再装载旧 `workbench.js`、旧项目页面和无关 OA runtime 后再由客户端替换，因此关闭了截图中“侧栏先出现、内容长时间空白”的结构性等待路径。

## 修改边界

- Gateway `/`、`/oms-v1[/]` 与安全回跳默认值统一到 `/workbench/?view=oms-v1`；
- 新增 `/legacy[/]` 到 `/workbench/?view=legacy` 的受保护别名；
- Workbench 对 OMS 与旧版分别装配页面和运行时，当前 OMS 不包含旧侧栏或 `workbench.js`；
- OMS 侧栏底部提供“旧版工作台”，旧版顶部和首项提供“返回 OMS 一期”；
- 未修改 IAM、Owner、API、数据库、schema、Grant、主体、项目、合同、票据、资金或文件事实。

## 本地验证

- `go test ./backend/platform/gateway/internal/gateway ./frontend/oms-workbench/internal/server`：通过；
- `go test -count=1 ./backend/platform/gateway/... ./frontend/oms-workbench/...`：通过；
- `node --test frontend/oms-workbench/internal/server/*.test.mjs`：235/235 通过；
- 测试明确断言当前 OMS 响应没有旧侧栏和 `workbench.js`，旧版响应不启动 `oms-phase1.js`；
- 默认登录页显示 `OMS Phase 1`、`登录 OMS 一期`，并把 `return_to` 固定为 `/workbench/?view=oms-v1`。

## 发布制品

```text
Gateway:  crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/gateway:v0.1.0-test.oms-v1.14@sha256:d58db150578c9992cd703be316560730945aae35930068628469f3593f29f2a5
Workbench: crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.14@sha256:32cf219f77f7b3da3f9aebb0500fec11c4979806bc7b3dc969c0d3424fe55ff7
```

- D1 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.14_linux_amd64.tar.gz`；
- 发布包 SHA-256：`0ab3af151c5e51139d36ffcdd2055633cb31f2798be3f08a95b7437710f60265`；
- 发布前备份：`/var/backups/omsbizfoundry/20261017T033416Z`；
- 发布目录：`/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.oms-v1.14_linux_amd64`；
- 环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.14.https.env`。

## 公网验证

- 升级前 `check`、候选 `preflight-upgrade`、升级内置备份与升级后 `check` 均通过；
- 17 个长运行容器全部 healthy，Gateway 与 Workbench 运行摘要和发布清单一致；
- `/` 与 `/oms-v1/` 返回 OMS 路由，`/legacy/` 返回旧版工作台，`/oms/` 继续返回参考 Demo；
- Chrome 从根域名实际进入后，标题为“OMS 经营管理系统”，没有旧 OA 外层侧栏；OMS shell 首屏直接可见，随后状态更新为“业务来源已同步”；
- 公网复读 29 个项目，保留“微信4123”及其 `¥1,243,123.00` 未回金额，证明本轮入口升级未清空业务数据；
- 点击“旧版工作台”进入独立旧版页面，可见项目立项、项目管理、项目资料、项目统计和 OMS 参考版；点击“返回 OMS 一期”可回到当前主产品。

## 结论边界

本轮关闭的是入口层级与旧运行时混装导致的感知／实际等待，不承诺网络、Owner 读取或大数据量场景恒定无延迟。旧功能只是降级为次级入口，没有删除；本轮也没有新增或迁移任何业务事实。
