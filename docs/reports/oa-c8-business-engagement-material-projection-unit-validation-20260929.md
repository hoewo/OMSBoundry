# OA C8 Business Engagement 间接资料投影单元验证

日期：2026-09-29  
判定：`c8-indirect-business-material-unit-supported / runtime-and-d1-pending`

## 本批问题 A → 结果 B

- A：Project 只能组合自己直接连接的 Evidence，尚不能证明业务记录中的资料可在不共享表、不复制领域模型的前提下进入项目资料视图。
- B：Business Engagement 发布版本化的当前 Evidence 引用投影；Project 先读取该投影，再以自身独立的 Evidence metadata 凭证逐文件组合，并保留两层 revision、digest、来源关系和局部失败。

## 已实现边界

- Business Engagement 新增 `GET /api/v1/references/business-engagements/{engagement_id}/materials`，只返回当前未被取代的 Evidence 引用、业务来源 revision 和确定性 digest。
- 端点使用独立的 `business-engagement.material-reference.read`，并由目标应用复验精确 Engagement／Project 范围。
- Project Resource Link 新增 `business-engagement/business-engagement` 类型；Project 不读 Business Engagement 表，也不保存其材料列表。
- Project workload 先读取业务来源投影，再用 `evidence-service.metadata.read + file:{id}` 读取每个文件；两次授权互不替代。
- 返回分段分别保留 Business Engagement linked/current revision、source digest，以及 Evidence linked/current revision、version 和 SHA-256。
- 错 Tenant、错 Project、错 source、非法 digest、业务来源拒绝、单个 Evidence 不可用和 revision drift 均失败关闭或仅降级对应分段。
- Project 响应模型没有 Bucket、Object Key、永久 URL 或长期存储凭证字段。

## 串行验证

以下命令按顺序执行，未并发运行测试包：

```text
cd backend/services/business-engagement
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd backend/services/project-workspace
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...

cd acceptance/access-contract
GOMAXPROCS=2 GOWORK=off nice -n 10 go test -p 1 ./...
```

三组均通过。覆盖当前引用投影排序／digest、专属 IAM action、相邻 Scope 拒绝、严格来源校验、直接与间接资料同时存在、业务来源失败隔离、同一业务来源内单个 Evidence 失败隔离、Evidence revision drift 和跨 Project 拒绝。

本批未启动 Docker、PostgreSQL、浏览器或完整 D1，没有增加持续运行进程。

## 尚未证明

- 新的 `000007` migration 尚未在真实 PostgreSQL 执行；
- Business Engagement → Project → Evidence 尚未用真实 IAM、三个独立进程和正式入口运行；
- Project 页面尚未通过真实浏览器点击间接资料源、下载分权和撤权恢复；
- 正式 D1 的 Suite Manifest、未选缺席、备份／空目标恢复和真实 Ubuntu 仍待验；
- 指标投影与 `WorkItemProjection` 尚未开始。

因此本批只关闭间接业务资料的单元／契约层不确定性，不把 C8 或 OA 完整体标记为完成；总进度仍为 98%。
