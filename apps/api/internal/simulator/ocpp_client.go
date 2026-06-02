package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp/v16"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
)

type OcppWebSocketClient struct {
	cpID      string
	url       string
	protocol  ocpp.Protocol
	codec     *ocpp.Codec
	pending   *ocpp.PendingCallRegistry
	eventBus  *realtime.EventBus

	conn     *websocket.Conn
	mu       sync.Mutex
	connected bool
	cancel   context.CancelFunc
}

func NewOcppWebSocketClient(cpID, url string, protocol ocpp.Protocol, eventBus *realtime.EventBus) *OcppWebSocketClient {
	return &OcppWebSocketClient{
		cpID:     cpID,
		url:      url,
		protocol: protocol,
		codec:    ocpp.NewCodec(),
		pending:  ocpp.NewPendingCallRegistry(),
		eventBus: eventBus,
	}
}

func (c *OcppWebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	connURL := fmt.Sprintf("%s/%s", c.url, c.cpID)
	dialer := websocket.Dialer{
		Subprotocols: []string{"ocpp1.6"},
	}

	conn, _, err := dialer.DialContext(ctx, connURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	c.connected = true

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go c.readLoop(ctx)

	c.publishEvent("charge_point.connected", nil, fmt.Sprintf("Connected to %s", connURL))
	return nil
}

func (c *OcppWebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	c.publishEvent("charge_point.disconnected", nil, "Disconnected")
	return nil
}

func (c *OcppWebSocketClient) Send(msg ocpp.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	raw, err := c.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	if msg.MessageTypeID == ocpp.CALL {
		c.pending.Register(ocpp.PendingCall{
			UniqueID:  msg.UniqueID,
			Action:    msg.Action,
			SentAt:    time.Now(),
			TimeoutAt: time.Now().Add(30 * time.Second),
		})
	}

	c.publishOCPPEvent("outbound", msg.Action, json.RawMessage(raw))
	return nil
}

func (c *OcppWebSocketClient) SendBootNotification(ctx context.Context, vendor, model string) error {
	msg, err := c.protocol.BuildBootNotification(ctx, ocpp.BootNotificationInput{
		ChargePointVendor: vendor,
		ChargePointModel:  model,
	})
	if err != nil {
		return err
	}
	return c.Send(msg)
}

func (c *OcppWebSocketClient) SendHeartbeat(ctx context.Context) error {
	msg, err := c.protocol.BuildHeartbeat(ctx)
	if err != nil {
		return err
	}
	return c.Send(msg)
}

func (c *OcppWebSocketClient) SendStatusNotification(ctx context.Context, connectorID int, status, errorCode string) error {
	msg, err := c.protocol.BuildStatusNotification(ctx, ocpp.StatusNotificationInput{
		ConnectorID: connectorID,
		Status:      status,
		ErrorCode:   errorCode,
	})
	if err != nil {
		return err
	}
	return c.Send(msg)
}

func (c *OcppWebSocketClient) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("[%s] read error: %v", c.cpID, err)
			return
		}

		msg, err := c.codec.Decode(raw)
		if err != nil {
			log.Printf("[%s] decode error: %v", c.cpID, err)
			continue
		}

		c.publishOCPPEvent("inbound", msg.Action, json.RawMessage(raw))

		switch msg.MessageTypeID {
		case ocpp.CALLRESULT:
			if call, ok := c.pending.Resolve(msg.UniqueID); ok {
				c.handleResponse(call, msg)
			}
		case ocpp.CALLERROR:
			log.Printf("[%s] CALLERROR: %s - %s", c.cpID, msg.ErrorCode, msg.ErrorDescription)
			c.publishEvent("ocpp.call_error.received", nil,
				fmt.Sprintf("CALLERROR: %s - %s", msg.ErrorCode, msg.ErrorDescription))
		}
	}
}

func (c *OcppWebSocketClient) handleResponse(call ocpp.PendingCall, msg ocpp.Message) {
	switch call.Action {
	case "StartTransaction":
		resp, err := v16.ParseStartTransactionResponse(msg.Payload)
		if err != nil {
			log.Printf("[%s] parse StartTransaction.conf: %v", c.cpID, err)
			return
		}
		if resp.IDTagInfo.Status == "Accepted" {
			log.Printf("[%s] StartTransaction accepted, transactionId=%d", c.cpID, resp.TransactionID)
		}
	case "BootNotification":
		resp, err := v16.ParseBootNotificationResponse(msg.Payload)
		if err != nil {
			log.Printf("[%s] parse BootNotification.conf: %v", c.cpID, err)
			return
		}
		log.Printf("[%s] BootNotification %s, interval=%d", c.cpID, resp.Status, resp.Interval)
	case "StopTransaction":
		resp, err := v16.ParseStopTransactionResponse(msg.Payload)
		if err != nil {
			log.Printf("[%s] parse StopTransaction.conf: %v", c.cpID, err)
			return
		}
		_ = resp
		log.Printf("[%s] StopTransaction accepted", c.cpID)
	}
}

func (c *OcppWebSocketClient) publishEvent(eventType string, connectorID *int, message string) {
	c.eventBus.Publish(realtime.Event{
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:          eventType,
		ChargePointID: c.cpID,
		ConnectorID:   connectorID,
		Message:       message,
		Timestamp:     time.Now().UTC(),
	})
}

func (c *OcppWebSocketClient) publishOCPPEvent(direction, action string, payload json.RawMessage) {
	dir := direction
	act := action
	c.eventBus.Publish(realtime.Event{
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:          "ocpp.message." + direction,
		ChargePointID: c.cpID,
		Direction:     &dir,
		Action:        &act,
		Payload:       payload,
		Message:       fmt.Sprintf("%s %s", direction, action),
		Timestamp:     time.Now().UTC(),
	})
}

// WireMessageLog is a callback for persisting OCPP messages to the database.
type WireMessageLog struct {
	ChargePointID string
	Direction     string
	Action        string
	Payload       json.RawMessage
	Timestamp     time.Time
}

// IsConnected returns whether the client is currently connected.
func (c *OcppWebSocketClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Ensure OCPPVersion returns the protocol version string.
func (c *OcppWebSocketClient) OCPPVersion() string {
	return c.protocol.Version()
}
