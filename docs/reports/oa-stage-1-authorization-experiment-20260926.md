# OA 第一阶段权限实验报告：Project 资源与业务事实边界

日期：2026-09-26  
结论：`insufficient-evidence`，但“直接资源授权”“最小 Business Fact Policy”“工作负载凭证生命周期”和“C1 项目责任权限子矩阵”四个子命题已通过

## 1. 本轮要回答的问题

参考 OA 的权限不是一次登录或一个管理员账号可以代表的。本轮只验证一个可证伪的小问题：

> 同一租户内，一个非管理员操作者只获得 Project A 的读取和修改权限时，系统能否让他操作 A，同时在凭证签发、读取和修改三个层面拒绝 Project B，并且拒绝请求不改变 B 的业务事实？

第一组实验不把直接管理员授权冒充完整 OA 策略；第二组实验补入一条 Project Assignment Business Fact Policy。跨租户反例、身份超级管理员反向隔离和 C1～C8 多角色矩阵仍不在阶段通过范围内。

## 2. 被验证的责任链

```text
Browser / acceptance client
  → Nginx（唯一宿主入口）
  → Gateway（路由、Session 换取能力凭证、基础验签）
  → IAM（Subject、Grant、Audience、Action、Resource Scope、短期签名）
  → Project Workspace（再次验签、精确 Project Scope、动作与领域最终授权）
  → Project 自有 PostgreSQL
```

重要边界：Gateway 不拥有账号、Grant 或 Project 事实；IAM 不读 Project 数据库；Project 不解释客户岗位名称，也不把 Gateway 的通过当作最终授权。

## 3. 环境与装配

| 项目 | 实际值 |
| --- | --- |
| Deployment Profile | 本地隔离 `D1 Compact` |
| Compose project | `omsd1resource` |
| 唯一宿主 HTTP 入口 | `127.0.0.1:18110` |
| 应用选择 | IAM、Workbench、People、Project、Contract、Money、Evidence、Gateway、Nginx |
| 数据边界 | 一个 PostgreSQL 实例，六个应用数据库／账号；台账与 Evidence 继续使用各自既有恢复格式 |
| Project schema | `000001_project_workspace.sql` |
| IAM schema | `000002_resource_capability_grants.sql` |
| Evidence Provider | Local；不代表 OSS 或公网 HTTPS 验收 |

这是本地真实运行栈证据，不是公司 Ubuntu、公网域名、TLS 或 ICP 生效后的外部证据。

## 4. 操作者与数据集

1. bootstrap 管理员通过正式 IAM Grant API 获得 Project tenant actions，仅用于建立测试数据和授权；
2. 管理员经 Nginx／Gateway 创建 Project A 与 Project B；
3. 新建并激活一个非管理员 Subject；
4. 只向该 Subject 授予：
   - `project-workspace.read`
   - `project-workspace.update`
   - Scope：唯一 `{"type":"project","id":"<Project A ID>"}`
5. 不授予 tenant scope、Project B、Project 创建／立项／取消等动作。

## 5. 可复验步骤

在已配置的隔离 D1 环境执行：

```bash
D1_PROJECT_NAME=omsd1resource \
PUBLIC_PORT=18110 \
EVIDENCE_PUBLIC_BASE_URL=http://127.0.0.1:18110/evidence \
  deployments/d1/ops/d1 check

D1_PROJECT_NAME=omsd1resource \
PUBLIC_PORT=18110 \
D1_PUBLIC_URL=http://127.0.0.1:18110 \
  acceptance/d1-resource-authorization/verify-running

D1_PROJECT_NAME=omsd1resource \
PUBLIC_PORT=18110 \
D1_PUBLIC_URL=http://127.0.0.1:18110 \
  acceptance/d1-delivery/verify-running
```

恢复演练使用测试专用 Compose project，命令会删除该 project 的卷：

```bash
D1_PROJECT_NAME=omsd1resource \
PUBLIC_PORT=18110 \
EVIDENCE_PUBLIC_BASE_URL=http://127.0.0.1:18110/evidence \
  deployments/d1/ops/d1 backup

D1_PROJECT_NAME=omsd1resource \
PUBLIC_PORT=18110 \
EVIDENCE_PUBLIC_BASE_URL=http://127.0.0.1:18110/evidence \
D1_CONFIRM_EMPTY_RESTORE=yes \
  deployments/d1/ops/d1 restore <BACKUP_ID>
```

