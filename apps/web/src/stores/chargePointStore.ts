import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api/chargePointsApi'
import type { ChargePoint, Connector } from '../api/chargePointsApi'
import { GatewayClient, type IGatewayClient } from '../ocpp/gatewayClient'
import { buildResponse, UnknownActionError, NOT_IMPLEMENTED } from '../ocpp/callResponses'
import { uniqueId } from '../ocpp/uniqueId'

// ConnectorState mirrors the OCPP 1.6J connector status enum plus a
// local-only `cablePluggedIn` flag the UI uses to drive the plug-in
// action. The status string is the spec-exact enum value so it can
// be emitted verbatim on the wire.
export type ConnectorStatus =
  | 'Available'
  | 'Preparing'
  | 'Charging'
  | 'SuspendedEV'
  | 'SuspendedEVSE'
  | 'Finishing'
  | 'Reserved'
  | 'Unavailable'
  | 'Faulted'

export interface ConnectorState {
  status: ConnectorStatus
  cablePluggedIn: boolean
}

interface TransactionState {
  // 1.6J numericId, populated asynchronously from the
  // StartTransaction.conf. Null while pending.
  transactionId: number | null
  connectorId: number
  idTag: string
  meterStart: number
  meterCurrent: number
  status: 'preparing' | 'charging' | 'finishing'
}

interface PendingCallInfo {
  action: string
  sentAt: number
  connectorId?: number
  idTag?: string
}

export interface ChargePointRuntime {
  client: IGatewayClient
  registration: 'disconnected' | 'pending' | 'accepted' | 'rejected'
  heartbeatInterval: number | null
  heartbeatTimer: ReturnType<typeof setTimeout> | null
  lastMessageSentAt: number
  pendingCalls: Map<string, PendingCallInfo>
  connectorStates: Map<number, ConnectorState>
  transactions: Map<number, TransactionState>
  meterCounter: number
  meterValuesTimer: ReturnType<typeof setInterval> | null
}

