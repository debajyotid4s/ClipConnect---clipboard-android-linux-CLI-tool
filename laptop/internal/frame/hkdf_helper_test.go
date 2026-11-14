package frame

import (
	"crypto/hmac"
	"crypto/sha256"
)

func hkdfSha256(ikm, salt []byte, info string) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	prk := mac.Sum(nil)
	n := (32 + 31) / 32
	okm := []byte{}
	block := []byte{}
	for i := 0; i < n; i++ {
		h := hmac.New(sha256.New, prk)
		input := append(append(append([]byte{}, block...), []byte(info)...), byte(i+1))
		h.Write(input)
		block = h.Sum(nil)
		okm = append(okm, block...)
	}
	return okm[:32]
}
