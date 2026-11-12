# OMS 当前租户 Party 运行时断点修复与验证记录（BF-280）

日期：2026-10-28
目标环境：`https://oatest.antuinfo.cn/` / D1
结论：当前活跃 OMS 租户的合同创建阻断已修复；不外推为单个 Operating Party 配置支持多个活跃租户。

## 问题与原因

真实管理员打开“新建项目”时，页面显示“合同功能尚未准备完成，请联系管理员”并禁用保存。Contract 和 Money 服务本身健康，直接原因是新 Tenant 没有自己的 Party migration mapping 和 runtime transition；既有准备与验收只覆盖了部署 bootstrap Tenant。与此同时，Workbench 仍引用旧 Tenant 的本方经营主体 Party ID，即使只修 runtime，后续也会在本方主体读取处失败。

问题 A → 可验证结果 B：新 Tenant 缺少 Party Directory authority 且本方主体指向旧 Tenant → 在不跨库写业务事实、不模糊合并主体的前提下，为当前 Tenant 建立精确 Party、受审阅映射和双 Owner cutover，页面重新开放项目＋合同创建表单。

## 操作与恢复点

- 修改前使用正式 D1 备份入口生成并校验联合备份 `20261028T114040Z`，恢复位置为 `/var/backups/omsbizfoundry/20261028T114040Z`。
- 通过 Party Directory 正式浏览器命令在当前 Tenant 建立 active Party：
  - 本方经营主体 `748e7e43-b5e3-42b7-a58c-c6bf9bd8c1cd`；
  - 精确旧名称施工方 `e4074a6e-8e38-47d9-b0ba-063c307419bc`。
- 已有 `甲方1` 使用当前 Tenant Party `1e09408e-0e19-468f-b612-d7b1e40b57fc`；没有把不同名称的“测试施工方1”模糊复用为“施工方1”。
- Contract mapping batch `c040fda0-a672-4b39-920a-7ab1c10c70f7` 和 Money mapping batch `e8730ef2-07bc-464d-873e-e7d8d4c65eab` 各记录三条完全一致名称的人工复核映射。
- 两个 Owner 分别执行不可变 `cutover` transition；结果均为 `mode=directory`、`directory_authority_established=true`、`mapped_party_count=3`、`referenced_party_count=3`。
- 活动环境用 `OMS_OPERATING_PARTY_ID=748e7e43-b5e3-42b7-a58c-c6bf9bd8c1cd` 取代旧兼容别名，并保留修改前环境文件副本；随后按正式 D1 apply 重建 Workbench／Gateway／Nginx。

## 实际验证

1. D1 `check` exit 0；Identity、Contract、Money、People、Party、Project、Evidence、Billing、Production、Results、Cost、Bonus、Finance 和 Payroll 的实际检查均通过。
2. 运行中的 Workbench 只含新的 `OMS_OPERATING_PARTY_ID`，旧 `OMS_QUALIFICATION_PARTY_ID` 为空。
3. Contract 与 Money 数据库按当前 Tenant 读取的最新 runtime transition 均为 `cutover`，批次分别与上述 mapping batch 一致。
4. 真实管理员 Chrome 刷新后进入“项目经营 → 项目列表 → 新建项目”：项目名称、甲方、应收总额、日期和备注字段正常显示；原红色“合同功能尚未准备完成”消失，保存动作不再因 runtime 禁用。验收点击取消，没有创建测试项目，项目总数仍为 0。
5. 本地防复发回归：

   ```text
   python3 -m unittest acceptance/oms-phase1-public/test_prepare_party_runtime.py
   ..........
   Ran 10 tests in 0.003s
   OK
   ```

   `prepare-party-runtime.py` 现在允许从 Owner-only（0600）JSON 文件读取目标租户账号，要求同时提供 `--expected-tenant-id`，并在 Contract／Money 的 preflight、mapping、transition 每次调用中显式覆盖该 Tenant；登录租户不一致时失败关闭。旧 `--qualification-party-id` 作为参数兼容别名保留，新名称是 `--operating-party-id`。

## 明确边界

- 本次只修复当前活跃 OMS Tenant，没有删除或改写旧 Tenant 的 Party、Contract、Money 历史。
- 单个 `OMS_OPERATING_PARTY_ID` 仍是单目标租户 Customer Overlay；同一 Workbench 同时承载多个活跃 Tenant 前，必须另行实现和验证 Tenant→Operating Party 配置。
- 本次没有创建项目、合同、票据或资金业务事实；两个 Party 是为恢复当前租户目录权威所需的正式目录记录。
- 该修复不改变 IAM Capability、Owner API、schema、数据所有权或精确金额语义，也不表示 OMS 全业务或 P9 已完成。
