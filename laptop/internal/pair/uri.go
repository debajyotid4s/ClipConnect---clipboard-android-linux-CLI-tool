package pair

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// PairInfo holds the parsed pairing information.
type PairInfo struct {
	Version  int
	Secret   []byte
	IPs      []string
	Port     int
	Hostname string
}

const (
	// supportedVersion is the only wire version this build understands.
	supportedVersion = 1
	// secretLen is the required length (bytes) of the pairing secret.
	secretLen = 32
	// defaultPort is used only when a URI omits the port; real URIs always
	// carry p=10369.
	defaultPort = 10369
)

// ParseURI parses a clipconnect://pair URI and returns pairing info. It enforces
// version 1, a 32-byte secret, a valid port in [1,65535], and at least one valid
// IP address. It builds the query with url.Values so host/IP data is escaped.
func ParseURI(raw string) (*PairInfo, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse uri: %w", err)
	}
	if u.Scheme != "clipconnect" {
		return nil, fmt.Errorf("bad scheme: %s", u.Scheme)
	}
	if u.Host != "pair" {
		return nil, fmt.Errorf("bad host: %s", u.Host)
	}

	q := u.Query()

	// v = version: must be exactly 1.
	if q.Get("v") != strconv.Itoa(supportedVersion) {
		return nil, fmt.Errorf("unsupported version: %q", q.Get("v"))
	}

	// k = secret (base64url, no padding): must decode to exactly 32 bytes.
	secret, err := decodeSecret(q.Get("k"))
	if err != nil {
		return nil, fmt.Errorf("decode secret: %w", err)
	}
	if len(secret) != secretLen {
		return nil, fmt.Errorf("secret length = %d, want %d", len(secret), secretLen)
	}

	// h = IPs (comma-separated): must contain at least one valid IP.
	ips, err := parseIPs(q.Get("h"))
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no valid IPs")
	}

	// p = port: must parse to a valid port; fall back to default only if absent.
	port := defaultPort
	if p := q.Get("p"); p != "" {
		port, err = strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port: %q", p)
		}
	}

	return &PairInfo{
		Version:  supportedVersion,
		Secret:   secret,
		IPs:      ips,
		Port:     port,
		Hostname: q.Get("n"),
	}, nil
}

// BuildURI creates a pairing URI from the given info. The query is built with
// url.Values so the secret, IPs, and hostname are properly escaped.
func BuildURI(secret []byte, ips []string, port int, hostname string) string {
	v := url.Values{}
	v.Set("v", strconv.Itoa(supportedVersion))
	v.Set("k", encodeSecret(secret))
	v.Set("h", strings.Join(ips, ","))
	v.Set("p", strconv.Itoa(port))
	v.Set("n", hostname)
	return "clipconnect://pair?" + v.Encode()
}
