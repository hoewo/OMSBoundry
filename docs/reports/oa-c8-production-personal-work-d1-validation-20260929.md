# C8 Production 个人工作正式 D1 验证

日期：2026-09-29  
状态：`passed-in-tested-scope`  
运行环境：公司 Ubuntu D1 Compact，2 vCPU／2 GiB，`linux/amd64`

## 1. 要验证的问题

一句话目标：

> Production 中“由当前人员负责且仍可执行”的工作项，经 `Nginx → Gateway → IAM → Production` 只读投影到当前登录人的 Workbench；相邻人员不可见，撤权后新凭证收敛，来源故障不拖垮工作台，也不产生业务写入。

这条旅程专门验证一个容易被错误实现的解耦边界：

- IAM Subject 是登录与授权身份；
- People Person 是人员名册事实；
- Project Assignment 记录业务责任人 Person 和引用的 People revision；
- Production 拥有 Work Item 及其阶段；
- Workbench 只组合来源投影，不拥有个人待办事实。

Person ID 与 IAM Subject ID 即使在某些夹具中碰巧相同，也不是同一种标识，不能直接比较。

## 2. 不得破坏的约束

1. Production 不得读取 People、Project 或 IAM 数据库，也不得复制这些应用的 ORM／领域模型。
2. Project 继续拥有 Assignment；People 继续拥有 Person 与 IAM Subject 的当前有效关联；Production 只保存自身 Work Item。
3. 跨应用读取必须使用版本化 HTTP 契约、独立 audience、最小 action 和精确 resource scope。
4. Gateway 删除客户端伪造身份 Header，只从已验证 Credential 派生身份。
5. Workbench 不接收客户端提供的 Subject，不持久化来源工作项，不代发业务写命令。
6. 撤权允许已经签发的短期 Credential 在其有界有效期内保持原能力；重新签发后必须收敛。
7. 只读投影、撤权实验和来源停机不得改变 Production 业务事实。

## 3. 初始失败与根因

首轮正式数据链能建立 People、Project、Assignment 和 Production Work Item，但当前登录人投影为空。

根因不是数据缺失，而是 Production 把 Project Assignment 的 `subject_id` 当作 IAM Subject 直接比较。Project 实际保存的是 People Person ID；IAM Subject 关联由 People 拥有。这个错误把两个不同应用的稳定标识误当成同一标识，表面上少了一次调用，实际形成了隐式语义耦合。

### 被否决的快捷方案

- 把 IAM Subject 冗余写进 Project Assignment：会让 Project 同时承担人员登录映射的生命周期。
- 在 Production 复制 Person→Subject 表：会产生过期副本、同步和双写问题。
- 让 Production 直接查 People 或 IAM 表：破坏数据库所有权和独立迁移。
- 为演示把夹具 Person ID 改成 Subject ID：隐藏缺陷，不能证明真实边界。
- 关闭目标应用授权或把 Subject 交给 Workbench：把服务端安全退化成客户端约定。

## 4. 修正后的边界

修正后的只读组合顺序为：

1. Production 用请求绑定的 Project workload credential 读取 Work Item 所属 Project Assignment；
2. Assignment 返回 Person ID 和记录责任关系时使用的 People source revision；
3. Production 用独立 People workload credential 调用版本化 `PersonPolicyFact`；
4. People 校验 Person、Tenant、状态与 revision，并返回该 Person 当前可信的 IAM Subject 关联；
5. Production 只有在当前 Credential Subject 与该关联一致、Assignment 仍 active、目标动作仍获授权且 Work Item 仍可执行时，才发布 `WorkItemProjection v1`；
6. Workbench 严格校验来源、Tenant 与 Subject 后只读展示。

工作负载权限同步收缩为：

- Project：读取精确 Project／Assignment 事实；
- People：`people-groups.policy-fact.read`，资源类型 `person`；
- Production：不获得 People 写权限、IAM 管理权限或其他应用数据库权限。

这个修正增加一次有界 API 调用，但没有增加新的共享事实所有者。它证明“解耦”不是减少所有调用，而是让每个语义由真正的所有者解释。

## 5. 低资源验证步骤

### 5.1 定向代码验证

在 Mac 上没有启动整套 Docker，只执行单模块串行检查：

```bash
cd backend/services/design-operations
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd ../identity-access
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

jq empty contracts/access/v1/examples/workload-identity-policy.json
sh -n acceptance/d1-c8-production-work-items/verify-running
git diff --check
```

结果：全部通过。

### 5.2 单镜像发布

只构建并推送受影响的 Production 镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/
  omsbizfoundry/production-operations:v0.1.0-test.16
