package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPreviewFileShowsFromTop(t *testing.T) {
	body := "line1\nline2\nline3\nline4\n"
	p := writeTempFile(t, "x.txt", body)
	out := PreviewFile(p, "x.txt", 0, 0, 40, 4)
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line4") {
		t.Errorf("preview should include all 4 lines, got:\n%s", out)
	}
}

func TestPreviewFileCentersOnLine(t *testing.T) {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "row")
	}
	body := strings.Join(lines, "\n")
	p := writeTempFile(t, "y.txt", body)
	// Ask for line 50 in a 10-row viewport: previewTop should pick ~45 so
	// line 50 sits near the middle. We can't read the visual position but
	// we can confirm line 50's number is in the rendered output.
	out := PreviewFile(p, "y.txt", 50, 0, 40, 10)
	if !strings.Contains(out, " 50 ") {
		t.Errorf("preview centered on line 50 should contain line number 50, got:\n%s", out)
	}
}

func TestPreviewFileEmptyPath(t *testing.T) {
	out := PreviewFile("", "", 0, 0, 40, 4)
	if !strings.Contains(out, "no selection") {
		t.Errorf("empty path should hint 'no selection', got %q", out)
	}
}

func TestPreviewFileMissing(t *testing.T) {
	out := PreviewFile("/nonexistent/path", "missing.txt", 0, 0, 40, 4)
	if !strings.Contains(out, "cannot read") {
		t.Errorf("missing file should hint 'cannot read', got %q", out)
	}
}

func TestPreviewFileBinary(t *testing.T) {
	p := writeTempFile(t, "blob.bin", "abc\x00def")
	out := PreviewFile(p, "blob.bin", 0, 0, 40, 4)
	if !strings.Contains(out, "binary") {
		t.Errorf("binary file should hint 'binary', got %q", out)
	}
}

func TestPreviewFileTooLarge(t *testing.T) {
	big := strings.Repeat("x", previewMaxBytes+1)
	p := writeTempFile(t, "big.txt", big)
	out := PreviewFile(p, "big.txt", 0, 0, 40, 4)
	if !strings.Contains(out, "too large") {
		t.Errorf("large file should hint 'too large', got %q", out)
	}
}

func TestPreviewFileDirectory(t *testing.T) {
	dir := t.TempDir()
	out := PreviewFile(dir, "some/dir", 0, 0, 40, 4)
	if !strings.Contains(out, "directory") {
		t.Errorf("directory should hint 'directory', got %q", out)
	}
}

func TestPreviewTopMath(t *testing.T) {
	cases := []struct {
		name   string
		line   int
		height int
		total  int
		want   int
	}{
		{"no line", 0, 10, 100, 0},
		{"file shorter than viewport", 5, 20, 8, 0},
		{"line near top", 3, 10, 100, 0},
		{"line in middle", 50, 10, 100, 44},
		{"line near bottom clamps", 99, 10, 100, 90},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := previewTop(c.line, c.height, c.total)
			if got != c.want {
				t.Errorf("previewTop(line=%d h=%d total=%d) = %d, want %d", c.line, c.height, c.total, got, c.want)
			}
		})
	}
}
