# Auth58 Evidence 原生入口能力验证

日期：2026-10-01；决策：BF-082。

结论：源码、固定镜像的隔离真实 IAM→Gateway→Evidence 权限矩阵及 Workbench 离线兼容启动取得直接证据。**主站未部署此入口变更，连续 C8 和正式 HTTPS／浏览器分权仍待验；完整 OA 仍为 insufficient-evidence。**

## 直接结果

Manifest 兼容旧单项或互斥的有限 ANY 集合；Gateway 仍复验签名、身份、时间和 Scope，原 JWT 不扩权，Evidence 最终判断操作和文件范围。

| 真实 IAM 合成账号的精确 File Credential（无 view） | 元数据 | 下载授权及原字节 |
| --- | --- | --- |
| metadata.read | 200 | 403 |
| download | 403 | 200，字节 SHA-256 一致 |
| metadata.read + download | 200 | 200，字节 SHA-256 一致 |

三个账号均由真实 IAM 邀请／激活／登录并签发，不是业务替身。相邻 File Scope 访问原文件的元数据和下载均 403。撤销 download 后旧短期 Credential 的残余窗口仍有效；重新交换返回 403 并清除旧 Cookie。无 Cookie 的 API POST 返回 401，不是 GET 页面重定向 303。

## 环境与边界

- 固定 Gateway test.49：`sha256:4daf8e516bf9868bcceeb7ca214e1fea95ff15b45d276fd9ca7aec7fda65c534`。
- 固定 Workbench test.49：`sha256:bbb8612a52fa8bccc022b8baaa25a62a640b89d4fab056f7c8b76f6ba03e0d1e`。
- IAM／Evidence 使用主站既有 test.8／test.7 二进制；合成 IAM／Evidence 各自独立受限数据库，不复用主站账号或文件。
- 私网三进程容器限制 0.35 CPU／224 MiB／无宿主端口；Workbench 限制 0.15 CPU／96 MiB／network=none。两者串行运行后停止。
- Workbench 实际非 root 镜像读取当前主 Manifest 的隔离副本，仅改 Evidence 入口字段，启动 ready、匿名登录页通过；不是已认证导航或浏览器证据。原主 Manifest／Workbench ID 未变。
- 主站 13 库全部 public 表的 canonical 摘要、17 个长期容器 ID 前后相等，17/17 healthy，Evidence 仍 Local。restore20 保持停止，未重启 Docker／主机或修改 DNS、RAM、Bucket、OSS 对象。
- 兼容闭包为 Gateway／Workbench、Manifest schema／验收／渲染和 C8 驱动；没有领域表、migration、共享数据或新服务。

## 测试与证据

Gateway、Workbench、access-contract 普通 Go 测试／vet、Gateway race、实际 D1 renderer→strict reader、发布包、应用静态边界审计和负例自测已运行通过。C8 离线测试覆盖三能力集合、额外 view、错误 action、Tenant／audience／File／Cookie 名，4 个方法、9 个用例通过。改后的完整 C8 尚未运行。

入口：`acceptance/evidence-oss-ecs/verify-entry-capabilities.py`、`entry-capability-probe.sh`、`verify-workbench-entry-candidate.py`。它们是 Auth58 合成验收器，不是通用升级器。

ECS 私有目录：`/opt/omsbizfoundry/acceptance/evidence-oss-transition-20261001/entry_capability_auth58_20261001c`。三账号矩阵保存在 `results-resume2.json`，最终撤权拒绝／主站冻结／停止后检保存在 `results-final.json`；后者先严格核对前者和保留夹具，不重建文件或账号。

- 三账号矩阵 SHA-256：`d7d1502b51fd0d0b114c9f3346bdf6c413d4a969005e9350a33c82fe198eb5fe`。
- 最终后检 SHA-256：`be2b234aaa31fd4644515af2990b514b8e915042ef491b057061c7dc656bae4a`。
- 主站原冻结 SHA-256：`484711a762f5ed9284b228fe71cab656cb8ed7abce9d09202400fcc78a1c4657`。
- Workbench 匿名页 SHA-256：`16178e6ec3b1015144df6f55f85a406c08350185aa2f25820f9615cefd2bdd8b`。

## 失败与未验事项

失败未改写成绿色：预检 Docker JSON／长期服务计数错误未创建夹具；a 的 Cookie 名断言失败；b 的 30 秒 TTL 被 Evidence 最小 1 分钟门禁拒绝，后续脚本又误把 403 错误对象当成下载地址；c 首轮用错 disposition，首次恢复用错版本 GET 路径，第二次恢复末尾把未认证 POST 错期望为 303。均先停止并保留诊断、按真实契约修正；最终仅恢复准确冻结的部分夹具完成遗漏后检。a／b／c 均停止且没有 OOM；合成库、文件和配置保留，未删除。

正式交付仍须 BF-077 联合备份、兼容 Gateway／Workbench 与 Manifest 升级和公网正反例，再在授权的原连续数据集运行 C8；不擅自重启 restore20。C4～C6 active Link／资料投影、Files 浏览器、OSS 主 Provider 切换及标准 Web OSS 是独立门槛。
