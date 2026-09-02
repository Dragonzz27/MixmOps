package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	shared "AutoOps/internal/server/ai/agent/shared"
	airuntime "AutoOps/internal/server/ai/runtime"
	"AutoOps/internal/server/cases"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	OwnerIncident = "incident"
)

type Service struct {
	cases          *cases.Service
	incidentRunner compose.Runnable[*shared.UserMessage, *schema.Message]
}

// StreamEvent is the public SSE-neutral event emitted by an Incident Agent.
// The handler serializes it as an SSE event; keeping it structured here lets
// the frontend distinguish assistant text from worker/tool lifecycle updates.
type StreamEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func NewService(cs *cases.Service, incidentRunner compose.Runnable[*shared.UserMessage, *schema.Message]) *Service {
	return &Service{cases: cs, incidentRunner: incidentRunner}
}
func (s *Service) History(ctx context.Context, owner, id string) ([]cases.Message, error) {
	if !validOwner(owner) {
		return nil, fmt.Errorf("invalid conversation owner")
	}
	return s.cases.Messages(ctx, owner, id)
}
func (s *Service) Chat(ctx context.Context, owner, id, message string) (string, error) {
	query, history, err := s.prepare(ctx, owner, id, message)
	if err != nil {
		return "", err
	}
	if s.incidentRunner == nil {
		return "", fmt.Errorf("agent for %s is unavailable", owner)
	}
	out, err := s.incidentRunner.Invoke(ctx, &shared.UserMessage{ID: owner + ":" + id, Query: query, History: history})
	if err != nil {
		return "", err
	}
	if err = s.persist(ctx, owner, id, message, out.Content); err != nil {
		return "", err
	}
	return out.Content, nil
}
func (s *Service) Stream(ctx context.Context, owner, id, message string, output chan<- StreamEvent) error {
	query, history, err := s.prepare(ctx, owner, id, message)
	if err != nil {
		return err
	}
	if s.incidentRunner == nil {
		return fmt.Errorf("agent for %s is unavailable", owner)
	}
	// Every lifecycle event is forwarded to the HTTP layer. This is a context
	// value rather than a global bus, so concurrent Incident sessions remain
	// isolated.
	runID, err := s.cases.StartAgentRun(ctx, owner, id, "incident-coordinator", message)
	if err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	ctx = airuntime.WithRunContext(ctx, airuntime.RunContext{RunID: runID, OwnerType: owner, OwnerID: id})
	ctx = airuntime.WithEventSink(ctx, func(event airuntime.Event) {
		_ = s.cases.RecordRuntimeEvent(context.Background(), event.Type, event.Data)
		// Keep a compact durable timeline in addition to normalized worker/tool
		// rows, so execution history survives an SSE disconnect or page refresh.
		if event.Type != "assistant" {
			if payload, marshalErr := json.Marshal(event.Data); marshalErr == nil {
				status, eventErr := runtimeEventStatus(event.Type, event.Data)
				_ = s.cases.AddTimeline(context.Background(), owner, id, event.Type, status, string(payload), eventErr)
			}
		}
		select {
		case output <- StreamEvent{Type: event.Type, Data: event.Data}:
		case <-ctx.Done():
		}
	})
	stream, err := s.incidentRunner.Stream(ctx, &shared.UserMessage{ID: owner + ":" + id, Query: query, History: history})
	if err != nil {
		_ = s.cases.FinishAgentRun(context.Background(), runID, "failed", "", err)
		return err
	}
	var full strings.Builder
	for {
		part, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			if isDeadlineError(recvErr, ctx) {
				_ = s.cases.FinishAgentRun(context.Background(), runID, "failed", full.String(), recvErr)
				return fmt.Errorf("Incident Agent 分析超时，请缩小问题范围后重试；已完成的对话内容不会丢失: %w", recvErr)
			}
			_ = s.cases.FinishAgentRun(context.Background(), runID, "failed", full.String(), recvErr)
			return recvErr
		}
		full.WriteString(part.Content)
		select {
		case output <- StreamEvent{Type: "assistant_token", Data: map[string]string{"content": part.Content}}:
		case <-ctx.Done():
			_ = s.cases.FinishAgentRun(context.Background(), runID, "failed", full.String(), ctx.Err())
			return ctx.Err()
		}
	}
	if err := s.persist(ctx, owner, id, message, full.String()); err != nil {
		_ = s.cases.FinishAgentRun(context.Background(), runID, "failed", full.String(), err)
		return err
	}
	return s.cases.FinishAgentRun(context.Background(), runID, "completed", full.String(), nil)
}

func runtimeEventStatus(eventType string, data any) (string, string) {
	status := "running"
	if eventType == "run_completed" || eventType == "worker_completed" || eventType == "tool_completed" {
		status = "completed"
	}
	if eventType == "run_failed" || eventType == "worker_failed" || eventType == "tool_failed" {
		status = "failed"
	}
	var raw map[string]any
	if mapped, ok := data.(map[string]any); ok {
		raw = mapped
	} else if payload, err := json.Marshal(data); err == nil {
		_ = json.Unmarshal(payload, &raw)
	}
	if raw != nil {
		if value, exists := raw["status"].(string); exists && value != "" {
			status = value
		}
		if value, exists := raw["error"].(string); exists {
			return status, value
		}
	}
	return status, ""
}

func isDeadlineError(err error, ctx context.Context) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded")
}
func (s *Service) prepare(ctx context.Context, owner, id, message string) (string, []*schema.Message, error) {
	if strings.TrimSpace(message) == "" {
		return "", nil, fmt.Errorf("message is required")
	}
	stored, err := s.History(ctx, owner, id)
	if err != nil {
		return "", nil, err
	}
	history := make([]*schema.Message, 0, len(stored))
	for _, m := range stored {
		if m.Role == "user" {
			history = append(history, schema.UserMessage(m.Content))
		} else {
			history = append(history, schema.AssistantMessage(m.Content, nil))
		}
	}
	prefix, err := s.context(ctx, owner, id)
	if err != nil {
		return "", nil, err
	}
	return prefix + "\n\n用户当前消息：" + message, history, nil
}
func (s *Service) context(ctx context.Context, owner, id string) (string, error) {
	switch owner {
	case OwnerIncident:
		i, err := s.cases.GetIncident(ctx, id)
		if err != nil {
			return "", err
		}
		return "你是故障排查 Agent。以下是 Incident 上下文。上下文可能经过长度裁剪，必要时请使用只读工具获取最新证据：\n" + truncateContext(string(i.Context), 60000), nil
	default:
		return "", fmt.Errorf("invalid conversation owner")
	}
}
func (s *Service) persist(ctx context.Context, owner, id, user, assistant string) error {
	if err := s.cases.AddMessage(ctx, owner, id, "user", user); err != nil {
		return err
	}
	return s.cases.AddMessage(ctx, owner, id, "assistant", assistant)
}
func validOwner(owner string) bool { return owner == OwnerIncident }

func truncateContext(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "\n...<incident context truncated>"
}
