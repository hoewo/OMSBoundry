# OA C2 Business Engagement 权限与正式入口实验

日期：2026-09-27  
结论：C2 第一组正式入口、角色、资源、撤权和服务身份矩阵通过；OA 完整体仍为 `insufficient-evidence`

## 1. 要验证的问题

参考 OA 的权限不是“登录后都能用”。经营承接同时受到站点角色、Project Assignment、目标资源、业务状态、revision 和撤权影响。本实验验证：

> Business Engagement 装入 D1 后，是否能经 `Nginx → Gateway → IAM／Policy → Business Engagement` 使用目标应用原生 Capability 完成经营闭环，同时保持 IAM 的重要治理职责、目标应用最终授权和 Project／Evidence 数据边界。

本实验不把身份超级管理员、页面按钮显隐或全能力 bootstrap 用户作为业务权限通过证据。

## 2. 实现与装配变化

- IAM 的 Project Assignment Policy 从单一目标扩展为按 `audience` 选择的多 Policy 集合；同一 Project／People 事实可以分别产生 Project Workspace 或 Business Engagement 的最小动作，重复 audience 失败关闭。
- 新增 `business-engagement-project-access@1.0.0`：经营负责人获得创建、读取、适用性、方式、材料和历史；生产负责人只读和读历史；普通参与者只读；项目角色不自动获得取消权。
- 新增 `reference-oa-c2@1.0.0`：OA 业务管理员和经营经理使用显式 tenant actions；总经理／生产经理只读；财务和身份超级管理员没有 Business Engagement 动作。
- Business Engagement 加入 D1 Suite Manifest、Workbench 菜单、独立数据库／账号／migration、独立工作负载密钥、健康检查、镜像构建、pull-only 映射、发布清单、备份／恢复命令和应用开关。它没有宿主端口，只经 Nginx 和 Gateway 暴露 `/business`。
- Business Engagement 页面使用同一路由和 HttpOnly 应用凭证；列表与详情由服务端返回 `allowed_actions`，页面不读取岗位名，显式权限刷新经 Gateway 让 IAM 重新签发并用新结果重绘动作。
- IAM 同时加载 Project 与 Business Engagement 两份 Assignment Policy；Business Engagement 只可用自己的工作负载身份换取精确 `project:{id}` 读取和 `file:{id}` 查看凭证，正式模式禁止静态 Project／Evidence Token。

没有新增共享业务表、跨应用 ORM、Gateway 角色判断、客户核心分支或目标应用中的客户岗位条件。

## 3. 运行环境

- 隔离 Compose project：`omsbizfoundry-c2`
- 公共入口：`http://127.0.0.1:18101`
- 选择集：IAM、Workbench、People & Groups、Project Workspace、Evidence Service、Business Engagement、Gateway、Nginx 和 PostgreSQL 18
- Contract 与 Money Flow 未启用，证明承接材料闭环不以二者为启动前提
- 本机 Docker 默认地址池已被其他保留环境占满，因此本实验只为隔离 project 增加四个临时私网 `/24`；没有删除或改动既有 Docker 网络
- 批量验收临时把该隔离 Gateway 的 `REQUESTS_PER_MINUTE` 从默认 120 提高到 1000，避免 89 个自动断言及初始化请求在一分钟内互相限流；正式默认值没有修改，限流本身不在本实验结论内

## 4. 直接证据

