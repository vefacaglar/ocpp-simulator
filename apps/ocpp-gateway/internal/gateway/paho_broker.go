package gateway

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// PahoBroker is the production Broker implementation wrapping an
// eclipse/paho.mqtt.golang client. It is safe to call Publish/Subscribe
// from any goroutine. Subscribe stores the handler in a map keyed by
// topic so the returned unsubscribe can detach exactly that handler.
type PahoBroker struct {
	client   mqtt.Client
	brokerMu sync.Mutex
	// handlers keeps the Go function references alive so the GC
	// doesn't collect the closures paho invokes.
	handlers map[string]mqtt.MessageHandler
}

// NewPahoBroker creates a broker that dials the given URL with the
// supplied client id. The client is built with reasonable defaults
// (5s keepalive, 30s clean timeout, automatic reconnect with backoff)
// and is not connected yet — call Connect.
func NewPahoBroker(brokerURL, clientID string) (*PahoBroker, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetKeepAlive(5 * time.Second).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectTimeout(10 * time.Second).
		SetMaxReconnectInterval(30 * time.Second).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	return &PahoBroker{
		client:   mqtt.NewClient(opts),
		handlers: make(map[string]mqtt.MessageHandler),
	}, nil
}

// Connect dials the broker. Returns an error if the underlying client
// is not in a connectable state after the timeout.
func (b *PahoBroker) Connect(ctx context.Context) error {
	if b.client.IsConnected() {
		return nil
	}
	token := b.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return errors.New("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}
	// Re-subscribe any handlers that were registered before Connect
	// (Subscribe allows pre-connect registration; paho applies them
	// when the link comes up).
	b.brokerMu.Lock()
	defer b.brokerMu.Unlock()
	for topic, handler := range b.handlers {
		b.client.Subscribe(topic, 0, handler)
	}
	log.Printf("[ocpp-gateway/mqtt] connected to broker")
	return nil
}

// Disconnect cleanly closes the broker link.
func (b *PahoBroker) Disconnect() error {
	if !b.client.IsConnected() {
		return nil
	}
	b.client.Disconnect(1000)
	return nil
}

// IsConnected returns true when the underlying paho client reports a
// live link.
func (b *PahoBroker) IsConnected() bool { return b.client.IsConnected() }

// Publish forwards payload to topic with qos=0, retain=false. The
// payload is passed straight to paho without copying or mutation.
func (b *PahoBroker) Publish(topic string, payload []byte) error {
	if !b.client.IsConnected() {
		return errors.New("mqtt not connected")
	}
	token := b.client.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return errors.New("mqtt publish timeout")
	}
	return token.Error()
}

// Subscribe registers handler for messages delivered to topic. The
// returned function unsubscribes and removes the handler so a future
// subscription on the same topic can register a fresh one.
func (b *PahoBroker) Subscribe(topic string, handler func(payload []byte)) (func(), error) {
	wrapped := func(_ mqtt.Client, m mqtt.Message) {
		// m.Payload() is a fresh slice owned by paho; we hand it
		// off as-is. Re-slicing is not necessary for the gateway
		// since the WS writer copies into the network buffer.
		handler(m.Payload())
	}
	b.brokerMu.Lock()
	b.handlers[topic] = wrapped
	b.brokerMu.Unlock()

	if b.client.IsConnected() {
		token := b.client.Subscribe(topic, 0, wrapped)
		if !token.WaitTimeout(5 * time.Second) {
			return nil, errors.New("mqtt subscribe timeout")
		}
		if err := token.Error(); err != nil {
			return nil, err
		}
	}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			if b.client.IsConnected() {
				b.client.Unsubscribe(topic)
			}
			b.brokerMu.Lock()
			delete(b.handlers, topic)
			b.brokerMu.Unlock()
		})
	}
	return unsubscribe, nil
}
