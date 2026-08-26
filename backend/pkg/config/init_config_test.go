package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	base := "server:\n  host: testhost\n  port: 9999\nembedder:\n  host: embedder-host\n  port: 11434\n  model: test-model\n  dimension: 768\nqdrant:\n  host: qdrant-host\n  port: 6334\n  collection: test-collection\n"
	profile := "prometheus:\n  url: http://prometheus:9090\nkubernetes:\n  namespace: test-ns\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "base.yaml"), []byte(base), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "profiles", "test.yaml"), []byte(profile), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := InitConfig(tmpDir, "test")
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
	if cfg.Embedder.Host != "embedder-host" {
		t.Errorf("Expected Embedder.Host=embedder-host, got %s", cfg.Embedder.Host)
	}
	if cfg.Embedder.Model != "test-model" {
		t.Errorf("Expected Embedder.Model=test-model, got %s", cfg.Embedder.Model)
	}
	if cfg.Embedder.Dimension != 768 {
		t.Errorf("Expected Embedder.Dimension=768, got %d", cfg.Embedder.Dimension)
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
	_, err := InitConfig("nonexistent", "missing")
	if err == nil {
		t.Error("Expected error for nonexistent config file")
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

func TestGetEmbedderAddr(t *testing.T) {
	cfg := &Config{
		Embedder: EmbedderConfig{Host: "embedder", Port: 11434},
	}
	expected := "http://embedder:11434"
	if addr := cfg.GetEmbedderAddr(); addr != expected {
		t.Errorf("Expected GetEmbedderAddr()=%s, got %s", expected, addr)
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
