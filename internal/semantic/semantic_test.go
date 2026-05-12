package semantic

import (
	"context"
	"strings"
	"testing"
)

func TestNaiveChunker(t *testing.T) {
	c := NewNaiveChunker()
	src := "package main\n\nfunc A() {}\n\nfunc B() {\n\treturn\n}\n"
	chunks, err := c.Chunk("foo.go", src)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	for _, ch := range chunks {
		if ch.Lang != "go" {
			t.Fatalf("Lang=%q want go", ch.Lang)
		}
		if ch.Start < 1 || ch.End < ch.Start {
			t.Fatalf("invalid range %d-%d", ch.Start, ch.End)
		}
	}
}

func TestNaiveChunkerCapsMaxLines(t *testing.T) {
	c := &NaiveChunker{MaxLines: 3}
	src := strings.Repeat("x\n", 10)
	chunks, _ := c.Chunk("blob.txt", src)
	for _, ch := range chunks {
		if got := ch.End - ch.Start + 1; got > 3 {
			t.Fatalf("chunk size %d exceeds cap", got)
		}
	}
}

func TestHashEmbedderDeterministic(t *testing.T) {
	e := NewHashEmbedder()
	v1, _ := e.Embed("the quick brown fox")
	v2, _ := e.Embed("the quick brown fox")
	if len(v1) != e.Dim() {
		t.Fatalf("Dim=%d want %d", len(v1), e.Dim())
	}
	if CosineSimilarity(v1, v2) < 0.999 {
		t.Fatalf("identical text not deterministic: %f", CosineSimilarity(v1, v2))
	}
}

func TestHashEmbedderSimilarityOrdering(t *testing.T) {
	e := NewHashEmbedder()
	a, _ := e.Embed("authentication token jwt validation")
	b, _ := e.Embed("auth jwt token validation")
	c, _ := e.Embed("file system disk usage report")
	ab := CosineSimilarity(a, b)
	ac := CosineSimilarity(a, c)
	if ab <= ac {
		t.Fatalf("expected related texts to score higher: ab=%f ac=%f", ab, ac)
	}
}

func TestHashEmbedderEmpty(t *testing.T) {
	e := NewHashEmbedder()
	v, err := e.Embed("")
	if err != nil {
		t.Fatalf("Embed empty: %v", err)
	}
	if len(v) != e.Dim() {
		t.Fatalf("Dim mismatch")
	}
	for _, x := range v {
		if x != 0 {
			t.Fatalf("empty text should yield zero vector")
		}
	}
}

func TestDefaultServiceEmbedFile(t *testing.T) {
	s := NewDefaultService()
	out, err := s.EmbedFile("x.go", "package x\n\nfunc Hello() string { return \"hi\" }\n")
	if err != nil {
		t.Fatalf("EmbedFile: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("no embedded chunks")
	}
	for _, ec := range out {
		if len(ec.Vector) != s.Embedder.Dim() {
			t.Fatalf("vector dim mismatch")
		}
	}
}

func TestDefaultServiceImpact(t *testing.T) {
	s := NewDefaultService()
	r, err := s.Impact(context.Background(), "/proj", "x.go", true)
	if err != nil {
		t.Fatalf("Impact: %v", err)
	}
	// Noop backends → empty findings, never nil.
	if r.Symbols == nil || r.Findings == nil {
		t.Fatalf("expected non-nil empty slices, got %+v", r)
	}
}

func TestNoopBackends(t *testing.T) {
	if dim := (NoopEmbedder{}).Dim(); dim != 0 {
		t.Fatalf("NoopEmbedder.Dim=%d", dim)
	}
	if s, _ := (NoopSummarizer{}).Summarize("hello"); s != "hello" {
		t.Fatalf("NoopSummarizer.Summarize=%q", s)
	}
	if syms, _ := (NoopSymbolIndex{}).Lookup(SymbolQuery{}); len(syms) != 0 {
		t.Fatalf("NoopSymbolIndex returned non-empty")
	}
	if findings, _ := (NoopFlowAnalyzer{}).Analyze(context.Background(), FlowQuery{}); len(findings) != 0 {
		t.Fatalf("NoopFlowAnalyzer returned non-empty")
	}
}
