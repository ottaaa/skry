package logfetch

import (
	"container/list"
	"sync"
)

// lru is a simple thread-safe LRU. K is the key type, V the value.
// Capacity is fixed at construction; oldest entries are evicted when full.
// Optional sizeFn lets a caller cap *byte* footprint instead of (or in
// addition to) entry count — used to bound the blob cache.
type lru[K comparable, V any] struct {
	mu       sync.Mutex
	cap      int
	bytesCap int
	bytesNow int
	sizeFn   func(V) int
	ll       *list.List
	idx      map[K]*list.Element
}

type lruEntry[K comparable, V any] struct {
	key  K
	val  V
	size int
}

// newLRU returns an LRU with at most maxEntries items. If bytesCap > 0 and
// sizeFn != nil, items are also evicted when the total reported size
// exceeds bytesCap (eviction prefers oldest first regardless of size).
func newLRU[K comparable, V any](maxEntries int, bytesCap int, sizeFn func(V) int) *lru[K, V] {
	return &lru[K, V]{
		cap:      maxEntries,
		bytesCap: bytesCap,
		sizeFn:   sizeFn,
		ll:       list.New(),
		idx:      make(map[K]*list.Element, maxEntries),
	}
}

func (c *lru[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.idx[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*lruEntry[K, V]).val, true
	}
	var zero V
	return zero, false
}

// Set inserts or updates a key. Returns true if the value was newly stored,
// false if it was rejected (e.g. a single value larger than bytesCap).
func (c *lru[K, V]) Set(key K, val V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	size := 0
	if c.sizeFn != nil {
		size = c.sizeFn(val)
	}
	if c.bytesCap > 0 && size > c.bytesCap {
		return false // single value bigger than the entire cache
	}
	if el, ok := c.idx[key]; ok {
		entry := el.Value.(*lruEntry[K, V])
		c.bytesNow += size - entry.size
		entry.val = val
		entry.size = size
		c.ll.MoveToFront(el)
	} else {
		entry := &lruEntry[K, V]{key: key, val: val, size: size}
		c.idx[key] = c.ll.PushFront(entry)
		c.bytesNow += size
	}
	c.evictOverLimit()
	return true
}

func (c *lru[K, V]) evictOverLimit() {
	for c.ll.Len() > c.cap || (c.bytesCap > 0 && c.bytesNow > c.bytesCap) {
		el := c.ll.Back()
		if el == nil {
			return
		}
		entry := el.Value.(*lruEntry[K, V])
		c.ll.Remove(el)
		delete(c.idx, entry.key)
		c.bytesNow -= entry.size
	}
}

// Reset drops every entry. Used when the host switches worktrees.
func (c *lru[K, V]) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	c.idx = make(map[K]*list.Element, c.cap)
	c.bytesNow = 0
}

// Len reports the current entry count.
func (c *lru[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
