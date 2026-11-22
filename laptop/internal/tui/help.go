package tui

// helpText is the help/troubleshooting screen shown in the TUI help tab.
const helpText = `ClipConnect — Android↔Linux synced clipboard

Install (Ubuntu):
  sudo apt install clipconnect

Pair an Android phone:
  1. Install the ClipConnect APK on your phone.
  2. Run "clipconnect" and choose "Pair Android device",
     or run "clipconnect pair" directly.
  3. Scan the QR code shown in the terminal.
  4. Keep both devices on the SAME Wi-Fi/LAN.

How it works:
  Copy text on either device and it appears on the other
  within ~2 seconds. Fully automatic — no send button.

Port:
  The daemon listens on TCP 10369. The firewall (ufw) is
  opened automatically on first pair (one sudo prompt).

Clipboard requirements (laptop):
  X11:  xclip or xsel (package xclip / xsel)
  Wayland (wlroots/Sway/Hyprland): wl-clipboard (wl-paste/wl-copy)
  KDE Plasma: wl-clipboard
  GNOME (Wayland): background clipboard reads are not
  supported by the shell; use XWayland or a later phase.

Android permissions / background:
  Some OEMs (MIUI, Xiaomi, Samsung) kill background apps.
  Allow auto-launch, background running, and battery
  "no restrictions" for ClipConnect in Settings.

Logs:
  clipconnect logs            show recent logs
  clipconnect logs --follow   follow in real time
  clipconnect logs --lines N  show N lines

Unpair / re-pair:
  clipconnect unpair          remove credentials (add --yes to skip confirm)
  Re-pairing generates a new secret and revokes old phones.

Security:
  End-to-end encrypted (AES-256-GCM). The QR is effectively
  a password: do not share it. No telemetry, no internet.
`
