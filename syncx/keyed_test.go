package syncx_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pinealctx/x/syncx"
)

// --- KeyedMutex tests ---

func TestKeyedMutex_Serialization(t *testing.T) {
	km := syncx.NewKeyedMutex[string]()
	var counter int
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			unlock := km.Lock("key")
			counter++
			unlock()
		})
	}
	wg.Wait()
	if counter != 100 {
		t.Fatalf("expected 100, got %d", counter)
	}
}

func TestKeyedMutex_DifferentKeysConcurrent(t *testing.T) {
	km := syncx.NewKeyedMutex[int]()

	unlock1 := km.Lock(1)
	defer unlock1()

	// key=2 must be acquirable while key=1 is held.
	done := make(chan struct{})
	go func() {
		unlock2 := km.Lock(2)
		close(done)
		unlock2()
	}()

	select {
	case <-done:
		// key=2 acquired while key=1 is held — different keys don't block each other.
	case <-time.After(time.Second):
		t.Fatal("key=2 blocked by key=1 — different keys should not interfere")
	}
}

func TestKeyedMutex_RefCountCleanup(t *testing.T) {
	km := syncx.NewKeyedMutex[string]()

	unlock := km.Lock("a")
	unlock()
	if km.Len() != 0 {
		t.Fatalf("expected 0 entries after unlock, got %d", km.Len())
	}

	// Multiple keys: all should be cleaned up.
	unlock1 := km.Lock("x")
	unlock2 := km.Lock("y")
	unlock1()
	if km.Len() != 1 {
		t.Fatalf("expected 1 entry after unlocking x, got %d", km.Len())
	}
	unlock2()
	if km.Len() != 0 {
		t.Fatalf("expected 0 entries after all unlocked, got %d", km.Len())
	}
}

func TestKeyedMutex_NoDataRace(t *testing.T) {
	km := syncx.NewKeyedMutex[string]()
	var counter int
	var wg sync.WaitGroup

	for range 200 {
		wg.Go(func() {
			unlock := km.Lock("race")
			counter++
			unlock()
		})
	}
	wg.Wait()
	if counter != 200 {
		t.Fatalf("expected 200, got %d", counter)
	}
}

func TestKeyedMutex_ZeroKey(t *testing.T) {
	km := syncx.NewKeyedMutex[int]()
	unlock := km.Lock(0)
	unlock()
	if km.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", km.Len())
	}
}

// --- KeyedLocker tests ---

func TestKeyedLocker_ExclusiveLock(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()
	var counter int
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			unlock := kl.Lock("k")
			counter++
			unlock()
		})
	}
	wg.Wait()
	if counter != 100 {
		t.Fatalf("expected 100, got %d", counter)
	}
}

func TestKeyedLocker_DifferentKeysConcurrent(t *testing.T) {
	kl := syncx.NewKeyedLocker[int]()

	unlock1 := kl.Lock(1)
	defer unlock1()

	done := make(chan struct{})
	go func() {
		unlock2 := kl.Lock(2)
		close(done)
		unlock2()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("key=2 blocked by key=1 — different keys should not interfere")
	}
}

func TestKeyedLocker_WriteLockBlocksRead(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()

	writeUnlock := kl.Lock("k")

	readAcquired := make(chan struct{})
	go func() {
		unlock := kl.RLock("k")
		close(readAcquired)
		unlock()
	}()

	select {
	case <-readAcquired:
		t.Fatal("read lock acquired while write lock was held")
	default:
	}

	writeUnlock()
	<-readAcquired
}

func TestKeyedLocker_ReadLockBlocksWrite(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()

	readUnlock := kl.RLock("k")

	writeAcquired := make(chan struct{})
	go func() {
		unlock := kl.Lock("k")
		close(writeAcquired)
		unlock()
	}()

	select {
	case <-writeAcquired:
		t.Fatal("write lock acquired while read lock was held")
	default:
	}

	readUnlock()
	<-writeAcquired
}

func TestKeyedLocker_ReadLocksConcurrent(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()
	const n = 100
	var wg sync.WaitGroup
	var ready sync.WaitGroup
	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64

	ready.Add(n)
	for range n {
		wg.Go(func() {
			ready.Done()
			ready.Wait()

			unlock := kl.RLock("k")
			cur := concurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			runtime.Gosched() // yield to let other goroutines overlap
			concurrent.Add(-1)
			unlock()
		})
	}
	wg.Wait()
	if maxConcurrent.Load() < 2 {
		t.Fatal("read locks should allow concurrent access")
	}
}

func TestKeyedLocker_RefCountCleanup(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()

	unlock := kl.Lock("a")
	unlock()
	if kl.Len() != 0 {
		t.Fatalf("expected 0 entries after unlock, got %d", kl.Len())
	}

	// Read lock cleanup.
	readUnlock := kl.RLock("b")
	readUnlock()
	if kl.Len() != 0 {
		t.Fatalf("expected 0 entries after read unlock, got %d", kl.Len())
	}

	// Same key, multiple RLocks: entry lives until last unlock.
	r1 := kl.RLock("c")
	r2 := kl.RLock("c")
	if kl.Len() != 1 {
		t.Fatalf("expected 1 entry for same-key double RLock, got %d", kl.Len())
	}
	r1()
	if kl.Len() != 1 {
		t.Fatalf("expected 1 entry after first RUnlock, got %d", kl.Len())
	}
	r2()
	if kl.Len() != 0 {
		t.Fatalf("expected 0 entries after all runlocks, got %d", kl.Len())
	}
}

func TestKeyedLocker_ZeroKey(t *testing.T) {
	kl := syncx.NewKeyedLocker[int]()
	unlock := kl.Lock(0)
	unlock()
	if kl.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", kl.Len())
	}
}

func TestKeyedLocker_NoDataRace(t *testing.T) {
	kl := syncx.NewKeyedLocker[string]()
	var counter int
	var wg sync.WaitGroup

	for range 200 {
		wg.Go(func() {
			unlock := kl.Lock("race")
			counter++
			unlock()
		})
	}
	for range 100 {
		wg.Go(func() {
			unlock := kl.RLock("race")
			unlock()
		})
	}
	wg.Wait()
	if counter != 200 {
		t.Fatalf("expected 200 writes, got %d", counter)
	}
}
