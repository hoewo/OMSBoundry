# 本地参考 OA 业务组合技术方案

## 目标与边界

该组合在不引入公网、Nginx、Gateway、中心 IAM 和 Workbench 的情况下，用真实应用进程和真实 PostgreSQL 验证参考 OA 的 J1～J4。它是业务架构验收环境，不是生产部署档，也不改变完整组合固定的入口链。

## 组合角色与最小性

应用没有永久的“核心／辅助”商业等级。参考 OA 中 Project 承担旅程入口、Party 能力当前嵌在两个台账中、Evidence 承担文件事实，但这些只是该组合中的角色，不改变各应用的数据所有权和契约边界。

最小组合是能够完成选定业务结果的最小应用集合及其显式直接依赖，不是最少进程或零依赖。Suite Manifest 声明所选结果应用和依赖；后续增加应用通过版本化 API、快照或事件扩展组合，不允许跨库读取、共享 ORM 或让新角色改写既有历史。

## 运行组成

| 应用 | 本地地址 | 数据所有权 |
| --- | --- | --- |
| Contract Ledger | `127.0.0.1:18081` | 合同、revision、签约主体和状态 |
| Money Flow Ledger | `127.0.0.1:18083` | 义务、实际资金、核销和余额 |
| People & Groups | `127.0.0.1:18084` | Person、Group、Membership 和定义 |
| Evidence Service | `127.0.0.1:18085` | File、Version、上传会话和本地字节 |
| Project Workspace | `127.0.0.1:18086` | Project、Assignment、Resource Link 和历史 |
| Design Operations | `127.0.0.1:18087` | design-v1 Work Item、Stage、Deliverable 和历史 |

每个应用使用独立 PostgreSQL service、database、账号和 volume。应用共享的 `app` 网络只承载版本化 HTTP 调用，不允许跨库查询、跨库外键或 ORM 复用。所有宿主机端口只绑定 `127.0.0.1`。

## 鉴权与秘密

- Contract 与 Money 使用各自正式支持的本地账号、会话和 CSRF；其他四个应用使用版本化 IAM 契约的模板 Provider。
- reference-read 使用独立随机 Bearer credential 和固定 tenant，不复用浏览器密码。
- `ops/local-oa init|up` 生成本机专用 `.env`，权限由 `umask 077` 控制；该文件、续跑状态和数据库卷不进入 Git。
- 不存在 `AUTH_DISABLED`、万能管理员或为了演示直接改库的路径。

## 可重复业务旅程

`acceptance/reference-oa-demo` 是组合验收驱动器，只调用应用公开 HTTP／表单入口：

1. J1 创建 People／Group／Membership、Project 和两类 Assignment，并完成立项。
2. J2 在 Contract 中建立并激活合同；在 Money 中由操作者显式建立并确认应收义务、登记收款和 Allocation。
3. J3 通过 Evidence 上传并完成 design-v1 四阶段。
4. J4 把 Contract、Obligation、Transaction、Evidence 和 Design 五个类型化引用挂到 Project，并在线读取各事实所有者的状态。

固定 Command ID 验证应用幂等；本地忽略的 `.runtime/demo-state.json` 只保存中断续跑所需 ID，不保存业务事实替代品。`reset --confirm-destroy-data` 才会删除演示数据。

## 可裁剪子集装配

### 设计交付子集

`composition-audit` 使用同一份 Compose service 定义和同一批应用镜像建立隔离项目 `omsbizfoundry-design-delivery-audit`。它先启动 People、Evidence、Project、Design 各自的 PostgreSQL，再以 `--no-deps` 只启动这四个应用；Contract、Money 及其数据库不得出现在运行 service 集合中。

子集沿用 J1 的人员／组／Project／Assignment 和 J3 的 Evidence／Design 全历史，只省略 J2 财务旅程，并在 J4 仅建立 Evidence 与 Design 两个稳定引用。Project 因而固定到 `active / revision 6`，Design 固定到 `completed / revision 18`。同一驱动器连续执行两次，业务 ID、revision 和 `.runtime/design-delivery-state.json` 摘要必须不变。

子集端口为 `127.0.0.1:19084`～`19087`，Compose project、数据库卷、Evidence 对象卷和续跑状态均与全量参考 OA 隔离。命令退出时只销毁该隔离项目的容器和卷；失败时先输出所选应用日志再清理。该机制验证运行时可选装和公开契约，不改变 Project 对可配置上游端点的认知，也不把 Contract／Money 的缺席解释为它们不属于全量参考 OA。

### 商业台账子集

`composition-audit-commercial` 使用隔离项目 `omsbizfoundry-commercial-ledger-audit`，只启动 Contract、Money 及其两个 PostgreSQL；People、Evidence、Project、Design 及其数据库不得出现在运行 service 集合中。Contract 和 Money 继续使用各自正式支持的本地账号、会话与 CSRF，驱动器只通过公开页面／HTTP 入口执行 J2，不直接写库。

该子集在 `127.0.0.1:19081` 和 `127.0.0.1:19083` 上完成正式合同、应收义务、实际收款、核销和完整纠错历史。连续运行两次时，Contract 必须保持 `closed / revision 4`，Money Obligation 必须保持 `closed / revision 2`，全部业务 ID 和 `.runtime/commercial-ledger-state.json` 摘要必须不变。

