# Evidence 本地文件与 OSS 切换验证

## 问题与结果

旧文件仍在 ECS 本地，直接替换单一 Provider 会读不到历史内容 → Evidence 自己按不可变对象引用读取旧文件，新会话使用显式选择的存储，切换和回退后 File ID、Version、摘要与业务引用不变。

这是 Evidence 内部的两种已有存储实现兼容，不是新增公共平台或新的业务系统。业务应用仍只认识 File ID 和 Version。

## 实现约束

- `STORAGE_PROVIDER=hybrid` 必须同时配置本地与 OSS，并显式设置 `PRIMARY_STORAGE_PROVIDER=local|aliyun-oss`。单一模式保持兼容。
- Primary 只选择新会话的物理存储。已有会话的恢复授权、完整性复核、访问和清理由其自身 Provider 路由；不修改已确认 Version，不在找不到对象时偷偷尝试另一种存储。
- 本地引用不能带 OSS Bucket；OSS 引用必须匹配配置的 Bucket；未知 Provider 拒绝。IAM、租户及资源鉴权保持原样。
- 回退不是把新 OSS 文件改成本地，也不是退回不认识混合引用的旧镜像；使用兼容镜像把 Primary 改回 local，历史两种文件仍可读。
- 混合冷备份使用明确的新格式，包含数据库、本地对象和外部对象引用。恢复校验两类引用、摘要和本地字节，旧格式继续可读。外部对象引用不等于 OSS 字节备份；独立 OSS 字节恢复仍须另行验证。
- 正式切换须先完成隔离恢复、双向读与回退验证，再部署 Evidence 专属出网及 IMDS 防护，最后验证公网 Nginx → Gateway → IAM → Evidence 与浏览器直传。

## 验收证据

1. Provider 单元测试覆盖双向读取、切换后旧会话继续、未知位置拒绝、不回退猜测、两种上传主存储与 readiness 失败。
2. 真实 PostgreSQL 测试覆盖混合冷备份／空目标恢复；原 Local v1 备份仍能恢复。
3. Auth58 隔离副本读回原有本地文件，新增 OSS 文件后读回，再切换 Primary 为 local；不可变 Version 内容与业务引用无差异。
4. 主环境切换前保持不变；每项未实际运行的检查记为待验证。

## 本事项不负责

不进行历史文件物理搬迁，不改变其他应用的数据，不把对象引用备份称为字节备份，不替代浏览器操作链、权限负例、Web OSS 静态分离或完整 OA 验收。
