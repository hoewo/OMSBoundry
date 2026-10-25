# OMS1 迭代 P9 D1 装配源码门禁报告

日期：2026-10-26

结论：P9 的 D1 选择、下游身份与静态交付缺口已经补齐并通过低资源源码门禁；本次没有启动容器，也没有执行全组合业务、联合备份／恢复或公网部署，因此 P9 尚未完成。

## 1. 审计发现

P2 已把 `Internal Finance Requests` 与 Payroll 放入 D1 profile、数据库、镜像和备份恢复，但按 P5／P7 的正式闭环复核后发现：

- Finance 的 D1 环境没有 Money／Evidence 端点，也没有独立工作负载身份；进程健康时付款和发票实时复验仍可能未接线；
- Payroll 的 D1 环境有 People 端点但缺 Money 端点，工资发放闭环不能由 D1 配置保证；
- D1 允许在缺少上述 Owner 依赖时选择 Finance／Payroll，错误会延迟到业务操作；
- Finance pull profile 没有覆盖新增 key helper 镜像。
- Finance／Payroll 的 Money 交付绑定虽已进入各自整库 dump，但原 D1 联合备份保持 Nginx 开放并顺序导出各 Owner，跨 Owner 写入可能落在不同检查点；升级预备份后短暂重开入口还可能接收不在回滚点内的新写入。

## 2. 已完成修改

- Finance 新增独立 Ed25519 workload key lifecycle 和短期 IAM Credential Source；正式 IAM 模式拒绝静态 Money／Evidence Token。
- Finance→Evidence 只申请 `evidence-service.metadata.read`＋精确 File ID；Finance→Money 只申请 `money-flow-ledger.internal-finance-payment.record`＋精确 Command ID。
- 下游返回 401 时仅失效该 Tenant／Audience／Action／Resource 缓存，换票后最多重放一次；网络或不确定结果继续返回 unknown，由原幂等命令恢复。
- D1 为 Finance 注入 Money／Evidence／IAM 端点、私钥只读 volume 与健康依赖；Payroll 补上 Money 端点和 People／Money 健康依赖。
- Suite 选择在任何镜像构建、停服务或数据动作前强制 Finance→Money＋Evidence、Payroll→People＋Money。
- build／pull、停用清理、示例环境、workload policy、Makefile tidy 与静态组合门禁同步更新。
- BF-245 运行前复核又修正 `FEEDBACK_SDK_URL` 错注入 Bonus、Workbench 缺失的问题，并把项目 115、首发 `apiKey`、URL、HMAC、通知及单／双 Key 轮换改为 D1 状态变化前的成组失败关闭。
- BF-247 让 D1 联合备份先关闭唯一公网 Nginx、等待 0～30 秒的可配置 drain，再开始首个 dump；普通备份成功或失败恢复原入口，升级成功备份保持停写直到 Compose 替换。一致性模式、入口原状态与 drain 秒数写入受 checksum 保护的 `backup-consistency.env`，旧备份保持可恢复。
- BF-248 在同一停写窗口为 IAM 与实际选装 Owner 生成 `database-fingerprints.tsv`：表集合、行数、列／约束／索引／触发器和逐行事实都进入 SHA-256，但原始值不出 PostgreSQL；空目标恢复在业务服务启动前重算并严格比较。IAM Session 数据按原 dump 契约排除，旧备份无该文件时继续兼容。

数据所有权没有改变：Finance 只保存 Evidence／Money 稳定引用，Payroll 只保存 People／Money 快照与引用；没有共享表、跨库写或 Workbench 业务事实。

## 3. 已执行验证

以下命令实际通过：

```sh
go test ./backend/services/internal-finance-requests/...
go vet ./backend/services/internal-finance-requests/...
go test ./backend/platform/identity-access/... ./packages/workload-identity/... ./backend/services/payroll/...
go vet ./backend/platform/identity-access/... ./packages/workload-identity/... ./backend/services/payroll/...
sh -n deployments/d1/ops/d1 deployments/d1/ops/test-p9-composition
python3 -m json.tool contracts/access/v1/examples/workload-identity-policy.json
deployments/d1/ops/test-p9-composition
deployments/d1/ops/test-release-bundle
git diff --check
```

`test-p9-composition` 使用实际 `docker compose config --format json`，但不 build、不 pull、不启动容器；它结构化验证 Workload、端点、volume 和健康依赖，覆盖 Finance 缺 Money、Finance 缺 Evidence、Payroll 缺 People、Payroll 缺 Money 四个失败关闭场景，并验证 Feedback 单 Key、双 Key 轮换正例及部分配置、错误项目、不安全 URL、歧义 Key、过量轮换 Key、短 HMAC、Session 模式负例。测试还用无状态 Docker mock 走过一次同选装的 D1 联合备份与空目标恢复命令图，核对 Finance／Payroll dump、选择声明、校验清单和 restore 命令均被纳入；BF-247 增加停入口→全部 dump→恢复入口的严格顺序、Payroll dump 故障 trap、非法 drain 和一致性文件 checksum，BF-248 增加指纹 checksum、匹配恢复和故意漂移失败关闭。另用短生命周期 PostgreSQL 18 实际执行指纹 SQL，验证 2 张表（含空表）、1 行事实及原始敏感值不出库，退出后容器删除。上述仍只证明运维编排和 SQL 可执行，不是十五库真实恢复证据。

`acceptance/p9-full-composition/verify-c0` 已提供真实运行窗口的只读前置入口：它要求十四个领域应用、十九个长运行服务精确匹配且全部健康，要求项目 115 通知配置，核对 Nginx 唯一发布端口、Suite Manifest、公共 readiness 与 D1 check，并记录镜像 ID／repo digest。原始 Compose 和 inspect 不落盘，防止部署秘密进入证据；`test-preflight` 用 mock 覆盖 19／19 正例、缺 Payroll 负例和 Feedback secret 不落盘。C0 尚未在真实全组合运行，不能记为通过。

## 4. P9 尚未完成

仍需在一次有界运行窗口内完成并在退出后清理：

1. 使用非占位部署秘密和当前镜像启动全组合，记录镜像 digest 与全部容器健康；
2. 连续执行 Tenant、账号／Person、薪酬、工资、财务申请、员工 API Key、发票上传／报销、实际付款与 Feedback 旅程；
3. 创建带 `public-ingress-quiesced-v1` 声明和十五库无秘密事实指纹的联合备份，恢复到空目标并通过自动指纹比对，再次执行 Evidence 字节、关键读取与续写；
4. 对比恢复前后 Tenant、稳定 ID、revision、精确金额、File Version、Money 引用、Key 非秘密元数据、反馈身份映射与审计；
5. 记录真实浏览器 Console、跨租户／撤权负例及回滚点。

P8 仍缺项目 115 服务端通知、有效 Feedback 运行凭证、双账号／双租户和真实跨域浏览器证据；该外部运行门禁未关闭前，也不能把“连续 Feedback”记为通过。

## 5. 资源状态

本批只执行 Go／shell／JSON／Compose 渲染测试，没有启动 D1、PostgreSQL 或其他常驻容器。全组合构建与恢复留到运行参数齐备后一次性执行，避免当前开发机长期占用资源。
