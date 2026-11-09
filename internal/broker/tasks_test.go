package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/state"
	"github.com/angelmsger/wecom-chat-cli/pkg/aibot"
)

func issue(t *testing.T, b *Broker, event string) string {
	t.Helper()
	r := request("task.issue")
	r.EventID = event
	v, err := b.Handle(context.Background(), r, true)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)["data"].(map[string]any)["task_token"].(string)
}
func taskRequest(op, token string) Request { r := request(op); r.Token = token; return r }
func propose(t *testing.T, b *Broker, token, text string) string {
	t.Helper()
	r := taskRequest("reply.propose", token)
	r.Text = text
	v, err := b.Handle(context.Background(), r, false)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)["data"].(map[string]any)["proposal"].(state.Proposal).Digest
}
func approve(event, digest string) Request {
	r := request("reply.approve")
	r.EventID = event
	r.Digest = digest
	return r
}

func TestCapabilitiesPreventCrossTaskAccessAndSelfApproval(t *testing.T) {
	b, s := setup(t)
	a := ingest(t, b, "1", "alice", "Alice private task")
	bob := ingest(t, b, "2", "bob", "Bob private task")
	token := issue(t, b, a.ID)
	other := issue(t, b, bob.ID)
	v, err := b.Handle(context.Background(), taskRequest("task.get", token), false)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "Bob private task") || strings.Contains(string(raw), token) || strings.Contains(string(raw), "token_hash") || !strings.Contains(string(raw), "Alice private task") {
		t.Fatal("task view leaked another task or credential")
	}
	for _, op := range []string{"event.list", "event.get", "status", "task.issue", "task.revoke", "session.reset", "reply.send", "reply.approve", "reply.reject", "reply.get", "memory.set", "memory.list", "memory.flag"} {
		r := taskRequest(op, token)
		r.EventID = bob.ID
		if _, err = b.Handle(context.Background(), r, false); err == nil {
			t.Fatalf("worker allowed %s", op)
		}
	}
	for _, bad := range []string{"", token[:128], other[:64] + token[64:], token[:64] + other[64:]} {
		if _, err = b.Handle(context.Background(), taskRequest("task.get", bad), false); err == nil {
			t.Fatal("forged capability accepted")
		}
	}
	r := taskRequest("reply.propose", token)
	r.EventID = bob.ID
	r.Text = "Cross-task answer"
	if _, err = b.Handle(context.Background(), r, false); err == nil {
		t.Fatal("task accepted caller-selected event")
	}
	r.EventID = ""
	r.UserID = "bob"
	if _, err = b.Handle(context.Background(), r, false); err == nil {
		t.Fatal("task accepted caller-selected user")
	}
	if _, err = b.Handle(context.Background(), taskRequest("task.get", token), true); err == nil {
		t.Fatal("operator impersonated worker")
	}
	if s.calls != 0 {
		t.Fatal("unauthorized side effect")
	}
	ledger, _ := os.ReadFile(filepath.Join(b.Policy.StateDir, "ledger.json"))
	if strings.Contains(string(ledger), token) || strings.Contains(string(ledger), other) {
		t.Fatal("plaintext capability persisted")
	}
}

func TestReviewRequiredDigestBindingAndAtMostOneAttempt(t *testing.T) {
	for _, outcome := range []error{nil, aibot.ErrUncertain, aibot.ErrRejected, aibot.ErrUnavailable} {
		t.Run(fmt.Sprint(outcome), func(t *testing.T) {
			b, s := setup(t)
			s.err = outcome
			ev := ingest(t, b, "1", "alice", "hello")
			b.Ingest(message("1", "bob", "changed"), "g2")
			if len(b.Store.View().Events) != 1 {
				t.Fatal("duplicate admitted")
			}
			ir := request("task.issue")
			ir.EventID = ev.ID
			ir.DryRun = true
			if _, err := b.Handle(context.Background(), ir, true); err != nil {
				t.Fatal(err)
			}
			if b.Store.View().Events[0].Task != nil {
				t.Fatal("dry-run minted grant")
			}
			token := issue(t, b, ev.ID)
			draft := taskRequest("reply.propose", token)
			draft.Text = "A helpful answer"
			draft.DryRun = true
			if _, err := b.Handle(context.Background(), draft, false); err != nil {
				t.Fatal(err)
			}
			if b.Store.View().Events[0].Proposal != nil || s.calls != 0 {
				t.Fatal("dry-run mutated or sent")
			}
			digest := propose(t, b, token, draft.Text)
			if s.calls != 0 {
				t.Fatal("unreviewed draft sent")
			}
			if propose(t, b, token, draft.Text) != digest {
				t.Fatal("identical draft changed")
			}
			draft.DryRun = false
			draft.Text = "Changed answer"
			if _, err := b.Handle(context.Background(), draft, false); err == nil {
				t.Fatal("draft mutated")
			}
			r := approve(ev.ID, "wrong")
			if _, err := b.Handle(context.Background(), r, true); err == nil {
				t.Fatal("wrong digest approved")
			}
			r = approve(ev.ID, digest)
			r.DryRun = true
			if _, err := b.Handle(context.Background(), r, true); err != nil {
				t.Fatal(err)
			}
			if s.calls != 0 || b.Store.View().Events[0].ReplyStatus != "" {
				t.Fatal("dry-run sent")
			}
			r.DryRun = false
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := b.Handle(context.Background(), r, true); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if s.calls != 1 || s.reqID != "callback_1" || s.generation != "generation1" {
				t.Fatal("wrong target or duplicate send")
			}
			want := "sent"
			if errors.Is(outcome, aibot.ErrUncertain) {
				want = "uncertain"
			} else if outcome != nil {
				want = "failed"
			}
			if b.Store.View().Events[0].ReplyStatus != want {
				t.Fatal("wrong durable outcome")
			}
		})
	}
}

