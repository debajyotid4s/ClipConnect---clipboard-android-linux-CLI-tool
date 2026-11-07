package pair

import (
	"encoding/base64"
	"fmt"
	"net/url"
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

// ParseURI parses a clipconnect://pair URI and returns pairing info.
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

	// v = version
	v := q.Get("v")
	if v != "1" {
		return nil, fmt.Errorf("unsupported version: %s", v)
	}

	// k = secret (base64url, no padding)
	k := q.Get("k")
	secret := make([]byte, base64URLEncodedLen(len(k)))
	decoded, err := base64URLDecode(secret, k)
	if err != nil {
		return nil, fmt.Errorf("decode secret: %w", err)
	}
	secret = decoded

	// h = IPs (comma-separated)
	h := q.Get("h")
	var ips []string
	if h != "" {
		for _, ip := range strings.Split(h, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				ips = append(ips, ip)
			}
		}
	}

	// p = port
	p := q.Get("p")
	port := 48123
	if p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	// n = hostname
	n2 := q.Get("n")

	return &PairInfo{
		Version:  1,
		Secret:   secret,
		IPs:      ips,
		Port:     port,
		Hostname: n2,
	}, nil
}

// BuildURI creates a pairing URI from the given info.
func BuildURI(secret []byte, ips []string, port int, hostname string) string {
	k := base64URLEncode(secret)
	h := strings.Join(ips, ",")
	return fmt.Sprintf("clipconnect://pair?v=1&k=%s&h=%s&p=%d&n=%s",
		k, h, port, hostname)
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(src []byte) string {
	encoded := base64.URLEncoding.EncodeToString(src)
	return strings.TrimRight(encoded, "=")
}

// base64URLEncodedLen returns the decoded length of a base64url-encoded string.
func base64URLEncodedLen(encodedLen int) int {
	return encodedLen * 6 / 8
}

// base64URLDecode decodes a base64url-encoded string (no padding).
func base64URLDecode(dst []byte, src string) ([]byte, error) {
	// Add padding
	padding := len(src) % 4
	if padding != 0 {
		src += strings.Repeat("=", 4-padding)
	}
	n, err := base64.URLEncoding.Decode(dst, []byte(src))
	if err != nil {
		return nil, err
	}
	return dst[:n], nil
}
