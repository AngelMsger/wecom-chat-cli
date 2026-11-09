package aibot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func peer(t *testing.T, handler func(*websocket.Conn)) *Client {
	t.Helper()
	up := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		handler(c)
	}))
	t.Cleanup(server.Close)
	c := New("bot", "synthetic-secret")
	c.ackTimeout = 60 * time.Millisecond
	c.heartbeat = time.Hour
	c.dial = func(ctx context.Context) (*websocket.Conn, error) {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
		return conn, err
	}
	return c
}
func authenticate(t *testing.T, c *websocket.Conn) bool {
	t.Helper()
	var f Frame
	if err := c.ReadJSON(&f); err != nil {
		t.Error(err)
		return false
	}
	var body map[string]string
	json.Unmarshal(f.Body, &body)
	if f.Cmd != "aibot_subscribe" || body["bot_id"] != "bot" || body["secret"] != "synthetic-secret" {
		t.Error("invalid subscription")
		return false
	}
	return c.WriteJSON(map[string]any{"headers": f.Headers, "errcode": 0}) == nil
}
func callback() *Frame {
	f := frame("aibot_msg_callback", "callback1", map[string]string{"msgid": "1"})
	return &f
}
func TestProtocolRoundTripAndGeneration(t *testing.T) {
	replies := make(chan Frame, 1)
	c := peer(t, func(conn *websocket.Conn) {
		if !authenticate(t, conn) {
			return
		}
		conn.WriteJSON(callback())
		var f Frame
		if conn.ReadJSON(&f) != nil {
			return
		}
		replies <- f
		conn.WriteJSON(map[string]any{"headers": f.Headers, "errcode": 0})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.once(ctx, func(f Frame, g string) error {
			if f.Headers.RequestID != "callback1" {
				t.Error("wrong callback")
			}
			got <- g
			return nil
		}, func(string) {})
	}()
	var generation string
	select {
	case generation = <-got:
	case <-time.After(time.Second):
		t.Fatal("missing callback")
	}
	if err := c.Reply(ctx, "old-generation", "callback1", "stream1", "hello"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("stale generation sent")
	}
	if err := c.Reply(ctx, generation, "callback1", "stream1", "hello"); err != nil {
		t.Fatal(err)
	}
	f := <-replies
	var body struct {
		MsgType string `json:"msgtype"`
		Stream  struct {
			ID, Content string
			Finish      bool
		} `json:"stream"`
	}
	json.Unmarshal(f.Body, &body)
	if f.Cmd != "aibot_respond_msg" || f.Headers.RequestID != "callback1" || body.MsgType != "stream" || body.Stream.ID != "stream1" || body.Stream.Content != "hello" || !body.Stream.Finish {
		t.Fatalf("invalid wire reply: %s", f.Body)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown hung")
	}
}
func TestMissingAcknowledgementIsUncertain(t *testing.T) {
	c := peer(t, func(conn *websocket.Conn) {
		if !authenticate(t, conn) {
			return
		}
		conn.WriteJSON(callback())
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- c.once(ctx, func(_ Frame, g string) error { got <- g; return nil }, func(string) {}) }()
	select {
	case g := <-got:
		if err := c.Reply(ctx, g, "callback1", "stream1", "hello"); !errors.Is(err, ErrUncertain) {
			t.Fatalf("want uncertain: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("no callback")
	}
	cancel()
	<-done
}
func TestTerminalConnectionFailures(t *testing.T) {
	for _, kind := range []string{"auth", "displaced", "intake"} {
		t.Run(kind, func(t *testing.T) {
			c := peer(t, func(conn *websocket.Conn) {
				if kind == "auth" {
					var f Frame
					conn.ReadJSON(&f)
					conn.WriteJSON(map[string]any{"headers": f.Headers, "errcode": 401})
					return
				}
				if !authenticate(t, conn) {
					return
				}
				if kind == "displaced" {
					conn.WriteJSON(frame("aibot_event_callback", "event1", map[string]any{"event": map[string]string{"eventtype": "disconnected_event"}}))
				} else {
					conn.WriteJSON(callback())
				}
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := c.Run(ctx, func(Frame, string) error { return errors.New("disk full") }, func(string) {})
			want := ErrAuthentication
			if kind == "displaced" {
				want = ErrDisplaced
			} else if kind == "intake" {
				want = ErrIntake
			}
			if !errors.Is(err, want) {
				t.Fatalf("want %v got %v", want, err)
			}
		})
	}
}
func TestHeartbeatRequiresAcknowledgement(t *testing.T) {
	ping := make(chan Frame, 1)
	c := peer(t, func(conn *websocket.Conn) {
		if !authenticate(t, conn) {
			return
		}
		var f Frame
		if conn.ReadJSON(&f) == nil {
			ping <- f
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	c.heartbeat = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := c.once(ctx, func(Frame, string) error { return nil }, func(string) {})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatal("missed heartbeat did not end connection")
	}
	f := <-ping
	if f.Cmd != "ping" || !strings.HasPrefix(f.Headers.RequestID, "ping_") {
		t.Fatal("invalid heartbeat wire format")
	}
}
