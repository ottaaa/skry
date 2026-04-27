package logfetch

import "sync"

// singleflight de-duplicates concurrent calls keyed by string. The first
// caller for a given key runs fn; subsequent callers block until fn
// returns and share its result. Modeled on golang.org/x/sync/singleflight
// but trimmed: no shared error semantics, no Forget, no Group nesting —
// just enough to fold prefetch + foreground requests for the same SHA
// into one git invocation.
type singleflight[T any] struct {
	mu    sync.Mutex
	calls map[string]*sfCall[T]
}

type sfCall[T any] struct {
	wg  sync.WaitGroup
	val T
	err error
}

func newSingleflight[T any]() *singleflight[T] {
	return &singleflight[T]{calls: map[string]*sfCall[T]{}}
}

// Do runs fn (or joins an already-in-flight call for key) and returns the
// shared result. Concurrent callers see the same val/err. The result is
// not cached across separate Do invocations — once the call completes the
// entry is removed.
func (g *singleflight[T]) Do(key string, fn func() (T, error)) (T, error) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &sfCall[T]{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return c.val, c.err
}
