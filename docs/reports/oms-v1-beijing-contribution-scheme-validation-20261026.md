# OMS1 北京五险一金参考方案验证（2026-10-26）

## 问题与结果

问题：工资页只在没有缴费方案时报错，没有默认值、设置入口或可执行的恢复动作，导致新 Tenant 无法生成工资表。

可验证结果：Payroll migration 幂等建立北京 2026 标准参考方案，可直接生成工资表；Workbench 从“设置比例”展示来源和完整组件，允许复制为 Tenant 自定义方案后调整。

## 参考口径

- 北京市人社局 2026 缴费基数：社保下限 7,270 元，上限 36,348 元，2026-07 起执行：<https://rsj.beijing.gov.cn/xxgk/2024zcwj/202608/t20260821_4831461.html>
- 北京投资指南公开口径：养老 16%／8%，失业 0.5%／0.5%，工伤单位 0.2%～1.9%，医疗（含生育）单位 9.8%、个人 2% 加 3 元／月：<https://sw.beijing.gov.cn/zwxx/swxx/202605/P020260512520283489932576.pdf>
- 北京 2026 住房公积金：单位与个人同比例 5%～12%，基数下限 2,540 元、上限 36,348 元：<https://www.beijing.gov.cn/fwcj/jiage/zffw1/685cd52595ff563574309b82.html>

工伤和公积金不是单一比例。候选参考方案分别预填最低 0.2% 和 5%，页面在复制为本公司方案前要求显式确认；这不是自动法律合规结论。

## 实现边界

- 标准方案是 Payroll 读取的跨 Tenant 目录，Tenant 方案仍通过精确 source ID／revision 追溯。
- 北京医保个人固定 3 元使用整数分；公积金按整元四舍五入，不使用二进制浮点。
- 新建工资表继续冻结方案 ID／revision 与完整快照，后续改比例不静默改写旧工资表。
- 不新建全国政策平台，不自动识别公司工伤行业档，不替代实际申报核对。

## 验证状态

- Payroll 单元测试：`GOMAXPROCS=2 go test -p 1 ./...` 通过。
- PostgreSQL 18 集成测试：`TEST_DATABASE_URL=... GOMAXPROCS=2 go test -p 1 -tags=integration ./internal/payroll` 通过，覆盖 migration、标准方案读取、Tenant 隔离、复制修订与工资计算。
- Workbench 目标 Node 测试：`oms_phase1_payroll_owner.test.mjs` 5/5 通过；Workbench Go 测试与 Payroll／Workbench `go vet` 通过。
- 使用生产 CSS 在 1440×1000 Chrome 视口完成可视检查：字段列、滚动区和固定操作栏未发现溢出或错位。
- 全量 Workbench Node 回归为 252/261；9 个失败属于本变更前已存在的操作列计数、Feedback 断言、Finance 资源版本及参考部署环境脚本问题，因此不将全量前端回归声明为通过。
- 本报告记录发布前本地实现与验证；后续已随 D1 Suite `v0.1.0-test.oms-v1.37` 发布，公网状态和恢复点见[发布记录](./oms-v1-beijing-contribution-scheme-publication-20261026.md)。
