package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Embedding  EmbeddingConfig  `mapstructure:"embedding"`
	Qdrant     QdrantConfig     `mapstructure:"qdrant"`
	OpenAI     OpenAIConfig     `mapstructure:"openai"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	Log        LogConfig        `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// EmbeddingConfig configures the selected embedding provider.
type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`
	Model      string `mapstructure:"model"`
	Dimension  int    `mapstructure:"dimension"`
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	Timeout    string `mapstructure:"timeout"`
	BatchSize  int    `mapstructure:"batch_size"`
	MaxRetries int    `mapstructure:"max_retries"`
	Normalize  bool   `mapstructure:"normalize"`
}

// QdrantConfig Qdrant 向量数据库配置
type QdrantConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
}

// OpenAIConfig OpenAI API 配置
type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	APIBase string `mapstructure:"api_base"`
}

// PrometheusConfig Prometheus 配置
type PrometheusConfig struct {
	URL string `mapstructure:"url"`
}

// KubernetesConfig controls out-of-cluster access to the local test cluster.
type KubernetesConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Kubeconfig string `mapstructure:"kubeconfig"`
	Context    string `mapstructure:"context"`
	Namespace  string `mapstructure:"namespace"`
	InCluster  bool   `mapstructure:"in_cluster"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

// InitConfig loads a dotenv file (normally backend/.env) and environment variables.
// The dotenv file is optional; explicitly exported environment variables win.
func InitConfig(envFile string) (*Config, error) {
	if envFile == "" {
		envFile = ".env"
	}
	if err := loadDotenv(envFile); err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnvironment(v)

	// 设置默认值
	setDefaults(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	if value := os.Getenv("AUTOOPS_K8S_KUBECONFIG"); value != "" {
		cfg.Kubernetes.Kubeconfig = value
	}
	if value := os.Getenv("KUBECONFIG"); value != "" && cfg.Kubernetes.Kubeconfig == "" {
		cfg.Kubernetes.Kubeconfig = value
	}
	if value := os.Getenv("AUTOOPS_K8S_CONTEXT"); value != "" {
		cfg.Kubernetes.Context = value
	}
	if value := os.Getenv("AUTOOPS_K8S_NAMESPACE"); value != "" {
		cfg.Kubernetes.Namespace = value
	}
	if value := os.Getenv("AUTOOPS_K8S_ENABLED"); value != "" {
		cfg.Kubernetes.Enabled = strings.EqualFold(value, "true") || value == "1"
	}
	if value := os.Getenv("EMBEDDING_API_KEY"); value != "" {
		cfg.Embedding.APIKey = value
	}
	if value := os.Getenv("OPENAI_API_KEY"); value != "" && cfg.Embedding.APIKey == "" {
		cfg.Embedding.APIKey = value
	}
	if value := os.Getenv("EMBEDDING_PROVIDER"); value != "" {
		cfg.Embedding.Provider = value
	}
	if value := os.Getenv("EMBEDDING_MODEL"); value != "" {
		cfg.Embedding.Model = value
	}
	if value := os.Getenv("EMBEDDING_BASE_URL"); value != "" {
		cfg.Embedding.BaseURL = value
	}
	return &cfg, nil
}

func loadDotenv(path string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}
	return nil
}

func bindEnvironment(v *viper.Viper) {
	for key, env := range map[string]string{
		"server.host": "AUTOOPS_SERVER_HOST", "server.port": "AUTOOPS_SERVER_PORT",
		"embedding.provider": "EMBEDDING_PROVIDER", "embedding.model": "EMBEDDING_MODEL", "embedding.dimension": "EMBEDDING_DIMENSION", "embedding.base_url": "EMBEDDING_BASE_URL", "embedding.api_key": "EMBEDDING_API_KEY", "embedding.timeout": "EMBEDDING_TIMEOUT", "embedding.batch_size": "EMBEDDING_BATCH_SIZE", "embedding.max_retries": "EMBEDDING_MAX_RETRIES", "embedding.normalize": "EMBEDDING_NORMALIZE",
		"qdrant.host": "AUTOOPS_QDRANT_HOST", "qdrant.port": "AUTOOPS_QDRANT_PORT", "qdrant.collection": "AUTOOPS_QDRANT_COLLECTION",
		"openai.api_key": "OPENAI_API_KEY", "openai.model": "OPENAI_MODEL", "openai.api_base": "OPENAI_API_BASE",
		"prometheus.url": "PROMETHEUS_URL", "kubernetes.enabled": "AUTOOPS_K8S_ENABLED", "kubernetes.kubeconfig": "AUTOOPS_K8S_KUBECONFIG", "kubernetes.context": "AUTOOPS_K8S_CONTEXT", "kubernetes.namespace": "AUTOOPS_K8S_NAMESPACE", "kubernetes.in_cluster": "AUTOOPS_K8S_IN_CLUSTER", "log.level": "AUTOOPS_LOG_LEVEL", "log.file": "AUTOOPS_LOG_FILE",
	} {
		_ = v.BindEnv(key, env)
	}
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// Server 默认值
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8819)

	// Embedder 默认值
	v.SetDefault("embedding.provider", "openai")
	v.SetDefault("embedding.model", "text-embedding-3-small")
	v.SetDefault("embedding.dimension", 1536)
	v.SetDefault("embedding.base_url", "https://api.openai.com/v1")
	v.SetDefault("embedding.timeout", "30s")
	v.SetDefault("embedding.batch_size", 32)
	v.SetDefault("embedding.max_retries", 2)
	v.SetDefault("embedding.normalize", true)

	// Qdrant 默认值
	v.SetDefault("qdrant.host", "localhost")
	v.SetDefault("qdrant.port", 6334)
	v.SetDefault("qdrant.collection", "autoops")

	// OpenAI 默认值
	v.SetDefault("openai.api_key", "")
	v.SetDefault("openai.model", "minimax/minimax-m2.1")
	v.SetDefault("openai.api_base", "https://api.qnaigc.com/v1")

	// Prometheus 默认值
	v.SetDefault("prometheus.url", "http://localhost:9090")

	// Kubernetes 默认值
	v.SetDefault("kubernetes.enabled", true)
	v.SetDefault("kubernetes.kubeconfig", "")
	v.SetDefault("kubernetes.context", "")
	v.SetDefault("kubernetes.namespace", "autoops-test")
	v.SetDefault("kubernetes.in_cluster", false)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.file", "log/AutoOps.log")
}

// GetServerAddr 获取服务器完整地址
func (c *Config) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetQdrantAddr 获取 Qdrant 服务地址
func (c *Config) GetQdrantAddr() string {
	return fmt.Sprintf("%s:%d", c.Qdrant.Host, c.Qdrant.Port)
}

// GetPrometheusURL 获取 Prometheus 地址
func (c *Config) GetPrometheusURL() string {
	if c.Prometheus.URL == "" {
		return "http://localhost:9090"
	}
	return c.Prometheus.URL
}
