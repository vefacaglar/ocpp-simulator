// callResponses builds the OCPP 1.6J CALLRESULT and CALLERROR
// responses that the simulated charge point sends back to the
// gateway in response to CSMS-initiated CALLs (RemoteStart, Reset,
// ChangeConfiguration, etc.). These responses are produced
// entirely in the browser; the gateway forwards them unchanged
// over MQTT to message-processor/ocpp-core, which audit-log them
// without further action.
//
// Per nextplan §2b: every response payload is built to the OCPP
// 1.6J spec — spec-exact field names, casing, and enum values.
// The internal domain model (transactions, runtime state) is
// never reflected on the wire.

// BuildResponseInput is the subset of the local connector state
// needed to answer CSMS-initiated commands. The simulator is
// intentionally permissive: most commands are acknowledged
// Accepted because the browser cannot enforce deep business
// policy. The point of the simulator is to test the wire format
// and gateway plumbing, not to model every CSMS guard rail.
export interface BuildResponseInput {
  chargePointId: string
  uniqueId: string
}

// buildResponse inspects an inbound CSMS-init CALL action and
// returns the spec-exact CALLRESULT payload. If the action is not
// recognized or cannot be answered, it throws UnknownActionError
// so the caller can wrap it in a NotImplemented CALLERROR.
export function buildResponse(action: string, _input: BuildResponseInput): unknown {
  switch (action) {
    case 'RemoteStartTransaction':
      // The simulator acknowledges the request. The actual
      // StartTransaction is the CP's own call triggered by the
      // user (or by the test driver). Returning Accepted is the
      // spec-exact happy-path answer.
      return { status: 'Accepted' }

    case 'RemoteStopTransaction':
      // Same shape as RemoteStartTransaction: acknowledge and
      // let the CP-side StopTransaction flow finish the
      // bookkeeping.
      return { status: 'Accepted' }

    case 'Reset':
      // The request carries a `type` field; we don't need to
      // inspect it here because the request is the only place
      // the type appears. The response is a status enum.
      return { status: 'Accepted' }

    case 'UnlockConnector':
      // Status enum: "Unlocked" or "UnlockFailed". The
      // simulator only ever reports Unlocked because unlocking
      // is the default behavior of a virtual charge point.
      return { status: 'Unlocked' }

    case 'ChangeConfiguration':
      return { status: 'Accepted' }

    case 'GetConfiguration': {
      // The simulator answers with a representative set of
      // configuration keys. These are the OCPP 1.6J keys the
      // message-processor expects to see in the log; they are
      // not authoritative. unknownKey is always empty because
      // the simulator advertises all keys it knows.
      const configurationKey: { key: string; readonly: boolean; value?: string }[] = [
        { key: 'HeartbeatInterval', readonly: false, value: '300' },
        { key: 'MeterValueSampleInterval', readonly: false, value: '60' },
        { key: 'ClockAlignedDataInterval', readonly: false, value: '0' },
        { key: 'ConnectionTimeOut', readonly: false, value: '60' },
        { key: 'WebSocketPingInterval', readonly: false, value: '0' },
        { key: 'LocalPreAuthorize', readonly: false, value: 'false' },
        { key: 'StopTransactionOnEVSideDisconnect', readonly: true, value: 'true' },
      ]
      return { configurationKey, unknownKey: [] }
    }

    case 'TriggerMessage':
      // TriggerMessage asks the CP to send a specific message
      // (Boot, Heartbeat, Status, MeterValues). The simulator
      // returns Accepted without dispatching; the test driver
      // invokes the corresponding button manually so the
      // behavior is observable in the UI.
      return { status: 'Accepted' }

    case 'ChangeAvailability':
      // Operative/Inoperative. We acknowledge and let the
      // caller drive the state via the connector control
      // buttons.
      return { status: 'Accepted' }

    default:
      // Unknown CSMS-init action. The simulator does not know
      // how to answer. The caller wraps this into a
      // NotImplemented CALLERROR.
      throw new UnknownActionError(action)
  }
}

// UnknownActionError is thrown by buildResponse for actions the
// simulator does not implement. The caller converts it into a
// NotImplemented CALLERROR per OCPP 1.6J §5.3.
export class UnknownActionError extends Error {
  readonly action: string
  constructor(action: string) {
    super(`unknown CSMS-init action: ${action}`)
    this.action = action
  }
}

// CALLERROR code per OCPP 1.6J §5.3: NotImplemented when the
// receiving party does not support the action. We use the same
// string the rest of the OCPP wire uses.
export const NOT_IMPLEMENTED = 'NotImplemented'
