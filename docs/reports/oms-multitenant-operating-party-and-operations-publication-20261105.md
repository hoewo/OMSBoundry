# OMS 多组织本方主体与内部管理修复发布记录（2026-11-05）

## 目标

同一 Workbench 承载多个组织时，本方经营主体必须随可信 Tenant 选择；财务操作人不应显示完整 Subject UUID；工资列表默认不展示已作废记录；管理员操作前不能仅因页面缓存的管理员状态暂时未就绪而错误拒绝。

## 实现

- 新增 `OMS_OPERATING_PARTY_BINDINGS`，格式为逗号分隔的 `tenant_uuid=party_uuid`。映射非空时只接受当前可信 Tenant 的精确项；缺项失败关闭，不回退全局 Party ID，也不按名称猜测。
- D1 配置、Compose、备份恢复、release bundle 校验和 ops 参数均携带并验证该映射；重复 Tenant、非法 UUID 和映射缺失均拒绝。
- 财务申请页读取当前组织 IAM Subject 目录，将审计 Subject 显示为组织内账号姓名；无法解析时只显示有界 `账号 <8位>`，不再暴露完整 UUID。
- 工资状态筛选默认选择“有效记录”，排除 `voided`；用户仍可显式选择“全部状态”或“已作废”。
- 成员关联等管理员动作在拒绝前重新读取 IAM Session，确认当前 Tenant 内管理员事实，避免初始化时序造成假阴性。

## 本地验证

- Workbench finance／payroll／member Node 专项共 17 项通过。
- Workbench config／server Go test 与 vet 通过。
- D1 release bundle 和升级选择脚本通过；目标差异检查通过。

## 公网发布证据

- Workbench：`v0.1.0-test.oms-v1.75@sha256:3eb794be17a0965d41bd68cabc03a8d49fb36ee0d5ca747873acf8ec61b360f8`
- release bundle：`dist/omsbizfoundry-d1_v0.1.0-test.oms-v1.75_linux_amd64.tar.gz`
- 候选配置：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.75.https.env`
- 正式恢复点：`/var/backups/omsbizfoundry/20261105T145201Z`
- 运行结果：19/19 长运行容器 healthy，备份 checksum、运行镜像摘要和公网根入口均通过。
- 当前两个 Tenant 的本方主体精确映射已用受信 Header 读取 HTML 验证：默认 Tenant → `95d3793f-7e0d-4ec9-acb8-9baf8bcb1039`；第二 Tenant → `748e7e43-b5e3-42b7-a58c-c6bf9bd8c1cd`。
- 公网 internal／finance／payroll 资源摘要与本地一致。

首次候选预检发现新解包目录权限过严，导致运行用户无法读取 release bundle；只修正该新候选目录为只读可读、ops 可执行后重新核对 checksum，第二次预检返回 `contracts-valid`，未修改制品内容。

发布验收未使用账号密码、未执行登录后点击，也未创建或修改业务事实；真实交互仍由用户现有 Session 复验。
