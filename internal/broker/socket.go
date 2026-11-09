package broker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/angelmsger/wecom-chat-cli/internal/securefs"
	e "github.com/angelmsger/wecom-chat-cli/pkg/errors"
	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
)

// Handler derives authority from the listener; no body field selects admin.
func (b *Broker) Handler(admin bool) http.Handler {
	sem := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fail := func(err error) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(e.Safe(err).Payload())
		}
		if r.Method != "POST" || r.URL.Path != "/v1/request" || r.URL.RawQuery != "" {
			fail(e.New(e.CategoryUsage, "BAD_REQUEST", "Use POST /v1/request."))
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			fail(e.New(e.CategoryRateLimit, "BUSY", "The local concurrency limit is reached."))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		defer r.Body.Close()
		v, err := io.ReadAll(r.Body)
		var req Request
		if err != nil || safejson.Decode(v, &req, true) != nil {
			fail(e.New(e.CategoryUsage, "BAD_REQUEST", "Malformed, oversized or unsupported request fields."))
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "" {
			if len(r.Header.Values("Authorization")) != 1 || !strings.HasPrefix(auth, "Bearer ") {
				fail(taskDenied())
				return
			}
			req.Token = strings.TrimPrefix(auth, "Bearer ")
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		out, err := b.Handle(ctx, req, admin)
		if err != nil {
			fail(err)
			return
		}
		_ = json.NewEncoder(w).Encode(out)
	})
}
func listen(path string, private bool) (net.Listener, error) {
	if err := securefs.Directory(filepath.Dir(path), private, false); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, e.New(e.CategoryConflict, "SOCKET_EXISTS", "The socket path already exists; the operator must verify no active broker before removing stale sockets.")
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, e.New(e.CategoryConfig, "SOCKET_IO", "Cannot create the protected local socket.")
	}
	mode := os.FileMode(0660)
	if private {
		mode = 0600
	}
	if err = os.Chmod(path, mode); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}
func (b *Broker) Serve(ctx context.Context, run func(context.Context) error) error {
	agent, err := listen(b.Policy.AgentSocket, false)
	if err != nil {
		return err
	}
	defer agent.Close()
	admin, err := listen(b.Policy.OperatorSocket, true)
	if err != nil {
		return err
	}
	defer admin.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	servers := []*http.Server{{Handler: b.Handler(false)}, {Handler: b.Handler(true)}}
	errs := make(chan error, 3)
	var wg sync.WaitGroup
	for i, l := range []net.Listener{agent, admin} {
		srv := servers[i]
		srv.ReadHeaderTimeout = 3 * time.Second
		srv.ReadTimeout = 5 * time.Second
		srv.WriteTimeout = 35 * time.Second
		srv.IdleTimeout = 10 * time.Second
		srv.MaxHeaderBytes = 4096
		srv.ErrorLog = log.New(io.Discard, "", 0)
		wg.Add(1)
		go func() { defer wg.Done(); errs <- srv.Serve(l) }()
	}
	defer func() {
		cancel()
		for _, srv := range servers {
			_ = srv.Close()
		}
		wg.Wait()
	}()
	wg.Add(1)
	go func() { defer wg.Done(); errs <- run(ctx) }()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errs:
			return err
		case <-tick.C:
			if err := b.Store.Expire(b.Now(), b.Policy.RetentionSeconds); err != nil {
				return err
			}
		}
	}
}

func Call(ctx context.Context, socket string, r Request) (json.RawMessage, error) {
	if !filepath.IsAbs(socket) || filepath.Clean(socket) != socket {
		return nil, e.New(e.CategoryUsage, "SOCKET_REQUIRED", "Use an absolute --socket path from the trusted operator.")
	}
	v, _ := json.Marshal(r)
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 35 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://bridge/v1/request", bytes.NewReader(v))
	req.Header.Set("Content-Type", "application/json")
	if r.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, e.New(e.CategoryNetwork, "BROKER_UNAVAILABLE", "Cannot reach the selected local broker; a write may still have completed. Inspect its event status before retrying.")
	}
	defer resp.Body.Close()
	v, err = io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if err != nil || len(v) > 2<<20 {
		return nil, e.New(e.CategoryParse, "BROKER_RESPONSE", "Invalid broker response.")
	}
	if resp.StatusCode != 200 {
		var p e.Payload
		if safejson.Decode(v, &p, true) != nil {
			return nil, e.New(e.CategoryParse, "BROKER_RESPONSE", "Invalid broker error response.")
		}
		return nil, e.New(p.Error.Category, p.Error.Code, p.Error.Message).WithHint(p.Error.Hint).WithNextSteps(p.Error.NextSteps...)
	}
	var data any
	if safejson.Decode(v, &data, false) != nil {
		return nil, e.New(e.CategoryParse, "BROKER_RESPONSE", "Invalid broker JSON.")
	}
	return v, nil
}
