# OA 登录安全令牌故障修复与验证

日期：2026-10-03。Auth58 登录故障已修复上线，实际公网 IAM 登录及原生项目目录加载通过。本批只修登录，不代表完整 OA 验收。

问题 A → 可验证结果 B：长期打开或多标签更新令牌后，表单仍提交旧值并跳到 JSON 错误页 → 提交前使用当前令牌，缺失时经已配置 IAM 更新，失败留在原页并保留输入。

## 故障和修复范围

在 Main 隔离 Chrome 中打开登录页，再通过同一上下文更新令牌，原隐藏字段与当前 Cookie 不同；旧值提交得到 403 `csrf_failed`，未发送账号密码。[复现记录](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/reproduction.json)。用户截图证明校验被拒绝，但未检查其原浏览器 Cookie，不能把具体触发原因说成已经取证。

直接复用已有 `ReferenceOAGatewayCSRF`，IAM 路径来自启用的 Suite Manifest。当前 Cookie 优先，缺失时作有界 GET 并验证响应与 Cookie 一致；登录 POST 不自动重放，busy 门禁阻止重复点击。原登录提示样式展示错误，保留输入，不打印或持久化密码，只接受同源成功重定向。身份页面与未版本化登录脚本增加 `no-store`。

没有关闭 CSRF、延长期限、重设密码、改 Grant 或创建账号；IAM、Gateway、Suite、业务边界和部署档不变。旧令牌仍被服务器拒绝。未把所有相邻表单的令牌问题归入本批完成范围。

## 实际验证

[本地回执](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/local-checks.json)：Workbench／Gateway Go 全量、Workbench vet 通过；Workbench 与 bidding adapter Node 181 项零失败／skip，其中登录与 CSRF 专项 17 项；原生 Workbench 独立启动两次，健康、配置注入与嵌入资源通过。独立启动没有真实 IAM，不能算公网登录证据。

[公网五项检查](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/public-c/browser.json)使用真实管理员账号、Main IAM 会话和登录按钮：

- 正常登录仅一次 POST。
- 清除隔离上下文的 CSRF Cookie 模拟过期，保留表单；一次令牌 GET 后一次登录 POST 成功。
- 另一个真实标签页更新令牌，旧隐藏值仍在；使用当前 Cookie 登录成功，无额外更新。旧值直接提交仍返回 403。
- 注入更新 503 时原页提示并保留输入，零登录 POST；取消注入后手动重试成功。
- 注入提交时 403 后原页提示，未自动重放；取消注入后手动重试成功。

503／竞态 403 是故障注入，不是自然事故。为控制公网限流，该五项阻止了无关的登录后业务脚本与授权读取，不能称其验证整个工作台。

另一次[完整脚本冒烟](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/public-d-full/browser.json)不阻断业务脚本／授权，真实登录、原生 IAM Session 复验、可见 Project Directory 加载通过。两批 pageerror 和业务写请求均为零；前后十二个非 IAM 库全部自有表／序列、IAM Grant 摘要和容器保持，17/17 healthy。[事实门禁](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/cloud/browser-after.json)。登录会话／审计／nonce 可以更新，不冒充整个 IAM 库零写。

首轮 public-b 在阻断水合脚本后等待模板内部节点而超时，失败保留。随后只修验证器为已认证静态标记与原生 Session 检查，再运行 public-c；没有因此重编产品或降低登录成功条件。

## 发布和恢复

开发测试版本 `v0.1.0-oa.20261003.2`，源归档 SHA `08fb690dafbdd6fa10f905e13748235263d3d8fd7b1ad4c938df9b13e789c515`。Workbench digest `sha256:76e3b9164bf3bf0511631d333b98c8ab900615cfa468b8292494ebd657d38fcb`，镜像抽取二进制与本地 AMD64 构建 SHA 相同。不是 clean-HEAD 正式发布。[制品与发布](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/cloud/publication.json)。[公网资源摘要](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/public-assets.json)与源码相等。

先联合冷备份 13 库、Evidence 字节和 IAM keys，短暂停服务后恢复；13 个新空目标恢复，facts、sequence、文件与密钥字节相同。不运行 migration，Project 000010／BE 000002 保持；这不是完整 Suite 还原运行。[恢复回执](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/cloud/restore.json)。离机私有备份复制、0600／ignored 和 SHA 核对完成后才 dispatch apply。[离机门禁](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/offhost-confirmed.json)。

Main 只重建 Workbench，其他 16 容器 ID、业务事实与入口策略保持，17/17 healthy、D1 check 通过，无 schema 变化。[部署回执](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/cloud/rollout.json)。本次旧 Workbench 可作无 migration 的容器回退，不允许旧 BE／Project 镜像读取新 schema。当前入口 `/opt/omsbizfoundry/acceptance/oa-login-csrf-20261003b/d1-active`；验证器与只读工具是显式补充驱动，不声称已在先前冻结的产品源归档中。

九份安全 JSON 已离机复制并逐份核对 SHA，未把 env、Cookie、密码、key 或私有备份加入仓库。[导出校验](../../acceptance/oa-initiation-repair/evidence/login-csrf-20261003/cloud/export-check.json)。公网后当前 wrapper 再次 check 退出 0；私有恢复 PG 和 restore20 保持停止。

## 继续工作的边界

原招投标组件与文件链以[此前报告](./oa-reused-bidding-ui-validation-20261003.md)为准。Main Evidence 仍 local，前端仍 ECS；OSS 分离、B03 分权／故障矩阵、其余 OA 流程、完整 Suite 恢复运行与跨客户验证未关闭，整体仍为 `insufficient-evidence`。
