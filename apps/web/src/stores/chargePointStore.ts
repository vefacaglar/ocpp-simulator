import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api/chargePointsApi'
import type { ChargePoint, Connector } from '../api/chargePointsApi'

type RegistrationState = 'disconnected' | 'pending' | 'accepted' | 'rejected'

type ConnectorStatus = 'Available' | 'Preparing' | 'Charging' | 'SuspendedEV' | 'SuspendedEVSE' | 'Finishing' | 'Faulted' | 'Unavailable'

interface ConnectorRuntimeState {
  status: ConnectorStatus
  cablePluggedIn: boolean
}

interface TransactionState {
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

interface ChargePointState {
  ws: WebSocket
  registration: RegistrationState
  heartbeatInterval: number | null
  heartbeatTimer: ReturnType<typeof setTimeout> | null
  lastMessageSentAt: number
  pendingCalls: Map<string, PendingCallInfo>
  connectorStates: Map<number, ConnectorRuntimeState>
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
  const cpStates = ref<Map<string, ChargePointState>>(new Map())

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
    const state = cpStates.value.get(selectedId.value)
    return state?.connectorStates.get(connectorId)?.status ?? 'Available'
  }

  function isCablePluggedIn(connectorId: number): boolean {
    if (!selectedId.value) return false
    const state = cpStates.value.get(selectedId.value)
    return state?.connectorStates.get(connectorId)?.cablePluggedIn ?? false
  }

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

  function generateUniqueId(): string {
    return crypto.randomUUID().replace(/-/g, '').substring(0, 32)
  }

