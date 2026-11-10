package broker

import (
	"context"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/state"
)

const pendingFeed = "pending_v1"

// Bot and deployment are fixed for a broker snapshot. These are the remaining
// session-key inputs, plus the epoch; using the tuple avoids hashing every row.
type assignmentScope struct {
	chatType, chatID, userID string
	epoch                    int64
}

func assignmentKey(ev state.Event) assignmentScope {
	return assignmentScope{ev.ChatType, ev.ChatID, ev.UserID, ev.SessionEpoch}
}

func latestAssignments(events []state.Event) map[assignmentScope]int64 {
	latest := make(map[assignmentScope]int64)
	for _, ev := range events {
		// Even revoked, expired or completed tasks supersede older unassigned
		// work. Do not reuse the live-task predicate here.
		if ev.Task != nil {
			key := assignmentKey(ev)
			latest[key] = max(latest[key], ev.Sequence)
		}
	}
	return latest
}

// A stream discovers unassigned work, not an audit history or a job queue.
// Busy sessions remain visible: the host queues them and issues a grant before
// invoking a model. Existing grants require explicit operator recovery.
func (b *Broker) pending(d state.Snapshot, ev state.Event, latest map[assignmentScope]int64) bool {
	if ev.Task != nil || ev.Proposal != nil || ev.Report != nil || ev.ReplyStatus != "" || ev.TaskAttempts >= 8 || latest[assignmentKey(ev)] > ev.Sequence {
		return false
	}
	return b.usable(d, &ev) == nil
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
	wait := time.Duration(r.WaitSeconds) * time.Second
	if r.WaitMillis != 0 {
		wait = time.Duration(r.WaitMillis) * time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return Page{}, err
		}
		b.mu.Lock()
		ch := b.changed
		b.mu.Unlock()
		page, err := b.events(r, r.IncludeSuppressed)
		if err != nil || len(page.Items) > 0 || wait == 0 {
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
