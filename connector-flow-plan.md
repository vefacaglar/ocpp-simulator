# Connector Start Flow Revizyon — Uygulama Planı

## 1. Mevcut Durum Analizi

### 1.1 Bugünkü Akış (Problemli)

```
Available → [Start TX butonu] → StatusNotification(Preparing) + StartTransaction → Charging
```

- `Available` durumunda tek tıkla "Start TX" butonu var.
- Kablo takılmadan (Plug In) transaction başlıyor.
- `idTag` hardcoded `"DEADBEEF"` — yetkilendirme yok.
- `Authorize` mesajı hiç gönderilmiyor.
- plan.md §7b'deki ConnectorStateMachine atlanıyor.

### 1.2 Kabul Edilen İletişim Modeli

| Katman | Kim kullanıyor | Ne yapıyor |
|--------|---------------|------------|
| OCPP session proxy (`/api/ws/{cpId}`) | Frontend-controlled simulated charge point | İlgili unit için ayrı OCPP WebSocket session açar ve frame'leri `centralSystemUrl/{cpId}` adresine taşır |
| UI realtime (`/api/realtime`) | Frontend | Log/state/event izler; OCPP transport değildir |
| Mock CSMS REST API | Dış test istemcisi / UI dev tool | CSMS-initiated CALL üretir ve bağlı CP'nin OCPP socket'inden gönderir |

Frontend şu an proxy WS üzerinden BootNotification, Heartbeat, StatusNotification, Authorize, StartTransaction, StopTransaction, MeterValues gönderiyor. Bu proje için bu kabul edilen CP-initiated davranıştır: UI cihaz davranışını kontrol eder, ama her unit yine kendi OCPP WebSocket session'ı üzerinden CSMS ile konuşur. Bu akış runtime command API'lerine taşınmamalı.

### 1.3 Karar: Unit Başına OCPP Session + CSMS-Initiated API

- **Local/CP-initiated akışlar:** UI, seçili unit'in `/api/ws/{cpId}` OCPP session'ını kullanır. Plug/Unplug/Authorize/Start/Stop/MeterValues/StatusNotification bu session'dan gider.
- **RemoteStart/RemoteStop (CSMS-initiated):** Dışarıdan istek mock CSMS API'sine gelir; mock CSMS hedef CP'nin mevcut OCPP socket'i üzerinden `RemoteStartTransaction` / `RemoteStopTransaction` CALL gönderir.
- **DB persistansı:** Bu faz kapsamında değil (frontend in-memory state yeterli).

---

## 2. Hedef Durum

### 2.1 Konnektör Durum Makinesi

```
Available ──Plug In──> Preparing ──(Auth OK | RemoteStart)──> Charging
   ^                       │                                     │
   │                       └──Unplug──> Available                │
   │                                                             │
   └──────────────── Finishing <──Stop / Unplug─────────────────┘

Available/Preparing/Charging ──Fault──> Faulted ──Clear──> (önceki durum)
Available ──Disable──> Unavailable ──Enable──> Available
```

### 2.2 Duruma-Bağlı UI Kontrolleri

| Durum | Butonlar | Tetiklenen |
|-------|----------|------------|
| `Available` | **Plug In**, Fault, Disable | Plug In → Preparing + StatusNotification |
| `Preparing` | **Authorize** (idTag input), **Unplug**, Fault | Authorize → Authorize.conf → (Accepted) → StartTransaction |
| `Charging` | **Stop**, MeterValues, Fault | Stop → StopTransaction → Finishing → Available |
| `Finishing` | **Unplug** | Unplug → Available |
| `Faulted` | **Clear Fault** | → önceki uygun durum |
| `Unavailable` | **Enable** | → Available |

### 2.3 İki Başlatma Akışı