sha256:1eba80b9fa0d78328ee04466971657291555806722db2a8e1f032dada5536f70
platform: linux/amd64
```

其余运行镜像继续使用已固定摘要；People 为 `v0.1.0-test.14`，Gateway 为 `v0.1.0-test.13`。发布仍按 ADR-0035 的兼容闭包组装，不在服务器现场编译。

### 5.3 首次升级失败与安全恢复

第一份 `test.16` 组合的 Compose 模板把 `PEOPLE_BASE_URL` 误放进相邻 Business Engagement 环境块，Production migration 以缺少绝对 People URL 拒绝执行。升级前备份已经产生，命名数据卷未删除；Production 容器未健康，其他服务恢复。

没有采用以下错误恢复方式：

- 不跳过 Production 备份门禁；否则可能遗漏已经存在的 Production 数据；
- 不执行 down migration；
- 不删除命名卷重来；
- 不把部分健康描述为候选已部署。

修正 Compose 后生成不可变 `test.17` release。由于完整升级会要求全部已选服务先健康，先使用同候选的 `d1 apply` 恢复 Production，再重新执行正式 `upgrade`。

### 5.4 正式升级

第二次正式升级执行：

```text
preflight
→ 联合备份
→ 保留命名卷、移除候选项目容器与 one-shot 容器
→ 全部 migration
→ 同一候选重建
→ health/check
```

结果：

- 升级前备份：`/var/backups/omsbizfoundry/20260929T151745Z`；
- 11 个备份对象执行 `sha256sum -c checksums.sha256` 全部为 `OK`；
- Contract、Money、Party、People、Project、Evidence、Business Engagement、Production、IAM migration 均成功；
- 13 个长期服务全部 healthy；
- Production 运行固定的 `v0.1.0-test.16@sha256:1eba80…f70` 镜像；
- `d1 check` 通过。

### 5.5 正式 C8 验收

验收驱动使用正式入口与临时 IAM 账号，按以下七步执行：

1. 建立验收所需的最小 IAM bootstrap 能力；
2. 建立当前 Subject 和相邻 Subject；
3. 经正式 API 建立 `People → Project → Production` 夹具；
4. 验证当前 Subject 恰好看到 1 个工作项，相邻 Subject 得到空集合；
5. 撤销目标可执行 action：旧 Credential 在有效期内仍可见，刷新后为空；恢复 Grant 并刷新后重新可见；
6. 撤销 `work-items.read`：旧 Credential 在有效期内仍返回 200，刷新后为 403；恢复后重新可读；
7. 停止 Production：Workbench 自身保持 200，来源独立返回 502／503；恢复 Production 后投影恢复，业务事实摘要不变。

首次运行在第六步遇到 Gateway 429。分析表明 bootstrap 已产生 79 次授权请求，随后矩阵与同一个固定一分钟 120 次窗口叠加；这是验收准备流量污染被测窗口，不是权限或业务缺陷。验收脚本改为在 bootstrap 完成后等待固定窗口自然翻页，没有重启 Gateway，也没有提高正式限流值。

不可变 `test.18` 验收包使用与 `test.17` 完全相同的运行镜像，仅携带修正后的验收驱动。第二次运行完整通过。

## 6. 直接证据

服务器证据目录：

```text
/opt/omsbizfoundry/releases/
  omsbizfoundry-d1_v0.1.0-test.18_linux_amd64/
  deployments/d1/.runtime/c8-production-work-items-evidence/
```

关键结果：

- 7/7 验收步骤通过；
- `http-evidence.tsv`：47 项，47 项符合预期，0 项偏差；
- 当前 Subject：恰好 1 个 WorkItemProjection；
- 相邻 Subject：0 个；
- 撤销 action 后刷新：0 个；
- 撤销 read 后刷新：HTTP 403；
- Production 停机期间 Workbench：HTTP 200，来源 502／503；
- Production 事实前后摘要一致：
  `d3a66bcc9fd2fe9f9992d19049aa947884e9d3316f2354c7772c7181470f0708`；
- 临时 Grant 已撤销，临时账号在清理阶段停用。

## 7. 对“解耦平衡点”的结论

本切片支持以下结论：

1. Project 不需要复制 IAM Subject，Production 也不需要共享 People 表；版本化 Person Policy Fact 足以闭合真实身份语义。
2. 允许显式依赖不等于耦合失败。Production 的最小交付组合需要 IAM、Project、People；依赖是可声明、可授权、可停机和可替换的。
3. Workbench 保持纯组合层：来源停机时其他页面仍活着，恢复后不需要同步或重建统一待办库。
4. 短期 Credential 的旧权限窗口是已声明的一致性模型；安全收敛点是重新签发，而不是假称瞬时全局撤权。
5. 额外 API hop 在本轮没有产生人类可感知阻塞；但这不是完整性能结论，仍需在连续 C1～C8 旅程中记录 p50／p95 和来源超时。

本切片不证明：全部 C1～C8 已形成同一连续旅程、双 Tenant 权限矩阵完成、公司 D1 空目标恢复完成、所有 C8 来源都已装入，也不证明跨客户 Base 已经成功。

## 8. 下一收缩点

1. 将本切片纳入同一 Suite／Overlay／数据集的 C1～C8 连续旅程；
2. 用现有联合备份在隔离空目标执行恢复与恢复后写入，不破坏当前运行卷；
3. 对 C8 真人浏览器资料下载／分权和 Workbench 个人工作执行人工体验验收；
4. 在完整旅程中增加相邻 Tenant 拒绝和来源超时／性能记录。

