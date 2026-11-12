# OMS 侧边栏组织切换公网发布记录（2026-10-30）

## 结论

BF-295 已发布到公网。桌面工作台侧边栏底部常驻显示当前组织；账号拥有多个启用组织时可原位展开并切换，只有一个启用组织时仅显示上下文，不呈现无效切换动作。切换复用 IAM 既有 `POST /api/v1/session/organization` 契约，成功后以新 Session 重载工作台并由 Gateway 清除旧组织 Capability Cookie。

本次只更新 Unified Workbench，不修改 Identity、Gateway、People、Owner、数据库 schema、Grant、账号、组织或成员关联数据。

## 发布制品

- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.64`
- 摘要：`sha256:888534ce4c40939c731cf704e0b0c04129ae9dc9616faf341bdc68bdbeb21e40`
- 平台：`linux/amd64`
- 构建元数据：`deployments/d1/.runtime/build-metadata/v64-workbench.json`

## 发布过程与恢复点

1. 从公网 `v0.1.0-test.oms-v1.63` 环境生成仅替换 `D1_WORKBENCH_IMAGE` 的候选配置，排除该行后的配置摘要保持一致。
2. IAM／装配契约预检通过，输出 `contracts-valid`。
3. 正式升级前生成联合备份 `/var/backups/omsbizfoundry/20261030T132510Z`，19 项 checksum 全部通过。
4. 正式 upgrade 成功，随后运行独立 D1 `check`；Identity schema 仍为 `000012_multi_organization_accounts.sql`。
5. 19/19 个长期运行容器健康，Workbench 运行镜像 ID 与上述摘要一致。

## 公网核对

- `https://oatest.antuinfo.cn/` 返回 `303` 到 `/workbench/?view=oms-v1`。
- Workbench 首屏返回 `200`。
- 公网与本地 CSS `v20261030.4` 摘要一致：`8b87fdf6afddd6522c9ff3bee95b2bab4cafcffaeacaac0e943e992bb398d970`。
- 公网与本地 Identity JS `v20261030.4` 摘要一致：`0bec017da05df56c378464aacdc6f2b60ee1557f024bd5f59335b54f920c684f`。
- 公网 JS 包含 `renderRailOrganization`、`switchOrganization` 与组织选项标记，确认新入口进入正式资源。

## 验证边界

本次公网核对未使用真实用户执行组织切换，因为该动作会旋转当前 Session；切换行为由本地专项 Node 测试、Workbench Go 测试以及 BF-294 已发布的 IAM／Gateway 组织切换集成契约覆盖。待具备明确授权且已加入两个组织的测试账号后，可补充真实双组织 Session、权限隔离和两个组织内不同 Person 关联的交互验收。未完成该补充验收不影响本次“制品已上线、装配健康”的结论，但不得把静态资源核对外推为真实账号交互已经执行。
