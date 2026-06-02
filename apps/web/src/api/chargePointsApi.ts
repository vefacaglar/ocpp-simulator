export interface ChargePoint {
  id: string
  name: string
  ocppVersion: string
  centralSystemUrl: string
  connectorCount: number
  autoConnect: boolean
  status: 'disconnected' | 'connecting' | 'connected'
  createdAt: string
  updatedAt: string
}

export interface Connector {
  id: number
  chargePointId: string
  evseId: number
  connectorNumber: number
  status: string
  createdAt: string
  updatedAt: string
}

export async function fetchChargePoints(): Promise<ChargePoint[]> {
  const res = await fetch('/api/charge-points')
  if (!res.ok) throw new Error('failed to fetch')
  return res.json()
}

export async function fetchChargePoint(id: string): Promise<{ chargePoint: ChargePoint; connectors: Connector[] }> {
  const res = await fetch(`/api/charge-points/${id}`)
  if (!res.ok) throw new Error('failed to fetch')
  return res.json()
}

export async function createChargePoint(input: {
  id: string
  name?: string
  ocppVersion?: string
  centralSystemUrl?: string
}): Promise<ChargePoint> {
  const res = await fetch('/api/charge-points', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error || 'failed to create')
  }
  return res.json()
}

export async function deleteChargePoint(id: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('failed to delete')
}

export async function addConnector(chargePointId: string): Promise<Connector> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({}),
  })
  if (!res.ok) throw new Error('failed to add connector')
  return res.json()
}

export async function deleteConnector(chargePointId: string, connectorId: number): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors/${connectorId}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('failed to delete connector')
}

export async function connect(chargePointId: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connect`, { method: 'POST' })
  if (!res.ok) throw new Error('failed to connect')
}

export async function disconnect(chargePointId: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/disconnect`, { method: 'POST' })
  if (!res.ok) throw new Error('failed to disconnect')
}

export async function boot(chargePointId: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/boot`, { method: 'POST' })
  if (!res.ok) throw new Error('failed to send boot')
}

export async function heartbeat(chargePointId: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/heartbeat`, { method: 'POST' })
  if (!res.ok) throw new Error('failed to send heartbeat')
}

export async function startTransaction(chargePointId: string, connectorId: number, idTag: string = 'DEADBEEF'): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors/${connectorId}/start-transaction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ idTag }),
  })
  if (!res.ok) throw new Error('failed to start transaction')
}

export async function stopTransaction(chargePointId: string, connectorId: number, reason: string = 'Local'): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors/${connectorId}/stop-transaction`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason }),
  })
  if (!res.ok) throw new Error('failed to stop transaction')
}

export async function sendMeterValues(chargePointId: string, connectorId: number): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors/${connectorId}/meter-values`, { method: 'POST' })
  if (!res.ok) throw new Error('failed to send meter values')
}

export async function setConnectorStatus(chargePointId: string, connectorId: number, status: string): Promise<void> {
  const res = await fetch(`/api/charge-points/${chargePointId}/connectors/${connectorId}/status`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ status }),
  })
  if (!res.ok) throw new Error('failed to set connector status')
}

export function connectChargePointWS(chargePointId: string): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return new WebSocket(`${protocol}//${host}/api/ws/${chargePointId}`)
}
