package session

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/crypto"
	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

// Message is a JSON frame message.
type Message struct {
	Type string `json:"t"`
	Text string `json:"text,omitempty"`
	Ver  int    `json:"v,omitempty"`
}

// Session holds an encrypted connection between two peers.
type Session struct {
	conn net.Conn
	recv *frame.Codec
	send *frame.Codec
}

// NewServer starts a TLS connection and application handshake as the server side.
func NewServer(conn net.Conn, secret []byte) (*Session, error) {
	tlsConn := tls.Server(conn, serverTLSConfig)
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("tls handshake: %w", err)
	}

	key := crypto.DeriveKey(secret)
	ae, err := crypto.NewGCM(key)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	s := &Session{
		conn: tlsConn,
		recv: frame.NewCodec(ae),
		send: frame.NewCodec(ae),
	}

	if err := writeHello(s); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("write hello: %w", err)
	}
	if err := readHello(s); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("read hello: %w", err)
	}

	return s, nil
}

// NewClient starts a TLS connection and application handshake as the client side.
func NewClient(conn net.Conn, secret []byte) (*Session, error) {
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("tls handshake: %w", err)
	}

	key := crypto.DeriveKey(secret)
	ae, err := crypto.NewGCM(key)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	s := &Session{
		conn: tlsConn,
		recv: frame.NewCodec(ae),
		send: frame.NewCodec(ae),
	}

	if err := writeHello(s); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("write hello: %w", err)
	}
	if err := readHello(s); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("read hello: %w", err)
	}

	return s, nil
}

func writeHello(s *Session) error {
	data, err := s.send.Encode([]byte(`{"t":"hello","v":1}`))
	if err != nil {
		return err
	}
	_, err = s.conn.Write(data)
	return err
}

func readHello(s *Session) error {
	data, err := readOneFrame(s.conn, s.recv)
	if err != nil {
		return err
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse hello: %w", err)
	}
	if msg.Type != "hello" || msg.Ver != 1 {
		return fmt.Errorf("unexpected hello type=%s ver=%d", msg.Type, msg.Ver)
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
