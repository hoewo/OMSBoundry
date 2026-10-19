# Money Flow Ledger：全新环境 Release 验收

里程碑：M6  
功能编号：MF-M6-001  
日期：2026-09-20  
状态：已实现并验证

## 1. 目标

证明 Money Flow Ledger 的实际 release bundle 可以在不依赖仓库源码运行时、不依赖 Contract Ledger、也不复用现有业务数据库的前提下，从全新 PostgreSQL 18 数据库安装为一个可登录、可完成代表性经营收支闭环、可导出数据并可安全重启的独立小系统。

本切片验证“交付物可用”，不以源码内 `httptest` 或当前开发库成功代替实际二进制验收。

## 2. 验收输入

验收只接受：

- 一个由 Money Flow `ops/build-release` 生成的 `.tar.gz`；
- 与归档同名的外部 `.sha256`；
- 一个可创建和删除临时数据库的 PostgreSQL 18 测试入口；
- 测试进程生成的临时目录、临时端口和一次性 bootstrap 密码。

归档中的 server、ops 和 backup 必须具有完全相同且非 `unknown` 的 version、commit、build time 和 target。验收不读取开发服务器进程、开发数据库或 Contract Ledger 配置。

## 3. Clean-room 安装顺序

自动化入口必须按以下顺序执行：

1. 校验外部 SHA-256，并严格解包固定六个 release 文件；AppleDouble、扩展属性副本和其他额外 entry 一律拒绝；
2. 创建一个全新的临时数据库，并确认 `ops check` 返回 `empty`；
3. 执行实际 bundle 的 `ops migrate`，再以 `check --require-current` 确认 schema 为 current；
4. 只向实际 bundle server 注入 Money Flow 数据库、临时监听地址、`MIGRATION_MODE=verify`、本地 HTTP Cookie 配置和一次性 bootstrap 凭据；
5. 等待 live／ready，通过页面登录首位 administrator；
6. 完成代表性业务链路和 v2 owner 台账导出；
7. 优雅停止服务，移除全部 bootstrap 凭据后从同一 bundle 重启；
8. 用既有账号重新登录，确认原事实和投影仍可读取；
9. 停止进程并删除临时数据库与临时文件。

任一步失败都必须终止验收并保留可定位的命令或 server 输出；不能跳过步骤后报告成功。

## 4. 代表性业务链路

第一版固定使用不绑定客户行业的最小应收案例：

```text
本方组织 + 往来个人
  → 人工应收义务 100.50 CNY
  → 确认形成 revision 1
  → 设置独立预计收款日期
  → 登记实际收款 40.25 CNY
  → 核销 40.25 CNY
  → 当前未结与预计金额均为 60.25 CNY
  → 导出本方 owner-ledger-export-v2
```

页面提交中的 Actor 只能来自登录会话。最终数据库至少直接核对一个本地用户、两个 Party、一条 open Obligation revision、一条 Cash Expectation、一条 Cash Transaction、一条 Allocation，以及精确的 `60.250000` outstanding。

## 5. 导出验收

导出必须来自实际运行进程，并满足：

- HTTP 响应是不可缓存的 attachment ZIP；
- 格式是 `omsbizfoundry.money-flow-ledger/owner-ledger-export-v2`、版本 2；
- 根目录只有 `manifest.json` 与十三张固定 CSV；
- manifest 的逐文件长度和 SHA-256 与归档原始字节一致；
- Obligation、Cash Expectation、Cash Transaction 和 Allocation CSV 包含本次业务事实；
- 预计金额不作为新事实进入归档。

## 6. 独立性和安全边界

- server 子进程使用显式白名单环境，不传入 Contract Ledger URL、数据库、Cookie、Evidence 或任何跨系统配置；
- 验收数据库名随机生成，绝不迁移、清理或覆盖现有数据库；
- bootstrap 密码只存在于验收子进程环境，不能写入 bundle、仓库、日志或验收归档；
- 第二次启动不再提供 bootstrap 三元组，既有管理员不得被覆盖或重复创建；
- 失败清理只终止本次启动的进程并删除本次随机数据库／临时目录；
- 当前没有 Contract → Money Flow 业务动作，因此本切片验证零依赖，不制造连接器联调。

## 7. 验收标准

1. 篡改归档、错误摘要、路径穿越、软链接、额外／缺失文件或错误文件类型在运行二进制前被拒绝。
2. 三个二进制 build identity 完全一致且可追溯，不连接数据库即可读取。
3. 实际 ops 对临时数据库依次报告 `empty → current`，最新 schema 与 bundle 嵌入清单一致。
4. 实际 server 仅在 `verify/current` 后监听，live 和 ready 均成功。
5. 首次登录和全部业务 POST 使用有效 CSRF 与会话可信 Actor。
6. 两个 Party、Obligation confirmation、Cash Expectation、Cash Transaction 和 Allocation 形成完整事实，金额精确且 current outstanding 为 `60.250000`。
7. 当前预计现金页面显示预计日和 `60.25 CNY`，没有把 `due_on` 当成 expected date。
8. v2 ZIP 文件集合、manifest、逐文件摘要及四类代表性事实通过离线核验。
9. 移除 bootstrap 凭据重启后仍可用原账号登录，用户数量保持 1，业务事实和预计投影不变。
10. Contract Ledger 不存在或停止时全部步骤仍通过；测试结束后临时数据库和进程归零。

## 8. 明确不做

- 不在本切片重复全量备份恢复和历史版本升级演练；它们分别进入后续 M6 切片；
- 不把开发数据库复制为“全新环境”；
- 不验证外部 TLS、反向代理、systemd、Kubernetes、DNS 或客户网络；
- 不引入客户岗位、行业费种、审批或专用核心字段；
- 不把三类客户装配场景塞进同一验收案例。
