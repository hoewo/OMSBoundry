# ECS Evidence OSS 适配与恢复验收

问题 A：控制台角色已创建，但不能证明已部署的 Evidence 镜像能使用实例身份完成文件生命周期。结果 B：使用现有镜像、正式 IAM 凭证和独立数据库，串行验证真实 OSS 字节、完成确认、下载和空库恢复。

这不是公网 Evidence 路由或真人浏览器验收。登录和凭证签发经过公网 Nginx → Gateway → IAM；控制请求访问隔离容器的私网地址，上传脚本替代浏览器直接调用 OSS。现有主栈、Local 对象与业务引用不切换。

## 运行要求

在目标 ECS 以 root 运行 Python 3 标准库脚本；已有 Docker、PostgreSQL 18 和部署后的 Evidence／IAM 镜像。当前操作者参数针对 Auth58 D1；默认 PostgreSQL 管理角色为 `oms_admin`，应用角色为 `evidence_service`。脚本不创建 RAM 用户或长期 AccessKey，不输出 Token、密码和签名 URL。

```sh
export PROBE_NAME=evidence_oss_auth58_20261001a
export PROBE_DATABASE=evidence_oss_auth58_20261001a
export PROBE_NETWORK=evidence_oss_auth58_20261001a
export PROBE_PUBLIC_URL=https://oatest.antuinfo.cn
export PROBE_BUCKET=oatest-auth58-files-1
export PROBE_REGION=cn-hangzhou
export PROBE_UPLOAD_ROLE_ARN=acs:ram::1169734041157847:role/auth58evidenceuploadrole
export PROBE_ECS_ROLE_NAME=Auth58EvidenceServiceRole
python3 acceptance/evidence-oss-ecs/verify.py

export PROBE_RESTORE_DATABASE=evidence_oss_auth58_20261001r
python3 acceptance/evidence-oss-ecs/verify-recovery.py
```

也可以通过 SSH 将脚本送到远端 `sudo env ... python3 -` 的标准输入；不要把 Docker inspect 的完整输出或密钥写入日志。数据库和容器名必须使用 `evidence_oss_` 保留前缀，首轮拒绝已有数据库。失败后先检查保留状态，不删除或重建主业务库。

## 验证范围

- 一个新容器，CPU 上限 0.5 核、内存上限 192 MiB，没有发布宿主端口；结束时停止。
- 64 KiB 单对象和 21 MiB 三段串行 multipart；900 秒、精确单对象上传 STS。
- 未认证 Evidence 请求 401；越对象写入、上传凭证读取和匿名对象读取 403；禁止覆盖时再次 PUT 为 409。
- 服务端走杭州 internal Endpoint 复核字节／SHA-256；complete 重放只有一份 Version 和一条完成审计。
- 短期签名下载、206 Range、SSE-OSS AES256；仅删除 URL 签名后的匿名拒绝不是到期拒绝证据。
- 冷备份恢复到另一空库，完整 `file_versions` canonical JSON 保持，再以正式 IAM 获取地址并复读原 OSS 对象。

OSS ZIP 只有数据库与 Provider 标识，不包含对象字节。空库恢复后对象仍存在的验证不能证明对象被误删后的恢复能力。备份需要另存到 ECS 外并核对摘要。

## 元数据边界

默认 D1 `app`／`data` 内部网络不能访问 ECS IMDS。探针单独使用带 `oms.purpose=evidence-oss-acceptance` 标签的出口 bridge；不要将其他应用加入它，也不要把整个 `app` 网络改成可出网。

实例角色属于 ECS 而非某个进程，因此有出口的 Nginx／Gateway 也可能访问 IMDS。主机侧定向保护脚本为 [`evidence-imds-guard`](../../deployments/d1/ops/evidence-imds-guard)：

```sh
EVIDENCE_IMDS_NETWORK=evidence_oss_auth58_20261001a \
  sh deployments/d1/ops/evidence-imds-guard apply
```

脚本只拦截 Docker `DOCKER-USER` 链中目标为 `100.100.100.200/32` 的流量，允许显式 Evidence bridge；不改主机 OUTPUT、SSH、公网入口或其他目的地址。重复 apply 必须保持相同规则；遇到非专用网络或陌生同名规则拒绝修改。`remove` 仅删除该脚本拥有的规则。主机重启／网络替换后的自动恢复尚未验收，不能声称已经持久化。

完成过恢复、但下载检查失败时，可在 canonical facts、镜像与目标 DB 一致的前提下重验：

```sh
PROBE_REUSE_RESTORED=yes python3 acceptance/evidence-oss-ecs/verify-recovery.py
```

该模式不向已有库再次执行 restore，不生成另一份 ZIP；它只重验已经恢复的记录和实际下载。GET 签名请求不得添加控制面专用的 JSON `Content-Type`。

实测结果及正式切换门禁见 [Auth58 云端验证报告](../../docs/reports/auth58-evidence-oss-adapter-validation-20261001.md)。

## Local／OSS 兼容与混合恢复

`verify-transition.py` 使用新的隔离源／目标库，从已保留 Local 冷备份恢复；固定摘要候选须支持 `hybrid`。串行检查原 Local 字节、CNAME STS／SDK、Local-primary 回退和 v2 混合空目标恢复。成功留下 12 个 Version、11 个 Local 字节及 1 个 OSS 引用；主 Suite 不停止或切换。`PROBE_IMAGE` 必须使用 `@sha256:`，源／目标名通过 `PROBE_TRANSITION_SOURCE`／`PROBE_TRANSITION_TARGET` 设置并拒绝已存在库。冷备份参数 `PROBE_LOCAL_BACKUP` 只能指向明确保留的 `/var/backups/omsbizfoundry/` 文件。

