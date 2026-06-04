import {
  createConnector,
  deleteChargePointCascade,
  deleteConnectorRecord,
  getChargePoint,
  listChargePoints,
  listConnectors,
  putChargePoint,
  type ChargePointRecord,
  type ConnectorRecord,
} from '../db/browserDb'
import { DEFAULT_CENTRAL_SYSTEM_URL } from '../config/defaults'

export type ChargePoint = ChargePointRecord
export type Connector = ConnectorRecord

export async function fetchChargePoints(): Promise<ChargePoint[]> {
  return listChargePoints()
}

export async function fetchChargePoint(id: string): Promise<{ chargePoint: ChargePoint; connectors: Connector[] }> {
  const chargePoint = await getChargePoint(id)
  if (!chargePoint) throw new Error('charge point not found')
  return {
    chargePoint,
    connectors: await listConnectors(id),
  }
}

export async function createChargePoint(input: {
  id: string
  name?: string
  ocppVersion?: string
  centralSystemUrl?: string
}): Promise<ChargePoint> {
  const existing = await getChargePoint(input.id)
  if (existing) throw new Error('charge point already exists')

  const now = new Date().toISOString()
  const record: ChargePoint = {
    id: input.id,
    name: input.name || input.id,
    ocppVersion: input.ocppVersion || '1.6J',
    centralSystemUrl: input.centralSystemUrl || DEFAULT_CENTRAL_SYSTEM_URL,
    connectorCount: 0,
    autoConnect: false,
    status: 'disconnected',
    createdAt: now,
    updatedAt: now,
  }
  await putChargePoint(record)
  return record
}

export async function deleteChargePoint(id: string): Promise<void> {
  await deleteChargePointCascade(id)
}

export async function addConnector(chargePointId: string): Promise<Connector> {
  return createConnector(chargePointId)
}

export async function deleteConnector(chargePointId: string, connectorId: number): Promise<void> {
  await deleteConnectorRecord(chargePointId, connectorId)
}

export async function fetchVersions(): Promise<{ version: string; label: string; status: string }[]> {
  return [
    { version: '1.6J', label: 'OCPP 1.6J', status: 'enabled' },
    { version: '2.0.1', label: 'OCPP 2.0.1', status: 'planned' },
  ]
}
