package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// PrometheusAlert 告警信息结构
type PrometheusAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt"`
	Value       string            `json:"value"`
}

// PrometheusAlertsResult 告警查询结果
type PrometheusAlertsResult struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []PrometheusAlert `json:"alerts"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// PrometheusAlertsOutput 告警查询输出
type PrometheusAlertsOutput struct {
	Success bool              `json:"success" jsonschema:"description=查询是否成功"`
	Alerts  []SimplifiedAlert `json:"alerts,omitempty" jsonschema:"description=活动告警列表，每个告警包含名称、描述、状态、激活时间和持续时间。相同 alertname 的告警只保留第一个"`
	Message string            `json:"message,omitempty" jsonschema:"description=操作结果的状态消息"`
	Error   string            `json:"error,omitempty" jsonschema:"description=如果查询失败，包含错误信息"`
}

// SimplifiedAlert 简化的告警信息
type SimplifiedAlert struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	AlertName   string `json:"alert_name" jsonschema:"description=告警名称，从 Prometheus 告警的 labels.alertname 字段提取"`
	Description string `json:"description" jsonschema:"description=告警描述信息，从 Prometheus 告警的 annotations.description 字段提取"`
	State       string `json:"state" jsonschema:"description=告警状态，通常为 'firing'（触发中）或 'pending'（待触发）"`
	ActiveAt    string `json:"active_at" jsonschema:"description=告警激活时间，RFC3339 格式的时间戳，例如 '2025-10-29T08:48:42.496134755Z'"`
	Duration    string `json:"duration" jsonschema:"description=告警持续时间，从激活时间到当前时间的时长，格式如 '2h30m15s'、'30m15s' 或 '15s'"`
	Severity    string `json:"severity"`
}

// QueryPrometheusAlerts exposes the normalized alert response to HTTP handlers.
func QueryPrometheusAlerts(url string) (PrometheusAlertsOutput, error) {
	result, err := queryPrometheusAlerts(url)
	if err != nil {
		return PrometheusAlertsOutput{Success: false, Error: err.Error()}, err
	}
	seen := map[string]bool{}
	alerts := make([]SimplifiedAlert, 0)
	for _, alert := range result.Data.Alerts {
		name := alert.Labels["alertname"]
		fingerprint := alert.Labels["fingerprint"]
		if fingerprint == "" {
			fingerprint = fingerprintFor(alert.Labels, alert.Annotations)
		}
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		severity := alert.Labels["severity"]
		if severity == "" {
			severity = "unknown"
		}
		alerts = append(alerts, SimplifiedAlert{Fingerprint: fingerprint, AlertName: name, Description: alert.Annotations["description"], State: alert.State, ActiveAt: alert.ActiveAt, Duration: calculateActiveTime(alert.ActiveAt), Severity: severity})
	}
	return PrometheusAlertsOutput{Success: true, Alerts: alerts, Message: fmt.Sprintf("Successfully retrieved %d active alerts", len(alerts))}, nil
}

// queryPrometheusAlerts 查询 Prometheus 告警
func queryPrometheusAlerts(url string) (PrometheusAlertsResult, error) {
	var result PrometheusAlertsResult
	baseURL := url
	alertsURL := baseURL + "/api/v1/alerts"
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(alertsURL)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return result, fmt.Errorf("Prometheus alerts returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}
	if result.Status != "success" {
		if result.Error == "" {
			result.Error = "Prometheus returned a non-success status"
		}
		return result, fmt.Errorf("Prometheus alerts query failed: %s", result.Error)
	}
	return result, nil
}

func fingerprintFor(labels, annotations map[string]string) string {
	keys := make([]string, 0, len(labels)+len(annotations))
	for key := range labels {
		keys = append(keys, "l:"+key)
	}
	for key := range annotations {
		keys = append(keys, "a:"+key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		if strings.HasPrefix(key, "l:") {
			b.WriteString(key + "=" + labels[strings.TrimPrefix(key, "l:")] + "\n")
		} else {
			b.WriteString(key + "=" + annotations[strings.TrimPrefix(key, "a:")] + "\n")
		}
	}
	return fmt.Sprintf("prom-%x", sha256.Sum256([]byte(b.String())))[:20]
}

// 计算从activeAt到当前时间的间隔
func calculateActiveTime(activeAt string) string {
	// 解析 activeAt 字符串为 time.Time
	activeTime, err := time.Parse(time.RFC3339Nano, activeAt)
	if err != nil {
		return "unknown"
	}
	// 计算间隔
	activeDuration := time.Since(activeTime)
	// 格式化持续时间
	hours := int(activeDuration.Hours())
	minutes := int(activeDuration.Minutes()) % 60
	seconds := int(activeDuration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// PrometheusInput Prometheus 工具输入
type PrometheusInput struct {
	// 空输入参数，不需要任何字段
}

// 创建工具
func NewPrometheusAlertsTool(url string) (tool.InvokableTool, error) {
	return utils.InferTool("query_prometheus_alerts",
		"Query active alerts from Prometheus alerting system. Use this tool for incident reports and background operations; it returns labels, annotations, state, severity, activation time, and duration.",
		func(ctx context.Context, input PrometheusInput) (output string, err error) {
			result, err := QueryPrometheusAlerts(url)
			if err != nil {
				return "", err
			}
			alertsOut := result

			// 转换为JSON
			jsonBytes, err := json.Marshal(alertsOut)
			if err != nil {
				log.Printf("Error marshaling alerts result to JSON: %v", err)
				return "", err
			}
			return string(jsonBytes), nil
		})
}
