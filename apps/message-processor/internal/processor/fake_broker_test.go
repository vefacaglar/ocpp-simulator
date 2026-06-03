package processor

import (
	"context"
	"sync"
)

// fakeBroker records Publish calls and lets tests trigger Subscribe
// handlers synchronously with a chosen topic. Subscribe handlers
// are identified by a subID (monotonic) so anonymous closures can
// be detached reliably.
type fakeBroker struct {
	mu        sync.Mutex
	published map[string][][]byte
	subs      map[subID]fakeSub
	byTopic   map[string][]subID
	nextSub   subID
	connected bool
}

type subID uint64

type fakeSub struct {
	topic   string
	handler func(topic string, payload []byte)
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

func (b *fakeBroker) Subscribe(topic string, handler func(string, []byte)) (func(), error) {
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

func (b *fakeBroker) Connect(_ context.Context) error { return nil }

func (b *fakeBroker) Disconnect() error { return nil }

// deliver fans out payload to every subscriber of subscriptionTopic.
// subscriptionTopic is the topic the processor subscribed to
// (e.g. "ocpp/+/in"); originatingTopic is the actual per-CP topic
// the message arrived on (e.g. "ocpp/CP-001/in") which is what
// gets passed to the handler.
func (b *fakeBroker) deliver(subscriptionTopic, originatingTopic string, payload []byte) int {
	b.mu.Lock()
	ids := append([]subID(nil), b.byTopic[subscriptionTopic]...)
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.mu.Unlock()
	for _, id := range ids {
		b.mu.Lock()
		s, ok := b.subs[id]
		b.mu.Unlock()
		if ok {
			s.handler(originatingTopic, cp)
		}
	}
	return len(ids)
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
