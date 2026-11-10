package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCLIOutputAndUnknownCommands(t *testing.T) {
	for _, args := range [][]string{{"no-such-command"}, {"memory", "oops"}, {"reply", "send", "--bad-flag"}, {"event", "list", "--format", "table"}, {"event", "list"}, {"event", "watch", "--replay", "--cursor", "saved", "--bot-id", "bot", "--deployment", "support"}, {"reply", "send", "--bot-id", "bot", "--deployment", "support", "--event", "x"}} {
		var out, diag bytes.Buffer
		code := Execute(context.Background(), args, strings.NewReader("hello"), &out, &diag)
		if code == 0 || out.Len() != 0 || !json.Valid(bytes.TrimSpace(diag.Bytes())) {
			t.Fatalf("%v => %d %s %s", args, code, out.String(), diag.String())
		}
	}
	var out, diag bytes.Buffer
	if Execute(context.Background(), []string{"config", "example"}, strings.NewReader(""), &out, &diag) != 0 || !json.Valid(out.Bytes()) || diag.Len() != 0 {
		t.Fatal("config output contract")
	}
}