主入口：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
C2_EVIDENCE_DIR=/tmp/omsbizfoundry-c2-evidence \
acceptance/d1-c2-business-engagement-authorization/verify-running
```

最终复验结果：89 个 HTTP 断言，0 个状态不匹配，最长本地请求 0.944209 秒。覆盖：

1. 经营负责人通过 Project Assignment Fact 获得精确 Project A、单 audience、`exp-iat <= 300s` 的 Business Engagement Credential，动作不含 `cancel`；
2. 生产负责人只获得 `read/history.read`；经营经理使用显式 tenant Grant；总经理只读；财务换证为 `403`；
3. Project A 的 Assignment Fact 用于 Project B 时为 `403`，Gateway 清除旧应用 Cookie，重新授权 Project A 后恢复；
4. 经营负责人创建 Engagement、确认适用、选择方式并登记真实 Evidence File Version；Business Engagement 分别向 IAM 换取精确 Project 与 File 服务凭证，目标服务再次复验资源 ID；
5. 经营负责人取消返回 `403`，生产负责人和总经理写入返回 `403`，经营经理可以取消；取消后即使 Credential 含方式动作，目标应用仍按本地状态返回 `409`；
6. 伪造 `Authorization`、`X-OMS-*` 和允许动作 Header 不能建立业务身份；
7. 撤销经营经理的 `method.select` 后刷新凭证，动作从 Claims 消失，直接命令返回 `403`，其他未撤销动作仍可工作；
8. 结束经营负责人 Assignment 后，旧 revision 的新换证返回 `403`；已签凭证只在原最长五分钟残余窗口内工作；
9. 移除 bootstrap 身份管理员的显式 Business Engagement Grant 后，即使仍有 IAM 管理权，业务换证也返回 `403`；
10. Engagement 历史按顺序保留创建、适用性变化、方式选择、材料登记和取消五个事件。
11. Workbench 通过正式 Gateway／IAM 换证进入经营承接页面；页面与脚本只从 Nginx 公共入口取得，匿名业务 API 不能绕过入口；
12. 列表与详情返回服务端收敛的 `allowed_actions`：经营负责人没有取消权，直接 Grant 撤销后刷新立即移除方式动作，取消状态只保留读取／历史；Assignment 结束后新浏览器入口回到授权流程而不是沿用旧 Cookie。

### 实际浏览器复验

在同一隔离 D1 中，以经营经理从 Workbench 点击进入 Business Engagement，实际页面成功显示列表、详情、材料缺口、revision 和当前动作。页面保持打开时，通过 IAM 管理 API 撤销 `business-engagement.evidence.manage`，点击“刷新权限”后该动作从页面消失；恢复 Grant 并再次刷新后动作重新出现。该复验证明页面显隐与 IAM 新凭证／目标应用 `allowed_actions` 同源，但不替代尚未执行的 409／503 浏览器故障专项。

最终隔离数据库观察到的累计 IAM 审计为 Policy 签发 18、Business Engagement→Project 工作负载签发 6、Business Engagement→Evidence 工作负载签发 6、撤权 60。多次验收、页面复验和权限撤销／恢复都会留下不可变审计，因此这些是累计值；验收脚本按一次 clean execution 的最低证据要求 Policy ≥3、Project workload ≥1、Evidence workload ≥1、撤权 ≥8 判定，不把累计值冒充单次请求数。

## 5. 既有权限模型回归

同一运行组合随后执行：

```bash
D1_ENV_FILE=/tmp/omsbizfoundry-d1-c2.env \
D1_PROJECT_NAME=omsbizfoundry-c2 \
D1_PUBLIC_URL=http://127.0.0.1:18101 \
acceptance/d1-c1-authorization/verify-running
```

C1 既有 Project 权限矩阵仍以 152 个 HTTP 断言、0 个状态不匹配通过，最长本地请求 0.186476 秒。多 Policy 改造没有改变 Project 的角色、Scope、Package、状态、撤权或身份管理员隔离语义。

同时通过：

- IAM config／Policy、Business Engagement 和相关普通测试；
- `go vet`；
- 新 Capability／Overlay／Policy JSON 校验；
- D1 Compose、Suite Manifest、镜像模式、Ingress 模式和 release bundle 自测；
- Business Engagement Linux 容器镜像实际构建。

## 6. 架构结论

本实验支持以下子结论：

- IAM 在 OA 中确实承担重职责：账号／Session、Capability Catalog、显式 Grant、Project Assignment Policy 求值、短期用户／工作负载凭证签发、撤权和不可变审计；
- “重 IAM”不等于“万能 IAM”：它不拥有 Project Assignment、Person、Engagement 状态或 Evidence File，不转发每个业务命令，也不能让身份管理员自动越权；
- Gateway 只负责入口换证、旧 Cookie 清除、Header 清洗、路由和基础限流；客户角色与业务状态没有进入 Gateway；
- Business Engagement 最终复验 audience、action、Scope、revision、外部引用和本地状态，证明 IAM 签发成功不等于业务命令必然成功；
- 同一版本化 Assignment Fact 可以通过 audience 专属 Policy 为不同目标应用产生不同最小动作，不需要复制 Project 数据或共享权限表。

因此，`Nginx + Gateway + IAM` 的权限架构已从“只适用于 Project”推进为“同一可信链可装入第二个独立业务应用”的运行证据。

## 7. 未通过与下一步

本实验仍不证明：

- 跨租户矩阵、账号停用对 C2 的专项行为、IAM／Policy／Project／Evidence 故障期间的 C2 连续性；
- 409／503 的真实浏览器恢复、Project 上下文到 Business Engagement 的完整深链接旅程；
- Business Engagement 联合备份后的空目标恢复、pull-only 不可变制品在另一 Ubuntu 主机运行；
- 下游 Contract／Money 建议、未中标／流标／多轮承接、Party 结构化主体；
- C3～C8 权限旅程和 OA 完整体。

下一最小切片应对当前已接入 D1 的数据做联合备份、空卷恢复和恢复后继续写入，同时补齐 409／503 浏览器恢复专项。真实公司 Ubuntu／公网环境准备完成后，复用同一镜像和验收入口，不在目标机重写权限或业务逻辑。
