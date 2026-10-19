# OA 完整 Suite 连续 C8 验证报告

日期：2026-09-30  
状态：`insufficient-evidence`  
连续门禁：`C0～C7 = 8/9 passed`，`C8 = failed with retained evidence`  
适用范围：参考 OA 完整体第一阶段、同一 Tenant／同一 Project 连续旅程中的资料、指标、个人工作与只读来源故障切片

## 1. 验证问题

问题 A：C1～C7 已分别形成主体、人员、项目、承接、合同、资金、生产、批复审计、成本外委和奖金事实，但这些事实存在并不等于最终使用者能在一个 Project 中看到完整资料、只看到自己的工作、按权限查看金额，并在一个只读来源故障时继续使用其余组合。

可验证结果 B：Project 只读组合 C2～C6 的 active Resource Link 与 Evidence metadata；Evidence 下载必须独立授权；Workbench 只返回当前 Subject 对同一 Project 的工作项；Money 指标按来源／Party／Currency 分区且撤权后返回 `null/redacted`；逐个停止只读来源时组合保持可用并恢复；全部读取和故障恢复不得改写十二个事实所有者。

本轮不验证公网 HTTPS、第二 Tenant、持续并发负载、第二客户、D2～D4，也不把“已有页面”或“17 个容器 healthy”单独当作 C8 通过。

## 2. 环境与连续基线

- 目标：公司 Ubuntu ECS，隔离 Compose project `omsbizfoundry-restore20`；
- 入口：回环地址 `http://127.0.0.1:18120`，链路为 Nginx → Gateway → IAM／目标应用；
- 长期容器：17 个，验证结束为 `17/17 healthy`；
- Tenant：`6539cd13-7190-40f1-8bd5-57f9e7c9b968`；
- Project：`7a3a222d-ebfa-45d3-a6c9-ab44e6a1651b`，revision 12；
- C7 输入状态 SHA-256：`af11c4b6df2a14943242e2f0c6e5e11075d0231b251b6c70cac4c7d1843d95e3`；
- C8 输出状态 SHA-256：`67c184d6384c7f38e54bf5750f78f00e00cb84e7ed0fbc2a48e8cf5245d88c4a`；
- 十二库读取前事实摘要清单 SHA-256：`05491ab3ba29b3b0b93aa9cb696c9f5d485bc504f5769298cafa52f3c20ac92e`。

正式证据目录：

```text
/opt/omsbizfoundry/validation-evidence/reference-oa-continuous-v1/c8
```

验证器为 `acceptance/d1-full-oa-journey/verify-c8`。它要求显式设置 `OA_JOURNEY_C8_CONFIRM=yes`，一次只停止一个来源，并在退出 trap 中恢复来源、撤销临时能力和落盘证据。

## 3. 科学验证步骤

1. 复核 C1、C2、C4、C5、C6、C7 的状态均属于同一 Tenant／Project，并从服务端复读保留的 C4 设计工作项；
2. 不创建第二个 Project 或重复生产类型，把已完成 C4 工作项的 `construction-support` 以 append-only follow-up 重新打开为 revision 9，形成一个可验证的当前个人工作项；
3. 仅向保留的 participant／neighbor Actor 发放 C8 所需 Project、Evidence、Money 和 Production 读取能力，并重新走登录与 Gateway authorization；
4. 经正式入口读取 Project detail、Project materials 和全部 active Resource Link，逐阶段核对 C2～C6 输出 ID；
5. 先只给 `evidence-service.metadata.read`，再增加 `evidence-service.download`，最后仅为诊断增加遗留 `evidence-service.view`，比较三次 Gateway／Evidence 结果；
6. 让 C1 production Subject 读取个人工作项，要求恰好命中同一 Project 的保留工作项；让相邻 Subject 读取，要求返回空列表；
7. 在无金额能力、旧凭证、授权刷新、撤权残留窗口和撤权刷新五个时点读取 Money 指标，核对分区、值、摘要和脱敏状态；
8. 对十二个业务数据库的所有 public 事实表按表名和 canonical JSON 行排序，保存读取前完整 TSV、行数和 SHA-256；
9. 只停止 Evidence，要求 Project materials 仍为 200 且对应分段为 `unavailable/source_unavailable`，Workbench 仍为 200；恢复 Evidence 后材料重新 available；
10. 只停止 Production，要求 Workbench 仍为 200、Production API 为 502/503；恢复后同一 Subject 的工作项重新可读；
11. 再次保存十二库完整 canonical TSV，逐文件和摘要逐字节比较；
12. 核对 49 个 HTTP 断言无偏差、17 个长期容器全部 healthy，再按规范退出门槛计算最终结果。

