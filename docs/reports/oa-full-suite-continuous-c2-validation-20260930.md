# OA 完整 Suite 连续旅程 C2 验证

日期：2026-09-30  
状态：`passed-in-current-reference-scope`  
总体阶段结论：仍为 `insufficient-evidence`

## 1. 问题与结果

问题 A：C1 的同一 Tenant／Project／Party 已成立，但历史 C2 证据来自另一组夹具，不能证明同一家公司的一项业务能从项目责任连续进入经营承接、主体、合同、结算与资料组合。  
可验证结果 B：在公司 Ubuntu 的隔离完整 D1 栈中，沿用 C1 的稳定 ID，完成 C2 正常、拒绝、故障、恢复、重放和只读组合，并证明每个应用只增加自己拥有的事实。

本轮结果为通过。它关闭连续旅程 C2 和 Business Engagement 间接资料运行链，不代表 C3～C8、双 Tenant、真人浏览器或公网 HTTPS 已通过。

## 2. 环境与固定输入

| 项目 | 事实 |
| --- | --- |
| 目标 | 公司 Ubuntu D1 隔离恢复栈 `omsbizfoundry-restore20` |
| 入口 | SSH 隧道后的 Nginx `http://127.0.0.1:18120` |
| 长期服务 | 验收前后均为 17/17 healthy |
| Tenant | `6539cd13-7190-40f1-8bd5-57f9e7c9b968` |
| Project | `7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，C2 后 revision 7 |
| C1 客户 Party | `320fdfee-8c96-4f7c-aa50-58d3c79eaae1` revision 1 |
| 证据根目录 | `/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1` |

证据目录中的私有 actor 状态保持 `0600`，本报告不复制密码、Session、Token、私钥或环境文件。

## 3. 连续执行事实

1. 重新读取 C1 state，复核 Tenant、Project、客户 Party、三项 Assignment、登录与各应用授权，并在写入前保存所有权计数。
2. 在 Evidence Service 写入投标与中标两份测试材料；两者均为稳定 File ID、version 1、revision 1、`available`。
3. 在 Party Directory 新建供应商；Contract 只建立来源自有本地快照和两条人工确认映射，再以追加式 transition 切到 directory runtime。
4. 在 Contract 建立 active 合同，并以客户／供应商精确 Party 映射创建 `50000.000000 CNY` 的 Settlement；Contract 没有接收或复制 Business Engagement 事实。
5. Project 只新增 Contract Resource Link；随后使用 `project-workspace` workload credential 读取 Contract 投影，精确 Scope 与 IAM audit 均存在。
6. Business Engagement 固定当前 Project revision，形成 `applicable → comparison → bid material → award material → ready`，最终 revision 5；一次 stale expected revision 返回 409 且历史不增。
7. 只停止 Evidence Service。award material 返回 503、retryable，Business Engagement 保持 revision 4 且只有一份有效材料；恢复 Evidence 后重放原 Command，恰好提交一次，再重放仍保持 revision 5 和五条历史事件。
8. Project 新增 Business Engagement Resource Link，并通过正式 `/materials` 运行链先取 BE current-only material projection，再为两个 File 分别换取 Evidence metadata credential；最终得到两个 `available` 分段，保留 BE revision／digest 与 Evidence revision／digest，且没有暴露 bucket／object key。
9. 复核 IAM audit、数据库所有权增量、C1／C2 state 摘要和 17/17 健康状态后落盘 `state-after-c2.json`。

## 4. 关键事实 ID

| 事实 | ID／结果 |
| --- | --- |
| 供应商 Party | `af8827ab-7940-4110-be11-c359d23c7047` revision 1 |
| 投标材料 | `e6b00b0d-0636-411d-8ebc-7bb16efe5b75` version 1 |
| 中标材料 | `40973231-38db-4f02-804b-5f02b99096b1` version 1 |
| Business Engagement | `0b2dc7c1-2cca-46e2-823f-6522c44f9683`，`ready` revision 5 |
| Contract | `d784e608-1388-416d-bf24-b5321182fee4`，`active` revision 1，directory runtime |
| Settlement | `39ddaf20-035a-412f-a26e-eb0bcecdac04` revision 1 |

## 5. 数据所有权增量

| 所有者 | 本轮精确增量 |
| --- | --- |
| Party Directory | Party +1；revision +1 |
| Evidence Service | File +2；version +2 |
| Business Engagement | Engagement +1；material +2；history event +5 |
| Contract Ledger | Contract +1；local Party snapshot +2；Party mapping +2；runtime transition +1；Contract revision +1；Settlement Party +2；Settlement +1 |
| Project Workspace | Project +0；Resource Link +2 |

该增量与预期完全一致。未发现 BE 写 Contract、Project 复制来源事实、共享业务表、跨库外键或共享 ORM。

## 6. 本轮暴露并修复的发布缺口

### 6.1 Project 缺少 Business Engagement 端点注入

首次最终资料读取为 503，且 IAM audit 中没有 BE workload exchange。最小原因定位到 D1 Compose 的 Project 环境未注入 `BUSINESS_ENGAGEMENT_BASE_URL`。补齐配置并只重建隔离 Project 后，错误从 503 前进为 422，证明第一层根因已关闭。

### 6.2 已部署 Business Engagement 镜像不含当前 reference API

422 时 IAM audit 已出现精确 BE credential。使用不打印 Token 的定向 reference probe 直接访问目标容器，得到 `/api/v1/references/business-engagements/{id}/materials` 404，证明请求已到目标但运行镜像落后于当前契约。只构建并推送 Business Engagement `linux/amd64` 镜像：

```text
crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/business-engagement:v0.1.0-test.27
sha256:6b133ac3f9d1e16daef713d43fd2afff77cefac3b4a0408e2f9abf80e5f28fc7
```

隔离栈只重建 BE 后，运行链进入成功路径。该事实再次支持 ADR-0035：发布必须按消费者配置、API 契约和 migration 的兼容闭包，而不是只看被编辑的服务。

### 6.3 验收驱动缺陷

驱动还修正了两个不属于产品失败的问题：重放后 Project revision 可为 6～7；实际 `text/plain; charset=utf-8` 应按媒体类型族校验而不是与 `text/plain` 字符串全等。两项均先由服务端事实确认，再收紧为不放宽业务约束的断言。

## 7. 发布制品链

- `test.27`：补齐 Project 的 BE 端点配置并通过升级预检；
- BE `v0.1.0-test.27`：单独的新运行镜像；
- `test.28`：固定新 BE 摘要的完整 image manifest；
- `test.31`：封装最终 C0／C1／C2 verifier、reference probe 源码和 Compose 配置的验收候选。

这些都是 development validation candidate，不是 production release。主运行栈未被破坏性故障注入；故障只发生在隔离恢复栈。

## 8. 架构判断

本轮支持以下当前范围结论：

- `Party + Contract + Business Engagement + Evidence + Project` 可以用版本化端口和显式 Resource Link 连成真实业务链，无需共享表；
- 客户／供应商当前档案、合同内快照、经营材料引用和项目只读组合分别由正确应用拥有；
- workload credential 可以按目标应用、动作和精确资源逐段签发，不需要 IAM 转发业务请求；
- 一个来源停机可以失败关闭并零写入，恢复后由同一 Command 安全继续；
- “尽量只做加法”的 Base／Overlay 方向在 C2 没有出现必须长期 fork 公共内核的反证。

同时，本轮明确了这种解耦的真实成本：消费者端点、目标 API、Capability 与运行镜像必须作为同一兼容闭包验证。这个成本不能用共享表规避，应由 release manifest、预检和连续旅程自动化承担。

## 9. 仍未证明

- 同一数据集的 C3～C8；
- C2 完整双 Tenant／跨租户矩阵、组织代理和全部字段／金额动作；
- C2 到 Money 的所有可选业务建议；
- 真人浏览器完整连续点击；
- 公网 HTTPS、离机备份与跨机器恢复；
- 第二真实客户下 Base／Overlay 的长期升级稳定性。

下一步在同一 Tenant、Project 和 C2 state 上执行 C3，不创建新 Project 掩盖连续性问题；仍保持一次一个阶段、一次只停止一个来源。
