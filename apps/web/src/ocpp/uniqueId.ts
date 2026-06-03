// uniqueId is the OCPP-J uniqueId for a CALL. Per spec the id is
// an opaque string and just needs to be unique within the
// charge point's session. We use a 32-char hex derived from
// crypto.randomUUID for entropy and brevity.
export function uniqueId(): string {
  // crypto.randomUUID is available in all modern browsers and
  // returns a canonical RFC 4122 UUIDv4. Strip the dashes for
  // a 32-char hex string.
  return crypto.randomUUID().replace(/-/g, '')
}
