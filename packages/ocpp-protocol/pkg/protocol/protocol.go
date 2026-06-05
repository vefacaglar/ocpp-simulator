// Package protocol defines the version-agnostic OCPP protocol
// surface as a set of small capability interfaces, plus the Factory
// that maps a version string to a registered implementation. The
// internal domain model (transactions, connectors, state machines)
// never crosses this boundary; only spec-exact wire payloads do.
// See plan.md §2b and the multi-version handoff plan §2.
//
// Capability segregation — not a fat Protocol super-interface —
// is intentional: OCPP 1.6J has StartTransaction/StopTransaction and
// Get/ChangeConfiguration; OCPP 2.0.1 has TransactionEvent and
// Get/SetVariables. A single fat interface would force v201 to
// implement 1.6J-only methods (or pretend to, with hidden state).
// Each version struct implements only the capabilities it actually
// supports, and the Factory surfaces them via type-asserted getters
// discovered at registration time.
package protocol

import (
	"context"
	"encoding/json"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

// Canonical version strings. These are the keys the Factory is
// indexed by, the values carried in MQTT topic segments, and the
// wire-level labels persisted in the canonical log. The WebSocket
// subprotocol token is mapped to these strings at the gateway
// boundary: "ocpp1.6" -> Version1_6, "ocpp2.0.1" -> Version2_0_1.
const (
	Version1_6  = "1.6J"
	Version2_0_1 = "2.0.1"
)

// --- Capability interfaces -------------------------------------------------

// BaseProtocol is the cross-version CP→CSMS surface: every OCPP
// charge point can boot, heartbeat, publish status and meter
// samples, and authorize an id token. The input shapes are the
// union of both versions — each version's codec reads what it
// needs (e.g. v16 uses IDTag, v201 uses IDToken).
type BaseProtocol interface {
	Version() string
	BuildBootNotification(ctx context.Context, in BootNotificationInput) (message.Message, error)
	BuildHeartbeat(ctx context.Context) (message.Message, error)
	BuildStatusNotification(ctx context.Context, in StatusNotificationInput) (message.Message, error)
	BuildMeterValues(ctx context.Context, in MeterValuesInput) (message.Message, error)
	BuildAuthorize(ctx context.Context, in AuthorizeInput) (message.Message, error)
}

// LegacyTransactionProtocol is OCPP 1.6J-only. v201 does NOT
// implement this — its transaction flow is TransactionEvent.
type LegacyTransactionProtocol interface {
	BuildStartTransaction(ctx context.Context, in StartTransactionInput) (message.Message, error)
	BuildStopTransaction(ctx context.Context, in StopTransactionInput) (message.Message, error)
}

// TransactionEventProtocol is OCPP 2.0.1-only. v16 does NOT
// implement this. The simulator must track a per-transaction
// monotonically-increasing SeqNo starting at 0; the input struct
// carries it explicitly so it cannot be hidden.
type TransactionEventProtocol interface {
	BuildTransactionEvent(ctx context.Context, in TransactionEventInput) (message.Message, error)
}

// LegacyConfigProtocol is OCPP 1.6J-only (Get/ChangeConfiguration).
// Replaced by the device-model VariableProtocol in 2.0.1.
type LegacyConfigProtocol interface {
	ParseGetConfigurationRequest(p json.RawMessage) (*GetConfigurationRequest, error)
	ParseChangeConfigurationRequest(p json.RawMessage) (*ChangeConfigurationRequest, error)
	BuildGetConfigurationResponse(keys []ConfigurationKey, unknown []string) (json.RawMessage, error)
	BuildChangeConfigurationResponse(status string) (json.RawMessage, error)
}

// VariableProtocol is OCPP 2.0.1-only (Get/SetVariables on the
// device model). The request/response structs differ structurally
// from the 1.6J Get/ChangeConfiguration pair, so this is a separate
// capability rather than a parameterization of LegacyConfigProtocol.
type VariableProtocol interface {
	ParseGetVariablesRequest(p json.RawMessage) (*GetVariablesRequest, error)
	ParseSetVariablesRequest(p json.RawMessage) (*SetVariablesRequest, error)
	BuildGetVariablesResponse(items []GetVariablesResultItem) (json.RawMessage, error)
	BuildSetVariablesResponse(items []SetVariablesResultItem) (json.RawMessage, error)
}

// RemoteControlProtocol covers the four CSMS-initiated actions
// whose request shapes are close enough across versions to share a
// parse/build surface: Reset, UnlockConnector, TriggerMessage,
// ChangeAvailability. The shared Build*Response(status) signatures
// are genuinely version-portable; the request structs stay
// 1.6J-shaped for the MVP (TODO: split when 2.0.1 CSMS-initiated
// parsing is actually exercised against a real CSMS).
type RemoteControlProtocol interface {
	ParseResetRequest(p json.RawMessage) (*ResetRequest, error)
	ParseUnlockConnectorRequest(p json.RawMessage) (*UnlockConnectorRequest, error)
	ParseTriggerMessageRequest(p json.RawMessage) (*TriggerMessageRequest, error)
	ParseChangeAvailabilityRequest(p json.RawMessage) (*ChangeAvailabilityRequest, error)
	BuildResetResponse(status string) (json.RawMessage, error)
	BuildUnlockConnectorResponse(status string) (json.RawMessage, error)
	BuildTriggerMessageResponse(status string) (json.RawMessage, error)
	BuildChangeAvailabilityResponse(status string) (json.RawMessage, error)
}

// LegacyRemoteTxProtocol is OCPP 1.6J-only: RemoteStartTransaction
// and RemoteStopTransaction. The request/response action names and
// payload shapes differ from 2.0.1, so we keep the two split rather
// than recombining.
type LegacyRemoteTxProtocol interface {
	ParseRemoteStartTransactionRequest(p json.RawMessage) (*RemoteStartTransactionRequest, error)
	ParseRemoteStopTransactionRequest(p json.RawMessage) (*RemoteStopTransactionRequest, error)
	BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error)
	BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error)
}

