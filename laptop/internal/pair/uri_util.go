package pair

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
)

// encodeSecret base64url-encodes bytes without padding.
func encodeSecret(src []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(src), "=")
}

// decodeSecret base64url-decodes a no-padding string to bytes.
func decodeSecret(src string) ([]byte, error) {
	if src == "" {
		return nil, fmt.Errorf("empty secret")
	}
	padding := len(src) % 4
	if padding != 0 {
		src += strings.Repeat("=", 4-padding)
	}
	dec, err := base64.URLEncoding.DecodeString(src)
	if err != nil {
		return nil, err
	}
	return dec, nil
}

// parseIPs splits a comma-separated host list and keeps only valid IP literals.
func parseIPs(s string) ([]string, error) {
	var ips []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ip := net.ParseIP(part)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP: %q", part)
		}
		ips = append(ips, part)
	}
	return ips, nil
}
