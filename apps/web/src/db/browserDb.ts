import type { RealtimeEvent } from '../realtime/realtimeClient'

const DB_NAME = 'ocpp-simulator-browser'
const DB_VERSION = 1
const MAX_LOGS_PER_CHARGE_POINT = 1000

export interface ChargePointRecord {
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

export interface ConnectorRecord {
  id: number
  chargePointId: string
  evseId: number
  connectorNumber: number
  status: string
  createdAt: string
  updatedAt: string
}

interface AppStateRecord {
  key: string
  value: unknown
}

export interface BrowserDbExport {
  version: 1
  exportedAt: string
  chargePoints: ChargePointRecord[]
  connectors: ConnectorRecord[]
  ocppLogs: RealtimeEvent[]
  appState: AppStateRecord[]
}

let dbPromise: Promise<IDBDatabase> | null = null

function db(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const database = req.result
      if (!database.objectStoreNames.contains('chargePoints')) {
        database.createObjectStore('chargePoints', { keyPath: 'id' })
      }
      if (!database.objectStoreNames.contains('connectors')) {
        const store = database.createObjectStore('connectors', { keyPath: 'id', autoIncrement: true })
        store.createIndex('chargePointId', 'chargePointId', { unique: false })
      }
      if (!database.objectStoreNames.contains('ocppLogs')) {
        const store = database.createObjectStore('ocppLogs', { keyPath: 'id' })
        store.createIndex('chargePointId', 'chargePointId', { unique: false })
        store.createIndex('chargePointTimestamp', ['chargePointId', 'timestamp'], { unique: false })
      }
      if (!database.objectStoreNames.contains('appState')) {
        database.createObjectStore('appState', { keyPath: 'key' })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error ?? new Error('failed to open browser database'))
  })
  return dbPromise
}

async function readonlyStore(storeName: string): Promise<IDBObjectStore> {
  const database = await db()
  return database.transaction(storeName, 'readonly').objectStore(storeName)
}

async function writableStore(storeName: string): Promise<IDBObjectStore> {
  const database = await db()
  return database.transaction(storeName, 'readwrite').objectStore(storeName)
}

function request<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error ?? new Error('browser database request failed'))
  })
}

function transactionDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error ?? new Error('browser database transaction failed'))
    tx.onabort = () => reject(tx.error ?? new Error('browser database transaction aborted'))
  })
}

export async function listChargePoints(): Promise<ChargePointRecord[]> {
  const store = await readonlyStore('chargePoints')
  return request<ChargePointRecord[]>(store.getAll()).then((rows) =>
    rows.sort((a, b) => a.createdAt.localeCompare(b.createdAt))
  )
}

async function listAppState(): Promise<AppStateRecord[]> {
  const store = await readonlyStore('appState')
  return request<AppStateRecord[]>(store.getAll())
}

export async function getChargePoint(id: string): Promise<ChargePointRecord | undefined> {
  const store = await readonlyStore('chargePoints')
  return request<ChargePointRecord | undefined>(store.get(id))
}

export async function putChargePoint(record: ChargePointRecord): Promise<void> {
  const store = await writableStore('chargePoints')
  await request(store.put(record))
}

export async function deleteChargePointCascade(id: string): Promise<void> {
  const existingConnectors = await listConnectors(id)
  const existingLogs = await listAllOcppLogs(id)
  const database = await db()
  const tx = database.transaction(['chargePoints', 'connectors', 'ocppLogs'], 'readwrite')
  tx.objectStore('chargePoints').delete(id)
  for (const connector of existingConnectors) tx.objectStore('connectors').delete(connector.id)
  for (const log of existingLogs) tx.objectStore('ocppLogs').delete(log.id)
  await transactionDone(tx)
}

export async function listConnectors(chargePointId: string): Promise<ConnectorRecord[]> {
  const store = await readonlyStore('connectors')
  const rows = await request<ConnectorRecord[]>(store.index('chargePointId').getAll(chargePointId))
  return rows.sort((a, b) => a.connectorNumber - b.connectorNumber)
}

