# OMS 账号与成员关联闭环公网发布记录（2026-10-30）

## 结论

BF-292 已发布到 `https://oatest.antuinfo.cn/`。本次只替换 Gateway 与 Unified Workbench 运行镜像，不修改 IAM、People、Owner schema、Grant 或业务数据；账号与成员仍由 IAM 和 People 分别拥有。

## 发布制品

- Gateway：`v0.1.0-test.oms-v1.61@sha256:fc9ae0efceccbb5e521c96ed45d4acb20bd96efbdc20534cb07f856176197cdd`
- Unified Workbench：`v0.1.0-test.oms-v1.61@sha256:0b5788f6a5d3d27c54c1f9605339abd40beeb55652131d9a325559a8a51da82b`
- 两个镜像均为 `linux/amd64`，运行容器的不可变 Image ID 与上述 digest 一致。

## 发布过程

1. 从当前 `v1.60` 环境文件生成 `v1.61` 候选，只替换 Gateway 和 Workbench 两个 digest 引用。
2. 候选 IAM 契约预检返回 `contracts-valid`。
3. 正式升级在变更前创建联合备份 `20261029T170347Z`；`checksums.sha256` 中 19 个条目全部复验为 `OK`。
4. 正式 upgrade 退出码为 0，随后使用同一候选环境再次独立执行 D1 `check`，全部 schema／存储检查通过。
5. 19 个正式长运行容器全部为 `healthy`；Gateway 和 Workbench 的运行配置精确引用本节 digest。

## 公网复验

- `/` 返回 `303` 到 `/workbench/?view=oms-v1`。
- `/open-api/v1/openapi.json` 与本地规范 SHA-256 均为 `2152f58efbe7d28d2df2d9e5d82f8745b5c6018092722611eca72488d082c9d8`，规范包含 `member_account_link`、`GET` 和 `/workbench/?view=oms-v1#member-account-link`。
- `/open-api/v1/docs` 和 OpenAPI 规范均返回 `200`。
- 公网四个版本化脚本与发布源码摘要逐一相同：
  - `oms-phase1.js`：`025dde90c7e3cad7fa3f0c71f417903c8079e4cfcdc7222205ba894d9481fc08`
  - `oms-phase1-internal.js`：`f4f032f19f43d42bc287a0bcc2dee7bf8cff81495fdbdcb3e3c6623a44ff3588`
  - `oms-phase1-open-api.js`：`b9ae30d66de8d453f7a32d70ae709f2577095cfcb7e126a65ccefb65a2857d9c`
  - `oms-phase1-identity.js`：`da843e8587d5b6376fd00d100e999d05a7ea205b8c8f86d422708183d6c58fd3`

发布后检查只执行读取、摘要和健康验证，没有创建邀请、关联或解绑成员，也没有写入业务记录。真实关联操作继续由管理员在成员维护、账号邀请或 `#member-account-link` 恢复入口显式发起。
