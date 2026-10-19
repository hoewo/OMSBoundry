# OA 公网请求放大修复验证（2026-10-05）

## 结论

问题 A → 可验证结果 B：用户在付款等经营页只执行少量点击，页面却在一分钟内自动产生大量换票、重定向和逐条读取，最终出现 429 且 CSS 被 429 JSON 替代 → 保持 D1 动态请求限额 120／分钟不变，消除同 Audience／Scope 的重复换票，并把不可变静态资产从业务请求预算分开；同一真实付款页首次读取加五次显式刷新不再出现 429 或资产 MIME 故障。

本批是入口与客户端请求编排缺陷修复，不增加业务能力，不修改 IAM Grant、应用数据库、schema、Suite Manifest、业务事实或金额语义，也不宣称完整 OA 已通过。

## 根因证据

故障窗口的 Gateway 日志在一分钟内记录 190 个请求，其中 46 个是 `/gateway/authorize/*` POST，首次 429 在该分钟内出现，另有 6 个静态资产请求也被返回 429。代码复核确认：

1. Workbench 的 `businessApplicationFetch` 对每个集合项和关联记录都先换取一次相同 Audience／Scope 的能力；
2. 每次换票经 Gateway 产生授权 POST 和应用根路径重定向 GET，随后才发送真实来源请求；
3. 付款读取还会读取 Project、Money 工作区和关联记录，因此一次用户操作可被放大为 `8 + 3N` 量级的入口请求；
4. Gateway 旧限流把 `/workbench/assets/*` 与动态业务请求放在同一桶，业务突发耗尽预算后，浏览器把 429 JSON 当作 CSS，继发严格 MIME 错误。

因此“120／分钟”规则本身没有被用户点击触发；缺陷在客户端请求编排和静态资源共桶实现。

## 修复边界

- Workbench 按 Audience 串行授权与来源请求，避免同一 HttpOnly Audience Cookie 被并发 Scope 覆盖；
- 同一页面、同一 Audience、同一 Scope 的已成功授权只保存 30 秒的内存指纹租约，不保存 Token 或 Cookie 内容；
- Scope 改变、租约到期或授权失败都重新经 Gateway／IAM 换票；
- 所有真实目标 API 请求仍逐次发出，目标应用继续复验签名、Tenant、Audience、能力、Scope、资源和业务状态；
- Gateway 仅豁免不可变 `/workbench/assets/*` 的 GET／HEAD；HTML、登录、授权和全部业务 API 继续使用每客户端每分钟 120 次动态预算；
- 未通过增加阈值、绕过 IAM、复用管理员 Header、缓存业务响应或合并业务所有权解决问题。

## 自动化验证

源码回归实际执行并通过：

- Gateway `go test ./...`：全部 package 通过；新增 20 次静态资产读取不消耗 10 次动态预算，随后第 11 个动态请求准确返回 429；
- Unified Workbench `go test ./...`：全部 package 通过；
- Workbench／frontend Node 全量：379 项通过，0 失败，0 skip；新增 26 个同 Scope Money 读取只换票一次，Cost Scope 从 business 改为 production 时必须重新换票；
- `gofmt` 与改动文件 `git diff --check` 通过。

Linux AMD64 非 root 制品校验：

- Gateway `/app/server` SHA-256：`a48974b229117a85ebfc666b2241cca56cac95a5d41a1f324f0545aab7b3ca4f`；
- Unified Workbench `/app/server` SHA-256：`4f5702aa85cd7567711f9e4a8481dff684089507188c30e854cf85d1490cbabc`；
- ACR 摘要分别为 Gateway `sha256:0ceb122595bbf9055986ec658ed07b34110baa0a4edcd4fc28fb3f4595a9531d`、Workbench `sha256:e81d55ee4b90cf0b45ce0a5997c6a0ebd345938da05f8f30741f0059dab972c6`。

## Main 发布与公网复测

发布脚本先备份主环境文件，按摘要拉取镜像，依次滚动 Unified Workbench 和 Gateway，逐个等待 healthy；任一步失败会恢复旧环境并重新拉起旧摘要。发布后：

- 17／17 个 Main 容器 healthy；
- 其余 15 个容器 ID 与发布前一致；
- Gateway 和 Unified Workbench 分别运行上述新摘要；
- 没有执行 migration、数据库写入、Grant 写入或业务命令。

真实公网无人工节流脚本 `acceptance/oa-initiation-repair/verify-rate-limit-repair-browser.mjs` 登录正式 IAM，打开用户报告故障的项目 `3b4c44d3-7b33-4a29-add3-3f6211ca893e` 付款页，完成首次读取并连续点击“刷新当前支付”五次。结果：

| 指标 | 结果 |
| --- | ---: |
| 总 HTTP 请求 | 44 |
| Gateway 授权 POST | 8 |
| Money 授权 POST | 1 |
| Money 工作区读取 | 6 |
| HTTP 429 | 0 |
| 静态资产失败／错误 MIME | 0 |
| 业务写请求 | 0 |
| 页面 JavaScript 错误 | 0 |

同一时段 Gateway 非健康探针日志为 62 个 200、10 个 303、0 个 429；其中 303 是授权换票后的既有重定向契约。安全证据保存在 ECS `/opt/omsbizfoundry/acceptance/oa-rate-limit-repair-20261005/`，包含部署前后容器清单、环境备份、镜像摘要、浏览器 JSON、截图和 SHA-256；不包含明文密码、Cookie、JWT、私钥或数据库 dump。

## 未关闭事项

- 本批只关闭请求放大和静态资源连带 429；其他页面尚未实现的 OA 操作、缺失业务授权、业务来源无记录或真实业务错误仍按各自清单处理；
- 30 秒页面租约不改变最长能力 TTL 和撤权残余窗口，撤权专项仍按现有 IAM 契约独立验证；
- D1 是单实例内存限流；D4 跨副本限流语义仍未实现；
- 本次付款页为 0 条业务记录，证明入口、权限和空态稳定，不代替有 N 条关联记录的金额／历史业务验收；
- 完整 OA、全角色、双 Tenant、完整恢复和跨客户长期稳定边界继续按总计划验收。
