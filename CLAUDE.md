# 概要

go语言相关的扩展基础库

## 当网络不可用时，设置代理环境变量

网络代理URI环境变量为PROXY_URI。
```
export http_proxy=$PROXY_URI
export https_proxy=$PROXY_URI
```

## Execution Baseline

- 任何问题都需要第一性原理，目的？为什么要这么做？如何验证？不允许直接跳到结论或解决方案。
- 思考问题需要有层次，层次是对抗复杂性的利器。**禁止在没有层次的情况下直接实现**，必须先设计层次结构（如模块划分、接口定义、抽象层次等），再在每个层次上实现。
- 开始任务前先明确目标和验收标准。
- 目标不清晰先澄清，不在模糊目标下实现。
- 如果遇到难题，先网络搜索相关信息来补齐知识盲区，再设计可验证方案。
- 复杂任务先拆分为可验证子目标并逐一实现，拆分时必须为每个子任务定义 audit-scope。
- 为每个任务建立检查清单，确保实现符合预期并且没有遗漏。
- 实现过程中持续记录日志，方便回顾和总结经验。
- 实现新功能前，先检查项目中是否已有可复用的基础设施（如 logger、config、cache、HTTP client 等），优先复用而非重新实现。
- **子任务完成后，等待用户调用 `/task-complete` 触发审计和提交流程**，不要自动提交。
- 代码注释：(函数，类型，接口，结构体，全局变量等)为公开时需要清晰的注释说明职责，为内部根据情况可简洁注释；复杂逻辑加行内注释，避免过度注释；注释用英文。

## 设计原则：可测试性

**所有分层、模块、函数设计都必须满足可测试性。**

具体要求：

1. **接口隔离**：模块间通过接口通信，不依赖具体实现
   ```go
   // ✅ 正确：依赖接口
   type AgentRunner struct {
       sessionService SessionService  // 接口类型
       toolRegistry   ToolRegistry    // 接口类型
   }

   // ❌ 错误：依赖具体实现
   type AgentRunner struct {
       sessionService *SessionServiceImpl
   }
   ```

2. **依赖注入**：所有依赖通过构造函数注入，便于 mock
   ```go
   // ✅ 正确：依赖注入
   func NewAgentRunner(sess SessionService, tools ToolRegistry) *AgentRunner {
       return &AgentRunner{sessionService: sess, toolRegistry: tools}
   }

   // ❌ 错误：内部创建依赖
   func NewAgentRunner() *AgentRunner {
       return &AgentRunner{sessionService: NewSessionService()}  // 无法 mock
   }
   ```

3. **单向依赖**：上层依赖下层，下层不依赖上层
   ```
   ✅ 正确：
   TUI → Agent → Session → DB

   ❌ 错误：
   Agent → TUI  （下层依赖上层）
   ```

4. **PubSub 解耦**：跨层向上通信通过事件，不直接调用
   ```go
   // ✅ 正确：发布事件
   func (a *Agent) Run(ctx context.Context) {
       a.bus.Publish(Event{Type: "agent.thinking", Data: "..."})
   }

   // ❌ 错误：直接调用上层
   func (a *Agent) Run(ctx context.Context) {
       a.tui.ShowSpinner("...")  // Agent 不应该知道 TUI
   }
   ```

**测试金字塔**：
```
        /\
       /  \  E2E Tests（少量）
      /────\
     /      \ Integration Tests（中等）
    /────────\
   /          \ Unit Tests（大量）
  /────────────\
```

## 任务管理

### 任务目录结构

```
.process
   task/
   task-xxx/
      task.md          # 任务目标、子任务划分、验收标准
      progress.md      # 跟踪日志 + 审计记录
      notes.md         # 讨论记录（由 /discuss 生成，可选）
      decisions.md     # 决策摘要（由 /discuss 生成，可选）
      subtask_1/
         task.md
         progress.md
      subtask_2/
         task.md
         progress.md
```

### 文件职责

- **task.md**：任务目标、描述、子任务划分（含 audit-scope）、参考资料、验收标准
- **progress.md**：跟踪日志 + 审计记录

### 做子任务计划时需要根据情况将 [Audit Router](.claude/agents/audit-router.md) 纳入考虑，明确每个子任务需要哪些审计项，让审计成为设计的一部分，而不是事后补充的检查点。

### task.md 中的 audit-scope 定义