本轮实际恢复的备份 ID 为 `20260926T130918Z`。恢复后再次执行上述资源授权专项和完整 D1 验收。

## 6. 允许／拒绝矩阵

| 验证 | 预期 | 实际 |
| --- | --- | --- |
| 非管理员省略资源请求 Project tenant credential | `403` | 通过 |
| 非管理员请求 Project B credential | `403` | 通过 |
| 非管理员请求 Project A credential | `303` 并取得短期能力 Cookie | 通过 |
| Credential audience | 只有 `project-workspace` | 通过 |
| Credential actions | 只有 `read`、`update` | 通过 |
| Credential scope | 只有 Project A | 通过 |
| Credential TTL | `0 < exp-iat <= 300s` | 通过 |
| 项目列表（tenant action） | `403` | 通过 |
| 读取 Project A | `200` | 通过 |
| 读取 Project B | `403` | 通过 |
| 修改 Project A | `200`，revision `1 → 2` | 通过 |
| 修改 Project B | `403` | 通过 |
| 拒绝后 Project B | 名称不变，revision 仍为 `1` | 通过 |
| `/project/` 页面与前缀内 JS | 经统一入口加载 | 通过 |
| 恢复后资源专项 | 全部再次通过 | 通过 |
| 恢复后完整 D1 | 登录、People、Project、台账、Evidence 生命周期通过 | 通过 |

## 7. 本轮发现并修正的问题

### 7.1 权限语言不一致

早期 Project 使用过 `project.*` 语义，而正式 audience 是 `project-workspace`。本轮统一为 `project-workspace.*`，由 Capability Manifest、IAM Catalog、Access Contract 和 Project 测试共同守卫。说明权限解耦不仅需要“有 Token”，还必须让签发者和执行者使用完全相同的动作语言。

### 7.2 Gateway 子路径下页面能打开但交互失效

Project 页面曾使用根路径 `/assets` 和 `/api`，直接运行正常，经 `/project` 装配后会请求错误地址。本轮改为从 suite prefix 计算资源和 API 地址；Gateway 模式使用已交换的能力凭证，不要求用户再粘贴 Token。该问题证明独立 Demo 通过不能替代正式装配验证。

### 7.3 备份恢复的跨容器就绪竞态

恢复脚本只等 PostgreSQL 容器内健康，随后 IAM `pg_restore` 仍可能遇到跨容器 TCP 尚未接受连接。现改为使用 IAM 的实际 `DATABASE_URL` 执行有界 `pg_isready`，再恢复数据。修复后销毁测试栈卷、恢复六个应用数据边界和签名密钥、schema 检查、权限专项及完整 D1 回归均通过。

## 8. 本轮可以下的结论

- Project 已进入可开关的 D1 Suite Manifest、独立数据库／migration、镜像、健康、首管授权、联合备份和空目标恢复边界；
- Nginx、Gateway、IAM 和 Project 的直接资源授权闭环可行；
- IAM 能签发只包含指定 audience、actions 和 Project ID 的短期凭证；
- Gateway 没有计算业务权限，Project 仍执行最终授权；
- 跨 Project 的凭证签发、读取和写入都能失败关闭，拒绝写入不改变业务事实；
- 恢复后权限链和完整组合可继续运行。

## 9. 直接 Resource Grant 实验后仍不能下的结论

- 不能宣称 OA 完整体权限通过；
- 不能宣称 Business Fact Snapshot 或动态 Policy 已实现；
- 不能证明“项目负责人／生产负责人／财务”等客户角色只改策略即可适配；
- 不能证明单项撤权后新凭证立即失败、旧凭证只保留最多五分钟残余窗口；
- 不能证明跨租户、字段级、金额级、文件级和业务状态拒绝矩阵已经完整；
- 不能用本地 HTTP 证据代替真实 Ubuntu、公网 HTTPS、DNS、TLS 或公司环境证据。

因此阶段总判断仍为 `insufficient-evidence`，但“精确 Project 直接授权可以在既有解耦边界内成立”已获得直接支持。

## 10. 下一项最小实验

下一轮不扩张到万能权限平台，而是只增加一条真实业务事实策略：

