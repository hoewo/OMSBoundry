# OA 长时间开页安全令牌与连续验证

日期：2026-10-02  
范围：Workbench 的 Project／People 授权入口；沿用 BF-084、BF-087、BF-090、BF-091 和 ADR-0035，不变更架构基线。

本报告保留 `.4` 的实际连续旅程证据；当前 Workbench 已推进 `.5`，补修未知回复提示和重新加载的旧抽屉遮挡，见[后续报告](./oa-acknowledgement-recovery-validation-20261002.md)。不覆盖本批失败或外推完整 OA 验收。

问题 A → 可验证结果 B：表单放置超过安全 Cookie 生命周期后，仍提交最初 HTML 的令牌而被拒绝 → 提交前读取当前 Cookie；缺失时只从配置的 IAM 入口取得新令牌，原输入保留，随后仍经 Gateway／IAM 正式授权。

不变量：不延长 Cookie、Session 或能力凭证寿命，不关闭 CSRF／IAM，不读取密码，不扩大 Grant，不自动重试业务写入，不替换业务 revision，不让工作台拥有业务事实。相邻未接通字段、全部页面的登录恢复和完整角色矩阵不属于本切片。

## 真实反例与恢复

Mac 解锁后继续同一夹具 C `70edead7-9dc9-44a3-9b35-b9d41b83264e`，不另建项目。明确确认生产下拉值为“不设置 / 清除当前负责人”，点击保存后显示“安全校验失败；当前输入保留，可重试”。原生复读仍 active revision 4、两名负责人、四条历史、六个回执；失败关闭在业务写入前。原始失败保留为 `production-clear-attempt2-csrf.json`，不覆盖第一次未确认选中值的失败。

代码确认 Gateway／IAM 同名 CSRF Cookie 为 3600 秒，原 Workbench 使用初始 HTML token。页面正常刷新后会话仍在，不需要新登录或授予权限；重新明确选空并保存成功，生产负责人清除为 revision 5。再明确清除经营负责人，页面两项均“待设置”，原生 revision 6，原分工保留为历史。该恢复证明刷新可解决此次阻断；不以源码推断取代后续候选的公网验证。

## 修复与候选检查

Workbench 从 Suite Manifest 注入已启用 IAM 路由，Project 与 People 共用小型 CSRF helper。已有 Cookie 优先，不因每次动作轮换；缺失时合并并发的一个同源 GET、禁止重定向／外部路径、核对响应与实际 Cookie。失败保留表单且零业务命令，下一次由用户显式重试。Gateway、IAM、各业务应用和契约不变。

实际串行：Node 52 项零失败／skip；Workbench Go test／vet；Gateway 原有 HTTP 安全回归；Python 纯验收门禁 6 项；JavaScript／Python 语法和 diff check 通过。新增 8 个 CSRF 用例覆盖其他页轮换、过期、并发、异常响应、网络恢复和授权失败零写。opt-in 合成浏览器未在本批运行；原 OA 仓库仍 clean，同提交 `826e0fc65aa09947fc4f64f62a2644fa5d11a302`。

## 实际公网续验

`v0.1.0-oa.20261002.4` 已部署 Auth58 Main。仍用原 C，不另建项目、不补 Grant：在基本信息抽屉明确选回原经营／生产人员，保留日期、类型和概况；通过 native Chrome DevTools 仅让 `oms_iam_csrf` Cookie 到期，确认其已缺失后直接点原保存按钮。没有刷新页面、修改登录会话或关闭安全检查。

真实结果为“保存成功”。Gateway 脱敏日志依次记录配置的 IAM `/auth/api/v1/csrf` GET 200、Project 授权 POST 303、**一次**原生 `basic-info-changes` POST 200，随后正常授权及详情读取；不是 Console 写业务、API 代点或自动重放。首次原生键盘输入标点未生效，仅产生另一临时 Cookie，已明确清除；修正输入后以布尔观察确认目标 Cookie 缺失才提交，未输出 Cookie／密码值。