## 4. 通过的切片

| 验证项 | 结果 | 直接证据 |
| --- | --- | --- |
| 同一连续身份 | `pass` | 同一 Tenant、同一 Project revision 12；未新建第二个 Project |
| C2 资料运行投影 | `pass` | 两个 BE→Evidence 间接资料段均 available，包含精确 version／digest，不暴露 bucket／object key |
| 当前 Subject 个人工作 | `pass` | production Subject 仅得到工作项 `ea4b71df-f19e-4bed-a3ba-bc4f4c0e0bc9`，相邻 Subject 得到空列表 |
| Money 指标分区与脱敏 | `pass` | 无金额权为 `null/redacted`；fresh grant 后为精确字符串金额；revoke＋refresh 后重新为 `null/redacted`，不是 0 |
| 单来源降级与恢复 | `pass` | Evidence 停机时 Project／Workbench 200；Production 停机时 Workbench 200、目标 API 503；两者恢复后重新可读 |
| 事实所有权不变 | `pass` | 十二库完整 canonical rows 前后逐字节相等，摘要清单相同 |
| HTTP 状态 | `pass` | 49 个断言：24×200、12×204、10×303、2×401、1×503，偏差 0 |
| 最终健康 | `pass` | 17/17 healthy |

十二库基线行数为：People 204、Party 36、Project 177、BE 70、Contract 36、Money 71、Production 73、Results 11、Billing 20、Cost 24、Bonus 31、Evidence 117。前后 TSV 全部相等，证明本轮读取、分权和顺序故障恢复没有暗写业务事实。

退出清理后又直接查询 IAM 两类 Grant 表：本轮 participant／neighbor 的 Project、Evidence、Money、Production 临时动作活动数为 `0`；远端隔离栈仍为 `17/17 healthy`。

## 5. 阻断 C8 的真实缺口

### 5.1 C4～C6 输出没有成为 Project active Resource Link

当前 Project 只有六条 active link：C2 Contract、C2 Business Engagement、C3 Money Obligation、C3 Money Transaction、C3 Billing Invoice，以及 C7 为奖金来源追加的 Settlement。C4 的 Design／Consultation、C5 的 Approval／Audit、C6 的 Cost／Outsourcing Engagement 均不存在 active link。

因此 Project 无法仅凭版本化 Link 发现这些成果，更不可能在资料页组合它们。当前 materials resolver 又只消费 `evidence-service/evidence-file` 与 `business-engagement/business-engagement`，即便补 Link，也还需要明确每类业务成果到 Evidence metadata 的只读投影契约。

这不是“页面漏画一块”，而是装配合同未完成。修复必须由各事实所有者提供稳定引用／资料投影，Project 只保存版本化 Link，不得读取其数据库或复制其领域模型。

### 5.2 Evidence 下载能力在 Gateway 入口被遗留 `view` 绑住

Evidence 目标服务已把 metadata 与 download 作为不同动作，下载 handler 接受 `evidence-service.download`。但 Suite Manifest 为 Evidence 配置的 Gateway `entry_action` 仍是遗留 `evidence-service.view`。

实测：

1. 仅 metadata：下载授权为 401；
2. metadata＋download，并刷新应用凭证：仍为 401；
3. 再增加遗留 view 并刷新：下载授权才为 200。

因此“下载另行授权”在目标服务内部存在，但从公网唯一入口无法独立表达。把 view 一并授予会扩大能力，并使下载分权失去意义。C8 必须判为失败。

修复方向应是让 Gateway entry 只承担“可进入该应用”的最小能力或支持动作级入口，而不是要求业务页面以更宽的遗留动作解锁所有 Evidence 路由；目标 Evidence 仍负责最终 action／file scope 授权。

