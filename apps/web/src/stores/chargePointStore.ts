import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as api from '../api/chargePointsApi'
import type { ChargePoint, Connector } from '../api/chargePointsApi'

export const useChargePointStore = defineStore('chargePoint', () => {
  const chargePoints = ref<ChargePoint[]>([])
  const selectedId = ref<string | null>(null)
  const selectedDetail = ref<{ chargePoint: ChargePoint; connectors: Connector[] } | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const wsConnections = ref<Map<string, WebSocket>>(new Map())

  const selectedChargePoint = computed(() =>
    chargePoints.value.find((cp) => cp.id === selectedId.value) ?? null
  )

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

  function connectChargePoint() {
    if (!selectedId.value) return
    const cpId = selectedId.value

    if (wsConnections.value.has(cpId)) {
      return
    }

    const ws = api.connectChargePointWS(cpId)

    ws.onopen = () => {
      wsConnections.value.set(cpId, ws)
      loadChargePoints()
      selectChargePoint(cpId)
    }

    ws.onclose = () => {
      wsConnections.value.delete(cpId)
      loadChargePoints()
      selectChargePoint(cpId)
    }

    ws.onerror = () => {
      wsConnections.value.delete(cpId)
      error.value = `Connection failed for ${cpId}`
    }
  }

  function disconnectChargePoint() {
    if (!selectedId.value) return
    disconnectWS(selectedId.value)
  }

  function disconnectWS(cpId: string) {
    const ws = wsConnections.value.get(cpId)
    if (ws) {
      ws.close()
      wsConnections.value.delete(cpId)
    }
  }

  function sendOCPPFrame(frame: unknown[]) {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(frame))
    } else {
      error.value = 'Not connected'
    }
  }

  function bootChargePoint() {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
    const frame = [2, uniqueId, "BootNotification", { chargePointVendor: "Simulator", chargePointModel: "OCPP-Sim" }]
    ws.send(JSON.stringify(frame))
  }

  function heartbeatChargePoint() {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
    const frame = [2, uniqueId, "Heartbeat", {}]
    ws.send(JSON.stringify(frame))
  }

  function startConnectorTransaction(connectorId: number) {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
    const frame = [2, uniqueId, "StartTransaction", {
      connectorId,
      idTag: "DEADBEEF",
      meterStart: 0,
      timestamp: new Date().toISOString()
    }]
    ws.send(JSON.stringify(frame))
  }

  function stopConnectorTransaction(_connectorId: number) {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
    const frame = [2, uniqueId, "StopTransaction", {
      transactionId: 1,
      meterStop: 1000,
      timestamp: new Date().toISOString(),
      reason: "Local"
    }]
    ws.send(JSON.stringify(frame))
  }

  function sendConnectorMeterValues(connectorId: number) {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
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
    ws.send(JSON.stringify(frame))
  }

  function setConnectorStatus(connectorId: number, status: string) {
    if (!selectedId.value) return
    const ws = wsConnections.value.get(selectedId.value)
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      error.value = 'Not connected'
      return
    }
    const uniqueId = crypto.randomUUID().replace(/-/g, '')
    const frame = [2, uniqueId, "StatusNotification", {
      connectorId,
      errorCode: "NoError",
      status,
      timestamp: new Date().toISOString()
    }]
    ws.send(JSON.stringify(frame))
  }

  function isConnected(cpId: string): boolean {
    return wsConnections.value.has(cpId)
  }

  return {
    chargePoints,
    selectedId,
    selectedDetail,
    loading,
    error,
    selectedChargePoint,
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
