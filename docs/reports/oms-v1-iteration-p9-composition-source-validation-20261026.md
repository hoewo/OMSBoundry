# OMS1 迭代 P9 D1 装配源码门禁报告

日期：2026-10-26

结论：P9 的 D1 选择、下游身份与静态交付缺口已经补齐并通过低资源源码门禁；本次没有启动容器，也没有执行全组合业务、联合备份／恢复或公网部署，因此 P9 尚未完成。

## 1. 审计发现

P2 已把 `Internal Finance Requests` 与 Payroll 放入 D1 profile、数据库、镜像和备份恢复，但按 P5／P7 的正式闭环复核后发现：

- Finance 的 D1 环境没有 Money／Evidence 端点，也没有独立工作负载身份；进程健康时付款和发票实时复验仍可能未接线；
- Payroll 的 D1 环境有 People 端点但缺 Money 端点，工资发放闭环不能由 D1 配置保证；
- D1 允许在缺少上述 Owner 依赖时选择 Finance／Payroll，错误会延迟到业务操作；
- Finance pull profile 没有覆盖新增 key helper 镜像。

## 2. 已完成修改

- Finance 新增独立 Ed25519 workload key lifecycle 和短期 IAM Credential Source；正式 IAM 模式拒绝静态 Money／Evidence Token。
- Finance→Evidence 只申请 `evidence-service.metadata.read`＋精确 File ID；Finance→Money 只申请 `money-flow-ledger.internal-finance-payment.record`＋精确 Command ID。
- 下游返回 401 时仅失效该 Tenant／Audience／Action／Resource 缓存，换票后最多重放一次；网络或不确定结果继续返回 unknown，由原幂等命令恢复。
- D1 为 Finance 注入 Money／Evidence／IAM 端点、私钥只读 volume 与健康依赖；Payroll 补上 Money 端点和 People／Money 健康依赖。
- Suite 选择在任何镜像构建、停服务或数据动作前强制 Finance→Money＋Evidence、Payroll→People＋Money。
- build／pull、停用清理、示例环境、workload policy、Makefile tidy 与静态组合门禁同步更新。

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

`test-p9-composition` 使用实际 `docker compose config`，但不 build、不 pull、不启动容器；它验证完整依赖选择可渲染，并覆盖 Finance 缺 Money、Finance 缺 Evidence、Payroll 缺 People、Payroll 缺 Money 四个失败关闭场景。

## 4. P9 尚未完成

仍需在一次有界运行窗口内完成并在退出后清理：

1. 使用非占位部署秘密和当前镜像启动全组合，记录镜像 digest 与全部容器健康；
2. 连续执行 Tenant、账号／Person、薪酬、工资、财务申请、员工 API Key、发票上传／报销、实际付款与 Feedback 旅程；
3. 创建联合备份，恢复到空目标，再次执行关键读取与续写；
4. 对比恢复前后 Tenant、稳定 ID、revision、精确金额、File Version、Money 引用、Key 非秘密元数据、反馈身份映射与审计；
5. 记录真实浏览器 Console、跨租户／撤权负例及回滚点。

P8 仍缺项目 115 服务端通知、有效 Feedback 运行凭证、双账号／双租户和真实跨域浏览器证据；该外部运行门禁未关闭前，也不能把“连续 Feedback”记为通过。

## 5. 资源状态

本批只执行 Go／shell／JSON／Compose 渲染测试，没有启动 D1、PostgreSQL 或其他常驻容器。全组合构建与恢复留到运行参数齐备后一次性执行，避免当前开发机长期占用资源。