### 5.3 Evidence 人类页面发送了契约不支持的 delivery mode

源码页面原先发送 `delivery_mode: redirect`，而 Evidence v1 OpenAPI 和服务实现只接受 `blob/direct`。这会让真人点击“查看／下载”得到 422。本轮验证器先复现该结果，再按正式契约改用 `direct` 完成后续诊断。

源码已改为 `direct`，并增加 `TestEvidenceDemoUsesSupportedDirectDeliveryMode`；模块测试通过。该修正尚未成为本轮服务器上运行的固定摘要，因此只能记作“仓库修复已验证”，不能反写成本轮真人浏览器已通过。

## 6. 验证器失败尝试与收缩

为了不把验收器错误误判成架构结论，本轮保留了四次收缩：

1. 最初试图新建第二个 `design-v1` 工作项，命中服务正确的 `ACTIVE_WORK_ITEM_CONFLICT`；改为在同一 C4 工作项上追加 follow-up，不改变 Project／类型边界；
2. 下载 payload 使用页面中的 `redirect`，服务正确返回 422；对照 OpenAPI 后改为 `direct`，并把页面错误登记为产品缺陷；
3. 验收器最初期望下载授权返回 201，但 OpenAPI 和 handler 明确为 200；只修断言，不改服务；
4. coverage artifact 最初从不含 `project_id` 的 link JSON 反推 ID，得到 null；改为使用已验证的连续 Project ID，并重跑完整 8 步。

最终状态来自修正后的完整重跑，不使用任一中途失败结果冒充通过。

## 7. 发布与静态边界审计

- `architecture-boundary-static-audit` 与其负例 self-test 均通过：15 个应用、跨应用源码导入 0、跨应用本地 replace 0、跨库 SQL 机制 0；
- D1 release bundle 审计发现 C7／C8 verifier 未随包交付，现已把 `verify-c7`、`verify-c8` 纳入构建、权限和固定清单；
- `PASS D1 release bundle test`；
- `PASS D1 upgrade selection test`；
- Evidence HTTP 页面回归测试通过。

这些修正证明验证制品可交付，但没有修复 5.1／5.2 的运行时架构缺口。

## 8. 边界判定

连续 C8 判定为 `insufficient-evidence`，因此连续 C1～C8 整体也必须保持 `insufficient-evidence`。不得把 C0～C7 的 8/9 通过率换算成“OA 完整体已通过”。

同时，本轮没有发现必须共享业务表、跨库读写、复制 ORM、合并事实所有者或长期 fork Base 才能修复的问题。失败集中在：

- 应用输出没有完整登记为显式、版本化 Project Link；
- Edge entry action 没有跟上 Evidence 的细粒度动作契约；
- 人类页面与既有 OpenAPI 枚举漂移；
- 发布包遗漏后续 verifier。

这些都是可通过“增加版本化 Link／投影、收窄 Gateway 契约、同步消费者和发布闭包”修复的加法型装配问题。因此第一阶段对架构方向的科学结论是：**解耦边界在已验证业务闭环中可行，但 OA 完整体的最终只读装配尚未闭合；当前证据支持继续修正，不支持宣布整体通过。**

## 9. 下一轮恢复门槛

只在原 Tenant／Project 上继续，不重建数据掩盖失败：

1. 为 C4／C5／C6 的目标成果定义并写入显式 active Resource Link；
2. 为需要出现在资料页的成果提供版本化 Evidence metadata 投影，不让 Project 导入领域模型；
3. 修正 Gateway Evidence entry contract，使精确 `metadata.read` 与 `download` 可以独立进入并由 Evidence 最终授权；
4. 发布包含页面 `direct` 修正及 C7/C8 verifier 的固定摘要；
5. 重跑 C8，要求 coverage C2～C6 全部 true、窄 download 能力返回 200、十二库 digest 不变、单来源恢复通过、17/17 healthy；
6. 再由真人浏览器复核资料列表、不可下载反馈、授权后下载、金额脱敏和相邻人员空工作项。

只有上述门槛全部通过，连续旅程才能从 `insufficient-evidence` 提升；第二客户场景不能用于绕开本阶段缺口。
