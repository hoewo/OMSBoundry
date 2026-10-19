# Auth58 C4 Production 资料来源验证（2026-10-01）

结论：来源端源码、真实 PostgreSQL 18 与真实 IAM8 签发／目标 verifier 的隔离验收通过；不是主站发布或完整 C4／C8 通过。对应 [资料引用 v1](../specs/assembly/production-material-reference-v1.md)。

## 实际通过

- 新增 canonical GET `/api/v1/references/production-work-items/{id}/materials`，只返回 Production 自己当前 File ID／Version／来源 revision、阶段关系、状态、revision 和确定性摘要；旧标量 reference 响应不变，无新 action、migration 或服务。
- 单元／HTTP、普通 Go 全包、vet 和 design／httpserver race 通过。两注册类型、当前替代、not-applicable／重开／取消、显式空、排序／digest、错范围与畸形事实失败关闭；读取禁止调用未实现的上游端口，意外写入也会使测试失败。
- 隔离 ECS 真 PG18 的 `TestMaterialReferencePostgresCurrentVersionsAndZeroWrites` 实际 PASS：设计和咨询各两版当前替代、重复 GET、错 Tenant 及全部 owned-table canonical 摘要不变。
- `TestMaterialReferencePostgresSnapshotSurvivesConcurrentReplacement` 实际 PASS：在 Item 与 Stage 读取之间提交一次真实成果替代命令；只读 repeatable-read 返回 revision 3／Version 1，下一读取返回 revision 4／Version 2。未把有意的替代写命令称为零写入。
- `TestProductionMaterialHTTPPostgresNativeIAM` 实际 PASS、无 SKIP：真实 IAM8 激活合成账号、签发单 native action／精确 Work Item 或 Tenant Scope，Production 正式 verifier 复验。匿名 401、邻资源／缺 action 403、合法资源／Tenant 200、错误 Tenant verifier 401；六个 owned tables 的 GET 前后摘要一致，no-store。
- 主站 13 DB canonical 摘要和 17 容器 ID 均未改变、全部健康；restore20 未启动。

## 范围与保留

夹具 `production_material_source_auth58_20261001a` 使用独立 internal 网络，无公开端口／IMDS；PG 与 runner 合计 CPU 上限 0.55、内存上限 416 MiB，串行执行。IAM 为已部署同摘要程序，不用测试鉴权替代其签发或目标 verifier；Project／Assignment／Evidence 的 bootstrap 输入是合成引用，读取阶段禁止上游调用，不是已验证全部真实写入连接器。

所有新运行容器已停止，未删容器、网络、PG 数据卷、IAM 专用库或诊断。Production 各测试临时 DB 由既有 helper 正常创建和清理，仅影响新隔离 PostgreSQL。脚本入口和复验约束见 [验收说明](../../acceptance/d1-c4-production-material-source/README.md)。原始记录位于 ECS `/opt/omsbizfoundry/acceptance/production-material-source-20261001/production_material_source_auth58_20261001a/`；公开记录不包含密码、token、签名 URL 或私钥。

## 尚未通过

后续 Project canonical Link／000008／typed consumer／Workload Policy 已完成源码及独立 PG18 migration／HTTP 测试端口／单库空目标恢复，见 [消费者实测](./auth58-project-production-material-consumer-validation-20261001.md)。两层真实跨进程、联合空目标恢复和主站升级仍待验；主站 Production 仍为旧镜像，新端口未正式交付。Nginx 公网／真实浏览器、C5／C6 资料来源、原连续 C8、Web OSS 和正式 Files OSS 切换仍待验。整体保持 `insufficient-evidence`。

来源专项原始记录 SHA-256：results `5ef4f39fd67683737e139da06c136f30639abc0203c5776a0a595d617ff4472e`，binaries `9756a30f44019f37ad32e5ad21a7bf7d011ef368ad99544b0e5d20e4ab2c3886`，test.log `2cd2e36e8a25d644e3e6fcd24897b9eb3ed40c06b39aad3ede69caddb0508e2a`。公开结果及二进制清单已有离机副本，不含运行凭据。