// RemoteTxProtocol is OCPP 2.0.1-only: RequestStartTransaction
// (carries idToken + remoteStartId) and RequestStopTransaction
// (carries a string transactionId).
type RemoteTxProtocol interface {
	ParseRequestStartTransactionRequest(p json.RawMessage) (*RequestStartTransactionRequest, error)
	ParseRequestStopTransactionRequest(p json.RawMessage) (*RequestStopTransactionRequest, error)
	BuildRequestStartTransactionResponse(status string) (json.RawMessage, error)
	BuildRequestStopTransactionResponse(status string) (json.RawMessage, error)
}

// --- Cross-version input structs -------------------------------------------

type BootNotificationInput struct {
	// 1.6J uses Vendor/Model as flat strings. 2.0.1 uses a
	// ChargingStation object (VendorName/Model). v16 reads
	// ChargePointVendor/ChargePointModel; v201 reads the same two
	// fields and wraps them in the chargingStation object on the
	// wire.
	ChargePointVendor string
	ChargePointModel  string
	// 2.0.1-only: the boot reason enum (PowerUp, RemoteReset, …).
	// 1.6J has no equivalent and ignores it.
	Reason string
	// 2.0.1-only: optional firmware version string carried in the
	// chargingStation object. 1.6J has no equivalent.
	FirmwareVersion string
}

type StatusNotificationInput struct {
	// 1.6J identifies a connector by its connectorId. 2.0.1 splits
	// this into an EVSE id and a connector id. v16 reads
	// ConnectorID; v201 reads EVSEID and ConnectorID (with
	// ConnectorID defaulting to 1 if zero on a single-connector
	// EVSE).
	ConnectorID int
	EVSEID      int
	Status      string
	// 1.6J-only. 2.0.1 has no errorCode in StatusNotification.
	ErrorCode string
	// 1.6J-only. 2.0.1 requires timestamp; the codec always writes
	// the current time.
	Timestamp time.Time
}

// AuthorizeInput carries the union of 1.6J and 2.0.1 authorize
// inputs. Exactly one of IDTag or IDToken is populated. v16 reads
// IDTag; v201 reads IDToken.
type AuthorizeInput struct {
	IDTag   string
	IDToken *IDToken
}

// IDToken is the OCPP 2.0.1 structured authorization token. Type is
// the spec enum (ISO14443, ISO15693, Central, KeyCode, Local,
// MacAddress, NoAuthorization, eMAID).
type IDToken struct {
	IDToken string `json:"idToken"`
	Type    string `json:"type"`
}

