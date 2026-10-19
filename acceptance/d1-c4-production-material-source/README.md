# C4 Production 资料来源隔离验收

生产成果不能由项目资料读取 → 先验证 Production 当前 File ID／Version 来源端、原生 IAM 能力与零读取写入。范围见 [资料引用 v1](../../docs/specs/assembly/production-material-reference-v1.md)。不代表 Project 消费者、Nginx 公网、真实浏览器或完整 C8 通过。

主代理先按低负载 `GOMAXPROCS=2 GOWORK=off GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -p 1 -tags=integration -c` 分别编译 Production 的 `internal/design` 和 `internal/httpserver`；命名为 `production-material-domain.test`／`production-material-http.test`。将二者、`probe.sh`、本驱动和既有原生 Production Capability Manifest 放到 ECS 的固定验收目录，再执行 `sudo python3 verify-ecs.py`。入口是一次性、明确名称，不支持覆盖已有结果或盲目重试。

驱动只新建独立 internal Docker bridge、PG18 与有界测试容器；两者合计 CPU 上限 0.55，内存上限 416 MiB，无公网端口，不访问 IMDS。真实 IAM8 签发单 action／精确 Work Item 或 Tenant 凭证，Production 正式 verifier 验证之；来源 bootstrap 采用合成 Project／Assignment／Evidence 引用，读取阶段这些端口禁止调用。每个测试自己的临时 Production DB 由既有测试 helper 负责创建和清理；IAM 专用 DB、容器、独立 PG 数据卷、配置与诊断保留。

检查两个类型的当前替代版本、错误 Tenant、全部 owned-table canonical 摘要；在 Item 与 Stages 读取之间提交一次真实替代命令，读到同一旧快照，下一请求再读到新快照。HTTP 验证匿名、邻资源、缺 native action、精确资源、Tenant 和错误 Tenant verifier。所有测试必须实际 PASS、无 SKIP，主站 13 DB 摘要／17 容器 ID 不变且健康，restore20 不启动；finally 停止全部新测试容器，不删除卷。

不会改主站镜像、环境、IAM 授权、业务表、对象或浏览器数据；密钥、token、密码和 URL 不进入公开报告。验收不是升级脚本。
