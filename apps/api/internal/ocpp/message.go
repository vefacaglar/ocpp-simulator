package ocpp

import "encoding/json"

type MessageTypeID int

const (
	CALL       MessageTypeID = 2
	CALLRESULT MessageTypeID = 3
	CALLERROR  MessageTypeID = 4
)

type Message struct {
	MessageTypeID     MessageTypeID
	UniqueID          string
	Action            string
	Payload           json.RawMessage
	ErrorCode         string
	ErrorDescription  string
	ErrorDetails      json.RawMessage
}

type ErrorCode string

	const (
	ErrorCodeNotImplemented      ErrorCode = "NotImplemented"
	ErrorCodeNotSupported        ErrorCode = "NotSupported"
	ErrorCodeInternalError       ErrorCode = "InternalError"
	ErrorCodeProtocolError       ErrorCode = "ProtocolError"
	ErrorCodeSecurityError       ErrorCode = "SecurityError"
	ErrorCodeFormationViolation  ErrorCode = "FormationViolation"
	ErrorCodePropertyConstraintViolation ErrorCode = "PropertyConstraintViolation"
	ErrorCodeOccurrenceConstraintViolation ErrorCode = "OccurrenceConstraintViolation"
	ErrorCodeTypeConstraintViolation      ErrorCode = "TypeConstraintViolation"
	ErrorCodeGenericError        ErrorCode = "GenericError"
	)