export const useChargePointStore = defineStore('chargePoint', () => {
  const chargePoints = ref<ChargePoint[]>([])
  const selectedId = ref<string | null>(null)
  const selectedDetail = ref<{ chargePoint: ChargePoint; connectors: Connector[] } | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const cpStates = ref<Map<string, ChargePointRuntime>>(new Map())

  const selectedChargePoint = computed(() =>
    chargePoints.value.find((cp) => cp.id === selectedId.value) ?? null
  )

  const registrationState = computed(() => {
    if (!selectedId.value) return 'disconnected'
    return cpStates.value.get(selectedId.value)?.registration ?? 'disconnected'
  })

  const isRegistered = computed(() => registrationState.value === 'accepted')

  function getConnectorStatus(connectorId: number): ConnectorStatus {
    if (!selectedId.value) return 'Available'
    return cpStates.value.get(selectedId.value)?.connectorStates.get(connectorId)?.status ?? 'Available'
  }

  function isCablePluggedIn(connectorId: number): boolean {
    if (!selectedId.value) return false
    return cpStates.value.get(selectedId.value)?.connectorStates.get(connectorId)?.cablePluggedIn ?? false
  }

  function isConnected(cpId: string): boolean {
    return cpStates.value.has(cpId) && cpStates.value.get(cpId)!.client.isOpen
  }

  // ─── CRUD (simulator-api) ─────────────────────────────────────────────

  async function loadChargePoints() {
    loading.value = true
    error.value = null
    try {
      chargePoints.value = await api.fetchChargePoints()
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function selectChargePoint(id: string) {
    selectedId.value = id
    try {
      selectedDetail.value = await api.fetchChargePoint(id)
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function createChargePoint(input: { id: string; name?: string; ocppVersion?: string; centralSystemUrl?: string }) {
    error.value = null
    try {
      await api.createChargePoint(input)
      await loadChargePoints()
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function removeChargePoint(id: string) {
    error.value = null
    try {
      disconnectWS(id)
      await api.deleteChargePoint(id)
      if (selectedId.value === id) {
        selectedId.value = null
        selectedDetail.value = null
      }
      await loadChargePoints()
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function addConnector() {
    if (!selectedId.value) return
    error.value = null
    try {
      await api.addConnector(selectedId.value)
      await selectChargePoint(selectedId.value)
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function removeConnector(connectorId: number) {
    if (!selectedId.value) return
    error.value = null
    try {
      await api.deleteConnector(selectedId.value, connectorId)
      await selectChargePoint(selectedId.value)
    } catch (e: any) {
      error.value = e.message
    }
  }

  // ─── Per-CP WebSocket lifecycle (ocpp-gateway) ────────────────────────

  async function connectChargePoint() {
    if (!selectedId.value) return
    const cpId = selectedId.value
    if (cpStates.value.has(cpId)) {
      return
    }
    // Ensure the charge point detail is loaded so we know the
    // OCPP version to advertise in the WebSocket subprotocol
    // header. selectChargePoint is idempotent and refreshes
    // selectedDetail from simulator-api.
    if (!selectedDetail.value || selectedDetail.value.chargePoint.id !== cpId) {
      try {
        await selectChargePoint(cpId)
      } catch (e) {
        // Fall through; if selectedDetail is still missing
        // we'll fall back to '1.6J' below.
      }
    }
    const ocppVersion =
      selectedDetail.value?.chargePoint.ocppVersion ?? '1.6J'

    let runtime!: ChargePointRuntime
    const client = new GatewayClient({
      chargePointId: cpId,
      ocppVersion,
      onOpen: () => {
        runtime.registration = 'pending'
        sendBootNotification(cpId)
        loadChargePoints()
        selectChargePoint(cpId)
      },
      onClose: () => {
        if (runtime.heartbeatTimer) {
          clearTimeout(runtime.heartbeatTimer)
          runtime.heartbeatTimer = null
        }
        if (runtime.meterValuesTimer) {
          clearInterval(runtime.meterValuesTimer)
          runtime.meterValuesTimer = null
        }
        runtime.registration = 'disconnected'
      },
      onError: () => {
        runtime.registration = 'disconnected'
        error.value = `Connection failed for ${cpId}`
      },
      onFrame: (frame) => handleOCPPFrame(cpId, frame),
    })
    runtime = {
      client,
      registration: 'disconnected',
      heartbeatInterval: null,
      heartbeatTimer: null,
      lastMessageSentAt: Date.now(),
      pendingCalls: new Map(),
      connectorStates: new Map(),
      transactions: new Map(),
      meterCounter: 0,
      meterValuesTimer: null,
    }
    cpStates.value.set(cpId, runtime)
    client.connect()
  }

  function disconnectChargePoint() {
    if (!selectedId.value) return
    disconnectWS(selectedId.value)
  }

  function disconnectWS(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state) return
    if (state.heartbeatTimer) {
      clearTimeout(state.heartbeatTimer)
      state.heartbeatTimer = null
    }
    if (state.meterValuesTimer) {
      clearInterval(state.meterValuesTimer)
      state.meterValuesTimer = null
    }
    state.client.close()
    cpStates.value.delete(cpId)
  }

  // ─── OCPP frame dispatch ──────────────────────────────────────────────

  function handleOCPPFrame(cpId: string, frame: unknown[]) {
    const state = cpStates.value.get(cpId)
    if (!state) return

    const typeID = frame[0] as number
    const uniqueId = frame[1] as string

    switch (typeID) {
      case 2: // CALL — CSMS-initiated command, build a CALLRESULT.
        handleInboundCall(cpId, state, uniqueId, frame[2] as string, frame[3] as Record<string, unknown> | undefined)
        return
      case 3: // CALLRESULT — answer to a CALL we sent.
        handleCallResult(cpId, state, uniqueId, frame[2] as Record<string, unknown>)
        return
      case 4: // CALLERROR — error reply to a CALL we sent.
        handleCallError(cpId, state, uniqueId, frame[2] as string, frame[3] as string)
        return
      default:
        // Unknown type. Drop and log. The codec would never
        // produce this; the gateway forwards raw frames so we
        // must defend.
        // eslint-disable-next-line no-console
        console.warn('[ocpp] unknown message type', typeID)
    }
  }

  // handleInboundCall answers a CSMS-initiated CALL (RemoteStart,
  // Reset, etc.) with a spec-exact CALLRESULT. Unknown actions get
  // a NotImplemented CALLERROR per OCPP 1.6J §5.3. The response
  // frame is sent back through the same GatewayClient.
  function handleInboundCall(
    _cpId: string,
    state: ChargePointRuntime,
    uid: string,
    action: string,
    _payload: Record<string, unknown> | undefined
  ) {
    try {
      const responsePayload = buildResponse(action, {
        chargePointId: _cpId,
        uniqueId: uid,
      })
      const resultFrame = [3, uid, responsePayload]
      state.client.send(resultFrame)
    } catch (err) {
      if (err instanceof UnknownActionError) {
        const errorFrame = [4, uid, NOT_IMPLEMENTED, `action ${action} not supported by simulator`, {}]
        state.client.send(errorFrame)
        return
      }
      // Any other error: GenericError CALLERROR. The
      // description carries the cause.
      const desc = err instanceof Error ? err.message : 'unknown error'
      const errorFrame = [4, uid, 'GenericError', desc, {}]
      state.client.send(errorFrame)
    }
  }

  // handleCallResult resolves a pending call we sent and drives
  // the relevant state transitions.
  function handleCallResult(cpId: string, state: ChargePointRuntime, uid: string, payload: Record<string, unknown>) {
    const pending = state.pendingCalls.get(uid)
    if (!pending) return
    state.pendingCalls.delete(uid)

    switch (pending.action) {
      case 'BootNotification':
        handleBootNotificationResponse(cpId, state, payload)
        return
      case 'Authorize':
        handleAuthorizeResponse(cpId, state, payload, pending)
        return
      case 'StartTransaction':
        handleStartTransactionResponse(cpId, state, payload)
        return
      case 'StopTransaction':
        handleStopTransactionResponse(cpId, state, payload)
        return
    }
  }

  // handleCallError records a CALLERROR reply and reverts the
  // connector to a safe state when the failed call would have
  // moved it forward.
  function handleCallError(_cpId: string, state: ChargePointRuntime, uid: string, code: string, desc: string) {
    const pending = state.pendingCalls.get(uid)
    if (!pending) return
    state.pendingCalls.delete(uid)

    if (pending.action === 'Authorize' || pending.action === 'StartTransaction') {
      if (pending.connectorId !== undefined) {
        const cs = state.connectorStates.get(pending.connectorId)
        if (cs) {
          cs.status = 'Available'
          cs.cablePluggedIn = false
        }
        state.transactions.delete(pending.connectorId)
      }
      error.value = `${pending.action} CALLERROR (${code}): ${desc}`
    }
  }

  // ─── BootNotification / Heartbeat (CP→CSMS) ──────────────────────────

  function handleBootNotificationResponse(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown>) {
    const status = payload.status as string
    const interval = (payload.interval as number) ?? 300

    if (status === 'Accepted') {
      state.registration = 'accepted'
      state.heartbeatInterval = interval
      sendInitialStatusNotifications(cpId)
      scheduleHeartbeat(cpId)
      loadChargePoints()
      selectChargePoint(cpId)
    } else if (status === 'Pending') {
      state.registration = 'pending'
    } else if (status === 'Rejected') {
      state.registration = 'rejected'
      // Retry after the suggested interval.
      setTimeout(() => {
        if (cpStates.value.has(cpId)) {
          sendBootNotification(cpId)
        }
      }, interval * 1000)
    }
  }

  function sendBootNotification(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || !state.client.isOpen) return
    const frame = [2, uniqueId(), 'BootNotification', { chargePointVendor: 'Simulator', chargePointModel: 'OCPP-Sim' }]
    sendFrame(cpId, state, frame, 'BootNotification')
  }

  function sendHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const frame = [2, uniqueId(), 'Heartbeat', {}]
    sendFrame(cpId, state, frame, 'Heartbeat')
  }

  function sendInitialStatusNotifications(cpId: string) {
    const detail = selectedDetail.value
    if (!detail) return
    for (const c of detail.connectors) {
      sendStatusNotification(cpId, c.connectorNumber, 'Available', 'NoError')
    }
  }

  function sendStatusNotification(cpId: string, connectorId: number, status: string, errorCode: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const frame = [2, uniqueId(), 'StatusNotification', {
      connectorId,
      errorCode,
      status,
      timestamp: new Date().toISOString(),
    }]
    sendFrame(cpId, state, frame, 'StatusNotification')
  }

  // ─── Authorize / StartTransaction / StopTransaction / MeterValues ───

  function handleAuthorizeResponse(
    cpId: string,
    state: ChargePointRuntime,
    payload: Record<string, unknown>,
    pending: PendingCallInfo
  ) {
    const idTagInfo = payload.idTagInfo as { status?: string } | undefined
    const status = idTagInfo?.status
    const connectorId = pending.connectorId
    const idTag = pending.idTag ?? 'DEADBEEF'
    if (connectorId === undefined) return
    const cs = state.connectorStates.get(connectorId)
    if (status === 'Accepted') {
      state.meterCounter += 100
      const meterStart = state.meterCounter
      const tx: TransactionState = {
        transactionId: null,
        connectorId,
        idTag,
        meterStart,
        meterCurrent: meterStart,
        status: 'preparing',
      }
      state.transactions.set(connectorId, tx)
      const frame = [2, uniqueId(), 'StartTransaction', {
        connectorId,
        idTag,
        meterStart,
        timestamp: new Date().toISOString(),
      }]
      sendFrame(cpId, state, frame, 'StartTransaction', { connectorId })
    } else {
      if (cs) cs.status = 'Available'
      error.value = `Authorize rejected: ${status ?? 'Unknown'}`
    }
  }

  function handleStartTransactionResponse(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown>) {
    const idTagInfo = payload.idTagInfo as { status?: string } | undefined
    const status = idTagInfo?.status
    const transactionId = payload.transactionId as number | undefined
    const pendingTx = Array.from(state.transactions.values()).find((tx) => tx.transactionId === null)
    if (!pendingTx) return
    const connectorId = pendingTx.connectorId
    const cs = state.connectorStates.get(connectorId)
    if (status === 'Accepted' && transactionId) {
      pendingTx.transactionId = transactionId
      pendingTx.status = 'charging'
      if (cs) cs.status = 'Charging'
      sendStatusNotification(cpId, connectorId, 'Charging', 'NoError')
      startMeterValuesTimer(cpId)
    } else {
      state.transactions.delete(connectorId)
      if (cs) cs.status = 'Available'
      error.value = `StartTransaction rejected: ${status ?? 'Unknown'}`
    }
  }

  function handleStopTransactionResponse(_cpId: string, _state: ChargePointRuntime, _payload: Record<string, unknown>) {
    // The .conf is informational; the local cleanup happened
    // before the call was sent.
  }

  function startMeterValuesTimer(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state) return
    if (state.meterValuesTimer) clearInterval(state.meterValuesTimer)
    state.meterValuesTimer = setInterval(() => {
      for (const [connectorId, tx] of state.transactions) {
        if (tx.status === 'charging' && tx.transactionId !== null) {
          sendMeterValuesForTransaction(cpId, connectorId, tx)
        }
      }
    }, 10000)
  }

  function sendMeterValuesForTransaction(cpId: string, connectorId: number, tx: TransactionState) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    state.meterCounter += 100
    tx.meterCurrent = state.meterCounter
    const frame = [2, uniqueId(), 'MeterValues', {
      connectorId,
      transactionId: tx.transactionId,
      meterValue: [
        {
          timestamp: new Date().toISOString(),
          sampledValue: [
            { value: String(tx.meterCurrent), measurand: 'Energy.Active.Import.Register', unit: 'Wh', context: 'Sample.Periodic' },
            { value: '7360', measurand: 'Power.Active.Import', unit: 'W' },
          ],
        },
      ],
    }]
    sendFrame(cpId, state, frame, 'MeterValues')
  }

  // ─── Wire send helper ────────────────────────────────────────────────

  function sendFrame(
    cpId: string,
    state: ChargePointRuntime,
    frame: unknown[],
    action: string,
    extra?: { connectorId?: number; idTag?: string }
  ) {
    if (!state.client.send(frame)) {
      // eslint-disable-next-line no-console
      console.warn('[ocpp] send dropped: socket not open', { cpId, action })
      return
    }
    const uid = frame[1] as string
    state.pendingCalls.set(uid, { action, sentAt: Date.now(), ...extra })
    state.lastMessageSentAt = Date.now()
    resetHeartbeatTimer(cpId)
  }

  // ─── Heartbeat scheduling ────────────────────────────────────────────

  function scheduleHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.heartbeatInterval) return
    if (state.heartbeatTimer) {
      clearTimeout(state.heartbeatTimer)
      state.heartbeatTimer = null
    }
    const delay = state.heartbeatInterval * 1000
    state.heartbeatTimer = setTimeout(() => {
      const since = Date.now() - state.lastMessageSentAt
      if (since >= delay) sendHeartbeat(cpId)
      scheduleHeartbeat(cpId)
    }, delay)
  }

  function resetHeartbeatTimer(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return
    scheduleHeartbeat(cpId)
  }

  // ─── Connector flow actions (UI buttons) ────────────────────────────

  function plugInConnector(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }
    let cs = state.connectorStates.get(connectorId)
    if (!cs) {
      cs = { status: 'Available', cablePluggedIn: false }
      state.connectorStates.set(connectorId, cs)
    }
    if (cs.status !== 'Available') {
      error.value = `Cannot plug in: connector is ${cs.status}`
      return
    }
    cs.cablePluggedIn = true
    cs.status = 'Preparing'
    sendStatusNotification(cpId, connectorId, 'Preparing', 'NoError')
  }

  function authorizeConnector(connectorId: number, idTag: string) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) {
      error.value = 'Not registered'
      return
    }
    const cs = state.connectorStates.get(connectorId)
    if (!cs || cs.status !== 'Preparing' || !cs.cablePluggedIn) {
      error.value = 'Connector must be Preparing with cable plugged in'
      return
    }
    const frame = [2, uniqueId(), 'Authorize', { idTag }]
    sendFrame(cpId, state, frame, 'Authorize', { connectorId, idTag })
  }

  function unplugConnector(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return
    const cs = state.connectorStates.get(connectorId)
    if (!cs) return
    if (cs.status === 'Preparing' || cs.status === 'Finishing') {
      cs.cablePluggedIn = false
      cs.status = 'Available'
      sendStatusNotification(cpId, connectorId, 'Available', 'NoError')
    } else if (cs.status === 'Charging') {
      error.value = 'Stop the transaction before unplugging'
    } else {
      error.value = `Cannot unplug: connector is ${cs.status}`
    }
  }

  function stopConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null) {
      error.value = 'No active transaction'
      return
    }
    if (state.meterValuesTimer) {
      clearInterval(state.meterValuesTimer)
      state.meterValuesTimer = null
    }
    const cs = state.connectorStates.get(connectorId)
    if (cs) cs.status = 'Finishing'
    sendStatusNotification(cpId, connectorId, 'Finishing', 'NoError')
    state.meterCounter += 50
    const meterStop = state.meterCounter
    const frame = [2, uniqueId(), 'StopTransaction', {
      transactionId: tx.transactionId,
      meterStop,
      timestamp: new Date().toISOString(),
      reason: 'Local',
      transactionData: [
        {
          timestamp: new Date().toISOString(),
          sampledValue: [
            { value: String(meterStop), measurand: 'Energy.Active.Import.Register', unit: 'Wh', context: 'Transaction.End' },
          ],
        },
      ],
    }]
    sendFrame(cpId, state, frame, 'StopTransaction')
    state.transactions.delete(connectorId)
  }

  function sendConnectorMeterValues(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null) {
      error.value = 'No active transaction'
      return
    }
    state.meterCounter += 10
    tx.meterCurrent = state.meterCounter
    const frame = [2, uniqueId(), 'MeterValues', {
      connectorId,
      transactionId: tx.transactionId,
      meterValue: [
        {
          timestamp: new Date().toISOString(),
          sampledValue: [
            { value: String(tx.meterCurrent), measurand: 'Energy.Active.Import.Register', unit: 'Wh', context: 'Sample.Periodic' },
            { value: '7360', measurand: 'Power.Active.Import', unit: 'W' },
          ],
        },
      ],
    }]
    sendFrame(cpId, state, frame, 'MeterValues')
  }

  function setConnectorStatus(connectorId: number, status: string) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return
    let cs = state.connectorStates.get(connectorId)
    if (!cs) {
      cs = { status: 'Available', cablePluggedIn: false }
      state.connectorStates.set(connectorId, cs)
    }
    cs.status = status as ConnectorStatus
    if (status === 'Available') cs.cablePluggedIn = false
    sendStatusNotification(cpId, connectorId, status, 'NoError')
  }

  // boot/heartbeat UI actions: send the corresponding CALL on
  // demand. These used to live in a runtime on the Go side; in
  // the v4.3 split they are pure CP-side sends.
  function bootChargePoint() {
    if (!selectedId.value) return
    sendBootNotification(selectedId.value)
  }

  function heartbeatChargePoint() {
    if (!selectedId.value) return
    sendHeartbeat(selectedId.value)
  }

  return {
    chargePoints,
    selectedId,
    selectedDetail,
    loading,
    error,
    selectedChargePoint,
    registrationState,
    isRegistered,
    loadChargePoints,
    selectChargePoint,
    createChargePoint,
    removeChargePoint,
    addConnector,
    removeConnector,
    connectChargePoint,
    disconnectChargePoint,
    bootChargePoint,
    heartbeatChargePoint,
    getConnectorStatus,
    isCablePluggedIn,
    plugInConnector,
    authorizeConnector,
    unplugConnector,
    stopConnectorTransaction,
    sendConnectorMeterValues,
    setConnectorStatus,
    isConnected,
  }
})
