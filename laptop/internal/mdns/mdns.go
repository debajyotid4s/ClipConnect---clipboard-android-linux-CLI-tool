package mdns

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/libp2p/zeroconf/v2"
)

// Advertiser publishes the _clipconnect._tcp service on the local network.
type Advertiser struct {
	server         *zeroconf.Server
	secret         []byte
	hostname       string
	logger         *slog.Logger
	advertisedPort int
}

// NewAdvertiser creates a new mDNS advertiser.
// secret is used to generate the service ID (never advertised directly).
func NewAdvertiser(secret []byte, port int, hostname string, ifaces []net.Interface, logger *slog.Logger) (*Advertiser, error) {
	id := mdnsId(secret)

	// TXT records: only the ID, never the secret
	txt := []string{"id=" + id}

	server, err := zeroconf.Register(hostname, "_clipconnect", "local.", port, txt, ifaces, nil)
	if err != nil {
		return nil, fmt.Errorf("mdns register: %w", err)
	}

	logger.Info("mDNS service advertised", "name", hostname, "port", port, "id", id)

	return &Advertiser{
		server:   server,
		secret:   secret,
		hostname: hostname,
		logger:   logger,
	}, nil
}

// Run starts the mDNS advertiser. It blocks until ctx is done.
func (a *Advertiser) Run(ctx context.Context) {
	<-ctx.Done()
	a.Stop()
}

// AdvertisePort updates the TXT record with the real port after binding.
func (a *Advertiser) AdvertisePort(port int) {
	if a.server == nil {
		return
	}
	id := mdnsId(a.secret)
	txt := []string{"id=" + id, "port=" + strconv.Itoa(port)}
	a.server.SetText(txt)
	a.advertisedPort = port
	a.logger.Info("mDNS port advertised", "port", port)
}

// Port returns the last port advertised via the TXT record.
func (a *Advertiser) Port() int {
	return a.advertisedPort
}

// Stop stops the mDNS advertiser.
func (a *Advertiser) Stop() {
	if a.server != nil {
		a.server.Shutdown()
	}
}

// mdnsId returns the first 4 bytes of HMAC-SHA256(secret, "mdns-id") as hex.
// This is advertised in the TXT record; the secret itself is never advertised.
func mdnsId(secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte("mdns-id"))
	digest := h.Sum(nil)
	return hex.EncodeToString(digest[:4])
}

// Discoverer looks for _clipconnect._tcp services on the local network.
type Discoverer struct {
	// Found is called for each discovered service.
	Found func(host string, port int, id string)
	logger *slog.Logger
}

// NewDiscoverer creates a new mDNS discoverer.
func NewDiscoverer(logger *slog.Logger) *Discoverer {
	return &Discoverer{logger: logger}
}

// Browse searches for _clipconnect._tcp services. It blocks until ctx is done.
func (d *Discoverer) Browse(ctx context.Context) error {
	entriesCh := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entriesCh {
			d.logger.Info("mDNS service discovered", "host", entry.HostName, "port", entry.Port)
			id := extractId(entry.Text)
			if id != "" {
				d.Found(entry.HostName, entry.Port, id)
			}
		}
	}()

	return zeroconf.Browse(ctx, "_clipconnect", "local.", entriesCh, nil)
}

// Stop stops the mDNS discoverer.
func (d *Discoverer) Stop() {
	// Browse runs until ctx is done; no explicit stop needed
}

// extractId extracts the id field from TXT records.
func extractId(txt []string) string {
	for _, field := range txt {
		if strings.HasPrefix(field, "id=") {
			return strings.TrimPrefix(field, "id=")
		}
	}
	return ""
}

// PickInterfaces selects suitable network interfaces for mDNS.
// Prefers interfaces that are up, not loopback, and support multicast.
func PickInterfaces() []net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var selected []net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		selected = append(selected, iface)
	}
	return selected
}
