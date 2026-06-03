package broker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

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
		// OrderMatters=false: each inbound message is dispatched
		// in its own goroutine. Without this, a single slow
		// canonical-log insert could head-of-line block every
		// other message's persistence.
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
	log.Printf("[ocpp-core/mqtt] connected to broker")
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
		return ErrBrokerDown
	}
	token := b.client.Publish(topic, 0, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return errors.New("mqtt publish timeout")
	}
	return token.Error()
}

func (b *PahoBroker) Subscribe(topic string, handler func(string, []byte)) (func(), error) {
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
	unsub := func() {
		once.Do(func() {
			if b.client.IsConnected() {
				b.client.Unsubscribe(topic)
			}
			b.brokerMu.Lock()
			delete(b.handlers, topic)
			b.brokerMu.Unlock()
		})
	}
	return unsub, nil
}
