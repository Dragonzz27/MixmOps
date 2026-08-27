package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	env := "AUTOOPS_SERVER_HOST=testhost\nAUTOOPS_SERVER_PORT=9999\nEMBEDDING_PROVIDER=ollama\nEMBEDDING_MODEL=test-model\nEMBEDDING_DIMENSION=768\nEMBEDDING_BASE_URL=http://embedding\nAUTOOPS_QDRANT_HOST=qdrant-host\nAUTOOPS_QDRANT_PORT=6334\nAUTOOPS_QDRANT_COLLECTION=test-collection\nPROMETHEUS_URL=http://prometheus:9090\nAUTOOPS_K8S_NAMESPACE=test-ns\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(env), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	cfg, err := InitConfig(filepath.Join(tmpDir, ".env"))
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// 验证 Server 配置
	if cfg.Server.Host != "testhost" {
		t.Errorf("Expected Server.Host=testhost, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected Server.Port=9999, got %d", cfg.Server.Port)
	}

	// 验证 Embedder 配置
	if cfg.Embedding.Model != "test-model" {
		t.Errorf("Expected Embedding.Model=test-model, got %s", cfg.Embedding.Model)
	}
	if cfg.Embedding.Dimension != 768 {
		t.Errorf("Expected Embedding.Dimension=768, got %d", cfg.Embedding.Dimension)
	}

	// 验证 Qdrant 配置
	if cfg.Qdrant.Host != "qdrant-host" {
		t.Errorf("Expected Qdrant.Host=qdrant-host, got %s", cfg.Qdrant.Host)
	}
	if cfg.Qdrant.Collection != "test-collection" {
		t.Errorf("Expected Qdrant.Collection=test-collection, got %s", cfg.Qdrant.Collection)
	}
	if cfg.Prometheus.URL != "http://prometheus:9090" || cfg.Kubernetes.Namespace != "test-ns" {
		t.Fatalf("profile was not merged: %#v", cfg)
	}
}

func TestInitConfigFromFileNotFound(t *testing.T) {
	_, err := InitConfig(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Errorf("missing optional env file should use defaults: %v", err)
	}
}

func TestGetServerAddr(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "localhost", Port: 8080},
	}
	expected := "localhost:8080"
	if addr := cfg.GetServerAddr(); addr != expected {
		t.Errorf("Expected GetServerAddr()=%s, got %s", expected, addr)
	}
}

func TestGetQdrantAddr(t *testing.T) {
	cfg := &Config{
		Qdrant: QdrantConfig{Host: "qdrant", Port: 6334},
	}
	expected := "qdrant:6334"
	if addr := cfg.GetQdrantAddr(); addr != expected {
		t.Errorf("Expected GetQdrantAddr()=%s, got %s", expected, addr)
	}
}
