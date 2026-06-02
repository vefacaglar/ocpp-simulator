package ocppschemas

import "fmt"

// Version represents an OCPP protocol version.
type Version string

const (
	V16  Version = "1.6"
	V201 Version = "2.0.1"
)

// Direction indicates whether a message is a request or response.
type Direction string

const (
	Request  Direction = "Request"
	Response Direction = "Response"
)

// Validate checks a payload against the official OCPP schema for the given
// version, action, and direction (request vs. response).
// This is a stub; schema files will be added in T12.
func Validate(version Version, action string, dir Direction, payload []byte) error {
	return fmt.Errorf("ocpp-schemas: validator not yet implemented (version=%s action=%s dir=%s)", version, action, dir)
}
