// Package state owns the private, bounded, single-writer ledger.
package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/angelmsger/wecom-chat-cli/internal/securefs"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
	"golang.org/x/sys/unix"
)

type Event struct {
	ID           string    `json:"id"`
	Sequence     int64     `json:"sequence"`
	UserID       string    `json:"user_id"`
	ChatType     string    `json:"chat_type"`
	ChatID       string    `json:"chat_id"`
	Text         string    `json:"text,omitempty"`
	ContentHash  string    `json:"content_hash"`
	Received     int64     `json:"received_at"`
	Expires      int64     `json:"reply_expires_at"`
	Decision     string    `json:"decision"`
	Reasons      []string  `json:"reasons"`
	RequestID    string    `json:"request_id"`
	Generation   string    `json:"generation"`
	ReplyStatus  string    `json:"reply_status,omitempty"`
	ReplyHash    string    `json:"reply_hash,omitempty"`
	Flagged      bool      `json:"flagged,omitempty"`
	SessionEpoch int64     `json:"session_epoch,omitempty"`
	TaskAttempts int       `json:"task_attempts,omitempty"`
	Task         *Task     `json:"task,omitempty"`
	Proposal     *Proposal `json:"proposal,omitempty"`
	Report       *Report   `json:"report,omitempty"`
}

// Token hashes are private ledger data, never part of a public task view.
type Task struct {
	ID        string `json:"id"`
	TokenHash string `json:"token_hash"`
	Policy    string `json:"policy"`
	RunID     string `json:"run_id"`
	Epoch     int64  `json:"epoch"`
	Expires   int64  `json:"expires_at"`
	Revoked   bool   `json:"revoked"`
}
type Proposal struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	Digest       string `json:"digest"`
	Text         string `json:"text"`
	Template     string `json:"template,omitempty"`
	ReviewStatus string `json:"review_status"`
	Created      int64  `json:"created_at"`
}
type Report struct {
	Reason  string `json:"reason"`
	TaskID  string `json:"task_id"`
	Created int64  `json:"created_at"`
}
type Memory struct {
	UserID   string `json:"user_id"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence_event"`
	Source   string `json:"source"`
	Updated  int64  `json:"updated_at"`
	Expires  int64  `json:"expires_at,omitempty"`
}

func (m Memory) Active(now int64) bool { return m.Status != "" && (m.Expires == 0 || m.Expires > now) }

type Snapshot struct {
	Version       int               `json:"version"`
	ID            string            `json:"id"`
	Scope         string            `json:"scope"`
	Events        []Event           `json:"events"`
	Memory        map[string]Memory `json:"memory"`
	SessionEpochs map[string]int64  `json:"session_epochs"`
}
type Store struct {
	mu     sync.Mutex
	dir    string
	lock   *os.File
	data   Snapshot
	broken bool
}

func RandomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure randomness unavailable")
	}
	return hex.EncodeToString(b[:])
}
func failure() error {
	return e.New(e.CategoryConfig, "STATE_IO", "Cannot safely read or persist state; preserve the ledger and ask the operator to inspect storage.")
}
func Open(dir, scope string) (*Store, error) {
	if err := securefs.Directory(dir, true, false); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "broker.lock")
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, failure()
	}
	f := os.NewFile(uintptr(fd), "broker.lock")
	if err = securefs.File(lockPath, true); err != nil {
		f.Close()
		return nil, err
	}
	if unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB) != nil {
		f.Close()
		return nil, e.New(e.CategoryConflict, "BROKER_RUNNING", "Another broker owns this ledger.")
	}
	s := &Store{dir: dir, lock: f, data: Snapshot{Version: 2, ID: RandomID(), Scope: scope, Events: []Event{}, Memory: map[string]Memory{}, SessionEpochs: map[string]int64{}}}
	path := filepath.Join(dir, "ledger.json")
	if _, err = os.Lstat(path); err == nil {
		if err = securefs.File(path, true); err != nil {
			f.Close()
			return nil, err
		}
		info, er := os.Stat(path)
		if er != nil || info.Size() > 64<<20 {
			f.Close()
			return nil, failure()
		}
		b, er := os.ReadFile(path)
		if er != nil || safejson.Decode(b, &s.data, true) != nil || (s.data.Version != 1 && s.data.Version != 2) || s.data.Scope != scope || s.data.ID == "" || s.data.Memory == nil || len(s.data.Events) > 10000 {
			f.Close()
			return nil, failure()
		}
		for i, v := range s.data.Events {
			if v.Sequence != int64(i+1) {
				f.Close()
				return nil, failure()
			}
		}
	} else if !os.IsNotExist(err) {
		f.Close()
		return nil, failure()
	}
	if err = s.Update(func(d *Snapshot) error {
		// Preserve legacy outcomes and tombstones; epoch-zero legacy events cannot
		// receive new task grants. Migration never enables a pending send.
		if d.Version == 1 {
			d.Version = 2
			d.SessionEpochs = map[string]int64{}
		}
		if d.SessionEpochs == nil {
			return failure()
		}
		if !validSnapshot(*d) {
			return failure()
		}
		for i := range d.Events {
			if d.Events[i].ReplyStatus == "sending" {
				d.Events[i].ReplyStatus = "uncertain"
			}
		}
		return nil
	}); err != nil {
		f.Close()
		return nil, err
	}
	return s, nil
}

func validHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func validSnapshot(d Snapshot) bool {
	if len(d.SessionEpochs) > 10000 {
		return false
	}
	for key, epoch := range d.SessionEpochs {
		if !validHex(key, 64) || epoch < 1 {
			return false
		}
	}
	ids := map[string]bool{}
	for _, v := range d.Events {
		if ids[v.ID] || v.ID == "" || !slices.Contains([]string{"", "sending", "sent", "failed", "uncertain"}, v.ReplyStatus) {
			return false
		}
		ids[v.ID] = true
		if v.Task != nil {
			t := v.Task
			if !validHex(v.ID, 64) || v.SessionEpoch < 1 || v.TaskAttempts < 1 || v.TaskAttempts > 8 || !validHex(t.ID, 32) || !validHex(t.TokenHash, 64) || !validHex(t.Policy, 64) || !validHex(t.RunID, 32) || t.Epoch != v.SessionEpoch || t.Expires <= v.Received || t.Expires > v.Expires {
				return false
			}
		}
		if v.Proposal != nil {
			p := v.Proposal
			if v.Task == nil || p.TaskID != v.Task.ID || !validHex(p.ID, 32) || !validHex(p.Digest, 64) || len(p.Text) > 4096 || !utf8.ValidString(p.Text) || !slices.Contains([]string{"pending", "approved", "rejected"}, p.ReviewStatus) {
				return false
			}
		}
		if v.Report != nil && (v.Task == nil || v.Report.TaskID != v.Task.ID || !slices.Contains([]string{"prompt_injection", "secret_request", "destructive_request", "abuse", "other"}, v.Report.Reason)) {
			return false
		}
	}
	return true
}
func (s *Store) Close() error { return s.lock.Close() }
func clone(d Snapshot) Snapshot {
	b, _ := json.Marshal(d)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) View() Snapshot { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.data) }

// Any persistence failure poisons this process, including a failure after rename.
func (s *Store) Update(fn func(*Snapshot) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return failure()
	}
	next := clone(s.data)
	if err := fn(&next); err != nil {
		return err
	}
	b, err := json.Marshal(next)
	if err != nil || len(b) > 64<<20 {
		return failure()
	}
	f, err := os.CreateTemp(s.dir, ".ledger-")
	if err != nil {
		s.broken = true
		return failure()
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err == nil {
		err = os.Rename(f.Name(), filepath.Join(s.dir, "ledger.json"))
	}
	if err == nil {
		var dir *os.File
		dir, err = os.Open(s.dir)
		if err == nil {
			err = dir.Sync()
			dir.Close()
		}
	}
	if err != nil {
		s.broken = true
		return failure()
	}
	s.data = next
	return nil
}
func (s *Store) Healthy() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.broken }
func (s *Store) Expire(now time.Time, retention int) error {
	d := s.View()
	needed := false
	for _, v := range d.Events {
		if (v.Text != "" || v.Proposal != nil && v.Proposal.Text != "") && v.Received+int64(retention) <= now.Unix() {
			needed = true
			break
		}
	}
	for _, m := range d.Memory {
		if !m.Active(now.Unix()) {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	return s.Update(func(d *Snapshot) error {
		for i := range d.Events {
			if d.Events[i].Received+int64(retention) <= now.Unix() {
				d.Events[i].Text = ""
				if d.Events[i].Proposal != nil {
					d.Events[i].Proposal.Text = ""
				}
			}
		}
		for id, m := range d.Memory {
			if !m.Active(now.Unix()) {
				delete(d.Memory, id)
			}
		}
		return nil
	})
}
