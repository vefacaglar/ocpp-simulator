import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api/chargePointsApi'
import type { ChargePoint, Connector } from '../api/chargePointsApi'
import { GatewayClient, type IGatewayClient } from '../ocpp/gatewayClient'
import { buildResponse, UnknownActionError, NOT_IMPLEMENTED } from '../ocpp/callResponses'
import { uniqueId } from '../ocpp/uniqueId'
import { useRealtimeStore } from './realtimeStore'
import { getAppState, setAppState } from '../db/browserDb'
import { useSettingsStore } from './settingsStore'

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
  availabilityRequested?: 'Operative' | 'Inoperative'
  faultCode?: string
  pendingRemoteStartIdTag?: string
}

type StopReason =
  | 'Local'
  | 'Remote'
  | 'EVDisconnected'
  | 'EmergencyStop'
  | 'PowerLoss'
  | 'Reboot'
  | 'SoftReset'
  | 'HardReset'
  | 'UnlockCommand'
  | 'Other'

interface TransactionState {
  // Wire transactionId. 1.6J: a numeric id assigned by the CSMS
  // asynchronously in StartTransaction.conf. 2.0.1: a string GUID
  // chosen by the CP up-front (in TransactionEvent Started) —
  // present from the moment the transaction begins. Null only
  // briefly while a 1.6J Start is in flight.
  transactionId: string | null
  connectorId: number
  // 2.0.1 only. Per-transaction monotonically-increasing counter
  // starting at 0; the spec requires the CP to assign it and the
  // CSMS uses it to detect missing events. 1.6J leaves this null.
  seqNo: number | null
  idTag: string
  idTokenType: string
  meterStart: number
  meterCurrent: number
  meterStop: number | null
  startedAt: string
  lastSampleAt: string | null
  soc: number
  voltage: number
  current: number
  powerW: number
  stopReason: StopReason | null
  stopPending: boolean
  status: 'preparing' | 'charging' | 'finishing'
}

interface PendingCallInfo {
  action: string
  sentAt: number
  connectorId?: number
  idTag?: string
  // 2.0.1 only: the OCPP 2.0.1 idToken.type carried by the
  // Authorize.req so a subsequent TransactionEvent(Started) can
  // include the same idToken (combined-authorization path).
  idTokenType?: string
}

