package gateway

// subprotocolToVersion maps the WebSocket subprotocol token a
// charge point selected at the OCPP-J handshake to the canonical
// version string used as the MQTT topic segment and the
// Factory key. The token is the only reliable source of the
// negotiated OCPP version (multi-version plan §3) and must agree
// with the subprotocols advertised by the upgrader.
//
// Returns (version, true) on a known token; ("", false) otherwise.
// The caller (WSHandler) MUST reject connections that return
// false — sending a CALL frame on a topic the schema validator
// would reject silently is the failure mode this check prevents.
func subprotocolToVersion(token string) (string, bool) {
	switch token {
	case "ocpp1.6":
		return "1.6J", true
	case "ocpp2.0.1":
		return "2.0.1", true
	default:
		return "", false
	}
}
