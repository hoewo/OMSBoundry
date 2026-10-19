# Project C5 完成检查点 - 交互规范

## 交互策略

### 核心任务

用户只读评估一个 active Project 是否同时拥有可追溯的批复和审计当前结果，并在有权限、来源未变化时显式确认该 C5 检查点；该页面不宣称完整 OA 或整个 Project 已完成。

### 主路径

输入 Project ID → GET `reference-oa-c5-v1` 评估 → 查看两项 requirement、精确来源和最近确认 → 满足且拥有 confirm 能力时显式确认 → Project revision 加一并冻结来源 → 之后重读显示一致或 drift。

### 决策点

- GET 永远零业务写入；依赖失败显示 `unavailable`，不能伪装为 `pending`。
- 确认按钮同时要求 overall `satisfied`、`allowed_actions` 包含 confirm 和无 drift。
- 确认提交服务端返回的 result ID／kind／revision／digest 与 Project expected revision。
- 确认只写 Project 的检查点、历史、命令和 revision，Project 保持 active，Results 不写入。
- 来源纠正或撤回只显示 drift；历史确认不被反向篡改。

## 页面状态

| 状态 | 触发条件 | 关键控件 |
| --- | --- | --- |
| 待评估 | 尚未输入 Project | Project ID、只读评估 |
| 满足 | 两项 requirement 都 satisfied | 精确来源、最近确认、显式确认 |
| 待办／不可用 | 缺结果或 Results 不可用 | 分项原因、来源恢复后重读 |
| 已确认 | 命令成功或幂等回放 | 确认 revision、冻结来源、Project 仍 active |
| 漂移／冲突 | 来源改变或 Project revision 变化 | drift 标记、重评、409 零写入说明 |
| 权限变化 | 401／403 或无 confirm | 只读内容、重新授权、隐藏确认 |

## 反馈与恢复

评估期间只锁定读取按钮。确认期间锁定确认按钮，成功后立即重读。409 不自动确认新来源；503 保留最近已读取投影但标记不可用，避免用户把旧结果当当前真相。重新授权后必须再次 GET，不能沿用旧 `allowed_actions`。

## 线框投影要求

`default.html` 投影待评估、满足可确认、待办／不可用、已确认、漂移／冲突和权限变化六种状态；明确显示“C5 检查点 ≠ 完整 OA 完成”。

正式 D1 已同时装入 Project、Contract、Evidence 与 Results，并通过统一入口和联合空目标恢复；这只证明页面和依赖可装配。独立进程之间的 approval→audit→assessment→confirmation 操作旅程尚未执行，不能把装配证据当作本页面“满足并可确认”状态已经端到端通过。
