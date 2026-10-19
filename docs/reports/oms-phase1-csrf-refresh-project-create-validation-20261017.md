# OMS 一期长期开页安全令牌修复与项目创建验证（2026-10-17）

## 结论

用户在公网 OMS 新建项目时收到 Gateway 的 `csrf_failed / 安全校验失败`。失败发生在业务写入前的 `/gateway/authorize/project-workspace`，不是项目名称、甲方、金额或日期的业务校验。OMS 页面仍在读取首次渲染 HTML 中的退出登录表单令牌；页面打开超过令牌有效期或其他标签页轮换令牌后，HTML 值与当前 Cookie 不再一致，Gateway 按安全边界拒绝请求。

修复后 OMS 与参考 OA 共用 `ReferenceOAGatewayCSRF`：每次换取应用 Capability Credential 前读取当前 CSRF Cookie；Cookie 已失效时，只向 Suite Manifest 配置的同源 IAM `/api/v1/csrf` 发起无业务载荷的 GET，并核对响应令牌与 Cookie 一致。没有有效 IAM 路由、响应异常、Cookie 不一致或刷新失败时继续失败关闭，保留表单输入，不发送 Owner 命令，也不回退到初始 HTML 令牌。

## 修改范围

- OMS 根节点从 Workbench `pageData.IAMRoutePrefix` 接收版本化 IAM 路由；
- OMS 授权封装改为 `await csrfProvider.token()`，不再查询退出登录表单的隐藏字段；
- CSS／JS 查询版本递增到 `20261017.1`，避免普通刷新命中旧脚本；
- Project、Contract、Party、Billing、Money、Evidence 的 Owner、API、schema、Grant、数据库和事实边界均未改变；
- 项目创建仍按 Project → Contract → Project Link 分段执行，修复只关闭第一段之前的安全令牌失配。

## 本地验证

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`：通过；
- `node --test frontend/oms-workbench/internal/server/reference_oa_csrf.test.mjs frontend/oms-workbench/internal/server/oms_phase1_pagination.test.mjs`：12/12 通过；
- `go test ./frontend/oms-workbench/...`：通过；
- `GOMAXPROCS=2 GOWORK=off go vet ./...`（`frontend/oms-workbench`）：通过；
- Unified Workbench server 测试固定 OMS 必须接收配置 IAM 路由、必须使用统一 CSRF Provider，且不得再次读取退出登录表单中的初始令牌。

## 制品

Unified Workbench 候选镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.13@sha256:3215e21d963f4f978f036ecad2e1f36d437d6680035ca0cde8097b0b9eb82b80
```

D1 发布包 `v0.1.0-test.oms-v1.13` 已生成并通过 SHA-256 校验；包内 `.env.example` 固定上述 Workbench 摘要，其他 16 个镜像保持原摘要。

## 公网发布与验证

- 安全组恢复当前运维来源 `/32` 后，升级前 D1 `check` 通过；
- 发布前备份：`/var/backups/omsbizfoundry/20261017T030537Z`；
- 发布目录：`/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.oms-v1.13_linux_amd64`；
- 发布环境：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.13.https.env`；
- 发布包 SHA-256：`5a89f60527590962bac922ec5d6da468b56a2ceff2fdead0faba850040db98e7`；
- 升级后正式 D1 `check` 通过，17 个长运行容器全部 healthy；本轮只有 Unified Workbench 镜像内容改变，D1 编排按版本／配置标签重验并重建所需平台容器；
- 公网 `/workbench/assets/oms-phase1.js` 确认包含 `root.dataset.identityRoutePrefix` 和 `await csrfProvider.token()`，且不再查询 `form[action="/gateway/logout"]`；
- Chrome 先在项目列表搜索“微信4123”，结果为 0 条，证明此前 Gateway 前置拒绝没有留下 Project 半成品；
- 使用同名项目、甲方“OMS 一期公网验收甲方 C854FF91”、应收 `¥1,243,123.00`、开启日期 `2026-10-17` 和备注“安全校验修复回归”重新提交，页面实际经历 Project → Contract → Party revision → Project Link 五段进度并成功返回列表；
- 列表复读 1 条“微信4123”，显示上述甲方、应收／未回均为 `¥1,243,123.00`；详情复读合同草稿、累计开票／实际回款均为 `¥0.00`、未回款 `¥1,243,123.00`、Project 建立 r1 与关联业务资料 r2；
- 操作后抽查 Gateway、Workbench、Project Workspace 和 Contract Ledger 最近 15 分钟日志，没有 `csrf`、`error`、`panic` 或 `fatal` 命中。

## 结论边界

公网“安全校验失败”故障已在项目创建路径关闭。证据证明新鲜 CSRF、Gateway 授权和既有三段 Owner 编排能够连续完成一次真实项目创建；不据此宣称全部 OMS 写动作、撤权、并发、恢复或长期开页矩阵已全量覆盖。本轮没有修改 Project、Contract、Party、Billing、Money、Evidence 的 API、数据库、schema、Grant 或事实所有权。
