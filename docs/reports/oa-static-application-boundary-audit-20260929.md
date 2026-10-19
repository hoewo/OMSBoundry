# OA 应用静态边界审计

日期：2026-09-29  
状态：`passed-in-static-scope`

## 1. 问题与结果

问题 A：局部运行报告可以证明若干跨应用链使用 HTTP，但不能自动证明整个工作树没有偷偷导入其他应用源码、通过本地 `replace` 依赖另一应用，或在 migration 中使用 PostgreSQL 跨库机制。  
可验证结果 B：建立持续失败门禁，扫描全部应用 module、依赖图和 SQL；当前工作树通过，故总报告可以把“最终静态依赖审计”从缺失证据收缩为已通过的静态范围。

## 2. 门禁

入口：

```bash
make architecture-boundary-static-audit
```

实现：

```text
acceptance/architecture-boundary-static-audit/verify
acceptance/architecture-boundary-static-audit/self-test
```

`verify` 对 `backend/services/*/go.mod`、`backend/platform/*/go.mod` 和 `frontend/*/go.mod` 执行以下检查：

1. 使用 `GOWORK=off go list -deps -test ./...`，证明应用不依赖根 `go.work` 才能解析；
2. 将依赖图与 15 个应用 module 清单比较，拒绝任何 App→App 源码依赖；
3. 检查应用 `go.mod` 的本地 replace，只允许根目录 `packages/*` 下显式共享的机制／wire contract 包，拒绝指向另一应用；
4. 扫描应用、部署和验收 SQL，拒绝 `dblink`、`postgres_fdw`、Foreign Table 和 `CREATE SERVER` 等跨数据库机制。

门禁已加入根级 `make check`。

## 3. 负例自证

`self-test` 在一次性临时目录建立两个最小 Go module：

- clean fixture 必须通过；
- App A 通过 `require + replace` 导入 App B package 时必须失败；
- App A migration 使用 `dblink` 时必须失败。

2026-09-29 实际结果：

```text
architecture-boundary-static-audit self-test: passed
architecture-boundary-static-audit: passed
  (apps=15,
   cross-app imports=0,
   cross-app replaces=0,
   cross-database SQL mechanisms=0)
```

脚本同时在 macOS Bash 3.2 和目标 Ubuntu 可用的 Bash 语法范围内运行。首版使用 `mapfile`／关联数组，被当前 macOS Bash 直接拒绝；修正为文件流后重新运行，正例与两个负例全部通过。这个失败被保留为门禁自身的可移植性证据。

## 4. 允许与不允许

允许：

- 应用内部包；
- `packages/workload-identity` 等没有业务事实所有权的共享机制包；
- `packages/payable-suggestion-contract` 等版本化 wire contract；
- 应用通过配置注入的 HTTP 端口消费其他应用公开契约；
- 一个 PostgreSQL 实例承载多个独立数据库／账号。

不允许：

- 导入另一个应用的领域、repository、ORM、`internal` 或公共 package；
- 用本地 replace 绕过模块独立性；
- `dblink`／FDW／Foreign Table 跨库读取；
- 因同机部署就把一个应用的表当作另一个应用的实现细节。

## 5. 证据边界

该审计证明当前默认 build/test 依赖图和 SQL 源文件中没有上述静态违规；它不单独证明：

- 运行时所有网络请求都经过预期版本化 API；
- 所有数据库账号在目标环境中都没有越权；
- 完整 C1～C8 连续旅程已经通过；
- build tag 下未进入默认依赖图的未来文件永远合规。

因此总报告可以把“静态依赖审计”标为通过，但仍需完整组合流量证据、数据库账号权限和连续旅程共同支持最终结论。

