package common

import "time"

type ChargePointStatus string

const (
	StatusDisconnected ChargePointStatus = "disconnected"
	StatusConnecting   ChargePointStatus = "connecting"
	StatusConnected    ChargePointStatus = "connected"
)

type ConnectorStatus string

const (
	ConnectorAvailable     ConnectorStatus = "Available"
	ConnectorPreparing     ConnectorStatus = "Preparing"
	ConnectorCharging      ConnectorStatus = "Charging"
	ConnectorSuspendedEV   ConnectorStatus = "SuspendedEV"
	ConnectorSuspendedEVSE ConnectorStatus = "SuspendedEVSE"
	ConnectorFinishing     ConnectorStatus = "Finishing"
	ConnectorReserved      ConnectorStatus = "Reserved"
	ConnectorUnavailable   ConnectorStatus = "Unavailable"
	ConnectorFaulted       ConnectorStatus = "Faulted"
)

type ChargePoint struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	OCPPVersion      string            `json:"ocppVersion"`
	CentralSystemURL string            `json:"centralSystemUrl"`
	ConnectorCount   int               `json:"connectorCount"`
	AutoConnect      bool              `json:"autoConnect"`
	Status           ChargePointStatus `json:"status"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

type Connector struct {
	ID             int            `json:"id"`
	ChargePointID  string         `json:"chargePointId"`
	EVSEID         int            `json:"evseId"`
	ConnectorNumber int           `json:"connectorNumber"`
	Status         ConnectorStatus `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}
