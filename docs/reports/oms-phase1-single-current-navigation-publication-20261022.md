# OMS 一期侧栏唯一选中态公网发布验证（2026-10-22）

## 结论

BF-169 已发布到 `https://oatest.antuinfo.cn/`。项目经营、应收回款和应付支付可以同时展开，但只有当前页面对应的一个桌面二级入口保留选中效果；已经展开的其他组不再残留历史页面高亮。当前一级组仍用于表达导航路径。

本次只替换 Unified Workbench；Owner、API、数据库、schema、Grant、业务事实和其他 16 个长运行组件均未改变。

## 发布制品

- 源码：`main@5779637`
- 镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.oms-v1.22@sha256:d9b11412c72a6d4b9d9a9aa8c8786855e4e8f5219503edb8514554c0966a4f42`
- 平台：`linux/amd64`
- 静态资源版本：`20261022.2`
- 候选环境文件：`/opt/omsbizfoundry/auth58-v0.1.0-test.oms-v1.22.https.env`
- 发布前完整备份：`/var/backups/omsbizfoundry/20261022T023910Z`

## 验证证据

- `node --check frontend/oms-workbench/internal/server/oms_phase1.js` 通过。
- Workbench Node 测试 235/235 通过。
- `GOWORK=off go test -count=1 ./...` 与 `go vet` 在 `frontend/oms-workbench` 通过。
- `git diff --check` 通过。
- 本地桌面浏览器依次展开三个业务组并进入“发票管理”后，DOM 返回 `expanded=[projects,receivable,payable]`、`activeGroups=[payable]`、桌面 `activeSecondary=[incoming-invoices]`。
- 候选镜像平台为 `linux/amd64`，升级预检返回 Identity & Access `contracts-valid`。
- 容器 ID 对比仅 Unified Workbench 发生变化；新容器为 healthy。
- 发布后 D1 全栈 `check` 通过，17/17 长运行容器 healthy。
- 公网正式登录、Party／Project／Billing／Money／Contract 五次 Gateway 授权和 Owner 只读复验通过；读取计数分别为 7、28、2、3、6，业务写入为 0。
- 公网页面包含静态资源 `20261022.2`，HTML SHA-256 为 `de9e453b279e5233530c57f1ad7d8793d74cfae52912972f25e0abbf2d626c11`。

## 数据边界

发布前盘点确认当前 Party Directory 有 7 个主体，其中 1 个是部署必须保留的本方经营主体；Project Workspace 有 28 个项目，其中 2 个属于 OMS `operations` 范围。用户同时提出清空甲方、施工方、项目及全部数据，但“全部”是否包含登录账号、权限、本方经营主体和旧 OA 事实需要在破坏性操作前明确。本轮只完成可恢复备份，没有执行数据删除。
