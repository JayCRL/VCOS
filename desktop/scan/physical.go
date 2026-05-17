package scan

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// skipDirs are directories we never descend into during physical scanning.
// They're either VCS, dependency caches, or build artifacts.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	".next": true, ".cache": true, "target": true, ".idea": true,
	".vscode": true, "vendor": true, "__pycache__": true,
	".turbo": true, ".parcel-cache": true, ".gradle": true, ".m2": true,
	"bin": true, "obj": true, ".venv": true, "venv": true, "env": true,
}

// maxFiles caps the total entries we'll surface. The GUI doesn't need a
// fully exhaustive tree — it needs a representative one. 2000 is plenty for
// most projects and keeps the JSON payload < 1 MB.
const maxFiles = 2000

// WalkPhysical produces the L1 file tree rooted at `root`. The returned
// TreeNode has Path = "." (relative root); descendants carry a path relative
// to root using forward slashes.
func WalkPhysical(root string) (*TreeNode, error) {
	rootName := filepath.Base(root)
	if rootName == "" || rootName == "." {
		abs, _ := filepath.Abs(root)
		rootName = filepath.Base(abs)
		if rootName == "" {
			rootName = "(root)"
		}
	}
	rootNode := &TreeNode{
		Name:    rootName,
		Path:    ".",
		IsDir:   true,
		ModTime: time.Now(),
	}
	count := 0
	if err := walk(rootNode, root, ".", &count); err != nil {
		return rootNode, err
	}
	return rootNode, nil
}

func walk(parent *TreeNode, abs, rel string, count *int) error {
	if *count >= maxFiles {
		return nil
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if skipDirs[name] {
			continue
		}
		// Skip hidden directories (but allow hidden files like .env to surface).
		if e.IsDir() && strings.HasPrefix(name, ".") {
			continue
		}
		*count++
		if *count > maxFiles {
			return nil
		}
		full := filepath.Join(abs, name)
		childRel := name
		if rel != "." && rel != "" {
			childRel = rel + "/" + name
		}
		info, err := e.Info()
		var size int64
		var mt time.Time
		if err == nil {
			size = info.Size()
			mt = info.ModTime()
		}
		child := &TreeNode{
			Name:    name,
			Path:    childRel,
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: mt,
		}
		parent.Children = append(parent.Children, child)
		if e.IsDir() {
			if err := walk(child, full, childRel, count); err != nil {
				// Continue on permission / IO errors; don't bubble up and
				// abort the whole walk.
				continue
			}
		}
	}
	return nil
}
