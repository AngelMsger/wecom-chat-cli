// Package aibot implements the official smart-bot WebSocket wire protocol.
package aibot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/angelmsger/wecom-chat-cli/pkg/safejson"
	"github.com/gorilla/websocket"
)

const Endpoint = "wss://openws.work.weixin.qq.com"

var ErrUnavailable = errors.New("connection unavailable")
var ErrUncertain = errors.New("reply outcome uncertain")
var ErrRejected = errors.New("reply rejected")
var ErrAuthentication = errors.New("subscription rejected")
var ErrDisplaced = errors.New("connection displaced by another subscriber")
var ErrIntake = errors.New("intake stopped; coverage may be incomplete")

type Frame struct {
	Cmd     string `json:"cmd,omitempty"`
	Headers struct {
		RequestID string `json:"req_id"`
	} `json:"headers"`
	Body    json.RawMessage `json:"body,omitempty"`
	ErrCode *int            `json:"errcode,omitempty"`
}

func frame(cmd, id string, body any) Frame {
	f := Frame{Cmd: cmd}
	f.Headers.RequestID = id
	if body != nil {
		f.Body, _ = json.Marshal(body)
	}
	return f
}
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure randomness unavailable")
	}
	return hex.EncodeToString(b[:])
}

type session struct {
	conn       *websocket.Conn
	generation string
	writeMu    sync.Mutex
	mu         sync.Mutex
	pending    map[string]chan Frame
	done       chan struct{}
}

func (s *session) write(f Frame) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.conn.WriteJSON(f)
}

type Client struct {
	bot, secret string
	mu          sync.Mutex
	current     *session
	dial        func(context.Context) (*websocket.Conn, error)
	ackTimeout  time.Duration
	heartbeat   time.Duration
}

func New(bot, secret string) *Client {
	c := &Client{bot: bot, secret: secret, ackTimeout: 5 * time.Second, heartbeat: 30 * time.Second}
	c.dial = func(ctx context.Context) (*websocket.Conn, error) {
		d := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: 10 * time.Second}
		conn, resp, err := d.DialContext(ctx, Endpoint, nil)
		if resp != nil && err != nil {
			resp.Body.Close()
		}
		return conn, err
	}
	return c
}
func (c *Client) Connected() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.current != nil }
func (c *Client) Generation() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return ""
	}
	return c.current.generation
}

// Reply uses only the original callback's request ID. No proactive send exists.
func (c *Client) Reply(ctx context.Context, generation, reqID, streamID, text string) error {
	c.mu.Lock()
	s := c.current
	c.mu.Unlock()
	if s == nil || s.generation != generation {
		return ErrUnavailable
	}
	ch := make(chan Frame, 1)
	s.mu.Lock()
	if _, ok := s.pending[reqID]; ok {
		s.mu.Unlock()
		return ErrUnavailable
	}
	s.pending[reqID] = ch
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, reqID); s.mu.Unlock() }()
	f := frame("aibot_respond_msg", reqID, map[string]any{"msgtype": "stream", "stream": map[string]any{"id": streamID, "content": text, "finish": true}})
	if err := s.write(f); err != nil {
		return ErrUncertain
	}
	t := time.NewTimer(c.ackTimeout)
	defer t.Stop()
	select {
	case ack := <-ch:
		if ack.ErrCode == nil {
			return ErrUncertain
		}
		if *ack.ErrCode != 0 {
			return ErrRejected
		}
		return nil
	case <-ctx.Done():
		return ErrUncertain
	case <-s.done:
		return ErrUncertain
	case <-t.C:
		return ErrUncertain
	}
}

