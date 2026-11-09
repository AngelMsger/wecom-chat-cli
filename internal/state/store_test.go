package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyMigrationPreservesReplayAndMemory(t *testing.T) {
	dir := privateTempDir(t)
	os.Chmod(dir, 0700)
	legacy := Snapshot{Version: 1, ID: "old-ledger", Scope: "support/bot", Events: []Event{{ID: "old-event", Sequence: 1, ReplyStatus: "sending", ReplyHash: "old-digest"}}, Memory: map[string]Memory{"alice": {UserID: "alice", Status: "blocked", Source: "operator"}}}
	b, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "ledger.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir, "support/bot")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v := s.View()
	if v.Version != 2 || v.ID != "old-ledger" || v.SessionEpochs == nil || v.Events[0].SessionEpoch != 0 || v.Events[0].ReplyStatus != "uncertain" || v.Events[0].ReplyHash != "old-digest" || v.Memory["alice"].Status != "blocked" {
		t.Fatal("migration lost protection or granted old work authority")
	}
}

func TestDurabilityLockScopeAndRecovery(t *testing.T) {
	d := privateTempDir(t)
	os.Chmod(d, 0700)
	s, err := Open(d, "bot/scope")
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(d, "bot/scope"); err == nil {
		other.Close()
		t.Fatal("second writer allowed")
	}
	err = s.Update(func(d *Snapshot) error {
		d.Events = append(d.Events, Event{ID: "one", Sequence: 1, ReplyStatus: "sending"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	id := s.View().ID
	s.Close()
	if wrong, err := Open(d, "other/scope"); err == nil {
		wrong.Close()
		t.Fatal("scope rebound")
	}
	s, err = Open(d, "bot/scope")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.View().ID != id || s.View().Events[0].ReplyStatus != "uncertain" {
		t.Fatal("crash replay risk")
	}
	info, _ := os.Stat(filepath.Join(d, "ledger.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("ledger permissions")
	}
}
func TestSymlinkAndWriteFailure(t *testing.T) {
	d := privateTempDir(t)
	os.Chmod(d, 0700)
	target := filepath.Join(privateTempDir(t), "target")
	os.WriteFile(target, []byte("sentinel"), 0600)
	os.Symlink(target, filepath.Join(d, "ledger.json"))
	if s, err := Open(d, "scope"); err == nil {
		s.Close()
		t.Fatal("symlink accepted")
	}
	v, _ := os.ReadFile(target)
	if string(v) != "sentinel" {
		t.Fatal("target modified")
	}
	d = privateTempDir(t)
	os.Chmod(d, 0700)
	s, err := Open(d, "scope")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Rename the directory so a commit cannot create its temporary file.
	moved := d + "-moved"
	if err = os.Rename(d, moved); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(moved)
	if err = s.Update(func(d *Snapshot) error { d.Memory["x"] = Memory{Status: "blocked"}; return nil }); err == nil {
		t.Fatal("failed persistence reported success")
	}
	if s.Healthy() || len(s.View().Memory) != 0 {
		t.Fatal("failed write published or process not poisoned")
	}
}

func privateTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}
