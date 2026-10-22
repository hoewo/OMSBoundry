# Project Workspace

Project Workspace 拥有 Project、项目 Assignment、类型化 Resource Link、可选客户 Party 关系、幂等命令和不可变历史。它不拥有 Party 当前档案、人员名册、合同、资金或文件事实。

## 本地启动

```bash
docker compose up -d postgres
cp .env.example .env
# 替换 .env 中的 TEMPLATE_ACCESS_TOKEN，并按需要配置 People 等引用端点。
set -a; source .env; set +a
go run ./cmd/server
```

- HTTP：`http://127.0.0.1:8086`
- PostgreSQL：`127.0.0.1:55437`
- 健康检查：`/health/live`、`/health/ready`
- Project API 前缀：`/api/v1/projects`
- 建项前合作主体候选：`GET/POST /api/v1/project-party-candidates`。Party Directory 保持中性主体主档；Project Workspace 只记录该稳定 Party 可进入后续项目的 `client`／`contractor` 选择器，不把候选当作已发生项目关系。登记必须携带 Party 当前 revision 并使用租户级 `project-workspace.create`，读取使用 `project-workspace.read`。
- 初始派发：`POST /api/v1/initiation-dispatches`，名称＋版本化 Person 引用，草稿与经营 Assignment 原子保存；编号／类型在完成前补齐。需要相应创建／层级及经营分工能力。

- 已立项基本资料更正候选：`POST /api/v1/projects/{id}/profile-corrections`，显式 `project-workspace.profile.correct`。旧 PUT 仍只处理立项期；新动作保留稳定身份、状态和外部引用，追加更正原因及历史。见 [规格](../../../arckit/spec/project-workspace/profile-correction.md)。
- Project Package 创建：`POST /api/v1/project-packages`
- 两级转换：`POST /api/v1/projects/{project_id}/package-conversions`

Project Package 只保存两级层级与转换事实；它不聚合合同金额、资金或生产状态。层级命令需要 tenant-scoped `project-workspace.hierarchy.manage`，资源级 Project 凭证不能同时修改 Project 与 Package。

除健康检查和服务说明外，所有业务 API 都要求 `Authorization: Bearer <credential>`。本地模板模式仍执行 tenant、action 与 scope 鉴权，不提供关闭鉴权的配置。

完成立项的可选 `initiation_profile` 是有界资料门槛，不是授权：省略或 `dual-owner-v1` 保留双负责人；`business-owner-v1` 允许生产负责人后补、Package 编号可选。普通项目仍需正式编号和类型，旧 Create 保持严格。000010 仅放宽未激活派发草稿，不重写旧事实；这些源码增量不代表仿 OA 前端或主站已通过。

参考 OA 专业分工为默认关闭的有界候选：migration 000012 保留旧四角色／v1 引用，新增六角色、Tenant 专业目录及独立历史；配置 `REFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE=reference-oa-production-personnel-v1` 后还需通过受保护命令给指定 Project 选装。四种新 native read／manage action 不继承旧 Grant，People 仍是显式版本化依赖。原 UI 独立预览、真实 IAM／People／PG18 与单库恢复已分层验证，正常 Workbench／Main 尚未启用。见[规格](../../../docs/specs/assembly/oa-production-personnel-v1.md)和[实际报告](../../../docs/reports/oa-production-personnel-native-validation-20261004.md)。

## 验证

```bash
go test ./...
go vet ./...
go build ./...

# PostgreSQL 18 集成测试（先启动 compose 中的数据库）
TEST_DATABASE_URL='postgres://project_workspace:project_workspace_dev@127.0.0.1:55437/project_workspace?sslmode=disable' \
  go test -tags=integration ./...
```

Resource Link 使用目标服务的版本化 reference-read 契约。未配置或不可用的目标服务会返回 503，Project 自身的读取与历史查询不依赖这些服务在线。

C4 资料消费切片（源码与隔离 PG18 已通过、主站未发布）：Production 成果来源已可只读发布 → Project 增加 canonical Work Item 关联与两层 typed reader，按精确来源／File 工作负载能力实时显示资料。000008 只扩充 Project 自有 Link 约束；不复制生产／Evidence 事实、升级或猜测旧 Design Link，也不增加下载能力。可选 `PRODUCTION_BASE_URL` 从部署配置注入；正式 IAM 使用短期 Workload Credential，不接收长期 token。7→8 旧事实保持、错项目零写入、来源离线的本地幂等复读、资料 GET 零写入、drift／403／503／空源及关联空目标恢复后继续追加已实际通过；上游为显式 HTTP 测试提供者，不冒充完整正式跨进程链。见 [规格](../../../docs/specs/assembly/production-material-reference-v1.md)和[消费者实测](../../../docs/reports/auth58-project-production-material-consumer-validation-20261001.md)。

