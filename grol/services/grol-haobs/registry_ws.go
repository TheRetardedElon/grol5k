package main

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

func dialWS(u *url.URL, timeout time.Duration) (net.Conn, error) {
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
	req := "GET " + u.RequestURI() + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, err
	}
	head := string(buf[:n])
	if !strings.Contains(head, " 101 ") {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed")
	}
	if !strings.Contains(head, wsAccept(key)) {
		conn.Close()
		return nil, fmt.Errorf("websocket accept mismatch")
	}
	return conn, nil
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

func writeWSText(conn net.Conn, payload []byte) error {
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
		hdr[1] |= 127
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(n))
		hdr = append(hdr, ext...)
	}
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := conn.Write(append(hdr, masked...))
	return err
}

func readWSText(conn net.Conn) ([]byte, error) {
	for {
		h := make([]byte, 2)
		if _, err := io.ReadFull(conn, h); err != nil {
			return nil, err
		}
		op := h[0] & 0x0f
		masked := h[1]&0x80 != 0
		n := int(h[1] & 0x7f)
		if n == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(conn, ext); err != nil {
				return nil, err
			}
			n = int(binary.BigEndian.Uint16(ext))
		} else if n == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(conn, ext); err != nil {
				return nil, err
			}
			n = int(binary.BigEndian.Uint64(ext))
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(conn, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(conn, payload); err != nil {
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
