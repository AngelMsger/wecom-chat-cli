package broker

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/angelmsger/wecom-chat-cli/internal/state"
	"github.com/angelmsger/wecom-chat-cli/pkg/aibot"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
)

// Reject fields that could make a task operation appear to choose its target,
// mode or authority. This also applies to dry-run and raw socket requests.
func validateRequest(r Request) error {
	allowed := map[string]string{
		"status": "", "event.list": "cursor limit wait include", "event.get": "event",
		"task.issue": "event", "task.revoke": "event", "session.reset": "event epoch",
		"task.get": "", "reply.propose": "text template", "memory.report": "reason",
		"reply.get": "event", "reply.approve": "event digest", "reply.reject": "event digest",
		"memory.list": "", "memory.set": "user status reason",
	}
	fields, ok := allowed[r.Op]
	if !ok {
		return e.New(e.CategoryUsage, "UNKNOWN_OPERATION", "Unsupported operation; use --help for the current task/review commands.")
	}
	used := map[string]bool{"event": r.EventID != "", "cursor": r.Cursor != "", "limit": r.Limit != 0, "wait": r.WaitSeconds != 0, "include": r.IncludeSuppressed, "text": r.Text != "", "template": r.Template != "", "digest": r.Digest != "", "reason": r.Reason != "", "user": r.UserID != "", "status": r.Status != "", "epoch": r.ExpectedEpoch != 0}
	for field, present := range used {
		if present && !strings.Contains(" "+fields+" ", " "+field+" ") {
			return e.New(e.CategoryUsage, "UNEXPECTED_FIELD", "This operation does not accept target, mode or unrelated fields.")
		}
	}
	return nil
}

