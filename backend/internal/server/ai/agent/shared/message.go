package shared

import "github.com/cloudwego/eino/schema"

type UserMessage struct {
	ID      string
	Query   string
	History []*schema.Message
}
