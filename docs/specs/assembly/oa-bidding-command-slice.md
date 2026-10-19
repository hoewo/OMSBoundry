# OA 招投标同项目命令切片

2026-10-03 后续多附件及组件复用已随 `.20261003.1` 发布：原 UI 的真实 local 文件上传／访问／单份移除、刷新和丢回复后原命令继续通过，同一 G 的 Project revision 4／BE revision 15，非夹具业务事实及 Grant 保持。见[最新报告](../../reports/oa-reused-bidding-ui-validation-20261003.md)。下方 `.11` 为方式切片原有终态，不覆盖后续追加材料；OSS、全角色、原站现场和完整 OA 仍未通过。

日期：2026-10-02。关联 BF-093／BF-094。状态：`.11` 已部署并在同一 G／同一来源完成公网三方式、不适用／恢复及刷新原生终态；Project revision 4、BE revision 7，旧事实保持。文件、多附件、原站现场等效和完整 OA 未通过；失败链与实际范围见[报告](../../reports/oa-bidding-command-validation-20261002.md)。

问题 A → 可验证结果 B：用户进入招投标页只能看到未连接提示 → 在同一 OA 项目内选择比选、竞争性谈判、招投标或阶段不适用，实际保存到 Business Engagement 并建立 Project 引用，刷新可复读，失败能继续原操作。

## 原操作和范围

以原 OA `BusinessInfoTab.tsx`、`BiddingMethodControl`、`BiddingFileList.tsx` 和 `oa2026/service.go` 为源码依据。保留三种方式、当前项目、阶段不适用／恢复及材料缺口，不跳独立产品，不显示技术 ID 填写框。原站现场等效尚未取得；本切片仅关闭方式／适用性命令和引用，文件选择／上传／查看／下载／移除另验，不以假文件或材料齐备冒充签约。

## 所有权和变化账

| 变化 | 归属 | 不变部分 |
| --- | --- | --- |
| 来源列表增加精确 `project_id` 过滤 | Business Engagement 自有只读 API 兼容扩展 | 原无过滤查询、三种方式、状态／材料规则、schema 和权限 |
| 三种方式／适用性表单、错误继续 | 参考 OA Workbench 客户装配 | 不拥有业务事实，不签发能力 |
| 创建后关联及部分成功恢复 | 客户装配连接器 | 两个所有者分别提交、分别授权，无跨库事务 |

GET 仅查询、不创建。列表必须在来源本库按 Tenant／Project 过滤后限制数量，再执行原生资源可见性；不以最新 100 条全局列表推断当前项目不存在。Engagement 当前一项目一条的唯一约束保持。来源已存在时复用，不按名称猜关联；多个／错 Tenant／错 Project 响应失败关闭。

## 命令和恢复

用户明确操作后才创建；先验证页面可见的原生动作，目标仍作最终判断。创建使用当前 Project ID／revision 和幂等 command。来源成功后 Link 使用 Project 自有 expected revision、来源 ID／revision；关联失败保留来源，重读 Project 后仅重试关联。方式和适用性各用自己的 command；未知响应不换 command 或改载荷，成功但回读失败仅回读。新鲜 409 需要显式载入最新，不自动覆盖他人修改；无权限不给 Grant、不以工作台服务身份代读／代写。

Project 引用动作必须使用原生 `project-workspace.resource-link.manage`，测试夹具必须核对已发布 Capability Manifest 和服务端常量，不自行发明或缩写为 `resource.manage`。这是已有契约接线修正，不是新增授权或业务边界变化。

BE 命令回执是领域事实，不含原生 `allowed_actions`；保存后先验证身份并 authenticated GET 同一来源，确认 revision 相同及原生动作，再继续 Link／适用性／方式。回读失败保留原命令供显式重放，不能发新 create；并发变化要求显式重读。不能把列表凭证动作冒充当前业务状态下的可执行动作。

实际部署必须注入 Project→Business Engagement 的既有引用端点，不能只检查 IAM workload permission 或容器 health。旧部署漏此配置时按原地址接线，使用既有 workload credential；禁止塞静态 token、代用户绕过最终授权或扩权。配置修复门禁要求 Project 镜像、其他 env、密钥挂载和端口不变。

材料的“当前方式非必传”与“已登记”必须区分：只有自有材料表中的当前有效引用可以显示已登记，不能把非必传导致的 `satisfied=true` 或已取代历史当成当前附件。

## 验收门禁

- 精确来源查询：当前／相邻 Project、Tenant、错误 UUID、原接口兼容、读前后事实不变。
- 单项目交互：三种方式、阶段不适用／恢复、现有记录复用、取消不写、缺权无可提交入口。
- 分段故障：创建未知响应原命令恢复；创建成功 Link 失败不重复创建；Link 已保存而响应丢失重放；方式保存后读取失败不重复写；stale／并发／迟到响应不串项目。
- 串行运行组件测试及真实 PostgreSQL；独立服务匿名拒绝；实际浏览器点击，适当时走 ACR 不可变制品、联合备份／空目标恢复和公网复验。未运行项保持待办。

本切片不改 People、IAM 签发、合同／资金事实、Evidence Provider、Suite 或部署档；不关闭完整立项、B02、B03 文件全链或整个 OA。整体结论仍为 `insufficient-evidence`。

下一附件切片已发现原生能力差距：原 OA 同种材料可有多份文件，而 BE 目前每种 requirement 只允许一条有效引用（其余为已取代历史）。先确定 BE 自有材料集合的兼容演进、旧 put/remove 语义和迁移证据，再接原 OA 逐份查看／下载／移除；不能把历史引用当成当前附件列表，也不能在 Workbench 新建事实库。本 `.8` 不含这项演进。