1. Project 发布带来源、revision 和有效时点的 Assignment／Resource Business Fact Snapshot；
2. IAM 内可信 Policy 端口把“某 Subject 是 Project A 的某类负责人”映射为 Project Workspace 的最小 actions 和 Scope；
3. 同一 Subject 对 A 允许、对 B 拒绝；Assignment 结束或 revision 失效后，新凭证签发失败关闭；
4. 旧凭证只按最长五分钟 TTL 存活；
5. 增加单项 Grant 撤销后重复同一矩阵；
6. 再把这一机制投到 C1 的真实角色矩阵，而不是先覆盖 C1～C8。

该实验已经由下节的最小 Business Fact Policy 复验完成；它只把结论推进到“单条业务事实驱动授权可行”，并不等于 OA 完整体权限可行。

## 11. 最小 Business Fact Policy 补充实验

### 11.1 新增责任链

```text
Browser / acceptance client
  → Nginx
  → Gateway（只转交不透明 Business Fact 引用）
  → IAM / Policy（核验 Subject、目标 scope 与两类版本化事实并签发）
      → Project Workspace Policy Fact（Assignment）
      → People & Groups Policy Fact（Person → IAM Subject）
  → Project Workspace（离线复验凭证并作最终授权）
```

Project Workspace 拥有 Assignment；People & Groups 拥有 Person 和 IAM Subject 映射；IAM 拥有策略求值、Grant 与凭证签发；Gateway 不读取事实或解释客户角色；目标应用不读取 Project／People 内部表。

### 11.2 环境与可复验命令

本次使用独立 Compose project `omsd1policy`，仅把 Nginx 发布到 `127.0.0.1:18120`，没有替换已有 18100／18110 验收数据。

```bash
PUBLIC_PORT=18120 D1_PROJECT_NAME=omsd1policy \
  deployments/d1/ops/d1 up

D1_PROJECT_NAME=omsd1policy \
D1_ENV_FILE=deployments/d1/.env \
D1_PUBLIC_URL=http://127.0.0.1:18120 \
  make d1-business-fact-authorization-acceptance
```

### 11.3 实际矩阵

| 验证 | 实际结果 |
| --- | --- |
| Project Assignment 事实 | 返回 source、稳定 ID、创建 revision、有效时点、活动状态和 Person 引用 |
| People Person 事实 | 返回 source、Person revision、状态和 `iam_subject_id` |
| 非管理员请求 Project A | IAM 按 `project-assignment-access@1.0.0` 签发 `read/update` |
| 同一非管理员读取 Project A | `200` |
| 同一非管理员读取 Project B | `403` |
| Project A 修改 | `200`，目标应用执行最终授权和 revision 校验 |
| 增加无关直接 Grant | 新凭证额外含 `history.read` |
| 只撤销该直接 Grant | 新凭证移除 `history.read`，保留策略产生的 `read/update`，账号仍有效 |
| 结束 Assignment | Assignment revision 推进，使用旧 revision 的新凭证请求 `403` |
| 结束前已签凭证 | 仍只在 `exp-iat <= 300s` 的契约残余窗口内工作 |
| 审计 | IAM 同时存在策略凭证签发和单项 Grant 撤销记录 |

### 11.4 实现中暴露并修正的问题

- POSIX shell 函数变量是全局的，验收脚本曾把撤权载荷覆盖成凭证载荷，导致撤权请求 `400`；现使用独立 `grant_payload` 并在失败时输出错误响应。
- IAM 的幂等 bootstrap 重复授予已有 active Grant 时，PostgreSQL `ON CONFLICT ... WHERE revoked_at IS NOT NULL` 返回零影响行，旧实现误报“有效用户不存在”；现区分 active Subject 下的重复 Grant 并返回 `409 Conflict`，被撤销的 Grant 仍可重新激活。

### 11.5 本轮可以下的新增结论

- 业务事实不必复制进 IAM 或目标应用数据库，也能产生精确到单一 Project 的短期授权；
- IAM 可以承担 OA 中的重要权限治理工作：组合身份、直接授予、版本化业务事实和版本化策略，并输出目标应用原生 Capability；
- Gateway 保持为安全入口和凭证交换层，没有演变为第二个权限系统；
- 目标应用仍能独立执行 scope、action、revision 和领域状态的最终授权；
- 单项撤权、Assignment 结束和短凭证残余窗口可以被观察和审计。

### 11.6 仍然不能下的结论

