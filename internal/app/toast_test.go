package app

import (
	"strings"
	"testing"
)

func TestSetToastInstallsAndReturnsTickCmd(t *testing.T) {
	m := Model{}
	cmd := m.setToast("hello", ToastInfo)
	if cmd == nil {
		t.Fatal("setToast should return an expiry tick Cmd for non-empty text")
	}
	if m.toast.text != "hello" {
		t.Errorf("toast.text = %q, want hello", m.toast.text)
	}
	if m.toast.kind != ToastInfo {
		t.Errorf("toast.kind = %v, want ToastInfo", m.toast.kind)
	}
	if m.toast.seq != 1 {
		t.Errorf("first toast seq should be 1, got %d", m.toast.seq)
	}
}

func TestSetToastEmptyClearsAndReturnsNil(t *testing.T) {
	m := Model{}
	_ = m.setToast("hello", ToastInfo)
	if m.toast.text == "" {
		t.Fatal("precondition: toast should be set")
	}
	cmd := m.setToast("", ToastInfo)
	if cmd != nil {
		t.Errorf("empty setToast must return nil Cmd")
	}
	if m.toast.text != "" {
		t.Errorf("toast should be cleared, got %q", m.toast.text)
	}
}

func TestSetToastSequenceMonotonic(t *testing.T) {
	m := Model{}
	_ = m.setToast("a", ToastInfo)
	_ = m.setToast("b", ToastSuccess)
	_ = m.setToast("c", ToastError)
	if m.toast.seq != 3 {
		t.Errorf("third toast seq should be 3, got %d", m.toast.seq)
	}
	if m.toast.text != "c" || m.toast.kind != ToastError {
		t.Errorf("latest toast should be (c, ToastError), got (%q, %v)", m.toast.text, m.toast.kind)
	}
}

// TestToastExpiredMatchingSeqClears reproduces what the Update handler
// does when a toastExpiredMsg actually arrives: clear iff seq matches.
func TestToastExpiredMatchingSeqClears(t *testing.T) {
	m := Model{}
	_ = m.setToast("a", ToastInfo)
	seq := m.toast.seq

	if msg := (toastExpiredMsg{seq: seq}); msg.seq == m.toast.seq {
		m.toast = toast{}
	}
	if m.toast.text != "" {
		t.Errorf("matching seq should clear toast, got %q", m.toast.text)
	}
}

func TestToastExpiredStaleSeqNoOp(t *testing.T) {
	m := Model{}
	_ = m.setToast("a", ToastInfo)
	staleSeq := m.toast.seq
	_ = m.setToast("b", ToastSuccess) // bumps seq, supersedes the earlier tick

	// The stale tick arrives — it must not clear the live toast.
	if msg := (toastExpiredMsg{seq: staleSeq}); msg.seq == m.toast.seq {
		t.Fatal("stale seq should not match current toast.seq")
	}
	if m.toast.text != "b" {
		t.Errorf("live toast should survive stale expiry, got %q", m.toast.text)
	}
}

func TestRenderToastEmpty(t *testing.T) {
	m := Model{}
	if got := m.renderToast(); got != "" {
		t.Errorf("empty toast should render empty string, got %q", got)
	}
}

func TestRenderToastIncludesText(t *testing.T) {
	cases := []ToastKind{ToastInfo, ToastSuccess, ToastError}
	for _, kind := range cases {
		m := Model{}
		_ = m.setToast("hello world", kind)
		out := m.renderToast()
		if !strings.Contains(out, "hello world") {
			t.Errorf("kind=%v: rendered toast should contain text, got %q", kind, out)
		}
	}
}

// TestToastDuration sanity-checks the constant — too short feels flaky
// to read, too long feels sticky. 3s matches glow.
func TestToastDuration(t *testing.T) {
	if ToastDuration < 1_000_000_000 || ToastDuration > 10_000_000_000 {
		t.Errorf("ToastDuration = %v, expected 1s..10s", ToastDuration)
	}
}
