# D1 Environment Promotion

## 元信息

- 路径: `tech/delivery/d1-environment-promotion.md`
- 技术领域: `delivery`
- 状态: ✅ 已采用
- 负责人: OMSBizFoundry
- 支持功能: `docs/specs/assembly/deployment-profiles.md`
- 影响设计: 不适用
- 关联模型: 不适用
- 关联 API: 不适用
- 决策状态: 已采用，受 `docs/decisions.md` 中 BF-024、BF-027、BF-028 约束
- 调研背景: D1 已在个人 ECS 验证，公网入口受 ICP 阻断，公司环境可能作为并行验证目标
- 变更历史:
  - 2026-09-24: 初始创建
  - 2026-09-27: 明确应用选择集变化时的升级前运行态快照、恢复清单和失败回退边界
  - 2026-09-28: 纳入 Production Operations，第九个应用开关、九份 Capability Manifest 与十四镜像发布单元
  - 2026-09-28: 纳入 Approval & Audit Results，第十个应用开关、十份 Capability Manifest 与十五镜像发布单元

## 方案概述

D1 环境提升采用“不可变制品＋目标环境配置”模型。个人 `xidou.me` 环境是长期保留的试验田；公司控制的阿里云环境是并行验证或正式交付目标。环境之间复制已经验证的镜像内容和版本化部署描述，不复制秘密、运行数据或云资源身份，也不在目标 ECS 从源码重新编译。

## 架构边界

```text
Source Registry digest manifest
          │
          ├─ fixed linux/amd64 pull
          ├─ content-preserving tag/push
          ▼
Target Registry digest manifest
          │
          ├─ pull-only D1 release bundle
          ├─ target-owned configuration and secrets
          ▼
Target ECS: Nginx → Gateway → IAM / enabled applications
```

制品提升不构成环境替换。源环境继续运行并保留自己的 DNS、证书、数据、秘密和备份；目标环境建立独立的云资源与验收证据。

## 发布单元

一次 D1 发布由以下不可分割的身份组成：

- 版本号与 Release Metadata；
- pull-only 部署包及外层 SHA-256；
- 包内逐文件 SHA-256；
- 目标 Registry 中十四个 `name@sha256:digest` 引用；
- Suite Manifest 和九份 IAM Capability Manifest；
- 目标环境独立生成的 `.env`、TLS 材料和秘密；
- 目标环境验收记录。

镜像标签只用于可读版本定位。运行身份以目标 Registry 实际返回的摘要为准。发布包中的 `.release-bundle` 阻止目标机切换到 build 模式。

## 环境所有权

每个环境独占以下资源：

- Registry 凭据、SSH 凭据和云账号权限；
- 域名、DNS 记录、证书与私钥；
- 数据库密码、IAM 签名私钥、Session 与管理员凭据；
- OSS Bucket、RAM 身份、STS 角色与 CORS；
- PostgreSQL 卷、Evidence 字节、业务数据和备份；
- 安全组、ECS 实例和公网 IP。

环境配置不得写入镜像或发布包。个人环境凭据不得作为公司环境的长期依赖。

## 制品提升

十四个源镜像必须已经使用不可变摘要并验证为 `linux/amd64`。提升器按源摘要拉取镜像、比较本地内容身份、赋予目标版本标签、向目标 Registry 推送并读取目标摘要。该过程不运行 Docker build，也不解析业务源码。

Registry 可能对清单表示进行规范化，因此目标摘要不由字符串替换或源摘要猜测。目标摘要清单由推送后的 Registry 响应生成，并成为目标发布包的唯一镜像输入。

## 部署描述

发布包包含 pull 模式运行所需的 Compose、Nginx 模板、数据库初始化脚本、Suite Manifest 渲染器、IAM Capability Manifest、D1 操作入口和外部验收入口。发布包不包含 Docker 构建上下文、应用源码、真实 `.env`、运行态 `.runtime`、证书、私钥、数据库数据或备份。

目标 ECS 只执行摘要镜像拉取、migration、启动、健康检查、备份和恢复。目标 ECS 不承担源码编译或临时镜像修补。