两个子集使用不同 Compose project、宿主机端口、数据库卷和续跑状态，且都在退出时清理自身资源。它们从相反方向证明同一应用镜像与数据边界支持中等粒度选装。源码工作区会构建所选应用；发布包模式只使用已装载镜像和静态驱动器，强制 `--no-build --pull never`，并在异构构建机上让驱动器通过所选 Compose 网络访问服务。两者仍属于合成验证，不代替第二客户或真实目标 Ubuntu 的实际装配验收。

## 故障行为

`failure-isolation` 依次停止 Contract、Money、People、Evidence 和 Design，再从 Project 读取聚合结果：Project 自身必须始终保持 `active`，两个 Assignment 快照必须继续可读，只有依赖已停止事实所有者的实时引用明确为 `unavailable`。People 停机不影响已冻结 Assignment 快照；每一轮结束后脚本恢复对应服务并等待健康检查。

新 Link 的写入仍失败关闭：目标引用无法在线验证时不得记录虚假成功。既有 Link 的读取降级与新写入的失败关闭是两个不同规则。

## 迁移到公司 Ubuntu

本地通过后迁移的是同一组预构建应用镜像、migration、API 契约和业务数据边界，不在目标机重新解释业务模型或重新编译源码。可移植演示包包含六个应用镜像、PostgreSQL 镜像、静态验收驱动器、Compose 和两层 SHA-256 清单；目标机以 `--no-build --pull never` 启动，缺少镜像时失败关闭。

演示包不带 `.env`、数据库卷、Evidence 字节、续跑状态、证书或私钥。目标 Ubuntu 重新生成本机凭据并从空数据库开始；个人试验田的数据和秘密不参与迁移。

公司完整入口环境验收另行增加：

- 目标 Registry 的不可变镜像摘要和目标环境秘密；
- Nginx 唯一公网 80／443 入口；
- Gateway 路由、Header 清理、限流和基础验签；
- 中心 IAM 登录、Session 和各应用 audience credential；
- Workbench 入口体验、TLS、备份恢复与目标机端口边界。

本地 `.env`、模板 token、账号密码和演示数据库不得复制到公司环境。公司环境未获明确授权前，不执行部署写入。

可移植包只证明六应用业务组合可以用同一制品换机复现，不把模板 IAM 宣称为正式 IAM，也不把宿主机内网演示宣称为公网或生产部署。

目标机在镜像装载前运行发布包内的只读 `preflight`。该门禁同时检查 Ubuntu 22.04+、宿主机与发布目标平台一致、当前用户的 Docker daemon／Compose v2 可用性、运行工具、8 GiB 可用磁盘、3 GiB 内存加 Swap、三层验收端口空闲，以及默认九个 Compose project 没有既有资源。预检不得安装包、启动／删除容器、生成秘密或配置入口；失败只报告全部已发现条件并退出，因此公司授权前仍不会产生目标环境写入。

当前高保真开发包为 `v0.1.0-demo.11`，目标平台为 `linux/amd64`。该包从独立归档解压后通过包外／包内摘要、九镜像离线装载、新秘密初始化、六个独立产品的 6/6 fresh-state／run／replay／cleanup、设计交付与商业台账两类子集的精确选择集和稳定重放、六应用与六数据库启动、两次同 ID 的 J1～J4、五来源故障隔离、最终健康检查和九个验收 Compose project 的零残留检查。七个运行镜像用于正式业务组合，另两个公开契约桩镜像只服务 Project／Design 的 Level A 独立边界，不进入子集或完整 OA。验收宿主为 ARM Mac，七个静态驱动通过已装载的 Linux/amd64 应用镜像运行；因此该证据覆盖制品自包含和跨架构构建机执行，不覆盖真实公司 Ubuntu 的宿主权限、磁盘、网络和 Docker 版本。

`v0.1.0-demo.11` 的元数据为 development provenance，`commit=unknown`。它是可运行的 Demo 候选，不是正式可追溯发布；正式交接使用干净 Git HEAD 重新构建并重复相同验收。

## 操作与验收入口

```sh
./deployments/local-oa/ops/local-oa up
./deployments/demos/ops/level-a audit --confirm-destroy-demo-data
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa demo
./deployments/local-oa/ops/local-oa failure-isolation
./deployments/local-oa/ops/local-oa verify
./deployments/local-oa/ops/local-oa composition-audit
./deployments/local-oa/ops/local-oa composition-audit-commercial
```

发布包入口为：

```sh
make local-oa-release VERSION=v0.1.0-demo.11 OUTPUT_DIR=dist
deployments/local-oa/ops/verify-release-bundle \
  dist/omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz \
  dist/omsbizfoundry-local-oa_v0.1.0-demo.11_linux_amd64.tar.gz.sha256
```

运行说明见 `deployments/local-oa/README.md`，Ubuntu 交接说明见 `docs/operations/local-oa-ubuntu-demo.md`。这些命令通过只证明本地业务组合和目标构建隔离，不代表完整入口架构、真实公司 Ubuntu 授权环境或公网生产验收通过。
