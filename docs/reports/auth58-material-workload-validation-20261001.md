# Auth58 C4～C6 资料真实 Workload 读取链（2026-10-01）

结论：隔离环境中，Project 自有服务身份经真实 IAM8 换票，Results、Cost／Outsourcing、Production 正式目标验签，再组合真实 Evidence7 元数据的只读链通过。主站没有升级；整体 OA 仍为 `insufficient-evidence`。

## 实际通过的范围

- b 先验证三类 Results／Cost／Engagement Link，Project revision 4；c 再加入 Production Work Item，四类 Link、Project revision 5。Project 创建与 Link 均走真实受保护 HTTP。
- IAM 签发、Project Workload Key／签名请求、目标各自 verifier、五种单动作精确资源范围均真实执行。c 的 10 条服务换票审计只有五种原生范围：Result read、Cost read、Engagement read、Work Item reference.read 和单 File metadata.read；没有金额、下载、写或 Tenant-wide 票据。
- 各来源拥有自己的 PG18 事实，只发布 File ID／Version／来源 revision／digest；Evidence 元数据独立读取。匿名 401、邻 Project 403、错 Project Link 422，失败零写入。
- Results 或 Production 进程暂停时，只有该来源分段 unavailable，其他分段正常；已成功 Link 在来源离线时仍本地重放。Evidence 暂停时四段 unavailable，恢复后四段 available。Project 与 Evidence 全表 canonical 始终不变；两个来源测试进程退出时分别核对五张 Results、十张 Cost／Engagement、六张 Production owned 表保持。
- 真正的进程读取检查无 SKIP。新过程夹具的普通 Go／vet、Linux AMD64 编译、Python 编译、Shell 语法及 D1 image publish self-test 通过；本地无真库环境的 integration 编译／SKIP 不算真库验收。
- 总限额 0.55 CPU／512 MiB、internal 网络、零公开端口；b／c 串行运行，各自使用新的独立 IAM／Project／Evidence 库。全部探针与私有 PG 已停止、无 OOM；主站 13 库摘要／17 容器 ID 在运行前、运行后及停止后相同，17/17 healthy，restore20 未启动。没有删除容器或卷。

## 明确的夹具与未覆盖事项

Results／Cost／Production 种子由各自 Service／Store 产生合法事实，但写入前的 Project／Contract／Party／Assignment／Evidence 引用是明确合成提供者；读取阶段零上游调用。Source HTTP 是单独编译的测试进程，使用正式生产 handler／IAM verifier，不是正式完整启动配置或真实来源写业务旅程。Evidence File Version 2 是独立库 SQL 元数据夹具与合成 Local 对象，不是上传、完整性校验或下载验收。

没有 Nginx／Gateway、公网／浏览器、OSS、联合多库／文件恢复、Main 兼容闭包切换或原连续 C8。不能用本报告关闭这些门槛，也不能将已有单库恢复外推为全 Suite 恢复。

## 证据与失败记录

入口：[专项说明](../../acceptance/d1-c5-c6-material-workload/README.md)，C4 模式显式 `MATERIAL_INCLUDE_PRODUCTION=yes`。ECS 根目录 `/opt/omsbizfoundry/acceptance/material-workload-20261001/`；b／c 私有目录保留日志、只读结果、脚本、密钥和数据卷。仅不含凭证的结果／二进制摘要／权限审计复制离机，并核对 SHA-256。

| 证据 | b：C5／C6 | c：加入 C4 |
| --- | --- | --- |
| results | `2c0da93f93dc3632680328688ae46fd00594c57ecb0580787a78e84aa09105ff` | `43ad92de1764d12b31506e9213b9e49b02ae0491f548704f270e840e7267ae4b` |
| binaries | `86e274f7a2f400b77796a1c69e3c1e2aeca1cdb475ac0fe97b480e3fe0db853d` | `8c5ff9add4773abe6afc7fbad940c1a92980a8fe1ca4820958adb2d6717830a1` |
| 原生权限 audit | `260d69c220862e0630b9529cf99ee7828b26dc1de0ce14130c697c8a16ef2439` | `f3dd18d48ebc9528d084fac93d1dda45e853262fb51bfeaa968b78a442b52f9a` |

首次 a 在启动检查中因脚本误用 Python HTTP 方法失败，清理时缺失来源日志又掩盖原错误；已停止 runner，私有 PG 独立停止后再用新 b 库重试。修正方法、尚未进入执行的来源环境变量和真实 204 Grant 契约，不修改业务或放宽授权。a 未改写为通过。c 固定自己私有目录的启动脚本，避免后续修改影响运行探针。

后续四个应用候选镜像发布、逐字节提取核对和实际空库启动已另行通过，见[制品报告](./auth58-material-candidate50-validation-20261001.md)。两份证据边界保持独立；尚未交付 Main，联合恢复和正式入口仍待验。
