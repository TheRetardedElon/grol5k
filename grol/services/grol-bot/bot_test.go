package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type chunkReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	n := copy(p, chunk)
	return n, nil
}

func TestChatRejectsTools(t *testing.T) {
	bot := NewBot("http://127.0.0.1:9")
	rr := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"web_search"}]}`
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(body)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestChatMaintainsSessionContextAndIdentity(t *testing.T) {
	var requests []map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode gateway request: %v", err)
		}
		requests = append(requests, got)
		reply := "ack-one"
		if len(requests) == 2 {
			reply = "ack-two"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": reply})
	}))
	defer upstream.Close()

	bot := NewBot(upstream.URL)

	first := httptest.NewRecorder()
	bot.chat(first, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"status please"}]}`,
	)))
	if first.Code != http.StatusOK {
		t.Fatalf("first status %d body %s", first.Code, first.Body.String())
	}
	var firstReply map[string]any
	if err := json.Unmarshal(first.Body.Bytes(), &firstReply); err != nil {
		t.Fatal(err)
	}
	sessionID, _ := firstReply["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("missing session id: %#v", firstReply)
	}

	second := httptest.NewRecorder()
	secondBody, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"messages": []map[string]string{
			{"role": "user", "content": "and now?"},
		},
	})
	bot.chat(second, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(string(secondBody))))
	if second.Code != http.StatusOK {
		t.Fatalf("second status %d body %s", second.Code, second.Body.String())
	}

	if len(requests) != 2 {
		t.Fatalf("gateway request count %d", len(requests))
	}
	msgs, _ := requests[1]["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("expected identity + first user + first assistant + second user, got %#v", requests[1]["messages"])
	}

	want := []struct {
		role    string
		content string
	}{
		{"system", identityPrompt},
		{"user", "status please"},
		{"assistant", "ack-one"},
		{"user", "and now?"},
	}
	for i, expected := range want {
		msg, _ := msgs[i].(map[string]any)
		if msg["role"] != expected.role || msg["content"] != expected.content {
			t.Fatalf("message %d = %#v, want role=%q content=%q", i, msg, expected.role, expected.content)
		}
	}

	if bot.store.SessionCount() != 1 {
		t.Fatalf("sessions %d", bot.store.SessionCount())
	}
	stored := bot.store.Messages(sessionID)
	if len(stored) != 4 {
		t.Fatalf("stored turns %d: %#v", len(stored), stored)
	}
}

func TestUnknownClientSessionIDIsNotAuthoritative(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": "ack"})
	}))
	defer upstream.Close()

	bot := NewBot(upstream.URL)
	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"session_id":"client-picked","messages":[{"role":"user","content":"hi"}]}`,
	)))

	var reply map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply["session_id"] == "client-picked" || reply["session_id"] == "" {
		t.Fatalf("bot trusted unknown client session id: %#v", reply)
	}
}

func TestStreamEmitsSessionAndPersistsFragmentedAssistant(t *testing.T) {
	bot := NewBot("http://gateway.invalid")
	bot.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/chat" {
				t.Fatalf("path %q", r.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
				},
				Body: io.NopCloser(&chunkReader{chunks: [][]byte{
					[]byte("event: del"),
					[]byte("ta\ndata: {\"text\":\"hello \"}\n"),
					[]byte("\nevent: delta\ndata: {\"text\":\"world\"}\n\n"),
					[]byte("event: done\ndata: {\"ok\":true}\n\n"),
				}}),
				Request: r,
			}, nil
		}),
	}

	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"stream it"}],"stream":true}`,
	)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	out := rr.Body.String()
	if !strings.Contains(out, "event: session") ||
		!strings.Contains(out, "event: delta") ||
		!strings.Contains(out, `{"text":"hello "}`) ||
		!strings.Contains(out, `{"text":"world"}`) {
		t.Fatalf("unexpected SSE:\n%s", out)
	}

	summaries := bot.store.Summaries()
	if len(summaries) != 1 {
		t.Fatalf("summaries %#v", summaries)
	}
	sessionID, _ := summaries[0]["id"].(string)
	stored := bot.store.Messages(sessionID)
	if len(stored) != 2 {
		t.Fatalf("stored turns %d: %#v", len(stored), stored)
	}
	if stored[1].Role != "assistant" || stored[1].Content != "hello world" {
		t.Fatalf("assistant history %#v", stored[1])
	}
}

func TestHealthReportsBotAndGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"ok":true,"service":"grol-ai-gateway","provisioned":true,"model":"test-model"}`)
	}))
	defer upstream.Close()

	bot := NewBot(upstream.URL)
	rr := httptest.NewRecorder()
	bot.health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["service"] != "grol-bot" {
		t.Fatalf("health %v", body)
	}
	gateway, _ := body["gateway"].(map[string]any)
	if gateway["reachable"] != true || gateway["model"] != "test-model" {
		t.Fatalf("gateway health %#v", gateway)
	}
}


func TestObserverSnapshotIsInjectedAsReadOnlyUntrustedContext(t *testing.T) {
	observer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"service": "grol-healthd",
				"hostd": map[string]any{"reachable": true},
			})
		case "/v1/snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"status": "ok",
				"mutation_capable": false,
				"system": map[string]any{
					"grol_version": "18.4.dev",
					"pretty_name": "GROL5000 OS",
				},
				"network": map[string]any{
					"connectivity": "online",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer observer.Close()

	var got map[string]any
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode gateway request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": "system looks healthy"})
	}))
	defer gateway.Close()

	bot := NewBotWithObserver(gateway.URL, observer.URL)
	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"how is the system?"}]}`,
	)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("expected identity + observer context + user, got %#v", got["messages"])
	}

	contextMsg, _ := msgs[1].(map[string]any)
	if contextMsg["role"] != "system" {
		t.Fatalf("observer context role %#v", contextMsg)
	}
	contextText, _ := contextMsg["content"].(string)
	if !strings.Contains(contextText, "untrusted data") ||
		!strings.Contains(contextText, "18.4.dev") ||
		!strings.Contains(contextText, "\"mutation_capable\":false") {
		t.Fatalf("observer context %q", contextText)
	}

	systemRR := httptest.NewRecorder()
	bot.systemSnapshot(systemRR, httptest.NewRequest(http.MethodGet, "/v1/system", nil))
	if systemRR.Code != http.StatusOK {
		t.Fatalf("system status %d body %s", systemRR.Code, systemRR.Body.String())
	}
}

func TestObserverUnavailableDoesNotBreakChat(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": "still online"})
	}))
	defer gateway.Close()

	bot := NewBotWithObserver(gateway.URL, "http://127.0.0.1:1")
	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"hello"}]}`,
	)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var reply map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply["text"] != "still online" {
		t.Fatalf("reply %#v", reply)
	}
}
