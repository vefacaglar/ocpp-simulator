# How a Charge Point and the Backend Communicate in OCPP

A from-scratch explanation, assuming no prior knowledge.

## There are two sides

On one side there's the **charge point** (the charging station — that box on the wall or by the road). On the other side there's your **backend system** (in OCPP terminology this is called the "Central System", or "CSMS" in OCPP 2.0.1).

These two are in physically separate places. The charge point is out on the street, the backend lives on a server. The internet sits between them.

How do they talk? They need to open a phone line. That line is a **WebSocket**.

## First, the connection is established

When the charge point powers on, **it** is the one that connects to the backend. Note the direction: the backend does not reach out to the device — the device reaches out to the backend.

A URL is pre-configured inside the device, for example:

```
ws://your-backend.com/ocpp/CP_12345
```

The `CP_12345` at the end is the device's identity. Once the backend accepts this connection, there is now an open phone line between them.

This line is **bidirectional** — both the device and the backend can talk, even at the same time. Like two people on a phone call who can both speak at once. The line stays open until the connection is closed.

## What do they say once the line is up?

This is where the JSON arrays come in. Everything they send over this line is just plain text — like a paragraph someone types. But the format of that text is fixed: it's a JSON array.

They can form three kinds of sentences:

- **"I'm asking you something / telling you something, and I expect a reply"** → this is a **CALL**, starts with `2`.
- **"Here's the answer to what you just asked"** → this is a **CALLRESULT**, starts with `3`.
- **"I couldn't do what you asked, there's an error"** → this is a **CALLERROR**, starts with `4`.

## A concrete dialogue

The device just powered on and connected to the backend. Its first job is to introduce itself. It sends a sentence like this:

```json
[2, "abc-001", "BootNotification", {"chargePointVendor": "Acme", "chargePointModel": "X9"}]
```

In plain language: "(2) I'm sending you a request, (abc-001) this is the request's tracking number, (BootNotification) the topic is: I just powered on, introducing myself, (payload) my vendor is Acme and my model is X9."

The backend hears this, processes it, and replies **using the same tracking number**:

```json
[3, "abc-001", {"currentTime": "2026-06-02T10:00:00Z", "interval": 300, "status": "Accepted"}]
```

In plain language: "(3) This is a reply, (abc-001) it's the answer to what you asked under abc-001, (payload) the current time is X, poll me every 300 seconds, and I accept you — you may operate."

## The most critical point: the tracking number

The reply does **not** contain the topic name. There's no "BootNotification" inside `[3, ...]`. So how does the backend know what this reply is a reply *to*? Only from that tracking number (`abc-001`).

This is the logic in your code: when you send a CALL, you set its tracking number aside — you keep it in a waiting list that says "I sent a BootNotification under abc-001, its reply hasn't arrived yet." When the reply comes back with `abc-001`, you find that entry in the list, conclude "OK, this is the reply to my BootNotification," and remove it from the list. This list is usually called the **pending request map**.

## The matter of direction

The thing people most often get confused about: the numbers `2/3/4` tell you the **type of the sentence, not who sent it**.

So a CALL (`2`) goes both from device to backend, and from backend to device. Examples:

**Device → backend CALL:** "I powered on" (BootNotification), "someone plugged in a cable" (StatusNotification), "here's the current meter reading" (MeterValues), "is this card valid?" (Authorize).

**Backend → device CALL:** "start charging for this user" (RemoteStartTransaction), "reset yourself" (Reset), "change this setting" (ChangeConfiguration).

So both sides can ask questions, and both sides answer. The line is fully bidirectional. Whoever sends a CALL, the other side returns a CALLRESULT to it.

## The whole flow in one sentence

The device powers on → it connects to the backend over WebSocket → they talk over the open line using JSON arrays → every request carries `2` + a tracking number → every reply carries `3` + the **same** tracking number → thanks to the tracking number, each reply is matched to its request → this continues until the line is closed.

## Quick reference: message frame shapes

| Type | MessageTypeId | Shape |
|------|---------------|-------|
| CALL (request) | `2` | `[2, <UniqueId>, <Action>, <Payload>]` |
| CALLRESULT (response) | `3` | `[3, <UniqueId>, <Payload>]` |
| CALLERROR (error) | `4` | `[4, <UniqueId>, <errorCode>, <errorDescription>, <errorDetails>]` |

A few notes:

- `<UniqueId>` is the tracking number that ties a request to its reply.
- Action names are **PascalCase** and case-sensitive: `"BootNotification"`, not `"bootnotification"`.
- The RPC framing above is identical in OCPP 1.6 and 2.0.1. Only the payload schemas and the set of available actions differ between versions.