# OA C8 首个后端切片低资源单元验证

日期：2026-09-29  
结论：`c8-first-backend-slice-unit-supported / runtime-assembly-pending`

## 本批目标

在不启动 Docker、数据库或完整 OA 组合的条件下，完成 C8 的第一条最小纵向切片：把 Evidence 文件元数据读取与短期下载授权分权，并让 Project Workspace 只按自身 active direct Evidence Resource Link 组合项目资料。

## 已实现

- Evidence 新增 `evidence-service.metadata.read` 与 `evidence-service.download`；
- metadata 端点接受 metadata.read 或旧 view，Access Grant 接受 download 或旧 view，新能力之间不互相放大；
- 旧 `evidence-service.view` 仅保留迁移兼容，现有消费者无需同批重写；
- Evidence reference 增加 schema、source、File Version、状态、SHA-256、大小和媒体类型，不暴露 Bucket、Object Key 或长期 URL；
- Project 新增 `project-workspace.materials.read` 和 `GET /api/v1/projects/{project_id}/materials`；
- 组合只读取 active direct `evidence-service/evidence-file` Link，保持输入顺序，并保留 Link／关系键、冻结与当前 revision、digest、source status、观测时间和耗时；
- 分段区分 `available`、`revision_changed`、`invalid`、`forbidden`、`unavailable`，无 direct Link 时明确返回 `empty`；
- Project workload 只获得精确 `evidence-service.metadata.read:file` 权限，不获得 download。

## 验证结果

所有命令均串行，使用 `GOMAXPROCS=2` 与 `go test -p 1`：

- Evidence module 全量非 integration 测试通过；
- Project Workspace module 全量非 integration 测试通过；
- Access Contract 的发布示例、Project workload 精确权限和 Manifest 负例定向测试通过；
- Docker 容器数在本批前后均为 0。

负例覆盖 metadata-only／download-only、legacy view、相邻 File、相邻 Project、错 Tenant、缺少 Project action、stale revision、上游 403、上游不可用和空集合。Project 的 ResourceReader 仍是版本化 HTTP 端口，未读取 Evidence 数据库或复用其 ORM。

## 后续独立收口的既有测试问题

Access Contract 全量测试中的 `TestBonusWorkloadPolicyHasOnlyExactSourceReferences` 仍只期望 Bonus 的 4 个来源引用权限，而当前策略已经包含 C7 的 3 个 payable-suggestion 权限。C8 只修改 Project workload 的 Evidence metadata 权限，没有改 Bonus 条目；因此本批保留该失败证据，等待独立 Gap 更新测试语义。

后续独立批次已将断言改为同时精确核对 4 个来源引用权限和 3 个 payable-suggestion 权限；`GOMAXPROCS=2 GOWORK=off go test -p 1 ./...` 在 `acceptance/access-contract` 全量通过。该修改没有改变发布策略，也没有扩大任何工作负载权限；复验前后 Docker 容器数均为 0。

## 尚未证明

- 真实 IAM service credential 换票和签名验收；
- PostgreSQL 中真实 Project／Resource Link 数据；
- Project 与 Evidence 独立进程间的停机、恢复和一次成功重试；
- 人类建立／浏览资料的页面、独立下载按钮和撤权刷新；
- 业务应用间接 Evidence 引用、WorkItem、类型化统计、正式 D1、联合恢复、双 Tenant 或真实 Ubuntu。

下一批最多启动一个临时 PostgreSQL 和 Project／Evidence 最小进程，完成后立即停止并确认容器数回到 0；不会同时启动完整 Nginx、Gateway、IAM 与全部业务应用。
