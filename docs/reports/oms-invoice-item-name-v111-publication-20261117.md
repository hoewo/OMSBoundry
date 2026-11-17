# OMS 发票票面项目名称 v1.111 发布记录

日期：2026-11-17（GMT+8）
事项：BF-343
状态：已发布；真实原 PDF 待用户重新上传复验

## 问题与结果

问题 A：发票明细表存在“项目名称”，但识别候选、原图标注、人工校对和交付没有对应字段。
可验证结果 B：Invoice Recognition 提取首条票面项目名称，Workbench 以“项目名称（票面明细）”展示、标注、人工修改、确认和导出；该字段不解释为 OMS 经营项目，也不自动关联 Project。

## 发布事实

- Suite：`v0.1.0-test.oms-v1.111`。
- Invoice Recognition 镜像摘要：`sha256:d1bf513e5bfe006feb9968cd45c6bb8e96fbdd1f55b99387322184a337cf4ec8`。
- Unified Workbench 镜像摘要：`sha256:c25ce446f0ffe872c1689231db9fed40268cb97bcbe4b4fc330f92b6e965ae4b`。
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.111_linux_amd64.tar.gz`，SHA-256 `700d61b45a6a3d06c12797faa3365248b0e525b4221dc1184546c348472ab506`。
- 升级前联合备份：`/var/backups/omsbizfoundry/20261117T142624Z`。
- 候选 IAM preflight、release bundle 检查、正式 D1 check 均通过；21/21 长运行容器处于 running／healthy。
- 运行容器镜像摘要与候选一致；Workbench 运行资源同时包含 `oms-phase1-workforce.js?v=20261117.4` 和“项目名称（票面明细）”标记。
- 公网根入口返回到 `/workbench/?view=oms-v1` 的预期 303。

## 自动验证

- Contract 验证字段可选且人工确认值最多 500 字符。
- 图片／PDF TSV 回归使用“项目名称／规格型号”表头、同行数量与金额，结果只保留 `*生产生活服务*餐饮服务`，并生成相同文本的区域和置信度。
- XML Provider、Invoice Recognition 全量 Go test／vet、Workbench JavaScript syntax、Node 310/310、Server Go test／vet 与 `git diff --check` 通过。
- 确认请求、搜索、CSV 与 JSON 交付均保留 `invoice_item_name`。

## 发布恢复说明

标准 upgrade 已完成备份与候选启动，但旧 Cost／Outsourcing 一次性 workload-key 容器仍引用已过期的默认网络端点，导致清理旧网络时退出；没有删除数据卷。随后使用同一候选执行标准 D1 up 收敛，正式 D1 check 与 21/21 长运行容器复验通过。该既有运维残留不改变本批业务代码或数据边界。

## 边界与用户复验

- 本批只识别发票明细表第一条“项目名称”；多明细数组不在范围内。
- 它不是 Project ID／名称，不查询、匹配、创建或改写 OMS 项目。
- 历史 Proposal／Confirmation 保持审计原值，不做后台重算。
- 发布过程没有上传、确认或删除业务发票。用户需重新上传原 PDF，才会由 `tsv-v3` Provider 生成新字段与标注。

[本地实现验证](./oms-invoice-item-name-local-validation-20261117.md)
