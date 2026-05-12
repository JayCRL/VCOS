// Package semantic implements the layered code-semantic stack referenced by
// the architecture doc §4.1:
//
//   L1  Tree-sitter (always-on)   — AST slicing, incremental summaries,
//                                   embedding chunking. Interfaces: Chunker,
//                                   Embedder, Summarizer.
//   L2  SCIP indexer (on-demand)  — cross-file symbol / call graph.
//                                   Interface: SymbolIndex.
//   L3  Joern (backup)            — data-flow / control-flow deep analysis,
//                                   spun up per-query, never resident.
//                                   Interface: FlowAnalyzer.
//
// This package ships interface definitions and lightweight default
// implementations (Naive line chunker, Hash embedder, Noop summarizer /
// index / analyzer) so the rest of VCOS can depend on these contracts
// today and swap in real Tree-sitter / SCIP / Joern integrations later
// without touching call-sites.
package semantic

// Embedder converts text into a fixed-length embedding vector.
type Embedder interface {
	// Embed returns a vector representation of text.
	Embed(text string) ([]float32, error)

	// Dim returns the embedding dimension, or 0 if unknown.
	Dim() int
}

// Summarizer produces concise natural-language summaries of structured
// content (code diffs, logs, conversation turns).
type Summarizer interface {
	// Summarize returns a one-line summary of content.
	Summarize(content string) (string, error)
}

// NoopEmbedder returns nil vectors. Always valid, never errors.
type NoopEmbedder struct{}

func (NoopEmbedder) Embed(string) ([]float32, error) { return nil, nil }
func (NoopEmbedder) Dim() int                        { return 0 }

// NoopSummarizer returns the input unchanged.
type NoopSummarizer struct{}

func (NoopSummarizer) Summarize(content string) (string, error) { return content, nil }
