# ADR-0036：Evidence File 使用稳定身份和服务端分配的不可变递增版本

日期：2026-09-29  
状态：Accepted

## 背景

C8 Project 资料旅程需要证明来源文件变化能够被发现并显式重连。Project 保存稳定 File ID 和建立连接时的 Evidence revision；但实际检查发现 Evidence Service 虽然已经建模 `File + Version`，数据库约束和公开写入 API 仍只允许 Version 1。这样只能用测试夹具伪造 `revision_changed`，无法通过产品路径产生同一 File 的新版本，也无法把 stale／relink 记为真实业务证据。

同时，业务应用需要两种不同语义：稳定 File 引用用于发现“当前版本已经变化”，精确 File Version 引用用于重现当时使用的不可变字节。把两者混成一个可变记录会丢失历史解释力；每次修订都生成新 File ID 又会让业务应用无法识别它仍是同一个逻辑文件。

## 决策

1. `File ID` 是一个租户内稳定的逻辑文件身份；新建 File 固定从 Version 1 开始。
2. 同一 File 的后续内容使用 `POST /v1/files/{file_id}/versions/upload-sessions` 建立上传会话。版本号由 Evidence Service 在数据库锁内分配，调用方不能指定、覆盖或复用版本号。
3. Version 使用正整数单调递增。失败、取消或清理的上传会话可以留下版本号空洞；版本号不得回收，因为重用会使日志、对象键和幂等结果产生歧义。
4. 已完成 Version 的元数据、摘要和对象引用不可更新、不可删除。显式版本引用 `/v1/references/files/{file_id}/versions/{version}` 永远解析同一事实。
5. 稳定引用 `/v1/references/files/{file_id}` 返回最新已完成 Version。尚在上传、失败或已取消的更高会话不得改变稳定引用。
6. 同一 File 同时最多存在一个活动的后续版本会话；服务端使用事务级 advisory lock 串行化版本分配，并继续使用独立幂等命令约束等价重放与载荷冲突。
7. `evidence-service.upload` 继续要求 Tenant Scope，用于人类文件库的创建、完成、取消和追加版本；精确 File Scope 继续只授予 metadata／download，不因本次变化扩大为写能力。HTTP handler 仍复验目标 File 位于当前 Principal 的可见范围。
8. 业务应用保存 File ID、建立时 Version／digest 和业务含义。需要历史重现时请求显式 Version；需要检测来源变化时请求稳定 File 引用并比较建立时与当前 Version。业务应用不得复制 Evidence 的对象位置或自行修改版本。
9. Suite 页面通过 `Browser → Nginx → Gateway → IAM → Evidence` 的 HttpOnly 短期能力 Cookie 调用同源 API；直连开发入口才允许人工粘贴模板 Token。页面不能因为服务端已有 Cookie 仍要求用户再次输入 Bearer Token。

## 原因

- 稳定身份让 Project、Contract 等业务应用可以把“仍是同一份逻辑资料”和“内容已修订”同时表达出来；
- 不可变显式版本保留审计、恢复和历史重现能力；
- 服务端分配、单调不复用与活动会话串行化消除客户端抢号和并发覆盖；
- 稳定引用只前进到已确认版本，不会把半成品暴露给消费者；
- Tenant upload 与精确 metadata／download 分权维持当前 IAM 最小权限边界，不为一个 UI 场景扩大工作负载写权。

## 后果

- Evidence migration 放宽 `upload_sessions.version` 与 `file_versions.version` 为任意正整数；旧 Version 1 数据无需重写。
- 本地／OSS Provider 的 Object Key、完整性复核、访问授权、清理和备份必须覆盖任意正版本；恢复后 File ID、全部 Version、摘要和对象字节必须保持。
- C8 stale 验收不能再使用直接改库或伪造版本；必须通过正式追加版本端点产生 Version 2，再由 Project 稳定引用观察 `revision_changed`，最后通过正式 Link 操作恢复。
- 现有只会创建 Version 1 的客户端继续兼容；新客户端可选择追加版本，不要求业务应用同步升级。

## 被否决方案

### 每次修订都创建新的 File ID

这保留了字节不可变，但丢失“同一逻辑文件”的稳定身份，Project 无法可靠区分替换、修订和另一份材料。

### 允许客户端提交目标版本号

这会引入抢号、覆盖、版本复用和对象键冲突，破坏幂等与审计。

### 原地覆盖 Version 1

这会让旧 Project Link、审计记录和恢复快照指向不同字节，直接违反不可变事实约束。

### 让 Project 复制最新版文件元数据

复制会形成第二事实源并掩盖 Evidence 停机或 revision drift。Project 只保存连接时快照并在读取时组合当前投影。
