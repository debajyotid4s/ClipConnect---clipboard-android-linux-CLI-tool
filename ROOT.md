# ClipConnect — Root Reference

Automated two-way clipboard sync between an Android phone and a Linux laptop over the LAN. The phone scans a laptop QR once, connects over TCP, and then both directions stay in sync automatically.

- **Laptop:** Go CLI/daemon (`laptop/`).
- **Phone:** Kotlin Android app, single module (`android/`).
- **Protocol:** shared wire spec (`protocol/SPEC.md`, `protocol/vectors.json`).

---

## Goal

Copy text on one device → it appears on the other within ~1 second, with no manual tap per copy. Pair once (QR), then sync continuously in both directions. No telemetry, no analytics, no internet access, no logging of clipboard plaintext or secrets.

---

## What works

- **Pairing:** `clipconnect pair` generates a QR (IP + fixed port + secret). The phone pairs by scanning the QR or by manual entry (IP + secret). Secret is generated once and persisted (`~/.config/clipconnect/secret`, mode `0600`), so the QR stays stable across restarts.
- **Fixed port:** the daemon always binds `0.0.0.0:10369` and persists the port so the QR and the phone's saved port stay in lockstep across restarts and fresh installs.
- **Laptop → Phone (incoming):** the daemon polls the laptop clipboard every 500 ms and sends each new clip. The phone's `TcpClient` read loop delivers clips; `IncomingClipApplier` writes them to the system clipboard (echo-guarded). **Verified working.**
- **Phone → Laptop (outgoing):** the app runs an in-process IPC server (`ClipboardIpcServer`, loopback TCP `127.0.0.1:39876`) that reads the phone clipboard via the public `ClipboardManager` API. A `ClipboardManager.OnPrimaryClipChangedListener` fires on every copy and sends the changed clip immediately (deduped by SHA-256); an 800 ms poll thread (`ConnectionService`) remains as a fallback in case MIUI suppresses the listener while backgrounded. **Verified working** (frames received by the daemon, `received clip` logged).
- **Handshake & encryption:** CCN1 handshake with HKDF-derived keys, AES-256-GCM with a per-message counter nonce, echo guard to prevent feedback loops. Conformance test vectors live in `protocol/vectors.json`.
- **Reconnect:** the phone redials with exponential backoff (1 s → 30 s).
- **Tests:** Go (`go test ./...`) and Android (`./gradlew testDebugUnitTest`) both pass.

---

## Issues / open problems

1. **Laptop paste can fail under X11.** When the daemon writes an incoming clip with `xclip`, a stuck or concurrent `xclip` instance can hold the clipboard lock and the write returns `exit status 1`. `X11Backend.Set` now bounds the call with a 3 s timeout and retries once, but a hung `xclip` still leaves the paste unsuccessful until the lock is freed. **On Wayland (`wl-copy`) this path is clean.**
2. **MIUI blocks background clipboard reads / kills the process.** MIUI (HyperOS) skips background broadcasts, restricts background app activity, and force-kills the process when the main thread stalls. Two parts to the fix:
   - **ANR (our bug):** a 10 s main-thread poll loop in `MainActivity` (`for(i in 0 until 20){ delay(500) }`) plus clipboard IPC on the compose UI thread caused an `Input dispatching timed out (5000ms)` ANR, which is what MIUI then killed. Fixed: the UI now observes `ConnectionService.connectionState` via a `StateFlow` (no polling), and `IncomingClipApplier.apply()` runs on `Dispatchers.IO`. See `docs/FINDINGS.md` (ANR backtrace + exit-reason codes).
   - **Background kill (device policy):** MIUI still freezes/kills the process when the screen is off or the app is backgrounded. The app ships a **Survival Checklist** (see the app UI) that reports state and deep-links to Autostart, Battery saver = No restrictions, Permissions, and falls back to App info. "Lock in Recents" and "Pause app activity if unused" are manual. These settings cannot be flipped silently (no safe API) — the user must confirm. Verified deep-links resolve on POCO F5 / HyperOS / Android 15.