// MeterValuesInput is the cross-version meter sample carrier. 1.6J
// requires connectorId; 2.0.1 requires evseId. TransactionID is
// 1.6J-only (the 2.0.1 transaction context lives in the
// TransactionEvent message, not the meter sample).
type MeterValuesInput struct {
	ConnectorID   int
	EVSEID        int
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

// StartTransactionInput is 1.6J-only. Kept here so the legacy
// capability can carry it; v201 must not see it.
type StartTransactionInput struct {
	ConnectorID int
	IDTag       string
	MeterStart  int
	Timestamp   time.Time
}

// StopTransactionInput is 1.6J-only.
type StopTransactionInput struct {
	TransactionID int
	IDTag         string
	MeterStop     int
	Timestamp     time.Time
	Reason        string
}

// TransactionEventInput is 2.0.1-only. SeqNo is a per-transaction
// monotonically-increasing counter starting at 0; the simulator is
// the source of truth for it (the spec requires the CP to assign
// it; the CSMS uses it to detect missing events). TransactionID is
// the string GUID the CP chose when the transaction started.
type TransactionEventInput struct {
	EventType     string // "Started" | "Updated" | "Ended"
	Timestamp     time.Time
	TriggerReason string
	SeqNo         int
	Offline       bool

	TransactionID string // GUID, in transactionInfo.transactionId
	EVSEID        int
	ConnectorID   int

	// Optional. On "Started" this is the combined-authorization
	// path (the CP need not send a separate Authorize).
	IDToken *IDToken

	MeterValue []MeterValue

	// Optional, only meaningful for "Ended".
	StoppedReason string

	// Optional; 2.0.1 supports up to three phases.
	NumberOfPhasesUsed *int
	CableMaxCurrent    *int
}

// --- CSMS-initiated request/response structs (1.6J-shaped MVP) -------------

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

// --- 2.0.1-only CSMS-initiated request/response structs -------------------

type RequestStartTransactionRequest struct {
	// 2.0.1 RequestStartTransactionRequest.req.
	EVSEID         *int    `json:"evseId,omitempty"`
	IDToken        IDToken `json:"idToken"`
	RemoteStartID  int     `json:"remoteStartId"`
	ChargingProfile *json.RawMessage `json:"chargingProfile,omitempty"`
}

type RequestStopTransactionRequest struct {
	// 2.0.1 RequestStopTransactionRequest.req. transactionId is
	// the string GUID the CP chose when the transaction started.
	TransactionID string `json:"transactionId"`
}

// GetVariablesRequest and SetVariablesRequest are 2.0.1-only.
type GetVariableComponent struct {
	Name  string   `json:"name"`
	EVSE  *EVSE    `json:"evse,omitempty"`
	Instance *string `json:"instance,omitempty"`
}

type GetVariableDescriptor struct {
	Component GetVariableComponent `json:"component"`
	Variable  Variable            `json:"variable"`
}

type GetVariablesRequest struct {
	GetVariableDescriptor []GetVariableDescriptor `json:"getVariableData"`
}

type SetVariableData struct {
	Component    GetVariableComponent `json:"component"`
	Variable     Variable             `json:"variable"`
	AttributeValue interface{}        `json:"attributeValue,omitempty"`
}

type SetVariablesRequest struct {
	SetVariableData []SetVariableData `json:"setVariableData"`
}

type EVSE struct {
	ID         int  `json:"id"`
	ConnectorID *int `json:"connectorId,omitempty"`
}

type Variable struct {
	Name         string `json:"name"`
	Instance     *string `json:"instance,omitempty"`
}

type GetVariablesResultItem struct {
	AttributeStatus string      `json:"attributeStatus"`
	Component       GetVariableComponent `json:"component"`
	Variable        Variable   `json:"variable"`
	AttributeValue  interface{} `json:"attributeValue,omitempty"`
}

type SetVariablesResultItem struct {
	AttributeStatus string      `json:"attributeStatus"`
	Component       GetVariableComponent `json:"component"`
	Variable        Variable   `json:"variable"`
	AttributeType   *string    `json:"attributeType,omitempty"`
	AttributeValue  interface{} `json:"attributeValue,omitempty"`
}

// --- Factory ---------------------------------------------------------------

// Factory maps a version string (Version1_6 / Version2_0_1) to a
// registered implementation and surfaces the capabilities that
// version implements. Getters all return (T, bool): false means
// "this version does not implement this capability" — never panic,
// never a fake implementation. Registration discovers capabilities
// via type assertion so each version struct only implements what
// it actually supports.
type Factory struct {
	base             map[string]BaseProtocol
	legacyTransaction map[string]LegacyTransactionProtocol
	transactionEvent  map[string]TransactionEventProtocol
	legacyConfig     map[string]LegacyConfigProtocol
	variable         map[string]VariableProtocol
	remoteControl    map[string]RemoteControlProtocol
	legacyRemoteTx   map[string]LegacyRemoteTxProtocol
	remoteTx         map[string]RemoteTxProtocol
}

func NewFactory() *Factory {
	return &Factory{
		base:              make(map[string]BaseProtocol),
		legacyTransaction: make(map[string]LegacyTransactionProtocol),
		transactionEvent:  make(map[string]TransactionEventProtocol),
		legacyConfig:      make(map[string]LegacyConfigProtocol),
		variable:          make(map[string]VariableProtocol),
		remoteControl:     make(map[string]RemoteControlProtocol),
		legacyRemoteTx:    make(map[string]LegacyRemoteTxProtocol),
		remoteTx:          make(map[string]RemoteTxProtocol),
	}
}

// Register indexes p by p.Version() and discovers its capabilities
// via type assertion. Re-registering the same version overwrites
// the previous registration.
func (f *Factory) Register(p BaseProtocol) {
	v := p.Version()
	f.base[v] = p
	if c, ok := p.(LegacyTransactionProtocol); ok {
		f.legacyTransaction[v] = c
	}
	if c, ok := p.(TransactionEventProtocol); ok {
		f.transactionEvent[v] = c
	}
	if c, ok := p.(LegacyConfigProtocol); ok {
		f.legacyConfig[v] = c
	}
	if c, ok := p.(VariableProtocol); ok {
		f.variable[v] = c
	}
	if c, ok := p.(RemoteControlProtocol); ok {
		f.remoteControl[v] = c
	}
	if c, ok := p.(LegacyRemoteTxProtocol); ok {
		f.legacyRemoteTx[v] = c
	}
	if c, ok := p.(RemoteTxProtocol); ok {
		f.remoteTx[v] = c
	}
}

// Versions returns the sorted list of registered version strings.
// Useful for diagnostics and tests; not hot-path.
func (f *Factory) Versions() []string {
	out := make([]string, 0, len(f.base))
	for v := range f.base {
		out = append(out, v)
	}
	return out
}

// Base returns the registered BaseProtocol for version.
func (f *Factory) Base(version string) (BaseProtocol, bool) {
	p, ok := f.base[version]
	return p, ok
}

// LegacyTransaction returns the 1.6J transaction protocol for
// version, or (nil, false) if version does not implement it.
func (f *Factory) LegacyTransaction(version string) (LegacyTransactionProtocol, bool) {
	p, ok := f.legacyTransaction[version]
	return p, ok
}

// TransactionEvent returns the 2.0.1 transaction-event protocol
// for version, or (nil, false).
func (f *Factory) TransactionEvent(version string) (TransactionEventProtocol, bool) {
	p, ok := f.transactionEvent[version]
	return p, ok
}

// LegacyConfig returns the 1.6J Get/ChangeConfiguration protocol
// for version, or (nil, false).
func (f *Factory) LegacyConfig(version string) (LegacyConfigProtocol, bool) {
	p, ok := f.legacyConfig[version]
	return p, ok
}

// Variable returns the 2.0.1 Get/SetVariables protocol for version,
// or (nil, false).
func (f *Factory) Variable(version string) (VariableProtocol, bool) {
	p, ok := f.variable[version]
	return p, ok
}

// RemoteControl returns the Reset/UnlockConnector/TriggerMessage/
// ChangeAvailability protocol for version, or (nil, false).
func (f *Factory) RemoteControl(version string) (RemoteControlProtocol, bool) {
	p, ok := f.remoteControl[version]
	return p, ok
}

// LegacyRemoteTx returns the 1.6J RemoteStart/StopTransaction
// protocol for version, or (nil, false).
func (f *Factory) LegacyRemoteTx(version string) (LegacyRemoteTxProtocol, bool) {
	p, ok := f.legacyRemoteTx[version]
	return p, ok
}

// RemoteTx returns the 2.0.1 RequestStart/StopTransaction
// protocol for version, or (nil, false).
func (f *Factory) RemoteTx(version string) (RemoteTxProtocol, bool) {
	p, ok := f.remoteTx[version]
	return p, ok
}