在制定子任务计划时，必须为每个子任务定义 `audit-scope`，明确关联的审计项和输出类型。**audit-scope 是必填项，没有 audit-scope 的子任务不允许开始执行。**

```markdown
### subtask_1: 实现 XXX 功能

**输出类型**: 代码
**audit-scope**:
- audit-go-code-style
- audit-go-naming
- audit-go-error-handling
- audit-test-strategy

**目标**: ...
**验收标准**: ...
```

输出类型与audit的可能的对应关系，详细的启用规则在 audit-router.md 中定义：
- **任意代码** → audit-engineering-baseline + audit-engineering-quality
- **go代码** → go-code-style、go-naming、go-error-handling、go-design 等代码相关 audit
- **文档** → audit-engineering-baseline（目标与验收、执行纪律）
- **配置** → audit-engineering-baseline、audit-config、audit-security
- **测试** → audit-test-strategy

## 路径
对于文件和目录路径，尽量使用相对路径，避免使用绝对路径，以确保在不同环境下的兼容性和可移植性。

## 代码注释规则
导出函数、类型、结构体、接口应在需要时提供清晰的英文注释，说明职责和使用边界；比较复杂的逻辑可以添加少量行内注释，解释关键步骤和决策理由；公共接口仅在用法不直观时再补充示例，避免模板鼓励过度注释。

## 项目结构
此内容需要在项目结构发生变化后更新，保持与实际项目结构一致，更新内容在下面：

### 任务历史

| 任务 | 目录 | 包 | 状态 | 子任务 |
|------|------|-----|------|--------|
| errorx 包 | `.process/task/task-errorx/` | errorx | ✅ 完成 | — |
| syncx 包 | `.process/task/task-syncx/` | syncx | ✅ 完成 | subtask_keyed, subtask_blocking_queue, subtask_ring_queue, subtask_refactor_shared |
| ds 包 | `.process/task/task-ds/` | ds | ✅ 完成 | subtask_ordered_map, subtask_set, subtask_bimap, subtask_stack, subtask_heap |
| syncx ReadThrough | `.process/task/task-syncx-readthrough/` | syncx | ✅ 完成 | subtask_read_through |
| syncx Pool[T] | `.process/task/task-syncx-pool/` | syncx | ✅ 完成 | subtask_pool |
| retryx 包 | `.process/task/task-retryx/` | retryx | ✅ 完成 | subtask_backoff, subtask_retry |
| ctxv 包 | `.process/task/task-ctxv/` | ctxv | ✅ 完成 | subtask_ctxv |
| syncx Dispatcher | `.process/task/task-syncx-dispatcher/` | syncx | ✅ 完成 | subtask_dispatcher |
| syncx SingleFlight | `.process/task/task-syncx-singleflight/` | syncx | ✅ 完成 | subtask_singleflight |
| syncx Group[T] | `.process/task/task-syncx-group/` | syncx | ✅ 完成 | subtask_group |
| ds Clone 补全 | `.process/task/task-ds-clone/` | ds | ✅ 完成 | subtask_clone |

设计讨论记录在 `.process/discuss/overview_N/` 中：
- `overview_0` — errorx + syncx 首批模块设计
- `overview_1` — ds 包设计
- `overview_2` — 后续模块评估（cache/stringx/timex/randx 等 10 个候选）
- `overview_3` — 扩展阶段规划（Pool/retryx/ctxv/Dispatcher/SingleFlight/Group/Clone）

