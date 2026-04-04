# syncx 实现进度

## 子任务完成记录

### [2026-04-04 02:50] subtask_keyed

**状态**: 已完成
**摘要**: KeyedMutex[K] + KeyedLocker[K]，引用计数自动清理，18 个测试，两轮审计全 PASS。

### [2026-04-04 15:42] subtask_blocking_queue

**状态**: 已完成
**摘要**: BlockingQueue[T]，sync.Cond + 环形缓冲区，25 个测试。Push/Pop 支持 context.AfterFunc 取消，Close/CloseNow 双语义，9 个审计 agent 全 PASS。
