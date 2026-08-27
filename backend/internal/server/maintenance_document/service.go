package maintenancedocument

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/sirupsen/logrus"
)

type MaintenanceDocument struct {
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modified_at"`
	Indexed     bool      `json:"indexed"`
}
type Service interface {
	List(context.Context, string, string, string) ([]MaintenanceDocument, error)
	Get(context.Context, string) (MaintenanceDocument, string, error)
	Upload(context.Context, *multipart.FileHeader) (MaintenanceDocument, error)
	Delete(context.Context, string) error
}
type service struct {
	root   string
	index  compose.Runnable[document.Source, bool]
	qdrant interface {
		DeleteDocument(context.Context, string) error
	}
	logger *logrus.Logger
}

func NewService(root string, index compose.Runnable[document.Source, bool], qdrant interface {
	DeleteDocument(context.Context, string) error
}, logger *logrus.Logger) Service {
	return &service{root: root, index: index, qdrant: qdrant, logger: logger}
}
func (s *service) List(_ context.Context, typ, tag, keyword string) ([]MaintenanceDocument, error) {
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return []MaintenanceDocument{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []MaintenanceDocument{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		d, _, err := s.Get(context.Background(), e.Name())
		if err != nil {
			continue
		}
		if typ != "" && d.Type != typ {
			continue
		}
		if tag != "" && !contains(d.Tags, tag) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(d.Title+" "+d.Description+" "+strings.Join(d.Tags, " ")), strings.ToLower(keyword)) {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}
func (s *service) Get(_ context.Context, name string) (MaintenanceDocument, string, error) {
	name = filepath.Base(name)
	if name == "." || !strings.HasSuffix(strings.ToLower(name), ".md") {
		return MaintenanceDocument{}, "", fmt.Errorf("invalid document name")
	}
	path := filepath.Join(s.root, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return MaintenanceDocument{}, "", err
	}
	meta, _, err := Parse(string(b))
	if err != nil {
		return MaintenanceDocument{}, "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return MaintenanceDocument{}, "", err
	}
	return MaintenanceDocument{Name: name, Title: meta.Title, Type: meta.Type, Description: meta.Description, Tags: meta.Tags, Size: info.Size(), ModifiedAt: info.ModTime(), Indexed: true}, string(b), nil
}
func (s *service) Upload(ctx context.Context, file *multipart.FileHeader) (MaintenanceDocument, error) {
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".md") {
		return MaintenanceDocument{}, fmt.Errorf("only Markdown files are supported")
	}
	name := filepath.Base(file.Filename)
	if name != file.Filename {
		return MaintenanceDocument{}, fmt.Errorf("invalid document name")
	}
	src, err := file.Open()
	if err != nil {
		return MaintenanceDocument{}, err
	}
	defer src.Close()
	b, err := io.ReadAll(src)
	if err != nil {
		return MaintenanceDocument{}, err
	}
	meta, _, err := Parse(string(b))
	if err != nil {
		return MaintenanceDocument{}, err
	}
	if err := os.MkdirAll(s.root, 0750); err != nil {
		return MaintenanceDocument{}, err
	}
	path := filepath.Join(s.root, name)
	if err := os.WriteFile(path, b, 0640); err != nil {
		return MaintenanceDocument{}, err
	}
	if _, err = s.index.Invoke(ctx, document.Source{URI: path}); err != nil {
		_ = os.Remove(path)
		return MaintenanceDocument{}, fmt.Errorf("index document: %w", err)
	}
	info, _ := os.Stat(path)
	return MaintenanceDocument{Name: name, Title: meta.Title, Type: meta.Type, Description: meta.Description, Tags: meta.Tags, Size: info.Size(), ModifiedAt: info.ModTime(), Indexed: true}, nil
}
func (s *service) Delete(ctx context.Context, name string) error {
	original := name
	name = filepath.Base(name)
	if name != original || !strings.HasSuffix(strings.ToLower(name), ".md") {
		return fmt.Errorf("invalid document name")
	}
	path := filepath.Join(s.root, name)
	if _, err := os.Stat(path); err != nil {
		return err
	}
	if s.qdrant != nil {
		if err := s.qdrant.DeleteDocument(ctx, name); err != nil {
			return err
		}
	}
	return os.Remove(path)
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if strings.EqualFold(v, target) {
			return true
		}
	}
	return false
}
