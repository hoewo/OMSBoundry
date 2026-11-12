# 财务发票占用状态修复与发布验证（BF-323，2026-11-09）

## 问题与结果

问题 A：财务申请仍显示草稿和“进入待支付”，提交后才提示“发票已在其他报销中提交”，用户既不知道占用记录，也无法区分申请状态与发票占用状态。

结果 B：Internal Finance Requests 现在对草稿／撤回申请实时投影已提交发票指纹。强重复时仍保留真实草稿状态，但不再返回 `submit` 动作；Workbench 在列表与详情显示占用申请标题、状态和查看入口。若读取后出现并发提交，正式提交事务继续最终复验，并在 `409 INVOICE_STRONG_DUPLICATE` 中返回稳定 `matched_request_ids`。

## 线上事实只读核对

本批只读检查现有数据，没有提交、更新、删除或迁移业务记录：

- 当前记录 `45972c19-a9bd-4661-9402-9c5442e6454e`，标题“1245123”，仍为 `draft / revision 1`；
- 占用记录 `5938d62b-f482-4148-ac4e-3cc71deb3032`，标题“4·14·12”，为 `payable / revision 2`，北京时间 2026-11-09 18:52:14 提交；
- 两条记录的发票号码均为 `24932000000038888512`，原件 SHA-256 也完全相同，因此是强重复而非疑似重复；
- 当前草稿建立在先，但草稿不预占发票；“4·14·12”先完成正式提交后形成占用指纹。

## 实现边界

- `finance_invoice_fingerprints` 仍只由成功提交事务追加；读取投影不新增表、migration、业务 revision 或审计事实；
- 强重复投影只移除当前申请的 `submit` 动作，不把草稿改成待支付、撤回或删除；
- HTTP 冲突返回稳定匹配申请 ID，普通本人列表只保留当前可读集合内的引用；完整标题、状态仍须通过既有 Finance 读取授权取得；
- Workbench 结构化保留 Owner 错误内容，显示可读取的占用申请并提供查看入口；不可读取时只显示有界短引用；
- 提交事务继续使用 Tenant 级 advisory lock 重新校验，读取时的预提示不能代替最终并发保护。

## 验证

- Internal Finance Requests：`go test -count=1 ./...`、`go vet ./...` 通过；
- PostgreSQL 18.6 隔离集成：`go test -tags=integration -count=1 ./internal/finance` 通过，覆盖草稿读取强重复投影、匹配申请 ID 和最终提交拒绝；
- Unified Workbench：`go test -count=1 ./...`、`go vet ./...` 通过；
- JavaScript：两个受影响文件 `node --check` 通过，全量 `node --test *.test.mjs` 共 301 项通过；
- 定向 `git diff --check` 通过。

## 当前状态

已发布到 `https://oatest.antuinfo.cn/`：

- release：`v0.1.0-test.oms-v1.94`，发布包 SHA-256 `adc982804034e34dd03a8cc914b645f2871f47d05663307034e6a39d366eaf71`；
- Internal Finance Requests 镜像：`sha256:228cc0afa11c718de313d3206dec03d5d4253c83e76564b655a9785c0f1a175f`；
- Unified Workbench 镜像：`sha256:163c644f10dc1b7dfe2d44d3f39e847b61a96a6e2471a858358d59ed761835f9`；
- 升级前一致性备份：`20261109T122746Z`；候选 IAM 合约预检和升级后独立 `d1 check` 通过；
- 第一次自动替换在停止旧栈时遇到一个已退出一次性密钥容器占用 Compose 网络的竞态，脚本在删除数据卷之前失败关闭；确认残留端点消失后使用同一候选环境显式 `d1 up`，未删除或恢复业务卷；
- 最终 21/21 正式长运行容器 healthy，Finance 与 Workbench 容器分别运行上述精确 digest；
- 公网 Finance JavaScript、通用 JavaScript 和 CSS 与本地构建 SHA-256 分别一致为 `a6640240fe66ee117e3151f3383d93f36ddac2eeb62df1804a52ce7030466440`、`f5a26ef45f194207bbce7d27ff3fe55606c7be81b9aef967992891aeb7117d81`、`dbf4bd77f7c8057d25f68204c1d71ddaf37ee281ce3c8d769d01c6d3cc120868`，并包含“当前发票不能再次提交”“查看占用申请”和 `matched_request_ids`；
- 发布后数据库只读复核确认“1245123”仍为 `draft / revision 1`，“4·14·12”仍为 `payable / revision 2`，没有因修复或发布发生业务 revision；
- 本轮没有以真人登录态再次提交、撤回或修改申请。登录态页面最终视觉效果由用户刷新现有页面复核；这不影响上述 Owner、镜像、公开资源和数据不变证据。
