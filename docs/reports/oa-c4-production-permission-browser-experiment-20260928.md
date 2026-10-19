# OA C4 Production 权限生命周期与真实浏览器实验

日期：2026-09-28

结论：`production-permission-browser-lifecycle-compatible`。该结论关闭 Production 专项 Grant 撤销、账号停用、Assignment 结束和真实浏览器权限恢复缺口；不替代双 Tenant 或真实 Ubuntu／公网环境验收，OA 整体仍为 `insufficient-evidence`。

## 验证对象

隔离 D1 组合 `omsd1c4` 只启用 People & Groups、Project Workspace、Evidence Service 与 Production Operations，公开入口为本机 `http://127.0.0.1:18140`。浏览器和自动验证都经过正式 Nginx → Gateway → IAM／Policy → Production 链路。

Production 列表与详情由目标应用返回 `allowed_actions`；页面只按该集合展示写入口。显式刷新会由 Gateway 使用当前 Suite Session 向 IAM 重新签发 Capability Credential；`401/403` 会立即清空页面动作、关闭提交态并显示权限恢复入口。

## 自动验收

实际执行：

```sh
acceptance/d1-c4-production-authorization/run-isolated verify
```

结果为 **86 个 HTTP 断言、0 个状态不匹配**。在原有最小组合、相邻角色／Project、IAM／Evidence 故障和业务闭环之外，新增证据包括：

- 独立非管理员浏览器操作者只通过显式 `production-work-item.*` Grant 获得动作；
- 撤销 `production-work-item.stage.manage` 后，旧 Credential 只在签名剩余窗口内有效；显式刷新后详情 `allowed_actions` 删除该动作，直接写请求返回 `403`；
- 恢复 Grant 并刷新后动作重新出现；
- 停用账号后，旧应用 Credential 仍只保留最长 300 秒残余读取窗口，但 Suite Session、新授权和重新登录立即返回 `401`；Gateway 清除应用 Cookie；
- 重新启用后必须建立新 Session、重新授权，目标动作才恢复；
- Project Assignment 结束后新 Policy Credential 拒发，结束前凭证只保留签名残余窗口；
- IAM 不可变审计分别出现 capability revoked、subject disabled 与 subject active。

## 真实浏览器

使用 Codex in-app browser 从统一登录页进入 Workbench，再从 `production-operations` 卡片进入 Production 工作区。实际观察到：

1. 初始列表和详情同时显示 `stage / manage`；
2. 执行受控撤权并点击“刷新权限”后，列表与详情都不再显示该动作，两个阶段只显示“当前无可用动作”；
3. 恢复 Grant 并刷新后，列表和详情重新出现 `stage / manage`；
4. 停用账号并刷新后，页面显示“权限上下文已经变化”，所有业务动作清空，提交态退出；
5. 重新启用账号后，旧页面不能静默恢复；返回工作台重新登录、重新授权，再进入同一 Work Item 后动作恢复。

浏览器验证使用 `browser-permission-control` 切换测试账号状态，不直接修改业务库或绕过 IAM。验收结束时账号已启用、被撤销的 Grant 已恢复。

## 备份与恢复

实际执行联合备份 `20260927T190404Z`，随后仅销毁并重建 `omsd1c4` 专属卷：

```sh
D1_CONFIRM_EMPTY_RESTORE=yes \
D1_ENV_FILE="$PWD/acceptance/d1-c4-production-authorization/d1.env.example" \
D1_PROJECT_NAME=omsd1c4 \
D1_COMPOSE_OVERRIDE_FILE="$PWD/acceptance/d1-c4-production-authorization/compose.override.yaml" \
  deployments/d1/ops/d1 restore 20260927T190404Z

acceptance/d1-c4-production-authorization/run-isolated verify-restored
```

恢复后 consultation Work Item `c56438ba-883a-4cc2-93b5-e6762b57c02d` 的 canonical SHA-256 为 `1b74b9a950f2e58e156a9ce75563d5ca922e9b0b64aa54281ffad434c7b89241`，保持 completed revision 5、一个成果和 5 条历史；design Work Item 与 IAM 故障恢复记录也保持。Evidence 恢复报告为 11 个 File、11 个 Version 和 11 个对象。

## 仍未证明

- 两个真实 Tenant 在同一 D1 中对相同与相邻资源的完整交叉拒绝矩阵；
- 公司私有 Ubuntu 或个人阿里云试验田的真实域名、TLS、升级和恢复；
- C5～C8 业务闭环及其权限旅程；
- 生产规模容量、HA、D2～D4 和跨客户长期升级。