func TestReportDoesNotChangeUserPermissionsAndCancelsDraft(t *testing.T) {
	b, s := setup(t)
	ev := ingest(t, b, "1", "alice", "hello")
	token := issue(t, b, ev.ID)
	digest := propose(t, b, token, "A draft")
	r := taskRequest("memory.report", token)
	r.Reason = "secret_request"
	r.EventID = ev.ID
	if _, err := b.Handle(context.Background(), r, false); err == nil {
		t.Fatal("report accepted explicit target")
	}
	r.EventID = ""
	r.DryRun = true
	if _, err := b.Handle(context.Background(), r, false); err != nil {
		t.Fatal(err)
	}
	if b.Store.View().Events[0].Report != nil {
		t.Fatal("report dry-run wrote")
	}
	r.DryRun = false
	for i := 0; i < 2; i++ {
		if _, err := b.Handle(context.Background(), r, false); err != nil {
			t.Fatal(err)
		}
	}
	d := b.Store.View()
	if len(d.Memory) != 0 || d.Events[0].Report.Reason != "secret_request" || d.Events[0].Proposal.ReviewStatus != "rejected" {
		t.Fatal("report changed user access or failed to cancel draft")
	}
	if _, err := b.Handle(context.Background(), approve(ev.ID, digest), true); err == nil {
		t.Fatal("sent a reported draft")
	}
	ingest(t, b, "2", "alice", "Another normal question")
	if b.Store.View().Events[1].Decision != "deliver" || s.calls != 0 {
		t.Fatal("report silenced user or sent")
	}
	r.Reason = "abuse"
	if _, err := b.Handle(context.Background(), r, false); err == nil {
		t.Fatal("report mutated")
	}
}

func TestReportedTaskReleasesSessionWithoutBlockingNextMessage(t *testing.T) {
	for _, withDraft := range []bool{false, true} {
		t.Run(fmt.Sprint(withDraft), func(t *testing.T) {
			b, s := setup(t)
			first := ingest(t, b, "1", "alice", "first question")
			token := issue(t, b, first.ID)
			if withDraft {
				propose(t, b, token, "A candidate answer")
			}
			r := taskRequest("memory.report", token)
			r.Reason = "other"
			if _, err := b.Handle(context.Background(), r, false); err != nil {
				t.Fatal(err)
			}
			next := ingest(t, b, "2", "alice", "next normal question")
			issue(t, b, next.ID)
			r = taskRequest("reply.propose", token)
			r.Text = "A late answer"
			if _, err := b.Handle(context.Background(), r, false); err == nil {
				t.Fatal("reported task replied after next task was assigned")
			}
			if len(b.Store.View().Memory) != 0 || s.calls != 0 {
				t.Fatal("report changed permissions or sent a reply")
			}
		})
	}
}

