package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/pendingcalls"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
	v16 "github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/v16"
)

type ResponseHandler func(cpID string, action string, msg message.Message)

type OcppWebSocketClient struct {
	cpID            string
	url             string
	protocol        protocol.Protocol
	codec           *codec.Codec
	pending         *pendingcalls.Registry
	eventBus        *realtime.EventBus
	responseHandler ResponseHandler
	inboundHandler  *InboundCallHandler

	conn               *websocket.Conn
	mu                 sync.Mutex
	connected          bool
	intentionalClose   bool
	cancel             context.CancelFunc
	reconnectCtx       context.Context
	reconnectCancel    context.CancelFunc
}

const (
	reconnectBaseDelay = 1 * time.Second
	reconnectMaxDelay  = 30 * time.Second
	reconnectMaxRetries = 0 // 0 = unlimited
)

func NewOcppWebSocketClient(cpID, url string, protocol protocol.Protocol, eventBus *realtime.EventBus) *OcppWebSocketClient {
	return &OcppWebSocketClient{
		cpID:     cpID,
		url:      url,
		protocol: protocol,
		codec:    codec.New(),
		pending:  pendingcalls.New(),
		eventBus: eventBus,
	}
}

func (c *OcppWebSocketClient) SetResponseHandler(handler ResponseHandler) {
	c.responseHandler = handler
}

func (c *OcppWebSocketClient) SetInboundHandler(handler *InboundCallHandler) {
	c.inboundHandler = handler
}

func (c *OcppWebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.intentionalClose = false
	c.mu.Unlock()

	return c.dial(ctx)
}

func (c *OcppWebSocketClient) dial(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	connURL := fmt.Sprintf("%s/%s", c.url, c.cpID)
	dialer := websocket.Dialer{
		Subprotocols: []string{"ocpp1.6"},
		HandshakeTimeout: 10 * time.Second,
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
	return nil
}

func (c *OcppWebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.intentionalClose = true

	if c.reconnectCancel != nil {
		c.reconnectCancel()
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

func (c *OcppWebSocketClient) Send(msg message.Message) error {
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

	if msg.MessageTypeID == message.CALL {
		c.pending.Register(pendingcalls.PendingCall{
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
	msg, err := c.protocol.BuildBootNotification(ctx, protocol.BootNotificationInput{
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
	msg, err := c.protocol.BuildStatusNotification(ctx, protocol.StatusNotificationInput{
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
	var readErr error
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
			readErr = err
			log.Printf("[%s] read error: %v", c.cpID, err)
			break
		}

		msg, err := c.codec.Decode(raw)
		if err != nil {
			log.Printf("[%s] decode error: %v", c.cpID, err)
			continue
		}

		c.publishOCPPEvent("inbound", msg.Action, json.RawMessage(raw))

		switch msg.MessageTypeID {
		case message.CALL:
			c.handleInboundCall(msg)
		case message.CALLRESULT:
			if call, ok := c.pending.Resolve(msg.UniqueID); ok {
				c.handleResponse(call, msg)
			}
		case message.CALLERROR:
			log.Printf("[%s] CALLERROR: %s - %s", c.cpID, msg.ErrorCode, msg.ErrorDescription)
			c.publishEvent("ocpp.call_error.received", nil,
				fmt.Sprintf("CALLERROR: %s - %s", msg.ErrorCode, msg.ErrorDescription))
		}
	}

	// Connection dropped — check if we should reconnect
	c.mu.Lock()
	intentional := c.intentionalClose
	c.connected = false
	c.mu.Unlock()

	if !intentional && readErr != nil {
		c.scheduleReconnect()
	}
}

func (c *OcppWebSocketClient) scheduleReconnect() {
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.reconnectCtx = ctx
	c.reconnectCancel = cancel
	c.mu.Unlock()

	delay := reconnectBaseDelay
	attempt := 0

	for {
		c.publishEvent("charge_point.reconnecting", nil,
			fmt.Sprintf("Reconnecting in %s (attempt %d)...", delay.Round(time.Millisecond), attempt+1))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		attempt++

		c.mu.Lock()
		intentional := c.intentionalClose
		c.mu.Unlock()
		if intentional {
			return
		}

		err := c.dial(ctx)
		if err != nil {
			log.Printf("[%s] reconnect attempt %d failed: %v", c.cpID, attempt, err)

			delay = delay * 2
			if delay > reconnectMaxDelay {
				delay = reconnectMaxDelay
			}
			if reconnectMaxRetries > 0 && attempt >= reconnectMaxRetries {
				c.publishEvent("charge_point.reconnect_failed", nil,
					fmt.Sprintf("Reconnect failed after %d attempts", attempt))
				return
			}
			continue
		}

		// Reconnected — re-boot
		log.Printf("[%s] reconnected after %d attempts", c.cpID, attempt)
		c.publishEvent("charge_point.connected", nil, "Reconnected")

		if err := c.SendBootNotification(ctx, "Simulator", "OCPP-Sim"); err != nil {
			log.Printf("[%s] re-boot after reconnect failed: %v", c.cpID, err)
		}
		return
	}
}

func (c *OcppWebSocketClient) handleResponse(call pendingcalls.PendingCall, msg message.Message) {
	if c.responseHandler != nil {
		c.responseHandler(c.cpID, call.Action, msg)
	}

	switch call.Action {
	case "StartTransaction":
		resp, err := v16.ParseStartTransactionResponse(msg.Payload)
		if err != nil {
			log.Printf("[%s] parse StartTransaction.conf: %v", c.cpID, err)
			return
		}
		if resp.IDTagInfo.Status == "Accepted" {
			log.Printf("[%s] StartTransaction accepted, transactionId=%d", c.cpID, resp.TransactionID)
		} else {
			log.Printf("[%s] StartTransaction rejected: %s", c.cpID, resp.IDTagInfo.Status)
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

func (c *OcppWebSocketClient) handleInboundCall(msg message.Message) {
	if c.inboundHandler == nil {
		log.Printf("[%s] no inbound handler, rejecting %s", c.cpID, msg.Action)
		errMsg, _ := c.codec.BuildError(msg.UniqueID, message.ErrorCodeNotImplemented,
			fmt.Sprintf("action %s not supported", msg.Action), nil)
		c.Send(errMsg)
		return
	}

	payload, err := c.inboundHandler.Handle(c.cpID, msg)
	if err != nil {
		log.Printf("[%s] inbound %s error: %v", c.cpID, msg.Action, err)
		errMsg, _ := c.codec.BuildError(msg.UniqueID, message.ErrorCodeNotImplemented, err.Error(), nil)
		c.Send(errMsg)
		return
	}

	resultMsg, err := c.codec.BuildResult(msg.UniqueID, payload)
	if err != nil {
		log.Printf("[%s] build CALLRESULT for %s error: %v", c.cpID, msg.Action, err)
		return
	}
	if err := c.Send(resultMsg); err != nil {
		log.Printf("[%s] send CALLRESULT for %s error: %v", c.cpID, msg.Action, err)
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
		RawFrame:      payload,
	})
}

type WireMessageLog struct {
	ChargePointID string
	Direction     string
	Action        string
	Payload       json.RawMessage
	Timestamp     time.Time
}

func (c *OcppWebSocketClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *OcppWebSocketClient) OCPPVersion() string {
	return c.protocol.Version()
}
