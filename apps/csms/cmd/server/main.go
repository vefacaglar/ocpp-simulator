package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/user/ocpp-simulator/apps/csms/internal/csms"
	"nhooyr.io/websocket"
)

var (
	connections *csms.ConnectionRegistry
	pending     *csms.PendingCallRegistry
	api         *csms.API
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	connections = csms.NewConnectionRegistry()
	pending = csms.NewPendingCallRegistry()
	api = csms.NewAPI(connections, pending)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ocpp/{chargePointId}", handleOCPP)
	api.RegisterRoutes(mux)

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

	// Register the connection
	connections.Register(chargePointId, conn)
	defer connections.Unregister(chargePointId)

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
	var arr []json.RawMessage
	if err := json.Unmarshal(frame, &arr); err != nil {
		return ""
	}
	if len(arr) < 2 {
		return ""
	}

	var typeID int
	if err := json.Unmarshal(arr[0], &typeID); err != nil {
		return ""
	}

	var uniqueID string
	if err := json.Unmarshal(arr[1], &uniqueID); err != nil {
		return ""
	}

	switch typeID {
	case 2: // CALL
		if len(arr) < 4 {
			return ""
		}
		var action string
		if err := json.Unmarshal(arr[2], &action); err != nil {
			return ""
		}
		return handleCall(uniqueID, action, string(arr[3]))
	case 3: // CALLRESULT
		log.Printf("CALLRESULT uniqueID=%s", uniqueID)
		// Dispatch to pending registry for CSMS-initiated CALLs
		if len(arr) >= 3 {
			pending.Resolve(uniqueID, []byte(arr[2]))
		}
		return ""
	case 4: // CALLERROR
		if len(arr) >= 5 {
			log.Printf("CALLERROR uniqueID=%s", uniqueID)
			var errorCode, errorDesc string
			json.Unmarshal(arr[2], &errorCode)
			json.Unmarshal(arr[3], &errorDesc)
			pending.ResolveError(uniqueID, errorCode, errorDesc)
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
		responsePayload = fmt.Sprintf(`{"status":"Accepted","currentTime":"%s","interval":30}`, now)
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

func isNetworkClose(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(*net.OpError); ok {
		return ne.Err.Error() == "use of closed network connection"
	}
	return false
}
