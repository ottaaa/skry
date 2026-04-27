package logfetch

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleflightDeduplicates(t *testing.T) {
	g := newSingleflight[int]()
	var calls int32
	start := make(chan struct{})
	wait := make(chan struct{})

	fn := func() (int, error) {
		atomic.AddInt32(&calls, 1)
		<-wait
		return 42, nil
	}

	const N = 10
	results := make([]int, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			v, _ := g.Do("k", fn)
			results[i] = v
		}(i)
	}
	close(start)
	// Give all goroutines a moment to enter Do.
	time.Sleep(10 * time.Millisecond)
	close(wait)
	wg.Wait()

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("fn invoked %d times, want exactly 1 (dedup)", calls)
	}
	for _, r := range results {
		if r != 42 {
			t.Errorf("all callers should see 42, got %d", r)
		}
	}
}

func TestSingleflightSequentialCallsRunIndependently(t *testing.T) {
	g := newSingleflight[int]()
	var calls int32
	fn := func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 1, nil
	}
	for range 3 {
		_, _ = g.Do("k", fn)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("sequential Do calls = %d, want 3", got)
	}
}
