package conversation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	shared "AutoOps/internal/server/ai/agent/shared"
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
func (s *Service) Stream(ctx context.Context, owner, id, message string, output chan<- string) error {
	query, history, err := s.prepare(ctx, owner, id, message)
	if err != nil {
		return err
	}
	if s.incidentRunner == nil {
		return fmt.Errorf("agent for %s is unavailable", owner)
	}
	stream, err := s.incidentRunner.Stream(ctx, &shared.UserMessage{ID: owner + ":" + id, Query: query, History: history})
	if err != nil {
		return err
	}
	var full strings.Builder
	for {
		part, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			if errors.Is(recvErr, context.DeadlineExceeded) {
				return fmt.Errorf("Incident Agent 分析超时，请缩小问题范围后重试；已完成的对话内容不会丢失: %w", recvErr)
			}
			return recvErr
		}
		full.WriteString(part.Content)
		select {
		case output <- part.Content:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.persist(ctx, owner, id, message, full.String())
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