export interface ChargePointRuntime {
  client: IGatewayClient
  ocppVersion: string
  connectionStatus: 'disconnected' | 'connecting' | 'connected'
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
  const realtimeStore = useRealtimeStore()
  const settingsStore = useSettingsStore()
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
    return cpStates.value.get(cpId)?.connectionStatus === 'connected'
  }

  function getConnectionStatus(cpId: string): 'disconnected' | 'connecting' | 'connected' {
    return cpStates.value.get(cpId)?.connectionStatus ?? 'disconnected'
  }

  // activeTransactionFor returns a read-only snapshot of the
  // transaction state for a connector, or null when no
  // transaction is in flight. Used by the UI to render the
  // current transactionId, seqNo (2.0.1), and meter values.
  function activeTransactionFor(connectorId: number): TransactionState | null {
    if (!selectedId.value) return null
    return cpStates.value.get(selectedId.value)?.transactions.get(connectorId) ?? null
  }

  function setRuntime(cpId: string, runtime: ChargePointRuntime) {
    const next = new Map(cpStates.value)
    next.set(cpId, runtime)
    cpStates.value = next
  }

  function updateRuntime(cpId: string, update: (runtime: ChargePointRuntime) => void) {
    const runtime = cpStates.value.get(cpId)
    if (!runtime) return
    update(runtime)
    setRuntime(cpId, runtime)
  }

  const chargePointsWithStatus = computed(() => {
    return chargePoints.value.map(cp => ({
      ...cp,
      status: getConnectionStatus(cp.id)
    }))
  })

  // ─── Local CRUD (browser IndexedDB) ───────────────────────────────────

  async function loadChargePoints() {
    loading.value = true
    error.value = null
    try {
      chargePoints.value = await api.fetchChargePoints()
      const persistedSelectedId = await getAppState<string>('selectedChargePointId')
      if (persistedSelectedId && chargePoints.value.some((cp) => cp.id === persistedSelectedId)) {
        await selectChargePoint(persistedSelectedId)
      }
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
      await setAppState('selectedChargePointId', id)
      await realtimeStore.loadEventsForChargePoint(id)
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

  async function updateChargePoint(id: string, input: { name?: string; ocppVersion?: string }) {
    error.value = null
    try {
      await api.updateChargePoint(id, input)
      await loadChargePoints()
      if (selectedId.value === id) {
        await selectChargePoint(id)
      }
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function removeChargePoint(id: string) {
    error.value = null
    try {
      disconnectWS(id)
      await api.deleteChargePoint(id)
      realtimeStore.clearEventsForChargePoint(id)
      if (selectedId.value === id) {
        selectedId.value = null
        selectedDetail.value = null
        await setAppState('selectedChargePointId', null)
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

  function connectChargePointById(cpId: string, ocppVersion: string) {
    if (isConnected(cpId)) return
    if (cpStates.value.has(cpId)) {
      disconnectWS(cpId)
    }
    let runtime!: ChargePointRuntime
    const client = new GatewayClient({
      chargePointId: cpId,
      ocppVersion,
      centralSystemUrl: settingsStore.centralSystemUrl,
      onOpen: () => {
        updateRuntime(cpId, (current) => {
          current.connectionStatus = 'connected'
          current.registration = 'pending'
        })
        sendBootNotification(cpId)
      },
      onClose: () => {
        updateRuntime(cpId, (current) => {
          current.connectionStatus = 'disconnected'
        })
      },
      onError: () => {
        updateRuntime(cpId, (current) => {
          current.connectionStatus = 'disconnected'
          current.registration = 'disconnected'
        })
        error.value = `Connection failed for ${cpId}`
      },
      onFrame: (frame) => handleOCPPFrame(cpId, frame),
    })
    runtime = {
      client,
      ocppVersion,
      connectionStatus: 'connecting',
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
    setRuntime(cpId, runtime)
    client.connect()
  }

  async function connectChargePoint() {
    if (!selectedId.value) return
    const cpId = selectedId.value
    if (!selectedDetail.value || selectedDetail.value.chargePoint.id !== cpId) {
      try {
        await selectChargePoint(cpId)
      } catch (e) {
        // Fall through; fall back to '1.6J' below.
      }
    }
    const ocppVersion = selectedDetail.value?.chargePoint.ocppVersion ?? '1.6J'
    connectChargePointById(cpId, ocppVersion)
  }

  function disconnectChargePoint() {
    if (!selectedId.value) return
    disconnectWS(selectedId.value)
  }

  function connectAll() {
    for (const cp of chargePoints.value) {
      if (!isConnected(cp.id)) {
        connectChargePointById(cp.id, cp.ocppVersion ?? '1.6J')
      }
    }
  }

  function disconnectAll() {
    for (const cp of chargePoints.value) {
      if (cpStates.value.has(cp.id)) {
        disconnectWS(cp.id)
      }
    }
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
    const next = new Map(cpStates.value)
    next.delete(cpId)
    cpStates.value = next
  }

  // ─── OCPP frame dispatch ──────────────────────────────────────────────

  function handleOCPPFrame(cpId: string, frame: unknown[]) {
    const state = cpStates.value.get(cpId)
    if (!state) return

    appendOcppLog(cpId, 'in', frame)

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

  // handleInboundCall answers a CSMS-initiated CALL and applies the
  // same local DC session flow the UI uses. Stateful commands decide
  // Accepted/Rejected from connector and transaction state, then the
  // CP emits its own OCPP calls over this WebSocket.
  function handleInboundCall(
    cpId: string,
    state: ChargePointRuntime,
    uid: string,
    action: string,
    payload: Record<string, unknown> | undefined
  ) {
    try {
      const responsePayload = handleStatefulInboundCall(cpId, state, action, payload) ?? buildResponse(action, {
        chargePointId: cpId,
        uniqueId: uid,
      })
      const resultFrame = [3, uid, responsePayload]
      sendResponseFrame(cpId, state, resultFrame, action)
    } catch (err) {
      if (err instanceof UnknownActionError) {
        const errorFrame = [4, uid, NOT_IMPLEMENTED, `action ${action} not supported by simulator`, {}]
        sendResponseFrame(cpId, state, errorFrame, action)
        return
      }
      // Any other error: GenericError CALLERROR. The
      // description carries the cause.
      const desc = err instanceof Error ? err.message : 'unknown error'
      const errorFrame = [4, uid, 'GenericError', desc, {}]
      sendResponseFrame(cpId, state, errorFrame, action)
    }
  }

  function handleStatefulInboundCall(
    cpId: string,
    state: ChargePointRuntime,
    action: string,
    payload: Record<string, unknown> | undefined
  ): Record<string, string> | null {
    switch (action) {
      case 'RemoteStartTransaction':
        return handleRemoteStart(cpId, state, payload)
      case 'RemoteStopTransaction':
        return handleRemoteStop(cpId, state, payload)
      case 'RequestStartTransaction':
        return handleRequestStartTransaction(cpId, state, payload)
      case 'RequestStopTransaction':
        return handleRequestStopTransaction(cpId, state, payload)
      case 'UnlockConnector':
        return handleUnlockConnector(cpId, state, payload)
      case 'Reset':
        return handleReset(cpId, state, payload)
      case 'ChangeAvailability':
        return handleChangeAvailability(cpId, state, payload)
      case 'TriggerMessage':
        return handleTriggerMessage(cpId, state, payload)
      default:
        return null
    }
  }

  function handleRemoteStart(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const idTag = typeof payload?.idTag === 'string' ? payload.idTag : ''
    const requestedConnectorId = typeof payload?.connectorId === 'number' ? payload.connectorId : undefined
    const connectorId = requestedConnectorId ?? firstConnectorIdInState(state)
    if (!idTag || connectorId === undefined) return { status: 'Rejected' }

    const cs = ensureConnectorState(state, connectorId)
    if (cs.status !== 'Available' && cs.status !== 'Preparing') {
      appendRuntimeEvent(cpId, 'runtime.command_rejected', `RemoteStart rejected: connector ${connectorId} is ${cs.status}`, connectorId, payload)
      return { status: 'Rejected' }
    }

    cs.pendingRemoteStartIdTag = idTag
    appendRuntimeEvent(cpId, 'runtime.remote_start_received', `RemoteStart accepted for connector ${connectorId}`, connectorId, payload)
    setTimeout(() => {
      const current = cpStates.value.get(cpId)
      if (!current) return
      const currentConnector = ensureConnectorState(current, connectorId)
      if (currentConnector.status === 'Available') {
        currentConnector.cablePluggedIn = true
        currentConnector.status = 'Preparing'
        appendRuntimeEvent(cpId, 'connector.cable_plugged', `Cable plugged on connector ${connectorId} by remote start`, connectorId)
        sendStatusNotification(cpId, connectorId, 'Preparing', 'NoError')
      }
      if (currentConnector.status === 'Preparing') {
        sendAuthorize(cpId, current, connectorId, idTag)
      }
    }, 0)
    return { status: 'Accepted' }
  }

  function handleRemoteStop(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    // 1.6J: transactionId is a number. 2.0.1 carries a string.
    // Coerce both to a string comparison key so the lookup is
    // version-agnostic.
    let key: string | undefined
    if (typeof payload?.transactionId === 'number') key = String(payload.transactionId)
    else if (typeof payload?.transactionId === 'string') key = payload.transactionId
    const entry = key !== undefined
      ? Array.from(state.transactions.entries()).find(([, tx]) => tx.transactionId === key)
      : undefined
    if (!entry) {
      appendRuntimeEvent(cpId, 'runtime.command_rejected', `RemoteStop rejected: transaction ${key ?? 'unknown'} not active`, undefined, payload)
      return { status: 'Rejected' }
    }
    appendRuntimeEvent(cpId, 'runtime.remote_stop_received', `RemoteStop accepted for transaction ${key}`, entry[0], payload)
    setTimeout(() => {
      const current = cpStates.value.get(cpId)
      if (current) stopConnectorTransactionInternal(cpId, current, entry[0], 'Remote')
    }, 0)
    return { status: 'Accepted' }
  }

  // handleRequestStartTransaction is the 2.0.1 counterpart of
  // handleRemoteStart. The payload carries {idToken:{idToken,type},
  // remoteStartId, evseId?}. We accept it when the targeted
  // connector is free, then plug in, send Authorize, and let
  // the normal Authorize.conf path drive TransactionEvent(Started).
  function handleRequestStartTransaction(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const idToken = payload?.idToken as { idToken?: string; type?: string } | undefined
    const idTag = idToken?.idToken ?? ''
    const idTokenType = idToken?.type ?? 'ISO14443'
    const evseId = typeof payload?.evseId === 'number' ? payload.evseId : undefined
    const connectorId = evseId ?? firstConnectorIdInState(state)
    if (!idTag || connectorId === undefined) return { status: 'Rejected' }

    const cs = ensureConnectorState(state, connectorId)
    if (cs.status !== 'Available' && cs.status !== 'Preparing') {
      appendRuntimeEvent(cpId, 'runtime.command_rejected', `RequestStart rejected: connector ${connectorId} is ${cs.status}`, connectorId, payload)
      return { status: 'Rejected' }
    }

    cs.pendingRemoteStartIdTag = idTag
    appendRuntimeEvent(cpId, 'runtime.remote_start_received', `RequestStart accepted for connector ${connectorId}`, connectorId, payload)
    setTimeout(() => {
      const current = cpStates.value.get(cpId)
      if (!current) return
      const currentConnector = ensureConnectorState(current, connectorId)
      if (currentConnector.status === 'Available') {
        currentConnector.cablePluggedIn = true
        currentConnector.status = 'Preparing'
        appendRuntimeEvent(cpId, 'connector.cable_plugged', `Cable plugged on connector ${connectorId} by remote start`, connectorId)
        sendStatusNotification(cpId, connectorId, 'Preparing', 'NoError')
      }
      if (currentConnector.status === 'Preparing') {
        sendAuthorize(cpId, current, connectorId, idTag, idTokenType)
      }
    }, 0)
    return { status: 'Accepted' }
  }

  // handleRequestStopTransaction is the 2.0.1 counterpart of
  // handleRemoteStop. The transactionId is a string GUID the
  // CP chose when the transaction started.
  function handleRequestStopTransaction(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const key = typeof payload?.transactionId === 'string' ? payload.transactionId : undefined
    const entry = key !== undefined
      ? Array.from(state.transactions.entries()).find(([, tx]) => tx.transactionId === key)
      : undefined
    if (!entry) {
      appendRuntimeEvent(cpId, 'runtime.command_rejected', `RequestStop rejected: transaction ${key ?? 'unknown'} not active`, undefined, payload)
      return { status: 'Rejected' }
    }
    appendRuntimeEvent(cpId, 'runtime.remote_stop_received', `RequestStop accepted for transaction ${key}`, entry[0], payload)
    setTimeout(() => {
      const current = cpStates.value.get(cpId)
      if (current) stopConnectorTransactionInternal(cpId, current, entry[0], 'Remote')
    }, 0)
    return { status: 'Accepted' }
  }

  function handleUnlockConnector(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const connectorId = typeof payload?.connectorId === 'number' ? payload.connectorId : undefined
    if (connectorId === undefined) return { status: 'UnlockFailed' }
    const cs = state.connectorStates.get(connectorId)
    if (!cs) return { status: 'UnlockFailed' }
    if (cs.status === 'Preparing' || cs.status === 'Finishing') {
      state.transactions.delete(connectorId)
      cs.cablePluggedIn = false
      cs.status = 'Available'
      cs.pendingRemoteStartIdTag = undefined
      appendRuntimeEvent(cpId, 'connector.unlocked', `Connector ${connectorId} unlocked`, connectorId, payload)
      sendStatusNotification(cpId, connectorId, 'Available', 'NoError')
      return { status: 'Unlocked' }
    }
    return { status: cs.status === 'Charging' ? 'UnlockFailed' : 'Unlocked' }
  }

  function handleReset(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const resetType = payload?.type === 'Hard' ? 'Hard' : payload?.type === 'Soft' ? 'Soft' : null
    if (!resetType) return { status: 'Rejected' }
    const reason: StopReason = resetType === 'Hard' ? 'HardReset' : 'SoftReset'
    appendRuntimeEvent(cpId, 'runtime.reset_requested', `${resetType} reset requested`, undefined, payload)
    for (const [connectorId, tx] of Array.from(state.transactions.entries())) {
      if (tx.transactionId !== null && !tx.stopPending) {
        stopConnectorTransactionInternal(cpId, state, connectorId, reason)
      }
    }
    setTimeout(() => {
      if (!cpStates.value.has(cpId)) return
      for (const [connectorId, cs] of state.connectorStates.entries()) {
        if (cs.status !== 'Unavailable') {
          cs.status = 'Available'
          cs.cablePluggedIn = false
          cs.pendingRemoteStartIdTag = undefined
          sendStatusNotification(cpId, connectorId, 'Available', 'NoError')
        }
      }
      sendBootNotification(cpId)
    }, 500)
    return { status: 'Accepted' }
  }

  function handleChangeAvailability(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const connectorId = typeof payload?.connectorId === 'number' ? payload.connectorId : undefined
    const type = payload?.type === 'Inoperative' ? 'Inoperative' : payload?.type === 'Operative' ? 'Operative' : null
    if (connectorId === undefined || !type) return { status: 'Rejected' }

    const connectors = connectorId === 0
      ? (Array.from(state.connectorStates.keys()).length > 0
        ? Array.from(state.connectorStates.keys())
        : selectedDetail.value?.connectors.map((connector) => connector.connectorNumber) ?? [])
      : [connectorId]
    if (connectors.length === 0) return { status: 'Rejected' }
    let scheduled = false
    for (const id of connectors) {
      const cs = ensureConnectorState(state, id)
      cs.availabilityRequested = type
      if (type === 'Inoperative') {
        if (cs.status === 'Charging' || cs.status === 'Preparing' || cs.status === 'Finishing') {
          scheduled = true
        } else {
          cs.status = 'Unavailable'
          cs.cablePluggedIn = false
          sendStatusNotification(cpId, id, 'Unavailable', 'NoError')
        }
      } else if (cs.status === 'Unavailable') {
        cs.status = 'Available'
        sendStatusNotification(cpId, id, 'Available', 'NoError')
      }
    }
    appendRuntimeEvent(cpId, 'runtime.availability_changed', `ChangeAvailability ${type} ${scheduled ? 'scheduled' : 'applied'}`, connectorId || undefined, payload)
    return { status: scheduled ? 'Scheduled' : 'Accepted' }
  }

  function handleTriggerMessage(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown> | undefined) {
    const requestedMessage = typeof payload?.requestedMessage === 'string' ? payload.requestedMessage : ''
    const connectorId = typeof payload?.connectorId === 'number' ? payload.connectorId : firstConnectorIdInState(state)
    setTimeout(() => {
      if (!cpStates.value.has(cpId)) return
      switch (requestedMessage) {
        case 'BootNotification':
          sendBootNotification(cpId)
          break
        case 'Heartbeat':
          sendHeartbeat(cpId)
          break
        case 'StatusNotification':
          if (connectorId !== undefined) {
            const cs = ensureConnectorState(state, connectorId)
            sendStatusNotification(cpId, connectorId, cs.status, cs.faultCode ?? 'NoError')
          }
          break
        case 'MeterValues':
          if (connectorId !== undefined) sendConnectorMeterValuesInternal(cpId, state, connectorId)
          break
      }
    }, 0)
    return { status: requestedMessage ? 'Accepted' : 'Rejected' }
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
      case 'TransactionEvent':
        handleTransactionEventResponse(cpId, state, payload, pending)
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

    if (
      pending.action === 'Authorize' ||
      pending.action === 'StartTransaction' ||
      pending.action === 'TransactionEvent'
    ) {
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
      setRuntime(cpId, state)
      sendInitialStatusNotifications(cpId)
      scheduleHeartbeat(cpId)
    } else if (status === 'Pending') {
      state.registration = 'pending'
      setRuntime(cpId, state)
    } else if (status === 'Rejected') {
      state.registration = 'rejected'
      setRuntime(cpId, state)
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
    const frame = buildBootNotificationFrame(state.ocppVersion)
    sendFrame(cpId, state, frame, 'BootNotification')
  }

  function sendHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const frame = [2, uniqueId(), 'Heartbeat', {}]
    sendFrame(cpId, state, frame, 'Heartbeat')
  }

  async function sendInitialStatusNotifications(cpId: string) {
    try {
      const { connectors } = await api.fetchChargePoint(cpId)
      const state = cpStates.value.get(cpId)
      if (!state) return

      // Connector 0 is the main Charge Point controller
      sendStatusNotification(cpId, 0, 'Available', 'NoError')

      for (const c of connectors) {
        ensureConnectorState(state, c.connectorNumber)
        sendStatusNotification(cpId, c.connectorNumber, 'Available', 'NoError')
      }
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error(`[cp ${cpId}] failed to send initial status notifications:`, e)
    }
  }

  function sendStatusNotification(cpId: string, connectorId: number, status: string, errorCode: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    const frame = buildStatusNotificationFrame(state.ocppVersion, connectorId, status, errorCode)
    sendFrame(cpId, state, frame, 'StatusNotification')
  }

  function ensureConnectorState(state: ChargePointRuntime, connectorId: number): ConnectorState {
    let cs = state.connectorStates.get(connectorId)
    if (!cs) {
      cs = { status: 'Available', cablePluggedIn: false }
      state.connectorStates.set(connectorId, cs)
    }
    return cs
  }

  function firstConnectorIdInState(state: ChargePointRuntime): number | undefined {
    const fromRuntime = state.connectorStates.keys().next().value as number | undefined
    if (fromRuntime !== undefined) return fromRuntime
    return selectedDetail.value?.connectors[0]?.connectorNumber
  }

  function appendRuntimeEvent(
    cpId: string,
    type: string,
    message: string,
    connectorId?: number,
    payload?: unknown
  ) {
    realtimeStore.appendEvent({
      id: `${cpId}-${type}-${connectorId ?? 'cp'}-${Date.now()}-${Math.random().toString(36).slice(2)}`,
      type,
      chargePointId: cpId,
      connectorId,
      payload,
      message,
      timestamp: new Date().toISOString(),
    })
  }

  function sendAuthorize(cpId: string, state: ChargePointRuntime, connectorId: number, idTag: string, idTokenType = 'ISO14443') {
    const frame = buildAuthorizeFrame(state.ocppVersion, idTag, idTokenType)
    sendFrame(cpId, state, frame, 'Authorize', { connectorId, idTag, idTokenType })
  }

  // ─── Authorize / StartTransaction / StopTransaction / MeterValues ───

  function handleAuthorizeResponse(
    cpId: string,
    state: ChargePointRuntime,
    payload: Record<string, unknown>,
    pending: PendingCallInfo
  ) {
    // 1.6J: Authorize.conf carries { idTagInfo: { status } }.
    // 2.0.1: same shape — the spec keeps the field name.
    const idTagInfo = payload.idTagInfo as { status?: string } | undefined
    const status = idTagInfo?.status
    const connectorId = pending.connectorId
    const idTag = pending.idTag ?? 'DEADBEEF'
    if (connectorId === undefined) return
    const cs = state.connectorStates.get(connectorId)
    if (status !== 'Accepted') {
      if (cs) cs.status = 'Available'
      appendRuntimeEvent(cpId, 'transaction.authorization_rejected', `Authorization rejected for connector ${connectorId}`, connectorId, { idTag, status })
      error.value = `Authorize rejected: ${status ?? 'Unknown'}`
      return
    }

    // Authorization accepted. Branch on the CP's negotiated OCPP
    // version: 1.6J sends StartTransaction, 2.0.1 sends a
    // TransactionEvent(Started) carrying the CP-chosen GUID.
    const version = state.ocppVersion ?? '1.6J'
    state.meterCounter += 100
    const meterStart = state.meterCounter
    if (version === '2.0.1') {
      const guid = crypto.randomUUID()
      const tx: TransactionState = {
        transactionId: guid,
        seqNo: 0,
        connectorId,
        idTag,
        idTokenType: pending.idTokenType ?? 'ISO14443',
        meterStart,
        meterCurrent: meterStart,
        meterStop: null,
        startedAt: new Date().toISOString(),
        lastSampleAt: null,
        soc: 20,
        voltage: 400,
        current: 0,
        powerW: 0,
        stopReason: null,
        stopPending: false,
        status: 'preparing',
      }
      state.transactions.set(connectorId, tx)
      appendRuntimeEvent(cpId, 'transaction.authorized', `Authorization accepted for connector ${connectorId}`, connectorId, { idTag, transactionId: guid })
      sendTransactionEventStarted(cpId, state, tx, 'Authorized')
    } else {
      const tx: TransactionState = {
        transactionId: null,
        seqNo: null,
        connectorId,
        idTag,
        idTokenType: 'ISO14443',
        meterStart,
        meterCurrent: meterStart,
        meterStop: null,
        startedAt: new Date().toISOString(),
        lastSampleAt: null,
        soc: 20,
        voltage: 400,
        current: 0,
        powerW: 0,
        stopReason: null,
        stopPending: false,
        status: 'preparing',
      }
      state.transactions.set(connectorId, tx)
      appendRuntimeEvent(cpId, 'transaction.authorized', `Authorization accepted for connector ${connectorId}`, connectorId, { idTag })
      const frame = [2, uniqueId(), 'StartTransaction', {
        connectorId,
        idTag,
        meterStart,
        timestamp: new Date().toISOString(),
      }]
      sendFrame(cpId, state, frame, 'StartTransaction', { connectorId })
    }
  }

  function handleStartTransactionResponse(cpId: string, state: ChargePointRuntime, payload: Record<string, unknown>) {
    const idTagInfo = payload.idTagInfo as { status?: string } | undefined
    const status = idTagInfo?.status
    const numericId = payload.transactionId as number | undefined
    const pendingTx = Array.from(state.transactions.values()).find((tx) => tx.transactionId === null)
    if (!pendingTx) return
    const connectorId = pendingTx.connectorId
    const cs = state.connectorStates.get(connectorId)
    if (status === 'Accepted' && numericId !== undefined) {
      pendingTx.transactionId = String(numericId)
      pendingTx.status = 'charging'
      if (cs) cs.status = 'Charging'
      appendRuntimeEvent(cpId, 'transaction.started', `Transaction ${numericId} started on connector ${connectorId}`, connectorId, { transactionId: numericId })
      sendStatusNotification(cpId, connectorId, 'Charging', 'NoError')
      startMeterValuesTimer(cpId)
    } else {
      state.transactions.delete(connectorId)
      if (cs) cs.status = 'Available'
      appendRuntimeEvent(cpId, 'transaction.failed', `StartTransaction rejected for connector ${connectorId}`, connectorId, { status })
      error.value = `StartTransaction rejected: ${status ?? 'Unknown'}`
    }
  }

  function handleStopTransactionResponse(cpId: string, state: ChargePointRuntime, _payload: Record<string, unknown>) {
    const pendingTx = Array.from(state.transactions.values()).find((tx) => tx.stopPending)
    if (!pendingTx) return
    finishStoppedTransaction(cpId, state, pendingTx.connectorId)
  }

  // handleTransactionEventResponse closes the loop on a 2.0.1
  // TransactionEvent.conf. The spec allows idTokenInfo,
  // updatedPersonalMessage, chargingPriority in the body — we only
  // honor idTokenInfo.status. A non-Accepted response drops the
  // connector back to Available.
  function handleTransactionEventResponse(
    cpId: string,
    state: ChargePointRuntime,
    payload: Record<string, unknown>,
    pending: PendingCallInfo
  ) {
    const idTokenInfo = payload.idTokenInfo as { status?: string } | undefined
    const status = idTokenInfo?.status
    const connectorId = pending.connectorId
    if (connectorId === undefined) return
    const tx = state.transactions.get(connectorId)
    if (!tx) return

    if (status === 'Accepted') {
      // Started: we are now charging. The CSMS does not assign an
      // id; the CP-chosen GUID (tx.transactionId) is the wire id.
      if (tx.status === 'preparing') {
        tx.status = 'charging'
        const cs = state.connectorStates.get(connectorId)
        if (cs) cs.status = 'Charging'
        appendRuntimeEvent(cpId, 'transaction.started', `Transaction ${tx.transactionId} started on connector ${connectorId}`, connectorId, { transactionId: tx.transactionId })
        sendStatusNotification(cpId, connectorId, 'Charging', 'NoError')
        startMeterValuesTimer(cpId)
      }
      // Updated/Ended: stopPending flips to true on Ended; the
      // .conf arrives here and we just confirm — the timeout
      // watchdog would catch a real miss.
      if (tx.stopPending) {
        finishStoppedTransaction(cpId, state, connectorId)
      }
    } else {
      // Anything other than Accepted tears the transaction down.
      state.transactions.delete(connectorId)
      const cs = state.connectorStates.get(connectorId)
      if (cs) cs.status = 'Available'
      appendRuntimeEvent(cpId, 'transaction.failed', `TransactionEvent rejected for connector ${connectorId}`, connectorId, { status })
      error.value = `TransactionEvent rejected: ${status ?? 'Unknown'}`
    }
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
    advanceDcMeter(tx, 10)
    if (state.ocppVersion === '2.0.1') {
      // 2.0.1 preferred path: TransactionEvent(Updated) carrying
      // the meter sample inside the event, with explicit
      // triggerReason and per-transaction seqNo.
      const frame = buildTransactionEventUpdatedFrame(state.ocppVersion, tx, 10)
      appendRuntimeEvent(cpId, 'meter_value.generated', `MeterValues (TransactionEvent Updated) for connector ${connectorId}`, connectorId, {
        transactionId: tx.transactionId,
        seqNo: tx.seqNo,
        powerW: tx.powerW,
        soc: tx.soc,
        meterCurrent: tx.meterCurrent,
      })
      sendFrame(cpId, state, frame, 'TransactionEvent', { connectorId })
    } else {
      const frame = [2, uniqueId(), 'MeterValues', {
        connectorId,
        transactionId: tx.transactionId === null ? undefined : Number(tx.transactionId),
        meterValue: [
          {
            timestamp: new Date().toISOString(),
            sampledValue: [
              { value: String(tx.meterCurrent), measurand: 'Energy.Active.Import.Register', unit: 'Wh', context: 'Sample.Periodic' },
              { value: String(tx.powerW), measurand: 'Power.Active.Import', unit: 'W', context: 'Sample.Periodic' },
              { value: tx.current.toFixed(1), measurand: 'Current.Import', unit: 'A', context: 'Sample.Periodic' },
              { value: tx.voltage.toFixed(1), measurand: 'Voltage', unit: 'V', context: 'Sample.Periodic' },
              { value: String(tx.soc), measurand: 'SoC', unit: 'Percent', context: 'Sample.Periodic' },
            ],
          },
        ],
      }]
      appendRuntimeEvent(cpId, 'meter_value.generated', `MeterValues generated for connector ${connectorId}`, connectorId, {
        transactionId: tx.transactionId,
        powerW: tx.powerW,
        soc: tx.soc,
        meterCurrent: tx.meterCurrent,
      })
      sendFrame(cpId, state, frame, 'MeterValues')
    }
  }

  function advanceDcMeter(tx: TransactionState, elapsedSeconds: number) {
    const nextSoc = Math.min(95, tx.soc + 1)
    const targetPowerW = nextSoc < 30
      ? 45000 + (nextSoc - 20) * 4500
      : nextSoc < 80
        ? 90000
        : Math.max(18000, 90000 - (nextSoc - 80) * 4800)
    tx.soc = nextSoc
    tx.powerW = Math.round(targetPowerW)
    tx.voltage = Math.round((390 + tx.soc * 1.2) * 10) / 10
    tx.current = Math.round((tx.powerW / tx.voltage) * 10) / 10
    tx.meterCurrent += Math.max(1, Math.round(tx.powerW * elapsedSeconds / 3600))
    tx.lastSampleAt = new Date().toISOString()
  }

  // ─── Wire send helper ────────────────────────────────────────────────

  function sendFrame(
    cpId: string,
    state: ChargePointRuntime,
    frame: unknown[],
    action: string,
    extra?: { connectorId?: number; idTag?: string; idTokenType?: string }
  ) {
    if (!state.client.send(frame)) {
      // eslint-disable-next-line no-console
      console.warn('[ocpp] send dropped: socket not open', { cpId, action })
      return
    }
    appendOcppLog(cpId, 'out', frame, action)
    const uid = frame[1] as string
    state.pendingCalls.set(uid, { action, sentAt: Date.now(), ...extra })
    state.lastMessageSentAt = Date.now()
    resetHeartbeatTimer(cpId)
  }

  function sendResponseFrame(cpId: string, state: ChargePointRuntime, frame: unknown[], action?: string) {
    if (!state.client.send(frame)) {
      // eslint-disable-next-line no-console
      console.warn('[ocpp] response send dropped: socket not open', { cpId })
      return
    }
    appendOcppLog(cpId, 'out', frame, action)
    state.lastMessageSentAt = Date.now()
  }

  function appendOcppLog(cpId: string, direction: 'in' | 'out', frame: unknown[], actionOverride?: string) {
    const messageType = frame[0] as number | undefined
    const uniqueIdValue = typeof frame[1] === 'string' ? frame[1] as string : ''
    const action = actionOverride ?? actionFromFrame(cpId, frame)
    realtimeStore.appendEvent({
      id: `${cpId}-${direction}-${uniqueIdValue || Date.now()}-${Math.random().toString(36).slice(2)}`,
      type: typeFromFrame(messageType, direction),
      chargePointId: cpId,
      direction,
      action,
      rawFrame: frame,
      message: messageFromFrame(messageType, direction, action, uniqueIdValue),
      timestamp: new Date().toISOString(),
    })
  }

  function actionFromFrame(cpId: string, frame: unknown[]): string | undefined {
    if (frame[0] === 2 && typeof frame[2] === 'string') return frame[2]
    if ((frame[0] === 3 || frame[0] === 4) && typeof frame[1] === 'string') {
      return cpStates.value.get(cpId)?.pendingCalls.get(frame[1])?.action
    }
    return undefined
  }

  function typeFromFrame(messageType: number | undefined, direction: 'in' | 'out'): string {
    if (messageType === 2) return direction === 'out' ? 'ocpp.call.sent' : 'ocpp.call.received'
    if (messageType === 3) return direction === 'out' ? 'ocpp.call_result.sent' : 'ocpp.call_result.received'
    if (messageType === 4) return direction === 'out' ? 'ocpp.call_error.sent' : 'ocpp.call_error.received'
    return direction === 'out' ? 'ocpp.frame.sent' : 'ocpp.frame.received'
  }

  function messageFromFrame(
    messageType: number | undefined,
    direction: 'in' | 'out',
    action: string | undefined,
    uniqueIdValue: string
  ): string {
    const label = action ?? `uid ${uniqueIdValue || 'unknown'}`
    if (messageType === 2) return `${direction === 'out' ? 'Sent' : 'Received'} ${label}`
    if (messageType === 3) return `${direction === 'out' ? 'Sent' : 'Received'} ${label} response`
    if (messageType === 4) return `${direction === 'out' ? 'Sent' : 'Received'} ${label} error`
    return `${direction === 'out' ? 'Sent' : 'Received'} raw frame`
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

  function stopConnectorTransactionInternal(
    cpId: string,
    state: ChargePointRuntime,
    connectorId: number,
    reason: StopReason
  ): boolean {
    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null || tx.stopPending) return false
    if (!state.client.isOpen) return false

    tx.status = 'finishing'
    tx.stopReason = reason
    tx.stopPending = true
    advanceDcMeter(tx, 5)
    tx.meterStop = tx.meterCurrent
    const cs = ensureConnectorState(state, connectorId)
    cs.status = 'Finishing'
    setRuntime(cpId, state)
    sendStatusNotification(cpId, connectorId, 'Finishing', 'NoError')
    appendRuntimeEvent(cpId, 'transaction.stopping', `Stopping transaction ${tx.transactionId} (${reason})`, connectorId, {
      transactionId: tx.transactionId,
      reason,
    })

    if (state.ocppVersion === '2.0.1') {
      // 2.0.1: one TransactionEvent(Ended) carries the stop
      // reason, transactionId (string), seqNo++, and the
      // Transaction.End meter sample. The unified flow replaces
      // the separate StopTransaction call.
      const frame = buildTransactionEventEndedFrame(state.ocppVersion, tx, reason)
      sendFrame(cpId, state, frame, 'TransactionEvent', { connectorId })
    } else {
      const frame = [2, uniqueId(), 'StopTransaction', {
        transactionId: Number(tx.transactionId),
        idTag: tx.idTag,
        meterStop: tx.meterStop,
        timestamp: new Date().toISOString(),
        reason,
        transactionData: [
          {
            timestamp: new Date().toISOString(),
            sampledValue: [
              { value: String(tx.meterStop), measurand: 'Energy.Active.Import.Register', unit: 'Wh', context: 'Transaction.End' },
              { value: String(tx.soc), measurand: 'SoC', unit: 'Percent', context: 'Transaction.End' },
            ],
          },
        ],
      }]
      sendFrame(cpId, state, frame, 'StopTransaction', { connectorId })
    }
    setTimeout(() => {
      const current = cpStates.value.get(cpId)
      const pending = current?.transactions.get(connectorId)
      if (current && pending?.stopPending) {
        appendRuntimeEvent(cpId, 'runtime.response_timeout', `Stop response timeout for connector ${connectorId}`, connectorId, {
          transactionId: pending.transactionId,
        })
        finishStoppedTransaction(cpId, current, connectorId)
      }
    }, 15000)
    return true
  }

  function finishStoppedTransaction(cpId: string, state: ChargePointRuntime, connectorId: number) {
    const tx = state.transactions.get(connectorId)
    if (!tx) return
    state.transactions.delete(connectorId)
    if (!Array.from(state.transactions.values()).some((other) => other.status === 'charging')) {
      if (state.meterValuesTimer) {
        clearInterval(state.meterValuesTimer)
        state.meterValuesTimer = null
      }
    }
    appendRuntimeEvent(cpId, 'transaction.finished', `Transaction ${tx.transactionId ?? 'pending'} finished`, connectorId, {
      transactionId: tx.transactionId,
      reason: tx.stopReason,
      meterStop: tx.meterStop,
    })
    const cs = ensureConnectorState(state, connectorId)
    if (cs.availabilityRequested === 'Inoperative') {
      cs.status = 'Unavailable'
      cs.cablePluggedIn = false
      setRuntime(cpId, state)
      sendStatusNotification(cpId, connectorId, 'Unavailable', 'NoError')
    }
  }

  function sendConnectorMeterValuesInternal(cpId: string, state: ChargePointRuntime, connectorId: number): boolean {
    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null || tx.stopPending) return false
    sendMeterValuesForTransaction(cpId, connectorId, tx)
    return true
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
    const cs = ensureConnectorState(state, connectorId)
    if (cs.status !== 'Available') {
      error.value = `Cannot plug in: connector is ${cs.status}`
      return
    }
    cs.cablePluggedIn = true
    cs.status = 'Preparing'
    appendRuntimeEvent(cpId, 'connector.cable_plugged', `Cable plugged on connector ${connectorId}`, connectorId)
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
    sendAuthorize(cpId, state, connectorId, idTag)
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
      cs.pendingRemoteStartIdTag = undefined
      appendRuntimeEvent(cpId, 'connector.cable_unplugged', `Cable unplugged on connector ${connectorId}`, connectorId)
      sendStatusNotification(cpId, connectorId, 'Available', 'NoError')
    } else if (cs.status === 'Charging') {
      appendRuntimeEvent(cpId, 'connector.ev_disconnected', `EV disconnected on connector ${connectorId}`, connectorId)
      stopConnectorTransactionInternal(cpId, state, connectorId, 'EVDisconnected')
    } else {
      error.value = `Cannot unplug: connector is ${cs.status}`
    }
  }

  function stopConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    if (!stopConnectorTransactionInternal(cpId, state, connectorId, 'Local')) {
      error.value = 'No active transaction'
    }
  }

  function sendConnectorMeterValues(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.client.isOpen) return
    if (!sendConnectorMeterValuesInternal(cpId, state, connectorId)) {
      error.value = 'No active transaction'
    }
  }

  function setConnectorStatus(connectorId: number, status: string) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return
    const cs = ensureConnectorState(state, connectorId)
    if (status === 'Faulted') {
      cs.faultCode = 'OtherError'
      const tx = state.transactions.get(connectorId)
      if (tx?.transactionId !== null && tx && !tx.stopPending) {
        stopConnectorTransactionInternal(cpId, state, connectorId, 'EmergencyStop')
      }
    } else if (status === 'SuspendedEV' || status === 'SuspendedEVSE') {
      const tx = state.transactions.get(connectorId)
      if (tx) tx.status = 'charging'
    } else if (status === 'Available') {
      cs.faultCode = undefined
    }
    cs.status = status as ConnectorStatus
    if (status === 'Available') cs.cablePluggedIn = false
    sendStatusNotification(cpId, connectorId, status, cs.faultCode ?? 'NoError')
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

  // ─── Version-aware wire builders ──────────────────────────────────
  //
  // Each builder returns a raw [2, uid, action, payload] frame for
  // the negotiated OCPP version. 1.6J uses the spec-exact flat
  // shapes (chargePointVendor/chargePointModel, errorCode in
  // StatusNotification, numeric transactionId). 2.0.1 wraps the
  // boot fields in {chargingStation:{...}}, drops errorCode from
  // StatusNotification, and routes transactions through
  // TransactionEvent with a per-transaction seqNo.

  function buildBootNotificationFrame(version: string): unknown[] {
    if (version === '2.0.1') {
      return [
        2,
        uniqueId(),
        'BootNotification',
        {
          reason: 'PowerUp',
          chargingStation: {
            model: 'OCPP-Sim',
            vendorName: 'Simulator',
          },
        },
      ]
    }
    return [2, uniqueId(), 'BootNotification', { chargePointVendor: 'Simulator', chargePointModel: 'OCPP-Sim' }]
  }

  function buildStatusNotificationFrame(
    version: string,
    connectorId: number,
    status: string,
    errorCode: string
  ): unknown[] {
    const timestamp = new Date().toISOString()
    if (version === '2.0.1') {
      return [
        2,
        uniqueId(),
        'StatusNotification',
        {
          timestamp,
          connectorStatus: status,
          evseId: connectorId,
          connectorId: 1,
        },
      ]
    }
    return [
      2,
      uniqueId(),
      'StatusNotification',
      { connectorId, errorCode, status, timestamp },
    ]
  }

  function buildAuthorizeFrame(
    version: string,
    idTag: string,
    idTokenType = 'ISO14443'
  ): unknown[] {
    if (version === '2.0.1') {
      return [
        2,
        uniqueId(),
        'Authorize',
        { idToken: { idToken: idTag, type: idTokenType } },
      ]
    }
    return [2, uniqueId(), 'Authorize', { idTag }]
  }

  // sendTransactionEventStarted fires the 2.0.1 unified
  // TransactionEvent.req for the Started phase. The seqNo is
  // recorded on the transaction state for the next event.
  function sendTransactionEventStarted(
    cpId: string,
    state: ChargePointRuntime,
    tx: TransactionState,
    triggerReason: string
  ) {
    if (state.ocppVersion !== '2.0.1') return
    const idToken = { idToken: tx.idTag, type: tx.idTokenType || 'ISO14443' }
    const frame = [
      2,
      uniqueId(),
      'TransactionEvent',
      {
        eventType: 'Started',
        timestamp: new Date().toISOString(),
        triggerReason,
        seqNo: tx.seqNo ?? 0,
        transactionInfo: { transactionId: tx.transactionId ?? crypto.randomUUID() },
        evse: { id: tx.connectorId, connectorId: 1 },
        idToken,
      },
    ]
    sendFrame(cpId, state, frame, 'TransactionEvent', { connectorId: tx.connectorId })
  }

  // buildTransactionEventUpdatedFrame is the per-tick meter
  // sample for 2.0.1. Advances seqNo on the local transaction
  // state so the spec's monotonic-counter invariant holds.
  function buildTransactionEventUpdatedFrame(
    _version: string,
    tx: TransactionState,
    _elapsedSeconds: number
  ): unknown[] {
    if (tx.seqNo !== null) tx.seqNo = tx.seqNo + 1
    const nextSeq = tx.seqNo ?? 0
    return [
      2,
      uniqueId(),
      'TransactionEvent',
      {
        eventType: 'Updated',
        timestamp: new Date().toISOString(),
        triggerReason: 'MeterValuePeriodic',
        seqNo: nextSeq,
        transactionInfo: { transactionId: tx.transactionId ?? '' },
        evse: { id: tx.connectorId, connectorId: 1 },
        meterValue: [
          {
            timestamp: new Date().toISOString(),
            sampledValue: [
              { value: String(tx.meterCurrent), measurand: 'Energy.Active.Import.Register', unit: 'Wh' },
              { value: String(tx.powerW), measurand: 'Power.Active.Import', unit: 'W' },
              { value: tx.current.toFixed(1), measurand: 'Current.Import', unit: 'A' },
              { value: tx.voltage.toFixed(1), measurand: 'Voltage', unit: 'V' },
              { value: String(tx.soc), measurand: 'SoC', unit: 'Percent' },
            ],
          },
        ],
      },
    ]
  }

  // buildTransactionEventEndedFrame is the 2.0.1 replacement
  // for the 1.6J StopTransaction. The stop reason from the local
  // enum is mapped onto the spec enum (Local, Remote, ...).
  function buildTransactionEventEndedFrame(
    _version: string,
    tx: TransactionState,
    reason: StopReason
  ): unknown[] {
    if (tx.seqNo !== null) tx.seqNo = tx.seqNo + 1
    return [
      2,
      uniqueId(),
      'TransactionEvent',
      {
        eventType: 'Ended',
        timestamp: new Date().toISOString(),
        triggerReason: reason === 'Remote' ? 'RemoteStop' : 'StopAuthorized',
        seqNo: tx.seqNo ?? 0,
        transactionInfo: {
          transactionId: tx.transactionId ?? '',
          stoppedReason: mapStopReasonToV201(reason),
        },
        evse: { id: tx.connectorId, connectorId: 1 },
        meterValue: [
          {
            timestamp: new Date().toISOString(),
            sampledValue: [
              { value: String(tx.meterStop ?? tx.meterCurrent), measurand: 'Energy.Active.Import.Register', unit: 'Wh' },
              { value: String(tx.soc), measurand: 'SoC', unit: 'Percent' },
            ],
          },
        ],
      },
    ]
  }

  function mapStopReasonToV201(reason: StopReason): string {
    switch (reason) {
      case 'Local': return 'Local'
      case 'Remote': return 'Remote'
      case 'EVDisconnected': return 'EVDisconnected'
      case 'EmergencyStop': return 'EmergencyStop'
      case 'PowerLoss': return 'PowerLoss'
      case 'Reboot':
      case 'SoftReset':
      case 'HardReset': return 'Reboot'
      case 'UnlockCommand': return 'UnlockCommand'
      default: return 'Other'
    }
  }

  return {
    chargePoints,
    chargePointsWithStatus,
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
    updateChargePoint,
    removeChargePoint,
    addConnector,
    removeConnector,
    connectChargePoint,
    disconnectChargePoint,
    connectAll,
    disconnectAll,
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
    getConnectionStatus,
    activeTransactionFor,
  }
})
