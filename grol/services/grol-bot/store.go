package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Session struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
	Updated  time.Time `json:"updated"`
}

type Event struct {
	TS        time.Time `json:"ts"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Preview   string    `json:"preview"`
}

type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	events   []Event
}

func NewStore() *Store {
	return &Store{sessions: map[string]*Session{}}
}

func (s *Store) GetOrCreate(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != "" {
		if sess, ok := s.sessions[id]; ok {
			return sess
		}
	}
	if id == "" {
		id = newID()
	}
	sess := &Session{ID: id, Updated: time.Now().UTC()}
	s.sessions[id] = sess
	return sess
}

func (s *Store) Append(id string, msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[id]
	if sess == nil {
		return
	}
	sess.Messages = append(sess.Messages, msg)
	if len(sess.Messages) > 32 {
		sess.Messages = sess.Messages[len(sess.Messages)-32:]
	}
	sess.Updated = time.Now().UTC()
}

func (s *Store) Note(id, role, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview := text
	if len(preview) > 160 {
		preview = preview[:160]
	}
	s.events = append(s.events, Event{
		TS:        time.Now().UTC(),
		SessionID: id,
		Role:      role,
		Preview:   preview,
	})
	if len(s.events) > 200 {
		s.events = s.events[len(s.events)-200:]
	}
}

func (s *Store) SessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *Store) Summaries() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]map[string]any, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, map[string]any{
			"id":       sess.ID,
			"turns":    len(sess.Messages),
			"updated":  sess.Updated,
		})
	}
	return out
}

func (s *Store) Activity() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
