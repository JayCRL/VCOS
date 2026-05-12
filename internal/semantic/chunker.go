package semantic

import (
	"strings"
)

// Chunk is a single contiguous slice of source text. Tree-sitter-backed
// chunkers will populate Symbol / Kind / Lang from the AST; the naive
// fallback leaves them empty.
type Chunk struct {
	Path   string
	Start  int    // 1-based start line
	End    int    // inclusive end line
	Symbol string // qualified symbol name when known (e.g. "pkg.Type.Method")
	Kind   string // "function" | "method" | "class" | "block" | ""
	Lang   string // "go" | "ts" | "py" | "" when unknown
	Text   string
}

// Chunker slices a source file into semantically meaningful chunks. The L1
// implementation backed by Tree-sitter is the architectural target; this
// package ships NaiveChunker as a zero-dep fallback so MOE / Embedder
// pipelines have something to consume today.
type Chunker interface {
	// Chunk slices the file content into chunks. path is informational and
	// used to set Chunk.Path and infer Chunk.Lang when possible.
	Chunk(path, content string) ([]Chunk, error)
}

// NaiveChunker splits by blank-line boundaries with a max-lines cap. It is
// language-agnostic and deterministic, which makes it sufficient for tests
// and for MOE embedding-chunking before Tree-sitter lands.
type NaiveChunker struct {
	// MaxLines caps each chunk size. Defaults to 60 when zero.
	MaxLines int
}

// NewNaiveChunker returns a chunker with sensible defaults.
func NewNaiveChunker() *NaiveChunker { return &NaiveChunker{MaxLines: 60} }

func (c *NaiveChunker) Chunk(path, content string) ([]Chunk, error) {
	maxLines := c.MaxLines
	if maxLines <= 0 {
		maxLines = 60
	}
	lang := inferLang(path)
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var buf []string
	start := 1
	flush := func(end int) {
		if len(buf) == 0 {
			return
		}
		chunks = append(chunks, Chunk{
			Path:  path,
			Start: start,
			End:   end,
			Lang:  lang,
			Text:  strings.Join(buf, "\n"),
		})
		buf = buf[:0]
	}
	for i, line := range lines {
		ln := i + 1
		if strings.TrimSpace(line) == "" && len(buf) > 0 {
			flush(ln - 1)
			start = ln + 1
			continue
		}
		if len(buf) >= maxLines {
			flush(ln - 1)
			start = ln
		}
		if len(buf) == 0 {
			start = ln
		}
		buf = append(buf, line)
	}
	flush(len(lines))
	return chunks, nil
}

func inferLang(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx"):
		return "ts"
	case strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx"):
		return "js"
	case strings.HasSuffix(path, ".py"):
		return "py"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	case strings.HasSuffix(path, ".java"):
		return "java"
	case strings.HasSuffix(path, ".c"), strings.HasSuffix(path, ".h"),
		strings.HasSuffix(path, ".cc"), strings.HasSuffix(path, ".cpp"),
		strings.HasSuffix(path, ".hpp"):
		return "c"
	default:
		return ""
	}
}
