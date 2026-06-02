package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ocpp/{chargePointId}", handleOCPP)

	addr := ":" + port
	log.Printf("Mock OCPP CSMS listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func handleOCPP(w http.ResponseWriter, r *http.Request) {
	chargePointId := r.PathValue("chargePointId")
	if chargePointId == "" {
		http.Error(w, "missing chargePointId", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"ocpp1.6"},
	})
	if err != nil {
		log.Printf("[%s] accept error: %v", chargePointId, err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "closing")

	log.Printf("[%s] connected from %s (subprotocol: %s)", chargePointId, r.RemoteAddr, r.Header.Get("Sec-WebSocket-Protocol"))

	ctx := r.Context()
	for {
		typ, reader, err := conn.Reader(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				log.Printf("[%s] disconnected", chargePointId)
				return
			}
			if isNetworkClose(err) {
				log.Printf("[%s] connection closed: %v", chargePointId, err)
				return
			}
			log.Printf("[%s] read error: %v", chargePointId, err)
			return
		}

		if typ != websocket.MessageText {
			log.Printf("[%s] ignoring non-text message", chargePointId)
			continue
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			log.Printf("[%s] read body error: %v", chargePointId, err)
			return
		}

		log.Printf("[%s] << %s", chargePointId, string(body))

		response := handleFrame(body)
		if response != "" {
			log.Printf("[%s] >> %s", chargePointId, response)
			if err := conn.Write(ctx, websocket.MessageText, []byte(response)); err != nil {
				log.Printf("[%s] write error: %v", chargePointId, err)
				return
			}
		}
	}
}

func handleFrame(frame []byte) string {
	s := strings.TrimSpace(string(frame))
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return ""
	}

	inner := s[1 : len(s)-1]
	parts := splitTopLevel(inner)
	if len(parts) < 2 {
		return ""
	}

	typeID := strings.TrimSpace(parts[0])
	uniqueID := strings.TrimSpace(parts[1])

	switch typeID {
	case "2": // CALL
		if len(parts) < 4 {
			return ""
		}
		action := strings.Trim(strings.TrimSpace(parts[2]), `"`)
		payload := strings.TrimSpace(parts[3])
		return handleCall(uniqueID, action, payload)
	case "3": // CALLRESULT
		log.Printf("CALLRESULT uniqueID=%s", uniqueID)
		return ""
	case "4": // CALLERROR
		if len(parts) >= 5 {
			log.Printf("CALLERROR uniqueID=%s code=%s desc=%s", uniqueID, parts[2], parts[3])
		}
		return ""
	default:
		return ""
	}
}

func handleCall(uniqueID, action, payload string) string {
	now := time.Now().UTC().Format(time.RFC3339)

	var responsePayload string
	switch action {
	case "BootNotification":
		responsePayload = fmt.Sprintf(`{"status":"Accepted","currentTime":"%s","interval":300}`, now)
	case "Heartbeat":
		responsePayload = fmt.Sprintf(`{"currentTime":"%s"}`, now)
	case "StatusNotification":
		responsePayload = `{}`
	case "Authorize":
		responsePayload = `{"idTagInfo":{"status":"Accepted"}}`
	case "StartTransaction":
		transactionID := nextTransactionID()
		responsePayload = fmt.Sprintf(`{"transactionId":%d,"idTagInfo":{"status":"Accepted"}}`, transactionID)
	case "MeterValues":
		responsePayload = `{}`
	case "StopTransaction":
		responsePayload = `{"idTagInfo":{"status":"Accepted"}}`
	default:
		return fmt.Sprintf(`[4,"%s","NotImplemented","Action not supported: %s",{}]`, uniqueID, action)
	}

	return fmt.Sprintf(`[3,"%s",%s]`, uniqueID, responsePayload)
}

var txCounter int

func nextTransactionID() int {
	txCounter++
	return txCounter
}

func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, c := range s {
		switch c {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func isNetworkClose(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(*net.OpError); ok {
		return ne.Err.Error() == "use of closed network connection"
	}
	return false
}
