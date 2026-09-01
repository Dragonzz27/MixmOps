package runtime

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Registry struct {
	mu    sync.RWMutex
	items map[string]AgentDefinition
}

func Load(root string) (*Registry, error) {
	r := &Registry{items: map[string]AgentDefinition{}}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			return nil, e
		}
		parts := strings.SplitN(string(b), "---", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid agent persona %s", f)
		}
		var d AgentDefinition
		if e = yaml.Unmarshal([]byte(parts[1]), &d); e != nil {
			return nil, e
		}
		if d.Name == "" {
			return nil, fmt.Errorf("agent name required")
		}
		if d.PermissionMode == "" {
			d.PermissionMode = ReadOnly
		}
		if d.PermissionMode != ReadOnly && d.PermissionMode != Proposal {
			return nil, fmt.Errorf("agent %s has invalid permission mode %q", d.Name, d.PermissionMode)
		}
		if _, ok := r.items[d.Name]; ok {
			return nil, fmt.Errorf("duplicate agent %q", d.Name)
		}
		d.Prompt = strings.TrimSpace(parts[2])
		if d.MaxTurns <= 0 {
			d.MaxTurns = 12
		}
		r.items[d.Name] = d
	}
	return r, nil
}
func (r *Registry) Get(name string) (AgentDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.items[name]
	return d, ok
}
func (r *Registry) All() []AgentDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AgentDefinition, 0, len(r.items))
	for _, d := range r.items {
		out = append(out, d)
	}
	return out
}
