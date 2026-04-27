package logfetch

import "testing"

func TestLRUEvictsOldest(t *testing.T) {
	c := newLRU[string, int](2, 0, nil)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts a (oldest)
	if _, ok := c.Get("a"); ok {
		t.Errorf("a should have been evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("b: got (%v,%v) want (2,true)", v, ok)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Errorf("c: got (%v,%v) want (3,true)", v, ok)
	}
}

func TestLRUMovesAccessedToFront(t *testing.T) {
	c := newLRU[string, int](2, 0, nil)
	c.Set("a", 1)
	c.Set("b", 2)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should be present")
	}
	c.Set("c", 3) // a was just touched → b is oldest
	if _, ok := c.Get("b"); ok {
		t.Errorf("b should have been evicted (a was MRU)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Errorf("a should still be present")
	}
}

func TestLRUByteCapRejectsTooLarge(t *testing.T) {
	c := newLRU[string, []byte](10, 100, func(b []byte) int { return len(b) })
	if ok := c.Set("big", make([]byte, 200)); ok {
		t.Errorf("entry larger than bytesCap should be rejected")
	}
	if _, found := c.Get("big"); found {
		t.Errorf("rejected entry should not be retrievable")
	}
}

func TestLRUByteCapEvictsOnOverflow(t *testing.T) {
	c := newLRU[string, []byte](10, 100, func(b []byte) int { return len(b) })
	c.Set("a", make([]byte, 60))
	c.Set("b", make([]byte, 50)) // total 110 > 100; a (oldest) is evicted
	if _, ok := c.Get("a"); ok {
		t.Errorf("a should have been evicted by byte-cap overflow")
	}
	if _, ok := c.Get("b"); !ok {
		t.Errorf("b should be present")
	}
}

func TestLRUResetClears(t *testing.T) {
	c := newLRU[string, int](2, 0, nil)
	c.Set("a", 1)
	c.Reset()
	if c.Len() != 0 {
		t.Errorf("Len after Reset = %d, want 0", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Errorf("a should be gone after Reset")
	}
}
