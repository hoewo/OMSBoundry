# OMS1 页面、账号与 Feedback 修订验证（2026-10-26）

## 结论

BF-268～BF-271 已在本地分支 `codex/company-internal-management` 实现并通过当前范围的源码门禁。没有部署到 D1 或公网；外部 Feedback project 115 真人回复／通知和 P9 真实恢复仍未执行。

## 实现范围

- 报销与付款申请使用两套表单。报销录入内容、金额、日期与可选发票；无发票只允许保存草稿，正式提交仍要求 Evidence `invoice-v1`。付款申请录入说明、金额、日期、现金／银行卡和收款信息；现金不保存银行账号，银行卡必须提供银行和账号。
- 开票／收票、回款、应付、支付和施工方列表按“一列一个事实”调整，移除方向拼接、名称列附加说明及金额列重复状态标签。
- 新增实际“账号与租户”页面：读取 IAM Session 和受保护 Tenant 端点；管理员使用 IAM 正式 Subject、邀请、停用／启用和密码重置接口。一次性邀请／重置凭证通过 URL fragment 进入登录页，成功后清除 fragment。
- 新增实际“产品反馈”页面。Feedback 对所有有效登录账号开放，移除 `feedback.use` Manifest、Grant 和参考 OA 岗位权限项；Gateway 仍要求 CSRF 与有效 IAM Session，退出后返回 401。
- 桌面侧栏固定显示当前账号与文字“退出登录”，顶部增加“反馈”与“退出”快捷入口；侧栏业务导航独立滚动，确保菜单展开后会话与反馈入口仍可达。移动端继续提供底部反馈，并可从顶部直接反馈或退出。
- P8／P9 Feedback 浏览器和外部旅程驱动改为 Session 门禁，不再撤销／恢复角色 Capability。

## 实际执行

以下检查均在本地实际运行并成功退出：

```text
cd frontend/oms-workbench && go test ./...
cd backend/platform/gateway && go test ./...
cd backend/platform/identity-access && go test ./...
cd backend/platform/identity-access && go test -tags=integration ./internal/httpserver
cd backend/services/internal-finance-requests && go test ./...
node --check frontend/oms-workbench/internal/server/oms_phase1.js
node --check frontend/oms-workbench/internal/server/oms_phase1_finance_owner.js
node --check frontend/oms-workbench/internal/server/oms_phase1_identity.js
node --check frontend/oms-workbench/internal/server/oms_phase1_feedback.js
node --check frontend/oms-workbench/internal/server/oms_phase1_login.js
node --check acceptance/p8-feedback-browser/run.mjs
node --check acceptance/p9-full-composition/test-c11-browser.mjs
node --check acceptance/p9-full-composition/verify-c10-feedback.mjs
node --check acceptance/p9-full-composition/verify-c11-browser.mjs
make p8-feedback-browser-test
make p9-feedback-driver-test
make p9-browser-driver-test
make d1-release-bundle-test
make d1-upgrade-selection-test
git diff --check
```

BF-271 追加断言已验证模板固定存在两个退出表单（侧栏、顶部）、三个反馈导航入口（侧栏、顶部、移动端），并验证长侧栏只滚动业务导航、文字退出入口保持可见；两处退出均复用 Gateway `/gateway/logout` 和服务端 CSRF。

浏览器门禁结果：

- P8：真实 headless Chrome、双回环 Origin，`cross-origin-bridge` 19 项、`session-expired` 5 项、`feedback-unavailable` 5 项通过；没有访问外部 project 115，也没有记录 Secret。
- P9 C10：离线双 Tenant pending／resume、通知隔离和证据负例通过。
- P9 C11：真实 headless Chrome 驱动成功退出，覆盖退出后配置 401 与重新登录恢复；测试结束关闭临时 server 和浏览器，没有启动 OMS 常驻服务。
- D1 release bundle 与升级选择：闭集和选装回归通过，已确认不再携带 Feedback Capability Manifest。

## 未验证与非结论

- 未部署 `oatest.antuinfo.cn`，因此线上仍不是本轮版本。
- 未使用有效 Feedback project 115 凭证执行真人提交、开发者回复和通知。
- 未执行真实 D1 双租户全链、联合备份／空目标恢复或回滚。
- 未因此改变 IAM、Evidence、Money、Feedback 或 Internal Finance Requests 的事实所有权。