- Project 校验 Person 引用所需的服务凭证自动获取、续期和无重启轮换已由下一节补齐，不再属于缺口；
- 尚未证明身份超级管理员与业务管理员的反向隔离、跨租户、组织／Group、临时代理、字段、金额、文件和业务状态权限；
- 三个稳定 Project 角色键的策略配置不是完整 OA 客户角色模型；
- 尚未完成 C1～C8 每条旅程的允许、相邻角色拒绝、跨资源拒绝和状态拒绝；
- 本地 HTTP D1 仍不能替代公司 Ubuntu 或公网 HTTPS 的真实环境证据。

因此阶段总判断继续保持 `insufficient-evidence`。服务凭证生命周期由下一节单独补证；其后应把已验证的机制投入 C1 项目责任矩阵，而不是把更多客户职位名硬编码进 IAM、Gateway 或 Project。

## 12. 工作负载身份与服务凭证生命周期补充实验

### 12.1 要验证的问题

上一轮用验收脚本把一份正式 IAM 签发的 People 短期凭证注入 Project 进程，只能证明 API 契约，不能作为生产方案。这个实验要求：

- Project 不保存长期 People Bearer Token，也不复用用户 Session；
- IAM 能识别 Project 这个工作负载，并限制它只能申请 People `view` 的 Person／Group 精确资源凭证；
- 凭证自动获取、缓存、接近过期刷新和 `401` 后单次重换不要求重启 Project；
- 工作负载私钥可以轮换，IAM 不持私钥；
- IAM 或 People 停机时，新 Assignment 失败关闭，但既有 Project 读取保持可用，恢复后无需重启 Project。

### 12.2 实现责任链

```text
Project Workspace（独占工作负载私钥）
  → 生成 60 秒、一次性、请求 SHA-256 绑定的 Ed25519 Assertion
  → IAM /api/v1/service-credentials
IAM（工作负载公钥＋版本化 Workload Policy＋PostgreSQL replay store）
  → 最长 60 秒、单 people-groups.view、精确 Person／Group Scope Credential
Project Workspace
  → People & Groups reference-read
People & Groups
  → 复验 audience/action/精确 resource scope，返回最小引用
```

IAM 成功交换写入 `iam_service_credential_issued` 不可变审计，Actor Kind 为 `service_identity`，Details 保存 Workload／Key／Assertion ID 和目标 scope，不记录秘密或 Token 原文。People 的 Resource Scope 只能读取对应单个 Person 或 Group，不能借 `people-groups.view` 列举租户目录、读取 catalog 或访问其他资源。

### 12.3 D1 密钥放置与轮换

D1 使用两个不同卷：

- Project 只挂载 `project-workload-private`；私钥文件要求 `0600`；
- IAM 只挂载 `iam-workload-public`；公钥集合允许 `active` 与短时 `retiring`；
- `workload-keyctl init/rotate/prune` 生成和轮换文件；Project 每次签 Assertion 重新读取私钥，IAM 每次验签重新读取公钥。

这只是 D1 的节点放置方式。D2～D4 使用同一身份、Assertion、交换 API 和 Capability Credential，只需把私钥／公钥分别安全分发到对应节点。

### 12.4 实际运行证据

隔离 Compose project `omsd1policy` 发布在 `127.0.0.1:18120`。同一 `acceptance/d1-business-fact-authorization/verify-running` 实际通过：

| 验证 | 实际结果 |
| --- | --- |
| 静态凭证 | Project 环境没有 `PEOPLE_ACCESS_TOKEN` |
| 首次引用验证 | Project 自动向 IAM 换取精确 Person Credential，Assignment 创建成功 |
| Assertion 约束 | 请求替换失败，重复 `jti` 返回冲突；凭证 `exp-iat <= 60s` |
| People 最终授权 | 只允许凭证中的 Person／Group；列表、catalog 和相邻资源拒绝 |
| 密钥轮换 | 轮换前后两个 Key ID 均进入 IAM 审计；Project 容器 ID 不变 |
| IAM 停机 | 新精确引用 Assignment 返回 `503`；既有 Project 读取 `200` |
| IAM 恢复 | 无需重启 Project，新 Assignment 成功 |
| People 停机 | 新精确引用 Assignment 返回 `503`；既有 Project 读取 `200` |
| People 恢复 | 无需重启 Project，新 Assignment 成功 |

实现还通过 Project、IAM、People 单元／HTTP 测试，IAM PostgreSQL 18 migration／系统集成测试和 Access Contract 稳定 Schema 检查。

