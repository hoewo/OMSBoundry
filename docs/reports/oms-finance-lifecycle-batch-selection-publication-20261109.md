# 财务生命周期批量选择发布验证（BF-321，2026-11-09）

## 发布结果

财务申请列表已随 Unified Workbench `v0.1.0-test.oms-v1.92` 发布到 `https://oatest.antuinfo.cn/`。草稿申请现在按既有 `submit` 能力进入提交，待支付／部分支付记录按既有付款能力进入确认支付；两种批量动作不可混选，缺少凭证的报销继续不可提交。

本次只替换 Unified Workbench 镜像，不修改 Internal Finance Requests、Money、Evidence、API、schema、Capability、Grant 或业务数据。

## 不可变制品

- Workbench 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.92@sha256:dcf9ea3120b5bde6f7d684cafcd6d9b5e168cc707fc2cfe2afd6cf012fbd46fa`
- D1 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.92_linux_amd64.tar.gz`
- 发布包 SHA-256：`41a59ef5d0fbec385c6d154ec6dbd5a3f451760c629dde7075453232e7cddb59`
- 公网财务脚本 SHA-256：`b42ff2e899311ac608a08c15b2235f43dc7c1d1b564c0ab18d8a109d4629180c`

## 升级与恢复证据

- 发布包外层 SHA-256 与包内 `RELEASE-CHECKSUMS.sha256` 均通过。
- 新环境文件由 v1.91 受限环境文件复制，只替换 `SUITE_VERSION` 和 `D1_WORKBENCH_IMAGE`；权限保持 `root:root 0600`。
- `preflight-upgrade` 使用候选 IAM 成功解析全部随包 Capability／Policy 契约。
- 升级前联合备份：`20261108T160513Z`，`checksums.sha256` 共 21 项且全部通过。
- Compose 清理旧网络时遇到活动端点竞态；数据卷未删除，随后用同一 v1.92 候选和同一环境执行幂等 `d1 up`，迁移和服务均正常恢复。未执行旧二进制回退或数据恢复。
- 独立 `d1 check` 通过；21 个长期容器全部为 healthy。
- 运行中的 Workbench 镜像与目标摘要一致，其余服务镜像保持原发布摘要。

## 公网验证

- `https://oatest.antuinfo.cn/` 返回 `303` 并指向 `/workbench/?view=oms-v1`。
- `oms-phase1-finance-owner.js?v=20261108.7` 返回 `200`，与本地发布文件 SHA-256 完全一致。
- 公网脚本包含草稿提交选择器、逐笔／批量 `openSubmit` 和“提交申请”主动作，不再包含旧的固定 `const selectable=` 付款选择逻辑。

## 未执行事项

发布验证没有在线提交、支付、删除或修改任何财务申请。使用真实 Owner 测试数据走通“勾选草稿—提交—刷新为待支付—再次勾选—确认支付”仍需另行安排受控写旅程；本次证据只证明制品、装配、运行健康和公网静态行为已经生效。
