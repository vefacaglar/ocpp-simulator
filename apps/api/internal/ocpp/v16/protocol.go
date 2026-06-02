package v16

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
)

type Protocol struct {
	codec *ocpp.Codec
}

func NewProtocol() *Protocol {
	return &Protocol{codec: ocpp.NewCodec()}
}

func (p *Protocol) Version() string {
	return "1.6J"
}

func (p *Protocol) BuildBootNotification(ctx context.Context, input ocpp.BootNotificationInput) (ocpp.Message, error) {
	payload := struct {
		ChargePointVendor string `json:"chargePointVendor"`
		ChargePointModel  string `json:"chargePointModel"`
	}{
		ChargePointVendor: input.ChargePointVendor,
		ChargePointModel:  input.ChargePointModel,
	}
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "BootNotification", payload)
}

func (p *Protocol) BuildHeartbeat(ctx context.Context) (ocpp.Message, error) {
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "Heartbeat", struct{}{})
}

func (p *Protocol) BuildStatusNotification(ctx context.Context, input ocpp.StatusNotificationInput) (ocpp.Message, error) {
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
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "StatusNotification", payload)
}

func (p *Protocol) BuildAuthorize(ctx context.Context, input ocpp.AuthorizeInput) (ocpp.Message, error) {
	payload := struct {
		IDTag string `json:"idTag"`
	}{
		IDTag: input.IDTag,
	}
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "Authorize", payload)
}

func (p *Protocol) BuildStartTransaction(ctx context.Context, input ocpp.StartTransactionInput) (ocpp.Message, error) {
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
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "StartTransaction", payload)
}

func (p *Protocol) BuildMeterValues(ctx context.Context, input ocpp.MeterValuesInput) (ocpp.Message, error) {
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
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "MeterValues", payload)
}

func (p *Protocol) BuildStopTransaction(ctx context.Context, input ocpp.StopTransactionInput) (ocpp.Message, error) {
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
	return p.codec.BuildCall(ocpp.GenerateUniqueID(), "StopTransaction", payload)
}

type BootNotificationResponse struct {
	Status     string `json:"status"`
	CurrentTime string `json:"currentTime"`
	Interval   int    `json:"interval"`
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