### 12.5 本轮结论与剩余边界

这轮消除了上一节唯一依赖人工 Token 注入的服务身份缺口，证明了 `Nginx + Gateway + IAM + Project + People` 组合中的机器身份也能遵守最小权限、轮换、审计和失败关闭，同时没有让 IAM 读取业务表或让 Project 获得签发最终权限的能力。

阶段结论仍为 `insufficient-evidence`：该结果只证明一条 Project→People 精确引用链，尚未证明 C1～C8 的人类角色矩阵、身份超级管理员默认无业务权、跨租户、临时代理、金额／字段／文件／状态权限，也没有代替真实公司 Ubuntu 或公网 HTTPS 证据。下一最小实验是把已验证机制投入 C1 项目责任的多角色／多资源正反矩阵。

## 13. C1 项目责任多角色／多 Project 补充实验

### 13.1 问题、假设与反证条件

**问题**：参考 OA 的站点角色、项目责任、资源范围、撤权和 Project 状态同时存在时，能否继续由 IAM 承担重要权限治理，而不把客户岗位、共享角色表或最终授权塞进 Gateway／Project？

**假设 H1**：客户站点角色经版本化 Overlay 映射为 Project 稳定 Capability，项目责任经版本化 Assignment Fact 映射为精确 Project Scope；Gateway 清洗入口，IAM／Policy 签发有界凭证，Project 用自身状态和 revision 最终授权，可以完成 C1 已实现部分。

**零假设 H0**：要实现参考 OA 权限，必须让 Project 硬编码客户岗位、让 IAM／Gateway 读取 Project 内部表、共享角色表，或允许经理／管理员越过资源和状态边界。

以下任一结果反驳 H1：身份管理员自动取得 Project 权限；经营经理可以管理生产负责人；同一负责人可以访问 Project B；参与者可以通过伪造 Header 写入；`active` 状态规则可被 Capability 绕过；Assignment 结束后仍能用旧 revision 签发新凭证；拒绝请求改变业务 revision。

### 13.2 实验变量和控制

| 类型 | 本轮设置 |
| --- | --- |
| 控制变量 | 同一代码版本、同一 D1 Compose、同一 Tenant、正式 IAM Provider、Nginx 唯一宿主入口、同一 Project／People schema |
| 自变量 | 站点角色、Project A／B、Capability action、Assignment role/fact/revision、Project 状态、Grant 是否撤销、是否伪造 Header |
| 因变量 | Credential 签发结果、HTTP 状态、Project revision／名称、凭证 actions／scope、IAM 审计、请求时间 |
| 操作者 | identity super admin、OA business admin、general manager、business manager、production manager、project business owner、project production owner、participant、finance |
| 业务对象 | 两个 Project；Project A 的双负责人和参与者 Assignment；Project B 作为跨资源反例 |

该实验没有用全能力管理员代替角色矩阵。bootstrap 身份只建立测试账号和显式 Grant；撤销全部 Project Grant 后重新登录，用于证明 IAM 管理权不产生业务权。

### 13.3 实现前发现的边界过粗

原有 `project-workspace.assignment.manage` 可以管理全部 Assignment。如果直接授予经营经理或生产经理，两者都能更换全部负责人，客户 Overlay 无法在不修改 Project 代码的情况下表达最小职责。

本轮在 Base 增加三项稳定、领域可复用的 Capability：

- `project-workspace.assignment.business-owner.manage`；
- `project-workspace.assignment.production-owner.manage`；
- `project-workspace.assignment.participant.manage`。

通用 `assignment.manage` 保留给显式业务管理员。Project 在 `assign` 时按请求 role key、在 `end` 时按数据库内既有 Assignment role key 二次选择所需动作，客户端不能通过省略或伪造角色绕过。客户角色仍只写在 `reference-oa-c1@1.0.0` Overlay，Schema 校验未知 action、错误 audience 和跨应用 action 失败关闭。

### 13.4 可复验命令

```bash
PUBLIC_PORT=18130 \
EVIDENCE_PUBLIC_BASE_URL=http://127.0.0.1:18130/evidence \
D1_PROJECT_NAME=omsd1c1 \
  deployments/d1/ops/d1 up

D1_PROJECT_NAME=omsd1c1 \
D1_ENV_FILE=deployments/d1/.env \
D1_PUBLIC_URL=http://127.0.0.1:18130 \
C1_EVIDENCE_DIR=deployments/d1/.runtime/c1-evidence \
  make d1-c1-authorization-acceptance
```

