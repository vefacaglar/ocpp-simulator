package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CoreClient is the interface the processor uses to ask ocpp-core for
// business decisions (Authorize, StartTransaction, StopTransaction).
// The HTTP implementation lives in HTTPCoreClient; tests provide a
// fake.
type CoreClient interface {
	// Authorize asks the core whether the given idTag is
	// accepted. Returns the response payload bytes the processor
	// will use as the Authorize.conf body.
	Authorize(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error)
	// StartTransaction asks the core to begin a transaction and
	// return the CALLRESULT payload (containing transactionId and
	// idTagInfo).
	StartTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error)
	// StopTransaction asks the core to end a transaction. Payload
	// is the .req body from the CP. The returned payload becomes
	// the .conf body.
	StopTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error)
}

// HTTPCoreClient implements CoreClient over HTTP. The endpoint
// paths follow the nextplan T26 surface (paths documented in
// plan.md §5.4 ocpp-core responsibilities). Request and response
// bodies are JSON.
type HTTPCoreClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewHTTPCoreClient(baseURL string) *HTTPCoreClient {
	return &HTTPCoreClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type coreEnvelope struct {
	ChargePointID string          `json:"chargePointId"`
	Payload       json.RawMessage `json:"payload"`
}

// doPOST issues a POST to baseURL/path with a coreEnvelope body and
// returns the response body bytes. Non-2xx responses and network
// errors are surfaced as ErrCoreUnavailable so the handler can emit a
// GenericError CALLERROR.
func (c *HTTPCoreClient) doPOST(ctx context.Context, path, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	env := coreEnvelope{ChargePointID: chargePointID, Payload: payload}
	body, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCoreUnavailable, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrCoreUnavailable, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrCoreUnavailable, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (c *HTTPCoreClient) Authorize(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	return c.doPOST(ctx, "/internal/transactions/authorize", chargePointID, payload)
}

func (c *HTTPCoreClient) StartTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	return c.doPOST(ctx, "/internal/transactions/start", chargePointID, payload)
}

func (c *HTTPCoreClient) StopTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	return c.doPOST(ctx, "/internal/transactions/stop", chargePointID, payload)
}

// corePayload extracts the .payload field from a coreEnvelope response
// body. ocpp-core wraps its CALLRESULT payload in {payload: ...} so
// the processor can hand the bytes straight to the codec.
func corePayload(respBody []byte) (json.RawMessage, error) {
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, errors.New("core response is not a coreEnvelope")
	}
	if len(env.Payload) == 0 {
		return nil, errors.New("core response has empty payload")
	}
	return env.Payload, nil
}
