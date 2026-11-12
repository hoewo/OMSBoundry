# OMS 多组织账号公网发布记录（2026-10-30）

## 结论

BF-294 已发布到 `https://oatest.antuinfo.cn/`。线上 Identity 已使用 schema 12；同一登录账号可在多个组织拥有独立 Subject，组织切换旋转 Session 并清除旧 Capability。People 仍保持同组织一个 Subject 最多关联一个 Person，不同组织可分别关联。

## 发布制品与恢复点

- Identity Access：`v0.1.0-test.oms-v1.63@sha256:224b72cd494d3e6733fe199d174b1e9aabc63ff6bba64270dec7bd328841cbb9`
- Gateway：`v0.1.0-test.oms-v1.63@sha256:e063640bc9635bcf4d0482e434628d202dccb3db2d5a6fd7c1f5989d01f42b60`
- Unified Workbench：`v0.1.0-test.oms-v1.63@sha256:11043e12f42d5c48ede43ad5217748702f0c68a09aad566eaccbea66014c947d`
- 平台：`linux/amd64`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T130723Z`
- `checksums.sha256` 的 19 个条目全部复验为 `OK`。

## 发布过程

1. 以 v1.62 环境文件为基线生成 v1.63 候选，只替换 Identity、Gateway 和 Workbench 三个 digest-pinned 镜像。
2. 候选 IAM 契约预检返回 `contracts-valid`。
3. D1 `upgrade` 在联合备份后完成，退出码为 0；Identity migration 将 schema 11 升级到 `000012_multi_organization_accounts.sql`。
4. 随后使用同一候选环境独立执行 D1 `check`，全部 schema 与存储检查通过。
5. 19 个正式长期容器全部健康，Identity、Gateway 和 Workbench 的运行 Image ID 与 Registry digest 精确一致。

## 公网复验

- `/` 返回 `303` 到 `/workbench/?view=oms-v1`，Workbench 返回 `200`。
- 公网 Identity JS `20261030.3` SHA-256 为 `d533496629b26bb5b9aaef337cfec95c50ff6bcb1cb0f6990f3bedf1daaf15ed`，与本地一致，包含组织列表、组织切换和多组织说明。
- 公网 System Admin JS `20261030.1` SHA-256 为 `7d58d2746cd4a7854652e5c1bdff863273aabb4e6c63816f291c583b03350115`，与本地一致。

本轮发布验收没有创建新组织、把现存账号加入另一组织、提升权限或修改 Person 关联，避免在没有额外业务授权的情况下改动线上身份数据。真实同账号双组织切换和两个 Person 分别关联由发布前 PostgreSQL 18 端到端集成测试覆盖，如需以现存公网账号验收，应先明确目标账号和两个组织。
