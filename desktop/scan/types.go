// Package scan provides the project deep-scan used by Stage 2 of the desktop
// wizard. It produces a two-layer view of the project:
//   - L1 physical: a file tree from filepath.Walk, skipping noisy directories.
//   - L3 semantic: a JSON summary produced by `claude --print` (modules,
//     entry points, dependencies, hotspots).
package scan

import "time"

// TreeNode is one entry in the L1 physical file tree.
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // relative to scan root
	IsDir    bool        `json:"isDir"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"modTime"`
	Children []*TreeNode `json:"children,omitempty"`
}

// EntryPoint marks a file that boots the project (main, index, etc.).
type EntryPoint struct {
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
}

// Module groups files into a logical unit.
type Module struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Responsibility string `json:"responsibility"`
}

// DepEdge is a directed module-to-module dependency.
type DepEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Hotspot flags a file worth attention (complex, frequently changed, risky).
type Hotspot struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Semantic is the L3 JSON product of the LLM scan.
type Semantic struct {
	Summary     string       `json:"summary"`
	Language    string       `json:"language"`
	EntryPoints []EntryPoint `json:"entryPoints"`
	Modules     []Module     `json:"modules"`
	Deps        []DepEdge    `json:"deps"`
	Hotspots    []Hotspot    `json:"hotspots"`
}
