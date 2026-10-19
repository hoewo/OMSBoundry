# OMSBizFoundry Deployment Profiles

日期：2026-09-21
状态：Architecture Baseline 0.1 已确认；当前应用组合 D1 已实现并验证，D2～D4 与资源规格尚未实现

2026-10-04 候选补充（BF-103）：显式选择 `reference-oa-bonus-v2` 时，Suite renderer 为 Bonus 声明有限 `capability_namespaces` 和旧／新 read ANY；空 profile 保持旧单 namespace 入口。选装与放置边界不变，不自动授权或修改业务 Scope。该新组合仅完成配置、reader 正负例及本地真实进程验证，尚未取得 D1 Compose 运行／升级／联合恢复或 Main 发布证据；交付需匹配 Gateway、Workbench、Suite schema／renderer、Bonus profile／schema 3 和 IAM 目录，不引用历史 D1 验收冒充此次升级通过。

## 1. 目的

同一套 OMSBizFoundry 产品最低可以运行在一台 ECS，也可以按预算、隔离和可用性目标扩展到两台、三台或高可用多节点。部署档只改变节点放置、副本和基础设施，不改变 API、数据所有权或业务代码。

部署配置由两个独立清单共同决定：

```text
Suite Manifest      = 安装／启用哪些应用
Deployment Profile  = 应用放在哪里、运行几个副本、连接什么数据设施
```

## 2. 部署矩阵

| 角色 | D1 Compact：1 ECS | D2 Data Split：2 ECS | D3 Role Split：3 ECS | D4 HA：多节点 |
| --- | --- | --- | --- | --- |
| 公网入口 | ECS-1：Nginx | ECS-1：Nginx | ECS-1：Nginx | ALB／CLB → 至少两个 Nginx |
| Gateway／Workbench | ECS-1 容器 | ECS-1 容器 | ECS-1 容器 | 至少两个可健康摘除的副本 |
| IAM | ECS-1 容器 | ECS-1 容器 | ECS-2 容器 | 至少两个无本地真相的副本 |
| People／Project | ECS-1 容器 | ECS-1 容器 | ECS-2 容器 | 按容量部署多个副本 |
| Contract／Money | ECS-1 容器 | ECS-1 容器 | ECS-2 容器 | 按容量部署多个副本 |
| Evidence 控制面 | ECS-1 容器 | ECS-1 容器 | ECS-2 容器 | 多副本，文件字节存 OSS |
| PostgreSQL | ECS-1，一个实例内独立数据库 | ECS-2 私网节点 | ECS-3 私网节点 | RDS HA 或另行验收的自建 HA |
| 文件字节 | 本地持久目录或 OSS | 推荐 OSS | OSS | OSS |
| 自动故障切换 | 无 | 无 | 无 | 必须验证 |

## 3. 网络与安全组

### D1

- 公网只开放 Nginx `80/443`；
- SSH 只允许运维来源；
- Gateway、应用、PostgreSQL 和 Evidence 内部端口不绑定公网地址；
- Docker 网络隔离入口、应用和数据访问。

当前 D1 Compose 已把 edge、app、data 划为三张网络，app／data 标记为 internal，并通过自动验收确认只有 Nginx 存在宿主端口映射。生产必须在 ECS 安全组继续只开放 80／443 和受限 SSH，不能因为容器网络已经隔离就放宽宿主防火墙。

### D2

- ECS-1 是唯一公网业务入口；
- ECS-2 不配置业务公网入口；
- PostgreSQL 端口只允许来自 ECS-1 应用安全组；
- 管理端口使用固定运维来源或堡垒机，不临时开放全网。

### D3

- 边缘安全组只开放 `80/443`，只允许到应用安全组的 Gateway 目标端口；
- 应用安全组只接受边缘节点和受控运维来源，只允许访问数据安全组与 OSS；
- 数据安全组只接受应用安全组访问 PostgreSQL；
- 节点间一律使用 VPC 私网地址。

### D4

