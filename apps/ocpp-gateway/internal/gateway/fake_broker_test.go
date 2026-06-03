package gateway

import (
	"context"
	"sync"
)

// fakeBroker is a test double for Broker. It records every Publish
// and every Subscribe in memory and lets tests fire handler
// callbacks synchronously. Subscriptions are identified by a
// monotonically increasing id rather than by Go func-value identity,
// because two anonymous closures in the same package can share the
// same underlying code address and `reflect.ValueOf(f).Pointer()` is
// not a stable identity for them.
type fakeBroker struct {
	mu          sync.Mutex
	published   map[string][][]byte
	subs        map[subID]fakeSub
	byTopic     map[string][]subID
	nextSub     subID
	connected   bool
	disconnects int
}

type subID uint64

type fakeSub struct {
	topic   string
	handler func([]byte)
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{
		published: make(map[string][][]byte),
		subs:      make(map[subID]fakeSub),
		byTopic:   make(map[string][]subID),
		connected: true,
	}
}

func (b *fakeBroker) Publish(topic string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.connected {
		return errBrokerDown
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.published[topic] = append(b.published[topic], cp)
	return nil
}

func (b *fakeBroker) Subscribe(topic string, handler func([]byte)) (func(), error) {
	b.mu.Lock()
	b.nextSub++
	id := b.nextSub
	b.subs[id] = fakeSub{topic: topic, handler: handler}
	b.byTopic[topic] = append(b.byTopic[topic], id)
	b.mu.Unlock()

	var once sync.Once
	detach := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.subs, id)
			ids := b.byTopic[topic]
			out := ids[:0]
			for _, x := range ids {
				if x == id {
					continue
				}
				out = append(out, x)
			}
			b.byTopic[topic] = out
		})
	}
	return detach, nil
}

func (b *fakeBroker) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.connected
}

func (b *fakeBroker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = true
	return nil
}

func (b *fakeBroker) Disconnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disconnects++
	b.connected = false
	return nil
}

// deliver fans out payload to every handler registered on topic. The
// handlers are invoked with the broker lock released so re-entrant
// calls (e.g. a handler that triggers another Subscribe) are safe.
func (b *fakeBroker) deliver(topic string, payload []byte) int {
	b.mu.Lock()
	ids := append([]subID(nil), b.byTopic[topic]...)
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.mu.Unlock()
	for _, id := range ids {
		b.mu.Lock()
		s, ok := b.subs[id]
		b.mu.Unlock()
		if ok {
			s.handler(cp)
		}
	}
	return len(ids)
}

func (b *fakeBroker) subscriberCount(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.byTopic[topic])
}

func (b *fakeBroker) publishedOn(topic string) [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.published[topic]))
	copy(out, b.published[topic])
	return out
}

var errBrokerDown = errBrokerDownT{}

type errBrokerDownT struct{}

func (errBrokerDownT) Error() string { return "broker down" }
