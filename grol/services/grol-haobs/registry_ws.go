package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxWSFrame = 1 << 20

type wsConn struct {
	net.Conn
	r *bufio.Reader
}

func dialWS(u *url.URL, timeout time.Duration) (*wsConn, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	addr := u.Host
	if !strings.Contains(addr, ":") {
		if u.Scheme == "wss" {
			addr += ":443"
		} else {
			addr += ":80"
		}
	}
	var conn net.Conn
	var err error
	d := net.Dialer{Timeout: timeout}
	if u.Scheme == "wss" {
		conn, err = tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: u.Hostname()})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	key := wsKey()
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %s", resp.Status)
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != wsAccept(key) {
		conn.Close()
		return nil, fmt.Errorf("websocket accept mismatch")
	}
	return &wsConn{Conn: conn, r: br}, nil
}

func wsKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.StdEncoding.EncodeToString(b[:])
}

func wsAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func writeWSText(conn *wsConn, payload []byte) error {
	if len(payload) > maxWSFrame {
		return fmt.Errorf("websocket frame too large")
	}
	var mask [4]byte
	_, _ = rand.Read(mask[:])
	hdr := []byte{0x81, 0x80}
	n := len(payload)
	switch {
	case n < 126:
		hdr[1] |= byte(n)
	case n <= 65535:
		hdr[1] |= 126
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(n))
		hdr = append(hdr, ext...)
	default:
		return fmt.Errorf("websocket frame too large")
	}
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := conn.Write(append(hdr, masked...))
	return err
}

func readWSText(conn *wsConn) ([]byte, error) {
	r := conn.r
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(r, h); err != nil {
			return nil, err
		}
		op := h[0] & 0x0f
		masked := h[1]&0x80 != 0
		n := int(h[1] & 0x7f)
		if n == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(r, ext); err != nil {
				return nil, err
			}
			n = int(binary.BigEndian.Uint16(ext))
		} else if n == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(r, ext); err != nil {
				return nil, err
			}
			nn := binary.BigEndian.Uint64(ext)
			if nn > maxWSFrame {
				return nil, fmt.Errorf("websocket frame too large")
			}
			n = int(nn)
		}
		if n > maxWSFrame {
			return nil, fmt.Errorf("websocket frame too large")
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(r, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch op {
		case 0x1:
			return payload, nil
		case 0x8:
			return nil, fmt.Errorf("websocket closed")
		default:
			continue
		}
	}
}
