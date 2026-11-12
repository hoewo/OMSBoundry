# OMS 账号页下拉排版回归修复及公网发布记录（2026-10-30）

## 问题与结论

BF-295 发布后，账号页出现组织切换按钮换行、搜索框被压缩、三级筛选器横向撑出视口。根因不是账号或组织数据，而是统一下拉增强会把原生 `select` 包装成实际参与布局的 `.oms-v1__select`；账号页仍把宽度和响应式规则设置在随后被隐藏的原生元素上，包装层保留全宽默认值。

BF-296 已把桌面和窄屏规则移到实际包装层：

- 组织切换保持受控宽度，切换按钮固定且不换行；
- 桌面搜索框保留主要空间，三级筛选使用有上限的弹性宽度；
- 1000px 以下搜索独占一行，筛选器弹性换行；
- 720px 以下筛选器按两列和末项单列收纳；
- 页面不再横向溢出，账号表格仍可在自己的容器内滚动。

修复已发布到公网。本次只替换 Unified Workbench，不修改 IAM、People、Owner、数据库 schema、Grant、账号、组织、成员关联或业务数据。

## 本地验证

- `node --test frontend/oms-workbench/internal/server/oms_phase1_organization_switch.test.mjs`：4/4 通过，包含增强下拉包装层宽度回归。
- `GOWORK=off go test ./internal/server`：通过。
- `git diff --check`：通过。
- 本地管理员页面以真实统一下拉增强逻辑运行；窄屏浏览器确认搜索独占一行、筛选按两列／单列排列、主操作不溢出，表格滚动局限在表格容器。

## 发布制品与恢复点

- Unified Workbench：`v0.1.0-test.oms-v1.65`
- 镜像摘要：`sha256:08725603900771f673f57f2b80ad7e81832e74d7ebaf22e1c6d7a0330329a524`
- 平台：`linux/amd64`
- 构建元数据：`deployments/d1/.runtime/build-metadata/v65-workbench.json`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T133848Z`
- `checksums.sha256`：19/19 项通过。

## 发布验证

1. 候选环境从 v1.64 派生，只替换 `D1_WORKBENCH_IMAGE`；排除该行后的配置 SHA-256 一致。
2. IAM／装配契约预检返回 `contracts-valid`。
3. 正式 upgrade 成功，独立 D1 `check` 通过；Identity schema 保持 `000012_multi_organization_accounts.sql`。
4. 19/19 个正式长期容器健康；Workbench 运行 Image ID 与 Registry 摘要精确一致。
5. `https://oatest.antuinfo.cn/` 返回 `303` 到 `/workbench/?view=oms-v1`，Workbench 返回 `200`。
6. 公网与本地 CSS `v20261030.5` 摘要一致：`45c8ad5528cfa43ad4e928262b5c57594e1bdb6c8ed766c60fca9ac84be5d86c`。
7. 公网 CSS 包含 `.oms-v1__identity-switch>.oms-v1__select` 与 `.oms-v1__identity-toolbar>.oms-v1__select`，确认修复规则进入正式资源。

## 边界

本次是纯 Workbench 排版修复，不改变统一下拉行为、组织切换请求、Session 旋转、Capability 清理、账号权限或成员关联规则。公网验证没有提交账号、组织、权限、成员或业务写命令。
