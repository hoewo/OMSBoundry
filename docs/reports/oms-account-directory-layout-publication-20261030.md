# OMS 账号目录排版公网发布记录（2026-10-30）

## 结论

BF-293 已发布到 `https://oatest.antuinfo.cn/`。本次只替换 Unified Workbench 运行镜像，不修改 IAM、People、其他 Owner、schema、Grant 或业务数据；账号与成员仍分别由 IAM 和 People 拥有。

## 发布制品与恢复点

- Unified Workbench：`v0.1.0-test.oms-v1.62@sha256:2c42cb2cd4d3333ad1acb9f4e77e76e0f734f9daa36d476dc6703ed7c1f72183`
- 平台：`linux/amd64`
- 升级前联合备份：`/var/backups/omsbizfoundry/20261030T121617Z`
- `checksums.sha256` 的 19 个条目全部复验为 `OK`。

## 发布过程

1. 以 v1.61 环境文件为基线生成 v1.62 候选；去掉 `D1_WORKBENCH_IMAGE` 后两份环境内容 SHA-256 相同，确认没有改变其他镜像或环境选择。
2. 候选 IAM 契约预检返回 `contracts-valid`。
3. D1 `upgrade` 在备份后完成，退出码为 0；随后使用同一候选环境独立执行 D1 `check`，全部 schema 与存储检查通过。
4. 19 个正式长期容器全部健康，0 个 unhealthy；Workbench 的运行 Image ID 与 Registry digest 精确一致。

## 公网复验

- `/` 返回 `303` 到 `/workbench/?view=oms-v1`，Workbench 返回 `200`。
- 公网 CSS `20261030.2` SHA-256 为 `5537dc8aed203b809e7f9cc157bd00caddfeeb45591d49f92c0809152060e158`，与本地一致。
- 公网 Identity JS `20261030.2` SHA-256 为 `e3463f3b863e148560cb699d62f8847d406991b0d6f9c348bbdabd13db17aa3d`，与本地一致。
- 真实管理员重新登录后，“账号与租户”显示轻量组织上下文、账号目录、成员关联、权限与状态、当前账号标记、三级筛选和收纳式操作菜单。
- 输入“戴鹏飞”后结果从 2 条收敛为 1 条；清空后恢复 2 条，过程中没有提交任何 Owner 写命令。

本轮发布验收只执行读取、筛选、摘要和健康验证，没有新增账号、修改权限、关联或解绑成员，也没有写入业务记录。
