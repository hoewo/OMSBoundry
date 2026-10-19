# OMS 一期六 Owner 公网写入与边界验证报告

日期：2026-10-13  
关联：BF-150、[OMS 一期能力矩阵](../specs/assembly/oms-phase1-small-system-capability-gap-v1.md)、[边界演变账](../specs/assembly/small-system-boundary-evolution-ledger.md)

状态：**OMS 一期当前需要的六个业务事实 Owner 已接入并通过公网真实纵向闭环；完整 Suite 的其他应用有意不作为一期依赖。**

## 1. 接入边界

| 一期产品事实 | Owner | 公网结果 |
| --- | --- | --- |
| 甲方、施工方当前档案 | Party Directory | 创建、筛选、按 ID 复读通过 |
| 项目及稳定资源引用 | Project Workspace | 创建、三类 Link、详情和目录筛选通过 |
| 上下游合同承诺 | Contract Ledger | 8,000,000.00 与 2,680,000.00 精确复读，合同 Party 和附件引用通过 |
| 双向发票 | Billing Invoice | 开票 3,200,000.00、收票 960,000.00，筛选和附件通过 |
| 实际资金 | Money Flow Ledger | 回款 2,200,000.00、支付 1,200,000.00，筛选和附件通过 |
| 附件字节 | Evidence Service | 六份上传、可用状态、短期访问授权、下载字节哈希通过 |

IAM、Gateway、Nginx 和 Unified Workbench 是正式装配组件，不拥有上述业务事实。People & Groups、Business Engagement、Production、Approval & Results、Cost & Outsourcing、Bonus Distribution、Design Operations 虽随完整 D1 部署，但静态测试确认 OMS runtime 没有调用它们；这是一期间的产品边界，不是漏接。

## 2. 主体运行时切换

升级前远端备份：`/var/backups/omsbizfoundry/20261012T175912Z`。

迁移脚本先分页读取全部活动 Party Directory，再审计 Contract 和 Money 的全部本地主体。匹配优先级为精确登记标识、显式兼容的历史 D1 夹具标识、唯一精确名称；歧义失败关闭，无法匹配时创建独立 Directory 主体。结果：Contract 4 条映射、Money 5 条映射，两域均切到 `directory`；第二次执行 `changed:false`。六项 planner 单测覆盖精确匹配、历史后缀、唯一名称、新建、歧义拒绝和停用主体拒绝。

## 3. 公网业务旅程

入口：`https://oatest.antuinfo.cn/oms-v1/`  
版本：`v0.1.0-test.oms-v1.2`  
run key：`oms-phase1-public-20261013-a`

最终稳定事实：

- Project：`2a112e09-c0bf-419d-b615-d3420faeeec5`，revision 4；
- 上游 Contract：`0851f494-3351-4b7f-be12-0541ede68f33`，8,000,000.00；
- 下游 Contract：`a6f953ed-636b-4cf0-94dc-2078e85d25ae`，2,680,000.00；
- 开票／收票：`2c255321-4e09-4afb-a8fa-f343b51f53f9`／`8175fd85-870d-47dd-90f0-9abcdc6bd0ec`，3,200,000.00／960,000.00；
- 回款／支付：`e9b7247c-eaf4-4792-a1f7-02afb3830bed`／`9d989f01-4592-413c-a6f7-88de0358e383`，2,200,000.00／1,200,000.00；
- 六份 Evidence 均为 version 1，下载字节 SHA-256 均为 `2640059609118b695c139804676372eca75c30d30e5906775902d13e44f5356c`。

脚本使用同一 run key 再执行一次，返回同一事实 ID，没有第二个项目、合同、票据、资金或附件。访问过快首次触发 HTTP 429；脚本改为遵守 Gateway `Retry-After` 的有界退避后通过，没有提高限额或绕过公网入口。

## 4. 分页、筛选与导出

- Workbench 使用完整 cursor／offset helper，而不是只读取前 100 条；
- 205 条 offset、201 条 cursor、多页重复记录和重复游标失败关闭共 4 项 Node 测试通过；
- 公网真实筛选集合复读到本轮 2 个 Party、1 个 Project、2 张 Billing 票据、2 条 Money 流水；
- 本地 UI 以项目和日期筛出唯一 100,000.00 开票记录并下载 `OMS-开票记录-2026-10-13.csv`，269 bytes，SHA-256 `92f199dbd3847c4afb0a312fb9fafdce8afe0a0776dfc4312fdb0f073af906e2`；
- 第一期导出是 Excel 可直接打开的 UTF-8 CSV，不宣称原生 `.xlsx`。

## 5. 运行健康

发布后 `deployments/d1/ops/d1 check` 通过；Contract／Money 当前 schema 均为 24，Evidence 为 2，Party、Project、Billing 及其他已选应用均为 current。17 个长运行容器全部 healthy。既有只读公网脚本再次通过：Party 8、Project 27、Billing 2、Money 4、Contract 6，业务写入 0。

## 6. 结论与未外推范围

可以明确回答“OMS 一期需要的小系统是否接上”：**是，六个需要的 Owner 已接上并通过真实公网闭环。** 不能回答成“所有小系统都接进 OMS”：其他应用与当前一期无关，强行接入反而会破坏产品和所有权边界。

本报告不覆盖无固定期限义务／逾期催收、公司内部成本、通用审批、生产履约、奖金分配、全角色撤权矩阵或第二客户。后续若进入这些业务，必须分别定义 Owner、契约与验收，不能从本次六 Owner 结论自动外推。