export async function createConnector(chargePointId: string): Promise<ConnectorRecord> {
  const chargePoint = await getChargePoint(chargePointId)
  if (!chargePoint) throw new Error('charge point not found')

  const existing = await listConnectors(chargePointId)
  const maxConnector = existing.reduce((max, c) => Math.max(max, c.connectorNumber), 0)
  const now = new Date().toISOString()
  const recordWithoutId: Omit<ConnectorRecord, 'id'> = {
    chargePointId,
    connectorNumber: maxConnector + 1,
    evseId: maxConnector + 1,
    status: 'Available',
    createdAt: now,
    updatedAt: now,
  }
  const connectorStore = await writableStore('connectors')
  const id = await request<IDBValidKey>(connectorStore.add(recordWithoutId))
  const record: ConnectorRecord = { ...recordWithoutId, id: Number(id) }
  chargePoint.connectorCount = existing.length + 1
  chargePoint.updatedAt = now
  await putChargePoint(chargePoint)
  return record
}

export async function deleteConnectorRecord(chargePointId: string, connectorId: number): Promise<void> {
  const connectorStore = await writableStore('connectors')
  await request(connectorStore.delete(connectorId))
  const chargePoint = await getChargePoint(chargePointId)
  if (chargePoint) {
    const existing = await listConnectors(chargePointId)
    chargePoint.connectorCount = existing.length
    chargePoint.updatedAt = new Date().toISOString()
    await putChargePoint(chargePoint)
  }
}

export async function getAppState<T>(key: string): Promise<T | undefined> {
  const store = await readonlyStore('appState')
  const row = await request<AppStateRecord | undefined>(store.get(key))
  return row?.value as T | undefined
}

export async function setAppState(key: string, value: unknown): Promise<void> {
  const store = await writableStore('appState')
  await request(store.put({ key, value }))
}

export async function appendOcppLog(event: RealtimeEvent): Promise<void> {
  const store = await writableStore('ocppLogs')
  await request(store.put(event))
  await pruneOldLogs(event.chargePointId)
}

async function pruneOldLogs(chargePointId: string): Promise<void> {
  const rows = await listAllOcppLogs(chargePointId)
  if (rows.length <= MAX_LOGS_PER_CHARGE_POINT) return
  rows.sort((a, b) => a.timestamp.localeCompare(b.timestamp))
  const store = await writableStore('ocppLogs')
  for (const row of rows.slice(0, rows.length - MAX_LOGS_PER_CHARGE_POINT)) {
    await request(store.delete(row.id))
  }
}

export async function listOcppLogs(chargePointId: string, limit = 500): Promise<RealtimeEvent[]> {
  const rows = await listAllOcppLogs(chargePointId)
  return rows.sort((a, b) => a.timestamp.localeCompare(b.timestamp)).slice(-limit)
}

async function listAllOcppLogs(chargePointId: string): Promise<RealtimeEvent[]> {
  const store = await readonlyStore('ocppLogs')
  const rows = await request<RealtimeEvent[]>(store.index('chargePointId').getAll(chargePointId))
  return rows
}

async function listAllConnectors(): Promise<ConnectorRecord[]> {
  const store = await readonlyStore('connectors')
  return request<ConnectorRecord[]>(store.getAll())
}

function normalizeConnector(record: ConnectorRecord): ConnectorRecord {
  if (record.evseId === record.connectorNumber) return record
  return { ...record, evseId: record.connectorNumber }
}

async function listAllLogs(): Promise<RealtimeEvent[]> {
  const store = await readonlyStore('ocppLogs')
  return request<RealtimeEvent[]>(store.getAll())
}

export async function exportBrowserDb(): Promise<BrowserDbExport> {
  return {
    version: 1,
    exportedAt: new Date().toISOString(),
    chargePoints: await listChargePoints(),
    connectors: (await listAllConnectors()).map(normalizeConnector),
    ocppLogs: await listAllLogs(),
    appState: await listAppState(),
  }
}

export async function importBrowserDb(data: BrowserDbExport): Promise<void> {
  if (data.version !== 1) {
    throw new Error('unsupported import version')
  }
  if (
    !Array.isArray(data.chargePoints) ||
    !Array.isArray(data.connectors) ||
    !Array.isArray(data.ocppLogs) ||
    !Array.isArray(data.appState)
  ) {
    throw new Error('invalid import file')
  }
  const database = await db()
  const tx = database.transaction(['chargePoints', 'connectors', 'ocppLogs', 'appState'], 'readwrite')
  for (const storeName of ['chargePoints', 'connectors', 'ocppLogs', 'appState']) {
    tx.objectStore(storeName).clear()
  }
  for (const row of data.chargePoints) tx.objectStore('chargePoints').put(row)
  for (const row of data.connectors) tx.objectStore('connectors').put(row)
  for (const row of data.ocppLogs) tx.objectStore('ocppLogs').put(row)
  for (const row of data.appState) tx.objectStore('appState').put(row)
  await transactionDone(tx)
}
