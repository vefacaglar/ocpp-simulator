#!/bin/sh
# wire-dump: bring up a temporary mosquitto_sub, run a fake
# charge point against the running v4.3 stack, and capture
# the raw OCPP-J frames on ocpp/+/in and ocpp/+/out to a log
# file under docs/wire-dumps/.
#
# Requires: backend stack already running (make dev or make
# dev-stack), and the host's docker daemon can reach the
# ocpp-simulator_default network.

set -eu

DUMPS_DIR="docs/wire-dumps"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$DUMPS_DIR/wire-dump-$STAMP.log"
mkdir -p "$DUMPS_DIR"

cleanup() {
  docker rm -f wire-sub 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Start the subscriber in the compose network so it can
# resolve the mqtt service name.
docker run --rm -d --name wire-sub --network ocpp-simulator_default \
  eclipse-mosquitto:2 \
  sh -c "mosquitto_sub -h mqtt -t 'ocpp/+/in' -t 'ocpp/+/out' -v > /tmp/wire.log 2>&1"
sleep 1

# Run the fake CP. It connects to ws://ocpp-gateway:7080/ws/CP-WIRE-DUMP,
# exchanges BootNotification / Heartbeat / StatusNotification,
# and finally sends a TotallyUnknownAction so the
# message-processor's NotImplemented CALLERROR path is also
# captured.
docker run --rm --network ocpp-simulator_default \
  -v "$(pwd)/scripts:/scripts" \
  python:3.12-alpine \
  sh -c "pip install --quiet websockets && python3 /scripts/fake_cp.py CP-WIRE-DUMP" \
  || true

# Give the subscriber a moment to drain.
sleep 1

# Pull the captured frames out of the subscriber container and
# append them to the dated log file. The script does not fail
# if the file is empty (e.g. broker was unreachable); that
# just produces an empty log.
docker exec wire-sub cat /tmp/wire.log 2>/dev/null > "$OUT" || true

echo "Wrote: $OUT"
echo "----- captured frames -----"
cat "$OUT"
echo "----- end -----"
