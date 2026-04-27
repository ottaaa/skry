package logfetch

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ottaaa/skry/internal/git"
)

// newTestFetcher injects fakes for showMeta / showFile / parentOf so we
// can exercise caching, singleflight and cancellation without forking
// real git.
func newTestFetcher() *Fetcher {
	f := New("/repo")
	return f
}

func TestMetaCachesResultsAcrossCalls(t *testing.T) {
	f := newTestFetcher()
	var calls int32
	f.showMeta = func(_, sha string) (string, []git.StatusEntry, error) {
		atomic.AddInt32(&calls, 1)
		return "msg-" + sha, []git.StatusEntry{{Path: sha + ".go", Status: git.StatusModified}}, nil
	}

	cmd1 := f.Meta("aaa", 1)
	msg1 := cmd1().(MetaArrivedMsg)
	if msg1.Body != "msg-aaa" || msg1.Seq != 1 || msg1.Err != nil {
		t.Errorf("first Meta: got %#v", msg1)
	}

	cmd2 := f.Meta("aaa", 2)
	msg2 := cmd2().(MetaArrivedMsg)
	if msg2.Seq != 2 || msg2.Body != "msg-aaa" {
		t.Errorf("second Meta should hit cache, got %#v", msg2)
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("showMeta called %d times, want 1 (cache hit second time)", got)
	}
}

func TestMetaSingleflightDedupesConcurrentMisses(t *testing.T) {
	f := newTestFetcher()
	var calls int32
	gate := make(chan struct{})
	f.showMeta = func(_, sha string) (string, []git.StatusEntry, error) {
		atomic.AddInt32(&calls, 1)
		<-gate
		return "body", nil, nil
	}

	var wg sync.WaitGroup
	const N = 8
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = f.Meta("aaa", uint64(i+1))()
		}(i)
	}
	time.Sleep(20 * time.Millisecond) // let all goroutines enter fetchMeta
	close(gate)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("concurrent Meta on same sha: showMeta called %d times, want 1", got)
	}
}

func TestMetaSurfacesError(t *testing.T) {
	f := newTestFetcher()
	want := errors.New("boom")
	f.showMeta = func(_, _ string) (string, []git.StatusEntry, error) { return "", nil, want }
	msg := f.Meta("aaa", 1)().(MetaArrivedMsg)
	if msg.Err == nil || msg.Err.Error() != want.Error() {
		t.Errorf("expected error %q, got %v", want, msg.Err)
	}
	// Failures must NOT be cached — a retry should re-invoke showMeta.
	var calls int32
	f.showMeta = func(_, _ string) (string, []git.StatusEntry, error) {
		atomic.AddInt32(&calls, 1)
		return "ok", nil, nil
	}
	_ = f.Meta("aaa", 2)()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("post-error retry: showMeta called %d times, want 1", got)
	}
}

func TestDiffCachesAndDedupes(t *testing.T) {
	f := newTestFetcher()
	var fileCalls int32
	f.parentOf = func(_, sha string) (string, error) { return "parent-" + sha, nil }
	f.showFile = func(_, ref, _ string) ([]byte, error) {
		atomic.AddInt32(&fileCalls, 1)
		return []byte("hello\nworld\n"), nil
	}

	msg1 := f.Diff("aaa", "x.go", 1)().(DiffArrivedMsg)
	if msg1.Err != nil || msg1.Binary {
		t.Fatalf("first Diff: got %#v", msg1)
	}
	if len(msg1.Rows) == 0 {
		t.Errorf("expected diff rows, got 0")
	}
	msg2 := f.Diff("aaa", "x.go", 2)().(DiffArrivedMsg)
	if msg2.Seq != 2 {
		t.Errorf("Seq mismatch: %d", msg2.Seq)
	}
	if got := atomic.LoadInt32(&fileCalls); got != 2 {
		t.Errorf("showFile calls = %d, want 2 (parent + sha for first miss only)", got)
	}
}

func TestDiffMarksBinary(t *testing.T) {
	f := newTestFetcher()
	f.parentOf = func(_, _ string) (string, error) { return "parent", nil }
	f.showFile = func(_, _, _ string) ([]byte, error) {
		return []byte{0x00, 0x01, 0x02}, nil // NUL byte → binary
	}
	msg := f.Diff("aaa", "x.bin", 1)().(DiffArrivedMsg)
	if !msg.Binary {
		t.Errorf("expected Binary=true, got %#v", msg)
	}
}

func TestResetDropsCaches(t *testing.T) {
	f := newTestFetcher()
	var calls int32
	f.showMeta = func(_, _ string) (string, []git.StatusEntry, error) {
		atomic.AddInt32(&calls, 1)
		return "body", nil, nil
	}
	_ = f.Meta("aaa", 1)()
	f.Reset("/repo2")
	_ = f.Meta("aaa", 2)()
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("Reset should drop cache; showMeta calls = %d, want 2", got)
	}
}

func TestPrefetchSkipsCachedAndIsBoundedByQueue(t *testing.T) {
	f := newTestFetcher()
	f.Start()
	defer f.Stop()

	var calls int32
	gate := make(chan struct{})
	f.showMeta = func(_, _ string) (string, []git.StatusEntry, error) {
		atomic.AddInt32(&calls, 1)
		<-gate
		return "", nil, nil
	}

	// Pre-cache aaa so it is skipped.
	f.meta.Set("aaa", metaEntry{body: "cached"})

	// Issue a prefetch batch including the cached one + new ones.
	f.Prefetch([]string{"aaa", "bbb", "ccc"})
	close(gate)

	// Wait for workers to finish (pool=2). Use a short polling loop.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&calls) == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("Prefetch invoked showMeta %d times, want 2 (aaa was cached)", got)
	}
}
