package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type PrometheusQueryInput struct {
	Query string `json:"query" jsonschema:"description=PromQL instant query"`
}

func NewPrometheusQueryTool(baseURL string) (tool.InvokableTool, error) {
	return utils.InferTool("query_prometheus_metrics", "Execute a bounded PromQL instant query for evidence such as error rate, latency, CPU, memory, or queue depth.", func(ctx context.Context, in PrometheusQueryInput) (string, error) {
		if strings.TrimSpace(in.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/query?query=" + url.QueryEscape(in.Query)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("Prometheus query returned HTTP %d: %s", resp.StatusCode, string(body))
		}
		var result struct {
			Status string `json:"status"`
			Data   any    `json:"data"`
			Error  string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}
		if result.Status != "success" {
			return "", fmt.Errorf("Prometheus query failed: %s", result.Error)
		}
		out, _ := json.Marshal(map[string]any{"query": in.Query, "data": result.Data})
		return string(out), nil
	})
}
