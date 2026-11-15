package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// helpCmd is `clipconnect help`.
var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help about ClipConnect",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(helpText)
	},
}

func init() {
	rootCmd.AddCommand(helpCmd)
}

const helpText = `ClipConnect — Android↔Linux synced clipboard

INSTALL (Ubuntu)
  sudo apt install clipconnect

START THE MENU
  clipconnect
  Then choose: Pair Android device, Unpair, View logs, or Help.

PAIR AN ANDROID PHONE
  clipconnect pair
  1. Install the ClipConnect APK on your phone.
  2. Scan the QR code shown in the terminal.
  3. Keep both devices on the SAME Wi-Fi/LAN.
  Copy text on either device and it appears on the other within ~2s.
  Fully automatic — no send button.

PORT
  The daemon listens on TCP 10369. ufw is opened automatically on first
  pair (one sudo prompt).

CLIPBOARD REQUIREMENTS (LAPTOP)
  X11:        xclip or xsel  (packages: xclip / xsel)
  Wayland     wl-clipboard   (packages: wl-clipboard; wl-paste/wl-copy)
  KDE Plasma: wl-clipboard
  GNOME (Wayland): background clipboard reads are not supported by the
                   shell; use XWayland or a later phase.

ANDROID PERMISSIONS / BACKGROUND
  Some OEMs (MIUI, Xiaomi, Samsung) kill background apps. In Settings allow:
  - Auto-launch and background running for ClipConnect
  - Battery saver → "no restrictions"
  - Keep the foreground notification running.

LOGS
  clipconnect logs                 show recent logs
  clipconnect logs --follow        follow in real time
  clipconnect logs --lines 100     show 100 lines

UNPAIR / RE-PAIR
  clipconnect unpair               remove credentials (add --yes to skip confirm)
  Re-pairing generates a new secret and revokes previously paired phones.

SECURITY
  End-to-end encrypted (AES-256-GCM, HKDF-SHA256). The QR is effectively a
  password — do not share it. No telemetry, no analytics, no internet use.
`
