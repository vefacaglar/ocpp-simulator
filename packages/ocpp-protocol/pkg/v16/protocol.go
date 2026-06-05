// Package v16 implements the OCPP 1.6J protocol. It is the only place that
// knows 1.6J-specific wire field names, casing, enums, and the integer
// transactionId model. It produces raw [2, uid, action, payload] CALL frames
// via codec and accepts matching CALL/CALLRESULT/CALLERROR frames back.
package v16

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

type Protocol struct {
	codec *codec.Codec
}

func NewProtocol() *Protocol {
	return &Protocol{codec: codec.New()}
}

func (p *Protocol) Version() string {
	return "1.6J"
}

func (p *Protocol) BuildBootNotification(ctx context.Context, input protocol.BootNotificationInput) (message.Message, error) {
	payload := struct {
		ChargePointVendor string `json:"chargePointVendor"`
		ChargePointModel  string `json:"chargePointModel"`
	}{
		ChargePointVendor: input.ChargePointVendor,
		ChargePointModel:  input.ChargePointModel,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "BootNotification", payload)
}

func (p *Protocol) BuildHeartbeat(ctx context.Context) (message.Message, error) {
	return p.codec.BuildCall(message.GenerateUniqueID(), "Heartbeat", struct{}{})
}

func (p *Protocol) BuildStatusNotification(ctx context.Context, input protocol.StatusNotificationInput) (message.Message, error) {
	payload := struct {
		ConnectorID int    `json:"connectorId"`
		ErrorCode   string `json:"errorCode"`
		Status      string `json:"status"`
		Timestamp   string `json:"timestamp,omitempty"`
	}{
		ConnectorID: input.ConnectorID,
		ErrorCode:   input.ErrorCode,
		Status:      input.Status,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "StatusNotification", payload)
}

func (p *Protocol) BuildAuthorize(ctx context.Context, input protocol.AuthorizeInput) (message.Message, error) {
	payload := struct {
		IDTag string `json:"idTag"`
	}{
		IDTag: input.IDTag,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "Authorize", payload)
}

func (p *Protocol) BuildStartTransaction(ctx context.Context, input protocol.StartTransactionInput) (message.Message, error) {
	payload := struct {
		ConnectorID int    `json:"connectorId"`
		IDTag       string `json:"idTag"`
		MeterStart  int    `json:"meterStart"`
		Timestamp   string `json:"timestamp"`
	}{
		ConnectorID: input.ConnectorID,
		IDTag:       input.IDTag,
		MeterStart:  input.MeterStart,
		Timestamp:   input.Timestamp.Format(time.RFC3339),
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "StartTransaction", payload)
}

func (p *Protocol) BuildMeterValues(ctx context.Context, input protocol.MeterValuesInput) (message.Message, error) {
	type sv struct {
		Value     string `json:"value"`
		Measurand string `json:"measurand,omitempty"`
		Unit      string `json:"unit,omitempty"`
	}
	type mv struct {
		Timestamp    string `json:"timestamp"`
		SampledValue []sv   `json:"sampledValue"`
	}

	mvs := make([]mv, len(input.MeterValues))
	for i, m := range input.MeterValues {
		svs := make([]sv, len(m.SampledValue))
		for j, s := range m.SampledValue {
			svs[j] = sv{Value: s.Value, Measurand: s.Measurand, Unit: s.Unit}
		}
		mvs[i] = mv{
			Timestamp:    m.Timestamp.Format(time.RFC3339),
			SampledValue: svs,
		}
	}

	payload := struct {
		ConnectorID   int   `json:"connectorId"`
		TransactionID *int  `json:"transactionId,omitempty"`
		MeterValue    []mv  `json:"meterValue"`
	}{
		ConnectorID:   input.ConnectorID,
		TransactionID: input.TransactionID,
		MeterValue:    mvs,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "MeterValues", payload)
}

func (p *Protocol) BuildStopTransaction(ctx context.Context, input protocol.StopTransactionInput) (message.Message, error) {
	payload := struct {
		TransactionID int    `json:"transactionId"`
		IDTag         string `json:"idTag,omitempty"`
		MeterStop     int    `json:"meterStop"`
		Timestamp     string `json:"timestamp"`
		Reason        string `json:"reason,omitempty"`
	}{
		TransactionID: input.TransactionID,
		IDTag:         input.IDTag,
		MeterStop:     input.MeterStop,
		Timestamp:     input.Timestamp.Format(time.RFC3339),
		Reason:        input.Reason,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "StopTransaction", payload)
}

type BootNotificationResponse struct {
	Status      string `json:"status"`
	CurrentTime string `json:"currentTime"`
	Interval    int    `json:"interval"`
}

type HeartbeatResponse struct {
	CurrentTime string `json:"currentTime"`
}

type AuthorizeResponse struct {
	IDTagInfo struct {
		Status string `json:"status"`
	} `json:"idTagInfo"`
}

