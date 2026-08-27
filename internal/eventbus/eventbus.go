// Package eventbus 提供进程内主题事件总线。
// 所有跨模块通知（新消息、账号状态变更等）统一走 Bus，
// 订阅方：OneBot 正向WS、反向WS、日志、未来扩展的插件等。
package eventbus

import (
	"sync"
	"time"
)

// 预定义主题。
const (
	TopicMessage     = "message"      // 收到新消息 Data: *browser.MessageEvent
	TopicAccount     = "account"      // 账号生命周期状态变更 Data: AccountStateEvent
	TopicReverseWS   = "reverse_ws"   // 反向WS连接状态变更 Data: ReverseWSState
)

const chanBuffer = 256

// Event 总线事件。
type Event struct {
	Topic   string      `json:"topic"`
	Time    int64       `json:"time"`
	Payload interface{} `json:"payload"`
}

type subscriber struct {
	ch     chan Event
	topics map[string]struct{}
}

// Bus 进程内事件总线。慢消费者采用丢弃策略（非阻塞发送）。
type Bus struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

func New() *Bus {
	return &Bus{subs: map[*subscriber]struct{}{}}
}

// Subscription 订阅句柄，调用 Cancel 退订。
type Subscription struct {
	b    *Bus
	sub  *subscriber
	once sync.Once
}

// Ch 只读事件通道。
func (s *Subscription) Ch() <-chan Event { return s.sub.ch }

// Cancel 退订并关闭底层通道。
func (s *Subscription) Cancel() {
	s.once.Do(func() {
		s.b.mu.Lock()
		delete(s.b.subs, s.sub)
		s.b.mu.Unlock()
		close(s.sub.ch)
	})
}

// Subscribe 订阅一个或多个主题。topics 为空表示订阅全部。
func (b *Bus) Subscribe(topics ...string) *Subscription {
	tm := map[string]struct{}{}
	for _, t := range topics {
		tm[t] = struct{}{}
	}
	sub := &subscriber{ch: make(chan Event, chanBuffer), topics: tm}
	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return &Subscription{b: b, sub: sub}
}

// Publish 发布事件到匹配主题的所有订阅者（非阻塞，满则丢弃）。
func (b *Bus) Publish(topic string, payload interface{}) {
	ev := Event{Topic: topic, Time: time.Now().Unix(), Payload: payload}
	b.mu.RLock()
	targets := make([]*subscriber, 0, len(b.subs))
	for s := range b.subs {
		if _, all := s.topics[""]; all {
			targets = append(targets, s)
			continue
		}
		if _, ok := s.topics[topic]; ok {
			targets = append(targets, s)
		}
	}
	b.mu.RUnlock()
	for _, s := range targets {
		select {
		case s.ch <- ev:
		default: // 慢消费者丢帧，避免阻塞发布方
		}
	}
}
