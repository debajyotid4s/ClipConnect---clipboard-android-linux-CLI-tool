package session

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

const (
	magic       = "CCN1"
	nonceLen    = 16
	keyLen      = 32
	handshakeTO = 5 * time.Second
)

var (
	infoC2S = []byte("clipconnect v1 c2s")
	infoS2C = []byte("clipconnect v1 s2c")
	helloMsg = []byte(`{"t":"hello","v":1}`)
)

// Message is a JSON frame message.
type Message struct {
	Type string  `json:"t"`
	Text string  `json:"text,omitempty"`
	Ver  int     `json:"v,omitempty"`
	Seq  uint64  `json:"seq,omitempty"`
}

// Session holds an encrypted connection between two peers.
type Session struct {
	conn net.Conn
	recv *frame.Codec
	send *frame.Codec
}

// NewServer performs the protocol handshake as the server and returns a ready Session.
func NewServer(conn net.Conn, secret []byte) (*Session, error) {
	s, err := handshake(conn, secret, false)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := exchangeHello(s); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

// NewClient performs the protocol handshake as the client and returns a ready Session.
func NewClient(conn net.Conn, secret []byte) (*Session, error) {
	s, err := handshake(conn, secret, true)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := exchangeHello(s); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

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

	return &Session{conn: conn, send: send, recv: recv}, nil
}

func hkdfSha256(ikm, salt, info []byte) []byte {
	key, err := hkdf.Key(sha256.New, ikm, salt, string(info), keyLen)
	if err != nil {
		return nil
	}
	return key
}

func exchangeHello(s *Session) error {
	// Send hello
	if err := s.WriteMsg(&Message{Type: "hello", Ver: 1}); err != nil {
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
	return nil
}

// ReadMsg reads one encrypted message from the session.
func (s *Session) ReadMsg() (*Message, error) {
	data, err := readOneFrame(s.conn, s.recv)
	if err != nil {
		return nil, err
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}
	return &msg, nil
}

// WriteMsg sends one encrypted message through the session.
func (s *Session) WriteMsg(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	enc, err := s.send.Encode(data)
	if err != nil {
		return err
	}
	_, err = s.conn.Write(enc)
	return err
}

// Close closes the connection.
func (s *Session) Close() error {
	return s.conn.Close()
}

// Conn returns the underlying net.Conn.
func (s *Session) Conn() net.Conn {
	return s.conn
}

// RemoteAddr returns the remote address of the connection.
func (s *Session) RemoteAddr() string {
	return s.conn.RemoteAddr().String()
}

// SetReadDeadline sets the read deadline on the underlying connection.
func (s *Session) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline on the underlying connection.
func (s *Session) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

func readOneFrame(r io.Reader, dec *frame.Codec) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("read frame header: %w", err)
	}

	length := binary.BigEndian.Uint32(header)
	if length > frame.MaxPlaintext+frame.TagSize {
		return nil, fmt.Errorf("frame too large: %d", length)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read frame body: %w", err)
	}

	full := append(header, body...)
	return dec.Decode(full)
}
