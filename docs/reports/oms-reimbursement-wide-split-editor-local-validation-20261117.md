# OMS 报销宽窗双区编辑本地验证（BF-341）

日期：2026-11-17（GMT+8）  
状态：本地实现与回归通过；后续已随 Suite v1.107 发布，见[发布记录](./oms-reimbursement-wide-split-editor-v107-publication-20261117.md)

## 可验证结果

- 报销弹窗桌面宽度上限从 720px 扩展到 1180px，主体成为“左侧费用明细＋右侧发票资料”两段工作区。
- 发票手工录入、上传识别和状态全部集中在右侧；每张发票由一个虚线边界完整包围，并以序号和明细名称关联左侧内容。
- 明细名称输入时实时更新对应发票标题；删除重复的关系说明、录入方式标题和方式解释，只保留动作标签与必要状态。
- 980px 以下按费用明细、发票资料顺序回落单列，不改变多项默认 1 条、草稿／提交数量门禁或逐明细一张发票规则。
- 仅修改 Unified Workbench 表面，不改变 Finance Owner、API、schema、Evidence、Capability 或业务事实。

## 已运行验证

- `node --check internal/server/oms_phase1.js`：通过。
- `node --check internal/server/oms_phase1_finance_owner.js`：通过。
- `node --test internal/server/oms_phase1_finance_owner.test.mjs`：10/10 通过。
- `node --test internal/server/*.test.mjs`：310/310 通过。
- Unified Workbench：`go test ./...`、`go vet ./...` 通过。
- `acceptance/p9-full-composition/test-preflight`：通过。
- `deployments/d1/ops/test-p9-composition`：通过。
- `git diff --check`：通过。

## 后续发布结果

- Workbench v1.107 镜像、发布包、升级前联合备份、正式 D1 check、21/21 容器健康和三份公网资源逐字节摘要均通过。
- 发布验收未创建或修改报销申请，也未上传发票。