**Akış A — Yerel Yetkilendirme (CP-initiated, unit OCPP session):**
```
[Plug In]    CP → StatusNotification(connectorId, Preparing)     ← CS {}
[Authorize]  CP → Authorize(idTag)                               ← CS {idTagInfo: Accepted}
             CP → StartTransaction(connectorId, idTag, meterStart) ← CS {transactionId, idTagInfo}
             CP → StatusNotification(connectorId, Charging)       ← CS {}
[charging]   CP → MeterValues(... transactionId ...)  (periyodik)
[Stop]       CP → StopTransaction(transactionId, meterStop, reason) ← CS {idTagInfo}
             CP → StatusNotification(connectorId, Finishing)      ← CS {}
[Unplug]     CP → StatusNotification(connectorId, Available)      ← CS {}
```

**Akış B — Uzaktan Başlatma (CSMS-initiated, mock CSMS API):**
```
[Plug In]    CP → StatusNotification(connectorId, Preparing)     ← CS {}
             CS → RemoteStartTransaction(idTag, connectorId)     → CP {status: Accepted}
             CP → StartTransaction(...)                          ← CS {transactionId}
             CP → StatusNotification(connectorId, Charging)       ← CS {}
```

---

## 3. Frontend Değişiklikleri

### 3.1 chargePointStore.ts — Yeni State Modeli

**Mevcut `TransactionState` genişletme:**
```typescript
interface TransactionState {
  transactionId: number | null
  connectorId: number
  idTag: string
  meterStart: number
  meterCurrent: number
  status: 'preparing' | 'charging' | 'finishing'
}
```

**Yeni: `ConnectorRuntimeState` ekleme:**
```typescript
interface ConnectorRuntimeState {
  status: ConnectorStatus  // 'Available' | 'Preparing' | 'Charging' | 'Finishing' | 'Faulted' | 'Unavailable'
  cablePluggedIn: boolean
  transaction: TransactionState | null
}

type ConnectorStatus = 'Available' | 'Preparing' | 'Charging' | 'SuspendedEV' | 'SuspendedEVSE' | 'Finishing' | 'Faulted' | 'Unavailable'
```

**`ChargePointState` güncelleme:**
```typescript
interface ChargePointState {
  ws: WebSocket
  registration: RegistrationState
  heartbeatInterval: number | null
  heartbeatTimer: ReturnType<typeof setTimeout> | null
  lastMessageSentAt: number
  pendingCalls: Map<string, { action: string; sentAt: number }>
  connectorStates: Map<number, ConnectorRuntimeState>  // yeni: connectorId → state
  meterValuesTimer: ReturnType<typeof setInterval> | null
}
```

### 3.2 chargePointStore.ts — Yeni Aksiyonlar

#### `plugInConnector(connectorId: number)`
```typescript
function plugInConnector(connectorId: number) {
  // 1. ConnectorRuntimeState oluştur veya güncelle
  //    status: 'Preparing', cablePluggedIn: true
  // 2. StatusNotification(Preparing) gönder (proxy WS)
  // 3. connectorStates Map'ini güncelle
}
```

#### `authorizeConnector(connectorId: number, idTag: string)`
```typescript
function authorizeConnector(connectorId: number, idTag: string) {
  // 1. Authorize CALL gönder (proxy WS): [2, uid, "Authorize", {idTag}]
  // 2. PendingCalls'a ekle: { action: "Authorize", connectorId, idTag }
  // 3. Authorize.conf gelene kadar bekle
}
```

#### `handleAuthorizeResponse(cpId, state, payload, pendingInfo)`
```typescript
function handleAuthorizeResponse(cpId, state, payload, pendingInfo) {
  const status = payload.idTagInfo?.status
  if (status === 'Accepted') {
    // 1. StartTransaction CALL gönder (proxy WS)
    // 2. PendingCalls'a ekle: { action: "StartTransaction", connectorId, idTag }
  } else {
    // 1. Hata göster: "Yetkilendirme reddedildi: {status}"
    // 2. Konnektörü Preparing'te tut (veya Available'a dön)
    // 3. connectorStates güncelle
  }
}
```

#### `unplugConnector(connectorId: number)`
```typescript
function unplugConnector(connectorId: number) {
  // 1. StatusNotification(Available) gönder
  // 2. connectorStates: status='Available', cablePluggedIn=false
  // 3. Aktif transaction varsa temizle
}
```

