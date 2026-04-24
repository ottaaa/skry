package clipboard

import "github.com/atotto/clipboard"

// Copy writes s to the system clipboard. Errors surface to the caller; the
// atotto implementation falls back to an in-memory buffer on unsupported
// platforms, so Copy never silently drops input.
func Copy(s string) error { return clipboard.WriteAll(s) }