## 公网入口

目标环境维持单一入口：

```text
Browser → Nginx:443 → Gateway → IAM / target application
```

80 端口只执行固定主机名的 HTTPS 跳转。Nginx 是唯一公开宿主端口的容器。Gateway、IAM、业务应用和 PostgreSQL 仅位于内部 Docker 网络，不由安全组或宿主端口直接公开。

## 数据与恢复

并行验证默认从空数据库和合成数据开始。环境提升不复制运行中的 PostgreSQL 目录，也不自动迁移个人试验田数据。

需要迁移业务数据时使用 D1 联合备份，并只恢复到明确的空目标。备份在写入目标前验证 SHA-256。恢复后旧 Session 失效，用户重新登录，并复核身份、授权、业务事实、Evidence 摘要与继续写入能力。

同机备份目录只是暂存位置。有效恢复责任需要 ECS 外的受控加密副本和实际空目标恢复证据。

### 应用选择集变化

候选 Suite Manifest 可能新增或关闭应用，但升级前备份必须描述“当前实际运行的旧组合”，不能用候选 `ENABLE_*` 推断旧组合。D1 在任何 migration、拉取／构建或容器替换前，从当前 Compose project 的已创建服务识别旧选择集，逐一检查并备份其中的 IAM 和业务应用；当前存在但候选关闭的应用仍必须进入升级前备份，候选新增但尚不存在的应用不得被提前检查或伪造备份。

每份联合备份包含 `suite-selection.env`，记录十个应用开关，并把该文件纳入 `checksums.sha256`。候选组合只有在旧选择集备份完成后才允许启动；启动失败时，备份 ID 和旧选择集共同构成显式恢复输入。恢复必须使用与 `suite-selection.env` 相符的旧环境配置和旧发布包，在隔离空目标或灾难恢复环境完成，不把候选新应用缺少备份误判为旧备份不完整。

该机制只保证选择集和数据资产可辨认、可校验、可恢复，不承诺自动原地回滚。生产切换仍需保留旧发布目录、旧环境文件和离机备份，并在恢复后重跑旧选择集健康检查与代表性业务读取。

## Evidence Provider

D1 当前整栈默认使用 Local Provider。真实 OSS 浏览器链具有独立验收证据，但 Local D1 与 OSS Provider 的整栈组合不因两组证据分别存在而自动成立。

目标环境在部署前明确选择 Evidence Provider。选择 Local 时验收本地上传、完成、授权、范围下载和联合恢复；选择 OSS 时还需验证公司 Bucket、RAM／STS、精确 CORS、multipart、服务端完整性、签名读取、匿名拒绝和恢复责任。

## 验收契约

每个目标环境独立证明：

- 发布包内外摘要正确；
- 十四个运行镜像来自目标摘要清单且平台为 `linux/amd64`；
- migration、健康检查与 schema 门禁通过；
- 只有 Nginx 暴露批准的宿主端口；
- HTTP 跳转、HTTPS 证书、TLS 策略和无效 Host 拒绝通过；
- IAM 登录、Secure Session、Gateway 授权和启用应用代表性旅程通过；
- 单应用停止不影响其余应用；
- 联合备份生成、离机复制和空目标恢复通过。

源环境的通过证据不替代目标环境验收。公司验证环境的通过证据也不自动构成生产容量、监控、RPO／RTO 或高可用承诺。

## 回退

运行回退选择上一份发布包和上一份目标摘要清单。已执行 migration 后的回退使用升级前联合备份恢复到新的空数据库，不修改 migration 历史，不重用部分恢复目标，也不在目标机现场修补镜像。

并行验证不默认切换生产 DNS。目标环境失败时，源试验田保持原状态。

## 外部前置条件

公司环境需要明确授权、隔离子域名、目标 ECS、Registry、TLS、安全组、Evidence Provider、数据范围和离机备份责任。缺少这些外部输入时，发布包与制品提升能力仍可验证，但公司公网链保持未验证。

操作步骤与等待期分支由 `docs/operations/d1-environment-promotion.md` 维护。
