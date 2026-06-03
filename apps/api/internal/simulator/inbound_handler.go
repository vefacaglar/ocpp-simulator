package simulator

import (
	"encoding/json"
	"fmt"

	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

// InboundCallHandler dispatches CSMS-initiated CALL messages to the appropriate runtime method.
type InboundCallHandler struct {
	runtime *Runtime
	factory *protocol.Factory
}

func NewInboundCallHandler(runtime *Runtime, factory *protocol.Factory) *InboundCallHandler {
	return &InboundCallHandler{
		runtime: runtime,
		factory: factory,
	}
}

// Handle processes an inbound CSMS-initiated CALL and returns the CALLRESULT payload.
// Returns an error if the action is not supported or processing fails.
func (h *InboundCallHandler) Handle(cpID string, msg message.Message) (json.RawMessage, error) {
	// Get the protocol for this charge point to parse/build payloads
	cp, ok := h.runtime.GetChargePoint(cpID)
	if !ok {
		return nil, fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.RLock()
	version := cp.Config.OCPPVersion
	cp.Mu.RUnlock()

	protocol, ok := h.factory.Create(version)
	if !ok {
		return nil, fmt.Errorf("unsupported OCPP version: %s", version)
	}

	switch msg.Action {
	case "RemoteStartTransaction":
		req, err := protocol.ParseRemoteStartTransactionRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleRemoteStartTransaction(cpID, req)
		if err != nil {
			return nil, err
		}
		return protocol.BuildRemoteStartTransactionResponse(status)

	case "RemoteStopTransaction":
		req, err := protocol.ParseRemoteStopTransactionRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleRemoteStopTransaction(cpID, req)
		if err != nil {
			return nil, err
		}
		return protocol.BuildRemoteStopTransactionResponse(status)

	case "Reset":
		req, err := protocol.ParseResetRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleReset(cpID, req)
		if err != nil {
			return nil, err
		}
		return protocol.BuildResetResponse(status)

	case "UnlockConnector":
		req, err := protocol.ParseUnlockConnectorRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleUnlockConnector(cpID, req.ConnectorID)
		if err != nil {
			return nil, err
		}
		return protocol.BuildUnlockConnectorResponse(status)

	case "ChangeConfiguration":
		req, err := protocol.ParseChangeConfigurationRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleChangeConfiguration(cpID, req.Key, req.Value)
		if err != nil {
			return nil, err
		}
		return protocol.BuildChangeConfigurationResponse(status)

	case "GetConfiguration":
		req, err := protocol.ParseGetConfigurationRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		configKeys, unknownKeys, err := h.runtime.HandleGetConfiguration(cpID, req.Key)
		if err != nil {
			return nil, err
		}
		return protocol.BuildGetConfigurationResponse(configKeys, unknownKeys)

	case "TriggerMessage":
		req, err := protocol.ParseTriggerMessageRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleTriggerMessage(cpID, req.RequestedMessage, req.ConnectorID)
		if err != nil {
			return nil, err
		}
		return protocol.BuildTriggerMessageResponse(status)

	case "ChangeAvailability":
		req, err := protocol.ParseChangeAvailabilityRequest(msg.Payload)
		if err != nil {
			return nil, err
		}
		status, err := h.runtime.HandleChangeAvailability(cpID, req.ConnectorID, req.Type)
		if err != nil {
			return nil, err
		}
		return protocol.BuildChangeAvailabilityResponse(status)

	default:
		return nil, fmt.Errorf("unsupported action: %s", msg.Action)
	}
}
