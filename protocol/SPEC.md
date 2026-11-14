# ClipConnect Wire Protocol v1

**Status:** v1 — text only, one paired phone, one laptop, same LAN.

---

## Transport

- **Protocol:** TCP
- **Default port:** 48123
- **Roles:** laptop = server, phone = client (phone dials and redials)

---

## Pairing

### Secret

32 random bytes from `crypto/rand`.

### QR payload

```
clipconnect://pair?v=1&k=<base64url secret>&h=<ip1,ip2>&p=48123&n=<hostname>
```

- `v` — protocol version (1)
- `k` — base64url-encoded secret (no padding)
- `h` — comma-separated private LAN IPs (fallback if mDNS fails)
- `p` — port
- `n` — laptop hostname

### mDNS

- Service name: `_clipconnect._tcp`
- TXT record: `id=<first 4 bytes of HMAC-SHA256(secret, "mdns-id"), hex>`
- The secret is never advertised.

---

## Handshake

Plaintext exchange, then all frames encrypted with AES-256-GCM.

1. **client → server:** `CCN1` (4 bytes ASCII) + `clientNonce` (16 random bytes)
2. **server → client:** `serverNonce` (16 random bytes)
3. **Key derivation** (HKDF-SHA256, ikm = secret):
   - `k_c2s` = HKDF(salt = clientNonce‖serverNonce, info = `"clipconnect v1 c2s"`, 32 bytes)
   - `k_s2c` = HKDF(salt = clientNonce‖serverNonce, info = `"clipconnect v1 s2c"`, 32 bytes)
4. Each side sends an encrypted `{"t":"hello","v":1}` frame first. If the first frame fails to decrypt, close silently.

---

## Frame Format

```
[4 bytes: big-endian length N][N bytes: AES-256-GCM ciphertext‖16-byte tag]
```

- **AAD:** the 4 length bytes
- **Nonce:** `0x00000000` (4 bytes) ‖ `counter` (8 bytes big-endian), counter starts at 0 per direction
- Counter increases by exactly 1 per frame. Mismatch → close connection.
- **Max frame:** plaintext 1 MiB. Reject length field above 1 MiB + 16 before allocating.

---

## Messages (UTF-8 JSON)

| Type | Direction | Schema |
|------|-----------|--------|
| `hello` | Both (handshake) | `{"t":"hello","v":1}` |
| `clip` | Either | `{"t":"clip","text":"...","seq":N}` |
| `ack` | Receiver | `{"t":"ack","seq":N}` |
| `ping` | Either | `{"t":"ping"}` |
| `pong` | Either | `{"t":"pong"}` |

- `seq` — unsigned 64-bit integer, monotonic per sender. Starts at 1, increments by 1 per clip frame.
- `ack` — sent by the receiver after successfully applying a clip. Contains the `seq` of the acknowledged clip.
- **Clip deduplication:** receiver ignores clips whose `text` hash matches a previously applied clip.
- **Unacked clip:** if no ack is received within 3 seconds or the socket is down, the sender keeps ONLY the latest unacked clip and resends it after reconnect.

---

## Limits

- Handshake timeout: 5 seconds
- Max unauthenticated connections: 4
- Max clip text: 256 KiB
- Ping interval: 25 seconds
- Link dead after: 60 seconds silence
- Reconnect backoff (phone): 1 s → 30 s exponential
- Ack timeout: 3 seconds
