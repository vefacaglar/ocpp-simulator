// Package codec encodes and decodes raw OCPP-J frames. It is intentionally
// version-agnostic: it only knows the wire envelope, not the action semantics.
// The version-specific protocol package (pkg/v16) uses the codec to assemble
// and parse payloads.
package codec

import (
	"encoding/json"
	"fmt"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

type Codec struct{}

func New() *Codec {
	return &Codec{}
}

func (c *Codec) Encode(msg message.Message) ([]byte, error) {
	switch msg.MessageTypeID {
	case message.CALL:
		if msg.Action == "" {
			return nil, fmt.Errorf("CALL requires action")
		}
		frame := [4]interface{}{message.CALL, msg.UniqueID, msg.Action, json.RawMessage(msg.Payload)}
		return json.Marshal(frame)
	case message.CALLRESULT:
		frame := [3]interface{}{message.CALLRESULT, msg.UniqueID, json.RawMessage(msg.Payload)}
		return json.Marshal(frame)
	case message.CALLERROR:
		frame := [5]interface{}{message.CALLERROR, msg.UniqueID, msg.ErrorCode, msg.ErrorDescription, json.RawMessage(msg.ErrorDetails)}
		return json.Marshal(frame)
	default:
		return nil, fmt.Errorf("unknown message type: %d", msg.MessageTypeID)
	}
}

func (c *Codec) Decode(raw []byte) (message.Message, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return message.Message{}, fmt.Errorf("invalid JSON array: %w", err)
	}

	if len(arr) < 2 {
		return message.Message{}, fmt.Errorf("frame too short: %d elements", len(arr))
	}

	var typeID message.MessageTypeID
	if err := json.Unmarshal(arr[0], &typeID); err != nil {
		return message.Message{}, fmt.Errorf("invalid message type id: %w", err)
	}

	var uniqueID string
	if err := json.Unmarshal(arr[1], &uniqueID); err != nil {
		return message.Message{}, fmt.Errorf("invalid unique id: %w", err)
	}

	msg := message.Message{
		MessageTypeID: typeID,
		UniqueID:      uniqueID,
	}

	switch typeID {
	case message.CALL:
		if len(arr) != 4 {
			return message.Message{}, fmt.Errorf("CALL frame must have 4 elements, got %d", len(arr))
		}
		if err := json.Unmarshal(arr[2], &msg.Action); err != nil {
			return message.Message{}, fmt.Errorf("invalid action: %w", err)
		}
		msg.Payload = arr[3]

	case message.CALLRESULT:
		if len(arr) != 3 {
			return message.Message{}, fmt.Errorf("CALLRESULT frame must have 3 elements, got %d", len(arr))
		}
		msg.Payload = arr[2]

	case message.CALLERROR:
		if len(arr) != 5 {
			return message.Message{}, fmt.Errorf("CALLERROR frame must have 5 elements, got %d", len(arr))
		}
		if err := json.Unmarshal(arr[2], &msg.ErrorCode); err != nil {
			return message.Message{}, fmt.Errorf("invalid error code: %w", err)
		}
		if err := json.Unmarshal(arr[3], &msg.ErrorDescription); err != nil {
			return message.Message{}, fmt.Errorf("invalid error description: %w", err)
		}
		msg.ErrorDetails = arr[4]

	default:
		return message.Message{}, fmt.Errorf("unknown message type id: %d", typeID)
	}

	return msg, nil
}

func (c *Codec) BuildCall(uniqueID, action string, payload interface{}) (message.Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return message.Message{}, fmt.Errorf("marshal payload: %w", err)
	}
	return message.Message{
		MessageTypeID: message.CALL,
		UniqueID:      uniqueID,
		Action:        action,
		Payload:       data,
	}, nil
}

func (c *Codec) BuildResult(uniqueID string, payload interface{}) (message.Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return message.Message{}, fmt.Errorf("marshal payload: %w", err)
	}
	return message.Message{
		MessageTypeID: message.CALLRESULT,
		UniqueID:      uniqueID,
		Payload:       data,
	}, nil
}

func (c *Codec) BuildError(uniqueID string, code message.ErrorCode, description string, details interface{}) (message.Message, error) {
	var data json.RawMessage
	if details != nil {
		d, err := json.Marshal(details)
		if err != nil {
			return message.Message{}, fmt.Errorf("marshal details: %w", err)
		}
		data = d
	} else {
		data = json.RawMessage(`{}`)
	}
	return message.Message{
		MessageTypeID:    message.CALLERROR,
		UniqueID:         uniqueID,
		ErrorCode:        string(code),
		ErrorDescription: description,
		ErrorDetails:     data,
	}, nil
}
