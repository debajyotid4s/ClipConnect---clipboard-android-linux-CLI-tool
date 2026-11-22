package session

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/debajyotid4s/ClipConnect---clipboard-android-linux-CLI-tool/laptop/internal/frame"
)

// newConnID generates a fresh random per-connection identity used to key the
// durable outbox. A new id is minted on every connection so a reconnecting
// peer cannot claim another peer's queued clips.
func newConnID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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

// WriteMsg sends one encrypted message through the session. It is safe for
// concurrent use: the mutex serializes encode + write so multiple writers
// (server Send, sendAck, pingLoop) cannot interleave frames or race the
// AES-GCM counter nonce.
func (s *Session) WriteMsg(msg *Message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
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