### 源码目录
x/                                  # 项目根目录
├── CLAUDE.md                          # 本文件
├── LICENSE
├── go.mod                             # github.com/pinealctx/x
├── docs/                              # 项目文档（设计、规划）
├── errorx/                            # 错误处理基础层（零外部依赖，泛型驱动）
│   ├── sentinel.go                    #   Sentinel[D] 哨兵错误（幽灵类型域隔离）
│   ├── errorx.go                      #   Error[Code] 带码错误 + IsCode/ContainsCode 链查询
│   ├── sentinel_test.go               #   Sentinel 测试（域隔离、跨域、fmt.Errorf 穿透）
│   └── errorx_test.go                 #   Error 测试（叶子/包装/链查询/跨域穿透/nil 安全）
├── syncx/                             # 并发原语扩展包（依赖 errorx）
│   ├── errors.go                      #   包级哨兵错误（域隔离）
│   ├── queue_internal.go              #   ringBuf[T] + closedState + waitCond（队列共享内部实现）
│   ├── keyed.go                       #   KeyedMutex[K], KeyedLocker[K]（引用计数自动清理）
│   ├── keyed_test.go                  #   Keyed 测试（序列化、并发、race detector）
│   ├── blocking_queue.go              #   BlockingQueue[T]（sync.Cond + ringBuf，阻塞/非阻塞双模式）
│   ├── blocking_queue_test.go         #   BlockingQueue 测试（25 个，含并发 + race detector）
│   ├── ring_queue.go                  #   RingQueue[T]（sync.Cond + ringBuf，满时驱逐最老）
│   ├── ring_queue_test.go             #   RingQueue 测试（24 个，含并发 + race detector）
│   └── queue_internal_test.go         #   ringBuf/waitCond/边界 内部单元测试（11 个）
│   ├── read_through.go                #   Cache[K,V] 接口 + ReadThrough[K,V] cache-aside + per-key stampede protection
│   ├── read_through_test.go           #   ReadThrough 测试（13 个，含并发 + race detector）
│   ├── pool.go                        #   Pool[T] 类型安全 sync.Pool 泛型封装（可选 reset）
│   ├── pool_test.go                   #   Pool 测试（12 个，含并发 + race detector）
│   ├── dispatcher.go                  #   Dispatcher[K,V] 按 key hash 路由到固定 goroutine 的 slot 调度器
│   ├── dispatcher_test.go             #   Dispatcher 测试（17 个，含并发 + race detector）
│   ├── singleflight.go                #   SingleFlight[K,V] 泛型并发去重（同 key 只执行一次，共享结果）
│   ├── singleflight_test.go           #   SingleFlight 测试（12 个，含并发 + race detector）
│   ├── group.go                        #   Group[T] 泛型并发结果收集器（收集所有 (T,error)，按提交顺序返回）
│   └── group_test.go                   #   Group 测试（16 个，含并发 + race detector + panic recovery）
├── retryx/                           # 泛型重试库（零外部依赖，可组合 backoff 策略）
│   ├── backoff.go                    #   BackoffStrategy 接口 + Exponential/Fixed + WithJitter/WithMaxWait 装饰器
│   ├── backoff_test.go               #   BackoffStrategy 测试（22 个，含边界值 + 装饰器组合）
│   ├── retry.go                      #   Do[T] 泛型重试 + Option（Attempts/Backoff/RetryIf/OnRetry）
│   └── retry_test.go                 #   Do[T] 测试（14 个，含并发 + race detector + context 取消）
├── ctxv/                              # 类型安全 context value（零外部依赖，泛型 Key[T]）
│   ├── ctxv.go                       #   Key[T], NewKey, WithValue, Value, MustValue, String
│   └── ctxv_test.go                  #   ctxv 测试（13 个，含并发 + race detector）
├── ds/                                # 泛型数据结构包（零外部依赖）
│   ├── ordered_map.go                 #   OrderedMap[K,V]（map+侵入式双向链表，O(1)+零分配迭代）
│   ├── ordered_map_test.go
│   ├── set.go                         #   Set[T]（map[T]struct{}，集合运算+关系判断）
│   ├── set_test.go
│   ├── bimap.go                       #   BiMap[K,V]（双map双向O(1)查找）
│   ├── bimap_test.go
│   ├── stack.go                       #   Stack[T]（slice LIFO栈）
│   ├── stack_test.go
│   ├── heap.go                        #   Heap[T]（二叉堆+自定义compare，min/max便捷构造）
│   └── heap_test.go
```

### 已评估不做（记录供参考）

| 模块 | 理由 | 替代方案 |
|------|------|---------|
| cache | 社区有成熟泛型库 | go-freelru |
| stringx | 3-5 行薄函数，封装增加认知负担 | stdlib strings |
| timex | Go 1.23+ Timer 已修复，vDSO 优化 Now() | stdlib time |
| randx | math/rand/v2 已是 ChaCha8+OS熵 | stdlib math/rand/v2 |
| convx | strconv 虽繁琐但语义清晰 | stdlib strconv |
| restx | 社区有成熟方案 | go-resty/resty |
| jsonx | 等 encoding/json/v2 稳定 | stdlib encoding/json |
| pubsub | 应用架构组件，非基础库 | 项目内实现 |
| slicex | 已被 lo 覆盖 | samber/lo |
| idgen | ID 策略因项目而异 | 应用层 |
| mess | AES+base58 整数混淆，场景太窄 | 按需实现 |
