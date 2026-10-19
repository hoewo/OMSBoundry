# OA C2 Business Engagement 权限与故障恢复实验

日期：2026-09-27  
结论：**正式入口、IAM 权限变化、409 与 Evidence 503 的自动化及真实浏览器恢复子命题通过；OA 整体仍为 `insufficient-evidence`**

## 1. 要回答的问题

OA 权限不是登录成功后把按钮藏起来。C2 必须同时证明：

> Nginx 与 Gateway 只承担入口边界，IAM／Policy 根据 Subject、Grant、Customer Overlay 和 Project Assignment 签发短期有界凭证，Business Engagement 再按 action、scope、revision 和当前状态作最终授权；权限变化、并发冲突或依赖故障不能产生越权或部分写入，并且用户有可解释的恢复路径。

## 2. 责任边界

| 组件 | 本实验中的责任 | 明确不负责 |
| --- | --- | --- |
| Nginx | 唯一对外入口与反向代理 | 账号、角色、业务授权 |
| Gateway | 路由、外来身份 Header 清洗、进入 IAM 换证 | 保存 Grant、解释客户岗位、最终授权 |
| IAM／Policy | Session、Subject、Grant、Overlay／Assignment Policy、短期 Capability Credential 与审计 | 转发每个业务请求、修改 Engagement |
| Business Engagement | 复验 Audience／Action／Scope，结合本地 revision、状态和引用结果最终授权并写历史 | 读取 IAM／Project／Evidence 内部表 |

身份超级管理员没有隐式经营业务权；Project Assignment 只作为版本化 Business Fact 输入 IAM／Policy；页面动作来自目标应用返回的 `allowed_actions`，隐藏按钮不代替服务端授权。

## 3. 可复验入口

隔离 Compose project 为 `omsbizfoundry-c2`，唯一宿主入口为 `http://127.0.0.1:18101`。故障注入只停止该 project 的 Evidence Service 容器：

```bash
D1_ENV_FILE=/absolute/path/d1-c2.env \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
D1_COMPOSE_OVERRIDE_FILE=/absolute/path/d1-c2-networks.yaml \
C2_ENABLE_DEPENDENCY_FAILURE_TEST=true \
  acceptance/d1-c2-business-engagement-authorization/verify-running
```

脚本在退出清理中保证恢复 Evidence Service。`D1_COMPOSE_OVERRIDE_FILE` 必须是存在的绝对路径，避免故障命令作用到不同 Compose 定义。

## 4. 409 并发恢复结果

验收先以 expected revision 2 完成方式选择，使 Engagement 到达 revision 3；随后再提交携带旧 revision 2 的适用性命令：

- 返回 `409 REVISION_CONFLICT`，`retryable=true`；
- 重新读取详情仍为 revision 3，方式仍为 `comparison`；
- 历史仍只有 `engagement_created / applicability_changed / method_selected`，没有冲突命令事件；
- 页面收到 409 后清除待重试命令、关闭编辑弹窗、读取最新详情，不自动重放旧 revision。

这证明“重新载入”不是把陈旧命令再发一次，目标应用的 expected revision 仍是最终写门禁。

## 5. Evidence 503 与恢复结果

在已有一条有效材料、Engagement revision 4 时停止 Evidence Service，再提交同一材料业务意图：

- 经 Nginx → Gateway → Business Engagement 返回 `503 REFERENCE_SERVICE_UNAVAILABLE`，`retryable=true`；
- Business Engagement 详情仍为 revision 4，活动材料仍恰好一条；
- 页面保留当前投影和待提交意图，明确显示“没有写入”，提供 Project 返回、重新载入和显式重试；
- Evidence 恢复健康后，验收以新的 `command_id` 重试同一意图，成功到 revision 5；
- 最终活动材料仍恰好一条，旧材料只按正常 supersede 历史保留，没有重复活动事实。

该行为同时证明 Business Engagement 不会在引用验证失败后先提交本地半写，也不会把依赖恢复解释成自动授权。

## 6. IAM 与正式入口回归

