# Auth58 Results 报告资料来源（2026-10-01）

结论：C5 的只读资料来源切片通过源码、真 PostgreSQL 18 和真实 IAM8 签发／Results verifier 检查。主站未更新；Project／Contract／Evidence 的写入前引用是显式测试提供者，不能解释为完整跨进程 C5／C8 通过。

## 最小改动

单个 Result 发布 `/api/v1/references/approval-audit-results/{id}/materials`，只从自有单行 snapshot 读取 File ID／冻结 Version／slot、Project、kind、状态和 revision。排序后的 items JSON 确定 SHA-256；不带金额、原金额摘要、对象地址、下载 URL 或权限凭证。空为明确数组，撤回保留引用并明确状态。沿用原生 `approval-audit-result.read` 和目标 Result／Project／Tenant Scope，不附带 amount.read；没有新服务、migration、写命令或 Completion Reference 改写。

## 实际检查

- 全包 Go 测试／vet、领域与 HTTP race 通过；本地无 TEST_DATABASE_URL 的 integration 只能编译／SKIP，不算真库通过。
- 纯投影验证两 kind、空／撤回、同 slot 多文件、确定排序／摘要、不变原 snapshot、金额变化不影响资料摘要及 18 类畸形自有事实；HTTP 验证三个 Scope、匿名、缺 read、相邻资源及畸形 Store 返回。
- ECS `TestResultsMaterialPostgresCurrentCorrectionWithdrawalAndZeroWrites` PASS、无 SKIP：更正为 v2，v1 历史保留，撤回保留 v2；每次读的五张 owned table canonical 不变，读取时 upstream resolver 不可调用。
- ECS `TestResultsMaterialHTTPPostgresNativeIAM` PASS、无 SKIP：真实 IAM8 签发单原生能力与精确 Scope，Results 正式 verifier 接受 Result／Project／Tenant，拒绝匿名、相邻 Result、只有 amount.read 和错 Tenant；GET 前后五表 canonical 相等，响应 no-store 且无金额。
- 仅启动已标记的私有 PG 与一个测试／IAM runner，internal 网络、零公开端口，总限额 0.55 CPU／416 MiB。主站 13 个数据库摘要、17 个容器 ID 前后及停止后不变，restore20 未启动；结束全部停止，容器、卷、IAM DB／密钥和诊断保留。

## 首轮失败与证据

第一次 `...20261001a` 的真库生命周期测试通过，但 HTTP 种子夹具只给金额结构填了一个分量，违反既有三分量要求，尚未进行 IAM 矩阵就失败。修复夹具为完整三分量，未放宽应用不变量；a 的失败记录保留。第二次 `...20261001b` 使用新的私有 IAM 角色／数据库，完整通过且不覆盖 a。

入口见 [专项说明](../../acceptance/d1-c5-results-material-source/README.md)。ECS 原始目录 `/opt/omsbizfoundry/acceptance/results-material-source-20261001/results_material_source_auth58_20261001b/`，公开 results／binaries 有离机副本。SHA-256：results `bf48b998cb58be1d38afd1323835deb675c3749035942d29c5d62127fc3cdbec`，binaries `b8260ae6a77ac24dbd735ecb8bb8c7ccb6530efc1dc4248df98bb7d0dd5859b5`，test.log `8ad73ebc0268ba0a136455daf42257e96aab1be5730fc7bd903d3b77992bd953`。

## 仍未关闭

Project canonical Results Link／消费者、Workload Policy、正式兼容发布、联合空目标恢复、原连续 C8、真实浏览器和 C6 来源均不能由本来源切片代替。Main Evidence 仍 Local，Standard Web OSS 尚未分离。整体维持 `insufficient-evidence`。
