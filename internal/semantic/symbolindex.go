package semantic

// Symbol describes a code symbol surfaced by an L2 cross-file indexer.
type Symbol struct {
	Name      string   // qualified name e.g. "pkg.Type.Method"
	Kind      string   // "function" | "method" | "type" | "var" | "const"
	Path      string   // defining file (relative to project root)
	Line      int      // 1-based definition line
	Signature string   // human-readable signature when available
	Refs      []string // optional list of paths referencing this symbol
}

// SymbolQuery scopes a SymbolIndex.Lookup.
type SymbolQuery struct {
	Name   string // exact or substring match (impl-defined)
	Path   string // restrict to a single file when set
	Kind   string // restrict to a symbol kind when set
	Limit  int    // 0 means unbounded
}

// SymbolIndex is the L2 architecture interface for cross-file symbol /
// call-graph lookups. The architecture target is a SCIP indexer warmed in
// the background; this package ships NoopSymbolIndex so callers can wire
// the dependency today.
type SymbolIndex interface {
	// Lookup returns symbols matching the query. Implementations should
	// return an empty slice rather than nil on no-match.
	Lookup(q SymbolQuery) ([]Symbol, error)

	// Refresh signals the index to re-scan the given project root.
	// SCIP-backed implementations may run this asynchronously. The Noop
	// implementation is a no-op.
	Refresh(projectRoot string) error
}

// NoopSymbolIndex returns empty results for every query.
type NoopSymbolIndex struct{}

func (NoopSymbolIndex) Lookup(SymbolQuery) ([]Symbol, error) { return []Symbol{}, nil }
func (NoopSymbolIndex) Refresh(string) error                 { return nil }
