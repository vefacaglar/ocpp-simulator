package processor

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// PahoBroker is the production Broker implementation for the
// processor. It wraps an eclipse/paho.mqtt.golang client with
// reconnection and pre-connect subscription support.
type PahoBroker struct {
	client   mqtt.Client
	brokerMu sync.Mutex
	handlers map[string]mqtt.MessageHandler
}

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
		SetConnectRetryInterval(5 * time.Second).
		// OrderMatters=false makes paho dispatch each inbound
		// message in its own goroutine. Without this, a single
		// slow handler would head-of-line block every other
		// handler — which for the processor means a single slow
		// business-decision HTTP call to ocpp-core would delay
		// every other CP's responses.
		SetOrderMatters(false)

	return &PahoBroker{
		client:   mqtt.NewClient(opts),
		handlers: make(map[string]mqtt.MessageHandler),
	}, nil
}

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
	b.brokerMu.Lock()
	defer b.brokerMu.Unlock()
	for topic, handler := range b.handlers {
		b.client.Subscribe(topic, 0, handler)
	}
	log.Printf("[message-processor/mqtt] connected to broker")
	return nil
}

func (b *PahoBroker) Disconnect() error {
	if !b.client.IsConnected() {
		return nil
	}
	b.client.Disconnect(1000)
	return nil
}

func (b *PahoBroker) IsConnected() bool { return b.client.IsConnected() }

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

func (b *PahoBroker) Subscribe(topic string, handler func(topic string, payload []byte)) (func(), error) {
	wrapped := func(_ mqtt.Client, m mqtt.Message) {
		handler(m.Topic(), m.Payload())
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
