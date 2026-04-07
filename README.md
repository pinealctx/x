# x

Generic extension libraries for Go. Zero external dependencies, generics-driven.

```
go get github.com/pinealctx/x
```

Requires Go 1.26+.

## Packages

- [errorx](#errorx) — coded errors and domain-isolated sentinels
- [syncx](#syncx) — concurrent primitives and patterns
- [ds](#ds) — generic data structures
- [retryx](#retryx) — retry with composable backoff
- [ctxv](#ctxv) — type-safe context values

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
```

### RingQueue

Fixed-capacity queue that evicts the oldest item when full.

```go
q := syncx.NewRingQueue[string](8)
q.Push(ctx, "msg")
v, err := q.Pop(ctx)
```

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
    syncx.WithReset(func(b *bytes.Buffer) { b.Reset() }))

buf := p.Get()
defer p.Put(buf)
```

### Dispatcher

Routes keyed work to a fixed set of goroutines by hash — preserves per-key ordering.

```go
d := syncx.NewDispatcher[string, int](8, func(ctx context.Context, key string, val int) {
    // always called on the same goroutine for the same key
})
d.Start(ctx)
d.Dispatch("user:42", 1)
```

### SingleFlight

Deduplicates concurrent calls for the same key.

```go
sf := syncx.NewSingleFlight[string, *Data]()
result, err, shared := sf.Do(ctx, "key", func(ctx context.Context) (*Data, error) {
    return fetchData(ctx)
})
```

### Group

Collects results from concurrent goroutines in submission order.

```go
g := syncx.NewGroup[int]()
g.Go(func() (int, error) { return compute1() })
g.Go(func() (int, error) { return compute2() })
results := g.Wait() // []Result[int] in submission order
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
m.Range(func(k string, v int) bool {
    fmt.Println(k, v) // a 1, b 2 — insertion order
    return true
})
```

### Set

Set algebra and relation checks.

```go
a := ds.NewSet("a", "b", "c")
b := ds.NewSet("b", "c", "d")

a.Union(b)        // {a, b, c, d}
a.Intersection(b) // {b, c}
a.Difference(b)   // {a}
a.IsSubset(b)     // false
```

### BiMap

Bidirectional O(1) lookup.

```go
m := ds.NewBiMap[string, int]()
m.Put("one", 1)
m.GetByKey("one") // 1, true
m.GetByVal(1)     // "one", true
```

### Stack

LIFO stack.

```go
s := ds.NewStack[int]()
s.Push(1)
s.Push(2)
v, _ := s.Pop() // 2
```

### Heap

Binary heap with custom comparator.

```go
h := ds.NewMinHeap[int]()   // min-heap
h := ds.NewMaxHeap[int]()   // max-heap
h.Push(3, 1, 2)
v, _ := h.Pop() // 1 (min)
```

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

## License

MIT
