# OMS 一期资质单位自交易防护验证报告

日期：2026-10-19  
对应决策：BF-162

## 问题与结论

用户在项目“测试项目100”中新增向甲方开票时，Billing 返回 `INVALID_REQUEST`。Project Owner 快照显示该项目的 `client_party_id` 与 Customer Overlay 配置的资质单位 ID `95d3793f-7e0d-4ec9-acb8-9baf8bcb1039` 相同；即页面曾允许把经营主体自身选为外部甲方，Billing 随后按自身不变量正确拒绝了发票方与收票方相同的登记。

修复后，Workbench 从新建项目、关联施工方、开票／收票和回款／支付的候选中排除资质单位自身，并在每个提交路径再做一次稳定 Party ID 校验。对既有错误项目，页面明确显示“不能登记自开自收票据，请重新建立项目并选择真实甲方”，不再把 Owner 的英文错误码直接丢给使用者。

## 边界

- 这是 OMS 装配角色约束，不是 Party Directory 的全局主体分类。
- Billing、Money、Contract 和 Project 仍拥有并最终校验自己的业务事实。
- Project 当前甲方快照不支持普通覆盖。本轮没有直接改库、删除项目或伪造更正历史；错误项目需以真实甲方重新建立。
- 本轮不改变 Owner、API、schema、Grant、IAM 或 Gateway。

## 实际验证

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js`：通过。
- `node --test frontend/oms-workbench/internal/server/*.test.mjs`：235/235 通过。
- `go test -count=1 ./frontend/oms-workbench/...`：通过。
- 升级前备份：`/var/backups/omsbizfoundry/20261019T130505Z`。
- 发布镜像：`v0.1.0-test.oms-v1.18@sha256:2b2e319dfacbc6b84b743f5e3a9dcf09846d37c99f7de8523eba8990adf3f54f`，Registry 报告 `linux/amd64`。
- D1 `check`：通过；17/17 长运行容器 healthy。
- 公网正式登录、Gateway 向 Party、Project、Billing、Money、Contract 授权以及 Owner 只读复验：通过，业务写入 0。
- 公网 `oms-phase1.js?v=20261019.1` 已包含 `assertExternalCounterparty` 和自开自收的中文处理提示。

## 公网运行时快照

只读验收返回 Party 6、Project 27、Billing 0、Money 2、Contract workspace 6。这些数字是整个 Owner 的读取快照，OMS 页面仍按 BF-161 对 `operations` 项目及其关联票款进行范围过滤。
