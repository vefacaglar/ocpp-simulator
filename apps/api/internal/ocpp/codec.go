package ocpp

import (
	"encoding/json"
	"fmt"
)

type Codec struct{}

func NewCodec() *Codec {
	return &Codec{}
}

func (c *Codec) Encode(msg Message) ([]byte, error) {
	switch msg.MessageTypeID {
	case CALL:
		if msg.Action == "" {
			return nil, fmt.Errorf("CALL requires action")
		}
		frame := [4]interface{}{CALL, msg.UniqueID, msg.Action, json.RawMessage(msg.Payload)}
		return json.Marshal(frame)
	case CALLRESULT:
		frame := [3]interface{}{CALLRESULT, msg.UniqueID, json.RawMessage(msg.Payload)}
		return json.Marshal(frame)
	case CALLERROR:
		frame := [5]interface{}{CALLERROR, msg.UniqueID, msg.ErrorCode, msg.ErrorDescription, json.RawMessage(msg.ErrorDetails)}
		return json.Marshal(frame)
	default:
		return nil, fmt.Errorf("unknown message type: %d", msg.MessageTypeID)
	}
}

func (c *Codec) Decode(raw []byte) (Message, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return Message{}, fmt.Errorf("invalid JSON array: %w", err)
	}

	if len(arr) < 2 {
		return Message{}, fmt.Errorf("frame too short: %d elements", len(arr))
	}

	var typeID MessageTypeID
	if err := json.Unmarshal(arr[0], &typeID); err != nil {
		return Message{}, fmt.Errorf("invalid message type id: %w", err)
	}

	var uniqueID string
	if err := json.Unmarshal(arr[1], &uniqueID); err != nil {
		return Message{}, fmt.Errorf("invalid unique id: %w", err)
	}

	msg := Message{
		MessageTypeID: typeID,
		UniqueID:      uniqueID,
	}

	switch typeID {
	case CALL:
		if len(arr) != 4 {
			return Message{}, fmt.Errorf("CALL frame must have 4 elements, got %d", len(arr))
		}
		if err := json.Unmarshal(arr[2], &msg.Action); err != nil {
			return Message{}, fmt.Errorf("invalid action: %w", err)
		}
		msg.Payload = arr[3]

	case CALLRESULT:
		if len(arr) != 3 {
			return Message{}, fmt.Errorf("CALLRESULT frame must have 3 elements, got %d", len(arr))
		}
		msg.Payload = arr[2]

	case CALLERROR:
		if len(arr) != 5 {
			return Message{}, fmt.Errorf("CALLERROR frame must have 5 elements, got %d", len(arr))
		}
		if err := json.Unmarshal(arr[2], &msg.ErrorCode); err != nil {
			return Message{}, fmt.Errorf("invalid error code: %w", err)
		}
		if err := json.Unmarshal(arr[3], &msg.ErrorDescription); err != nil {
			return Message{}, fmt.Errorf("invalid error description: %w", err)
		}
		msg.ErrorDetails = arr[4]

	default:
		return Message{}, fmt.Errorf("unknown message type id: %d", typeID)
	}

	return msg, nil
}

func (c *Codec) BuildCall(uniqueID, action string, payload interface{}) (Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Message{
		MessageTypeID: CALL,
		UniqueID:      uniqueID,
		Action:        action,
		Payload:       data,
	}, nil
}

func (c *Codec) BuildResult(uniqueID string, payload interface{}) (Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Message{
		MessageTypeID: CALLRESULT,
		UniqueID:      uniqueID,
		Payload:       data,
	}, nil
}

func (c *Codec) BuildError(uniqueID string, code ErrorCode, description string, details interface{}) (Message, error) {
	var data json.RawMessage
	if details != nil {
		d, err := json.Marshal(details)
		if err != nil {
			return Message{}, fmt.Errorf("marshal details: %w", err)
		}
		data = d
	} else {
		data = json.RawMessage(`{}`)
	}
	return Message{
		MessageTypeID:    CALLERROR,
		UniqueID:         uniqueID,
		ErrorCode:        string(code),
		ErrorDescription: description,
		ErrorDetails:     data,
	}, nil
}
