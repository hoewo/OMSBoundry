# OMS 一期主体目录运行态清理故障与修复报告

日期：2026-10-22  
决定：BF-171  
环境：`https://oatest.antuinfo.cn/`

## 现象

BF-170 全量 Demo 业务清理后，用户重新建立甲方并打开“新建项目”，页面显示“合同功能尚未准备完成，请联系管理员”，保存按钮被禁用。Project、Party、Contract 服务、权限和页面读取本身均正常。

## 根因

Workbench 的项目创建是 Project → Contract → Party → Project Link 的纵向闭环。提交前必须确认 Contract Ledger 已切换到 Party Directory 权威：

- `party_runtime_mode=directory`
- `directory_authority_established=true`

BF-170 把 Contract／Money 的 `party_runtime_transitions`、reviewed mapping 和本地迁移来源 Party 一并当作业务数据清除。两库 schema 仍 current、容器仍 healthy，普通列表仍能读取，因此原 D1 与公网计数验收没有发现；workspace context 则正确返回本地主体模式，前端按失败关闭原则禁用项目创建。

## 修复

没有直接插入或伪造 runtime transition。修复通过既有 Owner 正式入口完成：

1. 分别在 Contract 与 Money 建立一个仅用于迁移来源的本方主体快照；
2. 运行各 Owner 的 Party migration preflight；
3. 逐条审核并精确映射到 Party Directory 本方经营主体 `95d3793f-7e0d-4ec9-acb8-9baf8bcb1039`；
4. 运行各 Owner 的 `party-runtime-transition` cutover；
5. 再次运行准备命令验证幂等。

首次结果：两套 Owner 均为 `mode=directory`、`changed=true`、`mapped_party_count=1`、`referenced_party_count=0`、`created_directory_party_count=0`。第二次均为 `changed=false`。

## 防回归

`acceptance/oms-phase1-public/verify-public-oms-v1.py` 新增强制检查：取得 Contract／Money 正式 Capability 后，分别读取 workspace context；任一不是 directory authority 都立即失败。新增纯函数单测覆盖两套 Owner 的成功、失败和非目标 Owner 跳过路径。

## 验证

- Python 单测：8/8；
- Python 语法检查：通过；
- Party runtime 准备命令重复执行：两套均 `changed=false`；
- 增强后的公网正式登录、五个 Gateway 授权、Owner 只读与两套 directory authority：通过；
- Project 0、Billing 0、Money 资金记录 0；修复过程业务写入 0；
- D1 `check`：通过；
- 长期容器：17/17 healthy。

## 结论

这次提示由错误的清理边界造成，不是用户输入、网络抖动或后台随机故障。页面失败关闭行为正确，运维清理与交付验收有缺口。该缺口现已修复；以后“清空业务数据”必须保留或正式重建 Party Directory runtime 基准。
