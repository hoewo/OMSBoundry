# Auth58 Project 消费 Production 资料（2026-10-01）

结论：Project 消费切片的源码、PostgreSQL 18 的 7→8 升级、类型化 HTTP 资料读取和 Project 单库空目标恢复已实际通过。**主站未升级；本批上游 HTTP／Credential 是显式测试提供者，不是完整 Nginx→Gateway→IAM→Project→Production→Evidence 的通过证据。**

## 改动与边界

Project 只追加 `production-operations / production-work-item` Link 及自己的 000008 约束，不复制 Production 表、Evidence 字节或对象地址。链接前检查 Tenant／Project／来源身份和可选 revision，成功命令重放先读本地结果；资料读先取来源引用，再用精确 File metadata 能力读取文件事实。仅有单原生 reference.read／单 Work Item Scope 和 metadata.read／单 File Scope，没有下载或写来源权限。

`PRODUCTION_BASE_URL` 可选配置注入，未选来源不成为启动前提，正式 IAM 拒绝长期来源 token。旧 Design Link 保持原义，不自动改名或猜测。来源／文件 revision 变化显示 `revision_changed`，明确区分 forbidden、unavailable、invalid 和 empty；空来源版本变化也展示 drift。选项进入 Project 现有资源表单，没有新增页面框架或服务。

## 实际运行

- Project 全包普通测试、vet、projects／references／httpserver／config race 通过；typed reader 验证原生路径、固定来源、明确数组、排序／唯一关系和确定性 SHA-256，17 类畸形输入失败关闭。服务层覆盖源错误、源／File drift、旧／已解除 Link、不合法源零写入。
- `TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent` 真 PG18 PASS、无 SKIP：先执行原七个 migration，形成真实 Project 与旧 Design Link；000008 执行及复执行保持七张业务表 canonical、旧 Link ID／类型／revision，再允许追加 canonical Production Link。
- `TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore` 真 PG18 PASS、无 SKIP：错项目零写入、正确建立 Link、来源 503 时原命令复读不再请求来源；available／两类 drift／403／503／empty／empty drift 与恢复均不改变 Project 自有事实。
- Project custom dump 恢复至新空库（恢复前不 migrate），schema 8、八张表 canonical、Project ID／revision 2、Link ID 保持；恢复后显式解除 Link 追加到 revision 3。不是主站联合恢复，也不是把旧二进制直接拉起回退。
- Access Contract 权限约束、D1 release bundle、15 应用静态边界和边界负例自测通过。现有负责人选择脚本 13 项通过；本新增选项尚无真人点击证据。

恢复前全表 canonical SHA-256：`ba0701bd610b9aa2bb7fa4b3e31741048e939c104325c232ab6acc91b7bec588`。
保留 custom dump SHA-256：`8e066dad5fa99c899ae79b9ab432d4210d15626fb635c558db83b3cc754eae5a`。

## 夹具、失败和保留

只恢复启动自有 source 专项私有 PostgreSQL，新增单个消费者 runner；internal 网络、零宿主端口，总限额 0.5 CPU／352 MiB。未启动旧 source runner、restore20、Docker restart 或主机 reboot。主站 13 个数据库 canonical 和 17 个容器 ID 前后相等、17/17 healthy。所有探针结束停止，容器／卷／诊断／dump 保留；测试 helper 只清理隔离 PG 内自己创建的临时数据库。

首轮 `...20261001a` 的升级检查通过，但 HTTP 夹具误把 Evidence 原生 `/v1/references/files/...` 写为 `/api/v1/...`，返回 404；恢复尚未执行便失败，记录保留且不计通过。夹具同时改为真实 `evidence-service.metadata.read` 名称，正式应用的动作配置未改。第二轮 `...20261001b` 完整通过，未覆盖 a 的诊断。

入口见 [消费者验收说明](../../acceptance/d1-c4-project-material-consumer/README.md)。ECS 原始记录在 `/opt/omsbizfoundry/acceptance/project-material-consumer-20261001/project_material_consumer_auth58_20261001b/`，公开结果与二进制清单有离机副本。SHA-256：results `9e5710fb22e6a9a2c03e44a2e6d19bd4e365680ef609179ee6d1cff2cf341dc1`，binaries `b2f2a86f9b3fda3cc52d54224b49cf8ac584dbdf1fdada87ac8bedd548e2150e`，test.log `bc304417234d5f3c9355957a216b34c9ee77c2a9fb5be825f323678a307bb673`。

## 尚未通过

[Production 来源](./auth58-production-material-source-validation-20261001.md)已有独立真库／真 IAM 证据；两份组件证据不能拼接成未运行的完整旅程。正式兼容闭包和联合备份／空目标恢复、真实 Workload 跨进程与公网浏览器、C5/C6 material source、原连续 C8、Files 主 Provider 切换、Standard Web OSS 仍待验。整体继续 `insufficient-evidence`，本报告不宣称完整 OA 产品可用。
