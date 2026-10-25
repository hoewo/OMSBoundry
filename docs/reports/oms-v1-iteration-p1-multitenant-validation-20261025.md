# OMS1 迭代 P1 多租户与成员身份验证报告

日期：2026-10-25
分支：`codex/company-internal-management`
需求基线：`abe170fc38b1c5aca67d5f0389da9e8ed406ff9a`
结论：P1 本地验收通过，可以进入 P2；未部署 D1 或公网。

问题 A → 可验证结果 B：既有服务普遍携带 Tenant 字段，但 IAM、工作负载缓存和 Subject↔Person 仍可能被单一部署 Tenant 绑定，单元测试也无法证明恢复后隔离继续成立 → 在保留各 Owner 数据所有权的前提下建立真实多租户身份链，并以两个真实服务进程、两个租户、跨租户负例和双库空目标恢复证明边界。

## 1. 实现结果

- IAM 登录名全局唯一，登录后会话确定唯一 Tenant；登录请求不接受 Tenant 参数，首版没有租户选择或切换。
- 新增 `tenantctl create`，以单事务建立 Tenant、首个管理员、密码 verifier 和 `platform_operator` 审计；密码通过临时文件输入，不进入参数或输出。
- Subject 增加 revision；IAM 提供受工作负载 Capability 保护的 `GET /api/v1/references/subjects/{id}`，按凭证 Tenant 精确读取。
- People Person 保存 `iam_subject_id`、来源状态和 revision 快照；新建／换绑通过正式 IAM 端口复验同租户引用，并以数据库唯一约束禁止一个 Subject 同时绑定两个 Person。
- People 旧 schema 中已有但未经 IAM 证明的 UUID 显式迁移为 `legacy_unverified`，不伪装成有效账号；迁移结束后历史不可变触发器恢复。
- IAM 能力凭证验证器只信任签名中的合法 Tenant；12 个受保护 Owner 继续用本地查询／命令中的 Tenant 条件作最终隔离，不再要求它等于部署 bootstrap Tenant。
- 工作负载 Credential Source 增加请求 Tenant 版本，缓存键包含 Tenant；所有生产引用适配器按当前可信请求 Tenant 换票，避免 A 租户缓存被 B 租户复用。
- Identity／People D1 候选已挂载新 migration、Manifest、People workload key 和 `tenantctl`，但本报告未执行发布。

## 2. 真实双进程与恢复验收

入口：

```sh
./acceptance/p1-multitenant/run-isolated
```

2026-10-25 实际执行结果：`PASS`，耗时约 46 秒。该入口：

1. 创建随机命名的一次性 `postgres:18.6` 容器和四个空数据库；
2. 从当前源码构建 IAM Server、IAM Keyctl、Tenantctl、People Server 和 People workload Keyctl；
3. 启动同一个 IAM 进程，建立 Tenant A bootstrap 管理员并用 `tenantctl` 建立 Tenant B 与首管理员；
4. 两个管理员分别登录、取得 People Capability，并经同一 People 进程建立各自 Person；
5. 证明双方列表只含本租户记录，互猜 Person ID 为 `404`，A 尝试绑定 B Subject 为 `422` 且无写入；
6. 撤销 A 的 Person 写能力后，新凭证只读且写请求为 `403`；停用 B 操作员后其 IAM Session 立即为 `401`；
7. 停止两个进程，分别执行 IAM／People custom-format `pg_dump`，恢复到两个新空数据库；IAM dump 排除 Session 数据；
8. 用原签名密钥和工作负载密钥启动恢复目标，重新登录并确认 Tenant、Person ID、revision、Subject ID／状态快照不变；
9. 恢复后给 A 恢复写 Grant，成功追加第二个 Person，同时 B 仍只有原有一条记录。

验收进程只操作随机容器和系统临时目录；结束后无 `oms-p1-*` 容器残留，也未触碰 `oms-v1-integration-*`、D1 或开发数据库。

## 3. Owner 绑定审计与回归

当前受保护 Owner／应用均完成 IAM verifier 审计：Approval & Audit Results、Billing、Bonus、Business Engagement、Contract、Cost & Outsourcing、Design／Production、Evidence、Money、Party、People、Project。验证器接受任一合法签名 Tenant，并把 Tenant 交给 Owner；资源读取、列表和命令继续由 Owner 的 Tenant 条件与资源 Scope 最终授权。

本轮实际执行 14 个受影响 Go module 的 `go test ./...`，全部通过。另在一次性 PostgreSQL 18.6 中为每个 Owner 建立独立空数据库并实际运行代表性 integration tests，覆盖：

- Project／Business Engagement／Money 的 Tenant 过滤、搜索、排序和稳定分页；
- Contract 的文字检索、稳定 keyset 分页与 Evidence revision；
- Evidence 的真实本地文件 HTTP 生命周期；
- Party、Billing、Approval Results、Cost、Bonus、Design／Production、People 的跨租户读取或写入隔离、历史和不可变性；
- Project 的完整 PostgreSQL 纵向切片及主体建议 Tenant scope。

这些既有 Owner 测试不共享业务表；每个模块在独立数据库运行。普通测试还覆盖其他导出／附件授权路径。本轮没有把 Owner 的配置 Tenant 删除：它继续作为 bootstrap／模板默认值和工作负载无请求上下文时的兼容默认值，但不再作为 IAM 正式运行时的租户白名单。

## 4. 迁移、构建与秘密边界

实际通过：

- IAM 与 People 全新 PostgreSQL 18 migration；
- IAM `000001 → 000006` 主生命周期与 People `000001 → 000002` 旧 Subject 关联升级；
- migration 重复执行和 current schema gate；
- Identity／People Docker image 构建，Identity image 含可执行 `tenantctl`；
- D1 Compose 解析和 Capability Manifest 目录加载；
- `git diff --check`；
- 工作区 `ak_` 加 64 位十六进制秘密模式扫描为 0 个文件命中。

源码中的固定密码字符串只属于无外部账户的验收 fixture；运行时生成的签名私钥、工作负载私钥和数据库 dump 只存在于测试临时目录／一次性容器，未写入 Git。Feedback 项目 Key 未用于本阶段。

## 5. 未外推范围

- 本报告没有修改或发布 `oatest.antuinfo.cn`，也没有宣称新 D1 组合已上线。
- P2 的 Internal Finance Requests／Payroll Owner 尚未开始，P3～P9 仍未完成。
- P1 只建立身份、租户、Subject↔Person 与现有 Owner 兼容基础；不代表账号管理页面、工资、财务申请、开放 API、发票闭环或 Feedback 已完成。
- 真实公网 HTTPS、全组合连续旅程、联合备份恢复和浏览器证据继续属于 P9 门禁。
