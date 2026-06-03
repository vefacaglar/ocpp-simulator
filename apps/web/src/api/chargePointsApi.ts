// CRUD + config API client for simulator-api. OCPP frame transport
// is no longer here in the v4.3 split; the Vue app opens one
// WebSocket per charge point directly to ocpp-gateway (see
// src/ocpp/gatewayClient.ts). This module is intentionally limited
// to charge-point, connector, and settings management.

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

export async function fetchVersions(): Promise<{ version: string; label: string; status: string }[]> {
  const res = await fetch('/api/versions')
  if (!res.ok) throw new Error('failed to fetch versions')
  return res.json()
}