实际环境的旧 `.env` 最初没有 `PROJECT_PEOPLE_BASE_URL`，新 Assignment 因显式 People 依赖不可达返回 `503 reference_unavailable`。补齐 `http://people-groups:8084` 并只重建 Project 容器后继续通过；系统没有跳过引用校验。连续调试触发 Gateway 每分钟限流 `429`，重启隔离 Gateway 清空内存窗口后取得干净样本。两点均属于环境／验收过程发现，不是被隐藏的成功。

### 13.5 允许／拒绝矩阵

| 验证 | 预期 | 实际 |
| --- | --- | --- |
| 身份超级管理员换取 Project 凭证 | `403` | 通过 |
| 身份超级管理员伪造 Actor／Allowed-Actions Header 访问 Project | Gateway 清洗并 `303` 返回 Workbench 授权入口 | 通过 |
| OA 业务管理员创建 Project A／B、装配责任、完成立项 | 允许 | 通过 |
| 经营经理建立／结束经营负责人 | `200` | 通过 |
| 经营经理建立／结束生产负责人 | `403`，revision 不变 | 通过 |
| 生产经理建立生产负责人 | `200` | 通过 |
| 生产经理结束经营负责人 | `403`，revision 不变 | 通过 |
| 项目双负责人读取／更新 Project A | `200` | 通过 |
| 同一负责人读取 Project B 或请求错误 Fact／Scope | `403` | 通过 |
| 参与者读取 A | `200` | 通过 |
| 参与者更新 A，包含伪造 Header | `403` | 通过 |
| 总经理／财务读取 Project | `200` | 通过 |
| 总经理／财务更新 Project | `403` | 通过 |
| Project active 后负责人修改基础资料 | `409`，名称／revision 不变 | 通过 |
| 结束 Assignment 后用旧 fact revision 新签发 | `403` | 通过 |
| 结束前已签发凭证 | 只在 `exp-iat <= 300s` 内继续 | 通过 |
| IAM grant／policy issuance／revoke 审计 | 均存在 | 通过 |

证据文件共 117 行（表头加 116 次请求）：46 个 `200`、18 个 `201`、29 个 `204`、10 个 `303`、12 个 `403`、1 个 `409`，预期与实际状态全部一致；最大本地请求时间 0.404434 秒。该时间只用于发现异常等待，不代表生产容量或公网性能。

### 13.6 Base／Overlay／数据边界变化账

| 层 | 本轮变化 | 数量 |
| --- | --- | --- |
| Base Capability | 三项稳定 Assignment 角色动作 | 3 |
| Base 目标授权 | `assign`／`end` 按目标 role key 二次检查 | 1 处边界 |
| Customer Overlay | 参考 OA C1 站点角色到 Project actions 映射 | 1 个版本化样例 |
| Policy | 复用 `project-assignment-access@1.0.0` | 0 个新增策略引擎 |
| 数据 migration／共享表 | 无 | 0 |
| 客户核心分支／客户条件代码 | 无 | 0 |

这次 Base 变化不是把参考 OA 岗位写进通用内核，而是修正了一个跨客户也成立的稳定职责边界：管理经营负责人不等于管理生产负责人。若第二客户需要不同岗位组合，只组合现有 actions；只有出现新的稳定 Assignment 语义时才评估 Base 兼容演进。

### 13.7 结论和下一缺口

H1 在 C1 已实现的项目责任范围内获得支持，H0 未出现：IAM 承担 Subject、Grant、Overlay／Policy 组合、短期凭证和审计；Project／People 提供各自事实；Gateway 不计算最终权限；Project 保留资源、状态、revision 和 Assignment 的最终决定。用户强调的“OA 权限是重活”已经进入可执行架构证据，不再只是文档原则。

阶段总判定仍为 `insufficient-evidence`。本轮没有证明项目包、父子转换、取消后恢复、跨租户、组织／Group 范围、临时代理、页面 `allowed_actions` 刷新体验、金额／字段／文件细粒度动作，也没有覆盖 C2～C8 或真实公司 Ubuntu／公网 HTTPS。下一轮应继续完成 C1 剩余业务生命周期，而不是横向增加一批抽象权限功能。
