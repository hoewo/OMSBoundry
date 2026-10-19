# Auth58 C5／C6 资料真实 Workload 读取链

A→B：消费者只有测试凭证端口证据 → 真实 Project 独立 Workload Key 向 IAM8 换票，Results／Cost／Evidence 正式目标 verifier 执行原生单动作／精确资源授权。

仅私有 PG18＋一个有界 runner，内部运行 IAM8、候选 Project、既有 Evidence7，以及分别编译的 Results／Cost HTTP 测试进程。Source seed 经自己的 Service／Store 生成合法 snapshot，创建前 Project／Contract／Party／Evidence 引用明确使用合成测试提供者；GET 使用正式 IAM verifier，不调用上游。没有跨应用源码导入、共享业务表或畸形 SQL source snapshot。Evidence 元数据使用已知文件 fixture，不是上传／下载业务验收。

主检查包括真实受保护 Project 创建和三类 Link，真 Workload 票据／目标读／Evidence 元数据两层组合，相邻 Project／匿名／错 Project Link 拒绝，来源暂停局部分段失败、成功命令本地重放及恢复，Evidence 暂停后各段失败／恢复，Project 与 Evidence 全表 canonical 不变，两个 Source owned 表只读不变，以及 IAM audit 精确四种 audience／action／资源范围、无金额／下载权限。

总限额 0.55 CPU／512 MiB、零公开端口、internal 网络；只复用带明确用途标签且已停止的私有 PG，不启动旧 source runner 或 restore20。结束停止 runner／PG、恢复被暂挂的进程，保留容器、卷、独立角色／数据库、密钥和诊断。正式主站 13 库摘要／17 容器 ID 必须前后及停止后相同。

首次 ROOT／NAME 已存在则拒绝覆盖，重试须换专项名字／角色 DB 并保留失败。执行前上传当前四个 Linux AMD64 二进制、四个 Capability Manifest、Workload Policy、probe 和 driver；IAM8 与 Evidence7 从已验证本地云端制品只读复制并记录摘要。

这个检查不包含 Nginx／Gateway、公网／浏览器、实际来源写业务流程、联合多库恢复、OSS 或原连续 C8。未运行不得描述为通过。

补充 C4 时显式设置 `MATERIAL_INCLUDE_PRODUCTION=yes`；另用 c 专项目录／角色 DB，不覆盖已完成的 b。增加一个 Production HTTP 进程、单 Work Item read 和第四个 Link，Project 类型同时声明 design／consultation；总限额仍为 0.55 CPU／512 MiB。Production 在自己的 Service／Store 写种子，正式读取后六张 owned 表保持；来源暂停只影响自己的分段，Evidence 暂停影响全部四段。审计须精确五种原生 action／resource，不增加 Production write 或 Project-only 替代 Scope。

b／c 实际通过记录见[报告](../../docs/reports/auth58-material-workload-validation-20261001.md)。首次 a 的 HTTP 脚本错误及不完整启动保留；脚本修正清理缺失日志、环境变量与 204 契约后用新库重试，未将失败记录改写为通过。c 将启动脚本固定到自己的私有目录，避免后续编辑影响正在运行的探针。

候选制品步骤的 A→B：通过的源码尚没有不可变部署镜像 → 固定 source archive／Go toolchain／Linux AMD64 三个二进制的摘要，ECS 使用 `Dockerfile.verified-artifact` 串行组装并推送四个应用候选。ECS 缺 Buildx，不能把替代步骤声称为默认 BuildKit／provenance 发布；本机只交叉编译，不启动 Docker。基础 PG18 镜像固定 digest，保留非 root、server／ops／workload-keyctl 与同一 migration／API 边界。Registry tag 不覆盖，先检查缺席，发布后 digest 拉取、架构／UID／命令及三个文件摘要逐项相等。此步不改 Main、IAM 运行策略或数据库，不等于兼容闭包／升级／恢复验收。

制品通过后的 A→B：只提取文件尚未证明镜像可运行 → 每次只启动一个实际 `/app/server`＋既有私有 PG，在独立空库完成 migration、ready、`/app/ops check` 与资料端口匿名 401、全表零 GET 增量。正式 IAM verifier 使用既有专项 JWKS，每应用以自己的 keyctl 生成独立 Workload Key；相邻 URL 显式配置为不可用测试端点，此步不发生换票或来源写入，也不冒充完整最小业务组合／D1 Profile／联合恢复。限额 0.4 CPU／352 MiB、internal 网络／零公开端口，逐个停止并最终停止 PG；Main 不变。任何未实际运行的检查仍为待验。

四个候选 `test.50` 已发布，12 个文件 digest 核对及四次实际启动通过，完整边界见[制品报告](../../docs/reports/auth58-material-candidate50-validation-20261001.md)。`prepare-artifacts.py` 固定 toolchain／source／文件摘要，`publish-artifacts-ecs.py` 不覆盖 tag，`verify-image-startup-ecs.py` 逐个运行实际 server。失败的 publication 目录保留，成功为 publication-b。

候选包预检的 A→B：单镜像可启动不等于配置和契约兼容 → `verify-bundle-preflight-ecs.py` 固定开发 D1 archive，核对包内 checksum／17 个 digest／显式邻接端点／Manifest 原生入口，再在 network-none、只读、96 MiB／0.15 CPU 的 IAM8 容器执行无数据库 `validate-contracts`。只解析公共文档，不加载运行时 Workload 公钥或连接数据库，不启动整套 D1，不执行正式 `d1 upgrade`；synthetic config 留在私有专项，绝不覆盖 Main env／Compose／策略。联合备份、空目标恢复和正式入口仍是单独门禁。

bundle-preflight-a 已实际通过上述检查，IAM exit 0／无 OOM、Main 不变；公开结果已离机并核对摘要，配置和 synthetic 凭据不复制为正式环境。最终后检运行列表只有 17 个 healthy Main 容器，所有专项停止；不把停止的 probe 或 restore20 自动重启。本轮制品／预检在四小时内留停供用户检查。
