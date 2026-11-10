package broker

import (
	"context"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/state"
)

const pendingFeed = "pending_v1"

// A stream discovers unassigned work, not an audit history or a job queue.
// Busy sessions remain visible: the host queues them and issues a grant before
// invoking a model. Existing grants require explicit operator recovery.
func (b *Broker) pending(d state.Snapshot, ev state.Event) bool {
	if b.usable(d, &ev) != nil || ev.Task != nil || ev.Proposal != nil || ev.Report != nil || ev.ReplyStatus != "" || ev.TaskAttempts >= 8 {
		return false
	}
	for _, other := range d.Events[ev.Sequence:] {
		if other.Task != nil && other.SessionEpoch == ev.SessionEpoch && b.sessionKey(other) == b.sessionKey(ev) {
			return false
		}
	}
	return true
}

func (b *Broker) eventReference(d state.Snapshot, ev state.Event) any {
	return wrap(map[string]any{
		"schema_version": 1, "ledger_id": d.ID, "id": ev.ID,
		"cursor": b.feedCursor(d, ev.Sequence, pendingFeed), "sequence": ev.Sequence,
		"bot_id": b.Policy.BotID, "deployment": b.Policy.Deployment,
		"session_key": b.sessionKey(ev), "session_epoch": ev.SessionEpoch,
		"received_at": ev.Received, "reply_expires_at": ev.Expires,
		"content_hash": ev.ContentHash, "kind": "message", "view": "reference",
		"coverage": "live_callbacks_only; disconnects may lose messages",
	})
}

func (b *Broker) pollEvents(ctx context.Context, r Request) (Page, error) {
	// One deadline covers all filtered wakeups. Capture the notification channel
	// before reading state, then pin the initial head/skipped progress exactly once.
	timer := time.NewTimer(time.Duration(r.WaitSeconds) * time.Second)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		b.mu.Lock()
		ch := b.changed
		b.mu.Unlock()
		page, err := b.events(r, r.IncludeSuppressed)
		if err != nil || len(page.Items) > 0 || r.WaitSeconds == 0 {
			return page, err
		}
		r.Cursor, r.Replay = page.Next, false
		select {
		case <-ctx.Done():
			return Page{}, ctx.Err()
		case <-timer.C:
			return page, nil
		case <-ch:
		}
	}
}
