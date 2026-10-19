# 第 15 轮：单台 ECS 是最低档，不是架构上限

日期：2026-09-20  
状态：已进入开发决策总表；部署档为目标设计，尚未实现

## 本轮纠正的问题

此前文档反复把 Complete Suite 0.1 描述为“单台 ECS 组合”，容易被理解为产品只能部署在一台机器上。真正约束是：资金有限的小公司至少可以用一台 ECS 完成安装和使用，同时同一套产品必须支持把入口、应用和数据拆到两台或多台机器。

## 核心决策

部署能力分成两个互不混淆的配置维度：

- **Suite Manifest**：决定安装和启用哪些应用；
- **Deployment Profile**：决定这些应用放在哪些计算节点、运行几个副本、使用哪种数据库与文件存储。

所有部署档使用相同的应用镜像、API、IAM 契约、migration 和数据所有权。升级档位只能改变拓扑与容量，不能通过复制代码、共享数据库表或改变业务语义实现。

## 四个部署档

### D1 Compact：1 台 ECS 最低档

```text
ECS-1（公网）
  Nginx
  Docker: Gateway / Workbench / IAM / People / Project / Contract / Money / Evidence
  PostgreSQL：一个实例、每应用独立数据库与账号
  Evidence：本地持久目录或 OSS
```

目标是最低成本可用，不宣称高可用。只有 Nginx 开放 80／443，其他端口不对公网开放。

### D2 Data Split：2 台 ECS 数据分离档

```text
ECS-1（公网应用节点）
  Nginx
  Docker: Gateway / Workbench / IAM / People / Project / Contract / Money / Evidence

ECS-2（仅私网数据节点）
  PostgreSQL：每应用独立数据库与账号

Evidence bytes：推荐 OSS
```

它解决应用与数据库争抢资源、数据库直接暴露和数据盘管理混杂，不等于高可用。数据库安全组只允许应用节点访问必要端口。

### D3 Role Split：3 台 ECS 职责分离档

```text
ECS-1（公网边缘节点）
  Nginx
  Docker: Gateway / Workbench

ECS-2（仅私网应用节点）
  Docker: IAM / People / Project / Contract / Money / Evidence

ECS-3（仅私网数据节点）
  PostgreSQL：每应用独立数据库与账号

Evidence bytes：OSS
```

它进一步隔离公网入口、业务计算和数据，但每层仍只有一个节点，因此仍不宣称高可用。

### D4 HA：高可用多节点档

```text
ALB / CLB 或等价冗余入口
  → 至少 2 个跨可用区边缘／应用节点
      Nginx + Gateway + 可水平扩展的应用副本
  → RDS PostgreSQL 高可用版或另行验证的 PostgreSQL HA 集群
  → OSS
```

真正的自动故障切换必须有冗余入口、至少两个健康应用节点和高可用数据层。只增加 ECS 数量但仍让流量进入单个 Nginx，不能称为高可用。不采用云 SLB 时可以设计主动／备用入口，但在完成自动漂移、健康探测和故障演练前只能算灾备，不能冒充 HA。

## 不变量

- 业务应用不得通过 `localhost`、固定 IP 或机器名绑定部署档；所有端点由配置注入；
- 跨节点流量只走 VPC 私网，安全组按入口、应用、数据角色最小放行；
- 只有边缘入口拥有公网业务端口；数据库和内部服务不绑定公网；
- 多副本应用不得依赖容器本地 Session、临时文件或单机内存作为唯一真相；
- IAM 签名密钥、Session、幂等命令和业务事实在副本间保持一致；
- Evidence 多节点档使用 OSS 等共享对象存储，本地容器目录不能作为跨副本真相；
- 一个 PostgreSQL 实例可以承载多个应用数据库，但数据库、账号、migration、备份与恢复边界继续独立；
- 部署档升级必须有备份、迁移、健康检查、切流和回退步骤，不能靠手工复制运行目录。

## 与单应用独立性的关系

部署档描述 Complete Suite 如何组合，不改变小应用边界。Contract 等应用仍可以独立发布；需要 IAM 契约和文件能力时，把相应 Provider／Evidence Service 放入交付包。应用不能因为 D1 同机就读取其他数据库，也不能因为 D3 分机就引入新的业务语义。

## 仍然开放

- 各档的 CPU、内存、磁盘、带宽和并发推荐值，需要容量测试后确定；
- D4 选择 ALB、CLB 还是其他入口，以及 TLS 在负载均衡还是 Nginx 终止；
- 哪些应用首先支持多副本及其 Session、密钥轮换和后台任务单实例约束；
- PostgreSQL 使用 RDS 还是自建 HA 的成本与运维责任；
- D1 → D2 → D3 → D4 的自动化迁移和回退脚本。

具体节点矩阵见[部署档规格](../specs/assembly/deployment-profiles.md)。
