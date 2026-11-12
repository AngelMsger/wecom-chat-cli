package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/broker"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
)

func fixture() (State, broker.Page, Config) {
	s := State{Version: 1, Consumer: Digest("consumer")}
	p := broker.Page{Next: "cursor-4", Watch: &broker.WatchMetadata{Identity: Identity{Bot: "bot", Deployment: "support", Ledger: strings.Repeat("a", 32)}, Binding: Digest("policy"), After: 4}}
	now := time.Now().Unix()
	for i := 1; i <= 4; i++ {
		r := Record{Trust: Trust{"wecom_smart_bot", "untrusted_external", "none", false, warning}, Data: Reference{Version: 1, Ledger: p.Watch.Identity.Ledger, ID: Digest(i), Cursor: fmt.Sprint("cursor-", i), Sequence: int64(i), Bot: "bot", Deployment: "support", Session: Digest("session"), Epoch: 1, Received: now - 10, Expires: now + 120, Hash: Digest("content"), Kind: "message", View: "reference", Coverage: Coverage}}
		if i == 3 {
			r.Data.Expires = now - 1
		}
		if i == 4 {
			r.Data.Epoch = 2
		}
		p.Items = append(p.Items, r)
	}
	c := DefaultConfig()
	c.URL = "https://example.invalid/events"
	return s, p, c
}

