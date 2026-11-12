//go:build isolation && linux

package tests

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/broker"
	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/state"
	"github.com/angelmsger/wecom-chat-cli/pkg/aibot"
)

type isolationSender struct{}

func (isolationSender) Generation() string { return "g" }

func (isolationSender) Connected() bool                                             { return true }
func (isolationSender) Reply(context.Context, string, string, string, string) error { return nil }

// Run in a disposable, network-disabled Linux container as the setup supervisor.
// Only the setup process needs CHOWN/SETUID/SETGID; the worker has no capabilities.
func TestDifferentOSIdentities(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("requires isolated root setup supervisor; use scripts/container-exec.sh")
	}
	root, err := os.MkdirTemp("", "wc-isolation-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	os.Chmod(root, 0755)
	for _, d := range []string{"agent", "operator", "state", "relay", "consumer"} {
		if err = os.Mkdir(filepath.Join(root, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	agentDir := filepath.Join(root, "agent")
	if err = os.Chown(agentDir, 0, 2000); err != nil {
		t.Fatal(err)
	}
	os.Chmod(agentDir, 0750)
	secret := filepath.Join(root, "secret.env")
	os.WriteFile(secret, []byte("synthetic-only"), 0600)
	for _, name := range []string{"relay/config.json", "relay/state.json", "relay/secrets.env", "consumer/claims.json"} {
		if err = os.WriteFile(filepath.Join(root, name), []byte("synthetic-private-state"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	policy := filepath.Join(root, "policy.json")
	os.WriteFile(policy, []byte("{}"), 0644)
	p := config.Example()
	p.BotID = "bot"
	p.AllowedUsers = []string{"alice", "bob"}
	p.StateDir = filepath.Join(root, "state")
	p.AgentSocket = filepath.Join(agentDir, "s")
	p.OperatorSocket = filepath.Join(root, "operator", "s")
	s, err := state.Open(p.StateDir, "support/bot")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	b := broker.New(p, s, isolationSender{}, "synthetic-secret")
	for _, user := range []string{"alice", "bob"} {
		var f aibot.Frame
		f.Cmd = "aibot_msg_callback"
		f.Headers.RequestID = "cb_" + user
		f.Body, _ = json.Marshal(map[string]any{"msgid": user, "aibotid": "bot", "chattype": "single", "from": map[string]string{"userid": user}, "msgtype": "text", "text": map[string]string{"content": user + " private content"}})
		if err = b.Ingest(f, "g"); err != nil {
			t.Fatal(err)
		}
	}
	granted, err := b.Handle(context.Background(), broker.Request{Op: "task.issue", BotID: "bot", Deployment: "support", EventID: s.View().Events[0].ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	token := granted.(map[string]any)["data"].(map[string]any)["task_token"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Serve(ctx, func(ctx context.Context) error { <-ctx.Done(); return nil }) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err = os.Stat(p.OperatorSocket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("broker not ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err = os.Chown(p.AgentSocket, 0, 2000); err != nil {
		t.Fatal(err)
	}
	// Copy the probe outside the Go build directory so its executable is public,
	// while broker state and operator directories remain private.
	exe, _ := os.Executable()
	src, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	path := filepath.Join(root, "probe")
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(dst, src)
	dst.Close()
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command(path, "-test.run=^TestWorkerProbe$", "-test.v")
	c.Env = []string{"WECOM_CHAT_ISOLATION_ROOT=" + root, "WECOM_CHAT_TASK_TOKEN=" + token}
	c.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 2001, Gid: 2000, Groups: []uint32{2000}}}
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("worker probe: %v %s", err, out)
	}
}

func TestWorkerProbe(t *testing.T) {
	root := os.Getenv("WECOM_CHAT_ISOLATION_ROOT")
	if root == "" {
		t.Skip("subprocess only")
	}
	if os.Geteuid() == 0 {
		t.Fatal("worker retained root")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	token := os.Getenv("WECOM_CHAT_TASK_TOKEN")
	v, err := broker.Call(ctx, filepath.Join(root, "agent", "s"), broker.Request{Op: "task.get", BotID: "bot", Deployment: "support", Token: token})
	if err != nil || !strings.Contains(string(v), "alice private content") || strings.Contains(string(v), "bob private content") {
		t.Fatal("worker task access violated")
	}
	for _, op := range []string{"event.list", "event.watch", "task.issue", "reply.approve", "memory.set"} {
		if _, err := broker.Call(ctx, filepath.Join(root, "agent", "s"), broker.Request{Op: op, BotID: "bot", Deployment: "support", Token: token}); err == nil {
			t.Fatal("worker obtained privileged operation")
		}
	}
	if c, err := net.Dial("unix", filepath.Join(root, "operator", "s")); err == nil {
		c.Close()
		t.Fatal("worker reached operator socket")
	}
	for _, path := range []string{filepath.Join(root, "state", "ledger.json"), filepath.Join(root, "secret.env"), filepath.Join(root, "relay", "config.json"), filepath.Join(root, "relay", "state.json"), filepath.Join(root, "relay", "secrets.env"), filepath.Join(root, "consumer", "claims.json")} {
		if _, err := os.ReadFile(path); err == nil {
			t.Fatalf("worker read protected file: %s", path)
		}
	}
	if f, err := os.OpenFile(filepath.Join(root, "policy.json"), os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Fatal("worker can edit policy")
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil || !strings.Contains(string(status), "CapEff:\t0000000000000000") {
		t.Fatal("worker has capabilities")
	}
	if c, err := net.DialTimeout("tcp", "198.51.100.1:443", 150*time.Millisecond); err == nil {
		c.Close()
		t.Fatal("worker has outbound network")
	}
}