#### `handleAuthorizeRejected` — durum geri alma
```typescript
// Authorize reddedildiğinde:
// - Konnektör Preparing'te kalır (kablo hâlâ takılı)
// - Kullanıcı tekrar deneyebilir veya Unplug ile ayrılabilir
```

### 3.3 handleOCPPFrame — Authorize Response Routing

Mevcut `handleOCPPFrame` fonksiyonuna Authorize response routing ekle:

```typescript
if (pending.action === "Authorize") {
  handleAuthorizeResponse(cpId, state, payload, pending)
} else if (pending.action === "StartTransaction") {
  handleStartTransactionResponse(cpId, state, payload)
}
```

### 3.4 ConnectorCard Bileşeni (ChargePointDetail.vue)

**Mevcut:** Sabit buton seti (`Start TX`, `Stop TX`, `MeterValues`, `Fault`, `Clear`, `Disable`, `Enable`).

**Yeni:** Duruma-bağlı dinamik butonlar + Authorize akışı için idTag input.

```
┌─────────────────────────────────────────────┐
│ Connector 1                    [Available]  │
│                                             │
│ [Plug In]  [Fault]  [Disable]               │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ Connector 1                    [Preparing]  │
│                                             │
│ idTag: [__________]                         │
│ [Authorize]  [Unplug]  [Fault]              │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ Connector 1                    [Charging]   │
│ Session: TX-42 | Meter: 1250 Wh            │
│                                             │
│ [Stop]  [MeterValues]  [Fault]              │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ Connector 1                    [Finishing]  │
│                                             │
│ [Unplug]                                    │
└─────────────────────────────────────────────┘
```

### 3.5 Simulate Remote Start (Dev Tool)

"Preparing" durumunda, test amaçlı bir **"Simulate Remote Start"** butonu ekle:

```
┌─────────────────────────────────────────────┐
│ Connector 1                    [Preparing]  │
│                                             │
│ idTag: [__________]                         │
│ [Authorize]  [Unplug]  [Fault]              │
│ ─── Dev Tools ───                           │
│ [Simulate Remote Start]                     │
└─────────────────────────────────────────────┘
```

Bu buton backend'deki `POST /api/charge-points/{id}/remote-start` endpoint'ine çağrı yapar. Backend runtime `HandleRemoteStartTransaction`'ı tetikler ve sonuç realtime WS üzerinden UI'a düşer.

---

## 4. Backend Değişiklikleri

### 4.1 Yeni REST Endpoint — Simulate Remote Start

**Endpoint:** `POST /api/charge-points/{id}/remote-start`

**Request body:**
```json
{
  "idTag": "ABCDEF12",
  "connectorId": 1  // optional, omitted = first available
}
```

**Response:** `{ "status": "Accepted" }` veya `{ "status": "Rejected" }`

**Implementation:** `Runtime.HandleRemoteStartTransaction()` zaten mevcut. Sadece yeni bir HTTP handler ekle.

### 4.2 Yeni REST Endpoint — Simulate Remote Stop

**Endpoint:** `POST /api/charge-points/{id}/remote-stop`

**Request body:**
```json
{
  "transactionId": 42
}
```

**Response:** `{ "status": "Accepted" }` veya `{ "status": "Rejected" }`

**Implementation:** `Runtime.HandleRemoteStopTransaction()` zaten mevcut.

### 4.3 server.go — Yeni Handler'lar

```go
// handleRemoteStart — Simulate RemoteStartTransaction (dev tool)
func (s *Server) handleRemoteStart(w http.ResponseWriter, r *http.Request) {
    cpID := chi.URLParam(r, "id")
    var req struct {
        IDTag       string `json:"idTag"`
        ConnectorID *int   `json:"connectorId,omitempty"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    status, err := s.runtime.HandleRemoteStartTransaction(cpID, &ocpp.RemoteStartTransactionRequest{
        IDTag:       req.IDTag,
        ConnectorID: req.ConnectorID,
    })
    // ... respond with status
}

