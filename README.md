# x

Generic extension libraries for Go. Minimal external dependencies, generics-driven.

```
go get github.com/pinealctx/x
```

Requires Go 1.26+.

## Packages

- [errorx](#errorx) — coded errors and domain-isolated sentinels
- [panicx](#panicx) — panic recovery with stack capture
- [syncx](#syncx) — concurrent primitives and patterns
- [ds](#ds) — generic data structures
- [retryx](#retryx) — retry with composable backoff
- [ctxv](#ctxv) — type-safe context values
- [handlerx](#handlerx) — generic middleware chain for RPC handlers
- [pipeline](#pipeline) — declarative step-execution graph

---

## errorx

Typed error codes and phantom-type sentinel errors.

```go
// Define error codes per domain
type Code int
const (
    CodeNotFound Code = iota + 1
    CodeUnauthorized
)

// Leaf error
err := errorx.New(CodeNotFound, "user not found")

// Wrapped error
wrapped := errorx.Wrap(err, CodeUnauthorized, "access denied")

// Chain-aware code query
errorx.IsCode(wrapped, CodeUnauthorized) // true — checks top node only
errorx.ContainsCode(wrapped, CodeNotFound) // true — traverses full chain
```

```go
// Domain-isolated sentinels (phantom type prevents cross-domain errors.Is)
type myDomain struct{}
var ErrTimeout = errorx.NewSentinel[myDomain]("timeout")

errors.Is(ErrTimeout, ErrTimeout) // true
```

Also: `Newf`, `Wrapf` (format variants), `NewSentinelf`.

---

## panicx

Panic recovery that captures the stack trace as a structured `*PanicError`.

```go
// Recover inside a goroutine
defer func() {
    if r := recover(); r != nil {
        err := panicx.NewPanicError(r)
        log.Printf("panic: %v\nstack:\n  %s", err, strings.Join(err.Stack(), "\n  "))
    }
}()

// Adjust stack skip for wrapper functions
err := panicx.NewPanicErrorSkip(r, 2)
```

Use `errors.Is(err, panicx.ErrPanic)` to check whether an error originated from a panic.

---

## syncx

### KeyedMutex / KeyedLocker

Per-key locking with automatic cleanup via reference counting.

```go
km := syncx.NewKeyedMutex[string]()
unlock := km.Lock("user:42")
defer unlock()

kl := syncx.NewKeyedLocker[string]()
unlock := kl.RLock("resource:1")
defer unlock()
```

Also: `Len()` on both types.

### BlockingQueue

Context-aware blocking queue with close semantics.

```go
q := syncx.NewBlockingQueue[int](64)

// producer
q.Push(ctx, 42)

// consumer
v, err := q.Pop(ctx)

// graceful shutdown
q.Close() // drain remaining items
q.CloseNow() // discard remaining items
```

Also: `TryPush`, `TryPop` (non-blocking), `Peek`, `Len`.

### RingQueue

Fixed-capacity queue that evicts the oldest item when full.

```go
q := syncx.NewRingQueue[string](8)
q.Push("msg")
v, err := q.Pop(ctx)

// returns evicted value when full
old, ok := q.PushEvict("overflow")
```

Also: `PushEvict` (returns evicted value), `TryPop` (non-blocking), `Peek`, `Len`, `Close`, `CloseNow`.

### ReadThrough

Cache-aside with per-key stampede protection.

```go
rt := syncx.NewReadThrough[string, User](cache, func(ctx context.Context, key string) (User, error) {
    return db.GetUser(ctx, key)
})

user, err := rt.Get(ctx, "user:42")
```

### Pool

Type-safe wrapper around `sync.Pool`.

```go
p := syncx.NewPool(func() *bytes.Buffer { return new(bytes.Buffer) },
    func(b *bytes.Buffer) { b.Reset() })

buf := p.Get()
defer p.Put(buf)
```

### Dispatcher

Routes keyed work to a fixed set of goroutines by hash — preserves per-key ordering.

```go
d := syncx.NewDispatcher[string, int](8, func(key string, val int) error {
    // always called on the same goroutine for the same key
    return nil
})
defer d.Close()
d.Submit("user:42", 1)
```

Options: `WithBuffer(n)` to set per-slot buffer, `WithOnError(fn)` for error callback.
Also: `TrySubmit` (non-blocking).

### SingleFlight

Deduplicates concurrent calls for the same key.

```go
sf := syncx.NewSingleFlight[string, *Data]()
result, shared, err := sf.Do("key", func() (*Data, error) {
    return fetchData()
})
sf.Forget("key") // evict cached result
```

### Group

Collects results from concurrent goroutines in submission order.

```go
g := syncx.NewGroup[int](0)
g.Go(func() (int, error) { return compute1() })
g.Go(func() (int, error) { return compute2() })
results := g.Wait() // []Result[int] in submission order
```

### Race

Returns the first successful result; if all fail, returns the last error.

```go
val, err := syncx.Race(ctx,
    func(ctx context.Context) (string, error) { return fetchFromPrimary(ctx) },
    func(ctx context.Context) (string, error) { return fetchFromFallback(ctx) },
)
```

---

## ds

Non-concurrent-safe generic containers. Use external synchronization when sharing across goroutines.

### OrderedMap

Insertion-ordered map with O(1) access and zero-allocation iteration.

```go
m := ds.NewOrderedMap[string, int]()
m.Set("a", 1)
m.Set("b", 2)
for k, v := range m.All() {
    fmt.Println(k, v) // a 1, b 2 — insertion order
}
```

Also: `NewOrderedMapWithCapacity`, `Get`, `Has`, `Delete`, `Backward`, `Keys`, `Values`, `Clone`, `Len`, `Clear`.

### Set

Set algebra and relation checks.

```go
a := ds.NewSet("a", "b", "c")
b := ds.NewSet("b", "c", "d")

a.Union(b)       // {a, b, c, d}
a.Intersect(b)   // {b, c}
a.Difference(b)  // {a}
a.IsSubset(b)    // false
```

Also: `NewSetWithCapacity`, `Add`, `Remove`, `Has`, `SymmetricDifference`, `Equal`, `IsSuperset`, `ToSlice`, `Clone`, `Len`, `Clear`.

### BiMap

Bidirectional O(1) lookup.

```go
m := ds.NewBiMap[string, int]()
m.Set("one", 1)
m.GetByKey("one")  // 1, true
m.GetByValue(1)    // "one", true
```

Also: `NewBiMapWithCapacity`, `DeleteByKey`, `DeleteByValue`, `Keys`, `Values`, `Clone`, `Len`, `Clear`.

### Stack

LIFO stack.

```go
s := ds.NewStack[int]()
s.Push(1)
s.Push(2)
v, _ := s.Pop() // 2
```

Also: `NewStackWithCapacity`, `Peek`, `Clone`, `Len`, `Clear`.

### Heap

Binary heap with custom comparator.

```go
h := ds.NewMinHeap[int]()   // min-heap
h := ds.NewMaxHeap[int]()   // max-heap
h := ds.NewHeap(func(a, b int) int { return a - b }) // custom
h.Push(3)
h.Push(1)
h.Push(2)
v, _ := h.Pop() // 1 (min)
```

Also: `NewHeapFrom` (initialize from slice), `Peek`, `Drain` (pop-all iterator), `Clone`, `Len`, `Clear`.

### SortedMap

Ordered map combining O(1) key lookup with O(log n) sorted iteration. Backed by `tidwall/btree`.

```go
type Item struct {
    ID    int
    Score float64
}

m := ds.NewSortedMap[int, Item](
    func(v Item) int { return v.ID },    // key extraction
    func(a, b Item) bool { return a.Score < b.Score }, // sort order
)
m.Set(Item{ID: 1, Score: 3.0})
m.Set(Item{ID: 2, Score: 1.0})
m.Set(Item{ID: 3, Score: 2.0})

for v := range m.Ascend() {
    fmt.Println(v.ID, v.Score) // 2 1.0, 3 2.0, 1 3.0
}
```

Also: `Get`, `Has`, `Delete`, `AscendFrom`, `AscendAfter`, `Descend`, `DescendFrom`, `DescendBefore`, `Len`, `Clear`.

---

## retryx

Generic retry with composable backoff strategies.

```go
result, err := retryx.Do(ctx, func() (string, error) {
    return callAPI()
},
    retryx.Attempts(3),
    retryx.Backoff(
        retryx.WithJitter(
            retryx.NewExponential(100*time.Millisecond, 2.0),
            0.2,
        ),
    ),
    retryx.RetryIf(func(err error) bool {
        return errors.Is(err, ErrTransient)
    }),
    retryx.OnRetry(func(attempt int, err error) {
        log.Printf("attempt %d failed: %v", attempt, err)
    }),
)
```

Backoff strategies: `NewExponential`, `NewFixed`. Wrappers: `WithJitter`, `WithMaxWait`.

---

## ctxv

Type-safe context values without type assertions.

```go
var requestIDKey = ctxv.NewKey[string]("requestID")

// store
ctx = requestIDKey.WithValue(ctx, "req-123")

// retrieve
id, ok := requestIDKey.Value(ctx)       // "req-123", true
id   := requestIDKey.MustValue(ctx)     // panics if missing
```

---

## handlerx

Framework-agnostic generic middleware chain for RPC handlers.

```go
// Define handler and interceptors
h := func(ctx context.Context, req MyRequest) (MyResponse, error) {
    return MyResponse{Result: "ok"}, nil
}

// Chain interceptors (outermost first)
h = handlerx.Chain(h,
    handlerx.WithTimeout[MyRequest, MyResponse](5*time.Second),
    handlerx.WithRecovery[MyRequest, MyResponse](),
)

// Execute
resp, err := h(ctx, req)
```

---

## pipeline

Declarative step-execution graph: sequential (Then), concurrent all-must-succeed (Parallel), and concurrent first-success (Race).

```go
type state struct {
    Req  *Request
    Data *Data
}

err := pipeline.New[state]().
    Then("validate", func(ctx context.Context, s *state) error {
        return validate(s.Req)
    }).
    Parallel("fetch", fetchA, fetchB).
    Then("save", func(ctx context.Context, s *state) error {
        return save(ctx, s.Data)
    }).
    Run(ctx, &state{Req: req})
```

---

## License

MIT
