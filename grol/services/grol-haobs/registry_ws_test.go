package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func encodeServerText(payload []byte) []byte {
	hdr := []byte{0x81, 0x00}
	n := len(payload)
	if n < 126 {
		hdr[1] = byte(n)
	} else {
		hdr[1] = 126
		hdr = append(hdr, byte(n>>8), byte(n))
	}
	return append(hdr, payload...)
}

func readClientText(r *bufio.Reader) ([]byte, error) {
	h := make([]byte, 2)
	if _, err := io.ReadFull(r, h); err != nil {
		return nil, err
	}
	n := int(h[1] & 0x7f)
	if n == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, err
		}
		n = int(ext[0])<<8 | int(ext[1])
	}
	mask := make([]byte, 4)
	if _, err := io.ReadFull(r, mask); err != nil {
		return nil, err
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return payload, nil
}

func TestFetchRegistryWSCoalescedAuthRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/websocket" {
			http.Error(w, "nope", 404)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("no hijack")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		accept := wsAccept(r.Header.Get("Sec-WebSocket-Key"))
		hello := encodeServerText([]byte(`{"type":"auth_required","ha_version":"2026.10.0"}`))
		_, _ = conn.Write(append([]byte(
			"HTTP/1.1 101 Switching Protocols\r\n"+
				"Upgrade: websocket\r\n"+
				"Connection: Upgrade\r\n"+
				"Sec-WebSocket-Accept: "+accept+"\r\n\r\n",
		), hello...))
		br := bufio.NewReader(conn)
		authRaw, err := readClientText(br)
		if err != nil {
			return
		}
		var auth map[string]any
		_ = json.Unmarshal(authRaw, &auth)
		if auth["type"] != "auth" || auth["access_token"] != "secret-token" {
			return
		}
		_, _ = conn.Write(encodeServerText([]byte(`{"type":"auth_ok"}`)))
		listRaw, err := readClientText(br)
		if err != nil {
			return
		}
		var list map[string]any
		_ = json.Unmarshal(listRaw, &list)
		if list["type"] != "config/entity_registry/list" {
			return
		}
		_, _ = conn.Write(encodeServerText([]byte(
			`{"id":1,"type":"result","success":true,"result":[{"entity_id":"light.kitchen","id":"reg-kitchen","platform":"hue"}]}`,
		)))
	}))
	defer srv.Close()

	o := NewObserver(srv.URL, "secret-token", "/no/such")
	reg, err := o.fetchRegistryWS()
	if err != nil {
		t.Fatal(err)
	}
	got := reg["light.kitchen"]
	if got.RegistryID != "reg-kitchen" || got.Platform != "hue" || got.Domain != "light" {
		t.Fatalf("%#v", got)
	}
}

func TestReadWSTextRejectsHugeFrame(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	c := &wsConn{Conn: a, r: bufio.NewReader(a)}
	go func() {
		_, _ = b.Write([]byte{0x81, 127, 0, 0, 1, 0, 0, 0, 0, 0})
	}()
	_, err := readWSText(c)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err=%v", err)
	}
}

func TestDialWSParsesSplitHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		req, err := http.ReadRequest(br)
		if err != nil {
			done <- err
			return
		}
		accept := wsAccept(req.Header.Get("Sec-WebSocket-Key"))
		part1 := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n"
		part2 := "Connection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n"
		if _, err := conn.Write([]byte(part1)); err != nil {
			done <- err
			return
		}
		time.Sleep(20 * time.Millisecond)
		hello := encodeServerText([]byte(`{"type":"auth_required"}`))
		_, err = conn.Write(append([]byte(part2), hello...))
		done <- err
	}()
	u, _ := url.Parse("http://" + ln.Addr().String() + "/api/websocket")
	ws, err := dialWS(u, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	msg, err := readWSText(ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(msg), "auth_required") {
		t.Fatalf("%s", msg)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
