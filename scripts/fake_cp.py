"""Fake OCPP 1.6J charge point used to verify that the v4.3
stack carries raw OCPP-J frames on every hop. Connects to the
local ocpp-gateway, sends BootNotification + Heartbeat, then
stays connected a few seconds so any inbound RemoteStart from
ocpp-core can be answered. Finally sends an unknown action so
the message-processor's NotImplemented CALLERROR path is
exercised.
"""
import asyncio
import json
import random
import sys
import websockets


def uid() -> str:
    return format(random.getrandbits(128), "032x")


async def main(cp_id: str = "CP-WIRE") -> None:
    url = f"ws://ocpp-gateway:7080/ws/{cp_id}"
    sub = ["ocpp1.6", "ocpp2.0.1"]
    async with websockets.connect(url, subprotocols=sub) as ws:
        negotiated = ws.subprotocol
        print(f"[{cp_id}] connected negotiated subprotocol={negotiated}", flush=True)

        # BootNotification
        u = uid()
        boot = [2, u, "BootNotification", {
            "chargePointVendor": "DockerWireTest",
            "chargePointModel": "OCPP-Sim-1.6J",
        }]
        await ws.send(json.dumps(boot))
        print(f"[{cp_id}] sent BootNotification uid={u}", flush=True)
        resp = await asyncio.wait_for(ws.recv(), timeout=10)
        print(f"[{cp_id}] recv Boot.conf: {resp}", flush=True)

        # Heartbeat
        u = uid()
        await ws.send(json.dumps([2, u, "Heartbeat", {}]))
        print(f"[{cp_id}] sent Heartbeat uid={u}", flush=True)
        resp = await asyncio.wait_for(ws.recv(), timeout=5)
        print(f"[{cp_id}] recv Heartbeat.conf: {resp}", flush=True)

        # StatusNotification
        u = uid()
        await ws.send(json.dumps([2, u, "StatusNotification", {
            "connectorId": 1,
            "errorCode": "NoError",
            "status": "Available",
            "timestamp": "2026-06-03T15:00:00Z",
        }]))
        print(f"[{cp_id}] sent StatusNotification uid={u}", flush=True)
        try:
            resp = await asyncio.wait_for(ws.recv(), timeout=5)
            print(f"[{cp_id}] recv Status.conf: {resp}", flush=True)
        except asyncio.TimeoutError:
            print(f"[{cp_id}] no StatusNotification response (timeout)", flush=True)

        # Wait so an inbound RemoteStart from ocpp-core (if any) can be answered
        await asyncio.sleep(3)

        # Unknown action → expect NotImplemented CALLERROR from message-processor
        u = uid()
        await ws.send(json.dumps([2, u, "TotallyUnknownAction", {"x": 1}]))
        try:
            resp = await asyncio.wait_for(ws.recv(), timeout=5)
            print(f"[{cp_id}] recv unknown-action response: {resp}", flush=True)
        except asyncio.TimeoutError:
            print(f"[{cp_id}] no unknown-action response (timeout)", flush=True)


if __name__ == "__main__":
    cp_id = sys.argv[1] if len(sys.argv) > 1 else "CP-WIRE"
    asyncio.run(main(cp_id))
