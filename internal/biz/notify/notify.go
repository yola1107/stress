package notify

import (
	"context"
)

// Message 通知消息
type Message struct {
	Title   string
	Content string
}

// Notifier 通知发送接口
type Notifier interface {
	Send(ctx context.Context, msg *Message) error
}

// Noop 空实现
type Noop struct{}

func (Noop) Send(context.Context, *Message) error { return nil }