| 同一 C 的操作 | 实际结果 |
| --- | --- |
| 明确清除生产负责人 | active revision 5，经营分工保留，旧生产分工结束但历史不丢 |
| 明确清除经营负责人 | revision 6，无当前负责人，旧经营分工保留历史 |
| Cookie 缺失后在原表单恢复两类负责人 | revision 7，新分工均始于 revision 7；旧分工分别结束于 5／6，原快照不改 |
| 返回列表，按正式编号 Enter 搜索并打开 | 唯一 C，仍是 OA 工作台详情，不跳独立 Project 产品 |
| 刷新详情、浏览器 Back／Forward、页面返回 | C ID、资料、负责人保持；列表保留搜索条件 |
| 最终只读复读 | `final.passed=true`、`continuous_journey_assertions_passed=true`；一次 activation，取消／无效输入零事实及回执增量，no-op 只增加一个回执 |

最终全部既有 Project 行、用户“测试项目101”及旧 A／B、Grant 摘要和其他 11 个业务库 canonical facts 与派发前基线相等。只读脚本业务 API 写入／Grant 写入均零；正常 IAM 登录、换票、注销及审计不声称整库不变。本批公网编号为测试格式，搜索与返回已验，不据此声称原编号年份排序或项目包公网矩阵通过。

## 实际发布与恢复边界

- 平台 Linux/amd64，`official_release=false`，dirty source commit unknown；Source SHA-256 `002228663c3b843fb9394f88ad6967250dcb459cea981e0e0b494dcc5452a5b6`。
- Workbench binary SHA-256 `c0e2f6dc7e2c2781cfee3e0415dee9d4be391eadd44c56392b6a506e7e8a33d9`；公网 runtime SHA-256 `67000acd9195c07010251f31ec2cf8d66533b7d9fa8ab47d0ae698a39db8add9`。
- ACR Workbench digest `838de9698c9c64d06bbaccd04efb097a2def8bf247db4845549bc7d5012fac5d`；只重建 Workbench，其他 16 容器 ID 不变，17/17 healthy，D1 check 通过。
- publication 拉回字节、prepare、联合冷备份、13 个空数据库逐个还原、文件／IAM key 字节核对及 apply 通过；Project schema 10、Project 镜像、IAM／Gateway／Suite／Grant 均未改。本批没有 `.4` 阶段失败；`.3` 的过早 prepare 拒绝仍留在原报告。
- ECS 备份 `/var/backups/omsbizfoundry/oa-public-20261002d`；离机副本 `/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-csrf-backup-20261002.XSyHke/oa-public-20261002d-backup.tar.gz`，217351 bytes，目录 0700／文件 0600，实际 SHA-256 `c9cf429927ab070cca49b79c0d7387af24973c7920a07751329f6b2737a0b614` 相等，私有内容不入仓库。

13 库数据层恢复**不是完整 Suite 恢复运行**。私有 PG、restore20 保持停止，本地零 Docker；运维仍用 `/opt/omsbizfoundry/acceptance/oa-public-release-20261002a/d1-active` 保留既有 IAM override。

## 结论与后续门禁

本批关闭“当前管理员、同一普通项目、核心立项与编辑的连续路径”和此次 CSRF 阻断，不关闭完整 I01～I04。费率／甲方／资质完整字段、人员候选筛选、全角色／撤权／并发／部分成功、项目包、原站真人对照、后续经营／生产／资料统计、Evidence OSS、Web OSS 和正式制品仍待验；不先以新 B02 功能绕过第一步门禁。完整 OA 继续 `insufficient-evidence`。

证据：[真实点击及失败观察](../../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/browser-observation.json)、[最终原生检查](../../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/native-checkpoints.json)、[令牌更新与一次保存日志](../../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/gateway-renewal-trace.json)、[交付复读](../../acceptance/oa-initiation-repair/evidence/csrf-continuation-20261002/deployment-readback.json)。此前 `.3` 的导航故障、清除失败和锁屏阶段保留在[上一批报告](./oa-navigation-continuous-validation-20261002.md)，不覆盖成成功。
