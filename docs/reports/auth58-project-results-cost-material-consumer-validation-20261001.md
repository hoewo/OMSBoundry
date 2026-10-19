# Auth58 Project C5／C6 资料消费者（2026-10-01）

结论：候选消费者源码、权限契约、真实 PG18 升级／单库空目标恢复通过；主站未升级。上游 HTTP 与 Credential 是显式测试提供者，不是完整真实 IAM／多进程链。

## 改动与实测

Project 只新增 Results／Cost／Engagement 三类显式 Link、000009 约束和只读资料组合；校验同 Tenant／Project／ID／类型／可选 revision，新撤回来源拒绝关联，旧关联遇撤回保留状态／revision／digest、显示 invalid／source_withdrawn 且不调用 Evidence。沿用原生 read，Cost 两聚合分别单能力；无金额、下载、来源表复制或新服务。Cost 端点可选注入且正式 IAM 禁止静态 Token；Results Completion 原 Project 类型权限保持。现有类型下拉增加来源，没有新增 UI 平台。

- Project 全包测试／vet、四包 race、Access Contract、15 应用静态边界审计及负例自测、D1 release bundle 通过；原选择器 13 项、立项反馈和四弹窗取消 Node 回归通过。
- 三类型 typed reader 验证原生精确路径／动作／资源类型、明确空、同 slot 多文件／版本、slot 前缀排序、withdrawn，以及身份、schema、revision、摘要、非法／重复／无序引用、冻结版本和文本边界失败关闭。只有 active Link 参与，来源／文件 drift、空漂移和局部失败不写事实。
- ECS 四条真 PG18 用例 PASS、无 SKIP：历史 `TestProductionMaterialMigration7To8KeepsLegacyFactsAndIsIdempotent` 特意只执行 000008；新的 `TestRecordedMaterialMigration8To9KeepsFactsAndAllThreeLinks` 保持七张业务表／旧 Production Link／ID／revision，重复 migrate 不改事实，三类新 Link 可追加。
- `TestProductionMaterialProjectPostgresHTTPCompositionAndEmptyRestore` 再回归生产资料，在 current schema 9 的空库恢复保持事实并追加 unlink；`TestRecordedMaterialProjectPostgresThreeSourcesAndEmptyRestore` 验证三来源错 Project／新撤回 Link 零写入，旧成功命令源离线本地重放，逐来源 403／503／drift／空／撤回局部降级和全部 GET canonical 不变。dump→真正空库后原 ID／revision 4／三个 Link 保持，追加 unlink 成 revision 5、清单只余两个。
- 仅私有 PG 和一个 runner、internal 网络、零公开端口、限额 0.5 CPU／352 MiB；结束全部停止、不删容器或卷。主站 13 库摘要／17 容器 ID 前后及停止后相同，restore20 停止。

入口：[专项说明](../../acceptance/d1-c5-c6-project-material-consumer/README.md)。ECS `/opt/omsbizfoundry/acceptance/recorded-material-consumer-20261001/recorded_material_consumer_auth58_20261001a/` 保留原始结果和两份 dump。results SHA-256 `92d944b900b13c90d9dc4105cf2ff3bd44ea23494b42f62d7041141beb860f64`，binaries `86bbf5b5a56f8ead19a02b47936f550645d1995f72099f6e3fb6f79fa3c2a6ae`，log `9fa70ea787d05d096fd3d60c4fa9596422d5e2b589a1a021e23444dae8820e56`。

C4 current-schema 恢复 canonical `70282764f5e95fc716ecdf14f539b55fcfa58acef82bbe6572272382311517d5`／dump `ef0168bb63011a3e84fc85e447fe0385fdf2043243cca64265169cc41168e474`；三来源 canonical `d67688fee369435cf1bfd8089a3292fac9f00540f369a9ea047bce258d996827`／dump `71b0356f880909b5c2e356bcc9d8dde360912b924a9bffe3188171fd78ea5769`。

## 尚未代表

未发布候选镜像／主站；没有正式兼容闭包、联合多库／文件恢复、真实 Workload→目标来源→Evidence、公网 Nginx／Gateway／浏览器或原连续 C8 通过结论。单库恢复不能替代全 Suite。整体维持 `insufficient-evidence`。
