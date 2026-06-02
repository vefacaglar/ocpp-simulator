import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api/chargePointsApi'
import type { ChargePoint, Connector } from '../api/chargePointsApi'

type RegistrationState = 'disconnected' | 'pending' | 'accepted' | 'rejected'

interface ChargePointState {
  ws: WebSocket
  registration: RegistrationState
  heartbeatInterval: number | null
  heartbeatTimer: ReturnType<typeof setTimeout> | null
  lastMessageSentAt: number
  pendingCalls: Map<string, { action: string; sentAt: number }>
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
      }
      cpStates.value.set(cpId, state)

      const uniqueId = generateUniqueId()
      const frame = [2, uniqueId, "BootNotification", {
        chargePointVendor: "Simulator",
        chargePointModel: "OCPP-Sim"
      }]
      state.pendingCalls.set(uniqueId, { action: "BootNotification", sentAt: Date.now() })
      ws.send(JSON.stringify(frame))

      loadChargePoints()
      selectChargePoint(cpId)
    }

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data)
        if (Array.isArray(data)) {
          handleOCPPFrame(cpId, data)
        }
      } catch {}
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
    if (!state) return

    const typeID = frame[0] as number

    if (typeID === 3) {
      const uniqueId = frame[1] as string
      const payload = frame[2] as Record<string, unknown>
      const pending = state.pendingCalls.get(uniqueId)

      if (pending) {
        state.pendingCalls.delete(uniqueId)

        if (pending.action === "BootNotification") {
          handleBootNotificationResponse(cpId, state, payload)
        }
      }
    } else if (typeID === 4) {
      const uniqueId = frame[1] as string
      state.pendingCalls.delete(uniqueId)
    }
  }

  function handleBootNotificationResponse(cpId: string, state: ChargePointState, payload: Record<string, unknown>) {
    const status = payload.status as string
    const interval = payload.interval as number

    if (status === "Accepted") {
      state.registration = 'accepted'
      state.heartbeatInterval = interval

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
          state.pendingCalls.set(uniqueId, { action: "BootNotification", sentAt: Date.now() })
          state.ws.send(JSON.stringify(frame))
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

  function sendMessage(cpId: string, frame: unknown[], action: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.ws.readyState !== WebSocket.OPEN) return

    const uniqueId = frame[1] as string
    state.pendingCalls.set(uniqueId, { action, sentAt: Date.now() })
    state.lastMessageSentAt = Date.now()
    state.ws.send(JSON.stringify(frame))

    resetHeartbeatTimer(cpId)
  }

  function scheduleHeartbeat(cpId: string) {
    const state = cpStates.value.get(cpId)
    if (!state || state.registration !== 'accepted' || !state.heartbeatInterval) return

    if (state.heartbeatTimer) {
      clearTimeout(state.heartbeatTimer)
      state.heartbeatTimer = null
    }

    const delay = state.heartbeatInterval * 1000
    state.heartbeatTimer = setTimeout(() => {
      const timeSinceLastMessage = Date.now() - state.lastMessageSentAt
      if (timeSinceLastMessage >= delay) {
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

  function startConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }
    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StartTransaction", {
      connectorId,
      idTag: "DEADBEEF",
      meterStart: 0,
      timestamp: new Date().toISOString()
    }]
    sendMessage(selectedId.value, frame, "StartTransaction")
  }

  function stopConnectorTransaction(_connectorId: number) {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }
    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StopTransaction", {
      transactionId: 1,
      meterStop: 1000,
      timestamp: new Date().toISOString(),
      reason: "Local"
    }]
    sendMessage(selectedId.value, frame, "StopTransaction")
  }

  function sendConnectorMeterValues(connectorId: number) {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }
    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "MeterValues", {
      connectorId,
      meterValue: [{
        timestamp: new Date().toISOString(),
        sampledValue: [{
          value: "1000",
          measurand: "Energy.Active.Import.Register",
          unit: "Wh"
        }]
      }]
    }]
    sendMessage(selectedId.value, frame, "MeterValues")
  }

  function setConnectorStatus(connectorId: number, status: string) {
    if (!selectedId.value) return
    const state = cpStates.value.get(selectedId.value)
    if (!state || state.registration !== 'accepted') {
      error.value = 'Not registered'
      return
    }
    const uniqueId = generateUniqueId()
    const frame = [2, uniqueId, "StatusNotification", {
      connectorId,
      errorCode: "NoError",
      status,
      timestamp: new Date().toISOString()
    }]
    sendMessage(selectedId.value, frame, "StatusNotification")
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
    startConnectorTransaction,
    stopConnectorTransaction,
    sendConnectorMeterValues,
    setConnectorStatus,
    isConnected,
    sendOCPPFrame,
  }
})