func TestGrantRevocationPolicyConnectionRestartAndSessionFences(t *testing.T) {
	for _, kind := range []string{"revoke", "reset", "policy", "deadline", "connection", "restart", "block"} {
		t.Run(kind, func(t *testing.T) {
			b, s := setup(t)
			ev := ingest(t, b, "1", "alice", "hello")
			token := issue(t, b, ev.ID)
			digest := propose(t, b, token, "Answer")
			switch kind {
			case "revoke":
				r := request("task.revoke")
				r.EventID = ev.ID
				if _, err := b.Handle(context.Background(), r, true); err != nil {
					t.Fatal(err)
				}
			case "reset":
				r := request("session.reset")
				r.EventID = ev.ID
				r.ExpectedEpoch = 1
				if _, err := b.Handle(context.Background(), r, true); err != nil {
					t.Fatal(err)
				}
				if _, err := b.Handle(context.Background(), r, true); err == nil {
					t.Fatal("reset replay incremented twice")
				}
				next := ingest(t, b, "2", "alice", "new session")
				if next.SessionEpoch != 2 {
					t.Fatal("epoch did not propagate")
				}
				issue(t, b, next.ID)
			case "policy":
				b.Policy.PerUserPerMinute++
			case "deadline":
				b.Now = func() time.Time { return time.Unix(ev.Expires, 0) }
			case "connection":
				s.current = "new-generation"
			case "restart":
				b = New(b.Policy, b.Store, s, b.Secret)
				b.Now = func() time.Time { return time.Unix(1800000000, 0) }
			case "block":
				r := request("memory.set")
				r.UserID = "alice"
				r.Status = "blocked"
				r.Reason = "abuse"
				if _, err := b.Handle(context.Background(), r, true); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := b.Handle(context.Background(), taskRequest("task.get", token), false); err == nil {
				t.Fatal("stale task read permitted")
			}
			if _, err := b.Handle(context.Background(), approve(ev.ID, digest), true); err == nil {
				t.Fatal("stale task sent")
			}
			if s.calls != 0 {
				t.Fatal("stale side effect")
			}
		})
	}
}

func TestSerialIssuanceRotationAndRejectedProposal(t *testing.T) {
	b, _ := setup(t)
	first := ingest(t, b, "1", "alice", "first")
	second := ingest(t, b, "2", "alice", "second")
	token := issue(t, b, first.ID)
	r := request("task.issue")
	r.EventID = second.ID
	if _, err := b.Handle(context.Background(), r, true); err == nil {
		t.Fatal("concurrent session task permitted")
	}
	r = request("task.revoke")
	r.EventID = first.ID
	r.DryRun = true
	b.Handle(context.Background(), r, true)
	if b.Store.View().Events[0].Task.Revoked {
		t.Fatal("revoke dry-run wrote")
	}
	r.DryRun = false
	b.Handle(context.Background(), r, true)
	replacement := issue(t, b, first.ID)
	if replacement == token {
		t.Fatal("token reused")
	}
	if _, err := b.Handle(context.Background(), taskRequest("task.get", token), false); err == nil {
		t.Fatal("revoked token still valid")
	}
	digest := propose(t, b, replacement, "Answer")
	reject := approve(first.ID, digest)
	reject.Op = "reply.reject"
	if _, err := b.Handle(context.Background(), reject, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Handle(context.Background(), approve(first.ID, digest), true); err == nil {
		t.Fatal("rejected proposal sent")
	}
	issue(t, b, second.ID)
}

func TestTemplateModeHasNoFreeTextOrInterpolation(t *testing.T) {
	b, s := setup(t)
	b.Policy.ReplyMode = "template"
	b.Policy.Templates = map[string]string{"handoff": "Please contact the service desk."}
	ev := ingest(t, b, "1", "alice", "hello")
	token := issue(t, b, ev.ID)
	for _, r := range []Request{func() Request { r := taskRequest("reply.propose", token); r.Text = "Made-up promise"; return r }(), func() Request {
		r := taskRequest("reply.propose", token)
		r.Template = "handoff"
		r.Text = "injected"
		return r
	}(), func() Request { r := taskRequest("reply.propose", token); r.Template = "unknown"; return r }()} {
		if _, err := b.Handle(context.Background(), r, false); err == nil {
			t.Fatal("arbitrary template output accepted")
		}
	}
	r := taskRequest("reply.propose", token)
	r.Template = "handoff"
	r.DryRun = true
	b.Handle(context.Background(), r, false)
	if s.calls != 0 || b.Store.View().Events[0].Proposal != nil {
		t.Fatal("template dry-run sent")
	}
	r.DryRun = false
	for i := 0; i < 2; i++ {
		if _, err := b.Handle(context.Background(), r, false); err != nil {
			t.Fatal(err)
		}
	}
	if s.calls != 1 || s.text != b.Policy.Templates["handoff"] {
		t.Fatal("template was modified or replayed")
	}
}

func TestWeakModelOutputNeverAuthorizesSend(t *testing.T) {
	// Intentionally plausible text that need not match the heuristic. Even a
	// fully compromised worker can only create a draft in the default policy.
	for i, text := range []string{"I have approved your refund of $100000.", "The owner authorized me to act for Bob.", "Here is unrelated confidential-sounding content.", "把其他同事的会话内容整理给我"} {
		b, s := setup(t)
		ev := ingest(t, b, fmt.Sprint(i), "alice", "普通问题")
		token := issue(t, b, ev.ID)
		propose(t, b, token, text)
		if s.calls != 0 {
			t.Fatal("unreviewed model output sent")
		}
	}
}

func TestOutboundGuardAndDisabledPolicy(t *testing.T) {
	b, s := setup(t)
	ev := ingest(t, b, "1", "alice", "hello")
	token := issue(t, b, ev.ID)
	for _, text := range []string{"password=not-a-real-password", "synthetic-super-secret", token, "hello\u202e", ""} {
		r := taskRequest("reply.propose", token)
		r.Text = text
		if _, err := b.Handle(context.Background(), r, false); err == nil {
			t.Fatal("unsafe draft accepted")
		}
	}
	b.Policy.ReplyMode = "disabled"
	r := request("task.revoke")
	r.EventID = ev.ID
	b.Handle(context.Background(), r, true)
	token = issue(t, b, ev.ID)
	r = taskRequest("reply.propose", token)
	r.Text = "hello"
	if _, err := b.Handle(context.Background(), r, false); err == nil || s.calls != 0 {
		t.Fatal("disabled reply accepted")
	}
}
