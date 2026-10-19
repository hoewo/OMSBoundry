# 完整 OA 工作面推进记录：2026-10-03

招投标只是用户举例，不是范围缩减。目标仍为原 OA 的完整操作闭环；[完整交付清单](../specs/assembly/oa-complete-operation-delivery-ledger.md)保留经营、生产、资料、统计及权限全部工作面。本批推进经营参与人和合同原生端口，不能据此宣布完整 OA 或整套架构通过。

## 已实施与限定通过

BF-097 在 Project 新增有界 `commercial_contributor`、独立管理能力和 schema 11；不把旧 participant、经营负责人或生产负责人充当经营参与人。新能力由管理员显式授予，旧 Assignment Policy 1.2 未扩大。原 `BusinessPersonnelList`、`PersonnelSelect`、姓名组件、创建按钮、确认弹窗和样式直接复用；23 份原文件摘要一致，原 OA 仓库未修改。列表、520px 抽屉、选择和确认结束接入原生 Project／People 端口。

移出只结束 Project 自有 Assignment，旧历史保持，重入产生新的 Assignment ID；command ID／expected revision、可信 Actor 和来源 Person revision 由原用例复验。回复不确定时保留原命令手动继续，不自动重发新命令或覆盖冲突。账号／部门／岗位不属于目前的 DTO，界面明确资料未提供，不编造画像。

`.20261003.3` 是开发测试闭包，不是正式 clean-HEAD 交付：Project、Workbench 和 IAM capability catalog 0.7；固定 Nginx → Gateway、各应用所有权、旧策略及其他 14 个容器保持。Project 10→11 两次 migration 验证旧事实不变；迁移后禁止直接回退旧 Project 二进制。

公网 a／b 验证器定位到了隐藏演示按钮，属于验收脚本失败；c 定位真实面板后暴露 Workbench 未输出 People 配置路由。`.20261003.4` 仅补配置路由，d 真实添加／移出已提交，但后续读取触发 429，整轮失败保留；e 独立复读 revision 10、零业务写入通过。`.20261003.5` 去除人员只读回读的重复续签，每次写入仍先更新原生能力；f／g 是同名不同 Person ID 的定位歧义，h 错等缓存读取事件，均未写。i 已提交但测试提前关闭，随后只读原生回执证明 revision 11，不能记 i 浏览器通过。j 三个真实业务／恢复场景通过，最后快速相邻切页仍 429；没有换项目或覆盖失败回执。

`.20261003.6` 只改 Workbench：隐藏个人工作／指标面板不抢发读取，进入可见面板后仍按原生身份读取；200 项 Node 零 skip、Go／vet、前端 check／build、两次独立启动及制品发布通过。联合备份／13 库空目标恢复、文件及 IAM key 字节、离机 SHA 核对通过后部署，其他 16 容器 ID 保持。限流、IAM／Gateway／Project／策略和 schema 不变，不通过扩权或放宽限流消除反例。

`.6` 的 k／l／m 导航驱动均零业务写入失败，m 的安全请求／DOM 诊断明确登录成功、真实目录已加载 15 个项目、无 429 或脚本错误，超时点是全站 `networkidle` 生命周期；失败记录保留。n 改用真实名单回读及控件就绪门禁，三个场景实际通过：深链保持 C revision 13／三条商业历史；七个相邻经营页签快速切换、刷新及 reload 无 429／JS 错误／新增写入；隐藏工作及指标面板零请求，可见统计触发正式 Money 来源读取。n 未放宽业务／权限／计数断言，并经正式 IAM logout 204 结束本次会话。

人员只在既有 C `70edead7-9dc9-44a3-9b35-b9d41b83264e` 操作，没有新建或重置项目，未改用户“测试项目101”。C 从 revision 8 经五个唯一原生命令到 13，商业 Assignment 新增三条，当前一条、结束历史两条；丢回复后 reload 并重放原命令不新增 revision。新增显式 Grant 只有管理员一条，不自动给旧角色补权；旧经营／生产分工及历史核对保持。

## 分层证据

安全回执与截图保存在 [contributors-20261003](../../acceptance/oa-initiation-repair/evidence/contributors-20261003/)。凭据、env、数据库 dump、私钥与私有日志不进入仓库。

| 层次 | 本批实际结果 | 不能代替的证据 |
| --- | --- | --- |
| Project 真库 | 私有 PG18：Project 118／database 4／HTTP 98 条含子测试通过；schema 11 独立启动／重启、10 个匿名路由拒绝通过 | 原站等效、全角色与父包传播 |
| 原人员组件 | 合成端口五场景通过，Workbench Node 200 条零 skip、Go／vet通过；独立 Workbench 两次启动通过 | 完整正式角色矩阵 |
| `.3`／`.4`／`.5`／`.6` 发布 | 四批联合备份、13 库逐个空目标恢复、文件及 IAM key 字节、离机 SHA 核对通过；D1 check／17 healthy | 完整 Suite 恢复运行、正式交付 |
| 人员公网 | e 零写复读、j 三个业务／恢复场景通过；j 导航 429 失败保留；`.6` n 三项零写导航／可见来源读取通过 | B02 全项或完整 OA |
| Contract 真库候选 | 一个聚焦集成测试实际通过：超 JS safe integer 的十进制金额、草稿与正式 revision、协议／结算、幂等及反向授权；Go／vet通过 | 合同原组件、两链映射、附件或公网 |
| Contract 独立启动 | c 自有空库 ops migration→server verify、schema 19、9 个匿名路由拒绝、重启事实不变通过；13 主库／17 ID 保持，探针已停 | 不能把候选记为已部署 |

