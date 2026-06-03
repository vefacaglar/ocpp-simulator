// Package message defines the OCPP-J message envelope types and the standard
// error code set. These types are the only place that knows the raw wire
// framing: [MessageTypeId, UniqueId, Action, Payload] for CALL,
// [MessageTypeId, UniqueId, Payload] for CALLRESULT, and
// [MessageTypeId, UniqueId, ErrorCode, ErrorDescription, ErrorDetails] for
// CALLERROR. See plan.md §2b for the strict-compliance rule.
package message

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type MessageTypeID int

const (
	CALL       MessageTypeID = 2
	CALLRESULT MessageTypeID = 3
	CALLERROR  MessageTypeID = 4
)

type Message struct {
	MessageTypeID    MessageTypeID
	UniqueID         string
	Action           string
	Payload          json.RawMessage
	ErrorCode        string
	ErrorDescription string
	ErrorDetails     json.RawMessage
}

type ErrorCode string

const (
	ErrorCodeNotImplemented                ErrorCode = "NotImplemented"
	ErrorCodeNotSupported                  ErrorCode = "NotSupported"
	ErrorCodeInternalError                 ErrorCode = "InternalError"
	ErrorCodeProtocolError                 ErrorCode = "ProtocolError"
	ErrorCodeSecurityError                 ErrorCode = "SecurityError"
	ErrorCodeFormationViolation            ErrorCode = "FormationViolation"
	ErrorCodePropertyConstraintViolation   ErrorCode = "PropertyConstraintViolation"
	ErrorCodeOccurrenceConstraintViolation ErrorCode = "OccurrenceConstraintViolation"
	ErrorCodeTypeConstraintViolation       ErrorCode = "TypeConstraintViolation"
	ErrorCodeGenericError                  ErrorCode = "GenericError"
)

func GenerateUniqueID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", len(b))
	}
	return hex.EncodeToString(b)
}
