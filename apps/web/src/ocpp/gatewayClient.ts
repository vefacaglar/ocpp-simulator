// gatewayClient owns the per-charge-point WebSocket connection to
// ocpp-gateway. The URL format is /ws/{chargePointId}; the Vite dev
// server proxies this to ws://localhost:7080/ws/{chargePointId}.
//
// The client is intentionally dumb: it opens one socket per CP,
// frames are sent and received as raw text JSON arrays, and there
// is no protocol-level awareness. Higher-level helpers (pending
// call correlation, response building) live in callResponses.ts.
export type FrameHandler = (frame: unknown[]) => void

export interface GatewayClientOptions {
  chargePointId: string
  onFrame: FrameHandler
  onOpen?: () => void
  onClose?: () => void
  onError?: (err: Event) => void
}

// Public surface of GatewayClient. The class itself is also
// exported but consumers should program against this interface
// when only the public behavior is needed.
export interface IGatewayClient {
  readonly chargePointId: string
  readonly isOpen: boolean
  connect(): void
  send(frame: unknown[]): boolean
  close(): void
}

export class GatewayClient implements IGatewayClient {
  readonly chargePointId: string
  private readonly opts: GatewayClientOptions
  private socket: WebSocket | null = null
  private intentionalClose = false
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null

  constructor(opts: GatewayClientOptions) {
    this.chargePointId = opts.chargePointId
    this.opts = opts
  }

  connect(): void {
    this.intentionalClose = false
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const url = `${protocol}//${host}/ws/${encodeURIComponent(this.chargePointId)}`
    const ws = new WebSocket(url)
    this.socket = ws

    ws.onopen = () => {
      this.opts.onOpen?.()
    }

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data)
        if (Array.isArray(data)) {
          this.opts.onFrame(data as unknown[])
        }
      } catch {
        // eslint-disable-next-line no-console
        console.warn('[ocpp/gateway] non-JSON or invalid frame on /ws/' + this.chargePointId)
      }
    }

    ws.onclose = () => {
      this.socket = null
      this.opts.onClose?.()
      if (!this.intentionalClose) {
        this.scheduleReconnect()
      }
    }

    ws.onerror = (err) => {
      this.opts.onError?.(err)
    }
  }

  send(frame: unknown[]): boolean {
    const ws = this.socket
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return false
    }
    ws.send(JSON.stringify(frame))
    return true
  }

  close(): void {
    this.intentionalClose = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.socket) {
      this.socket.close()
      this.socket = null
    }
  }

  get isOpen(): boolean {
    return this.socket !== null && this.socket.readyState === WebSocket.OPEN
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      if (!this.intentionalClose) this.connect()
    }, 3000)
  }
}
