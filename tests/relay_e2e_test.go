//go:build e2e

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/broker"
	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/relay"
	"github.com/angelmsger/wecom-chat-cli/internal/state"
	"github.com/angelmsger/wecom-chat-cli/pkg/aibot"
)

func TestCLIRelayRestartAcceptanceClaimAndAuthority(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "wc-relay-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/wecom-chat-cli")
	build.Dir = ".."
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(b))
	}
	for _, name := range []string{"broker", "agent", "operator", "relay", "consumer"} {
		if err = os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	policy := config.Example()
	policy.BotID = "bot"
	policy.AllowedUsers = []string{"alice"}
	policy.StateDir = filepath.Join(dir, "broker")
	policy.AgentSocket = filepath.Join(dir, "agent", "s")
	policy.OperatorSocket = filepath.Join(dir, "operator", "s")
	ledger, err := state.Open(policy.StateDir, "support/bot")
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	sender := &sender{}
	b := broker.New(policy, ledger, sender, "synthetic-secret")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Serve(ctx, func(ctx context.Context) error { <-ctx.Done(); return nil }) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("broker did not stop")
		}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.Dial("unix", policy.OperatorSocket)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket not ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	calls := atomic.Int32{}
	lose := atomic.Bool{}
	lose.Store(true)
	incoming := make(chan []byte, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer synthetic-key" {
			t.Error("missing sender auth")
		}
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
		}
		incoming <- raw
		if lose.Load() {
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	cfg := relay.DefaultConfig()
	cfg.URL = srv.URL
	cfg.DeliveryMode = relay.HTTPAccepted
	cfg.TokenEnv = "TEST_CHAT_RELAY_TOKEN"
	cfg.BatchSize = 1
	cfgPath := filepath.Join(dir, "relay", "config.json")
	queue := filepath.Join(dir, "relay", "state.json")
	consumer := filepath.Join(dir, "consumer", "state.json")
	raw, _ := json.Marshal(cfg)
	if err = os.WriteFile(cfgPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(socket, input string, args ...string) (string, string, error) {
		c := exec.Command(bin, append([]string{"--socket", socket, "--bot-id", "bot", "--deployment", "support"}, args...)...)
		c.Env = append(os.Environ(), "TEST_CHAT_RELAY_TOKEN=synthetic-key")
		c.Stdin = strings.NewReader(input)
		var out, diag bytes.Buffer
		c.Stdout = &out
		c.Stderr = &diag
		err := c.Run()
		return out.String(), diag.String(), err
	}
	args := []string{"relay", "run", "--config", cfgPath, "--state", queue, "--duration", "1s", "--allow-writes"}
	// Dry run is offline and creates neither a state file nor a lock.
	if _, diag, err := run(filepath.Join(dir, "absent"), "", append(args, "--dry-run")...); err != nil {
		t.Fatal(err, diag)
	}
	if _, err = os.Stat(queue + ".lock"); !os.IsNotExist(err) {
		t.Fatal("dry run created lock")
	}
	if _, diag, err := run(policy.OperatorSocket, "", args...); err != nil {
		t.Fatal(err, diag)
	}
	if calls.Load() != 0 {
		t.Fatal("idle invoked webhook")
	}
	var f aibot.Frame
	f.Cmd = "aibot_msg_callback"
	f.Headers.RequestID = "cb-1"
	f.Body = json.RawMessage(`{"msgid":"one","aibotid":"bot","chattype":"single","from":{"userid":"alice"},"msgtype":"text","text":{"content":"hello"}}`)
	if err = b.Ingest(f, "g"); err != nil {
		t.Fatal(err)
	}
	if err = b.Ingest(f, "g"); err != nil {
		t.Fatal(err)
	}
	if _, diag, err := run(policy.OperatorSocket, "", args...); err == nil || !strings.Contains(diag, "RELAY_OUTCOME_UNKNOWN") {
		t.Fatal(err, diag)
	}
	original := <-incoming
	st, _, err := relay.Read(queue)
	if err != nil {
		t.Fatal(err)
	}
	id := st.Inflight.Batch.ID
	if _, diag, err := run(policy.OperatorSocket, "", append(args, "--retry-failed")...); err == nil || !strings.Contains(diag, "RELAY_OUTCOME_UNKNOWN") {
		t.Fatal(err, diag)
	}
	if calls.Load() != 1 {
		t.Fatal("restart replayed ambiguous POST")
	}
	if _, diag, err := run("", "", "relay", "resolve", "--state", queue, "--batch-id", id, "--outcome", "not_accepted", "--evidence", "verified-test-rejection", "--allow-writes"); err != nil {
		t.Fatal(err, diag)
	}
	lose.Store(false)
	if _, diag, err := run(policy.OperatorSocket, "", append(args, "--retry-failed", "--max-batches", "1")...); err != nil {
		t.Fatal(err, diag)
	}
	repeated := <-incoming
	if !bytes.Equal(original, repeated) {
		t.Fatal("changed retry batch")
	}
	st, _, err = relay.Read(queue)
	if err != nil || st.AcceptedEvents != 1 || st.Acknowledged != 0 {
		t.Fatal(err, st)
	}
	claimArgs := []string{"relay", "claim", "--consumer-state", consumer, "--batch-file", "-", "--allow-writes"}
	out, diag, err := run("", string(original), claimArgs...)
	if err != nil {
		t.Fatal(err, diag)
	}
	var claim relay.ClaimResult
	if json.Unmarshal([]byte(out), &claim) != nil || !claim.Claimed {
		t.Fatal(out)
	}
	out, diag, err = run("", string(original), claimArgs...)
	if err != nil || strings.Contains(out, `"claimed": true`) || strings.Contains(out, claim.Token) {
		t.Fatal(out, diag, err)
	}
	if _, diag, err = run("", "", "relay", "consume", "--consumer-state", consumer, "--batch-id", id, "--claim-id", claim.Token, "--result", "processed", "--allow-writes"); err != nil {
		t.Fatal(err, diag)
	}
	out, diag, err = run("", "", "relay", "status", "--consumer-state", consumer, "--batch-id", id)
	if err != nil || !strings.Contains(out, "processed") || strings.Contains(out, claim.Token) {
		t.Fatal(out, diag, err)
	}
	// A relay cannot turn the worker socket or a metadata ID into operator authority.
	bad := append([]string(nil), args...)
	for i, v := range bad {
		if v == queue {
			bad[i] = filepath.Join(dir, "relay", "denied.json")
		}
	}
	if _, _, err = run(policy.AgentSocket, "", bad...); err == nil {
		t.Fatal("worker socket allowed relay")
	}
	if sender.calls.Load() != 0 {
		t.Fatal("relay sent a chat reply")
	}
}
