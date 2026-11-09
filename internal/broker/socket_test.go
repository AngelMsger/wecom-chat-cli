package broker

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskCredentialOnlyAcceptedInSingleBearerHeader(t *testing.T) {
	b, _ := setup(t)
	ev := ingest(t, b, "1", "alice", "private task")
	token := issue(t, b, ev.ID)
	body := `{"op":"task.get","bot_id":"bot","deployment":"support"}`
	for _, tc := range []struct {
		name, body string
		headers    []string
		wantOK     bool
	}{
		{"valid", body, []string{"Bearer " + token}, true},
		{"missing", body, nil, false},
		{"json_token", fmt.Sprintf(`{"op":"task.get","bot_id":"bot","deployment":"support","token":%q}`, token), nil, false},
		{"json_token_with_header", fmt.Sprintf(`{"op":"task.get","bot_id":"bot","deployment":"support","Token":%q}`, token), []string{"Bearer " + token}, false},
		{"wrong_scheme", body, []string{"Basic " + token}, false},
		{"duplicate", body, []string{"Bearer " + token, "Bearer " + token}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://local/v1/request", strings.NewReader(tc.body))
			for _, header := range tc.headers {
				r.Header.Add("Authorization", header)
			}
			w := httptest.NewRecorder()
			b.Handler(false).ServeHTTP(w, r)
			if (w.Code == 200) != tc.wantOK || strings.Contains(w.Body.String(), token) || !tc.wantOK && strings.Contains(w.Body.String(), "private task") {
				t.Fatalf("invalid credential boundary: status %d", w.Code)
			}
		})
	}
}

func TestUntrustedSocketRejectsElevationAndAmbiguousInput(t *testing.T) {
	b, _ := setup(t)
	for _, body := range []string{
		`{"op":"status","bot_id":"bot","deployment":"support","admin":true}`,
		`{"op":"status","op":"memory.set","bot_id":"bot","deployment":"support"}`,
		`{"op":"status","bot_id":"other","deployment":"support"}`,
		`{"op":"status","bot_id":"bot","deployment":"other"}`,
		`{"op":"memory.set","bot_id":"bot","deployment":"support","user_id":"alice","status":"blocked","reason":"abuse"}`,
		strings.Repeat("x", 33<<10),
	} {
		r := httptest.NewRequest("POST", "http://local/v1/request", strings.NewReader(body))
		w := httptest.NewRecorder()
		b.Handler(false).ServeHTTP(w, r)
		if w.Code == 200 || strings.Contains(w.Body.String(), "synthetic-super-secret") {
			t.Fatalf("unsafe socket response: %d %s", w.Code, w.Body.String())
		}
	}
	if len(b.Store.View().Memory) != 0 {
		t.Fatal("failed requests mutated memory")
	}
}
