# ClipConnect

**Apple ecosystem-alike, bidirectional clipboard sync between a Linux device and an Android device.**

ClipConnect automatically shares copied text between a Linux laptop and an Android phone over the local network. Pair once by scanning a QR code, then copy on either device and paste on the other without manually sending each clip.

> ClipConnect is designed for local, private clipboard sharing. It does not require an internet connection and does not send clipboard contents to a cloud service.

## Features

- Bidirectional clipboard synchronization between Linux and Android
- One-time QR pairing with manual connection fallback
- Automatic reconnect with exponential backoff
- AES-256-GCM encryption with HKDF-derived session keys
- Echo protection to prevent clipboard feedback loops
- Linux support for Wayland and X11 clipboard backends
- Fixed TCP port for stable pairing and firewall configuration
- Shared protocol test vectors used by Go and Kotlin implementations
- No telemetry, analytics, or clipboard plaintext logging

## Architecture

```text
Linux clipboard  <->  Go daemon  <== encrypted TCP ==>  Android app  <->  Android clipboard
                         |                                  |
                    Wayland/X11                         ClipboardManager
```

The project is organized into three components:

- `laptop/` — Go CLI and background daemon
- `android/` — Kotlin Android application
- `protocol/` — wire protocol specification and cross-platform test vectors

The daemon polls the Linux clipboard and sends new text to the phone. The Android app reads clipboard changes through its local clipboard IPC service and sends them back to the daemon. Incoming clips are applied with an echo guard so they are not immediately sent back in the opposite direction.

## Requirements

### Linux

- Linux desktop with either:
  - Wayland: `wl-paste` and `wl-copy`
  - X11: `xclip` or `xsel`
- Go toolchain for building from source
- Linux laptop and Android phone reachable on the same LAN, or through compatible USB tethering

### Android

- Android device with the ClipConnect APK installed
- Permission to run the app continuously; some manufacturers may require disabling battery optimization

## Quick start

### 1. Build the Linux CLI

```bash
cd laptop
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o clipconnect ./cmd/clipconnect/
```

For ARM64 Linux, use `GOARCH=arm64` instead.

### 2. Pair the phone

Start the daemon:

```bash
./clipconnect daemon
```

The daemon generates and displays a QR code. Scan it from the ClipConnect Android app. The pairing secret is stored locally at:

```text
~/.config/clipconnect/secret
```

The secret is created with restrictive permissions and remains stable across daemon restarts. To generate or replace pairing credentials explicitly:

```bash
./clipconnect pair
```

The daemon listens on TCP port `10369`. If automatic firewall setup is unavailable, allow that port manually:

```bash
sudo ufw allow 10369/tcp
```

### 3. Use clipboard sync

After pairing, copy text on either device. It should appear on the other device automatically, normally within about one second.

Useful commands:

```bash
./clipconnect status
./clipconnect unpair
./clipconnect setup-phone
```

## Android build

From the `android/` directory:

```bash
./gradlew testDebugUnitTest
./gradlew assembleDebug
```

Install the generated debug APK on the phone, open ClipConnect, and scan the QR code printed by the Linux daemon.

## Protocol and security

ClipConnect uses a custom framed protocol documented in [`protocol/SPEC.md`](protocol/SPEC.md). The connection includes:

- A CCN1 handshake
- HKDF key derivation
- AES-256-GCM authenticated encryption
- Per-message counter-based nonces
- Acknowledgements and ping/pong liveness frames
- Shared conformance vectors in [`protocol/vectors.json`](protocol/vectors.json)

Clipboard text is not included in normal daemon logs; diagnostic logs record metadata such as message length instead. Pairing data is stored locally, and communication is intended for trusted local networks.

## Network and device limitations

- Both devices must be able to reach the Linux host on the same network path.
- Check that the Linux firewall permits TCP `10369`.
- On some MIUI/HyperOS devices, background clipboard access is restricted. Keep ClipConnect foregrounded or exempt it from battery optimization for reliable continuous sync.
- On X11, another `xclip` process holding the clipboard lock can prevent an incoming phone clip from being written. Wayland uses `wl-copy` and is generally unaffected by this issue.

## Development

Run the Linux tests and checks:

```bash
cd laptop
go test ./...
go vet ./...
```

The protocol directory contains the specification and cross-platform vectors used by both implementations.

## Project status

The core bidirectional sync path, encrypted transport, pairing flow, reconnect behavior, and automated tests are implemented. Planned improvements include stronger X11 paste fallback behavior, Android UI tests, and optional phone-side discovery improvements.

## License

No license file is currently included in this repository. Add or consult a project license before redistributing ClipConnect.