type StartTransactionResponse struct {
	TransactionID int `json:"transactionId"`
	IDTagInfo     struct {
		Status string `json:"status"`
	} `json:"idTagInfo"`
}

type StopTransactionResponse struct {
	IDTagInfo *struct {
		Status string `json:"status"`
	} `json:"idTagInfo,omitempty"`
}

func ParseBootNotificationResponse(payload json.RawMessage) (*BootNotificationResponse, error) {
	var resp BootNotificationResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse BootNotification.conf: %w", err)
	}
	return &resp, nil
}

func ParseHeartbeatResponse(payload json.RawMessage) (*HeartbeatResponse, error) {
	var resp HeartbeatResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse Heartbeat.conf: %w", err)
	}
	return &resp, nil
}

func ParseStartTransactionResponse(payload json.RawMessage) (*StartTransactionResponse, error) {
	var resp StartTransactionResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse StartTransaction.conf: %w", err)
	}
	return &resp, nil
}

func ParseStopTransactionResponse(payload json.RawMessage) (*StopTransactionResponse, error) {
	var resp StopTransactionResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("parse StopTransaction.conf: %w", err)
	}
	return &resp, nil
}

// CSMS-initiated request parsing

func (p *Protocol) ParseRemoteStartTransactionRequest(payload json.RawMessage) (*protocol.RemoteStartTransactionRequest, error) {
	var req protocol.RemoteStartTransactionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse RemoteStartTransaction.req: %w", err)
	}
	if req.IDTag == "" {
		return nil, fmt.Errorf("RemoteStartTransaction: idTag is required")
	}
	return &req, nil
}

func (p *Protocol) ParseRemoteStopTransactionRequest(payload json.RawMessage) (*protocol.RemoteStopTransactionRequest, error) {
	var req protocol.RemoteStopTransactionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse RemoteStopTransaction.req: %w", err)
	}
	return &req, nil
}

func (p *Protocol) ParseResetRequest(payload json.RawMessage) (*protocol.ResetRequest, error) {
	var req protocol.ResetRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse Reset.req: %w", err)
	}
	if req.Type != "Hard" && req.Type != "Soft" {
		return nil, fmt.Errorf("Reset: invalid type %q", req.Type)
	}
	return &req, nil
}

func (p *Protocol) ParseUnlockConnectorRequest(payload json.RawMessage) (*protocol.UnlockConnectorRequest, error) {
	var req protocol.UnlockConnectorRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse UnlockConnector.req: %w", err)
	}
	if req.ConnectorID <= 0 {
		return nil, fmt.Errorf("UnlockConnector: connectorId must be > 0")
	}
	return &req, nil
}

func (p *Protocol) ParseChangeConfigurationRequest(payload json.RawMessage) (*protocol.ChangeConfigurationRequest, error) {
	var req protocol.ChangeConfigurationRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse ChangeConfiguration.req: %w", err)
	}
	if req.Key == "" {
		return nil, fmt.Errorf("ChangeConfiguration: key is required")
	}
	return &req, nil
}

func (p *Protocol) ParseGetConfigurationRequest(payload json.RawMessage) (*protocol.GetConfigurationRequest, error) {
	var req protocol.GetConfigurationRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse GetConfiguration.req: %w", err)
	}
	return &req, nil
}

func (p *Protocol) ParseTriggerMessageRequest(payload json.RawMessage) (*protocol.TriggerMessageRequest, error) {
	var req protocol.TriggerMessageRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse TriggerMessage.req: %w", err)
	}
	if req.RequestedMessage == "" {
		return nil, fmt.Errorf("TriggerMessage: requestedMessage is required")
	}
	return &req, nil
}

func (p *Protocol) ParseChangeAvailabilityRequest(payload json.RawMessage) (*protocol.ChangeAvailabilityRequest, error) {
	var req protocol.ChangeAvailabilityRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse ChangeAvailability.req: %w", err)
	}
	if req.Type != "Operative" && req.Type != "Inoperative" {
		return nil, fmt.Errorf("ChangeAvailability: invalid type %q", req.Type)
	}
	return &req, nil
}

// CSMS-initiated response builders

func (p *Protocol) BuildRemoteStartTransactionResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildRemoteStopTransactionResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildResetResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildUnlockConnectorResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildChangeConfigurationResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildGetConfigurationResponse(configKeys []protocol.ConfigurationKey, unknownKeys []string) (json.RawMessage, error) {
	resp := struct {
		ConfigurationKey []protocol.ConfigurationKey `json:"configurationKey,omitempty"`
		UnknownKey       []string                   `json:"unknownKey,omitempty"`
	}{
		ConfigurationKey: configKeys,
		UnknownKey:       unknownKeys,
	}
	return json.Marshal(resp)
}

func (p *Protocol) BuildTriggerMessageResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildChangeAvailabilityResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}