// handleRemoteStop — Simulate RemoteStopTransaction (dev tool)
func (s *Server) handleRemoteStop(w http.ResponseWriter, r *http.Request) {
    cpID := chi.URLParam(r, "id")
    var req struct {
        TransactionID int `json:"transactionId"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    status, err := s.runtime.HandleRemoteStopTransaction(cpID, &ocpp.RemoteStopTransactionRequest{
        TransactionID: req.TransactionID,
    })
    // ... respond with status
}
```

### 4.4 Route Kayıtları

```go
r.Post("/api/charge-points/{id}/remote-start", s.handleRemoteStart)
r.Post("/api/charge-points/{id}/remote-stop", s.handleRemoteStop)
```

### 4.5 Backend Runtime — RemoteStart Akışı İyileştirmesi

Mevcut `HandleRemoteStartTransaction` zaten doğru çalışıyor:
1. Connector durumunu kontrol eder
2. Available → Preparing geçişi yapar
3. Async olarak `StartTransaction()` çağırır
4. `StartTransaction.conf` geldiğinde Preparing → Charging geçişi yapar

**Eklenmesi gereken:** RemoteStart tetiklendiğinde frontend'in durumu güncellemesi için realtime event'ler zaten publish ediliyor (`remote.start_transaction.accepted`, `transaction.started`, `transaction.confirmed`). Frontend bu event'leri dinleyerek UI'ı güncelleyecek.

### 4.6 Backend Runtime — RemoteStart Sonrası StatusNotification

Mevcut `HandleRemoteStartTransaction` → `StartTransaction()` akışında StatusNotification gönderiliyor mu?

**Mevcut kod (`runtime.go:256`):**
```go
if err := r.SetConnectorStatus(cpID, connectorID, common.ConnectorPreparing); err != nil {
    return fmt.Errorf("transition to Preparing: %w", err)
}
```

`SetConnectorStatus` zaten StatusNotification gönderiyor (eğer CP connected ise). ✅

`HandleStartTransactionResponse` içinde de Preparing → Charging geçişi var:
```go
r.SetConnectorStatus(cpID, targetConnector, common.ConnectorCharging)
```

Bu da StatusNotification gönderiyor. ✅

---

## 5. Frontend — Realtime Event Entegrasyonu

### 5.1 RemoteStart Akışında UI Güncelleme

RemoteStart backend tarafından tetiklendiğinde, frontend'in UI'ı güncellemesi gerek:

1. `remote.start_transaction.accepted` event'i gelir → konnektör durumu "Preparing" olarak güncellenir
2. `transaction.started` event'i gelir → konnektör durumu "Charging" olarak güncellenir
3. `transaction.confirmed` event'i gelir → transaction detayları gösterilir

**realtimeStore.ts veya App.vue event handler:**
```typescript
if (event.type === 'remote.start_transaction.accepted' && event.connectorId) {
  // Konnektör durumunu Preparing olarak güncelle
  updateConnectorStatus(event.connectorId, 'Preparing')
}
if (event.type === 'transaction.confirmed' && event.connectorId) {
  // Konnektör durumunu Charging olarak güncelle
  updateConnectorStatus(event.connectorId, 'Charging')
}
```

### 5.2 Connector Durumu Güncelleme Mekanizması

Mevcut durumda connector status sadece API'den fetch edildiğinde güncelleniyor (`selectChargePoint`). Realtime event'lerden gelen durum değişikliklerinin UI'a yansıması için:

**Seçenek A:** Her realtime event'te `selectChargePoint()` çağır (API round-trip var).
**Seçenek B:** Frontend'de connector state'i realtime event'lerden güncelle (daha hızlı).

**Önerilen:** Seçenek B — `cpStates.connectorStates` Map'ini realtime event'lerden güncelle.

---

## 6. Uygulama Sırası

### Adım 1: Frontend State Modeli
- `ConnectorRuntimeState` interface ekle
- `ChargePointState.connectorStates` Map ekle
- Mevcut `transactions` Map'ini `connectorStates.transaction` içine taşı

### Adım 2: Yeni Store Aksiyonları
- `plugInConnector(connectorId)` — StatusNotification(Preparing) gönder
- `authorizeConnector(connectorId, idTag)` — Authorize CALL gönder
- `handleAuthorizeResponse()` — Authorize.conf işle
- `unplugConnector(connectorId)` — StatusNotification(Available) gönder

### Adım 3: handleOCPPFrame Güncellemesi
- Authorize response routing ekle
- StartTransaction response routing'i mevcut (zaten var)

### Adım 4: ConnectorCard UI Revizyonu
- Duruma-bağlı buton rendering
- idTag input (Preparing durumunda)
- Simulate Remote Start butonu (dev tool)

### Adım 5: Backend — Remote Start/Stop Endpoint'leri
- `POST /api/charge-points/{id}/remote-start` handler
- `POST /api/charge-points/{id}/remote-stop` handler
- Route kayıtları

### Adım 6: Realtime Event Entegrasyonu
- RemoteStart event'lerini dinle
- Connector durumunu realtime event'lerden güncelle

### Adım 7: Test
- Manuel test: Plug In → Authorize (Accepted) → StartTransaction → Charging → Stop
- Manuel test: Plug In → Authorize (Rejected) → hata gösterimi
- Manuel test: Plug In → Simulate Remote Start → Charging
- Manuel test: Simulate Remote Stop → Finishing → Unplug

---

## 7. Dosya Değişiklik Özeti

### Değiştirilecek Dosyalar

| Dosya | Değişiklik |
|-------|-----------|
| `apps/web/src/stores/chargePointStore.ts` | `ConnectorRuntimeState`, `plugInConnector`, `authorizeConnector`, `handleAuthorizeResponse`, `unplugConnector`, Authorize response routing |
| `apps/web/src/components/ChargePointDetail.vue` | Duruma-bağlı butonlar, idTag input, Simulate Remote Start butonu |
| `apps/web/src/api/chargePointsApi.ts` | `remoteStart()`, `remoteStop()` API fonksiyonları |
| `apps/api/internal/api/server.go` | `handleRemoteStart`, `handleRemoteStop` handler'ları, route kayıtları |

### Değiştirilmeyecek Dosyalar

| Dosya | Neden |
|-------|-------|
| `apps/api/internal/simulator/runtime.go` | `HandleRemoteStartTransaction` zaten mevcut |
| `apps/api/internal/simulator/inbound_handler.go` | CSMS-initiated handling zaten mevcut |
| `apps/api/internal/simulator/state_machine.go` | Durum makinesi zaten doğru |
| `apps/api/internal/ocpp/v16/protocol.go` | BuildAuthorize zaten mevcut |
| `apps/csms/` | Mock CSMS handler'ları zaten mevcut |

---

## 8. Kapsam Dışı (Bu Faz)

- DB persistansı (transaction kaydetme) — frontend in-memory state yeterli
- SuspendedEV / SuspendedEVSE duraklatma akışları
- ChargingProfile / akıllı şarj limitleri
- Reservation (Reserved durumu)
- Auth-first sırası (kart önce, kablo sonra)
- MeterValues otomatik üretiminin backend tarafından yapılması
- StopTransaction response handling (idTagInfo)

---

## 9. Riskler ve Notlar

1. **Proxy vs Runtime çelişkisi:** Frontend proxy WS üzerinden StartTransaction gönderirken, backend runtime da kendi connection'ı üzerinden StartTransaction gönderebilir (RemoteStart durumunda). Bu çakışma olmaz çünkü RemoteStart'ta backend runtime kendi connection'ını kullanır, frontend proxy'yi değil.

2. **State senkronizasyonu:** Frontend connector state'i proxy WS üzerinden yönetir. Backend runtime connector state'i kendi in-memory yapısında yönetir. RemoteStart durumunda bu iki state farklı olabilir. Realtime event'ler bu senkronizasyonu sağlar.

3. **idTag girişi:** Kullanıcı idTag'i elle girer. Gerçek dünyada RFID okuyucu veya NFC kullanılır. Bu simülatörde elle giriş yeterli.

4. **Authorize reddedildiğinde:** Konnektör Preparing'te kalır. Kullanıcı yeni idTag ile tekrar deneyebilir veya Unplug ile ayrılabilir. Bu, gerçek dünya davranışına uygundur.
