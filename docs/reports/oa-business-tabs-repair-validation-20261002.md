# OA 项目详情页签修复验证

日期：2026-10-02。关联 BF-092。

问题 A → 可验证结果 B：真实详情仅接通甲方，其他页签硬禁用，用户无法继续检查 → 同一项目内切换九个经营栏目与生产信息，逐栏目读取已有原生事实，明确区分未接通、缺引用、缺权和读取失败。

## 范围和约束

原 OA `BusinessInfoTab.tsx` 的栏目键和同项目切换作为依据。保留甲方行内录入；没有把原 Demo 的金额、人员或假提交接进正式项目。未保存的甲方编辑要求先保存／取消，迟到响应不得串栏目或项目，刷新／Back／Forward 使用 URL 恢复。

Workbench 不拥有事实，商业分段使用 Project `commercial-summary` 及独立读取能力，承接／生产只读取 active 精确引用，成本／奖金用来源精确 Project 查询。金额未知不按零显示、不混合计算；成本缺经营／生产归属，不能冒充经营合计。经营参与人类型未闭合，不列负责人或生产 participant 假充。

## 当前验证

候选 77 项 Node 检查通过，包含 18 项新读取／交互单元检查；Workbench Go 初次回归发现隐藏技术应用名被路由配置带入页面，缩小为实际四种业务来源后全量 Go 回归通过，vet 通过。原站当前部署未现场执行；目前依据原仓库源码，不能宣称完成原站流程验收。

本批 `.6` 已通过 ACR publication／pull／二进制字节核对、preflight、13 库联合备份及逐个空目标恢复；恢复后事实／序列、Evidence／IAM 文件与 key 字节一致，Project schema 10 未变化、重复 migration 幂等，私有 PG 已停止。不是完整 Suite 恢复运行。首次 preflight 在 publication 最终 passed 之前被门禁拒绝，未创建备份或修改 Main；publication 完成后重跑通过。

正式 rollout／D1 check 通过，只替换 Workbench，其他 16 容器 ID 不变，17/17 healthy。公网原生 API 复核两份 runtime 精确摘要、12 业务库和 Grant 均与本批联合备份保持；现有用户 Project101 已由用户完成，不再拿旧待立项快照作基线。本项目商业 segment 为 0、当前账号 Cost／Bonus 授权 403；这两类事实不表示来源为空。当前环境未找到 active 商业链接项目，不能声称已用公网有金额记录验证合同／资金／开票展示。

真实 native Chrome 已逐个点击九个经营页签及生产信息，确认同一 Project、选中态与标题／内容一致、不跳另一系统；奖金栏目刷新、Back 到总览／Forward 回奖金均保留正确项目和栏目。甲方编辑打开后切合同被明确提示阻止，取消后切换成功，没有输入或提交业务写入。合同页实际截图检查未发现旧抽屉或限高遮挡。观察见[原生点击记录](../../../acceptance/oa-initiation-repair/evidence/business-tabs-20261002/browser-observation.json)。只证明导航／错误边界，不证明待实现表单或完整 UI 等效。

离机备份已保存到 `/Users/crispydog/Documents/aliyun/zhangfeng-antuiinfo/oa-business-tabs-backup-20261002.72gVZ0/oa-public-20261002f-backup.tar.gz`，目录 700／文件 600，219512 bytes，SHA-256 `4e6e32e032399c57c5165cd30849d69fa22ce578c6a3f00e63d9026660df653c` 与远端一致；私有归档不入 Git。点击后的第二次原生／事实复核通过：12 业务库、Grant 与本批备份一致，16 未替换容器保持，17/17 healthy；随后 `a/d1-active check` 再次 exit 0，Evidence provider 仍 local，私有 PG／restore20 保持停止。本批未启动本地 Docker。见[部署与复读证据](../../../acceptance/oa-initiation-repair/evidence/business-tabs-20261002/deployment-readback.json)。

测试制品 `official_release=false`，dirty workspace 没有伪称 clean-HEAD。Workbench binary SHA-256 `8b40742c529c4a20d2f4d7086c16f406a6b28b6d6cc8fdf1a48f47cd9f479e3f`，source archive `acdf684aa4f6c9ea22559d00b438d0c690ebe7deeb1780f6fe363fde89368a44`。ACR Workbench digest `sha256:dd3a56c9ba07d140b8b723d7e7d010150cccd1fbd62f614335fead5700948b7b`；Project／IAM／Gateway／Grant／schema 不进入本轮变更。

## 未完成

只读定位：Suite 已启用 Cost／Bonus，Gateway 分别要求 `cost-entry.read`／`bonus-pool.read`，IAM 当前两个 audience 的有效 Grant 查询为空。403 是真实授权装配缺口，不是页面无响应；本批不自动添加 Grant，也不借 Workbench 服务身份读取。后续需明确测试账号的原生只读／金额／写入能力矩阵再配置，不能直接给全量权限。

当前管理员 Cost／Bonus 原生来源授权仍被拒绝，没有为了通关新增 Grant；授权装配需要单独核对。经营参与人的添加／移除与分权；招投标新增／材料／适用性／方式编辑；合同及补充协议编辑；支付／开票写入；经营成本归属与录入；奖金计算／分配／发布；分类型生产实际录入。这批修复只读页签，不是上述完整业务或完整 OA 通过。原站部署现场、全部角色矩阵与有真实来源记录的公网浏览器验证仍未完成。
