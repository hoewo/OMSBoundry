# OA 立项确认丢失与抽屉恢复

日期：2026-10-02。沿用 BF-084、BF-087、BF-090、BF-091 和 ADR-0035；本切片不改变 Architecture Baseline 0.1。

问题 A → 可验证结果 B：后端已立项但回复丢失时，页面误称“尚未完成”，重新加载后旧抽屉又遮挡详情 → 明确提示“提交结果待确认”，保留输入；只在读取同一已立项项目的详情成功后关闭抽屉。

不变量：只改 Workbench；不改 Project API、schema、事实所有权、IAM／Gateway 契约、Grant、会话期限或幂等规则；不自动重试写入、不另建项目、不提前关闭读取失败的表单。不负责完整字段、全部角色矩阵、后续经营生产或整站验收。

## 实际发现与候选验证

在单个内存型、明确 opt-in 的隔离 Go UI probe 中，用原生 Chrome 点击。没有真实 IAM／数据库、没有本地 Docker；该故障开关只在测试代码中。

1. 原版本：后端已 active，却显示“立项尚未完成”。显式重试复用原命令，两个 activation 请求只有一次实际立项，派发和资料保存各一次。
2. 只修提示后的第一次复测：正确显示“提交结果待确认”，输入保留；点“重新加载资料”读取到同一项目详情，但旧抽屉仍遮挡。此失败保留，不当作通过。
3. 两项修复后的复测：结果待确认提示正确；原生重新加载关闭旧抽屉，进入同一项目详情。派发、资料保存、立项和 activation 请求各一次；恢复没有重放业务写入。

Go probe 退出正常不等于界面验收成功；第二次仍明确失败。第三次才确认抽屉恢复可用。三个 probe 已正常停止，临时浏览器标签已关闭。

本切片实际前端检查 59 项零失败／skip（runtime 44、目录 7、CSRF 8）；Workbench Go test／vet、JavaScript／Python 语法、Python 纯验收门禁 6 项和 diff check 通过。原生重试证据来自第一次 probe，新版本同命令重试另有单元检查；不声称新版本又跑了一次原生重试。

## 发布与公网验证

`v0.1.0-oa.20261002.5` 已串行部署到 Auth58 Main，只更新 Workbench，Project 沿用 `.2`。这是 development/test 制品、dirty source commit unknown，不是正式 clean-HEAD 发布。

- Source SHA-256 `e9f6d8881a35b9c124ba17155f59977b8428c4348559c39ab4397e56c1fe09ea`；Workbench binary `af9ff84577e9798ed332e124c336ba878b35f23889071acd872b2a1b007df8d8`；runtime `addcc1b3c39d495e54f354a02946f63e314c81cba4ed614b72f91f9e3fdf073b`。
- ACR digest `54b721cb9314590dc9cc6debbc57e3851d566f4f1ce476e24a088545d83a806e`；publication 拉回核对字节和发布期间 Main 不变通过。
- prepare → 13 库联合冷备份 → 13 空库事实／序列及文件／IAM key 字节恢复 → apply 通过；只重建 Workbench，其他 16 容器 ID 不变，17/17 healthy，Project schema 10、IAM／Gateway／Suite／Grant 不改。
- ECS 备份 `/var/backups/omsbizfoundry/oa-public-20261002e`；离机副本 `/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-ack-backup-20261002.oOCvd3/oa-public-20261002e-backup.tar.gz`，218197 bytes，0700／0600，实际 SHA `5dea7695a8df26b1af0f9f5aea75b5307ba84371852ee9ef529f82bf2719f36e` 相等。私有内容不入仓库；13 库数据恢复不是完整 Suite 恢复运行。
- 首次 staging 因 `/opt` 无普通用户写权限而拒绝，未写 Main 或创建目标；核对不存在后仅对精确新批次目录使用已有 sudo 权限，重新 staging 成功。部署各阶段无失败，不覆盖旧批次。

实际公网 native Chrome：普通刷新工作台；用已激活的同一 C 旧立项深链进入基本详情，URL 去掉 `view=initiation`，没有完成抽屉；点击返回保留 C 搜索条件，Enter 编号搜索唯一 C，重新打开及刷新后的资料和两类负责人保持。未点击业务提交按钮、未制造公网故障或新建夹具。

`post-ack-message-release` 只读复读实际通过：精确 `.5` 镜像与 runtime 相等，C 仍 active revision 7、四条分工、七条历史、九个回执，与原 `final` 全部相等。全部既有 Project 行、用户101及 A／B、Grant 摘要和其他 11 业务库仍等于原 baseline；业务 API／Grant 写入零。没有覆盖原 `final` 或前次失败。原生浏览器后再次运行 `a/d1-active check`，exit 0；17/17 healthy，私有 PG／restore20 停止，本地零 Docker。Evidence 的实际 provider 仍 local，不假称 OSS 链路通过。

## 当前结论

本批已关闭隔离故障模拟中的错误提示与旧抽屉遮挡，并通过同一公网项目的恢复导航和冻结事实检查。未知回复模拟没有在公网制造；不能把不同证据层混成公网故障复现通过。不能据此宣布完整 I01～I04 或完整 OA 通过；完整 OA 仍 `insufficient-evidence`。费率／甲方／资质等完整字段、角色／撤权／并发、项目包及后续页面继续按第一步门禁推进。

下一字段切片的只读核对：原 `ProjectInitiation.tsx` 291～351 回填并提交日期、甲方名称、资质单位和管理费率；701～731 的两类名称还在失焦时保存，不能把“取消完成抽屉”一概说成这些名称零写。当前克隆三项仍禁用；Project 的 Counterparty Context v1 写命令只允许 active 且名称非空，不能直接伪装成原待立项可清空／失焦保存行为。后续必须先确定兼容命令、字段所有者、精确费率和分段失败恢复，不能用页面暂存、Party 主档或浮点公式补齐表面。

证据：[隔离故障与三次浏览器观察](../../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/synthetic-browser-observations.json)、[真实公网点击](../../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/public-browser-observation.json)、[原生只读复核](../../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/public-native-readback.json)、[交付复读](../../acceptance/oa-initiation-repair/evidence/acknowledgement-recovery-20261002/deployment-readback.json)。前批 CSRF 和完整核心连续旅程仍见[.4 报告](./oa-csrf-continuation-validation-20261002.md)，不撤销其有限通过范围。
