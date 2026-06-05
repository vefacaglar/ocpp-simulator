package gateway

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// WSHandler returns an http.Handler that upgrades requests on
// /ws/{chargePointID} to WebSocket connections. Each connection binds
// to its own MQTT subscription through the Gateway. The handler is
// dumb: it copies bytes in both directions without ever inspecting
// them.
func (g *Gateway) WSHandler() http.Handler {
	upgrader := websocket.Upgrader{
		// Permissive check-origin: this gateway is local-dev only.
		// Production deployments should restrict this.
		CheckOrigin: func(r *http.Request) bool { return true },
		// Advertise both OCPP subprotocols. The actual subprotocol
		// selected by the client is echoed in the response header
		// and captured by conn.Subprotocol() after Upgrade, which
		// is the OCPP spec's expected handshake and the version
		// discovery point (multi-version plan §3).
		Subprotocols: []string{"ocpp1.6", "ocpp2.0.1"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cpID := extractChargePointID(r.URL.Path)
		if cpID == "" {
			http.Error(w, "missing charge point id", http.StatusBadRequest)
			return
		}

		// Negotiate the OCPP subprotocol from the
		// Sec-WebSocket-Protocol header. The gorilla
		// Upgrader.Upgrade will select the first advertised
		// token it finds in the client's offered list, so we
		// read it back via the response-header check BEFORE
		// Upgrade (offered list) AND post-Upgrade via
		// conn.Subprotocol() (the canonical version the client
		// selected). Doing the check pre-upgrade lets us
		// reject the request with a clean 400 instead of
		// upgrading then closing.
		offered := offeredSubprotocols(r)
		var version string
		var ok bool
		for _, tok := range offered {
			if version, ok = subprotocolToVersion(tok); ok {
				break
			}
		}
		if !ok {
			log.Printf("[ocpp-gateway] rejecting %s: no OCPP subprotocol offered (got %v)", cpID, offered)
			http.Error(w, "OCPP subprotocol required: ocpp1.6 or ocpp2.0.1", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ocpp-gateway] upgrade failed for %s: %v", cpID, err)
			return
		}
		// Defense-in-depth: confirm the negotiated subprotocol
		// matches the one the upgrader actually selected. The
		// upgrader picks the first match; if a client offered
		// both, we want the first we accepted.
		if got := conn.Subprotocol(); got != "" {
			if v, match := subprotocolToVersion(got); match {
				version = v
			} else {
				log.Printf("[ocpp-gateway] closing %s: negotiated subprotocol %q is not OCPP", cpID, got)
				_ = conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseProtocolError, "OCPP subprotocol required"),
					time.Now().Add(time.Second),
				)
				_ = conn.Close()
				return
			}
		}
		// Set a write deadline to detect half-open peers.
		_ = conn.SetWriteDeadline(time.Now().Add(60 * time.Second))

		cp, err := g.Connect(version, cpID)
		if err != nil {
			log.Printf("[ocpp-gateway] connect failed for %s: %v", cpID, err)
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()),
				time.Now().Add(time.Second),
			)
			_ = conn.Close()
			return
		}
		defer g.Disconnect(cpID)

		go g.pumpOutbound(cp, conn)
		g.pumpInbound(r.Context(), cp, conn)
	})
}

// offeredSubprotocols returns the list of WebSocket subprotocol
// tokens the client offered in the Sec-WebSocket-Protocol header,
// in order. An absent or empty header returns an empty slice (the
// caller then sees ok=false and rejects the connection).
func offeredSubprotocols(r *http.Request) []string {
	hdr := r.Header.Get("Sec-WebSocket-Protocol")
	if hdr == "" {
		return nil
	}
	parts := strings.Split(hdr, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// extractChargePointID parses /ws/{id} and returns the id, or "" if
// the path doesn't match. The path is sanitized for MQTT topic use
// (no wildcards, no slashes inside the id) so a malicious or buggy
// client can't escape into another tenant's topic.
func extractChargePointID(path string) string {
	const prefix = "/ws/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimPrefix(path, prefix)
	// Reject empty, and reject any '/' or '+' or '#' that would
	// change MQTT topic semantics.
	if id == "" || strings.ContainsAny(id, "/+#") {
		return ""
	}
	return id
}

// pumpInbound reads text frames from the CP WebSocket and publishes
// them unchanged to the /in topic. Exits on read error, on context
// cancellation, or when the registry marks the connection closed.
func (g *Gateway) pumpInbound(ctx context.Context, cp *Connection, conn *websocket.Conn) {
	defer conn.Close()
	for {
		// Use TextMessage explicitly. OCPP-J frames are always JSON
		// text. Binary frames are rejected by the read message type
		// matcher below.
		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ocpp-gateway] read error for %s: %v", cp.ChargePointID, err)
			return
		}
		log.Printf("[ocpp-gateway] RECEIVED frame from %s: %s", cp.ChargePointID, string(payload))
		if msgType != websocket.TextMessage {
			// OCPP-J is text only; close on anything else.
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "OCPP-J frames must be text"),
				time.Now().Add(time.Second),
			)
			return
		}
		if err := g.PublishInbound(cp.Version, cp.ChargePointID, payload); err != nil {
			log.Printf("[ocpp-gateway] publish inbound for %s failed: %v", cp.ChargePointID, err)
			return
		}
		log.Printf("[ocpp-gateway] PUBLISHED to MQTT for %s", cp.ChargePointID)
		select {
		case <-ctx.Done():
			return
		case <-cp.Closed():
			return
		default:
		}
	}
}

// pumpOutbound writes frames received on the CP's /out topic to the
// CP's WebSocket. Exits when the connection is closed by the registry
// or when the WS write fails.
func (g *Gateway) pumpOutbound(cp *Connection, conn *websocket.Conn) {
	defer conn.Close()
	for {
		select {
		case <-cp.Closed():
			return
		case payload, ok := <-cp.Outbound:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(60 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		}
	}
}
