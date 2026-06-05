package processor

import (
	"context"
	"encoding/json"
	"sync"
)

// memoryEmitter collects emitted audit events for assertion.
type memoryEmitter struct {
	mu     sync.Mutex
	events []AuditEvent
}

func (e *memoryEmitter) Emit(ev AuditEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, ev)
}

func (e *memoryEmitter) snapshot() []AuditEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]AuditEvent, len(e.events))
	copy(out, e.events)
	return out
}

// fakeCore implements CoreClient and records every business
// decision request, returning a canned response for each action.
type fakeCore struct {
	mu sync.Mutex

	authorizeCalls       []coreCall
	startTransactionCalls []coreCall
	stopTransactionCalls  []coreCall
	transactionEventCalls []coreCall

	authorizeResp       json.RawMessage
	startTransactionResp json.RawMessage
	stopTransactionResp  json.RawMessage
	transactionEventResp json.RawMessage

	failAuthorize       bool
	failStartTransaction bool
	failStopTransaction  bool
	failTransactionEvent bool
}

type coreCall struct {
	ChargePointID string
	Payload       json.RawMessage
}

func (c *fakeCore) Authorize(_ context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authorizeCalls = append(c.authorizeCalls, coreCall{ChargePointID: chargePointID, Payload: payload})
	if c.failAuthorize {
		return nil, ErrCoreUnavailable
	}
	if c.authorizeResp != nil {
		// Wrap the canned response in a coreEnvelope so the
		// processor extracts it via corePayload.
		return wrapInCoreEnvelope(c.authorizeResp)
	}
	return wrapInCoreEnvelope(json.RawMessage(`{"idTagInfo":{"status":"Accepted"}}`))
}

func (c *fakeCore) StartTransaction(_ context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startTransactionCalls = append(c.startTransactionCalls, coreCall{ChargePointID: chargePointID, Payload: payload})
	if c.failStartTransaction {
		return nil, ErrCoreUnavailable
	}
	if c.startTransactionResp != nil {
		return wrapInCoreEnvelope(c.startTransactionResp)
	}
	return wrapInCoreEnvelope(json.RawMessage(`{"transactionId":42,"idTagInfo":{"status":"Accepted"}}`))
}

func (c *fakeCore) StopTransaction(_ context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopTransactionCalls = append(c.stopTransactionCalls, coreCall{ChargePointID: chargePointID, Payload: payload})
	if c.failStopTransaction {
		return nil, ErrCoreUnavailable
	}
	if c.stopTransactionResp != nil {
		return wrapInCoreEnvelope(c.stopTransactionResp)
	}
	return wrapInCoreEnvelope(json.RawMessage(`{"idTagInfo":{"status":"Accepted"}}`))
}

func (c *fakeCore) TransactionEvent(_ context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transactionEventCalls = append(c.transactionEventCalls, coreCall{ChargePointID: chargePointID, Payload: payload})
	if c.failTransactionEvent {
		return nil, ErrCoreUnavailable
	}
	if c.transactionEventResp != nil {
		return wrapInCoreEnvelope(c.transactionEventResp)
	}
	return wrapInCoreEnvelope(json.RawMessage(`{"idTokenInfo":{"status":"Accepted"}}`))
}

func wrapInCoreEnvelope(payload json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
	}{Payload: payload})
}
