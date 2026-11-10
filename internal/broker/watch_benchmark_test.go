package broker

import (
	"fmt"
	"testing"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/state"
)

// The first page is pending; the remaining ledger belongs to another assigned
// session. Page limits must not multiply scans of that unrelated history.
func pendingBenchmarkFixture(size int) (*Broker, state.Snapshot) {
	p := config.Example()
	p.BotID = "bot"
	p.AllowedUsers = []string{"alice", "bob"}
	b := New(p, nil, &fakeSender{current: "generation1"}, "synthetic-secret")
	b.Now = func() time.Time { return time.Unix(1800000000, 0) }
	d := state.Snapshot{SessionEpochs: map[string]int64{}, Memory: map[string]state.Memory{}}
	for i := range size {
		ev := state.Event{ID: fmt.Sprint(i), Sequence: int64(i + 1), UserID: "alice", ChatType: "single", ChatID: "alice", SessionEpoch: 1, Received: b.Now().Unix(), Expires: b.Now().Unix() + 120, Generation: "generation1", Decision: "deliver"}
		if i >= 100 {
			ev.UserID, ev.ChatID = "bob", "bob"
			ev.Task = &state.Task{Revoked: true}
		}
		d.SessionEpochs[b.sessionKey(ev)] = 1
		d.Events = append(d.Events, ev)
	}
	return b, d
}

func BenchmarkPendingDiscovery(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			broker, snapshot := pendingBenchmarkFixture(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				latest := latestAssignments(snapshot.Events)
				count := 0
				for _, event := range snapshot.Events[:100] {
					if broker.pending(snapshot, event, latest) {
						count++
					}
				}
				if count != 100 {
					b.Fatalf("lost pending work: %d", count)
				}
			}
		})
	}
}
