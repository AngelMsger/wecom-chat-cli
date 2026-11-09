// Package app exposes a thin CLI over the broker's local protocol.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/angelmsger/agent-cli-kit/cliflags"
	"github.com/angelmsger/agent-cli-kit/output"
	"github.com/angelmsger/wecom-chat-cli/internal/broker"
	"github.com/angelmsger/wecom-chat-cli/internal/config"
	"github.com/angelmsger/wecom-chat-cli/internal/state"
	"github.com/angelmsger/wecom-chat-cli/pkg/aibot"
	"github.com/angelmsger/wecom-chat-cli/pkg/constants"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var socket, botID, deployment, format string
	var writes, dryRun bool
	root := &cobra.Command{Use: "wecom-chat-cli", Short: "Isolate agents from untrusted WeCom smart-bot conversations", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&socket, "socket", "", "Absolute worker or trusted operator Unix socket path")
	root.PersistentFlags().StringVar(&botID, "bot-id", "", "Expected bot ID from the operator")
	root.PersistentFlags().StringVar(&deployment, "deployment", "", "Expected deployment name from the operator")
	root.PersistentFlags().StringVar(&format, "format", "json", "Output: json or ndjson (trust fields cannot be removed)")
	root.PersistentFlags().BoolVar(&writes, "allow-writes", false, "Permit this invocation's mutation; grants no server authority")
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Validate a write without state changes or remote sends")
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		if format != "json" && format != "ndjson" {
			return e.New(e.CategoryUsage, "INVALID_FORMAT", "Use json or ndjson; raw, table and field projection are not supported.")
		}
		return nil
	}
	emit := func(c *cobra.Command, v any) error {
		return output.Emit(v, output.Options{Format: format, Writer: c.OutOrStdout()})
	}
	call := func(c *cobra.Command, r broker.Request, write bool) error {
		if !config.ID(botID) || !config.ID(deployment) {
			return e.New(e.CategoryUsage, "SCOPE_REQUIRED", "Supply --bot-id and --deployment from the broker operator.")
		}
		if write && !dryRun && !writes {
			return e.ReadOnlyBlocked(r.Op, e.ReadOnlyMode{DryRun: true})
		}
		r.BotID = botID
		r.Deployment = deployment
		r.DryRun = dryRun
		if r.Op == "task.get" || r.Op == "reply.propose" || r.Op == "memory.report" {
			r.Token = os.Getenv("WECOM_CHAT_TASK_TOKEN")
		}
		ctx, cancel := context.WithTimeout(c.Context(), 35*time.Second)
		defer cancel()
		v, err := broker.Call(ctx, socket, r)
		if err != nil {
			return err
		}
		if r.Op == "event.list" || r.Op == "memory.list" {
			var p broker.Page
			if err = json.Unmarshal(v, &p); err != nil {
				return err
			}
			return output.EmitList(p.Items, p.Next, p.HasMore, output.Options{Format: format, Writer: c.OutOrStdout(), NoticeWriter: c.ErrOrStderr()})
		}
		return emit(c, v)
	}
	root.AddCommand(&cobra.Command{Use: "version", Short: "Show the local CLI version", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return emit(c, map[string]string{"version": constants.Version, "commit": constants.Commit, "build_time": constants.BuildTime})
	}})
	root.AddCommand(&cobra.Command{Use: "status", Short: "Inspect broker connectivity and scope", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return call(c, broker.Request{Op: "status"}, false) }})
	cfg := &cobra.Command{Use: "config", Short: "Inspect operator policy"}
	cfg.AddCommand(&cobra.Command{Use: "example", Short: "Print a deny-by-default operator policy", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return emit(c, config.Example()) }})
	var policyPath string
	validate := &cobra.Command{Use: "validate", Short: "Validate a protected operator policy offline", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		p, err := config.Load(policyPath)
		if err != nil {
			return err
		}
		return emit(c, map[string]any{"valid": true, "policy_digest": p.Digest()})
	}}
	validate.Flags().StringVar(&policyPath, "policy", "", "Absolute policy JSON path")
	cfg.AddCommand(validate)
	root.AddCommand(cfg)
	auth := &cobra.Command{Use: "auth", Short: "Read credential provisioning guidance"}
	auth.AddCommand(&cobra.Command{Use: "guide", Short: "Explain broker-only smart-bot credentials", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return emit(c, map[string]any{"mode": "smart_bot_api_long_connection", "bot_id": "Obtain Bot ID from the smart-bot API configuration in WeCom administration.", "secret": "Set WECOM_CHAT_BOT_SECRET only in the isolated broker process environment.", "documentation": "https://github.com/WecomTeam/aibot-node-sdk", "coexistence": "Use a dedicated test bot first. A new subscriber can displace an existing connection; changing API mode can disable an existing webhook.", "next_steps": []string{"wecom-chat-cli config example"}})
	}})
	root.AddCommand(auth)
	service := &cobra.Command{Use: "broker", Short: "Run the credential-holding operator service"}
	var servePolicy string
	serve := &cobra.Command{Use: "serve", Short: "Maintain the smart-bot connection and scoped worker/operator sockets", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		p, err := config.Load(servePolicy)
		if err != nil {
			return err
		}
		if dryRun {
			return emit(c, map[string]any{"dry_run": true, "policy_digest": p.Digest(), "endpoint": aibot.Endpoint})
		}
		secret := os.Getenv("WECOM_CHAT_BOT_SECRET")
		if len(secret) < 8 || len(secret) > 4096 {
			return e.New(e.CategoryAuth, "AUTH_NO_SECRET", "Set the bot Secret only in the broker's protected environment.")
		}
		s, err := state.Open(p.StateDir, p.Deployment+"/"+p.BotID)
		if err != nil {
			return err
		}
		defer s.Close()
		if err = s.Expire(time.Now(), p.RetentionSeconds); err != nil {
			return err
		}
		client := aibot.New(p.BotID, secret)
		b := broker.New(p, s, client, secret)
		return b.Serve(c.Context(), func(ctx context.Context) error {
			err := client.Run(ctx, b.Ingest, func(status string) {
				output.EmitNotice(c.ErrOrStderr(), map[string]any{"_notice": map[string]string{"connection": status}})
			})
			if errors.Is(err, aibot.ErrAuthentication) {
				return e.New(e.CategoryAuth, "AUTH_REJECTED", "The smart-bot subscription was rejected; ask the operator to verify credentials.")
			}
			if errors.Is(err, aibot.ErrDisplaced) {
				return e.New(e.CategoryConflict, "CONNECTION_DISPLACED", "Another subscriber displaced this connection. Do not auto-restart; verify bot ownership with the operator.")
			}
			if err != nil {
				return e.New(e.CategoryConfig, "INTAKE_STOPPED", "Durable intake stopped. Preserve the ledger and inspect capacity and storage before restarting; coverage may be incomplete.")
			}
			return nil
		})
	}}
	serve.Flags().StringVar(&servePolicy, "policy", "", "Absolute policy JSON path")
	service.AddCommand(serve)
	root.AddCommand(service)
	events := &cobra.Command{Use: "event", Short: "Operator-only event discovery and monitoring"}
	var cursor string
	var limit int
	var includeSuppressed bool
	list := &cobra.Command{Use: "list", Short: "Read one bounded page; reuse its cursor", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return call(c, broker.Request{Op: "event.list", Cursor: cursor, Limit: limit, IncludeSuppressed: includeSuppressed}, false)
	}}
	list.Flags().StringVar(&cursor, "cursor", "", "Cursor from a prior page or event")
	list.Flags().IntVar(&limit, "limit", 50, "Maximum events, 1..100")
	list.Flags().BoolVar(&includeSuppressed, "include-suppressed", false, "Include redacted quarantined/suppressed events for operator inspection")
	events.AddCommand(list)
	var getEvent string
	get := &cobra.Command{Use: "get", Short: "Inspect one event and its durable reply outcome", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return call(c, broker.Request{Op: "event.get", EventID: getEvent}, false)
	}}
	get.Flags().StringVar(&getEvent, "event", "", "Event ID from list/watch or a reply result")
	events.AddCommand(get)
	var watchCursor string
	var duration time.Duration
	var maxEvents int
	watch := &cobra.Command{Use: "watch", Short: "Stream bounded NDJSON with resumable event cursors", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if duration < time.Second || duration > time.Hour || maxEvents < 1 || maxEvents > 10000 || !config.ID(botID) || !config.ID(deployment) {
			return e.New(e.CategoryUsage, "INVALID_WATCH", "Use duration 1s..1h, max-events 1..10000 and explicit bot/deployment scope.")
		}
		ctx, cancel := context.WithTimeout(c.Context(), duration)
		defer cancel()
		count := 0
		next := watchCursor
		for count < maxEvents && ctx.Err() == nil {
			v, err := broker.Call(ctx, socket, broker.Request{Op: "event.list", BotID: botID, Deployment: deployment, Cursor: next, Limit: min(100, maxEvents-count), WaitSeconds: 25})
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				return err
			}
			var p broker.Page
			if err = json.Unmarshal(v, &p); err != nil {
				return err
			}
			for _, item := range p.Items {
				if err = output.Emit(item, output.Options{Format: "ndjson", Writer: c.OutOrStdout()}); err != nil {
					return err
				}
				count++
			}
			next = p.Next
		}
		output.EmitNotice(c.ErrOrStderr(), map[string]any{"_notice": map[string]any{"cursor": next, "delivered": count, "complete": false, "reason": "watch_bound_reached"}})
		return nil
	}}
	watch.Flags().StringVar(&watchCursor, "cursor", "", "Last successfully handled event cursor")
	watch.Flags().DurationVar(&duration, "duration", time.Minute, "Total observation deadline, at most 1h")
	watch.Flags().IntVar(&maxEvents, "max-events", 100, "Maximum delivered events")
	events.AddCommand(watch)
	root.AddCommand(events)
	task := &cobra.Command{Use: "task", Short: "Issue or consume single-event worker grants"}
	var issueEvent string
	issue := &cobra.Command{Use: "issue", Short: "Operator: issue a short-lived grant; stdout contains a secret token", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return call(c, broker.Request{Op: "task.issue", EventID: issueEvent}, true)
	}}
	issue.Flags().StringVar(&issueEvent, "event", "", "Admitted event ID from event list/watch")
	task.AddCommand(issue)
	task.AddCommand(&cobra.Command{Use: "get", Short: "Worker: read only the task bound to WECOM_CHAT_TASK_TOKEN", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return call(c, broker.Request{Op: "task.get"}, false) }})
	var revokeEvent string
	var revokeYes bool
	revoke := &cobra.Command{Use: "revoke", Short: "Operator: revoke a task grant and prevent pending sends", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if !dryRun && !revokeYes {
			return e.New(e.CategoryUsage, "CONFIRMATION_REQUIRED", "Use --yes after reviewing the task to revoke.")
		}
		return call(c, broker.Request{Op: "task.revoke", EventID: revokeEvent}, true)
	}}
	revoke.Flags().StringVar(&revokeEvent, "event", "", "Event whose task must be revoked")
	revoke.Flags().BoolVar(&revokeYes, "yes", false, "Confirm revocation")
	task.AddCommand(revoke)
	root.AddCommand(task)
	session := &cobra.Command{Use: "session", Short: "Operator-controlled conversation fences"}
	var resetEvent string
	var epoch int64
	var resetYes bool
	reset := &cobra.Command{Use: "reset", Short: "Advance the session epoch and invalidate all previous tasks", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if !dryRun && !resetYes {
			return e.New(e.CategoryUsage, "CONFIRMATION_REQUIRED", "Use --yes after reviewing the session to reset.")
		}
		return call(c, broker.Request{Op: "session.reset", EventID: resetEvent, ExpectedEpoch: epoch}, true)
	}}
	reset.Flags().StringVar(&resetEvent, "event", "", "An event identifying the session")
	reset.Flags().Int64Var(&epoch, "expected-epoch", 0, "Current session epoch from an event record")
	reset.Flags().BoolVar(&resetYes, "yes", false, "Confirm the conversation reset")
	session.AddCommand(reset)
	root.AddCommand(session)
	reply := &cobra.Command{Use: "reply", Short: "Propose immutable replies and independently review sends"}
	var templateID string
	propose := &cobra.Command{Use: "propose", Short: "Worker: submit stdin text for review, or select an approved literal template", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		text := ""
		if templateID == "" {
			v, err := io.ReadAll(io.LimitReader(c.InOrStdin(), 4097))
			if err != nil || len(v) > 4096 {
				return e.New(e.CategoryUsage, "INVALID_TEXT", "Provide at most 4096 UTF-8 bytes on stdin.")
			}
			text = string(v)
		}
		return call(c, broker.Request{Op: "reply.propose", Text: text, Template: templateID}, true)
	}}
	propose.Flags().StringVar(&templateID, "template", "", "Policy-approved literal template ID; only in template mode")
	reply.AddCommand(propose)
	var proposalEvent string
	proposalGet := &cobra.Command{Use: "get", Short: "Operator: inspect the exact draft, target, digest and outcome", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return call(c, broker.Request{Op: "reply.get", EventID: proposalEvent}, false)
	}}
	proposalGet.Flags().StringVar(&proposalEvent, "event", "", "Event ID with a proposal")
	reply.AddCommand(proposalGet)
	for _, action := range []string{"approve", "reject"} {
		var id, digest string
		var yes bool
		cmd := &cobra.Command{Use: action, Short: "Operator: " + action + " the exact immutable proposal digest", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
			if !dryRun && !yes {
				return e.New(e.CategoryUsage, "CONFIRMATION_REQUIRED", "Inspect reply get, then pass the exact digest and --yes.")
			}
			return call(c, broker.Request{Op: "reply." + action, EventID: id, Digest: digest}, true)
		}}
		cmd.Flags().StringVar(&id, "event", "", "Reviewed event ID")
		cmd.Flags().StringVar(&digest, "digest", "", "Exact digest returned by reply get")
		cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the reviewed proposal action")
		reply.AddCommand(cmd)
	}
	root.AddCommand(reply)
	memory := &cobra.Command{Use: "memory", Short: "Manage scoped sender risk records"}
	var reportReason string
	report := &cobra.Command{Use: "report", Short: "Worker: report a fixed risk reason without changing user permissions", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return call(c, broker.Request{Op: "memory.report", Reason: reportReason}, true)
	}}
	report.Flags().StringVar(&reportReason, "reason", "", "prompt_injection, secret_request, destructive_request, abuse, other")
	memory.AddCommand(report)
	memory.AddCommand(&cobra.Command{Use: "list", Short: "List active risk records through the operator socket", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return call(c, broker.Request{Op: "memory.list"}, false) }})
	var userID, memStatus, memReason string
	var yes bool
	set := &cobra.Command{Use: "set", Short: "Operator-only block, quarantine, watch or clear for an allowlisted user", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if !dryRun && !yes {
			return e.New(e.CategoryUsage, "CONFIRMATION_REQUIRED", "Use --yes after reviewing the exact user and status; operator socket access is still required.")
		}
		return call(c, broker.Request{Op: "memory.set", UserID: userID, Status: memStatus, Reason: memReason}, true)
	}}
	set.Flags().StringVar(&userID, "user", "", "Stable user ID from an event or reviewed operator policy")
	set.Flags().StringVar(&memStatus, "status", "", "blocked, quarantined, watch or clear")
	set.Flags().StringVar(&memReason, "reason", "other", "Fixed reason code")
	set.Flags().BoolVar(&yes, "yes", false, "Confirm the selected user's policy change")
	memory.AddCommand(set)
	root.AddCommand(memory)
	var groups func(*cobra.Command)
	groups = func(c *cobra.Command) {
		if len(c.Commands()) > 0 && c.RunE == nil {
			c.RunE = func(c *cobra.Command, args []string) error {
				if len(args) > 0 {
					return e.New(e.CategoryUsage, "UNKNOWN_COMMAND", "Unknown command; consult --help.")
				}
				return c.Help()
			}
		}
		for _, sub := range c.Commands() {
			groups(sub)
		}
	}
	groups(root)
	root.SetFlagErrorFunc(func(*cobra.Command, error) error {
		return e.New(e.CategoryUsage, "INVALID_FLAGS", "Invalid flags; consult this command's --help.")
	})
	return root
}

func Execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	root := NewRootCmd()
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	argv, corrections := cliflags.Normalize(args, cliflags.Collect(root))
	for _, c := range corrections {
		output.EmitNotice(errOut, map[string]any{"_notice": c})
	}
	root.SetArgs(argv)
	if err := root.ExecuteContext(ctx); err != nil {
		ce := e.Safe(err)
		if ce.Code == "INTERNAL" {
			ce = e.New(e.CategoryUsage, "INVALID_COMMAND", "Command failed; consult --help and operator diagnostics.")
		}
		_ = json.NewEncoder(errOut).Encode(ce.Payload())
		return e.ExitCode(ce)
	}
	return 0
}