最新镜像重建后的最终复验共执行 **95 个 HTTP 断言，状态不匹配 0，最长本地请求 0.598558 秒**。IAM 审计记录为 Policy 27、Project 工作负载 9、Evidence 工作负载 9、撤权 84。除 409／503 外还覆盖：

- 经营负责人、生产负责人、经营经理、总经理、财务和身份管理员的允许／拒绝矩阵；
- Project A 精确 Scope 与 Project B 拒绝；
- 伪造 Authorization／`X-OMS-*` Header 失败；
- Grant 撤销后新凭证与服务端 `allowed_actions` 立即收敛；
- Assignment 结束后拒绝新签发，旧凭证残余只限签名 TTL；
- Business Engagement 用工作负载身份换取精确 Project／File 凭证，容器内没有静态下游 Token；
- IAM 记录 Policy、工作负载签发和撤权审计，目标应用保存不可变业务历史。

页面脚本从真实 Nginx／Gateway 路径读取，并通过 `node --check`；验收同时守卫 409、503、新 `command_id` 重试和携带 `project_id` 返回 Project 的实现标记。Project 页面已支持按该 `project_id` 自动打开来源项目。

## 7. 真实浏览器受控点击记录

2026-09-27 在已解锁的 macOS 上，通过可见的 Codex In-app Browser 对隔离入口 `http://127.0.0.1:18101` 完成最终复核。使用一次性 IAM 账号，经 Workbench → Gateway → IAM 分别进入 Project Workspace 和 Business Engagement；账号在验收后已停用，Suite Session 再读返回 `401`。

本次使用未被 Engagement 占用的 active Project `2152627e-d9a2-4425-8017-1c78b6c35078`（source revision 7）建立 Engagement `2a533e22-724d-4fcf-b2ba-b53ec1a8f53c`，观察如下：

1. 浏览器先把 Engagement 推进到 revision 3；另一已授权会话把方式更新为 `competitive_negotiation`、revision 4 后，浏览器仍以旧 revision 提交 `tender`。页面实际显示“本次操作没有写入”“检测到并发或状态冲突”“旧 revision 没有写入，也不会自动重放”，同时详情已读取 revision 4 和“竞争性谈判”，且没有出现重试旧命令的按钮。
2. 用户重新决定为 `comparison`，Engagement 到达 revision 5。填写 Evidence File `66dbd0ab-f13c-4a4b-b285-3cd374fa7c21`、version／source revision 1 后，停止且仅停止 `omsbizfoundry-c2-evidence-service-1`。提交后页面实际显示“依赖服务暂时不可用”“本次操作没有写入”，revision 仍为 5，全部表单输入仍保留，并出现“使用新命令重试”。
3. 在故障面板中以新标签打开“返回 Project 核对”，浏览器实际到达 `/project/?project_id=2152627e-d9a2-4425-8017-1c78b6c35078`，Project Workspace 自动选中同一 Project，并显示其 active 状态、revision 7、责任装配与只读动作。
4. Evidence Service 恢复 `healthy` 后点击“使用新命令重试”，页面成功推进到 revision 6，只登记一条中标文件引用。数据库复核为 `revision=6, active_materials=1, active_award=1`，证明失败请求没有半写，显式重试也没有产生重复活动事实。
5. 撤销该账号全部 Business Engagement Grant 后点击“刷新权限”，页面实际显示“权限上下文已经变化”，`allowed_actions` 收敛为空，材料表单退出编辑态，仅保留“返回统一工作台”；这不是只隐藏按钮，而是重新换证失败后的受控退出。

故障注入结束后 Evidence Service 已恢复 `healthy`。一次性账号已停用；本轮没有改变正式用户、正式 Project 或其他 Compose project。

## 8. 局限与下一步

真实浏览器故障恢复缺口已经关闭，但它没有证明跨租户、正式账号停用全旅程、IAM／Policy 故障全部组合、真实 Ubuntu／公网 HTTPS、C3～C8 权限矩阵或 OA 整体可行性。上述结论继续保持 `insufficient-evidence`。
