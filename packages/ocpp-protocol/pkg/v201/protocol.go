// Package v201 is a placeholder for the OCPP 2.0.1 protocol implementation.
// The codec, message envelope, protocol interface, and pending-call
// registry are already version-agnostic in pkg/codec, pkg/message,
// pkg/protocol, and pkg/pendingcalls, so the actual 2.0.1 wire format
// (string transactionId, TransactionEvent flow, device model, etc.) can be
// added here without touching the rest of the stack. The Factory already
// accepts any Protocol implementation keyed by the value returned from
// Version(), so main() can register this stub once the schemas land in
// packages/ocpp-schemas/v201.
package v201

import (
	"context"
	"encoding/json"

	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

type Protocol struct {
	codec *codec.Codec
}

func NewProtocol() *Protocol {
	return &Protocol{codec: codec.New()}
}

func (p *Protocol) Version() string {
	return "2.0.1"
}

// Compile-time check that the placeholder satisfies the interface.
var _ protocol.Protocol = (*Protocol)(nil)

// OCPP 2.0.1 uses a fundamentally different transaction model and message
// set (TransactionEvent, StatusNotification, BootNotification, etc., all
// carried over the same data transfer with a string transactionId). Until
// the actual wire format and packages/ocpp-schemas/v201 are added, the
// Build* methods below return ErrNotImplemented so the Factory can
// advertise the version but no real frames are produced.
var ErrNotImplemented = errNotImplemented{}

type errNotImplemented struct{}

func (errNotImplemented) Error() string { return "ocpp 2.0.1 protocol not implemented yet" }

func (p *Protocol) BuildBootNotification(ctx context.Context, input protocol.BootNotificationInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildHeartbeat(ctx context.Context) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildStatusNotification(ctx context.Context, input protocol.StatusNotificationInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildAuthorize(ctx context.Context, input protocol.AuthorizeInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildStartTransaction(ctx context.Context, input protocol.StartTransactionInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildMeterValues(ctx context.Context, input protocol.MeterValuesInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) BuildStopTransaction(ctx context.Context, input protocol.StopTransactionInput) (message.Message, error) {
	return message.Message{}, ErrNotImplemented
}

func (p *Protocol) ParseRemoteStartTransactionRequest(payload json.RawMessage) (*protocol.RemoteStartTransactionRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseRemoteStopTransactionRequest(payload json.RawMessage) (*protocol.RemoteStopTransactionRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseResetRequest(payload json.RawMessage) (*protocol.ResetRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseUnlockConnectorRequest(payload json.RawMessage) (*protocol.UnlockConnectorRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseChangeConfigurationRequest(payload json.RawMessage) (*protocol.ChangeConfigurationRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseGetConfigurationRequest(payload json.RawMessage) (*protocol.GetConfigurationRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseTriggerMessageRequest(payload json.RawMessage) (*protocol.TriggerMessageRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) ParseChangeAvailabilityRequest(payload json.RawMessage) (*protocol.ChangeAvailabilityRequest, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildResetResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildUnlockConnectorResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildChangeConfigurationResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildGetConfigurationResponse(configKeys []protocol.ConfigurationKey, unknownKeys []string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildTriggerMessageResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}

func (p *Protocol) BuildChangeAvailabilityResponse(status string) (json.RawMessage, error) {
	return nil, ErrNotImplemented
}
