# OMS1 P3 成员与薪酬档案本地验证报告

日期：2026-10-25  
状态：本地门禁通过；未部署 D1／公网  
决策：BF-238

## 1. A → B

成员基础信息、任职状态、固定工资、缴费基数和工资账户若继续混在 Workbench 浏览器数据中，就无法稳定隔离 Tenant、追溯历史或支持后续工资发放 → People & Groups 与 Payroll 分别成为成员和薪酬事实 Owner，经版本化 Person Reference 组合，在独立 PostgreSQL 18 数据库中保留 current projection、revision、幂等命令和审计。

## 2. 已交付

- People & Groups：部门、岗位、状态生效日期，以及 `hired`／`offboarded`／`rehired` 任职事件历史；重新入职沿用 Person ID。
- Payroll：按 Tenant＋Person 唯一挂载的 Compensation Profile 和 Payment Profile；金额均为整数分，变更要求生效月份、原因、expected revision 和命令 ID。
- Payment Profile：普通列表、详情和历史只输出后四位遮罩；完整账号读取要求独立 capability、精确 Payment Profile scope、用途并写敏感访问审计。
- Contribution Scheme：标准方案保存地区、来源、核对时间、六类个人／单位万分比、基数上下限和舍入；租户自定义方案从精确标准 ID／revision 复制并可追加本地 revision，不能改标准目录。
- Payroll→People：模板环境使用受限 token，IAM 环境使用独立 Payroll workload key、服务凭证和 `people-groups.person.reference.read`；引用失败关闭。
- Workbench：成员列表并行读取 People、Compensation 和 Payment；写入分别提交，Payroll 失败时不伪装回滚已成功的 People revision，并提示用户续办。Owner 模式不再把成员事实持久化到浏览器。
- D1 静态装配：Payroll workload key、workload policy、People endpoint、私钥挂载、pull override 和禁用清理均已加入；Payroll 镜像包含 keyctl。

## 3. 关键不变量

- People 与 Payroll 不共享表、ORM、数据库外键或跨库事务。
- People 状态只作为显示和默认选择提示；inactive Person 仍可维护离职结算、补发／扣回准备或重入职薪酬档案。
- People 新 revision 不改写 Payroll 已捕获的 Person revision；Payroll 变化也不写 People。
- 当前投影必须与最新不可变 revision 的全部业务字段一致，直接改投影或历史会被数据库约束拒绝。
- 普通响应、命令回放、审计详情和 Workbench 本地状态不含完整账号。

## 4. 实际验证

通过的门禁：

1. People、Payroll、Workbench Go 单元测试和 JavaScript 语法检查；
2. PostgreSQL 18.6 integration：新增／离职／重入职，薪酬与账户 revision、命令重放、双租户隔离、当前投影防篡改、历史不可变、标准方案复制和自定义方案修订；
3. Payroll HTTP：匿名拒绝、Capability 撤销、完整账号仅精确资源 scope 可读；
4. `./acceptance/p3-member-payroll-profiles/run-isolated`：真实 People／Payroll 双进程、两个数据库、版本化引用、账号缺失、遮罩／换卡历史、inactive 薪酬维护、稳定快照及未知 Person 负例；
5. Payroll D1 镜像含 server／ops／workload-keyctl，Compose 与 workload policy 静态解析通过；
6. 架构边界审计、格式、vet、diff 和秘密扫描通过。

## 5. 未外推范围

- 没有部署或变更 `oatest.antuinfo.cn`，也没有启动正式 D1 IAM／Gateway 组合。
- 工资期间／工资行、五险一金实际计算、绩效／个税、核对、同步、逐人／批量发放、Money 连接属于 P4，尚未由本报告证明。
- 财务申请正式明细、附件、审批和支付属于 P5；开放 API、闭环和 Feedback 分属 P6～P8。
- 标准缴费方案框架已完成，但仓库未内置未经权威核验的“全国默认比例”；正式导入必须携带地区、来源和核对时间。
