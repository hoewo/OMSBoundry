# OMS 工资批量输入标签 v1.102 发布记录

日期：2026-11-12

事项：BF-335

状态：已发布并完成只读验收

## 发布结果

- 公网入口：`https://oatest.antuinfo.cn/`
- Suite／Workbench：`v0.1.0-test.oms-v1.102`
- 源码提交：`570409e03f82cd4d63ddbb04952a60659eed7e3a`
- Workbench 镜像摘要：`sha256:6aee35319edca37024e37f08756bfa07d864f33667e802d21febc580bce8a072`
- 发布包：`omsbizfoundry-d1_v0.1.0-test.oms-v1.102_linux_amd64.tar.gz`
- 发布包 SHA-256：`97da061e3551e35986ad612fe9e7c752cda861e8eef07aaec891f5ca421c26f4`
- 升级前备份：`/var/backups/omsbizfoundry/20261112T160639Z`

## 验证

- 本地 Payroll 专项 9/9、Workbench Node 308/308、Go test／vet、P9 preflight、D1 composition 与 diff check 通过。
- 正式 D1 check 通过，21/21 长运行容器 healthy。
- 真实 Chrome 完整刷新后，工资批量录入的每名成员均显示“当月绩效（元）”与“个人所得税（元）”。
- 可访问名称包含成员姓名与字段名，两个输入不再由单一外层 label 包裹。

## 未外推

- 发布验收没有生成工资预览、确认写入或修改任何工资事实。
- Payroll API、schema、Capability、金额计算和 preview／commit revision 门禁均未改变。