  function connectChargePoint() {
    if (!selectedId.value) return
    const cpId = selectedId.value

    if (cpStates.value.has(cpId)) {
      return
    }

    const ws = api.connectChargePointWS(cpId)

    ws.onopen = () => {
      const state: ChargePointState = {
        ws,
        registration: 'pending',
        heartbeatInterval: null,
        heartbeatTimer: null,
        lastMessageSentAt: Date.now(),
        pendingCalls: new Map(),
        connectorStates: new Map(),
        transactions: new Map(),
        meterCounter: 0,
        meterValuesTimer: null,
      }
      cpStates.value.set(cpId, state)

      const uniqueId = generateUniqueId()
      const frame = [2, uniqueId, "BootNotification", {
        chargePointVendor: "Simulator",
        chargePointModel: "OCPP-Sim"
      }]
      sendMessage(cpId, frame, "BootNotification")

      loadChargePoints()
      selectChargePoint(cpId)
    }

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data)
        console.log('[WS] Received:', data)
        if (Array.isArray(data)) {
          handleOCPPFrame(cpId, data)
        }
      } catch (e) {
        console.error('[WS] Parse error:', e)
      }
    }

    ws.onclose = () => {
      const state = cpStates.value.get(cpId)
      if (state?.heartbeatTimer) {
        clearInterval(state.heartbeatTimer)
      }
      cpStates.value.delete(cpId)
      loadChargePoints()
      selectChargePoint(cpId)
    }

    ws.onerror = () => {
      cpStates.value.delete(cpId)
      error.value = `Connection failed for ${cpId}`
    }
  }

  function handleOCPPFrame(cpId: string, frame: unknown[]) {
    const state = cpStates.value.get(cpId)
    if (!state) {
      console.log('[OCPP] No state for', cpId)
      return
    }

    const typeID = frame[0] as number
    console.log('[OCPP] Frame type:', typeID, 'uniqueId:', frame[1])

    if (typeID === 3) {
      const uniqueId = frame[1] as string
      const payload = frame[2] as Record<string, unknown>
      const pending = state.pendingCalls.get(uniqueId)

      console.log('[OCPP] CALLRESULT uniqueId:', uniqueId, 'pending:', pending, 'payload:', payload)

      if (pending) {
        state.pendingCalls.delete(uniqueId)

        if (pending.action === "BootNotification") {
          console.log('[OCPP] Handling BootNotification response')
          handleBootNotificationResponse(cpId, state, payload)
        } else if (pending.action === "Authorize") {
          console.log('[OCPP] Handling Authorize response')
          handleAuthorizeResponse(cpId, state, payload, pending)
        } else if (pending.action === "StartTransaction") {
          console.log('[OCPP] Handling StartTransaction response')
          handleStartTransactionResponse(cpId, state, payload)
        } else if (pending.action === "StopTransaction") {
          console.log('[OCPP] Handling StopTransaction response')
          handleStopTransactionResponse(cpId, state, payload)
        }
      } else {
        console.log('[OCPP] No pending call for uniqueId:', uniqueId)
      }
    } else if (typeID === 4) {
      const uniqueId = frame[1] as string
      const pending = state.pendingCalls.get(uniqueId)
      state.pendingCalls.delete(uniqueId)

      if (pending?.action === "Authorize") {
        const connectorId = pending.connectorId
        if (connectorId !== undefined) {
          const cs = state.connectorStates.get(connectorId)
          if (cs) {
            cs.status = 'Preparing'
          }
        }
        error.value = `Authorize CALLERROR for connector ${connectorId}`
      }

      console.log('[OCPP] CALLERROR uniqueId:', uniqueId)
    }
  }

  function handleBootNotificationResponse(cpId: string, state: ChargePointState, payload: Record<string, unknown>) {
    const status = payload.status as string
    const interval = payload.interval as number

    console.log('[Boot] Response:', { status, interval })

    if (status === "Accepted") {
      state.registration = 'accepted'
      state.heartbeatInterval = interval

      console.log('[Boot] Accepted, starting heartbeat with interval:', interval)
      sendInitialStatusNotifications(cpId)
      scheduleHeartbeat(cpId)

      loadChargePoints()
      selectChargePoint(cpId)
    } else if (status === "Pending") {
      state.registration = 'pending'
    } else if (status === "Rejected") {
      state.registration = 'rejected'

      setTimeout(() => {
        if (cpStates.value.has(cpId)) {
          const uniqueId = generateUniqueId()
          const frame = [2, uniqueId, "BootNotification", {
            chargePointVendor: "Simulator",
            chargePointModel: "OCPP-Sim"
          }]
          sendMessage(cpId, frame, "BootNotification")
        }
      }, interval * 1000)
    }
  }

  function sendInitialStatusNotifications(cpId: string) {
    const detail = selectedDetail.value
    if (!detail) return

    for (const connector of detail.connectors) {
      sendStatusNotification(cpId, connector.connectorNumber, "Available", "NoError")
    }
  }

  function sendStatusNotification(cpId: string, connectorId: number, status: string, errorCode: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StatusNotification", {
      connectorId,
      errorCode,
      status,
      timestamp: new Date().toISOString()
    }]
    sendMessage(cpId, frame, "StatusNotification")
  }

  // ─── Authorize Response Handling ────────────────────────────────────────

  function handleAuthorizeResponse(cpId: string, state: ChargePointState, payload: Record<string, unknown>, pending: PendingCallInfo) {
    const idTagInfo = payload.idTagInfo as Record<string, unknown> | undefined
    const status = idTagInfo?.status as string | undefined
    const connectorId = pending.connectorId
    const idTag = pending.idTag ?? 'DEADBEEF'

    if (connectorId === undefined) {
      console.error('[Authorize] No connectorId in pending call')
      return
    }

    const cs = state.connectorStates.get(connectorId)

    if (status === 'Accepted') {
      console.log('[Authorize] Accepted for connector', connectorId)

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

      const uniqueId = generateUniqueId()
      const frame = [2, uniqueId, "StartTransaction", {
        connectorId,
        idTag,
        meterStart,
        timestamp: new Date().toISOString()
      }]
      sendMessage(cpId, frame, "StartTransaction", { connectorId })
    } else {
      console.log('[Authorize] Rejected for connector', connectorId, 'status:', status)
      if (cs) {
        cs.status = 'Preparing'
      }
      error.value = `Authorize rejected: ${status ?? 'Unknown'}`
    }
  }

  // ─── StartTransaction Response Handling ─────────────────────────────────

  function handleStartTransactionResponse(cpId: string, state: ChargePointState, payload: Record<string, unknown>) {
    const idTagInfo = payload.idTagInfo as Record<string, unknown> | undefined
    const txStatus = idTagInfo?.status as string | undefined
    const transactionId = payload.transactionId as number

    const pendingTx = Array.from(state.transactions.values()).find(tx => tx.transactionId === null)
    if (!pendingTx) {
      console.log('[StartTransaction] No pending transaction found')
      return
    }

    const connectorId = pendingTx.connectorId
    const cs = state.connectorStates.get(connectorId)

    if (txStatus === 'Accepted' && transactionId) {
      pendingTx.transactionId = transactionId
      pendingTx.status = 'charging'

      if (cs) {
        cs.status = 'Charging'
      }

      sendStatusNotification(cpId, connectorId, "Charging", "NoError")
      startMeterValuesTimer(cpId)
    } else {
      console.log('[StartTransaction] Rejected:', txStatus)
      state.transactions.delete(connectorId)
      if (cs) {
        cs.status = 'Preparing'
      }
      error.value = `StartTransaction rejected: ${txStatus ?? 'Unknown'}`
    }
  }

  // ─── StopTransaction Response Handling ──────────────────────────────────

  function handleStopTransactionResponse(cpId: string, state: ChargePointState, _payload: Record<string, unknown>) {
    console.log('[StopTransaction] Response received')
    // StopTransaction.conf is informational; the main cleanup happens in stopConnectorTransaction
  }

  // ─── Meter Values ───────────────────────────────────────────────────────

  function startMeterValuesTimer(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state) return

    if (state.meterValuesTimer) {
      clearInterval(state.meterValuesTimer)
    }

    state.meterValuesTimer = setInterval(() => {
      for (const [connectorId, tx] of state.transactions) {
        if (tx.status === 'charging' && tx.transactionId !== null) {
          sendMeterValuesForTransaction(cpId, connectorId, tx)
        }
      }
    }, 10000)
  }

  function stopMeterValuesTimer(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (state?.meterValuesTimer) {
      clearInterval(state.meterValuesTimer)
      state.meterValuesTimer = null
    }
  }

  function sendMeterValuesForTransaction(cpId: string, connectorId: number, tx: TransactionState) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return

    state.meterCounter += 100
    tx.meterCurrent = state.meterCounter

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "MeterValues", {
      connectorId,
      transactionId: tx.transactionId,
      meterValue: [{
        timestamp: new Date().toISOString(),
        sampledValue: [
          {
            value: String(tx.meterCurrent),
            measurand: "Energy.Active.Import.Register",
            unit: "Wh",
            context: "Sample.Periodic"
          },
          {
            value: "7360",
            measurand: "Power.Active.Import",
            unit: "W"
          }
        ]
      }]
    }]
    sendMessage(cpId, frame, "MeterValues")
  }

  // ─── Message Sending ────────────────────────────────────────────────────

  function sendMessage(cpId: string, frame: unknown[], action: string, extra?: { connectorId?: number; idTag?: string }) {
    const state = cpStates.value.get(cpId)
    if (!state || state.ws.readyState !== WebSocket.OPEN) return

    const uniqueId = frame[1] as string
    const pendingInfo: PendingCallInfo = { action, sentAt: Date.now(), ...extra }
    state.pendingCalls.set(uniqueId, pendingInfo)
    state.lastMessageSentAt = Date.now()
    state.ws.send(JSON.stringify(frame))

    resetHeartbeatTimer(cpId)
  }

  function scheduleHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.heartbeatInterval) {
      console.log('[Heartbeat] Cannot schedule:', { registration: state?.registration, interval: state?.heartbeatInterval })
      return
    }

    if (state.heartbeatTimer) {
      clearTimeout(state.heartbeatTimer)
      state.heartbeatTimer = null
    }

    const delay = state.heartbeatInterval * 1000
    console.log('[Heartbeat] Scheduling in', delay, 'ms')
    state.heartbeatTimer = setTimeout(() => {
      const timeSinceLastMessage = Date.now() - state.lastMessageSentAt
      console.log('[Heartbeat] Timer fired, timeSinceLastMessage:', timeSinceLastMessage, 'delay:', delay)
      if (timeSinceLastMessage >= delay) {
        console.log('[Heartbeat] Sending heartbeat')
        sendHeartbeat(cpId)
      }
      scheduleHeartbeat(cpId)
    }, delay)
  }

  function resetHeartbeatTimer(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return

    scheduleHeartbeat(cpId)
  }

  function sendHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') return

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "Heartbeat", {}]
    sendMessage(cpId, frame, "Heartbeat")
  }

  function disconnectChargePoint() {
    if (!selectedId.value) return
    disconnectWS(selectedId.value)
  }

  function disconnectWS(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (state) {
      if (state.heartbeatTimer) {
        clearTimeout(state.heartbeatTimer)
      }
      if (state.meterValuesTimer) {
        clearInterval(state.meterValuesTimer)
      }
      state.ws.close()
      cpStates.value.delete(cpId)
    }
  }

  function sendOCPPFrame(frame: unknown[]) {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered. Send BootNotification first.'
      return
    }
    const action = frame[2] as string
    sendMessage(selectedId.value, frame, action)
  }

  function bootChargePoint() {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "BootNotification", {
      chargePointVendor: "Simulator",
      chargePointModel: "OCPP-Sim"
    }]
    sendMessage(selectedId.value, frame, "BootNotification")
  }

  function heartbeatChargePoint() {
    if (!selectedId.value) return
    sendHeartbeat(selectedId.value)
  }

  // ─── Connector Flow: Plug In ────────────────────────────────────────────

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

    sendStatusNotification(cpId, connectorId, "Preparing", "NoError")
  }

  // ─── Connector Flow: Authorize ──────────────────────────────────────────

  function authorizeConnector(connectorId: number, idTag: string) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }

    const cs = state.connectorStates.get(connectorId)
    if (!cs || cs.status !== 'Preparing') {
      error.value = `Cannot authorize: connector is ${cs?.status ?? 'Unknown'}`
      return
    }

    if (!cs.cablePluggedIn) {
      error.value = 'Cable must be plugged in first'
      return
    }

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "Authorize", {
      idTag
    }]
    sendMessage(cpId, frame, "Authorize", { connectorId, idTag })
  }

  // ─── Connector Flow: Unplug ─────────────────────────────────────────────

  function unplugConnector(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }

    const cs = state.connectorStates.get(connectorId)
    if (!cs) {
      error.value = 'Connector state not found'
      return
    }

    if (cs.status === 'Preparing') {
      cs.cablePluggedIn = false
      cs.status = 'Available'
      sendStatusNotification(cpId, connectorId, "Available", "NoError")
    } else if (cs.status === 'Finishing') {
      cs.cablePluggedIn = false
      cs.status = 'Available'
      sendStatusNotification(cpId, connectorId, "Available", "NoError")
    } else if (cs.status === 'Charging') {
      error.value = 'Stop the transaction before unplugging'
      return
    } else {
      error.value = `Cannot unplug: connector is ${cs.status}`
      return
    }
  }

  // ─── Connector Flow: Stop Transaction ───────────────────────────────────

  function stopConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }

    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null) {
      error.value = 'No active transaction'
      return
    }

    const cs = state.connectorStates.get(connectorId)

    stopMeterValuesTimer(cpId)

    if (cs) {
      cs.status = 'Finishing'
    }
    sendStatusNotification(cpId, connectorId, "Finishing", "NoError")

    state.meterCounter += 50
    const meterStop = state.meterCounter

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StopTransaction", {
      transactionId: tx.transactionId,
      meterStop,
      timestamp: new Date().toISOString(),
      reason: "Local",
      transactionData: [{
        timestamp: new Date().toISOString(),
        sampledValue: [{
          value: String(meterStop),
          measurand: "Energy.Active.Import.Register",
          unit: "Wh",
          context: "Transaction.End"
        }]
      }]
    }]
    sendMessage(cpId, frame, "StopTransaction")

    state.transactions.delete(connectorId)
  }

  // ─── Connector Flow: Send MeterValues (manual) ──────────────────────────

  function sendConnectorMeterValues(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }

    const tx = state.transactions.get(connectorId)
    if (!tx || tx.transactionId === null) {
      error.value = 'No active transaction'
      return
    }

    state.meterCounter += 10
    tx.meterCurrent = state.meterCounter

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "MeterValues", {
      connectorId,
      transactionId: tx.transactionId,
      meterValue: [{
        timestamp: new Date().toISOString(),
        sampledValue: [
          {
            value: String(tx.meterCurrent),
            measurand: "Energy.Active.Import.Register",
            unit: "Wh",
            context: "Sample.Periodic"
          },
          {
            value: "7360",
            measurand: "Power.Active.Import",
            unit: "W"
          }
        ]
      }]
    }]
    sendMessage(cpId, frame, "MeterValues")
  }

  // ─── Connector Flow: Set Status (Fault, Disable, Enable, Clear) ─────────

  function setConnectorStatus(connectorId: number, status: string) {
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

    cs.status = status as ConnectorStatus
    if (status === 'Available') {
      cs.cablePluggedIn = false
    }

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StatusNotification", {
      connectorId,
      errorCode: "NoError",
      status,
      timestamp: new Date().toISOString()
    }]
    sendMessage(cpId, frame, "StatusNotification")
  }

  // ─── Legacy: Start Transaction (kept for backward compatibility) ────────

  function startConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const cpId = selectedId.value
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }

    sendStatusNotification(cpId, connectorId, "Preparing", "NoError")

    state.meterCounter += 100
    const meterStart = state.meterCounter

    const tx: TransactionState = {
      transactionId: null,
      connectorId,
      idTag: "DEADBEEF",
      meterStart,
      meterCurrent: meterStart,
      status: 'preparing',
    }
    state.transactions.set(connectorId, tx)

    let cs = state.connectorStates.get(connectorId)
    if (!cs) {
      cs = { status: 'Available', cablePluggedIn: false }
      state.connectorStates.set(connectorId, cs)
    }
    cs.status = 'Preparing'
    cs.cablePluggedIn = true

    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StartTransaction", {
      connectorId,
      idTag: tx.idTag,
      meterStart,
      timestamp: new Date().toISOString()
    }]
    sendMessage(cpId, frame, "StartTransaction", { connectorId })
  }

  // ─── Simulate Remote Start (dev tool) ───────────────────────────────────

  async function simulateRemoteStart(idTag: string = 'DEADBEEF', connectorId?: number) {
    if (!selectedId.value) return
    try {
      await api.remoteStart(selectedId.value, idTag, connectorId)
    } catch (e: any) {
      error.value = e.message
    }
  }

  async function simulateRemoteStop(transactionId: number) {
    if (!selectedId.value) return
    try {
      await api.remoteStop(selectedId.value, transactionId)
    } catch (e: any) {
      error.value = e.message
    }
  }

  // ─── Realtime Event Handler ─────────────────────────────────────────────

  function handleRealtimeEvent(event: { type: string; chargePointId: string; connectorId?: number; message?: string }) {
    if (event.chargePointId !== selectedId.value) return

    const cpId = event.chargePointId
    const state = cpStates.value.get(cpId)
    if (!state) return

    if (event.type === 'remote.start_transaction.accepted' && event.connectorId) {
      let cs = state.connectorStates.get(event.connectorId)
      if (!cs) {
        cs = { status: 'Available', cablePluggedIn: false }
        state.connectorStates.set(event.connectorId, cs)
      }
      cs.cablePluggedIn = true
      cs.status = 'Preparing'
    } else if (event.type === 'transaction.confirmed' && event.connectorId) {
      const cs = state.connectorStates.get(event.connectorId)
      if (cs) {
        cs.status = 'Charging'
      }
    } else if (event.type === 'transaction.stopped' && event.connectorId) {
      const cs = state.connectorStates.get(event.connectorId)
      if (cs) {
        cs.status = 'Finishing'
        cs.cablePluggedIn = false
      }
    } else if (event.type === 'transaction.failed' && event.connectorId) {
      const cs = state.connectorStates.get(event.connectorId)
      if (cs) {
        cs.status = 'Available'
        cs.cablePluggedIn = false
      }
    } else if (event.type === 'connector.status_changed' && event.connectorId) {
      // Backend-driven status change (e.g., from RemoteStart, ChangeAvailability)
      // The message typically contains the new status
    }
  }

  function isConnected(cpId: string): boolean {
    return cpStates.value.has(cpId)
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
    startConnectorTransaction,
    stopConnectorTransaction,
    sendConnectorMeterValues,
    setConnectorStatus,
    simulateRemoteStart,
    simulateRemoteStop,
    handleRealtimeEvent,
    isConnected,
    sendOCPPFrame,
  }
})
