// Package processor is the OCPP message-processor service. It consumes
// raw OCPP-J frames from ocpp/+/in, dispatches by action, and publishes
// raw CALLRESULT/CALLERROR frames to ocpp/{chargePointId}/out. The
// service has no database; per spec (nextplan §2b separation), audit
// observability is emitted to stdout/log only. The canonical OCPP
// message log is owned by ocpp-core, which subscribes to the same raw
// topics independently.
package processor

import (
	"encoding/json"
	"log"
)

// AuditEvent is the stdout/log-only record produced for every
// consumed inbound frame and every published outbound response. It is
// a wrapper for developer observability; it is never placed on the
// OCPP wire and never sent over MQTT or to another service.
type AuditEvent struct {
	Kind          string `json:"kind"` // "consumed" or "published"
	ChargePointID string `json:"chargePointId"`
	OCPPVersion   string `json:"ocppVersion,omitempty"`
	Topic         string `json:"topic"`
	Action        string `json:"action,omitempty"`
	UniqueID      string `json:"uniqueId,omitempty"`
	MessageType   string `json:"messageType,omitempty"`
	Direction     string `json:"direction,omitempty"`  // "inbound" or "outbound"
	Decision      string `json:"decision,omitempty"`    // for "published": "responded" | "noop" | "not_implemented" | "parse_error"
	Note          string `json:"note,omitempty"`
	// Frame is the raw wire payload as a JSON string. Included only
	// in dev/debug builds; production can omit it via Emitter
	// config. Never sent over MQTT.
	Frame string `json:"frame,omitempty"`
}

// AuditEmitter is the sink for AuditEvents. The production sink
// writes one JSON line per event to stdout; tests can supply an
// in-memory collector to assert on emitted events.
type AuditEmitter interface {
	Emit(AuditEvent)
}

// StdoutEmitter writes one JSON line per audit event to the standard
// logger. It is the default production emitter.
type StdoutEmitter struct {
	// IncludeFrame controls whether the raw frame string is
	// included in the emitted event. Useful to keep stdout quiet
	// at high traffic levels.
	IncludeFrame bool
}

func (e StdoutEmitter) Emit(ev AuditEvent) {
	// Use the standard logger so log level/format can be tuned via
	// log.SetFlags in the entrypoint. We encode the event to JSON
	// so downstream log shippers can parse it.
	clone := ev
	if !e.IncludeFrame {
		clone.Frame = ""
	}
	data, err := json.Marshal(clone)
	if err != nil {
		log.Printf("[message-processor/audit] marshal failed: %v", err)
		return
	}
	log.Printf("audit %s", data)
}

// defaultEmitter is the package-level emitter used by EmitAudit when
// the Processor was constructed without an explicit emitter. It can
// be replaced in tests via SetDefaultEmitter.
var defaultEmitter AuditEmitter = StdoutEmitter{IncludeFrame: false}

// SetDefaultEmitter swaps the package-level default emitter. Mainly
// for tests.
func SetDefaultEmitter(e AuditEmitter) { defaultEmitter = e }

// EmitAudit is a convenience for code paths that don't have direct
// access to the Processor struct. It uses the package-level
// defaultEmitter.
func EmitAudit(ev AuditEvent) { defaultEmitter.Emit(ev) }
