// Package config owns operator policy, not agent-controlled configuration.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/angelmsger/wecom-chat-cli/internal/securefs"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_.:@-]{1,256}$`)

func ID(s string) bool { return identifier.MatchString(s) }

type Policy struct {
	Version          int               `json:"version"`
	Deployment       string            `json:"deployment"`
	BotID            string            `json:"bot_id"`
	StateDir         string            `json:"state_dir"`
	AgentSocket      string            `json:"agent_socket"`
	OperatorSocket   string            `json:"operator_socket"`
	AllowedUsers     []string          `json:"allowed_users"`
	AllowedGroups    []string          `json:"allowed_groups"`
	ReplyMode        string            `json:"reply_mode"`
	Templates        map[string]string `json:"templates"`
	TaskTTLSeconds   int               `json:"task_ttl_seconds"`
	ReplyTTLSeconds  int               `json:"reply_ttl_seconds"`
	RetentionSeconds int               `json:"retention_seconds"`
	MaxEvents        int               `json:"max_events"`
	PerUserPerMinute int               `json:"per_user_per_minute"`
}

func Example() Policy {
	return Policy{Version: 2, Deployment: "support", BotID: "BOT_ID", StateDir: "/var/lib/wecom-chat", AgentSocket: "/run/wecom-chat-agent/bridge.sock", OperatorSocket: "/run/wecom-chat-operator/bridge.sock", AllowedUsers: []string{}, AllowedGroups: []string{}, ReplyMode: "review", Templates: map[string]string{}, TaskTTLSeconds: 120, ReplyTTLSeconds: 120, RetentionSeconds: 86400, MaxEvents: 10000, PerUserPerMinute: 20}
}
func (p Policy) Validate() error {
	bad := func() error {
		return e.New(e.CategoryConfig, "INVALID_POLICY", "Invalid policy identity, bounds, allowlists or socket paths.")
	}
	if p.Version != 2 || !ID(p.Deployment) || !ID(p.BotID) || p.ReplyTTLSeconds < 10 || p.ReplyTTLSeconds > 300 || p.TaskTTLSeconds < 10 || p.TaskTTLSeconds > p.ReplyTTLSeconds || p.RetentionSeconds < 300 || p.RetentionSeconds > 604800 || p.MaxEvents < 10 || p.MaxEvents > 10000 || p.PerUserPerMinute < 1 || p.PerUserPerMinute > 120 {
		return bad()
	}
	if p.ReplyMode != "disabled" && p.ReplyMode != "review" && p.ReplyMode != "template" {
		return bad()
	}
	if len(p.Templates) > 16 || p.ReplyMode == "template" && len(p.Templates) == 0 {
		return bad()
	}
	for id, text := range p.Templates {
		if !ID(id) || !utf8.ValidString(text) || len(text) == 0 || len(text) > 4096 {
			return bad()
		}
		for _, r := range text {
			if unicode.IsControl(r) && r != '\n' && r != '\t' || unicode.Is(unicode.Cf, r) {
				return bad()
			}
		}
	}
	if len(p.AllowedUsers) > 1000 || len(p.AllowedGroups) > 1000 {
		return bad()
	}
	for _, ids := range [][]string{p.AllowedUsers, p.AllowedGroups} {
		seen := map[string]bool{}
		for _, id := range ids {
			if !ID(id) || seen[id] {
				return bad()
			}
			seen[id] = true
		}
	}
	for _, path := range []string{p.StateDir, p.AgentSocket, p.OperatorSocket} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
			return bad()
		}
	}
	if len(p.AgentSocket) > 100 || len(p.OperatorSocket) > 100 || filepath.Dir(p.AgentSocket) == filepath.Dir(p.OperatorSocket) {
		return bad()
	}
	return nil
}
func (p Policy) Digest() string {
	b, _ := json.Marshal(p)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func Load(path string) (Policy, error) {
	var p Policy
	if err := securefs.File(path, false); err != nil {
		return p, err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > 64<<10 {
		return p, e.New(e.CategoryConfig, "POLICY_IO", "Cannot read bounded operator policy.")
	}
	b, err := os.ReadFile(path)
	if err != nil || safejson.Decode(b, &p, true) != nil {
		return p, e.New(e.CategoryConfig, "POLICY_IO", "Cannot decode operator policy.")
	}
	return p, p.Validate()
}