`verify-backup-tests.py` 在目标 PostgreSQL 18 和一个限额容器中运行交叉编译的 `systembackup.test`，真实执行 Local v1／mixed v2 冷备份集成测试；数据库夹具由测试创建与回收，不修改主库。不是所有 Evidence 集成测试均已执行的证据。

`inspect-read-response.py` 只读检查隔离 OSS 会话的响应长度／编码和摘要，不输出凭据、签名 URL 或文件内容。它用于定位文本 GET 压缩导致的 complete 500，不能当作业务上传验收。

## 精确合成对象离机恢复与真实到期

`verify-object-recovery.py` 只接受保留的 `m7`／`m8` 隔离夹具和固定合成内容；拒绝任意业务对象。`export` 将对象字节保存为不可覆盖的 0600 文件。离机复制并复核摘要后，`restore` 要求 `RECOVERY_ALLOW_REMOVE_ISOLATED=yes`，再次核对原对象与返回字节，然后仅删除这份合成对象、观察缺失、恢复并复读；保持所有不可变 Version 和主库不变。它不是生产备份工具。`expiry` 使用实际 900 秒 STS 等待到期，不注入时钟；到期前／刷新后用已存在对象的禁止覆盖 409 证明身份可用，到期后要求 403 `SecurityTokenExpired`，不增加业务写入。

`prepare-profile-candidate.py` 从主栈实际镜像生成固定摘要列表，不读取或输出密码；只替换显式候选 Evidence。`verify-profile-candidate.py` 在 ECS 克隆 0600 配置并只渲染提取后的 D1 包，不 rollout、不更改 DNS 或 RAM。Guard boot 测试入口为 [`test-evidence-imds-boot`](../../deployments/d1/ops/test-evidence-imds-boot)，只在新的 Linux 网络命名空间操作 iptables，不能代替主机／Docker 重启验收。

`prepare-browser-login.py` 仅用于用户已授权的 Auth58 IAM 登录：生成 0700 目录／0600 临时助手，通过浏览器复制已有账号，不将密码写到日志、URL 或浏览器密码管理器。登录后关闭助手、删除精确临时文件，并用非敏感文本覆盖剪贴板。

`rollout-project-feedback.py` 是 Auth58 显式固定的 `test.43／46／47／48` Project 页面更新操作证据，不是通用升级器：精确核对现有 17 个镜像和 Project 环境，先冷备份并在空库逐表核对，再只重建 Project、不执行 migration。只允许未改变的预检暂存恢复执行；已有 dump 或更新证据时拒绝重跑，失败不覆盖数据库。私有配置与备份留在受保护目录，浏览器结果另验。

`verify-project-picker-browser.py before/after/frozen` 仅后检明确合成夹具的负责人选择与立项，拒绝任何已关联 IAM Subject 的人员；不发命令或改 People。分别冻结 revision 3 与 6，精确核对命令、Assignment、历史增量及其他 Project／People 表不变。这些历史阶段不是任意当前数据验收器。

`verify-project-feedback-delivery.py` 经正式 HTTPS／IAM／Gateway 只读取得页面、外部 JS 和保留的 revision 2 验收项目，并单独注销探针会话；不执行项目命令，不代替浏览器。`verify-project-ingress-negative.py` 只测试无签名／伪造 Header 拒绝、匿名透镜不渲染及重建后 Project 的 IMDS 隔离；不冒充相邻角色、撤权或完整租户矩阵。

`verify-systemd-template.py` 使用临时 unit／非敏感环境，将模板路径绑定到已存在的候选 Guard，仅调用 systemd 的解析检查，不安装、启动或重启 unit，不执行 boot 或操作 iptables。全局主机其他 unit 的警告与夹具诊断分开计数；本检查不能作为实际启动持久化证据。

`stop-restore20.py` 仅在用户明确批准后，通过 `ALLOW_STOP_RESTORE20=yes` 停止精确演练 project 的 17 个长期服务，保留容器、卷、配置和备份，不操作主站；操作前快照与已有终态禁止覆盖，部分完成后不得直接重跑。`verify-restore20-stopped.py` 独立核对停止／保留状态、主站健康和启动时间；只把私有 ID／挂载写入 0600 证据文件。`--mount-order-diagnostic` 只读比较挂载集合与重启策略。恢复启动须重新安排依赖顺序与健康等待，不使用 `down -v` 或全 Suite 重建。

`python3 acceptance/evidence-oss-ecs/test-stop-restore20.py` 不调用 Docker／SSH；验证 8 项精确停止与失败关闭规则，不代替实际云端后检。

## 原生能力入口隔离验收（BF-082）

`verify-entry-capabilities.py`／`entry-capability-probe.sh` 固定 IAM、Evidence 和候选 Gateway 二进制，在一个 0.35 CPU／224 MiB 私网容器内运行真实三进程及独立库。合成文件、账号和诊断保留，结束必停；不作用于主 IAM Grant、业务文件或 restore20。名称限定 a／b／c，新库优先，恢复只接受精确已知部分事实，已有结果拒绝覆盖。它不接入 Nginx／真人浏览器，也不切换 OSS Provider。

`verify-workbench-entry-candidate.py` 用当前主 D1 清单副本在 network=none、0.15 CPU／96 MiB、非 root 的固定 Workbench 镜像中验证 strict reader 和匿名登录页；不接数据库、IAM 凭据或云身份。运行证据与保留失败见[专项报告](../../docs/reports/auth58-evidence-native-entry-validation-20261001.md)。两份脚本都不是主站升级器；正式交付必须另走 BF-077。
