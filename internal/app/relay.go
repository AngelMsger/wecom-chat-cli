package app

import (
	"context"
	"path/filepath"
	"time"

	"github.com/angelmsger/agent-cli-kit/output"
	"github.com/angelmsger/wecom-chat-cli/internal/broker"
	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/relay"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
	"github.com/spf13/cobra"
)

type relayApp struct {
	socket, bot, deployment *string
	writes, dry             *bool
	emit                    func(*cobra.Command, any) error
}

func (s *relayApp) guard(op string) error {
	if !*s.writes {
		return e.ReadOnlyBlocked(op, e.ReadOnlyMode{DryRun: true})
	}
	return nil
}
func (s *relayApp) commands() *cobra.Command {
	g := &cobra.Command{Use: "relay", Short: "Deliver pending references and track acceptance separately from processing"}
	g.AddCommand(s.runCommand(), s.relayStatusCommand(), s.relayResolveCommand(), s.relayClaimCommand(), s.relayConsumeCommand())
	return g
}
func (s *relayApp) runCommand() *cobra.Command {
	var configPath, statePath string
	var duration, interval time.Duration
	var maxBatches int
	var replay, retryFailed bool
	cmd := &cobra.Command{Use: "run", Short: "Trusted service: deliver nonempty metadata batches without a model", Long: "Collect pending references over the operator socket into a durable private queue.\nEmpty scans send nothing. explicit_ack requires an exact batch acknowledgment;\nhttp_accepted accepts only HTTP 200, never processing completion. Unknown outcomes\nstop until relay resolve. No grants, reply approvals or message bodies are sent.\nThe receiver must use a trusted supervisor to issue one grant per live event.", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if duration < time.Second || duration > 24*time.Hour || interval < time.Second || interval > time.Minute || maxBatches < 1 || maxBatches > 100000 || statePath == "" || !filepath.IsAbs(*s.socket) || filepath.Clean(*s.socket) != *s.socket || !config.ID(*s.bot) || !config.ID(*s.deployment) {
			return e.New(e.CategoryUsage, "RELAY_LIMITS", "Supply a private --state, absolute operator --socket, --bot-id, --deployment, duration 1s..24h, interval 1s..1m and max-batches 1..100000.")
		}
		cfg, err := relay.LoadConfig(configPath)
		if err != nil {
			return err
		}
		consumer := relay.Digest(struct {
			Socket, Bot, Deployment string
			Config                  relay.Config
		}{*s.socket, *s.bot, *s.deployment, cfg})
		var store *relay.Store
		if !*s.dry {
			if err = s.guard("relay run"); err != nil {
				return err
			}
			store, err = relay.Open(statePath)
			if err != nil {
				return err
			}
			defer store.Close()
		}
		st, exists, err := relay.Read(statePath)
		if err != nil {
			return err
		}
		if exists && (st.Consumer != consumer || replay) {
			return e.New(e.CategoryConflict, "RELAY_BINDING_CHANGED", "Resume with the original socket, bot, deployment and configuration without --replay; preserve queued work.")
		}
		if !exists {
			st = relay.State{Version: 1, Consumer: consumer, Replay: replay}
			if cfg.Mode() == relay.HTTPAccepted {
				st.DeliveryMode = relay.HTTPAccepted
			}
		}
		if *s.dry {
			return s.emit(cmd, map[string]any{"dry_run": true, "queued_events": st.Depth(), "delivery_mode": cfg.Mode(), "batch_size": cfg.BatchSize, "batch_window_seconds": cfg.BatchWindowSeconds, "content": "metadata_only", "durable": true, "replay": st.Replay})
		}
		hook, err := relay.NewWebhook(cfg)
		if err != nil {
			return err
		}
		if !exists {
			if err = store.Save(st); err != nil {
				return err
			}
		}
		if retryFailed {
			if st.Inflight != nil && (st.Inflight.Status == "sending" || st.Inflight.Status == "unknown") {
				return relay.UnknownOutcome()
			}
			st.ScanFailures, st.NextScan = 0, 0
			if st.Inflight != nil {
				st.Inflight.Attempts, st.Inflight.NextAttempt = 0, 0
				st.Inflight.Status, st.Inflight.HTTPStatus, st.Inflight.Failure = "", 0, ""
			}
			if err = store.Save(st); err != nil {
				return err
			}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), duration)
		defer cancel()
		scan := broker.Request{Op: "event.watch", BotID: *s.bot, Deployment: *s.deployment, Limit: 100}
		fetch := func(ctx context.Context, q broker.Request) (broker.Page, error) {
			child, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			raw, err := broker.Call(child, *s.socket, q)
			if err != nil {
				return broker.Page{}, err
			}
			var p broker.Page
			if safejson.Decode(raw, &p, true) != nil {
				return p, e.New(e.CategoryParse, "RELAY_PROTOCOL", "The broker did not return a valid watch page.")
			}
			return p, nil
		}
		notice := func(reason string, st relay.State, code string) {
			v := relayStatus(st)
			v["state"], v["error_code"] = reason, code
			output.EmitNotice(cmd.ErrOrStderr(), map[string]any{"_notice": map[string]any{"relay": v}})
		}
		return pumpRelay(ctx, cfg, scan, st, interval, maxBatches, fetch, hook.Send, store.Save, notice)
	}
	f := cmd.Flags()
	f.StringVar(&configPath, "config", "", "Private fixed-destination relay JSON (required)")
	f.StringVar(&statePath, "state", "", "Durable JSON queue in an existing private service-owned directory (required)")
	f.DurationVar(&duration, "duration", 5*time.Minute, "Maximum service session duration, up to 24h; resume the same state")
	f.DurationVar(&interval, "interval", time.Second, "Minimum local broker scan interval, 1s..1m")
	f.IntVar(&maxBatches, "max-batches", 1000, "Maximum acknowledged or HTTP-accepted batches in this session")
	f.BoolVar(&replay, "replay", false, "Replay retained pending work only when creating new state; default starts quietly now")
	f.BoolVar(&retryFailed, "retry-failed", false, "Explicitly reset retry budgets after repair; never replays an unknown outcome")
	return cmd
}
func validateRelayPage(p broker.Page, st relay.State, scan broker.Request) error {
	return relay.ValidatePage(p, st, scan.BotID, scan.Deployment, scan.Limit)
}

