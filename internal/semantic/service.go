package semantic

import (
	"context"
)

// Service composes the three architectural layers into a single facade so
// callers (MOE / dashboard / shadow) depend on one type instead of four. The
// zero value is unsafe — use NewDefaultService for offline-safe defaults.
type Service struct {
	Chunker    Chunker
	Embedder   Embedder
	Summarizer Summarizer
	Symbols    SymbolIndex
	Flow       FlowAnalyzer
}

// NewDefaultService wires the zero-dependency fallbacks: NaiveChunker,
// HashEmbedder, NoopSummarizer, NoopSymbolIndex, NoopFlowAnalyzer. Suitable
// for tests and for first-boot VibeOS without external tooling installed.
func NewDefaultService() *Service {
	return &Service{
		Chunker:    NewNaiveChunker(),
		Embedder:   NewHashEmbedder(),
		Summarizer: NoopSummarizer{},
		Symbols:    NoopSymbolIndex{},
		Flow:       NoopFlowAnalyzer{},
	}
}

// EmbedFile chunks then embeds a file. Returns one (chunk, vector) pair per
// chunk. Useful for batch ingestion into the MOE store.
type EmbeddedChunk struct {
	Chunk  Chunk
	Vector []float32
}

func (s *Service) EmbedFile(path, content string) ([]EmbeddedChunk, error) {
	if s == nil || s.Chunker == nil || s.Embedder == nil {
		return nil, nil
	}
	chunks, err := s.Chunker.Chunk(path, content)
	if err != nil {
		return nil, err
	}
	out := make([]EmbeddedChunk, 0, len(chunks))
	for _, c := range chunks {
		vec, err := s.Embedder.Embed(c.Text)
		if err != nil {
			return nil, err
		}
		out = append(out, EmbeddedChunk{Chunk: c, Vector: vec})
	}
	return out, nil
}

// Impact returns the symbols and flow findings touched by a change at the
// given path — the "影响分析" leg of L1+L2(+L3). Set deep=true to invoke L3.
type ImpactReport struct {
	Symbols  []Symbol
	Findings []FlowFinding
}

func (s *Service) Impact(ctx context.Context, projectRoot, path string, deep bool) (ImpactReport, error) {
	r := ImpactReport{}
	if s == nil {
		return r, nil
	}
	if s.Symbols != nil {
		syms, err := s.Symbols.Lookup(SymbolQuery{Path: path})
		if err != nil {
			return r, err
		}
		r.Symbols = syms
	}
	if deep && s.Flow != nil {
		findings, err := s.Flow.Analyze(ctx, FlowQuery{
			ProjectRoot: projectRoot,
			Path:        path,
			Kind:        "reachability",
		})
		if err != nil {
			return r, err
		}
		r.Findings = findings
	}
	return r, nil
}
