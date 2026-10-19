# 本地参考 OA 的 Ubuntu 可移植演示包

日期：2026-09-25  
状态：当前高保真开发包 `v0.1.0-demo.11` 已通过六个独立产品、两类子集与全量 clean-room 验收，并包含只读目标机预检；公司目标机与正式可追溯发布仍待独立验证

## 1. 用途与边界

该发布包用于把六个独立业务产品、两个可裁剪子集和六应用参考 OA 组合带到一台空白 Ubuntu Docker 主机，按“目标机预检 → 单系统 → 子组合 → 完整 OA”的顺序验证业务架构可移植性。目标机只装载预构建镜像，不拉取 Registry，也不编译源码。当前六套 Level A 驱动、高保真 Level B 驱动、两类子集装配、五来源隔离和只读 Ubuntu 预检已进入 `v0.1.0-demo.11`；该开发包于 2026-09-25 从归档解压后通过本手册的 clean-room 验收。

发布包不包含 Nginx、Gateway、中心 IAM、Workbench、TLS、DNS 或公网配置，因此它不是完整 D1，也不是生产上线包。完整入口链仍在获得公司授权、目标域名和秘密后单独验收。

## 2. 包内内容

- `images.tar`：Contract、Money、People、Evidence、Project、Design、PostgreSQL，以及仅供 Project／Design 独立 Demo 使用的两个公开契约桩，共九个同平台镜像；
- `deployments/local-oa/compose.yaml`：六应用、六独立数据库和独立数据卷；
- `bin/*-demo`：六个独立产品驱动器与一个静态 J1～J4 组合驱动器；
- `deployments/demos/`：六套最小 Compose、Level A 聚合入口和能力清单；
- `deployments/local-oa/ops/local-oa`：目标机预检、校验、装载、初始化、启动、演示和故障隔离入口；
- `RELEASE-METADATA`、包内 `RELEASE-CHECKSUMS.sha256` 和包外 `.sha256`。

包内不包含 `.env`、数据库卷、演示状态、证书、私钥、Registry 凭据或应用源码。每台目标机必须重新生成本机秘密和空数据库。

驱动器在同架构 Linux 目标机上直接运行。若在 ARM Mac 等异构构建机执行 clean-room 验收，运维入口会把同一个驱动器临时挂载进已装载的目标架构应用镜像运行；这不会增加发布镜像或正式运行组件。

## 3. 构建端

目标为常见公司 Ubuntu x86_64 时执行：

```sh
make local-oa-release \
  VERSION=v0.1.0-demo.11 \
  OUTPUT_DIR=/absolute/path/to/output
```

正式可追溯发布要求干净 Git HEAD，并显式提供构建时间：

```sh
RELEASE_PROVENANCE_MODE=official \
RELEASE_BUILT_AT=2026-09-25T00:00:00Z \
make local-oa-release VERSION=v0.1.0 OUTPUT_DIR=/absolute/path/to/output
```

开发状态允许生成 `development` 包，但不得把 `commit=unknown` 的包称为正式版本。

## 4. 目标 Ubuntu 前提

- 获得该主机的明确部署授权；
- Ubuntu 22.04、`x86_64`、Docker Engine 和 Docker Compose v2；
- 至少预留足以容纳压缩包、解包镜像、Docker 镜像和六个数据库卷的磁盘空间；
- 演示端口只向批准的内网或 SSH 隧道开放，不直接暴露公网；
- 默认仅使用合成演示数据，不导入公司真实数据。

解包后、装载镜像前先执行发布包自带的只读预检：

```sh
./deployments/local-oa/ops/local-oa preflight
```

预检不安装软件、不启动或删除容器，也不生成秘密。它一次核对：Ubuntu 版本、发布包与宿主机／Docker 的 CPU 平台、当前用户能否访问 Docker daemon、Compose v2、运行所需命令、至少 8 GiB 可用磁盘、至少 3 GiB 内存加 Swap、三层验收使用的本机端口，以及默认九个 Compose project 是否已有容器、网络或卷。只有最终输出 `HOST_PREFLIGHT_STATUS=passed` 才进入镜像装载。

磁盘或内存门槛只能在明确评估目标机容量后，通过 `LOCAL_OA_PREFLIGHT_MIN_FREE_MIB` 或 `LOCAL_OA_PREFLIGHT_MIN_MEMORY_MIB` 提高；不应为了让结果变绿而降低。端口或 Compose project 冲突必须先由主机负责人判断现有负载，预检本身不会清理资源。

完整 D1 宿主机基线仍可复用 `deployments/d1/ops/bootstrap-remote-host check` 做只读检查；对已有公司服务器不得在未确认影响前执行自动 `apply`。业务包预检通过也不等于 Nginx、Gateway、IAM、TLS 或公网条件已经具备。

## 5. 校验与解包

先在归档所在目录验证包外摘要：

```sh
sha256sum -c omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz.sha256
tar -xzf omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz
cd omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64
```

再验证包内每一个文件：

```sh
./deployments/local-oa/ops/local-oa verify-release
```

任一摘要失败时停止，不装载镜像，不尝试修补归档。

