# Auth58 C4～C6 资料候选制品验证（2026-10-01）

结论：四个应用的 `test.50` 不可变候选镜像已发布至 ACR，摘要拉取、12 个执行文件逐字节核对及四个实际应用启动检查通过。候选 D1 包已生成，但尚未切换主站；整体 OA 仍为 `insufficient-evidence`。

## 已实际通过

- 本机只用 Go 1.26.8 交叉编译 Linux AMD64、CGO 关闭的 server／ops／workload-keyctl；串行、`-p 1`，不启动本地 Docker。ECS 缺少 Buildx，使用固定 PG18 基础镜像和 `Dockerfile.verified-artifact` 有界组装；这不是默认 BuildKit 发布，也没有 BuildKit provenance 或 clean-HEAD 正式发布证明。
- 发布前确认四个 Registry tag 不存在，未覆盖 tag。逐个 push、digest pull、核对 Linux AMD64／非 root postgres／实际启动命令，并从不运行、network-none 的镜像提取容器核对 12 个文件 SHA-256 与编译制品一致。
- 每次只启动一个实际 `/app/server` 和既有私有 PG18：本应用独立空库 migration、ready、实际 `/app/ops check`、自己的 keyctl、资料端口匿名 401 和全表 GET canonical 不变均通过。结束逐个停止，最后停止 PG，全部无 OOM。
- 主站 13 库 canonical／17 容器 ID 在发布与启动前后及停止后保持，17/17 healthy；restore20 未启动，没有删除容器或卷。

| 候选应用 | 镜像 SHA-256 | 实际启动 schema |
| --- | --- | --- |
| Project Workspace | `3aed62e43cb49e06f4baa8cd08e18f6922a1a277bef1733ae2c3155088255336` | `000009_results_cost_material_resource_links.sql` |
| Production Operations | `e377f2e59708531ecd089b261c734cde25a6d19bfa262796a1553d2f471ee928` | `000002_production_type_definitions.sql` |
| Approval & Audit Results | `0212773891e63c2708b05ca180e6b9f8907f805590c19fc52cbaa93830eba5a0` | `000001_approval_audit_results.sql` |
| Cost & Outsourcing | `eedf3bb3783effc3af790969080d07c615a74aaf6722fe43fc8b84e2ae19d9fd` | `0002_payable_suggestions.sql` |

镜像前缀为 `crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/`，候选 tag 为 `v0.1.0-test.50`；部署使用上表 digest，不使用可变 tag。

## 证据范围与限制

镜像实际启动时，相邻服务 URL 明确指向不可用测试端点，IAM verifier 采用既有专项 JWKS；没有来源写入或 Workload 换票。因此这个检查证明可执行镜像及空库启动，不是完整最小业务组合、正式 D1 Profile 运行或真实业务验收。[真实 Workload 读取链](./auth58-material-workload-validation-20261001.md)是另一份范围明确的证据，其中来源写种子前的相邻引用和 Evidence 元数据仍为夹具，不能拼成完整正式写旅程。

候选 D1 包为开发验证制品，`RELEASE-METADATA` 的 commit 明确为 `unknown`。包含 17 个 digest-pinned 镜像：替换本次四个业务候选与 BF-082 的 Gateway／Workbench49，其余镜像采用当前主站摘要；Evidence 保留 Local7，不混入 OSS41 或切换 Primary。包内权限、策略、Manifest 渲染器及配置端点属于兼容集合，不能仅替换四个业务镜像。

候选包的内外 checksum、17 个镜像引用、全选配置与 14 个 Manifest 应用（IAM／Workbench＋12 个业务应用）、Project Production／Cost 端点、Evidence entry_actions 和隐藏导航通过。IAM8 在 network-none、只读、96 MiB／0.15 CPU 容器解析 12 份 Capability Manifest、3 份 Assignment Policy 和 Workload Policy，exit 0／无 OOM；全套配置只渲染，未启动任何 Suite 服务。主站 13 库／17 ID 不变。该解析不会加载运行时 Workload 公钥，也不证明部署、换票或联合恢复。

没有全 Suite 运行、主站 migration、联合多库／文件恢复、公网／浏览器、原连续 C8、标准 Web OSS 或正式 OSS 切换的通过结论。

## 主站切换前的兼容门禁

| 集合／操作 | 当前状态 | 必须保持 |
| --- | --- | --- |
| 四个业务镜像与各自 migration | 制品／实际空库启动通过，Main 未升级 | Project 当前 Main schema 7；候选到 9。来源事实、ID、revision 不变 |
| IAM8＋12 个 Capability Manifest＋Assignment／Workload Policy | 候选包无数据库解析通过；精确换票已有隔离证据 | 原生 action 与精确目标资源，不能补授 broad view 或金额／下载／写 |
| Gateway49＋Workbench49＋Suite Manifest／渲染／验收 | BF-082 隔离门禁通过，Main 未部署 | Evidence entry_actions 只是 ANY 粗入口，目标应用最终授权 |
| Project Production／Cost 端点配置 | 17 镜像全选配置渲染通过；没有 Suite 运行 | 从 Deployment Profile 配置注入；未选应用不成为启动前提 |
| 联合备份→离机→空目标恢复→继续写入 | 本次组合待完成 | 13 库、文件字节、密钥、策略、引用共同恢复，不用单库证据代替 |
| 正式入口与真人操作／完整 C8 | 待完成 | 能实际提交、复读、撤权、失败恢复，不能用数据探针替代 UX |

已执行 schema 9 后，不得将 Project48 换回旧镜像冒充回滚；恢复路径须遵循 BF-077／ADR-0035 的兼容闭包与空目标恢复。这里没有授权重启主机或 Docker，也没有启动 restore20。

## 归档与失败

ECS 专项根目录 `/opt/omsbizfoundry/acceptance/material-candidate50-20261001/`，入口与 recipe 见[专项说明](../../acceptance/d1-c5-c6-material-workload/README.md)。不含凭证的三份结果已离机并逐一核对摘要：

| 文件 | SHA-256 |
| --- | --- |
| `results-publication50.json` | `47f648114329e369cf40f2b136ecacebccc494531752c429f0ae7af00217bbed` |
| `results-startup50.json` | `2b0c80e57b61c62313f892624f6827168e2afb3f6efab42290aff878475e5633` |
| `artifacts.json` | `befedeffa604084f03d60b5b72bf1fa8b8bc89ec0931348164a38c449b2788b7` |
| `results-bundle-preflight50.json` | `4f82a35dbcceb39584e3cf9c06136e7a6d7fcbde7dd703f38607e9ed21c0118a` |
| 源码上下文 archive | `be3580d4561bbe7cf725b38f55d9b9c41f867411fdfdb9b5a4aecab8b546e46e` |
| 编译制品 archive | `0b864269fb9d74483316e3b929f3ff39e4b730e29e15877543ec2ed69e6882d6` |
| 候选 D1 archive | `c1839328f01a5d068e78acb7352cbd2c6721684432fda327786a4cc1ba910b72` |

首次 publication 在 Docker29 省略空 Entrypoint 字段时脚本失败，发生在 push 前；改为安全读取可选字段，用新的 publication-b 目录重试成功，首次目录与日志保留。只改变探针，不改变业务或放宽验收。

最终只读后检：四个候选 server、IAM parser 容器与私有 PG 均 stopped／exit 0／无 OOM，运行列表仅有 17 个 healthy 主站容器；主机可用内存约 822 MiB。本轮没有额外常驻栈，本机 Docker 未启动。按用户四小时内暂停要求，在这批制品与预检闭合后留停检查；Main 切换仍待下一批。
