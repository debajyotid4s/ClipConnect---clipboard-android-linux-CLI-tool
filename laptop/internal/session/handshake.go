package session

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

// handshake does the CCN1 + nonce exchange + HKDF key derivation.
func handshake(conn net.Conn, secret []byte, isClient bool) (*Session, error) {
	if err := conn.SetDeadline(time.Now().Add(handshakeTO)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	var clientNonce, serverNonce [nonceLen]byte
	if _, err := io.ReadFull(rand.Reader, clientNonce[:]); err != nil {
		return nil, fmt.Errorf("generate client nonce: %w", err)
	}

	if isClient {
		// Client: write CCN1 + clientNonce
		if _, err := conn.Write([]byte(magic)); err != nil {
			return nil, fmt.Errorf("client write magic: %w", err)
		}
		if _, err := conn.Write(clientNonce[:]); err != nil {
			return nil, fmt.Errorf("client write nonce: %w", err)
		}
		// Client: read serverNonce
		if _, err := io.ReadFull(conn, serverNonce[:]); err != nil {
			return nil, fmt.Errorf("client read server nonce: %w", err)
		}
	} else {
		// Server: read CCN1 + clientNonce
		magicBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, magicBuf); err != nil {
			return nil, fmt.Errorf("server read magic: %w", err)
		}
		if !bytes.Equal(magicBuf, []byte(magic)) {
			return nil, fmt.Errorf("bad magic: %q", magicBuf)
		}
		if _, err := io.ReadFull(conn, clientNonce[:]); err != nil {
			return nil, fmt.Errorf("server read client nonce: %w", err)
		}
		// Server: generate serverNonce and write it
		if _, err := io.ReadFull(rand.Reader, serverNonce[:]); err != nil {
			return nil, fmt.Errorf("server generate nonce: %w", err)
		}
		if _, err := conn.Write(serverNonce[:]); err != nil {
			return nil, fmt.Errorf("server write nonce: %w", err)
		}
	}

	// Derive keys: HKDF-SHA256(ikm=secret, salt=clientNonce‖serverNonce, info=..., length=32)
	salt := make([]byte, 0, nonceLen*2)
	salt = append(salt, clientNonce[:]...)
	salt = append(salt, serverNonce[:]...)

	kC2S := hkdfSha256(secret, salt, infoC2S)
	kS2C := hkdfSha256(secret, salt, infoS2C)

	// Create codecs: each direction gets its own key and counter
	var err error
	var c2s, s2c cipher.AEAD

	c2sBlock, err := aes.NewCipher(kC2S)
	if err != nil {
		return nil, fmt.Errorf("new c2s cipher: %w", err)
	}
	c2s, err = cipher.NewGCM(c2sBlock)
	if err != nil {
		return nil, fmt.Errorf("new c2s gcm: %w", err)
	}

	s2cBlock, err := aes.NewCipher(kS2C)
	if err != nil {
		return nil, fmt.Errorf("new s2c cipher: %w", err)
	}
	s2c, err = cipher.NewGCM(s2cBlock)
	if err != nil {
		return nil, fmt.Errorf("new s2c gcm: %w", err)
	}

	var send, recv *frame.Codec
	if isClient {
		send = frame.NewCodec(c2s) // client encrypts with c2s key
		recv = frame.NewCodec(s2c) // client decrypts with s2c key
	} else {
		send = frame.NewCodec(s2c) // server encrypts with s2c key
		recv = frame.NewCodec(c2s) // server decrypts with c2s key
	}

	return &Session{conn: conn, send: send, recv: recv, isServer: !isClient}, nil
}

func hkdfSha256(ikm, salt, info []byte) []byte {
	key, err := hkdf.Key(sha256.New, ikm, salt, string(info), keyLen)
	if err != nil {
		return nil
	}
	return key
}

// exchangeHello performs the hello exchange. The server generates a fresh
// per-connection identity and sends it in its hello (json "cid"). The client
// echoes it back in its own hello. Because a new random id is minted on every
// connection, a reconnecting peer can never claim another peer's queued clips,
// and a disconnected peer's id is orphaned and subject to expiry.
func exchangeHello(s *Session) error {
	hello := &Message{Type: "hello", Ver: 1}
	if s.isServer {
		id, err := newConnID()
		if err != nil {
			return fmt.Errorf("generate conn id: %w", err)
		}
		s.connID = id
		hello.Cid = id
	}
	if err := s.WriteMsg(hello); err != nil {
		return fmt.Errorf("write hello: %w", err)
	}
	// Read hello
	msg, err := s.ReadMsg()
	if err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	if msg.Type != "hello" || msg.Ver != 1 {
		return fmt.Errorf("unexpected hello: type=%s ver=%d", msg.Type, msg.Ver)
	}
	// Adopt the per-connection identity. On the server side the id was announced
	// in our own hello and echoed back by the client; on the client side it was
	// announced by the server. Fall back to the client-supplied pid only if no
	// cid was exchanged, so older clients still key their outbox by a stable id.
	if msg.Cid != "" {
		s.connID = msg.Cid
	} else if msg.Pid != "" {
		s.connID = msg.Pid
	}
	return nil
}