当前已验收的开发归档目标为 `linux/amd64`；具体大小和 SHA-256 以与归档同目录的 `.sha256` 文件为准。其 `RELEASE-METADATA` 使用 development provenance 标记基础 Git HEAD 与 dirty 工作树，因此只作为当前工作树的 Demo 候选，不作为正式可追溯发布；正式交接须从干净 Git HEAD 以 `RELEASE_PROVENANCE_MODE=official` 重新构建并复验。

## 6. 装载镜像

```sh
./deployments/local-oa/ops/local-oa load-images
```

`load-images` 装载九个镜像；完整 OA 门禁验证七个运行镜像，Level A 门禁验证包含两个契约桩在内的九个镜像均与发布目标平台一致。发布模式强制 `--no-build --pull never`；缺少镜像时拒绝启动，不会退回现场编译或联网拉取。

## 7. 六个独立产品验收

```sh
./deployments/demos/ops/level-a audit --confirm-destroy-demo-data
```

入口依次运行 Contract、Money、People、Evidence、Project、Design；每套执行 fresh-state、run、replay 和 cleanup。前四套只启动应用与自有 PostgreSQL，Project／Design 只额外启动一个不拥有业务事实的公开 reference-read 契约桩。最终必须输出六项 `passed`、六项能力 `complete` 和 `LEVEL_A_STATUS=passed`，并且不残留对应容器、卷或网络。

## 8. 可裁剪子集验收

先初始化完整组合自己的本机秘密，再运行两个方向相反的选装结果：

```sh
./deployments/local-oa/ops/local-oa init
./deployments/local-oa/ops/local-oa config
./deployments/local-oa/ops/local-oa composition-audit
./deployments/local-oa/ops/local-oa composition-audit-commercial
```

`init` 以 `0600` 权限生成目标机自己的随机数据库密码、管理员密码、模板 token、reference token 和本地 Evidence 签名密钥。不要从个人试验田复制 `.env`。

第一条只启动 People、Evidence、Project、Design 及四个数据库，证明 Contract／Money 缺席仍可完成 J1／J3／J4 子集；第二条只启动 Contract、Money 及两个数据库，证明其余四个应用缺席仍可完成 J2。两条命令都连续运行两次、核对状态摘要，并在退出时删除自己的容器、卷、网络和续跑状态。发布模式只接受已装载的同平台镜像并强制 `--no-build --pull never`。

这两项是合成装配证据，用于证明可选应用真实缺席，不是第二客户或真实公司 Ubuntu 验收。

## 9. 完整 OA 业务演示与故障隔离

```sh
./deployments/local-oa/ops/local-oa up
./deployments/local-oa/ops/local-oa status
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa failure-isolation
./deployments/local-oa/ops/local-oa verify
```

通过条件：

- 第一次完成高保真 J1～J4：People 两次名册快照、Contract／Money／Design 的 revision 与纠错历史、五个 Project 引用；
- 第二次保持相同业务 ID，不制造重复事实；
- Contract、Money、People、Evidence、Design 依次停机时，Project 始终保持 active、两个 Assignment 快照可读，只有依赖停止服务的实时引用明确 unavailable；
- 每个停止项自动恢复 healthy 后才进入下一轮，最终六应用全部 ready。

`v0.1.0-demo.11` 已实际满足上述条件：六个独立产品先通过 6/6，两个子集均以稳定摘要重放；全量两次执行复用相同 Project、Contract、Money、Evidence 与 Design 身份；五个来源逐项停机时 Project 保持 `active / revision 9`，两个 Assignment 快照始终可读；最终六个应用和六个数据库全部 healthy，显式 reset 后九个验收 Compose project 均无容器、卷或网络残留。包内 `local-oa preflight` 的正、反例自测也已通过；公司主机上仍需真实执行它。具体归档 SHA-256 与本次状态摘要以归档同目录的 `.sha256` 和仓库 `docs/testing.md` 为准，避免把外层摘要错误地自嵌入归档。该次验收在 ARM Mac 上通过 Docker 的 Linux/amd64 执行环境完成，证明归档、目标镜像与目标二进制可以脱离源码运行，但不代替真实公司 Ubuntu 主机的权限、磁盘、网络和 Docker 版本验收。

浏览器登录账号只在明确需要时显示：

```sh
./deployments/local-oa/ops/local-oa credentials
```

## 10. 停止和清理

保留数据停止：

```sh
./deployments/local-oa/ops/local-oa down
```

确认不再需要演示数据后删除六个数据库卷、Evidence 文件和续跑状态：

```sh
./deployments/local-oa/ops/local-oa reset --confirm-destroy-data
```

镜像可以在确认无其他演示实例引用后另行删除。归档和摘要是否保留由公司演示责任人决定。

## 11. 后续完整入口验证

业务包验收通过后，才把同版本六个业务镜像并入公司控制的 Registry 和完整 D1：

```text
Browser → Nginx:443 → Gateway → central IAM / Workbench / six business applications
```

该阶段重新生成公司环境秘密，并单独验收 DNS、证书、TLS、Cookie、Header 清理、audience、端口、备份和恢复。业务 Demo 通过不能替代这些入口证据。