普通 Project 可在创建时提交 `client_party.party_id + source_revision`。服务通过 Party Directory 精确验证后只冻结 ID、revision、kind 和 legal name；Party 后续变化不改写 Project 历史。不选择客户时无需配置 Party；选择时 `PARTY_BASE_URL` 不可用会让本次创建失败关闭。首个切片不允许通过基础资料更新重选客户。

OMS 一期需要一个项目关联多个施工方时，继续复用 Project 自有的类型化 Resource Link：`target_service=party-directory`、`resource_type=party`、`relation_key=contractor-party`，且必须携带 Party 当前 revision。Project 在写入前使用 Party Directory 的精确 reference-read 复验；只保存 Party ID／revision 和项目内关系，不复制或拥有主体主档。`GET /api/v1/project-directory` 同时返回授权范围内项目的当前有效 Resource Link，供工作台按稳定 ID 形成甲方／施工方工作视角；不按名称猜角色，已解除关系不进入当前目录。

## 负责人选择界面切片

问题 A：正式页面要求用户手填 Person／Group UUID 和 revision，普通用户无法完成责任装配。可验证结果 B：从 People & Groups 现有受保护列表选择 active 人员或编组，提交稳定 ID 和当前 revision，由 Project 服务再次验证并形成自己的 Assignment 快照。

界面保持现有蓝色、字体与平静工作区布局；顺序为角色、人员／编组、搜索选择、来源提示、添加负责人。类型切换清空旧选择，加载期间禁用提交，错误显示可重试提示；本切片不增加装饰动画或新依赖。

- 浏览器经 Gateway 换取已有 People 读取能力；不签发凭证、扩展 IAM 授权或直读 People 数据库。
- 正常选择只保存候选 ID、revision 和显示名，不保存 IAM Subject 或密码；不按姓名匹配业务身份。
- inactive／archived、错误 Tenant、畸形或重复 ID 候选不得提交；Project／类型切换或权限收回后，迟到响应不得恢复旧选择。
- 手填 ID 仅为明确选择的高级排查模式，独立本地模板仍可使用；它也必须经过原服务端验证，目录失败不自动退回此模式。
- 验收包括嵌入 JS 回归、Project 普通 Go 测试、正式页面真实选择与数据库后检；未运行的项目不记为通过。
- 不修改 People 档案、IAM 账号、Project 立项规则、API 或 migration；大名册分页／远程搜索、其他业务选择器和完整 OA 验收不由本切片承担。

后续布局切片：权限标签挤窄台账新建按钮 → 把技术动作列表放入可展开的次要说明，新建按钮保持完整横排文字。只调整页面 HTML／CSS；验收看正式页面按钮、展开说明和当前项目复读，不能据此声明全站 UX 通过。

弹窗取消切片：未填必填项时“取消”被当作提交并触发校验 → 独立的非提交按钮直接关闭所属弹窗，不触发 API、保存凭证或业务命令。覆盖新建项目、新建项目包、本地凭证与撤销确认四类弹窗；真正提交仍保留必填校验。验收包括实际嵌入处理器与正式浏览器空表取消，取消不删除已确认事实，不由此扩展到其他应用页面。

## 发布、备份与恢复

```bash
# 生成含 server、ops、配置样例、契约和迁移说明的校验发布包。
GOOS=linux GOARCH=amd64 ./ops/build-release v0.1.0 /absolute/output

# 本地 compose 数据库备份；输出是 PostgreSQL custom archive。
./ops/local-pg-dump --format=custom --no-owner --no-privileges > /tmp/project-workspace.dump

# 只向新建的空数据库恢复。恢复前不得在目标库执行 migration。
PGDATABASE=project_workspace_restore ./ops/local-pg-restore \
  --exit-on-error --no-owner --no-privileges --dbname=project_workspace_restore < /tmp/project-workspace.dump
```

发布包不会携带 `.env`、凭证或数据库内容；公司环境必须重新注入 IAM／引用端点和数据库配置。备份恢复是数据库整体迁移，不改变 Project ID、revision、Assignment、Resource Link 或历史事件。
