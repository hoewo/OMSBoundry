# M10-004 Contract 遗留 Evidence 迁移

日期：2026-09-22  
状态：规格已建立；只读预检实现中；迁移写入尚未开始

## A→B

**Contract Ledger 的历史附件仍由业务应用本地保存 → 在不改写合同历史、不丢失字节且可回退的前提下，把文件技术事实迁入独立 Evidence Service。**

## 已确认的模型差异

- Contract 的 `Evidence Document` 是业务聚合，拥有挂接目标、标题、用途和连续版本号；这些业务含义继续属于 Contract。
- Contract 的每个旧 `Evidence Version` 拥有独立 UUID、文件名、媒体类型、大小、SHA-256、Actor、时间和本地对象键。
- Evidence Service v1 的 File 只有不可变 Version 1；它拥有字节、摘要、Provider、存储位置和技术访问审计，不拥有合同目标或合同版本语义。
- 因此迁移采用“一条旧 Contract Evidence Version → 一个 Evidence File Version 1”，`evidence_file_id` 确定性复用旧 `version_id`。Contract 原 Document 和版本序号继续保留，不能把多条旧版本强塞入一个尚不支持多版本的 File。

## 不变量

1. 迁移前先生成并校验 Contract 全量数据库＋Evidence 联合备份；不从唯一在线副本直接试错。
2. 旧 Document、Version、Command、业务审计、Actor 和时间一律不更新、不删除。
3. 每个 `available` 旧版本在预检时必须找到普通文件，实际字节大小与 SHA-256 必须与数据库一致。
4. 每个 `missing` 旧版本继续明确为缺失，不创建虚假的 available Evidence File。
5. 迁入 File ID 固定等于旧 Version ID，Evidence Version 固定为 1；同一迁移批次重放必须得到同一结果。
6. Contract 新增显式引用映射，而不是让 Evidence 反向理解 Contract 数据库；两个应用禁止跨库外键和跨库写入。
7. 迁移期间旧下载仍可用；切换后 Contract 通过 Evidence 正式 API 获取访问授权，不读取 Evidence 数据库或对象存储。
8. 在所有映射和抽样下载验收完成前，不删除旧本地对象；回退只切回旧读取路径，不逆向改写历史。
9. 新附件写入切换必须有明确冻结点，防止同一附件同时写入旧内嵌存储和新服务。

## 分阶段执行

### Phase A：只读预检

- 输入：Contract 数据库、旧 Evidence 根目录、明确 Tenant ID。
- 只读扫描 Document／Version 和本地对象，不创建目录、不修改数据库、不上传对象。
- 输出确定性 JSON：数量、每条确定性 File ID 映射、缺失内容、大小／SHA 不一致、路径异常和是否可迁移。
- 任一 available 文件丢失、非普通文件、大小或 SHA 不一致时，禁止进入写入阶段。

### Phase B：隔离导入

- 从经过校验的备份副本或维护窗口读取旧字节。
- 使用 Evidence 专用离线导入命令，不开放“伪造历史”的公共 HTTP API。
- 导入命令消费预检清单，以固定 File ID 创建 File Version 1、迁移技术审计和对象；重复执行只验证既有事实完全相同。
- OSS 目标仍由 Evidence Storage Provider 管理；Contract 迁移器不能持有 OSS SDK 或 OSS 权限。

### Phase C：Contract 引用映射

- Contract 新增本地映射事实，至少包含旧 Document ID、旧 Version ID、旧版本序号、Evidence File ID、Evidence Version、迁移批次、源 SHA-256 和迁移时间。
- 映射只追加，不更新旧 Version；唯一约束确保一条旧 Version 只能映射一次。
- 读取采用“已映射走 Evidence；未映射继续走旧本地存储”的过渡策略。

### Phase D：冻结与切换

- 短暂停止 Contract 附件新增，重新运行预检，确保自 Phase A 后没有新增或变化。
- 完成剩余导入和映射后，把新增附件切到 Evidence API；Contract 只保存 File ID／Version 与业务含义。
- 验证 Evidence 停机时 Contract 结构化业务仍可用，附件操作明确失败且不伪造成功。

### Phase E：保留期与清理

- 至少完成全量摘要比对、代表性在线下载、备份恢复和回退演练后，开始旧对象保留期。
- 清理必须是单独、显式、可审计的运维动作；本切片默认不删除旧对象。

## 暂不实现

- 不在本次顺手给 Evidence v1 增加通用多版本模型；真实新需求出现时单独设计。
- 不把 Contract 标题、Purpose、Target 或客户分类复制进 Evidence 内核。
- 不让 Contract 直接生成 OSS 签名、持有 STS 权限或访问 Evidence 数据库。
- 不迁移 `missing` 内容、不猜测丢失文件、不按文件名去重。
- 不以一次性 SQL 跨库复制代替可校验的应用级迁移。

## 退出门槛

1. 同一源运行两次预检得到相同候选映射和问题集合；预检无写入副作用。
2. available 旧版本数量、迁入 File 数量、映射数量完全相等，所有 SHA-256 一致；missing 数量单独对账。
3. 多版本 Document 的每个旧版本都能按原序号下载正确字节。
4. 重跑导入和映射不产生重复 File、对象、映射或审计。
5. 切换前、切换后和恢复后，Contract 结构化事实与旧审计逐项不变。
6. Evidence 停机、OSS 拒绝、导入中断和 Contract 切回旧读取路径均有真实演练。
7. Contract 与 Evidence 的备份恢复顺序、引用一致性检查和旧对象清理步骤进入 D1 运维手册。

