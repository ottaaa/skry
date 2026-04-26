package editor

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

const maxHighlightBytes = 512 * 1024

var (
	highlightStyle = styles.Get("github-dark")
	formatter      = formatters.Get("terminal256")
)

// Highlight returns the source split into ANSI-colored lines. For very large
// files, highlighting is skipped to keep the UI responsive.
func Highlight(source, filename string) []string {
	if len(source) > maxHighlightBytes {
		return strings.Split(source, "\n")
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		// chroma's exported function uses the British spelling.
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		return strings.Split(source, "\n")
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, highlightStyle, it); err != nil {
		return strings.Split(source, "\n")
	}
	return strings.Split(buf.String(), "\n")
}
