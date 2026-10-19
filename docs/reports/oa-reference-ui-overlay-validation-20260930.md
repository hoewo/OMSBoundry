# 参考 OA 界面 Overlay 验证记录（2026-09-30）

状态：`source-level-reference-ui-deployed / backend-continuous-acceptance-still-insufficient-evidence`

## 要验证的问题

问题 A：统一工作台虽然能够登录和跳转各应用，但界面表达仍是技术装配台，无法让用户按参考 OA 的项目业务方式理解和操作。

可验证结果 B：在不改变 Nginx、Gateway、IAM、应用数据所有权和既有业务 API 的前提下，统一工作台采用参考 OA 的登录页、左侧业务导航、项目列表、项目详情和业务分区，并继续通过真实 IAM 登录与 Gateway 授权进入各应用。

## 本轮实现

- 登录页不再按截图近似重画，直接复用参考 OA `Login.tsx` 的 class／DOM 结构、`Login.css` 和原始品牌图；登录协议仍使用现有 IAM 的用户名／密码，不伪造参考系统的短信验证码能力。
- 登录后直接复用参考 OA 的 `App.css`、`AppSidebar.css`、`ManagementSectionTabs.css`、`ProjectList.css`、`ProjectStatusBeads.css`、`ProjectDetail.css`、`ProjectWorkspace.css`、`BasicInfoTab.css`、`BusinessInfoTab.css`、`FileManagement.css` 和 `Statistics.css` 页面契约；静态服务只增加 Ant Design DOM 兼容层。
- 登录后固定为与参考 OA 相同的项目型信息架构：项目立项、项目管理、项目资料管理、项目统计信息。立项、项目列表、资料表和统计台账均使用参考页面的原 class 结构；项目详情使用相同的顶部信息条、三级主导航、工作区摘要和领域卡片。
- 已装配应用仍由 Suite Manifest 决定，业务入口继续提交到真实 `/gateway/authorize/<application>`，不以静态页面绕过 IAM 或 Gateway。
- 原有个人工作和指标读取继续调用现有版本化接口；退出登录、CSRF 和授权表单继续使用原有服务端契约。
- 示例项目和部分统计数明确标注为“界面验证数据”或“演示数据”，不冒充已经写入各业务数据库的 OA 事实。

## 验证结果

- `frontend/oms-workbench` 执行 `go test ./...`：通过。
- `frontend/oms-workbench` 执行 `go vet ./...`：通过。
- 运行时 JavaScript 执行 `node --check`：通过。
- 浏览器实际检查：登录页、项目立项、项目列表、项目资料、项目统计、项目详情和详情主 Tab 均可显示与切换；宽屏工作区铺满剩余内容区，未再回退到旧通用后台页面。
- 公司 Ubuntu D1 只替换无状态 Unified Workbench 容器，其他业务服务和数据库未重建；替换后 `/health/ready` 返回 `ready`。
- 实际发布镜像：`crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/unified-workbench:v0.1.0-test.42@sha256:5f9a74bbcac129fa7499153bb62a32d303cd162350642c7fbb29091607cdfb3f`（Linux/AMD64）。
- 更新前环境文件备份：`/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.before-workbench-ui42-full.env`。

## 结论边界

本轮关闭的是“登录页和工作台看起来不像参考 OA”的界面问题，不等于完整 OA 已经通过最终验收。

已经成立：参考 OA 的主要视觉层次和项目型操作骨架已进入真实 Workbench；登录、授权跳转、个人工作和指标读取仍连接现有正式链路。

尚未成立：界面中的示例项目、部分筛选／新建／保存动作尚未全部绑定 Project 及其他领域应用的真实写入 API；连续 C8 暴露的 Project Link、资料投影和精确 Evidence 权限缺口也仍需修复。因此总体架构验证报告继续保持 `insufficient-evidence`，不能用本轮视觉完成度替代业务验收证据。

## 下一步验收

1. 使用真实 IAM 账号登录已部署入口，检查 Suite Manifest 对应的应用入口、菜单权限和登录后回跳。
2. 把项目列表和项目详情逐项换成 Project Workspace 及各业务应用的只读投影，去掉相应界面验证数据。
3. 按最小纵向闭环逐项接通项目立项、经营、生产和资料动作；每接通一项就验证允许、拒绝、撤权、依赖停机和恢复。
4. 修复连续 C8 的 Project Link／资料投影和 Evidence 精确能力缺口后，重新运行完整连续门禁并更新最终验证报告。