func pumpRelay(ctx context.Context, cfg relay.Config, scan broker.Request, st relay.State, interval time.Duration, maxBatches int, fetch func(context.Context, broker.Request) (broker.Page, error), send func(context.Context, relay.Batch) error, save func(relay.State) error, notice func(string, relay.State, string)) error {
	initialAcks := st.AckedBatches + st.AcceptedBatches
	started := false
	// Always validate the live ledger/policy before dispatching saved work.
	validated := false
	nextScan := time.Now()
	fail := func(err error) error { notice("failed", st, e.Safe(err).Code); return err }
	exhausted := func() error {
		return e.New(e.CategoryConflict, "RELAY_RETRIES_EXHAUSTED", "The persisted relay retry budget is exhausted. Repair the service, then explicitly use --retry-failed with the same state and configuration.")
	}
	if cfg.Mode() == relay.HTTPAccepted && st.Inflight != nil {
		if st.Inflight.Status == "sending" {
			st.Inflight.Status = "unknown"
			if err := save(st); err != nil {
				return fail(err)
			}
		}
		if st.Inflight.Status == "unknown" {
			return fail(relay.UnknownOutcome())
		}
	}
	if st.Inflight != nil && st.Inflight.Failure != "" {
		return fail(e.New(e.CategoryConflict, st.Inflight.Failure, "The saved delivery stopped on a terminal error. Repair the receiver and explicitly reset retries before resuming."))
	}
	for {
		if ctx.Err() != nil {
			notice("stopped", st, "")
			return nil
		}
		if st.ScanFailures > cfg.RetryLimit || (st.Inflight != nil && st.Inflight.Attempts > cfg.RetryLimit) {
			return fail(exhausted())
		}
		now := time.Now()
		scanDue := maxTime(nextScan, time.UnixMilli(st.NextScan))
		if (!validated || st.Depth() < cfg.QueueLimit) && !now.Before(scanDue) {
			scan.Cursor, scan.Replay = st.Cursor, st.Replay
			scan.Limit = max(1, min(100, cfg.QueueLimit-st.Depth()))
			p, err := fetch(ctx, scan)
			if err == nil {
				err = validateRelayPage(p, st, scan)
			}
			if err != nil {
				if ctx.Err() != nil {
					notice("stopped", st, "")
					return nil
				}
				if !relay.Retryable(err) {
					return fail(err)
				}
				st.ScanFailures++
				st.NextScan = time.Now().Add(max(interval, relay.RetryDelay(cfg, st.ScanFailures))).UnixMilli()
				if saveErr := save(st); saveErr != nil {
					return fail(saveErr)
				}
				if st.ScanFailures > cfg.RetryLimit {
					return fail(exhausted())
				}
				notice("scan_retry", st, e.Safe(err).Code)
			} else {
				// When the queue is full, validate without consuming the returned page.
				if st.Depth() < cfg.QueueLimit {
					if err = st.Collect(p, cfg, time.Now()); err != nil {
						return fail(err)
					}
					if err = save(st); err != nil {
						return fail(err)
					}
				} else {
					st.ScanFailures, st.NextScan = 0, 0
				}
				validated = true
				scan.Replay = false
				nextScan = time.Now().Add(interval)
				if !started {
					notice("started", st, "")
					started = true
				}
			}
		}
		if ctx.Err() != nil {
			continue
		}
		if validated {
			if st.Expire(time.Now()) {
				if err := save(st); err != nil {
					return fail(err)
				}
			}
			if st.Seal(cfg, time.Now()) {
				if err := save(st); err != nil {
					return fail(err)
				}
			}
			if st.Inflight != nil && time.Now().UnixMilli() >= max(st.Inflight.NextAttempt, st.NextDelivery) {
				if cfg.Mode() == relay.HTTPAccepted && len(st.Receipts) >= 10000 {
					return fail(e.New(e.CategoryRateLimit, "RELAY_RECEIPTS_FULL", "Delivery receipt capacity is exhausted. Archive and reconcile the saved state before provisioning a new relay."))
				}
				st.Inflight.Attempts++
				if cfg.Mode() == relay.HTTPAccepted {
					st.Inflight.Status, st.Inflight.LastAttempt, st.Inflight.HTTPStatus = "sending", time.Now().UnixMilli(), 0
				}
				st.NextDelivery = time.Now().Add(time.Duration(cfg.MinDeliverySeconds) * time.Second).UnixMilli()
				st.Inflight.NextAttempt = time.Now().Add(relay.RetryDelay(cfg, st.Inflight.Attempts)).UnixMilli()
				if err := save(st); err != nil {
					return fail(err)
				}
				err := send(ctx, st.Inflight.Batch)
				if err == nil {
					reason := "acknowledged"
					if cfg.Mode() == relay.HTTPAccepted {
						if err = st.Accept(time.Now(), "http_200"); err != nil {
							return fail(err)
						}
						reason = "accepted"
					} else {
						st.Ack(time.Now())
					}
					if err = save(st); err != nil {
						return fail(err)
					}
					notice(reason, st, "")
					if st.AckedBatches+st.AcceptedBatches-initialAcks >= int64(maxBatches) {
						notice("max_batches", st, "")
						return nil
					}
					continue
				}
				if !relay.Retryable(err) {
					st.Inflight.Failure = e.Safe(err).Code
				}
				if cfg.Mode() == relay.HTTPAccepted {
					st.Inflight.Status = "unknown"
					if x := e.Safe(err); x.Code == "RELAY_REJECTED" || x.Code == "RELAY_REDIRECT" {
						st.Inflight.Status, st.Inflight.HTTPStatus = "rejected", x.HTTPStatus
					}
				}
				st.Inflight.NextAttempt = time.Now().Add(relay.RetryDelay(cfg, st.Inflight.Attempts)).UnixMilli()
				if saveErr := save(st); saveErr != nil {
					return fail(saveErr)
				}
				if st.Inflight.Status == "unknown" {
					return fail(relay.UnknownOutcome())
				}
				if ctx.Err() != nil {
					notice("stopped", st, "")
					return nil
				}
				if !relay.Retryable(err) {
					return fail(err)
				}
				if st.Inflight.Attempts > cfg.RetryLimit {
					return fail(exhausted())
				}
				notice("delivery_retry", st, e.Safe(err).Code)
			}
		}
		wake := maxTime(nextScan, time.UnixMilli(st.NextScan))
		if validated && st.Depth() >= cfg.QueueLimit {
			wake = time.Now().Add(time.Minute)
		}
		if validated && st.Inflight != nil {
			wake = minTime(wake, time.UnixMilli(max(st.Inflight.NextAttempt, st.NextDelivery)))
		} else if validated && len(st.Pending) > 0 {
			wake = minTime(wake, time.UnixMilli(st.PendingSince).Add(time.Duration(cfg.BatchWindowSeconds)*time.Second))
		}
		timer := time.NewTimer(max(time.Until(wake), time.Millisecond))
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func relayStatus(st relay.State) map[string]any {
	v := map[string]any{"identity": st.Identity, "coverage": st.Coverage, "collected_sequence": st.After, "collected_events": st.Collected, "filtered_events": st.Filtered, "queued_events": st.Depth(), "acknowledged_events": st.Acknowledged, "acknowledged_batches": st.AckedBatches, "last_scan_at": st.LastScan, "last_ack_at": st.LastAck, "scan_failures": st.ScanFailures, "next_scan_at": st.NextScan}
	v["initialized"] = st.Binding != ""
	v["expired_events"] = st.Expired
	v["checkpoint_stage"] = "collected"
	v["delivery_mode"] = relay.ExplicitAck
	if st.DeliveryMode != "" {
		v["delivery_mode"] = st.DeliveryMode
	}
	v["accepted_events"], v["accepted_batches"], v["last_accepted_at"], v["next_delivery_at"] = st.AcceptedEvents, st.AcceptedBatches, st.LastAccepted, st.NextDelivery
	v["processing"] = "not_reported_by_webhook"
	if len(st.Receipts) > 0 {
		r := st.Receipts[len(st.Receipts)-1]
		v["last_receipt"] = map[string]any{"batch_id": r.ID, "status": r.Status, "events": r.Events, "accepted_at": r.AcceptedAt}
	}
	if st.Inflight != nil {
		v["inflight_id"], v["delivery_attempts"], v["next_attempt_at"] = st.Inflight.Batch.ID, st.Inflight.Attempts, st.Inflight.NextAttempt
		v["delivery_status"], v["http_status"] = st.Inflight.Status, st.Inflight.HTTPStatus
		v["terminal_error"] = st.Inflight.Failure
	}
	return v
}

func (s *relayApp) relayStatusCommand() *cobra.Command {
	var path, consumerPath, id string
	cmd := &cobra.Command{Use: "status", Short: "Read local delivery counts and timestamps; this does not probe process liveness", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if (path == "") == (consumerPath == "") {
			return e.New(e.CategoryUsage, "RELAY_STATUS", "Choose --state for sender delivery or --consumer-state for supervisor processing.")
		}
		if consumerPath != "" {
			st, exists, err := relay.ReadConsumer(consumerPath)
			if err != nil {
				return err
			}
			if !exists {
				return e.New(e.CategoryNotFound, "RELAY_STATE_MISSING", "No supervisor claims have been recorded.")
			}
			v, err := consumerStatus(st, id)
			if err != nil {
				return err
			}
			return s.emit(cmd, v)
		}
		st, exists, err := relay.Read(path)
		if err != nil {
			return err
		}
		if !exists {
			return e.New(e.CategoryNotFound, "RELAY_STATE_MISSING", "No relay state exists yet. Start the configured relay to establish its baseline.")
		}
		v := relayStatus(st)
		if id != "" {
			found := false
			if st.Inflight != nil && st.Inflight.Batch.ID == id {
				v["batch"] = st.Inflight
				found = true
			}
			for _, r := range st.Receipts {
				if r.ID == id {
					v["receipt"] = r
					found = true
					break
				}
			}
			if !found {
				return e.New(e.CategoryNotFound, "RELAY_BATCH_MISSING", "No pending or retained accepted batch matches this ID.")
			}
		}
		return s.emit(cmd, v)
	}}
	cmd.Flags().StringVar(&path, "state", "", "Private sender-owned relay state")
	cmd.Flags().StringVar(&consumerPath, "consumer-state", "", "Private supervisor-owned processing state; mutually exclusive with --state")
	cmd.Flags().StringVar(&id, "batch-id", "", "Inspect a specific retained delivery or supervisor claim")
	return cmd
}
