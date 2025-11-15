package market

import "context"

// Message represents a simple pub/sub payload used by the reference runtime.
type Message struct {
	Data []byte
}

// MessageHandler processes pub/sub messages.
type MessageHandler func(ctx context.Context, msg *Message) error

// MessageBus captures the subset of functionality the reference runtime needs.
// The production integration will adapt the real P2P layer to this interface.
type MessageBus interface {
	Subscribe(topic string, handler MessageHandler) error
	Publish(topic string, data []byte) error
}
