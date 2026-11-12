package app

import (
	"io"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/relay"
	"github.com/angelmsger/wecom-chat-cli/internal/securefs"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
	"github.com/spf13/cobra"
)

func (s *relayApp) relayClaimCommand() *cobra.Command {
	var path, input string
	cmd := &cobra.Command{Use: "claim", Short: "Atomically claim a webhook batch in supervisor-owned state before processing", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		dry := *s.dry
		var data []byte
		var err error
		if input == "-" {
			data, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), (1<<20)+1))
		} else if input != "" {
			data, err = securefs.ReadPrivate(input, 1<<20, true)
		}
		var batch relay.Batch
		if err != nil || len(data) == 0 || len(data) > 1<<20 || safejson.Decode(data, &batch, true) != nil {
			return e.New(e.CategoryUsage, "RELAY_BATCH", "Provide the intact webhook JSON using --batch-file FILE (private) or --batch-file - (stdin).")
		}
		if err = batch.Validate(); err != nil {
			return err
		}
		var store *relay.Store
		if !dry {
			if err = s.guard("relay claim"); err != nil {
				return err
			}
			store, err = relay.Open(path)
			if err != nil {
				return err
			}
			defer store.Close()
		}
		st, _, err := relay.ReadConsumer(path)
		if err != nil {
			return err
		}
		result, err := st.ClaimBatch(batch, time.Now())
		if err != nil {
			return err
		}
		if dry {
			return s.emit(cmd, map[string]any{"dry_run": true, "batch_id": batch.ID, "identity": batch.Identity, "would_claim": result.Claimed, "status": result.Status})
		}
		if err = store.SaveConsumer(st); err != nil {
			return err
		}
		return s.emit(cmd, result)
	}}
	cmd.Flags().StringVar(&path, "consumer-state", "", "Private supervisor-owned claim file, one per bot/deployment/ledger (required)")
	cmd.Flags().StringVar(&input, "batch-file", "", "Original relay batch JSON; - reads stdin (required)")
	return cmd
}

func (s *relayApp) relayConsumeCommand() *cobra.Command {
	var path, id, token, result string
	cmd := &cobra.Command{Use: "consume", Short: "Record a claimed batch as processed or failed; never send chat replies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		dry := *s.dry
		var store *relay.Store
		var err error
		if !dry {
			if err = s.guard("relay consume"); err != nil {
				return err
			}
			store, err = relay.Open(path)
			if err != nil {
				return err
			}
			defer store.Close()
		}
		st, exists, err := relay.ReadConsumer(path)
		if err != nil {
			return err
		}
		if !exists {
			return e.New(e.CategoryNotFound, "RELAY_STATE_MISSING", "Claim the batch before recording processing completion.")
		}
		if err = st.Complete(id, token, result, time.Now()); err != nil {
			return err
		}
		if !dry {
			if err = store.SaveConsumer(st); err != nil {
				return err
			}
		}
		return s.emit(cmd, map[string]any{"batch_id": id, "status": result, "dry_run": dry, "reported_by": "supervisor"})
	}}
	cmd.Flags().StringVar(&path, "consumer-state", "", "Private supervisor-owned claim file (required)")
	cmd.Flags().StringVar(&id, "batch-id", "", "Batch ID returned by relay claim (required)")
	cmd.Flags().StringVar(&token, "claim-id", "", "Claim token returned only to the successful claimant (required)")
	cmd.Flags().StringVar(&result, "result", "", "processed or failed (required); neither permits automatic replay")
	return cmd
}

func (s *relayApp) relayResolveCommand() *cobra.Command {
	var path, id, outcome, evidence string
	cmd := &cobra.Command{Use: "resolve", Short: "Reconcile an unknown HTTP acceptance using verified host evidence", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		dry := *s.dry
		if (outcome != "accepted" && outcome != "not_accepted") || !config.ID(evidence) {
			return e.New(e.CategoryUsage, "RELAY_RESOLUTION", "Choose accepted or not_accepted and a non-secret --evidence reference verified from host run history. An absent supervisor claim alone is not proof of rejection.")
		}
		var store *relay.Store
		var err error
		if !dry {
			if err = s.guard("relay resolve"); err != nil {
				return err
			}
			store, err = relay.Open(path)
			if err != nil {
				return err
			}
			defer store.Close()
		}
		st, exists, err := relay.Read(path)
		if err != nil {
			return err
		}
		if !exists || st.DeliveryMode != relay.HTTPAccepted || st.Inflight == nil || st.Inflight.Batch.ID != id || (st.Inflight.Status != "unknown" && st.Inflight.Status != "sending") {
			return e.New(e.CategoryConflict, "RELAY_RESOLUTION", "Only the exact uncertain batch in HTTP acceptance mode can be reconciled. Stop its relay first.")
		}
		if outcome == "accepted" {
			err = st.Accept(time.Now(), evidence)
		} else {
			st.Inflight.Status = "rejected"
			st.Inflight.Evidence = evidence
		}
		if err != nil {
			return err
		}
		if !dry {
			if err = store.Save(st); err != nil {
				return err
			}
		}
		return s.emit(cmd, map[string]any{"batch_id": id, "outcome": outcome, "dry_run": dry, "processing": "not_reported_by_webhook"})
	}}
	cmd.Flags().StringVar(&path, "state", "", "Private sender-owned relay state (required)")
	cmd.Flags().StringVar(&id, "batch-id", "", "Exact unknown batch ID (required)")
	cmd.Flags().StringVar(&outcome, "outcome", "", "accepted or not_accepted, verified from host history (required)")
	cmd.Flags().StringVar(&evidence, "evidence", "", "Non-secret host run or operator reconciliation reference (required)")
	return cmd
}

// Keep supervisor claim tokens out of routine status output. A claimant retains its
// token privately; an operator can inspect the protected file for recovery.
func consumerStatus(st relay.ConsumerState, id string) (map[string]any, error) {
	counts := map[string]int{}
	for _, c := range st.Batches {
		counts[c.Status]++
	}
	v := map[string]any{"identity": st.Identity, "batches": len(st.Batches), "events": len(st.Events), "processing": counts, "reported_by": "supervisor"}
	if id != "" {
		c, ok := st.Batches[id]
		if !ok {
			return nil, e.New(e.CategoryNotFound, "RELAY_BATCH_MISSING", "This batch has not been claimed in this supervisor state.")
		}
		c.Token = ""
		v["batch_id"] = id
		v["claim"] = c
	}
	return v, nil
}