- 公网只到负载均衡；边缘节点只接受负载均衡健康检查和转发；
- 至少两个节点跨可用区，负载均衡按健康状态摘除失败节点；
- 应用副本不保存唯一的本地 Session、上传文件或后台任务进度；
- 数据库和 OSS 使用明确的高可用、备份与恢复能力。

阿里云官方建议把公网服务和内部数据库放在不同安全域，只开放必要端口，并让内部服务走 VPC 私网；SLB 的价值是把流量分发到多个健康 ECS 并消除单应用节点故障。具体产品配置在实施时以当期官方文档和区域能力为准。

## 4. 放置规则

### 4.1 可以同机，但不能合并所有权

D1 中所有进程同机、PostgreSQL 也可以共用一个实例，但每个应用仍拥有独立数据库、数据库账号、migration、备份清单和恢复验收。禁止因为同机而增加跨库查询或共享数据表。

### 4.2 端点必须可配置

以下地址不能硬编码为 `localhost`：

- Gateway 到 IAM／业务应用；
- 应用到各自数据库；
- 应用到 Evidence Service；
- Evidence 到 OSS；
- 监控和健康检查。

D1 可以把配置值解析到本机或 Docker 服务名，D2 以上解析到 VPC 私网服务地址。应用代码和契约保持相同。

### 4.3 多副本前提

一个服务只有满足下列条件才可以在 D4 标记为多副本：

- Session／Token 状态、幂等命令和业务事实不依赖单实例内存；
- 签名密钥可安全共享和轮换；
- migration 只有一个明确执行者，其他副本只做 schema verify；
- 定时任务具有租约、选主或幂等消费规则；
- 健康检查能区分进程存活与依赖就绪；
- 任意一个副本停止后，已有请求与新请求行为经过演练。

## 5. 档位选择

- 预算最低、可接受维护停机：D1；
- 首先保护数据库资源和网络边界：D2；
- 需要把公网、应用和数据分层管理，但暂不要求自动故障切换：D3；
- 明确要求节点故障不中断业务：D4。

“两台／三台”本身不等于高可用。没有冗余流量入口、至少两个应用副本和高可用数据层时，只能声称资源或安全隔离。

## 6. 每个部署档的共同验收

- 同一 release image 与 Suite Manifest 能在目标档运行，不为档位生成客户专用分支；
- 只有声明的公网入口可访问，内部端口从公网不可达；
- 每个应用只能连接自己的数据库账号和公开服务端点；
- 登录、权限、Contract、Money、Evidence 与备份恢复主线通过；
- 停止任一节点时，实际影响与该档宣称一致；D1～D3 可以中断但不得损坏数据，D4 必须自动切换；
- 从较低档迁移后，ID、revision、摘要、审计和业务引用保持不变；
- 回退使用已验证的配置、镜像和备份，不直接复制运行中的数据库目录。

## 7. 未确定的资源规格

本文件不凭空规定 vCPU、内存、磁盘、带宽、数据库连接数或副本数上限。每档的推荐规格必须由代表性数据量、并发、文件大小、备份窗口和恢复目标测试得出，并记录测试环境与余量。

## 8. 实施参考

当前 D1 实现、限制和恢复证据见 [M7-006 D1 组合交付验收](../delivery/d1-combined-delivery-m7-006.md)，操作步骤见 [D1 Compact 运行手册](../../operations/d1-compact.md)。D2～D4 仍需在各自里程碑使用真实节点和安全组验证，不能从 D1 成功推断为已实现。

- [阿里云 ECS 安全组应用场景](https://www.alibabacloud.com/help/en/ecs/user-guide/security-groups-for-different-use-cases)：公网服务、内部服务和数据库的安全组隔离与最小端口放行；
- [阿里云 CLB 应用场景](https://www.alibabacloud.com/help/en/slb/classic-load-balancer/product-overview/scenarios)：多 ECS 流量分发和单节点故障隔离；
- [阿里云 ALB 服务器组](https://www.alibabacloud.com/help/en/slb/application-load-balancer/create-and-manage-a-server-group)：后端服务器组、健康检查和多后端放置。

这些链接只支撑通用基础设施机制，不替代本项目的 D4 故障演练和区域可用性确认。
