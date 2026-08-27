package maintenancedocument

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	TypeClusterArchitecture = "cluster-architecture"
	TypeOperationsManual    = "operations-manual"
	TypeIncidentRunbook     = "incident-runbook"
	TypeChangeStandard      = "change-standard"
)

var validTypes = map[string]bool{
	TypeClusterArchitecture: true,
	TypeOperationsManual:    true,
	TypeIncidentRunbook:     true,
	TypeChangeStandard:      true,
}

type Metadata struct {
	Title       string   `yaml:"title" json:"title"`
	Type        string   `yaml:"type" json:"type"`
	Description string   `yaml:"description" json:"description"`
	Tags        []string `yaml:"tags" json:"tags"`
}

// Parse validates YAML Front Matter and returns metadata plus Markdown body.
func Parse(content string) (Metadata, string, error) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return Metadata{}, "", fmt.Errorf("missing YAML Front Matter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return Metadata{}, "", fmt.Errorf("unclosed YAML Front Matter")
	}
	var metadata Metadata
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata); err != nil {
		return Metadata{}, "", fmt.Errorf("invalid YAML Front Matter: %w", err)
	}
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Type = strings.TrimSpace(metadata.Type)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if metadata.Title == "" {
		return Metadata{}, "", fmt.Errorf("Front Matter title is required")
	}
	if !validTypes[metadata.Type] {
		return Metadata{}, "", fmt.Errorf("invalid document type %q", metadata.Type)
	}
	for i := range metadata.Tags {
		metadata.Tags[i] = strings.TrimSpace(metadata.Tags[i])
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if body == "" || !hasH1(body) {
		return Metadata{}, "", fmt.Errorf("document body must contain a level-one heading")
	}
	return metadata, body, nil
}

func hasH1(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return true
		}
	}
	return false
}
