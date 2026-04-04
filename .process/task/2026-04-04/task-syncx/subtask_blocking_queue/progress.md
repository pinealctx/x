# subtask_blocking_queue 进度

## 跟踪日志

### 2026-04-04 subtask_blocking_queue: BlockingQueue[T]

**实现内容**:
- `syncx/blocking_queue.go`: BlockingQueue[T] — 基于 sync.Cond + 环形缓冲区的阻塞队列
  - 三种关闭状态：openState、closedDrain、closedNow
  - 阻塞模式 Push/Pop + context 取消支持
  - 非阻塞模式 TryPush/TryPop
  - Close（drain 模式）+ CloseNow（立即停止），幂等
  - Len/Peek 辅助方法
  - wait() 辅助方法：goroutine 监听 ctx.Done() 触发 Broadcast 解除 Cond.Wait()
- `syncx/blocking_queue_test.go`: 24 个测试

**修复项**:
1. Pop 在 CloseNow 后未正确返回 ErrQueueClosed（count > 0 时直接出队跳过了 closedNow 检查）
2. errcheck：所有 Push/Pop 返回值已检查或标注 nolint
3. unused-parameter：CloseIdempotent/CloseNowIdempotent/NoDataRace 改为 `_ *testing.T`
4. misspell：cancelled → canceled
5. gofmt：const 块对齐修正
6. audit-go-code-style：Len() 改用 defer q.mu.Unlock() 保持一致性
7. audit-test-strategy：补充 TestBlockingQueue_CapacityOneWrapAround（容量=1 多轮回绕测试）
8. wait() 性能优化：从 goroutine-per-wait 改为 context.AfterFunc，正常路径零 goroutine 创建
9. Pop 冗余分支合并：closedNow/closedDrain 在 count==0 时行为相同，合并为 `q.state != openState`
10. `*new(T)` → `var zero T`：意图更明确的零值写法
11. 补充 TestBlockingQueue_PanicOnNegativeCapacity 负数容量测试

**验证结果**:
- 25/25 BlockingQueue 测试 + 18 Keyed 测试 = 43 全通过
- `go vet` 无错误
- `golangci-lint` 0 issues
- `-race` 无数据竞争

## 审计记录

### [2026-04-04 15:42] subtask_blocking_queue 第一轮审计

**状态**: PASS（有修复项）

**审计结果**:
- ✅ audit-engineering-baseline: PASS（1 建议 → 已补充 progress.md）
- ✅ audit-engineering-quality: PASS
- ✅ audit-go-code-style: PASS（1 警告 → 已修复：Len() 使用 defer）
- ✅ audit-go-naming: PASS
- ✅ audit-go-concurrency: PASS
- ✅ audit-go-design: PASS
- ✅ audit-go-datacontainer: PASS
- ✅ audit-go-error-handling: PASS（1 警告：哨兵错误未用点分键格式，同步原语级别可接受）
- ✅ audit-test-strategy: PASS（1 警告 → 已修复：补充 capacity=1 回绕测试）
- ✅ gitleaks: no leaks
- ✅ go test -race: 24/24 pass
- ✅ golangci-lint: 0 issues
