package logfile

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestLogWritesJSONLine(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(Options{Dir: dir, Name: "test.log"})
	if err != nil {
		t.Fatal(err)
	}
	l.Log("watcher error", "err", "boom", "path", "/tmp/x")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	lines := readLines(t, filepath.Join(dir, "test.log"))
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %v", len(lines), lines)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rec["msg"] != "watcher error" || rec["err"] != "boom" || rec["path"] != "/tmp/x" {
		t.Errorf("unexpected record: %v", rec)
	}
	if _, ok := rec["ts"].(string); !ok {
		t.Errorf("ts should be a string, got %T", rec["ts"])
	}
}

func TestLogMalformedKV(t *testing.T) {
	dir := t.TempDir()
	l, _ := Open(Options{Dir: dir, Name: "test.log"})
	l.Log("oops", "dangling")
	_ = l.Close()
	lines := readLines(t, filepath.Join(dir, "test.log"))
	var rec map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &rec)
	if rec["_malformed"] != "dangling" {
		t.Errorf("expected _malformed=dangling, got %v", rec["_malformed"])
	}
}

func TestRotationByBytes(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(Options{Dir: dir, Name: "r.log", MaxBytes: 200, MaxBackups: 5})
	if err != nil {
		t.Fatal(err)
	}
	// Each line is ~70 bytes; 10 lines guarantee at least one rotation.
	for i := range 10 {
		l.Log("m", "i", i, "pad", strings.Repeat("x", 20))
	}
	_ = l.Close()
	entries, _ := os.ReadDir(dir)
	var active, backups int
	for _, e := range entries {
		switch {
		case e.Name() == "r.log":
			active++
		case strings.HasPrefix(e.Name(), "r.log."):
			backups++
		}
	}
	if active != 1 {
		t.Errorf("want 1 active file, got %d", active)
	}
	if backups == 0 {
		t.Errorf("expected at least 1 rotated backup, got 0")
	}
}

func TestPruneByCount(t *testing.T) {
	dir := t.TempDir()
	// Seed 5 fake backups.
	for i := range 5 {
		p := filepath.Join(dir, "p.log."+time.Now().Add(time.Duration(i)*time.Second).Format("20060102T150405"))
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Ensure distinct mtimes.
		past := time.Now().Add(-time.Duration(10-i) * time.Hour)
		_ = os.Chtimes(p, past, past)
	}
	l, err := Open(Options{Dir: dir, Name: "p.log", MaxBackups: 2, MaxBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Writing >10 bytes forces a rotate which triggers pruneByCount.
	l.Log("m", "pad", strings.Repeat("x", 20))
	_ = l.Close()

	entries, _ := os.ReadDir(dir)
	var backups int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "p.log.") {
			backups++
		}
	}
	if backups > 2 {
		t.Errorf("want ≤2 backups after prune, got %d", backups)
	}
}

func TestPruneByAgeOnOpen(t *testing.T) {
	dir := t.TempDir()
	// An old backup.
	old := filepath.Join(dir, "a.log.20000101T000000")
	_ = os.WriteFile(old, []byte("x"), 0o600)
	past := time.Now().Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(old, past, past)
	// A fresh backup.
	fresh := filepath.Join(dir, "a.log.20990101T000000")
	_ = os.WriteFile(fresh, []byte("x"), 0o600)

	l, err := Open(Options{Dir: dir, Name: "a.log", MaxAge: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	_ = l.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old backup should be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh backup should be kept, stat err=%v", err)
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger
	l.Log("no crash please")
}