// Run retries transport failures, but stops on bad credentials, displacement or
// failed durable intake. Callers must not assume disconnected messages replay.
func (c *Client) Run(ctx context.Context, onMessage func(Frame, string) error, notice func(string)) error {
	backoff := time.Second
	for {
		start := time.Now()
		err := c.once(ctx, onMessage, notice)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, ErrAuthentication) || errors.Is(err, ErrDisplaced) || errors.Is(err, ErrIntake) {
			return err
		}
		notice("disconnected_coverage_gap")
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(backoff/2)+1))
		delay := backoff + time.Duration(j.Int64())
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) once(ctx context.Context, onMessage func(Frame, string) error, notice func(string)) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Close()
	conn.SetReadLimit(64 << 10)
	s := &session{conn: conn, generation: randomID(), pending: map[string]chan Frame{}, done: make(chan struct{})}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	read := func() (Frame, error) {
		var f Frame
		kind, b, err := conn.ReadMessage()
		if err != nil {
			return f, err
		}
		if kind != websocket.TextMessage || safejson.Decode(b, &f, false) != nil || f.Headers.RequestID == "" || len(f.Headers.RequestID) > 512 {
			return f, ErrUnavailable
		}
		return f, nil
	}
	id := "aibot_subscribe_" + randomID()
	if s.write(frame("aibot_subscribe", id, map[string]string{"bot_id": c.bot, "secret": c.secret})) != nil {
		return ErrUnavailable
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	auth, err := read()
	if err != nil {
		return ErrUnavailable
	}
	if auth.Cmd != "" || auth.Headers.RequestID != id || auth.ErrCode == nil {
		return ErrUnavailable
	}
	if *auth.ErrCode != 0 {
		return ErrAuthentication
	}
	_ = conn.SetReadDeadline(time.Time{})
	c.mu.Lock()
	c.current = s
	c.mu.Unlock()
	notice("connected")
	defer func() {
		c.mu.Lock()
		if c.current == s {
			c.current = nil
		}
		c.mu.Unlock()
		close(s.done)
	}()
	callbacks := make(chan Frame, 128)
	readErr := make(chan error, 1)
	readerDone := make(chan struct{})
	defer func() { conn.Close(); <-readerDone }()
	go func() {
		defer close(readerDone)
		for {
			f, err := read()
			if err != nil {
				readErr <- ErrUnavailable
				return
			}
			if f.Cmd == "aibot_event_callback" {
				var body struct {
					Event struct {
						Type string `json:"eventtype"`
					} `json:"event"`
				}
				if safejson.Decode(f.Body, &body, false) != nil {
					readErr <- ErrUnavailable
					return
				}
				if body.Event.Type == "disconnected_event" {
					readErr <- ErrDisplaced
					return
				}
				continue
			}
			if f.Cmd == "aibot_msg_callback" {
				select {
				case callbacks <- f:
				default:
					readErr <- ErrIntake
					return
				}
				continue
			}
			if f.Cmd == "" {
				s.mu.Lock()
				ch := s.pending[f.Headers.RequestID]
				s.mu.Unlock()
				if ch != nil {
					select {
					case ch <- f:
					default:
					}
				}
			}
		}
	}()
	tick := time.NewTicker(c.heartbeat)
	defer tick.Stop()
	var ping chan Frame
	var pingID string
	var timeout <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readErr:
			return err
		case f := <-callbacks:
			if onMessage(f, s.generation) != nil {
				return ErrIntake
			}
		case <-tick.C:
			if ping != nil {
				return ErrUnavailable
			}
			pingID = "ping_" + randomID()
			ping = make(chan Frame, 1)
			s.mu.Lock()
			s.pending[pingID] = ping
			s.mu.Unlock()
			if s.write(frame("ping", pingID, nil)) != nil {
				return ErrUnavailable
			}
			timer = time.NewTimer(c.ackTimeout)
			timeout = timer.C
		case ack := <-ping:
			if ack.ErrCode == nil || *ack.ErrCode != 0 {
				return ErrUnavailable
			}
			timer.Stop()
			timeout = nil
			s.mu.Lock()
			delete(s.pending, pingID)
			s.mu.Unlock()
			ping = nil
		case <-timeout:
			return ErrUnavailable
		}
	}
}
