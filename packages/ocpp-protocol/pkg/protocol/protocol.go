// Package protocol defines the version-agnostic OCPP protocol interface,
// the CP-initiated request input types, the CSMS-initiated request types,
// and the Factory that maps a version string to a Protocol implementation.
// The internal domain model (transactions, connectors, state machines) never
// crosses this boundary; only spec-exact wire payloads do. See plan.md §2b.
package protocol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

type Protocol interface {
	Version() string

	// CP-initiated: build outbound CALL messages
	BuildBootNotification(ctx context.Context, input BootNotificationInput) (message.Message, error)
	BuildHeartbeat(ctx context.Context) (message.Message, error)
	BuildStatusNotification(ctx context.Context, input StatusNotificationInput) (message.Message, error)
	BuildAuthorize(ctx context.Context, input AuthorizeInput) (message.Message, error)
	BuildStartTransaction(ctx context.Context, input StartTransactionInput) (message.Message, error)
	BuildMeterValues(ctx context.Context, input MeterValuesInput) (message.Message, error)
	BuildStopTransaction(ctx context.Context, input StopTransactionInput) (message.Message, error)

	// CSMS-initiated: parse inbound CALL request payloads
	ParseRemoteStartTransactionRequest(payload json.RawMessage) (*RemoteStartTransactionRequest, error)
	ParseRemoteStopTransactionRequest(payload json.RawMessage) (*RemoteStopTransactionRequest, error)
	ParseResetRequest(payload json.RawMessage) (*ResetRequest, error)
	ParseUnlockConnectorRequest(payload json.RawMessage) (*UnlockConnectorRequest, error)
	ParseChangeConfigurationRequest(payload json.RawMessage) (*ChangeConfigurationRequest, error)
	ParseGetConfigurationRequest(payload json.RawMessage) (*GetConfigurationRequest, error)
	ParseTriggerMessageRequest(payload json.RawMessage) (*TriggerMessageRequest, error)
	ParseChangeAvailabilityRequest(payload json.RawMessage) (*ChangeAvailabilityRequest, error)

	// CSMS-initiated: build CALLRESULT response payloads
	BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error)
	BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error)
	BuildResetResponse(status string) (json.RawMessage, error)
	BuildUnlockConnectorResponse(status string) (json.RawMessage, error)
	BuildChangeConfigurationResponse(status string) (json.RawMessage, error)
	BuildGetConfigurationResponse(configKeys []ConfigurationKey, unknownKeys []string) (json.RawMessage, error)
	BuildTriggerMessageResponse(status string) (json.RawMessage, error)
	BuildChangeAvailabilityResponse(status string) (json.RawMessage, error)
}

type BootNotificationInput struct {
	ChargePointVendor string
	ChargePointModel  string
}

type StatusNotificationInput struct {
	ConnectorID int
	Status      string
	ErrorCode   string
}

type AuthorizeInput struct {
	IDTag string
}

type StartTransactionInput struct {
	ConnectorID int
	IDTag       string
	MeterStart  int
	Timestamp   time.Time
}

type MeterValuesInput struct {
	ConnectorID   int
	TransactionID *int
	MeterValues   []MeterValue
}

type MeterValue struct {
	Timestamp    time.Time
	SampledValue []SampledValue
}

type SampledValue struct {
	Value     string
	Measurand string
	Unit      string
}

type StopTransactionInput struct {
	TransactionID int
	IDTag         string
	MeterStop     int
	Timestamp     time.Time
	Reason        string
}

// CSMS-initiated request types

type RemoteStartTransactionRequest struct {
	IDTag           string          `json:"idTag"`
	ConnectorID     *int            `json:"connectorId,omitempty"`
	ChargingProfile json.RawMessage `json:"chargingProfile,omitempty"`
}

type RemoteStopTransactionRequest struct {
	TransactionID int `json:"transactionId"`
}

type ResetRequest struct {
	Type string `json:"type"`
}

type UnlockConnectorRequest struct {
	ConnectorID int `json:"connectorId"`
}

type ChangeConfigurationRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type GetConfigurationRequest struct {
	Key []string `json:"key,omitempty"`
}

type TriggerMessageRequest struct {
	RequestedMessage string `json:"requestedMessage"`
	ConnectorID      *int   `json:"connectorId,omitempty"`
}

type ChangeAvailabilityRequest struct {
	ConnectorID int    `json:"connectorId"`
	Type        string `json:"type"`
}

type ConfigurationKey struct {
	Key      string  `json:"key"`
	Readonly bool    `json:"readonly"`
	Value    *string `json:"value,omitempty"`
}

// Factory maps a version string ("1.6J", "2.0.1", ...) to a registered
// Protocol implementation. main() in each backend service registers the
// versions it supports at startup.
type Factory struct {
	protocols map[string]Protocol
}

func NewFactory() *Factory {
	return &Factory{
		protocols: make(map[string]Protocol),
	}
}

func (f *Factory) Register(p Protocol) {
	f.protocols[p.Version()] = p
}

func (f *Factory) Create(version string) (Protocol, bool) {
	p, ok := f.protocols[version]
	return p, ok
}