func taskDenied() error {
	return e.New(e.CategoryPermission, "TASK_DENIED", "Task grant is invalid, expired, revoked or no longer matches the current session, connection or policy.")
}
func (b *Broker) usable(d state.Snapshot, ev *state.Event) error {
	now := b.Now().Unix()
	if ev == nil || ev.Decision != "deliver" || !b.eligible(*ev) || ev.SessionEpoch < 1 || d.SessionEpochs[b.sessionKey(*ev)] != ev.SessionEpoch || ev.Expires <= now || ev.Received+int64(b.Policy.RetentionSeconds) <= now {
		return taskDenied()
	}
	m := d.Memory[ev.UserID]
	if m.Active(now) && (m.Status == "blocked" || m.Status == "quarantined") {
		return taskDenied()
	}
	if ev.Generation == "" || b.Sender.Generation() != ev.Generation {
		return taskDenied()
	}
	return nil
}
func (b *Broker) liveTask(d state.Snapshot, ev *state.Event) error {
	if err := b.usable(d, ev); err != nil {
		return err
	}
	t := ev.Task
	if t == nil || t.Revoked || t.Expires <= b.Now().Unix() || t.Epoch != ev.SessionEpoch || t.Policy != b.Policy.Digest() || t.RunID != b.runID {
		return taskDenied()
	}
	return nil
}
func (b *Broker) authorized(d *state.Snapshot, token string) (*state.Event, error) {
	if len(token) != 129 || token[64] != '.' {
		return nil, taskDenied()
	}
	if _, err := hex.DecodeString(token[:64] + token[65:]); err != nil {
		return nil, taskDenied()
	}
	ev := find(d, token[:64])
	if ev == nil || ev.Task == nil {
		return nil, taskDenied()
	}
	actual := hash("task-token", token)
	if subtle.ConstantTimeCompare([]byte(actual), []byte(ev.Task.TokenHash)) != 1 {
		return nil, taskDenied()
	}
	if err := b.liveTask(*d, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

func (b *Broker) issue(r Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.Store.View()
	ev := find(&d, r.EventID)
	if err := b.usable(d, ev); err != nil {
		return nil, err
	}
	if ev.Proposal != nil || ev.Report != nil || ev.ReplyStatus != "" || ev.TaskAttempts >= 8 {
		return nil, e.New(e.CategoryConflict, "TASK_CLOSED", "This event has a proposal/outcome or exhausted its task-grant budget.")
	}
	if ev.Task != nil && !ev.Task.Revoked {
		return nil, e.New(e.CategoryConflict, "TASK_EXISTS", "A grant already exists. Inspect the event; revoke a lost grant before issuing another.")
	}
	for _, other := range d.Events {
		if other.ID == ev.ID || other.Task == nil || other.SessionEpoch != ev.SessionEpoch || b.sessionKey(other) != b.sessionKey(*ev) {
			continue
		}
		if other.Sequence > ev.Sequence {
			return nil, e.New(e.CategoryConflict, "STALE_EVENT", "A newer event in this session has already been assigned; do not issue older work.")
		}
		if b.liveTask(d, &other) == nil && other.Report == nil && other.ReplyStatus == "" && (other.Proposal == nil || other.Proposal.ReviewStatus == "pending") {
			return nil, e.New(e.CategoryConflict, "SESSION_BUSY", "This session already has an active task; finish, reject or revoke it before issuing the next event.")
		}
	}
	expires := min(ev.Expires, b.Now().Unix()+int64(b.Policy.TaskTTLSeconds))
	if r.DryRun {
		return wrap(map[string]any{"dry_run": true, "event_id": ev.ID, "session_key": b.sessionKey(*ev), "session_epoch": ev.SessionEpoch, "expires_at": expires}), nil
	}
	token := ev.ID + "." + state.RandomID() + state.RandomID()
	task := state.Task{ID: state.RandomID(), TokenHash: hash("task-token", token), Policy: b.Policy.Digest(), RunID: b.runID, Epoch: ev.SessionEpoch, Expires: expires}
	err := b.Store.Update(func(d *state.Snapshot) error { v := find(d, ev.ID); v.Task = &task; v.TaskAttempts++; return nil })
	if err != nil {
		return nil, err
	}
	// This one operator-only response is the sole token disclosure. Never log it
	// or pass it through a model; the supervisor provisions one worker's environment.
	return wrap(map[string]any{"task_id": task.ID, "event_id": ev.ID, "session_key": b.sessionKey(*ev), "session_epoch": task.Epoch, "expires_at": task.Expires, "task_token": token}), nil
}
func (b *Broker) revoke(r Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.Store.View()
	ev := find(&d, r.EventID)
	if ev == nil || ev.Task == nil {
		return nil, e.New(e.CategoryNotFound, "TASK_NOT_FOUND", "No task grant exists for that event.")
	}
	out := wrap(map[string]any{"event_id": ev.ID, "revoked": true, "dry_run": r.DryRun})
	if r.DryRun || ev.Task.Revoked {
		return out, nil
	}
	err := b.Store.Update(func(d *state.Snapshot) error { find(d, ev.ID).Task.Revoked = true; return nil })
	return out, err
}
func (b *Broker) reset(r Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.Store.View()
	ev := find(&d, r.EventID)
	if ev == nil {
		return nil, e.New(e.CategoryNotFound, "EVENT_NOT_FOUND", "Select an event from the session to reset.")
	}
	key := b.sessionKey(*ev)
	epoch := d.SessionEpochs[key]
	if epoch == math.MaxInt64 {
		return nil, e.New(e.CategoryConflict, "SESSION_EPOCH_LIMIT", "Session epoch capacity is exhausted.")
	}
	if r.ExpectedEpoch < 1 || r.ExpectedEpoch != epoch {
		return nil, e.New(e.CategoryConflict, "SESSION_EPOCH", "Supply the current session epoch; a reset may already have completed.")
	}
	out := wrap(map[string]any{"session_key": key, "session_epoch": epoch + 1, "dry_run": r.DryRun})
	if r.DryRun {
		return out, nil
	}
	err := b.Store.Update(func(d *state.Snapshot) error { d.SessionEpochs[key] = epoch + 1; return nil })
	return out, err
}

func (b *Broker) worker(ctx context.Context, r Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.Store.View()
	ev, err := b.authorized(&d, r.Token)
	if err != nil {
		return nil, err
	}
	switch r.Op {
	case "task.get":
		ids := []string{}
		if b.Policy.ReplyMode == "template" {
			for id := range b.Policy.Templates {
				ids = append(ids, id)
			}
			slices.Sort(ids)
		}
		return wrap(map[string]any{"task_id": ev.Task.ID, "expires_at": ev.Task.Expires, "reply_mode": b.Policy.ReplyMode, "template_ids": ids, "event": b.eventView(d, *ev)}), nil
	case "memory.report":
		if !reasonValid(r.Reason) {
			return nil, e.New(e.CategoryUsage, "INVALID_REASON", "Use prompt_injection, secret_request, destructive_request, abuse or other.")
		}
		if ev.Report != nil {
			if ev.Report.Reason != r.Reason {
				return nil, e.New(e.CategoryConflict, "REPORT_CONFLICT", "This event already has an immutable risk report.")
			}
			return wrap(ev.Report), nil
		}
		report := state.Report{Reason: r.Reason, TaskID: ev.Task.ID, Created: b.Now().Unix()}
		if r.DryRun {
			return wrap(map[string]any{"dry_run": true, "report": report}), nil
		}
		err = b.Store.Update(func(d *state.Snapshot) error {
			v := find(d, ev.ID)
			v.Report = &report
			if v.Proposal != nil && v.ReplyStatus == "" {
				v.Proposal.ReviewStatus = "rejected"
			}
			return nil
		})
		return wrap(report), err
	case "reply.propose":
		return b.proposeLocked(ctx, r, d, ev)
	default:
		return nil, denied()
	}
}

var secretPattern = regexp.MustCompile(`(?i)-----BEGIN.{0,20}PRIVATE KEY-----|\b(password|api[_-]?key|secret|access[_-]?token)\s*[:=]\s*\S{4,}|\b(sk-[a-zA-Z0-9_-]{16,}|AKIA[A-Z0-9]{16})\b`)

func (b *Broker) checkText(text, token string) error {
	if !utf8.ValidString(text) || len(text) == 0 || len(text) > 4096 || len(screen(text)) > 0 || secretPattern.MatchString(text) || b.Secret != "" && strings.Contains(text, b.Secret) || token != "" && strings.Contains(text, token) {
		return e.New(e.CategoryPermission, "REPLY_CONTENT_BLOCKED", "Reply is empty, too large or matches an outbound risk rule.")
	}
	return nil
}
func (b *Broker) proposalDigest(ev state.Event, p state.Proposal) string {
	return hash("proposal-v2", b.Scope(), ev.ID, ev.UserID, ev.ChatType, ev.ChatID, ev.RequestID, ev.Generation, ev.Task.ID, ev.Task.Policy, fmt.Sprint(ev.Task.Epoch), fmt.Sprint(ev.Task.Expires), p.Template, p.Text)
}
func (b *Broker) proposalView(ev state.Event) any {
	p := *ev.Proposal
	if ev.Received+int64(b.Policy.RetentionSeconds) <= b.Now().Unix() {
		p.Text = ""
	}
	return wrap(map[string]any{"event_id": ev.ID, "session_key": b.sessionKey(ev), "session_epoch": ev.SessionEpoch, "sender_user_id": ev.UserID, "conversation": map[string]string{"type": ev.ChatType, "id": ev.ChatID}, "expires_at": ev.Task.Expires, "proposal": p, "send_status": ev.ReplyStatus, "risk_report": ev.Report, "revoked": ev.Task.Revoked})
}
func (b *Broker) proposeLocked(ctx context.Context, r Request, d state.Snapshot, ev *state.Event) (any, error) {
	if ev.Report != nil {
		return nil, e.New(e.CategoryPermission, "TASK_REPORTED", "This task reported risk and cannot produce a new reply; user permissions are unchanged.")
	}
	if b.Policy.ReplyMode == "disabled" {
		return nil, e.New(e.CategoryPermission, "REPLIES_DISABLED", "Reply proposals are disabled by operator policy.")
	}
	text := r.Text
	if b.Policy.ReplyMode == "template" {
		var ok bool
		text, ok = b.Policy.Templates[r.Template]
		if !ok || r.Text != "" {
			return nil, e.New(e.CategoryPermission, "TEMPLATE_REQUIRED", "Select an operator-approved literal template; free text and substitutions are forbidden.")
		}
	} else if r.Template != "" {
		return nil, e.New(e.CategoryUsage, "UNEXPECTED_TEMPLATE", "Review mode accepts a final text draft, not a template selector.")
	}
	if err := b.checkText(text, r.Token); err != nil {
		return nil, err
	}
	p := state.Proposal{ID: state.RandomID(), TaskID: ev.Task.ID, Text: text, Template: r.Template, ReviewStatus: "pending", Created: b.Now().Unix()}
	p.Digest = b.proposalDigest(*ev, p)
	if ev.Proposal != nil {
		if ev.Proposal.Digest != p.Digest {
			return nil, e.New(e.CategoryConflict, "PROPOSAL_CONFLICT", "This event already has a different immutable proposal.")
		}
		return b.proposalView(*ev), nil
	}
	if ev.ReplyStatus != "" {
		return nil, e.New(e.CategoryConflict, "REPLY_CLOSED", "This event already has a send outcome.")
	}
	if r.DryRun {
		return wrap(map[string]any{"dry_run": true, "event_id": ev.ID, "digest": p.Digest, "reply_mode": b.Policy.ReplyMode}), nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := b.Store.Update(func(d *state.Snapshot) error { find(d, ev.ID).Proposal = &p; return nil }); err != nil {
		return nil, err
	}
	ev.Proposal = &p
	if b.Policy.ReplyMode == "template" {
		return b.sendLocked(ctx, d, ev)
	}
	return b.proposalView(*ev), nil
}
func (b *Broker) proposalGet(r Request) (any, error) {
	d := b.Store.View()
	ev := find(&d, r.EventID)
	if ev == nil || ev.Proposal == nil {
		return nil, e.New(e.CategoryNotFound, "PROPOSAL_NOT_FOUND", "No proposal exists for that event.")
	}
	return b.proposalView(*ev), nil
}
func (b *Broker) review(ctx context.Context, r Request) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.Store.View()
	ev := find(&d, r.EventID)
	if ev == nil || ev.Proposal == nil {
		return nil, e.New(e.CategoryNotFound, "PROPOSAL_NOT_FOUND", "Inspect an event with a submitted proposal.")
	}
	if r.Digest == "" || r.Digest != ev.Proposal.Digest {
		return nil, e.New(e.CategoryConflict, "PROPOSAL_DIGEST", "Approval/rejection must name the exact digest returned by reply get.")
	}
	if ev.ReplyStatus != "" {
		return b.proposalView(*ev), nil
	}
	if r.Op == "reply.reject" {
		if r.DryRun {
			return wrap(map[string]any{"dry_run": true, "event_id": ev.ID, "digest": r.Digest, "action": "reject"}), nil
		}
		err := b.Store.Update(func(d *state.Snapshot) error { find(d, ev.ID).Proposal.ReviewStatus = "rejected"; return nil })
		if err != nil {
			return nil, err
		}
		ev.Proposal.ReviewStatus = "rejected"
		return b.proposalView(*ev), nil
	}
	if b.Policy.ReplyMode != "review" {
		return nil, e.New(e.CategoryPermission, "REVIEW_MODE_REQUIRED", "Only review policy permits an operator to approve a free-text draft.")
	}
	if err := b.canSend(d, ev); err != nil {
		return nil, err
	}
	if r.DryRun {
		return wrap(map[string]any{"dry_run": true, "event_id": ev.ID, "digest": r.Digest, "action": "approve"}), nil
	}
	return b.sendLocked(ctx, d, ev)
}
func (b *Broker) canSend(d state.Snapshot, ev *state.Event) error {
	if err := b.liveTask(d, ev); err != nil {
		return err
	}
	if ev.Report != nil || ev.Proposal == nil || ev.Proposal.ReviewStatus != "pending" || ev.Proposal.TaskID != ev.Task.ID || ev.Proposal.Digest != b.proposalDigest(*ev, *ev.Proposal) {
		return e.New(e.CategoryConflict, "PROPOSAL_CLOSED", "The proposal is rejected, changed or no longer bound to the current task.")
	}
	return b.checkText(ev.Proposal.Text, "")
}

// Caller holds the admission lock across persistence and I/O. A completed revoke,
// reset or block cannot race a send that has not started. No alternate send path.
func (b *Broker) sendLocked(ctx context.Context, d state.Snapshot, ev *state.Event) (any, error) {
	if err := b.canSend(d, ev); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	err := b.Store.Update(func(d *state.Snapshot) error {
		v := find(d, ev.ID)
		v.ReplyStatus = "sending"
		v.ReplyHash = ev.Proposal.Digest
		v.Proposal.ReviewStatus = "approved"
		return nil
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if b.Now().Unix() >= ev.Task.Expires {
		err = aibot.ErrUnavailable
	} else {
		err = b.Sender.Reply(ctx, ev.Generation, ev.RequestID, "stream_"+hash(ev.ID)[:32], ev.Proposal.Text)
	}
	status := "sent"
	if err != nil {
		status = "uncertain"
		if errors.Is(err, aibot.ErrRejected) || errors.Is(err, aibot.ErrUnavailable) {
			status = "failed"
		}
	}
	if err = b.Store.Update(func(d *state.Snapshot) error { find(d, ev.ID).ReplyStatus = status; return nil }); err != nil {
		return nil, e.New(e.CategoryConflict, "WRITE_UNCERTAIN", "Reply may have been sent; outcome persistence failed. Preserve the ledger and never replay the write.")
	}
	ev.ReplyStatus = status
	ev.Proposal.ReviewStatus = "approved"
	return b.proposalView(*ev), nil
}
