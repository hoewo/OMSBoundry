# OMS1 迭代 P7 员工发票报销闭环本地验证报告

日期：2026-10-26

结论：P7 本地门禁通过；未部署 D1 或公网，P8 Feedback V2 与 P9 全组合仍未实施。

## 1. 问题与可验证结果

问题 A：P6 只有员工 API Key 与 `/open-api/v1/` 安全入口，外部 CLI／Skill 仍不能把发票文件、本人报销和最终付款串成真实事实链。

结果 B：使用正式 IAM、Gateway、Evidence、Internal Finance Requests、Money 和独立 PostgreSQL 数据库，经真实 TLS 入口完成一张虚构 PDF 发票从上传、提交、无需审批待支付、财务确认到本人查询已支付的闭环；依赖故障后沿原幂等标识恢复且不重复记账。

## 2. 实现范围

- Gateway 将 Evidence 上传会话、已签名本地数据面和 Finance 本人命令加入静态 `/open-api/v1/` allowlist；客户端身份头在转发前清除，只向 Owner 传递 IAM 签发的短期能力凭证。
- Evidence migration 3／4 保存发票校验状态和未引用清理尝试。首版支持 PDF、OFD、XML、JPEG、PNG 与 20 MiB 单文件上限；完成阶段复验哈希、真实结构，并拒绝 PDF 主动内容、XML DTD／外部实体和异常 OFD 包。
- Evidence 的已完成未引用文件按 7 天候选清理；删除前通过版本化 Finance 引用端口复验，存在引用或依赖状态未知时失败关闭并保存尝试结果。
- Internal Finance Requests migration 3／4 保存发票指纹、归档／恢复／软删除事实；提交阶段重新解析 Evidence File ID／Version，强重复阻断、疑似重复需显式确认。
- 本人 API 提供草稿、提交、撤回、恢复、删除、稳定游标列表、详情和条件读取；Tenant／Subject／Person 从 API Key 的当前 IAM 映射派生，不能由请求切换。
- 财务确认继续通过版本化 Money Payment Delivery。People Person 必须经受审计映射关联 Money `individual Party`，Money 只接受精确 `internal_finance_payment` 资源范围能力。
- 中文使用说明和 OpenAPI V1 同步交付；Webhook、AI／CLI runtime、自动代付和他人报销查询不在 P7。

文件安全是格式、结构、内容哈希和已知危险主动内容的有界校验，不等同于通用杀毒引擎；不得将本阶段描述为已经接入商业恶意软件扫描服务。

## 3. 真实闭环验收

入口：

```sh
./acceptance/p7-invoice-reimbursement/run-isolated
```

该入口以低并发构建并启动 IAM、Gateway、Evidence、Internal Finance Requests、Money、People 引用端口，一个限 1 CPU／768 MiB 的 PostgreSQL 18.6 容器及真实自签名 TLS Nginx；退出时无论成功失败均停止进程并删除容器。

本次结果：

- 13 个 HTTP 状态断言，0 个不匹配；
- Evidence 停机返回可重试 `503`，恢复后以原上传幂等键成功；
- 危险 PDF 在完成阶段返回 `422`；
- Finance 停机返回可重试 `503`，恢复后以原建单幂等键成功；
- 正式提交进入 `payable`，审批策略快照为 `not_required`；
- 同一发票代码＋号码再次提交返回 `409`；
- Money 停机产生未知付款结果，恢复后复用原付款命令；Finance 记录两次尝试，Money 最终只有 1 个交付、1 笔资金和 1 笔核销；
- 员工本人详情最终为 `paid`，支付／分配均为 12345 分；
- API Key 撤销后下一次本人调用立即返回 `401`。

验收数据均为临时虚构数据，未写入 D1 或公网。

## 4. 自动化回归

以下检查均在当前工作树实际运行并通过：

```sh
# 五个受影响 Go module，低并发串行
GOMAXPROCS=2 GOWORK=off go test -p 1 -count=1 ./...
GOMAXPROCS=2 GOWORK=off go vet ./...

# 一个临时 PostgreSQL 18.6，相关数据库集成测试
go test -p 1 -count=1 -tags=integration ./internal/evidence ./internal/httpserver
go test -p 1 -count=1 -tags=integration ./internal/finance

jq -e . backend/platform/gateway/internal/gateway/openapi.v1.json \
  contracts/access/v1/examples/money-flow-ledger-capabilities.json \
  acceptance/p7-invoice-reimbursement/suite-manifest.json
sh -n acceptance/p7-invoice-reimbursement/run-isolated
git diff --check
```

数据库测试另覆盖 Tenant 隔离、命令同键异内容冲突、不可变 revision、疑似重复确认、90 天草稿归档／恢复／软删除，以及已引用或引用状态未知的 Evidence 清理失败关闭。HTTP 测试覆盖本人不能读取他人申请和错误 Tenant scope 拒绝。

## 5. 阶段边界与下一步

P7 只证明本地发票报销纵向闭环，不表示 D1／公网已发布，也不表示银行代付、通用审批、OCR／AI、Webhook 或 OMS1 全业务闭环完成。下一阶段是 P8 Feedback SDK V2 正式装配；随后 P9 才执行 D1 全组合、恢复、连续旅程和公网证据。