3. **Network must be aligned.** The phone and laptop must be on the same LAN subnet and reachable (same WiFi, or USB tethering on both). The laptop must actually listen on the interface the phone reaches — a laptop on Ethernet-only while the phone is on WiFi will not connect. Confirm with `ping` both ways before trusting the app.
4. **No Android UI tests / no README at repo root** (see *Yet to do*).

---

## Yet to be done

- **README.md at repo root** (currently git-ignored; `ROOT.md` serves as the overview).
- **Stable paste under X11** — add a fallback writer (e.g. `wl-copy` on Wayland, `xsel`, or a short retry/backoff that reaps stuck `xclip` processes) so `phone → laptop` paste never silently fails.
- **Android unit/UI tests** beyond the crypto/clip/network tests already present.
- **Soak test on device:** survival at 5 min / 30 min / 1 h (screen off), after unlock, and after reboot — recording the exit reason for every death. Pending: owner applies the Survival Checklist settings first.
- **mDNS auto-discovery on the phone** — currently the phone connects to the QR/port directly (reliable); auto-discovery was dropped after MIUI's `NsdManager` rejected the service type.
- **Regenerate `protocol/vectors.json`** from current code (a `cmd/genvectors` helper was removed; vectors can be regenerated by hand or re-added as a dev tool).

---

## Technologies & decisions (per topic)

| Topic | Technology | Decision / why |
|-------|-----------|----------------|
| Laptop backend | `xclip`/`xsel` (X11) or `wl-paste`/`wl-copy` (Wayland) via `os/exec` | stdlib `exec.CommandContext` with fixed arg vectors; never a shell string; clipboard text never in args. |
| Laptop bind | `0.0.0.0:10369` (IPv4 wildcard, fixed port) | `0.0.0.0` (not `:0`) so IPv4 LAN connections work; fixed port so the QR and phone stay stable. |
| Encryption | AES-256-GCM + HKDF, per-message counter nonce | Authenticated encryption; echo guard prevents re-delivering a applied clip. |
| Wire protocol | Custom framed protocol (`protocol/SPEC.md`) | Length-prefixed frames; ack for delivery, ping/pong for liveness. |
| Test vectors | `protocol/vectors.json` consumed by Go and Kotlin | Protocol conformance tests share one vector set across both platforms. |
| Phone clipboard read (outgoing) | In-process IPC (`ClipboardIpcServer`/`ClipboardIpcClient`, loopback TCP) | Reads the clipboard via the public `ClipboardManager` API (API 29+). No ADB, no Shell UID 2000, no `app_process` — works on any device. Replaced the abandoned ADB/Shell *ClipBridge* design. |
| Phone clipboard write (incoming) | `ClipboardManager.setPrimaryClip` | Public API; same path as the IPC read. Echo-guarded via `ClipboardDedupe`. |
| Continuous outgoing poll | `OnPrimaryClipChangedListener` + fallback poll (`ConnectionService`) | Event-driven send on copy; 800 ms poll as MIUI fallback. Dedupes by SHA-256. |
| Connection | Raw `java.net.Socket`, no networking libraries | Minimal surface; manual handshake + reconnect with exponential backoff. |
| Crypto (phone) | `javax.crypto`, custom HKDF/AES-GCM | No external crypto libraries; matches the laptop's derivation. |
| Pairing UI | QR scan (quickie library) + manual IP/secret entry | Direct TCP connection is reliable where mDNS discovery is not. |
| Firewall | `sudo ufw allow 10369/tcp` (auto-run on first start, once) | Auto-adds the ufw rule via `/dev/tty` password prompt on a fresh host; detects an existing rule and skips the prompt afterward. Best-effort — prints manual instructions if sudo is unavailable. |

---

## Build & run

```bash
# Go (from laptop/)
cd laptop
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o clipconnect ./cmd/clipconnect/
go vet ./...
go test ./...

# Android (from android/)
./gradlew testDebugUnitTest
./gradlew assembleDebug

# Run the daemon (prints QR, opens ufw once)
./clipconnect daemon
```
