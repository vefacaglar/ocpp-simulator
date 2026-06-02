package ocpp

import (
	"context"
	"time"
)

type Protocol interface {
	Version() string

	BuildBootNotification(ctx context.Context, input BootNotificationInput) (Message, error)
	BuildHeartbeat(ctx context.Context) (Message, error)
	BuildStatusNotification(ctx context.Context, input StatusNotificationInput) (Message, error)
	BuildAuthorize(ctx context.Context, input AuthorizeInput) (Message, error)
	BuildStartTransaction(ctx context.Context, input StartTransactionInput) (Message, error)
	BuildMeterValues(ctx context.Context, input MeterValuesInput) (Message, error)
	BuildStopTransaction(ctx context.Context, input StopTransactionInput) (Message, error)
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
	ConnectorID  int
	TransactionID *int
	MeterValues  []MeterValue
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