Contract 启动 a 的开发 auto 15 秒迁移预算不足，且同期人员写入使全 Main 摘要比较失败；b 的迁移探针缺自有 Evidence 临时目录。两个原失败保留，c 使用正式安装顺序和受限临时目录，不宣称开发 auto 超时已修复，不重开 restore20。

所有权 guard 初次失败于 IAM `subjects` 摘要：成功登录更新管理员 `updated_at`。没有直接排除账号表或重写 frozen before；保留空目标恢复库的 37 个账号全行摘要与冻结前态完全相同，再逐字段比较当前账号，只有一个管理员 `updated_at` 改变，密码、状态、角色、版本及其他字段均相等，其他受保护表摘要相等。最终 after guard 实际通过：核对 n 后新鲜摘要、C 原字段／旧分工／旧历史、五个唯一命令／三条新 Assignment、唯一同 Tenant 管理员 Grant；所有其他业务所有者事实保持，checker 零业务写入。

当前 `.6` Workbench 摘要 `fb097ceae605cd771e4106934d235472992c1749a67556c41139f32c0140bd3c`，Linux binary SHA `ccdc52246dc6c6a5a319a8da6353938ccae186e12bb4a970ccd46557a106421b`，源归档 SHA `f029e37996590c8697f74d0cccddf348336a2751adeecbe709ebdf3ab882b69b`。当前运维 wrapper 为 `/opt/omsbizfoundry/acceptance/oa-projection-visibility-20261003a/d1-active`，旧 wrapper 不再用于当前 up。它是有限开发测试发布，不是正式 clean-HEAD release。

四份私有离机归档均保持 0600、忽略于 Git，校验值分别为：`.3` `21df0a8cff41e6a64e57bd7d67096f171e9be357bf589d842609410095f4b80b`／239001 bytes；`.4` `aeb1603e88ed088531dcef89351f9a8edabe577a9ad4fccba0395821262b3c9a`／239123 bytes；`.5` `175b4de8d883def155f0a18768ab98e1ed00907162f3bd4ffb4aaf0b7ad3bfec`／239673 bytes；`.6` `4238903ba45523d0c4c4b64c1a282a21d45aba4a52efb482479846ed0f9e7c20`／240901 bytes。库、卷、配置和恢复探针数据均保留；空目标数据恢复不等于完整 Suite 恢复运行。

最终只读收尾已运行当前 wrapper `check`，exit 0、17/17 healthy、容器 ID 不再改变，restore20 和全部验证探针均停止；C revision 13、Project 11、BE 2、主 Contract 18、Evidence local／前端 ECS 逐项复读。新收尾 helper 初次错误期待 publication 的 image_id 字段，随后又错误期待主 Contract 已为候选 19，均只读失败、没有触发业务写入；改为不可变镜像实际 ID及主站保留的精确 schema 18 后通过，不升级主 Contract 来迎合错误断言。

## 合同推进及不能省略的映射

Contract 新增版本化 JSON 工作面／命令端口，复用已有草稿、主体、激活、协议与结算用例，不增加候选仓库已有的 schema 19。只接受正式 IAM Bearer，创建限 Tenant，既有合同只接受精确 Contract Scope 或 Tenant；参考模板、cookie 或 local 管理员不能调用。金额仍为十进制字符串，GET 零写，已确认事实追加，不用客户端 Actor 授权。该二进制未部署，原合同表单尚未接通；主站保留旧 Contract／schema `000018_party_migration_mappings.sql`，不能把隔离候选的 19 记为主站状态。

[两链操作映射](../specs/assembly/oa-contract-original-operation-map.md)已逐字段读取原组件：甲方→资质单位、资质单位→公司；780px 编辑／1360px 预览；分项费用、可空及零费率；三种协议、定案意图与 confirmed Settlement、合同／协议／定案材料归属。Project 已有 contract 草稿引用及 DraftVersion，不需要假装草稿正式生效。当前缺的不是排版：类型化 Overlay 分项／费率／两链持久化、正式主体上下文、附件业务归属、部分成功续办及定案更正仍须补齐，再接原组件。甲方文本不能自动成为 Party，签署日不能自动成为生效日；不能把旧浮点计算、共表访问、失败时物理删除文件或覆盖正式协议照搬进稳定 Base。

下一纵向闭环仍是原两链合同 → 协议／定案，然后支付／开票、成本／奖金、分类型生产及资料统计。缺少该映射与真实浏览器证据时 B04～B06 保持未通过。

## 保留的整体缺口

人员账号／岗位资料、离职候选、完整角色分权、父包传播、完整立项字段、两链合同及资金、生产各类型阶段、成果／外委、资料统计仍需逐项闭环。n 截图的项目统计上半部仍明确标注“界面验证数据”，下半部 Money 为实时原生读取；该混合页面不算真实完整统计。Evidence Provider 仍为 local／local-http，Workbench 静态资源仍在 ECS；OSS 分离未完成。旧 G 文件验收和用户“测试项目101”不在本批写入范围。

整体架构验证结论仍为 `insufficient-evidence`。局部 API、组件、发布或数据恢复不能相加成为完整 OA 通过；正文只记录实际结果，失败反例与历史批次不覆盖。
