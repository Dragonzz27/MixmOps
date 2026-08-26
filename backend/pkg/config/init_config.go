package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Embedder   EmbedderConfig   `mapstructure:"embedder"`
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

// EmbedderConfig 嵌入模型配置
type EmbedderConfig struct {
	Host      string `mapstructure:"host"`
	Port      int    `mapstructure:"port"`
	Model     string `mapstructure:"model"`
	Dimension int    `mapstructure:"dimension"`
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

// InitConfig loads the base YAML configuration and a named environment profile.
func InitConfig(configDir string, profile string) (*Config, error) {
	if configDir == "" {
		configDir = "./config"
	}
	if profile == "" {
		profile = "minikube"
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(filepath.Join(configDir, "base.yaml"))
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read base config: %w", err)
	}
	profileFile := filepath.Join(configDir, "profiles", profile+".yaml")
	profileViper := viper.New()
	profileViper.SetConfigType("yaml")
	profileViper.SetConfigFile(profileFile)
	if err := profileViper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read profile %q: %w", profile, err)
	}
	if err := v.MergeConfigMap(profileViper.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to merge profile %q: %w", profile, err)
	}
	v.SetEnvPrefix("AUTOOPS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper) {
	// Server 默认值
	v.SetDefault("server.host", "localhost")
	v.SetDefault("server.port", 8819)

	// Embedder 默认值
	v.SetDefault("embedder.host", "localhost")
	v.SetDefault("embedder.port", 11434)
	v.SetDefault("embedder.model", "nomic-embed-text")
	v.SetDefault("embedder.dimension", 384)

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

// GetEmbedderAddr 获取嵌入模型服务地址
func (c *Config) GetEmbedderAddr() string {
	return fmt.Sprintf("http://%s:%d", c.Embedder.Host, c.Embedder.Port)
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