func TestQueueAtomicResumeAndImmutableBatch(t *testing.T) {
	s, p, c := fixture()
	now := time.Now()
	if err := s.Collect(p, c, now); err != nil {
		t.Fatal(err)
	}
	if s.Depth() != 3 || s.Filtered != 1 || s.After != 4 || s.Seal(c, now) {
		t.Fatal("filter or debounce", s)
	}
	if !s.Seal(c, now.Add(time.Second)) || len(s.Inflight.Batch.Threads) != 2 || len(s.Inflight.Batch.Threads[0].Events) != 2 {
		t.Fatal("thread grouping", s)
	}
	batch, _ := json.Marshal(s.Inflight.Batch)
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path); err == nil {
		other.Close()
		t.Fatal("concurrent writer")
	}
	if err = f.Save(s); err != nil {
		t.Fatal(err)
	}
	f.Close()
	resumed, exists, err := Read(path)
	if err != nil || !exists {
		t.Fatal(err)
	}
	got, _ := json.Marshal(resumed.Inflight.Batch)
	if string(got) != string(batch) {
		t.Fatal("batch changed on restart")
	}
	// An unrelated later observation cannot mutate a previously attempted batch.
	v := p.Items[0].(Record).Data
	v.Sequence, v.ID = 5, Digest(5)
	p.Items = []any{Record{Trust: Trust{"wecom_smart_bot", "untrusted_external", "none", false, warning}, Data: v}}
	p.Watch.After, p.Next = 5, "cursor-5"
	if err = resumed.Collect(p, c, now); err != nil {
		t.Fatal(err)
	}
	if resumed.Seal(c, now.Add(time.Hour)) {
		t.Fatal("changed inflight batch")
	}
	got, _ = json.Marshal(resumed.Inflight.Batch)
	if string(got) != string(batch) {
		t.Fatal("mutated retry payload")
	}
	resumed.Ack(now)
	if resumed.Depth() != 1 || resumed.Acknowledged != 3 || resumed.AckedBatches != 1 {
		t.Fatal(resumed)
	}
	f, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.Save(resumed); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0600 {
		t.Fatal("queue is not private")
	}
	// A replacement by another writer must not be overwritten even with its lock.
	if err = os.WriteFile(path, []byte(`{"unrelated":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if f.Save(resumed) == nil {
		t.Fatal("overwrote external change")
	}
}

func TestQueueCapacityAndCorruption(t *testing.T) {
	s, p, c := fixture()
	c.QueueLimit = 2
	if s.Collect(p, c, time.Now()) == nil || s.After != 0 || s.Depth() != 0 {
		t.Fatal("capacity advanced cursor")
	}
	c.QueueLimit = 100
	if err := s.Collect(p, c, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.Seal(c, time.Now().Add(time.Minute))
	s.Inflight.Batch.Threads[0].Events[0].Trust.Content = "trusted"
	if s.Validate() == nil {
		t.Fatal("accepted corrupted trust metadata")
	}
	for _, mode := range []string{"symlink", "public", "parent", "unrelated"} {
		t.Run(mode, func(t *testing.T) {
			dir, _ := filepath.EvalSymlinks(t.TempDir())
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "state.json")
			s, p, c := fixture()
			_ = s.Collect(p, c, time.Now())
			b, _ := json.Marshal(s)
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, exists, err := Read(path); err != nil || !exists {
				t.Fatalf("valid state must be readable before corruption: %v", err)
			}
			switch mode {
			case "symlink":
				link := filepath.Join(dir, "linked.json")
				if err := os.Symlink(path, link); err != nil {
					t.Fatal(err)
				}
				path = link
			case "public":
				_ = os.Chmod(path, 0644)
			case "parent":
				_ = os.Chmod(dir, 0777)
			case "unrelated":
				_ = os.WriteFile(path, []byte(`{"version":1}`), 0600)
			}
			if _, _, err := Read(path); err == nil {
				t.Fatal("unsafe state accepted")
			}
		})
	}
}

func TestWebhookAcknowledgmentAndRedaction(t *testing.T) {
	for _, test := range []struct {
		name           string
		status         int
		response, code string
		retry          bool
	}{
		{"ack", 200, `{"ack":"batch"}`, "", false},
		{"durable-handoff", 202, `{"ack":"batch"}`, "", false},
		{"empty", 204, "", "RELAY_ACK", false},
		{"wrong", 200, `{"ack":"different"}`, "RELAY_ACK", false},
		{"duplicate-key", 200, `{"ack":"bad","ack":"batch"}`, "RELAY_ACK", false},
		{"oversized", 200, strings.Repeat("secret", 1000), "RELAY_ACK", false},
		{"interrupted", 200, `{"ack":`, "RELAY_UNREACHABLE", true},
		{"unauthorized", 401, "secret", "RELAY_REJECTED", false},
		{"forbidden", 403, "secret", "RELAY_REJECTED", false},
		{"rate-limit", 429, "secret", "RELAY_REJECTED", true},
		{"server", 503, "secret", "RELAY_REJECTED", true},
		{"redirect", 302, "secret", "RELAY_REDIRECT", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.Header.Get("Idempotency-Key") != "batch" || r.Header.Get("Authorization") != "Bearer token-secret" {
					t.Error("request contract")
				}
				w.Header().Set("Location", "/secret-destination")
				if test.name == "interrupted" {
					w.Header().Set("Content-Length", "100")
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.response))
			}))
			defer srv.Close()
			c := DefaultConfig()
			c.URL, c.TokenEnv = srv.URL, "RELAY_TEST_TOKEN"
			t.Setenv(c.TokenEnv, "token-secret")
			h, err := NewWebhook(c)
			if err != nil {
				t.Fatal(err)
			}
			err = h.Send(context.Background(), Batch{ID: "batch"})
			if calls != 1 {
				t.Fatal("hidden retry or redirect", calls)
			}
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || e.Safe(err).Code != test.code || Retryable(err) != test.retry || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), srv.URL) {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigValidationAndFatalErrors(t *testing.T) {
	_, _, c := fixture()
	for _, url := range []string{"http://example.com", "file:///etc/passwd", "https://user:pass@example.com", "https://example.com?token=secret", "https://example.com#fragment"} {
		c.URL = url
		if c.Validate() == nil {
			t.Fatal("accepted destination", url)
		}
	}
	for _, code := range []string{"EVENT_GAP", "WATCH_BINDING_CHANGED", "POLL_CAPACITY", "POLL_BUDGET", "POLL_SCOPE_LIMIT", "AUTH_REJECTED", "WATCH_PROTOCOL"} {
		if Retryable(e.New(e.CategoryRateLimit, code, "test")) {
			t.Fatal("retried fatal", code)
		}
	}
}

func TestExpiryPreservesUncertainBatchesAndMessageIdentity(t *testing.T) {
	s, p, c := fixture()
	now := time.Now()
	if err := s.Collect(p, c, now); err != nil {
		t.Fatal(err)
	}
	if !s.Seal(c, now.Add(time.Second)) {
		t.Fatal("batch not sealed")
	}
	b := s.Inflight.Batch
	// Different session epochs are grouped separately; every message survives.
	if len(b.Threads) != 2 || b.Threads[0].Epoch == b.Threads[1].Epoch {
		t.Fatal(b)
	}
	s.Inflight.Attempts = 1
	if s.Expire(now.Add(time.Hour)) || Digest(s.Inflight.Batch) != Digest(b) {
		t.Fatal("mutated attempted batch")
	}
	s.Inflight.Attempts = 0
	if !s.Expire(now.Add(time.Hour)) || s.Depth() != 0 || s.Expired != 3 || s.Validate() != nil {
		t.Fatal(s)
	}
	consumer := ConsumerState{Version: 1, Batches: map[string]Claim{}, Events: map[string]string{}}
	r, err := consumer.ClaimBatch(b, now.Add(time.Hour))
	if err != nil || r.Claimed || r.Status != "expired" || len(consumer.Events) != 3 || consumer.Validate() != nil {
		t.Fatal(r, err)
	}
	r, err = consumer.ClaimBatch(b, now.Add(2*time.Hour))
	if err != nil || r.Claimed {
		t.Fatal(r, err)
	}
}

func TestMalformedPageNeverAdvancesCollection(t *testing.T) {
	for _, kind := range []string{"missing", "scope", "regression", "duplicate", "body", "trust", "cursor"} {
		s, p, c := fixture()
		switch kind {
		case "missing":
			p.Watch = nil
		case "scope":
			p.Watch.Identity.Bot = "other"
		case "regression":
			p.Watch.After = 0
		case "duplicate":
			p.Items[1] = p.Items[0]
		case "body":
			p.Items[0] = map[string]any{"body": "untrusted text"}
		case "trust":
			r := p.Items[0].(Record)
			r.Trust.HostActions = true
			p.Items[0] = r
		case "cursor":
			p.Next = ""
		}
		before := Digest(s)
		if s.Collect(p, c, time.Now()) == nil || Digest(s) != before {
			t.Fatal(kind, s)
		}
	}
}

func TestPrivateStateRejectsHardLinks(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	_ = os.Chmod(dir, 0700)
	path := filepath.Join(dir, "state")
	s, _, _ := fixture()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(s); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err = os.Link(path, filepath.Join(dir, "copy")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = Read(path); err == nil {
		t.Fatal("hardlinked state accepted")
	}
}
