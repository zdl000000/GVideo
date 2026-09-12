// Package bus 是进程内同步事件总线：Subscribe 的处理器在 Publish 调用方
// goroutine 内联执行，保持「通知随请求落库」的既有语义；处理器自身的错误
// 由处理器记录，不向发布方传播。
package bus

import (
	"context"
	"sync"
)

// NotificationEvent 承载通知写路径所需的全部字段，字段语义与
// notifications 模块仓库的写入方法一一对应。
type NotificationEvent struct {
	RecipientID    int64  // 通知接收方（视频作者/被关注者）
	ActorID        int64  // 触发方（点赞/评论/关注的人）
	Kind           string // comment | like | favorite | follow
	VideoID        int64  // 0 = 不关联视频
	CommentID      int64  // 0 = 不关联评论
	VideoTitle     string
	CommentPreview string
}

// Handler 处理一条通知事件；实现必须并发安全且不向发布方返回错误。
type Handler func(ctx context.Context, event NotificationEvent)

// Bus 聚合订阅者。所有方法并发安全。
type Bus struct {
	mu       sync.RWMutex
	handlers []Handler
}

func New() *Bus {
	return &Bus{}
}

// Subscribe 注册一个处理器；重复注册会收到重复事件。
func (b *Bus) Subscribe(handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// Publish 向全部订阅者同步分发事件；快照当前订阅者列表后逐一调用，
// 订阅者的 panic 会向发布方传播（与进程内直调一致）。
func (b *Bus) Publish(ctx context.Context, event NotificationEvent) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()
	for _, handler := range handlers {
		handler(ctx, event)
	}
}
